// check_archive_mount.go —— 判据件 `scripts/gates/check-archive-mount.py`（463 行）的 **Go 真搬**
// （v2.5.14 · M2 一枚 · 入口 `checkArchiveMountGateMain` · 承接 GAP-20261001-13）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳（`zerg gate check <判据名>`）；
//	M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。本件 = M2 一枚。
//	验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 本件与新口径（外部树改**显式入参旋钮**）：
//
//	py 的真源 = **仓外**一棵树（仓根同级的 `Zerg-归档`，另加挂载表 / `hdiutil info` 两处现读）。
//	本件按新口径把这层落成**显式入参旋钮**，旋钮名**照抄 py 已有的同名旗标**（不造新名）：
//	  `--root`（仓根）· `--path`（归档树根 = 原路径）· `--mount-table`（挂载表件）
//	  · `--mountpoint` · `--min-files` · `--sample` · `--hdiutil-info` · `--no-hdiutil`
//	  · env 旋钮 `ZERG_ARCHIVE_ROOT`（py 现读在册的唯一 env 名）。
//	  ① 旋钮**缺省值 = py 默认推导逐字同**（见 (a) 的逐字对比）—— 不传旋钮时与 py 逐字对齐（两态对拍已证）；
//	  ② **退 2 边界**：py 只在「**旋钮显式指的**归档树根在盘上完全取不到」一档给 rc=2；默认档（无旋钮）
//	     不判这一档（未挂**断链**是常态 = 本门 A3 核心格要判的形态）。本件**逐字沿用** ⇒ 只有「都取不到」
//	     才退 2，✗ 禁「任一取不到」——后者会让真仓默认档恒红（`../Zerg-归档` 现读就是断链）。
//	  ③ **取不到一律 rc=2，✗ 不静默绿**：仓根 `--root` 取不到 / 不是目录、挂载表读不到 / 0 行、两处读数打架、
//	     旋钮指空的树根 —— 四档全 2（仓根档走空集守卫，stderr `BLOCKED …` 与 py `require_domain` 逐字同）。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3194`（门㉛ · tri 档））；
//	· **不**删 `scripts/gates/check-archive-mount.py` 原件（退场规则：**对拍通过后才**退场）；
//	· **已接线**：入口 `checkArchiveMountGateMain` 由 `gateCheckGoEntry["check-archive-mount"]` 分派（`zerg gate check check-archive-mount`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `root = a.root or dirname(dirname(dirname(abspath(__file__))))`
//	    （**脚本位置**：`scripts/gates/<件>` 上溯三级 = 仓根）；本件**无 `__file__` 语境**
//	    ⇒ 不给 `--root` 时从 **cwd** 上溯找含 `scripts/gates` 的目录、找不到回落 `abspath(cwd)`。
//	    落在同一真仓、cwd=仓根（或 cwd 在仓内任意层）时两者读数逐字相同（已对拍甲/乙）；
//	    「脚本位置 ≠ cwd 上溯根」时路径推导面不同 ⇒ **不属逐字对齐承诺**，对拍一律 cwd=仓根 / 显式传 `--root`。
//	    **旋钮缺省值 vs py 默认推导逐字对比**：
//	      py   `path = explicit_arch or os.path.join(os.path.dirname(os.path.abspath(root)),
//	                                                  DEFAULT_ARCHIVE_NAME)`
//	      本件 `camDefaultPath(root)` = `filepath.Join(filepath.Dir(abs(root)), camDefaultArchiveName)`
//	    ⇒ **事实判断（是否需串拼）**：拼接的那一段是 `<root 的父目录> + "/" + "Zerg-归档"` ——
//	      拼接字符串里**没有 `..` 段**（`Zerg-归档` 是裸件名、父目录来自 `Dir` 已是 clean）⇒
//	      `filepath.Join` 只多做一个 `Clean`，而两侧输入本就 clean/abs **逐字同构**
//	      ⇒ **不需要**（也**不该**）用字符串串拼（✗ 前例 `check-idspace-map.go` 里那一段含 `..` 才必须串拼）。
//	    env 旋钮 `ZERG_ARCHIVE_ROOT` 与 `--path` 的关系逐字照 py：`--path` > env > 默认。
//	(b) **自指四形态（先真判 · 六发血证形态）**：
//	  ① 本件**在自己的扫描面内吗？—— 否**。py 的 `judge()` 全流程**零目录遍历、零 glob**：只
//	     `os.path.islink/exists/isdir/realpath/readlink`、只 `open()` 调用方给的单件（挂载表 / 样本件）、
//	     只跑 `mount` / `hdiutil info` 两处现读；`--self-test` 只在 `tempfile` 里造件。⇒ 本门**永不自扫本件**
//	     （量化依据：`os.walk` 仅 1 处、在 `count_files(path)` 内、对象恒为调用方给的 `path`）。
//	  ② 本件**落在「他门」扫描面内吗？—— 是**：`check-hardcoded-private-paths` 的 `SCAN_DIRS` 含 `core/`；
//	     `check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`（非 `_test.go`）⇒ 本件正文须按血证形态处置：
//	     **零私有绝对路径字面量**（盘根 / 家目录前缀一概不写；仓根与外部树一律运行时取）、
//	     **不写** `check-error-kinds` 被扫的两条用法形态（其「错误构造调用形」与「JSON 键字面量形」两条正则）、
//	     **不写**反引号包裹的门扫描标记原串。
//	  ③ 本件唯一的 `/Volumes/` 字面量 = `camDefaultMountpoint`（挂载点名 · 与 py 源逐字同）。
//	     量化依据：`check-hardcoded-private-paths` 的 NEEDLES = `/Volumes/` + 卷名 与 `/Users/` + 用户名
//	     （两个**实例**串，其原串在本件正文里**不出现**）⇒ `/Volumes/ZERG-ARCHIVE` **不含**任一 needle ⇒ 不命中。
//	  ④ 处置有效性**已证**：落仓前后 `check-hardcoded-private-paths` 与 `check-error-kinds` 两门输出
//	     逐字相同（sha16 落仓前后一致，见交付读数）⇒ 本件落仓**没有**给这两门添一个字节。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 长选项**前缀缩写**（如 `--min 3`⇒`--min-files`）本件不实现；`--opt=值` 等号形本件支持（与 py 同）。
//	② py `open(..., encoding="utf-8", errors="replace")` 的非法字节替换按**非法序列**逐段替 U+FFFD；
//	   本件按原始字节直出 ⇒ 仅当挂载表/样本件含非 UTF-8 字节时分叉（真机 `mount` 输出恒 ASCII ⇒ 不触发）。
//	③ `mount` / `hdiutil info` 两处**现读**本件同样真跑子进程 ⇒ 读数同源；但两态跑在不同时刻时读数可能变
//	   （对拍一律用 `--mount-table` / `--hdiutil-info` 夹具钉死）。
//	④ `os.walk` 的**符号链接目录**语义（列进 `dirs` 但不下降）本件照搬；`followlinks=False` 一致。
//	⑤ Python 版本 / 终端宽度差异不改本件（本门无 `--json`、无机器读面 ⇒ 退 2 用法错）。
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

