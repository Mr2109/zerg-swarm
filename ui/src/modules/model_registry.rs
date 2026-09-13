//! 模型登记库页（模型库——登记表；设计见 docs/01-设计/设计-模型库与对话自动调入-20260911.md）
//!
//! 数据源：GET /api/models/registry（只读快照，与其它 /api 同一套令牌中间件）。
//!
//! 三个界面状态各自独立处理（都不得白屏、不得 panic）：
//! - 拉取中 → 转圈 + 文案；
//! - 失败   → 可读错误 + 「重试」按钮（页面内自愈，不重启 UI）；
//! - 空目录 → 友好空态 + 一行「怎么加模型」（zerg-model probe --store）。
//! 坏记录（errors>0 / 带 error 文案）仍照常列出，用同情的语气说明——坏记录不能弄坏整页。
//!
//! 数据拉取在 tokio runtime 内异步执行（见 api::fetch_model_registry_async），
//! UI 线程每帧只做一次「结果回填」的轮询（pump），从不阻塞。
//!
//! 本批（视图/筛选/详情）新增三样（设计 §1.1 / §1.2 / 2补.10）：
//! ① **两种视图**——能力视图（默认：顶部一排能力芯片，按数据里真实出现过的能力动态生成）
//!    / 工程视图（紧凑表格，排障用）。视图选择**记住**（写 ~/.zerg-ui-prefs.json）；
//! ② **搜索**——id / 显示名 / digest 模糊匹配（大小写不敏感），与能力筛选叠加生效；
//! ③ **详情层**——点卡片或表格行弹出：能力断言带 source/evidence、建材逐项、
//!    引擎配方、license 全字段（缺值显「—」，**空值不是错误**）、digest 全文、路径 + 复制按钮。
//!
//! 本批**不做**：自动路由、对话内选模型、发现/本地范围切换（后续批）。
//! 接口形状一律照现有 /api/models/registry（字段名不变、后端不改）。
//!
//! 补批（待修补 #39）**证据链三样**：详情层让"结论是从哪来的"看得见——
//! ① 能力断言的**引擎维度** `capabilities[].engines`（"这能力是在哪个引擎上测出来的"）；
//! ② **无法判定**的能力 `unverifiable[]`（"这些能力当时没能判定（预算不够/超时）"）；
//! ③ 许可证结论的**来源锚** `license.evidence`（"这个许可结论是从哪读来的"）。
//! 三样都照抄接口值：缺 / 空数组 → 显「—」，绝不填占位（与后端"整键不出现"同一口径）。

use egui::RichText;
use rust_i18n::t; // 文件级导入——否则 t!() 报 cannot find macro（B2a 同坑）
use crate::api::{self, SharedResult};
use crate::modules::icons::icon_text;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;

/// 已就绪的数据（Ok = 快照；Err = 可读错误文案）。None = 尚未拿到任何结果。
static DATA: Mutex<Option<Result<serde_json::Value, String>>> = Mutex::new(None);
/// 在途请求句柄（结果由 pump() 每帧取回一次）
static PENDING: Mutex<Option<SharedResult<serde_json::Value>>> = Mutex::new(None);
/// 是否有请求在途（防重复发起——含重试与自动首次拉取）
static LOADING: AtomicBool = AtomicBool::new(false);

/// 当前视图（None = 尚未从偏好文件读取；默认能力视图——设计 §1.1 / §十二#1）
static VIEW: Mutex<Option<View>> = Mutex::new(None);
/// 搜索词（会话级；空 = 不过滤）
static SEARCH: Mutex<String> = Mutex::new(String::new());
/// 能力芯片选中项（None = 全部）
static CAP: Mutex<Option<String>> = Mutex::new(None);
/// 详情层当前展开的记录键（None = 未展开）
static SELECTED: Mutex<Option<String>> = Mutex::new(None);
/// 最近一次复制成功的路径（用于「已复制」反馈）
static COPIED: Mutex<Option<String>> = Mutex::new(None);

/// 数据是否已就绪（首次进页自动拉取；失败后由「重试」手动触发，不无限自动重试）
fn has_data() -> bool {
    DATA.lock().map(|d| d.is_some()).unwrap_or(false)
}

fn loading() -> bool {
    LOADING.load(Ordering::SeqCst)
}

/// 发起一次异步拉取（已在途则忽略——幂等）
fn start_fetch() {
    if LOADING.swap(true, Ordering::SeqCst) {
        return; // 已在途
    }
    let handle = api::fetch_model_registry_async();
    if let Ok(mut p) = PENDING.lock() {
        *p = Some(handle);
    }
}

/// 清掉旧结果（含错误态）后重新拉取——「重试」按钮
fn retry() {
    if let Ok(mut d) = DATA.lock() {
        *d = None;
    }
    start_fetch();
}

/// 每帧泵一次：把在途结果取回 DATA（只取一次，不阻塞——锁只在结果已就绪时短暂持有）
fn pump() {
    let mut incoming: Option<Result<serde_json::Value, String>> = None;
    if let Ok(mut p) = PENDING.lock() {
        if let Some(h) = p.as_ref() {
            let mut g = h.lock().unwrap_or_else(|e| e.into_inner());
            if g.is_some() {
                incoming = g.take(); // take：结果只回填一次
            }
        }
        if incoming.is_some() {
            *p = None;
        }
    }
    if let Some(r) = incoming {
        if let Ok(mut d) = DATA.lock() {
            *d = Some(r);
        }
        LOADING.store(false, Ordering::SeqCst);
    }
}

// ───────────────────────── 纯函数（可单测，不依赖 egui 状态） ─────────────────────────

/// 缺值占位符（两语言同形——数据缺项不是错误，设计：空值显「—」）
const DASH: &str = "—";

/// 两种视图（设计 §1.1：能力视图给用的人看，工程视图给你和我看）
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
enum View {
    Capability,
    Engineering,
}

impl View {
    /// 落盘取值（~/.zerg-ui-prefs.json 的 mreg_view 字段）
    fn as_str(self) -> &'static str {
        match self {
            View::Capability => "capability",
            View::Engineering => "engineering",
        }
    }

    /// 纯解析：陌生取值/大小写差异 → None（用默认，不猜）
    fn parse(s: &str) -> Option<View> {
        match s.trim().to_ascii_lowercase().as_str() {
            "capability" => Some(View::Capability),
            "engineering" => Some(View::Engineering),
            _ => None,
        }
    }
}

/// 偏好的一个字段值：缺 = 没这个字段（显「—」），而不是"空字符串=错误"
#[derive(Clone, PartialEq, Debug)]
enum FieldVal {
    Missing,
    Text(String),
    Flag(bool),
}

impl FieldVal {
    /// 展示文案（Flag 走 i18n；Missing 走「—」占位符——缺值不报错）
    fn display(&self) -> String {
        match self {
            FieldVal::Missing => DASH.to_string(),
            FieldVal::Text(s) => s.clone(),
            FieldVal::Flag(true) => t!("mreg.value_yes").to_string(),
            FieldVal::Flag(false) => t!("mreg.value_no").to_string(),
        }
    }
}

/// 偏好文件路径（与语言/预览渲染器偏好同一文件——见 app.rs::preview_pref_path）
fn prefs_path() -> std::path::PathBuf {
    // 2026-09-13 Q10：偏好文件落点迁到 <UI 状态目录>/prefs.json（与语言/预览渲染器偏好同一文件）
    crate::api::prefs_path()
}

/// 纯解析：偏好文件文本 → 视图。坏 JSON / 缺字段 / 陌生取值一律 None（用默认能力视图）
fn parse_view_pref(s: &str) -> Option<View> {
    let v: serde_json::Value = serde_json::from_str(s).ok()?;
    View::parse(v.get("mreg_view").and_then(|x| x.as_str())?)
}

/// 纯合并：把视图写回偏好 JSON 文本。
/// **必须保留其它字段**（locale / ai_model / preview_renderer…）——整文件覆盖会吃掉用户偏好。
/// 坏 JSON / 非对象 → 视为空对象重来（不 panic、不丢文件）。
fn merge_view_pref(existing: &str, view: View) -> String {
    let mut v: serde_json::Value = serde_json::from_str(existing).unwrap_or_else(|_| serde_json::json!({}));
    if !v.is_object() {
        v = serde_json::json!({});
    }
    v["mreg_view"] = serde_json::Value::String(view.as_str().to_string());
    serde_json::to_string_pretty(&v).unwrap_or_else(|_| "{}".to_string())
}

/// 写盘（读改写——失败静默：偏好丢一次不影响功能）
fn save_view(view: View) {
    let p = prefs_path();
    // 读改写：读时兼容旧路径（新落点优先）——保住 locale/ai_model/auth_token 等既有字段
    let existing = crate::api::read_prefs().unwrap_or_default();
    let _ = std::fs::write(&p, merge_view_pref(&existing, view));
}

/// 读盘：无文件 / 坏文件 → 默认能力视图（能力视图是设计拍定的默认）
fn load_view() -> View {
    crate::api::read_prefs()
        .and_then(|s| parse_view_pref(&s))
        .unwrap_or(View::Capability)
}

/// 当前视图（首次调用从偏好文件惰性加载一次）
fn current_view() -> View {
    let mut g = VIEW.lock().unwrap_or_else(|e| e.into_inner());
    if g.is_none() {
        *g = Some(load_view());
    }
    g.unwrap_or(View::Capability)
}

fn set_view(v: View) {
    if let Ok(mut g) = VIEW.lock() {
        *g = Some(v);
    }
}

fn search_state() -> String {
    SEARCH.lock().map(|s| s.clone()).unwrap_or_default()
}

fn set_search(s: String) {
    if let Ok(mut g) = SEARCH.lock() {
        *g = s;
    }
}

fn cap_state() -> Option<String> {
    CAP.lock().ok().and_then(|c| c.clone())
}

