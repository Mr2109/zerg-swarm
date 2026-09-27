// family_publish_set.go —— 免树只读正门：`zerg publish set <仓内路径>…`
// （「这件会不会发 + 哪一层拦的」）。
//
// 为什么要本枚（病 · 已入账 `GAP-20260927-407`）：`zerg publish tree has` 判「在 / 不在**某棵现成
// 产出树**里」，**必须有树**（无树 ⇒ rc=8/2）；于是「某件**会不会**发」这一问在命令面上**没有免树
// 的正门** —— 上一批门 B9 判据⑥ 报 3 条（表里 `public=no`、两器排除面没收）时，只能「复用发布链
// 解析器复算取件步」间接验，拿不到一个可手敲的口。
//
// 本命令的**真源 = 发布判定链本身**（不新开第二份口径）：
//
//	① 白名单        `publish/whitelist.txt`                    （`check-history-secrets.py:85 load_whitelist`
//	                                                              / `:169 published_path` 逐条同义）
//	② EXCLUDES      `scripts/build/publish-public.sh` 的 `EXCLUDES=( … )`
//	                （`:101 load_excludes` / `:120 excluded` 的**单一真源**；含噪声件 `.bak/.orig/.rej`
//	                 与 `__pycache__` —— 与 `excluded()` 逐条同义）
//	③ DROP_EXACT    `publish/mirror-public-lib.py` 的 `DROP_EXACT = { … }`
//	④ DROP_PREFIX   `publish/mirror-public-lib.py` 的 `DROP_PREFIX = ( … )`
//	⑤ 映射丢弃      `publish/mirror-public-lib.py:519 map_path` 返回 `None`（`publish/` 分支的结构性处置）
//	会发 ⇒ 同上 `map_path` 的返回值就是**公开路径**。
//
// 层序照 `Assets.exclude_kind`（`mirror-public-lib.py:845`）+ 调用点（`:1176` / `:1199`）的现读顺序：
// 白名单 → EXCLUDES → DROP_EXACT → DROP_PREFIX → 映射丢弃（**先拦先判**：报了层名就不再往下走）。
//
// ★ 表数据（白名单 / EXCLUDES / `DROP_*` / `MAP_EXACT`）一律**从真源件现读解析**（本件不抄表）；
//
//	只有 `map_path` 的 6 条结构分支是**逐条镜像**（`:519-538` 逐行对读，非「另立一份判定」）。
//	★ 同一份解析也是 `scripts/gates/check-publish-face-sync.py:115 classify` 用的那把闸
//	（它 `load_module` 进 `mirror-public-lib.py` 取 `map_path`）—— 本命令与它**同源同序**。
//
// 出口三态（与 `net probe` / `publish tree has` 同一条纪律：答案缺身份 ⇒ 宁可不答）：
//
//	`会发` / `不发 + 哪一层拦的` ⇒ rc=0（「不发」也是一条**答案**，逐条带层名 + 口径）
//	**拿不到结论** ⇒ rc=8 且 stdout 一行都不出（判定链真源读不到 / 解析不出 —— 白名单 0 条 /
//	  `EXCLUDES=( … )` 块缺 / `DROP_*` 表解析不到 ⇒ 「按什么算的」讲不出来）
//	用法错 ⇒ rc=2（缺路径 / 路径形状不合格（口径复用 `publishPathBad`）/ `--json` 缺字段 /
//	  `--json` 里缺 `caliber` 身份格）
//
// ★ 未覆盖面（**照实登记，不半落、不扩面**，见回执 §八）：
//
//	(a) 私有面硬门禁层（`publish/private-paths.txt` × `check-public-tree-private.py` 的 `is_allowed`）
//	    未判 —— 真跑走到那一层是**整批中止**（`:1226`），与「进不进产出树」不是同一个问题；
//	(b) 非 blob 层（子模块 gitlink，`:1221` 的 `mode not in TEXT_MODES`）未判 —— 它要读对象模式，
//	    属「树面」读数，与本命令「免树」口径相抵（本命令只读清单类真源件，不读任何树/索引）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// publishSetCaliber —— 这条读数的**口径**（写死一处 · 人面与机器面都读它）。
const publishSetCaliber = "仓内相对路径（仓根为基）· 判定链逐层：白名单 → EXCLUDES → DROP_EXACT → DROP_PREFIX → 映射丢弃"

