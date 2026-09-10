// 主控 API 客户端（8580——X-Auth-Token）
// 异步: tokio runtime（egui-async 在 eframe 0.36 下有帧号 bug——自己管理）
use serde::Deserialize;
use serde_json::Value;
use std::sync::{Arc, Mutex, OnceLock};

pub const API_BASE: &str = "http://127.0.0.1:8580";
pub const API_TOKEN: &str = "x3gw-shared-2026";
pub const AI_BASE: &str = "http://127.0.0.1:8082"; // F5 AI 动力（网关——Mr2109统一接口）

/// http_client — 统一构造 reqwest client（2026-09-10 根治回环代理问题）
/// 背景：reqwest 默认尊重系统代理（macOS Clash Party :7895），对 **127.0.0.1 回环请求**也会走代理，
/// 结果是"主控离线"（系统 UI/URLSession 会自动绕过回环，故只有本进程中招）；此前靠 start-zerg-ui.sh 剥代理治标。
/// 本 UI 的全部请求都指向回环（8580 主控 / 8082 网关），故一律 no_proxy —— 不再依赖启动脚本。
/// A15（2026-09-10 审计）：三档 client 各用 OnceLock 缓存单例——reqwest::Client 内含连接池、本应长期复用；
/// 原实现每次请求都 build() 一个新的 → 每请求一条新 TCP、keep-alive 全废（与 3s/5s/10s 轮询叠加）。
/// 返回 clone()（Client 内部是 Arc，浅拷贝共享连接池），调用方签名与用法不变。
static CLIENT_DEFAULT: OnceLock<reqwest::Client> = OnceLock::new();
static CLIENT_JSON: OnceLock<reqwest::Client> = OnceLock::new();
static CLIENT_AI: OnceLock<reqwest::Client> = OnceLock::new();

pub fn http_client() -> reqwest::Client {
    // A03（2026-09-10 审计）：连接超时 3s——服务器不可达时不再挂死 worker；**不设总超时**（流式生成可能数分钟）
    CLIENT_DEFAULT
        .get_or_init(|| match reqwest::Client::builder()
            .no_proxy()
            .connect_timeout(std::time::Duration::from_secs(3))
            .build()
        {
            Ok(c) => c,
            // 兜底（构建失败极罕见：TLS 后端初始化异常）——宁可退回默认 client，也绝不递归调用自身
            // （2026-09-10 审计 A01 修正：原写法 unwrap_or_else(|_| http_client()) 会无限递归 → 栈溢出）
            Err(_) => reqwest::Client::new(),
        })
        .clone()
}

/// JSON 短请求 client（A03 2026-09-10）：连接 3s + 总 20s——轮询/列表/操作类，防单请求挂死拖垮 2 线程 runtime
pub fn http_client_json() -> reqwest::Client {
    CLIENT_JSON
        .get_or_init(|| match reqwest::Client::builder()
            .no_proxy()
            .connect_timeout(std::time::Duration::from_secs(3))
            .timeout(std::time::Duration::from_secs(20))
            .build()
        {
            // 兜底：默认档单例（http_client 用独立 OnceLock，不会递归）
            Err(_) => http_client(),
            Ok(c) => c,
        })
        .clone()
}

/// AI 网关 client（A03 2026-09-10）：连接 3s + 总 120s——模型生成慢，20s 会误杀
pub fn http_client_ai() -> reqwest::Client {
    CLIENT_AI
        .get_or_init(|| match reqwest::Client::builder()
            .no_proxy()
            .connect_timeout(std::time::Duration::from_secs(3))
            .timeout(std::time::Duration::from_secs(120))
            .build()
        {
            // 兜底：默认档单例（http_client 用独立 OnceLock，不会递归）
            Err(_) => http_client(),
            Ok(c) => c,
        })
        .clone()
}

/// parse_api_error（A19 2026-09-10 审计）：统一解析错误响应体——主控/网关两种格式
/// `{"error":"msg"}` 或 `{"error":{"type":"code","message":"msg"}}`；非 JSON（网关 HTML / 空 body）回显前 200 字符。
/// 供 sync_get_public / doc_op_blocking / json_body 共用——原 doc_op_blocking 缺 message 分支，错误提示退化成"HTTP 500"。
fn parse_api_error(status: reqwest::StatusCode, body: &str) -> String {
    if let Ok(v) = serde_json::from_str::<Value>(body) {
        if let Some(e) = v.get("error") {
            if let Some(s) = e.as_str() {
                return format!("HTTP {}: {}", status, s);
            }
            if let Some(m) = e.get("message").and_then(|m| m.as_str()) {
                return format!("HTTP {}: {}", status, m);
            }
        }
    }
    let brief: String = body.chars().take(200).collect();
    if brief.is_empty() {
        format!("HTTP {}", status)
    } else {
        format!("HTTP {}: {}", status, brief)
    }
}

/// json_body（A04 2026-09-10 审计）：**先判 HTTP 状态再解析**——5xx/4xx 不再被当成"成功但空数据"
async fn json_body(r: reqwest::Response) -> Result<Value, String> {
    let status = r.status();
    let text = r.text().await.unwrap_or_default();
    if !status.is_success() {
        // A19（2026-09-10 审计）：错误体解析统一走 parse_api_error
        return Err(parse_api_error(status, &text));
    }
    serde_json::from_str::<Value>(&text).map_err(|e| format!("解析失败: {}", e))
}