fn set_cap(c: Option<String>) {
    if let Ok(mut g) = CAP.lock() {
        *g = c;
    }
}

fn selected_state() -> Option<String> {
    SELECTED.lock().ok().and_then(|c| c.clone())
}

fn set_selected(s: Option<String>) {
    if let Ok(mut g) = SELECTED.lock() {
        *g = s;
    }
}

fn copied_state() -> Option<String> {
    COPIED.lock().ok().and_then(|c| c.clone())
}

fn set_copied(s: Option<String>) {
    if let Ok(mut g) = COPIED.lock() {
        *g = s;
    }
}

/// 记录键：id|version（(id, version) 唯一——接口按此排序；名字只是别名，id 才是身份）
fn rec_key(rec: &serde_json::Value) -> String {
    let id = json_str(rec.get("id"));
    let ver = json_str(rec.get("version"));
    format!("{}|{}", if id.is_empty() { "?" } else { id.as_str() }, ver)
}

/// 字符串字段取值（缺/非串 → 空串；调用方再决定占位）
fn json_str(v: Option<&serde_json::Value>) -> String {
    v.and_then(|x| x.as_str()).unwrap_or("").trim().to_string()
}

/// 空串 → 「—」占位（缺值不报错）
fn or_dash(s: String) -> String {
    if s.is_empty() {
        DASH.to_string()
    } else {
        s
    }
}

/// 显示名：优先 name，缺则退回 id（缺 id 再退回占位符）——「名字是别名，id 是身份」
fn display_name(rec: &serde_json::Value) -> String {
    let name = rec.get("name").and_then(|x| x.as_str()).unwrap_or("").trim();
    if !name.is_empty() {
        return name.to_string();
    }
    let id = rec.get("id").and_then(|x| x.as_str()).unwrap_or("").trim();
    if !id.is_empty() {
        return id.to_string();
    }
    String::from("?")
}

/// 许可徽标：commercial → (i18n 键, 颜色)。
/// 取值口径见设计 §1.4：yes 绿/可商用、no 红/非商用、revenue_gated 黄/营收门槛、其余 灰/待核。
fn license_badge(commercial: &str) -> (&'static str, egui::Color32) {
    match commercial {
        "yes" => ("mreg.license_yes", egui::Color32::from_rgb(80, 200, 120)),
        "no" => ("mreg.license_no", egui::Color32::from_rgb(220, 80, 80)),
        "revenue_gated" => ("mreg.license_revenue_gated", egui::Color32::from_rgb(240, 200, 80)),
        _ => ("mreg.license_unknown", egui::Color32::from_rgb(150, 150, 150)),
    }
}

/// 人类可读体积（二进制单位——与本机习惯一致）
fn human_size(bytes: i64) -> String {
    let b = bytes.max(0) as f64;
    const KB: f64 = 1024.0;
    if b >= KB * KB * KB {
        format!("{:.2} GB", b / (KB * KB * KB))
    } else if b >= KB * KB {
        format!("{:.1} MB", b / (KB * KB))
    } else if b >= KB {
        format!("{:.1} KB", b / KB)
    } else {
        format!("{} B", bytes.max(0))
    }
}

/// 建材汇总：文件数 + 合计字节（files 缺失/非数组 = 0）
fn file_summary(files: Option<&Vec<serde_json::Value>>) -> (usize, i64) {
    match files {
        Some(arr) => {
            let total = arr
                .iter()
                .map(|f| f.get("size").and_then(|x| x.as_i64()).unwrap_or(0))
                .sum();
            (arr.len(), total)
        }
        None => (0, 0),
    }
}

/// 快照里的记录列表（缺 records / 非数组 → 空切片，绝不 panic）
fn record_list(v: &serde_json::Value) -> &[serde_json::Value] {
    v.get("records")
        .and_then(|r| r.as_array())
        .map(|a| a.as_slice())
        .unwrap_or(&[])
}

/// 该记录是否**具备**某能力（value=true 才算具备；value=false = 实测没有，不算）
fn has_capability(rec: &serde_json::Value, name: &str) -> bool {
    rec.get("capabilities")
        .and_then(|c| c.as_array())
        .map(|arr| {
            arr.iter().any(|c| {
                c.get("name").and_then(|x| x.as_str()) == Some(name)
                    && c.get("value").and_then(|x| x.as_bool()).unwrap_or(false)
            })
        })
        .unwrap_or(false)
}

/// 能力芯片：**从数据里真实出现过的能力动态生成**（不写死全表）。
/// 口径：只收 value=true 的能力（"具备"才配当筛选项——value=false 的芯片点开会是空列表）；
/// 去重 + 排序（稳定顺序，不随 JSON 顺序跳动）。
fn capability_chips(records: &[serde_json::Value]) -> Vec<String> {
    let mut out: Vec<String> = Vec::new();
    for rec in records {
        if let Some(arr) = rec.get("capabilities").and_then(|c| c.as_array()) {
            for c in arr {
                if !c.get("value").and_then(|x| x.as_bool()).unwrap_or(false) {
                    continue;
                }
                let n = c.get("name").and_then(|x| x.as_str()).unwrap_or("").trim();
                if !n.is_empty() && !out.iter().any(|x| x == n) {
                    out.push(n.to_string());
                }
            }
        }
    }
    out.sort();
    out
}

/// 芯片点击：点当前已选的能力 = 取消筛选（回到「全部」）
fn chip_toggle(current: Option<&str>, clicked: &str) -> Option<String> {
    if current == Some(clicked) {
        None
    } else {
        Some(clicked.to_string())
    }
}

/// 搜索：id / 显示名(name) / digest 三字段模糊匹配（大小写不敏感）；空查询 = 全通过
fn matches_search(rec: &serde_json::Value, query: &str) -> bool {
    let q = query.trim().to_lowercase();
    if q.is_empty() {
        return true;
    }
    ["id", "name", "digest"].iter().any(|k| {
        rec.get(*k)
            .and_then(|x| x.as_str())
            .map(|s| s.to_lowercase().contains(&q))
            .unwrap_or(false)
    })
}

/// 筛选（搜索 + 能力，叠加生效 = 逻辑与）：两者都通过才留下
fn matches_filter(rec: &serde_json::Value, query: &str, cap: Option<&str>) -> bool {
    matches_search(rec, query) && cap.map(|c| has_capability(rec, c)).unwrap_or(true)
}

/// 过滤后的记录引用（保持原顺序——接口已按 (id, version) 排序）
fn filter_records<'a>(
    records: &'a [serde_json::Value],
    query: &str,
    cap: Option<&str>,
) -> Vec<&'a serde_json::Value> {
    records.iter().filter(|r| matches_filter(r, query, cap)).collect()
}

/// digest 前 12 位（表格列用；空 = 「—」）
fn short_digest(d: &str) -> String {
    let t = d.trim();
    if t.is_empty() {
        return DASH.to_string();
    }
    t.chars().take(12).collect()
}

/// 按**字符**（不是字节）截断，超长加省略号——中文/emoji 不会截断成半个字（不 panic）
fn truncate_chars(s: &str, max: usize) -> String {
    if s.chars().count() <= max {
        return s.to_string();
    }
    let mut out: String = s.chars().take(max).collect();
    out.push('…');
    out
}

/// 许可证 9 个字段的（标签键, 字段名）——字段名照 schema: zerg.model.v1（标准 §五 + #12 来源锚）。
const LICENSE_FIELDS: [(&str, &str); 9] = [
    ("mreg.lic_spdx", "spdx"),
    ("mreg.lic_name", "license_name"),
    ("mreg.lic_link", "license_link"),
    ("mreg.lic_commercial", "commercial"),
    ("mreg.lic_gated", "gated"),
    ("mreg.lic_source_url", "source_url"),
    ("mreg.lic_accepted_by", "accepted_by"),
    ("mreg.lic_accepted_at", "accepted_at"),
    // 待修补 #39：许可证结论的来源锚（probe.license.v1 读的是哪个键/哪个文件）。
    // 放最后——保持既有 8 个字段的下标稳定（既有断言不受影响）。
    ("mreg.lic_evidence", "evidence"),
];

/// license 全字段（有就显示、无就「—」）。
/// commercial 双来源：优先 `license.commercial`，缺则退回顶层 `commercial`
/// （顶层 commercial 是现有接口的形状；license 块将来出现时自动接管，不用改这里）。
fn license_rows(rec: &serde_json::Value) -> Vec<(&'static str, FieldVal)> {
    let lic = rec.get("license");
    LICENSE_FIELDS
        .iter()
        .map(|(label, field)| {
            let val = match *field {
                "commercial" => {
                    let a = json_str(lic.and_then(|l| l.get("commercial")));
                    let b = json_str(rec.get("commercial"));
                    if !a.is_empty() {
                        FieldVal::Text(a)
                    } else if !b.is_empty() {
                        FieldVal::Text(b)
                    } else {
                        FieldVal::Missing
                    }
                }
                "gated" => match lic.and_then(|l| l.get("gated")).and_then(|x| x.as_bool()) {
                    Some(b) => FieldVal::Flag(b),
                    None => FieldVal::Missing,
                },
                other => {
                    let s = json_str(lic.and_then(|l| l.get(other)));
                    if s.is_empty() {
                        FieldVal::Missing
                    } else {
                        FieldVal::Text(s)
                    }
                }
            };
            (*label, val)
        })
        .collect()
}

