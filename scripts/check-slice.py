#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-slice.py — 切片合同（片）机器判据检查器

一句话：照 `docs/01-设计/切片合同-模板.md` 的**共同规则表**（§1 字段表 + §2 冲突优先级 +
§3.3 子合同继承四条 + §3.4 判据 R0–R13）判一片 / 一批片：**合法 / 非法 / 需递归**，并自带
探针集跑混淆矩阵（§3.5 质量门槛：假绿 = 0 · 假红 ≤ 10%）。

规则侧与检查器侧的关系（模板 §7 逐字）：模板是「写的侧」，本脚本是「判的侧」，两件共用同一份
规则表 —— 任一侧改了字段或判据，另一侧必须同步（否则「规则写了但被静默跳过」）。

用法
----
    python3 scripts/check-slice.py --check <片.json> [<片.json> ...]   # 判合法性
    python3 scripts/check-slice.py --probe                            # 跑探针集 + 混淆矩阵
    python3 scripts/check-slice.py --list-rules                       # 列规则 / 启用状态 / 全部登记表
    python3 scripts/check-slice.py --selftest                         # 成对自证（正例·反例·strict·真隐患
                                                                      #   + 前置缺件闸正反两半）

选项
----
    --set-open            该批片**不是**完整集合 ⇒ 只判片内规则，依赖闭包记「不可判」
    --disable-rule <编号>  禁用一条**已登记**规则（登记表见 --list-rules）；未登记 ⇒ rc=2
    -q / --quiet          少打登记块（结论行与命中行照打）

退出码（借仓内 `scripts/check-build-tags.py` / `scripts/accept-no-probe.sh` 的 0/1/2 惯例）
----
    0  全部合法（可有「需递归」标记 —— R7 超限**判需递归、不打回**，逐字 §3.4-R7）
    1  有非法片（逐条打印原因码 + 命中哪条 R）；或探针假绿 > 0；或假红率 > 10%；
       或「错误类 / 需递归类」探针未命中
    2  用法错 · 前置缺件（片/探针/标定档案相关问题 · **探针公共根不可达**）· **strict 违约**
       （未知字段键 / 未知规则键 / 未登记的禁用请求）· 自检不过 · 命中缺「改写建议」
       （R4：无改写建议不算有效报红）—— **不给结论**

公开树侧（无 `../Zerg-内部文档` 与 `docs/`）的行为（2026-09-19 开发文档分家后新增）
----
    ★ 探针集引用的源件（切片合同模板等）随开发文档分家搬到**工作树之外**的同级目录
      `../Zerg-内部文档/`。公开树（`publish/whitelist.txt` 只收 `scripts/`）里没有它 ⇒ 每条探针的
      `landing` / `context_needed` 都落 R2-NOT-FOUND ⇒ 全被算成「假红」（实测假红率 63.6%，
      把「本仓缺件」错报成「探针写错了」）。
    ⇒ **前置缺件闸**（`probe_external_roots` / `precheck_probe_sources`）：探针引用的**仓外公共根**
      （现读 = `../Zerg-内部文档/01-设计`）在本仓不可达 ⇒ **rc=2 BLOCKED、不给结论**，并逐条打印缺件
      路径与引用它的探针名（**不**把每条探针判成假红）。
    ⇒ 本 scope 在公开树侧 = **BLOCKED、不适用**（既不是红也不是绿）；私有树按原口径正常判。
    ★ 只闸**公共根目录**：根可达而具体源件真丢 ⇒ 各条探针照旧判红（R2-NOT-FOUND）—— 判据一字不放宽。

strict 等价（§3.5 逐字：未知规则键＝错误）
----
    ① 片文件里出现**未知字段键** ⇒ 错误（rc=2，不给结论）；
    ② 片文件里的 `rules` 段出现**未知规则键** ⇒ 错误（rc=2）；
    ③ 片文件里的 `rules` 段试图关闭一条**未登记**的规则 ⇒ 错误（rc=2）；
    ④ 命令行 `--disable-rule` 给**未知规则键**或**未登记禁用**的规则 ⇒ 错误（rc=2）。
    ⇒ 任何一次「规则被跳过」都必须先过登记表，否则不给结论。