/// F5 AI 调用（网关 8082 /v1/responses——OpenAI responses 格式）
/// 解析 output 里的 output_text 文本（跳过 reasoning）
pub async fn ai_prompt_blocking(model: &str, prompt: &str) -> Result<String, String> {
    let client = http_client_ai();
    let url = format!("{}/v1/responses", AI_BASE);
    // 适配器铁律（Mr2109 2026-08-27+28）：程序不硬编码 max_tokens——传 0/不带由适配器决定
    // 思考不能关——深度由 reasoning effort(low) 统一控制
    let body = serde_json::json!({
        "model": model,
        "input": prompt,
        "reasoning": {"effort": "low"},
    });
    let resp = client
        .post(&url)
        .header("Content-Type", "application/json")
        .header("X-Auth-Token", API_TOKEN)
        .json(&body)
        .send()
        .await
        .map_err(|e| format!("AI 请求失败: {}", e))?;
    let status = resp.status();
    // A16（2026-09-10 审计）：**先判状态、再解析 JSON**——错误体可能是网关 502 HTML / 空 body，
    // 原实现先 resp.json() 会把真实状态码吞掉、只报"AI 响应解析失败"。
    let text = resp
        .text()
        .await
        .map_err(|e| format!("AI 响应读取失败: {}", e))?;
    if !status.is_success() {
        let brief: String = text.chars().take(200).collect();
        return Err(format!(
            "AI 错误({}): {}",
            status,
            if brief.is_empty() { "<空响应>".to_string() } else { brief }
        ));
    }
    let json: Value =
        serde_json::from_str(&text).map_err(|e| format!("AI 响应解析失败: {}", e))?;
    // 解析 output_text（跳过 reasoning）
    // A14（2026-09-10 审计）：**累积全部** output_text——responses 格式 output 常含多个 item（工具调用 + 多段文本），
    // 原实现遇到第一段就 return，F5「总结/续写/翻译/润色」输出被静默截断
    let mut out = String::new();
    if let Some(output) = json.get("output").and_then(|o| o.as_array()) {
        for item in output {
            if let Some(content) = item.get("content").and_then(|c| c.as_array()) {
                for part in content {
                    if part.get("type").and_then(|t| t.as_str()) == Some("output_text") {
                        if let Some(text) = part.get("text").and_then(|t| t.as_str()) {
                            out.push_str(text);
                        }
                    }
                }
            }
        }
    }
    if out.is_empty() {
        Err(format!("AI 响应无文本: {}", json))
    } else {
        Ok(out)
    }
}

/// 共享异步结果（tokio spawn + Mutex——每帧轮询）
pub type SharedResult<T> = Arc<Mutex<Option<Result<T, String>>>>;

#[derive(Deserialize, Debug, Clone, Default)]
pub struct TaskInfo {
    pub id: Option<String>,
    pub description: Option<String>,
    #[serde(rename = "type")]
    pub task_type: Option<String>,
    pub priority: Option<i64>,
    pub status: Option<String>,
    pub model: Option<String>,
    pub machine: Option<String>,
    #[serde(rename = "ref_task_id")]
    pub ref_task_id: Option<String>,
    #[serde(rename = "created_at")]
    pub created_at: Option<String>,
    #[serde(rename = "completed_at")]
    pub completed_at: Option<String>,
}

#[derive(Deserialize, Debug, Clone, Default)]
pub struct TaskListResp {
    pub tasks: Option<Vec<TaskInfo>>,
    pub count: Option<i64>,
}

#[derive(Deserialize, Debug, Clone, Default)]
pub struct GitStatusResp {
    pub branches: Option<Vec<String>>,
    pub worktrees: Option<Vec<Value>>,
    pub unmerged: Option<Vec<String>>,
}

/// 启动 tokio runtime（一次性）
/// A18（2026-09-10 审计）：worker_threads 2 → 4（3s/5s/10s 轮询 + 流式 + 文档/AI 请求并发时 2 线程易饥饿）；
/// 且构建失败不再直接 panic 崩 UI——先降级重试并记录，最终才 expect（该路径理论不可达）。
/// 注：签名仍为 &'static Runtime（改 Result 会牵动 app.rs 全部调用点，本次不动调用方签名）。
pub fn runtime() -> &'static tokio::runtime::Runtime {
    static RT: OnceLock<tokio::runtime::Runtime> = OnceLock::new();
    RT.get_or_init(|| {
        let build = |threads: usize| {
            tokio::runtime::Builder::new_multi_thread()
                .worker_threads(threads)
                .thread_name("zerg-ui-rt")
                .enable_all()
                .build()
        };
        match build(4) {
            Ok(rt) => rt,
            Err(e) => {
                eprintln!("[api] tokio runtime(4 线程) 创建失败: {}——降级为 1 线程重试", e);
                match build(1) {
                    Ok(rt) => rt,
                    Err(e2) => {
                        eprintln!("[api] tokio runtime(1 线程) 创建失败: {}", e2);
                        tokio::runtime::Builder::new_multi_thread()
                            .enable_all()
                            .build()
                            .expect("tokio runtime 创建失败（已降级两次，环境异常）")
                    }
                }
            }
        }
    })
}

/// 同步请求主控 API（GET——带 token——在 runtime 内跑）——公开（app 用）
pub async fn sync_get_public(path: &str) -> Result<Value, String> {
    let client = http_client_json();
    let url = format!("{}{}", API_BASE, path);
    let resp = client
        .get(&url)
        .header("X-Auth-Token", API_TOKEN)
        .timeout(std::time::Duration::from_secs(5))
        .send()
        .await
        .map_err(|e| e.to_string())?;
    if !resp.status().is_success() {
        // v2.5.6 错误码设计（2026-08-29）: 解析错误 body 的 error 消息——UI 显示具体原因（如"任务不存在: xxx"）
        // 之前只有 "HTTP 404"——看不到主控/网关返回的具体错误
        // A19（2026-09-10 审计）：抽公共 parse_api_error——两种 error 形态 + 非 JSON 正文统一处理
        let status = resp.status();
        let body = resp.text().await.unwrap_or_default();
        return Err(parse_api_error(status, &body));
    }
    resp.json::<Value>().await.map_err(|e| e.to_string())
}

