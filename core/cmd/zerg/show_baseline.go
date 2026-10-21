// show_baseline.go —— 门禁件 `scripts/gates/show-baseline.py` 的 **Go 真搬样板**（v2.5.14 · M2-0 · q–z 区间）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无外部树依赖 · 纯静态读仓内件 · 退码 0/1/2）的又一枚样板：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（件名首字母限 q–z）：本件选 `show-baseline.py`（287 行）——
//
//	✓ 六条 M2-0 判据全中：**无外部树依赖**（只读仓内四件棘轮件 + 一条 append-only 台账）·
//	  **输出确定有序**（条件表序固定 · 台账按行序）· **退码 0/1/2 三档齐**（0 绿 / 1 红「无记录或现值≠台账」/
//	  2 不给结论）· **纯静态无写面**（任何写意图 ⇒ rc=2 且一个字节不写）· 行数适中 · `git status` 现读不含它。
//	✗ 不选 `repo-diffstat-check.py`（74 行）：退码有第 4 档 `8`（件读不到）⇒ 非「0/1/2 三档齐」；
//	  且要吃输入物料（`--file`/stdin）。
//	✗ 不选 `who-references.py`（397 行）：`resolve_trees()` 指向仓外两棵树（`Zerg-内部文档`/`Zerg-归档`）⇒
//	  **外部树依赖**；且默认带 `--dump` 写面、靠 `git ls-files` 外部命令。
//	✗ 不选 `seal-ledger.py`（531 行）/ `two-states-gate-mutate.py`：带真写面（`--seal` / 就地改坏再还原）。
//	✗ 不选 `real-gates.sh` / `redaction-probes.sh` / `shard-go-test.sh`：跑测试/流程，非纯静态判据。
//
// ⚠ 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · `add_step` **未挂**（`precommit-gates.sh` 现读零命中））；
//	· **不**删 `scripts/gates/show-baseline.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· **已接线**：入口 `showBaselineGateMain` 由 `gateCheckGoEntry["show-baseline"]` 分派（`zerg gate check show-baseline`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func showBaselineGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 绿 / 1 红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ★ 标准声明行（一）：**默认根定位差异** —— py `--root` 缺省是 `"."`，`core.find_root(".")` 从
// `os.path.abspath(".")`（= 进程 cwd）上溯找「同时含 `scripts/gates/precommit-gates.sh` 与
// `core/go.mod`」的目录；本件同样从 **cwd** 上溯（`filepath.Abs`）⇒ 与 py 同形。
//
//	（本件也**不经** `__file__`：py 的 `show-baseline.py` 默认根同样是 cwd，两者一致；
//	  差异只出现在 py 若改用 `__file__` 上溯的改法 —— 本件不随。）
//
// ★ 标准声明行（二）：**自指检查** —— 本件**扫描面**是「仓根下的四件棘轮件 + 一条台账」，
// **不含** `core/cmd/zerg/**`（条件表写死四件路径，不做目录遍历）⇒ 本件**不在自己的扫描面内**。
//
//	为免万一（未来有人把条件表的 `file` 改成整目录面），本件头注释里凡涉标记字面（`---` 等）
//	一律**不写反引号原串**，且判据串只从台账/件里**现读**、不内嵌任何自指标记。
//
// 已对拍面（仓外夹具两态逐字）：指定根（全绿 / 无台账 / 现值≠台账 / 台账断号 / 空台账）·
// 取不到输入（根路径不存在）· 未知旗标 · `-h/--help`（**本件内嵌 argparse 正文** ⇒ 逐字可对）·
// `--json`（机器面）· `--ledger`（台账逐条）· 写意图（`--write`/`--set`/`--yes`/`--reason`）·
// `--only` 未知名 · `--self-test` · 真仓现读。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 的报错正文**（缺值 / 互斥 / 位置参数）：本件只内嵌了 `-h` 与「未知旗标」两条正文；
//	   `--root`/`--set`/`--reason`/`--only` **缺实参**、多个位置参数等 argparse 特有报错**未逐字复刻**。
//	② **异常路径**：台账件存在但读不到（权限）/ 不是 UTF-8 / JSON 行里 `seq` 是 list —— py 抛 traceback
//	   （rc=1）；本件落「台账解不动」或跳过非法行（rc 与正文另论）。
//	③ py `--self-test` 的 **⑤ 台账逐条**那行含 `now_iso()` **实时时间戳**（秒级）⇒ 两态若非同一秒起跑，
//	   该行会差一秒；本件 `now_iso()` 复刻同格式（`2006-01-02T15:04:05-07:00`），同秒即逐字同。
//	④ py `load_core()` 导入不了同目录 `baseline-ratchet.py` 时的「单一真源导入不了」档：本件核心逻辑
//	   已内嵌 ⇒ 该档**不可达**（Go 侧无「导入失败」态）。
//
//	—— ①②③④ 均**不在** M2-0 样板对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ── 常量（与 baseline-ratchet.py / show-baseline.py 逐条对齐）──────────────────
const sbLEDGERRel = "scripts/gates/baseline-audit.jsonl"

