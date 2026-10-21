// eval_stability_stats.go —— 件 scripts/evals/stability-stats.py（580 行）的 Go 真搬
// （批-11 探针/生成器面 · 入口 evalStabilityStatsGateMain）。
//
// 为什么有这件：批-11 是「探针与生成器」档（scripts/evals 余件 · sandbox-probes · wall · calib）。
// 同档逐件现读后，本件是该档里唯一「纯静态只读」件 —— 其余：sandbox-probes 的 conn_jit/jitprobe/
// miniprobe/lx2-linux 自带 socket|mmap-RWX|ctypes，egg-prototype-pack 无 CLI 入口（只三枚函数），
// negctl-two-states.sh 真改仓内件（mv 探针件）且起 py 子进程，evals 的 compare-wall-argv/make-manifest/
// v25_* 一律真跑子进程，wall 的 *-mutate 真改仓内件并与 cargo 交互，calib 三件真跑子进程并联网。
//
// 本件真身速览（逐处现读 scripts/evals/stability-stats.py · 非按件名推断）：
//
//	· 形态 = 纯静态只读：只 import argparse/json/math/sys（全标准库），零 subprocess / 零联网 / 零真 TTY；
//	  tempfile 仅出现在 self-test 的内存输入助手（且 mkstemp + unlink 自回收）。默认档不写任何盘。
//	· 核心 = 二项 CDF（lgamma 对数式 + fsum 精确求和）＋ Clopper-Pearson 单侧 95% 上/下界（二分 200 步）。
//	· 输入 = JSONL（每行一观测）；字段名可用 --group-field/--hash-field/--diverged-field 改写，
//	  指纹字段自动认 vector_hash/sig/hash/sha256/fingerprint/vector，分歧字段认 diverged/changed/unstable，
//	  反向字段认 same/identical/same_as_reference。
//	· 输出 = render_text（人读）或 --json（机器）。
//	· 退码 = 0 正常 / 2 硬失败（自检红 · 输入非法 · 无输入 · argparse 错）；py 顶层 sys.exit(main())。
//
// 出厂声明 (a) 默认根定位差异：本件不依赖 __file__（不吃仓外树、不按脚本位置上溯），输入一律走 argv/显式
// 路径 ⇒ 该差异面**不适用**，如实声明。
// 出厂声明 (b) 自指检查：本件落在 core/cmd/zerg 全件扫描面内 ⇒ 件内零绝对路径字面量、零形如「引号+键名+引号」
// 的协议原串（JSON 键名走输出构造，非件头协议字面量），仓内相对路径仅作非引号注释出现（复核 0/0，见回执）。
//
// 本单边界（照实）：本件已接线（inproc 表分派） ——
//
//	· 不在 core/cmd/zerg/main.go 注册命令（✗ 不进命令树 · `add_step` **未挂**（`precommit-gates.sh` 现读零命中））；
//	· 不删 scripts/evals/stability-stats.py 原件（退场：对拍通过后才逐件点名退场）；
//	· **已接线**：入口 `evalStabilityStatsGateMain` 由 `gateCheckGoEntry["stability-stats"]` 分派（`zerg gate check stability-stats`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//	· ✗ 不改任何登记面 / family_*.go / 已落旧件。
//
// 入口签名（与 py 的 main() 对齐）：
//
//	func evalStabilityStatsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py sys.argv[1:] 同形（不含程序名）
//	  · 退码 —— 同 py：0 正常 / 2 硬失败
//	  · 输出 —— stdout/stderr 与 py 逐字节对齐（见回执「已对拍面 / 未覆盖面」）
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

const essAlpha = 0.05

var essTiers = []float64{0.50, 0.15, 0.10, 0.05, 0.02, 0.01, 0.005}
var essHashFields = []string{"vector_hash", "sig", "hash", "sha256", "fingerprint", "vector"}
var essDivergedFields = []string{"diverged", "changed", "unstable"}
var essReversedFields = []string{"same", "identical", "same_as_reference"}

// essStdin 是 stdin 注入点（读面走 "-" 时用；测试件替换它）。
var essStdin io.Reader = os.Stdin

// essRunSelfTest 是自检注入点（负控用；测试件可替换成必红桩）。
var essRunSelfTest = essRunSelfTestSafe

// essHelpText 是 argparse 帮助正文（含 usage 块），由对拍台从 py 现读逐字节对齐后嵌入。
const essHelpText = `usage: stability-stats.py [-h] [--self-test] [--group-field GROUP_FIELD]
                          [--hash-field HASH_FIELD]
                          [--diverged-field DIVERGED_FIELD]
                          [--reference GROUP=HASH] [--alpha ALPHA] [--json]
                          [--no-preflight] [--no-table]
                          [inputs ...]

分歧率 + Clopper-Pearson 单侧 95% 上界（T7.3 可复现性口径）

positional arguments:
  inputs                JSONL 观测文件（- 表示 stdin）

optional arguments:
  -h, --help            show this help message and exit
  --self-test           只跑已知答案自测
  --group-field GROUP_FIELD
  --hash-field HASH_FIELD
  --diverged-field DIVERGED_FIELD
  --reference GROUP=HASH
                        显式声明某组的参考向量
  --alpha ALPHA
  --json                输出机器可读 JSON
  --no-preflight        跳过分析前自检（不建议）
  --no-table            不打印样本量对照表
`

// essUsage 是 argparse 错误信息用的 usage 块（= 帮助正文前 6 行）。
const essUsage = `usage: stability-stats.py [-h] [--self-test] [--group-field GROUP_FIELD]
                          [--hash-field HASH_FIELD]
                          [--diverged-field DIVERGED_FIELD]
                          [--reference GROUP=HASH] [--alpha ALPHA] [--json]
                          [--no-preflight] [--no-table]
                          [inputs ...]
`

// --------------------------------------------------------------------------- //
// 核心统计：二项 CDF / Clopper-Pearson 单侧界
// --------------------------------------------------------------------------- //

// essPyFsum = CPython math.fsum（Shewchuk 精确求和；Go 无此函数，逐行直译）。
func essPyFsum(xs []float64) float64 {
	partials := []float64{}
	for _, xx := range xs {
		x := xx
		i := 0
		for _, yy := range partials {
			y := yy
			if math.Abs(x) < math.Abs(y) {
				x, y = y, x
			}
			hi := x + y
			lo := y - (hi - x)
			if lo != 0.0 {
				partials[i] = lo
				i++
			}
			x = hi
		}
		partials = append(partials[:i], x)
	}
	var s float64
	for _, p := range partials {
		s += p
	}
	return s
}

