// cli_ci_green_test.go —— `zerg ci green` 的**成对判据**（组4 §二.4 `W-51` · `研-禁:134` `R-21`
// 归绿核心 · 任务单序133 · 与组3 `Q-025` 同条）。
//
// 判据栏逐字：**一条只读命令出「必需检查集合全 `success` 且 `skipped` = 0」+ `head_sha`；
// 负控：有 `skipped` ⇒ 判红**。
//
// 本件把这条判据的两个半边都钉住（夹具自建自管 · 不碰真仓状态目录、不碰网络、不写任何件）：
//
//	① 正控 · 判绿那一路：声明集合逐名 `success` ⇒ **rc=0** · 读数带 `head_sha`（人面 + 机器面两处）
//	② 负控 · **判据栏那半**：集合内有一条 `skipped` ⇒ **rc=1**（判红）+ 逐条点名
//	③ 负控 · 非 `success`（`failure` / `cancelled`）⇒ **rc=1**
//	④ 负控 · 「记录里没有它」**不许当 `success`**（最重的假绿）⇒ 集合内有一条判不出 ⇒
//	   **rc=8 且 stdout 0 字节**（一行都不出）
//	⑤ 边界 · **集合外**的 `skipped` 不参与（口径写死 = 「集合内」）⇒ 照旧 rc=0
//	⑥ 边界 · 判序写死：**判得出非 success 的那一条压过判不出的那一条** ⇒ 两者同时命中 ⇒ rc=1
//	⑦ 负控 · 声明件 / 记录件的坏形状（缺件 / 非 JSON / 形状号不认 / `source` 闭集外 / `checks` 空 /
//	   重名 / 空名 / 无 `head_sha` / `check_runs` 空）⇒ 一律 **rc=8 且 stdout 0 字节**
//	⑧ 负控 · `--json` 里缺身份两格（`head_sha` / `caliber`）⇒ **rc=2 且 stdout 0 字节**
//	⑨ 用法面：缺 `--run` / `--run` 给两次 / 多余位置参数 / 裸 `--json` ⇒ 一律 rc=2 · stdout 0 字节；
//	   字段表**两处同值**（命令树登记 ⟷ 裸 `--json` 时 stderr 印的那一行）
//	⑩ 声明件 ⟷ `publish/ci/ci.yml` **现读对拍**（两套真源并存就会漂 ⇒ 立判据），并带成对负控
//	   （合成 yml 里改一个 job 名 ⇒ 判定口当场报差 —— 证明这一格不是恒绿）
//	⑪ **零副作用**：跑前跑后 声明件 + 记录件 的 `sha256` 逐字同值、且两份件所在的目录**一件不多**
//
// 落点：`package main_test` ＋ `zerg.RunForTest` —— 跑的是**当前源码**的行为
// （与 `cli_publish_tree_test.go` / `cli_net_probe_test.go` 同一条路）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// ciGreenCmd —— 本件唯一的命令路径（判据里逐字点名的那一条）。
const ciGreenCmd = "ci green"

// ciGreenFieldsWant —— 命令树登记的那张字段表（判据栏「六格」的现读真值）。
var ciGreenFieldsWant = []string{"head_sha", "caliber", "decl_source", "required", "skipped", "verdict"}

// ciGreenRun —— 跑一条命令 ⇒ (rc, stdout, stderr)。
func ciGreenRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// ciGreenDir —— 一个自管目录（夹具落在这里 · 跑完自动清）。
func ciGreenDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// ciGreenWrite —— 落一件夹具（**只在 t.TempDir() 里**）。
func ciGreenWrite(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("落夹具 %s：%v", path, err)
	}
	return path
}

// ciGreenDeclBody —— 造一份**声明件**正文（形状与 `publish/ci/required-checks.json` 同）。
func ciGreenDeclBody(id, source string, names []string) string {
	esc := make([]string, 0, len(names))
	for _, n := range names {
		esc = append(esc, `"`+n+`"`)
	}
	return `{"id":"` + id + `","source":"` + source + `","source_note":"夹具","checks":[` +
		strings.Join(esc, ",") + `]}`
}

