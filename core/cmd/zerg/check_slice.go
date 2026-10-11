// check_slice.go —— py 门禁件 `scripts/gates/check-slice.py`（2206 行）的 **Go 真搬**（M2 剩余 A 类最后一枚）。
//
// 入口：checkSliceGateMain(stdout, stderr io.Writer, argv []string) int
//
//	· argv 与 py `sys.argv[1:]` 同形（不含程序名）。
//	· 退码 0/1/2 三档齐（0 全合法 / 1 有非法或探针门槛不过 / 2 用法错·前置缺件·strict 违约·自检不过）。
//
// 出厂声明 (a) **默认根定位差异**：py 用 `here=dirname(abspath(__file__))`、`repo_root=join(here,'..','..')`
//
//	自脚本位置推导；Go 入口无 `__file__` 语境 ⇒ 默认取**当前工作目录**（`csDefaultRepoRoot`）。派生 `here`
//	一律 = `join(repo_root,'scripts','gates')`（与 py 的同源关系逐字一致：对拍时把 py 副本放在
//	`<夹具根>/scripts/gates/check-slice.py`）。「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 不声称逐字对齐；
//	对拍一律**显式传 `--repo-root`**（乙环境另令 py 副本落 `scripts/gates`）。
//	同源第二处：py 的 `ZERG_EGG_PROFILE_DIR` env 决定标定档案目录（同名 env，未改）。
//	新口径旋钮面差：py **无** `--repo-root` 旗标（自 `__file__` 推）⇒ 本入口新增 `--repo-root` 显式入参；
//	缺省 = cwd ⇒ 列**有意旋钮面差**。
//
// 出厂声明 (b) **自指检查**：本件不落在它自己扫描的面内（本件不遍历仓树、无私有绝对路径字面量、无
//
//	`setErr(` / `layer:` 形态）。前瞻消解见件尾注。
//
// 遍历/排序点：本件**无目录遍历**（无 os.walk / 无 ReadDir 下钻）—— 取数全是定点读（片文件 / 探针集 /
//
//	标定档案 / landing 落点文件）。=> 标准第⑩条「readdir 原序 + 先文件后下钻」**不适用**（无遍历面）。
//
// 正则面（★ 逐处现读 · 复刻 py 的**实际**行为而非意图）：py 的四处关键正则源码里是**双反斜杠**
//
//	（`r'\\.'` / `r'\\b...'`），现读实跑坐实：`FULLSCAN_RE` 只吃「全量」、`PATHY_EXT_RE` 对真实扩展名
//	零命中、`EXTERNAL_REF_RE` 对 `../x` 零命中 —— 本入口用**同一 pattern 串**编译（Go RE2 vs py re：
//	此四处无回看/无反向引用，语义等价）⇒ 逐字复刻其（含「死」）行为。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type ruleRow struct{ Num, Key, State, Text, Cost, Impl string }

var pyRuleTable = []ruleRow{
	{"R0", "one_thing_shape", "启用", "正面句式：`the_one_thing` 必须能套进「When/While ⟨条件⟩，⟨落点⟩ 必须 ⟨单一动作⟩」且只含一个动词", "低", "句式正则 + 「必须」之后动作段的动词计数（词表登记见 REG_VERBS）"},
	{"R1", "single_defect", "启用", "一片只绑**一个** `defect_id`", "低", "字符串含分隔符后切成 >1 个 id ⇒ 红；列表长度 >1 ⇒ 红"},
	{"R2", "landing_path_exists", "启用", "`landing` 含路径样式且**当前存在**（前状态检查）", "低", "路径形态：含 `/` 或命中登记扩展名；存在性按**仓根**解析（绝对路径按原样）"},
	{"R3", "criteria_triplet", "启用", "`criteria` 三件套齐全（命令+期望输出+当前红/绿）", "低", "每条须有非空 `cmd` / `expect` / `state`，且 `state` 命中红或绿 ⇒ 缺一判红"},
	{"R4", "unbounded_words", "启用", "无界词三层：含糊词 / 可选词（can·may·可选）/ 主观词；**命中必须附\"改写成什么\"** ✗ 否则不算有效报红", "低", "三层词表（REG_R4）+ 强制改写建议：命中字典给出 fix；给不出 fix 的命中 ⇒ 拒判 rc=2"},
	{"R5", "no_empty_shell", "启用", "`hard_limits` / `side_effects` / `output_format` / `exit_cond` 非空且非占位（空壳检测：不得等于模板占位文本、不得含未替换的 `[]`/`<>`；未决必须写成 `NEEDS-CLARIFICATION`）", "低", "四字段逐个查：空壳 / 占位词（REG_PLACEHOLDER）/ 未替换括号；`NEEDS-CLARIFICATION` 合法"},
	{"R6", "gate_cmds_expect", "启用", "`gate_cmds` 可解析且**每条带 expect**", "低", "每条须为 {cmd, expect}；expect 取值域登记（REG_EXPECT_KEYS）；exit-only 的假绿命令形态（REG_EXIT_TRAP）判红"},
	{"R7", "granularity_line_bound", "路由", "粒度：默认**单文件** + 行数上界（上界数值 = 本仓标定取值（待实测复核））；**超限判\"需递归\"，而不是打回** ✗", "低", "落点文件数 >1 ⇒ 需递归；行数上界取标定档案（缺 ⇒ 记「未标定」，不猜、不判红）"},
	{"R8", "slice_set_dag", "启用", "片集合：`depends_on` 可**拓扑排序且无环**；同一 `landing`+`defect_id` 不重复", "低", "自依赖直接判红；≥2 片时 Kahn 拓扑排序查环；同 (landing, defect_id) 重复判红"},
	{"R9", "set_coverage_and_closure", "部分", "片集合：该缺陷的**每条验收判据至少被一片承接**（未覆盖清单随打回附上）+ **集合停机判据**（全部 open defect 被覆盖、依赖闭包闭合）", "中", "已实现：依赖闭包闭合（集合级）。未实现：验收判据覆盖清单 + 停机判据（缺输入 schema，登记见 --list-rules）"},
	{"R10", "self_contained", "部分", "**自包含性**：片带落点原文片段 + 判据命令 + 执行者可见的最小上下文；**无设计稿也能复现**", "中", "已实现（句法侧）：context_needed 非空 + 含落点原文片段 + 不得把设计稿当上下文。未实现：只给片实跑一次（非纯句法）"},
	{"R11", "context_budget_and_order", "启用", "`context_budget`（执行者所需文件/token 上界）；landing 片段与 criteria **固定放最前**（长上下文 U 型曲线：arXiv 2307.03172《Lost in the Middle》〔会议版：TACL 2024〕）", "低", "三断言：context_budget 在且有数值上界与出处；context_needed 首条 = 落点原文片段；criteria 键序在其他 post/invariant 字段之前"},
	{"R12", "exit_cond_structured", "启用", "`exit_cond` 结构化三项：成功（引用 criteria id）/ 失败（可观察条件）/ 升级（对象+触发次数+记录字段）", "低", "success 须引用现存 criteria id；failure 非空；escalate 须有 to/times/fields"},
	{"R13", "static_layer_decomposition_abstain", "禁用", "静态层不做\"分解是否成立\"的判定 ✗ —— **本仓设计决定**（无需外部出处）；下沉到**执行后验收**（依据：**HTN 计划验证为 NP-complete**，Behnke et al., ICAPS 2015〔会议版〕）", "—", "规则内容就是「不做」⇒ 本检查器对它不产出任何红/绿（登记并打印，见 DISABLED_RULES）"},
}

type clauseRow struct{ Ref, Code, Text string }

var pyClauseTable = []clauseRow{
	{"§1-pre-defect_id", "REQUIRED", "`defect_id` 必填（一片只绑一个 ⇒ 另见 R1）"},
	{"§1-pre-landing", "REQUIRED", "`landing` 必填且现读得出（路径样式/存在性 ⇒ 另见 R2）"},
	{"§1-pre-open_questions", "S1-PRE-OPENQ", "`open_questions` 必填；**无则必须写「无」**（留空 ≠ 写「无」）；每条 ≤1 项；仅限进执行前"},
	{"§1-post-criteria", "REQUIRED", "`criteria` 必填（三件套 ⇒ 另见 R3）"},
	{"§1-post-gate_cmds", "REQUIRED", "`gate_cmds` 必填（每条带 expect ⇒ 另见 R6）"},
	{"§1-post-output_format", "S1-POST-NEGCTRL", "`output_format` 必须含**「改动前判据必红」的负控证据**（非空非占位 ⇒ 另见 R5）"},
	{"§1-post-exit_cond", "REQUIRED", "`exit_cond` 必填（结构化三项 ⇒ 另见 R12）"},
	{"§1-inv-hard_limits", "REQUIRED", "`hard_limits` 必填（非空非占位 ⇒ 另见 R5）"},
	{"§1-inv-side_effects", "S1-INV-SIDEEFFECTS", "`side_effects` ∈ {只读, 写工作树, 触网, git 写, 删除} **之一**"},
	{"§1-inv-escalate_if", "S1-INV-ESCALATEIF", "有 `side_effects` 即须配 `escalate_if`（表外字段）"},
	{"§1-inv-budget", "S1-INV-BUDGET", "`budget` 必须给**标定出处**；常数只能当初始值；无出处判红"},
	{"§1-inv-atomicity", "S1-INV-ATOMICITY", "`atomicity` = 落在执行者**标定技能/门禁闭包**内 + `criteria` 可在**一次调用内**判定"},
	{"§1-inv-slice_id", "S1-INV-SLICEID", "`slice_id` 必填、非「待定」，且与 `task_id` 一致（片 = 单子）"},
	{"§1-inv-depends_on", "REQUIRED", "`depends_on` 必填；**无依赖写「无」**（空数组会被 R5 形态命中）"},
	{"§1-out-the_one_thing", "REQUIRED", "`the_one_thing` 必填（R0 的判据对象）"},
	{"§1-out-context_budget", "REQUIRED", "`context_budget` 必填（R11 的判据对象）"},
	{"§2-conflict", "S2-CONFLICT", "冲突时必须输出 `CONFLICT` 标记、不得自行裁量；**有冲突却未标记 ⇒ 判红**"},
	{"§3.3-inherit", "S33-INHERIT", "子合同继承四条：子 pre ⊇ 父 pre / 子 post ⊆ 父 post / 子 hard_limits ⊆ 父 hard_limits / 父 exit_cond 由子集合共同满足 ⇒ 违反任一条判红"},
}

type r4Row struct{ Layer, Pat, Fix string }

var pyRegR4 = []r4Row{
	{"含糊", "相关", "把「相关」改写为具体路径或符号，例：`docs/01-设计/切片合同-模板.md:1`"},
	{"含糊", "大概", "改写为具体数值并标出处，例：`wall_s_max: 43.6`，出处 `~/.zerg/egg-profiles/task-budget-calib.yaml`"},
	{"含糊", "大致", "改写为具体数值并标出处（例：`steps_max: 3`）"},
	{"含糊", "差不多", "改写为具体数值（例：「≤3 步」）"},
	{"含糊", "一些", "改写为确定条数（例：「3 条」）"},
	{"含糊", "若干", "改写为确定条数（例：「2 项」）"},
	{"含糊", "适当", "写出具体判据（例：「≤3 步」）"},
	{"含糊", "尽量", "改写为 MUST 判据（例：「必须 ≤3 步」）"},
	{"含糊", "必要时", "写出触发条件（例：「当 X 命中时」）"},
	{"含糊", "一般来说", "写死条件（例：「当 τ 命中时」）"},
	{"含糊", "可能", "改写为条件句（例：「若 X 命中则…」）"},
	{"含糊", "有点", "改写为可判阈值"},
	{"含糊", "看情况", "写出条件与分支（例：「当 A 时…否则…」）"},
	{"含糊", "视情况", "写出条件与分支（例：「当 A 时…否则…」）"},
	{"含糊", "按惯例", "写出具体格式（例：「贴命令 + 原始输出 + 退出码」）"},
	{"含糊", "合理", "写出可判阈值（例：「假红率 ≤ 10%」）"},
	{"含糊", "等等", "列全条目并给条数（例：「C1 · C2 · C3，共 3 条」）"},
	{"含糊", "之类的", "列全条目（例：「红/绿」两种状态写全）"},
	{"含糊", "等其他", "列全条目"},
	{"含糊", "设计稿", "把设计稿里的原文片段**抄进** context_needed（例：v2.1 §3.3 抬头那一行原文）"},
	{"含糊", "模板那边", "改写为具体路径（例：`docs/01-设计/切片合同-模板.md`）"},
	{"含糊", "那边", "改写为具体路径"},
	{"含糊", "做完为止", "写出可观察的成功/失败条件（例：`C1 ∧ C2 ∧ C3`）"},
	{"含糊", "后续再说", "改写为 `NEEDS-CLARIFICATION` 或写死条件"},
	{"含糊", "效果良好", "改写为可判判据（例：「C1 输出 = 1 且 rc=0」）"},
	{"含糊", "效果好", "改写为可判判据"},
	{"含糊", "跑一下", "写出可复制命令（例：`python3 scripts/gates/check-slice.py --probe`）"},
	{"含糊", "优化一下", "写出优化项与阈值（例：「token ≤ 118」）"},
	{"含糊", "差不多就行", "改写为具体阈值"},
	{"含糊", "有问题再说", "改写为可观察失败条件"},
	{"可选", "可以", "改写为全大写 MUST / MAY（BCP 14 只认全大写关键词；「可以」不是关键词）"},
	{"可选", "可选", "写全大写 MAY（或不写）"},
	{"可选", "可选地", "写全大写 MAY"},
	{"可选", "也许", "写死条件或不写"},
	{"可选", "\\bmay\\b", "改写为全大写 MUST 或 MAY（小写不计入机器判据，RFC 8174 逐字）"},
	{"可选", "\\bcan\\b", "改写为全大写 MUST 或 MAY（小写不计入机器判据）"},
	{"可选", "\\bcould\\b", "改写为全大写 MUST 或 MAY"},
	{"可选", "\\boptionally\\b", "改写为全大写 MAY"},
	{"主观", "良好", "改写为可判阈值（例：「3/3 条由绿变红」）"},
	{"主观", "优秀", "改写为可判阈值"},
	{"主观", "美观", "改写为可判阈值（例：「像素差 = 0」）"},
	{"主观", "优雅", "改写为可判阈值"},
	{"主观", "简洁", "改写为可判阈值（例：「≤ 30 行」）"},
	{"主观", "顺畅", "改写为可判阈值（例：「wall ≤ 43.6s」）"},
	{"主观", "流畅", "改写为可判阈值"},
	{"主观", "清晰", "改写为可判阈值"},
	{"主观", "觉得", "删去主观判断，写可观察判据"},
	{"主观", "应该没问题", "写可观察判据"},
	{"主观", "体验好", "写可判阈值"},
	{"主观", "稳定", "写可判阈值（例：「连续 3 次全绿」）"},
}

type kvRow struct{ A, B string }

var pyRegExitTrap = []kvRow{
	{"\\bgofmt\\s+-l\\b", "gofmt -l 列出文件时仍 exit 0"},
	{"\\bgo\\s+test\\b", "go test 无测试文件仍 exit 0"},
	{"\\bpytest\\b", "pytest 无收集单列退码 5"},
}
var pyNotChecked = []kvRow{
	{"§3.3-抬头 BCP 14", "模板 §0 的「抬头 BCP 14 固定句存在性」**不属 R0–R13** ⇒ 本检查器不判（不得自创 R14）；片文件里也没有承载该抬头句的字段"},
	{"§3.3-递归深度", "递归切片「深度上限 2」与「同一 (defect_id, landing) 再现 ⇒ 判环升级」—— 片文件无「深度」字段、标定档案无深度键、R0–R13 无对应条目 ⇒ 本检查器只判「同 (landing, defect_id) 重复 ⇒ R8」，深度不判"},
	{"§3.3-取词口径", "「小写 must/should 不计入机器判据」的取词口径（只认全大写 MUST/SHOULD/MAY）未实施；R4 只判小写 may/can/could/可选 这类模糊可选词"},
	{"§3.2-atomicity 闭包", "「执行者标定技能/门禁闭包」字段在标定档案里**不存在** ⇒ 闭包本身记「未标定」，只判文本是否同时给出「闭包」与「一次调用内可判」（登记口径）"},
	{"R0-词表法", "「只含一个动词」用**词表法**近似（词表见上方「判据口径来源」块、最长匹配非重叠），不是句法分析：词表外的动作词会判 R0-NO-VERB；词表词的非动词用法（如「附带」含「带」）可能判 R0-MULTIVERB"},
	{"档案绑定上界", "R7 行数上界、§3.6 探针条数门槛、§3.5 假红上限一律**先从标定档案读键**，缺则用登记默认并打印来源；现档案只有 steps_max / tool_calls_max / wall_s_max / assistant_tokens_est_max"},
}
var pyRegVerbs = []string{"写明", "写进", "写入", "写成", "写出", "补上", "补齐", "补入", "给出", "照抄", "替换为", "替换", "换成", "改为", "改成", "修正", "修好", "删除", "移除", "去掉", "重构", "重命名", "迁移", "新建", "落盘", "同步", "更新", "升级", "执行", "运行", "实现", "导出", "打印", "接入", "生成", "插入", "整理", "改造", "修复", "封住", "拦住", "指定", "删掉", "添加", "加上", "加入", "添上", "写", "补", "加", "改", "修", "建", "跑", "换", "带", "放", "贴", "抄"}
var pyRegConj = []string{"并", "并且", "同时", "以及", "一起", "和", "与"}
var pyRegPlaceholder = []string{"按惯例", "做完为止", "看情况", "TODO", "TBD", "XXX", "待定", "待补", "占位", "N/A", "n/a", "略", "同前", "同上"}
var pyRegExpectKeys = []string{"exit", "stdout", "stderr", "contains", "regex", "eq"}
var pyRegCtxKinds = []string{"snippet", "file", "cmd", "note"}
var pyRegSnippetKinds = []string{"snippet"}
var pyR11AfterCriteria = []string{"gate_cmds", "output_format", "exit_cond", "hard_limits", "side_effects", "escalate_if", "budget", "atomicity", "slice_id", "depends_on", "context_budget", "conflicts"}
var pySideEffectEnum = []string{"只读", "写工作树", "触网", "git 写", "删除"}
var pyStateRed = []string{"红", "red", "RED"}
var pyStateGreen = []string{"绿", "green", "GREEN"}
var pyMetaKeys = []string{"conflicts", "parent", "task_id", "rules", "_note"}
var pyProbeKnownKeys = []string{"name", "expect", "class", "why", "expect_rules", "slice", "slices", "note"}
var pyProbeExpects = []string{"red", "green", "recursion", "error"}
var pyJsonFields = []string{}
var pyOffWords = []string{"off", "disable", "disabled", "skip", "skipped", "false", "no", "0"}
var pyFieldKeys = []string{"defect_id", "landing", "context_needed", "open_questions", "criteria", "gate_cmds", "output_format", "exit_cond", "hard_limits", "side_effects", "budget", "atomicity", "slice_id", "depends_on", "the_one_thing", "context_budget", "escalate_if", "edit_assert_expect"}
var pyRequiredFields = [][2]string{
	{"defect_id", "§1-pre-defect_id"},
	{"landing", "§1-pre-landing"},
	{"context_needed", "§1-pre-context_needed"},
	{"open_questions", "§1-pre-open_questions"},
	{"criteria", "§1-post-criteria"},
	{"gate_cmds", "§1-post-gate_cmds"},
	{"output_format", "§1-post-output_format"},
	{"exit_cond", "§1-post-exit_cond"},
	{"hard_limits", "§1-inv-hard_limits"},
	{"side_effects", "§1-inv-side_effects"},
	{"budget", "§1-inv-budget"},
	{"atomicity", "§1-inv-atomicity"},
	{"slice_id", "§1-inv-slice_id"},
	{"depends_on", "§1-inv-depends_on"},
	{"the_one_thing", "§1-out-the_one_thing"},
	{"context_budget", "§1-out-context_budget"},
}
var pyOptionalFields = [][3]string{
	{"edit_assert_expect", "§1-post-edit_assert_expect", "期望命中数（写盘门 edit-assert 的 E2 判据对象）：一组 `(target, expect, reason)`，形态见 §4.2 四形态"},
}
var pyProbeExpectCN = map[string]string{
	"red":       "红/违规",
	"green":     "绿/合法",
	"recursion": "需递归",
	"error":     "错误(rc=2)",
}
var pyProbeGotCN = map[string]string{
	"red":       "判红",
	"green":     "判绿",
	"recursion": "判需递归",
	"error":     "判错误",
}
var pyPartialRules = map[string][3]string{
	"R9":  {"依赖闭包闭合：集合里每片的 `depends_on` 目标必须在集合内（`--set-open` 时记「不可判」）", "「该缺陷的每条验收判据至少被一片承接」+「全部 open defect 被覆盖」—— 缺输入 schema：缺陷条目与它的验收判据清单在片文件里没有字段可承载（模板未定）；本检查器不猜", "§3.4-R9（「部分（片集合级）」）；模板 §8-*未给承接口径"},
	"R10": {"句法侧：context_needed 非空 + 含「落点原文片段」+ 不得把设计稿/别的文档当上下文", "「无设计稿也能复现」的**实跑**那一半（只给片、不给设计稿，跑一次看能否复现）—— 非纯句法", "§3.4-R10（「部分（另需「只给片」实跑一次，非纯句法）」）"},
}
var pyDisabledRules = map[string][4]string{
	"R13": {"规则内容就是「静态层不做\"分解是否成立\"的判定」⇒ 静态层无对象可判", "§3.4-R13（逐字；本仓设计决定）", "该判定下沉到**执行后验收**（不属本检查器的判域）", "本检查器对 R13 不产出任何红/绿；R13 不出现在任何一片的命中行里"},
}
var pyRouteRules = map[string]string{
	"R7": "超限判\"需递归\"，而不是打回（§3.4-R7 的 ✗）⇒ 本条**永不产出红色**，只产出「需递归」标记",
}
var pyRegisteredDisable = map[string]string{
	"R9":  "R9 的未判部分见 PARTIAL_RULES；整体禁用只关掉已实现的那一半（依赖闭包闭合）",
	"R10": "R10 的未判部分见 PARTIAL_RULES；整体禁用只关掉已实现的那一半（句法侧自包含性）",
}
var pyProbeDefaults = map[string]float64{
	"probe_vague_min":   10.0,
	"probe_legal_min":   10.0,
	"probe_fp_rate_max": 0.1,
}

