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
pub mod icons; // P3 图标统一封装（iconflow——14 包 34 TTF——MIT）

use eframe::egui;
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
        id: "chat",
        name: "对话",
        icon: icon_text("chat-circle-text"),
        description: "AI 对话（借鉴 Hermes——思考可见——全工具）",
        is_core: true,
        version: "0.1.0",
    });
    reg.register(ModuleManifest {
        id: "tasks",
        name: "任务队列",
        icon: icon_text("list-checks"),
        description: "AI 任务体系——外部任务创建/调度/执行/复查/成果",
        is_core: true,
        version: "0.1.0",
    });
    reg.register(ModuleManifest {
        id: "internal-tasks",
        name: "内部任务",
        icon: icon_text("wrench"),
        description: "AI 任务体系——16 类内部任务自动编排/手动执行",
        is_core: true,
        version: "0.1.0",
    });
    reg.register(ModuleManifest {
        id: "cluster",
        name: "集群",
        icon: icon_text("chart-bar"),
        description: "基础设施——机器状态/负载/GPU/健康",
        is_core: true,
        version: "0.1.0",
    });
    reg.register(ModuleManifest {
        id: "models",
        name: "模型库",
        icon: icon_text("computer-tower"),
        description: "基础设施——模型管理/加载/适配器选项",
        is_core: true,
        version: "0.1.0",
    });
    reg.register(ModuleManifest {
        id: "resources",
        name: "资源库",
        icon: icon_text("package"),
        description: "基础设施——模型/工具/skill/mcp 资源信任度",
        is_core: true,
        version: "0.1.0",
    });

    // 🧩 可装卸箱（甲板箱——默认在船）
    // 🦋 虫茧（T8——zerg-cocoon 第一个茧——示例虫茧 egui 集装箱——破茧换新）
    reg.register(ModuleManifest {
        id: "roundtable",
        name: "虫茧",
        icon: icon_text("boxes"),
        description: "虫族集装箱平台——示例虫茧（群 AI 讨论——小说生成只是它的一个项目）",
        is_core: false,
        version: "0.1.0",
    });
    reg.register(ModuleManifest {
        id: "docs",
        name: "文档",
        icon: icon_text("books"),
        description: "文档三栏 + md 编辑器（M3 换 Ferrite 集装箱）",
        is_core: false,
        version: "0.1.0",
    });
    reg.register(ModuleManifest {
        id: "git",
        name: "Git",
        icon: icon_text("git-branch"),
        description: "Git 状态/分支/提交",
        is_core: false,
        version: "0.1.0",
    });
    reg.register(ModuleManifest {
        id: "logs",
        name: "日志",
        icon: icon_text("scroll"),
        description: "主控运行日志",
        is_core: false,
        version: "0.1.0",
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
            ui.colored_label(egui::Color32::from_rgb(80, 200, 120), "● 主控在线");
        } else {
            ui.colored_label(egui::Color32::from_rgb(220, 80, 80), "● 主控离线");
        }
        ui.separator();

        // 板块切换（集装箱排布）——M31(2026-09-10 审计): 只读借用注册表，
        // 原来每帧 clone 全部清单（含 String 图标）与 active；点击只在循环后落一次 active。
        let mut switched: Option<String> = None;
        for m in registry.visible() {
            let label = format!("{} {}", m.icon, m.name);
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
        if ui.button("➕").on_hover_text("模块管理（吊装系统——启用/禁用集装箱）").clicked() {
            on_open_manager();
        }
        ui.separator();

        // 右侧对齐：English + 登录用户（右到左布局）
        ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
            // 登录用户（v2.5.6——固定 Mr2109——未来登录系统）
            ui.label("👤 Mr2109");
            ui.separator();
            // v2.5.7 HUD 开关（English 旁——Mr2109：图标开关——⌘H 是系统键冲突）
            let hud_label = format!("{}", icon_text("gauge"));
            let hud_btn = ui
                .button(if hud_on { egui::RichText::new(&hud_label).strong() } else { egui::RichText::new(&hud_label).weak() })
                .on_hover_text(if hud_on { "仪表 HUD：显示中（点击隐藏）" } else { "仪表 HUD：已隐藏（点击显示）" });
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
