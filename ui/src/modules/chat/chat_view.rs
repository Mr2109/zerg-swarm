//! 对话视图（v2.5.7——借鉴 Hermes 桌面 UI——Mr2109完全借鉴）
//! 结构: 左侧会话列表 + 右侧消息流 + 底部输入框——思考分离显示（C5 完善）

use eframe::egui;
use crate::modules::icons::icon_text; // P3 图标（iconflow——替换 emoji）
use serde_json::Value;
use std::sync::{Arc, Mutex};

use crate::api;
use egui_commonmark::CommonMarkViewer; // P4-23 消息 Markdown 渲染（借文档查看模式 egui_commonmark——Mr2109: 文档实现方式借用到对话）

// P4-25 md 预处理（补 egui_commonmark 不支持的格式）：
// ① ==高亮==（GFM highlight——pulldown-cmark 不支持）→ **加粗**（视觉近似）
// ② <sup>/<sub> 上下标标签剥离（InlineHtml 不渲染——剥标签留内容）
fn md_preprocess(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    let chars: Vec<char> = s.chars().collect();
    let n = chars.len();
    let mut i = 0;
    while i < n {
        // ==高亮== → **加粗**（配对）
        if i + 1 < n && chars[i] == '=' && chars[i + 1] == '=' {
            // 找配对 ==
            let mut j = i + 2;
            while j + 1 < n && !(chars[j] == '=' && chars[j + 1] == '=') {
                j += 1;
            }
            if j + 1 < n {
                out.push_str("**");
                i += 2;
                while i < j {
                    out.push(chars[i]);
                    i += 1;
                }
                out.push_str("**");
                i += 2; // 跳过收尾 ==
                continue;
            }
        }
        // <sup>/<sub>/</sup>/</sub> 标签剥离
        if chars[i] == '<' {
            let rest: String = chars[i..].iter().collect();
            if rest.starts_with("<sup>") || rest.starts_with("<sub>") {
                i += 5;
                continue;
            }
            if rest.starts_with("</sup>") || rest.starts_with("</sub>") {
                i += 6;
                continue;
            }
        }
        out.push(chars[i]);
        i += 1;
    }
    out
}

// P4-26 数学公式渲染（KaTeX CLI → SVG → egui Image——egui_commonmark render_math_fn）
// 缓存：公式 → SVG 字节（npx 启动慢——只首次渲染，缓存后即时）
static MATH_CACHE: std::sync::Mutex<Option<std::collections::HashMap<String, Vec<u8>>>> =
    std::sync::Mutex::new(None);

fn render_math(ui: &mut egui::Ui, math: &str, inline: bool) {
    let mut cache = MATH_CACHE.lock().unwrap();
    let map = cache.get_or_insert_with(std::collections::HashMap::new);
    let svg_bytes = if let Some(b) = map.get(math) {
        b.clone()
    } else {
        // P4-26b mathjax-full 脚本（KaTeX CLI 的 --format svg 实际输出 HTML 无 <svg>——
        // 换 mathjax-full tex2svg.js 输出标准 SVG——resvg 可加载）
        let mut cmd = std::process::Command::new("node");
        cmd.arg(std::env::var("HOME").unwrap_or_default() + "/.zerg-math/tex2svg.js");
        if !inline {
            cmd.arg("display");
        }
        cmd.stdin(std::process::Stdio::piped())
            .stdout(std::process::Stdio::piped())
            .stderr(std::process::Stdio::null());
        let mut child = match cmd.spawn() {
            Ok(c) => c,
            Err(_) => {
                ui.weak(format!("${}$", math));
                return;
            }
        };
        if let Some(mut stdin) = child.stdin.take() {
            use std::io::Write;
            let _ = stdin.write_all(math.as_bytes());
        }
        let out = child.wait_with_output();
        let svg = match out {
            Ok(o) if o.status.success() => String::from_utf8_lossy(&o.stdout).to_string(),
            _ => String::new(),
        };
        if svg.trim().is_empty() || !svg.contains("<svg") {
            ui.weak(format!("${}$", math));
            return;
        }
        // P4-27 暗色主题适配：mathjax SVG 默认黑色（fill 继承）——在 <svg> 标签注入浅色
        let svg = svg.trim();
        let svg = if let Some(pos) = svg.find('>') {
            let tag = &svg[..pos];
            let rest = &svg[pos..];
            if tag.contains("fill=") {
                svg.to_string()
            } else {
                format!("{} fill={:?}{}", tag, "#c9c9c9", rest)
            }
        } else {
            svg.to_string()
        };
        let bytes = svg.as_bytes().to_vec();
        map.insert(math.to_string(), bytes.clone());
        bytes
    };
    let uri: String = format!("math-{}.svg", math.len());
    let img = egui::Image::from_bytes(
        std::borrow::Cow::Owned(uri),
        egui::load::Bytes::Shared(std::sync::Arc::from(svg_bytes.as_slice())),
    );
    if inline {
        ui.add(img.fit_to_original_size(1.0));
    } else {
        ui.with_layout(egui::Layout::top_down(egui::Align::Center), |ui| {
            ui.add(img.fit_to_original_size(1.0));
        });
    }
}

/// 对话视图状态
pub struct ChatView {
    // 会话列表
    pub sessions: Vec<Value>,
    sessions_loading: bool,
    // 当前会话
    pub active_session: Option<String>,
    pub messages: Vec<Value>,
    messages_loading: bool,
    // 输入
    pub input: String,
    pub sending: bool,
    pub send_error: Option<String>,
    // 异步句柄
    sessions_pending: Option<api::SharedResult<Vec<Value>>>,
    session_pending: Option<api::SharedResult<Value>>,
    create_pending: Option<api::SharedResult<Value>>,
    // C3 流式发送（生成中状态）
    stream: Option<api::SharedChatStream>,
    streaming: bool,
    stream_content: String,
    stream_reasoning: String,
    // P4-35 工具执行中状态（tool_start 事件）——等待反馈
    stream_tool: Option<String>,
    stream_compacting: bool,
    stream_tool_elapsed: usize,
    stream_started: f64,
    // C5 模型胶囊（当前模型 + 可用列表 + 切换）
    pub current_model: String,
    models: Vec<String>,
    models_pending: Option<api::SharedResult<Vec<String>>>,
    model_update_pending: Option<api::SharedResult<Value>>,
    // C5 思考折叠（消息 id → 展开）
    thinking_open: std::collections::HashSet<i64>,
    // C6 会话搜索
    search_query: String,
    search_results: Vec<Value>,
    search_pending: Option<api::SharedResult<Value>>,
    // C7 派单
    delegate_pending: Option<api::SharedResult<Value>>,
    // D3 多模态：待发送图片（data URL + 文件名）
    pending_image: Option<String>,
    pending_image_name: String,
    // D4 固定
    pin_pending: Option<api::SharedResult<Value>>,
    archive_pending: Option<api::SharedResult<Value>>,
    // P4-23 消息 markdown 渲染缓存（msg_id → cache——文档查看模式同款 CommonMarkViewer）
    pub msg_md_cache: std::collections::HashMap<i64, egui_commonmark::CommonMarkCache>,
    // P4-29 消息区虚拟列表（长对话性能——egui_virtual_list——变高行+懒算高度缓存）
    pub vlist: egui_virtual_list::VirtualList,
    // P4-10 重命名（行内编辑——Hermes rename 借鉴）
    renaming_id: Option<String>,
    rename_text: String,
    rename_pending: Option<api::SharedResult<Value>>,
    // P0 点击编辑（Hermes user-edit 借鉴）
    editing_id: Option<i64>,
    editing_content: String,
    edit_pending: Option<api::SharedResult<Value>>,
    // P1 输入历史（Hermes 借鉴——↑↓ 浏览历史输入）
    input_history: Vec<String>,
    history_idx: Option<usize>,
    // P1 草稿持久化（Hermes 借鉴——会话级草稿——切换不丢）
    drafts: std::collections::HashMap<String, String>,
    // P1 斜杠命令菜单
    slash_open: bool,
    // P2 多图（Vec<(dataURL, name)>——Hermes 多附件借鉴）
    pending_images: Vec<(String, String)>,
    // P2 表情反应（消息 id → emoji——Hermes message-reactions 借鉴——本地状态）
    reactions: std::collections::HashMap<i64, String>,
    reacting_id: Option<i64>,
    // P2 语音朗读中
    speaking_id: Option<i64>,
    // P2-3 侧栏宽度（可拖拽——egui 拖拽分栏）
    sidebar_w: f32,
}

impl Default for ChatView {
    fn default() -> Self {
        Self::new()
    }
}

/// 消息操作（C7——复制/重试/派单）
#[derive(Clone)]
enum MsgAction {
    Copy(String),
    Retry(String),
    Delegate(String),
}