/// 拉取任务（blocking——app 内 spawn 用）
pub async fn fetch_tasks_blocking() -> Result<Vec<TaskInfo>, String> {
    let v = sync_get_public("/api/tasks").await?;
    let resp: TaskListResp = serde_json::from_value(v).map_err(|e| e.to_string())?;
    Ok(resp.tasks.unwrap_or_default())
}

/// 任务详情（blocking——app 内 spawn 用）
pub async fn fetch_task_detail_blocking(id: String) -> Result<Value, String> {
    let path = format!("/api/tasks/{}", id);
    sync_get_public(&path).await
}

/// Git 状态（blocking）
pub async fn fetch_git_status_blocking() -> Result<GitStatusResp, String> {
    let v = sync_get_public("/api/git/status").await?;
    serde_json::from_value(v).map_err(|e| e.to_string())
}

/// 主控日志（blocking）
pub async fn fetch_logs_blocking() -> Result<Vec<String>, String> {
    let v = sync_get_public("/api/logs/main?limit=100").await?;
    Ok(v.get("lines")
        .and_then(|l| l.as_array())
        .map(|a| {
            a.iter()
                .filter_map(|x| x.as_str().map(|s| s.to_string()))
                .collect()
        })
        .unwrap_or_default())
}

/// 文档目录（blocking）——v2.5.6 返回 files + dirs（Mr2109 2026-08-29: 目录树+文件列表）
pub async fn fetch_docs_blocking() -> Result<(Vec<String>, Vec<String>), String> {
    let v = sync_get_public("/api/docs").await?;
    let files = v
        .get("files")
        .and_then(|f| f.as_array())
        .map(|a| {
            a.iter()
                .filter_map(|x| x.as_str().map(|s| s.to_string()))
                .collect()
        })
        .unwrap_or_default();
    let dirs = v
        .get("dirs")
        .and_then(|f| f.as_array())
        .map(|a| {
            a.iter()
                .filter_map(|x| x.as_str().map(|s| s.to_string()))
                .collect()
        })
        .unwrap_or_default();
    Ok((files, dirs))
}

/// 文档内容（blocking）
pub async fn fetch_doc_content_blocking(path: String) -> Result<String, String> {
    // A12（2026-09-10 审计）：逐段编码路径——文档名含空格/#/?/中文时不再被截断或 400（'/' 保留）
    let p = format!("/api/docs/{}", urlencode_path(&path));
    let v = sync_get_public(&p).await?;
    Ok(v.get("content")
        .and_then(|c| c.as_str().map(|s| s.to_string()))
        .unwrap_or_default())
}

/// 文档操作（blocking）——v2.5.6 右键菜单/编辑器（Mr2109 2026-08-29）
pub async fn doc_op_blocking(action: &str, payload: serde_json::Value) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/docs/{}", API_BASE, action);
    let resp = client
        .post(&url)
        .header("X-Auth-Token", API_TOKEN)
        .json(&payload)
        .timeout(std::time::Duration::from_secs(10))
        .send()
        .await
        .map_err(|e| e.to_string())?;
    if !resp.status().is_success() {
        // A19（2026-09-10 审计）：与 sync_get_public 统一走 parse_api_error——
        // 原实现只认 {"error":"str"}，漏了 {"error":{"type":..,"message":".."}}，错误提示退化成"HTTP 500"
        let status = resp.status();
        let body = resp.text().await.unwrap_or_default();
        return Err(parse_api_error(status, &body));
    }
    Ok(())
}

/// 文档操作（异步——结果交回 UI 轮询；APP-A02 2026-09-10 审计修复）
/// 与 doc_op_blocking 同语义，但不阻塞 UI 线程、不丢结果：UI 拿到 Ok 才改本地状态，Err 红字提示
pub fn doc_op_async(action: &str, payload: serde_json::Value) -> SharedResult<()> {
    let out: SharedResult<()> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    let action = action.to_string();
    runtime().spawn(async move {
        let r = doc_op_blocking(&action, payload).await;
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(r);
    });
    out
}

