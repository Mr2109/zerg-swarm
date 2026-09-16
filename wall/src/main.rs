//! `zerg-wall` 命令行入口（批 2'.1 冻结的对外接口）。
//!
//! ```text
//! zerg-wall plan --spec <spec.json> [--platform linux|macos]
//!                                     # 纯函数：读配方 ⇒ stdout 一行 JSON（argv + allowlist + note）；不 exec、不写盘
//!                                     # --platform 仅供**离线**出别的平台的计划（交叉对拍/取证）；默认本机平台
//! zerg-wall run  --spec <spec.json>   # 先出计划（写到 stderr 留痕），再按 argv exec；子进程退出码原样透传
//!                                     # run **不接受** --platform：要跑的就是本机这一格
//!                                     # macOS 一档：exec 之前由**本进程**直调 Seatbelt 施加策略
//!                                     #（策略原文也进 stderr 留痕）；施加失败 ⇒ rc=2，**绝不回落成裸跑**
//! zerg-wall --version
//! ```
//!
//! **退出码**（与仓内判据脚本同一套语义，别混）：
//!   - `0`  成功；
//!   - `2`  **硬失败**：配方被拒 / 用法错 / 平台没落地 / 可执行不在 PATH（`run` 时）——
//!     一律是这一码，因为对消费侧而言「被拒」与「跑不通」都必须**当成红灯处理**，
//!     绝不允许被读成「没封闭但照跑」。子进程自己退出码 `exit code` 原样透传（`run`）。

use std::process::{exit, Command};

use zerg_wall::{json, plan_for, spec::Spec, Error};

fn main() {
    match run() {
        Ok(code) => exit(code),
        Err(e) => {
            eprintln!("茧壁（zerg-wall）：{e}");
            exit(2);
        }
    }
}

fn usage() -> Error {
    Error::new(
        "用法：zerg-wall plan --spec <spec.json> [--platform linux|macos] | \
         zerg-wall run --spec <spec.json> | zerg-wall --version",
    )
}

fn run() -> Result<i32, Error> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let first = args.first().ok_or_else(usage)?;
    match first.as_str() {
        "--version" | "-V" => {
            println!("zerg-wall {}", env!("CARGO_PKG_VERSION"));
            Ok(0)
        }
        "plan" | "run" => {
            let (path, asked) = parse_args(&args)?;
            let spec = load_spec(&path)?;
            let host = zerg_wall::current_platform();
            let os = if first == "plan" {
                // `plan` 允许**离线**点名平台（给交叉对拍/离线取证用）：默认仍是本机平台。
                // 「认不得的平台 / 一档没落地」都由 plan_for 拒绝 —— 不存在「伪装平台混过去」。
                asked.clone().unwrap_or_else(|| host.clone())
            } else {
                // `run` **不许**点名平台：要跑的就是本机这一格；拿别的平台的计划去跑是说不清的事。
                if let Some(p) = &asked {
                    return Err(Error::new(&format!(
                        "run 不接受 --platform {p:?}——拒：要跑的就是本机平台（{host}）；\
                         交叉平台的计划只能用 plan 离线出"
                    )));
                }
                host
            };
            let p = plan_for(&os, &spec)?;
            if first == "plan" {
                // 一行 JSON 到 stdout —— 这就是冻结的产物形状
                println!("{}", p.to_json());
                return Ok(0);
            }
            // run：计划留痕到 stderr（stdout 留给子进程），再施加封闭、最后 exec
            eprintln!("茧壁计划（run）：{}", p.to_json());
            if p.argv.is_empty() {
                return Err(Error::new("计划里没有可执行的 argv——拒（不猜）"));
            }
            // 封闭由谁施加：Linux 由 argv 里的 bwrap 施加；macOS 没有外部沙箱程序 ⇒ 本进程直调
            // Seatbelt。两种形态都**留痕**（策略原文进 stderr），失败一律报错、绝不改道 ✓
            let conf = zerg_wall::platform::confinement_for(&os, &spec)?;
            if let zerg_wall::platform::Confinement::InProcess { profile } = &conf {
                eprintln!(
                    "茧壁策略（本进程直调 Seatbelt，{} 字节）：\n{profile}",
                    profile.len()
                );
            }
            conf.enforce()?;
            let status = Command::new(&p.argv[0])
                .args(&p.argv[1..])
                .status()
                .map_err(|e| {
                    Error::new(&format!(
                        "起不来 {:?}（{}）——拒绝：**绝不**改用别的执行方式或裸跑",
                        p.argv[0], e
                    ))
                })?;
            match status.code() {
                Some(c) => Ok(c),
                None => {
                    eprintln!("茧壁：子进程被信号终止（没有退出码）");
                    Ok(2)
                }
            }
        }
        other => Err(Error::new(&format!(
            "认不得的子命令 {other:?}——拒（不许静默忽略参数）；{}",
            usage().msg()
        ))),
    }
}

/// 解析参数：`--spec <path>`（必需）· `--platform <linux|macos>`（可选，**只给 `plan` 用**）。
///
/// 其余参数一律拒绝：静默忽略参数会让「跑错配方 / 跑错平台」看不出来。
fn parse_args(args: &[String]) -> Result<(String, Option<String>), Error> {
    let mut spec: Option<String> = None;
    let mut platform: Option<String> = None;
    let mut i = 1; // args[0] 是子命令
    while i < args.len() {
        let a = &args[i];
        let (key, inline) = match a.split_once('=') {
            Some((k, v)) => (k.to_string(), Some(v.to_string())),
            None => (a.clone(), None),
        };
        let value = match inline {
            Some(v) => v,
            None => {
                let v = args
                    .get(i + 1)
                    .ok_or_else(|| Error::new(&format!("{key} 后面缺值——拒")))?
                    .clone();
                i += 1;
                v
            }
        };
        if value.trim().is_empty() {
            return Err(Error::new(&format!("{key} 的值是空串——拒")));
        }
        let slot = match key.as_str() {
            "--spec" => &mut spec,
            "--platform" => &mut platform,
            other => {
                return Err(Error::new(&format!(
                    "认不得的参数 {other:?}——拒（不许静默忽略参数）"
                )));
            }
        };
        if slot.is_some() {
            return Err(Error::new(&format!("{key} 给了两次——拒（说不清用哪一个）")));
        }
        *slot = Some(value);
        i += 1;
    }
    let spec =
        spec.ok_or_else(|| Error::new("缺 --spec <spec.json>——拒（配方是唯一输入，不许猜）"))?;
    Ok((spec, platform))
}

/// 读配方文件并解析（解析路径只有这一条：读写两端同源，不另开一份）。
fn load_spec(path: &str) -> Result<Spec, Error> {
    let text = std::fs::read_to_string(path)
        .map_err(|e| Error::new(&format!("读不到配方 {path:?}：{e}——拒")))?;
    let v = json::parse(&text)?;
    Spec::from_json(&v)
}