零依赖 · python3 ≥ 3.6（本机 3.9.6 实跑）· 不联网 · 不改任何文件 · 不碰 git。
"""

import json
import os
import re
import sys

# ══════════════════════════════════════════════════════════════════
# 0. 位置与外部输入（标定档案 / 探针集）
# ══════════════════════════════════════════════════════════════════

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.abspath(os.path.join(HERE, os.pardir))
PROBE_PATH = os.path.join(HERE, 'slice-probes.json')

_EGG_DIR = os.environ.get('ZERG_EGG_PROFILE_DIR') or os.path.expanduser('~/.zerg/egg-profiles')
CALIB_PATH = os.path.join(_EGG_DIR, 'task-budget-calib.yaml')
CALIB_REF = 'task-budget-calib'          # 出处判据：source 必须指到这个档案（登记口径，见 §8-4）


def load_calib():
    """只读标定档案（只认 `key: value` 标量行）。返回 (dict, 出错原因|None)。

    铁律（照 `scripts/calib-task-budget.py`）：闸门**只读档案**；读不到 ⇒ 记「未标定」，不猜。
    """
    if not os.path.isfile(CALIB_PATH):
        return {}, '档案不存在: %s' % CALIB_PATH
    out = {}
    try:
        with open(CALIB_PATH, 'r', encoding='utf-8', errors='replace') as fh:
            for ln in fh:
                ln = ln.strip()
                if not ln or ln.startswith('#') or ':' not in ln:
                    continue
                k, v = ln.split(':', 1)
                out[k.strip()] = v.strip()
    except (IOError, OSError) as exc:
        return {}, '档案读不出: %s' % exc
    return out, None


# ══════════════════════════════════════════════════════════════════
# 1. 规则登记表（改这里 = 改契约）
# ══════════════════════════════════════════════════════════════════

# (编号, 规则键, 状态, 规则原文〔§3.4 逐字摘要〕, 成本, 本检查器实施口径)
RULE_TABLE = [
    ('R0', 'one_thing_shape', '启用',
     '正面句式：`the_one_thing` 必须能套进「When/While ⟨条件⟩，⟨落点⟩ 必须 ⟨单一动作⟩」且只含一个动词',
     '低', '句式正则 + 「必须」之后动作段的动词计数（词表登记见 REG_VERBS）'),
    ('R1', 'single_defect', '启用', '一片只绑**一个** `defect_id`',
     '低', '字符串含分隔符后切成 >1 个 id ⇒ 红；列表长度 >1 ⇒ 红'),
    ('R2', 'landing_path_exists', '启用', '`landing` 含路径样式且**当前存在**（前状态检查）',
     '低', '路径形态：含 `/` 或命中登记扩展名；存在性按**仓根**解析（绝对路径按原样）'),
    ('R3', 'criteria_triplet', '启用', '`criteria` 三件套齐全（命令+期望输出+当前红/绿）',
     '低', '每条须有非空 `cmd` / `expect` / `state`，且 `state` 命中红或绿 ⇒ 缺一判红'),
    ('R4', 'unbounded_words', '启用',
     '无界词三层：含糊词 / 可选词（can·may·可选）/ 主观词；**命中必须附"改写成什么"** ✗ 否则不算有效报红',
     '低', '三层词表（REG_R4）+ 强制改写建议：命中字典给出 fix；给不出 fix 的命中 ⇒ 拒判 rc=2'),
    ('R5', 'no_empty_shell', '启用',
     '`hard_limits` / `side_effects` / `output_format` / `exit_cond` 非空且非占位'
     '（空壳检测：不得等于模板占位文本、不得含未替换的 `[]`/`<>`；未决必须写成 `NEEDS-CLARIFICATION`）',
     '低', '四字段逐个查：空壳 / 占位词（REG_PLACEHOLDER）/ 未替换括号；`NEEDS-CLARIFICATION` 合法'),
    ('R6', 'gate_cmds_expect', '启用', '`gate_cmds` 可解析且**每条带 expect**',
     '低', '每条须为 {cmd, expect}；expect 取值域登记（REG_EXPECT_KEYS）；'
           'exit-only 的假绿命令形态（REG_EXIT_TRAP）判红'),
    ('R7', 'granularity_line_bound', '路由',
     '粒度：默认**单文件** + 行数上界（上界数值 = 本仓标定取值（待实测复核））；'
     '**超限判"需递归"，而不是打回** ✗',
     '低', '落点文件数 >1 ⇒ 需递归；行数上界取标定档案（缺 ⇒ 记「未标定」，不猜、不判红）'),
    ('R8', 'slice_set_dag', '启用',
     '片集合：`depends_on` 可**拓扑排序且无环**；同一 `landing`+`defect_id` 不重复',
     '低', '自依赖直接判红；≥2 片时 Kahn 拓扑排序查环；同 (landing, defect_id) 重复判红'),
    ('R9', 'set_coverage_and_closure', '部分',
     '片集合：该缺陷的**每条验收判据至少被一片承接**（未覆盖清单随打回附上）+ '
     '**集合停机判据**（全部 open defect 被覆盖、依赖闭包闭合）',
     '中', '已实现：依赖闭包闭合（集合级）。未实现：验收判据覆盖清单 + 停机判据（缺输入 schema，登记见 --list-rules）'),
    ('R10', 'self_contained', '部分',
     '**自包含性**：片带落点原文片段 + 判据命令 + 执行者可见的最小上下文；**无设计稿也能复现**',
     '中', '已实现（句法侧）：context_needed 非空 + 含落点原文片段 + 不得把设计稿当上下文。'
           '未实现：只给片实跑一次（非纯句法）'),
    ('R11', 'context_budget_and_order', '启用',
     '`context_budget`（执行者所需文件/token 上界）；landing 片段与 criteria **固定放最前**'
     '（长上下文 U 型曲线：arXiv 2307.03172《Lost in the Middle》〔会议版：TACL 2024〕）',
     '低', '三断言：context_budget 在且有数值上界与出处；context_needed 首条 = 落点原文片段；'
           'criteria 键序在其他 post/invariant 字段之前'),
    ('R12', 'exit_cond_structured', '启用',
     '`exit_cond` 结构化三项：成功（引用 criteria id）/ 失败（可观察条件）/ 升级（对象+触发次数+记录字段）',
     '低', 'success 须引用现存 criteria id；failure 非空；escalate 须有 to/times/fields'),
    ('R13', 'static_layer_decomposition_abstain', '禁用',
     '静态层不做"分解是否成立"的判定 ✗ —— **本仓设计决定**（无需外部出处）；下沉到**执行后验收**'
     '（依据：**HTN 计划验证为 NP-complete**，Behnke et al., ICAPS 2015〔会议版〕）',
     '—', '规则内容就是「不做」⇒ 本检查器对它不产出任何红/绿（登记并打印，见 DISABLED_RULES）'),
]

RULE_KEYS = [r[0] for r in RULE_TABLE]
RULE_NAMES = dict((r[0], r[1]) for r in RULE_TABLE)
RULE_STATE = dict((r[0], r[2]) for r in RULE_TABLE)

# 禁用登记表：**禁用任何规则必须显式登记并打印**（§3.5 本仓要求）——防「规则写了但被静默跳过」
DISABLED_RULES = {
    'R13': {
        'why': '规则内容就是「静态层不做"分解是否成立"的判定」⇒ 静态层无对象可判',
        'evidence': '§3.4-R13（逐字；本仓设计决定）',
        'downstream': '该判定下沉到**执行后验收**（不属本检查器的判域）',
        'effect': '本检查器对 R13 不产出任何红/绿；R13 不出现在任何一片的命中行里',
    },
}

# 部分实现登记表：整条规则已登记，**未判的那一半逐条写出来**（不静默）
PARTIAL_RULES = {
    'R9': {
        'done': '依赖闭包闭合：集合里每片的 `depends_on` 目标必须在集合内（`--set-open` 时记「不可判」）',
        'undone': '「该缺陷的每条验收判据至少被一片承接」+「全部 open defect 被覆盖」—— 缺输入 schema：'
                  '缺陷条目与它的验收判据清单在片文件里没有字段可承载（模板未定）；本检查器不猜',
        'evidence': '§3.4-R9（「部分（片集合级）」）；模板 §8-*未给承接口径',
    },
    'R10': {
        'done': '句法侧：context_needed 非空 + 含「落点原文片段」+ 不得把设计稿/别的文档当上下文',
        'undone': '「无设计稿也能复现」的**实跑**那一半（只给片、不给设计稿，跑一次看能否复现）—— 非纯句法',
        'evidence': '§3.4-R10（「部分（另需「只给片」实跑一次，非纯句法）」）',
    },
}

# 路由规则登记表：超限**不打回**，只产出「需递归」标记（逐字 §3.4-R7 的 ✗）
ROUTE_RULES = {
    'R7': '超限判"需递归"，而不是打回（§3.4-R7 的 ✗）⇒ 本条**永不产出红色**，只产出「需递归」标记',
}

# 已登记**可被禁用**的规则（`--disable-rule` 只认这张表的键；别的键 ⇒ rc=2）
REGISTERED_DISABLE = {
    'R9': 'R9 的未判部分见 PARTIAL_RULES；整体禁用只关掉已实现的那一半（依赖闭包闭合）',
    'R10': 'R10 的未判部分见 PARTIAL_RULES；整体禁用只关掉已实现的那一半（句法侧自包含性）',
}

# 非 R 判据登记表：模板 §1 字段表 / §2 冲突优先级 / §3.3 继承四条 —— 是共同规则表的另一半，
# **不是新规则**（模板 §7：共同规则表 = §1 表 + §2 + §3 继承四条 + §4 的 R0–R13）
CLAUSE_TABLE = [
    ('§1-pre-defect_id', 'REQUIRED', '`defect_id` 必填（一片只绑一个 ⇒ 另见 R1）'),
    ('§1-pre-landing', 'REQUIRED', '`landing` 必填且现读得出（路径样式/存在性 ⇒ 另见 R2）'),
    ('§1-pre-open_questions', 'S1-PRE-OPENQ',
     '`open_questions` 必填；**无则必须写「无」**（留空 ≠ 写「无」）；每条 ≤1 项；仅限进执行前'),
    ('§1-post-criteria', 'REQUIRED', '`criteria` 必填（三件套 ⇒ 另见 R3）'),
    ('§1-post-gate_cmds', 'REQUIRED', '`gate_cmds` 必填（每条带 expect ⇒ 另见 R6）'),
    ('§1-post-output_format', 'S1-POST-NEGCTRL',
     '`output_format` 必须含**「改动前判据必红」的负控证据**（非空非占位 ⇒ 另见 R5）'),
    ('§1-post-exit_cond', 'REQUIRED', '`exit_cond` 必填（结构化三项 ⇒ 另见 R12）'),
    ('§1-inv-hard_limits', 'REQUIRED', '`hard_limits` 必填（非空非占位 ⇒ 另见 R5）'),
    ('§1-inv-side_effects', 'S1-INV-SIDEEFFECTS',
     '`side_effects` ∈ {只读, 写工作树, 触网, git 写, 删除} **之一**'),
    ('§1-inv-escalate_if', 'S1-INV-ESCALATEIF', '有 `side_effects` 即须配 `escalate_if`（表外字段）'),
    ('§1-inv-budget', 'S1-INV-BUDGET', '`budget` 必须给**标定出处**；常数只能当初始值；无出处判红'),
    ('§1-inv-atomicity', 'S1-INV-ATOMICITY',
     '`atomicity` = 落在执行者**标定技能/门禁闭包**内 + `criteria` 可在**一次调用内**判定'),
    ('§1-inv-slice_id', 'S1-INV-SLICEID', '`slice_id` 必填、非「待定」，且与 `task_id` 一致（片 = 单子）'),
    ('§1-inv-depends_on', 'REQUIRED', '`depends_on` 必填；**无依赖写「无」**（空数组会被 R5 形态命中）'),
    ('§1-out-the_one_thing', 'REQUIRED', '`the_one_thing` 必填（R0 的判据对象）'),
    ('§1-out-context_budget', 'REQUIRED', '`context_budget` 必填（R11 的判据对象）'),
    ('§2-conflict', 'S2-CONFLICT',
     '冲突时必须输出 `CONFLICT` 标记、不得自行裁量；**有冲突却未标记 ⇒ 判红**'),
    ('§3.3-inherit', 'S33-INHERIT',
     '子合同继承四条：子 pre ⊇ 父 pre / 子 post ⊆ 父 post / 子 hard_limits ⊆ 父 hard_limits / '
     '父 exit_cond 由子集合共同满足 ⇒ 违反任一条判红'),
]

# 必填字段登记表：(字段 → 缺件时判红所用编号)
REQUIRED_FIELDS = [
    ('defect_id', '§1-pre-defect_id'), ('landing', '§1-pre-landing'),
    ('context_needed', '§1-pre-context_needed'), ('open_questions', '§1-pre-open_questions'),
    ('criteria', '§1-post-criteria'), ('gate_cmds', '§1-post-gate_cmds'),
    ('output_format', '§1-post-output_format'), ('exit_cond', '§1-post-exit_cond'),
    ('hard_limits', '§1-inv-hard_limits'), ('side_effects', '§1-inv-side_effects'),
    ('budget', '§1-inv-budget'), ('atomicity', '§1-inv-atomicity'),
    ('slice_id', '§1-inv-slice_id'), ('depends_on', '§1-inv-depends_on'),
    ('the_one_thing', '§1-out-the_one_thing'), ('context_budget', '§1-out-context_budget'),
]

# 片文件的**已知键**（strict：别的键 ⇒ 错误 rc=2）
# ★ 可选字段登记表（Q15 / C5 · 2026-09-18 edit-assert 开工时登记）：
#   依据：`docs/01-设计/设计-改码与变异安全门-v1.1.md` §4.2（期望命中数写进片）
#        + §9 Q15 + 附录A C5（strict 白名单是「期望值写进片」的**硬墙**：未知键 ⇒ rc=2）。
#   口径三条：① **不**进 REQUIRED_FIELDS（那 16 个一个不动）；② **只**进 KNOWN_SLICE_KEYS（strict 白名单）；
#            ③ 不给该字段的片 ⇒ 行为与登记前**逐字一致**（不得因此变红；本条即「反例」的守卫）。
#   值形态（由 edit-assert 消费，本检查器**不**判其内部结构）：一组 {target, expect, reason}。
OPTIONAL_FIELDS = [
    ('edit_assert_expect', '§1-post-edit_assert_expect',
     '期望命中数（写盘门 edit-assert 的 E2 判据对象）：一组 `(target, expect, reason)`，形态见 §4.2 四形态'),
]
FIELD_KEYS = [f for f, _ in REQUIRED_FIELDS] + ['escalate_if'] + [f for f, _ref, _desc in OPTIONAL_FIELDS]
META_KEYS = ['conflicts', 'parent', 'task_id', 'rules', '_note']
KNOWN_SLICE_KEYS = set(FIELD_KEYS) | set(META_KEYS)

# 登记口径表（取值域由本检查器实施时定稿并登记 —— 模板 §8-4 明示「由检查器实施时定稿并登记」）
REG_VERBS = [
    # R0：动作段（「必须」之后）的动词词表；最长匹配、非重叠。
    # 口径登记：这是**词表法**，不是句法分析（登记见 PARTIAL/未实现清单）——所以词表只收
    # 「动作段里几乎只会作为动词出现」的词，不收「落/标/注/设」这类会在名词里出现的单字。
    '写明', '写进', '写入', '写成', '写出', '补上', '补齐', '补入', '给出', '照抄',
    '替换为', '替换', '换成', '改为', '改成', '修正', '修好', '删除', '移除', '去掉',
    '重构', '重命名', '迁移', '新建', '落盘', '同步', '更新', '升级', '执行', '运行',
    '实现', '导出', '打印', '接入', '生成', '插入', '整理', '改造', '修复', '封住',
    '拦住', '指定', '删掉', '添加', '加上', '加入', '添上', '写', '补', '加', '改',
    '修', '建', '跑', '换', '带', '放', '贴', '抄',
]
REG_CONJ = ['并', '并且', '同时', '以及', '一起', '和', '与']

REG_R4 = [
    # (层, 词/式子, 改写建议) —— R4 命中**必须**附改写建议，否则不算有效报红（本表每行都带 fix）
    ('含糊', '相关', '把「相关」改写为具体路径或符号，例：`docs/01-设计/切片合同-模板.md:1`'),
    ('含糊', '大概', '改写为具体数值并标出处，例：`wall_s_max: 43.6`，出处 `~/.zerg/egg-profiles/task-budget-calib.yaml`'),
    ('含糊', '大致', '改写为具体数值并标出处（例：`steps_max: 3`）'),
    ('含糊', '差不多', '改写为具体数值（例：「≤3 步」）'),
    ('含糊', '一些', '改写为确定条数（例：「3 条」）'),
    ('含糊', '若干', '改写为确定条数（例：「2 项」）'),
    ('含糊', '适当', '写出具体判据（例：「≤3 步」）'),
    ('含糊', '尽量', '改写为 MUST 判据（例：「必须 ≤3 步」）'),
    ('含糊', '必要时', '写出触发条件（例：「当 X 命中时」）'),
    ('含糊', '一般来说', '写死条件（例：「当 τ 命中时」）'),
    ('含糊', '可能', '改写为条件句（例：「若 X 命中则…」）'),
    ('含糊', '有点', '改写为可判阈值'),
    ('含糊', '看情况', '写出条件与分支（例：「当 A 时…否则…」）'),
    ('含糊', '视情况', '写出条件与分支（例：「当 A 时…否则…」）'),
    ('含糊', '按惯例', '写出具体格式（例：「贴命令 + 原始输出 + 退出码」）'),
    ('含糊', '合理', '写出可判阈值（例：「假红率 ≤ 10%」）'),
    ('含糊', '等等', '列全条目并给条数（例：「C1 · C2 · C3，共 3 条」）'),
    ('含糊', '之类的', '列全条目（例：「红/绿」两种状态写全）'),
    ('含糊', '等其他', '列全条目'),
    ('含糊', '设计稿', '把设计稿里的原文片段**抄进** context_needed（例：v2.1 §3.3 抬头那一行原文）'),
    ('含糊', '模板那边', '改写为具体路径（例：`docs/01-设计/切片合同-模板.md`）'),
    ('含糊', '那边', '改写为具体路径'),
    ('含糊', '做完为止', '写出可观察的成功/失败条件（例：`C1 ∧ C2 ∧ C3`）'),
    ('含糊', '后续再说', '改写为 `NEEDS-CLARIFICATION` 或写死条件'),
    ('含糊', '效果良好', '改写为可判判据（例：「C1 输出 = 1 且 rc=0」）'),
    ('含糊', '效果好', '改写为可判判据'),
    ('含糊', '跑一下', '写出可复制命令（例：`python3 scripts/check-slice.py --probe`）'),
    ('含糊', '优化一下', '写出优化项与阈值（例：「token ≤ 118」）'),
    ('含糊', '差不多就行', '改写为具体阈值'),
    ('含糊', '有问题再说', '改写为可观察失败条件'),
    ('可选', '可以', '改写为全大写 MUST / MAY（BCP 14 只认全大写关键词；「可以」不是关键词）'),
    ('可选', '可选', '写全大写 MAY（或不写）'),
    ('可选', '可选地', '写全大写 MAY'),
    ('可选', '也许', '写死条件或不写'),
    ('可选', r'\bmay\b', '改写为全大写 MUST 或 MAY（小写不计入机器判据，RFC 8174 逐字）'),
    ('可选', r'\bcan\b', '改写为全大写 MUST 或 MAY（小写不计入机器判据）'),
    ('可选', r'\bcould\b', '改写为全大写 MUST 或 MAY'),
    ('可选', r'\boptionally\b', '改写为全大写 MAY'),
    ('主观', '良好', '改写为可判阈值（例：「3/3 条由绿变红」）'),
    ('主观', '优秀', '改写为可判阈值'),
    ('主观', '美观', '改写为可判阈值（例：「像素差 = 0」）'),
    ('主观', '优雅', '改写为可判阈值'),
    ('主观', '简洁', '改写为可判阈值（例：「≤ 30 行」）'),
    ('主观', '顺畅', '改写为可判阈值（例：「wall ≤ 43.6s」）'),
    ('主观', '流畅', '改写为可判阈值'),
    ('主观', '清晰', '改写为可判阈值'),
    ('主观', '觉得', '删去主观判断，写可观察判据'),
    ('主观', '应该没问题', '写可观察判据'),
    ('主观', '体验好', '写可判阈值'),
    ('主观', '稳定', '写可判阈值（例：「连续 3 次全绿」）'),
]

REG_PLACEHOLDER = [
    '按惯例', '做完为止', '看情况', 'TODO', 'TBD', 'XXX', '待定', '待补', '占位',
    'N/A', 'n/a', '略', '同前', '同上', 'NEEDS-CLARIFICATION-?',
]
REG_PLACEHOLDER = [p for p in REG_PLACEHOLDER if p != 'NEEDS-CLARIFICATION-?']

REG_EXPECT_KEYS = ['exit', 'stdout', 'stderr', 'contains', 'regex', 'eq']

REG_EXIT_TRAP = [
    (r'\bgofmt\s+-l\b', 'gofmt -l 列出文件时仍 exit 0'),
    (r'\bgo\s+test\b', 'go test 无测试文件仍 exit 0'),
    (r'\bpytest\b', 'pytest 无收集单列退码 5'),
]

REG_SNIPPET_KINDS = ['snippet']
REG_CTX_KINDS = ['snippet', 'file', 'cmd', 'note']

# R11 顺序口径：criteria 必须出现在这些 post/invariant 字段之前
R11_AFTER_CRITERIA = ['gate_cmds', 'output_format', 'exit_cond', 'hard_limits', 'side_effects',
                      'escalate_if', 'budget', 'atomicity', 'slice_id', 'depends_on',
                      'context_budget', 'conflicts']

SIDE_EFFECT_ENUM = ['只读', '写工作树', '触网', 'git 写', '删除']

STATE_RED_WORDS = ['红', 'red', 'RED']
STATE_GREEN_WORDS = ['绿', 'green', 'GREEN']

FULLSCAN_RE = re.compile(r'\./\.\.\.|全量|\ball\b')
PATHY_EXT_RE = re.compile(r'\.(md|py|sh|go|json|yaml|yml|txt|toml|cfg|ini|sql|db|ts|js)\b')

# 明确**不判**登记表（不静默：本检查器判域之外的东西，一律逐条登记并打印）
NOT_CHECKED = [
    ('§3.3-抬头 BCP 14',
     '模板 §0 的「抬头 BCP 14 固定句存在性」**不属 R0–R13** ⇒ 本检查器不判（不得自创 R14）；'
     '片文件里也没有承载该抬头句的字段'),
    ('§3.3-递归深度',
     '递归切片「深度上限 2」与「同一 (defect_id, landing) 再现 ⇒ 判环升级」—— 片文件无「深度」字段、'
     '标定档案无深度键、R0–R13 无对应条目 ⇒ 本检查器只判「同 (landing, defect_id) 重复 ⇒ R8」，深度不判'),
    ('§3.3-取词口径',
     '「小写 must/should 不计入机器判据」的取词口径（只认全大写 MUST/SHOULD/MAY）未实施；'
     'R4 只判小写 may/can/could/可选 这类模糊可选词'),
    ('§3.2-atomicity 闭包',
     '「执行者标定技能/门禁闭包」字段在标定档案里**不存在** ⇒ 闭包本身记「未标定」，'
     '只判文本是否同时给出「闭包」与「一次调用内可判」（登记口径）'),
    ('R0-词表法',
     '「只含一个动词」用**词表法**近似（词表见上方「判据口径来源」块、最长匹配非重叠），不是句法分析：'
     '词表外的动作词会判 R0-NO-VERB；词表词的非动词用法（如「附带」含「带」）可能判 R0-MULTIVERB'),
    ('档案绑定上界',
     'R7 行数上界、§3.6 探针条数门槛、§3.5 假红上限一律**先从标定档案读键**，缺则用登记默认并打印来源；'
     '现档案只有 steps_max / tool_calls_max / wall_s_max / assistant_tokens_est_max'),
]

# 探针门槛（§3.6：≥10 条含糊 + ≥10 条合法；§3.5：假绿 = 0、假红 ≤ 10%）——
# 取值来源：标定档案（键 `probe_vague_min` / `probe_legal_min` / `probe_fp_rate_max`），
# 档案缺这些键 ⇒ 用登记默认值并**打印**（与「上界取自标定档案」同规，不静默）
PROBE_DEFAULTS = {'probe_vague_min': 10, 'probe_legal_min': 10, 'probe_fp_rate_max': 0.10}


# ══════════════════════════════════════════════════════════════════
# 2. 结果容器与内部错误
# ══════════════════════════════════════════════════════════════════

class InternalError(Exception):
    """内部错误 ⇒ rc=2，不给结论（例：命中给不出改写建议 —— R4 约束）。"""


class Result(object):
    def __init__(self, name):
        self.name = name
        self.hits = []        # [{rule, code, msg, fix}]
        self.notes = []       # 非红注记（未标定 / 不可判 / 登记项）
        self.recursion = []   # R7「需递归」标记

    def hit(self, rule, code, msg, fix):
        """加一条命中。**fix 必填**：无改写建议的命中不算有效报红（§3.4-R4）⇒ 拒判 rc=2。"""
        if not fix or not str(fix).strip():
            raise InternalError(
                '命中 %s / %s 缺「改写建议」⇒ 按 §3.4-R4「无改写建议不算有效报红」拒绝出结论' % (rule, code))
        self.hits.append({'rule': rule, 'code': code, 'msg': msg, 'fix': fix})

    def note(self, code, msg):
        self.notes.append({'code': code, 'msg': msg})

    def need_recursion(self, code, msg):
        self.recursion.append({'code': code, 'msg': msg})

    def red_rules(self):
        out = []
        for h in self.hits:
            if h['rule'] not in out:
                out.append(h['rule'])
        return out

    def is_red(self):
        return bool(self.hits)


def _nonempty(x):
    if x is None:
        return False
    if isinstance(x, str):
        return x.strip() != ''
    if isinstance(x, (list, dict)):
        return len(x) > 0
    return True


def _as_text(x):
    if x is None:
        return ''
    if isinstance(x, str):
        return x
    try:
        return json.dumps(x, ensure_ascii=False)
    except (TypeError, ValueError):
        return str(x)


# ══════════════════════════════════════════════════════════════════
# 3. 逐规则判据
# ══════════════════════════════════════════════════════════════════

def count_verbs(text):
    """R0 口径：动词计数（登记词表、最长匹配、非重叠）。"""
    n = 0
    i = 0
    words = sorted(REG_VERBS, key=len, reverse=True)
    while i < len(text):
        for w in words:
            if text.startswith(w, i):
                n += 1
                i += len(w)
                break
        else:
            i += 1
    return n


def r4_scan(res, where, text):
    if not isinstance(text, str) or not text.strip():
        return
    for layer, pat, fix in REG_R4:
        rx = pat if pat.startswith('\\b') else re.escape(pat)
        m = re.search(rx, text)
        if m:
            code = {'含糊': 'R4-VAGUE', '可选': 'R4-OPTIONAL', '主观': 'R4-SUBJECTIVE'}[layer]
            lo = max(0, m.start() - 12)
            frag = text[lo:m.end() + 12].replace('\n', ' ')
            res.hit('R4', code,
                    '%s[%s] 命中无界词「%s」：…%s…' % (where, layer, pat, frag), fix)


def r0(res, sl):
    v = sl.get('the_one_thing')
    if not _nonempty(v) or not isinstance(v, str):
        res.hit('R0', 'R0-MISSING',
                'the_one_thing 缺失/为空',
                '写成一句话，照句式：「When/While ⟨条件⟩，⟨落点⟩ 必须 ⟨单一动作⟩」（只含一个动词）')
        return
    s = v.strip()
    m = re.match(r'^(When|While)\s+(.+?)[，,]\s*(.+)$', s)
    if not m:
        res.hit('R0', 'R0-SHAPE',
                'the_one_thing 套不进「When/While ⟨条件⟩，⟨落点⟩ 必须 ⟨单一动作⟩」：%s' % s[:60],
                '改写成：When/While ⟨条件⟩，⟨落点⟩ 必须 ⟨单一动作⟩，例：'
                '「When 模板骨架已落盘时，docs/01-设计/切片合同-模板.md 的抬头必须写明 BCP 14 固定句。」')
        return
    tail = m.group(3)
    if '必须' not in tail:
        res.hit('R0', 'R0-NO-MUST',
                '⟨落点⟩ 之后缺「必须」（正面句式的谓词）：%s' % tail[:60],
                '在⟨落点⟩后补「必须 ⟨单一动作⟩」，例：「…的抬头必须写明 BCP 14 固定句。」')
        return
    action = tail.split('必须', 1)[1]
    nverbs = count_verbs(action)
    nconj = sum(1 for c in REG_CONJ if c in action)
    if nverbs != 1:
        code = 'R0-MULTIVERB' if nverbs > 1 else 'R0-NO-VERB'
        res.hit('R0', code,
                '⟨单一动作⟩ 段动词数 = %d%s（只允许 1 个）：%s'
                % (nverbs, '（另含并列词）' if nconj else '', action[:60]),
                '只留一个动词，例：「必须写明 BCP 14 固定句。」（删去并/和/同时 之类的并列动作）')


def r1(res, sl):
    v = sl.get('defect_id')
    if isinstance(v, list):
        ids = [x for x in v if _nonempty(x)]
    elif isinstance(v, str):
        s = v.strip()
        if s in ('无', 'NEEDS-CLARIFICATION'):
            ids = [s]
        else:
            ids = [t for t in re.split(r'[,，、;；\s]+', s) if t]
    elif _nonempty(v):
        ids = [str(v)]
    else:
        ids = []
    if len(ids) > 1:
        res.hit('R1', 'R1-MULTI-DEFECT',
                '一片绑了 %d 个 defect_id：%s' % (len(ids), ' / '.join(ids)),
                '拆成 %d 片，每片只绑一个 defect_id（例：本片留 `%s`，其余另开片）' % (len(ids), ids[0]))
    elif not ids:
        res.hit('R1', 'R1-NO-DEFECT',
                'defect_id 缺失/为空',
                '填一个本片所修缺陷的 id，例：`DEF-20260918-S3-01`')


def landing_candidates(landing):
    s = landing.strip()
    cands = []
    parts = s.split()
    first = parts[0] if parts else s
    if ':' in first:
        cands.append(first.split(':', 1)[0])
    cands.append(first)
    cands.append(s.split(':')[0].split(' ', 1)[0])
    out = []
    for c in cands:
        c = c.strip().rstrip('。，,;；)）】」')
        if c and c not in out:
            out.append(c)
    return out


def r2(res, sl):
    v = sl.get('landing')
    if not _nonempty(v) or not isinstance(v, str):
        res.hit('R2', 'R2-NO-PATH',
                'landing 缺失/为空（无路径样式）',
                '写成「文件路径:行号/符号」，例：`docs/01-设计/切片合同-模板.md:1`')
        return
    cands = landing_candidates(v)
    pathy = [c for c in cands if ('/' in c or PATHY_EXT_RE.search(c))]
    if not pathy:
        res.hit('R2', 'R2-NO-PATH',
                'landing 无路径样式（无 `/`、无登记扩展名）：%s' % v[:60],
                '改写成具体路径或「路径:符号」，例：`docs/01-设计/切片合同-模板.md:1`')
        return
    found = None
    for c in pathy:
        p = c if os.path.isabs(c) else os.path.join(REPO_ROOT, c)
        if os.path.exists(p):
            found = c
            break
    if found is None:
        res.hit('R2', 'R2-NOT-FOUND',
                'landing 的路径样式**当前不存在**（前状态检查不过）；试过：%s' % ' , '.join(pathy),
                '改成现读得出的落点，例：先把目标文件建出来（`touch <路径>`）或把 landing 指向已存在文件')
    else:
        res.note('R2-OK', 'landing 前状态检查通过：%s' % found)


def r3(res, sl):
    v = sl.get('criteria')
    if not _nonempty(v):
        res.hit('R3', 'R3-MISSING',
                'criteria 缺失/为空',
                '写成列表，每项 = 三件套 {id, cmd, expect, state}，例：'
                '{"id":"C1","cmd":"awk ... | grep -c X","expect":"1","state":"改动前：红｜落盘后：绿"}')
        return
    if isinstance(v, str):
        res.hit('R3', 'R3-NON-STRUCTURED',
                'criteria 是自由文本（三件套缺：无结构化命令 / 期望输出 / 当前红-绿状态）：%s' % v[:60],
                '改成列表，每项 = {id, cmd, expect, state}；命令必须可复制、expect 必须写死、state 写明当前红还是绿')
        return
    if not isinstance(v, list):
        res.hit('R3', 'R3-NON-STRUCTURED', 'criteria 不是列表（实为 %s）' % type(v).__name__,
                '改成列表，每项 = {id, cmd, expect, state}')
        return
    for idx, item in enumerate(v):
        tag = 'criteria[%d]' % idx
        if not isinstance(item, dict):
            res.hit('R3', 'R3-NON-STRUCTURED', '%s 不是 {id,cmd,expect,state} 结构' % tag,
                    '改成对象 {id, cmd, expect, state}')
            continue
        if not _nonempty(item.get('id')):
            res.hit('R3', 'R3-MISSING-ID', '%s 缺 id（exit_cond 成功项要引用它，R12）' % tag,
                    '补一个稳定 id，例：`"id": "C1"`')
        if not _nonempty(item.get('cmd')):
            res.hit('R3', 'R3-MISSING-CMD', '%s 缺「可复制命令」' % tag,
                    '补可原样复制运行的命令，例：`"cmd": "awk \'/^## 0\\./,/^## 1\\./\' <文件> | grep -c X"`')
        if not _nonempty(item.get('expect')):
            res.hit('R3', 'R3-MISSING-EXPECT', '%s 缺「期望输出」' % tag,
                    '补 expect 并**写死**期望值，例：`"expect": "1"`（无期望输出 ⇒ 无从判红绿 ⇒ 必假绿）')
        st = item.get('state')
        if not _nonempty(st):
            res.hit('R3', 'R3-MISSING-STATE', '%s 缺「当前红/绿状态」' % tag,
                    '补 state，例：`"state": "改动前：红（输出 0，rc=1）｜落盘后：绿（输出 1）"`')
        elif isinstance(st, str) and not any(w in st for w in STATE_RED_WORDS + STATE_GREEN_WORDS):
            res.hit('R3', 'R3-STATE-NO-REDGREEN',
                    '%s 的 state 未写红/绿：%s' % (tag, st[:40]),
                    '把当前状态写成「红」或「绿」，例：`"state": "改动前：红"`')


def r5(res, sl):
    fields = ['hard_limits', 'side_effects', 'output_format', 'exit_cond']
    for f in fields:
        if f not in sl:
            res.hit('R5', 'R5-MISSING', '%s 缺失（空壳检测对象四字段之一）' % f,
                    '填 %s；未决就写 `NEEDS-CLARIFICATION`（不得留空、不得留 `[]`/`<>`）' % f)
            continue
        v = sl[f]
        if not _nonempty(v):
            name = '空列表/空对象'
            if isinstance(v, str) or v is None:
                name = '空字符串/None'
            res.hit('R5', 'R5-EMPTY', '%s 是空壳（%s；空数组 = 未替换的 `[]` 形态）' % (f, name),
                    '填实质内容；未决就写 `NEEDS-CLARIFICATION`')
            continue
        txt = _as_text(v)
        for ph in REG_PLACEHOLDER:
            if ph in txt:
                res.hit('R5', 'R5-PLACEHOLDER',
                        '%s 命中占位文本「%s」：%s' % (f, ph, txt[:50]),
                        '把「%s」改写成实质内容或 `NEEDS-CLARIFICATION`' % ph)
                break
        if '[]' in txt or '<>' in txt:
            res.hit('R5', 'R5-UNREPLACED',
                    '%s 含未替换的 `[]`/`<>`：%s' % (f, txt[:50]),
                    '把 `[]`/`<>` 换成实际内容；未决就写 `NEEDS-CLARIFICATION`')


def r6(res, sl):
    v = sl.get('gate_cmds')
    if not _nonempty(v):
        res.hit('R6', 'R6-MISSING', 'gate_cmds 缺失/为空',
                '写成 `[{cmd, expect}]`，例：`[{"cmd": "… | grep -c X", "expect": {"exit": 0, "stdout": "1"}}]`')
        return
    if not isinstance(v, list):
        res.hit('R6', 'R6-NON-LIST', 'gate_cmds 不是列表（实为 %s）' % type(v).__name__,
                '改成列表，每项 = `{cmd, expect}`')
        return
    for idx, item in enumerate(v):
        tag = 'gate_cmds[%d]' % idx
        if not isinstance(item, dict):
            res.hit('R6', 'R6-NON-PARSE', '%s 不是 `{cmd, expect}` 结构' % tag,
                    '改成 `{"cmd": "…", "expect": {...}}`')
            continue
        cmd = item.get('cmd')
        if not _nonempty(cmd) or not isinstance(cmd, str):
            res.hit('R6', 'R6-NON-PARSE', '%s 的 cmd 缺失或不是可解析字符串' % tag,
                    '把 cmd 写成一行可复制命令字符串')
            continue
        if 'expect' not in item or not _nonempty(item.get('expect')):
            res.hit('R6', 'R6-NO-EXPECT', '%s 无 expect（cmd=%s）' % (tag, cmd[:40]),
                    '补 expect，例：`"expect": {"exit": 0, "stdout": "1"}`（退出码会假绿 ⇒ 至少再写 stdout）')
            continue
        exp = item.get('expect')
        if isinstance(exp, dict):
            bad = [k for k in exp.keys() if k not in REG_EXPECT_KEYS]
            if bad:
                res.hit('R6', 'R6-EXPECT-KEY-UNKNOWN',
                        '%s 的 expect 含未登记键 %s（登记取值域：%s）'
                        % (tag, ' / '.join(bad), ' · '.join(REG_EXPECT_KEYS)),
                        '改用登记键，例：`{"exit": 0, "stdout": "1"}`')
            keys = set(exp.keys())
        elif isinstance(exp, str):
            keys = set(['str'])
        else:
            res.hit('R6', 'R6-EXPECT-NON-PARSE',
                    '%s 的 expect 不可解析（实为 %s）' % (tag, type(exp).__name__),
                    '把 expect 写成 dict（`{"exit":0,"stdout":"1"}`）或非空字符串')
            keys = set()
        if keys == set(['exit']):
            for pat, why in REG_EXIT_TRAP:
                if re.search(pat, cmd):
                    res.hit('R6', 'R6-EXIT-ONLY-TRAP',
                            '%s 只给了 exit 期望，而该命令是登记过的假绿形态（%s）' % (tag, why),
                            '补 stdout/contains 期望，例：`"expect": {"exit": 0, "stdout": "1"}`；'
                            '或把命令换成能由绿变红的形态（如 `… | grep -c X`）')


def landing_primary(landing):
    """landing 的**主落点文件**：优先取「存在的那一个候选」，否则取第一个路径样候选。"""
    cands = landing_candidates(landing)
    for c in cands:
        p = c if os.path.isabs(c) else os.path.join(REPO_ROOT, c)
        if os.path.exists(p):
            return c
    pathy = [c for c in cands if ('/' in c or PATHY_EXT_RE.search(c))]
    return pathy[0] if pathy else None


def r7(res, sl, calib):
    """粒度（路由规则）：落点文件数 >1 ⇒ 需递归；行数上界取标定档案（缺 ⇒ 未标定，不猜）。"""
    files = set()
    land = sl.get('landing')
    if isinstance(land, str) and land.strip():
        prim = landing_primary(land)
        if prim:
            files.add(prim)
    ctx = sl.get('context_needed')
    if isinstance(ctx, list):
        for it in ctx:
            if isinstance(it, dict) and it.get('kind') == 'file' and _nonempty(it.get('value')):
                files.add(str(it['value']))
    if len(files) > 1:
        res.need_recursion('R7-NEEDS-RECURSION-MULTIFILE',
                           '落点文件数 = %d（>1）：%s ⇒ 判「需递归」，不打回（§3.4-R7）'
                           % (len(files), ' , '.join(sorted(files))))
    bound = calib.get('slice_lines_max') or calib.get('slice_line_bound')
    if not bound:
        res.note('R7-UNCALIBRATED',
                 '行数上界「未标定」：`%s` 无 `slice_lines_max` 键 ⇒ 记未标定，不猜（§3.4-R7 上界 = 本仓标定取值）'
                 % os.path.basename(CALIB_PATH))
        return
    try:
        bnum = int(float(bound))
    except (TypeError, ValueError):
        res.note('R7-UNCALIBRATED', '行数上界值不可解析（%s）⇒ 记未标定，不猜' % bound)
        return
    for c in sorted(files):
        p = c if os.path.isabs(c) else os.path.join(REPO_ROOT, c)
        if not os.path.isfile(p):
            continue
        try:
            with open(p, 'r', encoding='utf-8', errors='replace') as fh:
                n = sum(1 for _ in fh)
        except (IOError, OSError):
            continue
        if n > bnum:
            res.need_recursion('R7-NEEDS-RECURSION-LINES',
                               '落点 %s 现行行数 %d > 标定上界 %d ⇒ 判「需递归」，不打回（§3.4-R7）'
                               % (c, n, bnum))


def r8(res, sl, sl_id, all_slices):
    dep = sl.get('depends_on')
    deps = []
    if isinstance(dep, list):
        deps = [str(x) for x in dep if _nonempty(x)]
    elif _nonempty(dep):
        s = str(dep).strip()
        if s != '无':
            deps = [t for t in re.split(r'[,，、;；\s]+', s) if t]
    if not _nonempty(dep):
        res.note('R8-NO-DEPENDS-ON', 'depends_on 缺失/为空：§1 表要求**无依赖写「无」**（空数组会被 R5 形态命中）')
    if sl_id and sl_id in deps:
        res.hit('R8', 'R8-SELF-DEP', 'depends_on 含自身 slice_id（%s）⇒ 成环' % sl_id,
                '删掉自身依赖；本片无依赖就写 `"depends_on": "无"`')
    # 集合级
    if len(all_slices) < 2:
        res.note('R8-SINGLE', '单片模式：拓扑排序 / 重复检查不适用（片集合级，见 §3.4-R8）')
    else:
        ids = {}
        for s in all_slices:
            sid = s['obj'].get('slice_id')
            if _nonempty(sid):
                ids.setdefault(str(sid), []).append(s['path'])
        for sid, paths in ids.items():
            if len(paths) > 1:
                res.hit('R8', 'R8-DUP-SLICE-ID',
                        'slice_id `%s` 在集合里出现 %d 次：%s' % (sid, len(paths), ' , '.join(paths)),
                        '片 = 单子：一个 slice_id 只能有一片；给重复的那片换 id（或合并两片）')
        seen = {}
        for s in all_slices:
            key = (_as_text(s['obj'].get('landing')).strip(), _as_text(s['obj'].get('defect_id')).strip())
            if key[0] or key[1]:
                seen.setdefault(key, []).append(s['path'])
        for key, paths in seen.items():
            if len(paths) > 1:
                res.hit('R8', 'R8-DUP-LANDING-DEFECT',
                        '同一 (landing, defect_id) 重复出现 %d 次：%s（%s）'
                        % (len(paths), ' , '.join(paths), key[0]),
                        '同一 landing+defect_id 只允许一片；合并重复片或改 landing/defect_id')
        # Kahn 拓扑
        graph = {}
        for s in all_slices:
            sid = str(s['obj'].get('slice_id') or s['path'])
            graph[sid] = []
            d = s['obj'].get('depends_on')
            if isinstance(d, list):
                graph[sid] = [str(x) for x in d if _nonempty(x)]
            elif _nonempty(d) and str(d).strip() != '无':
                graph[sid] = [t for t in re.split(r'[,，、;；\s]+', str(d)) if t]
        cyc = find_cycle(graph)
        if cyc:
            res.hit('R8', 'R8-CYCLE',
                    'depends_on 成环（拓扑排序失败）：%s' % ' → '.join(cyc),
                    '打断环：把环上某一片的 depends_on 指向环外（或拆片）；例：把 S3-C2 的 depends_on 改为 `"无"`')


def find_cycle(graph):
    """Kahn 拓扑排序；剩余节点 ⇒ 有环。返回一个环（节点序列）或 None。"""
    indeg = dict((k, 0) for k in graph)
    for k, deps in graph.items():
        for d in deps:
            if d in indeg:
                indeg[k] += 1
    queue = [k for k in indeg if indeg[k] == 0]
    seen = set()
    while queue:
        n = queue.pop()
        seen.add(n)
        for k, deps in graph.items():
            if n in deps:
                indeg[k] -= 1
                if indeg[k] == 0:
                    queue.append(k)
    left = [k for k in graph if k not in seen]
    if not left:
        return None
    # 从剩余节点里走一条环出来（用于打印）
    path = [left[0]]
    cur = left[0]
    for _ in range(len(left) + 1):
        deps = [d for d in graph.get(cur, []) if d in left]
        if not deps:
            break
        nxt = deps[0]
        if nxt in path:
            return path[path.index(nxt):] + [nxt]
        path.append(nxt)
        cur = nxt
    return left


def r9(res, all_slices, set_open):
    if len(all_slices) < 2:
        res.note('R9-SINGLE', '单片模式：依赖闭包 / 停机判据不适用（片集合级，见 §3.4-R9）')
        res.note('R9-UNDONE', PARTIAL_RULES['R9']['undone'])
        return
    ids = set()
    for s in all_slices:
        sid = s['obj'].get('slice_id')
        if _nonempty(sid):
            ids.add(str(sid))
    missing = []
    for s in all_slices:
        d = s['obj'].get('depends_on')
        deps = []
        if isinstance(d, list):
            deps = [str(x) for x in d if _nonempty(x)]
        elif _nonempty(d) and str(d).strip() != '无':
            deps = [t for t in re.split(r'[,，、;；\s]+', str(d)) if t]
        for t in deps:
            if t not in ids:
                missing.append((s['path'], t))
    if set_open:
        res.note('R9-SET-OPEN', '--set-open：该批片不是完整集合 ⇒ 依赖闭包记「不可判」（未判：%d 个外指依赖）' % len(missing))
    elif missing:
        detail = ' ； '.join('%s → %s' % (p, t) for p, t in missing[:5])
        res.hit('R9', 'R9-CLOSURE-OPEN',
                '依赖闭包未闭合：%d 个 depends_on 指向集合外的片（%s）' % (len(missing), detail),
                '把被依赖的片一并纳入本批（或在只查局部时加 `--set-open` 并登记「未判」）')
    res.note('R9-UNDONE', PARTIAL_RULES['R9']['undone'])


def is_snippet_item(x):
    if isinstance(x, dict):
        return x.get('kind') == 'snippet' and _nonempty(x.get('value'))
    if isinstance(x, str):
        return x.strip().startswith('原文片段') or '原文片段：' in x
    return False


def ctx_values(x):
    if isinstance(x, dict):
        return _as_text(x.get('value'))
    return _as_text(x)


def r10(res, sl):
    ctx = sl.get('context_needed')
    if not _nonempty(ctx):
        res.hit('R10', 'R10-EMPTY-CONTEXT', 'context_needed 缺失/为空（执行者可见上下文为零）',
                '补最小上下文：第一条 = 落点原文片段，再列目标文件与判据命令')
        return
    items = ctx if isinstance(ctx, list) else [ctx]
    if not any(is_snippet_item(x) for x in items):
        res.hit('R10', 'R10-NO-SNIPPET',
                'context_needed 里没有「落点原文片段」（自包含性缺一半）',
                '加一条 `{"kind": "snippet", "value": "<落点那一行原文>"}`（固定放最前）')
    for x in items:
        v = ctx_values(x)
        if '设计稿' in v:
            res.hit('R10', 'R10-SEE-DESIGN',
                    'context_needed 把设计稿当上下文：%s' % v[:50],
                    '把设计稿里那段原文**抄进**本字段，例：`{"kind":"snippet","value":"v2.1 §3.3 抬头固定句那一行原文"}`')
        if isinstance(x, dict) and x.get('kind') and x['kind'] not in REG_CTX_KINDS:
            res.hit('R10', 'R10-CTX-KIND-UNKNOWN',
                    'context_needed 条目 kind=%s 未登记（登记：%s）' % (x['kind'], ' · '.join(REG_CTX_KINDS)),
                    '改用登记 kind，例：`{"kind":"file","value":"docs/…"}`')
    res.note('R10-UNDONE', PARTIAL_RULES['R10']['undone'])


def r11(res, sl, calib):
    ctx = sl.get('context_needed')
    items = ctx if isinstance(ctx, list) else ([ctx] if _nonempty(ctx) else [])
    if not items:
        res.note('R11-NO-CTX', 'context_needed 空 ⇒ 顺序断言无对象（另见 R10）')
    elif not is_snippet_item(items[0]):
        res.hit('R11', 'R11-SNIPPET-NOT-FIRST',
                'context_needed 首条不是「落点原文片段」（R11：landing 片段固定放最前）',
                '把落点原文片段移到首条，形态：`{"kind":"snippet","value":"<原文>"}`')
    order = [k for k in sl.keys() if k in FIELD_KEYS or k in META_KEYS]
    if 'criteria' in order:
        ci = order.index('criteria')
        late = [k for k in R11_AFTER_CRITERIA if k in order and order.index(k) < ci]
        if late:
            res.hit('R11', 'R11-CRITERIA-NOT-FIRST',
                    'criteria 未固定放最前：字段键序里它排在 %s 之后' % ' / '.join(late),
                    '把 criteria 移到 pre 栏之后、%s 之前（键序即执行者看到的顺序）' % late[0])
    cb = sl.get('context_budget')
    if not _nonempty(cb):
        res.hit('R11', 'R11-NO-CONTEXT-BUDGET',
                'context_budget 缺失/为空（执行者所需文件/token 上界缺）',
                '补 `{"files": 1, "tokens": 2000, "source": "~/.zerg/egg-profiles/task-budget-calib.yaml"}`')
        return
    if isinstance(cb, dict):
        nums = [k for k in ('files', 'tokens') if isinstance(cb.get(k), (int, float))]
        if not nums:
            res.hit('R11', 'R11-NO-BOUND', 'context_budget 没有数值上界（files / tokens）：%s' % _as_text(cb)[:50],
                    '补数值上界，例：`{"files": 1, "tokens": 2000, "source": "…task-budget-calib.yaml"}`')
        src = _as_text(cb.get('source'))
        if not src.strip():
            res.hit('R11', 'R11-NO-SOURCE', 'context_budget 缺出处（与 budget 同规：无出处判红）',
                    '补 `"source": "~/.zerg/egg-profiles/task-budget-calib.yaml"`')
        elif CALIB_REF not in src and '标定档案' not in src:
            res.hit('R11', 'R11-SOURCE-NOT-CALIB',
                    'context_budget 的出处不指向标定档案：%s' % src[:50],
                    '把出处改为标定档案，例：`"source": "~/.zerg/egg-profiles/task-budget-calib.yaml"`')
    elif isinstance(cb, str):
        if not re.search(r'\d', cb):
            res.hit('R11', 'R11-NO-BOUND', 'context_budget 文本里没有数值上界：%s' % cb[:50],
                    '写成 `{"files": 1, "tokens": 2000, "source": "…task-budget-calib.yaml"}`')
        if CALIB_REF not in cb and '标定档案' not in cb:
            res.hit('R11', 'R11-NO-SOURCE', 'context_budget 文本里没有标定出处：%s' % cb[:50],
                    '把出处改为标定档案，例：`出处 = ~/.zerg/egg-profiles/task-budget-calib.yaml`')
    if not calib:
        res.note('R11-UNCALIBRATED', '标定档案读不到 ⇒ context_budget 上界记「未标定」，不猜')


def r12(res, sl):
    ec = sl.get('exit_cond')
    if not _nonempty(ec):
        res.hit('R12', 'R12-MISSING', 'exit_cond 缺失/为空',
                '写成三项：`{"success": ["C1"], "failure": "<可观察条件>", '
                '"escalate": {"to": "Mr2109", "times": 2, "fields": ["片 id", "命令", "退出码"]}}`')
        return
    if isinstance(ec, str):
        res.hit('R12', 'R12-NON-STRUCTURED',
                'exit_cond 是自由文本（成功/失败/升级三项皆缺）：%s' % ec[:50],
                '改成三项结构：成功引用 criteria id；失败写可观察条件；升级写对象+触发次数+记录字段')
        return
    if not isinstance(ec, dict):
        res.hit('R12', 'R12-NON-STRUCTURED', 'exit_cond 不是对象（实为 %s）' % type(ec).__name__,
                '改成 `{"success": [...], "failure": "...", "escalate": {...}}`')
        return
    ids = []
    crit = sl.get('criteria')
    if isinstance(crit, list):
        for it in crit:
            if isinstance(it, dict) and _nonempty(it.get('id')):
                ids.append(str(it['id']))
    succ = ec.get('success')
    if not _nonempty(succ):
        res.hit('R12', 'R12-MISSING-SUCCESS', 'exit_cond 缺「成功」项（或为空）',
                '补 `"success": ["C1", "C2"]`，列出必须同时成立的 criteria id')
    else:
        toks = succ if isinstance(succ, list) else re.split(r'[,，、;；\s]+', _as_text(succ))
        toks = [str(t) for t in toks if _nonempty(t)]
        unknown = [t for t in toks if ids and t not in ids]
        if not ids:
            res.hit('R12', 'R12-SUCCESS-NO-CRITERIA',
                    'exit_cond.success 引用 criteria id，但本片 criteria 没有可用 id',
                    '先给每条 criteria 一个 id（例 C1），再在 success 里引用它')
        elif unknown:
            res.hit('R12', 'R12-SUCCESS-UNKNOWN-ID',
                    'exit_cond.success 引用了不存在的 criteria id：%s（现存：%s）'
                    % (' / '.join(unknown), ' · '.join(ids)),
                    '把 success 改成现存 id，例：`"success": ["%s"]`' % ids[0])
    if not _nonempty(ec.get('failure')):
        res.hit('R12', 'R12-MISSING-FAILURE', 'exit_cond 缺「失败」（可观察条件）',
                '补 `"failure": "<命令输出与期望不符 / 负控未由绿变红>，附命令、退出码、原始输出"`')
    esc = ec.get('escalate')
    if not _nonempty(esc):
        res.hit('R12', 'R12-MISSING-ESCALATE', 'exit_cond 缺「升级」（对象+触发次数+记录字段）',
                '补 `"escalate": {"to": "Mr2109", "times": 2, "fields": ["片 id", "命令", "退出码", "原文", "时间"]}`')
    elif isinstance(esc, dict):
        if not _nonempty(esc.get('to')):
            res.hit('R12', 'R12-BAD-ESCALATE', 'exit_cond.escalate 缺升级对象（to）',
                    '补 `"to": "Mr2109"`')
        t = esc.get('times')
        if not isinstance(t, int) or t < 1:
            res.hit('R12', 'R12-BAD-ESCALATE', 'exit_cond.escalate 的触发次数（times）不是 ≥1 的整数：%s' % _as_text(t),
                    '补 `"times": 2`（同一 criteria 连续失败次数）')
        f = esc.get('fields')
        if not _nonempty(f) or not isinstance(f, list):
            res.hit('R12', 'R12-BAD-ESCALATE', 'exit_cond.escalate 缺记录字段（fields，非空列表）',
                    '补 `"fields": ["片 id", "命令", "退出码", "原文", "时间"]`')
    else:
        res.hit('R12', 'R12-BAD-ESCALATE', 'exit_cond.escalate 不是对象',
                '改成 `{"to": "Mr2109", "times": 2, "fields": [...]}`')


# ── 非 R 判据（模板 §1 表 / §2 / §3.3 —— 共同规则表的另一半，不是新规则）──

def clause_open_questions(res, sl):
    v = sl.get('open_questions')
    if v is None:
        return  # 缺件已由必填表命中
    if isinstance(v, str):
        if not v.strip() or v.strip() not in ('无', 'NEEDS-CLARIFICATION'):
            res.hit('§1-pre-open_questions', 'S1-PRE-OPENQ',
                    'open_questions 是自由文本（无则必须写「无」）：%s' % v[:40],
                    '无未决就写 `"open_questions": "无"`（留空 ≠ 写「无」）；有疑问写成列表，每条 ≤1 项')
        return
    if isinstance(v, list):
        if not v:
            res.hit('§1-pre-open_questions', 'S1-PRE-OPENQ',
                    'open_questions 是空列表（留空 ≠ 写「无」）',
                    '写成 `"open_questions": "无"`')
            return
        for idx, item in enumerate(v):
            t = _as_text(item)
            nq = t.count('？') + t.count('?')
            nv = count_verbs(t)
            if nq > 1 or nv > 1:
                res.hit('§1-pre-open_questions', 'S1-PRE-OPENQ-MULTI',
                        'open_questions[%d] 不止一项（问号 %d / 动词 %d）：%s' % (idx, nq, nv, t[:40]),
                        '拆成多条，每条只留一个问题、一个动作；例：「是否需要先落 S3-C0？」')
        return
    res.hit('§1-pre-open_questions', 'S1-PRE-OPENQ',
            'open_questions 形态不明（实为 %s）' % type(v).__name__,
            '写「无」或写列表，例：`["是否需要先落 S3-C0？"]`')


def clause_output_format(res, sl):
    v = sl.get('output_format')
    if not _nonempty(v):
        return
    txt = _as_text(v)
    if ('负控' not in txt) or ('红' not in txt):
        res.hit('§1-post-output_format', 'S1-POST-NEGCTRL',
                'output_format 缺「改动前判据必红」的负控证据',
                '补负控段，例：「② 负控（改动前必红）：把 X 改坏后重跑同三条命令，必须红（已实跑：三条由 1 变 0）」')


def clause_side_effects(res, sl):
    se = sl.get('side_effects')
    if not _nonempty(se):
        return
    if isinstance(se, str):
        if se.strip().split()[0] not in SIDE_EFFECT_ENUM:
            res.hit('§1-inv-side_effects', 'S1-INV-SIDEEFFECTS',
                    'side_effects 不在枚举内：%s（枚举：%s）' % (se[:40], ' · '.join(SIDE_EFFECT_ENUM)),
                    '改成五值之一，例：`"side_effects": "写工作树"`')
        return
    if isinstance(se, list):
        bad = [x for x in se if _as_text(x).strip().split()[0] not in SIDE_EFFECT_ENUM]
        if bad:
            res.hit('§1-inv-side_effects', 'S1-INV-SIDEEFFECTS',
                    'side_effects 含枚举外值：%s（枚举：%s）' % (' / '.join(_as_text(b) for b in bad), ' · '.join(SIDE_EFFECT_ENUM)),
                    '改成五值之一，例：`"side_effects": ["写工作树"]`')


def clause_escalate_if(res, sl):
    if _nonempty(sl.get('side_effects')) and not _nonempty(sl.get('escalate_if')):
        res.hit('§1-inv-escalate_if', 'S1-INV-ESCALATEIF',
                '有 side_effects 却没配 escalate_if（表外字段缺 ⇒ 不合格）',
                '补 `"escalate_if": "需要写工作树之外的文件 / 需要 git 写 / 需要删除 ⇒ 升级 Mr2109"`')


def clause_budget(res, sl):
    b = sl.get('budget')
    if not _nonempty(b):
        return
    txt = _as_text(b)
    if isinstance(b, dict):
        src = _as_text(b.get('source'))
    else:
        src = txt if ('出处' in txt or CALIB_REF in txt or '标定档案' in txt) else ''
    if not src.strip() or (CALIB_REF not in src and '标定档案' not in src):
        res.hit('§1-inv-budget', 'S1-INV-BUDGET',
                'budget 没有标定出处（无出处的常数判红）：%s' % txt[:50],
                '补出处，例：`"budget": {"value": "wall_s_max 43.6", '
                '"source": "~/.zerg/egg-profiles/task-budget-calib.yaml"}`')
    elif not re.search(r'\d', txt):
        res.note('S1-INV-BUDGET-NOVALUE', 'budget 有出处但没写出具体数值（登记：不判红，只注记）')


def clause_atomicity(res, sl, calib):
    v = sl.get('atomicity')
    if not _nonempty(v):
        return
    txt = _as_text(v)
    if '闭包' not in txt or ('一次调用' not in txt and '一次' not in txt):
        res.hit('§1-inv-atomicity', 'S1-INV-ATOMICITY',
                'atomicity 未同时给出「闭包」与「一次调用内可判」：%s' % txt[:50],
                '改写成：`"atomicity": "落在执行者标定技能/门禁闭包内，且 criteria 可在一次调用内判定'
                '（<哪几条、各一次什么命令>）"`')
    if not any(k for k in calib if 'closure' in k or 'skill' in k):
        res.note('S1-INV-ATOMICITY-UNCALIBRATED',
                 '「标定技能/门禁闭包」字段在标定档案里不存在 ⇒ 该条记「未标定」，不猜')


def clause_slice_id(res, sl):
    sid = sl.get('slice_id')
    if not _nonempty(sid):
        return
    if isinstance(sid, str) and sid.strip() in ('待定', 'TBD', 'TODO', 'NEEDS-CLARIFICATION'):
        res.hit('§1-inv-slice_id', 'S1-INV-SLICEID',
                'slice_id = 「%s」（占位，未定）' % sid.strip(),
                '填真实单子 id，例：`"slice_id": "S3-C1"`（片 = 单子：同一实体同一 id）')
    tid = sl.get('task_id')
    if _nonempty(tid) and _nonempty(sid) and str(tid) != str(sid):
        res.hit('§1-inv-slice_id', 'S1-INV-SLICEID-TASK',
                'slice_id（%s）≠ task_id（%s）：两名两体' % (sid, tid),
                '让两者一致，例：把 `"task_id"` 改成 `"%s"`' % sid)


def clause_conflict(res, sl):
    """§2 冲突优先级：有冲突却未标 CONFLICT ⇒ 判红。

    登记口径（模板未给机器口径，本检查器定稿并登记）：**潜在冲突** = criteria 里出现
    「全量」形态（`./...` / 全量 / all）**且** budget 给了带数值的上界 ⇒ 必须显式标 `conflicts`。
    """
    crit = sl.get('criteria')
    txt = _as_text(crit)
    has_full = bool(FULLSCAN_RE.search(txt))
    btxt = _as_text(sl.get('budget'))
    has_bound = bool(re.search(r'\d', btxt))
    conflicts = sl.get('conflicts')
    if has_full and has_bound:
        if not _nonempty(conflicts):
            res.hit('§2-conflict', 'S2-CONFLICT',
                    'criteria 含全量形态（`./...`/全量/all）而 budget 给了数值上界 ⇒ 潜在冲突未标 CONFLICT',
                    '标出冲突：`"conflicts": [{"a": "criteria", "b": "budget", '
                    '"where": "<哪一句与哪个上界冲突>"}]`（标记不是解决，裁量权交回上游）')
        else:
            items = conflicts if isinstance(conflicts, list) else [conflicts]
            for i, c in enumerate(items):
                if isinstance(c, dict):
                    miss = [k for k in ('a', 'b', 'where') if not _nonempty(c.get(k))]
                    if miss:
                        res.hit('§2-conflict', 'S2-CONFLICT-INCOMPLETE',
                                'conflicts[%d] 没写清是哪两个字段、冲突在哪一句（缺 %s）' % (i, ' / '.join(miss)),
                                '补全三键，例：`{"a": "criteria", "b": "budget", "where": "全量 go test 必须绿 vs wall_s_max 43.6"}`')
                elif not _nonempty(c):
                    res.hit('§2-conflict', 'S2-CONFLICT-INCOMPLETE',
                            'conflicts[%d] 为空' % i,
                            '写成 `{"a": "criteria", "b": "budget", "where": "<冲突句>"}`')
    if _nonempty(conflicts) and not has_full:
        res.note('S2-CONFLICT-MARKED', '本片已标 CONFLICT（本检查器不再追判）')


def clause_inherit(res, child, parent, union_cmds=None):
    """§3.3 子合同继承四条：子 pre ⊇ 父 pre · 子 post ⊆ 父 post · 子 hard_limits ⊆ 父 hard_limits ·
    父 exit_cond 由子集合共同满足 ⇒ 违反任一条判红。

    `union_cmds` = **所有**子片 criteria 命令的并集（第①条与第④条是集合级判据：父 exit_cond
    「由子集合共同满足」，故用并集；不传则退化为只看本子片）。
    """
    # ① 子 pre ⊇ 父 pre（context_needed 的 value 集合 + open_questions 条目）
    pctx = set()
    for x in (parent.get('context_needed') if isinstance(parent.get('context_needed'), list) else []):
        pctx.add(ctx_values(x))
    cctx = set()
    for x in (child.get('context_needed') if isinstance(child.get('context_needed'), list) else []):
        cctx.add(ctx_values(x))
    lose_ctx = sorted(pctx - cctx)
    pq = parent.get('open_questions')
    cq = child.get('open_questions')
    pq_list = pq if isinstance(pq, list) else ([pq] if _nonempty(pq) and pq != '无' else [])
    cq_list = cq if isinstance(cq, list) else ([cq] if _nonempty(cq) and cq != '无' else [])
    lose_q = [x for x in pq_list if _as_text(x) not in [_as_text(y) for y in cq_list]]
    if lose_ctx or lose_q:
        res.hit('§3.3-inherit', 'S33-INHERIT-PRE',
                '子 pre ⊉ 父 pre：丢了 %s' % (' / '.join(lose_ctx + [_as_text(x) for x in lose_q])[:80]),
                '把父片 pre 的条目照抄进子片 context_needed / open_questions（子 pre ⊇ 父 pre）')
    # ② 子 post ⊆ 父 post（criteria 的 cmd 集合）
    pcmds = set()
    if isinstance(parent.get('criteria'), list):
        for it in parent['criteria']:
            if isinstance(it, dict):
                pcmds.add(_as_text(it.get('cmd')).strip())
    if isinstance(child.get('criteria'), list):
        extra = []
        for it in child['criteria']:
            if isinstance(it, dict):
                c = _as_text(it.get('cmd')).strip()
                if c not in pcmds:
                    extra.append(c)
        if extra:
            res.hit('§3.3-inherit', 'S33-INHERIT-POST',
                    '子 post ⊄ 父 post：子多出 %d 条父片没有的判据（%s）' % (len(extra), extra[0][:50]),
                    '删掉子片多出的判据，或让父片先承接它（子 post ⊆ 父 post：防子片绕过父片边界）')
    # ③ 子 hard_limits ⊆ 父 hard_limits
    ph = parent.get('hard_limits')
    ch = child.get('hard_limits')
    ph_list = [ _as_text(x) for x in ph ] if isinstance(ph, list) else ([_as_text(ph)] if _nonempty(ph) else [])
    ch_list = [ _as_text(x) for x in ch ] if isinstance(ch, list) else ([_as_text(ch)] if _nonempty(ch) else [])
    extra_h = [x for x in ch_list if x not in ph_list]
    if extra_h:
        res.hit('§3.3-inherit', 'S33-INHERIT-LIMITS',
                '子 hard_limits ⊄ 父 hard_limits：多出 %s' % extra_h[0][:60],
                '删掉子片多出的 hard_limits，或先写进父片（子 hard_limits ⊆ 父 hard_limits）')
    # ④ 父 exit_cond 由子集合共同满足（父 success 引用的 criteria cmd 必须被子集合共同承接）
    pec = parent.get('exit_cond')
    if isinstance(pec, dict) and isinstance(pec.get('success'), list) and isinstance(parent.get('criteria'), list):
        pid_cmds = {}
        for it in parent['criteria']:
            if isinstance(it, dict) and _nonempty(it.get('id')):
                pid_cmds[str(it['id'])] = _as_text(it.get('cmd')).strip()
        ccmds = set()
        if isinstance(child.get('criteria'), list):
            for it in child['criteria']:
                if isinstance(it, dict):
                    ccmds.add(_as_text(it.get('cmd')).strip())
        scope = ccmds | set(union_cmds or [])
        miss = [str(t) for t in pec['success'] if pid_cmds.get(str(t)) and pid_cmds[str(t)] not in scope]
        if miss:
            res.hit('§3.3-inherit', 'S33-INHERIT-EXITCOND',
                    '父 exit_cond 的成功项 %s 没有被子集合承接' % ' / '.join(miss),
                    '把父片 success 里那几条判据放进某一子片的 criteria（父 exit_cond 由子集合共同满足）')


# ══════════════════════════════════════════════════════════════════
# 4. 一片 / 一批的判定
# ══════════════════════════════════════════════════════════════════

def check_slice(obj, name, opts, all_slices, calib):
    res = Result(name)
    sid = obj.get('slice_id')
    sid = str(sid) if _nonempty(sid) else None
    # 必填缺件（登记表，不静默）
    for f, ref in REQUIRED_FIELDS:
        if f not in obj:
            res.hit(ref.split('-')[-1] if False else '§1', 'S1-MISSING-FIELD',
                    '必填字段缺失：%s（依据 %s）' % (f, ref),
                    '把 %s 补进片文件（模板 §1 表：必填）；未决写 `NEEDS-CLARIFICATION`' % f)
    if not opts['disable'].get('R0'):
        r0(res, obj)
    if not opts['disable'].get('R1'):
        r1(res, obj)
    if not opts['disable'].get('R2'):
        r2(res, obj)
    if not opts['disable'].get('R3'):
        r3(res, obj)
    if not opts['disable'].get('R4'):
        for f in ('landing', 'the_one_thing', 'output_format', 'atomicity', 'escalate_if',
                  'exit_cond', 'side_effects', 'conflicts'):
            r4_scan(res, f, _as_text(obj.get(f)))
        r4_scan(res, 'budget', _as_text(obj.get('budget')))
        r4_scan(res, 'context_budget', _as_text(obj.get('context_budget')))
        if isinstance(obj.get('criteria'), str):
            r4_scan(res, 'criteria', obj['criteria'])
        for i, it in enumerate(obj.get('criteria') if isinstance(obj.get('criteria'), list) else []):
            if isinstance(it, dict):
                r4_scan(res, 'criteria[%d].cmd' % i, _as_text(it.get('cmd')))
                r4_scan(res, 'criteria[%d].expect' % i, _as_text(it.get('expect')))
                r4_scan(res, 'criteria[%d].state' % i, _as_text(it.get('state')))
        for i, it in enumerate(obj.get('gate_cmds') if isinstance(obj.get('gate_cmds'), list) else []):
            if isinstance(it, dict):
                r4_scan(res, 'gate_cmds[%d].cmd' % i, _as_text(it.get('cmd')))
        for i, it in enumerate(obj.get('context_needed') if isinstance(obj.get('context_needed'), list) else []):
            r4_scan(res, 'context_needed[%d]' % i, ctx_values(it))
        for i, it in enumerate(obj.get('open_questions') if isinstance(obj.get('open_questions'), list) else []):
            r4_scan(res, 'open_questions[%d]' % i, _as_text(it))
        for i, it in enumerate(obj.get('hard_limits') if isinstance(obj.get('hard_limits'), list) else []):
            r4_scan(res, 'hard_limits[%d]' % i, _as_text(it))
    if not opts['disable'].get('R5'):
        r5(res, obj)
    if not opts['disable'].get('R6'):
        r6(res, obj)
    if not opts['disable'].get('R7'):
        r7(res, obj, calib)
    if not opts['disable'].get('R8'):
        r8(res, obj, sid, all_slices)
    if not opts['disable'].get('R10'):
        r10(res, obj)
    if not opts['disable'].get('R11'):
        r11(res, obj, calib)
    if not opts['disable'].get('R12'):
        r12(res, obj)
    # 非 R 判据（§1 表 / §2 / §3.3）
    clause_open_questions(res, obj)
    clause_output_format(res, obj)
    clause_side_effects(res, obj)
    clause_escalate_if(res, obj)
    clause_budget(res, obj)
    clause_atomicity(res, obj, calib)
    clause_slice_id(res, obj)
    clause_conflict(res, obj)
    return res


def _resolve_parent(pref, by_id):
    if isinstance(pref, dict):
        return pref
    if isinstance(pref, str):
        if pref in by_id:
            return by_id[pref]['obj']
        cand = pref if os.path.isabs(pref) else os.path.join(REPO_ROOT, pref)
        if os.path.isfile(cand):
            obj, _err = load_slice_file(cand)
            return obj
    return None


def evaluate_set(all_s, opts, calib):
    """片集合的**唯一**判定入口（--check 与 --probe 共用，避免两条路走岔）。

    all_s: [{'path':…, 'obj':…}]；返回 results（与 all_s 同序、同长）。
    集合级规则：R8（环/重复）· R9（依赖闭包）· §3.3 继承四条（含第④条用**子集合并集**）。
    """
    by_id = {}
    for s in all_s:
        sid = s['obj'].get('slice_id')
        if _nonempty(sid):
            by_id[str(sid)] = s
    results = []
    for s in all_s:
        results.append(check_slice(s['obj'], s['path'], opts, all_s, calib))
    if not opts['disable'].get('R9'):
        r9(_single_join(results), all_s, opts['set_open'])
    # §3.3 继承：先算「每个父片的全部子命令并集」，再逐子片判四条
    children_cmds = {}
    for s in all_s:
        pref = s['obj'].get('parent')
        if not _nonempty(pref):
            continue
        pobj = _resolve_parent(pref, by_id)
        pid = _as_text(pobj.get('slice_id')) if isinstance(pobj, dict) else _as_text(pref)
        bucket = children_cmds.setdefault(pid, set())
        if isinstance(s['obj'].get('criteria'), list):
            for it in s['obj']['criteria']:
                if isinstance(it, dict):
                    bucket.add(_as_text(it.get('cmd')).strip())
    for i, s in enumerate(all_s):
        pref = s['obj'].get('parent')
        if not _nonempty(pref):
            continue
        pobj = _resolve_parent(pref, by_id)
        if pobj is None:
            results[i].hit('§3.3-inherit', 'S33-INHERIT-NO-PARENT',
                           '子片声明了 parent 但父片读不到：%s' % _as_text(pref)[:50],
                           '把父片路径写对（相对仓根或绝对路径），或把父片对象内联进 `parent`')
            continue
        pid = _as_text(pobj.get('slice_id'))
        clause_inherit(results[i], s['obj'], pobj, children_cmds.get(pid, set()))
    return results


def check_set(slices, opts, calib):
    """slices: [{'path':…, 'obj':…}] ⇒ 同 evaluate_set（保留此名给探针侧用）。"""
    return evaluate_set(slices, opts, calib)


def _single_join(results):
    """R9 是集合级规则；命中挂到首片的 Result 上（打印时标注「集合级」）。"""
    if not results:
        return Result('(空)')
    return results[0]


# ══════════════════════════════════════════════════════════════════
# 5. strict 等价（§3.5：未知规则键＝错误）
# ══════════════════════════════════════════════════════════════════

OFF_WORDS = ('off', 'disable', 'disabled', 'skip', 'skipped', 'false', 'no', '0')


def _is_off(v):
    if v is False or v == 0:
        return True
    if isinstance(v, str):
        return v.strip().lower() in OFF_WORDS
    if isinstance(v, dict):
        return _is_off(v.get('enabled', v.get('state', True)))
    return False


def strict_errors_for_slice(obj, path):
    """strict 等价：未知字段键 ⇒ 错误；`rules` 段未知规则键 / 未登记禁用 ⇒ 错误。

    返回错误串列表（空 = 过 strict）。**任一错误 ⇒ rc=2，不给结论**（AJV strict 的等价物：
    任何「规则被悄悄跳过」的入口都必须先过登记表）。
    """
    errs = []
    if not isinstance(obj, dict):
        return ['%s: [STRICT-NOT-OBJECT] 片文件顶层不是对象（实为 %s）' % (path, type(obj).__name__)]
    for k in obj.keys():
        if k not in KNOWN_SLICE_KEYS:
            errs.append('%s: [STRICT-UNKNOWN-FIELD] 未知字段键 `%s`（已知键 %d 个：%s；'
                        '新增字段必须先在模板 §1 表登记）'
                        % (path, k, len(KNOWN_SLICE_KEYS), ' · '.join(sorted(KNOWN_SLICE_KEYS))))
    rules = obj.get('rules')
    if rules is not None:
        if not isinstance(rules, dict):
            errs.append('%s: [STRICT-BAD-RULES] `rules` 段必须是对象（键 = 规则键，值 = 开关）' % path)
        else:
            for rk, rv in rules.items():
                if rk not in RULE_KEYS:
                    errs.append('%s: [STRICT-UNKNOWN-RULE] 未知规则键 `%s`（已知规则键：%s）'
                                % (path, rk, ' · '.join(RULE_KEYS)))
                elif _is_off(rv) and rk not in REGISTERED_DISABLE:
                    errs.append('%s: [STRICT-UNREGISTERED-DISABLE] 片内试图禁用 `%s` —— **未登记**'
                                '（§3.5：禁用任何规则必须显式登记并打印；可登记禁用的键：%s）'
                                % (path, rk, ' · '.join(sorted(REGISTERED_DISABLE)) or '(无)'))
    return errs


def validate_disables(keys):
    """校验命令行 --disable-rule：未知规则键 / 未登记 ⇒ 错误串列表。"""
    errs = []
    for k in keys:
        if k not in RULE_KEYS:
            errs.append('[STRICT-UNKNOWN-RULE] --disable-rule 给了未知规则键 `%s`（已知：%s）'
                        % (k, ' · '.join(RULE_KEYS)))
        elif k in DISABLED_RULES:
            errs.append('[STRICT-ALREADY-DISABLED] `%s` 已是永久禁用项（登记见 DISABLED_RULES / --list-rules），'
                        '重复禁用请求不登记' % k)
        elif k not in REGISTERED_DISABLE:
            errs.append('[STRICT-UNREGISTERED-DISABLE] `%s` 未被登记为可禁用（§3.5：禁用任何规则必须**先显式登记**；'
                        '已登记的键：%s）' % (k, ' · '.join(sorted(REGISTERED_DISABLE)) or '(无)'))
    return errs


# ══════════════════════════════════════════════════════════════════
# 6. 打印（登记块 / 片报告 / 规则清单）
# ══════════════════════════════════════════════════════════════════

def calib_note(calib):
    if not calib:
        return '标定档案：读不到（%s）⇒ 与档案绑定的上界一律记「未标定」，不猜' % CALIB_PATH
    return '标定档案：%s（现读键：%s）' % (CALIB_PATH, ' · '.join(sorted(calib.keys())))


def probe_thresholds(calib):
    out = {}
    for k, d in PROBE_DEFAULTS.items():
        v = calib.get(k)
        if v is None:
            out[k] = (d, '登记默认（档案无 `%s` 键）' % k)
        else:
            try:
                out[k] = (float(v) if k.endswith('_max') else int(float(v)), '标定档案')
            except (TypeError, ValueError):
                out[k] = (d, '档案值不可解析 ⇒ 回退登记默认')
    return out


def print_registration(opts, calib, quiet=False):
    if quiet:
        return
    print('════════════════════════════════════════════════════════════════')
    print('切片判据检查器 check-slice.py · 仓根 %s' % REPO_ROOT)
    print('共同规则表：docs/01-设计/切片合同-模板.md（§1 字段表 + §2 冲突优先级 + §3.3 继承四条 + §3.4 R0–R13）')
    n_en = len([k for k in RULE_KEYS if RULE_STATE[k] == '启用'])
    print('── 规则登记（启用 %d · 部分 %d · 路由 %d · 禁用 %d）──'
          % (n_en, len(PARTIAL_RULES), len(ROUTE_RULES), len(DISABLED_RULES)))
    for num, key, state, _text, _cost, _impl in RULE_TABLE:
        mark = {'启用': '  ', '部分': '～', '路由': '→', '禁用': '×'}[state]
        line = '%s [%-2s] %-4s %-36s' % (mark, num, state, key)
        if key in DISABLED_RULES:
            d = DISABLED_RULES[key]
            line += '  理由：%s｜依据：%s｜影响：%s' % (d['why'], d['evidence'], d['effect'])
        elif num in PARTIAL_RULES:
            line += '  未判：%s' % PARTIAL_RULES[num]['undone'][:70]
        elif num in ROUTE_RULES:
            line += '  %s' % ROUTE_RULES[num][:70]
        print(line)
        if num in opts['disable']:
            print('      ✱ 本次运行**已登记禁用**（--disable-rule %s）⇒ 本规则不参与判定' % num)
    print('  [禁用-登记表] 可被 `--disable-rule` 关掉的键（预登记）：%s'
          % (' · '.join(sorted(REGISTERED_DISABLE)) or '(无)'))
    for k in sorted(REGISTERED_DISABLE):
        print('       · %s —— %s' % (k, REGISTERED_DISABLE[k]))
    print('── 判据口径来源（登记，不静默）──')
    print('  R0 动词词表：%d 条（最长匹配、非重叠）' % len(REG_VERBS))
    n4 = {}
    for layer, _p, _f in REG_R4:
        n4[layer] = n4.get(layer, 0) + 1
    print('  R4 三层词表：含糊 %d · 可选 %d · 主观 %d（**每条都带改写建议**；给不出 fix 的命中 ⇒ 拒判 rc=2）'
          % (n4.get('含糊', 0), n4.get('可选', 0), n4.get('主观', 0)))
    print('  R5 占位词表：%s' % ' · '.join(REG_PLACEHOLDER))
    print('  R6 expect 取值域：%s（登记口径；exit-only + 登记假绿形态 ⇒ 判红）'
          % ' · '.join(REG_EXPECT_KEYS))
    print('  R6 登记的假绿形态：%s' % ' · '.join('%s（%s）' % (p, w) for p, w in REG_EXIT_TRAP))
    print('  R11 顺序口径：context_needed 首条 = 落点原文片段；criteria 须在 %s 之前'
          % ' · '.join(R11_AFTER_CRITERIA[:6]) + ' 等之前')
    th = probe_thresholds(calib)
    print('  探针门槛（§3.6/§3.5）：含糊-违规 ≥%s · 合法 ≥%s · 假红率 ≤%s'
          % (th['probe_vague_min'][0], th['probe_legal_min'][0], th['probe_fp_rate_max'][0]))
    print('      取值来源：含糊 %s · 合法 %s · 假红 %s'
          % (th['probe_vague_min'][1], th['probe_legal_min'][1], th['probe_fp_rate_max'][1]))
    print('  ' + calib_note(calib))
    print('  片文件已知键（strict：别的键 ⇒ rc=2）：%d 个 —— 必填 %d + 表外 1 + **可选 %d**（%s）+ 元键 %s'
          % (len(KNOWN_SLICE_KEYS), len(REQUIRED_FIELDS), len(OPTIONAL_FIELDS),
             ' · '.join(f for f, _ref, _desc in OPTIONAL_FIELDS), ' · '.join(META_KEYS)))
    print('  可选字段登记（Q15/C5）：不给 ⇒ 与登记前逐字一致、**不得变红**；只放宽 strict 白名单，'
          '不进 REQUIRED_FIELDS')
    print('  非 R 判据（模板 §1 表 / §2 / §3.3 的另一半，**不是新规则**）：%d 条' % len(CLAUSE_TABLE))
    for ref, code, _text in CLAUSE_TABLE:
        print('       %-24s → %s' % (ref, code))
    print('  明确**不判**登记表（判域之外，逐条写明；不静默）：%d 条' % len(NOT_CHECKED))
    for ref, text in NOT_CHECKED:
        print('       %-18s %s' % (ref, text))
    print('════════════════════════════════════════════════════════════════')


def print_list_rules(opts, calib):
    """--list-rules：规则表 + 启用状态 + 全部登记表 + 片文件已知键。"""
    print_registration(opts, calib, quiet=False)
    print('── R0–R13 逐条（§3.4 逐字摘要 · 成本 · 本检查器实施口径）──')
    for num, key, state, text, cost, impl in RULE_TABLE:
        print('[%s] %s ｜ 状态：%s ｜ 成本：%s' % (num, key, state, cost))
        print('    规则：%s' % text)
        print('    实施：%s' % impl)
    print('── 禁用登记表（§3.5：禁用任何规则必须显式登记并打印）──')
    for k in sorted(DISABLED_RULES):
        d = DISABLED_RULES[k]
        print('[%s] 禁用 —— 理由：%s' % (k, d['why']))
        print('      依据：%s' % d['evidence'])
        print('      下沉：%s' % d['downstream'])
        print('      影响：%s' % d['effect'])
    print('── 部分实现登记表（整条已登记，未判的那一半写出来）──')
    for k in sorted(PARTIAL_RULES):
        d = PARTIAL_RULES[k]
        print('[%s] 已判：%s' % (k, d['done']))
        print('      未判：%s' % d['undone'])
        print('      依据：%s' % d['evidence'])
    print('── 路由登记表（超限不打回）──')
    for k in sorted(ROUTE_RULES):
        print('[%s] %s' % (k, ROUTE_RULES[k]))
    print('── 非 R 判据（模板 §1 字段表 / §2 冲突优先级 / §3.3 继承四条）──')
    for ref, code, text in CLAUSE_TABLE:
        print('[%-22s] %-20s %s' % (ref, code, text))
    print('── 片文件已知键（strict：别的键 ⇒ rc=2）──')
    for f, ref in REQUIRED_FIELDS:
        print('  必填 %-16s ← %s' % (f, ref))
    print('  表外 %-16s ← §1-inv-escalate_if（有 side_effects 即须配）' % 'escalate_if')
    for f, ref, desc in OPTIONAL_FIELDS:
        print('  可选 %-16s ← %s' % (f, ref))
        print('        ↑ **可选**（%s；应到 §9-Q15 / 附录A C5 登记；edit-assert 消费，'
              '本检查器不判其内部结构）' % desc)
    for m in META_KEYS:
        print('  元键 %-16s' % m)
    print('── R4 三层词表（每条都带改写建议；无 fix 的命中 ⇒ 拒判 rc=2）──')
    for layer, pat, fix in REG_R4:
        print('  [%s] %-14s → %s' % (layer, pat, fix))
    print('── 命令一览 ──')
    print('  --check <片.json> [<片.json> …]  判合法性（单片或片集合；退出码 0/1/2）')
    print('  --probe                          跑探针集并打混淆矩阵（假绿>0 ⇒ rc=1）')
    print('  --list-rules                     本命令')
    print('  --selftest                       四条自证（正例·反例·strict·真隐患）')
    print('  --set-open / --disable-rule <编号> / -q')
    return 0


# ══════════════════════════════════════════════════════════════════
# 7. 片报告打印
# ══════════════════════════════════════════════════════════════════

def print_result(res, idx, total, opts):
    order = [r[0] for r in RULE_TABLE]
    hits_by = {}
    for h in res.hits:
        hits_by.setdefault(h['rule'], []).append(h)
    print('── 片 %d/%d · %s ──' % (idx, total, res.name))
    line = []
    for num in order:
        if RULE_STATE[num] == '禁用':
            continue
        if num in opts['disable']:
            line.append('%s 已登记禁用' % num)
        else:
            line.append('%s%s' % (num, '✗' if num in hits_by else '✓'))
    print('  逐规则：' + ' · '.join(line))
    if hits_by:
        print('  命中 %d 条（红）：' % len(res.hits))
        for h in res.hits:
            print('    ✗ [%s] %s — %s' % (h['rule'], h['code'], h['msg']))
            print('        改写建议：%s' % h['fix'])
    else:
        print('  命中 0 条 ⇒ 本片合法（无红）')
    for r in res.recursion:
        print('  → 需递归（R7：超限判「需递归」不打回）：[%s] %s' % (r['code'], r['msg']))
    for n in res.notes:
        print('  · 注记 [%s] %s' % (n['code'], n['msg']))
    for num in sorted(DISABLED_RULES):
        print('  × [%s] 禁用（登记）：%s' % (num, DISABLED_RULES[num]['why']))
    if res.hits:
        print('  小计：红 —— 命中 %d 条规则：%s' % (len(res.red_rules()), ' · '.join(res.red_rules())))
    elif res.recursion:
        print('  小计：需递归（不打回）')
    else:
        print('  小计：绿')


# ══════════════════════════════════════════════════════════════════
# 8. 探针集与混淆矩阵（§3.5 质量门槛 / §3.6 探针集）
# ══════════════════════════════════════════════════════════════════

PROBE_KNOWN_KEYS = ['name', 'expect', 'class', 'why', 'expect_rules', 'slice', 'slices', 'note']
PROBE_EXPECTS = ['red', 'green', 'recursion', 'error']
PROBE_EXPECT_CN = {'red': '红/违规', 'green': '绿/合法', 'recursion': '需递归', 'error': '错误(rc=2)'}
PROBE_GOT_CN = {'red': '判红', 'green': '判绿', 'recursion': '判需递归', 'error': '判错误'}

# ── 前置缺件闸：探针引用的**仓外公共根**（2026-09-19 开发文档分家后新增）─────────────────
#   背景（实测）：探针集引用的源件（`切片合同-模板.md` 等）随开发文档分家搬到**工作树之外**
#   的同级目录 `../Zerg-内部文档/`。公开树（`publish/whitelist.txt` 只收 `scripts/`）里没有它 ⇒
#   每条探针的 landing / context_needed 都落 R2-NOT-FOUND ⇒ 全被算成「假红」（实测假红率
#   63.6%）——「本仓缺件」被错报成「探针写错了」。口径与仓内同族（缺件不许静默降级）：
#   探针引用的**仓外公共根**在本仓不可达 ⇒ rc=2 **不给结论**（BLOCKED），逐条打印缺件路径。
#   ★ 只闸**公共根目录**：根可达而具体源件真丢 ⇒ 各条探针照旧判红（R2-NOT-FOUND），判据不放宽。
#   ★ 私有树（`../Zerg-内部文档/` 在位）行为一字未改：本闸放行，其余判定逐条与改前一致。
EXTERNAL_REF_RE = re.compile(r'(?:\.\./)+[^\s\'"，,；;、）)】」》|<>`]+')


def _clean_ref(tok):
    """去引号后缀标点与 `:行号` / `:行号-行号`（`../Zerg-内部文档/…/模板.md:12-14` ⇒ 路径）。"""
    tok = tok.strip().rstrip('。，,;；、）)】」》`')
    return re.sub(r':\d+(?:-\d+)?$', '', tok)


def probe_external_refs(p):
    """一条探针里所有 `../` 出仓的路径引用（landing / context_needed / criteria / gate_cmds）。"""
    objs = []
    if isinstance(p.get('slice'), dict):
        objs.append(p['slice'])
    if isinstance(p.get('slices'), list):
        objs += [o for o in p['slices'] if isinstance(o, dict)]
    out = []
    for o in objs:
        bag = [o.get('landing')]
        cn = o.get('context_needed')
        if isinstance(cn, list):
            for it in cn:
                bag.append(it.get('value') if isinstance(it, dict) else it)
        for key in ('criteria', 'gate_cmds'):
            v = o.get(key)
            if isinstance(v, list):
                for it in v:
                    if isinstance(it, dict):
                        bag.append(it.get('cmd'))
        for s in bag:
            if not isinstance(s, str):
                continue
            for m in EXTERNAL_REF_RE.findall(s):
                c = _clean_ref(m)
                if c and c not in out:
                    out.append(c)
    return out


def probe_external_roots(probes, repo_root=None):
    """返回 (公共根, info, missing)。

    info = {仓外根（相对仓根）: {'probes': [探针名…], 'refs': [例引用…]}}；
    公共根 = 所有出仓引用的**目录前缀的公共前缀**（分家后现读只有一棵：`../Zerg-内部文档/01-设计`）；
    missing = 在本仓（repo_root，默认 REPO_ROOT）**不可达**的那些根（相对写法原样返回）。
    """
    base = REPO_ROOT if repo_root is None else os.path.abspath(repo_root)
    info = {}
    for p in probes:
        nm = p.get('name', '?')
        for ref in probe_external_refs(p):
            d = os.path.normpath(os.path.dirname(ref))
            if not d or d == '.':
                continue
            slot = info.setdefault(d, {'probes': [], 'refs': []})
            if nm not in slot['probes']:
                slot['probes'].append(nm)
            if ref not in slot['refs']:
                slot['refs'].append(ref)
    roots = sorted(info)
    common = ''
    if roots:
        try:
            common = os.path.commonpath(roots)
        except ValueError:
            common = ''
    missing = [r for r in roots if not os.path.exists(os.path.join(base, r))]
    return common, info, missing


def precheck_probe_sources(probes, repo_root=None):
    """前置缺件闸的判据出口：返回 (公共根, info, missing)。missing 非空 ⇒ rc=2 不给结论。"""
    return probe_external_roots(probes, repo_root)


def print_probe_source_gate(common, info, missing):
    """缺件打印：公共根 + 逐条缺件路径 + 引用它的探针名 + 公开树/私有树口径。"""
    print('✗ 前置缺件：探针集引用的**仓外公共根**在本仓不可达 ⇒ rc=2，不给结论（BLOCKED）')
    print('  探针公共根：%s（仓根下解析 = %s）'
          % (common or '（无法归一）',
             os.path.join(REPO_ROOT, common) if common else '（无）'))
    for r in missing:
        slot = info[r]
        print('  缺件路径：%s（仓根下解析 = %s）' % (r, os.path.join(REPO_ROOT, r)))
        print('      引用它的探针 %d 条（例：%s）｜例引用：%s'
              % (len(slot['probes']), ' · '.join(slot['probes'][:3]),
                 slot['refs'][0] if slot['refs'] else '—'))
    print('  口径：公开树侧（无 `../Zerg-内部文档` 与 `docs/`）本 scope 为 BLOCKED、不适用；'
          '私有树把 `Zerg-内部文档/` 放回同级目录再跑。')
    print('  只闸**公共根目录**：根可达而具体源件真丢 ⇒ 各条探针照旧判红（R2-NOT-FOUND），判据不放宽。')


def load_probes():
    if not os.path.isfile(PROBE_PATH):
        return None, ['探针集文件不存在：%s（前置缺件 ⇒ 不给结论）' % PROBE_PATH]
    try:
        with open(PROBE_PATH, 'r', encoding='utf-8') as fh:
            data = json.load(fh)
    except (IOError, OSError, ValueError) as exc:
        return None, ['探针集解析失败：%s' % exc]
    probes = data.get('probes') if isinstance(data, dict) else None
    errs = []
    if not isinstance(probes, list) or not probes:
        return None, ['探针集里没有 `probes` 列表（前置缺件）']
    for i, p in enumerate(probes):
        if not isinstance(p, dict):
            errs.append('probes[%d] 不是对象' % i)
            continue
        for k in p.keys():
            if k not in PROBE_KNOWN_KEYS:
                errs.append('[STRICT-UNKNOWN-FIELD] probes[%d] 未知探针键 `%s`（已知：%s）'
                            % (i, k, ' · '.join(PROBE_KNOWN_KEYS)))
        if p.get('expect') not in PROBE_EXPECTS:
            errs.append('probes[%d]（%s）的 expect 不在 %s 内（分类标签必填）'
                        % (i, p.get('name'), ' / '.join(PROBE_EXPECTS)))
        if not isinstance(p.get('slice'), dict) and not isinstance(p.get('slices'), list):
            errs.append('probes[%d]（%s）既没有 `slice` 也没有 `slices`' % (i, p.get('name')))
    return probes, errs


def probe_verdict(p, opts, calib):
    """返回 (verdict, strict_errs, results)。"""
    objs = []
    if isinstance(p.get('slice'), dict):
        objs = [p['slice']]
    elif isinstance(p.get('slices'), list):
        objs = [o for o in p['slices'] if isinstance(o, dict)]
    strict = []
    for i, o in enumerate(objs):
        strict += strict_errors_for_slice(o, '%s#%d' % (p.get('name'), i))
    if strict:
        return 'error', strict, []
    slices = [{'path': '%s#%d' % (p.get('name'), i), 'obj': o} for i, o in enumerate(objs)]
    results = check_set(slices, opts, calib)
    if any(r.is_red() for r in results):
        return 'red', [], results
    if any(r.recursion for r in results):
        return 'recursion', [], results
    return 'green', [], results


def run_probe(opts, calib):
    probes, errs = load_probes()
    if errs:
        for e in errs:
            print('✗ %s' % e)
        return 2
    # ── 前置缺件闸（2026-09-19 分家后新增）：探针引用的**仓外公共根**不可达 ⇒ rc=2 不给结论。
    #   为什么在跑探针之前：缺件时继续跑只会把每条探针判成「假红」（实测 63.6%），
    #   把「本仓缺件」错报成「探针写错了」——那是假红，不是判据。私有树本闸放行、行为不变。
    common, _info, missing = precheck_probe_sources(probes)
    if missing:
        print_probe_source_gate(common, _info, missing)
        return 2
    print_registration(opts, calib, quiet=opts['quiet'])
    th = probe_thresholds(calib)
    n_red = len([p for p in probes if p['expect'] == 'red'])
    n_green = len([p for p in probes if p['expect'] == 'green'])
    n_rec = len([p for p in probes if p['expect'] == 'recursion'])
    n_err = len([p for p in probes if p['expect'] == 'error'])
    n_bnd = len([p for p in probes if p.get('class') == '边界'])
    print('── 探针集 %s · 共 %d 条（期望红 %d · 期望绿 %d · 期望需递归 %d · 期望错误 %d · 其中边界 %d）──'
          % (PROBE_PATH, len(probes), n_red, n_green, n_rec, n_err, n_bnd))
    gate_bad = []
    if n_red < th['probe_vague_min'][0]:
        gate_bad.append('含糊/违规 %d < 门槛 %d' % (n_red, th['probe_vague_min'][0]))
    if n_green < th['probe_legal_min'][0]:
        gate_bad.append('合法 %d < 门槛 %d' % (n_green, th['probe_legal_min'][0]))
    if n_bnd < 2:
        gate_bad.append('边界 %d < 2' % n_bnd)
    if n_red < 5:
        gate_bad.append('期望判红 %d < 5（探针集不得写成「永远绿」）' % n_red)
    if gate_bad:
        print('✗ 探针集门槛不过（§3.6）⇒ 前置缺件，不给结论：%s' % ' ； '.join(gate_bad))
        return 2

    cnt = {}
    mism = []
    rule_bad = []
    for p in probes:
        got, strict, results = probe_verdict(p, opts, calib)
        exp = p['expect']
        cnt[(exp, got)] = cnt.get((exp, got), 0) + 1
        ok = (exp == got)
        labels = set()
        for r in results:
            labels |= set(r.red_rules())
            for rec in r.recursion:
                labels.add(rec['code'].split('-')[0])
        codes = []
        for r in results:
            for h in r.hits:
                codes.append(h['code'])
        if strict:
            labels.add('STRICT')
            codes = ['STRICT'] + [s.split(']')[0].split('[')[-1] for s in strict]
        want = list(p.get('expect_rules') or [])
        missing_rule = [w for w in want if w not in labels]
        if missing_rule:
            ok = False
            rule_bad.append((p.get('name'), missing_rule, want))
        print('  %s [%s] %-34s 期望=%s 实判=%s%s%s'
              % ('✓' if ok else '✗', p.get('class', '-'), p.get('name'),
                 PROBE_EXPECT_CN.get(exp), PROBE_GOT_CN.get(got),
                 ('  命中：%s' % ' · '.join(codes[:6])) if codes else '',
                 ('  缺标注规则：%s' % '·'.join(missing_rule)) if missing_rule else ''))
        if not ok:
            mism.append((p, exp, got, codes, strict, results))
    if rule_bad:
        print('  ── 标注规则未命中（探针标签与实判不一致 ⇒ 假绿的口子）──')
        for nm, miss, want in rule_bad:
            print('  ✗ %s：标注 `expect_rules`=%s，实判缺 %s' % (nm, '·'.join(want), '·'.join(miss)))
    if mism:
        print('  ── 不符探针明细 ──')
        for p, exp, got, codes, strict, results in mism:
            print('  ✗ %s：期望 %s，实判 %s —— %s' % (p.get('name'), PROBE_EXPECT_CN[exp], PROBE_GOT_CN[got],
                                                  p.get('why', '')))
            for s in strict:
                print('      | %s' % s)
            for r in results:
                for h in r.hits:
                    print('      | [%s] %s — %s' % (h['rule'], h['code'], h['msg'][:90]))
    tp, fp = cnt.get(('red', 'red'), 0), cnt.get(('green', 'red'), 0)
    tn = cnt.get(('green', 'green'), 0)
    fn = cnt.get(('red', 'green'), 0) + cnt.get(('red', 'recursion'), 0) + cnt.get(('red', 'error'), 0)
    rec_ok, err_ok = cnt.get(('recursion', 'recursion'), 0), cnt.get(('error', 'error'), 0)
    rec_bad = sum(v for (e, g), v in cnt.items() if e == 'recursion' and g != 'recursion')
    err_bad = sum(v for (e, g), v in cnt.items() if e == 'error' and g != 'error')
    green_mis = sum(v for (e, g), v in cnt.items() if e == 'green' and g != 'green')
    print('════════════════════════════════════════════════════════════════')
    print('── 混淆矩阵（标注集 = 探针文件里的 `expect` 标签）──')
    cols = ['red', 'green', 'recursion', 'error']
    print('  期望↓ / 实判→        %s' % '  '.join('%-12s' % PROBE_EXPECT_CN[c] for c in cols))
    for e in cols:
        row = '  %-18s' % PROBE_EXPECT_CN[e]
        for g in cols:
            row += '  %-12s' % str(cnt.get((e, g), 0))
        print(row)
    print('  真阳 TP（期望红·判红）= %d ｜ 假阴 FN/假绿（期望红·未判红）= %d' % (tp, fn))
    print('  真阴 TN（期望绿·判绿）= %d ｜ 假阳 FP/假红（期望绿·判红）= %d' % (tn, fp))
    print('  需递归类命中 = %d/%d（未命中 %d）｜ 错误类命中 = %d/%d（未命中 %d）'
          % (rec_ok, rec_ok + rec_bad, rec_bad, err_ok, err_ok + err_bad, err_bad))
    print('  合计：探针 %d 条（期望红 %d · 期望绿 %d · 期望需递归 %d · 期望错误 %d）'
          % (len(probes), n_red, n_green, n_rec, n_err))
    fp_rate = (float(fp) / n_green) if n_green else 0.0
    print('── 门槛判定（§3.5：假绿率必须 0、假红率 ≤ 10%）──')
    rc = 0
    if fn == 0:
        print('  假绿率 = 0 ✓（%d 条期望红全部被拦住）' % n_red)
    else:
        print('  ✗ 假绿率 > 0（%d 条真隐患被放过）⇒ rc=1' % fn)
        rc = 1
    if fp_rate <= th['probe_fp_rate_max'][0]:
        print('  假红率 = %.1f%% ✓（≤ %.1f%%，%d/%d）'
              % (fp_rate * 100, th['probe_fp_rate_max'][0] * 100, fp, n_green))
    else:
        print('  ✗ 假红率 = %.1f%% > %.1f%% ⇒ rc=1'
              % (fp_rate * 100, th['probe_fp_rate_max'][0] * 100))
        rc = 1
    if green_mis:
        print('  ✗ 期望绿但未判绿（需递归/错误）= %d ⇒ rc=1' % green_mis)
        rc = 1
    if rec_bad or err_bad:
        print('  ✗ 需递归类未命中 %d · 错误类未命中 %d ⇒ rc=1' % (rec_bad, err_bad))
        rc = 1
    if rule_bad:
        print('  ✗ 探针标注规则（expect_rules）未命中 %d 条 ⇒ rc=1（标签与实判不一致 = 假绿的口子）'
              % len(rule_bad))
        rc = 1
    print('  探针条数门槛：含糊/违规 %d ≥ %d ✓ · 合法 %d ≥ %d ✓ · 边界 %d ≥ 2 ✓'
          % (n_red, th['probe_vague_min'][0], n_green, th['probe_legal_min'][0], n_bnd))
    print('结论：探针集 %s；rc = %d' % ('过门槛（假绿 0 / 假红在限内）' if rc == 0 else '不过门槛', rc))
    return rc


# ══════════════════════════════════════════════════════════════════
# 9. 自证（四条硬要求，全部走真实命令行 + 真实退出码）
# ══════════════════════════════════════════════════════════════════

def _run_check_cmd(path, extra=None):
    cmd = [sys.executable, os.path.abspath(__file__), '--check', path, '-q']
    if extra:
        cmd = [sys.executable, os.path.abspath(__file__)] + list(extra)
    try:
        import subprocess
        pr = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        out = pr.communicate()[0].decode('utf-8', 'replace')
        return pr.returncode, out
    except (IOError, OSError) as exc:
        return 99, '子进程起不来：%s' % exc


def run_selftest(opts, calib):
    import shutil
    import tempfile
    print_registration(opts, calib, quiet=opts['quiet'])
    probes, errs = load_probes()
    if errs:
        for e in errs:
            print('✗ %s' % e)
        return 2
    byname = dict((p['name'], p) for p in probes)
    need = ['template-正例', 'template-反例', '真隐患-缺期望输出']
    for nm in need:
        if nm not in byname:
            print('✗ 自证夹具缺件：探针集里没有 `%s`（前置缺件 ⇒ 不给结论）' % nm)
            return 2
    tmp = tempfile.mkdtemp(prefix='check-slice-selftest-')
    bad = 0
    checks = []
    try:
        # (a) 模板正例 ⇒ rc=0
        p = byname['template-正例']
        fa = os.path.join(tmp, 'a-正例.json')
        with open(fa, 'w', encoding='utf-8') as fh:
            json.dump(p['slice'], fh, ensure_ascii=False, indent=2)
        rc, out = _run_check_cmd(fa)
        checks.append(('(a) 模板正例 ⇒ 期望 rc=0', rc == 0, rc, out))
        # (b) 模板反例 ⇒ rc=1 且原因码与模板标注的 R 一致
        p = byname['template-反例']
        fb = os.path.join(tmp, 'b-反例.json')
        with open(fb, 'w', encoding='utf-8') as fh:
            json.dump(p['slice'], fh, ensure_ascii=False, indent=2)
        rc, out = _run_check_cmd(fb)
        inproc = check_set([{'path': 'b-反例.json', 'obj': p['slice']}], opts, calib)
        got_rules = []
        for r in inproc:
            got_rules += r.red_rules()
        want = list(p.get('expect_rules') or [])
        missing_r = [x for x in want if x not in got_rules]
        extra_r = [x for x in got_rules if x not in want and x.startswith('R')]
        ok_b = (rc == 1) and not missing_r
        checks.append(('(b) 模板反例 ⇒ 期望 rc=1 且命中模板标注的违规 R（%s）' % '·'.join(want),
                       ok_b, rc, out + '\n[进程内] 实命中 R：%s；缺：%s；多（未标注）：%s'
                       % ('·'.join(got_rules), '·'.join(missing_r) or '无', '·'.join(extra_r) or '无')))
        # (c) strict：未知字段键 / 未知规则键 / 未登记禁用
        bad_slice = dict((k, v) for k, v in p['slice'].items())
        bad_slice['r14_new_rule'] = '自造的规则键（不存在于 R0–R13）'
        fc = os.path.join(tmp, 'c-未知规则键.json')
        with open(fc, 'w', encoding='utf-8') as fh:
            json.dump(bad_slice, fh, ensure_ascii=False, indent=2)
        rc, out = _run_check_cmd(fc)
        checks.append(('(c1) 片里出现未知字段/规则键 ⇒ 期望 rc=2（strict：未知键＝错误）', rc == 2, rc, out))
        rc, out = _run_check_cmd(None, ['--check', fa, '--disable-rule', 'R99'])
        checks.append(('(c2) --disable-rule R99（未知规则键）⇒ 期望 rc=2', rc == 2, rc, out))
        rc, out = _run_check_cmd(None, ['--check', fa, '--disable-rule', 'R7'])
        checks.append(('(c3) --disable-rule R7（已知规则但**未登记**禁用）⇒ 期望 rc=2', rc == 2, rc, out))
        # (d) 真隐患探针：缺 criteria 期望输出 ⇒ 必红
        p = byname['真隐患-缺期望输出']
        fd = os.path.join(tmp, 'd-真隐患.json')
        with open(fd, 'w', encoding='utf-8') as fh:
            json.dump(p['slice'], fh, ensure_ascii=False, indent=2)
        rc, out = _run_check_cmd(fd)
        checks.append(('(d) 真隐患（criteria 缺期望输出）⇒ 期望 rc=1', rc == 1, rc, out))
        # (e) 前置缺件闸·**正**：本仓（私有树）探针公共根可达 ⇒ 闸放行、--probe 照判（rc 与改前一致）
        common, info, missing = precheck_probe_sources(probes, REPO_ROOT)
        rc, out = _run_check_cmd(None, ['--probe'])
        ok_e = (rc == 0) and (not missing)
        checks.append(('(e) 前置缺件闸·正：探针公共根可达（%s）⇒ 闸放行、--probe 照判 rc=0'
                       % (common or '—'), ok_e, rc,
                       out + '\n[进程内] 出仓引用根：%s；缺件：%s'
                       % (' · '.join(sorted(info)) or '无', ' · '.join(missing) or '无')))
        # (f) 前置缺件闸·**反**：/tmp 仿真公开树（仓根无 docs/、同级无 Zerg-内部文档/）⇒ 期望 rc=2 且打印缺件路径。
        #     夹具放在 tmp 下自成父目录 ⇒ `../Zerg-内部文档` 一定不存在（不赌 /tmp 里有没有同名目录）。
        import subprocess
        sim_repo = os.path.join(tmp, 'sim-public', 'repo')
        os.makedirs(os.path.join(sim_repo, 'scripts'))
        shutil.copy2(os.path.abspath(__file__), os.path.join(sim_repo, 'scripts', 'check-slice.py'))
        shutil.copy2(PROBE_PATH, os.path.join(sim_repo, 'scripts', 'slice-probes.json'))
        pr = subprocess.Popen([sys.executable,
                               os.path.join(sim_repo, 'scripts', 'check-slice.py'), '--probe'],
                              cwd=sim_repo, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        out = pr.communicate()[0].decode('utf-8', 'replace')
        rc = pr.returncode
        ok_f = (rc == 2) and ('缺件路径' in out) and ('../Zerg-内部文档' in out)
        checks.append(('(f) 前置缺件闸·反：仿真公开树（无 ../Zerg-内部文档）⇒ 期望 rc=2 且打印缺件路径',
                       ok_f, rc, out))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    # R4 约束的守卫（无改写建议的命中 ⇒ 拒判 rc=2）——进程内验证
    guard_ok = False
    try:
        Result('guard').hit('R4', 'R4-VAGUE', '自造的无 fix 命中', '')
    except InternalError:
        guard_ok = True
    print('── 自证 ──')
    for desc, ok, rc, out in checks:
        if not ok:
            bad += 1
        print('%s %s（实得 rc=%d）' % ('✓' if ok else '✗', desc, rc))
        for ln in out.splitlines():
            s = ln.strip()
            if not s:
                continue
            if (('✗ [R' in s) or ('STRICT' in s) or s.startswith('结论')
                    or s.startswith('[进程内]') or s.startswith('✗ 前置缺件')
                    or s.startswith('缺件路径') or s.startswith('探针公共根')):
                print('      | %s' % s)
    print('%s R4 约束守卫：无改写建议的命中必须拒判（InternalError）' % ('✓' if guard_ok else '✗'))
    if not guard_ok:
        bad += 1
    print('自证结论：%s（用例 %d 条，失败 %d 条）'
          % ('全过' if bad == 0 else '不过', len(checks) + 1, bad))
    return 0 if bad == 0 else 2


# ══════════════════════════════════════════════════════════════════
# 10. 命令行
# ══════════════════════════════════════════════════════════════════

USAGE = """用法:
  python3 scripts/check-slice.py --check <片.json> [<片.json> ...]   判合法性（单片或片集合）
  python3 scripts/check-slice.py --probe                            跑探针集 + 混淆矩阵
  python3 scripts/check-slice.py --list-rules                       列规则 / 启用状态 / 登记表
  python3 scripts/check-slice.py --selftest                         四条自证（正例·反例·strict·真隐患）
