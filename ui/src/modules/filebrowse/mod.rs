//! 文件/目录浏览器组件（阶段 1——2026-09-13 设计「文件浏览器集装箱」§4.1 / §4.3）
//!
//! 定位（§4.0 结论）：**把能力做成一只箱子**，不拆页面——
//! - 本目录是**内建组件库**，供文档模块、模型登记库、任务等多处**吊装复用**；
//! - 顶栏只多一只**薄壳箱** `file-browser`（`modules/mod.rs` 注册），作为它的一个入口。
//!
//! 组成：
//! - [`roots`]   根集合与选择（GET /api/fileroots——根 + 显示上限/类型闸门配置）
//! - [`browser`] 列表渲染 + 显示截断 + 类型判定（**纯函数**，单测钉死）
//! - [`actions`] open / reveal / 复制路径 + 错误码 → 可读文案
//!
//! 安全边界（§4.4）：只读根（`writable=false`）**界面上不给任何写入口**；
//! open/reveal 的路径白名单由**后端**把关（前端只传 `root` + 相对 `path`）。
//!
//! 阶段 1 明确不做：内嵌编辑器（正文只读渲染）、任意路径浏览、给 docs 以外根写权限。

pub mod actions;
pub mod browser;
pub mod roots;

use std::sync::{Arc, Mutex};

use eframe::egui;

use crate::api;
use browser::{FbIntent, FbView};
use roots::RootsState;

/// 取锁（中毒也恢复——与 app.rs 的 lock_recover 同款：一次 panic 不该让整个组件永久瘫痪）
fn lock<T>(m: &Mutex<T>) -> std::sync::MutexGuard<'_, T> {
    m.lock().unwrap_or_else(|e| e.into_inner())
}

/// 文件浏览器组件实例（**多处吊装 = 各持一份实例**——互不干扰）
pub struct FileBrowse {
    /// 根集合 + 配置（异步拉取结果；None = 还没到）
    roots: Arc<Mutex<Option<RootsState>>>,
    /// 根集合拉取失败/周期（60s——根清单几乎不变）
    last_roots: Option<std::time::Instant>,
    /// 当前根 id（空 = 待按后端 default 选定）
    root_id: String,
    /// 当前选中目录（相对根的相对路径）
    dir: String,
    /// 当前选中文件（相对根的相对路径）
    file: String,
    /// 当前根的列表（files, dirs——相对路径）
    listing: Arc<Mutex<Option<(Vec<String>, Vec<String>)>>>,
    /// 列表拉取周期（5s——与既有文档模块同频）
    last_listing: Option<std::time::Instant>,
    /// 列表错误（异步槽 → 界面）
    list_err: Arc<Mutex<Option<String>>>,
    /// 选中文件内容（空 = 未选/读取中）
    content: Arc<Mutex<Option<String>>>,
    /// 内容读取错误（异步槽 → 界面）
    content_err: Arc<Mutex<Option<String>>>,
    /// markdown 渲染缓存
    cache: egui_commonmark::CommonMarkCache,
    /// 可读错误（界面红字——列表/动作失败）
    err: Option<String>,
    /// 最近一次交给系统的动作提示
    action_msg: Option<String>,
    /// 在途动作结果
    action_result: Option<api::SharedResult<String>>,
}

impl Default for FileBrowse {
    fn default() -> Self {
        Self::new()
    }
}

impl FileBrowse {
    pub fn new() -> Self {
        FileBrowse {
            roots: Arc::new(Mutex::new(None)),
            last_roots: None,
            root_id: String::new(),
            dir: String::new(),
            file: String::new(),
            listing: Arc::new(Mutex::new(None)),
            last_listing: None,
            list_err: Arc::new(Mutex::new(None)),
            content: Arc::new(Mutex::new(None)),
            content_err: Arc::new(Mutex::new(None)),
            cache: egui_commonmark::CommonMarkCache::default(),
            err: None,
            action_msg: None,
            action_result: None,
        }
    }