impl ChatView {
    pub fn new() -> Self {
        let mut v = Self {
            sessions: Vec::new(),
            sessions_loading: false,
            active_session: None,
            messages: Vec::new(),
            messages_loading: false,
            input: String::new(),
            sending: false,
            send_error: None,
            sessions_pending: None,
            session_pending: None,
            create_pending: None,
            stream: None,
            streaming: false,
            stream_content: String::new(),
            stream_reasoning: String::new(),
            stream_tool: None,
            stream_tool_elapsed: 0,
            stream_compacting: false,
            stream_started: 0.0,
            current_model: "example-35b-v2".to_string(), // 默认对话模型（Mr2109）
            models: Vec::new(),
            models_pending: None,
            model_update_pending: None,
            thinking_open: std::collections::HashSet::new(),
            search_query: String::new(),
            search_results: Vec::new(),
            search_pending: None,
            delegate_pending: None,
            pending_image: None,
            pending_image_name: String::new(),
            pin_pending: None,
            archive_pending: None,
            msg_md_cache: std::collections::HashMap::new(),
            vlist: {
                let mut v = egui_virtual_list::VirtualList::new();
                v.over_scan(200.0);
                v
            },
            renaming_id: None,
            rename_text: String::new(),
            rename_pending: None,
            editing_id: None,
            editing_content: String::new(),
            edit_pending: None,
            input_history: Vec::new(),
            history_idx: None,
            drafts: std::collections::HashMap::new(),
            slash_open: false,
            pending_images: Vec::new(),
            reactions: std::collections::HashMap::new(),
            reacting_id: None,
            speaking_id: None,
            sidebar_w: 200.0,
        };
        v.refresh_sessions();
        v.models_pending = Some(api::fetch_available_models_async());
        v
    }

    /// 刷新会话列表（异步）
    pub fn refresh_sessions(&mut self) {
        self.sessions_loading = true;
        self.sessions_pending = Some(api::fetch_chat_sessions_async());
    }

    /// 新建会话
    pub fn new_session(&mut self) {
        // P4-10 用当前模型创建（不写死）
        self.create_pending = Some(api::create_chat_session_async(self.current_model.clone()));
    }

    /// 打开会话（加载消息）
    pub fn open_session(&mut self, id: String) {
        // P1 草稿持久化（切换前保存旧会话草稿）
        if let Some(prev) = self.active_session.clone() {
            let cur = std::mem::take(&mut self.input);
            self.drafts.insert(prev, cur);
        }
        self.active_session = Some(id.clone());
        // P4-10 切换会话立即清空旧消息（防显示旧会话内容直到新消息返回）
        self.messages.clear();
        self.stream_content.clear();
        self.stream_reasoning.clear();
        // P1 恢复新会话草稿
        self.input = self.drafts.get(&id).cloned().unwrap_or_default();
        self.messages_loading = true;
        self.session_pending = Some(api::fetch_chat_session_async(id));
        self.slash_open = false;
        self.editing_id = None;
    }

    /// 切换模型（C5 胶囊——更新会话模型）
    pub fn switch_model(&mut self, model: String) {
        if self.current_model == model {
            return;
        }
        self.current_model = model.clone();
        if let Some(sid) = self.active_session.clone() {
            self.model_update_pending = Some(api::chat_update_model_async(sid, model));
        }
    }

    /// 发送消息（C3 流式——乐观显示 user + 流式接收 assistant）
    pub fn send(&mut self) {
        let content = self.input.trim().to_string();
        if content.is_empty() || self.streaming {
            return;
        }
        let Some(sid) = self.active_session.clone() else {
            self.send_error = Some("请先新建或选择会话".to_string());
            return;
        };
        // 立即显示用户消息（乐观更新）
        self.messages.push(Value::Object(
            [
                ("role".to_string(), Value::String("user".to_string())),
                ("content".to_string(), Value::String(content.clone())),
            ]
            .into_iter()
            .collect(),
        ));
        // P1 输入历史（记录发送内容——去重最近一条）
        if !content.is_empty() {
            if self.input_history.last() != Some(&content) {
                self.input_history.push(content.clone());
                if self.input_history.len() > 50 {
                    self.input_history.remove(0);
                }
            }
        }
        self.history_idx = None;
        self.slash_open = false;
        self.input.clear();
        self.streaming = true;
        self.stream_content.clear();
        self.stream_reasoning.clear();
        self.send_error = None;
        // P2 多图（Vec——兼容旧单图字段）
        let imgs: Vec<String> = self.pending_images.iter().map(|(d, _)| d.clone()).collect();
        let img_single = if imgs.len() == 1 {
            Some(imgs[0].clone())
        } else {
            None
        };
        self.pending_images.clear();
        self.stream = Some(api::chat_send_stream_async(sid, content, img_single, imgs));
        self.stream_started = now_f64(); // P4-35 等待计时起点
        self.stream_started = now_f64();
    }

    /// 选择图片（D3/P2——rfd 文件对话框 → base64 data URL——多图追加）
    pub fn pick_image(&mut self) {
        if let Some(path) = rfd::FileDialog::new()
            .add_filter("图片", &["png", "jpg", "jpeg", "gif", "webp"])
            .pick_file()
        {
            if let Ok(bytes) = std::fs::read(&path) {
                let ext = path
                    .extension()
                    .and_then(|e| e.to_str())
                    .unwrap_or("png")
                    .to_string();
                let name = path
                    .file_name()
                    .and_then(|n| n.to_str())
                    .unwrap_or("图片")
                    .to_string();
                let b64 = base64_std(&bytes);
                let data_url = format!("data:image/{};base64,{}", ext, b64);
                if self.pending_images.len() < 8 {
                    self.pending_images.push((data_url, name));
                }
            }
        }
    }

    /// 停止生成（C3——断开连接——后端 ctx cancel）
    pub fn stop(&mut self) {
        if let Some(s) = self.stream.clone() {
            s.lock().unwrap().cancelled = true;
        }
        self.stream = None;
        self.streaming = false;
        // 停止后重新拉会话（拿已生成的部分——后端可能已存）
        if let Some(sid) = self.active_session.clone() {
            self.session_pending = Some(api::fetch_chat_session_async(sid));
        }
    }

    /// 每帧轮询异步结果
    pub fn poll(&mut self) {
        // 会话列表
        let pending = self.sessions_pending.take();
        if let Some(p) = pending {
            let done = p.lock().unwrap().clone();
            if let Some(res) = done {
                self.sessions_loading = false;
                match res {
                    Ok(list) => {
                        self.sessions = list;
                        // P4-1 初次加载自动打开最近一次对话（列表倒序——第一个=最近）
                        if self.active_session.is_none() {
                            if let Some(first) = self.sessions.first() {
                                if let Some(sid) = first.get("id").and_then(|x| x.as_str()) {
                                    self.active_session = Some(sid.to_string());
                                    self.session_pending = Some(api::fetch_chat_session_async(sid.to_string()));
                                }
                            }
                        }
                    }
                    Err(_) => {}
                }
            } else {
                self.sessions_pending = Some(p);
            }
        }
        // 会话详情
        let pending = self.session_pending.take();
        if let Some(p) = pending {
            let done = p.lock().unwrap().clone();
            if let Some(res) = done {
                self.messages_loading = false;
                match res {
                    Ok(v) => {
                        if let Some(msgs) = v.get("messages").and_then(|m| m.as_array()) {
                            self.messages = msgs.clone();
                        }
                        // C5 会话模型 → 胶囊
                        if let Some(m) = v.get("model").and_then(|x| x.as_str()) {
                            if !m.is_empty() {
                                self.current_model = m.to_string();
                            }
                        }
                    }
                    Err(_) => {}
                }
            } else {
                self.session_pending = Some(p);
            }
        }
        // C5 模型列表
        let pending = self.models_pending.take();
        if let Some(p) = pending {
            let done = p.lock().unwrap().clone();
            if let Some(res) = done {
                match res {
                    Ok(list) => self.models = list,
                    Err(_) => {}
                }
            } else {
                self.models_pending = Some(p);
            }
        }
        // C5 模型切换结果
        let pending = self.model_update_pending.take();
        if let Some(p) = pending {
            let done = p.lock().unwrap().clone();
            if let Some(res) = done {
                if let Err(e) = res {
                    self.send_error = Some(format!("模型切换失败: {}", e));
                }
            } else {
                self.model_update_pending = Some(p);
            }
        }
        // C6 会话搜索
        let pending = self.search_pending.take();
        if let Some(p) = pending {
            let done = p.lock().unwrap().clone();
            if let Some(res) = done {
                match res {
                    Ok(v) => {
                        if let Some(arr) = v.as_array() {
                            self.search_results = arr.clone();
                        }
                    }
                    Err(_) => {}
                }
            } else {
                self.search_pending = Some(p);
            }
        }
        // C7 派单结果
        let pending = self.delegate_pending.take();
        if let Some(p) = pending {
            let done = p.lock().unwrap().clone();
            if let Some(res) = done {
                match res {
                    Ok(v) => {
                        let id = v.get("id").and_then(|x| x.as_str()).unwrap_or("");
                        self.send_error = Some(format!("{} 已派单（任务 {}）", icon_text("rocket-launch"), id));
                    }
                    Err(e) => self.send_error = Some(format!("派单失败: {}", e)),
                }
            } else {
                self.delegate_pending = Some(p);
            }
        }
        // P0 编辑结果（成功后刷新会话消息）
        let ep = self.edit_pending.take();
        if let Some(p) = ep {
            let done = p.lock().unwrap().clone();
            if let Some(res) = done {
                match res {
                    Ok(_) => {
                        if let Some(sid) = self.active_session.clone() {
                            self.session_pending = Some(api::fetch_chat_session_async(sid));
                        }
                    }
                    Err(e) => self.send_error = Some(format!("编辑失败: {}", e)),
                }
            } else {
                self.edit_pending = Some(p);
            }
        }
        // C3 流式发送轮询（生成中——每帧读 stream state）
        if self.streaming {
            let s = self.stream.clone();
            if let Some(s) = s {
                let st = s.lock().unwrap();
                self.stream_content = st.content.clone();
                self.stream_reasoning = st.reasoning.clone();
                self.stream_tool = st.tool_name.clone(); // P4-35 工具执行中状态
                self.stream_tool_elapsed = st.tool_elapsed; // P4-36 心跳计时
                self.stream_compacting = st.compacting; // P4-39 压缩进行中
                if let Some(e) = st.error.clone() {
                    self.send_error = Some(e);
                }
                if st.done {
                    drop(st);
                    self.stream = None;
                    self.streaming = false;
                    // 流结束——重拉会话（拿完整消息 + 标题）
                    if let Some(sid) = self.active_session.clone() {
                        self.session_pending = Some(api::fetch_chat_session_async(sid));
                    }
                }
            }
        }
        // P4-10 重命名结果
        let pending = self.rename_pending.take();
        if let Some(p) = pending {
            let done = p.lock().unwrap().clone();
            if let Some(res) = done {
                match res {
                    Ok(_) => {
                        self.renaming_id = None;
                        self.refresh_sessions();
                    }
                    Err(_) => {}
                }
            } else {
                self.rename_pending = Some(p);
            }
        }
        // 新建会话
        let pending = self.create_pending.take();
        if let Some(p) = pending {
            let done = p.lock().unwrap().clone();
            if let Some(res) = done {
                match res {
                    Ok(v) => {
                        if let Some(id) = v.get("id").and_then(|i| i.as_str()) {
                            self.open_session(id.to_string());
                            self.refresh_sessions();
                        }
                    }
                    Err(_) => {}
                }
            } else {
                self.create_pending = Some(p);
            }
        }
    }

