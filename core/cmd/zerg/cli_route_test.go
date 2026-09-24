// cli_route_test.go —— `zerg route` 族（**显式指定机器**：钉 / 查 / 撒）的**成对判据**
// （2026-09-24 · 单独定制 > 路由默认规则）。
//
// 设计稿（逐字底账）：`Zerg-内部文档/项目文档/v2.5.12/设计-指定机器路由-v1.0-20260924.md`。
// 判据栏三条（命令面）：
//
//	① 钉得住：`route pin --model <模型> --machine <机器> --ttl 2h --yes` ⇒ rc=0，
//	   且覆盖表**盘面**上真的出现那一行（模型 / 机器 / 到期时刻 / TTL 秒数）。
//	② 查得出：`route ls --json model,machine,expires_at,remaining_s,state` ⇒ rc=0 · 五格 · `count` 对得上。
//	③ 撒得掉：`route unpin --model <模型> --yes` ⇒ rc=0，且**一行撤回**（查面随即 0 条）；重跑仍 rc=0（幂等）。
//
// 外加四条**成对负控**（缺一条，上三条就只是「跑通了」）：
//
//	· 缺时效（`--ttl` 不给 / 给了 0 / 给了坏形状）⇒ **rc=2 且 stdout 0 字节且表零变**
//	  （不许悄悄钉成永久 —— 「不永久改变默认」那条约束的第一道闸）；
//	· 缺人签（`--yes` 不给、`--dry-run` 也不给）⇒ **rc=2 且 stdout 0 字节且表零变**；
//	· 干跑（`--dry-run`）⇒ rc=0 计划件只在 stdout、**表零变**（零副作用的正控）；
//	· 查面坏件（表不是 JSON）⇒ **rc=8 且 stdout 0 字节**（读不到不许当「0 条」）。
//
// 落点：`package main_test` ＋ `zerg.RunForTest`（跑**当前源码**的行为，不是盘上旧制品）。
// 全程隔离：`ZERG_ROUTE_PINS` 一律指到 `t.TempDir()` ⇒ 不碰真状态目录、不碰真机群、不发网络请求。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/routepin"
)

// routeFieldsWant —— 命令树登记的那张字段表（判据栏「五格」的现读真值 · 顺序即人面列序）。
var routeFieldsWant = []string{"model", "machine", "expires_at", "remaining_s", "state"}

// routeRun —— 跑一条命令 ⇒ (rc, stdout, stderr)。
func routeRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// routeTablePath —— 本用例独占的覆盖表落点（并把它设进环境 ⇒ 命令面与判据件读的是**同一份**）。
func routeTablePath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "route_pins.json")
	t.Setenv(routepin.EnvPath, p)
	return p
}