// ciGreenRecordBody —— 造一份**记录件**正文（`head_sha` + 逐格三列）。
func ciGreenRecordBody(id, head string, rows [][2]string) string {
	cells := make([]string, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, `{"name":"`+r[0]+`","status":"completed","conclusion":"`+r[1]+`"}`)
	}
	body := `{"head_sha":"` + head + `","check_runs":[` + strings.Join(cells, ",") + `]}`
	if id != "" {
		body = `{"id":"` + id + `",` + strings.TrimPrefix(body, "{")
	}
	return body
}

// ciGreenFixture —— 声明件 + 记录件一套（返回两件路径）。
func ciGreenFixture(t *testing.T, source string, names []string, head string, rows [][2]string) (string, string) {
	t.Helper()
	dir := ciGreenDir(t)
	decl := ciGreenWrite(t, filepath.Join(dir, "required-checks.json"),
		ciGreenDeclBody("zerg.ci.required-checks.v1", source, names))
	rec := ciGreenWrite(t, filepath.Join(dir, "run.json"),
		ciGreenRecordBody("zerg.ci.check-runs.v1", head, rows))
	return decl, rec
}

// ciGreenRow —— 解 `--json` 的那一行 result（解不开即 Fatal —— 夹具坏了不许静默）。
func ciGreenRow(t *testing.T, out string) map[string]string {
	t.Helper()
	var doc struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &doc); err != nil {
		t.Fatalf("`--json` 的输出不是包封：%v（%q）", err, out)
	}
	if len(doc.Items) != 1 {
		t.Fatalf("`--json` 的 items 要恰好 1 条（现读 %d）：%q", len(doc.Items), out)
	}
	return doc.Items[0]
}

// ciGreenEnvelopeKeys —— `--json` 顶层六键（缺一件即 Fatal）。
func ciGreenEnvelopeKeys(t *testing.T, out string) map[string]json.RawMessage {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &doc); err != nil {
		t.Fatalf("`--json` 不是 JSON 对象：%v（%q）", err, out)
	}
	want := []string{"schema", "kind", "items", "meta", "warnings", "truncated"}
	for _, k := range want {
		if _, ok := doc[k]; !ok {
			t.Errorf("包封缺顶层键 %q（现读 %v）", k, doc)
		}
	}
	if len(doc) != len(want) {
		// 失败侧会多一枚 `error`（§九 M7）—— 成功侧不许有第七键。
		if _, ok := doc["error"]; !ok || len(doc) != len(want)+1 {
			t.Errorf("包封顶层键数 = %d（要 %d）：%v", len(doc), len(want), doc)
		}
	}
	return doc
}

// ---- ① 正控：判绿那一路（读数带 head_sha · 人面机器面两处）-------------------------------