    /// 拉根集合（首次即拉 + 60s 刷新——根清单几乎不变，拉太勤没意义）
    fn poll_roots(&mut self) {
        let due = self
            .last_roots
            .map(|t| t.elapsed().as_secs() >= 60)
            .unwrap_or(true);
        if !due {
            return;
        }
        self.last_roots = Some(std::time::Instant::now());
        let store = self.roots.clone();
        api::runtime().spawn(async move {
            match api::fetch_fileroots_blocking().await {
                Ok(v) => *lock(&store) = Some(RootsState::from_json(&v)),
                // 拿不到根清单：不猜路径、不清空已有值（离线时界面照旧可用）
                Err(e) => eprintln!("[filebrowse] /api/fileroots failed: {}", e),
            }
        });
    }

    /// 拉当前根的列表（首次即拉 + 5s 刷新）
    fn poll_listing(&mut self) {
        if self.root_id.is_empty() {
            return;
        }
        // §尾巴 a：当前根已知不存在 ⇒ 不发起列表请求（闸门是纯函数 should_fetch_listing，单测钉死），
        // 并清掉旧列表——别把上一根的目录挂到这颗不存在的根名下；空态由 render 显示。
        let fetch = {
            let g = lock(&self.roots);
            roots::should_fetch_listing(&self.root_id, g.as_ref())
        };
        if !fetch {
            *lock(&self.listing) = None;
            return;
        }
        let due = self
            .last_listing
            .map(|t| t.elapsed().as_secs() >= 5)
            .unwrap_or(true);
        if !due {
            return;
        }
        self.last_listing = Some(std::time::Instant::now());
        let store = self.listing.clone();
        let err = self.list_err.clone();
        let root = self.root_id.clone();
        api::runtime().spawn(async move {
            match api::fetch_docs_root_blocking(Some(&root)).await {
                Ok(v) => {
                    *lock(&store) = Some(v);
                    *lock(&err) = None;
                }
                // 失败保留旧列表（一次抖动不该把第一栏刷成空）+ 记错误
                Err(e) => *lock(&err) = Some(e),
            }
        });
    }

    /// 读取选中文件（内容整份取回——读取不设上限；截断只发生在渲染，§4.4）
    fn read_file(&mut self) {
        let path = self.file.clone();
        if path.is_empty() {
            return;
        }
        let root = self.root_id.clone();
        *lock(&self.content) = None;
        *lock(&self.content_err) = None;
        let store = self.content.clone();
        let err = self.content_err.clone();
        api::runtime().spawn(async move {
            match api::fetch_doc_content_any_root_blocking(&root, path).await {
                Ok(txt) => {
                    *lock(&store) = Some(txt);
                    *lock(&err) = None;
                }
                Err(e) => {
                    *lock(&store) = None;
                    *lock(&err) = Some(e);
                }
            }
        });
    }

    /// 每帧：拉数据 + 收异步结果
    fn poll(&mut self) {
        self.poll_roots();
        // 根清单到达 → 选定默认根（后端 default 标记优先）
        if self.root_id.is_empty() {
            if let Some(s) = lock(&self.roots).as_ref() {
                if let Some(d) = roots::default_root_id(&s.roots) {
                    self.root_id = d;
                }
            }
        }
        self.poll_listing();
        // 收列表错误
        if let Some(e) = lock(&self.list_err).take() {
            self.err = Some(e);
        }
        // 收动作结果
        let pending = self.action_result.as_ref().map(|arc| lock(arc).take());
        if let Some(Some(r)) = pending {
            self.action_result = None;
            match r {
                Ok(abs) => self.action_msg = Some(rust_i18n::t!("fb.action.done", abs = abs).to_string()),
                Err(e) => self.err = Some(e),
            }
        }
    }

    /// 发起一个「交给系统」的动作（open / reveal）
    fn start_action(&mut self, action: &str, root: String, path: String, mode: Option<&'static str>) {
        self.action_msg = None;
        self.err = None;
        self.action_result = Some(actions::fileroot_action_async(action, root, path, mode));
    }

    fn view(&self, state: Option<&RootsState>) -> FbView {
        FbView {
            root: self.root_id.clone(),
            // 拿不到根清单 ⇒ 按不可写处理（保守：宁可少一个写菜单，也不给错根写入口）
            writable: state.map(|s| s.is_writable(&self.root_id)).unwrap_or(false),
            dir: self.dir.clone(),
            file: self.file.clone(),
            cfg: state.map(|s| s.config.clone()).unwrap_or_default(),
        }
    }

