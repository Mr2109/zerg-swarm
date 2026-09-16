//! 共享的 **exec 包装**：把「按卵环境变量」编成一条 shell 前缀（`export K=V …; exec "$@"`）。
//!
//! **为什么它不在 `platform/linux.rs` 里**：Linux 与 macOS 两个一档都走同一条包装
//! （bwrap 里 `-- /bin/sh -c …`；macOS 里策略施加后 exec `/bin/sh -c …`）⇒ 两处各留一份
//! 就会漂移（同一个落点两套真相），故提到这里、两侧共用。
//!
//! **为什么不用引擎自带的机制**：X3 真机 2026-09-15 实测，bubblewrap 0.11.1 上
//! `bwrap … --setenv K=V -- /bin/true` 直接失败（`bwrap: setenv failed`，rc=1）⇒ 凡声明了 env 的卵
//! 当场秒死，`LD_LIBRARY_PATH` 这类**必需**通路全断。包装 exec 用同一份卵声明面把变量送进空间。

use std::collections::BTreeMap;

/// 空间内 shell（`/bin` 是 usrmerge 符号链接；真机实测可用；macOS 上 `/bin/sh` 同样在）。
pub const SH_PATH: &str = "/bin/sh";

/// 包装脚本的 `$0`（**不参与** `exec "$@"`）。
///
/// 为什么用一个固定普通字当 `$0`（而不是顺手写 `--`）：那个 `--` 到底是 `$0` 还是「选项终止符」
/// 随 shell 实现而异 —— 若被当选项终止符，`$0` 会变成引擎路径、`$@` 少一个参数，
/// 于是「引擎名丢了却照样能起」（参数错位没人看得出来）。
pub const WRAPPER_ARG0: &str = "zerg-egg";

/// 把按卵环境变量编成包装脚本：`export K=V …; exec "$@"`（键按名字排序 ⇒ argv 逐字可复现）。
///
/// 值一律单引号包住（值里的单引号按 POSIX 惯例转义）⇒ 空格/换行/`$`/反引号/双引号都不会被二次展开。
/// 无变量时返回 `None`（**不加一层 sh**：多一层会把 execve 失败的归因变含糊）。
pub fn exec_wrapper_script(env: &BTreeMap<String, String>) -> Option<String> {
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
        let mut env = BTreeMap::new();
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