// publishSetChainCaliber —— 链本身的摆法（哪一件真源决定哪一层）。人面「口径行」第二句用它。
const publishSetChainCaliber = "① publish/whitelist.txt → ② scripts/build/publish-public.sh 的 EXCLUDES=( ) → ③④ publish/mirror-public-lib.py 的 DROP_EXACT / DROP_PREFIX → ⑤ 同件 map_path 映射丢弃"

// publishSetFields —— `--json` 可取的五格（**顺序即人面五列的顺序**）。
//
// ★ 与 `main.go` 里那条登记**同一个值**（登记那一处必须写成 `[]string{…}` 字面量 —— 原因见
// `net probe` / `publish tree has` 那条注释：契约脚本的 `FIELDS_RE` 只认字面量）。
// 两处同值由 `cli_publish_set_test.go` 用现跑对拍钉住。
var publishSetFields = []string{"path", "caliber", "will_publish", "layer", "public_path"}

// publishSetRequired —— 每条读数**必须**带的身份格（缺任一件 ⇒ 退码 2）。
//
// 为什么 `caliber` 不接受被投影掉：`--json <字段>` 是字段投影，而「按哪条链算的」是这条读数的
// **身份** —— 投影掉它，剩下的「会发=否」就又变成了本命令要治的那个病（结论不带口径）。
var publishSetRequired = []string{"caliber"}

// 层名（写死一处；机器面与人面同值）。
const (
	publishSetLayerNone      = "—"
	publishSetLayerWhitelist = "白名单未命中"
	publishSetLayerExcludes  = "EXCLUDES"
	publishSetLayerDropExact = "DROP_EXACT"
	publishSetLayerDropPre   = "DROP_PREFIX"
	publishSetLayerMapDrop   = "映射丢弃"
)

var (
	publishSetExcludesRe  = regexp.MustCompile(`(?s)EXCLUDES=\((.*?)\n\)`)
	publishSetDropExactRe = regexp.MustCompile(`(?s)DROP_EXACT = \{(.*?)\n\}`)
	publishSetDropPreRe   = regexp.MustCompile(`(?s)DROP_PREFIX = \((.*?)\n\)`)
	publishSetMapExactRe  = regexp.MustCompile(`(?s)MAP_EXACT = \{(.*?)\n\}`)
	publishSetMapLineRe   = regexp.MustCompile(`^\s*"([^"]*)"\s*:\s*"([^"]*)"`)
	publishSetQuoteLineRe = regexp.MustCompile(`^\s*"([^"]*)"`)
)

// publishChain —— 判定链的**现读**面（全部来自真源件；一处不抄表）。
type publishChain struct {
	whitelist  []string
	excludes   []string
	dropExact  []string
	dropPrefix []string
	mapExact   map[string]string
}

// publishSetNoiseSuffix / publishSetNoiseDirs —— 与 `check-history-secrets.py:97-98` 逐条同义
// （`excluded()` 的第一条判据：噪声件也归 EXCLUDES 这一层拦）。
var (
	publishSetNoiseSuffix = []string{".bak", ".orig", ".rej"}
	publishSetNoiseDirs   = []string{"__pycache__"}
)

// publishSetReadLines 读一件文本（只读），失败返回错误。
func publishSetReadLines(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(raw), "\n"), nil
}

// publishSetBlockLines 从整件文本里取一段表体的**行**（`re` 必须带一个组）。
func publishSetBlockLines(text string, re *regexp.Regexp) []string {
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return nil
	}
	return strings.Split(m[1], "\n")
}