const camProg = "check-archive-mount.py"

// camUsage —— argparse 80 列下的 usage 头（错误档与 `-h` 共用 · 逐字照 py 现读）。
const camUsage = `usage: check-archive-mount.py [-h] [--root ROOT] [--mount-table MOUNT_TABLE]
                              [--path PATH] [--mountpoint MOUNTPOINT]
                              [--min-files MIN_FILES] [--sample SAMPLE]
                              [--hdiutil-info HDIUTIL_INFO] [--no-hdiutil]
                              [--self-test] [--list-rules]
`

// camHelp —— `-h`/`--help` 全文（Python 3.9 argparse · 80 列 · 逐字固化）。
const camHelp = camUsage + `
optional arguments:
  -h, --help            show this help message and exit
  --root ROOT
  --mount-table MOUNT_TABLE
  --path PATH
  --mountpoint MOUNTPOINT
  --min-files MIN_FILES
  --sample SAMPLE
  --hdiutil-info HDIUTIL_INFO
  --no-hdiutil
  --self-test
  --list-rules
`

const (
	// camDefaultMountpoint —— 挂载点（三处硬编码同用这一个串 · 名字一动三处全穿）。
	camDefaultMountpoint = pathArchiveMountDefault
	// camDefaultArchiveName —— 原路径的**名字**（仓根同级的 `Zerg-归档`）。
	camDefaultArchiveName = "Zerg-归档"
	camDefaultSample      = "版次索引.md"
	camDefaultMinFiles    = 38
)

// camRules —— `--list-rules` 逐行（与 py RULES 表逐字同）。
var camRules = []string{
	"A1 挂载行（按挂载点路径取）必须含 `read-only`（判据 1 · 负控 ⑤⑦）",
	"A2 原路径必须是符号链接且 readlink == 挂载点（判据 4 · 负控 ④）",
	"A3 ★核心：未挂 ⇒ 读原路径必须**明确失败**（ENOENT）；解析到目录（含空）⇒ 红（判据 9 · 负控 ④）",
	"A4 挂了但读到空：件数必须 ≥ --min-files 且 > 0（判据 3 · 负控 ⑤⑥）",
	"A5 挂了但读到错件：样本件必须读得到且非空（判据 2 的同一族 · 负控 ⑧）",
	"A6 两处读数（mount 行 ⟷ hdiutil info）不一致 ⇒ rc=2（§三 3.3 交叉核对 · 负控 ⑥）",
}

// ── 判定袋（与 py Verdict 同形）────────────────────────────────────────
type camVerdict struct {
	reds    [][2]string // (码, 判词)
	notes   []string    // 只报不判（读数）
	noconcl []string    // 不给结论
}

