//! 错误类型：茧壁的失败**一律是拒绝**（fail-closed），不做「降级继续跑」。
//!
//! 为什么不直接用 `String`：调用方（`main.rs` 的退出码 / 将来的 Go 侧）要能区分
//! 「配方被拒」与「程序自己出错」——本端把所有拒绝都归到退出码 `2`（见 `main.rs` 的口径说明）。

use std::fmt;

/// 茧壁错误（拒绝理由）。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Error {
    msg: String,
}

impl Error {
    /// 把一段拒绝理由包成错误（理由里要写清「拒绝什么、为什么」）。
    pub fn new(msg: &str) -> Self {
        Self {
            msg: msg.to_string(),
        }
    }

    /// 拒绝理由原文。
    pub fn msg(&self) -> &str {
        &self.msg
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.msg)
    }
}

impl std::error::Error for Error {}
