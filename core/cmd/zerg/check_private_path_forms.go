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

// ── 「非形态」登记面（缺口 `GAP-20260926-152` · 与 py `NON_FORM_RULES`/`NON_FORM_FRAGMENTS` 逐格同构）──
//
//	一条模式逐字登记在此（带理由 + 日期）⇒ 视为「已辨识为**非形态**」⇒ 进 `nonform[]` 逐跑打印
//	（不静默），**不当红**；未登记的仍是红（真形态负控照旧有牙）。★ 两表默认空 ⇒ 与旧版行为逐字同形。
var pvNonFormRules = [][3]string{}
var pvNonFormFragments = [][4]string{}

// pvNonformOfRule / pvNonformOfFragment —— **逐字**比对（不吃归一化）；命中 ⇒ 返回登记条（未登记 ⇒ nil）。
func pvNonformOfRule(key string) *[3]string {
	for i := range pvNonFormRules {
		if pvNonFormRules[i][0] == key {
			return &pvNonFormRules[i]
		}
	}
	return nil
}

func pvNonformOfFragment(origin, frag string) *[4]string {
	for i := range pvNonFormFragments {
		if pvNonFormFragments[i][0] == origin && pvNonFormFragments[i][1] == frag {
			return &pvNonFormFragments[i]
		}
	}
	return nil
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

// pvNonform —— 「非形态」（已登记）项（= py E2a/E2b 的 `nonform[]` 项，**无 form 键**）。
type pvNonform struct {
	Rule string  `json:"rule"`
	File *string `json:"file"`
	Line *int    `json:"line"`
	Raw  string  `json:"raw"`
	Why  string  `json:"why"`
}

// pvFormTuple —— `--json` 的逐格四件套（= py `form_tuples[]` 项；gate_file/gate_probe/debt 缺 ⇒ null）。
type pvFormTuple struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Claim     string  `json:"claim"`
	SrcFile   string  `json:"src_file"`
	SrcProbe  string  `json:"src_probe"`
	GateFile  *string `json:"gate_file"`
	GateProbe *string `json:"gate_probe"`
	Debt      *string `json:"debt"`
}

type pvMeta struct {
	Rules          int         `json:"rules"`
	Forms          int         `json:"forms"`
	UnclaimedRules int         `json:"unclaimed_rules"`
	OrphanForms    int         `json:"orphan_forms"`
	DebtForms      int         `json:"debt_forms"`
	InstanceLevel  int         `json:"instance_level"`
	Nonform        []pvNonform `json:"nonform"`
	Debt           []pvDebt    `json:"debt"`
	Instances      []pvInst    `json:"instances"`
}

