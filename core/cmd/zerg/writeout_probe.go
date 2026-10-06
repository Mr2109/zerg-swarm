// writeout_probe.go —— 探针件 `scripts/sandbox-probes/writeout-probe.py`（17 行）的 **Go 真搬**
// （v2.5.14 · M2 · 探针件搬运 · 批-5 粒 A · 入口 `writeoutProbeGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
// \tM2 = 按价值**真搬 Go**（逐件搬 · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 env 跑两态
// \t（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的新口径差
// \t与未覆盖面）。本单**只落码 · 不接线**（✗ 不注册命令 · ✗ 不改 main.go · ✗ 不删 py 原件）。
//
// 选它的理由（本单口径：探针/静态分析类 · ⑤ 可否 Go 化 = 可 —— 现读全文 17 行坐实）：
//
// \t✓ 纯探针：只读两枚环境变量、做两次写尝试、打一行 JSON —— **零 subprocess · 零联网 ·
// \t  零真 TTY · 零计时 · 零随机 · 零 argv**（py 是裸脚本，不读 `sys.argv`）。
// \t✓ 唯一写面 = `PROBE_DIR`（件自己点建后写 `inside.txt`）＋ `PROBE_OUTSIDE`（写一探）
// \t  —— 两枚**皆由调用方传入**，仓外夹具里指向夹具目录 ⇒ 全格自回收（件自身不写任何固定路径）。
// \t✓ 退码面简单：0 正常出 JSON；取不到输入档按新口径 fail-closed 2。
// \t✗ 未选同族的 `jitprobe.py` / `miniprobe.py` / `conn_jit.py` / `lx2-linux.py`：普查表 ⑤ 判
// \t  「不可」（`mmap` RWX/mprotect、socket bind/egress、Linux 沙箱具名语义 —— 平台具名面不可搬）。
//
// 出厂声明行（每枚必写）：
//
// \t(a) 默认根定位差异：py 只读 `os.environ`，**无 `__file__` 上溯** ⇒ 本件无「脚本位置 ≠ cwd」
// \t    的路径推导面，此条**不适用**（如实说明，不是漏写）。
// \t(b) 自指检查：本件**不落在**任何他门扫描面内 —— 现读件内**零绝对路径字面量**、零 `setErr(`
// \t    调用（`check-error-kinds` 的 R3 只认错误构造调用首参，且其注释明言不扫 JSON 字面）、
// \t    零茧壁/私有路径形态标记。JSON 键名（`write_inside` 等）是**输出信封标签**，不在任何
// \t    被扫闭集里。落件后以「同根串影子仓」复核（存/不存两态他门读数 sha16 逐字相同）。
//
// 入口签名（与 py 脚本体对齐）：
//
// \tfunc writeoutProbeGateMain(stdout, stderr io.Writer, argv []string) int
// \t  · argv  —— py 原件**不读 argv**（裸脚本）⇒ 本件同样忽略 argv。
// \t  · 退码  —— 0 正常（**含两条写尝试任一失败** · 仍出 JSON）/ 1 **崩溃档**（env 缺 ⇒ KeyError ·
// \t             makedirs 失败 ⇒ FileExistsError 一族）—— **逐字沿用 py 原件的裸回溯 rc=1**
// \t             （C 类最外层无兜底）。★ 口径：本件**不再走**「取不到输入 ⇒ fail-closed 2」那条通用
// \t             新口径 —— 本单 = 对齐口径，让 `check-py-go-pairing-drift` 对该键不再因本件而分叉。
//
// ★★ 口径差逐一声明（三面 · 本单现读实测）：
//
// \t① **rc 面 = 逐字同**（py 1 ⟷ 本件 1）。改前本件退 2 ⇒ 正是接线批跳过不接的成因。
// \t② **stdout 面 = 逐字同**（两态皆空 —— py 在 `print(json.dumps(...))` 之前就崩）。
// \t③ **stderr 面 = 不复刻（已声明）** —— py 的回溯正文里含**解释器面的帧**、且随解释器漂：
// \t   CPython 3.11+ 印 `File "<frozen os>", line 709, in __getitem__`；系统 /usr/bin/python3 3.9 印
// \t   `File "…/Versions/3.9/lib/python3.9/os.py", line 679, in __getitem__` ＋ `raise KeyError(key) from None`。
// \t   ⇒ 该帧**既不可从仓根推得、又随解释器版本/安装路径漂** ⇒ 逐字复刻等于把某一版解释器的行号/路径
// \t   写死进仓件（换环境必红；违 zerg-m2-go-port「帧落在解释器安装目录 ⇒ 只对齐 rc＋stdout」一条）
// \t   ⇒ 本件走**具名文案**（一行：`<异常类>: '<键>'（…）`），并逐条列此差异。
// \t   ★ 后果：`check-py-go-pairing-drift` 对该键的 **stderr 面仍会判「异」** —— 那是**门侧掩码缺口**
// \t     （回溯帧不在该门 `MASK_RULES` 覆盖面内），**不是 py⟷Go 实现分叉**；✗ 本件不迁就它、✗ 不改棘轮值。
//
// ═══ 逐字对齐的 py 语义（现读逐点交代） ═══
//
// \t① `os.environ["PROBE_DIR"]` / `["PROBE_OUTSIDE"]`：缺键 ⇒ KeyError（py 裸回溯 rc=1）。
// \t② `os.makedirs(w, exist_ok=True)`：建 w 含父；失败 ⇒ py 裸回溯 rc=1。
// \t③ `open(os.path.join(w,"inside.txt"),"w").write("x")`：成功 ⇒ `"ok"`；异常 ⇒ `"FAIL:"+type(e).__name__`。
// \t④ `open(outside,"w").write("x")`：成功 ⇒ `"OPEN"`；异常 ⇒ `"BLOCKED:"+type(e).__name__`。
// \t⑤ `print(json.dumps(r))`：`json.dumps` 缺省分隔符 `", "` / `": "`、**键序 = 插入序**、
// \t  `ensure_ascii=True`（非 ASCII 一律 `\uXXXX`）⇒ 本件自写 `wopPyJSONStr` 逐字复刻。
// \t⑥ `os.path.join` 语义 ⇒ 自写 `wopPyJoin`（✗ 不用 `filepath.Join`；本件只拼单段相对名，
// \t  三类语义差不触发，仍照写**恒同构**以守纪律）。
// \t⑦ `os.makedirs(w, exist_ok=True)` 失败档的**errno 类名要按 py 纠**：py 先 `mkdir(name)` 抛
// \t  `OSError`、再用 `path.isdir(name)` 决定吞不吞 ⇒ 「w 存在且**非**目录」透传 **FileExistsError**；
// \t  而 Go `os.MkdirAll` 在同一档给 **ENOTDIR**（NotADirectoryError）⇒ 本件按 py 语义纠回 EEXIST
// \t  （`wopMakedirsPy`）。★ 残留近似：**父段是普通文件**（w 本身不存在）时 py 仍给 FileExistsError、
// \t  本件给 NotADirectoryError ⇒ 该格未复刻（列未覆盖面）。
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"syscall"
)

