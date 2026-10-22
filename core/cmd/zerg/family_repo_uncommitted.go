// family_repo_uncommitted.go —— 「未提交件」的**只读分类面**（缺口 `GAP-20260926-78`）。
//
// 病灶（缺口正文逐字）：「无『未提交件按 mtime/来源分类』的机器读面：`git diff --stat` 对 `??` 件
// 恒空且只退 0 空输出 ⇒ 无法一眼分清有意改动/构建副产物/来源不明」。手搓现状：本笔取证 10 件
// 非本晚改动**只能逐件人读 mtime 与回执**，制表靠人工。
//
// 现读缺口（2026-10-23 实测 · 仓根 73 件未提交）：
//
//	① `git diff --stat` ⇒ 退 0、45 行，**29 件未跟踪件一件都不在里面**（`??` 件恒不列）⇒ 按它报
//	   「工作树有几件未提交」必然少数，且**空输出也退 0**（空 ≠ 干净）。
//	② `zerg repo status --json` ⇒ 有 `path/status/untracked/mtime/sha256`，但**没有**：kind（有意改
//	   / 副产物）、mtime 分桶、**来源**（谁的）；且未跟踪面走 `-u normal` ⇒ 目录被折成一行
//	   （本仓读数 17 行 vs `-uall` 的 29 件）⇒ 逐件对不上。
//	③ `zerg ls-face --others` ⇒ 只列路径，不带 kind/mtime/来源（缺口 2026-10-14 那条口径注已点名
//	   「属拿 ls 面凑，不许用」）。
//
// 本面的口径（**四条，逐条可机检**）：
//
//	① **真源 = `git status --porcelain=v1 -z --untracked-files=all`**（`--no-optional-locks`）：
//	   已跟踪改动与未跟踪件**同一张表**出，逐件一行；不另立第二套「哪些件没提交」的口径。
//	   未跟踪目录**展开到件**（`-uall`）—— 目录一行会丢掉 mtime 与逐件身份。
//	② **mtime 分桶**：`<1h` / `1-6h` / `6-24h` / `1-7d` / `>7d` / `未知`（边界写死在此，出口逐字印；
//	   未知 = 该件 lstat 读不到，如已删除件 —— **不拿 0 顶替**）。
//	③ **来源 = 审计账对窗口**（唯一机器可读的「谁改的」＝ `<状态目录>/edit_audit.jsonl` 该件**末条**
//	   `edit` 行）—— 缺省读**两处账**（本 lane 账 + 全局账 `~/.zerg/state/edit_audit.jsonl`，同件取
//	   `at` 最大者；`--audit <件>` 可再加、可重复）：每枚 lane 把账落在自己的 `<lane>/state/` 下
//	   ⇒ 只读一枚账，别的 lane 的改动会全被误判成「来源不明」（现场实测：lane 账 0 行 / 全局账 12530 行）。
//
//	   ★ 判归属的**主语**是「产出当前字节的那条账行」—— 拿现内容 `sha256` 对账里的 `after_sha256`
//	   （与 `audit cover` 同一口径），**不是**「时刻最后的那条」：改回去/再改一次都会让末条换人。
//	   命中的那条：时刻 ≥ 窗口起点且匹配 `--me` ⇒ `本笔`；≥ 起点不匹配 ⇒ `他路`（证据列带 proposal/by，
//	   谁的一眼可读）；< 起点 ⇒ `存量`。**账里有该件、但没有一条对得上现内容** ⇒ `来源不明`
//	   （账解释不了这堆字节：绕正门的直写 / 别的 lane 自己的账 / 生成器重写）。**一条账行都没有** ⇒
//	   mtime 在窗口内是 `来源不明`、窗口外是 `存量`。
//	   窗口起点 = `--since`（缺省 `24h`，即 `now-24h`；给 RFC3339 也可）。**不给 `--me` ⇒ `本笔` 档恒 0**
//	   （不猜「我」是谁）—— 出口明写这一句，不用假值占位。
//	④ **零写操作**：只跑 `git status`（且 `--no-optional-locks`，连 `.git/index` 的 stat 刷新都不做）、
//	   只 `lstat`、只读审计件；写旗标一律拒（`rc=2`，与同族 `repo status` / `repo diffstat` 同守）。
//
// 退码：`0` 读到（**无论几件** —— 「有未提交」是状态不是错）/ `2` 用法错（含写旗标）/ `8` 读不到
// （仓根解析不到、不是 git 工作树、`git status` 失败）。**没有 `1`**：本面不给「脏/绿」判词，只给读数。
//
// ★ 本件**未登记**进 `main.go` 命令树（另一路正在改 `main.go`）—— 注册行原文见本件末尾注释；
// 未登记期间 `--json` 的包封用 `repoUncommittedSpec()`（与将来登记行同一份声明），登记后 `find()`
// 命中，二者不产生第二套字段表。
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// repoUncommittedFields —— `--json` 字段表（**登记行与本件共用的唯一一份**）。
//
// 为什么 `status` 与 `kind` 都留：`status` = porcelain 的两列**原样**（对得上 `git status` 的人眼面），
// `kind` = 归一后的中文类目（改 / 增 / 删 / 改名 / 未跟踪 / 冲突，副产物再带 `·副产物` 后缀）。
// 二者不是同一份口径的两个名字：前者是 git 的原文，后者是本面的归类，改一边不该动另一边。
var repoUncommittedFields = []string{
	"path",     // 仓根相对路径（porcelain 原样）
	"status",   // porcelain 两列原样（` M` / `??` / `R ` …）
	"kind",     // 归一类别（改 / 增 / 删 / 改名 / 未跟踪 / 冲突 [·副产物]）
	"artifact", // 副产物判词（是/否 · 路径规则见 rucArtifactPatterns）
	"mtime",    // 该件 mtime（RFC3339 秒级 · 读不到 ⇒ `（读不到）`）
	"age_min",  // 距窗口起跑时刻的分钟数（可负 = mtime 在未来 · 读不到 ⇒ 空）
	"bucket",   // mtime 分桶（<1h / 1-6h / 6-24h / 1-7d / >7d / 未知）
	"source",   // 来源（本笔 / 他路 / 存量 / 来源不明）
	"proposal", // 审计末行的提案号（无审计行 ⇒ 空 · 不填 `—`）
	"by",       // 审计末行的改动者（无审计行 ⇒ 空）
	"audit_at", // 审计末行时刻（无审计行 ⇒ 空）
	"evidence", // 一行证据（proposal · by · 时刻；无审计行 ⇒ 逐字「无审计行」）
}

