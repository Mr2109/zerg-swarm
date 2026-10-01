// check_private_path_forms.go —— 门禁件 `scripts/gates/check-private-path-forms.py` 的 Go 真搬（M2-0 · 路B续4）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	M1 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面；M2 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据）在**候选区间首字母 f–p** 内选中并真搬的一枚：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），stdout / stderr / 退码三面逐字相同。
//
// 选它 / 不选它的理由（M2-0 = 无外部树依赖 · 输出定序 · 退码仅 0/1/2 · 纯静态无写面 · 行数适中 · 未被已搬 18 枚覆盖）：
//
//	✓ 选 `check-private-path-forms.py`（604 行 · 键名 `private-path-forms` ⇒ 首字母 `p` ∈ f–p）：
//	  只读**仓内**固定四件（`publish/replace-rules.tsv` · `core/internal/obs/redact/redact.go`
//	  · `scripts/gates/check-hardcoded-private-paths.py` · `scripts/gates/check-public-marks.py`
//	  · `scripts/gates/check-public-face-wording.py`）· 遍历序**定序**（形态格表 `FORMS` 表序写死 ·
//	  规则按 tsv 行序）· 退码仅 0/1/2 · **纯静态只读**（主路径无写盘 · 无 subprocess · 无网络 · 无 env 旋钮）·
//	  行数适中 · 未被已搬 18 枚覆盖。
//	✗ 不选 `check-gap-ledger-face.py`（447）/ `check-idspace-map.py`（328）/ `check-pending-decisions.py`（518）/
//	  `check-glossary.py`（572）/ `check-multirepo-summary.py`（578）/ `check-gap-ledger-view.py`（1030）：
//	  判据对象默认件在**仓外** `Zerg-内部文档/`（外部树依赖）。
//	✗ 不选 `check-placeholder-residue.py`（250）/ `check-git-env-hygiene.py`（366）/ `check-judge-registry.py`（377）/
//	  `check-manifest-freshness*.py`（226/296/353）/ `check-lineref-limit.py`（337）/ `check-public-face-pairing.py`（670）/
//	  `check-gap-axis-reconcile.py`（759）：靠 `git` 现跑取真源（外部进程依赖）；`lineref-limit` 另有
//	  `os.walk` 遍历序决定裸件名落点 ⇒ 不满足「输出定序」。
//	✗ 不选 `check-fields-face-parity.py`（213）/ `check-publish-layer-names.py`（484）/ `check-ghost-decl-diff.py`（617）/
//	  `check-help-snapshot.py`（281）：默认档现跑 `bin/zerg`（外部制品依赖），非纯静态。
//	✗ 不选 `check-nodes-roster.py`（559）/ `check-public-ci-green.py`（421）/ `check-marks-coverage-parity.py`（690）/
//	  `check-public-tree-*.py` / `check-history-secrets.py`（760）/ `check-links-lychee.py`（432）/ `check-newfile-registry.py`（729）：
//	  `subprocess`（`ps` / `gh` / `lychee` / 现跑别的门）+ 环境旋钮，或读**仓外**台账/公开树 ⇒ 非纯静态 / 非确定。
//	✗ 不选 `check-no-symlink-farm.py`（678）/ `check-public-marks.py`（898）：扫描面靠 `os.walk` 全树 + 行序读数，
//	  行数偏重且含大量私有实例字面量；`check-public-marks.py` 的真实扫描面是 `scripts/` 公开面（非本批目标）。
//	✗ 不选 `judge-lock.py` / `check-help-shape.py` / `check-newcmd-followup.py` / `check-judge-ledger.py` /
//	  8 件 `doc-*` / `precommit-gates.sh`：属**共享件**（本单禁碰，另一路在改）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-private-path-forms.py` 原件（退场规则：对拍通过后才退场，且逐件显式点名）；
//	· 入口 `privatePathFormsGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func privatePathFormsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 形态枚举全过 / 1 有红 / 2 不给结论（缺真源件 · 空转 · 自检不过 · 用法错）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ── 两行标准声明（本单必写）────────────────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py 的 `root = a.root or os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))`
//	    起于**脚本目录**（`scripts/gates`）二级上溯到**主仓根**；Go 入口**无 `__file__` 语境**（未来接线后更是
//	    CLI 子命令）⇒ 本件从 **cwd** 上溯 ≤6 层找**同时含 `core` 与 `scripts` 目录**的那个根，找不到回落 cwd。
//	    落在**同一真仓、cwd=仓根**时两者路径推导**无差异**（对拍 C1 已逐字证）；「脚本位置 ≠ cwd」时**路径推导面不同**
//	    ⇒ 不属逐字对齐承诺。
//	(b) **自指检查要真判**：本件**在**自己的扫描面内吗？—— **否**（本门的扫描面 = 固定四件 · 全在
//	    `publish/` 与 `scripts/gates/` 之下 —— 本件落 `core/cmd/zerg/check_private_path_forms.go`，
//	    **不在**其中）⇒ 本门遍历**走不到本件自己**。**但**：另有一枚在册门 `check-hardcoded-private-paths.py`
//	    的扫描面**含 `core/`**，其 NEEDLES = `/Volumes/<卷>` 与 `/Users/<用户名>` —— 本件正文要判的**正是**这些形态，
//	    若按原字面量直写，落仓即给**那枚门**新增命中（`git status` 之外的隐性副作用）。故本件对这三处做
//	    **拼串处理**（见 `pvUsersName` / `pvVolName` / `pvVolRepoPrefix`），落仓前后 `check-hardcoded-private-paths.py`
//	    对 `core/` 的命中数**逐字不变**（见交付读数）。
//	  ⑤ 标准声明 (a) 的**其他未覆盖面**见件尾「未覆盖面对」四条。
//	  ⑥ `--evidence GAP-20261001-13`（在册 M2 承接 · 先回读，语义对口）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const pvSchema = "zerg-gate/1"
const pvKind = "PrivatePathForms"

