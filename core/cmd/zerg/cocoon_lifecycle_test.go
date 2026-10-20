package main

// cocoon_lifecycle_test.go —— 虫茧生命周期五条（`open|close|ls|inspect|bind` · C1）的成对负控测试件。
//
// 四格（每一格都带负控 —— 哑化对应判据即红）：
//
//	① 登记面  `TestCocoonLifecycleRegistered` / `TestCocoonOpenRegistered`：
//	   五条在册 · runner 非 nil · usage 里承诺的旗标真能被解析面认（`--purge` 解析 case 在）
//	   · `cocoon open` 不再 `refuses`（真跑已开放）· 卵包四条（pack|info|verify|xfer）一个字节没动。
//	② 全链    `TestCocoonOpenLsCloseChain`：open → ls → bind → inspect → close（留盘）→ ls → close --purge
//	   全链走 FakeBackend；逐段读盘对拍（meta.phase / 端口 / 墓碑 / 目录在不在）。
//	③ 用法面  `TestCocoonOpenUsageFace` / `TestCocoonCloseUsageFace` / `TestCocoonInspectUsageFace`：
//	   缺参 · 多参 · 路径出允许根 · 契约缺/坏 · id 不存在 · `--purge` 缺 `--confirm` ·
//	   `--confirm` 值不等（**零删除**）· 终态的茧不能再 bind。
//	④ 只读面  `TestCocoonInspectReadOnly` / `TestCocoonLsDoesNotListOrphans`：
//	   inspect 不写盘（连锁件都不建 —— 整棵树逐件 sha256 对拍）· 令牌只出脱敏形 ·
//	   ls 不把「有目录无 meta」的孤儿列成茧 · `open --dry-run` 零副作用。
//	⑤ 契约哈希锁（判据 9）`TestCocoonSpecDriftOpenRefused` / `TestCocoonSpecDriftInspectReports`：
//	   改一行 cocoon.toml ⇒ `open` 拒（退 1 · 带期望/实得前 8 位 · 零写入 · dry-run 同拒）；
//	   `inspect` 报 drift 但 rc=0 且不写盘（整树对拍）· 没有锁值的老茧 ⇒ unlocked。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/cocoon"
)

// ccLifeSpec — 最小可过契约（无 readiness ⇒ 起完即就绪；假后端不起监听，故不能声明 readiness 端口探测）。
const ccLifeSpec = `[茧]
name   = "shop"
image  = "python:3.12-slim"
digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

[[服务]]
name  = "api"
entry = ["uvicorn", "app:api"]

[[端口]]
inner = 8000
host  = "auto"
bind  = "127.0.0.1"

[资源]
cpu     = 1
mem_gb  = 2
disk_gb = 10
`

// ccLifeSetup —— 沙箱：项目目录（含 cocoon.toml）+ 独立状态根 + 允许根。
func ccLifeSetup(t *testing.T) (proj, root string) {
	t.Helper()
	base := t.TempDir()
	proj = filepath.Join(base, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatalf("建项目目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, cocoon.SpecFileName), []byte(ccLifeSpec), 0o644); err != nil {
		t.Fatalf("写 cocoon.toml 失败: %v", err)
	}
	root = filepath.Join(base, "cocoons")
	t.Setenv(cocoonRootEnv, root)
	t.Setenv(cocoonAllowRootsEnv, base)
	t.Setenv(cocoonBackendEnv, cocoonBackendFake)
	t.Setenv(cocoonTokenEnv, "")
	t.Setenv(cocoonAIUpstreamEnv, "")
	return proj, root
}

// ccLifeRun —— 造一个 invocation 直接跑命令（不走进程，便于 FakeBackend 同进程共享实例账）。
func ccLifeRun(t *testing.T, fn func(*invocation, io.Writer, io.Writer) int,
	path []string, args []string, fields []string, kv map[string]string) (int, string, string) {
	t.Helper()
	inv := &invocation{path: path, args: args}
	if len(fields) > 0 {
		inv.jsonGiven = true
		inv.fields = fields
	}
	for k, v := range kv {
		switch k {
		case "--confirm":
			inv.confirm, inv.confirmGiven = v, true
		case "--dry-run":
			inv.dryRun = true
		default:
			inv.kvSet(k, v)
		}
	}
	var o, e strings.Builder
	rc := fn(inv, &o, &e)
	return rc, o.String(), e.String()
}

// ccLifeRows —— 从包封里取 items（机器面恒数组）。
func ccLifeRows(t *testing.T, out string) []map[string]any {
	t.Helper()
	var env struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 输出不是合法包封：%v\n%s", err, out)
	}
	return env.Items
}