    /// 渲染对话视图（主区）
    pub fn render(&mut self, ui: &mut egui::Ui) {
        self.poll();
        ui.heading(format!("{} 对话", icon_text("message-circle")));
        ui.add_space(4.0);
        let avail = ui.available_size();
        let sidebar_w = self.sidebar_w.clamp(140.0, avail.x * 0.5);
        ui.horizontal(|ui| {
            // ── 左侧会话列表 ──
            let (srect, _) = ui.allocate_exact_size(egui::vec2(sidebar_w, avail.y), egui::Sense::hover());
            let mut s_ui = ui.new_child(
                egui::UiBuilder::new()
                    .max_rect(srect)
                    .layout(egui::Layout::top_down(egui::Align::Min)),
            );
            self.render_sidebar(&mut s_ui);
            // ── 可拖拽分隔线（P2-3——左右分栏拖动）──
            let sep_resp = ui.separator().interact(egui::Sense::drag());
            if sep_resp.dragged() {
                let delta = sep_resp.drag_delta().x;
                self.sidebar_w = (self.sidebar_w + delta).clamp(140.0, avail.x * 0.5);
            }
            if sep_resp.hovered() || sep_resp.dragged() {
                ui.output_mut(|o| o.cursor_icon = egui::CursorIcon::ResizeHorizontal);
            }
            // ── 右侧消息区 ──
            let main_w = (avail.x - self.sidebar_w - 16.0).max(300.0);
            let (mrect, _) = ui.allocate_exact_size(egui::vec2(main_w, avail.y), egui::Sense::hover());
            // 消息区间隙（Mr2109 2026-09-01 定）：左 10 / 右 20（分割线紧贴——右边多留白）
            let inner_w = (mrect.width() - 30.0).max(240.0);
            let inner_rect = egui::Rect::from_min_size(
                egui::pos2(mrect.left() + 10.0, mrect.top()),
                egui::vec2(inner_w, mrect.height()),
            );
            let mut m_ui = ui.new_child(
                egui::UiBuilder::new()
                    .max_rect(inner_rect)
                    .layout(egui::Layout::top_down(egui::Align::Min)),
            );
            self.render_messages(&mut m_ui);
        });
    }

