// 虫族 UI 主程序（eframe::App——三栏布局——任务视图——离线全灰）
// 异步: 自己管理（tokio spawn + SharedResult——不用 egui-async——eframe 0.36 兼容问题）
use eframe::egui;
use rust_i18n::t;

use crate::api::{self, TaskInfo};
use crate::modules::icons::icon_text; // P3 图标（iconflow）
use crate::modules::zerg_module; // 模块注册表（平台页 id 常量——C9 第 1 步）

// APP-A15（2026-09-10 审计）: 删除死代码 `struct AsyncData<T>` 及其 take()/is_done()。
// 依据：全文件 grep 只有这里的声明与 impl，从未被实例化（模块内改用
// `Arc<Mutex<Option<..>>>` + 直接 take）。删除原因：它没有任何调用点，
// 且 take() 里的 `and_then(|r| r.ok())` 又是一处吞错模板，留着只会误导维护者。
// 若将来需要「每帧检查 + 消费」的通用封装，请连同错误通道一起设计（不要吞 Err）。

use std::sync::{Arc, Mutex};

pub struct ZergApp {
    // 连接状态
    online: bool,
    // v2.5.7 HUD 悬浮层（右上——core 状态/模块/running 数——点 ✕ 隐藏本会话）
    hud_hidden: bool,
    // 当前语言（默认中文）
    locale: String,
    // 数据（异步——自己管理）
    tasks: Arc<Mutex<Option<Vec<TaskInfo>>>>,
    // 任务已加载（区分"拉取中"vs"暂无任务"）
    tasks_loaded: bool,
    online_result: Arc<Mutex<Option<bool>>>,
    // 2026-09-10 审计 APP-A04/A11: 最近一次轮询/操作失败提示——失败时保留旧数据并红字提示（不静默）
    poll_err: Arc<Mutex<Option<String>>>,
    task_detail: Arc<Mutex<Option<serde_json::Value>>>,
    git_status: Arc<Mutex<Option<api::GitStatusResp>>>,
    logs: Arc<Mutex<Option<Vec<String>>>>,
    // file-browser 薄壳箱的组件实例（**独立状态**——它自己那套根/选中/内容，与别处互不干扰）
    fb: crate::modules::filebrowse::FileBrowse,
    // v2.5.7 对话模块（Mr2109——完全借鉴 Hermes——第一板块）
    chat_view: crate::modules::chat::ChatView,
    resources: Arc<Mutex<Option<serde_json::Value>>>,
    // 集群状态
    cluster: Arc<Mutex<Option<serde_json::Value>>>,
    // 资源库类型（models/tools/skills/mcp）
    res_type: String,
    // 资源表格排序（列: 0名称/1状态/2次数/3故障/4时间/5简介——Mr2109 2026-08-21）
    res_sort_col: i32,
    res_sort_desc: bool,
    // 选中的模型（点击显示简介——Mr2109 2026-08-20）
    selected_model: Option<String>,
    selected_model_desc: String,
    model_detail: std::sync::Arc<std::sync::Mutex<Option<serde_json::Value>>>, // v2.5.6 模型详情（适配器选项+加载状态）
    model_tab: String, // v2.5.6 模型右栏标签页: "detail"=模型详情 / "adapter"=适配器选项
    adapter_edit: std::sync::Arc<
        std::sync::Mutex<
            std::collections::HashMap<String, std::collections::HashMap<String, serde_json::Value>>,
        >,
    >, // v2.5.6 适配器编辑缓冲（模型→键值）
    adapter_msg: String, // v2.5.6 适配器应用结果提示
    adapter_confirm: bool, // v2.5.6 适配器修改确认态（Mr2109 2026-08-27——点应用后确认才生效）
    split_model: f32,  // v2.5.6 模型库左右分割比例（可拖拽——持久化）
    split_it: f32,     // v2.5.6 内部任务左右分割比例（可拖拽——持久化）
    adapter_schema: std::sync::Arc<std::sync::Mutex<Option<serde_json::Value>>>, // v2.5.6 当前模型适配器 schema（编辑控件渲染）
    // 触发计时
    last_ping: f64,
    last_tasks: f64,
    last_detail: f64,
    last_git: f64,
    last_logs: f64,
    last_res: f64,
    last_cluster: f64,
    // 导航（v2.5.6 虫茧注册表——顶部导航——Mr2109 2026-08-29）
    registry: crate::modules::ModuleRegistry,
    // v2.5.6 模块管理面板开关（➕ 吊装系统——M2）
    show_module_manager: bool,
    // C9 第 1 步（2026-09-13）：虫茧改**按契约装载**——宿主只认
    // `CocoonMeta`（铭牌）+ `Cocoon::render`（吊点），不再持有某个茧的具体类型。
    // 已装载的茧实例（懒加载缓存：第一次「打开」才建——示例虫茧 new() 会开库 + 起 runtime）。
    // 用 trait 对象 ⇒ 新增一个茧宿主零改动（登记见 modules/cocoon.rs 的 REGISTERED 表）。
    // 2026-09-11 B 批（决策 5）的口径不变：跨仓茧未启用 feature ⇒ 实例为空，平台卡片显「未装载」。
    cocoons: Vec<Box<dyn crate::modules::cocoon::Cocoon>>,
    // T8 虫茧平台态（false=平台启动器应用栅格；true=示例虫茧全屏）
    /// 虫茧平台里**当前打开的应用卡**（None = 停在平台栅格）。2026-09-13 Mr2109纠正：
    /// 「文档」是平台的一张独立应用卡（与示例虫茧平级），不是虫茧的子标签 ⇒ 用箱 id 记录。
    cocoon_app: Option<String>,
    /// C9 第 3 步：**宿主能力通道**（借用宿主的 Ferrite 编辑器池 + 共享令牌）——**跨帧存活**
    /// （编辑器池要保光标/撤销/滚动）。宿主每帧只清 `exit_requested` 就交给茧（见 `cocoon_view`）。
    cocoon_ctx: crate::modules::cocoon::CocoonCtx,
    /// C9 第 3 步：正在显示「安装指引」的**未装载茧** id（None = 没弹窗）——点平台卡「安装」时置位。
    cocoon_install_hint: Option<String>,
    // 选中的任务
    selected_task: Option<TaskInfo>,
    // 当前请求的任务 ID
    detail_id: String,
    // 刷新计时
    last_refresh: f64,
    // 内部任务清单（Mr2109 2026-08-22）
    internal_tasks: std::sync::Arc<std::sync::Mutex<Option<Vec<serde_json::Value>>>>,
    last_it_fetch: std::time::Instant,
    // APP-A06: 周期配置独立计时器（原来共用 last_it_fetch——被 30s 清单刷新归零后 60s 条件永不成立）
    last_it_interval_fetch: std::time::Instant,
    // A07 治本（2026-09-10）：引擎状态=后端单一真相源（10s 轮询）——本地不再持有“真相”，只做乐观提示
    // 丙批 N4（2026-09-10）：前缀缓存命中率（网关 8082——30s 轮询）
    prefix_cache: api::SharedResult<serde_json::Value>,
    last_pc_fetch: std::time::Instant,
    engine_state: api::SharedResult<serde_json::Value>,
    last_engine_fetch: std::time::Instant,
    it_ctrl_busy: Option<bool>, // 请求在飞（按钮显示“切换中…”）
    // APP-A07: 启停结果回报——成功才刷新状态；失败红字提示且不改显示
    it_ctrl_result: api::SharedResult<()>,
    it_ctrl_target: Option<bool>, // 待生效的目标值（后端成功返回后套用）
    it_ctrl_confirm: bool,        // 二次确认态（防误点启停内部任务引擎）
    it_selected: Option<String>,  // v2.5.6 内部任务选中（左右布局——右侧详情+skill）
    it_intervals: std::sync::Arc<std::sync::Mutex<Option<serde_json::Value>>>, // v2.5.6 周期配置缓存
    it_custom_for: Option<String>, // v2.5.6 正在指定周期（任务 id）
    it_custom_hours: String,       // v2.5.6 自定义周期输入
    // 归档列表（Mr2109 2026-08-22）
    archive: std::sync::Arc<std::sync::Mutex<Option<Vec<serde_json::Value>>>>,
    last_archive_fetch: std::time::Instant,
}

impl ZergApp {
    pub fn new() -> Self {
        // 2026-09-13 Q10/E18：偏好 + 布局的落点迁移（首访一次性；目标已存在绝不覆盖；测试环境跳过）
        crate::api::migrate_persistent_state_once();
        // C9 第 1 步：把在册的茧打一行日志（诊断「平台上有哪些茧、装载没装载」；一次）
        crate::modules::cocoon::log_catalog_once();
        Self {
            online: false,
            hud_hidden: false,
            locale: crate::detect_locale(), // 决策②：prefs → 系统语言 → en
            tasks: Arc::new(Mutex::new(None)),
            tasks_loaded: false,
            online_result: Arc::new(Mutex::new(None)),
            poll_err: Arc::new(Mutex::new(None)),
            task_detail: Arc::new(Mutex::new(None)),
            git_status: Arc::new(Mutex::new(None)),
            logs: Arc::new(Mutex::new(None)),
            fb: crate::modules::filebrowse::FileBrowse::new(),
            chat_view: crate::modules::chat::ChatView::new(),
            resources: Arc::new(Mutex::new(None)),
            cluster: Arc::new(Mutex::new(None)),
            res_type: "tools".to_string(), // 资源库默认工具库（模型库已独立板块——2026-08-27）
            res_sort_col: 0,
            res_sort_desc: false,
            selected_model: None,
            selected_model_desc: String::new(),
            model_detail: std::sync::Arc::new(std::sync::Mutex::new(None)),
            model_tab: "detail".to_string(),
            adapter_edit: std::sync::Arc::new(std::sync::Mutex::new(
                std::collections::HashMap::new(),
            )),
            adapter_msg: String::new(),
            adapter_confirm: false,
            adapter_schema: std::sync::Arc::new(std::sync::Mutex::new(None)),
            split_model: load_layout_ratio("split_model", 0.32), // v2.5.6 布局持久化（Mr2109——拖动后下次默认）
            split_it: load_layout_ratio("split_it", 0.36),
            last_ping: 0.0,
            last_tasks: 0.0,
            last_detail: 0.0,
            last_git: 0.0,
            last_logs: 0.0,
            last_res: 0.0,
            last_cluster: 0.0,
            registry: {
                let mut r = crate::modules::build_registry();
                r.load(); // v2.5.6 恢复持久化状态（哪些箱卸下了）
                r.load_state(); // 2026-09-13（Q9）：恢复导航选中（父 + 子）
                r
            },
            show_module_manager: false,
            // C9 第 1 步：已装载的茧实例（空起步——打开平台栅格里的应用卡时才懒建）
            cocoons: Vec::new(),
            cocoon_app: None,
            // C9 第 3 步：宿主能力通道——令牌从宿主既有解析链取（`api::api_token`），只驻内存。
            cocoon_ctx: crate::modules::cocoon::CocoonCtx::with_host_token(
                crate::modules::cocoon::host_token(),
            ),
            cocoon_install_hint: None,
            selected_task: None,
            detail_id: String::new(),
            last_refresh: 0.0,
            internal_tasks: std::sync::Arc::new(std::sync::Mutex::new(None)),
            last_it_fetch: std::time::Instant::now(),
            last_it_interval_fetch: std::time::Instant::now(),
            prefix_cache: Arc::new(Mutex::new(None)),
            last_pc_fetch: std::time::Instant::now(),
            engine_state: Arc::new(Mutex::new(None)),
            last_engine_fetch: std::time::Instant::now(),
            it_ctrl_busy: None,
            it_ctrl_result: Arc::new(Mutex::new(None)),
            it_ctrl_target: None,
            it_ctrl_confirm: false,
            it_selected: None,
            it_intervals: std::sync::Arc::new(std::sync::Mutex::new(None)),
            it_custom_for: None,
            it_custom_hours: String::new(),
            archive: std::sync::Arc::new(std::sync::Mutex::new(None)),
            last_archive_fetch: std::time::Instant::now(),
        }
    }

