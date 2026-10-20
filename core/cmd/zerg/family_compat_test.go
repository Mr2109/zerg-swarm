// family_compat_test.go —— `zerg compat list|check|migrate|manifest`（跨版本状态文件兼容层归入命令面 · 批 B7）的判据。
//
// 落点 = `package main`（**内部**）：四个动作都走**外部**入口 `run`（与矩阵同一条 argv 面 ⇒ 判据与真跑同形）。
// 全部合成盘面（`t.TempDir()` 当状态目录与回执目录）：不碰真 `~/.zerg/state`、不碰 `$ZERG_STATE_DIR`、
// 不起 `zerg-compat` 二进制。
//
// 本件钉住的四格（任务单 §三 B7 的「判据件至少」）：
//
//	① **登记面**：命令树里真有 `compat list|check|migrate|manifest` 四条（旗标逐字 · arity none ·
//	   已开放 · 字段表在），且**双轨期源件一字不动**（`core/cmd/zerg-compat/main.go` 仍在 · 它自己的
//	   `0/1/2/3/4` 码空间仍在）；
//	② **正常路径**：旧版状态文件 ⇒ `check` 退 8（待迁移）→ `migrate --dry-run` 退 0 且**一个字节不落**
//	   → `migrate` 退 0（备份 + 盖章）→ 再 `check` 退 0（就绪）→ 二跑 `migrate` 幂等 → `list` / `manifest`
//	   出表；`--json` 面按字段表出行；
//	③ **用法面**：未知旗标 / 多余位置参数 / 取值旗标裸给 / `--json` 不给字段（K2 甲 ⇒ 1）/ `manifest`
//	   收旗标 / `list` 收 `--dry-run` ⇒ 一律 2（除 K2 那格外）＋ stdout 零字节；且这两枚旗标**不泄漏**
//	   给别族（真给了仍退 2「未知旗标」）；
//	④ **只读面**：`list` / `check` / `manifest` / `migrate --dry-run` 跑前跑后目录**逐件 sha256 不变**。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// compatRun —— 走**外部**入口（与矩阵/真跑同一条 argv 面）。
func compatRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// compatDirSig —— 目录的**逐件 sha256 指纹**（只读面的两态对拍；排序后拼成一行）。
func compatDirSig(t *testing.T, dir string) string {
	t.Helper()
	var ents []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		sum := sha256.Sum256(data)
		ents = append(ents, p+"="+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("目录指纹读不了 %s：%v", dir, err)
	}
	sort.Strings(ents)
	return strings.Join(ents, "\n")
}

// compatWrite —— 铺一件合成状态文件。
func compatWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败 %s：%v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败 %s：%v", path, err)
	}
}

// ① 登记面。
func TestCompatCommandRegistered(t *testing.T) {
	type want struct {
		path   string
		flags  []string
		arity  string
		fields []string
	}
	for _, w := range []want{
		{"compat list", []string{"--state-dir", "--receipts-dir", "--json"}, arityNone,
			[]string{"name", "file", "dir", "current_schema", "min_readable", "envelope", "stamps_schema", "owner"}},
		{"compat check", []string{"--state-dir", "--receipts-dir", "--json"}, arityNone,
			[]string{"name", "path", "outcome", "on_disk_schema", "current_schema", "envelope", "owner", "error"}},
		{"compat migrate", []string{"--state-dir", "--receipts-dir", "--dry-run", "--json"}, arityNone,
			[]string{"name", "path", "outcome", "error", "on_disk_schema", "current_schema"}},
		{"compat manifest", nil, arityNone, nil},
	} {
		info, ok := CommandInfoOfForTest(w.path)
		if !ok {
			t.Fatalf("命令树里没有 `zerg %s`（批 B7 的核心那一条）", w.path)
		}
		if !strings.HasPrefix(info.Usage, "zerg "+w.path) {
			t.Errorf("`%s` 的用法串没以 `zerg %s` 开头：%q", w.path, w.path, info.Usage)
		}
		for _, f := range w.flags {
			if !strings.Contains(info.Usage, f) {
				t.Errorf("`%s` 的用法串漏了旗标 %s（源件四枚必须逐字保留）：%q", w.path, f, info.Usage)
			}
		}
		if w.path == "compat manifest" {
			// 差异 ④：manifest **不收任何旗标** —— 用法串里一个 `--` 都不许出现。
			if strings.Contains(info.Usage, "--") {
				t.Errorf("`compat manifest` 的用法串不该列任何旗标（差异 ④）：%q", info.Usage)
			}
		}
		if info.Passthrough {
			t.Errorf("`%s` 不该是透传型（它自己判旗标语义）", w.path)
		}
		if !info.OpenForRun {
			t.Errorf("`%s` 要标成**已开放**（真跑 —— 不是占位动作）", w.path)
		}
		var arity string
		var fields []string
		for _, c := range commands {
			if joinPath(c.path) == w.path {
				arity, fields = c.arity, c.fields
			}
		}
		if arity != w.arity {
			t.Errorf("`%s` 的 arity = %q（要 %q —— 源件的多余位置参数一律静默忽略，命令面按 K14 拒）", w.path, arity, w.arity)
		}
		got := map[string]bool{}
		for _, f := range fields {
			got[f] = true
		}
		for _, f := range w.fields {
			if !got[f] {
				t.Errorf("`%s` 的字段表缺 %q（--json 面点不出来）：%v", w.path, f, fields)
			}
		}
	}

	// ★ 双轨期：源件**一字不动**（本批只搬逻辑、不删入口）；它自己的码空间（4 = 用法错）仍在。
	src := filepath.Join("..", "zerg-compat", "main.go")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("双轨期源件必须还在（%s）：%v", src, err)
	}
	for _, want := range []string{"exitPending = 2", "exitFuture  = 3", "exitUsage   = 4", "func cmdManifest() int"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("源件 `%s` 里的 %q 不见了 —— 双轨期它必须一字不动", src, want)
		}
	}
}

