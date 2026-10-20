// family_sched_run_test.go —— `zerg sched run`（调度器归入命令面 · 批 B2）的判据。
//
// 落点 = `package main`（**内部**）：常驻档的一条 argv **跑不返回**（矩阵一格 = 一条 argv = 一跑），
// 所以「起 → 停」那一趟走实现的同一入口 `schedRunServe`（`cmdSchedRun` 只多一层信号 ctx）；
// 用法面 / 命令树面走**外部**入口 `run`（`package main_test` 用的同一件）。
// 全部合成盘面（`t.TempDir()` 当工作目录）：不碰真 `~/.zerg/state`、不起 `zerg-scheduler` 二进制、
// 不派任何任务（工作目录里没有 `docs/issues/` ⇒ 一轮扫空）。
//
// 本件钉住的四格（任务单 §三 B2 的「判据件至少」）：
//
//	① **登记面**：命令树里真有 `sched run`（旗标逐字五枚 = 源件 `-workdir`/`-container`/
//	   `-agent-bin`/`-dev`/`-repo` · `arity=none`），且**既有四条 `sched` 动作一条不少**；
//	② **正常路径**：`schedRunServe` 起 → ctx 结束 → 停干净（退 0）—— 启动五行逐字、调度器两档与
//	   `RepoDir` 的映射逐格对（容器 / 开发 / 仓库三条合成配置）；
//	③ **用法面**：未知旗标 / 位置参数 / `--workdir` 裸给 / 不认的契约主号 ⇒ 一律 2 + stdout 零字节；
//	   且这五枚新旗标**不泄漏**给别族（`trace --workdir` 仍 2）；
//	④ **只读面**：跑前/跑后工作目录**逐件 sha256 不变**（常驻档只读配置与任务单目录）。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// schedRunInvoke —— 走**外部**入口跑一次（与 `package main_test` 的 `RunForTest` 同一件）。
func schedRunInvoke(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// schedRunTreeHash —— 工作目录的逐件指纹（相对路径 + sha256 + 字节数 · 排序后拼一行）。
func schedRunTreeHash(t *testing.T, root string) string {
	t.Helper()
	var rows []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			rows = append(rows, "d "+rel)
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(raw)
		rows = append(rows, "f "+rel+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("工作目录指纹取不到：%v", err)
	}
	sort.Strings(rows)
	return strings.Join(rows, "\n")
}

// ① 登记面：path / 旗标逐字 / arity / 既有四条动作仍在。
func TestSchedRunCommandRegistered(t *testing.T) {
	info, ok := CommandInfoOfForTest("sched run")
	if !ok {
		t.Fatalf("命令树里没有 `zerg sched run`（批 B2 的核心那一条）")
	}
	for _, f := range []string{"--workdir", "--container", "--agent-bin", "--dev", "--repo"} {
		if !strings.Contains(info.Usage, f) {
			t.Errorf("用法串漏了旗标 %s（源件五枚必须逐字保留）：%q", f, info.Usage)
		}
	}
	if !strings.Contains(info.Usage, "zerg sched run") {
		t.Errorf("用法串没以 `zerg sched run` 开头：%q", info.Usage)
	}
	if info.Passthrough {
		t.Errorf("`sched run` 不该是透传型（它自己判旗标语义）")
	}
	if !info.OpenForRun {
		t.Errorf("`sched run` 要标成**已开放**（真跑起进程 —— 不是占位动作）")
	}

	// arity：不收位置参数（同族 `sched show/pause/resume` 都是 none）。
	var arity string
	for _, c := range commands {
		if joinPath(c.path) == "sched run" {
			arity = c.arity
		}
	}
	if arity != arityNone {
		t.Errorf("`sched run` 的 arity = %q（要 %q —— 位置参数一律拒）", arity, arityNone)
	}

	// 既有四条动作**一条不少**（归入是「只增一格」，✗ 不许挤掉控制面那四条）。
	for _, p := range []string{"sched show", "sched pause", "sched resume", "sched set"} {
		if _, ok := CommandInfoOfForTest(p); !ok {
			t.Errorf("`zerg %s` 不见了 —— 本批只许新增 `sched run`，不许动控制面四条", p)
		}
	}
}

// ② 正常路径：三组合成配置逐格对（容器档 / 开发档 / 仓库档）＋ 起停一整趟退 0。
func TestSchedRunNormalPath(t *testing.T) {
	wd := t.TempDir()
	repo := filepath.Join(t.TempDir(), "dev-repo")

	// 裸跑（源件缺省档）：不带任何旗标 ⇒ 工作目录 = `.`（= 当前目录），仓库不设。
	inv, err := parseInvocation([]string{"sched", "run", "--workdir", wd})
	if err != nil {
		t.Fatalf("解析 `sched run --workdir` 失败：%v", err)
	}
	cfg, rc := schedRunParse(inv, &bytes.Buffer{})
	if rc != exitOK {
		t.Fatalf("`sched run --workdir <dir>` 的配置档 rc=%d（要 0）", rc)
	}
	if cfg.workDir != wd {
		t.Errorf("工作目录 = %q（要 %q —— 绝对路径）", cfg.workDir, wd)
	}
	if cfg.container || cfg.dev || cfg.repo != "" {
		t.Errorf("裸跑档的容器/开发/仓库三格该是零值：%+v", cfg)
	}
	if cfg.agentBin != schedRunDefaultAgentBin {
		t.Errorf("`--agent-bin` 缺省 = %q（要源件的 %q）", cfg.agentBin, schedRunDefaultAgentBin)
	}
	if s := schedRunPrepare(cfg); s.ContainerMode || s.DevMode || s.RepoDir != "" {
		t.Errorf("裸跑档的调度器三格该是零值：ContainerMode=%v DevMode=%v RepoDir=%q",
			s.ContainerMode, s.DevMode, s.RepoDir)
	}

	// 容器 + 开发 + 仓库三档同给（源件三段逐条搬）。
	inv, err = parseInvocation([]string{"sched", "run", "--workdir", wd,
		"--container", "--agent-bin", "/opt/zerg-agent-linux-arm64", "--dev", "--repo", repo})
	if err != nil {
		t.Fatalf("解析全旗标形态失败：%v", err)
	}
	cfg, rc = schedRunParse(inv, &bytes.Buffer{})
	if rc != exitOK {
		t.Fatalf("全旗标形态 rc=%d（要 0）", rc)
	}
	if !cfg.container || !cfg.dev || cfg.repo != repo || cfg.agentBin != "/opt/zerg-agent-linux-arm64" {
		t.Fatalf("全旗标形态解析不对：%+v", cfg)
	}
	s := schedRunPrepare(cfg)
	if !s.ContainerMode || s.ContainerAgentBin != "/opt/zerg-agent-linux-arm64" {
		t.Errorf("容器档没落到调度器：ContainerMode=%v bin=%q", s.ContainerMode, s.ContainerAgentBin)
	}
	if !s.DevMode || s.RepoDir != repo {
		t.Errorf("开发档没落到调度器：DevMode=%v RepoDir=%q（要 %q）", s.DevMode, s.RepoDir, repo)
	}

	// 只给 `--dev` 不给 `--repo` ⇒ RepoDir = 工作目录（源件 `⚠️ DevMode without -repo` 那一档）。
	inv, err = parseInvocation([]string{"sched", "run", "--workdir", wd, "--dev"})
	if err != nil {
		t.Fatalf("解析 `--dev` 单给失败：%v", err)
	}
	cfg, rc = schedRunParse(inv, &bytes.Buffer{})
	if rc != exitOK {
		t.Fatalf("`--dev` 单给 rc=%d（要 0）", rc)
	}
	if s := schedRunPrepare(cfg); s.RepoDir != wd {
		t.Errorf("只给 `--dev` 时 RepoDir = %q（要工作目录 %q —— 源件同档）", s.RepoDir, wd)
	}

	// 起 → 停一整趟：ctx 已结束 ⇒ Start 一轮即退，Serve 退 0（同一实现，只是 ctx 不同源）。
	var errb bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rc := schedRunServe(ctx, cfg, &errb); rc != exitOK {
		t.Fatalf("常驻档起停一趟 rc=%d（要 0）· stderr=\n%s", rc, errb.String())
	}
	got := errb.String()
	for _, want := range []string{
		"📂 working directory: " + wd,
		"🔀 dev mode: in-container branch work → commit → PR",
		"⚠️ DevMode without -repo; using workDir as the dev repository",
		"⚙️  concurrency limit: ",
		"📡 stop signal received, shutting down gracefully...",
		"🛑 scheduler stopped",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("起停面缺 %q\n—— 实得：\n%s", want, got)
		}
	}

	// 容器档那一行（源件 `🐳` 判词）单独一格。
	var cbuf bytes.Buffer
	cctx, ccancel := context.WithCancel(context.Background())
	ccancel()
	ccfg, crc := schedRunParse(mustInvokeSched(t, "run", "--workdir", wd, "--container", "--agent-bin", "/opt/a"), &bytes.Buffer{})
	if crc != exitOK {
		t.Fatalf("容器档配置 rc=%d（要 0）", crc)
	}
	if rc := schedRunServe(cctx, ccfg, &cbuf); rc != exitOK {
		t.Fatalf("容器档起停 rc=%d（要 0）", rc)
	}
	if !strings.Contains(cbuf.String(), "🐳 container mode: docker run zerg-dev (CA outside sandbox — Linux agent=/opt/a)") {
		t.Errorf("容器档没打源件那行判词：\n%s", cbuf.String())
	}
}

