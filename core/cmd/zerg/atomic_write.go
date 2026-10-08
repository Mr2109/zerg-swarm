// atomic_write.go —— 件 scripts/dev/atomic-write.sh（263 行）的 Go 真搬
// （非 gates 面 · M2-0 工具档 · 入口 atomicWriteGateMain · 不接线 / 不注册 / 不删原件）。
//
// 为什么有这件（逐处现读 scripts/dev/atomic-write.sh 现值 · 非按件名推断）：
//
//	· 形态 = 换件正门工具：算旧件 sha256 -> 留档到 scripts/gates/_history/ ->
//	  回读校验 -> 写前预校（防中途被改）-> 临时件落目标 -> 写后回读对拍。
//	· 外部工具面 = 只调 OS 基本功（shasum/stat/cp/mv/cmp/date/mktemp/mkdir），
//	  全部由 Go 标准库在进程内等价承载 ⇒ 无「必须调外部工具」的阻塞面（可 Go）。
//	· 退码 = 恰 0/1/2（0 换件成功 · 1 拒写：越界/留档失败/回读不符/写后对拍不符 ·
//	  2 不给结论：用法错/缺件/自检不过/预校发现中途被改）。
//	· 非确定面 = 留档名时间戳（.bak-YYYYMMDD-HHMMSS）· 同名冲突后缀 $$（= 进程 pid）·
//	  预校快照 mktemp 随机名 · 自检夹具临时目录 ⇒ 对拍按掩码（见回执）。
//
// 入口签名（与原件 `$@` 对齐）：
//
//	func atomicWriteGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与原件位置参数同形（不含脚本名）
//	  · 退码  —— 0/1/2（见上）
//	  · 输出  —— stdout/stderr 与原件逐字节对齐（除已声明的非确定面掩码）
//
// 出厂声明 (a) 默认根定位差异：原件以 `dirname($BASH_SOURCE)/../..` 定仓根（脚本位置上溯 · 物理路径）；
// 本 Go 件无 __file__ 语境 ⇒ 走同包 repoRoot()（ZERG_REPO → 可执行件目录上溯 → cwd 上溯，判据 =
// 仓根有 core/internal/version/version.go）⇒「脚本位置 ≠ cwd/可执行件」时默认根不同（✗ 不声称逐字对齐）。
// 自检走进程内调用并显式传合成夹具根 ⇒ 该差异在自检档**不适用**。
//
// 出厂声明 (b) 自指检查：本件落在 core/cmd/zerg/*.go 全件扫描面内 ⇒ 逐条消解：
//
//	· 零私有绝对路径字面量（只写 `scripts/gates/_history` 这类仓根相对串）；
//	· 零连续「引号 + 键名 + 引号」原串 / 零错误登记函数的括弧引号形态；
//	· 零命令树层级字面量（本件不进命令树）。
//
// 本单边界（照实）：只落码、不接线 ——
//
//	· 不在 core/cmd/zerg/main.go 注册命令（✗ 不进命令树 · ✗ 不改 gate_inproc_table.go）；
//	· 不删 scripts/dev/atomic-write.sh 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 atomicWriteGateMain 目前无调用点（M2 的刻意状态）。
//
// ★ 自检缝合点（照拄原件）：环境变量 ATOMIC_WRITE_DROP_REL（留档落点覆盖）与
// ATOMIC_WRITE_NEG（负控：mutate_target / corrupt_archive）—— 本件在**顶层入口**读这两枚，
// 自检走显式字段（不改进程 env）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// awHistWhitelist = 原件 HIST_WHITELIST（照抄 Go 真身 scripts/build/cmd/zerg-build-all/main.go 的 historyDropOK 口径）。
const awHistWhitelist = "scripts/gates/_history"

