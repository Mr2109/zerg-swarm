// check_help_snapshot.go —— 判据件 `scripts/gates/` 下「帮助成文快照 ↔ 运行期逐行对拍」
// 那枚（281 行 · 波6 序70）的 **Go 真搬**（甲案 C 类第 2 枚 · M2 路A续）。
//
// 为什么这一枚在甲案下**适格**（旧批 laneB450 曾把「真跑子进程」判为不适格 · 本批改判）：
//
//	甲案口径 = 判据件可真跑子进程 / 吃制品，**条件 = 输入确定 ⇒ 输出可逐字对拍**。
//	本件跑 `bin/zerg help export --dry-run` 与 `bin/zerg help export --json commands --dry-run`
//	两半，产物是现役 CLI 制品（只读复制到仓外锁定指纹）⇒ 同一制品 + 同一快照 ⇒ 输出逐字确定。
//	快照那一半是**成文产物**（文档仓 `<前缀>-<日期>.md`）⇒ 必须显式 `--snapshot` 钉版才叫确定
//	（缺省取 mtime 最新的一份 = 会随文档仓变动而漂 ⇒ 属「有意旋钮面差」，见件头 (a)）。
//
// 本件的双重价值：① 它是**已知泄漏源头** —— 同干 py 原件用 `tempfile.mkdtemp(prefix=...)` 搭自检夹具
// 且**全件 0 处回收** ⇒ 已在 /private/var/tmp 堆下成百上千枚同名临时目录（已入账 GAP 同族）。
// 本件自检**自带 `defer os.RemoveAll`**，测试件钉住「跑测前后同名前缀计数相等（delta=0）」⇒ 边修边清。
// ② 「真跑子进程」这一面在甲案下已适格 ⇒ 直接搬，不再列为不适格。
//
// 出厂声明 (a) **默认根定位差异**：py 的默认根 = `dirname(dirname(dirname(abspath(__file__))))`
// （脚本上溯三级 = 仓根）；Go 入口**无 `__file__` 语境** ⇒ 本件默认从**当前工作目录**上溯 ≤6 层、找
// 含 `scripts/gates/` 的目录，找不到回落 cwd。落在**同一真仓、cwd=仓根**时两者读数逐字相同；
// 「脚本位置 ≠ cwd」时**路径推导面不同** ⇒ 这一面**不属逐字对齐承诺**，对拍一律显式传 `--repo-root`。
// 另有**有意旋钮面差**：快照缺省档（不传 `--snapshot`）走 `os.walk` 找 mtime 最新的一份 ——
// 那是**输入会随文档仓变动**的档 ⇒ 确定化必须显式钉版；缺省档只保「选中的是 (mtime, 路径) 最大者」。
//
// 出厂声明 (b) **自指检查 —— 判定为「真」**：本件落 `core/cmd/zerg/`，正落在下列三门的扫描面内
// （都现跑 `core/cmd/zerg/*.go` 的非测试件）：
//
//	· 私有绝对路径门：扫 `core/` 全树，命中词 = 三类私有绝对路径前缀 —— 本件**代码行**一处不写
//	  （注释行按该门口径本就不算；本件亦不写），遗留悬空字面计数 = 0。
//	· error.kind 闭集门：扫 `core/cmd/zerg/*.go` 的 **setErr 首参字面量**与 **JSON kind 键值**两形态
//	  —— 本件**一处不写**（本门机器面字段表里没有 kind 这一格），遗留悬空字面计数 = 0。
//	· 茧壁层级门：扫 `core/cmd/zerg/*.go` 的**层级常量赋值**与**命令树层级标记键值**两形态 —— 本件**一处不写**，
//	  遗留悬空字面计数 = 0。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func checkHelpSnapshotGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 两半都合 / 1 判红 / 2 不给结论（与 py 三码逐字对齐）
//
// 词法面（§十条标准①）逐处现读：本件**无 `fnmatch`**（不适用）· 正则两条都是 RE2 可直接表达的
// 锚定行匹配（无环视、无 `\b`、无 `(?i)`、无 `\d`；`\s` 已展开成显式 Unicode 空白类，含 0x1c-0x1f
// / 0x85 / 0xa0 / \p{Zs}\p{Zl}\p{Zp}）· 路径拼接一律走 `hsPyJoin`（py `os.path.join` 三语义差：
// 吃 `..` 不折 · **后段绝对则丢前段** · 空段 `join('a',”) == 'a/'`）、取目录走 `hsPyDirname`；
// `open()` 文本模式折 `\r\n` 与裸 `\r`（见 `hsFoldNewlines`）· `str.splitlines()` 按 py 全套行边界
// （见 `hsSplitLines`）· `str.strip()` 按 py Unicode 语义（见 `hsPyTrim`）。
// 符号链接面（⑨）：`isfile/isdir` 一律 `os.Stat`（跟随链），✗ 不用 `DirEntry.IsDir()`。
// `posixpath.normpath` 保留恰两个前导斜杠（`//a/b`）⟷ `filepath.Clean` 折成 `/a/b` 的差 **不触发**
// （本件✗ 无 normpath 调用面 · 唯一词法折点是默认根推导，而那面已声明为 (a) 差）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const hsVersion = "1.0.0"

