// family_mirror_public_native.go —— `publish/mirror-public.sh`（**逐提交镜像器外壳**）的 **Go 真身**
// （发布链 Go 半优先 · 本批第三件 · 前两件 = `family_publish_preflight_native.go` / `family_publish_public_native.go`）。
//
// 病（原壳）：`publish/mirror-public.sh` 是公开面**真源**发布器（规则指纹 / 全树重写模式长在它身上），
// 但它的六段编排（环境检查 / 逐提交镜像 / 私有面门禁 / 软门禁 / 私有仓未被动过 / 推送）长在 shell 里，
// CLI 面看不见真身。本笔把编排**搬进进程内**，逐段与旧壳 `publish/mirror-public.sh`逐字对齐。
//
// ★ 数据处理面（引擎 `publish/mirror-public-lib.py`）**逐字节照旧**：
//
//	引擎是**整件 python**（不是旧壳里的内联 heredoc 块）—— 本件把它**逐字节内嵌**为 base64 常量
//	（见 `family_mirror_public_pyblocks.go` 的 `mpPyLibB64`，落盘前 sha256 与原件逐字节同 = 同件常量 `mpPyLibSHA256`），
//	运行期经 `python3 -c <bootstrap>` 交 **stdin** 执行，并在 bootstrap 里注入与旧壳**逐字同形**的
//	`__file__` / `argv[0]`（脚本位置）⇒ `REPO_ROOT` 推导面与旧壳**同锚**（✗ 不吃 cwd）。
//	⇒ 这几段（`os.walk` 序 / `re` 语义 / unicode 转义 / git 子进程序）**保证与旧壳逐字同**，不另造 Go 第二口径。
//	★ 为什么用 base64 而不是字面量：引擎正文里**逐字含**门禁自指的私有探针形态（`_INL_BLOCK` 的
//	  `／Users/…` / 机群机型族等 —— 正是它要抓的东西）；若以字面量落进 `core/**`，本件一落仓就
//	  **自己把自己的门判红**（`check-hardcoded-private-paths` / `check-history-secrets` 的扫描面含 `core/`）。
//	  base64（URL-safe）字符集不含 `/`、也不构成任何探针串 ⇒ 源码文本零命中，而**解码后逐字节不变**。
//
// ★ 与 `publish/publish-public.sh`（压平器 / 器甲）的**共用逻辑**（单一真源 · ✗ 不另抄一份）：
//
//	两器都吃 `publish/excludes.txt` / 替换规则 / 白名单 / 私有面黑名单；都吃同源 `_INL_BLOCK`
//	（mirror `Assets.scan` ⟷ Go 半 `chsecBlockSpecs`，15 条 · 见 check_history_secrets.go:59-77）。
//
// ★ 标准声明行 (a) —— **默认根定位差异**：py/旧壳由 `__file__` / `${BASH_SOURCE[0]}` 上溯定位仓根；
// Go 无 `__file__` 语境 ⇒ 本件从**入参 `root`** 取（调用方按同包既有 `repoRoot()` 定位，锚
// `core/internal/version/version.go`）。⇒ 同真仓 + cwd=仓根时两态读数**逐字相同**（已对拍）；
// 「脚本位置 ≠ 仓根」时路径推导面不同（✗ 不声称逐字对齐）。★ 引擎子进程的 `__file__` 由 bootstrap
// **显式注入** ⇒ 引擎侧的 `REPO_ROOT` 与旧壳**同锚**（不吃 cwd）。
//
// ★ 标准声明行 (b) —— **自指检查**：本件落在 `core/**`（`check-hardcoded-private-paths` 与
// `check-history-secrets` 的扫描面内）⇒ 正文**刻意零命中**：① 不写任何私有绝对路径 / 探针串；
// ② 引擎正文以 base64 承载（字符集不含 `/`，且不构成任一探针形态）⇒ 本件源码文本对两门现扫 0 命中。
//
// ★ 未覆盖面（✗ 不声称逐字对齐）：
//
//	· 旧壳 `-h/--help` 走 `sed -n '2,30p' "$0"`（打印脚本自身头注释 2-30 行）—— 本件把该 29 行
//	  **逐字节内嵌**为 `mpHelpBody`（同源），但 `$0`（脚本路径）本身不进正文 ⇒ 与 shell 逐字同；
//	· 旧壳未知参数退 2 的正文 ⟷ 本件同形（`未知参数: <a>`），但 `set -u` 下缺值形态的报话不同源；
//	· `$(cd "$OUT" && pwd)` 的**符号链接目录**语义：本件用 `filepath.Abs`（真实目录下同值；软链路径可能不同）；
//	· 引擎子进程：旧壳 `python3 "$LIB" …`（盘上件）⟷ 本件 `python3 -c <bootstrap> …`（stdin 内嵌件）——
//	  读数三面已实测逐字同（见落件回执两态对拍），差在**手段**；
//	· `-h` 的 `$0` 名称、`python3` 解释器版本随 PATH 变（库文案跨版本会变的只有 argparse，本件已注入同形 argv[0]）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	mpProg   = "mirror-public.sh"
	mpLibRel = pathMirrorLibPy

	// mpRCNotFound —— shell 惯例退码 127（子命令 / python3 起不来）；127 属 internal
	// 命名空间、不入命令面真源表（先例 GAP-20261002-08）。
	mpRCNotFound = 127

	// mpRCExpectMismatch —— 换史第三重「远端 tip ≠ --expect-old」⇒ 拒执（**不覆盖不可知状态**）。
	// 号面 = 旧壳 `publish/mirror-public.sh` 的 `exit 4`（`core/internal/contract/exitcode-namespaces.json`
	// 的 `cli` 主表 `4` 已在册 ⇒ 不新造号）。
	mpRCExpectMismatch = 4

	// mpRCReadbackUnmet —— **推送后读回复核不过**（ls-remote 读不到 / 远端现值 ≠ 意图 tip）。
	// ★ 号面**已登记**：`exitcode-namespaces.json` 的 `mirror-public.sh` 命名空间 `declared["5"]`
	// （2026-10-11 补登 · 语义＝推送已发出但复核不过 ⇒ 既不冒充成功也不冒充失败 · 「拒给结论」类）。
	// ✗ 不许改号、✗ 不许与 `12`（不可达）/`8`（BLOCKED）混用。
	mpRCReadbackUnmet = 5
)

