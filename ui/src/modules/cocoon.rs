//! 虫茧契约（C9 第 1 步——2026-09-13，Mr2109 拍板 Q2「做茧契约」）
//!
//! 宿主只认**一份契约**：`CocoonMeta`（铭牌）+ `Cocoon::render`（吊点）。
//! 每个茧是一个**独立 git 仓**（自带界面 + 自带后端服务）；宿主通过 `path` 依赖在**编译期**
//! 把茧的**界面**装进来（可选 feature），茧的**后端**是它自己的进程（服务型茧自拉起）。
//!
//! **新增一个茧 = 就这四步**（宿主其余代码零改动）：
//!   ① 新仓（独立 git——`zerg-cocoon/<茧名>`）
//!   ② `ui/Cargo.toml` 一行 `optional` path 依赖 + 一行 feature
//!   ③ 本文件一行 `impl Cocoon for <茧类型>`
//!   ④ 本文件 `REGISTERED` 表一行铭牌
//! ⇒ 平台栅格卡片、装载、路由、i18n 文案全部由**铭牌驱动**。
//!
//! 设计：`docs/01-设计/设计-虫茧独立仓-20260913.md` §三（目标架构/契约）+ §零 Q2。

use eframe::egui;

/// 茧的形态——同一套字段要能表达「示例虫茧」（纯 UI）与未来的「文档茧」（UI + 自带后端服务）
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CocoonKind {
    /// UI 茧：只有界面，不起自己的进程（数据/AI 走宿主 API 或自备）——示例虫茧
    Ui,
    /// 服务型茧：界面 + **自带后端服务进程**（自己的端口、自己的启停）——文档茧（C9 第 2 步）
    ///
    /// 本步还没有服务型茧（第一个是第 2 步的「文档」）⇒ 暂无构造点；
    /// 形态先立进契约，免得第 2 步再来改契约（改契约 = 所有茧一起改）。
    #[allow(dead_code)]
    Service,
}

/// 茧铭牌：茧自己说清「我是谁」——宿主不再写死名字/图标/描述/版本/仓地址
///
/// 字段口径（设计 §三）：`id/name/desc/version/icon` 是宿主渲染平台栅格要的；
/// `kind/repo/needs_service/capabilities` 是「独立仓 + 自带后端」这条路要的
/// （未装载 ⇒ 指向 `repo` 给安装指引；服务型茧 ⇒ `needs_service` + 能力申领）。
#[derive(Debug, Clone)]
pub struct CocoonMeta {
    /// 唯一 id（平台栅格卡片 id + 宿主路由键）
    pub id: &'static str,
    /// 默认名（未本地化的兜底；本地化优先用 `name_key`）
    pub name: &'static str,
    /// 展示名的 i18n 键（空串 ⇒ 用 `name`）
    pub name_key: &'static str,
    /// 简介的 i18n 键（空串 ⇒ 无简介）
    pub desc_key: &'static str,
    /// 图标（emoji 或 `icons::icon_text` 图标名）
    pub icon: &'static str,
    /// **茧自己的版本**（来茧 crate 的 `CARGO_PKG_VERSION`——与宿主版本解耦）
    pub version: &'static str,
    /// 形态（UI 茧 / 服务型茧）
    pub kind: CocoonKind,
    /// 茧的**独立 git 仓**地址（未装载时的「安装指引」指向它——设计 §4.3）
    pub repo: &'static str,
    /// 是否需要**自带后端服务**（`CocoonKind::Service` ⇒ true）
    pub needs_service: bool,
    /// 能力申领（宿主据此授权；未申领的能力茧拿不到——设计 §三「茧不能越过平台」）
    pub capabilities: &'static [&'static str],
    /// 本构建是否已装载（编译期 feature / 运行期安装态）
    /// false ⇒ 平台卡片照常显示 + 标「未装载」+ 给安装指引（绝不静默失败——设计 §4.3）
    pub loaded: bool,
}

/// 借用宿主 md 编辑器（`CocoonCtx::markdown_editor`）的返回——**窄三态**。
///
/// 契约**不**暴露 Ferrite 的类型/缓冲/历史（`MdEditor`/`TextBuffer`/`EditOp` 一个都不出现在这里）：
/// 茧只看到「有没有改过 / 有没有要求保存」。这样 Ferrite 内部怎么演进都不牵动任何茧。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EditorOutcome {
    /// 本次调用没有改动（首次装载、或只是渲染了一帧）。
    Unchanged,
    /// 文本被编辑过——调用方传进来的 `text` **已写回**最新内容（茧据此置「未保存」标记）。
    Changed,
    /// 用户在编辑器里按了保存快捷键（⌘S / Ctrl+S）——茧据此落盘。
    SaveRequested,
}

impl EditorOutcome {
    /// 是否产生改动（`Changed` 或 `SaveRequested`）。
    pub fn changed(self) -> bool {
        !matches!(self, EditorOutcome::Unchanged)
    }