// repoUncommittedBuckets —— mtime 分桶**闭集**（边界写死在此 · 出口按此序印）。
var repoUncommittedBuckets = []string{"<1h", "1-6h", "6-24h", "1-7d", ">7d", "未知"}

// repoUncommittedSources —— 来源**闭集**（顺序 = 人面表的分组序：本笔在最上，来源不明最下）。
var repoUncommittedSources = []string{"本笔", "他路", "存量", "来源不明"}

// rucArtifactPatterns —— 「构建副产物 / 编辑器垃圾」的**路径规则**（只判形状，不猜语义）。
//
// 三条口径：
//
//	① 只给**判词**（`artifact=是`），不因此把该件踢出清单 —— 「是副产物」也是未提交件，
//	   踢掉就又回到「少报」那半边病。
//	② 规则**形状稳定**：目录前缀 / 扩展名 / 精确件名三族，都是「一眼可复核」的形状；
//	   不写正则、不写任何本机绝对路径（红线：`check-hardcoded-private-paths`）。
//	③ 命中不等于「该删」—— 本面**只读**，不清、不删、不建议删。
var (
	rucArtifactDirs  = []string{"bin/", "dist/", "build/", "_history/", "node_modules/", "__pycache__/", ".cache/", "tmp/"}
	rucArtifactExts  = []string{".o", ".a", ".so", ".dylib", ".test", ".out", ".log", ".tmp", ".pyc", ".class"}
	rucArtifactNames = []string{".DS_Store", "coverage.out", "coverage.html"}

	rucSinceDefault   = 24 * time.Hour
	rucSinceDefaultS  = "24h"
	rucMaxAuditBuffer = 4 << 20
)

// rucEntry —— porcelain 的一行（**只装 git 给的东西**，不在这里加判断）。
type rucEntry struct {
	xy   string // 两列状态码原样
	path string // 仓根相对路径（`-z` ⇒ 无引号、无转义）
}

// rucAuditRec —— `edit_audit.jsonl` 里与该件有关的**末条**记录。
type rucAuditRec struct {
	At        time.Time
	AtRaw     string
	Event     string
	Proposal  string
	By        string
	AfterSHA  string // 该条改动**产出内容**的 sha256（判「是不是它产出当前字节」的唯一凭据）
	BeforeSHA string // 改前 sha（本面只留档，不参与判词）
}

// rucRow —— 一行的分类结果（`--json` 键就是这些字段名）。
type rucRow struct {
	path     string
	status   string
	kind     string
	artifact bool
	mtime    time.Time
	mtKnown  bool
	ageMin   int
	bucket   string
	source   string
	proposal string
	by       string
	auditAt  string
	evidence string
}

// repoUncommittedSpec —— 本命令的**声明**（`kind` / 字段表 / usage 的唯一一份）。
//
// 为什么要它（而不是只写 `main.go` 里那份结构体字面量）：本件落地时 `main.go` 正被另一路改
// ⇒ 本笔**不登记**；未登记期间 `--json` 的包封（`emitEnvelopeWith` 要一枚 `*command`）与
// 「合法字段」报错都要用同一份声明 ⇒ 声明放本件、注册行**引用本函数**，两处不各写一遍。
func repoUncommittedSpec() *command {
	return &command{
		path: []string{"repo", "uncommitted"},
		kind: "RepoUncommitted",
		summary: "**未提交件分类面**（只读 · 缺口 `GAP-20260926-78`）：把工作树里所有未提交件逐件摊成" +
			"「路径 / 类目 / 副产物判词 / mtime / 分桶 / **来源**」一行 —— 真源 = `git status --porcelain=v1 -z -uall`" +
			"（未跟踪目录**展开到件**；`git diff --stat` 对 `??` 件恒空、空输出还退 0 ⇒ 拿它数必然少报）；" +
			"mtime 分桶 `<1h/1-6h/6-24h/1-7d/>7d/未知`；来源按 `edit_audit.jsonl` 该件**末条 `edit` 行**对窗口起点" +
			"（`--since`，缺省 24h）分 `本笔/他路/存量/来源不明` 四档，证据列带 proposal/by ⇒ **哪件是谁的一眼可读**；" +
			"不给 `--me` ⇒ `本笔` 档恒 0（不猜「我」是谁）。零写操作（`--no-optional-locks`，不刷 `.git/index`）、写旗标一律拒",
		usage:    "zerg repo uncommitted [--root <仓根>] [--since <时长|RFC3339>] [--me <提案号|人>] [--audit <审计账>]... [--json <字段>]",
		arity:    "none",
		args:     []string{"（不收位置参数：仓根由 `--root` 或 env `ZERG_REPO` / cwd 上溯定）"},
		fields:   repoUncommittedFields,
		endpoint: "",
		run:      cmdRepoUncommitted,
	}
}

