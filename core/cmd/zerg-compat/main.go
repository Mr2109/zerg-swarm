// zerg-compat — B7 / G10：跨版本状态文件的兼容层命令行。
//
// 用途：把「哪些状态文件带 schema 版本号、现在是什么版本、要不要迁移」变成可以直接看的输出，
// 并提供一个**幂等、可重放**的一次性迁移入口（主控启动时也会对已接线的文件自动迁移）。
//
// 子命令：
//
//	zerg-compat list                    列出清单（文件→当前 schema→最低可读→信封→归属）
//	zerg-compat check                   只读体检：逐文件报现状；有待迁移 ⇒ rc=2，有更高版本 ⇒ rc=3
//	zerg-compat migrate [--dry-run]     执行一次性迁移（幂等；失败 rc=1）
//	zerg-compat manifest                打印内嵌清单原文（CI 门禁的对照面）
//
// 目录解析：ZERG_STATE_DIR / ZERG_RECEIPTS_DIR（或 --state-dir / --receipts-dir）。
// 本命令会**打印它将要操作的目录**——不写死私有路径，也不猜用户的家目录。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Mr2109/zerg-swarm/core/internal/compat"
)

const (
	exitOK      = 0
	exitFailed  = 1 // 迁移/校验失败（留痕已在日志里）
	exitPending = 2 // check：存在待迁移文件
	exitFuture  = 3 // check：存在高于本机认知的 schema —— 请升级本机二进制
	exitUsage   = 4
)

// 退码唯一真源 = zerg help exit-codes（契约 §三）——**本件是组件自有码空间**（进程语义与命令面不同，§6.1）。
// 撞号处逐条登记在《开工记录-批A-20260920.md》T-04 节：本件 `4 = 用法错误` 与命令面 `4 = 未认证` 撞号；
// **不许就地改数字** —— 本件自己的 `2` 已被「有待迁移文件」占了，改过去是**在自己家里造第二个两义**。
// 值面对齐要先给自有语义重新取号（属 `P-013` ⑨ 的拍板 · 批 C 的 T-26 执行）；命令面只把它**原样转出**。

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return exitUsage
	}
	cmd := args[0]
	fs := flag.NewFlagSet("zerg-compat "+cmd, flag.ContinueOnError)
	stateDir := fs.String("state-dir", "", "状态目录（默认 ZERG_STATE_DIR → ~/.zerg/state）")
	receiptsDir := fs.String("receipts-dir", "", "升级回执目录（默认 ZERG_RECEIPTS_DIR → ~/.zerg/update_receipts）")
	dryRun := fs.Bool("dry-run", false, "只报告将要迁移的文件，不落盘")
	asJSON := fs.Bool("json", false, "以 JSON 输出（机器可读）")
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}

	cfg := compat.DefaultConfig()
	if *stateDir != "" {
		cfg.Dirs.State = *stateDir
	}
	if *receiptsDir != "" {
		cfg.Dirs.Receipts = *receiptsDir
	}

	switch cmd {
	case "list":
		return cmdList(cfg, *asJSON)
	case "check":
		return cmdCheck(cfg, *asJSON)
	case "migrate":
		return cmdMigrate(cfg, *dryRun, *asJSON)
	case "manifest":
		return cmdManifest()
	case "help", "-h", "--help":
		usage()
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "未知子命令 %q\n\n", cmd)
		usage()
		return exitUsage
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `zerg-compat —— 跨版本状态文件兼容层（B7 / G10）

用法:
  zerg-compat list      [--state-dir D] [--receipts-dir D] [--json]
  zerg-compat check     [--state-dir D] [--receipts-dir D] [--json]
  zerg-compat migrate   [--state-dir D] [--receipts-dir D] [--dry-run] [--json]
  zerg-compat manifest

退出码: 0 就绪/成功 · 1 迁移或校验失败 · 2 有待迁移文件 · 3 出现更高版本 schema（请升级本机二进制） · 4 用法错误
  ↑ 本件是**组件自有码空间**：退码唯一真源 = zerg help exit-codes（契约 §三）；
    与命令面表撞号处逐条登记在《开工记录-批A-20260920.md》T-04 节（值面对齐 = P-013 ⑨ 拍板 · 批 C 的 T-26）。
`)
}

func cmdList(cfg compat.Config, asJSON bool) int {
	entries := compat.Entries()
	if asJSON {
		return emitJSON(entries)
	}
	fmt.Printf("状态目录: %s\n回执目录: %s\n\n", cfg.Dirs.State, cfg.Dirs.Receipts)
	fmt.Printf("%-20s %-28s %-7s %-7s %-8s %-7s %s\n", "名称", "文件", "当前", "最低读", "信封", "盖版本", "归属")
	for _, e := range entries {
		fmt.Printf("%-20s %-28s %-7d %-7d %-8s %-7s %s\n",
			e.Name, e.File, e.CurrentSchema, e.MinReadable, e.Envelope, e.StampsSchema, e.Owner)
	}
	fmt.Printf("\n（明确不登记：%d 项——理由见 core/internal/compat/compat.json 的 excluded）\n", len(compat.MustManifest().Excluded))
	return exitOK
}

