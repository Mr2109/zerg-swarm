// 虫族 UI 主程序（eframe::App——三栏布局——任务视图——离线全灰）
// 异步: 自己管理（tokio spawn + SharedResult——不用 egui-async——eframe 0.36 兼容问题）
use eframe::egui;
use rust_i18n::t;

use crate::api::{self, TaskInfo};
use crate::modules::icons::icon_text; // P3 图标（iconflow）

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
    docs: Arc<Mutex<Option<(Vec<String>, Vec<String>)>>>, // v2.5.6 (files, dirs)——目录树+文件列表
    // 文档视图状态（三栏——Mr2109 2026-08-29: 目录树|文件|正文）
    doc_dir: String, // 第一栏选中的目录（如 "项目文档/v2.5.6"）
    doc_file: String,
    doc_content: Arc<Mutex<Option<String>>>,
    // APP-A20（2026-09-10 审计）: 正文拉取失败文案——原来失败写 None，第三栏只剩
    // 永久 spinner（无法区分「在读」与「读失败」）。失败时置此字段，渲染成红字。
    doc_content_err: Arc<Mutex<Option<String>>>,
    // v2.5.6 md 编辑器（Mr2109 2026-08-29: 第三栏=md 编辑器——编辑/预览/保存）
    // APP-A15（2026-09-10 审计）: 删除死状态 `doc_edit`——全文件只有声明/初始化/一处赋值
    // （「切到编辑模式时同步缓冲」），没有任何读取点，等于空操作；真实编辑缓冲是
    // Ferrite 编辑器 rope（ferrite_editor）。删除原因：留着会让维护者以为还有一层同步逻辑。
    doc_edit_mode: bool,    // true=编辑模式 / false=预览模式
    doc_edit_dirty: bool,   // 有未保存修改
    doc_md_cache: egui_commonmark::CommonMarkCache, // markdown 渲染缓存
    // M3 Ferrite 重写编辑器（Mr2109 2026-08-29——替换 TextEdit——rope 缓冲）
    ferrite_editor: crate::modules::ferrite::MdEditor,
    ferrite_loaded: bool, // 是否已载入当前文件到编辑器（切文件重置）
    // M06(2026-09-10 审计): 编辑器全量文本缓存——(epoch, text)。每帧 text() 是 Rope→String 全量克隆，
    // 大文档直接拖垮帧率；按 cache_epoch 失效，仅在内容变更后重建。
    ferrite_text_cache: Option<(u64, String)>,
    // 2026-09-10 审计 APP-A02: 文档操作(save/delete/rename/mkdir/copy)结果回报——
    // 先确认成功再改本地状态;失败红字提示且不动状态(reject-before-persist)
    doc_op_result: api::SharedResult<()>,
    doc_op_ctx: Option<(String, String)>, // (kind, path)——成功后据此改本地状态
    doc_op_err: Option<String>,           // 失败提示(下次成功时清除)
    // F4 滚动同步（编辑→预览单向——防反馈环）
    preview_sync_line: usize,     // 上次同步的编辑滚动行
    preview_content_h: f32,       // 预览内容高度（上次渲染）
    preview_last_offset: f32,     // 预览当前滚动位置（用户手动滚动保持）
    // APP-A15（2026-09-10 审计）: 删除死状态 `preview_syncing`——全文件只被赋值、
    // 没有任何读取点（原注释承诺的「防覆盖」从未参与判断，滚动同步实际只靠
    // preview_sync_line 比较）。删除原因：留着会让维护者以为还有一层保护。
    // F5 AI 动力（Mr2109统一接口——网关 8082）
    ai_busy: bool,                // AI 调用中
    ai_status: String,            // AI 状态提示（busy 时显示）
    ai_pending: Option<(String, api::SharedResult<String>)>, // (动作, 异步结果) 待处理
    ai_output: Option<(String, String)>, // (动作, 结果) 完成显示
    // v2.5.7 对话模块（Mr2109——完全借鉴 Hermes——第一板块）
    chat_view: crate::modules::chat::ChatView,
    // v2.5.6 文档右键操作（Mr2109 2026-08-29）
    doc_clipboard: Option<String>, // 复制缓冲（复制的文件路径）
    doc_input: Option<(String, String, String)>, // 输入对话框 (标题, 当前值, 动作令牌)——令牌用于逻辑判断(不依赖文案语言)
    // APP-A09: 输入缓冲提升到 self（原来每帧从初值重建局部变量 → 打不进字、提交的是打开时的旧值）
    doc_input_buf: String, // 编辑中的输入内容
    doc_input_new: bool,   // 刚打开——仅首帧 request_focus（防每帧抢焦点/断中文 IME）
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
    adapter_edit: std::sync::Arc<std::sync::Mutex<std::collections::HashMap<String, std::collections::HashMap<String, serde_json::Value>>>>, // v2.5.6 适配器编辑缓冲（模型→键值）
    adapter_msg: String, // v2.5.6 适配器应用结果提示
    adapter_confirm: bool, // v2.5.6 适配器修改确认态（Mr2109 2026-08-27——点应用后确认才生效）
    split_model: f32, // v2.5.6 模型库左右分割比例（可拖拽——持久化）
    split_it: f32, // v2.5.6 内部任务左右分割比例（可拖拽——持久化）
    split_docs1: f32, // v2.5.6 文档三栏: 分类栏宽度比例（可拖拽——持久化）
    split_docs2: f32, // v2.5.6 文档三栏: 文件栏宽度比例（可拖拽——持久化）
    adapter_schema: std::sync::Arc<std::sync::Mutex<Option<serde_json::Value>>>, // v2.5.6 当前模型适配器 schema（编辑控件渲染）
    // 触发计时
    last_ping: f64,
    last_tasks: f64,
    last_detail: f64,
    last_git: f64,
    last_logs: f64,
    last_docs: f64,
    last_res: f64,
    last_cluster: f64,
    // 导航（v2.5.6 集装箱注册表——顶部导航——Mr2109 2026-08-29）
    registry: crate::modules::ModuleRegistry,
    // v2.5.6 模块管理面板开关（➕ 吊装系统——M2）
    show_module_manager: bool,
    // T8 虫茧集装箱（示例虫茧——懒加载——点虫茧首次建——切走引擎后台继续 M2）
    // 2026-09-11 B 批（决策 5）：示例虫茧为独立仓（zerg-cocoon）的**可选**集装箱——
    // 公开快照不启用该 feature（不编译、不链接）；未启用时平台栅格显示"未装载"。
    #[cfg(feature = "zerg-roundtable")]
    roundtable: Option<Box<zerg_roundtable::ui::RoundtableApp>>,
    // T8 虫茧平台态（false=平台启动器应用栅格；true=示例虫茧全屏）
    rt_active: bool,
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
    // M06 双渲染器（2026-09-10 Mr2109：两种都保留，含切换）——ferrite=自研样式 / commonmark=带缓存，选择持久化
    preview_renderer_cm: bool,
    preview_cm_cache: egui_commonmark::CommonMarkCache,
    engine_state: api::SharedResult<serde_json::Value>,
    last_engine_fetch: std::time::Instant,
    it_ctrl_busy: Option<bool>, // 请求在飞（按钮显示“切换中…”）
    // APP-A07: 启停结果回报——成功才刷新状态；失败红字提示且不改显示
    it_ctrl_result: api::SharedResult<()>,
    it_ctrl_target: Option<bool>, // 待生效的目标值（后端成功返回后套用）
    it_ctrl_confirm: bool,        // 二次确认态（防误点启停内部任务引擎）
    it_selected: Option<String>, // v2.5.6 内部任务选中（左右布局——右侧详情+skill）
    it_intervals: std::sync::Arc<std::sync::Mutex<Option<serde_json::Value>>>, // v2.5.6 周期配置缓存
    it_custom_for: Option<String>, // v2.5.6 正在指定周期（任务 id）
    it_custom_hours: String, // v2.5.6 自定义周期输入
    // 归档列表（Mr2109 2026-08-22）
    archive: std::sync::Arc<std::sync::Mutex<Option<Vec<serde_json::Value>>>>,
    last_archive_fetch: std::time::Instant,
}

