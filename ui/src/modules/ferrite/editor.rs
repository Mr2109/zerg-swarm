//! 单光标核心编辑器（抄 Ferrite FerriteEditor 精简——M3 Ferrite 重写）
//! 功能: 光标移动/文本输入删除/选择/撤销重做——渲染到 egui Ui

use eframe::egui;
use egui::text::{LayoutJob, TextFormat};

use super::buffer::TextBuffer;
use super::history::{apply_redo_ops, apply_undo_ops, EditHistory, EditOp};

/// 编辑器状态
pub struct MdEditor {
    /// 文本缓冲（rope）
    pub buffer: TextBuffer,
    /// 光标字符位置
    pub cursor: usize,
    /// 选择起点（None = 无选择）
    pub selection: Option<usize>,
    /// 撤销历史
    history: EditHistory,
    /// 内容变更标记（外部检查保存）
    pub dirty: bool,
    /// 垂直滚动偏移（字符行）
    pub scroll_line: usize,
    /// F3 行缓存——可见行文本缓存（line → text——编辑后失效）
    line_cache: std::collections::HashMap<usize, String>,
    /// 缓存版本号（编辑后自增——失效所有缓存）
    cache_epoch: u64,
}

impl Default for MdEditor {
    fn default() -> Self {
        Self::new()
    }
}

impl MdEditor {
    pub fn new() -> Self {
        Self {
            buffer: TextBuffer::new(),
            cursor: 0,
            selection: None,
            history: EditHistory::new(),
            dirty: false,
            scroll_line: 0,
            line_cache: std::collections::HashMap::new(),
            cache_epoch: 0,
        }
    }

    /// 载入内容（清历史）
    pub fn load(&mut self, content: &str) {
        self.buffer = TextBuffer::from_string(content);
        self.cursor = 0;
        self.selection = None;
        self.history = EditHistory::new();
        self.dirty = false;
        self.scroll_line = 0;
    }

    /// 取全量文本
    pub fn text(&self) -> String {
        self.buffer.to_string()
    }

    /// 跳到某行（大纲点击——光标定位 + 滚动跟随）
    pub fn jump_to_line(&mut self, line: usize) {
        let line_count = self.buffer.line_count();
        let line = line.min(line_count.saturating_sub(1));
        self.cursor = self.buffer.line_to_char(line);
        self.selection = None;
        // 滚动到目标行（如果不在可见区）
        let visible = 20usize; // 粗略可见行数
        if line < self.scroll_line {
            self.scroll_line = line;
        } else if line >= self.scroll_line + visible {
            self.scroll_line = line.saturating_sub(visible / 2);
        }
    }

    /// 当前光标行
    pub fn cursor_line(&self) -> usize {
        self.buffer.char_to_line(self.cursor)
    }

    /// 总行数
    pub fn line_count(&self) -> usize {
        self.buffer.line_count()
    }

    /// 当前滚动行
    pub fn scroll_line(&self) -> usize {
        self.scroll_line
    }

    // ─────────────── 编辑操作 ───────────────

    /// 插入文本（当前光标）
    pub fn insert_text(&mut self, text: &str) {
        if text.is_empty() {
            return;
        }
        // 有选择——先删
        let ops = self.replace_selection_ops();
        let mut ops = ops.unwrap_or_default();
        ops.push(EditOp::Insert {
            pos: self.cursor,
            text: text.to_string(),
        });
        self.buffer.insert(self.cursor, text);
        self.cursor += text.chars().count();
        self.selection = None;
        self.history.push(ops);
        self.dirty = true;
        self.invalidate_cache();
    }

    /// F5 AI 结果追加到文档末尾（AI 续写用）
    pub fn append_text(&mut self, text: &str) {
        if text.is_empty() {
            return;
        }
        let pos = self.buffer.len();
        self.buffer.insert(pos, text);
        self.cursor = self.buffer.len();
        self.history.push(vec![EditOp::Insert {
            pos,
            text: text.to_string(),
        }]);
        self.dirty = true;
        self.invalidate_cache();
    }

    /// 删除（退格——光标前）
    pub fn backspace(&mut self) {
        if let Some(ops) = self.replace_selection_ops() {
            self.history.push(ops);
            self.dirty = true;
            self.invalidate_cache();
            return;
        }
        if self.cursor == 0 {
            return;
        }
        let start = self.cursor.saturating_sub(1);
        let removed = self.buffer.to_string()[..self.buffer_byte(start)]
            .chars()
            .last()
            .map(|c| c.to_string())
            .unwrap_or_default();
        self.buffer.remove(start, self.cursor);
        self.history.push(vec![EditOp::Delete {
            pos: start,
            text: removed,
        }]);
        self.cursor = start;
        self.dirty = true;
        self.invalidate_cache();
    }