func cmdCheck(cfg compat.Config, asJSON bool) int {
	statuses := compat.StatusAll(cfg)
	if asJSON {
		return emitJSONWithExit(statuses, exitCodeFor(statuses))
	}
	fmt.Printf("状态目录: %s\n回执目录: %s\n\n", cfg.Dirs.State, cfg.Dirs.Receipts)
	fmt.Printf("%-20s %-52s %-9s %s\n", "名称", "文件", "结论", "磁盘版本")
	for _, st := range statuses {
		ver := "—"
		if st.OnDisk >= 0 {
			ver = fmt.Sprint(st.OnDisk)
		}
		line := fmt.Sprintf("%-20s %-52s %-9s %s", st.Name, st.Path, st.Outcome, ver)
		if st.Err != "" {
			line += "  ⚠ " + st.Err
		}
		fmt.Println(line)
	}
	code := exitCodeFor(statuses)
	fmt.Println()
	switch code {
	case exitPending:
		fmt.Println("结论：存在待迁移文件（跑 zerg-compat migrate，或让主控启动时自动迁移）")
	case exitFuture:
		fmt.Println("结论：存在高于本机二进制认知的 schema —— 请升级本机二进制（不猜、不回写）")
	case exitFailed:
		fmt.Println("结论：存在不可解析/不可用的状态文件（见上面的 ⚠）")
	default:
		fmt.Println("结论：全部就绪")
	}
	return code
}

func cmdMigrate(cfg compat.Config, dryRun, asJSON bool) int {
	if dryRun {
		statuses := compat.StatusAll(cfg)
		if asJSON {
			return emitJSONWithExit(statuses, exitCodeFor(statuses))
		}
		fmt.Println("--dry-run：只报告将要迁移的文件，不落盘")
		pending := 0
		for _, st := range statuses {
			if st.Outcome == compat.OutcomeMigrated {
				fmt.Printf("  待迁移  %s（磁盘 schema=%d → 目标 %d）\n", st.Path, st.OnDisk, st.Expected)
				pending++
			}
		}
		fmt.Printf("共 %d 个文件待迁移\n", pending)
		return exitOK
	}

	reports := compat.MigrateAll(cfg)
	if asJSON {
		code := exitOK
		for _, r := range reports {
			if r.Outcome == compat.OutcomeCorrupt {
				code = exitFailed
			}
		}
		return emitJSONWithExit(reports, code)
	}
	fmt.Printf("状态目录: %s\n回执目录: %s\n\n", cfg.Dirs.State, cfg.Dirs.Receipts)
	var migrated, current, missing, corrupt int
	for _, r := range reports {
		switch r.Outcome {
		case compat.OutcomeMigrated:
			migrated++
			fmt.Printf("  ✅ 已迁移  %s\n", r.Path)
		case compat.OutcomeCurrent:
			current++
		case compat.OutcomeMissing:
			missing++
		case compat.OutcomeFuture:
			fmt.Printf("  ⏫ 跳过    %s（schema 高于本机认知——请升级本机二进制）\n", r.Path)
		default:
			corrupt++
			fmt.Printf("  ❌ 失败    %s：%s\n", r.Path, r.Err)
		}
	}
	fmt.Printf("\n迁移 %d · 已是当前 %d · 不存在 %d · 失败 %d\n", migrated, current, missing, corrupt)
	if corrupt > 0 {
		return exitFailed
	}
	return exitOK
}

// cmdManifest —— 打印内嵌清单原文（CI 门禁 / 排障的对照面）。
// 直接写原始字节而不再 json.Marshal 一遍：RawMessage 再序列化会把 `<` 转义成 \u003c，
// 让人读的清单变成难读的一行。
func cmdManifest() int {
	if _, err := os.Stdout.Write(append(compat.ManifestJSONForDump(), '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "写输出失败: %v\n", err)
		return exitFailed
	}
	return exitOK
}

// exitCodeFor —— 结论 → 退出码。优先级：失败(1) > 高版本(3) > 待迁移(2) > 就绪(0)。
// 为什么「失败」压过一切：一个读不出来的状态文件是**眼下就要处理的**，比「有个未来版本」更紧急。
func exitCodeFor(statuses []compat.FileStatus) int {
	code := exitOK
	for _, st := range statuses {
		switch st.Outcome {
		case compat.OutcomeCorrupt:
			return exitFailed
		case compat.OutcomeFuture:
			code = exitFuture
		case compat.OutcomeMigrated:
			if code == exitOK {
				code = exitPending
			}
		}
	}
	return code
}

func emitJSON(v any) int {
	return emitJSONWithExit(v, exitOK)
}

func emitJSONWithExit(v any, code int) int {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化输出失败: %v\n", err)
		return exitFailed
	}
	os.Stdout.Write(append(b, '\n'))
	return code
}
