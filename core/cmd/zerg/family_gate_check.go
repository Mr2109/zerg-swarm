// family_gate_check.go —— `zerg gate check <判据名>`：**判据转发壳**（v2.5.14 · M1 · 设计-判据 CLI 化-v1.7 §9.1）。
//
// 形状（写死）：**动词做命令、名词做参数** ⇒ `zerg gate check <判据名>`；判据名 ⇒ 件 是**参数**，
// 不是 194 条平级命令（§5.3 硬边「189 件不做 189 条平级命令」）。本壳在 M1 只做「**打得着**」——
// 真搬 Go 是 M2，本壳**不改判据实现**（子件一字不动）。
//
// 三条纪律（逐条对应已拍表 · 设计 §四）：
//
//	① **只转发、不翻译**：子件退多少、壳就退多少（`ee.ExitCode()` 原样转出）——
//	   0/1/2 三码逐字透传；**零分支**改写退码。子件 stdout/stderr/stdin 逐字对接，
//	   故态 A（既有门步口径）与态 B（本壳）的输出**逐字相同**（验收 §9.3「两态对拍」）。
//	② **件自身崩 / 跑不起来**（exec 层：解释器找不到 / 不可执行）⇒ 壳给 `8`（不给结论）
//	   ＋ `detail=judge_broken` ＋ `raw_rc=-1` 留痕。**不抄 Docker 的 125/126/127**（那是它自家约定）；
//	   透传会把「壳故障」冒充成「件判负」✗（拍定第二条）。
//	③ **名字 ⇒ 件** 的解析面**保守且现读可机检**：名单真源 = **现读** `<仓根>/scripts/` 的
//	   **递归**列举 —— ★ M1-19：`*.py` / `*.sh` **全收**（不看可执行位，仓内多为 0644）；
//	   **兼收「无扩展名 ＋ 有可执行位 ＋ shebang 可识别」的无后缀活门**（件名 ＝ 整个文件名）。
//	   ✗ 有扩展名但不是 `.py`/`.sh` 的（`.go`/`.md`/`.json`/`.tsv`/`.yml`…）**一律不收** ——
//	   名录只装**判据件**，不装数据件/说明件。名字 ＝ 文件名去扩展名，§6「判据名 ＝ 件名去扩展名，逐字相同」。
//	   **不手敲名录**、**不模糊匹配**、**不猜**：名字不在册 ⇒ 拒（`2`）并**给可用名列表**；
//	   递归后同名落在不同子目录 ⇒ **显式报歧义并拒**（✗ 不静默挑一个）。
//	   ★ 名录由目录列举派生 ⇒ 新增判据件**自动**在册，**不动命令树**（§5.3）。
//
// 路径边界（承 §6）：解析出的件必须在 `<仓根>/scripts/` 前缀内，越界 ⇒ `2`（壳自身错，
// `judge_resolved=false`），不 exec。
//
// ★ 本刀补落（M1 批B · 四条**合一件**）：
//
//	M1-06 壳级 `--dry-run` / `--json` **全表干跑档**（逐件名字⇒件解析 · ✗ 不 exec · 零副作用）；
//	M1-08 **退码台账**（`scripts/gates/judge-run-ledger.jsonl` · append-only · 六字段
//	      `raw_rc`/`detail`/`sha256`/`head`/`judge_resolved`/`dry_run`）的壳侧**两支写点**；
//	M1-09 **件级互斥锁 ＋ 槽池取槽**（调现成的 `scripts/gates/judge-lock.py` · 单件**不拿**全表锁 ·
//	      锁竞争/超预算/排队过久 ⇒ `detail=judge_locked_by_other` ⇒ 退 `2` 不给结论）；
//	M1-10 **转发开关** `ZERG_GATE_FORWARD`（唯一变量名 · 唯一取值 `off` · 缺省＝不设 ⇒ 转发开；
//	      `off` ⇒ 关掉转发这一路：**不 exec** ⇒ 子件跑不起来 ⇒ 退 `8` ＋ `detail=judge_broken`）。
//	★ 仍不在本刀：`--timeout`（属后续面）。
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// gateCheckRosterDirRel —— 判据件名录的**目录真源**（相对仓根）。只此一处拼它，别处不许再拼。
// ★ 第 1 刀（M1 名录扩面）：由 `scripts/gates` 放宽到 `scripts` 并**递归全子树** —— 子目录件也可达。
// ★ 本刀（M1-19 · 名录后缀放开）：后缀面也**放开** —— 除 `.py` / `.sh` 外，**兼收无后缀活门**
// （判据 =「无扩展名 ＋ 有可执行位 ＋ shebang 可识别」，见 gateCheckShebangInterp）。
const gateCheckRosterDirRel = "scripts"

// gateCheckInterpreter —— 判据件扩展名 ⇒ 解释器（**不是**按位可执行位判 —— 件在仓库里多为 0644）。
// 闭集只有两种：`.py` ⇒ `python3`；`.sh` ⇒ `bash`（设计 §五「库类 21 件 + sh 42 件不特殊化」）。
// ★ M1-19：**无扩展名**件不在这张表里（`Ext("")` 落空）⇒ 解释器按件自身 shebang 现读
// （gateCheckShebangInterp）；本表只管**有扩展名**那一路的映射。
var gateCheckInterpreter = map[string]string{".py": "python3", ".sh": "bash"}

// gateCheckShebangKnown —— 「shebang 可识别」的解释器**闭集**（✗ 不收任意二进制 —— 名录里的
// 无后缀件只有它首行解释器的 basename 落在这张表里才算数）。与 gateCheckInterpreter 的**值**
// 同源（`python3` / `bash`），另多认几个仓内通用的 sh 方言与 `python` 裸名。
var gateCheckShebangKnown = map[string]bool{
	"python3": true, "python": true, "bash": true, "sh": true, "dash": true, "zsh": true,
}

// gateCheckShebangInterp —— 现读**无后缀件**首行 shebang ⇒ 解释器（M1-19 无后缀档的**唯一**判据面）。
//
// 「可识别」是**闭集**判据（✗ 不按位猜、✗ 不认任意二进制）：首行须 `#!` 起头；取首 token
// （或 `/usr/bin/env` 之后的第一个非旗标 token）的 **basename**，须落在 gateCheckShebangKnown 里。
// 返回 `(程序名, 前置参数, ok)`；程序名交给 `exec.Command`（走 PATH 解析）。
// 例：`#!/usr/bin/env python3` ⇒ ("python3", [], true)；`#!/bin/bash` ⇒ ("bash", [], true)；
//
//	`#!/usr/bin/env -S python3 -u` ⇒ ("python3", ["-u"], true)；`#!/bin/sh -e` ⇒ ("sh", ["-e"], true)。
func gateCheckShebangInterp(path string) (string, []string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, false
	}
	defer f.Close()
	br := bufio.NewReader(f)
	line, rerr := br.ReadString('\n')
	if rerr != nil && line == "" {
		return "", nil, false
	}
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "#!") {
		return "", nil, false
	}
	fields := strings.Fields(strings.TrimSpace(line[2:]))
	if len(fields) == 0 {
		return "", nil, false
	}
	prog := filepath.Base(fields[0])
	args := fields[1:]
	// `#!/usr/bin/env [-S] <真解释器> [参数…]` —— env 只是**跳板**，真解释器在旗标之后。
	if prog == "env" {
		i := 0
		for i < len(args) && strings.HasPrefix(args[i], "-") {
			i++
		}
		if i >= len(args) {
			return "", nil, false
		}
		prog = filepath.Base(args[i])
		args = args[i+1:]
	}
	if !gateCheckShebangKnown[prog] {
		return "", nil, false
	}
	return prog, args, true
}

