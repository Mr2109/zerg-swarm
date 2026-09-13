//! 虫茧标准接口（v2.5.6 UI 蓝图——Mr2109 2026-08-29）
//!
//! 虫族 UI = 虫茧船：船体（核心）标准化，虫茧（模块）可吊装可卸下。
//! 本文件定义"虫茧标准"——任何独立应用套上这个接口 = 一个虫茧。
//! 设计借鉴 dsh/Cordis（贡献 + 可逆生命周期）、VS Code（自举验证/懒加载）。
//!
//! M1 阶段：定义标准 + 注册表元数据 + 顶部导航布局（渲染仍走 ZergApp 现有方法）。
//! M3 阶段：Ferrite 适配器用 trait 对象实现（真虫茧——升级=换 crate 版本——船体不动）。
//! 未来：生态箱接入（外部应用实现本 trait 挂上船）。

use eframe::egui;

/// 模块清单（吊装元数据——船桥显示 + 模块管理）
#[derive(Debug, Clone)]
pub struct ModuleManifest {
    /// 唯一 id（如 "tasks" / "docs"）
    pub id: &'static str,
    /// 显示名称的 i18n 键（如 "mod.tasks.name"）——渲染处用 t!(name_key)；
    /// 若键不存在，rust-i18n 会原样返回键名（外部箱因此可直接放普通文本）
    pub name_key: &'static str,
    /// 图标（emoji——PingFang 支持）
    pub icon: String,
    /// 简介的 i18n 键（如 "mod.tasks.desc"）——渲染处用 t!(desc_key)
    pub desc_key: &'static str,
    /// 是否核心箱（船体箱——不可禁用——任务体系+基础设施）
    pub is_core: bool,
    /// 版本（箱内容版本——Ferrite 升级=版本变——船体无感）
    pub version: &'static str,
    /// **父箱 id**（`None` = 一级/顶级箱）。父箱必须是"容器式"箱（`is_group: true`）且不可卸载。
    /// 语义：本箱挂在哪个父箱的二级页签下；`None` ⇒ 出现在顶栏一级（`top_level()`）。
    /// 子箱自身仍是**真箱**：`id`/`name_key`/`icon`/`desc_key`/`is_core`/版本全部保留
    /// ⇒ i18n 键与 modules.json 形状零改动（旧文件里的 `{"git":false}` 仍表示"git 箱被卸下"）。
    pub parent: Option<&'static str>,
    /// **同 parent 内的排序**（父箱页签顺序即此值升序）；一级箱之间（父箱与无父箱）也按此值升序。
    /// `sort_by_key` 稳定 ⇒ 同 value 时保持注册顺序。
    pub order: u32,
    /// **是否父箱/分组箱**：纯分组容器——**没有自己的页面**，其内容 = 子箱的二级页签。
    /// 语义：① `top_level()` 会显示它；② 不可卸载（`toggle()` 拒绝，导航骨架）；
    /// ③ 不可作为最终渲染目标——`effective_module()` 会把它下钻到子箱，无子箱时才回退到它自己。
    pub is_group: bool,
}

/// 虫茧标准接口——任何独立应用实现它 = 一个虫茧
///
/// 借鉴 Cordis：贡献（render/ai_hook）+ 可逆生命周期（on_load/on_unload 成对）
pub trait ZergModule {
    /// 吊装元数据
    fn manifest(&self) -> ModuleManifest;

    /// 吊装（on_load——点击加载——懒加载：VS Code Activation Events 经验）
    /// 可逆：与 on_unload 成对——卸载自动回滚（Cordis 效应）
    fn on_load(&mut self);

    /// 卸下（on_unload——可逆——清理本箱注册的效应）
    fn on_unload(&mut self) {}

    /// 舱位渲染——箱子自己的世界（egui 可嵌入：渲染到指定 Ui 而非窗口）
    /// 独立应用（如 Ferrite）套上适配器后在此渲染完整界面
    fn render(&mut self, ui: &mut egui::Ui);
}