// task_retry_blocking 重跑任务（右键——failed→queued）
pub async fn task_retry_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/retry", API_BASE, id);
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// task_move_blocking 重排任务（右键——top/bottom/up/down）
pub async fn task_move_blocking(id: &str, action: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/move?action={}", API_BASE, id, action);
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// task_delete_blocking 删除任务（右键——queued 移除）
pub async fn task_delete_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}", API_BASE, id);
    let resp = client.delete(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// task_pause_blocking 暂停/继续任务（右键——queued→paused / paused→queued）
pub async fn task_pause_blocking(id: &str, pause: bool) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/pause?pause={}", API_BASE, id, pause);
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// fetch_internal_tasks_blocking 拉内部任务清单（Mr2109 2026-08-22）
pub async fn fetch_internal_tasks_blocking() -> Result<Vec<serde_json::Value>, String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks", API_BASE);
    let resp = client.get(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    // A05（2026-09-10 审计）：先判 HTTP 状态（json_body），字段缺失返回 Err——不再把 500 错误体当成"空列表"
    let v = json_body(resp).await?;
    v.get("items")
        .cloned()
        .and_then(|a| a.as_array().cloned())
        .ok_or_else(|| "响应缺少 items 字段".to_string())
}

// run_internal_task_blocking 手动执行内部任务（Mr2109 2026-08-22）
pub async fn run_internal_task_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/{}/run", API_BASE, id);
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// stop_internal_tasks_blocking 停止内部任务（Mr2109 2026-08-27——UI 按钮）
pub async fn stop_internal_tasks_blocking() -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/stop", API_BASE);
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// start_internal_tasks_blocking 启动内部任务（Mr2109 2026-08-27——UI 按钮）
pub async fn start_internal_tasks_blocking() -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/start", API_BASE);
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// set_internal_interval_blocking 设置内部任务周期（Mr2109 2026-08-27——循环周期 1h-24h/指定）
pub async fn set_internal_interval_blocking(id: &str, hours: f64) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/{}/interval", API_BASE, id);
    let resp = client
        .post(&url)
        .header("X-Auth-Token", API_TOKEN)
        .json(&serde_json::json!({"hours": hours}))
        .send()
        .await
        .map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// set_internal_mode_blocking 设置内部任务运行模式（Mr2109 2026-08-28——自动/手动开关）
// auto_run=true=自动运行（编排触发）——false=手动运行（只手动触发）
pub async fn set_internal_mode_blocking(id: &str, auto_run: bool) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/{}/mode", API_BASE, id);
    let resp = client
        .post(&url)
        .header("X-Auth-Token", API_TOKEN)
        .json(&serde_json::json!({"auto_run": auto_run}))
        .send()
        .await
        .map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// fetch_internal_intervals_blocking 查询周期配置（Mr2109 2026-08-27）
pub async fn fetch_internal_intervals_blocking() -> Result<serde_json::Value, String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/intervals", API_BASE);
    let resp = client.get(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    // A05（2026-09-10 审计）：先判 HTTP 状态——5xx 错误体不再被当成"成功但空数据"
    json_body(resp).await
}

// task_terminate_blocking 终止执行中任务（右键——running→failed）
pub async fn task_terminate_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/terminate", API_BASE, id);
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// task_requeue_blocking 执行中任务重回队列（右键——running→queued）
pub async fn task_requeue_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/requeue", API_BASE, id);
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// fetch_archive_blocking 拉归档列表（Mr2109 2026-08-22）
pub async fn fetch_archive_blocking() -> Result<Vec<serde_json::Value>, String> {
    let client = http_client_json();
    let url = format!("{}/api/archive", API_BASE);
    let resp = client.get(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    // A05（2026-09-10 审计）：先判 HTTP 状态；entries 字段缺失返回 Err——不再把错误显示成"无数据"
    let v = json_body(resp).await?;
    v.get("entries")
        .cloned()
        .and_then(|a| a.as_array().cloned())
        .ok_or_else(|| "响应缺少 entries 字段".to_string())
}

/// 资源库（blocking）
pub async fn fetch_resources_blocking(res_type: String) -> Result<Value, String> {
    // A20（2026-09-10 审计）：路径段做 URL 编码（复用以 `/` 分段的 urlencode_path）——含 #/?/空格 不再打错 URL
    let path = format!("/api/resources/{}", urlencode_path(&res_type));
    sync_get_public(&path).await
}

/// 集群状态（blocking）
pub async fn fetch_cluster_blocking() -> Result<Value, String> {
    sync_get_public("/api/fleet/status").await
}

// fetch_model_detail_blocking 模型详情（Mr2109 2026-08-27——适配器选项+加载状态）
pub async fn fetch_model_detail_blocking(name: &str) -> Result<Value, String> {
    let client = http_client_json();
    // A20（2026-09-10 审计）：模型名（fleet 键可能含空格/斜杠）编码后入路径
    let url = format!("{}/api/models/{}", API_BASE, urlencode_path(name));
    let resp = client.get(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        resp.json().await.map_err(|e| e.to_string())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// model_start_blocking 启动模型（Mr2109 2026-08-27——UI 开关）
pub async fn model_start_blocking(name: &str) -> Result<(), String> {
    let client = http_client_json();
    // A20（2026-09-10 审计）：模型名编码后入路径
    let url = format!("{}/api/models/{}/start", API_BASE, urlencode_path(name));
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// model_stop_blocking 停止模型（Mr2109 2026-08-27——UI 开关）
pub async fn model_stop_blocking(name: &str) -> Result<(), String> {
    let client = http_client_json();
    // A20（2026-09-10 审计）：模型名编码后入路径
    let url = format!("{}/api/models/{}/stop", API_BASE, urlencode_path(name));
    let resp = client.post(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// fetch_adapter_schema_blocking 适配器参数 schema（Mr2109 2026-08-27——编辑控件渲染）
pub async fn fetch_adapter_schema_blocking(name: &str) -> Result<serde_json::Value, String> {
    let client = http_client_json();
    // A20（2026-09-10 审计）：模型名编码后入路径
    let url = format!("{}/api/models/{}/adapter-opts", API_BASE, urlencode_path(name));
    let resp = client.get(&url).header("X-Auth-Token", API_TOKEN).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        resp.json().await.map_err(|e| e.to_string())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// update_adapter_opts_blocking 更新适配器配置（实时生效）
pub async fn update_adapter_opts_blocking(name: &str, cfg: serde_json::Value) -> Result<(), String> {
    let client = http_client_json();
    // A20（2026-09-10 审计）：模型名编码后入路径
    let url = format!("{}/api/models/{}/adapter-opts", API_BASE, urlencode_path(name));
    let resp = client
        .put(&url)
        .header("X-Auth-Token", API_TOKEN)
        .json(&cfg)
        .send()
        .await
        .map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        let txt = resp.text().await.unwrap_or_default();
        Err(if txt.is_empty() { format!("HTTP 错误") } else { txt })
    }
}

/// 异步拉取任务（tokio spawn——结果存 SharedResult）
pub fn fetch_tasks_async() -> SharedResult<Vec<TaskInfo>> {
    let out: SharedResult<Vec<TaskInfo>> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let v = sync_get_public("/api/tasks").await;
        let result = match v {
            Ok(v) => {
                let resp: Result<TaskListResp, String> =
                    serde_json::from_value(v).map_err(|e| e.to_string());
                match resp {
                    Ok(r) => Ok(r.tasks.unwrap_or_default()),
                    Err(e) => Err(e),
                }
            }
            Err(e) => Err(e),
        };
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(result);
    });
    out
}

/// 异步拉取 git 状态
pub fn fetch_git_status_async() -> SharedResult<GitStatusResp> {
    let out: SharedResult<GitStatusResp> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let v = sync_get_public("/api/git/status").await;
        let result = match v {
            Ok(v) => match serde_json::from_value(v) {
                Ok(r) => Ok(r),
                Err(e) => Err(e.to_string()),
            },
            Err(e) => Err(e),
        };
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(result);
    });
    out
}

/// 异步拉取任务详情（含 trace）
pub fn fetch_task_detail_async(id: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let path = format!("/api/tasks/{}", id);
        let result = sync_get_public(&path).await;
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(result);
    });
    out
}

/// 异步拉取主控日志
pub fn fetch_main_logs_async() -> SharedResult<Vec<String>> {
    let out: SharedResult<Vec<String>> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let v = sync_get_public("/api/logs/main?limit=100").await;
        let result = match v {
            Ok(v) => Ok(v
                .get("lines")
                .and_then(|l| l.as_array())
                .map(|a| {
                    a.iter()
                        .filter_map(|x| x.as_str().map(|s| s.to_string()))
                        .collect()
                })
                .unwrap_or_default()),
            Err(e) => Err(e),
        };
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(result);
    });
    out
}