// gateCheckLockToolRel —— M1-09 件级锁 ＋ 槽池取槽的**现成件**（调它，✗ 不另造第二把）。
// 与 `scripts/gates/judge-lock.py` 逐字同源（`<仓根>/scripts/gates/judge-lock.py`）。
var gateCheckLockToolRel = []string{"gates", "judge-lock.py"}

// gateCheckLockToolKey —— 锁件对应的**判据名 / inproc 键**（M2 判据件 Go 化后两器同名）。
// 用于退役态判定：锁件 py 不在盘时，据它现读 gateCheckGoEntry 看 Go 半是否在册
// （见 gateCheckAcquire · G16 · 2026-10-07 退役批）。
const gateCheckLockToolKey = "judge-lock"

// gateCheckLedgerRel —— M1-08 判据运行台账件（append-only）。与读侧
// `scripts/gates/check-judge-ledger.py` 的 `LEDGER_DEFAULT` 逐字同源。
var gateCheckLedgerRel = []string{"gates", "judge-run-ledger.jsonl"}

// gateCheckTrustedRunnerEnv —— ★ 本批（2026-10-08）「受信运行器」旋钮的**唯一**变量名。
//
// 病根：危险档那 4 处闸步（isolate-ro / all / real-gates / pack-release）要从 CLI 正门
// （`zerg gate check <判据名>`）跑，却卡在本壳「缺 `--yes` 一律拒执」（下面 ⑤ 那道盾）——
// 闸运行器是**程序化、受信、无人在场**的调用者，不应被当成「生人/模型直敲」。
//
// 口子（✗ 不放宽 `--yes` 本身）：**只**免「`gate check` 危险档（写工具 / 重编类 / 整套门类）
// 缺 `--yes` ⇒ 拒执」这**一道**盾；给了 `--yes` 的语义一字不动。
//
// ★ 边界（要求 ⓓ）：本旋钮**只**在本件这一处被读、**只**改这一道盾 ——
// `cmdGuarded` / `writeGuardYes` / `repo commit` / 各判据件的判定面**一律不读它** ⇒
// 缺它 / 值不对 ⇒ 与今天**逐字同**（裸跑仍 rc=2，✗ 不得变 0）。本批只开这一枚旋钮。
const gateCheckTrustedRunnerEnv = "ZERG_TRUSTED_RUNNER"

// gateCheckTrustedRunnerValue —— 旋钮**唯一**被认的取值（闭集 · 逐字相等）。
// 缺省 / 空串 / "true" / "0" / 任意别的串 ⇒ 视为**没设**（fail-closed · 与今天逐字同）。
const gateCheckTrustedRunnerValue = "1"

// gateCheckTrustedRunner —— 现读「受信运行器」身份（留痕用）。ok=true ⇒ 本轮免盾生效。
func gateCheckTrustedRunner() (id string, ok bool) {
	if strings.TrimSpace(os.Getenv(gateCheckTrustedRunnerEnv)) != gateCheckTrustedRunnerValue {
		return "", false
	}
	return gateCheckTrustedRunnerEnv + "=" + gateCheckTrustedRunnerValue, true
}

// gateCheckRoster —— 现读名录：`{判据名: 件绝对路径}` ＋ 同名歧义列表（保守面）＋ 读不动的原因。
//
// 判据名 ＝ 文件名去扩展名（§6）；只收 `.py` / `.sh` 两种扩展名。递归全子树后，同名（含**不同子
// 目录**下的同名件，如 `a/x.py` 与 `b/x.sh`）逐条登记在 `dupes` —— 调用方**保守拒**（不许挑一个），
// 因为「挑」就是猜。
func gateCheckRoster(root string) (map[string]string, []string, error) {
	dir := filepath.Join(root, gateCheckRosterDirRel)
	paths := map[string][]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// `__pycache__` 是**编译缓存**、不是判据件 ⇒ 不递归进去（也不许它污染名录）。
			if d.Name() == "__pycache__" {
				return fs.SkipDir
			}
			return nil
		}
		n := d.Name()
		ext := filepath.Ext(n)
		if _, ok := gateCheckInterpreter[ext]; ok {
			name := strings.TrimSuffix(n, ext)
			paths[name] = append(paths[name], p)
			return nil
		}
		// ★ M1-19 · 无后缀档：`Ext(n) == ""` 的件再走一道「可执行位 ＋ shebang 可识别」的判据。
		// ✗ 有扩展名但非 `.py`/`.sh` 的件（`.go`/`.md`/`.json`/…）在这里**直接出局** ——
		// 名录只装判据件，绝不把数据件/说明件收进来。
		if ext != "" {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil || info.Mode().Perm()&0o111 == 0 {
			return nil
		}
		if _, _, ok := gateCheckShebangInterp(p); !ok {
			return nil
		}
		paths[n] = append(paths[n], p)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	byName := map[string]string{}
	dupes := []string{}
	names := make([]string, 0, len(paths))
	for n := range paths {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		ps := paths[name]
		if len(ps) == 1 {
			byName[name] = ps[0]
			continue
		}
		// 同名 ⇒ 名字 ⇒ 件 **不唯一**：显式报歧义（仓根相对路径逐条摆出 · ✗ 不静默挑一个）。
		sort.Strings(ps)
		rel := make([]string, 0, len(ps))
		for _, p := range ps {
			if r, rerr := filepath.Rel(root, p); rerr == nil {
				rel = append(rel, r)
			} else {
				rel = append(rel, p)
			}
		}
		dupes = append(dupes, fmt.Sprintf("%s（%s 同名 %d 件）", name, strings.Join(rel, " · "), len(ps)))
	}
	sort.Strings(dupes)
	return byName, dupes, nil
}

// gateCheckRunInproc —— 进程内跑一件已搬的 Go 判据体。
//
// ★ cwd 差异（**exec 档 `cmd.Dir = root` ⟷ inproc 档默认是壳的 cwd**）：先切到 root 并 defer 恢复。
// 现读普查 107 件的根定位：A 优先读 `ZERG_REPO_ROOT` 3 件 · B 仅 cwd 上溯 24 件 · C `os.Executable` 1 件 ·
// D 认 `--root`/`--repo` 旗标缺省上溯 63 件 · E 共享 `repoRoot()` 5 件 · E 不定根 11 件
// ⇒ 只设 env 仅动 3/107 件，**chdir 是唯一覆盖全 107 件的解**（B/D 的缺省档与 E(none) 全靠 cwd）。
//
// ★ env 补充：`ZERG_REPO_ROOT` 与 `ZERG_REPO` 是**两个不同的名字**（判据件局部族读前者、
// 共享族读后者）⇒ 两个都设，令「显式旋钮优先」的 3 件在 cwd 变动时仍稳定。
//
// ★ 单件调用 ⇒ 无并发；`os.Chdir` 是进程级全局状态，故必须在返回前恢复（defer）。
func gateCheckRunInproc(fn func(stdout, stderr io.Writer, argv []string) int, root string, stdout, stderr io.Writer, argv []string) int {
	wd, werr := os.Getwd()
	if werr == nil {
		if cerr := os.Chdir(root); cerr == nil {
			defer func() {
				// ★ 恢复失败**不许静默**（GAP-20261002-02）：cwd 是进程级全局状态，留在 root
				// 会把污染扩散给后续所有判据。真实场景 wd 必在；wd 已被删（如上游测试没收
				// 回自己的 t.TempDir()）时如实留痕 —— 只留痕，**退码一字不动**。
				if rerr := os.Chdir(wd); rerr != nil {
					fmt.Fprintf(stderr, "%s: ⚠ cwd 回收失败（%v）：进程 cwd 留在 %s\n", progName, rerr, root)
				}
			}()
		}
	}
	oldRoot, hadRoot := os.LookupEnv("ZERG_REPO_ROOT")
	oldRepo, hadRepo := os.LookupEnv("ZERG_REPO")
	_ = os.Setenv("ZERG_REPO_ROOT", root)
	_ = os.Setenv("ZERG_REPO", root)
	defer func() {
		if hadRoot {
			_ = os.Setenv("ZERG_REPO_ROOT", oldRoot)
		} else {
			_ = os.Unsetenv("ZERG_REPO_ROOT")
		}
		if hadRepo {
			_ = os.Setenv("ZERG_REPO", oldRepo)
		} else {
			_ = os.Unsetenv("ZERG_REPO")
		}
	}()
	return fn(stdout, stderr, argv)
}

