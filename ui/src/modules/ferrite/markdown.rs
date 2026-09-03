//! Markdown 解析与渲染（抄 Ferrite parser 精简——M3 Ferrite 重写）
//! comrak 解析 → 遍历 AST → 渲染 egui widget（标题/段落/列表/代码块/引用/分隔线）

use comrak::nodes::{AstNode, NodeValue};
use comrak::{parse_document, Arena, Options};
use eframe::egui;

/// 渲染 markdown 文本到 egui Ui
pub fn render_markdown(ui: &mut egui::Ui, text: &str) {
    let arena = Arena::new();
    let mut options = Options::default();
    options.extension.autolink = true;
    options.extension.table = true;
    options.extension.strikethrough = true;
    options.render.unsafe_ = false;

    let root = parse_document(&arena, text, &options);
    render_node(ui, root);
}

/// 递归渲染 AST 节点
fn render_node<'a>(ui: &mut egui::Ui, node: &'a AstNode<'a>) {
    for child in node.children() {
        render_child(ui, child);
    }
}

fn render_child<'a>(ui: &mut egui::Ui, node: &'a AstNode<'a>) {
    match &node.data.borrow().value {
        NodeValue::Heading(heading) => {
            let text = collect_text(node);
            let size = match heading.level {
                1 => 24.0,
                2 => 20.0,
                3 => 17.0,
                _ => 15.0,
            };
            ui.add_space(4.0);
            ui.label(
                egui::RichText::new(text)
                    .size(size)
                    .strong()
                    .color(ui.visuals().strong_text_color()),
            );
            ui.add_space(2.0);
        }
        NodeValue::Paragraph => {
            // P4-22 段落单 galley 混合格式（LayoutJob——Strong/Emph/Code/Link 无缝混排——
            // 原 horizontal_wrapped 逐 label 有 8px 间距——句子中间格式切换字距异常=乱）
            let text_color = ui.visuals().text_color();
            let strong_color = ui.visuals().strong_text_color();
            let mut job = egui::text::LayoutJob::default();
            build_inline_job(node, &mut job, 14.0, text_color, strong_color);
            ui.label(job);
            // P4-21 段落间距 6px（Mr2109: 11px 太多——呼吸感适中）
            ui.add_space(6.0);
        }
        NodeValue::Text(t) => {
            let _ = t;
        }
        NodeValue::List(list) => {
            let mut index = 0usize;
            for item in node.children() {
                if let NodeValue::Item(_) = item.data.borrow().value {
                    let bullet = if list.list_type == comrak::nodes::ListType::Bullet {
                        "•".to_string()
                    } else {
                        index += 1;
                        format!("{}.", index)
                    };
                    let item_text = collect_text(item);
                    ui.horizontal(|ui| {
                        ui.label(egui::RichText::new(bullet).color(ui.visuals().weak_text_color()));
                        ui.label(egui::RichText::new(item_text).size(14.0));
                    });
                }
            }
            ui.add_space(2.0);
        }
        NodeValue::CodeBlock(code_block) => {
            let lang = code_block.info.clone();
            let code = code_block.literal.clone();
            // F6 代码块——syntect 语法高亮（深色背景 + 着色文本——Ferrite 同款）
            super::syntax::render_code_block(ui, &code, &lang);
            ui.add_space(2.0);
        }
        NodeValue::Code(c) => {
            ui.monospace(&c.literal);
        }
        NodeValue::Table(_table) => {
            // P4-18 表格渲染（表头加粗 + 斑马纹 + 网格线——Hermes markdown-table 借鉴）
            let mut rows: Vec<Vec<String>> = Vec::new();
            for tr in node.children() {
                if let NodeValue::TableRow(_) = tr.data.borrow().value {
                    let mut cells = Vec::new();
                    for td in tr.children() {
                        cells.push(collect_text(td));
                    }
                    rows.push(cells);
                }
            }
            if !rows.is_empty() {
                egui::Frame::new()
                    .fill(ui.visuals().faint_bg_color)
                    .stroke(ui.visuals().widgets.noninteractive.bg_stroke)
                    .corner_radius(6.0)
                    .inner_margin(egui::Margin::symmetric(8, 6))
                    .show(ui, |ui| {
                        egui::Grid::new(ui.next_auto_id())
                            .striped(true)
                            .min_col_width(60.0)
                            .spacing(egui::vec2(20.0, 6.0))
                            .show(ui, |ui| {
                                for (ri, row) in rows.iter().enumerate() {
                                    for cell in row {
                                        if ri == 0 {
                                            ui.label(egui::RichText::new(cell).strong());
                                        } else {
                                            ui.label(cell);
                                        }
                                    }
                                    ui.end_row();
                                }
                            });
                    });
                ui.add_space(6.0);
            }
        }
        NodeValue::BlockQuote => {
            let text = collect_text(node);
            egui::Frame::new()
                .fill(ui.visuals().faint_bg_color)
                .corner_radius(4.0)
                .inner_margin(egui::Margin::symmetric(8, 4))
                .show(ui, |ui| {
                    ui.label(egui::RichText::new(format!("❝ {}", text)).italics());
                });
            ui.add_space(2.0);
        }
        NodeValue::ThematicBreak => {
            ui.separator();
            ui.add_space(2.0);
        }
        NodeValue::SoftBreak | NodeValue::LineBreak => {
            // 段内换行
        }
        NodeValue::Emph => {
            let text = collect_text(node);
            ui.label(egui::RichText::new(text).italics());
        }
        NodeValue::Strong => {
            let text = collect_text(node);
            ui.label(egui::RichText::new(text).strong());
        }
        NodeValue::Link(link) => {
            let text = collect_text(node);
            ui.hyperlink_to(text, &link.url);
        }
        NodeValue::Item(_) => {
            // 由 List 处理
        }
        _ => {
            // 其他节点（表格等）——递归子节点兜底
            render_node(ui, node);
        }
    }
}