func ccLifeCell(row map[string]any, k string) string {
	v, ok := row[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// ccLifeSnap —— 整棵树的逐件 sha256（判「有没有写盘」用：内容 + 路径集合；✗ 不看 atime）。
func ccLifeSnap(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			out[rel] = "dir"
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(b)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("快照 %s 失败: %v", dir, err)
	}
	return out
}

func ccLifeDiffSnap(t *testing.T, before, after map[string]string) {
	t.Helper()
	for k, v := range after {
		if before[k] != v {
			t.Errorf("盘上多了/改了 %q（before=%q after=%q）", k, before[k], v)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			t.Errorf("盘上少了 %q", k)
		}
	}
}

// ccLifeMetaPhase — 读盘上某茧的 phase（直接读 meta，不经命令面）。
func ccLifeMetaPhase(t *testing.T, root, id string) (cocoon.Phase, bool) {
	t.Helper()
	m, err := cocoon.Load(cocoon.MetaPath(filepath.Join(root, id)))
	if err != nil {
		return "", false
	}
	return m.Phase, true
}

// ── ① 登记面 ────────────────────────────────────────────────────────────

func TestCocoonLifecycleRegistered(t *testing.T) {
	wantFlag := map[string][]string{
		"cocoon open":    {"--dry-run", "--json"},
		"cocoon close":   {"--purge", "--confirm=", "--json"},
		"cocoon ls":      {"--json"},
		"cocoon inspect": {"--json"},
		"cocoon bind":    {"--json"},
	}
	seen := map[string]bool{}
	for _, c := range catalog() {
		j := strings.Join(c.path, " ")
		flags, ok := wantFlag[j]
		if !ok {
			continue
		}
		seen[j] = true
		if c.run == nil {
			t.Fatalf("%s 在册但 run 为 nil", j)
		}
		if c.refuses {
			t.Errorf("%s 仍标 refuses（五条已真跑开放）", j)
		}
		for _, f := range flags {
			if !strings.Contains(c.usage, f) {
				t.Errorf("%s 的 usage 没宣告 %q：%s", j, f, c.usage)
			}
		}
	}
	for j := range wantFlag {
		if !seen[j] {
			t.Fatalf("%s 不在命令树里", j)
		}
	}
	// `open` 的字段表必须与 runner 真能出的字段对得上（空字段表 ⇒ `--json` 会被 dispatch 当说明面拒）。
	for _, c := range catalog() {
		if strings.Join(c.path, " ") == "cocoon open" && len(c.fields) == 0 {
			t.Fatal("`cocoon open` 没有字段表 ⇒ --json 会被当说明面拒（2）")
		}
	}
}

func TestCocoonOpenRegistered(t *testing.T) {
	// ★ 卵包四条（A12 归名）一个字节没动：仍 passthrough + 挂 cmdCocoonEgg。
	for _, sub := range []string{"pack", "info", "verify", "xfer"} {
		var hit *command
		for _, c := range catalog() {
			if len(c.path) == 2 && c.path[0] == "cocoon" && c.path[1] == sub {
				hit = c
			}
		}
		if hit == nil {
			t.Fatalf("`cocoon %s` 不在册（卵包四条被本单挪动了）", sub)
		}
		if !hit.passthrough || runnerName(hit.run) != runnerName(cmdCocoonEgg) {
			t.Errorf("`cocoon %s` 的转发面被动过（passthrough=%t runner=%s）", sub, hit.passthrough, runnerName(hit.run))
		}
	}
	// ★ `--purge` 必须在解析面有专用 case（只登记旗标、解析器不认 ⇒ 一带就「未知旗标」= 命令不可用）。
	got := parseInvocationForTest(t, []string{"cocoon", "close", "--purge", "--confirm=x"})
	if len(got.unknown) > 0 {
		t.Fatalf("`--purge` 没上解析户口（unknown=%v）⇒ 命令等于不可用", got.unknown)
	}
	if !got.hasFlag("--purge") {
		t.Fatal("`--purge` 解析后 hasFlag 为假（kv 没写进去）")
	}
}

// parseInvocationForTest —— 走真解析面（不猜）。
func parseInvocationForTest(t *testing.T, args []string) *invocation {
	t.Helper()
	inv, err := parseInvocation(args)
	if err != nil {
		t.Fatalf("解析 %v 失败: %v", args, err)
	}
	return inv
}

// ── ② open → ls → close 全链（FakeBackend）────────────────────────────────

func TestCocoonOpenLsCloseChain(t *testing.T) {
	proj, root := ccLifeSetup(t)

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase", "pid", "ports", "egg", "backend", "dir"}, nil)
	if rc != exitOK {
		t.Fatalf("open rc=%d 期望 0 · stderr=%s", rc, errs)
	}
	rows := ccLifeRows(t, out)
	if len(rows) != 1 {
		t.Fatalf("open 应出 1 行，得 %d：%s", len(rows), out)
	}
	id := ccLifeCell(rows[0], "id")
	if id == "" {
		t.Fatalf("open 没给茧 id：%s", out)
	}
	if got := ccLifeCell(rows[0], "phase"); got != string(cocoon.PhaseRunning) {
		t.Errorf("open 后 phase=%q 期望 running", got)
	}
	if got := ccLifeCell(rows[0], "backend"); got != cocoonBackendFake {
		t.Errorf("backend=%q 期望 %q（假后端要如实署名）", got, cocoonBackendFake)
	}
	if ports := ccLifeCell(rows[0], "ports"); !strings.Contains(ports, "8000→") {
		t.Errorf("ports=%q 没含 8000→<宿主>（端口没回写 meta）", ports)
	}
	// 读盘对拍：meta 真源真落了 running + 端口。
	m, err := cocoon.Load(cocoon.MetaPath(filepath.Join(root, id)))
	if err != nil {
		t.Fatalf("meta 读不到：%v", err)
	}
	if m.Phase != cocoon.PhaseRunning || len(m.Ports) != 1 {
		t.Fatalf("meta 与输出不符：phase=%s ports=%v", m.Phase, m.Ports)
	}

	// ls：只读 meta 真源。
	rc, out, errs = ccLifeRun(t, cmdCocoonLs, []string{"cocoon", "ls"}, nil,
		[]string{"id", "phase", "ports", "egg"}, nil)
	if rc != exitOK {
		t.Fatalf("ls rc=%d · %s", rc, errs)
	}
	lsRows := ccLifeRows(t, out)
	if len(lsRows) != 1 || ccLifeCell(lsRows[0], "id") != id {
		t.Fatalf("ls 没列出刚起的茧：%s", out)
	}
	if got := ccLifeCell(lsRows[0], "egg"); got != "—" {
		t.Errorf("未绑卵的茧 egg 应印 —，得 %q", got)
	}

	// bind → inspect 看得到。
	rc, _, errs = ccLifeRun(t, cmdCocoonBind, []string{"cocoon", "bind"}, []string{id, "example-35b"},
		[]string{"id", "egg"}, nil)
	if rc != exitOK {
		t.Fatalf("bind rc=%d · %s", rc, errs)
	}
	rc, out, errs = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id}, nil, nil)
	if rc != exitOK {
		t.Fatalf("inspect rc=%d · %s", rc, errs)
	}
	if !strings.Contains(out, "example-35b") {
		t.Fatalf("inspect 没显示绑定的卵：\n%s", out)
	}
	if !strings.Contains(out, cocoon.EnvAIBase) {
		t.Fatalf("绑了卵却没有 AI 注入面（%s 缺席）：\n%s", cocoon.EnvAIBase, out)
	}
	if !strings.Contains(out, "peer_identity") && !strings.Contains(out, "对端身份=none") {
		t.Errorf("inspect 没印对端身份强度（假后端必须是 none，✗ 不冒充 strong）：\n%s", out)
	}

	// close（留盘）→ phase gone，目录还在。
	rc, _, errs = ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{id}, nil, nil)
	if rc != exitOK {
		t.Fatalf("close rc=%d · %s", rc, errs)
	}
	if ph, ok := ccLifeMetaPhase(t, root, id); !ok || ph != cocoon.PhaseGone {
		t.Fatalf("close 后 meta.phase=%q 期望 gone", ph)
	}
	if _, err := os.Stat(filepath.Join(root, id)); err != nil {
		t.Fatalf("close 不带 --purge 应**留盘**，目录却不见了：%v", err)
	}
	// ls 仍列它（留盘 = 真源还在）。
	_, out, _ = ccLifeRun(t, cmdCocoonLs, []string{"cocoon", "ls"}, nil, []string{"id", "phase"}, nil)
	if !strings.Contains(out, id) {
		t.Fatalf("留盘的茧（phase=gone）应仍能从 ls 读到：%s", out)
	}

	// --purge 缺 --confirm ⇒ 退 2 且**零删除**。
	rc, _, errs = ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{id}, nil,
		map[string]string{"--purge": "true"})
	if rc != exitUsage {
		t.Fatalf("`--purge` 缺 confirm rc=%d 期望 2 · %s", rc, errs)
	}
	if _, err := os.Stat(filepath.Join(root, id)); err != nil {
		t.Fatalf("缺 confirm 时目录被删了（**零删除**被破坏）：%v", err)
	}
	// --confirm 值不等 ⇒ 同样拒执、零删除。
	rc, _, _ = ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{id}, nil,
		map[string]string{"--purge": "true", "--confirm": "别的-id"})
	if rc != exitUsage {
		t.Fatalf("`--confirm` 值不等 rc=%d 期望 2", rc)
	}
	if _, err := os.Stat(filepath.Join(root, id)); err != nil {
		t.Fatalf("confirm 不匹配时目录被删了：%v", err)
	}
	// 正确 confirm ⇒ 删净 + 墓碑。
	rc, out, errs = ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{id},
		[]string{"id", "phase", "purged", "deleted_files", "tombstone"}, map[string]string{"--purge": "true", "--confirm": id})
	if rc != exitOK {
		t.Fatalf("--purge 正确档 rc=%d · %s", rc, errs)
	}
	rows = ccLifeRows(t, out)
	if got := ccLifeCell(rows[0], "purged"); got != "true" {
		t.Errorf("purged=%q 期望 true", got)
	}
	if _, err := os.Stat(filepath.Join(root, id)); !os.IsNotExist(err) {
		t.Fatalf("--purge 后目录仍在（err=%v）", err)
	}
	tomb := cocoon.FlatMetaPath(root, id)
	if _, err := os.Stat(tomb); err != nil {
		t.Fatalf("--purge 后墓碑不在（先墓碑再删这一条没落）：%v", err)
	}
	// ls 不再列它（目录没了、meta 成了墓碑）。
	_, out, _ = ccLifeRun(t, cmdCocoonLs, []string{"cocoon", "ls"}, nil, []string{"id"}, nil)
	if strings.Contains(out, id) {
		t.Fatalf("删净的茧仍出现在 ls 里（把墓碑当成了活茧）：%s", out)
	}
}

