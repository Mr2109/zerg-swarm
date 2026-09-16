//! 茧壁一档（macOS/Seatbelt **直调**）的**能失败**用例（批 2'.4）。
//!
//! 四组口径各有一组用例：
//!   ① **黄金配方逐条**（= `scripts/sandbox-probes/pF.sb`，本机四态实测过的那一份）——少一条、多一条都要红；
//!   ② **不许把过滤加在 bind 上**（§10.6 实测：`(allow network-bind (local …))` 会把 unix socket 挡掉）；
//!   ③ **可写面只到声明的路径**（全局 `file-write*` 必须红 —— 那是「授权级封闭」失效的形态）；
//!   ④ **同契约不同落地**：macOS 与 Linux 接受/拒绝**同一批**配方（不许 macOS 侧偷偷放宽或收紧）。
//!
//! 本文件是**纯函数**用例（不 exec、不施加沙箱）⇒ 在任何平台上都跑；
//! 活体四态在 `tests/macos_seatbelt.rs`（仅 macOS）。

use std::collections::BTreeMap;

use zerg_wall::platform::macos;
use zerg_wall::platform::{confinement_for, Confinement};
use zerg_wall::spec::Spec;
use zerg_wall::{json, plan_for};

/// 一份 macOS 形态的配方：路径按**宿主路径**（macOS 无视图级封闭 ⇒ 声明即宿主路径）。
fn mac_spec() -> Spec {
    let mut env = BTreeMap::new();
    env.insert(
        "PROBE_DIR".to_string(),
        "/private/tmp/wall-demo/work".to_string(),
    );
    Spec {
        egg_id: "demo-egg".to_string(),
        schema_version: 1,
        engine_path_in_space: "/usr/bin/python3".to_string(),
        engine_args: vec!["/repo/scripts/sandbox-probes/miniprobe.py".to_string()],
        engine_roots: vec!["/opt/llama/build/bin".to_string()],
        weight_path: "/opt/models/demo".to_string(),
        weight_files: vec!["/opt/models/demo/model.gguf".to_string()],
        env,
        devices: vec![],
        extra_ro_binds: vec!["/opt/models/demo/model.gguf:/models/model.gguf".to_string()],
        extra_rw_binds: vec!["/private/tmp/wall-demo/kv:/work-kv".to_string()],
        work_dir: "/private/tmp/wall-demo/work".to_string(),
        memlock_bytes: 64 * 1024 * 1024,
    }
}

/// ① 黄金配方逐条：策略文本必须**逐行**含这些规则（pF 那一套，四态实测过）。
#[test]
fn policy_is_the_measured_golden_recipe() {
    let p = macos::policy(&mac_spec()).expect("好配方必须能出策略");
    for line in [
        "(version 1)",
        "(deny default)",
        "(allow process*)",
        "(allow sysctl-read)",
        "(allow mach-lookup)",
        "(allow file-read*)",
        "(allow network-bind)",
        "(allow network-inbound (local ip \"localhost:*\"))",
        "(allow network-outbound (remote ip \"localhost:*\"))",
    ] {
        assert!(
            p.profile.contains(line),
            "策略缺黄金配方里的一行：{line}\n实际：\n{}",
            p.profile
        );
    }
    assert!(
        p.profile.ends_with('\n'),
        "策略文本要以换行收尾（一行一条规则）：{:?}",
        p.profile
    );
}

/// ② bind 那一档**不许加过滤**（§10.6 实测：加了会把 unix socket 挡掉 ⇒ `PermissionError:1`）。
///
/// 这条是「看着更严、实际更坏」的典型：过滤看起来是收紧了策略，实际是把空间内 IPC 一起打死。
#[test]
fn bind_rule_is_never_filtered() {
    let p = macos::policy(&mac_spec()).expect("好配方必须能出策略");
    assert!(
        !p.profile.contains("(allow network-bind ("),
        "bind 不许带过滤（会把 unix socket 挡掉）：\n{}",
        p.profile
    );
    assert!(
        p.allowlist.iter().any(|a| a.contains("不过滤")),
        "allowlist 要写明这条不过滤及其理由：{:?}",
        p.allowlist
    );
}

