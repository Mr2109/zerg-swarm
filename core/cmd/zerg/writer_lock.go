// writer_lock.go —— 件 scripts/dev/writer-lock.sh（260 行）的 Go 真搬
// （非 gates 面 · M2-0 生成器/工具档 · 入口 writerLockGateMain · 已接线（inproc 表分派） / 不注册 / 不删原件）。
//
// 为什么有这件（逐处现读 scripts/dev/writer-lock.sh 现值 · 非按件名推断）：
//
//	· 形态 = 写者互斥工具（mkdir 原子锁 + owner 件），零联网 / 零 TTY；写面**只**落 --dir 指定的锁落点。
//	· 退码 = 恰 0/1/2（0 成功 · 1 确实的坏形态 · 2 不给结论）。
//	· 非确定面 = owner 件的 pid / since / since_epoch 与 show_owner 的「已持有 Ns」⇒ 对拍按掩码（见回执）。
//
// 入口签名（与原件 `$@` 对齐）：
//
//	func writerLockGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与原件位置参数同形（不含脚本名）
//	  · 退码  —— 0/1/2（见上）
//	  · 输出  —— stdout/stderr 与原件逐字节对齐（除已声明的非确定面掩码）
//
// 出厂声明 (a) 默认根定位差异：原件以 `dirname($BASH_SOURCE)/../..` 定仓根（脚本位置上溯 · 物理路径）；
// 本 Go 件无 __file__ 语境 ⇒ 走同包 repoRoot()（ZERG_REPO → 可执行件目录上溯 → cwd 上溯，判据 =
// 仓根有 core/internal/version/version.go）⇒「脚本位置 ≠ cwd/可执行件」时默认根不同（✗ 不声称逐字对齐）。
// ★ 显式 --dir 为绝对路径时两侧同值、该差异面**不适用**。
//
// 出厂声明 (b) 自指检查：本件落在 core/cmd/zerg/*.go 全件扫描面内 ⇒ 逐条消解：
//
//	· 零私有绝对路径字面量（只写 `scripts/gates/.writer-lock` 这类仓根相对串）；
//	· 零连续「引号 + 键名 + 引号」原串 / 零错误登记函数的括弧引号形态；
//	· 复核 0/0（见回执）。
//
// 本单边界（照实）：已接线（inproc 表分派） ——
//
//	· 不在 core/cmd/zerg/main.go 注册命令（✗ 不进命令树 · ✗ 不改 gate_inproc_table.go）；
//	· 不删 scripts/dev/writer-lock.sh 原件（退场规则：对拍通过后才逐件点名退场）；
//	· **已接线**：入口 `writerLockGateMain` 由 `gateCheckGoEntry["writer-lock"]` 分派（`zerg gate check writer-lock`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// ★ --self-test 走**进程内**调用（本件无接线二进制，不做 os.Executable 重入）⇒ 与原件「复制自身再
// 起 bash 子进程」是两种手段，但 18 格断言与文案逐字同构；夹具临时目录路径属非确定面。
package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// wlkLockRelDefault = 原件 LOCK_REL_DEFAULT（仓根相对）。
const wlkLockRelDefault = "scripts/gates/.writer-lock"

// wlkUsage 复刻原件 usage()（`cat <<'USAGE' … USAGE` 逐字正文 + 尾换行 · 印到 stdout）。
const wlkUsage = "用法\n" +
	"  bash scripts/dev/writer-lock.sh acquire [--dir <锁目录>] [--who <持有者>]\n" +
	"  bash scripts/dev/writer-lock.sh release [--dir <锁目录>] [--who <持有者>]\n" +
	"  bash scripts/dev/writer-lock.sh status  [--dir <锁目录>]\n" +
	"  bash scripts/dev/writer-lock.sh --help\n" +
	"  bash scripts/dev/writer-lock.sh --self-test\n" +
	"说明\n" +
	"  acquire  取锁（mkdir 原子建锁目录）；锁已存在 ⇒ 拒、逐字点名持有者与时刻（不给结论）\n" +
	"  release  放锁（先删 owner 件、再 rmdir 锁目录）；你不是持有者 ⇒ 拒（绝不代放/绝不自动破锁）\n" +
	"  status   查锁（只读）：报锁在不在、持有者是谁、已持有多久\n" +
	"  --dir    锁落点（默认 scripts/gates/.writer-lock；相对路径按仓根解析，绝对路径原样用）\n" +
	"  --who    持有者名（默认取环境 ZERG_WRITER，再退回 hostname）\n" +
	"退码\n" +
	"  0 成功 · 1 确实的坏形态（非持有者放锁 / 锁落点被非目录占着）· 2 不给结论（锁已存在 / 用法错 / 未知旗标 / 自检不过）\n"