// mustInvokeSched —— 造一枚已解析的 invocation（`run` 之外的旗标面直取，便于单测配置档）。
func mustInvokeSched(t *testing.T, argv ...string) *invocation {
	t.Helper()
	inv, err := parseInvocation(append([]string{"sched"}, argv...))
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	return inv
}

// ③ 用法面：四格必须 2 + stdout 零字节；新旗标不泄漏给别族。
func TestSchedRunUsageFace(t *testing.T) {
	wd := t.TempDir()
	cases := []struct {
		name string
		argv []string
		want string // stderr 必含
	}{
		{"未知旗标", []string{"sched", "run", "--nosuchflag-zz"}, "未知旗标"},
		{"多余位置参数", []string{"sched", "run", "extra"}, "不收位置参数"},
		{"--workdir 裸给", []string{"sched", "run", "--workdir"}, "--workdir"},
		{"不认的契约主号", []string{"sched", "run", "--schema", "zerg/v9"}, "契约主号"},
	}
	for _, c := range cases {
		rc, out, errb := schedRunInvoke(t, c.argv...)
		if rc != exitUsage {
			t.Errorf("%s：rc=%d（要 %d · 用法错）", c.name, rc, exitUsage)
		}
		if out != "" {
			t.Errorf("%s：stdout 要零字节（用法类错误不吐包封 · K2 甲档）：%q", c.name, out)
		}
		if !strings.Contains(errb, c.want) {
			t.Errorf("%s：stderr 没说到 %q：%q", c.name, c.want, errb)
		}
	}
	// `--workdir` 空值（`--workdir=`）与裸给同档。
	if rc, _, errb := schedRunInvoke(t, "sched", "run", "--workdir="); rc != exitUsage ||
		!strings.Contains(errb, "`--workdir` 要一个") {
		t.Errorf("`--workdir=` rc=%d（要 2）· stderr=%q", rc, errb)
	}
	// 有效目录 ⇒ 一路到常驻档（本测试只判「解析不拦」：用已取消 ctx 不行，走 run 会阻塞 ⇒
	// 只判 `schedRunParse` 那一层，见上「正常路径」；这里补一格 workdir 是**相对路径**也认）。
	inv := mustInvokeSched(t, "run", "--workdir", wd)
	cfg, rc := schedRunParse(inv, &bytes.Buffer{})
	if rc != exitOK || !filepath.IsAbs(cfg.workDir) {
		t.Errorf("有效目录：rc=%d（要 0）· workDir=%q（要是绝对路径）", rc, cfg.workDir)
	}

	// **不泄漏**：新旗标在别族的命令上一律仍 2（数据驱动收口那一排）。
	// ★ `sched run --container` 不在此列 —— 它会一路进常驻档（`run()` 一跑不返回），
	//   那条归上「正常路径」的配置档判（`schedRunParse` 直取），不在此处起进程。
	for _, argv := range [][]string{
		{"trace", "--workdir", wd},
		{"sched", "show", "--workdir", wd},
		{"sched", "show", "--dev"},
	} {
		rc, out, _ := schedRunInvoke(t, argv...)
		if rc != exitUsage || out != "" {
			t.Errorf("%v：rc=%d out=%q（别族给新旗标要仍 2 且 stdout 零字节）", argv, rc, out)
		}
	}
}

// ④ 只读面：常驻档起停一趟，工作目录逐件 sha256 不变。
func TestSchedRunReadOnly(t *testing.T) {
	wd := t.TempDir()
	// 造一点「盘上本来就有」的东西：任务单目录 + 一份 workers 配置（读面都要碰它们）。
	if err := os.MkdirAll(filepath.Join(wd, "docs", "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wd, "docs", "issues", "T-1.md"), []byte("# 任务一\n优先级: P3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wd, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wd, "config", "workers.yaml"), []byte("devices: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := schedRunTreeHash(t, wd)
	inv := mustInvokeSched(t, "run", "--workdir", wd)
	cfg, rc := schedRunParse(inv, &bytes.Buffer{})
	if rc != exitOK {
		t.Fatalf("配置档 rc=%d（要 0）", rc)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var errb bytes.Buffer
	if rc := schedRunServe(ctx, cfg, &errb); rc != exitOK {
		t.Fatalf("起停一趟 rc=%d（要 0）· stderr=\n%s", rc, errb.String())
	}
	if after := schedRunTreeHash(t, wd); after != before {
		t.Errorf("`zerg sched run` 动了工作目录（只读面只该读配置与任务单）：\n改前：\n%s\n改后：\n%s", before, after)
	}
}