// routeTableDigest —— 盘面指纹（不存在 ⇒ "absent"）：负控「表一个字节没动」的取法。
func routeTableDigest(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "absent"
		}
		t.Fatalf("读覆盖表：%v", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// routeTableRows —— 从盘面读回覆盖表的行（判「真机上真写了一行」用）。
func routeTableRows(t *testing.T, p string) []routepin.Pin {
	t.Helper()
	tbl, err := routepin.Load(p)
	if err != nil {
		t.Fatalf("覆盖表读不回：%v", err)
	}
	return tbl.Pins
}

// routeJSON —— `route ls --json …` 的包封。
type routeEnvelope struct {
	Schema string              `json:"schema"`
	Kind   string              `json:"kind"`
	Items  []map[string]string `json:"items"`
	Meta   struct {
		Count int `json:"count"`
	} `json:"meta"`
}

func routeDecode(t *testing.T, out string) routeEnvelope {
	t.Helper()
	var e routeEnvelope
	if err := json.Unmarshal([]byte(out), &e); err != nil {
		t.Fatalf("`--json` 不是合法包封（%v）：%q", err, out)
	}
	return e
}

// ---- ① 钉 ---------------------------------------------------------------------------

// TestRoutePin_PinThenTableLands —— 正控：钉一条 ⇒ rc=0，**盘面上真的多了一行**，TTL 也对得上。
func TestRoutePin_PinThenTableLands(t *testing.T) {
	p := routeTablePath(t)
	rc, out, errb := routeRun(t, "route", "pin",
		"--model", "example-35b-v2", "--machine", "x3", "--ttl", "2h", "--yes")
	if rc != 0 {
		t.Fatalf("钉一条该 rc=0，得到 %d · stderr=%s", rc, errb)
	}
	rows := routeTableRows(t, p)
	if len(rows) != 1 {
		t.Fatalf("盘面上该**恰好一行**，实得 %d 行：%+v", len(rows), rows)
	}
	r := rows[0]
	if r.Model != "example-35b-v2" || r.Machine != "x3" {
		t.Fatalf("钉下来的不是点名那一行：%+v", r)
	}
	exp, err := time.Parse(time.RFC3339, r.ExpiresAt)
	if err != nil {
		t.Fatalf("到期时刻不是 RFC3339：%q", r.ExpiresAt)
	}
	left := time.Until(exp)
	if left < 119*time.Minute || left > 121*time.Minute {
		t.Fatalf("TTL 要对得上 `2h`（±1 分钟），实得 %s · 行=%+v", left.Round(time.Second), r)
	}
	if r.TTLSeconds != 7200 {
		t.Fatalf("行里该记下 TTL 秒数 7200，实得 %d", r.TTLSeconds)
	}
	// 人面要给出**一行撤回**的用法（「撒」这条动作必须自陈）。
	if !strings.Contains(out, "route unpin") {
		t.Errorf("钉完的人面该给出一行撤回用法，实得：%q", out)
	}
}

// TestRoutePin_RequiresTTL —— 负控：**不许不过期**（不给 / 给 0 / 给坏形状 ⇒ rc=2 · stdout 0 · 表零变）。
func TestRoutePin_RequiresTTL(t *testing.T) {
	p := routeTablePath(t)
	before := routeTableDigest(t, p) // "absent"
	for _, c := range []struct {
		name string
		argv []string
	}{
		{"不给 --ttl", []string{"route", "pin", "--model", "m", "--machine", "x3", "--yes"}},
		{"--ttl 裸给", []string{"route", "pin", "--model", "m", "--machine", "x3", "--ttl", "--yes"}},
		{"--ttl 0", []string{"route", "pin", "--model", "m", "--machine", "x3", "--ttl", "0", "--yes"}},
		{"--ttl 坏形状", []string{"route", "pin", "--model", "m", "--machine", "x3", "--ttl", "好久", "--yes"}},
		{"缺 --machine", []string{"route", "pin", "--model", "m", "--ttl", "2h", "--yes"}},
		{"缺 --model", []string{"route", "pin", "--machine", "x3", "--ttl", "2h", "--yes"}},
	} {
		rc, out, _ := routeRun(t, c.argv...)
		if rc != 2 {
			t.Errorf("%s ⇒ rc=%d（要 2 —— 不许悄悄钉成永久）", c.name, rc)
		}
		if out != "" {
			t.Errorf("%s ⇒ stdout 要 0 字节（现读 %d 字节）：%q", c.name, len(out), out)
		}
	}
	if after := routeTableDigest(t, p); after != before {
		t.Fatalf("六条负控跑完，覆盖表该一个字节没动：%s → %s", before, after)
	}
}

// TestRoutePin_RequiresYes —— 负控：D2 人签 —— 缺 `--yes` ⇒ rc=2（计划件只走 stderr）· 表零变。
func TestRoutePin_RequiresYes(t *testing.T) {
	p := routeTablePath(t)
	before := routeTableDigest(t, p)
	rc, out, errb := routeRun(t, "route", "pin", "--model", "m", "--machine", "x3", "--ttl", "2h")
	if rc != 2 {
		t.Fatalf("缺 --yes ⇒ rc=2，得到 %d", rc)
	}
	if out != "" {
		t.Fatalf("缺 --yes ⇒ stdout 要 0 字节（现读 %d）：%q", len(out), out)
	}
	if !strings.Contains(errb, "2h") && !strings.Contains(errb, "x3") {
		t.Errorf("计划件该走 stderr 并写出要做什么，实得：%q", errb)
	}
	if after := routeTableDigest(t, p); after != before {
		t.Fatalf("缺 --yes 时覆盖表该零变：%s → %s", before, after)
	}
	// 同一把「人签」配「撒」也成立。
	rc2, out2, _ := routeRun(t, "route", "unpin", "--model", "m")
	if rc2 != 2 || out2 != "" {
		t.Fatalf("撒缺 --yes ⇒ rc=2 且 stdout 0 字节，得到 rc=%d · out=%q", rc2, out2)
	}
}

// TestRoutePin_DryRunZeroSideEffect —— 正控/零副作用：`--dry-run` ⇒ rc=0、计划件走 stdout、**表零变**。
func TestRoutePin_DryRunZeroSideEffect(t *testing.T) {
	p := routeTablePath(t)
	before := routeTableDigest(t, p)
	rc, out, _ := routeRun(t, "route", "pin", "--model", "m", "--machine", "x3", "--ttl", "30m", "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑该 rc=0，得到 %d", rc)
	}
	if !strings.Contains(out, "x3") || !strings.Contains(out, "route pins") && !strings.Contains(out, routepin.FileName) {
		t.Errorf("干跑的计划件该点名机器与落点，实得：%q", out)
	}
	if after := routeTableDigest(t, p); after != before {
		t.Fatalf("干跑该零副作用：%s → %s", before, after)
	}
}