    /// 删除（Del——光标后）
    pub fn delete_forward(&mut self) {
        if let Some(ops) = self.replace_selection_ops() {
            self.history.push(ops);
            self.dirty = true;
            self.invalidate_cache();
            return;
        }
        if self.cursor >= self.buffer.len() {
            return;
        }
        let end = self.cursor + 1;
        let removed = self.buffer.to_string()[self.buffer_byte(self.cursor)..self.buffer_byte(end)]
            .to_string();
        self.buffer.remove(self.cursor, end);
        self.history.push(vec![EditOp::Delete {
            pos: self.cursor,
            text: removed,
        }]);
        self.dirty = true;
        self.invalidate_cache();
    }

    /// 回车（插入 \n）
    pub fn enter(&mut self) {
        self.insert_text("\n");
    }

    /// 有选择时返回删除选择的 ops
    fn replace_selection_ops(&mut self) -> Option<Vec<EditOp>> {
        let (start, end) = self.selection_range()?;
        let removed = self.buffer.to_string()[self.buffer_byte(start)..self.buffer_byte(end)].to_string();
        self.buffer.remove(start, end);
        self.cursor = start;
        self.selection = None;
        Some(vec![EditOp::Delete {
            pos: start,
            text: removed,
        }])
    }

    fn selection_range(&self) -> Option<(usize, usize)> {
        let sel = self.selection?;
        if sel == self.cursor {
            return None;
        }
        Some((sel.min(self.cursor), sel.max(self.cursor)))
    }

    fn buffer_byte(&self, char_pos: usize) -> usize {
        self.buffer.char_to_byte(char_pos)
    }

    // ─────────────── F3 行缓存 ───────────────

    /// 取行文本（缓存命中直接返回——否则从 rope 读并缓存）
    fn get_line_text(&mut self, line: usize) -> String {
        if let Some(t) = self.line_cache.get(&line) {
            return t.clone();
        }
        let t = self.buffer.line_str(line);
        // 只缓存可见窗口附近（防无限增长）
        if self.line_cache.len() > 500 {
            self.line_cache.clear();
        }
        self.line_cache.insert(line, t.clone());
        t
    }

    /// 编辑后失效缓存（清空 + 版本号自增）
    fn invalidate_cache(&mut self) {
        self.line_cache.clear();
        self.cache_epoch += 1;
    }

    // ─────────────── 撤销/重做 ───────────────

    pub fn undo(&mut self) {
        if let Some(ops) = self.history.undo() {
            apply_undo_ops(&mut self.buffer, &ops);
            self.cursor = self.cursor.min(self.buffer.len());
            self.selection = None;
            self.dirty = true;
            self.invalidate_cache();
        }
    }

    pub fn redo(&mut self) {
        if let Some(ops) = self.history.redo() {
            apply_redo_ops(&mut self.buffer, &ops);
            self.cursor = self.cursor.min(self.buffer.len());
            self.selection = None;
            self.dirty = true;
            self.invalidate_cache();
        }
    }

    // ─────────────── 光标移动 ───────────────

    pub fn move_left(&mut self) {
        if self.cursor > 0 {
            self.cursor -= 1;
        }
        self.selection = None;
    }

    pub fn move_right(&mut self) {
        if self.cursor < self.buffer.len() {
            self.cursor += 1;
        }
        self.selection = None;
    }

    pub fn move_up(&mut self) {
        let line = self.cursor_line();
        if line > 0 {
            let line_start = self.buffer.line_to_char(line - 1);
            let col = self.cursor - self.buffer.line_to_char(line);
            self.cursor = (line_start + col).min(self.buffer.line_to_char(line) - 1).max(line_start);
        }
        self.selection = None;
    }

    pub fn move_down(&mut self) {
        let line = self.cursor_line();
        if line + 1 < self.buffer.line_count() {
            let next_start = self.buffer.line_to_char(line + 1);
            let col = self.cursor - self.buffer.line_to_char(line);
            let next_end = if line + 2 < self.buffer.line_count() {
                self.buffer.line_to_char(line + 2).saturating_sub(1)
            } else {
                self.buffer.len()
            };
            self.cursor = (next_start + col).min(next_end).max(next_start);
        }
        self.selection = None;
    }

    pub fn move_home(&mut self) {
        let line = self.cursor_line();
        self.cursor = self.buffer.line_to_char(line);
        self.selection = None;
    }

    pub fn move_end(&mut self) {
        let line = self.cursor_line();
        let line_end = if line + 1 < self.buffer.line_count() {
            self.buffer.line_to_char(line + 1).saturating_sub(1)
        } else {
            self.buffer.len()
        };
        self.cursor = line_end.max(self.buffer.line_to_char(line));
        self.selection = None;
    }

    // ─────────────── 渲染 ───────────────

