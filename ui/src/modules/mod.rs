//! 集装箱模块系统（v2.5.6 UI 蓝图——Mr2109 2026-08-29）
//!
//! 虫族 UI = 集装箱船：
//! - 船体（核心）: 任务体系（任务队列/内部任务）+ 基础设施（集群/模型库/资源库）——永驻
//! - 集装箱（模块）: 文档/Git/日志 + 未来生态应用——可吊装可卸下
//!
//! 模块注册表 + 顶部导航渲染入口。

pub mod zerg_module;
pub mod ferrite; // M3 Ferrite 重写 md 编辑器（Mr2109 2026-08-29）
pub mod chat; // v2.5.7 对话模块（Mr2109——完全借鉴 Hermes——第一板块）
pub mod upgrade; // L2 自动升级页（2026-09-11）
pub mod model_registry; // 模型登记库页（GET /api/models/registry——列表+卡片+空态）
pub mod filebrowse; // 文件/目录浏览器**组件库**（阶段 1——2026-09-13 设计「文件浏览器集装箱」§4.1）
                    // 注意：本目录是内建**组件**（供多处吊装），不是顶栏箱；顶栏只多一只薄壳箱 file-browser
pub mod icons; // P3 图标统一封装（iconflow——14 包 34 TTF——MIT）

use eframe::egui;
use rust_i18n::t;   // i18n（B1 抽取：导航/模块名走键）
use crate::modules::icons::icon_text; // P3 图标（iconflow）
pub use zerg_module::{ModuleManifest, ModuleRegistry};

