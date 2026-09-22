// cli_matrix_test.go —— 命令面自身的**契约测试 · 第一/二层**（§九 M17 · 开工单 T-42）。
//
// 落点：**外部测试包 `package main_test`**（判据②）—— 它只能看见导出标识符，
// 命令树/`run`/退码表这些未导出面由 `export_test.go` 那座**只读桥**开出来
// （桥直接指真源，不另抄副本）。
//
// 三层测试的落点（§九 M17「三层测试落点」）：
//
//	层① **进程内 · 契约面**（本文件）：`RunForTest` 直接跑一条命令，判退码与 stdout 字节数。
//	层② **进程内 · 静态面**（本文件）：矩阵覆盖（无空白/无幽灵/每条命令 ≥1 must-fail）·
//	     退码表唯一（同数字两义 ⇒ 红）· 包封形状。
//	层③ **进程外 · 合成夹具**（cli_exec_test.go）：纯 Go `os/exec` + 自写夹具，**不引依赖**。
//
// 一条硬纪律：**每条口令都必须是 must-fail**（`want_rc != 0`）。矩阵里若混进一条 rc=0 的，
// 本测试当场红 —— 「must-fail 矩阵」不许被稀释成「随便跑几条」。
package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

type matrixCase struct {
	ID              string   `json:"id"`
	Command         string   `json:"command"`
	Argv            []string `json:"argv"`
	WantRC          int      `json:"want_rc"`
	WantStdoutBytes int      `json:"want_stdout_bytes"`
	Why             string   `json:"why"`
	// ── 公开面档（两档矩阵 · 设计-CI适配-v1.1 §十三 `M2`）─────────────────────────────
	// 为什么需要分档：**同一格被两棵树要求不同值**。公开树缺三样本机有的东西 ——
	//   · `bin/`（`publish/whitelist.txt` 不含它）⇒ `build ls` 先撞缺件退 8；
	//   · `scripts/gates/precommit-gates.sh`（`publish-public.sh` 的 EXCLUDES 里的发布机制自举件）
	//     ⇒ `gate run` 先撞「门禁最小入口不在」退 2；
	//   · `scripts/calib/*` 与 `scripts/evals/compare.py`（公开面只进 `make-manifest.py`）
	//     ⇒ `calib run` / `eval run` 先撞「件不在盘上」退 8。
	// 三样都让 CLI **走不到**本机那套参数校验 ⇒ 把单一真源整体改成公开面的值会让**本机立刻变红**
	// （那是挪红，不是修红）；反过来只保本机值则公开面永远红。
	// ⇒ 分两档：本机档 = 上面既有两列（`want_rc` / `want_stdout_bytes`，**一字不动**）；
	//   公开面档 = 下面两个**可选覆盖列**。
	// **公开面档存哪（写死）**：就存本件（`core/cmd/zerg/testdata/cli-matrix.json`）的 `cases[]` 里，
	// 不另开第二份矩阵 —— 单一真源仍是本件。`check-cli-contract.py --emit-matrix` 的增量合并
	// 只刷新 `want_rc`/`want_stdout_bytes` 两列（`hit.update(d)`），`id/command/argv/why` 与本组
	// 公开面列**同等逐字保留**（所以「在公开树上跑一次 emit」不会把本机档改掉 —— 何况公开树
	// 没有 `bin/zerg`，emit 在那儿本来就给不出结论，退 2）。
	// 用指针是为了分清「显式给 0」与「这一格没分档」。
	WantRCPublic          *int   `json:"want_rc_public"`
	WantStdoutBytesPublic *int   `json:"want_stdout_bytes_public"`
	WhyPublic             string `json:"why_public"`
}

// matrixTierDecl —— 档位声明（`tiers` 段一格）：**公开面档存哪要写死**，不写在聊天里。
type matrixTierDecl struct {
	Where string `json:"where"`
	Judge string `json:"judge"`
	Why   string `json:"why"`
}

type matrixFile struct {
	ID         string `json:"id"`
	Exemptions []struct {
		Command string `json:"command"`
		Reason  string `json:"reason"`
	} `json:"exemptions"`
	Tiers struct {
		Local  matrixTierDecl `json:"local"`
		Public matrixTierDecl `json:"public"`
	} `json:"tiers"`
	Cases []matrixCase `json:"cases"`
}