func TestCiGreen_GreenVerdictCarriesHeadSHA(t *testing.T) {
	const head = "9f1c0a4d3e2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c"
	decl, rec := ciGreenFixture(t, "unverified", []string{"core", "docs"}, head,
		[][2]string{{"core", "success"}, {"docs", "success"}, {"别的检查", "skipped"}})

	rc, out, errb := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl)
	if rc != 0 {
		t.Fatalf("判绿那一路 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	// 人面三样：口径 / 声明来处（含 source）/ 身份（head_sha）逐字在。
	for _, want := range []string{"口径 =", "声明件", "source=unverified", "判 = 绿", "head_sha = " + head} {
		if !strings.Contains(errb, want) {
			t.Errorf("人面缺 %q：\n%s", want, errb)
		}
	}
	// 「未核」那一格不许被省掉（`source=unverified` ⇒ 口径行必须明写）。
	if !strings.Contains(errb, "未核") {
		t.Errorf("`source=unverified` 时人面必须明写「必需集合未核」：\n%s", errb)
	}
	// 人面那一行表格：一树/一条一格（这里是**一条**读数）。
	if !strings.Contains(out, head) || !strings.Contains(out, "绿") {
		t.Errorf("人面表格缺 `head_sha` 或判词：%q", out)
	}

	// 机器面：六格齐 + 身份两格逐字。
	rc2, out2, _ := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl,
		"--json", "head_sha,caliber,decl_source,required,skipped,verdict")
	if rc2 != 0 {
		t.Fatalf("`--json` 全字段 rc=%d（要 0）", rc2)
	}
	row := ciGreenRow(t, out2)
	ciGreenEnvelopeKeys(t, out2)
	if row["head_sha"] != head {
		t.Errorf("机器面 head_sha = %q（要 %q）", row["head_sha"], head)
	}
	if row["verdict"] != "绿" || row["skipped"] != "0" || row["required"] != "2" {
		t.Errorf("机器面三格 = %v", row)
	}
	if strings.TrimSpace(row["caliber"]) == "" {
		t.Error("机器面 `caliber` 是空的（口径那一格不许空）")
	}
	if row["decl_source"] != "unverified" {
		t.Errorf("机器面 decl_source = %q（要 unverified）", row["decl_source"])
	}
}

// ---- ② 负控 · **判据栏那半**：集合内有一条 skipped ⇒ 判红 --------------------------------

func TestCiGreen_SkippedIsRed(t *testing.T) {
	decl, rec := ciGreenFixture(t, "unverified", []string{"core", "docs"}, "aaaa1111",
		[][2]string{{"core", "success"}, {"docs", "skipped"}})

	rc, out, errb := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl)
	if rc != 1 {
		t.Fatalf("集合内有 `skipped` ⇒ rc=%d（判据栏逐字要**判红** = 1）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "非绿：docs") || !strings.Contains(errb, "conclusion=skipped") {
		t.Errorf("判红要**逐条点名**（含 conclusion）：\n%s", errb)
	}
	if !strings.Contains(errb, "判 = 红") {
		t.Errorf("人面判词缺「判 = 红」：\n%s", errb)
	}
	// 判红 = **一条答案**（不是「不给结论」）⇒ 那一行照出（stdout 非空）。
	if !strings.Contains(out, "红") {
		t.Errorf("判红那一行要照出（stdout）：%q", out)
	}
	// 机器面同一条读数。
	_, out2, _ := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl,
		"--json", "head_sha,caliber,verdict,skipped")
	if r := ciGreenRow(t, out2); r["verdict"] != "红" || r["skipped"] != "1" {
		t.Errorf("机器面判红那条 = %v", r)
	}
}

// ---- ③ 负控：非 success（failure / cancelled）⇒ 判红 ------------------------------------

func TestCiGreen_NonSuccessIsRed(t *testing.T) {
	for _, concl := range []string{"failure", "cancelled", "timed_out", "neutral"} {
		decl, rec := ciGreenFixture(t, "unverified", []string{"core"}, "bbbb2222",
			[][2]string{{"core", concl}})
		rc, _, errb := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl)
		if rc != 1 {
			t.Errorf("conclusion=%s ⇒ rc=%d（要 1）· stderr=%s", concl, rc, errb)
		}
	}
}

// ---- ④ 负控：集合内有一条**判不出** ⇒ 8 且一行都不出（「没出现」≠ success） ---------------

func TestCiGreen_AbsentRequiredIsBlocked(t *testing.T) {
	decl, rec := ciGreenFixture(t, "unverified", []string{"core", "docs"}, "cccc3333",
		[][2]string{{"core", "success"}, {"ui", "success"}})

	rc, out, errb := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl)
	if rc != 8 {
		t.Fatalf("必需检查不在记录里 ⇒ rc=%d（要 8）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("不给结论那一档 **stdout 必须 0 字节**（一行都不出）：%q", out)
	}
	if !strings.Contains(errb, "判不出：docs") {
		t.Errorf("要逐条点名「判不出」的那一条：\n%s", errb)
	}
	// `--json` 的失败路径 = §九 M7 错误包封（**错误面**，不是结果面）。
	_, out2, _ := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl, "--json", "head_sha,verdict")
	if out2 == "" {
		t.Error("给了 `--json` 的失败路径要出错误包封（0 字节只留给结果面）")
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out2)), &doc); err != nil {
		t.Fatalf("失败包封解不开：%v（%q）", err, out2)
	}
	if _, ok := doc["error"]; !ok {
		t.Errorf("失败包封里没有 `error` 块：%q", out2)
	}
}