// TestRoutePin_Idempotent —— 幂等：同一条钉两次 ⇒ 第二次 rc=0 且**一个字都不写**（盘面指纹不变）。
func TestRoutePin_Idempotent(t *testing.T) {
	p := routeTablePath(t)
	if rc, _, e := routeRun(t, "route", "pin", "--model", "m", "--machine", "x3", "--ttl", "2h", "--yes"); rc != 0 {
		t.Fatalf("第一次钉该 rc=0，得到 %d · %s", rc, e)
	}
	d1 := routeTableDigest(t, p)
	rc, out, errb := routeRun(t, "route", "pin", "--model", "m", "--machine", "x3", "--ttl", "2h", "--yes")
	if rc != 0 {
		t.Fatalf("重跑该 rc=0（幂等不许变错），得到 %d · %s", rc, errb)
	}
	if d2 := routeTableDigest(t, p); d2 != d1 {
		t.Fatalf("「已在该状态」时该一个字都不写：%s → %s", d1, d2)
	}
	if !strings.Contains(out, "幂等") && !strings.Contains(errb, "幂等") {
		t.Errorf("幂等那一档该说明「没写」，实得 stdout=%q stderr=%q", out, errb)
	}
}

// ---- ② 查 ---------------------------------------------------------------------------

// TestRouteLs_JSONFiveFields —— 正控：查面五格 + `count` 对得上；空表 ⇒ rc=0 且 0 条（默认态不是错）。
func TestRouteLs_JSONFiveFields(t *testing.T) {
	p := routeTablePath(t)
	rc, out, errb := routeRun(t, "route", "ls", "--json", strings.Join(routeFieldsWant, ","))
	if rc != 0 {
		t.Fatalf("空表查 face 该 rc=0（「没钉」是正常态），得到 %d · %s", rc, errb)
	}
	e := routeDecode(t, out)
	if e.Meta.Count != 0 || len(e.Items) != 0 {
		t.Fatalf("空表该 0 条，实得 count=%d items=%d", e.Meta.Count, len(e.Items))
	}
	// 钉一条再查：五格齐、值对得上。
	if rc, _, errb := routeRun(t, "route", "pin", "--model", "m1", "--machine", "x3", "--ttl", "90m", "--yes"); rc != 0 {
		t.Fatalf("夹具钉一条失败：%d · %s", rc, errb)
	}
	rc, out, errb = routeRun(t, "route", "ls", "--json", strings.Join(routeFieldsWant, ","))
	if rc != 0 {
		t.Fatalf("有钉时查面该 rc=0，得到 %d · %s", rc, errb)
	}
	e = routeDecode(t, out)
	if e.Meta.Count != 1 || len(e.Items) != 1 {
		t.Fatalf("该恰好 1 条，实得 count=%d items=%d：%v", e.Meta.Count, len(e.Items), e.Items)
	}
	row := e.Items[0]
	for _, f := range routeFieldsWant {
		if _, ok := row[f]; !ok {
			t.Errorf("五格里缺 %q：%v", f, row)
		}
	}
	if row["model"] != "m1" || row["machine"] != "x3" {
		t.Errorf("查面那一行的模型/机器不对：%v", row)
	}
	if row["state"] == "" || row["remaining_s"] == "" || row["expires_at"] == "" {
		t.Errorf("查面三格该有值：%v", row)
	}
	if _, err := time.Parse(time.RFC3339, row["expires_at"]); err != nil {
		t.Errorf("expires_at 该是 RFC3339（跨语言可比），实得 %q", row["expires_at"])
	}
	if filepath.Base(p) != routepin.FileName {
		t.Errorf("夹具落点该叫 %s，实得 %s", routepin.FileName, p)
	}
}

