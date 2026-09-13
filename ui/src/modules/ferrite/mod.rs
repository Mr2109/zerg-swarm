//! Ferrite 重写 md 编辑器（M3——Mr2109一步到位重写）
//! 抄 Ferrite 源码核心——完全适配虫族——实现 ZergModule 虫茧标准
//!
//! 2026-09-13（C9 第 4 步）：文档界面整块迁进文档茧（`zerg-cocoon/文档`）后，本模块的
//! **自研渲染半场**（`markdown`/`syntax`/`toc`）与编辑器的一部分方法（大纲跳转/滚动同步/
//! 追写等）在宿主里**暂无消费者**——但它们是**宿主能力**（设计 §4.2：Ferrite 绳编辑器留主仓），
//! 且茧侧借用的正是本模块（`MdEditor` 经 `HostEditor` 通道）。**不删能力**（红线：不把宿主
//! 能力删掉、也不把它的代码复制进茧），只在本模块统一免责，免得一大片 dead_code 噪声掩盖真问题。
// 保留：本模块部分 API 经 `dyn`（茧契约的 HostEditor 等）调用，rustc 看不见 ⇒ 会误报 dead_code。
// 2026-09-14：确凿死掉的 markdown/syntax/toc 三块已删除（各 336/175/73 行），此处只为动态调用留豁免。
#![allow(dead_code)]

pub mod buffer;
pub mod editor;
pub mod history;

pub use editor::MdEditor;