/// 外部生态箱（M4——配置文件声明——第三方开发者挂船）
/// 与内建箱区别：String 字段（运行时读文件——非编译期）
#[derive(Debug, Clone, serde::Deserialize)]
pub struct ExternalModule {
    /// 唯一 id（如 "ext-calc"）
    pub id: String,
    /// 显示名称
    pub name: String,
    /// 图标（emoji）
    pub icon: String,
    /// 简介
    pub description: String,
    /// 版本
    pub version: String,
    /// 外部地址（URL 或路径——占位阶段显示/打开）
    pub url: String,
}

/// 模块注册表（吊装系统——船体管理所有虫茧）
///
/// M1 阶段：元数据注册表——顶部导航渲染 + 启用状态管理。
/// 渲染仍由 ZergApp 现有方法承担（match id）——功能不变风险最小。
/// M3 起：Ferrite 等独立应用以 trait 对象注册——真虫茧。
/// M4：外部生态箱——配置文件声明（external-modules.json）——第三方挂船。
pub struct ModuleRegistry {
    /// 所有模块清单（含核心箱 + 可装卸箱）
    pub modules: Vec<ModuleManifest>,
    /// 外部生态箱（M4——配置文件声明）
    pub external: Vec<ExternalModule>,
    /// 当前选中的模块 id（船桥高亮）
    pub active: String,
    /// 每个模块的启用状态（id → 是否在船）——可装卸箱可禁用
    pub enabled: std::collections::HashMap<String, bool>,
    /// **记住每个父箱上次选中的子箱**（父箱 id → 子箱 id）。落盘 `ui_state.json`；
    /// 切到某父箱时（`effective_module`）优先用它，父箱页签与 HUD 面包屑也据此高亮。
    pub remembered_child: std::collections::BTreeMap<String, String>,
}

impl Default for ModuleRegistry {
    fn default() -> Self {
        Self {
            modules: Vec::new(),
            external: Vec::new(),
            active: "chat".to_string(), // P4-19 默认对话模块（对话最常用——原 tasks 启动停在任务队列）
            enabled: std::collections::HashMap::new(),
            remembered_child: std::collections::BTreeMap::new(),
        }
    }
}

impl ModuleRegistry {
    /// 注册一个虫茧（吊装上船）
    pub fn register(&mut self, m: ModuleManifest) {
        let id = m.id.to_string();
        self.modules.push(m);
        // 核心箱永驻在船；可装卸箱默认在船。
        // M32(2026-09-10 审计)：原来这里 `let is_core = m.is_core; ... let _ = is_core;`
        // 是纯死代码（读出来又丢弃），已删除；核心箱保护由 `toggle()` 用同一字段实现。
        self.enabled.insert(id, true);
    }

    /// 可显示的模块列表（启用的全部箱——**平铺**）
    ///
    /// 2026-09-13（导航精简）：顶栏改走 `top_level()`/`children_of()`（父子分组），本函数不再是
    /// 导航数据源；作为虫茧体系的**平铺视图 API**保留（E17：find()/visible() 是既有查询面，
    /// 供未来 M3 trait 接线与外部调用）。故显式允许 dead_code，避免"导航换了就删 API"。
    #[allow(dead_code)]
    pub fn visible(&self) -> Vec<&ModuleManifest> {
        self.modules
            .iter()
            .filter(|m| self.enabled.get(m.id).copied().unwrap_or(true))
            .collect()
    }

    /// 可显示的外部生态箱（启用的——顶部导航）
    pub fn external_visible(&self) -> Vec<&ExternalModule> {
        self.external
            .iter()
            .filter(|m| self.enabled.get(&m.id).copied().unwrap_or(true))
            .collect()
    }

