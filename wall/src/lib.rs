//! 茧壁（`zerg-wall`）—— 设计稿《设计-茧壁-统一封闭契约与等级自证-20260916》§五之二。
//!
//! **它是什么**：一个**独立 Rust 程序**，把「卵配方（绑定 / 环境 / 限额 / 设备）」翻译成**平台命令**，
//! 并**如实报出自己额外放行了什么**（`allowlist`）。一档：Linux 调 `bwrap`、macOS 直调 Seatbelt；
//! 二档（直调 unshare/seccomp/landlock）不在本批。
//!
//! **它不是什么（刻意不做，别在这里补）**：
//!   - **不判定等级** ✗：`observed` 一律由**外部实读**给出（设计稿 §3.1 引 GKE：「沙箱内自报不可信」）
//!     ⇒ 茧壁唯一的自报面是 `allowlist`（它额外放行了什么），**不新增等级字段、不改 `Verdict` 形状**；
//!   - **不猜测、不补默认值** ✗：配方缺字段 / 陌生 `schema_version` / 认不得的平台一律**拒绝**
//!     （**绝不**回落成裸 exec —— 那正是「宣称封闭、实际直跑」的事故形态）；
//!   - **不放宽策略** ✗：照抄 A3S 的铁律「平台模块执行决策，不得在宿主能力缺失时静默放宽它」。
//!
//! **对外接口（批 2'.1 冻结，两侧只认它）**：见 `README.md` 与 `main.rs` 的用法说明。

pub mod error;
pub mod json;
pub mod platform;
pub mod shim;
pub mod spec;

pub use error::Error;
pub use spec::Spec;

/// 计划（`plan`）输出的格式版本。**与卵声明格式版本（Go 侧 `registry.EggSchemaVersionCurrent`）同值**：
/// 跨语言契约，改一处必须同步改另一处（两处都在注释里点名对方）。
pub const PLAN_SCHEMA_VERSION: u32 = 1;

/// 一次「计划」的产物：要跑什么 + 额外放行了什么。
///
/// `argv` 与 `allowlist` **两个字段分开**（判据 8 的精神）：前者是**执行面**（逐字可复现），
/// 后者是**自报面**（超出基线放了什么）；**不许**把两者压成一个「够不够安全」的布尔。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Plan {
    /// 格式版本（=`PLAN_SCHEMA_VERSION`）。
    pub schema_version: u32,
    /// 出这份计划的平台（`linux` / `macos`）。
    pub platform: String,
    /// 要执行的命令（`argv[0]` 是可执行名，其余是参数）。
    ///
    /// Linux 一档 = `bwrap` 的**完整命令行**（封闭由 bwrap 施加）；macOS 一档 = **封闭之内**要跑的
    /// 那条命令行（封闭由茧壁本进程经 `sandbox_init` 施加，不经 argv —— 见 `platform::macos`）。
    pub argv: Vec<String>,
    /// 放行清单：**超出基线**的每一项授权（基线见 `platform::linux::BASELINE_NOTE`）。
    pub allowlist: Vec<String>,
    /// 如实说明（含「哪些东西不在本 argv 内」这类必须说清的事）。
    pub note: String,
}

impl Plan {
    /// 序列化成**一行 JSON**（`plan` 子命令的 stdout 就是它）。
    ///
    /// 手写而非引 serde：字段少且固定，且依赖政策是零第三方依赖（见 `Cargo.toml`）。
    pub fn to_json(&self) -> String {
        let mut s = String::from("{");
        s.push_str(&format!("\"schema_version\":{}", self.schema_version));
        s.push_str(&format!(",\"platform\":{}", json::quote(&self.platform)));
        s.push_str(",\"argv\":[");
        for (i, a) in self.argv.iter().enumerate() {
            if i > 0 {
                s.push(',');
            }
            s.push_str(&json::quote(a));
        }
        s.push_str("],\"allowlist\":[");
        for (i, a) in self.allowlist.iter().enumerate() {
            if i > 0 {
                s.push(',');
            }
            s.push_str(&json::quote(a));
        }
        s.push_str(&format!("],\"note\":{}}}", json::quote(&self.note)));
        s
    }
}

/// 本机平台名（与 `Plan.platform` 同一套取值）。
pub fn current_platform() -> String {
    std::env::consts::OS.to_string()
}

/// 按本机平台出计划。
pub fn plan(spec: &Spec) -> Result<Plan, Error> {
    plan_for(&current_platform(), spec)
}

/// 按**指定平台**出计划（测试用；生产走 `plan`）。
///
/// 认不得的平台 / 该平台一档尚未落地 ⇒ **拒绝**，并且在错误里说清是「没做」还是「认不得」
/// —— 两者都不许回落成裸 exec（回落 = 一个没有封闭、却看起来正常的空间）。
pub fn plan_for(os: &str, spec: &Spec) -> Result<Plan, Error> {
    match os {
        "linux" => platform::linux::plan(spec),
        "macos" => platform::macos::plan(spec),
        other => Err(Error::new(&format!(
            "平台 {other:?} 认不得（本端只认 linux / macos）——拒绝：不许降级成裸 exec"
        ))),
    }
}