选项: --set-open（该批不是完整集合）· --disable-rule <编号> · -q/--quiet · -h/--help
退出码: 0 全部合法 / 1 有非法（打印原因码 + 命中哪条 R）/ 2 用法错·前置缺件·strict 违约·自检不过（不给结论）"""


def parse_args(argv):
    opts = {'paths': [], 'mode': None, 'set_open': False, 'disable': {}, 'disable_list': [], 'quiet': False}
    i = 0
    while i < len(argv):
        v = argv[i]
        if v == '--check':
            opts['mode'] = 'check'
            i += 1
            while i < len(argv) and not argv[i].startswith('-'):
                opts['paths'].append(argv[i])
                i += 1
        elif v == '--probe':
            opts['mode'] = 'probe'
            i += 1
        elif v == '--list-rules':
            opts['mode'] = 'list'
            i += 1
        elif v == '--selftest':
            opts['mode'] = 'selftest'
            i += 1
        elif v == '--set-open':
            opts['set_open'] = True
            i += 1
        elif v == '--disable-rule' and i + 1 < len(argv):
            opts['disable_list'].append(argv[i + 1])
            i += 2
        elif v in ('-q', '--quiet'):
            opts['quiet'] = True
            i += 1
        elif v in ('-h', '--help'):
            print(USAGE)
            return None
        else:
            sys.stderr.write('✗ 未知参数: %s\n' % v)
            print(USAGE)
            return None
    return opts


def load_slice_file(path):
    """返回 (obj, 错误串|None)。"""
    if not os.path.exists(path):
        return None, '片文件不存在：%s' % path
    if not os.path.isfile(path):
        return None, '不是普通文件：%s' % path
    try:
        with open(path, 'r', encoding='utf-8') as fh:
            return json.load(fh), None
    except (IOError, OSError) as exc:
        return None, '片文件读不出：%s（%s）' % (path, exc)
    except ValueError as exc:
        return None, '片文件不是合法 JSON：%s（%s）' % (path, exc)


def main(argv):
    if not argv:
        print(USAGE)
        return 2
    opts = parse_args(argv)
    if opts is None:
        return 2
    derr = validate_disables(opts['disable_list'])
    if derr:
        for e in derr:
            print('✗ %s' % e)
        print('✗ strict 违约 ⇒ rc=2，不给结论（§3.5：未知规则键＝错误；禁用必须显式登记并打印）')
        return 2
    for k in opts['disable_list']:
        opts['disable'][k] = True
    calib, calib_err = load_calib()
    if opts['mode'] is None:
        print(USAGE)
        return 2
    if opts['mode'] == 'list':
        return print_list_rules(opts, calib)
    if opts['mode'] == 'probe':
        return run_probe(opts, calib)
    if opts['mode'] == 'selftest':
        return run_selftest(opts, calib)
    # ── check ──
    if not opts['paths']:
        sys.stderr.write('✗ --check 后面没有给片文件\n')
        print(USAGE)
        return 2
    # 阶段①：装载 + strict（任一错误 ⇒ 不给结论）
    slices = []
    strict = []
    for p in opts['paths']:
        obj, err = load_slice_file(p)
        if err:
            strict.append(err)
            continue
        strict += strict_errors_for_slice(obj, p)
        slices.append({'path': p, 'obj': obj})
    if strict:
        print_registration(opts, calib, quiet=opts['quiet'])
        for e in strict:
            print('✗ %s' % e)
        print('✗ 前置缺件 / strict 违约 ⇒ rc=2，**不给结论**（strict 等价：未知规则键/字段键＝错误）')
        return 2
    # 阶段②：判据
    print_registration(opts, calib, quiet=opts['quiet'])
    if opts['set_open']:
        print('注：--set-open 已声明 ⇒ 依赖闭包记「不可判」（该批不是完整集合）')
    if calib_err:
        print('注：标定档案读不到（%s）⇒ 与档案绑定的上界记「未标定」，不猜' % calib_err)
    all_s = [{'path': s['path'], 'obj': s['obj']} for s in slices]
    results = evaluate_set(all_s, opts, calib)
    nred = 0
    nrec = 0
    for i, r in enumerate(results):
        print_result(r, i + 1, len(results), opts)
        if r.is_red():
            nred += 1
        if r.recursion:
            nrec += 1
    print('════════════════════════════════════════════════════════════════')
    print('结论：共 %d 片 · 合法 %d · 非法 %d · 需递归标记 %d（R7 超限不打回 ⇒ 不计入非法）'
          % (len(results), len(results) - nred, nred, nrec))
    if nred:
        codes = []
        for r in results:
            for h in r.hits:
                codes.append(h['code'])
        print('原因码汇总：%s' % ' · '.join(codes))
        print('rc = 1（有非法片）')
        return 1
    print('rc = 0（全部合法）')
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main(sys.argv[1:]))
    except InternalError as exc:
        sys.stderr.write('✗ 内部错误（不给结论）：%s\n' % exc)
        sys.exit(2)
    except KeyboardInterrupt:
        sys.exit(2)