// 快照文件名前缀（py `SNAP_GLOB`）。
const hsSnapGlob = "导出-命令面帮助-"

// hsDecodeSentinel —— 快照件非法 UTF-8 的**内部哨兵**（不进 stdout）：py 在此裸回溯（rc=1），
// 本件同走 rc=1、stderr 走声明文案。用不可打印前缀避免与真判词相撞。
const hsDecodeSentinel = "\x00decode:"

// ── Unicode 空白面（py `\s` / `str.isspace()` 的等价类）────────────────────────────
//
//	py `\s`（str 模式）= [ \t\n\r\f\v] ∪ Unicode 空白：额外含 0x1c-0x1f（文件/组/记录/单元分隔符）、
//	0x85（NEL）、0xa0（NBSP，本就在 \p{Zs} 里）以及 Zs/Zl/Zp 三族。展开成显式类（✗ 不用 Go 的 ASCII \s）。
const hsSpaceCls = `[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\p{Zs}\p{Zl}\p{Zp}]`
const hsNonSpaceCls = `[^\t\n\v\f\r \x{1c}-\x{1f}\x{85}\p{Zs}\p{Zl}\p{Zp}]`

var (
	// py `RUNTIME_RE = re.compile(r"^\s{2}zerg\s+(\S.*?)\s*(?:｜|$)")`
	hsRuntimeRE = regexp.MustCompile("^" + hsSpaceCls + "{2}zerg" + hsSpaceCls + "+(" + hsNonSpaceCls + ".*?)" + hsSpaceCls + "*(?:｜|$)")
	// py `SNAPSHOT_RE = re.compile(r"^\|\s*`zerg ([^`]+)`\s*\|")`
	hsSnapshotRE = regexp.MustCompile("^\\|" + hsSpaceCls + "*`zerg ([^`]+)`" + hsSpaceCls + "*\\|")
)

// hsIsFile —— py `os.path.isfile`：跟随符号链接（os.Stat，✗ 非 Lstat/DirEntry.IsDir）。
func hsIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// hsPyJoin —— py `os.path.join`（三语义差全保：吃 `..` 不折 · 后段绝对丢前段 · 空段补尾 `/`）。
func hsPyJoin(a, b string) string {
	if b == "" {
		if a == "" {
			return ""
		}
		return a + "/"
	}
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" {
		return b
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// hsPyDirname —— py `os.path.dirname`（与 `filepath.Dir` 在尾斜杠/空串处有别）。
func hsPyDirname(p string) string {
	i := strings.LastIndex(p, "/")
	switch {
	case i < 0:
		return ""
	case i == 0:
		return "/"
	default:
		return strings.TrimRight(p[:i], "/")
	}
}

// hsFoldNewlines —— py 文本模式 open()/subprocess(text=True) 的 universal newlines：`\r\n` 与裸 `\r` ⇒ `\n`。
func hsFoldNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// hsIsLineBoundary —— py `str.splitlines()` 的行边界集合。
func hsIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// hsSplitLines —— py `str.splitlines()`（keepends=False）逐字复刻：末尾边界不产空尾项，空串 ⇒ []。
func hsSplitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	var cur []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if hsIsLineBoundary(r) {
			if r == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, string(cur))
			cur = nil
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// hsPyIsSpace —— py `str.isspace()`。
func hsPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ':
		return true
	}
	if r < 0x80 {
		return false
	}
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	if r == 0x85 || r == 0xa0 {
		return true
	}
	if r < 0x1000 {
		return false
	}
	return unicode.Is(unicode.Zs, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
}

// hsPyTrim —— py `str.strip()`（默认无参 = 去两端 Unicode 空白，✗ 非 strings.TrimSpace 的 ASCII 口径）。
func hsPyTrim(s string) string {
	return strings.TrimFunc(s, hsPyIsSpace)
}