func (v *camVerdict) red(code, msg string) { v.reds = append(v.reds, [2]string{code, msg}) }
func (v *camVerdict) note(msg string)      { v.notes = append(v.notes, msg) }
func (v *camVerdict) nc(msg string)        { v.noconcl = append(v.noconcl, msg) }

// ── 路径谓词（与 py os.path.* 同义）──────────────────────────────────
func camIsLink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func camExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func camIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// camArchiveRootAbsent —— 归档树根在盘上**完全取不到**（既不是符号链接、也不是实体档/目录）。
//
// 注意：**悬空符号链接不算缺席** —— 那是「未挂档」的常态（符号链接在、目标 = 挂载点不在），
// 正是本门 A3 核心格要判的形态（读它**必须明确失败** = 对的样子）。缺席 = 连符号链接都没有。
func camArchiveRootAbsent(p string) bool {
	return !camIsLink(p) && !camExists(p)
}

// camMountLine —— 按**挂载点路径**取那一行（不是按卷名）。
func camMountLine(text, mountpoint string) (string, bool) {
	needle := " on " + mountpoint + " ("
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, needle) {
			return strings.TrimRight(line, "\n"), true
		}
	}
	return "", false
}

var camReParen = regexp.MustCompile(`\(([^)]*)\)`)

// camMountRO —— 挂载行选项表里有没有 `read-only`（py: 取**第一对**圆括号内的逗号表）。
func camMountRO(line string) bool {
	m := camReParen.FindStringSubmatch(line)
	opts := ""
	if m != nil {
		opts = m[1]
	}
	for _, o := range strings.Split(opts, ",") {
		if strings.TrimSpace(o) == "read-only" {
			return true
		}
	}
	return false
}

// camHdiutilKnows —— `hdiutil info` 里认不认这个挂载点（两处读数的第二处）。
func camHdiutilKnows(mountpoint, infoText string) bool {
	return strings.Contains(infoText, mountpoint)
}

// camCountFiles —— 复刻 py `os.walk` 的**文件**计数：
// 目录（跟随软链）计入子目录、但**软链目录不下降**；非目录一律计 1（含悬空软链）。
func camCountFiles(path string) int {
	n := 0
	var walk func(dir string)
	walk = func(dir string) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range ents {
			full := filepath.Join(dir, e.Name())
			fi, serr := os.Stat(full) // 跟随软链判目录
			if serr == nil && fi.IsDir() {
				if e.Type()&os.ModeSymlink != 0 {
					continue // os.walk(followlinks=False)：软链目录列进 dirs 但不下降
				}
				walk(full)
				continue
			}
			n++
		}
	}
	walk(path)
	return n
}

