//! 操作级撤销（抄 Ferrite EditHistory 精简——M3 Ferrite 重写）
//! 记录 insert/delete 操作而非快照——内存高效

use super::buffer::TextBuffer;

/// 编辑操作
#[derive(Debug, Clone)]
pub enum EditOp {
    Insert { pos: usize, text: String },
    Delete { pos: usize, text: String },
}

/// 撤销/重做双栈
#[derive(Debug, Default)]
pub struct EditHistory {
    undo_stack: Vec<Vec<EditOp>>, // 每组 = 一次编辑（可能多操作）
    redo_stack: Vec<Vec<EditOp>>,
    max_depth: usize,
}

impl EditHistory {
    pub fn new() -> Self {
        Self {
            undo_stack: Vec::new(),
            redo_stack: Vec::new(),
            max_depth: 100,
        }
    }

    /// 记录一次编辑（操作组）
    pub fn push(&mut self, ops: Vec<EditOp>) {
        if ops.is_empty() {
            return;
        }
        self.undo_stack.push(ops);
        if self.undo_stack.len() > self.max_depth {
            self.undo_stack.remove(0);
        }
        self.redo_stack.clear(); // 新编辑清空 redo
    }

    /// 撤销——返回可应用的操作组
    pub fn undo(&mut self) -> Option<Vec<EditOp>> {
        let ops = self.undo_stack.pop()?;
        self.redo_stack.push(ops.clone());
        Some(ops)
    }

    /// 重做——返回可应用的操作组
    pub fn redo(&mut self) -> Option<Vec<EditOp>> {
        let ops = self.redo_stack.pop()?;
        self.undo_stack.push(ops.clone());
        Some(ops)
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
