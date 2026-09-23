// cli_model_add_test.go —— `model add`（`Q-099`）+ `config reload`（`Q-100`）的判据机检（波① `T1a`）。
//
// 判据（`任务清单-缺口收口-20260923.md` §自排补遗 `S1` 的两条形状 · 逐条落成断言）：
//
//	`model add`     含 `--dry-run` ✓ 先校验后写 ✓ ⇒ **干跑零字节改动**（现跑 sha256 对拍）·
//	                真写只有**该块该增的那几行**动（其余逐字节不变）· 写完读回再校 · 任一步不过 ⇒ **回滚**
//	`config reload` 照 `nginx -s reload`：**先校验、失败回滚** ⇒ 名册件解析不过时 **一个请求都不发**
//
// 块定位与字段语义（`Q-099` 修面 · 2026-09-24）——本件的判据面：
//
//	块按 `--model`（**模型名** · `models:` 段的键）定位/新建；`--host` 只作该条 `host:` 的字段值。
//	两种正规形态都要能写：**列表形**（`  <名>:` + 条目行）与**裸映射形**（`  <名>: { … }`），
//	且**不新造第三种**（写回时该块保持/退化成两种正规形态之一）。
//	成对负控：空 `--model` / 块名不是标识符（不许新建）/ `models:` 段缺失 / 认不出的块形态 /
//	`--host` 值不在名册 / 重复 file / 坏档 / `--json` 不给字段 / 缺 `--yes` ——
//	逐格断言「退码对 **且** 文件逐字节没动」。
//
// 夹具纪律：全在 `t.TempDir()` 里合成，**绝不碰真 `gateway/fleet.yaml`**（它身上有 Mr2109 的在途改动）
// —— 正控各格另外现场对拍真名册的 sha256（`repoFleetSHA`），把「夹具不碰真件」也钉住。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fleetFixture = `# 合成夹具（判据件不碰真名册）
auth:
  token: ""

models:
  probe-list:
    - { host: Mr2109, backend: llama-server, file: "/models/old-a.gguf", mem_gb: 6, ctx_window: 8192 }
    - { host: Mr2109, backend: llama-server, file: "/models/old-b.gguf", mem_gb: 8, ctx_window: 16384, added: "2026-09-01" }
  probe-bare: { host: x3, backend: ds4-server, file: "/data/models/old-c.gguf", mem_gb: 18, ctx_window: 131072 }

aliases:
  zerg-a: old-a

fleet:
  Mr2109:
    host: Mr2109
    port: 8580
  x3:
    host: <worker-ip>
    port: 8100