/// 构建全量模块注册表（船体 5 核心箱 + 甲板 3 可装卸箱）
///
/// 核心箱（不可禁用——船体骨架）:
/// - tasks / internal-tasks: AI 任务体系（心脏）
/// - cluster / models / resources: 基础设施三件套（骨架）
/// 可装卸箱（默认在船）:
/// - docs / git / logs: 预置模块
pub fn build_registry() -> ModuleRegistry {
    let mut reg = ModuleRegistry::default();

    // ── 父箱（导航骨架——纯分组：没有自己的页面，内容 = 子箱的二级页签；不可卸载）──
    // 2026-09-13 设计《UI 大调动-导航精简与分组》§4.1/§4.2：顶栏一级 = 3 父箱 + 3 无父箱。
    // ⚡ 主控在线（父箱）——首个页签=集群（Mr2109 2026-09-13 定：不另设「概览」页）
    reg.register(ModuleManifest {
        id: "main-online",
        name_key: "mod.main_online.name",
        icon: icon_text("monitor"), // 图标名必须真实存在（icons 测试守）
        desc_key: "mod.main_online.desc",
        is_core: true, // 父箱不可卸（导航骨架）
        version: env!("CARGO_PKG_VERSION"),
        parent: None,
        order: 10,
        is_group: true,
    });
    // 📋 任务（父箱：任务队列 + 内部任务）
    reg.register(ModuleManifest {
        id: "tasks-group",
        name_key: "mod.tasks_group.name",
        icon: icon_text("list-checks"),
        desc_key: "mod.tasks_group.desc",
        is_core: true,
        version: env!("CARGO_PKG_VERSION"),
        parent: None,
        order: 30, // Mr2109 2026-09-13 二次调整：让「对话」排到第 2 位
        is_group: true,
    });
    // 💻 模型（父箱：模型库 + 模型登记库）
    reg.register(ModuleManifest {
        id: "models-group",
        name_key: "mod.models_group.name",
        icon: icon_text("computer-tower"),
        desc_key: "mod.models_group.desc",
        is_core: true,
        version: env!("CARGO_PKG_VERSION"),
        parent: None,
        order: 40, // Mr2109 2026-09-13 二次调整
        is_group: true,
    });

    // 🐛 虫茧（父箱：平台 + 文档 —— Mr2109 2026-09-13 二次调整：文档移入虫茧）
    reg.register(ModuleManifest {
        id: "cocoon",
        name_key: "mod.roundtable.name", // 父箱名沿用「虫茧」（子箱「平台」另起键，避免页签重名）
        icon: icon_text("boxes"),
        desc_key: "mod.roundtable.desc",
        is_core: true, // 父箱不可卸（导航骨架）
        version: env!("CARGO_PKG_VERSION"),
        parent: None,
        order: 60,
        is_group: true,
    });
    // ⚡ 核心（船体箱——永驻；一级：chat / resources；子箱：tasks / internal-tasks / cluster / models）
    // v2.5.7 对话模块（Mr2109——完全借鉴 Hermes——第一板块）
    reg.register(ModuleManifest {
        id: "chat",
        name_key: "mod.chat.name",
        icon: icon_text("chat-circle-text"),
        desc_key: "mod.chat.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        parent: None, // 不分组——保持一级（设计 Q7；顺序由Mr2109 2026-09-13 二次调整定为第 2 位）
        order: 20,
        is_group: false,
    });
    reg.register(ModuleManifest {
        id: "tasks",
        name_key: "mod.tasks.name",
        icon: icon_text("list-checks"),
        desc_key: "mod.tasks.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        parent: Some("tasks-group"),
        order: 10,
        is_group: false,
    });
    reg.register(ModuleManifest {
        id: "internal-tasks",
        name_key: "mod.internal_tasks.name",
        icon: icon_text("wrench"),
        desc_key: "mod.internal_tasks.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        parent: Some("tasks-group"),
        order: 20,
        is_group: false,
    });
    reg.register(ModuleManifest {
        id: "cluster",
        name_key: "mod.cluster.name",
        icon: icon_text("chart-bar"),
        desc_key: "mod.cluster.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        parent: Some("main-online"),
        order: 10, // 主控在线默认页签（Mr2109 2026-09-13 定）
        is_group: false,
    });
    reg.register(ModuleManifest {
        id: "models",
        name_key: "mod.models.name",
        icon: icon_text("computer-tower"),
        desc_key: "mod.models.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        parent: Some("models-group"),
        order: 10, // 模型父页默认页签
        is_group: false,
    });
    // 模型登记库（模型库——登记表：许可/能力/建材/校验；数据源 GET /api/models/registry）
    // 与上面的「模型库」（机群已加载模型管理）区分：本箱是 ~/.zerg/models 里登记的模型目录。
    reg.register(ModuleManifest {
        id: "model-registry",
        name_key: "mod.model_registry.name",
        icon: icon_text("package"),
        desc_key: "mod.model_registry.desc",
        is_core: false,
        version: env!("CARGO_PKG_VERSION"),
        parent: Some("models-group"),
        order: 20,
        is_group: false,
    });
    reg.register(ModuleManifest {
        id: "resources",
        name_key: "mod.resources.name",
        icon: icon_text("package"),
        desc_key: "mod.resources.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        parent: None, // 不分组——保持一级（设计 Q7；二次调整后为第 5 位）
        order: 50,
        is_group: false,
    });

    // 🧩 可装卸箱（甲板箱——默认在船）
    // 文件浏览器（阶段 1——2026-09-13 设计「文件浏览器集装箱」§4.1）：只是一只**薄壳箱**，
    // 真正的实现是内建组件 ui/src/modules/filebrowse/（供文档/模型/任务等多处吊装）。
    // 卸下本箱不影响文档模块内的「在访达中显示/用默认应用打开」（§七 12 条）。
    reg.register(ModuleManifest {
        id: "file-browser",
        name_key: "mod.file_browser.name",
        icon: icon_text("folder-open"),
        desc_key: "mod.file_browser.desc",
        is_core: false,
        version: env!("CARGO_PKG_VERSION"),
        parent: Some("main-online"),
        order: 20,
        is_group: false,
    });
    // 🦋 虫茧（T8——zerg-cocoon 第一个茧——示例虫茧 egui 集装箱——破茧换新）
    // 虫茧 = 独立应用平台（集装箱平台），**保持顶级**、不并进「主控在线」（Mr2109 2026-09-13：R2 只列了五项）。
    // 仍在船、可装卸；其平台页标题 **保留**（设计 §4.4）。
    reg.register(ModuleManifest {
        id: "roundtable",
        name_key: "mod.cocoon_platform.name",
        icon: icon_text("boxes"),
        desc_key: "mod.cocoon_platform.desc",
        is_core: false,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        // 2026-09-13 二次调整：虫茧升为**父箱**，本箱是它的「平台」页（默认页签）。
        // 公开快照解除跨仓依赖时本页显示「未装载」，但同父箱下的「文档」照常可用。
        parent: Some("cocoon"),
        order: 10,
        is_group: false,
    });
    reg.register(ModuleManifest {
        id: "docs",
        name_key: "mod.docs.name",
        icon: icon_text("books"),
        desc_key: "mod.docs.desc",
        is_core: false,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        // 2026-09-13 Mr2109二次调整：**文档移入「虫茧」**（虫茧第 2 个页签）
        parent: Some("cocoon"),
        order: 20,
        is_group: false,
    });
    reg.register(ModuleManifest {
        id: "git",
        name_key: "mod.git.name",
        icon: icon_text("git-branch"),
        desc_key: "mod.git.desc",
        is_core: false,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        parent: Some("main-online"),
        order: 40,
        is_group: false,
    });
    reg.register(ModuleManifest {
        id: "logs",
        name_key: "mod.logs.name",
        icon: icon_text("scroll"),
        desc_key: "mod.logs.desc",
        is_core: false,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
        parent: Some("main-online"),
        order: 50,
        is_group: false,
    });
    // ↑ L2 自动升级页（2026-09-11）——2026-09-13 起并入「主控在线」父页
    reg.register(ModuleManifest {
        id: "upgrade",
        name_key: "mod.upgrade.name",
        icon: icon_text("arrow-up-circle"),
        desc_key: "mod.upgrade.desc",
        is_core: false,
        version: env!("CARGO_PKG_VERSION"),
        parent: Some("main-online"),
        order: 30,
        is_group: false,
    });

    // M4 生态箱接入——配置文件声明外部模块（<UI 状态目录>/external-modules.json；暂不支持 parent——设计 §4.2 规则 5）
    reg.load_external();

    reg
}

