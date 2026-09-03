//! 图标辅助（P3——iconflow 统一封装——14 包 34 TTF——MIT——v2.6 开源干净）
//!
//! 用法：`ui.button(icon_text("plus"))` 或 `ui.label(icon_text_n("trash", 14.0))`
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
pub fn icon_text(name: &str) -> String {
    icon_text_n(name, 14.0)
}

/// 取图标字形（指定字号）
pub fn icon_text_n(name: &str, size: f32) -> String {
    icon_glyph(iconflow::Pack::Phosphor, name, size)
        .or_else(|| icon_glyph(iconflow::Pack::Lucide, name, size))
        .unwrap_or_else(|| name.to_string())
}

/// 解析 codepoint → 字形字符串
fn icon_glyph(pack: iconflow::Pack, name: &str, _size: f32) -> Option<String> {
    let r = iconflow::try_icon(pack, name, iconflow::Style::Regular, iconflow::Size::Regular).ok()?;
    Some(char::from_u32(r.codepoint)?.to_string())
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
    RichText::new(name.to_string()).size(size)
}