    /// 每帧更新（定时触发异步请求 + 收集结果）
    fn update_async(&mut self, now: f64) {
        // 在线探测（3s）
        // APP-A05（2026-09-10 审计）: 删掉 ping_async() 那一路——它内部已经发一次 /api/tasks，
        // 句柄又被丢弃，等于每 3s 发两次全量任务拉取；改用轻量 /api/capabilities 判活
        // （任务接口偶发报错不该把整页判成"主控离线"）。
        if now - self.last_ping > 3.0 {
            self.last_ping = now;
            let store = self.online_result.clone();
            api::runtime().spawn(async move {
                let ok = api::sync_get_public("/api/capabilities").await.is_ok();
                *lock_recover(&store) = Some(ok);
            });
        }
        // 收集在线结果
        if let Some(ok) = lock_recover(&self.online_result).take() {
            self.online = ok;
        }
        // 丙批 N4（2026-09-10）：前缀缓存命中率轮询（网关 8082——30s）
        if lock_recover(&self.prefix_cache).is_none()
            || self.last_pc_fetch.elapsed().as_secs() >= 30
        {
            let store = self.prefix_cache.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                let r = api::fetch_prefix_cache_blocking().await;
                *lock_recover(&store) = Some(r);
            });
            self.last_pc_fetch = now;
        }
        // A07 治本（2026-09-10）：引擎状态轮询（10s）——服务端为唯一真相源
        if lock_recover(&self.engine_state).is_none()
            || self.last_engine_fetch.elapsed().as_secs() >= 10
        {
            let store = self.engine_state.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                let r = api::fetch_internal_state_blocking().await;
                *lock_recover(&store) = Some(r);
            });
            self.last_engine_fetch = now;
        }
        // APP-A07: 启停结果——成功则立刻回读真实状态（不再靠本地翻转）；失败只提示
        if let Some(r) = lock_recover(&self.it_ctrl_result).take() {
            self.it_ctrl_busy = None;
            self.it_ctrl_target = None;
            match r {
                Ok(()) => {
                    // 立即重拉状态（把“切换中…”换成后端事实）
                    self.last_engine_fetch =
                        std::time::Instant::now() - std::time::Duration::from_secs(60);
                    *lock_recover(&self.poll_err) = None;
                }
                Err(e) => {
                    *lock_recover(&self.poll_err) =
                        Some(t!("err.internal_tasks", err = e).to_string());
                }
            }
        }
        // 在线后拉任务（3s）
        if self.online && now - self.last_tasks > 3.0 {
            self.last_tasks = now;
            let store = self.tasks.clone();
            let perr = self.poll_err.clone();
            api::runtime().spawn(async move {
                // APP-A04: 失败保留旧值（原实现 .ok() 写 None，一次抖动就把队列变成"暂无任务"）
                match api::fetch_tasks_blocking().await {
                    Ok(v) => {
                        *lock_recover(&store) = Some(v);
                        *lock_recover(&perr) = None;
                    }
                    Err(e) => *lock_recover(&perr) = Some(t!("task.list_err", err = e).to_string()),
                }
            });
        }
        // 任务已加载标记（主线程——store 有值=已拉到）
        if !self.tasks_loaded && lock_recover(&self.tasks).is_some() {
            self.tasks_loaded = true;
        }
        // 在线后拉 Git 状态（10s——数据常驻——点击即有内容）
        if self.online && now - self.last_git > 10.0 {
            self.last_git = now;
            let store = self.git_status.clone();
            let perr = self.poll_err.clone();
            api::runtime().spawn(async move {
                // APP-A04: 失败保留旧值
                match api::fetch_git_status_blocking().await {
                    Ok(v) => {
                        *lock_recover(&store) = Some(v);
                        *lock_recover(&perr) = None;
                    }
                    Err(e) => *lock_recover(&perr) = Some(t!("err.git", err = e).to_string()),
                }
            });
        }
        // 拉主控日志（10s）
        if self.online && now - self.last_logs > 10.0 {
            self.last_logs = now;
            let store = self.logs.clone();
            let perr = self.poll_err.clone();
            api::runtime().spawn(async move {
                // APP-A04: 失败保留旧值
                match api::fetch_logs_blocking().await {
                    Ok(v) => {
                        *lock_recover(&store) = Some(v);
                        *lock_recover(&perr) = None;
                    }
                    Err(e) => *lock_recover(&perr) = Some(t!("err.main_logs", err = e).to_string()),
                }
            });
        }
        // 拉资源库（30s——用当前类型——Mr2109 2026-08-27 修复: 之前硬编码 models 导致资源库被刷成模型库）
        if self.online && now - self.last_res > 30.0 {
            self.last_res = now;
            let store = self.resources.clone();
            let rt = self.res_type.clone();
            let perr = self.poll_err.clone();
            api::runtime().spawn(async move {
                // APP-A04: 失败保留旧值（原实现把资源库刷成空/加载中）
                match api::fetch_resources_blocking(rt).await {
                    Ok(v) => {
                        *lock_recover(&store) = Some(v);
                        *lock_recover(&perr) = None;
                    }
                    Err(e) => *lock_recover(&perr) = Some(t!("err.resources", err = e).to_string()),
                }
            });
        }
        // 拉集群状态（10s）
        if self.online && now - self.last_cluster > 10.0 {
            self.last_cluster = now;
            let store = self.cluster.clone();
            let perr = self.poll_err.clone();
            api::runtime().spawn(async move {
                // APP-A04: 失败保留旧值
                match api::fetch_cluster_blocking().await {
                    Ok(v) => {
                        *lock_recover(&store) = Some(v);
                        *lock_recover(&perr) = None;
                    }
                    Err(e) => *lock_recover(&perr) = Some(t!("err.cluster", err = e).to_string()),
                }
            });
        }
    }

    /// 渲染主视图（v2.5.6——内容区由虫茧注册表分发）
    /// 渲染任务视图（主区——队列 + 详情各占一半——水平布局）
    fn tasks_view(&mut self, ui: &mut egui::Ui) {
        // 设计 §4.4：删重复标题块。
        let list: Vec<TaskInfo> = lock_recover(&self.tasks).clone().unwrap_or_default();
        // 默认选中最新任务（第一条——加载后自动）
        if self.selected_task.is_none() && !list.is_empty() {
            let first = list[0].clone();
            self.selected_task = Some(first.clone());
            let id = first.id.clone().unwrap_or_default();
            self.detail_id = id.clone();
            let store = self.task_detail.clone();
            let perr = self.poll_err.clone();
            api::runtime().spawn(async move {
                // APP-A04: 失败保留旧详情 + 提示（原实现失败写 None → 右栏永久 spinner）
                match api::fetch_task_detail_blocking(id).await {
                    Ok(v) => *lock_recover(&store) = Some(v),
                    Err(e) => {
                        *lock_recover(&perr) = Some(t!("task.detail_err", err = e).to_string())
                    }
                }
            });
        }
        // APP-A13（2026-09-10 审计）: 详情头部每帧用最新列表回查（原来用点击那一刻的快照——
        // 任务 running→done 后详情仍显示旧状态/旧机器/旧时长，与左栏自相矛盾）
        if !self.detail_id.is_empty() {
            if let Some(fresh) = list
                .iter()
                .find(|t| t.id.as_deref() == Some(self.detail_id.as_str()))
            {
                self.selected_task = Some(fresh.clone());
            }
        }
        // 归档折叠区（Mr2109 2026-08-22——30 天归档/90 天删除可查）
        // APP-A19（2026-09-10 审计）: ① 整块从下方 `if list.is_empty() { … return; }` 之后
        // 挪到之前——队列清空（任务全删/全归档）时「归档」恰恰是最有用的面板，原来直接消失；
        // ② 拉取失败不再伪装成「暂无归档」（原来 .unwrap_or_default() 把 Err 变成空数组）——
        // 失败保留旧值 + 顶部提示条 + 面板内区分「加载中/拉取失败」。
        if lock_recover(&self.archive).is_none() || self.last_archive_fetch.elapsed().as_secs() > 60
        {
            let store = self.archive.clone();
            let perr = self.poll_err.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                match api::fetch_archive_blocking().await {
                    Ok(items) => *lock_recover(&store) = Some(items),
                    Err(e) => {
                        *lock_recover(&perr) = Some(t!("task.archive_err", err = e).to_string())
                    }
                }
            });
            self.last_archive_fetch = now;
        }
        let arc_snap = lock_recover(&self.archive).clone();
        let arc_count = arc_snap.as_ref().map(|v| v.len()).unwrap_or(0);
        egui::CollapsingHeader::new(t!("task.archive_group", n = arc_count))
            .id_salt("task_group_archive") // APP-A08: 稳定 id（标题含计数——每帧变会让展开态被重置）
            .default_open(false)
            .show(ui, |ui| {
                match &arc_snap {
                    Some(arcs) => {
                        if arcs.is_empty() {
                            ui.weak(t!("task.no_archive"));
                        }
                        for a in arcs {
                            let tid = a
                                .get("task_id")
                                .and_then(|v| v.as_str())
                                .unwrap_or("?")
                                .to_string();
                            let when = a
                                .get("archived")
                                .and_then(|v| v.as_str())
                                .unwrap_or("?")
                                .to_string();
                            let size = a.get("size").and_then(|v| v.as_i64()).unwrap_or(0);
                            // APP-A16: 尾 6 字符/前 16 字符都按「字符」切（原来按字节下标——
                            // 多字节 task_id/时间串会切在字符中间 panic，整个 UI 退出）
                            ui.label(t!(
                                "task.archive_row",
                                id = short_id(&tid),
                                when = when.chars().take(16).collect::<String>(),
                                kb = size / 1024
                            ));
                        }
                    }
                    // APP-A19: None = 还没拉到或拉取失败——不再谎报「暂无归档」
                    None => {
                        ui.weak(t!("task.archive_loading"));
                    }
                }
            });
        ui.add_space(4.0);
        if list.is_empty() {
            // 已加载但无任务——显示"暂无任务"（不是"拉取中"）
            if self.tasks_loaded {
                ui.weak(t!("task.none"));
            } else {
                ui.spinner();
                ui.weak(t!("common.loading"));
            }
            return;
        }
        // 两列布局（队列 | 详情——ui.columns 均分——宽度正确）
        ui.columns(2, |cols| {
            // 左列——任务队列（ScrollArea 限制高度——超出可滚——独立 ID 防串扰）
            let avail_h = cols[0].available_height().max(200.0);
            egui::ScrollArea::vertical()
                .id_salt("queue_scroll")
                .max_height(avail_h)
                .show(&mut cols[0], |ui| {
                    let running: Vec<_> = list
                        .iter()
                        .filter(|t| t.status.as_deref() == Some("running"))
                        .collect();
                    let queued: Vec<_> = list
                        .iter()
                        .filter(|t| {
                            let s = t.status.as_deref().unwrap_or("");
                            s == "queued" || s == "paused"
                        })
                        .collect();
                    // APP-A03（2026-09-10 审计）: 后端会产出 waiting_retry（环境故障挂起，机器恢复自动重派）
                    // 原实现三个分组都不含它 → 任务在界面凭空消失、右键不可达。补专门分组。
                    let waiting: Vec<_> = list
                        .iter()
                        .filter(|t| t.status.as_deref() == Some("waiting_retry"))
                        .collect();
                    let done: Vec<_> = list
                        .iter()
                        .filter(|t| {
                            let s = t.status.as_deref().unwrap_or("");
                            s == "done" || s == "failed"
                        })
                        .collect();
                    // APP-A03: 其它未知状态兜底显示——任何状态都不许凭空消失
                    let others: Vec<_> = list
                        .iter()
                        .filter(|t| {
                            let s = t.status.as_deref().unwrap_or("");
                            !matches!(
                                s,
                                "running"
                                    | "queued"
                                    | "paused"
                                    | "done"
                                    | "failed"
                                    | "waiting_retry"
                            )
                        })
                        .collect();
                    // Mr2109: 执行完成组按完成时间降序（最新在最上面——固定不随刷新变）
                    let mut done = done.clone();
                    done.sort_by(|a, b| {
                        let pa = a.completed_at.as_deref().unwrap_or("").to_string();
                        let pb = b.completed_at.as_deref().unwrap_or("").to_string();
                        pb.cmp(&pa)
                    });
                    // Mr2109 2026-08-20: 复查任务排在执行任务下级（缩进——一眼看出对应关系）
                    // 复查任务（ref_task_id 非空）→ 不单独显示——渲染到对应执行任务下方
                    let reviews: Vec<TaskInfo> = done
                        .iter()
                        .filter(|t| {
                            t.ref_task_id
                                .as_deref()
                                .map(|r| !r.is_empty())
                                .unwrap_or(false)
                        })
                        .map(|t| (*t).clone())
                        .collect();
                    let done_main: Vec<&TaskInfo> = done
                        .iter()
                        .filter(|t| {
                            t.ref_task_id
                                .as_deref()
                                .map(|r| r.is_empty())
                                .unwrap_or(true)
                        })
                        .copied()
                        .collect();

                    let running_count = running.len();
                    egui::CollapsingHeader::new(format!(
                        "🔄 {}（{}）",
                        t!("task.state_running"),
                        running_count
                    ))
                    .id_salt("task_group_running") // APP-A08: 稳定 id
                    .default_open(true)
                    .show(ui, |ui| {
                        for t in &running {
                            self.task_row(ui, t);
                        }
                        if running_count == 0 {
                            ui.weak(t!("common.none"));
                        }
                    });
                    let queued_count = queued.len();
                    egui::CollapsingHeader::new(format!(
                        "⏳ {}（{}）",
                        t!("task.state_queued"),
                        queued_count
                    ))
                    .id_salt("task_group_queued") // APP-A08: 稳定 id
                    .default_open(true)
                    .show(ui, |ui| {
                        for t in &queued {
                            self.task_row(ui, t);
                        }
                        if queued_count == 0 {
                            ui.weak(t!("common.none"));
                        }
                    });
                    let waiting_count = waiting.len();
                    egui::CollapsingHeader::new(t!("task.group_waiting", n = waiting_count))
                        .id_salt("task_group_waiting_retry") // APP-A08: 稳定 id
                        .default_open(true)
                        .show(ui, |ui| {
                            for t in &waiting {
                                self.task_row(ui, t);
                            }
                            if waiting_count == 0 {
                                ui.weak(t!("common.none"));
                            }
                        });
                    let others_count = others.len();
                    if others_count > 0 {
                        egui::CollapsingHeader::new(t!("task.group_others", n = others_count))
                            .id_salt("task_group_others") // APP-A08: 稳定 id
                            .default_open(true)
                            .show(ui, |ui| {
                                for t in &others {
                                    self.task_row(ui, t);
                                }
                            });
                    }
                    let done_count = done_main.len();
                    // Mr2109 2026-08-22: 执行完成任务按日期分组（上级=完成日期——一眼看哪天完成）
                    let mut by_date: std::collections::BTreeMap<String, Vec<&TaskInfo>> =
                        Default::default();
                    for t in &done_main {
                        // APP-A16: 取日期同样按字符切（原 `s[..10]` 是字节下标——含多字节的时间串会 panic）
                        let date = t
                            .completed_at
                            .as_deref()
                            .map(|s| s.chars().take(10).collect::<String>())
                            .unwrap_or_else(|| t!("common.unknown_date").to_string());
                        by_date.entry(date).or_default().push(t);
                    }
                    egui::CollapsingHeader::new(format!(
                        "✅ {}（{}）",
                        t!("task.state_done"),
                        done_count
                    ))
                    .id_salt("task_group_done") // APP-A08: 稳定 id
                    .default_open(false)
                    .show(ui, |ui| {
                        if done_count == 0 {
                            ui.weak(t!("common.none"));
                        }
                        // 日期分组（倒序——最新日期在上——BTreeMap 反序）
                        let dates: Vec<String> = by_date.keys().rev().cloned().collect();
                        for date in dates {
                            let day_tasks = &by_date[&date];
                            egui::CollapsingHeader::new(format!(
                                "📅 {}（{}）",
                                date,
                                day_tasks.len()
                            ))
                            .id_salt(format!("task_day_{}", date)) // APP-A08: 按日期稳定 id
                            .default_open(false)
                            .show(ui, |ui| {
                                for t in day_tasks {
                                    self.task_row(ui, t);
                                    // 复查任务排在执行任务下级（缩进——Mr2109 2026-08-20）
                                    let ref_id = t.id.clone().unwrap_or_default();
                                    for rv in &reviews {
                                        if rv.ref_task_id.as_deref() == Some(ref_id.as_str()) {
                                            ui.indent("review_indent", |ui| {
                                                self.task_row(ui, rv);
                                            });
                                        }
                                    }
                                }
                            });
                        }
                    });
                });
            // 右列——任务详情（ScrollArea 限制高度——独立 ID 防串扰）
            let avail_h2 = cols[1].available_height().max(200.0);
            egui::ScrollArea::vertical()
                .id_salt("detail_scroll")
                .max_height(avail_h2)
                .show(&mut cols[1], |ui| {
                    self.task_detail(ui);
                });
        });
    }

    /// APP-A11（2026-09-10 审计）: 任务操作统一收口——结果不再静默丢弃。
    /// 失败写提示条（poll_err，界面红字）；无论成败都刷新列表，刷新失败保留旧值（APP-A04）。
    fn task_op<F>(&self, fut: F, what: &str)
    where
        F: std::future::Future<Output = Result<(), String>> + Send + 'static,
    {
        let store = self.tasks.clone();
        let perr = self.poll_err.clone();
        let what = what.to_string();
        api::runtime().spawn(async move {
            if let Err(e) = fut.await {
                *lock_recover(&perr) = Some(t!("task.op_failed", what = what, err = e).to_string());
            }
            match api::fetch_tasks_blocking().await {
                Ok(v) => *lock_recover(&store) = Some(v),
                Err(e) => *lock_recover(&perr) = Some(t!("task.list_err", err = e).to_string()),
            }
        });
    }

    /// 任务行（短ID | 描述 | 总执行时间 | 是否成功——通俗易懂）
    fn task_row(&mut self, ui: &mut egui::Ui, t: &TaskInfo) {
        let id = t.id.as_deref().unwrap_or("?");
        let desc = t.description.as_deref().unwrap_or("?");
        let status = t.status.as_deref().unwrap_or("?");
        let color = match status {
            "running" => egui::Color32::from_rgb(80, 180, 255),
            "queued" => egui::Color32::from_rgb(240, 200, 80),
            "waiting_retry" => egui::Color32::from_rgb(255, 170, 60), // APP-A03: 等待重试(环境故障挂起)
            "done" => egui::Color32::from_rgb(80, 200, 120),
            "failed" => egui::Color32::from_rgb(220, 80, 80),
            _ => egui::Color32::GRAY,
        };
        // 短 ID（末 6 位）
        let sid = short_id(id);
        // 复查任务标识（ref_task_id 非空 = 复查任务——前缀 🔍）
        let is_review = t
            .ref_task_id
            .as_deref()
            .map(|r| !r.is_empty())
            .unwrap_or(false);
        let sid_display = if is_review {
            format!("🔍 {}", sid)
        } else {
            sid.to_string()
        };
        // 描述（截断 24 字——防超长）
        let desc_short: String = if desc.chars().count() > 24 {
            desc.chars().take(24).collect::<String>() + "…"
        } else {
            desc.to_string()
        };
        // 总执行时间（created_at → completed_at）
        let dur = task_duration(t.created_at.as_deref(), t.completed_at.as_deref());
        // 是否成功（done=成功 failed=失败 running=… queued=待——用中文——字体兼容）
        let ok = match status {
            "done" => t!("task.ok_success").to_string(),
            "failed" => t!("task.ok_failed").to_string(),
            "running" => "…".to_string(),
            _ => t!("task.ok_pending").to_string(),
        };
        // 失败行红色显示（Mr2109——失败醒目）
        let final_color = if status == "failed" {
            egui::Color32::from_rgb(255, 80, 80) // 亮红——失败醒目
        } else {
            color
        };
        let text = format!("{} | {} | {} | {}", sid_display, desc_short, dur, ok);
        let selected = self
            .selected_task
            .as_ref()
            .map(|s| s.id.as_deref() == Some(id))
            .unwrap_or(false);
        let resp = ui.selectable_label(selected, egui::RichText::new(text).color(final_color));
        if resp.clicked() {
            let id = t.id.clone().unwrap_or_default();
            self.selected_task = Some(t.clone());
            self.detail_id = id.clone();
            // 拉详情
            let store = self.task_detail.clone();
            let perr = self.poll_err.clone();
            api::runtime().spawn(async move {
                // APP-A04: 失败保留旧详情 + 提示（原实现失败写 None → 右栏永久 spinner）
                match api::fetch_task_detail_blocking(id).await {
                    Ok(v) => *lock_recover(&store) = Some(v),
                    Err(e) => {
                        *lock_recover(&perr) = Some(t!("task.detail_err", err = e).to_string())
                    }
                }
            });
        }
        // 右键菜单（Mr2109 2026-08-21: 复制/重跑/置顶置底/上移下移/删除）
        let task_id_owned = t.id.clone().unwrap_or_default();
        let status_owned = status.to_string();
        resp.context_menu(|ui| {
            // 复制任务描述
            if ui
                .button(t!("task.copy_desc", icon = icon_text("copy")))
                .clicked()
            {
                if let Some(desc) = t.description.clone() {
                    ui.ctx().copy_text(desc);
                }
                ui.close();
            }
            // 重跑（failed 任务）
            if status == "failed" && ui.button(t!("task.op_retry")).clicked() {
                let id = task_id_owned.clone();
                // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                self.task_op(
                    async move { api::task_retry_blocking(&id).await },
                    t!("task.op_retry").as_ref(),
                );
                ui.close();
            }
            // 重回排队（done/failed 任务——重新入队——Mr2109 2026-08-21）
            if (status == "done" || status == "failed")
                && ui.button(t!("task.back_to_queue")).clicked()
            {
                let id = task_id_owned.clone();
                // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                self.task_op(
                    async move { api::task_retry_blocking(&id).await },
                    t!("task.back_to_queue").as_ref(),
                );
                ui.close();
            }
            // 暂停/继续（queued 任务——Mr2109 2026-08-21）
            if status_owned == "queued" {
                if ui
                    .button(t!("task.pause", icon = icon_text("pause")))
                    .clicked()
                {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(
                        async move { api::task_pause_blocking(&id, true).await },
                        t!("task.op_pause").as_ref(),
                    );
                    ui.close();
                }
                if ui
                    .button(t!("task.resume", icon = icon_text("play")))
                    .clicked()
                {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(
                        async move { api::task_pause_blocking(&id, false).await },
                        t!("task.op_resume").as_ref(),
                    );
                    ui.close();
                }
                ui.separator();
                if ui.button(t!("task.pin_top")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(
                        async move { api::task_move_blocking(&id, "top").await },
                        t!("task.op_pin_top").as_ref(),
                    );
                    ui.close();
                }
                if ui.button(t!("task.pin_bottom")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(
                        async move { api::task_move_blocking(&id, "bottom").await },
                        t!("task.op_pin_bottom").as_ref(),
                    );
                    ui.close();
                }
                if ui.button(t!("task.move_up")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(
                        async move { api::task_move_blocking(&id, "up").await },
                        t!("task.op_move_up").as_ref(),
                    );
                    ui.close();
                }
                if ui.button(t!("task.move_down")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(
                        async move { api::task_move_blocking(&id, "down").await },
                        t!("task.op_move_down").as_ref(),
                    );
                    ui.close();
                }
                if ui
                    .button(format!("{} {}", icon_text("trash"), t!("action.delete")))
                    .clicked()
                {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(
                        async move { api::task_delete_blocking(&id).await },
                        t!("task.op_delete").as_ref(),
                    );
                    ui.close();
                }
            }
            // 执行中任务操作（running——终止/重回队列——Mr2109 2026-08-22）
            if status_owned == "running" {
                if ui.button(t!("task.terminate")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(
                        async move { api::task_terminate_blocking(&id).await },
                        t!("task.op_terminate").as_ref(),
                    );
                    ui.close();
                }
                if ui.button(t!("task.requeue")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(
                        async move { api::task_requeue_blocking(&id).await },
                        t!("task.op_requeue").as_ref(),
                    );
                    ui.close();
                }
            }
        });
    }

    /// 任务详情（右栏——含时间线）
    fn task_detail(&mut self, ui: &mut egui::Ui) {
        ui.heading(t!("task.detail"));
        ui.add_space(4.0);
        if let Some(t) = &self.selected_task {
            ui.label(format!("ID: {}", t.id.as_deref().unwrap_or("?")));
            ui.label(format!(
                "{}: {}",
                t!("task.desc"),
                t.description.as_deref().unwrap_or("?")
            ));
            ui.label(format!(
                "{}: {}",
                t!("task.type"),
                t.task_type.as_deref().unwrap_or("?")
            ));
            ui.label(format!(
                "{}: {}",
                t!("task.priority"),
                t.priority.unwrap_or(0)
            ));
            ui.label(format!(
                "{}: {}",
                t!("task.status"),
                t.status.as_deref().unwrap_or("?")
            ));
            ui.label(format!(
                "{}: {}",
                t!("task.model"),
                t.model.as_deref().unwrap_or("?")
            ));
            ui.label(format!(
                "{}: {}",
                t!("task.machine"),
                t.machine.as_deref().unwrap_or("?")
            ));
            ui.add_space(8.0);
            ui.separator();
            if let Some(detail) = lock_recover(&self.task_detail).clone() {
                ui.label(t!("task.timeline"));
                if let Some(trace) = detail.get("trace") {
                    if let Some(rounds) = trace.get("rounds").and_then(|r| r.as_array()) {
                        // 内层不再用固定高度 ScrollArea（嵌套滚动冲突——外层统一滚——Mr2109 2026-08-20）
                        for round in rounds {
                            let rn = round.get("round").and_then(|r| r.as_i64()).unwrap_or(0);
                            let tools = round
                                .get("tools")
                                .and_then(|t| t.as_array())
                                .map(|a| a.len())
                                .unwrap_or(0);
                            // 每轮执行时间（Mr2109 2026-08-20——duration 字段）
                            let dur = round.get("duration").and_then(|d| d.as_str()).unwrap_or("");
                            let header = if dur.is_empty() {
                                format!(
                                    "[{}{}] {} {}",
                                    t!("task.round"),
                                    rn,
                                    t!("task.tools"),
                                    tools
                                )
                            } else {
                                // Mr2109 2026-08-21: 耗时排在工具调用后面
                                format!(
                                    "[{}{}] {} {} | ⏱ {}",
                                    t!("task.round"),
                                    rn,
                                    t!("task.tools"),
                                    tools,
                                    dur
                                )
                            };
                            egui::CollapsingHeader::new(header)
                                // APP-A08: 轮次号做稳定 id（耗时字段每轮刷新都可能变——原来一展开就被重置）
                                .id_salt(format!("timeline_round_{}", rn))
                                .show(ui, |ui| {
                                    if let Some(tools_arr) =
                                        round.get("tools").and_then(|t| t.as_array())
                                    {
                                        for tool in tools_arr {
                                            if let Some(name) =
                                                tool.get("name").and_then(|n| n.as_str())
                                            {
                                                ui.label(format!("  └─ {}", name));
                                            }
                                        }
                                    }
                                });
                        }
                    } else {
                        ui.weak(t!("task.no_rounds"));
                    }
                } else {
                    ui.weak(t!("task.no_trace"));
                }
                ui.add_space(8.0);
                ui.separator();
                // S8: 结晶阶段进度卡（subtask_mode=true 时 stages[] 渲染——阶段/目标/状态）
                if detail
                    .get("subtask_mode")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false)
                {
                    if let Some(stages) = detail.get("stages").and_then(|v| v.as_array()) {
                        ui.label(t!("task.stage_crystallize"));
                        for s in stages {
                            let id = s.get("id").and_then(|v| v.as_str()).unwrap_or("?");
                            let goal = s.get("goal").and_then(|v| v.as_str()).unwrap_or("");
                            let status = s
                                .get("status")
                                .and_then(|v| v.as_str())
                                .unwrap_or("pending");
                            let (icon, color) = match status {
                                "done" => ("✅", egui::Color32::from_rgb(76, 175, 80)),
                                "partial" => ("🟡", egui::Color32::from_rgb(255, 193, 7)),
                                "blocked" => ("⛔", egui::Color32::from_rgb(244, 67, 54)),
                                "running" => ("🔄", egui::Color32::from_rgb(33, 150, 243)),
                                _ => ("⏳", egui::Color32::GRAY),
                            };
                            ui.horizontal(|ui| {
                                ui.colored_label(color, format!("{} {}", icon, id));
                                ui.weak(goal);
                            });
                        }
                        ui.add_space(8.0);
                        ui.separator();
                    }
                }
                // 执行报告（直接展开——外层统一滚——不嵌套 ScrollArea）
                if let Some(rep) = detail.get("exec_report").and_then(|r| r.as_str()) {
                    ui.label(t!("task.report_exec"));
                    ui.monospace(rep);
                }
                // 复查报告（直接展开——外层统一滚——不嵌套 ScrollArea）
                if let Some(rep) = detail.get("review_report").and_then(|r| r.as_str()) {
                    ui.add_space(8.0);
                    ui.label(t!("task.report_review"));
                    ui.monospace(rep);
                }
            } else {
                ui.spinner();
                ui.weak(t!("common.loading"));
            }
        } else {
            ui.weak(t!("task.click_hint"));
        }
    }

    /// 渲染模块管理面板（➕ 吊装系统——Mr2109 2026-08-29 M2）
    /// 核心箱（船体——不可禁用）+ 可装卸箱（checkbox 开关——变更即持久化）
    /// 模块管理（➕）——2026-09-13 重构：**镜像顶栏**。
    ///
    /// 旧版按「核心箱 / 可装卸箱」两张平铺清单罗列（导航大调动之前的形状）✗：
    /// 三个父箱被当普通核心箱列着、子箱平铺在下面只会「（父箱名）」小注 ⇒ 面板与顶栏对不上。
    /// 现在：按 `top_level()`（＝顶栏顺序）逐项成组，**父箱作组头（▾ + 🔒）**，其下箱缩进一行；
    /// 虫茧平台里的应用箱（`parent = 虫茧`，如文档）同样落在虫茧下面。全部箱都列（含已卸下的
    /// —— 否则卸下后就再也装不回来）。
    fn module_manager_view(&mut self, ctx: &egui::Context) {
        let mut close = false;
        // 预先收集成纯数据，避免渲染闭包与 registry 的借用冲突。
        // 行：(id, 标签, 简介, 是否锁定, 缩进层级)
        let mut rows: Vec<(String, String, String, bool, usize)> = Vec::new();
        for m in self.registry.top_level() {
            let marker = if m.is_group { " ▾" } else { "" };
            rows.push((
                m.id.to_string(),
                format!("{} {}{}", m.icon, t!(m.name_key), marker),
                t!(m.desc_key).to_string(),
                m.is_core,
                0,
            ));
            // 其下箱：parent == 本箱 的全部箱（父箱的子模块；虫茧平台里的应用箱也在此列）
            let mut kids: Vec<&crate::modules::zerg_module::ModuleManifest> = self
                .registry
                .modules
                .iter()
                .filter(|c| c.parent == Some(m.id))
                .collect();
            kids.sort_by_key(|c| c.order);
            for c in kids {
                rows.push((
                    c.id.to_string(),
                    format!("{} {}", c.icon, t!(c.name_key)),
                    t!(c.desc_key).to_string(),
                    c.is_core,
                    1,
                ));
            }
        }
        let ext_rows: Vec<(String, String, String)> = self
            .registry
            .external
            .iter()
            .map(|m| {
                (
                    m.id.clone(),
                    format!("{} {}", m.icon, m.name),
                    m.description.clone(),
                )
            })
            .collect();

        egui::Window::new(t!("modules.title"))
            .collapsible(false)
            .resizable(true)
            .default_size([480.0, 430.0])
            .show(ctx, |ui| {
                let mut changed = false;
                let mut to_toggle: Option<String> = None;
                ui.label(t!("modules.nav_section"));
                ui.separator();
                egui::ScrollArea::vertical()
                    .max_height(300.0)
                    .auto_shrink([false, true])
                    .show(ui, |ui| {
                        for (id, label, desc, locked, depth) in &rows {
                            ui.horizontal(|ui| {
                                if *depth > 0 {
                                    ui.add_space(22.0);
                                }
                                if *locked {
                                    ui.label(label.as_str());
                                    ui.weak(desc.as_str());
                                    ui.label("🔒").on_hover_text(t!("modules.group_badge"));
                                } else {
                                    let on = self.registry.enabled.get(id).copied().unwrap_or(true);
                                    let mut next = on;
                                    if ui.checkbox(&mut next, label.as_str()).changed()
                                        && next != on
                                    {
                                        to_toggle = Some(id.clone());
                                    }
                                    ui.weak(desc.as_str());
                                }
                            });
                            if *depth == 0 {
                                ui.add_space(3.0);
                            }
                        }
                    });
                if let Some(id) = to_toggle {
                    // 装卸——卸下时若正在查看该箱 ⇒ 回退（父箱首个启用子箱；平台应用 ⇒ 回平台自身）
                    self.registry.toggle(&id);
                    if !self.registry.enabled.get(&id).copied().unwrap_or(true)
                        && self.registry.active == id
                    {
                        self.registry.active = self.registry.fallback_after_disable(&id);
                        self.registry.save_state();
                    }
                    changed = true;
                }
                // M4 生态箱（外部模块——配置文件声明——第三方开发者挂船）
                if !ext_rows.is_empty() {
                    ui.add_space(8.0);
                    ui.label(t!("modules.ext_section"));
                    ui.separator();
                    let mut ext_toggle: Option<String> = None;
                    for (id, label, desc) in &ext_rows {
                        let on = self.registry.enabled.get(id).copied().unwrap_or(true);
                        let mut next = on;
                        ui.horizontal(|ui| {
                            if ui.checkbox(&mut next, label.as_str()).changed() && next != on {
                                ext_toggle = Some(id.clone());
                            }
                            ui.weak(desc.as_str());
                        });
                    }
                    if let Some(id) = ext_toggle {
                        self.registry.toggle(&id);
                        if !self.registry.enabled.get(&id).copied().unwrap_or(true)
                            && self.registry.active == id
                        {
                            self.registry.active = self.registry.fallback_after_disable(&id);
                            self.registry.save_state();
                        }
                        changed = true;
                    }
                }
                if changed {
                    self.registry.save(); // 持久化（M2-c）
                }
                ui.add_space(12.0);
                if ui.button(t!("action.close")).clicked() {
                    close = true;
                }
            });
        if close {
            self.show_module_manager = false;
        }
    }

    /// 渲染主区（v2.5.6——虫茧注册表分发——Mr2109 2026-08-29）
    /// 当前**有效**箱（父箱 ⇒ 记忆子箱 / order 最小子箱）。渲染分发、HUD 面包屑都用它。
    /// 纯查询（只读注册表）——三个父箱因此不需要各自的渲染臂（设计 §五）。
    fn effective_active(&self) -> String {
        // 2026-09-13（Mr2109纠正）：虫茧平台栅格里打开的应用（如「文档」）**直接走它自己的渲染臂**
        // ——与子页签同款「零重复实现」，且一级导航高亮仍停在「虫茧」（人还在平台里）。
        if self.registry.active == zerg_module::PLATFORM_PAGE_ID {
            if let Some(app) = &self.cocoon_app {
                return app.clone();
            }
        }
        let remembered = self
            .registry
            .remembered_child
            .get(&self.registry.active)
            .map(String::as_str)
            .unwrap_or("");
        self.registry
            .effective_module(&self.registry.active, remembered)
    }

    /// 按契约渲染一个已装载的茧（C9 第 1 步——宿主只认 `Cocoon::render` 吊点）。
    ///
    /// 懒装载：实例第一次被打开时才建（`cocoon::load`），此后缓存在 `self.cocoons`
    /// ⇒ 切走再回来仍是**同一个实例**（示例虫茧引擎后台继续——与契约化前行为一致）。
    /// 退出通道：茧在 `render` 里把「回平台栅格」写进 `ctx`，宿主在这里读并清打开态
    /// （实例保留——再打开即回到原状）。
    fn cocoon_view(&mut self, ui: &mut egui::Ui, id: &str) {
        if !self.cocoons.iter().any(|c| c.meta().id == id) {
            if let Some(app) = crate::modules::cocoon::load(id) {
                self.cocoons.push(app);
            }
        }
        if let Some(idx) = self.cocoons.iter().position(|c| c.meta().id == id) {
            // C9 第 3 步：通道**跨帧复用**（宿主能力：Ferrite 编辑器池 + 令牌）——只清「回平台」请求。
            self.cocoon_ctx.exit_requested = false;
            let ctx = &mut self.cocoon_ctx;
            self.cocoons[idx].render(ui, ctx);
            if self.cocoon_ctx.exit_requested {
                self.cocoon_app = None;
            }
        }
    }

    /// C9 第 3 步：未装载茧的「安装指引」弹窗——平台页点卡上的「安装」时置位 `cocoon_install_hint`，
    /// 这里画；用户关掉即清状态（`None` ⇒ 什么都不画，绝不空白页/报错）。
    fn render_cocoon_install_hint(&mut self, ui: &egui::Ui) {
        let Some(id) = self.cocoon_install_hint.clone() else {
            return;
        };
        if cocoon_install_hint_window(ui, &id) {
            self.cocoon_install_hint = None;
        }
    }

    fn main_view(&mut self, ui: &mut egui::Ui) {
        // 2026-09-13（设计《UI 大调动-导航精简与分组》§4.3/§五）：先解析**有效**箱——
        // active 是父箱时下钻到记忆/首个子箱 ⇒ 三个父箱**不需要独立的渲染臂**，各模块既有实现零改动。
        let eff = self.effective_active();
        // 父箱下已无启用子箱 ⇒ 内容区空态（设计 §4.2 规则 4「不留孤儿」）；二级页签行给同款提示。
        let empty_group = self
            .registry
            .find(&eff)
            .map(|m| m.is_group && self.registry.children_of(&eff).is_empty())
            .unwrap_or(false);
        if empty_group {
            ui.weak(t!("nav.no_submodules"));
            return;
        }
        // 2026-09-13（Mr2109纠正）：从虫茧平台栅格打开的**应用** → 顶部一条「← 虫茧平台」面包屑。
        // 应用自身是完整界面（不加标题，设计 §4.4），宿主只提供一层返回。
        // C9 第 1 步（2026-09-13）：茧**按契约装载**——打开的是契约茧 ⇒ 直接走 `Cocoon::render`
        // 吊点并返回（宿主不再为任何茧写渲染臂——C9 第 4 步文档界面也已迁出）。
        if self.registry.active == zerg_module::PLATFORM_PAGE_ID {
            if let Some(open) = self.cocoon_app.clone() {
                if !cocoon_openable(&open) {
                    // 未装载的茧：不停在它的视图（铭牌 loaded 驱动——不再逐茧写 cfg）
                    self.cocoon_app = None;
                } else {
                    if ui.button(format!("← {}", t!("cocoon.platform"))).clicked() {
                        self.cocoon_app = None;
                    }
                    ui.add_space(6.0);
                    if crate::modules::cocoon::meta_of(&open).is_some() {
                        self.cocoon_view(ui, &open);
                        return;
                    }
                }
            }
        }
        match eff.as_str() {
            "chat" => {
                // v2.5.7 对话模块（Mr2109——完全借鉴 Hermes——第一板块）
                self.chat_view.render(ui);
            }
            "tasks" => self.tasks_view(ui),
            "upgrade" => crate::modules::upgrade::ui(ui),
            "model-registry" => crate::modules::model_registry::ui(ui),
            // 文件浏览器（阶段 1——2026-09-13 设计「文件浏览器虫茧」§4.1/§4.3）：
            // **薄壳箱**——真正实现是内建组件 ui/src/modules/filebrowse/（宿主通用文件浏览能力）。
            // 首版：顶部根选择器 + 第一栏目录/文件列表 + 第二栏选中文件内容预览（不做内嵌编辑器）。
            "file-browser" => {
                // 2026-09-13（设计 §4.4）：删掉重复标题块（模块名/简介）——第一行即内容。
                self.fb.render(ui);
            }
            "internal-tasks" => self.internal_tasks_view(ui),
            "cluster" => {
                // 设计 §4.4：删重复标题块。
                if let Some(r) = lock_recover(&self.cluster).clone() {
                    egui::ScrollArea::vertical().show(ui, |ui| {
                        // 总览
                        let healthy = r.get("healthy_count").and_then(|h| h.as_u64()).unwrap_or(0);
                        let total = r
                            .get("total_machines")
                            .and_then(|h| h.as_u64())
                            .unwrap_or(0);
                        let active = r
                            .get("total_active_requests")
                            .and_then(|h| h.as_u64())
                            .unwrap_or(0);
                        ui.label(t!(
                            "cluster.health_summary",
                            healthy = healthy,
                            total = total,
                            active = active
                        ));
                        ui.separator();
                        // 机器列表（dict——local/mini1/x3）
                        if let Some(machines) = r.get("machines").and_then(|m| m.as_object()) {
                            for (name, mv) in machines {
                                let model = mv.get("model").and_then(|n| n.as_str()).unwrap_or("—");
                                let mem_avail = mv
                                    .get("mem_available_gb")
                                    .and_then(|n| n.as_f64())
                                    .unwrap_or(0.0);
                                let mem_total = mv
                                    .get("mem_total_gb")
                                    .and_then(|n| n.as_f64())
                                    .unwrap_or(0.0);
                                let load = mv.get("load").and_then(|n| n.as_f64()).unwrap_or(0.0);
                                let gpu = mv
                                    .get("gpu_used_gb")
                                    .and_then(|n| n.as_f64())
                                    .unwrap_or(0.0);
                                let healthy_m =
                                    mv.get("healthy").and_then(|n| n.as_bool()).unwrap_or(true);
                                let color = if healthy_m {
                                    egui::Color32::from_rgb(80, 200, 120)
                                } else {
                                    egui::Color32::from_rgb(220, 80, 80)
                                };
                                ui.label(egui::RichText::new(format!("📊 {}", name)).color(color));
                                ui.indent(name, |ui| {
                                    ui.label(t!("cluster.machine_model", model = model));
                                    ui.label(t!(
                                        "cluster.machine_mem",
                                        avail = format!("{:.0}", mem_avail),
                                        total = format!("{:.0}", mem_total)
                                    ));
                                    ui.label(t!(
                                        "cluster.machine_load",
                                        load = format!("{:.2}", load),
                                        gpu = format!("{:.1}", gpu)
                                    ));
                                });
                                ui.separator();
                            }
                        }
                    });
                } else {
                    ui.spinner();
                    ui.weak(t!("common.loading"));
                }
            }
            "git" => {
                // 设计 §4.4：删重复标题块。
                if let Some(g) = lock_recover(&self.git_status).clone() {
                    // 分支（通俗化——task-xxx → 任务类型名）
                    egui::CollapsingHeader::new(format!(
                        "🌿 {}（{}）",
                        t!("git.branches"),
                        g.branches.as_ref().map(|b| b.len()).unwrap_or(0)
                    ))
                    .id_salt("git_branches") // APP-A08: 稳定 id
                    .show(ui, |ui| {
                        if let Some(branches) = &g.branches {
                            for b in branches {
                                ui.label(format!("  {}", humanize_branch(b)));
                            }
                        }
                    });
                    // worktree（通俗化——只显示分支名——不显示完整路径）
                    egui::CollapsingHeader::new(format!(
                        "📂 {}（{}）",
                        t!("git.worktrees"),
                        g.worktrees.as_ref().map(|w| w.len()).unwrap_or(0)
                    ))
                    .id_salt("git_worktrees") // APP-A08: 稳定 id
                    .show(ui, |ui| {
                        if let Some(wts) = &g.worktrees {
                            for wt in wts {
                                let branch =
                                    wt.get("branch").and_then(|b| b.as_str()).unwrap_or("?");
                                ui.label(format!("  🔀 {}", humanize_branch(branch)));
                            }
                        }
                    });
                    // 未 merge（待脑检查）
                    egui::CollapsingHeader::new(format!(
                        "{} {}（{}）",
                        icon_text("warning"),
                        t!("git.unmerged"),
                        g.unmerged.as_ref().map(|u| u.len()).unwrap_or(0)
                    ))
                    .id_salt("git_unmerged") // APP-A08: 稳定 id
                    .show(ui, |ui| {
                        if let Some(un) = &g.unmerged {
                            for u in un {
                                ui.label(format!("  ⏳ {}", humanize_branch(u)));
                            }
                        }
                        if g.unmerged.as_ref().map(|u| u.is_empty()).unwrap_or(true) {
                            ui.weak(format!("  {}", t!("common.none")));
                        }
                    });
                } else {
                    ui.spinner();
                    ui.weak(t!("common.loading"));
                }
            }
            "logs" => {
                // 设计 §4.4：删重复标题块。
                if let Some(lines) = lock_recover(&self.logs).clone() {
                    egui::ScrollArea::vertical()
                        .auto_shrink([false, false])
                        .show(ui, |ui| {
                            for l in lines {
                                let ll = l.to_ascii_lowercase(); // 日志级别判定：大小写不敏感 + 中英双认（去掉对单一语言的依赖）
                                let color = if ll.contains("error") || l.contains("错误") {
                                    egui::Color32::from_rgb(220, 80, 80)
                                } else if ll.contains("warn") || l.contains("警告") {
                                    egui::Color32::from_rgb(240, 200, 80)
                                } else {
                                    egui::Color32::from_rgb(200, 200, 200)
                                };
                                ui.label(egui::RichText::new(l.as_str()).color(color));
                            }
                        });
                } else {
                    ui.spinner();
                    ui.weak(t!("common.loading"));
                }
            }
            // 🐛 虫茧=平台（Mr2109 2026-09-03：平台界面呈现无数应用——示例虫茧只是其一）
            // rt_active=false → 平台启动器（应用栅格）；true → 示例虫茧全屏（引擎后台继续 M2）
            "roundtable" => {
                // 注：未装载的茧在 `main_view` 开头就被清掉打开态（铭牌 `loaded` 驱动——
                // 不再逐茧写 `#[cfg(not(feature))]` 分支，C9 第 1 步）。
                if self.cocoon_app.is_none() {
                    // C9 第 3 步：未装载茧的「安装指引」弹窗（点卡上的「安装」触发）——先画，浮在栅格上。
                    self.render_cocoon_install_hint(ui);
                    // ── 平台界面：应用栅格（无数茧——每个=独立虫茧应用——示例虫茧=第一个）──
                    ui.heading(format!("{} {}", icon_text("boxes"), t!("cocoon.platform")));
                    ui.weak(t!("cocoon.platform_hint"));
                    ui.add_space(10.0);
                    // 应用清单：**契约注册表驱动**（C9 第 1 步）——每卡的 id/图标/名字/简介
                    // 全部来自茧自己的铭牌（`cocoon::catalog()`），宿主不再为某个茧写死。
                    // 2026-09-13（Mr2109纠正）：栅格里**每个茧 = 一张独立应用卡**。「文档」是内置应用，
                    // 但在这里与示例虫茧平级——点进去是完整界面（顶部一条「← 虫茧平台」面包屑返回）。
                    // 卡片清单由 platform_apps 给出 ⇒ 卸下的卡消失；未装载的茧照常出卡（铭牌在册）。
                    let cards: Vec<(String, String, String, String)> = self
                        .registry
                        .platform_apps()
                        .into_iter()
                        .filter_map(|id| {
                            // ① 契约茧：铭牌即数据源（名字/简介走 i18n 键——仍是双语文案）
                            // C9 第 4 步：宿主**不再**为任何应用写死卡片（文档也是茧了 ⇒ 走本分支）
                            if let Some(m) = crate::modules::cocoon::meta_of(id) {
                                let name = if m.name_key.is_empty() {
                                    m.name.to_string()
                                } else {
                                    t!(m.name_key).to_string()
                                };
                                let desc = if m.desc_key.is_empty() {
                                    String::new()
                                } else {
                                    t!(m.desc_key).to_string()
                                };
                                return Some((id.to_string(), m.icon.to_string(), name, desc));
                            }
                            // ② 非茧：不是契约注册表里的应用 ⇒ 不出卡（宿主零硬编码）
                            None
                        })
                        .collect();
                    // 卡片网格（wrap 布局——每卡固定宽 260）
                    egui::ScrollArea::vertical()
                        .auto_shrink([false, false])
                        .show(ui, |ui| {
                            egui::Grid::new("cocoon_grid")
                                .num_columns(3)
                                .spacing([14.0, 14.0])
                                .show(ui, |ui| {
                                    for (id, glyph, name, desc) in &cards {
                                        let (rect, _) = ui.allocate_exact_size(
                                            egui::vec2(260.0, 132.0),
                                            egui::Sense::click(),
                                        );
                                        let hover = rect.contains(
                                            ui.ctx().pointer_interact_pos().unwrap_or_default(),
                                        );
                                        let (fill, stroke) = if hover {
                                            (
                                                egui::Color32::from_rgb(42, 46, 56),
                                                egui::Stroke::new(
                                                    1.0,
                                                    egui::Color32::from_rgb(90, 110, 170),
                                                ),
                                            )
                                        } else {
                                            (
                                                egui::Color32::from_rgb(32, 35, 43),
                                                egui::Stroke::new(
                                                    1.0,
                                                    egui::Color32::from_rgb(55, 60, 70),
                                                ),
                                            )
                                        };
                                        ui.painter().rect_filled(rect, 10.0, fill);
                                        ui.painter().rect_stroke(
                                            rect,
                                            10.0,
                                            stroke,
                                            egui::StrokeKind::Inside,
                                        );
                                        let mut card_ui = ui.new_child(
                                            egui::UiBuilder::new()
                                                .max_rect(egui::Rect::from_min_max(
                                                    rect.min + egui::vec2(14.0, 12.0),
                                                    rect.max - egui::vec2(14.0, 12.0),
                                                ))
                                                .layout(egui::Layout::top_down(egui::Align::Min)),
                                        );
                                        card_ui.label(
                                            egui::RichText::new(format!("{glyph} {name}"))
                                                .size(16.0)
                                                .strong(),
                                        );
                                        card_ui.add_space(6.0);
                                        card_ui.label(
                                            egui::RichText::new(desc.as_str())
                                                .size(12.0)
                                                .color(egui::Color32::from_rgb(170, 175, 185)),
                                        );
                                        card_ui.with_layout(
                                            egui::Layout::right_to_left(egui::Align::BOTTOM),
                                            |ui| {
                                                if cocoon_openable(id) {
                                                    // 能不能打开 = 铭牌驱动（C9 第 1 步）：契约茧看 `loaded`
                                                    // （未装载 ⇒ 卡片照常但打不开）。
                                                    if ui
                                                        .button(
                                                            egui::RichText::new(t!("action.open"))
                                                                .size(12.0),
                                                        )
                                                        .clicked()
                                                    {
                                                        self.cocoon_app = Some(id.clone());
                                                    }
                                                } else if crate::modules::cocoon::install_guide(id)
                                                    .is_some()
                                                {
                                                    // C9 第 3 步：**未安装 ⇒ 提示安装**（设计 §4.3 / Q1）——按钮指向
                                                    // 它的独立仓；点击弹安装指引（不报错、不空白，也绝不假装可用）。
                                                    // 文案先取出到局部量：源码级守线（`platform_wires_the_install_affordance`）
                                                    // 按字面找 `t!` 宏里的 `cocoon.install` 键，而整条 if 超 100 列时
                                                    // rustfmt 会把宏参数拆行、把这条子串断开 ⇒ 两者只能二者之一绿。
                                                    let install_label = t!("cocoon.install");
                                                    if ui
                                                        .button(
                                                            egui::RichText::new(install_label)
                                                                .size(12.0),
                                                        )
                                                        .clicked()
                                                    {
                                                        self.cocoon_install_hint = Some(id.clone());
                                                    }
                                                }
                                            },
                                        );
                                        if !cocoon_openable(id) {
                                            // 未装载：明示「未装载」（绝不静默失败——设计 §4.3）
                                            card_ui.label(
                                                egui::RichText::new(t!("cocoon.not_loaded"))
                                                    .size(11.0)
                                                    .weak(),
                                            );
                                        }
                                        if card_ui.rect_contains_pointer(rect)
                                            && ui.ctx().input(|i| i.pointer.any_click())
                                            && cocoon_openable(id)
                                        {
                                            self.cocoon_app = Some(id.clone());
                                        }
                                        ui.allocate_exact_size(
                                            egui::vec2(0.0, 0.0),
                                            egui::Sense::hover(),
                                        );
                                        ui.end_row();
                                    }
                                });
                        });
                }
            }
            "models" => {
                // 模型库独立板块（Mr2109 2026-08-27——排资源库上面）
                if self.res_type != "models" {
                    self.res_type = "models".to_string();
                    let store = self.resources.clone();
                    api::runtime().spawn(async move {
                        let r = api::fetch_resources_blocking("models".to_string())
                            .await
                            .ok();
                        *lock_recover(&store) = r;
                    });
                }
                ui.heading(format!(
                    "{} {}",
                    icon_text("computer-tower"),
                    t!("page.model_library")
                ));
                ui.add_space(4.0);
                ui.weak(t!("models.group_hint"));
                ui.add_space(4.0);
                self.models_view(ui);
            }
            "resources" => {
                // 兜底: 从模型库切过来 res_type 残留 models——强制回 tools（Mr2109 2026-08-27 修复）
                if self.res_type == "models" {
                    self.res_type = "tools".to_string();
                    let store = self.resources.clone();
                    api::runtime().spawn(async move {
                        let r = api::fetch_resources_blocking("tools".to_string())
                            .await
                            .ok();
                        *lock_recover(&store) = r;
                    });
                }
                // 设计 §4.4：删重复标题块（其下的「3 库切换」是操作控件——保留）。
                // 3 库切换（模型库已独立板块——Mr2109 2026-08-27）
                ui.horizontal(|ui| {
                    let types = [
                        (t!("resources.tools").to_string(), "tools"),
                        (t!("resources.skills").to_string(), "skills"),
                        (t!("resources.mcp").to_string(), "mcp"),
                    ];
                    for (label, t) in types {
                        if ui.selectable_label(self.res_type == t, label).clicked() {
                            self.res_type = t.to_string();
                            // 拉对应类型
                            let store = self.resources.clone();
                            let rt = t.to_string();
                            api::runtime().spawn(async move {
                                let r = api::fetch_resources_blocking(rt).await.ok();
                                *lock_recover(&store) = r;
                            });
                        }
                    }
                });
                ui.separator();
                // 左列表 + 右简介（模型点击显示——Mr2109 2026-08-20）
                ui.columns(2, |cols| {
                    // 左列——列表
                    if let Some(r) = lock_recover(&self.resources).clone() {
                        if let Some(items) = r.get("items").and_then(|i| i.as_array()) {
                            egui::ScrollArea::vertical().id_salt("res_list").show(
                                &mut cols[0],
                                |ui| {
                                    // 模型库——按设备分组 + 字母排序（Mr2109 2026-08-20）
                                    if self.res_type == "models" {
                                        let mut groups: std::collections::BTreeMap<
                                            String,
                                            Vec<String>,
                                        > = std::collections::BTreeMap::new();
                                        let mut descs: std::collections::HashMap<String, String> =
                                            std::collections::HashMap::new();
                                        for item in items {
                                            let name = item
                                                .get("name")
                                                .and_then(|n| n.as_str())
                                                .unwrap_or("?")
                                                .to_string();
                                            let machine = item
                                                .get("machine")
                                                .and_then(|m| m.as_str())
                                                .unwrap_or("unknown")
                                                .to_string();
                                            let desc = item
                                                .get("description")
                                                .and_then(|d| d.as_str())
                                                .unwrap_or("")
                                                .to_string();
                                            groups
                                                .entry(machine.clone())
                                                .or_default()
                                                .push(name.clone());
                                            descs.insert(name, desc);
                                        }
                                        for (machine, names) in &groups {
                                            let icon = match machine.as_str() {
                                                "x3" => "📊 X3".to_string(),
                                                "local" => {
                                                    t!("resources.machine_local").to_string()
                                                }
                                                "mini1" => "🍎 mini1".to_string(),
                                                "mini2" => "🍎 mini2".to_string(),
                                                _ => t!("resources.machine_unknown").to_string(),
                                            };
                                            egui::CollapsingHeader::new(t!(
                                                "models.header_count",
                                                icon = icon,
                                                count = names.len()
                                            ))
                                            .id_salt(format!("res_models_{}", machine)) // APP-A08: 稳定 id
                                            // APP-A21（2026-09-10 审计）: 默认展开——原来默认为折叠，
                                            // 进「资源库」只看到设备分组标题、看不到任何模型，容易以为没模型
                                            .default_open(true)
                                            .show(
                                                ui,
                                                |ui| {
                                                    // 设备内按字母排序（固定——刷新不变——Mr2109）
                                                    let mut sorted = names.clone();
                                                    sorted.sort();
                                                    for n in &sorted {
                                                        let selected =
                                                            self.selected_model.as_deref()
                                                                == Some(n.as_str());
                                                        let trust = descs
                                                            .get(n)
                                                            .cloned()
                                                            .unwrap_or_default();
                                                        // 资源信任度标记（🆕 新模型——2026-08-21 Mr2109）
                                                        let trust_mark = if trust.contains("🆕") {
                                                            " 🆕"
                                                        } else {
                                                            ""
                                                        };
                                                        // 模型名前面不加图标（Mr2109 2026-08-31——干净列表）
                                                        if ui
                                                            .selectable_label(
                                                                selected,
                                                                format!("  {}{}", n, trust_mark),
                                                            )
                                                            .clicked()
                                                        {
                                                            self.selected_model = Some(n.clone());
                                                            self.selected_model_desc = descs
                                                                .get(n)
                                                                .cloned()
                                                                .unwrap_or_default();
                                                        }
                                                    }
                                                },
                                            );
                                        }
                                    } else {
                                        // 工具/skill/mcp——表格（Mr2109 2026-08-21: 表格形式——状态/次数/故障/时间）
                                        let mut rows: Vec<(
                                            String,
                                            String,
                                            String,
                                            i64,
                                            i64,
                                            String,
                                            String,
                                        )> = items
                                            .iter()
                                            .filter_map(|item| {
                                                let name = item
                                                    .get("name")
                                                    .and_then(|n| n.as_str())?
                                                    .to_string();
                                                let version = item
                                                    .get("version")
                                                    .and_then(|v| v.as_str())
                                                    .unwrap_or("v1.0.0")
                                                    .to_string();
                                                let trust = item
                                                    .get("trust")
                                                    .and_then(|t| t.as_str())
                                                    .unwrap_or("未知")
                                                    .to_string();
                                                let uses = item
                                                    .get("uses")
                                                    .and_then(|u| u.as_i64())
                                                    .unwrap_or(0);
                                                let faults = item
                                                    .get("faults")
                                                    .and_then(|f| f.as_i64())
                                                    .unwrap_or(0);
                                                let since = item
                                                    .get("since")
                                                    .and_then(|s| s.as_str())
                                                    .unwrap_or("-")
                                                    .to_string();
                                                let desc = item
                                                    .get("desc")
                                                    .and_then(|d| d.as_str())
                                                    .unwrap_or("-")
                                                    .to_string();
                                                Some((
                                                    name, version, trust, uses, faults, since, desc,
                                                ))
                                            })
                                            .collect();
                                        rows.sort_by(|a, b| {
                                            let ord = match self.res_sort_col {
                                                1 => a.2.cmp(&b.2), // 状态
                                                2 => a.3.cmp(&b.3), // 次数
                                                3 => a.4.cmp(&b.4), // 故障
                                                4 => a.5.cmp(&b.5), // 时间
                                                5 => a.6.cmp(&b.6), // 简介
                                                _ => a.0.cmp(&b.0), // 名称
                                            };
                                            if self.res_sort_desc {
                                                ord.reverse()
                                            } else {
                                                ord
                                            }
                                        });
                                        egui::Grid::new("res_table").striped(true).show(ui, |ui| {
                                            // 列头（点击排序——Mr2109 2026-08-21）
                                            let headers = [
                                                (t!("resources.col_name").to_string(), 0),
                                                (t!("resources.col_version").to_string(), 1),
                                                (t!("resources.col_state").to_string(), 2),
                                                (t!("resources.col_calls").to_string(), 3),
                                                (t!("resources.col_faults").to_string(), 4),
                                                (t!("resources.col_added").to_string(), 5),
                                                (t!("resources.col_desc").to_string(), 6),
                                            ];
                                            for (label, col) in headers {
                                                let arrow = if self.res_sort_col == col {
                                                    if self.res_sort_desc {
                                                        " ▼"
                                                    } else {
                                                        " ▲"
                                                    }
                                                } else {
                                                    ""
                                                };
                                                if ui
                                                    .button(format!("{}{}", label, arrow))
                                                    .clicked()
                                                {
                                                    if self.res_sort_col == col {
                                                        self.res_sort_desc = !self.res_sort_desc;
                                                    } else {
                                                        self.res_sort_col = col;
                                                        self.res_sort_desc = false;
                                                    }
                                                }
                                            }
                                            ui.end_row();
                                            for (name, version, trust, uses, faults, since, desc) in
                                                &rows
                                            {
                                                // P4-49 工具名后不加图标（Mr2109 UI 偏好——与模型名一致）
                                                ui.label(name.clone());
                                                ui.label(version.clone());
                                                ui.label(if trust == "新" {
                                                    t!("resources.trust_new").to_string()
                                                } else if trust == "正式" {
                                                    t!("resources.trust_official").to_string()
                                                } else if trust == "未知" || trust.is_empty() {
                                                    t!("common.unknown").to_string()
                                                } else {
                                                    trust.clone()
                                                });
                                                ui.label(t!("resources.uses_count", n = uses));
                                                let fault_color = if *faults > 0 {
                                                    egui::Color32::from_rgb(255, 80, 80)
                                                } else {
                                                    egui::Color32::GRAY
                                                };
                                                ui.colored_label(
                                                    fault_color,
                                                    format!("{}", faults),
                                                );
                                                ui.label(since.clone());
                                                ui.label(desc.clone());
                                                ui.end_row();
                                            }
                                        });
                                    }
                                },
                            );
                        } else {
                            cols[0].weak(t!("common.none"));
                        }
                    } else {
                        cols[0].spinner();
                        cols[0].weak(t!("common.loading"));
                    }
                    // 右列——模型简介
                    if self.res_type == "models" {
                        cols[1].heading(t!("models.desc_title"));
                        if let Some(m) = &self.selected_model {
                            cols[1].label(t!("models.model_label", model = m));
                            cols[1].add_space(4.0);
                            if !self.selected_model_desc.is_empty() {
                                cols[1].label(&self.selected_model_desc);
                            } else {
                                cols[1].weak(t!("models.no_desc"));
                            }
                        } else {
                            cols[1].weak(t!("models.click_hint"));
                        }
                    } else {
                        cols[1].weak(t!("resources.lib_hint"));
                    }
                });
            }
            _ => {
                // M4 生态箱（外部模块——配置文件声明——第三方开发者挂船）
                let active_id = eff.clone();
                let ext = self
                    .registry
                    .external
                    .iter()
                    .find(|m| m.id == active_id)
                    .cloned();
                if let Some(m) = ext {
                    ui.add_space(12.0);
                    ui.heading(format!("{} {}", m.icon, m.name));
                    ui.weak(&m.description);
                    ui.label(t!("common.version_label", version = m.version));
                    ui.separator();
                    ui.add_space(8.0);
                    egui::Frame::new()
                        .fill(ui.visuals().faint_bg_color)
                        .corner_radius(8.0)
                        .inner_margin(egui::Margin::same(16))
                        .show(ui, |ui| {
                            ui.strong(t!("ext.title"));
                            ui.add_space(4.0);
                            ui.label(t!("ext.hint"));
                            ui.label(t!("ext.roadmap"));
                            ui.add_space(8.0);
                            if !m.url.is_empty() && ui.button(t!("ext.open")).clicked() {
                                // APP-A18（2026-09-10 审计）: ① URL 来自外部模块配置文件
                                // （M4 生态箱——第三方开发者挂船），先校验 scheme（仅 http/https），
                                // 拼错或 file://、javascript: 之类一律拒绝并红字提示；
                                // ② spawn 出的 Child 必须回收——原来 Result/Child 一起丢弃，
                                // `open` 退出后留下僵尸进程直到 UI 退出。这里在后台线程 wait()
                                // （不阻塞 UI 线程）。
                                let url = m.url.clone();
                                if url.starts_with("http://") || url.starts_with("https://") {
                                    match std::process::Command::new("open").arg(&url).spawn() {
                                        Ok(mut child) => {
                                            std::thread::spawn(move || {
                                                let _ = child.wait(); // 回收子进程（防僵尸）
                                            });
                                        }
                                        Err(e) => {
                                            *lock_recover(&self.poll_err) =
                                                Some(t!("ext.open_failed", err = e).to_string());
                                        }
                                    }
                                } else {
                                    *lock_recover(&self.poll_err) =
                                        Some(t!("ext.bad_url", url = url).to_string());
                                }
                            }
                        });
                } else {
                    // 未知/未启用模块——空舱（M2 模块管理后此处显示占位）
                    ui.weak(t!("ext.missing"));
                }
            }
        }
    }

    /// 模型库视图（独立板块——Mr2109 2026-08-27——排资源库上面）
    fn models_view(&mut self, ui: &mut egui::Ui) {
        let total_w = ui.available_width();
        let avail_h = ui.available_height().max(200.0);
        let left_w = (total_w * self.split_model).clamp(220.0, total_w - 340.0);
        let mut dragging = false;
        // 全高区域分配（Mr2109 2026-08-27——窗口宽高变化内部自动跟随）
        ui.allocate_ui_with_layout(
            egui::vec2(total_w, avail_h),
            egui::Layout::left_to_right(egui::Align::TOP),
            |ui| {
            let (lrect, _) = ui.allocate_exact_size(egui::vec2(left_w, avail_h), egui::Sense::hover());
            let mut l_ui = ui.new_child(egui::UiBuilder::new().max_rect(lrect).layout(egui::Layout::top_down(egui::Align::Min)));
            {
                // 左列内容
            // 左列——按设备分组 + 字母排序
            if let Some(r) = lock_recover(&self.resources).clone() {
                if let Some(items) = r.get("items").and_then(|i| i.as_array()) {
                    egui::ScrollArea::vertical().id_salt("models_list").auto_shrink(false).show(&mut l_ui, |ui| {
                        let mut groups: std::collections::BTreeMap<String, Vec<String>> =
                            std::collections::BTreeMap::new();
                        let mut descs: std::collections::HashMap<String, String> =
                            std::collections::HashMap::new();
                        for item in items {
                            let name = item.get("name").and_then(|n| n.as_str()).unwrap_or("?").to_string();
                            let machine = item.get("machine").and_then(|m| m.as_str()).unwrap_or("unknown").to_string();
                            let desc = item.get("description").and_then(|d| d.as_str()).unwrap_or("").to_string();
                            groups.entry(machine.clone()).or_default().push(name.clone());
                            descs.insert(name, desc);
                        }
                        for (machine, names) in &groups {
                            let icon = match machine.as_str() {
                                "x3" => "📊 X3".to_string(),
                                "local" => t!("resources.machine_local").to_string(),
                                "mini1" => "🍎 mini1".to_string(),
                                "mini2" => "🍎 mini2".to_string(),
                                _ => t!("resources.machine_unknown").to_string(),
                            };
                            egui::CollapsingHeader::new(t!("models.header_count", icon = icon, count = names.len()))
                                .id_salt(format!("models_view_{}", machine)) // APP-A08: 稳定 id
                                // APP-A21（2026-09-10 审计）: 默认展开（同资源库——否则进「模型库」左栏是空的）
                                .default_open(true)
                                .show(ui, |ui| {
                                let mut sorted = names.clone();
                                sorted.sort();
                                for n in &sorted {
                                    let selected = self.selected_model.as_deref() == Some(n.as_str());
                                    let trust = descs.get(n).cloned().unwrap_or_default();
                                    let trust_mark = if trust.contains("🆕") { " 🆕" } else { "" };
                                    if ui.selectable_label(selected, format!("  ⬤ {}{}", n, trust_mark)).clicked() {
                                        self.selected_model = Some(n.clone());
                                        self.selected_model_desc = descs.get(n).cloned().unwrap_or_default();
                                        // v2.5.6 拉模型详情（适配器选项+加载状态）
                                        let name2 = n.clone();
                                        let store = self.model_detail.clone();
                                        let sstore = self.adapter_schema.clone();
                                        let edstore = self.adapter_edit.clone();
                                        api::runtime().spawn(async move {
                                            let r = api::fetch_model_detail_blocking(&name2).await.ok();
                                            *lock_recover(&store) = r;
                                            // 拉适配器 schema（编辑控件渲染——各模型各自参数集）
                                            let s = api::fetch_adapter_schema_blocking(&name2).await.ok();
                                            if let Some(ref sv) = s {
                                                if let Some(arr) = sv.get("schema").and_then(|v| v.as_array()) {
                                                    let mut init = std::collections::HashMap::new();
                                                    for o in arr {
                                                        if let (Some(k), Some(v)) = (o.get("key").and_then(|x| x.as_str()), o.get("value")) {
                                                            init.insert(k.to_string(), v.clone());
                                                        }
                                                    }
                                                    lock_recover(&edstore).insert(name2.clone(), init);
                                                }
                                            }
                                            *lock_recover(&sstore) = s;
                                        });
                                        self.adapter_msg.clear();
                                    }
                                }
                            });
                        }
                    });
                } else {
                    l_ui.weak(t!("common.none"));
                }
            } else {
                l_ui.spinner();
                l_ui.weak(t!("common.loading"));
            }
            } // 左列块结束
                // 拖拽条（8px 分隔线——鼠标移到显示 ⇔ 光标——可拖）
                let (drect, dresp) = ui.allocate_exact_size(egui::vec2(8.0, avail_h), egui::Sense::drag());
                ui.painter().rect_filled(drect, 0.0, ui.visuals().faint_bg_color);
                ui.painter().vline(drect.center().x, drect.y_range(), egui::Stroke::new(1.0, ui.visuals().weak_text_color()));
                let _ = dresp.clone().on_hover_cursor(egui::CursorIcon::ResizeHorizontal);
                if dresp.dragged() {
                    let dx = ui.input(|i| i.pointer.delta().x);
                    self.split_model = (self.split_model + dx / total_w).clamp(0.2, 0.6);
                }
                // APP-A12: 松手才落盘（原来每帧同步读写 JSON）
                if dresp.drag_stopped() {
                    dragging = true;
                }
                // 右列
                let right_w = (ui.available_width() - 8.0).max(280.0);
                let (rrect, _) = ui.allocate_exact_size(egui::vec2(right_w, avail_h), egui::Sense::hover());
                let mut r_ui = ui.new_child(egui::UiBuilder::new().max_rect(rrect).layout(egui::Layout::top_down(egui::Align::Min)));
                {
                    // 右列内容原体（整体滚动——2026-08-27 修复面板乱: 内容超长被 clip 裁掉不可滚）
                    egui::ScrollArea::vertical().id_salt("models_right_scroll").auto_shrink(false).show(&mut r_ui, |r_ui| {
            // 右列——模型简介 + 适配器选项 + 启动/停止（Mr2109 2026-08-27）
            r_ui.heading(t!("models.detail_title"));
            if let Some(m) = &self.selected_model {
                r_ui.label(t!("models.model_label", model = m));
                r_ui.add_space(4.0);
                if !self.selected_model_desc.is_empty() {
                    r_ui.label(&self.selected_model_desc);
                } else {
                    r_ui.weak(t!("models.no_desc"));
                }
                r_ui.add_space(8.0);
                r_ui.separator();
                // 适配器选项 + 启动状态（从 /api/models/{name} 拉）
                let detail = lock_recover(&self.model_detail).clone();
                match detail {
                    Some(d) => {
                        let loaded = d.get("loaded").and_then(|v| v.as_bool()).unwrap_or(false);
                        let can_start = d.get("can_start").and_then(|v| v.as_bool()).unwrap_or(false);
                        let status = d.get("status").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                        // 启动状态 + 开关
                        r_ui.horizontal(|ui| {
                            if loaded {
                                ui.colored_label(egui::Color32::from_rgb(80, 200, 120), t!("models.local_badge", status = status));
                                if ui.button(t!("models.stop")).clicked() {
                                    // APP-A11: 结果不再 let _ = 丢弃——失败红字提示
                                    let name2 = m.clone();
                                    let store = self.model_detail.clone();
                                    let perr = self.poll_err.clone();
                                    api::runtime().spawn(async move {
                                        if let Err(e) = api::model_stop_blocking(&name2).await {
                                            *lock_recover(&perr) = Some(t!("models.stop_failed", name = name2, err = e).to_string());
                                        }
                                        if let Ok(v) = api::fetch_model_detail_blocking(&name2).await {
                                            *lock_recover(&store) = Some(v);
                                        }
                                    });
                                }
                            } else {
                                ui.colored_label(egui::Color32::from_rgb(200, 140, 80), format!("○ {}", status));
                                if can_start {
                                    if ui.button(t!("models.start")).clicked() {
                                        // APP-A11: 失败红字提示
                                        let name2 = m.clone();
                                        let store = self.model_detail.clone();
                                        let perr = self.poll_err.clone();
                                        api::runtime().spawn(async move {
                                            if let Err(e) = api::model_start_blocking(&name2).await {
                                                *lock_recover(&perr) = Some(t!("models.start_failed", name = name2, err = e).to_string());
                                            }
                                            if let Ok(v) = api::fetch_model_detail_blocking(&name2).await {
                                                *lock_recover(&store) = Some(v);
                                            }
                                        });
                                    }
                                } else {
                                    ui.weak(t!("models.remote_hint"));
                                }
                            }
                        });
                        r_ui.add_space(6.0);
                        // 标签页切换（Mr2109 2026-08-27——模型详情/适配器选项分开）
                        r_ui.horizontal(|ui| {
                            if ui.selectable_label(self.model_tab == "detail", t!("models.tab_detail")).clicked() {
                                self.model_tab = "detail".to_string();
                            }
                            if ui.selectable_label(self.model_tab == "adapter", t!("models.tab_adapter")).clicked() {
                                self.model_tab = "adapter".to_string();
                            }
                        });
                        r_ui.separator();
                        if self.model_tab == "detail" {
                        // 模型详情（部署配置——模型所有参数）
                        egui::Grid::new("model_opts").striped(true).show(r_ui, |ui| {
                            let opt = |ui: &mut egui::Ui, k: &str, v: String| {
                                ui.label(k);
                                ui.label(v);
                                ui.end_row();
                            };
                            opt(ui, t!("models.opt_device").as_ref(), d.get("host").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, t!("models.opt_backend").as_ref(), d.get("backend").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, t!("models.opt_file").as_ref(), d.get("file").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, t!("models.opt_mem").as_ref(), format!("{}", d.get("mem_gb").and_then(|v| v.as_f64()).unwrap_or(0.0)));
                            opt(ui, t!("models.opt_ssd").as_ref(), if d.get("ssd").and_then(|v| v.as_bool()).unwrap_or(false) { t!("common.yes").to_string() } else { t!("common.no").to_string() });
                            opt(ui, t!("models.opt_ctx").as_ref(), format!("{}", d.get("ctx_window").and_then(|v| v.as_i64()).unwrap_or(0)));
                            opt(ui, t!("models.opt_arch").as_ref(), d.get("arch").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, t!("models.opt_thinking").as_ref(), if d.get("thinking").and_then(|v| v.as_bool()).unwrap_or(false) { t!("models.thinking_default_on").to_string() } else { "-".to_string() });
                            opt(ui, t!("models.opt_moe").as_ref(), d.get("moe").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, t!("models.opt_template").as_ref(), d.get("template").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, t!("models.opt_modality").as_ref(), d.get("modality").and_then(|v| v.as_str()).unwrap_or("text").to_string());
                            if let Some(mp) = d.get("mmproj").and_then(|v| v.as_str()) {
                                if !mp.is_empty() {
                                    opt(ui, t!("models.opt_mmproj").as_ref(), mp.to_string());
                                }
                            }
                            opt(ui, t!("models.opt_tools").as_ref(), match d.get("tool_support") {
                                Some(v) if v.as_bool() == Some(true) => t!("common.yes").to_string(),
                                Some(v) if v.as_bool() == Some(false) => t!("common.no").to_string(),
                                _ => t!("common.unknown").to_string(),
                            });
                            opt(ui, t!("models.opt_added").as_ref(), d.get("added").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, t!("models.opt_verified").as_ref(), if d.get("verified").and_then(|v| v.as_bool()).unwrap_or(false) { "✅" } else { "-" }.to_string());
                        });
                        // 启动参数（cmd/env）
                        if let Some(cmd) = d.get("cmd").and_then(|v| v.as_array()) {
                            if !cmd.is_empty() {
                                r_ui.add_space(4.0);
                                r_ui.label(t!("models.launch_args", args = cmd.iter().filter_map(|c| c.as_str()).collect::<Vec<_>>().join(" ")));
                            }
                        }
                        if let Some(env) = d.get("env").and_then(|v| v.as_array()) {
                            if !env.is_empty() {
                                r_ui.label(t!("models.env_vars", env = env.iter().filter_map(|c| c.as_str()).collect::<Vec<_>>().join(" ")));
                            }
                        }
                        } else {
                        // 适配器选项（真正的适配器配置——推理参数——可编辑——实时生效——Mr2109 2026-08-27）
                        let schema = lock_recover(&self.adapter_schema).clone();
                        match schema {
                            Some(sv) => {
                                if let Some(arr) = sv.get("schema").and_then(|v| v.as_array()) {
                                    if arr.is_empty() {
                                        r_ui.weak(t!("adapter.none"));
                                    } else {
                                        // 编辑缓冲（懒初始化）
                                        {
                                            let mut ed = lock_recover(&self.adapter_edit);
                                            if !ed.contains_key(m.as_str()) {
                                                let mut init = std::collections::HashMap::new();
                                                for o in arr {
                                                    if let (Some(k), Some(v)) = (o.get("key").and_then(|x| x.as_str()), o.get("value")) {
                                                        init.insert(k.to_string(), v.clone());
                                                    }
                                                }
                                                ed.insert(m.clone(), init);
                                            }
                                        }
                                        let cur = lock_recover(&self.adapter_edit).get(m.as_str()).cloned().unwrap_or_default();
                                        let mut changed: Option<(String, serde_json::Value)> = None;
                                        // 垂直卡片流（Mr2109 2026-08-27——参数名+控件一行——描述弱字换行下一行——分隔线——不拥挤）
                                        // APP-A22（2026-09-10 审计）: 原来这里再套一个纵向 ScrollArea
                                        // （adapter_edit_scroll，max_height = available_height()-70 每帧重算），
                                        // 而外层 models_right_scroll 已经是纵向滚动 → 同轴嵌套滚动（滚轮作用层
                                        // 不稳定、底部「应用」按钮与列表互相排挤、缩放时尺寸抖动），也违背本文件
                                        // 既定约定（961 行：内层不再套固定高度 ScrollArea，外层统一滚）。
                                        // 改为纯 scope：不产生第二个滚动容器，参数直接跟着外层滚。
                                        r_ui.scope(|ui| {
                                            for o in arr {
                                                let key = o.get("key").and_then(|x| x.as_str()).unwrap_or("?").to_string();
                                                let typ = o.get("type").and_then(|x| x.as_str()).unwrap_or("string").to_string();
                                                let desc = o.get("desc").and_then(|x| x.as_str()).unwrap_or("").to_string();
                                                let curv = cur.get(&key).cloned().unwrap_or(serde_json::Value::Null);
                                                ui.horizontal(|ui| {
                                                    ui.label(egui::RichText::new(key.to_string()).strong());
                                                    ui.add_space(6.0);
                                                    match typ.as_str() {
                                                        "number" => {
                                                            let mut f = curv.as_f64().unwrap_or(0.0);
                                                            let resp = ui.add(egui::DragValue::new(&mut f).speed(0.01));
                                                            if resp.changed() {
                                                                changed = Some((key.clone(), serde_json::json!(f)));
                                                            }
                                                        }
                                                        "bool" => {
                                                            let mut b = curv.as_bool().unwrap_or(false);
                                                            if ui.checkbox(&mut b, "").changed() {
                                                                changed = Some((key.clone(), serde_json::json!(b)));
                                                            }
                                                        }
                                                        "enum" => {
                                                            let opts: Vec<String> = o.get("options").and_then(|x| x.as_array()).map(|a| a.iter().filter_map(|v| v.as_str().map(|s| s.to_string())).collect()).unwrap_or_default();
                                                            let cur_s = curv.as_str().unwrap_or("").to_string();
                                                            egui::ComboBox::from_id_salt(format!("ad_enum_{}", key)).selected_text(cur_s.clone()).show_ui(ui, |ui| {
                                                                for opt in &opts {
                                                                    if ui.selectable_label(cur_s == *opt, opt.clone()).clicked() {
                                                                        changed = Some((key.clone(), serde_json::json!(opt)));
                                                                    }
                                                                }
                                                            });
                                                        }
                                                        "array" => {
                                                            let s = curv.as_array().map(|a| a.iter().filter_map(|v| v.as_str()).collect::<Vec<_>>().join(", ")).unwrap_or_default();
                                                            let mut buf = s;
                                                            if ui.text_edit_singleline(&mut buf).changed() {
                                                                let items: Vec<String> = buf.split(',').map(|x| x.trim().to_string()).filter(|x| !x.is_empty()).collect();
                                                                changed = Some((key.clone(), serde_json::json!(items)));
                                                            }
                                                        }
                                                        _ => {
                                                            let mut s = curv.as_str().unwrap_or("").to_string();
                                                            if ui.text_edit_singleline(&mut s).changed() {
                                                                changed = Some((key.clone(), serde_json::json!(s)));
                                                            }
                                                        }
                                                    }
                                                });
                                                if !desc.is_empty() {
                                                    ui.add_space(2.0);
                                                    ui.label(egui::RichText::new(desc).weak().small());
                                                }
                                                ui.add_space(4.0);
                                                ui.separator();
                                                ui.add_space(2.0);
                                            }
                                        });
                                        if let Some((k, v)) = changed {
                                            if let Some(eb) = lock_recover(&self.adapter_edit).get_mut(m.as_str()) {
                                                eb.insert(k, v);
                                            }
                                        }
                                        // 应用按钮（两步确认——Mr2109 2026-08-27——修改后确认才生效）
                                        r_ui.add_space(6.0);
                                        if !self.adapter_confirm {
                                            r_ui.horizontal(|ui| {
                                                if ui.button(t!("adapter.apply")).clicked() {
                                                    self.adapter_confirm = true;
                                                    self.adapter_msg.clear();
                                                }
                                                if !self.adapter_msg.is_empty() {
                                                    ui.colored_label(egui::Color32::from_rgb(80, 200, 120), &self.adapter_msg);
                                                }
                                            });
                                        } else {
                                            r_ui.horizontal(|ui| {
                                                ui.colored_label(egui::Color32::from_rgb(220, 180, 60), t!("adapter.confirm_apply", icon = icon_text("warning")));
                                                if ui.button(t!("adapter.confirm_ok")).clicked() {
                                                    let cfg = lock_recover(&self.adapter_edit).get(m.as_str()).cloned().unwrap_or_default();
                                                    let cfg = serde_json::Value::Object(cfg.into_iter().collect());
                                                    let name2 = m.clone();
                                                    let sstore = self.adapter_schema.clone();
                                                    let dstore = self.model_detail.clone();
                                                    let edstore = self.adapter_edit.clone();
                                                    api::runtime().spawn(async move {
                                                        let r = api::update_adapter_opts_blocking(&name2, cfg).await;
                                                        // 刷新 schema + detail + 编辑缓冲
                                                        let s = api::fetch_adapter_schema_blocking(&name2).await.ok();
                                                        if let Some(ref sv) = s {
                                                            if let Some(arr) = sv.get("schema").and_then(|v| v.as_array()) {
                                                                let mut init = std::collections::HashMap::new();
                                                                for o in arr {
                                                                    if let (Some(k), Some(v)) = (o.get("key").and_then(|x| x.as_str()), o.get("value")) {
                                                                        init.insert(k.to_string(), v.clone());
                                                                    }
                                                                }
                                                                lock_recover(&edstore).insert(name2.clone(), init);
                                                            }
                                                        }
                                                        *lock_recover(&sstore) = s;
                                                        let d = api::fetch_model_detail_blocking(&name2).await.ok();
                                                        *lock_recover(&dstore) = d;
                                                    });
                                                    self.adapter_msg = t!("adapter.applied").to_string();
                                                    self.adapter_confirm = false;
                                                }
                                                if ui.button(t!("action.cancel_x")).clicked() {
                                                    self.adapter_confirm = false;
                                                    self.adapter_msg.clear();
                                                }
                                            });
                                        }
                                        r_ui.weak(t!("adapter.hint"));
                                    }
                                } else {
                                    r_ui.weak(t!("adapter.none"));
                                }
                            }
                            None => {
                                r_ui.weak(t!("adapter.loading"));
                            }
                        }
                        }
                    }
                    None => {
                        r_ui.weak(t!("adapter.loading"));
                    }
                }
            } else {
                r_ui.weak(t!("models.click_detail"));
            }
                    }); // models_right_scroll 结束
            } // 右列块结束
        });
        if dragging {
            self.save_layout_ratio("split_model", self.split_model);
        }
    }

    /// APP-A07（2026-09-10 审计）: 内部任务启停包成 SharedResult——api.rs 的 start/stop 是
    /// async fn（无异步句柄版），这里在 app.rs 内本地包装，不改 api.rs 签名。
    /// M06 双渲染器：预览渲染器偏好持久化（~/.zerg-ui-prefs.json——重启后保持Mr2109的选择）
    fn preview_pref_path() -> std::path::PathBuf {
        // 2026-09-13 Q10：偏好文件落点迁到 <UI 状态目录>/prefs.json
        // （读时兼容旧 ~/.zerg-ui-prefs.json——见 api::read_prefs）
        crate::api::prefs_path()
    }
    /// 语言偏好持久化（多语言决策②：显式切换写入 ~/.zerg-ui-prefs.json 的 locale 字段）
    fn save_locale_pref(locale: &str) {
        let p = Self::preview_pref_path();
        // 读：新落点优先 → 旧 HOME 根文件回退（不丢既有字段）
        let mut v: serde_json::Value = crate::api::read_prefs()
            .and_then(|s| serde_json::from_str(&s).ok())
            .unwrap_or_else(|| serde_json::json!({}));
        if !v.is_object() {
            v = serde_json::json!({});
        }
        v["locale"] = serde_json::Value::String(locale.to_string());
        if let Ok(s) = serde_json::to_string_pretty(&v) {
            let _ = std::fs::write(&p, s);
        }
    }

    fn it_ctrl_async(start: bool) -> api::SharedResult<()> {
        let out: api::SharedResult<()> = Arc::new(Mutex::new(None));
        let out2 = out.clone();
        api::runtime().spawn(async move {
            let r = if start {
                api::start_internal_tasks_blocking().await.map(|_| ())
            } else {
                api::stop_internal_tasks_blocking().await.map(|_| ())
            };
            *lock_recover(&out2) = Some(r);
        });
        out
    }

    /// 内部任务视图（Mr2109 2026-08-22——看到所有内部任务 + 手动执行按钮）
    fn internal_tasks_view(&mut self, ui: &mut egui::Ui) {
        // 设计 §4.4：删重复标题块（heading + 简介 weak）。
        // 丙批 N4（2026-09-10）：前缀缓存命中率面板（网关 8082——数据来自 /api/metrics/prefix_cache）
        {
            let pc = lock_recover(&self.prefix_cache).clone();
            let mut refresh = false;
            egui::CollapsingHeader::new(t!("it.prefix_cache", icon = icon_text("chart-line")))
                .default_open(false)
                .show(ui, |ui| {
                    match &pc {
                        Some(Ok(v)) => {
                            let f = |k: &str| v.get(k).and_then(|x| x.as_f64()).unwrap_or(0.0);
                            let u = |k: &str| v.get(k).and_then(|x| x.as_u64()).unwrap_or(0);
                            let ver = v
                                .get("prompt_version")
                                .and_then(|x| x.as_str())
                                .unwrap_or("");
                            let samples = v
                                .get("window")
                                .and_then(|w| w.get("samples"))
                                .and_then(|x| x.as_u64())
                                .unwrap_or(0);
                            ui.horizontal(|ui| {
                                ui.label(t!(
                                    "it.hit_rate",
                                    pct = format!("{:.1}", f("ratio") * 100.0)
                                ));
                                ui.weak(t!(
                                    "it.samples",
                                    n = samples,
                                    hits = u("hits"),
                                    misses = u("misses")
                                ));
                            });
                            if v.get("has_baseline")
                                .and_then(|x| x.as_bool())
                                .unwrap_or(false)
                            {
                                ui.weak(t!(
                                    "it.baseline",
                                    pct = format!("{:.1}", f("baseline_ratio") * 100.0)
                                ));
                            }
                            ui.weak(t!(
                                "it.prompt_version",
                                ver = if ver.is_empty() {
                                    t!("it.no_sample").to_string()
                                } else {
                                    ver.to_string()
                                }
                            ));
                            if let Some(alerts) = v.get("alerts").and_then(|x| x.as_array()) {
                                if !alerts.is_empty() {
                                    ui.colored_label(
                                        egui::Color32::from_rgb(220, 100, 90),
                                        t!(
                                            "it.alerts",
                                            icon = icon_text("warning"),
                                            n = alerts.len()
                                        ),
                                    );
                                    for al in alerts.iter().take(2) {
                                        if let Some(msg) =
                                            al.get("message").and_then(|x| x.as_str())
                                        {
                                            ui.weak(msg);
                                        }
                                    }
                                }
                            }
                        }
                        Some(Err(e)) => {
                            ui.colored_label(
                                egui::Color32::from_rgb(220, 100, 90),
                                t!("it.read_failed", err = e),
                            );
                        }
                        None => {
                            ui.weak(t!("it.reading"));
                        }
                    }
                    if ui.small_button(t!("action.refresh")).clicked() {
                        refresh = true;
                    }
                });
            if refresh {
                self.last_pc_fetch = std::time::Instant::now() - std::time::Duration::from_secs(60);
            }
        }
        ui.add_space(6.0);
        ui.add_space(8.0);
        // v2.5.6 内部任务启停按钮（Mr2109 2026-08-27）
        // A07 治本（2026-09-10）：状态显示取自后端单一真相源——本地不再持有“真相”，只做乐观提示
        ui.horizontal(|ui| {
            let snap = lock_recover(&self.engine_state).clone();
            let (known, enabled, stopped, running, reason, tick) = match &snap {
                Some(Ok(v)) => {
                    let s = v.get("state").unwrap_or(v);
                    let g = |k: &str| s.get(k).and_then(|x| x.as_str()).unwrap_or("").to_string();
                    (
                        true,
                        s.get("enabled").and_then(|x| x.as_bool()).unwrap_or(false),
                        s.get("stopped").and_then(|x| x.as_bool()).unwrap_or(false),
                        s.get("running").and_then(|x| x.as_bool()).unwrap_or(false),
                        g("reason"),
                        g("last_tick"),
                    )
                }
                _ => (false, false, false, false, String::new(), String::new()),
            };
            let err = match &snap {
                Some(Err(e)) => Some(e.clone()),
                _ => None,
            };
            if let Some(b) = self.it_ctrl_busy {
                ui.colored_label(
                    egui::Color32::from_rgb(220, 180, 60),
                    t!(
                        "it.switching",
                        target = if b {
                            t!("it.start").to_string()
                        } else {
                            t!("it.stop").to_string()
                        }
                    ),
                );
            } else if self.it_ctrl_confirm {
                // APP-A07: 二次确认（防误点启停引擎）
                ui.colored_label(
                    egui::Color32::from_rgb(220, 180, 60),
                    t!(
                        "it.confirm_engine",
                        icon = icon_text("warning"),
                        action = if running {
                            t!("it.stop").to_string()
                        } else {
                            t!("it.start").to_string()
                        }
                    ),
                );
                if ui.button(t!("action.confirm")).clicked() {
                    let target = !running;
                    self.it_ctrl_target = Some(target);
                    self.it_ctrl_busy = Some(target);
                    self.it_ctrl_result = Self::it_ctrl_async(target);
                    self.it_ctrl_confirm = false;
                }
                if ui.button(t!("action.cancel_x")).clicked() {
                    self.it_ctrl_confirm = false;
                }
            } else {
                let label = if running {
                    t!("it.stop_task", icon = icon_text("stop-circle")).to_string()
                } else {
                    t!("it.start_task", icon = icon_text("play")).to_string()
                };
                if ui.add_enabled(known, egui::Button::new(label)).clicked() {
                    self.it_ctrl_confirm = true;
                }
            }
            // 状态文案（诚实呈现：未启用 / 已停止 / 运行中 / 异常）
            if let Some(e) = err {
                ui.colored_label(
                    egui::Color32::from_rgb(220, 100, 90),
                    t!("it.state_unknown", err = e),
                );
            } else if !known {
                ui.weak(t!("it.state_loading"));
            } else if !enabled {
                ui.colored_label(
                    egui::Color32::GRAY,
                    t!(
                        "it.not_enabled",
                        reason = if reason.is_empty() {
                            t!("it.gate_hint").to_string()
                        } else {
                            reason
                        }
                    ),
                );
            } else if stopped {
                ui.colored_label(egui::Color32::from_rgb(220, 180, 60), t!("it.stopped_hint"));
            } else if running {
                ui.colored_label(
                    egui::Color32::from_rgb(120, 200, 120),
                    t!("it.running_hint"),
                );
            } else {
                ui.colored_label(
                    egui::Color32::from_rgb(220, 100, 90),
                    t!(
                        "it.abnormal",
                        reason = if reason.is_empty() {
                            t!("it.heartbeat_stall").to_string()
                        } else {
                            reason
                        }
                    ),
                );
            }
            if !tick.is_empty() && tick.len() >= 16 {
                ui.weak(t!("it.heartbeat", tick = &tick[11..16]));
            }
        });
        ui.add_space(8.0);
        // 拉取内部任务清单（缓存——每 30 秒刷新）
        if lock_recover(&self.internal_tasks).is_none()
            || self.last_it_fetch.elapsed().as_secs() > 30
        {
            let store = self.internal_tasks.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                let items = api::fetch_internal_tasks_blocking()
                    .await
                    .unwrap_or_default();
                let mut st = lock_recover(&store);
                *st = Some(items);
            });
            self.last_it_fetch = now;
        }
        // 周期配置（缓存——60 秒刷新）
        // APP-A06: 改用独立计时器（原来共用 last_it_fetch——被上面每 30s 的清单刷新归零，
        // 60s 条件永远不成立 → 周期下拉一直显示启动时拉的旧值）
        if lock_recover(&self.it_intervals).is_none()
            || self.last_it_interval_fetch.elapsed().as_secs() > 60
        {
            let store = self.it_intervals.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                let v = api::fetch_internal_intervals_blocking().await.ok();
                let mut st = lock_recover(&store);
                *st = v;
            });
            self.last_it_interval_fetch = now;
        }
        // 周期配置 map（defID → 小时）
        let interval_map: std::collections::HashMap<String, f64> = lock_recover(&self.it_intervals)
            .clone()
            .and_then(|v| v.get("intervals").cloned())
            .and_then(|v| v.as_object().cloned())
            .map(|obj| {
                obj.iter()
                    .filter_map(|(k, v)| v.as_f64().map(|h| (k.clone(), h)))
                    .collect()
            })
            .unwrap_or_default();
        let items = lock_recover(&self.internal_tasks)
            .clone()
            .unwrap_or_default();
        if items.is_empty() {
            ui.weak(t!("it.loading"));
            return;
        }
        // v2.5.6 左右布局（Mr2109 2026-08-27）: 左=任务列表（可点击）——右=详情+skill
        let sel = self.it_selected.clone();
        let mut clicked: Option<String> = None;
        // 左右可拖分割（Mr2109 2026-08-27——间隔可拖——比例持久化——窗口宽高变化自动跟随）
        let total_w = ui.available_width();
        let avail_h = ui.available_height().max(200.0);
        let left_w = (total_w * self.split_it).clamp(220.0, total_w - 340.0);
        let mut dragging = false;
        ui.allocate_ui_with_layout(
            egui::vec2(total_w, avail_h),
            egui::Layout::left_to_right(egui::Align::TOP),
            |ui| {
                // 左列: 任务清单（点击选中）
                let (lrect, _) =
                    ui.allocate_exact_size(egui::vec2(left_w, avail_h), egui::Sense::hover());
                let mut l_ui = ui.new_child(
                    egui::UiBuilder::new()
                        .max_rect(lrect)
                        .layout(egui::Layout::top_down(egui::Align::Min)),
                );
                egui::ScrollArea::vertical()
                    .id_salt("it_left_list")
                    .auto_shrink(false)
                    .show(&mut l_ui, |ui| {
                        for it in &items {
                            let id = it
                                .get("id")
                                .and_then(|v| v.as_str())
                                .unwrap_or("?")
                                .to_string();
                            let desc = it
                                .get("description")
                                .and_then(|v| v.as_str())
                                .unwrap_or("?")
                                .to_string();
                            // 左列截断（Mr2109 2026-08-27——超长文字叠到右界面）
                            let desc_short: String = if desc.chars().count() > 32 {
                                format!("{}…", desc.chars().take(30).collect::<String>())
                            } else {
                                desc.clone()
                            };
                            ui.horizontal(|ui| {
                                if ui.button(t!("action.run")).clicked() {
                                    // APP-A11: 失败提示（原 let _ = 静默）
                                    let id2 = id.clone();
                                    let perr = self.poll_err.clone();
                                    api::runtime().spawn(async move {
                                        if let Err(e) = api::run_internal_task_blocking(&id2).await
                                        {
                                            *lock_recover(&perr) = Some(
                                                t!("it.run_failed", id = id2, err = e).to_string(),
                                            );
                                        }
                                    });
                                }
                                // 运行模式开关（Mr2109 2026-08-28——自动/手动——点击切换）
                                let auto_run =
                                    it.get("auto_run").and_then(|v| v.as_bool()).unwrap_or(true);
                                let mode_btn = if auto_run {
                                    t!("it.mode_auto").to_string()
                                } else {
                                    t!("it.mode_manual").to_string()
                                };
                                if ui
                                    .selectable_label(false, mode_btn)
                                    .on_hover_text(t!("it.mode_tip"))
                                    .clicked()
                                {
                                    let id2 = id.clone();
                                    let new_mode = !auto_run;
                                    let perr = self.poll_err.clone();
                                    api::runtime().spawn(async move {
                                        // APP-A11: 失败提示（原 let _ = 静默）
                                        if let Err(e) =
                                            api::set_internal_mode_blocking(&id2, new_mode).await
                                        {
                                            *lock_recover(&perr) = Some(
                                                t!("it.mode_failed", id = id2, err = e).to_string(),
                                            );
                                        }
                                    });
                                    // 本地立即翻转（后端刷新 30s 后同步）
                                    if let Some(items_ref) =
                                        lock_recover(&self.internal_tasks).as_mut()
                                    {
                                        for it2 in items_ref.iter_mut() {
                                            if it2.get("id").and_then(|v| v.as_str())
                                                == Some(id.as_str())
                                            {
                                                if let Some(obj) = it2.as_object_mut() {
                                                    obj.insert(
                                                        "auto_run".to_string(),
                                                        serde_json::json!(new_mode),
                                                    );
                                                }
                                            }
                                        }
                                    }
                                }
                                // 周期下拉（Mr2109 2026-08-27——任务循环周期 1h-24h/指定——未设置默认显示默认执行周期）
                                let cur_h = interval_map.get(&id).copied().unwrap_or(0.0);
                                let def_h = it
                                    .get("default_hours")
                                    .and_then(|v| v.as_f64())
                                    .unwrap_or(0.0);
                                let sel_text = if cur_h > 0.0 {
                                    if cur_h == cur_h.trunc() {
                                        format!("⏱{}h", cur_h as i64)
                                    } else {
                                        t!("it.interval_hours", h = cur_h).to_string()
                                    }
                                } else if def_h > 0.0 {
                                    t!("it.interval_default", h = def_h as i64).to_string()
                                // 默认执行周期
                                } else {
                                    t!("it.interval").to_string()
                                };
                                egui::ComboBox::from_id_salt(format!("it_iv_{}", id))
                                    .selected_text(sel_text)
                                    .width(70.0)
                                    .show_ui(ui, |ui| {
                                        for opt in [0.0_f64, 1.0, 2.0, 6.0, 12.0, 24.0] {
                                            let text = if opt == 0.0 {
                                                t!("it.interval_none").to_string()
                                            } else {
                                                t!("it.hours", h = opt as i64).to_string()
                                            };
                                            if ui.selectable_label(cur_h == opt, text).clicked() {
                                                let id2 = id.clone();
                                                let perr = self.poll_err.clone();
                                                api::runtime().spawn(async move {
                                                    // APP-A11: 失败提示（原 let _ = 静默）
                                                    if let Err(e) =
                                                        api::set_internal_interval_blocking(
                                                            &id2, opt,
                                                        )
                                                        .await
                                                    {
                                                        *lock_recover(&perr) = Some(
                                                            t!(
                                                                "it.set_interval_failed",
                                                                id = id2,
                                                                err = e
                                                            )
                                                            .to_string(),
                                                        );
                                                    }
                                                });
                                            }
                                        }
                                        // 指定周期
                                        if self.it_custom_for.as_deref() == Some(id.as_str()) {
                                            let mut h = self.it_custom_hours.clone();
                                            if ui.text_edit_singleline(&mut h).changed() {
                                                self.it_custom_hours = h.clone();
                                            }
                                            if ui.button(t!("action.ok")).clicked() {
                                                if let Ok(hv) = h.trim().parse::<f64>() {
                                                    if hv > 0.0 {
                                                        let id2 = id.clone();
                                                        let perr = self.poll_err.clone();
                                                        api::runtime().spawn(async move {
                                                            // APP-A11: 失败提示（原 let _ = 静默）
                                                            if let Err(e) =
                                                                api::set_internal_interval_blocking(
                                                                    &id2, hv,
                                                                )
                                                                .await
                                                            {
                                                                *lock_recover(&perr) = Some(
                                                                    t!(
                                                                        "it.set_custom_failed",
                                                                        id = id2,
                                                                        err = e
                                                                    )
                                                                    .to_string(),
                                                                );
                                                            }
                                                        });
                                                    }
                                                }
                                                self.it_custom_for = None;
                                                self.it_custom_hours.clear();
                                            }
                                        } else if ui
                                            .selectable_label(
                                                false,
                                                t!(
                                                    "it.interval_custom",
                                                    icon = icon_text("pencil-simple")
                                                ),
                                            )
                                            .clicked()
                                        {
                                            self.it_custom_for = Some(id.clone());
                                            self.it_custom_hours = format!("{}", cur_h as i64);
                                        }
                                    });
                                let selected = sel.as_deref() == Some(id.as_str());
                                if ui
                                    .selectable_label(selected, format!("{} | {}", id, desc_short))
                                    .clicked()
                                {
                                    clicked = Some(id.clone());
                                }
                            });
                            ui.add_space(2.0);
                        }
                    });
                // 拖拽条（8px 分隔线——鼠标移到显示 ⇔ 光标——可拖）
                let (drect, dresp) =
                    ui.allocate_exact_size(egui::vec2(8.0, avail_h), egui::Sense::drag());
                ui.painter()
                    .rect_filled(drect, 0.0, ui.visuals().faint_bg_color);
                ui.painter().vline(
                    drect.center().x,
                    drect.y_range(),
                    egui::Stroke::new(1.0, ui.visuals().weak_text_color()),
                );
                let _ = dresp
                    .clone()
                    .on_hover_cursor(egui::CursorIcon::ResizeHorizontal);
                if dresp.dragged() {
                    let dx = ui.input(|i| i.pointer.delta().x);
                    self.split_it = (self.split_it + dx / total_w).clamp(0.2, 0.6);
                }
                // APP-A12: 松手才落盘
                if dresp.drag_stopped() {
                    dragging = true;
                }
                // 右列: 选中任务详情 + skill
                let right_w = (ui.available_width() - 8.0).max(280.0);
                let (rrect, _) =
                    ui.allocate_exact_size(egui::vec2(right_w, avail_h), egui::Sense::hover());
                let mut r_ui = ui.new_child(egui::UiBuilder::new().max_rect(rrect));
                let detail_item = items
                    .iter()
                    .find(|it| it.get("id").and_then(|v| v.as_str()) == sel.as_deref());
                match detail_item {
                    Some(it) => {
                        let desc = it
                            .get("description")
                            .and_then(|v| v.as_str())
                            .unwrap_or("?")
                            .to_string();
                        let template = it
                            .get("template")
                            .and_then(|v| v.as_str())
                            .unwrap_or("?")
                            .to_string();
                        let cooldown = it
                            .get("cooldown")
                            .and_then(|v| v.as_str())
                            .unwrap_or("?")
                            .to_string();
                        let skill = it
                            .get("skill")
                            .and_then(|v| v.as_str())
                            .unwrap_or("")
                            .to_string();
                        r_ui.heading(format!("🔧 {}", sel.as_deref().unwrap_or("?")));
                        r_ui.add_space(4.0);
                        r_ui.label(t!("it.notes", desc = desc));
                        r_ui.label(t!("it.cooldown", c = cooldown));
                        r_ui.add_space(4.0);
                        r_ui.separator();
                        r_ui.strong(t!("it.template"));
                        r_ui.label(template);
                        r_ui.add_space(8.0);
                        r_ui.separator();
                        r_ui.strong(t!("it.skill_content"));
                        if skill.is_empty() {
                            r_ui.weak(t!("it.skill_none"));
                        } else {
                            egui::ScrollArea::vertical()
                                .id_salt("it_right_skill")
                                .max_height(320.0)
                                .show(&mut r_ui, |ui| {
                                    ui.label(skill);
                                });
                        }
                    }
                    None => {
                        r_ui.weak(t!("it.click_detail"));
                    }
                }
            },
        );
        if dragging {
            self.save_layout_ratio("split_it", self.split_it);
        }
        if let Some(c) = clicked {
            self.it_selected = Some(c);
        }
    }

    // ─── 布局持久化（Mr2109 2026-08-27——左右分割比例——拖动后下次启动默认）───
    fn layout_path() -> std::path::PathBuf {
        // E18/Q10（2026-09-13）：布局比例从 <任务根>/ui_layout.json 搬到 <UI 状态目录>/ui_layout.json
        // （~/.zerg/state/ui）——首访由 api::migrate_persistent_state_once() 一次性迁移，不覆盖已有目标。
        api::ui_dir().join("ui_layout.json")
    }
    fn save_layout_ratio(&self, key: &str, val: f32) {
        let path = Self::layout_path();
        let mut map: std::collections::HashMap<String, f32> = std::collections::HashMap::new();
        if let Ok(s) = std::fs::read_to_string(&path) {
            if let Ok(m) = serde_json::from_str::<std::collections::HashMap<String, f32>>(&s) {
                map = m;
            }
        }
        map.insert(key.to_string(), val);
        // APP-A12: 写失败不再静默（原 let _ = 吞掉——布局悄悄不保存）
        if let Err(e) = std::fs::write(&path, serde_json::to_string(&map).unwrap_or_default()) {
            eprintln!(
                "[zerg-ui] failed to persist layout {}: {}",
                path.display(),
                e
            ); // 日志英文（设计稿 §7-3）
        }
    }
}