// cmdRepoUncommitted —— 本面唯一入口。
func cmdRepoUncommitted(inv *invocation, stdout, stderr io.Writer) int {
	// ① 同族同守：写旗标一律拒（`repo status` / `repo diffstat` 那一口径）。
	if hits := repoWriteHits(inv.orig); len(hits) > 0 {
		inv.setErr("usage", "write_flag_refused", "本命令是只读分类面 · 写旗标一律拒")
		fmt.Fprintf(stderr, "%s: 拒（退码 2 · 不给结论）：命令行里出现写旗标 %v\n", progName, hits)
		fmt.Fprintf(stderr, "%s: `repo uncommitted` **只读**（不清、不删、不建议删）· 要写请用 `%s repo commit`\n",
			progName, progName)
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `repo uncommitted` 不收位置参数 —— 多给了 %q\n", progName, inv.args[0])
		fmt.Fprintf(stderr, "用法：%s\n", repoUncommittedSpec().usage)
		return exitUsage
	}

	// ② 仓根：`--root` 优先（本面要能在仓外仓上读），否则走同族推导（env → 可执行件 → cwd）。
	root := strings.TrimSpace(inv.flagVal("--root"))
	if root == "" {
		root = repoRoot()
	}
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不了（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "不做的：不猜一个路径（在仓内跑，或给 `--root <仓根>`）\n")
		return exitBlocked
	}
	root = filepath.Clean(root)
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		inv.setErr("blocked", "root_not_dir", "仓根不是一个目录")
		fmt.Fprintf(stderr, "%s: 仓根不是一个目录：%s ⇒ 读不了（退码 8）\n", progName, root)
		return exitBlocked
	}

	// ③ 窗口起点：`--since` 收「时长」（24h / 90m / 7d / 1h30m）或 RFC3339 绝对时刻；缺省 24h。
	now := time.Now()
	since := now.Add(-rucSinceDefault)
	sinceRaw := rucSinceDefaultS
	if v := strings.TrimSpace(inv.flagVal("--since")); v != "" {
		t, err := rucParseSince(v, now)
		if err != nil {
			inv.setErr("usage", "bad_flag_value", "--since 解析不了")
			fmt.Fprintf(stderr, "%s: `--since` 解析不了：%q（%v）⇒ 退码 2\n", progName, v, err)
			fmt.Fprintf(stderr, "收两种形态：时长（`24h` / `90m` / `7d` / `1h30m`）或 RFC3339 绝对时刻（`2026-10-23T00:00:00+08:00`）\n")
			return exitUsage
		}
		since, sinceRaw = t, v
	}
	me := strings.TrimSpace(inv.flagVal("--me"))

	// ④ 真源：一条 `git status` 把已跟踪改动与未跟踪件**同一张表**取回来。
	//    `--no-optional-locks`：连索引 stat 刷新都不做（本面只读，不许在工作树落痕迹）；
	//    `-uall`：未跟踪**目录展开到件**（否则目录一行，mtime/逐件身份全丢 —— 本仓 17 行 vs 29 件）。
	out, err := runGitRONoLock(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		// 不是 git 工作树 / git 跑不起来：这一态**没有读数**，不许当「0 件（干净）」。
		inv.setErr("blocked", "git_status_failed", "git status 读不到")
		fmt.Fprintf(stderr, "%s: `git status` 在 %s 里读不到（%v）⇒ 不给结论（退码 8）\n", progName, root, err)
		fmt.Fprintf(stderr, "口径：读不到既不是「干净」也不是「N 件」—— 不许当绿\n")
		return exitBlocked
	}
	ents := rucParsePorcelain(out)

	// ⑤ 来源证据：审计账（唯一机器可读的「谁改的」）。
	//
	// ★ 为什么是**多账**（2026-10-23 实测发现）：`editAuditPath()` 的三级优先里第一级是
	//   `ZERG_STATE_DIR`，而本机每一枚 lane 都把自己那份 `edit_audit.jsonl` 落在
	//   `<lane>/state/` 下（现读 851 个 lane 目录）⇒ **只读一枚账，会把别的 lane 的改动全判成
	//   「来源不明」**（本笔实测：lane 账 0 行，全局账 12530 行）。故缺省读「本 lane 账 + 全局账」
	//   两处、同件取 `at` 最大者；`--audit <件>` 可再加（可重复）。显式 `ZERG_EDIT_AUDIT` ⇒ 只读它
	//   （那是**明确点名的覆盖**，不再掺全局账 —— 夹具与隔离场景要的就是这个语义）。
	aset := rucReadAudits(rucAuditLedgers(inv))
	if aset.broken > 0 {
		inv.warnf("审计账读不到 %d 处（共 %d 处）⇒ 那部分改动判不出「谁的」，按 mtime 降级分档",
			aset.broken, len(aset.ledgers))
		fmt.Fprintf(stderr, "%s: ⚠ 审计账 %s ⇒ 来源判不出「谁的」，只按 mtime 分档（不当绿）\n",
			progName, aset.brokenText())
	}

	// ⑥ 逐件分类。
	rows := make([]rucRow, 0, len(ents))
	for _, e := range ents {
		r := rucRow{path: e.path, status: e.xy}
		r.kind = rucKind(e.xy)
		r.artifact = rucIsArtifact(e.path)
		if r.artifact {
			r.kind += "·副产物"
		}
		if mt, ok := rucMtime(root, e.path); ok {
			r.mtime, r.mtKnown = mt, true
			r.ageMin = int(now.Sub(mt).Minutes())
		}
		r.bucket = rucBucket(r.mtime, r.mtKnown, now)
		sha, shaOK := rucFileSHA256(root, e.path)
		r.source, r.proposal, r.by, r.auditAt, r.evidence = rucClassify(
			e.path, aset, since, me, r.mtKnown, r.mtime, sha, shaOK)
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		si, sj := rucSourceRank(rows[i].source), rucSourceRank(rows[j].source)
		if si != sj {
			return si < sj
		}
		bi, bj := rucBucketRank(rows[i].bucket), rucBucketRank(rows[j].bucket)
		if bi != bj {
			return bi < bj
		}
		return rows[i].path < rows[j].path
	})

	bySource := rucCountBy(rows, func(r rucRow) string { return r.source })
	byBucket := rucCountBy(rows, func(r rucRow) string { return r.bucket })
	tracked, untracked := rucTrackedSplit(rows)

	// ⑦ 机器面。
	if inv.jsonGiven {
		return rucEmitJSON(inv, stdout, stderr, root, since, sinceRaw, me, aset,
			rows, bySource, byBucket, tracked, untracked)
	}

	// ⑧ 人面：抬头 + 分组表 + 合计（**0 件也逐字印 0** —— 空输出不许当干净）。
	fmt.Fprintf(stdout, "仓 %s · 窗口起点 %s（--since %s）· 审计 %s\n",
		root, since.Format(time.RFC3339), sinceRaw, aset.summaryText())
	fmt.Fprintf(stdout, "真源 git status --porcelain=v1 -z -uall ⇒ %d 件未提交（已跟踪 %d · 未跟踪 %d）\n",
		len(rows), tracked, untracked)
	if me == "" {
		fmt.Fprintf(stdout, "未给 --me ⇒ `本笔` 档恒 0（不猜「我」是谁；给 `--me <提案号|人>` 才分得出本笔）\n")
	} else {
		fmt.Fprintf(stdout, "本笔口径：审计末行的 proposal/by 含 %q 且末行时刻 ≥ 窗口起点\n", me)
	}
	fmt.Fprintln(stdout)
	if len(rows) == 0 {
		fmt.Fprintf(stdout, "0 件未提交（工作树干净）—— 这是**读数 0**，不是「读不到」\n")
		fmt.Fprintf(stdout, "（真源现跑：`git -C %s status --porcelain=v1 -z -uall` 无输出）\n", root)
		return exitOK
	}
	cols := []string{"source", "bucket", "kind", "artifact", "mtime", "age_min", "path", "evidence"}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		table = append(table, []string{
			r.source, r.bucket, r.kind, rucBool(r.artifact), rucMtimeText(r), rucAgeText(r), r.path, r.evidence,
		})
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, cols, table)
	fmt.Fprintf(stdout, "\n合计 %d 件 · 来源：%s\n", len(rows), rucCountText(bySource, repoUncommittedSources))
	fmt.Fprintf(stdout, "mtime 桶：%s\n", rucCountText(byBucket, repoUncommittedBuckets))
	fmt.Fprintf(stdout, "口径：来源 = 拿现内容 `sha256` 对审计账里的 `after_sha256`（**产出当前字节**那条）再看它 vs 窗口起点"+
		"（不是猜「谁在写」，是**有据可查**的那一半；账里有该件而一条都对不上 ⇒ 来源不明，不拿旧账当存量）；"+
		"副产物判词只按路径形状（%s）· 本面零写操作、不清不删\n", strings.Join(rucArtifactPatternsText(), " · "))
	return exitOK
}