    /// 加载外部生态箱（M4——<UI 状态目录>/external-modules.json 配置文件声明；2026-09-13 起默认 ~/.zerg/state/ui）
    /// 格式: {"modules": [{"id","name","icon","description","version","url"}]}
    /// 找不到文件 = 无外部箱（正常）；读取/解析失败（M33 2026-09-10 审计：不再静默）
    /// 打一次日志便于定位（用户写的 JSON 写错时不再无声无息）。
    pub fn load_external(&mut self) {
        let path = crate::api::ui_dir().join("external-modules.json");
        let s = match std::fs::read_to_string(&path) {
            Ok(s) => s,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return, // 无文件=正常
            Err(e) => {
                eprintln!("[modules] failed to read the external module config {}: {}", path.display(), e); // M33
                return;
            }
        };
        #[derive(serde::Deserialize)]
        struct Config {
            modules: Vec<ExternalModule>,
        }
        let cfg = match serde_json::from_str::<Config>(&s) {
            Ok(c) => c,
            Err(e) => {
                eprintln!("[modules] failed to parse the external module config {}: {}", path.display(), e); // M33
                return;
            }
        };
        self.external = cfg.modules;
        // 外部箱默认在船（enabled 记录——id 直接是完整 id 如 "ext-calc"）
        for m in &self.external {
            self.enabled.entry(m.id.clone()).or_insert(true);
        }
    }

    /// 按 id 查清单
    pub fn find(&self, id: &str) -> Option<&ModuleManifest> {
        self.modules.iter().find(|m| m.id == id)
    }

    /// 本箱是否在船（缺记录 = 默认在船）
    pub fn is_enabled(&self, id: &str) -> bool {
        self.enabled.get(id).copied().unwrap_or(true)
    }

    // ─── 父子导航：纯查询 API（E17——导航/面板/回退共用；纯函数⇒可单测）──────────────

    /// **一级导航项**：启用的父箱 + **没有父箱**的启用箱，按 `order` 升序（稳定的）。
    /// （设计 §4.1/§七1：主控在线 / 任务 / 模型 / 对话 / 资源库 / 文档 + ➕ + 右侧三件。）
    pub fn top_level(&self) -> Vec<&ModuleManifest> {
        let mut v: Vec<&ModuleManifest> = self
            .modules
            .iter()
            .filter(|m| m.parent.is_none() && self.is_enabled(m.id))
            .collect();
        v.sort_by_key(|m| m.order); // 稳定排序：同 order 保持注册顺序
        v
    }

    /// **某父箱下的启用子箱**，按 `order` 升序。父箱不存在/无启用子箱 ⇒ 空。
    /// 卸下的子箱自动跳过（设计 §4.2 规则 4「不留孤儿」）。
    pub fn children_of(&self, parent_id: &str) -> Vec<&ModuleManifest> {
        let mut v: Vec<&ModuleManifest> = self
            .modules
            .iter()
            .filter(|m| m.parent == Some(parent_id) && self.is_enabled(m.id))
            .collect();
        v.sort_by_key(|m| m.order);
        v
    }