// wlkCtx 承载一次调用的落点与 IO。root 为仓根（自检时替换为合成夹具根）。
type wlkCtx struct {
	root        string
	stdout      io.Writer
	stderr      io.Writer
	who         string
	lockDir     string
	ownerFile   string
	lockDisplay string
}

func (c *wlkCtx) warn(msg string) { fmt.Fprintf(c.stderr, "%s\n", msg) }

func wlkOrDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// wlkHostname 照 `hostname 2>/dev/null || echo unknown`。
func wlkHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}

// wlkOwnerField 照 `awk -F= -v k=KEY '$1==k { sub(/^[^=]*=/,""); print; exit }'` 的语义：
// 逐行按首个 `=` 拆；左段 == key ⇒ 回「首个 = 之后」的全部（无 `=` 的行则回整行）。首个命中即返。
func wlkOwnerField(path, key string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		left, right := line, line
		if i := strings.IndexByte(line, '='); i >= 0 {
			left, right = line[:i], line[i+1:]
		}
		if left == key {
			return right, true
		}
	}
	return "", false
}

// wlkHasLine 照 `grep -q '^<want>$'`（整行相等）。
func wlkHasLine(path, want string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if line == want {
			return true
		}
	}
	return false
}

// writeOwner 照原件 write_owner()：写 who/pid/host/since/since_epoch/dir 六行。
func (c *wlkCtx) writeOwner() bool {
	nowE := time.Now().Unix()
	nowS := time.Now().Format("2006-01-02T15:04:05-0700")
	var b strings.Builder
	fmt.Fprintf(&b, "who=%s\n", c.who)
	fmt.Fprintf(&b, "pid=%d\n", os.Getpid())
	fmt.Fprintf(&b, "host=%s\n", wlkHostname())
	fmt.Fprintf(&b, "since=%s\n", nowS)
	fmt.Fprintf(&b, "since_epoch=%d\n", nowE)
	fmt.Fprintf(&b, "dir=%s\n", c.lockDisplay)
	if err := os.WriteFile(c.ownerFile, []byte(b.String()), 0o644); err != nil {
		return false
	}
	return true
}