// ── 解析面 ────────────────────────────────────────────────────────────────────

// rucParsePorcelain —— `-z` 输出的逐条解析。
//
// `-z` 的形状：每条 `XY<空格>路径`，条与条之间是 NUL；**改名/复制**那条后面**紧跟**一个 NUL 字段
// = 原路径（本面报新路径，原路径只用于人读时的心智对位 ⇒ 这里丢弃，但**必须吃掉**，
// 否则下一条会被当成一条独立记录、路径错位一格）。
func rucParsePorcelain(out string) []rucEntry {
	parts := strings.Split(out, "\x00")
	ents := make([]rucEntry, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		rec := parts[i]
		if len(rec) < 4 || rec[2] != ' ' {
			continue
		}
		xy := rec[:2]
		if xy[0] == 'R' || xy[0] == 'C' {
			if i+1 < len(parts) && parts[i+1] != "" {
				i++ // 原名：吃掉，不报（见上）
			}
		}
		ents = append(ents, rucEntry{xy: xy, path: rec[3:]})
	}
	return ents
}

// rucKind —— porcelain 两列 → 归一类别（**闭集**；认不出的形态原样进 `其他(<XY>)`，不硬塞一档）。
func rucKind(xy string) string {
	if len(xy) < 2 {
		return "其他(" + xy + ")"
	}
	x, y := xy[0], xy[1]
	switch {
	case x == '?' && y == '?':
		return "未跟踪"
	case x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D'):
		return "冲突"
	case x == 'R' || y == 'R':
		return "改名"
	case x == 'C' || y == 'C':
		return "复制"
	case x == 'A' || y == 'A':
		return "增"
	case x == 'D' || y == 'D':
		return "删"
	case x == 'M' || y == 'M':
		return "改"
	case x == 'T' || y == 'T':
		return "类型变"
	}
	return "其他(" + xy + ")"
}

// rucIsArtifact —— 副产物判词（**只看路径形状** · 规则见 rucArtifactDirs/Exts/Names）。
func rucIsArtifact(p string) bool {
	slashed := filepath.ToSlash(p)
	for _, d := range rucArtifactDirs {
		if strings.HasPrefix(slashed, d) || strings.Contains(slashed, "/"+d) {
			return true
		}
	}
	base := filepath.Base(slashed)
	for _, n := range rucArtifactNames {
		if base == n {
			return true
		}
	}
	lower := strings.ToLower(base)
	for _, ext := range rucArtifactExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// rucArtifactPatternsText —— 人面里逐条印出副产物规则（判词可复核，不是黑箱）。
func rucArtifactPatternsText() []string {
	out := []string{"目录 " + strings.Join(rucArtifactDirs, "/ ") + "/"}
	out = append(out, "扩展名 "+strings.Join(rucArtifactExts, " "))
	return append(out, "件名 "+strings.Join(rucArtifactNames, " "))
}

// rucMtime —— 该件 mtime。读不到（已删除件 / 权限）⇒ ok=false：**调用方不许拿 0 或 now 顶替**。
func rucMtime(root, rel string) (time.Time, bool) {
	st, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return time.Time{}, false
	}
	return st.ModTime(), true
}