// TestRouteLs_UsageFace —— 用法面：裸 `--json` / 未知字段 ⇒ rc=2 且 stdout 0 字节；
// 命令树登记的字段表 ⟷ 裸 `--json` 时 stderr 印的字段表**两处同值**。
func TestRouteLs_UsageFace(t *testing.T) {
	routeTablePath(t)
	rc, out, errb := routeRun(t, "route", "ls", "--json")
	if rc != 2 {
		t.Fatalf("裸 --json ⇒ rc=2，得到 %d", rc)
	}
	if out != "" {
		t.Fatalf("裸 --json ⇒ stdout 0 字节（现读 %d）：%q", len(out), out)
	}
	for _, f := range routeFieldsWant {
		if !strings.Contains(errb, f) {
			t.Errorf("stderr 的字段表该逐格列出 %q，实得：%q", f, errb)
		}
	}
	rc2, out2, _ := routeRun(t, "route", "ls", "--json", "model,没这一格")
	if rc2 != 2 {
		t.Fatalf("未知字段 ⇒ rc=2，得到 %d", rc2)
	}
	// 本仓口径（`Q-155` 那条归一 · 见 `cli_version_jsongap_test.go`）：给了 `--json` 时，
	// 用法错也要出**可读的错包封**（`error.exit_code` 与真退码同源），而 **items 一条都不出**。
	var bad struct {
		Items []map[string]string `json:"items"`
		Error struct {
			ExitCode int `json:"exit_code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out2), &bad); err != nil {
		t.Fatalf("未知字段那一档该出错包封（%v）：%q", err, out2)
	}
	if bad.Error.ExitCode != 2 || len(bad.Items) != 0 {
		t.Fatalf("未知字段 ⇒ 包封 error.exit_code=2 且 items 空，实得 %q", out2)
	}
	// 命令树登记那一份（现读真源）必须逐格等于判据栏那五格。
	info, ok := zerg.CommandInfoOfForTest("route ls")
	if !ok {
		t.Fatal("命令树里找不到 `route ls`")
	}
	if strings.Join(info.Fields, ",") != strings.Join(routeFieldsWant, ",") {
		t.Fatalf("命令树登记的五格与判据栏不一致：登记=%v 期望=%v", info.Fields, routeFieldsWant)
	}
	if info.DangerLevel != "" {
		t.Fatalf("`route ls` 是只读面，不该挂危险档，实得 %q", info.DangerLevel)
	}
	for _, p := range []string{"route pin", "route unpin"} {
		pi, ok := zerg.CommandInfoOfForTest(p)
		if !ok {
			t.Fatalf("命令树里找不到 `%s`", p)
		}
		if pi.DangerLevel != "D2" {
			t.Errorf("`%s` 是写面，该挂 D2 人签闸，实得 %q", p, pi.DangerLevel)
		}
	}
}

// TestRouteLs_BrokenTableIsBlocked —— 负控：表坏件 ⇒ **rc=8**（读不到不许当「0 条」）。
// 两条流分开判（本仓口径：给了 `--json` 时错也要出**包封**，调用方读 `error.exit_code`；
// 没给 `--json` 时 stdout 一个字节都不出）：
//
//	· 无 `--json` ⇒ rc=8 且 **stdout 0 字节**；
//	· 有 `--json <字段>` ⇒ rc=8、包封里 `error.exit_code=8` 且 **items 一条都不出**。
func TestRouteLs_BrokenTableIsBlocked(t *testing.T) {
	p := routeTablePath(t)
	if err := os.WriteFile(p, []byte("{ 这不是 JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := routeRun(t, "route", "ls")
	if rc != 8 {
		t.Fatalf("坏件该 rc=8（不给结论），得到 %d · stderr=%s", rc, errb)
	}
	if out != "" {
		t.Fatalf("没给 --json 时 stdout 要 0 字节（现读 %d）：%q", len(out), out)
	}
	rc, out, _ = routeRun(t, "route", "ls", "--json", strings.Join(routeFieldsWant, ","))
	if rc != 8 {
		t.Fatalf("带 --json 的坏件也该 rc=8，得到 %d", rc)
	}
	var e struct {
		Items []map[string]string `json:"items"`
		Error struct {
			Kind     string `json:"kind"`
			ExitCode int    `json:"exit_code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &e); err != nil {
		t.Fatalf("带 --json 的坏件该出**可读的错包封**（%v）：%q", err, out)
	}
	if e.Error.ExitCode != 8 {
		t.Errorf("包封里 error.exit_code 该与真退码同源（8），实得 %d：%q", e.Error.ExitCode, out)
	}
	if len(e.Items) != 0 {
		t.Errorf("读不到时 items 一条都不许出（不许拿「0 条」冒充答案）：%q", out)
	}
}

// ---- ③ 撒 ---------------------------------------------------------------------------

// TestRouteUnpin_OneLineRevoke —— 正控：撒 ⇒ 一行撤回（查面随即 0 条）；重跑仍 rc=0（幂等）。
func TestRouteUnpin_OneLineRevoke(t *testing.T) {
	routeTablePath(t)
	if rc, _, e := routeRun(t, "route", "pin", "--model", "m1", "--machine", "x3", "--ttl", "2h", "--yes"); rc != 0 {
		t.Fatalf("夹具钉一条失败：%d · %s", rc, e)
	}
	rc, _, errb := routeRun(t, "route", "unpin", "--model", "m1", "--yes")
	if rc != 0 {
		t.Fatalf("撒该 rc=0，得到 %d · %s", rc, errb)
	}
	rc, out, _ := routeRun(t, "route", "ls", "--json", "model,machine,expires_at,remaining_s,state")
	if rc != 0 {
		t.Fatalf("撒后查面该 rc=0，得到 %d", rc)
	}
	if e := routeDecode(t, out); e.Meta.Count != 0 {
		t.Fatalf("撒后该 0 条，实得 %d：%v", e.Meta.Count, e.Items)
	}
	// 幂等：没得撒也退 0。
	rc2, out2, _ := routeRun(t, "route", "unpin", "--model", "m1", "--yes")
	if rc2 != 0 || out2 == "" {
		t.Fatalf("幂等重跑该 rc=0 且给一行读数，得到 rc=%d · out=%q", rc2, out2)
	}
}

// TestRouteUnpin_AllAndScope —— 撒全部（不给 `--model`）⇒ 表清空 + 件仍在（空表不是缺件）。
func TestRouteUnpin_AllAndScope(t *testing.T) {
	p := routeTablePath(t)
	for _, m := range []string{"m1", "m2"} {
		if rc, _, e := routeRun(t, "route", "pin", "--model", m, "--machine", "x3", "--ttl", "2h", "--yes"); rc != 0 {
			t.Fatalf("夹具钉 %s 失败：%d · %s", m, rc, e)
		}
	}
	if rc, _, e := routeRun(t, "route", "unpin", "--yes"); rc != 0 {
		t.Fatalf("撒全部该 rc=0，得到 %d · %s", rc, e)
	}
	if rows := routeTableRows(t, p); len(rows) != 0 {
		t.Fatalf("撒全部后该空表，实得 %+v", rows)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("撒全部后件该还在（空表 ≠ 缺件）：%v", err)
	}
}

// ---- ④ 命令面外框 -------------------------------------------------------------------

// TestRouteFamily_Frame —— 命令树外框：三条命令都在树里、有用法串、三条各自可跑；
// 前缀撞名不许模糊命中（`route pinned` ⇒ rc=2）。
func TestRouteFamily_Frame(t *testing.T) {
	paths := zerg.CommandPathsForTest()
	want := []string{"route pin", "route ls", "route unpin"}
	set := map[string]bool{}
	for _, p := range paths {
		set[p] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("命令树里缺 `%s`（命令树该 +3）", w)
		}
		info, _ := zerg.CommandInfoOfForTest(w)
		if !strings.Contains(info.Usage, "--machine") && w == "route pin" {
			t.Errorf("`route pin` 的用法串该写出 `--machine`，实得：%q", info.Usage)
		}
		if !info.OpenForRun {
			t.Errorf("`%s` 该已开放执行", w)
		}
	}
	routeTablePath(t)
	rc, out, _ := routeRun(t, "route", "pinned")
	if rc != 2 || out != "" {
		t.Fatalf("前缀撞名该 rc=2 且 stdout 0 字节（不许模糊命中），得到 rc=%d · out=%q", rc, out)
	}
	rc, out, _ = routeRun(t, "route")
	if rc != 2 || out != "" {
		t.Fatalf("裸 `route` 该 rc=2 且 stdout 0 字节，得到 rc=%d · out=%q", rc, out)
	}
}

