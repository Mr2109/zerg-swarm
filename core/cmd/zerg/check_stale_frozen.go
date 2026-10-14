// check_stale_frozen.go —— 只读：列出盘上「冻结件」的**冻结值 vs 现读** ＋ 刷新正门。
//
// 搬自 scripts/gates/check-stale-frozen.py（GAP-20261005-31 首版）。它治的是「门间连锁面无面看着」：
// 改一个被冻结的源件后，下游多处冻结值各自变红，但没有一处能同批列「改了 X ⇒ 会红 Y,Z」。
//
// ★ 本件属【生成器/制品类】（父方裁定）：適格 = 可复现对拍，**不是**「零 subprocess」。
//   本件 py 原件的行为面分三档：
//     · 纯 IO 面（judge-registry / real-gates.sh / 公开标记.tsv）—— Go 侧**原生**实现；
//     · **制品/子进程面**（跑现役 bin/zerg help×4 取行数/字节数）—— Go 侧 exec.Command 直接复刻；
//     · **跨件 importlib 面**（affected-map / cli-contract / help-shape / approve-backfill 四件
//       经 importlib 现载兄弟判据件取值）—— ✗ 不在 Go 侧重写（＝第二条实现＝双真源），改经
//       **python3 子进程**用同一 importlib 载入同一兄弟件取同一读数，**锁兄弟件 sha**（见回执）。
//
// 出厂声明 (a) **默认根定位差异**：py `DEFAULT_ROOT = dirname(dirname(HERE))`（脚本位置上溯两级 = 仓根）；
//   本件**无 `__file__` 语境** ⇒ 不给 `--root` 时从 **cwd** 上溯 ≤6 层找含 `scripts/gates/` 的目录，
//   找不到回落 `abspath(cwd)`。「脚本位置 ≠ cwd 上溯根」时路径推导面不同 ⇒ 不属逐字对齐承诺，
//   对拍一律显式传 `--root` / cwd=仓根。
//
// 出厂声明 (b) **自指检查**：
//   · 本件落点在 `core/cmd/zerg/`，**不在** `scripts/gates/` 直下 ⇒ 不落进本件自己的扫描面
//     （face⑦ 只列 `scripts/gates/*.py|*.sh` 直下门件；face① 只读 judge-registry.json 的 entries）。
//   · 件头/正文**不写**任何他门触发形态（`"kind": "…"` 一类连续键值原串、`setErr(` 连续原串）。
//   · 无写面（默认档只读）・无随机/时刻字面（`--json` 只报只读值）。

// ── 以下为桥/参数/自检/入口段（同件第二段）────────────────────────────

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

const csfProg = "check-stale-frozen.py"

// csfJSONFields —— 本门机器面字段表（**唯一真源**：`--json <字段>` 的装包与拒收同读这一处）。
// 定序 = csfJSONPayload 的全量键序；子集输出按用户给的字段序。
var csfJSONFields = []string{"root", "faces", "stale_points", "stale_faces", "rc", "mode"}

// csfRefresh 的键与 py 的 REFRESH 字典逐字同（仅按键取值，不被迭代 ⇒ 无插入序问题）；值改现役形（py 原件已退 ⇒ 调用串走 zerg gate check）。
var csfRefresh = map[string]string{
	"judge-registry":   "zerg gate check check-judge-registry --emit",
	"affected-map":     "zerg gate check check-affected-map --emit",
	"cli-contract":     "zerg gate check check-cli-contract --emit-matrix（★ count 两键按收口批随动 · 门78/门⑪ 现读对拍）",
	"help-shape":       "zerg gate check check-help-shape --emit-baseline（现读打印 · 照抄 · 禁手写）",
	"approve-backfill": "zerg gate check check-approve-backfill --refresh-constant（先干跑 · 加 --yes 才落）",
	"real-gates":       "把新 .sh 门件同批登记进 real-gates.sh 的 SH_TABLE 与字面 SH_GATES（两边不许漂移）",
	"public-marks":     "在 scripts/公开标记.tsv 补一行（文件\\tpublic\\t理由）",
}

type csfDetail struct {
	label  string
	frozen any
	live   any
	note   string
}

type csfFace struct {
	jian   string // 件
	xtai   string // 冻结形态
	status string // 状态
	dong   string // 冻结点
	xian   string // 现读
	stale  int    // 过期数
	detail []csfDetail
	zheng  string // 刷新正门
}

func csfNewFace(jian, xtai, zheng string) *csfFace {
	return &csfFace{jian: jian, xtai: xtai, status: "一致", detail: []csfDetail{}, zheng: zheng}
}

func (f *csfFace) staleAdd(label string, frozen, live any, note string) {
	f.stale++
	f.status = "过期"
	f.detail = append(f.detail, csfDetail{label: label, frozen: frozen, live: live, note: note})
}

