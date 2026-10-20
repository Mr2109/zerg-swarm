// family_registry_sync.go —— `zerg registry sync`（`GAP-20261020-03` 下半 · Mr2109 拍「方案 B：一处登记，其余自动校验」）。
//
// 病（上半 `registry_fanout_test.go` 逐字记着）：新增一件要在五处分别声明，五处互不联动，
// 也没有一处能告诉你「还缺哪一处」⇒ 每次被挡都要人工翻日志才知道缺哪格。
// 上半 = **判据件**（棘轮 · 缺面就红并点名）；本件 = **补面**（把能由件本身推出来的那两面补齐）。
//
// ★★★ 硬约束（本件存在的边界，别在第二处再放宽一次）：
//
//	本命令**只补能由「件本身」推出来的那一面** ——
//	  ① `scripts/公开标记.tsv` 缺行 ⇒ 补一行（第 2 列缺省 `no`；第 3 列逐字写「自动补登 · 待人工确认用途」）；
//	  ② `core/internal/contract/` 的干跑语义表里 `.sh` 缺条目 ⇒ **一律不补**，只在输出里列为「需人写」。
//
//	为什么②不补：干跑语义是**判断**（这一件有没有干跑站点 · 零副作用的判据是什么），
//	不是**事实**。凭「它是个 .sh」就替它写一条语义 = 凭空造语义，比缺一条坏得多
//	（同族旧账：判据被台账喂饱 ⇒ 门从此永不红）。同理**门步骤 / 契约矩阵一律不碰** ——
//	本命令一个字节都不写这三个面。
//
// 面（`zerg registry sync [--dry-run] [--json <字段>]`）：
//
//	· 扫面 = `scripts/**` 下的 `.sh` / `.py`（口径与判据件 `fanoutScripts` 逐字同：
//	  两个后缀 · `filepath.Walk` · 相对仓根 `/` 分隔 · 排序）；
//	· `--dry-run` = **零副作用**（逐字节：不建 tmp、不 rename、不建目录），只把**将改的行**打出来；
//	· 真跑 = **原子写**（同目录 `CreateTemp` → 写 → `Sync` → 权限照原件 → `rename`），
//	  只**尾部追加**（`公开标记.tsv` 的既有纪律：按落件时刻成波，✗ 不按字典序插入）；
//	· 幂等：同一棵树上连跑两遍，第二遍**补 0 行**（`meta.changed=false`）；
//	· 仓根 = env `ZERG_REPO_ROOT`（非空即逐字采信）＞ cwd 上溯找含 `scripts/公开标记.tsv` 的目录 ＞ cwd。
//
// 退码（三档齐）：
//
//	0 读得全（**含「有缺面」** —— 缺面是这命令要补的东西，不是它的失败）；
//	2 用法错（收位置参数 / `--json` 不给字段 / `--json` 字段名不在册）—— 执行前判、零写盘；
//	8 不给结论（仓根 / 标记表 / 语义表读不到，或原子写失败 —— ✗ 不当「补 0 行」的绿）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// rsFields —— `--json` 字段表（与命令树 `fields:` 同源）。
var rsFields = []string{"action", "face", "file", "line"}

// 两个动作（闭集 · 机器面取值）：补一行 / 需人写。
const (
	rsActAddMark    = "add_mark"
	rsActNeedsHuman = "needs_human"
	// 两面（闭集）：公开标记 / 干跑语义。
	rsFaceMarks = "公开标记"
	rsFaceDry   = "干跑语义"
	// rsAutoNote —— 自动补登那一行的第 3 列（逐字固定：人读到它就知道这行**没经人确认**）。
	rsAutoNote = "自动补登 · 待人工确认用途"
	// rsDefaultPublic —— 第 2 列缺省值（`no` = 不进公开面；要进公开面是人拍的事）。
	rsDefaultPublic = "no"
)

// rsPlan —— 一次 sync 的**全部读数与将改项**（纯计算，不碰盘）。
type rsPlan struct {
	Root      string
	Scripts   int
	Sh        int
	Py        int
	MarkRows  int // 公开标记表现有登记行数（表头不算）
	DryRows   int // 干跑语义表现有条目数
	AddMarks  []string
	NeedHuman []string
}

// rsMarkLine —— 自动补登的那一行（列间 TAB；第 3 列逐字固定）。
func rsMarkLine(rel string) string {
	return rel + "\t" + rsDefaultPublic + "\t" + rsAutoNote
}