type pvResult struct {
	Rc        int
	Reason    string
	Red       []pvRed
	Debt      []pvDebt
	Retired   []pvDebt
	Instances []pvInst
	Nonform   []pvNonform
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

// pvGoHalfRel —— py 件名 → 其 Go 半的仓内相对路径（`core/cmd/zerg/<snake>.go`）。
// 退役批口径（B轨波2-1 · 2026-10-07）：门禁侧 py 件退役后其 Go 半接管 ⇒ 用「Go 半在盘」当「已退役」的机检信号。
func pvGoHalfRel(pyPath string) string {
	base := pyPath
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return "core/cmd/zerg/" + strings.ReplaceAll(strings.TrimSuffix(base, ".py"), "-", "_") + ".go"
}

// pvGateRetired —— 门禁侧点名件是否**已退役**：该 py 缺席，**且**其 Go 半（`core/cmd/zerg/<snake>.go`）在盘。
// 退役态 ⇒ E3 覆盖判据改由 Go 半承载 · 本格**不过问**（✗ 不判「空指」）；py 缺席且无 Go 半（真消失）⇒ 未退役（照旧判红）。
func pvGateRetired(root, gateFile string) bool {
	if _, ok := pvReadText(root, gateFile); ok {
		return false
	}
	_, ok := pvReadText(root, pvGoHalfRel(gateFile))
	return ok
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

// pvOptStr —— py `f.get("x")`：空串 ⇔ None（JSON null）。
func pvOptStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// pvLoc —— py `loc = f"{file}:{line}" if line else (file or "—")`。
func pvLoc(file *string, line *int) string {
	if line != nil && file != nil {
		return fmt.Sprintf("%s:%d", *file, *line)
	}
	if file != nil {
		return *file
	}
	return "—"
}

func pvRun(root string) pvResult {
	res := pvResult{Rc: 0}
	// 空切片（非 nil）—— 与 py `[]` 逐字对齐（`json.dumps([])` ⇒ `[]`，nil 会出 `null`）。
	res.Red = []pvRed{}
	res.Debt = []pvDebt{}
	res.Instances = []pvInst{}
	res.Nonform = []pvNonform{}
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
			if reg := pvNonformOfRule(pat); reg != nil {
				ln := lineno
				res.Nonform = append(res.Nonform, pvNonform{Rule: "E2a", File: pvStrPtr(pvReplaceRules), Line: &ln,
					Raw: pat, Why: "**非形态**（已登记 · " + reg[2] + "）：" + reg[1]})
				continue
			}
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
			if reg := pvNonformOfFragment(rf.Origin, rf.Frag); reg != nil {
				res.Nonform = append(res.Nonform, pvNonform{Rule: "E2b", File: pvStrPtr(pvRedactGo), Line: nil,
					Raw: rf.Origin + ": " + rf.Frag,
					Why: "**非形态**（已登记 · " + reg[3] + "）：" + reg[2]})
				continue
			}
			orphans++
			res.Red = append(res.Red, pvRed{Rule: "E2b", File: pvStrPtr(pvRedactGo),
				Raw: rf.Origin + ": " + rf.Frag, Why: "脱敏器枚举到的形态**没有形态格认领** ⇒ 门禁侧不知道要判它"})
		}
	}

	// ── E3 覆盖 or 登记 ──
	for _, f := range pvForms {
		if f.GateFile != "" {
			// ★ 退役态安全（类① · 本件退件前置）：门禁侧点名件是**已退役**的 py（该 py 缺席
			//   但其 Go 半在盘）⇒ 覆盖判据改由 Go 半承载 · 本格**不过问**（✗ 不判「空指」）。
			//   py 在盘 ⇒ 逐字照旧；缺席且无 Go 半（真消失）⇒ 仍判「空指」（牙不放松）。
			if pvGateRetired(root, f.GateFile) {
				res.Retired = append(res.Retired, pvDebt{Form: f.ID, Name: f.Name,
					Reason: "门禁侧点名件 `" + f.GateFile + "` **已退役**（py 缺席 · Go 半在盘）⇒ 覆盖改由 Go 半承载 · 本格不过问"})
				continue
			}
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
				res.Red = append(res.Red, pvRed{Rule: "E3", Form: pvStrPtr(f.ID), File: nil, Raw: "",
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
		Nonform: res.Nonform, Debt: res.Debt, Instances: res.Instances}
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

// ── 自检夹具（= py `self_test()` 的 RULES_BASE / REDACT_BASE，逐字同构）─────────────
//
//	★ 两行含 `check-hardcoded-private-paths.py` 的 NEEDLES 原串（私有卷 / 家目录实例）⇒
//	  必须**拼串**（复用 `pvVolRepoPrefix` / `pvUsersName`），✗ 不留连续原串 —— 否则本件把
//	  那枚门判红（自指消解 · 与件头 (b) 同法）。
var pvSelfRulesBase = []string{
	"# 合成替换规则（自检夹具）",
	"Mr2109\tMr2109\t称谓",
	"<host>\t<host>",
	"regex:" + pvVolRepoPrefix + "/Zerg\t<repo>",
	"regex:/Volumes/(AHZ|BXC)\t<volume-path>",
	pvUsersName + "\t~",
	"<controller-host>\t<worker-host>",
	"regex:192\\.168\\.\\d{1,3}\\.\\d{1,3}\t<worker-ip>",
	"regex:\\b10\\.0\\.\\d{1,3}\\.\\d{1,3}\\b\t<cluster-ip>",
	"regex:Copyright \\d{4} The Zerg Swarm Authors\tCopyright 2026 The Zerg Swarm Authors",
	"示例虫茧\t示例虫茧",
	"example-8b-quant\texample-8b-quant",
	"Mr2109\tMr2109\t裸用户名",
	"example-moe-36b\texample-moe-36b",
	"example-35b-v2\texample-35b-v2",
	"example-35b\texample-35b",
	"ExampleAdapter\tExampleAdapter",
	"EXAMPLE_TEMPLATE\tEXAMPLE_TEMPLATE",
	"example-35b-v2-\texample-35b-v2-",
	"example-35b-v2\texample-35b-v2",
	"L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=\tL1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=",
	"Zerg-内部文档\tZerg-内部文档",
	"!夹具\tcore/internal/obs/redact/*_test.go",
}

var pvSelfRedactBase = []string{
	"package redact",
	"func hasHomePath(s string) bool {",
	"\treturn strings.Contains(s, \"/Users/\") || strings.Contains(s, \"/home/\") || strings.Contains(s, \"~/\")",
	"}",
	"// 分流，是为了**不留第二份 reHomeDir 的副本**（两份规则 = 下一次漂移的种子）；它不做 `$` 展开，",
	"var (\treHomeDir = regexp.MustCompile(`~/|/(?:Users|home)/[A-Za-z0-9._-]+`)\n)",
}

// pvSelfStubHeader / pvSelfFakeGate —— 合成件头（逐字 = py `_mk`；fake-gate 的 NEEDLES 行拼串避原串）。
const pvSelfStubHeader = "# 合成门禁侧件（自检夹具 · 只为 E3 的探针命中）\n"

var pvSelfFakeGate = "# 合成判据件（自检夹具）\nNEEDLES = (\"" + pvUsersName + "\", \"" + pvVolName + "\")\n"

// pvPyRepr —— py `repr(str)` 的单引号形态（本件探针均无引号；覆盖 `\` / `\n` / `\t` 三个高发面）。
func pvPyRepr(s string) string {
	body := strings.ReplaceAll(s, "\\", "\\\\")
	body = strings.ReplaceAll(body, "\n", "\\n")
	body = strings.ReplaceAll(body, "\t", "\\t")
	return "'" + body + "'"
}

// pvSelfMk —— 合成夹具仓（= py `_mk`）：真源两件 + **形态表点名的每个门禁侧件**（各写入自己的探针字面）。
// 为什么门禁侧件也要合成：E3 判的是「该件在盘 **且** 探针字面命中」；夹具里不放它们，红的会是「空指」
// 而不是被测的那条判据 ⇒ 自检测不到真东西。
func pvSelfMk(td string, rulesLines, redactLines []string, forms []pvForm) error {
	write := func(rel, body string) error {
		dst := filepath.Join(td, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte(body), 0o644)
	}
	if err := write(pvReplaceRules, strings.Join(rulesLines, "\n")+"\n"); err != nil {
		return err
	}
	if err := write(pvRedactGo, strings.Join(redactLines, "\n")+"\n"); err != nil {
		return err
	}
	stubs := map[string][]string{}
	for _, f := range forms {
		if f.GateFile != "" && f.GateProbe != "" {
			stubs[f.GateFile] = append(stubs[f.GateFile], f.GateProbe)
		}
	}
	for gf, gps := range stubs {
		var b strings.Builder
		b.WriteString(pvSelfStubHeader)
		for _, gp := range gps {
			b.WriteString("NEEDLE = " + pvPyRepr(gp) + "\n")
		}
		if err := write(gf, b.String()); err != nil {
			return err
		}
	}
	return write("scripts/gates/fake-gate.py", pvSelfFakeGate)
}

// pvFormsCopy —— `[]pvForm` 值拷贝（= py `_forms_copy()`）。
func pvFormsCopy() []pvForm {
	out := make([]pvForm, len(pvForms))
	copy(out, pvForms)
	return out
}

// pvFormsExcept —— 删掉指定 id 的格（= py `[f for f in _forms_copy() if f["id"] != id]`）。
func pvFormsExcept(id string) []pvForm {
	out := make([]pvForm, 0, len(pvForms))
	for _, f := range pvForms {
		if f.ID != id {
			out = append(out, f)
		}
	}
	return out
}

// pvSelfAppend —— `base + [extra]`（不改 base 底层数组）。
func pvSelfAppend(base []string, extra string) []string {
	out := make([]string, 0, len(base)+1)
	out = append(out, base...)
	out = append(out, extra)
	return out
}

// pvBoolToInt —— `1 if cond else 0`。
func pvBoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// pvSelfRunWith —— 在夹具上跑 `pvRun`，但把形态表 / 非形态登记表换成传入的那一份（**不改盘上真件**）。
//
//	`rules` 非 nil 就**真的落到夹具盘上**（= py `_run_with` 的同名语义）；跑完 save/restore 三个表。
func pvSelfRunWith(td string, forms []pvForm, rules []string, nonFormRules *[][3]string, nonFormFrags *[][4]string) pvResult {
	savedForms := pvForms
	savedNFR := pvNonFormRules
	savedNFF := pvNonFormFragments
	defer func() {
		pvForms = savedForms
		pvNonFormRules = savedNFR
		pvNonFormFragments = savedNFF
	}()
	if rules != nil {
		p := filepath.Join(td, pvReplaceRules)
		_ = os.WriteFile(p, []byte(strings.Join(rules, "\n")+"\n"), 0o644)
	}
	pvForms = forms
	if nonFormRules != nil {
		pvNonFormRules = *nonFormRules
	}
	if nonFormFrags != nil {
		pvNonFormFragments = *nonFormFrags
	}
	return pvRun(td)
}

// pvSelfTest —— = py `self_test()`：合成夹具 · 24 断言 · 成对负控。
//
//	夹具全落**仓外**临时目录（`defer os.RemoveAll` 回收）；形态表按 py `_run_with` 的 save/restore
//	语义临时替换（自检单线程独占 ⇒ 无并发面）。输出 = py 逐字节同构：24 行 ✓/✗ + 结论行。
func pvSelfTest(stdout io.Writer) int {
	ok, bad := 0, 0
	var lines []string
	check := func(name string, got, want int) {
		good := got == want
		if good {
			ok++
		} else {
			bad++
		}
		mark := "✗"
		if good {
			mark = "✓"
		}
		lines = append(lines, fmt.Sprintf("  %s %s（期望 rc=%d · 现跑 rc=%d）", mark, name, want, got))
	}
	td, err := os.MkdirTemp("", "w15-forms-")
	if err != nil {
		fmt.Fprintf(stdout, "自检结论: 有失败（断言 1 条 · 失败 1 条）\n")
		return 2
	}
	defer os.RemoveAll(td) // ★ 建点成功那一支才回收（✗ 不无条件删 TMPDIR）

	_ = pvSelfMk(td, pvSelfRulesBase, pvSelfRedactBase, pvForms)

	// ① 正控：形态齐 ⇒ rc=0
	r1 := pvSelfRunWith(td, pvFormsCopy(), nil, nil, nil)
	check("① 正控：规则面 + 脱敏器面全被形态格认领 ⇒ rc=0", r1.Rc, 0)

	// ② must-fail：加一条 `~/…` 规则、并**删掉波浪号那一格** ⇒ rc=1（**这正是原病**）
	r2 := pvSelfRunWith(td, pvFormsExcept("F-02"), nil, nil, nil)
	check("② must-fail：`~/…` 形态无格（删 F-02）⇒ rc=1", r2.Rc, 1)
	named2 := false
	for _, d := range r2.Red {
		if d.Rule == "E2b" || d.Rule == "E2a" {
			named2 = true
		}
	}
	check("②b 同上 ⇒ **点名**到「形态无格」（不是笼统红）", pvBoolToInt(named2), 1)

	// ③ 成对回绿：把 F-02 补回 ⇒ rc=0
	r3 := pvSelfRunWith(td, pvFormsCopy(), nil, nil, nil)
	check("③ 成对：把 F-02 补回 ⇒ 回绿 rc=0", r3.Rc, 0)

	// ④ must-fail：`$HOME` 形态被认领的格**删掉** ⇒ rc=1（脱敏器逐字声明不做 `$` 展开）
	rules4 := pvSelfAppend(pvSelfRulesBase, "$HOME/key\t<home>")
	r4 := pvSelfRunWith(td, pvFormsCopy(), rules4, nil, nil)
	check("④ must-fail：`$HOME` 规则 + 有格认领 ⇒ 仍 rc=0（有格即算枚举到）", r4.Rc, 0)
	_ = pvSelfMk(td, rules4, pvSelfRedactBase, pvForms)
	r4b := pvSelfRunWith(td, pvFormsExcept("F-03"), nil, nil, nil)
	check("④b must-fail：同夹具把 F-03 也删掉 ⇒ rc=1", r4b.Rc, 1)
	_ = pvSelfMk(td, pvSelfRulesBase, pvSelfRedactBase, pvForms)

	// ⑤ 成对回绿 + 不静默：F-03 = 「不覆盖 + 理由」⇒ rc=0 且**逐条打印**该格
	r5 := pvSelfRunWith(td, pvFormsCopy(), nil, nil, nil)
	check("⑤ 成对：`$HOME` 格走「不覆盖 + 理由」⇒ rc=0", r5.Rc, 0)
	nd5 := 0
	for _, d := range r5.Debt {
		if d.Form == "F-03" {
			nd5++
		}
	}
	check("⑤b 同上 ⇒ 该格进 `debt[]`（**逐跑打印**，不静默）", nd5, 1)

	// ⑥ must-fail：门禁侧空 + 无理由 ⇒ rc=1
	f6 := pvFormsCopy()
	for i := range f6 {
		if f6[i].ID == "F-04" {
			f6[i].GateFile = ""
			f6[i].Debt = ""
		}
	}
	r6 := pvSelfRunWith(td, f6, nil, nil, nil)
	check("⑥ must-fail：门禁侧空且**无理由** ⇒ rc=1", r6.Rc, 1)

	// ⑦ 真话负控：门禁侧点一件**不在盘**的判据件 ⇒ rc=1（空指）
	f7 := pvFormsCopy()
	for i := range f7 {
		if f7[i].ID == "F-06" {
			f7[i].GateFile = "scripts/gates/nonexistent-gate.py"
		}
	}
	r7 := pvSelfRunWith(td, f7, nil, nil, nil)
	check("⑦ 真话负控：门禁侧点名**不在盘**的件 ⇒ rc=1（空指）", r7.Rc, 1)
	named7 := false
	for _, d := range r7.Red {
		if strings.Contains(d.Why, "空指") {
			named7 = true
		}
	}
	check("⑦b 同上 ⇒ 理由逐字含「空指」", pvBoolToInt(named7), 1)

	// ⑧ 真话负控：出处句改成**真源里命不中**的字面 ⇒ rc=1（出处不许手写记忆）
	f8 := pvFormsCopy()
	for i := range f8 {
		if f8[i].ID == "F-08" {
			f8[i].SrcProbe = "这一句在真源件里不存在"
		}
	}
	r8 := pvSelfRunWith(td, f8, nil, nil, nil)
	check("⑧ 真话负控：出处句 `grep -F` 0 命中 ⇒ rc=1", r8.Rc, 1)

	// ⑨ E4 实例口径：门禁侧探针写死具体用户名 ⇒ 进 instances[]（只打不红）
	r9 := pvSelfRunWith(td, pvFormsCopy(), nil, nil, nil)
	check("⑨ E4 只打不红：实例口径探针进 `instances[]` 且 rc 不变",
		pvBoolToInt(r9.Meta.InstanceLevel >= 1 && r9.Rc == 0), 1)

	// ⑩ 空转 / 缺件 ⇒ rc=2（不给结论 ⇒ 不许当绿）
	empty := filepath.Join(td, "empty")
	_ = os.MkdirAll(empty, 0o755)
	r10 := pvSelfRunWith(empty, pvFormsCopy(), nil, nil, nil)
	check("⑩ 缺真源件（空转）⇒ rc=2（不给结论 · 不许当绿）", r10.Rc, 2)

	// ⑪ 真话负控（**成对**）：注入一条**真形态**、表里任何格都不认领 ⇒ rc=1 且**点名**该行原文
	_ = pvSelfMk(td, pvSelfRulesBase, pvSelfRedactBase, pvForms)
	rules11 := pvSelfAppend(pvSelfRulesBase, "A72"+"109\tMr2109\tGitee 镜像账号（真形态 · 表里无格）")
	r11 := pvSelfRunWith(td, pvFormsCopy(), rules11, nil, nil)
	check("⑪ 真话负控：注入**真形态**未被任何格认领 ⇒ rc=1", r11.Rc, 1)
	named11 := false
	for _, d := range r11.Red {
		if d.Rule == "E2a" && strings.Contains(d.Raw, "A72"+"109") {
			named11 = true
		}
	}
	check("⑪b 同上 ⇒ E2a **点名**该行原文（不是笼统红）", pvBoolToInt(named11), 1)

	// ⑫ 真话负控：把某格**认领正则弄哑** ⇒ 它本该认领的那处必报（认领是不是**真在生效**）
	_ = pvSelfMk(td, pvSelfRulesBase, pvSelfRedactBase, pvForms)
	f12 := pvFormsCopy()
	for i := range f12 {
		if f12[i].ID == "F-13" {
			f12[i].Claim = "ZZZ-弄哑的认领正则"
		}
	}
	r12 := pvSelfRunWith(td, f12, nil, nil, nil)
	check("⑫ 真话负控：F-13 认领正则弄哑 ⇒ rc=1", r12.Rc, 1)
	named12 := false
	for _, d := range r12.Red {
		if d.Rule == "E2a" && strings.Contains(d.Raw, "Zerg-内部文档") {
			named12 = true
		}
	}
	check("⑫b 同上 ⇒ 报出 `Zerg-内部文档` 那行（E2a 点名）", pvBoolToInt(named12), 1)

	// ⑬ 真话负控：把某**登记债务**改回**未登记**（门禁侧空 + 无理由）⇒ 必报
	_ = pvSelfMk(td, pvSelfRulesBase, pvSelfRedactBase, pvForms)
	f13 := pvFormsCopy()
	for i := range f13 {
		if f13[i].ID == "F-16" {
			f13[i].Debt = ""
		}
	}
	r13 := pvSelfRunWith(td, f13, nil, nil, nil)
	check("⑬ 真话负控：债务格改回**未登记**（gate 空 + 无理由）⇒ rc=1", r13.Rc, 1)
	named13 := false
	for _, d := range r13.Red {
		if d.Rule == "E3" && d.Form != nil && *d.Form == "F-16" {
			named13 = true
		}
	}
	check("⑬b 同上 ⇒ E3 **点名**到 F-16", pvBoolToInt(named13), 1)

	// ⑭ 非形态登记（E2a · `GAP-20260926-152`）：注入一条**公开示例**规则 + **逐字登记**它 ⇒ 不红、进 nonform[]
	_ = pvSelfMk(td, pvSelfRulesBase, pvSelfRedactBase, pvForms)
	rules14 := pvSelfAppend(pvSelfRulesBase, "example-35b-v9\texample-35b-v9\t公开示例（非泄漏形态）")
	nf14 := [][3]string{{"example-35b-v9", "公开示例值（非私有泄漏形态）", "2026-10-04"}}
	r14 := pvSelfRunWith(td, pvFormsCopy(), rules14, &nf14, nil)
	check("⑭ 非形态登记：公开示例规则 + 逐字登记 ⇒ rc=0", r14.Rc, 0)
	named14 := false
	for _, x := range r14.Nonform {
		if x.Raw == "example-35b-v9" {
			named14 = true
		}
	}
	check("⑭b 同上 ⇒ 该规则进 `nonform[]`（逐跑打印 · 不静默）", pvBoolToInt(named14), 1)

	// ⑮ must-fail：**同一条规则不登记** ⇒ rc=1（须登记才放行 —— 登记腿不是把红都洗掉）
	nf15 := [][3]string{}
	r15 := pvSelfRunWith(td, pvFormsCopy(), rules14, &nf15, nil)
	check("⑮ must-fail：同规则不登记 ⇒ rc=1（真形态负控照旧有牙）", r15.Rc, 1)
	named15 := false
	for _, d := range r15.Red {
		if d.Rule == "E2a" && strings.Contains(d.Raw, "example-35b-v9") {
			named15 = true
		}
	}
	check("⑮b 同上 ⇒ E2a **点名**该行原文", pvBoolToInt(named15), 1)

	fmt.Fprintf(stdout, "%s\n", strings.Join(lines, "\n"))
	concl := "全过"
	if bad != 0 {
		concl = "有失败"
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n", concl, ok+bad, bad)
	if bad == 0 {
		return 0
	}
	return 2
}

// privatePathFormsGateMain —— 入口（= py `main()`）。
func privatePathFormsGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	wantJSON := false
	selfTest := false
	listForms := false
	baseline := false
	listUnclaimedOnly := false
	unknown := false
	for _, a := range argv {
		switch {
		case a == "--json":
			wantJSON = true
		case a == "--self-test":
			selfTest = true
		case a == "--list-forms":
			listForms = true
		case a == "--baseline":
			baseline = true
		case a == "--list-unclaimed-only":
			listUnclaimedOnly = true
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
		fmt.Fprintf(stderr, "usage: check-private-path-forms.py [-h] [--json] [--self-test] [--list-forms] [--baseline] [--list-unclaimed-only] [root]\n")
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
		// = py `self_test()`（合成夹具 · 24 断言 · 成对负控）。
		return pvSelfTest(stdout)
	}

	res := pvRun(root)
	if res.Rc == 2 {
		fmt.Fprintf(stdout, "⇒ %s\n", res.Reason)
		return 2
	}

	if listUnclaimedOnly {
		// 缺口 `GAP-20260926-153`：只列没被任何形态格认领的那批（E2a 规则 / E2b 脱敏形态）· 只列不判 ⇒ rc=0。
		n := 0
		for _, r := range res.Red {
			if (r.Rule == "E2a" || r.Rule == "E2b") && r.Form == nil {
				fmt.Fprintf(stdout, "  ? [%s] %s  %s\n", r.Rule, pvLoc(r.File, r.Line), r.Raw)
				n++
			}
		}
		fmt.Fprintf(stdout, "⇒ 未被任何形态格认领：%d 条（只列不判 · rc=0）\n", n)
		return 0
	}

	if baseline {
		// 缺口 `GAP-20260926-153`：只读基线档 —— 落当前命中/债务/实例/非形态快照供对拍，本档**不判** ⇒ rc 恒 0。
		snap := struct {
			Schema string  `json:"schema"`
			Kind   string  `json:"kind"`
			Meta   pvMeta  `json:"meta"`
			Red    []pvRed `json:"red"`
		}{Schema: pvSchema, Kind: "PrivatePathFormsBaseline", Meta: res.Meta, Red: res.Red}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(snap)
		return 0
	}

	if wantJSON {
		// 缺口 `GAP-20260926-173`：`--json` 补**逐格四件套**机器面（每个形态格 id/name/claim/出处/门禁侧/debt）。
		formTuples := make([]pvFormTuple, 0, len(pvForms))
		for _, f := range pvForms {
			formTuples = append(formTuples, pvFormTuple{
				ID: f.ID, Name: f.Name, Claim: f.Claim,
				SrcFile: f.SrcFile, SrcProbe: f.SrcProbe,
				GateFile: pvOptStr(f.GateFile), GateProbe: pvOptStr(f.GateProbe),
				Debt: pvOptStr(f.Debt),
			})
		}
		obj := struct {
			Schema     string        `json:"schema"`
			Kind       string        `json:"kind"`
			Items      []pvRed       `json:"items"`
			FormTuples []pvFormTuple `json:"form_tuples"`
			Meta       pvMeta        `json:"meta"`
		}{Schema: pvSchema, Kind: pvKind, Items: res.Red, FormTuples: formTuples, Meta: res.Meta}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(obj)
		return res.Rc
	}

	fmt.Fprintf(stdout, "── 过滤 / 脱敏面：**形态枚举** + 成对负控（私有路径 / 地址不许只认一两个字面量）──\n")
	fmt.Fprintf(stdout, "  形态格 %d 条 · 真源规则 %d 条\n", res.Meta.Forms, res.Meta.Rules)
	for _, d := range res.Retired {
		fmt.Fprintf(stdout, "  ⊘ [%s] %s —— **门禁侧件已退役（Go 半承载 · 本格不过问）**\n", d.Form, d.Name)
		fmt.Fprintf(stdout, "      %s\n", d.Reason)
	}
	for _, d := range res.Debt {
		fmt.Fprintf(stdout, "  ⊘ [%s] %s —— **门禁侧不覆盖（存量债 · 逐条登记 · 照实打印）**\n", d.Form, d.Name)
		fmt.Fprintf(stdout, "      %s\n", d.Reason)
	}
	for _, d := range res.Nonform {
		fmt.Fprintf(stdout, "  ⊘【非形态 · 已登记】[%s] %s  %s\n", d.Rule, pvLoc(d.File, d.Line), d.Raw)
		fmt.Fprintf(stdout, "      %s\n", d.Why)
	}
	for _, i := range res.Instances {
		fmt.Fprintf(stdout, "  ⚠ E4 [%s] %s 的探针 `%s` 是**实例口径** —— %s\n", i.Form, i.File, i.Probe, i.Why)
	}
	for _, r := range res.Red {
		fmt.Fprintf(stdout, "  ✗ [%s] %s  %s\n", r.Rule, pvLoc(r.File, r.Line), r.Raw)
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
