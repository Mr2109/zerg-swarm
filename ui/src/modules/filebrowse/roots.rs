//! 根集合与选择（文件浏览器阶段 1——2026-09-13 设计「文件浏览器虫茧」§4.2 / §4.3）
//!
//! 后端契约（GET /api/fileroots）：
//! ```json
//! {"roots":[{"id","label","path","default","writable"}...],
//!  "config":{"display_max":1048576,"allow_all_types":false,"text_exts":[".md",...]}}
//! ```
//! - **根只读集合**由**后端**把关（§4.4）——前端只拿 id 与相对路径，绝不拼绝对路径回传。
//! - 界面文案优先 i18n 键 `fb.root.<id>`，取不到再退回后端 label（后端 label 是中文，
//!   英文界面下若不键优先就会露出中文）。
//! - 本文件里的解析/选择逻辑都是**纯函数**（不碰网络/UI）——单测直接钉死。

use eframe::egui;

use super::browser::FbIntent;

/// 界面显示上限缺省值（§4.4 Q3：读取不限，只有「界面内渲染」上限 1 MiB）
pub const DEFAULT_DISPLAY_MAX: usize = 1048576;

/// 文本类扩展名缺省清单（仅在后端 config.text_exts 缺失时兜底——权威清单在后端，可配置）
pub const DEFAULT_TEXT_EXTS: [&str; 21] = [
    ".md",
    ".markdown",
    ".txt",
    ".json",
    ".yaml",
    ".yml",
    ".toml",
    ".rs",
    ".go",
    ".py",
    ".sh",
    ".ts",
    ".js",
    ".html",
    ".css",
    ".sql",
    ".csv",
    ".log",
    ".ini",
    ".c",
    ".h",
];

/// 一个根（后端白名单里的一项）
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RootInfo {
    pub id: String,
    /// 后端给的中文名（i18n 键取不到时退而用之）
    pub label: String,
    /// 根的**绝对路径**（只用于显示与「复制路径」——不回传后端）
    pub path: String,
    /// 是否默认根（后端只应标一个）
    pub is_default: bool,
    /// 是否可写（阶段 1 只有 docs 根可写——写菜单只对可写根出现）
    pub writable: bool,
    /// 根目录在磁盘上是否存在（新后端字段；**缺字段 = true**——旧后端绝不误报「不存在」）
    pub exists: bool,
}

impl RootInfo {
    pub fn from_json(v: &serde_json::Value) -> RootInfo {
        RootInfo {
            id: v
                .get("id")
                .and_then(|x| x.as_str())
                .unwrap_or_default()
                .to_string(),
            label: v
                .get("label")
                .and_then(|x| x.as_str())
                .unwrap_or_default()
                .to_string(),
            path: v
                .get("path")
                .and_then(|x| x.as_str())
                .unwrap_or_default()
                .to_string(),
            is_default: v.get("default").and_then(|x| x.as_bool()).unwrap_or(false),
            writable: v.get("writable").and_then(|x| x.as_bool()).unwrap_or(false),
            // §尾巴 a：缺字段按 true（=未知/存在）。**绝不** `unwrap_or(false)`——
            // 那会把所有旧后端的根一律误报成「该根不存在」。
            exists: v.get("exists").and_then(|x| x.as_bool()).unwrap_or(true),
        }
    }
}

/// 后端 config（界面显示上限 + 类型闸门——§4.4「将来放开=改配置不改代码」）
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FilerootsConfig {
    /// 界面内渲染上限（字节）；读取侧无上限
    pub display_max: usize,
    /// 放开为任意类型（默认关——误点打开任意可执行文件是真实风险）
    pub allow_all_types: bool,
    /// 文本类扩展名清单（open 与「读入 UI」共用同一份——§九 Q5）
    pub text_exts: Vec<String>,
}

impl Default for FilerootsConfig {
    fn default() -> Self {
        FilerootsConfig {
            display_max: DEFAULT_DISPLAY_MAX,
            allow_all_types: false,
            text_exts: DEFAULT_TEXT_EXTS.iter().map(|s| s.to_string()).collect(),
        }
    }
}

