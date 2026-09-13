//! 交给系统的动作 + 错误码映射（文件浏览器阶段 1——2026-09-13 设计 §4.3 / §4.4）
//!
//! - `open` / `reveal`：POST /api/fileroots/{open,reveal}，**只传 root + 相对 path**
//!   （绝对路径的白名单校验在**后端**——前端不拼绝对路径回传，§4.2 ③）。
//! - 复制路径：沿用既有写法 `ui.ctx().copy_text(..)`（与任务模块/模型登记库同款）。
//! - 错误码 → 可读文案：`INVALID_ROOT / INVALID_PATH / NOT_FOUND / NOT_ALLOWED /
//!   READ_FAILED / OPEN_FAILED` 六个码 + 未收录码兜底；**任何输入都不 panic**。

use eframe::egui;

use crate::api;

/// 后端错误码 → i18n 文案键（§4.2 错误码表；未收录 → `fb.error.unknown`）
pub fn fileroot_error_key(code: &str) -> &'static str {
    match code.trim().to_ascii_uppercase().as_str() {
        "INVALID_ROOT" => "fb.error.invalid_root",
        "INVALID_PATH" => "fb.error.invalid_path",
        "NOT_FOUND" => "fb.error.not_found",
        "NOT_ALLOWED" => "fb.error.not_allowed",
        "READ_FAILED" => "fb.error.read_failed",
        "OPEN_FAILED" => "fb.error.open_failed",
        _ => "fb.error.unknown",
    }
}

/// 错误码 → 当前语言的可读文案（不 panic）
pub fn fileroot_error_text(code: &str) -> String {
    rust_i18n::t!(fileroot_error_key(code)).to_string()
}

/// 从后端错误体取错误码（两种形态，与 api::parse_api_error 同源）：
/// - `{"error":{"type":"NOT_ALLOWED","message":"…"}}`
/// - `{"error":"NOT_ALLOWED"}`
pub fn fileroot_error_code(body: &serde_json::Value) -> Option<String> {
    let e = body.get("error")?;
    if let Some(c) = e.get("type").and_then(|x| x.as_str()) {
        return Some(c.to_string());
    }
    e.as_str().map(|s| s.to_string())
}

/// 错误体 → 可读文案：有码走码；无码回退服务端 message；都没有 → 通用文案（**绝不**把原始
/// JSON 直接糊到界面上，也绝不 panic）
pub fn fileroot_error_from_body(body: &serde_json::Value) -> String {
    if let Some(code) = fileroot_error_code(body) {
        // 未收录码：文案是通用的，但把码附在后面便于排查（服务端新增码不必同步发 UI）
        let key = fileroot_error_key(&code);
        if key == "fb.error.unknown" {
            return format!("{} ({})", fileroot_error_text(&code), code);
        }
        return fileroot_error_text(&code);
    }
    if let Some(m) = body
        .get("error")
        .and_then(|e| e.get("message").or(Some(e)))
        .and_then(|m| m.as_str())
    {
        return m.to_string();
    }
    rust_i18n::t!("fb.error.unknown").to_string()
}

/// open / reveal 的 blocking 实现（POST /api/fileroots/{action}）
/// 成功 → Ok(后端回报的绝对路径 abs)；失败 → Err(可读文案)
pub async fn fileroot_action_blocking(
    action: &str,
    root: &str,
    path: &str,
    mode: Option<&str>,
) -> Result<String, String> {
    let client = api::http_client_json();
    let url = format!("{}/api/fileroots/{}", api::api_base(), action);
    let mut payload = serde_json::json!({ "root": root, "path": path });
    if let Some(m) = mode {
        payload["mode"] = serde_json::json!(m);
    }
    let resp = client
        .post(&url)
        .header("X-Auth-Token", api::api_token())
        .json(&payload)
        .timeout(std::time::Duration::from_secs(10))
        .send()
        .await
        .map_err(|e| format!("{}", e))?;
    let status = resp.status();
    let text = resp.text().await.unwrap_or_default();
    let body: serde_json::Value = serde_json::from_str(&text).unwrap_or(serde_json::Value::Null);
    if !status.is_success() {
        return Err(fileroot_error_from_body(&body));
    }
    if body.get("ok").and_then(|o| o.as_bool()) != Some(true) {
        return Err(fileroot_error_from_body(&body));
    }
    Ok(body
        .get("abs")
        .and_then(|a| a.as_str())
        .unwrap_or_default()
        .to_string())
}

/// open / reveal（异步——结果交回 UI 轮询；与 api::doc_op_async 同款，不阻塞 UI 线程）
pub fn fileroot_action_async(
    action: &str,
    root: String,
    path: String,
    mode: Option<&'static str>,
) -> api::SharedResult<String> {
    let out: api::SharedResult<String> = std::sync::Arc::new(std::sync::Mutex::new(None));
    let out2 = out.clone();
    let action = action.to_string();
    api::runtime().spawn(async move {
        let r = fileroot_action_blocking(&action, &root, &path, mode).await;
        *out2.lock().unwrap_or_else(|e| e.into_inner()) = Some(r);
    });
    out
}

/// 复制路径（沿用既有 `ui.ctx().copy_text` 写法——任务模块/模型登记库同款）
pub fn copy_path(ctx: &egui::Context, path: &str) {
    if !path.is_empty() {
        ctx.copy_text(path.to_string());
    }
}