// ── 两档矩阵的档位（设计 §十三 `M2`）──────────────────────────────────────────────
const (
	tierLocal  = "local"
	tierPublic = "public"
)

// matrixTier —— **唯一一处**档位判据：仓根有没有 `bin/`。
// 为什么拿 `bin/` 当判据：它正是两档不同的**同一个成因**（公开面白名单不含 `bin/`；缺它
// ⇒ `build ls` 先撞缺件退 8）。用「本次红因本身」当档位判据，不另立一个「我在哪棵树」的开关。
// `ZERG_MATRIX_TIER=local|public` 是**取证用**的显式覆盖（同机两跑对照）：写错即 Fatal（假读数）。
func matrixTier(t *testing.T) string {
	t.Helper()
	if v := strings.TrimSpace(os.Getenv("ZERG_MATRIX_TIER")); v != "" {
		if v != tierLocal && v != tierPublic {
			t.Fatalf("ZERG_MATRIX_TIER=%q 不是 local/public —— 取证档位写错就是假读数", v)
		}
		return v
	}
	root := matrixRepoRoot()
	if root == "" {
		t.Fatal("解析不到仓根（沿途 8 级都没有 core/go.mod）⇒ **判不出档位**：不给结论，不许默认成某一档")
	}
	if st, err := os.Stat(filepath.Join(root, "bin")); err == nil && st.IsDir() {
		return tierLocal
	}
	return tierPublic
}

// tierJudgeLine —— 档位判据的可读面（打印用；与 matrixTier **同一判据**，不另立说法）。
// ★ 显式覆盖时必须**照实**打出来（否则日志会把「被迫的档位」说成「判据判出来的档位」——
// 取证时最容易被自己骗过去的就是这一句）。
func tierJudgeLine(t *testing.T, tier string) string {
	obs, auto := "仓根（解析不到）", "（判不出）"
	if root := matrixRepoRoot(); root != "" {
		obs = "仓根 " + root
		if st, err := os.Stat(filepath.Join(root, "bin")); err == nil && st.IsDir() {
			obs += " 有 bin/"
			auto = tierLocal
		} else {
			obs += " **没有 bin/**"
			auto = tierPublic
		}
	}
	tierWhy := map[string]string{
		tierLocal:  "本机档（= 既有两列 want_rc/want_stdout_bytes）",
		tierPublic: "公开面档（= 可选覆盖列 want_rc_public/want_stdout_bytes_public）",
	}
	if v := strings.TrimSpace(os.Getenv("ZERG_MATRIX_TIER")); v != "" {
		return fmt.Sprintf("%s ⇒ 自动判据本该是 %s 档，但被**显式覆盖 ZERG_MATRIX_TIER=%s** 越过（本跑档位 = %s）",
			obs, auto, v, tier)
	}
	return fmt.Sprintf("%s ⇒ %s 档｜%s", obs, auto, tierWhy[auto])
}

// matrixRepoRoot —— 从测试进程的 cwd 往上找带 `core/go.mod` 的那一级（最多 8 级）。
func matrixRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	d := wd
	for i := 0; i < 8; i++ {
		if st, err := os.Stat(filepath.Join(d, "core", "go.mod")); err == nil && !st.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}

// wantFor —— 按档位取「这一格的期望值」：本机档 = 既有两列；公开面档 = 可选覆盖列
// （没给覆盖列 ⇒ 回落到本机列 —— **两档同值就是常态**，只有缺件面那几格才真不同）。
func wantFor(c matrixCase, tier string) matrixCase {
	if tier != tierPublic {
		return c
	}
	if c.WantRCPublic != nil {
		c.WantRC = *c.WantRCPublic
	}
	if c.WantStdoutBytesPublic != nil {
		c.WantStdoutBytes = *c.WantStdoutBytesPublic
	}
	return c
}