impl FilerootsConfig {
    /// 解析 config（缺失/非法一律落缺省——界面不该因为后端少一个字段就渲染空白）
    /// display_max=0 视为缺省：0 会让界面永远只显示空内容，不是有意义的配置。
    pub fn from_json(v: Option<&serde_json::Value>) -> FilerootsConfig {
        let mut c = FilerootsConfig::default();
        let Some(v) = v else { return c };
        if let Some(n) = v.get("display_max").and_then(|x| x.as_u64()) {
            let n = n as usize;
            if n > 0 {
                c.display_max = n;
            }
        }
        if let Some(b) = v.get("allow_all_types").and_then(|x| x.as_bool()) {
            c.allow_all_types = b;
        }
        if let Some(a) = v.get("text_exts").and_then(|x| x.as_array()) {
            let list: Vec<String> = a
                .iter()
                .filter_map(|x| x.as_str().map(|s| s.to_string()))
                .collect();
            if !list.is_empty() {
                c.text_exts = list;
            }
        }
        c
    }

    /// 类型闸门：该路径是否允许「读入 UI / 用默认应用打开」（§4.4）
    /// - 清单内类型 ⇒ 允许
    /// - 无扩展名 ⇒ 不允许（§九 Q4：无扩展名默认只给「在访达中显示」）
    /// - allow_all_types ⇔ 一律允许（放开是改配置）
    pub fn ext_allowed(&self, path: &str) -> bool {
        if self.allow_all_types {
            return true;
        }
        let name = path.rsplit('/').next().unwrap_or(path).to_ascii_lowercase();
        let has_ext = name
            .rsplit_once('.')
            .map(|(_, e)| !e.is_empty())
            .unwrap_or(false);
        if !has_ext {
            return false;
        }
        self.text_exts
            .iter()
            .any(|e| name.ends_with(&e.to_ascii_lowercase()))
    }
}

/// 根集合 + 配置（拉到即缓存——根清单几乎不变）
#[derive(Debug, Clone, Default)]
pub struct RootsState {
    pub roots: Vec<RootInfo>,
    pub config: FilerootsConfig,
}

impl RootsState {
    pub fn from_json(v: &serde_json::Value) -> RootsState {
        let roots = v
            .get("roots")
            .and_then(|r| r.as_array())
            .map(|a| a.iter().map(RootInfo::from_json).collect())
            .unwrap_or_default();
        RootsState {
            roots,
            config: FilerootsConfig::from_json(v.get("config")),
        }
    }

    pub fn find(&self, id: &str) -> Option<&RootInfo> {
        self.roots.iter().find(|r| r.id == id)
    }

    /// 当前根绝对路径（显示 + 复制路径用）
    pub fn path_of(&self, id: &str) -> Option<&str> {
        self.find(id).map(|r| r.path.as_str())
    }

    /// 当前根是否可写（写菜单只对可写根出现；拿不到根清单时按**不可写**处理——保守）
    pub fn is_writable(&self, id: &str) -> bool {
        self.find(id).map(|r| r.writable).unwrap_or(false)
    }

    /// 当前根是否**已知不存在**（§尾巴 a）——只有后端明确给 `exists=false` 才为 true。
    /// 根 id 不在清单里 ⇒ false（未知 ≠ 不存在——不误报）。缺 exists 字段已在
    /// [`RootInfo::from_json`] 里落成 true，故旧后端恒为 false。
    pub fn known_missing(&self, id: &str) -> bool {
        self.find(id).map(|r| !r.exists).unwrap_or(false)
    }
}

/// 当前根是否已知不存在（**纯函数——单测钉死**）：`None`（清单未到）或未知根一律 false。
/// 关键回归点：旧后端缺 `exists` 字段时**绝不**误报「该根不存在」。
pub fn root_missing(id: &str, state: Option<&RootsState>) -> bool {
    state.map(|s| s.known_missing(id)).unwrap_or(false)
}

/// 是否应发起列表请求（**纯函数**）：根为空、或当前根已知不存在 ⇒ 不拉。
/// 清单未到（None）或未知根 ⇒ 照常拉（不能因为没清单/新增根就不拉）。
pub fn should_fetch_listing(root_id: &str, state: Option<&RootsState>) -> bool {
    if root_id.is_empty() {
        return false;
    }
    !root_missing(root_id, state)
}