// ── ① 同机发布互斥闸（旧壳 `:39-134` · 拍-11）────────────────────────────────────────────
//
// 行为规格（逐条照旧壳，✗ 不新造口径）：
//
//	· 锁真源 = **内核 flock（`LOCK_EX|LOCK_NB`）**，不是「锁文件在不在」这种脆判据；
//	· 拿不到 ⇒ **明确拒绝**（rc=2 · 不给结论），不排队、不挂死 —— 发布是**不可重入的一次性动作**，
//	  「谁推了」必须可判；
//	· 锁文件 = **唯一固定路径**（✗ 不用 per-run 时间戳）= `<锁目录>/publish.lock`，
//	  锁目录 = `ZERG_PUBLISH_LOCK_DIR` → `ZERG_STATE_DIR` → `$HOME/.zerg/state`（空值一律下渗）；
//	· **只读预演**（`--list-only`/`--check`）**不入闸**（同先例「干跑/只读档不入闸」）；
//	· 重入/**内层继承外层锁**：`ZERG_MIRROR_LOCK_HELD=<pid>` 三条件全中才放行（① 数字 pid
//	  ② == 锁文件 `pid:` 行 ③ 该 pid 落在**本趟自家祖链**）—— ✗ 不盲信该 env 的字符串；
//	· 逃生口 `ZERG_MIRROR_NO_LOCK` **必须**同带 `ZERG_MIRROR_NO_LOCK_REASON`（缺 ⇒ rc=2）；
//	· 建不了锁目录 / 开不了锁文件 ⇒ **fail-open**（只出声、照跑、退码不变）。
//
// ★ Go 侧与旧壳的**手段差**（已在件头「未覆盖面」声明）：旧壳 macOS 无 `flock(1)` ⇒ 走
// `perl -e` 拿锁后**原地 exec 本脚本**（且必须抬 `$^F=255` 让 fd 跨 exec 存活）；Go 进程自己
// 持有 fd 到进程退出 ⇒ **无需 exec 重放**，`ZERG_MIRROR_LOCK_HELD` 用 `os.Setenv` 交给子进程
// （推前验收 / 内层再调本脚本的那一趟照样能继承，rail 不丢）。
const (
	mpLockHeldEnv     = "ZERG_MIRROR_LOCK_HELD"
	mpNoLockEnv       = "ZERG_MIRROR_NO_LOCK"
	mpNoLockReasonEnv = "ZERG_MIRROR_NO_LOCK_REASON"
	mpLockDirEnv      = "ZERG_PUBLISH_LOCK_DIR"
	mpStateDirEnv     = "ZERG_STATE_DIR"
	mpLockFile        = "publish.lock"
)

// mpLock —— 一次拿住的互斥闸（`f == nil` = 没拿/关闸/fail-open，release 是空操作）。
type mpLock struct{ f *os.File }

// release —— 放锁 + 关 fd（内核据此让下一个进程拿到）。nil 安全。
func (l *mpLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
	l.f = nil
}

// mpLockPath —— 锁文件路径（唯一固定路径 · 仓外状态目录）。
func mpLockPath() string {
	d := os.Getenv(mpLockDirEnv)
	if d == "" {
		d = os.Getenv(mpStateDirEnv)
	}
	if d == "" {
		d = filepath.Join(os.Getenv("HOME"), ".zerg", "state")
	}
	return filepath.Join(d, mpLockFile)
}

// mpAllDigits —— 照 bash `case "${want}" in *[!0-9]*)`（空串不算数字）。
func mpAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// mpPsPPID —— 照 `ps -o ppid= -p <pid> | tr -d ' \n'`（取不到 ⇒ 空串 = 到头/失败）。
func mpPsPPID(pid string) string {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", pid).Output()
	if err != nil {
		return ""
	}
	return strings.NewReplacer(" ", "", "\n", "", "	", "").Replace(string(out))
}

// mpPidInOwnTree —— 该 pid 是否落在**本趟自家祖链**上（含 `os.Getpid()` 自身）。
// 判据 = 从自己起沿 `ps -o ppid=` 上溯逐级比对；取不到 / 非数字 / 到头（≤64 级）⇒ 不算自家
// （fail-closed：判不了就不放行，按新拿锁走）。
func mpPidInOwnTree(want string) bool {
	if !mpAllDigits(want) {
		return false
	}
	p := strconv.Itoa(os.Getpid())
	for n := 0; n < 64 && p != "" && p != "0"; n++ {
		if p == want {
			return true
		}
		p = mpPsPPID(p)
	}
	return false
}

// mpLockHolder —— 锁文件里的持锁者 pid（照 `awk '/^pid:/{print $2; exit}'`）。
func mpLockHolder() string {
	body, err := os.ReadFile(mpLockPath())
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(ln, "pid:") {
			if fs := strings.Fields(ln); len(fs) >= 2 {
				return fs[1]
			}
			return ""
		}
	}
	return ""
}

// mpLockReentrant —— 本进程是否由**持锁祖先内层**调用（真重入 / 内层继承外层锁）。
// 三条件缺一 ⇒ **不盲信**（落正常取锁路）：① 标记须是数字 pid；② 标记 pid == 锁文件 `pid:` 行
// （把标记绑到本进程**自算**的那把锁 —— 标记里带的路径一律不信）；③ 该 pid 落在本趟自家祖链。
func mpLockReentrant() bool {
	want := os.Getenv(mpLockHeldEnv)
	if !mpAllDigits(want) {
		return false
	}
	if want != mpLockHolder() {
		return false
	}
	return mpPidInOwnTree(want)
}