// regex patterns
var pyPat_FULLSCAN_RE = "\\./\\.\\.\\.|全量|\\ball\\b"
var pyPat_PATHY_EXT_RE = "\\.(md|py|sh|go|json|yaml|yml|txt|toml|cfg|ini|sql|db|ts|js)\\b"
var pyPat_EXTERNAL_REF_RE = "(?:\\.\\./)+[^\\s\\'\"，,；;、）)】」》|<>`]+"

// ── 有序 JSON 值（区分缺键 / 显式 null / key 序；py dict 保插入序 ⇒ _as_text 逐字需保序）──
type jpair struct {
	k string
	v *jv
}
type jv struct {
	kind byte // 'n' null · 'b' bool · 'i' number · 's' string · 'a' array · 'o' object
	b    bool
	num  string
	s    string
	arr  []*jv
	obj  []jpair
}

func jparse(data []byte) (*jv, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := jpVal(dec)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func jpVal(dec *json.Decoder) (*jv, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return jpFromTok(dec, tok)
}

func jpFromTok(dec *json.Decoder, tok json.Token) (*jv, error) {
	switch t := tok.(type) {
	case nil:
		return &jv{kind: 'n'}, nil
	case bool:
		return &jv{kind: 'b', b: t}, nil
	case string:
		return &jv{kind: 's', s: t}, nil
	case json.Number:
		return &jv{kind: 'i', num: t.String()}, nil
	case json.Delim:
		if t == '[' {
			o := &jv{kind: 'a', arr: []*jv{}}
			for dec.More() {
				el, err := jpVal(dec)
				if err != nil {
					return nil, err
				}
				o.arr = append(o.arr, el)
			}
			if _, err := dec.Token(); err != nil { // closing ]
				return nil, err
			}
			return o, nil
		}
		if t == '{' {
			o := &jv{kind: 'o', obj: []jpair{}}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := kt.(string)
				val, err := jpVal(dec)
				if err != nil {
					return nil, err
				}
				o.obj = append(o.obj, jpair{key, val})
			}
			if _, err := dec.Token(); err != nil { // closing }
				return nil, err
			}
			return o, nil
		}
	}
	return nil, fmt.Errorf("bad token")
}

func jget(o *jv, k string) *jv {
	if o == nil || o.kind != 'o' {
		return nil
	}
	for _, kv := range o.obj {
		if kv.k == k {
			return kv.v
		}
	}
	return nil
}

func jpresent(o *jv, k string) bool {
	if o == nil || o.kind != 'o' {
		return false
	}
	for _, kv := range o.obj {
		if kv.k == k {
			return true
		}
	}
	return false
}

func csPyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		if r >= 0x1c && r <= 0x1f {
			return true
		}
		return unicode.IsSpace(r)
	})
}

func jNonempty(v *jv) bool {
	if v == nil || v.kind == 'n' {
		return false
	}
	switch v.kind {
	case 's':
		return csPyStrip(v.s) != ""
	case 'a':
		return len(v.arr) > 0
	case 'o':
		return len(v.obj) > 0
	}
	return true
}

func jIsStr(v *jv) bool { return v != nil && v.kind == 's' }

func jJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// pyJSONDumps —— 复刻 json.dumps(x, ensure_ascii=False) 默认分隔符（', ' 与 ': '）· 保 key 序。
func pyJSONDumps(v *jv) string {
	if v == nil {
		return "null"
	}
	switch v.kind {
	case 'n':
		return "null"
	case 'b':
		if v.b {
			return "true"
		}
		return "false"
	case 'i':
		return v.num
	case 's':
		return jJSONStr(v.s)
	case 'a':
		parts := make([]string, 0, len(v.arr))
		for _, el := range v.arr {
			parts = append(parts, pyJSONDumps(el))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 'o':
		parts := make([]string, 0, len(v.obj))
		for _, kv := range v.obj {
			parts = append(parts, jJSONStr(kv.k)+": "+pyJSONDumps(kv.v))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "null"
}

func jAsText(v *jv) string {
	if v == nil || v.kind == 'n' {
		return ""
	}
	if v.kind == 's' {
		return v.s
	}
	return pyJSONDumps(v)
}

func isIntLiteral(n string) bool {
	return !strings.ContainsAny(n, ".eE")
}

// ── py os.path 语义（★ 逐处现读）──
func pyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initial = 2
		}
	}
	comps := []string{}
	for _, c := range strings.Split(path, "/") {
		if c == "" || c == "." {
			continue
		}
		comps = append(comps, c)
	}
	out := []string{}
	for _, c := range comps {
		if c == ".." && len(out) > 0 && out[len(out)-1] != ".." {
			out = out[:len(out)-1]
		} else if c == ".." && initial == 0 && len(out) == 0 {
			out = append(out, "..")
		} else if c == ".." && initial == 0 {
			out = append(out, "..")
		} else if c == ".." {
			// absolute root: drop
		} else {
			out = append(out, c)
		}
	}
	res := strings.Join(out, "/")
	if initial > 0 {
		res = strings.Repeat("/", initial) + res
	}
	if res == "" {
		return "."
	}
	return res
}

func pyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path = path + b
		} else {
			path = path + "/" + b
		}
	}
	return path
}

func pyDirname(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	if i == 0 {
		return "/"
	}
	return p[:i]
}

func pyBasename(p string) string {
	return filepath.Base(p)
}

func pyIsAbs(p string) bool { return strings.HasPrefix(p, "/") }

func pyAbspath(p string) string {
	if pyIsAbs(p) {
		return pyNormpath(p)
	}
	wd, _ := os.Getwd()
	return pyNormpath(pyJoin(wd, p))
}

func pyCommonpath(paths []string) (string, bool) {
	if len(paths) == 0 {
		return "", false
	}
	split := func(p string) []string {
		abs := pyIsAbs(p)
		parts := []string{}
		for _, c := range strings.Split(p, "/") {
			if c != "" {
				parts = append(parts, c)
			}
		}
		if abs {
			parts = append([]string{"/"}, parts...)
		}
		return parts
	}
	base := split(paths[0])
	for _, p := range paths[1:] {
		cs := split(p)
		n := len(base)
		if len(cs) < n {
			n = len(cs)
		}
		i := 0
		for i < n && base[i] == cs[i] {
			i++
		}
		base = base[:i]
	}
	if len(base) == 0 {
		return "", false
	}
	if base[0] == "/" {
		if len(base) == 1 {
			return "/", true
		}
		return "/" + strings.Join(base[1:], "/"), true
	}
	return strings.Join(base, "/"), true
}

// ── 运行上下文（py 全局 HERE/REPO_ROOT/CALIB_PATH/PROBE_PATH 的对应）──
var (
	csRepoRoot  string
	csCalibPath string
)

func csGatesDir() string  { return pyJoin(csRepoRoot, "scripts", "gates") }
func csProbePath() string { return pyJoin(csGatesDir(), "slice-probes.json") }

func csDefaultRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return pyAbspath(wd)
}

func csCalibPathFromEnv() string {
	egg := os.Getenv("ZERG_EGG_PROFILE_DIR")
	if egg == "" {
		home, _ := os.UserHomeDir()
		egg = pyJoin(home, ".zerg", "egg-profiles")
	}
	return pyJoin(egg, "task-budget-calib.yaml")
}

// ── 结果容器 ──
type internalErr struct{ msg string }

func (e internalErr) Error() string { return e.msg }

type hitT struct{ rule, code, msg, fix string }
type noteT struct{ code, msg string }

type result struct {
	name      string
	hits      []hitT
	notes     []noteT
	recursion []noteT
}

func (r *result) hit(rule, code, msg, fix string) {
	if csPyStrip(fix) == "" {
		panic(internalErr{fmt.Sprintf("命中 %s / %s 缺「改写建议」⇒ 按 §3.4-R4「无改写建议不算有效报红」拒绝出结论", rule, code)})
	}
	r.hits = append(r.hits, hitT{rule, code, msg, fix})
}
func (r *result) note(code, msg string) { r.notes = append(r.notes, noteT{code, msg}) }
func (r *result) needRecursion(code, msg string) {
	r.recursion = append(r.recursion, noteT{code, msg})
}
func (r *result) redRules() []string {
	out := []string{}
	for _, h := range r.hits {
		found := false
		for _, x := range out {
			if x == h.rule {
				found = true
				break
			}
		}
		if !found {
			out = append(out, h.rule)
		}
	}
	return out
}
func (r *result) isRed() bool { return len(r.hits) > 0 }

// ── 常量（表由 py 源码程序化导出 · tables.inc）──
var pyRuleKeys = func() []string {
	out := []string{}
	for _, r := range pyRuleTable {
		out = append(out, r.Num)
	}
	return out
}()
var pyRuleNames = func() map[string]string {
	o := map[string]string{}
	for _, r := range pyRuleTable {
		o[r.Num] = r.Key
	}
	return o
}()
var pyRuleState = func() map[string]string {
	o := map[string]string{}
	for _, r := range pyRuleTable {
		o[r.Num] = r.State
	}
	return o
}()
var FIELD_KEYS_SET = func() map[string]bool {
	o := map[string]bool{}
	for _, k := range pyFieldKeys {
		o[k] = true
	}
	return o
}()
var META_KEYS_SET = func() map[string]bool {
	o := map[string]bool{}
	for _, k := range pyMetaKeys {
		o[k] = true
	}
	return o
}()
var pyKnownSliceKeys = func() map[string]bool {
	o := map[string]bool{}
	for _, k := range pyFieldKeys {
		o[k] = true
	}
	for _, k := range pyMetaKeys {
		o[k] = true
	}
	return o
}()
var pyRuleKeysSet = func() map[string]bool {
	o := map[string]bool{}
	for _, k := range pyRuleKeys {
		o[k] = true
	}
	return o
}()

var pyFullscanRe = regexp.MustCompile(pyPat_FULLSCAN_RE)
var pyPathyExtRe = regexp.MustCompile(pyPat_PATHY_EXT_RE)
var pyExternalRefRe = regexp.MustCompile(pyPat_EXTERNAL_REF_RE)
var pyExitTrapRe = func() []*regexp.Regexp {
	o := []*regexp.Regexp{}
	for _, kv := range pyRegExitTrap {
		o = append(o, regexp.MustCompile(kv.A))
	}
	return o
}()

func csContainsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ── R0 动词计数（词表法 · 最长匹配 · 非重叠 —— 按 rune 迭代）──
func countVerbs(text string) int {
	n := 0
	i := 0
	runes := []rune(text)
	words := make([][]rune, 0, len(pyRegVerbs))
	for _, w := range pyRegVerbs {
		words = append(words, []rune(w))
	}
	sort.SliceStable(words, func(a, b int) bool { return len(words[a]) > len(words[b]) })
	hasPrefix := func(buf, w []rune) bool {
		if len(w) > len(buf) {
			return false
		}
		for k := 0; k < len(w); k++ {
			if buf[k] != w[k] {
				return false
			}
		}
		return true
	}
	for i < len(runes) {
		matched := false
		for _, w := range words {
			if hasPrefix(runes[i:], w) {
				n++
				i += len(w)
				matched = true
				break
			}
		}
		if !matched {
			i++
		}
	}
	return n
}

// ── R4 逐字段扫描 ──
func r4Scan(res *result, where, text string) {
	if csPyStrip(text) == "" {
		return
	}
	for _, row := range pyRegR4 {
		pat := row.Pat
		var sIdx, eIdx int = -1, -1
		if strings.HasPrefix(pat, "\\\\b") {
			re := regexp.MustCompile(pat)
			runes := []rune(text)
			// find via byte regex then map to rune indices
			bi := re.FindStringIndex(text)
			if bi != nil {
				sIdx = len([]rune(text[:bi[0]]))
				eIdx = sIdx + len([]rune(text[bi[0]:bi[1]]))
				_ = runes
			}
		} else if strings.Contains(text, pat) {
			runes := []rune(text)
			pr := []rune(pat)
			for i := 0; i+len(pr) <= len(runes); i++ {
				if string(runes[i:i+len(pr)]) == pat {
					sIdx = i
					eIdx = i + len(pr)
					break
				}
			}
		}
		if sIdx < 0 {
			continue
		}
		code := map[string]string{"含糊": "R4-VAGUE", "可选": "R4-OPTIONAL", "主观": "R4-SUBJECTIVE"}[row.Layer]
		runes := []rune(text)
		lo := sIdx - 12
		if lo < 0 {
			lo = 0
		}
		hi := eIdx + 12
		if hi > len(runes) {
			hi = len(runes)
		}
		frag := strings.ReplaceAll(string(runes[lo:hi]), "\n", " ")
		res.hit("R4", code, fmt.Sprintf("%s[%s] 命中无界词「%s」：…%s…", where, row.Layer, pat, frag), row.Fix)
	}
}

func r0(res *result, sl *jv) {
	v := jget(sl, "the_one_thing")
	if !jNonempty(v) || !jIsStr(v) {
		res.hit("R0", "R0-MISSING", "the_one_thing 缺失/为空",
			"写成一句话，照句式：「When/While ⟨条件⟩，⟨落点⟩ 必须 ⟨单一动作⟩」（只含一个动词）")
		return
	}
	s := csPyStrip(v.s)
	m := regexp.MustCompile(`^(When|While)\s+(.+?)[，,]\s*(.+)$`).FindStringSubmatch(s)
	if m == nil {
		res.hit("R0", "R0-SHAPE",
			fmt.Sprintf("the_one_thing 套不进「When/While ⟨条件⟩，⟨落点⟩ 必须 ⟨单一动作⟩」：%s", runeSliceStr(s, 0, 60)),
			"改写成：When/While ⟨条件⟩，⟨落点⟩ 必须 ⟨单一动作⟩，例："+
				"「When 模板骨架已落盘时，docs/01-设计/切片合同-模板.md 的抬头必须写明 BCP 14 固定句。」")
		return
	}
	tail := m[3]
	if !strings.Contains(tail, "必须") {
		res.hit("R0", "R0-NO-MUST",
			fmt.Sprintf("⟨落点⟩ 之后缺「必须」（正面句式的谓词）：%s", runeSliceStr(tail, 0, 60)),
			"在⟨落点⟩后补「必须 ⟨单一动作⟩」，例：「…的抬头必须写明 BCP 14 固定句。」")
		return
	}
	action := strings.SplitN(tail, "必须", 2)[1]
	nverbs := countVerbs(action)
	nconj := 0
	for _, c := range pyRegConj {
		if strings.Contains(action, c) {
			nconj++
		}
	}
	if nverbs != 1 {
		code := "R0-NO-VERB"
		if nverbs > 1 {
			code = "R0-MULTIVERB"
		}
		extra := ""
		if nconj > 0 {
			extra = "（另含并列词）"
		}
		res.hit("R0", code,
			fmt.Sprintf("⟨单一动作⟩ 段动词数 = %d%s（只允许 1 个）：%s", nverbs, extra, runeSliceStr(action, 0, 60)),
			"只留一个动词，例：「必须写明 BCP 14 固定句。」（删去并/和/同时 之类的并列动作）")
	}
}

func runeSliceStr(s string, lo, hi int) string {
	r := []rune(s)
	if lo > len(r) {
		lo = len(r)
	}
	if hi > len(r) {
		hi = len(r)
	}
	return string(r[lo:hi])
}