// ── ③ 用法面（缺参 / 出允许根 / 契约坏 / id 不存在 / purge 缺 confirm）──────

func TestCocoonOpenUsageFace(t *testing.T) {
	proj, root := ccLifeSetup(t)

	// 缺参 / 多参 ⇒ 2，stdout 零字节。
	if rc, out, _ := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, nil, nil, nil); rc != exitUsage || out != "" {
		t.Fatalf("缺参 rc=%d out=%q 期望 2/空", rc, out)
	}
	if rc, _, _ := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj, proj}, nil, nil); rc != exitUsage {
		t.Fatalf("多参 rc=%d 期望 2", rc)
	}
	// 目录不存在 ⇒ 2。
	if rc, _, _ := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"},
		[]string{filepath.Join(proj, "nope")}, nil, nil); rc != exitUsage {
		t.Fatalf("目录不存在 rc=%d 期望 2", rc)
	}
	// 出允许根 ⇒ 2（realpath 后校验）：另起一棵临时树、不给它进允许根。
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, cocoon.SpecFileName), []byte(ccLifeSpec), 0o644); err != nil {
		t.Fatalf("写外部契约失败: %v", err)
	}
	rc, _, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{outside}, nil, nil)
	if rc != exitUsage || !strings.Contains(errs, "允许根") {
		t.Fatalf("出允许根 rc=%d 期望 2 且判词含「允许根」· %s", rc, errs)
	}
	// 符号链接逃逸：链接指向允许根外 ⇒ 同样拒（realpath 之后判）。
	link := filepath.Join(proj, "escape")
	if err := os.Symlink(outside, link); err == nil {
		if rc, _, _ := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{link}, nil, nil); rc != exitUsage {
			t.Fatalf("符号链接逃逸 rc=%d 期望 2", rc)
		}
	}
	// 缺 cocoon.toml ⇒ 1（读不到 ≠ 用法错）。
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(cocoonAllowRootsEnv, filepath.Dir(empty)+string(os.PathListSeparator)+filepath.Dir(proj))
	if rc, _, _ := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{empty}, nil, nil); rc != exitFail {
		t.Fatalf("缺契约 rc=%d 期望 1", rc)
	}
	// 契约非法（明文密钥）⇒ 2。
	bad := filepath.Join(filepath.Dir(empty), "bad")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	badSpec := strings.Replace(ccLifeSpec, "[资源]", "[环境]\nAPI_TOKEN = \"sk-live-123\"\n\n[资源]", 1)
	if err := os.WriteFile(filepath.Join(bad, cocoon.SpecFileName), []byte(badSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, _ := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{bad}, nil, nil); rc != exitUsage {
		t.Fatalf("明文密钥契约 rc=%d 期望 2（解析必拒）", rc)
	}
	// 全程零副作用：上面所有失败档都不该在状态根里留下茧。
	if entries, err := os.ReadDir(root); err == nil && len(entries) != 0 {
		t.Fatalf("失败档在状态根里留了 %d 个条目（应零写入）", len(entries))
	}
}