    /// 处理渲染层交回的意图（改状态 / 发请求）
    fn apply(&mut self, intents: Vec<FbIntent>, ctx: &egui::Context) {
        for it in intents {
            match it {
                FbIntent::RootChanged(_id) => {
                    // 切根：清空旧根的选中与缓存（**不能**让上一根的内容留在这根名下）
                    self.dir.clear();
                    self.file.clear();
                    *lock(&self.listing) = None;
                    *lock(&self.content) = None;
                    *lock(&self.content_err) = None;
                    *lock(&self.list_err) = None;
                    self.err = None;
                    self.action_msg = None;
                    self.last_listing = None; // 立即重拉
                }
                FbIntent::PickDir(d) => {
                    self.dir = d;
                    self.file.clear();
                    *lock(&self.content) = None;
                    *lock(&self.content_err) = None;
                }
                FbIntent::PickFile(f) => {
                    self.file = f;
                    self.read_file();
                }
                FbIntent::Reveal { root, path } => self.start_action("reveal", root, path, None),
                FbIntent::Open { root, path, mode } => {
                    self.start_action("open", root, path, Some(mode.as_str()))
                }
                FbIntent::CopyPath(p) => actions::copy_path(ctx, &p),
            }
        }
    }

    /// 渲染整个组件（顶部根选择器 + 第一栏目录/文件 + 第二栏内容预览——不做内嵌编辑器）
    pub fn render(&mut self, ui: &mut egui::Ui) {
        self.poll();
        let mut out: Vec<FbIntent> = Vec::new();
        let state = lock(&self.roots).clone();

        // 顶部：根栏（根选择器 + 当前根绝对路径 + 复制路径，§4.3）+ 只读根提示
        roots::root_bar(ui, state.as_ref(), &mut self.root_id, &mut out);
        let view = self.view(state.as_ref());
        if !view.writable {
            ui.weak(rust_i18n::t!("fb.readonly"));
        }
        if let Some(e) = &self.err {
            ui.colored_label(egui::Color32::from_rgb(230, 90, 90), format!("⚠ {}", e));
        }
        if let Some(m) = &self.action_msg {
            ui.weak(m.clone());
        }
        ui.separator();

        let (dirs, files) = lock(&self.listing).clone().unwrap_or_default();
        let content = lock(&self.content).clone();
        let cerr = lock(&self.content_err).clone();
        let files_loaded = lock(&self.listing).is_some();
        // §尾巴 a：当前根已知不存在（纯函数判定）⇒ 左栏走空态、右栏不展示
        let root_id = self.root_id.clone();
        let missing = roots::root_missing(&root_id, state.as_ref());
        ui.columns(2, |cols| {
            egui::ScrollArea::vertical()
                .id_salt("fb_col2_list")
                .auto_shrink(false)
                .show(&mut cols[0], |ui| {
                    if missing {
                        roots::render_root_missing(ui, state.as_ref(), &root_id, &mut out);
                        return;
                    }
                    if !files_loaded {
                        ui.spinner();
                        ui.weak(rust_i18n::t!("common.loading"));
                        return;
                    }
                    browser::render_list(ui, &view, &dirs, &files, &mut out);
                });
            egui::ScrollArea::vertical()
                .id_salt("fb_col2_content")
                .auto_shrink(false)
                .show(&mut cols[1], |ui| {
                    if missing {
                        return; // 根不存在：右栏不展示（空态已在左栏给出）
                    }
                    if view.file.is_empty() && cerr.is_none() {
                        ui.weak(rust_i18n::t!("docs.pick_hint"));
                        return;
                    }
                    browser::render_content(
                        ui,
                        &view,
                        content.as_deref(),
                        cerr.as_deref(),
                        &mut self.cache,
                        &mut out,
                    );
                });
        });
        self.apply(out, ui.ctx());
    }
}

#[cfg(test)]
mod tests {
    use super::browser::{can_open, display_window, mib, truncated_open_allowed};
    use super::roots::{
        default_root_id, root_label, FilerootsConfig, RootInfo, RootsState, DEFAULT_DISPLAY_MAX,
    };
    use super::{actions, roots};