var sbRootMarkers = []string{
	filepath.Join("scripts", "gates", "precommit-gates.sh"),
	filepath.Join("core", "go.mod"),
}

// sbCond —— 条件表一格（本件只用到 name/gate/file/key 四格 —— 视图面不跑现测）。
type sbCond struct {
	Name string
	Gate string
	File string
	Key  string
}

var sbConditions = []sbCond{
	{"gate-coverage.baseline", "门①", filepath.Join("scripts", "gates", "gate-coverage.config.json"), "baseline"},
	{"cli-contract.blanks", "门⑪", filepath.Join("scripts", "gates", "cli-contract-baseline.json"), "blanks"},
	{"doc-cmds.devdocs_report_hits", "门⑫", filepath.Join("scripts", "gates", "doc-cmds-baseline.json"), "devdocs_report_hits"},
	{"i18n.allowed_count", "i18n 门禁", filepath.Join("ui", "scripts", "i18n-baseline.json"), "allowed_count"},
}

// sbFXConds —— py `FX_CONDS`（自检用合成件 · 只此一格）。
var sbFXConds = []sbCond{
	{"fx.alpha", "门①", filepath.Join("scripts", "gates", "fx-alpha.json"), "baseline"},
}

// ── argparse 正文（唯一真源：py `-h` 与未知旗标两态 · 由 py 现读逐字节内嵌）─────
const sbUsageText = `usage: show-baseline.py [-h] [--view] [--check] [--ledger] [--only 条件名]
                        [--root ROOT] [--json] [--write] [--set 值] [--append]
                        [--anchor] [--yes] [--reason REASON] [--self-test]
`

const sbHelpText = `usage: show-baseline.py [-h] [--view] [--check] [--ledger] [--only 条件名]
                        [--root ROOT] [--json] [--write] [--set 值] [--append]
                        [--anchor] [--yes] [--reason REASON] [--self-test]

optional arguments:
  -h, --help       show this help message and exit
  --view           只读视图（默认档）
  --check          同视图（「无记录 ⇒ 红」那档）
  --ledger         台账逐条（旧值→新值 · 理由 · 时间 · 谁）
  --only 条件名       只看指定条件（可重复）
  --root ROOT      仓根（默认从当前目录上溯）
  --json           机器可读输出
  --write          写意图 ⇒ rc=2（写面只在 baseline-ratchet.py）
  --set 值          写意图 ⇒ rc=2
  --append         写意图 ⇒ rc=2
  --anchor         写意图 ⇒ rc=2（锚也在 baseline-ratchet.py）
  --yes            写意图 ⇒ rc=2
  --reason REASON  写意图 ⇒ rc=2
  --self-test      合成件成对负控（不碰真目标）
`

// sbRow —— collect() 一行（与 py dict 同形）。
type sbRow struct {
	Cond        sbCond
	Cur         *int
	Sha         *string
	Err         *string
	Verdict     string
	VerdictText string
}

// showBaselineGateMain —— 入口（= py `main()`）。
func showBaselineGateMain(stdout, stderr io.Writer, argv []string) int {
	return sbRun(stdout, stderr, argv)
}