func TestCocoonCloseUsageFace(t *testing.T) {
	proj, root := ccLifeSetup(t)
	// 起一个真茧（后面验「缺 confirm ⇒ 零删除、phase 不变」）。
	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase"}, nil)
	if rc != exitOK {
		t.Fatalf("open rc=%d · %s", rc, errs)
	}
	id := ccLifeCell(ccLifeRows(t, out)[0], "id")

	if rc, _, _ := ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, nil, nil, nil); rc != exitUsage {
		t.Fatalf("close 缺参 rc=%d 期望 2", rc)
	}
	if rc, _, _ := ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{"../x"}, nil, nil); rc != exitUsage {
		t.Fatalf("close 路径分隔符 id rc=%d 期望 2", rc)
	}
	if rc, _, _ := ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{"no-such-cocoon"}, nil, nil); rc != exitFail {
		t.Fatalf("close 不存在的 id rc=%d 期望 1", rc)
	}
	// --purge 缺 confirm ⇒ 2，且**盘上什么都没动**（phase 仍 running · 目录仍在）。
	before := ccLifeSnap(t, root)
	rc, _, errs = ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{id}, nil,
		map[string]string{"--purge": "true"})
	if rc != exitUsage || !strings.Contains(errs, "--confirm") {
		t.Fatalf("--purge 缺 confirm rc=%d 期望 2 且判词点名 --confirm · %s", rc, errs)
	}
	ccLifeDiffSnap(t, before, ccLifeSnap(t, root))
	if ph, _ := ccLifeMetaPhase(t, root, id); ph != cocoon.PhaseRunning {
		t.Fatalf("缺 confirm 时 meta 被改了：phase=%s", ph)
	}
	// --confirm 值不等 ⇒ 2，同样零副作用。
	rc, _, _ = ccLifeRun(t, cmdCocoonClose, []string{"cocoon", "close"}, []string{id}, nil,
		map[string]string{"--purge": "true", "--confirm": id + "-x"})
	if rc != exitUsage {
		t.Fatalf("confirm 值不等 rc=%d 期望 2", rc)
	}
	ccLifeDiffSnap(t, before, ccLifeSnap(t, root))
}