// camJudge —— 纯函数式判定（合成夹具与真目标走同一条路 ⇒ 自检因此有区分度 · 逐字复刻 py judge()）。
func camJudge(v *camVerdict, mountText string, mountNone bool, path, mountpoint string,
	minFiles int, sample, hdiutilText string, hdiutilChecked bool) {
	if mountNone || strings.TrimSpace(mountText) == "" {
		v.nc("挂载表读不到 / 0 行 ⇒ 空转不给结论（工具：`mount`）")
		return
	}
	mountedLine, hasLine := camMountLine(mountText, mountpoint)

	// ---- 原路径那一格 ----
	isLink := camIsLink(path)
	exists := camExists(path)
	if isLink {
		target, _ := os.Readlink(path)
		v.note(fmt.Sprintf("原路径 %s = 符号链接 → %s（判据 4 现读）", path, target))
	} else if exists {
		v.note(fmt.Sprintf("原路径 %s = **实体目录**（判据 4 要求它是符号链接）", path))
	} else {
		v.note(fmt.Sprintf("原路径 %s 不存在 / 解析不到（fail-closed 的那一态）", path))
	}

	isdir := camIsDir(path)
	if !hasLine {
		// ---- 未挂态：核心判据 ----
		v.note(fmt.Sprintf("挂载表里没有 ` on %s (` 那一行 ⇒ **未挂档**（按需加载的常态）", mountpoint))
		switch {
		case isdir && !isLink:
			v.red("A3", fmt.Sprintf("未挂档但原路径解析成一个**目录**（%s）⇒ 读它会**成功返回空**，"+
				"正是要防的「静默返回空」形态 ⇒ 红（挂载点那个空目录必须收掉）", path))
		case isdir && isLink:
			rp, _ := filepath.EvalSymlinks(path)
			v.red("A3", fmt.Sprintf("未挂档但原路径（经符号链接）解析成一个**目录**（%s → %s）⇒ 读它会**成功返回空** ⇒ 红",
				path, rp))
		case exists && !isdir:
			v.red("A3", fmt.Sprintf("未挂档但原路径是一个**普通件**（%s）⇒ 读它不是「明确失败」⇒ 红", path))
		default:
			v.note("未挂档且原路径**解析不到**（ENOENT / 悬空符号链接）⇒ 读它**明确失败** ⇒ 这就是对的样子")
		}
		if hdiutilChecked && hdiutilText != "" && camHdiutilKnows(mountpoint, hdiutilText) {
			v.nc("挂载表说未挂、`hdiutil info` 认这个挂载点 ⇒ **两处读数打架** ⇒ rc=2 不给结论")
		}
		return
	}

	// ---- 已挂态 ----
	v.note(fmt.Sprintf("挂载行现读：%s", mountedLine))
	if !camMountRO(mountedLine) {
		v.red("A1", "挂载行**不含 `read-only`** ⇒ 挂的是**可写**档（chmod 那一条挡不住 owner 自己改回来）⇒ 红")
	}
	if hdiutilChecked {
		if hdiutilText == "" {
			v.nc("`hdiutil info` 取不到 ⇒ 两处读数只剩一处 ⇒ 不给结论")
		} else if !camHdiutilKnows(mountpoint, hdiutilText) {
			v.nc(fmt.Sprintf("挂载表说挂了、`hdiutil info` 不认 %s ⇒ **两处读数打架** ⇒ rc=2 不给结论", mountpoint))
		}
	}
	if !isLink {
		v.red("A2", "已挂档但原路径**不是符号链接**（实体目录/件）⇒ 数据面与挂载面会同时成立两份，"+
			"读数不再唯一 ⇒ 红（判据 4）")
	}
	// 件面
	if isdir {
		n := camCountFiles(path)
		v.note(fmt.Sprintf("经原路径数到的**文件**数 = %d（基线 ≥ %d）", n, minFiles))
		if n == 0 {
			v.red("A4", "已挂档但**件数 0** ⇒ 「挂了但读到空」⇒ 分辨得出「挂了但卷里没东西」⇒ 红")
		} else if n < minFiles {
			v.red("A4", fmt.Sprintf("已挂档但件数 %d < 基线 %d ⇒ 档不是本档（换了卷 / 拷了半份）⇒ 红", n, minFiles))
		}
		sp := filepath.Join(path, sample)
		body, err := os.ReadFile(sp)
		if err != nil {
			v.red("A5", fmt.Sprintf("已挂档但**样本件读不到**（%s：%s）⇒ 「挂了但读到错件」⇒ 红", sp, camStrerror(err)))
		} else if len(body) == 0 {
			v.red("A5", fmt.Sprintf("样本件 %s **读到了但是 0 字节** ⇒ 读到错件 ⇒ 红", sp))
		} else {
			v.note(fmt.Sprintf("样本件 %s 读得到（%d 字节）⇒ 不是空卷、也不是错件", sample, len(body)))
		}
	} else {
		v.red("A4", fmt.Sprintf("挂载点 %s 原地取不到（挂载行在、路径不在）⇒ 读数打架 ⇒ 红", mountpoint))
	}
}

// camStrerror —— py `e.strerror` 的对应面（Go 没有 strerror 直接取法 ⇒ 按 errno 映射常见几条；
// 落在映射外的走 `err.Error()`，本门对拍格只触发 ENOENT / EISDIR 两条）。
func camStrerror(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "No such file or directory"
	case errors.Is(err, fs.ErrPermission):
		return "Permission denied"
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		var eno syscall.Errno
		if errors.As(pe.Err, &eno) {
			switch eno {
			case syscall.ENOTDIR:
				return "Not a directory"
			case syscall.EISDIR:
				return "Is a directory"
			case syscall.ELOOP:
				return "Too many levels of symbolic links"
			case syscall.ENAMETOOLONG:
				return "File name too long"
			}
		}
	}
	return err.Error()
}

// ── 两处读数（读件 / 跑命令）──────────────────────────────────────────
// read_text(cmd=None, path=None)：给了 `path` 就读件（夹具档），否则跑 `cmd`（现读档）。
// 读不到 ⇒ ok=false（**不许当空串**）。
func camReadTextFile(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func camReadTextCmd(cmd []string) (string, bool) {
	if len(cmd) == 0 {
		return "", false
	}
	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// ── Q-176：本门**无机器读面** ⇒ 给了 `--json` 不许沉默 ✗ ──────────────────
// 口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
// 退码 = **用法错 rc=2**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
func camRefuseJSON(stderr io.Writer, argv []string) int {
	var hit []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return -1
	}
	parts := make([]string, 0, len(hit))
	for _, h := range hit {
		parts = append(parts, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(parts, " · "))
	// JSON_FIELDS 现读 = 空表 ⇒ 字段表恒为「无（本门只出人读面）」（唯一真源：py 的 JSON_FIELDS）。
	fmt.Fprint(stderr, "   字段表：无（本门只出人读面）\n")
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2
}

// ── argparse 子集 ───────────────────────────────────────────────────
type camArgs struct {
	Root        string
	MountTable  string
	Path        string
	Mountpoint  string
	MinFiles    int
	Sample      string
	HdiutilInfo string
	NoHdiutil   bool
	SelfTest    bool
	ListRules   bool
}

// camNegNum —— argparse `_negative_number_matcher`：`-1` / `-1.5` 这类**不算选项**（本 parser 无负数形选项）。
var camReNegNum = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

// camAtoi —— py `int(s)` 的实用子集（去空白 · 下划线分隔 · 正负号）。
func camAtoi(s string) (int, bool) {
	t := strings.ReplaceAll(strings.TrimSpace(s), "_", "")
	if t == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return 0, false
	}
	return int(n), true
}

// camArgErr —— argparse 错误档（usage + `prog: error: msg`）⇒ rc=2。
func camArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, camUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", camProg, msg)
	return 2
}

