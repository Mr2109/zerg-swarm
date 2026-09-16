//! macOS 一档（Seatbelt 直调）的**活体四态**用例（批 2'.4）。
//!
//! 判据（任务单 §3 2'.4）：**本地靶子四态与 `scripts/sandbox-probes/` 历史回执一致**
//! （`evidence-two-states-20260916.txt` 的 pF 那一行：unix socket ok · loopback ok · 写空间内 ok ·
//! **出网 BLOCKED**），外加写**空间外**必须被拦（授权级封闭的本体）。
//!
//! **两条防假绿**（这是本用例最要紧的部分）：
//!   ① **每条断言都有基线对照**：先跑一次**无封闭**的同一靶子 —— 出网基线不可达、或空间外本来就写不进去
//!      ⇒ 断言**没有区分度** ⇒ 用例**直接红**并说明原因（不许静默跳过、不许写成「看起来过了」）；
//!   ② **每次 `run` 都要看到策略留痕**（stderr 里的策略原文）—— 没有它就说明封闭那一步没发生。
//!
//! 只在 macOS 上跑（Linux 侧同一条判据在 `verify-two-states.py` 的 bwrap 分支里，任务 2'.5）。

#![cfg(target_os = "macos")]

use std::path::{Path, PathBuf};
use std::process::Command;

use zerg_wall::json;

/// 仓根（`wall/` 的上一级）。
fn repo_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("wall 必须在仓根之下")
        .to_path_buf()
}

fn probe(name: &str) -> PathBuf {
    let p = repo_root().join("scripts/sandbox-probes").join(name);
    assert!(
        p.is_file(),
        "探针缺失 {} ⇒ 硬失败（不许静默跳过：判据靠它取证）",
        p.display()
    );
    p
}

fn python3() -> String {
    let p = "/usr/bin/python3";
    assert!(
        Path::new(p).is_file(),
        "{p} 不在 ⇒ 环境问题 ⇒ 硬失败（不静默跳过）"
    );
    p.to_string()
}

fn tmp_space(tag: &str) -> PathBuf {
    let p = PathBuf::from(format!(
        "/private/tmp/zerg-wall-seatbelt-{}-{}",
        std::process::id(),
        tag
    ));
    let _ = std::fs::remove_dir_all(&p);
    std::fs::create_dir_all(p.join("work")).expect("建靶子目录");
    std::fs::create_dir_all(p.join("weights")).expect("建权重目录");
    std::fs::write(p.join("weights/model.gguf"), b"not-a-real-weight").expect("造权重占位");
    p
}

/// 写一份 macOS 形态的配方（路径按**宿主路径**：macOS 没有视图级封闭 ⇒ 声明即宿主路径）。
///
/// `env_fragment` 直接给**变量块的 JSON 片段**（空串 = 不带按卵变量）—— 让每条用例明确自己测的是
/// 「带 env / 不带 env」哪条路，而不是事后按字符串去改配方（那种改法一旦格式变了就悄悄不生效）。
fn write_spec(space: &Path, probe_name: &str, env_fragment: &str) -> PathBuf {
    let work = space.join("work");
    let weight = space.join("weights/model.gguf");
    let spec = format!(
        r#"{{
  "egg_id": "wall-seatbelt-live",
  "schema_version": 1,
  "engine_path_in_space": "{py}",
  "engine_args": ["{probe}"],
  "weight_path": "{weights}",
  "weight_files": ["{weight}"],
  "env": {{{env}}},
  "extra_ro_binds": ["{weight}:/models/model.gguf"],
  "extra_rw_binds": [],
  "work_dir": "{work}"
}}"#,
        py = python3(),
        probe = probe(probe_name).display(),
        weights = space.join("weights").display(),
        weight = weight.display(),
        env = env_fragment,
        work = work.display(),
    );
    let path = space.join(format!("spec-{probe_name}.json"));
    std::fs::write(&path, spec).expect("写配方");
    path
}

/// 按卵变量块的 JSON 片段（`"KEY":"value"` 逗号分隔）。
fn env_fragment(pairs: &[(&str, &str)]) -> String {
    pairs
        .iter()
        .map(|(k, v)| format!("\"{k}\":\"{v}\""))
        .collect::<Vec<String>>()
        .join(",")
}

/// 靶子要用的变量：空间内可写区（`PROBE_DIR`）+ 用例自带的那些。
fn probe_env(space: &Path, extra: &[(&str, String)]) -> String {
    let mut pairs: Vec<(String, String)> = vec![(
        "PROBE_DIR".to_string(),
        space.join("work").display().to_string(),
    )];
    for (k, v) in extra {
        pairs.push((k.to_string(), v.clone()));
    }
    let refs: Vec<(&str, &str)> = pairs
        .iter()
        .map(|(k, v)| (k.as_str(), v.as_str()))
        .collect();
    env_fragment(&refs)
}