impl ZergApp {
    pub fn new() -> Self {
        Self {
            online: false,
            hud_hidden: false,
            locale: "zh-CN".to_string(),
            tasks: Arc::new(Mutex::new(None)),
            tasks_loaded: false,
            online_result: Arc::new(Mutex::new(None)),
            poll_err: Arc::new(Mutex::new(None)),
            task_detail: Arc::new(Mutex::new(None)),
            git_status: Arc::new(Mutex::new(None)),
            logs: Arc::new(Mutex::new(None)),
            docs: Arc::new(Mutex::new(None)),
            doc_dir: String::new(), // 空 = 首次拉到目录树后自动选第一个（环境无关化——不再硬编码某台机器的中文目录名）
            doc_file: String::new(),
            doc_content: Arc::new(Mutex::new(None)),
            doc_content_err: Arc::new(Mutex::new(None)),
            doc_edit_mode: false, // 默认预览模式
            doc_edit_dirty: false,
            ferrite_editor: crate::modules::ferrite::MdEditor::new(),
            ferrite_loaded: false,
            ferrite_text_cache: None,
            doc_op_result: Arc::new(Mutex::new(None)),
            doc_op_ctx: None,
            doc_op_err: None,
            preview_sync_line: 0,
            preview_content_h: 0.0,
            preview_last_offset: 0.0,
            ai_busy: false,
            ai_status: String::new(),
            ai_pending: None,
            ai_output: None,
            chat_view: crate::modules::chat::ChatView::new(),
            doc_md_cache: egui_commonmark::CommonMarkCache::default(),
            doc_clipboard: None,
            doc_input: None,
            doc_input_buf: String::new(),
            doc_input_new: false,
            resources: Arc::new(Mutex::new(None)),
            cluster: Arc::new(Mutex::new(None)),
            res_type: "tools".to_string(), // 资源库默认工具库（模型库已独立板块——2026-08-27）
            res_sort_col: 0,
            res_sort_desc: false,
            selected_model: None,
            selected_model_desc: String::new(),
            model_detail: std::sync::Arc::new(std::sync::Mutex::new(None)),
            model_tab: "detail".to_string(),
            adapter_edit: std::sync::Arc::new(std::sync::Mutex::new(std::collections::HashMap::new())),
            adapter_msg: String::new(),
            adapter_confirm: false,
            adapter_schema: std::sync::Arc::new(std::sync::Mutex::new(None)),
            split_model: load_layout_ratio("split_model", 0.32), // v2.5.6 布局持久化（Mr2109——拖动后下次默认）
            split_it: load_layout_ratio("split_it", 0.36),
            split_docs1: load_layout_ratio("split_docs1", 0.22),
            split_docs2: load_layout_ratio("split_docs2", 0.35),
            last_ping: 0.0,
            last_tasks: 0.0,
            last_detail: 0.0,
            last_git: 0.0,
            last_logs: 0.0,
            last_docs: 0.0,
            last_res: 0.0,
            last_cluster: 0.0,
            registry: {
                let mut r = crate::modules::build_registry();
                r.load(); // v2.5.6 恢复持久化状态（哪些箱卸下了）
                r
            },
            show_module_manager: false,
            #[cfg(feature = "zerg-roundtable")]
            roundtable: None,
            rt_active: false,
            selected_task: None,
            detail_id: String::new(),
            last_refresh: 0.0,
            internal_tasks: std::sync::Arc::new(std::sync::Mutex::new(None)),
            last_it_fetch: std::time::Instant::now(),
            last_it_interval_fetch: std::time::Instant::now(),
            // M06 双渲染器（2026-09-10 Mr2109：两种都保留）：选择持久化，重启后保持
            preview_renderer_cm: Self::load_preview_pref().or_else(|| std::env::var("ZERG_PREVIEW_RENDERER").ok().map(|v| v == "commonmark")).unwrap_or(false),
            preview_cm_cache: Default::default(),
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
        // 文档操作结果(APP-A02 2026-09-10 审计)——成功才改本地状态;失败只提示、不改状态
        if let Some(r) = lock_recover(&self.doc_op_result).take() {
            let ctx = self.doc_op_ctx.take();
            match r {
                Ok(()) => {
                    if let Some((kind, path)) = ctx {
                        match kind.as_str() {
                            "del_dir" => {
                                if self.doc_dir == path {
                                    self.doc_dir = String::new(); // 选中目录被删 → 回到"未选"，下一帧自动选第一个
                                }
                            }
                            "del_file" => {
                                if self.doc_file == path {
                                    self.doc_file = String::new();
                                    *lock_recover(&self.doc_content) = None;
                                }
                            }
                            "save" => {
                                self.doc_edit_dirty = false;
                                self.doc_edit_mode = false; // 保存成功才回预览
                            }
                            _ => {}
                        }
                    }
                    self.doc_op_err = None;
                }
                Err(e) => self.doc_op_err = Some(e),
            }
        }
        // 丙批 N4（2026-09-10）：前缀缓存命中率轮询（网关 8082——30s）
        if lock_recover(&self.prefix_cache).is_none() || self.last_pc_fetch.elapsed().as_secs() >= 30 {
            let store = self.prefix_cache.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                let r = api::fetch_prefix_cache_blocking().await;
                *lock_recover(&store) = Some(r);
            });
            self.last_pc_fetch = now;
        }
        // A07 治本（2026-09-10）：引擎状态轮询（10s）——服务端为唯一真相源
        if lock_recover(&self.engine_state).is_none() || self.last_engine_fetch.elapsed().as_secs() >= 10 {
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
                    self.last_engine_fetch = std::time::Instant::now() - std::time::Duration::from_secs(60);
                    *lock_recover(&self.poll_err) = None;
                }
                Err(e) => {
                    *lock_recover(&self.poll_err) = Some(t!("err.internal_tasks", err = e).to_string());
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
        if !self.tasks_loaded {
            if lock_recover(&self.tasks).is_some() {
                self.tasks_loaded = true;
            }
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
        // 拉文档目录（5s——v2.5.6 实时显示变动：Mr2109 2026-08-29 之前 30s 太慢——文档改动等半分钟）
        if self.online && now - self.last_docs > 5.0 {
            self.last_docs = now;
            let store = self.docs.clone();
            let perr = self.poll_err.clone();
            api::runtime().spawn(async move {
                // APP-A04: 失败保留旧值
                match api::fetch_docs_blocking().await {
                    Ok(v) => {
                        *lock_recover(&store) = Some(v);
                        *lock_recover(&perr) = None;
                    }
                    Err(e) => *lock_recover(&perr) = Some(t!("err.docs", err = e).to_string()),
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

    /// F5 AI 动力——发起 AI 操作（总结/续写/翻译/润色——网关 8082）
    fn ai_run(&mut self, action: &str) {
        // 取文本：编辑器优先，未 load（预览模式）则从文档缓存取
        let text = if self.ferrite_loaded {
            self.ferrite_editor.text()
        } else {
            lock_recover(&self.doc_content).clone().unwrap_or_default()
        };
        if text.trim().is_empty() {
            self.ai_output = Some(("error".to_string(), t!("ai.empty_doc").to_string()));
            return;
        }
        // AI 模型解析（环境无关化 2026-09-11）：ZERG_AI_MODEL → ~/.zerg-ui-prefs.json 的 ai_model
        // 原先硬编码私有模型名（外部用户没有该模型 → 四个 AI 动作必然失败）；缺配置时明确告知怎么配。
        let model = std::env::var("ZERG_AI_MODEL")
            .ok()
            .filter(|m| !m.trim().is_empty())
            .or_else(Self::load_ai_model_pref)
            .unwrap_or_default();
        if model.is_empty() {
            self.ai_output = Some(("error".to_string(), t!("ai.no_model").to_string()));
            return;
        }
        // 安全截断（按 char 边界——中文 3 字节/字不能切中间）
        let mut clip_len = text.len().min(6000);
        while !text.is_char_boundary(clip_len) {
            clip_len -= 1;
        }
        let clip = &text[..clip_len];
        let prompt = match action {
            "summarize" => format!("请用中文总结以下 Markdown 文档的核心内容，输出简洁要点列表：\n\n{}", clip),
            "continue" => format!("请用中文续写以下 Markdown 文档，保持原有风格，直接输出续写内容（不要重复已有内容）：\n\n{}", clip),
            "translate" => format!("请将以下 Markdown 文档翻译成中文，保持 Markdown 格式（标题/列表/代码块原样）：\n\n{}", clip),
            "polish" => format!("请润色以下 Markdown 文档，改善表达但不改变原意和结构，输出润色后的完整文档：\n\n{}", clip),
            _ => return,
        };
        self.ai_busy = true;
        self.ai_status = t!("ai.running", action = action).to_string();
        let result: api::SharedResult<String> = Arc::new(Mutex::new(None));
        let r2 = result.clone();
        api::runtime().spawn(async move {
            let res = api::ai_prompt_blocking(&model, &prompt).await;
            *lock_recover(&r2) = Some(res);
        });
        self.ai_pending = Some((action.to_string(), result));
    }

    /// F5 AI 结果轮询（每帧检查 pending 是否完成）
    fn ai_poll(&mut self) {
        if let Some((action, result)) = self.ai_pending.clone() {
            if let Some(res) = lock_recover(&result).clone() {
                self.ai_pending = None;
                self.ai_busy = false;
                match res {
                    Ok(text) => {
                        self.ai_output = Some((action, text));
                        self.ai_status = String::new();
                    }
                    Err(e) => {
                        // 失败弹窗提示（不能静默）
                        self.ai_output = Some(("error".to_string(), t!("ai.call_failed", err = e).to_string()));
                        self.ai_status = String::new();
                    }
                }
            }
        }
    }

    /// APP-A10（2026-09-10 审计）: 确保 Ferrite 已载入当前文件——预览模式下做 AI 插入/替换前调用。
    /// 原来 rope 里只有 AI 那段，进编辑模式时的 `if !ferrite_loaded { load(文件内容) }` 会把它覆盖掉。
    fn ensure_editor_loaded(&mut self) {
        if !self.ferrite_loaded {
            let cur = lock_recover(&self.doc_content).clone().unwrap_or_default();
            self.ferrite_editor.load(&cur);
            self.ferrite_text_cache = None;
            self.ferrite_loaded = true;
        }
    }

    /// APP-A10: 预览模式渲染的是 doc_content（编辑模式渲染 rope）——AI 结果同步过去才看得见
    fn sync_preview_after_ai(&mut self) {
        if !self.doc_edit_mode {
            let txt = self.ferrite_editor.text();
            *lock_recover(&self.doc_content) = Some(txt);
            self.ferrite_text_cache = None;
        }
    }

    /// F5 AI 结果弹窗（总结/翻译/续写结果——可插入/替换/复制）
    fn ai_result_view(&mut self, ctx: &egui::Context) {
        if let Some((action, text)) = self.ai_output.clone() {
            let title = match action.as_str() {
                "summarize" => t!("ai.summarize", icon = icon_text("sparkles")).to_string(),
                "continue" => t!("ai.continue", icon = icon_text("pencil-simple")).to_string(),
                "translate" => t!("ai.translate", icon = icon_text("translate")).to_string(),
                "polish" => t!("ai.polish", icon = icon_text("sparkles")).to_string(),
                "error" => t!("ai.error_title", icon = icon_text("warning")).to_string(),
                _ => t!("ai.result", icon = icon_text("sparkles")).to_string(),
            };
            // 窗口占界面 70% 高度（Mr2109 2026-08-29——太高挡内容）
            let screen_rect = ctx.viewport_rect();
            let win_h = (screen_rect.height() * 0.7).max(240.0);
            let win_w = (screen_rect.width() * 0.6).clamp(420.0, 900.0);
            egui::Window::new(title)
                .collapsible(false)
                .resizable(true)
                .default_size([win_w, win_h])
                .max_height(win_h)
                .show(ctx, |ui| {
                    egui::ScrollArea::vertical().auto_shrink(false).show(ui, |ui| {
                        ui.label(&text); // 借用——避免 move（后面按钮还要用）
                    });
                    ui.separator();
                    ui.horizontal(|ui| {
                        if action == "error" {
                            if ui.button(t!("action.close_icon", icon = icon_text("x-circle"))).clicked() {
                                self.ai_output = None;
                            }
                            return;
                        }
                        if ui.button(t!("ai.insert_at_end")).clicked() {
                            // APP-A10: 编辑器未载入（预览模式点的）→ 先载入当前文档再追加，
                            // 否则结果既看不见、进编辑模式时又会被文件内容 load 覆盖掉
                            self.ensure_editor_loaded();
                            self.ferrite_editor.append_text(&format!("\n\n{}", text));
                            self.doc_edit_dirty = true;
                            self.sync_preview_after_ai();
                            self.ai_output = None;
                        }
                        if ui.button(t!("ai.replace_all", icon = icon_text("note-pencil"))).clicked() {
                            // APP-A10: 同上——预览模式渲染的是 doc_content，结果同步过去才看得见
                            self.ensure_editor_loaded();
                            self.ferrite_editor.load(&text);
                            self.doc_edit_dirty = true;
                            self.sync_preview_after_ai();
                            self.ai_output = None;
                        }
                        if ui.button(t!("action.close_icon", icon = icon_text("x-circle"))).clicked() {
                            self.ai_output = None;
                        }
                    });
                });
        }
    }

    /// 渲染主视图（v2.5.6——内容区由集装箱注册表分发）
    /// 渲染任务视图（主区——队列 + 详情各占一半——水平布局）
    fn tasks_view(&mut self, ui: &mut egui::Ui) {
        ui.heading(t!("task.queue"));
        ui.add_space(4.0);
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
                    Err(e) => *lock_recover(&perr) = Some(t!("task.detail_err", err = e).to_string()),
                }
            });
        }
        // APP-A13（2026-09-10 审计）: 详情头部每帧用最新列表回查（原来用点击那一刻的快照——
        // 任务 running→done 后详情仍显示旧状态/旧机器/旧时长，与左栏自相矛盾）
        if !self.detail_id.is_empty() {
            if let Some(fresh) = list.iter().find(|t| t.id.as_deref() == Some(self.detail_id.as_str())) {
                self.selected_task = Some(fresh.clone());
            }
        }
        // 归档折叠区（Mr2109 2026-08-22——30 天归档/90 天删除可查）
        // APP-A19（2026-09-10 审计）: ① 整块从下方 `if list.is_empty() { … return; }` 之后
        // 挪到之前——队列清空（任务全删/全归档）时「归档」恰恰是最有用的面板，原来直接消失；
        // ② 拉取失败不再伪装成「暂无归档」（原来 .unwrap_or_default() 把 Err 变成空数组）——
        // 失败保留旧值 + 顶部提示条 + 面板内区分「加载中/拉取失败」。
        if lock_recover(&self.archive).is_none() || self.last_archive_fetch.elapsed().as_secs() > 60 {
            let store = self.archive.clone();
            let perr = self.poll_err.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                match api::fetch_archive_blocking().await {
                    Ok(items) => *lock_recover(&store) = Some(items),
                    Err(e) => *lock_recover(&perr) = Some(t!("task.archive_err", err = e).to_string()),
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
                            let tid = a.get("task_id").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                            let when = a.get("archived").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                            let size = a.get("size").and_then(|v| v.as_i64()).unwrap_or(0);
                            // APP-A16: 尾 6 字符/前 16 字符都按「字符」切（原来按字节下标——
                            // 多字节 task_id/时间串会切在字符中间 panic，整个 UI 退出）
                            ui.label(t!("task.archive_row", id = short_id(&tid), when = when.chars().take(16).collect::<String>(), kb = size / 1024));
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
            egui::ScrollArea::vertical().id_salt("queue_scroll").max_height(avail_h).show(&mut cols[0], |ui| {
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
                        !matches!(s, "running" | "queued" | "paused" | "done" | "failed" | "waiting_retry")
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
                    .filter(|t| t.ref_task_id.as_deref().map(|r| !r.is_empty()).unwrap_or(false))
                    .map(|t| (*t).clone())
                    .collect();
                let done_main: Vec<&TaskInfo> = done
                    .iter()
                    .filter(|t| t.ref_task_id.as_deref().map(|r| r.is_empty()).unwrap_or(true))
                    .map(|t| *t)
                    .collect();

                let running_count = running.len();
                egui::CollapsingHeader::new(format!("🔄 {}（{}）", t!("task.state_running"), running_count))
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
                egui::CollapsingHeader::new(format!("⏳ {}（{}）", t!("task.state_queued"), queued_count))
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
                let mut by_date: std::collections::BTreeMap<String, Vec<&TaskInfo>> = Default::default();
                for t in &done_main {
                    // APP-A16: 取日期同样按字符切（原 `s[..10]` 是字节下标——含多字节的时间串会 panic）
                    let date = t.completed_at.as_deref().map(|s| s.chars().take(10).collect::<String>()).unwrap_or_else(|| t!("common.unknown_date").to_string());
                    by_date.entry(date).or_default().push(t);
                }
                egui::CollapsingHeader::new(format!("✅ {}（{}）", t!("task.state_done"), done_count))
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
                            egui::CollapsingHeader::new(format!("📅 {}（{}）", date, day_tasks.len()))
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
            egui::ScrollArea::vertical().id_salt("detail_scroll").max_height(avail_h2).show(&mut cols[1], |ui| {
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
        let is_review = t.ref_task_id.as_deref().map(|r| !r.is_empty()).unwrap_or(false);
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
                    Err(e) => *lock_recover(&perr) = Some(t!("task.detail_err", err = e).to_string()),
                }
            });
        }
        // 右键菜单（Mr2109 2026-08-21: 复制/重跑/置顶置底/上移下移/删除）
        let task_id_owned = t.id.clone().unwrap_or_default();
        let status_owned = status.to_string();
        resp.context_menu(|ui| {
            // 复制任务描述
            if ui.button(t!("task.copy_desc", icon = icon_text("copy"))).clicked() {
                if let Some(desc) = t.description.clone() {
                    ui.ctx().copy_text(desc);
                }
                ui.close();
            }
            // 重跑（failed 任务）
            if status == "failed" && ui.button(t!("task.op_retry")).clicked() {
                let id = task_id_owned.clone();
                // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                self.task_op(async move { api::task_retry_blocking(&id).await }, t!("task.op_retry").as_ref());
                ui.close();
            }
            // 重回排队（done/failed 任务——重新入队——Mr2109 2026-08-21）
            if (status == "done" || status == "failed") && ui.button(t!("task.back_to_queue")).clicked() {
                let id = task_id_owned.clone();
                // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                self.task_op(async move { api::task_retry_blocking(&id).await }, t!("task.back_to_queue").as_ref());
                ui.close();
            }
            // 暂停/继续（queued 任务——Mr2109 2026-08-21）
            if status_owned == "queued" {
                if ui.button(t!("task.pause", icon = icon_text("pause"))).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_pause_blocking(&id, true).await }, t!("task.op_pause").as_ref());
                    ui.close();
                }
                if ui.button(t!("task.resume", icon = icon_text("play"))).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_pause_blocking(&id, false).await }, t!("task.op_resume").as_ref());
                    ui.close();
                }
                ui.separator();
                if ui.button(t!("task.pin_top")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_move_blocking(&id, "top").await }, t!("task.op_pin_top").as_ref());
                    ui.close();
                }
                if ui.button(t!("task.pin_bottom")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_move_blocking(&id, "bottom").await }, t!("task.op_pin_bottom").as_ref());
                    ui.close();
                }
                if ui.button(t!("task.move_up")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_move_blocking(&id, "up").await }, t!("task.op_move_up").as_ref());
                    ui.close();
                }
                if ui.button(t!("task.move_down")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_move_blocking(&id, "down").await }, t!("task.op_move_down").as_ref());
                    ui.close();
                }
                if ui.button(format!("{} {}", icon_text("trash"), t!("action.delete"))).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_delete_blocking(&id).await }, t!("task.op_delete").as_ref());
                    ui.close();
                }
            }
            // 执行中任务操作（running——终止/重回队列——Mr2109 2026-08-22）
            if status_owned == "running" {
                if ui.button(t!("task.terminate")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_terminate_blocking(&id).await }, t!("task.op_terminate").as_ref());
                    ui.close();
                }
                if ui.button(t!("task.requeue")).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_requeue_blocking(&id).await }, t!("task.op_requeue").as_ref());
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
            ui.label(format!("{}: {}", t!("task.desc"), t.description.as_deref().unwrap_or("?")));
            ui.label(format!("{}: {}", t!("task.type"), t.task_type.as_deref().unwrap_or("?")));
            ui.label(format!("{}: {}", t!("task.priority"), t.priority.unwrap_or(0)));
            ui.label(format!("{}: {}", t!("task.status"), t.status.as_deref().unwrap_or("?")));
            ui.label(format!("{}: {}", t!("task.model"), t.model.as_deref().unwrap_or("?")));
            ui.label(format!("{}: {}", t!("task.machine"), t.machine.as_deref().unwrap_or("?")));
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
                            let dur = round
                                .get("duration")
                                .and_then(|d| d.as_str())
                                .unwrap_or("");
                            let header = if dur.is_empty() {
                                format!("[{}{}] {} {}", t!("task.round"), rn, t!("task.tools"), tools)
                            } else {
                                // Mr2109 2026-08-21: 耗时排在工具调用后面
                                format!("[{}{}] {} {} | ⏱ {}", t!("task.round"), rn, t!("task.tools"), tools, dur)
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
                if detail.get("subtask_mode").and_then(|v| v.as_bool()).unwrap_or(false) {
                    if let Some(stages) = detail.get("stages").and_then(|v| v.as_array()) {
                        ui.label(t!("task.stage_crystallize"));
                        for s in stages {
                            let id = s.get("id").and_then(|v| v.as_str()).unwrap_or("?");
                            let goal = s.get("goal").and_then(|v| v.as_str()).unwrap_or("");
                            let status = s.get("status").and_then(|v| v.as_str()).unwrap_or("pending");
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
    fn module_manager_view(&mut self, ctx: &egui::Context) {
        let mut close = false;
        egui::Window::new(t!("modules.title"))
            .collapsible(false)
            .resizable(true)
            .default_size([420.0, 380.0])
            .show(ctx, |ui| {
                ui.label(t!("modules.core_section"));
                ui.separator();
                let mut changed = false;
                for m in self.registry.modules.iter().filter(|m| m.is_core) {
                    ui.horizontal(|ui| {
                        ui.label(format!("{} {}", m.icon, t!(m.name_key)));
                        ui.weak(t!(m.desc_key));
                        ui.label("🔒");
                    });
                }
                ui.add_space(8.0);
                ui.label(t!("modules.loadable_section"));
                ui.separator();
                // 先收集切换请求（避免迭代中改 registry——借用冲突）
                let mut to_toggle: Option<String> = None;
                for m in self.registry.modules.iter().filter(|m| !m.is_core) {
                    let on = self.registry.enabled.get(m.id).copied().unwrap_or(true);
                    let mut next = on;
                    ui.horizontal(|ui| {
                        if ui.checkbox(&mut next, format!("{} {}", m.icon, t!(m.name_key))).changed() {
                            if next != on {
                                to_toggle = Some(m.id.to_string());
                            }
                        }
                        ui.weak(t!(m.desc_key));
                    });
                }
                if let Some(id) = to_toggle {
                    // 切换——卸下时若正在查看该箱——回任务队列
                    self.registry.toggle(&id);
                    if !self.registry.enabled.get(&id).copied().unwrap_or(true) && self.registry.active == id {
                        self.registry.active = "tasks".to_string();
                    }
                    changed = true;
                }
                // M4 生态箱（外部模块——配置文件声明——第三方开发者挂船）
                if !self.registry.external.is_empty() {
                    ui.add_space(8.0);
                    ui.label(t!("modules.ext_section"));
                    ui.separator();
                    let mut ext_toggle: Option<String> = None;
                    let ext_list = self.registry.external.clone();
                    for m in &ext_list {
                        let on = self.registry.enabled.get(&m.id).copied().unwrap_or(true);
                        let mut next = on;
                        ui.horizontal(|ui| {
                            if ui.checkbox(&mut next, format!("{} {}", m.icon, m.name)).changed() {
                                if next != on {
                                    ext_toggle = Some(m.id.clone());
                                }
                            }
                            ui.weak(&m.description);
                        });
                    }
                    if let Some(id) = ext_toggle {
                        self.registry.toggle(&id);
                        if !self.registry.enabled.get(&id).copied().unwrap_or(true) && self.registry.active == id {
                            self.registry.active = "tasks".to_string();
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

    /// 渲染主区（v2.5.6——集装箱注册表分发——Mr2109 2026-08-29）

    /// M06(2026-09-10 审计): 取编辑器全量文本（带 epoch 缓存——避免每帧 Rope→String 克隆）
    fn ferrite_text_cached(&mut self) -> String {
        let epoch = self.ferrite_editor.epoch();
        if let Some((e, t)) = &self.ferrite_text_cache {
            if *e == epoch {
                return t.clone();
            }
        }
        let t = self.ferrite_editor.text();
        self.ferrite_text_cache = Some((epoch, t.clone()));
        t
    }

    fn main_view(&mut self, ui: &mut egui::Ui) {
        match self.registry.active.as_str() {
            "chat" => {
                // v2.5.7 对话模块（Mr2109——完全借鉴 Hermes——第一板块）
                self.chat_view.render(ui);
            }
            "tasks" => self.tasks_view(ui),
            "internal-tasks" => self.internal_tasks_view(ui),
            "cluster" => {
                ui.heading(t!("cluster.status"));
                ui.add_space(4.0);
                if let Some(r) = lock_recover(&self.cluster).clone() {
                    egui::ScrollArea::vertical().show(ui, |ui| {
                        // 总览
                        let healthy = r.get("healthy_count").and_then(|h| h.as_u64()).unwrap_or(0);
                        let total = r.get("total_machines").and_then(|h| h.as_u64()).unwrap_or(0);
                        let active = r.get("total_active_requests").and_then(|h| h.as_u64()).unwrap_or(0);
                        ui.label(t!("cluster.health_summary", healthy = healthy, total = total, active = active));
                        ui.separator();
                        // 机器列表（dict——local/mini1/x3）
                        if let Some(machines) = r.get("machines").and_then(|m| m.as_object()) {
                            for (name, mv) in machines {
                                let model = mv.get("model").and_then(|n| n.as_str()).unwrap_or("—");
                                let mem_avail = mv.get("mem_available_gb").and_then(|n| n.as_f64()).unwrap_or(0.0);
                                let mem_total = mv.get("mem_total_gb").and_then(|n| n.as_f64()).unwrap_or(0.0);
                                let load = mv.get("load").and_then(|n| n.as_f64()).unwrap_or(0.0);
                                let gpu = mv.get("gpu_used_gb").and_then(|n| n.as_f64()).unwrap_or(0.0);
                                let healthy_m = mv.get("healthy").and_then(|n| n.as_bool()).unwrap_or(true);
                                let color = if healthy_m { egui::Color32::from_rgb(80, 200, 120) } else { egui::Color32::from_rgb(220, 80, 80) };
                                ui.label(egui::RichText::new(format!("📊 {}", name)).color(color));
                                ui.indent(name, |ui| {
                                    ui.label(t!("cluster.machine_model", model = model));
                                    ui.label(t!("cluster.machine_mem", avail = format!("{:.0}", mem_avail), total = format!("{:.0}", mem_total)));
                                    ui.label(t!("cluster.machine_load", load = format!("{:.2}", load), gpu = format!("{:.1}", gpu)));
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
                ui.heading(t!("git.overview"));
                ui.add_space(4.0);
                if let Some(g) = lock_recover(&self.git_status).clone() {
                    // 分支（通俗化——task-xxx → 任务类型名）
                    egui::CollapsingHeader::new(format!("🌿 {}（{}）", t!("git.branches"), g.branches.as_ref().map(|b| b.len()).unwrap_or(0)))
                        .id_salt("git_branches") // APP-A08: 稳定 id
                        .show(ui, |ui| {
                        if let Some(branches) = &g.branches {
                            for b in branches {
                                ui.label(format!("  {}", humanize_branch(b)));
                            }
                        }
                    });
                    // worktree（通俗化——只显示分支名——不显示完整路径）
                    egui::CollapsingHeader::new(format!("📂 {}（{}）", t!("git.worktrees"), g.worktrees.as_ref().map(|w| w.len()).unwrap_or(0)))
                        .id_salt("git_worktrees") // APP-A08: 稳定 id
                        .show(ui, |ui| {
                        if let Some(wts) = &g.worktrees {
                            for wt in wts {
                                let branch = wt.get("branch").and_then(|b| b.as_str()).unwrap_or("?");
                                ui.label(format!("  🔀 {}", humanize_branch(branch)));
                            }
                        }
                    });
                    // 未 merge（待脑检查）
                    egui::CollapsingHeader::new(
                        format!(
                            "{} {}（{}）",
                            icon_text("warning"),
                            t!("git.unmerged"),
                            g.unmerged.as_ref().map(|u| u.len()).unwrap_or(0)
                        ),
                    )
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
                ui.heading(t!("page.logs"));
                ui.add_space(4.0);
                if let Some(lines) = lock_recover(&self.logs).clone() {
                    egui::ScrollArea::vertical().auto_shrink([false, false]).show(ui, |ui| {
                        for l in lines {
                            let ll = l.to_ascii_lowercase();   // 日志级别判定：大小写不敏感 + 中英双认（去掉对单一语言的依赖）
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
                #[cfg(not(feature = "zerg-roundtable"))]
                {
                    // 未装载该集装箱：永远停在平台栅格（不进入不存在的视图）
                    self.rt_active = false;
                }
                if !self.rt_active {
                    // ── 平台界面：应用栅格（无数茧——每个=独立集装箱应用——示例虫茧=第一个）──
                    ui.heading(format!("{} {}", icon_text("boxes"), t!("cocoon.platform")));
                    ui.weak(t!("cocoon.platform_hint"));
                    ui.add_space(10.0);
                    // 应用清单（平台雏形——未来读集装箱注册/目录扫描——现静态声明可扩展）
                    // 结构：每卡=独立 git 集装箱应用（id/名字/描述/打开）
                    let cards: [(String, String, String); 1] = [(
                        "roundtable".into(),
                        t!("cocoon.roundtable.name").to_string(),
                        t!("cocoon.roundtable.desc").to_string(),
                    )];
                    // 卡片网格（wrap 布局——每卡固定宽 260）
                    egui::ScrollArea::vertical().auto_shrink([false, false]).show(ui, |ui| {
                        egui::Grid::new("cocoon_grid").num_columns(3).spacing([14.0, 14.0]).show(ui, |ui| {
                            for (id, name, desc) in &cards {
                                let (rect, _) = ui.allocate_exact_size(egui::vec2(260.0, 132.0), egui::Sense::click());
                                let hover = rect.contains(ui.ctx().pointer_interact_pos().unwrap_or_default());
                                let (fill, stroke) = if hover {
                                    (egui::Color32::from_rgb(42, 46, 56), egui::Stroke::new(1.0, egui::Color32::from_rgb(90, 110, 170)))
                                } else {
                                    (egui::Color32::from_rgb(32, 35, 43), egui::Stroke::new(1.0, egui::Color32::from_rgb(55, 60, 70)))
                                };
                                ui.painter().rect_filled(rect, 10.0, fill);
                                ui.painter().rect_stroke(rect, 10.0, stroke, egui::StrokeKind::Inside);
                                let mut card_ui = ui.new_child(
                                    egui::UiBuilder::new()
                                        .max_rect(egui::Rect::from_min_max(rect.min + egui::vec2(14.0, 12.0), rect.max - egui::vec2(14.0, 12.0)))
                                        .layout(egui::Layout::top_down(egui::Align::Min)),
                                );
                                card_ui.label(egui::RichText::new(format!("📖 {name}")).size(16.0).strong());
                                card_ui.add_space(6.0);
                                card_ui.label(egui::RichText::new(desc.as_str()).size(12.0).color(egui::Color32::from_rgb(170, 175, 185)));
                                card_ui.with_layout(egui::Layout::right_to_left(egui::Align::BOTTOM), |ui| {
                                    if ui.button(egui::RichText::new(t!("action.open")).size(12.0)).clicked() {
                                        // 2026-09-11 B 批（决策 5）：集装箱需编译时装载（feature）
                                        if cfg!(feature = "zerg-roundtable") {
                                            self.rt_active = true;
                                        }
                                    }
                                });
                                if !cfg!(feature = "zerg-roundtable") {
                                    card_ui.label(
                                        egui::RichText::new(t!("cocoon.not_loaded"))
                                            .size(11.0)
                                            .weak(),
                                    );
                                }
                                if card_ui.rect_contains_pointer(rect)
                                    && ui.ctx().input(|i| i.pointer.any_click())
                                    && cfg!(feature = "zerg-roundtable")
                                {
                                    self.rt_active = true;
                                }
                                ui.allocate_exact_size(egui::vec2(0.0, 0.0), egui::Sense::hover());
                                ui.end_row();
                            }
                        });
                    });
                }
                // ── 示例虫茧全屏（嵌中央区——面包屑一层——切走引擎后台继续 M2）──
                // 三层收一层（2026-09-04）：宿主不再画返回条——示例虫茧面包屑自带"← 虫茧平台"
                // 2026-09-11 B 批（决策 5）：整块随 feature 编译——未启用时不存在该视图
                #[cfg(feature = "zerg-roundtable")]
                if self.rt_active {
                    if self.roundtable.is_none() {
                        self.roundtable = Some(Box::new(zerg_roundtable::ui::RoundtableApp::new()));
                    }
                    let rt = self.roundtable.as_mut().unwrap();
                    rt.embedded = true;
                    rt.render(ui);
                    // 面包屑"← 虫茧平台"点击请求 → 退出回平台栅格（引擎后台继续 M2）
                    if rt.exit_platform {
                        self.rt_active = false;
                    }
                }
            }
            "docs" => {
                ui.heading(t!("page.docs"));
                ui.add_space(4.0);
                let docs_snap = lock_recover(&self.docs).clone(); // 先释放借用——内部闭包要 &mut self（F5 AI 按钮）
                if let Some((files, dirs)) = docs_snap {
                    // 环境无关化（2026-09-11）：未选目录时自动选第一个可用目录（原先硬编码 "00-总览"，外部用户没有该目录）
                    if self.doc_dir.is_empty() {
                        if let Some(first) = dirs.first() {
                            self.doc_dir = first.clone();
                        }
                    }
                    // v2.5.6 三栏（Mr2109 2026-08-29）: 第一栏=目录树（dirs）| 第二栏=选中目录文件 | 第三栏=正文
                    // 目录树——层级缩进（如 "项目文档" 顶层 / "项目文档/v2.5.6" 子级缩进）
                    let total_w = ui.available_width();
                    let avail_h = ui.available_height().max(200.0);
                    let col1_w = (total_w * self.split_docs1).clamp(140.0, 320.0);
                    let col2_w = (total_w * self.split_docs2).clamp(160.0, total_w - col1_w - 420.0);
                    let mut drag1 = false;
                    let mut drag2 = false;
                    ui.allocate_ui_with_layout(
                        egui::vec2(total_w, avail_h),
                        egui::Layout::left_to_right(egui::Align::TOP),
                        |ui| {
                        // 栏1——目录树
                        let (c1, _) = ui.allocate_exact_size(egui::vec2(col1_w, avail_h), egui::Sense::hover());
                        let mut c1_ui = ui.new_child(egui::UiBuilder::new().max_rect(c1).layout(egui::Layout::top_down(egui::Align::Min)));
                        egui::ScrollArea::vertical().id_salt("docs_col1").auto_shrink(false).show(&mut c1_ui, |ui| {
                            ui.heading(t!("docs.tree"));
                            ui.add_space(4.0);
                            for dir in &dirs {
                                let depth = dir.split('/').count() - 1; // 子目录缩进
                                // v2.5.6 只显示目录名（不含父路径前缀——Mr2109: 项目文档/v2.5.6 显示为 v2.5.6）
                                let dir_name = dir.split('/').last().unwrap_or(dir);
                                let label = if depth == 0 {
                                    format!("📁 {}", dir_name)
                                } else {
                                    format!("{}└ 📁 {}", "  ".repeat(depth), dir_name)
                                };
                                let resp = ui.selectable_label(self.doc_dir == *dir, label);
                                // v2.5.6 目录右键（Mr2109 2026-08-29: 重命名/删除/新建子目录）
                                resp.context_menu(|ui| {
                                    if ui.button(t!("action.rename")).clicked() {
                                        self.doc_input = Some((t!("docs.rename_dir").to_string(), dir.clone(), "rename_dir".to_string()));
                                        self.doc_input_buf = dir.clone(); // APP-A09
                                        self.doc_input_new = true;
                                        ui.close();
                                    }
                                    if ui.button(format!("{} {}", icon_text("trash"), t!("action.delete"))).clicked() {
                                        let path = dir.clone();
                                        // APP-A02: 先确认成功再改本地状态(原实现丢结果 + 立即切目录)
                                        self.doc_op_ctx = Some(("del_dir".to_string(), path.clone()));
                                        self.doc_op_result = api::doc_op_async("delete", serde_json::json!({"path": path}));
                                        ui.close();
                                    }
                                    if ui.button(format!("{} {}", icon_text("folder-plus"), t!("docs.new_subdir"))).clicked() {
                                        let base = dir.clone();
                                        self.doc_input = Some((t!("docs.new_subdir").to_string(), format!("{}/", base), "new_subdir".to_string()));
                                        self.doc_input_buf = format!("{}/", base); // APP-A09
                                        self.doc_input_new = true;
                                        ui.close();
                                    }
                                });
                                if resp.clicked() {
                                    self.doc_dir = dir.clone();
                                    self.doc_file = String::new();
                                    self.doc_edit_dirty = false;
                                    self.ferrite_loaded = false; // M3 切目录重置编辑器
                                    *lock_recover(&self.doc_content) = None;
                                }
                            }
                            // v2.5.6 空白右键——新建目录（Mr2109 2026-08-29）
                            ui.add_space(4.0);
                            if ui.button(format!("{} {}", icon_text("folder-plus"), t!("docs.new_dir"))).clicked() {
                                self.doc_input = Some((t!("docs.new_dir").to_string(), String::new(), "new_dir".to_string()));
                                self.doc_input_buf = String::new(); // APP-A09
                                self.doc_input_new = true;
                            }
                        });
                        // 拖拽条1
                        let (d1, dr1) = ui.allocate_exact_size(egui::vec2(8.0, avail_h), egui::Sense::drag());
                        ui.painter().rect_filled(d1, 0.0, ui.visuals().faint_bg_color);
                        ui.painter().vline(d1.center().x, d1.y_range(), egui::Stroke::new(1.0, ui.visuals().weak_text_color()));
                        let _ = dr1.clone().on_hover_cursor(egui::CursorIcon::ResizeHorizontal);
                        if dr1.dragged() {
                            let dx = ui.input(|i| i.pointer.delta().x);
                            self.split_docs1 = (self.split_docs1 + dx / total_w).clamp(0.12, 0.35);
                        }
                        // APP-A12: 拖动中只改内存——松手那一帧才落盘（原来每帧同步读写 JSON，拖 UI 卡顿）
                        if dr1.drag_stopped() {
                            drag1 = true;
                        }
                        // 栏2——选中目录的文件（直接子文件——不含更深子目录）
                        let (c2, _) = ui.allocate_exact_size(egui::vec2(col2_w, avail_h), egui::Sense::hover());
                        let mut c2_ui = ui.new_child(egui::UiBuilder::new().max_rect(c2).layout(egui::Layout::top_down(egui::Align::Min)));
                        egui::ScrollArea::vertical().id_salt("docs_col2").auto_shrink(false).show(&mut c2_ui, |ui| {
                            ui.heading(format!("📄 {}", self.doc_dir));
                            ui.add_space(4.0);
                            let dir_prefix = format!("{}/", self.doc_dir);
                            let dir_files: Vec<&String> = files
                                .iter()
                                .filter(|f| f.starts_with(&dir_prefix))
                                .filter(|f| {
                                    // 直接子文件（去掉前缀后不含更深的 /）
                                    let rest = &f[dir_prefix.len()..];
                                    !rest.contains('/')
                                })
                                .collect();
                            for f in &dir_files {
                                let fname = f.split('/').last().unwrap_or(f);
                                let selected = self.doc_file == f.as_str();
                                let resp = ui.selectable_label(selected, format!("📄 {}", fname));
                                // v2.5.6 文件右键（Mr2109 2026-08-29: 重命名/复制/粘贴/删除）
                                resp.context_menu(|ui| {
                                    if ui.button(t!("action.rename")).clicked() {
                                        self.doc_input = Some((t!("docs.rename_file").to_string(), f.to_string(), "rename_file".to_string()));
                                        self.doc_input_buf = f.to_string(); // APP-A09
                                        self.doc_input_new = true;
                                        ui.close();
                                    }
                                    if ui.button(format!("{} {}", icon_text("copy"), t!("action.copy"))).clicked() {
                                        self.doc_clipboard = Some(f.to_string());
                                        ui.close();
                                    }
                                    if let Some(src) = self.doc_clipboard.clone() {
                                        if ui.button(format!("{} {}", icon_text("clipboard"), t!("docs.paste_here"))).clicked() {
                                            // 目标 = 当前目录 + 源文件名（冲突加副本后缀）
                                            let fname_src = src.split('/').last().unwrap_or(&src).to_string();
                                            let target = format!("{}/{}", self.doc_dir, fname_src);
                                            // APP-A02: 粘贴(复制)也走结果回报——失败可见
                                            self.doc_op_ctx = Some(("copy".to_string(), String::new()));
                                            self.doc_op_result = api::doc_op_async("copy", serde_json::json!({"from": src, "to": target}));
                                            ui.close();
                                        }
                                    }
                                    if ui.button(format!("{} {}", icon_text("trash"), t!("action.delete"))).clicked() {
                                        let path = f.to_string();
                                        // APP-A02: 成功才清空选中/内容
                                        self.doc_op_ctx = Some(("del_file".to_string(), path.clone()));
                                        self.doc_op_result = api::doc_op_async("delete", serde_json::json!({"path": path}));
                                        ui.close();
                                    }
                                });
                                if resp.clicked() {
                                    self.doc_file = f.to_string();
                                    self.doc_edit_dirty = false;
                                    self.ferrite_loaded = false; // M3 切文件重置编辑器
                                    // APP-A20（2026-09-10 审计）: 切文件立即清空旧正文+旧错误——
                                    // 新内容到达前不再短暂显示上一个文件的正文；
                                    // 拉取失败也不写 None 了事（原来第三栏永远转圈、无法判断读失败），
                                    // 失败文案落到 doc_content_err 由第三栏红字渲染。
                                    let path = f.to_string();
                                    let store = self.doc_content.clone();
                                    let err = self.doc_content_err.clone();
                                    *lock_recover(&self.doc_content) = None;
                                    *lock_recover(&self.doc_content_err) = None;
                                    api::runtime().spawn(async move {
                                        match api::fetch_doc_content_blocking(path).await {
                                            Ok(txt) => {
                                                *lock_recover(&store) = Some(txt);
                                                *lock_recover(&err) = None;
                                            }
                                            Err(e) => {
                                                *lock_recover(&store) = None;
                                                *lock_recover(&err) = Some(t!("docs.read_failed", err = e).to_string());
                                            }
                                        }
                                    });
                                }
                            }
                            if dir_files.is_empty() {
                                ui.weak(t!("common.none"));
                            }
                        });
                        // 拖拽条2
                        let (d2, dr2) = ui.allocate_exact_size(egui::vec2(8.0, avail_h), egui::Sense::drag());
                        ui.painter().rect_filled(d2, 0.0, ui.visuals().faint_bg_color);
                        ui.painter().vline(d2.center().x, d2.y_range(), egui::Stroke::new(1.0, ui.visuals().weak_text_color()));
                        let _ = dr2.clone().on_hover_cursor(egui::CursorIcon::ResizeHorizontal);
                        if dr2.dragged() {
                            let dx = ui.input(|i| i.pointer.delta().x);
                            self.split_docs2 = (self.split_docs2 + dx / total_w).clamp(0.15, 0.45);
                        }
                        // APP-A12: 松手才落盘
                        if dr2.drag_stopped() {
                            drag2 = true;
                        }
                        // 栏3——文件内容
                        let c3_w = (ui.available_width() - 8.0).max(300.0);
                        let (c3, _) = ui.allocate_exact_size(egui::vec2(c3_w, avail_h), egui::Sense::hover());
                        let mut c3_ui = ui.new_child(egui::UiBuilder::new().max_rect(c3).layout(egui::Layout::top_down(egui::Align::Min)));
                        if self.doc_file.is_empty() {
                            c3_ui.weak(t!("docs.pick_hint"));
                        } else {
                            c3_ui.horizontal(|ui| {
                                ui.label(format!("📄 {}", self.doc_file.split('/').last().unwrap_or("")));
                                // v2.5.6 md 编辑器工具栏（Mr2109 2026-08-29）
                                if ui.button(if self.doc_edit_mode { t!("docs.preview").to_string() } else { format!("{} {}", icon_text("note-pencil"), t!("action.edit")) }).clicked() {
                                    // APP-A15: 原来这里还有一段「切到编辑时同步 doc_edit 缓冲」——
                                    // doc_edit 已证明是只写不读的死状态（且 rope 缓冲由下方
                                    // `if !ferrite_loaded { load(文件内容) }` 负责），整段删除。
                                    self.doc_edit_mode = !self.doc_edit_mode;
                                }
                                // APP-A02: 文档操作失败红字提示（不改本地状态）
                                let doc_err = self.doc_op_err.clone();
                                if let Some(e) = doc_err {
                                    ui.colored_label(
                                        egui::Color32::from_rgb(230, 90, 90),
                                        format!("⚠ {}", e.chars().take(80).collect::<String>()),
                                    );
                                }
                                if self.doc_edit_dirty {
                                    if ui.button(format!("{} {}", icon_text("floppy-disk"), t!("action.save"))).clicked() {
                                        // 保存——调 /api/docs/save（M3: 取 Ferrite 编辑器文本）
                                        let path = self.doc_file.clone();
                                        // APP-A01 修复(2026-09-10): 编辑器从未载入(预览模式 AI 插入等)时，
                                        // 绝不能用空/不完整缓冲覆盖整份文档——回落到当前文档内容
                                        let content = if self.ferrite_loaded {
                                            self.ferrite_editor.text()
                                        } else {
                                            lock_recover(&self.doc_content).clone().unwrap_or_default()
                                        };
                                        // APP-A02: 保存成功才清 dirty/回预览；失败红字提示
                                        self.doc_op_ctx = Some(("save".to_string(), path.clone()));
                                        self.doc_op_result = api::doc_op_async("save", serde_json::json!({"path": path, "content": content}));
                                    }
                                }
                                // F5 AI 动力（Mr2109统一接口——网关 8082——虫族版编辑器本质特征）
                                if !self.ai_busy {
                                    if ui.button(t!("docs.ai_summary")).clicked() {
                                        self.ai_run("summarize");
                                    }
                                    if ui.button(format!("{} {}", icon_text("pencil-simple"), t!("docs.ai_continue"))).clicked() {
                                        self.ai_run("continue");
                                    }
                                    if ui.button(format!("{} {}", icon_text("translate"), t!("docs.ai_translate"))).clicked() {
                                        self.ai_run("translate");
                                    }
                                    if ui.button(t!("docs.ai_polish")).clicked() {
                                        self.ai_run("polish");
                                    }
                                } else {
                                    ui.spinner();
                                    ui.weak(&self.ai_status);
                                }
                            });
                            c3_ui.separator();
                            if self.doc_edit_mode {
                                // 编辑模式——M3 Ferrite 重写编辑器 split 并排（左编辑 + 右 comrak 实时预览）
                                let avail_h3 = c3_ui.available_height().max(200.0);
                                // 首次进入编辑——载入内容到 rope 缓冲
                                if !self.ferrite_loaded {
                                    if let Some(content) = lock_recover(&self.doc_content).clone() {
                                        self.ferrite_editor.load(&content);
                                    }
                                    self.ferrite_text_cache = None; // M06: 载入新内容 → 失效
                                    self.ferrite_loaded = true;
                                }
                                // F4 大纲条（标题横向——点击跳转）
                                let edit_text = self.ferrite_text_cached(); // M06: 带缓存
                                let toc_entries = crate::modules::ferrite::toc::parse_toc(&edit_text);
                                if !toc_entries.is_empty() {
                                    c3_ui.horizontal(|ui| {
                                        ui.weak(t!("docs.outline"));
                                        let mut jump: Option<usize> = None;
                                        egui::ScrollArea::horizontal()
                                            .id_salt("ferrite_toc")
                                            .auto_shrink(false)
                                            .max_height(24.0)
                                            .show(ui, |ui| {
                                                for e in &toc_entries {
                                                    let label = match e.level {
                                                        1 => format!("{} {}", "#", e.text),
                                                        2 => format!("{} {}", "##", e.text),
                                                        _ => format!("{} {}", "###", e.text),
                                                    };
                                                    if ui
                                                        .selectable_label(
                                                            e.line == self.ferrite_editor.cursor_line(),
                                                            egui::RichText::new(label).size(12.0),
                                                        )
                                                        .clicked()
                                                    {
                                                        jump = Some(e.line);
                                                    }
                                                    ui.separator();
                                                }
                                            });
                                        if let Some(line) = jump {
                                            self.ferrite_editor.jump_to_line(line);
                                        }
                                    });
                                    c3_ui.separator();
                                }
                                // split: 左编辑 55% + 右预览 45%
                                let total_w = c3_ui.available_width();
                                let edit_w = total_w * 0.55;
                                let preview_w = total_w - edit_w;
                                c3_ui.allocate_ui_with_layout(
                                    egui::vec2(total_w, avail_h3),
                                    egui::Layout::left_to_right(egui::Align::TOP),
                                    |ui| {
                                        // 左——编辑器
                                        let (erect, _) = ui.allocate_exact_size(
                                            egui::vec2(edit_w, avail_h3),
                                            egui::Sense::hover(),
                                        );
                                        let mut e_ui = ui.new_child(
                                            egui::UiBuilder::new()
                                                .max_rect(erect)
                                                .layout(egui::Layout::top_down(egui::Align::Min)),
                                        );
                                        self.ferrite_editor.render(&mut e_ui);
                                        if self.ferrite_editor.dirty {
                                            self.doc_edit_dirty = true;
                                            self.ferrite_editor.dirty = false;
                                        }
                                        // 右——实时预览（comrak + F4 滚动同步 编辑→预览单向）
                                        let (prect, _) = ui.allocate_exact_size(
                                            egui::vec2(preview_w, avail_h3),
                                            egui::Sense::hover(),
                                        );
                                        let mut p_ui = ui.new_child(
                                            egui::UiBuilder::new()
                                                .max_rect(prect)
                                                .layout(egui::Layout::top_down(egui::Align::Min)),
                                        );
                                        p_ui.separator();
                                        // M06 双渲染器（2026-09-10 Mr2109：两种都保留，含切换）
                                        // ferrite=自研样式（每帧 comrak 解析）；commonmark=带缓存（大文档省一半，且支持图片/公式/任务列表）
                                        let mut flip = false;
                                        p_ui.horizontal(|ui| {
                                            let cur = if self.preview_renderer_cm { t!("docs.renderer_cm").to_string() } else { t!("docs.renderer_ferrite").to_string() };
                                            if ui.small_button(t!("docs.renderer_switch", cur = cur)).clicked() {
                                                flip = true;
                                            }
                                            ui.weak(t!("docs.renderer_remember"));
                                        });
                                        if flip {
                                            self.preview_renderer_cm = !self.preview_renderer_cm;
                                            Self::save_preview_pref(self.preview_renderer_cm);
                                        }
                                        let preview_text = self.ferrite_text_cached(); // M06: 带缓存
                                        // F4 滚动同步：编辑 scroll_line 变化 → 预览跟随（比例换算）
                                        let line_count = self.ferrite_editor.line_count().max(1);
                                        let editor_line = self.ferrite_editor.scroll_line();
                                        let sync_target = if editor_line != self.preview_sync_line {
                                            self.preview_sync_line = editor_line;
                                            let ratio = editor_line as f32 / line_count as f32;
                                            let viewport = avail_h3;
                                            (ratio * (self.preview_content_h - viewport)).max(0.0)
                                        } else {
                                            self.preview_last_offset
                                        };
                                        // 借用规则：缓存先取出为局部变量，闭包内不再触碰 self（egui 嵌套闭包不可再借 &mut self）
                                        let use_cm = self.preview_renderer_cm;
                                        let mut cm_cache = std::mem::take(&mut self.preview_cm_cache);
                                        let out = egui::ScrollArea::vertical()
                                            .id_salt("ferrite_preview")
                                            .auto_shrink(false)
                                            .vertical_scroll_offset(sync_target)
                                            .show(&mut p_ui, |ui| {
                                                if use_cm {
                                                    // 能力对齐：带公式渲染（与对话/消息同款 KaTeX→SVG），否则切过去公式退化为纯文本
                                                    egui_commonmark::CommonMarkViewer::new()
                                                        .render_math_fn(Some(&crate::modules::chat::chat_view::render_math))
                                                        .show(ui, &mut cm_cache, &preview_text);
                                                } else {
                                                    crate::modules::ferrite::markdown::render_markdown(ui, &preview_text);
                                                }
                                            });
                                        self.preview_cm_cache = cm_cache;
                                        self.preview_content_h = out.content_size.y;
                                        self.preview_last_offset = out.state.offset.y;
                                    },
                                );
                            } else if let Some(content) = lock_recover(&self.doc_content).clone() {
                                egui::ScrollArea::vertical().id_salt("docs_col3").auto_shrink(false).show(&mut c3_ui, |ui| {
                                    // v2.5.6 md 渲染（egui_commonmark CommonMarkViewer——支持标题/列表/代码块/表格）
                                    egui_commonmark::CommonMarkViewer::new().show(ui, &mut self.doc_md_cache, &content);
                                });
                            } else if let Some(e) = lock_recover(&self.doc_content_err).clone() {
                                // APP-A20: 拉取失败——红字报错（原来只写 None，这里永远转圈）
                                c3_ui.add_space(8.0);
                                c3_ui.colored_label(
                                    egui::Color32::from_rgb(230, 90, 90),
                                    format!("⚠ {}", e),
                                );
                                if c3_ui.button(t!("action.reload")).clicked() {
                                    let path = self.doc_file.clone();
                                    let store = self.doc_content.clone();
                                    let err = self.doc_content_err.clone();
                                    *lock_recover(&self.doc_content_err) = None;
                                    api::runtime().spawn(async move {
                                        match api::fetch_doc_content_blocking(path).await {
                                            Ok(txt) => {
                                                *lock_recover(&store) = Some(txt);
                                                *lock_recover(&err) = None;
                                            }
                                            Err(e2) => {
                                                *lock_recover(&store) = None;
                                                *lock_recover(&err) = Some(t!("docs.read_failed", err = e2).to_string());
                                            }
                                        }
                                    });
                                }
                            } else {
                                c3_ui.spinner();
                                c3_ui.weak(t!("common.loading"));
                            }
                        }
                        });
                    if drag1 {
                        self.save_layout_ratio("split_docs1", self.split_docs1);
                    }
                    if drag2 {
                        self.save_layout_ratio("split_docs2", self.split_docs2);
                    }
                    // v2.5.6 输入对话框（重命名/新建目录——Mr2109 2026-08-29）
                    // APP-A09: 输入内容存 self.doc_input_buf（原来每帧从打开时的初值重建局部变量——
                    // 用户输入只活在当帧：打不进字、点确定提交的还是旧值；request_focus 也改成仅首帧）
                    if let Some((title, current, action)) = self.doc_input.clone() {
                        let mut new_val = std::mem::take(&mut self.doc_input_buf);
                        let mut needs_focus = self.doc_input_new;
                        let mut close = false;
                        let mut do_submit = false;
                        egui::Window::new(title.as_str())
                            .collapsible(false)
                            .resizable(false)
                            .show(ui.ctx(), |ui| {
                                ui.horizontal(|ui| {
                                    ui.label(t!("common.name_label"));
                                    let resp = ui.text_edit_singleline(&mut new_val);
                                    if needs_focus {
                                        resp.request_focus(); // 仅首帧——原来每帧抢焦点（中文 IME 打不进）
                                        needs_focus = false;
                                    }
                                    if ui.button(t!("action.ok")).clicked() {
                                        do_submit = true;
                                    }
                                    if ui.button(t!("action.cancel")).clicked() {
                                        close = true;
                                    }
                                });
                                if ui.input(|i| i.key_pressed(egui::Key::Enter)) {
                                    do_submit = true;
                                }
                            });
                        self.doc_input_buf = new_val; // 回写——下一帧继续编辑
                        self.doc_input_new = false;
                        if do_submit {
                            // 提交——根据标题判断操作类型
                            let v = self.doc_input_buf.trim().to_string();
                            let current_path = current.clone();
                            if !v.is_empty() {
                                // 动作判定用 ASCII 令牌（原实现按标题文案 contains —— 语言一改即失效；
                                // 且 "新建子目录" 不含 "新建目录" → 子目录提交曾经什么都不发生，此处一并修复）
                                if action == "rename_dir" || action == "rename_file" {
                                    // old=当前完整路径, new=父目录+v（目录）或 v（文件）
                                    let parent = current_path.rfind('/').map(|i| current_path[..i].to_string()).unwrap_or_default();
                                    let new_path = if parent.is_empty() { v.clone() } else { format!("{}/{}", parent, v) };
                                    self.doc_op_ctx = Some(("rename".to_string(), String::new()));
                                    self.doc_op_result = api::doc_op_async("rename", serde_json::json!({"old": current_path, "new": new_path}));
                                } else if action == "new_dir" || action == "new_subdir" {
                                    let new_path = if current_path.ends_with('/') {
                                        format!("{}{}", current_path, v)
                                    } else if current_path.is_empty() {
                                        v.clone()
                                    } else {
                                        format!("{}/{}", current_path, v)
                                    };
                                    self.doc_op_ctx = Some(("mkdir".to_string(), String::new()));
                                    self.doc_op_result = api::doc_op_async("mkdir", serde_json::json!({"dir": new_path}));
                                }
                            }
                            close = true;
                        }
                        if close {
                            self.doc_input = None;
                            self.doc_input_buf.clear();
                        }
                    }
                } else {
                    ui.spinner();
                    ui.weak(t!("common.loading"));
                }
            }
            "models" => {
                // 模型库独立板块（Mr2109 2026-08-27——排资源库上面）
                if self.res_type != "models" {
                    self.res_type = "models".to_string();
                    let store = self.resources.clone();
                    api::runtime().spawn(async move {
                        let r = api::fetch_resources_blocking("models".to_string()).await.ok();
                        *lock_recover(&store) = r;
                    });
                }
                ui.heading(format!("{} {}", icon_text("computer-tower"), t!("page.model_library")));
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
                        let r = api::fetch_resources_blocking("tools".to_string()).await.ok();
                        *lock_recover(&store) = r;
                    });
                }
                ui.heading(t!("page.resources"));
                ui.add_space(4.0);
                // 3 库切换（模型库已独立板块——Mr2109 2026-08-27）
                ui.horizontal(|ui| {
                    let types = [(t!("resources.tools").to_string(), "tools"), (t!("resources.skills").to_string(), "skills"), (t!("resources.mcp").to_string(), "mcp")];
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
                            egui::ScrollArea::vertical().id_salt("res_list").show(&mut cols[0], |ui| {
                                // 模型库——按设备分组 + 字母排序（Mr2109 2026-08-20）
                                if self.res_type == "models" {
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
                                            .id_salt(format!("res_models_{}", machine)) // APP-A08: 稳定 id
                                            // APP-A21（2026-09-10 审计）: 默认展开——原来默认为折叠，
                                            // 进「资源库」只看到设备分组标题、看不到任何模型，容易以为没模型
                                            .default_open(true)
                                            .show(ui, |ui| {
                                            // 设备内按字母排序（固定——刷新不变——Mr2109）
                                            let mut sorted = names.clone();
                                            sorted.sort();
                                            for n in &sorted {
                                                let selected = self.selected_model.as_deref() == Some(n.as_str());
                                                let trust = descs.get(n).cloned().unwrap_or_default();
                                                // 资源信任度标记（🆕 新模型——2026-08-21 Mr2109）
                                                let trust_mark = if trust.contains("🆕") { " 🆕" } else { "" };
                                                // 模型名前面不加图标（Mr2109 2026-08-31——干净列表）
                                                if ui.selectable_label(selected, format!("  {}{}", n, trust_mark)).clicked() {
                                                    self.selected_model = Some(n.clone());
                                                    self.selected_model_desc = descs.get(n).cloned().unwrap_or_default();
                                                }
                                            }
                                        });
                                    }
                                } else {
                                    // 工具/skill/mcp——表格（Mr2109 2026-08-21: 表格形式——状态/次数/故障/时间）
                                    let mut rows: Vec<(String, String, String, i64, i64, String, String)> = items
                                        .iter()
                                        .filter_map(|item| {
                                            let name = item.get("name").and_then(|n| n.as_str())?.to_string();
                                            let version = item.get("version").and_then(|v| v.as_str()).unwrap_or("v1.0.0").to_string();
                                            let trust = item.get("trust").and_then(|t| t.as_str()).unwrap_or("未知").to_string();
                                            let uses = item.get("uses").and_then(|u| u.as_i64()).unwrap_or(0);
                                            let faults = item.get("faults").and_then(|f| f.as_i64()).unwrap_or(0);
                                            let since = item.get("since").and_then(|s| s.as_str()).unwrap_or("-").to_string();
                                            let desc = item.get("desc").and_then(|d| d.as_str()).unwrap_or("-").to_string();
                                            Some((name, version, trust, uses, faults, since, desc))
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
                                        if self.res_sort_desc { ord.reverse() } else { ord }
                                    });
                                    egui::Grid::new("res_table").striped(true).show(ui, |ui| {
                                        // 列头（点击排序——Mr2109 2026-08-21）
                                        let headers = [(t!("resources.col_name").to_string(), 0), (t!("resources.col_version").to_string(), 1),
                                            (t!("resources.col_state").to_string(), 2), (t!("resources.col_calls").to_string(), 3),
                                            (t!("resources.col_faults").to_string(), 4), (t!("resources.col_added").to_string(), 5),
                                            (t!("resources.col_desc").to_string(), 6)];
                                        for (label, col) in headers {
                                            let arrow = if self.res_sort_col == col { if self.res_sort_desc { " ▼" } else { " ▲" } } else { "" };
                                            if ui.button(format!("{}{}", label, arrow)).clicked() {
                                                if self.res_sort_col == col {
                                                    self.res_sort_desc = !self.res_sort_desc;
                                                } else {
                                                    self.res_sort_col = col;
                                                    self.res_sort_desc = false;
                                                }
                                            }
                                        }
                                        ui.end_row();
                                        for (name, version, trust, uses, faults, since, desc) in &rows {
                                            // P4-49 工具名后不加图标（Mr2109 UI 偏好——与模型名一致）
                                            ui.label(name.clone());
                                            ui.label(version.clone());
                                            ui.label(if trust == "新" { t!("resources.trust_new").to_string() } else if trust == "正式" { t!("resources.trust_official").to_string() } else if trust == "未知" || trust.is_empty() { t!("common.unknown").to_string() } else { trust.clone() });
                                            ui.label(t!("resources.uses_count", n = uses));
                                            let fault_color = if *faults > 0 { egui::Color32::from_rgb(255, 80, 80) } else { egui::Color32::GRAY };
                                            ui.colored_label(fault_color, format!("{}", faults));
                                            ui.label(since.clone());
                                            ui.label(desc.clone());
                                            ui.end_row();
                                        }
                                    });
                                }
                            });
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
                let active_id = self.registry.active.clone();
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
                            if !m.url.is_empty() {
                                if ui.button(t!("ext.open")).clicked() {
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
                                        *lock_recover(&self.poll_err) = Some(t!("ext.bad_url", url = url).to_string());
                                    }
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
                                                    ui.label(egui::RichText::new(format!("{}", key)).strong());
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
                                            lock_recover(&self.adapter_edit).get_mut(m.as_str()).map(|eb| { eb.insert(k, v); });
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
        let home = std::env::var("HOME").unwrap_or_else(|_| "/tmp".to_string());
        std::path::PathBuf::from(home).join(".zerg-ui-prefs.json")
    }
    /// 读取 AI 模型偏好（与 preview_renderer 同一文件 ~/.zerg-ui-prefs.json 的 ai_model 字段）
    fn load_ai_model_pref() -> Option<String> {
        let s = std::fs::read_to_string(Self::preview_pref_path()).ok()?;
        let v: serde_json::Value = serde_json::from_str(&s).ok()?;
        v.get("ai_model").and_then(|x| x.as_str()).map(|x| x.to_string()).filter(|m| !m.trim().is_empty())
    }

    fn load_preview_pref() -> Option<bool> {
        let s = std::fs::read_to_string(Self::preview_pref_path()).ok()?;
        Self::parse_preview_pref(&s)
    }
        /// 纯解析（可单测）：ferrite→false / commonmark→true / 其它或坏 JSON→None（用默认）
        fn parse_preview_pref(s: &str) -> Option<bool> {
            let v: serde_json::Value = serde_json::from_str(s).ok()?;
            match v.get("preview_renderer").and_then(|x| x.as_str())? {
                "commonmark" => Some(true),
                "ferrite" => Some(false),
                _ => None,
            }
        }

    fn save_preview_pref(cm: bool) {
        let v = serde_json::json!({ "preview_renderer": if cm { "commonmark" } else { "ferrite" } });
        let _ = std::fs::write(Self::preview_pref_path(), v.to_string());
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
        ui.heading(t!("it.title"));
        ui.add_space(4.0);
        ui.weak(t!("it.desc"));

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
                            let ver = v.get("prompt_version").and_then(|x| x.as_str()).unwrap_or("");
                            let samples = v
                                .get("window")
                                .and_then(|w| w.get("samples"))
                                .and_then(|x| x.as_u64())
                                .unwrap_or(0);
                            ui.horizontal(|ui| {
                                ui.label(t!("it.hit_rate", pct = format!("{:.1}", f("ratio") * 100.0)));
                                ui.weak(t!("it.samples", n = samples, hits = u("hits"), misses = u("misses")));
                            });
                            if v.get("has_baseline").and_then(|x| x.as_bool()).unwrap_or(false) {
                                ui.weak(t!("it.baseline", pct = format!("{:.1}", f("baseline_ratio") * 100.0)));
                            }
                            ui.weak(t!("it.prompt_version", ver = if ver.is_empty() { t!("it.no_sample").to_string() } else { ver.to_string() }));
                            if let Some(alerts) = v.get("alerts").and_then(|x| x.as_array()) {
                                if !alerts.is_empty() {
                                    ui.colored_label(
                                        egui::Color32::from_rgb(220, 100, 90),
                                        t!("it.alerts", icon = icon_text("warning"), n = alerts.len()),
                                    );
                                    for al in alerts.iter().take(2) {
                                        if let Some(msg) = al.get("message").and_then(|x| x.as_str()) {
                                            ui.weak(msg);
                                        }
                                    }
                                }
                            }
                        }
                        Some(Err(e)) => {
                            ui.colored_label(egui::Color32::from_rgb(220, 100, 90), t!("it.read_failed", err = e));
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
                    t!("it.switching", target = if b { t!("it.start").to_string() } else { t!("it.stop").to_string() }),
                );
            } else if self.it_ctrl_confirm {
                // APP-A07: 二次确认（防误点启停引擎）
                ui.colored_label(
                    egui::Color32::from_rgb(220, 180, 60),
                    t!("it.confirm_engine", icon = icon_text("warning"), action = if running { t!("it.stop").to_string() } else { t!("it.start").to_string() }),
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
                let label = if running { t!("it.stop_task", icon = icon_text("stop-circle")).to_string() } else { t!("it.start_task", icon = icon_text("play")).to_string() };
                if ui.add_enabled(known, egui::Button::new(label)).clicked() {
                    self.it_ctrl_confirm = true;
                }
            }
            // 状态文案（诚实呈现：未启用 / 已停止 / 运行中 / 异常）
            if let Some(e) = err {
                ui.colored_label(egui::Color32::from_rgb(220, 100, 90), t!("it.state_unknown", err = e));
            } else if !known {
                ui.weak(t!("it.state_loading"));
            } else if !enabled {
                ui.colored_label(
                    egui::Color32::GRAY,
                    t!("it.not_enabled", reason = if reason.is_empty() { t!("it.gate_hint").to_string() } else { reason }),
                );
            } else if stopped {
                ui.colored_label(egui::Color32::from_rgb(220, 180, 60), t!("it.stopped_hint"));
            } else if running {
                ui.colored_label(egui::Color32::from_rgb(120, 200, 120), t!("it.running_hint"));
            } else {
                ui.colored_label(
                    egui::Color32::from_rgb(220, 100, 90),
                    t!("it.abnormal", reason = if reason.is_empty() { t!("it.heartbeat_stall").to_string() } else { reason }),
                );
            }
            if !tick.is_empty() && tick.len() >= 16 {
                ui.weak(t!("it.heartbeat", tick = &tick[11..16]));
            }
        });
        ui.add_space(8.0);
        // 拉取内部任务清单（缓存——每 30 秒刷新）
        if lock_recover(&self.internal_tasks).is_none() || self.last_it_fetch.elapsed().as_secs() > 30 {
            let store = self.internal_tasks.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                let items = api::fetch_internal_tasks_blocking().await.unwrap_or_default();
                let mut st = lock_recover(&store);
                *st = Some(items);
            });
            self.last_it_fetch = now;
        }
        // 周期配置（缓存——60 秒刷新）
        // APP-A06: 改用独立计时器（原来共用 last_it_fetch——被上面每 30s 的清单刷新归零，
        // 60s 条件永远不成立 → 周期下拉一直显示启动时拉的旧值）
        if lock_recover(&self.it_intervals).is_none() || self.last_it_interval_fetch.elapsed().as_secs() > 60 {
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
        let items = lock_recover(&self.internal_tasks).clone().unwrap_or_default();
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
            let (lrect, _) = ui.allocate_exact_size(egui::vec2(left_w, avail_h), egui::Sense::hover());
            let mut l_ui = ui.new_child(egui::UiBuilder::new().max_rect(lrect).layout(egui::Layout::top_down(egui::Align::Min)));
            egui::ScrollArea::vertical().id_salt("it_left_list").auto_shrink(false).show(&mut l_ui, |ui| {
                for it in &items {
                    let id = it.get("id").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                    let desc = it.get("description").and_then(|v| v.as_str()).unwrap_or("?").to_string();
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
                                if let Err(e) = api::run_internal_task_blocking(&id2).await {
                                    *lock_recover(&perr) = Some(t!("it.run_failed", id = id2, err = e).to_string());
                                }
                            });
                        }
                        // 运行模式开关（Mr2109 2026-08-28——自动/手动——点击切换）
                        let auto_run = it.get("auto_run").and_then(|v| v.as_bool()).unwrap_or(true);
                        let mode_btn = if auto_run { t!("it.mode_auto").to_string() } else { t!("it.mode_manual").to_string() };
                        if ui.selectable_label(false, mode_btn).on_hover_text(t!("it.mode_tip")).clicked() {
                            let id2 = id.clone();
                            let new_mode = !auto_run;
                            let perr = self.poll_err.clone();
                            api::runtime().spawn(async move {
                                // APP-A11: 失败提示（原 let _ = 静默）
                                if let Err(e) = api::set_internal_mode_blocking(&id2, new_mode).await {
                                    *lock_recover(&perr) = Some(t!("it.mode_failed", id = id2, err = e).to_string());
                                }
                            });
                            // 本地立即翻转（后端刷新 30s 后同步）
                            if let Some(items_ref) = lock_recover(&self.internal_tasks).as_mut() {
                                for it2 in items_ref.iter_mut() {
                                    if it2.get("id").and_then(|v| v.as_str()) == Some(id.as_str()) {
                                        if let Some(obj) = it2.as_object_mut() {
                                            obj.insert("auto_run".to_string(), serde_json::json!(new_mode));
                                        }
                                    }
                                }
                            }
                        }
                        // 周期下拉（Mr2109 2026-08-27——任务循环周期 1h-24h/指定——未设置默认显示默认执行周期）
                        let cur_h = interval_map.get(&id).copied().unwrap_or(0.0);
                        let def_h = it.get("default_hours").and_then(|v| v.as_f64()).unwrap_or(0.0);
                        let sel_text = if cur_h > 0.0 {
                            if cur_h == cur_h.trunc() {
                                format!("⏱{}h", cur_h as i64)
                            } else {
                                t!("it.interval_hours", h = cur_h).to_string()
                            }
                        } else if def_h > 0.0 {
                            t!("it.interval_default", h = def_h as i64).to_string() // 默认执行周期
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
                                            if let Err(e) = api::set_internal_interval_blocking(&id2, opt).await {
                                                *lock_recover(&perr) = Some(t!("it.set_interval_failed", id = id2, err = e).to_string());
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
                                                    if let Err(e) = api::set_internal_interval_blocking(&id2, hv).await {
                                                        *lock_recover(&perr) = Some(t!("it.set_custom_failed", id = id2, err = e).to_string());
                                                    }
                                                });
                                            }
                                        }
                                        self.it_custom_for = None;
                                        self.it_custom_hours.clear();
                                    }
                                } else if ui.selectable_label(false, t!("it.interval_custom", icon = icon_text("pencil-simple"))).clicked() {
                                    self.it_custom_for = Some(id.clone());
                                    self.it_custom_hours = format!("{}", cur_h as i64);
                                }
                            });
                        let selected = sel.as_deref() == Some(id.as_str());
                        if ui.selectable_label(selected, format!("{} | {}", id, desc_short)).clicked() {
                            clicked = Some(id.clone());
                        }
                    });
                    ui.add_space(2.0);
                }
            });
            // 拖拽条（8px 分隔线——鼠标移到显示 ⇔ 光标——可拖）
            let (drect, dresp) = ui.allocate_exact_size(egui::vec2(8.0, avail_h), egui::Sense::drag());
            ui.painter().rect_filled(drect, 0.0, ui.visuals().faint_bg_color);
            ui.painter().vline(drect.center().x, drect.y_range(), egui::Stroke::new(1.0, ui.visuals().weak_text_color()));
            let _ = dresp.clone().on_hover_cursor(egui::CursorIcon::ResizeHorizontal);
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
            let (rrect, _) = ui.allocate_exact_size(egui::vec2(right_w, avail_h), egui::Sense::hover());
            let mut r_ui = ui.new_child(egui::UiBuilder::new().max_rect(rrect));
            let detail_item = items.iter().find(|it| {
                it.get("id").and_then(|v| v.as_str()) == sel.as_deref()
            });
            match detail_item {
                Some(it) => {
                    let desc = it.get("description").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                    let template = it.get("template").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                    let cooldown = it.get("cooldown").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                    let skill = it.get("skill").and_then(|v| v.as_str()).unwrap_or("").to_string();
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
                        egui::ScrollArea::vertical().id_salt("it_right_skill").max_height(320.0).show(&mut r_ui, |ui| {
                            ui.label(skill);
                        });
                    }
                }
                None => {
                    r_ui.weak(t!("it.click_detail"));
                }
            }
        });
        if dragging {
            self.save_layout_ratio("split_it", self.split_it);
        }
        if let Some(c) = clicked {
            self.it_selected = Some(c);
        }
    }

    // ─── 布局持久化（Mr2109 2026-08-27——左右分割比例——拖动后下次启动默认）───
    fn layout_path() -> std::path::PathBuf {
        let mut p = api::task_root();
        p.push("ui_layout.json");
        p
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
            eprintln!("[zerg-ui] failed to persist layout {}: {}", path.display(), e);   // 日志英文（设计稿 §7-3）
        }
    }
}

// 布局比例持久化（Mr2109 2026-08-27——左右分割——拖动后下次启动默认）
fn load_layout_ratio(key: &str, default: f32) -> f32 {
    let mut p = api::task_root();
    p.push("ui_layout.json");
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
        //  · 真有后台任务在飞（AI 调用 / 文档写操作 / 内部任务启停）或示例虫茧引擎在跑 → 立即重绘；
        //  · 其余时候 500ms 唤醒一次（数据轮询是 3/5/10/30/60s 级，帧级重绘没有任何意义，
        //    但完全不等又会饿死定时轮询，故保留一个低频心跳）。
        let busy = self.ai_busy
            || self.ai_pending.is_some()
            || lock_recover(&self.doc_op_result).is_some()
            || lock_recover(&self.it_ctrl_result).is_some()
            || self.it_ctrl_busy.is_some()
            || self.rt_active;
        if busy {
            ui.ctx().request_repaint();
        } else {
            ui.ctx().request_repaint_after(std::time::Duration::from_millis(500));
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
            crate::modules::top_nav_bar(ui, &mut self.registry, self.online, &self.locale, &mut || {
                switch_locale = true;
            }, &mut || {
                open_manager = true;
            }, !self.hud_hidden, &mut || {
                self.hud_hidden = !self.hud_hidden;
            });
            if switch_locale {
                if self.locale == "zh-CN" {
                    self.locale = "en".to_string();
                    rust_i18n::set_locale("en");
                } else {
                    self.locale = "zh-CN".to_string();
                    rust_i18n::set_locale("zh-CN");
                }
            }
            if open_manager {
                self.show_module_manager = true; // ➕ 打开吊装系统面板
            }
        });

        // v2.5.6 模块管理面板（➕ 吊装系统——M2）
        if self.show_module_manager {
            self.module_manager_view(ui.ctx());
        }

        // F5 AI 动力（结果轮询 + 结果弹窗）
        self.ai_poll();
        self.ai_result_view(ui.ctx());

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
            self.main_view(ui);
        });

        // v2.5.7 HUD 悬浮层（右上角——core 状态/模块/running 数——点 ✕ 隐藏——nav 图标开关）
        if !self.hud_hidden && self.online {
            self.hud_view(ui.ctx());
        }
    }
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
    let idx = id
        .char_indices()
        .rev()
        .nth(5)
        .map(|(i, _)| i)
        .unwrap_or(0);
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
        //   · UI 侧唯一来源 = ui/Cargo.toml `version`（窗口标题 + 底栏 + 10 个集装箱箱版本全用
        //     env!("CARGO_PKG_VERSION") 编译期取值，改一处即全改）
        //   · Go 侧唯一来源 = core/internal/version.Version（启动横幅 + /api/capabilities + openapi info.version）
        //   · 两处一致性由门禁 scripts/check_version.py 断言（CI + 发布导出）
        // 收版时改这两处 + 运行中二进制复核（/api/capabilities、/api/openapi.json、窗口标题）。
        // 数据：当前模块名 + running 任务数（复用现有 tasks——不新拉）
        let mod_name: String = self
            .registry
            .modules
            .iter()
            .find(|m| m.id == self.registry.active)
            .map(|m| t!(m.name_key).to_string())
            .unwrap_or_else(|| self.registry.active.clone());
        let running = lock_recover(&self.tasks)
            .as_ref()
            .map(|ts| ts.iter().filter(|t| t.status.as_deref() == Some("running")).count())
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
                            if ui.small_button("✕").on_hover_text(t!("hud.hide_tip")).clicked() {
                                self.hud_hidden = true;
                            }
                        });
                    });
            });
    }

}

#[cfg(test)]
mod m06_preview_pref_tests {
    use super::ZergApp;

    /// M06 双渲染器：偏好解析（Mr2109的"记住选择"）
    #[test]
    fn parse_preview_pref_cases() {
        assert_eq!(ZergApp::parse_preview_pref(r#"{"preview_renderer":"commonmark"}"#), Some(true));
        assert_eq!(ZergApp::parse_preview_pref(r#"{"preview_renderer":"ferrite"}"#), Some(false));
        assert_eq!(ZergApp::parse_preview_pref(r#"{"preview_renderer":"weird"}"#), None);
        assert_eq!(ZergApp::parse_preview_pref("not json"), None);
        assert_eq!(ZergApp::parse_preview_pref("{}"), None);
    }
}