// rucFileSHA256 —— 现内容的 sha256（判「哪条账行产出当前字节」的唯一凭据）。
//
// 读不到（已删除件 / 权限 / 是目录）⇒ ok=false：调用方退回「按末行时刻判」并**明写不精确**。
// 不做任何缓存：缓存会让「同一跑里两份读数」有机会不一致。
func rucFileSHA256(root, rel string) (string, bool) {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// rucBucket —— mtime 分桶（边界写死：1h / 6h / 24h / 7d）。读不到 ⇒ `未知`。
func rucBucket(mt time.Time, known bool, now time.Time) string {
	if !known {
		return "未知"
	}
	age := now.Sub(mt)
	switch {
	case age < time.Hour:
		return "<1h"
	case age < 6*time.Hour:
		return "1-6h"
	case age < 24*time.Hour:
		return "6-24h"
	case age < 7*24*time.Hour:
		return "1-7d"
	}
	return ">7d"
}

func rucBucketRank(b string) int {
	for i, v := range repoUncommittedBuckets {
		if v == b {
			return i
		}
	}
	return len(repoUncommittedBuckets)
}

func rucSourceRank(s string) int {
	for i, v := range repoUncommittedSources {
		if v == s {
			return i
		}
	}
	return len(repoUncommittedSources)
}

// rucParseSince —— 「时长」或「RFC3339 绝对时刻」→ 窗口起点。
//
// 为什么自己解 `d`：`time.ParseDuration` 不认 `d`（`7d` 是最自然的窗口写法），所以单补一条 `Nd`，
// 其余（`1h30m` / `90m`）仍交给标准库 —— 不另造一套时长语法。
func rucParseSince(v string, now time.Time) (time.Time, error) {
	s := strings.TrimSpace(v)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if strings.HasSuffix(s, "d") {
		if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil && n >= 0 {
			return now.Add(-time.Duration(n) * 24 * time.Hour), nil
		}
		return time.Time{}, fmt.Errorf("天数是整数：`7d`")
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	if d < 0 {
		return time.Time{}, fmt.Errorf("时长不许为负")
	}
	return now.Add(-d), nil
}

// ── 来源面（审计行对窗口）──────────────────────────────────────────────────────

// rucReadAudit —— 读**一枚**审计账，返回「该件 → 该件**全部** `edit` 行」+ 读到的行数 + 读错误。
//
// 口径四条：
//
//	① **只认带 `file` 的行**（`edit` 事件那批；`gap_ledger_written` 那批用的是 `gap_ledger_path`，
//	   不是工作树件 ⇒ 不许拿来当「谁改了这文件」的证据）。
//	② **该件全部行都留**（不是只留末条）：判归属要拿 `after_sha256` 去对**现内容**的 sha ——
//	   产出当前字节的往往**不是**时刻最后的那条（改回去/再改一次都会让末条换人）。
//	③ `after_sha256` = 那条改动**产出内容**的摘要（现网 6854 条 `edit` 行**全带**这格）。
//	   本面只拿它做**对拍**，不自己另算一套「内容身份」—— 与 `audit cover`（`GAP-20260928-98`）
//	   的「工作树 sha256 ↔ 审计 `after_sha256`」同一口径。
//	④ 坏行**跳过**、整件读不到**如实回错**（调用方据此降级归属判词，不许当绿）。
func rucReadAudit(path string) (map[string][]rucAuditRec, int, error) {
	recs := map[string][]rucAuditRec{}
	f, err := os.Open(path)
	if err != nil {
		return recs, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), rucMaxAuditBuffer)
	lines := 0
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		lines++
		var d map[string]any
		if err := json.Unmarshal([]byte(raw), &d); err != nil {
			continue
		}
		file, _ := d["file"].(string)
		file = strings.TrimPrefix(strings.TrimSpace(file), "./")
		if file == "" {
			continue
		}
		atRaw, _ := d["at"].(string)
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(atRaw))
		if err != nil {
			continue
		}
		ev, _ := d["event"].(string)
		prop, _ := d["proposal"].(string)
		by, _ := d["by"].(string)
		after, _ := d["after_sha256"].(string)
		before, _ := d["before_sha256"].(string)
		recs[file] = append(recs[file], rucAuditRec{
			At: at, AtRaw: strings.TrimSpace(atRaw), Event: ev, Proposal: prop, By: by,
			AfterSHA: strings.ToLower(strings.TrimSpace(after)), BeforeSHA: strings.ToLower(strings.TrimSpace(before)),
		})
	}
	if err := sc.Err(); err != nil {
		return recs, lines, err
	}
	return recs, lines, nil
}

// rucAuditLedgers —— 本面要读的审计账（**去重 · 保序**）。
//
// 三条口径：
//
//	① `--audit <件>`（可重复）在最前 —— 人显式点名的账优先读（也用于跨 lane 归位）。
//	② 显式 `ZERG_EDIT_AUDIT` ⇒ **只读它**：那是「明确点名的覆盖」，隔离夹具与「只看这一枚账」
//	   的场景要的就是这个语义（不再掺全局账，否则同一跑在不同机器上读数不同）。
//	③ 否则读「本 lane 账（`editAuditPath()` 三级优先）+ 全局账 `~/.zerg/state/edit_audit.jsonl`」：
//	   lane 账天然只装本 lane 的改动，全局账装没设 `ZERG_STATE_DIR` 那些路的改动 ⇒ 缺一就必然
//	   把一半改动判成「来源不明」。
func rucAuditLedgers(inv *invocation) []string {
	out := make([]string, 0, 3)
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		for _, x := range out {
			if x == p {
				return
			}
		}
		out = append(out, p)
	}
	for _, p := range inv.flagVals("--audit") {
		add(p)
	}
	if strings.TrimSpace(os.Getenv("ZERG_EDIT_AUDIT")) != "" {
		add(editAuditPath())
		return out
	}
	add(editAuditPath())
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".zerg", "state", "edit_audit.jsonl"))
	}
	return out
}

