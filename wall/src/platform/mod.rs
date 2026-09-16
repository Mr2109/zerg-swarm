//! 平台适配层 —— **一个契约，多种落地**（设计稿 §4.1：契约与实现分离）。
//!
//! 铁律（照抄 A3S-Lab/Sandbox 的自述）：**平台模块执行决策，不得在宿主能力缺失时静默放宽它**
//! ✗ —— 平台不支持 / 一档没落地 / 二进制不在 PATH，全都必须**报错**，绝不回落成裸 exec 或「尽力而为」。

pub mod linux;
pub mod macos;

/// 平台可用性一句话（给 `note` 用；不进任何安全判断）。
pub fn tier_note(os: &str) -> &'static str {
    match os {
        "linux" => "一档：调用 bwrap（封闭强度来自内核原语 + bubblewrap，茧壁不自研锁芯）",
        "macos" => "一档：直调 Seatbelt（沙箱强度受平台限制，只有授权级、没有「看不见」那一级）",
        _ => "未知平台：无落地",
    }
}

/// **封闭由谁施加** —— 两个一档在这里唯一的形态差异。
///
/// 为什么要把这件事显式建模（而不是散在 `main` 里写 `if macos`）：它是「`argv` 是执行面」这条
/// 冻结口径在 macOS 上的落点 —— Linux 把封闭写进 `argv`（bwrap），macOS 没有可写进去的外部沙箱程序
/// ⇒ 只能由茧壁**本进程**施加。两种形态都必须**如实交代**（`allowlist` + `note`），不许看起来一样。
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Confinement {
    /// 由外部沙箱程序施加（Linux 一档：bwrap 在 `argv` 里）—— 茧壁自己不动手。
    External,
    /// 由茧壁本进程直调内核接口施加（macOS 一档：Seatbelt `sandbox_init`）。
    InProcess { profile: String },
}

impl Confinement {
    /// exec 之前施加。**失败 ⇒ 报错**，绝不改道（不回落成裸跑、不换沙箱、不「尽力而为」）。
    pub fn enforce(&self) -> Result<(), Error> {
        match self {
            Confinement::External => Ok(()),
            Confinement::InProcess { profile } => macos::apply(profile),
        }
    }
}

use crate::error::Error;
use crate::spec::Spec;

/// 按平台给出「封闭由谁施加」；认不得的平台 ⇒ 拒（与 `plan_for` 同一套口径，绝不静默放过）。
pub fn confinement_for(os: &str, s: &Spec) -> Result<Confinement, Error> {
    match os {
        "linux" => Ok(Confinement::External),
        "macos" => Ok(Confinement::InProcess {
            profile: macos::policy(s)?.profile,
        }),
        other => Err(Error::new(&format!(
            "平台 {other:?} 认不得（本端只认 linux / macos）——拒绝：不许降级成裸 exec"
        ))),
    }
}
