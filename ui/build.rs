// build.rs — 把代码身份（commit + 构建时间）编进 UI 二进制（2026-09-11，自动升级模块 P0-a）
//
// 为什么：自动升级的 verify 阶段要回答"现在跑的是哪份代码"——UI 也必须能自报，
// 不能只有主控能自报（否则版本矩阵缺一件）。UI 侧版本号已由 Cargo.toml 经
// env!("CARGO_PKG_VERSION") 提供；这里补上 git sha 与构建时间。
//
// 取值：优先 git（浅克隆/无 git 时回落 unknown），时间用 UTC ISO8601。
use std::process::Command;

fn main() {
    let sha = cmd("git", &["rev-parse", "--short", "HEAD"]).unwrap_or_else(|| "unknown".to_string());
    let dirty = cmd("git", &["status", "--porcelain"])
        .map(|s| !s.trim().is_empty())
        .unwrap_or(false);
    let sha = if dirty && sha != "unknown" { format!("{}+dirty", sha) } else { sha };
    let build_time = cmd("date", &["-u", "+%Y-%m-%dT%H:%M:%SZ"])
        .unwrap_or_else(|| "unknown".to_string());

    println!("cargo:rustc-env=ZERG_GIT_SHA={}", sha);
    println!("cargo:rustc-env=ZERG_BUILD_TIME={}", build_time);

    // HEAD 变化时重跑（浅克隆下路径可能不存在——忽略即可）
    println!("cargo:rerun-if-changed=../.git/HEAD");
    println!("cargo:rerun-if-changed=build.rs");
}

fn cmd(program: &str, args: &[&str]) -> Option<String> {
    let out = Command::new(program).args(args).output().ok()?;
    if !out.status.success() {
        return None;
    }
    let s = String::from_utf8_lossy(&out.stdout).trim().to_string();
    if s.is_empty() {
        None
    } else {
        Some(s)
    }
}