// essBinomCDF = P(X <= k)，X ~ Binomial(n, p)。
func essBinomCDF(k, n int, p float64) float64 {
	if n < 0 {
		panic("n must be >= 0")
	}
	if k < 0 {
		return 0.0
	}
	if k >= n {
		return 1.0
	}
	if p <= 0.0 {
		return 1.0
	}
	if p >= 1.0 {
		return 0.0
	}
	lp, lq := math.Log(p), math.Log1p(-p)
	terms := make([]float64, 0, k+1)
	for i := 0; i <= k; i++ {
		lg, _ := math.Lgamma(float64(n + 1))
		lg2, _ := math.Lgamma(float64(i + 1))
		lg3, _ := math.Lgamma(float64(n - i + 1))
		terms = append(terms, lg-lg2-lg3+float64(i)*lp+float64(n-i)*lq)
	}
	m := terms[0]
	for _, t := range terms {
		if t > m {
			m = t
		}
	}
	exps := make([]float64, 0, len(terms))
	for _, t := range terms {
		exps = append(exps, math.Exp(t-m))
	}
	val := math.Exp(m) * essPyFsum(exps)
	return math.Min(1.0, math.Max(0.0, val))
}

func essCheckKN(k, n int) {
	if n <= 0 {
		panic("n 必须是正整数")
	}
	if k < 0 || k > n {
		panic("k 必须满足 0 <= k <= n")
	}
}

// essCPUpper = 分歧率 p 的单侧 1-alpha 上界（Clopper-Pearson 精确）。
func essCPUpper(k, n int, alpha float64) float64 {
	essCheckKN(k, n)
	if k >= n {
		return 1.0
	}
	if k == 0 {
		return 1.0 - math.Pow(alpha, 1.0/float64(n))
	}
	lo, hi := 0.0, 1.0
	for i := 0; i < 200; i++ {
		mid := 0.5 * (lo + hi)
		if essBinomCDF(k, n, mid) > alpha {
			lo = mid
		} else {
			hi = mid
		}
	}
	return 0.5 * (lo + hi)
}

// essCPLower = 分歧率 p 的单侧 1-alpha 下界。
func essCPLower(k, n int, alpha float64) float64 {
	essCheckKN(k, n)
	if k <= 0 {
		return 0.0
	}
	lo, hi := 0.0, 1.0
	for i := 0; i < 200; i++ {
		mid := 0.5 * (lo + hi)
		if 1.0-essBinomCDF(k-1, n, mid) > alpha {
			hi = mid
		} else {
			lo = mid
		}
	}
	return 0.5 * (lo + hi)
}

func essRuleOfThree(n int) float64 {
	return math.Min(1.0, 3.0/float64(n))
}

// essTierFor = 能对外声称的最强档位（严格 X > u）。
func essTierFor(u float64) (float64, bool) {
	sorted := append([]float64(nil), essTiers...)
	sort.Float64s(sorted)
	for _, t := range sorted {
		if t > u {
			return t, true
		}
	}
	return 0, false
}

func essMinNForUpperBound(x, alpha float64) int {
	if !(0.0 < x && x < 1.0) {
		panic("x 必须在 (0,1)")
	}
	return int(math.Ceil(math.Log(alpha) / math.Log(1.0-x)))
}

func essMinNToDetect(pTrue, conf float64) int {
	if !(0.0 < pTrue && pTrue < 1.0) {
		panic("p_true 必须在 (0,1)")
	}
	return int(math.Ceil(math.Log(1.0-conf) / math.Log(1.0-pTrue)))
}

// essPyRound = Python round()（半偶舍入，返回 int）。
func essPyRound(x float64) int {
	fl := math.Floor(x)
	diff := x - fl
	switch {
	case diff < 0.5:
		return int(fl)
	case diff > 0.5:
		return int(fl) + 1
	default:
		if int(math.Mod(fl, 2)) == 0 {
			return int(fl)
		}
		return int(fl) + 1
	}
}

// --------------------------------------------------------------------------- //
// 输入解析
// --------------------------------------------------------------------------- //

func essPyIsSpaceRune(r rune) bool {
	switch {
	case r == ' ' || (r >= '\t' && r <= '\r'):
		return true
	case r >= 0x1c && r <= 0x1f:
		return true
	case r == 0x85 || r == 0xa0:
		return true
	case r == 0x1680 || (r >= 0x2000 && r <= 0x200a):
		return true
	case r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000:
		return true
	}
	return false
}

// essPyStrip = py str.strip()（剥 py 的 Unicode 空白集）。
func essPyStrip(s string) string {
	return strings.TrimFunc(s, essPyIsSpaceRune)
}

// essPySplitLines = py str.splitlines()（换行符见下；\r\n / \r 已由 univers 折成 \n）。
func essPySplitLines(s string) []string {
	out := []string{}
	cur := strings.Builder{}
	for _, r := range s {
		if r == '\n' || r == '\v' || r == '\f' || r == '\x1c' || r == '\x1d' ||
			r == '\x1e' || r == '\x85' || r == '\u2028' || r == '\u2029' {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// essUniversalNewlines = py 文本模式 open() 的通用换行（\r\n / \r ⇒ \n）。
func essUniversalNewlines(b []byte) string {
	s := string(b)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func essPick(rec map[string]json.RawMessage, names []string) (json.RawMessage, bool) {
	for _, nm := range names {
		if v, ok := rec[nm]; ok {
			return v, true
		}
	}
	return nil, false
}

// essRawToStr = py str(<json value>)。
func essRawToStr(raw json.RawMessage) string {
	var num json.Number
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return string(raw)
	}
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return t
	case json.Number:
		num = t
		if strings.ContainsAny(num.String(), ".eE") {
			f, err := num.Float64()
			if err == nil {
				return essPyFloat(f)
			}
		}
		return num.String()
	}
	return string(raw)
}

// essRawToBool = py bool(<json value>)。
func essRawToBool(raw json.RawMessage) bool {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return false
	}
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return len(t) > 0
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return true
		}
		return f != 0
	case []interface{}:
		return len(t) > 0
	case map[string]interface{}:
		return len(t) > 0
	}
	return false
}

type essSlot struct {
	hashes    []string
	flags     []bool
	reference string
	hasRef    bool
}

func essHasObservation(rec map[string]json.RawMessage, hashField, divergedField string) bool {
	if divergedField != "" {
		if _, ok := rec[divergedField]; ok {
			return true
		}
	}
	if hashField != "" {
		if _, ok := rec[hashField]; ok {
			return true
		}
	}
	all := append(append(append([]string{}, essHashFields...), essDivergedFields...), essReversedFields...)
	for _, nm := range all {
		if _, ok := rec[nm]; ok {
			return true
		}
	}
	return false
}

