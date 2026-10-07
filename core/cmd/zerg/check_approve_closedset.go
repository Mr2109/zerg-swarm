// check_approve_closedset.go —— 门禁件 `scripts/gates/check-approve-closedset.py` 的 **Go 真搬**（M2 · 甲案 C 类 · 真·待搬清单第十七枚）。
//
// 为什么有这件：py 真身 `scripts/gates/check-approve-closedset.py`（407 行 · sha16 38b3feacf5a92416）。
// 口径：档位闭集与契约槽的**只登记**对拍件（2.5.11 人签 F 批 · F4）—— 判 S1 闭集没被顺手扩 · S2 单向铁律
// 登记在位 · S3/S4 三个预留旗标名没被占 · S5 公开面零改。本件**只判、只登记**：一个字节都不写。
//
// ── 出厂声明 (a) · 默认根定位差异 ────────────────────────────────────────────────
//
//	py 从 `os.path.dirname(os.path.abspath(__file__))` 上溯两级得 `DEFAULT_ROOT`（= 仓根）；
//	Go 无 `__file__` 语境 ⇒ 用同族件 `check_approve_ci.go` 的 `caciFindRepoRoot()`（cwd 上溯至最近的
//	含 `scripts/gates/` 的目录，找不到则回落 cwd）。常见档（cwd = 仓根）二者同值；
//	「脚本位置 ≠ cwd 上溯根」时**不声称逐字对齐**。
//	prog 名：py argparse 取 `basename(sys.argv[0])`（本件规范名 = `check-approve-closedset.py`）；
//	Go 件内置同名字面量 `cacsProg`（✗ 不用 os.Args[0] —— 那会让探针名进 usage/错误行）。
//
// ── 出厂声明 (b) · 自指检查 ─────────────────────────────────────────────────────
//
//	判「否」：本件**不落在自己的扫描面内** —— 本门只读仓内 `core/internal/contract/registry.json`、
//	仓外开发文档面的一份 md、以及现跑 `bin/zerg approve keygen` 与那份同步门；三者都不含 `core/cmd/zerg/`。
//	件内零绝对路径字面量（无 `/Volumes/...` / `/Users/...` 一类）、无 self-test 魔法串连续落地；
//	★ 他门自指第四形态已逐条消解：`check-error-kinds` 的 R3 用法面正则吃两种**连续引号原串**
//	（一是 `setErr` 的调用形、二是 JSON 键名后跟冒号再跟引号串）⇒ 本件**两种都不写**：
//	本门本就无 error.kind 面，且件内键名一律走「前缀 + 字面」拼串、✗ 不落可被扫的连续原串。
//	现读复核：对本件与测试件跑 R3 正则，命中 **0/0**（见收尾读数）。
//	`check-hardcoded-private-paths` 扫 `core` 树，其 NEEDLES 三条本件零命中；
//	`check-wall-layer` 的 `layer(Host|Node|Space)\s*=\s*"…"` / `layer:\s*"…"` 本件零命中。
//
// ★ A 类真口子（**逐字保留**，✗ 不改 py 原件，交回入账）：py 的自检用
//
//	`tempfile.mkdtemp(prefix="closedset-")` **从不回收** ⇒ 每跑一次在系统临时目录留一个目录。
//	Go 侧自带 `defer os.RemoveAll(tmp)` 止血，测试件钉 `delta=0` 子用例 + 窄牙负控（删 defer ⇒ FAIL）。
//
// ★ C 类：py 最外层裸 `sys.exit(main())` ⇒ 未捕异常 = 回溯 rc=1（不是「不给结论 rc=2」）；
//
//	Go 侧遇「契约读不了 / 登记表读不了」一律 fail-closed，逐条写进未覆盖面。
//
// ★ 运行档不可复现面：自检 ⑬/⑭ 走真子进程，其 rc 与 stdout 已定；`mkdtemp` 的随机后缀不进 stdout。
//
// ★ 已声明的口径差（单列 · 见未覆盖面）：① 「契约不是合法 JSON（%s）」里的 %s 是 py 异常正文，
//
//	Go 侧换成 encoding/json 的报错正文；② 「（跑不起来：%s）」同理。两处都只在该档出现。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ── 常量（逐字取自 py 原件）─────────────────────────────────────────────────────

const cacsProg = "check-approve-closedset.py"
const cacsVersion = "check-approve-closedset.py v1.0.0"

// cacsPyExe：py 侧用 `sys.executable` 跑那份同步门；本机钉 `/usr/bin/python3`（GAP-20261001-61）。
const cacsPyExe = "/usr/bin/python3"

var cacsRegistryRel = caciPyJoin("core", "internal", "contract", "registry.json")
var cacsDevdocsTail = caciPyJoin("项目文档", "v2.5.11")

const cacsLedgerFile = "登记表-档位闭集与契约槽-20260923.md"

var cacsSyncGateRel = caciPyJoin("scripts", "gates", "check-publish-face-sync.py")

// cacsSyncGoHalfRel —— 同步门（check-publish-face-sync）的 Go 真搬件（相对仓根）。
// 退役容忍判据用（缺口 GAP-20261007-16）：该 py 退役后 S5 由 Go 半承载 ⇒ 不过问，✗ 不 fail-closed。
var cacsSyncGoHalfRel = caciPyJoin("core", "cmd", "zerg", "check_publish_face_sync.go")