// publishSetPlainEntries —— 纯清单件（白名单）的行口径（跳空行与 `#` 注释）。
func publishSetPlainEntries(lines []string) []string {
	out := []string{}
	for _, ln := range lines {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		out = append(out, s)
	}
	return out
}

// publishSetShellEntries —— shell 数组（`EXCLUDES=( … )`）的行口径：
// 跳注释与空行 · 切掉行内注释 · 剥首尾引号（与 `load_excludes` 逐条同义）。
func publishSetShellEntries(lines []string) []string {
	out := []string{}
	for _, ln := range lines {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if i := strings.Index(s, "#"); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
		s = strings.Trim(s, `"'`)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// publishSetQuotedEntries —— Python 表（`DROP_EXACT` / `DROP_PREFIX`）的行口径：
// 跳注释与空行 ⇒ 取该行**首个**双引号字面量（表体里一行一条，行内注释与逗号都在字面量之后）。
func publishSetQuotedEntries(lines []string) []string {
	out := []string{}
	for _, ln := range lines {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if m := publishSetQuoteLineRe.FindStringSubmatch(s); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

// publishSetMapEntries —— `MAP_EXACT = { "私有": "公开" }` 的行口径。
func publishSetMapEntries(lines []string) map[string]string {
	out := map[string]string{}
	for _, ln := range lines {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if m := publishSetMapLineRe.FindStringSubmatch(s); m != nil {
			out[m[1]] = m[2]
		}
	}
	return out
}

// publishSetLoadChain 现读判定链的四件真源；`why != ""` ⇒ **不给结论**（退码 8 · 一行都不出）。
//
// 为什么真源缺一件就整条不给结论（而不是「那一层照空表算」）：空表会把「按什么算的」变成
// 一个**比真值更宽**的答案（例如 EXCLUDES 解析不出 ⇒ 每一件都读成「会发」）—— 那正是本命令
// 要治的假读数。同一条纪律见 `check-publish-face-sync.py:268`（EXCLUDES 一条都解析不出来 ⇒ 拒绝给结论）。
func publishSetLoadChain(root string) (*publishChain, string) {
	wlRel := "publish/whitelist.txt"
	shRel := "scripts/build/publish-public.sh"
	libRel := "publish/mirror-public-lib.py"

	wlLines, err := publishSetReadLines(filepath.Join(root, wlRel))
	if err != nil {
		return nil, fmt.Sprintf("白名单真源读不到（%s）", wlRel)
	}
	shLines, err := publishSetReadLines(filepath.Join(root, shRel))
	if err != nil {
		return nil, fmt.Sprintf("EXCLUDES 真源读不到（%s）", shRel)
	}
	libText, err := os.ReadFile(filepath.Join(root, libRel))
	if err != nil {
		return nil, fmt.Sprintf("镜像器真源读不到（%s）", libRel)
	}

	c := &publishChain{}
	c.whitelist = publishSetPlainEntries(wlLines)
	c.excludes = publishSetShellEntries(publishSetBlockLines(strings.Join(shLines, "\n"), publishSetExcludesRe))
	c.dropExact = publishSetQuotedEntries(publishSetBlockLines(string(libText), publishSetDropExactRe))
	c.dropPrefix = publishSetQuotedEntries(publishSetBlockLines(string(libText), publishSetDropPreRe))
	c.mapExact = publishSetMapEntries(publishSetBlockLines(string(libText), publishSetMapExactRe))

	if len(c.whitelist) == 0 {
		return nil, fmt.Sprintf("白名单一条都解析不出来（%s）", wlRel)
	}
	if len(c.excludes) == 0 {
		return nil, fmt.Sprintf("`EXCLUDES=( … )` 一条都解析不出来（%s）", shRel)
	}
	if len(c.dropExact) == 0 || len(c.dropPrefix) == 0 {
		return nil, fmt.Sprintf("`DROP_EXACT` / `DROP_PREFIX` 解析不出来（%s）", libRel)
	}
	if len(c.mapExact) == 0 {
		return nil, fmt.Sprintf("`MAP_EXACT` 解析不出来（%s）", libRel)
	}
	return c, ""
}

// publishSetWhitelisted —— 层 ①（`published_path` 逐条同义：`/` 结尾 = 前缀 · 否则全等）。
func publishSetWhitelisted(rel string, entries []string) bool {
	for _, e := range entries {
		if strings.HasSuffix(e, "/") {
			if strings.HasPrefix(rel, e) {
				return true
			}
		} else if rel == e {
			return true
		}
	}
	return false
}

// publishSetExcluded —— 层 ②（`excluded()` 逐条同义：噪声件 + EXCLUDES 全等或前缀）。
func publishSetExcluded(rel string, excludes []string) bool {
	for _, suf := range publishSetNoiseSuffix {
		if strings.HasSuffix(rel, suf) {
			return true
		}
	}
	for _, part := range strings.Split(rel, "/") {
		for _, dir := range publishSetNoiseDirs {
			if part == dir {
				return true
			}
		}
	}
	for _, e := range excludes {
		if rel == e || strings.HasPrefix(rel, strings.TrimSuffix(e, "/")+"/") {
			return true
		}
	}
	return false
}

// publishSetMapPath —— 层 ⑤（`publish/mirror-public-lib.py:519 map_path` 的逐条镜像；
// 空串 = 该函数返回 `None` = 不进公开面）。表数据走 `MAP_EXACT`（现读解析），
// 结构分支逐行照 `:519-538`（含 `DROP_*` 已在层 ③④ 判过，故此处不再重复）。
func (c *publishChain) publishSetMapPath(rel string) string {
	if mp, ok := c.mapExact[rel]; ok {
		return mp
	}
	if rel == "docs/skills/tool-upgrade.md" {
		return "docs/design/工具升级规范.md"
	}
	if strings.HasPrefix(rel, "publish/docs/design/") {
		return "docs/design/" + filepath.Base(rel)
	}
	if strings.HasPrefix(rel, "publish/docs/") {
		base := filepath.Base(rel)
		if base == "README.md" || base == "README.zh-CN.md" {
			return base
		}
		return "docs/" + base
	}
	if strings.HasPrefix(rel, "publish/ci/") {
		return ".github/workflows/" + filepath.Base(rel)
	}
	if strings.HasPrefix(rel, "publish/") {
		return ""
	}
	return rel
}

// verdict —— 一件的**三态**读数：会发（`是`）/ 不发（`否` + 层名）/ 公开路径（不发 ⇒ `—`）。
func (c *publishChain) verdict(rel string) (will string, layer string, pub string) {
	if !publishSetWhitelisted(rel, c.whitelist) {
		return "否", publishSetLayerWhitelist, publishSetLayerNone
	}
	if publishSetExcluded(rel, c.excludes) {
		return "否", publishSetLayerExcludes, publishSetLayerNone
	}
	for _, e := range c.dropExact {
		if rel == e {
			return "否", publishSetLayerDropExact, publishSetLayerNone
		}
	}
	for _, p := range c.dropPrefix {
		if strings.HasPrefix(rel, p) {
			return "否", publishSetLayerDropPre, publishSetLayerNone
		}
	}
	mp := c.publishSetMapPath(rel)
	if mp == "" {
		return "否", publishSetLayerMapDrop, publishSetLayerNone
	}
	return "是", publishSetLayerNone, mp
}

// cmdPublishSet —— `zerg publish set <仓内路径>…`：免树只读，逐件出「会不会发 + 哪一层拦的」。
func cmdPublishSet(inv *invocation, stdout, stderr io.Writer) int {
	// ── ① `K2` 甲档 + 身份格必查（口径不许被投影掉）──
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		missing := []string{}
		for _, need := range publishSetRequired {
			if !hasFieldName(inv.fields, need) {
				missing = append(missing, need)
			}
		}
		if len(missing) > 0 {
			inv.setErr("usage", "identity_cells_required", "「会不会发」的读数必须带口径")
			fmt.Fprintf(stderr, "%s: 拒（退码 2 · 不给结论）：`--json` 里缺 %s\n", progName, strings.Join(missing, " · "))
			fmt.Fprintf(stderr, "%s: 理由：口径（`caliber`）是这条读数的**身份**（按哪条链算的）"+
				"—— 不是可选投影格 ⇒ 缺它就不出这一条读数\n", progName)
			return exitUsage
		}
	}

	// ── ② 位置参数：**至少一件**（仓内相对路径）──
	if len(inv.args) == 0 {
		inv.setErr("usage", "need_one_path", "要点名至少一个仓内路径")
		fmt.Fprintf(stderr, "%s: 用法：%s publish set <仓内路径>… [--json <字段>]\n", progName, progName)
		fmt.Fprintf(stderr, "%s: 它答的是「这件**会不会**发 + 哪一层拦的」（**免树** · 只读清单类真源件 · 不碰树/索引/盘外）\n",
			progName)
		return exitUsage
	}
	paths := []string{}
	for _, a := range inv.args {
		rel := strings.TrimSpace(a)
		// 路径形状口径**复用** `publishPathBad`（与 `publish tree has` 同一把尺：相对 · `/` 分隔 ·
		// 无前导 `./` · 不许空段 · 不许 `..`）—— 不另立第二份。
		if bad := publishPathBad(rel); bad != "" {
			inv.setErr("usage", "bad_repo_path", bad)
			fmt.Fprintf(stderr, "%s: 路径不合格：%s（口径 = %s）\n", progName, bad, publishSetCaliber)
			return exitUsage
		}
		paths = append(paths, rel)
	}

	// ── ③ 判定链真源现读；缺一件 ⇒ **不给结论**（退码 8 · stdout 一行都不出）──
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 判定链读不到 ⇒ **不给结论**（退码 8）\n", progName)
		return exitBlocked
	}
	chain, why := publishSetLoadChain(root)
	if why != "" {
		inv.setErr("blocked", "chain_source_unreadable", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ **不给结论**（退码 8 · 一行都不出）\n", progName, why)
		fmt.Fprintf(stderr, "%s: 口径：%s\n", progName, publishSetChainCaliber)
		return exitBlocked
	}

	// ── ④ 人面：口径 + 链势 + 逐件一行（「不发」也照出，逐条带层名）──
	fmt.Fprintf(stderr, "%s: 口径 = %s\n", progName, publishSetCaliber)
	fmt.Fprintf(stderr, "%s: 判定链（先拦先判）= %s\n", progName, publishSetChainCaliber)
	fmt.Fprintf(stderr, "%s: 链真源现读：白名单 %d 条 · EXCLUDES %d 条 · DROP_EXACT %d 条 · DROP_PREFIX %d 条 · MAP_EXACT %d 条\n",
		progName, len(chain.whitelist), len(chain.excludes), len(chain.dropExact), len(chain.dropPrefix), len(chain.mapExact))

	rows := []map[string]string{}
	for _, rel := range paths {
		will, layer, pub := chain.verdict(rel)
		fmt.Fprintf(stderr, "%s: 件=%s · 会发=%s · 拦的层=%s · 公开路径=%s（口径=%s）\n",
			progName, rel, will, layer, pub, publishSetCaliber)
		rows = append(rows, map[string]string{
			"path":         rel,
			"caliber":      publishSetCaliber,
			"will_publish": will,
			"layer":        layer,
			"public_path":  pub,
		})
	}
	// 「不发」也是一条答案（与 `publish tree has` 的「不在」同一条口径）：
	// 本命令的 `8` 只留给「讲不出答案」（判定链真源读不到）。
	return listCmd(inv, stdout, stderr, publishSetFields, rows)
}