    /// 某箱的父箱 id（`None` = 一级；未知箱也返回 `None`）。
    pub fn parent_of(&self, id: &str) -> Option<&'static str> {
        self.find(id).and_then(|m| m.parent)
    }

    /// **有效渲染箱**：把"逻辑选择（`active`）"解析成真正要渲染/高亮的箱。
    /// - `active` 是父箱：记忆子箱（`remembered_child`）仍可用 ⇒ 用它；否则回退到该父箱
    ///   `order` 最小的启用子箱；父箱**无**启用子箱 ⇒ 返回父箱自己（内容区给"子模块均未装载"）。
    /// - `active` 不是父箱（普通箱/子箱/未知）：原样返回。
    ///
    /// 纯函数（只读注册表）⇒ 三个父箱因此**不需要各自的渲染臂**（分发走本函数的结果）。
    pub fn effective_module(&self, active: &str, remembered_child: &str) -> String {
        match self.find(active) {
            Some(m) if m.is_group => {
                let kids = self.children_of(active);
                if !remembered_child.is_empty()
                    && kids.iter().any(|c| c.id == remembered_child)
                {
                    return remembered_child.to_string();
                }
                match kids.first() {
                    Some(c) => c.id.to_string(),
                    None => active.to_string(), // 无子箱 ⇒ 父箱自己（概览空态）
                }
            }
            _ => active.to_string(),
        }
    }

    /// 某箱被**卸下**后当前选中应落到哪（E15——原来写死 `"tasks"`，父子化后会落在一个子箱 id 上）。
    /// 子箱 ⇒ 落到其父箱的首个启用子箱（父箱页签自动亮对）；无父箱（一级箱）⇒ 回退默认 `"chat"`。
    /// 纯函数（不修改，只在给定"已加载过 enabled"的注册表上查询）。
    pub fn fallback_after_disable(&self, disabled_id: &str) -> String {
        match self.parent_of(disabled_id) {
            // 归属**父箱**（is_group）⇒ 落到该父箱的第一个子箱；归属**平台**（非 group，
            // 如「文档」之于虫茧）⇒ 落到平台自身（栅格还在，文档只是被卸下的应用卡）。
            Some(p) if self.find(p).map(|m| m.is_group).unwrap_or(false) => self
                .children_of(p)
                .first()
                .map(|c| c.id.to_string())
                .unwrap_or_else(|| p.to_string()),
            Some(p) => p.to_string(),
            None => "chat".to_string(),
        }
    }

    /// **二级页签的父箱**：只有 `is_group` 父箱才有页签行。
    /// 归属平台的应用（文档 ⇒ 虫茧）返回 `None` ⇒ 导航不出现页签、也不出现在一级。
    pub fn sub_tabs_parent_of(&self, effective: &str) -> Option<&'static str> {
        self.parent_of(effective)
            .filter(|p| self.find(p).map(|m| m.is_group).unwrap_or(false))
    }

    /// **虫茧平台栅格的应用卡 id**（顺序 = 展示顺序）：归属本平台的启用箱（文档）+ 平台自带的
    /// 跨仓应用（示例虫茧，排最后）。文档被卸下 ⇒ 卡片消失；示例虫茧未编译进来 ⇒ 由渲染层显示「未装载」。
    pub fn platform_apps(&self) -> Vec<&'static str> {
        let mut v: Vec<&'static str> = self.children_of("roundtable").iter().map(|m| m.id).collect();
        v.push("roundtable");
        v.retain(|id| self.is_enabled(id));
        v
    }

    /// HUD/面包屑「父 › 子」的 **i18n 键序列**（E16——原来只显示箱 id 对应的名字）。
    /// 子箱有父箱 ⇒ `[父箱名键, 子箱名键]`；否则 `[自身名键]`；未知 id ⇒ 空。
    /// 返回**键**（不是译文）⇒ 纯函数、可单测，渲染处再 `t!()` 拼接。
    pub fn breadcrumb_keys(&self, effective: &str) -> Vec<&'static str> {
        match self.find(effective) {
            Some(m) => match m.parent.and_then(|p| self.find(p)) {
                Some(g) => vec![g.name_key, m.name_key],
                None => vec![m.name_key],
            },
            None => Vec::new(),
        }
    }

    /// 切换启用状态（可装卸箱——核心箱 / 父箱不可禁用）
    pub fn toggle(&mut self, id: &str) {
        if let Some(m) = self.find(id) {
            if m.is_core || m.is_group {
                return; // 船体箱 / 父箱（导航骨架）不可卸
            }
        }
        let e = self.enabled.entry(id.to_string()).or_insert(true);
        *e = !*e;
    }

    /// 持久化——哪些箱在船上（<UI 状态目录>/modules.json；2026-09-13 起默认 ~/.zerg/state/ui，旧 /tmp 文件首次访问自动搬）
    /// 只存 enabled 状态——模块清单是代码内建的（虫茧注册）
    pub fn save(&self) {
        let dir = crate::api::ui_dir();
        // M33(2026-09-10 审计): 目录/写盘失败不再静默——用户禁用/启用选择重启即失效却无感知
        if let Err(e) = std::fs::create_dir_all(&dir) {
            eprintln!("[modules] failed to create the config directory {}: {}", dir.display(), e);
            return;
        }
        let path = format!("{}/modules.json", dir.display());
        let enabled: std::collections::BTreeMap<String, bool> = self
            .enabled
            .iter()
            .filter(|(_, v)| !**v) // 只记禁用的（默认都在船——增量记录）
            .map(|(k, v)| (k.clone(), *v))
            .collect();
        match serde_json::to_string(&enabled) {
            Ok(s) => {
                if let Err(e) = std::fs::write(&path, s) {
                    eprintln!("[modules] failed to save the module state {}: {}", path, e); // M33
                }
            }
            Err(e) => eprintln!("[modules] failed to serialize the module state: {}", e), // M33
        }
    }

    /// 加载持久化状态（启动时调用——恢复"哪些箱卸下了"）
    /// 找不到文件 = 默认全在船（首次启动）；解析失败打日志（M33 2026-09-10 审计）
    pub fn load(&mut self) {
        let path = crate::api::ui_dir().join("modules.json");
        let s = match std::fs::read_to_string(&path) {
            Ok(s) => s,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return,
            Err(e) => {
                eprintln!("[modules] failed to read the module state {}: {}", path.display(), e); // M33
                return;
            }
        };
        let disabled: std::collections::BTreeMap<String, bool> = match serde_json::from_str(&s) {
            Ok(d) => d,
            Err(e) => {
                eprintln!("[modules] failed to parse the module state {}: {}", path.display(), e); // M33
                return;
            }
        };
        for (id, v) in disabled {
            if let Some(m) = self.find(&id) {
                if !m.is_core {
                    // 恢复禁用状态（核心箱永驻——不可被持久化禁用）
                    self.enabled.insert(id, !v);
                }
            }
        }
    }

    // ─── 导航选中状态（父+子）：<UI 状态目录>/ui_state.json（Q9/Q10——2026-09-13）──────

    /// 导航状态文件：`<UI 状态目录>/ui_state.json`（与 modules.json 同目录）
    fn nav_state_path() -> std::path::PathBuf {
        crate::api::ui_dir().join("ui_state.json")
    }

    /// 持久化"当前选中 + 各父箱记忆的子箱"（父+子）。写失败打日志不静默（M33 纪律）。
    pub fn save_state(&self) {
        let dir = crate::api::ui_dir();
        if let Err(e) = std::fs::create_dir_all(&dir) {
            eprintln!("[modules] failed to create the state directory {}: {}", dir.display(), e);
            return;
        }
        let state = NavState {
            active: self.active.clone(),
            children: self.remembered_child.clone(),
        };
        let path = Self::nav_state_path();
        match serde_json::to_string(&state) {
            Ok(s) => {
                if let Err(e) = std::fs::write(&path, s) {
                    eprintln!("[modules] failed to save the navigation state {}: {}", path.display(), e);
                }
            }
            Err(e) => eprintln!("[modules] failed to serialize the navigation state: {}", e),
        }
    }

    /// 恢复"当前选中 + 各父箱记忆的子箱"。找不到文件 = 用默认（chat）；坏文件打日志用默认。
    /// 只接受仍然存在的箱 id（箱被删/改名后不留僵尸选中）。
    pub fn load_state(&mut self) {
        let path = Self::nav_state_path();
        let s = match std::fs::read_to_string(&path) {
            Ok(s) => s,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return,
            Err(e) => {
                eprintln!("[modules] failed to read the navigation state {}: {}", path.display(), e);
                return;
            }
        };
        let state: NavState = match serde_json::from_str(&s) {
            Ok(v) => v,
            Err(e) => {
                eprintln!("[modules] failed to parse the navigation state {}: {}", path.display(), e);
                return;
            }
        };
        if self.find(&state.active).is_some() {
            self.active = state.active;
        }
        for (parent, child) in state.children {
            if self.find(&parent).is_some() && self.find(&child).is_some() {
                self.remembered_child.insert(parent, child);
            }
        }
    }
}