// wopPyJoin —— `posixpath.join(a, b)` 的逐字同构（照 CPython 逐段规则直译）。✗ 不用 filepath.Join。
func wopPyJoin(a, b string) string {
	if strings.HasPrefix(b, "/") { // 后段绝对 ⇒ py 丢弃前面所有段
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

// wopPyJSONStr —— `json.dumps(<str>, ensure_ascii=True)` 的**字符串编码**逐字同构。
// CPython 的转义面：`"` `\` 走短转义，`\n\r\t\b\f` 短转义，其余 `[^ -~]`（含 0x7f 与全部非 ASCII）
// 一律 `\uXXXX`（>0xFFFF 走代理对）⇒ 逐码点迭代。
func wopPyJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			switch {
			case r >= 0x20 && r <= 0x7e: // 可打印 ASCII 原样
				b.WriteRune(r)
			case r <= 0xffff:
				fmt.Fprintf(&b, `\u%04x`, r)
			default: // 星面之外 ⇒ UTF-16 代理对（与 ensure_ascii 同构）
				r -= 0x10000
				fmt.Fprintf(&b, `\u%04x\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// wopTracebackNotReproduced —— 口径差③的统一尾注（py 回溯正文不复刻：帧落在解释器面 ⇒ 不可移植）。
const wopTracebackNotReproduced = "（py 真身裸回溯 rc=1 · 回溯帧落在解释器面（<frozen os> / os.py）⇒ 正文不复刻 · 见件头口径差③）"

// wopKeyError —— `os.environ[<键>]` 缺键档：对齐 py 的 `KeyError` ＋ **rc=1**（✗ 不 fail-closed 2）。
func wopKeyError(stderr io.Writer, key string) int {
	fmt.Fprintf(stderr, "KeyError: '%s'%s\n", key, wopTracebackNotReproduced)
	return 1
}

// wopMakedirsPy —— `os.makedirs(w, exist_ok=True)` 的失败档同构（errno 类名按 py 语义纠，见件头⑦）。
func wopMakedirsPy(w string) error {
	err := os.MkdirAll(w, 0o777)
	if err == nil {
		return nil
	}
	// py：`mkdir(name)` 抛 OSError，而 exist_ok=True 只在 `path.isdir(name)` 为真时才吞
	// ⇒ 「w 存在且**非**目录」透传的是 **FileExistsError**；Go 的 MkdirAll 在该档给 ENOTDIR ⇒ 纠回。
	if fi, serr := os.Stat(w); serr == nil && !fi.IsDir() {
		return &os.PathError{Op: "mkdir", Path: w, Err: syscall.EEXIST}
	}
	return err
}

// wopPyErrName —— `type(e).__name__` 的同构映射（只覆盖本件真会触发的 errno 档）。
// 顺序要紧：EISDIR/ENOTDIR 先于 ENOENT/权限档判（它们互斥，但先窄后宽更稳）。
func wopPyErrName(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, syscall.EISDIR):
		return "IsADirectoryError"
	case errors.Is(err, syscall.ENOTDIR):
		return "NotADirectoryError"
	case errors.Is(err, fs.ErrNotExist):
		return "FileNotFoundError"
	case errors.Is(err, fs.ErrPermission):
		return "PermissionError"
	case errors.Is(err, fs.ErrExist):
		return "FileExistsError"
	default:
		return "OSError"
	}
}

// writeoutProbeGateMain —— 入口（rc 0 正常出 JSON / 2 取不到输入 · 不给结论）。
func writeoutProbeGateMain(stdout, stderr io.Writer, argv []string) int {
	_ = argv // py 原件是裸脚本、不读 argv ⇒ 照搬忽略（✗ 不照它起 argparse）

	w, ok := os.LookupEnv("PROBE_DIR") // ① 缺键 ⇒ KeyError（py 裸回溯 rc=1 ⇒ 本件对齐 rc=1）
	if !ok {
		return wopKeyError(stderr, "PROBE_DIR")
	}
	outside, ok := os.LookupEnv("PROBE_OUTSIDE")
	if !ok {
		return wopKeyError(stderr, "PROBE_OUTSIDE")
	}
	if err := wopMakedirsPy(w); err != nil { // ② 点建 w 含父（exist_ok=True ⇒ 失败 = 裸回溯 rc=1）
		fmt.Fprintf(stderr, "%s: '%s'%s\n", wopPyErrName(err), w, wopTracebackNotReproduced)
		return 1
	}

	inside := "ok" // ③
	if err := os.WriteFile(wopPyJoin(w, "inside.txt"), []byte("x"), 0o666); err != nil {
		inside = "FAIL:" + wopPyErrName(err)
	}
	out := "OPEN" // ④
	if err := os.WriteFile(outside, []byte("x"), 0o666); err != nil {
		out = "BLOCKED:" + wopPyErrName(err)
	}

	// ⑤ 逐字复刻 json.dumps：缺省分隔符 `", "` / `": "` · 键序 = 插入序
	fmt.Fprintf(stdout, "{\"write_inside\": %s, \"write_outside\": %s, \"write_outside_target\": %s}\n",
		wopPyJSONStr(inside), wopPyJSONStr(out), wopPyJSONStr(outside))
	return 0
}