// essLoadObservations 读 JSONL ⇒ {group: slot}（保行序；与 py load_observations 同构）。
func essLoadObservations(path, text, groupField, hashField, divergedField string) (map[string]*essSlot, error) {
	groups := map[string]*essSlot{}
	for lineno, line := range essPySplitLines(text) {
		ln := lineno + 1
		line = essPyStrip(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var rec map[string]json.RawMessage
		dec := json.NewDecoder(strings.NewReader(line))
		if err := dec.Decode(&rec); err != nil {
			return nil, fmt.Errorf("%s:%d 不是合法 JSON：%s", path, ln, essPyJSONErr(line))
		}
		gRaw, ok := rec[groupField]
		g := "_all"
		if ok {
			g = essRawToStr(gRaw)
		}
		slot := groups[g]
		if slot == nil {
			slot = &essSlot{}
			groups[g] = slot
		}
		if refRaw, ok := rec["reference"]; ok {
			isNull := strings.TrimSpace(string(refRaw)) == "null"
			if !isNull {
				if !essHasObservation(rec, hashField, divergedField) {
					slot.reference = essRawToStr(refRaw)
					slot.hasRef = true
					continue
				}
			}
		}
		var hRaw json.RawMessage
		var hasH bool
		if hashField != "" {
			hRaw, hasH = rec[hashField]
		} else {
			hRaw, hasH = essPick(rec, essHashFields)
		}
		var dRaw json.RawMessage
		var hasD bool
		var revRaw json.RawMessage
		var hasRev bool
		if divergedField != "" {
			dRaw, hasD = rec[divergedField]
		} else {
			dRaw, hasD = essPick(rec, essDivergedFields)
			revRaw, hasRev = essPick(rec, essReversedFields)
		}
		if hasD && strings.TrimSpace(string(dRaw)) != "null" {
			slot.flags = append(slot.flags, essRawToBool(dRaw))
			continue
		}
		if hasRev && strings.TrimSpace(string(revRaw)) != "null" {
			slot.flags = append(slot.flags, !essRawToBool(revRaw))
			continue
		}
		if !hasH || strings.TrimSpace(string(hRaw)) == "null" {
			return nil, fmt.Errorf("%s:%d 既没有可识别的向量指纹字段，也没有 diverged 字段", path, ln)
		}
		slot.hashes = append(slot.hashes, essRawToStr(hRaw))
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("%s 里没有可用观测", path)
	}
	return groups, nil
}

// --------------------------------------------------------------------------- //
// 单组分析
// --------------------------------------------------------------------------- //

type essGroupResult struct {
	group        string
	n            int
	k            int
	unique       int
	hasUnique    bool
	reference    string
	hasReference bool
	rate         float64
	cpUpper      float64
	cpLower      float64
	ruleOfThree  float64
	hasROT       bool
	alpha        float64
	topValues    []essTopVal
	hasTop       bool
	modeCount    int
	verdict      string
}

type essTopVal struct {
	hash  string
	count int
}

func essAnalyzeGroup(name string, slot *essSlot, alpha float64, refOverride string, hasRefOverride bool) (*essGroupResult, error) {
	hashes := slot.hashes
	flags := slot.flags
	ref := ""
	hasRef := false
	if hasRefOverride {
		ref = refOverride
		hasRef = true
	} else if slot.hasRef {
		ref = slot.reference
		hasRef = true
	}
	counts := map[string]int{}
	order := []string{}
	for _, h := range hashes {
		if _, ok := counts[h]; !ok {
			order = append(order, h)
		}
		counts[h]++
	}
	if !hasRef && len(counts) > 0 {
		best := 0
		for _, c := range counts {
			if c > best {
				best = c
			}
		}
		cand := []string{}
		for _, h := range order {
			if counts[h] == best {
				cand = append(cand, h)
			}
		}
		sort.Strings(cand)
		ref = cand[0]
		hasRef = true
	}
	k := 0
	if hasRef {
		for _, h := range hashes {
			if h != ref {
				k++
			}
		}
	}
	for _, f := range flags {
		if f {
			k++
		}
	}
	n := len(hashes) + len(flags)
	if n == 0 {
		return nil, fmt.Errorf("组 %s 是空的", name)
	}
	if n > 1 && math.Log(float64(n)) > 200 {
		return nil, fmt.Errorf("组 %s 的 N 过大", name)
	}
	if k > n {
		k = n
	}
	u := essCPUpper(k, n, alpha)
	l := essCPLower(k, n, alpha)
	rate := float64(k) / float64(n)
	res := &essGroupResult{
		group: name, n: n, k: k, reference: ref, hasReference: hasRef,
		rate: rate, cpUpper: u, cpLower: l, alpha: alpha,
	}
	if len(counts) > 0 {
		res.unique = len(counts)
		res.hasUnique = true
	}
	if k == 0 {
		res.ruleOfThree = essRuleOfThree(n)
		res.hasROT = true
	}
	if len(counts) > 0 {
		type kv struct {
			h string
			c int
		}
		arr := make([]kv, 0, len(order))
		for _, h := range order {
			arr = append(arr, kv{h, counts[h]})
		}
		sort.SliceStable(arr, func(i, j int) bool {
			if arr[i].c != arr[j].c {
				return arr[i].c > arr[j].c
			}
			return arr[i].h < arr[j].h
		})
		res.hasTop = true
		lim := 3
		if len(arr) < lim {
			lim = len(arr)
		}
		for _, e := range arr[:lim] {
			res.topValues = append(res.topValues, essTopVal{e.h, e.c})
		}
		res.modeCount = arr[0].c
	}
	res.verdict = essVerdict(res)
	return res, nil
}

func essPct(x float64) string {
	return fmt.Sprintf("%.2f%%", 100.0*x)
}

func essPctTier(x float64) string {
	if x < 0.01 {
		return fmt.Sprintf("%.1f%%", 100.0*x)
	}
	return fmt.Sprintf("%d%%", essPyRound(100.0*x))
}

func essVerdict(res *essGroupResult) string {
	n, k, u, l := res.n, res.k, res.cpUpper, res.cpLower
	one := fmt.Sprintf("单侧 95%%（alpha=%.2f）", res.alpha)
	lines := []string{}
	if k == 0 {
		t, hasT := essTierFor(u)
		lines = append(lines, fmt.Sprintf(
			"N=%d、0/%d 次分歧 ⇒ 分歧率 95%% 上界 = %s（精确 CP；rule of three 3/%d = %s）",
			n, n, essPct(u), n, essPct(essRuleOfThree(n))))
		if !hasT {
			lines = append(lines, fmt.Sprintf(
				"结论：**本 N 连「分歧率 <50%%」都撑不住**（要 <50%% 需 N≥%d 精确 / N≥%d rule-of-three）"+
					" ⇒ 等于没做统计。", essMinNForUpperBound(0.50, essAlpha), int(math.Ceil(3.0/0.50))+1))
		} else {
			forbidden := []float64{}
			for _, x := range essTiers {
				if x <= u {
					forbidden = append(forbidden, x)
				}
			}
			s := fmt.Sprintf("结论：只能声称「分歧率 < %s」（由精确上界 %s 定档）", essPctTier(t), essPct(u))
			if len(forbidden) > 0 {
				maxF := forbidden[0]
				for _, x := range forbidden {
					if x > maxF {
						maxF = x
					}
				}
				need := essMinNForUpperBound(maxF, essAlpha)
				diff := need - n
				if diff < 0 {
					diff = 0
				}
				s += fmt.Sprintf("；**不能**声称「< %s」（需 N≥%d，本组还差 %d 个全一致样本）",
					essPctTier(maxF), need, diff)
			} else {
				s += "；比最严档位（<0.5%）还强，已是本工具最强档"
			}
			s += "。**不能**声称「分歧率 = 0」。"
			lines = append(lines, s)
		}
		return strings.Join(lines, "\n")
	}
	if k == n {
		lines = append(lines, fmt.Sprintf(
			"N=%d、%d/%d 次**全部分歧** ⇒ 分歧率 95%% 下界 = %s（精确 CP，= 0.05^(1/%d)）；"+
				"同时把「一致率」压到 < %s", n, k, n, essPct(l), n, essPct(1.0-l)))
		lines = append(lines, fmt.Sprintf(
			"结论：可声称「分歧率 ≥ %s」（%s）；不可对分歧率给上界（N 越大下界越高，N=59 才 ≥2.5%%）",
			essPct(l), one))
		return strings.Join(lines, "\n")
	}
	lines = append(lines, fmt.Sprintf(
		"N=%d、%d 次分歧 ⇒ 点估计 %s；CP %s 上界 = %s、下界 = %s",
		n, k, essPct(res.rate), one, essPct(u), essPct(l)))
	t, hasT := essTierFor(u)
	tierBit := ""
	if hasT {
		tierBit = fmt.Sprintf("（对外档位「< %s」）", essPctTier(t))
	}
	lines = append(lines, fmt.Sprintf(
		"结论：可声称「分歧率 < %s」%s；点估计 %s **不是**可宣传的口径（N=%d 太薄）",
		essPct(u), tierBit, essPct(res.rate), n))
	return strings.Join(lines, "\n")
}

// --------------------------------------------------------------------------- //
// 渲染（人读）
// --------------------------------------------------------------------------- //

func essRenderText(results []*essGroupResult, alpha float64, showTable bool) string {
	out := []string{}
	out = append(out, strings.Repeat("=", 78))
	out = append(out, fmt.Sprintf("稳定性统计口径 · 分歧率 + Clopper-Pearson 单侧 %.0f%% 上界（alpha=%.2f）",
		100*(1-alpha), alpha))
	out = append(out, strings.Repeat("=", 78))
	for _, r := range results {
		out = append(out, "")
		out = append(out, fmt.Sprintf("组：%s", r.group))
		bits := []string{fmt.Sprintf("N=%d", r.n), fmt.Sprintf("分歧 k=%d", r.k),
			fmt.Sprintf("点估计 %s", essPct(r.rate)),
			fmt.Sprintf("CP 上界 %s", essPct(r.cpUpper)), fmt.Sprintf("CP 下界 %s", essPct(r.cpLower))}
		if r.hasUnique {
			bits = append(bits, fmt.Sprintf("唯一值 %d", r.unique))
		}
		if r.hasReference {
			bits = append(bits, fmt.Sprintf("参考向量 %s", r.reference))
		}
		if r.hasROT {
			bits = append(bits, fmt.Sprintf("rule of three %s", essPct(r.ruleOfThree)))
		}
		out = append(out, "  "+strings.Join(bits, " · "))
		if r.hasTop {
			parts := []string{}
			for _, v := range r.topValues {
				hh := []rune(v.hash)
				if len(hh) > 24 {
					hh = hh[:24]
				}
				parts = append(parts, fmt.Sprintf("%s×%d", string(hh), v.count))
			}
			out = append(out, "  取值分布："+strings.Join(parts, " | "))
		}
		for _, line := range strings.Split(r.verdict, "\n") {
			out = append(out, "  "+line)
		}
	}
	if showTable {
		out = append(out, "")
		out = append(out, strings.Repeat("-", 78))
		out = append(out, "样本量对照表（0 分歧时能声称什么 / 要多少样本才能捕到分歧）")
		out = append(out, strings.Repeat("-", 78))
		ts := []string{}
		for _, x := range []float64{0.15, 0.10, 0.05, 0.02, 0.01, 0.005} {
			ts = append(ts, fmt.Sprintf("<%g%% 需 N≥%d", 100*x, essMinNForUpperBound(x, essAlpha)))
		}
		out = append(out, "  要声称「分歧率 <X%」(0 分歧)： "+strings.Join(ts, " · "))
		rs := []string{}
		for _, x := range []float64{0.05, 0.01} {
			rs = append(rs, fmt.Sprintf("<%g%% 需 N≥%d", 100*x, int(math.Ceil(3.0/x))))
		}
		out = append(out, "  rule of three 3/n 近似：      "+strings.Join(rs, " · "))
		ps := []string{}
		for _, p := range []float64{0.10, 0.05, 0.01} {
			ps = append(ps, fmt.Sprintf("p=%g%% 需 N≥%d", 100*p, essMinNToDetect(p, 0.95)))
		}
		out = append(out, "  真率 p 要 ≥95% 概率捕到一次： "+strings.Join(ps, " · "))
		out = append(out, "  口径铁律：N=2 的「两次一样」上界 ≈ 78%——**没有任何统计意义**。")
	}
	return strings.Join(out, "\n")
}

// --------------------------------------------------------------------------- //
// 自检（已知答案；独立参考值来自 scipy，容差 1e-9）
// --------------------------------------------------------------------------- //

var essUpCases = [][3]float64{
	{0, 6, 0.39303776899708265},
	{0, 20, 0.13910834066826516},
	{0, 21, 0.13294591102652337},
	{0, 25, 0.11292814500684320},
	{0, 40, 0.07215752450551455},
	{0, 55, 0.05301105495125376},
	{0, 59, 0.04950760988822693},
	{0, 80, 0.03675419777998230},
	{0, 300, 0.009936081944457708},
	{1, 20, 0.21610616420684730},
	{2, 20, 0.28261852488586087},
	{5, 59, 0.16996255552575096},
	{20, 40, 0.63890834000166220},
}

var essLoCases = [][3]float64{
	{1, 20, 0.0025613787765302806},
	{5, 59, 0.033987176725468960},
	{21, 21, 0.86705408897347660},
	{25, 25, 0.88707185499315680},
	{25, 30, 0.68102881805268490},
	{12, 12, 0.77907780805444420},
	{20, 40, 0.36109165999833780},
}

var essTierCases = [][2]float64{
	{0.39303777, 0.50},
	{0.13910834, 0.15},
	{0.04950761, 0.05},
	{0.00993608, 0.01},
	{0.07215753, 0.10},
	{0.77639320, -1},
}

const essTol = 1e-9

func essSelfTest() (int, []string) {
	passed := 0
	failed := []string{}
	checkF := func(name string, got, want, tol float64) {
		if math.Abs(got-want) <= tol {
			passed++
		} else {
			failed = append(failed, fmt.Sprintf("%s：期望 %v，实得 %v", name, want, got))
		}
	}
	checkI := func(name string, got, want int) {
		if got == want {
			passed++
		} else {
			failed = append(failed, fmt.Sprintf("%s：期望 %v，实得 %v", name, want, got))
		}
	}
	checkS := func(name, got, want string) {
		if got == want {
			passed++
		} else {
			failed = append(failed, fmt.Sprintf("%s：期望 %q，实得 %q", name, want, got))
		}
	}
	for _, c := range essUpCases {
		checkF(fmt.Sprintf("cp_upper(%d/%d)", int(c[0]), int(c[1])), essCPUpper(int(c[0]), int(c[1]), essAlpha), c[2], 1e-9)
	}
	for _, c := range essLoCases {
		checkF(fmt.Sprintf("cp_lower(%d/%d)", int(c[0]), int(c[1])), essCPLower(int(c[0]), int(c[1]), essAlpha), c[2], 1e-9)
	}
	for _, n := range []int{1, 2, 6, 20, 59, 300} {
		closed := 1.0 - math.Pow(essAlpha, 1.0/float64(n))
		checkF(fmt.Sprintf("0/%d 闭式 vs 数值解", n), essCPUpper(0, n, essAlpha), closed, 1e-12)
	}
	worst := math.Inf(1)
	for n := 1; n <= 400; n++ {
		gap := essRuleOfThree(n) - essCPUpper(0, n, essAlpha)
		worst = math.Min(worst, gap)
	}
	if worst >= -1e-12 {
		passed++
	} else {
		failed = append(failed, fmt.Sprintf("rule of three 全程 >= 精确上界：实得 %v", worst))
	}
	checkF("cp_upper(n/n)=1", essCPUpper(7, 7, essAlpha), 1.0, 0)
	checkF("cp_lower(0/n)=0", essCPLower(0, 6, essAlpha), 0.0, 0)
	checkF("cp_lower(n/n)=0.05^(1/n)", essCPLower(21, 21, essAlpha), math.Pow(essAlpha, 1.0/21), 1e-12)
	checkF("binom_cdf 归一", essBinomCDF(20, 20, 0.3), 1.0, 0)
	checkF("binom_cdf(-1)=0", essBinomCDF(-1, 20, 0.3), 0.0, 0)
	checkF("binom_cdf(0,20,0.139108)=alpha", essBinomCDF(0, 20, 0.13910834066826516), essAlpha, 1e-9)
	if essCPUpper(1, 20, essAlpha) > essCPUpper(0, 20, essAlpha) {
		passed++
	} else {
		failed = append(failed, "上界随 k 单调增：假")
	}
	if essCPLower(25, 25, essAlpha) > essCPLower(21, 21, essAlpha) {
		passed++
	} else {
		failed = append(failed, "下界随 n 单调增(k=n)：假")
	}
	for _, c := range essTierCases {
		t, has := essTierFor(c[0])
		if c[1] < 0 {
			if !has {
				passed++
			} else {
				failed = append(failed, fmt.Sprintf("tier_for(%.8f)：期望 None，实得 %v", c[0], t))
			}
			continue
		}
		if has && t == c[1] {
			passed++
		} else {
			failed = append(failed, fmt.Sprintf("tier_for(%.8f)：期望 %v，实得 %v/%v", c[0], c[1], t, has))
		}
	}
	checkS("档位显示 0.5% 不被 round 成 0%", essPctTier(0.005), "0.5%")
	checkS("档位显示 15%", essPctTier(0.15), "15%")
	for _, c := range [][2]float64{{0.50, 5}, {0.15, 19}, {0.10, 29}, {0.05, 59}, {0.01, 299}} {
		checkI(fmt.Sprintf("min_n_for_upper_bound(%g)", c[0]), essMinNForUpperBound(c[0], essAlpha), int(c[1]))
	}
	for _, c := range [][2]float64{{0.50, 5}, {0.15, 19}, {0.05, 59}, {0.01, 299}} {
		checkI(fmt.Sprintf("min_n_to_detect(%g)", c[0]), essMinNToDetect(c[0], 0.95), int(c[1]))
	}
	obs := ""
	for i := 0; i < 6; i++ {
		if i > 0 {
			obs += "\n"
		}
		obs += `{"group":"g","vector_hash":"aa"}`
	}
	gs, err := essLoadObservationsText(obs, "group", "", "")
	if err == nil {
		r, err2 := essAnalyzeGroup("g", gs["g"], essAlpha, "", false)
		if err2 == nil {
			checkI("0/6 端到端 n", r.n, 6)
			checkI("0/6 端到端 k", r.k, 0)
			checkI("0/6 端到端 uniq", r.unique, 1)
			checkF("0/6 端到端上界", r.cpUpper, essUpCases[0][2], 1e-9)
			checkF("0/6 端到端 rule of three", r.ruleOfThree, 0.5, 0)
		}
	}
	obs = ""
	for i := 0; i < 20; i++ {
		if i > 0 {
			obs += "\n"
		}
		obs += fmt.Sprintf(`{"group":"g","diverged":%s}`, map[bool]string{true: "false", false: "true"}[i != 0])
	}
	gs, err = essLoadObservationsText(obs, "group", "", "")
	if err == nil {
		r, err2 := essAnalyzeGroup("g", gs["g"], essAlpha, "", false)
		if err2 == nil {
			checkI("1/20(diverged) n", r.n, 20)
			checkI("1/20(diverged) k", r.k, 1)
			checkF("1/20(diverged) 上界", r.cpUpper, essUpCases[9][2], 1e-9)
		}
	}
	obs = ""
	for i := 0; i < 5; i++ {
		if i > 0 {
			obs += "\n"
		}
		obs += `{"group":"g","vector_hash":"solo"}`
	}
	for i := 0; i < 25; i++ {
		obs += fmt.Sprintf("\n{\"group\":\"g\",\"vector_hash\":\"h%d\"}", i)
	}
	gs, err = essLoadObservationsText(obs, "group", "", "")
	if err == nil {
		r, err2 := essAnalyzeGroup("g", gs["g"], essAlpha, "solo", true)
		if err2 == nil {
			checkI("25/30 对显式参考 n", r.n, 30)
			checkI("25/30 对显式参考 k", r.k, 25)
			checkF("25/30 对显式参考下界", r.cpLower, essLoCases[4][2], 1e-9)
		}
	}
	obs = ""
	for i := 0; i < 25; i++ {
		if i > 0 {
			obs += "\n"
		}
		obs += fmt.Sprintf(`{"group":"g","vector_hash":"u%d"}`, i)
	}
	gs, err = essLoadObservationsText(obs, "group", "", "")
	if err == nil {
		r, err2 := essAnalyzeGroup("g", gs["g"], essAlpha, "solo", true)
		if err2 == nil {
			checkI("25/25 全分歧 n", r.n, 25)
			checkI("25/25 全分歧 k", r.k, 25)
			checkF("25/25 全分歧下界", r.cpLower, essLoCases[3][2], 1e-9)
		}
	}
	for _, bad := range []string{"", "{\n", `{"group":"g"}`} {
		if _, err := essLoadObservationsText(bad, "group", "", ""); err != nil {
			passed++
		} else {
			failed = append(failed, fmt.Sprintf("非法输入未抛错：%q", bad))
		}
	}
	for _, bad := range [][2]int{{0, 0}, {3, 2}} {
		func() {
			defer func() {
				if recover() != nil {
					passed++
				} else {
					failed = append(failed, fmt.Sprintf("非法 k/n 未抛错：%v", bad))
				}
			}()
			essCPUpper(bad[0], bad[1], essAlpha)
		}()
	}
	return passed, failed
}

func essRunSelfTestSafe() (passed int, failed []string) {
	defer func() {
		if r := recover(); r != nil {
			passed = 0
			failed = []string{fmt.Sprintf("自检自身抛异常（视为红）：%v", r)}
		}
	}()
	return essSelfTest()
}

// essLoadObservationsText 是自测用的内存版输入解析（py 用 tempfile；此处直接按串解析，输出等价）。
func essLoadObservationsText(text, groupField, hashField, divergedField string) (map[string]*essSlot, error) {
	return essLoadObservations("<mem>", text, groupField, hashField, divergedField)
}

// --------------------------------------------------------------------------- //
// JSON 输出（对齐 py json.dumps(..., ensure_ascii=False, indent=2)）
// --------------------------------------------------------------------------- //

type essJObj struct {
	keys []string
	vals []interface{}
}

func essNewJObj() *essJObj { return &essJObj{} }

func (o *essJObj) set(k string, v interface{}) *essJObj {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
	return o
}

func essPyFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if math.IsNaN(f) {
		return "NaN"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

func essJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\b':
			b.WriteString("\\b")
		case '\t':
			b.WriteString("\\t")
		case '\n':
			b.WriteString("\\n")
		case '\f':
			b.WriteString("\\f")
		case '\r':
			b.WriteString("\\r")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func essJSONEmit(v interface{}, indent string, out *strings.Builder) {
	switch t := v.(type) {
	case *essJObj:
		if len(t.keys) == 0 {
			out.WriteString("{}")
			return
		}
		out.WriteString("{\n")
		for i, k := range t.keys {
			out.WriteString(indent + "  ")
			out.WriteString(essJSONString(k))
			out.WriteString(": ")
			essJSONEmit(t.vals[i], indent+"  ", out)
			if i < len(t.keys)-1 {
				out.WriteString(",")
			}
			out.WriteString("\n")
		}
		out.WriteString(indent + "}")
	case []interface{}:
		if len(t) == 0 {
			out.WriteString("[]")
			return
		}
		out.WriteString("[\n")
		for i, e := range t {
			out.WriteString(indent + "  ")
			essJSONEmit(e, indent+"  ", out)
			if i < len(t)-1 {
				out.WriteString(",")
			}
			out.WriteString("\n")
		}
		out.WriteString(indent + "]")
	case string:
		out.WriteString(essJSONString(t))
	case float64:
		out.WriteString(essPyFloat(t))
	case int:
		out.WriteString(strconv.Itoa(t))
	case bool:
		if t {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case nil:
		out.WriteString("null")
	default:
		out.WriteString("null")
	}
}

func essGroupToJSON(r *essGroupResult) *essJObj {
	o := essNewJObj()
	o.set("group", r.group)
	o.set("n", r.n)
	o.set("k", r.k)
	if r.hasUnique {
		o.set("unique", r.unique)
	} else {
		o.set("unique", nil)
	}
	if r.hasReference {
		o.set("reference", r.reference)
	} else {
		o.set("reference", nil)
	}
	o.set("rate", r.rate)
	o.set("cp_upper", r.cpUpper)
	o.set("cp_lower", r.cpLower)
	if r.hasROT {
		o.set("rule_of_three", r.ruleOfThree)
	} else {
		o.set("rule_of_three", nil)
	}
	o.set("alpha", r.alpha)
	if r.hasTop {
		arr := []interface{}{}
		for _, v := range r.topValues {
			e := essNewJObj()
			e.set("hash", v.hash)
			e.set("count", v.count)
			arr = append(arr, e)
		}
		o.set("top_values", arr)
		o.set("mode_count", r.modeCount)
	}
	o.set("verdict", r.verdict)
	return o
}

func essRenderJSON(results []*essGroupResult, alpha float64) string {
	top := essNewJObj()
	top.set("alpha", alpha)
	arr := []interface{}{}
	for _, r := range results {
		arr = append(arr, essGroupToJSON(r))
	}
	top.set("groups", arr)
	var b strings.Builder
	essJSONEmit(top, "", &b)
	return b.String()
}

// --------------------------------------------------------------------------- //
// 参数解析（argparse 子集同构：唯一前缀缩写 / == 值 / -h / --）
// --------------------------------------------------------------------------- //

type essArgs struct {
	selfTest      bool
	groupField    string
	hashField     string
	divergedField string
	reference     []string
	alpha         float64
	json          bool
	noPreflight   bool
	noTable       bool
	inputs        []string
	helpRequested bool
	unrecognized  []string
	parseErr      string
}

var essLongOpts = []string{"--help", "--self-test", "--group-field", "--hash-field",
	"--diverged-field", "--reference", "--alpha", "--json", "--no-preflight", "--no-table"}

func essTakesValue(opt string) bool {
	switch opt {
	case "--group-field", "--hash-field", "--diverged-field", "--reference", "--alpha":
		return true
	}
	return false
}

func essArgLooksLikeOption(t string) bool {
	if len(t) < 2 || t[0] != '-' {
		return false
	}
	if strings.HasPrefix(t, "--") {
		return true
	}
	if _, err := strconv.ParseFloat(t, 64); err == nil {
		return false
	}
	return true
}

func essResolveLong(name string) (string, string, bool) {
	// exact
	for _, o := range essLongOpts {
		if o == name {
			return o, "", true
		}
	}
	cands := []string{}
	for _, o := range essLongOpts {
		if strings.HasPrefix(o, name) {
			cands = append(cands, o)
		}
	}
	if len(cands) == 1 {
		return cands[0], "", true
	}
	if len(cands) > 1 {
		return "", strings.Join(cands, ", "), false
	}
	return "", "", false
}

func essParseArgs(argv []string) *essArgs {
	a := &essArgs{groupField: "group", alpha: essAlpha}
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if tok == "--" {
			i++
			for i < len(argv) {
				a.inputs = append(a.inputs, argv[i])
				i++
			}
			break
		}
		if strings.HasPrefix(tok, "--") {
			name := tok
			val := ""
			hasVal := false
			if eq := strings.Index(tok, "="); eq >= 0 {
				name = tok[:eq]
				val = tok[eq+1:]
				hasVal = true
			}
			canon, amb, ok := essResolveLong(name)
			if !ok {
				if amb != "" {
					a.parseErr = fmt.Sprintf("ambiguous option: %s could match %s", name, amb)
					return a
				}
				a.unrecognized = append(a.unrecognized, tok)
				i++
				continue
			}
			if canon == "--help" {
				a.helpRequested = true
				return a
			}
			if essTakesValue(canon) {
				if !hasVal {
					if i+1 < len(argv) && !essArgLooksLikeOption(argv[i+1]) {
						val = argv[i+1]
						hasVal = true
						i++
					} else {
						a.parseErr = fmt.Sprintf("argument %s: expected one argument", canon)
						return a
					}
				}
				if canon == "--alpha" {
					f, err := strconv.ParseFloat(val, 64)
					if err != nil {
						a.parseErr = fmt.Sprintf("argument --alpha: invalid float value: '%s'", val)
						return a
					}
					a.alpha = f
				} else if canon == "--group-field" {
					a.groupField = val
				} else if canon == "--hash-field" {
					a.hashField = val
				} else if canon == "--diverged-field" {
					a.divergedField = val
				} else if canon == "--reference" {
					a.reference = append(a.reference, val)
				}
			} else {
				if hasVal {
					a.parseErr = fmt.Sprintf("argument %s: ignored explicit argument '%s'", canon, val)
					return a
				}
				switch canon {
				case "--self-test":
					a.selfTest = true
				case "--json":
					a.json = true
				case "--no-preflight":
					a.noPreflight = true
				case "--no-table":
					a.noTable = true
				}
			}
			i++
			continue
		}
		if len(tok) > 1 && tok[0] == '-' {
			if tok == "-h" {
				a.helpRequested = true
				return a
			}
			a.unrecognized = append(a.unrecognized, tok)
			i++
			continue
		}
		a.inputs = append(a.inputs, tok)
		i++
	}
	return a
}

// --------------------------------------------------------------------------- //
// JSON 解码错误正文（对齐 CPython json 的单行常见形态）
// --------------------------------------------------------------------------- //

type essJErr struct {
	msg string
	pos int
}

func essScanWS(rs []rune, i *int) {
	for *i < len(rs) && (rs[*i] == ' ' || rs[*i] == '\t' || rs[*i] == '\n' || rs[*i] == '\r') {
		*i++
	}
}

func essScanString(rs []rune, i *int) (essJErr, bool) {
	start := *i
	*i++ // opening quote
	for *i < len(rs) {
		c := rs[*i]
		if c == '\\' {
			*i += 2
			continue
		}
		if c == '"' {
			*i++
			return essJErr{}, false
		}
		*i++
	}
	return essJErr{"Unterminated string starting at", start}, true
}

func essScanNumber(rs []rune, i *int) (essJErr, bool) {
	for *i < len(rs) {
		c := rs[*i]
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			*i++
			continue
		}
		break
	}
	return essJErr{}, false
}

func essScanValue(rs []rune, i *int) (essJErr, bool) {
	essScanWS(rs, i)
	if *i >= len(rs) {
		return essJErr{"Expecting value", *i}, true
	}
	c := rs[*i]
	switch {
	case c == '"':
		return essScanString(rs, i)
	case c == '{':
		return essScanObject(rs, i)
	case c == '[':
		return essScanArray(rs, i)
	case c == 'n':
		if strings.HasPrefix(string(rs[*i:]), "null") {
			*i += 4
			return essJErr{}, false
		}
		return essJErr{"Expecting value", *i}, true
	case c == 't':
		if strings.HasPrefix(string(rs[*i:]), "true") {
			*i += 4
			return essJErr{}, false
		}
		return essJErr{"Expecting value", *i}, true
	case c == 'f':
		if strings.HasPrefix(string(rs[*i:]), "false") {
			*i += 5
			return essJErr{}, false
		}
		return essJErr{"Expecting value", *i}, true
	case c == '-' || (c >= '0' && c <= '9'):
		return essScanNumber(rs, i)
	}
	return essJErr{"Expecting value", *i}, true
}

func essScanObject(rs []rune, i *int) (essJErr, bool) {
	*i++ // {
	essScanWS(rs, i)
	if *i < len(rs) && rs[*i] == '}' {
		*i++
		return essJErr{}, false
	}
	for {
		essScanWS(rs, i)
		if *i >= len(rs) || rs[*i] != '"' {
			return essJErr{"Expecting property name enclosed in double quotes", *i}, true
		}
		if err, bad := essScanString(rs, i); bad {
			return err, true
		}
		essScanWS(rs, i)
		if *i >= len(rs) || rs[*i] != ':' {
			return essJErr{"Expecting ':' delimiter", *i}, true
		}
		*i++
		if err, bad := essScanValue(rs, i); bad {
			return err, true
		}
		essScanWS(rs, i)
		if *i < len(rs) && rs[*i] == ',' {
			*i++
			continue
		}
		if *i < len(rs) && rs[*i] == '}' {
			*i++
			return essJErr{}, false
		}
		return essJErr{"Expecting ',' delimiter", *i}, true
	}
}

func essScanArray(rs []rune, i *int) (essJErr, bool) {
	*i++ // [
	essScanWS(rs, i)
	if *i < len(rs) && rs[*i] == ']' {
		*i++
		return essJErr{}, false
	}
	for {
		if err, bad := essScanValue(rs, i); bad {
			return err, true
		}
		essScanWS(rs, i)
		if *i < len(rs) && rs[*i] == ',' {
			*i++
			continue
		}
		if *i < len(rs) && rs[*i] == ']' {
			*i++
			return essJErr{}, false
		}
		return essJErr{"Expecting ',' delimiter", *i}, true
	}
}

func essPyJSONErr(s string) string {
	rs := []rune(s)
	i := 0
	e, bad := essScanValue(rs, &i)
	if bad {
		return fmt.Sprintf("%s: line 1 column %d (char %d)", e.msg, e.pos+1, e.pos)
	}
	essScanWS(rs, &i)
	if i < len(rs) {
		return fmt.Sprintf("Extra data: line 1 column %d (char %d)", i+1, i)
	}
	return "Expecting value: line 1 column 1 (char 0)"
}

// --------------------------------------------------------------------------- //
// 入口
// --------------------------------------------------------------------------- //

func evalStabilityStatsGateMain(stdout, stderr io.Writer, argv []string) int {
	a := essParseArgs(argv)
	if a.helpRequested {
		fmt.Fprint(stdout, essHelpText)
		return 0
	}
	if a.parseErr != "" {
		fmt.Fprint(stderr, essUsage)
		fmt.Fprintf(stderr, "stability-stats.py: error: %s\n", a.parseErr)
		return 2
	}
	if len(a.unrecognized) > 0 {
		fmt.Fprint(stderr, essUsage)
		fmt.Fprintf(stderr, "stability-stats.py: error: unrecognized arguments: %s\n", strings.Join(a.unrecognized, " "))
		return 2
	}

	if a.selfTest {
		passed, failed := essRunSelfTest()
		fmt.Fprintf(stdout, "自测：%d 项通过，%d 项失败\n", passed, len(failed))
		for _, f := range failed {
			fmt.Fprintf(stdout, "  FAIL %s\n", f)
		}
		if len(failed) > 0 {
			fmt.Fprint(stdout, "自测红 ⇒ 拒跑真目标（rc=2）\n")
			return 2
		}
		fmt.Fprint(stdout, "自测全绿（已知答案：0/6=39.30%、0/20=13.91%、1/20=21.61%、5/59=17.00%、"+
			"0/59=4.95%、0/300=0.99%、21/21 下界=86.71%、25/25 下界=88.71%）\n")
		return 0
	}

	if len(a.inputs) == 0 {
		fmt.Fprint(stdout, essHelpText)
		return 2
	}

	if !a.noPreflight {
		passed, failed := essRunSelfTest()
		if len(failed) > 0 {
			fmt.Fprintf(stderr, "预检自测失败（%d 项）⇒ 拒跑真目标（rc=2）\n", len(failed))
			for _, f := range failed {
				fmt.Fprintf(stderr, "  FAIL %s\n", f)
			}
			return 2
		}
		fmt.Fprintf(stderr, "预检自检：%d 项全绿（本工具自己的正确性先过检）\n", passed)
	}

	refs := map[string]string{}
	for _, item := range a.reference {
		if !strings.Contains(item, "=") {
			fmt.Fprintf(stderr, "--reference 需要 GROUP=HASH 形式：%s\n", item)
			return 2
		}
		idx := strings.Index(item, "=")
		refs[item[:idx]] = item[idx+1:]
	}

	slots := map[string]*essSlot{}
	for _, p := range a.inputs {
		var text string
		if p == "-" {
			b, err := io.ReadAll(essStdin)
			if err != nil {
				fmt.Fprintf(stderr, "输入错误 ⇒ 硬失败（rc=2）：%v\n", err)
				return 2
			}
			text = essUniversalNewlines(b)
		} else {
			b, err := os.ReadFile(p)
			if err != nil {
				fmt.Fprintf(stderr, "输入错误 ⇒ 硬失败（rc=2）：%s\n", essPyOSErr(err, p))
				return 2
			}
			text = essUniversalNewlines(b)
		}
		gs, err := essLoadObservations(p, text, a.groupField, a.hashField, a.divergedField)
		if err != nil {
			fmt.Fprintf(stderr, "输入错误 ⇒ 硬失败（rc=2）：%s\n", err.Error())
			return 2
		}
		names := []string{}
		for g := range gs {
			names = append(names, g)
		}
		sort.Strings(names)
		for _, g := range names {
			s := gs[g]
			tgt := slots[g]
			if tgt == nil {
				tgt = &essSlot{}
				slots[g] = tgt
			}
			tgt.hashes = append(tgt.hashes, s.hashes...)
			tgt.flags = append(tgt.flags, s.flags...)
			if s.hasRef && !tgt.hasRef {
				tgt.reference = s.reference
				tgt.hasRef = true
			}
		}
	}

	names := []string{}
	for g := range slots {
		names = append(names, g)
	}
	sort.Strings(names)
	results := []*essGroupResult{}
	for _, g := range names {
		ref, hasRef := refs[g]
		r, err := essAnalyzeGroup(g, slots[g], a.alpha, ref, hasRef)
		if err != nil {
			fmt.Fprintf(stderr, "输入错误 ⇒ 硬失败（rc=2）：%s\n", err.Error())
			return 2
		}
		results = append(results, r)
	}

	if a.json {
		fmt.Fprint(stdout, essRenderJSON(results, a.alpha)+"\n")
	} else {
		fmt.Fprint(stdout, essRenderText(results, a.alpha, !a.noTable)+"\n")
	}
	return 0
}

// essPyOSErr 复刻 py 对缺失文件的 OSError 正文（"[Errno 2] No such file or directory: '<path>'"）。
func essPyOSErr(err error, path string) string {
	if os.IsNotExist(err) {
		return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", path)
	}
	return err.Error()
}