// camParse —— 返回 (args, rc)。rc>=0 ⇒ 已出结论（帮助/用法错）；rc<0 ⇒ 继续。
func camParse(stdout, stderr io.Writer, argv []string) (camArgs, int) {
	a := camArgs{Mountpoint: camDefaultMountpoint, MinFiles: camDefaultMinFiles, Sample: camDefaultSample}
	var positional, unknown []string
	afterDD := false
	i := 0
	for i < len(argv) {
		tok := argv[i]
		i++
		if afterDD {
			positional = append(positional, tok)
			continue
		}
		if tok == "--" {
			afterDD = true
			continue
		}
		if tok == "-h" || tok == "--help" {
			fmt.Fprint(stdout, camHelp)
			return a, 0
		}
		if strings.HasPrefix(tok, "--") {
			name, val, hasEq := tok, "", false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				name, val, hasEq = tok[:eq], tok[eq+1:], true
			}
			needVal := func() (string, bool) {
				if hasEq {
					return val, true
				}
				if i < len(argv) && (!strings.HasPrefix(argv[i], "-") || camReNegNum.MatchString(argv[i])) {
					v := argv[i]
					i++
					return v, true
				}
				return "", false
			}
			flagErr := func(nm string) (camArgs, int) {
				return a, camArgErr(stderr, fmt.Sprintf("argument %s: ignored explicit argument '%s'", nm, val))
			}
			switch name {
			case "--root":
				v, ok := needVal()
				if !ok {
					return a, camArgErr(stderr, "argument --root: expected one argument")
				}
				a.Root = v
			case "--mount-table":
				v, ok := needVal()
				if !ok {
					return a, camArgErr(stderr, "argument --mount-table: expected one argument")
				}
				a.MountTable = v
			case "--path":
				v, ok := needVal()
				if !ok {
					return a, camArgErr(stderr, "argument --path: expected one argument")
				}
				a.Path = v
			case "--mountpoint":
				v, ok := needVal()
				if !ok {
					return a, camArgErr(stderr, "argument --mountpoint: expected one argument")
				}
				a.Mountpoint = v
			case "--sample":
				v, ok := needVal()
				if !ok {
					return a, camArgErr(stderr, "argument --sample: expected one argument")
				}
				a.Sample = v
			case "--hdiutil-info":
				v, ok := needVal()
				if !ok {
					return a, camArgErr(stderr, "argument --hdiutil-info: expected one argument")
				}
				a.HdiutilInfo = v
			case "--min-files":
				v, ok := needVal()
				if !ok {
					return a, camArgErr(stderr, "argument --min-files: expected one argument")
				}
				n, okN := camAtoi(v)
				if !okN {
					return a, camArgErr(stderr, fmt.Sprintf("argument --min-files: invalid int value: '%s'", v))
				}
				a.MinFiles = n
			case "--no-hdiutil":
				if hasEq {
					return flagErr("--no-hdiutil")
				}
				a.NoHdiutil = true
			case "--self-test":
				if hasEq {
					return flagErr("--self-test")
				}
				a.SelfTest = true
			case "--list-rules":
				if hasEq {
					return flagErr("--list-rules")
				}
				a.ListRules = true
			default:
				unknown = append(unknown, tok)
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			unknown = append(unknown, tok)
			continue
		}
		positional = append(positional, tok)
	}
	if len(unknown) > 0 || len(positional) > 0 {
		bad := append(append([]string{}, unknown...), positional...)
		return a, camArgErr(stderr, "unrecognized arguments: "+strings.Join(bad, " "))
	}
	return a, -1
}

