//! 文本缓冲（抄 Ferrite TextBuffer 精简——M3 Ferrite 重写）
//! ropey::Rope 封装——O(log n) 文本操作——4MB 文件流畅

use ropey::Rope;

/// 文本缓冲——rope 数据结构支撑大文件编辑
#[derive(Debug, Clone)]
pub struct TextBuffer {
    rope: Rope,
}

impl Default for TextBuffer {
    fn default() -> Self {
        Self::new()
    }
}

impl TextBuffer {
    /// 新建空缓冲
    pub fn new() -> Self {
        Self { rope: Rope::new() }
    }

    /// 从字符串创建
    pub fn from_string(content: &str) -> Self {
        Self {
            rope: Rope::from_str(content),
        }
    }

    /// 字符长度
    pub fn len(&self) -> usize {
        self.rope.len_chars()
    }

    /// 是否空
    pub fn is_empty(&self) -> bool {
        self.rope.len_chars() == 0
    }

    /// 行数（空缓冲 = 1 行）
    pub fn line_count(&self) -> usize {
        self.rope.len_lines()
    }

    /// 插入文本（字符位置）
    pub fn insert(&mut self, pos: usize, text: &str) {
        let char_pos = self.rope.len_chars().min(pos);
        self.rope.insert(char_pos, text);
    }

    /// 删除区间（字符位置）
    pub fn remove(&mut self, start: usize, end: usize) {
        let len = self.rope.len_chars();
        let s = start.min(len);
        let e = end.min(len).max(s);
        self.rope.remove(s..e);
    }

    /// 全量文本（预览/保存用）
    pub fn to_string(&self) -> String {
        self.rope.to_string()
    }

    /// 行数→字符位置
    pub fn line_to_char(&self, line: usize) -> usize {
        self.rope.line_to_char(line.min(self.line_count().saturating_sub(1)))
    }

    /// 字符位置→行数
    pub fn char_to_line(&self, pos: usize) -> usize {
        self.rope.char_to_line(pos.min(self.len()))
    }

    /// 字符位置→字节位置（M07 2026-09-10 审计修复）
    /// 原实现逐行 to_string() 累加字节 → O(行数)，大文档每次按键都全量扫；
    /// ropey 的 `slice(0..pos).len_bytes()` 沿树求字节长度 → O(log n)。
    pub fn char_to_byte(&self, char_pos: usize) -> usize {
        let pos = char_pos.min(self.len());
        self.rope.slice(0..pos).len_bytes()
    }

    /// 取字符区间文本（M07：替代 `to_string()[a..b]` 的全量克隆——只取所需片段）
    pub fn slice_chars(&self, start: usize, end: usize) -> String {
        let len = self.rope.len_chars();
        let s = start.min(len);
        let e = end.min(len).max(s);
        self.rope.slice(s..e).to_string()
    }

    /// 取单个字符（越界返回 None）
    pub fn char_at(&self, pos: usize) -> Option<char> {
        if pos >= self.rope.len_chars() {
            return None;
        }
        Some(self.rope.char(pos))
    }

    /// 取某行文本
    pub fn line_str(&self, line: usize) -> String {
        let line = line.min(self.line_count().saturating_sub(1));
        self.rope.line(line).to_string()
    }
}
