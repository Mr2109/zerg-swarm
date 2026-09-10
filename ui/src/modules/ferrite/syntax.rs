//! 语法高亮（抄 Ferrite markdown/syntax.rs 精简——M3 F6）
//! syntect 代码块高亮——懒加载 SyntaxSet/ThemeSet（缓存——只初始化一次）

use eframe::egui::{Color32, RichText};
use syntect::easy::HighlightLines;
use syntect::highlighting::{Style, ThemeSet};
use syntect::parsing::SyntaxSet;
use syntect::util::LinesWithEndings;

use std::cell::RefCell;
use std::collections::HashMap;
use std::hash::{Hash, Hasher};
use std::sync::Arc;

/// M25(2026-09-10 审计): 高亮结果缓存——按 (lang, code) 内容哈希复用。
/// 原来每个代码块每帧都重跑 syntect 高亮（长文档多代码块时纯浪费，叠加 M06 放大）。
/// 缓存与预览 AST 解耦，不新增依赖；上限 64 块，超限整表清空（代码块数量有限，够用）。
pub type Highlighted = Arc<Vec<Vec<(Color32, String)>>>;

thread_local! {
    static HIGHLIGHT_CACHE: RefCell<HashMap<u64, Option<Highlighted>>> = RefCell::new(HashMap::new());
}

/// 语法集缓存（默认加载——含常见语言）
fn syntax_set() -> &'static SyntaxSet {
    use std::sync::OnceLock;
    static SET: OnceLock<SyntaxSet> = OnceLock::new();
    SET.get_or_init(SyntaxSet::load_defaults_newlines)
}

/// 主题集缓存（默认主题——base16-ocean.dark 深色）
fn theme_set() -> &'static ThemeSet {
    use std::sync::OnceLock;
    static SET: OnceLock<ThemeSet> = OnceLock::new();
    SET.get_or_init(ThemeSet::load_defaults)
}

/// syntect Style → egui Color32
fn style_to_color(style: &Style) -> Color32 {
    Color32::from_rgb(
        style.foreground.r,
        style.foreground.g,
        style.foreground.b,
    )
}

/// 高亮代码块——返回带颜色的行列表（每行 Vec<(Color32, String)>）
/// 语言未知/无高亮 → 返回 None（调用方用默认等宽渲染）。
/// M25: 结果按 (lang, code) 哈希缓存并返回 `Arc`——重复渲染不重算、不深拷贝。
pub fn highlight_code_block(code: &str, lang: &str) -> Option<Highlighted> {
    let mut hasher = std::collections::hash_map::DefaultHasher::new();
    lang.hash(&mut hasher);
    0u8.hash(&mut hasher); // 分隔符——避免 (lang,code) 拼接歧义
    code.hash(&mut hasher);
    let key = hasher.finish();

    if let Some(hit) = HIGHLIGHT_CACHE.with(|c| c.borrow().get(&key).cloned()) {
        return hit;
    }
    let computed = compute_highlight(code, lang).map(Arc::new);
    HIGHLIGHT_CACHE.with(|c| {
        let mut cache = c.borrow_mut();
        if cache.len() > 64 {
            cache.clear();
        }
        cache.insert(key, computed.clone());
    });
    computed
}

/// 实际高亮计算（无缓存——供 highlight_code_block 调用）
fn compute_highlight(code: &str, lang: &str) -> Option<Vec<Vec<(Color32, String)>>> {
    let ss = syntax_set();
    // 找语法（语言别名如 rust/rs/python/py——找不到返回 None）
    let syntax = if lang.is_empty() {
        ss.find_syntax_plain_text()
    } else {
        ss.find_syntax_by_token(lang)
            .or_else(|| ss.find_syntax_by_extension(lang))
            .unwrap_or_else(|| ss.find_syntax_plain_text())
    };
    if syntax.name == "Plain Text" {
        return None;
    }
    let ts = theme_set();
    let theme = &ts.themes["base16-ocean.dark"];
    let mut h = HighlightLines::new(syntax, theme);
    let mut out = Vec::new();
    for line in LinesWithEndings::from(code) {
        if let Ok(ranges) = h.highlight_line(line, ss) {
            let mut parts = Vec::new();
            for (style, text) in ranges {
                parts.push((style_to_color(&style), text.to_string()));
            }
            out.push(parts);
        }
    }
    Some(out)
}

/// 高亮代码块并渲染到 egui Ui（深色背景 + 着色文本）
pub fn render_code_block(ui: &mut egui::Ui, code: &str, lang: &str) {
    let bg = ui.visuals().extreme_bg_color;
    egui::Frame::new()
        .fill(bg)
        .corner_radius(4.0)
        .inner_margin(egui::Margin::same(8))
        .show(ui, |ui| {
            if !lang.is_empty() {
                ui.weak(lang);
            }
            // P2 diff 高亮（lang=diff 或代码含 diff 标记——Hermes diff-lines 借鉴）
            if is_diff_block(code, lang) {
                render_diff_lines(ui, code);
                return;
            }
            match highlight_code_block(code, lang) {
                Some(highlighted) => {
                    // 逐行渲染（每段不同颜色）——M25: highlighted 是 Arc，按引用遍历
                    for parts in highlighted.iter() {
                        ui.horizontal_wrapped(|ui| {
                            for (color, text) in parts.iter() {
                                ui.label(RichText::new(text.clone()).monospace().color(*color));
                            }
                        });
                    }
                }
                None => {
                    ui.monospace(code);
                }
            }
        });
}

/// 检测 diff 代码块（lang=diff 或 行首 +/- 且含 @@ hunk）
fn is_diff_block(code: &str, lang: &str) -> bool {
    if lang.eq_ignore_ascii_case("diff") {
        return true;
    }
    let lines: Vec<&str> = code.lines().collect();
    if lines.len() < 2 {
        return false;
    }
    let mut sign = 0;
    let mut hunk = false;
    for l in lines.iter().take(12) {
        let t = l.trim_start();
        if t.starts_with("+") || t.starts_with("-") {
            sign += 1;
        }
        if t.starts_with("@@") {
            hunk = true;
        }
    }
    sign >= 4 && hunk
}

/// diff 行级着色（+ 绿 / - 红 / @@ 蓝 / 头 弱化）
fn render_diff_lines(ui: &mut egui::Ui, code: &str) {
    for line in code.lines() {
        let t = line.trim_start();
        let (color, body): (egui::Color32, &str) = if t.starts_with("@@") {
            (egui::Color32::from_rgb(90, 160, 255), line)
        } else if t.starts_with('+') {
            (egui::Color32::from_rgb(120, 200, 120), line)
        } else if t.starts_with('-') {
            (egui::Color32::from_rgb(230, 130, 130), line)
        } else if t.starts_with("diff --git") || t.starts_with("index ") || t.starts_with("--- ") || t.starts_with("+++ ") {
            (ui.visuals().weak_text_color(), line)
        } else {
            (ui.visuals().text_color(), line)
        };
        ui.label(RichText::new(body).monospace().color(color));
    }
}