// ② 正常路径（旧版 → 待迁移 → 迁移 → 就绪 → 幂等）。
func TestCompatNormalPath(t *testing.T) {
	state, receipts := t.TempDir(), t.TempDir()
	legacy := `{"stopped":true,"since":"2026-01-02T03:04:05Z"}`
	engine := filepath.Join(state, "internal_engine.json")
	compatWrite(t, engine, legacy)

	// ① 体检：待迁移 ⇒ 8（源件 2 ⇒ 命令面 `blocked`；`error.detail` 分家）。
	rc, out, errb := compatRun(t, "compat", "check", "--state-dir", state, "--receipts-dir", receipts)
	if rc != exitBlocked {
		t.Fatalf("`compat check`（有旧版）rc=%d（要 %d = blocked）\nstdout:\n%s\nstderr:\n%s", rc, exitBlocked, out, errb)
	}
	for _, want := range []string{"状态目录: " + state, "回执目录: " + receipts, "internal_engine", "结论：存在待迁移文件"} {
		if !strings.Contains(out, want) {
			t.Errorf("`compat check` 人面缺 %q：\n%s", want, out)
		}
	}

	// ② --dry-run：只报计划、**一个字节不落**。
	before := compatDirSig(t, state)
	rc, out, errb = compatRun(t, "compat", "migrate", "--state-dir", state, "--receipts-dir", receipts, "--dry-run")
	if rc != exitOK {
		t.Fatalf("`compat migrate --dry-run` rc=%d（要 0）\nstderr:\n%s", rc, errb)
	}
	for _, want := range []string{"--dry-run：只报告将要迁移的文件，不落盘", "待迁移", "共 1 个文件待迁移"} {
		if !strings.Contains(out, want) {
			t.Errorf("`compat migrate --dry-run` 人面缺 %q：\n%s", want, out)
		}
	}
	if after := compatDirSig(t, state); after != before {
		t.Fatalf("`--dry-run` 改动了盘面（零副作用判据）：\n before=%s\n after =%s", before, after)
	}

	// ③ 真迁移：盖章 + 备份（迁移正文逐字保留）。
	rc, out, errb = compatRun(t, "compat", "migrate", "--state-dir", state, "--receipts-dir", receipts)
	if rc != exitOK {
		t.Fatalf("`compat migrate` rc=%d（要 0）\nstdout:\n%s\nstderr:\n%s", rc, out, errb)
	}
	if !strings.Contains(out, "已迁移") || !strings.Contains(out, "迁移 1 · 已是当前 0") {
		t.Errorf("`compat migrate` 人面缺迁移汇总：\n%s", out)
	}
	migrated, rerr := os.ReadFile(engine)
	if rerr != nil {
		t.Fatalf("迁移后状态文件不见了：%v", rerr)
	}
	for _, want := range []string{`"schema": 1`, `"stopped": true`, `"since": "2026-01-02T03:04:05Z"`} {
		if !strings.Contains(string(migrated), want) {
			t.Errorf("迁移后的正文缺 %q（载荷必须一个不少）：\n%s", want, migrated)
		}
	}
	baks, _ := filepath.Glob(engine + ".bak-*")
	if len(baks) != 1 {
		t.Fatalf("应恰好 1 份备份（源件同档），实际 %d：%v", len(baks), baks)
	}
	if b, _ := os.ReadFile(baks[0]); string(b) != legacy {
		t.Errorf("备份必须是迁移前原文，实际 %q", string(b))
	}

	// ④ 再体检：就绪 ⇒ 0。
	rc, out, errb = compatRun(t, "compat", "check", "--state-dir", state, "--receipts-dir", receipts)
	if rc != exitOK {
		t.Fatalf("迁移后 `compat check` rc=%d（要 0）\nstdout:\n%s\nstderr:\n%s", rc, out, errb)
	}
	if !strings.Contains(out, "结论：全部就绪") {
		t.Errorf("迁移后应报「全部就绪」：\n%s", out)
	}

	// ⑤ 二跑幂等：不再新增备份、正文逐字节不变。
	sig := compatDirSig(t, state)
	rc, _, errb = compatRun(t, "compat", "migrate", "--state-dir", state, "--receipts-dir", receipts)
	if rc != exitOK {
		t.Fatalf("二跑 `compat migrate` rc=%d（要 0 —— 幂等短路）\nstderr:\n%s", rc, errb)
	}
	if after := compatDirSig(t, state); after != sig {
		t.Fatalf("二跑 migrate 改动了盘面（幂等破了）：\n before=%s\n after =%s", sig, after)
	}

	// ⑥ list：表头 + 逐条 + excluded 计数。
	rc, out, errb = compatRun(t, "compat", "list", "--state-dir", state, "--receipts-dir", receipts)
	if rc != exitOK {
		t.Fatalf("`compat list` rc=%d（要 0）\nstderr:\n%s", rc, errb)
	}
	for _, want := range []string{"状态目录: " + state, "名称", "最低读", "信封", "internal_engine", "明确不登记："} {
		if !strings.Contains(out, want) {
			t.Errorf("`compat list` 人面缺 %q：\n%s", want, out)
		}
	}

	// ⑦ manifest：原始字节（不是再序列化的一行）。
	rc, out, errb = compatRun(t, "compat", "manifest")
	if rc != exitOK {
		t.Fatalf("`compat manifest` rc=%d（要 0）\nstderr:\n%s", rc, errb)
	}
	for _, want := range []string{`"entries"`, `"internal_engine"`, `"excluded"`} {
		if !strings.Contains(out, want) {
			t.Errorf("`compat manifest` 输出缺 %q（应是清单原文）", want)
		}
	}
	if strings.Count(out, "\n") < 20 {
		t.Errorf("清单原文应是多行 JSON（不再 json.Marshal 成一行）：%d 行", strings.Count(out, "\n"))
	}

	// ⑧ --json 面：标准包封 + 字段投影（差异 ③）。
	rc, out, errb = compatRun(t, "compat", "check", "--state-dir", state, "--receipts-dir", receipts, "--json", "name,outcome,on_disk_schema")
	if rc != exitOK {
		t.Fatalf("`compat check --json …` rc=%d（要 0）\nstderr:\n%s", rc, errb)
	}
	for _, want := range []string{`"schema":"zerg/v1"`, `"kind":"CompatCheck"`, `"outcome":"current"`, `"on_disk_schema":"1"`} {
		if !strings.Contains(out, want) {
			t.Errorf("`compat check --json` 缺 %q：\n%s", want, out)
		}
	}
	if strings.Contains(out, `"owner"`) {
		t.Errorf("`--json` 只许出点名的那几格（字段投影）：\n%s", out)
	}
}

