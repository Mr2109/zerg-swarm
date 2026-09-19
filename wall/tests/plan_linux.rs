//! 茧壁一档（Linux/bwrap）的**能失败**用例（批 2'.2）。
//!
//! 三条口径各有一组用例：
//!   ① **逐字对齐 Go 侧配方**（`hatch.BuildBwrapArgv`）—— 顺序与取值都不许「顺手改进」；
//!   ② **fail-closed**：缺字段 / 陌生版本 / 认不得的平台 / 一档没落地 / 挂错权重 / 绑定形式错，
//!      一律**拒绝**（绝不回落成裸 exec）；
//!   ③ **不放宽策略**：本端产出的 argv 里不许出现「把宿主整根挂进来」或「共享网络」这类放宽项。

use std::collections::BTreeMap;

use zerg_wall::json;
use zerg_wall::platform::linux::BASELINE_NOTE;
use zerg_wall::shim::exec_wrapper_script;
use zerg_wall::spec::Spec;
use zerg_wall::{plan_for, Error};

/// 一份「最小可用」的配方：一枚权重 + 一个可写工作目录 + 一个引擎库目录。
fn base_spec() -> Spec {
    let mut env = BTreeMap::new();
    env.insert("LD_LIBRARY_PATH".to_string(), "/libs/lib".to_string());
    env.insert("HIP_VISIBLE_DEVICES".to_string(), "0".to_string());
    Spec {
        egg_id: "demo-egg".to_string(),
        schema_version: 1,
        engine_path_in_space: "/engine/bin/llama-server".to_string(),
        engine_args: vec!["--port".to_string(), "9100".to_string()],
        engine_roots: vec!["/home/g01/llama.cpp-src/build-hip-flash/bin".to_string()],
        weight_path: "/data/models/demo".to_string(),
        weight_files: vec!["/data/models/demo/model.gguf".to_string()],
        env,
        devices: vec!["/dev/kfd".to_string(), "/dev/dri/renderD128".to_string()],
        extra_ro_binds: vec![
            "/data/models/demo/model.gguf:/models/model.gguf".to_string(),
            "/home/g01/llama-k2/build/lib:/libs/lib".to_string(),
        ],
        extra_rw_binds: vec!["/home/g01/.zerg/work/demo-egg:/work".to_string()],
        work_dir: "/work".to_string(),
        memlock_bytes: 64 * 1024 * 1024,
    }
}

/// ① 黄金 argv：逐字对齐 Go 侧 `hatch.BuildBwrapArgv` 的顺序与取值。
///
/// **两半判据**（与 `scripts/evals/compare-wall-argv.py` 同口径）：`argv[0]` = 可执行名 `bwrap`
/// （冻结接口：`argv` 是**执行面**），`argv[1..]` 才与 Go 侧 `BuildBwrapArgv` 逐条相同。
#[test]
fn golden_argv_matches_go_recipe() {
    let s = base_spec();
    let p = plan_for("linux", &s).expect("好配方必须能出计划");
    let expected: Vec<String> = [
        "bwrap",
        "--ro-bind",
        "/usr",
        "/usr",
        "--symlink",
        "usr/bin",
        "/bin",
        "--symlink",
        "usr/sbin",
        "/sbin",
        "--symlink",
        "usr/lib",
        "/lib",
        "--symlink",
        "usr/lib64",
        "/lib64",
        "--proc",
        "/proc",
        "--dev",
        "/dev",
        "--tmpfs",
        "/tmp",
        "--ro-bind",
        "/sys",
        "/sys",
        "--dir",
        "/models",
        // 引擎目录 → /engine
        "--ro-bind",
        "/home/g01/llama.cpp-src/build-hip-flash/bin",
        "/engine",
        // 额外只读（顺序 = manifest 顺序，不改序）
        "--ro-bind",
        "/data/models/demo/model.gguf",
        "/models/model.gguf",
        "--ro-bind",
        "/home/g01/llama-k2/build/lib",
        "/libs/lib",
        // 额外可写
        "--bind",
        "/home/g01/.zerg/work/demo-egg",
        "/work",
        // 设备（必须显式）
        "--dev-bind",
        "/dev/kfd",
        "/dev/kfd",
        "--dev-bind",
        "/dev/dri/renderD128",
        "/dev/dri/renderD128",
        // 隔离 + 工作目录
        "--unshare-pid",
        "--die-with-parent",
        "--chdir",
        "/work",
        "--",
        // 入口：有 env ⇒ 包装 exec（键名排序）
        "/bin/sh",
        "-c",
        "export HIP_VISIBLE_DEVICES='0' LD_LIBRARY_PATH='/libs/lib'; exec \"$@\"",
        "zerg-egg",
        "/engine/bin/llama-server",
        // 引擎参数
        "--port",
        "9100",
    ]
    .iter()
    .map(|x| (*x).to_string())
    .collect();
    assert_eq!(p.argv, expected, "argv 与 Go 侧配方不逐字一致 ⇒ 迁移桥断了");
    assert_eq!(p.platform, "linux");
    assert_eq!(p.schema_version, 1);
}