func TestCocoonInspectUsageFace(t *testing.T) {
	proj, root := ccLifeSetup(t)
	if rc, _, _ := ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, nil, nil, nil); rc != exitUsage {
		t.Fatal("inspect 缺参应退 2")
	}
	if rc, _, _ := ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{"../x"}, nil, nil); rc != exitUsage {
		t.Fatal("inspect 非法 id 应退 2")
	}
	if rc, _, _ := ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{"no-such"}, nil, nil); rc != exitFail {
		t.Fatal("inspect 不存在的 id 应退 1")
	}
	// bind 用法面：缺一个参数 ⇒ 2；空卵名 ⇒ 2。
	if rc, _, _ := ccLifeRun(t, cmdCocoonBind, []string{"cocoon", "bind"}, []string{"x"}, nil, nil); rc != exitUsage {
		t.Fatal("bind 缺参应退 2")
	}
	// 终态的茧不许再绑。
	gone := cocoon.New("gone-1", "", "t")
	if err := gone.SetPhase(cocoon.PhaseGone); err != nil {
		t.Fatalf("造 gone meta 失败: %v", err)
	}
	if err := cocoon.Save(cocoon.MetaPath(filepath.Join(root, "gone-1")), gone); err != nil {
		t.Fatalf("落 gone meta 失败: %v", err)
	}
	if rc, _, _ := ccLifeRun(t, cmdCocoonBind, []string{"cocoon", "bind"}, []string{"gone-1", "example-35b-v2"}, nil, nil); rc != exitFail {
		t.Fatalf("终态茧 bind rc=%d 期望 1", rc)
	}
	_ = proj
}

// ── ⑤ 判据 9（v2 §E）：契约哈希锁 —— open 拒 / inspect 报 ────────────────────
//
// 两格（互为对照：同一份改动，写面拒、观察面报）：
//
//	① `TestCocoonSpecDriftOpenRefused`：起一个茧 → 改一行 `cocoon.toml` → 再 `open` ⇒
//	   退 1 + 判词点「漂移」+ 期望/实得各前 8 位（完整 64 位不许出现）· 状态根零写入 ·
//	   `--dry-run` 同拒 · **改回去能过**（负控：拒的是哈希不符，不是「第二次 open 一律拒」）。
//	② `TestCocoonSpecDriftInspectReports`：同一现场 `inspect` ⇒ **rc=0** 报 `drift` ·
//	   锁定值/现值两格都出 · 整棵树（状态根 + 项目目录）逐件 sha256 前后相同（不写盘）·
//	   改回去 ⇒ `ok` · 没有 `spec_sha` 的老茧 ⇒ `unlocked`（✗ 不当成一致）。

