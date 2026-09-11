//! 模型登记库页（模型库——登记表；设计见 docs/01-设计/设计-模型库与对话自动调入-20260911.md）
//!
//! 本批只做**列表 + 卡片 + 空态**（不做视图切换 / 工程视图 / 自动路由——那是后续批）。
//! 数据源：GET /api/models/registry（只读快照，与其它 /api 同一套令牌中间件）。
//!
//! 三个界面状态各自独立处理（都不得白屏、不得 panic）：
//! - 拉取中 → 转圈 + 文案；
//! - 失败   → 可读错误 + 「重试」按钮（页面内自愈，不重启 UI）；
//! - 空目录 → 友好空态 + 一行「怎么加模型」（zerg-model probe --store）。
//! 坏记录（errors>0 / 带 error 文案）仍照常列出，用同情的语气说明——坏记录不能弄坏整页。
//!
//! 数据拉取在 tokio runtime 内异步执行（见 api::fetch_model_registry_async），
//! UI 线程每帧只做一次「结果回填」的轮询（pump），从不阻塞。

use egui::RichText;
use rust_i18n::t; // 文件级导入——否则 t!() 报 cannot find macro（B2a 同坑）
use crate::api::{self, SharedResult};
use crate::modules::icons::icon_text;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;

/// 已就绪的数据（Ok = 快照；Err = 可读错误文案）。None = 尚未拿到任何结果。
static DATA: Mutex<Option<Result<serde_json::Value, String>>> = Mutex::new(None);
/// 在途请求句柄（结果由 pump() 每帧取回一次）
static PENDING: Mutex<Option<SharedResult<serde_json::Value>>> = Mutex::new(None);
/// 是否有请求在途（防重复发起——含重试与自动首次拉取）
static LOADING: AtomicBool = AtomicBool::new(false);

/// 数据是否已就绪（首次进页自动拉取；失败后由「重试」手动触发，不无限自动重试）
fn has_data() -> bool {
    DATA.lock().map(|d| d.is_some()).unwrap_or(false)
}

fn loading() -> bool {
    LOADING.load(Ordering::SeqCst)
}

/// 发起一次异步拉取（已在途则忽略——幂等）
fn start_fetch() {
    if LOADING.swap(true, Ordering::SeqCst) {
        return; // 已在途
    }
    let handle = api::fetch_model_registry_async();
    if let Ok(mut p) = PENDING.lock() {
        *p = Some(handle);
    }
}

/// 清掉旧结果（含错误态）后重新拉取——「重试」按钮
fn retry() {
    if let Ok(mut d) = DATA.lock() {
        *d = None;
    }
    start_fetch();
}

/// 每帧泵一次：把在途结果取回 DATA（只取一次，不阻塞——锁只在结果已就绪时短暂持有）
fn pump() {
    let mut incoming: Option<Result<serde_json::Value, String>> = None;
    if let Ok(mut p) = PENDING.lock() {
        if let Some(h) = p.as_ref() {
            let mut g = h.lock().unwrap_or_else(|e| e.into_inner());
            if g.is_some() {
                incoming = g.take(); // take：结果只回填一次
            }
        }
        if incoming.is_some() {
            *p = None;
        }
    }
    if let Some(r) = incoming {
        if let Ok(mut d) = DATA.lock() {
            *d = Some(r);
        }
        LOADING.store(false, Ordering::SeqCst);
    }
}

// ───────────────────────── 纯函数（可单测，不依赖 egui） ─────────────────────────

/// 显示名：优先 name，缺则退回 id（缺 id 再退回占位符）——「名字是别名，id 是身份」
fn display_name(rec: &serde_json::Value) -> String {
    let name = rec.get("name").and_then(|x| x.as_str()).unwrap_or("").trim();
    if !name.is_empty() {
        return name.to_string();
    }
    let id = rec.get("id").and_then(|x| x.as_str()).unwrap_or("").trim();
    if !id.is_empty() {
        return id.to_string();
    }
    String::from("?")
}

