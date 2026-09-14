// 主控 API 客户端（8580——X-Auth-Token）
// 异步: tokio runtime（egui-async 在 eframe 0.36 下有帧号 bug——自己管理）
use serde::Deserialize;
use serde_json::Value;
use rust_i18n::t;   // i18n（B3 抽取：错误文案走键）
use std::sync::{Arc, Mutex, OnceLock};

/// 主控 API 基址（2026-09-11 B 批：环境无关化——ZERG_api_base() 可覆盖，默认本机 8580）
pub fn api_base() -> &'static str {
    static BASE: OnceLock<String> = OnceLock::new();
    BASE.get_or_init(|| {
        std::env::var("ZERG_api_base()")
            .ok()
            .map(|v| v.trim().to_string())
            .filter(|v| !v.is_empty())
            .unwrap_or_else(|| "http://127.0.0.1:8580".to_string())
    })
    .as_str()
}
/// 共享令牌（2026-09-11 A 批：不再硬编码——库内零明文）
/// 解析顺序：环境变量 ZERG_AUTH_TOKEN → ZERG_API_TOKEN → ~/.zerg-ui-prefs.json 的 auth_token → ~/.zerg/token
pub fn api_token() -> &'static str {
    static TOKEN: OnceLock<String> = OnceLock::new();
    TOKEN
        .get_or_init(|| {
            for k in ["ZERG_AUTH_TOKEN", "ZERG_api_token()"] {
                if let Ok(v) = std::env::var(k) {
                    let v = v.trim().to_string();
                    if !v.is_empty() {
                        return v;
                    }
                }
            }
            if let Some(home) = std::env::var_os("HOME") {
                let home = std::path::PathBuf::from(home);
                // UI 偏好文件（用户可在其中持久化令牌）——2026-09-13：新落点优先、旧落点兼容
                if let Some(txt) = read_prefs() {
                    if let Ok(v) = serde_json::from_str::<serde_json::Value>(&txt) {
                        if let Some(t) = v.get("auth_token").and_then(|x| x.as_str()) {
                            let t = t.trim();
                            if !t.is_empty() {
                                return t.to_string();
                            }
                        }
                    }
                }
                // 共享令牌文件（与主控/子端同源）
                if let Ok(txt) = std::fs::read_to_string(home.join(".zerg/token")) {
                    let t = txt.trim();
                    if !t.is_empty() {
                        return t.to_string();
                    }
                }
            }
            eprintln!("[api] no shared token configured (ZERG_AUTH_TOKEN or ~/.zerg/token) — the controller API will return 401");
            String::new()
        })
        .as_str()
}
/// 网关（AI 动力）基址（ZERG_ai_base() 可覆盖，默认本机 8082）
pub fn ai_base() -> &'static str {
    static BASE: OnceLock<String> = OnceLock::new();
    BASE.get_or_init(|| {
        std::env::var("ZERG_ai_base()")
            .ok()
            .map(|v| v.trim().to_string())
            .filter(|v| !v.is_empty())
            .unwrap_or_else(|| "http://127.0.0.1:8082".to_string())
    })
    .as_str()
}

/// http_client — 统一构造 reqwest client（2026-09-10 根治回环代理问题）
/// 背景：reqwest 默认尊重系统代理（macOS Clash Party :7895），对 **127.0.0.1 回环请求**也会走代理，
/// 结果是"主控离线"（系统 UI/URLSession 会自动绕过回环，故只有本进程中招）；此前靠 start-zerg-ui.sh 剥代理治标。
/// 本 UI 的全部请求都指向回环（8580 主控 / 8082 网关），故一律 no_proxy —— 不再依赖启动脚本。
/// A15（2026-09-10 审计）：三档 client 各用 OnceLock 缓存单例——reqwest::Client 内含连接池、本应长期复用；
/// 原实现每次请求都 build() 一个新的 → 每请求一条新 TCP、keep-alive 全废（与 3s/5s/10s 轮询叠加）。
/// 返回 clone()（Client 内部是 Arc，浅拷贝共享连接池），调用方签名与用法不变。
static CLIENT_DEFAULT: OnceLock<reqwest::Client> = OnceLock::new();
static CLIENT_JSON: OnceLock<reqwest::Client> = OnceLock::new();
// 2026-09-13（C9 第 4 步）：F5 AI 动力（总结/续写/翻译/润色）的唯一消费者是**旧文档界面**——
// 它已整块迁进文档茧，本构建里这三件（client/工厂/调用）暂无消费者。**保留**：AI 网关调用是
// **宿主能力**（茧侧按口径走宿主网关，只是本茧还没接），不删能力；标 dead_code 免得噪声掩盖真问题。
// （下一批：给对话/任务接上，或随文档茧的 AI 按钮一起回来。）
#[allow(dead_code)]
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
/// （见上：C9 第 4 步后宿主内暂无消费者，保留为宿主能力面）
#[allow(dead_code)]
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
/// 供 sync_get_public / json_body 共用——原缺 message 分支时错误提示会退化成"HTTP 500"
/// （2026-09-13 C9 第 4 步：原第三个消费者 `doc_op_blocking` 已随文档写端点迁进文档茧）。
fn parse_api_error(status: reqwest::StatusCode, body: &str) -> String {
    if let Ok(v) = serde_json::from_str::<Value>(body) {
        if let Some(e) = v.get("error") {
            if let Some(s) = e.as_str() {
                return format!("HTTP {}: {}", status, s);
            }
            // 多语言 L4（2026-09-11）：优先按服务端错误码渲染本地化文案
            // （服务端 message 恒为中文——双写期；UI 按 code 换成当前语言）
            if let Some(c) = e.get("type").and_then(|c| c.as_str()) {
                if let Some(msg) = localized_api_error(c) {
                    return format!("HTTP {}: {}", status, msg);
                }
            }
            // 未收录的 code → 回显服务端 message（客户端不改即可运行；新增 code 不必同步发 UI）
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

/// localized_api_error（多语言 L4）：错误码 → 当前语言的文案。
/// 键名规则：`apierr.<code 小写>`（如 MISSING_MACHINE → apierr.missing_machine）。
/// rust-i18n 对**未定义键原样返回键名**——据此判定命中：返回 None 时调用方回退服务端 message。
pub(crate) fn localized_api_error(code: &str) -> Option<String> {
    let key = format!("apierr.{}", code.to_ascii_lowercase());
    let v = rust_i18n::t!(key.as_str()).to_string();
    if v == key {
        None
    } else {
        Some(v)
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
    serde_json::from_str::<Value>(&text).map_err(|e| t!("err.parse_failed", err = e).to_string())
}

/// F5 AI 调用（网关 8082 /v1/responses——OpenAI responses 格式）
/// 解析 output 里的 output_text 文本（跳过 reasoning）
/// （见上：C9 第 4 步后宿主内暂无消费者——旧文档界面的四个 AI 按钮随界面迁出，保留为宿主能力面）
#[allow(dead_code)]
pub async fn ai_prompt_blocking(model: &str, prompt: &str) -> Result<String, String> {
    let client = http_client_ai();
    let url = format!("{}/v1/responses", ai_base());
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
        .header("X-Auth-Token", api_token())
        .json(&body)
        .send()
        .await
        .map_err(|e| t!("err.ai_request", err = e).to_string())?;
    let status = resp.status();
    // A16（2026-09-10 审计）：**先判状态、再解析 JSON**——错误体可能是网关 502 HTML / 空 body，
    // 原实现先 resp.json() 会把真实状态码吞掉、只报"AI 响应解析失败"。
    let text = resp
        .text()
        .await
        .map_err(|e| t!("err.ai_read", err = e).to_string())?;
    if !status.is_success() {
        let brief: String = text.chars().take(200).collect();
        return Err(t!(
            "err.ai_status",
            status = status,
            brief = if brief.is_empty() { t!("common.empty_response").to_string() } else { brief }
        ).to_string());
    }
    let json: Value =
        serde_json::from_str(&text).map_err(|e| t!("err.ai_parse", err = e).to_string())?;
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
        Err(t!("err.ai_no_text", json = json).to_string())
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
                eprintln!("[api] failed to create the tokio runtime (4 threads): {} — retrying with 1 thread", e);
                match build(1) {
                    Ok(rt) => rt,
                    Err(e2) => {
                        eprintln!("[api] failed to create the tokio runtime (1 thread): {}", e2);
                        tokio::runtime::Builder::new_multi_thread()
                            .enable_all()
                            .build()
                            .expect("failed to create the tokio runtime (downgraded twice — abnormal environment)")
                    }
                }
            }
        }
    })
}