    /// 是否触发了保存。
    pub fn save_requested(self) -> bool {
        matches!(self, EditorOutcome::SaveRequested)
    }
}

/// 宿主 ↔ 茧 的通道（设计 §三：`CocoonCtx` 给的是**宿主能力**，茧不能越过平台）
///
/// 已接线的通道（每条都有真消费者）：
/// 1. 茧 → 宿主「退回平台栅格」：`exit_requested`（示例虫茧面包屑「← 虫茧平台」）。
/// 2. 茧 → 宿主「**借用绳编辑器**」：`markdown_editor`（C9 第 3 步——消除文档茧的编辑器降级；
///    消费者 = `zerg-cocoon/文档` 的 `editor.rs`：经 `HostEditor` 注入点接到本通道上）。
/// 3. 茧 → 宿主「**取自家服务的令牌**」：`cocoon_token`（同上消费者）。
///
/// 本对象**跨帧存活**（宿主持有一份，每帧只清 `exit_requested`）——因为编辑器池必须跨帧保状态
/// （光标/撤销/滚动）。故 `Default` 之外不再逐帧新建。
#[derive(Default)]
pub struct CocoonCtx {
    /// 茧 → 宿主：请求退出回平台栅格（示例虫茧面包屑「← 虫茧平台」）。宿主每帧 `render` 后读。
    pub exit_requested: bool,
    /// 宿主共享令牌（茧自家服务用）——宿主装配时注入，**只驻内存、不落盘**。
    token: Option<String>,
    /// 宿主能力：绳编辑器池——每个 `id` 一份 `MdEditor`（跨帧保留光标/撤销/滚动）。
    ///
    /// `MdEditor` 是宿主**自有**能力（`modules/ferrite/`，对话等模块共用）：这里只是**借出**，
    /// 不复制进任何茧（红旗：不把 Ferrite 代码复制进任何地方）。
    editors: std::collections::HashMap<String, crate::modules::ferrite::MdEditor>,
    /// 上一次交给每个编辑器渲染的文本——用来识别「调用方换了内容」⇒ 重新载入缓冲。
    handed: std::collections::HashMap<String, String>,
}

impl std::fmt::Debug for CocoonCtx {
    /// 手写（`MdEditor` 无 Debug；也**故意**不打印令牌明文）。
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("CocoonCtx")
            .field("exit_requested", &self.exit_requested)
            .field("editors", &self.editors.len())
            .field("token", &self.token.is_some())
            .finish()
    }
}

impl CocoonCtx {
    /// 宿主装配：带共享令牌建一条通道（令牌从宿主既有解析链取，见 `host_token`）。
    pub fn with_host_token(token: Option<String>) -> Self {
        Self {
            token: token.filter(|t| !t.trim().is_empty()),
            ..Default::default()
        }
    }

    /// **宿主能力：借用宿主的 md 编辑器（Ferrite）渲染一段可编辑文本。**
    ///
    /// - `id`：这份编辑缓冲在宿主里的键——同一个茧的同一份文档要一直用同一个 `id`
    ///   （否则每次都是新缓冲，光标与撤销会丢）；
    /// - `text`：进 = 当前内容；出 = 编辑后的内容（有改动时宿主**已写回**）；
    /// - 返回 [`EditorOutcome`]：没改 / 改过 / 要求保存。
    ///
    /// 为什么契约上要有这条：茧是**独立仓**，不能反向依赖宿主、更不能把船体的 Ferrite
    /// （rope/comrak/syntect，约 1500 行）复制一份；没有它，茧只能用 `TextEdit` 凑合 ⇒
    /// **体验降级**（C9 第 3 步要消除的正是它）。接口刻意窄：只给「渲染一段文本 + 三态返回」。
    pub fn markdown_editor(&mut self, ui: &mut egui::Ui, id: &str, text: &mut String) -> EditorOutcome {
        // 调用方换了内容（首次 / 切换文档 / 宿主回填）⇒ 重新装载；否则保留编辑器里的编辑与历史。
        let needs_load = self.handed.get(id).map(|prev| prev != text).unwrap_or(true);
        let ed = self
            .editors
            .entry(id.to_string())
            .or_insert_with(crate::modules::ferrite::MdEditor::new);
        if needs_load {
            ed.load(text);
        }
        ed.render(ui);
        let now = ed.text();
        let changed = now != *text;
        if changed {
            *text = now;
        }
        self.handed.insert(id.to_string(), text.clone());
        // 保存快捷键优先于「改动」——茧看到 SaveRequested 就知道该落盘了。
        if save_shortcut(ui) {
            EditorOutcome::SaveRequested
        } else if changed {
            EditorOutcome::Changed
        } else {
            EditorOutcome::Unchanged
        }
    }