func loadMatrix(t *testing.T) matrixFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "cli-matrix.json"))
	if err != nil {
		t.Fatalf("读不到矩阵（testdata/cli-matrix.json）：%v", err)
	}
	var m matrixFile
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("矩阵不是合法 JSON：%v", err)
	}
	if m.ID != "cli-contract.v1" {
		t.Fatalf("矩阵 id = %q（要 cli-contract.v1）", m.ID)
	}
	if len(m.Cases) == 0 {
		t.Fatal("矩阵一个 case 都没有（空转 = 假覆盖）")
	}
	// 两档矩阵（§十三 `M2`）：**公开面档存哪要写死**，写不清就是「档位存在聊天里」⇒ 不给结论。
	if strings.TrimSpace(m.Tiers.Public.Where) == "" {
		t.Fatal("矩阵没写「公开面档存哪」（`tiers.public.where` 是空的）—— §十三 M2 要求写死；" +
			"写不清的档位等于没有档位（不给结论，不许默认成某一档）")
	}
	return m
}

// judgeCase 是本测试的**唯一判定口**（判据①「真的红」的落点）。
// 抽成函数是为了让负控能直接喂一个错的期望值进来（见 TestCLIContractHarnessNegativeControl）。
func judgeCase(c matrixCase, rc, stdoutBytes int) error {
	if rc != c.WantRC {
		return fmt.Errorf("退码 %d ≠ 期望 %d", rc, c.WantRC)
	}
	if stdoutBytes != c.WantStdoutBytes {
		return fmt.Errorf("stdout %d 字节 ≠ 期望 %d 字节", stdoutBytes, c.WantStdoutBytes)
	}
	return nil
}

// runCase 跑一条口令（进程内 · 层①）。stdin 一律为空：**无 TTY 零提示词**（K6）。
func runCase(c matrixCase) (int, int) {
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(c.Argv, &out, &errb)
	return rc, out.Len()
}

// TestCLIContractMustFailMatrix —— 判据①：每条口令**真的红**（退码 + stdout 字节数逐条对）。
// 两档矩阵（§十三 `M2`）：判据取 `wantFor(c, 档位)` —— 本机档 = 既有两列；公开面档 = 可选覆盖列。
func TestCLIContractMustFailMatrix(t *testing.T) {
	m := loadMatrix(t)
	tier := matrixTier(t)
	two := twoTierCases(m)
	t.Logf("档位 = %s ｜ 判据：%s", tier, tierJudgeLine(t, tier))
	t.Logf("公开面档存哪（写死）：%s ｜ 档位判据声明：%s", m.Tiers.Public.Where, m.Tiers.Public.Judge)
	for _, c := range two {
		w := wantFor(c, tier)
		t.Logf("  两档格 %s：本档(%s) 期望 rc=%d/stdout=%d 字节 ｜ 本机档 %d/%d ｜ 公开面档 %d/%d ｜ 公开面为什么不同：%s",
			c.ID, tier, w.WantRC, w.WantStdoutBytes, c.WantRC, c.WantStdoutBytes,
			effRC(c, tierPublic), effBytes(c, tierPublic), c.WhyPublic)
	}
	bad, n := 0, 0
	for _, c := range m.Cases {
		if c.WantRC == 0 {
			t.Errorf("case %s 的 want_rc = 0 —— must-fail 矩阵不许有 rc=0 的条目（判据①）", c.ID)
			bad++
			continue
		}
		rc, outBytes := runCase(c)
		if err := judgeCase(wantFor(c, tier), rc, outBytes); err != nil {
			t.Errorf("case %s（档 %s · %v · %s）不红/形状变了：%v", c.ID, tier, c.Argv, c.Why, err)
			bad++
		}
		n++
	}
	t.Logf("矩阵：%d 条 case 跑完 · 不符 %d 条 · 命令覆盖 %d 条 · 两档格 %d 条",
		n, bad, len(commandsInMatrix(m)), len(two))
}

// twoTierCases —— 分了档的格（有任一公开面覆盖列）。空 = 分档没落地（见负控）。
func twoTierCases(m matrixFile) []matrixCase {
	out := []matrixCase{}
	for _, c := range m.Cases {
		if c.WantRCPublic != nil || c.WantStdoutBytesPublic != nil {
			out = append(out, c)
		}
	}
	return out
}

func effRC(c matrixCase, tier string) int    { return wantFor(c, tier).WantRC }
func effBytes(c matrixCase, tier string) int { return wantFor(c, tier).WantStdoutBytes }