// hsPyStr —— py `str(v)` 的**已覆盖**子集（None / 字符串 / 布尔 / 整数型数值）。
//
//	★ 缺键 vs 显式 null 的逐字对齐靠它：缺键 ⇒ `""`（默认值）· 显式 null ⇒ `"None"`（两者 strip 后不同）。
func hsPyStr(v interface{}) string {
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
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// hsPyRepr —— py `%s` 对 list[str] / bool / None / str 的呈现（自检打印用）。
func hsPyRepr(v interface{}) string {
	switch t := v.(type) {
	case []string:
		parts := make([]string, len(t))
		for i, s := range t {
			parts[i] = "'" + s + "'"
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return "'" + t + "'"
	case nil:
		return "None"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// hsRunArtifact —— 跑一件制品（py `subprocess.run([...], capture_output=True, text=True,
// stdin=DEVNULL)`）。返回 (折叠后标准输出, 退码, 起不来与否)。起不来 = 制品不是可执行件。
func hsRunArtifact(binz string, args ...string) (string, int, bool) {
	cmd := exec.Command(binz, args...)
	cmd.Stdin = nil // DEVNULL 等价：无输入供给
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err == nil {
		return hsFoldNewlines(out.String()), 0, true
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return hsFoldNewlines(out.String()), ee.ExitCode(), true
	}
	return "", 0, false
}

// hsRuntimeRows —— py `runtime_rows()`：现跑 `help export --dry-run` ⇒ (逐条清单行 | None, 理由)。
func hsRuntimeRows(binz string) ([]string, string) {
	if !hsIsFile(binz) {
		return nil, "`bin/zerg` 不在盘（先用 `zerg build all --only cli` 出新制品）"
	}
	stdout, rc, ok := hsRunArtifact(binz, "help", "export", "--dry-run")
	if !ok {
		return nil, "现跑 `help export --dry-run` 起不来"
	}
	if rc != 0 {
		return nil, fmt.Sprintf("现跑 `help export --dry-run` 退码 %d", rc)
	}
	var rows []string
	for _, l := range hsSplitLines(stdout) {
		if m := hsRuntimeRE.FindStringSubmatch(l); m != nil {
			rows = append(rows, hsPyTrim(m[1]))
		}
	}
	if len(rows) == 0 {
		return nil, "现跑清单零行（制品不对 / 输出格式改了？）"
	}
	return rows, ""
}

// hsRuntimeJSONRows —— py `runtime_json_rows()`：现跑 `--json commands --dry-run` ⇒ (items, 是否取不到, 理由)。
//
//	★ JSON null 面（§标准）：缺键与显式 null 在 py 都走 `.get("items") or []` ⇒ 都成 `[]`，本件逐字对齐。
func hsRuntimeJSONRows(binz string) ([]interface{}, bool, string) {
	if !hsIsFile(binz) {
		return nil, true, "`bin/zerg` 不在盘"
	}
	stdout, rc, ok := hsRunArtifact(binz, "help", "export", "--json", "commands", "--dry-run")
	if !ok {
		return nil, true, "现跑 `--json commands` 起不来"
	}
	if rc != 0 {
		return nil, true, fmt.Sprintf("现跑 `--json commands` 退码 %d", rc)
	}
	var objs []string
	for _, l := range hsSplitLines(stdout) {
		if strings.HasPrefix(hsPyTrim(l), "{") {
			objs = append(objs, l)
		}
	}
	if len(objs) == 0 {
		return nil, true, "现跑没有机器面那一行 JSON"
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(objs[len(objs)-1]), &m); err != nil {
		return nil, true, fmt.Sprintf("机器面 JSON 读不回来（%s）", err.Error())
	}
	val := m["items"]
	if val == nil { // 缺键 或 显式 null —— py 两态都成 []
		return []interface{}{}, false, ""
	}
	if arr, isArr := val.([]interface{}); isArr {
		return arr, false, ""
	}
	// 非数组且非空：py 会在此裸回溯（`it.get` 打在 str/float 上 ⇒ AttributeError）。
	// 甲案口径：py 裸回溯 ⇒ fail-closed（取不到）· 口径差列入未覆盖面。
	return nil, true, "机器面 `items` 不是数组（py 于此裸回溯 ⇒ fail-closed）"
}

// ── A 档：只读**落盘制品**（py `ARTIFACT_HUMAN` / `ARTIFACT_JSON` · 缺口 `GAP-20261001-47` 拆档）──
// 与 `hsRuntimeRows` / `hsRuntimeJSONRows` 的**唯一**不同：不吃子进程、只读一件文件 ⇒ 结论逐字可复现。
const hsArtifactHuman = "help-export-dry-run.txt"
const hsArtifactJSON = "help-export-json-commands.txt"

// hsRuntimeRowsFromFile —— py `runtime_rows_from_file()`：读**落盘制品** ⇒ (逐条清单行 | None, 理由)。
//
//	抽行口径与 `hsRuntimeRows` 逐字同一正则（`hsRuntimeRE`）—— 两档不许分家。
//	✗ 非法 UTF-8：py `open(encoding="utf-8").read()` 裸回溯（UnicodeDecodeError ⇒ rc=1）⇒ 本件走哨兵。
func hsRuntimeRowsFromFile(path string) ([]string, string) {
	if !hsIsFile(path) {
		return nil, "现跑制品不在盘：" + path + "（先落一份 `help export --dry-run` 输出再判）"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "现跑制品读不了（" + hsOSErrorStr(err, path) + "）"
	}
	if !utf8.Valid(raw) {
		return nil, hsDecodeSentinel + path
	}
	var rows []string
	for _, l := range hsSplitLines(hsFoldNewlines(string(raw))) {
		if m := hsRuntimeRE.FindStringSubmatch(l); m != nil {
			rows = append(rows, hsPyTrim(m[1]))
		}
	}
	if len(rows) == 0 {
		return nil, "制品里一行 `zerg …` 清单都没有（制品不对 / 输出格式改了？）"
	}
	return rows, ""
}

// hsRuntimeJSONRowsFromFile —— py `runtime_json_rows_from_file()`：读**落盘制品**的机器面那一行 JSON ⇒ (items, 是否取不到, 理由)。
func hsRuntimeJSONRowsFromFile(path string) ([]interface{}, bool, string) {
	if !hsIsFile(path) {
		return nil, true, "现跑制品不在盘：" + path + "（先落一份 `help export --json commands --dry-run` 输出）"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, true, "现跑制品读不了（" + hsOSErrorStr(err, path) + "）"
	}
	if !utf8.Valid(raw) {
		return nil, true, hsDecodeSentinel + path
	}
	var objs []string
	for _, l := range hsSplitLines(hsFoldNewlines(string(raw))) {
		if strings.HasPrefix(hsPyTrim(l), "{") {
			objs = append(objs, l)
		}
	}
	if len(objs) == 0 {
		return nil, true, "制品里没有机器面那一行 JSON"
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(objs[len(objs)-1]), &m); err != nil {
		return nil, true, fmt.Sprintf("机器面 JSON 读不回来（%s）", err.Error())
	}
	val := m["items"]
	if val == nil { // 缺键 或 显式 null —— py 两态都成 []
		return []interface{}{}, false, ""
	}
	if arr, isArr := val.([]interface{}); isArr {
		return arr, false, ""
	}
	// 非数组且非空：py 于 check_items_named 处裸回溯（`it.get` 打在 str/float 上 ⇒ AttributeError）。
	// 甲案口径：py 裸回溯 ⇒ fail-closed（取不到）· 口径差列入未覆盖面。
	return nil, true, "机器面 `items` 不是数组（py 于此裸回溯 ⇒ fail-closed）"
}

// hsSnapshotRows —— py `snapshot_rows()`：快照 §一 表行 ⇒ (命令名行 | None, 理由)。
func hsSnapshotRows(path string) ([]string, string) {
	if !hsIsFile(path) {
		return nil, "快照不在盘：" + path
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "快照读不了（" + hsOSErrorStr(err, path) + "）"
	}
	// py `open(encoding="utf-8").read()` 遇非法 UTF-8 ⇒ **UnicodeDecodeError 裸回溯**（非 OSError），
	// 退码 1 · stdout 只剩已写下的前言两行。甲案口径：py 裸回溯 ⇒ fail-closed ⇒ 本件同走 rc=1，
	// stderr 走**声明文案**（✗ 不写死跑测机路径）。判定用 `utf8.Valid`（py 的严格解码等价面）。
	if !utf8.Valid(raw) {
		return nil, hsDecodeSentinel + path
	}
	text := hsFoldNewlines(string(raw))
	var rows []string
	for _, l := range hsSplitLines(text) {
		if m := hsSnapshotRE.FindStringSubmatch(l); m != nil {
			rows = append(rows, hsPyTrim(m[1]))
		}
	}
	if len(rows) == 0 {
		return nil, "快照里一行 `zerg …` 表行都没有（§一 表被改坏了？）"
	}
	return rows, ""
}

// hsOSErrorStr —— py `OSError` 的 str 形态（已覆盖：权限拒绝 ⇒ `[Errno 13] Permission denied: '<路径>'`）。
//
//	为什么这么写：py 读面用 `except OSError as e` 把 e 原样印进判词 ⇒ 要逐字对齐就得复刻它的文案。
//	✗ 不写死任何跑测机路径：路径来自运行期入参。
func hsOSErrorStr(err error, path string) string {
	if os.IsPermission(err) {
		return "[Errno 13] Permission denied: '" + path + "'"
	}
	if os.IsNotExist(err) {
		return "[Errno 2] No such file or directory: '" + path + "'"
	}
	return err.Error()
}

// hsNewestSnapshot —— py `newest_snapshot()`：文档仓里最新的快照（按 (mtime, 路径) 最大）。
//
//	行序决定者：**不是**遍历序 —— py `hits.sort()` 按 (mtime, 路径) 排序取末项 ⇒ 遍历序不进输出。
//	本件仍按 os.walk 形态（readdir 原序 · 先当前层文件后按 dirs 原序下钻）枚举，但结论由排序定。
func hsNewestSnapshot(docsRoot string) (string, string) {
	type hit struct {
		mtime int64
		path  string
	}
	var hits []hit
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			return
		}
		ents, err := f.ReadDir(-1) // -1 ⇒ 一次读全，保持 readdir 原序
		_ = f.Close()
		if err != nil {
			return
		}
		var dirs, files []string
		for _, e := range ents {
			name := e.Name()
			// 跟随链：用 os.Stat 判目录（✗ 非 e.IsDir()）
			p := hsPyJoin(dir, name)
			if fi, serr := os.Stat(p); serr == nil && fi.IsDir() {
				dirs = append(dirs, name)
			} else {
				files = append(files, name)
			}
		}
		// 先产出当前层**全部文件**
		for _, fn := range files {
			if strings.HasPrefix(fn, hsSnapGlob) && strings.HasSuffix(fn, ".md") {
				full := hsPyJoin(dir, fn)
				if fi, serr := os.Stat(full); serr == nil {
					hits = append(hits, hit{mtime: fi.ModTime().UnixNano(), path: full})
				}
			}
		}
		// 再按 dirs **原序**下钻（✗ 早钻）
		for _, dn := range dirs {
			switch dn {
			case ".git", "node_modules", "__pycache__":
				continue
			}
			walk(hsPyJoin(dir, dn))
		}
	}
	walk(docsRoot)
	if len(hits) == 0 {
		return "", "文档仓里找不到快照（`" + hsSnapGlob + "*.md`）—— 先跑 `zerg help export`"
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].mtime != hits[j].mtime {
			return hits[i].mtime < hits[j].mtime
		}
		return hits[i].path < hits[j].path
	})
	return hits[len(hits)-1].path, ""
}

