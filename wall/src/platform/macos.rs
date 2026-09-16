//! macOS 一档：茧壁**直调 Seatbelt**（`sandbox_init` FFI），**不经** `sandbox-exec` 命令行。
//!
//! **为什么直调**（设计稿 §五之二，Mr2109 2026-09-16 拍板「沙箱程序必须用 Rust 写」）：
//! `sandbox-exec` 自 Sierra 起被 Apple 标 DEPRECATED、官方替代至今缺席
//! （`apple/containerization#737` 自 2026-05-12 Open 无回复）；`sandbox_init` 是同一条内核接口的
//! **库入口** —— 本机实测（macOS 26，2026-09-16）仍可用：`rc=0` 且写空间外被拦 ✓。
//! **弃用风险照实标**（进 `note`）：两处都是「弃用但仍可用」，茧壁不假装它是稳定契约。
//!
//! **平台差异（如实说，别糊）**：macOS **没有视图级封闭** —— 拿不到 mount/pid namespace，
//! 所以「空间内路径」这一层不存在：声明里的路径**按宿主路径原样使用**，茧壁**不做「空间↔宿主」映射**
//! ✗（映射是视图层的事，凭空映射 = 我们造第二份真相）。⇒ 本机等级只能到 `enclosed.os`（授权级）；
//! 只读面也收不窄（见 `policy` 的 `file-read*` 一条）。
//!
//! **策略**（= `scripts/sandbox-probes/pF.sb` 的黄金配方，本机四态已实测；少放一条是一条）：
//! `(deny default)` + `process*` + `sysctl-read` + `mach-lookup`（基线）+ `file-read*`（全局，
//! **如实列进 allowlist**）+ `file-write* (subpath 声明的可写区)` + `(allow network-bind)`
//! （**不加过滤**：加了会把 unix socket 挡掉，§10.6 实测 `PermissionError:1`）+
//! `network-inbound/outbound` 限 localhost ⇒ 空间内 IPC/loopback 可用、**出网被拦** ✓。

use std::collections::BTreeSet;
use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int, c_ulong};

use crate::error::Error;
use crate::shim::{exec_wrapper_script, SH_PATH, WRAPPER_ARG0};
use crate::spec::{split_bind, Spec};
use crate::{Plan, PLAN_SCHEMA_VERSION};

// Seatbelt 的库入口（libSystem）。`flags = 0` ⇒ 第一个参数按**策略文本**解释
// （`SANDBOX_NAMED = 1` 才是「按名字取预置 profile」，我们不走那条：预置 profile 不受卵声明控制）。
extern "C" {
    fn sandbox_init(profile: *const c_char, flags: c_ulong, errorbuf: *mut *mut c_char) -> c_int;
    fn sandbox_free_error(errorbuf: *mut c_char);
}

/// 基线说明（写进 `note`；**基线项不进 allowlist** —— allowlist 只列「超出基线」的授权）。
pub const BASELINE_NOTE: &str =
    "基线 = (deny default) + (allow process*) + (allow sysctl-read) + (allow mach-lookup)";

/// 一次「策略」的产物：**策略文本**与**放行清单**同源产出。
///
/// 两者必须同源：分开写就会漂移 —— 「清单里说 A、实际施加的是 B」正是本批要消灭的形态
/// （§6.9 精神）。故这里只有 [`policy`] 一个产出口，`plan` 与 `run` 都取它。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Policy {
    /// Seatbelt 策略文本（`sandbox_init` 的第一个参数）。
    pub profile: String,
    /// 放行清单：**超出基线**的每一项授权（基线见 [`BASELINE_NOTE`]）。
    pub allowlist: Vec<String>,
}