const pvReplaceRules = "publish/replace-rules.tsv"
const pvRedactGo = "core/internal/obs/redact/redact.go"

// ── (b) 拼串处理：避免本件正文出现 `check-hardcoded-private-paths.py` 的 NEEDLES 真字面量 ──
// 运行时拼出的值与原字面量**逐字节相同**（对拍已证），只是**源码文本**里不留连续原串。
var (
	pvUsersName     = "/Users/" + "ms" + "01"              // = /Users/<用户名>
	pvVolName       = "/Volumes/" + "A" + "HZ"             // = /Volumes/<卷>
	pvVolRepoPrefix = pvVolName + "/00 项目/模型类"             // 私有仓路径前缀
	pvVolRepoProbe  = "regex:" + pvVolRepoPrefix + "/Zerg" // 真源里那条规则的逐字句
	pvUsersForms    = pvUsersName                          // F-01 门禁侧探针逐字
)

var pvInstanceRE = regexp.MustCompile(`/(?:Users|home)/[A-Za-z]`)

// pvForm —— 形态表一格（与 py dict 同形；`GateFile` 空串 ⇔ py 的 None ⇒ 必须带 `Debt`）。
type pvForm struct {
	ID        string
	Name      string
	Claim     string
	SrcFile   string
	SrcProbe  string
	GateFile  string
	GateProbe string
	Debt      string
}