/// 许可徽标：commercial → (i18n 键, 颜色)。
/// 取值口径见设计 §1.4：yes 绿/可商用、no 红/非商用、revenue_gated 黄/营收门槛、其余 灰/待核。
fn license_badge(commercial: &str) -> (&'static str, egui::Color32) {
    match commercial {
        "yes" => ("mreg.license_yes", egui::Color32::from_rgb(80, 200, 120)),
        "no" => ("mreg.license_no", egui::Color32::from_rgb(220, 80, 80)),
        "revenue_gated" => ("mreg.license_revenue_gated", egui::Color32::from_rgb(240, 200, 80)),
        _ => ("mreg.license_unknown", egui::Color32::from_rgb(150, 150, 150)),
    }
}

/// 人类可读体积（二进制单位——与本机习惯一致）
fn human_size(bytes: i64) -> String {
    let b = bytes.max(0) as f64;
    const KB: f64 = 1024.0;
    if b >= KB * KB * KB {
        format!("{:.2} GB", b / (KB * KB * KB))
    } else if b >= KB * KB {
        format!("{:.1} MB", b / (KB * KB))
    } else if b >= KB {
        format!("{:.1} KB", b / KB)
    } else {
        format!("{} B", bytes.max(0))
    }
}

/// 建材汇总：文件数 + 合计字节（files 缺失/非数组 = 0）
fn file_summary(files: Option<&Vec<serde_json::Value>>) -> (usize, i64) {
    match files {
        Some(arr) => {
            let total = arr
                .iter()
                .map(|f| f.get("size").and_then(|x| x.as_i64()).unwrap_or(0))
                .sum();
            (arr.len(), total)
        }
        None => (0, 0),
    }
}

// ───────────────────────── 页面 ─────────────────────────

/// 页面入口（app.rs 的 main_view 分派调用——不进 App 结构体，照 upgrade.rs 的既有写法）
pub fn ui(ui: &mut egui::Ui) {
    pump();
    // 首次进页自动拉取；失败后停在错误态，等用户点「重试」
    if !has_data() && !loading() {
        start_fetch();
    }

    ui.horizontal(|ui| {
        ui.heading(format!("{} {}", icon_text("package"), t!("mreg.title")));
        if loading() {
            ui.label(RichText::new(t!("mreg.loading").to_string()).italics());
        }
        ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
            if ui.button(t!("mreg.refresh").to_string()).clicked() {
                retry();
            }
        });
    });
    ui.weak(t!("mreg.hint").to_string());
    ui.add_space(6.0);
    ui.separator();

    // 三态分派：加载中 / 失败（可重试） / 有数据
    let data = DATA.lock().map(|d| d.clone()).unwrap_or(None);
    match data {
        None => {
            ui.add_space(24.0);
            ui.vertical_centered(|ui| {
                ui.spinner();
                ui.weak(t!("mreg.loading").to_string());
            });
        }
        Some(Err(e)) => {
            ui.add_space(16.0);
            ui.colored_label(
                egui::Color32::from_rgb(220, 80, 80),
                format!("{} {}", icon_text("warning"), t!("mreg.error_title")),
            );
            ui.label(RichText::new(e).monospace());
            ui.add_space(8.0);
            if ui.button(t!("mreg.retry").to_string()).clicked() {
                retry();
            }
        }
        Some(Ok(v)) => render_registry(ui, &v),
    }
}

/// 有数据：摘要 + 空态 / 卡片列表
fn render_registry(ui: &mut egui::Ui, v: &serde_json::Value) {
    let count = v.get("count").and_then(|c| c.as_u64()).unwrap_or(0);
    let bad = v.get("bad_records").and_then(|c| c.as_u64()).unwrap_or(0);
    let root = v.get("root").and_then(|c| c.as_str()).unwrap_or("");
    let records = v.get("records").and_then(|r| r.as_array());

    ui.horizontal_wrapped(|ui| {
        ui.label(t!("mreg.count", count = count, bad = bad).to_string());
        if !root.is_empty() {
            ui.weak(RichText::new(root).monospace());
        }
    });
    if bad > 0 {
        ui.colored_label(
            egui::Color32::from_rgb(240, 200, 80),
            t!("mreg.bad_banner", n = bad).to_string(),
        );
    }
    ui.add_space(6.0);

    let empty = records.map(|r| r.is_empty()).unwrap_or(true);
    if empty {
        // 空态：友好提示 + 一行「怎么加模型」（引导到 zerg-model probe --store）
        ui.add_space(28.0);
        ui.vertical_centered(|ui| {
            ui.heading(t!("mreg.empty_title").to_string());
            ui.add_space(6.0);
            ui.label(t!("mreg.empty_hint").to_string());
        });
        return;
    }

    egui::ScrollArea::vertical()
        .auto_shrink([false, false])
        .show(ui, |ui| {
            for rec in records.unwrap() {
                card(ui, rec);
                ui.add_space(6.0);
            }
        });
}