// gateCheckRosterSorted —— 名录的名字表（给人读的可用名列表；排序稳定）。
func gateCheckRosterSorted(byName map[string]string) []string {
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// gateCheckAvailableLines —— 可用名列表的人读渲染（每行若干名，按显示宽度折行）。
func gateCheckAvailableLines(names []string, perLine int) []string {
	out := []string{}
	for i := 0; i < len(names); i += perLine {
		j := i + perLine
		if j > len(names) {
			j = len(names)
		}
		out = append(out, "  "+strings.Join(names[i:j], "  "))
	}
	return out
}

// gateCheckLedgerRow —— M1-08 台账一行的六字段（与读侧 `check-judge-ledger.py` 的 `REQUIRED`
// 逐字一致）。`detail` 与 `raw_rc` **分列**：`raw_rc` 只装子件原样退码，✗ 不许顶替壳退码。
type gateCheckLedgerRow struct {
	RawRC         int    `json:"raw_rc"`
	Detail        string `json:"detail"`
	SHA256        string `json:"sha256"`
	Head          string `json:"head"`
	JudgeResolved bool   `json:"judge_resolved"`
	DryRun        bool   `json:"dry_run"`
	// ★ 本批「受信运行器」免盾留痕（缺省空 ⇒ `omitempty` 令该键**不出现** ⇒ 行字节与今天逐字同）。
	//   值形态 `trusted-runner:ZERG_TRUSTED_RUNNER=1` —— 谁免的盾；「哪一步」见
	//   stderr 审计行（`本步=zerg gate check <判据名>`）与闸运行器那一步的日志。
	Waiver string `json:"waiver,omitempty"`
}

// gateCheckFileSHA256 —— 判据件内容哈希（读不到 ⇒ `unavailable` · 如实记 · 不猜）。
func gateCheckFileSHA256(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "unavailable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// gateCheckHead —— 现读 HEAD（读 `<仓根>/.git/HEAD` → ref 解引用；取不到 ⇒ `unknown`）。
// **不 exec**（本壳不在解析面起额外进程）、不猜。
func gateCheckHead(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return "unknown"
	}
	s := strings.TrimSpace(string(b))
	if strings.HasPrefix(s, "ref:") {
		ref := strings.TrimSpace(strings.TrimPrefix(s, "ref:"))
		rb, rerr := os.ReadFile(filepath.Join(root, ".git", filepath.FromSlash(ref)))
		if rerr != nil {
			return "unknown"
		}
		return strings.TrimSpace(string(rb))
	}
	return s
}

// gateCheckLedgerAppend —— M1-08：往台账**追加**一行（**append-only**：只 `O_APPEND`，
// ✗ 不改旧行 / ✗ 不删行）。写不进 ⇒ **如实报**（stderr），但**不改**本命令退码
// （退码只由转发口径定 —— 台账是留痕面、不是判定面）。
func gateCheckLedgerAppend(root, judgePath string, rawRC int, detail string, resolved, dryRun bool, waiver string, stderr io.Writer) {
	row := gateCheckLedgerRow{
		RawRC:         rawRC,
		Detail:        detail,
		SHA256:        gateCheckFileSHA256(judgePath),
		Head:          gateCheckHead(root),
		JudgeResolved: resolved,
		DryRun:        dryRun,
		Waiver:        waiver,
	}
	line, err := json.Marshal(row)
	if err != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 台账行序列化失败：%v\n", progName, err)
		return
	}
	led := filepath.Join(root, gateCheckRosterDirRel, filepath.Join(gateCheckLedgerRel...))
	if err := os.MkdirAll(filepath.Dir(led), 0o755); err != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 台账目录建不出（%s）：%v ⇒ 本轮留痕缺（如实报）\n", progName, filepath.Dir(led), err)
		return
	}
	f, err := os.OpenFile(led, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 台账写不进（%s）：%v ⇒ 本轮留痕缺（如实报）\n", progName, led, err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 台账写失败（%s）：%v\n", progName, led, err)
	}
}

// gateCheckAcquire —— M1-09：取**件级互斥锁 ＋ 槽池槽位**（调现成的 `judge-lock.py`）。
// 单件跑**不拿**全表锁（本壳从不打开 `gate-run.lock`）。返回 `(rc, proceed)`：
//   - 取到 ⇒ (0, true) 继续 exec；
//   - 竞争 / 超预算 / 排队过久（取号件退非 0）⇒ 原样转出它的码（现读 = `2` · **不给结论**）；
//   - 取号件不在盘 ⇒ (0, true) 但**如实报**（非静默降级 —— 合成夹具/最小仓口径）。
func gateCheckAcquire(root, name string, stderr io.Writer) (int, bool) {
	tool := filepath.Join(root, gateCheckRosterDirRel, filepath.Join(gateCheckLockToolRel...))
	if _, err := os.Stat(tool); err != nil {
		// ★ 退役态（G16 · 2026-10-07 退役批）：锁件 `scripts/gates/judge-lock.py` 已退役（不在盘）
		//   而它的 **Go 半**（`judge_lock.go` · inproc 键 `judge-lock`）在册 ⇒ **退役态不过问**：
		//   静默跳过取锁（✗ 不告警 / ✗ 不改退码）。否则每跑一门都往 stderr 吐一行警告 ——
		//   把 `zerg gate check <键>` 的 stderr 面弄脏（「两态三面逐字同」取证的前提）。
		//   ★ 负控：锁件不在盘 **且** Go 半也不在册（合成夹具 / 最小仓）⇒ 保留原「如实报」档
		//   （✗ 不静默降级 —— 那一路读数只能靠现读 gateCheckGoEntry 判，测试件可删键做负控）。
		if _, inproc := gateCheckGoEntry[gateCheckLockToolKey]; inproc {
			return exitOK, true
		}
		fmt.Fprintf(stderr, "%s: ⚠ 件级锁件不在盘（%s）⇒ 本轮**不取锁**（如实报 · 非静默降级）\n", progName, tool)
		return exitOK, true
	}
	cmd := armChildGroup(exec.Command("python3", tool, "--name", name, "--slots", "8", "--wait", "0"))
	cmd.Dir = root
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(stderr, "%s: 件级锁/槽池未取得（judge-lock rc=%d）：%s", progName, ee.ExitCode(), errb.String())
			if !strings.HasSuffix(errb.String(), "\n") {
				fmt.Fprintln(stderr)
			}
			// 竞争/超预算 ⇒ 现读自有号（取号件自己定的档 · 本壳**不改写**）。
			return ee.ExitCode(), false
		}
		fmt.Fprintf(stderr, "%s: ⛔ 件级锁件跑不起来：%v ⇒ **不给结论**\n", progName, err)
		return exitBlocked, false
	}
	return exitOK, true
}