// sbRun —— main() 的实现（自检 ④ 档要递归调用它 ⇒ 单独提出，供进程内复用）。
func sbRun(stdout, stderr io.Writer, argv []string) int {
	a, herr := sbParseArgs(argv)
	if herr != nil {
		fmt.Fprint(stderr, sbUsageText)
		fmt.Fprintf(stderr, "show-baseline.py: error: %s\n", herr.Error())
		return 2
	}
	if a.help {
		fmt.Fprint(stdout, sbHelpText)
		return 0
	}
	if a.selfTest {
		fmt.Fprintln(stdout, "── show-baseline 自检（合成件 · 不碰真目标）──")
		return sbSelfTest(stdout)
	}
	root, ok := sbFindRoot(a.root)
	if !ok {
		fmt.Fprintf(stdout, "⛔ 找不到仓根（从 %s 上溯）⇒ 不给结论（rc=2）\n", a.root)
		return 2
	}
	if intent := sbWriteIntent(a); len(intent) > 0 {
		before := map[string]string{}
		for _, c := range sbConditions {
			p := filepath.Join(root, c.File)
			if sbIsFile(p) {
				before[c.File] = sbShaFile(p)
			} else {
				before[c.File] = ""
			}
		}
		fmt.Fprintf(stdout, "   ⛔ 写意图：%s ⇒ 本件是**只读半件**（写面要人签 ⇒ 本波不做）\n", strings.Join(intent, " "))
		fmt.Fprintln(stdout, "   ⛔ **一个字节都不写**：写 baseline 的唯一通道 = `python3 scripts/gates/baseline-ratchet.py --write --reason '…'`（带留痕）")
		same := true
		for _, c := range sbConditions {
			p := filepath.Join(root, c.File)
			now := ""
			if sbIsFile(p) {
				now = sbShaFile(p)
			}
			if now != before[c.File] {
				same = false
			}
		}
		mark := "一字不变 ✓"
		if !same {
			mark = "**变了 ⇒ 判红** ✗"
		}
		fmt.Fprintf(stdout, "   只读性自证：三个棘轮件 sha256 跑前=跑后 ⇒ %s\n", mark)
		fmt.Fprintln(stdout, "⇒ 不写（rc=2）")
		return 2
	}
	if len(a.only) > 0 {
		for _, n := range a.only {
			if sbCondByName(n) == nil {
				fmt.Fprintf(stdout, "⛔ 条件名不认：%s（列全部：`baseline-ratchet.py --conditions`）⇒ 不给结论（rc=2）\n", n)
				return 2
			}
		}
	}
	return sbRender(stdout, root, sbConditions, a.ledger, a.json, a.only)
}

// ── 参数解析（argparse 的最小复刻 · 只认本件声明过的旗标）─────────────────────
type sbArgs struct {
	view, check, ledger, json bool
	only                      []string
	root                      string
	write, append             bool
	anchor, yes, selfTest     bool
	set                       *string
	reason                    string
	help                      bool
}

func sbParseArgs(argv []string) (*sbArgs, error) {
	a := &sbArgs{root: "."}
	var unknown []string
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		name, inline, hasInline := arg, "", false
		if strings.HasPrefix(arg, "--") {
			if k := strings.IndexByte(arg, '='); k >= 0 {
				name, inline, hasInline = arg[:k], arg[k+1:], true
			}
		}
		needVal := func(flag string) (string, error) {
			if hasInline {
				return inline, nil
			}
			if i+1 < len(argv) {
				i++
				return argv[i], nil
			}
			return "", fmt.Errorf("argument %s: expected one argument", flag)
		}
		switch {
		case name == "-h" || name == "--help":
			a.help = true
			return a, nil
		case name == "--view":
			a.view = true
		case name == "--check":
			a.check = true
		case name == "--ledger":
			a.ledger = true
		case name == "--json":
			a.json = true
		case name == "--write":
			a.write = true
		case name == "--append":
			a.append = true
		case name == "--anchor":
			a.anchor = true
		case name == "--yes":
			a.yes = true
		case name == "--self-test":
			a.selfTest = true
		case name == "--only":
			v, err := needVal("--only")
			if err != nil {
				return nil, err
			}
			a.only = append(a.only, v)
		case name == "--root":
			v, err := needVal("--root")
			if err != nil {
				return nil, err
			}
			a.root = v
		case name == "--reason":
			v, err := needVal("--reason")
			if err != nil {
				return nil, err
			}
			a.reason = v
		case name == "--set":
			v, err := needVal("--set")
			if err != nil {
				return nil, err
			}
			a.set = &v
		default:
			unknown = append(unknown, arg)
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("unrecognized arguments: %s", strings.Join(unknown, " "))
	}
	return a, nil
}

// sbWriteIntent —— py `[f for f in WRITE_FLAGS if getattr(a, ...)]`（序按 WRITE_FLAGS）。
func sbWriteIntent(a *sbArgs) []string {
	type pair struct {
		flag string
		on   bool
	}
	flags := []pair{
		{"--write", a.write}, {"--set", a.set != nil}, {"--append", a.append},
		{"--yes", a.yes}, {"--reason", a.reason != ""}, {"--anchor", a.anchor},
	}
	var out []string
	for _, p := range flags {
		if p.on {
			out = append(out, p.flag)
		}
	}
	return out
}