/// 跑一次靶子（**无封闭**基线），返回 (rc, stdout)。
fn run_baseline(probe_name: &str, space: &Path, extra: &[(&str, String)]) -> (i32, String) {
    let mut cmd = Command::new(python3());
    cmd.arg(probe(probe_name))
        .env("PROBE_DIR", space.join("work"));
    for (k, v) in extra {
        cmd.env(k, v);
    }
    let out = cmd.output().expect("基线跑探针");
    (
        out.status.code().unwrap_or(-1),
        String::from_utf8_lossy(&out.stdout).to_string(),
    )
}

/// 跑一次靶子（**经茧壁**：run ⇒ 本进程直调 Seatbelt ⇒ exec 探针），返回 (rc, stdout, stderr)。
fn run_through_wall(spec: &Path) -> (i32, String, String) {
    let bin = env!("CARGO_BIN_EXE_zerg-wall");
    let out = Command::new(bin)
        .args(["run", "--spec", spec.to_str().unwrap()])
        .output()
        .expect("起茧壁");
    (
        out.status.code().unwrap_or(-1),
        String::from_utf8_lossy(&out.stdout).to_string(),
        String::from_utf8_lossy(&out.stderr).to_string(),
    )
}

fn field(v: &json::Value, key: &str) -> String {
    v.as_obj()
        .expect("探针输出是对象")
        .get(key)
        .unwrap_or_else(|| panic!("探针输出缺字段 {key}"))
        .as_str()
        .expect("字段是字符串")
        .to_string()
}

/// 判别目标（出网那一格）：默认与历史回执一致，可用 `PROBE_EGRESS_HOST/PORT` 覆盖。
fn egress_target() -> (String, String) {
    (
        std::env::var("PROBE_EGRESS_HOST").unwrap_or_else(|_| "1.1.1.1".to_string()),
        std::env::var("PROBE_EGRESS_PORT").unwrap_or_else(|_| "443".to_string()),
    )
}

/// **四态**：空间内 unix socket / loopback / 写空间内 —— 封闭态必须全部照常可用（**不许**「一封到底」），
/// 出网必须被拦；基线对照必须有区分度。
#[test]
fn macos_four_states_match_the_measured_receipt() {
    let space = tmp_space("four");
    let (host, port) = egress_target();
    let extra: Vec<(&str, String)> = vec![
        ("PROBE_EGRESS_HOST", host.clone()),
        ("PROBE_EGRESS_PORT", port.clone()),
    ];

    // ① 基线（无封闭）：同一靶子、同一判别目标
    let (base_rc, base_out) = run_baseline("miniprobe.py", &space, &extra);
    assert_eq!(base_rc, 0, "基线探针应 rc=0；输出={base_out}");
    let base = json::parse(base_out.trim()).expect("基线输出是 JSON");
    let base_egress = field(&base, "egress");
    assert_eq!(
        field(&base, "unix_bind"),
        "ok",
        "基线都起不来 unix socket ⇒ 靶子坏了（不是封闭做对了）"
    );

    // ② 经茧壁（封闭态）
    let spec = write_spec(&space, "miniprobe.py", &probe_env(&space, &extra));
    let (rc, out, err) = run_through_wall(&spec);
    assert_eq!(rc, 0, "茧壁 run 应 rc=0；stdout={out}\nstderr={err}");
    assert!(
        err.contains("(deny default)") && err.contains("sandbox_init"),
        "run 必须留下策略留痕（否则说明封闭那一步没发生）：\n{err}"
    );
    let got = json::parse(out.trim()).expect("封闭态输出是 JSON");

    // ③ 三态：封闭**不许**把空间内通信一起打死
    assert_eq!(
        field(&got, "unix_bind"),
        "ok",
        "空间内 unix socket 必须可用（bind 那一档不许加过滤）：{out}\nstderr={err}"
    );
    assert!(
        field(&got, "tcp_bind").starts_with("ok:"),
        "空间内 loopback bind 必须可用：{out}"
    );
    assert_eq!(field(&got, "write_work"), "ok", "空间内可写必须可用：{out}");

    // ④ 出网：**先证基线可达**（否则这一格没有区分度 —— 这正是 X3 上踩过的测量学坑）
    assert_eq!(
        base_egress,
        "OPEN",
        "判别目标 {} 基线不可达（{}）⇒ 出网断言没有区分度 ⇒ 硬失败：用 PROBE_EGRESS_HOST/PORT 指定一个本机可达目标",
        field(&base, "egress_target"),
        base_egress
    );
    let got_egress = field(&got, "egress");
    assert!(
        got_egress.starts_with("BLOCKED"),
        "出网必须被拦（基线可达 ⇒ 有区分度）：封闭态={got_egress}\n{out}"
    );

    let _ = std::fs::remove_dir_all(&space);
}

