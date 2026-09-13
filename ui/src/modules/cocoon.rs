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

/// 宿主 ↔ 茧 的通道（设计 §三：`CocoonCtx` 给的是**宿主能力**，茧不能越过平台）
///
/// 本步只有一条**已接线**的通道：茧 → 宿主「退回平台栅格」。
/// 其余宿主能力（当前根 / 可写判定 / 调 API 的通道）等第一个**服务型茧**（文档茧）接入时
/// 按需加——本步不凭空造没有消费者的字段（无消费者的能力面只会烂掉）。
#[derive(Debug, Default)]
pub struct CocoonCtx {
    /// 茧 → 宿主：请求退出回平台栅格（示例虫茧面包屑「← 虫茧平台」）。宿主每帧 `render` 后读。
    pub exit_requested: bool,
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

/// 在册的茧（**唯一登记表**——一行一个茧）
pub const REGISTERED: &[CocoonMeta] = &[ROUNDTABLE];

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
}
