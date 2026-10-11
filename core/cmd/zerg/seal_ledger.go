// seal_ledger.go —— 判据件 `scripts/gates/seal-ledger.py`（531 行 · sha16 9ca69cf8d4cc9a2f）的
// **Go 真搬**（v2.5.14 · M2 · 核章档（第四档）族靶 · 入口 `sealLedgerGateMain`）。
//
// 验收判据 = 对同一 argv 跑两态（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**
// （除下列显式声明的新口径差异面与未覆盖面）。
//
//	· 入口签名：`func sealLedgerGateMain(stdout, stderr io.Writer, argv []string) int`
//	  argv 与 py `sys.argv[1:]` 同形（不含程序名）。
//	· 旗标面（逐字照 py argparse）：`--seal <趟目录>` · `--verify` · `--check` · `--ledger <件>`
//	  · `--write` · `--reason <一句>` · `--json <字段>`（值被忽略 · 恒 dump 全表）· `-h/--help`。
//	· 退码：0 全部章有效 / 1 有 stale·gap（逐枚点名）/ 2 判不了（缺件·读不回·用法错）/ 8 自检不过。
//	· 写面：**唯一写通道** = `--seal <趟目录> --write --reason <一句>` ⇒ append-only 追加到章账
//	  （缺省 `scripts/gates/seal/ledger.jsonl`）。缺 `--reason` ⇒ rc=2。**本件对拍与自检一律只在
//	  仓外夹具走写面，✗ 不写真实章账。**
//	· 子进程面：`git -C <root> …`（多条）· `<root>/bin/zerg version` · `bash <root>/scripts/gates/
//	  precommit-gates.sh --list` · `<root>/bin/zerg gate show <步名> --json cmd`。
//	· 遍历面：**本件零目录遍历**（无 `os.walk`/`listdir`/`glob`）—— 唯一的「面」是
//	  `git status --porcelain -z` 的输出令牌串，经 `sorted(set(...))` ⇒ 输出序决定者 = 该显式排序。
//	· 外部真源：章账 `scripts/gates/seal/ledger.jsonl` · 门件 `scripts/gates/precommit-gates.sh`
//	  · 档位链真源 `scripts/gates/wallclock-baseline.json`（仅用于打印其仓内相对路径）。
//
// 出厂声明行（每枚必写）：
//
//	(a) **默认根定位差异**：py 从 `__file__` 上溯两层（`HERE/../..`）⟷ Go 无 `__file__` 语境，
//	    只能从 **cwd 上溯**（找 `core/cmd/zerg/main.go`，找不到回落 cwd）。「脚本位置 ≠ cwd」时
//	    路径推导面不同 ⇒ ✗ 不声称逐字对齐。夹具对拍时两档 cwd 逐字相同、且夹具根无 `core/…` ⇒
//	    两态 REPO_ROOT 都等于夹具根。
//	(b) **自指检查**：本件落在 `core/cmd/zerg/`，**不落在** `scripts/gates/*.py` 的判据扫描面内；
//	    件内零绝对路径字面量、零私有路径原串；引用的外部件一律是**仓内相对**串
//	    （`scripts/gates/seal/ledger.jsonl` 等），与 py 原件同形。逐条依据见件尾「自指」节。
//
// 未覆盖面（详见件尾）：① argparse 帮助/报错正文 ② seal 记录的 `at` 时刻字段（Go 侧另给
// Go-only 注入旋钮 `--now`；py 不认该旗标 ⇒ 该旗标属声明口径差）③ py `subprocess` 的 timeout
// 超时档正文 ④ `--check` 自检的运行时表达（py 起 python 子进程 ⟷ Go 进程内复用入口）。
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const (
	// slgLedgerRel —— 章账缺省落点（**别名** · 单一真源 = `family_gate_seal_verify.go` 的 `sealLedgerDefaultRel`）。
	// ★ 本处 ✗ 不再另写一份同值字面量（「同值异名常量并存」已并 · GAP-20261001-65）。
	slgLedgerRel = sealLedgerDefaultRel
	// slgGateScriptRel —— 门件（步表真源）。
	slgGateScriptRel = "scripts/gates/precommit-gates.sh"
	// slgBaselineName —— 档位链真源（仅打印其仓内相对路径）。
	slgBaselineName = "wallclock-baseline.json"
	// slgChainSeed —— 链式摘要种子（逐字照 py `CHAIN_SEED`）。
	slgChainSeed = "zerg-seal-chain/1"
	// slgSelfTestFailRC —— 自检不过的退码（**⊆ 真源退码表已登记集** · 具名常量）。
	slgSelfTestFailRC = 8
	// slgBLOCKEDRC —— 「判不了 · 不给结论」退码。
	slgBLOCKEDRC = 2
	// slgJudgeRC —— 「有不符项」退码。
	slgJudgeRC = 1
)

// slgSealStates —— 核章输出**只许**这三种状态（闭集 · 禁 PASS）。
var slgSealStates = []string{"未真跑（核章）", "stale", "gap"}

// slgChainKeys —— `chain_of` 逐格纳入的键（**序**照 py 逐字）。
var slgChainKeys = []string{"step", "status", "rc", "worktree_fp", "head",
	"item_set_digest", "judge_ver", "gate_ver", "chapter_no"}

// ────────────────────────────────────────────────────────────────────────────
// py 语义原语（自写同构 · ✗ 不用 filepath.Join / filepath.Abs / filepath.Rel）
// ────────────────────────────────────────────────────────────────────────────

// slgPyJoin —— `os.path.join`（posixpath 语义）：逐段串拼、后段绝对则重置、空段按原样处理。
// ★ 与 `filepath.Join` 不同：`..` 原样留、后段绝对重置、空段成 `a/`。
func slgPyJoin(a string, rest ...string) string {
	path := a
	for _, b := range rest {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// slgPyNormpath —— `os.path.normpath`（含「恰两枚前导斜杠保留」的 CPython 特例）。
func slgPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initial = 2
		}
	}
	comps := strings.Split(path, "/")
	var newComps []string
	for _, comp := range comps {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initial == 0 && len(newComps) == 0) ||
			(len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	path = strings.Join(newComps, "/")
	if initial > 0 {
		path = strings.Repeat("/", initial) + path
	}
	if path == "" {
		return "."
	}
	return path
}