/// **写空间外必须被拦**（授权级封闭的本体）：基线能写、封闭态写不进去。
///
/// 这条把「可写面只到声明路径」钉到活体上：策略若退化成全局 `file-write*`，基线那格照样 OK，
/// 而封闭态这格会变成 `OPEN` ⇒ 用例红。
#[test]
fn macos_write_outside_the_declared_space_is_refused() {
    let space = tmp_space("writeout");
    let outside = space.join("outside.txt");
    let extra: Vec<(&str, String)> = vec![("PROBE_OUTSIDE", outside.display().to_string())];

    let (base_rc, base_out) = run_baseline("writeout-probe.py", &space, &extra);
    assert_eq!(base_rc, 0, "基线探针应 rc=0；输出={base_out}");
    let base = json::parse(base_out.trim()).expect("基线输出是 JSON");
    assert_eq!(
        field(&base, "write_inside"),
        "ok",
        "基线写空间内都不成 ⇒ 靶子坏了：{base_out}"
    );
    assert_eq!(
        field(&base, "write_outside"),
        "OPEN",
        "基线就写不进空间外（权限问题）⇒ 这一格没有区分度 ⇒ 硬失败"
    );
    // 基线**会把那个文件造出来**（它本来就能写）⇒ 先删掉，再让封闭态去试写：
    // 不删的话「文件不存在」那条断言就恒假（第一次跑就撞过：那是**用例自己的**缺陷，不是实现的）
    std::fs::remove_file(&outside).expect("删掉基线造出的空间外文件");
    assert!(!outside.exists(), "基线产物没删干净 ⇒ 后面的断言没有区分度");

    let spec = write_spec(&space, "writeout-probe.py", &probe_env(&space, &extra));
    let (rc, out, err) = run_through_wall(&spec);
    assert_eq!(rc, 0, "茧壁 run 应 rc=0；stdout={out}\nstderr={err}");
    let got = json::parse(out.trim()).expect("封闭态输出是 JSON");
    assert_eq!(field(&got, "write_inside"), "ok", "空间内仍要能写：{out}");
    assert!(
        field(&got, "write_outside").starts_with("BLOCKED"),
        "空间外必须写不进去（授权级封闭）：{out}\nstderr={err}"
    );
    assert!(
        !outside.exists(),
        "空间外的文件竟然真的被创建了：{}",
        outside.display()
    );

    let _ = std::fs::remove_dir_all(&space);
}

/// 负例：**策略文本不合法** ⇒ `sandbox_init` 返回非 0 ⇒ `apply` **必须报错**（不许吞掉）。
///
/// 这条钉的是「施加失败 = 失败」：若哪天有人把失败吞掉（`Ok(())`），`run` 就会在**没有封闭**的情况下
/// 继续跑引擎 —— 正是设计稿反复点名的「宣称封闭、实际直跑」形态。
///
/// 为什么可以在**本进程**里试（不改道裸跑）：实测（macOS 26）坏策略是 `rc=-1` **且不施加任何策略**
/// （脚本里那份自检就是先证明这一点才敢这么写的）—— 进程不会被关进一个「默认拒绝」里出不来。
#[test]
fn macos_bad_profile_is_refused() {
    let err = zerg_wall::platform::macos::apply("(this is not a seatbelt profile)")
        .expect_err("坏策略必须报错（不许当成施加成功）");
    assert!(
        err.msg().contains("sandbox_init 失败"),
        "错误里要写明失败点：{}",
        err.msg()
    );
    assert!(
        err.msg().contains("绝不") && err.msg().contains("裸跑"),
        "错误里要写明绝不回落：{}",
        err.msg()
    );
    // 失败**没有**顺手施加任何封闭：本进程仍能写空间内/外（否则就不是「拒绝并退出」）
    let p = PathBuf::from(format!(
        "/private/tmp/zerg-wall-seatbelt-{}-nofallback.txt",
        std::process::id()
    ));
    std::fs::write(&p, b"x").expect("坏策略不应关闭进程的写面");
    let _ = std::fs::remove_file(&p);
}

/// 负例：**引擎路径不存在** ⇒ rc=2 且理由写明「起不来」，**绝不**换别的执行方式。
///
/// ⚠ 这条负例**必须用不带 env 的配方**（直 exec）：带 env 时 argv[0] 是 `/bin/sh`，
/// 引擎起不来是**包装里**的 `exec "$@"` 失败 ⇒ 退出码是 shell 的 127，而不是茧壁的 2
/// —— 那是同一条链上两个不同的失败面（见 `wall/README.md` 的口径表），不是同一个判据。
#[test]
fn macos_missing_engine_is_a_hard_failure() {
    let space = tmp_space("missing");
    let spec = write_spec(&space, "miniprobe.py", "");
    let text = std::fs::read_to_string(&spec).expect("读配方");
    assert!(
        text.contains("\"env\": {},"),
        "这条用例要的是**不带 env** 的配方（直 exec 那条路）：{text}"
    );
    let broken = text.replace("/usr/bin/python3", "/nonexistent/wall-no-engine");
    std::fs::write(&spec, broken).expect("改写配方");

    let (rc, out, err) = run_through_wall(&spec);
    assert_eq!(
        rc, 2,
        "可执行不在 ⇒ 硬失败 rc=2；stdout={out}\nstderr={err}"
    );
    assert!(err.contains("起不来"), "理由要写明起不来：{err}");
    assert!(
        err.contains("绝不"),
        "理由要写明绝不改道（不换执行方式、不裸跑）：{err}"
    );

    let _ = std::fs::remove_dir_all(&space);
}
