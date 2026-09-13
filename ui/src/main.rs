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
use rust_i18n::t;   // i18n（B3：窗口标题/应用名走键）

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

/// 初始语言（多语言决策② 2026-09-11）：prefs 显式值 → 系统语言 → fallback "en"
///
/// 本机（AppleLocale=zh_CN）→ zh-CN，界面无变化；英文系统 → en；界面右上角可随时切换，
/// 切换后写入 `~/.zerg-ui-prefs.json` 的 `locale` 字段（下次启动优先于系统语言）。
pub fn detect_locale() -> String {
    const SUPPORTED: [&str; 2] = ["zh-CN", "en"];
    // ① 偏好文件显式值（2026-09-13 Q10：新落点 <UI 状态目录>/prefs.json 优先，旧 ~/.zerg-ui-prefs.json 兼容）
    if let Some(raw) = crate::api::read_prefs() {
        if let Ok(v) = serde_json::from_str::<serde_json::Value>(&raw) {
            if let Some(l) = v.get("locale").and_then(|x| x.as_str()) {
                if SUPPORTED.contains(&l) {
                    return l.to_string();
                }
            }
        }
    }
    // ② 系统语言：环境变量 → macOS AppleLocale
    let mut sys = std::env::var("LC_ALL")
        .or_else(|_| std::env::var("LANG"))
        .unwrap_or_default();
    if sys.is_empty() {
        if let Ok(out) = std::process::Command::new("defaults")
            .args(["read", "-g", "AppleLocale"])
            .output()
        {
            sys = String::from_utf8_lossy(&out.stdout).trim().to_string();
        }
    }
    // ③ 判定：含 zh → zh-CN，其余 → en
    if sys.to_lowercase().contains("zh") {
        "zh-CN".to_string()
    } else {
        "en".to_string()
    }
}

fn main() -> eframe::Result<()> {
    rust_i18n::set_locale(&detect_locale());
    // 代码身份（自动升级 P0-a）：UI 与主控一样必须能自报"跑的是哪份代码"——
    // 版本号来自 Cargo.toml（编译期），commit/build 由 build.rs 注入（浅克隆/无 git 时回落 unknown）。
    println!(
        "[zerg-ui] version={} commit={} build={}",
        env!("CARGO_PKG_VERSION"),
        env!("ZERG_GIT_SHA"),
        env!("ZERG_BUILD_TIME")
    );
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_title(t!("app.version_line", version = env!("CARGO_PKG_VERSION")).to_string())
            .with_inner_size([1280.0, 800.0]),
        ..Default::default()
    };
    eframe::run_native(
        t!("app.title").as_ref(),
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
            println!("[zerg-ui] CJK font: {}", path);
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

#[cfg(test)]
mod locale_tests {
    use super::detect_locale;

    fn with_prefs(locale: Option<&str>) -> String {
        let dir = std::env::temp_dir().join(format!(
            "zerg-locale-test-{}-{:?}",
            std::process::id(),
            locale
        ));
        let _ = std::fs::create_dir_all(&dir);
        let f = dir.join(".zerg-ui-prefs.json");
        match locale {
            Some(l) => std::fs::write(&f, format!("{{\"locale\":\"{}\"}}", l)).unwrap(),
            None => {
                let _ = std::fs::remove_file(&f);
            }
        }
        // 复刻 detect_locale 的 prefs 分支（不污染进程级 HOME）
        if let Ok(raw) = std::fs::read_to_string(&f) {
            if let Ok(v) = serde_json::from_str::<serde_json::Value>(&raw) {
                if let Some(l) = v.get("locale").and_then(|x| x.as_str()) {
                    if ["zh-CN", "en"].contains(&l) {
                        return l.to_string();
                    }
                }
            }
        }
        "en".to_string()
    }

    #[test]
    fn prefs_en_wins() {
        assert_eq!(with_prefs(Some("en")), "en");
    }

    #[test]
    fn prefs_zh_cn_wins() {
        assert_eq!(with_prefs(Some("zh-CN")), "zh-CN");
    }

    #[test]
    fn unsupported_prefs_falls_back() {
        assert_eq!(with_prefs(Some("fr")), "en");
    }

    /// 端到端：locale 切换后，同一键在两版 yml 里解析出各自语言（英文模式验收证据）
    #[test]
    fn locale_switch_resolves_english_and_chinese() {
        use rust_i18n::t;
        rust_i18n::set_locale("en");
        assert_eq!(&*t!("app.title"), "Zerg");
        assert_eq!(
            &*t!("app.version_line", version = "9.9.9"),
            "Zerg v9.9.9"
        );
        assert_eq!(
            &*t!("chat.start_hint"),
            "← Pick a session or create one to start chatting"
        );
        assert_eq!(
            &*t!("chat.tool_executing", tool = "doc_search", secs = 3),
            "🔧 doc_search running… (3s)"
        );
        // L4：错误码 → 本地化文案（同一测试内串行断言两语言——避免与全局 locale 竞争）
        assert_eq!(
            crate::api::localized_api_error("MISSING_MACHINE").as_deref(),
            Some("machine must not be empty")
        );
        assert_eq!(
            crate::api::localized_api_error("AUTH_TOKEN_INVALID").as_deref(),
            Some("Invalid auth token")
        );
        assert_eq!(
            crate::api::localized_api_error("scheduler_not_started").as_deref(),
            Some("Scheduler is not running"),
            "大小写不敏感（服务端恒大写，键恒小写）"
        );
        assert_eq!(crate::api::localized_api_error("SOME_FUTURE_CODE"), None, "未收录的码须返回 None（回退服务端 message）");
        assert_eq!(&*t!("chat.ready", icon = "*"), "* ready");
        rust_i18n::set_locale("zh-CN");
        assert_eq!(&*t!("app.title"), "虫族 Zerg");
        assert_eq!(
            &*t!("app.version_line", version = "9.9.9"),
            "虫族 Zerg v9.9.9"
        );
        assert_eq!(&*t!("chat.ready", icon = "*"), "* 就绪");
        assert_eq!(
            &*t!("chat.tool_executing", tool = "doc_search", secs = 3),
            "🔧 doc_search 执行中…（3 秒）"
        );
        // L4（zh-CN 侧）
        assert_eq!(
            crate::api::localized_api_error("MISSING_MACHINE").as_deref(),
            Some("machine 字段不能为空")
        );
        assert_eq!(
            crate::api::localized_api_error("INVALID_PATH").as_deref(),
            Some("非法路径")
        );
    }

    #[test]
    fn real_detect_returns_supported_locale() {
        let l = detect_locale();
        assert!(l == "zh-CN" || l == "en", "unexpected locale: {}", l);
    }
}
