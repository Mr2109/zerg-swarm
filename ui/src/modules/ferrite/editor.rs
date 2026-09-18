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
    /// 选择起点（None = 无选择）——M35: shift+方向键会设置此项（锚点）
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
        self.invalidate_cache(); // M06: 载入即版本自增——外部按 epoch 的缓存随之失效
    }

    /// 缓存版本号（M06 2026-09-10 审计：预览/大纲文本按此缓存，避免每帧 Rope→String 克隆）
    pub fn epoch(&self) -> u64 {
        self.cache_epoch
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
        let cursor_before = self.cursor; // M36: 记录编辑前光标（撤销恢复用）
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
        self.history.push(ops, cursor_before, self.cursor); // M36
        self.dirty = true;
        self.invalidate_cache();
    }

    /// F5 AI 结果追加到文档末尾（AI 续写用）
    pub fn append_text(&mut self, text: &str) {
        if text.is_empty() {
            return;
        }
        let cursor_before = self.cursor; // M36
        let pos = self.buffer.len();
        self.buffer.insert(pos, text);
        self.cursor = self.buffer.len();
        self.history.push(
            vec![EditOp::Insert {
                pos,
                text: text.to_string(),
            }],
            cursor_before,
            self.cursor,
        );
        self.dirty = true;
        self.invalidate_cache();
    }

    /// 删除（退格——光标前）
    pub fn backspace(&mut self) {
        let cursor_before = self.cursor; // M36
        if let Some(ops) = self.replace_selection_ops() {
            self.history.push(ops, cursor_before, self.cursor);
            self.dirty = true;
            self.invalidate_cache();
            return;
        }
        if self.cursor == 0 {
            return;
        }
        let start = self.cursor.saturating_sub(1);
        // M01(2026-09-10 审计)修复：原来用 `..buffer_byte(start)` 取"被删字符"，
        // 实际切到的是 start-1 位置（差一位）→ 撤销会把文本改坏；改为取 [start, cursor) 区间
        let removed = self.buffer.slice_chars(start, self.cursor);
        self.buffer.remove(start, self.cursor);
        self.cursor = start;
        self.history.push(
            vec![EditOp::Delete {
                pos: start,
                text: removed,
            }],
            cursor_before,
            self.cursor,
        );
        self.dirty = true;
        self.invalidate_cache();
    }

    /// 删除（Del——光标后）
    pub fn delete_forward(&mut self) {
        let cursor_before = self.cursor; // M36
        if let Some(ops) = self.replace_selection_ops() {
            self.history.push(ops, cursor_before, self.cursor);
            self.dirty = true;
            self.invalidate_cache();
            return;
        }
        if self.cursor >= self.buffer.len() {
            return;
        }
        let end = self.cursor + 1;
        let removed = self.buffer.slice_chars(self.cursor, end); // M07: 不再整篇 to_string()
        self.buffer.remove(self.cursor, end);
        self.history.push(
            vec![EditOp::Delete {
                pos: self.cursor,
                text: removed,
            }],
            cursor_before,
            self.cursor,
        );
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
        let removed = self.buffer.slice_chars(start, end); // M07: 只取选区片段
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
        if let Some(group) = self.history.undo() {
            apply_undo_ops(&mut self.buffer, &group.ops);
            // M36(2026-09-10 审计): 恢复编辑前光标（原来只 min(len)——撤销后光标不在被改文本处）
            self.cursor = group.cursor_before.min(self.buffer.len());
            self.selection = None;
            self.dirty = true;
            self.invalidate_cache();
        }
    }

    pub fn redo(&mut self) {
        if let Some(group) = self.history.redo() {
            apply_redo_ops(&mut self.buffer, &group.ops);
            // M36: 恢复到编辑后光标
            self.cursor = group.cursor_after.min(self.buffer.len());
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
            self.cursor = (line_start + col)
                .min(self.buffer.line_to_char(line) - 1)
                .max(line_start);
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

    // ─────── M35(2026-09-10 审计): Shift+方向键扩展选择 ───────
    // 原来全文件只有把 selection 置 None 的赋值、没有任何 `selection = Some(..)`，
    // shift+方向键也被显式排除 → 选择/替换功能全是死代码。以下 *_extend 变体
    // 在移动光标前用当前光标作为锚点，形成 [锚点, 光标) 选区（普通 move_* 仍清空选区）。

    /// 若尚无选择，以当前光标为选择锚点
    fn ensure_sel_anchor(&mut self) {
        if self.selection.is_none() {
            self.selection = Some(self.cursor);
        }
    }

    /// 锚点与光标重合时清除选区（避免"零宽选择"）
    fn clear_collapsed_selection(&mut self) {
        if self.selection == Some(self.cursor) {
            self.selection = None;
        }
    }

    pub fn move_left_extend(&mut self) {
        self.ensure_sel_anchor();
        if self.cursor > 0 {
            self.cursor -= 1;
        }
        self.clear_collapsed_selection();
    }

    pub fn move_right_extend(&mut self) {
        self.ensure_sel_anchor();
        if self.cursor < self.buffer.len() {
            self.cursor += 1;
        }
        self.clear_collapsed_selection();
    }

    pub fn move_up_extend(&mut self) {
        self.ensure_sel_anchor();
        let line = self.cursor_line();
        if line > 0 {
            let line_start = self.buffer.line_to_char(line - 1);
            let col = self.cursor - self.buffer.line_to_char(line);
            self.cursor = (line_start + col)
                .min(self.buffer.line_to_char(line) - 1)
                .max(line_start);
        }
        self.clear_collapsed_selection();
    }

    pub fn move_down_extend(&mut self) {
        self.ensure_sel_anchor();
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
        self.clear_collapsed_selection();
    }

    pub fn move_home_extend(&mut self) {
        self.ensure_sel_anchor();
        let line = self.cursor_line();
        self.cursor = self.buffer.line_to_char(line);
        self.clear_collapsed_selection();
    }

    pub fn move_end_extend(&mut self) {
        self.ensure_sel_anchor();
        let line = self.cursor_line();
        let line_end = if line + 1 < self.buffer.line_count() {
            self.buffer.line_to_char(line + 1).saturating_sub(1)
        } else {
            self.buffer.len()
        };
        self.cursor = line_end.max(self.buffer.line_to_char(line));
        self.clear_collapsed_selection();
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

        // M16/M17(2026-09-10 审计): 指针悬停判定——编辑器不再消费"全局"滚轮
        let pointer_over = ui.rect_contains_pointer(ui.max_rect());

        // F3 滚轮滚动（egui scroll_delta——更新 scroll_line）
        let scroll_delta = ui.input(|i| i.smooth_scroll_delta.y);
        if pointer_over && scroll_delta.abs() > 0.0 {
            let lines = (scroll_delta / row_height).round() as i64;
            self.scroll_line =
                (self.scroll_line as i64 + lines).clamp(0, scroll_max as i64) as usize;
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

        // M16(2026-09-10 审计): 编辑器可聚焦区域——仅聚焦时消费键盘。
        // 原来直接读全局 events（不判焦点）→ 任何控件里的输入都会同时写进编辑器 buffer。
        let edit_resp = ui.interact(
            ui.max_rect(),
            ui.id().with("ferrite_editor_focus"),
            egui::Sense::click(),
        );
        if edit_resp.clicked() {
            edit_resp.request_focus();
        }
        let editor_focused = edit_resp.has_focus();

        // 键盘输入（仅聚焦时）
        let output = if !editor_focused {
            Vec::new()
        } else {
            ui.ctx().input(|i| {
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
                            // M35: Shift+方向键/Home/End → 扩展选择（原来被排除=选择功能死代码）
                            egui::Key::ArrowLeft if modifiers.shift => {
                                out.push(KeyAction::SelectLeft)
                            }
                            egui::Key::ArrowRight if modifiers.shift => {
                                out.push(KeyAction::SelectRight)
                            }
                            egui::Key::ArrowUp if modifiers.shift => out.push(KeyAction::SelectUp),
                            egui::Key::ArrowDown if modifiers.shift => {
                                out.push(KeyAction::SelectDown)
                            }
                            egui::Key::Home if modifiers.shift => out.push(KeyAction::SelectHome),
                            egui::Key::End if modifiers.shift => out.push(KeyAction::SelectEnd),
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
            })
        };

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
                // M35: 扩展选择
                KeyAction::SelectLeft => self.move_left_extend(),
                KeyAction::SelectRight => self.move_right_extend(),
                KeyAction::SelectUp => self.move_up_extend(),
                KeyAction::SelectDown => self.move_down_extend(),
                KeyAction::SelectHome => self.move_home_extend(),
                KeyAction::SelectEnd => self.move_end_extend(),
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
    // M35: Shift 扩展选择
    SelectLeft,
    SelectRight,
    SelectUp,
    SelectDown,
    SelectHome,
    SelectEnd,
}

#[cfg(test)]
mod m01_tests {
    use super::*;

    /// M01 回归（2026-09-10 审计）：退格 → 撤销 必须还原原文（原来记录错字符，撤销把文本改坏）
    #[test]
    fn backspace_undo_restores_original() {
        for original in ["abc", "你好a", "cat猫", ""] {
            let mut ed = MdEditor::new();
            ed.load(original);
            ed.cursor = ed.buffer.len();
            let before_len = ed.buffer.len();
            if before_len == 0 {
                continue;
            }
            ed.backspace();
            assert_eq!(
                ed.buffer.len(),
                before_len - 1,
                "退格应删一个字符: {}",
                original
            );
            ed.undo();
            assert_eq!(
                ed.buffer.to_string(),
                original,
                "撤销后应还原原文: {}",
                original
            );
        }
    }

    /// M07 回归：char_to_byte 与 ropey 字节长度一致（含多字节 CJK/emoji）
    #[test]
    fn char_to_byte_matches_bytes() {
        let mut ed = MdEditor::new();
        ed.load("你好a🙂bc");
        for pos in 0..=ed.buffer.len() {
            let expected = ed
                .buffer
                .to_string()
                .chars()
                .take(pos)
                .map(|c| c.len_utf8())
                .sum::<usize>();
            assert_eq!(
                ed.buffer.char_to_byte(pos),
                expected,
                "char {} 字节偏移不一致",
                pos
            );
        }
    }

    /// M36 回归：撤销/重做恢复光标到编辑位置（原来只 min(len)，光标错乱）
    #[test]
    fn undo_redo_restores_cursor() {
        let mut ed = MdEditor::new();
        ed.load("hello world");
        ed.cursor = 5;
        ed.insert_text("X");
        assert_eq!(ed.buffer.to_string(), "helloX world");
        assert_eq!(ed.cursor, 6);
        ed.undo();
        assert_eq!(ed.buffer.to_string(), "hello world");
        assert_eq!(ed.cursor, 5, "撤销应恢复到编辑前光标");
        ed.redo();
        assert_eq!(ed.buffer.to_string(), "helloX world");
        assert_eq!(ed.cursor, 6, "重做应恢复到编辑后光标");
    }

    /// M35 回归：Shift+方向键设置选择；输入时替换选区（原选择功能全是死代码）
    #[test]
    fn shift_arrow_sets_selection_and_replaces() {
        let mut ed = MdEditor::new();
        ed.load("abcdef");
        ed.cursor = 6;
        ed.move_left_extend();
        ed.move_left_extend();
        assert_eq!(ed.selection, Some(6), "shift 扩展应以原光标为锚点");
        assert_eq!(ed.cursor, 4);
        ed.insert_text("XY");
        assert_eq!(ed.buffer.to_string(), "abcdXY", "插入应替换选区");
        assert_eq!(ed.selection, None);
    }

    /// M36/M39 回归：历史溢出裁剪后仍可连续撤销且光标正确
    #[test]
    fn history_overflow_keeps_recent_edits() {
        let mut ed = MdEditor::new();
        ed.load("");
        for i in 0..120u32 {
            ed.cursor = ed.buffer.len();
            ed.insert_text(&((b'a' + (i % 26) as u8) as char).to_string());
        }
        assert_eq!(ed.buffer.len(), 120);
        for _ in 0..100 {
            ed.undo();
        }
        // 只保留最近 100 次编辑——撤销 100 次后应剩 20 个字符
        assert_eq!(ed.buffer.len(), 20, "溢出裁剪应只丢最旧的编辑");
    }
}