    /// 渲染左侧会话列表
    fn render_sidebar(&mut self, ui: &mut egui::Ui) {
        ui.horizontal(|ui| {
            if ui.button(format!("{} 新会话", icon_text("plus"))).clicked() {
                self.new_session();
            }
            if ui.button(icon_text("arrows-clockwise")).on_hover_text("刷新").clicked() {
                self.refresh_sessions();
            }
        });
        ui.separator();
        // 会话搜索框（C6——FTS5——输入即搜）
        let mut do_search = false;
        ui.horizontal(|ui| {
            let resp = ui.add(
                egui::TextEdit::singleline(&mut self.search_query)
                    .hint_text(format!("{} 搜索会话…", icon_text("magnifying-glass")))
                    .desired_width(140.0),
            );
            if resp.changed() {
                do_search = !self.search_query.trim().is_empty();
            }
            if !self.search_query.trim().is_empty() && ui.button("✕").clicked() {
                self.search_query.clear();
                self.search_results.clear();
            }
        });
        if do_search {
            let q = self.search_query.trim().to_string();
            if !q.is_empty() {
                self.search_pending = Some(api::chat_search_async(q));
            }
        }
        // 搜索结果（显示在会话列表上方）
        if !self.search_results.is_empty() {
            let mut jump: Option<String> = None;
            ui.separator();
            egui::ScrollArea::vertical()
                .id_salt("chat_search_results")
                .auto_shrink(false)
                .max_height(150.0)
                .show(ui, |ui| {
                    for r in &self.search_results {
                        let sid = r.get("session_id").and_then(|x| x.as_str()).unwrap_or("");
                        let snip = r.get("content").and_then(|x| x.as_str()).unwrap_or("");
                        let label = format!("{} {}", icon_text("magnifying-glass"), truncate(snip, 40));
                        if ui.selectable_label(false, label).clicked() {
                            jump = Some(sid.to_string());
                        }
                    }
                });
            if let Some(sid) = jump {
                self.open_session(sid);
                self.search_results.clear();
            }
        }
        if self.sessions_loading && self.sessions.is_empty() {
            ui.spinner();
            ui.weak("加载中...");
        }
        // 会话列表（分组：📌 已置顶 / 💬 会话——Hermes sidebar 借鉴——P0）
        let sessions = self.sessions.clone();
        let mut selected: Option<String> = None;
        let mut deleted: Option<String> = None;
        let mut delegate_sid: Option<(String, String)> = None; // v2.5.7 对话→任务: 会话级派任务（id+标题）
        let mut pinned: Option<(String, bool)> = None;
        let mut archived: Option<String> = None;
        let mut renaming: Option<(String, String)> = None;
        let mut rename_commit: Option<(String, String)> = None;
        let streaming_now = self.streaming;
        let active_sid = self.active_session.clone();
        let mut render_group = |ui: &mut egui::Ui, group: &str, items: Vec<&Value>| {
            if items.is_empty() {
                return;
            }
            ui.weak(egui::RichText::new(group).size(10.0));
            ui.add_space(2.0);
            for s in items {
                let id = s.get("id").and_then(|i| i.as_str()).unwrap_or("");
                let title = s.get("title").and_then(|t| t.as_str()).unwrap_or("新会话");
                let is_active = active_sid.as_deref() == Some(id);
                let label = if title.is_empty() { "新会话" } else { title };
                let ts = s
                    .get("updated_at")
                    .and_then(|t| t.as_f64())
                    .or_else(|| s.get("last_active").and_then(|t| t.as_f64()))
                    .unwrap_or(0.0);
                let age = format_age(ts);
                let abs = format_abs_time(ts);
                let running = streaming_now && is_active;
                // P4-7 会话行（Mr2109 2026-08-31）：单行左对齐——不显示模型行——hover 显示标题+模型+建立时间
                let row_h = 24.0;
                let (row_rect, row_resp) = ui.allocate_exact_size(
                    egui::vec2(ui.available_width(), row_h),
                    egui::Sense::click(),
                );
                // 调试：验证行左对齐（P4-8——只打印前3行）
                static mut DEBUG_CNT: u32 = 0;
                let mut dbg_cnt = 0;
                unsafe { dbg_cnt = DEBUG_CNT; }
                if dbg_cnt < 3 {
                    println!("[zerg-ui] 行 rect: left={:.1} top={:.1} w={:.1}", row_rect.left(), row_rect.top(), row_rect.width());
                    unsafe { DEBUG_CNT += 1; }
                }
                ui.add_space(3.0); // 队列间间距（Mr2109 2026-08-31——每个队列有点间距）
                let left = row_rect.left() + 4.0;
                // 选中/悬停背景
                if is_active {
                    ui.painter()
                        .rect_filled(row_rect, 4.0, ui.visuals().selection.bg_fill);
                } else if row_resp.hovered() {
                    ui.painter()
                        .rect_filled(row_rect, 4.0, ui.visuals().widgets.hovered.weak_bg_fill);
                }
                // 运行状态点（生成中 ● 色点）
                if running {
                    ui.painter().circle_filled(
                        egui::pos2(left - 7.0, row_rect.top() + 7.0),
                        3.5,
                        egui::Color32::from_rgb(250, 180, 60),
                    );
                }
                // 标题（P4-9 painter 绝对坐标左对齐 + 字体度量精确截断——保证贴左不居中）
                let age_w = if !age.is_empty() { 42.0 } else { 4.0 };
                let title_w = (row_rect.width() - 12.0 - age_w - 6.0).max(30.0);
                let title_font = egui::FontId::proportional(12.0);
                let title_color = ui.visuals().text_color();
                // P4-10 行内重命名（TextEdit 替换标题）
                let is_renaming = self.renaming_id.as_deref() == Some(id);

                if is_renaming {
                    let te = egui::TextEdit::singleline(&mut self.rename_text)
                        .desired_width(title_w)
                        .font(egui::TextStyle::Body);
                    let r = ui.put(egui::Rect::from_min_size(
                        egui::pos2(left, row_rect.top() + 2.0),
                        egui::vec2(title_w, 18.0),
                    ), te);
                    if r.lost_focus()
                        && ui.input(|i| i.key_pressed(egui::Key::Enter))
                    {
                        rename_commit = Some((id.to_string(), self.rename_text.trim().to_string()));
                    }
                    if ui.input(|i| i.key_pressed(egui::Key::Escape)) {
                        self.renaming_id = None;
                    }
                }
                let mut title_text = label.to_string();
                // 字体度量精确截断（egui galley 测量——中英文混排宽度准确）
                let measured = ui.fonts_mut(|f| f.layout_no_wrap(title_text.clone(), title_font.clone(), title_color).size().x);
                if measured > title_w {
                    let mut lo = 0usize;
                    let mut hi = label.chars().count();
                    while lo < hi {
                        let mid = (lo + hi + 1) / 2;
                        let t: String = label.chars().take(mid).collect();
                        let w = ui.fonts_mut(|f| f.layout_no_wrap(t, title_font.clone(), title_color).size().x);
                        if w <= title_w - 12.0 { lo = mid; } else { hi = mid - 1; }
                    }
                    let mut out: String = label.chars().take(lo).collect();
                    out.push('…');
                    title_text = out;
                }
                if !is_renaming {
                    ui.painter().text(
                        egui::pos2(left, row_rect.top() + 2.0),
                        egui::Align2::LEFT_TOP,
                        title_text,
                        title_font,
                        title_color,
                    );
                }
                // 相对时间（右侧对齐）
                if !age.is_empty() {
                    ui.painter().text(
                        egui::pos2(row_rect.right() - 4.0, row_rect.top() + 7.0),
                        egui::Align2::RIGHT_TOP,
                        age,
                        egui::FontId::proportional(10.0),
                        ui.visuals().weak_text_color(),
                    );
                }
                if row_resp.clicked() {
                    selected = Some(id.to_string());
                }
                // P4-7 hover 显示详细内容：标题 + 调用模型 + 建立时间（Mr2109 2026-08-31）
                // P4-32 加 source 显示（desktop/cron/agent——Hermes source 对齐）
                let model = s.get("model").and_then(|t| t.as_str()).unwrap_or("");
                let source = s.get("source").and_then(|t| t.as_str()).unwrap_or("desktop");
                let source_label = match source {
                    "cron" => "定时任务",
                    "agent" => "子端任务",
                    "api" => "API 会话",
                    _ => "桌面会话",
                };
                let hover_text = if !model.is_empty() {
                    format!("{}\n来源: {}\n模型: {}\n建立时间: {}", label, source_label, short_model(model), abs)
                } else {
                    format!("{}\n来源: {}\n建立时间: {}", label, source_label, abs)
                };
                row_resp.clone().on_hover_text(hover_text);
                row_resp.context_menu(|ui| {
                    // P4-33 右键菜单：复制ID / 重命名 / 置顶 / 归档 / 删除
                    if ui.button(format!("{} 复制ID", icon_text("copy"))).clicked() {
                        ui.ctx().copy_text(id.to_string());
                        ui.close();
                    }
                    // P4-10 重命名（行内编辑）
                    if ui.button(format!("{} 重命名", icon_text("pencil-simple"))).clicked() {
                        renaming = Some((id.to_string(), label.to_string()));
                        ui.close();
                    }
                    if ui.button(if is_pinned_for(s) { format!("{} 取消置顶", icon_text("push-pin")) } else { format!("{} 置顶", icon_text("push-pin")) }).clicked() {
                        pinned = Some((id.to_string(), !is_pinned_for(s)));
                        ui.close();
                    }
                    if ui.button(format!("{} 归档", icon_text("archive"))).clicked() {
                        archived = Some(id.to_string());
                        ui.close();
                    }
                    if ui.button(format!("{} 删除", icon_text("trash"))).clicked() {
                        deleted = Some(id.to_string());
                        ui.close();
                    }
                    // v2.5.7 对话→任务集成: 派任务（整个对话为任务来源——后端附最后用户请求）
                    if ui.button(format!("{} 派任务", icon_text("rocket-launch"))).clicked() {
                        delegate_sid = Some((id.to_string(), label.to_string()));
                        ui.close();
                    }
                });
            }
        };
        let mut pinned_items: Vec<&Value> = Vec::new();
        let mut normal_items: Vec<&Value> = Vec::new();
        for s in &sessions {
            if is_pinned_for(s) {
                pinned_items.push(s);
            } else {
                normal_items.push(s);
            }
        }
        egui::ScrollArea::vertical()
            .id_salt("chat_sidebar")
            .auto_shrink(false)
            .show(ui, |ui| {
                render_group(ui, &format!("{} 已置顶", icon_text("push-pin")), pinned_items.clone());
                if !pinned_items.is_empty() && !normal_items.is_empty() {
                    ui.add_space(6.0);
                }
                render_group(ui, &format!("{} 会话", icon_text("message-circle")), normal_items.clone());
            });
        if let Some((pid, pin)) = pinned {
            self.pin_pending = Some(api::chat_set_pinned_async(pid, pin));
            self.refresh_sessions();
        }
        // P4-10 重命名（进入行内编辑）
        if let Some((rid, rtext)) = renaming {
            self.renaming_id = Some(rid);
            self.rename_text = rtext;
        }
        // P4-10 重命名提交（Enter 确认）
        if let Some((rid, new_title)) = rename_commit {
            if !new_title.is_empty() {
                let sid = rid.clone();
                self.rename_pending = Some(api::chat_rename_session_async(sid, new_title));
            }
            self.renaming_id = None;
        }
        if let Some(id) = selected {
            self.open_session(id);
        }
        if let Some(id) = archived {
            self.archive_pending = Some(api::chat_set_archived_async(id, true));
            self.refresh_sessions();
        }
        if let Some(id) = deleted {
            if self.active_session.as_deref() == Some(&id) {
                self.active_session = None;
                self.messages.clear();
            }
            let _ = api::delete_chat_session_async(id);
            self.refresh_sessions();
        }
        // v2.5.7 对话→任务: 会话级派任务（POST /api/tasks——带 parent_session_id——后端附最后用户请求）
        if let Some((sid, label)) = delegate_sid {
            let model = self.current_model.clone();
            let desc = format!("处理对话「{}」的请求（从对话发起——详见来源会话）", label);
            self.delegate_pending = Some(api::chat_delegate_task_async(desc, model, Some(sid)));
        }
    }

