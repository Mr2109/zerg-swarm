// 虫族 UI（v0.1 骨架——Rust + egui/eframe——一体化——主控显示层）
// 三栏布局: 侧栏导航 | 主区（Tab 视图）| 右栏详情
// 连主控（8580 API）——主控挂了全灰"离线"——5s 重连

mod api;
mod app;
mod modules; // v2.5.6 集装箱模块系统（Mr2109 2026-08-29——注册表+顶部导航）

// 多国语言（rust-i18n——默认中文——locales/ 目录）
// fallback 方向 = en（设计稿 §7-7 拍板 2026-09-11）：不支持的系统语言回落英文，
// 中文键本就全覆盖，中文用户不会撞到 fallback；启动仍显式 set_locale（L2 改为跟随系统）。
rust_i18n::i18n!("locales", fallback = "en");

/// 搜索 macOS 26+ 动态字体包里的 PingFang（AssetsV2——路径随系统更新变）
fn find_pingfang_assets() -> Option<String> {
    let base = "/System/Library/AssetsV2";
    let entries = std::fs::read_dir(base).ok()?;
    for e in entries.flatten() {
        let name = e.file_name().to_string_lossy().to_string();
        if !name.starts_with("com_apple_MobileAsset_Font") {
            continue;
        }
        if let Ok(assets) = std::fs::read_dir(e.path()) {
            for a in assets.flatten() {
                let p = a.path().join("AssetData/PingFang.ttc");
                if p.exists() {
                    return Some(p.to_string_lossy().to_string());
                }
            }
        }
    }
    None
}

fn main() -> eframe::Result<()> {
    // 默认中文（Mr2109——虫族母语）
    rust_i18n::set_locale("zh-CN");
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_title("虫族 Zerg v2.5.8")
            .with_inner_size([1280.0, 800.0]),
        ..Default::default()
    };
    eframe::run_native(
        "虫族 Zerg",
        options,
        Box::new(|cc| {
            // 加载中文字体（egui 默认字体不含中文——乱码修复）
            setup_fonts(&cc.egui_ctx);
            // P4-26 图片加载器（svg loader——数学公式 KaTeX SVG 渲染用）
            egui_extras::install_image_loaders(&cc.egui_ctx);
            Ok(Box::new(app::ZergApp::new()))
        }),
    )
}

/// 加载系统中文字体（macOS PingFang / Linux Noto / Windows 微软雅黑——跨平台）
/// v2.5.5 修复（2026-08-21 Mr2109发现——字体粗糙）: 中文字体放第一位（主字体）
/// 默认 egui 字体（Ubuntu-Light）渲染粗糙——PingFang/SF 原生字体清晰
/// 2026-08-31 修复（Mr2109：字体粗糙——根因 macOS 26 系统字体移 AssetsV2 动态包——旧路径失效 fallback 到 STHeiti Light）
fn setup_fonts(ctx: &egui::Context) {
    let mut fonts = egui::FontDefinitions::default();
    // 候选系统中文字体路径（跨平台——第一优先 = 主字体）
    let mut candidates = vec![
        // P4-28 PingFangSC 独立 TTF（fonttools 从 TTC 提取 face 3——TTC 集合第一个 face 是
        // PingFangHK（繁体）——ab_glyph 只读第一个 face——HK 标点全角居中（Mr2109: 标点在中间）——SC 标点偏下偏左）
        // 可选：用户自备字体（ZERG_FONT_PATH 指定），或 ~/Library/Fonts/PingFangSC.ttf
        std::env::var("ZERG_FONT_PATH").unwrap_or_default(),
        format!(
            "{}/Library/Fonts/PingFangSC.ttf",
            std::env::var("HOME").unwrap_or_default()
        ),
        // macOS（PingFang 优先——原生清晰）
        "/System/Library/Fonts/PingFang.ttc".to_string(), // 旧路径（macOS ≤15）
        // macOS 26+：系统字体在 AssetsV2 动态资产包（路径随系统更新变——动态查找）
        "/System/Library/Fonts/STHeiti Light.ttc".to_string(),
        "/System/Library/Fonts/Hiragino Sans GB.ttc".to_string(),
        // Linux
        "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc".to_string(),
        "/usr/share/fonts/truetype/wqy/wqy-microhei.ttc".to_string(),
        // Windows
        "C:\\Windows\\Fonts\\msyh.ttc".to_string(),
        "C:\\Windows\\Fonts\\simhei.ttf".to_string(),
    ];
    // macOS 26 动态字体包搜索（PingFang 优先插入候选第一位——但 SC TTF 保持第 0 位——P4-28）
    if let Some(pf) = find_pingfang_assets() {
        candidates.insert(1, pf);
    }
    let mut loaded = false;
    for path in candidates {
        if let Ok(bytes) = std::fs::read(&path) {
            fonts
                .font_data
                .insert("cjk".to_owned(), std::sync::Arc::new(egui::FontData::from_owned(bytes)));
            // 中文字体放字体族【第一位】（主字体——拉丁+中文都用它——替代默认 Ubuntu 粗糙字体）
            for family in [egui::FontFamily::Proportional, egui::FontFamily::Monospace] {
                let fam = fonts.families.entry(family).or_default();
                fam.insert(0, "cjk".to_owned());
            }
            loaded = true;
            println!("[zerg-ui] 中文字体: {}", path);
            break;
        }
    }
    // 找不到系统字体时——用 macOS 通用字体（SF/Helvetica——拉丁清晰）
    if !loaded {
        for path in ["/System/Library/Fonts/SFNS.ttf", "/System/Library/Fonts/Helvetica.ttc"] {
            if let Ok(bytes) = std::fs::read(path) {
                fonts
                    .font_data
                    .insert("latin".to_owned(), std::sync::Arc::new(egui::FontData::from_owned(bytes)));
                for family in [egui::FontFamily::Proportional, egui::FontFamily::Monospace] {
                    let fam = fonts.families.entry(family).or_default();
                    fam.insert(0, "latin".to_owned());
                }
                break;
            }
        }
    }
    // P3 图标字体（iconflow——14 包 34 TTF——追加注册——保中文字体）
    crate::modules::icons::install_iconflow_fonts(&mut fonts);
    // P3-2 等宽字体对齐 Hermes（Menlo——macOS 原生等宽——代码/diff 同款）
    for path in ["/System/Library/Fonts/Menlo.ttc", "/System/Library/Fonts/Monaco.ttf"] {
        if let Ok(bytes) = std::fs::read(path) {
            fonts
                .font_data
                .insert("mono".to_owned(), std::sync::Arc::new(egui::FontData::from_owned(bytes)));
            let fam = fonts.families.entry(egui::FontFamily::Monospace).or_default();
            fam.insert(0, "mono".to_owned());
            break;
        }
    }
    ctx.set_fonts(fonts);
    // v2.5.5 修复: 提高基础字号（14pt——Retina 上更清晰——默认 12pt 粗糙）
    // egui 0.36 API: global_style_mut（不是 set_style）
    ctx.global_style_mut(|style| {
        style.text_styles.insert(
            egui::TextStyle::Body,
            egui::FontId::new(14.0, egui::FontFamily::Proportional),
        );
        style.text_styles.insert(
            egui::TextStyle::Button,
            egui::FontId::new(14.0, egui::FontFamily::Proportional),
        );
        style.text_styles.insert(
            egui::TextStyle::Heading,
            egui::FontId::new(20.0, egui::FontFamily::Proportional),
        );
    });
}
