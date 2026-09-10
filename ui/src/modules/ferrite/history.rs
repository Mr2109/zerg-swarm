//! 操作级撤销（抄 Ferrite EditHistory 精简——M3 Ferrite 重写）
//! 记录 insert/delete 操作而非快照——内存高效

use super::buffer::TextBuffer;
use std::collections::VecDeque;

/// 编辑操作
#[derive(Debug, Clone)]
pub enum EditOp {
    Insert { pos: usize, text: String },
    Delete { pos: usize, text: String },
}

/// 一次编辑（操作组 + 编辑前后光标位置）
///
/// M36(2026-09-10 审计)：原来撤销/重做只把光标夹到 `len`，位置错乱（不在被改文本处）。
/// 现在把编辑前/后的光标一起记录，撤销恢复到 `cursor_before`、重做恢复到 `cursor_after`。
#[derive(Debug, Clone)]
pub struct EditGroup {
    pub ops: Vec<EditOp>,
    pub cursor_before: usize,
    pub cursor_after: usize,
}

/// 撤销/重做双栈
///
/// M39(2026-09-10 审计)：改用 `VecDeque`——溢出裁剪由 `remove(0)`（O(n) 搬移整个 Vec）
/// 改为 `pop_front()`（O(1)）。
#[derive(Debug, Default)]
pub struct EditHistory {
    undo_stack: VecDeque<EditGroup>,
    redo_stack: Vec<EditGroup>,
    max_depth: usize,
}

impl EditHistory {
    pub fn new() -> Self {
        Self {
            undo_stack: VecDeque::new(),
            redo_stack: Vec::new(),
            max_depth: 100,
        }
    }

    /// 记录一次编辑（操作组 + 编辑前后光标）
    pub fn push(&mut self, ops: Vec<EditOp>, cursor_before: usize, cursor_after: usize) {
        if ops.is_empty() {
            return;
        }
        self.undo_stack.push_back(EditGroup {
            ops,
            cursor_before,
            cursor_after,
        });
        // M39: 超出深度用 pop_front（O(1)），不再 remove(0) 搬移
        while self.undo_stack.len() > self.max_depth {
            self.undo_stack.pop_front();
        }
        self.redo_stack.clear(); // 新编辑清空 redo
    }

    /// 撤销——返回可应用的操作组（含光标位置）
    pub fn undo(&mut self) -> Option<EditGroup> {
        let group = self.undo_stack.pop_back()?;
        self.redo_stack.push(group.clone());
        Some(group)
    }

    /// 重做——返回可应用的操作组（含光标位置）
    pub fn redo(&mut self) -> Option<EditGroup> {
        let group = self.redo_stack.pop()?;
        self.undo_stack.push_back(group.clone());
        Some(group)
    }

    pub fn can_undo(&self) -> bool {
        !self.undo_stack.is_empty()
    }

    pub fn can_redo(&self) -> bool {
        !self.redo_stack.is_empty()
    }
}

/// 应用撤销操作组（倒序回滚）
pub fn apply_undo_ops(buffer: &mut TextBuffer, ops: &[EditOp]) {
    for op in ops.iter().rev() {
        match op {
            EditOp::Insert { pos, .. } => {
                buffer.remove(*pos, *pos + op_char_len(op));
            }
            EditOp::Delete { pos, text } => {
                buffer.insert(*pos, text);
            }
        }
    }
}

/// 应用重做操作组（正序）
pub fn apply_redo_ops(buffer: &mut TextBuffer, ops: &[EditOp]) {
    for op in ops {
        match op {
            EditOp::Insert { pos, text } => {
                buffer.insert(*pos, text);
            }
            EditOp::Delete { pos, text } => {
                buffer.remove(*pos, *pos + text.chars().count());
            }
        }
    }
}

fn op_char_len(op: &EditOp) -> usize {
    match op {
        EditOp::Insert { text, .. } => text.chars().count(),
        EditOp::Delete { text, .. } => text.chars().count(),
    }
}