    /// 渲染右侧消息区
    fn render_messages(&mut self, ui: &mut egui::Ui) {
        if self.active_session.is_none() {
            ui.add_space(40.0);
            ui.centered_and_justified(|ui| {
                ui.weak("← 选择或新建会话开始对话");
            });
            return;
        }
        if self.messages_loading && self.messages.is_empty() {
            ui.add_space(20.0);
            ui.spinner();
            ui.weak("加载中...");
            return;
        }
        // 错误提示
        if let Some(err) = self.send_error.clone() {
            ui.colored_label(egui::Color32::from_rgb(220, 80, 80), format!("⚠️ {}", err));
            ui.add_space(4.0);
        }
        // 消息流（ScrollArea + P4-29 虚拟列表——长对话只渲染视口±over_scan——恒定渲染成本）
        let msgs = self.messages.clone();
        let stream_content = self.stream_content.clone();
        let stream_reasoning = self.stream_reasoning.clone();
        let streaming = self.streaming;
        let avail_h = ui.available_height();
        let input_h = 60.0;
        let scroll_h = (avail_h - input_h - 16.0).max(120.0);
        // 时间线分组预计算（P1——间隔 >30 分钟插时间标签——每条消息前是否有标签）
        let mut timeline: Vec<bool> = Vec::with_capacity(msgs.len());
        let mut last_ts: Option<f64> = None;
        for m in &msgs {
            let ts = m.get("timestamp").and_then(|t| t.as_f64()).unwrap_or(0.0);
            let show = if let Some(lt) = last_ts {
                ts > 0.0 && lt > 0.0 && ts - lt > 1800.0
            } else {
                false
            };
            timeline.push(show);
            if ts > 0.0 {
                last_ts = Some(ts);
            }
        }
        let mut action: Option<MsgAction> = None;
        egui::ScrollArea::vertical()
            .id_salt("chat_messages")
            .auto_shrink(false)
            .max_height(scroll_h)
            .stick_to_bottom(true) // P4-31 新消息/流式块自动滚底（虚拟化后内容撑高——不自动到底流式不可见）
            .show(ui, |ui| {
                // 拆借字段（虚拟列表闭包 FnMut 内无 &mut self 冲突——P4-29）
                let msg_md_cache = &mut self.msg_md_cache;
                let editing_id = &mut self.editing_id;
                let editing_content = &mut self.editing_content;
                let thinking_open = &mut self.thinking_open;
                let speaking_id = &mut self.speaking_id;
                // 虚拟列表：每行一条消息——只渲染视口±200px（egui_virtual_list 懒算高度缓存）
                self.vlist.ui_custom_layout(ui, msgs.len(), |ui, i| {
                    if let Some(show) = timeline.get(i) {
                        if *show {
                            ui.add_space(8.0);
                            let ts = msgs[i].get("timestamp").and_then(|t| t.as_f64()).unwrap_or(0.0);
                            ui.weak(egui::RichText::new(format!("── {} ──", format_time(ts))).size(10.0));
                            ui.add_space(4.0);
                        }
                    }
                    if let Some(a) = Self::render_message(
                        ui,
                        &msgs[i],
                        msg_md_cache,
                        streaming,
                        editing_id,
                        editing_content,
                        thinking_open,
                        speaking_id,
                        &mut self.reactions,
                        &mut self.reacting_id,
                    ) {
                        action = Some(a);
                    }
                    1 // 一行一条消息
                });
                // 生成中的 assistant 消息（流式实时显示——C3）
                if streaming {
                    // P4-39 T5: 上下文压缩进行中（compacting 事件——Hermes TurnActivityIndicator 风格）
                    if self.stream_compacting {
                        ui.horizontal(|ui| {
                            ui.label(icon_text("compress"));
                            ui.spinner();
                            ui.weak(
                                egui::RichText::new("正在压缩历史…（上下文窗口管理）")
                                    .size(12.0),
                            );
                        });
                        ui.add_space(4.0);
                    }
                    // P4-35 工具执行中状态（tool_start 事件——"🔧 bash 执行中…"——等待有反馈）
                    if let Some(tool) = self.stream_tool.clone() {
                        ui.horizontal(|ui| {
                            ui.label(icon_text("robot"));
                            ui.spinner();
                            // P4-36 心跳计时（tool_ping 事件——"执行中…（N 秒）"——每 2s 更新）
                            ui.weak(
                                egui::RichText::new(format!(
                                    "🔧 {} 执行中…（{} 秒）",
                                    tool, self.stream_tool_elapsed
                                ))
                                .size(12.0),
                            );
                        });
                        ui.add_space(4.0);
                    }
                    ui.horizontal_wrapped(|ui| {
                        ui.label(icon_text("robot"));
                        egui::Frame::new()
                            .fill(ui.visuals().faint_bg_color)
                            .corner_radius(8.0)
                            .inner_margin(egui::Margin::same(8))
                            .show(ui, |ui| {
                                // P4-30 思考实时流式显示（Mr2109: 思考不是实时流式——根因: truncate 前 60 字
                                // 显示固定——实际后端逐 token 流式——改为尾部实时滚动 + 可展开全文）
                                if !stream_reasoning.is_empty() {
                                    let rn = stream_reasoning.chars().count();
                                    let sopen = self.thinking_open.contains(&-1);
                                    if sopen {
                                        // 展开：全文实时流式（ScrollArea 自动滚底——Hermes/Claude 风格）
                                        egui::ScrollArea::vertical()
                                            .id_salt("stream_reasoning")
                                            .max_height(220.0)
                                            .auto_shrink(false)
                                            .stick_to_bottom(true)
                                            .show(ui, |ui| {
                                                // P4-31 思考内容 md 渲染（CommonMarkViewer——与消息同款）
                                                let cache = msg_md_cache.entry(-2).or_default();
                                                CommonMarkViewer::new().render_math_fn(Some(&render_math)).show(ui, cache, &md_preprocess(&stream_reasoning));
                                            });
                                    } else {
                                        // 折叠：实时尾部 60 字（滚动更新——看得见思考在动）+ 字数
                                        let tail: String = stream_reasoning
                                            .chars()
                                            .rev()
                                            .take(60)
                                            .collect::<Vec<_>>()
                                            .into_iter()
                                            .rev()
                                            .collect();
                                        if ui
                                            .button(format!(
                                                "{} 思考中…（{} 字）点击展开",
                                                icon_text("brain"),
                                                rn
                                            ))
                                            .clicked()
                                        {
                                            self.thinking_open.insert(-1);
                                        }
                                        ui.weak(egui::RichText::new(format!("{}", tail)).size(11.0).weak());
                                    }
                                }
                                if stream_content.is_empty() && self.stream_tool.is_none() {
                                    // P4-35 思考等待计时器（Hermes TurnActivityIndicator 简化——卡没卡一眼知道）
                                    let wait = (now_f64() - self.stream_started).max(0.0);
                                    ui.spinner();
                                    ui.weak(
                                        egui::RichText::new(format!("思考中…（{:.0} 秒）", wait))
                                            .size(12.0),
                                    );
                                } else if stream_content.is_empty() {
                                    ui.spinner();
                                    ui.weak("工具执行中...");
                                } else {
                                    // D1 流式内容也 Markdown 渲染（P4-29 用拆借的 cache——闭包内无 self 冲突）
                                    let cache = msg_md_cache.entry(-1).or_default();
                                    CommonMarkViewer::new().render_math_fn(Some(&render_math)).show(ui, cache, &md_preprocess(&stream_content));
                                }
                            });
                    });
                    ui.add_space(6.0);
                }
            });
        // C7 消息操作执行（复制/重试/派单）——ScrollArea 外（P4-29 拆借结束后——无 &mut self 冲突）
        if let Some(a) = action {
            match a {
                MsgAction::Copy(text) => {
                    ui.ctx().copy_text(text);
                }
                MsgAction::Retry(text) => {
                    self.input = text;
                    self.send();
                }
                MsgAction::Delegate(text) => {
                    let model = self.current_model.clone();
                    let psid = self.active_session.clone();
                    self.delegate_pending = Some(api::chat_delegate_task_async(text, model, psid));
                    self.send_error = Some(format!("{} 已派单到任务队列", icon_text("rocket-launch")).to_string());
                }
            }
        }
        // 底部输入区（C5 模型胶囊 + D3 图片 + 输入 + 发送）
        ui.separator();
        let mut send_clicked = false;
        let mut stop_clicked = false;
        // D3/P2 已选图片 chips（多图——Hermes 多附件借鉴）
        if !self.pending_images.is_empty() {
            ui.horizontal_wrapped(|ui| {
                let mut remove: Option<usize> = None;
                for (i, (_, name)) in self.pending_images.iter().enumerate() {
                    let label = format!("{} {}", icon_text("image"), truncate(name, 18));
                    if ui.small_button(label).clicked() {
                        remove = Some(i);
                    }
                }
                if let Some(i) = remove {
                    self.pending_images.remove(i);
                }
            });
        }
        // P0 编辑模式（Hermes user-edit 借鉴——编辑框在输入区上方）
        if let Some(mid) = self.editing_id {
            let mut cancel_edit = false;
            let mut save_edit = false;
            // ② 富文本快捷工具栏（挂起清单②——markdown 片段插入编辑缓冲——光标跟踪 API 深——先尾部插最小版）
            let mut md_insert: Option<&str> = None;
            ui.horizontal(|ui| {
                ui.weak(egui::RichText::new(format!("{} 编辑消息 #{}\t", icon_text("pencil-simple"), mid)).size(11.0));
                ui.weak(egui::RichText::new("格式:").size(10.0));
                if ui.small_button("B").on_hover_text("粗体 **文字**").clicked() {
                    md_insert = Some("****");
                }
                if ui.small_button("I").on_hover_text("斜体 *文字*").clicked() {
                    md_insert = Some("**");
                }
                if ui.small_button("<>").on_hover_text("行内代码 `文字`").clicked() {
                    md_insert = Some("``");
                }
                if ui.small_button("{}").on_hover_text("代码块 ```语言").clicked() {
                    md_insert = Some("\n```\n\n```\n");
                }
                if ui.small_button("•").on_hover_text("列表项 - 文字").clicked() {
                    md_insert = Some("\n- ");
                }
                if ui.small_button("🔗").on_hover_text("链接 [文字](url)").clicked() {
                    md_insert = Some("[](url)");
                }
                if ui.small_button(">").on_hover_text("引用 > 文字").clicked() {
                    md_insert = Some("\n> ");
                }
                ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                    if ui.small_button("✕").on_hover_text("取消").clicked() {
                        cancel_edit = true;
                    }
                });
            });
            if let Some(frag) = md_insert {
                self.editing_content.push_str(frag);
            }
            let eresp = ui.add(
                egui::TextEdit::multiline(&mut self.editing_content)
                    .desired_rows(2)
                    .hint_text("编辑内容... (Enter 保存)")
                    .desired_width(ui.available_width() - 80.0),
            );
            let eenter = eresp.lost_focus()
                && ui.input(|i| i.key_pressed(egui::Key::Enter))
                && !ui.input(|i| i.modifiers.shift);
            if ui.small_button(format!("{} 保存", icon_text("floppy-disk"))).clicked() {
                save_edit = true;
            }
            if eenter {
                save_edit = true;
            }
            if cancel_edit {
                self.editing_id = None;
                self.editing_content.clear();
            }
            if save_edit {
                let c = self.editing_content.trim().to_string();
                if !c.is_empty() {
                    self.edit_pending = Some(api::chat_edit_message_async(mid, c));
                    self.editing_id = None;
                    self.editing_content.clear();
                }
            }
        }
        {
            // 第一行：输入框（全宽——P2-3 按钮下移）
            // P4-13 Enter=发送 / Shift+Enter=换行（return_key 改 Shift+Enter——
            // multiline 默认 Enter 是换行——改为 Enter 留给发送检测）
            // P4-16 输入框透明化（Hermes 调研——--dt-input-bg:0% + 淡边框 7%——
            // 输入区浮在消息区上不抢视觉——聚焦时边框亮起）
            let input_frame = egui::Frame::new()
                .fill(egui::Color32::TRANSPARENT)
                .stroke(egui::Stroke::new(1.0, ui.visuals().widgets.noninteractive.bg_stroke.color.gamma_multiply(0.7)))
                .corner_radius(8.0)
                .inner_margin(egui::Margin::symmetric(10, 6));
            let input_resp = input_frame.show(ui, |ui| {
                ui.add(
                    egui::TextEdit::multiline(&mut self.input)
                        .desired_rows(2)
                        .frame(egui::Frame::NONE)
                        .hint_text("输入消息... (Enter 发送 / Shift+Enter 换行 / / 命令)")
                        .desired_width(ui.available_width())
                        .return_key(egui::KeyboardShortcut::new(
                            egui::Modifiers::SHIFT,
                            egui::Key::Enter,
                        )),
                )
            });
            let resp = input_resp.inner;
            let enter = resp.has_focus()
                && ui.input(|i| i.key_pressed(egui::Key::Enter))
                && !ui.input(|i| i.modifiers.shift);
            // P1 输入历史（↑↓ 浏览——Hermes 借鉴）
            if resp.has_focus() {
                // P2 粘贴图片（Cmd+V 读剪贴板 PNG——JXA——Hermes paste-to-focus 借鉴）
                if ui.input(|i| i.modifiers.command && i.key_pressed(egui::Key::V)) {
                    if let Some(b64) = paste_clipboard_png() {
                        if self.pending_images.len() < 8 {
                            self.pending_images
                                .push((format!("data:image/png;base64,{}", b64), "剪贴板图片".to_string()));
                        }
                    }
                }
                if ui.input(|i| i.key_pressed(egui::Key::ArrowUp)) && !self.input_history.is_empty() {
                    let n = self.input_history.len();
                    let idx = match self.history_idx {
                        Some(i) if i > 0 => i - 1,
                        _ => n - 1,
                    };
                    self.history_idx = Some(idx);
                    self.input = self.input_history[idx].clone();
                }
                if ui.input(|i| i.key_pressed(egui::Key::ArrowDown)) {
                    if let Some(i) = self.history_idx {
                        if i + 1 < self.input_history.len() {
                            let ni = i + 1;
                            self.history_idx = Some(ni);
                            self.input = self.input_history[ni].clone();
                        } else {
                            self.history_idx = None;
                            self.input.clear();
                        }
                    }
                }
            }
            // P1 斜杠命令（输入 / 开头弹菜单——/new /clear /delegate）
            self.slash_open = self.input.starts_with('/') && !self.streaming;
            if self.slash_open {
                let cmd = self.input.trim().to_string();
                let mut picked: Option<String> = None;
                egui::Frame::new()
                    .fill(ui.visuals().extreme_bg_color)
                    .corner_radius(6.0)
                    .inner_margin(egui::Margin::same(6))
                    .show(ui, |ui| {
                        ui.weak(egui::RichText::new("命令:").size(11.0));
                        let cmds = [
                            ("/new", "新建会话"),
                            ("/clear", "清空输入"),
                            ("/delegate", "派单给任务队列（后接内容）"),
                        ];
                        for (k, desc) in cmds {
                            if ui.button(format!("{}  {}", k, desc)).clicked() {
                                picked = Some(k.to_string());
                            }
                        }
                    });
                if let Some(p) = picked {
                    match p.as_str() {
                        "/new" => {
                            self.new_session();
                            self.input.clear();
                        }
                        "/clear" => self.input.clear(),
                        "/delegate" => {
                            let rest = cmd.trim_start_matches("/delegate").trim().to_string();
                            if !rest.is_empty() {
                                let model = self.current_model.clone();
                                let psid = self.active_session.clone();
                                self.delegate_pending = Some(api::chat_delegate_task_async(rest, model, psid));
                                self.send_error = Some(format!("{} 已派单到任务队列", icon_text("rocket-launch")).to_string());
                            }
                            self.input.clear();
                        }
                        _ => {}
                    }
                    self.slash_open = false;
                }
            }
            // P1 建议药丸（输入区上方——快捷提问——Hermes SuggestionPills 借鉴）
            if !streaming && self.input.is_empty() && self.editing_id.is_none() {
                ui.horizontal_wrapped(|ui| {
                    let pills = ["📝 总结这段对话", "🔍 搜索知识库：虫族经济", "📌 帮我规划 v2.6 开源步骤", "🤔 分析一个技术问题"];
                    let mut fill: Option<String> = None;
                    for p in pills {
                        if ui.small_button(p).clicked() {
                            fill = Some(p.to_string());
                        }
                    }
                    if let Some(f) = fill {
                        self.input = f;
                    }
                });
            }
            // 第二行：按钮行（P2-3 按钮下移——胶囊 + 📎 + 发送/停止）
            ui.horizontal(|ui| {
                // 模型胶囊（当前模型——点击切换——Hermes model-pill 借鉴）
                let models = self.models.clone();
                let cur = self.current_model.clone();
                let mut new_model: Option<String> = None;
                egui::ComboBox::from_id_salt("chat_model_pill")
                    .selected_text(format!("{} {}", icon_text("brain"), short_model(&cur)))
                    .width(150.0)
                    .show_ui(ui, |ui| {
                        for m in &models {
                            if ui.selectable_label(*m == cur, m).clicked() {
                                new_model = Some(m.clone());
                            }
                        }
                        if !models.contains(&cur) {
                            if ui.selectable_label(true, &cur).clicked() {
                                new_model = Some(cur.clone());
                            }
                        }
                    })
                    .response
                    .on_hover_text(format!("当前模型: {}", cur));
                if let Some(nm) = new_model {
                    self.switch_model(nm);
                }
                // D3 图片选择按钮
                if ui
                    .button(icon_text("paperclip"))
                    .on_hover_text("发送图片")
                    .clicked()
                {
                    self.pick_image();
                }
                if streaming {
                    if ui.button(format!("{} 停止", icon_text("stop"))).clicked() {
                        stop_clicked = true;
                    }
                } else if ui.button(format!("{} 发送", icon_text("send"))).clicked() {
                    send_clicked = true;
                }
                if enter && !streaming {
                    send_clicked = true;
                }
            });
        }
        if stop_clicked {
            self.stop();
        }
        if send_clicked {
            self.send();
        }
        // P2 底部状态栏（Hermes footer 借鉴——版本/会话数/消息数/模型）
        ui.separator();
        ui.horizontal(|ui| {
            ui.weak(egui::RichText::new("虫族 Zerg v2.5.7").size(10.0));
            ui.weak(egui::RichText::new(format!("· {} 会话", self.sessions.len())).size(10.0));
            ui.weak(egui::RichText::new(format!("· {} 消息", self.messages.len())).size(10.0));
            if self.streaming {
                ui.colored_label(
                    egui::Color32::from_rgb(250, 180, 60),
                    egui::RichText::new("● 生成中").size(9.0),
                );
            } else {
                ui.weak(egui::RichText::new(format!("{} 就绪", icon_text("circle-check"))).size(9.0));
            }
            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                ui.weak(egui::RichText::new(format!("{} {}", icon_text("brain"), short_model(&self.current_model))).size(9.0));
            });
        });
    }

    /// 渲染单条消息（气泡——C5 思考可折叠——C7 hover 操作——D4 时间显示）
    // P4-29 关联函数（参数拆借——虚拟列表闭包内无 &mut self 冲突）
    fn render_message(
        ui: &mut egui::Ui,
        m: &Value,
        msg_md_cache: &mut std::collections::HashMap<i64, egui_commonmark::CommonMarkCache>,
        streaming: bool,
        editing_id: &mut Option<i64>,
        editing_content: &mut String,
        thinking_open: &mut std::collections::HashSet<i64>,
        speaking_id: &mut Option<i64>,
        reactions: &mut std::collections::HashMap<i64, String>,
        reacting_id: &mut Option<i64>,
    ) -> Option<MsgAction> {
        let role = m.get("role").and_then(|r| r.as_str()).unwrap_or("");
        let content = m.get("content").and_then(|c| c.as_str()).unwrap_or("");
        let reasoning = m.get("reasoning").and_then(|r| r.as_str()).unwrap_or("").to_string();
        let msg_id = m.get("id").and_then(|i| i.as_i64()).unwrap_or(-1);
        let ts = m.get("timestamp").and_then(|t| t.as_f64()).unwrap_or(0.0);
        let time_label = format_time(ts);
        let mut action: Option<MsgAction> = None;
        match role {
            "user" => {
                // P0 点击编辑（Hermes user-edit 借鉴——点击气泡进入编辑）
                // P4-17 非气泡模式（Mr2109——Hermes 非气泡——文本直接显示无边无底）
                let edit_click = egui::Frame::new()
                    .fill(egui::Color32::TRANSPARENT)
                    .stroke(egui::Stroke::NONE)
                    .inner_margin(egui::Margin { left: 2, right: 2, top: 4, bottom: 4 })
                    .show(ui, |ui| {
                        // P4-23 Markdown 渲染（CommonMarkViewer——文档查看模式同款）
                        let cache = msg_md_cache.entry(msg_id).or_default();
                        CommonMarkViewer::new().render_math_fn(Some(&render_math)).show(ui, cache, &md_preprocess(content));
                    })
                    .response
                    .interact(egui::Sense::click());
                if edit_click.clicked() && !streaming {
                    *editing_id = Some(msg_id);
                    *editing_content = content.to_string();
                }
                ui.horizontal(|ui| {
                    ui.weak(egui::RichText::new(time_label).size(12.0).weak());
                    // P0 hover 操作栏（渐显——Hermes 借鉴——hover 才显示）
                    if edit_click.hovered() {
                        if ui.small_button(icon_text("pencil-simple")).on_hover_text("编辑").clicked() {
                            *editing_id = Some(msg_id);
                            *editing_content = content.to_string();
                        }
                        if ui.small_button(icon_text("copy")).on_hover_text("复制").clicked() {
                            action = Some(MsgAction::Copy(content.to_string()));
                        }
                        if ui.small_button(icon_text("arrows-clockwise")).on_hover_text("重试").clicked() {
                            action = Some(MsgAction::Retry(content.to_string()));
                        }
                        if ui.small_button(icon_text("rocket-launch")).on_hover_text("派单").clicked() {
                            action = Some(MsgAction::Delegate(content.to_string()));
                        }
                    }
                });
            }
            "assistant" => {
                // 思考块（C5 可折叠——Hermes ThinkingDisclosure 借鉴——默认折叠显示预览）
                if !reasoning.is_empty() {
                    let open = thinking_open.contains(&msg_id);
                    let label = if open {
                        format!("{} 思考（点击收起）", icon_text("brain")).to_string()
                    } else {
                        format!("{} 思考: {}", icon_text("brain"), truncate(&reasoning, 80))
                    };
                    let resp = egui::Frame::new()
                        .fill(ui.visuals().extreme_bg_color)
                        .corner_radius(6.0)
                        .inner_margin(egui::Margin::same(6))
                        .show(ui, |ui| {
                            ui.weak(egui::RichText::new(&label).size(12.0));
                        })
                        .response
                        .interact(egui::Sense::click());
                    if resp.clicked() {
                        if open {
                            thinking_open.remove(&msg_id);
                        } else {
                            thinking_open.insert(msg_id);
                        }
                    }
                    if open {
                        egui::Frame::new()
                            .fill(ui.visuals().extreme_bg_color)
                            .corner_radius(6.0)
                            .inner_margin(egui::Margin::same(6))
                            .show(ui, |ui| {
                                // P4-31 思考内容 md 渲染（CommonMarkViewer——与消息同款）
                                let cache = msg_md_cache.entry(msg_id * 10 + 1).or_default();
                                CommonMarkViewer::new().render_math_fn(Some(&render_math)).show(ui, cache, &md_preprocess(&reasoning));
                            });
                    }
                }
                // 工具调用轨迹（C4b——与思考共用折叠状态）
                if let Some(tc) = m.get("tool_calls").and_then(|t| t.as_str()) {
                    if !tc.is_empty() && tc != "null" { // P4-31 无工具调用不显示（后端 NULL 序列化成字符串 "null"）
                        let topen = thinking_open.contains(&msg_id);
                        let tlabel = if topen {
                            format!("{} 工具调用（点击收起）", icon_text("wrench")).to_string()
                        } else {
                            format!("{} 工具调用…", icon_text("wrench")).to_string()
                        };
                        let tresp = egui::Frame::new()
                            .fill(ui.visuals().extreme_bg_color)
                            .corner_radius(6.0)
                            .inner_margin(egui::Margin::same(6))
                            .show(ui, |ui| {
                                ui.weak(egui::RichText::new(&tlabel).size(12.0));
                            })
                            .response
                            .interact(egui::Sense::click());
                        if tresp.clicked() {
                            if topen {
                                thinking_open.remove(&msg_id);
                            } else {
                                thinking_open.insert(msg_id);
                            }
                        }
                        if topen {
                            // P4-50 工具调用轨迹 MD 渲染（Mr2109: 密密麻麻JSON不适合人类看——转MD表格+代码块）
                            egui::Frame::new()
                                .fill(ui.visuals().extreme_bg_color)
                                .corner_radius(6.0)
                                .inner_margin(egui::Margin::same(6))
                                .show(ui, |ui| {
                                    let cache = msg_md_cache.entry(msg_id * 10 + 2).or_default();
                                    CommonMarkViewer::new()
                                        .render_math_fn(Some(&render_math))
                                        .show(ui, cache, &tool_calls_to_md(tc));
                                });
                        }
                    }
                }
                ui.horizontal_wrapped(|ui| {
                    ui.label(icon_text("robot"));
                    // P2 表情反应徽章（Hermes message-reactions 借鉴——消息角）
                    if let Some(emoji) = reactions.get(&msg_id) {
                        ui.weak(egui::RichText::new(format!("{} ", emoji)).size(12.0));
                    }
                    let bubble = egui::Frame::new()
                        .fill(egui::Color32::TRANSPARENT)
                        .stroke(egui::Stroke::NONE)
                        .inner_margin(egui::Margin { left: 2, right: 2, top: 4, bottom: 4 })
                        .show(ui, |ui| {
                            // P4-23 Markdown 渲染（CommonMarkViewer——文档查看模式同款）
                            let cache = msg_md_cache.entry(msg_id).or_default();
                            CommonMarkViewer::new().render_math_fn(Some(&render_math)).show(ui, cache, &md_preprocess(content));
                        })
                        .response
                        .interact(egui::Sense::click());
                    // P2 双击消息 → 表情选择（iMessage 式）
                    if bubble.double_clicked() {
                        *reacting_id = if *reacting_id == Some(msg_id) {
                            None
                        } else {
                            Some(msg_id)
                        };
                    }
                    ui.weak(egui::RichText::new(time_label).size(12.0).weak());
                    // P2 表情选择（reacting 状态）
                    if *reacting_id == Some(msg_id) {
                        for e in ["👍", "❤️", "🔥", "🎉", "😂"] {
                            if ui.small_button(e).clicked() {
                                if reactions.get(&msg_id) == Some(&e.to_string()) {
                                    reactions.remove(&msg_id);
                                } else {
                                    reactions.insert(msg_id, e.to_string());
                                }
                                *reacting_id = None;
                            }
                        }
                    }
                    // C7/P2 hover 操作（复制/重试/派单/朗读）
                    ui.horizontal(|ui| {
                        if ui.small_button(icon_text("copy")).on_hover_text("复制").clicked() {
                            action = Some(MsgAction::Copy(content.to_string()));
                        }
                        if ui.small_button(icon_text("arrows-clockwise")).on_hover_text("重试").clicked() {
                            action = Some(MsgAction::Retry(content.to_string()));
                        }
                        if ui.small_button(icon_text("rocket-launch")).on_hover_text("派单到任务").clicked() {
                            action = Some(MsgAction::Delegate(content.to_string()));
                        }
                        // P2 语音朗读（macOS say）
                        let speaking = *speaking_id == Some(msg_id);
                        if ui
                            .small_button(if speaking { icon_text("stop") } else { icon_text("speaker-high") })
                            .on_hover_text("朗读回复")
                            .clicked()
                        {
                            if speaking {
                                *speaking_id = None;
                                let _ = std::process::Command::new("pkill").arg("-f").arg("say").spawn();
                            } else {
                                let text = content.to_string();
                                *speaking_id = Some(msg_id);
                                std::thread::spawn(move || {
                                    let _ = std::process::Command::new("say")
                                        .arg("-r")
                                        .arg("170")
                                        .arg(&text)
                                        .output();
                                });
                            }
                        }
                    });
                });
            }
            "tool" => {
                ui.weak(format!("{} {}", icon_text("wrench"), truncate(content, 80)));
            }
            "system" => {
                ui.weak(format!("{} {}", icon_text("paperclip"), truncate(content, 80)));
            }
            _ => {}
        }
        ui.add_space(6.0);
        action
    }
}