/// 状态灯 = 「主控在线」父箱（设计 §4.1：第一个一级项就是 ● 主控在线，可点）
const STATUS_GROUP_ID: &str = "main-online";

/// 渲染顶部导航栏（船桥——**两段式**：一级 + 二级页签；Mr2109 2026-09-13）
///
/// 布局: [● 主控在线] [📋 任务] [💻 模型] [💬 对话] [📦 资源库] [📚 文档] [➕] … [English] [👤 Mr2109]
///        集群 | 文件浏览器 | 升级 | Git | 日志 | 虫茧     ← 二级页签（仅当前 effective 箱属于某父箱时）
/// - 一级 = `top_level()`（父箱 + 无父箱），父箱按钮可点（点=切到该父箱，自动落到记忆/首个子箱）
/// - **状态灯可点**：保留 ● 与绿/红，点它 = 进「主控在线」父箱（hover 提示 status.light_tip）
/// - 二级 = 当前 effective 箱所属父箱的 `children_of()`（`selectable_label`——与一级同款视觉）
/// - ➕ = 吊装系统入口（模块管理）；右侧固定: 语言切换 + HUD 开关 + 登录用户
pub fn top_nav_bar(
    ui: &mut egui::Ui,
    registry: &mut ModuleRegistry,
    online: bool,
    locale: &str,
    on_switch_locale: &mut dyn FnMut(),
    on_open_manager: &mut dyn FnMut(),
    // v2.5.7 HUD 开关（English 旁图标——Mr2109: ⌘H 是系统键冲突——改显式按钮）
    hud_on: bool,
    on_toggle_hud: &mut dyn FnMut(),
) {
    // 当前**有效**箱（父箱 ⇒ 记忆子箱 / order 最小子箱）——二级页签高亮与父箱归属都用它。
    let remembered = registry
        .remembered_child
        .get(&registry.active)
        .map(String::as_str)
        .unwrap_or("");
    let effective = registry.effective_module(&registry.active, remembered);
    // 当前二级页签所属父箱（None ⇒ 不画二级行：顶级箱/外部箱）
    let active_parent: Option<&'static str> = registry.parent_of(&effective);

    ui.vertical(|ui| {
        // ── 一级导航（含可点状态灯）────────────────────────────────────────────
        ui.horizontal(|ui| {
            let mut switched: Option<String> = None;
            for m in registry.top_level() {
                // 选中判定：active 就是它，或其子箱（effective）正挂在它下面
                let selected = registry.active == m.id || active_parent == Some(m.id);
                if m.id == STATUS_GROUP_ID {
                    // 主控在线状态灯——**可点按钮**（保留 ● 与绿/红；点=切到该父箱）
                    let (dot, color) = if online {
                        ("●", egui::Color32::from_rgb(80, 200, 120))
                    } else {
                        ("●", egui::Color32::from_rgb(220, 80, 80))
                    };
                    let text = if online { t!("status.online") } else { t!("status.offline") };
                    let btn = ui
                        .add(
                            egui::Button::new(egui::RichText::new(format!("{} {}", dot, text)).color(color))
                                .frame(selected),
                        )
                        .on_hover_text(t!("status.light_tip"));
                    if btn.clicked() {
                        switched = Some(m.id.to_string());
                    }
                } else {
                    let label = format!("{} {}", m.icon, t!(m.name_key));
                    if ui.selectable_label(selected, label).clicked() {
                        switched = Some(m.id.to_string());
                    }
                }
            }
            // M4 生态箱（外部模块——配置文件声明——第三方开发者挂船；暂不支持 parent——设计 §4.2 规则 5）
            let ext_visible = registry.external_visible();
            if !ext_visible.is_empty() {
                ui.separator();
            }
            for m in &ext_visible {
                let label = format!("{} {}", m.icon, m.name);
                if ui.selectable_label(registry.active == m.id, label).clicked() {
                    switched = Some(m.id.clone());
                }
            }
            if let Some(id) = switched {
                // M32(2026-09-10 审计): 注册表目前仅"元数据"——此处只切 active。
                // trait ZergModule 的 on_load/on_unload 生命周期**尚未接线**（见 zerg_module.rs）。
                // 父箱可以直接设为 active ⇒ 渲染前由 `effective_module()` 下钻到子箱。
                registry.active = id;
            }

            // 吊装系统入口（➕——模块管理——Mr2109 2026-08-29 M2）
            if ui.button("➕").on_hover_text(t!("modules.open_tip")).clicked() {
                on_open_manager();
            }
            ui.separator();

            // 右侧对齐：English + 登录用户（右到左布局）
            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                // 登录用户（v2.5.6——固定 Mr2109——未来登录系统）
                ui.label(format!("👤 {}", std::env::var("USER").unwrap_or_else(|_| "user".to_string())));   // 环境无关化：不再硬编码用户名
                ui.separator();
                // v2.5.7 HUD 开关（English 旁——Mr2109：图标开关——⌘H 是系统键冲突）
                let hud_label = format!("{}", icon_text("gauge"));
                let hud_btn = ui
                    .button(if hud_on { egui::RichText::new(&hud_label).strong() } else { egui::RichText::new(&hud_label).weak() })
                    .on_hover_text(if hud_on { t!("hud.tip_on") } else { t!("hud.tip_off") });
                if hud_btn.clicked() {
                    on_toggle_hud();
                }
                ui.separator();
                // 语言切换（多语言——中文/English）
                if locale == "zh-CN" {
                    if ui.button("English").clicked() {
                        on_switch_locale();
                    }
                } else {
                    if ui.button("中文").clicked() {
                        on_switch_locale();
                    }
                }
            });
        });

        // ── 二级页签（仅当当前 effective 箱属于某个父箱时显示）──────────────────
        if let Some(parent) = active_parent {
            ui.horizontal(|ui| {
                let kids = registry.children_of(parent);
                if kids.is_empty() {
                    // 设计 §4.2 规则 4：父箱下全部子箱被卸下 ⇒ 父箱仍在，给一句提示
                    ui.weak(t!("nav.no_submodules"));
                } else {
                    let mut pick: Option<String> = None;
                    for c in &kids {
                        let label = format!("{} {}", c.icon, t!(c.name_key));
                        if ui.selectable_label(effective == c.id, label).clicked() {
                            pick = Some(c.id.to_string());
                        }
                    }
                    if let Some(child) = pick {
                        // 记住「父 + 子」：切到子箱并记入记忆（app.rs 落盘 ui_state.json）
                        registry.remembered_child.insert(parent.to_string(), child.clone());
                        registry.active = child;
                    }
                }
            });
        }
    });
}