/// ③ 可写面**只到声明的路径**：全局 `file-write*` / 缺掉声明的可写区，都必须红。
#[test]
fn write_face_is_limited_to_declared_paths() {
    let s = mac_spec();
    let p = macos::policy(&s).expect("好配方必须能出策略");
    assert!(
        !p.profile.contains("(allow file-write*)\n"),
        "不许全局可写（那是授权级封闭当场失效的形态）：\n{}",
        p.profile
    );
    let mut got: Vec<String> = p
        .profile
        .lines()
        .filter(|l| l.starts_with("(allow file-write* (subpath "))
        .map(|l| l.to_string())
        .collect();
    got.sort();
    let mut want = vec![
        "(allow file-write* (subpath \"/private/tmp/wall-demo/work\"))".to_string(),
        "(allow file-write* (subpath \"/private/tmp/wall-demo/kv\"))".to_string(),
    ];
    want.sort();
    assert_eq!(
        got, want,
        "可写面必须恰好 = 声明的路径（work_dir ∪ 可写绑定宿主侧）"
    );
}

/// ③b 同一个路径声明两次 ⇒ 策略里**只有一条**（去重；否则策略文本不可逐字复现）。
#[test]
fn duplicate_writable_paths_are_deduped() {
    let mut s = mac_spec();
    // 可写绑定的宿主侧与 work_dir 同一路径
    s.extra_rw_binds = vec!["/private/tmp/wall-demo/work:/work".to_string()];
    let p = macos::policy(&s).expect("好配方必须能出策略");
    let n = p
        .profile
        .lines()
        .filter(|l| l.starts_with("(allow file-write* (subpath "))
        .count();
    assert_eq!(n, 1, "重复路径必须只出一条规则：\n{}", p.profile);
}

/// ④ argv 是**封闭之内**要跑的那条命令行：有 env ⇒ 与 Linux 同一份包装；无 env ⇒ 直 exec。
#[test]
fn argv_is_the_command_inside_the_confinement() {
    let p = plan_for("macos", &mac_spec()).expect("好配方必须能出计划");
    assert_eq!(p.platform, "macos");
    assert_eq!(
        p.argv[0], "/bin/sh",
        "有 env ⇒ 包装 exec（与 Linux 共用一份）"
    );
    assert_eq!(p.argv[1], "-c");
    assert!(
        p.argv[2].starts_with("export PROBE_DIR="),
        "包装脚本要带按卵变量：{}",
        p.argv[2]
    );
    assert_eq!(p.argv[4], "/usr/bin/python3", "包装后第一个参数 = 引擎");

    let mut s = mac_spec();
    s.env.clear();
    let p = plan_for("macos", &s).expect("好配方必须能出计划");
    assert_eq!(p.argv[0], "/usr/bin/python3", "无 env ⇒ 直 exec 引擎");
    assert_eq!(p.argv[1], "/repo/scripts/sandbox-probes/miniprobe.py");
    assert!(
        !p.argv[0].starts_with('-'),
        "argv[0] 必须是可执行名（不得是选项）：{:?}",
        p.argv[0]
    );
    assert!(
        !p.argv.iter().any(|a| a == "sandbox-exec"),
        "一档是**直调** Seatbelt ⇒ 不许调 sandbox-exec CLI：{:?}",
        p.argv
    );
}

/// ⑤ 放行清单如实：全局只读要列出来；**只读声明面不许伪装成额外授权**（macOS 上它们已被全局只读覆盖）；
/// 设备面在 macOS 上不产生策略项（IOKit 通路未实测 ⇒ 只记不宣称）。
#[test]
fn allowlist_is_honest_about_the_wide_read_grant() {
    let p = plan_for("macos", &mac_spec()).expect("好配方必须能出计划");
    assert!(
        p.allowlist.iter().any(|a| a.starts_with("file-read*:全局")),
        "全局只读必须如实列进放行清单：{:?}",
        p.allowlist
    );
    assert!(
        !p.allowlist.iter().any(|a| a.starts_with("ro-mount:")),
        "macOS 上声明的只读面不产生额外授权 ⇒ 不许列成 ro-mount（那是 Linux 侧的形态）：{:?}",
        p.allowlist
    );
    assert!(
        p.allowlist.iter().any(|a| a == "env:PROBE_DIR"),
        "按卵环境变量要列出来：{:?}",
        p.allowlist
    );
    assert!(
        p.note.contains("没有视图级封闭") && p.note.contains("iokit-open"),
        "note 要写清 macOS 的平台边界（无视图级封闭 + 设备面未实测）：{}",
        p.note
    );
    assert!(
        p.note.contains("sandbox_init"),
        "note 要说清封闭由谁施加：{}",
        p.note
    );
}