// hsCompareLines —— py `compare_lines()`：逐行比 ⇒ (相等吗, 判词表)。
func hsCompareLines(snapRows, liveRows []string) (bool, []string) {
	var bad []string
	if len(snapRows) != len(liveRows) {
		bad = append(bad, fmt.Sprintf("逐行对拍：行数不等 —— 快照 %d 行 / 现跑 %d 行", len(snapRows), len(liveRows)))
	}
	for i := 0; i < len(snapRows) && i < len(liveRows); i++ {
		a, b := snapRows[i], liveRows[i]
		if a != b {
			bad = append(bad, fmt.Sprintf("逐行对拍：第 %d 行不同 —— 快照 `zerg %s` ⟷ 现跑 `zerg %s`", i+1, a, b))
			if len(bad) >= 10 {
				bad = append(bad, "（只报前 10 处差异）")
				break
			}
		}
	}
	return len(bad) == 0, bad
}

// hsItemsGet —— `it.get(key, "")` 的逐字复刻：缺键 ⇒ ""（默认）· 显式 null ⇒ "None"。
func hsItemsGet(it interface{}, key string) (interface{}, bool) {
	m, ok := it.(map[string]interface{})
	if !ok {
		return nil, false
	}
	v, present := m[key]
	if !present {
		return "", true
	}
	return v, true
}