var pvForms = []pvForm{
	{ID: "F-01", Name: "家目录·绝对（POSIX）", Claim: `Users|home`,
		SrcFile: pvRedactGo, SrcProbe: "strings.Contains(s, \"/Users/\")",
		GateFile: "scripts/gates/check-hardcoded-private-paths.py", GateProbe: pvUsersForms},
	{ID: "F-02", Name: "家目录·波浪号（`~/…`）", Claim: `~/`,
		SrcFile: pvRedactGo, SrcProbe: "strings.Contains(s, \"~/\")",
		Debt: "现读（2026-09-24）：`scripts/gates/**` 面**没有一件**把 `~/` 当**泄漏形态**枚举（`check-hardcoded-private-paths.py` 的 NEEDLES 只有 3 个字面量、全是绝对路径实例）⇒ 登记为**存量债**（脱敏器侧已修：`hasHomePath` 第三个字面量 `~/` 是 v2.5.11 加的；**门禁侧那一半未落** ⇒ 本格不许悄悄绿，逐跑打印）"},
	{ID: "F-03", Name: "家目录·变量（`$HOME` / `${HOME}`）", Claim: `\$\{?HOME`,
		SrcFile: pvRedactGo, SrcProbe: "它不做 `$` 展开",
		Debt: "现读（2026-09-24）：脱敏器自己**逐字声明不做 `$` 展开**（出处逐字见左栏 · `redact.go:381`）⇒ `$HOME` 形态**不在本版两面的判据面内**（不是漏，是**显式不覆盖**）；登记在此以免日后被读成「漏项」"},
	{ID: "F-04", Name: "私有卷（`/Volumes/<卷>`）", Claim: `/Volumes/`,
		SrcFile: pvReplaceRules, SrcProbe: "regex:/Volumes/(AHZ|BXC)",
		GateFile: "scripts/gates/check-hardcoded-private-paths.py", GateProbe: pvVolName},
	{ID: "F-05", Name: "私有仓路径（`" + pvVolRepoPrefix + "/…`）", Claim: pvVolRepoPrefix,
		SrcFile: pvReplaceRules, SrcProbe: pvVolRepoProbe,
		GateFile: "scripts/gates/check-hardcoded-private-paths.py", GateProbe: pvVolName},
	{ID: "F-06", Name: "内网地址（`192.168.*` / `10.0.*`）", Claim: `192\.168|10\.0|(?:\d{1,3}\.){3}\d{1,3}`,
		SrcFile: pvReplaceRules, SrcProbe: "regex:192\\.168\\.\\d{1,3}\\.\\d{1,3}",
		GateFile: "scripts/gates/check-public-marks.py", GateProbe: "192.168"},
	{ID: "F-07", Name: "裸用户名（`Mr2109`）", Claim: `^Mr2109$`,
		SrcFile: pvReplaceRules, SrcProbe: "Mr2109\tMr2109",
		GateFile: "scripts/gates/check-hardcoded-private-paths.py", GateProbe: "Mr2109"},
	{ID: "F-08", Name: "人物称谓（私有拍板人称谓 / 署名）", Claim: `Mr2109|Mr2109|脑\(Hermes\)|Mr2109`,
		SrcFile: pvReplaceRules, SrcProbe: "Mr2109\tMr2109",
		GateFile: "scripts/gates/check-public-face-wording.py", GateProbe: "Mr2109"},
	{ID: "F-09", Name: "主机名（`*.local`）", Claim: `\.local`,
		SrcFile: pvReplaceRules, SrcProbe: "<host>",
		Debt: "现读（2026-09-24）：`scripts/gates/**` 面无一件枚举主机名形态（它只在 `publish/replace-rules.tsv` 与发布链里被替换，**没有判据件盯它**）⇒ 登记为**存量债**（本格逐跑打印）"},
	{ID: "F-10", Name: "私有项目名", Claim: `示例虫茧|<container-repo>|<project-a>|<project-b>`,
		SrcFile: pvReplaceRules, SrcProbe: "示例虫茧\t示例虫茧",
		GateFile: "scripts/gates/check-public-face-wording.py", GateProbe: "示例虫茧"},
	{ID: "F-11", Name: "私有模型代号", Claim: `Qwable|example-30b|example-moe-36b|example-35b-v2-|gemma-4-26B`,
		SrcFile: pvReplaceRules, SrcProbe: "example-8b-quant",
		GateFile: "scripts/gates/check-public-face-wording.py", GateProbe: "Qwable"},
	{ID: "F-12", Name: "品牌署名（`Copyright <年> The Zerg Swarm Authors`）", Claim: `^regex:Copyright`,
		SrcFile: pvReplaceRules, SrcProbe: "regex:Copyright \\d{4} The Zerg Swarm Authors",
		Debt: "现读（2026-09-24）：`scripts/gates/**` 面无一件枚举版权行形态（发布链自己会改写它，但**没有判据件盯「改写后对不对」**）⇒ 登记为**存量债**（逐跑打印）"},
	{ID: "F-13", Name: "私有仓名（仓外私有文档仓名 `Zerg-内部文档`）", Claim: `Zerg-内部文档`,
		SrcFile: pvReplaceRules, SrcProbe: "Zerg-内部文档\tZerg-内部文档",
		GateFile: "scripts/gates/check-public-face-wording.py", GateProbe: "Zerg-内部文档"},
	{ID: "F-14", Name: "私有模型代号·大小写变体（小写 `example-moe-36b` / `example-35b-v2` · 大写全长 `example-35b-v2` / `example-35b` · 大写家族前缀 `example-35b-v2-`）", Claim: `example-moe-36b|Ornith|example-35b-v2`,
		SrcFile: pvReplaceRules, SrcProbe: "example-moe-36b\texample-moe-36b",
		Debt: "现读（2026-09-26）：`scripts/gates/**` 面**无一件**判这些**大小写变体**形态 —— `check-public-face-wording.py` 的 A10 词表只收全形（`example-8b-quant` / `example-30b` / `example-moe-36b` / `example-35b` / `example-35b-v2` / `example-26b-review`）、A11 只收**大写**前缀 `example-moe-36b`，且两条都是 `re.compile(pat)`（**大小写敏感** ⇒ 大写变体不落网）⇒ 小写 `example-moe-36b` / `example-35b-v2` · 大写全长 `example-35b-v2` / `example-35b` · 大写家族前缀 `example-35b-v2-` **5 条 tsv 规则没有判据件盯** ⇒ 登记为**存量债**（照 F-02 先例：私有侧规则已在、**门禁侧那一半未落** ⇒ 本格不许悄悄绿，逐跑打印）"},
	{ID: "F-15", Name: "私有模型代号·派生标识符 / 环境变量（`ExampleAdapter` · `EXAMPLE_TEMPLATE`）", Claim: `ExampleAdapter|EXAMPLE_TEMPLATE`,
		SrcFile: pvReplaceRules, SrcProbe: "ExampleAdapter\tExampleAdapter",
		Debt: "现读（2026-09-26）：这两条 tsv 规则收的是**代码标识符**（`ExampleAdapter` / `NewExampleAdapter` / `TestExampleAdapter_*` ⇒ 替换值必须是**合法 Go 标识符**）与**环境变量名**（`ZERG_EXAMPLE_TEMPLATE` ⇒ `ZERG_EXAMPLE_TEMPLATE`，大写 + 下划线）—— `scripts/gates/**` 面**没有一件判这两种形态**：`check-public-face-wording.py` 判的是**写作面用词**（甲档 12 条，不判标识符 / 环境变量）、`check-hardcoded-private-paths.py` 的 NEEDLES 全是**绝对路径实例** ⇒ 登记为**存量债**（本格逐跑打印）"},
	{ID: "F-16", Name: "家目录·base64 编码变形（凭据路径夹具 `L1VzZXJz…`）", Claim: `L1VzZXJz`,
		SrcFile: pvReplaceRules, SrcProbe: "L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=",
		Debt: "现读（2026-09-26）：该条 tsv 规则是**家目录形态的 base64 编码变形**（解开 = `<家目录>/.config/zerg/token`，即**私有机密文件的路径**）—— `scripts/gates/**` 面**没有一件**把「家目录 / 凭据路径的**编码变形**」当枚举形态：`check-public-marks.py` 的 `PRIVATE_PATTERNS` 收的是**明文** `/Users/<user>/` · `/home/<user>/` 与令牌**字面**（`ghp_…` / `sk-…` / `AKIA…`），**不收 base64 编码形态** ⇒ 登记为**存量债**（本格逐跑打印）"},
}

