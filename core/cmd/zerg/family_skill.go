// family_skill.go —— `zerg skill` 族：技能库的**命令面**（M1 = ls / show 两条只读）。
//
// 病灶（设计稿 `zerg-技能库设计稿-20261017.md` §1 现读）：技能机制**早就在运行期** ——
// `core/internal/agent/skill_manager.go` 扫 `<根>/<名>/SKILL.md`、frontmatter 的
// name/description 注入系统提示（`:95-111`）、`skill_load` 读正文（`:114-120`）；
// 而**命令面零对应** ⇒ 人看不见有哪些技能、看不出某条为什么没被加载。本件把读面接出正门。
//
// 四条硬口径：
//
//	① **不另立第二份解析**：frontmatter 只由 `agent` 自己读（本件零解析）——
//	   `ls` 的 description 逐字取自 `agent.(*SkillManager).ListDescriptions()` 的**注入面原文**
//	   （与系统提示里那一行**同一个渲染口**），`show` 的正文只走
//	   `agent.NewSkillManager(dir).Load(名)` —— 与 CA 侧 `skill_load` **同一函数**
//	   ⇒ 「show 出来的」与「模型加载到的」**构造上逐字节相同**，不靠对拍。
//	   ★ 故本件**没有** frontmatter 解析器：`parseFrontmatter` 至今未导出，而本笔的边界是
//	   ✗ 不动 `core/internal/**` ⇒ 宁走 agent 的现成两个读口，也不在这里抄一份 YAML 抠字。
//	② **根只有一个**：`statepath.SkillsDir()`（`ZERG_SKILLS_DIR` → `<仓根>/core/internal/agent/skills`
//	   → 空串；现读口径 `core/internal/statepath/statepath.go:147-163`）。
//	   ✗ 不开 `--root`、✗ 不新增第二个默认根、✗ 不把 `~/.zerg/skills` 写成默认值。
//	③ **读面零写**：`ls` 只 `ReadDir`+`ReadFile`；`show` 只读一个件。不起子进程、不联网。
//	④ **读不到 ≠ 没有**：agent 会**静默跳过**的件（子目录没 `SKILL.md` / 根下散件）本件照实点名，
//	   用 `state` 列标出来（五档闭集），不跟着一起静默 —— 否则 `ls` 会骗人。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ── 字段面 / 显示面 ──────────────────────────────────────────────────────────
//
// ★ `--json` 字段表**不在这里**：`main.go` 的每条 `fields:` 一律写字面量
// `[]string{…}`（契约脚本 `clicFieldsRE` 直读那一行 · `check_cli_contract.go:329`）。
// 本件只放**人面**的列（与 `api ls` 先例同形：显示列 ⊂ 字段面 · `readonly.go:163`）。

var skillLsDisplay = []string{"name", "description", "state"}

// skillNameRE —— 技能名（= 目录名）。与 Hermes / Claude Code 的技能目录同规：
// 小写字母数字起头，允许 `-` `.` `_`，1..64 字符。**结构性拒绝** `..` 与路径分隔符
// （不是靠正则兜底 —— `..` 能过 `[a-z0-9.-]`，必须单判一次）。
//
// ★ M1（ls/show）不写面 ⇒ 本枚只在 M2（`skill new|edit|rm` 的名字校验）读；留着是为了
// 让 M2 不必再动一次名字口径（设计稿 §3.3 的 `^[a-z0-9][a-z0-9-]{0,63}$` 是它的简写版）。
var skillNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// skillState* —— `ls`/`show` 的 `state` 闭集**五档**（判据见设计稿 §3.1 的表；
// 与 `agent.(*SkillManager).scan` 逐条对齐 —— 前两档 agent 会加载，后三档要么不对要么加载不到）。
const (
	skillStateOK     = "ok"             // agent 能加载，描述也在
	skillStateNoDesc = "no-description" // 能加载，但注入面是空描述（等于没触发器）
	skillStateNoFM   = "no-frontmatter" // 能加载（name 退目录名），描述为空
	skillStateNoDoc  = "no-skill-md"    // **加载不到**：是目录但没有 SKILL.md（agent 静默跳过）
	skillStateNotDir = "not-a-dir"      // **加载不到**：根下的散件（现读 `code_audit.md`）
)

// skillEntry —— 一件技能（或一件**本该是技能但不成**的件）的现读。
type skillEntry struct {
	Name  string
	Desc  string
	Path  string // SKILL.md 绝对路径（not-a-dir 时是那件散件本身）
	State string
	Files int // 子件数（references/scripts/templates 下的普通件；只为"这条技能有多重"）
	Bytes int64
}

