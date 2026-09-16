//! Linux 一档：茧壁把配方翻译成 **bwrap 命令行**（不自己写 setns/pivot_root —— 二档才直调原语）。
//!
//! **逐字对齐 Go 侧 `hatch.BuildBwrapArgv`**（`agent/internal/hatch/hatch.go`）：本批的验收桥就是
//! 「同一份配方，两侧 argv **逐条一致**」（任务单 2'.3）。任何顺序/取值上的「顺手改进」都算漂移 ——
//! 真要改配方，先改设计稿与 Go 侧，两侧同批改（同一落点两套真相 = 最难查的一类静默故障）。

use crate::error::Error;
use crate::shim::{exec_wrapper_script, SH_PATH, WRAPPER_ARG0};
use crate::spec::{split_bind, Spec, SPACE_MODELS_DIR};
use crate::{Plan, PLAN_SCHEMA_VERSION};

/// bwrap 可执行名（不在 PATH ⇒ `run` 必须失败，绝不换别的沙箱或裸跑）。
pub const BWRAP: &str = "bwrap";

/// 空间内引擎目录（与 Go 侧 `backend.spaceEngineDir` / `hatch` 的 `/engine` 同值）。
pub const SPACE_ENGINE_DIR: &str = "/engine";

/// 基线说明（写进 `note`；**基线项不进 allowlist** —— allowlist 只列「超出基线」的授权）。
pub const BASELINE_NOTE: &str =
    "基线 = 只读 /usr + usrmerge 四个符号链接（/bin /sbin /lib /lib64）+ /proc + /dev + \
tmpfs /tmp + 空目录 /models（`--dir`，不是挂载点）";

/// 缺省设备：KFD（计算接口）+ 渲染节点（缓冲分配/映射）。
///
/// 注意：**默认 `--dev /dev` 看不到 GPU**（X3 实测），必须显式 `--dev-bind` 这两个。
pub fn default_devices() -> Vec<String> {
    vec!["/dev/kfd".to_string(), "/dev/dri/renderD128".to_string()]
}

/// 出计划：配方 ⇒ bwrap argv + 放行清单 + 如实说明。
pub fn plan(s: &Spec) -> Result<Plan, Error> {
    s.validate()?;

    // ① `argv[0]` = **可执行名**（`bwrap`）—— 冻结接口（`README.md`「计划的形状」与任务单 §3：
    //    「`argv` —— 执行面 … `argv[0]` 是 `bwrap`」；退出码表里的「可执行不在 PATH ⇒ 2」也只有
    //    在 `argv[0]` 是可执行名时才说得通，`run` 正是 `Command::new(argv[0])`）。
    //
    // ⚠ **与 Go 侧 `hatch.BuildBwrapArgv` 的唯一差异就在这里**：那个函数只给**参数**序列（`bwrap`
    //    由 `BuildSystemdRunArgv` 补在前面）。⇒ 迁移桥（`scripts/compare-wall-argv.py`，任务 2'.3）
    //    的判据写成两半：① `wall.argv[0] == "bwrap"`；② `wall.argv[1..]` 与 Go 的 argv **逐条相同**。
    //    两侧都不许在自己的那半边「顺手改」—— 改了就是同一个落点两套真相。
    let mut argv: Vec<String> = vec![BWRAP.to_string()];
    argv.extend(
        [
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
            // 宿主 /sys **只读**进空间：设备「可不可用」由 /sys 下的拓扑/属性描述（GPU 如此，
            // 别的加速器同理）；缺它时 X3 真机上引擎报 `no ROCm-capable device is detected`
            // 并**静默降级 CPU**（真机实测，2026-09-15）。只读 ⇒ 不放开写面。
            "--ro-bind",
            "/sys",
            "/sys",
            // /models 只造一个**空目录**：宿主权重由 extra_ro_binds 逐文件绑到 /models/<基名>。
            // 用 --dir 而不是 --tmpfs：既不构成挂载点（判据才有判别力），也明确它在逐文件绑定之前。
            "--dir",
            SPACE_MODELS_DIR,
        ]
        .iter()
        .map(|s| (*s).to_string()),
    );

    // ② 引擎自己的库/构建目录（宿主侧）→ /engine（只读）
    let mut allowlist: Vec<String> =
        vec!["ro-mount:/sys（设备拓扑：任何要认设备的引擎都读它）".to_string()];
    for root in &s.engine_roots {
        argv.push("--ro-bind".to_string());
        argv.push(root.clone());
        argv.push(SPACE_ENGINE_DIR.to_string());
    }

    // ③ 额外只读绑定（host:space）—— 权重逐文件、库目录、模板都走这条通道
    for b in &s.extra_ro_binds {
        let (host, space) = split_bind("只读", b)?;
        argv.push("--ro-bind".to_string());
        argv.push(host.to_string());
        argv.push(space.to_string());
        allowlist.push(format!("ro-mount:{space}"));
    }

    // ④ 额外**可写**绑定（bwrap `--bind`）：一次性工作目录与 KV 盘
    for b in &s.extra_rw_binds {
        let (host, space) = split_bind("可写", b)?;
        argv.push("--bind".to_string());
        argv.push(host.to_string());
        argv.push(space.to_string());
        allowlist.push(format!("rw-mount:{space}"));
    }

    // ⑤ GPU：必须显式（默认 --dev 看不到）
    let devices = if s.devices.is_empty() {
        default_devices()
    } else {
        s.devices.clone()
    };
    for d in &devices {
        argv.push("--dev-bind".to_string());
        argv.push(d.clone());
        argv.push(d.clone());
        allowlist.push(format!("device:{d}"));
    }

    // ⑥ 隔离：进程视图（空间内 pid 1 就是引擎）
    argv.push("--unshare-pid".to_string());
    argv.push("--die-with-parent".to_string());

    // ⑦ 工作目录（在包装脚本/引擎之前：chdir 之后 exec，两边 cwd 都是它）
    if !s.work_dir.is_empty() {
        argv.push("--chdir".to_string());
        argv.push(s.work_dir.clone());
        allowlist.push(format!("chdir:{}", s.work_dir));
    }

    // ⑧ 入口：有按卵环境变量 ⇒ 包装 exec（`/bin/sh -c 'export …; exec "$@"'`）；
    //    没有 ⇒ 直 exec（多一层 sh 会把 execve 失败的归因变含糊）
    argv.push("--".to_string());
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
    for k in s.env.keys() {
        allowlist.push(format!("env:{k}"));
    }

    // ⑨ 如实说明（该说清的都说清：哪些不在本 argv 内、等级由谁给）
    let mut note = format!(
        "{}；{BASELINE_NOTE}；allowlist 只列超出基线的授权；\
         observed 等级由**外部实读**给出（茧壁不判定等级，照 GKE「沙箱内自报不可信」）；\
         归属层（systemd-run --user 的 slice/单元/限额）不在本 argv 内",
        crate::platform::tier_note("linux")
    );
    if s.memlock_bytes > 0 {
        note.push_str(&format!(
            "；memlock_bytes={} 由归属层（systemd-run LimitMEMLOCK）下发，**不在本 argv 内**",
            s.memlock_bytes
        ));
    }
    if !s.weight_path.trim().is_empty() {
        note.push_str(&format!(
            "；weight_path={} 仅出证用、不参与挂载（逐文件绑定见 extra_ro_binds）",
            s.weight_path
        ));
    }

    Ok(Plan {
        schema_version: PLAN_SCHEMA_VERSION,
        platform: "linux".to_string(),
        argv,
        allowlist,
        note,
    })
}