/// ①b `argv[0]` 必须是**可执行名**（`run` 就是 `Command::new(argv[0])`；退出码表里的
/// 「可执行不在 PATH ⇒ 2」也只有这时才说得通）。
///
/// 这条是从缺陷反推出来的（2026-09-16 批 2'.3）：早先的实现把 `argv` 只当「bwrap 参数序列」，
/// 于是 `argv[0] == "--ro-bind"` —— `run` 在任何 Linux 机器上都只能以「起不来」收场，
/// 而 `plan` 的载荷看起来完全正常（**一个「看着正常、跑不起来」的形态**）。
/// 所以这里断言两件事：①可执行名在首位；②其后**恰好**是 Go 侧那套参数（顺序不变）。
#[test]
fn argv0_is_the_executable_not_a_flag() {
    let p = plan_for("linux", &base_spec()).expect("好配方必须能出计划");
    assert_eq!(p.argv[0], "bwrap", "argv[0] 必须是可执行名：{:?}", p.argv);
    assert!(
        !p.argv[0].starts_with('-'),
        "argv[0] 是选项开头 ⇒ `run` 永远起不来（缺陷形态）：{:?}",
        p.argv[0]
    );
    assert_eq!(
        p.argv[1], "--ro-bind",
        "可执行名之后必须直接接 Go 侧参数序列的第一项"
    );
}

/// ② fail-closed 一组负例（每一条都必须红）。
#[test]
fn refuses_bad_specs() {
    // 缺 egg_id
    let mut s = base_spec();
    s.egg_id = "   ".to_string();
    assert!(plan_for("linux", &s).is_err(), "空 egg_id 必须拒");

    // 陌生 schema_version
    let mut s = base_spec();
    s.schema_version = 99;
    assert!(plan_for("linux", &s).is_err(), "陌生版本必须拒");

    // 引擎路径不是空间内绝对路径
    let mut s = base_spec();
    s.engine_path_in_space = "engine/bin/llama-server".to_string();
    assert!(plan_for("linux", &s).is_err(), "相对引擎路径必须拒");

    // 权重一处都没声明
    let mut s = base_spec();
    s.weight_files.clear();
    s.weight_path = String::new();
    s.extra_ro_binds
        .retain(|b| !b.contains("/models/model.gguf"));
    assert!(plan_for("linux", &s).is_err(), "无权重声明必须拒");

    // 工作目录写成宿主侧形态（相对）
    let mut s = base_spec();
    s.work_dir = "work".to_string();
    assert!(plan_for("linux", &s).is_err(), "相对工作目录必须拒");

    // 绑定形式不对（没有 `:`）
    let mut s = base_spec();
    s.extra_ro_binds.push("/data/models/other.gguf".to_string());
    assert!(plan_for("linux", &s).is_err(), "绑定缺 space 半边必须拒");

    // 权重声明了却没有任何对应绑定
    let mut s = base_spec();
    s.weight_files
        .push("/data/models/demo/mmproj.gguf".to_string());
    assert!(plan_for("linux", &s).is_err(), "声明未挂必须拒");

    // 声明与绑定挂的不是同一个文件（同一落点、不同宿主文件）
    let mut s = base_spec();
    s.extra_ro_binds[0] = "/data/models/other/model.gguf:/models/model.gguf".to_string();
    assert!(plan_for("linux", &s).is_err(), "挂错文件必须拒");

    // 落点撞车（两个权重基名相同）
    let mut s = base_spec();
    s.weight_files.push("/data/other/model.gguf".to_string());
    s.extra_ro_binds
        .push("/data/other/model.gguf:/models/model.gguf".to_string());
    assert!(plan_for("linux", &s).is_err(), "落点撞车必须拒");

    // 只读绑定落在 /models 本身（整目录挂载）
    let mut s = base_spec();
    s.extra_ro_binds.push("/data/models:/models".to_string());
    assert!(plan_for("linux", &s).is_err(), "整目录挂 /models 必须拒");

    // 可写绑定落在 /models 之下（权重会被改写）
    let mut s = base_spec();
    s.extra_rw_binds
        .push("/data/models:/models/demo".to_string());
    assert!(
        plan_for("linux", &s).is_err(),
        "可写落点在 /models 之下必须拒"
    );

    // 非法环境变量名
    let mut s = base_spec();
    s.env.insert("BAD-NAME".to_string(), "1".to_string());
    assert!(plan_for("linux", &s).is_err(), "非法 env 名必须拒");

    // 认不得的平台：必须拒（绝不回落成裸 exec）
    let s = base_spec();
    let err = plan_for("windows", &s).expect_err("windows 必须拒");
    assert!(
        err.msg().contains("裸 exec"),
        "拒绝理由要写明不许降级：{}",
        err.msg()
    );
    // macOS 一档（批 2'.4 起已落地）：**不再**是「没落地」，但仍是**另一套命令**——
    // 判据是「同契约不同落地」，不是「同一条 argv」（详见 `tests/plan_macos.rs`）
    let p = plan_for("macos", &s).expect("macOS 一档已落地，好配方必须能出计划");
    assert_eq!(p.platform, "macos");
    assert_eq!(
        p.argv[0], "/bin/sh",
        "有 env 时 macOS 侧同样走包装 exec（与 Linux 共用一份构造）"
    );
}