// cacsSyncRetired —— 同步门 py 不在盘（已退役）且其 Go 真搬件在盘 ⇒ 退役态（S5 不过问）。
// 两缀分离：仅「py 不在盘」不够（真·缺件也长这样）⇒ 必须 Go 半在盘才算退役。
func cacsSyncRetired(root string) bool {
	return !caciExists(caciPyJoin(root, cacsSyncGateRel)) && caciExists(caciPyJoin(root, cacsSyncGoHalfRel))
}

// cacsTopKeys：批前现读（5 键 · 按序）—— ★ 序也是判据的一部分（`list(d.keys())` == 本表）。
var cacsTopKeys = []string{"schema", "note", "authority", "entries", "change_flow"}

const cacsEntriesN = 15 // 批前现读（2026-09-23 波① T1b：13→14；T3：14→15）
const cacsSigAlgN = 0   // ★ 必须保持 0
const cacsApproveN = 1  // 批前现读

// cacsI4Verbatim：`v1.3 §⑥ I4` 逐字（登记表必须带这一句 —— 一个标点都不许改）。
const cacsI4Verbatim = "只许加、不许减：一旦有件按新档签过，删掉这一档的代码 = 那些件永远验不了"

var cacsLedgerMust = []string{cacsI4Verbatim, "未落地", "名字还没被占"}

// cacsReservedFlags：预留旗标名（`P27` · 一个都不许占）。★ 表序 = py 元组序 ⇒ 输出行序决定者之一。
var cacsReservedFlags = []string{"--strength", "--token", "--rekey"}

// cacsJSONFields：本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）。本门**没有**机器读面。
var cacsJSONFields = []string{}

const cacsUsage = "usage: check-approve-closedset.py [-h] [--root ROOT] [--state STATE]\n" +
	"                                  [--devdocs DEVDOCS] [--registry REGISTRY]\n" +
	"                                  [--static-only] [--timeout TIMEOUT]\n" +
	"                                  [--self-test] [--no-self-test]\n"

const cacsHelp = cacsUsage + "\n" +
	"check-approve-closedset.py v1.0.0\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help           show this help message and exit\n" +
	"  --root ROOT          仓根（默认 = 本脚本的上两级）\n" +
	"  --state STATE        状态目录（默认 = `ZERG_STATE_DIR` > `~/.zerg/state`）\n" +
	"  --devdocs DEVDOCS    开发文档面根（默认 = `ZERG_DEVDOCS_ROOT` > 仓同级 `Zerg-\n" +
	"                       docs/项目文档/v2.5.11`）\n" +
	"  --registry REGISTRY  契约路径（默认 = 仓内\n" +
	"                       `core/internal/contract/registry.json`；自检夹具用）\n" +
	"  --static-only        只判静态两格（自检内部用 · 不跑命令面）\n" +
	"  --timeout TIMEOUT    每一格的时限（秒 · 默认 60）\n" +
	"  --self-test          只跑自检（不碰真目标）\n" +
	"  --no-self-test       跳过自检（只有已自证过的调用方才该用）\n"

var cacsLongOpts = []string{"--help", "--root", "--state", "--devdocs", "--registry",
	"--static-only", "--timeout", "--self-test", "--no-self-test"}

// ── 有序 JSON 对象（★ py dict 保插入序 ⇒ `list(d.keys())` 是文档序；Go map 迭代随机 ⇒ 自建保序面）──

type cacsObj struct {
	keys []string
	vals map[string]json.RawMessage
}

// cacsParseObj：按**文档序**取顶层/嵌套对象的键，值留 `RawMessage`（保原文序）。
func cacsParseObj(raw []byte) (cacsObj, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return cacsObj{}, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return cacsObj{}, false
	}
	o := cacsObj{vals: map[string]json.RawMessage{}}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return cacsObj{}, false
		}
		k, ok := kt.(string)
		if !ok {
			return cacsObj{}, false
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return cacsObj{}, false
		}
		if _, seen := o.vals[k]; !seen {
			o.keys = append(o.keys, k)
		}
		o.vals[k] = v
	}
	if _, err := dec.Token(); err != nil {
		return cacsObj{}, false
	}
	return o, true
}

// cacsDumps：py `json.dumps(..., ensure_ascii=False)` 的**键序保真**等价面（分隔符 `, ` / `: `）。
func cacsDumps(o cacsObj) string {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		kb, _ := json.Marshal(k)
		sb.Write(kb)
		sb.WriteString(": ")
		sb.Write(o.vals[k])
	}
	sb.WriteByte('}')
	return sb.String()
}

func cacsRawJSON(s string) json.RawMessage { return json.RawMessage(s) }

// ── 判定口（纯函数：自检与真目标**共用同一套**）──────────────────────────────────

// cacsReadRegistry：契约解析（保序键 + entries 面）。why == "" 表示成功。
func cacsReadRegistry(raw []byte) (cacsObj, bool, int, string) {
	var anyv any
	if err := json.Unmarshal(raw, &anyv); err != nil {
		return cacsObj{}, false, 0, fmt.Sprintf("契约不是合法 JSON（%s）", err.Error())
	}
	if _, ok := anyv.(map[string]any); !ok {
		return cacsObj{}, false, 0, "契约不是 JSON 对象"
	}
	o, ok := cacsParseObj(raw)
	if !ok {
		return cacsObj{}, false, 0, "契约不是 JSON 对象"
	}
	isList := false
	n := 0
	if v, has := o.vals["entries"]; has {
		var arr []json.RawMessage
		if err := json.Unmarshal(v, &arr); err == nil {
			isList = true
			n = len(arr)
		}
	}
	return o, isList, n, ""
}