func TestCocoonSpecDriftOpenRefused(t *testing.T) {
	proj, root := ccLifeSetup(t)
	specPath := filepath.Join(proj, cocoon.SpecFileName)

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase"}, nil)
	if rc != exitOK {
		t.Fatalf("首个 open rc=%d · %s", rc, errs)
	}
	id := ccLifeCell(ccLifeRows(t, out)[0], "id")

	// 锁必须真落盘两格：只有 sha 没有目录 ⇒ inspect 回读不到契约（那一格就成摆设）。
	m, err := cocoon.Load(cocoon.MetaPath(filepath.Join(root, id)))
	if err != nil {
		t.Fatalf("meta 读不到：%v", err)
	}
	real, rerr := filepath.EvalSymlinks(proj)
	if rerr != nil {
		t.Fatalf("realpath(%s) 失败: %v", proj, rerr)
	}
	wantSHA, _ := cocoon.LockSpec([]byte(ccLifeSpec))
	if m.SpecSHA != wantSHA {
		t.Fatalf("meta.spec_sha 与契约字节的 sha256 不符：得 %q 要 %q", m.SpecSHA, wantSHA)
	}
	if m.SpecDir != real {
		t.Fatalf("meta.spec_dir 应是契约目录的 realpath %q，得 %q", real, m.SpecDir)
	}

	// 改一行（仍是合法契约：只动资源数 —— 免得拒因落在 spec_invalid 上，那样这一格就没证明漂移）。
	modified := strings.Replace(ccLifeSpec, "mem_gb  = 2", "mem_gb  = 3", 1)
	if modified == ccLifeSpec {
		t.Fatal("夹具没改到（mem_gb 那一行不在 ccLifeSpec 里）")
	}
	if err := os.WriteFile(specPath, []byte(modified), 0o644); err != nil {
		t.Fatal(err)
	}
	nowSHA, _ := cocoon.LockSpec([]byte(modified))
	if nowSHA == wantSHA {
		t.Fatal("改了一行，锁值却没变（漂移判据会全哑）")
	}

	before := ccLifeSnap(t, root)
	rc, out, errs = ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj}, nil, nil)
	if rc != exitFail {
		t.Fatalf("改过契约再 open 应退 1，得 rc=%d · stderr=%s", rc, errs)
	}
	if out != "" {
		t.Fatalf("被拒的 open 不该往 stdout 写东西（会被人当成功读数）：%q", out)
	}
	for _, want := range []string{"漂移", cocoon.SpecShort(wantSHA), cocoon.SpecShort(nowSHA)} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr 没含 %q（期望与实得的前 8 位都要印）：%s", want, errs)
		}
	}
	// ★ 完整 64 位一个都不许出现（指纹不进 stderr）。
	for _, full := range []string{wantSHA, nowSHA} {
		if strings.Contains(errs, full) {
			t.Fatalf("stderr 里出现了完整哈希 %q：%s", full, errs)
		}
	}
	// 零副作用：拒在分配 id / 写 meta 之前 ⇒ 状态根一个条目都不许多。
	ccLifeDiffSnap(t, before, ccLifeSnap(t, root))
	if entries, rerr := os.ReadDir(root); rerr != nil || len(entries) != 1 {
		t.Fatalf("被拒的 open 在状态根里留了条目（应零写入）：%d 条 · %v", len(entries), rerr)
	}

	// `--dry-run` 同拒：干跑放行、真跑才拒 ⇒ 计划件就是在骗人。
	if rc, _, _ := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj}, nil,
		map[string]string{"--dry-run": "true"}); rc != exitFail {
		t.Fatalf("漂移时 `--dry-run` 也须退 1（干跑与真跑同一判据），得 %d", rc)
	}

	// 负控：改回原样 ⇒ 能过。证明「拒」是哈希不符，不是「第二次 open 一律拒」。
	if err := os.WriteFile(specPath, []byte(ccLifeSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errs = ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id", "phase"}, nil)
	if rc != exitOK {
		t.Fatalf("契约改回原样后 open 应放行，得 rc=%d · %s", rc, errs)
	}
	if got := ccLifeCell(ccLifeRows(t, out)[0], "id"); got == id || got == "" {
		t.Fatalf("第二次 open 应分配新 id（得 %q，首个是 %q）", got, id)
	}
}

func TestCocoonSpecDriftInspectReports(t *testing.T) {
	proj, root := ccLifeSetup(t)
	specPath := filepath.Join(proj, cocoon.SpecFileName)

	rc, out, errs := ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj},
		[]string{"id"}, nil)
	if rc != exitOK {
		t.Fatalf("open rc=%d · %s", rc, errs)
	}
	id := ccLifeCell(ccLifeRows(t, out)[0], "id")

	m, err := cocoon.Load(cocoon.MetaPath(filepath.Join(root, id)))
	if err != nil {
		t.Fatalf("meta 读不到：%v", err)
	}
	locked := m.SpecSHA
	if locked == "" {
		t.Fatal("open 没把 spec_sha 写进 meta（判据 9 的锁值没落盘）")
	}

	// 一致时先验 ok（正控：drift 不是写死的）。
	rc, out, errs = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id},
		[]string{"spec_lock", "spec_sha"}, nil)
	if rc != exitOK {
		t.Fatalf("inspect rc=%d · %s", rc, errs)
	}
	if got := ccLifeCell(ccLifeRows(t, out)[0], "spec_lock"); got != "ok" {
		t.Fatalf("契约未改时 spec_lock 应为 ok，得 %q：%s", got, out)
	}

	// 改一行。
	modified := strings.Replace(ccLifeSpec, "mem_gb  = 2", "mem_gb  = 3", 1)
	if err := os.WriteFile(specPath, []byte(modified), 0o644); err != nil {
		t.Fatal(err)
	}
	nowSHA, _ := cocoon.LockSpec([]byte(modified))

	// ★ 快照两棵树：状态根 + 项目目录（inspect 一句「只读」要按整树对拍，不是看单个文件）。
	rootBefore := ccLifeSnap(t, root)
	projBefore := ccLifeSnap(t, proj)

	rc, out, errs = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id},
		[]string{"spec_lock", "spec_sha", "spec_now"}, nil)
	if rc != exitOK {
		t.Fatalf("inspect 报漂移时必须 rc=0（观察面不拒：拒了就没法看别的），得 %d · %s", rc, errs)
	}
	row := ccLifeRows(t, out)[0]
	if got := ccLifeCell(row, "spec_lock"); got != "drift" {
		t.Fatalf("spec_lock=%q 期望 drift：%s", got, out)
	}
	if got := ccLifeCell(row, "spec_sha"); got != locked {
		t.Errorf("spec_sha=%q 期望锁定值 %q", got, locked)
	}
	if got := ccLifeCell(row, "spec_now"); got != nowSHA {
		t.Errorf("spec_now=%q 期望现值 %q", got, nowSHA)
	}
	// 不写盘：前后整树逐件 sha256 相同（连 meta.json.lock 都不许被建出来）。
	ccLifeDiffSnap(t, rootBefore, ccLifeSnap(t, root))
	ccLifeDiffSnap(t, projBefore, ccLifeSnap(t, proj))
	if _, err := os.Stat(cocoon.MetaPath(filepath.Join(root, id)) + ".lock"); err != nil {
		// 锁件是 open 时 Save 建的（本来就该在）—— 这里只确认 inspect 没把它「重建」成异常形态。
		t.Logf("（锁件状态：%v）", err)
	}

	// 人面也报（不带 --json）：只印前 8 位，不往终端抄指纹。
	rc, out, _ = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id}, nil, nil)
	if rc != exitOK || !strings.Contains(out, "漂移") {
		t.Fatalf("人面没报漂移（rc=%d）：\n%s", rc, out)
	}
	if strings.Contains(out, locked) {
		t.Fatalf("人面印了完整锁定哈希（只许前 8 位）：\n%s", out)
	}
	if !strings.Contains(out, cocoon.SpecShort(locked)) || !strings.Contains(out, cocoon.SpecShort(nowSHA)) {
		t.Fatalf("人面没印两枚前 8 位：\n%s", out)
	}

	// 负控①：改回去 ⇒ ok（drift 会跟着内容走，不是一次性的）。
	if err := os.WriteFile(specPath, []byte(ccLifeSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	_, out, _ = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id},
		[]string{"spec_lock"}, nil)
	if got := ccLifeCell(ccLifeRows(t, out)[0], "spec_lock"); got != "ok" {
		t.Fatalf("契约改回后 spec_lock 应回 ok，得 %q", got)
	}

	// 负控②：没有 spec_sha 的老茧（脚本写入 / 老版本）⇒ unlocked，✗ 不许冒充一致。
	legacy := "legacy-1"
	lm := cocoon.New(legacy, "", "t")
	if err := lm.SetPhase(cocoon.PhaseRunning); err != nil {
		t.Fatal(err)
	}
	if err := cocoon.Save(cocoon.MetaPath(filepath.Join(root, legacy)), lm); err != nil {
		t.Fatal(err)
	}
	_, out, _ = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{legacy},
		[]string{"spec_lock", "spec_sha"}, nil)
	lrow := ccLifeRows(t, out)[0]
	if got := ccLifeCell(lrow, "spec_lock"); got != "unlocked" {
		t.Fatalf("没有 spec_sha 的老茧 spec_lock 应为 unlocked（✗ 不当成一致），得 %q", got)
	}
	if got := ccLifeCell(lrow, "spec_sha"); got != "—" {
		t.Fatalf("老茧 spec_sha 应印 —（三值纪律），得 %q", got)
	}
}