// rsRepoRoot —— 见件头「仓根」那一行：env ＞ cwd 上溯 ＞ cwd（返回 (根, 取法·只给人读)）。
func rsRepoRoot() (string, string) {
	if env := strings.TrimSpace(os.Getenv("ZERG_REPO_ROOT")); env != "" {
		return env, "env ZERG_REPO_ROOT"
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "cwd 取不到"
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if st, serr := os.Stat(filepath.Join(dir, "scripts", "公开标记.tsv")); serr == nil && st.Mode().IsRegular() {
			return dir, "cwd 上溯"
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd, "cwd（未上溯到含 scripts/公开标记.tsv 的目录）"
}

// rsScanScripts —— 现读脚本面（口径与 `fanoutScripts` 逐字同：`.sh` / `.py` 两后缀 · 跳过点目录）。
func rsScanScripts(root string) ([]string, error) {
	var out []string
	base := filepath.Join(root, "scripts")
	err := filepath.Walk(base, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info == nil {
			return nil
		}
		if info.IsDir() {
			if p != base && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".sh") || strings.HasSuffix(p, ".py") {
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				return rerr
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// rsReadMarks —— 读 `scripts/公开标记.tsv` 的第一列集合（跳过空行与 `#` 注释；表头自然被收进集合，无害）。
func rsReadMarks(root string) (map[string]bool, int, error) {
	b, err := os.ReadFile(filepath.Join(root, "scripts", "公开标记.tsv"))
	if err != nil {
		return nil, 0, err
	}
	out := map[string]bool{}
	n := 0
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimRight(ln, "\r")
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		f := strings.TrimSpace(strings.SplitN(ln, "\t", 2)[0])
		if f == "" {
			continue
		}
		if !out[f] {
			n++
		}
		out[f] = true
	}
	return out, n, nil
}

// rsReadDryrun —— 读干跑语义表的 `件[].file` 集合（**只读**：本命令绝不写这张表）。
func rsReadDryrun(root string) (map[string]bool, int, error) {
	b, err := os.ReadFile(filepath.Join(root, "core", "internal", "contract", "dryrun-semantics.json"))
	if err != nil {
		return nil, 0, err
	}
	var d struct {
		Items []struct {
			File string `json:"file"`
		} `json:"件"`
	}
	if jerr := json.Unmarshal(b, &d); jerr != nil {
		return nil, 0, fmt.Errorf("干跑语义表不是合法 JSON：%w", jerr)
	}
	out := map[string]bool{}
	for _, it := range d.Items {
		if it.File != "" {
			out[it.File] = true
		}
	}
	return out, len(d.Items), nil
}

// rsBuildPlan —— 纯计算：现读三面 ⇒ 将补的行 ＋ 需人写的件。不写盘、不建目录。
func rsBuildPlan(root string) (*rsPlan, error) {
	scripts, err := rsScanScripts(root)
	if err != nil {
		return nil, fmt.Errorf("扫 scripts/ 失败：%w", err)
	}
	marks, markRows, err := rsReadMarks(root)
	if err != nil {
		return nil, fmt.Errorf("读 scripts/公开标记.tsv 失败：%w", err)
	}
	dry, dryRows, err := rsReadDryrun(root)
	if err != nil {
		return nil, fmt.Errorf("读 core/internal/contract/dryrun-semantics.json 失败：%w", err)
	}
	p := &rsPlan{Root: root, Scripts: len(scripts), MarkRows: markRows, DryRows: dryRows}
	for _, s := range scripts {
		if strings.HasSuffix(s, ".sh") {
			p.Sh++
		} else {
			p.Py++
		}
		if !marks[s] {
			p.AddMarks = append(p.AddMarks, rsMarkLine(s))
		}
		// ② 语义面：**只列不补**（硬约束）。
		if strings.HasSuffix(s, ".sh") && !dry[s] {
			p.NeedHuman = append(p.NeedHuman, s)
		}
	}
	return p, nil
}

// rsAppendRows —— 原子追加（同目录 tmp → Sync → 权限照原件 → rename）；只追加、不重排。
func rsAppendRows(path string, rows []string) error {
	if len(rows) == 0 {
		return nil
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if st, serr := os.Stat(path); serr == nil {
		mode = st.Mode().Perm()
	}
	var b bytes.Buffer
	b.Write(old)
	if len(old) > 0 && !bytes.HasSuffix(old, []byte("\n")) {
		b.WriteString("\n")
	}
	for _, r := range rows {
		b.WriteString(r)
		b.WriteString("\n")
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".公开标记-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.Write(b.Bytes()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// rsRows —— 机器面：一行一条待办（`add_mark` 带将写的那一行；`needs_human` 只有件名 · `line` 整键略去）。
func rsRows(p *rsPlan) []map[string]string {
	rows := make([]map[string]string, 0, len(p.AddMarks)+len(p.NeedHuman))
	for _, ln := range p.AddMarks {
		f := strings.SplitN(ln, "\t", 2)[0]
		rows = append(rows, map[string]string{"action": rsActAddMark, "face": rsFaceMarks, "file": f, "line": ln})
	}
	for _, f := range p.NeedHuman {
		rows = append(rows, map[string]string{"action": rsActNeedsHuman, "face": rsFaceDry, "file": f})
	}
	return rows
}

// cmdRegistrySync —— `zerg registry sync` 的实现（退码见件头）。
func cmdRegistrySync(inv *invocation, stdout, stderr io.Writer) int {
	// 用法面先判（零副作用）—— 位置参数 / --json 两档全在**碰盘之前**。
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "本命令不收位置参数")
		fmt.Fprintf(stderr, "%s: `registry sync` 不收位置参数（收到 %q）⇒ 退码 2\n", progName, inv.args[0])
		return exitUsage
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		if rc := validateSelected(inv, stderr, inv.path, inv.fields); rc != exitOK {
			return rc
		}
	}

	root, how := rsRepoRoot()
	plan, err := rsBuildPlan(root)
	if err != nil {
		inv.setErr("blocked", "file_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 取不到真源 ⇒ 不给结论（退码 8）：%v\n", progName, err)
		return exitBlocked
	}

	changed := false
	if !inv.dryRun && len(plan.AddMarks) > 0 {
		if werr := rsAppendRows(filepath.Join(root, "scripts", "公开标记.tsv"), plan.AddMarks); werr != nil {
			inv.setErr("blocked", "write_failed", werr.Error())
			fmt.Fprintf(stderr, "%s: 原子写失败 ⇒ 不给结论（退码 8 · 盘上原件未被替换）：%v\n", progName, werr)
			return exitBlocked
		}
		changed = true
	}
	inv.changed = &changed

	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, rsFields, rsRows(plan))
	}

	// 人面：读数 → 将改/已改的行 → 需人写的件 → 硬约束那一条。
	fmt.Fprintf(stdout, "%s registry sync（仓根 %s · 取自 %s）\n", progName, root, how)
	fmt.Fprintf(stdout, "  脚本面 %d 件（.sh %d · .py %d）· 公开标记已登记 %d 行 · 干跑语义已登记 %d 条\n",
		plan.Scripts, plan.Sh, plan.Py, plan.MarkRows, plan.DryRows)
	if len(plan.AddMarks) == 0 {
		fmt.Fprintf(stdout, "  公开标记：**不缺行**（0 行待补）\n")
	} else {
		verb := "已补"
		if inv.dryRun {
			verb = "将补"
		}
		fmt.Fprintf(stdout, "  公开标记：%s %d 行（第 2 列 `%s` · 第 3 列逐字 `%s` · 尾部追加）\n",
			verb, len(plan.AddMarks), rsDefaultPublic, rsAutoNote)
		for _, ln := range plan.AddMarks {
			fmt.Fprintf(stdout, "    + %s\n", ln)
		}
	}
	if len(plan.NeedHuman) == 0 {
		fmt.Fprintf(stdout, "  干跑语义：`.sh` 全在册（0 件需人写）\n")
	} else {
		fmt.Fprintf(stdout, "  干跑语义：**%d 件需人写**（本命令✗不代写语义 —— 要判断有没有干跑站点、零副作用判据是什么，那是人写的事）\n",
			len(plan.NeedHuman))
		for _, f := range plan.NeedHuman {
			fmt.Fprintf(stdout, "    ! %s\n", f)
		}
	}
	if inv.dryRun {
		fmt.Fprintf(stdout, "  --dry-run：盘上一字节未动（将改 %d 行 · 未建 tmp、未 rename）\n", len(plan.AddMarks))
	} else {
		fmt.Fprintf(stdout, "  写入：%s（原子写：同目录 tmp → rename；本次 changed=%t）\n",
			filepath.Join(root, "scripts", "公开标记.tsv"), changed)
	}
	return exitOK
}
