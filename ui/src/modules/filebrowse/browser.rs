//! 目录/文件列表渲染 + 显示截断 + 类型闸门（文件浏览器阶段 1——2026-09-13 设计 §4.3 / §4.4）
//!
//! 分工：
//! - **纯函数**（`display_window` / `mib` / `can_open`）——不碰 UI，单测直接钉死；
//! - **渲染函数**（`render_list` / `system_menu` / `render_content`）——不持有状态，
//!   用户意图以 [`FbIntent`] 交回调用方去改状态/发请求（避免闭包里再借 &mut 应用状态）。
//!
//! 显示上限的口径（§4.4 Q3 Mr2109拍板）：
//! **读取不限**（后端整份取回）——**只有界面渲染**截到 `display_max` 字节，
//! 并按字符边界回退（中文不切半个字），同时给出「已显示前 X MiB／共 Y MiB」提示与
//! 「用默认应用打开」入口。

use eframe::egui;

use super::roots::FilerootsConfig;

/// 打开方式（后端契约：open 的 mode 字段）
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FbOpenMode {
    File,
    Dir,
}

impl FbOpenMode {
    pub fn as_str(self) -> &'static str {
        match self {
            FbOpenMode::File => "file",
            FbOpenMode::Dir => "dir",
        }
    }
}

/// 用户意图（渲染层 → 状态层）——渲染函数只产生意图，不发请求、不改状态
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FbIntent {
    /// 切根（根选择器）——调用方须清空旧根的目录/文件选中与缓存
    RootChanged(String),
    /// 选中目录
    PickDir(String),
    /// 选中文件（去读内容）
    PickFile(String),
    /// 在访达中显示（POST /api/fileroots/reveal）
    Reveal { root: String, path: String },
    /// 用默认应用打开（POST /api/fileroots/open）
    Open {
        root: String,
        path: String,
        mode: FbOpenMode,
    },
    /// 复制路径到剪贴板
    CopyPath(String),
}

/// 渲染视图参数（一次渲染所需的一切——避免渲染函数再去借应用/组件状态）
#[derive(Debug, Clone)]
pub struct FbView {
    /// 当前根 id（回传给后端的 root 字段——绝对路径**绝不**回传）
    pub root: String,
    /// 当前根是否可写（只读根：不出现任何写菜单）
    pub writable: bool,
    /// 第一栏当前选中目录（相对根的相对路径；空 = 根）
    pub dir: String,
    /// 当前选中文件（相对根的相对路径）
    pub file: String,
    /// 后端配置（显示上限 + 类型闸门）
    pub cfg: FilerootsConfig,
}

impl FbView {
    /// 该条目是否允许「用默认应用打开 / 读入 UI」（目录恒可——文件看类型清单）
    pub fn can_open(&self, path: &str, is_dir: bool) -> bool {
        can_open(path, is_dir, &self.cfg)
    }
}

/// 类型闸门（§4.4 / §九 Q4+Q5）：目录恒可；文件按可配置文本清单；无扩展名默认只给「在访达中显示」
pub fn can_open(path: &str, is_dir: bool, cfg: &FilerootsConfig) -> bool {
    is_dir || cfg.ext_allowed(path)
}

/// 截断提示栏里是否给「用默认应用打开」按钮（**纯函数**，§4.4）：
/// open 与读入共用同一份可配置类型清单——清单外只保留「在访达中显示」。
/// 截断提示恒针对**已读入的内容文件**（is_dir 恒 false），与右键菜单共用同一套 `can_open`，
/// 两处不许各写一套判定（否则清单放开后难免一处漏掉）。
pub fn truncated_open_allowed(file: &str, cfg: &FilerootsConfig) -> bool {
    can_open(file, false, cfg)
}

/// 显示窗口（**纯函数**）：返回（可渲染前缀, 渲染字节数, 是否截断）。
/// 截断按**字符边界**回退——中文 3 字节/字，绝不能切在中间（否则 egui 渲染乱码/panic 风险）。
pub fn display_window(content: &str, display_max: usize) -> (&str, usize, bool) {
    if content.len() <= display_max {
        return (content, content.len(), false);
    }
    let mut end = display_max.min(content.len());
    while end > 0 && !content.is_char_boundary(end) {
        end -= 1;
    }
    (&content[..end], end, true)
}

/// 字节数 → MiB 文本（1 位小数——提示文案用）
pub fn mib(bytes: usize) -> String {
    format!("{:.1}", bytes as f64 / (1024.0 * 1024.0))
}