func splitIDs(s string) []string {
	return regexp.MustCompile(`[,，、;；\s]+`).Split(s, -1)
}

func r1(res *result, sl *jv) {
	v := jget(sl, "defect_id")
	var ids []string
	if v != nil && v.kind == 'a' {
		for _, x := range v.arr {
			if jNonempty(x) {
				ids = append(ids, jAsText(x))
			}
		}
	} else if jIsStr(v) {
		s := csPyStrip(v.s)
		if s == "无" || s == "NEEDS-CLARIFICATION" {
			ids = []string{s}
		} else {
			for _, t := range splitIDs(s) {
				if t != "" {
					ids = append(ids, t)
				}
			}
		}
	} else if jNonempty(v) {
		ids = []string{jAsText(v)}
	}
	if len(ids) > 1 {
		res.hit("R1", "R1-MULTI-DEFECT",
			fmt.Sprintf("一片绑了 %d 个 defect_id：%s", len(ids), strings.Join(ids, " / ")),
			fmt.Sprintf("拆成 %d 片，每片只绑一个 defect_id（例：本片留 `%s`，其余另开片）", len(ids), ids[0]))
	} else if len(ids) == 0 {
		res.hit("R1", "R1-NO-DEFECT", "defect_id 缺失/为空",
			"填一个本片所修缺陷的 id，例：`DEF-20260918-S3-01`")
	}
}

func landingCandidates(landing string) []string {
	s := csPyStrip(landing)
	cands := []string{}
	parts := strings.Fields(s)
	first := s
	if len(parts) > 0 {
		first = parts[0]
	}
	if strings.Contains(first, ":") {
		cands = append(cands, strings.SplitN(first, ":", 2)[0])
	}
	cands = append(cands, first)
	col := strings.SplitN(s, ":", 2)[0]
	sp := strings.SplitN(col, " ", 2)[0]
	cands = append(cands, sp)
	out := []string{}
	trimset := "。，,;；)）】」"
	for _, c := range cands {
		c = strings.TrimRight(csPyStrip(c), trimset)
		if c != "" && !csContainsStr(out, c) {
			out = append(out, c)
		}
	}
	return out
}

func r2(res *result, sl *jv) {
	v := jget(sl, "landing")
	if !jNonempty(v) || !jIsStr(v) {
		res.hit("R2", "R2-NO-PATH", "landing 缺失/为空（无路径样式）",
			"写成「文件路径:行号/符号」，例：`docs/01-设计/切片合同-模板.md:1`")
		return
	}
	cands := landingCandidates(v.s)
	pathy := []string{}
	for _, c := range cands {
		if strings.Contains(c, "/") || pyPathyExtRe.MatchString(c) {
			pathy = append(pathy, c)
		}
	}
	if len(pathy) == 0 {
		res.hit("R2", "R2-NO-PATH",
			fmt.Sprintf("landing 无路径样式（无 `/`、无登记扩展名）：%s", runeSliceStr(v.s, 0, 60)),
			"改写成具体路径或「路径:符号」，例：`docs/01-设计/切片合同-模板.md:1`")
		return
	}
	found := ""
	for _, c := range pathy {
		p := c
		if !pyIsAbs(c) {
			p = pyJoin(csRepoRoot, c)
		}
		if _, err := os.Stat(p); err == nil {
			found = c
			break
		}
	}
	if found == "" {
		res.hit("R2", "R2-NOT-FOUND",
			fmt.Sprintf("landing 的路径样式**当前不存在**（前状态检查不过）；试过：%s", strings.Join(pathy, " , ")),
			"改成现读得出的落点，例：先把目标文件建出来（`touch <路径>`）或把 landing 指向已存在文件")
	} else {
		res.note("R2-OK", fmt.Sprintf("landing 前状态检查通过：%s", found))
		r2ContentAnchor(res, sl, found)
	}
}

// r2FirstSnippet —— 「落点原文片段」真源：context_needed 首个 kind=="snippet" 的 value
// （与 R10/R11 同源定义：`{"kind":"snippet","value":"<落点那一行原文>"}`）。
func r2FirstSnippet(sl *jv) string {
	ctx := jget(sl, "context_needed")
	items := []*jv{}
	if ctx != nil && ctx.kind == 'a' {
		items = ctx.arr
	} else if jNonempty(ctx) {
		items = []*jv{ctx}
	}
	for _, x := range items {
		if isSnippetItem(x) {
			return ctxValues(x)
		}
	}
	return ""
}

// r2DeclaredLine —— 取 landing 首 token 的「:N」的 N（无 / 非数 ⇒ 0）。
func r2DeclaredLine(landing string) int {
	tok := csPyStrip(landing)
	if parts := strings.Fields(tok); len(parts) > 0 {
		tok = parts[0]
	}
	tok = strings.TrimRight(tok, "。，,;；)）】」")
	i := strings.LastIndex(tok, ":")
	if i < 0 || i == len(tok)-1 {
		return 0
	}
	if n, err := strconv.Atoi(tok[i+1:]); err == nil && n >= 1 {
		return n
	}
	return 0
}