// ── ④ 只读面（inspect 不写盘 · 令牌只出脱敏 · ls 不列孤儿 · dry-run 零副作用）──

func TestCocoonInspectReadOnly(t *testing.T) {
	_, root := ccLifeSetup(t)
	id := "manual-1"
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := cocoon.New(id, "", "t")
	if err := m.SetPhase(cocoon.PhaseRunning); err != nil {
		t.Fatal(err)
	}
	m.PID = 4242
	m.Ports = map[int]int{8000: 20000}
	if err := cocoon.Save(cocoon.MetaPath(dir), m); err != nil {
		t.Fatal(err)
	}
	// 删掉锁件，验「inspect 自己不许把它建回来」（`cocoon.Load` 会 O_CREATE）。
	_ = os.Remove(cocoon.MetaPath(dir) + ".lock")

	before := ccLifeSnap(t, root)
	rc, out, errs := ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id},
		[]string{"id", "phase", "pid", "ports", "token"}, nil)
	if rc != exitOK {
		t.Fatalf("inspect rc=%d · %s", rc, errs)
	}
	ccLifeDiffSnap(t, before, ccLifeSnap(t, root)) // 一个文件都不许多 / 少
	if _, err := os.Stat(cocoon.MetaPath(dir) + ".lock"); !os.IsNotExist(err) {
		t.Fatal("inspect 把锁件建出来了（只读面写盘）")
	}
	if got := ccLifeCell(ccLifeRows(t, out)[0], "pid"); got != "4242" {
		t.Errorf("pid=%q 期望 4242", got)
	}

	// 令牌面：给一枚真值 ⇒ 只许出脱敏形。
	raw := strings.Repeat("Ab3", 14) + "Zz" // 44 字符
	if len(raw) < 20 {
		t.Fatal("夹具令牌太短")
	}
	t.Setenv(cocoonTokenEnv, raw)
	rc, out, errs = ccLifeRun(t, cmdCocoonInspect, []string{"cocoon", "inspect"}, []string{id},
		[]string{"token", "token_source"}, nil)
	if rc != exitOK {
		t.Fatalf("inspect(token) rc=%d · %s", rc, errs)
	}
	if strings.Contains(out, raw) {
		t.Fatalf("**令牌真值进了 stdout**（只许脱敏形）：%s", out)
	}
	wantRedacted := cocoon.RedactToken(raw)
	if !strings.Contains(out, wantRedacted) {
		t.Fatalf("脱敏形 %q 不在输出里：%s", wantRedacted, out)
	}
	if got := ccLifeCell(ccLifeRows(t, out)[0], "token_source"); !strings.HasPrefix(got, "env:") {
		t.Errorf("token_source=%q 期望 env:…", got)
	}
}