// rucLedgerState —— 一枚账的读数（读到几行 / 读不到的原因）。
type rucLedgerState struct {
	path  string
	lines int
	err   error
}

// rucAuditSet —— 多账合并后的证据面。
//
// 两张索引各司其职：
//
//	byPath —— 该件**全部** `edit` 行（依 `at` 升序），用于「末行是谁」的展示与「账里压根没有它」的判词；
//	shaIdx —— `path → after_sha256 → 那条行`（同 sha 取 `at` 最大者 —— 改回去再改回来的情形），
//	          用于「**产出当前字节**的是哪条」的精确对拍。
type rucAuditSet struct {
	byPath  map[string][]rucAuditRec
	shaIdx  map[string]map[string]rucAuditRec
	ledgers []rucLedgerState
	lines   int
	broken  int
}

// rucReadAudits —— 逐账读、合并（同一件同 sha：`at` 大的赢；同刻取**后读**的那枚 ⇒ 顺序确定、可复现）。
func rucReadAudits(paths []string) rucAuditSet {
	s := rucAuditSet{
		byPath:  map[string][]rucAuditRec{},
		shaIdx:  map[string]map[string]rucAuditRec{},
		ledgers: make([]rucLedgerState, 0, len(paths)),
	}
	for _, p := range paths {
		recs, lines, err := rucReadAudit(p)
		s.ledgers = append(s.ledgers, rucLedgerState{path: p, lines: lines, err: err})
		if err != nil {
			s.broken++
			continue
		}
		s.lines += lines
		for f, list := range recs {
			s.byPath[f] = append(s.byPath[f], list...)
			if len(list) > 0 && s.shaIdx[f] == nil {
				s.shaIdx[f] = map[string]rucAuditRec{}
			}
			for _, r := range list {
				if r.AfterSHA == "" {
					continue // 无摘要的行只能当「末行展示」，不能当「产出当前字节」的证据
				}
				if cur, ok := s.shaIdx[f][r.AfterSHA]; ok && r.At.Before(cur.At) {
					continue
				}
				s.shaIdx[f][r.AfterSHA] = r
			}
		}
	}
	for f := range s.byPath {
		list := s.byPath[f]
		sort.SliceStable(list, func(i, j int) bool { return list[i].At.Before(list[j].At) })
		s.byPath[f] = list
	}
	return s
}

// latest —— 该件「时刻最后」的那条（人面证据用；**不**等于「产出当前字节」那条）。
func rucLatest(list []rucAuditRec) (rucAuditRec, bool) {
	if len(list) == 0 {
		return rucAuditRec{}, false
	}
	return list[len(list)-1], true
}