/// 单个模型卡片
fn card(ui: &mut egui::Ui, rec: &serde_json::Value) {
    let id = rec.get("id").and_then(|x| x.as_str()).unwrap_or("?");
    let version = rec.get("version").and_then(|x| x.as_str()).unwrap_or("");
    let name = display_name(rec);
    let commercial = rec.get("commercial").and_then(|x| x.as_str()).unwrap_or("");
    let ctx = rec.get("context_window").and_then(|x| x.as_i64()).unwrap_or(0);
    let errors = rec.get("errors").and_then(|x| x.as_u64()).unwrap_or(0);
    let warns = rec.get("warns").and_then(|x| x.as_u64()).unwrap_or(0);
    let default_eligible = rec.get("default_eligible").and_then(|x| x.as_bool()).unwrap_or(false);
    let err_msg = rec.get("error").and_then(|x| x.as_str()).unwrap_or("");
    let files = rec.get("files").and_then(|x| x.as_array());
    let caps = rec.get("capabilities").and_then(|x| x.as_array());

    let (lk, color) = license_badge(commercial);
    let (file_count, file_bytes) = file_summary(files);

    ui.group(|ui| {
        // 第一行：显示名 + id + 版本 + 许可徽标 + 默认资格
        ui.horizontal_wrapped(|ui| {
            ui.strong(name.as_str());
            if name != id {
                ui.weak(RichText::new(id).monospace());
            }
            if !version.is_empty() {
                ui.weak(RichText::new(format!("v{}", version)).monospace());
            }
            ui.colored_label(color, format!("● {}", t!(lk)));
            if default_eligible {
                ui.colored_label(
                    egui::Color32::from_rgb(80, 200, 120),
                    format!("✓ {}", t!("mreg.default_ok")),
                );
            } else {
                ui.weak(t!("mreg.default_no").to_string());
            }
        });

        // 第二行：能力标签（capabilities[].name——必要时带 source 小字）
        ui.horizontal_wrapped(|ui| {
            match caps {
                Some(arr) if !arr.is_empty() => {
                    for c in arr {
                        let cn = c.get("name").and_then(|x| x.as_str()).unwrap_or("");
                        if cn.is_empty() {
                            continue;
                        }
                        let src = c.get("source").and_then(|x| x.as_str()).unwrap_or("");
                        let label = if src.is_empty() {
                            cn.to_string()
                        } else {
                            t!("mreg.cap_tag", name = cn, src = src).to_string()
                        };
                        ui.label(RichText::new(label).small());
                    }
                }
                _ => {
                    ui.weak(t!("mreg.no_caps").to_string());
                }
            }
        });

        // 第三行：建材（文件数 + 合计体积） + 上下文窗口
        ui.horizontal_wrapped(|ui| {
            ui.label(t!("mreg.files", count = file_count, size = human_size(file_bytes)).to_string());
            if ctx > 0 {
                ui.label(t!("mreg.context", ctx = ctx).to_string());
            } else {
                ui.weak(t!("mreg.context_unknown").to_string());
            }
        });

        // 第四行：错误 / 告警计数（>0 醒目）；坏记录额外给同情式说明 + 原始原因
        ui.horizontal_wrapped(|ui| {
            if errors > 0 {
                ui.colored_label(
                    egui::Color32::from_rgb(220, 80, 80),
                    format!("{} {}", icon_text("warning"), t!("mreg.errors", n = errors)),
                );
            }
            if warns > 0 {
                ui.colored_label(
                    egui::Color32::from_rgb(240, 200, 80),
                    t!("mreg.warns", n = warns).to_string(),
                );
            }
        });
        if !err_msg.is_empty() {
            ui.colored_label(
                egui::Color32::from_rgb(230, 150, 90),
                t!("mreg.bad_hint").to_string(),
            );
            ui.label(RichText::new(err_msg).monospace().small());
        }
    });
}