/// 同步请求主控 API（GET——带 token——在 runtime 内跑）——公开（app 用）
pub async fn sync_get_public(path: &str) -> Result<Value, String> {
    let client = http_client_json();
    let url = format!("{}{}", api_base(), path);
    let resp = client
        .get(&url)
        .header("X-Auth-Token", api_token())
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
/// ⚠ 2026-09-13（C9 第 4 步）：**无 root 参数的「文档目录」函数已删**——旧文档界面随文档茧迁出；
/// 宿主只剩**通用**文件浏览（见下方 `fetch_docs_root_blocking`：始终按白名单根取，root=None 亦走
/// `/api/docs` 的缺省根，与文件浏览器同一条路）。
/// 列表响应解析（files + dirs）——文件浏览器「按根列表」与老端点共用同一份解析
/// （避免两处各写一遍 filter_map，语义一旦分叉就会出现「老端点有目录、新端点没有」的诡异差异）
fn docs_listing(v: &Value) -> (Vec<String>, Vec<String>) {
    let pick = |key: &str| -> Vec<String> {
        v.get(key)
            .and_then(|f| f.as_array())
            .map(|a| {
                a.iter()
                    .filter_map(|x| x.as_str().map(|s| s.to_string()))
                    .collect()
            })
            .unwrap_or_default()
    };
    (pick("files"), pick("dirs"))
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

// ─────────────────────────────────────────────────────────────────────────────
// 文件浏览器阶段 1（2026-09-13 设计「文件浏览器虫茧」§4.2）：根集合 + 按根读取
// 说明：GET 侧在此（本模块可复用私有的 urlencode 做查询串转义）；
//      POST 侧（open/reveal）在 ui/src/modules/filebrowse/actions.rs（要按错误码映射文案）。
// ─────────────────────────────────────────────────────────────────────────────

/// 根集合 + 界面显示配置（GET /api/fileroots）
/// 200 → {"roots":[{"id","label","path","default","writable"}...],
///        "config":{"display_max":1048576,"allow_all_types":false,"text_exts":[".md",...]}}
pub async fn fetch_fileroots_blocking() -> Result<Value, String> {
    sync_get_public("/api/fileroots").await
}

/// 按根列目录（GET /api/docs?root=<id>）
/// root=None ⇒ 不带 root 参数 ⇒ **与改造前的 /api/docs 逐字节等价**（§4.5 老行为不变）
pub async fn fetch_docs_root_blocking(root: Option<&str>) -> Result<(Vec<String>, Vec<String>), String> {
    let path = match root {
        Some(r) if !r.is_empty() => format!("/api/docs?root={}", urlencode(r)),
        _ => "/api/docs".to_string(),
    };
    let v = sync_get_public(&path).await?;
    Ok(docs_listing(&v))
}

/// 按根读文件（GET /api/docs?root=<id>&path=<rel>）→（内容, 后端报的字节数）
/// **读取不设上限**（§4.4 拍板：文本整份取回）——这里不做任何截断，截断只发生在界面渲染。
pub async fn fetch_doc_content_in_root_blocking(root: &str, path: &str) -> Result<(String, Option<u64>), String> {
    let p = format!("/api/docs?root={}&path={}", urlencode(root), urlencode(path));
    let v = sync_get_public(&p).await?;
    let content = v
        .get("content")
        .and_then(|c| c.as_str())
        .unwrap_or_default()
        .to_string();
    let size = v.get("size").and_then(|s| s.as_u64());
    Ok((content, size))
}

/// 按根读文件（统一入口）——文档模块与文件浏览器共用：
/// root 为空或 "docs" 走**老端点** `/api/docs/<rel>`（§4.5 兼容），其余根走参数化端点。
pub async fn fetch_doc_content_any_root_blocking(root: &str, path: String) -> Result<String, String> {
    if root.is_empty() || root == "docs" {
        fetch_doc_content_blocking(path).await
    } else {
        fetch_doc_content_in_root_blocking(root, &path)
            .await
            .map(|(c, _size)| c)
    }
}

// ── 文档操作客户端（`doc_op_blocking` / `doc_op_async`）2026-09-13（C9 第 4 步）已删 ──
// 原因：文档**写**后端（`/api/docs/{mkdir,rename,delete,copy,save}`）随文档茧整块迁出，
// 客户端实现由茧自带（`zerg-cocoon/文档` 的 `src/api.rs`）。宿主只剩**通用**文件浏览：
//   · 根清单/打开/显示 → `/api/fileroots*`（POST 侧在 modules/filebrowse/actions.rs）
//   · 读（含 docs 根）  → `/api/docs?root=&path=`（含老路径 `/api/docs/<rel>`，见上方）

// task_retry_blocking 重跑任务（右键——failed→queued）
pub async fn task_retry_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/retry", api_base(), id);
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// task_move_blocking 重排任务（右键——top/bottom/up/down）
pub async fn task_move_blocking(id: &str, action: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/move?action={}", api_base(), id, action);
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// task_delete_blocking 删除任务（右键——queued 移除）
pub async fn task_delete_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}", api_base(), id);
    let resp = client.delete(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// task_pause_blocking 暂停/继续任务（右键——queued→paused / paused→queued）
pub async fn task_pause_blocking(id: &str, pause: bool) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/pause?pause={}", api_base(), id, pause);
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// fetch_internal_tasks_blocking 拉内部任务清单（Mr2109 2026-08-22）
pub async fn fetch_internal_tasks_blocking() -> Result<Vec<serde_json::Value>, String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks", api_base());
    let resp = client.get(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    // A05（2026-09-10 审计）：先判 HTTP 状态（json_body），字段缺失返回 Err——不再把 500 错误体当成"空列表"
    let v = json_body(resp).await?;
    v.get("items")
        .cloned()
        .and_then(|a| a.as_array().cloned())
        .ok_or_else(|| t!("err.missing_items").to_string())
}

// run_internal_task_blocking 手动执行内部任务（Mr2109 2026-08-22）
pub async fn run_internal_task_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/{}/run", api_base(), id);
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        Err(format!("HTTP {}", resp.status()))
    }
}

// stop_internal_tasks_blocking 停止内部任务（Mr2109 2026-08-27——UI 按钮）
/// 停止内部任务（Mr2109 2026-08-27——UI 按钮）
/// 2026-09-10 治本（APP-A07）：返回响应体（内含 state）——契约"变更即回状态"，UI 无需二次往返
pub async fn stop_internal_tasks_blocking() -> Result<serde_json::Value, String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/stop", api_base());
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    json_body(resp).await
}