/// 导航状态文件形状（`ui_state.json`）：当前选中箱 + 父箱 → 记忆子箱
#[derive(Debug, Clone, Default, serde::Serialize, serde::Deserialize)]
struct NavState {
    /// 当前选中的箱 id（可以是父箱——渲染前经 `effective_module` 下钻）
    #[serde(default)]
    active: String,
    /// 父箱 id → 上次选中的子箱 id
    #[serde(default)]
    children: std::collections::BTreeMap<String, String>,
}


#[cfg(test)]
mod tests {
    use super::*;

    fn ids(v: &[&ModuleManifest]) -> Vec<String> {
        v.iter().map(|m| m.id.to_string()).collect()
    }

    /// 设计 §4.1/§七1/Q5：一级导航恰 6 项、顺序 = 主控在线 → 任务 → 模型 → 对话 → 资源库 → 文档。
    #[test]
    fn top_level_is_six_in_lao_dao_order() {
        let reg = crate::modules::build_registry();
        let got = ids(&reg.top_level());
        // Mr2109 2026-09-13 二次调整后的顺序：主控在线 → 对话 → 任务 → 模型 → 资源库 → 虫茧
        assert_eq!(
            got,
            vec!["main-online", "chat", "tasks-group", "models-group", "resources", "roundtable"],
            "一级六项顺序（二次调整）；虫茧仍是普通箱，占第 6 位"
        );
        // 「文档」已移入虫茧平台 ⇒ **不再是一级导航项**
        assert!(!got.iter().any(|x| x == "docs"), "文档不应出现在一级导航：{:?}", got);
    }