// skillRoot —— 唯一解析口。返回 (根, 为什么判不了)。
func skillRoot() (string, string) {
	dir := strings.TrimSpace(statepath.SkillsDir())
	if dir == "" {
		return "", "解析不到技能根（`ZERG_SKILLS_DIR` 未设，且仓内 core/internal/agent/skills 不在）"
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return "", "技能根不在盘上：" + dir + "（" + err.Error() + "）"
	}
	if !fi.IsDir() {
		return "", "技能根不是目录：" + dir
	}
	return dir, ""
}

// skillDescs —— 从 `ListDescriptions()` 的**注入面原文**取「名 → 描述」。
//
// 为什么走这里而不是自己读 frontmatter：`ListDescriptions()` 就是**进系统提示的那一份**
// （`skill_manager.go:95-111`：`- 名: 描述` 逐行）⇒ 取它等于取「模型真正看到的触发器」，
// 且保住了「唯一解析点」这条纪律（本件零解析）。行前缀 `- ` 与分隔 `: ` 都是该函数
// 写死的形状；解析不到的行**跳过**（不猜、不补一个描述）。
func skillDescs(sm *agent.SkillManager) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(sm.ListDescriptions(), "\n") {
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		rest := strings.TrimPrefix(line, "- ")
		i := strings.Index(rest, ": ")
		if i < 0 {
			continue
		}
		out[rest[:i]] = rest[i+2:]
	}
	return out
}

// skillAgentName —— 一件技能目录在 **agent 口径下**的名字（= 加载时用的那把钥匙）。
//
// 三支（顺序即优先级）：
//  1. `SKILL.md` 路径落在 `<根>/<名>/` 里 ⇒ 名 = 目录名（绝大多数）；
//  2. 否则用**正文逐字节相同**反查（frontmatter `name` 与目录名不一致的那种）——
//     `Load(名)` 返回的就是整篇正文，逐字节比即可，**不必解析 frontmatter**；
//  3. 都查不到 ⇒ 退目录名（agent 自己的兜底口径 `skill_manager.go:61-63`）。
func skillAgentName(sm *agent.SkillManager, root, doc, dir string, data []byte) string {
	for _, n := range sm.SkillNames() {
		if filepath.Join(root, n, "SKILL.md") == doc {
			return n
		}
	}
	body := string(data)
	for _, n := range sm.SkillNames() {
		if b, err := sm.Load(n); err == nil && b == body {
			return n
		}
	}
	return dir
}

// skillScan —— 逐件现读（**只读**）。口径与 `agent.(*SkillManager).scan` 对齐到"能对上"为止：
// 只认子目录、跳过 `.` 开头；**额外**把 agent 会静默跳过的件补成 `not-a-dir` / `no-skill-md` 行 ——
// 「读不到 ≠ 没有」：agent 看不见的件，命令面必须看得见（否则 `ls` 会骗人）。
func skillScan(root string) ([]skillEntry, error) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	sm := agent.NewSkillManager(root) // ★ 单一解析口：frontmatter 只由 agent 读（本件零解析）
	descs := skillDescs(sm)
	out := []skillEntry{}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // AppleDouble `._*` / 隐藏目录：不是技能（同 skill_manager.go:49-53）
		}
		abs := filepath.Join(root, name)
		if !e.IsDir() {
			out = append(out, skillEntry{Name: name, Path: abs, State: skillStateNotDir})
			continue
		}
		doc := filepath.Join(abs, "SKILL.md")
		data, rerr := os.ReadFile(doc)
		if rerr != nil {
			// 子目录但没有 SKILL.md：agent 跳过（`skill_manager.go:54-58` 读失败即 continue）；
			// 命令面照实点名 —— 这正是「读不到 ≠ 没有」的那一格。
			out = append(out, skillEntry{Name: name, Path: doc, State: skillStateNoDoc})
			continue
		}
		nm := skillAgentName(sm, root, doc, name, data)
		desc := descs[nm]
		st := skillStateOK
		switch {
		case !strings.HasPrefix(string(data), "---\n"):
			st = skillStateNoFM
		case strings.TrimSpace(desc) == "":
			st = skillStateNoDesc
		}
		out = append(out, skillEntry{
			Name: nm, Desc: desc, Path: doc, State: st,
			Files: skillSubfileCount(abs), Bytes: int64(len(data)),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// skillSubfileCount —— 一件技能目录下的**普通件**数（不含 SKILL.md 本身）。
func skillSubfileCount(dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 数不出就少数，不猜（Files 只是"有多重"的参考量）
		}
		if d.IsDir() || p == filepath.Join(dir, "SKILL.md") {
			return nil
		}
		n++
		return nil
	})
	return n
}

func skillNames(es []skillEntry) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name)
	}
	return out
}

