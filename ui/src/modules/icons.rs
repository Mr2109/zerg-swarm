//! 图标辅助（P3——iconflow 统一封装——14 包 34 TTF——MIT——v2.6 开源干净）
//!
//! 用法：`ui.button(icon_text("plus"))`；带字号/图标字体族用 `ui.label(icon_rt("trash", 14.0))`
//! 主力风格：Phosphor（6 字重现代）——导航/线条用 Lucide
//! 替换 emoji 缺字清单（2026-08-29 实证 NotoEmoji 缺）：⚙️→settings 🖥️→monitor 🗑️→trash ✏️→edit 🟢→status

use eframe::egui::{self, FontData, FontDefinitions, FontFamily, FontId, RichText};

/// 注册 iconflow 全部字体（追加到现有 FontDefinitions——保中文字体——egui 0.36 API）
pub fn install_iconflow_fonts(definitions: &mut FontDefinitions) {
    let fallback: Vec<String> = definitions.font_data.keys().cloned().collect();
    for font in iconflow::fonts() {
        definitions.font_data.insert(
            font.family.to_string(),
            std::sync::Arc::new(FontData::from_static(font.bytes)),
        );
        let fam = definitions
            .families
            .entry(FontFamily::Name(font.family.into()))
            .or_default();
        fam.insert(0, font.family.to_string());
        for fb in &fallback {
            if fb != &font.family {
                fam.push(fb.clone());
            }
        }
        // P3 关键：图标字体加入 Proportional/Monospace fallback（尾部）
        // → PUA 字形字符（icon_text 返回）可混合中文渲染（CJK 缺字 → fallback 图标字体）
        for family in [FontFamily::Proportional, FontFamily::Monospace] {
            let fam = definitions.families.entry(family).or_default();
            if !fam.iter().any(|f| f == &font.family) {
                fam.push(font.family.to_string());
            }
        }
    }
}

/// 取图标字形（默认 Phosphor Regular——回退 Lucide——再回退原文）
///
/// M29(2026-09-10 审计)：返回的是**纯字形 `String`，不含字号信息**——字号由调用方
/// （`RichText`/`TextFormat`）决定。原 `icon_text_n(name, size)` 的 `size` 参数从未被
/// 使用（`icon_glyph` 里是 `_size`），属功能沉默失效，已删除该函数与参数。
/// 需要"图标自带字体族 + 指定字号"请改用 [`icon_rt`]。
pub fn icon_text(name: &str) -> String {
    icon_glyph(iconflow::Pack::Phosphor, name)
        .or_else(|| icon_glyph(iconflow::Pack::Lucide, name))
        .unwrap_or_else(|| {
            // M30: 拼错/未收录的图标名不再静默回退为文本——告警一次
            warn_missing_icon(name);
            name.to_string()
        })
}

/// 解析 codepoint → 字形字符串
fn icon_glyph(pack: iconflow::Pack, name: &str) -> Option<String> {
    let r = iconflow::try_icon(pack, name, iconflow::Style::Regular, iconflow::Size::Regular).ok()?;
    Some(char::from_u32(r.codepoint)?.to_string())
}

/// M30(2026-09-10 审计): 图标名未收录时一次性告警（按名去重——避免每帧刷屏）
/// 用于集中发现拼错的图标名（`#foo` 之类会被 iconflow 拒绝），便于替换为正确名。
fn warn_missing_icon(name: &str) {
    use std::collections::HashSet;
    use std::sync::{Mutex, OnceLock};
    static SEEN: OnceLock<Mutex<HashSet<String>>> = OnceLock::new();
    let seen = SEEN.get_or_init(|| Mutex::new(HashSet::new()));
    if let Ok(mut s) = seen.lock() {
        if s.insert(name.to_string()) {
            eprintln!("[icons] unknown icon name (falling back to text): {}", name);
        }
    }
}

/// 图标 RichText（默认字号 14——按钮/标签用）
pub fn icon_rt(name: &str, size: f32) -> RichText {
    // 取图标所在字体族——渲染时用该字体
    if let Some(r) = iconflow::try_icon(
        iconflow::Pack::Phosphor,
        name,
        iconflow::Style::Regular,
        iconflow::Size::Regular,
    )
    .ok()
    .or_else(|| {
        iconflow::try_icon(
            iconflow::Pack::Lucide,
            name,
            iconflow::Style::Regular,
            iconflow::Size::Regular,
        )
        .ok()
    }) {
        let glyph = char::from_u32(r.codepoint).unwrap_or('?');
        return RichText::new(glyph.to_string())
            .font(FontId::new(size, FontFamily::Name(r.family.into())));
    }
    // M30: 未收录图标名回退为文本时告警一次（便于发现遗漏/拼错）
    warn_missing_icon(name);
    RichText::new(name.to_string()).size(size)
}