    /// 后端 /api/fileroots 的真实形态（五项——§4.2 Mr2109：含「模型权重」）
    fn fileroots_fixture() -> serde_json::Value {
        serde_json::json!({
            "roots": [
                {"id":"docs","label":"虫族文档","path":"<volume-path>","default":true,"writable":true},
                {"id":"repo","label":"虫族仓库","path":"<volume-path>","default":false,"writable":false},
                {"id":"models","label":"模型登记库","path":"~/.zerg/models","default":false,"writable":false},
                {"id":"tasks","label":"任务目录","path":"/tmp/zerg/tasks","default":false,"writable":false},
                {"id":"weights","label":"模型权重","path":"~/models","default":false,"writable":false}
            ],
            "config": {"display_max": 1048576, "allow_all_types": false, "text_exts": [".md",".txt"]}
        })
    }

    /// 根解析：五项、顺序、default/writable 旗标——与后端契约逐项对齐
    #[test]
    fn parse_fileroots_keeps_five_roots_in_order() {
        let s = RootsState::from_json(&fileroots_fixture());
        let ids: Vec<&str> = s.roots.iter().map(|r| r.id.as_str()).collect();
        assert_eq!(ids, vec!["docs", "repo", "models", "tasks", "weights"]);
        assert!(s.find("docs").unwrap().writable, "docs 根必须可写（写菜单依赖它）");
        for ro in ["repo", "models", "tasks", "weights"] {
            assert!(!s.find(ro).unwrap().writable, "{} 根必须只读（§九 Q2）", ro);
        }
        assert_eq!(s.path_of("weights"), Some("~/models"));
        assert_eq!(s.config.display_max, 1048576);
        assert_eq!(s.config.text_exts, vec![".md".to_string(), ".txt".to_string()]);
        assert!(!s.config.allow_all_types);
    }

    /// 默认根：后端 default 标记优先；无标记取第一项；空清单 → None（三种情形都不许 panic）
    #[test]
    fn default_root_id_prefers_backend_flag() {
        let flagged = vec![
            RootInfo { id: "a".into(), label: "A".into(), path: "/a".into(), is_default: false, writable: false, exists: true },
            RootInfo { id: "b".into(), label: "B".into(), path: "/b".into(), is_default: true, writable: false, exists: true },
        ];
        assert_eq!(default_root_id(&flagged).as_deref(), Some("b"));
        let none_flagged = vec![
            RootInfo { id: "a".into(), label: "A".into(), path: "/a".into(), is_default: false, writable: false, exists: true },
        ];
        assert_eq!(default_root_id(&none_flagged).as_deref(), Some("a"));
        assert_eq!(default_root_id(&[]), None);
    }

    /// 配置缺省与非法值：缺字段落缺省；display_max=0 视为缺省（0 会让界面永远空白）
    #[test]
    fn config_defaults_and_zero_display_max_fall_back() {
        let c = FilerootsConfig::from_json(None);
        assert_eq!(c.display_max, DEFAULT_DISPLAY_MAX);
        assert!(!c.allow_all_types);
        assert!(c.text_exts.iter().any(|e| e == ".md"), "缺省文本清单须含 .md");
        let zero = FilerootsConfig::from_json(Some(&serde_json::json!({"display_max": 0})));
        assert_eq!(zero.display_max, DEFAULT_DISPLAY_MAX, "display_max=0 视为未配置");
        let custom = FilerootsConfig::from_json(Some(&serde_json::json!({"display_max": 4096})));
        assert_eq!(custom.display_max, 4096);
    }

    /// 根显示名：i18n 键优先（中文界面看键值），键缺失才退回后端 label
    #[test]
    fn root_label_prefers_i18n_then_backend_label() {
        // 命中键：返回键值（不返回后端中文 label —— 英文界面才不会露中文）
        let hit = root_label("docs", "虫族文档", |k| {
            if k == "fb.root.docs" { "Docs".to_string() } else { k.to_string() }
        });
        assert_eq!(hit, "Docs");
        // 未命中键（rust-i18n 原样返回键名）：退回后端 label
        let miss = root_label("unknown-root", "后端名", |k| k.to_string());
        assert_eq!(miss, "后端名");
    }