/// ⑥ 一行 JSON 的冻结形状（与 Linux 侧同一形状：schema_version / platform / argv / allowlist / note）。
#[test]
fn plan_json_keeps_the_frozen_shape() {
    let p = plan_for("macos", &mac_spec()).expect("好配方必须能出计划");
    let text = p.to_json();
    assert_eq!(text.lines().count(), 1, "必须是一行：{text}");
    let v = json::parse(&text).expect("自产 JSON 必须能自解");
    let obj = v.as_obj().expect("顶层是对象");
    let mut keys: Vec<String> = obj.keys().cloned().collect();
    keys.sort();
    assert_eq!(
        keys,
        vec![
            "allowlist".to_string(),
            "argv".to_string(),
            "note".to_string(),
            "platform".to_string(),
            "schema_version".to_string()
        ],
        "载荷的键集合是冻结的（不许为 macOS 另加字段或改名）"
    );
}

/// ⑦ **同契约不同落地**：macOS 与 Linux 接受/拒绝同一批配方。
///
/// 判据分两半：① 坏的配方两侧都要拒（macOS 不许放宽）；② 好的配方两侧都要能出计划（macOS 不许加戏）。
#[test]
fn macos_accepts_and_refuses_the_same_specs_as_linux() {
    let good = mac_spec();
    assert!(plan_for("macos", &good).is_ok() && plan_for("linux", &good).is_ok());

    // 权重声明了却没有对应绑定（Linux 侧「引擎读不到权重」那类静默故障）
    let mut bad = mac_spec();
    bad.extra_ro_binds.clear();
    assert!(plan_for("macos", &bad).is_err(), "坏配方 macOS 侧同样要拒");
    assert!(plan_for("linux", &bad).is_err(), "坏配方 Linux 侧同样要拒");

    // 陌生 schema_version
    let mut bad = mac_spec();
    bad.schema_version = 7;
    assert!(
        plan_for("macos", &bad).is_err(),
        "陌生版本 macOS 侧同样要拒"
    );

    // 认不得的平台（不是「没落地」，而是根本不认得）
    let err = plan_for("plan9", &good).expect_err("认不得的平台必须拒");
    assert!(err.msg().contains("裸 exec"), "{}", err.msg());
}

/// ⑧ 封闭由谁施加：Linux = 外部（argv 里的 bwrap）；macOS = 本进程直调（策略与 plan 同源）。
#[test]
fn confinement_kind_matches_the_platform() {
    let s = mac_spec();
    assert_eq!(
        confinement_for("linux", &s).expect("linux 应可判"),
        Confinement::External
    );
    let c = confinement_for("macos", &s).expect("macos 应可判");
    match c {
        Confinement::InProcess { profile } => {
            assert!(
                profile.contains("(deny default)"),
                "策略要含默认拒绝：{profile}"
            );
            // **同源**：run 施加的策略与 plan 交代的清单必须来自同一份产物
            let pol = macos::policy(&s).expect("策略");
            assert_eq!(profile, pol.profile);
            assert_eq!(
                plan_for("macos", &s).expect("计划").allowlist,
                pol.allowlist,
                "plan 的放行清单必须与 run 施加的策略同源"
            );
        }
        Confinement::External => panic!("macOS 一档必须是本进程直调（没有外部沙箱程序）"),
    }
    let err = confinement_for("freebsd", &s).expect_err("认不得的平台必须拒");
    assert!(err.msg().contains("裸 exec"), "{}", err.msg());
}

/// ⑨ 路径里的引号不许能把策略**改写**（策略文本是我们拼的 ⇒ 注入面就在路径里）。
#[test]
fn path_quotes_cannot_rewrite_the_policy() {
    let mut s = mac_spec();
    s.work_dir = "/private/tmp/a\")(allow file-write*)\"".to_string();
    let p = macos::policy(&s).expect("出策略（转义后仍是一份合法文本）");
    assert!(
        p.profile.contains("\\\""),
        "路径里的引号必须被转义：\n{}",
        p.profile
    );
    assert!(
        !p.profile.contains("(allow file-write*)\n"),
        "路径里的引号不许凭空多出一条规则：\n{}",
        p.profile
    );
}
