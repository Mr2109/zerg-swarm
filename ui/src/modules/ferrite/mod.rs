//! Ferrite 重写 md 编辑器（M3——Mr2109一步到位重写）
//! 抄 Ferrite 源码核心——完全适配虫族——实现 ZergModule 集装箱标准

pub mod buffer;
pub mod editor;
pub mod history;
pub mod markdown;
pub mod syntax; // F6 语法高亮（syntect——代码块）
pub mod toc;

pub use editor::MdEditor;