/// 由卵配方产出策略（纯函数，可离线单测；**不 exec、不写盘**）。
///
/// 可写区 = `work_dir` ∪ 可写绑定的**宿主侧**（macOS 无空间 ⇒ 宿主路径就是生效路径；
/// 空间侧那一半在 macOS 上无意义，故**不用**它，免得悬空引用一个不存在的落点）。
pub fn policy(s: &Spec) -> Result<Policy, Error> {
    s.validate()?;

    let mut writable: Vec<String> = Vec::new();
    if !s.work_dir.trim().is_empty() {
        writable.push(s.work_dir.trim().to_string());
    }
    for b in &s.extra_rw_binds {
        let (host, _space) = split_bind("可写", b)?;
        writable.push(host.to_string());
    }
    // 去重（保持首次出现顺序 ⇒ 策略文本逐字可复现）
    let mut seen: BTreeSet<String> = BTreeSet::new();
    writable.retain(|p| seen.insert(p.clone()));
    for p in &writable {
        reject_control_chars(p)?;
    }

    let mut profile = String::new();
    profile.push_str("(version 1)\n");
    profile.push_str("(deny default)\n");
    profile.push_str("(allow process*)\n");
    profile.push_str("(allow sysctl-read)\n");
    profile.push_str("(allow mach-lookup)\n");
    // 只读面**收不窄**：dyld/Framework/解释器的加载面在 macOS 上无法按声明路径收口
    // （收窄方案**未实测** ⇒ 不许当「已验证」用），故按黄金配方给全局只读，并如实列进 allowlist。
    profile.push_str("(allow file-read*)\n");
    for p in &writable {
        profile.push_str(&format!(
            "(allow file-write* (subpath {}))\n",
            seatbelt_quote(p)
        ));
    }
    profile.push_str("(allow network-bind)\n");
    profile.push_str("(allow network-inbound (local ip \"localhost:*\"))\n");
    profile.push_str("(allow network-outbound (remote ip \"localhost:*\"))\n");

    let mut allowlist: Vec<String> = vec![
        "file-read*:全局（**比基线大得多**：macOS 没有视图级封闭，只读面收不到声明路径）"
            .to_string(),
    ];
    for p in &writable {
        allowlist.push(format!("rw-path:{p}（宿主路径）"));
    }
    allowlist.push(
        "network-bind（**不过滤** —— 加过滤会把 unix socket 挡掉：§10.6 实测 PermissionError:1）"
            .to_string(),
    );
    allowlist.push("network-inbound:localhost".to_string());
    allowlist.push("network-outbound:localhost".to_string());
    for k in s.env.keys() {
        allowlist.push(format!("env:{k}"));
    }

    Ok(Policy { profile, allowlist })
}

/// 在**本进程**施加策略（`run` 的 exec 之前调用；macOS 一档没有外部沙箱程序可代劳）。
///
/// 失败一律**报错退出**，绝不改道：**不回落成裸跑**、不换别的沙箱、不「尽力而为」✓
/// （回落 = 一个没有封闭、却看起来正常的空间 —— 设计稿 §五/§六 反复点名的形态）。
pub fn apply(profile: &str) -> Result<(), Error> {
    let c = CString::new(profile).map_err(|_| Error::new("策略文本里有 NUL 字节——拒（不猜）"))?;
    let mut err: *mut c_char = std::ptr::null_mut();
    let rc = unsafe { sandbox_init(c.as_ptr(), 0, &mut err) };
    if rc == 0 {
        return Ok(());
    }
    let detail = if err.is_null() {
        "（内核未给错误串）".to_string()
    } else {
        let s = unsafe { CStr::from_ptr(err) }.to_string_lossy().to_string();
        unsafe { sandbox_free_error(err) };
        s
    };
    Err(Error::new(&format!(
        "sandbox_init 失败（rc={rc}：{detail}）——拒：**绝不**回落成裸跑（不换沙箱、不尽力而为）"
    )))
}