/// P4-22 段内内联渲染（LayoutJob 混合格式——单 galley 无缝混排——无 spacing 拆字问题）
/// Strong=strong_text_color（egui 0.36 strong 是颜色增强非字重——与 Heading 一致）
fn build_inline_job<'a>(
    node: &'a AstNode<'a>,
    job: &mut egui::text::LayoutJob,
    size: f32,
    text_color: egui::Color32,
    strong_color: egui::Color32,
) {
    for child in node.children() {
        match &child.data.borrow().value {
            NodeValue::Text(t) => {
                job.append(t, 0.0, plain_fmt(size, text_color));
            }
            NodeValue::Code(c) => {
                job.append(
                    &c.literal,
                    0.0,
                    egui::TextFormat {
                        font_id: egui::FontId::monospace(size - 1.0),
                        color: text_color,
                        background: egui::Color32::from_black_alpha(30),
                        ..Default::default()
                    },
                );
            }
            NodeValue::SoftBreak | NodeValue::LineBreak => {
                job.append(" ", 0.0, plain_fmt(size, text_color));
            }
            NodeValue::Strong => {
                let text = collect_text(child);
                job.append(&text, 0.0, plain_fmt(size, strong_color));
            }
            NodeValue::Emph => {
                let text = collect_text(child);
                job.append(
                    &text,
                    0.0,
                    egui::TextFormat {
                        font_id: egui::FontId::proportional(size),
                        color: text_color,
                        italics: true,
                        ..Default::default()
                    },
                );
            }
            NodeValue::Link(link) => {
                let text = collect_text(child);
                job.append(
                    &text,
                    0.0,
                    egui::TextFormat {
                        font_id: egui::FontId::proportional(size),
                        color: egui::Color32::from_rgb(90, 150, 230),
                        underline: egui::Stroke::new(1.0, egui::Color32::from_rgb(90, 150, 230)),
                        ..Default::default()
                    },
                );
                let _ = link;
            }
            _ => {
                build_inline_job(child, job, size, text_color, strong_color);
            }
        }
    }
}

/// 普通文本格式
fn plain_fmt(size: f32, color: egui::Color32) -> egui::TextFormat {
    egui::TextFormat {
        font_id: egui::FontId::proportional(size),
        color,
        ..Default::default()
    }
}

/// 收集节点下所有文本（拼接）
fn collect_text<'a>(node: &'a AstNode<'a>) -> String {
    let mut out = String::new();
    collect_text_into(node, &mut out);
    out
}

fn collect_text_into<'a>(node: &'a AstNode<'a>, out: &mut String) {
    match &node.data.borrow().value {
        NodeValue::Text(t) => out.push_str(t),
        NodeValue::Code(c) => out.push_str(&c.literal),
        NodeValue::SoftBreak | NodeValue::LineBreak => out.push(' '),
        NodeValue::Strong | NodeValue::Emph => {
            for c in node.children() {
                collect_text_into(c, out);
            }
        }
        NodeValue::Link(l) => {
            for c in node.children() {
                collect_text_into(c, out);
            }
            let _ = l;
        }
        _ => {
            for c in node.children() {
                collect_text_into(c, out);
            }
        }
    }
}