/// 能力断言逐条：(name, value, source, evidence, engines)。
/// engines（待修补 #39）是这条断言被**证过成立**的引擎——回答"这能力是在哪个引擎上测出来的"；
/// 去空白、丢空串、保序、去重；缺 / 空数组 → 空 vec（展示层显「—」，绝不填占位）。
fn cap_rows(rec: &serde_json::Value) -> Vec<(String, FieldVal, String, String, Vec<String>)> {
    rec.get("capabilities")
        .and_then(|c| c.as_array())
        .map(|arr| {
            arr.iter()
                .map(|c| {
                    let name = json_str(c.get("name"));
                    let val = match c.get("value").and_then(|x| x.as_bool()) {
                        Some(b) => FieldVal::Flag(b),
                        None => FieldVal::Missing,
                    };
                    let src = or_dash(json_str(c.get("source")));
                    let ev = or_dash(json_str(c.get("evidence")));
                    let engines = engine_list(c.get("engines"));
                    (if name.is_empty() { "?".to_string() } else { name }, val, src, ev, engines)
                })
                .collect()
        })
        .unwrap_or_default()
}

/// 从一条能力的 engines 字段取引擎列表：去空白、丢空串、保序、去重（缺 / 非数组 → 空 vec）。
fn engine_list(v: Option<&serde_json::Value>) -> Vec<String> {
    let mut out: Vec<String> = Vec::new();
    if let Some(arr) = v.and_then(|x| x.as_array()) {
        for e in arr {
            let s = e.as_str().unwrap_or("").trim();
            if !s.is_empty() && !out.iter().any(|x| x == s) {
                out.push(s.to_string());
            }
        }
    }
    out
}

/// 引擎列表展示：多个用「, 」连接；空列表 → 「—」（缺值不是错误，不是占位）。
fn engines_display(engines: &[String]) -> String {
    if engines.is_empty() {
        DASH.to_string()
    } else {
        engines.join(", ")
    }
}

/// 不可判定能力逐条（待修补 #39）：(name, reason 原文码, evidence)。
/// 三个字段照抄接口（name/reason/evidence 一字不改）；缺项显「—」。
/// reason 保留原文码（证据完整性），展示用 uv_reason_label 给通俗标签。
fn unverifiable_rows(rec: &serde_json::Value) -> Vec<(String, String, String)> {
    rec.get("unverifiable")
        .and_then(|u| u.as_array())
        .map(|arr| {
            arr.iter()
                .map(|u| {
                    let name = json_str(u.get("name"));
                    (
                        if name.is_empty() { "?".to_string() } else { name },
                        json_str(u.get("reason")),
                        or_dash(json_str(u.get("evidence"))),
                    )
                })
                .collect()
        })
        .unwrap_or_default()
}

/// 不可判定原因 → 通俗文案（i18n 键）；未知原因码**原样回退**（不猜、不改写原因分类）。
fn uv_reason_label(reason: &str) -> String {
    match reason.trim() {
        "budget_exhausted" => t!("mreg.uv_reason_budget").to_string(),
        "timeout" => t!("mreg.uv_reason_timeout").to_string(),
        "" => DASH.to_string(),
        other => other.to_string(),
    }
}

/// 建材逐项：(role, name, sha256, size 人类可读)——缺项「—」，size 缺失显「—」（不是 0）
fn file_rows(rec: &serde_json::Value) -> Vec<(String, String, String, String)> {
    rec.get("files")
        .and_then(|f| f.as_array())
        .map(|arr| {
            arr.iter()
                .map(|f| {
                    let size = match f.get("size").and_then(|x| x.as_i64()) {
                        Some(n) => human_size(n),
                        None => DASH.to_string(),
                    };
                    (
                        or_dash(json_str(f.get("role"))),
                        or_dash(json_str(f.get("name"))),
                        or_dash(json_str(f.get("sha256"))),
                        size,
                    )
                })
                .collect()
        })
        .unwrap_or_default()
}

/// 引擎配方名列表（接口已按字典序排好——不再排序，保持与接口一致）
fn recipe_rows(rec: &serde_json::Value) -> Vec<String> {
    rec.get("engine_recipes")
        .and_then(|r| r.as_array())
        .map(|arr| {
            arr.iter()
                .filter_map(|x| x.as_str())
                .map(|s| s.trim().to_string())
                .filter(|s| !s.is_empty())
                .collect()
        })
        .unwrap_or_default()
}

// ───────────────────────── 页面 ─────────────────────────

/// 页面入口（app.rs 的 main_view 分派调用——不进 App 结构体，照 upgrade.rs 的既有写法）
pub fn ui(ui: &mut egui::Ui) {
    pump();
    // 首次进页自动拉取；失败后停在错误态，等用户点「重试」
    if !has_data() && !loading() {
        start_fetch();
    }

    ui.horizontal(|ui| {
        // 2026-09-13（设计《UI 大调动》§4.4）：删掉重复标题块（heading）——保留 loading 与「刷新」按钮。
        if loading() {
            ui.label(RichText::new(t!("mreg.loading").to_string()).italics());
        }
        ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
            if ui.button(t!("mreg.refresh").to_string()).clicked() {
                retry();
            }
        });
    });
    ui.weak(t!("mreg.hint").to_string());
    ui.add_space(6.0);
    ui.separator();

    // 三态分派：加载中 / 失败（可重试） / 有数据
    let data = DATA.lock().map(|d| d.clone()).unwrap_or(None);
    match data {
        None => {
            ui.add_space(24.0);
            ui.vertical_centered(|ui| {
                ui.spinner();
                ui.weak(t!("mreg.loading").to_string());
            });
        }
        Some(Err(e)) => {
            ui.add_space(16.0);
            ui.colored_label(
                egui::Color32::from_rgb(220, 80, 80),
                format!("{} {}", icon_text("warning"), t!("mreg.error_title")),
            );
            ui.label(RichText::new(e).monospace());
            ui.add_space(8.0);
            if ui.button(t!("mreg.retry").to_string()).clicked() {
                retry();
            }
        }
        Some(Ok(v)) => {
            // 控件层（视图切换 / 搜索 / 能力芯片）——只有这一层写状态
            controls(ui, &v, current_view());
            ui.add_space(4.0);
            ui.separator();
            ui.add_space(4.0);
            // 内容层 + 详情层——纯读状态，动作回填
            render_registry(ui, &v);
        }
    }
}

/// 控件层：视图切换 / 搜索框 / 能力芯片。
/// 借用纪律：闭包内只写局部变量，所有状态变更等布局调用返回后再做（照 references/ui-rust-egui.md）。
fn controls(ui: &mut egui::Ui, v: &serde_json::Value, view: View) {
    let mut new_view: Option<View> = None;
    let mut new_search: Option<String> = None;
    let mut new_cap: Option<Option<String>> = None;
    let mut search = search_state();

    ui.horizontal_wrapped(|ui| {
        ui.label(RichText::new(t!("mreg.view_label").to_string()).small());
        if ui
            .selectable_label(view == View::Capability, t!("mreg.view_capability").to_string())
            .clicked()
        {
            new_view = Some(View::Capability);
        }
        if ui
            .selectable_label(view == View::Engineering, t!("mreg.view_engineering").to_string())
            .clicked()
        {
            new_view = Some(View::Engineering);
        }
        ui.separator();
        ui.label(RichText::new(t!("mreg.search_label").to_string()).small());
        let resp = ui.add(
            egui::TextEdit::singleline(&mut search)
                .desired_width(220.0)
                .hint_text(t!("mreg.search_placeholder").to_string()),
        );
        if resp.changed() {
            new_search = Some(search.clone());
        }
        if !search.is_empty() && ui.button(t!("mreg.search_clear").to_string()).clicked() {
            new_search = Some(String::new());
        }
    });

    // 能力芯片：只长在能力视图上（工程视图里不显示芯片，也就不用「看不见的筛选」坑人）
    if view == View::Capability {
        let chips = capability_chips(record_list(v));
        if !chips.is_empty() {
            let cur = cap_state();
            let cur_s = cur.clone();
            ui.horizontal_wrapped(|ui| {
                ui.label(RichText::new(t!("mreg.filter_label").to_string()).small());
                if ui
                    .selectable_label(cur_s.is_none(), t!("mreg.filter_all").to_string())
                    .clicked()
                {
                    new_cap = Some(None);
                }
                for c in &chips {
                    let on = cur_s.as_deref() == Some(c.as_str());
                    if ui.selectable_label(on, c.clone()).clicked() {
                        new_cap = Some(chip_toggle(cur_s.as_deref(), c));
                    }
                }
            });
        }
    }

    ui.weak(RichText::new(t!("mreg.detail_hint").to_string()).small());

    // 布局已返回——现在才改状态（改视图顺带落盘：记住选择）
    if let Some(nv) = new_view {
        if nv != view {
            set_view(nv);
            save_view(nv);
        }
    }
    if let Some(s) = new_search {
        set_search(s);
    }
    if let Some(c) = new_cap {
        set_cap(c);
    }
}

/// 内容层 / 详情层的渲染输入（显式传参——测试不依赖全局状态，避免测试间互相干扰）
struct ViewState<'a> {
    view: View,
    search: &'a str,
    cap: Option<&'a str>,
    selected: Option<&'a str>,
    copied: Option<&'a str>,
}

/// 渲染动作（内容层「只提议、不落地」——由调用方在布局返回后应用）
#[derive(Clone, PartialEq, Debug)]
enum MregAction {
    None,
    /// 展开某条记录的详情
    Open(String),
    /// 关闭详情层
    Close,
    /// 复制某路径到剪贴板
    Copy(String),
}

/// 便利入口（读全局状态、应用动作）——ui() 与既有单测都走它
fn render_registry(ui: &mut egui::Ui, v: &serde_json::Value) {
    let search = search_state();
    let cap = cap_state();
    let selected = selected_state();
    let copied = copied_state();
    let st = ViewState {
        view: current_view(),
        search: &search,
        cap: cap.as_deref(),
        selected: selected.as_deref(),
        copied: copied.as_deref(),
    };
    let action = render_registry_with(ui, v, &st);
    apply_action(ui, action);
}