// ---- ⑤ 边界：**集合外**的 skipped 不参与（口径 = 集合内） --------------------------------

func TestCiGreen_OutsideSkippedDoesNotCount(t *testing.T) {
	decl, rec := ciGreenFixture(t, "unverified", []string{"core"}, "dddd4444",
		[][2]string{{"core", "success"}, {"不在声明里的检查", "skipped"}})
	rc, _, errb := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl)
	if rc != 0 {
		t.Fatalf("集合外的 `skipped` 不参与（口径写死 = 集合内）⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "集合内 skipped = 0") {
		t.Errorf("人面要把「集合内」这三个字印出来（口径不许含糊）：\n%s", errb)
	}
}

// ---- ⑥ 边界：判序写死 —— 判得出的非 success 压过判不出的 --------------------------------

func TestCiGreen_RedBeatsBlocked(t *testing.T) {
	decl, rec := ciGreenFixture(t, "unverified", []string{"core", "docs"}, "eeee5555",
		[][2]string{{"core", "failure"}})
	rc, out, errb := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl)
	if rc != 1 {
		t.Fatalf("「有一条判得出非 success」⇒ rc=%d（判序①：真读数不许被 8 吞）· stderr=%s", rc, errb)
	}
	if out == "" {
		t.Error("判红那一档要照出那一行（它不是「不给结论」）")
	}
	if !strings.Contains(errb, "非绿：core") || !strings.Contains(errb, "判不出：docs") {
		t.Errorf("两件事都要印（判红 + 判不出的那一条照实）：\n%s", errb)
	}
}

// ---- ⑦ 负控：两件坏形状一律 8 且 0 字节 -------------------------------------------------