/// 截断显示
// now_f64 — 当前时间（秒——等待计时器用）
fn now_f64() -> f64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs_f64())
        .unwrap_or(0.0)
}

fn truncate(s: &str, max: usize) -> String {
    let chars: Vec<char> = s.chars().collect();
    if chars.len() > max {
        let mut out: String = chars[..max].iter().collect();
        out.push('…');
        out
    } else {
        s.to_string()
    }
}

/// 时间格式化（D4——timestamp 秒 → HH:MM）
fn format_time(ts: f64) -> String {
    if ts <= 0.0 {
        return String::new();
    }
    let secs = ts as i64;
    let h = (secs / 3600) % 24;
    let m = (secs % 3600) / 60;
    format!("{:02}:{:02}", h, m)
}

/// 相对时间（P0——Hermes 借鉴——刚刚/N时/N天——秒级时间戳）
fn format_age(ts: f64) -> String {
    if ts <= 0.0 {
        return String::new();
    }
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .unwrap_or(0);
    let age = (now - ts as i64).max(0);
    if age < 60 {
        "刚刚".to_string()
    } else if age < 3600 {
        format!("{}分", age / 60)
    } else if age < 86400 {
        format!("{}时", age / 3600)
    } else if age < 86400 * 30 {
        format!("{}天", age / 86400)
    } else {
        format!("{}月", age / (86400 * 30))
    }
}