#[cfg(test)]
mod tests {
    use super::{display_name, file_summary, human_size, license_badge};
    use serde_json::json;

    /// 许可徽标取值映射（设计 §1.4 的四色口径）
    #[test]
    fn test_license_badge_mapping() {
        assert_eq!(license_badge("yes").0, "mreg.license_yes");
        assert_eq!(license_badge("no").0, "mreg.license_no");
        assert_eq!(license_badge("revenue_gated").0, "mreg.license_revenue_gated");
        assert_eq!(license_badge("unknown").0, "mreg.license_unknown");
        // 空/陌生取值一律按「待核」——绝不默认成可商用
        assert_eq!(license_badge("").0, "mreg.license_unknown");
        assert_eq!(license_badge("whatever").0, "mreg.license_unknown");
    }

    /// 显示名：缺 name 退回 id，两者都缺退回占位符
    #[test]
    fn test_display_name_fallback() {
        assert_eq!(display_name(&json!({"id": "m1", "name": "好模型"})), "好模型");
        assert_eq!(display_name(&json!({"id": "m1", "name": ""})), "m1");
        assert_eq!(display_name(&json!({"id": "m1"})), "m1");
        assert_eq!(display_name(&json!({})), "?");
    }

    /// 体积格式化（二进制单位）
    #[test]
    fn test_human_size() {
        assert_eq!(human_size(0), "0 B");
        assert_eq!(human_size(512), "512 B");
        assert_eq!(human_size(1024), "1.0 KB");
        assert_eq!(human_size(1536), "1.5 KB");
        assert_eq!(human_size(1024 * 1024), "1.0 MB");
        assert_eq!(human_size(1024 * 1024 * 1024), "1.00 GB");
        // 负值（脏数据）不 panic——归零
        assert_eq!(human_size(-5), "0 B");
    }

    /// 建材汇总：数量与合计；缺字段/非数组归零
    #[test]
    fn test_file_summary() {
        let files = vec![
            json!({"role": "weights", "size": 1000}),
            json!({"role": "mmproj", "size": 500}),
        ];
        assert_eq!(file_summary(Some(&files)), (2, 1500));
        // size 缺失按 0 计，不 panic
        let partial = vec![json!({"role": "weights"})];
        assert_eq!(file_summary(Some(&partial)), (1, 0));
        assert_eq!(file_summary(None), (0, 0));
    }

    /// 渲染路径不 panic（空态 / 好记录+坏记录混合）——坏记录不得弄坏整页。
    /// 无头 egui（0.36 入口 run_ui；每帧 clear 纹理增量，否则 drop panic——照 M06 基准测试写法）
    #[test]
    fn test_render_paths_do_not_panic() {
        let empty = json!({"count": 0, "bad_records": 0, "root": "/tmp/none", "records": []});
        // 完整记录：许可/能力/建材/上下文/默认资格俱全
        let good = json!({
            "id": "qwen3-vl-8b", "version": "2026-09-01", "name": "Qwen3-VL-8B",
            "commercial": "yes",
            "capabilities": [{"name": "vision", "value": true, "source": "probed"},
                             {"name": "tools", "value": true, "source": "declared"}],
            "files": [{"role": "weights", "name": "a.gguf", "sha256": "ab", "size": 2048},
                      {"role": "mmproj", "name": "m.gguf", "sha256": "cd", "size": 512}],
            "context_window": 32768, "engine_recipes": ["llama.cpp"],
            "errors": 0, "warns": 1, "default_eligible": true
        });
        // 坏记录：只有 id/version + errors/error（其余字段缺省——照后端坏记录形状）；
        // 许可缺失/非口径取值也必须退化成「待核」而不是崩
        let bad = json!({"id": "broken", "version": "?", "errors": 1,
                         "error": "unexpected end of JSON input"});
        let mixed = json!({"count": 2, "bad_records": 1, "root": "/root",
                           "records": [good, bad]});

        let ctx = egui::Context::default();
        for payload in [&empty, &mixed] {
            let mut out = ctx.run_ui(Default::default(), |ui| super::render_registry(ui, payload));
            out.textures_delta.clear();
        }
    }
}