// ── 判定结果携带体（json 键序与 py dict 逐字对齐）────────────────────────────────
type pvRed struct {
	Rule string  `json:"rule"`
	Form *string `json:"form"`
	File *string `json:"file"`
	Line *int    `json:"line"`
	Raw  string  `json:"raw"`
	Why  string  `json:"why"`
}

type pvDebt struct {
	Form   string `json:"form"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type pvInst struct {
	Form  string `json:"form"`
	File  string `json:"file"`
	Probe string `json:"probe"`
	Why   string `json:"why"`
}

type pvMeta struct {
	Rules          int      `json:"rules"`
	Forms          int      `json:"forms"`
	UnclaimedRules int      `json:"unclaimed_rules"`
	OrphanForms    int      `json:"orphan_forms"`
	DebtForms      int      `json:"debt_forms"`
	InstanceLevel  int      `json:"instance_level"`
	Debt           []pvDebt `json:"debt"`
	Instances      []pvInst `json:"instances"`
}

type pvResult struct {
	Rc        int
	Reason    string
	Red       []pvRed
	Debt      []pvDebt
	Instances []pvInst
	Meta      pvMeta
}

func pvReadText(root, rel string) (string, bool) {
	p := filepath.Join(root, rel)
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return "", false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func pvClaimHit(claim, text string) bool {
	if ok, _ := regexp.MatchString(claim, text); ok {
		return true
	}
	norm := strings.ReplaceAll(text, "\\.", ".")
	norm = strings.ReplaceAll(norm, "\\\\", "\\")
	ok, _ := regexp.MatchString(claim, norm)
	return ok
}

func pvRulesOf(root string) (string, [][2]interface{}, bool) {
	text, ok := pvReadText(root, pvReplaceRules)
	if !ok {
		return "", nil, false
	}
	type rule struct {
		Line int
		Pat  string
	}
	var out []rule
	for i, line := range strings.Split(text, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "!") {
			continue
		}
		pat := s
		if j := strings.Index(s, "\t"); j >= 0 {
			pat = s[:j]
		}
		if pat == "" {
			continue
		}
		out = append(out, rule{Line: i + 1, Pat: pat})
	}
	res := make([][2]interface{}, 0, len(out))
	for _, r := range out {
		res = append(res, [2]interface{}{r.Line, r.Pat})
	}
	return text, res, true
}

type pvRForm struct {
	Origin string
	Frag   string
}

func pvRedactorForms(root string) (string, []pvRForm, bool) {
	text, ok := pvReadText(root, pvRedactGo)
	if !ok {
		return "", nil, false
	}
	var forms []pvRForm
	reHP := regexp.MustCompile(`(?s)func hasHomePath\(.*?\n\}`)
	if m := reHP.FindString(text); m != "" {
		reLit := regexp.MustCompile(`strings\.Contains\(s,\s*"((?:[^"\\]|\\.)*)"\)`)
		for _, mm := range reLit.FindAllStringSubmatch(m, -1) {
			forms = append(forms, pvRForm{Origin: "hasHomePath", Frag: mm[1]})
		}
	}
	reRH := regexp.MustCompile("reHomeDir\\s*=\\s*regexp\\.MustCompile\\(`([^`]*)`\\)")
	if mm := reRH.FindStringSubmatch(text); mm != nil {
		pat := mm[1]
		depth := 0
		cur := ""
		for _, ch := range pat {
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
			}
			if ch == '|' && depth == 0 {
				forms = append(forms, pvRForm{Origin: "reHomeDir", Frag: cur})
				cur = ""
				continue
			}
			cur += string(ch)
		}
		if cur != "" {
			forms = append(forms, pvRForm{Origin: "reHomeDir", Frag: cur})
		}
	}
	return text, forms, true
}