func commandsInMatrix(m matrixFile) map[string]bool {
	set := map[string]bool{}
	for _, c := range m.Cases {
		set[c.Command] = true
	}
	return set
}

// TestCLIContractMatrixNoBlank —— 判据④「矩阵不留空白」（§九 M17 `T5` 的进程内一半）：
//
//	① 命令树里**每条**命令都要有 ≥1 条 case，或一条**带 reason** 的豁免；
//	   没有 case 又没有 reason 的豁免 ⇒ 空白（红）；有豁免但 reason 空 ⇒ **不给结论**（测试直接 Fatal）。
//	② 矩阵里**不许有幽灵**（case 点名的命令在命令树里不存在）。
func TestCLIContractMatrixNoBlank(t *testing.T) {
	m := loadMatrix(t)
	have := map[string]int{}
	for _, c := range m.Cases {
		have[c.Command]++
	}
	exempt := map[string]string{}
	for _, e := range m.Exemptions {
		if strings.TrimSpace(e.Reason) == "" {
			t.Fatalf("豁免 %q **没有 reason** —— 写不清理由的豁免就是洗白（§九 M17 T5：reason 缺 ⇒ 不给结论 2）", e.Command)
		}
		exempt[e.Command] = e.Reason
	}
	tree := zerg.CommandPathsForTest()
	if len(tree) == 0 {
		t.Fatal("命令树读出来是空的（空转 = 假覆盖）")
	}
	blanks := []string{}
	for _, p := range tree {
		if have[p] == 0 && exempt[p] == "" {
			blanks = append(blanks, p)
		}
	}
	if len(blanks) > 0 {
		sort.Strings(blanks)
		t.Errorf("矩阵有 %d 条**空白**（命令没有 must-fail case 也没有带 reason 的豁免）：%s",
			len(blanks), strings.Join(blanks, " · "))
	}
	ghosts := []string{}
	for _, p := range sortedKeys(have) {
		if _, ok := zerg.CommandInfoOfForTest(p); !ok {
			ghosts = append(ghosts, p)
		}
	}
	if len(ghosts) > 0 {
		t.Errorf("矩阵里有 %d 条**幽灵条目**（命令树里没有这条命令）：%s", len(ghosts), strings.Join(ghosts, " · "))
	}
	t.Logf("命令树 %d 条 · 有 case 的 %d 条 · 带 reason 的豁免 %d 条 · 空白 %d 条",
		len(tree), len(have), len(exempt), len(blanks))
}

// TestCLIContractHarnessNegativeControl —— **成对负控**：先证明这台机器能把红判出来，
// 再谈「全过」。喂一条改正过的期望值，judgeCase 必须报错（否则本测试就是恒绿装置）。
func TestCLIContractHarnessNegativeControl(t *testing.T) {
	m := loadMatrix(t)
	c := m.Cases[0]
	rc, outBytes := runCase(c)
	if err := judgeCase(c, rc, outBytes); err != nil {
		t.Fatalf("负控第 0 步就不成立：真值都判不过（%v）", err)
	}
	wrong := c
	wrong.WantRC = c.WantRC + 1
	if err := judgeCase(wrong, rc, outBytes); err == nil {
		t.Error("负控失败：期望值改错后**没有**报错 —— 这台机器判不出红（恒绿装置，不许当覆盖）")
	}
	wrong2 := c
	wrong2.WantStdoutBytes = c.WantStdoutBytes + 1
	if err := judgeCase(wrong2, rc, outBytes); err == nil {
		t.Error("负控失败：stdout 字节数改错后**没有**报错")
	}
}