// wlkIsDir / wlkIsRegular 跟随符号链接（照 bash `[ -d ]` / `[ -f ]`）。
func wlkIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func wlkIsRegular(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// wlkRmdir 照 `rmdir`（只删空目录；非目录/非空 ⇒ false）。
func wlkRmdir(p string) bool {
	if !wlkIsDir(p) {
		return false
	}
	return os.Remove(p) == nil
}

// wlkDir 照 posixpath.dirname（仅用于 mkdir -p 父目录）。
func wlkDir(p string) string {
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

// showOwner 照原件 show_owner()（逐字文案）。
func (c *wlkCtx) showOwner() {
	if !wlkIsDir(c.lockDir) {
		fmt.Fprintf(c.stdout, "   锁状态：无锁（%s 不存在）\n", c.lockDisplay)
		return
	}
	if !wlkIsRegular(c.ownerFile) {
		fmt.Fprintf(c.stdout, "   锁状态：锁存在（%s）· 持有者不明（owner 件读不到）· 时刻不明\n", c.lockDisplay)
		return
	}
	w, _ := wlkOwnerField(c.ownerFile, "who")
	p, _ := wlkOwnerField(c.ownerFile, "pid")
	s, _ := wlkOwnerField(c.ownerFile, "since")
	e, _ := wlkOwnerField(c.ownerFile, "since_epoch")
	h, _ := wlkOwnerField(c.ownerFile, "host")
	age := ""
	if e != "" {
		if ep, err := strconv.ParseInt(e, 10, 64); err == nil {
			age = strconv.FormatInt(time.Now().Unix()-ep, 10) + "s"
		}
	}
	fmt.Fprintf(c.stdout, "   锁状态：锁存在（%s）\n", c.lockDisplay)
	fmt.Fprintf(c.stdout, "   持有者：%s（pid %s · 主机 %s）\n",
		wlkOrDefault(w, "不明"), wlkOrDefault(p, "不明"), wlkOrDefault(h, "不明"))
	fmt.Fprintf(c.stdout, "   持有时刻：%s · 已持有：%s\n", wlkOrDefault(s, "不明"), wlkOrDefault(age, "不明"))
}

// cmdAcquire 照原件 cmd_acquire()。
func (c *wlkCtx) cmdAcquire() int {
	if fi, err := os.Stat(c.lockDir); err == nil && !fi.IsDir() {
		c.warn(fmt.Sprintf("   ✗ 取锁被拒：锁落点被一个非目录占着（%s）⇒ 确实的坏形态", c.lockDisplay))
		return 1
	}
	_ = os.MkdirAll(wlkDir(c.lockDir), 0o755)
	// ★ mkdir 是原子原语：它的 rc 就是「锁在不在」的现读。
	if err := os.Mkdir(c.lockDir, 0o755); err != nil {
		c.warn("   ✗ 取锁被拒：锁已存在 ⇒ 不给结论（不许静默抢 · 本锁绝不自动破锁）")
		c.showOwner()
		return 2
	}
	if !c.writeOwner() {
		_ = wlkRmdir(c.lockDir)
		c.warn("   ✗ 取锁失败：owner 件写不进去 ⇒ 已回退锁目录、不给结论")
		return 2
	}
	fmt.Fprintf(c.stdout, "✅ 取锁成功：%s（持有者 %s）\n", c.lockDisplay, c.who)
	c.showOwner()
	return 0
}

// cmdRelease 照原件 cmd_release()。
func (c *wlkCtx) cmdRelease() int {
	if !wlkIsDir(c.lockDir) {
		c.warn(fmt.Sprintf("   ✗ 放锁被拒：没有锁可放（%s 不存在）⇒ 你不是持有者", c.lockDisplay))
		return 1
	}
	if !wlkIsRegular(c.ownerFile) {
		c.warn("   ✗ 放锁被拒：锁存在但 owner 件读不到 ⇒ 判不了你是不是持有者、不给结论（锁未动）")
		return 2
	}
	ow, _ := wlkOwnerField(c.ownerFile, "who")
	if ow != c.who {
		c.warn(fmt.Sprintf("   ✗ 放锁被拒：你不是持有者（本锁持有者=%s · 你=%s）", wlkOrDefault(ow, "不明"), c.who))
		c.warn("     ⇒ 不许代放（本锁绝不自动破锁 · 锁未动）")
		return 1
	}
	if err := os.Remove(c.ownerFile); err != nil {
		c.warn(fmt.Sprintf("   ✗ 放锁失败：锁目录清不掉（%s）⇒ 锁未干净释放、确实的坏形态", c.lockDisplay))
		return 1
	}
	if !wlkRmdir(c.lockDir) {
		c.warn(fmt.Sprintf("   ✗ 放锁失败：锁目录清不掉（%s）⇒ 锁未干净释放、确实的坏形态", c.lockDisplay))
		return 1
	}
	fmt.Fprintf(c.stdout, "✅ 放锁成功：%s（持有者 %s 已释放）\n", c.lockDisplay, c.who)
	return 0
}

// wlkParseAndRun = 参数解析 + 子命令分派（照原件 while/case）。root 决定相对 --dir 的解析基准。
func wlkParseAndRun(root string, stdout, stderr io.Writer, argv []string) int {
	c := &wlkCtx{root: root, stdout: stdout, stderr: stderr}
	var (
		sub      string
		dirOpt   string
		whoOpt   string
		selftest bool
	)
	for i := 0; i < len(argv); {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			io.WriteString(stdout, wlkUsage)
			return 0
		case a == "--self-test":
			selftest = true
			i++
		case a == "--dir":
			if i+1 >= len(argv) {
				io.WriteString(stderr, "用法错：--dir 缺实参（见 --help）\n")
				return 2
			}
			dirOpt = argv[i+1]
			i += 2
		case a == "--who":
			if i+1 >= len(argv) {
				io.WriteString(stderr, "用法错：--who 缺实参（见 --help）\n")
				return 2
			}
			whoOpt = argv[i+1]
			i += 2
		case a == "acquire" || a == "release" || a == "status":
			sub = a
			i++
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "用法错：未知旗标 %s（见 --help）\n", a)
			return 2
		default:
			fmt.Fprintf(stderr, "用法错：多余的位置参数 %s（见 --help）\n", a)
			return 2
		}
	}
	if selftest {
		return wlkSelftest(c)
	}
	if sub == "" {
		io.WriteString(stderr, "用法错：缺子命令（acquire|release|status · 见 --help）\n")
		return 2
	}
	c.who = wlkOrDefault(whoOpt, wlkOrDefault(os.Getenv("ZERG_WRITER"), wlkHostname()))
	d := wlkOrDefault(dirOpt, wlkLockRelDefault)
	if !strings.HasPrefix(d, "/") {
		// 照原件 `${REPO_ROOT}/${d}` 的原样拼接（✗ 不 Clean）。
		d = c.root + "/" + d
	}
	c.lockDir = d
	c.ownerFile = d + "/owner"
	c.lockDisplay = d
	switch sub {
	case "acquire":
		return c.cmdAcquire()
	case "release":
		return c.cmdRelease()
	}
	c.showOwner()
	return 0
}