    /// 该茧自家服务所需的令牌（宿主从 `ZERG_AUTH_TOKEN` / `ZERG_API_TOKEN` / 偏好文件 /
    /// `~/.zerg/token` 的既有链取；**只驻内存，不落盘、不写日志**）。
    /// 未注入（宿主没配令牌）⇒ `None`——茧据此显「未配置」，而不是拿空串去撞 401。
    pub fn cocoon_token(&self) -> Option<&str> {
        self.token.as_deref()
    }
}

/// ⌘S / Ctrl+S（本帧、非 repeat）——`markdown_editor` 据此返回 `EditorOutcome::SaveRequested`。
/// 判定走**事件自带的修饰键**（不依赖 `InputState.modifiers` 的帧序）⇒ 可被单测直接喂事件。
fn save_shortcut(ui: &egui::Ui) -> bool {
    ui.input(|i| {
        i.events.iter().any(|e| {
            matches!(
                e,
                egui::Event::Key {
                    key: egui::Key::S,
                    pressed: true,
                    repeat: false,
                    modifiers,
                    ..
                } if modifiers.command
            )
        })
    })
}

/// 茧契约（吊点）——任何独立应用实现它 = 一个虫茧
pub trait Cocoon {
    /// 铭牌（茧自描述）。
    ///
    /// 设计稿写的是关联函数 `fn meta() -> CocoonMeta`；但注册表要以 `Vec<Box<dyn Cocoon>>`
    /// 持有**实例**并按 id 检索，而关联函数不在 vtable 上（`dyn Cocoon` 调不到、`where Self: Sized`
    /// 又被排除）⇒ 铭牌做成**实例方法**。字段全是 `'static`，调用零成本。
    fn meta(&self) -> CocoonMeta;

    /// 吊点：渲染自己（嵌在宿主中央区，**不画标题**——设计 §4.4「模块页不重复模块名标题」）。
    /// 签名对齐现有 egui 装载方式（`RoundtableApp::render(&mut self, ui: &mut egui::Ui)`），
    /// 另加一条宿主通道 `ctx`。
    fn render(&mut self, ui: &mut egui::Ui, ctx: &mut CocoonCtx);
}

// ─── 登记点（唯一——新茧在这里加一行）────────────────────────────────────────

/// 示例虫茧铭牌（**第一个茧**——独立仓 `Mr2109/zerg-cocoon-roundtable`）
///
/// `version`：跨仓在编译期取不到依赖 crate 的 `CARGO_PKG_VERSION`（cargo 不给消费者导出），
/// 故与茧自己的 `Cargo.toml` **手工同步**——由本模块测试守（`zerg_roundtable::version()`；
/// 公开镜像未启用该 feature ⇒ 该测试不编译）。
pub const ROUNDTABLE: CocoonMeta = CocoonMeta {
    id: "roundtable",
    name: "Roundtable",
    name_key: "cocoon.roundtable.name",
    desc_key: "cocoon.roundtable.desc",
    icon: "📖",
    version: "1.0.1",
    kind: CocoonKind::Ui,
    repo: "https://github.com/Mr2109/zerg-cocoon-roundtable",
    needs_service: false,
    // 示例虫茧自备引擎（自有库 + 自有 gateway token）⇒ 不向宿主申领 API 能力；
    // 只申领一条**真实的**：切走后宿主保留其实例（app 的茧实例缓存）⇒ 引擎后台继续。
    capabilities: &["background.run"],
    // 编译期 feature = 装载态：公开镜像未启用 ⇒ false ⇒ 卡片显「未装载」+ 安装指引
    loaded: cfg!(feature = "zerg-roundtable"),
};

/// 文档茧铭牌（**第一个服务型茧**——独立仓 `Mr2109/zerg-cocoon-docs`，自带 Go 服务，端口默认 8610）
///
/// ⚠ C9 第 3 步：本步**只登记铭牌**（发现 + 未安装提示）——**不**把茧代码编进宿主（第 4 步才
///   加 path 依赖 + feature）。故 `loaded` 恒为 false ⇒ 平台页出「未安装文档 → 安装」（指向其独立仓）。
/// `version`：跨仓在编译期取不到依赖 crate 的 `CARGO_PKG_VERSION` ⇒ 与茧自己的 `Cargo.toml`
/// **手工同步**（同示例虫茧口径；第 4 步接上依赖后改由 `zerg_cocoon_docs::version()` 守）。
pub const DOCS: CocoonMeta = CocoonMeta {
    id: "docs",
    name: "Docs",
    name_key: "cocoon.docs.name",
    desc_key: "cocoon.docs.desc",
    icon: "📚",
    version: "0.1.0",
    kind: CocoonKind::Service,
    repo: "https://github.com/Mr2109/zerg-cocoon-docs",
    needs_service: true,
    // 服务型茧**必须**申领能力（fail-closed，见 service_kind_requires_service_flag_and_capabilities）：
    // docs.read / docs.write = 对它**自己那份**文档目录的读写；service.self = 自带进程自管。
    capabilities: &["docs.read", "docs.write", "service.self"],
    // 装载态 = 编译期 feature：第 4 步装上 `zerg-cocoon-docs` 后才是 true。
    // 本步该 feature 无 `dep:`（只有声明，见 Cargo.toml）⇒ 永不被启用 ⇒ 卡片恒显「未安装」。
    loaded: cfg!(feature = "zerg-cocoon-docs"),
};