/// 动作落地（布局之后调用——egui 借用纪律）
fn apply_action(ui: &mut egui::Ui, action: MregAction) {
    match action {
        MregAction::Open(k) => set_selected(Some(k)),
        MregAction::Close => set_selected(None),
        MregAction::Copy(p) => {
            // 复制路径：egui 0.36 的 ctx 级剪贴板接口（与 app.rs / chat_view.rs 同一写法）
            ui.ctx().copy_text(p.clone());
            set_copied(Some(p));
        }
        MregAction::None => {}
    }
}

/// 有数据：摘要 + 空态 / 卡片列表（能力视图）或紧凑表格（工程视图） + 详情层
fn render_registry_with(ui: &mut egui::Ui, v: &serde_json::Value, st: &ViewState<'_>) -> MregAction {
    let count = v.get("count").and_then(|c| c.as_u64()).unwrap_or(0);
    let bad = v.get("bad_records").and_then(|c| c.as_u64()).unwrap_or(0);
    let root = v.get("root").and_then(|c| c.as_str()).unwrap_or("");
    let all = record_list(v);

    ui.horizontal_wrapped(|ui| {
        ui.label(t!("mreg.count", count = count, bad = bad).to_string());
        if !root.is_empty() {
            ui.weak(RichText::new(root).monospace());
        }
    });
    if bad > 0 {
        ui.colored_label(
            egui::Color32::from_rgb(240, 200, 80),
            t!("mreg.bad_banner", n = bad).to_string(),
        );
    }

    if all.is_empty() {
        // 空态：友好提示 + 一行「怎么加模型」（引导到 zerg-model probe --store）
        ui.add_space(28.0);
        ui.vertical_centered(|ui| {
            ui.heading(t!("mreg.empty_title").to_string());
            ui.add_space(6.0);
            ui.label(t!("mreg.empty_hint").to_string());
        });
        return MregAction::None;
    }

    // 能力筛选只在能力视图生效（芯片只在能力视图可见——避免"看不见的筛选"）
    let cap_eff = if st.view == View::Capability { st.cap } else { None };
    let shown = filter_records(all, st.search, cap_eff);

    ui.add_space(6.0);
    let mut action = MregAction::None;
    if shown.is_empty() {
        // 无匹配：说清是"筛选"造成的，不是"没有模型"
        ui.add_space(16.0);
        ui.vertical_centered(|ui| {
            ui.heading(t!("mreg.no_match").to_string());
            ui.add_space(6.0);
            ui.weak(RichText::new(t!("mreg.no_match_hint").to_string()).small());
        });
    } else {
        if shown.len() != all.len() {
            ui.weak(
                RichText::new(t!("mreg.filtered_count", shown = shown.len(), total = all.len()).to_string())
                    .small(),
            );
        }
        match st.view {
            View::Capability => {
                egui::ScrollArea::vertical()
                    .auto_shrink([false, false])
                    .show(ui, |ui| {
                        for rec in &shown {
                            let key = rec_key(rec);
                            let is_sel = st.selected == Some(key.as_str());
                            if card(ui, rec, is_sel) {
                                action = MregAction::Open(key);
                            }
                            ui.add_space(6.0);
                        }
                    });
            }
            View::Engineering => {
                action = table(ui, &shown, st.selected);
            }
        }
    }

    // 详情层（弹层）——选中项在**全量**里找（改搜索词不该把已打开的详情弄没）
    if let Some(sel) = st.selected {
        if let Some(rec) = all.iter().find(|r| rec_key(r) == sel) {
            let d = detail_window(ui.ctx().clone(), rec, st.copied);
            if d != MregAction::None {
                action = d; // 弹层在顶层——它的动作优先
            }
        }
    }
    action
}

/// 单个模型卡片（能力视图）。返回是否被点击（点卡片 = 展开详情）。
fn card(ui: &mut egui::Ui, rec: &serde_json::Value, is_selected: bool) -> bool {
    let id = rec.get("id").and_then(|x| x.as_str()).unwrap_or("?");
    let version = rec.get("version").and_then(|x| x.as_str()).unwrap_or("");
    let name = display_name(rec);
    let commercial = rec.get("commercial").and_then(|x| x.as_str()).unwrap_or("");
    let ctx = rec.get("context_window").and_then(|x| x.as_i64()).unwrap_or(0);
    let errors = rec.get("errors").and_then(|x| x.as_u64()).unwrap_or(0);
    let warns = rec.get("warns").and_then(|x| x.as_u64()).unwrap_or(0);
    let default_eligible = rec.get("default_eligible").and_then(|x| x.as_bool()).unwrap_or(false);
    let err_msg = rec.get("error").and_then(|x| x.as_str()).unwrap_or("");
    let files = rec.get("files").and_then(|x| x.as_array());
    let caps = rec.get("capabilities").and_then(|x| x.as_array());

    let (lk, color) = license_badge(commercial);
    let (file_count, file_bytes) = file_summary(files);

    let inner = ui.group(|ui| {
        // 第一行：显示名 + id + 版本 + 许可徽标 + 默认资格
        ui.horizontal_wrapped(|ui| {
            if is_selected {
                ui.colored_label(egui::Color32::from_rgb(120, 180, 255), "▸");
            }
            ui.strong(name.as_str());
            if name != id {
                ui.weak(RichText::new(id).monospace());
            }
            if !version.is_empty() {
                ui.weak(RichText::new(format!("v{}", version)).monospace());
            }
            ui.colored_label(color, format!("● {}", t!(lk)));
            if default_eligible {
                ui.colored_label(
                    egui::Color32::from_rgb(80, 200, 120),
                    format!("✓ {}", t!("mreg.default_ok")),
                );
            } else {
                ui.weak(t!("mreg.default_no").to_string());
            }
        });

        // 第二行：能力标签（capabilities[].name——必要时带 source 小字）
        ui.horizontal_wrapped(|ui| {
            match caps {
                Some(arr) if !arr.is_empty() => {
                    for c in arr {
                        let cn = c.get("name").and_then(|x| x.as_str()).unwrap_or("");
                        if cn.is_empty() {
                            continue;
                        }
                        let src = c.get("source").and_then(|x| x.as_str()).unwrap_or("");
                        let label = if src.is_empty() {
                            cn.to_string()
                        } else {
                            t!("mreg.cap_tag", name = cn, src = src).to_string()
                        };
                        ui.label(RichText::new(label).small());
                    }
                }
                _ => {
                    ui.weak(t!("mreg.no_caps").to_string());
                }
            }
        });

        // 第三行：建材（文件数 + 合计体积） + 上下文窗口
        ui.horizontal_wrapped(|ui| {
            ui.label(t!("mreg.files", count = file_count, size = human_size(file_bytes)).to_string());
            if ctx > 0 {
                ui.label(t!("mreg.context", ctx = ctx).to_string());
            } else {
                ui.weak(t!("mreg.context_unknown").to_string());
            }
        });

        // 第四行：错误 / 告警计数（>0 醒目）；坏记录额外给同情式说明 + 原始原因
        ui.horizontal_wrapped(|ui| {
            if errors > 0 {
                ui.colored_label(
                    egui::Color32::from_rgb(220, 80, 80),
                    format!("{} {}", icon_text("warning"), t!("mreg.errors", n = errors)),
                );
            }
            if warns > 0 {
                ui.colored_label(
                    egui::Color32::from_rgb(240, 200, 80),
                    t!("mreg.warns", n = warns).to_string(),
                );
            }
        });
        if !err_msg.is_empty() {
            ui.colored_label(
                egui::Color32::from_rgb(230, 150, 90),
                t!("mreg.bad_hint").to_string(),
            );
            ui.label(RichText::new(err_msg).monospace().small());
        }
    });

    // 整张卡片可点（group 默认不感知点击——补一个 click sense）
    let resp = inner
        .response
        .interact(egui::Sense::click())
        .on_hover_cursor(egui::CursorIcon::PointingHand);
    resp.clicked()
}

/// 工程视图：紧凑表格（列宽固定 + 长文本截断），点行 = 展开详情。
/// 目标是「工程排障一眼看全」：id / 版本 / digest(12) / 文件数与体积 / 许可 / 错误告警 / 路径
fn table(ui: &mut egui::Ui, rows: &[&serde_json::Value], selected: Option<&str>) -> MregAction {
    let mut action = MregAction::None;
    let mut clicked: Option<String> = None;
    let headers = [
        "mreg.col_id",
        "mreg.col_version",
        "mreg.col_digest12",
        "mreg.col_files",
        "mreg.col_license",
        "mreg.col_issues",
        "mreg.col_path",
    ];

    egui_extras::TableBuilder::new(ui)
        .id_salt("mreg_eng_table")
        .striped(true)
        .resizable(false)
        .sense(egui::Sense::click())
        .cell_layout(egui::Layout::left_to_right(egui::Align::Center))
        .column(egui_extras::Column::exact(170.0))
        .column(egui_extras::Column::exact(80.0))
        .column(egui_extras::Column::exact(110.0))
        .column(egui_extras::Column::exact(140.0))
        .column(egui_extras::Column::exact(80.0))
        .column(egui_extras::Column::exact(110.0))
        .column(egui_extras::Column::remainder().clip(true))
        .header(20.0, |mut header| {
            for h in headers {
                header.col(|ui| {
                    ui.strong(t!(h).to_string());
                });
            }
        })
        .body(|mut body| {
            for rec in rows {
                let key = rec_key(rec);
                let is_sel = selected == Some(key.as_str());
                let id = rec.get("id").and_then(|x| x.as_str()).unwrap_or("?");
                let version = rec.get("version").and_then(|x| x.as_str()).unwrap_or("");
                let digest = rec.get("digest").and_then(|x| x.as_str()).unwrap_or("");
                let commercial = rec.get("commercial").and_then(|x| x.as_str()).unwrap_or("");
                let errors = rec.get("errors").and_then(|x| x.as_u64()).unwrap_or(0);
                let warns = rec.get("warns").and_then(|x| x.as_u64()).unwrap_or(0);
                let path = rec.get("path").and_then(|x| x.as_str()).unwrap_or("");
                let (file_count, file_bytes) = file_summary(rec.get("files").and_then(|x| x.as_array()));
                let (lk, color) = license_badge(commercial);

                body.row(20.0, |mut row| {
                    row.set_selected(is_sel);
                    row.col(|ui| {
                        ui.label(RichText::new(truncate_chars(id, 24)).monospace().small());
                    });
                    row.col(|ui| {
                        ui.label(RichText::new(truncate_chars(version, 12)).monospace().small());
                    });
                    row.col(|ui| {
                        ui.label(RichText::new(short_digest(digest)).monospace().small());
                    });
                    row.col(|ui| {
                        ui.label(
                            RichText::new(format!("{} · {}", file_count, human_size(file_bytes))).small(),
                        );
                    });
                    row.col(|ui| {
                        ui.colored_label(color, RichText::new(t!(lk).to_string()).small());
                    });
                    row.col(|ui| {
                        if errors > 0 {
                            ui.colored_label(
                                egui::Color32::from_rgb(220, 80, 80),
                                RichText::new(format!("{} / {}", errors, warns)).small(),
                            );
                        } else {
                            ui.label(RichText::new(format!("0 / {}", warns)).small());
                        }
                    });
                    row.col(|ui| {
                        ui.label(RichText::new(truncate_chars(path, 40)).monospace().small());
                    });
                    if row.response().clicked() && clicked.is_none() {
                        clicked = Some(key.clone());
                    }
                });
            }
        });

    if let Some(k) = clicked {
        action = MregAction::Open(k);
    }
    action
}