// 布局比例持久化（Mr2109 2026-08-27——左右分割——拖动后下次启动默认）
fn load_layout_ratio(key: &str, default: f32) -> f32 {
    let p = api::ui_dir().join("ui_layout.json");
    if let Ok(s) = std::fs::read_to_string(p) {
        if let Ok(m) = serde_json::from_str::<std::collections::HashMap<String, f32>>(&s) {
            if let Some(v) = m.get(key) {
                return *v;
            }
        }
    }
    default
}

impl eframe::App for ZergApp {
    fn ui(&mut self, ui: &mut egui::Ui, _frame: &mut eframe::Frame) {
        let now = ui.ctx().input(|i| i.time);
        // 每帧更新异步（定时触发 + 收集）
        self.update_async(now);
        // APP-A17（2026-09-10 审计）: 原来这里是无条件 `request_repaint()`——界面完全静止
        // 也满速重绘（GPU/CPU 常驻占用、笔记本耗电）。改为按需：
        //  · 真有后台任务在飞（内部任务启停）或某个茧在跑 → 立即重绘；
        //  · 其余时候 500ms 唤醒一次（数据轮询是 3/5/10/30/60s 级，帧级重绘没有任何意义，
        //    但完全不等又会饿死定时轮询，故保留一个低频心跳）。
        // 2026-09-13（C9 第 4 步）：文档界面的 AI/写操作随茧迁出 ⇒ 这两路不再由宿主驱动重绘。
        let busy = lock_recover(&self.it_ctrl_result).is_some()
            || self.it_ctrl_busy.is_some()
            || self.cocoon_app.is_some();
        if busy {
            ui.ctx().request_repaint();
        } else {
            ui.ctx()
                .request_repaint_after(std::time::Duration::from_millis(500));
        }

        if !self.online {
            egui::Panel::top("status").show(ui, |ui| {
                ui.horizontal(|ui| {
                    ui.colored_label(
                        egui::Color32::from_rgb(220, 80, 80),
                        t!("status.offline_waiting"),
                    );
                });
            });
            egui::CentralPanel::default().show(ui, |ui| {
                ui.centered_and_justified(|ui| {
                    ui.label(
                        egui::RichText::new(t!("status.offline_full"))
                            .size(24.0)
                            .color(egui::Color32::from_rgb(150, 150, 150)),
                    );
                });
            });
            return;
        }

        // v2.5.6 顶部导航（船桥——Mr2109 2026-08-29：板块上移一排 + 右侧 English/用户）
        egui::Panel::top("nav").show(ui, |ui| {
            let mut switch_locale = false;
            let mut open_manager = false;
            // 记住「父 + 子」（Q9/Q10）：导航切换后若选中/记忆变了 ⇒ 落盘 ui_state.json
            let nav_before = (
                self.registry.active.clone(),
                self.registry.remembered_child.clone(),
            );
            crate::modules::top_nav_bar(
                ui,
                &mut self.registry,
                self.online,
                &self.locale,
                &mut || {
                    switch_locale = true;
                },
                &mut || {
                    open_manager = true;
                },
                !self.hud_hidden,
                &mut || {
                    self.hud_hidden = !self.hud_hidden;
                },
            );
            if switch_locale {
                if self.locale == "zh-CN" {
                    self.locale = "en".to_string();
                    rust_i18n::set_locale("en");
                } else {
                    self.locale = "zh-CN".to_string();
                    rust_i18n::set_locale("zh-CN");
                }
                Self::save_locale_pref(&self.locale);
            }
            if open_manager {
                self.show_module_manager = true; // ➕ 打开吊装系统面板
            }
            if nav_before.0 != self.registry.active
                || nav_before.1 != self.registry.remembered_child
            {
                self.registry.save_state();
            }
        });

        // v2.5.6 模块管理面板（➕ 吊装系统——M2）
        if self.show_module_manager {
            self.module_manager_view(ui.ctx());
        }

        egui::CentralPanel::default().show(ui, |ui| {
            // APP-A04/A11: 轮询/操作失败提示条（原来失败全静默——界面看起来"一切正常"）
            let notice = lock_recover(&self.poll_err).clone();
            if let Some(msg) = notice {
                ui.horizontal(|ui| {
                    ui.colored_label(egui::Color32::from_rgb(230, 90, 90), format!("⚠ {}", msg));
                    if ui.small_button(t!("action.clear")).clicked() {
                        *lock_recover(&self.poll_err) = None;
                    }
                });
                ui.separator();
            }
            // 设计 §4.4/E19（2026-09-13）：内容区统一加一层顶部呼吸（**一处**——不在每页自己写）。
            ui.add_space(6.0);
            self.main_view(ui);
        });

        // v2.5.7 HUD 悬浮层（右上角——core 状态/模块/running 数——点 ✕ 隐藏——nav 图标开关）
        if !self.hud_hidden && self.online {
            self.hud_view(ui.ctx());
        }
    }
}