func TestCiGreen_BadShapesAreBlocked(t *testing.T) {
	dir := ciGreenDir(t)
	good := ciGreenWrite(t, filepath.Join(dir, "good.json"),
		ciGreenRecordBody("zerg.ci.check-runs.v1", "1234abcd", [][2]string{{"core", "success"}}))
	goodDecl := ciGreenWrite(t, filepath.Join(dir, "good-decl.json"),
		ciGreenDeclBody("zerg.ci.required-checks.v1", "unverified", []string{"core"}))
	badJSON := ciGreenWrite(t, filepath.Join(dir, "bad.json"), "{ 不是 JSON")
	emptyDecl := ciGreenWrite(t, filepath.Join(dir, "empty-decl.json"),
		ciGreenDeclBody("zerg.ci.required-checks.v1", "unverified", nil))
	dupDecl := ciGreenWrite(t, filepath.Join(dir, "dup-decl.json"),
		ciGreenDeclBody("zerg.ci.required-checks.v1", "unverified", []string{"core", "core"}))
	blankDecl := ciGreenWrite(t, filepath.Join(dir, "blank-decl.json"),
		ciGreenDeclBody("zerg.ci.required-checks.v1", "unverified", []string{"core", " "}))
	badIDDecl := ciGreenWrite(t, filepath.Join(dir, "badid-decl.json"),
		ciGreenDeclBody("zerg.ci.required-checks.v9", "unverified", []string{"core"}))
	badSrcDecl := ciGreenWrite(t, filepath.Join(dir, "badsrc-decl.json"),
		ciGreenDeclBody("zerg.ci.required-checks.v1", "猜的", []string{"core"}))
	noHead := ciGreenWrite(t, filepath.Join(dir, "nohead.json"),
		ciGreenRecordBody("zerg.ci.check-runs.v1", "", [][2]string{{"core", "success"}}))
	noChecks := ciGreenWrite(t, filepath.Join(dir, "nochecks.json"),
		ciGreenRecordBody("zerg.ci.check-runs.v1", "1234abcd", nil))
	badIDRec := ciGreenWrite(t, filepath.Join(dir, "badid.json"),
		ciGreenRecordBody("zerg.ci.check-runs.v9", "1234abcd", [][2]string{{"core", "success"}}))
	blankName := ciGreenWrite(t, filepath.Join(dir, "blankname.json"),
		ciGreenRecordBody("zerg.ci.check-runs.v1", "1234abcd", [][2]string{{" ", "success"}}))
	dupName := ciGreenWrite(t, filepath.Join(dir, "dupname.json"),
		ciGreenRecordBody("zerg.ci.check-runs.v1", "1234abcd", [][2]string{{"core", "success"}, {"core", "failure"}}))

	cases := []struct {
		name       string
		decl, rec  string
		wantErrSub string
	}{
		{"声明件不在盘", filepath.Join(dir, "nonexistent.json"), good, "声明件"},
		{"声明件不是 JSON", badJSON, good, "声明件"},
		{"声明件形状号不认", badIDDecl, good, "形状号"},
		{"声明件 source 闭集外", badSrcDecl, good, "闭集"},
		{"声明件 checks 空", emptyDecl, good, "空的"},
		{"声明件重名", dupDecl, good, "两次"},
		{"声明件空名", blankDecl, good, "空名字"},
		{"声明件是目录", dir, good, "声明件"},
		{"记录件不在盘", goodDecl, filepath.Join(dir, "nonexistent.json"), "记录件"},
		{"记录件不是 JSON", goodDecl, badJSON, "记录件"},
		{"记录件形状号不认", goodDecl, badIDRec, "形状号"},
		{"记录件没有 head_sha", goodDecl, noHead, "head_sha"},
		{"记录件一条检查都没有", goodDecl, noChecks, "空的"},
		{"记录件有空名字", goodDecl, blankName, "空的"},
		{"记录件同名两条", goodDecl, dupName, "两次"},
	}
	for _, c := range cases {
		rc, out, errb := ciGreenRun(t, "ci", "green", "--run", c.rec, "--decl", c.decl)
		if rc != 8 {
			t.Errorf("%s ⇒ rc=%d（要 8）· stderr=%s", c.name, rc, errb)
		}
		if out != "" {
			t.Errorf("%s ⇒ stdout 必须 0 字节（%d 字节）", c.name, len(out))
		}
		if !strings.Contains(errb, c.wantErrSub) {
			t.Errorf("%s ⇒ stderr 要点名到「%s」：\n%s", c.name, c.wantErrSub, errb)
		}
	}
}

// ---- ⑧ 负控：`--json` 缺身份两格 ⇒ 2 且 0 字节 ------------------------------------------