/// ③ 不放宽策略：argv 里不许有「宿主整根挂进来」「共享网络」这类放宽项。
#[test]
fn never_broadens_the_enclosure() {
    let s = base_spec();
    let p = plan_for("linux", &s).expect("好配方必须能出计划");
    let joined = p.argv.join(" ");
    assert!(!joined.contains("--share-net"), "不许共享宿主网络");
    assert!(
        !joined.contains("--ro-bind / /"),
        "不许把宿主整根只读挂进空间（视图级封闭会当场失真）"
    );
    // 可写绑定只允许落在工作目录那种专用落点（本配方里就是 /work）
    let rw_targets: Vec<&String> = p
        .argv
        .windows(3)
        .filter(|w| w[0] == "--bind")
        .map(|w| &w[2])
        .collect();
    assert_eq!(
        rw_targets,
        vec![&"/work".to_string()],
        "可写落点清单与配方不符"
    );
}

/// 放行清单：只列超出基线的授权；`memlock_bytes` **不在 argv 内** ⇒ 不进 allowlist，只在 note 里如实说明。
#[test]
fn allowlist_lists_grants_beyond_baseline() {
    let s = base_spec();
    let p = plan_for("linux", &s).expect("好配方必须能出计划");
    for want in [
        "device:/dev/kfd",
        "device:/dev/dri/renderD128",
        "ro-mount:/libs/lib",
        "rw-mount:/work",
        "chdir:/work",
        "env:LD_LIBRARY_PATH",
        "env:HIP_VISIBLE_DEVICES",
    ] {
        assert!(
            p.allowlist.iter().any(|a| a == want),
            "allowlist 缺 {want}：{:?}",
            p.allowlist
        );
    }
    // 大小写不敏感（变异体当时写成大写 MEMLOCK 就滑过去了 —— 探针本身也是被探的对象）
    assert!(
        !p.allowlist
            .iter()
            .any(|a| a.to_lowercase().contains("memlock") || a.starts_with("limit:")),
        "memlock 不在本 argv 内 ⇒ 不许列进 allowlist（那会看着像保障）：{:?}",
        p.allowlist
    );
    assert!(
        p.note.contains("不在本 argv 内"),
        "note 必须说明 memlock 的落点：{}",
        p.note
    );
    assert!(
        p.note.contains(BASELINE_NOTE),
        "note 必须写明基线：{}",
        p.note
    );
    assert!(
        p.note.contains("外部实读"),
        "note 必须写明等级由谁给：{}",
        p.note
    );

    // 反向：memlock 为 0 时不许出现那句（否则就是一句恒真的空话）
    let mut s0 = base_spec();
    s0.memlock_bytes = 0;
    let p0 = plan_for("linux", &s0).expect("好配方必须能出计划");
    assert!(
        !p0.note.contains("memlock_bytes="),
        "没声明 memlock 就不许提它：{}",
        p0.note
    );
}