/// 异步拉取文档目录
pub fn fetch_docs_async() -> SharedResult<Vec<String>> {
    let out: SharedResult<Vec<String>> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let v = sync_get_public("/api/docs").await;
        let result = match v {
            Ok(v) => Ok(v
                .get("files")
                .and_then(|f| f.as_array())
                .map(|a| {
                    a.iter()
                        .filter_map(|x| x.as_str().map(|s| s.to_string()))
                        .collect()
                })
                .unwrap_or_default()),
            Err(e) => Err(e),
        };
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(result);
    });
    out
}

/// 异步拉取资源库（模型）
pub fn fetch_resources_async(res_type: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        // A20（2026-09-10 审计）：路径段编码（与 fetch_resources_blocking 一致）
        let path = format!("/api/resources/{}", urlencode_path(&res_type));
        let result = sync_get_public(&path).await;
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(result);
    });
    out
}

/// 探测主控是否在线
/// A17（2026-09-10 审计）：探针原打 `/api/tasks`（全量任务列表——重，且任务接口单点故障会被误判为"主控离线"）
/// → 改打 `/api/fleet/status`（轻量集群状态，与 fetch_cluster_blocking / 单测同源）。
/// 台账：app.rs 侧曾丢弃本函数句柄并重复 spawn 同一请求（APP-A05 2026-09-10 已删掉那一路），
/// 现本函数**全仓无调用方**；按"不删公共函数"原则保留为兼容入口，故加 #[allow(dead_code)]。
#[allow(dead_code)]
pub fn ping_async() -> SharedResult<bool> {
    let out: SharedResult<bool> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let ok = sync_get_public("/api/fleet/status").await.is_ok();
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(ok));
    });
    out
}

// ═══════════ v2.5.7 对话模块客户端（/api/chat/*——Mr2109借鉴 Hermes）═══════════

/// 会话列表（异步）
pub fn fetch_chat_sessions_async() -> SharedResult<Vec<Value>> {
    let out: SharedResult<Vec<Value>> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let res = sync_get_public("/api/chat/sessions").await;
        let v = match res {
            Ok(v) => v.get("sessions").cloned().unwrap_or(Value::Array(vec![])),
            Err(e) => {
                *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e));
                return;
            }
        };
        let arr = v.as_array().cloned().unwrap_or_default();
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(arr));
    });
    out
}