func TestCocoonLsDoesNotListOrphans(t *testing.T) {
	_, root := ccLifeSetup(t)
	// 孤儿：有目录、无 meta（reconcile 的活，ls 不许把它当成茧）。
	orphan := filepath.Join(root, "orphan-1")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphan, "junk.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 坏账：meta 是半截 JSON。
	broken := filepath.Join(root, "broken-1")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cocoon.MetaPath(broken), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 真茧：一册在盘的 meta。
	live := filepath.Join(root, "live-1")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	m := cocoon.New("live-1", "example-35b-v2", "t")
	if err := m.SetPhase(cocoon.PhaseRunning); err != nil {
		t.Fatal(err)
	}
	if err := cocoon.Save(cocoon.MetaPath(live), m); err != nil {
		t.Fatal(err)
	}

	rc, out, errs := ccLifeRun(t, cmdCocoonLs, []string{"cocoon", "ls"}, nil,
		[]string{"id", "phase", "egg"}, nil)
	if rc != exitOK {
		t.Fatalf("ls rc=%d · %s", rc, errs)
	}
	rows := ccLifeRows(t, out)
	if len(rows) != 1 || ccLifeCell(rows[0], "id") != "live-1" {
		t.Fatalf("ls 清单应只有 live-1，得 %s", out)
	}
	for _, bad := range []string{"orphan-1", "broken-1"} {
		if strings.Contains(out, bad) {
			t.Fatalf("ls 把 %s 当成了茧（孤儿/坏账只归 reconcile）：%s", bad, out)
		}
	}
	if !strings.Contains(errs, "跳过") {
		t.Errorf("孤儿/坏账被静默吞了（应在 stderr 报一个计数）：%q", errs)
	}

	// `open --dry-run` 零副作用：状态根一个条目都不许多。
	proj, _ := ccLifeSetup(t)
	freshRoot := filepath.Join(t.TempDir(), "fresh")
	t.Setenv(cocoonRootEnv, freshRoot)
	proj2 := filepath.Join(filepath.Dir(freshRoot), "proj2")
	if err := os.MkdirAll(proj2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj2, cocoon.SpecFileName), []byte(ccLifeSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(cocoonAllowRootsEnv, filepath.Dir(freshRoot))
	rc, out, _ = ccLifeRun(t, cmdCocoonOpen, []string{"cocoon", "open"}, []string{proj2}, nil,
		map[string]string{"--dry-run": "true"})
	if rc != exitOK {
		t.Fatalf("open --dry-run rc=%d 期望 0", rc)
	}
	if !strings.Contains(out, "计划件") {
		t.Fatalf("--dry-run 没出计划件：%s", out)
	}
	if _, err := os.Stat(freshRoot); !os.IsNotExist(err) {
		t.Fatalf("--dry-run 建了状态根（零副作用被破坏）：%v", err)
	}
	_ = proj
}