/// 配方走 JSON 那条路（与结构体构造**必须同源**：读写两端一份实现）。
#[test]
fn spec_from_json_equals_struct_path() {
    let text = r#"{
      "egg_id": "demo-egg",
      "schema_version": 1,
      "engine_path_in_space": "/engine/bin/llama-server",
      "engine_args": ["--port", "9100"],
      "engine_roots": ["/home/g01/llama.cpp-src/build-hip-flash/bin"],
      "weight_path": "/data/models/demo",
      "weight_files": ["/data/models/demo/model.gguf"],
      "env": {"LD_LIBRARY_PATH": "/libs/lib", "HIP_VISIBLE_DEVICES": "0"},
      "devices": ["/dev/kfd", "/dev/dri/renderD128"],
      "extra_ro_binds": ["/data/models/demo/model.gguf:/models/model.gguf", "/home/g01/llama-k2/build/lib:/libs/lib"],
      "extra_rw_binds": ["/home/g01/.zerg/work/demo-egg:/work"],
      "work_dir": "/work",
      "memlock_bytes": 67108864,
      "profile": {"schema_version": 2, "note": "茧壁不消费的决策面字段：忽略即可"}
    }"#;
    let v = json::parse(text).expect("配方 JSON 应能解析");
    let from_json = Spec::from_json(&v).expect("配方应能构造");
    assert_eq!(from_json, base_spec(), "JSON 路径与结构体路径必须同源");
    let a = plan_for("linux", &from_json).expect("出计划");
    let b = plan_for("linux", &base_spec()).expect("出计划");
    assert_eq!(a.argv, b.argv, "两条路径的 argv 必须逐字相同");

    // 反向：缺必需键 ⇒ 拒
    let bad = json::parse(r#"{"egg_id":"x"}"#).expect("能解析");
    assert!(
        Spec::from_json(&bad).is_err(),
        "缺 schema_version/engine_path_in_space 必须拒"
    );
}

/// 计划 JSON 的形状：五个固定键、argv/allowlist 是数组、能被本端自己的解析器读回。
#[test]
fn plan_json_roundtrip() {
    let p = plan_for("linux", &base_spec()).expect("出计划");
    let text = p.to_json();
    assert!(!text.contains('\n'), "必须是一行 JSON");
    let v = json::parse(&text).expect("自己产的 JSON 必须能被自己读回");
    assert_eq!(
        v.get("platform").and_then(|x| x.as_str().ok()),
        Some("linux")
    );
    assert_eq!(
        v.get("argv").and_then(|x| x.as_arr().ok()).map(|a| a.len()),
        Some(p.argv.len())
    );
    assert!(v.get("allowlist").is_some(), "allowlist 必须在载荷里");
    assert!(v.get("note").is_some(), "note 必须在载荷里");
}

/// 包装脚本：无 env ⇒ 直 exec；有 env ⇒ 包装且 `$0` 固定（参数错位没人看得出来，故钉死）。
#[test]
fn wrapper_only_when_env_present() {
    let mut s = base_spec();
    s.env.clear();
    let p = plan_for("linux", &s).expect("出计划");
    assert!(exec_wrapper_script(&s.env).is_none());
    assert!(
        !p.argv.iter().any(|a| a == "/bin/sh"),
        "无 env 时不许加包装 shell（多一层 sh 会让 execve 失败的归因变含糊）"
    );
    let tail: Vec<String> = p.argv[p.argv.len() - 4..].to_vec();
    assert_eq!(
        tail,
        ["--", "/engine/bin/llama-server", "--port", "9100"]
            .iter()
            .map(|x| (*x).to_string())
            .collect::<Vec<String>>(),
        "无 env 时必须直 exec，且引擎路径与参数都不许错位"
    );
}

/// 缺省设备：配方不声明设备时用 /dev/kfd + renderD128（**默认 --dev /dev 看不到 GPU**，真机实测）。
#[test]
fn default_devices_used_when_unset() {
    let mut s = base_spec();
    s.devices.clear();
    let p = plan_for("linux", &s).expect("出计划");
    let joined = p.argv.join(" ");
    assert!(
        joined.contains("--dev-bind /dev/kfd /dev/kfd"),
        "缺省 KFD 必须显式绑"
    );
    assert!(joined.contains("--dev-bind /dev/dri/renderD128 /dev/dri/renderD128"));
}

/// 错误类型必须能当普通错误用（消费侧要打印理由，不是 debug 串）。
#[test]
fn error_displays_message() {
    let e = Error::new("拒绝示例");
    assert_eq!(format!("{e}"), "拒绝示例");
}