/// 在册的茧（**唯一登记表**——一行一个茧）
pub const REGISTERED: &[CocoonMeta] = &[ROUNDTABLE, DOCS];

/// 全部茧的铭牌（平台栅格 / 未装载提示的数据源——**不需要实例**）
pub fn catalog() -> &'static [CocoonMeta] {
    REGISTERED
}

/// 按 id 查铭牌（`None` = 该 id 不是茧 ⇒ 宿主内建应用，见 `app::cocoon_openable`）
pub fn meta_of(id: &str) -> Option<&'static CocoonMeta> {
    catalog().iter().find(|m| m.id == id)
}

/// 懒装载：按 id 建实例（`None` = 本构建未装载该茧 ⇒ 平台停在栅格 + 安装指引）
///
/// 懒加载（VS Code Activation Events 经验）：示例虫茧 `new()` 会开自己的库 + 起 tokio runtime，
/// 既不能每帧重建、也不该在宿主启动时就建 ⇒ 第一次「打开」才建，实例缓存在 `ZergApp.cocoons`。
pub fn load(id: &str) -> Option<Box<dyn Cocoon>> {
    match id {
        #[cfg(feature = "zerg-roundtable")]
        "roundtable" => Some(Box::new(zerg_roundtable::ui::RoundtableApp::new()) as Box<dyn Cocoon>),
        _ => None,
    }
}

/// 平台卡片能否进入（**铭牌驱动**）：未装载 ⇒ 不可进入（卡片照常 + 安装指引，绝不静默失败）。
/// 宿主 `app::cocoon_openable` 用同一判据 ⇒ 这条口径只有一处（纯函数，可单测两态）。
pub fn openable(m: &CocoonMeta) -> bool {
    // 宿主仍内建渲染的内置页（如 docs）：即使该茧未装载，也保留可进入 —— 避免「未装载 ⇒ 安装」
    // 期间出现功能空档（Mr2109 口径：不留可见空档）。茧装载后走 loaded 分支。
    if host_builtin_openable(m.id) {
        return true;
    }
    m.loaded
}

/// 未装载茧的安装指引数据源（平台页与测试共用）：返回它的**独立仓地址**。
///
/// `None` ⇒ 不需要安装指引（已装载 / 不是茧）——平台页据此只画「打开」，不画「安装」。
/// 设计 §4.3：点「安装」⇒ 告知独立仓地址 + 目标目录 + 「clone 后重新编译装载」，绝不假装可用。
pub fn install_guide(id: &str) -> Option<&'static str> {
    match meta_of(id) {
        Some(m) if !m.loaded && m.repo.starts_with("http") => Some(m.repo),
        _ => None,
    }
}

/// 宿主装配用的**共享令牌**（复用宿主既有解析链 `api::api_token`：环境变量 → 偏好文件 →
/// `~/.zerg/token`）。空 ⇒ `None`（未配置）。**只驻内存**——经茧契约拿到的一律不落盘。
pub fn host_token() -> Option<String> {
    let t = crate::api::api_token();
    if t.trim().is_empty() {
        None
    } else {
        Some(t.to_string())
    }
}

// ─── 示例虫茧 = 契约的第一个实现 ──────────────────────────────────────────────
//
// 茧**不依赖宿主**（它是独立仓，不能反向依赖 zerg-ui）：这份 glue 写在宿主侧，
// 宿主因此只认契约、不认某个茧的内部类型。

#[cfg(feature = "zerg-roundtable")]
impl Cocoon for zerg_roundtable::ui::RoundtableApp {
    fn meta(&self) -> CocoonMeta {
        ROUNDTABLE
    }

    fn render(&mut self, ui: &mut egui::Ui, ctx: &mut CocoonCtx) {
        // 嵌入态 ⇒ 示例虫茧自画「← 虫茧平台」面包屑（三层收一层 2026-09-04）
        self.embedded = true;
        // 显式走它自己的 inherent render（同名方法易混——全限定最稳）
        zerg_roundtable::ui::RoundtableApp::render(self, ui);
        // 茧 → 宿主：面包屑被点 ⇒ 请求回平台栅格（示例虫茧每帧开头重置该标志）
        if self.exit_platform {
            ctx.exit_requested = true;
        }
    }
}