    /// 显示截断（§4.4 Q3）：按真实字节数比较——超上限只渲染前 display_max 字节并置提示位
    #[test]
    fn display_window_truncates_at_display_max_bytes() {
        let small = "x".repeat(1024);
        let (shown, n, cut) = display_window(&small, DEFAULT_DISPLAY_MAX);
        assert_eq!((n, cut), (1024, false));
        assert_eq!(shown.len(), 1024, "小文件必须整份渲染");
        // 恰好等于上限：不算截断
        let exact = "y".repeat(DEFAULT_DISPLAY_MAX);
        assert_eq!(display_window(&exact, DEFAULT_DISPLAY_MAX).1, DEFAULT_DISPLAY_MAX);
        assert!(!display_window(&exact, DEFAULT_DISPLAY_MAX).2);
        // 3 MiB 文本：只渲染前 1 MiB，且必须给提示
        let big = "z".repeat(3 * 1024 * 1024);
        let (big_shown, bn, bcut) = display_window(&big, DEFAULT_DISPLAY_MAX);
        assert_eq!(bn, DEFAULT_DISPLAY_MAX, "3 MiB 文本只渲染前 1 MiB");
        assert_eq!(big_shown.len(), DEFAULT_DISPLAY_MAX);
        assert!(bcut, "超出上限必须置「已截断」提示位");
        assert_eq!(display_window("", 1024), ("", 0, false));
    }

    /// 截断必须回退到**字符边界**——中文 3 字节/字，切在中间就是乱码
    #[test]
    fn display_window_respects_char_boundaries() {
        let zh = "中文内容"; // 12 字节
        let (shown, n, cut) = display_window(zh, 7);
        assert!(cut);
        assert_eq!(shown, "中文");
        assert_eq!(n, 6, "7 落在第 3 个字中间 → 回退到 6 字节");
        assert_eq!(n % 3, 0, "回退后必须是 3 的整数倍（UTF-8 中文一字 3 字节）");
        // 大文本（1 Mi 个「中」）：渲染字节数 ≤ 上限 且 落在字符边界
        let big = "中".repeat(1024 * 1024);
        let (_s, bn, bcut) = display_window(&big, DEFAULT_DISPLAY_MAX);
        assert!(bcut);
        assert!(bn <= DEFAULT_DISPLAY_MAX);
        assert!(big.is_char_boundary(bn), "截断点必须落在字符边界");
    }

    /// MiB 文案（提示用）：1 位小数
    #[test]
    fn mib_text_renders_one_decimal() {
        assert_eq!(mib(0), "0.0");
        assert_eq!(mib(1024 * 1024), "1.0");
        assert_eq!(mib(3 * 1024 * 1024), "3.0");
        assert_eq!(mib(1536 * 1024), "1.5");
    }

    /// 类型闸门（§4.4 / Q4+Q5）：文本清单内可开、清单外只给「在访达中显示」、无扩展名不给开
    #[test]
    fn can_open_type_gate() {
        let cfg = FilerootsConfig::from_json(Some(&serde_json::json!({
            "allow_all_types": false, "text_exts": [".md", ".txt"]
        })));
        assert!(can_open("a/b.md", false, &cfg), "清单内文本可开");
        assert!(can_open("a/b.TXT", false, &cfg), "扩展名大小写不敏感");
        assert!(!can_open("model.gguf", false, &cfg), "二进制默认不读入 UI / 不开（Q4）");
        assert!(!can_open("Makefile", false, &cfg), "无扩展名默认只给「在访达中显示」");
        assert!(can_open("任何目录", true, &cfg), "目录恒可开");
        let all = FilerootsConfig::from_json(Some(&serde_json::json!({"allow_all_types": true})));
        assert!(can_open("model.gguf", false, &all), "放开类型=改配置即生效（不改代码）");
    }