// gateCheckFullTableFlags —— M1-06 壳级全表旗标识别：只在**首枚**起连字符、且整段**只由**
// `{--dry-run, --json}` 构成时生效（✗ 不与判据名同给）。ok=false ⇒ 形态不合法（用法错）。
func gateCheckFullTableFlags(tail []string) (dry, js, ok bool) {
	if len(tail) == 0 || !strings.HasPrefix(tail[0], "-") {
		return false, false, false
	}
	for _, a := range tail {
		switch a {
		case "--dry-run":
			dry = true
		case "--json":
			js = true
		default:
			return false, false, false
		}
	}
	return dry, js, true
}

// gateCheckFullTableEntry —— 全表干跑的一格（名字 ⇒ 件）。
type gateCheckFullTableEntry struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	JudgeResolved bool   `json:"judge_resolved"`
}

// gateCheckFullTable —— M1-06：**全表干跑**（逐件名字⇒件解析 · ✗ 不 exec · **零副作用**）。
// `count` = 名录件数（构造上 == 名录）；有解析不到的件 ⇒ rc=`1`（一般失败）；名录读不到/同名歧义 ⇒
// rc=`2`（不给结论）。`--json` ⇒ 机器面 `{count,resolved,dry_run,entries[]}`。
func gateCheckFullTable(root string, stdout, stderr io.Writer, dry, js bool) int {
	byName, dupes, err := gateCheckRoster(root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: ⛔ 判据名录读不到（%s/ 列举失败：%v）⇒ **不给结论**\n",
			progName, gateCheckRosterDirRel, err)
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitBlocked
	}
	if len(dupes) > 0 {
		fmt.Fprintf(stderr, "%s: ⛔ 判据名录里有**同名多件**（名字 ⇒ 件 不唯一）⇒ **不给结论**（禁猜）：\n", progName)
		for _, d := range dupes {
			fmt.Fprintf(stderr, "  · %s\n", d)
		}
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitBlocked
	}
	names := gateCheckRosterSorted(byName)
	prefix := filepath.Join(root, gateCheckRosterDirRel) + string(os.PathSeparator)
	entries := make([]gateCheckFullTableEntry, 0, len(names))
	resolved := 0
	for _, n := range names {
		p := byName[n]
		ok := strings.HasPrefix(filepath.Clean(p)+string(os.PathSeparator), prefix)
		if ok {
			resolved++
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			rel = p
		}
		entries = append(entries, gateCheckFullTableEntry{Name: n, Path: rel, JudgeResolved: ok})
	}
	count := len(names)
	if js {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]any{
			"count":    count,
			"resolved": resolved,
			"dry_run":  dry,
			"entries":  entries,
		})
	} else {
		fmt.Fprintf(stdout, "全表干跑（dry_run=%v）：件 %d · 解析到 %d\n", dry, count, resolved)
		for _, e := range entries {
			mark := "✓"
			if !e.JudgeResolved {
				mark = "✗"
			}
			fmt.Fprintf(stdout, "  %s %s\t%s\n", mark, e.Name, e.Path)
		}
	}
	if resolved != count {
		fmt.Fprintf(stderr, "%s: 有名录件解析不到（%d/%d）⇒ rc=%d\n", progName, count-resolved, count, exitFail)
		return exitFail
	}
	return exitOK
}

// gateCheckClassRE —— 件头 `#zerg-class:` 声明行的正则（**派生副本**口径 · 与
// `scripts/gates/check-writer-guard.py` 的 G0/G1 用**同一串** · 语义逐字同）。`D0` = 非危险档占位。
//
// ★ GAP-20261008-11 后它**不再**是壳侧分类真源（真源上移到 `gate_inproc_table.go` 的
// `gateCheckClassEntry`）—— 只作门禁侧「件头 ↔ 中央表」一致性机检的参照串；保留以免 parity 面断链。
var gateCheckClassRE = regexp.MustCompile(`^#\s*zerg-class:\s*(read|write)\s+(D0|D2|D3)\s*$`)

// gateCheckClassOf —— 【中央登记】按**判据名**取危险档（真源 = `gate_inproc_table.go` 的
// `gateCheckClassEntry` · GAP-20261008-11）。
//
// ★ 为什么不再读件头：件头随**件离盘**（退役）而消失 ⇒ 分类读空 ⇒ 壳侧「默认档要授权」这道闸
//
//	**静默失明**（`--dry-run` 不再出计划件、裸跑直落子件）。中央表与件在不在盘**无关** ⇒ 退役
//	前后判定**逐字同**（退件流程要判的正是这一格）。件头降级为**派生副本**（check-writer-guard.py
//	的 G0/G1/G2 仍按件头机检登记面）。
//
// 缺登记 ⇒ 返回 ("read", "")：**读侧不受闸影响**（普通只读判据件照旧直跑）。会写盘的**漏登记**件
// 由门禁侧机检管（check-writer-guard.py 的 G2「命中写形态必须显式标 write」⇒ rc=1 点名）。
func gateCheckClassOf(name string) (cls, lvl string) {
	if s, ok := gateCheckClassEntry[name]; ok {
		return s.Class, s.Level
	}
	return "read", ""
}

// gateCheckRebuildDirRel —— 「构建/发布入口」目录（相对仓根 · **逐段**给）—— **重编类属性真源之一**。
// ✗ 不是名字名单、✗ 不是点名豁免表：门80 明令壳件里不许出现按名名单 / 写死名件路径；这里判的是
// **目录角色** —— 凡落在这个入口目录下的件，跑它就是在真跑重编/发布。逐段给（✗ 不写真值串）。
var gateCheckRebuildDirRel = []string{"scripts", "build"}

// gateCheckRepoBinTok —— 「repo 根下 bin/ 生产制品」路径 token 的现读形态（可带 ./ 或 $REPO_ROOT/ 前缀）。
// 必须以 bin/ 起 ⇒ ✗ 不误伤 /bin/bash · …/node-a/bin/… · fakebin/… 这类非仓根 bin 路径。
const gateCheckRepoBinTok = `(?:\$\{REPO_ROOT\}/|\$REPO_ROOT/|\./)?bin/[A-Za-z0-9._-]+`

// gateCheckArtifactWriteREs —— 「写 repo bin/ 生产制品」的**写位形态**族（逐条对应
// check-writer-guard.py 的写形态口径，但**落点面是本壳关心的产物目录 bin/**；那件**刻意不判产物面**，
// 所以构建入口这类脚本在它那里看不见）。只在**非注释行**上判（注释/三引号块里的写法不是真动作）。
var gateCheckArtifactWriteREs = []*regexp.Regexp{
	regexp.MustCompile(`>>?\s*"?` + gateCheckRepoBinTok),
	regexp.MustCompile(`(?:^|\s)-o[=\s]\s*"?` + gateCheckRepoBinTok),
	regexp.MustCompile(`(?:^|[;&|]|\s)(?:cp|mv|install|rsync)\b[^;|]*\s"?` + gateCheckRepoBinTok),
	regexp.MustCompile(`\btee\s+(?:-a\s+)?"?` + gateCheckRepoBinTok),
	regexp.MustCompile(`\bof="?` + gateCheckRepoBinTok),
}