/// 交给系统的两项（§4.3 右键菜单）——**只读根同样可用**（打开 ≠ 改，§九 Q2）。
/// 「用默认应用打开」受类型闸门限制（清单外只给「在访达中显示」）。
pub fn system_menu(ui: &mut egui::Ui, view: &FbView, path: &str, is_dir: bool, out: &mut Vec<FbIntent>) {
    ui.separator();
    if ui
        .button(format!("{} {}", crate::modules::icons::icon_text("folder-open"), rust_i18n::t!("fb.action.reveal")))
        .clicked()
    {
        out.push(FbIntent::Reveal {
            root: view.root.clone(),
            path: path.to_string(),
        });
        ui.close();
    }
    if view.can_open(path, is_dir)
        && ui
            .button(format!("{} {}", crate::modules::icons::icon_text("folder-open"), rust_i18n::t!("fb.action.open")))
            .clicked()
    {
        out.push(FbIntent::Open {
            root: view.root.clone(),
            path: path.to_string(),
            mode: if is_dir { FbOpenMode::Dir } else { FbOpenMode::File },
        });
        ui.close();
    }
}

/// 第一栏：目录（缩进树）+ 当前目录的直接子文件（视觉沿用既有文档模块：📁 / 📄）
pub fn render_list(
    ui: &mut egui::Ui,
    view: &FbView,
    dirs: &[String],
    files: &[String],
    out: &mut Vec<FbIntent>,
) {
    for d in dirs {
        let depth = d.split('/').count() - 1;
        let name = d.rsplit('/').next().unwrap_or(d.as_str());
        let label = if depth == 0 {
            format!("📁 {}", name)
        } else {
            format!("{}└ 📁 {}", "  ".repeat(depth), name)
        };
        let resp = ui.selectable_label(view.dir == *d, label);
        resp.context_menu(|ui| system_menu(ui, view, d, true, out));
        if resp.clicked() {
            out.push(FbIntent::PickDir(d.clone()));
        }
    }
    ui.add_space(4.0);
    let dir_prefix = if view.dir.is_empty() {
        String::new()
    } else {
        format!("{}/", view.dir)
    };
    let mut any = false;
    for f in files {
        let rest = f.strip_prefix(dir_prefix.as_str()).unwrap_or(f.as_str());
        if rest.contains('/') {
            continue; // 只看当前目录的直接子文件
        }
        any = true;
        let name = f.rsplit('/').next().unwrap_or(f.as_str());
        let resp = ui.selectable_label(view.file == *f, format!("📄 {}", name));
        resp.context_menu(|ui| system_menu(ui, view, f, false, out));
        if resp.clicked() {
            out.push(FbIntent::PickFile(f.clone()));
        }
    }
    if !any {
        ui.weak(rust_i18n::t!("common.none"));
    }
}

/// 第二栏：内容预览（含显示截断提示 + 「用默认应用打开」入口；不做内嵌编辑器——阶段 1）
pub fn render_content(
    ui: &mut egui::Ui,
    view: &FbView,
    content: Option<&str>,
    err: Option<&str>,
    cache: &mut egui_commonmark::CommonMarkCache,
    out: &mut Vec<FbIntent>,
) {
    if let Some(e) = err {
        ui.add_space(8.0);
        ui.colored_label(egui::Color32::from_rgb(230, 90, 90), format!("⚠ {}", e));
        return;
    }
    let Some(text) = content else {
        ui.spinner();
        ui.weak(rust_i18n::t!("common.loading"));
        return;
    };
    let (shown, rendered, cut) = display_window(text, view.cfg.display_max);
    if cut {
        // §4.4：超出即截断显示 + 明确提示「已显示前 1 MiB／共 N MiB，可『用默认应用打开』看全文」
        ui.horizontal_wrapped(|ui| {
            ui.colored_label(
                egui::Color32::from_rgb(240, 200, 80),
                rust_i18n::t!(
                    "fb.display.truncated",
                    shown = mib(rendered),
                    total = mib(text.len())
                )
                .to_string(),
            );
            if truncated_open_allowed(&view.file, &view.cfg)
                && ui.button(rust_i18n::t!("fb.action.open")).clicked()
            {
                out.push(FbIntent::Open {
                    root: view.root.clone(),
                    path: view.file.clone(),
                    mode: FbOpenMode::File,
                });
            }
        });
        ui.separator();
    }
    egui::ScrollArea::vertical()
        .id_salt("fb_content")
        .auto_shrink(false)
        .show(ui, |ui| {
            egui_commonmark::CommonMarkViewer::new().show(ui, cache, shown);
        });
}