    /// §尾巴 a【关键回归点】：`exists` 字段缺失（旧后端）必须按 **true** 处理——
    /// 若默认成 false，所有旧后端的根都会被误报为「该根不存在」。
    #[test]
    fn exists_field_defaults_to_true_for_old_backend() {
        // 旧后端：响应里根本没有 exists 字段
        let old = serde_json::json!({
            "roots": [
                {"id":"docs","label":"虫族文档","path":"/d","default":true,"writable":true}
            ]
        });
        let s = RootsState::from_json(&old);
        let r = s.find("docs").unwrap();
        assert!(r.exists, "缺 exists 字段必须落成 true（旧后端回归点）");
        assert!(!s.known_missing("docs"), "旧后端的根不得被判为「不存在」");
        assert!(!roots::root_missing("docs", Some(&s)), "纯函数也不得误报");
        // 新后端：显式 exists=false ⇒ 识别为不存在；显式 true ⇒ 存在
        let new = serde_json::json!({
            "roots": [
                {"id":"weights","label":"模型权重","path":"/w","writable":false,"exists":false},
                {"id":"docs","label":"虫族文档","path":"/d","writable":true,"exists":true}
            ]
        });
        let s2 = RootsState::from_json(&new);
        assert!(!s2.find("weights").unwrap().exists, "显式 false 必须被解析出来");
        assert!(s2.known_missing("weights"));
        assert!(!s2.known_missing("docs"));
        // 清单未到 / 未知根 id：不误报（未知 ≠ 不存在）
        assert!(!roots::root_missing("weights", None), "根清单未到不得判不存在");
        assert!(!roots::root_missing("brand-new-root", Some(&s2)), "未知根 id 不得判不存在");
    }

    /// §尾巴 a：当前根已知不存在 ⇒ **不发起列表请求**（纯函数闸门）；存在/未知/无清单照常拉。
    #[test]
    fn missing_root_skips_listing_fetch() {
        let s = RootsState::from_json(&serde_json::json!({
            "roots": [
                {"id":"weights","label":"模型权重","path":"/w","writable":false,"exists":false},
                {"id":"docs","label":"虫族文档","path":"/d","writable":true,"exists":true}
            ]
        }));
        assert!(
            !roots::should_fetch_listing("weights", Some(&s)),
            "根不存在 ⇒ 不拉列表（进入空态），否则每 5s 白刷一次必然失败的请求"
        );
        assert!(roots::should_fetch_listing("docs", Some(&s)), "存在的根照常拉");
        assert!(
            roots::should_fetch_listing("docs", None),
            "根清单未到不得因此不拉（离线/首帧照常尝试）"
        );
        assert!(
            roots::should_fetch_listing("unknown-root", Some(&s)),
            "未知根 id 不误判——照常拉"
        );
        assert!(!roots::should_fetch_listing("", Some(&s)), "未选根不拉");
    }

    /// §尾巴 b【类型闸门】：截断提示里的「用默认应用打开」按钮仅在 `can_open` 放行时出现
    /// （清单外只保留「在访达中显示」——§4.4）。
    #[test]
    fn truncation_open_button_respects_type_gate() {
        let cfg = FilerootsConfig::from_json(Some(&serde_json::json!({
            "allow_all_types": false, "text_exts": [".md", ".txt"]
        })));
        assert!(
            truncated_open_allowed("notes/long.md", &cfg),
            "清单内文本：截断提示给「用默认应用打开」（看全文）"
        );
        assert!(
            !truncated_open_allowed("weights/model.gguf", &cfg),
            "清单外（.gguf）：截断提示不得出现 open 按钮，只留 reveal"
        );
        assert!(
            !truncated_open_allowed("Makefile", &cfg),
            "无扩展名同样不给 open 按钮"
        );
        // 放开类型 = 改配置即生效（不改代码）
        let all = FilerootsConfig::from_json(Some(&serde_json::json!({"allow_all_types": true})));
        assert!(
            truncated_open_allowed("weights/model.gguf", &all),
            "allow_all_types 后按钮随之出现"
        );
    }