#[cfg(test)]
mod nav_tests {
    use super::*;

    /// 顶栏**两段**渲染冒烟（无头 egui——能失败：借用冲突/空态/离线分支 panic 即红）。
    /// 覆盖：① 顶级箱（不画二级）② 父箱在线/离线（状态灯 + 二级页签）③ 父箱无子箱（空态提示）。
    #[test]
    fn top_nav_bar_renders_two_tiers_without_panic() {
        let ctx = egui::Context::default();
        let mut reg = build_registry();

        // ① 顶级箱（chat）——无二级页签行
        reg.active = "chat".to_string();
        let mut out = ctx.run_ui(Default::default(), |ui| {
            top_nav_bar(ui, &mut reg, true, "zh-CN", &mut (|| {}), &mut (|| {}), false, &mut (|| {}));
        });
        out.textures_delta.clear();

        // ② 父箱（main-online）——状态灯（在线/离线各一次）+ 二级页签
        reg.active = "main-online".to_string();
        for online in [true, false] {
            let mut out = ctx.run_ui(Default::default(), |ui| {
                top_nav_bar(ui, &mut reg, online, "zh-CN", &mut (|| {}), &mut (|| {}), true, &mut (|| {}));
            });
            out.textures_delta.clear();
        }
        // 状态灯是主控在线父箱的选中按钮 ⇒ active 仍是父箱
        assert_eq!(reg.active, "main-online");

        // ③ 父箱下全部子箱被卸下 ⇒ 空态提示（设计 §4.2 规则 4）——不 panic
        // 注：虫茧（roundtable）2026-09-13 起是**顶级**箱，不在本父箱下，故不在此列
        for id in ["cluster", "file-browser", "upgrade", "git", "logs"] {
            reg.enabled.insert(id.to_string(), false);
        }
        assert!(reg.children_of("main-online").is_empty());
        let mut out = ctx.run_ui(Default::default(), |ui| {
            top_nav_bar(ui, &mut reg, true, "en", &mut (|| {}), &mut (|| {}), false, &mut (|| {}));
        });
        out.textures_delta.clear();

        // ④ 二级页签点击目标语义：设置记忆子箱后 effective 就是它（纯函数已单测；此处锁父子记忆）
        reg.remembered_child.insert("main-online".to_string(), "git".to_string());
        reg.enabled.insert("git".to_string(), true);
        assert_eq!(reg.effective_module("main-online", "git"), "git");
    }
}
