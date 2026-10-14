// eggspecgen —— F24「乙·单向派生」的子端命令面（★ 落在 agent/internal/eggpkg/** 之内 ⇒ 不触模块边界）。
//
// 一条命令可复现四件事：
//
//	--check     一致性门（真源 sha16 ⟷ 派生件头 · 派生件全文 ⟷ 真源重生）· rc=0 绿 / 1 红 / 2 不给结论
//	--emit      由真源重生派生件全文（打 stdout；落盘走 dev edit --file 正门）
//	--self-test 四态亲测 ＋ 负控（末行逐字对拍）
//	--parity    只报告档：14 格 A/B 分叉读数（现读校验拄证锚）
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Mr2109/zerg-swarm/agent/internal/eggpkg"
)

func main() {
	repoRoot := flag.String("repo-root", "", "仓根（缺省：由 cwd 上溯找 core/cmd/zerg/zerg_egg_version.go）")
	doCheck := flag.Bool("check", false, "跑一致性门（rc=0 绿 / 1 红 / 2 不给结论）")
	doEmit := flag.Bool("emit", false, "由真源重生派生件全文到 stdout")
	doSelf := flag.Bool("self-test", false, "四态亲测 ＋ 负控（末行逐字对拍）")
	doParity := flag.Bool("parity", false, "只报告档：14 格 A/B 分叉读数（JSON）")
	flag.Parse()

	root := *repoRoot
	if root == "" {
		r, err := discoverRepoRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "eggspecgen: %v\n", err)
			os.Exit(2)
		}
		root = r
	}

	rc := 0
	switch {
	case *doCheck:
		g := eggpkg.EggSpecGate(root)
		fmt.Fprintln(os.Stdout, g.Message)
		fmt.Fprintf(os.Stdout, "status=%d named=%s source_sha=%s derived_sha=%s\n",
			g.Status, g.Named, g.SourceSHA, g.DerivedSHA)
		rc = g.Status
	case *doEmit:
		b, err := eggpkg.EggRenderDerived(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "eggspecgen: %v\n", err)
			os.Exit(2)
		}
		if _, err := os.Stdout.Write(b); err != nil {
			fmt.Fprintf(os.Stderr, "eggspecgen: %v\n", err)
			os.Exit(2)
		}
	case *doSelf:
		rc = eggpkg.EggSpecSelfTest(os.Stdout, os.Stderr)
	case *doParity:
		b, err := eggpkg.EggParityJSON(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "eggspecgen: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, string(b))
	default:
		fmt.Fprint(os.Stdout, usage)
		rc = 2
	}
	os.Exit(rc)
}

const usage = "用法：eggspecgen [--repo-root <仓根>] (--check | --emit | --self-test | --parity)\n" +
	"  --check      一致性门（真源 sha16 ⟷ 派生件头 · 派生件全文 ⟷ 真源重生）rc=0 绿 / 1 红 / 2 不给结论\n" +
	"  --emit       由真源重生派生件全文到 stdout（落盘走 dev edit --file 正门）\n" +
	"  --self-test  四态亲测 ＋ 负控（末行逐字对拍）\n" +
	"  --parity     只报告档：14 格 A/B 分叉读数（JSON）\n"

// discoverRepoRoot —— 由 cwd 上溯找**同时**含 `core/cmd/zerg/zerg_egg_version.go` 与 `agent/go.mod` 的目录；
// 取不到 ⇒ 报错（✗ 不静默回落 cwd）。
func discoverRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(dir, "core", "cmd", "zerg", "zerg_egg_version.go")) &&
			fileExists(filepath.Join(dir, "agent", "go.mod")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("由 cwd 上溯未找到仓根（须含 core/cmd/zerg/zerg_egg_version.go 与 agent/go.mod）")
		}
		dir = parent
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

var _ = io.Discard