// summaryText —— 人面抬头那一格：逐账印「路径 行数」（读不到的具名），不合并成一个数字掩盖口径。
func (s rucAuditSet) summaryText() string {
	parts := make([]string, 0, len(s.ledgers))
	for _, l := range s.ledgers {
		if l.err != nil {
			parts = append(parts, l.path+" 读不到")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %d 行", l.path, l.lines))
	}
	if len(parts) == 0 {
		return "无账可读（`--audit` 与 env 都没给出可用落点）"
	}
	return fmt.Sprintf("%d 处（%s）", len(s.ledgers), strings.Join(parts, " · "))
}

// brokenText —— 读不到那几处的具名串（人面/警告用）。
func (s rucAuditSet) brokenText() string {
	var parts []string
	for _, l := range s.ledgers {
		if l.err != nil {
			parts = append(parts, l.path+"："+l.err.Error())
		}
	}
	if len(parts) == 0 {
		return "全部读到"
	}
	return strings.Join(parts, " · ")
}

// rucClassify —— 一行的来源判词 + 证据（**四档闭集**；判词与证据同源，不各说一套）。
//
// 判据的**主语**是「产出当前字节的那条账行」，不是「时刻最后的那条」：
//
//	① 现内容 sha256 在账里找得到 ⇒ 该条就是归属证据（精确对拍，`after_sha256` 是现网账里的原格）；
//	   再看它的 `at` 与窗口起点/`--me`：≥ 起点且命中 ⇒ `本笔`；≥ 起点不命中 ⇒ `他路`；< 起点 ⇒ `存量`。
//	② 有账行、但**现内容 sha 一条都对不上** ⇒ `来源不明`（账解释不了现在这堆字节 —— 多半是绕过
//	   `dev edit` 正门改的 / 别的 lane 用自己的账 / 生成器重写过），证据列写明「末行是谁、差多少条」。
//	③ **一条账行都没有** ⇒ mtime 在窗口内 = `来源不明`，在窗口外 = `存量`。
//	④ 件读不到（已删除件 / 权限）⇒ 没有 sha 可比，退回按「末行时刻」判（口径同 ①，只是不精确），
//	   证据列逐字写「无现内容可比」。
//
// 账整片读不到（`broken == len(ledgers)`）时 `本笔`/`他路` 判不了 ⇒ 只按 mtime 落 `存量`/`来源不明`，
// 证据列明写「审计读不到」—— 不把「判不了」写成某个具体归属。
//
// ★ 为什么宁可留 `来源不明` 也不升成 `存量`：`存量` 的语义是「**账与件对得上**，只是旧」；
// 对不上还报存量，就是拿旧账掩盖「今夜改过没留账」—— 那正是本缺口要治的那种「只能人读」的含混。
func rucClassify(path string, aset rucAuditSet, since time.Time, me string,
	mtKnown bool, mt time.Time, sha string, shaOK bool) (source, proposal, by, auditAt, evidence string) {
	list := aset.byPath[path]
	var explain *rucAuditRec
	if shaOK {
		if m := aset.shaIdx[path]; m != nil {
			if r, ok := m[sha]; ok {
				explain = &r
			}
		}
	}
	if explain != nil {
		source = rucVerdict(*explain, since, me)
		evidence = rucJoinNonEmpty(" · ", explain.Proposal, explain.By, explain.AtRaw)
		if evidence == "" {
			evidence = "审计行（无 proposal/by）"
		}
		return source, explain.Proposal, explain.By, explain.AtRaw, evidence
	}
	if last, ok := rucLatest(list); ok {
		if !shaOK {
			// 口径④：无现内容可比（删除件）⇒ 按末行时刻判，但明写「不精确」
			source = rucVerdict(last, since, me)
			evidence = rucJoinNonEmpty(" · ", last.Proposal, last.By, last.AtRaw) +
				"（该件已无现内容可比 ⇒ 按末行时刻判，不精确）"
			return source, last.Proposal, last.By, last.AtRaw, evidence
		}
		// 口径②：账里有它，但没有一条产出当前字节
		source = "来源不明"
		evidence = "现内容 sha 与账内 " + strconv.Itoa(len(list)) + " 条 `after_sha256` 一条都对不上（收尾那条：" +
			rucJoinNonEmpty(" · ", last.Proposal, last.By, last.AtRaw) + "）"
		return source, last.Proposal, last.By, last.AtRaw, evidence
	}
	fresh := mtKnown && !mt.Before(since)
	if aset.broken > 0 && aset.broken == len(aset.ledgers) {
		if fresh {
			return "来源不明", "", "", "", "审计读不到 ⇒ 判不了归属（mtime 在窗口内）"
		}
		return "存量", "", "", "", "审计读不到 ⇒ 判不了归属（mtime 在窗口外）"
	}
	if fresh {
		return "来源不明", "", "", "", "无审计行（mtime 在窗口内 ⇒ 不是走 dev edit 正门改的 / 或别处账未读）"
	}
	if !mtKnown {
		return "存量", "", "", "", "无审计行 · mtime 读不到"
	}
	return "存量", "", "", "", "无审计行（mtime 在窗口外）"
}

// rucVerdict —— 拿到「产出当前字节的那条账行」之后，按窗口起点与 `--me` 定四档里的三档。
func rucVerdict(rec rucAuditRec, since time.Time, me string) string {
	inWin := !rec.At.Before(since)
	switch {
	case inWin && me != "" && rucMatchMe(rec, me):
		return "本笔"
	case inWin:
		return "他路"
	}
	return "存量"
}

// rucHumanGap —— 时长摊成人读的一格（天/小时/分钟三档；判据用真时长，这里只做显示）。
func rucHumanGap(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%.1f 天", d.Hours()/24)
	case d >= time.Hour:
		return fmt.Sprintf("%.1f 小时", d.Hours())
	}
	return fmt.Sprintf("%d 分钟", int(d.Minutes()))
}

// rucMatchMe —— `--me` 的匹配口径：末行 `proposal` 或 `by` **含**该串（大小写不敏感）。
//
// 用「含」不用「等」：`--me DEV-3162` 要能命中 `DEV-3162`，`--me Hermes` 要能命中 `Hermes (AI)`。
// 这是**人给的自报锚**，不是身份认证（本面不做认证，只做归属对位）。
func rucMatchMe(rec rucAuditRec, me string) bool {
	m := strings.ToLower(me)
	return strings.Contains(strings.ToLower(rec.Proposal), m) || strings.Contains(strings.ToLower(rec.By), m)
}

// ── 出口面 ────────────────────────────────────────────────────────────────────

// rucEmitJSON —— `--json` 面。字段校验用**本件声明的字段表**（未登记期间 `fieldListOf` 取不到）。
func rucEmitJSON(inv *invocation, stdout, stderr io.Writer, root string, since time.Time, sinceRaw, me string,
	aset rucAuditSet, rows []rucRow, bySource, byBucket map[string]int, tracked, untracked int) int {
	if len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: --json 需要逗号分隔的字段列表\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(repoUncommittedFields, ","))
		fmt.Fprintf(stderr, "下一步：例 `%s repo uncommitted --json %s`\n", progName, strings.Join(repoUncommittedFields[:3], ","))
		return exitUsage
	}
	for _, f := range inv.fields {
		if !rucFieldLegal(f) {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, []string{"repo", "uncommitted"}, f)
		}
	}
	cmd := find([]string{"repo", "uncommitted"})
	if cmd == nil {
		// 未登记（本件落地时 `main.go` 正被另一路改）⇒ 用同一份声明出包封；
		// 登记后 `find()` 命中同一条，两条路给出同一个 kind 与同一份字段表。
		cmd = repoUncommittedSpec()
	}
	inv.metaAddStr("root", root)
	inv.metaAddStr("since", since.Format(time.RFC3339))
	inv.metaAddStr("since_arg", sinceRaw)
	inv.metaAddStr("audit", aset.summaryText())
	inv.metaAddJSON("audit_ledgers", rucLedgerJSON(aset))
	inv.metaAddJSON("by_source", rucCountJSON(bySource, repoUncommittedSources))
	inv.metaAddJSON("by_bucket", rucCountJSON(byBucket, repoUncommittedBuckets))
	inv.metaAddJSON("tracked", strconv.Itoa(tracked))
	inv.metaAddJSON("untracked", strconv.Itoa(untracked))
	if me != "" {
		inv.metaAddStr("me", me)
	}
	switch {
	case aset.broken > 0 && aset.broken == len(aset.ledgers):
		inv.metaAddStr("audit_state", "读不到："+aset.brokenText())
	case aset.broken > 0:
		// 部分账读不到 ⇒ 不许写成「读不到」（那会把读到的那部分一并抹掉）
		inv.metaAddJSON("audit_lines", strconv.Itoa(aset.lines))
		inv.metaAddStr("audit_state", "部分读不到："+aset.brokenText())
	default:
		inv.metaAddJSON("audit_lines", strconv.Itoa(aset.lines))
		inv.metaAddStr("audit_state", "读到")
	}
	objs := make([]string, 0, len(rows))
	for _, r := range rows {
		objs = append(objs, marshalObjectRegistered(inv.fields, r.fields()))
	}
	emitEnvelopeWith(stdout, cmd, "["+strings.Join(objs, ",")+"]", len(rows), inv)
	return exitOK
}