/// 新建会话（异步）
pub fn create_chat_session_async(model: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions", API_BASE);
        let body = serde_json::json!({"model": model});
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .json(&body)
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

/// 流式消息状态（C3——SSE 逐字——UI 每帧读）
#[derive(Default)]
pub struct ChatStreamState {
    pub content: String,
    pub reasoning: String,
    // P4-35 工具执行状态（tool_start 事件——UI 显示执行中）
    pub tool_name: Option<String>,
    pub tool_count: usize,
    pub compacting: bool,
    pub tool_elapsed: usize,
    pub done: bool,
    pub cancelled: bool,
    pub error: Option<String>,
    pub steer_undrained: Vec<String>, // C2: 未被工具边界消费的插话（客户端排队续发）
}
pub type SharedChatStream = Arc<Mutex<ChatStreamState>>;

/// A09（2026-09-10 审计）：流式任务守卫——任务 panic / 异常提前结束时兜底收敛状态机
/// （JoinHandle 仍由调用方丢弃，但至少保证 done=true + error，UI 不会永久停在"生成中"）
struct StreamDoneGuard(SharedChatStream);
impl Drop for StreamDoneGuard {
    fn drop(&mut self) {
        let mut st = self.0.lock().unwrap_or_else(|e| e.into_inner());
        if !st.done {
            st.done = true;
            if st.error.is_none() {
                st.error = Some("内部错误：流式任务异常结束".to_string());
            }
        }
    }
}

/// 发消息（C3 流式——POST /send——SSE 读取——边收边更新状态）
/// image: 可选单图 data URL（D3）——images: 多图数组（P2——优先）
pub fn chat_send_stream_async(session_id: String, content: String, image: Option<String>, images: Vec<String>, reuse_user_id: Option<i64>) -> SharedChatStream {
    use futures_util::StreamExt;
    let state: SharedChatStream = Arc::new(Mutex::new(ChatStreamState::default()));
    let s2 = state.clone();
    runtime().spawn(async move {
        // A09：任务退出（含 panic）兜底——保证状态机收敛
        let _guard = StreamDoneGuard(s2.clone());
        let client = http_client();
        let url = format!("{}/api/chat/sessions/{}/send", API_BASE, session_id);
        let mut body = serde_json::json!({"content": content});
        // C3(2026-09-10): 重生成——复用既有 user 消息（服务端软删其后消息）
        if let Some(uid) = reuse_user_id {
            body["reuse_user_id"] = serde_json::Value::Number(uid.into());
        }
        if !images.is_empty() {
            body["images"] = serde_json::Value::Array(images.into_iter().map(serde_json::Value::String).collect());
        } else if let Some(img) = image {
            body["image"] = serde_json::Value::String(img);
        }
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .json(&body)
            .send()
            .await;
        match resp {
            Ok(r) => {
                // A02（2026-09-10 审计）：先判 HTTP 状态——否则 4xx/5xx 被当成"空白回复"静默结束，用户看不到任何错误
                if !r.status().is_success() {
                    let code = r.status();
                    let body = r.text().await.unwrap_or_default();
                    let brief: String = body.chars().take(300).collect();
                    let mut st = s2.lock().unwrap_or_else(|e| e.into_inner());
                    st.error = Some(format!("HTTP {}: {}", code, brief));
                    st.done = true;
                    return;
                }
                let mut stream = r.bytes_stream();
                let mut buf: Vec<u8> = Vec::new();
                // A08（2026-09-10 审计）：取消信号与 stream.next() 并列等待——模型思考/工具执行期间长时间无
                // 字节输出时，点 ⏹ 也能立即生效（原实现只在收到下一个 chunk 后才检查 cancelled）
                loop {
                    // 停止检查（用户点 ⏹——断开连接→后端 ctx cancel）
                    if s2.lock().unwrap_or_else(|e| e.into_inner()).cancelled {
                        break;
                    }
                    let stop = tokio::select! {
                        maybe = stream.next() => match maybe {
                            Some(Ok(chunk)) => {
                                buf.extend_from_slice(&chunk);
                                // 按 \n\n 分割 SSE 事件
                                loop {
                                    let text = String::from_utf8_lossy(&buf);
                                    let Some(pos) = text.find("\n\n") else { break };
                                    let event = text[..pos].to_string();
                                    // A06（2026-09-10 审计）：显式解析事件名——不再只靠 contains("event: done") 判断
                                    let ev_name = event
                                        .lines()
                                        .find_map(|l| l.trim_start().strip_prefix("event:").map(|s| s.trim().to_string()));
                                    let mut st = s2.lock().unwrap_or_else(|e| e.into_inner());
                                    // A06（2026-09-10 审计）：收集本事件块内**全部** data: 行（SSE 规范多行 data 以 \n 拼接，
                                    // 前缀 "data:" 后的空格可选）——原实现只取首条 data 行、且强制要求冒号后有空格
                                    let mut payload = String::new();
                                    let mut has_data = false;
                                    for l in event.lines() {
                                        if let Some(rest) = l.trim_start().strip_prefix("data:") {
                                            if has_data {
                                                payload.push('\n');
                                            }
                                            payload.push_str(rest.strip_prefix(' ').unwrap_or(rest));
                                            has_data = true;
                                        }
                                    }
                                    let parsed: Option<Value> = if has_data {
                                        serde_json::from_str::<Value>(&payload).ok()
                                    } else {
                                        None
                                    };
                                    if let Some(v) = &parsed {
                                        if let Some(t) = v.get("type").and_then(|x| x.as_str()) {
                                            let txt = v.get("text").and_then(|x| x.as_str()).unwrap_or("");
                                            match t {
                                                "reasoning" => st.reasoning.push_str(txt),
                                                "output" => st.content.push_str(txt),
                                                "tool_start" => {
                                                    // P4-35 工具执行中状态（name 字段——执行前事件）
                                                    st.tool_name = v.get("name").and_then(|x| x.as_str()).map(|s| s.to_string());
                                                }
                                                "compacting" => {
                                                    // P4-39 T5: 上下文压缩进行中（Hermes "compacting" 状态）
                                                    st.compacting = true;
                                                }
                                                "compact_done" => {
                                                    st.compacting = false;
                                                }
                                                "tool" => {
                                                    // P4-35 工具执行完成——清除执行中状态（结果事件）
                                                    st.tool_name = None;
                                                }
                                                "tool_ping" => {
                                                    // P4-36 工具执行心跳——计时（UI 显示"执行中 N 秒"）
                                                    st.tool_elapsed = v.get("elapsed").and_then(|x| x.as_u64()).unwrap_or(0) as usize;
                                                }
                                                _ => {}
                                            }
                                        }
                                        if let Some(e) = v.get("error").and_then(|x| x.as_str()) {
                                            st.error = Some(e.to_string());
                                        }
                                    }
                                    if ev_name.as_deref() == Some("done") {
                                        st.done = true;
                                        // C2: 回收未消费插话（交队列续发）
                                        if let Some(v) = &parsed {
                                            if let Some(arr) = v.get("steer_undrained").and_then(|x| x.as_array()) {
                                                st.steer_undrained = arr
                                                    .iter()
                                                    .filter_map(|x| x.as_str().map(|s| s.to_string()))
                                                    .collect();
                                            }
                                        }
                                    }
                                    drop(st);
                                    buf.drain(..pos + 2);
                                }
                                false
                            }
                            // A07（2026-09-10 审计）：断流不再静默 break——写 error，用户可与正常结束区分
                            Some(Err(e)) => {
                                s2.lock().unwrap_or_else(|e| e.into_inner()).error = Some(format!("流中断: {}", e));
                                true
                            }
                            None => true,
                        },
                        // 150ms 心跳：无字节输出时也回来重查取消标志
                        _ = tokio::time::sleep(std::time::Duration::from_millis(150)) => false,
                    };
                    if stop {
                        break;
                    }
                }
                // 流结束（正常或取消）
                s2.lock().unwrap_or_else(|e| e.into_inner()).done = true;
            }
            Err(e) => {
                s2.lock().unwrap_or_else(|e| e.into_inner()).error = Some(format!("请求失败: {}", e));
                s2.lock().unwrap_or_else(|e| e.into_inner()).done = true;
            }
        }
    });
    state
}

/// 会话详情（含消息——异步）
pub fn fetch_chat_session_async(session_id: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let res = sync_get_public(&format!("/api/chat/sessions/{}", session_id)).await;
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(res);
    });
    out
}