// start_internal_tasks_blocking 启动内部任务（Mr2109 2026-08-27——UI 按钮）
/// 启动内部任务（Mr2109 2026-08-27——UI 按钮）
/// 2026-09-10 治本（APP-A07）：门控未放行时后端返回 403 + 原因——这里如实报错（不再假成功）
pub async fn start_internal_tasks_blocking() -> Result<serde_json::Value, String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/start", api_base());
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    json_body(resp).await
}

/// 内部任务引擎状态（2026-09-10 治本——UI 每 10s 轮询；服务端为唯一真相源）
pub async fn fetch_internal_state_blocking() -> Result<serde_json::Value, String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/state", api_base());
    let resp = client.get(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    json_body(resp).await
}

// set_internal_interval_blocking 设置内部任务周期（Mr2109 2026-08-27——循环周期 1h-24h/指定）
pub async fn set_internal_interval_blocking(id: &str, hours: f64) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/internal-tasks/{}/interval", api_base(), id);
    let resp = client
        .post(&url)
        .header("X-Auth-Token", api_token())
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
    let url = format!("{}/api/internal-tasks/{}/mode", api_base(), id);
    let resp = client
        .post(&url)
        .header("X-Auth-Token", api_token())
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
    let url = format!("{}/api/internal-tasks/intervals", api_base());
    let resp = client.get(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    // A05（2026-09-10 审计）：先判 HTTP 状态——5xx 错误体不再被当成"成功但空数据"
    json_body(resp).await
}

// task_terminate_blocking 终止执行中任务（右键——running→failed）
pub async fn task_terminate_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/terminate", api_base(), id);
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// task_requeue_blocking 执行中任务重回队列（右键——running→queued）
pub async fn task_requeue_blocking(id: &str) -> Result<(), String> {
    let client = http_client_json();
    let url = format!("{}/api/tasks/{}/requeue", api_base(), id);
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    if resp.status().is_success() { Ok(()) } else { Err(format!("HTTP {}", resp.status())) }
}