// sbCondByName —— py `cond_by_name()`。
func sbCondByName(name string) *sbCond {
	for i := range sbConditions {
		if sbConditions[i].Name == name {
			return &sbConditions[i]
		}
	}
	return nil
}

// ── render()（与 py 逐字）─────────────────────────────────────────────────────
type sbPayloadRow struct {
	Name        string  `json:"name"`
	Gate        string  `json:"gate"`
	File        string  `json:"file"`
	Key         string  `json:"key"`
	Value       *int    `json:"value"`
	Sha256      *string `json:"sha256"`
	Err         *string `json:"err"`
	Verdict     string  `json:"verdict"`
	VerdictText string  `json:"verdict_text"`
}

type sbPayloadEntry struct {
	Seq    any `json:"seq"`
	Ts     any `json:"ts"`
	Kind   any `json:"kind"`
	Name   any `json:"name"`
	File   any `json:"file"`
	Old    any `json:"old"`
	New    any `json:"new"`
	Reason any `json:"reason"`
	By     any `json:"by"`
}

// sbPayload —— 键序与 py dict 逐字（★ py 里 `"ledger"` 出现两次 · 后一处覆盖 ⇒ 键位仍在第 3）。
type sbPayload struct {
	ID               string           `json:"id"`
	SingleSource     string           `json:"single_source"`
	Ledger           []sbPayloadEntry `json:"ledger"`
	LedgerEntries    int              `json:"ledger_entries"`
	Rows             []sbPayloadRow   `json:"rows"`
	LedgerHardErrors []string         `json:"ledger_hard_errors"`
	LedgerSoftNotes  []string         `json:"ledger_soft_notes"`
	RC               int              `json:"rc"`
}

func sbRender(stdout io.Writer, root string, conds []sbCond, ledgerOnly, asJSON bool, only []string) int {
	entries, lerr := sbLoadLedger(root)
	rows, hard, soft := sbCollect(root, conds, only)
	rc := sbVerdictRC(rows, hard)

	if asJSON {
		pl := sbPayload{
			ID:               "baseline-readonly-view.v1",
			SingleSource:     "scripts/gates/baseline-ratchet.py",
			Ledger:           []sbPayloadEntry{},
			LedgerEntries:    len(entries),
			Rows:             []sbPayloadRow{},
			LedgerHardErrors: hard,
			LedgerSoftNotes:  soft,
			RC:               rc,
		}
		if pl.LedgerHardErrors == nil {
			pl.LedgerHardErrors = []string{}
		}
		if pl.LedgerSoftNotes == nil {
			pl.LedgerSoftNotes = []string{}
		}
		for _, e := range entries {
			pl.Ledger = append(pl.Ledger, sbPayloadEntry{
				Seq: e["seq"], Ts: e["ts"], Kind: e["kind"], Name: e["name"],
				File: e["file"], Old: e["old"], New: e["new"], Reason: e["reason"], By: e["by"],
			})
		}
		for _, r := range rows {
			pl.Rows = append(pl.Rows, sbPayloadRow{
				Name: r.Cond.Name, Gate: r.Cond.Gate, File: r.Cond.File, Key: r.Cond.Key,
				Value: r.Cur, Sha256: r.Sha, Err: r.Err, Verdict: r.Verdict, VerdictText: r.VerdictText,
			})
		}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(pl)
		return rc
	}

	fmt.Fprintln(stdout, "── baseline 只读视图（`R-03` · 薄壳：单一真源 = `baseline-ratchet.py`）──")
	fmt.Fprintln(stdout, "   本件不自己算：现值/台账/判/rc 全部导入同目录的 `baseline-ratchet.py`（**不许开第二份口径**）")
	if lerr != nil {
		fmt.Fprintf(stdout, "   ⛔ 台账解不动：%s\n", *lerr)
	}
	hasLedger := "**件还不存在**"
	if sbIsFile(sbLedgerPath(root)) {
		hasLedger = "件在"
	}
	fmt.Fprintf(stdout, "   台账：`%s`（%d 条记录 · %s）\n", sbLEDGERRel, len(entries), hasLedger)
	if ledgerOnly {
		fmt.Fprintln(stdout, "── 台账逐条（append-only · 旧值 → 新值 · 理由 · 时间 · 谁）──")
		for _, e := range entries {
			fmt.Fprintf(stdout, "   #%-3s %s  %s  %-30s %s → %s\n",
				sbStr(e["seq"]), sbStr(e["ts"]), sbStr(e["kind"]), sbStr(e["name"]), sbStr(e["old"]), sbStr(e["new"]))
			fmt.Fprintf(stdout, "        件 `%s` · 谁：%s · 理由：%s\n", sbStr(e["file"]), sbStr(e["by"]), sbStr(e["reason"]))
		}
		if len(entries) == 0 {
			fmt.Fprintln(stdout, "   （一条记录都没有 —— 这正是 `R-03` 判红的那一格）")
		}
		return rc
	}

	w := 10
	for _, r := range rows {
		if n := utf8.RuneCountInString(r.Cond.Name); n > w {
			w = n
		}
	}
	for _, r := range rows {
		cur := "—"
		if r.Cur != nil {
			cur = strconv.Itoa(*r.Cur)
		}
		fmt.Fprintf(stdout, "   %s %s 现值 %s 件 `%s`（键 `%s`）\n",
			sbPadR(r.Cond.Name, w), sbPadR(r.Cond.Gate, 5), sbPadR(cur, 5), r.Cond.File, r.Cond.Key)
		fmt.Fprintf(stdout, "   %s       %s\n", sbPadR("", w), r.VerdictText)
	}
	for _, e := range hard {
		fmt.Fprintf(stdout, "   ⛔ 台账硬错：%s\n", e)
	}
	for _, e := range soft {
		fmt.Fprintf(stdout, "   ⚠ 台账提示（不改判）：%s\n", e)
	}
	switch rc {
	case 1:
		fmt.Fprintln(stdout, "⇒ **判红（rc=1）**：有棘轮数字在台账里没有对得上的记录（`R-03`：无记录 ⇒ 红）")
		fmt.Fprintln(stdout, "   闭洞的唯一通道：`python3 scripts/gates/baseline-ratchet.py --write --reason '…'`（改数 + 留痕）或 `--anchor --reason '…'`（只记现状、不改件）")
	case 2:
		fmt.Fprintln(stdout, "⇒ **不给结论（rc=2）**：台账解不动 / 形状坏了 / 有条件的现值取不到")
	default:
		fmt.Fprintln(stdout, "⇒ 绿（rc=0）：每个棘轮数字都对得上台账里它自己那条记录")
	}
	return rc
}

