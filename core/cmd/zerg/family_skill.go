// family_skill.go —— `zerg skill` 族：技能库的**命令面**（M1 = ls / show 两条只读 · M2 = new / edit / rm 三条 D2 写面）。
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
//	⑤ **写面 D2 三态**（M2 · 同 `doc meta fill` / `script inventory sync`）：`--dry-run` 零副作用 ·
//	   缺 `--yes` ⇒ 2 · **审计先落盘**（写不进审计就不改件）· 写完**回读对拍 sha256**。
//	   三条写面都只动 `<技能根>/<名>/` 这一处；本文件之外的存储根一个都不新增。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ── 字段面 / 显示面 ──────────────────────────────────────────────────────────
//
// ★ `--json` 字段表**不在这里**：`main.go` 的每条 `fields:` 一律写字面量
// `[]string{…}`（契约脚本 `clicFieldsRE` 直读那一行 · `check_cli_contract.go:329`）。
// 本件只放**人面**的列（与 `api ls` 先例同形：显示列 ⊂ 字段面 · `readonly.go:163`）。

var skillLsDisplay = []string{"name", "description", "state"}

// 三条写面的人面显示列（`result` 头一格 = planned / created / updated / removed / unchanged）。
// 与字段面分开、照 `api ls` 先例（显示列 ⊂ 字段面）：`--json` 仍按 main.go 的 `fields:` 字面量投影。
var (
	skillNewDisplay  = []string{"result", "name", "path", "bytes"}
	skillEditDisplay = []string{"result", "name", "mode", "backup_path"}
	skillRmDisplay   = []string{"result", "name", "trash_path"}
)

// skillNameRE —— 技能名（= 目录名）。与 Hermes / Claude Code 的技能目录同规：
// 小写字母数字起头，只许 `-`，1..64 字符（`^[a-z0-9][a-z0-9-]{0,63}$`）。
// **结构性拒绝** `..` 与路径分隔符（不是靠正则兜底 —— `..` 能过 `[a-z0-9.-]`，必须单判一次）。
//
// ★ M1（ls/show）不写面 ⇒ 本枚只在 M2（`skill new|edit|rm` 的名字校验）读 ——
// 口径对齐设计稿 §3.3 的 `^[a-z0-9][a-z0-9-]{0,63}$`（`_` / `.` 一律不收：
// 名同时是目录名与 `skill_load` 的钥匙，收窄只会少几种名字，不会误伤现存货）。
var skillNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

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

// ── M2 · D2 写面（new / edit / rm）────────────────────────────────────────────
//
// 次序照 `doc meta fill`（`family_doc.go:150-230`）：判冲突 → 判根 → 判名 → 计划 → 审计 → 写 → 回读。
// 三条铁律：
//
//	① `--dry-run` ⇒ 只出计划件（`result=planned`）· 不建目录 / 不写件 / 不备份 / 不挪目录 / **不写审计**；
//	② 缺 `--yes` ⇒ 2（`writeGuardYes` 在**第一句**：两旗标同给也在那里判掉，先于任何盘面动作）；
//	③ **审计先落盘**：写不进 `<状态目录>/skill_audit.jsonl` 就不改件（同 `appendDocMetaAudit` 的形态）。

// skillAuditLine —— 审计的一行（一行一事件 · 追加只写 · 落 `<状态目录>/skill_audit.jsonl`）。
//
// ★ sha 两格是**全 64 位**（回读对拍用）；人面/机器面的 `--json` 里给 `shortSHA` 的前 16 位
// （与 `doc meta fill` 的 `before_sha256` / `after_sha256` 列同形）。
type skillAuditLine struct {
	At           string `json:"at"`
	Event        string `json:"event"` // skill_new / skill_edit / skill_rm
	Name         string `json:"name"`
	Path         string `json:"path"`
	Mode         string `json:"mode"` // template / whole / desc / trash
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
	By           string `json:"by"`
	AuditPath    string `json:"audit_path"`
}