// cacsJudgeClosedset：S1 —— 四格读数 == 批前登记值。
func cacsJudgeClosedset(keys []string, entriesIsList bool, entriesLen int, text string) []string {
	problems := []string{}
	if !cacsStrSliceEq(keys, cacsTopKeys) {
		problems = append(problems, fmt.Sprintf("S1 破：顶层键 = %s ≠ 批前 5 键 %s（顺序也照批前那一份）",
			strings.Join(keys, ","), strings.Join(cacsTopKeys, ",")))
	}
	if !entriesIsList || entriesLen != cacsEntriesN {
		entStr := "（不是数组）"
		if entriesIsList {
			entStr = strconv.Itoa(entriesLen)
		}
		problems = append(problems, fmt.Sprintf("S1 破：`entries` = %s 条 ≠ 批前 %d 条（闭集被顺手扩/缩）",
			entStr, cacsEntriesN))
	}
	nSig := strings.Count(text, "sig_alg")
	if nSig != cacsSigAlgN {
		problems = append(problems, fmt.Sprintf("S1 破：含 `sig_alg` **%d** 次 ≠ **%d**（`I-7`：那是**新增条目**，"+
			"先拍再改 ⇒ 顺手加进来就是越线）", nSig, cacsSigAlgN))
	}
	nApp := strings.Count(text, "approve")
	if nApp != cacsApproveN {
		problems = append(problems, fmt.Sprintf("S1 破：含 `approve` %d 次 ≠ 批前 %d 次", nApp, cacsApproveN))
	}
	return problems
}

// cacsJudgeLedger：S2 —— 登记表必须逐字带 I4 那一句 + 两个状态词。
func cacsJudgeLedger(text string) []string {
	miss := []string{}
	for _, s := range cacsLedgerMust {
		if !strings.Contains(text, s) {
			miss = append(miss, s)
		}
	}
	if len(miss) == 0 {
		return []string{}
	}
	quoted := make([]string, 0, len(miss))
	for _, m := range miss {
		quoted = append(quoted, fmt.Sprintf("「%s」", m))
	}
	return []string{fmt.Sprintf("S2 破：登记表缺 %s（缺件/缺句 ⇒ 不就是**登记**；`R10`：不许把预留接口读成「已经升过了」）",
		strings.Join(quoted, "、"))}
}

// cacsJudgeFlagNotTaken：S3/S4 —— 预留旗标名还没被占 ⇒ rc=2 且 stdout 0 字节。
func cacsJudgeFlagNotTaken(name string, rc int, rcNone bool, outLen int) []string {
	problems := []string{}
	s := "4"
	if name == "--strength" {
		s = "3"
	}
	if rcNone || rc != 2 {
		problems = append(problems, fmt.Sprintf("S%s 破：`%s` ⇒ rc=%s ≠ 2（名字**已经被占了**？占名 = 把接口面形态先定死 ⇒ "+
			"随接口一起拍，不许顺手占）", s, name, cacsRcStr(rc, rcNone)))
	}
	if outLen != 0 {
		problems = append(problems, fmt.Sprintf("S%s 破：`%s` ⇒ stdout %d 字节 ≠ 0", s, name, outLen))
	}
	return problems
}

// cacsJudgeSync：S5 —— 公开面零改 = 那份同步门 rc=0。
func cacsJudgeSync(rc int, rcNone bool, tail string) []string {
	if !rcNone && rc == 0 {
		return []string{}
	}
	return []string{fmt.Sprintf("S5 破：`%s` rc=%s ≠ 0（公开面**有漂移**）—— stderr 末行：%s",
		cacsSyncGateRel, cacsRcStr(rc, rcNone), tail)}
}

// ── 取数（只读）───────────────────────────────────────────────────────────────────

func cacsStrSliceEq(a, b []string) bool {
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

func cacsRcStr(rc int, rcNone bool) string {
	if rcNone {
		return "None"
	}
	return strconv.Itoa(rc)
}

func cacsExpandUser(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home := os.Getenv("HOME")
		if home == "" {
			if h, err := os.UserHomeDir(); err == nil {
				home = h
			}
		}
		if home != "" {
			return caciPyJoin(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func cacsStateDirOf(given string, givenSet bool) string {
	if givenSet {
		return caciPyAbspath(given)
	}
	if env := os.Getenv("ZERG_STATE_DIR"); env != "" {
		return caciPyAbspath(env)
	}
	return caciPyJoin(cacsExpandUser("~"), ".zerg", "state")
}

func cacsDevdocsRootOf(root, given string, givenSet bool) string {
	if givenSet {
		return caciPyAbspath(given)
	}
	if env := os.Getenv("ZERG_DEVDOCS_ROOT"); env != "" {
		return caciPyAbspath(env)
	}
	// 同级根归一（GAP-20260927-82 批1）：ZERG_DOCS_ALT（同 statepath.DocsAltRoot）/ ZERG_DOCS（同 scriptInvDocsRoot）
	sibling := os.Getenv("ZERG_DOCS_ALT")
	if sibling == "" {
		sibling = os.Getenv("ZERG_DOCS")
	}
	if sibling == "" {
		sibling = caciPyJoin(caciPyDirname(root), "Zerg-内部文档")
	}
	return caciPyJoin(sibling, cacsDevdocsTail)
}

// cacsRunCmd：跑一条命令面（stdin=DEVNULL · 追加 ZERG_STATE_DIR）⇒ (rc, rcNone, stdout, stderr)。
func cacsRunCmd(argv []string, state string, timeoutS int) (int, bool, []byte, string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutS)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = caciSetEnv(os.Environ(), "ZERG_STATE_DIR", state)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return 0, true, nil, "（超时被强杀）"
	}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), false, outBuf.Bytes(), caciDecodeReplace(errBuf.Bytes())
		}
		return 0, true, nil, fmt.Sprintf("（跑不起来：%s）", err.Error())
	}
	return 0, false, outBuf.Bytes(), caciDecodeReplace(errBuf.Bytes())
}

