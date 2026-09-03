//! 大纲解析（抄 Ferrite toc 精简——M3 Ferrite 重写）
//! 解析 markdown 标题（#/##/###）→ 大纲条目——点击跳转

/// 大纲条目
#[derive(Debug, Clone)]
pub struct TocEntry {
    pub level: usize, // 1-3（# 数量）
    pub text: String, // 标题文本
    pub line: usize,  // 标题所在行号（0 基）
}

/// 解析大纲（只取 H1-H3）
pub fn parse_toc(text: &str) -> Vec<TocEntry> {
    let mut entries = Vec::new();
    for (idx, line) in text.lines().enumerate() {
        let trimmed = line.trim_start();
        if !trimmed.starts_with('#') {
            continue;
        }
        // 统计 # 数量
        let mut level = 0;
        for ch in trimmed.chars() {
            if ch == '#' {
                level += 1;
            } else {
                break;
            }
        }
        if level < 1 || level > 3 {
            continue;
        }
        // 跳过 "###" 后空格
        let rest = trimmed[level..].trim_start();
        if rest.is_empty() {
            continue;
        }
        entries.push(TocEntry {
            level,
            text: rest.to_string(),
            line: idx,
        });
    }
    entries
}