`

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s 不过：%v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func fleetFixtureAt(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "fleet.yaml")
	if err := os.WriteFile(path, []byte(fleetFixture), 0o644); err != nil {
		t.Fatalf("写夹具不过：%v", err)
	}
	return dir, path
}

// repoFleetSHA —— 真名册件（仓内 `gateway/fleet.yaml`）的 sha256；找不到就返回 ""（判据件跳过这一格）。
//
// 为什么还要这一格：本件所有命令都带 `--path <临时夹具>`，但**判据要说清「我没碰真件」** ——
// 记性靠不住，对拍靠得住。
func repoFleetSHA(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 8; i++ {
		p := filepath.Join(dir, "gateway", "fleet.yaml")
		if _, err := os.Stat(p); err == nil {
			return fileSHA(t, p)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// writeFixture —— 任意文本的临时名册件（夹具一律在 `t.TempDir()` 里）。
func writeFixture(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fleet.yaml")
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatalf("写夹具不过：%v", err)
	}
	return p
}

// linesOf —— 按 `\n` 切行（末尾换行不算一行），与实现侧的切法一致。
func linesOf(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

// planLine —— 取计划件里以某前缀开头的那一行（计划件的行式面判据都靠它）。
func planLine(out, prefix string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			return l
		}
	}
	return ""
}

// 判据面 · ① 干跑零字节改动 + 计划件里有那一行（**块不存在 ⇒ 出新建计划**）。
func TestModelAddDryRunZeroSideEffect(t *testing.T) {
	_, path := fleetFixtureAt(t)
	repoBefore := repoFleetSHA(t)
	before := fileSHA(t, path)
	rc, out, errb := runCapture("model", "add", "--path", path, "--model", "new-1", "--host", "Mr2109",
		"--file", "/models/new-1.gguf", "--mem-gb", "6", "--ctx", "16384",
		"--arch", "qwen3", "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "零副作用") {
		t.Fatalf("干跑 stderr 里没有「零副作用」判词：%s", errb)
	}
	if !strings.Contains(out, `/models/new-1.gguf`) {
		t.Fatalf("计划件里没有那一行：%s", out)
	}
	if blk := planLine(out, "块"); !strings.Contains(blk, "new-1") || !strings.Contains(blk, "新建块") {
		t.Fatalf("计划件没报「块按 --model 定位/新建」：%s", blk)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("干跑改了文件：%s → %s", before, after)
	}
	if repoBefore != "" {
		if after := repoFleetSHA(t); after != repoBefore {
			t.Fatalf("真名册件被动了（本件只许碰 --path 指的临时夹具）：%s → %s", repoBefore, after)
		}
	}
}

// 判据面 · ② **两形态 + 新建**三种定位：干跑各出计划（rc=0 · 零字节改动），且计划件**点名块与形态**。
//
// 这一格就是修面的主判据（改前：传真主机名 ⇒ rc=2；在册模型名当 `--host` ⇒ rc=2 且说的是「没有那一块」）。
func TestModelAddDryRunLocatesBothShapesAndNew(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	for _, c := range []struct{ name, model, wantForm string }{
		{"列表形块（改到）", "probe-list", "列表形"},
		{"裸映射形块（改到）", "probe-bare", "裸映射形"},
		{"块不存在（新建）", "probe-new", "新建块"},
	} {
		rc, out, errb := runCapture("model", "add", "--path", path, "--model", c.model,
			"--host", "Mr2109", "--file", "/models/"+c.model+"-Mr2109.gguf", "--dry-run")
		if rc != 0 {
			t.Fatalf("%s：干跑 rc=%d（要 0）· stderr=%s", c.name, rc, errb)
		}
		blk := planLine(out, "块")
		if !strings.Contains(blk, "`"+c.model+":`") || !strings.Contains(blk, c.wantForm) {
			t.Fatalf("%s：计划件没点名块/形态（要 `%s:` + %s）：%s", c.name, c.model, c.wantForm, blk)
		}
		if !strings.Contains(out, "`"+c.model+":`") {
			t.Fatalf("%s：计划件没点名 `--model` 定的那一块：%s", c.name, out)
		}
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("三格干跑里有格子改了文件：%s → %s", before, after)
	}
}

// 判据面 · ③ 真写（**列表形**块）：只有一行新增（其余逐字节不变）+ 写完解析过（读回再校）+ 3 条在册。
func TestModelAddListShapeWriteOneLineAdded(t *testing.T) {
	_, path := fleetFixtureAt(t)
	repoBefore := repoFleetSHA(t)
	rc, out, errb := runCapture("model", "add", "--path", path, "--model", "probe-list",
		"--host", "x3", "--file", "/models/new-x3.gguf", "--mem-gb", "18", "--ctx", "131072",
		"--arch", "qwen3", "--desc", "夹具新条", "--added", "2026-09-24", "--yes")
	if rc != 0 {
		t.Fatalf("真写 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "probe-list") {
		t.Fatalf("行式面里没有模型名：%s", out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回不过：%v", err)
	}
	oldLines, newLines := linesOf(fleetFixture), linesOf(string(got))
	if len(newLines) != len(oldLines)+1 {
		t.Fatalf("行数 %d → %d（要 +1 —— 列表形块只许续一行）", len(oldLines), len(newLines))
	}
	if !strings.Contains(string(got), `/models/new-x3.gguf`) {
		t.Fatalf("新条没写进去：%s", got)
	}
	// 逐字节：删掉新插入的那一行，其余必须与原档**逐字相同**（新增式判据）
	at := -1
	for i, l := range newLines {
		if strings.Contains(l, "/models/new-x3.gguf") {
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("找不到新行的位置")
	}
	if !strings.HasPrefix(newLines[at], "    - { host: x3,") {
		t.Fatalf("新条不是列表形条目（缩进/形状不对）：%q", newLines[at])
	}
	restored := append(append([]string{}, newLines[:at]...), newLines[at+1:]...)
	if strings.Join(restored, "\n") != strings.Join(oldLines, "\n") {
		t.Fatalf("除了新增那一行，其余**不是**逐字节相同 ⇒ 不是新增式")
	}
	// 写面报的行号必须是**真行号**（报错行 = 写面说谎）
	if !strings.Contains(errb, fmt.Sprintf("新条第 %d 行", at+1)) {
		t.Fatalf("报的落点行号不对（真行号 = %d）：%s", at+1, errb)
	}
	// 读回再校：对**改后**的档再干跑一次，`probe-list` 下要能看到 3 条（解析器认了）
	rc2, out2, errb2 := runCapture("model", "add", "--path", path, "--model", "probe-list",
		"--host", "Mr2109", "--file", "/models/again.gguf", "--dry-run")
	if rc2 != 0 || !strings.Contains(out2, "`probe-list` 下 3 条") {
		t.Fatalf("改后档没解析成 3 条：rc=%d out=%s stderr=%s", rc2, out2, errb2)
	}
	if repoBefore != "" {
		if after := repoFleetSHA(t); after != repoBefore {
			t.Fatalf("真名册件被动了：%s → %s", repoBefore, after)
		}
	}
}

// 判据面 · ④ 真写（**裸映射形**块）：展开成**列表形**（不新造第三种形态）——原条正文逐字保留 + 新条紧随。
func TestModelAddBareShapeExpandsToList(t *testing.T) {
	_, path := fleetFixtureAt(t)
	rc, out, errb := runCapture("model", "add", "--path", path, "--model", "probe-bare",
		"--host", "Mr2109", "--file", "/models/new-bare-Mr2109.gguf", "--mem-gb", "18", "--yes")
	if rc != 0 {
		t.Fatalf("真写（裸映射形块）rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "probe-bare") {
		t.Fatalf("行式面里没有模型名：%s", out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回不过：%v", err)
	}
	oldLines, newLines := linesOf(fleetFixture), linesOf(string(got))
	if len(newLines) != len(oldLines)+2 {
		t.Fatalf("行数 %d → %d（要 +2：原条那一行拆成 3 行 ⇒ 净增 2）", len(oldLines), len(newLines))
	}
	// 原条正文**逐字**搬进列表形条目（只换前缀），键那一行也在
	if !strings.Contains(string(got), "    - { host: x3, backend: ds4-server, file: \"/data/models/old-c.gguf\", mem_gb: 18, ctx_window: 131072 }") {
		t.Fatalf("原条正文没有逐字保留成列表形条目：%s", got)
	}
	if !strings.Contains(string(got), "\n  probe-bare:\n") {
		t.Fatalf("块首没有落成列表形（`  probe-bare:`）：%s", got)
	}
	if strings.Contains(string(got), "  probe-bare: { ") {
		t.Fatalf("块仍是同行裸映射形（没展开 ⇒ 两条写不进同一块）：%s", got)
	}
	// 逐字节：把这次动的两行回调成原形，其余必须与原档逐字相同
	var restored []string
	for _, l := range newLines {
		switch {
		case strings.Contains(l, "/models/new-bare-Mr2109.gguf"):
			continue // 新增条
		case strings.HasPrefix(l, "    - { host: x3, backend: ds4-server, file: \"/data/models/old-c.gguf\""):
			restored = append(restored, `  probe-bare: { host: x3, backend: ds4-server, file: "/data/models/old-c.gguf", mem_gb: 18, ctx_window: 131072 }`)
		case strings.TrimRight(l, " ") == "  probe-bare:":
			continue // 展开时新生的键那一行（由上面那一行还原）
		default:
			restored = append(restored, l)
		}
	}
	if strings.Join(restored, "\n") != strings.Join(oldLines, "\n") {
		t.Fatalf("除展开那一处外，其余**不是**逐字节相同：\n%s", strings.Join(restored, "\n"))
	}
	// 写面报的行号必须是**真行号**
	at := -1
	for i, l := range newLines {
		if strings.Contains(l, "/models/new-bare-Mr2109.gguf") {
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("找不到新行的位置")
	}
	if !strings.Contains(errb, fmt.Sprintf("新条第 %d 行", at+1)) {
		t.Fatalf("报的落点行号不对（真行号 = %d）：%s", at+1, errb)
	}
	// 读回再校：该块现在 2 条
	rc2, out2, errb2 := runCapture("model", "add", "--path", path, "--model", "probe-bare",
		"--host", "x3", "--file", "/models/again.gguf", "--dry-run")
	if rc2 != 0 || !strings.Contains(out2, "`probe-bare` 下 2 条") {
		t.Fatalf("改后档 `probe-bare` 没解析成 2 条：rc=%d out=%s stderr=%s", rc2, out2, errb2)
	}
}

// 判据面 · ⑤ 真写（**新建块**）：块首 + 新条两行（其余逐字节不变）· 落在 `models:` 段末 · 报的行号是真的。
func TestModelAddNewBlockRealWriteTwoLinesAdded(t *testing.T) {
	_, path := fleetFixtureAt(t)
	rc, out, errb := runCapture("model", "add", "--path", path, "--model", "probe-fresh",
		"--host", "x3", "--file", "/models/fresh.gguf", "--mem-gb", "17", "--ctx", "131072", "--yes")
	if rc != 0 {
		t.Fatalf("真写（新建块）rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "probe-fresh") {
		t.Fatalf("行式面里没有模型名：%s", out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回不过：%v", err)
	}
	oldLines, newLines := linesOf(fleetFixture), linesOf(string(got))
	if len(newLines) != len(oldLines)+2 {
		t.Fatalf("行数 %d → %d（要 +2：块首 + 条目）", len(oldLines), len(newLines))
	}
	// 新增的两行：块首 + 列表形条目，且**落在 `models:` 段末**（`probe-bare` 之后、`aliases:` 之前）
	hdr, at := -1, -1
	for i, l := range newLines {
		if l == "  probe-fresh:" {
			hdr = i
		}
		if strings.Contains(l, "/models/fresh.gguf") {
			at = i
		}
	}
	if hdr < 0 || at != hdr+1 {
		t.Fatalf("新建块的两行没接在一起：块首下标=%d 条目下标=%d", hdr, at)
	}
	if !strings.HasPrefix(newLines[at], "    - { host: x3,") {
		t.Fatalf("新条不是列表形条目：%q", newLines[at])
	}
	lastModels := -1
	for i, l := range newLines {
		if strings.Contains(l, `  probe-bare: { host: x3`) {
			lastModels = i
		}
	}
	if lastModels < 0 || hdr != lastModels+1 {
		t.Fatalf("新块没落在 `models:` 段末（前一条下标=%d · 块首下标=%d）", lastModels, hdr)
	}
	// 逐字节：删掉新增那两行，其余必须与原档逐字相同
	restored := append(append([]string{}, newLines[:hdr]...), newLines[at+1:]...)
	if strings.Join(restored, "\n") != strings.Join(oldLines, "\n") {
		t.Fatalf("除新增那两行外，其余**不是**逐字节相同")
	}
	// 写面报的行号必须是**真行号**
	if !strings.Contains(errb, fmt.Sprintf("新条第 %d 行", at+1)) {
		t.Fatalf("报的落点行号不对（真行号 = %d）：%s", at+1, errb)
	}
	// 读回再校：新块 1 条
	rc2, out2, errb2 := runCapture("model", "add", "--path", path, "--model", "probe-fresh",
		"--host", "Mr2109", "--file", "/models/again.gguf", "--dry-run")
	if rc2 != 0 || !strings.Contains(out2, "`probe-fresh` 下 1 条") {
		t.Fatalf("新建块没解析成 1 条：rc=%d out=%s stderr=%s", rc2, out2, errb2)
	}
}

// 判据面 · ⑥ 名册件**末行无换行**时：写回也不给它添一个（「其余逐字节不变」含末换行这一字节）。
//
// 为什么要立：真名册件 `gateway/fleet.yaml` 的末行就是无换行的（`git diff` 里 `\ No newline at end of file`）
// —— 夹具若一律以换行结尾，这条就会漏（真写多出第二处改动，人得自己发现）。
func TestModelAddPreservesMissingTrailingNewline(t *testing.T) {
	text := strings.TrimRight(fleetFixture, "\n")
	p := writeFixture(t, text)
	rc, _, errb := runCapture("model", "add", "--path", p, "--model", "probe-list", "--host", "x3",
		"--file", "/models/tail.gguf", "--yes")
	if rc != 0 {
		t.Fatalf("真写 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读回不过：%v", err)
	}
	if strings.HasSuffix(string(got), "\n") {
		t.Fatalf("写回给名册件添了末换行（原档没有）⇒ 「其余逐字节不变」破功")
	}
	oldLines, newLines := linesOf(text), linesOf(string(got))
	if len(newLines) != len(oldLines)+1 {
		t.Fatalf("行数 %d → %d（要 +1）", len(oldLines), len(newLines))
	}
	at := -1
	for i, l := range newLines {
		if strings.Contains(l, "/models/tail.gguf") {
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("找不到新行的位置")
	}
	restored := append(append([]string{}, newLines[:at]...), newLines[at+1:]...)
	if strings.Join(restored, "\n") != strings.Join(oldLines, "\n") {
		t.Fatalf("除新增那一行外，其余**不是**逐字节相同")
	}
}

// 反例探针〇 · 空 `--model` ⇒ 2 且**文件逐字节没动**（安全网：块名是必给的那一枚）。
func TestModelAddNegativeEmptyModel(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, out, errb := runCapture("model", "add", "--path", path, "--host", "Mr2109",
		"--file", "/models/m.gguf", "--yes")
	if rc != 2 {
		t.Fatalf("空 --model：rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Fatalf("用法错时 stdout 该空，得到 %q", out)
	}
	if !strings.Contains(errb, "--model") {
		t.Fatalf("判词没点名缺的是 --model：%s", errb)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("被拒了却改了文件")
	}
}

// 反例探针一 · 块不存在 **且不许新建**（块名不是标识符 ⇒ 当不了 `models:` 的键）⇒ 2 且不动。
func TestModelAddNegativeBlockAbsentNotCreatable(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, _, errb := runCapture("model", "add", "--path", path, "--model", "bad name: x",
		"--host", "Mr2109", "--file", "/models/m.gguf", "--yes")
	if rc != 2 || !strings.Contains(errb, "--model") {
		t.Fatalf("非法块名：rc=%d stderr=%s（要 2 + 点名 --model）", rc, errb)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("被拒了却改了文件")
	}
	// 同一格的另一半：`models:` 段整个不在 ⇒ 也**不代劳新建**（退非 0 · 不动）
	p2 := writeFixture(t, "auth:\n  token: \"\"\n\nfleet:\n  Mr2109: { host: Mr2109, port: 8580 }\n")
	b2 := fileSHA(t, p2)
	rc2, _, errb2 := runCapture("model", "add", "--path", p2, "--model", "probe-x",
		"--host", "Mr2109", "--file", "/models/m.gguf", "--yes")
	if rc2 != 2 || !strings.Contains(errb2, "`models:` 段") {
		t.Fatalf("没有 `models:` 段：rc=%d stderr=%s（要 2 + 点名段）", rc2, errb2)
	}
	if after := fileSHA(t, p2); after != b2 {
		t.Fatalf("没有 `models:` 段却改了文件")
	}
}

// 反例探针二 · 块形态**认不出**（不是列表形也不是同行裸映射形）⇒ 退非 0 且不动（不猜）。
func TestModelAddNegativeUnknownBlockShape(t *testing.T) {
	p := writeFixture(t, "models:\n  probe-weird:\n    host: x3\n    file: /data/models/w.gguf\n\nfleet:\n  x3: { host: <worker-ip>, port: 8100 }\n")
	before := fileSHA(t, p)
	rc, _, errb := runCapture("model", "add", "--path", p, "--model", "probe-weird",
		"--host", "x3", "--file", "/data/models/w2.gguf", "--yes")
	if rc == 0 {
		t.Fatalf("认不出的块形态却 rc=0（要非 0 —— 不许猜）")
	}
	if !strings.Contains(errb, "不猜") {
		t.Fatalf("判词里没有「不猜」：%s", errb)
	}
	if after := fileSHA(t, p); after != before {
		t.Fatalf("被拒了却改了文件")
	}
}

// 反例探针三 · `--host` 的值不在名册（`fleet:` 节点 ∪ 现存条目的 `host:`）⇒ 2 且不动（规则 E 现核）。
func TestModelAddNegativeHostNotInRoster(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, _, errb := runCapture("model", "add", "--path", path, "--model", "probe-list", "--host", "deepseek-v4-flash",
		"--file", "/models/m.gguf", "--yes")
	if rc != 2 || !strings.Contains(errb, "不在名册") {
		t.Fatalf("主机不在名册：rc=%d stderr=%s（要 2 + 「不在名册」）", rc, errb)
	}
	if !strings.Contains(errb, "Mr2109") || !strings.Contains(errb, "x3") {
		t.Fatalf("判词没列出在册主机：%s", errb)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("被拒了却改了文件")
	}
}

// 反例探针四 · 同一模型块下重复 `file` ⇒ 2 且不动（不覆盖别人的条）。
func TestModelAddNegativeDuplicateFile(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, _, errb := runCapture("model", "add", "--path", path, "--model", "probe-list", "--host", "Mr2109",
		"--file", "/models/old-a.gguf", "--yes")
	if rc != 2 || !strings.Contains(errb, "不覆盖") {
		t.Fatalf("重复 file：rc=%d stderr=%s（要 2 + 「不覆盖」）", rc, errb)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("被拒了却改了文件")
	}
}

// 反例探针五 · 缺 `--yes` ⇒ 2 且**文件逐字节没动**（fail-closed · D2 档）。
func TestModelAddRequiresYes(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, out, errb := runCapture("model", "add", "--path", path, "--model", "new-1", "--host", "Mr2109",
		"--file", "/models/new-1.gguf")
	if rc != 2 {
		t.Fatalf("缺 --yes rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Fatalf("缺 --yes 时 stdout 该是空的，得到 %q", out)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("缺 --yes 却改了文件：%s → %s", before, after)
	}
}

// 反例探针六 · 现档就是坏 YAML ⇒ 1 且**不写**（先校验后写的用处）。
func TestModelAddNegativeBadYAMLStaysUntouched(t *testing.T) {
	p := writeFixture(t, "models:\n  probe-x:\n    - { host: Mr2109, file: \"/x.gguf\"\n") // 少一个 `}`
	before := fileSHA(t, p)
	rc, out, _ := runCapture("model", "add", "--path", p, "--model", "probe-x",
		"--host", "Mr2109", "--file", "/models/m.gguf", "--yes")
	if rc != 1 {
		t.Fatalf("坏档 rc=%d（要 1）", rc)
	}
	if out != "" {
		t.Fatalf("坏档时 stdout 该空，得到 %q", out)
	}
	if after := fileSHA(t, p); after != before {
		t.Fatalf("坏档被改了（先校验后写破功）")
	}
}

// 反例探针七 · `--json` 不给字段 ⇒ 退码取自退码表（`usage` · 归一后 = 2）且 stdout 0 字节（K2 四件套）。
func TestModelAddJSONNoFields(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	wantK2 := usageCodeFromTable(t)
	rc, out, _ := runCapture("model", "add", "--path", path, "--model", "probe-list", "--host", "Mr2109",
		"--file", "/models/m.gguf", "--json")
	if rc != wantK2 || out != "" {
		t.Fatalf("`--json` 不给字段：rc=%d stdout=%q（要 %d + 空）", rc, out, wantK2)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("只读用法面却改了文件")
	}
}

// `config reload` · ① 干跑：只校验、**一个请求都不发**（连主控都不用起 ⇒ 就是这个判据的牙）。
func TestConfigReloadDryRunSendsNoRequest(t *testing.T) {
	_, path := fleetFixtureAt(t)
	rc, out, errb := runCapture("config", "reload", "--path", path, "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "POST") || !strings.Contains(errb, "零副作用") {
		t.Fatalf("干跑计划件不完整：out=%s err=%s", out, errb)
	}
	// `models:` 段的一级键是**模型名**（不是机器名）⇒ 这一行报的是「在册模型」。
	if !strings.Contains(out, "probe-list") || !strings.Contains(out, "在册模型") {
		t.Fatalf("干跑没报在册模型：%s", out)
	}
}

// `config reload` · ② 先校验、失败回滚：坏档 ⇒ 1 且不发请求。
//
// 2026-09-23（乙案）换夹具：原夹具 `models: [这 不是 映射]` 在**主控同一入口**下是「解析过 ·
// 0 个 host」（见本文件末尾那条已核事实）——它只对**旧 CLI 的严格档**是坏档。判据意思不变
// （坏档 ⇒ 1 且不发请求），夹具换成主控也认的**真类型错**。
func TestConfigReloadBadConfigRefusesBeforeRequest(t *testing.T) {
	p := writeFixture(t, "models:\n  probe-x:\n    - { host: x3, ctx_window: 大 }\n")
	rc, out, errb := runCapture("config", "reload", "--path", p, "--yes")
	if rc != 1 {
		t.Fatalf("坏档 rc=%d（要 1）", rc)
	}
	if out != "" {
		t.Fatalf("坏档 stdout 该空，得到 %q", out)
	}
	if !strings.Contains(errb, "不发请求") {
		t.Fatalf("判词里没有「不发请求」：%s", errb)
	}
}

// `config reload` · ③ 缺 `--yes` ⇒ 2（D2 档 fail-closed，且**没打主控**）。
func TestConfigReloadRequiresYes(t *testing.T) {
	_, path := fleetFixtureAt(t)
	rc, out, errb := runCapture("config", "reload", "--path", path)
	if rc != 2 || out != "" {
		t.Fatalf("缺 --yes：rc=%d stdout=%q（要 2 + 空）· stderr=%s", rc, out, errb)
	}
}

// 乙案（同件同判 · 2026-09-23）· 解析器口径：CLI 与主控**同一函数**
// `config.LoadFleetConfig`（主控启动 `zerg-core/main.go:99` · 热加载 `handlers.go:1482`）。
//
// 为什么立这条：主控认**两种正规形态**（`internal/config/config.go:140` 逐字「兼容单 dict 和
// 多候选数组两种格式」· 实现 `parseCandidates`）——**裸映射**（`models:` 下 `名: { … }`，真名册件的
// `deepseek-v4-flash` / `DeepSeek-V4-Flash-Vision-Exp` / `GLM-5.3-Flash` 即此形）与**列表**（`- { … }`）。
// 此前 CLI 是裸 `yaml.Unmarshal`（严格档）⇒ 同一件两套解析器 ⇒ 裸映射形态被**误判不过**。
//
// 判据两半（成对，缺一半即假绿）：
//
//	正控：**同一份内容**的两种形态，`config reload --dry-run` 都必须 rc=0，且「在册」那行**逐字相同**（同判）；
//	反控：**类型真的错**（`models:` 给标量 · `ctx_window` 给非数）⇒ 仍必须退非 0（不许把判据放宽成「什么都过」）。
func TestFleetParserSameEntryBothShapes(t *testing.T) {
	listShape := `models:
  probe-x3:
    - { host: x3, backend: llama-server, file: "/data/models/probe.gguf", mem_gb: 18, ctx_window: 131072 }