    /// 设计 §4.1/§七18：children_of("main-online") 顺序 = 集群/文件浏览器/升级/Git/日志。
    #[test]
    fn children_of_main_online_order() {
        let reg = crate::modules::build_registry();
        let got = ids(&reg.children_of("main-online"));
        assert_eq!(
            got,
            vec!["cluster", "file-browser", "upgrade", "git", "logs"],
            "主控在线的页签顺序（Mr2109 verbatim：集群、文件浏览器、升级、git、日志——恰五项）"
        );
    }

    #[test]
    fn children_of_tasks_and_models_groups() {
        let reg = crate::modules::build_registry();
        assert_eq!(ids(&reg.children_of("tasks-group")), vec!["tasks", "internal-tasks"]);
        assert_eq!(ids(&reg.children_of("models-group")), vec!["models", "model-registry"]);
        // 二次调整（Mr2109纠正版）：文档归属虫茧平台，是平台的**应用卡**而非页签
        assert_eq!(ids(&reg.children_of("roundtable")), vec!["docs"]);
        assert_eq!(reg.platform_apps(), vec!["docs", "roundtable"]);
        assert!(reg.children_of("chat").is_empty(), "无子箱的顶级箱 children_of 应为空");
        assert!(reg.children_of("nope").is_empty());
    }

    /// 卸下的子箱自动跳过（设计 §4.2 规则 4「不留孤儿」）。
    #[test]
    fn children_of_skips_disabled() {
        let mut reg = crate::modules::build_registry();
        reg.enabled.insert("git".into(), false);
        let got = ids(&reg.children_of("main-online"));
        assert!(!got.iter().any(|x| x == "git"), "卸下的 git 不应出现在页签清单");
    }