/// 出计划：配方 ⇒ 策略（本进程施加）+ 命令 + 放行清单 + 如实说明。
///
/// `argv` 是**封闭之内**要跑的那条命令行：macOS 一档的封闭由茧壁本进程经 `sandbox_init` 施加
/// （不经 `argv`，也没有外部沙箱程序可写进去）⇒ 策略与清单由 `note`/`allowlist` 如实交代。
pub fn plan(s: &Spec) -> Result<Plan, Error> {
    let pol = policy(s)?;

    // 入口：有按卵环境变量 ⇒ 包装 exec（与 Linux 侧**同一份**构造，见 `crate::shim`）；
    // 没有 ⇒ 直 exec（多一层 sh 会把 execve 失败的归因变含糊）
    let mut argv: Vec<String> = Vec::new();
    match exec_wrapper_script(&s.env) {
        Some(script) => {
            argv.push(SH_PATH.to_string());
            argv.push("-c".to_string());
            argv.push(script);
            argv.push(WRAPPER_ARG0.to_string());
            argv.push(s.engine_path_in_space.clone());
        }
        None => argv.push(s.engine_path_in_space.clone()),
    }
    argv.extend(s.engine_args.iter().cloned());

    let mut note = format!(
        "{}；{BASELINE_NOTE}；macOS **没有视图级封闭** ⇒ 声明里的路径按**宿主路径原样**使用\
         （茧壁不做「空间↔宿主」映射）；策略由茧壁**本进程**经 sandbox_init 施加（不经 argv），\
         `argv` 是**封闭之内**要跑的那条命令行；设备面（devices）在 macOS 上不产生策略项\
         （Seatbelt 的对应原语是 iokit-open，**未实测** ⇒ 只记不宣称）；声明的只读面\
         （engine_roots / weight_files / extra_ro_binds）在 macOS 上**不产生额外授权**\
         （已被 file-read* 覆盖）；observed 等级由**外部实读**给出\
         （茧壁不判定等级，照 GKE「沙箱内自报不可信」）",
        crate::platform::tier_note("macos")
    );
    if s.memlock_bytes > 0 {
        note.push_str(&format!(
            "；memlock_bytes={} 不在本策略内（macOS 侧由承接层用 setrlimit 下发）",
            s.memlock_bytes
        ));
    }
    if !s.weight_path.trim().is_empty() {
        note.push_str(&format!(
            "；weight_path={} 仅出证用、不参与任何授权（只读已全局放行）",
            s.weight_path
        ));
    }

    Ok(Plan {
        schema_version: PLAN_SCHEMA_VERSION,
        platform: "macos".to_string(),
        argv,
        allowlist: pol.allowlist,
        note,
    })
}

/// 把路径包成 Seatbelt 的字符串字面量（转义 `\` 与 `"`）。
///
/// 策略文本是我们拼出来的 ⇒ 路径里的引号若原样落进去就能**改写策略**（注入一行新规则）；
/// 控制字符（含换行）不在这里转义而是**直接拒**（见 [`reject_control_chars`]）：策略文本一行一条规则，
/// 换行能凭空多出规则，且 Seatbelt 对字符串里的原始换行行为不值得赌。
fn seatbelt_quote(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            _ => out.push(c),
        }
    }
    out.push('"');
    out
}

/// 路径里带控制字符 ⇒ 拒（策略文本一行一条规则：换行/回车能改写策略）。
fn reject_control_chars(p: &str) -> Result<(), Error> {
    if let Some(c) = p.chars().find(|c| c.is_control()) {
        return Err(Error::new(&format!(
            "路径 {p:?} 里含控制字符（U+{:04X}）——拒：策略文本一行一条规则，换行能改写策略",
            c as u32
        )));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn seatbelt_quote_escapes_quotes_and_backslashes() {
        assert_eq!(seatbelt_quote("/a b"), "\"/a b\"");
        assert_eq!(seatbelt_quote("/a\"b"), "\"/a\\\"b\"");
        assert_eq!(seatbelt_quote("/a\\b"), "\"/a\\\\b\"");
    }

    #[test]
    fn control_chars_in_path_are_refused() {
        assert!(reject_control_chars("/private/tmp/ok").is_ok());
        let err = reject_control_chars("/private/tmp/a\nb").expect_err("换行必须拒");
        assert!(err.msg().contains("控制字符"), "{}", err.msg());
    }
}
