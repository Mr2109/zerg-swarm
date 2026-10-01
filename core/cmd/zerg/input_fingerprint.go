// input_fingerprint.go —— 判据件 scripts/gates/input-fingerprint.py（165 行 · sha16 a6e8bff64fe5ba8c）的 Go 真搬
// （v2.5.14 · M2 · 输入指纹缓存族靶 · 入口 inputFingerprintGateMain）。
//
// 为什么有这件（不是顺手 Go 化）：
//
//	M2 = 按价值真搬 Go（逐批判据搬 Go · 落点 core/cmd/zerg/）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），stdout / stderr / 退码三面逐字相同（除下列显式声明的新口径差异面与未覆盖面）。
//
// 本件真身速览（逐处现读，非按件名推断）：
//
//	· 输入指纹 = sha256( 逐件「路径 \t 件内容 sha256」按路径排序以换行拼接 )；件缺/非普通件 ⇒ 记字面 MISSING 参与指纹。
//	· 哈希算法 = sha256 十六进制小写 64 位（fingerprint 输出取前 12 位进 CACHED/MISS 行）。
//	· 子进程面 = 真跑 subprocess.call(cmd, shell=True)（命中档的 --compare 与未命中档的 --cmd 两处；
//	  py 经 /bin/sh -c；子进程 stdout/stderr 继承本进程 ⇒ 其正文混进本门 stdout/stderr）。
//	· 写面 = 未命中档 json.dump 落缓存件（os.makedirs(dirname(abspath(cache)), exist_ok=True) + 文本写）。
//	· 遍历面 = 零（无 os.walk / os.listdir / glob）；输出序的决定者 = fingerprint/缓存 inputs 字段各自的 sorted(inputs)
//	  （py 明写的码点序 ⇒ Go sort.Strings，二者对 str 同序）＋ 缓存 JSON 顶层对象的键为 dict 插入序
//	  （加载既有件保原序、新键追加到末尾）⇒ 本件零 Go map 迭代进输出。
//	· 外部真源真身路径 = 无（本件不吃仓外树；--cache / --inputs 全走显式入参）。
//
// 本单边界（照实）：本件只落码、不接线 ——
//
//	· 不在 core/cmd/zerg/main.go 里注册命令（✗ 不进命令树 · ✗ 不改 add_step）；
//	· 不删 scripts/gates/input-fingerprint.py 原件（退场规则：对拍通过后才退场 · 逐件点名）；
//	· 入口 inputFingerprintGateMain 目前无调用点（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 family_*.go / 已落旧件。
//
// 入口签名（与 py 的 main() 对齐）：
//
//	func inputFingerprintGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py sys.argv[1:] 同形（不含程序名）
//	  · 退码  —— 0 命中/未命中且冷跑 rc=0 · 1 未命中且冷跑 rc≠0（逐字回传该 rc）· 2 盖章档 / 未知档 / 空转 / 缺 --cache /
//	    两态不一致 / 缓存件写不进（不给结论）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ═══════════════════════ 帮助面 / 用法错面（GAP-20261001-61 帮助面 golden）═══════════════════════
//
//	py 用 argparse.ArgumentParser()（add_help 默认 True）⇒ -h/--help 打帮助正文退 0；未知旗标 / 缺值 / 多余位置参数
//	打「usage + <prog>: error: …」退 2。本件逐字节复刻：ifpUsageText / ifpHelpText 两常量；prog 固定取 py 件基名
//	input-fingerprint.py（py 的 prog = os.path.basename(sys.argv[0])，仓里跑恒为该基名）。
//	★ 帮助 golden = py -h stdout 的 sha256 = b682873728365bf39a091906a2a1af364b06b2ad5d3a223cae3a3fd6bedc29d5（374 字节），
//	  由测试件钉住（TestInputFingerprintHelpGolden）。
//
// ★★ 两条标准声明行：
//
//	(a) 默认根定位差异（✗ 不声称逐字对齐）：py 无仓外树依赖、无 __file__ 上溯根；唯一涉 cwd 的是
//	    os.path.abspath(--cache)（相对 --cache 时以 cwd 为基）。Go 侧同法自写 ifpPyAbspath（纯词法 normpath），
//	    与 py 逐字同构（两者都取 os.Getwd()）。⇒ 该面逐字对齐，例外只在 py normpath 的「恰两个前导斜杠」角落（见未覆盖面）。
//	(b) 自指检查 —— 本件不在自己的扫描面内 ⇒ 自指 = 假（逐条给依据）：
//	    ① 本判据自身零遍历面、零仓库内件扫描（不读任何仓内源码），只读 --inputs/--cache 两个显式入参所指的件 ⇒
//	       不存在「本件落进自己的扫描面」的形态。
//	    ② 本件零绝对路径字面量（无私有卷根 / 私有家目录形态；--cache/--inputs 全走入参）。
//	    ③ 正文里的旗标名 / 文案只作字符串常量与打印内容，不落进任何被扫面。
//	    ④ --self-test 的合成件只写在 os.MkdirTemp 的临时目录里（仓外）⇒ 不落进仓树。
//	    ⇒ 处理有效：本件落仓前后，真仓 py 侧同命令读数不变（对拍甲档即真仓原件的现读件）。
//
// 已对拍面（仓外夹具两态逐字 · 甲=真仓 py 原件绝对路径 + /usr/bin/python3 / 乙=由已落仓件重建的仓外探针）：
//
//	-h 帮助（sha256 golden）· 未知旗标 · 缺值 · 多余位置参数 · 首次 MISS 并落缓存（缓存件字节比）· 再跑 CACHED ·
//	CACHED+--compare 两态一致（rc=0）· CACHED+--compare 两态不一致（rc=2）· MISS+--cmd 真跑取 rc ·
//	输入件缺（MISSING 参与指纹）· 档位 seal/stamp（rc=2）· 未知档位（rc=2）· 空转（rc=2）· 缺 --cache（rc=2）·
//	--self-test（7 控）· --cmd 跑不存在的命令（rc=127 逐字回传）· 键缺 / 键=null / 键={} 三态。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 缩写面（allow_abbrev=True）：--com 一类前缀缩写 py 认、本件不认（未实现前缀匹配）⇒ 不属逐字对齐承诺。
//	② argparse --inputs=… 的 nargs='*' 细节、`--` 分隔符、负数字面量（-_\\d）当值：本件按「-- 后全当位置参数」以外的
//	   简化读法处理，逐字不保证。
//	③ 缓存件顶层非 JSON 对象（如数组）：py load_cache 原样返回后 cache.get 裸回溯（C 类无兜底 ⇒ rc=1 回溯）；
//	   本件 fail-closed rc=2 具名（属声明的口径差，不逐字）。
//	④ 缓存键值非对象（如字符串/数）：py prev.get 裸回溯 rc=1；本件 rc=2 具名（同上）。
//	⑤ py cache[key]['rc'] 被 json.loads 成非整数（浮点/字符串）：py int()/repr 行为本件未逐字复刻（真仓写入恒为整数）。
//	⑥ 缓存件写失败：py open(...,'w') 抛异常 ⇒ 回溯 rc=1；本件 rc=2 具名（fail-closed）。
//	⑦ /bin/sh 起不来（非 ExitError）：py OSError 回溯 rc=1；本件 rc=2 具名。
//	⑧ py repr() 的正文（仅自检失败档出现）：本件 ifpPyRepr 只覆盖常见 ASCII 情形，极端串不保证逐字。
//	⑨ py normpath 的「恰两个前导斜杠」角落（//a ⇒ 保留两个）：本件 ifpPyNormpath 逐字复刻该规则，但未造对拍格坐实。
//	⑩ 非 UTF-8 路径/正文：py 与 Go 的替换粒度不同（真仓夹具全为合法 UTF-8 ⇒ 不触发）。
//	⑪ --cmd 子进程被信号终止：py subprocess.call 回负值；本件 ExitCode() 回 128+N ⇒ 不逐字。
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
	"sort"
	"strconv"
	"strings"
)