/// 删会话（异步）
pub fn delete_chat_session_async(session_id: String) -> SharedResult<bool> {
    let out: SharedResult<bool> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}", API_BASE, session_id);
        let resp = client
            .delete(&url)
            .header("X-Auth-Token", API_TOKEN)
            .send()
            .await;
        match resp {
            Ok(r) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(r.status().is_success())),
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

/// 搜索对话（异步）
pub fn search_chat_async(query: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let path = format!("/api/chat/search?q={}", urlencode(&query));
        let res = sync_get_public(&path).await;
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(res);
    });
    out
}

/// URL 编码（搜索参数）
fn urlencode(s: &str) -> String {
    s.chars()
        .map(|c| match c {
            'a'..='z' | 'A'..='Z' | '0'..='9' | '-' | '_' | '.' | '~' => c.to_string(),
            ' ' => "%20".to_string(),
            _ => {
                let mut b = [0u8; 4];
                let bytes = c.encode_utf8(&mut b).as_bytes();
                bytes.iter().map(|&x| format!("%{:02X}", x)).collect()
            }
        })
        .collect()
}

/// 路径编码（A12 2026-09-10 审计）：按 `/` 分段各自 urlencode——保留路径分隔符，其余字符转义
fn urlencode_path(path: &str) -> String {
    path.split('/').map(urlencode).collect::<Vec<_>>().join("/")
}

/// 可用模型列表（C5 模型胶囊——GET /api/fleet/models 提取 name）
pub fn fetch_available_models_async() -> SharedResult<Vec<String>> {
    let out: SharedResult<Vec<String>> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let res = sync_get_public("/api/fleet/models").await;
        match res {
            Ok(v) => {
                let mut names: Vec<String> = Vec::new();
                let arr = if let Some(a) = v.as_array() {
                    a.clone()
                } else if let Some(ms) = v.get("models").and_then(|x| x.as_array()) {
                    ms.clone()
                } else {
                    vec![]
                };
                for m in arr {
                    // P4-33 fleet/models 字段是 id 非 name（ormith-1.0-35b local/x3 双条——去重）
                    if let Some(n) = m
                        .get("name")
                        .and_then(|x| x.as_str())
                        .or_else(|| m.get("id").and_then(|x| x.as_str()))
                    {
                        if !names.contains(&n.to_string()) {
                            names.push(n.to_string());
                        }
                    }
                }
                names.sort();
                names.dedup();
                *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(names));
            }
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
        }
    });
    out
}

/// 切换会话模型（C5——POST /api/chat/sessions/{id}/model）
pub fn chat_update_model_async(session_id: String, model: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}/model", API_BASE, session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .json(&serde_json::json!({"model": model}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

/// 服务端中断（批次B 2026-09-10——POST /api/chat/sessions/{id}/abort）
/// 语义: 取消运行中的轮次——已流出部分由后端落库（"（已中断）"）
pub fn chat_abort_async(session_id: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}/abort", API_BASE, session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

/// 插话 steer（批次C2 2026-09-10——POST /api/chat/sessions/{id}/steer）
/// 语义: 生成中纯文本 → 挂下一次工具边界（不打断）；返回 turn_running=false 时调用方应改为排队
pub fn chat_steer_async(session_id: String, content: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}/steer", API_BASE, session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .json(&serde_json::json!({"content": content}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

/// 重生成（批次C3 2026-09-10——POST /api/chat/sessions/{id}/regenerate）
/// 语义: 软删末条 user 之后的消息 → 返回 (user_message_id, content) 供客户端重跑
pub fn chat_regenerate_async(session_id: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}/regenerate", API_BASE, session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

/// 会话搜索（C6——GET /api/chat/search?q=）
pub fn chat_search_async(q: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        // A11（2026-09-10 审计）：查询串需 URL 编码——`&`/`#`/`+`/中文会破坏 URL（与 search_chat_async 保持一致）
        let res = sync_get_public(&format!("/api/chat/search?q={}", urlencode(&q))).await;
        match res {
            Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
        }
    });
    out
}

/// 对话→任务派单（C7——POST /api/tasks——parent 关联）
/// 派单到任务队列（C7——POST /api/tasks——v2.5.7 带 parent_session_id 关联来源对话）
pub fn chat_delegate_task_async(description: String, model: String, parent_session_id: Option<String>) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/tasks", API_BASE);
        let mut body = serde_json::json!({"description": description, "model": model, "type": "external", "priority": 3});
        if let Some(psid) = parent_session_id {
            body["parent_session_id"] = serde_json::Value::String(psid);
        }
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .json(&body)
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