// skillAuditPath —— 审计落点（本机状态目录 · 不入仓）。
func skillAuditPath() string { return filepath.Join(stateDirOf(), "skill_audit.jsonl") }

// appendSkillAudit —— 追加一行；**失败即拒**（调用方据此不改件）。
func appendSkillAudit(path string, line skillAuditLine) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(line)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// skillBytesSHA —— 对 []byte 取 sha256（写前算与回读对拍**同一函数** ⇒ 不会出现「算法两样」）。
func skillBytesSHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// skillNameCheck —— 名的结构性校验（**执行前判**，先于任何盘面动作）。
func skillNameCheck(name string) string {
	if strings.TrimSpace(name) == "" {
		return "技能名不许为空"
	}
	if strings.Contains(name, "..") {
		return "技能名不许含 `..`（路径穿越）"
	}
	if strings.ContainsAny(name, `/\`) {
		return "技能名不许含路径分隔符（它同时是目录名）"
	}
	if !skillNameRE.MatchString(name) {
		return "技能名不合规（`^[a-z0-9][a-z0-9-]{0,63}$`：小写字母数字起头 · 不许大写/空格/`_`/`.`）"
	}
	return ""
}

// skillFMName —— 取 frontmatter 的 `name` 那一行（**只为拦「整篇替换时顺手改名」**）。
//
// ★ 为什么这里能读而 `ls`/`show` 零解析：本函数**逐字照** `core/internal/agent/skill_manager.go:74-92`
// 的口径（起头 `---\n` · 到 `\n---\n` · 只认 `name:` 前缀 · `TrimSpace`）—— 与 agent 的容错**同规**，
// 所以不会出现「命令面说名不同、agent 说明相同」这种判据分家。仍不是第二套 YAML 口径：
// 只用一枚键，且**只在一态**（`--from` 整篇）读。M1 不导出 `ParseFrontmatter` 的理由不变
// （边界 `✗ core/internal/**`），本函数把那条偏差收窄成一枚纯函数、可单测。
func skillFMName(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return ""
	}
	end := strings.Index(content, "\n---\n")
	if end == -1 {
		return ""
	}
	name := ""
	for _, line := range strings.Split(content[4:end], "\n") {
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		}
	}
	return name
}

// skillTemplate —— 新技能的骨架（最小但**能直接进系统提示**：`description` 那行就是触发器）。
func skillTemplate(name, desc string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n# " + name + "\n\n" +
		"## 何时用\n\n（写清**什么情况下该加载这条技能** —— 这一行会进模型的系统提示，越具体越好；" +
		"不写等于没有触发器。）\n\n## 步骤\n\n1. …\n\n## 自检\n\n- …（怎么算做完）\n"
}

// skillReplaceDescLine —— 只替换 frontmatter 里的 `description:` 那一行（**正文一字不动**）。
//
// 返回 nil = 现件没有可换的那一行（无 `---` 头 / 没有 `description:`）⇒ 调用方退 2，
// **不猜、不补**（不偷偷在末尾追加一行 —— 那会改形状）。
func skillReplaceDescLine(src []byte, desc string) []byte {
	lines := strings.Split(string(src), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return nil // frontmatter 里没有 description 行
		}
		if strings.HasPrefix(lines[i], "description:") {
			lines[i] = "description: " + desc
			return []byte(strings.Join(lines, "\n"))
		}
	}
	return nil
}

// skillReadFrom —— 读 `--from` 的来源：`-` = stdin（让「整篇替换」不必过 shell 引号），否则读件。
func skillReadFrom(from string) ([]byte, error) {
	if from == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(from)
}

// skillBy —— `--by` 的现读（缺省「（未声明）」· 与 `doc meta fill` 同形）。
func skillBy(inv *invocation) string {
	if by := strings.TrimSpace(inv.flagVal("--by")); by != "" {
		return by
	}
	return "（未声明）"
}

// skillNowStamp —— 备份/回收站的目录戳（UTC · 同设计稿 `<ts>` 形）。
func skillNowStamp() string { return time.Now().UTC().Format("20060102T150405Z") }

// skillTargetMissing —— 三条写面共用的「名不在册」一档：给**最像的合法输入** ＋ 3 候选，退 2。
func skillTargetMissing(inv *invocation, stderr io.Writer, root, action, name string) int {
	es, _ := skillScan(root)
	return emitNameMiss(inv, stderr, "skill", action, name, skillNames(es), "技能")
}

// cmdSkillNew —— `zerg skill new <名> [--desc <串> | --from <件|->] [--dry-run | --yes]`。
func cmdSkillNew(inv *invocation, stdout, stderr io.Writer) int {
	// ★ 调用点粒度：`--dry-run` 与 `--yes` 同给必须在**任何盘面动作之前**判掉
	//   （`guard.go:211-230` 的 `writeGuardYes` 已含这一判）
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if why := skillNameCheck(name); why != "" {
		inv.setErr("usage", "bad_name", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 退码 2\n", progName, why)
		fmt.Fprintf(stderr, "形态：%s skill new <名> (--desc <一句话触发器> | --from <件|->) [--dry-run | --yes]\n", progName)
		return exitUsage
	}
	desc := strings.TrimSpace(inv.desc)
	from := strings.TrimSpace(inv.flagVal("--from"))
	if desc != "" && from != "" {
		inv.setErr("usage", "source_conflict", "`--desc` 与 `--from` 互斥")
		fmt.Fprintf(stderr, "%s: `--desc`（骨架＋触发器）与 `--from`（整篇来源）**只能给一个** ⇒ 退码 2\n", progName)
		return exitUsage
	}
	if desc == "" && from == "" {
		inv.setErr("usage", "desc_required", "缺 --desc")
		fmt.Fprintf(stderr, "%s: 要给 `--desc <一句话触发器>`（它就是 `skill_load` 清单里那一行）或 `--from <件|->` ⇒ 退码 2\n", progName)
		return exitUsage
	}
	if strings.ContainsAny(desc, "\r\n") {
		inv.setErr("usage", "desc_multiline", "描述必须单行")
		fmt.Fprintf(stderr, "%s: `--desc` 必须单行（frontmatter 里是一行 YAML · 换行会截断）⇒ 退码 2\n", progName)
		return exitUsage
	}
	root, why := skillRoot()
	if root == "" {
		inv.setErr("blocked", "skill_root_absent", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不写（退码 8）\n", progName, why)
		return exitBlocked
	}
	target := filepath.Join(root, name)
	if _, err := os.Stat(target); err == nil {
		inv.setErr("conflict", "skill_exists", "同名技能已存在："+name)
		fmt.Fprintf(stderr, "%s: 同名技能已存在：%s ⇒ **不覆盖**（退码 14）；要改现有那条走 `%s skill edit %s`\n",
			progName, target, progName, name)
		return exitConflict
	} else if !os.IsNotExist(err) {
		inv.setErr("blocked", "skill_target_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 判不了 %s 在不在（%v）⇒ 不写（退码 8）\n", progName, target, err)
		return exitBlocked
	}
	mode, body := "template", []byte(nil)
	if from != "" {
		src, rerr := skillReadFrom(from)
		if rerr != nil {
			inv.setErr("usage", "source_unreadable", rerr.Error())
			fmt.Fprintf(stderr, "%s: 读不到 `--from` 的来源（%v）⇒ 退码 2\n", progName, rerr)
			return exitUsage
		}
		if len(src) == 0 {
			inv.setErr("usage", "source_empty", "`--from` 的内容是空的")
			fmt.Fprintf(stderr, "%s: `--from` 的内容是空的 ⇒ 退码 2（空技能等于没建）\n", progName)
			return exitUsage
		}
		if fm := skillFMName(string(src)); fm != name {
			inv.setErr("usage", "name_mismatch", "`--from` 的 frontmatter name 与 <名> 不同")
			fmt.Fprintf(stderr, "%s: `--from` 里 name=%q ≠ %q ⇒ 退码 2（名同时是目录名，`new` 不改名）\n", progName, fm, name)
			return exitUsage
		}
		mode, body = "whole", src
	} else {
		body = []byte(skillTemplate(name, desc))
	}
	after := skillBytesSHA(body)
	doc := filepath.Join(target, "SKILL.md")
	auditPath := skillAuditPath()
	row := map[string]string{
		"result": "", "name": name, "path": doc,
		"before_sha256": "（新建：无前件）", "after_sha256": shortSHA(after),
		"bytes": fmt.Sprintf("%d", len(body)), "audit_path": auditPath,
	}
	if !inv.yes {
		// 走到这里还没 --yes ⇒ `writeGuardYes` 已经退过 2；本支只为 `--dry-run` 存在。
		row["result"] = "planned"
		fmt.Fprintf(stderr, "%s: 干跑 —— 将建 %s（%d 字节 · mode=%s · sha256 %s）· **零写**\n",
			progName, doc, len(body), mode, shortSHA(after))
		return listCmd(inv, stdout, stderr, skillNewDisplay, []map[string]string{row})
	}
	line := skillAuditLine{At: time.Now().Format(time.RFC3339), Event: "skill_new", Name: name,
		Path: doc, Mode: mode, BeforeSHA256: "", AfterSHA256: after, By: skillBy(inv), AuditPath: auditPath}
	if err := appendSkillAudit(auditPath, line); err != nil {
		inv.setErr("blocked", "audit_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: **审计落不下盘 ⇒ 拒执**：%v（退码 8）\n", progName, err)
		return exitBlocked
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		inv.setErr("blocked", "skill_dir_create_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 建不出 %s：%v（退码 8）\n", progName, target, err)
		return exitBlocked
	}
	if err := os.WriteFile(doc, body, 0o644); err != nil {
		inv.setErr("failed", "skill_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不进 %s：%v（审计已留痕：%s）\n", progName, doc, err, auditPath)
		return exitFail
	}
	// 回读对拍（同 `family_doc.go:204-209`）：不一致 = 报失败，不报成功
	if back, rerr := os.ReadFile(doc); rerr != nil || skillBytesSHA(back) != after {
		inv.setErr("failed", "writeback_mismatch", "回读与写前算出的 sha256 不一致")
		fmt.Fprintf(stderr, "%s: **回读对拍不一致**（%s）⇒ 报失败、不报成功\n", progName, doc)
		return exitFail
	}
	row["result"] = "created"
	fmt.Fprintf(stderr, "%s: 已建 %s（%d 字节）· 审计 %s\n", progName, doc, len(body), auditPath)
	return listCmd(inv, stdout, stderr, skillNewDisplay, []map[string]string{row})
}

// cmdSkillEdit —— `zerg skill edit <名> (--desc <新触发器> | --from <件|->) [--dry-run | --yes]`。
//
// 本版**无 patch**：要么只换 frontmatter 的 `description:` 一行（`--desc` · 正文一字不动），
// 要么整篇覆盖（`--from` · `-` = stdin）。写前备份到 `<状态目录>/skill-backup/<ts>/<名>/SKILL.md`。
func cmdSkillEdit(inv *invocation, stdout, stderr io.Writer) int {
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if why := skillNameCheck(name); why != "" {
		inv.setErr("usage", "bad_name", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 退码 2\n", progName, why)
		fmt.Fprintf(stderr, "形态：%s skill edit <名> (--desc <新触发器> | --from <件|->) [--dry-run | --yes]\n", progName)
		return exitUsage
	}
	// 改法二选一（互斥 · 同给 ⇒ 2 · 都不给 ⇒ 2）—— 执行前判
	desc := strings.TrimSpace(inv.desc)
	from := strings.TrimSpace(inv.flagVal("--from"))
	if (desc == "") == (from == "") {
		inv.setErr("usage", "edit_mode_required", "`--desc` 与 `--from` 必须恰好给一个")
		fmt.Fprintf(stderr, "%s: 给且只给一种改法：`--desc <新触发器>`（只改描述行）或 `--from <件|->`（整篇）⇒ 退码 2\n", progName)
		return exitUsage
	}
	if strings.ContainsAny(desc, "\r\n") {
		inv.setErr("usage", "desc_multiline", "描述必须单行")
		fmt.Fprintf(stderr, "%s: `--desc` 必须单行（frontmatter 里是一行 YAML · 换行会截断）⇒ 退码 2\n", progName)
		return exitUsage
	}
	root, why := skillRoot()
	if root == "" {
		inv.setErr("blocked", "skill_root_absent", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不写（退码 8）\n", progName, why)
		return exitBlocked
	}
	target := filepath.Join(root, name)
	fi, serr := os.Stat(target)
	if serr != nil {
		if os.IsNotExist(serr) {
			return skillTargetMissing(inv, stderr, root, "edit", name)
		}
		inv.setErr("blocked", "skill_target_unreadable", serr.Error())
		fmt.Fprintf(stderr, "%s: 判不了 %s 在不在（%v）⇒ 不写（退码 8）\n", progName, target, serr)
		return exitBlocked
	}
	if !fi.IsDir() {
		inv.setErr("usage", "skill_not_a_dir", "根下散件不是技能目录："+name)
		fmt.Fprintf(stderr, "%s: %s 是根下的散件、不是技能目录 ⇒ 改不了（退码 2；要新建走 `%s skill new`）\n", progName, target, progName)
		return exitUsage
	}
	doc := filepath.Join(target, "SKILL.md")
	before, rerr := os.ReadFile(doc)
	if rerr != nil {
		inv.setErr("blocked", "skill_body_unreadable", rerr.Error())
		fmt.Fprintf(stderr, "%s: %s 的现件读不出来（%v）⇒ 不改（退码 8）\n", progName, doc, rerr)
		return exitBlocked
	}
	mode, after := "desc", []byte(nil)
	if from != "" {
		mode = "whole"
		src, ferr := skillReadFrom(from)
		if ferr != nil {
			inv.setErr("usage", "source_unreadable", ferr.Error())
			fmt.Fprintf(stderr, "%s: 读不到 `--from` 的来源（%v）⇒ 退码 2\n", progName, ferr)
			return exitUsage
		}
		if len(src) == 0 {
			inv.setErr("usage", "source_empty", "`--from` 的内容是空的")
			fmt.Fprintf(stderr, "%s: `--from` 的内容是空的 ⇒ 退码 2（空技能等于删了正文）\n", progName)
			return exitUsage
		}
		// 整篇替换的硬判据：新件 frontmatter 的 name 必须**逐字等于** <名>（否则等于改名换件）
		if fm := skillFMName(string(src)); fm != name {
			inv.setErr("usage", "name_mismatch", "`--from` 的 frontmatter name 与 <名> 不同")
			fmt.Fprintf(stderr, "%s: `--from` 里 name=%q ≠ %q ⇒ 退码 2（改名不归 `edit` 管）\n", progName, fm, name)
			return exitUsage
		}
		after = src
	} else {
		after = skillReplaceDescLine(before, desc) // 只替换 `description:` 那一行，正文一字不动
		if after == nil {
			inv.setErr("usage", "no_frontmatter", "现件没有 frontmatter，改不了描述行")
			fmt.Fprintf(stderr, "%s: 现件没有 `---` frontmatter（或没有 `description:` 行）⇒ 改不了描述行（走整篇 `--from`）⇒ 退码 2\n", progName)
			return exitUsage
		}
	}
	auditPath := skillAuditPath()
	backup := filepath.Join(stateDirOf(), "skill-backup", skillNowStamp(), name, "SKILL.md")
	beforeSHA, afterSHA := skillBytesSHA(before), skillBytesSHA(after)
	row := map[string]string{
		"result": "", "name": name, "path": doc, "mode": mode,
		"before_sha256": shortSHA(beforeSHA), "after_sha256": shortSHA(afterSHA),
		"bytes": fmt.Sprintf("%d", len(after)), "backup_path": backup, "audit_path": auditPath,
	}
	if string(before) == string(after) {
		// 幂等（§十二 P-031）：一字不改 ⇒ 不备份、不写审计、不碰件
		row["result"] = "unchanged"
		fmt.Fprintf(stderr, "%s: 现件与新内容**逐字节相同** ⇒ 一个字节都不写（result=unchanged）\n", progName)
		return listCmd(inv, stdout, stderr, skillEditDisplay, []map[string]string{row})
	}
	if !inv.yes {
		row["result"] = "planned"
		fmt.Fprintf(stderr, "%s: 干跑 —— 将整篇改 %s（mode=%s · %d → %d 字节 · 备份 %s）· **零写**\n",
			progName, doc, mode, len(before), len(after), backup)
		return listCmd(inv, stdout, stderr, skillEditDisplay, []map[string]string{row})
	}
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		inv.setErr("blocked", "skill_backup_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 备份目录建不出 %s：%v ⇒ 不改件（退码 8）\n", progName, filepath.Dir(backup), err)
		return exitBlocked
	}
	if err := os.WriteFile(backup, before, 0o600); err != nil {
		inv.setErr("blocked", "skill_backup_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不出备份 %s：%v ⇒ 不改件（退码 8）\n", progName, backup, err)
		return exitBlocked
	}
	if back, berr := os.ReadFile(backup); berr != nil || skillBytesSHA(back) != beforeSHA {
		inv.setErr("blocked", "skill_backup_failed", "备份回读与改前字节不一致")
		fmt.Fprintf(stderr, "%s: **备份回读不一致**（%s）⇒ 不改件（退码 8）\n", progName, backup)
		return exitBlocked
	}
	line := skillAuditLine{At: time.Now().Format(time.RFC3339), Event: "skill_edit", Name: name,
		Path: doc, Mode: mode, BeforeSHA256: beforeSHA, AfterSHA256: afterSHA, By: skillBy(inv), AuditPath: auditPath}
	if err := appendSkillAudit(auditPath, line); err != nil {
		inv.setErr("blocked", "audit_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: **审计落不下盘 ⇒ 拒执**：%v（退码 8）\n", progName, err)
		return exitBlocked
	}
	if err := os.WriteFile(doc, after, 0o644); err != nil {
		inv.setErr("failed", "skill_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不进 %s：%v（审计已留痕：%s · 备份 %s）\n", progName, doc, err, auditPath, backup)
		return exitFail
	}
	if back, verr := os.ReadFile(doc); verr != nil || skillBytesSHA(back) != afterSHA {
		inv.setErr("failed", "writeback_mismatch", "回读与写前算出的 sha256 不一致")
		fmt.Fprintf(stderr, "%s: **回读对拍不一致**（%s）⇒ 报失败、不报成功（备份在 %s）\n", progName, doc, backup)
		return exitFail
	}
	row["result"] = "updated"
	fmt.Fprintf(stderr, "%s: 已改 %s（mode=%s · %d → %d 字节）· 备份 %s · 审计 %s\n",
		progName, doc, mode, len(before), len(after), backup, auditPath)
	return listCmd(inv, stdout, stderr, skillEditDisplay, []map[string]string{row})
}

// cmdSkillRm —— `zerg skill rm <名> [--dry-run | --yes]`。
//
// **不硬删**：整目录 `os.Rename` 进 `<状态目录>/skill-trash/<ts>/<名>/`（同卷即时生效 · 可逆）。
// 跨卷（EXDEV）⇒ 照实报 1，**不假装挪过去了**（也不退化成 `RemoveAll` —— 那样就不可逆了）。
func cmdSkillRm(inv *invocation, stdout, stderr io.Writer) int {
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if why := skillNameCheck(name); why != "" {
		inv.setErr("usage", "bad_name", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 退码 2\n", progName, why)
		fmt.Fprintf(stderr, "形态：%s skill rm <名> [--dry-run | --yes]\n", progName)
		return exitUsage
	}
	root, why := skillRoot()
	if root == "" {
		inv.setErr("blocked", "skill_root_absent", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不写（退码 8）\n", progName, why)
		return exitBlocked
	}
	target := filepath.Join(root, name)
	fi, serr := os.Stat(target)
	if serr != nil {
		if os.IsNotExist(serr) {
			return skillTargetMissing(inv, stderr, root, "rm", name)
		}
		inv.setErr("blocked", "skill_target_unreadable", serr.Error())
		fmt.Fprintf(stderr, "%s: 判不了 %s 在不在（%v）⇒ 不写（退码 8）\n", progName, target, serr)
		return exitBlocked
	}
	if !fi.IsDir() {
		inv.setErr("usage", "skill_not_a_dir", "根下散件不是技能目录："+name)
		fmt.Fprintf(stderr, "%s: %s 是根下的散件、不是技能目录 ⇒ 下架不了（退码 2）\n", progName, target)
		return exitUsage
	}
	doc := filepath.Join(target, "SKILL.md")
	beforeSHA := ""
	if b, berr := os.ReadFile(doc); berr == nil {
		beforeSHA = skillBytesSHA(b)
	}
	trash := filepath.Join(stateDirOf(), "skill-trash", skillNowStamp(), name)
	auditPath := skillAuditPath()
	row := map[string]string{
		"result": "", "name": name, "path": target,
		"trash_path": trash, "audit_path": auditPath,
	}
	if !inv.yes {
		row["result"] = "planned"
		fmt.Fprintf(stderr, "%s: 干跑 —— 将把 %s 整目录挪进 %s · **零写**（不挪、不删）\n", progName, target, trash)
		return listCmd(inv, stdout, stderr, skillRmDisplay, []map[string]string{row})
	}
	line := skillAuditLine{At: time.Now().Format(time.RFC3339), Event: "skill_rm", Name: name,
		Path: target, Mode: "trash", BeforeSHA256: beforeSHA, AfterSHA256: "", By: skillBy(inv), AuditPath: auditPath}
	if err := appendSkillAudit(auditPath, line); err != nil {
		inv.setErr("blocked", "audit_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: **审计落不下盘 ⇒ 拒执**：%v（退码 8）\n", progName, err)
		return exitBlocked
	}
	if err := os.MkdirAll(filepath.Dir(trash), 0o700); err != nil {
		inv.setErr("blocked", "skill_trash_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 回收站目录建不出 %s：%v ⇒ 不动件（退码 8）\n", progName, filepath.Dir(trash), err)
		return exitBlocked
	}
	if err := os.Rename(target, trash); err != nil {
		inv.setErr("failed", "skill_trash_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 挪不进回收站（%v）⇒ 照实报失败（退码 1 · 跨卷时请人工搬；审计已留痕 %s）\n", progName, err, auditPath)
		return exitFail
	}
	if ti, terr := os.Stat(trash); terr != nil || !ti.IsDir() {
		inv.setErr("failed", "skill_trash_failed", "回收站里读不到刚挪过去的那一件")
		fmt.Fprintf(stderr, "%s: **挪完回读不到** %s ⇒ 报失败、不报成功\n", progName, trash)
		return exitFail
	}
	row["result"] = "removed"
	fmt.Fprintf(stderr, "%s: 已下架 %s（整目录在 %s · 可逆：挪回去即可）· 审计 %s\n", progName, target, trash, auditPath)
	return listCmd(inv, stdout, stderr, skillRmDisplay, []map[string]string{row})
}