// hsCheckItemsNamed —— py `check_items_named()`。
func hsCheckItemsNamed(items []interface{}) []string {
	if len(items) == 0 {
		return []string{"`items` 是空的（零条 ⇒ 不许当绿）"}
	}
	var bad []string
	for i, it := range items {
		v, _ := hsItemsGet(it, "command")
		if hsPyTrim(hsPyStr(v)) == "" {
			bad = append(bad, fmt.Sprintf("第 %d 条 `items` 没有 `command`（逐条指不到是哪条）", i+1))
			if len(bad) >= 10 {
				bad = append(bad, "（只报前 10 条）")
				break
			}
		}
	}
	allSummary := true
	for _, it := range items {
		v, _ := hsItemsGet(it, "commands")
		if hsPyTrim(hsPyStr(v)) == "" {
			allSummary = false
			break
		}
	}
	if len(items) > 0 && !allSummary {
		bad = append(bad, "有 `items` 丢了摘要格 `commands`（字段只增不改 ⇒ 一个都不许丢）")
	}
	return bad
}

// hsListRules —— py `LIST_RULES`（逐字）。
const hsListRules = `判据（两半 · 波6 序70）
------------------------
  ① items_named：现跑 ` + "`bin/zerg help export --json commands --dry-run`" + ` ⇒ 机器面 ` + "`items`" + `
     逐条含非空 ` + "`command`" + `（行身份）· 摘要格 ` + "`commands`" + ` 一个都不丢（字段只增不改 §九 M6）
  ② line_by_line：成文快照（` + "`导出-命令面帮助-<日期>.md`" + ` · 产物 · §一 表行）⟷ 现跑
     ` + "`bin/zerg help export --dry-run`" + `（逐条清单行）—— 逐行比（条数相等 + 第 i 行同名）

不假装覆盖（照实登记）
----------------------
  · 只对拍**命令名**那一列（说明 / 字段面 / 端点 / 层级的逐字对拍不在本格）
  · 快照取「文档仓里 mtime 最新的那一份」；要钉版用 ` + "`--snapshot`" + `
  · 本件**不挂门**（只单跑）—— 挂门那一半 = 另一条（不在本笔）

退码
----
  0 = 两半都合 · 1 = 判红（逐行不一致 / 逐条缺名 / 条数不等）· 2 = 不给结论（缺制品 / 缺快照 / 零行）
`

