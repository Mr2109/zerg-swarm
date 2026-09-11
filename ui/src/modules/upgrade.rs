//! Upgrade page (L2 of the auto-upgrade module, 2026-09-11).
//!
//! Design: the UI only **triggers and displays** — every judgement and the actual swap lives in
//! `scripts/zerg-upgrade.sh` (the six-stage core: plan → drain → swap → restart → verify → report).
//! This page therefore owns no upgrade logic of its own: it runs that script and shows its output,
//! so there is exactly one implementation of "how an upgrade happens".
//!
//! State lives in module statics (the page needs no field on the app struct), and the script runs on
//! a worker thread so the UI never blocks. Lines arrive over an mpsc channel and are drained per frame.

use egui::RichText;
use rust_i18n::t; // 文件级导入——否则 t!() 报 cannot find macro（B2a 同坑）
use std::io::{BufRead, BufReader};
use std::process::{Command, Stdio};
use std::sync::mpsc::{channel, Receiver};
use std::sync::Mutex;

static LOG: Mutex<Vec<String>> = Mutex::new(Vec::new());
static RX: Mutex<Option<Receiver<String>>> = Mutex::new(None);
static RUNNING: Mutex<bool> = Mutex::new(false);

const MAX_LINES: usize = 800;

fn push(line: String) {
    if let Ok(mut log) = LOG.lock() {
        log.push(line);
        if log.len() > MAX_LINES {
            let drop_n = log.len() - MAX_LINES;
            log.drain(0..drop_n);
        }
    }
}

/// Locate the repository root: prefer $ZERG_ROOT, else derive from the executable path
/// (the deployed layout is <repo>/bin/zerg-ui).
fn repo_root() -> Option<std::path::PathBuf> {
    if let Ok(r) = std::env::var("ZERG_ROOT") {
        if !r.is_empty() {
            return Some(std::path::PathBuf::from(r));
        }
    }
    let exe = std::env::current_exe().ok()?;
    let bin = exe.parent()?; // .../bin
    let root = bin.parent()?; // repository root
    Some(root.to_path_buf())
}

fn script_path() -> Option<std::path::PathBuf> {
    let p = repo_root()?.join("scripts").join("zerg-upgrade.sh");
    if p.exists() {
        Some(p)
    } else {
        None
    }
}

/// Run the upgrade script with the given flags on a worker thread, streaming stdout/stderr lines.
fn spawn(args: Vec<String>) {
    let Some(script) = script_path() else {
        push(String::from("[err] scripts/zerg-upgrade.sh not found (set ZERG_ROOT)"));
        return;
    };
    {
        let Ok(mut r) = RUNNING.lock() else { return };
        if *r {
            push(String::from("[err] an upgrade run is already in flight"));
            return;
        }
        *r = true;
    }
    let (tx, rx) = channel::<String>();
    if let Ok(mut slot) = RX.lock() {
        *slot = Some(rx);
    }
    std::thread::spawn(move || {
        let mut child = match Command::new("bash")
            .arg(&script)
            .args(&args)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
        {
            Ok(c) => c,
            Err(e) => {
                let _ = tx.send(format!("[err] spawn failed: {}", e));
                let _ = tx.send(String::from("__DONE__"));
                return;
            }
        };
        let out = child.stdout.take();
        let err = child.stderr.take();
        let tx_err = tx.clone();
        let h = std::thread::spawn(move || {
            if let Some(e) = err {
                for line in BufReader::new(e).lines().map_while(Result::ok) {
                    let _ = tx_err.send(format!("[stderr] {}", line));
                }
            }
        });
        if let Some(o) = out {
            for line in BufReader::new(o).lines().map_while(Result::ok) {
                let _ = tx.send(line);
            }
        }
        let code = child.wait().ok().and_then(|s| s.code()).unwrap_or(-1);
        let _ = h.join();
        let _ = tx.send(format!("[exit] {}", code));
        let _ = tx.send(String::from("__DONE__"));
    });
}

/// Drain the channel into the visible log (called once per frame).
fn pump() {
    let mut done = false;
    if let Ok(mut slot) = RX.lock() {
        if let Some(rx) = slot.as_ref() {
            while let Ok(line) = rx.try_recv() {
                if line == "__DONE__" {
                    done = true;
                    continue;
                }
                push(line);
            }
        }
        if done {
            *slot = None;
        }
    }
    if done {
        if let Ok(mut r) = RUNNING.lock() {
            *r = false;
        }
    }
}

fn running() -> bool {
    RUNNING.lock().map(|r| *r).unwrap_or(false)
}

/// Page entry point (called from the app's module dispatch).
pub fn ui(ui: &mut egui::Ui) {
    pump();
    let busy = running();

    ui.horizontal(|ui| {
        ui.heading(t!("upgrade.title").to_string());
        if busy {
            ui.label(RichText::new(t!("upgrade.running").to_string()).italics());
        } else {
            ui.weak(t!("upgrade.idle").to_string());
        }
    });
    ui.weak(t!("upgrade.hint").to_string());
    ui.add_space(6.0);

    ui.horizontal_wrapped(|ui| {
        ui.add_enabled_ui(!busy, |ui| {
            if ui.button(t!("upgrade.btn_status").to_string()).clicked() {
                spawn(vec!["--status".into()]);
            }
            if ui.button(t!("upgrade.btn_check").to_string()).clicked() {
                spawn(vec!["--check".into()]);
            }
            if ui.button(t!("upgrade.btn_plan").to_string()).clicked() {
                spawn(vec!["--plan".into()]);
            }
            if ui.button(t!("upgrade.btn_receipts").to_string()).clicked() {
                spawn(vec!["--receipts".into()]);
            }
        });
        ui.add_enabled_ui(!busy, |ui| {
            if ui
                .button(RichText::new(t!("upgrade.btn_apply").to_string()).strong())
                .clicked()
            {
                spawn(vec![String::from("--force")]);
            }
            if ui.button(t!("upgrade.btn_rollback").to_string()).clicked() {
                spawn(vec![String::from("--rollback")]);
            }
        });
        if ui.button(t!("upgrade.btn_clear").to_string()).clicked() {
            if let Ok(mut log) = LOG.lock() {
                log.clear();
            }
        }
    });

    ui.separator();
    egui::ScrollArea::vertical()
        .auto_shrink([false, false])
        .show(ui, |ui| {
            let lines = LOG.lock().map(|l| l.clone()).unwrap_or_default();
            if lines.is_empty() {
                ui.weak(t!("upgrade.empty").to_string());
            }
            for line in &lines {
                ui.label(RichText::new(line).monospace());
            }
        });
}