// ── 自检（先自证会红，再扫真目标）────────────────────────────────────────────────

func cacsSelfTest(stdout io.Writer, root, devdocs string) bool {
	ok, bad := 0, 0
	check := func(cond bool, msgOK, msgBad string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", msgOK)
			ok++
		} else {
			fmt.Fprintf(stdout, "   ✗ %s\n", msgBad)
			bad++
		}
	}

	regPath := caciPyJoin(root, cacsRegistryRel)
	if !caciExists(regPath) {
		fmt.Fprintf(stdout, "   ✗ 自检前置：契约不在（%s）⇒ 自检跑不起来\n", regPath)
		return false
	}
	goodRaw, err := os.ReadFile(regPath)
	if err != nil {
		fmt.Fprintf(stdout, "   ✗ 自检前置：契约读不了（%s）⇒ 自检跑不起来\n", regPath)
		return false
	}
	goodTxt := string(goodRaw)

	o0, isList0, n0, why := cacsReadRegistry(goodRaw)
	check(why == "" && len(cacsJudgeClosedset(o0.keys, isList0, n0, goodTxt)) == 0,
		"① 正控：真契约四格 == 批前登记值 ⇒ 不判红（S1）", "① 正控失败：真契约被判红（"+why+"）")

	// ② 负控：造一条含 `sig_alg` 的条目 ⇒ 判红
	d2 := cacsCloneObj(o0)
	ents2 := cacsRawArray(o0.vals["entries"])
	ents2 = append(ents2, cacsRawJSON("{\"id\": \"sigalg.probe\", \"note\": \"signature algorithm closed set\", \"sig_alg\": \"ed25519\"}"))
	d2.vals["entries"] = cacsArrayRaw(ents2)
	p2 := cacsJudgeClosedset(d2.keys, true, len(ents2), cacsDumps(d2))
	check(len(p2) > 0 && cacsAnyContains(p2, "sig_alg"),
		"② **负控（`F4` 原文点名的那一格）**：造一条含 `sig_alg` 的条目 ⇒ **判红**",
		"② 负控失败：含 `sig_alg` 还判绿 —— 那就是「本批没人偷偷新增」这句被判假")

	// ②b 成对负控：条数不变、只把 `sig_alg` 悄悄塞进正文 ⇒ 仍判红（且**只此一条**）
	d2b := cacsCloneObj(o0)
	ents2b := cacsRawArray(o0.vals["entries"])
	mod2b := make([]json.RawMessage, 0, len(ents2b))
	for _, e := range ents2b {
		eo, okE := cacsParseObj(e)
		if !okE {
			mod2b = append(mod2b, e)
			continue
		}
		note := ""
		if v, has := eo.vals["note"]; has {
			var s string
			if json.Unmarshal(v, &s) == nil {
				note = s
			}
		}
		nb, _ := json.Marshal(note + " sig_alg")
		cacsObjSet(&eo, "note", json.RawMessage(nb))
		mod2b = append(mod2b, cacsRawJSON(cacsDumps(eo)))
	}
	d2b.vals["entries"] = cacsArrayRaw(mod2b)
	p2b := cacsJudgeClosedset(d2b.keys, true, len(mod2b), cacsDumps(d2b))
	check(len(p2b) > 0 && cacsAnyContains(p2b, "sig_alg") && len(p2b) == 1,
		"②b 成对负控：**条数不变**、只把 `sig_alg` 悄悄塞进正文 ⇒ **仍判红**（那一格自己有牙）",
		"②b 失败：条数不变就判绿（= 那一格只靠条数，`sig_alg` 这格是虚的）")

	// ③ 负控：`entries` 少一条（15 → 14）⇒ 判红
	d3 := cacsCloneObj(o0)
	e3 := cacsRawArray(o0.vals["entries"])
	if len(e3) > cacsEntriesN-1 {
		e3 = e3[:cacsEntriesN-1]
	}
	d3.vals["entries"] = cacsArrayRaw(e3)
	check(len(cacsJudgeClosedset(d3.keys, true, len(e3), cacsDumps(d3))) > 0,
		"③ 负控：`entries` 少一条（13 → 12）⇒ 判红", "③ 负控失败：条数变了还判绿")

	// ④ 负控：顶层少一键（5 → 4）⇒ 判红
	d4 := cacsCloneObj(o0)
	cacsObjDel(&d4, "change_flow")
	ev, eok := d4.vals["entries"]
	dn, dis := 0, false
	if eok {
		var arr []json.RawMessage
		if json.Unmarshal(ev, &arr) == nil {
			dis, dn = true, len(arr)
		}
	}
	check(len(cacsJudgeClosedset(d4.keys, dis, dn, cacsDumps(d4))) > 0,
		"④ 负控：顶层少一键（5 → 4）⇒ 判红", "④ 负控失败：键少了还判绿")

	ledGood := "## 单向铁律\n" + cacsI4Verbatim + "\n状态：**未落地** · 预留旗标名**名字还没被占**\n"
	check(len(cacsJudgeLedger(ledGood)) == 0, "⑤ 正控：登记表逐字带 I4 + 两个状态词 ⇒ 不判红（S2）",
		"⑤ 正控失败：齐了还判红")
	check(len(cacsJudgeLedger(strings.ReplaceAll(ledGood, "未落地", "已落地"))) > 0,
		"⑥ 负控：把「未落地」写成「已落地」⇒ 判红（`R10`：不许把预留读成已实现）",
		"⑥ 负控失败：状态词被改成「已落地」还判绿")
	check(len(cacsJudgeLedger(strings.ReplaceAll(ledGood, cacsI4Verbatim, ""))) > 0,
		"⑦ 负控：抹掉 `I4` 逐字句 ⇒ 判红", "⑦ 负控失败：铁律句被抹掉还判绿")

	check(len(cacsJudgeFlagNotTaken("--strength", 2, false, 0)) == 0,
		"⑧ 正控：`--strength` ⇒ rc=2 且 stdout 0 字节 ⇒ 不判红（`M-20`）", "⑧ 正控失败：期望形态被判红")
	check(len(cacsJudgeFlagNotTaken("--strength", 0, false, 0)) > 0,
		"⑨ 负控：`--strength` rc=0 ⇒ 判红（名字被占了）", "⑨ 负控失败：被占了还判绿")
	check(len(cacsJudgeFlagNotTaken("--rekey", 2, false, 5)) > 0,
		"⑩ 负控：`--rekey` stdout 非空 ⇒ 判红", "⑩ 负控失败：stdout 非空还判绿")

	check(len(cacsJudgeSync(0, false, "")) == 0, "⑪ 正控：同步门 rc=0 ⇒ 不判红（S5）", "⑪ 正控失败：rc=0 还判红")
	check(len(cacsJudgeSync(1, false, "漂移 3 条")) > 0, "⑫ 负控：同步门 rc=1 ⇒ 判红（公开面有漂移）",
		"⑫ 负控失败：rc=1 还判绿")

	// ⑬/⑭ **真命令行 + 真退出码** 的成对负控（判据口在真目标上有没有牙）
	// ★ A 类止血：py 这里 mkdtemp 从不回收 ⇒ Go 侧 defer RemoveAll（输出面不可见差异）。
	tmp, err := os.MkdirTemp("", "closedset-")
	if err != nil {
		fmt.Fprintf(stdout, "   ✗ 自检前置：夹具建不起来（%s）⇒ 自检跑不起来\n", err.Error())
		return false
	}
	defer os.RemoveAll(tmp)

	ledp := caciPyJoin(tmp, cacsLedgerFile)
	if err := os.WriteFile(ledp, []byte(ledGood), 0o644); err != nil {
		fmt.Fprintf(stdout, "   ✗ 自检前置：登记表夹具写不了（%s）⇒ 自检跑不起来\n", err.Error())
		return false
	}
	fxBad := caciPyJoin(tmp, "reg_sigalg.json")
	d5 := cacsCloneObj(o0)
	e5 := cacsRawArray(o0.vals["entries"])
	e5 = append(e5, cacsRawJSON("{\"id\": \"sigalg.probe\", \"sig_alg\": \"ed25519\"}"))
	d5.vals["entries"] = cacsArrayRaw(e5)
	if err := os.WriteFile(fxBad, []byte(cacsDumps(d5)), 0o644); err != nil {
		fmt.Fprintf(stdout, "   ✗ 自检前置：契约夹具写不了（%s）⇒ 自检跑不起来\n", err.Error())
		return false
	}
	fxGood := caciPyJoin(tmp, "reg_good.json")
	if err := os.WriteFile(fxGood, goodRaw, 0o644); err != nil {
		fmt.Fprintf(stdout, "   ✗ 自检前置：契约夹具写不了（%s）⇒ 自检跑不起来\n", err.Error())
		return false
	}

	rcBad, outBad := cacsSpawnSelf([]string{"--no-self-test", "--static-only", "--registry", fxBad, "--devdocs", tmp})
	check(rcBad == 1 && strings.Contains(outBad, "S1 破"),
		"⑬ **真命令行负控**：夹具契约含 `sig_alg` ⇒ 本件 **rc=1** 且点名 `S1 破`",
		fmt.Sprintf("⑬ 真命令行负控失败：rc=%s（该 1）· 输出里有没有 S1 破 = %s", cacsRcStr(rcBad, false), caciBool(strings.Contains(outBad, "S1 破"))))
	rcGood, outGood := cacsSpawnSelf([]string{"--no-self-test", "--static-only", "--registry", fxGood, "--devdocs", tmp})
	check(rcGood == 0 && strings.Contains(outGood, "全绿"),
		"⑭ **真命令行正控**：夹具契约 == 批前 ⇒ 本件 **rc=0**",
		fmt.Sprintf("⑭ 真命令行正控失败：rc=%s（该 0）", cacsRcStr(rcGood, false)))

	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节 · 未新增任何条目）\n", ok, bad)
	return bad == 0
}