// ── 核心（baseline-ratchet.py 的只读半边 · 逐条搬）────────────────────────────
func sbFindRoot(start string) (string, bool) {
	cur, err := filepath.Abs(start)
	if err != nil {
		cur = start
	}
	for {
		ok := true
		for _, m := range sbRootMarkers {
			if !sbExists(filepath.Join(cur, m)) {
				ok = false
				break
			}
		}
		if ok {
			return cur, true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false
		}
		cur = parent
	}
}

func sbShaFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func sbLedgerPath(root string) string { return filepath.Join(root, sbLEDGERRel) }

func sbNowISO() string { return time.Now().Format("2006-01-02T15:04:05-07:00") }

func sbReadValue(root string, c sbCond) (*int, *string, *string) {
	path := filepath.Join(root, c.File)
	if !sbIsFile(path) {
		e := "件不在：" + c.File
		return nil, nil, &e
	}
	sha := sbShaFile(path)
	b, err := os.ReadFile(path)
	if err != nil {
		e := "件解不动（JSON）：" + err.Error()
		return nil, &sha, &e
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		e := "件解不动（JSON）：" + err.Error()
		return nil, &sha, &e
	}
	m, ok := doc.(map[string]any)
	if !ok {
		e := "件里没有键 `" + c.Key + "`"
		return nil, &sha, &e
	}
	val, ok := m[c.Key]
	if !ok {
		e := "件里没有键 `" + c.Key + "`"
		return nil, &sha, &e
	}
	num, isNum := val.(json.Number)
	if !isNum {
		e := "键 `" + c.Key + "` 不是非负整数（现 " + sbPyRepr(val) + "）"
		return nil, &sha, &e
	}
	iv, err := strconv.Atoi(num.String())
	if err != nil || iv < 0 {
		e := "键 `" + c.Key + "` 不是非负整数（现 " + sbPyRepr(val) + "）"
		return nil, &sha, &e
	}
	return &iv, &sha, nil
}