/// 平台卡片能否打开（C9 第 1 步——**铭牌驱动**，宿主不再写死「哪个茧已装载」）：
/// - 契约茧：看铭牌的 `loaded`（未装载 ⇒ 卡片照常显示 + 标「未装载」+ 打不开 + 给安装指引）
/// - 非茧 id（宿主自己的页：对话/任务/集群……）：恒可——它们走主仓自己的渲染臂
fn cocoon_openable(id: &str) -> bool {
    match crate::modules::cocoon::meta_of(id) {
        Some(m) => crate::modules::cocoon::openable(m),
        None => true,
    }
}

/// C9 第 3 步：未装载茧的**安装指引**弹窗（设计 §4.3：「点开或点『安装』⇒ 平台给出安装指引：
/// 告知茧的独立仓地址与目标目录，并提示 clone 后重新编译装载。绝不静默失败、绝不假装可用」）。
///
/// 返回 `true` = 用户点了「关闭」（宿主清掉打开态）。**纯渲染**：数据只从铭牌与 i18n 取。
/// 不是茧 / 已装载 ⇒ 直接返回 `true`（不画窗口——免得弹出个空窗）。
fn cocoon_install_hint_window(ui: &egui::Ui, id: &str) -> bool {
    let Some(repo) = crate::modules::cocoon::install_guide(id) else {
        return true;
    };
    let name = crate::modules::cocoon::meta_of(id)
        .map(|m| {
            if m.name_key.is_empty() {
                m.name.to_string()
            } else {
                t!(m.name_key).to_string()
            }
        })
        .unwrap_or_else(|| id.to_string());
    let mut close = false;
    egui::Window::new(t!("cocoon.install_title"))
        .collapsible(false)
        .resizable(false)
        .id(egui::Id::new("cocoon_install_hint"))
        .show(ui.ctx(), |ui| {
            ui.strong(name);
            ui.add_space(6.0);
            ui.label(t!("cocoon.install_repo"));
            // 可选中 ⇒ 能直接复制（不写死路径，不含任何私有绝对路径）
            ui.add(egui::Label::new(egui::RichText::new(repo).monospace()).selectable(true));
            ui.add_space(8.0);
            ui.label(t!("cocoon.install_steps"));
            ui.add_space(10.0);
            if ui.button(t!("action.close")).clicked() {
                close = true;
            }
        });
    close
}