// TestRoute_HeaderIsNotAFlag —— 「请求侧头」不是命令面旗标：
//
//	· 头那一档**只有网关侧一个入口**（`X-Zerg-Machine`），命令面没有任何写法能写它 ⇒ 它永不落盘；
//	· `--machine` 是 `route pin` 的旗标；别的命令上给它是**值照收、语义各判**那一档（本仓解析器
//	  写死的口径：不用的命令静默忽略，不许给两条命令各开一个分叉）⇒ rc=0 且**行为不变**。
func TestRoute_HeaderIsNotAFlag(t *testing.T) {
	p := routeTablePath(t)
	_, base, _ := routeRun(t, "route", "ls", "--json", "model,machine,expires_at,remaining_s,state")
	rc, out, _ := routeRun(t, "route", "ls", "--machine", "x3", "--json", "model,machine,expires_at,remaining_s,state")
	if rc != 0 {
		t.Fatalf("`route ls --machine x3` ⇒ 值照收、语义各判（rc=0），得到 %d", rc)
	}
	if out != base {
		t.Errorf("多给一枚本命令不用的旗标不该改变行为：\n  不带：%q\n  带了：%q", base, out)
	}
	// `--machine` 裸给（后面没跟值）⇒ `route pin` 判「没给」⇒ rc=2（不许拿空串当机器名）。
	rc, out, _ = routeRun(t, "route", "pin", "--model", "m", "--machine", "--ttl", "2h", "--yes")
	if rc != 2 || out != "" {
		t.Fatalf("`--machine` 裸给该 rc=2 且 stdout 0 字节，得到 rc=%d · out=%q", rc, out)
	}
	// 头**不是**命令面旗标：没有任何命令面写法带 `X-Zerg-Machine`（点一下名字表就当自证）。
	if strings.Contains(string(mustReadFile(t, "family_route.go")), "X-Zerg-Machine") {
		t.Error("命令面那一件里不该出现请求头名（头只在网关侧那一件里）")
	}
	if after := routeTableDigest(t, p); after != "absent" {
		t.Fatalf("这一整条用例该零写入，实得盘面 %s", after)
	}
}

// mustReadFile —— 读一件当前包里的源码（只读，判据件自用）。
func mustReadFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("读不到 %s：%v", name, err)
	}
	return b
}

// TestRoute_NoWriteWithoutYes_SourceSelfCheck —— 静态自检：命令面那一件里，
// 写盘只出现在**过了人签闸**之后（`tbl.Save` / `routepin.Save` 调用点必须在 `routeWriteGate` 之后）。
func TestRoute_NoWriteWithoutYes_SourceSelfCheck(t *testing.T) {
	src, err := os.ReadFile("family_route.go")
	if err != nil {
		t.Fatalf("读不到 family_route.go：%v", err)
	}
	text := string(src)
	gate := strings.Index(text, "func routeWriteGate")
	if gate < 0 {
		t.Fatal("family_route.go 里找不到 `routeWriteGate`（人签闸那一道该是唯一判门）")
	}
	for _, w := range []string{"tbl.Save(", "routepin.Save("} {
		i := strings.Index(text, w)
		if i < 0 {
			continue
		}
		if i < gate {
			t.Errorf("写盘调用 %q 出现在 `routeWriteGate` **之前** —— 人签闸被绕过了", w)
		}
	}
}