/// 绝对时间（P0——hover 显示——MM-DD HH:MM）
fn format_abs_time(ts: f64) -> String {
    if ts <= 0.0 {
        return String::new();
    }
    let secs = ts as i64;
    let d = secs / 86400;
    let h = (secs / 3600) % 24;
    let m = (secs % 3600) / 60;
    format!("{:02}-{:02} {:02}:{:02}", (d % 12) + 1, (d % 28) + 1, h, m)
}

/// 会话是否固定（pinned 字段兼容 bool/int）
fn is_pinned_for(s: &Value) -> bool {
    match s.get("pinned") {
        Some(p) => {
            if let Some(b) = p.as_bool() {
                b
            } else {
                p.as_i64().map(|i| i != 0).unwrap_or(false)
            }
        }
        None => false,
    }
}

/// 读剪贴板 PNG（P2——osascript JXA——NSPasteboard——返回 base64）
fn paste_clipboard_png() -> Option<String> {
    let script = "ObjC.import('AppKit'); var pb=$.NSPasteboard.generalPasteboard; var d=pb.dataForType($.NSPasteboardTypePNG); if(!d){''}else{var b64=$.NSData.dataWithData(d).base64EncodedStringWithOptions(0); b64.js}";
    let out = std::process::Command::new("osascript")
        .arg("-l")
        .arg("JavaScript")
        .arg("-e")
        .arg(script)
        .output()
        .ok()?;
    if !out.status.success() {
        return None;
    }
    let s = String::from_utf8_lossy(&out.stdout).trim().to_string();
    if s.is_empty() {
        None
    } else {
        Some(s)
    }
}