    /// 错误码 → 文案：六个码映射到**互不相同**的键，且键在 yml 里真实存在（文案 ≠ 键名）
    #[test]
    fn fileroot_error_mapping_is_distinct_and_readable() {
        let codes = [
            "INVALID_ROOT",
            "INVALID_PATH",
            "NOT_FOUND",
            "NOT_ALLOWED",
            "READ_FAILED",
            "OPEN_FAILED",
        ];
        let mut keys = std::collections::BTreeSet::new();
        let mut texts = std::collections::BTreeSet::new();
        for c in codes {
            let k = actions::fileroot_error_key(c);
            assert!(keys.insert(k), "错误码 {} 复用了同一个键 {}", c, k);
            let t = actions::fileroot_error_text(c);
            assert_ne!(t, k, "键 {} 在 locales 里不存在（界面会直接露出键名）", k);
            assert!(!t.trim().is_empty(), "{} 的文案不能为空", c);
            texts.insert(t);
        }
        assert_eq!(texts.len(), codes.len(), "六个码的文案必须互不相同（否则界面无法分辨）");
        // 大小写不敏感（服务端恒大写，调用方可能传小写）
        assert_eq!(actions::fileroot_error_key("not_allowed"), "fb.error.not_allowed");
        // 未收录码 → 通用兜底，不 panic
        assert_eq!(actions::fileroot_error_key("SOME_FUTURE_CODE"), "fb.error.unknown");
        assert_ne!(actions::fileroot_error_text("SOME_FUTURE_CODE"), "fb.error.unknown");
    }

    /// 错误体解析：两种形态 + 畸形体——都不 panic，且给出可读文案（不糊原始 JSON）
    #[test]
    fn fileroot_error_bodies_never_panic() {
        // 注意：这里**不能**写 `assert_eq!(from_body(x), fileroot_error_text(code))`——
        // 两边各读一次全局 locale，而 main.rs::locale_tests 会在并行运行中 `set_locale`，
        // 两次读可能跨越切换点拿到不同语言而偶发失败（本仓库既有 flake）。
        // 改为断言「结果 ∈ 该键在全部内置 locale 下的译文」：语义不变、与 locale 无关。
        // 形态一：{"error":{"type","message"}}
        let b1 = serde_json::json!({"error":{"type":"NOT_ALLOWED","message":"该类型不允许"}});
        assert_eq!(actions::fileroot_error_code(&b1).as_deref(), Some("NOT_ALLOWED"));
        assert!(
            translations_of("fb.error.not_allowed").contains(&actions::fileroot_error_from_body(&b1)),
            "NOT_ALLOWED 须落 fb.error.not_allowed 的文案（当前语言）"
        );
        // 形态二：{"error":"NOT_FOUND"}
        let b2 = serde_json::json!({"error":"NOT_FOUND"});
        assert!(
            translations_of("fb.error.not_found").contains(&actions::fileroot_error_from_body(&b2)),
            "NOT_FOUND 须落 fb.error.not_found 的文案（当前语言）"
        );
        // 未收录码：附码便于排查
        let b3 = serde_json::json!({"error":{"type":"WEIRD_CODE","message":"x"}});
        assert!(actions::fileroot_error_from_body(&b3).contains("WEIRD_CODE"));
        // 无码有 message：回退服务端文案（与语言无关）
        let b4 = serde_json::json!({"error":{"message":"后端说明"}});
        assert_eq!(actions::fileroot_error_from_body(&b4), "后端说明");
        // 完全空/畸形：通用文案，不 panic
        assert!(
            translations_of("fb.error.unknown")
                .contains(&actions::fileroot_error_from_body(&serde_json::Value::Null)),
            "畸形/空体须落通用文案 fb.error.unknown（当前语言）"
        );
        assert!(!actions::fileroot_error_from_body(&serde_json::json!("oops")).is_empty());
    }

    /// 取某扁平键在**全部内置 locale**（zh-CN + en）下的文案集合。
    /// 消除「两次 `t!()` 互等」与 main.rs 并行 `set_locale` 的竞态（见上面测试注释）。
    fn translations_of(key: &str) -> Vec<String> {
        const ZH: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/locales/zh-CN.yml"));
        const EN: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/locales/en.yml"));
        [ZH, EN].iter().filter_map(|y| flat_value(y, key)).collect()
    }

    /// 从扁平 yml 取单个键的值（去首尾双引号）
    fn flat_value(yml: &str, key: &str) -> Option<String> {
        yml.lines().find_map(|l| {
            let (k, v) = l.trim().split_once(':')?;
            if k.trim() == key {
                Some(v.trim().trim_matches('"').to_string())
            } else {
                None
            }
        })
    }