/// 当前根不存在的空态（§尾巴 a）：文案走 i18n 键 `fb.root.missing`（带路径），
/// 右侧给「在访达中显示」入口（显示该根本身；后端若报错也只是可读错误，不 panic）。
/// 返回 true = 本帧渲染了空态（调用方不必再渲染文件列表）。
pub fn render_root_missing(
    ui: &mut egui::Ui,
    state: Option<&RootsState>,
    root_id: &str,
    out: &mut Vec<FbIntent>,
) -> bool {
    if !root_missing(root_id, state) {
        return false;
    }
    let path = state
        .and_then(|s| s.path_of(root_id))
        .unwrap_or_default()
        .to_string();
    ui.add_space(8.0);
    ui.horizontal_wrapped(|ui| {
        ui.colored_label(
            egui::Color32::from_rgb(240, 200, 80),
            rust_i18n::t!("fb.root.missing", path = path.clone()).to_string(),
        );
        // 根不存在时仍可「在访达中显示」——揭示该路径本身（父目录非白名单内相对路径，不做）
        if ui.button(rust_i18n::t!("fb.action.reveal")).clicked() {
            out.push(FbIntent::Reveal {
                root: root_id.to_string(),
                path: String::new(),
            });
        }
    });
    true
}

/// 默认根 id（**纯函数——单测钉死**）：
/// 优先后端 `default:true` 标记；无标记则取清单第一项；空清单 → None。
pub fn default_root_id(roots: &[RootInfo]) -> Option<String> {
    roots
        .iter()
        .find(|r| r.is_default)
        .or_else(|| roots.first())
        .map(|r| r.id.clone())
}

/// 根显示名：i18n 键 `fb.root.<id>` 优先，取不到（rust-i18n 原样返回键名）退回后端 label。
/// lookup 注入是为了可单测（不依赖全局 locale）。
pub fn root_label(id: &str, backend_label: &str, lookup: impl Fn(&str) -> String) -> String {
    let key = format!("fb.root.{}", id);
    let v = lookup(&key);
    if v == key || v.trim().is_empty() {
        backend_label.to_string()
    } else {
        v
    }
}

/// 完整「根栏」一行：标签 + 根选择器 + 当前根**绝对路径** + 「复制路径」按钮（§4.3）
/// 返回 true = 本次切了根（调用方须清空旧根的选中与缓存）
pub fn root_bar(
    ui: &mut egui::Ui,
    roots: Option<&RootsState>,
    current: &mut String,
    out: &mut Vec<FbIntent>,
) -> bool {
    let mut changed = false;
    ui.horizontal_wrapped(|ui| {
        ui.label(rust_i18n::t!("fb.root.label"));
        changed = root_selector(ui, roots, current, out);
        if let Some(p) = roots.and_then(|s| s.path_of(current.as_str())) {
            ui.separator();
            ui.weak(egui::RichText::new(p).size(11.0));
            if ui.button(rust_i18n::t!("fb.path.copy")).clicked() {
                out.push(FbIntent::CopyPath(p.to_string()));
            }
        }
    });
    changed
}

/// 渲染根选择器（ComboBox——默认选中后端默认根，然后由调用方切根）。
/// 返回 true = 本次切了根（调用方须清空该根的选中目录/文件与缓存内容）。
pub fn root_selector(
    ui: &mut egui::Ui,
    roots: Option<&RootsState>,
    current: &mut String,
    out: &mut Vec<FbIntent>,
) -> bool {
    let mut changed = false;
    let list: &[RootInfo] = roots.map(|s| s.roots.as_slice()).unwrap_or(&[]);
    // 根清单还没到（或后端没给根）：退化为单一只读标签——界面不空、不猜路径
    if list.is_empty() {
        ui.label(egui::RichText::new(rust_i18n::t!("fb.root.docs").to_string()).weak());
        return false;
    }
    if current.is_empty() {
        if let Some(d) = default_root_id(list) {
            *current = d;
            changed = true;
        }
    }
    let cur_label = list
        .iter()
        .find(|r| r.id == *current)
        .map(|r| root_label(&r.id, &r.label, |k| rust_i18n::t!(k).to_string()))
        .unwrap_or_else(|| current.clone());
    egui::ComboBox::from_id_salt("fb_root_selector")
        .selected_text(cur_label)
        .show_ui(ui, |ui| {
            for r in list {
                let label = root_label(&r.id, &r.label, |k| rust_i18n::t!(k).to_string());
                if ui.selectable_label(r.id == *current, label).clicked() && r.id != *current {
                    *current = r.id.clone();
                    changed = true;
                    out.push(FbIntent::RootChanged(r.id.clone()));
                }
            }
        });
    changed
}
