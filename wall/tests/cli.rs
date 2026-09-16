//! CLI 级用例（批 2'.2）：真实起二进制、看真实退出码与 stdout —— 口径是「退出码就是断言」。
//!
//! 为什么要有这一组：库级用例证明不了「命令行那条路」通不通（参数解析、退出码、一行 JSON 的约定）。
//! 这组用例跑的是 `CARGO_BIN_EXE_zerg-wall`，即本批真实构建出来的那个二进制。

use std::path::PathBuf;
use std::process::Command;

fn bin() -> PathBuf {
    PathBuf::from(env!("CARGO_BIN_EXE_zerg-wall"))
}

fn example_spec() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("examples/demo-spec.json")
}

fn tmp_file(name: &str) -> PathBuf {
    let mut p = std::env::temp_dir();
    p.push(format!("zerg-wall-test-{}-{}", std::process::id(), name));
    p
}

fn run(args: &[&str]) -> (i32, String, String) {
    let out = Command::new(bin())
        .args(args)
        .output()
        .expect("二进制应能起来");
    (
        out.status.code().unwrap_or(-1),
        String::from_utf8_lossy(&out.stdout).to_string(),
        String::from_utf8_lossy(&out.stderr).to_string(),
    )
}

/// 正例：离线点名 linux 出计划 ⇒ rc=0、stdout **一行** JSON、含 argv 与 allowlist。
#[test]
fn cli_plan_linux_is_one_line_json() {
    let spec = example_spec();
    let (rc, out, err) = run(&[
        "plan",
        "--spec",
        spec.to_str().unwrap(),
        "--platform",
        "linux",
    ]);
    assert_eq!(rc, 0, "plan 应成功；stderr={err}");
    assert_eq!(out.lines().count(), 1, "stdout 必须是一行 JSON：{out}");
    assert!(
        out.contains("\"platform\":\"linux\""),
        "载荷要写明平台：{out}"
    );
    assert!(out.contains("\"allowlist\":["), "载荷要有放行清单：{out}");
    assert!(
        out.contains("\"--unshare-pid\""),
        "argv 要真的带上隔离项：{out}"
    );
}

/// 默认平台 = **本机平台**（不再有「一档没落地」这一态：macOS 一档批 2'.4 起已落地）。
///
/// 判据是「本机平台出得出计划」+「载荷里写的平台与运行平台一致」——两件事都要**看证据**：
/// 出计划的平台若与运行平台不符（比如 macOS 上出了 linux 计划），`run` 那一步就是说不清的事。
#[test]
fn cli_plan_defaults_to_host_platform() {
    let spec = example_spec();
    let (rc, out, err) = run(&["plan", "--spec", spec.to_str().unwrap()]);
    assert_eq!(rc, 0, "默认平台应能出计划；stderr={err}");
    let host = std::env::consts::OS;
    assert!(
        out.contains(&format!("\"platform\":\"{host}\"")),
        "载荷里的平台必须是本机平台 {host}：{out}"
    );
    assert_eq!(out.lines().count(), 1, "stdout 必须是一行 JSON：{out}");
}

/// 负例：**认不得的平台**照旧必须拒（不许回落成裸 exec）——这条是「不许降级」那一半。
#[test]
fn cli_plan_refuses_unknown_platform() {
    let spec = example_spec();
    let (rc, out, err) = run(&[
        "plan",
        "--spec",
        spec.to_str().unwrap(),
        "--platform",
        "windows",
    ]);
    assert_eq!(rc, 2, "认不得的平台必须硬失败；stdout={out}");
    assert!(err.contains("裸 exec"), "拒绝理由要写明不许降级：{err}");
}

/// 负例：硬失败一律 rc=2（缺参数 / 认不得的参数 / 坏配方 / run 点名平台）。
#[test]
fn cli_hard_failures_are_rc_2() {
    let spec = example_spec();
    let spec_path = spec.to_str().unwrap().to_string();

    let (rc, _, err) = run(&["plan"]);
    assert_eq!(rc, 2, "缺 --spec 必须 rc=2");
    assert!(err.contains("--spec"), "理由要点名缺什么：{err}");

    let (rc, _, err) = run(&["plan", "--spec", &spec_path, "--platfrom", "linux"]);
    assert_eq!(rc, 2, "认不得的参数必须 rc=2（不许静默忽略）");
    assert!(err.contains("认不得"), "理由要写明认不得：{err}");

    let (rc, _, err) = run(&[
        "plan",
        "--spec",
        &spec_path,
        "--platform",
        "linux",
        "--platform",
        "linux",
    ]);
    assert_eq!(rc, 2, "同一个参数给两次必须 rc=2");
    assert!(err.contains("两次"), "理由要写明重复：{err}");

    // run 不接受 --platform（要跑的就是本机这一格）
    let (rc, _, err) = run(&["run", "--spec", &spec_path, "--platform", "linux"]);
    assert_eq!(rc, 2, "run 点名平台必须 rc=2");
    assert!(err.contains("不接受"), "理由要写明为什么不接受：{err}");

    // 坏配方：权重声明了却没有对应绑定（真机上就是「引擎读不到权重」那类静默故障）
    let bad = tmp_file("bad-spec.json");
    std::fs::write(
        &bad,
        r#"{"egg_id":"bad","schema_version":1,"engine_path_in_space":"/engine/bin/llama-server",
            "weight_files":["/opt/models/bad/model.gguf"]}"#,
    )
    .expect("写临时配方");
    let (rc, _, err) = run(&[
        "plan",
        "--spec",
        bad.to_str().unwrap(),
        "--platform",
        "linux",
    ]);
    assert_eq!(rc, 2, "坏配方必须 rc=2");
    assert!(
        err.contains("没有对应的只读绑定"),
        "理由要指到具体缺陷：{err}"
    );
    let _ = std::fs::remove_file(&bad);

    // 找不到的配方文件：也是硬失败（不许当成「没配方就直跑」）
    let (rc, _, err) = run(&[
        "plan",
        "--spec",
        "/nonexistent/zerg-wall-spec.json",
        "--platform",
        "linux",
    ]);
    assert_eq!(rc, 2, "读不到配方必须 rc=2");
    assert!(err.contains("读不到配方"), "理由要写明读不到：{err}");
}

/// `--version` 走的是「不碰配方」的那条路：rc=0 且打印名字与版本。
#[test]
fn cli_version_prints_identity() {
    let (rc, out, _) = run(&["--version"]);
    assert_eq!(rc, 0);
    assert!(out.starts_with("zerg-wall "), "版本行要能被机器读：{out}");
}
