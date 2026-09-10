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
        // M38(2026-09-10 审计): CommonMark 要求 # 之后必须是空格/制表符/行尾，
        // 否则 `#foo`、C 预处理器 `#include` 等会被误判为标题。
        // level 个 '#' 都是 ASCII 单字节，故 `trimmed[level..]` 字节下标安全。
        let after = &trimmed[level..];
        if !after.is_empty() && !after.starts_with(' ') && !after.starts_with('\t') {
            continue;
        }
        // 跳过 "###" 后空格
        let rest = after.trim_start();
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

#[cfg(test)]
mod m38_tests {
    use super::*;

    #[test]
    fn hash_without_space_is_not_heading() {
        // M38 回归：无空格的 #foo 不是标题
        let entries = parse_toc("#tag\n#include <stdio.h>\n#### too deep\n");
        assert!(entries.is_empty(), "无空格 # 行不应成为大纲条目: {:?}", entries);
    }

    #[test]
    fn real_headings_still_parsed() {
        let entries = parse_toc("# A\ntext\n## B\ntext\n### C\n");
        assert_eq!(entries.len(), 3);
        assert_eq!(entries[0].text, "A");
        assert_eq!(entries[1].text, "B");
        assert_eq!(entries[2].text, "C");
        assert_eq!(entries[2].line, 4);
    }
}