/// 详情层（弹层）：完整信息一览——能力断言（source/evidence）、建材逐项、引擎配方、
/// license 全字段（缺值「—」）、digest 全文、路径 + 复制路径按钮。
fn detail_window(ctx: egui::Context, rec: &serde_json::Value, copied: Option<&str>) -> MregAction {
    let mut action = MregAction::None;
    let mut open = true;
    let key = rec_key(rec);
    let id = json_str(rec.get("id"));
    let version = json_str(rec.get("version"));
    let name = display_name(rec);
    let path = json_str(rec.get("path"));
    let digest = json_str(rec.get("digest"));
    let commercial = json_str(rec.get("commercial"));
    let ctx_win = rec.get("context_window").and_then(|x| x.as_i64()).unwrap_or(0);
    let errors = rec.get("errors").and_then(|x| x.as_u64()).unwrap_or(0);
    let warns = rec.get("warns").and_then(|x| x.as_u64()).unwrap_or(0);
    let default_eligible = rec.get("default_eligible").and_then(|x| x.as_bool()).unwrap_or(false);
    let err_msg = json_str(rec.get("error"));
    let (file_count, file_bytes) = file_summary(rec.get("files").and_then(|x| x.as_array()));
    let (lk, color) = license_badge(&commercial);
    let is_copied = !path.is_empty() && copied == Some(path.as_str());

    egui::Window::new(t!("mreg.detail_title").to_string())
        .id(egui::Id::new(("mreg_detail", key)))
        .open(&mut open)
        .collapsible(false)
        .resizable(true)
        .default_width(620.0)
        .default_height(520.0)
        .show(&ctx, |ui| {
            // 抬头：显示名 + id + 版本 + 许可徽标
            ui.horizontal_wrapped(|ui| {
                ui.strong(name.as_str());
                ui.weak(RichText::new(id.as_str()).monospace());
                if !version.is_empty() {
                    ui.weak(RichText::new(format!("v{}", version)).monospace());
                }
                ui.colored_label(color, format!("● {}", t!(lk)));
                if default_eligible {
                    ui.colored_label(
                        egui::Color32::from_rgb(80, 200, 120),
                        format!("✓ {}", t!("mreg.default_ok")),
                    );
                } else {
                    ui.weak(t!("mreg.default_no").to_string());
                }
            });
            ui.separator();

            // 概览（键 = 值 两列）
            egui::Grid::new("mreg_detail_meta")
                .num_columns(2)
                .spacing([12.0, 4.0])
                .show(ui, |ui| {
                    ui.weak(t!("mreg.detail_name").to_string());
                    ui.label(name.as_str());
                    ui.end_row();
                    ui.weak(t!("mreg.detail_id").to_string());
                    ui.label(RichText::new(if id.is_empty() { DASH } else { id.as_str() }).monospace());
                    ui.end_row();
                    ui.weak(t!("mreg.detail_version").to_string());
                    ui.label(RichText::new(if version.is_empty() { DASH } else { version.as_str() }).monospace());
                    ui.end_row();
                    ui.weak(t!("mreg.detail_context").to_string());
                    ui.label(if ctx_win > 0 {
                        t!("mreg.context", ctx = ctx_win).to_string()
                    } else {
                        t!("mreg.context_unknown").to_string()
                    });
                    ui.end_row();
                    ui.weak(t!("mreg.detail_files").to_string());
                    ui.label(t!("mreg.files", count = file_count, size = human_size(file_bytes)).to_string());
                    ui.end_row();
                    ui.weak(t!("mreg.detail_issues").to_string());
                    ui.label(t!("mreg.errors", n = errors).to_string() + " · " + &t!("mreg.warns", n = warns));
                    ui.end_row();
                    ui.weak(t!("mreg.detail_digest").to_string());
                    ui.label(RichText::new(or_dash(digest.clone())).monospace().small());
                    ui.end_row();
                    ui.weak(t!("mreg.detail_path").to_string());
                    ui.label(RichText::new(or_dash(path.clone())).monospace().small());
                    ui.end_row();
                });

            // 复制路径（egui 0.36：ui.ctx().copy_text —— 与 app.rs 复制任务描述同一写法）
            if !path.is_empty() {
                ui.horizontal(|ui| {
                    if ui
                        .button(format!("{} {}", icon_text("copy"), t!("mreg.copy_path")))
                        .clicked()
                    {
                        action = MregAction::Copy(path.clone());
                    }
                    if is_copied {
                        ui.weak(t!("mreg.copied").to_string());
                    }
                });
            }

            // 坏记录原始错误（有才显示）
            if !err_msg.is_empty() {
                ui.colored_label(
                    egui::Color32::from_rgb(230, 150, 90),
                    t!("mreg.detail_raw_error").to_string(),
                );
                ui.label(RichText::new(err_msg).monospace().small());
            }

            // 许可证全字段（缺值「—」——空值不是错误）
            ui.add_space(8.0);
            ui.strong(t!("mreg.detail_license").to_string());
            egui::Grid::new("mreg_detail_license")
                .num_columns(2)
                .spacing([12.0, 2.0])
                .show(ui, |ui| {
                    for (label, val) in license_rows(rec) {
                        ui.weak(t!(label).to_string());
                        ui.label(RichText::new(val.display()).small());
                        ui.end_row();
                    }
                });

            // 能力断言：name / 取值 / 来源 / 证据 / 实测引擎（证据是"凭什么这么说"——标准 §四；
            // 实测引擎回答"这能力是在哪个引擎上测出来的"——待修补 #39）
            ui.add_space(8.0);
            ui.strong(t!("mreg.detail_caps").to_string());
            let caps = cap_rows(rec);
            if caps.is_empty() {
                ui.weak(t!("mreg.detail_no_caps").to_string());
            } else {
                egui::Grid::new("mreg_detail_caps")
                    .num_columns(5)
                    .spacing([12.0, 2.0])
                    .striped(true)
                    .show(ui, |ui| {
                        ui.weak(t!("mreg.col_cap_name").to_string());
                        ui.weak(t!("mreg.col_cap_value").to_string());
                        ui.weak(t!("mreg.col_cap_source").to_string());
                        ui.weak(t!("mreg.col_cap_evidence").to_string());
                        ui.weak(t!("mreg.col_cap_engines").to_string());
                        ui.end_row();
                        for (name, val, src, ev, engines) in caps {
                            ui.label(RichText::new(name).small());
                            ui.label(RichText::new(val.display()).small());
                            ui.label(RichText::new(src).small());
                            ui.label(RichText::new(ev).small());
                            ui.label(RichText::new(engines_display(&engines)).small());
                            ui.end_row();
                        }
                    });
            }

            // 无法判定的能力（待修补 #39）：预算不够 / 超时 → 那时没探出结论（不是"没有"）。
            // 通俗标题 + 通俗原因 + 原始证据：既让人看懂，也保留可追责的原文码。
            ui.add_space(8.0);
            ui.strong(t!("mreg.detail_unverifiable").to_string());
            let uvs = unverifiable_rows(rec);
            if uvs.is_empty() {
                ui.weak(t!("mreg.uv_none").to_string());
            } else {
                egui::Grid::new("mreg_detail_unverifiable")
                    .num_columns(3)
                    .spacing([12.0, 2.0])
                    .striped(true)
                    .show(ui, |ui| {
                        ui.weak(t!("mreg.col_uv_name").to_string());
                        ui.weak(t!("mreg.col_uv_reason").to_string());
                        ui.weak(t!("mreg.col_uv_evidence").to_string());
                        ui.end_row();
                        for (name, reason, ev) in uvs {
                            ui.label(RichText::new(name).small());
                            ui.label(RichText::new(uv_reason_label(&reason)).small());
                            ui.label(RichText::new(ev).small());
                            ui.end_row();
                        }
                    });
            }

            // 建材逐项：role / name / sha256 / size
            ui.add_space(8.0);
            ui.strong(t!("mreg.detail_files").to_string());
            let files = file_rows(rec);
            if files.is_empty() {
                ui.weak(t!("mreg.detail_no_files").to_string());
            } else {
                egui::Grid::new("mreg_detail_files")
                    .num_columns(4)
                    .spacing([12.0, 2.0])
                    .striped(true)
                    .show(ui, |ui| {
                        ui.weak(t!("mreg.col_file_role").to_string());
                        ui.weak(t!("mreg.col_file_name").to_string());
                        ui.weak(t!("mreg.col_file_sha").to_string());
                        ui.weak(t!("mreg.col_file_size").to_string());
                        ui.end_row();
                        for (role, fname, sha, size) in files {
                            ui.label(RichText::new(role).small());
                            ui.label(RichText::new(fname).small());
                            ui.label(RichText::new(sha).monospace().small());
                            ui.label(RichText::new(size).small());
                            ui.end_row();
                        }
                    });
            }

            // 引擎配方
            ui.add_space(8.0);
            ui.strong(t!("mreg.detail_recipes").to_string());
            let recipes = recipe_rows(rec);
            if recipes.is_empty() {
                ui.weak(t!("mreg.detail_no_recipes").to_string());
            } else {
                ui.horizontal_wrapped(|ui| {
                    for r in recipes {
                        ui.label(RichText::new(r).small());
                    }
                });
            }
        });

    if !open {
        return MregAction::Close;
    }
    action
}