// TestCLIContractMatrixTierNegativeControl —— 两档机制（设计 §十三 `M2`）的**成对负控**：
//
//	① 两档格**确实存在**（一条都没有 ⇒ 「分档」是空转）；
//	② 每条两档格的**两档值必须不同**（同值 ⇒ 那格本不该分档 —— 分档是给缺件面用的，不是洗白用的）；
//	③ 拿**另一档**的期望值去判**本档现跑** ⇒ **必红**（证明档位真的进了判决，不是只被打印出来）。
//
// ③ 是本件最要紧的一条：本机档跑 ⇒ 用公开面档的期望必红；公开面档跑 ⇒ 用本机档的期望必红。
// 少一条（比如只比「本档判得过」）就分不清「分档生效」与「两档都恰好同值」。
func TestCLIContractMatrixTierNegativeControl(t *testing.T) {
	m := loadMatrix(t)
	tier := matrixTier(t)
	other := tierLocal
	if tier == tierLocal {
		other = tierPublic
	}
	two := twoTierCases(m)
	if len(two) == 0 {
		t.Fatal("矩阵里一条两档格都没有 ⇒ 「分档」没落地（空转 ⇒ 不给结论）")
	}
	bad := 0
	for _, c := range two {
		if effRC(c, tierLocal) == effRC(c, tierPublic) && effBytes(c, tierLocal) == effBytes(c, tierPublic) {
			t.Errorf("两档格 %s 的两档值**完全相同**（本机 %d/%d = 公开面 %d/%d）—— 同值就不该分档（分档只给缺件面用）",
				c.ID, effRC(c, tierLocal), effBytes(c, tierLocal), effRC(c, tierPublic), effBytes(c, tierPublic))
			bad++
		}
		rc, outBytes := runCase(c)
		if err := judgeCase(wantFor(c, tier), rc, outBytes); err != nil {
			t.Errorf("负控第 0 步不成立：两档格 %s 在**本档**（%s）都判不过（%v）", c.ID, tier, err)
			bad++
			continue
		}
		if err := judgeCase(wantFor(c, other), rc, outBytes); err == nil {
			t.Errorf("负控失败：两档格 %s 拿**另一档**（%s）的期望值也判过了 —— 档位没进判决（分档是摆设）", c.ID, other)
			bad++
		}
	}
	t.Logf("两档格 %d 条 · 本档 = %s（另一档 = %s）⇒ 每格：本档判过 · 另一档必红（失败 %d 条）",
		len(two), tier, other, bad)
}

// TestCLIContractExitCodeTableUnique —— 判据③「退码表唯一」的**进程内**一半：
// 表内数字不许重复（同数字两义 ⇒ 红），每格三要素齐（号/名/语义）。
func TestCLIContractExitCodeTableUnique(t *testing.T) {
	rows := zerg.ExitCodeTableForTest()
	if len(rows) == 0 {
		t.Fatal("退码表读出来是空的（空转 = 假覆盖）")
	}
	seen := map[int]string{}
	dups := []string{}
	for _, r := range rows {
		if r.Name == "" || r.Meaning == "" {
			t.Errorf("退码 %d 的三要素不齐（名=%q 语义=%q）", r.Code, r.Name, r.Meaning)
		}
		if prev, ok := seen[r.Code]; ok {
			dups = append(dups, fmt.Sprintf("%d 同时是 %q 与 %q", r.Code, prev, r.Name))
			continue
		}
		seen[r.Code] = r.Name
	}
	if len(dups) > 0 {
		t.Errorf("退码表内**同数字两义** %d 处：%s（§十二 P-013 占号纪律：禁止同数字两义）",
			len(dups), strings.Join(dups, " · "))
	}
	t.Logf("退码表：%d 格 · 数字唯一 ✓（%v）", len(rows), sortedCodes(rows))
}