// awUsage 复刻原件 usage()（`cat <<'USAGE' … USAGE` 逐字正文 + 尾换行 · 印到 stdout）。
const awUsage = `用法
  bash scripts/dev/atomic-write.sh --target <仓内相对路径> --from <仓外新件绝对路径>
  bash scripts/dev/atomic-write.sh --help
  bash scripts/dev/atomic-write.sh --self-test
说明
  --target  要换的件（仓内相对路径，例 scripts/gates/precommit-gates.sh）
  --from    新内容（仓外新件的绝对路径）
  步骤：算旧件 sha256 -> 落 scripts/gates/_history/<件名>.bak-<时间戳> -> 回读校验
        -> 失败即拒写（目标件一个字节不动）-> 写前预校 -> 临时件落目标 -> 写后回读对拍
退码
  0 换件成功 · 1 拒写（越界/留档失败/回读不符/写后对拍不符）· 2 不给结论（用法错/缺件/自检不过/预校发现中途被改）
`

// awCtx 承载一次调用的仓根 / 留档落点 / 负控与 IO（自检时替换为合成夹具根）。
type awCtx struct {
	root    string
	histRel string
	neg     string
	stdout  io.Writer
	stderr  io.Writer
}

func (c *awCtx) warn(msg string) { fmt.Fprintf(c.stderr, "%s\n", msg) }

// die 照原件 die()：stderr 逐字一行，退 rc。
func (c *awCtx) die(rc int, msg string) int {
	c.warn(msg)
	return rc
}

func awOrDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// awBase 照 basename（末段；本件输入恒为无尾斜杠的绝对件路径）。
func awBase(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// awDir 照 posixpath.dirname（剥尾斜杠后取父段）。
func awDir(p string) string {
	if p == "" {
		return "."
	}
	i := len(p)
	for i > 1 && p[i-1] == '/' {
		i--
	}
	p = p[:i]
	j := strings.LastIndexByte(p, '/')
	if j < 0 {
		return "."
	}
	if j == 0 {
		return "/"
	}
	return p[:j]
}

// awTrimNL 照 `$(cat <件>)`：剥全部尾换行（命令替换语义）。
func awTrimNL(s string) string { return strings.TrimRight(s, "\n") }

// awSha256 照原件 sha_of()（shasum -a 256 / sha256sum 的等价：文件字节的 sha256 小写十六进制）。
// 读不到 ⇒ ok=false（原件此时回空串并继续，调用方按逐字现状处理）。
func awSha256(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), true
}

// awBytes 照原件 bytes_of()（stat -f%z｜stat -c%s）。
func awBytes(path string) (int64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	return fi.Size(), true
}

// awCopyP 照 `cp -p <src> <dst>`（内容逐字节 + 保源件权限）。
func awCopyP(src, dst string) bool {
	sfi, err := os.Stat(src)
	if err != nil || !sfi.Mode().IsRegular() {
		return false
	}
	in, err := os.Open(src)
	if err != nil {
		return false
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sfi.Mode().Perm())
	if err != nil {
		return false
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return false
	}
	return out.Close() == nil
}

// awCmpEqual 照 `cmp -s <a> <b>`（逐字节相等 ⇒ true）。
func awCmpEqual(a, b string) bool {
	ab, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}

// awRandSuffix 产 n 位 [A-Za-z0-9]（字符集照 mktemp 的随机后缀）。属**非确定面** ⇒ 对拍按掩码。
func awRandSuffix(n int) string {
	const cs = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	seed := time.Now().UnixNano()
	for i := range b {
		seed = seed*6364136223846793005 + 1442695040888963407
		b[i] = cs[uint64(seed>>33)%uint64(len(cs))]
	}
	return string(b)
}