// ③ 用法面：一律 2（K2 那格 1）＋ stdout 零字节；旗标不泄漏给别族。
func TestCompatUsageFace(t *testing.T) {
	state, receipts := t.TempDir(), t.TempDir()
	cases := []struct {
		name string
		argv []string
		want int
	}{
		{"list 未知旗标", []string{"compat", "list", "--nosuchflag-zz"}, exitUsage},
		{"check 未知旗标", []string{"compat", "check", "--nosuchflag-zz"}, exitUsage},
		{"migrate 未知旗标", []string{"compat", "migrate", "--nosuchflag-zz"}, exitUsage},
		{"manifest 未知旗标", []string{"compat", "manifest", "--nosuchflag-zz"}, exitUsage},
		{"不认的契约主号", []string{"compat", "list", "--schema", "zerg/v9"}, exitUsage},
		{"多余位置参数（arity none）", []string{"compat", "list", "extra"}, exitUsage},
		{"--state-dir 裸给", []string{"compat", "list", "--state-dir"}, exitUsage},
		{"--receipts-dir 裸给", []string{"compat", "check", "--receipts-dir"}, exitUsage},
		{"--json 不给字段（K2 甲档）", []string{"compat", "list", "--json"}, exitUsage},
		{"--json 点错字段", []string{"compat", "list", "--json", "nosuch_field"}, exitUsage},
		// 差异 ④：manifest 不收旗标（源件静默忽略 ⇒ 命令面明拒）。
		{"manifest 收 --json", []string{"compat", "manifest", "--json", "a"}, exitUsage},
		{"manifest 收 --state-dir", []string{"compat", "manifest", "--state-dir", state}, exitUsage},
		// 差异 ⑤：`--dry-run` 只归 migrate。
		{"list 收 --dry-run", []string{"compat", "list", "--dry-run"}, exitUsage},
		{"check 收 --dry-run", []string{"compat", "check", "--dry-run"}, exitUsage},
		{"manifest 收 --dry-run", []string{"compat", "manifest", "--dry-run"}, exitUsage},
	}
	for _, c := range cases {
		rc, out, errb := compatRun(t, c.argv...)
		if rc != c.want {
			t.Errorf("%s：argv=%v rc=%d（要 %d）\nstdout=%q\nstderr=%s", c.name, c.argv, rc, c.want, out, errb)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("%s：用法面/失败面 stdout 必须零字节，实得 %q", c.name, out)
		}
	}

	// 旗标不泄漏：本批上户口的两枚给别族仍退 2「未知旗标」（硬约束⑤：既有命令的退码一字不动）。
	for _, argv := range [][]string{
		{"version", "--state-dir", state},
		{"gate", "ls", "--receipts-dir", receipts},
		{"code", "show", "x", "--state-dir", state},
	} {
		rc, out, errb := compatRun(t, argv...)
		if rc != exitUsage || strings.TrimSpace(out) != "" {
			t.Errorf("旗标泄漏：argv=%v rc=%d stdout=%q（要 2 / 零字节）\nstderr:%s", argv, rc, out, errb)
		}
	}

	// 目录注入必须**真生效**：合成目录里的旧版被迁，且只动那一件。
	engine := filepath.Join(state, "internal_engine.json")
	compatWrite(t, engine, `{"stopped":true}`)
	other := filepath.Join(state, "ui", "prefs.json")
	compatWrite(t, other, `{"schema":1,"theme":"dark"}`) // 已是当前 schema ⇒ 迁移**不该**碰它
	otherSig := compatDirSig(t, filepath.Join(state, "ui"))
	if rc, _, errb := compatRun(t, "compat", "migrate", "--state-dir", state, "--receipts-dir", receipts); rc != exitOK {
		t.Fatalf("合成目录迁移 rc=%d（要 0）\nstderr:%s", rc, errb)
	}
	if b, _ := os.ReadFile(engine); !strings.Contains(string(b), `"schema": 1`) {
		t.Errorf("--state-dir 没生效（合成目录里的旧版没被迁）：%s", b)
	}
	if after := compatDirSig(t, filepath.Join(state, "ui")); after != otherSig {
		t.Errorf("迁移动了不该动的件（ui/prefs.json 已是当前 schema）：\n before=%s\n after =%s", otherSig, after)
	}
}