// gateCheckWritesRepoArtifact —— 现读件的**非注释行**里，有没有「写 repo bin/ 生产制品」的写位形态。
// 行判口径与 check-writer-guard.py 同规（跳空行/注释行/三引号块）；只读文本、✗ 不 exec、✗ 不写盘。
func gateCheckWritesRepoArtifact(absPath string) bool {
	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	inDoc := false
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		tog := strings.Count(line, "\"\"\"") + strings.Count(line, "'''")
		if inDoc {
			if tog%2 == 1 {
				inDoc = false
			}
			continue
		}
		if tog%2 == 1 {
			inDoc = true
		}
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		for _, re := range gateCheckArtifactWriteREs {
			if re.MatchString(line) {
				return true
			}
		}
	}
	return false
}

// gateCheckRebuildClass —— 现读该件是否属「重编类」（会真跑重编 / 覆盖 bin/ 生产制品）。**属性判 · ✗ 不靠名**：
//
//	① 目录角色：件落在构建/发布入口目录（gateCheckRebuildDirRel）下；
//	② 行为属性：非注释行里现读到「写 repo bin/ 生产制品」的写位形态（gateCheckWritesRepoArtifact）。
//
// 命中任一 ⇒ 跑它会真跑重编 / 覆盖 bin/ 里的生产制品 ⇒ 本壳据此给它与写工具同档的确认盾（缺 --yes ⇒ 拒执）。
// ✗ 不设点名豁免表（门80 明令）、✗ 不写死任何一个件名。
func gateCheckRebuildClass(full, root string) (bool, string) {
	entryDir := filepath.Join(root, filepath.Join(gateCheckRebuildDirRel...))
	if rel, err := filepath.Rel(entryDir, full); err == nil && rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return true, "构建/发布入口目录（" + filepath.ToSlash(filepath.Join(gateCheckRebuildDirRel...)) + "）"
	}
	if gateCheckWritesRepoArtifact(full) {
		return true, "非注释行里写 repo bin/ 生产制品（现读写位形态）"
	}
	return false, ""
}

// gateCheckSuiteRosterREs —— 「gate 名册目录」的**现读形态**族（**跑整套门类**属性真源之一）。逐条给，
// ✗ 不写死任何一个件名：`${GATEDIR}` 是仓内名册（gate 层）目录的惯用 shell 变量名，`gates/*` 是直接对
// 名册目录取通配的形态。名册目录 = 本壳可执行件所在的 gate 层（与名录目录派生同源，**不另立名单**）。
var gateCheckSuiteRosterREs = []*regexp.Regexp{
	regexp.MustCompile(`\$\{?GATEDIR\}?`),
	regexp.MustCompile(`gates/\*`),
}

// gateCheckSuiteExecRE —— 「命令位上起一个 shell / 解释器」的现读形态（行首、空白、`;`/`&`/`|`/`(` 之后）。
// ✗ 不匹配「引号里的字面说明」（如文档/夹具串里的 `bash …`）—— 引号不在这个起首集合里 ⇒ 字面说明天然不算真调用。
var gateCheckSuiteExecRE = regexp.MustCompile(`(?:^|[\s;&|(])(?:bash|sh|zsh|exec|dash|perl|python3)\s`)

// gateCheckSuiteVarBodyRE —— 「执行体由 shell 变量解析」的形态（`"${X}"`）：跑的不是写死的件名，而是
// 现读解析出来的东西 —— 这正是「真跑整套门」与「只读读步骤表」的分界（名册件是目录派生 ⇒ 只能由变量接住）。
var gateCheckSuiteVarBodyRE = regexp.MustCompile(`"\$\{[A-Za-z_][A-Za-z0-9_]*\}[^"]*"`)

// gateCheckSuiteSelfRE —— 「自指」体（`BASH_SOURCE` / `$0`）：件起自己（如自检负控）≠ 跑整套门 ⇒ 排除。
var gateCheckSuiteSelfRE = regexp.MustCompile(`BASH_SOURCE|\$0\b`)

// gateCheckSuiteReadRE —— 「只读档」调用（`--list` / `--self-test` / `--selftest` / `--help`）：同行带这些 ⇒ 不判真跑。
var gateCheckSuiteReadRE = regexp.MustCompile(`--list\b|--self-test\b|--selftest\b|--help\b`)

// gateCheckSuiteStepCmdRE —— 「以 shell 起 `-c "<串>"` 真跑一条命令」的形态（整套门**执行器**的特征）。
var gateCheckSuiteStepCmdRE = regexp.MustCompile(`(?:^|[\s;&|(])(?:bash|sh|zsh|dash)\s+-c\b`)

// gateCheckStripQuoted —— 去掉行内**被引号包住的区间**（单/双引号 · 认反斜杠转义 · 逐字节扫），只留
// 「命令骨架」文本。判「真起 shell 跑一条命令」时用：`bash|sh|zsh|dash` 这个**调用关键字**与它的 `-c`
// 必然落在引号**外**（引号里的是它要跑的**参数**）；⇒ 凡 `bash -c` 出现在引号**内**，那它只是**数据 /
// 人读说明串**（典型：`print("… sh -c '<目标>' …")` 这类文案），不是真调用 ⇒ 不算。
func gateCheckStripQuoted(line string) string {
	var b strings.Builder
	var q byte // 0 = 不在引号内；否则 = 当前引号字符
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '\\' && i+1 < len(line) {
			if q == 0 {
				b.WriteByte(c)
			}
			i++
			if q == 0 {
				b.WriteByte(line[i])
			}
			continue
		}
		if q == 0 {
			if c == '"' || c == '\'' {
				q = c
				continue
			}
			b.WriteByte(c)
			continue
		}
		if c == q {
			q = 0
		}
	}
	return b.String()
}