// slgPyAbspath —— `os.path.abspath`（纯词法：相对则与 cwd 串拼、再 normpath）。
func slgPyAbspath(path string) string {
	if strings.HasPrefix(path, "/") {
		return slgPyNormpath(path)
	}
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return slgPyNormpath(slgPyJoin(wd, path))
}

// slgPyDirname —— `os.path.dirname`（= `split` 的 head · 剥尾斜杠）。
func slgPyDirname(path string) string {
	p := path
	for len(p) > 1 && strings.HasSuffix(p, "/") {
		p = p[:len(p)-1]
	}
	i := strings.LastIndex(p, "/")
	switch {
	case i < 0:
		return ""
	case i == 0:
		return "/"
	default:
		return p[:i]
	}
}

// slgPyRelpath —— `os.path.relpath`（元素级公共前缀 + `..` 前缀算法）。
func slgPyRelpath(path, start string) string {
	if path == "" {
		return "."
	}
	startAbs := slgPyAbspath(start)
	pathAbs := slgPyAbspath(path)
	startList := slgSplitComps(startAbs)
	pathList := slgSplitComps(pathAbs)
	n := 0
	for n < len(startList) && n < len(pathList) && startList[n] == pathList[n] {
		n++
	}
	var rel []string
	for i := n; i < len(startList); i++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pathList[n:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

func slgSplitComps(p string) []string {
	var out []string
	for _, x := range strings.Split(p, "/") {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

// slgPyStrip —— `str.strip()`（py 的 Unicode 空白，**含 \x1c-\x1f / \x85 / \xa0**）。
func slgPyStrip(s string) string {
	return strings.TrimFunc(s, slgIsPySpace)
}

func slgIsPySpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r',
		0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	switch {
	case r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x1680, r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

// slgIsFile / slgIsDir —— 跟随符号链接（与 py `os.path.isfile` / `isdir` 同为 stat 语义）。
func slgIsFile(p string) bool { fi, err := os.Stat(p); return err == nil && fi.Mode().IsRegular() }
func slgIsDir(p string) bool  { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

// slgReadText —— py 文本模式读（universal newlines：`\r\n` / 裸 `\r` 折成 `\n`）。
func slgReadText(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s, true
}

// ────────────────────────────────────────────────────────────────────────────
// 基础判据件（逐字照 py）
// ────────────────────────────────────────────────────────────────────────────

func slgDie(stderr io.Writer, msg string, rc int) int {
	fmt.Fprintln(stderr, msg)
	return rc
}

// slgShaFile —— py `sha_file`：读不回 ⇒ 空串。
func slgShaFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// slgGit —— py `git(root, *args)`：rc≠0 或起不来 ⇒ 空串；否则 stdout。
func slgGit(root string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	full := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// slgLedgerRelOf —— py `ledger_rel_of`：仓外/绝对 ⇒ 回落缺省落点。
func slgLedgerRelOf(root, ledger string) string {
	rel := slgPyRelpath(slgPyAbspath(ledger), root)
	if strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "/") {
		return slgLedgerRel
	}
	return rel
}

// slgItemSetDigest —— py `item_set_digest`：只收「改动的件」（git 脏面），件名排序后整串再 hash。
// ★ 精确排除章账自身（只排除那一个件）。
func slgItemSetDigest(root, excludeRel string) (string, []string) {
	raw := slgGit(root, "--no-optional-locks", "status", "--porcelain", "-z")
	var names []string
	for _, tok := range strings.Split(raw, "\x00") {
		tok = strings.TrimRight(tok, "\n")
		if tok == "" {
			continue
		}
		if len([]rune(tok)) > 3 && tok[2] == ' ' {
			tok = tok[3:]
		}
		names = append(names, tok)
	}
	skip := excludeRel
	if skip == "" {
		skip = slgLedgerRel
	}
	seen := map[string]bool{}
	var uniq []string
	for _, n := range names {
		if n == "" || n == skip || seen[n] {
			continue
		}
		seen[n] = true
		uniq = append(uniq, n)
	}
	sort.Strings(uniq)
	h := sha256.New()
	h.Write([]byte("item-set/1\x00"))
	for _, n := range uniq {
		blob := slgPyStrip(slgGit(root, "--no-optional-locks", "hash-object", "--", n))
		if blob == "" {
			blob = "-"
		}
		h.Write([]byte(n + "\x00" + blob + "\x00"))
	}
	return hex.EncodeToString(h.Sum(nil)), uniq
}

// slgJudgeVersion —— py `judge_version`：命令串 + 串里点到的在盘门件内容。
func slgJudgeVersion(root, stepCmd string) string {
	h := sha256.New()
	h.Write([]byte("judge/1\x00"))
	h.Write([]byte(stepCmd))
	h.Write([]byte("\x00"))
	norm := strings.ReplaceAll(stepCmd, "'", " ")
	norm = strings.ReplaceAll(norm, "\"", " ")
	for _, tok := range strings.Fields(norm) {
		if !strings.Contains(tok, "/") {
			continue
		}
		cand := slgPyJoin(root, tok)
		if slgIsFile(cand) {
			h.Write([]byte(tok + "=" + slgShaFile(cand)))
			h.Write([]byte("\x00"))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// slgGateVersion —— py `gate_version`：门件 sha + `zerg` 自报身份（取不到就只算门件 sha）。
func slgGateVersion(root string) (string, string) {
	h := sha256.New()
	h.Write([]byte("gate/1\x00"))
	h.Write([]byte(slgShaFile(slgPyJoin(root, slgGateScriptRel))))
	ident := ""
	zerg := slgPyJoin(root, "bin", "zerg")
	if slgIsFile(zerg) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		cmd := exec.CommandContext(ctx, zerg, "version")
		out, err := cmd.Output()
		cancel()
		if err == nil {
			ident = strings.Join(strings.Fields(string(out)), " ")
			if len([]rune(ident)) > 200 {
				ident = string([]rune(ident)[:200])
			}
		}
	}
	h.Write([]byte("\x00" + ident))
	return hex.EncodeToString(h.Sum(nil)), ident
}

// slgRow —— `results.tsv` 一行（五格）。
type slgRow struct{ Status, Step, RC, Secs, Log string }

// slgReadResults —— py `read_results`：≥5 格才收。
func slgReadResults(path string) []slgRow {
	text, ok := slgReadText(path)
	if !ok {
		return nil
	}
	var rows []slgRow
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimRight(ln, "\n")
		if ln == "" {
			continue
		}
		c := strings.Split(ln, "\t")
		if len(c) < 5 {
			continue
		}
		rows = append(rows, slgRow{c[0], c[1], c[2], c[3], c[4]})
	}
	return rows
}

// slgReadFreshness —— py `read_freshness`：读不回/解不开 ⇒ 空表。
func slgReadFreshness(dir string) map[string]interface{} {
	b, err := os.ReadFile(slgPyJoin(dir, "freshness.json"))
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return map[string]interface{}{}
	}
	return m
}

// slgChainOf —— py `chain_of`（链式摘要 · 改任一格后续章全对不上）。
func slgChainOf(prevChain string, rec map[string]interface{}) string {
	h := sha256.New()
	h.Write([]byte(slgChainSeed + "\x00"))
	h.Write([]byte(prevChain + "\x00"))
	for _, k := range slgChainKeys {
		h.Write([]byte(k + "=" + slgChainGet(rec, k) + "\x00"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// slgChainGet —— `rec.get(k, "")` 的逐字同构（缺键 ⇒ ""、在场 None ⇒ "None"）。
func slgChainGet(rec map[string]interface{}, k string) string {
	v, ok := rec[k]
	if !ok {
		return ""
	}
	return slgPyStr(v)
}

// slgPyStr —— py `str(v)` / f-string 的常用档。
func slgPyStr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// slgToInt —— py `int(v)` 的常用档（float64 / 十进制串 / 缺省）。
func slgToInt(v interface{}, def int) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		n := 0
		ok := false
		for _, r := range t {
			if r < '0' || r > '9' {
				if n == 0 && !ok {
					return def
				}
				break
			}
			n = n*10 + int(r-'0')
			ok = true
		}
		if !ok {
			return def
		}
		return n
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
	}
	return def
}

// slgTruthy —— py 真值档（缺键/空串/0/None/空集 ⇒ 假）。
func slgTruthy(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return t != ""
	case bool:
		return t
	case float64:
		return t != 0
	case []interface{}:
		return len(t) > 0
	case map[string]interface{}:
		return len(t) > 0
	}
	return true
}

// slgLastChain —— py `last_chain`：回 (末枚 chain, 章数)。
func slgLastChain(ledger string) (string, int) {
	text, ok := slgReadText(ledger)
	if !ok {
		return "", 0
	}
	prev, n := "", 0
	for _, ln := range strings.Split(text, "\n") {
		ln = slgPyStrip(ln)
		if ln == "" {
			continue
		}
		var r map[string]interface{}
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			continue
		}
		if v, ok := r["chain"]; ok {
			prev = slgPyStr(v)
		} else {
			prev = ""
		}
		n = slgToInt(r["chapter_no"], 0)
	}
	return prev, n
}

// slgLoadLedger —— py `load_ledger`：解不开的行收成 `{"_unparseable": 前 80 码点}`。
func slgLoadLedger(ledger string) []map[string]interface{} {
	text, ok := slgReadText(ledger)
	if !ok {
		return nil
	}
	var out []map[string]interface{}
	for _, ln := range strings.Split(text, "\n") {
		ln = slgPyStrip(ln)
		if ln == "" {
			continue
		}
		var r map[string]interface{}
		if err := json.Unmarshal([]byte(ln), &r); err != nil || r == nil {
			r = map[string]interface{}{"_unparseable": slgRuneSlice(ln, 80)}
		}
		out = append(out, r)
	}
	return out
}

func slgRuneSlice(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// slgStepCmds —— py `step_cmds`：门件自报的现读步集 + 各步命令串。ok=false ⇒ 判不了。
func slgStepCmds(root string) ([]string, map[string]string, bool) {
	script := slgPyJoin(root, slgGateScriptRel)
	if !slgIsFile(script) {
		return nil, nil, false
	}
	// ★ GAP-20261010-25（本片）：机读面优先（`--list --json`）—— 复用同族助手 `pgsParseListJSON`。
	// 机读面在场而形状坏 ⇒ **判不了**（ok=false · 不给结论），✗ 不许静默退回人面。
	{
		mcmd := armChildGroup(exec.Command("bash", script, "--list", "--json"))
		runningChild = mcmd
		defer func() { runningChild = nil }()
		mtimer := time.AfterFunc(180*time.Second, func() { terminateChildTree(mcmd) })
		mout, merr := mcmd.Output()
		mtimer.Stop()
		if merr == nil {
			if steps, _, isM, perr := pgsParseListJSON(string(mout)); isM {
				if perr != "" {
					return nil, nil, false
				}
				var mnames []string
				for _, s := range steps {
					if s.Name != "" {
						mnames = append(mnames, s.Name)
					}
				}
				if len(mnames) == 0 {
					return nil, nil, false
				}
				return mnames, slgResolveCmds(root, mnames), true
			}
		}
	}
	// ② 退化路：机读面不在 ⇒ 人面 `--list`（与改前逐字相同）。
	cmd := armChildGroup(exec.Command("bash", script, "--list"))
	runningChild = cmd
	defer func() { runningChild = nil }()
	ctimer := time.AfterFunc(180*time.Second, func() { terminateChildTree(cmd) })
	out, err := cmd.Output()
	ctimer.Stop()
	if err != nil {
		return nil, nil, false
	}
	var names []string
	for _, ln := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(ln, "   ") {
			continue
		}
		rest := strings.TrimLeft(ln, " ")
		i := strings.Index(rest, " ")
		if i < 0 {
			continue
		}
		rest = strings.TrimLeft(rest[i:], " ")
		j := strings.Index(rest, " ")
		if j < 0 {
			continue
		}
		nm := strings.TrimLeft(rest[j:], " ")
		if nm != "" {
			names = append(names, nm)
		}
	}
	if len(names) == 0 {
		return nil, nil, false
	}
	return names, slgResolveCmds(root, names), true
}

// slgResolveCmds —— 逐名取命令串（`<root>/bin/zerg gate show <步名> --json cmd`）。
// ★ GAP-20261010-25（本片）：自机读面/人面两条退化路**共用这一支**（✗ 不各写一份）。
func slgResolveCmds(root string, names []string) map[string]string {
	cmds := map[string]string{}
	zerg := slgPyJoin(root, "bin", "zerg")
	if slgIsFile(zerg) {
		for _, nm := range names {
			c2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
			q := exec.CommandContext(c2, zerg, "gate", "show", nm, "--json", "cmd")
			o, err := q.Output()
			cancel2()
			if err == nil {
				cmds[nm] = strings.Join(strings.Fields(string(o)), " ")
			}
		}
	}
	return cmds
}

// ────────────────────────────────────────────────────────────────────────────
// JSON 序列化（py `json.dumps(..., ensure_ascii=False)` 的逐字同构）
// ────────────────────────────────────────────────────────────────────────────

type slgPair struct{ k, v string }

func slgJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
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
			if r < 0x20 {
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func slgJSONObj(pairs []slgPair) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, p := range pairs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(slgJSONStr(p.k))
		b.WriteString(": ")
		b.WriteString(p.v)
	}
	b.WriteByte('}')
	return b.String()
}

func slgJSONArrStr(items []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(slgJSONStr(s))
	}
	b.WriteByte(']')
	return b.String()
}

// slgJSONVal —— 任意 JSON 值（供核章机读面）。
func slgJSONVal(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return slgJSONStr(t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	case []interface{}:
		var b strings.Builder
		b.WriteByte('[')
		for i, x := range t {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(slgJSONVal(x))
		}
		b.WriteByte(']')
		return b.String()
	default:
		return slgJSONStr(slgPyStr(v))
	}
}

func slgJSONArrVal(items []interface{}) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(slgJSONVal(x))
	}
	b.WriteByte(']')
	return b.String()
}

// ────────────────────────────────────────────────────────────────────────────
// 盖章（写 · 唯一写通道 · append-only）
// ────────────────────────────────────────────────────────────────────────────

func slgNowUTC(override string) string {
	if override != "" {
		return override
	}
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func slgDoSeal(stdout, stderr io.Writer, root, runDir, ledger string, write bool, reason string, nowOverride string) int {
	rt := slgPyJoin(runDir, "results.tsv")
	if !slgIsFile(rt) {
		return slgDie(stderr, "✗ `--seal "+runDir+"` 里没有 results.tsv ⇒ 不是一趟门跑产物 ⇒ 不给结论(2)", slgBLOCKEDRC)
	}
	rows := slgReadResults(rt)
	if len(rows) == 0 {
		return slgDie(stderr, "✗ `"+rt+"` 一行都读不出 ⇒ 判不了(2)", slgBLOCKEDRC)
	}
	fp := slgReadFreshness(runDir)
	isd, changed := slgItemSetDigest(root, slgLedgerRelOf(root, ledger))
	gv, gident := slgGateVersion(root)
	_, cmds, _ := slgStepCmds(root)
	_ = cmds
	prev, n := slgLastChain(ledger)
	wrote := 0
	var lines []string
	for _, r := range rows {
		n++
		rec := map[string]interface{}{
			"chapter_no": n, "at": slgNowUTC(nowOverride),
			"step": r.Step, "status": r.Status, "rc": r.RC, "secs": r.Secs, "log": r.Log,
			"worktree_fp":     slgDefStr(fp, "worktree_fp"),
			"head":            slgDefStr(fp, "head"),
			"item_set_digest": isd,
			"judge_ver":       slgJudgeVersion(root, cmdOf(cmds, r.Step)),
			"gate_ver":        gv,
			"executed":        true,
		}
		chain := slgChainOf(prev, rec)
		rec["chain"] = chain
		prev = chain
		pairs := []slgPair{
			{"chapter_no", fmt.Sprintf("%d", n)},
			{"at", slgJSONStr(slgPyStr(rec["at"]))},
			{"step", slgJSONStr(r.Step)},
			{"status", slgJSONStr(r.Status)},
			{"rc", slgJSONStr(r.RC)},
			{"secs", slgJSONStr(r.Secs)},
			{"log", slgJSONStr(r.Log)},
			{"worktree_fp", slgJSONStr(slgPyStr(rec["worktree_fp"]))},
			{"head", slgJSONStr(slgPyStr(rec["head"]))},
			{"item_set_digest", slgJSONStr(isd)},
			{"judge_ver", slgJSONStr(slgPyStr(rec["judge_ver"]))},
			{"gate_ver", slgJSONStr(gv)},
			{"executed", "true"},
			{"chain", slgJSONStr(chain)},
		}
		lines = append(lines, slgJSONObj(pairs))
		wrote++
	}
	steps := make([]string, 0, len(rows))
	for _, r := range rows {
		steps = append(steps, slgJSONStr(r.Step))
	}
	coverNo := n + 1
	if coverNo < 1 {
		coverNo = 1
	}
	cover := []slgPair{
		{"chapter_no", fmt.Sprintf("%d", coverNo)},
		{"kind", slgJSONStr("cover")},
		{"run_dir", slgJSONStr(slgPyAbspath(runDir))},
		{"steps", "[" + strings.Join(steps, ", ") + "]"},
		{"n", fmt.Sprintf("%d", wrote)},
		{"baseline_chain", slgJSONStr(prev)},
		{"gate_ver", slgJSONStr(gv)},
		{"gate_ident", slgJSONStr(gident)},
		{"at", slgJSONStr(slgNowUTC(nowOverride))},
		{"executed", "true"},
	}
	lines = append(lines, slgJSONObj(cover))
	if !write {
		fmt.Fprintf(stdout, "[试跑] 将盖章 %d 枚（未落盘 · 加 --write 才写）\n", wrote)
		fmt.Fprintf(stdout, "  件集内容摘要 %s · 改动面 %d 件 · 门版本 %s %s\n",
			slgRuneSlice(isd, 16), len(changed), slgRuneSlice(gv, 16), gident)
		return 0
	}
	if reason == "" {
		return slgDie(stderr, "✗ --seal --write 必须带 --reason（唯一写通道要留痕）⇒ 不给结论(2)", slgBLOCKEDRC)
	}
	f, err := os.OpenFile(ledger, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return slgDie(stderr, "✗ 章账写不回："+ledger, slgBLOCKEDRC)
	}
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		f.Close()
		return slgDie(stderr, "✗ 章账写不回："+ledger, slgBLOCKEDRC)
	}
	f.Close()
	fmt.Fprintf(stdout, "✓ 已盖 %d 枚章（落点 %s）· 件集内容摘要 %s · 改动面 %d 件\n",
		wrote, ledger, slgRuneSlice(isd, 16), len(changed))
	return 0
}

func cmdOf(cmds map[string]string, step string) string {
	if cmds == nil {
		return ""
	}
	return cmds[step]
}

func slgDefStr(m map[string]interface{}, k string) string {
	v, ok := m[k]
	if !ok {
		return ""
	}
	return slgPyStr(v)
}

// ────────────────────────────────────────────────────────────────────────────
// 核章（只读 · 不真跑 · 不写盘 · 不续期）
// ────────────────────────────────────────────────────────────────────────────

type slgVerdict struct {
	ChapterNo interface{}
	Step      string
	State     string
	Reasons   []string
}

func slgDoVerify(stdout, stderr io.Writer, root, ledger string, jsonOn bool) int {
	recs := slgLoadLedger(ledger)
	if len(recs) == 0 {
		return slgDie(stderr, "✗ 核章：章账 "+ledger+" 读不到任何一枚章 ⇒ **不给结论**(2)（不许当绿）", slgBLOCKEDRC)
	}
	names, cmds, ok := slgStepCmds(root)
	if !ok {
		return slgDie(stderr, "✗ 核章：读不回门件自己的步骤清单 ⇒ **不给结论**(2)", slgBLOCKEDRC)
	}
	isd, changed := slgItemSetDigest(root, slgLedgerRelOf(root, ledger))
	gv, gident := slgGateVersion(root)

	var seals, covers []map[string]interface{}
	for _, r := range recs {
		if _, bad := r["_unparseable"]; bad {
			continue
		}
		if slgPyStr(r["kind"]) == "cover" {
			covers = append(covers, r)
		} else {
			seals = append(seals, r)
		}
	}

	// 链完整：序号连续无洞 + 每枚章的 chain 逐枚复算
	var chainGap []string
	prev, expect := "", 1
	for _, r := range seals {
		if slgToInt(r["chapter_no"], -1) != expect {
			chainGap = append(chainGap, fmt.Sprintf("序号有洞：期望 %d 得 %s", expect, slgPyStr(r["chapter_no"])))
		}
		if slgChainOf(prev, r) != slgChainGet(r, "chain") {
			chainGap = append(chainGap, fmt.Sprintf("章 #%s 链摘要对不上（**历史被改写 / 手工续期**）", slgPyStr(r["chapter_no"])))
		}
		prev = slgChainGet(r, "chain")
		expect++
	}

	// 覆盖：章盖的步集 vs 现读步集
	var sealedSteps []string
	for _, c := range covers {
		if arr, ok := c["steps"].([]interface{}); ok {
			for _, s := range arr {
				sealedSteps = append(sealedSteps, slgPyStr(s))
			}
		}
	}
	sealedSet := map[string]bool{}
	for _, s := range sealedSteps {
		sealedSet[s] = true
	}
	curSet := map[string]bool{}
	for _, s := range names {
		curSet[s] = true
	}
	var missing, extra []string
	for s := range curSet {
		if !sealedSet[s] {
			missing = append(missing, s)
		}
	}
	for s := range sealedSet {
		if !curSet[s] {
			extra = append(extra, s)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	var verdicts []slgVerdict
	nStale, nGap := 0, 0
	for _, r := range seals {
		step := slgPyStr(r["step"])
		var reasons []string
		if !slgTruthy(r["item_set_digest"]) {
			reasons = append(reasons, "缺格：件集内容摘要为空 ⇒ 章无效（硬前提①）")
		}
		if !slgTruthy(r["judge_ver"]) {
			reasons = append(reasons, "缺格：判据件自身版本为空 ⇒ 章无效（硬前提①）")
		}
		if !slgTruthy(r["gate_ver"]) {
			reasons = append(reasons, "缺格：门版本为空 ⇒ 章无效")
		}
		if slgTruthy(r["item_set_digest"]) && slgPyStr(r["item_set_digest"]) != isd {
			reasons = append(reasons, fmt.Sprintf("件集内容摘要不符（章 %s ≠ 现读 %s）",
				slgRuneSlice(slgPyStr(r["item_set_digest"]), 12), slgRuneSlice(isd, 12)))
		}
		if slgTruthy(r["judge_ver"]) {
			if _, in := cmds[step]; in {
				if slgPyStr(r["judge_ver"]) != slgJudgeVersion(root, cmdOf(cmds, step)) {
					reasons = append(reasons, "判据件自身版本变（判据换了，旧结论不算数）")
				}
			}
		}
		if slgTruthy(r["gate_ver"]) && slgPyStr(r["gate_ver"]) != gv {
			reasons = append(reasons, "门版本变")
		}
		if !curSet[step] {
			reasons = append(reasons, "gap：该步已不在现读步集（增删改名）")
		}
		st := slgSealStates[0]
		if len(reasons) > 0 {
			st = "stale"
			for _, x := range reasons {
				if strings.HasPrefix(x, "gap") || strings.HasPrefix(x, "缺格") {
					st = "gap"
					break
				}
			}
		}
		if st == "gap" {
			nGap++
		} else if st == "stale" {
			nStale++
		}
		verdicts = append(verdicts, slgVerdict{r["chapter_no"], step, st, reasons})
	}

	badRender := 0
	for _, v := range verdicts {
		found := false
		for _, s := range slgSealStates {
			if v.State == s {
				found = true
				break
			}
		}
		if !found {
			badRender++
		}
	}
	nExec := 0 // executed 恒 False ⇒ 计数恒 0
	head := "未真跑 · 结论由章支持（**有不符项**）"
	if len(chainGap) == 0 && len(missing) == 0 && len(extra) == 0 && nStale == 0 && nGap == 0 {
		head = "未真跑 · 结论由章支持"
	}
	fmt.Fprintf(stdout, "── 核章档（第四档）· %s ──\n", head)
	fmt.Fprintf(stdout, "── 档位链：日常快速档 ⊂ 核章档 ⊂ 中档 ⊂ 盖章档（真源 %s 的 `chain`）──\n",
		slgPyRelpath(slgPyJoin(root, "scripts", "gates", slgBaselineName), root))
	fmt.Fprintf(stdout, "── 章账 %s · 章 %d 枚 · 覆盖表 %d 张 · executed=true 计数 %d（**必须 0**）· 现读步集 %d 步 ──\n",
		ledger, len(seals), len(covers), nExec, len(names))
	fmt.Fprintf(stdout, "── 件集内容摘要 %s · 改动面 %d 件 · 门版本 %s %s ──\n",
		slgRuneSlice(isd, 16), len(changed), slgRuneSlice(gv, 16), gident)
	if len(chainGap) > 0 {
		fmt.Fprint(stdout, "── 链完整性：**不过** ──\n")
		for _, g := range chainGap {
			fmt.Fprintf(stdout, "   ✗ %s\n", g)
		}
	} else {
		fmt.Fprintf(stdout, "── 链完整性：✓ 序号 1..%d 连续 · 逐枚链摘要复算一致 ──\n", len(seals))
	}
	if len(missing) > 0 || len(extra) > 0 {
		fmt.Fprintf(stdout, "── 覆盖：**gap** —— 现读有而章无 %d 步 · 章有而现读无 %d 步 ──\n", len(missing), len(extra))
		for _, s := range missing {
			fmt.Fprintf(stdout, "   gap 缺步：%s\n", s)
		}
		for _, s := range extra {
			fmt.Fprintf(stdout, "   gap 多步：%s\n", s)
		}
	} else {
		fmt.Fprint(stdout, "── 覆盖：✓ 章盖的步集 == 现读步集（步集不缩）──\n")
	}
	for _, v := range verdicts {
		fmt.Fprintf(stdout, "   [%s] #%s %s\n", v.State, slgPyStr(v.ChapterNo), v.Step)
		for _, rs := range v.Reasons {
			fmt.Fprintf(stdout, "        ✗ %s\n", rs)
		}
	}
	bad := len(chainGap) > 0 || len(missing) > 0 || len(extra) > 0 || nStale > 0 || nGap > 0 || badRender > 0 || nExec > 0
	if jsonOn {
		var varr []string
		for _, v := range verdicts {
			rs := make([]interface{}, 0, len(v.Reasons))
			for _, x := range v.Reasons {
				rs = append(rs, x)
			}
			varr = append(varr, slgJSONObj([]slgPair{
				{"chapter_no", slgJSONVal(v.ChapterNo)},
				{"step", slgJSONStr(v.Step)},
				{"state", slgJSONStr(v.State)},
				{"executed", "false"},
				{"reasons", slgJSONArrVal(rs)},
			}))
		}
		fmt.Fprintln(stdout, slgJSONObj([]slgPair{
			{"kind", slgJSONStr("SealVerify")},
			{"state", slgJSONStr(head)},
			{"seals", fmt.Sprintf("%d", len(seals))},
			{"executed_true", fmt.Sprintf("%d", nExec)},
			{"stale", fmt.Sprintf("%d", nStale)},
			{"gap", fmt.Sprintf("%d", nGap)},
			{"chain_gap", slgSliceStrJSON(chainGap)},
			{"missing", slgSliceStrJSON(missing)},
			{"extra", slgSliceStrJSON(extra)},
			{"item_set_digest", slgJSONStr(isd)},
			{"gate_ver", slgJSONStr(gv)},
			{"verdicts", "[" + strings.Join(varr, ", ") + "]"},
		}))
	}
	if bad {
		fmt.Fprint(stdout, "⇒ 核章：**有不符项**（rc=1）—— 过期**不等于**重跑全量：只补跑该章覆盖的那几步\n")
		return slgJudgeRC
	}
	fmt.Fprint(stdout, "⇒ 核章：章链有效（rc=0）· **未真跑任何一步**（executed=true == 0）\n")
	return 0
}

func slgSliceStrJSON(items []string) string {
	if items == nil {
		items = []string{}
	}
	return slgJSONArrStr(items)
}

// ────────────────────────────────────────────────────────────────────────────
// 自检（成对正负控 · 合成夹具 · 不碰真目标）
// ────────────────────────────────────────────────────────────────────────────

func slgSelfTest(stdout, stderr io.Writer) int {
	tmp, err := os.MkdirTemp("", "seal-ledger-selftest.")
	if err != nil {
		fmt.Fprintln(stderr, "✗ 自检：建不了临时夹具")
		return slgBLOCKEDRC
	}
	defer os.RemoveAll(tmp)
	bad := 0
	chk := func(what string, ok bool, extra string) {
		line := "   ✓ "
		if !ok {
			line = "   ✗ "
		}
		line += what
		if !ok && extra != "" {
			line += "（" + extra + "）"
		}
		fmt.Fprintln(stdout, line)
		if !ok {
			bad++
		}
	}

	repo := slgPyJoin(tmp, "repo")
	if err := os.MkdirAll(slgPyJoin(repo, "scripts", "gates"), 0o755); err != nil {
		fmt.Fprintln(stderr, "✗ 自检：建不了夹具")
		return slgBLOCKEDRC
	}
	// 夹具门件（自报一条步）
	_ = os.WriteFile(slgPyJoin(repo, "scripts", "gates", "precommit-gates.sh"),
		[]byte("#!/bin/bash\necho '   1     tri    步甲'\n"), 0o644)
	slgRunQuiet("git", "init", "-q", repo)
	_ = os.WriteFile(slgPyJoin(repo, "judge.py"), []byte("print('judge')\n"), 0o644)
	slgRunQuiet("git", "-C", repo, "add", "-A")

	run := slgPyJoin(tmp, "zerg-gates-fake")
	_ = os.MkdirAll(run, 0o755)
	_ = os.WriteFile(slgPyJoin(run, "results.tsv"),
		[]byte("PASS\t步甲\t0\t1s\t/nowhere/a.log\n"), 0o644)
	fb, _ := json.Marshal(map[string]interface{}{"head": "abc1234", "worktree_fp": strings.Repeat("f", 64)})
	_ = os.WriteFile(slgPyJoin(run, "freshness.json"), fb, 0o644)
	ledger := slgPyJoin(tmp, "seal.jsonl")

	// 正控 1：盖章（--write --reason）落盘
	var so, se strings.Builder
	rc := slgDoSeal(&so, &se, repo, run, ledger, true, "自检", "")
	chk("正控 1：--seal --write --reason ⇒ rc=0 且落盘",
		rc == 0 && slgIsFile(ledger), slgRuneSlice(se.String(), 200))

	// 负控 2：--seal --write 不带 --reason ⇒ rc=2（唯一写通道要留痕）
	so.Reset()
	se.Reset()
	rc = slgDoSeal(&so, &se, repo, run, ledger, true, "", "")
	chk("负控 2：--seal --write 缺 --reason ⇒ rc=2", rc == slgBLOCKEDRC, fmt.Sprintf("rc=%d", rc))

	// 核章输出必须带「未真跑 · 结论由章支持」且逐字不含 PASS
	so.Reset()
	se.Reset()
	rc = slgDoVerify(&so, &se, repo, ledger, false)
	out := so.String()
	_ = rc
	chk("正控 3：核章输出带「未真跑 · 结论由章支持」",
		strings.Contains(out, "未真跑 · 结论由章支持"), "")
	chk("负控 4（守门）：核章输出**逐字不含** `PASS`/`通过`",
		!strings.Contains(out, "PASS") && !strings.Contains(out, "通过"), "")
	chk("正控 5：断言 executed=true 计数 == 0 现印",
		strings.Contains(out, "executed=true 计数 0"), slgRuneSlice(out, 200))
	chk("正控 6：核章的步状态落在三态闭集里",
		slgHasLineContaining(out, "未真跑（核章）"), "")

	// 负控 7：改一个被盖住的件 ⇒ 件集内容摘要变 ⇒ 必须报 stale（不是全绿）
	_ = os.WriteFile(slgPyJoin(repo, "judge.py"), []byte("print('judgeCHANGED')\n"), 0o644)
	so.Reset()
	se.Reset()
	rc = slgDoVerify(&so, &se, repo, ledger, false)
	out7 := so.String()
	chk("负控 7：改动面变 ⇒ 核章报 stale 且 rc=1（**不许全绿**）",
		rc == slgJudgeRC && strings.Contains(out7, "stale"), fmt.Sprintf("rc=%d", rc))

	// 负控 8：手工续期（改章里一格）⇒ 链摘要对不上 ⇒ 报 gap
	recs := slgLoadRawLines(ledger)
	if len(recs) > 0 {
		recs[0]["status"] = "FORGED"
	}
	slgWriteRawLines(ledger, recs)
	so.Reset()
	se.Reset()
	rc = slgDoVerify(&so, &se, repo, ledger, false)
	out8 := so.String()
	chk("负控 8：手工改章一格 ⇒ 链摘要对不上 ⇒ 报 gap 且 rc=1",
		rc == slgJudgeRC && strings.Contains(out8, "链摘要对不上"), fmt.Sprintf("rc=%d", rc))

	// 负控 9：缺格 ⇒ 章无效
	var recs2 []map[string]interface{}
	if len(recs) > 0 {
		c := map[string]interface{}{}
		for k, v := range recs[0] {
			c[k] = v
		}
		c["item_set_digest"] = ""
		recs2 = append(recs2, c)
	}
	led2 := slgPyJoin(tmp, "seal2.jsonl")
	slgWriteRawLines(led2, recs2)
	so.Reset()
	se.Reset()
	rc = slgDoVerify(&so, &se, repo, led2, false)
	out9 := so.String()
	chk("负控 9：章缺格（件集内容摘要为空）⇒ 判无效（rc=1）",
		rc == slgJudgeRC && strings.Contains(out9, "缺格"), fmt.Sprintf("rc=%d", rc))

	// 负控 10：章账读不到 ⇒ rc=2 不给结论
	so.Reset()
	se.Reset()
	rc = slgDoVerify(&so, &se, repo, slgPyJoin(tmp, "none.jsonl"), false)
	chk("负控 10：章账空 ⇒ rc=2（不给结论 · 不许当绿）", rc == slgBLOCKEDRC, fmt.Sprintf("rc=%d", rc))

	fmt.Fprintln(stdout, "── seal-ledger.py 自检（成对正负控 · 真退出码）──")
	if bad == 0 {
		fmt.Fprintln(stdout, "自检结论: 全过")
		return 0
	}
	fmt.Fprintf(stdout, "自检结论: **不过**（%d 条）\n", bad)
	return slgSelfTestFailRC
}

func slgHasLineContaining(s, sub string) bool {
	for _, ln := range strings.Split(s, "\n") {
		if strings.Contains(ln, sub) {
			return true
		}
	}
	return false
}

func slgRunQuiet(name string, args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, name, args...).Run()
}

func slgLoadRawLines(path string) []map[string]interface{} {
	text, ok := slgReadText(path)
	if !ok {
		return nil
	}
	var out []map[string]interface{}
	for _, ln := range strings.Split(text, "\n") {
		if slgPyStrip(ln) == "" {
			continue
		}
		var r map[string]interface{}
		if err := json.Unmarshal([]byte(ln), &r); err != nil || r == nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

func slgWriteRawLines(path string, recs []map[string]interface{}) {
	var b strings.Builder
	for _, r := range recs {
		enc, err := json.Marshal(r)
		if err != nil {
			continue
		}
		b.Write(enc)
		b.WriteByte('\n')
	}
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
}

// ────────────────────────────────────────────────────────────────────────────
// 入口（= py `main()`）
// ────────────────────────────────────────────────────────────────────────────

func slgRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepathJoin3(d, "core", "cmd", "zerg", "main.go")); err == nil {
			return d
		}
		parent := slgPyDirname(d)
		if parent == "" || parent == d {
			break
		}
		d = parent
	}
	return wd
}

// filepathJoin3 —— 内部只用于「探根」这种本机路径（❌ 不进任何判据输出）。
func filepathJoin3(parts ...string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		if strings.HasSuffix(out, "/") {
			out += p
		} else {
			out += "/" + p
		}
	}
	return out
}

func slgUsageText() string {
	return "usage: seal-ledger.py [--seal SEAL] [--verify] [--check] [--ledger LEDGER] " +
		"[--write] [--reason REASON] [--json JSON]\n"
}

const slgUsageDieMsg = "✗ 用法：`--seal <趟目录> [--write --reason <一句>]` ｜ `--verify` ｜ `--check`（三选一）"

// slgArgparseUsage —— argparse 的**短用法**正文（未知参数 / 缺值时先印它 · 逐字对拍 py argparse：
// `-h` 帮助面之外的这条）。宽 78 折行、续行缩进 22 空格 —— 与 py `print_usage` 输出逐字节同。
const slgArgparseUsage = "usage: seal-ledger.py [-h] [--seal SEAL] [--verify] [--check]\n" +
	"                      [--ledger LEDGER] [--write] [--reason REASON]\n" +
	"                      [--json JSON]\n"

// slgUsageErr —— argparse 口径的用法错正文：短用法 + `<prog>: error: <msg>`（rc=2）。
func slgUsageErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, slgArgparseUsage+"seal-ledger.py: error: "+msg+"\n")
	return slgBLOCKEDRC
}