/// 短模型名（胶囊显示——去版本尾巴）
fn short_model(s: &str) -> String {
    if s.len() > 18 {
        truncate(s, 18)
    } else {
        s.to_string()
    }
}

/// base64 编码（D3 图片——标准库简易实现）
fn base64_std(bytes: &[u8]) -> String {
    const TABLE: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::with_capacity((bytes.len() + 2) / 3 * 4);
    let mut i = 0;
    while i + 3 <= bytes.len() {
        let n = ((bytes[i] as u32) << 16) | ((bytes[i + 1] as u32) << 8) | (bytes[i + 2] as u32);
        out.push(TABLE[(n >> 18) as usize & 63] as char);
        out.push(TABLE[(n >> 12) as usize & 63] as char);
        out.push(TABLE[(n >> 6) as usize & 63] as char);
        out.push(TABLE[n as usize & 63] as char);
        i += 3;
    }
    let rem = bytes.len() - i;
    if rem == 1 {
        let n = (bytes[i] as u32) << 16;
        out.push(TABLE[(n >> 18) as usize & 63] as char);
        out.push(TABLE[(n >> 12) as usize & 63] as char);
        out.push('=');
        out.push('=');
    } else if rem == 2 {
        let n = ((bytes[i] as u32) << 16) | ((bytes[i + 1] as u32) << 8);
        out.push(TABLE[(n >> 18) as usize & 63] as char);
        out.push(TABLE[(n >> 12) as usize & 63] as char);
        out.push(TABLE[(n >> 6) as usize & 63] as char);
        out.push('=');
    }
    out
}

/// 供 app.rs 使用的类型别名（避免循环引用）
pub type ChatState = Arc<Mutex<ChatView>>;

// P4-50 工具调用 JSON → Markdown（Mr2109: 工具调用记录密密麻麻一大团——完全不适合人类看——转MD: 每轮工具=标题+参数代码块+结果/错误）
fn tool_calls_to_md(tc: &str) -> String {
    let v: Vec<Value> = match serde_json::from_str(tc) {
        Ok(v) => v,
        Err(_) => return tc.to_string(), // 解析失败——原样显示
    };
    let mut md = String::new();
    for (i, call) in v.iter().enumerate() {
        let name = call.get("name").and_then(|x| x.as_str()).unwrap_or("?");
        let round = call.get("round").and_then(|x| x.as_i64()).unwrap_or((i + 1) as i64);
        md.push_str(&format!("**R{}**  `{}`\n\n", round, name));
        if let Some(args) = call.get("args").and_then(|x| x.as_str()) {
            if !args.is_empty() && args != "{}" {
                md.push_str(&format!("```json\n{}\n```\n\n", args));
            }
        }
        if let Some(result) = call.get("result").and_then(|x| x.as_str()) {
            if !result.is_empty() {
                // P4-50 结果进代码块（web_fetch 等结果是 Markdown 文本——直接进正文会被渲染器解析——黑屏爆炸）
                // 截断 4000（web_fetch 结果可 67KB——渲染器扛不住）
                // P4-50 fix: &result[..4000] 字节硬切——切到 UTF-8 多字节字符中间 panic（崩溃 chat_view.rs:1768）
                // 字符安全截断（char boundary 回退——中文/emoji 不崩）
                let body = if result.len() > 4000 {
                    let mut end = 4000;
                    while end > 0 && !result.is_char_boundary(end) {
                        end -= 1;
                    }
                    format!("{}…（结果过长已截断——共 {} 字符）", &result[..end], result.len())
                } else {
                    result.to_string()
                };
                // 4 反引号围栏（结果含 3 反引号不会提前闭合——无需转义）
                md.push_str(&format!("✅ 结果:\n````\n{}\n````\n\n", body));
            }
        }
        if let Some(err) = call.get("error").and_then(|x| x.as_str()) {
            if !err.is_empty() {
                md.push_str(&format!("❌ **错误:**\n````\n{}\n````\n\n", err));
            }
        }
        if i + 1 < v.len() {
            md.push_str("---\n\n");
        }
    }
    md
}