// r2ContentAnchor —— R2 内容锚：**声明片段**必须在 **landing 声明的行**里逐字命中。
// 「锚不适用」（无行号 / 无片段 / 件读不到）⇒ 只记 note；锚适用而不命中 ⇒ 红。
func r2ContentAnchor(res *result, sl *jv, found string) {
	land := jget(sl, "landing")
	if !jIsStr(land) {
		return
	}
	n := r2DeclaredLine(land.s)
	snip := r2FirstSnippet(sl)
	if n == 0 || snip == "" {
		res.note("R2-ANCHOR-SKIP", "内容锚跳过：landing 无「:行号」或片无「落点原文片段」")
		return
	}
	p := found
	if !pyIsAbs(p) {
		p = pyJoin(csRepoRoot, p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		res.note("R2-ANCHOR-SKIP", fmt.Sprintf("内容锚跳过：落点件读不到（%s）", found))
		return
	}
	lines := strings.Split(string(data), "\n")
	if n > len(lines) {
		res.hit("R2", "R2-SNIPPET-DRIFT",
			fmt.Sprintf("landing 声明行 %d 超出件行数 %d（%s）", n, len(lines), found),
			"把 landing 的「:行号」改成片段真身所在行")
		return
	}
	line := lines[n-1]
	if !strings.Contains(line, snip) {
		res.hit("R2", "R2-SNIPPET-DRIFT",
			fmt.Sprintf("声明片段与声明行不逐字命中（%s:%d）：片段=%q｜该行=%q",
				found, n, runeSliceStr(snip, 0, 60), runeSliceStr(line, 0, 60)),
			"二选一：① 把「:行号」改成片段真身所在行；② 把片段改成该行现读逐字（片段 = 落点那一行原文）")
		return
	}
	res.note("R2-SNIPPET-OK", fmt.Sprintf("内容锚命中：%s:%d 含声明片段", found, n))
}

func r3(res *result, sl *jv) {
	v := jget(sl, "criteria")
	if !jNonempty(v) {
		res.hit("R3", "R3-MISSING", "criteria 缺失/为空",
			"写成列表，每项 = 三件套 {id, cmd, expect, state}，例："+
				"{\"id\":\"C1\",\"cmd\":\"awk ... | grep -c X\",\"expect\":\"1\",\"state\":\"改动前：红｜落盘后：绿\"}")
		return
	}
	if jIsStr(v) {
		res.hit("R3", "R3-NON-STRUCTURED",
			fmt.Sprintf("criteria 是自由文本（三件套缺：无结构化命令 / 期望输出 / 当前红-绿状态）：%s", runeSliceStr(v.s, 0, 60)),
			"改成列表，每项 = {id, cmd, expect, state}；命令必须可复制、expect 必须写死、state 写明当前红还是绿")
		return
	}
	if v.kind != 'a' {
		res.hit("R3", "R3-NON-STRUCTURED", fmt.Sprintf("criteria 不是列表（实为 %s）", pyTypeName(v)),
			"改成列表，每项 = {id, cmd, expect, state}")
		return
	}
	for idx, item := range v.arr {
		tag := fmt.Sprintf("criteria[%d]", idx)
		if item.kind != 'o' {
			res.hit("R3", "R3-NON-STRUCTURED", fmt.Sprintf("%s 不是 {id,cmd,expect,state} 结构", tag),
				"改成对象 {id, cmd, expect, state}")
			continue
		}
		if !jNonempty(jget(item, "id")) {
			res.hit("R3", "R3-MISSING-ID", fmt.Sprintf("%s 缺 id（exit_cond 成功项要引用它，R12）", tag),
				"补一个稳定 id，例：`\"id\": \"C1\"`")
		}
		if !jNonempty(jget(item, "cmd")) {
			res.hit("R3", "R3-MISSING-CMD", fmt.Sprintf("%s 缺「可复制命令」", tag),
				"补可原样复制运行的命令，例：`\"cmd\": \"awk '/^## 0\\\\./,/^## 1\\\\./' <文件> | grep -c X\"`")
		}
		if !jNonempty(jget(item, "expect")) {
			res.hit("R3", "R3-MISSING-EXPECT", fmt.Sprintf("%s 缺「期望输出」", tag),
				"补 expect 并**写死**期望值，例：`\"expect\": \"1\"`（无期望输出 ⇒ 无从判红绿 ⇒ 必假绿）")
		}
		st := jget(item, "state")
		if !jNonempty(st) {
			res.hit("R3", "R3-MISSING-STATE", fmt.Sprintf("%s 缺「当前红/绿状态」", tag),
				"补 state，例：`\"state\": \"改动前：红（输出 0，rc=1）｜落盘后：绿（输出 1）\"`")
		} else if jIsStr(st) {
			ok := false
			for _, w := range append(append([]string{}, pyStateRed...), pyStateGreen...) {
				if strings.Contains(st.s, w) {
					ok = true
					break
				}
			}
			if !ok {
				res.hit("R3", "R3-STATE-NO-REDGREEN",
					fmt.Sprintf("%s 的 state 未写红/绿：%s", tag, runeSliceStr(st.s, 0, 40)),
					"把当前状态写成「红」或「绿」，例：`\"state\": \"改动前：红\"`")
			}
		}
	}
}

func pyTypeName(v *jv) string {
	switch v.kind {
	case 'n':
		return "NoneType"
	case 'b':
		return "bool"
	case 'i':
		return "int"
	case 's':
		return "str"
	case 'a':
		return "list"
	case 'o':
		return "dict"
	}
	return "object"
}

func r5(res *result, sl *jv) {
	fields := []string{"hard_limits", "side_effects", "output_format", "exit_cond"}
	for _, f := range fields {
		if !jpresent(sl, f) {
			res.hit("R5", "R5-MISSING", fmt.Sprintf("%s 缺失（空壳检测对象四字段之一）", f),
				fmt.Sprintf("填 %s；未决就写 `NEEDS-CLARIFICATION`（不得留空、不得留 `[]`/`<>`）", f))
			continue
		}
		v := jget(sl, f)
		if !jNonempty(v) {
			name := "空列表/空对象"
			if jIsStr(v) || v.kind == 'n' {
				name = "空字符串/None"
			}
			res.hit("R5", "R5-EMPTY", fmt.Sprintf("%s 是空壳（%s；空数组 = 未替换的 `[]` 形态）", f, name),
				"填实质内容；未决就写 `NEEDS-CLARIFICATION`")
			continue
		}
		txt := jAsText(v)
		for _, ph := range pyRegPlaceholder {
			if strings.Contains(txt, ph) {
				res.hit("R5", "R5-PLACEHOLDER",
					fmt.Sprintf("%s 命中占位文本「%s」：%s", f, ph, runeSliceStr(txt, 0, 50)),
					fmt.Sprintf("把「%s」改写成实质内容或 `NEEDS-CLARIFICATION`", ph))
				break
			}
		}
		if strings.Contains(txt, "[]") || strings.Contains(txt, "<>") {
			res.hit("R5", "R5-UNREPLACED",
				fmt.Sprintf("%s 含未替换的 `[]`/`<>`：%s", f, runeSliceStr(txt, 0, 50)),
				"把 `[]`/`<>` 换成实际内容；未决就写 `NEEDS-CLARIFICATION`")
		}
	}
}

func r6(res *result, sl *jv) {
	v := jget(sl, "gate_cmds")
	if !jNonempty(v) {
		res.hit("R6", "R6-MISSING", "gate_cmds 缺失/为空",
			"写成 `[{cmd, expect}]`，例：`[{\"cmd\": \"… | grep -c X\", \"expect\": {\"exit\": 0, \"stdout\": \"1\"}}]`")
		return
	}
	if v.kind != 'a' {
		res.hit("R6", "R6-NON-LIST", fmt.Sprintf("gate_cmds 不是列表（实为 %s）", pyTypeName(v)),
			"改成列表，每项 = `{cmd, expect}`")
		return
	}
	for idx, item := range v.arr {
		tag := fmt.Sprintf("gate_cmds[%d]", idx)
		if item.kind != 'o' {
			res.hit("R6", "R6-NON-PARSE", fmt.Sprintf("%s 不是 `{cmd, expect}` 结构", tag),
				"改成 `{\"cmd\": \"…\", \"expect\": {...}}`")
			continue
		}
		cmd := jget(item, "cmd")
		if !jNonempty(cmd) || !jIsStr(cmd) {
			res.hit("R6", "R6-NON-PARSE", fmt.Sprintf("%s 的 cmd 缺失或不是可解析字符串", tag),
				"把 cmd 写成一行可复制命令字符串")
			continue
		}
		exp := jget(item, "expect")
		if !jpresent(item, "expect") || !jNonempty(exp) {
			res.hit("R6", "R6-NO-EXPECT", fmt.Sprintf("%s 无 expect（cmd=%s）", tag, runeSliceStr(cmd.s, 0, 40)),
				"补 expect，例：`\"expect\": {\"exit\": 0, \"stdout\": \"1\"}`（退出码会假绿 ⇒ 至少再写 stdout）")
			continue
		}
		keys := map[string]bool{}
		if exp.kind == 'o' {
			bad := []string{}
			for _, kv := range exp.obj {
				if !csContainsStr(pyRegExpectKeys, kv.k) {
					bad = append(bad, kv.k)
				}
				keys[kv.k] = true
			}
			if len(bad) > 0 {
				res.hit("R6", "R6-EXPECT-KEY-UNKNOWN",
					fmt.Sprintf("%s 的 expect 含未登记键 %s（登记取值域：%s）", tag, strings.Join(bad, " / "), strings.Join(pyRegExpectKeys, " · ")),
					"改用登记键，例：`{\"exit\": 0, \"stdout\": \"1\"}`")
			}
		} else if jIsStr(exp) {
			keys["str"] = true
		} else {
			res.hit("R6", "R6-EXPECT-NON-PARSE",
				fmt.Sprintf("%s 的 expect 不可解析（实为 %s）", tag, pyTypeName(exp)),
				"把 expect 写成 dict（`{\"exit\":0,\"stdout\":\"1\"}`）或非空字符串")
		}
		if len(keys) == 1 && keys["exit"] {
			for i, re := range pyExitTrapRe {
				if re.MatchString(cmd.s) {
					res.hit("R6", "R6-EXIT-ONLY-TRAP",
						fmt.Sprintf("%s 只给了 exit 期望，而该命令是登记过的假绿形态（%s）", tag, pyRegExitTrap[i].B),
						"补 stdout/contains 期望，例：`\"expect\": {\"exit\": 0, \"stdout\": \"1\"}`；"+
							"或把命令换成能由绿变红的形态（如 `… | grep -c X`）")
				}
			}
		}
	}
}

func landingPrimary(landing string) string {
	cands := landingCandidates(landing)
	for _, c := range cands {
		p := c
		if !pyIsAbs(c) {
			p = pyJoin(csRepoRoot, c)
		}
		if _, err := os.Stat(p); err == nil {
			return c
		}
	}
	pathy := []string{}
	for _, c := range cands {
		if strings.Contains(c, "/") || pyPathyExtRe.MatchString(c) {
			pathy = append(pathy, c)
		}
	}
	if len(pathy) > 0 {
		return pathy[0]
	}
	return ""
}

func r7(res *result, sl *jv, calib map[string]string) {
	files := map[string]bool{}
	land := jget(sl, "landing")
	if jIsStr(land) && csPyStrip(land.s) != "" {
		prim := landingPrimary(land.s)
		if prim != "" {
			files[prim] = true
		}
	}
	ctx := jget(sl, "context_needed")
	if ctx != nil && ctx.kind == 'a' {
		for _, it := range ctx.arr {
			if it.kind == 'o' && jIsStr(jget(it, "kind")) && jget(it, "kind").s == "file" && jNonempty(jget(it, "value")) {
				files[jAsText(jget(it, "value"))] = true
			}
		}
	}
	if len(files) > 1 {
		fkeys := []string{}
		for k := range files {
			fkeys = append(fkeys, k)
		}
		sort.Strings(fkeys)
		res.needRecursion("R7-NEEDS-RECURSION-MULTIFILE",
			fmt.Sprintf("落点文件数 = %d（>1）：%s ⇒ 判「需递归」，不打回（§3.4-R7）", len(files), strings.Join(fkeys, " , ")))
	}
	bound := calib["slice_lines_max"]
	if bound == "" {
		bound = calib["slice_line_bound"]
	}
	if bound == "" {
		res.note("R7-UNCALIBRATED",
			fmt.Sprintf("行数上界「未标定」：`%s` 无 `slice_lines_max` 键 ⇒ 记未标定，不猜（§3.4-R7 上界 = 本仓标定取值）", pyBasename(csCalibPath)))
		return
	}
	bnum, ok := parseIntOrFloat(bound)
	if !ok {
		res.note("R7-UNCALIBRATED", fmt.Sprintf("行数上界值不可解析（%s）⇒ 记未标定，不猜", bound))
		return
	}
	fkeys := []string{}
	for k := range files {
		fkeys = append(fkeys, k)
	}
	sort.Strings(fkeys)
	for _, c := range fkeys {
		p := c
		if !pyIsAbs(c) {
			p = pyJoin(csRepoRoot, c)
		}
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		n := countLines(string(data))
		if n > bnum {
			res.needRecursion("R7-NEEDS-RECURSION-LINES",
				fmt.Sprintf("落点 %s 现行行数 %d > 标定上界 %d ⇒ 判「需递归」，不打回（§3.4-R7）", c, n, bnum))
		}
	}
}

func parseIntOrFloat(s string) (int, bool) {
	f, err := strconv.ParseFloat(csPyStrip(s), 64)
	if err != nil {
		return 0, false
	}
	return int(f), true
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func parseDeps(d *jv) []string {
	deps := []string{}
	if d != nil && d.kind == 'a' {
		for _, x := range d.arr {
			if jNonempty(x) {
				deps = append(deps, jAsText(x))
			}
		}
	} else if jNonempty(d) {
		s := csPyStrip(jAsText(d))
		if s != "无" {
			for _, t := range splitIDs(s) {
				if t != "" {
					deps = append(deps, t)
				}
			}
		}
	}
	return deps
}

type sliceIn struct {
	path string
	obj  *jv
}

func r8(res *result, sl *jv, slID string, allSlices []sliceIn) {
	dep := jget(sl, "depends_on")
	deps := parseDeps(dep)
	if !jNonempty(dep) {
		res.note("R8-NO-DEPENDS-ON", "depends_on 缺失/为空：§1 表要求**无依赖写「无」**（空数组会被 R5 形态命中）")
	}
	if slID != "" && csContainsStr(deps, slID) {
		res.hit("R8", "R8-SELF-DEP", fmt.Sprintf("depends_on 含自身 slice_id（%s）⇒ 成环", slID),
			"删掉自身依赖；本片无依赖就写 `\"depends_on\": \"无\"`")
	}
	if len(allSlices) < 2 {
		res.note("R8-SINGLE", "单片模式：拓扑排序 / 重复检查不适用（片集合级，见 §3.4-R8）")
	} else {
		ids := map[string][]string{}
		for _, s := range allSlices {
			sid := jget(s.obj, "slice_id")
			if jNonempty(sid) {
				k := jAsText(sid)
				ids[k] = append(ids[k], s.path)
			}
		}
		for _, sid := range sortedKeys(ids) {
			paths := ids[sid]
			if len(paths) > 1 {
				res.hit("R8", "R8-DUP-SLICE-ID",
					fmt.Sprintf("slice_id `%s` 在集合里出现 %d 次：%s", sid, len(paths), strings.Join(paths, " , ")),
					"片 = 单子：一个 slice_id 只能有一片；给重复的那片换 id（或合并两片）")
			}
		}
		seen := map[string][]string{}
		for _, s := range allSlices {
			k0 := csPyStrip(jAsText(jget(s.obj, "landing")))
			k1 := csPyStrip(jAsText(jget(s.obj, "defect_id")))
			key := k0 + "\x00" + k1
			if k0 != "" || k1 != "" {
				seen[key] = append(seen[key], s.path)
			}
		}
		for _, key := range sortedKeys(seen) {
			paths := seen[key]
			if len(paths) > 1 {
				k0 := strings.SplitN(key, "\x00", 2)[0]
				res.hit("R8", "R8-DUP-LANDING-DEFECT",
					fmt.Sprintf("同一 (landing, defect_id) 重复出现 %d 次：%s（%s）", len(paths), strings.Join(paths, " , "), k0),
					"同一 landing+defect_id 只允许一片；合并重复片或改 landing/defect_id")
			}
		}
		graph := map[string][]string{}
		order := []string{}
		for _, s := range allSlices {
			sid := jAsText(jget(s.obj, "slice_id"))
			if sid == "" {
				sid = s.path
			}
			if _, exists := graph[sid]; !exists {
				order = append(order, sid)
			}
			d := jget(s.obj, "depends_on")
			graph[sid] = parseDeps(d)
		}
		cyc := findCycle(graph, order)
		if len(cyc) > 0 {
			res.hit("R8", "R8-CYCLE", fmt.Sprintf("depends_on 成环（拓扑排序失败）：%s", strings.Join(cyc, " → ")),
				"打断环：把环上某一片的 depends_on 指向环外（或拆片）；例：把 S3-C2 的 depends_on 改为 `\"无\"`")
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func findCycle(graph map[string][]string, order []string) []string {
	indeg := map[string]int{}
	for k := range graph {
		indeg[k] = 0
	}
	for _, k := range order {
		for _, d := range graph[k] {
			if _, ok := indeg[d]; ok {
				indeg[k]++
			}
		}
	}
	queue := []string{}
	for _, k := range order {
		if indeg[k] == 0 {
			queue = append(queue, k)
		}
	}
	seen := map[string]bool{}
	for len(queue) > 0 {
		n := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		seen[n] = true
		for _, k := range order {
			if csContainsStr(graph[k], n) {
				indeg[k]--
				if indeg[k] == 0 {
					queue = append(queue, k)
				}
			}
		}
	}
	left := []string{}
	for _, k := range order {
		if !seen[k] {
			left = append(left, k)
		}
	}
	if len(left) == 0 {
		return nil
	}
	path := []string{left[0]}
	cur := left[0]
	for i := 0; i <= len(left); i++ {
		deps := []string{}
		for _, d := range graph[cur] {
			if csContainsStr(left, d) {
				deps = append(deps, d)
			}
		}
		if len(deps) == 0 {
			break
		}
		nxt := deps[0]
		if csContainsStr(path, nxt) {
			idx := 0
			for i, p := range path {
				if p == nxt {
					idx = i
					break
				}
			}
			return append(append([]string{}, path[idx:]...), nxt)
		}
		path = append(path, nxt)
		cur = nxt
	}
	return left
}

func r9(res *result, allSlices []sliceIn, setOpen bool) {
	if len(allSlices) < 2 {
		res.note("R9-SINGLE", "单片模式：依赖闭包 / 停机判据不适用（片集合级，见 §3.4-R9）")
		res.note("R9-UNDONE", pyPartialRules["R9"][1])
		return
	}
	ids := map[string]bool{}
	for _, s := range allSlices {
		sid := jget(s.obj, "slice_id")
		if jNonempty(sid) {
			ids[jAsText(sid)] = true
		}
	}
	type missT struct{ p, t string }
	missing := []missT{}
	for _, s := range allSlices {
		deps := parseDeps(jget(s.obj, "depends_on"))
		for _, t := range deps {
			if !ids[t] {
				missing = append(missing, missT{s.path, t})
			}
		}
	}
	if setOpen {
		res.note("R9-SET-OPEN", fmt.Sprintf("--set-open：该批片不是完整集合 ⇒ 依赖闭包记「不可判」（未判：%d 个外指依赖）", len(missing)))
	} else if len(missing) > 0 {
		lim := missing
		if len(lim) > 5 {
			lim = lim[:5]
		}
		parts := []string{}
		for _, m := range lim {
			parts = append(parts, m.p+" → "+m.t)
		}
		res.hit("R9", "R9-CLOSURE-OPEN",
			fmt.Sprintf("依赖闭包未闭合：%d 个 depends_on 指向集合外的片（%s）", len(missing), strings.Join(parts, " ； ")),
			"把被依赖的片一并纳入本批（或在只查局部时加 `--set-open` 并登记「未判」）")
	}
	res.note("R9-UNDONE", pyPartialRules["R9"][1])
}

func isSnippetItem(x *jv) bool {
	if x != nil && x.kind == 'o' {
		return jIsStr(jget(x, "kind")) && jget(x, "kind").s == "snippet" && jNonempty(jget(x, "value"))
	}
	if jIsStr(x) {
		s := csPyStrip(x.s)
		return strings.HasPrefix(s, "原文片段") || strings.Contains(x.s, "原文片段：")
	}
	return false
}

func ctxValues(x *jv) string {
	if x != nil && x.kind == 'o' {
		return jAsText(jget(x, "value"))
	}
	return jAsText(x)
}

func r10(res *result, sl *jv) {
	ctx := jget(sl, "context_needed")
	if !jNonempty(ctx) {
		res.hit("R10", "R10-EMPTY-CONTEXT", "context_needed 缺失/为空（执行者可见上下文为零）",
			"补最小上下文：第一条 = 落点原文片段，再列目标文件与判据命令")
		return
	}
	items := []*jv{}
	if ctx.kind == 'a' {
		items = ctx.arr
	} else {
		items = []*jv{ctx}
	}
	anySnip := false
	for _, x := range items {
		if isSnippetItem(x) {
			anySnip = true
			break
		}
	}
	if !anySnip {
		res.hit("R10", "R10-NO-SNIPPET", "context_needed 里没有「落点原文片段」（自包含性缺一半）",
			"加一条 `{\"kind\": \"snippet\", \"value\": \"<落点那一行原文>\"}`（固定放最前）")
	}
	for _, x := range items {
		v := ctxValues(x)
		if strings.Contains(v, "设计稿") {
			res.hit("R10", "R10-SEE-DESIGN", fmt.Sprintf("context_needed 把设计稿当上下文：%s", runeSliceStr(v, 0, 50)),
				"把设计稿里那段原文**抄进**本字段，例：`{\"kind\":\"snippet\",\"value\":\"v2.1 §3.3 抬头固定句那一行原文\"}`")
		}
		if x.kind == 'o' && jNonempty(jget(x, "kind")) {
			if !csContainsStr(pyRegCtxKinds, jget(x, "kind").s) {
				res.hit("R10", "R10-CTX-KIND-UNKNOWN",
					fmt.Sprintf("context_needed 条目 kind=%s 未登记（登记：%s）", jget(x, "kind").s, strings.Join(pyRegCtxKinds, " · ")),
					"改用登记 kind，例：`{\"kind\":\"file\",\"value\":\"docs/…\"}`")
			}
		}
	}
	res.note("R10-UNDONE", pyPartialRules["R10"][1])
}

func r11(res *result, sl *jv, calib map[string]string) {
	ctx := jget(sl, "context_needed")
	items := []*jv{}
	if ctx != nil && ctx.kind == 'a' {
		items = ctx.arr
	} else if jNonempty(ctx) {
		items = []*jv{ctx}
	}
	if len(items) == 0 {
		res.note("R11-NO-CTX", "context_needed 空 ⇒ 顺序断言无对象（另见 R10）")
	} else if !isSnippetItem(items[0]) {
		res.hit("R11", "R11-SNIPPET-NOT-FIRST", "context_needed 首条不是「落点原文片段」（R11：landing 片段固定放最前）",
			"把落点原文片段移到首条，形态：`{\"kind\":\"snippet\",\"value\":\"<原文>\"}`")
	}
	order := []string{}
	for _, kv := range sl.obj {
		if FIELD_KEYS_SET[kv.k] || META_KEYS_SET[kv.k] {
			order = append(order, kv.k)
		}
	}
	ci := -1
	for i, k := range order {
		if k == "criteria" {
			ci = i
			break
		}
	}
	if ci >= 0 {
		late := []string{}
		for _, k := range pyR11AfterCriteria {
			if csContainsStr(order, k) {
				for i, o := range order {
					if o == k && i < ci {
						late = append(late, k)
					}
				}
			}
		}
		if len(late) > 0 {
			res.hit("R11", "R11-CRITERIA-NOT-FIRST",
				fmt.Sprintf("criteria 未固定放最前：字段键序里它排在 %s 之后", strings.Join(late, " / ")),
				fmt.Sprintf("把 criteria 移到 pre 栏之后、%s 之前（键序即执行者看到的顺序）", late[0]))
		}
	}
	cb := jget(sl, "context_budget")
	if !jNonempty(cb) {
		res.hit("R11", "R11-NO-CONTEXT-BUDGET", "context_budget 缺失/为空（执行者所需文件/token 上界缺）",
			"补 `{\"files\": 1, \"tokens\": 2000, \"source\": \"~/.zerg/egg-profiles/task-budget-calib.yaml\"}`")
		return
	}
	if cb.kind == 'o' {
		nums := []string{}
		for _, k := range []string{"files", "tokens"} {
			vv := jget(cb, k)
			if vv != nil && vv.kind == 'i' {
				nums = append(nums, k)
			}
		}
		if len(nums) == 0 {
			res.hit("R11", "R11-NO-BOUND", fmt.Sprintf("context_budget 没有数值上界（files / tokens）：%s", runeSliceStr(jAsText(cb), 0, 50)),
				"补数值上界，例：`{\"files\": 1, \"tokens\": 2000, \"source\": \"…task-budget-calib.yaml\"}`")
		}
		src := csPyStrip(jAsText(jget(cb, "source")))
		if src == "" {
			res.hit("R11", "R11-NO-SOURCE", "context_budget 缺出处（与 budget 同规：无出处判红）",
				"补 `\"source\": \"~/.zerg/egg-profiles/task-budget-calib.yaml\"`")
		} else if !strings.Contains(src, "task-budget-calib") && !strings.Contains(src, "标定档案") {
			res.hit("R11", "R11-SOURCE-NOT-CALIB", fmt.Sprintf("context_budget 的出处不指向标定档案：%s", runeSliceStr(src, 0, 50)),
				"把出处改为标定档案，例：`\"source\": \"~/.zerg/egg-profiles/task-budget-calib.yaml\"`")
		}
	} else if jIsStr(cb) {
		if !regexp.MustCompile(`\d`).MatchString(cb.s) {
			res.hit("R11", "R11-NO-BOUND", fmt.Sprintf("context_budget 文本里没有数值上界：%s", runeSliceStr(cb.s, 0, 50)),
				"写成 `{\"files\": 1, \"tokens\": 2000, \"source\": \"…task-budget-calib.yaml\"}`")
		}
		if !strings.Contains(cb.s, "task-budget-calib") && !strings.Contains(cb.s, "标定档案") {
			res.hit("R11", "R11-NO-SOURCE", fmt.Sprintf("context_budget 文本里没有标定出处：%s", runeSliceStr(cb.s, 0, 50)),
				"把出处改为标定档案，例：`出处 = ~/.zerg/egg-profiles/task-budget-calib.yaml`")
		}
	}
	if len(calib) == 0 {
		res.note("R11-UNCALIBRATED", "标定档案读不到 ⇒ context_budget 上界记「未标定」，不猜")
	}
}

func r12(res *result, sl *jv) {
	ec := jget(sl, "exit_cond")
	if !jNonempty(ec) {
		res.hit("R12", "R12-MISSING", "exit_cond 缺失/为空",
			"写成三项：`{\"success\": [\"C1\"], \"failure\": \"<可观察条件>\", "+
				"\"escalate\": {\"to\": \"Mr2109\", \"times\": 2, \"fields\": [\"片 id\", \"命令\", \"退出码\"]}}`")
		return
	}
	if jIsStr(ec) {
		res.hit("R12", "R12-NON-STRUCTURED",
			fmt.Sprintf("exit_cond 是自由文本（成功/失败/升级三项皆缺）：%s", runeSliceStr(ec.s, 0, 50)),
			"改成三项结构：成功引用 criteria id；失败写可观察条件；升级写对象+触发次数+记录字段")
		return
	}
	if ec.kind != 'o' {
		res.hit("R12", "R12-NON-STRUCTURED", fmt.Sprintf("exit_cond 不是对象（实为 %s）", pyTypeName(ec)),
			"改成 `{\"success\": [...], \"failure\": \"...\", \"escalate\": {...}}`")
		return
	}
	ids := []string{}
	crit := jget(sl, "criteria")
	if crit != nil && crit.kind == 'a' {
		for _, it := range crit.arr {
			if it.kind == 'o' && jNonempty(jget(it, "id")) {
				ids = append(ids, jAsText(jget(it, "id")))
			}
		}
	}
	succ := jget(ec, "success")
	if !jNonempty(succ) {
		res.hit("R12", "R12-MISSING-SUCCESS", "exit_cond 缺「成功」项（或为空）",
			"补 `\"success\": [\"C1\", \"C2\"]`，列出必须同时成立的 criteria id")
	} else {
		toks := []string{}
		if succ.kind == 'a' {
			for _, t := range succ.arr {
				if jNonempty(t) {
					toks = append(toks, jAsText(t))
				}
			}
		} else {
			for _, t := range splitIDs(jAsText(succ)) {
				if t != "" {
					toks = append(toks, t)
				}
			}
		}
		unknown := []string{}
		for _, t := range toks {
			if len(ids) > 0 && !csContainsStr(ids, t) {
				unknown = append(unknown, t)
			}
		}
		if len(ids) == 0 {
			res.hit("R12", "R12-SUCCESS-NO-CRITERIA",
				"exit_cond.success 引用 criteria id，但本片 criteria 没有可用 id",
				"先给每条 criteria 一个 id（例 C1），再在 success 里引用它")
		} else if len(unknown) > 0 {
			res.hit("R12", "R12-SUCCESS-UNKNOWN-ID",
				fmt.Sprintf("exit_cond.success 引用了不存在的 criteria id：%s（现存：%s）", strings.Join(unknown, " / "), strings.Join(ids, " · ")),
				fmt.Sprintf("把 success 改成现存 id，例：`\"success\": [\"%s\"]`", ids[0]))
		}
	}
	if !jNonempty(jget(ec, "failure")) {
		res.hit("R12", "R12-MISSING-FAILURE", "exit_cond 缺「失败」（可观察条件）",
			"补 `\"failure\": \"<命令输出与期望不符 / 负控未由绿变红>，附命令、退出码、原始输出\"`")
	}
	esc := jget(ec, "escalate")
	if !jNonempty(esc) {
		res.hit("R12", "R12-MISSING-ESCALATE", "exit_cond 缺「升级」（对象+触发次数+记录字段）",
			"补 `\"escalate\": {\"to\": \"Mr2109\", \"times\": 2, \"fields\": [\"片 id\", \"命令\", \"退出码\", \"原文\", \"时间\"]}`")
	} else if esc.kind == 'o' {
		if !jNonempty(jget(esc, "to")) {
			res.hit("R12", "R12-BAD-ESCALATE", "exit_cond.escalate 缺升级对象（to）", "补 `\"to\": \"Mr2109\"`")
		}
		t := jget(esc, "times")
		okTimes := false
		if t != nil && t.kind == 'i' && isIntLiteral(t.num) {
			if n, err := strconv.Atoi(t.num); err == nil && n >= 1 {
				okTimes = true
			}
		}
		if !okTimes {
			res.hit("R12", "R12-BAD-ESCALATE", fmt.Sprintf("exit_cond.escalate 的触发次数（times）不是 ≥1 的整数：%s", jAsText(t)),
				"补 `\"times\": 2`（同一 criteria 连续失败次数）")
		}
		f := jget(esc, "fields")
		if !jNonempty(f) || f.kind != 'a' {
			res.hit("R12", "R12-BAD-ESCALATE", "exit_cond.escalate 缺记录字段（fields，非空列表）",
				"补 `\"fields\": [\"片 id\", \"命令\", \"退出码\", \"原文\", \"时间\"]`")
		}
	} else {
		res.hit("R12", "R12-BAD-ESCALATE", "exit_cond.escalate 不是对象",
			"改成 `{\"to\": \"Mr2109\", \"times\": 2, \"fields\": [...]}`")
	}
}

// ── 非 R 判据（模板 §1 表 / §2 / §3.3）──

func clauseOpenQuestions(res *result, sl *jv) {
	v := jget(sl, "open_questions")
	if v == nil || (v.kind == 'n' && jpresent(sl, "open_questions")) {
		if v == nil {
			return
		}
	}
	if v == nil {
		return
	}
	if v.kind == 'n' {
		// py: v is None ⇒ return（缺件已由必填表命中）
		return
	}
	if jIsStr(v) {
		s := csPyStrip(v.s)
		if s == "" || (s != "无" && s != "NEEDS-CLARIFICATION") {
			res.hit("§1-pre-open_questions", "S1-PRE-OPENQ",
				fmt.Sprintf("open_questions 是自由文本（无则必须写「无」）：%s", runeSliceStr(v.s, 0, 40)),
				"无未决就写 `\"open_questions\": \"无\"`（留空 ≠ 写「无」）；有疑问写成列表，每条 ≤1 项")
		}
		return
	}
	if v.kind == 'a' {
		if len(v.arr) == 0 {
			res.hit("§1-pre-open_questions", "S1-PRE-OPENQ", "open_questions 是空列表（留空 ≠ 写「无」）",
				"写成 `\"open_questions\": \"无\"`")
			return
		}
		for idx, item := range v.arr {
			t := jAsText(item)
			nq := strings.Count(t, "？") + strings.Count(t, "?")
			nv := countVerbs(t)
			if nq > 1 || nv > 1 {
				res.hit("§1-pre-open_questions", "S1-PRE-OPENQ-MULTI",
					fmt.Sprintf("open_questions[%d] 不止一项（问号 %d / 动词 %d）：%s", idx, nq, nv, runeSliceStr(t, 0, 40)),
					"拆成多条，每条只留一个问题、一个动作；例：「是否需要先落 S3-C0？」")
			}
		}
		return
	}
	res.hit("§1-pre-open_questions", "S1-PRE-OPENQ",
		fmt.Sprintf("open_questions 形态不明（实为 %s）", pyTypeName(v)),
		"写「无」或写列表，例：`[\"是否需要先落 S3-C0？\"]`")
}

func clauseOutputFormat(res *result, sl *jv) {
	v := jget(sl, "output_format")
	if !jNonempty(v) {
		return
	}
	txt := jAsText(v)
	if !strings.Contains(txt, "负控") || !strings.Contains(txt, "红") {
		res.hit("§1-post-output_format", "S1-POST-NEGCTRL",
			"output_format 缺「改动前判据必红」的负控证据",
			"补负控段，例：「② 负控（改动前必红）：把 X 改坏后重跑同三条命令，必须红（已实跑：三条由 1 变 0）」")
	}
}

func firstWord(s string) string {
	fs := strings.Fields(s)
	if len(fs) == 0 {
		return ""
	}
	return fs[0]
}

func clauseSideEffects(res *result, sl *jv) {
	se := jget(sl, "side_effects")
	if !jNonempty(se) {
		return
	}
	if jIsStr(se) {
		if firstWord(csPyStrip(se.s)) != "" && !csContainsStr(pySideEffectEnum, firstWord(csPyStrip(se.s))) {
			res.hit("§1-inv-side_effects", "S1-INV-SIDEEFFECTS",
				fmt.Sprintf("side_effects 不在枚举内：%s（枚举：%s）", runeSliceStr(se.s, 0, 40), strings.Join(pySideEffectEnum, " · ")),
				"改成五值之一，例：`\"side_effects\": \"写工作树\"`")
		}
		return
	}
	if se.kind == 'a' {
		bad := []string{}
		for _, x := range se.arr {
			t := csPyStrip(jAsText(x))
			if t != "" && !csContainsStr(pySideEffectEnum, firstWord(t)) {
				bad = append(bad, jAsText(x))
			}
		}
		if len(bad) > 0 {
			res.hit("§1-inv-side_effects", "S1-INV-SIDEEFFECTS",
				fmt.Sprintf("side_effects 含枚举外值：%s（枚举：%s）", strings.Join(bad, " / "), strings.Join(pySideEffectEnum, " · ")),
				"改成五值之一，例：`\"side_effects\": [\"写工作树\"]`")
		}
	}
}

func clauseEscalateIf(res *result, sl *jv) {
	if jNonempty(jget(sl, "side_effects")) && !jNonempty(jget(sl, "escalate_if")) {
		res.hit("§1-inv-escalate_if", "S1-INV-ESCALATEIF",
			"有 side_effects 却没配 escalate_if（表外字段缺 ⇒ 不合格）",
			"补 `\"escalate_if\": \"需要写工作树之外的文件 / 需要 git 写 / 需要删除 ⇒ 升级 Mr2109\"`")
	}
}

func clauseBudget(res *result, sl *jv) {
	b := jget(sl, "budget")
	if !jNonempty(b) {
		return
	}
	txt := jAsText(b)
	src := ""
	if b.kind == 'o' {
		src = jAsText(jget(b, "source"))
	} else {
		if strings.Contains(txt, "出处") || strings.Contains(txt, "task-budget-calib") || strings.Contains(txt, "标定档案") {
			src = txt
		}
	}
	if csPyStrip(src) == "" || (!strings.Contains(src, "task-budget-calib") && !strings.Contains(src, "标定档案")) {
		res.hit("§1-inv-budget", "S1-INV-BUDGET",
			fmt.Sprintf("budget 没有标定出处（无出处的常数判红）：%s", runeSliceStr(txt, 0, 50)),
			"补出处，例：`\"budget\": {\"value\": \"wall_s_max 43.6\", "+
				"\"source\": \"~/.zerg/egg-profiles/task-budget-calib.yaml\"}`")
	} else if !regexp.MustCompile(`\d`).MatchString(txt) {
		res.note("S1-INV-BUDGET-NOVALUE", "budget 有出处但没写出具体数值（登记：不判红，只注记）")
	}
}

func clauseAtomicity(res *result, sl *jv, calib map[string]string) {
	v := jget(sl, "atomicity")
	if !jNonempty(v) {
		return
	}
	txt := jAsText(v)
	if !strings.Contains(txt, "闭包") || (!strings.Contains(txt, "一次调用") && !strings.Contains(txt, "一次")) {
		res.hit("§1-inv-atomicity", "S1-INV-ATOMICITY",
			fmt.Sprintf("atomicity 未同时给出「闭包」与「一次调用内可判」：%s", runeSliceStr(txt, 0, 50)),
			"改写成：`\"atomicity\": \"落在执行者标定技能/门禁闭包内，且 criteria 可在一次调用内判定"+
				"（<哪几条、各一次什么命令>）\"`")
	}
	found := false
	for k := range calib {
		if strings.Contains(k, "closure") || strings.Contains(k, "skill") {
			found = true
			break
		}
	}
	if !found {
		res.note("S1-INV-ATOMICITY-UNCALIBRATED",
			"「标定技能/门禁闭包」字段在标定档案里不存在 ⇒ 该条记「未标定」，不猜")
	}
}

func clauseSliceID(res *result, sl *jv) {
	sid := jget(sl, "slice_id")
	if !jNonempty(sid) {
		return
	}
	if jIsStr(sid) {
		s := csPyStrip(sid.s)
		if s == "待定" || s == "TBD" || s == "TODO" || s == "NEEDS-CLARIFICATION" {
			res.hit("§1-inv-slice_id", "S1-INV-SLICEID",
				fmt.Sprintf("slice_id = 「%s」（占位，未定）", s),
				"填真实单子 id，例：`\"slice_id\": \"S3-C1\"`（片 = 单子：同一实体同一 id）")
		}
	}
	tid := jget(sl, "task_id")
	if jNonempty(tid) && jNonempty(sid) && jAsText(tid) != jAsText(sid) {
		res.hit("§1-inv-slice_id", "S1-INV-SLICEID-TASK",
			fmt.Sprintf("slice_id（%s）≠ task_id（%s）：两名两体", jAsText(sid), jAsText(tid)),
			fmt.Sprintf("让两者一致，例：把 `\"task_id\"` 改成 `\"%s\"`", jAsText(sid)))
	}
}

func clauseConflict(res *result, sl *jv) {
	crit := jget(sl, "criteria")
	txt := jAsText(crit)
	hasFull := pyFullscanRe.MatchString(txt)
	btxt := jAsText(jget(sl, "budget"))
	hasBound := regexp.MustCompile(`\d`).MatchString(btxt)
	conflicts := jget(sl, "conflicts")
	if hasFull && hasBound {
		if !jNonempty(conflicts) {
			res.hit("§2-conflict", "S2-CONFLICT",
				"criteria 含全量形态（`./...`/全量/all）而 budget 给了数值上界 ⇒ 潜在冲突未标 CONFLICT",
				"标出冲突：`\"conflicts\": [{\"a\": \"criteria\", \"b\": \"budget\", "+
					"\"where\": \"<哪一句与哪个上界冲突>\"}]`（标记不是解决，裁量权交回上游）")
		} else {
			items := []*jv{}
			if conflicts.kind == 'a' {
				items = conflicts.arr
			} else {
				items = []*jv{conflicts}
			}
			for i, c := range items {
				if c.kind == 'o' {
					miss := []string{}
					for _, k := range []string{"a", "b", "where"} {
						if !jNonempty(jget(c, k)) {
							miss = append(miss, k)
						}
					}
					if len(miss) > 0 {
						res.hit("§2-conflict", "S2-CONFLICT-INCOMPLETE",
							fmt.Sprintf("conflicts[%d] 没写清是哪两个字段、冲突在哪一句（缺 %s）", i, strings.Join(miss, " / ")),
							"补全三键，例：`{\"a\": \"criteria\", \"b\": \"budget\", \"where\": \"全量 go test 必须绿 vs wall_s_max 43.6\"}`")
					}
				} else if !jNonempty(c) {
					res.hit("§2-conflict", "S2-CONFLICT-INCOMPLETE", fmt.Sprintf("conflicts[%d] 为空", i),
						"写成 `{\"a\": \"criteria\", \"b\": \"budget\", \"where\": \"<冲突句>\"}`")
				}
			}
		}
	}
	if jNonempty(conflicts) && !hasFull {
		res.note("S2-CONFLICT-MARKED", "本片已标 CONFLICT（本检查器不再追判）")
	}
}

func clauseInherit(res *result, child, parent *jv, unionCmds []string) {
	pctx := map[string]bool{}
	if pc := jget(parent, "context_needed"); pc != nil && pc.kind == 'a' {
		for _, x := range pc.arr {
			pctx[ctxValues(x)] = true
		}
	}
	cctx := map[string]bool{}
	if cc := jget(child, "context_needed"); cc != nil && cc.kind == 'a' {
		for _, x := range cc.arr {
			cctx[ctxValues(x)] = true
		}
	}
	loseCtx := []string{}
	for k := range pctx {
		if !cctx[k] {
			loseCtx = append(loseCtx, k)
		}
	}
	sort.Strings(loseCtx)
	pq := jget(parent, "open_questions")
	cq := jget(child, "open_questions")
	pqList := asListOrOne(pq)
	cqList := asListOrOne(cq)
	cqTexts := []string{}
	for _, y := range cqList {
		cqTexts = append(cqTexts, jAsText(y))
	}
	loseQ := []*jv{}
	for _, x := range pqList {
		if !csContainsStr(cqTexts, jAsText(x)) {
			loseQ = append(loseQ, x)
		}
	}
	if len(loseCtx) > 0 || len(loseQ) > 0 {
		parts := append([]string{}, loseCtx...)
		for _, x := range loseQ {
			parts = append(parts, jAsText(x))
		}
		res.hit("§3.3-inherit", "S33-INHERIT-PRE",
			fmt.Sprintf("子 pre ⊉ 父 pre：丢了 %s", runeSliceStr(strings.Join(parts, " / "), 0, 80)),
			"把父片 pre 的条目照抄进子片 context_needed / open_questions（子 pre ⊇ 父 pre）")
	}
	pcmds := map[string]bool{}
	if pc := jget(parent, "criteria"); pc != nil && pc.kind == 'a' {
		for _, it := range pc.arr {
			if it.kind == 'o' {
				pcmds[csPyStrip(jAsText(jget(it, "cmd")))] = true
			}
		}
	}
	if cc := jget(child, "criteria"); cc != nil && cc.kind == 'a' {
		extra := []string{}
		for _, it := range cc.arr {
			if it.kind == 'o' {
				c := csPyStrip(jAsText(jget(it, "cmd")))
				if !pcmds[c] {
					extra = append(extra, c)
				}
			}
		}
		if len(extra) > 0 {
			res.hit("§3.3-inherit", "S33-INHERIT-POST",
				fmt.Sprintf("子 post ⊄ 父 post：子多出 %d 条父片没有的判据（%s）", len(extra), runeSliceStr(extra[0], 0, 50)),
				"删掉子片多出的判据，或让父片先承接它（子 post ⊆ 父 post：防子片绕过父片边界）")
		}
	}
	ph := jget(parent, "hard_limits")
	ch := jget(child, "hard_limits")
	phList := asTextList(ph)
	chList := asTextList(ch)
	extraH := []string{}
	for _, x := range chList {
		if !csContainsStr(phList, x) {
			extraH = append(extraH, x)
		}
	}
	if len(extraH) > 0 {
		res.hit("§3.3-inherit", "S33-INHERIT-LIMITS",
			fmt.Sprintf("子 hard_limits ⊄ 父 hard_limits：多出 %s", runeSliceStr(extraH[0], 0, 60)),
			"删掉子片多出的 hard_limits，或先写进父片（子 hard_limits ⊆ 父 hard_limits）")
	}
	pec := jget(parent, "exit_cond")
	if pec != nil && pec.kind == 'o' {
		psucc := jget(pec, "success")
		pcrit := jget(parent, "criteria")
		if psucc != nil && psucc.kind == 'a' && pcrit != nil && pcrit.kind == 'a' {
			pidCmds := map[string]string{}
			for _, it := range pcrit.arr {
				if it.kind == 'o' && jNonempty(jget(it, "id")) {
					pidCmds[jAsText(jget(it, "id"))] = csPyStrip(jAsText(jget(it, "cmd")))
				}
			}
			ccmds := map[string]bool{}
			if cc := jget(child, "criteria"); cc != nil && cc.kind == 'a' {
				for _, it := range cc.arr {
					if it.kind == 'o' {
						ccmds[csPyStrip(jAsText(jget(it, "cmd")))] = true
					}
				}
			}
			scope := map[string]bool{}
			for k := range ccmds {
				scope[k] = true
			}
			for _, k := range unionCmds {
				scope[k] = true
			}
			miss := []string{}
			for _, t := range psucc.arr {
				ts := jAsText(t)
				if cm, ok := pidCmds[ts]; ok && !scope[cm] {
					miss = append(miss, ts)
				}
			}
			if len(miss) > 0 {
				res.hit("§3.3-inherit", "S33-INHERIT-EXITCOND",
					fmt.Sprintf("父 exit_cond 的成功项 %s 没有被子集合承接", strings.Join(miss, " / ")),
					"把父片 success 里那几条判据放进某一子片的 criteria（父 exit_cond 由子集合共同满足）")
			}
		}
	}
}

func asListOrOne(v *jv) []*jv {
	if v != nil && v.kind == 'a' {
		return v.arr
	}
	if jNonempty(v) && !(jIsStr(v) && csPyStrip(v.s) == "无") {
		return []*jv{v}
	}
	return nil
}

func asTextList(v *jv) []string {
	out := []string{}
	if v != nil && v.kind == 'a' {
		for _, x := range v.arr {
			out = append(out, jAsText(x))
		}
	} else if jNonempty(v) {
		out = append(out, jAsText(v))
	}
	return out
}

// ── 一片 / 一批的判定 ──

type optsT struct {
	paths       []string
	mode        string
	setOpen     bool
	disable     map[string]bool
	disableList []string
	quiet       bool
}

func checkSlice(obj *jv, name string, opts *optsT, allSlices []sliceIn, calib map[string]string) *result {
	res := &result{name: name}
	sid := ""
	if v := jget(obj, "slice_id"); jNonempty(v) {
		sid = jAsText(v)
	}
	for _, rf := range pyRequiredFields {
		if !jpresent(obj, rf[0]) {
			res.hit("§1", "S1-MISSING-FIELD",
				fmt.Sprintf("必填字段缺失：%s（依据 %s）", rf[0], rf[1]),
				fmt.Sprintf("把 %s 补进片文件（模板 §1 表：必填）；未决写 `NEEDS-CLARIFICATION`", rf[0]))
		}
	}
	if !opts.disable["R0"] {
		r0(res, obj)
	}
	if !opts.disable["R1"] {
		r1(res, obj)
	}
	if !opts.disable["R2"] {
		r2(res, obj)
	}
	if !opts.disable["R3"] {
		r3(res, obj)
	}
	if !opts.disable["R4"] {
		for _, f := range []string{"landing", "the_one_thing", "output_format", "atomicity", "escalate_if", "exit_cond", "side_effects", "conflicts"} {
			r4Scan(res, f, jAsText(jget(obj, f)))
		}
		r4Scan(res, "budget", jAsText(jget(obj, "budget")))
		r4Scan(res, "context_budget", jAsText(jget(obj, "context_budget")))
		if c := jget(obj, "criteria"); jIsStr(c) {
			r4Scan(res, "criteria", c.s)
		}
		if c := jget(obj, "criteria"); c != nil && c.kind == 'a' {
			for i, it := range c.arr {
				if it.kind == 'o' {
					r4Scan(res, fmt.Sprintf("criteria[%d].cmd", i), jAsText(jget(it, "cmd")))
					r4Scan(res, fmt.Sprintf("criteria[%d].expect", i), jAsText(jget(it, "expect")))
					r4Scan(res, fmt.Sprintf("criteria[%d].state", i), jAsText(jget(it, "state")))
				}
			}
		}
		if g := jget(obj, "gate_cmds"); g != nil && g.kind == 'a' {
			for i, it := range g.arr {
				if it.kind == 'o' {
					r4Scan(res, fmt.Sprintf("gate_cmds[%d].cmd", i), jAsText(jget(it, "cmd")))
				}
			}
		}
		if c := jget(obj, "context_needed"); c != nil && c.kind == 'a' {
			for i, it := range c.arr {
				r4Scan(res, fmt.Sprintf("context_needed[%d]", i), ctxValues(it))
			}
		}
		if c := jget(obj, "open_questions"); c != nil && c.kind == 'a' {
			for i, it := range c.arr {
				r4Scan(res, fmt.Sprintf("open_questions[%d]", i), jAsText(it))
			}
		}
		if c := jget(obj, "hard_limits"); c != nil && c.kind == 'a' {
			for i, it := range c.arr {
				r4Scan(res, fmt.Sprintf("hard_limits[%d]", i), jAsText(it))
			}
		}
	}
	if !opts.disable["R5"] {
		r5(res, obj)
	}
	if !opts.disable["R6"] {
		r6(res, obj)
	}
	if !opts.disable["R7"] {
		r7(res, obj, calib)
	}
	if !opts.disable["R8"] {
		r8(res, obj, sid, allSlices)
	}
	if !opts.disable["R10"] {
		r10(res, obj)
	}
	if !opts.disable["R11"] {
		r11(res, obj, calib)
	}
	if !opts.disable["R12"] {
		r12(res, obj)
	}
	clauseOpenQuestions(res, obj)
	clauseOutputFormat(res, obj)
	clauseSideEffects(res, obj)
	clauseEscalateIf(res, obj)
	clauseBudget(res, obj)
	clauseAtomicity(res, obj, calib)
	clauseSliceID(res, obj)
	clauseConflict(res, obj)
	return res
}

func resolveParent(pref *jv, byID map[string]sliceIn) *jv {
	if pref != nil && pref.kind == 'o' {
		return pref
	}
	if jIsStr(pref) {
		if s, ok := byID[pref.s]; ok {
			return s.obj
		}
		cand := pref.s
		if !pyIsAbs(pref.s) {
			cand = pyJoin(csRepoRoot, pref.s)
		}
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			obj, _ := loadSliceFile(cand)
			return obj
		}
	}
	return nil
}

func evaluateSet(allS []sliceIn, opts *optsT, calib map[string]string) []*result {
	byID := map[string]sliceIn{}
	for _, s := range allS {
		sid := jget(s.obj, "slice_id")
		if jNonempty(sid) {
			byID[jAsText(sid)] = s
		}
	}
	results := []*result{}
	for _, s := range allS {
		results = append(results, checkSlice(s.obj, s.path, opts, allS, calib))
	}
	if !opts.disable["R9"] {
		r9(singleJoin(results), allS, opts.setOpen)
	}
	childrenCmds := map[string]map[string]bool{}
	for _, s := range allS {
		pref := jget(s.obj, "parent")
		if !jNonempty(pref) {
			continue
		}
		pobj := resolveParent(pref, byID)
		pid := jAsText(pref)
		if pobj != nil {
			pid = jAsText(jget(pobj, "slice_id"))
		}
		bucket := childrenCmds[pid]
		if bucket == nil {
			bucket = map[string]bool{}
			childrenCmds[pid] = bucket
		}
		if c := jget(s.obj, "criteria"); c != nil && c.kind == 'a' {
			for _, it := range c.arr {
				if it.kind == 'o' {
					bucket[csPyStrip(jAsText(jget(it, "cmd")))] = true
				}
			}
		}
	}
	for i, s := range allS {
		pref := jget(s.obj, "parent")
		if !jNonempty(pref) {
			continue
		}
		pobj := resolveParent(pref, byID)
		if pobj == nil {
			results[i].hit("§3.3-inherit", "S33-INHERIT-NO-PARENT",
				fmt.Sprintf("子片声明了 parent 但父片读不到：%s", runeSliceStr(jAsText(pref), 0, 50)),
				"把父片路径写对（相对仓根或绝对路径），或把父片对象内联进 `parent`")
			continue
		}
		pid := jAsText(jget(pobj, "slice_id"))
		uc := []string{}
		for k := range childrenCmds[pid] {
			uc = append(uc, k)
		}
		sort.Strings(uc)
		clauseInherit(results[i], s.obj, pobj, uc)
	}
	return results
}

func singleJoin(results []*result) *result {
	if len(results) == 0 {
		return &result{name: "(空)"}
	}
	return results[0]
}

// ── strict 等价 ──

func isOff(v *jv) bool {
	if v == nil {
		return false
	}
	if v.kind == 'b' {
		return !v.b
	}
	if v.kind == 'i' && v.num == "0" {
		return true
	}
	if jIsStr(v) {
		return csContainsStr(pyOffWords, strings.ToLower(csPyStrip(v.s)))
	}
	if v.kind == 'o' {
		inner := jget(v, "enabled")
		if inner == nil {
			inner = jget(v, "state")
		}
		if inner == nil {
			inner = &jv{kind: 'b', b: true}
		}
		return isOff(inner)
	}
	return false
}

func strictErrorsForSlice(obj *jv, path string) []string {
	errs := []string{}
	if obj == nil || obj.kind != 'o' {
		return []string{fmt.Sprintf("%s: [STRICT-NOT-OBJECT] 片文件顶层不是对象（实为 %s）", path, pyTypeName(obj))}
	}
	for _, kv := range obj.obj {
		if !pyKnownSliceKeys[kv.k] {
			keys := sortedKeys(pyKnownSliceKeys)
			errs = append(errs, fmt.Sprintf("%s: [STRICT-UNKNOWN-FIELD] 未知字段键 `%s`（已知键 %d 个：%s；新增字段必须先在模板 §1 表登记）",
				path, kv.k, len(pyKnownSliceKeys), strings.Join(keys, " · ")))
		}
	}
	rules := jget(obj, "rules")
	if rules != nil && rules.kind != 'n' {
		if rules.kind != 'o' {
			errs = append(errs, fmt.Sprintf("%s: [STRICT-BAD-RULES] `rules` 段必须是对象（键 = 规则键，值 = 开关）", path))
		} else {
			for _, rk := range rules.obj {
				if !pyRuleKeysSet[rk.k] {
					errs = append(errs, fmt.Sprintf("%s: [STRICT-UNKNOWN-RULE] 未知规则键 `%s`（已知规则键：%s）",
						path, rk.k, strings.Join(pyRuleKeys, " · ")))
				} else if isOff(rk.v) && pyRegisteredDisable[rk.k] == "" {
					errs = append(errs, fmt.Sprintf("%s: [STRICT-UNREGISTERED-DISABLE] 片内试图禁用 `%s` —— **未登记**（§3.5：禁用任何规则必须显式登记并打印；可登记禁用的键：%s）",
						path, rk.k, strings.Join(sortedKeys(pyRegisteredDisable), " · ")))
				}
			}
		}
	}
	return errs
}

func validateDisables(keys []string) []string {
	errs := []string{}
	for _, k := range keys {
		if !pyRuleKeysSet[k] {
			errs = append(errs, fmt.Sprintf("[STRICT-UNKNOWN-RULE] --disable-rule 给了未知规则键 `%s`（已知：%s）", k, strings.Join(pyRuleKeys, " · ")))
		} else if _, ok := pyDisabledRules[k]; ok {
			errs = append(errs, fmt.Sprintf("[STRICT-ALREADY-DISABLED] `%s` 已是永久禁用项（登记见 DISABLED_RULES / --list-rules），重复禁用请求不登记", k))
		} else if pyRegisteredDisable[k] == "" {
			errs = append(errs, fmt.Sprintf("[STRICT-UNREGISTERED-DISABLE] `%s` 未被登记为可禁用（§3.5：禁用任何规则必须**先显式登记**；已登记的键：%s）", k, strings.Join(sortedKeys(pyRegisteredDisable), " · ")))
		}
	}
	return errs
}

// ── 打印（登记块 / 片报告 / 规则清单）──

func loadCalib() (map[string]string, string) {
	out := map[string]string{}
	fi, err := os.Stat(csCalibPath)
	if err != nil || fi.IsDir() {
		return out, fmt.Sprintf("档案不存在: %s", csCalibPath)
	}
	data, err := os.ReadFile(csCalibPath)
	if err != nil {
		return out, fmt.Sprintf("档案读不出: %v", err)
	}
	for _, ln := range strings.Split(string(data), "\n") {
		ln = csPyStrip(ln)
		if ln == "" || strings.HasPrefix(ln, "#") || !strings.Contains(ln, ":") {
			continue
		}
		kv := strings.SplitN(ln, ":", 2)
		out[csPyStrip(kv[0])] = csPyStrip(kv[1])
	}
	return out, ""
}

func calibNote(calib map[string]string) string {
	if len(calib) == 0 {
		return fmt.Sprintf("标定档案：读不到（%s）⇒ 与档案绑定的上界一律记「未标定」，不猜", csCalibPath)
	}
	return fmt.Sprintf("标定档案：%s（现读键：%s）", csCalibPath, strings.Join(sortedKeys(calib), " · "))
}

func probeThresholds(calib map[string]string) (map[string]float64, map[string]string) {
	vals := map[string]float64{}
	srcs := map[string]string{}
	for k, d := range pyProbeDefaults {
		v, ok := calib[k]
		if !ok || v == "" {
			vals[k] = d
			srcs[k] = fmt.Sprintf("登记默认（档案无 `%s` 键）", k)
			continue
		}
		f, err := strconv.ParseFloat(csPyStrip(v), 64)
		if err != nil {
			vals[k] = d
			srcs[k] = "档案值不可解析 ⇒ 回退登记默认"
			continue
		}
		if strings.HasSuffix(k, "_max") {
			vals[k] = f
		} else {
			vals[k] = float64(int(f))
		}
		srcs[k] = "标定档案"
	}
	return vals, srcs
}

func printRegistration(stdout io.Writer, opts *optsT, calib map[string]string, quiet bool) {
	if quiet {
		return
	}
	fmt.Fprintln(stdout, "════════════════════════════════════════════════════════════════")
	fmt.Fprintf(stdout, "切片判据检查器 check-slice.py · 仓根 %s\n", csRepoRoot)
	fmt.Fprintln(stdout, "共同规则表：Zerg-内部文档/01-设计/切片合同-模板.md（§1 字段表 + §2 冲突优先级 + §3.3 继承四条 + §3.4 R0–R13）")
	nEn := 0
	for _, k := range pyRuleKeys {
		if pyRuleState[k] == "启用" {
			nEn++
		}
	}
	fmt.Fprintf(stdout, "── 规则登记（启用 %d · 部分 %d · 路由 %d · 禁用 %d）──\n", nEn, len(pyPartialRules), len(pyRouteRules), len(pyDisabledRules))
	markMap := map[string]string{"启用": "  ", "部分": "～", "路由": "→", "禁用": "×"}
	for _, r := range pyRuleTable {
		line := fmt.Sprintf("%s [%-2s] %-4s %-36s", markMap[r.State], r.Num, r.State, r.Key)
		if d, ok := pyDisabledRules[r.Key]; ok {
			line += fmt.Sprintf("  理由：%s｜依据：%s｜影响：%s", d[0], d[1], d[3])
		} else if d, ok := pyPartialRules[r.Num]; ok {
			line += fmt.Sprintf("  未判：%s", runeSliceStr(d[1], 0, 70))
		} else if rr, ok := pyRouteRules[r.Num]; ok {
			line += fmt.Sprintf("  %s", runeSliceStr(rr, 0, 70))
		}
		fmt.Fprintln(stdout, line)
		if opts.disable[r.Num] {
			fmt.Fprintf(stdout, "      ✱ 本次运行**已登记禁用**（--disable-rule %s）⇒ 本规则不参与判定\n", r.Num)
		}
	}
	fmt.Fprintf(stdout, "  [禁用-登记表] 可被 `--disable-rule` 关掉的键（预登记）：%s\n", joinOr(sortedKeys(pyRegisteredDisable), " · "))
	for _, k := range sortedKeys(pyRegisteredDisable) {
		fmt.Fprintf(stdout, "       · %s —— %s\n", k, pyRegisteredDisable[k])
	}
	fmt.Fprintln(stdout, "── 判据口径来源（登记，不静默）──")
	fmt.Fprintf(stdout, "  R0 动词词表：%d 条（最长匹配、非重叠）\n", len(pyRegVerbs))
	n4 := map[string]int{}
	for _, row := range pyRegR4 {
		n4[row.Layer]++
	}
	fmt.Fprintf(stdout, "  R4 三层词表：含糊 %d · 可选 %d · 主观 %d（**每条都带改写建议**；给不出 fix 的命中 ⇒ 拒判 rc=2）\n",
		n4["含糊"], n4["可选"], n4["主观"])
	fmt.Fprintf(stdout, "  R5 占位词表：%s\n", strings.Join(pyRegPlaceholder, " · "))
	fmt.Fprintf(stdout, "  R6 expect 取值域：%s（登记口径；exit-only + 登记假绿形态 ⇒ 判红）\n", strings.Join(pyRegExpectKeys, " · "))
	trapParts := []string{}
	for _, kv := range pyRegExitTrap {
		trapParts = append(trapParts, fmt.Sprintf("%s（%s）", kv.A, kv.B))
	}
	fmt.Fprintf(stdout, "  R6 登记的假绿形态：%s\n", strings.Join(trapParts, " · "))
	fmt.Fprintf(stdout, "  R11 顺序口径：context_needed 首条 = 落点原文片段；criteria 须在 %s 之前 等之前\n", strings.Join(pyR11AfterCriteria[:6], " · "))
	vals, srcs := probeThresholds(calib)
	fmt.Fprintf(stdout, "  探针门槛（§3.6/§3.5）：含糊-违规 ≥%s · 合法 ≥%s · 假红率 ≤%s\n",
		fmtNum(vals["probe_vague_min"]), fmtNum(vals["probe_legal_min"]), fmtNum(vals["probe_fp_rate_max"]))
	fmt.Fprintf(stdout, "      取值来源：含糊 %s · 合法 %s · 假红 %s\n", srcs["probe_vague_min"], srcs["probe_legal_min"], srcs["probe_fp_rate_max"])
	fmt.Fprintf(stdout, "  %s\n", calibNote(calib))
	optNames := []string{}
	for _, f := range pyOptionalFields {
		optNames = append(optNames, f[0])
	}
	fmt.Fprintf(stdout, "  片文件已知键（strict：别的键 ⇒ rc=2）：%d 个 —— 必填 %d + 表外 1 + **可选 %d**（%s）+ 元键 %s\n",
		len(pyKnownSliceKeys), len(pyRequiredFields), len(pyOptionalFields), strings.Join(optNames, " · "), strings.Join(pyMetaKeys, " · "))
	fmt.Fprintln(stdout, "  可选字段登记（Q15/C5）：不给 ⇒ 与登记前逐字一致、**不得变红**；只放宽 strict 白名单，不进 REQUIRED_FIELDS")
	fmt.Fprintf(stdout, "  非 R 判据（模板 §1 表 / §2 / §3.3 的另一半，**不是新规则**）：%d 条\n", len(pyClauseTable))
	for _, c := range pyClauseTable {
		fmt.Fprintf(stdout, "       %-24s → %s\n", c.Ref, c.Code)
	}
	fmt.Fprintf(stdout, "  明确**不判**登记表（判域之外，逐条写明；不静默）：%d 条\n", len(pyNotChecked))
	for _, nc := range pyNotChecked {
		fmt.Fprintf(stdout, "       %-18s %s\n", nc.A, nc.B)
	}
	fmt.Fprintln(stdout, "════════════════════════════════════════════════════════════════")
}

func fmtNum(f float64) string {
	if f == float64(int(f)) {
		return strconv.Itoa(int(f))
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func joinOr(xs []string, sep string) string {
	if len(xs) == 0 {
		return "(无)"
	}
	return strings.Join(xs, sep)
}

// joinOrNo —— py 里 `'·'.join(xs) or '无'` 的同构（空 ⇒ **「无」**，不是 `(无)`）。
// 用于自证 `[进程内] …缺/多` 与「出仓引用根/缺件」两处（py 该两处空值恰为「无」）。
func joinOrNo(xs []string, sep string) string {
	if len(xs) == 0 {
		return "无"
	}
	return strings.Join(xs, sep)
}

func printListRules(stdout io.Writer, opts *optsT, calib map[string]string) int {
	printRegistration(stdout, opts, calib, false)
	fmt.Fprintln(stdout, "── R0–R13 逐条（§3.4 逐字摘要 · 成本 · 本检查器实施口径）──")
	for _, r := range pyRuleTable {
		fmt.Fprintf(stdout, "[%s] %s ｜ 状态：%s ｜ 成本：%s\n", r.Num, r.Key, r.State, r.Cost)
		fmt.Fprintf(stdout, "    规则：%s\n", r.Text)
		fmt.Fprintf(stdout, "    实施：%s\n", r.Impl)
	}
	fmt.Fprintln(stdout, "── 禁用登记表（§3.5：禁用任何规则必须显式登记并打印）──")
	for _, k := range sortedKeys(pyDisabledRules) {
		d := pyDisabledRules[k]
		fmt.Fprintf(stdout, "[%s] 禁用 —— 理由：%s\n", k, d[0])
		fmt.Fprintf(stdout, "      依据：%s\n", d[1])
		fmt.Fprintf(stdout, "      下沉：%s\n", d[2])
		fmt.Fprintf(stdout, "      影响：%s\n", d[3])
	}
	fmt.Fprintln(stdout, "── 部分实现登记表（整条已登记，未判的那一半写出来）──")
	for _, k := range sortedKeys(pyPartialRules) {
		d := pyPartialRules[k]
		fmt.Fprintf(stdout, "[%s] 已判：%s\n", k, d[0])
		fmt.Fprintf(stdout, "      未判：%s\n", d[1])
		fmt.Fprintf(stdout, "      依据：%s\n", d[2])
	}
	fmt.Fprintln(stdout, "── 路由登记表（超限不打回）──")
	for _, k := range sortedKeys(pyRouteRules) {
		fmt.Fprintf(stdout, "[%s] %s\n", k, pyRouteRules[k])
	}
	fmt.Fprintln(stdout, "── 非 R 判据（模板 §1 字段表 / §2 冲突优先级 / §3.3 继承四条）──")
	for _, c := range pyClauseTable {
		fmt.Fprintf(stdout, "[%-22s] %-20s %s\n", c.Ref, c.Code, c.Text)
	}
	fmt.Fprintln(stdout, "── 片文件已知键（strict：别的键 ⇒ rc=2）──")
	for _, rf := range pyRequiredFields {
		fmt.Fprintf(stdout, "  必填 %-16s ← %s\n", rf[0], rf[1])
	}
	fmt.Fprintf(stdout, "  表外 %-16s ← §1-inv-escalate_if（有 side_effects 即须配）\n", "escalate_if")
	for _, f := range pyOptionalFields {
		fmt.Fprintf(stdout, "  可选 %-16s ← %s\n", f[0], f[1])
		fmt.Fprintf(stdout, "        ↑ **可选**（%s；应到 §9-Q15 / 附录A C5 登记；edit-assert 消费，本检查器不判其内部结构）\n", f[2])
	}
	for _, mk := range pyMetaKeys {
		fmt.Fprintf(stdout, "  元键 %-16s\n", mk)
	}
	fmt.Fprintln(stdout, "── R4 三层词表（每条都带改写建议；无 fix 的命中 ⇒ 拒判 rc=2）──")
	for _, row := range pyRegR4 {
		fmt.Fprintf(stdout, "  [%s] %-14s → %s\n", row.Layer, row.Pat, row.Fix)
	}
	fmt.Fprintln(stdout, "── 命令一览 ──")
	fmt.Fprintln(stdout, "  --check <片.json> [<片.json> …]  判合法性（单片或片集合；退出码 0/1/2）")
	fmt.Fprintln(stdout, "  --probe                          跑探针集并打混淆矩阵（假绿>0 ⇒ rc=1）")
	fmt.Fprintln(stdout, "  --list-rules                     本命令")
	fmt.Fprintln(stdout, "  --selftest                       四条自证（正例·反例·strict·真隐患）")
	fmt.Fprintln(stdout, "  --set-open / --disable-rule <编号> / -q")
	return 0
}

func printResult(stdout io.Writer, res *result, idx, total int, opts *optsT) {
	order := pyRuleKeys
	hitsBy := map[string][]hitT{}
	for _, h := range res.hits {
		hitsBy[h.rule] = append(hitsBy[h.rule], h)
	}
	fmt.Fprintf(stdout, "── 片 %d/%d · %s ──\n", idx, total, res.name)
	line := []string{}
	for _, num := range order {
		if pyRuleState[num] == "禁用" {
			continue
		}
		if opts.disable[num] {
			line = append(line, fmt.Sprintf("%s 已登记禁用", num))
		} else {
			mark := "✓"
			if len(hitsBy[num]) > 0 {
				mark = "✗"
			}
			line = append(line, fmt.Sprintf("%s%s", num, mark))
		}
	}
	fmt.Fprintf(stdout, "  逐规则：%s\n", strings.Join(line, " · "))
	if len(res.hits) > 0 {
		fmt.Fprintf(stdout, "  命中 %d 条（红）：\n", len(res.hits))
		for _, h := range res.hits {
			fmt.Fprintf(stdout, "    ✗ [%s] %s — %s\n", h.rule, h.code, h.msg)
			fmt.Fprintf(stdout, "        改写建议：%s\n", h.fix)
		}
	} else {
		fmt.Fprintln(stdout, "  命中 0 条 ⇒ 本片合法（无红）")
	}
	for _, r := range res.recursion {
		fmt.Fprintf(stdout, "  → 需递归（R7：超限判「需递归」不打回）：[%s] %s\n", r.code, r.msg)
	}
	for _, n := range res.notes {
		fmt.Fprintf(stdout, "  · 注记 [%s] %s\n", n.code, n.msg)
	}
	for _, num := range sortedKeys(pyDisabledRules) {
		fmt.Fprintf(stdout, "  × [%s] 禁用（登记）：%s\n", num, pyDisabledRules[num][0])
	}
	if len(res.hits) > 0 {
		fmt.Fprintf(stdout, "  小计：红 —— 命中 %d 条规则：%s\n", len(res.redRules()), strings.Join(res.redRules(), " · "))
	} else if len(res.recursion) > 0 {
		fmt.Fprintln(stdout, "  小计：需递归（不打回）")
	} else {
		fmt.Fprintln(stdout, "  小计：绿")
	}
}

// ── 探针集与混淆矩阵 ──

func loadProbes() ([]*jv, []string) {
	fi, err := os.Stat(csProbePath())
	if err != nil || fi.IsDir() {
		return nil, []string{fmt.Sprintf("探针集文件不存在：%s（前置缺件 ⇒ 不给结论）", csProbePath())}
	}
	data, err := os.ReadFile(csProbePath())
	if err != nil {
		return nil, []string{fmt.Sprintf("探针集解析失败：%v", err)}
	}
	root, err := jparse(data)
	if err != nil {
		return nil, []string{fmt.Sprintf("探针集解析失败：%v", err)}
	}
	var probes []*jv
	if root != nil && root.kind == 'o' {
		probes = jget(root, "probes").arr
	}
	errs := []string{}
	if root == nil || root.kind != 'o' || jget(root, "probes") == nil || jget(root, "probes").kind != 'a' || len(probes) == 0 {
		return nil, []string{"探针集里没有 `probes` 列表（前置缺件）"}
	}
	for i, p := range probes {
		if p.kind != 'o' {
			errs = append(errs, fmt.Sprintf("probes[%d] 不是对象", i))
			continue
		}
		for _, kv := range p.obj {
			if !csContainsStr(pyProbeKnownKeys, kv.k) {
				errs = append(errs, fmt.Sprintf("[STRICT-UNKNOWN-FIELD] probes[%d] 未知探针键 `%s`（已知：%s）", i, kv.k, strings.Join(pyProbeKnownKeys, " · ")))
			}
		}
		exp := jget(p, "expect")
		if !jIsStr(exp) || !csContainsStr(pyProbeExpects, exp.s) {
			errs = append(errs, fmt.Sprintf("probes[%d]（%s）的 expect 不在 %s 内（分类标签必填）", i, jAsText(jget(p, "name")), strings.Join(pyProbeExpects, " / ")))
		}
		sl := jget(p, "slice")
		sls := jget(p, "slices")
		if !(sl != nil && sl.kind == 'o') && !(sls != nil && sls.kind == 'a') {
			errs = append(errs, fmt.Sprintf("probes[%d]（%s）既没有 `slice` 也没有 `slices`", i, jAsText(jget(p, "name"))))
		}
	}
	return probes, errs
}

func cleanRef(tok string) string {
	tok = strings.TrimRight(csPyStrip(tok), "。，,;；、）)】」》`")
	return regexp.MustCompile(`:\d+(?:-\d+)?$`).ReplaceAllString(tok, "")
}

func probeExternalRefs(p *jv) []string {
	objs := []*jv{}
	if sl := jget(p, "slice"); sl != nil && sl.kind == 'o' {
		objs = append(objs, sl)
	}
	if sls := jget(p, "slices"); sls != nil && sls.kind == 'a' {
		for _, o := range sls.arr {
			if o.kind == 'o' {
				objs = append(objs, o)
			}
		}
	}
	out := []string{}
	for _, o := range objs {
		bag := []*jv{jget(o, "landing")}
		if cn := jget(o, "context_needed"); cn != nil && cn.kind == 'a' {
			for _, it := range cn.arr {
				if it.kind == 'o' {
					bag = append(bag, jget(it, "value"))
				} else {
					bag = append(bag, it)
				}
			}
		}
		for _, key := range []string{"criteria", "gate_cmds"} {
			if v := jget(o, key); v != nil && v.kind == 'a' {
				for _, it := range v.arr {
					if it.kind == 'o' {
						bag = append(bag, jget(it, "cmd"))
					}
				}
			}
		}
		for _, s := range bag {
			if !jIsStr(s) {
				continue
			}
			for _, mm := range pyExternalRefRe.FindAllString(s.s, -1) {
				c := cleanRef(mm)
				if c != "" && !csContainsStr(out, c) {
					out = append(out, c)
				}
			}
		}
	}
	return out
}

func probeExternalRoots(probes []*jv, base string) (string, map[string]map[string]interface{}, []string) {
	info := map[string]map[string]interface{}{}
	dirs := []string{}
	for _, p := range probes {
		nm := jAsText(jget(p, "name"))
		if nm == "" {
			nm = "?"
		}
		for _, ref := range probeExternalRefs(p) {
			d := pyNormpath(pyDirname(ref))
			if d == "" || d == "." {
				continue
			}
			slot, ok := info[d]
			if !ok {
				slot = map[string]interface{}{"probes": []string{}, "refs": []string{}}
				info[d] = slot
				dirs = append(dirs, d)
			}
			ps := slot["probes"].([]string)
			if !csContainsStr(ps, nm) {
				slot["probes"] = append(ps, nm)
			}
			rs := slot["refs"].([]string)
			if !csContainsStr(rs, ref) {
				slot["refs"] = append(rs, ref)
			}
		}
	}
	sort.Strings(dirs)
	common := ""
	if len(dirs) > 0 {
		if c, ok := pyCommonpath(dirs); ok {
			common = c
		}
	}
	missing := []string{}
	for _, r := range dirs {
		if _, err := os.Stat(pyJoin(base, r)); err != nil {
			missing = append(missing, r)
		}
	}
	return common, info, missing
}

func printProbeSourceGate(stdout io.Writer, common string, info map[string]map[string]interface{}, missing []string) {
	fmt.Fprintln(stdout, "✗ 前置缺件：探针集引用的**仓外公共根**在本仓不可达 ⇒ rc=2，不给结论（BLOCKED）")
	cr := "（无法归一）"
	cp := "（无）"
	if common != "" {
		cr = common
		cp = pyJoin(csRepoRoot, common)
	}
	fmt.Fprintf(stdout, "  探针公共根：%s（仓根下解析 = %s）\n", cr, cp)
	for _, r := range missing {
		slot := info[r]
		ps := slot["probes"].([]string)
		rs := slot["refs"].([]string)
		fmt.Fprintf(stdout, "  缺件路径：%s（仓根下解析 = %s）\n", r, pyJoin(csRepoRoot, r))
		ex := "—"
		if len(rs) > 0 {
			ex = rs[0]
		}
		lim := ps
		if len(lim) > 3 {
			lim = lim[:3]
		}
		fmt.Fprintf(stdout, "      引用它的探针 %d 条（例：%s）｜例引用：%s\n", len(ps), strings.Join(lim, " · "), ex)
	}
	fmt.Fprintln(stdout, "  口径：公开树侧（无 `../Zerg-内部文档` 与 `docs/`）本 scope 为 BLOCKED、不适用；私有树把 `Zerg-内部文档/` 放回同级目录再跑。")
	fmt.Fprintln(stdout, "  只闸**公共根目录**：根可达而具体源件真丢 ⇒ 各条探针照旧判红（R2-NOT-FOUND），判据不放宽。")
}

func probeVerdict(p *jv, opts *optsT, calib map[string]string) (string, []string, []*result) {
	objs := []*jv{}
	if sl := jget(p, "slice"); sl != nil && sl.kind == 'o' {
		objs = []*jv{sl}
	} else if sls := jget(p, "slices"); sls != nil && sls.kind == 'a' {
		for _, o := range sls.arr {
			if o.kind == 'o' {
				objs = append(objs, o)
			}
		}
	}
	strict := []string{}
	for i, o := range objs {
		strict = append(strict, strictErrorsForSlice(o, fmt.Sprintf("%s#%d", jAsText(jget(p, "name")), i))...)
	}
	if len(strict) > 0 {
		return "error", strict, nil
	}
	slices := []sliceIn{}
	for i, o := range objs {
		slices = append(slices, sliceIn{path: fmt.Sprintf("%s#%d", jAsText(jget(p, "name")), i), obj: o})
	}
	results := evaluateSet(slices, opts, calib)
	for _, r := range results {
		if r.isRed() {
			return "red", nil, results
		}
	}
	for _, r := range results {
		if len(r.recursion) > 0 {
			return "recursion", nil, results
		}
	}
	return "green", nil, results
}

func runProbe(stdout io.Writer, opts *optsT, calib map[string]string) int {
	probes, errs := loadProbes()
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(stdout, "✗ %s\n", e)
		}
		return 2
	}
	common, info, missing := probeExternalRoots(probes, csRepoRoot)
	if len(missing) > 0 {
		printProbeSourceGate(stdout, common, info, missing)
		return 2
	}
	printRegistration(stdout, opts, calib, opts.quiet)
	vals, _ := probeThresholds(calib)
	nRed, nGreen, nRec, nErr, nBnd := 0, 0, 0, 0, 0
	for _, p := range probes {
		exp := jAsText(jget(p, "expect"))
		switch exp {
		case "red":
			nRed++
		case "green":
			nGreen++
		case "recursion":
			nRec++
		case "error":
			nErr++
		}
		if jAsText(jget(p, "class")) == "边界" {
			nBnd++
		}
	}
	fmt.Fprintf(stdout, "── 探针集 %s · 共 %d 条（期望红 %d · 期望绿 %d · 期望需递归 %d · 期望错误 %d · 其中边界 %d）──\n",
		csProbePath(), len(probes), nRed, nGreen, nRec, nErr, nBnd)
	gateBad := []string{}
	if float64(nRed) < vals["probe_vague_min"] {
		gateBad = append(gateBad, fmt.Sprintf("含糊/违规 %d < 门槛 %s", nRed, fmtNum(vals["probe_vague_min"])))
	}
	if float64(nGreen) < vals["probe_legal_min"] {
		gateBad = append(gateBad, fmt.Sprintf("合法 %d < 门槛 %s", nGreen, fmtNum(vals["probe_legal_min"])))
	}
	if nBnd < 2 {
		gateBad = append(gateBad, fmt.Sprintf("边界 %d < 2", nBnd))
	}
	if nRed < 5 {
		gateBad = append(gateBad, fmt.Sprintf("期望判红 %d < 5（探针集不得写成「永远绿」）", nRed))
	}
	if len(gateBad) > 0 {
		fmt.Fprintf(stdout, "✗ 探针集门槛不过（§3.6）⇒ 前置缺件，不给结论：%s\n", strings.Join(gateBad, " ； "))
		return 2
	}
	type cntKey struct{ e, g string }
	cnt := map[cntKey]int{}
	type mismT struct {
		p       *jv
		exp     string
		got     string
		strict  []string
		results []*result
	}
	mism := []mismT{}
	type ruleBadT struct {
		nm         string
		miss, want []string
	}
	ruleBad := []ruleBadT{}
	for _, p := range probes {
		got, strict, results := probeVerdict(p, opts, calib)
		exp := jAsText(jget(p, "expect"))
		cnt[cntKey{exp, got}]++
		ok := exp == got
		labels := map[string]bool{}
		for _, r := range results {
			for _, rl := range r.redRules() {
				labels[rl] = true
			}
			for _, rec := range r.recursion {
				labels[strings.SplitN(rec.code, "-", 2)[0]] = true
			}
		}
		codes := []string{}
		for _, r := range results {
			for _, h := range r.hits {
				codes = append(codes, h.code)
			}
		}
		if len(strict) > 0 {
			labels["STRICT"] = true
			codes = []string{"STRICT"}
			for _, s := range strict {
				part := strings.SplitN(s, "]", 2)[0]
				ix := strings.LastIndex(part, "[")
				if ix >= 0 {
					codes = append(codes, part[ix+1:])
				}
			}
		}
		want := []string{}
		if wr := jget(p, "expect_rules"); wr != nil && wr.kind == 'a' {
			for _, w := range wr.arr {
				want = append(want, jAsText(w))
			}
		}
		missingRule := []string{}
		for _, w := range want {
			if !labels[w] {
				missingRule = append(missingRule, w)
			}
		}
		if len(missingRule) > 0 {
			ok = false
			ruleBad = append(ruleBad, ruleBadT{jAsText(jget(p, "name")), missingRule, want})
		}
		mark := "✗"
		if ok {
			mark = "✓"
		}
		cls := jAsText(jget(p, "class"))
		if cls == "" {
			cls = "-"
		}
		suffix := ""
		if len(codes) > 0 {
			lim := codes
			if len(lim) > 6 {
				lim = lim[:6]
			}
			suffix += fmt.Sprintf("  命中：%s", strings.Join(lim, " · "))
		}
		if len(missingRule) > 0 {
			suffix += fmt.Sprintf("  缺标注规则：%s", strings.Join(missingRule, "·"))
		}
		fmt.Fprintf(stdout, "  %s [%s] %-34s 期望=%s 实判=%s%s\n",
			mark, cls, jAsText(jget(p, "name")), pyProbeExpectCN[exp], pyProbeGotCN[got], suffix)
		if !ok {
			mism = append(mism, mismT{p, exp, got, strict, results})
		}
	}
	if len(ruleBad) > 0 {
		fmt.Fprintln(stdout, "  ── 标注规则未命中（探针标签与实判不一致 ⇒ 假绿的口子）──")
		for _, rb := range ruleBad {
			fmt.Fprintf(stdout, "  ✗ %s：标注 `expect_rules`=%s，实判缺 %s\n", rb.nm, strings.Join(rb.want, "·"), strings.Join(rb.miss, "·"))
		}
	}
	if len(mism) > 0 {
		fmt.Fprintln(stdout, "  ── 不符探针明细 ──")
		for _, mm := range mism {
			fmt.Fprintf(stdout, "  ✗ %s：期望 %s，实判 %s —— %s\n", jAsText(jget(mm.p, "name")), pyProbeExpectCN[mm.exp], pyProbeGotCN[mm.got], jAsText(jget(mm.p, "why")))
			for _, s := range mm.strict {
				fmt.Fprintf(stdout, "      | %s\n", s)
			}
			for _, r := range mm.results {
				for _, h := range r.hits {
					fmt.Fprintf(stdout, "      | [%s] %s — %s\n", h.rule, h.code, runeSliceStr(h.msg, 0, 90))
				}
			}
		}
	}
	tp := cnt[cntKey{"red", "red"}]
	fp := cnt[cntKey{"green", "red"}]
	tn := cnt[cntKey{"green", "green"}]
	fn := cnt[cntKey{"red", "green"}] + cnt[cntKey{"red", "recursion"}] + cnt[cntKey{"red", "error"}]
	recOK := cnt[cntKey{"recursion", "recursion"}]
	errOK := cnt[cntKey{"error", "error"}]
	recBad, errBad, greenMis := 0, 0, 0
	for k, v := range cnt {
		if k.e == "recursion" && k.g != "recursion" {
			recBad += v
		}
		if k.e == "error" && k.g != "error" {
			errBad += v
		}
		if k.e == "green" && k.g != "green" {
			greenMis += v
		}
	}
	fmt.Fprintln(stdout, "════════════════════════════════════════════════════════════════")
	fmt.Fprintln(stdout, "── 混淆矩阵（标注集 = 探针文件里的 `expect` 标签）──")
	cols := []string{"red", "green", "recursion", "error"}
	head := "  期望↓ / 实判→        "
	for i, c := range cols {
		if i > 0 {
			head += "  "
		}
		head += fmt.Sprintf("%-12s", pyProbeExpectCN[c])
	}
	fmt.Fprintln(stdout, head)
	for _, e := range cols {
		row := fmt.Sprintf("  %-18s", pyProbeExpectCN[e])
		for _, g := range cols {
			row += fmt.Sprintf("  %-12s", strconv.Itoa(cnt[cntKey{e, g}]))
		}
		fmt.Fprintln(stdout, row)
	}
	fmt.Fprintf(stdout, "  真阳 TP（期望红·判红）= %d ｜ 假阴 FN/假绿（期望红·未判红）= %d\n", tp, fn)
	fmt.Fprintf(stdout, "  真阴 TN（期望绿·判绿）= %d ｜ 假阳 FP/假红（期望绿·判红）= %d\n", tn, fp)
	fmt.Fprintf(stdout, "  需递归类命中 = %d/%d（未命中 %d）｜ 错误类命中 = %d/%d（未命中 %d）\n",
		recOK, recOK+recBad, recBad, errOK, errOK+errBad, errBad)
	fmt.Fprintf(stdout, "  合计：探针 %d 条（期望红 %d · 期望绿 %d · 期望需递归 %d · 期望错误 %d）\n", len(probes), nRed, nGreen, nRec, nErr)
	fpRate := 0.0
	if nGreen > 0 {
		fpRate = float64(fp) / float64(nGreen)
	}
	fmt.Fprintln(stdout, "── 门槛判定（§3.5：假绿率必须 0、假红率 ≤ 10%）──")
	rc := 0
	if fn == 0 {
		fmt.Fprintf(stdout, "  假绿率 = 0 ✓（%d 条期望红全部被拦住）\n", nRed)
	} else {
		fmt.Fprintf(stdout, "  ✗ 假绿率 > 0（%d 条真隐患被放过）⇒ rc=1\n", fn)
		rc = 1
	}
	if fpRate <= vals["probe_fp_rate_max"] {
		fmt.Fprintf(stdout, "  假红率 = %.1f%% ✓（≤ %.1f%%，%d/%d）\n", fpRate*100, vals["probe_fp_rate_max"]*100, fp, nGreen)
	} else {
		fmt.Fprintf(stdout, "  ✗ 假红率 = %.1f%% > %.1f%% ⇒ rc=1\n", fpRate*100, vals["probe_fp_rate_max"]*100)
		rc = 1
	}
	if greenMis > 0 {
		fmt.Fprintf(stdout, "  ✗ 期望绿但未判绿（需递归/错误）= %d ⇒ rc=1\n", greenMis)
		rc = 1
	}
	if recBad > 0 || errBad > 0 {
		fmt.Fprintf(stdout, "  ✗ 需递归类未命中 %d · 错误类未命中 %d ⇒ rc=1\n", recBad, errBad)
		rc = 1
	}
	if len(ruleBad) > 0 {
		fmt.Fprintf(stdout, "  ✗ 探针标注规则（expect_rules）未命中 %d 条 ⇒ rc=1（标签与实判不一致 = 假绿的口子）\n", len(ruleBad))
		rc = 1
	}
	fmt.Fprintf(stdout, "  探针条数门槛：含糊/违规 %d ≥ %s ✓ · 合法 %d ≥ %s ✓ · 边界 %d ≥ 2 ✓\n",
		nRed, fmtNum(vals["probe_vague_min"]), nGreen, fmtNum(vals["probe_legal_min"]), nBnd)
	concl := "不过门槛"
	if rc == 0 {
		concl = "过门槛（假绿 0 / 假红在限内）"
	}
	fmt.Fprintf(stdout, "结论：探针集 %s；rc = %d\n", concl, rc)
	return rc
}

func loadSliceFile(path string) (*jv, string) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Sprintf("片文件不存在：%s", path)
	}
	if fi.IsDir() {
		return nil, fmt.Sprintf("不是普通文件：%s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Sprintf("片文件读不出：%s（%v）", path, err)
	}
	obj, err := jparse(data)
	if err != nil {
		return nil, fmt.Sprintf("片文件不是合法 JSON：%s（%v）", path, err)
	}
	return obj, ""
}

// ── 命令行 ──

const csUsage = `用法:
  python3 scripts/gates/check-slice.py --check <片.json> [<片.json> ...]   判合法性（单片或片集合）
  python3 scripts/gates/check-slice.py --probe                            跑探针集 + 混淆矩阵
  python3 scripts/gates/check-slice.py --list-rules                       列规则 / 启用状态 / 登记表
  python3 scripts/gates/check-slice.py --selftest                         四条自证（正例·反例·strict·真隐患）
选项: --set-open（该批不是完整集合）· --disable-rule <编号> · -q/--quiet · -h/--help
退出码: 0 全部合法 / 1 有非法（打印原因码 + 命中哪条 R）/ 2 用法错·前置缺件·strict 违约·自检不过（不给结论）`

func csRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	hit := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	quoted := []string{}
	for _, h := range hit {
		quoted = append(quoted, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	ft := "无（本门只出人读面）"
	if len(pyJsonFields) > 0 {
		qs := []string{}
		for _, f := range pyJsonFields {
			qs = append(qs, "`"+f+"`")
		}
		ft = strings.Join(qs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", ft)
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	return 2, true
}

func csParseArgs(stdout, stderr io.Writer, argv []string) (*optsT, string, bool) {
	opts := &optsT{disable: map[string]bool{}}
	repoRoot := ""
	i := 0
	for i < len(argv) {
		v := argv[i]
		switch {
		case v == "--check":
			opts.mode = "check"
			i++
			for i < len(argv) && !strings.HasPrefix(argv[i], "-") {
				opts.paths = append(opts.paths, argv[i])
				i++
			}
		case v == "--probe":
			opts.mode = "probe"
			i++
		case v == "--list-rules":
			opts.mode = "list"
			i++
		case v == "--self-test" || v == "--selftest":
			opts.mode = "selftest"
			i++
		case v == "--set-open":
			opts.setOpen = true
			i++
		case v == "--disable-rule" && i+1 < len(argv):
			opts.disableList = append(opts.disableList, argv[i+1])
			i += 2
		case v == "-q" || v == "--quiet":
			opts.quiet = true
			i++
		case v == "-h" || v == "--help":
			fmt.Fprintln(stdout, csUsage)
			return nil, "", false
		case v == "--repo-root" && i+1 < len(argv):
			repoRoot = argv[i+1]
			i += 2
		case strings.HasPrefix(v, "--repo-root="):
			repoRoot = v[len("--repo-root="):]
			i++
		default:
			fmt.Fprintf(stderr, "✗ 未知参数: %s\n", v)
			fmt.Fprintln(stdout, csUsage)
			return nil, "", false
		}
	}
	return opts, repoRoot, true
}

func checkSliceGateMain(stdout, stderr io.Writer, argv []string) (rc int) {
	defer func() {
		if r := recover(); r != nil {
			if ie, ok := r.(internalErr); ok {
				fmt.Fprintf(stderr, "✗ 内部错误（不给结论）：%s\n", ie.msg)
				rc = 2
				return
			}
			panic(r)
		}
	}()
	if rcq, hit := csRefuseJSON(stderr, argv); hit {
		return rcq
	}
	csRepoRoot = csDefaultRepoRoot()
	if len(argv) == 0 {
		fmt.Fprintln(stdout, csUsage)
		return 2
	}
	opts, repoOverride, ok := csParseArgs(stdout, stderr, argv)
	if !ok {
		return 2
	}
	if repoOverride != "" {
		csRepoRoot = pyAbspath(repoOverride)
	}
	csCalibPath = csCalibPathFromEnv()
	derr := validateDisables(opts.disableList)
	if len(derr) > 0 {
		for _, e := range derr {
			fmt.Fprintf(stdout, "✗ %s\n", e)
		}
		fmt.Fprintln(stdout, "✗ strict 违约 ⇒ rc=2，不给结论（§3.5：未知规则键＝错误；禁用必须显式登记并打印）")
		return 2
	}
	for _, k := range opts.disableList {
		opts.disable[k] = true
	}
	calib, calibErr := loadCalib()
	if opts.mode == "" {
		fmt.Fprintln(stdout, csUsage)
		return 2
	}
	if opts.mode == "list" {
		return printListRules(stdout, opts, calib)
	}
	if opts.mode == "probe" {
		return runProbe(stdout, opts, calib)
	}
	if opts.mode == "selftest" {
		return runSelftest(stdout, stderr, opts, calib)
	}
	// ── check ──
	if len(opts.paths) == 0 {
		fmt.Fprintln(stderr, "✗ --check 后面没有给片文件")
		fmt.Fprintln(stdout, csUsage)
		return 2
	}
	slices := []sliceIn{}
	strict := []string{}
	for _, p := range opts.paths {
		obj, errStr := loadSliceFile(p)
		if errStr != "" {
			strict = append(strict, errStr)
			continue
		}
		strict = append(strict, strictErrorsForSlice(obj, p)...)
		slices = append(slices, sliceIn{path: p, obj: obj})
	}
	if len(strict) > 0 {
		printRegistration(stdout, opts, calib, opts.quiet)
		for _, e := range strict {
			fmt.Fprintf(stdout, "✗ %s\n", e)
		}
		fmt.Fprintln(stdout, "✗ 前置缺件 / strict 违约 ⇒ rc=2，**不给结论**（strict 等价：未知规则键/字段键＝错误）")
		return 2
	}
	printRegistration(stdout, opts, calib, opts.quiet)
	if opts.setOpen {
		fmt.Fprintln(stdout, "注：--set-open 已声明 ⇒ 依赖闭包记「不可判」（该批不是完整集合）")
	}
	if calibErr != "" {
		fmt.Fprintf(stdout, "注：标定档案读不到（%s）⇒ 与档案绑定的上界记「未标定」，不猜\n", calibErr)
	}
	results := evaluateSet(slices, opts, calib)
	nred, nrec := 0, 0
	for i, r := range results {
		printResult(stdout, r, i+1, len(results), opts)
		if r.isRed() {
			nred++
		}
		if len(r.recursion) > 0 {
			nrec++
		}
	}
	fmt.Fprintln(stdout, "════════════════════════════════════════════════════════════════")
	fmt.Fprintf(stdout, "结论：共 %d 片 · 合法 %d · 非法 %d · 需递归标记 %d（R7 超限不打回 ⇒ 不计入非法）\n",
		len(results), len(results)-nred, nred, nrec)
	if nred > 0 {
		codes := []string{}
		for _, r := range results {
			for _, h := range r.hits {
				codes = append(codes, h.code)
			}
		}
		fmt.Fprintf(stdout, "原因码汇总：%s\n", strings.Join(codes, " · "))
		fmt.Fprintln(stdout, "rc = 1（有非法片）")
		return 1
	}
	fmt.Fprintln(stdout, "rc = 0（全部合法）")
	return 0
}

// ── 自检（★ 有意未覆盖面：py 走 subprocess + tempfile + shutil 自定位自拷；Go 入口无 `__file__`
//
//	语境 ⇒ 本档以**进程内**等价调用复刻 (a)–(e) 与 R4 守卫，退码对齐 py 的实得值，(f) 仿真树那一步
//	在件内以进程内 --probe 复刻（因探针外引用闸现读为「死面」，实得 rc 与 py 同）。stdout 细节面
//	声明为**部分未覆盖**。）──
func runSelftest(stdout, stderr io.Writer, opts *optsT, calib map[string]string) int {
	printRegistration(stdout, opts, calib, opts.quiet)
	probes, errs := loadProbes()
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(stdout, "✗ %s\n", e)
		}
		return 2
	}
	byname := map[string]*jv{}
	for _, p := range probes {
		byname[jAsText(jget(p, "name"))] = p
	}
	for _, nm := range []string{"template-正例", "template-反例", "真隐患-缺期望输出"} {
		if byname[nm] == nil {
			fmt.Fprintf(stdout, "✗ 自证夹具缺件：探针集里没有 `%s`（前置缺件 ⇒ 不给结论）\n", nm)
			return 2
		}
	}
	tmp, err := os.MkdirTemp("", "check-slice-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "✗ 自证临时目录起不来：%v\n", err)
		return 2
	}
	defer os.RemoveAll(tmp)
	type chkT struct {
		desc string
		ok   bool
		rc   int
		out  string
	}
	bad := 0
	checks := []chkT{}
	runIn := func(w io.Writer, argv []string) (int, string) {
		var o, e bytes.Buffer
		rc := checkSliceGateMain(&o, &e, argv)
		s := o.String() + e.String()
		if w != nil {
			_, _ = w.Write([]byte(s))
		}
		return rc, s
	}
	writeSlice := func(name string, obj *jv) string {
		fp := filepath.Join(tmp, name)
		b, _ := jsonMarshalPy(obj)
		_ = os.WriteFile(fp, b, 0o644)
		return fp
	}
	// (a)
	pa := byname["template-正例"]
	fa := writeSlice("a-正例.json", jget(pa, "slice"))
	rcA, outA := runIn(nil, []string{"--check", fa, "-q"})
	checks = append(checks, chkT{"(a) 模板正例 ⇒ 期望 rc=0", rcA == 0, rcA, outA})
	// (b)
	pb := byname["template-反例"]
	fb := writeSlice("b-反例.json", jget(pb, "slice"))
	rcB, outB := runIn(nil, []string{"--check", fb, "-q"})
	inproc := evaluateSet([]sliceIn{{path: "b-反例.json", obj: jget(pb, "slice")}}, opts, calib)
	gotRules := []string{}
	for _, r := range inproc {
		gotRules = append(gotRules, r.redRules()...)
	}
	want := []string{}
	if wr := jget(pb, "expect_rules"); wr != nil && wr.kind == 'a' {
		for _, w := range wr.arr {
			want = append(want, jAsText(w))
		}
	}
	missingR := []string{}
	for _, x := range want {
		if !csContainsStr(gotRules, x) {
			missingR = append(missingR, x)
		}
	}
	extraR := []string{}
	for _, x := range gotRules {
		if !csContainsStr(want, x) && strings.HasPrefix(x, "R") {
			extraR = append(extraR, x)
		}
	}
	okB := rcB == 1 && len(missingR) == 0
	checks = append(checks, chkT{fmt.Sprintf("(b) 模板反例 ⇒ 期望 rc=1 且命中模板标注的违规 R（%s）", strings.Join(want, "·")), okB, rcB,
		outB + fmt.Sprintf("\n[进程内] 实命中 R：%s；缺：%s；多（未标注）：%s", strings.Join(gotRules, "·"), joinOrNo(missingR, "·"), joinOrNo(extraR, "·"))})
	// (c)
	badSliceObj := &jv{kind: 'o', obj: append([]jpair{}, jget(pb, "slice").obj...)}
	badSliceObj.obj = append(badSliceObj.obj, jpair{"r14_new_rule", &jv{kind: 's', s: "自造的规则键（不存在于 R0–R13）"}})
	fc := writeSlice("c-未知规则键.json", badSliceObj)
	rcC1, outC1 := runIn(nil, []string{"--check", fc, "-q"})
	checks = append(checks, chkT{"(c1) 片里出现未知字段/规则键 ⇒ 期望 rc=2（strict：未知键＝错误）", rcC1 == 2, rcC1, outC1})
	rcC2, outC2 := runIn(nil, []string{"--check", fa, "--disable-rule", "R99"})
	checks = append(checks, chkT{"(c2) --disable-rule R99（未知规则键）⇒ 期望 rc=2", rcC2 == 2, rcC2, outC2})
	rcC3, outC3 := runIn(nil, []string{"--check", fa, "--disable-rule", "R7"})
	checks = append(checks, chkT{"(c3) --disable-rule R7（已知规则但**未登记**禁用）⇒ 期望 rc=2", rcC3 == 2, rcC3, outC3})
	// (d)
	pd := byname["真隐患-缺期望输出"]
	fd := writeSlice("d-真隐患.json", jget(pd, "slice"))
	rcD, outD := runIn(nil, []string{"--check", fd, "-q"})
	checks = append(checks, chkT{"(d) 真隐患（criteria 缺期望输出）⇒ 期望 rc=1", rcD == 1, rcD, outD})
	// (e)
	common, info, missing := probeExternalRoots(probes, csRepoRoot)
	rcE, outE := runIn(nil, []string{"--probe"})
	okE := rcE == 0 && len(missing) == 0
	checks = append(checks, chkT{fmt.Sprintf("(e) 前置缺件闸·正：探针公共根可达（%s）⇒ 闸放行、--probe 照判 rc=0", csOrDash(common)), okE, rcE,
		outE + fmt.Sprintf("\n[进程内] 出仓引用根：%s；缺件：%s", joinOrNo(sortedKeys(info), " · "), joinOrNo(missing, " · "))})
	// (f) 仿真公开树：进程内以 --repo-root 指向无同级 Zerg-内部文档 的夹具根
	simRepo := filepath.Join(tmp, "sim-public", "repo")
	_ = os.MkdirAll(filepath.Join(simRepo, "scripts", "gates"), 0o755)
	saved := csRepoRoot
	// py (f) 把 PROBE_PATH 拷进仿真仓 `scripts/gates/`（py 由 `__file__` 推探针路径）；本件探针路径
	// 按 `csRepoRoot` 解 ⇒ 同样把探针件拷进仿真仓，探针集才在（否则先撞「探针集文件不存在」而到不了缺件闸）。
	if b, err := os.ReadFile(csProbePath()); err == nil {
		_ = os.WriteFile(filepath.Join(simRepo, "scripts", "gates", "slice-probes.json"), b, 0o644)
	}
	rcF, outF := runIn(nil, []string{"--probe", "--repo-root", simRepo})
	csRepoRoot = saved
	okF := rcF == 2 && strings.Contains(outF, "缺件路径") && strings.Contains(outF, "../Zerg-内部文档")
	checks = append(checks, chkT{"(f) 前置缺件闸·反：仿真公开树（无 ../Zerg-内部文档）⇒ 期望 rc=2 且打印缺件路径", okF, rcF, outF})
	fmt.Fprintln(stdout, "── 自证 ──")
	for _, c := range checks {
		if !c.ok {
			bad++
		}
		mark := "✓"
		if !c.ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "%s %s（实得 rc=%d）\n", mark, c.desc, c.rc)
		for _, ln := range strings.Split(c.out, "\n") {
			s := csPyStrip(ln)
			if s == "" {
				continue
			}
			if strings.Contains(s, "✗ [R") || strings.Contains(s, "STRICT") || strings.HasPrefix(s, "结论") ||
				strings.HasPrefix(s, "[进程内]") || strings.HasPrefix(s, "✗ 前置缺件") ||
				strings.HasPrefix(s, "缺件路径") || strings.HasPrefix(s, "探针公共根") {
				fmt.Fprintf(stdout, "      | %s\n", s)
			}
		}
	}
	guardOK := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(internalErr); ok {
					guardOK = true
				}
			}
		}()
		(&result{name: "guard"}).hit("R4", "R4-VAGUE", "自造的无 fix 命中", "")
	}()
	gm := "✗"
	if guardOK {
		gm = "✓"
	} else {
		bad++
	}
	fmt.Fprintf(stdout, "%s R4 约束守卫：无改写建议的命中必须拒判（InternalError）\n", gm)
	concl := "不过"
	if bad == 0 {
		concl = "全过"
	}
	fmt.Fprintf(stdout, "自证结论：%s（用例 %d 条，失败 %d 条）\n", concl, len(checks)+1, bad)
	if bad == 0 {
		return 0
	}
	return 2
}

func csOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// jsonMarshalPy —— 把有序 jv 以 indent=2 形态写盘（仅自检夹具用）。
func jsonMarshalPy(v *jv) ([]byte, error) {
	return []byte(pyJSONIndent2(v, 0)), nil
}

func pyJSONIndent2(v *jv, depth int) string {
	pad := strings.Repeat("  ", depth)
	pad2 := strings.Repeat("  ", depth+1)
	switch v.kind {
	case 'n':
		return "null"
	case 'b':
		if v.b {
			return "true"
		}
		return "false"
	case 'i':
		return v.num
	case 's':
		return jJSONStr(v.s)
	case 'a':
		if len(v.arr) == 0 {
			return "[]"
		}
		parts := []string{}
		for _, el := range v.arr {
			parts = append(parts, pad2+pyJSONIndent2(el, depth+1))
		}
		return "[\n" + strings.Join(parts, ",\n") + "\n" + pad + "]"
	case 'o':
		if len(v.obj) == 0 {
			return "{}"
		}
		parts := []string{}
		for _, kv := range v.obj {
			parts = append(parts, pad2+jJSONStr(kv.k)+": "+pyJSONIndent2(kv.v, depth+1))
		}
		return "{\n" + strings.Join(parts, ",\n") + "\n" + pad + "}"
	}
	return "null"
}