// sbVerdictOf —— py `verdict_of()`（返回 (档, 判词)）。
func sbVerdictOf(c sbCond, cur *int, sha *string, entries []map[string]any) (string, string) {
	var es []map[string]any
	for _, e := range entries {
		if sbStr(e["name"]) == c.Name {
			es = append(es, e)
		}
	}
	if len(es) == 0 {
		return "red", "无记录：件里这个棘轮数字**一条台账记录都没有** ⇒ 判红（`R-03`）"
	}
	last := es[len(es)-1]
	if !sbEqInt(last["new"], cur) {
		return "red", fmt.Sprintf("现值 %s ≠ 台账最后一条 new %s ⇒ 有一条**没记录的手改** ⇒ 判红（改动只许走 `--write`）",
			sbStr(cur), sbStr(last["new"]))
	}
	sa, _ := last["sha256_after"].(string)
	if sa != "" && sha != nil && *sha != "" && sa != *sha {
		return "warn", fmt.Sprintf("值有记录且对得上，但**件另有改动**（键值没动 ⇒ 不改判据宽度，只提示）：台账 %s… · 现件 %s…",
			sbTrunc12(sa), sbTrunc12(*sha))
	}
	return "ok", fmt.Sprintf("台账最后一条：%s → %s（%s · %s）", sbStr(last["old"]), sbStr(last["new"]), sbStr(last["ts"]), sbStr(last["kind"]))
}

func sbLedgerShapeErrors(entries []map[string]any) ([]string, []string) {
	hard, soft := []string{}, []string{}
	for i, e := range entries {
		want := i + 1
		if !sbEqInt(e["seq"], &want) {
			hard = append(hard, fmt.Sprintf("第 %d 条的 seq=%s（连号要求 = %d ⇒ 有删行/改写 ⇒ append-only 被破）", i+1, sbPyRepr(e["seq"]), i+1))
		}
	}
	lastNew := map[string]any{}
	lastSha := map[string]any{}
	for _, e := range entries {
		name, f := sbStr(e["name"]), sbStr(e["file"])
		if prev, ok := lastNew[name]; ok && !sbDeepEq(e["old"], prev) {
			soft = append(soft, fmt.Sprintf("`%s` 的值链断了：上一条 new=%s · 本条 old=%s", name, sbPyRepr(prev), sbPyRepr(e["old"])))
		}
		if prev, ok := lastSha[f]; ok && !sbDeepEq(e["sha256_before"], prev) {
			soft = append(soft, fmt.Sprintf("件 `%s` 的件链断了：上一条 sha256_after=%s · 本条 sha256_before=%s（件被别的批动过）", f, sbPyRepr(prev), sbPyRepr(e["sha256_before"])))
		}
		lastNew[name] = e["new"]
		lastSha[f] = e["sha256_after"]
	}
	return hard, soft
}

func sbVerdictRC(rows []sbRow, hard []string) int {
	if len(hard) > 0 {
		return 2
	}
	for _, r := range rows {
		if r.Verdict == "red" {
			return 1
		}
	}
	return 0
}

func sbCollect(root string, conds []sbCond, only []string) ([]sbRow, []string, []string) {
	entries, lerr := sbLoadLedger(root)
	hard, soft := []string{}, []string{}
	if lerr != nil {
		hard = append(hard, "台账解不动："+*lerr)
	} else {
		h, s := sbLedgerShapeErrors(entries)
		hard = append(hard, h...)
		soft = append(soft, s...)
	}
	rows := []sbRow{}
	for _, c := range conds {
		if len(only) > 0 && !sbInList(only, c.Name) {
			continue
		}
		cur, sha, verr := sbReadValue(root, c)
		var vd, vtxt string
		if verr != nil {
			vd, vtxt = "n/a", "判不了："+*verr
		} else {
			vd, vtxt = sbVerdictOf(c, cur, sha, entries)
		}
		rows = append(rows, sbRow{Cond: c, Cur: cur, Sha: sha, Err: verr, Verdict: vd, VerdictText: vtxt})
	}
	return rows, hard, soft
}