// cacsSelfExe —— 自检「真命令行」子进程用的自身可执行件（py 走 `sys.executable`）；测试件注入仓外探针壳。
var cacsSelfExe = func() (string, error) { return os.Executable() }

// cacsSpawnSelf：以自身可执行体再跑一次本门（等价 py 的 `sys.executable + __file__` 再 spawn）。
//
// ★ 必须**带 inproc 路由前缀**（`gate check <键>`）：`bin/zerg` 的 argv 首段是子命令，直接把本门旗标
// 喂给制品会被当成未知命令 ⇒ 子进程恒退 2（自检 ⑬/⑭ 两格假红）。py 无这一层（它是 `python3 <件> <argv>`）。
func cacsSpawnSelf(args []string) (int, string) {
	exe, err := cacsSelfExe()
	if err != nil || exe == "" {
		return exitBlocked, "" // ★ 子进程**起不来**（非 ExitError）⇒ 读不到 ⇒ 归 `exitBlocked`(8「有 BLOCKED · 读不到不许当健康」)；
		// 原先裸写 127（shell 惯例）不在命令面真源表里 ⇒ 门⑪ T3 报（GAP-20261002-08）。
	}
	argv := append([]string{"gate", "check", "check-approve-closedset"}, args...)
	cmd := exec.Command(exe, argv...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	rc := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			rc = ee.ExitCode()
		} else {
			rc = 127
		}
	}
	return rc, caciDecodeReplace(outBuf.Bytes())
}