// ── py 值语义小工具 ────────────────────────────────────────────────────
func csfPctS(v any) string { // py "%s" % v  == str(v)
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func csfRuneSlice16(s string) string { // py str[:16] 按码点切
	r := []rune(s)
	if len(r) > 16 {
		r = r[:16]
	}
	return string(r)
}

func csfNumInt(v any) (int, bool) {
	switch t := v.(type) {
	case json.Number:
		n, err := strconv.Atoi(t.String())
		if err != nil {
			f, err2 := t.Float64()
			if err2 != nil {
				return 0, false
			}
			return int(f), true
		}
		return n, true
	case int:
		return t, true
	case float64:
		return int(t), true
	}
	return 0, false
}

// csfPyEq 复刻 py `a != b` 的常见档：两者 str 化相等即相等（None/int/str 混排的近似对齐）。
func csfPyEq(a, b any) bool { return csfPctS(a) == csfPctS(b) }

// csfSplitLines 复刻 py str.splitlines() 的行边界集（\n \r \r\n \v \f \x1c-\x1e \x85 \u2028 \u2029）。
func csfSplitLines(s string) []string {
	out := []string{}
	var cur strings.Builder
	r := []rune(s)
	i := 0
	for i < len(r) {
		c := r[i]
		switch {
		case c == '\n' || c == '\v' || c == '\f' || c == '\u001c' || c == '\u001d' || c == '\u001e' || c == '\u0085' || c == '\u2028' || c == '\u2029':
			out = append(out, cur.String())
			cur.Reset()
			i++
		case c == '\r':
			out = append(out, cur.String())
			cur.Reset()
			i++
			if i < len(r) && r[i] == '\n' {
				i++
			}
		default:
			cur.WriteRune(c)
			i++
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func csfSha256File(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// csfPyJsonStr 复刻 json.dumps(..., ensure_ascii=False) 的单串转义。
func csfPyJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if c < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", c))
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func csfJSONAny(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return csfPyJSONStr(t)
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(t)
	default:
		return csfPyJSONStr(fmt.Sprintf("%v", t))
	}
}

// ── ① judge-registry.json（纯 IO · 原生）─────────────────────────────────
func csfProbeJudgeRegistry(root string) *csfFace {
	rel := "scripts/gates/judge-registry.json"
	f := csfNewFace(rel, "每件 sha256 缓存键（cache_key = sha256|tool_version）＋ count", csfRefresh["judge-registry"])
	p := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(p)
	if err != nil {
		f.status = "取不到"
		f.xian = "注册表读不到：" + csfOSErr(err, p)
		return f
	}
	var reg map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&reg); err != nil {
		f.status = "取不到"
		f.xian = "注册表读不到：" + csfJSONErr(err)
		return f
	}
	entriesA, _ := reg["entries"].([]any)
	f.dong = fmt.Sprintf("entries %d 条 · count=%s", len(entriesA), csfPctS(reg["count"]))
	f.xian = fmt.Sprintf("entries %d 条（盘上现读件名表）", len(entriesA))
	if !csfPyEq(reg["count"], len(entriesA)) {
		f.staleAdd("count ⟷ len(entries)", reg["count"], len(entriesA), "自洽")
	}
	for _, ea := range entriesA {
		e, _ := ea.(map[string]any)
		relE := csfPctS(e["path"])
		frozen := e["sha256"]
		fp := filepath.Join(root, filepath.FromSlash(relE))
		if fi, err := os.Stat(fp); err != nil || fi.IsDir() {
			f.staleAdd(relE, frozen, "（盘上无此件）", "表里有件盘上无")
			continue
		}
		live, err := csfSha256File(fp)
		if err != nil {
			f.staleAdd(relE, frozen, "（盘上无此件）", "表里有件盘上无")
			continue
		}
		if !csfPyEq(live, frozen) {
			f.staleAdd(relE, frozen, live, "缓存键过期 ⇒ 门79 红")
		}
	}
	return f
}

// ── ② affected-map.json（JSON 归 Go；sibling build_map 走 python 桥）──────
func csfProbeAffectedMap(root string, stderr io.Writer) *csfFace {
	rel := "scripts/gates/affected-map.json"
	f := csfNewFace(rel, "input_fingerprint（入键=步表文本＋点名件 sha256＋config＋newcmd · 内容相对 · ✗ 不含 HEAD）· source_head（仅溯源标注）", csfRefresh["affected-map"])
	p := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(p)
	if err != nil {
		f.status = "取不到"
		f.xian = "声明表读不到：" + csfOSErr(err, p)
		return f
	}
	var mp map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&mp); err != nil {
		f.status = "取不到"
		f.xian = "声明表读不到：" + csfJSONErr(err)
		return f
	}
	f.dong = fmt.Sprintf("input_fingerprint=%s · source_head=%s", csfRuneSlice16(csfPctS(mp["input_fingerprint"])), csfPctS(mp["source_head"]))
	res, errStr := csfBridge(root, "affected_map", stderr)
	if errStr != "" {
		f.status = "取不到"
		f.xian = "重算失败（" + errStr + "）⇒ 不给结论"
		return f
	}
	liveFP := res["fp"]
	liveHead := res["head"]
	f.xian = fmt.Sprintf("input_fingerprint=%s · source_head=%s", csfRuneSlice16(csfPctS(liveFP)), csfPctS(liveHead))
	if !csfPyEq(mp["input_fingerprint"], liveFP) {
		f.staleAdd("input_fingerprint", csfRuneSlice16(csfPctS(mp["input_fingerprint"])), csfRuneSlice16(csfPctS(liveFP)),
			"步表/点名件/config 任一变 ⇒ 过期 ⇒ affected-steps 报不给结论")
	}
	return f
}

// ── ③ cli-contract-baseline.json ───────────────────────────────────────
func csfProbeCliContract(root string, stderr io.Writer) *csfFace {
	rel := "scripts/gates/cli-contract-baseline.json"
	f := csfNewFace(rel, "commands（树条数相等判 T5c）· mustfail_cases（格级棘轮 T5b · 只许上调）· blanks（棘轮）", csfRefresh["cli-contract"])
	p := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(p)
	if err != nil {
		f.status = "取不到"
		f.xian = "基线读不到：" + csfOSErr(err, p)
		return f
	}
	var base map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&base); err != nil {
		f.status = "取不到"
		f.xian = "基线读不到：" + csfJSONErr(err)
		return f
	}
	res, errStr := csfBridge(root, "cli_contract", stderr)
	if errStr != "" {
		f.status = "取不到"
		f.xian = "现读重算失败（" + errStr + "）⇒ 不给结论"
		return f
	}
	nCmd, _ := csfNumInt(res["n_cmd"])
	nMf, _ := csfNumInt(res["n_mf"])
	f.dong = fmt.Sprintf("commands=%s · mustfail_cases=%s · blanks=%s", csfPctS(base["commands"]), csfPctS(base["mustfail_cases"]), csfPctS(base["blanks"]))
	f.xian = fmt.Sprintf("commands=%d · mustfail_cases=%d", nCmd, nMf)
	if !csfNumEq(base["commands"], nCmd) {
		f.staleAdd("commands（T5c 相等判）", base["commands"], nCmd, "命令树条数漂了 ⇒ 门⑪ 红")
	}
	if !csfNumEq(base["mustfail_cases"], nMf) {
		f.staleAdd("mustfail_cases（T5b 棘轮）", base["mustfail_cases"], nMf, "只许上调")
	}
	return f
}

func csfNumEq(v any, n int) bool {
	if m, ok := csfNumInt(v); ok {
		return m == n
	}
	return false
}

// ── ④ check-help-shape.py BASELINE（Go 半 chsBaseline 取源 · ✗ 不读 py 件；bin/zerg 走 exec）──
func csfProbeHelpShape(root string, stderr io.Writer) *csfFace {
	rel := "scripts/gates/check-help-shape.py"
	f := csfNewFace(rel, "BASELINE：help --all / help / help dangerous / help config 的 lines·bytes（(+)rows）", csfRefresh["help-shape"])
	res, errStr := csfBridge(root, "help_shape", stderr)
	if errStr != "" {
		f.status = "取不到"
		f.xian = "BASELINE 读不到：" + errStr
		return f
	}
	baseline, _ := res["baseline"].(map[string]any)
	zbin := filepath.Join(root, "bin", "zerg")
	if fi, err := os.Stat(zbin); err != nil || fi.IsDir() {
		f.status = "取不到"
		f.xian = fmt.Sprintf("bin/zerg 不在（%s）⇒ 取不到 help 面读数", zbin)
		return f
	}
	type fc struct {
		key  string
		argv []string
	}
	faces := []fc{
		{"help --all", []string{"help", "--all"}},
		{"help", []string{"help"}},
		{"help dangerous", []string{"help", "dangerous"}},
		{"help config", []string{"help", "config"}},
	}
	got := make([][2]int, len(faces))
	for i, it := range faces {
		cmd := exec.Command(zbin, it.argv...)
		out, err := cmd.Output()
		if err != nil {
			f.status = "取不到"
			f.xian = fmt.Sprintf("跑 bin/zerg %s 失败：%s", strings.Join(it.argv, " "), csfOSErr(err, ""))
			return f
		}
		text := strings.ToValidUTF8(string(out), "\uFFFD")
		got[i] = [2]int{len(csfSplitLines(text)), len([]byte(text))}
	}
	var dongParts, xianParts []string
	for i, it := range faces {
		bm, _ := baseline[it.key].(map[string]any)
		dongParts = append(dongParts, fmt.Sprintf("%s=%s/%sB", it.key, csfPctS(bm["lines"]), csfPctS(bm["bytes"])))
		xianParts = append(xianParts, fmt.Sprintf("%s=%d/%dB", it.key, got[i][0], got[i][1]))
	}
	f.dong = strings.Join(dongParts, " · ")
	f.xian = strings.Join(xianParts, " · ")
	for i, it := range faces {
		bm, _ := baseline[it.key].(map[string]any)
		fl, okL := csfNumInt(bm["lines"])
		fb, okB := csfNumInt(bm["bytes"])
		gl, gb := got[i][0], got[i][1]
		if !okL || !okB || fl != gl || fb != gb {
			f.staleAdd(fmt.Sprintf("%s lines/bytes", it.key),
				fmt.Sprintf("%s/%sB", csfPctS(bm["lines"]), csfPctS(bm["bytes"])),
				fmt.Sprintf("%d/%dB", gl, gb), "改命令树用法串 ⇒ 字节漂仅观察（不进 rc）；真红走闸A 逐字覆盖/闸B 无重复＋行数常量")
		}
	}
	return f
}

// ── ⑤ core/cmd/zerg/check_approve_backfill.go DEV_EDIT_SHA256（Go 真源 · 缺口 `GAP-20261012-02`）──
func csfProbeApproveBackfill(root string, stderr io.Writer) *csfFace {
	rel := "core/cmd/zerg/check_approve_backfill.go"
	f := csfNewFace(rel, "常量 DEV_EDIT_SHA256 = 在册批准件 approvals/dev_edit.json 整件 sha256", csfRefresh["approve-backfill"])
	res, errStr := csfBridge(root, "approve_backfill", stderr)
	if errStr != "" {
		f.status = "取不到"
		f.xian = "件读不到：" + errStr
		return f
	}
	frozen := csfPctS(res["sha"])
	ticketRel := csfPctS(res["rel"])
	state := os.Getenv("ZERG_STATE_DIR")
	if state == "" {
		home, _ := os.UserHomeDir()
		state = filepath.Join(home, ".zerg", "state")
	}
	ticket := filepath.Join(state, filepath.FromSlash(ticketRel))
	f.dong = fmt.Sprintf("DEV_EDIT_SHA256=%s…", csfRuneSlice16(frozen))
	if fi, err := os.Stat(ticket); err != nil || fi.IsDir() {
		f.xian = fmt.Sprintf("在册批准件不在（%s）⇒ 未取数", ticket)
		f.status = "取不到"
		return f
	}
	live, err := csfSha256File(ticket)
	if err != nil {
		f.xian = fmt.Sprintf("在册批准件不在（%s）⇒ 未取数", ticket)
		f.status = "取不到"
		return f
	}
	f.xian = fmt.Sprintf("现读件 sha=%s…", csfRuneSlice16(live))
	if !csfPyEq(live, frozen) {
		f.staleAdd("DEV_EDIT_SHA256", csfRuneSlice16(frozen)+"…", csfRuneSlice16(live)+"…", "人签换签 ⇒ 门⑰ 红 ⇒ 走 --refresh-constant")
	}
	return f
}

// ── ⑥ real-gates.sh SH_GATES 清册（纯 IO · 原生）────────────────────────
var (
	csfReSHGates = regexp.MustCompile(`(?s)SH_GATES=\((.*?)\)`)
	csfReSHTable = regexp.MustCompile(`(?s)SH_TABLE="\$\(cat <<'EOF'\n(.*?)\nEOF`)
	csfReWS      = regexp.MustCompile(`\s+`)
)

func csfProbeRealGates(root string) *csfFace {
	rel := "scripts/gates/real-gates.sh"
	f := csfNewFace(rel, "字面 SH_GATES 清册键集 ≡ SH_TABLE 键集（棘轮第 ④ 条 · 外部消费方走字面正则）", csfRefresh["real-gates"])
	p := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(p)
	if err != nil {
		f.status = "取不到"
		f.xian = "件读不到：" + csfOSErr(err, p)
		return f
	}
	src := string(raw)
	m := csfReSHGates.FindStringSubmatch(src)
	tbl := csfReSHTable.FindStringSubmatch(src)
	if m == nil || tbl == nil {
		f.status = "取不到"
		f.xian = "字面 SH_GATES / SH_TABLE 抠不出来 ⇒ 不给结论"
		return f
	}
	var gates []string
	for _, t := range csfReWS.Split(strings.ReplaceAll(m[1], "\\", " "), -1) {
		if strings.TrimSpace(t) != "" {
			gates = append(gates, t)
		}
	}
	sort.Strings(gates)
	var table []string
	for _, l := range csfSplitLines(tbl[1]) {
		if strings.TrimSpace(l) != "" {
			table = append(table, strings.SplitN(l, "\t", 2)[0])
		}
	}
	sort.Strings(table)
	f.dong = fmt.Sprintf("SH_GATES %d 键 · SH_TABLE %d 键", len(gates), len(table))
	same := csfStrSliceEq(gates, table)
	if same {
		f.xian = "两边键集逐字相等"
	} else {
		f.xian = "两边键集不等"
	}
	if !same {
		f.staleAdd("SH_GATES ⟷ SH_TABLE 键集",
			csfJoinOrDash(csfSetDiff(gates, table)), csfJoinOrDash(csfSetDiff(table, gates)),
			"只在一边 ⇒ 门65 rc=8「判不了」")
	}
	return f
}

func csfStrSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func csfSetDiff(a, b []string) []string {
	bset := map[string]bool{}
	for _, x := range b {
		bset[x] = true
	}
	out := []string{}
	for _, x := range a {
		if !bset[x] {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

func csfJoinOrDash(xs []string) string {
	if len(xs) == 0 {
		return "-"
	}
	return strings.Join(xs, ",")
}

// ── ⑦ scripts/公开标记.tsv（纯 IO · 原生）───────────────────────────────
func csfProbePublicMarks(root string) *csfFace {
	rel := "scripts/公开标记.tsv"
	f := csfNewFace(rel, "门件（scripts/gates/*.py|*.sh 直下）⇕ 表第一列（门件未登记 ⇒ 门「悬空/漏登记」）", csfRefresh["public-marks"])
	p := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(p)
	if err != nil {
		f.status = "取不到"
		f.xian = "标记表读不到：" + csfOSErr(err, p)
		return f
	}
	var rows []string
	lines := csfSplitLines(string(raw))
	for i, l := range lines {
		if i == 0 {
			continue
		}
		if strings.TrimSpace(l) != "" {
			rows = append(rows, strings.SplitN(l, "\t", 2)[0])
		}
	}
	gdir := filepath.Join(root, "scripts", "gates")
	des, err := os.ReadDir(gdir)
	if err != nil {
		f.status = "取不到"
		f.xian = "scripts/gates/ 列举失败：" + csfOSErr(err, gdir)
		return f
	}
	var ondisk []string
	for _, de := range des {
		n := de.Name()
		if !(strings.HasSuffix(n, ".py") || strings.HasSuffix(n, ".sh")) {
			continue
		}
		if fi, err := os.Stat(filepath.Join(gdir, n)); err != nil || fi.IsDir() {
			continue
		}
		ondisk = append(ondisk, n)
	}
	sort.Strings(ondisk)
	declared := map[string]bool{}
	for _, r := range rows {
		declared[r] = true
	}
	var missing []string
	for _, n := range ondisk {
		if !declared["scripts/gates/"+n] {
			missing = append(missing, n)
		}
	}
	f.dong = fmt.Sprintf("表 %d 行 · scripts/gates 直下门件 %d 件", len(rows), len(ondisk))
	f.xian = fmt.Sprintf("未登记门件 %d 件", len(missing))
	for _, n := range missing {
		f.staleAdd("scripts/gates/"+n, "（表里无此行）", "（盘上有此件）", "未登记 ⇒ 悬空/漏登记面红（首版只报此直下门件面）")
	}
	return f
}

// ── ⑧ 取数命令面（首版未代跑）───────────────────────────────────────────
func csfProbeRefCmds() []*csfFace {
	rows := [][4]string{
		{"scripts/gates/real-gates.sh / precommit-gates.sh", "步骤表自检冻结计数（全表步数 / 档位三数 / scope 步数 / 单步档全表命中）",
			"改 precommit-gates.sh 步骤表 ⇒ 该件自检当场红（半改即红）", "该件自检（`bash scripts/gates/precommit-gates.sh --list` ＋ 自检段）"},
		{"scripts/build/publish-public.sh ＋ publish/mirror-public-lib.py", "两器排除三表（EXCLUDES / DROP_PREFIX / DROP_EXACT）",
			"新增件未进三表 ⇒ release 面「镜像器仍会发它」红", "两器同步登记 + `check-publish-face-sync.py`"},
		{"core/internal/contract/dryrun-semantics.json / dev-targets.json", "门⑩ 干跑语义登记 / 门⑫ 目标闭集",
			"改这两件 ⇒ 门⑩/门⑫ 现读红", "`zerg gate check check-newfile-registry` / `check-dev-targets.py`"},
	}
	out := []*csfFace{}
	for _, r := range rows {
		f := csfNewFace(r[0], r[1], r[3])
		f.dong = "（首版未代跑）"
		f.xian = r[2]
		f.status = "取数命令"
		out = append(out, f)
	}
	return out
}

func csfCollect(root string, stderr io.Writer) []*csfFace {
	faces := []*csfFace{
		csfProbeJudgeRegistry(root), csfProbeAffectedMap(root, stderr), csfProbeCliContract(root, stderr),
		csfProbeHelpShape(root, stderr), csfProbeApproveBackfill(root, stderr), csfProbeRealGates(root),
		csfProbePublicMarks(root),
	}
	faces = append(faces, csfProbeRefCmds()...)
	return faces
}

// ── 渲染（逐字对齐 py render()）─────────────────────────────────────────
func csfRender(faces []*csfFace, out io.Writer) (int, int) {
	nStale := 0
	nFaceStale := 0
	for _, f := range faces {
		nStale += f.stale
		if f.stale > 0 {
			nFaceStale++
		}
	}
	fmt.Fprint(out, "冻结件 stale 报告（只读 · 只报告不进退码 · GAP-20261005-31 首版）\n")
	marks := map[string]string{"一致": "✓", "过期": "✗", "取不到": "···", "取数命令": "·"}
	for _, f := range faces {
		mark := marks[f.status]
		if mark == "" {
			mark = "?"
		}
		fmt.Fprint(out, "\n")
		fmt.Fprintf(out, "── %s ──  冻结形态：%s\n", f.jian, f.xtai)
		fmt.Fprintf(out, "   冻结点：%s\n", f.dong)
		fmt.Fprintf(out, "   现读  ：%s\n", f.xian)
		fmt.Fprintf(out, "   %s 状态：%s（过期 %d 处）\n", mark, f.status, f.stale)
		for _, d := range f.detail {
			fmt.Fprintf(out, "       · %s\n", d.label)
			fmt.Fprintf(out, "           冻结 %s  ⟶  现读 %s   [%s]\n", csfPctS(d.frozen), csfPctS(d.live), d.note)
		}
		fmt.Fprintf(out, "   刷新正门：%s\n", f.zheng)
	}
	fmt.Fprint(out, "\n")
	fmt.Fprintf(out, "合计：冻结面 %d 个 · 其中过期 %d 个 · 过期点 %d 处（rc=0 只报告）\n", len(faces), nFaceStale, nStale)
	return nStale, nFaceStale
}

// csfKV —— 机器面一格的键/值（值已是 JSON 字面），用于按字段表装包。
type csfKV struct{ k, v string }

func csfJSONPayload(root string, faces []*csfFace, nStale, nFaceStale int, fields string) string {
	// faces 数组现值渲染（键序与 py 逐字同）。
	var fb strings.Builder
	fb.WriteString("[")
	for i, f := range faces {
		if i > 0 {
			fb.WriteString(", ")
		}
		fb.WriteString("{")
		fb.WriteString("\"件\": " + csfPyJSONStr(f.jian))
		fb.WriteString(", \"冻结形态\": " + csfPyJSONStr(f.xtai))
		fb.WriteString(", \"状态\": " + csfPyJSONStr(f.status))
		fb.WriteString(", \"冻结点\": " + csfPyJSONStr(f.dong))
		fb.WriteString(", \"现读\": " + csfPyJSONStr(f.xian))
		fb.WriteString(", \"过期数\": " + strconv.Itoa(f.stale))
		fb.WriteString(", \"明细\": [")
		for j, d := range f.detail {
			if j > 0 {
				fb.WriteString(", ")
			}
			fb.WriteString("[" + csfPyJSONStr(d.label) + ", " + csfJSONAny(d.frozen) + ", " + csfJSONAny(d.live) + ", " + csfPyJSONStr(d.note) + "]")
		}
		fb.WriteString("]")
		fb.WriteString(", \"刷新正门\": " + csfPyJSONStr(f.zheng))
		fb.WriteString("}")
	}
	fb.WriteString("]")
	// 全量 payload（定序 = root, faces, stale_points, stale_faces, rc, mode）。
	all := []csfKV{
		{"root", csfPyJSONStr(root)},
		{"faces", fb.String()},
		{"stale_points", strconv.Itoa(nStale)},
		{"stale_faces", strconv.Itoa(nFaceStale)},
		{"rc", "0"},
		{"mode", csfPyJSONStr("report-only")},
	}
	sel := all
	if fields != "" {
		// `--json <字段>`：真收字段 —— 只出所选键（按用户给的字段序），不认的名已由
		// csfRefuseUnknownJSONFields 先拦（rc=2），此处对残余未知名跳过（防御）。
		byKey := map[string]string{}
		for _, kv := range all {
			byKey[kv.k] = kv.v
		}
		sel = nil
		for _, f := range strings.Split(fields, ",") {
			k := strings.TrimSpace(f)
			if k == "" {
				continue
			}
			if v, ok := byKey[k]; ok {
				sel = append(sel, csfKV{k, v})
			}
		}
	}
	var b strings.Builder
	b.WriteString("{")
	for i, kv := range sel {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("\"" + kv.k + "\": " + kv.v)
	}
	b.WriteString("}")
	return b.String()
}

// ── 默认根（声明 (a)）──────────────────────────────────────────────────
func csfDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	cur := wd
	for i := 0; i < 6; i++ {
		if fi, err := os.Stat(filepath.Join(cur, "scripts", "gates")); err == nil && fi.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return wd
}

// csfPyAbs 复刻 os.path.abspath 的常见档（绝对化 + clean）。
func csfPyAbs(p string) string {
	if !filepath.IsAbs(p) {
		wd, err := os.Getwd()
		if err == nil {
			p = filepath.Join(wd, p)
		}
	}
	return filepath.Clean(p)
}

// ── OS/JSON 错误正文（对齐 py 只读失败档）──────────────────────────────
var csfStrerror = map[int]string{
	2:  "No such file or directory",
	13: "Permission denied",
	17: "File exists",
	20: "Not a directory",
	21: "Is a directory",
	22: "Invalid argument",
	36: "File name too long",
	40: "Too many levels of symbolic links",
}

func csfOSErr(err error, path string) string {
	if pe, ok := err.(*os.PathError); ok {
		if en, ok := pe.Err.(syscall.Errno); ok {
			n := int(en)
			if s, ok := csfStrerror[n]; ok {
				return fmt.Sprintf("[Errno %d] %s: '%s'", n, s, path)
			}
		}
	}
	// *exec.ExitError / 其它
	if _, ok := err.(*exec.ExitError); ok {
		return "Command '" + filepath.Base(path) + "' returned non-zero exit status"
	}
	return err.Error()
}

func csfJSONErr(err error) string {
	// 尽力对齐 json 解析失败正文（真仓两件常年合法；非法 JSON 档列未覆盖面）。
	return err.Error()
}

// ── python 桥：用同一 importlib 载入同一兄弟判据件取同一读数 ─────────────
// ✗ 不在 Go 侧重写 build_map / parse_tree / load_matrix / DEV_EDIT_SHA256
//
//	—— 那会是那些件的第二条实现（双真源）。故经 python3 子进程复用。
//
// ★ `help_shape` 一路**已取源改走 Go 半**（见下方 `if op == "help_shape"`）：那不是「重写第二条
//
//	实现」—— `chsBaseline` 本就是该判据的 **Go 半**（与 py 半同一冻结真源），只是从此**不再读**
//	`scripts/gates/check-help-shape.py` 取 `BASELINE`（解该 py 件的退役读耦合）。
const csfBridgeSrc = `
import importlib.util, sys, json, os
root = sys.argv[1]
op = sys.argv[2]
def load_module(path):
    name = "_sf_" + os.path.splitext(os.path.basename(path))[0].replace("-", "_")
    spec = importlib.util.spec_from_file_location(name, path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod
def emit(obj):
    sys.stdout.write("\n\x00SF\x00" + json.dumps(obj, ensure_ascii=False) + "\n")
try:
    g = os.path.join(root, "scripts", "gates")
    if op == "affected_map":
        mod = load_module(os.path.join(g, "check-affected-map.py"))
        fresh = mod.build_map(root)
        emit({"ok": True, "fp": fresh.get("input_fingerprint"), "head": fresh.get("source_head")})
    elif op == "cli_contract":
        mod = load_module(os.path.join(g, "check-cli-contract.py"))
        tree = mod.parse_tree(root)
        matrix = mod.load_matrix(root)
        cases = matrix.get("cases", [])
        n = 0
        for c in cases:
            try:
                if int(c.get("want_rc")) != 0:
                    n += 1
            except (TypeError, ValueError):
                pass
        emit({"ok": True, "n_cmd": len(tree), "n_mf": n})
    elif op == "approve_backfill":
        mod = load_module(os.path.join(g, "check-approve-backfill.py"))
        emit({"ok": True, "sha": mod.DEV_EDIT_SHA256, "rel": mod.TICKET_REL})
    else:
        emit({"ok": False, "err": "unknown op"})
except Exception as e:
    emit({"ok": False, "err": str(e)})
`

// csfBridge ⇒ (result, "") 成功；否则 (nil, 失败正文) 作 py 异常的 str(e)。
func csfBridge(root, op string, stderr io.Writer) (map[string]any, string) {
	// ★ 退役态（G16 · 2026-10-07 退役批）：`affected_map` 这一路的兄弟件
	//   `scripts/gates/check-affected-map.py` 退役（不在盘）⇒ 现载取数这条路断（原样 =
	//   「重算失败 ⇒ 不给结论」= fail-closed）。⇒ 回落到**同判据的 Go 半** `afmBuildMap`
	//   （本包内 · 与 py 半同一真源 · ✗ 非第二条实现）现算同一读数，✗ 不因缺 py 而 fail-closed。
	//   py 件在盘时照旧走下面那条 importlib 桥（两态逐字同 · 单一真源不变）。
	if op == "affected_map" {
		sib := filepath.Join(root, "scripts", "gates", "check-affected-map.py")
		if fi, err := os.Stat(sib); err != nil || fi.IsDir() {
			mp, ok := afmBuildMap(root)
			if !ok {
				return nil, "兄弟件已退役，且 Go 半 afmBuildMap 也算不出（步表/声明面取不到）"
			}
			fp, head := "", ""
			if v, ok := mp.get("input_fingerprint"); ok {
				fp = v.pyStr()
			}
			if v, ok := mp.get("source_head"); ok {
				head = v.pyStr()
			}
			return map[string]any{"ok": true, "fp": fp, "head": head}, ""
		}
	}
	// ★ 取源 = Go 半常量（2026-10-08 · 解 `scripts/gates/check-help-shape.py` 的退役读耦合）：
	//   `help_shape` 这一路的冻结真源 = 本包内 `chsBaseline`（该判据的 **Go 半** · 与 py 半同一
	//   冻结真源 · ✗ 非第二条实现）。★ **无论该 py 件在不在盘**一律取 Go 半 —— ✗ 不再读
	//   `scripts/gates/check-help-shape.py` 取 `BASELINE`（该 py 由此对本件解除读耦合 ⇒ 可退）。
	//   ★「两半同值」这条约束**不取消**：由两侧各自判据（py S2/S4e/S5c ⟷ Go `chsJudge`）＋门113
	//   （`check-py-go-pairing-drift`）维持；本处只换取源 · ✗ 不改任何冻结读数（两半现值逐字同）。
	//   ★ 断言面一字不动：`csfProbeHelpShape` 仍逐格比 4 面 lines/bytes。★ Go 半 `chsBaseline`
	//   空 ⇒ fail-closed（不给结论 · 与在盘态同）。
	if op == "help_shape" {
		bl := map[string]any{}
		for k, v := range chsBaseline {
			m := map[string]any{}
			if v.HasLines {
				m["lines"] = v.Lines
			}
			if v.HasBytes {
				m["bytes"] = v.Bytes
			}
			bl[k] = m
		}
		if len(bl) == 0 {
			return nil, "Go 半 chsBaseline 为空（冻结读数取不到）"
		}
		return map[string]any{"ok": true, "baseline": bl}, ""
	}
	// ★ 退役态（2026-10-07 退役批 · cli_contract 桥退役容忍）：`cli_contract` 这一路的兄弟件
	//   `scripts/gates/check-cli-contract.py` 退役（不在盘）⇒ 下面那条 importlib 桥取不到其
	//   parse_tree/load_matrix（原样 = 「现读重算失败 ⇒ 不给结论」= fail-closed）。⇒ 回落**同判据的
	//   Go 半** `clicParseTree` / `clicLoadMatrix`（本包内 · 与 py 半同一真源 · ✗ 非第二条实现）现算
	//   同一读数，✗ 不因缺 py 而 fail-closed。★ 两半皆取不到（Go 半读不到 main.go 或 cli-matrix.json）
	//   ⇒ fail-closed。py 件在盘时照旧走下面那条 importlib 桥（两态逐字同 · 单一真源不变）。
	if op == "cli_contract" {
		sib := filepath.Join(root, "scripts", "gates", "check-cli-contract.py")
		if fi, err := os.Stat(sib); err != nil || fi.IsDir() {
			mainP := filepath.Join(root, "core", "cmd", "zerg", "main.go")
			if fi, err := os.Stat(mainP); err != nil || fi.IsDir() {
				return nil, "兄弟件已退役，且 Go 半 clicParseTree 取不到 main.go（命令树真源读不到）"
			}
			matrix := clicLoadMatrix(root)
			if matrix == nil {
				return nil, "兄弟件已退役，且 Go 半 clicLoadMatrix 取不到 cli-matrix.json（现读基线读不到）"
			}
			n := 0
			for _, c := range clicCasesOf(matrix) {
				if rc, ok := clicPyInt(clicMapGetOr(c, "want_rc", nil)); ok && rc != 0 {
					n++
				}
			}
			return map[string]any{"ok": true, "n_cmd": len(clicParseTree(root)), "n_mf": n}, ""
		}
	}
	// ★ 退役态（2026-10-07 退役批 · approve_backfill 桥退役容忍）：`approve_backfill` 这一路的兄弟件
	//   `scripts/gates/check-approve-backfill.py` 退役（不在盘）⇒ importlib 桥取不到其 DEV_EDIT_SHA256 /
	//   TICKET_REL（原样 = 「件读不到 ⇒ 取不到」= fail-closed）。⇒ 回落**同判据的 Go 半常量**
	//   `cafbDevEditSHA256` / `cafbTicketRel`（本包内 · 与 py 半同一真源 · ✗ 非第二条实现），
	//   ✗ 不因缺 py 而 fail-closed。★ Go 半常量也取不到（空）⇒ fail-closed。
	if op == "approve_backfill" {
		sib := filepath.Join(root, "scripts", "gates", "check-approve-backfill.py")
		if fi, err := os.Stat(sib); err != nil || fi.IsDir() {
			if cafbDevEditSHA256 == "" {
				return nil, "兄弟件已退役，且 Go 半 cafbDevEditSHA256 为空（登记常量取不到）"
			}
			return map[string]any{"ok": true, "sha": cafbDevEditSHA256, "rel": cafbTicketRel}, ""
		}
	}
	cmd := exec.Command("python3", "-c", csfBridgeSrc, root, op)
	var ob, eb bytes.Buffer
	cmd.Stdout = &ob
	cmd.Stderr = &eb
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, "python3 子进程起不来：" + err.Error()
		}
	}
	// ★ 把兄弟件在 importlib 期写出的 stderr（如 SyntaxWarning）原样转出，
	//   复刻 py 原件 importlib 现载时该警告直写进程 stderr 的行为。
	if stderr != nil && eb.Len() > 0 {
		stderr.Write(eb.Bytes())
	}
	out := ob.String()
	idx := strings.Index(out, "\x00SF\x00")
	if idx < 0 {
		msg := strings.TrimSpace(eb.String())
		if msg == "" {
			msg = strings.TrimSpace(out)
		}
		return nil, "python3 桥无返回（" + msg + "）"
	}
	line := out[idx+len("\x00SF\x00"):]
	if nl := strings.IndexByte(line, '\n'); nl >= 0 {
		line = line[:nl]
	}
	var res map[string]any
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	if err := dec.Decode(&res); err != nil {
		return nil, "python3 桥返回不可解析：" + err.Error()
	}
	if ok, _ := res["ok"].(bool); !ok {
		return nil, csfPctS(res["err"])
	}
	return res, ""
}

// ── argparse 面（usage/help/错误正文逐字对齐 py 现读）───────────────────
const csfUsageLine = "usage: check-stale-frozen.py [-h] [--root ROOT] [--json [字段]] [--self-test]"

const csfHelpText = csfUsageLine + "\n" +
	"\n" +
	"options:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --root ROOT  仓根（默认由本件位置上溯两级的仓根）\n" +
	"  --json [字段]  机器面（逗号分隔字段；缺省全量）\n" +
	"  --self-test  只跑自检（合成夹具 · 不碰真目标）\n"

type csfArgs struct {
	root     string
	jsonVal  string
	jsonSet  bool
	selfTest bool
	help     bool
}

// csfResolvePrefix 复刻 argparse 的唯一前缀缩写（长选项）。
func csfResolvePrefix(s string) string {
	if !strings.HasPrefix(s, "--") {
		return ""
	}
	known := []string{"--help", "--root", "--json", "--self-test"}
	var hits []string
	for _, k := range known {
		if strings.HasPrefix(k, s) {
			hits = append(hits, k)
		}
	}
	if len(hits) == 1 {
		return hits[0]
	}
	return ""
}

func csfParse(argv []string) (csfArgs, string) {
	a := csfArgs{root: csfDefaultRoot()}
	var rest []string
	i := 0
	for i < len(argv) {
		s := argv[i]
		name := s
		var inlineVal string
		hasInline := false
		if strings.HasPrefix(s, "--") {
			if eq := strings.Index(s, "="); eq >= 0 {
				inlineVal = s[eq+1:]
				hasInline = true
				name = s[:eq]
			}
			if r := csfResolvePrefix(name); r != "" {
				name = r
			}
		}
		switch {
		case name == "-h" || name == "--help":
			a.help = true
		case name == "--self-test":
			a.selfTest = true
		case name == "--root":
			if hasInline {
				a.root = inlineVal
			} else {
				if i+1 >= len(argv) {
					return a, "argument --root: expected one argument"
				}
				i++
				a.root = argv[i]
			}
		case name == "--json":
			a.jsonSet = true
			if hasInline {
				a.jsonVal = inlineVal
			} else if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++
				a.jsonVal = argv[i]
			} else {
				a.jsonVal = ""
			}
		default:
			rest = append(rest, s)
		}
		i++
	}
	if len(rest) > 0 {
		return a, "unrecognized arguments: " + strings.Join(rest, " ")
	}
	return a, ""
}

// csfRefuseUnknownJSONFields —— `--json <字段>` 里有不认的名 ⇒ rc=2 ＋ 印可选字段表（Q-176 口径）。
// 与同族判据件（check_registry_vs_disk / check_public_tree_selfcheck）同形：字段名打错 ≠ 零命中。
// ★ 先于 argparse 判（与 py 同序）；`--json`（无值 / 后随 `-`）⇒ 收全量，不拒。
func csfRefuseUnknownJSONFields(stderr io.Writer, argv []string) *int {
	var want []string
	for i, x := range argv {
		if x == "--json" {
			v := ""
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				v = argv[i+1]
			}
			want = append(want, strings.Split(v, ",")...)
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(strings.SplitN(x, "=", 2)[1], ",")...)
		}
	}
	var fields []string
	for _, f := range want {
		if s := strings.TrimSpace(f); s != "" {
			fields = append(fields, s)
		}
	}
	known := map[string]bool{}
	for _, f := range csfJSONFields {
		known[f] = true
	}
	var bad []string
	for _, f := range fields {
		if !known[f] {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	quoted := func(xs []string) string {
		out := make([]string, 0, len(xs))
		for _, x := range xs {
			out = append(out, "`"+x+"`")
		}
		return strings.Join(out, " · ")
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", quoted(bad))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", quoted(csfJSONFields))
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开\n")
	rc := 2
	return &rc
}

// csfRun —— 主逻辑（自检亦经此跑以捕获 stdout）。
func csfRun(argv []string, stdout, stderr io.Writer) int {
	// ① Q-176：`--json <字段>` 坏字段名先于 argparse 判（与同族判据件同序）。
	if rc := csfRefuseUnknownJSONFields(stderr, argv); rc != nil {
		return *rc
	}
	a, errMsg := csfParse(argv)
	if a.help {
		fmt.Fprint(stdout, csfHelpText)
		return 0
	}
	if errMsg != "" {
		fmt.Fprint(stderr, csfUsageLine+"\n"+csfProg+": error: "+errMsg+"\n")
		return 2
	}
	if a.selfTest {
		return csfSelfTest(stdout, stderr)
	}
	root := csfPyAbs(a.root)
	if fi, err := os.Stat(filepath.Join(root, "scripts", "gates")); err != nil || !fi.IsDir() {
		fmt.Fprintf(stderr, "⛔ 用法错（rc=2）：仓根取不到 scripts/gates/（--root=%s）\n", root)
		return 2
	}
	faces := csfCollect(root, stderr)
	nStale, nFaceStale := csfRender(faces, stdout)
	if a.jsonSet {
		fmt.Fprintln(stdout, csfJSONPayload(root, faces, nStale, nFaceStale, a.jsonVal))
	}
	return 0
}

// ── 自检（合成夹具 · 不碰真目标）───────────────────────────────────────
const csfSelftestTSVHead = "件\t面\t理由\n"

func csfSelfTestWrite(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// csfSelftestFixture 造一个仓外合成仓根（三处纯 IO 冻结点自洽）。
// frozenSHA == "" ⇒ judge-registry 的 sha256 冻成现读（一致）；否则冻成该值（可造负控）。
func csfSelftestFixture(frozenSHA string) (string, error) {
	root, err := os.MkdirTemp("", "stale-frozen-selftest-")
	if err != nil {
		return "", err
	}
	gdir := filepath.Join(root, "scripts", "gates")
	xpy := filepath.Join(gdir, "x.py")
	if err := csfSelfTestWrite(xpy, "print(2)\n"); err != nil {
		return "", err
	}
	frozen := frozenSHA
	if frozen == "" {
		frozen, err = csfSha256File(xpy)
		if err != nil {
			return "", err
		}
	}
	reg := `{"schema": "judge-registry/1", "count": 1, "dupes": [], "entries": [{"id": "x", "name": "x", "path": "scripts/gates/x.py", "cwd": ".", "interp": "python3", "sha256": "` + frozen + `", "tool_version": "python3/3.9", "cache_key": "` + frozen + `|python3/3.9"}]}`
	if err := csfSelfTestWrite(filepath.Join(gdir, "judge-registry.json"), reg); err != nil {
		return "", err
	}
	rg := "SH_GATES=(x)\n\nSH_TABLE=\"$(cat <<'EOF'\n" + "x\tg_x\nEOF\n)\"\n"
	if err := csfSelfTestWrite(filepath.Join(gdir, "real-gates.sh"), rg); err != nil {
		return "", err
	}
	tsv := csfSelftestTSVHead +
		"scripts/gates/judge-registry.json\tpublic\tx\n" +
		"scripts/gates/real-gates.sh\tpublic\tx\n" +
		"scripts/gates/x.py\tpublic\tx\n"
	if err := csfSelfTestWrite(filepath.Join(root, "scripts", "公开标记.tsv"), tsv); err != nil {
		return "", err
	}
	return root, nil
}

func csfSelfTest(stdout, stderr io.Writer) int {
	var fails []string
	expect := func(label, root, name, wantStatus string, wantStale int) {
		var f *csfFace
		for _, x := range csfCollect(root, stderr) {
			if x.jian == name {
				f = x
				break
			}
		}
		if f == nil {
			fails = append(fails, label+"：找不到面 "+name)
		} else if f.status != wantStatus || f.stale != wantStale {
			fails = append(fails, fmt.Sprintf("%s：面 %s 期望「%s · 过期 %d」实得「%s · 过期 %d」",
				label, name, wantStatus, wantStale, f.status, f.stale))
		}
	}
	capture := func(argv []string) (int, string) {
		var ob, eb bytes.Buffer
		rc := csfRun(argv, &ob, &eb)
		return rc, ob.String()
	}

	// ① 正控：三处冻结点自洽 ⇒ 各面「一致 · 过期 0」
	root, err := csfSelftestFixture("")
	if err == nil {
		expect("正控", root, "scripts/gates/judge-registry.json", "一致", 0)
		expect("正控", root, "scripts/gates/real-gates.sh", "一致", 0)
		expect("正控", root, "scripts/公开标记.tsv", "一致", 0)
		// ② 形状：人面／--json 两态
		rcH, outH := capture([]string{"--root", root})
		if rcH != 0 {
			fails = append(fails, fmt.Sprintf("形状·人面：期望 rc=0 实得 rc=%d", rcH))
		}
		if !strings.Contains(outH, "合计：冻结面") {
			fails = append(fails, "形状·人面：输出缺「合计：冻结面」行")
		}
		rcJ, outJ := capture([]string{"--root", root, "--json"})
		if rcJ != 0 {
			fails = append(fails, fmt.Sprintf("形状·--json：期望 rc=0 实得 rc=%d", rcJ))
		} else {
			var payload map[string]any
			for _, line := range csfSplitLines(outJ) {
				if strings.HasPrefix(line, "{") {
					payload = map[string]any{}
					dec := json.NewDecoder(strings.NewReader(line))
					dec.UseNumber()
					if err := dec.Decode(&payload); err != nil {
						fails = append(fails, "形状·--json：payload 解析失败（"+err.Error()+"）")
						payload = nil
						break
					}
				}
			}
			if payload == nil {
				fails = append(fails, "形状·--json：无 JSON 行")
			} else {
				mode := csfPctS(payload["mode"])
				rc := csfPctS(payload["rc"])
				_, hasFaces := payload["faces"]
				if mode != "report-only" || rc != "0" || !hasFaces {
					fails = append(fails, "形状·--json：payload 形状不对（缺 mode=report-only / rc=0 / faces）")
				}
			}
		}
		os.RemoveAll(root)
	} else {
		fails = append(fails, "正控夹具造不出："+err.Error())
	}

	// ③ 成对负控：冻错 sha256 ⇒ 报过期；写回现读 ⇒ 回绿
	if r2, err := csfSelftestFixture(strings.Repeat("0", 64)); err == nil {
		expect("负控·冻错", r2, "scripts/gates/judge-registry.json", "过期", 1)
		os.RemoveAll(r2)
	} else {
		fails = append(fails, "负控夹具造不出："+err.Error())
	}
	if r3, err := csfSelftestFixture(""); err == nil {
		expect("负控·回绿", r3, "scripts/gates/judge-registry.json", "一致", 0)
		os.RemoveAll(r3)
	} else {
		fails = append(fails, "负控夹具造不出："+err.Error())
	}

	// ④ 空转：仓根取不到 ⇒ rc=2（不静默绿）
	empty, err := os.MkdirTemp("", "stale-frozen-empty-")
	if err == nil {
		for _, bad := range []struct {
			label string
			path  string
		}{
			{"空转·根不存在", filepath.Join(empty, "no-such-root")},
			{"空转·无 scripts/gates", empty},
		} {
			rc, _ := capture([]string{"--root", bad.path})
			if rc != 2 {
				fails = append(fails, fmt.Sprintf("%s：期望 rc=2（不静默绿）实得 rc=%d", bad.label, rc))
			}
		}
		os.RemoveAll(empty)
	} else {
		fails = append(fails, "空转夹具造不出："+err.Error())
	}

	// ⑤ 字段面：`--json <字段>` 真收字段（只出所选键）＋ 不认字段名 ⇒ rc=2（Q-176）。
	captureE := func(argv []string) (int, string, string) {
		var ob, eb bytes.Buffer
		rc := csfRun(argv, &ob, &eb)
		return rc, ob.String(), eb.String()
	}
	if r5, err := csfSelftestFixture(""); err == nil {
		rcF, outF, _ := captureE([]string{"--root", r5, "--json", "root"})
		if rcF != 0 {
			fails = append(fails, fmt.Sprintf("字段面：--json root 期望 rc=0 实得 rc=%d", rcF))
		} else {
			got := ""
			for _, line := range csfSplitLines(outF) {
				if strings.HasPrefix(line, "{") {
					got = line
				}
			}
			if !strings.Contains(got, "\"root\":") || strings.Contains(got, "\"faces\":") {
				fails = append(fails, "字段面：--json root 该只出 root 键（真收字段）实得 "+got)
			}
		}
		rcB, _, errB := captureE([]string{"--root", r5, "--json", "nope,root"})
		if rcB != 2 {
			fails = append(fails, fmt.Sprintf("字段面：--json nope,root 期望 rc=2 实得 rc=%d", rcB))
		}
		if !strings.Contains(errB, "不认的字段名") {
			fails = append(fails, "字段面：--json nope stderr 缺「不认的字段名」")
		}
		os.RemoveAll(r5)
	} else {
		fails = append(fails, "字段面夹具造不出："+err.Error())
	}

	if len(fails) > 0 {
		fmt.Fprintf(stdout, "✗ check-stale-frozen 自检不过（%d 条）：\n", len(fails))
		for _, f := range fails {
			fmt.Fprint(stdout, "  - "+f+"\n")
		}
		return 2
	}
	fmt.Fprint(stdout, "✓ check-stale-frozen 自检全过（正控 3 面 · 负控成对 · 空转 2 格 · 形状 2 态 · 字段 2 格）\n")
	return 0
}

// ── 入口 ───────────────────────────────────────────────────────────────
func csfGateMain(stdout, stderr io.Writer, argv []string) int {
	return csfRun(argv, stdout, stderr)
}
