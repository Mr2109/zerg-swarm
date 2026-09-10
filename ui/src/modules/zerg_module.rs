//! 集装箱标准接口（v2.5.6 UI 蓝图——Mr2109 2026-08-29）
//!
//! 虫族 UI = 集装箱船：船体（核心）标准化，集装箱（模块）可吊装可卸下。
//! 本文件定义"集装箱标准"——任何独立应用套上这个接口 = 一个集装箱。
//! 设计借鉴 dsh/Cordis（贡献 + 可逆生命周期）、VS Code（自举验证/懒加载）。
//!
//! M1 阶段：定义标准 + 注册表元数据 + 顶部导航布局（渲染仍走 ZergApp 现有方法）。
//! M3 阶段：Ferrite 适配器用 trait 对象实现（真集装箱——升级=换 crate 版本——船体不动）。
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
}

/// 集装箱标准接口——任何独立应用实现它 = 一个集装箱
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

/// 模块注册表（吊装系统——船体管理所有集装箱）
///
/// M1 阶段：元数据注册表——顶部导航渲染 + 启用状态管理。
/// 渲染仍由 ZergApp 现有方法承担（match id）——功能不变风险最小。
/// M3 起：Ferrite 等独立应用以 trait 对象注册——真集装箱。
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
}

impl Default for ModuleRegistry {
    fn default() -> Self {
        Self {
            modules: Vec::new(),
            external: Vec::new(),
            active: "chat".to_string(), // P4-19 默认对话模块（对话最常用——原 tasks 启动停在任务队列）
            enabled: std::collections::HashMap::new(),
        }
    }
}

impl ModuleRegistry {
    /// 注册一个集装箱（吊装上船）
    pub fn register(&mut self, m: ModuleManifest) {
        let id = m.id.to_string();
        self.modules.push(m);
        // 核心箱永驻在船；可装卸箱默认在船。
        // M32(2026-09-10 审计)：原来这里 `let is_core = m.is_core; ... let _ = is_core;`
        // 是纯死代码（读出来又丢弃），已删除；核心箱保护由 `toggle()` 用同一字段实现。
        self.enabled.insert(id, true);
    }

    /// 可显示的模块列表（启用的——顶部导航）
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

    /// 加载外部生态箱（M4——/tmp/zerg-ui/external-modules.json 配置文件声明）
    /// 格式: {"modules": [{"id","name","icon","description","version","url"}]}
    /// 找不到文件 = 无外部箱（正常）；读取/解析失败（M33 2026-09-10 审计：不再静默）
    /// 打一次日志便于定位（用户写的 JSON 写错时不再无声无息）。
    pub fn load_external(&mut self) {
        let path = crate::api::ui_dir().join("external-modules.json");
        let s = match std::fs::read_to_string(&path) {
            Ok(s) => s,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return, // 无文件=正常
            Err(e) => {
                eprintln!("[modules] 读取外部模块配置失败 {}: {}", path.display(), e); // M33
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
                eprintln!("[modules] 解析外部模块配置失败 {}: {}", path.display(), e); // M33
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

    /// 切换启用状态（可装卸箱——核心箱不可禁用）
    pub fn toggle(&mut self, id: &str) {
        if let Some(m) = self.find(id) {
            if m.is_core {
                return; // 船体箱不可卸
            }
        }
        let e = self.enabled.entry(id.to_string()).or_insert(true);
        *e = !*e;
    }

    /// 持久化——哪些箱在船上（/tmp/zerg-ui/modules.json）
    /// 只存 enabled 状态——模块清单是代码内建的（集装箱注册）
    pub fn save(&self) {
        let dir = crate::api::ui_dir();
        // M33(2026-09-10 审计): 目录/写盘失败不再静默——用户禁用/启用选择重启即失效却无感知
        if let Err(e) = std::fs::create_dir_all(&dir) {
            eprintln!("[modules] 创建配置目录失败 {}: {}", dir.display(), e);
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
                    eprintln!("[modules] 保存模块状态失败 {}: {}", path, e); // M33
                }
            }
            Err(e) => eprintln!("[modules] 序列化模块状态失败: {}", e), // M33
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
                eprintln!("[modules] 读取模块状态失败 {}: {}", path.display(), e); // M33
                return;
            }
        };
        let disabled: std::collections::BTreeMap<String, bool> = match serde_json::from_str(&s) {
            Ok(d) => d,
            Err(e) => {
                eprintln!("[modules] 解析模块状态失败 {}: {}", path.display(), e); // M33
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
}
