// 虫族 UI 主程序（eframe::App——三栏布局——任务视图——离线全灰）
// 异步: 自己管理（tokio spawn + SharedResult——不用 egui-async——eframe 0.36 兼容问题）
use eframe::egui;
use rust_i18n::t;

use crate::api::{self, SharedResult, TaskInfo};
use crate::modules::icons::icon_text; // P3 图标（iconflow）

/// 通用异步结果（每帧检查——take 消费）
struct AsyncData<T> {
    result: SharedResult<T>,
    last_trigger: f64,
}

impl<T> AsyncData<T> {
    fn new() -> Self {
        Self {
            result: Arc::new(Mutex::new(None)),
            last_trigger: 0.0,
        }
    }
    /// 检查结果（消费——返回克隆数据）
    fn take(&self) -> Option<T>
    where
        T: Clone,
    {
        self.result.lock().unwrap().take().and_then(|r| r.ok())
    }
    /// 是否已完成（不管结果）
    fn is_done(&self) -> bool {
        self.result.lock().unwrap().is_some()
    }
}

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
    // v2.5.6 md 编辑器（Mr2109 2026-08-29: 第三栏=md 编辑器——编辑/预览/保存）
    doc_edit: String,       // 编辑缓冲（TextEdit 内容）
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
    preview_syncing: bool,        // 本次帧是否正在同步（防覆盖）
    // F5 AI 动力（Mr2109统一接口——网关 8082）
    ai_busy: bool,                // AI 调用中
    ai_status: String,            // AI 状态提示（busy 时显示）
    ai_pending: Option<(String, api::SharedResult<String>)>, // (动作, 异步结果) 待处理
    ai_output: Option<(String, String)>, // (动作, 结果) 完成显示
    // v2.5.7 对话模块（Mr2109——完全借鉴 Hermes——第一板块）
    chat_view: crate::modules::chat::ChatView,
    // v2.5.6 文档右键操作（Mr2109 2026-08-29）
    doc_clipboard: Option<String>, // 复制缓冲（复制的文件路径）
    doc_input: Option<(String, String)>, // 输入对话框 (标题, 当前值)——重命名/新建用
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
    internal_stopped: bool, // v2.5.6 内部任务启停状态（toggle 按钮）
    // APP-A07: 启停结果回报——成功才翻转 internal_stopped；失败红字提示且不改本地状态
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
            doc_dir: "00-总览".to_string(), // v2.5.6 默认选中总览目录
            doc_file: String::new(),
            doc_content: Arc::new(Mutex::new(None)),
            doc_edit: String::new(),
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
            preview_syncing: false,
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
            roundtable: None,
            rt_active: false,
            selected_task: None,
            detail_id: String::new(),
            last_refresh: 0.0,
            internal_tasks: std::sync::Arc::new(std::sync::Mutex::new(None)),
            last_it_fetch: std::time::Instant::now(),
            last_it_interval_fetch: std::time::Instant::now(),
            internal_stopped: false,
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
                *store.lock().unwrap() = Some(ok);
            });
        }
        // 收集在线结果
        if let Some(ok) = self.online_result.lock().unwrap().take() {
            self.online = ok;
        }
        // 文档操作结果(APP-A02 2026-09-10 审计)——成功才改本地状态;失败只提示、不改状态
        if let Some(r) = self.doc_op_result.lock().unwrap().take() {
            let ctx = self.doc_op_ctx.take();
            match r {
                Ok(()) => {
                    if let Some((kind, path)) = ctx {
                        match kind.as_str() {
                            "del_dir" => {
                                if self.doc_dir == path {
                                    self.doc_dir = "00-总览".to_string();
                                }
                            }
                            "del_file" => {
                                if self.doc_file == path {
                                    self.doc_file = String::new();
                                    *self.doc_content.lock().unwrap() = None;
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
        // APP-A07: 内部任务启停结果——成功才翻转按钮状态；失败只提示（原实现不成功也翻转）
        if let Some(r) = self.it_ctrl_result.lock().unwrap().take() {
            match r {
                Ok(()) => {
                    if let Some(target) = self.it_ctrl_target.take() {
                        self.internal_stopped = target;
                    }
                    *self.poll_err.lock().unwrap() = None;
                }
                Err(e) => {
                    self.it_ctrl_target = None;
                    *self.poll_err.lock().unwrap() = Some(format!("内部任务启停: {}", e));
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
                        *store.lock().unwrap() = Some(v);
                        *perr.lock().unwrap() = None;
                    }
                    Err(e) => *perr.lock().unwrap() = Some(format!("任务列表: {}", e)),
                }
            });
        }
        // 任务已加载标记（主线程——store 有值=已拉到）
        if !self.tasks_loaded {
            if self.tasks.lock().unwrap().is_some() {
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
                        *store.lock().unwrap() = Some(v);
                        *perr.lock().unwrap() = None;
                    }
                    Err(e) => *perr.lock().unwrap() = Some(format!("git 状态: {}", e)),
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
                        *store.lock().unwrap() = Some(v);
                        *perr.lock().unwrap() = None;
                    }
                    Err(e) => *perr.lock().unwrap() = Some(format!("主控日志: {}", e)),
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
                        *store.lock().unwrap() = Some(v);
                        *perr.lock().unwrap() = None;
                    }
                    Err(e) => *perr.lock().unwrap() = Some(format!("文档目录: {}", e)),
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
                        *store.lock().unwrap() = Some(v);
                        *perr.lock().unwrap() = None;
                    }
                    Err(e) => *perr.lock().unwrap() = Some(format!("资源库: {}", e)),
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
                        *store.lock().unwrap() = Some(v);
                        *perr.lock().unwrap() = None;
                    }
                    Err(e) => *perr.lock().unwrap() = Some(format!("集群状态: {}", e)),
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
            self.doc_content.lock().unwrap().clone().unwrap_or_default()
        };
        if text.trim().is_empty() {
            self.ai_output = Some(("error".to_string(), "文档内容为空——无法执行 AI 操作".to_string()));
            return;
        }
        let model = "example-35b-v2".to_string(); // 中文好/工具正常（Mr2109）
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
        self.ai_status = format!("AI {} 中...", action);
        let result: api::SharedResult<String> = Arc::new(Mutex::new(None));
        let r2 = result.clone();
        api::runtime().spawn(async move {
            let res = api::ai_prompt_blocking(&model, &prompt).await;
            *r2.lock().unwrap() = Some(res);
        });
        self.ai_pending = Some((action.to_string(), result));
    }

    /// F5 AI 结果轮询（每帧检查 pending 是否完成）
    fn ai_poll(&mut self) {
        if let Some((action, result)) = self.ai_pending.clone() {
            if let Some(res) = result.lock().unwrap().clone() {
                self.ai_pending = None;
                self.ai_busy = false;
                match res {
                    Ok(text) => {
                        self.ai_output = Some((action, text));
                        self.ai_status = String::new();
                    }
                    Err(e) => {
                        // 失败弹窗提示（不能静默）
                        self.ai_output = Some(("error".to_string(), format!("AI 调用失败: {}", e)));
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
            let cur = self.doc_content.lock().unwrap().clone().unwrap_or_default();
            self.ferrite_editor.load(&cur);
            self.ferrite_text_cache = None;
            self.ferrite_loaded = true;
        }
    }

    /// APP-A10: 预览模式渲染的是 doc_content（编辑模式渲染 rope）——AI 结果同步过去才看得见
    fn sync_preview_after_ai(&mut self) {
        if !self.doc_edit_mode {
            let txt = self.ferrite_editor.text();
            *self.doc_content.lock().unwrap() = Some(txt);
            self.ferrite_text_cache = None;
        }
    }

    /// F5 AI 结果弹窗（总结/翻译/续写结果——可插入/替换/复制）
    fn ai_result_view(&mut self, ctx: &egui::Context) {
        if let Some((action, text)) = self.ai_output.clone() {
            let title = match action.as_str() {
                "summarize" => format!("{} AI 总结", icon_text("sparkles")),
                "continue" => format!("{} AI 续写", icon_text("pencil-simple")),
                "translate" => format!("{} AI 翻译", icon_text("translate")),
                "polish" => format!("{} AI 润色", icon_text("sparkles")),
                "error" => format!("{} AI 错误", icon_text("warning")),
                _ => format!("{} AI 结果", icon_text("sparkles")),
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
                            if ui.button(format!("{} 关闭", icon_text("x-circle"))).clicked() {
                                self.ai_output = None;
                            }
                            return;
                        }
                        if ui.button("📥 插入到文档末尾").clicked() {
                            // APP-A10: 编辑器未载入（预览模式点的）→ 先载入当前文档再追加，
                            // 否则结果既看不见、进编辑模式时又会被文件内容 load 覆盖掉
                            self.ensure_editor_loaded();
                            self.ferrite_editor.append_text(&format!("\n\n{}", text));
                            self.doc_edit_dirty = true;
                            self.sync_preview_after_ai();
                            self.ai_output = None;
                        }
                        if ui.button(format!("{} 替换全文", icon_text("note-pencil"))).clicked() {
                            // APP-A10: 同上——预览模式渲染的是 doc_content，结果同步过去才看得见
                            self.ensure_editor_loaded();
                            self.ferrite_editor.load(&text);
                            self.doc_edit_dirty = true;
                            self.sync_preview_after_ai();
                            self.ai_output = None;
                        }
                        if ui.button(format!("{} 关闭", icon_text("x-circle"))).clicked() {
                            self.ai_output = None;
                        }
                    });
                });
        }
    }

    /// 渲染主视图（v2.5.6——内容区由集装箱注册表分发）
    /// 渲染任务视图（主区——队列 + 详情各占一半——水平布局）
    fn tasks_view(&mut self, ui: &mut egui::Ui) {
        ui.heading(t!("task_queue"));
        ui.add_space(4.0);
        let list: Vec<TaskInfo> = self.tasks.lock().unwrap().clone().unwrap_or_default();
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
                    Ok(v) => *store.lock().unwrap() = Some(v),
                    Err(e) => *perr.lock().unwrap() = Some(format!("任务详情: {}", e)),
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
        if list.is_empty() {
            // 已加载但无任务——显示"暂无任务"（不是"拉取中"）
            if self.tasks_loaded {
                ui.weak("暂无任务");
            } else {
                ui.spinner();
                ui.weak(t!("loading"));
            }
            return;
        }
        // 归档折叠区（Mr2109 2026-08-22——30 天归档/90 天删除可查）
        if self.archive.lock().unwrap().is_none() || self.last_archive_fetch.elapsed().as_secs() > 60 {
            let store = self.archive.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                let items = api::fetch_archive_blocking().await.unwrap_or_default();
                let mut st = store.lock().unwrap();
                *st = Some(items);
            });
            self.last_archive_fetch = now;
        }
        let arc_count = self.archive.lock().unwrap().clone().unwrap_or_default().len();
        egui::CollapsingHeader::new(format!("🗄️ 归档（{}）", arc_count))
            .id_salt("task_group_archive") // APP-A08: 稳定 id（标题含计数——每帧变会让展开态被重置）
            .default_open(false)
            .show(ui, |ui| {
                let arcs = self.archive.lock().unwrap().clone().unwrap_or_default();
                if arcs.is_empty() {
                    ui.weak("暂无归档（30 天归档 / 90 天删除——容量有界）");
                }
                for a in arcs {
                    let tid = a.get("task_id").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                    let when = a.get("archived").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                    let size = a.get("size").and_then(|v| v.as_i64()).unwrap_or(0);
                    let short = if tid.len() > 6 { &tid[tid.len() - 6..] } else { &tid };
                    ui.label(format!("📦 {} | 归档: {} | {}KB", short, &when[..when.len().min(16)], size / 1024));
                }
            });
        ui.add_space(4.0);
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
                egui::CollapsingHeader::new(format!("🔄 {}（{}）", t!("running"), running_count))
                    .id_salt("task_group_running") // APP-A08: 稳定 id
                    .default_open(true)
                    .show(ui, |ui| {
                        for t in &running {
                            self.task_row(ui, t);
                        }
                        if running_count == 0 {
                            ui.weak(t!("none"));
                        }
                    });
                let queued_count = queued.len();
                egui::CollapsingHeader::new(format!("⏳ {}（{}）", t!("queued"), queued_count))
                    .id_salt("task_group_queued") // APP-A08: 稳定 id
                    .default_open(true)
                    .show(ui, |ui| {
                        for t in &queued {
                            self.task_row(ui, t);
                        }
                        if queued_count == 0 {
                            ui.weak(t!("none"));
                        }
                    });
                let waiting_count = waiting.len();
                egui::CollapsingHeader::new(format!("🔁 等待重试（{}）", waiting_count))
                    .id_salt("task_group_waiting_retry") // APP-A08: 稳定 id
                    .default_open(true)
                    .show(ui, |ui| {
                        for t in &waiting {
                            self.task_row(ui, t);
                        }
                        if waiting_count == 0 {
                            ui.weak(t!("none"));
                        }
                    });
                let others_count = others.len();
                if others_count > 0 {
                    egui::CollapsingHeader::new(format!("❔ 其它状态（{}）", others_count))
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
                    let date = t.completed_at.as_deref().map(|s| if s.len() >= 10 { s[..10].to_string() } else { s.to_string() }).unwrap_or_else(|| "未知日期".to_string());
                    by_date.entry(date).or_default().push(t);
                }
                egui::CollapsingHeader::new(format!("✅ {}（{}）", t!("done"), done_count))
                    .id_salt("task_group_done") // APP-A08: 稳定 id
                    .default_open(false)
                    .show(ui, |ui| {
                        if done_count == 0 {
                            ui.weak(t!("none"));
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
                *perr.lock().unwrap() = Some(format!("{}失败: {}", what, e));
            }
            match api::fetch_tasks_blocking().await {
                Ok(v) => *store.lock().unwrap() = Some(v),
                Err(e) => *perr.lock().unwrap() = Some(format!("任务列表: {}", e)),
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
            "done" => "成功",
            "failed" => "失败",
            "running" => "…",
            _ => "待",
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
                    Ok(v) => *store.lock().unwrap() = Some(v),
                    Err(e) => *perr.lock().unwrap() = Some(format!("任务详情: {}", e)),
                }
            });
        }
        // 右键菜单（Mr2109 2026-08-21: 复制/重跑/置顶置底/上移下移/删除）
        let task_id_owned = t.id.clone().unwrap_or_default();
        let status_owned = status.to_string();
        resp.context_menu(|ui| {
            // 复制任务描述
            if ui.button(format!("{} 复制描述", icon_text("copy"))).clicked() {
                if let Some(desc) = t.description.clone() {
                    ui.ctx().copy_text(desc);
                }
                ui.close();
            }
            // 重跑（failed 任务）
            if status == "failed" && ui.button("🔁 重跑任务").clicked() {
                let id = task_id_owned.clone();
                // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                self.task_op(async move { api::task_retry_blocking(&id).await }, "重跑任务");
                ui.close();
            }
            // 重回排队（done/failed 任务——重新入队——Mr2109 2026-08-21）
            if (status == "done" || status == "failed") && ui.button("🔙 重回排队").clicked() {
                let id = task_id_owned.clone();
                // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                self.task_op(async move { api::task_retry_blocking(&id).await }, "重回排队");
                ui.close();
            }
            // 暂停/继续（queued 任务——Mr2109 2026-08-21）
            if status_owned == "queued" {
                if ui.button(format!("{} 暂停", icon_text("pause"))).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_pause_blocking(&id, true).await }, "暂停任务");
                    ui.close();
                }
                if ui.button(format!("{} 继续", icon_text("play"))).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_pause_blocking(&id, false).await }, "继续任务");
                    ui.close();
                }
                ui.separator();
                if ui.button("⏫ 置顶").clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_move_blocking(&id, "top").await }, "置顶任务");
                    ui.close();
                }
                if ui.button("⏬ 置底").clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_move_blocking(&id, "bottom").await }, "置底任务");
                    ui.close();
                }
                if ui.button("⬆ 上移").clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_move_blocking(&id, "up").await }, "上移任务");
                    ui.close();
                }
                if ui.button("⬇ 下移").clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_move_blocking(&id, "down").await }, "下移任务");
                    ui.close();
                }
                if ui.button(format!("{} 删除", icon_text("trash"))).clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_delete_blocking(&id).await }, "删除任务");
                    ui.close();
                }
            }
            // 执行中任务操作（running——终止/重回队列——Mr2109 2026-08-22）
            if status_owned == "running" {
                if ui.button("🛑 终止").clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_terminate_blocking(&id).await }, "终止任务");
                    ui.close();
                }
                if ui.button("🔁 重回队列").clicked() {
                    let id = task_id_owned.clone();
                    // APP-A11: 结果不再 let _ = 丢弃——失败写提示条（原来操作失败界面无任何反馈）
                    self.task_op(async move { api::task_requeue_blocking(&id).await }, "重回队列");
                    ui.close();
                }
            }
        });
    }

    /// 任务详情（右栏——含时间线）
    fn task_detail(&mut self, ui: &mut egui::Ui) {
        ui.heading(t!("task_detail"));
        ui.add_space(4.0);
        if let Some(t) = &self.selected_task {
            ui.label(format!("ID: {}", t.id.as_deref().unwrap_or("?")));
            ui.label(format!("{}: {}", t!("task_desc"), t.description.as_deref().unwrap_or("?")));
            ui.label(format!("{}: {}", t!("task_type"), t.task_type.as_deref().unwrap_or("?")));
            ui.label(format!("{}: {}", t!("task_priority"), t.priority.unwrap_or(0)));
            ui.label(format!("{}: {}", t!("task_status"), t.status.as_deref().unwrap_or("?")));
            ui.label(format!("{}: {}", t!("task_model"), t.model.as_deref().unwrap_or("?")));
            ui.label(format!("{}: {}", t!("task_machine"), t.machine.as_deref().unwrap_or("?")));
            ui.add_space(8.0);
            ui.separator();
            if let Some(detail) = self.task_detail.lock().unwrap().clone() {
                ui.label(t!("timeline"));
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
                                format!("[{}{}] {} {}", t!("round"), rn, t!("tools"), tools)
                            } else {
                                // Mr2109 2026-08-21: 耗时排在工具调用后面
                                format!("[{}{}] {} {} | ⏱ {}", t!("round"), rn, t!("tools"), tools, dur)
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
                        ui.weak(t!("no_rounds"));
                    }
                } else {
                    ui.weak(t!("no_trace"));
                }
                ui.add_space(8.0);
                ui.separator();
                // S8: 结晶阶段进度卡（subtask_mode=true 时 stages[] 渲染——阶段/目标/状态）
                if detail.get("subtask_mode").and_then(|v| v.as_bool()).unwrap_or(false) {
                    if let Some(stages) = detail.get("stages").and_then(|v| v.as_array()) {
                        ui.label("🧩 结晶阶段");
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
                    ui.label("📄 执行报告");
                    ui.monospace(rep);
                }
                // 复查报告（直接展开——外层统一滚——不嵌套 ScrollArea）
                if let Some(rep) = detail.get("review_report").and_then(|r| r.as_str()) {
                    ui.add_space(8.0);
                    ui.label("🔍 复查报告");
                    ui.monospace(rep);
                }
            } else {
                ui.spinner();
                ui.weak(t!("loading"));
            }
        } else {
            ui.weak(t!("click_task"));
        }
    }

    /// 渲染模块管理面板（➕ 吊装系统——Mr2109 2026-08-29 M2）
    /// 核心箱（船体——不可禁用）+ 可装卸箱（checkbox 开关——变更即持久化）
    fn module_manager_view(&mut self, ctx: &egui::Context) {
        let mut close = false;
        egui::Window::new("🧩 模块管理（吊装系统）")
            .collapsible(false)
            .resizable(true)
            .default_size([420.0, 380.0])
            .show(ctx, |ui| {
                ui.label("⚡ 核心（船体箱——不可禁用——任务体系+基础设施）");
                ui.separator();
                let mut changed = false;
                for m in self.registry.modules.iter().filter(|m| m.is_core) {
                    ui.horizontal(|ui| {
                        ui.label(format!("{} {}", m.icon, m.name));
                        ui.weak(m.description);
                        ui.label("🔒");
                    });
                }
                ui.add_space(8.0);
                ui.label("🧩 模块（集装箱——可吊装/卸下——变化即持久化）");
                ui.separator();
                // 先收集切换请求（避免迭代中改 registry——借用冲突）
                let mut to_toggle: Option<String> = None;
                for m in self.registry.modules.iter().filter(|m| !m.is_core) {
                    let on = self.registry.enabled.get(m.id).copied().unwrap_or(true);
                    let mut next = on;
                    ui.horizontal(|ui| {
                        if ui.checkbox(&mut next, format!("{} {}", m.icon, m.name)).changed() {
                            if next != on {
                                to_toggle = Some(m.id.to_string());
                            }
                        }
                        ui.weak(m.description);
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
                    ui.label("🌍 生态箱（外部应用——配置文件声明——可吊装/卸下）");
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
                if ui.button("关闭").clicked() {
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
                ui.heading(t!("cluster_status"));
                ui.add_space(4.0);
                if let Some(r) = self.cluster.lock().unwrap().clone() {
                    egui::ScrollArea::vertical().show(ui, |ui| {
                        // 总览
                        let healthy = r.get("healthy_count").and_then(|h| h.as_u64()).unwrap_or(0);
                        let total = r.get("total_machines").and_then(|h| h.as_u64()).unwrap_or(0);
                        let active = r.get("total_active_requests").and_then(|h| h.as_u64()).unwrap_or(0);
                        ui.label(format!("✅ 健康 {}/{} ｜ 🌀 活跃请求 {}", healthy, total, active));
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
                                    ui.label(format!("  模型: {}", model));
                                    ui.label(format!("  内存: {:.0}/{:.0} GB", mem_avail, mem_total));
                                    ui.label(format!("  负载: {:.2} ｜ GPU: {:.1} GB", load, gpu));
                                });
                                ui.separator();
                            }
                        }
                    });
                } else {
                    ui.spinner();
                    ui.weak(t!("loading"));
                }
            }
            "git" => {
                ui.heading(t!("git_overview"));
                ui.add_space(4.0);
                if let Some(g) = self.git_status.lock().unwrap().clone() {
                    // 分支（通俗化——task-xxx → 任务类型名）
                    egui::CollapsingHeader::new(format!("🌿 {}（{}）", t!("branches"), g.branches.as_ref().map(|b| b.len()).unwrap_or(0)))
                        .id_salt("git_branches") // APP-A08: 稳定 id
                        .show(ui, |ui| {
                        if let Some(branches) = &g.branches {
                            for b in branches {
                                ui.label(format!("  {}", humanize_branch(b)));
                            }
                        }
                    });
                    // worktree（通俗化——只显示分支名——不显示完整路径）
                    egui::CollapsingHeader::new(format!("📂 {}（{}）", t!("worktrees"), g.worktrees.as_ref().map(|w| w.len()).unwrap_or(0)))
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
                            t!("unmerged"),
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
                            ui.weak(format!("  {}", t!("none")));
                        }
                    });
                } else {
                    ui.spinner();
                    ui.weak(t!("loading"));
                }
            }
            "logs" => {
                ui.heading(t!("logs"));
                ui.add_space(4.0);
                if let Some(lines) = self.logs.lock().unwrap().clone() {
                    egui::ScrollArea::vertical().auto_shrink([false, false]).show(ui, |ui| {
                        for l in lines {
                            let color = if l.contains("error") || l.contains("错误") {
                                egui::Color32::from_rgb(220, 80, 80)
                            } else if l.contains("warn") || l.contains("警告") {
                                egui::Color32::from_rgb(240, 200, 80)
                            } else {
                                egui::Color32::from_rgb(200, 200, 200)
                            };
                            ui.label(egui::RichText::new(l.as_str()).color(color));
                        }
                    });
                } else {
                    ui.spinner();
                    ui.weak(t!("loading"));
                }
            }
            // 🐛 虫茧=平台（Mr2109 2026-09-03：平台界面呈现无数应用——示例虫茧只是其一）
            // rt_active=false → 平台启动器（应用栅格）；true → 示例虫茧全屏（引擎后台继续 M2）
            "roundtable" => {
                if !self.rt_active {
                    // ── 平台界面：应用栅格（无数茧——每个=独立集装箱应用——示例虫茧=第一个）──
                    ui.heading(format!("{} {}", icon_text("boxes"), t!("cocoon_platform")));
                    ui.weak(t!("cocoon_platform_hint"));
                    ui.add_space(10.0);
                    // 应用清单（平台雏形——未来读集装箱注册/目录扫描——现静态声明可扩展）
                    // 结构：每卡=独立 git 集装箱应用（id/名字/描述/打开）
                    let cards: [(String, &str, &str); 1] = [(
                        "roundtable".into(),
                        "示例虫茧",
                        "自动小说创作工坊——13 角色圆桌讨论→锁定设定→章节→评审（首个茧——引擎层 35 测试）",
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
                                card_ui.label(egui::RichText::new(*desc).size(12.0).color(egui::Color32::from_rgb(170, 175, 185)));
                                card_ui.with_layout(egui::Layout::right_to_left(egui::Align::BOTTOM), |ui| {
                                    if ui.button(egui::RichText::new(t!("cocoon_open")).size(12.0)).clicked() {
                                        self.rt_active = true;
                                    }
                                });
                                if card_ui.rect_contains_pointer(rect) && ui.ctx().input(|i| i.pointer.any_click()) {
                                    self.rt_active = true;
                                }
                                ui.allocate_exact_size(egui::vec2(0.0, 0.0), egui::Sense::hover());
                                ui.end_row();
                            }
                        });
                    });
                } else {
                    // ── 示例虫茧全屏（嵌中央区——面包屑一层——切走引擎后台继续 M2）──
                    // 三层收一层（2026-09-04）：宿主不再画返回条——示例虫茧面包屑自带"← 虫茧平台"
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
                ui.heading(t!("docs"));
                ui.add_space(4.0);
                let docs_snap = self.docs.lock().unwrap().clone(); // 先释放借用——内部闭包要 &mut self（F5 AI 按钮）
                if let Some((files, dirs)) = docs_snap {
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
                            ui.heading("📚 目录树");
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
                                    if ui.button("📝 重命名").clicked() {
                                        self.doc_input = Some(("重命名目录".to_string(), dir.clone()));
                                        self.doc_input_buf = dir.clone(); // APP-A09
                                        self.doc_input_new = true;
                                        ui.close();
                                    }
                                    if ui.button(format!("{} 删除", icon_text("trash"))).clicked() {
                                        let path = dir.clone();
                                        // APP-A02: 先确认成功再改本地状态(原实现丢结果 + 立即切目录)
                                        self.doc_op_ctx = Some(("del_dir".to_string(), path.clone()));
                                        self.doc_op_result = api::doc_op_async("delete", serde_json::json!({"path": path}));
                                        ui.close();
                                    }
                                    if ui.button(format!("{} 新建子目录", icon_text("folder-plus"))).clicked() {
                                        let base = dir.clone();
                                        self.doc_input = Some(("新建子目录".to_string(), format!("{}/", base)));
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
                                    *self.doc_content.lock().unwrap() = None;
                                }
                            }
                            // v2.5.6 空白右键——新建目录（Mr2109 2026-08-29）
                            ui.add_space(4.0);
                            if ui.button(format!("{} 新建目录", icon_text("folder-plus"))).clicked() {
                                self.doc_input = Some(("新建目录".to_string(), String::new()));
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
                                    if ui.button("📝 重命名").clicked() {
                                        self.doc_input = Some(("重命名文件".to_string(), f.to_string()));
                                        self.doc_input_buf = f.to_string(); // APP-A09
                                        self.doc_input_new = true;
                                        ui.close();
                                    }
                                    if ui.button(format!("{} 复制", icon_text("copy"))).clicked() {
                                        self.doc_clipboard = Some(f.to_string());
                                        ui.close();
                                    }
                                    if let Some(src) = self.doc_clipboard.clone() {
                                        if ui.button(format!("{} 粘贴到此处", icon_text("clipboard"))).clicked() {
                                            // 目标 = 当前目录 + 源文件名（冲突加副本后缀）
                                            let fname_src = src.split('/').last().unwrap_or(&src).to_string();
                                            let target = format!("{}/{}", self.doc_dir, fname_src);
                                            // APP-A02: 粘贴(复制)也走结果回报——失败可见
                                            self.doc_op_ctx = Some(("copy".to_string(), String::new()));
                                            self.doc_op_result = api::doc_op_async("copy", serde_json::json!({"from": src, "to": target}));
                                            ui.close();
                                        }
                                    }
                                    if ui.button(format!("{} 删除", icon_text("trash"))).clicked() {
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
                                    // 拉文档内容（异步）
                                    let path = f.to_string();
                                    let store = self.doc_content.clone();
                                    api::runtime().spawn(async move {
                                        let r = api::fetch_doc_content_blocking(path).await.ok();
                                        *store.lock().unwrap() = r;
                                    });
                                }
                            }
                            if dir_files.is_empty() {
                                ui.weak(t!("none"));
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
                            c3_ui.weak("← 左侧选择目录和文件查看/编辑内容");
                        } else {
                            c3_ui.horizontal(|ui| {
                                ui.label(format!("📄 {}", self.doc_file.split('/').last().unwrap_or("")));
                                // v2.5.6 md 编辑器工具栏（Mr2109 2026-08-29）
                                if ui.button(if self.doc_edit_mode { "🔍 预览".to_string() } else { format!("{} 编辑", icon_text("note-pencil")) }).clicked() {
                                    self.doc_edit_mode = !self.doc_edit_mode;
                                    // 切到编辑时同步缓冲
                                    if self.doc_edit_mode {
                                        if let Some(content) = self.doc_content.lock().unwrap().clone() {
                                            self.doc_edit = content;
                                        }
                                    }
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
                                    if ui.button(format!("{} 保存", icon_text("floppy-disk"))).clicked() {
                                        // 保存——调 /api/docs/save（M3: 取 Ferrite 编辑器文本）
                                        let path = self.doc_file.clone();
                                        // APP-A01 修复(2026-09-10): 编辑器从未载入(预览模式 AI 插入等)时，
                                        // 绝不能用空/不完整缓冲覆盖整份文档——回落到当前文档内容
                                        let content = if self.ferrite_loaded {
                                            self.ferrite_editor.text()
                                        } else {
                                            self.doc_content.lock().unwrap().clone().unwrap_or_default()
                                        };
                                        // APP-A02: 保存成功才清 dirty/回预览；失败红字提示
                                        self.doc_op_ctx = Some(("save".to_string(), path.clone()));
                                        self.doc_op_result = api::doc_op_async("save", serde_json::json!({"path": path, "content": content}));
                                    }
                                }
                                // F5 AI 动力（Mr2109统一接口——网关 8082——虫族版编辑器本质特征）
                                if !self.ai_busy {
                                    if ui.button("✨ 总结").clicked() {
                                        self.ai_run("summarize");
                                    }
                                    if ui.button(format!("{} 续写", icon_text("pencil-simple"))).clicked() {
                                        self.ai_run("continue");
                                    }
                                    if ui.button(format!("{} 翻译", icon_text("translate"))).clicked() {
                                        self.ai_run("translate");
                                    }
                                    if ui.button("💄 润色").clicked() {
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
                                    if let Some(content) = self.doc_content.lock().unwrap().clone() {
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
                                        ui.weak("📑 大纲:");
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
                                        let preview_text = self.ferrite_text_cached(); // M06: 带缓存
                                        // F4 滚动同步：编辑 scroll_line 变化 → 预览跟随（比例换算）
                                        let line_count = self.ferrite_editor.line_count().max(1);
                                        let editor_line = self.ferrite_editor.scroll_line();
                                        let sync_target = if editor_line != self.preview_sync_line {
                                            self.preview_syncing = true;
                                            self.preview_sync_line = editor_line;
                                            let ratio = editor_line as f32 / line_count as f32;
                                            let viewport = avail_h3;
                                            (ratio * (self.preview_content_h - viewport)).max(0.0)
                                        } else {
                                            self.preview_syncing = false;
                                            self.preview_last_offset
                                        };
                                        let out = egui::ScrollArea::vertical()
                                            .id_salt("ferrite_preview")
                                            .auto_shrink(false)
                                            .vertical_scroll_offset(sync_target)
                                            .show(&mut p_ui, |ui| {
                                                crate::modules::ferrite::markdown::render_markdown(ui, &preview_text);
                                            });
                                        self.preview_content_h = out.content_size.y;
                                        self.preview_last_offset = out.state.offset.y;
                                    },
                                );
                            } else if let Some(content) = self.doc_content.lock().unwrap().clone() {
                                egui::ScrollArea::vertical().id_salt("docs_col3").auto_shrink(false).show(&mut c3_ui, |ui| {
                                    // v2.5.6 md 渲染（egui_commonmark CommonMarkViewer——支持标题/列表/代码块/表格）
                                    egui_commonmark::CommonMarkViewer::new().show(ui, &mut self.doc_md_cache, &content);
                                });
                            } else {
                                c3_ui.spinner();
                                c3_ui.weak(t!("loading"));
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
                    if let Some((title, current)) = self.doc_input.clone() {
                        let mut new_val = std::mem::take(&mut self.doc_input_buf);
                        let mut needs_focus = self.doc_input_new;
                        let mut close = false;
                        let mut do_submit = false;
                        egui::Window::new(title.as_str())
                            .collapsible(false)
                            .resizable(false)
                            .show(ui.ctx(), |ui| {
                                ui.horizontal(|ui| {
                                    ui.label("名称:");
                                    let resp = ui.text_edit_singleline(&mut new_val);
                                    if needs_focus {
                                        resp.request_focus(); // 仅首帧——原来每帧抢焦点（中文 IME 打不进）
                                        needs_focus = false;
                                    }
                                    if ui.button("确定").clicked() {
                                        do_submit = true;
                                    }
                                    if ui.button("取消").clicked() {
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
                            let t = title.clone();
                            let v = self.doc_input_buf.trim().to_string();
                            let current_path = current.clone();
                            if !v.is_empty() {
                                if t.contains("重命名") {
                                    // old=当前完整路径, new=父目录+v（目录）或 v（文件）
                                    let parent = current_path.rfind('/').map(|i| current_path[..i].to_string()).unwrap_or_default();
                                    let new_path = if parent.is_empty() { v.clone() } else { format!("{}/{}", parent, v) };
                                    self.doc_op_ctx = Some(("rename".to_string(), String::new()));
                                    self.doc_op_result = api::doc_op_async("rename", serde_json::json!({"old": current_path, "new": new_path}));
                                } else if t.contains("新建目录") {
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
                    ui.weak(t!("loading"));
                }
            }
            "models" => {
                // 模型库独立板块（Mr2109 2026-08-27——排资源库上面）
                if self.res_type != "models" {
                    self.res_type = "models".to_string();
                    let store = self.resources.clone();
                    api::runtime().spawn(async move {
                        let r = api::fetch_resources_blocking("models".to_string()).await.ok();
                        *store.lock().unwrap() = r;
                    });
                }
                ui.heading(format!("{} 模型库", icon_text("computer-tower")));
                ui.add_space(4.0);
                ui.weak("按设备分组——模型按字母排序——点击查看简介（资源信任度 🆕=新入库）");
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
                        *store.lock().unwrap() = r;
                    });
                }
                ui.heading(t!("resources"));
                ui.add_space(4.0);
                // 3 库切换（模型库已独立板块——Mr2109 2026-08-27）
                ui.horizontal(|ui| {
                    let types = [("🔧 工具库", "tools"), ("📚 Skill 库", "skills"), ("🔌 MCP 库", "mcp")];
                    for (label, t) in types {
                        if ui.selectable_label(self.res_type == t, label).clicked() {
                            self.res_type = t.to_string();
                            // 拉对应类型
                            let store = self.resources.clone();
                            let rt = t.to_string();
                            api::runtime().spawn(async move {
                                let r = api::fetch_resources_blocking(rt).await.ok();
                                *store.lock().unwrap() = r;
                            });
                        }
                    }
                });
                ui.separator();
                // 左列表 + 右简介（模型点击显示——Mr2109 2026-08-20）
                ui.columns(2, |cols| {
                    // 左列——列表
                    if let Some(r) = self.resources.lock().unwrap().clone() {
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
                                            "x3" => "📊 X3",
                                            "local" => "💻 本机",
                                            "mini1" => "🍎 mini1",
                                            "mini2" => "🍎 mini2",
                                            _ => "❓ 未配置",
                                        };
                                        egui::CollapsingHeader::new(format!("{}（{} 个模型）", icon, names.len()))
                                            .id_salt(format!("res_models_{}", machine)) // APP-A08: 稳定 id
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
                                        let headers = [("名称", 0), ("版本", 1), ("状态", 2), ("调用次数", 3), ("故障", 4), ("入库时间", 5), ("简介", 6)];
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
                                            ui.label(if trust == "新" { "新" } else if trust == "正式" { "正式" } else { trust.as_str() });
                                            ui.label(format!("{} 次", uses));
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
                            cols[0].weak(t!("none"));
                        }
                    } else {
                        cols[0].spinner();
                        cols[0].weak(t!("loading"));
                    }
                    // 右列——模型简介
                    if self.res_type == "models" {
                        cols[1].heading("📖 模型简介");
                        if let Some(m) = &self.selected_model {
                            cols[1].label(format!("模型: {}", m));
                            cols[1].add_space(4.0);
                            if !self.selected_model_desc.is_empty() {
                                cols[1].label(&self.selected_model_desc);
                            } else {
                                cols[1].weak("（暂无简介——设备未配置描述）");
                            }
                        } else {
                            cols[1].weak("点击左侧模型查看简介");
                        }
                    } else {
                        cols[1].weak("工具/Skill/MCP 库——点击切换查看");
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
                    ui.label(format!("版本: {}", m.version));
                    ui.separator();
                    ui.add_space(8.0);
                    egui::Frame::new()
                        .fill(ui.visuals().faint_bg_color)
                        .corner_radius(8.0)
                        .inner_margin(egui::Margin::same(16))
                        .show(ui, |ui| {
                            ui.strong("🌍 生态集装箱（外部应用）");
                            ui.add_space(4.0);
                            ui.label("这是通过配置文件声明的第三方集装箱——当前为元数据占位（M4 阶段）。");
                            ui.label("v2.6 起支持：独立进程容器 + 标准适配层（实现 trait ZergModule）——真生态箱。");
                            ui.add_space(8.0);
                            if !m.url.is_empty() {
                                if ui.button("🔗 打开外部地址").clicked() {
                                    let _ = std::process::Command::new("open")
                                        .arg(&m.url)
                                        .spawn();
                                }
                            }
                        });
                } else {
                    // 未知/未启用模块——空舱（M2 模块管理后此处显示占位）
                    ui.weak("集装箱未启用或不存在");
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
            if let Some(r) = self.resources.lock().unwrap().clone() {
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
                                "x3" => "📊 X3",
                                "local" => "💻 本机",
                                "mini1" => "🍎 mini1",
                                "mini2" => "🍎 mini2",
                                _ => "❓ 未配置",
                            };
                            egui::CollapsingHeader::new(format!("{}（{} 个模型）", icon, names.len()))
                                .id_salt(format!("models_view_{}", machine)) // APP-A08: 稳定 id
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
                                            *store.lock().unwrap() = r;
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
                                                    edstore.lock().unwrap().insert(name2.clone(), init);
                                                }
                                            }
                                            *sstore.lock().unwrap() = s;
                                        });
                                        self.adapter_msg.clear();
                                    }
                                }
                            });
                        }
                    });
                } else {
                    l_ui.weak(t!("none"));
                }
            } else {
                l_ui.spinner();
                l_ui.weak(t!("loading"));
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
            r_ui.heading("📖 模型详情");
            if let Some(m) = &self.selected_model {
                r_ui.label(format!("模型: {}", m));
                r_ui.add_space(4.0);
                if !self.selected_model_desc.is_empty() {
                    r_ui.label(&self.selected_model_desc);
                } else {
                    r_ui.weak("（暂无简介——设备未配置描述）");
                }
                r_ui.add_space(8.0);
                r_ui.separator();
                // 适配器选项 + 启动状态（从 /api/models/{name} 拉）
                let detail = self.model_detail.lock().unwrap().clone();
                match detail {
                    Some(d) => {
                        let loaded = d.get("loaded").and_then(|v| v.as_bool()).unwrap_or(false);
                        let can_start = d.get("can_start").and_then(|v| v.as_bool()).unwrap_or(false);
                        let status = d.get("status").and_then(|v| v.as_str()).unwrap_or("?").to_string();
                        // 启动状态 + 开关
                        r_ui.horizontal(|ui| {
                            if loaded {
                                ui.colored_label(egui::Color32::from_rgb(80, 200, 120), format!("● {}（本机）", status));
                                if ui.button("🛑 停止").clicked() {
                                    // APP-A11: 结果不再 let _ = 丢弃——失败红字提示
                                    let name2 = m.clone();
                                    let store = self.model_detail.clone();
                                    let perr = self.poll_err.clone();
                                    api::runtime().spawn(async move {
                                        if let Err(e) = api::model_stop_blocking(&name2).await {
                                            *perr.lock().unwrap() = Some(format!("停止模型 {} 失败: {}", name2, e));
                                        }
                                        if let Ok(v) = api::fetch_model_detail_blocking(&name2).await {
                                            *store.lock().unwrap() = Some(v);
                                        }
                                    });
                                }
                            } else {
                                ui.colored_label(egui::Color32::from_rgb(200, 140, 80), format!("○ {}", status));
                                if can_start {
                                    if ui.button("▶ 启动").clicked() {
                                        // APP-A11: 失败红字提示
                                        let name2 = m.clone();
                                        let store = self.model_detail.clone();
                                        let perr = self.poll_err.clone();
                                        api::runtime().spawn(async move {
                                            if let Err(e) = api::model_start_blocking(&name2).await {
                                                *perr.lock().unwrap() = Some(format!("启动模型 {} 失败: {}", name2, e));
                                            }
                                            if let Ok(v) = api::fetch_model_detail_blocking(&name2).await {
                                                *store.lock().unwrap() = Some(v);
                                            }
                                        });
                                    }
                                } else {
                                    ui.weak("（远程设备——加载走 agent）");
                                }
                            }
                        });
                        r_ui.add_space(6.0);
                        // 标签页切换（Mr2109 2026-08-27——模型详情/适配器选项分开）
                        r_ui.horizontal(|ui| {
                            if ui.selectable_label(self.model_tab == "detail", "📄 模型详情").clicked() {
                                self.model_tab = "detail".to_string();
                            }
                            if ui.selectable_label(self.model_tab == "adapter", "🔧 适配器选项").clicked() {
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
                            opt(ui, "设备", d.get("host").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, "后端", d.get("backend").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, "模型文件", d.get("file").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, "内存 (GB)", format!("{}", d.get("mem_gb").and_then(|v| v.as_f64()).unwrap_or(0.0)));
                            opt(ui, "SSD 分流", if d.get("ssd").and_then(|v| v.as_bool()).unwrap_or(false) { "是" } else { "否" }.to_string());
                            opt(ui, "上下文", format!("{}", d.get("ctx_window").and_then(|v| v.as_i64()).unwrap_or(0)));
                            opt(ui, "架构", d.get("arch").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, "思考模型", if d.get("thinking").and_then(|v| v.as_bool()).unwrap_or(false) { "✅ 默认开 <think>" } else { "-" }.to_string());
                            opt(ui, "MoE 结构", d.get("moe").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, "模板", d.get("template").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, "模态", d.get("modality").and_then(|v| v.as_str()).unwrap_or("text").to_string());
                            if let Some(mp) = d.get("mmproj").and_then(|v| v.as_str()) {
                                if !mp.is_empty() {
                                    opt(ui, "视觉投影", mp.to_string());
                                }
                            }
                            opt(ui, "工具支持", match d.get("tool_support") {
                                Some(v) if v.as_bool() == Some(true) => "是".to_string(),
                                Some(v) if v.as_bool() == Some(false) => "否".to_string(),
                                _ => "未知".to_string(),
                            });
                            opt(ui, "接入日期", d.get("added").and_then(|v| v.as_str()).unwrap_or("-").to_string());
                            opt(ui, "已验证", if d.get("verified").and_then(|v| v.as_bool()).unwrap_or(false) { "✅" } else { "-" }.to_string());
                        });
                        // 启动参数（cmd/env）
                        if let Some(cmd) = d.get("cmd").and_then(|v| v.as_array()) {
                            if !cmd.is_empty() {
                                r_ui.add_space(4.0);
                                r_ui.label(format!("启动参数: {}", cmd.iter().filter_map(|c| c.as_str()).collect::<Vec<_>>().join(" ")));
                            }
                        }
                        if let Some(env) = d.get("env").and_then(|v| v.as_array()) {
                            if !env.is_empty() {
                                r_ui.label(format!("环境变量: {}", env.iter().filter_map(|c| c.as_str()).collect::<Vec<_>>().join(" ")));
                            }
                        }
                        } else {
                        // 适配器选项（真正的适配器配置——推理参数——可编辑——实时生效——Mr2109 2026-08-27）
                        let schema = self.adapter_schema.lock().unwrap().clone();
                        match schema {
                            Some(sv) => {
                                if let Some(arr) = sv.get("schema").and_then(|v| v.as_array()) {
                                    if arr.is_empty() {
                                        r_ui.weak("（无适配器——走旧路由）");
                                    } else {
                                        // 编辑缓冲（懒初始化）
                                        {
                                            let mut ed = self.adapter_edit.lock().unwrap();
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
                                        let cur = self.adapter_edit.lock().unwrap().get(m.as_str()).cloned().unwrap_or_default();
                                        let mut changed: Option<(String, serde_json::Value)> = None;
                                        // 垂直卡片流（Mr2109 2026-08-27——参数名+控件一行——描述弱字换行下一行——分隔线——不拥挤）
                                        // 滚动区自适应填满+应用按钮贴底（2026-08-27——max_height=可用高度-按钮区——不留空缺）
                                        let avail_h = r_ui.available_height();
                                        egui::ScrollArea::vertical().id_salt("adapter_edit_scroll").max_height((avail_h - 70.0).max(120.0)).show(r_ui, |ui| {
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
                                            self.adapter_edit.lock().unwrap().get_mut(m.as_str()).map(|eb| { eb.insert(k, v); });
                                        }
                                        // 应用按钮（两步确认——Mr2109 2026-08-27——修改后确认才生效）
                                        r_ui.add_space(6.0);
                                        if !self.adapter_confirm {
                                            r_ui.horizontal(|ui| {
                                                if ui.button("⚡ 应用（实时生效）").clicked() {
                                                    self.adapter_confirm = true;
                                                    self.adapter_msg.clear();
                                                }
                                                if !self.adapter_msg.is_empty() {
                                                    ui.colored_label(egui::Color32::from_rgb(80, 200, 120), &self.adapter_msg);
                                                }
                                            });
                                        } else {
                                            r_ui.horizontal(|ui| {
                                                ui.colored_label(egui::Color32::from_rgb(220, 180, 60), format!("{} 确认应用这些修改？", icon_text("warning")));
                                                if ui.button("✅ 确认生效").clicked() {
                                                    let cfg = self.adapter_edit.lock().unwrap().get(m.as_str()).cloned().unwrap_or_default();
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
                                                                edstore.lock().unwrap().insert(name2.clone(), init);
                                                            }
                                                        }
                                                        *sstore.lock().unwrap() = s;
                                                        let d = api::fetch_model_detail_blocking(&name2).await.ok();
                                                        *dstore.lock().unwrap() = d;
                                                    });
                                                    self.adapter_msg = "已生效".to_string();
                                                    self.adapter_confirm = false;
                                                }
                                                if ui.button("✖ 取消").clicked() {
                                                    self.adapter_confirm = false;
                                                    self.adapter_msg.clear();
                                                }
                                            });
                                        }
                                        r_ui.weak("每个模型的适配器参数集各自不同——编辑后需确认才生效（持久化重启恢复）");
                                    }
                                } else {
                                    r_ui.weak("（无适配器——走旧路由）");
                                }
                            }
                            None => {
                                r_ui.weak("（适配器选项加载中…）");
                            }
                        }
                        }
                    }
                    None => {
                        r_ui.weak("（适配器选项加载中…）");
                    }
                }
            } else {
                r_ui.weak("点击左侧模型查看详情");
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
    fn it_ctrl_async(start: bool) -> api::SharedResult<()> {
        let out: api::SharedResult<()> = Arc::new(Mutex::new(None));
        let out2 = out.clone();
        api::runtime().spawn(async move {
            let r = if start {
                api::start_internal_tasks_blocking().await
            } else {
                api::stop_internal_tasks_blocking().await
            };
            *out2.lock().unwrap() = Some(r);
        });
        out
    }

    /// 内部任务视图（Mr2109 2026-08-22——看到所有内部任务 + 手动执行按钮）
    fn internal_tasks_view(&mut self, ui: &mut egui::Ui) {
        ui.heading("🔧 内部任务（进化——为自己）");
        ui.add_space(4.0);
        ui.weak("16 类内部任务——编排自动运行——也可手动执行（空闲检测触发）。单槽铁律: 排队串行。");
        ui.add_space(8.0);
        // v2.5.6 内部任务启停按钮（Mr2109 2026-08-27——一个 toggle——点停止变启动/点启动变停止）
        // 显示状态: 根据后端 internal_stopped——UI 记录本地状态（点击后切换）
        ui.horizontal(|ui| {
            let stopped = self.internal_stopped;
            let label = if stopped { format!("{} 启动内部任务", icon_text("play")) } else { format!("{} 停止内部任务", icon_text("stop-circle")) };
            // APP-A07: 二次确认 + 结果回报（原来一次误点即启停、不成功也翻转本地状态）
            if self.it_ctrl_confirm {
                ui.colored_label(
                    egui::Color32::from_rgb(220, 180, 60),
                    format!("{} 确认{}内部任务引擎？", icon_text("warning"), if stopped { "启动" } else { "停止" }),
                );
                if ui.button("✅ 确认").clicked() {
                    self.it_ctrl_target = Some(!stopped);
                    self.it_ctrl_result = Self::it_ctrl_async(!stopped);
                    self.it_ctrl_confirm = false;
                }
                if ui.button("✖ 取消").clicked() {
                    self.it_ctrl_confirm = false;
                }
            } else if ui.button(label).clicked() {
                self.it_ctrl_confirm = true;
            }
            if stopped {
                ui.weak("（当前: 已停止——点启动恢复自动触发）");
            } else {
                ui.weak("（当前: 运行中——点停止暂停自动触发）");
            }
        });
        ui.add_space(8.0);
        // 拉取内部任务清单（缓存——每 30 秒刷新）
        if self.internal_tasks.lock().unwrap().is_none() || self.last_it_fetch.elapsed().as_secs() > 30 {
            let store = self.internal_tasks.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                let items = api::fetch_internal_tasks_blocking().await.unwrap_or_default();
                let mut st = store.lock().unwrap();
                *st = Some(items);
            });
            self.last_it_fetch = now;
        }
        // 周期配置（缓存——60 秒刷新）
        // APP-A06: 改用独立计时器（原来共用 last_it_fetch——被上面每 30s 的清单刷新归零，
        // 60s 条件永远不成立 → 周期下拉一直显示启动时拉的旧值）
        if self.it_intervals.lock().unwrap().is_none() || self.last_it_interval_fetch.elapsed().as_secs() > 60 {
            let store = self.it_intervals.clone();
            let now = std::time::Instant::now();
            api::runtime().spawn(async move {
                let v = api::fetch_internal_intervals_blocking().await.ok();
                let mut st = store.lock().unwrap();
                *st = v;
            });
            self.last_it_interval_fetch = now;
        }
        // 周期配置 map（defID → 小时）
        let interval_map: std::collections::HashMap<String, f64> = self
            .it_intervals
            .lock()
            .unwrap()
            .clone()
            .and_then(|v| v.get("intervals").cloned())
            .and_then(|v| v.as_object().cloned())
            .map(|obj| {
                obj.iter()
                    .filter_map(|(k, v)| v.as_f64().map(|h| (k.clone(), h)))
                    .collect()
            })
            .unwrap_or_default();
        let items = self.internal_tasks.lock().unwrap().clone().unwrap_or_default();
        if items.is_empty() {
            ui.weak("加载中…（或 API 不可用）");
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
                        if ui.button("▶ 执行").clicked() {
                            // APP-A11: 失败提示（原 let _ = 静默）
                            let id2 = id.clone();
                            let perr = self.poll_err.clone();
                            api::runtime().spawn(async move {
                                if let Err(e) = api::run_internal_task_blocking(&id2).await {
                                    *perr.lock().unwrap() = Some(format!("执行内部任务 {} 失败: {}", id2, e));
                                }
                            });
                        }
                        // 运行模式开关（Mr2109 2026-08-28——自动/手动——点击切换）
                        let auto_run = it.get("auto_run").and_then(|v| v.as_bool()).unwrap_or(true);
                        let mode_btn = if auto_run { "🔁 自动" } else { "✋ 手动" };
                        if ui.selectable_label(false, mode_btn).on_hover_text("运行模式: 自动=编排自动触发 / 手动=只手动触发（点击切换）").clicked() {
                            let id2 = id.clone();
                            let new_mode = !auto_run;
                            let perr = self.poll_err.clone();
                            api::runtime().spawn(async move {
                                // APP-A11: 失败提示（原 let _ = 静默）
                                if let Err(e) = api::set_internal_mode_blocking(&id2, new_mode).await {
                                    *perr.lock().unwrap() = Some(format!("切换运行模式 {} 失败: {}", id2, e));
                                }
                            });
                            // 本地立即翻转（后端刷新 30s 后同步）
                            if let Some(items_ref) = self.internal_tasks.lock().unwrap().as_mut() {
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
                                format!("⏱{}h", cur_h)
                            }
                        } else if def_h > 0.0 {
                            format!("⏱默认{}h", def_h as i64) // 默认执行周期
                        } else {
                            "⏱周期".to_string()
                        };
                        egui::ComboBox::from_id_salt(format!("it_iv_{}", id))
                            .selected_text(sel_text)
                            .width(70.0)
                            .show_ui(ui, |ui| {
                                for opt in [0.0_f64, 1.0, 2.0, 6.0, 12.0, 24.0] {
                                    let text = if opt == 0.0 {
                                        "✖ 不设置".to_string()
                                    } else {
                                        format!("{} 小时", opt as i64)
                                    };
                                    if ui.selectable_label(cur_h == opt, text).clicked() {
                                        let id2 = id.clone();
                                        let perr = self.poll_err.clone();
                                        api::runtime().spawn(async move {
                                            // APP-A11: 失败提示（原 let _ = 静默）
                                            if let Err(e) = api::set_internal_interval_blocking(&id2, opt).await {
                                                *perr.lock().unwrap() = Some(format!("设置周期 {} 失败: {}", id2, e));
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
                                    if ui.button("确定").clicked() {
                                        if let Ok(hv) = h.trim().parse::<f64>() {
                                            if hv > 0.0 {
                                                let id2 = id.clone();
                                                let perr = self.poll_err.clone();
                                                api::runtime().spawn(async move {
                                                    // APP-A11: 失败提示（原 let _ = 静默）
                                                    if let Err(e) = api::set_internal_interval_blocking(&id2, hv).await {
                                                        *perr.lock().unwrap() = Some(format!("设置指定周期 {} 失败: {}", id2, e));
                                                    }
                                                });
                                            }
                                        }
                                        self.it_custom_for = None;
                                        self.it_custom_hours.clear();
                                    }
                                } else if ui.selectable_label(false, format!("{} 指定…", icon_text("pencil-simple"))).clicked() {
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
                    r_ui.label(format!("说明: {}", desc));
                    r_ui.label(format!("冷却: {}", cooldown));
                    r_ui.add_space(4.0);
                    r_ui.separator();
                    r_ui.strong("任务模板:");
                    r_ui.label(template);
                    r_ui.add_space(8.0);
                    r_ui.separator();
                    r_ui.strong("Skill 内容（该类任务沉淀——执行中进化）:");
                    if skill.is_empty() {
                        r_ui.weak("（暂无 skill——该类任务首次执行时由模型自举建立）");
                    } else {
                        egui::ScrollArea::vertical().id_salt("it_right_skill").max_height(320.0).show(&mut r_ui, |ui| {
                            ui.label(skill);
                        });
                    }
                }
                None => {
                    r_ui.weak("← 点击左侧任务查看详情和 skill");
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
        let mut p = std::path::PathBuf::from("/tmp/zerg-tasks");
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
            eprintln!("[zerg-ui] 布局持久化失败 {}: {}", path.display(), e);
        }
    }
}

// 布局比例持久化（Mr2109 2026-08-27——左右分割——拖动后下次启动默认）
fn load_layout_ratio(key: &str, default: f32) -> f32 {
    let mut p = std::path::PathBuf::from("/tmp/zerg-tasks");
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
        ui.ctx().request_repaint();

        if !self.online {
            egui::Panel::top("status").show(ui, |ui| {
                ui.horizontal(|ui| {
                    ui.colored_label(
                        egui::Color32::from_rgb(220, 80, 80),
                        t!("offline_waiting"),
                    );
                });
            });
            egui::CentralPanel::default().show(ui, |ui| {
                ui.centered_and_justified(|ui| {
                    ui.label(
                        egui::RichText::new(t!("offline_full"))
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
            let notice = self.poll_err.lock().unwrap().clone();
            if let Some(msg) = notice {
                ui.horizontal(|ui| {
                    ui.colored_label(egui::Color32::from_rgb(230, 90, 90), format!("⚠ {}", msg));
                    if ui.small_button("✕ 清除").clicked() {
                        *self.poll_err.lock().unwrap() = None;
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

/// 任务 ID 缩短显示
fn short_id(id: &str) -> &str {
    if id.len() > 6 {
        &id[id.len() - 6..]
    } else {
        id
    }
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
        (Some(c), _) if c < 0 => "未知".to_string(),
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
        (Some(_), None) => "进行中".to_string(),
        _ => "—".to_string(),
    }
}

/// 内部任务视图（Mr2109 2026-08-22——看到所有内部任务 + 手动执行按钮）
/// 分支名通俗化（task-internal-health-check-1787... → 内部任务·健康巡检）
fn humanize_branch(branch: &str) -> String {
    let b = branch.trim();
    if b == "main" {
        return "main（主分支）".to_string();
    }
    // task-xxx-1787... 格式——提取类型
    if let Some(idx) = b.find("task-") {
        let rest = &b[idx + 5..];
        // 去掉时间戳（最后一段数字）
        let parts: Vec<&str> = rest.split('-').collect();
        if parts.len() >= 2 {
            // 内部任务（internal-类型）
            if parts[0] == "internal" && parts.len() >= 3 {
                let task_type = match parts[1] {
                    "health" => "系统健康巡检",
                    "tool" => "工具库检查",
                    "code" => "代码质量",
                    "knowledge" => "经验沉淀",
                    _ => parts[1],
                };
                return format!("内部任务·{}", task_type);
            }
            // 外部任务（task-xxx）
            return format!("外部任务·{}", parts[0]);
        }
        return rest.to_string();
    }
    b.to_string()
}


impl ZergApp {
    /// HUD 悬浮层（挂起清单 ③——最小实现：core 状态点 + 当前模块 + running 任务数）
    fn hud_view(&mut self, ctx: &egui::Context) {
        // 数据：当前模块名 + running 任务数（复用现有 tasks——不新拉）
        let mod_name: String = self
            .registry
            .modules
            .iter()
            .find(|m| m.id == self.registry.active)
            .map(|m| m.name.to_string())
            .unwrap_or_else(|| self.registry.active.clone());
        let running = self
            .tasks
            .lock()
            .unwrap()
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
                                    egui::RichText::new(format!("⚙ {} 运行中", running)).size(11.0),
                                );
                            } else {
                                ui.weak(egui::RichText::new("空闲").size(11.0));
                            }
                            if ui.small_button("✕").on_hover_text("隐藏 HUD——右上角仪表图标重新显示").clicked() {
                                self.hud_hidden = true;
                            }
                        });
                    });
            });
    }

}