/// APP-A14（2026-09-10 审计）: 共享状态取锁统一走这里——`Mutex::lock().unwrap()`
/// 在锁中毒（持锁任务 panic 过）时会直接 panic，而本文件所有写侧都跑在 `api::runtime().spawn`
/// 的线程里，任何一次异常都会让 UI 线程此后每帧崩掉。改用 `into_inner()`：中毒只是
/// “上一个持有者异常退出”，续用内部值比整页崩溃安全得多（chat_view.rs 同款实现）。
fn lock_recover<T>(m: &Mutex<T>) -> std::sync::MutexGuard<'_, T> {
    m.lock().unwrap_or_else(|e| e.into_inner())
}

/// 任务 ID 缩短显示
/// APP-A16（2026-09-10 审计）: 按「字符」取尾 6 个——原来 `&id[id.len() - 6..]` 是字节下标，
/// 后端若返回含中文/多字节的 id 就会切在字符中间 panic（'byte index is not a char boundary'）
/// → 整个 UI 退出。本文件 429 行的 AI 截断早就做了 is_char_boundary 保护，此处统一。
fn short_id(id: &str) -> &str {
    if id.chars().count() <= 6 {
        return id;
    }
    // 从后往前数第 6 个字符的起始字节位置（char_indices 保证落在边界上）
    let idx = id.char_indices().rev().nth(5).map(|(i, _)| i).unwrap_or(0);
    &id[idx..]
}

