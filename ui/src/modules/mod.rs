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

    // ⚡ 核心（船体箱——永驻）
    // v2.5.7 对话模块（Mr2109——完全借鉴 Hermes——第一板块——排任务队列前）
    reg.register(ModuleManifest {
        id: "upgrade",
        name_key: "mod.upgrade.name",
        icon: icon_text("arrow-up-circle"),
        desc_key: "mod.upgrade.desc",
        is_core: false,
        version: env!("CARGO_PKG_VERSION"),
    });
    reg.register(ModuleManifest {
        id: "chat",
        name_key: "mod.chat.name",
        icon: icon_text("chat-circle-text"),
        desc_key: "mod.chat.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
    });
    reg.register(ModuleManifest {
        id: "tasks",
        name_key: "mod.tasks.name",
        icon: icon_text("list-checks"),
        desc_key: "mod.tasks.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
    });
    reg.register(ModuleManifest {
        id: "internal-tasks",
        name_key: "mod.internal_tasks.name",
        icon: icon_text("wrench"),
        desc_key: "mod.internal_tasks.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
    });
    reg.register(ModuleManifest {
        id: "cluster",
        name_key: "mod.cluster.name",
        icon: icon_text("chart-bar"),
        desc_key: "mod.cluster.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
    });
    reg.register(ModuleManifest {
        id: "models",
        name_key: "mod.models.name",
        icon: icon_text("computer-tower"),
        desc_key: "mod.models.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
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
    });
    reg.register(ModuleManifest {
        id: "resources",
        name_key: "mod.resources.name",
        icon: icon_text("package"),
        desc_key: "mod.resources.desc",
        is_core: true,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
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
    });
    // 🦋 虫茧（T8——zerg-cocoon 第一个茧——示例虫茧 egui 集装箱——破茧换新）
    reg.register(ModuleManifest {
        id: "roundtable",
        name_key: "mod.roundtable.name",
        icon: icon_text("boxes"),
        desc_key: "mod.roundtable.desc",
        is_core: false,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
    });
    reg.register(ModuleManifest {
        id: "docs",
        name_key: "mod.docs.name",
        icon: icon_text("books"),
        desc_key: "mod.docs.desc",
        is_core: false,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
    });
    reg.register(ModuleManifest {
        id: "git",
        name_key: "mod.git.name",
        icon: icon_text("git-branch"),
        desc_key: "mod.git.desc",
        is_core: false,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
    });
    reg.register(ModuleManifest {
        id: "logs",
        name_key: "mod.logs.name",
        icon: icon_text("scroll"),
        desc_key: "mod.logs.desc",
        is_core: false,
        // 箱版本 = 构建版本（Cargo.toml 单一来源；勿写死——APP-A23 收口）
        version: env!("CARGO_PKG_VERSION"),
    });

    // M4 生态箱接入——配置文件声明外部模块（/tmp/zerg-ui/external-modules.json）
    reg.load_external();

    reg
}

/// 渲染顶部导航栏（船桥——Mr2109 2026-08-29）
///
/// 布局: [🐝 虫族] [● 主控在线] [📋 任务队列] [🔧 内部任务] ... [➕] ... [English] [👤 Mr2109]
/// - 板块按钮 = 集装箱（点击切换——选中高亮）
/// - ➕ = 吊装系统入口（模块管理——M2）
/// - 右侧固定: 语言切换 + 登录用户
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
    ui.horizontal(|ui| {
        // 主控在线状态灯（第一个——Mr2109 2026-08-29 删品牌——第一个=主控在线）
        if online {
            ui.colored_label(egui::Color32::from_rgb(80, 200, 120), format!("● {}", t!("status.online")));
        } else {
            ui.colored_label(egui::Color32::from_rgb(220, 80, 80), format!("● {}", t!("status.offline")));
        }
        ui.separator();

        // 板块切换（集装箱排布）——M31(2026-09-10 审计): 只读借用注册表，
        // 原来每帧 clone 全部清单（含 String 图标）与 active；点击只在循环后落一次 active。
        let mut switched: Option<String> = None;
        for m in registry.visible() {
            let label = format!("{} {}", m.icon, t!(m.name_key));
            if ui.selectable_label(registry.active == m.id, label).clicked() {
                switched = Some(m.id.to_string());
            }
        }
        // M4 生态箱（外部模块——配置文件声明——第三方开发者挂船）
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
            // trait ZergModule 的 on_load/on_unload 生命周期**尚未接线**（见 zerg_module.rs），
            // 待模块真正持有状态后再在此处调用（旧注释"懒加载 M2 完整实现"是误导，已更正）。
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
}