    /// 渲染编辑器到 Ui（核心——逐行绘制——F3 虚拟滚动/滚轮/点击）
    pub fn render(&mut self, ui: &mut egui::Ui) {
        let cursor_line = self.cursor_line();
        let line_count = self.buffer.line_count();

        // 可见行范围（虚拟滚动——每行固定高度）
        let row_height = ui.text_style_height(&egui::TextStyle::Monospace);
        let avail_h = ui.available_height().max(100.0);
        let visible_lines = (avail_h / row_height).max(5.0) as usize;
        let scroll_max = line_count.saturating_sub(visible_lines);
        if self.scroll_line > scroll_max {
            self.scroll_line = scroll_max;
        }

        // F3 滚轮滚动（egui scroll_delta——更新 scroll_line）
        let scroll_delta = ui.input(|i| i.smooth_scroll_delta.y);
        if scroll_delta.abs() > 0.0 {
            let lines = (scroll_delta / row_height).round() as i64;
            self.scroll_line = (self.scroll_line as i64 + lines).clamp(0, scroll_max as i64) as usize;
        }

        // F3 点击定位光标（鼠标点击 → 行 → 字符位置）
        let click_pos = ui.input(|i| i.pointer.interact_pos());
        if let Some(pos) = click_pos {
            if ui.rect_contains_pointer(ui.max_rect()) && ui.input(|i| i.pointer.any_click()) {
                let rel_y = pos.y - ui.max_rect().top();
                let line = self.scroll_line + (rel_y / row_height) as usize;
                if line < line_count {
                    self.cursor = self.buffer.line_to_char(line);
                }
            }
        }

        let _ = egui::ScrollArea::vertical()
            .id_salt("ferrite_edit")
            .auto_shrink(false)
            .show(ui, |ui| {
                let start_line = self.scroll_line;
                let end_line = (start_line + visible_lines + 2).min(line_count);

                for line in start_line..end_line {
                    let line_text = self.get_line_text(line);
                    let is_cursor_line = line == cursor_line;
                    let mut job = LayoutJob::default();
                    let fmt = TextFormat {
                        font_id: egui::FontId::monospace(14.0),
                        ..Default::default()
                    };
                    // 行号
                    job.append(
                        &format!("{:>4} ", line + 1),
                        0.0,
                        TextFormat {
                            font_id: egui::FontId::monospace(12.0),
                            color: egui::Color32::from_gray(120),
                            ..Default::default()
                        },
                    );
                    // 光标行高亮
                    if is_cursor_line {
                        job.append(
                            &line_text,
                            0.0,
                            TextFormat {
                                font_id: egui::FontId::monospace(14.0),
                                background: egui::Color32::from_rgb(30, 45, 60),
                                ..fmt.clone()
                            },
                        );
                    } else {
                        job.append(&line_text, 0.0, fmt.clone());
                    }
                    ui.label(job);
                }
            });

        // 键盘输入
        let output = ui.ctx().input(|i| {
            let mut out: Vec<KeyAction> = Vec::new();
            for e in &i.events {
                match e {
                    egui::Event::Text(t) => out.push(KeyAction::Text(t.clone())),
                    egui::Event::Key {
                        key,
                        pressed: true,
                        modifiers,
                        ..
                    } => match key {
                        egui::Key::Backspace => out.push(KeyAction::Backspace),
                        egui::Key::Delete => out.push(KeyAction::DeleteForward),
                        egui::Key::Enter => out.push(KeyAction::Enter),
                        egui::Key::ArrowLeft if !modifiers.shift => out.push(KeyAction::Left),
                        egui::Key::ArrowRight if !modifiers.shift => out.push(KeyAction::Right),
                        egui::Key::ArrowUp if !modifiers.shift => out.push(KeyAction::Up),
                        egui::Key::ArrowDown if !modifiers.shift => out.push(KeyAction::Down),
                        egui::Key::Home if !modifiers.shift => out.push(KeyAction::Home),
                        egui::Key::End if !modifiers.shift => out.push(KeyAction::End),
                        egui::Key::Z if modifiers.command && !modifiers.shift => {
                            out.push(KeyAction::Undo)
                        }
                        egui::Key::Z if modifiers.command && modifiers.shift => {
                            out.push(KeyAction::Redo)
                        }
                        _ => {}
                    },
                    _ => {}
                }
            }
            out
        });

        for action in output {
            match action {
                KeyAction::Text(t) => self.insert_text(&t),
                KeyAction::Backspace => self.backspace(),
                KeyAction::DeleteForward => self.delete_forward(),
                KeyAction::Enter => self.enter(),
                KeyAction::Left => self.move_left(),
                KeyAction::Right => self.move_right(),
                KeyAction::Up => self.move_up(),
                KeyAction::Down => self.move_down(),
                KeyAction::Home => self.move_home(),
                KeyAction::End => self.move_end(),
                KeyAction::Undo => self.undo(),
                KeyAction::Redo => self.redo(),
            }
        }
    }
}

enum KeyAction {
    Text(String),
    Backspace,
    DeleteForward,
    Enter,
    Left,
    Right,
    Up,
    Down,
    Home,
    End,
    Undo,
    Redo,
}