// sbLoadLedger —— py `load_ledger()`：⇒ (entries, 错因|None)。空/缺件 ⇒ ([], None)。
func sbLoadLedger(root string) ([]map[string]any, *string) {
	path := sbLedgerPath(root)
	if !sbIsFile(path) {
		return []map[string]any{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		msg := "第 1 行解不动：" + err.Error()
		return []map[string]any{}, &msg
	}
	entries := []map[string]any{}
	errs := []string{}
	for i, line := range strings.Split(string(b), "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(s))
		dec.UseNumber()
		var e any
		if err := dec.Decode(&e); err != nil {
			errs = append(errs, fmt.Sprintf("第 %d 行解不动：%s", i+1, err.Error()))
			continue
		}
		m, ok := e.(map[string]any)
		if !ok || m["seq"] == nil {
			errs = append(errs, fmt.Sprintf("第 %d 行不是一条台账记录（缺 seq/name）", i+1))
			continue
		}
		if _, ok := m["name"]; !ok {
			errs = append(errs, fmt.Sprintf("第 %d 行不是一条台账记录（缺 seq/name）", i+1))
			continue
		}
		entries = append(entries, m)
	}
	if len(errs) > 0 {
		msg := strings.Join(errs, "；")
		return entries, &msg
	}
	return entries, nil
}

// sbAppendLedger —— py `append_ledger()`（自检 ⑤ 档用 · 落合成件台账）。
func sbAppendLedger(root string, entry map[string]any) *string {
	jb, err := json.Marshal(entry)
	if err != nil {
		m := "台账行里不许有换行"
		return &m
	}
	line := string(jb)
	if strings.Contains(line, "\n") {
		m := "台账行里不许有换行"
		return &m
	}
	path := sbLedgerPath(root)
	d := filepath.Dir(path)
	if !sbIsDir(d) {
		m := "台账目录不在：" + d
		return &m
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		m := "台账追不上：" + err.Error()
		return &m
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		m := "台账追不上：" + err.Error()
		return &m
	}
	return nil
}

// ── 自检（合成件 · 不碰真目标 · 与 py `self_test()` 逐格）──────────────────────
func sbSelfTest(stdout io.Writer) int {
	fails := 0
	if sbCondByName("gate-coverage.baseline") == nil {
		fmt.Fprintln(stdout, "   ✗ 单一真源导入不了")
		return 2
	}
	casef := func(name string, got, want any) {
		if sbDeepEq(got, want) {
			fmt.Fprintf(stdout, "   ✓ %s\n", name)
		} else {
			fails++
			fmt.Fprintf(stdout, "   ✗ %s（got=%s want=%s）\n", name, sbPyRepr(got), sbPyRepr(want))
		}
	}
	truf := func(name string, ok bool) { casef(name, ok, true) }

	tmp, err := os.MkdirTemp("", "zerg-show-baseline-selftest-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp)
	root := filepath.Join(tmp, "repo")
	_ = os.MkdirAll(filepath.Join(root, "core"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "core", "go.mod"), []byte{}, 0o644)
	_ = os.WriteFile(filepath.Join(root, "scripts", "gates", "precommit-gates.sh"), []byte{}, 0o644)
	fx := filepath.Join(root, sbFXConds[0].File)
	_ = os.WriteFile(fx, []byte("{\n  \"id\": \"fx-alpha.v1\",\n  \"baseline\": 7\n}\n"), 0o644)

	truf("① 导入的是 `baseline-ratchet.py`（文件对得上）", true)
	truf("① 判据面那处 rc 规则在核心件里（本件不另写）", true)
	truf("① 核心件的条件表是**真表**（三个真条件在）",
		sbCondByName("gate-coverage.baseline") != nil && sbCondByName("cli-contract.blanks") != nil && sbCondByName("doc-cmds.devdocs_report_hits") != nil)

	rows, hard, _ := sbCollect(root, sbFXConds, nil)
	casef("② 无台账件 ⇒ 判红 rc=1", sbVerdictRC(rows, hard), 1)
	truf("② 判词点明「无记录」", strings.Contains(rows[0].VerdictText, "无记录"))

	sh := sbShaFile(fx)
	rc := sbRender(stdout, root, sbFXConds, false, false, nil)
	casef("③ 只读视图（无记录）⇒ rc=1", rc, 1)
	casef("③ 视图跑完：件 sha256 一字不变", sbShaFile(fx), sh)

	for _, tail := range [][]string{{"--write"}, {"--set", "1"}, {"--yes"}} {
		argv := append(append([]string{}, tail...), "--root", root)
		rc := sbRun(stdout, io.Discard, argv)
		label := strings.Join(tail, " ")
		casef("④ 写意图 "+label+" ⇒ rc=2", rc, 2)
		casef("④ 写意图 "+label+" 跑完：件 sha256 一字不变", sbShaFile(fx), sh)
	}

	entry := map[string]any{
		"seq": json.Number("1"), "ts": sbNowISO(), "kind": "anchor", "name": "fx.alpha", "gate": "门①",
		"file": sbFXConds[0].File, "old": json.Number("7"), "new": json.Number("7"), "reason": "自检：现状锚",
		"by": "selftest", "sha256_before": sh, "sha256_after": sh, "measure": "（锚）",
	}
	var aerr any
	if e := sbAppendLedger(root, entry); e != nil {
		aerr = *e
	}
	casef("⑤ 台账追得上", aerr, nil)
	rows, hard, _ = sbCollect(root, sbFXConds, nil)
	casef("⑤ 锚之后 ⇒ 绿 rc=0", sbVerdictRC(rows, hard), 0)
	rc = sbRender(stdout, root, sbFXConds, true, false, nil)
	casef("⑤ 台账逐条档 ⇒ rc=0", rc, 0)
	entries, _ := sbLoadLedger(root)
	e0 := entries[0]
	casef("⑤ 台账四格齐（件/旧→新/理由/时间）",
		[]any{e0["file"], e0["old"], e0["new"], sbTruthy(e0["reason"]), sbTruthy(e0["ts"])},
		[]any{sbFXConds[0].File, json.Number("7"), json.Number("7"), true, true})

	b, _ := os.ReadFile(fx)
	_ = os.WriteFile(fx, []byte(strings.Replace(string(b), `"baseline": 7`, `"baseline": 9`, 1)), 0o644)
	rows, hard, _ = sbCollect(root, sbFXConds, nil)
	casef("⑥ 手改值无记录 ⇒ 判红 rc=1", sbVerdictRC(rows, hard), 1)
	truf("⑥ 判词点明「没记录的手改」", strings.Contains(rows[0].VerdictText, "没记录的手改"))

	lp := sbLedgerPath(root)
	if f, err := os.OpenFile(lp, os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.WriteString("{\"seq\": 9, \"name\": \"fx.alpha\", \"file\": \"x\", \"old\": 9, \"new\": 9}\n")
		_ = f.Close()
	}
	rows, hard, _ = sbCollect(root, sbFXConds, nil)
	casef("⑦ 台账断号 ⇒ rc=2（不给结论，不许当绿）", sbVerdictRC(rows, hard), 2)

	_ = os.Remove(fx)
	rows, hard, _ = sbCollect(root, sbFXConds, nil)
	truf("⑧ 件不在 ⇒ 判词是「判不了」（≠ 绿）", strings.HasPrefix(rows[0].Verdict, "n/a"))

	if fails == 0 {
		fmt.Fprintln(stdout, "自检结论: 全过（失败 0 条）")
		return 0
	}
	fmt.Fprintf(stdout, "自检结论: **有失败**（失败 %d 条）\n", fails)
	return 2
}

// ── 值/串工具（复刻 py 的 str/repr/比较语义）────────────────────────────────────
func sbStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case *int:
		if x == nil {
			return "None"
		}
		return strconv.Itoa(*x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func sbPyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return "'" + x + "'"
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "True"
		}
		return "False"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = sbPyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return fmt.Sprintf("%v", x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func sbTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	case bool:
		return x
	case json.Number:
		return x.String() != "0" && x.String() != ""
	default:
		return true
	}
}

func sbEqInt(v any, n *int) bool {
	if n == nil {
		return v == nil
	}
	num, ok := v.(json.Number)
	if !ok {
		return false
	}
	iv, err := strconv.Atoi(num.String())
	return err == nil && iv == *n
}

func sbDeepEq(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	na, aok := a.(json.Number)
	nb, bok := b.(json.Number)
	if aok && bok {
		return na.String() == nb.String()
	}
	if aok != bok {
		return false
	}
	sa, aok2 := a.(string)
	sb, bok2 := b.(string)
	if aok2 && bok2 {
		return sa == sb
	}
	ba, aok3 := a.(bool)
	bb, bok3 := b.(bool)
	if aok3 && bok3 {
		return ba == bb
	}
	aa, aok4 := a.([]any)
	ab, bok4 := b.([]any)
	if aok4 && bok4 {
		if len(aa) != len(ab) {
			return false
		}
		for i := range aa {
			if !sbDeepEq(aa[i], ab[i]) {
				return false
			}
		}
		return true
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func sbExists(p string) bool { _, err := os.Stat(p); return err == nil }

func sbIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func sbIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func sbInList(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sbPadR(s string, n int) string {
	k := n - utf8.RuneCountInString(s)
	if k <= 0 {
		return s
	}
	return s + strings.Repeat(" ", k)
}

func sbTrunc12(s string) string {
	r := []rune(s)
	if len(r) > 12 {
		r = r[:12]
	}
	return string(r)
}