`
	bareShape := `models:
  probe-x3: { host: x3, backend: llama-server, file: "/data/models/probe.gguf", mem_gb: 18, ctx_window: 131072 }
`
	rosterLine := func(t *testing.T, out string) string {
		t.Helper()
		for _, l := range strings.Split(out, "\n") {
			if strings.Contains(l, "在册") {
				return l
			}
		}
		return ""
	}
	// ── 正控：两形态同判（都 rc=0 · 「在册」行逐字相同）──
	var lines []string
	for _, c := range []struct{ name, text string }{
		{"列表形态", listShape}, {"裸映射形态", bareShape},
	} {
		rc, out, errb := runCapture("config", "reload", "--path", writeFixture(t, c.text), "--dry-run")
		if rc != 0 {
			t.Fatalf("%s：`config reload --dry-run` rc=%d（要 0 · 主控认的形态 CLI 也必须认）· stderr=%s", c.name, rc, errb)
		}
		l := rosterLine(t, out)
		if l == "" || !strings.Contains(l, "probe-x3") {
			t.Fatalf("%s：计划件里没有在册模型那行：%s", c.name, out)
		}
		lines = append(lines, l)
	}
	if lines[0] != lines[1] {
		t.Fatalf("两形态**判据不同**（同件同判破功）：列表=%q ⇔ 裸映射=%q", lines[0], lines[1])
	}
	// ── 反控：**类型真的错** ⇒ 仍必须退非 0（两形态各一格 + 语法错一格；判据不许放宽）──
	//
	// 已核事实（2026-09-23 · 真读数 · **记账不判**）：主控同一入口对「`models:` 的值不是映射」
	// 是**静默当空**——`models: 这不是映射` / `models:`（空）/ `models: [ … ]` 三种都 ⇒ 解析过、0 个模型、
	// **不报错**（`ParseFleetConfig` 只对 MappingNode 的值走 `parseCandidates`，其余形态的 Content 对不上
	// key-value 步长就整段跳过）。⇒ 改后 CLI 与主控**同判**（都 rc=0）——这是**主控侧既有口径的洞**，
	// 不是本笔引入的放宽；改前 CLI 那声 rc=1 来自**分叉的严格档**，不是安全网。收紧它属**改主控侧**，
	// 本笔不做 ✗（已在回执里记成缺口）。
	for _, c := range []struct{ name, text string }{
		{"`ctx_window` 给非数（裸映射）", "models:\n  probe-x3: { host: x3, ctx_window: 大 }\n"},
		{"`ctx_window` 给非数（列表）", "models:\n  probe-x3:\n    - { host: x3, ctx_window: 大 }\n"},
		{"流式映射少一个 `}`（语法错）", "models:\n  probe-x3:\n    - { host: x3, ctx_window: 131072\n"},
	} {
		rc, out, errb := runCapture("config", "reload", "--path", writeFixture(t, c.text), "--dry-run")
		if rc == 0 {
			t.Fatalf("反控「%s」：rc=0（要非 0 —— 类型真错必须判红，判据不许放宽）· stdout=%q", c.name, out)
		}
		if !strings.Contains(errb, "解析不过") {
			t.Fatalf("反控「%s」：判词里没有「解析不过」：%s", c.name, errb)
		}
	}
}