// cmdSkillLs —— `zerg skill ls [--json <字段>]`。
func cmdSkillLs(inv *invocation, stdout, stderr io.Writer) int {
	root, why := skillRoot()
	if root == "" {
		inv.setErr("blocked", "skill_root_absent", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 列不出技能面（不给结论 · 退码 8）\n", progName, why)
		return exitBlocked
	}
	es, err := skillScan(root)
	if err != nil {
		inv.setErr("blocked", "skill_root_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了技能根 %s（%v）⇒ 不给结论（退码 8）\n", progName, root, err)
		return exitBlocked
	}
	if len(es) == 0 {
		// 空转 = 假覆盖（同 `calib ls` 的 roster_empty 先例 · family_calib.go:143-148）
		inv.setErr("blocked", "skill_root_empty", "技能根一件都没有")
		fmt.Fprintf(stderr, "%s: 技能根 %s 一件都没有 ⇒ 不给结论（退码 8）\n", progName, root)
		return exitBlocked
	}
	rows := make([]map[string]string, 0, len(es))
	for _, e := range es {
		rows = append(rows, map[string]string{
			"name": e.Name, "description": e.Desc, "state": e.State,
			"files": fmt.Sprintf("%d", e.Files), "bytes": fmt.Sprintf("%d", e.Bytes),
			"path": e.Path,
		})
	}
	fmt.Fprintf(stderr, "%s: 技能根 %s · %d 件（`ok` = agent 能加载；其余见 `state` 列）\n", progName, root, len(es))
	return listCmd(inv, stdout, stderr, skillLsDisplay, rows)
}

// cmdSkillShow —— `zerg skill show <名> [--json <字段>]`。
//
// 正文来源写死：`agent.NewSkillManager(dir).Load(名)` —— 与 CA 侧 `skill_load` 同一函数
// （`core/internal/agent/loop.go:471-477` → `SkillManager.Load`）。所以这里**不自己读文件**，
// 也不自己拼 frontmatter：一旦这份实现读出的字节与 CA 加载的不同，"show 了但加载是另一份"
// 这类故障就永远查不出来。人面把正文**逐字**打 stdout（说明行走 stderr）⇒ 可与磁盘件 `cmp`。
func cmdSkillShow(inv *invocation, stdout, stderr io.Writer) int {
	root, why := skillRoot()
	if root == "" {
		inv.setErr("blocked", "skill_root_absent", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 8）\n", progName, why)
		return exitBlocked
	}
	es, err := skillScan(root)
	if err != nil {
		inv.setErr("blocked", "skill_root_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了技能根 %s（%v）⇒ 不给结论（退码 8）\n", progName, root, err)
		return exitBlocked
	}
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if name == "" {
		return emitNameMiss(inv, stderr, "skill", "show", "", skillNames(es), "技能")
	}
	entry := (*skillEntry)(nil)
	for i := range es {
		if es[i].Name == name {
			entry = &es[i]
			break
		}
	}
	if entry == nil {
		// 「名给错」= 用法错 2（同族先例 `gap show` 的件头口径：「账读得到、点名点到账外」归用法面）
		return emitNameMiss(inv, stderr, "skill", "show", name, skillNames(es), "技能")
	}
	body, lerr := agent.NewSkillManager(root).Load(name)
	if lerr != nil {
		inv.setErr("blocked", "skill_body_unreadable", lerr.Error())
		fmt.Fprintf(stderr, "%s: %s 的正文读不出来（%v）⇒ 不给结论（退码 8）\n", progName, name, lerr)
		return exitBlocked
	}
	lines := strings.Count(body, "\n") + 1
	rc := exitOK
	if entry.State != skillStateOK {
		// 读得到 ≠ 能加载：照实报 1（kind=failed），✗ 不许把"agent 认不出"报成成功
		inv.setErr("failed", "skill_not_loadable", "state="+entry.State)
		fmt.Fprintf(stderr, "%s: 这一条 state=%s —— agent 侧加载行为与正文不一定一致（照实退码 1）\n", progName, entry.State)
		rc = exitFail
	}
	if inv.jsonGiven {
		if r := requireFields(inv, stderr); r != exitOK {
			return r
		}
		row := map[string]string{
			"name": entry.Name, "description": entry.Desc, "state": entry.State,
			"path": entry.Path, "bytes": fmt.Sprintf("%d", len(body)),
			"lines": fmt.Sprintf("%d", lines), "body": body,
		}
		return selectJSON(stdout, stderr, inv, inv.path, fieldListOf(inv.path), row)
	}
	// 人面：stdout **逐字**打正文（末尾不再补任何字节 —— 与磁盘件 `cmp` 必须零字节差）。
	// 说明行走 stderr ⇒ 可 `zerg skill show x | pbcopy`、可喂给对话。
	fmt.Fprint(stdout, body)
	fmt.Fprintf(stderr, "%s: 技能 %s · %s · %d 字节（正文逐字来自 SKILL.md，与 CA 侧 `skill_load` 同一函数）\n",
		progName, entry.Name, entry.Path, len(body))
	return rc
}