    /// E17：effective_module 三态（记忆子箱 / 回退父箱首子箱 / 父箱无子箱回自己）。
    #[test]
    fn effective_module_three_states() {
        let mut reg = crate::modules::build_registry();
        // ① 记忆子箱可用 → 用它
        assert_eq!(reg.effective_module("main-online", "git"), "git");
        // ② 记忆子箱被卸下/不存在 → 回退到该父箱 order 最小的启用子箱
        reg.enabled.insert("cluster".into(), false);
        assert_eq!(reg.effective_module("main-online", "no-such-child"), "file-browser");
        assert_eq!(reg.effective_module("main-online", "cluster"), "file-browser"); // 记忆的也被卸
        // ③ 父箱无启用子箱 → 返回父箱自己
        for id in ["cluster", "file-browser", "upgrade", "git", "logs", "roundtable"] {
            reg.enabled.insert(id.to_string(), false);
        }
        assert_eq!(reg.effective_module("main-online", ""), "main-online");
        assert_eq!(reg.effective_module("main-online", "git"), "main-online");
        // 非父箱原样返回（含子箱与顶级箱）
        assert_eq!(reg.effective_module("chat", ""), "chat");
        assert_eq!(reg.effective_module("git", ""), "git");
    }

    /// E16：parent_of + 面包屑键序列（HUD「父 › 子」）。
    #[test]
    fn parent_of_and_breadcrumb_keys() {
        let reg = crate::modules::build_registry();
        assert_eq!(reg.parent_of("git"), Some("main-online"));
        assert_eq!(reg.parent_of("file-browser"), Some("main-online"));
        assert_eq!(reg.parent_of("internal-tasks"), Some("tasks-group"));
        assert_eq!(reg.parent_of("model-registry"), Some("models-group"));
        assert_eq!(reg.parent_of("chat"), None);
        assert_eq!(reg.parent_of("nope"), None);
        // HUD 面包屑：子页 = [父名键, 子名键]；一级页 = [自身名键]
        assert_eq!(reg.breadcrumb_keys("git"), vec!["mod.main_online.name", "mod.git.name"]);
        assert_eq!(
            reg.breadcrumb_keys("internal-tasks"),
            vec!["mod.tasks_group.name", "mod.internal_tasks.name"]
        );
        assert_eq!(reg.breadcrumb_keys("chat"), vec!["mod.chat.name"]);
        assert!(reg.breadcrumb_keys("nope").is_empty());
    }

    /// E15：卸箱回退落到所属父箱的首个子箱（原来写死 "tasks"）。
    #[test]
    fn fallback_after_disable_lands_on_parents_first_child() {
        let reg = crate::modules::build_registry();
        assert_eq!(reg.fallback_after_disable("git"), "cluster");
        assert_eq!(reg.fallback_after_disable("internal-tasks"), "tasks");
        // 文档归属虫茧平台（非 group）⇒ 卸下后回退到**平台自身**（栅格照常，只是少一张卡）
        assert_eq!(reg.fallback_after_disable("docs"), "roundtable");
        assert_eq!(reg.sub_tabs_parent_of("docs"), None, "归属平台的应用不该有二级页签");
        assert_eq!(reg.sub_tabs_parent_of("cluster"), Some("main-online"));
        assert_eq!(reg.fallback_after_disable("chat"), "chat"); // 真正的无父一级箱 → 默认 chat
    }

    /// 父箱：is_group + 不可卸（toggle 拒绝）；图标名真实存在；子箱 toggle 生效。
    #[test]
    fn groups_unloadable_protected_and_icons_resolve() {
        let mut reg = crate::modules::build_registry();
        for g in ["main-online", "tasks-group", "models-group"] {
            let m = reg.find(g).unwrap();
            assert!(m.is_group, "{} 必须是父箱（is_group）", g);
            assert!(m.is_core, "{} 必须不可卸（is_core）", g);
            assert_eq!(
                m.icon.chars().count(),
                1,
                "{} 图标名未收录（回退成了文本）: {}",
                g, m.icon
            );
            reg.toggle(g); // 尝试卸下
            assert!(reg.is_enabled(g), "父箱 {} 不可被卸下（导航骨架）", g);
        }
        // 子箱 toggle 生效
        reg.toggle("git");
        assert!(!reg.is_enabled("git"));
    }
}