#[cfg(test)]
mod tests {
    use super::{display_name, file_summary, human_size, license_badge};
    use serde_json::json;

    /// 许可徽标取值映射（设计 §1.4 的四色口径）
    #[test]
    fn test_license_badge_mapping() {
        assert_eq!(license_badge("yes").0, "mreg.license_yes");
        assert_eq!(license_badge("no").0, "mreg.license_no");
        assert_eq!(license_badge("revenue_gated").0, "mreg.license_revenue_gated");
        assert_eq!(license_badge("unknown").0, "mreg.license_unknown");
        // 空/陌生取值一律按「待核」——绝不默认成可商用
        assert_eq!(license_badge("").0, "mreg.license_unknown");
        assert_eq!(license_badge("whatever").0, "mreg.license_unknown");
    }

    /// 显示名：缺 name 退回 id，两者都缺退回占位符
    #[test]
    fn test_display_name_fallback() {
        assert_eq!(display_name(&json!({"id": "m1", "name": "好模型"})), "好模型");
        assert_eq!(display_name(&json!({"id": "m1", "name": ""})), "m1");
        assert_eq!(display_name(&json!({"id": "m1"})), "m1");
        assert_eq!(display_name(&json!({})), "?");
    }

    /// 体积格式化（二进制单位）
    #[test]
    fn test_human_size() {
        assert_eq!(human_size(0), "0 B");
        assert_eq!(human_size(512), "512 B");
        assert_eq!(human_size(1024), "1.0 KB");
        assert_eq!(human_size(1536), "1.5 KB");
        assert_eq!(human_size(1024 * 1024), "1.0 MB");
        assert_eq!(human_size(1024 * 1024 * 1024), "1.00 GB");
        // 负值（脏数据）不 panic——归零
        assert_eq!(human_size(-5), "0 B");
    }

    /// 建材汇总：数量与合计；缺字段/非数组归零
    #[test]
    fn test_file_summary() {
        let files = vec![
            json!({"role": "weights", "size": 1000}),
            json!({"role": "mmproj", "size": 500}),
        ];
        assert_eq!(file_summary(Some(&files)), (2, 1500));
        // size 缺失按 0 计，不 panic
        let partial = vec![json!({"role": "weights"})];
        assert_eq!(file_summary(Some(&partial)), (1, 0));
        assert_eq!(file_summary(None), (0, 0));
    }

    /// 渲染路径不 panic（空态 / 好记录+坏记录混合）——坏记录不得弄坏整页。
    /// 无头 egui（0.36 入口 run_ui；每帧 clear 纹理增量，否则 drop panic——照 M06 基准测试写法）
    #[test]
    fn test_render_paths_do_not_panic() {
        let empty = json!({"count": 0, "bad_records": 0, "root": "/tmp/none", "records": []});
        // 完整记录：许可/能力/建材/上下文/默认资格俱全
        let good = json!({
            "id": "qwen3-vl-8b", "version": "2026-09-01", "name": "Qwen3-VL-8B",
            "commercial": "yes",
            "capabilities": [{"name": "vision", "value": true, "source": "probed"},
                             {"name": "tools", "value": true, "source": "declared"}],
            "files": [{"role": "weights", "name": "a.gguf", "sha256": "ab", "size": 2048},
                      {"role": "mmproj", "name": "m.gguf", "sha256": "cd", "size": 512}],
            "context_window": 32768, "engine_recipes": ["llama.cpp"],
            "errors": 0, "warns": 1, "default_eligible": true
        });
        // 坏记录：只有 id/version + errors/error（其余字段缺省——照后端坏记录形状）；
        // 许可缺失/非口径取值也必须退化成「待核」而不是崩
        let bad = json!({"id": "broken", "version": "?", "errors": 1,
                         "error": "unexpected end of JSON input"});
        let mixed = json!({"count": 2, "bad_records": 1, "root": "/root",
                           "records": [good, bad]});

        let ctx = egui::Context::default();
        for payload in [&empty, &mixed] {
            let mut out = ctx.run_ui(Default::default(), |ui| super::render_registry(ui, payload));
            out.textures_delta.clear();
        }
    }
}

/// 本批（视图 / 筛选 / 搜索 / 详情层）的纯函数与渲染路径单测。
/// 独立成模块（不动上面既有 5 个用例），渲染一律走显式传参的 render_registry_with，
/// 不碰全局状态——避免测试之间互相干扰（同一进程并行跑）。
#[cfg(test)]
mod view_tests {
    use super::*;
    use serde_json::json;

    /// 示例快照：两条好记录（能力/建材/配方/路径俱全）+ 一条坏记录
    fn sample() -> serde_json::Value {
        json!({
            "count": 3, "bad_records": 1, "root": "/tmp/models",
            "records": [
                {
                    "id": "qwen3-vl-8b", "version": "2026-09-01", "name": "Qwen3-VL-8B",
                    "path": "/tmp/models/manifests/qwen3-vl-8b/2026-09-01.json",
                    "digest": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
                    "commercial": "yes", "context_window": 32768,
                    "capabilities": [
                        {"name": "vision", "value": true, "source": "probed", "evidence": "probe.vision.1x1.v1 ok",
                         "engines": ["llama.cpp", "vllm"]},
                        {"name": "text", "value": true, "source": "probed"}
                    ],
                    "unverifiable": [
                        {"name": "embedding", "reason": "budget_exhausted",
                         "evidence": "probe.embedding.v1 budget=2048 (budget_exhausted: truncated)"}
                    ],
                    "license": {"spdx": "apache-2.0", "commercial": "yes", "gated": false,
                                "evidence": "probe.license.v1 (gguf_key: general.license=\"apache-2.0\")"},
                    "files": [{"role": "weights", "name": "a.gguf", "sha256": "aa11", "size": 2048},
                              {"role": "mmproj", "name": "m.gguf", "sha256": "bb22", "size": 512}],
                    "engine_recipes": ["llama.cpp", "vllm"],
                    "errors": 0, "warns": 1, "default_eligible": true
                },
                {
                    "id": "kokoro-82m", "version": "v1", "name": "",
                    "path": "/tmp/models/manifests/kokoro-82m/v1.json",
                    "commercial": "revenue_gated", "context_window": 0,
                    "capabilities": [{"name": "text", "value": true, "source": "declared"}],
                    "files": [{"role": "weights", "size": 100}],
                    "errors": 0, "warns": 0, "default_eligible": false
                },
                {
                    "id": "broken", "version": "?", "path": "/tmp/models/manifests/broken/x.json",
                    "errors": 1, "error": "unexpected end of JSON input"
                }
            ]
        })
    }