func TestCiGreen_IdentityCellsRequired(t *testing.T) {
	const head = "7777aaaa8888bbbb"
	decl, rec := ciGreenFixture(t, "unverified", []string{"core"}, head, [][2]string{{"core", "success"}})

	for _, fields := range []string{"head_sha", "caliber", "verdict", "decl_source,required"} {
		rc, out, errb := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl, "--json", fields)
		if rc != 2 {
			t.Errorf("`--json %s` 缺身份那两格 ⇒ rc=%d（要 2）· stderr=%s", fields, rc, errb)
		}
		// 拒执 ⇒ **一条读数都不出**。这一档的 stdout 是 §九 M7 的**错误包封**（错误面，不是结果面 ——
		// 与 `publish tree has` 同一条口径）：`items` 必须是空的 `[]`，且必须带 `error` 块。
		if out != "" {
			var doc struct {
				Items []map[string]string `json:"items"`
				Error map[string]any      `json:"error"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &doc); err != nil {
				t.Errorf("`--json %s` 的 stdout 解不开：%v（%q）", fields, err, out)
				continue
			}
			if len(doc.Items) != 0 {
				t.Errorf("`--json %s` 缺身份格 ⇒ **不许出读数**（现读 %d 条）：%q", fields, len(doc.Items), out)
			}
			if doc.Error == nil {
				t.Errorf("`--json %s` 缺身份格 ⇒ 错误面要带 `error` 块：%q", fields, out)
			}
		}
	}
	// 正控：两格齐 ⇒ 0（证明上一条不是恒红）。
	rc, _, _ := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl, "--json", "head_sha,caliber")
	if rc != 0 {
		t.Errorf("身份两格齐 ⇒ rc=%d（要 0）", rc)
	}
}

// ---- ⑨ 用法面 + 字段表两处同值 ----------------------------------------------------------

func TestCiGreen_UsageFace(t *testing.T) {
	decl, rec := ciGreenFixture(t, "unverified", []string{"core"}, "9999cccc", [][2]string{{"core", "success"}})

	cases := []struct {
		name string
		argv []string
	}{
		{"缺 --run", []string{"ci", "green", "--decl", decl}},
		{"--run 给两次", []string{"ci", "green", "--run", rec, "--run", rec, "--decl", decl}},
		{"多余位置参数", []string{"ci", "green", rec, "--decl", decl}},
		{"裸 --json", []string{"ci", "green", "--run", rec, "--decl", decl, "--json"}},
	}
	for _, c := range cases {
		rc, out, _ := ciGreenRun(t, c.argv...)
		if rc != 2 {
			t.Errorf("%s ⇒ rc=%d（要 2）", c.name, rc)
		}
		if out != "" {
			t.Errorf("%s ⇒ stdout 要 0 字节（现读 %d 字节）", c.name, len(out))
		}
	}

	// 字段表**两处同值**：命令树登记 ⟷ 裸 `--json` 时 stderr 印的那一行。
	info, ok := zerg.CommandInfoOfForTest(ciGreenCmd)
	if !ok {
		t.Fatalf("命令树里没有 %q（登记漏了）", ciGreenCmd)
	}
	if strings.Join(info.Fields, ",") != strings.Join(ciGreenFieldsWant, ",") {
		t.Errorf("命令树登记的字段表 = %v（要 %v）", info.Fields, ciGreenFieldsWant)
	}
	_, _, errb := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl, "--json")
	if !strings.Contains(errb, strings.Join(ciGreenFieldsWant, ",")) {
		t.Errorf("裸 `--json` 时 stderr 要印出同一张字段表：\n%s", errb)
	}
	// 说明面/用法串那两处也要与命令树同值（用法串里点名的旗标 = 解析器认的旗标）。
	for _, flag := range []string{"--run", "--decl", "--json"} {
		if !strings.Contains(info.Usage, flag) {
			t.Errorf("用法串缺 %s：%q", flag, info.Usage)
		}
	}
}

// ---- ⑩ 声明件 ⟷ `publish/ci/ci.yml` 现读对拍（两套真源并存就会漂 ⇒ 立判据） --------------

// ciJobNamesFromYML —— 从 CI 件里取「检查名」集合。
//
// 解析纪律（本仓序124 那条「抓取/解析要有解析器或**显式分界符**」，**不用正则兜散文**）：
//
//	· 状态机：见到**行首** `jobs:` 进 `jobs` 块，见到下一个**行首非空格**的键（`^[A-Za-z]`）出块；
//	· job id = 块内**恰好两个空格**缩进的键行 `^  <id>:`；
//	· 检查名 = 该 job 块里**第一条** `^    name: <值>` 的值；没有 `name:` ⇒ 用 job id
//	  （GitHub 面上「job 没写 name ⇒ 检查名就是 job id」—— 这一格是**口径**，写在这里一处）。
var ciJobIDRe = regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*$`)
var ciJobNameRe = regexp.MustCompile(`^    name:\s*(.+?)\s*$`)
var ciTopKeyRe = regexp.MustCompile(`^[A-Za-z]`)

func ciJobNamesFromYML(src string) []string {
	names := []string{}
	inJobs, cur := false, ""
	curName := ""
	flush := func() {
		if cur == "" {
			return
		}
		if curName != "" {
			names = append(names, curName)
		} else {
			names = append(names, cur)
		}
		cur, curName = "", ""
	}
	for _, ln := range strings.Split(src, "\n") {
		if ln == "jobs:" {
			inJobs = true
			continue
		}
		if inJobs && ciTopKeyRe.MatchString(ln) {
			flush()
			inJobs = false
			continue
		}
		if !inJobs {
			continue
		}
		if m := ciJobIDRe.FindStringSubmatch(ln); m != nil {
			flush()
			cur = m[1]
			continue
		}
		if cur != "" && curName == "" {
			if m := ciJobNameRe.FindStringSubmatch(ln); m != nil {
				curName = m[1]
			}
		}
	}
	flush()
	return names
}

// ciDeclNames —— 读一份声明件的 `checks`（现读真件用）。
func ciDeclNames(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读声明件 %s：%v", path, err)
	}
	var doc struct {
		ID     string   `json:"id"`
		Source string   `json:"source"`
		Checks []string `json:"checks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("声明件不是合的 JSON：%v", err)
	}
	if doc.ID != "zerg.ci.required-checks.v1" {
		t.Errorf("声明件形状号 = %q（要 zerg.ci.required-checks.v1）", doc.ID)
	}
	if doc.Source == "" {
		t.Error("声明件 `source` 是空的（「必需」那格从哪儿来的必须写下来）")
	}
	return doc.Checks
}

// ciSameSet —— 两个名字集合是不是**同集合**（去重后逐条比 · 差集逐条打印）。
func ciSameSet(a, b []string) (bool, []string, []string) {
	set := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	ma, mb := set(a), set(b)
	onlyA, onlyB := []string{}, []string{}
	for k := range ma {
		if !mb[k] {
			onlyA = append(onlyA, k)
		}
	}
	for k := range mb {
		if !ma[k] {
			onlyB = append(onlyB, k)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return len(onlyA) == 0 && len(onlyB) == 0, onlyA, onlyB
}

func TestCiGreen_DeclarationMatchesCIFile(t *testing.T) {
	root := repoRootFromCLI(t)
	ciPath := filepath.Join(root, "publish", "ci", "ci.yml")
	declPath := filepath.Join(root, "publish", "ci", "required-checks.json")

	ciRaw, err := os.ReadFile(ciPath)
	if err != nil {
		t.Fatalf("读 CI 件 %s：%v", ciPath, err)
	}
	ymlNames := ciJobNamesFromYML(string(ciRaw))
	if len(ymlNames) == 0 {
		t.Fatalf("CI 件里一个 job 名都没解析出来（解析器空转不许当绿）：%s", ciPath)
	}
	declNames := ciDeclNames(t, declPath)
	if len(declNames) == 0 {
		t.Fatalf("声明件里一个检查名都没有：%s", declPath)
	}

	ok, onlyDecl, onlyYML := ciSameSet(declNames, ymlNames)
	if !ok {
		t.Errorf("声明集合 ⟷ `publish/ci/ci.yml` 的 job 名**不同集合**：\n  只声明件有：%v\n  只 CI 件有：%v\n"+
			"⇒ 两套真源并存就会漂；改了 `ci.yml` 的 job 名 ⇒ **同批**改声明件", onlyDecl, onlyYML)
	}
	t.Logf("现读对拍 ✓ 声明 %d 条 ⟷ CI 件 job 名 %d 条（同集合）", len(declNames), len(ymlNames))

	// 成对负控：合成 yml 里改一个 job 名 ⇒ 判定口当场报差（证明这一格不是恒绿）。
	synth := "name: CI\non:\n  push:\n    branches: [main]\njobs:\n  core:\n    name: 改过的名字\n  docs:\n    name: 文档双语门禁（配对/标题/链接）\n"
	synthNames := ciJobNamesFromYML(synth)
	if len(synthNames) != 2 {
		t.Fatalf("合成 yml 解析出 %d 个 job 名（要 2）：%v", len(synthNames), synthNames)
	}
	okN, onlyA, onlyB := ciSameSet(declNames, synthNames)
	if okN {
		t.Error("负控失败：改过 job 名的合成件竟判成「同集合」（判定口没有区分度）")
	}
	if len(onlyA) == 0 || len(onlyB) == 0 {
		t.Errorf("负控失败：差集没算出来（onlyDecl=%v onlyYML=%v）", onlyA, onlyB)
	}
}

// ---- ⑪ 零副作用：两份件跑前跑后逐字同 + 目录一件不多 ------------------------------------

func TestCiGreen_ReadOnlyZeroSideEffect(t *testing.T) {
	decl, rec := ciGreenFixture(t, "unverified", []string{"core", "docs"}, "abc98765",
		[][2]string{{"core", "success"}, {"docs", "success"}})

	sum := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("读 %s：%v", p, err)
		}
		h := sha256.Sum256(b)
		return hex.EncodeToString(h[:])
	}
	beforeDecl, beforeRec := sum(decl), sum(rec)
	dirEntries := func() []string {
		ents, err := os.ReadDir(filepath.Dir(decl))
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, e := range ents {
			out = append(out, e.Name())
		}
		sort.Strings(out)
		return out
	}
	beforeList := dirEntries()

	for i := 0; i < 2; i++ { // 跑两遍（第二遍不许因为第一遍留下什么而变）
		if rc, _, errb := ciGreenRun(t, "ci", "green", "--run", rec, "--decl", decl); rc != 0 {
			t.Fatalf("第 %d 遍 rc=%d · stderr=%s", i+1, rc, errb)
		}
	}

	if afterDecl := sum(decl); afterDecl != beforeDecl {
		t.Errorf("声明件被改了：%s ⟶ %s", beforeDecl, afterDecl)
	}
	if afterRec := sum(rec); afterRec != beforeRec {
		t.Errorf("记录件被改了：%s ⟶ %s", beforeRec, afterRec)
	}
	if afterList := dirEntries(); strings.Join(afterList, ",") != strings.Join(beforeList, ",") {
		t.Errorf("夹具目录多了/少了件：%v ⟶ %v", beforeList, afterList)
	}
}

// ---- ⑫ 默认落点：不给 `--decl` ⇒ 用仓根下的声明件（口径行里印出它） ---------------------

func TestCiGreen_DefaultDeclarationPath(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	declPath := filepath.Join(root, "publish", "ci", "required-checks.json")
	names := ciDeclNames(t, declPath)

	// 拿**真声明件**的名单造一份全 success 的记录件 ⇒ 这一路同时证明：真声明件**解析得上**、
	// 且「一条只读命令出归绿读数」在真件上成立。
	rows := make([][2]string, 0, len(names))
	for _, n := range names {
		rows = append(rows, [2]string{n, "success"})
	}
	dir := ciGreenDir(t)
	rec := ciGreenWrite(t, filepath.Join(dir, "run.json"),
		ciGreenRecordBody("zerg.ci.check-runs.v1", "0123456789abcdef0123456789abcdef01234567", rows))

	rc, out, errb := ciGreenRun(t, "ci", "green", "--run", rec)
	if rc != 0 {
		t.Fatalf("不给 `--decl` ⇒ 走默认落点 · rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "publish/ci/required-checks.json") {
		t.Errorf("默认落点要印出来（读者要知道判的是哪一份声明）：\n%s", errb)
	}
	if !strings.Contains(out, "0123456789abcdef0123456789abcdef01234567") {
		t.Errorf("读数要带 head_sha：%q", out)
	}
}