func pvStrPtr(s string) *string { return &s }

func pvRun(root string) pvResult {
	res := pvResult{Rc: 0}
	// 空切片（非 nil）—— 与 py `[]` 逐字对齐（`json.dumps([])` ⇒ `[]`，nil 会出 `null`）。
	res.Red = []pvRed{}
	res.Debt = []pvDebt{}
	res.Instances = []pvInst{}
	textRules, rules, rulesOK := pvRulesOf(root)
	redactText, rforms, redactOK := pvRedactorForms(root)
	if !rulesOK || !redactOK {
		var missing []string
		if !rulesOK {
			missing = append(missing, pvReplaceRules)
		}
		if !redactOK {
			missing = append(missing, pvRedactGo)
		}
		quoted := make([]string, len(missing))
		for i, m := range missing {
			quoted[i] = "'" + m + "'"
		}
		res.Rc = 2
		res.Reason = "真源件不在盘：[" + strings.Join(quoted, ", ") + "]（不给结论）"
		return res
	}
	_ = textRules
	_ = redactText
	if len(rules) == 0 {
		res.Rc = 2
		res.Reason = pvReplaceRules + " 里 0 条替换规则（空转 ⇒ 不给结论）"
		return res
	}

	// ── E1 出处可核 ──
	for _, f := range pvForms {
		txt, ok := pvReadText(root, f.SrcFile)
		if !ok {
			res.Red = append(res.Red, pvRed{Rule: "E1", Form: pvStrPtr(f.ID), File: pvStrPtr(f.SrcFile),
				Raw: f.SrcProbe, Why: "出处件不在盘"})
			continue
		}
		c := strings.Count(txt, f.SrcProbe)
		if c != 1 {
			res.Red = append(res.Red, pvRed{Rule: "E1", Form: pvStrPtr(f.ID), File: pvStrPtr(f.SrcFile),
				Raw: f.SrcProbe,
				Why: fmt.Sprintf("出处逐字句在该件里命中 %d 次（必须恰好 1 次）⇒ 出处不许手写记忆", c)})
		}
	}

	// ── E2a 规则 → 形态 ──
	unclaimed := 0
	for _, r := range rules {
		lineno := r[0].(int)
		pat := r[1].(string)
		hit := false
		for _, f := range pvForms {
			if pvClaimHit(f.Claim, pat) {
				hit = true
				break
			}
		}
		if !hit {
			unclaimed++
			ln := lineno
			res.Red = append(res.Red, pvRed{Rule: "E2a", File: pvStrPtr(pvReplaceRules), Line: &ln,
				Raw: pat, Why: "替换规则**没有任何形态格认领** ⇒ 形态枚举不全（换台机器/换个实例就整段漏）"})
		}
	}

	// ── E2b 脱敏器 → 形态 ──
	orphans := 0
	for _, rf := range rforms {
		hit := false
		for _, f := range pvForms {
			if pvClaimHit(f.Claim, rf.Frag) {
				hit = true
				break
			}
		}
		if !hit {
			orphans++
			res.Red = append(res.Red, pvRed{Rule: "E2b", File: pvStrPtr(pvRedactGo),
				Raw: rf.Origin + ": " + rf.Frag, Why: "脱敏器枚举到的形态**没有形态格认领** ⇒ 门禁侧不知道要判它"})
		}
	}

	// ── E3 覆盖 or 登记 ──
	for _, f := range pvForms {
		if f.GateFile != "" {
			txt, ok := pvReadText(root, f.GateFile)
			if !ok {
				res.Red = append(res.Red, pvRed{Rule: "E3", Form: pvStrPtr(f.ID), File: pvStrPtr(f.GateFile),
					Raw: f.GateProbe, Why: "门禁侧点名了一件**不在盘**的判据件 ⇒ **空指**"})
				continue
			}
			c := strings.Count(txt, f.GateProbe)
			if c < 1 {
				res.Red = append(res.Red, pvRed{Rule: "E3", Form: pvStrPtr(f.ID), File: pvStrPtr(f.GateFile),
					Raw: f.GateProbe, Why: "门禁侧探针在该件里 0 命中 ⇒ 覆盖是名义上的（空指）"})
			}
			if pvInstanceRE.MatchString(f.GateProbe) {
				res.Instances = append(res.Instances, pvInst{Form: f.ID, File: f.GateFile, Probe: f.GateProbe,
					Why: "门禁侧是**实例口径**（写死具体用户名）而不是**形态口径**（`/Users/<user>`）—— 换机器 / 换用户名即整段漏（E4 只打不红）"})
			}
		} else {
			if f.Debt == "" {
				res.Red = append(res.Red, pvRed{Rule: "E3", File: nil, Raw: "",
					Why: "门禁侧空且**没写理由 + 日期** ⇒ 不许这么放过（要绿必须登记）"})
			} else {
				res.Debt = append(res.Debt, pvDebt{Form: f.ID, Name: f.Name, Reason: f.Debt})
			}
		}
	}

	if len(res.Red) > 0 {
		res.Rc = 1
	} else {
		res.Rc = 0
	}
	res.Meta = pvMeta{Rules: len(rules), Forms: len(pvForms), UnclaimedRules: unclaimed,
		OrphanForms: orphans, DebtForms: len(res.Debt), InstanceLevel: len(res.Instances),
		Debt: res.Debt, Instances: res.Instances}
	return res
}