// ── 常量面（照 py CACHE_TIERS / BANNED_TIERS 逐字）──

const (
	ifpProg      = "input-fingerprint.py"
	ifpKeyNone   = "None"
	ifpTierDaily = "daily"
)

var ifpCacheTiers = []string{"daily", "medium"}
var ifpBannedTiers = []string{"seal", "stamp"}

// ifpUsageText —— argparse format_usage() 的逐字节正文（3 行 + 换行）。错误面 = 本段 + <prog>: error: 行。
const ifpUsageText = "usage: input-fingerprint.py [-h] [--tier TIER] [--cache CACHE] [--key KEY]\n" +
	"                            [--inputs [INPUTS ...]] [--cmd CMD] [--compare]\n" +
	"                            [--self-test]\n"

// ifpHelpText —— argparse -h 的逐字节正文（= usage + 空行 + 段标题 + 逐旗标行）。段标题「optional arguments:」
// 由件头声明的解释器 /usr/bin/python3（3.9.6）产出（3.10+ 会改作 options: ⇒ 换解释器即分叉，见未覆盖面「前提」）。
const ifpHelpText = ifpUsageText + "\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --tier TIER\n" +
	"  --cache CACHE\n" +
	"  --key KEY\n" +
	"  --inputs [INPUTS ...]\n" +
	"  --cmd CMD\n" +
	"  --compare\n" +
	"  --self-test\n"