// sealLedgerGateMain —— 入口（= py `main()`）。argv 与 py `sys.argv[1:]` 同形。
func sealLedgerGateMain(stdout, stderr io.Writer, argv []string) int {
	sealVal, hasSeal := "", false
	verify, check, write := false, false, false
	ledger, hasLedger := "", false
	reason := ""
	jsonVal, hasJSON := "", false
	nowOverride := ""
	help := false

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			help = true
		case a == "--seal":
			if i+1 >= len(argv) {
				return slgUsageErr(stderr, "argument --seal: expected one argument")
			}
			i++
			sealVal, hasSeal = argv[i], true
		case strings.HasPrefix(a, "--seal="):
			sealVal, hasSeal = a[len("--seal="):], true
		case a == "--ledger":
			if i+1 >= len(argv) {
				return slgUsageErr(stderr, "argument --ledger: expected one argument")
			}
			i++
			ledger, hasLedger = argv[i], true
		case strings.HasPrefix(a, "--ledger="):
			ledger, hasLedger = a[len("--ledger="):], true
		case a == "--reason":
			if i+1 >= len(argv) {
				return slgUsageErr(stderr, "argument --reason: expected one argument")
			}
			i++
			reason = argv[i]
		case strings.HasPrefix(a, "--reason="):
			reason = a[len("--reason="):]
		case a == "--json":
			if i+1 >= len(argv) {
				return slgUsageErr(stderr, "argument --json: expected one argument")
			}
			i++
			jsonVal, hasJSON = argv[i], true
		case strings.HasPrefix(a, "--json="):
			jsonVal, hasJSON = a[len("--json="):], true
		case a == "--now":
			if i+1 >= len(argv) {
				return slgUsageErr(stderr, "argument --now: expected one argument")
			}
			i++
			nowOverride = argv[i]
		case strings.HasPrefix(a, "--now="):
			nowOverride = a[len("--now="):]
		case a == "--verify":
			verify = true
		case a == "--check":
			check = true
		case a == "--write":
			write = true
		default:
			return slgUsageErr(stderr, "unrecognized arguments: "+a)
		}
	}
	if help {
		fmt.Fprint(stdout, slgUsageText())
		return 0
	}
	root := slgRepoRoot()
	if !hasLedger {
		ledger = slgPyJoin(root, "scripts", "gates", "seal", "ledger.jsonl")
	}
	if check {
		return slgSelfTest(stdout, stderr)
	}
	if hasSeal && sealVal != "" {
		return slgDoSeal(stdout, stderr, root, sealVal, ledger, write, reason, nowOverride)
	}
	if verify {
		_ = jsonVal
		return slgDoVerify(stdout, stderr, root, ledger, hasJSON && jsonVal != "")
	}
	fmt.Fprintln(stderr, slgUsageDieMsg)
	return slgBLOCKEDRC
}