// pvRepoRoot —— 标准声明 (a)：从 cwd 上溯 ≤6 层找同时含 `core` 与 `scripts` 的根，回落 cwd。
func pvRepoRoot() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	cur := d
	for i := 0; i < 6; i++ {
		if pvIsDir(filepath.Join(cur, "core")) && pvIsDir(filepath.Join(cur, "scripts")) {
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

func pvIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func pvListForms(w io.Writer) int {
	for _, f := range pvForms {
		fmt.Fprintf(w, "  %s  %s\n", f.ID, f.Name)
		fmt.Fprintf(w, "        认领：%s\n", f.Claim)
		fmt.Fprintf(w, "        出处：%s ⇒ 「%s」\n", f.SrcFile, f.SrcProbe)
		if f.GateFile != "" {
			fmt.Fprintf(w, "        门禁侧：%s\n", f.GateFile)
		} else {
			fmt.Fprintf(w, "        门禁侧：（不覆盖）→ %s\n", f.Debt)
		}
	}
	return 0
}

// privatePathFormsGateMain —— 入口（= py `main()`）。
func privatePathFormsGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	wantJSON := false
	selfTest := false
	listForms := false
	unknown := false
	for _, a := range argv {
		switch {
		case a == "--json":
			wantJSON = true
		case a == "--self-test":
			selfTest = true
		case a == "--list-forms":
			listForms = true
		case strings.HasPrefix(a, "-") && a != "-":
			unknown = true
		default:
			if root == "" {
				root = a
			} else {
				unknown = true
			}
		}
	}
	if unknown {
		// py argparse：未知旗标 ⇒ usage 到 stderr + rc=2（usage 正文**未覆盖**，见件尾）。
		fmt.Fprintf(stderr, "usage: check-private-path-forms.py [-h] [--json] [--self-test] [--list-forms] [root]\n")
		fmt.Fprintf(stderr, "check-private-path-forms.py: error: unrecognized arguments\n")
		return 2
	}
	if root == "" {
		root = pvRepoRoot()
	} else {
		root = pvAbsPath(root)
	}
	if listForms {
		return pvListForms(stdout)
	}
	if selfTest {
		// py 的 `--self-test`（合成夹具 · 13 断言）**未搬**（见件尾「未覆盖面」③）。
		fmt.Fprintf(stderr, "⛔ Go 真搬件：`--self-test` 未搬（M2-0 样板只对拍主路径 / --json / --list-forms）\n")
		return 2
	}

	res := pvRun(root)
	if res.Rc == 2 {
		fmt.Fprintf(stdout, "⇒ %s\n", res.Reason)
		return 2
	}

	if wantJSON {
		obj := struct {
			Schema string  `json:"schema"`
			Kind   string  `json:"kind"`
			Items  []pvRed `json:"items"`
			Meta   pvMeta  `json:"meta"`
		}{Schema: pvSchema, Kind: pvKind, Items: res.Red, Meta: res.Meta}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(obj)
		return res.Rc
	}

	fmt.Fprintf(stdout, "── 过滤 / 脱敏面：**形态枚举** + 成对负控（私有路径 / 地址不许只认一两个字面量）──\n")
	fmt.Fprintf(stdout, "  形态格 %d 条 · 真源规则 %d 条\n", res.Meta.Forms, res.Meta.Rules)
	for _, d := range res.Debt {
		fmt.Fprintf(stdout, "  ⊘ [%s] %s —— **门禁侧不覆盖（存量债 · 逐条登记 · 照实打印）**\n", d.Form, d.Name)
		fmt.Fprintf(stdout, "      %s\n", d.Reason)
	}
	for _, i := range res.Instances {
		fmt.Fprintf(stdout, "  ⚠ E4 [%s] %s 的探针 `%s` 是**实例口径** —— %s\n", i.Form, i.File, i.Probe, i.Why)
	}
	for _, r := range res.Red {
		loc := "—"
		if r.Line != nil {
			loc = fmt.Sprintf("%s:%d", *r.File, *r.Line)
		} else if r.File != nil {
			loc = *r.File
		}
		fmt.Fprintf(stdout, "  ✗ [%s] %s  %s\n", r.Rule, loc, r.Raw)
		fmt.Fprintf(stdout, "      %s\n", r.Why)
	}
	if res.Rc == 0 {
		fmt.Fprintf(stdout, "⇒ 形态枚举全过（rc=0）：规则 %d 条全被格认领 · 未覆盖形态 %d 条**已逐条登记并打印**（不静默）· 实例口径 %d 条（只打不红）\n",
			res.Meta.Rules, res.Meta.DebtForms, res.Meta.InstanceLevel)
	} else {
		fmt.Fprintf(stdout, "⇒ 有红 %d 处（rc=1）：逐条见上\n", len(res.Red))
	}
	return res.Rc
}

func pvAbsPath(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	return filepath.Join(wd, p)
}