/// 任务执行时长（created_at → completed_at——人性化：秒/分/时）
fn task_duration(created: Option<&str>, completed: Option<&str>) -> String {
    let parse = |s: Option<&str>| -> Option<i64> {
        s.and_then(|v| {
            // 尝试 RFC3339 / unix 秒
            v.parse::<i64>().ok().or_else(|| {
                chrono::DateTime::parse_from_rfc3339(v)
                    .ok()
                    .map(|d| d.timestamp())
            })
        })
    };
    let c = parse(created);
    let d = parse(completed);
    match (c, d) {
        // 零值时间（0001-01-01——CreatedAt 未设置——旧复查任务）→ 未知（2026-08-21 修复——不显示巨大错误时长）
        (Some(c), _) if c < 0 => t!("common.unknown").to_string(),
        (Some(c), Some(d)) if d >= c => {
            let secs = d - c;
            if secs < 60 {
                format!("{}s", secs)
            } else if secs < 3600 {
                format!("{}m{}s", secs / 60, secs % 60)
            } else {
                format!("{}h{}m", secs / 3600, (secs % 3600) / 60)
            }
        }
        (Some(_), None) => t!("task.in_progress").to_string(),
        _ => "—".to_string(),
    }
}

/// 内部任务视图（Mr2109 2026-08-22——看到所有内部任务 + 手动执行按钮）
/// 分支名通俗化（task-internal-health-check-1787... → 内部任务·健康巡检）
fn humanize_branch(branch: &str) -> String {
    let b = branch.trim();
    if b == "main" {
        return t!("git.main_branch").to_string();
    }
    // task-xxx-1787... 格式——提取类型
    if let Some(idx) = b.find("task-") {
        let rest = &b[idx + 5..];
        // 去掉时间戳（最后一段数字）
        let parts: Vec<&str> = rest.split('-').collect();
        if parts.len() >= 2 {
            // 内部任务（internal-类型）
            if parts[0] == "internal" && parts.len() >= 3 {
                let task_type: String = match parts[1] {
                    "health" => t!("it.type_health").to_string(),
                    "tool" => t!("it.type_tool").to_string(),
                    "code" => t!("it.type_code").to_string(),
                    "knowledge" => t!("it.type_knowledge").to_string(),
                    _ => parts[1].to_string(),
                };
                return t!("task.internal_label", ty = task_type).to_string();
            }
            // 外部任务（task-xxx）
            return t!("task.external_label", id = parts[0]).to_string();
        }
        return rest.to_string();
    }
    b.to_string()
}