    /// 视图偏好的纯解析：显式值 / 大小写 / 坏 JSON / 陌生取值
    #[test]
    fn test_view_pref_parse() {
        assert_eq!(parse_view_pref(r#"{"mreg_view":"engineering"}"#), Some(View::Engineering));
        assert_eq!(parse_view_pref(r#"{"mreg_view":"Capability"}"#), Some(View::Capability));
        // 坏 JSON / 缺字段 / 陌生取值 → None（用默认，不猜）
        assert_eq!(parse_view_pref("{oops"), None);
        assert_eq!(parse_view_pref("{}"), None);
        assert_eq!(parse_view_pref(r#"{"mreg_view":"表格视图"}"#), None);
        // 默认视图 = 能力视图（设计 §十二#1 拍板）
        assert_eq!(View::Capability.as_str(), "capability");
        assert_eq!(View::Engineering.as_str(), "engineering");
    }

    /// 写回偏好必须**保留其它字段**（locale/ai_model…）——整文件覆盖会吃掉用户偏好
    #[test]
    fn test_merge_view_pref_preserves_other_keys() {
        let merged = merge_view_pref(r#"{"locale":"en","ai_model":"gemma"}"#, View::Engineering);
        let v: serde_json::Value = serde_json::from_str(&merged).unwrap();
        assert_eq!(v["mreg_view"], json!("engineering"));
        assert_eq!(v["locale"], json!("en"));
        assert_eq!(v["ai_model"], json!("gemma"));
        // 坏 JSON / 非对象 → 不 panic，重建成只带本字段的对象
        for bad in ["not json", "[]", ""] {
            let m = merge_view_pref(bad, View::Capability);
            let v: serde_json::Value = serde_json::from_str(&m).unwrap();
            assert_eq!(v["mreg_view"], json!("capability"));
        }
    }

    /// 能力芯片：只收 value=true 的、去重、排序（从数据动态生成，不写死全表）
    #[test]
    fn test_capability_chips_dynamic_true_only_sorted() {
        let v = sample();
        let chips = capability_chips(record_list(&v));
        // text 出现两次只留一个；vision 在前（字典序）；坏记录无 chips 不崩
        assert_eq!(chips, vec!["text".to_string(), "vision".to_string()]);
        // value=false 的能力不当芯片（点开必然是空列表——不骗人）
        let only_false = json!({"records": [{"id": "x",
            "capabilities": [{"name": "audio_out", "value": false, "source": "probed"}]}]});
        assert!(capability_chips(record_list(&only_false)).is_empty());
        // 空目录 / 缺 capabilities 字段 → 空芯片，不 panic
        assert!(capability_chips(record_list(&json!({"records": []}))).is_empty());
        assert!(capability_chips(record_list(&json!({"records": [{"id": "y"}]}))).is_empty());
    }

    /// 芯片点击：点已选中的能力 = 取消筛选（回到「全部」）
    #[test]
    fn test_chip_toggle() {
        assert_eq!(chip_toggle(None, "vision"), Some("vision".to_string()));
        assert_eq!(chip_toggle(Some("vision"), "text"), Some("text".to_string()));
        assert_eq!(chip_toggle(Some("vision"), "vision"), None);
    }

    /// 搜索：id / 显示名 / digest 三字段，大小写不敏感；空查询全通过
    #[test]
    fn test_matches_search_case_insensitive() {
        let v = sample();
        let recs = record_list(&v);
        let qwen = &recs[0];
        assert!(matches_search(qwen, "QWEN3")); // id 大小写不敏感
        assert!(matches_search(qwen, "qwen3-vl")); // id 前缀
        assert!(matches_search(qwen, "Qwen3-VL-8B")); // 显示名
        assert!(matches_search(qwen, "ABCDEF12")); // digest（大写）
        assert!(matches_search(qwen, "  ")); // 空/空白查询 = 全通过
        assert!(!matches_search(qwen, "kokoro")); // 不匹配就是 false
        assert!(!matches_search(qwen, "vision")); // 能力名不参与搜索（那是芯片的活）
    }

    /// 筛选叠加：搜索 **且** 能力（逻辑与）；工程视图不叠能力芯片
    #[test]
    fn test_filter_combines_search_and_capability() {
        let v = sample();
        let recs = record_list(&v);
        // 只有能力筛选
        let by_cap = filter_records(recs, "", Some("vision"));
        assert_eq!(by_cap.len(), 1);
        assert_eq!(by_cap[0].get("id").unwrap().as_str(), Some("qwen3-vl-8b"));
        // 能力 + 搜索（命中）
        assert_eq!(filter_records(recs, "qwen", Some("vision")).len(), 1);
        // 能力 + 搜索（不命中）→ 空
        assert!(filter_records(recs, "kokoro", Some("vision")).is_empty());
        // 无筛选 → 全量（坏记录也在——照常列出）
        assert_eq!(filter_records(recs, "", None).len(), 3);
        // 搜索命中坏记录 id 也留下
        assert_eq!(filter_records(recs, "broken", None).len(), 1);
    }

    /// 表格用的 digest 前 12 位与按字符截断（UTF-8 安全——中文/emoji 不 panic、不截半个字）
    #[test]
    fn test_short_digest_and_truncate_chars() {
        assert_eq!(short_digest("abcdef1234567890"), "abcdef123456");
        assert_eq!(short_digest("ab"), "ab");
        assert_eq!(short_digest(""), DASH);
        assert_eq!(short_digest("   "), DASH);
        assert_eq!(truncate_chars("abcdef", 10), "abcdef"); // 不超长原样返回
        assert_eq!(truncate_chars("abcdefghijkl", 6), "abcdef…");
        assert_eq!(truncate_chars("中文路径很长很长", 3), "中文路…"); // 按字符不按字节
        assert_eq!(truncate_chars("🙂🙂🙂🙂", 2), "🙂🙂…");
        // 多个字节/码点的组合也不 panic
        let mixed = "a中b文c🙂d";
        assert_eq!(truncate_chars(mixed, 99), mixed);
        assert_eq!(truncate_chars(mixed, 4).chars().count(), 5); // 4 字符 + 省略号
    }

    /// 具备能力：value=false 不算具备（实测没有 ≠ 有）
    #[test]
    fn test_has_capability_requires_true() {
        let v = sample();
        let recs = record_list(&v);
        assert!(has_capability(&recs[0], "vision"));
        assert!(has_capability(&recs[0], "text"));
        assert!(!has_capability(&recs[0], "audio_out"));
        assert!(!has_capability(&recs[0], "不存在的能力"));
        // 缺 capabilities 字段不 panic
        assert!(!has_capability(&recs[2], "text"));
        let f = json!({"capabilities": [{"name": "vision", "value": false}]});
        assert!(!has_capability(&f, "vision"));
        // 缺 value 字段（非 true）也不给「具备」
        let n = json!({"capabilities": [{"name": "vision"}]});
        assert!(!has_capability(&n, "vision"));
    }

    /// 记录键：(id, version) —— 显示名不参与身份
    #[test]
    fn test_rec_key() {
        assert_eq!(rec_key(&json!({"id": "m1", "version": "v2"})), "m1|v2");
        // 缺 id → 占位符；缺 version → 空尾（仍唯一到 id 粒度）
        assert_eq!(rec_key(&json!({"version": "v2"})), "?|v2");
        assert_eq!(rec_key(&json!({"id": "m1"})), "m1|");
    }

    /// license 全字段：一条都没登记 → 9 行值全是 Missing（显「—」），**不是错误**
    #[test]
    fn test_license_rows_all_missing_is_not_error() {
        let rows = license_rows(&json!({"id": "x"}));
        assert_eq!(rows.len(), 9);
        assert!(rows.iter().all(|(_, v)| *v == FieldVal::Missing));
        // 标签键顺序与 schema 字段一一对应（spdx / license_name / … / accepted_at / evidence）
        let keys: Vec<&str> = rows.iter().map(|(k, _)| *k).collect();
        assert_eq!(
            keys,
            vec![
                "mreg.lic_spdx",
                "mreg.lic_name",
                "mreg.lic_link",
                "mreg.lic_commercial",
                "mreg.lic_gated",
                "mreg.lic_source_url",
                "mreg.lic_accepted_by",
                "mreg.lic_accepted_at",
                "mreg.lic_evidence",
            ]
        );
        // 缺值 → 展示为「—」（占位符，不是空串、不是报错）
        assert_eq!(rows[6].1.display(), DASH);
    }

    /// license 部分字段 + accepted_by/at 留痕；commercial 双来源（license.commercial 优先，退回顶层）
    #[test]
    fn test_license_rows_partial_and_commercial_fallback() {
        // ① 只有顶层 commercial（现有接口的形状）→ 商用性行有值，其余「—」
        let top = license_rows(&json!({"id": "x", "commercial": "yes"}));
        assert_eq!(top[3].1, FieldVal::Text("yes".to_string()));
        assert_eq!(top[0].1, FieldVal::Missing);
        // ② license 块出现时接管，且 accepted_by/at 有就显示
        let full = license_rows(&json!({
            "id": "x", "commercial": "no",
            "license": {"spdx": "CC-BY-NC-4.0", "license_name": "CC BY-NC 4.0",
                        "license_link": "https://example.org/l", "commercial": "no",
                        "gated": true, "source_url": "https://hf.co/repo",
                        "accepted_by": "Mr2109", "accepted_at": "2026-09-11T10:00:00Z"}
        }));
        assert_eq!(full[0].1, FieldVal::Text("CC-BY-NC-4.0".to_string()));
        assert_eq!(full[1].1, FieldVal::Text("CC BY-NC 4.0".to_string()));
        assert_eq!(full[2].1, FieldVal::Text("https://example.org/l".to_string()));
        assert_eq!(full[3].1, FieldVal::Text("no".to_string()));
        assert_eq!(full[4].1, FieldVal::Flag(true));
        assert_eq!(full[5].1, FieldVal::Text("https://hf.co/repo".to_string()));
        assert_eq!(full[6].1, FieldVal::Text("Mr2109".to_string()));
        assert_eq!(full[7].1, FieldVal::Text("2026-09-11T10:00:00Z".to_string()));
        // ③ license.commercial 优先于顶层（两处冲突时以 license 块为准）
        let both = license_rows(&json!({"id": "x", "commercial": "yes",
                                        "license": {"commercial": "unknown"}}));
        assert_eq!(both[3].1, FieldVal::Text("unknown".to_string()));
        // ④ 空格子串不算有值
        let blank = license_rows(&json!({"id": "x", "commercial": "   "}));
        assert_eq!(blank[3].1, FieldVal::Missing);
    }

    /// 能力断言逐条：name / value / source / evidence / engines；缺 source/evidence 显「—」
    #[test]
    fn test_cap_rows_source_and_evidence() {
        let v = sample();
        let recs = record_list(&v);
        let rows = cap_rows(&recs[0]);
        assert_eq!(rows.len(), 2);
        assert_eq!(rows[0].0, "vision");
        assert_eq!(rows[0].1, FieldVal::Flag(true));
        assert_eq!(rows[0].2, "probed");
        assert_eq!(rows[0].3, "probe.vision.1x1.v1 ok");
        // 待修补 #39：引擎维度逐条带出（保序）
        assert_eq!(rows[0].4, vec!["llama.cpp".to_string(), "vllm".to_string()]);
        // 第二条没有 evidence → 「—」（缺证据不等于错，只是没写）；也没有引擎 → 空
        assert_eq!(rows[1].0, "text");
        assert_eq!(rows[1].3, DASH);
        assert!(rows[1].4.is_empty());
        // 缺 capabilities 字段 → 空列表，不 panic
        assert!(cap_rows(&recs[2]).is_empty());
        // name 缺失 → 占位符；value 缺失 → Missing（显「—」）；无引擎 → 空 vec（不填占位）
        let odd = cap_rows(&json!({"capabilities": [{"source": "manual", "evidence": "e"}]}));
        assert_eq!(odd[0].0, "?");
        assert_eq!(odd[0].1, FieldVal::Missing);
        assert_eq!(odd[0].2, "manual");
        assert_eq!(odd[0].3, "e");
        assert!(odd[0].4.is_empty());
    }

    /// 建材逐项与引擎配方：逐项字段齐全；size 缺失显「—」（不是 0 B）
    #[test]
    fn test_file_rows_and_recipe_rows() {
        let v = sample();
        let recs = record_list(&v);
        let f = file_rows(&recs[0]);
        assert_eq!(f.len(), 2);
        assert_eq!(f[0], ("weights".into(), "a.gguf".into(), "aa11".into(), "2.0 KB".into()));
        // 缺 name/sha256/role 显「—」；缺 size 显「—」（不伪装成 0 B）
        let g = file_rows(&json!({"files": [{"size": 0}, {}]}));
        assert_eq!(g[0], (DASH.into(), DASH.into(), DASH.into(), "0 B".into()));
        assert_eq!(g[1].3, DASH);
        // 缺 files 字段 → 空，不 panic
        assert!(file_rows(&recs[2]).is_empty());
        let r = recipe_rows(&recs[0]);
        assert_eq!(r, vec!["llama.cpp".to_string(), "vllm".to_string()]);
        assert!(recipe_rows(&recs[2]).is_empty());
        let blank = recipe_rows(&json!({"engine_recipes": [null, "", "  ", "vllm"]}));
        assert_eq!(blank, vec!["vllm".to_string()]);
    }

    /// 新渲染路径不 panic：能力视图（带芯片筛选/搜索）· 工程视图（表格）·
    /// 详情弹层（选中记录）· 无匹配态。全部走显式状态，不动全局。
    #[test]
    fn test_render_new_views_do_not_panic() {
        let v = sample();
        let sel_key = rec_key(&record_list(&v)[0]);
        let ctx = egui::Context::default();
        let states = vec![
            // 能力视图 + 无筛选
            ViewState { view: View::Capability, search: "", cap: None, selected: None, copied: None },
            // 能力视图 + 能力芯片 + 搜索叠加
            ViewState {
                view: View::Capability,
                search: "qwen",
                cap: Some("vision"),
                selected: Some(sel_key.as_str()),
                copied: None,
            },
            // 详情弹层 + 「已复制」反馈
            ViewState {
                view: View::Capability,
                search: "",
                cap: None,
                selected: Some(sel_key.as_str()),
                copied: Some("/tmp/models/manifests/qwen3-vl-8b/2026-09-01.json"),
            },
            // 工程视图（紧凑表格）
            ViewState {
                view: View::Engineering,
                search: "",
                cap: Some("vision"),
                selected: None,
                copied: None,
            },
            // 工程视图 + 搜索命中坏记录
            ViewState {
                view: View::Engineering,
                search: "broken",
                cap: None,
                selected: Some(sel_key.as_str()),
                copied: None,
            },
            // 无匹配态（筛选出空集，且已选中的记录被筛掉——详情仍按全量找得到）
            ViewState {
                view: View::Capability,
                search: "zzz-不存在",
                cap: None,
                selected: Some(sel_key.as_str()),
                copied: None,
            },
        ];
        for st in &states {
            let mut out = ctx.run_ui(Default::default(), |ui| {
                let _ = super::render_registry_with(ui, &v, st);
            });
            out.textures_delta.clear();
        }
        // 空目录 + 工程视图（空态优先，不画空表格）
        let empty = json!({"count": 0, "bad_records": 0, "root": "/tmp/none", "records": []});
        let st = ViewState { view: View::Engineering, search: "", cap: None, selected: None, copied: None };
        let mut out = ctx.run_ui(Default::default(), |ui| {
            let _ = super::render_registry_with(ui, &empty, &st);
        });
        out.textures_delta.clear();
    }

    /// 控件层（视图切换 / 搜索框 / 能力芯片）与整页渲染不 panic。
    /// 两种视图各过一遍（能力视图才有芯片行），再走一次整页 ui()。
    /// 预置 DATA = 已就绪 → ui() 不会发起拉取（has_data() 为真），测试不碰网络。
    #[test]
    fn test_page_and_controls_render_do_not_panic() {
        let v = sample();
        let ctx = egui::Context::default();
        for view in [View::Capability, View::Engineering] {
            let mut out = ctx.run_ui(Default::default(), |ui| super::controls(ui, &v, view));
            out.textures_delta.clear();
        }
        if let Ok(mut d) = DATA.lock() {
            *d = Some(Ok(v.clone()));
        }
        let mut out = ctx.run_ui(Default::default(), |ui| super::ui(ui));
        out.textures_delta.clear();
    }

    /// 待修补 #39：引擎列表取值（去空白/丢空串/保序/去重；缺或非数组 → 空）
    #[test]
    fn test_engine_list_and_display() {
        // 缺字段 / 非数组 / 空数组 → 空 vec（缺 = 未知，不填占位）
        assert!(engine_list(None).is_empty());
        assert!(engine_list(Some(&json!("llama.cpp"))).is_empty());
        assert!(engine_list(Some(&json!([]))).is_empty());
        // 去空白 + 丢空串 + 去重 + 保序
        let got = engine_list(Some(&json!([" llama.cpp ", "llama.cpp", "", "  ", "vllm"])));
        assert_eq!(got, vec!["llama.cpp".to_string(), "vllm".to_string()]);
        // 非字符串项跳过、不 panic
        assert!(engine_list(Some(&json!([1, null, true]))).is_empty());
        // 展示：空 → 「—」，多个用「, 」连接
        assert_eq!(engines_display(&[]), DASH);
        assert_eq!(
            engines_display(&["llama.cpp".to_string(), "vllm".to_string()]),
            "llama.cpp, vllm"
        );
    }

    /// 待修补 #39：不可判定能力逐条（照抄接口值；缺字段/非数组 → 空；缺项用占位）
    #[test]
    fn test_unverifiable_rows() {
        let v = sample();
        let recs = record_list(&v);
        let rows = unverifiable_rows(&recs[0]);
        assert_eq!(rows.len(), 1);
        assert_eq!(rows[0].0, "embedding");
        assert_eq!(rows[0].1, "budget_exhausted"); // 原文码照抄——不改写原因分类
        assert!(rows[0].2.starts_with("probe.embedding.v1"));
        // 缺 unverifiable 字段 / 非数组 → 空，不 panic（坏记录也不崩）
        assert!(unverifiable_rows(&recs[2]).is_empty());
        assert!(unverifiable_rows(&json!({"unverifiable": "oops"})).is_empty());
        // 缺 name/reason/evidence → 占位符 / 「—」，不 panic
        let odd = unverifiable_rows(&json!({"unverifiable": [{}]}));
        assert_eq!(odd[0].0, "?");
        assert_eq!(odd[0].1, "");
        assert_eq!(odd[0].2, DASH);
    }

    /// 待修补 #39：不可判定原因 → 通俗文案；未知原因码原样回退（不猜、不改写）
    #[test]
    fn test_uv_reason_label() {
        assert_eq!(uv_reason_label("budget_exhausted"), t!("mreg.uv_reason_budget").to_string());
        assert_eq!(uv_reason_label("timeout"), t!("mreg.uv_reason_timeout").to_string());
        // 已知原因码必须真的译出来（不是把 i18n 键原样吐回）
        let budget = uv_reason_label("budget_exhausted");
        assert!(!budget.is_empty() && !budget.contains("mreg."));
        // 空白 → 「—」；陌生原因码 → 原样回退（含 trim）
        assert_eq!(uv_reason_label(""), DASH);
        assert_eq!(uv_reason_label("  weird_reason  "), "weird_reason");
    }

    /// 待修补 #39：license 来源锚有值即显示、缺则「—」（空值不是错误、不是占位）
    #[test]
    fn test_license_rows_includes_evidence() {
        let with = license_rows(&json!({"id": "x", "license": {
            "spdx": "apache-2.0", "commercial": "yes",
            "evidence": "probe.license.v1 (gguf_key: general.license=\"apache-2.0\")"}}));
        assert_eq!(with[8].0, "mreg.lic_evidence");
        assert_eq!(
            with[8].1,
            FieldVal::Text("probe.license.v1 (gguf_key: general.license=\"apache-2.0\")".to_string())
        );
        // 缺 evidence → Missing（显「—」）
        let without = license_rows(&json!({"id": "x", "commercial": "yes"}));
        assert_eq!(without[8].1, FieldVal::Missing);
        // 空白 evidence 不算有值
        let blank = license_rows(&json!({"id": "x", "license": {"evidence": "   "}}));
        assert_eq!(blank[8].1, FieldVal::Missing);
    }

    /// 待修补 #39：证据链三样的渲染路径不 panic（齐全 / 全缺 两种极端都过一遍）
    #[test]
    fn test_render_evidence_chain_do_not_panic() {
        let full = sample();
        let bare = json!({"count": 1, "bad_records": 0, "root": "/tmp/m",
            "records": [{"id": "bare", "version": "v1", "commercial": "unknown",
                         "capabilities": [{"name": "text", "value": true, "source": "declared"}],
                         "files": []}]});
        let ctx = egui::Context::default();
        for payload in [&full, &bare] {
            let sel = rec_key(&record_list(payload)[0]);
            let st = ViewState {
                view: View::Capability,
                search: "",
                cap: None,
                selected: Some(sel.as_str()),
                copied: None,
            };
            let mut out = ctx.run_ui(Default::default(), |ui| {
                let _ = super::render_registry_with(ui, payload, &st);
            });
            out.textures_delta.clear();
        }
    }
}