// wlkMkdtempRaw 照原件 `tmp="$(mktemp -d "${TMPDIR:-/tmp}/wl-selftest.XXXXXX")"` 的**字符串拼接**语义：
//
//	· 落点 = `$TMPDIR`（缺省 /tmp）**原样**拼 `/` ＋ 模式 ＋ 6 位 `[A-Za-z0-9]` 后缀 —— ✗ 不走 filepath.Join/Clean
//	  （原件是 raw 拼接：`$TMPDIR` 尾带 `/` 时**原样保留 `//`**；用 Join 会收敛成 `/` ⇒ 两态 stdout 差一字节）；
//	· 已存在 ⇒ 换后缀重试；建不出（父目录不存在 / 无权限）⇒ 回 err（调用方照 `|| { warn …; exit 2; }` 报「自检临时目录建不起来」）。
func wlkMkdtempRaw(pattern string) (string, error) {
	base := os.TempDir() // = $TMPDIR 原样（不 Clean）；缺省 /tmp。与 bash `${TMPDIR:-/tmp}` 同值同形。
	for i := 0; i < 1000; i++ {
		name := base + "/" + pattern + wlkRandSuffix(6)
		err := os.Mkdir(name, 0o700)
		if err == nil {
			return name, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", &os.PathError{Op: "mkdir", Path: base, Err: os.ErrExist}
}

// wlkRandSuffix 产 n 位 [A-Za-z0-9]（字符集照 mktemp 的随机后缀）。属**非确定面** ⇒ 对拍按掩码。
func wlkRandSuffix(n int) string {
	const cs = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// 兜底（只求唯一）：纳秒计时逐位取。
		seed := time.Now().UnixNano()
		for i := range b {
			b[i] = cs[seed%int64(len(cs))]
			seed /= int64(len(cs))
		}
		return string(b)
	}
	for i := range b {
		b[i] = cs[int(b[i])%len(cs)]
	}
	return string(b)
}

// wlkSelftest 复刻原件 selftest() 的 8 例 18 格（改走进程内调用 · 夹具根 = 合成仓 fake）。
func wlkSelftest(c *wlkCtx) int {
	tmp, err := wlkMkdtempRaw("wl-selftest.")
	if err != nil {
		c.warn("自检临时目录建不起来")
		return 2
	}
	defer os.RemoveAll(tmp)
	fake := tmp + "/repo"
	for _, d := range []string{fake + "/scripts/gates", fake + "/scripts/dev", fake + "/bin"} {
		_ = os.MkdirAll(d, 0o755)
	}
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
	run := func(argv ...string) (int, string) {
		var ob, eb strings.Builder
		rc := wlkParseAndRun(fake, &ob, &eb, argv)
		return rc, ob.String() + eb.String()
	}

	fmt.Fprintf(c.stdout, "  —— writer-lock.sh --self-test（合成夹具 · 逐格独立锁目录 · 全在 %s 下）——\n", tmp)

	c1 := fake + "/scripts/gates/case1/.writer-lock"
	rc, _ := run("acquire", "--dir", "scripts/gates/case1/.writer-lock", "--who", "wA")
	chk("① 空目录取锁 ⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("① 锁目录已建出", wlkIsDir(c1), "")
	chk("① owner 记下持有者 wA", wlkHasLine(c1+"/owner", "who=wA"), "")

	_, _ = run("acquire", "--dir", "scripts/gates/case2/.writer-lock", "--who", "wA")
	rc, o2 := run("acquire", "--dir", "scripts/gates/case2/.writer-lock", "--who", "wB")
	chk("② 已持有再取 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	chk("② 逐字点名持有者 wA", strings.Contains(o2, "持有者：wA"), "")
	chk("② 逐字报持有时刻", strings.Contains(o2, "持有时刻："), "")

	c3 := fake + "/scripts/gates/case3/.writer-lock"
	_, _ = run("acquire", "--dir", "scripts/gates/case3/.writer-lock", "--who", "wA")
	rc, _ = run("release", "--dir", "scripts/gates/case3/.writer-lock", "--who", "wA")
	chk("③ 持有者放锁 ⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("③ 放锁后锁目录已清", !wlkIsDir(c3), "")
	rc, _ = run("acquire", "--dir", "scripts/gates/case3/.writer-lock", "--who", "wC")
	chk("③ 释放后再取锁 ⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))

	c4 := fake + "/scripts/gates/case4/.writer-lock"
	_, _ = run("acquire", "--dir", "scripts/gates/case4/.writer-lock", "--who", "wA")
	rc, o4 := run("release", "--dir", "scripts/gates/case4/.writer-lock", "--who", "wZ")
	chk("④ 非持有者放锁 ⇒ rc=1（拒）", rc == 1, fmt.Sprintf("rc=%d", rc))
	chk("④ 逐字点名你不是持有者", strings.Contains(o4, "你不是持有者"), "")
	chk("④ 锁未被放掉（仍在）", wlkIsDir(c4), "")

	rc, o5 := run("--bogus")
	chk("⑤ 未知旗标 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	chk("⑤ 逐字点名未知旗标 --bogus", strings.Contains(o5, "未知旗标 --bogus"), "")

	rc, _ = run("--help")
	chk("⑥ --help ⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))

	rc, o7 := run("status", "--dir", "scripts/gates/case4/.writer-lock")
	chk("⑦ 查锁 ⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("⑦ 查锁报出持有者 wA（只读）", strings.Contains(o7, "持有者：wA"), "")

	_ = os.MkdirAll(fake+"/scripts/gates/case5", 0o755)
	_ = os.WriteFile(fake+"/scripts/gates/case5/.writer-lock", []byte("x\n"), 0o644)
	rc, _ = run("acquire", "--dir", "scripts/gates/case5/.writer-lock", "--who", "wA")
	chk("⑧ 锁落点被非目录占着 ⇒ rc=1（拒）", rc == 1, fmt.Sprintf("rc=%d", rc))

	fmt.Fprintf(c.stdout, "  —— 自检：%d 格过 · %d 格红（夹具 %s）——\n", pass, fail, tmp)
	if fail == 0 {
		return 0
	}
	c.warn("自检有红 ⇒ 不给结论")
	return 2
}

// writerLockGateMain = 件入口（argv 与原件位置参数同形 · 不含脚本名）。
func writerLockGateMain(stdout, stderr io.Writer, argv []string) int {
	return wlkParseAndRun(repoRoot(), stdout, stderr, argv)
}