// ═══════════════════════ 有序 JSON 模型（复刻 py dict 插入序 + json.dump(indent=1, ensure_ascii=False)）═══
//
// py 的缓存件顶层是 dict ⇒ 键序 = 插入序（加载既有件保原序、新键追加末尾）。Go map 迭代随机 ⇒ 禁用。
// 本件用「键切片 + 值表」的 ifpJObj 保序，自写编码器逐字节复刻 py json.dump 的缩进/分隔/转义。

type ifpJVal interface{}

type ifpJObj struct {
	keys []string
	vals map[string]ifpJVal
}

func ifpNewObj() *ifpJObj { return &ifpJObj{vals: map[string]ifpJVal{}} }

func (o *ifpJObj) get(k string) (ifpJVal, bool) {
	v, ok := o.vals[k]
	return v, ok
}

// set —— 复刻 py dict 赋值语义：既有键保位、新键追加末尾。
func (o *ifpJObj) set(k string, v ifpJVal) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

type ifpJArr []ifpJVal

// ifpPyStr —— py str() 语义的常见情形（用于 %s 文案面：缓存 rc 值）。
func ifpPyStr(v ifpJVal) string {
	switch t := v.(type) {
	case nil:
		return ifpKeyNone
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

// ifpTruthy —— py 真值判据的常见情形（if prev 那一处）。
func ifpTruthy(v ifpJVal) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return t != ""
	case bool:
		return t
	case json.Number:
		return t.String() != "0"
	case *ifpJObj:
		return len(t.keys) > 0
	case ifpJArr:
		return len(t) > 0
	}
	return true
}

// ── 有序解析（json.Decoder.Token 走 token 流 ⇒ 保键序）──

func ifpDecodeVal(dec *json.Decoder) (ifpJVal, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tv := t.(type) {
	case json.Delim:
		switch tv {
		case '{':
			obj := ifpNewObj()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				ks, _ := kt.(string)
				vv, err := ifpDecodeVal(dec)
				if err != nil {
					return nil, err
				}
				obj.set(ks, vv)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return obj, nil
		case '[':
			arr := ifpJArr{}
			for dec.More() {
				vv, err := ifpDecodeVal(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, vv)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return arr, nil
		}
		return nil, fmt.Errorf("ifp: 意外的 JSON 分隔符 %q", tv)
	case string:
		return tv, nil
	case json.Number:
		return tv, nil
	case bool:
		return tv, nil
	case nil:
		return nil, nil
	}
	return nil, fmt.Errorf("ifp: 意外的 JSON token")
}

func ifpDecodeJSON(data []byte) (ifpJVal, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := ifpDecodeVal(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("ifp: JSON 尾部有多余内容")
		}
		return nil, err
	}
	return v, nil
}