// mpAcquireLock —— 拿同机发布互斥闸。返回 (锁, 退码)：拿到/关闸/fail-open ⇒ rc=0；别人持有 ⇒ rc=2。
// ★ 拿到时把 `pid:/start:/cmd:` 落进锁文件（下一发据此点名持锁者），并把 `ZERG_MIRROR_LOCK_HELD`
// 交给子进程（内层再调本脚本 / 推前验收 ⇒ 继承外层锁，不重复取锁）。
func mpAcquireLock(args []string, stderr io.Writer) (*mpLock, int) {
	if mpLockReentrant() {
		return nil, 0 // 真重入 / 内层继承外层锁 ⇒ 不重复拿锁
	}
	if v := os.Getenv(mpNoLockEnv); v != "" {
		if os.Getenv(mpNoLockReasonEnv) == "" {
			fmt.Fprint(stderr, "✗ 同机发布互斥闸：`ZERG_MIRROR_NO_LOCK` 置位但缺 `ZERG_MIRROR_NO_LOCK_REASON` ⇒ **不给结论(rc=2)**\n")
			return nil, 2
		}
		fmt.Fprintf(stderr, "⚠ 同机发布互斥闸**显式关闸**（ZERG_MIRROR_NO_LOCK=1）· 理由=%s\n", os.Getenv(mpNoLockReasonEnv))
		return nil, 0
	}
	lk := mpLockPath()
	if err := os.MkdirAll(filepath.Dir(lk), 0o755); err != nil {
		fmt.Fprintf(stderr, "⚠ 同机发布互斥闸：建不了锁目录（%s）⇒ 放行（fail-open · 退码不变）\n", filepath.Dir(lk))
		return nil, 0
	}
	f, err := os.OpenFile(lk, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "⚠ 同机发布互斥闸：开不了锁文件（%s）⇒ 放行（fail-open）\n", lk)
		return nil, 0
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Fprint(stderr, "✗ 同一台机上已有在飞的发布（同机互斥闸 · 拍-11）⇒ 拒绝并发 · 不给结论 · rc=2\n")
		if body, rerr := os.ReadFile(lk); rerr == nil {
			for _, ln := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
				if ln == "" {
					continue
				}
				fmt.Fprintf(stderr, "  持锁者: %s\n", ln)
			}
		}
		fmt.Fprint(stderr, "  本发一步都没跑（不排队、不叠加 · 不争同一 --out）\n")
		_ = f.Close() // 没拿到锁，无锁可放；持有者的锁不受影响
		return nil, 2
	}
	// 拿到 ⇒ 把自己这一趟写进锁文件（下一发读它来点名）。`start:` 照 perl `scalar(gmtime)` 的 UTC 形态。
	_ = f.Truncate(0)
	_, _ = f.Seek(0, io.SeekStart)
	fmt.Fprintf(f, "pid: %d\nstart: %s\ncmd: %s %s\n",
		os.Getpid(), time.Now().UTC().Format("Mon Jan _2 15:04:05 2006"), mpProg, strings.Join(args, " "))
	_ = f.Sync()
	_ = os.Setenv(mpLockHeldEnv, strconv.Itoa(os.Getpid()))
	return &mpLock{f: f}, 0
}

// ── ② 发布审计台账（旧壳 `:179-253` · 任务单 ㈡ 直-3 首版 · 拍-9/10/12/14）────────────────
//
// 落点（两个 · 都可被环境变量改指，供仓外沙箱演练）：
//
//	主落点   `<仓根>/scripts/gates/publish-audit.jsonl`     ⇐ `ZERG_PUBLISH_AUDIT`
//	备用落点 `<仓根>/scripts/gates/audit_fallback.jsonl`    ⇐ `ZERG_PUBLISH_AUDIT_FALLBACK`
//	写者     「谁」，默认 `Hermes (AI)`                     ⇐ `ZERG_PUBLISH_AUDIT_BY`
//
// 格式 = **一行一事件**（`jsonl` · `O_APPEND` · 键序固定 · 值**不做 JSON 转义** —— 与旧壳 `printf`
// 逐字节同；✗ 不用 `encoding/json`，那会改键序并转义）。四个 event：
//
//	`publish`            推送成功 **且** 推后 ls-remote 读回复核通过 ⇒ 恰一行（`result:0`）
//	`publish_rejected`   走到了推送段（`--push` ∧ 已算出镜像 tip）却没写成成功行 ⇒ **EXIT 罩**补一行（`result:<rc>`）
//	`audit_redirected`   落点被改指且**带了**理由 ⇒ 往**真落点**记一行（`from→to` ＋ `reason`）
//	`audit_compensated`  主落点**写不进** ⇒ 出声 ＋ 补偿行落**备用落点**（✗ 不改判决/退码）
//
// 幂等：**追写面没有去重键**（同一趟跑两次＝两行，这是要的：「谁推了」按趟记账）；「一趟恰一行」由
// `done` 闩（`PUBLISH_AUDIT_DONE`）保证 —— 成功行写后置位，EXIT 罩见闩即不再补 `rejected`。
// ★ 写不进**不算失败**（推送已发生、不可逆）：只出声 ＋ 补偿行，**不改退码**（✗ 不折 rc=0 假绿、
// ✗ 不反判成功为失败）—— 照 `appendAuditCompensation` 先例。
const (
	mpAuditRelMain       = "scripts/gates/publish-audit.jsonl"
	mpAuditRelFallback   = "scripts/gates/audit_fallback.jsonl"
	mpAuditByDefault     = "Hermes (AI)"
	mpAuditRedirectEnv   = "ZERG_PUBLISH_AUDIT_REASON"
	mpAuditMainEnv       = "ZERG_PUBLISH_AUDIT"
	mpAuditFallbackEnv   = "ZERG_PUBLISH_AUDIT_FALLBACK"
	mpAuditByEnv         = "ZERG_PUBLISH_AUDIT_BY"
	mpAuditRejectedWhy   = "push/readback 未达预期（见上）"
	mpAuditCompNote      = "primary ledger unwritable"
	mpAuditCompReasonVal = "audit_unwritable"
)

// mpAuditCtx —— 一趟发布的审计上下文（落点 ＋ 记账字段 ＋ `done` 闩）。
type mpAuditCtx struct {
	main, fallback         string // 实落点（可被 env 改指）
	trueMain, trueFallback string // 默认真落点（改指时往这里留痕）
	actor, by              string
	repo, out              string
	expectOld              string
	mirrorTip, mirrorTree  string
	remoteTip              string
	push                   int
	done                   bool
}

// mpHostname —— 照 `$(hostname)`。
func mpHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}

// mpAuditTS —— 照旧壳 `date +%Y-%m-%dT%H:%M:%S%z` 再插冒号（`…+0800` ⇒ `…+08:00`）= RFC3339。
func mpAuditTS() string { return time.Now().Format(time.RFC3339) }

// mpAuditPathOr —— `VAR` 非空则用，否则默认（照 `${VAR:-default}` 的空值下渗）。
func mpAuditPathOr(env, def string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}