// gateCheckSuiteClass —— 现读该件是否属「**跑整套门类**」（会真跑成套的 gate 步）。**属性判 · ✗ 不靠名**：
//
//	① **名册执行**：该件现读名册目录（gateCheckSuiteRosterREs），且**命令位**起 shell/解释器去跑一个由
//	   变量解析的执行体（gateCheckSuiteVarBodyRE），且非自指、非同行只读档 —— 即「遍历名册 · 逐个真跑」；
//	② **步命令执行**：以 `bash -c "<串>"` 真跑一条命令（gateCheckSuiteStepCmdRE · **只在引号外**判：先经
//	   gateCheckStripQuoted 剥掉引号区间 ⇒ 人读说明串里的 `sh -c` 不算）—— 整套门的**执行器**本体。
//
// 与「只读读步骤表」判据的分界：后者**把步骤表当文本读**（open + 解析 add_step），从不起命令位的解释器
// 去跑名册件 ⇒ 两条属性都不命中（**误伤面 = 0**）。命中任一 ⇒ 跑它会真跑整套门 ⇒ 本壳据此给它与写工具
// 同档的确认盾（缺 --yes ⇒ 拒执）。✗ 不设点名豁免表、✗ 不写死任何一个件名（门80 明令）。
func gateCheckSuiteClass(absPath string) (bool, string) {
	f, err := os.Open(absPath)
	if err != nil {
		return false, ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	inDoc := false
	rosterSig := false
	execVar := false
	stepCmd := false
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		tog := strings.Count(line, "\"\"\"") + strings.Count(line, "'''")
		if inDoc {
			if tog%2 == 1 {
				inDoc = false
			}
			continue
		}
		if tog%2 == 1 {
			inDoc = true
		}
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		readMode := gateCheckSuiteReadRE.MatchString(line)
		if !stepCmd && !readMode && gateCheckSuiteStepCmdRE.MatchString(gateCheckStripQuoted(line)) {
			stepCmd = true
		}
		if !rosterSig {
			for _, re := range gateCheckSuiteRosterREs {
				if re.MatchString(line) {
					rosterSig = true
					break
				}
			}
		}
		if !execVar && !readMode && gateCheckSuiteExecRE.MatchString(line) &&
			gateCheckSuiteVarBodyRE.MatchString(line) && !gateCheckSuiteSelfRE.MatchString(line) {
			execVar = true
		}
	}
	if rosterSig && execVar {
		return true, "现读名册目录并命令位起解释器去跑一个由变量解析的执行体（遍历名册 · 逐个真跑）"
	}
	if stepCmd {
		return true, "以 shell 起 -c 真跑步命令串（整套门执行器本体）"
	}
	return false, ""
}

// gateCheckWritePlan —— 危险档件（写工具 / 重编类）--dry-run 的计划件（零副作用 · 只打印 · ✗ 不 exec）。
func gateCheckWritePlan(name, full, lvl, kind, why string, rest []string) string {
	need := "--yes"
	if lvl == dangerD3 {
		need = fmt.Sprintf("--confirm=<%s> 与 --yes 同时到", name)
	}
	var b strings.Builder
	b.WriteString("计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）\n")
	fmt.Fprintf(&b, "  动作     : %s gate check %s\n", progName, name)
	fmt.Fprintf(&b, "  危险档   : %s（%s）\n", lvl, dangerLevelWord(lvl))
	fmt.Fprintf(&b, "  判据件   : %s\n", name) // 用**判据名**（✗ 不用件基名）⇒ 件离盘后逐字同
	fmt.Fprintf(&b, "  执行要   : %s\n", need)
	fmt.Fprintf(&b, "  子件参数 : %s\n", orDash(strings.Join(rest, " ")))
	if kind == "重编类" {
		fmt.Fprintf(&b, "  它会动   : 该**重编类**件会**真跑重编 / 覆盖 bin/ 里的生产制品**（属性判：%s）\n", why)
		fmt.Fprintf(&b, "  来源     : 属性判（GAP-20261005-20）—— 构建/发布入口 或 非注释行写 repo bin/ · ✗ 非点名\n")
	} else if kind == "整套门类" {
		fmt.Fprintf(&b, "  它会动   : 该**整套门类**件会**真跑整套门 / 步**（遍历名册逐个真跑 或 shell 起 -c 跑步命令）（属性判：%s）\n", why)
		fmt.Fprintf(&b, "  来源     : 属性判（GAP-20261005-20）—— 名册执行 或 步命令执行 · ✗ 非点名\n")
	} else {
		fmt.Fprintf(&b, "  它会动   : 该**写工具**会真改盘（换件 / 落临时副本 / 落锁）—— 只读件不受影响\n")
		fmt.Fprintf(&b, "  来源     : 中央分类登记 gateCheckClassEntry（`gate_inproc_table.go` · 判据名 ⇒ 危险档 · GAP-20261008-11）\n")
	}
	fmt.Fprintf(&b, "  留痕     : 一行一事件 · 追加只写 · **写失败即拒**（§九 M3 C5）\n")
	return b.String()
}

// gateCheckStripShellFlags —— 过闸后**壳级确认旗标不落给子件**（`--yes` / `--dry-run` /
// `--confirm*` 从 rest 里剔掉：它们在壳侧已消费，透传给写工具只会混淆其参数面）。
func gateCheckStripShellFlags(rest []string) []string {
	out := make([]string, 0, len(rest))
	skipNext := false
	for _, a := range rest {
		if skipNext {
			skipNext = false
			continue
		}
		switch {
		case a == "--yes" || a == "--dry-run":
			continue
		case a == "--confirm":
			skipNext = true // 值形态：连同它的值一起剔
			continue
		case strings.HasPrefix(a, "--confirm="):
			continue
		}
		out = append(out, a)
	}
	return out
}