// fields —— 一行 → `--json` 的键值面（键名与字段表**逐字同源**）。
func (r rucRow) fields() map[string]string {
	m := map[string]string{
		"path":     r.path,
		"status":   r.status,
		"kind":     r.kind,
		"artifact": rucBool(r.artifact),
		"bucket":   r.bucket,
		"source":   r.source,
		"evidence": r.evidence,
	}
	if r.mtKnown {
		m["mtime"] = r.mtime.Format(time.RFC3339)
		m["age_min"] = strconv.Itoa(r.ageMin)
	} else {
		m["mtime"] = "（读不到）"
		// age_min 缺席：不许拿 0 顶替（`emitSelected` 同口径：缺值整键略去）
	}
	if r.proposal != "" {
		m["proposal"] = r.proposal
	}
	if r.by != "" {
		m["by"] = r.by
	}
	if r.auditAt != "" {
		m["audit_at"] = r.auditAt
	}
	return m
}

func rucFieldLegal(f string) bool {
	for _, v := range repoUncommittedFields {
		if v == f {
			return true
		}
	}
	return false
}

func rucBool(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

func rucMtimeText(r rucRow) string {
	if !r.mtKnown {
		return "（读不到）"
	}
	return r.mtime.Format(time.RFC3339)
}

func rucAgeText(r rucRow) string {
	if !r.mtKnown {
		return ""
	}
	return strconv.Itoa(r.ageMin)
}

func rucJoinNonEmpty(sep string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, sep)
}

func rucCountBy(rows []rucRow, key func(rucRow) string) map[string]int {
	m := map[string]int{}
	for _, r := range rows {
		m[key(r)]++
	}
	return m
}

// rucCountText —— 按闭集顺序印计数（闭集里 0 也印 ⇒ 「他路 0」是读数，不是缺席）。
func rucCountText(m map[string]int, order []string) string {
	parts := make([]string, 0, len(order))
	for _, k := range order {
		parts = append(parts, k+" "+strconv.Itoa(m[k]))
	}
	return strings.Join(parts, " · ")
}

func rucCountJSON(m map[string]int, order []string) string {
	parts := make([]string, 0, len(order))
	for _, k := range order {
		parts = append(parts, jstr(k)+":"+strconv.Itoa(m[k]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func rucTrackedSplit(rows []rucRow) (tracked, untracked int) {
	for _, r := range rows {
		if r.kind == "未跟踪" || strings.HasPrefix(r.kind, "未跟踪·") {
			untracked++
			continue
		}
		tracked++
	}
	return tracked, untracked
}

// rucLedgerJSON —— 逐账读数进 `meta`（`{路径: 行数}`；读不到 ⇒ 字符串「读不到：原因」）。
func rucLedgerJSON(aset rucAuditSet) string {
	parts := make([]string, 0, len(aset.ledgers))
	for _, l := range aset.ledgers {
		if l.err != nil {
			parts = append(parts, jstr(l.path)+":"+jstr("读不到："+l.err.Error()))
			continue
		}
		parts = append(parts, jstr(l.path)+":"+strconv.Itoa(l.lines))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// ── 注册行（原文 · 由改 `main.go` 的那一路照贴；本件**不自己登记**）────────────────
//
// 贴进 `core/cmd/zerg/main.go` 的 `commands` 表（与 `{"repo","diffstat"}` 那条相邻即可）：
//
//		// ---- `GAP-20260926-78`（2026-10-23）：**未提交件分类面**（只读 · 零写盘）----
//		// `git diff --stat` 对 `??` 件恒空、空输出还退 0 ⇒ 数不出「工作树有几件未提交」；
//		// 本面真源 = `git status --porcelain=v1 -z -uall`（未跟踪目录展开到件），逐件给
//		// kind/artifact/mtime/bucket/source；来源 = 现内容 `sha256` 对审计账 `after_sha256`
//		// 找出「产出当前字节」那条，再看它 vs 窗口起点分 本笔/他路/存量/来源不明 四档。
//		// ★ 声明（path/kind/summary/usage/fields）全在 `repoUncommittedSpec()`，此处只转一手 ⇒ 两处不各写一份。
//		{
//			path:   []string{"repo", "uncommitted"},
//			kind:   "RepoUncommitted",
//			summary: repoUncommittedSpec().summary,
//			usage:   repoUncommittedSpec().usage,
//			arity:   "none",
//			args:    []string{"（不收位置参数：仓根由 `--root` 或 env `ZERG_REPO` / cwd 上溯定）"},
//			fields:  repoUncommittedFields,
//			endpoint: "",
//			run:     cmdRepoUncommitted,
//		},
//
// 只贴 `run: cmdRepoUncommitted` 一行也能跑（`path` 必须同时登记，否则命令树找不到它）；
// 上面的形态是**推荐**形态：声明与本件同源，登记行只做接线。