func sortedCodes(rows []zerg.ExitCodeRowForTest) []int {
	out := []int{}
	for _, r := range rows {
		out = append(out, r.Code)
	}
	sort.Ints(out)
	return out
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// runCapture 跑一条口令并把两路输出都带回来（本文件里给「要判输出文本」的测试用）。
func runCapture(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestCLIContractSubmitSliceCodesPassthrough —— 判据 T-44 ②：片的 400 语义**透传不翻译**。
//
// 合成主控（`net/http` 起在本机随机端口 · **不碰生产 8580** · 用完即关）：按 API 声明
// （`core/internal/api/routes.go` 的 `/api/tasks` POST）回 400 + **片语义码**，命令面必须
// **逐字**把它打出来、**不替换**、**不吞**，退码从契约表取（400 = 用法错 `2`）。
// 顺带钉住旗标 ↔ 请求体的映射（缺映射就会在这里露出来）。
func TestCLIContractSubmitSliceCodesPassthrough(t *testing.T) {
	seen := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/tasks" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		seen = append(seen, string(body))
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		switch {
		case m["slice_id"] == nil && m["depends_on"] == nil && m["acceptance"] == nil:
			w.WriteHeader(400)
			fmt.Fprint(w, `{"code":"SLICE_MISSING_ID","error":"声明了 slice 字段但缺 slice_id"}`)
		case m["slice_id"] != nil && m["acceptance"] == nil:
			w.WriteHeader(400)
			fmt.Fprint(w, `{"code":"SLICE_MISSING_ACCEPTANCE","error":"slice 字段已声明但没给 acceptance 声明"}`)
		default:
			w.WriteHeader(201)
			fmt.Fprint(w, `{"id":"synthetic-task-1","status":"queued"}`)
		}
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	t.Setenv("ZERG_PORT", strconv.Itoa(port))

	cases := []struct {
		argv   []string
		wantRC int
		want   string
	}{
		{[]string{"task", "submit", "--desc", "合成", "--yes"}, 2, "SLICE_MISSING_ID"},
		{[]string{"task", "submit", "--desc", "合成", "--slice-id", "S-1", "--yes"}, 2, "SLICE_MISSING_ACCEPTANCE"},
		{[]string{"task", "submit", "--desc", "合成", "--acceptance", "判据甲", "--yes"}, 0, "synthetic-task-1"},
	}
	for _, c := range cases {
		rc, out, errb := runCapture(c.argv...)
		if rc != c.wantRC {
			t.Errorf("%v 退码 = %d（要 %d）· stderr=%s", c.argv, rc, c.wantRC, errb)
		}
		if !strings.Contains(out+errb, c.want) {
			t.Errorf("%v 的输出里没有逐字的 %q（透传不翻译）：stdout=%s stderr=%s", c.argv, c.want, out, errb)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("合成主控收到 %d 次请求（要 3 次）", len(seen))
	}
	// 映射面：`--slice-id` → `slice_id`、`--acceptance` → `acceptance`（可重复）
	if !strings.Contains(seen[1], `"slice_id":"S-1"`) {
		t.Errorf("第二次请求体没有把 --slice-id 映射成 slice_id：%s", seen[1])
	}
	if !strings.Contains(seen[2], `"acceptance":["判据甲"]`) {
		t.Errorf("第三次请求体没有把 --acceptance 映射成数组：%s", seen[2])
	}
}

// TestCLIContractJSONShape —— 判据⑥/`T4`「`--json` 形状守卫（运行时只读）」（§九 M6）：
// 跑一条**离线**命令，逐键核包封六键 + `schema` 取值 + `items` 恒数组。
// 门 `check-cli-contract.py` 的 T4 就是调这一条（`-run TestCLIContractJSONShape`）：
// 它读的是**当前源码**的运行期行为，不是 `bin/` 里的旧制品。
func TestCLIContractJSONShape(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := zerg.RunForTest([]string{"version", "--json", "name"}, &out, &errb); rc != 0 {
		t.Fatalf("version --json name 退码 = %d（要 0）· stderr=%s", rc, errb.String())
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("--json 输出不是对象：%v", err)
	}
	for _, k := range zerg.EnvelopeKeysForTest() {
		if _, ok := doc[k]; !ok {
			t.Errorf("包封缺键 %q（§九 M6 I1：六键恒在）", k)
		}
	}
	for k := range doc {
		found := false
		for _, want := range zerg.EnvelopeKeysForTest() {
			if k == want {
				found = true
			}
		}
		if !found {
			t.Errorf("包封多了未声明键 %q（真源 = EnvelopeKeysForTest）", k)
		}
	}
	var schema string
	if err := json.Unmarshal(doc["schema"], &schema); err != nil || schema != zerg.ContractSchemaForTest() {
		t.Errorf("schema = %s（要 %q）", string(doc["schema"]), zerg.ContractSchemaForTest())
	}
	if !bytes.HasPrefix(bytes.TrimSpace(doc["items"]), []byte("[")) {
		t.Errorf("items 不是数组：%s（§九 M6 I3：恒数组、永不为 null）", string(doc["items"]))
	}
}