    /// 集装箱图标名必须真实存在（拼错会每帧告警并回退成文字）
    #[test]
    fn file_browser_icon_name_exists() {
        let g = crate::modules::icons::icon_text("folder-open");
        assert_eq!(g.chars().count(), 1, "folder-open 未收录/拼错——回退成了文本: {}", g);
    }

    /// 只读根不给写入口（§七 11 条）——用真实根清单判定
    #[test]
    fn writable_roots_gate_for_write_menus() {
        let s = RootsState::from_json(&fileroots_fixture());
        assert!(s.is_writable("docs"));
        for ro in ["repo", "models", "tasks", "weights"] {
            assert!(!s.is_writable(ro), "{} 是只读根（只读根不得出现写菜单）", ro);
        }
        assert!(!s.is_writable("nonexistent"), "未知根按只读处理（保守）");
        // 根选择器的默认根解析与契约一致（docs 在船即有默认根）
        assert_eq!(default_root_id(&s.roots).as_deref(), Some("docs"));
        assert!(roots::DEFAULT_TEXT_EXTS.contains(&".md"));
    }

    /// i18n：zh-CN 与 en 的 fb.* 键集合必须**完全一致**（少一个键某语言就露出键名）
    #[test]
    fn i18n_fb_keys_match_between_locales() {
        const ZH: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/locales/zh-CN.yml"));
        const EN: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/locales/en.yml"));
        let zh = keys_with_prefix(ZH, "fb.");
        let en = keys_with_prefix(EN, "fb.");
        assert!(zh.len() >= 10, "fb.* 键数量不足（设计 §4.3 约 10 键）：{}", zh.len());
        assert_eq!(
            zh, en,
            "zh-CN 与 en 的 fb.* 键集合必须一致：缺的一方会直接显示键名"
        );
        for k in [
            "fb.root.label",
            "fb.root.docs",
            "fb.root.repo",
            "fb.root.models",
            "fb.root.tasks",
            "fb.root.weights",
            "fb.root.missing",
            "fb.action.reveal",
            "fb.action.open",
            "fb.path.copy",
            "fb.readonly",
            "fb.display.truncated",
            "fb.error.invalid_root",
            "fb.error.invalid_path",
            "fb.error.not_found",
            "fb.error.not_allowed",
            "fb.error.read_failed",
            "fb.error.open_failed",
            "fb.error.unknown",
        ] {
            assert!(zh.contains(k), "locales/zh-CN.yml 缺少键 {}", k);
        }
        // 错误码映射里出现的每个键都必须真实存在（否则界面直接显示键名）
        for code in ["INVALID_ROOT", "INVALID_PATH", "NOT_FOUND", "NOT_ALLOWED", "READ_FAILED", "OPEN_FAILED"] {
            assert!(
                zh.contains(actions::fileroot_error_key(code)),
                "错误码 {} 的键未收录进 locales",
                code
            );
        }
        // 模块箱名称/简介也必须成对出现（顶栏导航按 name_key 渲染）
        let zh_mod = keys_with_prefix(ZH, "mod.file_browser.");
        let en_mod = keys_with_prefix(EN, "mod.file_browser.");
        assert_eq!(zh_mod, en_mod, "mod.file_browser.* 键集合必须一致");
        assert!(zh_mod.contains("mod.file_browser.name"), "缺少 mod.file_browser.name");
        assert!(zh_mod.contains("mod.file_browser.desc"), "缺少 mod.file_browser.desc");
    }

    /// 抽 yml 里以某前缀开头的扁平键（取每行第一个 ':' 之前的部分）
    fn keys_with_prefix(yml: &str, prefix: &str) -> std::collections::BTreeSet<String> {
        yml.lines()
            .filter_map(|l| {
                let l = l.trim();
                if l.is_empty() || l.starts_with('#') {
                    return None;
                }
                let (k, _) = l.split_once(':')?;
                let k = k.trim();
                if k.starts_with(prefix) {
                    Some(k.to_string())
                } else {
                    None
                }
            })
            .collect()
    }
}