impl ZergApp {
    /// HUD 悬浮层（挂起清单 ③——最小实现：core 状态点 + 当前模块 + running 任务数）
    fn hud_view(&mut self, ctx: &egui::Context) {
        // APP-A23 收口（2026-09-11 单一来源改造）: 版本号**不再多处硬编码**——
        //   · UI 侧唯一来源 = ui/Cargo.toml `version`（窗口标题 + 底栏 + 10 个虫茧箱版本全用
        //     env!("CARGO_PKG_VERSION") 编译期取值，改一处即全改）
        //   · Go 侧唯一来源 = core/internal/version.Version（启动横幅 + /api/capabilities + openapi info.version）
        //   · 两处一致性由门禁 scripts/check_version.py 断言（CI + 发布导出）
        // 收版时改这两处 + 运行中二进制复核（/api/capabilities、/api/openapi.json、窗口标题）。
        // 数据：当前模块名 + running 任务数（复用现有 tasks——不新拉）
        // E16（2026-09-13）：HUD「当前模块」显示 **父 › 子** 两段。
        // 纯函数 breadcrumb_keys() 产出 i18n 键序列（可单测）——无父箱时只显示自身名。
        let eff = self.effective_active();
        let crumb_keys = self.registry.breadcrumb_keys(&eff);
        let mod_name: String = if crumb_keys.is_empty() {
            eff.clone()
        } else {
            crumb_keys
                .iter()
                .map(|k| t!(*k).to_string())
                .collect::<Vec<_>>()
                .join(" › ")
        };
        let running = lock_recover(&self.tasks)
            .as_ref()
            .map(|ts| {
                ts.iter()
                    .filter(|t| t.status.as_deref() == Some("running"))
                    .count()
            })
            .unwrap_or(0);
        let (dot, dot_color) = if self.online {
            ("●", egui::Color32::from_rgb(80, 220, 120))
        } else {
            ("○", egui::Color32::from_rgb(220, 80, 80))
        };
        egui::Area::new(egui::Id::new("zerg_hud"))
            .anchor(egui::Align2::RIGHT_TOP, egui::vec2(-10.0, 36.0))
            .order(egui::Order::Foreground)
            .show(ctx, |ui| {
                egui::Frame::new()
                    .fill(egui::Color32::from_rgba_unmultiplied(18, 20, 26, 200))
                    .stroke(egui::Stroke::new(1.0, egui::Color32::from_gray(70)))
                    .corner_radius(8.0)
                    .inner_margin(egui::Margin::symmetric(10, 6))
                    .show(ui, |ui| {
                        ui.horizontal(|ui| {
                            ui.colored_label(dot_color, dot);
                            ui.label(egui::RichText::new("core").size(11.0));
                            ui.separator();
                            ui.label(egui::RichText::new(mod_name.as_str()).size(11.0).strong());
                            ui.separator();
                            if running > 0 {
                                ui.colored_label(
                                    egui::Color32::from_rgb(250, 200, 90),
                                    egui::RichText::new(t!("hud.running", n = running)).size(11.0),
                                );
                            } else {
                                ui.weak(egui::RichText::new(t!("hud.idle")).size(11.0));
                            }
                            if ui
                                .small_button("✕")
                                .on_hover_text(t!("hud.hide_tip"))
                                .clicked()
                            {
                                self.hud_hidden = true;
                            }
                        });
                    });
            });
    }
}