// hsSelfTest —— py `self_test()`：15 格（1 正控性质 + 2 判红性质 + 逐行/条数 + items 四态 + A 档四态）。
//
//	★ 泄漏修复落点：py 原件用 `tempfile.mkdtemp(prefix=...)` 且**全件 0 处回收** ⇒ 每跑一枚自检
//	  就在 TMPDIR 堆下一枚同名临时目录。本件**自带 `defer os.RemoveAll`** ⇒ 跑测前后计数不变。
func hsSelfTest(out io.Writer) bool {
	total, failed := 0, 0
	casef := func(name string, want, got interface{}) {
		total++
		ok := hsPyRepr(want) == hsPyRepr(got)
		if !ok {
			failed++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(out, "  %s %s（期望 %s · 实得 %s）\n", mark, name, hsPyRepr(want), hsPyRepr(got))
	}

	tmp, err := os.MkdirTemp("", "hsnap-selftest-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp) // ★ 回收：py 原件缺这一句（见件头）

	good := hsPyJoin(tmp, "导出-命令面帮助-20260924.md")
	_ = os.WriteFile(good, []byte("# 导出：命令面帮助（自动生成 · 勿手改）\n\n## 一、命令清单\n\n"+
		"| 命令 | 说明 |\n|---|---|\n"+
		"| `zerg agent ls` | 子端清单 |\n"+
		"| `zerg gap ls` | 缺口账 |\n"), 0o644)

	rows, _ := hsSnapshotRows(good)
	casef("① 快照行抽得出来（两条）", []string{"agent ls", "gap ls"}, rows)
	_, why := hsSnapshotRows(hsPyJoin(tmp, "nope.md"))
	casef("① 快照不在盘 ⇒ 判不了", true, strings.Contains(why, "不在盘"))

	litLines := []string{"  zerg agent ls ｜ 茧壁:node ｜ 子端清单", "  zerg gap ls ｜ 茧壁:host ｜ 缺口账"}
	var got2 []string
	for _, l := range litLines {
		if m := hsRuntimeRE.FindStringSubmatch(l); m != nil {
			got2 = append(got2, hsPyTrim(m[1]))
		}
	}
	casef("② 逐行：抽得出（前缀 `  zerg `）", []string{"agent ls", "gap ls"}, got2)

	live := []string{"agent ls ｜ 茧壁:node ｜ 子端清单", "gap ls ｜ 茧壁:host ｜ 缺口账"}
	ok, bad := hsCompareLines(rows, live)
	casef("② 逐行：不同 ⇒ 判红（快照说的是哪些行）", false, ok)
	hit1 := false
	for _, b := range bad {
		if strings.Contains(b, "第 1 行") {
			hit1 = true
		}
	}
	casef("② 逐行：判词点名到第 1 行", true, hit1)
	ok2, _ := hsCompareLines(rows, rows)
	casef("② 逐行：同名同序 ⇒ 合", true, ok2)
	ok3, _ := hsCompareLines(rows, append(append([]string{}, rows...), "extra"))
	casef("② 逐行：条数不等 ⇒ 判红", false, ok3)

	itemsOK := []interface{}{
		map[string]interface{}{"command": "zerg a", "commands": "9"},
		map[string]interface{}{"command": "zerg b", "commands": "9"},
	}
	casef("③ items 逐条含命令名 ⇒ 合", []string{}, hsCheckItemsNamed(itemsOK))
	casef("③ items 缺命令名 ⇒ 判红", true, len(hsCheckItemsNamed([]interface{}{map[string]interface{}{"commands": "9"}})) > 0)
	casef("③ items 丢了摘要格 ⇒ 判红", true, len(hsCheckItemsNamed([]interface{}{map[string]interface{}{"command": "zerg a"}})) > 0)
	casef("③ items 零条 ⇒ 判红（不许当绿）", true, len(hsCheckItemsNamed([]interface{}{})) > 0)

	// ── A 档（`GAP-20261001-47` · 拆档）：只读落盘制品、不起子进程 ──────────────
	art := hsPyJoin(tmp, "artifact")
	_ = os.MkdirAll(art, 0o755)
	_ = os.WriteFile(hsPyJoin(art, hsArtifactHuman),
		[]byte("  zerg agent ls ｜ 茧壁:node ｜ 子端清单\n  zerg gap ls ｜ 茧壁:host ｜ 缺口账\n"), 0o644)
	_ = os.WriteFile(hsPyJoin(art, hsArtifactJSON),
		[]byte("{\"schema\":\"zerg-gate/v1\",\"items\":[{\"command\":\"zerg agent ls\",\"commands\":\"9\"}]}\n"), 0o644)
	frows, _ := hsRuntimeRowsFromFile(hsPyJoin(art, hsArtifactHuman))
	casef("A 档 ①制品逐条清单读得出（与现跑同正则）", []string{"agent ls", "gap ls"}, frows)
	fitems, _, _ := hsRuntimeJSONRowsFromFile(hsPyJoin(art, hsArtifactJSON))
	fcmds := []string{}
	for _, it := range fitems {
		if m, isMap := it.(map[string]interface{}); isMap {
			if c, _ := m["command"].(string); c != "" {
				fcmds = append(fcmds, c)
			}
		}
	}
	casef("A 档 ②制品机器面 items 读得出", []string{"zerg agent ls"}, fcmds)
	_, w3 := hsRuntimeRowsFromFile(hsPyJoin(art, "nope.txt"))
	casef("A 档 ③制品不在盘 ⇒ 判不了（不许当空清单）", true, strings.Contains(w3, "不在盘"))
	_, w4 := hsRuntimeRowsFromFile(hsPyJoin(art, hsArtifactJSON))
	casef("A 档 ④制品里没有清单行 ⇒ 判不了", true, w4 != "")

	fmt.Fprintf(out, "  自检 %d 格 · 失败 %d 格\n", total, failed)
	return failed == 0
}

// ── 参数解析（py argparse 的已覆盖面）────────────────────────────────────────────
type hsArgs struct {
	repoRoot        string
	repoSet         bool
	docsRoot        string
	docsSet         bool
	snapshot        string
	snapSet         bool
	runtimeArtifact string
	runtimeSet      bool
	json            bool
	listRules       bool
	selfTest        bool
	badArg          string // 未知旗标 / 缺值 ⇒ 用法错 rc=2
}

func hsParseArgs(argv []string) hsArgs {
	var a hsArgs
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		val := func() (string, bool) {
			if j := strings.Index(s, "="); j >= 0 {
				return s[j+1:], true
			}
			if i+1 < len(argv) {
				i++
				return argv[i], true
			}
			return "", false
		}
		switch {
		case s == "--json":
			a.json = true
		case s == "--list-rules":
			a.listRules = true
		case s == "--self-test":
			a.selfTest = true
		case s == "--repo-root" || strings.HasPrefix(s, "--repo-root="):
			if v, ok := val(); ok {
				a.repoRoot, a.repoSet = v, true
			} else {
				a.badArg = s
			}
		case s == "--docs-root" || strings.HasPrefix(s, "--docs-root="):
			if v, ok := val(); ok {
				a.docsRoot, a.docsSet = v, true
			} else {
				a.badArg = s
			}
		case s == "--snapshot" || strings.HasPrefix(s, "--snapshot="):
			if v, ok := val(); ok {
				a.snapshot, a.snapSet = v, true
			} else {
				a.badArg = s
			}
		case s == "--runtime-artifact" || strings.HasPrefix(s, "--runtime-artifact="):
			if v, ok := val(); ok {
				a.runtimeArtifact, a.runtimeSet = v, true
			} else {
				a.badArg = s
			}
		default:
			a.badArg = s
		}
	}
	return a
}

// hsDefaultRoot —— 声明 (a)：从 cwd 上溯 ≤6 层找含 `scripts/gates/` 的目录（py 用 `__file__` 上溯三级）。
func hsDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	start, err := filepath.Abs(wd)
	if err != nil {
		return wd
	}
	d := start
	for i := 0; i < 6; i++ {
		if fi, serr := os.Stat(hsPyJoin(d, "scripts/gates")); serr == nil && fi.IsDir() {
			return d
		}
		parent := hsPyDirname(d)
		if parent == d || parent == "" {
			break
		}
		d = parent
	}
	return wd
}