func cacsCloneObj(o cacsObj) cacsObj {
	n := cacsObj{keys: append([]string{}, o.keys...), vals: map[string]json.RawMessage{}}
	for k, v := range o.vals {
		n.vals[k] = v
	}
	return n
}

func cacsObjDel(o *cacsObj, k string) {
	delete(o.vals, k)
	out := o.keys[:0]
	for _, kk := range o.keys {
		if kk != k {
			out = append(out, kk)
		}
	}
	o.keys = out
}

// cacsObjSet：py `dict(e, note=...)` 的同构面 —— **键不在场时追加到末尾**（保序）。
func cacsObjSet(o *cacsObj, k string, v json.RawMessage) {
	if _, has := o.vals[k]; !has {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func cacsRawArray(v json.RawMessage) []json.RawMessage {
	arr := []json.RawMessage{}
	if len(v) > 0 {
		_ = json.Unmarshal(v, &arr)
	}
	return arr
}

func cacsArrayRaw(a []json.RawMessage) json.RawMessage {
	if a == nil {
		a = []json.RawMessage{}
	}
	b, _ := json.Marshal(a)
	return json.RawMessage(b)
}

func cacsAnyContains(ss []string, needle string) bool {
	for _, s := range ss {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// ── 主 ───────────────────────────────────────────────────────────────────────────

func cacsScan(stdout io.Writer, root, state, devdocs, registry string, staticOnly bool, timeout int) int {
	reds, nocalls := []string{}, []string{}
	regPath := registry
	if regPath == "" {
		regPath = caciPyJoin(root, cacsRegistryRel)
	}
	if !caciExists(regPath) {
		fmt.Fprintf(stdout, "⛔ 未取数：契约不在（%s）⇒ **不出结论**（rc=2）\n", regPath)
		return 2
	}
	raw, err := os.ReadFile(regPath)
	if err != nil {
		fmt.Fprintf(stdout, "⛔ 未取数：契约读不了（%s）⇒ **不出结论**（rc=2）\n", regPath)
		return 2
	}
	o, isList, nEnt, why := cacsReadRegistry(raw)
	if why != "" {
		fmt.Fprintf(stdout, "⛔ 未取数：%s ⇒ **不出结论**（rc=2）\n", why)
		return 2
	}
	txt := string(raw)
	fmt.Fprintf(stdout, "── S1 闭集对拍（%s）\n", regPath)
	fmt.Fprintf(stdout, "   · 顶层键 %d 个 = %s（批前 %s）\n", len(o.keys), strings.Join(o.keys, ","), strings.Join(cacsTopKeys, ","))
	entStr := "（不是数组）"
	if isList {
		entStr = strconv.Itoa(nEnt)
	}
	fmt.Fprintf(stdout, "   · `entries` %s 条（批前 %d） · 含 `sig_alg` %d 次（★必须保持 %d） · 含 `approve` %d 次（批前 %d）\n",
		entStr, cacsEntriesN, strings.Count(txt, "sig_alg"), cacsSigAlgN, strings.Count(txt, "approve"), cacsApproveN)
	sum := sha256.Sum256(raw)
	fmt.Fprintf(stdout, "   · 整件 sha256 = %s（**只印不判**：无关的合法改动不该变假红）\n", hex.EncodeToString(sum[:]))
	reds = append(reds, cacsJudgeClosedset(o.keys, isList, nEnt, txt)...)

	fmt.Fprintf(stdout, "── S2 单向铁律登记（仓外开发文档面 = %s）\n", devdocs)
	led := caciPyJoin(devdocs, cacsLedgerFile)
	if !caciIsDir(devdocs) || !caciExists(led) {
		nocalls = append(nocalls, fmt.Sprintf("S2 **未取数**：登记表不在（%s）⇒ 不给结论（rc=2）—— 缺件 ≠ 绿", led))
	} else {
		lt, err := caciPyReadText(led)
		if err != nil {
			nocalls = append(nocalls, fmt.Sprintf("S2 **未取数**：登记表读不了（%s）⇒ 不给结论（rc=2）—— 缺件 ≠ 绿", led))
		} else {
			fmt.Fprintf(stdout, "   · 登记表 %s（%d 字节）\n", cacsLedgerFile, len([]byte(lt)))
			reds = append(reds, cacsJudgeLedger(lt)...)
		}
	}

	if staticOnly {
		fmt.Fprint(stdout, "   · `--static-only`：本跑只判静态两格（自检内部用）\n")
	} else {
		zerg := caciPyJoin(root, "bin", "zerg")
		fmt.Fprintf(stdout, "── S3/S4 预留旗标名还没被占（`P27` · 沙箱状态目录 %s）\n", state)
		if !caciExists(zerg) {
			nocalls = append(nocalls, fmt.Sprintf("S3/S4 **未取数**：`bin/zerg` 不在（%s）⇒ 不给结论（rc=2）—— "+
				"先走自举档 `zerg build all --only cli`", zerg))
		} else {
			for _, flag := range cacsReservedFlags {
				rc, rcNone, out, errS := cacsRunCmd([]string{zerg, "approve", "keygen", flag, "token", "--by", "<名>"}, state, timeout)
				fmt.Fprintf(stdout, "   · `zerg approve keygen %s token` ⇒ rc=%s · stdout %d 字节 · stderr 首行 = %s\n",
					flag, cacsRcStr(rc, rcNone), len(out), caciFirstLine(errS))
				reds = append(reds, cacsJudgeFlagNotTaken(flag, rc, rcNone, len(out))...)
			}
		}

		fmt.Fprint(stdout, "── S5 公开面零改（`F4` 判据④ 点名的那条命令）\n")
		sync := caciPyJoin(root, cacsSyncGateRel)
		if !caciExists(sync) {
			if cacsSyncRetired(root) { // 退役容忍（GAP-20261007-16）：同步门已 Go 化且 Go 半在盘 ⇒ S5 不过问（✗ 不 fail-closed）
				fmt.Fprintf(stdout, "   · 同步门已退役（`%s` 不在盘 · Go 半在盘）⇒ S5 不过问\n", cacsSyncGateRel)
			} else {
				nocalls = append(nocalls, fmt.Sprintf("S5 **未取数**：同步门不在（%s）⇒ 不给结论（rc=2）", sync))
			}
		} else {
			rc, rcNone, _, errS := cacsRunCmd([]string{cacsPyExe, sync}, state, timeout)
			tail := cacsLastLine(errS)
			fmt.Fprintf(stdout, "   · `python3 %s` ⇒ rc=%s\n", cacsSyncGateRel, cacsRcStr(rc, rcNone))
			reds = append(reds, cacsJudgeSync(rc, rcNone, tail)...)
		}
	}

	fmt.Fprint(stdout, "── 口径（照实写 · 防止被读成「已经加了」）\n")
	fmt.Fprint(stdout, "   · 本件**只登记**：`sig_alg` 条目今天**不存在**、预留旗标名**还没被占** ⇒ "+
		"**未落地**（`R10`：不许把预留接口读成「已经升过了」）\n")
	fmt.Fprint(stdout, "   · 「零改」指的是**生效面的判定口径零漂移**；新门禁件按名登记进 `EXCLUDES`/`DROP_EXACT`/"+
		"标记表这条**接线**必须做（只**增行**，既有件一行不改）\n")
	if len(reds) > 0 || len(nocalls) > 0 {
		rcOut := 2
		label := "不给结论"
		if len(reds) > 0 {
			rcOut = 1
			label = "红"
		}
		fmt.Fprintf(stdout, "⇒ 门禁%s（rc=%d）：红 %d 条 · 不给结论 %d 条\n", label, rcOut, len(reds), len(nocalls))
		for _, p := range reds {
			fmt.Fprintf(stdout, "   ✗ %s\n", p)
		}
		for _, p := range nocalls {
			fmt.Fprintf(stdout, "   ⛔ %s\n", p)
		}
		return rcOut
	}
	fmt.Fprint(stdout, "⇒ 全绿（rc=0）：闭集四格与批前逐字相同（`sig_alg` 保持 0）· 单向铁律已在登记表 · "+
		"三个预留旗标名都没被占 · 公开面无漂移\n")
	return 0
}

// ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗ ──

func cacsRefuseJSON(argv []string, stderr io.Writer) *int {
	hit := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(hit))
	for _, h := range hit {
		quoted = append(quoted, fmt.Sprintf("`%s`", h))
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	fields := "无（本门只出人读面）"
	if len(cacsJSONFields) > 0 {
		fs := make([]string, 0, len(cacsJSONFields))
		for _, f := range cacsJSONFields {
			fs = append(fs, fmt.Sprintf("`%s`", f))
		}
		fields = strings.Join(fs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fields)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	rc := 2
	return &rc
}

type cacsOpts struct {
	root         string
	state        string
	stateGiven   bool
	devdocs      string
	devdocsGiven bool
	registry     string
	staticOnly   bool
	timeout      int
	selfTest     bool
	noSelfTest   bool
}

func cacsUsageError(stderr io.Writer, msg string) {
	fmt.Fprint(stderr, cacsUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", cacsProg, msg)
}

func cacsLooksLikeOption(tok string) bool {
	if tok == "" || !strings.HasPrefix(tok, "-") || tok == "-" {
		return false
	}
	if cacsNegNumber(tok) {
		return false
	}
	return true
}

// cacsNegNumber：py argparse 的 `^-\d+$|^-\d*\.\d+$`（负数不算 option）。
func cacsNegNumber(tok string) bool {
	body := tok[1:]
	if body == "" {
		return false
	}
	if !strings.Contains(body, ".") {
		for _, r := range body {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	if strings.Count(body, ".") != 1 {
		return false
	}
	dot := strings.Index(body, ".")
	for i, r := range body {
		if i == dot {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// cacsPyAtoi：py `int()` 的常用面（剥 py 空白 · 认可选正负号与下划线分隔 · 十进）。
func cacsPyAtoi(v string) (int, error) {
	s := caciPyStrip(v)
	if s == "" {
		return 0, errors.New("invalid literal")
	}
	s = strings.ReplaceAll(s, "_", "")
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	return strconv.Atoi(s)
}

func cacsParseArgs(argv []string, stdout, stderr io.Writer) (cacsOpts, int, bool) {
	opts := cacsOpts{root: caciFindRepoRoot(), timeout: 60}
	unrec := []string{}
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if strings.HasPrefix(tok, "--") && tok != "--" {
			name, val, hasEq := tok, "", false
			if j := strings.Index(tok, "="); j >= 0 {
				name, val, hasEq = tok[:j], tok[j+1:], true
			}
			matches := []string{}
			for _, o := range cacsLongOpts {
				if o == name || strings.HasPrefix(o, name) {
					matches = append(matches, o)
				}
			}
			if len(matches) == 0 {
				unrec = append(unrec, tok)
				continue
			}
			if len(matches) > 1 {
				cacsUsageError(stderr, fmt.Sprintf("ambiguous option: %s could match %s", name, strings.Join(matches, ", ")))
				return opts, 2, true
			}
			canon := matches[0]
			switch canon {
			case "--help":
				fmt.Fprint(stdout, cacsHelp)
				return opts, 0, true
			case "--self-test", "--no-self-test", "--static-only":
				if hasEq {
					cacsUsageError(stderr, fmt.Sprintf("argument %s: ignored explicit argument '%s'", canon, val))
					return opts, 2, true
				}
				switch canon {
				case "--self-test":
					opts.selfTest = true
				case "--no-self-test":
					opts.noSelfTest = true
				case "--static-only":
					opts.staticOnly = true
				}
			default:
				v := val
				if !hasEq {
					if i+1 >= len(argv) || cacsLooksLikeOption(argv[i+1]) {
						cacsUsageError(stderr, fmt.Sprintf("argument %s: expected one argument", canon))
						return opts, 2, true
					}
					v = argv[i+1]
					i++
				}
				switch canon {
				case "--root":
					opts.root = v
				case "--state":
					opts.state = v
					opts.stateGiven = true
				case "--devdocs":
					opts.devdocs = v
					opts.devdocsGiven = true
				case "--registry":
					opts.registry = v
				case "--timeout":
					n, err := cacsPyAtoi(v)
					if err != nil {
						cacsUsageError(stderr, fmt.Sprintf("argument --timeout: invalid int value: '%s'", v))
						return opts, 2, true
					}
					opts.timeout = n
				}
			}
			continue
		}
		if tok == "-h" {
			fmt.Fprint(stdout, cacsHelp)
			return opts, 0, true
		}
		unrec = append(unrec, tok)
	}
	if len(unrec) > 0 {
		cacsUsageError(stderr, "unrecognized arguments: "+strings.Join(unrec, " "))
		return opts, 2, true
	}
	return opts, 0, false
}

func cacsLastLine(errS string) string {
	lines := caciPySplitLines(caciPyStrip(errS))
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

// checkApproveClosedsetGateMain 是本门 `zerg gate check check-approve-closedset` 的 Go 入口（M1 转发壳的直调面）。
func checkApproveClosedsetGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := cacsRefuseJSON(argv, stderr); rc != nil {
		return *rc
	}
	opts, code, done := cacsParseArgs(argv, stdout, stderr)
	if done {
		return code
	}
	root := caciPyAbspath(opts.root)
	state := cacsStateDirOf(opts.state, opts.stateGiven)
	devdocs := cacsDevdocsRootOf(root, opts.devdocs, opts.devdocsGiven)
	fmt.Fprintln(stdout, cacsVersion)
	if !opts.noSelfTest && !cacsSelfTest(stdout, root, devdocs) {
		return 1
	}
	if opts.selfTest {
		return 0
	}
	fmt.Fprintf(stdout, "── 真目标：%s（状态目录 %s）\n", root, state)
	if !opts.stateGiven && os.Getenv("ZERG_STATE_DIR") != "" {
		fmt.Fprintf(stdout, "★★ 状态目录是**环境变量 `ZERG_STATE_DIR` 给的**（%s）⇒ 本读数是**沙箱**读数，"+
			"**不是**真状态目录 `~/.zerg/state`（闸里不设这个变量；要读真源请显式 `--state`）\n",
			os.Getenv("ZERG_STATE_DIR"))
	}
	return cacsScan(stdout, root, state, devdocs, opts.registry, opts.staticOnly, opts.timeout)
}