/// 打印在册茧一行一个（**一次**——诊断「平台上有哪些茧、装载没装载」）。
/// 宿主 stderr 日志（M33 纪律：不静默）；真实读取铭牌全字段，避免「字段只写不读」的烂尾。
pub fn log_catalog_once() {
    static ONCE: std::sync::Once = std::sync::Once::new();
    ONCE.call_once(|| {
        for m in catalog() {
            eprintln!(
                "[cocoon] id={} name_key={} v{} kind={:?} service={} caps={:?} loaded={} repo={}",
                m.id,
                m.name_key,
                m.version,
                m.kind,
                m.needs_service,
                m.capabilities,
                m.loaded,
                m.repo
            );
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    /// 宿主源码（断言「宿主只认契约」：具体茧的 crate 名不得出现在宿主代码里）
    const APP_SRC: &str = include_str!("../app.rs");
    const MOD_SRC: &str = include_str!("mod.rs");

    /// 测试夹具：契约在**宿主侧**——这里做一个最小实现验通道语义，
    /// **绝不**去 `new()` 真茧（`RoundtableApp::new()` 会开真库 + 起 runtime + 写日志 = 真状态污染）。
    struct FakeCocoon {
        request_exit: bool,
        renders: usize,
    }

    const FAKE_META: CocoonMeta = CocoonMeta {
        id: "fake",
        name: "Fake",
        name_key: "",
        desc_key: "",
        icon: "🧪",
        version: "0.0.1",
        kind: CocoonKind::Ui,
        repo: "https://example.invalid/fake",
        needs_service: false,
        capabilities: &[],
        loaded: true,
    };

    impl Cocoon for FakeCocoon {
        fn meta(&self) -> CocoonMeta {
            FAKE_META
        }
        fn render(&mut self, ui: &mut egui::Ui, ctx: &mut CocoonCtx) {
            ui.label("fake");
            self.renders += 1;
            if self.request_exit {
                ctx.exit_requested = true;
            }
        }
    }

    /// 铭牌完整性：id 唯一非空、名字/图标/仓地址齐全（少了 id 就无法路由；少了 repo
    /// 未装载时给不出安装指引——设计 §4.3）。能失败：任一字段写空或 id 撞车。
    #[test]
    fn catalog_entries_are_complete_and_unique() {
        let all = catalog();
        assert!(!all.is_empty(), "契约注册表不能为空（平台栅格会空）");
        let mut seen = std::collections::BTreeSet::new();
        for m in all {
            assert!(!m.id.is_empty(), "茧 id 不能为空");
            assert!(seen.insert(m.id), "茧 id 重复：{}", m.id);
            assert!(!m.name.is_empty(), "{} 缺默认名（本地化键缺失时要有兜底）", m.id);
            assert!(
                !m.name_key.is_empty() || !m.name.is_empty(),
                "{} 既无 i18n 名键也无默认名 ⇒ 卡片会空白",
                m.id
            );
            assert!(!m.icon.is_empty(), "{} 缺图标", m.id);
            assert!(!m.repo.is_empty(), "{} 缺独立仓地址（未装载时给不出安装指引）", m.id);
            assert!(
                m.repo.starts_with("http"),
                "{} 的仓地址应是可点击/可复制的地址：{}",
                m.id,
                m.repo
            );
        }
    }

    /// 形态口径：服务型茧必须声明需要服务 + 至少申领一项能力（fail-closed：
    /// 服务型茧不申领能力就拿不到宿主 / 自家后端的接口面，宁可编不过也不静默降级）。
    #[test]
    fn service_kind_requires_service_flag_and_capabilities() {
        for m in catalog() {
            match m.kind {
                CocoonKind::Service => {
                    assert!(m.needs_service, "{} 是服务型茧 ⇒ needs_service 必须 true", m.id);
                    assert!(
                        !m.capabilities.is_empty(),
                        "{} 是服务型茧但没申领任何能力（fail-closed）",
                        m.id
                    );
                }
                CocoonKind::Ui => {
                    assert!(!m.needs_service, "{} 是 UI 茧 ⇒ 不该要宿主配合起服务", m.id);
                }
            }
        }
    }

    /// 装载态由编译期 feature 决定，且**未装载也留在册**（卡片照常 + 「未装载」+ 安装指引）。
    #[test]
    fn roundtable_stays_listed_and_loaded_matches_feature() {
        let m = meta_of("roundtable").expect("示例虫茧是契约的第一个实现，必须在册");
        assert_eq!(m.loaded, cfg!(feature = "zerg-roundtable"));
        assert_eq!(m.kind, CocoonKind::Ui, "示例虫茧是纯 UI 茧");
        assert_eq!(m.id, ROUNDTABLE.id);
        assert!(meta_of("not-a-cocoon").is_none());
    }

    /// 示例虫茧铭牌 = 它自己 crate 的版本（能失败：茧升版而铭牌没跟着改）。
    /// 公开镜像未启用该 feature ⇒ 不编译。
    #[cfg(feature = "zerg-roundtable")]
    #[test]
    fn roundtable_meta_version_matches_the_cocoon_crate() {
        assert_eq!(
            ROUNDTABLE.version,
            zerg_roundtable::version(),
            "铭牌版本与示例虫茧 crate 的版本漂移了（跨仓编译期取不到，只能手工同步）"
        );
    }

    /// 吊点接通：宿主在 `render` 里拿到茧的退出请求（用夹具验**通道语义**，不碰真茧）。
    #[test]
    fn render_channel_carries_exit_request() {
        let ctx = egui::Context::default();
        let mut app = FakeCocoon { request_exit: true, renders: 0 };
        let mut ch = CocoonCtx::default();
        let mut out = ctx.run_ui(Default::default(), |ui| {
            assert_eq!(app.meta().id, "fake");
            app.render(ui, &mut ch);
        });
        out.textures_delta.clear();
        assert!(ch.exit_requested, "茧请求退出 ⇒ 宿主通道必须为真");
        assert_eq!(app.renders, 1, "渲染一次（吊点被调用）");

        // 反例：不请求退出 ⇒ 通道保持默认（宿主不会误退出）
        let mut quiet = FakeCocoon { request_exit: false, renders: 0 };
        let mut ch2 = CocoonCtx::default();
        let mut out2 = ctx.run_ui(Default::default(), |ui| quiet.render(ui, &mut ch2));
        out2.textures_delta.clear();
        assert!(!ch2.exit_requested);
        assert!(!CocoonCtx::default().exit_requested);
    }

    /// 懒装载入口：未知 id ⇒ `None`（不 panic）；未编译进来的茧 ⇒ `None`。
    #[test]
    fn load_returns_none_for_unknown_and_unbuilt_cocoons() {
        assert!(load("not-a-cocoon").is_none(), "未知 id 必须 None");
        if !cfg!(feature = "zerg-roundtable") {
            assert!(
                load("roundtable").is_none(),
                "未装载的茧 ⇒ None（平台停在栅格 + 安装指引，不静默失败）"
            );
        }
        // 已装载的真茧**不在单测里 new()**：RoundtableApp::new() 会开真库 + 起 runtime + 写日志。
        // 装载路径由「铭牌 loaded」+「app::cocoon_view 按 id 懒建」两处共同保证。
    }

    /// 铭牌的 i18n 键必须在**两个 locale** 里都存在。
    /// 为什么单列一条：卡片文案现在走 `t!(m.name_key)`（动态键），i18n 门禁的
    /// 「静态引用完整」看不到它；键写错时 rust-i18n 会把**键名原样显示**在卡片上。
    #[test]
    fn meta_i18n_keys_exist_in_both_locales() {
        const ZH: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/locales/zh-CN.yml"));
        const EN: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/locales/en.yml"));
        fn has(yml: &str, key: &str) -> bool {
            yml.lines()
                .any(|l| l.split_once(':').map(|(k, _)| k.trim() == key).unwrap_or(false))
        }
        for m in catalog() {
            for key in [m.name_key, m.desc_key] {
                if key.is_empty() {
                    continue;
                }
                assert!(has(ZH, key), "zh-CN.yml 缺茧铭牌的键 {}（茧 {}）", key, m.id);
                assert!(has(EN, key), "en.yml 缺茧铭牌的键 {}（茧 {}）", key, m.id);
            }
        }
    }

    /// **宿主零硬编码**（C9 第 1 步的核心断言）：具体茧的 crate 名只允许出现在本契约模块
    /// （`impl` + 懒装载入口）与 `Cargo.toml` 依赖声明里——app/mod 等宿主代码一律不得直呼某个茧。
    /// 能失败：有人把茧的名字/图标/装载写回宿主。
    #[test]
    fn host_code_names_no_concrete_cocoon() {
        assert!(
            !APP_SRC.contains("zerg_roundtable"),
            "app.rs 仍直呼示例虫茧 crate ⇒ 应改走契约注册表（catalog/load）"
        );
        assert!(
            !APP_SRC.contains("RoundtableApp"),
            "app.rs 仍持有示例虫茧具体类型 ⇒ 应持有 Vec<Box<dyn Cocoon>>"
        );
        // 反向对照：契约模块自己必须留着这份 impl（防误删——删了懒装载入口就编不过）
        assert!(
            MOD_SRC.contains("pub mod cocoon;"),
            "modules/mod.rs 必须挂上契约模块"
        );
    }

    // ─── C9 第 3 步新增：宿主能力通道（编辑器借用 + 令牌）与未安装两态 ──────────────

    /// 夹具茧：**真调用**宿主能力通道（证明通道不是死字段）——记录每次 `EditorOutcome` 与令牌。
    struct ChannelCocoon {
        /// 茧自己的文本（传进通道，被宿主编辑器改写后写回）
        text: String,
        outcomes: Vec<EditorOutcome>,
        token_seen: Option<String>,
        token_checks: usize,
    }

    impl ChannelCocoon {
        fn new(text: &str) -> Self {
            Self {
                text: text.to_string(),
                outcomes: Vec::new(),
                token_seen: None,
                token_checks: 0,
            }
        }
    }

    impl Cocoon for ChannelCocoon {
        fn meta(&self) -> CocoonMeta {
            FAKE_META
        }
        fn render(&mut self, ui: &mut egui::Ui, ctx: &mut CocoonCtx) {
            let out = ctx.markdown_editor(ui, "channel-fixture", &mut self.text);
            self.outcomes.push(out);
            self.token_seen = ctx.cocoon_token().map(str::to_string);
            self.token_checks += 1;
        }
    }

    /// 一帧（带任意 RawInput）——省得每条测试都写一遍 `run_ui` 样板。
    fn frame(
        ctx: &egui::Context,
        raw: egui::RawInput,
        cocoon: &mut ChannelCocoon,
        ch: &mut CocoonCtx,
    ) {
        let mut out = ctx.run_ui(raw, |ui| cocoon.render(ui, ch));
        out.textures_delta.clear();
    }

    /// **宿主能力通道「借用绳编辑器」真的被消费**（C9 第 3 步的核心断言）：
    /// ① 茧把文本交给通道 ⇒ 宿主的 Ferrite 编辑器被真建出来并载入；
    /// ② 编辑器侧改了文本 ⇒ 下一次调用把改动**写回茧**（真双向，不是空壳）。
    /// 能失败：把 `markdown_editor` 换成空实现（返回恒 `Unchanged` / 不建编辑器）即红。
    #[test]
    fn markdown_editor_channel_is_really_consumed() {
        let ctx = egui::Context::default();
        let mut cocoon = ChannelCocoon::new("# 标题\n正文");
        let mut ch = CocoonCtx::default();

        frame(&ctx, Default::default(), &mut cocoon, &mut ch);
        // ① 首次：把文本交给编辑器渲染（本次没改动）
        assert_eq!(cocoon.outcomes, vec![EditorOutcome::Unchanged]);
        let ed = ch
            .editors
            .get("channel-fixture")
            .expect("宿主必须为该 id 真建出编辑器（通道真接线，不是空壳）");
        assert_eq!(ed.text(), "# 标题\n正文", "茧的文本必须进了宿主编辑器缓冲");

        // ② 反向：在宿主编辑器里编辑 ⇒ 下一帧把改动写回茧
        ch.editors
            .get_mut("channel-fixture")
            .unwrap()
            .insert_text("X");
        frame(&ctx, Default::default(), &mut cocoon, &mut ch);
        assert_eq!(cocoon.outcomes[1], EditorOutcome::Changed, "编辑器改过 ⇒ 必须报 Changed");
        assert_eq!(cocoon.text, "X# 标题\n正文", "改动必须写回茧的缓冲");

        // ③ 变异对照：两边一致 ⇒ 不得再报「改过」（证明 Changed 不是恒真）
        frame(&ctx, Default::default(), &mut cocoon, &mut ch);
        assert_eq!(cocoon.outcomes[2], EditorOutcome::Unchanged);
        // 编辑缓冲跨帧存活（同一个 id 用同一个编辑器）
        assert_eq!(ch.editors.len(), 1, "同一个 id 只能有一份编辑缓冲（光标/撤销要跨帧）");
        assert_eq!(cocoon.token_checks, 3, "每帧都真读了通道（不是摆设）");
    }

    /// ⌘S / Ctrl+S ⇒ `SaveRequested`（茧据此落盘）；无修饰键的 S 不算。
    #[test]
    fn save_shortcut_yields_save_requested_through_the_channel() {
        fn press(modifiers: egui::Modifiers) -> egui::RawInput {
            let mut raw = egui::RawInput::default();
            raw.events.push(egui::Event::Key {
                key: egui::Key::S,
                physical_key: None,
                pressed: true,
                repeat: false,
                modifiers,
            });
            raw
        }
        let ctx = egui::Context::default();
        let mut cocoon = ChannelCocoon::new("正文");
        let mut ch = CocoonCtx::default();

        frame(&ctx, press(egui::Modifiers::COMMAND), &mut cocoon, &mut ch);
        assert_eq!(
            cocoon.outcomes,
            vec![EditorOutcome::SaveRequested],
            "⌘S ⇒ 茧必须收到 SaveRequested"
        );
        frame(&ctx, press(egui::Modifiers::NONE), &mut cocoon, &mut ch);
        assert_eq!(cocoon.outcomes[1], EditorOutcome::Unchanged, "光按 S 不是保存");

        // 三态语义自证（能失败：谁改 changed()/save_requested() 的判定）
        assert!(EditorOutcome::Changed.changed() && !EditorOutcome::Changed.save_requested());
        assert!(EditorOutcome::SaveRequested.changed() && EditorOutcome::SaveRequested.save_requested());
        assert!(!EditorOutcome::Unchanged.changed() && !EditorOutcome::Unchanged.save_requested());
    }

    /// **宿主能力通道「取自家服务令牌」真的被消费**；且未注入 ⇒ `None`（不瞎编、不返回空串）；
    /// 令牌**只驻内存**——`Debug` 输出不得出现明文。
    #[test]
    fn cocoon_token_channel_is_really_consumed() {
        let ctx = egui::Context::default();
        let mut cocoon = ChannelCocoon::new("");
        let mut ch = CocoonCtx::with_host_token(Some("t0ken-through-channel".to_string()));
        frame(&ctx, Default::default(), &mut cocoon, &mut ch);
        assert_eq!(
            cocoon.token_seen.as_deref(),
            Some("t0ken-through-channel"),
            "茧必须能从通道里拿到宿主令牌"
        );
        assert_eq!(cocoon.token_checks, 1);
        // 反例：未注入 / 空白 ⇒ None
        assert_eq!(CocoonCtx::default().cocoon_token(), None, "未注入 ⇒ None");
        assert_eq!(
            CocoonCtx::with_host_token(Some("   ".to_string())).cocoon_token(),
            None,
            "空白令牌不算令牌"
        );
        let dbg = format!("{:?}", ch);
        assert!(
            !dbg.contains("t0ken-through-channel"),
            "令牌不得出现在 Debug 输出里：{}",
            dbg
        );
        assert!(dbg.contains("token: true"), "Debug 只报「有没有令牌」：{}", dbg);
    }

    /// 文档茧铭牌（本步＝**未安装**态）：形态是服务型 + 独立仓地址 + 版本口径。
    #[test]
    fn docs_meta_declares_the_service_cocoon_as_uninstalled() {
        let m = meta_of("docs").expect("文档茧必须在册（未安装也要能被『发现』）");
        assert_eq!(m.id, "docs");
        assert_eq!(m.kind, CocoonKind::Service);
        assert!(m.needs_service, "服务型茧 ⇒ needs_service 必须 true");
        assert!(!m.capabilities.is_empty(), "服务型茧必须申领能力（fail-closed）");
        assert_eq!(m.repo, "https://github.com/Mr2109/zerg-cocoon-docs");
        assert_eq!(m.version, "0.1.0", "铭牌版本与<container-repo> Cargo.toml 手工同步（跨仓编译期取不到）");
        // 本步**不**集成茧代码 ⇒ feature 不得被启用（第 4 步接上 path 依赖后此处随之更新）
        assert!(
            !cfg!(feature = "zerg-cocoon-docs"),
            "C9 第 3 步：宿主未编入文档茧 ⇒ 该 feature 不得被启用"
        );
        assert!(!DOCS.loaded, "未安装 ⇒ 铭牌 loaded=false");
    }

    /// **两态**（验收）：已装载 ⇒ 可开；未装载 ⇒ 不可开 + 给得出安装指引（指向独立仓）。
    #[test]
    fn platform_two_states_installed_vs_not_installed() {
        // ① 未装载态（本步的文档茧）
        let docs = meta_of("docs").unwrap();
        assert!(openable(docs),
            "docs 未装载但宿主仍内建渲染 ⇒ 保留入口（不留功能空档；第 4 步迁走后应改回 false）");
        assert_eq!(install_guide("docs"), Some(docs.repo), "未装载 ⇒ 安装指引指向它的独立仓");
        assert!(docs.repo.starts_with("http"), "安装指引要可点/可复制");

        // ② 已装载态：同一判据必须为「可开」——用夹具铭牌（绝不 new() 真茧）
        assert!(openable(&FAKE_META), "已装载 ⇒ 可开（防「一律不可开」的假实现）");
        let unloaded = CocoonMeta { loaded: false, ..FAKE_META };
        assert!(!openable(&unloaded));

        // ③ 不需要指引的场合：已装载的茧、不是茧的 id、未知 id
        assert_eq!(install_guide("chat"), None, "宿主内建应用不是茧 ⇒ 无安装指引");
        assert_eq!(install_guide("nope"), None);
        assert_eq!(
            install_guide("roundtable").is_some(),
            !ROUNDTABLE.loaded,
            "示例虫茧按它自己的装载态（默认构建已装载 ⇒ 无指引；公开镜像 ⇒ 有指引）"
        );
    }
}

/// 宿主**自身**仍内建渲染的内置页（未迁移进独立茧之前，平台不夺走入口）。
/// 目前只有 `docs`：它的界面代码仍内联在宿主里（第 4 步迁走后才从本表移除）。
pub fn host_builtin_openable(id: &str) -> bool {
    id == "docs"
}