// ── 默认根 / 默认原路径（声明 (a)）────────────────────────────────────
// camDefaultRoot —— py 用 `__file__` 上溯三级；本件无此语境 ⇒ 从 cwd 上溯找含 `scripts/gates`
// 的目录，找不到回落 abspath(cwd)。
func camDefaultRoot() string {
	start, err := os.Getwd()
	if err != nil {
		return "."
	}
	d, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	cur := d
	for {
		if fi, e := os.Stat(filepath.Join(cur, "scripts", "gates")); e == nil && fi.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return d
}

// camDefaultPath —— py `os.path.join(os.path.dirname(os.path.abspath(root)), NAME)`。
//
// ★ 事实判断（见件头 (a)）：拼接段无 `..` ⇒ `filepath.Join` 与 `os.path.join` **逐字同** ⇒ 不串拼。
func camDefaultPath(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return filepath.Join(filepath.Dir(abs), camDefaultArchiveName)
}

// camPyRepr —— py `repr(str)` 的实用子集（含 ' 不含 " ⇒ 双引号；其余单引号）。
func camPyRepr(s string) string {
	esc := strings.ReplaceAll(s, "\\", "\\\\")
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		return "\"" + esc + "\""
	}
	return "'" + esc + "'"
}

// ── `--self-test`：13 格合成夹具（成对为主 · 一件真东西都不碰）──────────────
func camSelfTest(stdout io.Writer) int {
	bad, total := 0, 0

	type camCase struct {
		name  string
		want  int
		mode  string // "" = None · "none" · "ro" · "rw"
		build func(d, mp, path string)
		hdi   string // "knows" · "unknown" · ""(=不查)
	}

	kase := func(c camCase) {
		total++
		d, err := os.MkdirTemp("", "cam")
		if err != nil {
			bad++
			fmt.Fprintf(stdout, "  ✗ %s ⇒ rc=?（期望 %d） ｜ 夹具目录造不出\n", c.name, c.want)
			return
		}
		defer func() { _ = os.RemoveAll(d) }()
		mp := filepath.Join(d, "mnt")
		_ = os.MkdirAll(mp, 0o755)
		path := filepath.Join(d, camDefaultArchiveName)
		if c.build != nil {
			c.build(d, mp, path)
		}
		var mountText string
		mountNone := false
		switch c.mode {
		case "":
			mountNone = true
		case "none":
			mountText = "dev on / (apfs, local, read-only, sealed)\n"
		case "ro":
			mountText = fmt.Sprintf("dev on %s (apfs, local, nodev, read-only, journaled)\n", mp)
		default: // rw
			mountText = fmt.Sprintf("dev on %s (apfs, local, nodev, journaled)\n", mp)
		}
		hdiText, checked := "", false
		switch c.hdi {
		case "knows":
			hdiText, checked = fmt.Sprintf("hdiutil: %s\n", mp), true
		case "unknown":
			hdiText, checked = "hdiutil:（没有这一条）\n", true
		default:
			hdiText, checked = "", false
		}
		v := &camVerdict{}
		camJudge(v, mountText, mountNone, path, mp, 2, camDefaultSample, hdiText, checked)
		rc := 0
		switch {
		case len(v.noconcl) > 0:
			rc = 2
		case len(v.reds) > 0:
			rc = 1
		}
		ok := rc == c.want
		if !ok {
			bad++
		}
		why := ""
		if !ok {
			if len(v.reds) > 0 {
				why = " ｜ " + v.reds[0][1]
			} else {
				why = " ｜ " + strings.Join(v.noconcl, " · ")
			}
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s ⇒ rc=%d（期望 %d）%s\n", mark, c.name, rc, c.want, why)
	}

	buildNone := func(_d, _mp, _p string) {}
	buildEmptyDir := func(_d, _mp, path string) { _ = os.MkdirAll(path, 0o755) }
	buildRealDir := func(_d, _mp, path string) {
		_ = os.MkdirAll(path, 0o755)
		for i := 0; i < 3; i++ {
			_ = os.WriteFile(filepath.Join(path, fmt.Sprintf("f%d.md", i)), []byte("x"), 0o644)
		}
		_ = os.WriteFile(filepath.Join(path, camDefaultSample), []byte("索引"), 0o644)
	}
	buildDangling := func(d, _mp, path string) { _ = os.Symlink(filepath.Join(d, "mnt-gone"), path) }
	buildMountedOK := func(_d, mp, path string) {
		_ = os.Symlink(mp, path)
		for i := 0; i < 3; i++ {
			_ = os.WriteFile(filepath.Join(mp, fmt.Sprintf("f%d.md", i)), []byte("x"), 0o644)
		}
		_ = os.WriteFile(filepath.Join(mp, camDefaultSample), []byte("索引"), 0o644)
	}
	buildMountedEmpty := func(_d, mp, path string) { _ = os.Symlink(mp, path) }
	buildMountedNoSample := func(_d, mp, path string) {
		_ = os.Symlink(mp, path)
		for i := 0; i < 3; i++ {
			_ = os.WriteFile(filepath.Join(mp, fmt.Sprintf("f%d.md", i)), []byte("x"), 0o644)
		}
	}

	for _, c := range []camCase{
		{"① 未挂 + 原路径不存在 ⇒ 读**明确失败**（对的样子）", 0, "none", buildNone, "unknown"},
		{"①′ 未挂 + 悬空符号链接 ⇒ 读**明确失败**（对的样子）", 0, "none", buildDangling, "unknown"},
		{"② 未挂 + 原路径是**空目录** ⇒ 静默返回空（要防的核心形态）", 1, "none", buildEmptyDir, "unknown"},
		{"③ 未挂 + 原路径是**非空实体目录** ⇒ 数据面/挂载面同时两份", 1, "none", buildRealDir, "unknown"},
		{"④ 已挂 read-only + 件数够 + 样本读得到 ⇒ 绿", 0, "ro", buildMountedOK, "knows"},
		{"⑤ 已挂**可写**（挂载行无 read-only）⇒ 红", 1, "rw", buildMountedOK, "knows"},
		{"⑥ 已挂 read-only 但**件数 0** ⇒ 「挂了但读到空」", 1, "ro", buildMountedEmpty, "knows"},
		{"⑦ 已挂 read-only 但原路径是**实体目录** ⇒ 「不是本档」", 1, "ro", buildRealDir, "knows"},
		{"⑦′ 已挂 read-only 但**样本件读不到** ⇒ 「读到错件」", 1, "ro", buildMountedNoSample, "knows"},
		{"⑧ 两处读数打架（挂载表有行 · `hdiutil` 不认）⇒ 不给结论", 2, "ro", buildMountedOK, "unknown"},
		{"⑨ 挂载表 0 行 ⇒ 空转不给结论", 2, "", nil, "knows"},
	} {
		kase(c)
	}

	// ⑩ 新档的牙：旋钮**显式**指的归档树根在盘上取不到 ⇒ 缺席档 rc=2；未挂**断链**不算缺席（成对）。
	d10, err := os.MkdirTemp("", "cam")
	if err == nil {
		defer func() { _ = os.RemoveAll(d10) }()
		absent := filepath.Join(d10, "no-such-arch")
		dangling := filepath.Join(d10, "dangling-arch")
		_ = os.Symlink(filepath.Join(d10, "gone"), dangling)
		for _, pr := range []struct {
			label string
			want  bool
			probe string
		}{
			{"⑩ 旋钮指到不在盘的归档树根 ⇒ 缺席档（rc=2 档）", true, absent},
			{"⑩′ 未挂断链（符号链接在）⇒ **不**算缺席（默认档常态）", false, dangling},
		} {
			total++
			got := camArchiveRootAbsent(pr.probe)
			ok := got == pr.want
			if !ok {
				bad++
			}
			mark := "✓"
			if !ok {
				mark = "✗"
			}
			fmt.Fprintf(stdout, "  %s %s（archive_root_absent=%s · 期望 %s）\n",
				mark, pr.label, camPyBool(got), camPyBool(pr.want))
		}
	}
	fmt.Fprintf(stdout, "  ── 自检：%d 格 · %d 格不过\n", total, bad)
	if bad == 0 {
		return 0
	}
	return 2
}

// camPyBool —— py `str(bool)` ⇒ "True"/"False"（✗ 不是 Go 的 "true"/"false"）。
func camPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// ── 入口 ─────────────────────────────────────────────────────────────
func checkArchiveMountGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：`--json` 在本门**未实现** ⇒ rc=2（先于 argparse）。
	if rc := camRefuseJSON(stderr, argv); rc >= 0 {
		return rc
	}
	a, rc := camParse(stdout, stderr, argv)
	if rc >= 0 {
		return rc
	}

	if a.ListRules {
		fmt.Fprint(stdout, "门㉛ 归档挂载档 —— 判据（A 档）；缺陷一格 = rc=1；读数打架/空转/旋钮指空的树根 = rc=2\n")
		for _, r := range camRules {
			fmt.Fprintf(stdout, "  %s\n", r)
		}
		fmt.Fprint(stdout, "  红线：本门只读 —— 不挂/不摘/不写/不 chmod；挂摘只走 `zerg archive mount on|off|status`\n")
		return 0
	}
	if a.SelfTest {
		fmt.Fprint(stdout, "── 自检（合成夹具 · 13 格成对负控）──\n")
		rc := camSelfTest(stdout)
		if rc == 0 {
			fmt.Fprint(stdout, "自检结论: 全过\n")
		} else {
			fmt.Fprint(stdout, "自检结论: 不过（拒绝扫真目标）\n")
		}
		return rc
	}

	// 仓根 = 本件上溯三级（`scripts/gates/<件>` → 仓根）；归档树根（原路径）= **仓根同级**的 `Zerg-归档`。
	// 取根体例：`--path` 旗标 > env `ZERG_ARCHIVE_ROOT` > 默认 `<仓根同级>/Zerg-归档`（默认值逐字不变）。
	root := a.Root
	if root == "" {
		root = camDefaultRoot()
	}
	// ★ 空集守卫（py `require_domain([root] if os.path.isdir(root) else [], …)` 逐字同构 · 同款空域残留）：
	//   仓根取不到（`--root` 不存在 / 不是目录）⇒ 归档树根由它派生、判不了 ⇒ rc=2 **不给结论**（✗ 不当 0）。
	//   负控：默认档（无 `--root`）派生出的仓根恒在盘 ⇒ 不触发；正常根 + 未挂档仍按 A3 判绿（不动 A 档判据）。
	if !camIsDir(root) {
		fmt.Fprintf(stderr, "BLOCKED —— 空域 / 取不到输入：仓根 %s（归档树根由它派生）（现读 0 < 写死下限 1）⇒ rc=2 不给结论\n", root)
		return 2
	}
	explicitArch := a.Path
	if explicitArch == "" {
		explicitArch = os.Getenv("ZERG_ARCHIVE_ROOT")
	}
	path := explicitArch
	if path == "" {
		path = camDefaultPath(root)
	}
	// 缺席分支：**旋钮显式指的**归档树根若在盘上完全取不到 ⇒ rc=2 不给结论（✗ 不当 0）。
	// ⚠ 默认档（无旋钮）**不判这一档**：本机 `../Zerg-归档` 是**未挂断链**（符号链接在、目标不在），
	//   那是本门 A3 核心格要判的常态 ⇒ 硬一律判 rc=2 会翻改现役默认档 ⇒ 只收口旋钮档。
	if explicitArch != "" && camArchiveRootAbsent(path) {
		fmt.Fprintf(stdout, "门㉛ 归档挂载档（`A8`）· 仓根 %s\n", root)
		fmt.Fprintf(stdout, "  ⛔ 归档树根（旋钮**显式**指到）在盘上取不到：%s\n", path)
		fmt.Fprint(stdout, "     （既不是符号链接、也不是实体档/目录 ⇒ 挂载档判不了 ⇒ **不给结论**）\n")
		fmt.Fprintf(stdout, "     旋钮现读：`--path`=%s · env `ZERG_ARCHIVE_ROOT`=%s\n",
			camPyRepr(a.Path), camPyRepr(os.Getenv("ZERG_ARCHIVE_ROOT")))
		fmt.Fprint(stdout, "     ⚠ 默认档（无旋钮）不判这一档 —— 未挂断链是常态（见件内注释）\n")
		fmt.Fprint(stdout, "⇒ **不给结论**（rc=2）\n")
		return 2
	}

	var mountText string
	mountNone := false
	if a.MountTable != "" {
		if s, ok := camReadTextFile(a.MountTable); ok {
			mountText = s
		} else {
			mountNone = true
		}
	} else {
		if s, ok := camReadTextCmd([]string{"mount"}); ok {
			mountText = s
		} else {
			mountNone = true
		}
	}
	if mountNone && a.MountTable == "" {
		fmt.Fprint(stdout, "门㉛ 归档挂载档：挂载表读不到（`mount` 跑了没成）⇒ **不给结论**（退码 2）\n")
		return 2
	}
	hdiutilText := ""
	checked := false
	if !a.NoHdiutil {
		checked = true
		if a.HdiutilInfo != "" {
			if s, ok := camReadTextFile(a.HdiutilInfo); ok {
				hdiutilText = s
			}
		} else if s, ok := camReadTextCmd([]string{"hdiutil", "info"}); ok {
			hdiutilText = s
		}
	}

	fmt.Fprintf(stdout, "门㉛ 归档挂载档（`A8`）· 仓根 %s\n", root)
	fmt.Fprintf(stdout, "  原路径     : %s\n", path)
	fmt.Fprintf(stdout, "  挂载点     : %s\n", a.Mountpoint)
	fmt.Fprintf(stdout, "  件数基线   : ≥ %d（现读基线 38）\n", a.MinFiles)
	v := &camVerdict{}
	camJudge(v, mountText, mountNone, path, a.Mountpoint, a.MinFiles, a.Sample, hdiutilText, checked)
	for _, n := range v.notes {
		fmt.Fprintf(stdout, "  · %s\n", n)
	}
	for _, r := range v.reds {
		fmt.Fprintf(stdout, "  ✗ %s %s\n", r[0], r[1])
	}
	for _, m := range v.noconcl {
		fmt.Fprintf(stdout, "  ⛔ %s\n", m)
	}
	if len(v.noconcl) > 0 {
		fmt.Fprintf(stdout, "⇒ **不给结论**（rc=2）：%d 条\n", len(v.noconcl))
		return 2
	}
	if len(v.reds) > 0 {
		fmt.Fprintf(stdout, "⇒ 红（rc=1）：%d 条\n", len(v.reds))
		return 1
	}
	fmt.Fprint(stdout, "⇒ 全绿（rc=0）：挂载档 read-only · 原路径是符号链接 · 未挂则明确失败 · 件数与样本件都对得上\n")
	return 0
}