#[cfg(test)]
mod nav_trim_tests {
    //! 2026-09-13 设计《UI 大调动-导航精简与分组》§4.4/§七6：源码级断言（能失败）。
    //! 11 处「重复标题块」必须消失；对照组（保留项）必须仍在。

    const APP: &str = include_str!("app.rs");
    const CHAT: &str = include_str!("modules/chat/chat_view.rs");
    const MREG: &str = include_str!("modules/model_registry.rs");
    const UPGRADE: &str = include_str!("modules/upgrade.rs");
    const ZH_YML: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/locales/zh-CN.yml"));
    const EN_YML: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/locales/en.yml"));

    /// §4.4 要删的 11 处（app.rs 8 + chat_view 1 + model_registry 1 + upgrade 1）全部消失。
    #[test]
    fn eleven_duplicate_title_blocks_are_gone() {
        for pat in [
            "ui.heading(t!(\"cluster.status\"))",
            "ui.heading(t!(\"git.overview\"))",
            "ui.heading(t!(\"page.logs\"))",
            "ui.heading(t!(\"page.docs\"))",
            "ui.heading(t!(\"page.resources\"))",
            "ui.heading(t!(\"task.queue\"))",
            "ui.heading(t!(\"it.title\"))",
        ] {
            assert!(!APP.contains(pat), "app.rs 仍残留重复标题块: {}", pat);
        }
        // file-browser 臂：模块名 heading + 简介 weak 整块删（这两个键整个源文件里只在该块用过）
        assert!(
            !APP.contains("t!(\"mod.file_browser.name\")"),
            "file-browser 标题块未删干净"
        );
        assert!(
            !APP.contains("t!(\"mod.file_browser.desc\")"),
            "file-browser 简介块未删干净"
        );
        // 另 3 处（其它文件）
        assert!(
            !CHAT.contains("ui.heading(t!(\"chat.title\""),
            "chat_view 标题块仍在"
        );
        assert!(
            !MREG.contains("t!(\"mreg.title\")"),
            "model_registry 标题块仍在"
        );
        assert!(
            !UPGRADE.contains("ui.heading(t!(\"upgrade.title\")"),
            "upgrade 标题块仍在"
        );
    }

    /// §4.4 保留项 / 功能控件仍在（防误删）。
    #[test]
    fn retained_headings_and_controls_still_present() {
        assert!(
            APP.contains("t!(\"cocoon.platform\")"),
            "虫茧平台标题被误删（§4.4 要求保留）"
        );
        assert!(
            APP.contains("t!(\"resources.mcp\")"),
            "资源库「3 库切换」工具条被误删"
        );
        assert!(
            UPGRADE.contains("upgrade.btn_check"),
            "升级「检查更新」按钮被误删"
        );
        assert!(
            MREG.contains("mreg.refresh"),
            "模型登记库「刷新」按钮被误删"
        );
        // 内容区统一顶部呼吸（E19——一处，不在每页自己写）；needle 用 concat! 拼以免命中本断言自身
        let needle = concat!("// 设计 §4.4", "/E19");
        assert_eq!(APP.matches(needle).count(), 1, "内容区呼吸应恰好一处");
    }

    fn key_lines(yml: &str) -> std::collections::BTreeSet<String> {
        yml.lines()
            .filter(|l| !l.starts_with('#') && !l.trim().is_empty())
            .filter_map(|l| {
                let k = l.split_once(':')?.0.trim();
                if k.is_empty() || k.contains(' ') {
                    None
                } else {
                    Some(k.to_string())
                }
            })
            .collect()
    }

    /// 设计 §4.6/§七10：新增 8 键 zh-CN/en **两侧齐全**，且全键集合相等。
    #[test]
    fn i18n_new_keys_symmetric_and_key_sets_equal() {
        let new_keys = [
            "status.light_tip",
            "mod.main_online.name",
            "mod.main_online.desc",
            "mod.tasks_group.name",
            "mod.tasks_group.desc",
            "mod.models_group.name",
            "mod.models_group.desc",
            "nav.no_submodules",
        ];
        let zh = key_lines(ZH_YML);
        let en = key_lines(EN_YML);
        for k in new_keys {
            assert!(zh.contains(k), "locales/zh-CN.yml 缺新键 {}", k);
            assert!(en.contains(k), "locales/en.yml 缺新键 {}", k);
        }
        assert_eq!(
            zh, en,
            "zh-CN 与 en 的键集合必须完全一致（少一个键某语言就露出键名）"
        );
    }
}

#[cfg(test)]
mod cocoon_platform_tests {
    //! C9 第 3 步（2026-09-13）：平台页「未安装 ⇒ 提示安装」**两态** + 安装指引弹窗（能失败的检查）。

    use super::{cocoon_install_hint_window, cocoon_openable};
    use crate::modules::cocoon;

    const APP: &str = include_str!("app.rs");

    /// **两态**：可进入性由 `cocoon::openable` 决定 —— C9 第 4 步宿主**不再**内建渲染文档界面
    /// （已整块迁进茧）⇒ 文档茧与示例虫茧**同一判据**（纯 `loaded`），没有「宿主内建回退」。
    #[test]
    fn platform_card_two_states() {
        // ① 文档茧：可进 = 它自己的装载态（本机默认构建＝已装载；公开镜像＝未装载 ⇒ 打不开 + 给指引）
        let docs = cocoon::meta_of("docs").expect("文档茧在册");
        assert_eq!(
            cocoon_openable("docs"),
            docs.loaded,
            "未装载 ⇒ 打不开（宿主已不内建渲染文档界面——第 4 步回归纯 loaded 判据）"
        );
        assert_eq!(
            cocoon::install_guide("docs").is_some(),
            !docs.loaded,
            "未装载 ⇒ 给得出安装指引（指向独立仓）；已装载 ⇒ 不给指引"
        );
        // ② 宿主自己的页 / 非茧 id 照旧可进（不降级既有行为）
        assert!(cocoon_openable("chat"), "宿主自己的页恒可进入");
        assert!(
            cocoon_openable("not-a-cocoon-id"),
            "非茧 id 恒可（走主仓自己的渲染臂）"
        );
        // ③ 已装载的示例虫茧照旧——按铭牌判定，不写死
        let rt = cocoon::meta_of("roundtable").expect("示例虫茧在册");
        assert_eq!(cocoon_openable("roundtable"), rt.loaded);
        // ④ 反例：已装载的茧必须可开（防「一律不可开」的假实现）
        assert!(cocoon::openable(&cocoon::CocoonMeta {
            loaded: true,
            ..cocoon::ROUNDTABLE
        }));
        assert!(!cocoon::openable(&cocoon::CocoonMeta {
            loaded: false,
            ..cocoon::ROUNDTABLE
        }));
    }

    /// 安装指引弹窗：未装载的茧 ⇒ **画得出来**（不 panic、不空白，返回「未关闭」）；
    /// 不是茧 / 未知 id ⇒ 不弹空窗（返回「可关」）。
    #[test]
    fn install_hint_window_renders_for_uninstalled_only() {
        let ctx = egui::Context::default();
        for id in ["docs", "roundtable"] {
            if cocoon::install_guide(id).is_none() {
                continue; // 该构建已装载 ⇒ 本条不适用（两态都对）
            }
            let mut closed = true;
            let mut out = ctx.run_ui(Default::default(), |ui| {
                closed = cocoon_install_hint_window(ui, id);
            });
            out.textures_delta.clear();
            assert!(!closed, "{}：安装指引必须画出来且不自行关闭", id);
        }
        for id in ["chat", "not-a-cocoon-id"] {
            let mut closed = false;
            let mut out = ctx.run_ui(Default::default(), |ui| {
                closed = cocoon_install_hint_window(ui, id);
            });
            out.textures_delta.clear();
            assert!(closed, "{} 不是茧 ⇒ 不该弹安装指引（免得空白窗）", id);
        }
    }

    /// 源码级守线（能失败）：卡片上的「安装」按钮 + 弹窗接线必须在（防误删）。
    #[test]
    fn platform_wires_the_install_affordance() {
        assert!(
            APP.contains("t!(\"cocoon.install\")"),
            "卡片上的「安装」按钮被删了"
        );
        assert!(
            APP.contains("cocoon::install_guide(id)"),
            "「安装」按钮没接指引数据源"
        );
        assert!(
            APP.contains("cocoon_install_hint_window"),
            "安装指引窗口被删了"
        );
        assert!(
            APP.contains("self.render_cocoon_install_hint(ui)"),
            "平台页没画安装指引"
        );
    }
}