// fetch_archive_blocking 拉归档列表（Mr2109 2026-08-22）
pub async fn fetch_archive_blocking() -> Result<Vec<serde_json::Value>, String> {
    let client = http_client_json();
    let url = format!("{}/api/archive", api_base());
    let resp = client.get(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    // A05（2026-09-10 审计）：先判 HTTP 状态；entries 字段缺失返回 Err——不再把错误显示成"无数据"
    let v = json_body(resp).await?;
    v.get("entries")
        .cloned()
        .and_then(|a| a.as_array().cloned())
        .ok_or_else(|| t!("err.missing_entries").to_string())
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
    let url = format!("{}/api/models/{}", api_base(), urlencode_path(name));
    let resp = client.get(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
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
    let url = format!("{}/api/models/{}/start", api_base(), urlencode_path(name));
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
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
    let url = format!("{}/api/models/{}/stop", api_base(), urlencode_path(name));
    let resp = client.post(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
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
    let url = format!("{}/api/models/{}/adapter-opts", api_base(), urlencode_path(name));
    let resp = client.get(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
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
    let url = format!("{}/api/models/{}/adapter-opts", api_base(), urlencode_path(name));
    let resp = client
        .put(&url)
        .header("X-Auth-Token", api_token())
        .json(&cfg)
        .send()
        .await
        .map_err(|e| e.to_string())?;
    if resp.status().is_success() {
        Ok(())
    } else {
        let txt = resp.text().await.unwrap_or_default();
        Err(if txt.is_empty() { t!("err.http").to_string() } else { txt })
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

/// 模型登记库快照（GET /api/models/registry——异步，不阻塞 UI 线程）
/// 复用 sync_get_public（自动带 X-Auth-Token、走 http_client_json、错误正文经 parse_api_error）
pub fn fetch_model_registry_async() -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let result = sync_get_public("/api/models/registry").await;
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
        let url = format!("{}/api/chat/sessions", api_base());
        let body = serde_json::json!({"model": model});
        let resp = client
            .post(&url)
            .header("X-Auth-Token", api_token())
            .json(&body)
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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
    // A09/A08 备选（2026-09-10）：流式任务句柄——join 用于回读 panic 原因；abort 句柄可克隆，
    // 供停止时立即断流（不与 join 的所有权冲突：观察者拿走 join，停止仍能 abort）
    pub join: Option<tokio::task::JoinHandle<()>>,
    pub abort: Option<tokio::task::AbortHandle>,
}
pub type SharedChatStream = Arc<Mutex<ChatStreamState>>;

/// A09 备选（2026-09-10）：流式任务 panic 上报槽——仅 panic 记录（正常结束/用户停止不记）
static STREAM_PANIC: Mutex<Option<String>> = Mutex::new(None);

/// 取走最近一次流式任务 panic 信息（UI 每帧调用——取走即清空）
pub fn take_stream_panic() -> Option<String> {
    STREAM_PANIC.lock().unwrap_or_else(|e| e.into_inner()).take()
}

/// A08 备选（2026-09-10）：主动取消流式任务——abort 立即丢弃 HTTP 响应流 → 连接断开 →
/// 服务端据此取消生成（与 /api/chat/.../abort 端点构成双保险）；Drop 守卫会把状态机收敛为 done。
pub fn chat_stream_abort(state: &SharedChatStream) {
    let h = state.lock().unwrap_or_else(|e| e.into_inner()).abort.take();
    if let Some(h) = h {
        h.abort();
    }
}

/// A09（2026-09-10 审计）：流式任务守卫——任务 panic / 异常提前结束时兜底收敛状态机
/// （JoinHandle 仍由调用方丢弃，但至少保证 done=true + error，UI 不会永久停在"生成中"）
struct StreamDoneGuard(SharedChatStream);
impl Drop for StreamDoneGuard {
    fn drop(&mut self) {
        let mut st = self.0.lock().unwrap_or_else(|e| e.into_inner());
        if !st.done {
            st.done = true;
            // A08 备选：用户点停止（cancelled）后守卫兜底不再报“内部错误”——UI 已有“已停止生成”
            if st.error.is_none() && !st.cancelled {
                st.error = Some(t!("err.stream_internal_end").to_string());
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
    let join_handle = runtime().spawn(async move {
        // A09：任务退出（含 panic）兜底——保证状态机收敛
        let _guard = StreamDoneGuard(s2.clone());
        let client = http_client();
        let url = format!("{}/api/chat/sessions/{}/send", api_base(), session_id);
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
            .header("X-Auth-Token", api_token())
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
                                s2.lock().unwrap_or_else(|e| e.into_inner()).error = Some(t!("err.stream_broken", err = e).to_string());
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
                s2.lock().unwrap_or_else(|e| e.into_inner()).error = Some(t!("err.request", err = e).to_string());
                s2.lock().unwrap_or_else(|e| e.into_inner()).done = true;
            }
        }
    });
    // A09 备选（2026-09-10）：句柄入状态（原实现裸丢弃）——停止时可 abort；panic 时原因可回读
    {
        let mut st = state.lock().unwrap_or_else(|e| e.into_inner());
        st.abort = Some(join_handle.abort_handle());
        st.join = Some(join_handle);
    }
    // A09 备选：观察者任务——join 流式任务，仅 panic 记入上报槽（正常结束/被 abort 不记），UI 每帧取走
    let obs = state.clone();
    runtime().spawn(async move {
        let h = obs.lock().unwrap_or_else(|e| e.into_inner()).join.take();
        if let Some(h) = h {
            if let Err(e) = h.await {
                if e.is_panic() {
                    *STREAM_PANIC.lock().unwrap_or_else(|e| e.into_inner()) = Some(format!("{}", e));
                }
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
        let url = format!("{}/api/chat/sessions/{}", api_base(), session_id);
        let resp = client
            .delete(&url)
            .header("X-Auth-Token", api_token())
            .send()
            .await;
        match resp {
            Ok(r) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(r.status().is_success())),
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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
        let url = format!("{}/api/chat/sessions/{}/model", api_base(), session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", api_token())
            .json(&serde_json::json!({"model": model}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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
        let url = format!("{}/api/chat/sessions/{}/abort", api_base(), session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", api_token())
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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
        let url = format!("{}/api/chat/sessions/{}/steer", api_base(), session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", api_token())
            .json(&serde_json::json!({"content": content}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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
        let url = format!("{}/api/chat/sessions/{}/regenerate", api_base(), session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", api_token())
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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
        let url = format!("{}/api/tasks", api_base());
        let mut body = serde_json::json!({"description": description, "model": model, "type": "external", "priority": 3});
        if let Some(psid) = parent_session_id {
            body["parent_session_id"] = serde_json::Value::String(psid);
        }
        let resp = client
            .post(&url)
            .header("X-Auth-Token", api_token())
            .json(&body)
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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
        let url = format!("{}/api/chat/sessions/{}/archive", api_base(), session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", api_token())
            .json(&serde_json::json!({"archived": archived}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
        }
    });
    out
}

pub fn chat_set_pinned_async(session_id: String, pinned: bool) -> SharedResult<Value> {
    let out: SharedResult<Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}/pinned", api_base(), session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", api_token())
            .json(&serde_json::json!({"pinned": pinned}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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
        let url = format!("{}/api/chat/sessions/{}/title", api_base(), session_id);
        let resp = client
            .post(&url)
            .header("X-Auth-Token", api_token())
            .json(&serde_json::json!({"title": title}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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
        let url = format!("{}/api/chat/messages/{}", api_base(), mid);
        let resp = client
            .patch(&url)
            .header("X-Auth-Token", api_token())
            .json(&serde_json::json!({"content": content, "truncate": truncate}))
            .send()
            .await;
        match resp {
            Ok(r) => match json_body(r).await {
                Ok(v) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Ok(v)),
                Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(e)),
            },
            Err(e) => *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(Err(t!("err.request", err = e).to_string())),
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

        /// 清空指定变量（drop 时恢复）——用于剔除会旁路代理的白名单（NO_PROXY/no_proxy）
        fn clear(vars: &[&'static str]) -> Self {
            let saved = vars.iter().map(|k| (*k, std::env::var(k).ok())).collect();
            for k in vars {
                std::env::remove_var(k);
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
            // D 批补（2026-09-11 文档/代码一致性审计）：补齐 ALL_PROXY 两个拼写——
            // 只守 4 个变量时，ambient 环境里的 ALL_PROXY 会让「裸 client」也走真实代理，
            // 对照组的「必须连不通」就会偶发翻成 Some(true)（审计中真实踩到过一次）。
            ("ALL_PROXY", "http://127.0.0.1:9"),
            ("all_proxy", "http://127.0.0.1:9"),
        ]);
        // 同时剔除旁路白名单：NO_PROXY 会让裸 client 绕过死代理直连回环 → 对照失效
        let _env_np = EnvGuard::clear(&["NO_PROXY", "no_proxy"]);

        let rt = match tokio::runtime::Runtime::new() {
            Ok(rt) => rt,
            Err(_) => return,
        };
        // A13（2026-09-10 审计）：Err(_) → None（"连不通"），与"状态码非 2xx"（Some(false)）区分——
        // 主控离线时探测结果为 None → 走下面的跳过分支，不再误报"根治失败"
        let probe = |client: reqwest::Client| {
            rt.block_on(async move {
                match client
                    .get(format!("{}/api/fleet/status", api_base()))
                    .header("X-Auth-Token", api_token())
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
        // 对照（2026-09-11 审计加固——原断言依赖"环境变量生效 + 无 NO_PROXY/ALL_PROXY 干扰"，
        // 在子代理机器环境里偶发翻过（裸 client 竟 Some(true)），且受 reqwest 环境代理缓存影响，
        // 无法稳定复现）。改为**显式死代理**做对照：与进程环境、与环境代理缓存彻底无关。
        let dead_proxy = reqwest::Proxy::all("http://127.0.0.1:9").expect("构造死代理");
        // 对照①：显式死代理 + no_proxy → 仍能打通（这正是被根治的性质——我们的 client 不吃代理）
        let ours_explicit = probe(
            reqwest::Client::builder()
                .proxy(dead_proxy.clone())
                .no_proxy()
                .build()
                .expect("client 构造失败"),
        );
        match ours_explicit {
            None => panic!("no_proxy client 在显式死代理下未能打通回环（根治失败）"),
            Some(false) => panic!("no_proxy client 打通了但状态码非 2xx"),
            Some(true) => {}
        }
        // 对照②：同一个显式死代理、不加 no_proxy → 必须连不通（证明死代理确实在起作用=对照有效）
        let bare = probe(
            reqwest::Client::builder()
                .proxy(dead_proxy)
                .build()
                .expect("client 构造失败"),
        );
        eprintln!("no_proxy client 成功=true; 显式死代理裸 client 结果={:?}（None=被死代理吞掉）", bare);
        assert_eq!(
            bare, None,
            "对照失效：显式死代理下裸 client 竟然连上了 127.0.0.1（对照本身不成立，需检查 reqwest 行为）"
        );
    }
}

// ═══════════ 丙批补（2026-09-10）：召回指针回跳 + 网关前缀命中率 ═══════════

/// 取会话某段窗口（召回指针"回到原文"——GET /api/chat/sessions/{id}/window）
/// 返回 {session_id, around_id, limit, text}；text 为可直接展示的窗口文本
pub fn fetch_chat_window_async(session_id: String, around_id: i64) -> SharedResult<serde_json::Value> {
    let out: SharedResult<serde_json::Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!(
            "{}/api/chat/sessions/{}/window?around_id={}&limit=20",
            api_base(), session_id, around_id
        );
        let r = match client.get(&url).header("X-Auth-Token", api_token()).send().await {
            Ok(resp) => json_body(resp).await,
            Err(e) => Err(t!("err.request", err = e).to_string()),
        };
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(r);
    });
    out
}

/// 重置该会话的压缩冷却/硬熔断（丙批补——手动恢复入口）
pub fn chat_compact_reset_async(session_id: String) -> SharedResult<serde_json::Value> {
    let out: SharedResult<serde_json::Value> = Arc::new(Mutex::new(None));
    let out2 = out.clone();
    runtime().spawn(async move {
        let client = http_client_json();
        let url = format!("{}/api/chat/sessions/{}/compact-reset", api_base(), session_id);
        let r = match client.post(&url).header("X-Auth-Token", api_token()).send().await {
            Ok(resp) => json_body(resp).await,
            Err(e) => Err(t!("err.request", err = e).to_string()),
        };
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(r);
    });
    out
}

/// 网关前缀命中率（丙批 N4——网关 8082；需 X-Auth-Token）
pub async fn fetch_prefix_cache_blocking() -> Result<serde_json::Value, String> {
    let client = http_client_json();
    let url = format!("{}/api/metrics/prefix_cache", ai_base());
    let resp = client.get(&url).header("X-Auth-Token", api_token()).send().await.map_err(|e| e.to_string())?;
    json_body(resp).await
}

#[cfg(test)]
mod a08_abort_tests {
    use super::*;
    use std::sync::atomic::{AtomicBool, Ordering};

    /// A08 备选（2026-09-10）：chat_stream_abort 必须真正取消流式任务——
    /// 否则点 ⏹ 只是标记 cancelled，任务仍要等下一个 chunk/await 才收手。
    #[test]
    fn abort_handle_cancels_stream_task() {
        let state: SharedChatStream = Arc::new(Mutex::new(ChatStreamState::default()));
        let flag = Arc::new(AtomicBool::new(false));
        let f2 = flag.clone();
        let h = runtime().spawn(async move {
            tokio::time::sleep(std::time::Duration::from_millis(400)).await;
            f2.store(true, Ordering::SeqCst);
        });
        {
            let mut st = state.lock().unwrap_or_else(|e| e.into_inner());
            st.abort = Some(h.abort_handle());
            st.join = Some(h);
        }
        chat_stream_abort(&state);
        std::thread::sleep(std::time::Duration::from_millis(700));
        assert!(!flag.load(Ordering::SeqCst), "abort 后任务仍在跑——A08 立即断流机制失效");
    }

    /// A09 备选：句柄入状态后，正常结束应能被 join（不 panic → 不上报）
    #[test]
    fn join_handle_settles_without_panic_report() {
        let _ = take_stream_panic(); // 清空历史
        let state: SharedChatStream = Arc::new(Mutex::new(ChatStreamState::default()));
        let h = runtime().spawn(async move {});
        {
            let mut st = state.lock().unwrap_or_else(|e| e.into_inner());
            st.join = Some(h);
        }
        // 观察者等价逻辑：join 完成后不应写入 panic 槽
        let jest = state.lock().unwrap_or_else(|e| e.into_inner()).join.take();
        if let Some(h) = jest {
            assert!(runtime().block_on(h).is_ok(), "正常任务 join 不应报错");
        }
        assert!(take_stream_panic().is_none(), "正常结束不应上报 panic");
    }
}

/// UI 运行目录（ZERG_UI_DIR 可覆盖，默认 <tmp>/zerg-ui）——模块清单/外部模块声明落此
pub fn ui_dir() -> std::path::PathBuf {
    // 2026-09-13（Mr2109拍板 Q8）：默认位置由 /tmp/zerg-ui 改为跟随主控状态根 ~/.zerg/state/ui。
    // 原因：/tmp 不是持久位置 —— 状态一旦被清，用户"卸下（禁用）"的虫茧会按默认全在船而复live；
    // 且与本项目既有纪律（运行态状态进 ~/.zerg，见 statepath.Dir 的同一规则）不一致。
    // ZERG_UI_DIR 仍可显式覆盖（本机行为不变）；首次访问会把旧 /tmp 文件搬一次，绝不覆盖已有目标。
    if let Ok(v) = std::env::var("ZERG_UI_DIR") {
        if !v.trim().is_empty() {
            return std::path::PathBuf::from(v);
        }
    }
    // 测试进程一律用临时目录：既有用例会经由本函数读/写虫茧状态，
    // 旧位置是 /tmp/zerg-ui（无害），改到 ~/.zerg/state/ui 后就变成"测试写用户真实状态"——
    // 属 skill 里那条"测试别写真机状态（同族会复发）"的事故，这里从源头隔离，连迁移也不做。
    if cfg!(test) {
        return std::env::temp_dir().join("zerg-ui-state-under-test");
    }
    let home = std::env::var("HOME").unwrap_or_else(|_| "/tmp".to_string());
    let dir = std::path::PathBuf::from(home)
        .join(".zerg")
        .join("state")
        .join("ui");
    migrate_legacy_ui_state(&dir);
    dir
}

/// 旧位置（本次改动之前）：`<ZERG_TMP_DIR|/tmp>/zerg-ui`
fn legacy_ui_dir() -> std::path::PathBuf {
    let base = std::env::var("ZERG_TMP_DIR").unwrap_or_else(|_| "/tmp".to_string());
    std::path::PathBuf::from(base).join("zerg-ui")
}

/// 随容器状态一起搬的文件（两者都是"用户选择"类状态）
/// 2026-09-13：加入 `ui_state.json`（导航选中：父+子——Q9/Q10，与 modules.json 同目录的用户选择状态）。
const UI_STATE_FILES: [&str; 3] = ["modules.json", "external-modules.json", "ui_state.json"];

/// 一次性迁移（进程内只做一次）：把旧 /tmp/zerg-ui 的状态搬到新目录。
fn migrate_legacy_ui_state(new_dir: &std::path::Path) {
    use std::sync::Once;
    static ONCE: Once = Once::new();
    ONCE.call_once(|| migrate_ui_state_files(new_dir, &legacy_ui_dir()));
}

/// 迁移实现（与调用点分离，便于测试）：目标已存在则不覆盖；旧文件不存在则跳过；失败打日志不静默。
fn migrate_ui_state_files(new_dir: &std::path::Path, old_dir: &std::path::Path) {
    for name in UI_STATE_FILES {
        let old = old_dir.join(name);
        let new = new_dir.join(name);
        if new.exists() || !old.exists() {
            continue;
        }
        if let Err(e) = std::fs::create_dir_all(new_dir) {
            eprintln!("[zerg-ui] failed to create the state directory {}: {}", new_dir.display(), e);
            continue;
        }
        match std::fs::copy(&old, &new) {
            Ok(_) => eprintln!("[zerg-ui] migrated legacy UI state {} -> {}", old.display(), new.display()),
            Err(e) => eprintln!("[zerg-ui] failed to migrate legacy UI state {} -> {}: {}", old.display(), new.display(), e),
        }
    }
}

/// UI 界面偏好文件（**新落点**）：`<UI 状态目录>/prefs.json`
/// （2026-09-13 Q10：由散在 HOME 根的 `~/.zerg-ui-prefs.json` 迁移到 ~/.zerg/state/ui）
pub fn prefs_path() -> std::path::PathBuf {
    ui_dir().join("prefs.json")
}

/// 界面偏好的**旧落点**（2026-09-13 之前）：`~/.zerg-ui-prefs.json`
/// ——读时兼容：迁移未及/旧机器上仍能读到（含 auth_token/locale/ai_model/mreg_view…）
pub fn legacy_prefs_path() -> std::path::PathBuf {
    let home = std::env::var("HOME").unwrap_or_else(|_| "/tmp".to_string());
    std::path::PathBuf::from(home).join(".zerg-ui-prefs.json")
}

/// 读界面偏好**文本**：新落点优先，缺则回退旧落点（保证既有字段一个不丢——尤其 auth_token）。
pub fn read_prefs() -> Option<String> {
    if let Ok(s) = std::fs::read_to_string(prefs_path()) {
        return Some(s);
    }
    std::fs::read_to_string(legacy_prefs_path()).ok()
}

/// 迁移一对文件：目标已存在则不覆盖；旧文件不存在则跳过；失败打日志不静默。
/// （模块级复用——modules.json 与 prefs.json / ui_layout.json 同一语义。）
fn migrate_file(new_path: &std::path::Path, old_path: &std::path::Path, what: &str) {
    if new_path == old_path || new_path.exists() || !old_path.exists() {
        return;
    }
    if let Some(dir) = new_path.parent() {
        if let Err(e) = std::fs::create_dir_all(dir) {
            eprintln!("[zerg-ui] failed to create the state directory {}: {}", dir.display(), e);
            return;
        }
    }
    match std::fs::copy(old_path, new_path) {
        Ok(_) => eprintln!("[zerg-ui] migrated legacy {} {} -> {}", what, old_path.display(), new_path.display()),
        Err(e) => eprintln!("[zerg-ui] failed to migrate legacy {} {} -> {}: {}", what, old_path.display(), new_path.display(), e),
    }
}

/// 迁移界面偏好：`~/.zerg-ui-prefs.json` → `<UI 状态目录>/prefs.json`（目标已存在绝不覆盖）
pub fn migrate_prefs_file(new_path: &std::path::Path, old_path: &std::path::Path) {
    migrate_file(new_path, old_path, "UI prefs");
}

/// 迁移布局比例文件：`<任务根>/ui_layout.json` → `<UI 状态目录>/ui_layout.json`（E18）
pub fn migrate_layout_file(new_path: &std::path::Path, old_path: &std::path::Path) {
    migrate_file(new_path, old_path, "layout");
}

/// 进程内一次性自动迁移（偏好 + 布局）。测试环境跳过——不读写真机 HOME/任务根。
pub fn migrate_persistent_state_once() {
    if cfg!(test) {
        return;
    }
    use std::sync::Once;
    static ONCE: Once = Once::new();
    ONCE.call_once(|| {
        migrate_prefs_file(&prefs_path(), &legacy_prefs_path());
        let old_layout = task_root().join("ui_layout.json");
        migrate_layout_file(&ui_dir().join("ui_layout.json"), &old_layout);
    });
}

#[cfg(test)]
mod ui_dir_tests {
    use super::*;

    fn tmp(name: &str) -> std::path::PathBuf {
        let d = std::env::temp_dir().join(format!("zerg-ui-state-test-{}-{}", std::process::id(), name));
        let _ = std::fs::remove_dir_all(&d);
        d
    }

    #[test]
    fn migrate_copies_when_target_missing() {
        let old = tmp("old-a");
        let new = tmp("new-a");
        std::fs::create_dir_all(&old).unwrap();
        std::fs::write(old.join("modules.json"), r#"{"git":false,"logs":false}"#).unwrap();
        migrate_ui_state_files(&new, &old);
        let got = std::fs::read_to_string(new.join("modules.json")).unwrap();
        assert_eq!(got, r#"{"git":false,"logs":false}"#); // 字面值断言：逐字节相同
    }

    #[test]
    fn migrate_never_overwrites_existing_target() {
        let old = tmp("old-b");
        let new = tmp("new-b");
        std::fs::create_dir_all(&old).unwrap();
        std::fs::create_dir_all(&new).unwrap();
        std::fs::write(old.join("modules.json"), r#"{"git":false}"#).unwrap();
        std::fs::write(new.join("modules.json"), r#"{"logs":false}"#).unwrap(); // 目标已有 → 不许动
        migrate_ui_state_files(&new, &old);
        assert_eq!(std::fs::read_to_string(new.join("modules.json")).unwrap(), r#"{"logs":false}"#);
    }

    #[test]
    fn migrate_skips_when_legacy_missing() {
        let old = tmp("old-c");   // 故意不创建
        let new = tmp("new-c");
        migrate_ui_state_files(&new, &old);
        assert!(!new.join("modules.json").exists());
        assert!(!new.join("external-modules.json").exists());
    }

    // ─── E18/§七17：布局比例文件迁移（两态——内容保持 / 不覆盖已有目标）─────────────

    #[test]
    fn layout_migration_keeps_content_when_target_missing() {
        let old = tmp("layout-old");
        let new = tmp("layout-new");
        std::fs::create_dir_all(&old).unwrap();
        std::fs::write(old.join("ui_layout.json"), r#"{"split_model":0.41,"split_it":0.5}"#).unwrap();
        migrate_layout_file(&new.join("ui_layout.json"), &old.join("ui_layout.json"));
        let got = std::fs::read_to_string(new.join("ui_layout.json")).unwrap();
        assert_eq!(got, r#"{"split_model":0.41,"split_it":0.5}"#, "非默认比例必须原样搬过去");
    }

    #[test]
    fn layout_migration_never_overwrites_existing_target() {
        let old = tmp("layout-old2");
        let new = tmp("layout-new2");
        std::fs::create_dir_all(&old).unwrap();
        std::fs::create_dir_all(&new).unwrap();
        std::fs::write(old.join("ui_layout.json"), r#"{"split_model":0.41}"#).unwrap();
        std::fs::write(new.join("ui_layout.json"), r#"{"split_model":0.9}"#).unwrap(); // 目标已有
        migrate_layout_file(&new.join("ui_layout.json"), &old.join("ui_layout.json"));
        assert_eq!(
            std::fs::read_to_string(new.join("ui_layout.json")).unwrap(),
            r#"{"split_model":0.9}"#,
            "目标已存在 ⇒ 绝不覆盖（新落点为准）"
        );
    }

    // ─── §十 Q10：界面偏好迁移（含 auth_token——别弄丢 token 读取）─────────────────

    #[test]
    fn prefs_migration_moves_and_never_overwrites() {
        let old = tmp("prefs-old");
        let new = tmp("prefs-new");
        std::fs::create_dir_all(&old).unwrap();
        let old_file = old.join(".zerg-ui-prefs.json");
        std::fs::write(&old_file, r#"{"auth_token":"t0k","locale":"zh-CN"}"#).unwrap();
        let new_path = new.join("prefs.json");
        migrate_prefs_file(&new_path, &old_file);
        assert_eq!(
            std::fs::read_to_string(&new_path).unwrap(),
            r#"{"auth_token":"t0k","locale":"zh-CN"}"#,
            "旧偏好（含 auth_token）必须原样搬到新落点"
        );
        // 第二次：目标已存在 ⇒ 不覆盖
        std::fs::write(&new_path, r#"{"auth_token":"keep"}"#).unwrap();
        migrate_prefs_file(&new_path, &old_file);
        assert_eq!(std::fs::read_to_string(&new_path).unwrap(), r#"{"auth_token":"keep"}"#);
    }

    #[test]
    fn ui_state_file_is_in_migration_whitelist() {
        assert!(UI_STATE_FILES.contains(&"ui_state.json"), "ui_state.json 必须在迁移白名单里");
    }

    // B7 / G10：**跨语言契约测试** —— UI 自己的状态文件必须出现在 Go 侧的兼容清单
    // （core/internal/compat/compat.json）里，且归属/信封/版本关系自洽。
    //
    // 为什么这条测试在 Rust 侧：清单是「哪些文件跨版本必须读懂」的单一真源，而 ui/*.json 是
    // Rust 写的。只写在 Go 侧就等于「UI 新加一个状态文件，Go 门禁看不见」。这条把两侧钉在一起。
    #[test]
    fn ui_state_files_are_declared_in_compat_manifest() {
        let manifest_path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("..")
            .join("core")
            .join("internal")
            .join("compat")
            .join("compat.json");
        let text = std::fs::read_to_string(&manifest_path).unwrap_or_else(|e| {
            panic!("读不到 Go 侧兼容清单 {}: {}（跨版本状态契约不能缺席）", manifest_path.display(), e)
        });
        let m: serde_json::Value = serde_json::from_str(&text).expect("兼容清单必须是合法 JSON");
        let entries = m["entries"].as_array().expect("兼容清单必须有 entries 数组");

        // UI 写出的全部状态文件（相对状态目录）：一个都不能漏登记
        let want = [
            "ui/modules.json",
            "ui/external-modules.json",
            "ui/ui_state.json",
            "ui/prefs.json",
            "ui/ui_layout.json",
        ];
        for file in want {
            let hit = entries.iter().find(|e| e["file"].as_str() == Some(file));
            let e = hit.unwrap_or_else(|| panic!("{} 未登记在 core/internal/compat/compat.json 里", file));
            assert_eq!(e["owner"].as_str(), Some("ui"), "{} 的 owner 必须是 ui", file);
            let env = e["envelope"].as_str().unwrap_or("");
            assert!(env == "inband" || env == "sidecar", "{} 的 envelope 非法（{}）", file, env);
            let cur = e["current_schema"].as_i64().unwrap_or(0);
            let min = e["min_readable"].as_i64().unwrap_or(-1);
            assert!(cur >= 1, "{} 的 current_schema 必须 ≥1（实际 {}）", file, cur);
            assert!(cur >= min, "{} 的 current_schema({}) 必须 ≥ min_readable({})", file, cur, min);
            // 形状是「每个键都是数据」的文件**必须**走旁路信封——在信封里塞 schema 会吃掉用户状态
            if file == "ui/modules.json" || file == "ui/ui_layout.json" || file == "ui/external-modules.json" {
                assert_eq!(env, "sidecar", "{} 的形状不允许把 schema 塞进信封（会被读侧当成数据）", file);
            }
        }
    }
}

/// CA 任务根目录（与主控一致：ZERG_TASK_ROOT → <ZERG_TMP_DIR|/tmp>/zerg-tasks）
pub fn task_root() -> std::path::PathBuf {
    if let Ok(v) = std::env::var("ZERG_TASK_ROOT") {
        if !v.trim().is_empty() {
            return std::path::PathBuf::from(v);
        }
    }
    let base = std::env::var("ZERG_TMP_DIR").unwrap_or_else(|_| "/tmp".to_string());
    std::path::PathBuf::from(base).join("zerg-tasks")
}