// hsJSONDumps —— py `json.dumps(obj, ensure_ascii=False)` 的逐字复刻（默认分隔符 `, ` / `: `）。
//
//	仅用于本门唯一一条机器面输出；键序按插入序逐字摆。
func hsJSONDumpsString(s string) string {
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

func hsJSONDumpsBad(bad []string) string {
	parts := make([]string, len(bad))
	for i, s := range bad {
		parts[i] = hsJSONDumpsString(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// checkHelpSnapshotGateMain —— 入口（= py `main()`）。
func checkHelpSnapshotGateMain(stdout, stderr io.Writer, argv []string) int {
	a := hsParseArgs(argv)
	if a.badArg != "" {
		fmt.Fprintf(stderr, "usage: check-help-snapshot [--repo-root DIR] [--docs-root DIR] [--snapshot FILE] [--runtime-artifact DIR] [--json] [--list-rules] [--self-test]\n")
		fmt.Fprintf(stderr, "check-help-snapshot: error: 未知参数 %s（本入口只保「未知项 ⇒ 退码 2」 · 帮助正文不列）\n", a.badArg)
		return 2
	}
	if a.listRules {
		fmt.Fprint(stdout, hsListRules)
		return 0
	}
	if a.selfTest {
		fmt.Fprint(stdout, "── 自检（合成夹具 · 不碰真目标）──\n")
		if hsSelfTest(stdout) {
			return 0
		}
		return 2
	}

	repoRoot := a.repoRoot
	if !a.repoSet {
		repoRoot = hsDefaultRoot()
	}
	binz := hsPyJoin(hsPyJoin(repoRoot, "bin"), "zerg")
	docsRoot := a.docsRoot
	if !a.docsSet {
		docsRoot = hsPyJoin(hsPyDirname(repoRoot), "Zerg-内部文档")
	}
	fmt.Fprintf(stdout, "帮助成文快照 ↔ 运行期逐行对拍（波6 序70）· v%s\n", hsVersion)
	fmt.Fprint(stdout, "  判据见 `--list-rules`：① items 逐条含命令名 · ② 快照 ⟷ 现跑 逐行\n")

	var items []interface{}
	var itemsNone bool
	var jwhy, lwhy string
	var live []string
	if a.runtimeArtifact != "" {
		// A 档（`GAP-20261001-47` · 拆档）：只读落盘制品 · **不起子进程** ⇒ 结论逐字可复现
		// （与 HEAD 处编译制品解耦）。缺该档 ⇒ 行为一字不动（B 档：现跑 bin/zerg）。
		fmt.Fprintf(stdout, "  取数档 = A 档（只读落盘制品 %s）—— 不起 bin/zerg 子进程\n", a.runtimeArtifact)
		items, itemsNone, jwhy = hsRuntimeJSONRowsFromFile(hsPyJoin(a.runtimeArtifact, hsArtifactJSON))
		live, lwhy = hsRuntimeRowsFromFile(hsPyJoin(a.runtimeArtifact, hsArtifactHuman))
	} else {
		items, itemsNone, jwhy = hsRuntimeJSONRows(binz)
		live, lwhy = hsRuntimeRows(binz)
	}
	if strings.HasPrefix(jwhy, hsDecodeSentinel) || strings.HasPrefix(lwhy, hsDecodeSentinel) {
		// py 裸回溯面（rc=1 · stderr 走声明文案 · stdout 只剩已写下的前言 + A 档行）
		fmt.Fprintf(stderr, "UnicodeDecodeError: 'utf-8' codec can't decode byte 0x?? in position ??: invalid start byte\n")
		fmt.Fprintln(stderr, "  （py 原件在此裸回溯 ⇒ 退码 1；本入口对齐退码与 stdout，stderr 文案为声明式）")
		return 1
	}
	if live == nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", lwhy)
		return 2
	}
	snapPath := a.snapshot
	if !a.snapSet {
		var swhy string
		snapPath, swhy = hsNewestSnapshot(docsRoot)
		if snapPath == "" {
			fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", swhy)
			return 2
		}
	}
	snap, snwhy := hsSnapshotRows(snapPath)
	if snap == nil {
		if strings.HasPrefix(snwhy, hsDecodeSentinel) {
			// py 裸回溯面（rc=1 · stderr 走声明文案 · stdout 只剩前言两行）
			fmt.Fprintf(stderr, "UnicodeDecodeError: 'utf-8' codec can't decode byte 0x?? in position ??: invalid start byte\n")
			fmt.Fprintln(stderr, "  （py 原件在此裸回溯 ⇒ 退码 1；本入口对齐退码与 stdout，stderr 文案为声明式）")
			return 1
		}
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", snwhy)
		return 2
	}
	fmt.Fprintf(stdout, "  快照 = %s（%d 行）· 现跑 = %d 行\n", snapPath, len(snap), len(live))

	var bad []string
	var itemBad []string
	if !itemsNone {
		itemBad = hsCheckItemsNamed(items)
	}
	if itemsNone {
		bad = append(bad, fmt.Sprintf("① `--json commands` 现跑读不回来（%s）⇒ 这一半判不了", jwhy))
	}
	for _, b := range itemBad {
		bad = append(bad, "① "+b)
	}
	ok, lbad := hsCompareLines(snap, live)
	for _, b := range lbad {
		bad = append(bad, "② "+b)
	}
	if len(bad) > 0 {
		fmt.Fprint(stdout, "⇒ 判红（rc=1）：\n")
		for _, b := range bad {
			fmt.Fprintf(stdout, "  ✗ %s\n", b)
		}
	} else {
		fmt.Fprintf(stdout, "  ✓ ① items %d 条逐条含命令名（摘要格 `commands` 一条不丢）\n", len(items))
		fmt.Fprintf(stdout, "  ✓ ② 逐行对拍 %d 行 ⇒ 一致\n", len(snap))
		fmt.Fprint(stdout, "⇒ 两半都合（rc=0）\n")
	}
	if a.json {
		itemsTotal := "null"
		itemsNamed := "null"
		if !itemsNone {
			itemsTotal = strconv.Itoa(len(items))
			itemsNamed = "true"
			if len(itemBad) > 0 {
				itemsNamed = "false"
			}
		}
		fetchModeTail := ""
		if a.runtimeArtifact != "" {
			fetchModeTail = `, "fetch_mode": "artifact"`
		}
		fmt.Fprintf(stdout, `{"schema": "zerg-gate/v1", "gate": "check-help-snapshot", "version": %s, `+
			`"snapshot": %s, "snapshot_rows": %d, "live_rows": %d, "items_total": %s, `+
			`"items_named": %s, "line_by_line": %s, "bad": %s, "mode": "judge"%s}`+"\n",
			hsJSONDumpsString(hsVersion), hsJSONDumpsString(snapPath), len(snap), len(live),
			itemsTotal, itemsNamed, hsJSONBool(ok), hsJSONDumpsBad(bad), fetchModeTail)
	}
	if len(bad) > 0 {
		return 1
	}
	return 0
}

func hsJSONBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