/// 会话固定/取消（D4——POST /api/chat/sessions/{id}/pinned）
pub fn chat_set_archived_async(session_id: String, archived: bool) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}/archive", API_BASE, session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .json(&serde_json::json!({"archived": archived}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

pub fn chat_set_pinned_async(session_id: String, pinned: bool) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}/pinned", API_BASE, session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .json(&serde_json::json!({"pinned": pinned}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

/// P4-10 重命名会话（POST /api/chat/sessions/{id}/title）
pub fn chat_rename_session_async(session_id: String, title: String) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}/title", API_BASE, session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", API_TOKEN)
            .json(&serde_json::json!({"title": title}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

/// 编辑用户消息（P0——PATCH /api/chat/messages/{mid}——Hermes user-edit 借鉴）
pub fn chat_edit_message_async(mid: i64, content: String, truncate: bool) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/messages/{}", API_BASE, mid);
        let resp = client
            .patch(&url)
            .header("X-Auth-Token", API_TOKEN)
            .json(&serde_json::json!({"content": content, "truncate": truncate}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(format!("请求失败: {}", e))),
        }
    });
    out
}

#[cfg(test)]
mod api_error_parse_tests {
    use super::*;

    /// A19（2026-09-10 审计）：错误体统一解析——两种 error 形态 + 非 JSON 正文：
    /// `{"error":"msg"}` / `{"error":{"type":..,"message":"msg"}}` / 网关 HTML / 空 body
    #[test]
    fn parse_api_error_handles_all_shapes() {
        let s = reqwest::StatusCode::BAD_REQUEST;
        let a = parse_api_error(s, r#"{"error":"任务不存在: abc"}"#);
        assert!(a.contains("任务不存在: abc"), "形态一（error 字符串）未解析: {a}");
        let b = parse_api_error(s, r#"{"error":{"type":"not_found","message":"会话不存在: 42"}}"#);
        assert!(b.contains("会话不存在: 42"), "形态二（error.message）未解析: {b}");
        let c = parse_api_error(reqwest::StatusCode::BAD_GATEWAY, "<html>502 Bad Gateway</html>");
        assert!(c.contains("502 Bad Gateway"), "非 JSON 正文未回显: {c}");
        let d = parse_api_error(reqwest::StatusCode::BAD_GATEWAY, "");
        assert_eq!(d, "HTTP 502 Bad Gateway", "空 body 格式异常: {d}");
    }
}

#[cfg(test)]
mod proxy_root_fix_tests {
    use super::*;

    /// 环境变量守卫（A13 2026-09-10 审计）：测试结束恢复原值——不再把 4 个代理变量永久留在进程环境里污染其它测试
    struct EnvGuard(Vec<(&'static str, Option<String>)>);
    impl EnvGuard {
        fn set(vars: &[(&'static str, &str)]) -> Self {
            let saved = vars.iter().map(|(k, _)| (*k, std::env::var(k).ok())).collect();
            for (k, v) in vars {
                std::env::set_var(k, v);
            }
            EnvGuard(saved)
        }
    }
    impl Drop for EnvGuard {
        fn drop(&mut self) {
            for (k, v) in &self.0 {
                match v {
                    Some(val) => std::env::set_var(k, val),
                    None => std::env::remove_var(k),
                }
            }
        }
    }

    /// 根治验证：设置一个"死代理"环境变量后——
    /// 统一 client（no_proxy）必须仍能打通回环主控；裸 Client::new() 则应失败（复现旧 bug）。
    /// 主控未运行时跳过（避免 CI/离线环境误报）。
    #[test]
    fn no_proxy_client_reaches_loopback() {
        let _env = EnvGuard::set(&[
            ("HTTP_PROXY", "http://127.0.0.1:9"),
            ("HTTPS_PROXY", "http://127.0.0.1:9"),
            ("http_proxy", "http://127.0.0.1:9"),
            ("https_proxy", "http://127.0.0.1:9"),
        ]);

        let rt = match tokio::runtime::Runtime::new() {
            Ok(rt) => rt,
            Err(_) => return,
        };
        // A13（2026-09-10 审计）：Err(_) → None（"连不通"），与"状态码非 2xx"（Some(false)）区分——
        // 主控离线时探测结果为 None → 走下面的跳过分支，不再误报"根治失败"
        let probe = |client: reqwest::Client| {
            rt.block_on(async move {
                match client
                    .get(format!("{}/api/fleet/status", API_BASE))
                    .header("X-Auth-Token", API_TOKEN)
                    .send()
                    .await
                {
                    Ok(r) => Some(r.status().is_success()),
                    Err(_) => None,
                }
            })
        };

        let ours = probe(http_client());
        match ours {
            None => {
                eprintln!("跳过：主控未运行，无法验证回环可达性");
                return;
            }
            Some(false) => panic!("no_proxy client 未能打通回环（根治失败）"),
            Some(true) => {}
        }
        // 对照：裸 client 在死代理下**必须连不通**（None）——这正是被根治的旧 bug
        // （A13 的 probe 语义：None=连不通 / Some(bool)=拿到 HTTP 状态；旧断言按 Some(false) 写，已过时）
        let bare = probe(reqwest::Client::new());
        eprintln!("no_proxy client 成功=true; 裸 client 结果={:?}（None=被死代理吞掉）", bare);
        assert_eq!(
            bare, None,
            "对照失效：裸 client 竟然连上了 127.0.0.1（环境变量未生效，或 reqwest 已不再走代理）"
        );
    }
}