// ifpJSONStr —— 复刻 py json 串转义（ensure_ascii=False：非 ASCII 原样留，控制字符走 \\u00xx）。
func ifpJSONStr(s string) string {
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

// ifpEncode —— py json.dump(indent=1) 的逐字节复刻（item 分隔 ",\n" + 缩进，key 分隔 ": "）。
func ifpEncode(v ifpJVal, level int, b *strings.Builder) {
	pad := func(n int) { b.WriteString(strings.Repeat(" ", n)) }
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		b.WriteString(ifpJSONStr(t))
	case json.Number:
		b.WriteString(t.String())
	case *ifpJObj:
		if len(t.keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for i, k := range t.keys {
			pad(level + 1)
			b.WriteString(ifpJSONStr(k))
			b.WriteString(": ")
			ifpEncode(t.vals[k], level+1, b)
			if i < len(t.keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		pad(level)
		b.WriteString("}")
	case ifpJArr:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, e := range t {
			pad(level + 1)
			ifpEncode(e, level+1, b)
			if i < len(t)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		pad(level)
		b.WriteString("]")
	default:
		b.WriteString("null")
	}
}

func ifpMarshal(v ifpJVal) []byte {
	var b strings.Builder
	ifpEncode(v, 0, &b)
	return []byte(b.String())
}

// ═══════════════════════ py path 语义（自写同构 · ✗ 不用 filepath.Join/Abs）═══

// ifpPyJoin —— 照 CPython posixpath.join 逐段直译（b 绝对 ⇒ 弃前段；path 空或以 / 收尾 ⇒ 直拼；否则加 /）。
func ifpPyJoin(parts ...string) string {
	path := ""
	if len(parts) > 0 {
		path = parts[0]
	}
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// ifpPyNormpath —— 照 CPython posixpath.normpath 逐条直译（含「恰两个前导斜杠保留」规则）。
func ifpPyNormpath(path string) string {
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
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initial == 0 && len(newComps) == 0) || (len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// ifpPyAbspath —— py os.path.abspath：非绝对则 join(cwd, path) 再 normpath（纯词法）。
func ifpPyAbspath(path string) string {
	if !strings.HasPrefix(path, "/") {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = ""
		}
		path = ifpPyJoin(cwd, path)
	}
	return ifpPyNormpath(path)
}

// ifpPyDirname —— py os.path.dirname = posixpath.split(p)[0]（剥尾斜杠、非全斜杠时）。
func ifpPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// ifpHead —— py s[:n]（按码点切，✗ 按字节）。
func ifpHead(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// ═══════════════════════ 判据体（照 py 逐函数）═══

// ifpFileDigest —— py file_digest：非普通件 ⇒ 字面 MISSING；否则 sha256 十六进制小写 64 位。
func ifpFileDigest(p string) string {
	st, err := os.Stat(p)
	if err != nil || !st.Mode().IsRegular() {
		return "MISSING"
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "MISSING"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ifpFingerprint —— py fingerprint：逐件「路径 \t 摘要」按路径排序后换行拼接，再取 sha256。
func ifpFingerprint(inputs []string) string {
	sorted := append([]string(nil), inputs...)
	sort.Strings(sorted)
	parts := make([]string, 0, len(sorted))
	for _, p := range sorted {
		parts = append(parts, p+"\t"+ifpFileDigest(p))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// ifpLoadCache —— py load_cache：件缺 / 解析失败 ⇒ 空对象；解析成功但顶层非对象 ⇒ (nil, false) 走 fail-closed。
func ifpLoadCache(path string) (*ifpJObj, bool) {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return ifpNewObj(), true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ifpNewObj(), true
	}
	v, err := ifpDecodeJSON(data)
	if err != nil {
		return ifpNewObj(), true
	}
	obj, ok := v.(*ifpJObj)
	if !ok {
		return nil, false
	}
	return obj, true
}

// ifpWriteCache —— py os.makedirs(dirname(abspath(cache)), exist_ok=True) + open(w) + json.dump(indent=1)。
func ifpWriteCache(path string, top *ifpJObj) error {
	dir := ifpPyDirname(ifpPyAbspath(path))
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, ifpMarshal(top), 0o644)
}

// ifpRunCmd —— py subprocess.call(cmd, shell=True)：经 /bin/sh -c，子进程 stdout/stderr 继承本门写入器。
func ifpRunCmd(cmd string, stdout, stderr io.Writer) (int, bool) {
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Stdout = stdout
	c.Stderr = stderr
	c.Stdin = os.Stdin
	err := c.Run()
	if err == nil {
		return 0, true
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), true
	}
	return 0, false
}

// ifpPrevRC —— py int(prev.get('rc', -1)) 的常见情形：键缺 ⇒ -1；整数 ⇒ 其值；其余 ⇒ (0,false) 交调用方 fail-closed。
func ifpPrevRC(obj *ifpJObj) (int, bool) {
	v, ok := obj.get("rc")
	if !ok {
		return -1, true
	}
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := n.Int64()
	if err != nil {
		return 0, false
	}
	return int(i), true
}

// ═══════════════════════ argv 解析（复刻 argparse 的已覆盖面）═══

type ifpOpts struct {
	tier     string
	cache    string
	key      string
	inputs   []string
	cmd      string
	compare  bool
	selfTest bool
}

func ifpUsageErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, ifpUsageText)
	fmt.Fprintf(stderr, "%s: error: %s\n", ifpProg, msg)
	return 2
}

// ifpLooksLikeOpt —— argparse 的 _parse_optional 归类（保守：'-' 起头且非单 '-' 即视作 option 形态）。
func ifpLooksLikeOpt(s string) bool {
	return strings.HasPrefix(s, "-") && s != "-"
}

// ifpParseArgs —— 返回 (opts, 退码, 是否已终结)。终结 = 已把 -h 正文或用法错正文写出，调用方直接回退码。
func ifpParseArgs(argv []string, stdout, stderr io.Writer) (*ifpOpts, int, bool) {
	o := &ifpOpts{tier: ifpTierDaily, key: "default", inputs: []string{}}
	var extras []string
	i := 0
	for i < len(argv) {
		a := argv[i]
		if a == "-h" || a == "--help" {
			// argparse 的 help action 在解析途中即 fire ⇒ 立即打印退 0（不收集后续 unrecognized）。
			fmt.Fprint(stdout, ifpHelpText)
			return nil, 0, true
		}
		if strings.HasPrefix(a, "--") {
			name := a
			val := ""
			hasVal := false
			if eq := strings.Index(a, "="); eq >= 0 {
				name = a[:eq]
				val = a[eq+1:]
				hasVal = true
			}
			switch name {
			case "--tier", "--cache", "--key", "--cmd":
				if !hasVal {
					if i+1 >= len(argv) || ifpLooksLikeOpt(argv[i+1]) {
						return nil, ifpUsageErr(stderr, "argument "+name+": expected one argument"), true
					}
					val = argv[i+1]
					i++
				}
				switch name {
				case "--tier":
					o.tier = val
				case "--cache":
					o.cache = val
				case "--key":
					o.key = val
				case "--cmd":
					o.cmd = val
				}
			case "--inputs":
				if hasVal {
					o.inputs = []string{val}
				} else {
					got := []string{}
					j := i + 1
					for j < len(argv) && !ifpLooksLikeOpt(argv[j]) {
						got = append(got, argv[j])
						j++
					}
					o.inputs = got
					i = j - 1
				}
			case "--compare", "--self-test":
				if hasVal {
					return nil, ifpUsageErr(stderr, "argument "+name+": ignored explicit argument '"+val+"'"), true
				}
				if name == "--compare" {
					o.compare = true
				} else {
					o.selfTest = true
				}
			default:
				extras = append(extras, a)
			}
			i++
			continue
		}
		extras = append(extras, a)
		i++
	}
	if len(extras) > 0 {
		return nil, ifpUsageErr(stderr, "unrecognized arguments: "+strings.Join(extras, " ")), true
	}
	return o, 0, false
}

// inputFingerprintGateMain —— 与 py main() 对齐的入口。
//
// ★ stdout 缓冲语义（复刻 CPython）：py 的 sys.stdout 在**非 TTY**（管道/重定向）下是 8KiB 块缓冲，
// 本门自己 print 的正文**不立即落 fd**；而 --cmd 子进程继承 fd 1 **直写**。⇒ 子进程正文先落、本门正文在
// 进程退出时整体 flush 到后面（实测 py `--cmd 'printf hi'` ⇒ stdout = "hi" + "MISS …"）。本件照此：本门自身
// 正文先写进程内 buf，子进程写**真实** stdout，最后 io.Copy(buf → stdout) 一次性落盘。
// 未覆盖：>8KiB 的自动中途 flush、stdout 为 TTY 时的行缓冲（对拍一律管道 ⇒ 不触发）。
func inputFingerprintGateMain(stdout, stderr io.Writer, argv []string) int {
	var buf bytes.Buffer
	rc := ifpCore(&buf, stdout, stderr, argv)
	_, _ = io.Copy(stdout, &buf)
	return rc
}

// ifpCore —— 本门主体。stdout = 本门自身正文的落点（进程内 buf）；childOut = 子进程继承的落点（真实 stdout）。
func ifpCore(stdout, childOut, stderr io.Writer, argv []string) int {
	o, rc, stop := ifpParseArgs(argv, stdout, stderr)
	if stop {
		return rc
	}
	if o.selfTest {
		return ifpSelfTest(stdout, stderr)
	}
	if ifpIn(o.tier, ifpBannedTiers) {
		fmt.Fprintf(stderr, "✗ 盖章档（%s）**禁用缓存** ⇒ 必全冷跑 ⇒ rc=2（不给结论）\n", o.tier)
		return 2
	}
	if !ifpIn(o.tier, ifpCacheTiers) {
		fmt.Fprintf(stderr, "✗ 未知档位 %s（闭集 = daily/medium 走缓存 · seal/stamp 禁用）⇒ rc=2\n", ifpPyRepr(o.tier))
		return 2
	}
	if len(o.inputs) == 0 {
		fmt.Fprint(stderr, "✗ 无输入件 ⇒ 空转 ⇒ rc=2\n")
		return 2
	}
	if o.cache == "" {
		fmt.Fprint(stderr, "✗ 缺 --cache 件 ⇒ rc=2\n")
		return 2
	}
	fp := ifpFingerprint(o.inputs)
	top, topIsObj := ifpLoadCache(o.cache)
	if !topIsObj {
		fmt.Fprint(stderr, "✗ 缓存件顶层不是 JSON 对象（py 原件此处 cache.get 裸回溯）⇒ rc=2（不给结论）\n")
		return 2
	}
	if prev, ok := top.get(o.key); ok && ifpTruthy(prev) {
		pobj, isObj := prev.(*ifpJObj)
		if !isObj {
			fmt.Fprint(stderr, "✗ 缓存键值不是 JSON 对象（py 原件此处 prev.get 裸回溯）⇒ rc=2（不给结论）\n")
			return 2
		}
		pfp, _ := pobj.get("fp")
		ps, _ := pfp.(string)
		if ps == fp {
			fmt.Fprintf(stdout, "CACHED %s\n", ifpHead(fp, 12))
			fmt.Fprint(stdout, "  （本行**不是通过**：输入指纹命中既有缓存，本轮未真跑 · 硬线：命中不得渲染成绿）\n")
			if o.compare && o.cmd != "" {
				rcNow, okRun := ifpRunCmd(o.cmd, childOut, stderr)
				if !okRun {
					fmt.Fprint(stderr, "✗ 冷跑子进程起不来（/bin/sh 不可用）⇒ rc=2（不给结论）\n")
					return 2
				}
				prevRC, okPrev := ifpPrevRC(pobj)
				if !okPrev {
					fmt.Fprint(stderr, "✗ 缓存 rc 不是整数（py 原件此处 int() 行为未逐字复刻）⇒ rc=2（不给结论）\n")
					return 2
				}
				if prevRC != rcNow {
					raw, _ := pobj.get("rc")
					fmt.Fprintf(stderr, "✗ 冷跑对拍不等：缓存 rc=%s ⟷ 冷跑 rc=%s ⇒ rc=2\n", ifpPyStr(raw), strconv.Itoa(rcNow))
					return 2
				}
				fmt.Fprintf(stdout, "  ── 冷跑对拍：缓存 rc=%s == 冷跑 rc=%s ✓\n", strconv.Itoa(prevRC), strconv.Itoa(rcNow))
			}
			return 0
		}
	}
	fmt.Fprintf(stdout, "MISS %s（未命中 ⇒ 真跑并记缓存）\n", ifpHead(fp, 12))
	rc = 0
	if o.cmd != "" {
		var okRun bool
		rc, okRun = ifpRunCmd(o.cmd, childOut, stderr)
		if !okRun {
			fmt.Fprint(stderr, "✗ 冷跑子进程起不来（/bin/sh 不可用）⇒ rc=2（不给结论）\n")
			return 2
		}
	}
	entry := ifpNewObj()
	entry.set("fp", fp)
	entry.set("rc", json.Number(strconv.Itoa(rc)))
	ins := append([]string(nil), o.inputs...)
	sort.Strings(ins)
	arr := ifpJArr{}
	for _, s := range ins {
		arr = append(arr, s)
	}
	entry.set("inputs", arr)
	top.set(o.key, entry)
	if err := ifpWriteCache(o.cache, top); err != nil {
		fmt.Fprintf(stderr, "✗ 缓存件写不进：%v ⇒ rc=2（不给结论）\n", err)
		return 2
	}
	if rc == 0 {
		return 0
	}
	return rc
}

func ifpIn(s string, xs []string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ifpPyRepr —— py repr(str) 的常见 ASCII 情形（仅自检失败档的 %r 面用；极端串见未覆盖面⑧）。
func ifpPyRepr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\'':
			b.WriteString("\\'")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// ═══════════════════════ --self-test（7 控 · py self_test 逐字）═══
//
// py 的 run() 真起子进程（subprocess.run([sys.executable, abspath(__file__)] + args)）；本件进程内直调入口
// （同 argv ⇒ 读数一致）⇒ 打印文案逐字对齐，差在实现手段（子进程 ⟷ 进程内，属第二类·运行时可表达性差）。
// ★ py 的 mkdtemp 在 finally 里 shutil.rmtree（**自回收**）⇒ 非 A 类泄漏；本件 defer os.RemoveAll 同效。
func ifpSelfTest(stdout, stderr io.Writer) int {
	var fails []string
	tmp, err := os.MkdirTemp("", "input-fp-selftest-")
	if err != nil {
		fmt.Fprintf(stderr, "✗ 自检建临时目录失败：%v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	inp := filepath.Join(tmp, "a.txt")
	if err := os.WriteFile(inp, []byte("v1"), 0o644); err != nil {
		fmt.Fprintf(stderr, "✗ 自检写件失败：%v\n", err)
		return 1
	}
	cache := filepath.Join(tmp, "c.json")
	run := func(args []string) (int, string) {
		var so, se bytes.Buffer
		rc := inputFingerprintGateMain(&so, &se, args)
		return rc, so.String()
	}
	rc, out := run([]string{"--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "true"})
	if rc != 0 || !strings.Contains(out, "MISS") {
		fails = append(fails, fmt.Sprintf("① 首次冷跑期望 MISS/rc=0，实得 rc=%d out=%s", rc, ifpPyRepr(ifpHead(out, 60))))
	}
	rc, out = run([]string{"--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "true"})
	if !strings.Contains(out, "CACHED") {
		fails = append(fails, fmt.Sprintf("② 期望 CACHED，实得 %s", ifpPyRepr(ifpHead(out, 60))))
	}
	if strings.Contains(out, "PASS") {
		fails = append(fails, "② 命中却出现 PASS（硬线：命中不得显示成 PASS）")
	}
	rc, _ = run([]string{"--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "true", "--compare"})
	if rc != 0 {
		fails = append(fails, fmt.Sprintf("③ 两态一致期望 rc=0，实得 %d", rc))
	}
	rc, _ = run([]string{"--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "false", "--compare"})
	if rc != 2 {
		fails = append(fails, fmt.Sprintf("④ 两态不一致期望 rc=2，实得 %d", rc))
	}
	for _, t := range []string{"seal", "stamp"} {
		rc, _ = run([]string{"--tier", t, "--cache", cache, "--inputs", inp})
		if rc != 2 {
			fails = append(fails, fmt.Sprintf("⑤ 档位 %s 期望 rc=2，实得 %d", t, rc))
		}
	}
	rc, _ = run([]string{"--tier", "daily", "--cache", cache})
	if rc != 2 {
		fails = append(fails, fmt.Sprintf("⑥ 空转期望 rc=2，实得 %d", rc))
	}
	if err := os.WriteFile(inp, []byte("v2"), 0o644); err != nil {
		fails = append(fails, fmt.Sprintf("⑦ 自检改件失败：%v", err))
	}
	rc, out = run([]string{"--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "true"})
	if !strings.Contains(out, "MISS") {
		fails = append(fails, fmt.Sprintf("⑦ 输入变期望 MISS，实得 %s", ifpPyRepr(ifpHead(out, 60))))
	}
	if len(fails) > 0 {
		fmt.Fprint(stdout, "自检不过：\n")
		for _, x := range fails {
			fmt.Fprint(stdout, "  ✗ "+x+"\n")
		}
		return 1
	}
	fmt.Fprint(stdout, "自检通过：7 控（正控 3 + 负控 4）逐条对拍 ✓\n")
	return 0
}