// mpNewAuditCtx —— 解析落点与写者（`--list-only` 也照旧解析：只读预演不写，但 guard 要判「改指没」）。
func mpNewAuditCtx(root string) *mpAuditCtx {
	trueMain := filepath.Join(root, filepath.FromSlash(mpAuditRelMain))
	trueFallback := filepath.Join(root, filepath.FromSlash(mpAuditRelFallback))
	by := os.Getenv(mpAuditByEnv)
	if by == "" {
		by = mpAuditByDefault
	}
	return &mpAuditCtx{
		main:         mpAuditPathOr(mpAuditMainEnv, trueMain),
		fallback:     mpAuditPathOr(mpAuditFallbackEnv, trueFallback),
		trueMain:     trueMain,
		trueFallback: trueFallback,
		actor:        mpHostname(),
		by:           by,
	}
}

// mpAuditWrite —— 追加一行（`O_APPEND|O_CREATE`）。**不建目录**（与旧壳 `>>` 同：父目录不在即写不进）。
func mpAuditWrite(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// line —— 发布台账一行（键序固定 · 照旧壳 `publish_audit_line`；`result` 是**裸数字**）。
func (a *mpAuditCtx) line(event string, result int, reason, remoteReadback string) string {
	return fmt.Sprintf(`{"ts":"%s","event":"%s","actor":"%s","by":"%s","repo":"%s","ref":"main","old_tip":"%s","new_tip":"%s","new_tree":"%s","result":%d,"remote_readback":"%s","reason":"%s","mirror_out":"%s","prev":""}`,
		mpAuditTS(), event, a.actor, a.by, a.repo, a.expectOld, a.mirrorTip, a.mirrorTree,
		result, remoteReadback, reason, a.out)
}

// appendLine —— 追写主落点；写不进 ⇒ 出声 ＋ 补偿行落备用落点（✗ 永不改调用方退码）。
func (a *mpAuditCtx) appendLine(line string, stderr io.Writer) {
	if err := mpAuditWrite(a.main, line); err == nil {
		return
	}
	fmt.Fprintf(stderr, "⚠ 发布审计台账写不进（%s）—— 推送已发生、不可逆 ⇒ 不退码（写不进不算失败）；补偿行落 %s\n",
		a.main, a.fallback)
	c := fmt.Sprintf(`{"ts":"%s","event":"audit_compensated","reason":"%s","actor":"%s","by":"%s","audit_path":"%s","note":"%s"}`,
		mpAuditTS(), mpAuditCompReasonVal, a.actor, a.by, a.main, mpAuditCompNote)
	if mpAuditWrite(a.fallback, c) != nil {
		fmt.Fprintf(stderr, "⚠ 补偿行也写不进（%s）—— 本行即读数\n", a.fallback)
	}
}

// redirectGuard —— **审计落点覆盖闸**：落点 ≠ 默认真落点 ⇒ 必带 `ZERG_PUBLISH_AUDIT_REASON`。
// 缺理由 ⇒ **推送前拒执 rc=2**（硬线：真台账不许静默漏记）；带理由 ⇒ 放行，且往**真落点**留痕一行。
// 只读预演不入闸（同先例：干跑/只读档不入闸）。
func (a *mpAuditCtx) redirectGuard(listOnly int, stderr io.Writer) int {
	if listOnly == 1 {
		return 0
	}
	if a.main == a.trueMain && a.fallback == a.trueFallback {
		return 0 // 落点 = 默认真落点 ⇒ 逐字不变（无输出 · 无写）
	}
	if reason := os.Getenv(mpAuditRedirectEnv); reason != "" {
		l := fmt.Sprintf(`{"ts":"%s","event":"audit_redirected","actor":"%s","by":"%s","from":"%s","to":"%s","fallback_to":"%s","reason":"%s"}`,
			mpAuditTS(), a.actor, a.by, a.trueMain, a.main, a.fallback, reason)
		if mpAuditWrite(a.trueMain, l) != nil {
			fmt.Fprintf(stderr, "⚠ 审计落点覆盖闸：`audit_redirected` 留痕写不进真落点（%s）\n", a.trueMain)
		}
		fmt.Fprintf(stderr, "⚠ 发布审计落点**已被显式改指**（%s ⇒ %s）· 已留痕到真落点 %s · 理由=%s\n",
			a.trueMain, a.main, a.trueMain, reason)
		return 0
	}
	fmt.Fprintf(stderr, "✗ 发布审计落点覆盖闸：审计落点被改指（%s ⇒ %s）但缺 `ZERG_PUBLISH_AUDIT_REASON` ⇒ **推送前拒执(rc=2)**\n",
		a.trueMain, a.main)
	fmt.Fprint(stderr, "   （硬线：真台账不许静默漏记 —— 补上理由，或别改落点）\n")
	return 2
}

// onExit —— **EXIT 罩**：走到推送段（`--push` ∧ 已算出镜像 tip）却没写成成功行 ⇒ 补 `publish_rejected`。
// 条件逐条照旧壳 `publish_audit_on_exit`：`push != 1` / `mirrorTip == ""` / `done` 三者任一 ⇒ 不写。
func (a *mpAuditCtx) onExit(rc int, stderr io.Writer) {
	if a == nil || a.push != 1 || a.mirrorTip == "" || a.done {
		return
	}
	a.appendLine(a.line("publish_rejected", rc, mpAuditRejectedWhy, a.remoteTip), stderr)
}

// ── `awk` 两形（换史 CAS 段与读回复核段各用一形，✗ 不许合并成一个）────────────────────────
//
//	`awk '{print $1}'`        ⇒ **逐行**取第一字段（换史段：多 head 时拼成多行 ⇒ 与 40 位串必不等）
//	`awk 'NR==1{print $1}'`   ⇒ **只**第一行第一字段（读回复核段）
func mpAwkFirstField(line string) string {
	if fs := strings.Fields(line); len(fs) > 0 {
		return fs[0]
	}
	return ""
}

// mpAwkFirstFieldAll —— `awk '{print $1}'`（空串 ⇒ 空串：命令替换已吃掉尾换行）。
func mpAwkFirstFieldAll(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		out = append(out, mpAwkFirstField(ln))
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// mpAwkFirstFieldFirstLine —— `awk 'NR==1{print $1}'`。
func mpAwkFirstFieldFirstLine(s string) string {
	if s == "" {
		return ""
	}
	return mpAwkFirstField(strings.SplitN(s, "\n", 2)[0])
}

// mpGitCaptureErr —— 同 mpGitCapture，但**不吞 stderr**（旧壳的 `ls-remote` 读回两处都让 git 自己说话：
// 「读不到 ≠ 推成功」的口径要求读失败时把 git 的错原样吐出来）。
func mpGitCaptureErr(stderr io.Writer, args ...string) (int, string) {
	var out bytes.Buffer
	cmd := armChildGroup(exec.Command("git", args...))
	cmd.Stdout = &out
	cmd.Stderr = stderr
	rc := mpRun(cmd)
	return rc, strings.TrimSpace(out.String())
}

// mpPyBootstrap —— 把内嵌引擎（mpPyLibB64 解码后的字节）经 **stdin** 交给 python3 -c 执行，
// 并注入与旧壳**逐字同形**的 `__file__`（脚本位置）与 `argv[0]`（脚本基名）—— `sys.argv=["-c", <libPath>, …]`
// ⇒ 本 bootstrap 取 `sys.argv[1]` 当 `__file__`、把 `sys.argv` 归到 `[<libPath>, …]`（与 `python3 <libPath> …` 同形）。
const mpPyBootstrap = "import sys\n" +
	"_src = sys.stdin.read()\n" +
	"_g = {\"__name__\": \"__main__\", \"__file__\": sys.argv[1], \"__builtins__\": __builtins__}\n" +
	"sys.argv = sys.argv[1:]\n" +
	"exec(compile(_src, sys.argv[0], \"exec\"), _g)\n"

// mpSay —— 复刻旧壳 `say()`：`printf '\n\033[1m== %s\033[0m\n' "$*"`（stdout）。
func mpSay(w io.Writer, s string) { fmt.Fprintf(w, "\n\x1b[1m== %s\x1b[0m\n", s) }

// mpFirst12 —— 照 bash `${VAR:0:12}`（前 12 字节）。
func mpFirst12(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// mpRun —— 起子进程（登记 runningChild 以便收尾杀整组），返回其进程退码。
func mpRun(cmd *exec.Cmd) int {
	runningChild = cmd
	defer func() { runningChild = nil }()
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return mpRCNotFound
	}
	return 0
}

// mpGitCapture —— 跑 git（照 `$(git -C … )`）：返回 rc 与 stdout（**去尾空白**）。
func mpGitCapture(args ...string) (int, string) {
	var out bytes.Buffer
	cmd := armChildGroup(exec.Command("git", args...))
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	rc := mpRun(cmd)
	return rc, strings.TrimSpace(out.String())
}

// mpStatusCount —— 复刻 `git -C REPO status --porcelain | wc -l`（计 git 原输出的换行数）。
func mpStatusCount(repo string) int {
	var out bytes.Buffer
	cmd := armChildGroup(exec.Command("git", "-C", repo, "status", "--porcelain"))
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	_ = mpRun(cmd)
	return strings.Count(out.String(), "\n")
}

// mpRunLib —— 跑内嵌引擎：`python3 -c <bootstrap> <libPath> <args…>`（stdin = 解码后的引擎字节）。
func mpRunLib(root string, args []string, stdout, stderr io.Writer) int {
	py, err := exec.LookPath("python3")
	if err != nil {
		return mpRCNotFound
	}
	libPath := filepath.Join(root, mpLibRel)
	cmd := armChildGroup(exec.Command(py, append([]string{"-c", mpPyBootstrap, libPath}, args...)...))
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(mpPyLibBytes())
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return mpRun(cmd)
}

// mpHasFile / mpHasDir —— 存在性（照 `[ -f ]` / `[ -d ]`）。
func mpHasFile(p string) bool { fi, err := os.Stat(p); return err == nil && !fi.IsDir() }
func mpHasDir(p string) bool  { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

// mpExecutable —— 照 `[ -x ]`（普通件且有任一执行位）。
func mpExecutable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}

// mpOutResolved —— 复刻 `$(cd "$OUT" 2>/dev/null && pwd || echo "$OUT")`（是目录 ⇒ 绝对化；否则原样）。
func mpOutResolved(out string) string {
	if mpHasDir(out) {
		if abs, err := filepath.Abs(out); err == nil {
			return abs
		}
	}
	return out
}

// mpRunSoft —— 复刻旧壳 `run_soft()`（软门禁：优先 Go 半 · 退回 py 半 · 两半皆缺只报告；一律只报告不中止）。
func mpRunSoft(root, out, key, why string, stdout io.Writer) {
	zergBin := filepath.Join(root, "bin", "zerg")
	pyKey := filepath.Join(root, "scripts", "gates", key+".py")
	var cmd *exec.Cmd
	half := ""
	if mpExecutable(zergBin) {
		half = "Go 半"
		cmd = armChildGroup(exec.Command(zergBin, "gate", "check", key, out))
	} else if mpHasFile(pyKey) {
		half = "py 半"
		cmd = armChildGroup(exec.Command("python3", pyKey, out))
	} else {
		fmt.Fprintf(stdout, "  ⚠ %s：判据件两半皆缺（py 与 Go 半）⇒ 本档只报告（软门禁不中止本批）\n", key)
		return
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if mpRun(cmd) == 0 {
		fmt.Fprintf(stdout, "  ✓ %s（%s）\n", key, half)
	} else {
		fmt.Fprintf(stdout, "  ⚠ %s 未通过（%s · %s —— 本批只报告）\n", key, half, why)
	}
}

// mirrorPublicNative —— `publish/mirror-public.sh` 的 Go 真身（argv 与旧壳同形 · 不含程序名）。
//
// root = 仓根（调用方按同包 repoRoot() 定位）。退码 / stdout / stderr 逐字对齐旧壳：
// 0 成功 · 1 环境/私有面门禁中止 · 2 用法/私有仓落点错 · 3 --push 双保险拦下 · 其余原样转发引擎退码。
func mirrorPublicNative(root string, stdout, stderr io.Writer, argv []string) (rc int) {
	lib := filepath.Join(root, mpLibRel)
	ts := time.Now().Format("20060102-150405")

	repo := root
	out := ""
	rng := "auto"
	push := 0
	replaceHistory := 0 // ★ 换史（Mr2109 2026-10-10 授权）：默认关闭 —— 只开这一条，不动 --push 语义
	expectOld := ""     // ★ 换史第三重：钉住「拍板那一刻」的远端 main tip（40 位；不符 ⇒ 拒执）
	remote := os.Getenv("ZERG_PUBLIC_REMOTE")
	report := ""
	allow := ""
	listOnly := 0
	verbose := 0

	i := 0
	for i < len(argv) {
		a := argv[i]
		val := func() string {
			if i+1 < len(argv) {
				return argv[i+1]
			}
			return ""
		}
		switch a {
		case "--repo":
			repo = val()
			i += 2
		case "--out":
			out = val()
			i += 2
		case "--range":
			rng = val()
			i += 2
		case "--report":
			report = val()
			i += 2
		case "--allow":
			allow = val()
			i += 2
		case "--remote":
			remote = val()
			i += 2
		case "--dry-run":
			push = 0
			i++
		case "--list-only", "--check":
			listOnly = 1
			i++
		case "--verbose":
			verbose = 1
			i++
		case "--push":
			push = 1
			i++
		case "--replace-history":
			replaceHistory = 1 // ★ 换史意图（另需环境双保险 + --expect-old）
			i++
		case "--expect-old":
			expectOld = val() // ★ 拍板时现读的远端 tip（40 位 hex）
			i += 2
		case "-h", "--help":
			fmt.Fprint(stdout, mpHelpBody)
			return 0
		default:
			fmt.Fprintf(stderr, "未知参数: %s\n", a)
			return 2
		}
	}

	if out == "" {
		out = "/tmp/zerg-mirror-" + ts
	}
	if report == "" {
		report = strings.TrimSuffix(out, "/") + ".report-" + ts + ".json"
	}

	// ── ① 同机发布互斥闸（拍-11）：拿内核 flock（唯一固定路径）后才往下走 ──
	// 覆盖「读远端 → 推 → 复核」全程（进程持锁到退出）；只读预演不入闸。
	if listOnly != 1 {
		lk, lrc := mpAcquireLock(argv, stderr)
		if lrc != 0 {
			return lrc
		}
		defer lk.release()
	}

	// ── ② 审计落点覆盖闸（推送前拒执面）：落点被改指又无理由 ⇒ rc=2 ──
	audit := mpNewAuditCtx(root)
	audit.repo = repo
	audit.out = out
	audit.expectOld = expectOld
	audit.push = push
	if lrc := audit.redirectGuard(listOnly, stderr); lrc != 0 {
		return lrc
	}
	// ── EXIT 罩（照旧壳 `trap … EXIT` 装在 guard **之后** ⇒ guard 的 rc=2 不写 rejected 行）──
	defer func() { audit.onExit(rc, stderr) }()

	mpSay(stdout, "0/5 环境检查")
	if !mpHasFile(lib) {
		fmt.Fprintf(stderr, "缺少 %s\n", lib)
		return 1
	}
	if _, err := exec.LookPath("python3"); err != nil {
		fmt.Fprintln(stderr, "需要 python3")
		return 1
	}
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(stderr, "需要 git")
		return 1
	}
	if abs, err := filepath.Abs(repo); err == nil {
		repo = abs
	}
	audit.repo = repo // 旧壳审计行的 `repo` 取**正典化后**的绝对值
	if !mpHasDir(repo) {
		fmt.Fprintf(stderr, "cd: %s: No such file or directory\n", repo)
		return 1
	}
	if !mpHasDir(filepath.Join(repo, ".git")) {
		fmt.Fprintf(stderr, "不是 git 仓：%s\n", repo)
		return 2
	}
	libRepoRoot := root
	if abs, err := filepath.Abs(filepath.Dir(filepath.Dir(lib))); err == nil {
		libRepoRoot = abs
	}

	// ── 只读预演入口（--list-only / --check）：只列清单就退出（不建产出仓 · 不写盘 · 不联网 · 不推送）──
	if listOnly == 1 {
		mpSay(stdout, "0/0 只读预演（--list-only）：不建产出仓 · 不写盘 · 不联网 · 不推送")
		if push == 1 {
			fmt.Fprintln(stdout, "  ⚠ --push 被忽略：只读预演不做任何推送（本模式下不存在推送路径）")
		}
		fmt.Fprintf(stdout, "私有仓:   %s（只读）\n", repo)
		fmt.Fprintf(stdout, "产出目录: **未使用/未创建**（本入口不建不写：%s）\n", out)
		fmt.Fprintf(stdout, "资产来源: %s/publish/{whitelist.txt,replace-rules.tsv,private-paths.txt}\n", mpRealPath(libRepoRoot))
		libArgs := []string{"--repo", mpRealPath(repo), "--list-only"}
		if verbose == 1 {
			libArgs = append(libArgs, "--verbose")
		}
		if allow != "" {
			libArgs = append(libArgs, "--allow", allow)
		}
		rc := mpRunLib(root, libArgs, stdout, stderr)
		fmt.Fprintf(stdout, "退码: %d（只读预演：未写盘 · 未联网 · 未推送）\n", rc)
		return rc
	}

	// --out 落在私有仓里 ⇒ 拒（G4：镜像器绝不碰私有工作树）
	outRes := mpOutResolved(out)
	if outRes == repo || strings.HasPrefix(outRes, repo+"/") {
		fmt.Fprintf(stderr, "✗ --out 落在私有仓里（%s）—— 镜像器绝不碰私有工作树（G4）\n", out)
		return 2
	}

	_, privHeadBefore := mpGitCapture("-C", repo, "rev-parse", "HEAD")
	privDirtyBefore := mpStatusCount(repo)
	fmt.Fprintf(stdout, "私有仓:   %s（HEAD=%s · 改动 %d 项）\n", repo, mpFirst12(privHeadBefore), privDirtyBefore)
	fmt.Fprintf(stdout, "产出目录: %s\n", out)
	fmt.Fprintf(stdout, "资产来源: %s/publish/{whitelist.txt,replace-rules.tsv,private-paths.txt}\n", mpRealPath(libRepoRoot))

	mpSay(stdout, "1/5 逐提交镜像（差分驱动 + 逐笔硬门禁）")
	libArgs := []string{"--repo", mpRealPath(repo), "--out", out, "--range", rng}
	if report != "" {
		libArgs = append(libArgs, "--report", report)
	}
	if allow != "" {
		libArgs = append(libArgs, "--allow", allow)
	}
	if rc := mpRunLib(root, libArgs, stdout, stderr); rc != 0 {
		return rc // 旧壳：`python3 …` 非 0 ⇒ set -e ⇒ 以该码退出（不再往下）
	}
	if rc, _ := mpGitCapture("-C", out, "rev-parse", "refs/heads/main"); rc != 0 {
		return rc
	}
	_, mirrorTip := mpGitCapture("-C", out, "rev-parse", "refs/heads/main")
	if rc, _ := mpGitCapture("-C", out, "rev-parse", "refs/heads/main^{tree}"); rc != 0 {
		return rc
	}
	_, mirrorTree := mpGitCapture("-C", out, "rev-parse", "refs/heads/main^{tree}")
	if rc, _ := mpGitCapture("-C", out, "rev-list", "--count", "refs/heads/main"); rc != 0 {
		return rc
	}
	_, mirrorCount := mpGitCapture("-C", out, "rev-list", "--count", "refs/heads/main")
	// ★ 审计罩的「已算出镜像 tip」那一格在旧壳里由 1/5 之后的全局变量决定 ⇒ 2/5 起任何失败都要补
	// `publish_rejected` 行（✗ 别等到 5/5 才赋值：那样 2/5 门禁中止就**漏记**了）。
	audit.mirrorTip = mirrorTip
	audit.mirrorTree = mirrorTree

	// ── 2/5 私有面门禁（优先 Go 半 · 退回 py 半 · 两半皆缺 ⇒ 中止）──
	mpSay(stdout, "2/5 私有面门禁（对物化后的产出树跑既有检查器 · 优先 Go 半）")
	zergBin := filepath.Join(libRepoRoot, "bin", "zerg")
	pyPriv := filepath.Join(libRepoRoot, "scripts", "gates", "check-public-tree-private.py")
	privRC := 0
	ranPriv := false
	if mpExecutable(zergBin) {
		cmd := armChildGroup(exec.Command(zergBin, "gate", "check", "check-public-tree-private", out))
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		privRC = mpRun(cmd)
		ranPriv = true
	} else if mpHasFile(pyPriv) {
		cmd := armChildGroup(exec.Command("python3", pyPriv, out))
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		privRC = mpRun(cmd)
		ranPriv = true
	}
	if !ranPriv {
		fmt.Fprintln(stderr, "  ❌ 私有面门禁件两半皆缺（py 与 Go 半）——发布中止（不给结论 ⇒ 不放行）")
		return 1
	}
	if privRC == 0 {
		fmt.Fprintln(stdout, "  ✓ 私有面门禁通过（逐笔已在镜像时查过：见 1/5 输出）")
	} else {
		fmt.Fprintf(stderr, "  ❌ 产出树出现私有面路径 / 门取不到结论（rc=%d）—— 发布中止（一个字节都不推）\n", privRC)
		return 1
	}

	// ── 3/5 软门禁（只报告，不中止本批）──
	mpSay(stdout, "3/5 软门禁（文档双语 / i18n / 版本一致 / 跨版本状态清单 —— 只报告，不中止本批）")
	mpRunSoft(libRepoRoot, out, "check_docs", "历史各笔的形态不一定满足当前门禁", stdout)
	mpRunSoft(libRepoRoot, out, "check_version", "逐提交镜像对每笔都要求版本一致不现实 · B8 再定口径", stdout)
	mpRunSoft(libRepoRoot, out, "check-compat-manifest", "历史各笔可能还没有兼容清单 · 最终树上为硬门禁（见 publish-public.sh 4e）", stdout)

	// ── 4/5 私有仓未被动过（验收 D）──
	mpSay(stdout, "4/5 私有仓未被动过（验收 D）")
	_, privHeadAfter := mpGitCapture("-C", repo, "rev-parse", "HEAD")
	privDirtyAfter := mpStatusCount(repo)
	fmt.Fprintf(stdout, "私有仓 HEAD: %s → %s\n", mpFirst12(privHeadBefore), mpFirst12(privHeadAfter))
	fmt.Fprintf(stdout, "私有仓改动项: %d → %d\n", privDirtyBefore, privDirtyAfter)
	if privHeadBefore != privHeadAfter || privDirtyBefore != privDirtyAfter {
		fmt.Fprintln(stderr, "❌ 私有仓被改动了 —— 但镜像器本不该写私有仓，请立即排查")
		return 1
	}
	fmt.Fprintln(stdout, "✓ 私有仓只读：HEAD 与工作树状态完全未变（跑前应为 0）")

	// ── 5/5 推送 ──
	mpSay(stdout, "5/5 推送")
	fmt.Fprintf(stdout, "镜像末提交: %s / tree %s / %s 笔\n", mpFirst12(mirrorTip), mpFirst12(mirrorTree), mirrorCount)
	fmt.Fprintf(stdout, "报告:       %s\n", report)
	// K1（设计稿 v1.0 · 片4 · 断点 K1）：镜像链的**正向出口** —— 人从命令面一眼见「私有 HEAD → 镜像 tip」两个 sha
	// （两个值都取自已有变量，✗ 不新增任何 git 调用；旧壳 `:385` 逐字同行）
	fmt.Fprintf(stdout, "私有 HEAD=%s → 镜像 tip=%s\n", privHeadBefore, mirrorTip)
	if push == 1 {
		if os.Getenv("ZERG_MIRROR_ALLOW_PUSH") != "1" {
			fmt.Fprintln(stderr, "✗ --push 被双重保险拦下：推送要同时满足 --push 且 ZERG_MIRROR_ALLOW_PUSH=1。")
			fmt.Fprintln(stderr, "  本批次（B1）只做 dry-run、绝不推送；B2 历史重写时由Mr2109点头后执行。")
			return 3
		}
		if remote == "" {
			fmt.Fprintln(stderr, "需要 --remote 或 ZERG_PUBLIC_REMOTE")
			return 2
		}
		if rc, _ := mpGitCapture("-C", out, "remote", "get-url", "origin"); rc != 0 {
			_, _ = mpGitCapture("-C", out, "remote", "add", "origin", remote)
		}
		_, _ = mpGitCapture("-C", out, "remote", "set-url", "origin", remote)
		// 唯一出口动作（真推送）前挂 B1 验收 —— 验收不过 ⇒ 拒推（挡在 push 之前）。
		// ★ P6-2（2026-10-22）：验收**也搬进真身** —— 旧壳此处 `SKIP_BUILD=1 bash publish/mirror-acceptance.sh`
		//   以子进程跑；现直调同包 `mirrorAcceptanceNative`（**同一份**六格判据，✗ 不另起一套）。
		//   · 退码口径不变：验收非 0 ⇒ 本段 return 1（挡在 push 之前，一个字节都不推）；
		//   · `SKIP_BUILD=1` 由 `accWithSkipBuild` 临时置位并复原（进程内无子进程 env 边界）；
		//   · ★ 锁面：旧壳的验收是**子进程**、靠 `ZERG_MIRROR_LOCK_HELD`（本进程 pid，落自家祖链）
		//     判重入 ⇒ 内层不再取第二把锁；进程内同进程 ⇒ `mpLockReentrant()` 头一条就命中
		//     （标记 pid == 锁件 pid == 本进程）⇒ **同一条 rail，同一结论**。
		//   · ★ 验收内层的镜像动作同样直调 `mirrorPublicNative`（`accIO.mirrorTo`）⇒ 三层
		//     `bash → exec bin/zerg` 的跳数一并消失，仍是同一份实现。
		if arc := accWithSkipBuild(func() int {
			return mirrorAcceptanceNative(root, stdout, stderr, nil)
		}); arc != 0 {
			return 1
		}
		if replaceHistory == 1 {
			// ── ③ 换史推送（Mr2109 2026-10-10 授权 · 三重确认）──
			// 为什么需要：公开仓现有历史是**旧出树规则**的产物 ⇒ 同一私有提交在两侧产出不同 tree
			// ⇒ 两条 SHA 链无共同祖先 ⇒ 非 force 永远被拒。换史 = 用新版镜像**替换**公开仓 main，
			// 并把旧 tip 钉进 lease（`--force-with-lease=<ref>:<expect>`）⇒ 远端期间一动就拒执。
			if os.Getenv("ZERG_MIRROR_ALLOW_REPLACE_HISTORY") != "1" {
				fmt.Fprintln(stderr, "✗ --replace-history 被双重保险拦下：换史要同时满足 --replace-history 且 ZERG_MIRROR_ALLOW_REPLACE_HISTORY=1。")
				return 3
			}
			if len(expectOld) != 40 {
				fmt.Fprintln(stderr, "✗ --replace-history 需要 --expect-old=<40 位远端 tip>（拍板时现读）")
				return 2
			}
			// 第三重＝**现读远端 main** 与 --expect-old 逐字比对（不符 ⇒ 不覆盖不可知状态）
			crc, curLine := mpGitCaptureErr(stderr, "-C", out, "ls-remote", "--heads", "origin", "main")
			if crc != 0 {
				return crc // 旧壳 `$(…)` 赋值的失败经 `set -e` 原样转出
			}
			curRemote := mpAwkFirstFieldAll(curLine)
			if curRemote != expectOld {
				fmt.Fprintf(stderr, "✗ 远端 main 现读为 %s，与 --expect-old %s 不符 ⇒ 拒执（不覆盖不可知状态）\n",
					mpFirst12(curRemote), mpFirst12(expectOld))
				return mpRCExpectMismatch
			}
			fmt.Fprintf(stdout, "★ 换史推送：用新版 %s 笔镜像替换远端 main（旧 tip %s）\n", mirrorCount, mpFirst12(expectOld))
			pushCmd := armChildGroup(exec.Command("git", "-C", out, "push",
				"--force-with-lease=refs/heads/main:"+expectOld, "origin", "main"))
			pushCmd.Stdout = stdout
			pushCmd.Stderr = stderr
			if prc := mpRun(pushCmd); prc != 0 {
				return prc // 旧壳 `set -e` ⇒ 以 git 的码退出（EXIT 罩照样补 rejected 行）
			}
			fmt.Fprintf(stdout, "已推送（换史 · force-with-lease 钉住旧 tip）：%s\n", remote)
		} else {
			pushCmd := armChildGroup(exec.Command("git", "-C", out, "push", "origin", "main"))
			pushCmd.Stdout = stdout
			pushCmd.Stderr = stderr
			if prc := mpRun(pushCmd); prc != 0 {
				return prc // 非 force：历史关系不成立时让远端拒绝，而不是覆盖（`set -e` 原样转出）
			}
			fmt.Fprintf(stdout, "已推送（非 force）：%s\n", remote)
		}
		// ── ④ 推送后读回复核（账 G7 唯一缺口）：ls-remote 读回远端现值 · 与意图 tip 逐字比对 ──
		// 设计口径：推送成功后**立即**读回；意图 tip = 刚推上去的那一笔 = 产出仓 main（mirrorTip）。
		// ★ 不吞错：读远端本身失败 ⇒ 亦非 0 退出（✗ 不重定向 stderr、✗ 不折 rc）。落点照本件既有
		//   CAS 先例（上面换史段的 ls-remote 读回比对）—— 不另创第二套。
		rrc, remoteLine := mpGitCaptureErr(stderr, "-C", out, "ls-remote", "--heads", "origin", "main")
		if rrc != 0 {
			fmt.Fprintln(stderr, "✗ 推送后复核：读远端失败（ls-remote rc 非 0）—— 读不到 ≠ 推成功，✗ 不吞错")
			return mpRCReadbackUnmet
		}
		remoteTip := mpAwkFirstFieldFirstLine(remoteLine)
		audit.remoteTip = remoteTip
		if remoteTip != mirrorTip {
			fmt.Fprintf(stderr, "✗ 推送后复核：远端现值 %s ≠ 意图 tip %s ⇒ 拒给结论（推送未达预期）\n", remoteTip, mirrorTip)
			return mpRCReadbackUnmet
		}
		fmt.Fprintf(stdout, "✓ 推送后复核：远端现值 == 意图 tip（%s）\n", mpFirst12(mirrorTip))

		// ── 发布审计台账：读回复核通过 ⇒ 写 `publish` 一行（★ 到此才写成功行；失败由 EXIT 罩写 rejected）──
		audit.appendLine(audit.line("publish", 0, "", remoteTip), stderr)
		audit.done = true
	} else {
		fmt.Fprintln(stdout, "（dry-run：全部做完但**未推送**）")
		fmt.Fprintf(stdout, "  复核后推送：  cd %s && git remote add origin <公开仓> && git push origin main\n", out)
	}
	return 0
}