// awMkdtempRaw 照 `mktemp -d "${TMPDIR:-/tmp}/<pattern>XXXXXX"` 的**字符串拼接**语义：
// 落点 = `$TMPDIR`（缺省 /tmp）原样拼 `/` ＋ pattern ＋ 6 位后缀（✗ 不走 filepath.Join/Clean）。
func awMkdtempRaw(pattern string) (string, error) {
	base := os.TempDir() // = $TMPDIR 原样（不 Clean）；缺省 /tmp。
	for i := 0; i < 1000; i++ {
		name := base + "/" + pattern + awRandSuffix(6)
		err := os.Mkdir(name, 0o700)
		if err == nil {
			return name, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", os.ErrExist
}

// awMktempFileRaw 照 `mktemp "${TMPDIR:-/tmp}/<pattern>XXXXXX"`（建件 · mode 0600）。
func awMktempFileRaw(pattern string) (string, error) {
	base := os.TempDir()
	for i := 0; i < 1000; i++ {
		name := base + "/" + pattern + awRandSuffix(6)
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_ = f.Close()
			return name, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", os.ErrExist
}

// awDropOk 照原件 drop_ok()（落点白名单 · $1 = 仓内相对落点）。
func (c *awCtx) awDropOk(rel string) bool {
	if strings.HasPrefix(rel, awHistWhitelist+"/") {
		return true
	}
	c.warn(fmt.Sprintf("   ✗ 留档落点 %s 不在 %s/ 之下 ⇒ 判红、拒写（目标件一个字节不动）", rel, awHistWhitelist))
	return false
}

// awArchivePrev 照拄原件 archive_prev()（Go 真身 scripts/build/cmd/zerg-build-all/main.go 的 archivePrevPiece）：$1 = 将被就地覆盖的件（绝对路径）。
func (c *awCtx) awArchivePrev(abs string) bool {
	fi, err := os.Stat(abs)
	if err != nil || !fi.Mode().IsRegular() { // `[ -f "$b" ] || return 0`
		return true
	}
	base := awBase(abs)
	hist := c.root + "/" + c.histRel
	if err := os.MkdirAll(hist, 0o755); err != nil {
		c.warn(fmt.Sprintf("   ✗ 留档目录建不起来：%s ⇒ 拒写", hist))
		return false
	}
	src, _ := awSha256(abs)
	// `for hit in "${hist}/${base}".bak-*` —— bash glob 按字典序；只读留档目录内命中项。
	ents, _ := os.ReadDir(hist)
	var names []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasPrefix(e.Name(), base+".bak-") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, nm := range names {
		hit := hist + "/" + nm
		if hfi, herr := os.Stat(hit); herr == nil && hfi.Mode().IsRegular() {
			if h, _ := awSha256(hit); h == src {
				fmt.Fprintf(c.stdout,
					"   🗄  留档跳过：%s 与已有留档 %s 逐字节相同（sha256 %s）—— 不重复留、不覆盖\n",
					base, nm, src)
				return true
			}
		}
	}
	stamp := time.Now().Format("20060102-150405")
	newPath := hist + "/" + base + ".bak-" + stamp
	if _, err := os.Lstat(newPath); err == nil { // `[ -e "${new}" ]`
		newPath = newPath + "." + strconv.Itoa(os.Getpid())
	}
	rel := strings.TrimPrefix(newPath, c.root+"/") // `"${new#"${REPO_ROOT}/"}"`
	if !c.awDropOk(rel) {
		return false
	}
	if !awCopyP(abs, newPath) {
		c.warn(fmt.Sprintf("   ✗ 留档失败（cp -p %s → %s）⇒ 拒写", abs, newPath))
		return false
	}
	if c.neg == "corrupt_archive" { // 自检缝合点：模拟「cp 出来的留档件是坏的」
		_ = os.WriteFile(newPath, nil, 0o644)
	}
	h, _ := awSha256(newPath)
	if h != src {
		c.warn(fmt.Sprintf("   ✗ 留档回读 sha 与旧件不同（%s ≠ %s）：%s ⇒ 拒写", h, src, awBase(newPath)))
		return false
	}
	nb, _ := awBytes(newPath)
	fmt.Fprintf(c.stdout, "   🗄  已留档旧件：%s/%s（%d 字节 · sha256 %s）\n",
		c.histRel, awBase(newPath), nb, src)
	return true
}

// awRunCtx = 参数解析 + 主流程（照原件 while/case）。
func awRunCtx(c *awCtx, argv []string) int {
	var (
		selftest  bool
		targetRel string
		fromAbs   string
	)
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			io.WriteString(c.stdout, awUsage)
			return 0
		case a == "--self-test":
			selftest = true
			i++
		case a == "--target":
			if i+1 >= len(argv) {
				return c.die(2, "用法错：--target 缺实参（见 --help）")
			}
			targetRel = argv[i+1]
			i += 2
		case a == "--from":
			if i+1 >= len(argv) {
				return c.die(2, "用法错：--from 缺实参（见 --help）")
			}
			fromAbs = argv[i+1]
			i += 2
		case strings.HasPrefix(a, "-"):
			return c.die(2, fmt.Sprintf("用法错：未知旗标 %s（见 --help）", a))
		default:
			return c.die(2, fmt.Sprintf("用法错：多余的位置参数 %s（见 --help）", a))
		}
	}
	if selftest {
		return awSelftest(c)
	}
	if targetRel == "" {
		return c.die(2, "用法错：缺 --target（见 --help）")
	}
	if fromAbs == "" {
		return c.die(2, "用法错：缺 --from（见 --help）")
	}
	if !strings.HasPrefix(fromAbs, "/") {
		return c.die(2, "用法错：--from 要绝对路径（见 --help）")
	}
	if ffi, ferr := os.Stat(fromAbs); ferr != nil || !ffi.Mode().IsRegular() {
		return c.die(2, fmt.Sprintf("用法错：--from 件不存在：%s", fromAbs))
	}
	targetAbs := c.root + "/" + targetRel
	if dfi, derr := os.Stat(awDir(targetAbs)); derr != nil || !dfi.IsDir() {
		return c.die(2, "用法错：--target 目录不存在（见 --help）")
	}

	oldSha := ""
	if tfi, terr := os.Stat(targetAbs); terr == nil && tfi.Mode().IsRegular() {
		oldSha, _ = awSha256(targetAbs)
		if !c.awArchivePrev(targetAbs) {
			return c.die(1, fmt.Sprintf("留档失败/落点越界 ⇒ 拒写（%s 一个字节未动）", targetRel))
		}
		if c.neg == "mutate_target" { // 自检缝合点：模拟「别人中途改了目标件」
			if f, ferr := os.OpenFile(targetAbs, os.O_APPEND|os.O_WRONLY, 0o644); ferr == nil {
				_, _ = f.WriteString("\n# intruder\n")
				_ = f.Close()
			}
		}
		// ★ 写前预校：真正写入前再读一次目标件，与第①步记忆的旧件 sha 比。
		snap, serr := awMktempFileRaw("aw-precheck.")
		if serr != nil {
			return c.die(2, "拒写：预校快照建不起来")
		}
		awCopyP(targetAbs, snap)
		curSha, _ := awSha256(targetAbs)
		if curSha != oldSha {
			c.warn("   ✗ 写前预校失败：目标件现值 sha 与第①步读到的旧件 sha 不同")
			c.warn(fmt.Sprintf("     记忆旧件 sha256 %s", oldSha))
			c.warn(fmt.Sprintf("     目标件现 sha256 %s", curSha))
			c.warn(fmt.Sprintf("     ⇒ 有人在本件开工后中途改过 %s ⇒ 拒写、不给结论（目标件一个字节不动）", targetRel))
			if awCmpEqual(targetAbs, snap) {
				c.warn("   cmp：目标件与预校时刻读数逐字节相同（cmp -s ⇒ rc=0）⇒ 本件未写一个字节")
			} else {
				c.warn("   cmp：目标件与预校时刻读数不同（cmp -s ⇒ rc=1）—— 异常，请人工核")
			}
			_ = os.Remove(snap)
			return 2
		}
		_ = os.Remove(snap)
	}

	newSha, ok := awSha256(fromAbs)
	if !ok {
		return c.die(1, fmt.Sprintf("算新件 sha 失败 ⇒ 拒写（%s 未动）", targetRel))
	}
	tmp := targetAbs + ".atomicwrite." + strconv.Itoa(os.Getpid())
	if !awCopyP(fromAbs, tmp) {
		return c.die(1, fmt.Sprintf("写临时件失败 ⇒ 拒写（%s 未动）", targetRel))
	}
	if h, _ := awSha256(tmp); h != newSha {
		_ = os.Remove(tmp)
		return c.die(1, fmt.Sprintf("临时件回读 sha 不符 ⇒ 拒写（%s 未动）", targetRel))
	}
	if err := os.Rename(tmp, targetAbs); err != nil {
		_ = os.Remove(tmp)
		return c.die(1, "落目标失败 ⇒ 拒写")
	}
	finalSha, _ := awSha256(targetAbs)
	if finalSha != newSha {
		return c.die(1, fmt.Sprintf("写后回读 sha 不符（%s ≠ %s）⇒ 异常，请人工核", finalSha, newSha))
	}
	nb, _ := awBytes(targetAbs)
	fmt.Fprintf(c.stdout, "✅ 换件成功：%s（%d 字节 · sha256 %s）\n", targetRel, nb, finalSha)
	if oldSha != "" {
		fmt.Fprintf(c.stdout, "   旧件 sha256 %s → 新件 sha256 %s（旧件已在 %s/ 留档）\n", oldSha, finalSha, c.histRel)
	} else {
		fmt.Fprintf(c.stdout, "   首建（无旧件可留档）\n")
	}
	return 0
}

// awSelftest 复刻原件 selftest() 的 15 格（改走进程内调用 · 夹具根 = 合成仓 fake）。
func awSelftest(c *awCtx) int {
	tmp, err := awMkdtempRaw("aw-selftest.")
	if err != nil {
		c.warn("自检临时目录建不起来")
		return 2
	}
	defer os.RemoveAll(tmp)
	fake := tmp + "/repo"
	outside := tmp + "/outside"
	for _, d := range []string{fake + "/scripts/gates", fake + "/scripts/dev", fake + "/bin", outside} {
		_ = os.MkdirAll(d, 0o755)
	}
	G := fake + "/scripts/gates/demo.sh"
	_ = os.WriteFile(G, []byte("OLD v1\n"), 0o644)
	oldSha, _ := awSha256(G)
	_ = os.WriteFile(outside+"/new2.sh", []byte("NEW v2\n"), 0o644)

	fmt.Fprintf(c.stdout, "  —— atomic-write.sh --self-test（合成夹具 · 全在 %s 下）——\n", tmp)

	pass, fail := 0, 0
	chk := func(desc string, cond bool, reading string) {
		if cond {
			fmt.Fprintf(c.stdout, "    ✅ %s\n", desc)
			pass++
			return
		}
		fmt.Fprintf(c.stdout, "    ❌ %s  [%s]\n", desc, reading)
		fail++
	}
	run := func(histRel, neg string, argv ...string) (int, string) {
		var ob, eb strings.Builder
		cc := &awCtx{root: fake, histRel: histRel, neg: neg, stdout: &ob, stderr: &eb}
		rc := awRunCtx(cc, argv)
		return rc, ob.String() + eb.String()
	}
	hist := fake + "/" + awHistWhitelist
	bakNames := func() []string {
		ents, _ := os.ReadDir(hist)
		var ns []string
		for _, e := range ents {
			if !e.IsDir() && strings.HasPrefix(e.Name(), "demo.sh.bak-") {
				ns = append(ns, e.Name())
			}
		}
		sort.Strings(ns)
		return ns
	}
	readTrim := func(p string) string {
		b, _ := os.ReadFile(p)
		return awTrimNL(string(b))
	}

	// ① 正常换件
	rc, _ := run(awHistWhitelist, "", "--target", "scripts/gates/demo.sh", "--from", outside+"/new2.sh")
	chk("① 正常换件 ⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("① 目标件 == 新件内容", readTrim(G) == "NEW v2", "")
	ns := bakNames()
	chk("① _history 命中 1 枚", len(ns) == 1, fmt.Sprintf("命中 %d", len(ns)))
	arch := ""
	if len(ns) > 0 {
		arch = hist + "/" + ns[0]
	}
	ah, _ := awSha256(arch)
	chk("② 留档件 sha == 旧件现算 sha（两串相同）", arch != "" && ah == oldSha,
		fmt.Sprintf("留档=%s 旧件=%s", ah, oldSha))

	// ①b 同名同 sha 已存在 ⇒ 跳过不重写
	_ = os.WriteFile(G, []byte("OLD v1\n"), 0o644)
	_ = os.WriteFile(outside+"/new3.sh", []byte("NEW v3\n"), 0o644)
	rc, _ = run(awHistWhitelist, "", "--target", "scripts/gates/demo.sh", "--from", outside+"/new3.sh")
	ns = bakNames()
	chk("①b 同名同 sha 已存在 ⇒ 跳过不重写（仍 1 枚）", rc == 0 && len(ns) == 1,
		fmt.Sprintf("rc=%d 命中 %d", rc, len(ns)))

	// ③ 中途被改（能分真假）：预校发现目标件被改 ⇒ rc=2 且目标件未被写入
	_ = os.WriteFile(G, []byte("OLD v1\n"), 0o644)
	_ = os.WriteFile(outside+"/x.sh", []byte("NEW vX\n"), 0o644)
	rc, o3 := run(awHistWhitelist, "mutate_target", "--target", "scripts/gates/demo.sh", "--from", outside+"/x.sh")
	chk("③ 中途被改 ⇒ rc=2（非零）", rc == 2, fmt.Sprintf("rc=%d", rc))
	cur3 := readTrim(G)
	chk("③ 目标件未被写入新内容（未动）", cur3 != "NEW vX" && cur3 == "OLD v1\n\n# intruder",
		fmt.Sprintf("现值=%s", cur3))
	chk("③ 回执里有 cmp 证明行", strings.Contains(o3, "cmp"), "")

	// ④ 落点越界 ⇒ 报错码正确（照拄白名单口径）
	_ = os.WriteFile(G, []byte("OLD v1\n"), 0o644)
	rc, o4 := run("bin", "", "--target", "scripts/gates/demo.sh", "--from", outside+"/new2.sh")
	chk("④ 落点越界 ⇒ rc=1", rc == 1, fmt.Sprintf("rc=%d", rc))
	chk("④ 落点越界 ⇒ 逐字点名（不在 …/_history/ 之下）", strings.Contains(o4, "不在"), "")

	// ⑤ 未知旗标 ⇒ 用法错码 + 逐字
	rc, o5 := run(awHistWhitelist, "", "--bogus")
	chk("⑤ 未知旗标 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	chk("⑤ 未知旗标 ⇒ 逐字点名", strings.Contains(o5, "未知旗标 --bogus"), "")

	// ⑥ 回读校验负控（能分真假）：留档件被写坏 ⇒ 回读不符 ⇒ rc=1、拒写
	_ = os.WriteFile(G, []byte("OLD v9 unique\n"), 0o644)
	rc, o6 := run(awHistWhitelist, "corrupt_archive", "--target", "scripts/gates/demo.sh", "--from", outside+"/new2.sh")
	chk("⑥ 留档回读不符 ⇒ rc=1（拒写）", rc == 1, fmt.Sprintf("rc=%d", rc))
	cur6 := readTrim(G)
	chk("⑥ 回读不符 ⇒ 目标件未被覆盖", cur6 == "OLD v9 unique", fmt.Sprintf("现值=%s", cur6))
	chk("⑥ 回读不符 ⇒ 逐字点名", strings.Contains(o6, "留档回读 sha 与旧件不同"), "")

	fmt.Fprintf(c.stdout, "  —— 自检：%d 格过 · %d 格红（夹具 %s）——\n", pass, fail, tmp)
	if fail == 0 {
		return 0
	}
	c.warn("自检有红 ⇒ 不给结论")
	return 2
}

// awParseAndRun 由顶层入口与自检共用：自检显式传字段，顶层入口读 env 默认值。
func awParseAndRun(root string, stdout, stderr io.Writer, argv []string) int {
	c := &awCtx{
		root:    root,
		histRel: awOrDefault(os.Getenv("ATOMIC_WRITE_DROP_REL"), awHistWhitelist),
		neg:     os.Getenv("ATOMIC_WRITE_NEG"),
		stdout:  stdout,
		stderr:  stderr,
	}
	return awRunCtx(c, argv)
}

// atomicWriteGateMain = 件入口（argv 与原件位置参数同形 · 不含脚本名）。
func atomicWriteGateMain(stdout, stderr io.Writer, argv []string) int {
	return awParseAndRun(repoRoot(), stdout, stderr, argv)
}
