//! Linux 一档：茧壁把配方翻译成 **bwrap 命令行**（不自己写 setns/pivot_root —— 二档才直调原语）。
//!
//! **逐字对齐 Go 侧 `hatch.BuildBwrapArgv`**（`agent/internal/hatch/hatch.go`）：本批的验收桥就是
//! 「同一份配方，两侧 argv **逐条一致**」（任务单 2'.3）。任何顺序/取值上的「顺手改进」都算漂移 ——
//! 真要改配方，先改设计稿与 Go 侧，两侧同批改（同一落点两套真相 = 最难查的一类静默故障）。

use crate::error::Error;
use crate::spec::{split_bind, Spec, SPACE_MODELS_DIR};
use crate::{Plan, PLAN_SCHEMA_VERSION};

/// bwrap 可执行名（不在 PATH ⇒ `run` 必须失败，绝不换别的沙箱或裸跑）。
pub const BWRAP: &str = "bwrap";

/// 空间内引擎目录（与 Go 侧 `backend.spaceEngineDir` / `hatch` 的 `/engine` 同值）。
pub const SPACE_ENGINE_DIR: &str = "/engine";

/// 空间内 shell（`/bin` 是 usrmerge 符号链接；真机实测可用）。
const SH_PATH: &str = "/bin/sh";

/// 包装脚本的 `$0`（**不参与** `exec "$@"`）。
///
/// 为什么用一个固定普通字当 `$0`（而不是顺手写 `--`）：那个 `--` 到底是 `$0` 还是「选项终止符」
/// 随 shell 实现而异 —— 若被当选项终止符，`$0` 会变成引擎路径、`$@` 少一个参数，
/// 于是「引擎名丢了却照样能起」（参数错位没人看得出来）。
const WRAPPER_ARG0: &str = "zerg-egg";

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

    // ① 固定前缀：全新根（与 Go 侧顺序逐条一致）
    let mut argv: Vec<String> = [
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
    .map(|s| (*s).to_string())
    .collect();

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

/// 把按卵环境变量编成包装脚本：`export K=V …; exec "$@"`（键按名字排序 ⇒ argv 逐字可复现）。
///
/// **为什么不用 bwrap `--setenv`**：X3 真机 2026-09-15 实测，bubblewrap 0.11.1 上
/// `bwrap … --setenv K=V -- /bin/true` 直接失败（`bwrap: setenv failed`，rc=1）⇒ 凡声明了 env 的卵
/// 当场秒死，`LD_LIBRARY_PATH` 这类**必需**通路全断。包装 exec 用同一份卵声明面把变量送进空间。
///
/// 值一律单引号包住（值里的单引号按 POSIX 惯例转义）⇒ 空格/换行/`$`/反引号/双引号都不会被二次展开。
pub fn exec_wrapper_script(env: &std::collections::BTreeMap<String, String>) -> Option<String> {
    if env.is_empty() {
        return None;
    }
    let mut s = String::from("export ");
    for (i, (k, v)) in env.iter().enumerate() {
        if i > 0 {
            s.push(' ');
        }
        s.push_str(k);
        s.push('=');
        s.push_str(&shell_single_quote(v));
    }
    s.push_str("; exec \"$@\"");
    Some(s)
}

/// 把任意字符串包成 shell 单引号字面量（值里的单引号先闭合、加一个转义单引号、再重开）。
pub fn shell_single_quote(v: &str) -> String {
    format!("'{}'", v.replace('\'', "'\\''"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wrapper_quotes_and_sorts() {
        let mut env = std::collections::BTreeMap::new();
        env.insert("B".to_string(), "x'$y".to_string());
        env.insert("A".to_string(), "1 2".to_string());
        assert_eq!(
            exec_wrapper_script(&env).expect("应产出脚本"),
            "export A='1 2' B='x'\\''$y'; exec \"$@\""
        );
        assert!(
            exec_wrapper_script(&Default::default()).is_none(),
            "无变量不加包装"
        );
    }
}