// gateCheck —— `zerg gate check <判据名> [子件参数…]` 的转发壳。
//
// `tail` = 用户敲在 `zerg gate check` 之后的**一切**（首枚 = 判据名，其余**逐字**交给子件）；
// 例外：首枚起连字符且整段只由壳级旗标构成 ⇒ 走**全表干跑档**（M1-06）。
func gateCheck(inv *invocation, stdout, stderr io.Writer, root string) int {
	tail := []string{}
	if len(inv.orig) > 2 {
		tail = inv.orig[2:]
	}

	// M1-06 · 壳级全表档：`--dry-run` / `--json`（只认这两位 · 只认首枚段 · ✗ 不与判据名同给）。
	if len(tail) > 0 && strings.HasPrefix(tail[0], "-") {
		dry, js, ok := gateCheckFullTableFlags(tail)
		if !ok {
			fmt.Fprintf(stderr, "%s: `gate check` 首枚位置参数 = 判据名；壳级旗标只认 --dry-run/--json（收到 %q 起）\n", progName, tail[0])
			fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
			return exitUsage
		}
		return gateCheckFullTable(root, stdout, stderr, dry, js)
	}

	if len(tail) == 0 {
		fmt.Fprintf(stderr, "%s: `gate check` 要给判据名（例：zerg gate check check-affected-map）\n", progName)
		fmt.Fprintf(stderr, "%s: 名字 ⇒ 件 的现读名录 = %s/（判据名 ＝ 件名去扩展名）\n", progName, gateCheckRosterDirRel)
		fmt.Fprintf(stderr, "%s: 或给壳级全表档 `--dry-run` / `--json`（全表干跑 · ✗ 不 exec）\n", progName)
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitUsage
	}
	name := tail[0]
	if strings.HasPrefix(name, "-") {
		// 首枚是旗标（缺判据名）：与「无参数」同档 —— 用法错。
		fmt.Fprintf(stderr, "%s: `gate check` 首枚位置参数 = 判据名，不给旗标（收到 %q）\n", progName, name)
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitUsage
	}
	rest := tail[1:]

	// M1-10 · 转发开关（**唯一变量名** `ZERG_GATE_FORWARD` · **唯一取值** `off` · 缺省＝不设 ⇒ 转发开）。
	// 语义**只收紧**：`off` ⇒ 关掉转发这一路（**不 exec**）⇒ 子件跑不起来 ⇒ 退 `8`（不给结论）。
	if strings.TrimSpace(os.Getenv("ZERG_GATE_FORWARD")) == "off" {
		fmt.Fprintf(stderr, "%s: 转发已关（ZERG_GATE_FORWARD=off）⇒ **不 exec** · 子件跑不起来 · detail=judge_broken · raw_rc=-1 · judge_resolved=true\n", progName)
		return exitBlocked
	}

	byName, dupes, err := gateCheckRoster(root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: ⛔ 判据名录读不到（%s/ 列举失败：%v）⇒ **不给结论**\n",
			progName, gateCheckRosterDirRel, err)
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitBlocked
	}
	if len(dupes) > 0 {
		fmt.Fprintf(stderr, "%s: ⛔ 判据名录里有**同名多件**（名字 ⇒ 件 不唯一）⇒ **不给结论**（禁猜）：\n", progName)
		for _, d := range dupes {
			fmt.Fprintf(stderr, "  · %s\n", d)
		}
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitBlocked
	}

	full, ok := byName[name]
	// ★ 退役最后一道前置：**名解析并入 inproc 键** —— 键 ∈ inproc 表（gateCheckGoEntry）
	//   ⇒ 视为在册（✗ 不再要求 py 件在盘）。
	//   病根：名录真源 = 现读 `scripts/` 递归列举（gateCheckRoster）⇒ inproc 表只决定
	//         「进程内 vs exec」、**不参与名解析** ⇒ 某键的 py 件退役（已删）后，判据体虽仍在
	//         Go 侧（gateCheckGoEntry）跑得动，却被判「不在册」rc=2（门105 步即
	//         `zerg gate check <键>` ⇒ 删 py 即 BLOCKED）。
	//   治法：键在表里 ⇒ 合成它的件路径、判定为**已解析**，随后照原路走下面的 inproc 分派。
	//         合成的路径只喂下游三处 —— ① 路径边界（落 `scripts/` 前缀内 ⇒ 必过）② 类判
	//         （读不到件 ⇒ 三判全取安全默认：只读档 / 非重编 / 非整套门）③ 台账（sha256 记
	//         `unavailable`，如实记）。
	//   ✗ 不改「py 件在盘」的任何行为：那一路 byName 命中、full 用真件路径、逐字同旧。
	if !ok {
		_, inproc := gateCheckGoEntry[name]
		// ★ A 方案（本批）· 名解析并入**中央分类表键**（另一半，改处3）：重编/写档类件（登在
		//   gate_inproc_table.go 的 gateCheckClassEntry）退役后 py 件离盘，靠此路仍可解析 ⇒ 它的
		//   D2 确认盾不随件消失（分类真源已上移到中央表；合成路径只喂下游路径边界/台账，✗ 不参与类判）。
		_, inclass := gateCheckClassEntry[name]
		if inproc || inclass {
			full = filepath.Join(root, gateCheckRosterDirRel, "gates", name+".py")
			ok = true
		}
	}
	if !ok {
		// **保守拒**：名字不在册 ⇒ 拒并给可用名列表（✗ 不猜 · 不模糊匹配）。
		fmt.Fprintf(stderr, "%s: 判据名 %q **不在册**（名字 ⇒ 件 只认逐字同名；不模糊匹配）\n", progName, name)
		names := gateCheckRosterSorted(byName)
		fmt.Fprintf(stderr, "%s: 可用名 %d 条（现读 %s/ 递归列举）：\n", progName, len(names), gateCheckRosterDirRel)
		for _, ln := range gateCheckAvailableLines(names, 4) {
			fmt.Fprintln(stderr, ln)
		}
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitUsage
	}

	// 路径边界（承 §6）：必须在 `<仓根>/scripts/` 前缀内 —— 越界 ⇒ 壳自身错，不 exec。
	prefix := filepath.Join(root, "scripts") + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(full)+string(os.PathSeparator), prefix) {
		fmt.Fprintf(stderr, "%s: ⛔ 解析出的件越出 <仓根>/scripts/ 前缀（%s）⇒ **不给结论**（不 exec）\n",
			progName, full)
		fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
		return exitUsage
	}

	// ★ M1 批B · **写工具危险档闸**（壳侧 exec 前 · **唯一写盘点之前**）：
	//   名录收写工具（目录派生 ⇒ 硬排除＝造第二真源），但**壳侧强制危险档 ＋ 非交互拒执**。
	//   真源 = 中央分类登记 gateCheckClassEntry（`gate_inproc_table.go` · 按判据名 · GAP-20261008-11）——
	//   ★ 主人裁定「按 ② 落」= 只认「缺 `--yes`」才退 2（`--yes` 本身就是命令行上的确认盾）——
	//   ✗ 不取「无 TTY 一律 2」（本壳**从不提问**，但 `--yes` 到了就放行）。
	//   退码**零新占号**：全部落在既有「2 = 用法错」这一档（见 guard.go 的三态与 family_gate_matrix.go 先例）。
	// ★ 本刀（GAP-20261005-20）：**重编类**（会真跑重编 / 覆盖 bin/ 生产制品）**并入同一道确认盾** ——
	//   名录按目录派生 ⇒ 里面混着构建/发布入口脚本与「非注释行写 repo bin/」的件，它们在名录里与只读判据
	//   长得一样、此前无护栏 ⇒ 缺 `--yes` 一律拒执并指路。**属性判 · ✗ 不靠名**（门80 明令壳件里不许出现
	//   按名名单 / 写死名件路径）—— 属性真源见 gateCheckRebuildClass。
	// ★ 本刀再半（GAP-20261005-20 另一半）：**整套门类**（会真跑整套门 / 步：遍历名册逐个真跑 或 shell 起 -c
	//   跑步命令）**同样并入这道盾** —— 属性真源见 gateCheckSuiteClass（属性判 · ✗ 不靠名）。
	// ★ 本批「受信运行器」免盾的留痕载体（缺省空 ⇒ 台账行不含该字段 ⇒ 与今天逐字同）。
	waiver := ""
	cls, lvl := gateCheckClassOf(name)
	rebuild, rebuildWhy := gateCheckRebuildClass(full, root)
	suite, suiteWhy := gateCheckSuiteClass(full)
	// ★ A 方案（本批）· 合成式优先级（改处2）：**已登中央表以表为准**（登 read ⇒ 显式降级，压过甲目录判/乙行为判）/
	//   **未登走「甲‖乙」属性判兜底**（= 现状逐字同）。rebuild/suite 仍算（仅供下方 kind 显示分支用）。
	_, registered := gateCheckClassEntry[name]
	shield := rebuild || suite
	if registered {
		shield = cls == "write"
	}
	if shield {
		// 无件头声明级的重编类 / 整套门类 ⇒ 取最轻的确认盾档 D2（`--yes` 即可 · ✗ 不是不可逆档）。
		if (rebuild || suite) && lvl == "" {
			lvl = dangerD2
		}
		kind, why := "写工具", ""
		switch {
		case rebuild:
			kind, why = "重编类", rebuildWhy
		case suite:
			kind, why = "整套门类", suiteWhy
		}
		// ① `--dry-run` 与 `--yes` 同给 ⇒ 2（guard.go:295-306 的**唯一**共享判定 · 逐字同一口径）。
		if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
			return rc
		}
		// ② `--dry-run`：只出计划件、**不 exec**（本批唯一返回 0 的那一态 · 照 guard.go:58-63）。
		if inv.dryRun {
			fmt.Fprint(stdout, gateCheckWritePlan(name, full, lvl, kind, why, rest))
			fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
			return exitOK
		}
		// ③ D3：`--confirm` 必须给（照 guard.go:68-76）。
		if lvl == dangerD3 && !inv.confirmGiven {
			msg := fmt.Sprintf("`%s` 是 %s 档（不可逆）——**缺确认 ⇒ 不执行**", name, lvl)
			inv.setErr("usage", "confirm_required", msg)
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			fmt.Fprintf(stderr, "要执行得给：--confirm=<%s> --yes\n", name)
			fmt.Fprintf(stderr, "先看计划件：%s gate check %s --dry-run\n", progName, name)
			fmt.Fprintf(stderr, "error.kind=usage · detail=confirm_required · retryable=false · remedy=fix_usage\n")
			return exitUsage
		}
		// ④ `--confirm` 给了就校验值（D2/D3 同口径 · 值必须与目标「判据名」逐字相同 · guard.go:77-83）。
		if inv.confirmGiven && inv.confirm != name {
			msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，目标是 %q）⇒ 不执行", inv.confirm, name)
			inv.setErr("usage", "confirm_mismatch", msg)
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			fmt.Fprintf(stderr, "`--confirm` 的值必须与目标**逐字相同**（§4.1 K7）\n")
			fmt.Fprintf(stderr, "error.kind=usage · detail=confirm_mismatch · retryable=false · remedy=fix_usage\n")
			return exitUsage
		}
		// ⑤ 缺 `--yes`（含**一切非交互**档：脚本 / 管道 / agent —— 本壳从不提问）⇒ 2（fail-closed）。
		// ★ 本批：**受信运行器**（闸运行器 · `ZERG_TRUSTED_RUNNER=1`）从 CLI 正门跑危险档判据件时，
		//   免**这一道** `--yes` 盾（✗ 不放宽 `--yes` 本身）。身份真源 = gateCheckTrustedRunner()。
		trustedID, trusted := gateCheckTrustedRunner()
		if !inv.yes && !trusted {
			msg := fmt.Sprintf("`%s` 是**%s**（%s 档）—— **缺 --yes ⇒ 不执行**（非交互一律 fail-closed：从不提问）", name, kind, lvl)
			inv.setErr("usage", "yes_required", msg)
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			if why != "" {
				acts := "**真跑重编 / 覆盖 bin/ 里的生产制品**"
				if kind == "整套门类" {
					acts = "**真跑整套门 / 步**"
				}
				fmt.Fprintf(stderr, "%s: 属性判据 = %s ⇒ 它会%s；要真跑请走正门，✗ 别拿 `gate check` 当「探判据在不在」的工具（名录里它与只读判据同形）。\n", progName, why, acts)
			}
			fmt.Fprintf(stderr, "先看计划件：%s gate check %s --dry-run\n", progName, name)
			fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
			return exitUsage
		}
		// ★ 本批：受信运行器**免了这一道盾** ⇒ ⓑ 留痕（谁免的盾 · 哪一步 · 免的是哪一档）。
		//   · 一行审计进 stderr（闸运行器那一步的回执/日志会收它）；
		//   · 同一枚身份写进本步台账行（waiver 字段）——「哪一步」由 stderr 行的判据名 +
		//     台账的 sha256/head 共同指认（台账六字段形状不变）。
		if !inv.yes && trusted {
			waiver = "trusted-runner:" + trustedID
			fmt.Fprintf(stderr, "%s: ★ 免盾：受信运行器（%s）⇒ 免「%s（%s 档）」的 --yes 确认盾 · 本步=%s gate check %s\n",
				progName, trustedID, kind, lvl, progName, name)
			fmt.Fprintf(stderr, "%s: ★ 留痕 waiver=%s（写进本步台账行）· --yes 本身未放宽（生人/模型直敲仍拒执）\n",
				progName, waiver)
		}
		// ⑥ 过闸：**壳级确认旗标不落给子件**（剔掉 --yes/--dry-run/--confirm*）。
		rest = gateCheckStripShellFlags(rest)
	}

	// M1-09 · 取件级锁 ＋ 槽池槽位（调现成的 judge-lock.py · 单件不拿全表锁）。
	if rc, proceed := gateCheckAcquire(root, name, stderr); !proceed {
		return rc
	}

	// ★ M2 接线：命中已搬 Go 体 ⇒ **进程内**调用（跳过解释器解析 / shebang / argv 构造）。
	// 排在锁（上面那一段）之后 ⇒ 锁天然覆盖；排在本行之后原有的解释器解析之前
	// ⇒「无后缀 shebang 不可识别退 8」那一档自然只留给 exec 路径（命中 Go 体走不到那里）。
	if fn, hit := gateCheckGoEntry[name]; hit {
		rc := gateCheckRunInproc(fn, root, stdout, stderr, rest)
		// 与 exec 档同规：登记在跑的这一次（append-only；写不出只报 stderr、**不改退码**）。
		// ★ 与 exec 档逐支对齐（真源 family_gate_check.go 的成功支 :818 / 失败支 :806）：
		//   rc==0 ⇒ detail="ok"（exec 档即「cmd.Run() 无 err」那一支）；rc!=0 ⇒ detail="judge_rc"。
		//   raw_rc 两档同口径 = 本件原样退码（exec 档传 ee.ExitCode()）。
		detail := "judge_rc"
		if rc == 0 {
			detail = "ok"
		}
		gateCheckLedgerAppend(root, full, rc, detail, true, false, waiver, stderr)
		return rc
	}

	interp := gateCheckInterpreter[filepath.Ext(full)]
	// ★ M1-19 · 无后缀活门：解释器按件自身 shebang 现读（名录构造上已保证可识别；
	// 真读不到 ⇒ 件跑不起来 ⇒ **不给结论**，✗ 不硬凑一个解释器去 exec）。
	pre := []string{}
	if interp == "" {
		p, a, ok := gateCheckShebangInterp(full)
		if !ok {
			fmt.Fprintf(stderr, "%s: ⛔ 无后缀判据件 shebang 不可识别（%s）⇒ **不给结论**（不 exec）\n", progName, full)
			fmt.Fprintf(stderr, "%s: judge_resolved=false\n", progName)
			return exitBlocked
		}
		interp, pre = p, a
	}
	argv := append(append([]string{}, pre...), full)
	argv = append(argv, rest...)
	cmd := armChildGroup(exec.Command(interp, argv...))
	// 与 `cmdGate` 的其余转发同规：登记在跑的这一步（人打断时 `--cancel-on-interrupt` 有东西可取消）。
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：子件的码原样返回（连异常码也不改写）。
			// M1-08 · 失败支也落一行台账（`raw_rc` = 子件原样退码 · 与本壳退码**分列**）。
			gateCheckLedgerAppend(root, full, ee.ExitCode(), "judge_rc", true, false, waiver, stderr)
			return ee.ExitCode()
		}
		// exec 层跑不起来（解释器找不到 / 件不可执行）：**不给结论**（拍定第二条）。
		// `raw_rc=-1` = **没有进程退码可言**（不是把故障写成 0 —— 硬边「禁用 raw_rc 顶替退码」）；留痕。
		gateCheckLedgerAppend(root, full, -1, "judge_broken", true, false, waiver, stderr)
		fmt.Fprintf(stderr, "%s: ⛔ 判据件跑不起来（exec 层 · %s）：%v\n", progName, filepath.Base(full), err)
		fmt.Fprintf(stderr, "%s: detail=judge_broken · raw_rc=-1 · judge_resolved=true\n", progName)
		fmt.Fprintf(stderr, "%s: ⇒ 不给结论（%d）：壳故障与「件判负」不许同形\n", progName, exitBlocked)
		return exitBlocked
	}
	// M1-08 · 成功支落一行台账。
	gateCheckLedgerAppend(root, full, 0, "ok", true, false, waiver, stderr)
	return exitOK
}