// ④ 只读面：四个动作里只有 migrate（非 dry-run）能改盘。
func TestCompatReadOnly(t *testing.T) {
	state, receipts := t.TempDir(), t.TempDir()
	compatWrite(t, filepath.Join(state, "internal_engine.json"), `{"stopped":true}`)
	compatWrite(t, filepath.Join(state, "ui", "prefs.json"), `{"theme":"dark"}`)
	compatWrite(t, filepath.Join(receipts, "r1.json"), `{"schema":1,"ok":true}`)

	before := compatDirSig(t, state)
	beforeR := compatDirSig(t, receipts)
	for _, argv := range [][]string{
		{"compat", "list", "--state-dir", state, "--receipts-dir", receipts},
		{"compat", "check", "--state-dir", state, "--receipts-dir", receipts},
		{"compat", "migrate", "--state-dir", state, "--receipts-dir", receipts, "--dry-run"},
		{"compat", "manifest"},
		{"compat", "check", "--state-dir", state, "--receipts-dir", receipts, "--json", "name,outcome"},
	} {
		if rc, out, errb := compatRun(t, argv...); rc != exitOK && rc != exitBlocked {
			t.Errorf("只读面 argv=%v rc=%d（要 0 或 8 —— 待迁移档）\nstdout:%s\nstderr:%s", argv, rc, out, errb)
		}
	}
	if after := compatDirSig(t, state); after != before {
		t.Fatalf("只读命令改动了状态目录：\n before=%s\n after =%s", before, after)
	}
	if after := compatDirSig(t, receipts); after != beforeR {
		t.Fatalf("只读命令改动了回执目录：\n before=%s\n after =%s", beforeR, after)
	}
}
