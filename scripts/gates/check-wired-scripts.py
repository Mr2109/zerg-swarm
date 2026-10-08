#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-wired-scripts.py — 门③：脚本接线自检（**挂闸档：`tri` + `--strict-report` = 阻断**）

挂接现状（2026-09-18 ③批次 · 升阻断）
------------------------------------
本门起初按门③ 自己的建议**只报告档**起步（2026-09-18 首跑如实报「断言 A 命中 8 只未挂」= 存量债，
直接阻断等于把提交闸锁死 ⇒ 同 D2 先例）。③批次把这 8 只**逐只挂了闸**（docs +2 · pub +2 ·
新 scope `tools` +1 · 新 scope `slice` +1 · 发布闸 闸②b/闸⑥），首跑 A 命中 8→0、B 未登记 0
⇒ 按「清到 0 再升阻断」的拍板把它升成：提交闸 `gates` scope · 步骤 `门③ 接线：scripts 门脚本有没有被闸调用（阻断）`
· 模式 `tri` · 命令串 `python3 scripts/gates/check-wired-scripts.py --strict-report`。
★ 升档要**两处一起改**（模式 `tri-report`→`tri` **且**命令串加 `--strict-report`）：脚本自身默认仍是
「只报告」（rc=0，命中只入清单）——**不带这个开关就等于挂了个恒绿步**（假阻断）。成对证据：
同一夹具同一命中，不带开关 rc=0 / 带开关 rc=1（`--root <夹具> --strict-report`）；升档全过程见
`scripts/gates/check-wired-scripts.md` §五（已执行）。
★ 判据与退码**一字未改**：改的只是「挂闸时的档位 + 一个开关」。

为什么存在
----------
2026-09-18 存量债务普查（docs/项目文档/v2.5.10/债务台账-20260918.md，§0 表 #11 与 §5）
记下两条同族的「假覆盖」：

  ① **有门不挂**：`check-glossary.py` / `check-manifest-freshness.py` / `check-slice.py` /
     `check-i18n-drift.sh` / `check-tool-version-sync.sh` 这 5 只门脚本**不在任何闸里** ——
     门写了、脚本绿过、但没有任何一步会替你跑它 ⇒ 它在提交时等于不存在（假覆盖）。
  ② **孤岛死重**：`scripts/` 下有零引用件（既不被 scripts/ 内调用、也不被别处引用），
     它们躺在仓里没人知道该不该留。

本门把这两条写成**可判定的**判据（不是词表、不是「看着像」）：

  断言 A（门脚本必须上岗）：凡**门脚本** —— 命名以 `check-` 开头的脚本，或**说明书自称门**
    的脚本（同干 `.md` 的头部自称门）—— 必须出现在**至少一个闸**里。
    闸 = 提交闸（`scripts/gates/precommit-gates.sh`）+ 发布闸面（见下方 GATES，现读其真实文件名与调用方式）。
    「出现在闸里」的判据 = **按名调用形态**：名字出现在闸文件的一行上，且该行
      ① 不是纯注释行（`#` 开头）——「注释里提过」不算上岗；
      ② 不是纯清单项（整行就是 `"scripts/<…>/<名>"` 这类数组/数据项，**任意层深**）——「在 EXCLUDES / DROP_EXACT
         这类排除表里被点名」不算上岗；
      ③ 名字落在**命令位**上（**真接线面**）：紧接在解释器/启动器之后（`python3` / `bash` / `${PY}` / `./` …，
         中间只允许选项旗标），可带引号与路径前缀 —— 例 `python3 scripts/gates/<名>` ·
         `python3 "$Z/scripts/gates/<名>" "$D"`。**只被文案提到**（`echo "…按名调用 <名>"`）·
         **只在说明/注释里出现** · **只在 grep 模式串里出现**一律不算。
    ★ 2026-09-26 **收紧（治根）**：旧判据只做 ①②且 ② 的正则只认 `scripts/<名>` 两段 —— 而 2026-09-20 脚本
      分域后清单项写成 `scripts/gates/<名>` ⇒ ② 当场失效；再叠加 ③ 缺失，于是「被 `publish/mirror-public-lib.py`
      的 `DROP_EXACT`（**公开面排除表**，与「接了闸」毫无关系）点名」与「只在文案里被提到」都算成「已挂」。
      实测假上岗 **4 只**：`check-registry-vs-disk.py`（`mirror-public-lib.py:390` 清单项）·
      `check-help-snapshot.py`（`:377` 清单项）· `check-git-env-hygiene.py`（`:404` 清单项）·
      `check-public-face-anchor.py`（`publish-preflight.sh:187` 文案 + `mirror-public-lib.py:346` 清单项）。
      收紧后这 4 只**如实打红**（不为变绿加豁免 / 不登记白名单：它们是**真未挂**，处置 = 接线）。

  断言 B（零引用件必须登记）：`scripts/` 顶层、**零引用**的脚本必须登记白名单（带 reason + date）。
    「引用」判据 = 该脚本名在**引用面**（见下方口径）里的出现次数。见 §口径 的三条排除。

退码（三档 · 与仓内既有门的 0/1/2 惯例一致）
--------------------------------------------
  0 = 只报告档的正常出口：**无论命中多少**（断言 A 命中 N 只、断言 B 未登记 M 只）
  1 = **仅**在 `--strict-report` 下、且至少一处命中时
  2 = **不给结论**（缺件 / 配置错 / 空转 / 自检不过）—— 一律显式打印，不许静默
      · 仓根缺 `scripts/` 目录 · 闸清单**一个都不在位** · 候选 0 只 · 引用面 0 个文件
      · 白名单项缺 reason / 缺 date / date 形态不对 / 重名 / **悬空**（点名的脚本已不存在）
      · `--self-test` 任一用例不过（自检不过 ⇒ 拒绝扫真目标，项目口径）

用法
----
    python3 scripts/gates/check-wired-scripts.py                 # 只报告（rc=0）
    python3 scripts/gates/check-wired-scripts.py --strict-report # 命中即 rc=1（临时用；常规挂闸另见 .md）
    python3 scripts/gates/check-wired-scripts.py --list-rules    # 打判据 / 闸清单 / 口径 / 排除域 / 升级路径
    python3 scripts/gates/check-wired-scripts.py --self-test     # 只跑自检（成对负控，不碰真目标）
    python3 scripts/gates/check-wired-scripts.py --root DIR --whitelist J # 对夹具跑（自检内部用）

说明书：scripts/gates/check-wired-scripts.md（为何起步只报告 · 怎么升阻断 · 两断言各自的自证方式）。
"""

import argparse
import datetime
import fnmatch
import json
import os
import re
import subprocess
import sys
import tempfile
import unicodedata

# ─────────────────────────────────────────────────────────────────────────────
# 闸清单（现读 2026-09-18 的仓内实况；改名/搬家必须同步本表与 .md §二）
# 判据只用这份表 —— 「还有哪些文件跑脚本」不进判据（否则任何脚本都能自称闸）。
# ─────────────────────────────────────────────────────────────────────────────
GATES = [
    # (路径, 类别, 标签, 现读到的调用方式)
    ("scripts/gates/precommit-gates.sh", "commit", "提交闸",
     "bash scripts/gates/precommit-gates.sh [--scope go|rust|pub|tags|docs]（步骤表 add_step 的 STEP_CMD）"),
    ("scripts/build/publish-preflight.sh", "release", "发布闸·推送前置（闸⓪~⑥，含闸②b）",
     'bash scripts/build/publish-preflight.sh <产物目录>（推送前硬闸，八道全过才允许推；rc=2 = 有闸缺件不给结论）'),
    ("publish/mirror-public.sh", "release", "发布闸·逐提交镜像器",
     "publish/mirror-public.sh --out DIR [--push]（对产出树跑门禁）"),
    ("publish/mirror-public-lib.py", "release", "发布闸·镜像器库（跑 checker 的那半边）",
     "被 publish/mirror-public.sh / publish/parity-compare.py 调用（check-history-secrets.py / check-public-tree-private.py 的调用方）"),
    ("publish/ci/ci.yml", "release", "发布闸·CI（公开仓推后闸）",
     "GitHub Actions：run: python3 scripts/... （check_version / check-gotoolchain / check-compat-manifest / check_docs / check-shell-unicode-vars）"),
    ("publish/ci/release-agent.yml", "release", "发布闸·CI（Release 重切）",
     "GitHub Actions：python3 scripts/gates/check_version.py · scripts/evals/make-manifest.py"),
    ("publish/mirror-acceptance.sh", "release", "发布闸·验收套件（推送前置）",
     "SKIP_BUILD=1 bash publish/mirror-acceptance.sh（`publish/mirror-public.sh` PUSH=1 与 `publish/mirror-cn.sh` 推送段前各一行；rc=1 拦截 · `GAP-20260927-138`）"),
]

# 相关但不是闸（打印为「排除域」；判据不吃它们 —— 别把验收套件/弃用器/测试当闸）
NOT_GATES = [
    ("scripts/build/publish-public.sh", "压平快照发布器 · 已弃用（Q5 待废弃；现役为 mirror-public.sh）"),
    ("scripts/build/test-publish-parity.sh", "两器一致性测试（测试面）"),
]

# ─────────────────────────────────────────────────────────────────────────────
# 引用面的三条排除（判据的一部分，写死在代码里、打印在输出里，改它要改 .md）
#   ① 本门自指的三件文件：否则我自己的名单/说明书会把「零引用」直接写成「有引用」（自伤）。
#   ② 本门的**结论落账面**：现行债务台账 —— 「记录不能充作引用」。
#      台账点名一个死件不会让它活过来；若把记录算作引用，那「写下这条债」这个动作本身
#      就把债抹掉了（观测行为抹掉被观测事实）。这是本门判据里唯一需要解释的一条。
#   ③ 候选脚本自身（`scripts/<名>` 那一个文件）。
# 口径对照（现跑，2026-09-18）：全仓含记录面 = 0 只零引用；本门口径 = 4 只；
#   再排除 docs/ 整个文档面 = 29 只（那会把「只在文档里指路」的活门误判成死件 ⇒ 不采）。
# ─────────────────────────────────────────────────────────────────────────────
SELF_FILES = [
    "scripts/gates/check-wired-scripts.py",
    "scripts/gates/check-wired-scripts.md",
]
RECORD_SURFACES = [
    #   ★ 2026-09-19 「项目文档」二次分家**批2**（**只加注、条目值一字不动**）：本条当时**仍在咬**
    #     （仓内现跑命中 2 件：`docs/项目文档/v2.5.10/债务台账-20260918.md` / `.tsv`）。
    #   ★ 2026-09-19 **批5 现读（文案同步 · 条目值仍一字不动）**：批3 **已落盘**（`docs/项目文档/` 整目录
    #     移出工作树 → 同级 `Zerg-内部文档/项目文档/`）⇒ 本条今天**仓内零命中**（现跑：记录面 0 件，
    #     见「【口径】…− 记录面 0」那行）。条目**本批不删**：① 删它属「零命中即删」的收尾（批6）；
    #     ② 删/留今天**行为等同**（记录面都是 0）⇒ 没有为收红去动判据的理由。
    #     不**改指**仓外（`../Zerg-内部文档/项目文档/*/债务台账-*`）的理由见下（同批2 结论）：
    #     本门只列**仓内**（文件系统遍历 / git 目录）相对路径，`../` 形态的条目前置进不来，
    #     直接写仓外 glob = 空条目（另一种假覆盖）。要真覆盖仓外那棵须把**引用面**也加上仓外根
    #     = 改判据（不是加注）⇒ 属批6 决策，本批不动。
    "docs/项目文档/*/债务台账-*",   # 本门结论的落账面（glob ⇒ 换日期/换版本不用改代码）
    #   ★ 2026-10-02（缺口 `GAP-20260927-139` · 记录面不得充作引用）：`scripts/公开标记.tsv`
    #     点名了全部 148 只候选脚本，却 0 处命令位调用 ⇒ 旧口径把它算作「代码侧引用」，
    #     于是 27 条 WHITELIST 全变「⚠ 可下架」、「未登记 N 只 ⇒ rc=1」这一档结构上永不红。
    #     本条即把这张记录类表接进 RECORD_SURFACES（门 :113-119 早已写着「记录不能充作引用」）。
    "scripts/公开标记.tsv",          # 公开面标记表（记录类表面 · 148 只候选全被点名 ⇒ 必须排除）
]

# ─────────────────────────────────────────────────────────────────────────────
# 白名单（零引用件的登记表 · 逐条 reason + date）
#   规矩（棘轮）：**只许减不许增** —— 新写一条必须能说出「它为什么还该在仓里」；
#   已登记的件一旦有了引用（接线/被别的脚本调用），下次跑会提示可下架。
#   ✗ 不许为了让首跑好看而填空：登记项必须是真死件（附理由），不是「先压下去再说」。
#   ★ 2026-09-19「项目文档」二次分家**批5**（一次 +20 条 · 逐条 reason+date）：
#     批3 把 `docs/项目文档/` 整目录移出工作树（→ 同级 `Zerg-内部文档/项目文档/`）⇒ 该目录里那批
#     评测/夹具档案不再进引用面（引用面 = 已跟踪文件），20 只脚本的**唯一**点名当场消失
#     = 断言 B「未登记 20 只」FAIL(rc=1)。处置 = **登记**（不是接线：不能为了凑引用去造调用方；
#     也不是放宽判据：引用面/断言一个字节没改）。自证（现跑）：登记前 rc=1（未登记 20）
#     ↔ 登记后 rc=0（未登记 0 · 白名单 25/25），`--self-test` 16/16。
#   ★ 棘轮仍然有效：这 20 条一旦重新有了引用，`--list-rules` 会提示可下架。
# ─────────────────────────────────────────────────────────────────────────────
WHITELIST = [
    {
        "name": "v25_more_classes.py",
        "reason": "一次性历史评测脚本：v2.5 补充任务类测试（example-35b-v2 · 2026-08-13），"
                  "结果已并入当版文档；保留作评测口径的可复核原件。",
        "date": "2026-09-18",
    },
    {
        "name": "v25_more_tasks.py",
        "reason": "一次性历史评测脚本：v2.5 更多实战任务（2026-08-13），同族口径。",
        "date": "2026-09-18",
    },
    {
        "name": "v25_real10_v2.py",
        "reason": "一次性历史评测脚本：v2.5 真实任务集重测（responses 后 · 2026-08-13），同族口径。",
        "date": "2026-09-18",
    },
    {
        "name": "v25_real10.py",
        "reason": "一次性历史评测脚本：v2.5 真实任务集（2026-08-13，同族口径见 v25_real10_v2.py）。"
                  "它原本唯一的引用 = `docs/issues/live-task2.log`（引擎运行态日志，非跟踪件）——"
                  "该目录已于 2026-09-19 随开发文档分家整目录迁出工作树（`Zerg-内部文档/issues/`）"
                  "⇒ 引用面归零，**接线已不可行**（不能为了凑引用去造一个调用方）。"
                  "登记自证：登记前 断言 B「未登记 1 只」= FAIL(rc=1) ↔ 登记后 白名单 5 条、"
                  "未登记 0 只 = rc=0（豁免数只增这 1 条，逐条带 reason+date）。",
        "date": "2026-09-19",
    },
    {
        "name": "v24_full_test.py",
        "reason": "零引用件（登记，非死件）：v2.4 全量评测脚本（测试存档体系的 v2.4 那一代）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.9/09-模块-Swift壳工具链-20260829.md` 与 `…/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "v25_agent_tasks.py",
        "reason": "零引用件（登记，非死件）：v2.5 子端/agent 类任务评测脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`（另见 `v2.5.0/设计-测试存档体系.md`）——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "v25_benchmark.py",
        "reason": "零引用件（登记，非死件）：v2.5 基准/对比评测脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "v25_eval_task.py",
        "reason": "零引用件（登记，非死件）：v2.5 单任务评测脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md` 与 `v2.5.10/任务表-协作骨架v2.1仓内核对结论-20260918.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "v25_full_test.py",
        "reason": "零引用件（登记，非死件）：v2.5 全量测试脚本（v2.5.0 时代测试存档体系那一代）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md` 与 `v2.5.0/测试报告-v2.5.md`、`v2.5.0/INDEX.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "v25_m1_accept.py",
        "reason": "零引用件（登记，非死件）：v2.5 M1 里程碑验收脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "v25_real_tasks.py",
        "reason": "零引用件（登记，非死件）：v2.5 真实任务集评测脚本（同族见 v25_real10.py / v25_real10_v2.py）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`（另见 `v2.5.0/设计-测试存档体系.md`）——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "v25_realproj.py",
        "reason": "零引用件（登记，非死件）：v2.5 真实项目集评测脚本（同族口径）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "v25_research.py",
        "reason": "零引用件（登记，非死件）：v2.5 调研类任务评测脚本（同族口径）。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "run_all.sh",
        "reason": "零引用件（登记，非死件）：v2.4/v2.5 评测批跑总入口。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.6~v2.5.9/09-模块-Swift壳工具链-20260829.md` 与 `…/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "calib-x3-model.sh",
        "reason": "零引用件（登记，非死件）：EDGE-NODE 模型标定脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`、`v2.5.9/茧壁-自主作业日志.md`、`v2.5.10/09-模块-Swift壳工具链-20260829.md`、`v2.5.10/12-体系-评测部署运维-20260829.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "test-fleet-sandbox.sh",
        "reason": "零引用件（登记，非死件）：舰队面沙箱端到端夹具脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "test-relay-sandbox.sh",
        "reason": "零引用件（登记，非死件）：中转（relay）沙箱夹具脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "test-setup-sandbox.sh",
        "reason": "零引用件（登记，非死件）：安装/初始化沙箱夹具脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`（唯一一处点名）——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "test-update-sandbox.sh",
        "reason": "零引用件（登记，非死件）：升级/换件沙箱夹具脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.9/变更-v2.5.9.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "stability-stats.py",
        "reason": "零引用件（登记，非死件）：稳定性统计脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.10/任务表-协作骨架v2.1仓内核对结论-20260918.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    {
        "name": "ab-campaigns.py",
        "reason": "零引用件（登记，非死件）：A/B 活动编排脚本。它在**仓内已跟踪引用面**里唯一的点名出自 `docs/项目文档/v2.5.10/任务表-协作骨架v2.1仓内核对结论-20260918.md`——该目录（`docs/项目文档/`，17 套快照）已于 2026-09-19「项目文档」二次分家**整目录移出工作树**（现位置 = 同级 `Zerg-内部文档/项目文档/`，件级 sha256 已核对）⇒ 仓内引用面不再含该点名，**接线已不可行**（不能为了凑引用去造一个调用方）。登记自证：登记前 断言 B「未登记 20 只」= FAIL(rc=1) ↔ 登记后 未登记 0 只 = rc=0（豁免只增这 20 条、逐条带 reason+date，判据一字未改）。",
        "date": "2026-09-19",
    },
    # ── 2026-09-23 **波④ 接线收口**：波③b 两枚**只读半件**的登记（逐条 reason + date）───────────────
    #   为什么是「登记」而不是「接线」：本门的口径逐字写着「要么接线（被谁调用）、要么登记白名单（附理由）」
    #   且写着「**不能为了凑引用去造一个调用方**」。这两枚**自己声明不判**（见下逐条的引用原文）
    #   ⇒ 不是门、不该挂进任何闸（挂成步 = 挂了个不会判的步）。
    #   ★ 登记自证（现跑 · 同一时点）：登记前 **断言 B「未登记 2 只」= rc=1**（`--strict-report`）↔
    #     登记后 **未登记 0 只 = rc=0**；白名单 25 → 27 条；断言 A 两跑都 0（判据**一字未改**）。
    #   ★ 棘轮仍有效：这两件一旦重新有了引用（谁真去调它），`--list-rules` 会提示可下架。
    # ── 2026-10-08 缺口 `GAP-20260927-148` **新口径重登记**（24 条 · 逐条 reason 逐字同 Go 半件 `cwsWhitelist`）──
    #   旧 B 口径 = 「任一出现次数」⇒ 这 24 件在记录面/散文/注释/清单里的点名被读成「有引用」⇒
    #   「可下架」名单静默失真；换成与断言 A 同源的「命令位调用」口径后判零引用 ⇒ 按棘轮逐条登记。
    {
        "name": "lib-sign.sh",
        "reason": "零命令位引用（登记 · 非死件 · `GAP-20260927-148` 新口径 = 与断言 A 同源的「命令位调用」）：macOS 签名身份公共库；被 `scripts/build/build-all.sh` 与 `scripts/build/pack-release.sh` 以 shell `source` 内建引入（`source` 不在本门启动器表内）+ 读判据件 `[ -f \"$REPO_ROOT/scripts/build/lib-sign.sh\" ]` ⇒ 判零引用；Go 对等体 = `scripts/build/internal/sign`。活件（唯一签名真源），保留。",
        "date": "2026-10-08",
    },
    {
        "name": "publish-public.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：压平快照发布器（Q5 已弃用 · 现役 = `publish/mirror-public.sh`）；未被任何闸/脚本按命令位执行，仍作 `EXCLUDES` 单一真源被多件按路径读正文（check-history-secrets / check-public-tree-private / check-registry-vs-disk / family_publish 等 Go 半件）⇒ 零引用。留待 Q5 退役。",
        "date": "2026-10-08",
    },
    {
        "name": "start-zerg-core.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：主控启动／按仓根重写 plist 的引导脚本；被 `scripts/build/zerg-upgrade.sh` 以变量 `STARTCORE=\"${ZERG_START_CORE:-…}\"` 赋值引用（变量间接 ⇒ 非命令位字面）⇒ 零引用。活件保留（plist 真源形态）。",
        "date": "2026-10-08",
    },
    {
        "name": "start-zerg-ui.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：UI 脱离会话启动的单一真源（CLI `zerg ui start` 壳转账 · `core/cmd/zerg/ui_start.go` 的常量 + exec）；引用为 Go 常量与 shell 变量（`LAUNCHER_REL` / `STARTUI`），非命令位字面 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "zerg-swap-agent.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：子端换件脚本（只报告档）；CLI `zerg swap agent` 壳转账（`core/cmd/zerg/swap_agent.go` 的常量 + exec），引用为 Go 常量而非命令位字面 ⇒ 零引用。活件保留（换件三闸形态）。",
        "date": "2026-10-08",
    },
    {
        "name": "zerg-swap-core.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：主控换件脚本（事故面固化 · 三道门禁＋两前置闸）；CLI `zerg swap core`（`core/cmd/zerg/family_swap_core.go` 常量）＋ `.githooks/pre-commit` 与步骤表注释点名 ⇒ 无命令位字面引用。活件保留（换件唯一入口）。",
        "date": "2026-10-08",
    },
    {
        "name": "zerg-swap-ui.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：桌面 UI 换件脚本（只报告档）；CLI `zerg swap ui`（`core/cmd/zerg/swap_ui.go` 常量 + exec），引用为 Go 常量 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "zerg-upgrade.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：六阶段换件内核（自举／发布共用 · Go 与 Rust 两处皆调用方）；被 `core/internal/selfupdate` 与 `scripts/svc/fleet-relay.sh`、各 `scripts/sandbox/*` 以变量 `KERNEL` 赋值调用（变量间接）⇒ 非命令位字面 ⇒ 零引用。活件保留（真源内核）。",
        "date": "2026-10-08",
    },
    {
        "name": "calib-local-model.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：模型标定件；CLI `zerg calib local-model`（`core/cmd/zerg/family_calib.go` 数据 `File` + `Run: bash`），引用为 Go 数据结构字段而非命令位字面 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "calib-task-budget.py",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：标定件；CLI `zerg calib task-budget`（`core/cmd/zerg/family_calib.go` 数据 `File` + `Run: python3`），引用为 Go 字段 ⇒ 零引用。活件保留（闸门日后只读其档案）。",
        "date": "2026-10-08",
    },
    {
        "name": "inplace-window.py",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：就地窗口公共件（`GAP-20260927-112`）；Go 真搬 = `core/cmd/zerg/inplace_window.go`；被 `scripts/gates/two-states-gate-mutate.py` / `scripts/wall/wall-bridge-mutate.py` 以 importlib 载入（路径拼在 Python 字符串／docstring 里，非命令位字面）⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "chat-harness.py",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：评测夹具件；CLI `zerg eval chat-harness`（`core/cmd/zerg/family_eval.go` 数据 `File` + `Run: python3`），引用为 Go 字段 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "compare.py",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：评测比较器；CLI `zerg eval compare`（`core/cmd/zerg/family_eval.go` ＋ Go 真搬 `eval_compare.go`），引用为 Go 字段 ⇒ 零引用。活件保留（对拍通过后逐件点名退场）。",
        "date": "2026-10-08",
    },
    {
        "name": "replay-fidelity.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：复现保真度验收件；CLI `zerg eval replay-fidelity`（`core/cmd/zerg/family_eval.go` 数据 `File` + `Run: bash`），引用为 Go 字段 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "accept-no-probe-selftest.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：验收自证件；由 `scripts/gates/accept-no-probe.sh` 以变量 `SELFTEST` 转交，并在 `scripts/gates/real-gates.sh` 的 `SH_GATES=(…)` 数组项中被循环调用（变量／数组间接）⇒ 非命令位字面 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "accept-no-probe.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：验收负控夹具；`scripts/gates/real-gates.sh` 的 `SH_GATES=(…)` 数组成员（循环变量间接调用）⇒ 非命令位字面 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "judge-targets.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：overlay 两态取证靶；`scripts/gates/real-gates.sh` 的 `SH_GATES=(…)` 数组成员（循环变量间接）⇒ 非命令位字面 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "overlay-guard.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：overlay 叠加鉴别器；同上 `scripts/gates/real-gates.sh` 的 `SH_GATES=(…)` 数组成员 ⇒ 非命令位字面 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "shard-go-test.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：贵步分片执行器；在 `scripts/gates/precommit-gates.sh` 的 `add_step …` 命令串里被调（命令串内引号被转义 ⇒ 落本门命令位正则的引号识别之外）⇒ 判零引用。活件保留（真接线）。",
        "date": "2026-10-08",
    },
    {
        "name": "zerg-hello",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：仓内演示插件（`§6.3 S6` 判据①证据件 · 零副作用）；由插件机制按名调用（CLI `zerg hello`），非 shell 命令位字面 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "archive-mount.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：归档区按需挂载命令面（on/off/status）；CLI `zerg archive mount`（`core/cmd/zerg/archive_mount.go` 常量 + exec），引用为 Go 常量 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "fleet-relay.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：机群镜像中继；被 `scripts/sandbox/test-relay-sandbox.sh` 以变量 `RELAY` 调用（变量间接），其余点名在注释／文案里 ⇒ 非命令位字面 ⇒ 零引用。活件保留。",
        "date": "2026-10-08",
    },
    {
        "name": "zerg-core-daemon.sh",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：launchd 托管的启动包装；由 `deploy/服务声明.tsv` 的 `/bin/bash …`（绝对解释器 · TSV 声明列）与 plist 引用（绝对解释器不在本门启动器表＋非命令位文件面）⇒ 零引用。活件保留（com.zerg.core 的被调入口）。",
        "date": "2026-10-08",
    },
    {
        "name": "zerg-egg.py",
        "reason": "零命令位引用（登记 · 非死件 · 新口径）：.egg 封装工具的 py 原件；Go 真搬 = `core/cmd/zerg/zerg_egg.go`（DEV-1798），原件按退场规则保留待对拍通过后逐件点名退场 ⇒ 无命令位字面引用。活件保留。",
        "date": "2026-10-08",
    },
]

# ─────────────────────────────────────────────────────────────────────────────
# 断言 A 的「建议挂到哪里」（逐条给理由/证据；键 = 脚本名）
#   本表只影响**报告的可行动性**，不影响判据 —— 没登记建议的命中项会打印「未登记建议 ⇒ 人工判定」。
# ─────────────────────────────────────────────────────────────────────────────
SUGGEST = {
    "check-slice.py": (
        "提交闸 · 新 scope（例 slice，模式 tri-report 起步）",
        "自带 `--probe` / `--selftest` / `--list-rules` 三档与「自检不过 rc=2」的口径"
        "（Zerg-内部文档/01-设计/报告模板-六项成对.md:69-70 —— 该件 2026-09-19 已随开发文档分家迁出工作树，原路径 docs/01-设计/）⇒ 可直接当步骤；"
        "但体量大（109 KB + slice-probes.json）⇒ 成本未标定，起步只报告。"),
    "check-i18n-drift.sh": (
        "提交闸 · docs scope（模式 tri-report 起步）",
        "漂移三件套（配对 / 提交级陈旧 / 源注释），①②阻塞、③告警（docs/en/README.md:99）；"
        "用法 `bash scripts/gates/check-i18n-drift.sh`（docs/site/README.md:189）；存量债未清 ⇒ 起步只报告。"),
    "check-tool-version-sync.sh": (
        "提交闸 · 新 scope（例 tools，模式 tri）",
        "它就是为「发布前勾选」造的（docs/skills/tool-upgrade-checklist.md:52 列在手检清单里：台账 ↔ 工具文档抬头 ↔ 履历）；"
        "自带前置自检（缺 tools/versions.json 即 rc=2）⇒ 与 tri 三档一一对应，可直接阻断。"),
    "check-hardcoded-private-paths.py": (
        "发布闸 · scripts/build/publish-preflight.sh 新增一道（紧跟闸② 私有面，模式 tri）",
        "它判私有绝对路径，与 闸②（私有面）/ 闸④（危险路径）同族；"
        "今天它只被 scripts/build/publish-public.sh:130 的 EXCLUDES **清单项** 与 precommit-gates.sh:500 的**注释**提到 —— "
        "从没被任何闸按名执行 ⇒ 按本门判据必须报出来（子串口径会漏掉这一只）。"),
    "edit-assert": (
        "提交闸 · pub scope 增一步 `python3 scripts/docs/edit-assert --self-test`（模式 tri）",
        "它今天只被 pub scope 的「scripts/*（无后缀 + 首行 #!）按 shebang 语法」步按 **glob** 做语法检查"
        "（precommit-gates.sh:521），**没有按名跑过它的自检**；自检自带真命令行 + 真退出码 16 条（scripts/docs/edit-assert.md:34）⇒ 有牙齿。"),
    "mutate-scan": (
        "提交闸 · pub scope 增一步 `python3 scripts/gates/mutate-scan --self-test`（模式 tri）",
        "同族：只在 glob 语法检查里被覆盖；自检不过即拒绝跑真目标（scripts/gates/mutate-scan.md:27, 48）⇒ 适合当步骤。"),
    # ── 2026-09-26 判据收紧后新露出的 4 只（**真未挂**：旧判据把它们读成「已挂」，证据只是公开面排除表
    #    `publish/mirror-public-lib.py` 的 `DROP_EXACT` 清单项 / `publish-preflight.sh` 里的一句文案）──
    #    ★ 本表只给「挂到哪里」的建议，不改判据、不改退码；**不把它们登记白名单**（登记 = 说它该在仓里当死件，
    #      与事实相反：这 4 只都该挂）。
    "check-registry-vs-disk.py": (
        "提交闸 · gates scope 增一步 `python3 scripts/gates/check-registry-vs-disk.py --strict`（模式 tri）",
        "纯仓内只读：登记表（`publish/whitelist.txt` / `EXCLUDES`）↔ 盘上真物对拍（空指 / 悬空 / 文案不符）；"
        "与闸②b / 闸④ 同族，但它的目标面在仓内 ⇒ 也可放发布闸。"
        "★ 现读 `--strict` = **rc=1（三格合计 5 处）**，而今天没有任何真闸会跑到它 ⇒ 这 5 处红是**隐形的**"
        "（`all.sh` 逐门自检档只跑它的夹具自检 rc=0；`real-gates.sh` 是按变量名泛跑的档，不是按名接线）。"),
    "check-git-env-hygiene.py": (
        "提交闸 · gates scope 增一步 `python3 scripts/gates/check-git-env-hygiene.py --strict`（模式 tri）",
        "它判「hook / 门禁脚本对外仓·合成仓调 git 前先清环境」= **门禁自身的卫生面**，与 gates scope 同族；"
        "现档 = 报告档（件内自称与 `check-registry-vs-disk.py` 同规）⇒ 起步可 tri-report。"),
    "check-public-face-anchor.py": (
        "发布闸 · `scripts/build/publish-preflight.sh` ⑧ 系列新增一道（紧跟 ⑧c 命令名 / ⑧d 成对闸，模式 tri）",
        "公开面**漂移锚**（两版窗口对拍）与 ⑧c/⑧d 同面同规（同属「公开面判据件」）；"
        "今天它只被 `publish-preflight.sh:187` 的一句**文案**（`.py` 名字出现在那行的尾注里）与"
        "`publish/mirror-public-lib.py:346` 的 `DROP_EXACT` 清单项提到 ⇒ 0 次真调用。"),
    "check-help-snapshot.py": (
        "提交闸 · docs scope 增一步 `python3 scripts/gates/check-help-snapshot.py`（模式 tri-report 起步）",
        "判「`--help` 现跑 ↔ 最新快照」；读同干文档仓（默认同级 `Zerg-内部文档`）⇒ 落 docs 面。"
        "与文档类门同族，存量差异未清 ⇒ 起步只报告（同 `check-glossary.py` 先例）。"),
}

# ─────────────────────────────────────────────────────────────────────────────
# 报告宽度计算（中文按 2 列宽；本机 python3 = 3.9，不用 3.10+ 语法）
# ─────────────────────────────────────────────────────────────────────────────
def dw(s):
    return sum(2 if unicodedata.east_asian_width(c) in ("W", "F") else 1 for c in s)

def pad(s, n):
    s = str(s)
    w = dw(s)
    return s + " " * max(0, n - w)

def trunc(s, n):
    s = str(s)
    if dw(s) <= n:
        return s
    out = ""
    for c in s:
        if dw(out) + dw(c) > n - 1:
            return out + "…"
        out += c
    return out

def rel(root, p):
    try:
        return os.path.relpath(p, root)
    except Exception:
        return p

# ─────────────────────────────────────────────────────────────────────────────
# 扫描面
# ─────────────────────────────────────────────────────────────────────────────
def git_out(root, args):
    try:
        r = subprocess.run(["git"] + args, cwd=root, stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE)
    except OSError:
        return None
    if r.returncode != 0:
        return None
    return r.stdout.decode("utf-8", "surrogateescape")

SKIP_DIRS = {".git", ".zerg", "node_modules", "__pycache__", "target", "dist",
             "vendor", "venv", ".venv", ".idea", ".vscode"}
SKIP_EXT = {".png", ".jpg", ".jpeg", ".gif", ".ico", ".pdf", ".gguf", ".bin", ".db",
            ".zip", ".tar", ".gz", ".so", ".dylib", ".a", ".o", ".woff", ".woff2",
            ".ttf", ".mp4", ".webp", ".pt", ".safetensors", ".lock", ".pyc"}
MAX_BYTES = 3 * 1024 * 1024

def list_files(root):
    """引用面/候选面的文件清单。

    · 真仓（有 .git）⇒ `git ls-files`（= 索引里的已跟踪文件；**未跟踪件不在判据内**，
      与债务台账的现读口径 `git grep` 一致，也让并行路上的半成品不会抖动本门数字）。
    · 非 git 目录（自检夹具）⇒ 文件系统遍历，同样排除 SKIP_DIRS。
    返回 (files, mode)。
    """
    if os.path.isdir(os.path.join(root, ".git")):
        out = git_out(root, ["ls-files", "-z"])
        if out:
            fs = [p for p in out.split("\0") if p]
            return fs, "git-ls-files（已跟踪 = 索引）"
    fs = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS and not d.startswith(".")]
        for fn in filenames:
            p = os.path.join(dirpath, fn)
            if os.path.islink(p):
                continue
            fs.append(rel(root, p))
    return sorted(fs), "文件系统遍历（非 git 目录）"

def candidate_files(root, files, mode):
    """候选面（**枚举待判对象**）—— 可换 fs；**引用面（judge_b 吃的 `files`）不许换**。

    ★ `GAP-20260927-159/169`：候选面原先也从索引派生（`list_files` 只给 `git ls-files`）
      ⇒ 未跟踪的新件**静默掉出候选面**（本门开张后新建的脚本永远看不到 = 永久静默）。
      改法照 `check-gate-coverage.py` 的现成形状：**候选面 = 索引件 ∪ 未跟踪未忽略件**，
      索引退成分档器（tracked ⇒ 存量债 / untracked ⇒ 新件）。
    ★ 方向性（`GAP-20260927-169`）：只换**候选面**这一半；引用面 `files` 一字不动 ——
      在途件里一句字面引用会把「零引用死件」**洗白**（= 松判），故引用面必须仍走索引。
    ★ 禁裸 fs 递归：白名单前缀下 ignored 实测 170,736 件 ⇒ 盘上面一律走
      `git ls-files --others --exclude-standard`，不用 `os.walk` 全量。
    """
    if not mode.startswith("git"):
        return files                      # 非 git 夹具：list_files 已是 fs 遍历 ⇒ 原样
    out = git_out(root, ["ls-files", "--others", "--exclude-standard", "-z"])
    extra = [p for p in (out or "").split("\0") if p]
    if not extra:
        return files
    return sorted(set(files) | set(extra))

def read_text(path):
    try:
        if os.path.getsize(path) > MAX_BYTES:
            return None
        with open(path, "rb") as f:
            d = f.read()
        if b"\0" in d[:8192]:
            return None
        return d.decode("utf-8", "surrogateescape")
    except Exception:
        return None

def is_record_surface(p):
    """记录面判定：整串 fnmatch（`*` 在本实现里可跨 `/`）+ 逐段 fnmatch 兜一层。

    注意 fnmatch 的 `*` 会匹配到 `/`（与 glob 语义不同）—— 对本门的记录面 glob
    （`docs/项目文档/*/债务台账-*`）正是想要的：* 吃到版本目录名。
    """
    for pat in RECORD_SURFACES:
        if fnmatch.fnmatch(p, pat):
            return True
        seg = pat.split("/")
        parts = p.split("/")
        if len(parts) == len(seg) and all(fnmatch.fnmatch(a, b) for a, b in zip(parts, seg)):
            return True
    return False

# ─────────────────────────────────────────────────────────────────────────────
# 候选（门脚本）
# ─────────────────────────────────────────────────────────────────────────────
DOC_MARK = "门"          # 说明书自称门的标记（同干 .md 头部出现即算）
DOC_HEAD_LINES = 60

def is_gate_file(p):
    return any(g[0] == p for g in GATES)

# ── 候选面的排除域 ────────────────────────────────────────────────────────────
#   scripts/ 下**分域之前就有**的工作/实验子目录：不递归进候选面（既有口径，2026-09-20 前
#   它们本就因「只看顶层」而不在判据里）。脚本分域第二段把 100 件搬进 9 个**域**目录后，
#   候选面下沉一层——但只下沉到域目录，这两棵仍旧打印成「排除域」（口径见 .md「排除域」节）。
#   ★ 2026-09-20 T-56 决议⑤：第三棵 `scripts/253/` 已退役删除（零调用方 · **删目录与摘名单同
#     一提交**）⇒ 名单从三棵收成两棵，见下 `EXCL_SUBDIRS` 与 `check-gate-coverage.py` 的
#     `NOSUFFIX_EXCL`（两个门同口径，只改一处会打架）。
EXCL_SUBDIRS = ("exportnames", "sandbox-probes")

def _candidate_paths(files):
    """scripts/ 顶层件 + scripts/<域>/ 一件（域 = 除 EXCL_SUBDIRS 外的子目录）。"""
    out = []
    for p in files:
        if not p.startswith("scripts/") or p == "scripts/":
            continue
        parts = p.split("/")
        if len(parts) == 2:
            out.append(p)                       # scripts/<件>
        elif len(parts) == 3 and parts[1] not in EXCL_SUBDIRS:
            out.append(p)                       # scripts/<域>/<件>
    return out

def build_candidates(root, files):
    """返回 (cands, notes)。

    候选面 = scripts/ 下**顶层 + 域子目录一层**的所有脚本（.py / .sh / 无后缀但首行 #!）——
    2026-09-20 脚本分域第二段起，100 件脚本下沉到 `scripts/<域>/` ⇒ 候选面随之下沉一层
    （旧口径只看 `scripts/` 顶层，下沉后顶层归零 ⇒ 本门会误报「候选 0 只」拒绝出结论）。
    **既有的两棵工作/实验子目录仍在排除域**（见 EXCL_SUBDIRS 与 .md「排除域」节）：
    它们不是域目录，进候选面就是**改判据**，不在「机械搬动」的范围内。
    （第三棵 `scripts/253/` **已于 2026-09-20 退役删除** —— T-56 决议⑤：引用面 6 行 / 5 个
      门禁件、零调用方；那条既有的零引用件 `scripts/253/branch_system.sh` 随之消失。）
    **两断言共用同一份**：
      · 断言 A 只吃其中 kind 非空的门脚本（命名 check- / 说明书自称门）；
      · 断言 B 吃全部（零引用件不限门脚本 —— 今天的 4 只里三只是 v25_* 评测脚本）。
    """
    cands = []
    notes = []
    tops = _candidate_paths(files)
    for p in sorted(tops):
        ap = os.path.join(root, p)
        base = os.path.basename(p)
        if not os.path.isfile(ap):
            continue
        kind = ""
        evidence = ""
        if not base.endswith((".py", ".sh")):
            try:
                with open(ap, "rb") as f:
                    head = f.read(2).decode("latin-1")
            except Exception:
                head = ""
            if head != "#!":
                continue                      # 无后缀又没 shebang ⇒ 不是脚本（数据/目录/别的）
        if base.startswith("check-"):
            kind = "命名 check-"
            evidence = "名以 check- 开头"
        else:
            # 说明书自称门：**同目录**同干 .md 的头部出现「门」
            stem = os.path.splitext(base)[0]
            md = "%s/%s.md" % (os.path.dirname(p), stem) if os.path.dirname(p) else "%s.md" % stem
            if md in files:
                t = read_text(os.path.join(root, md))
                if t:
                    for i, line in enumerate(t.splitlines()[:DOC_HEAD_LINES], 1):
                        if DOC_MARK in line:
                            kind = "说明书自称门"
                            evidence = "%s:%d 「%s」" % (md, i, trunc(line.strip(), 46))
                            break
        cands.append({"name": base, "path": p, "kind": kind, "evidence": evidence})
    return cands, notes

# ─────────────────────────────────────────────────────────────────────────────
# 断言 A：出现在至少一个闸里（按名调用形态）
# ─────────────────────────────────────────────────────────────────────────────
def bare_item_re(name):
    """纯清单项的整行形态：`"scripts/<…>/<名>",`（**任意层深**）。

    ★ 2026-09-26 修：旧式只认 `scripts/<名>` 两段 ⇒ 2026-09-20 脚本分域（清单项写成
      `"scripts/gates/<名>"`）之后这条排除当场失效，清单项被当成「接线」（本门的病根）。
    """
    return re.compile(r'^["\']?(?:\./)?(?:scripts/)?(?:[A-Za-z0-9_.\-]+/)*'
                      + re.escape(name) + r'["\']?,?$')

# ── 「真接线」= 命令位调用（判据的③，见模块头）────────────────────────────────
#   解释器/启动器闭集；`${PY}` / `$PY` / `./` 也算（闸里常用变量形式的解释器）。
RUNNER_RE = (r"\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*"
             r"|python3|python|bash|sh|zsh|dash|node|env|exec|time|nice|\./")
PATH_PREFIX = r"(?:[\w.${}/-]*/)"

def call_re(name):
    """返回「命令位调用」正则：启动器 + （选项旗标）* + 路径 + 名。

    两种形态都收：带引号（`python3 "$Z/scripts/gates/<名>" "$D"`）· 不带引号
    （`python3 scripts/gates/<名> --strict`）。名字后面必须是分隔符（空白 / 引号 / `;` / `&` / `|` / `)` / 行尾）
    —— 防止 `check-x.py` 命中 `check-x.py.bak` 这类前缀。
    """
    n = re.escape(name)
    head = r'(?:^|[\s"\'=(;|&])(?:' + RUNNER_RE + r')\s+(?:-[^\s]+\s+)*'
    return re.compile(head + r'"' + PATH_PREFIX + r'*' + n + r'"'
                      + r'|' + head + PATH_PREFIX + r'*' + n + r'(?=[\s"\';|)]|$)')

# ── 「zerg 子命令」接线形：`zerg gate check <判据名>`（`GAP-20261001-74`）─────────────
#   ★ 光把 `zerg` 加进 RUNNER_RE **不解**：call_re 的 head 是「启动器 + 旗标* + 路径前缀 + 名」，
#     而 `zerg` 与判据名之间夹着 `gate check` **两个词（非旗标）** ⇒ 现有正则结构上吃不进
#     ⇒ 另写专用规则（闭集语义 = 启动器；`gate check` 不是旗标、不能塞进旗标位）。
#   ★ 判据名 = 件名**去扩展名**（与 family_gate_check.go / gate_inproc_table.go 的键同口径）。
ZERG_GATE_RE = r"(?:[^\s]*/)?zerg\s+gate\s+check\s+(?:[^\s]*/)*"

def zerg_gate_re(name):
    """`zerg gate check <件名去扩展名>` 命令位形：启动器 `zerg`（可带路径前缀）＋ `gate check`
    ＋ 路径前缀 ＋ 去扩展名判据名 ＋ 边界（名后可接旗标 / 分隔符 / 行尾）。"""
    stem = os.path.splitext(name)[0]
    head = r'(?:^|[\s"\'=(;|&])'
    return re.compile(head + ZERG_GATE_RE + re.escape(stem) + r'(?=[\s"\';|)&]|$)')

def wiring_hit(root, gate_path, name):
    """在闸文件里找「真接线形态」；返回 (行号, 行内容) 或 None。

    三类不算上岗（见模块头 ①②③）：
      ① 纯注释行（`#` 开头）；
      ② 纯清单项（整行就是 `"scripts/<…>/<名>",` —— 排除表/白名单点名叫不动它）；
      ③ 不在命令位上（只被 `echo` 文案 / 说明文字 / grep 模式串提到）。
    """
    p = os.path.join(root, gate_path)
    t = read_text(p)
    if t is None:
        return None
    rx = bare_item_re(name)
    rx_call = call_re(name)
    rx_zerg = zerg_gate_re(name)
    stem = os.path.splitext(name)[0]
    for i, line in enumerate(t.splitlines(), 1):
        # ★ 预筛放宽：`zerg gate check` 形里名字**不带扩展名**（行里无 `name`、只有去扩展名 stem）
        if name not in line and stem not in line:
            continue
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        if rx.match(s):
            continue
        if not (rx_call.search(line) or rx_zerg.search(line)):
            continue
        return (i, s)
    return None

def judge_a(root, cands):
    """返回 (hits, wired)：hits = 未挂的候选；wired = 已挂的候选（附证据）"""
    hits, wired = [], []
    n_gate_present = 0
    missing_gates = []
    for g, kind, label, how in GATES:
        if os.path.isfile(os.path.join(root, g)):
            n_gate_present += 1
        else:
            missing_gates.append(g)
    for c in cands:
        if is_gate_file(c["path"]):
            wired.append((c, "闸自身（它就是闸）", "%s" % c["path"]))
            continue
        found = None
        for g, kind, label, how in GATES:
            h = wiring_hit(root, g, c["name"])
            if h:
                found = (g, label, h[0], h[1])
                break
        if found:
            wired.append((c, found[0], ":%d  %s" % (found[2], trunc(found[3], 56))))
        else:
            hits.append(c)
    return hits, wired, n_gate_present, missing_gates

# ─────────────────────────────────────────────────────────────────────────────
# 断言 C：件头自述三态（**显式 token** · 首版 UNSTATED-only）
#   ★ ✗ 不解析自然语言：件头现读里 `不挂死`（=不卡住）/`只报告档起步`（历史档位）/`未挂`（描述历史）
#     都曾字面碰撞（见设计稿 §1.6）⇒ 任何「按词面抽」的判据都会把 `不挂死` 读成「未接线」。
#     显式 token 把「语义」搬出判据、搬进件头作者的手；判据只做**逐字枚举**（闭集＞自由文本）。
#   ★ 形（写死）：行首（可带缩进）`#` ＋ 空白 ＋ `zerg-wire:` ＋ 空白 ＋ <WIRED|UNWIRED|UNSTATED>；
#     位置：件头**前 40 行内**、**全件唯一**（0 个 / ≥2 个 ⇒ 判 UNSTATED，不猜）。
#   ★ 首版 UNSTATED-only：`UNSTATED` **只打印+计数、不进 rc**（≈ 门① 基线棘轮先例，不锁死提交闸）；
#     只有**失真**（`WIRED`×真未挂 ／ `UNWIRED`×真已挂）才计 `hits_c`，其 rc 同断言 A/B 受
#     `--strict-report` 门控 ⇒ **默认档不因新增面而转红**（现读全仓 0 token ⇒ hits_c 恒 0）。
# ─────────────────────────────────────────────────────────────────────────────
WIRE_MARK = "zerg-wire:"
WIRE_HEAD_LINES = 40
_WIRE_TOKEN_RE = re.compile(r'^\s*#\s*zerg-wire:\s*(WIRED|UNWIRED|UNSTATED)\b')

def read_wire_token(root, relpath):
    """读件头前 `WIRE_HEAD_LINES` 行内**唯一**的显式 token。返回 (state, note)。"""
    t = read_text(os.path.join(root, relpath))
    if t is None:
        return "UNSTATED", "读不到件"
    vals = []
    for line in t.splitlines()[:WIRE_HEAD_LINES]:
        m = _WIRE_TOKEN_RE.match(line)
        if m:
            vals.append(m.group(1))
    if len(vals) == 1:
        return vals[0], ""
    if not vals:
        return "UNSTATED", "件头前 %d 行无 token" % WIRE_HEAD_LINES
    return "UNSTATED", "件头 token 多重（%s）⇒ 不猜" % "/".join(vals)

def judge_c(root, cands, wired_names):
    """断言 C：件头自述三态 × 真接线。返回 (hits, rows, counts)。

    rows=[(name, declared, wired, note)]；counts = 三态计数。
    hits = **失真**（WIRED×未挂 ／ UNWIRED×已挂）；`UNSTATED` 一律不进 hits（首版不判红）。
    """
    hits, rows = [], []
    counts = {"WIRED": 0, "UNWIRED": 0, "UNSTATED": 0}
    for c in cands:
        declared, note = read_wire_token(root, c["path"])
        counts[declared] = counts.get(declared, 0) + 1
        wired = c["name"] in wired_names
        rows.append((c["name"], declared, wired, note))
        if declared == "WIRED" and not wired:
            hits.append((c["name"], "WIRED", "自称已接线，但闸清单命令位里找不到它"))
        elif declared == "UNWIRED" and wired:
            hits.append((c["name"], "UNWIRED", "自称未接线，但闸清单里已按名调用它"))
    return hits, rows, counts

# ─────────────────────────────────────────────────────────────────────────────
# 断言 B：零引用件必须登记白名单
# ─────────────────────────────────────────────────────────────────────────────
def ref_cmd_hits(name, text):
    """引用面「命令位调用」计数（B 列与断言 A **同口径** · 缺口 `GAP-20260927-148`）。

    与 `wiring_hit` 用**同一套谓词**（启动器 + 命令位 + 名字边界），区别只在
    「命中即止（bool）」→「逐行累加（int）」⇒ A/B 两列从此读同一把尺：只在散文 /
    说明 / 清单 / 注释 / `echo` 文案里被点名的件，按两列都算「零引用」。
    """
    rx = bare_item_re(name)
    rx_call = call_re(name)
    rx_zerg = zerg_gate_re(name)
    stem = os.path.splitext(name)[0]
    n = 0
    for line in text.splitlines():
        # ★ 预筛放宽：`zerg gate check` 形里名字**不带扩展名**（行里无 name、只有去扩展名 stem）
        if name not in line and stem not in line:
            continue
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        if rx.match(s):
            continue
        if not (rx_call.search(line) or rx_zerg.search(line)):
            continue
        n += 1
    return n

def ref_counts(root, files, name, self_rel):
    """返回 (total, refs_code, refs_doc, sample_paths_sorted)。

    ★ 引用计数 = **命令位调用**（与断言 A 同口径 · 缺口 `GAP-20260927-148`）；
      旧口径「任一出现次数」（`text.count(name)`）会把记录面 / 散文 / 注释里的点名
      读成「有引用」⇒「可下架」名单静默失真（本缺口的症状）。
    """
    total = 0
    code = 0
    doc = 0
    samples = []
    for p in files:
        if p == self_rel:
            continue
        if p in SELF_FILES:
            continue
        if is_record_surface(p):
            continue
        t = read_text(os.path.join(root, p))
        if t is None:
            continue
        n = ref_cmd_hits(name, t)
        if n == 0:
            continue
        total += n
        if p.startswith("docs/") or "/docs/" in p:
            doc += n
        else:
            code += n
        samples.append("%s(x%d)" % (trunc(p, 52), n))
    return total, code, doc, sorted(samples, key=lambda s: (-dw(s), s))

def validate_whitelist(root, cand_names, wl):
    """返回 (errs, entries_by_name)。配置错一律 rc=2，不许静默。"""
    errs = []
    seen = {}
    for i, e in enumerate(wl, 1):
        n = (e.get("name") or "").strip()
        r = (e.get("reason") or "").strip()
        d = (e.get("date") or "").strip()
        if not n:
            errs.append("白名单第 %d 项缺 name" % i)
            continue
        if not r:
            errs.append("白名单「%s」缺 reason（登记必须说清它为什么还该在仓里）" % n)
        if not d:
            errs.append("白名单「%s」缺 date" % n)
        elif not re.match(r"^\d{4}-\d{2}-\d{2}$", d):
            errs.append("白名单「%s」的 date 形态不对（要 YYYY-MM-DD，现为 %r）" % (n, d))
        if n in seen:
            errs.append("白名单重名「%s」（第 %d 项与第 %d 项）" % (n, seen[n], i))
        seen[n] = i
        if n not in cand_names:
            errs.append("白名单**悬空**「%s」：scripts/ 顶层此刻没有这只脚本（删件后必须同步下架）" % n)
    return errs, seen

def judge_b(root, files, cands, wl_names):
    rows = []
    for c in cands:
        total, code, doc, samples = ref_counts(root, files, c["name"], c["path"])
        rows.append({"name": c["name"], "total": total, "code": code,
                     "doc": doc, "samples": samples,
                     "wl": c["name"] in wl_names})
    zero = [r for r in rows if r["total"] == 0]
    unregistered = [r for r in zero if not r["wl"]]
    registered = [r for r in zero if r["wl"]]
    # 已登记但已有引用 ⇒ 提示可下架（棘轮只许减）
    stale = [r for r in rows if r["wl"] and r["total"] > 0]
    doc_only = [r for r in rows if r["total"] > 0 and r["code"] == 0]
    return zero, unregistered, registered, stale, doc_only, rows

# ─────────────────────────────────────────────────────────────────────────────
# 报告
# ─────────────────────────────────────────────────────────────────────────────
def header(root, files, mode, n_scan):
    head = "?"
    dirty = "?"
    if os.path.isdir(os.path.join(root, ".git")):
        h = git_out(root, ["rev-parse", "--short", "HEAD"])
        head = (h or "").strip() or "?"
        s = git_out(root, ["status", "--porcelain"])
        dirty = len([x for x in (s or "").splitlines() if x.strip()])
    return head, dirty

def report(root, opts, files, mode, a_cands, b_cands, hits_a, wired_a, n_gate_present, missing_gates,
           zero, unreg, reg, stale, doc_only, rows, wl, errs, hits_c, c_rows, c_counts):
    out = []
    ts = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    head, dirty = header(root, files, mode, len(files))
    n_check = len([c for c in a_cands if c["kind"] == "命名 check-"])
    n_decl = len([c for c in a_cands if c["kind"] == "说明书自称门"])
    out.append("门③ check-wired-scripts — 脚本接线自检（挂闸档 `tri` + `--strict-report` ⇒ 阻断；不带开关时默认只报告）")
    out.append("")
    out.append("仓根   : %s" % root)
    out.append("快照   : %s ｜ HEAD %s ｜ 工作树改动 %s 件" % (ts, head, dirty))
    out.append("扫描面 : A 候选（门脚本）%d 只（命名 check- %d · 说明书自称门 %d）"
               % (len(a_cands), n_check, n_decl))
    out.append("         B 候选（scripts/ 顶层脚本）%d 只" % len(b_cands))
    out.append("         引用面 %d 个文件（%s；排除 自指 %d · 记录面 %d · 候选自身）"
               % (len(files), mode, len(SELF_FILES),
                  len([p for p in files if is_record_surface(p)])))
    out.append("闸清单 : 在位 %d / %d（提交闸 %d · 发布闸 %d）%s"
               % (n_gate_present, len(GATES),
                  len([g for g in GATES if g[1] == "commit"]),
                  len([g for g in GATES if g[1] == "release"]),
                  "" if not missing_gates else "  ⚠ 不在位：%s" % " · ".join(missing_gates)))
    for g, kind, label, how in GATES:
        mark = "✓" if os.path.isfile(os.path.join(root, g)) else "✗"
        out.append("         %s %-14s %s" % (mark, label, g))

    out.append("")
    out.append("【断言 A】门脚本必须出现在至少一个闸里 —— 命中 %d 只（未挂）" % len(hits_a))
    if not hits_a:
        out.append("  ✓ 无未挂门脚本")
    else:
        out.append("   %s %s %s %s %s" % (pad("#", 4), pad("脚本", 30), pad("类型", 14),
                                          pad("闸里按名调用", 12), "建议挂到哪里"))
        for i, c in enumerate(hits_a, 1):
            sug, why = SUGGEST.get(c["name"], ("未登记建议 ⇒ 人工判定（该补哪条闸由人定）", ""))
            out.append("   %s %s %s %s %s" % (pad(i, 4), pad(c["name"], 30),
                                              pad(c["kind"], 14), pad("0 处", 12),
                                              trunc(sug, 60)))
            if why:
                for j, seg in enumerate(re.split(r"；|；", why)):
                    out.append("       %s %s" % ("理由：" if j == 0 else "      ", trunc(seg, 96)))
    out.append("  已挂门脚本 %d 只（不打印；--list-rules 可看闸清单与调用方式）" % len(wired_a))

    out.append("")
    if opts.wired_only:
        out.append("【断言 B】**未判**（--wired-only：零引用普查**未执行**）—— 本档**不给出** B 的任何数字"
                   "（≠「零引用 0 · 未登记 0」）")
    else:
        out.append("【断言 B】零引用件必须登记白名单 —— 候选 %d 只 · 零引用 %d 只 · **未登记 %d 只**"
                   % (len(rows), len(zero), len(unreg)))
    if reg:
        out.append("  已登记（%d 只）：" % len(reg))
        by_name = {e["name"]: e for e in wl if e.get("name")}
        for r in reg:
            e = by_name.get(r["name"], {})
            out.append("    %s  date=%s" % (pad(r["name"], 26), e.get("date", "?")))
            out.append("      reason: %s" % trunc(e.get("reason", "?"), 200))
    if unreg:
        out.append("  ✗ 未登记（%d 只）—— 要么接线（被谁调用）、要么登记白名单（附理由）：" % len(unreg))
        for r in unreg:
            out.append("    %s  refs=0（引用面里零命中）" % pad(r["name"], 26))
    elif zero:
        out.append("  ✓ 零引用件全部已登记（%d/%d）" % (len(reg), len(zero)))
    if stale:
        out.append("  ⚠ 白名单可下架（已有引用 ⇒ 不再是死件；棘轮只许减）：%s"
                   % " · ".join("%s(refs=%d)" % (r["name"], r["total"]) for r in stale))
    if doc_only:
        out.append("  ⚠ 仅文档点名（total>0 但引用面里没有代码侧调用 ⇒ 不是死件、也不是活件；供人工判）："
                   "共 %d 只" % len(doc_only))
        for r in doc_only[:12]:
            out.append("    %s  refs=%d（文档 %d / 代码侧 %d）例：%s"
                       % (pad(r["name"], 26), r["total"], r["doc"], r["code"],
                          " · ".join(r["samples"][:3])))
        if len(doc_only) > 12:
            out.append("    …另有 %d 只同为「仅文档点名」（口径相同，为控篇幅不逐条打印）" % (len(doc_only) - 12))

    # ── 断言 C：件头自述三态（首版 UNSTATED-only）────────────────────────────────
    out.append("")
    cw = c_counts.get("WIRED", 0)
    cu = c_counts.get("UNWIRED", 0)
    cn = c_counts.get("UNSTATED", 0)
    out.append("【断言 C】件头自述三态（`# zerg-wire: WIRED|UNWIRED|UNSTATED` · 行首 · 件头前 %d 行 · 全件唯一）"
               "—— WIRED %d · UNWIRED %d · **UNSTATED %d**" % (WIRE_HEAD_LINES, cw, cu, cn))
    out.append("   ★ 首版 UNSTATED-only：UNSTATED 只打印+计数、**不判红**；真接线 = 命中闸清单命令位（同断言 A）")
    out.append("   ★ 失真（WIRED×未挂 ／ UNWIRED×已挂）计 hits_c，rc 同 A/B 受 --strict-report 门控（默认档不因此转红）")
    if hits_c:
        out.append("  ✗ 失真 %d 只（自称 ≠ 真接线）：" % len(hits_c))
        for name, declared, why in hits_c:
            out.append("    %s  自述=%s · %s" % (pad(name, 30), declared, why))
    else:
        out.append("  ✓ 无失真（已自述 %d 只均与真接线一致）" % (cw + cu))
    if cn:
        out.append("  · UNSTATED %d 只（不判红；逐件打印）：" % cn)
        for name, _d, _w, note in c_rows:
            if _d == "UNSTATED":
                out.append("    %s  %s" % (pad(name, 30), note))

    # 排除域（现读；项目口径：先出「哪些域没有任何门看着」再谈新门）
    out.append("")
    out.append("【排除域】不在本门判据里（现读，非记忆）")
    deep = sorted({"/".join(p.split("/")[:3]) for p in files
                   if p.startswith("scripts/") and p.count("/") > 3})
    for d in deep:
        n = len([p for p in files if p.startswith(d + "/")])
        out.append("  · 更深一层：%s/（%d 件，候选面只到 scripts/<域>/ 这一层）" % (d, n))
    for d in EXCL_SUBDIRS:
        n = len([p for p in files if p.startswith("scripts/%s/" % d)])
        if n:
            out.append("  · 排除域：scripts/%s/（%d 件，分域前就有的工作/实验目录，不进判据）" % (d, n))
    if os.path.isdir(os.path.join(root, ".git")):
        o = git_out(root, ["ls-files", "-o", "--exclude-standard", "-z", "--", "scripts"])
        untracked = [p for p in (o or "").split("\0") if p]
        if untracked:
            out.append("  · 未跟踪件 %d 件（未入库 ⇒ 不在判据内；入库后首跑会看到它）：%s"
                       % (len(untracked), " · ".join(trunc(p, 40) for p in untracked[:6])))
    for p, why in NOT_GATES:
        if os.path.isfile(os.path.join(root, p)):
            out.append("  · 相关但不是闸：%s（%s）" % (p, why))
    out.append("  · 候选面 = scripts/ **顶层 + 一层子目录**（scripts/<件> 与 scripts/<域>/<件>；"
               ".py/.sh/无后缀 shebang）；更深的树不在判据内")

    if errs:
        out.append("")
        out.append("【配置错】%d 条（⇒ rc=2，不给结论）" % len(errs))
        for e in errs:
            out.append("  ✗ %s" % e)

    out.append("")
    hit_n = len(hits_a) + len(unreg) + len(hits_c)
    out.append("【口径】引用 = 脚本名在引用面文件里出现；引用面 = 已跟踪文本文件 − 自指 2 件 − 记录面"
               "（现行债务台账，glob %s）− 候选自身。" % " · ".join(RECORD_SURFACES))
    out.append("        「记录不能充作引用」：台账点名不让死件复活（否则写下这条债就把债抹掉了）。")
    out.append("        「在闸里」= **命令位调用**（真接线面）：名字紧跟解释器/启动器（`python3`/`bash`/`${PY}`/`./`…），"
               "中间只允许选项旗标。")
    out.append("        不算接线的三类：纯注释行 · 纯清单项（`\"scripts/<…>/<名>\"`，**含被排除表/白名单点名**）"
               "· 只被文案/说明/grep 模式串提到。")
    out.append("")
    b_field = "未判（--wired-only）" if opts.wired_only else ("未登记 %d" % len(unreg))
    out.append("【结论】A 命中 %d · B %s · C 失真 %d · 配置错 %d" % (len(hits_a), b_field, len(hits_c), len(errs)))
    if errs:
        out.append("        rc=2（缺件/配置错/空转 ⇒ 不给结论，不许静默）")
    elif opts.strict_report and hit_n:
        out.append("        --strict-report 且命中 %d ⇒ rc=1" % hit_n)
    elif opts.strict_report:
        out.append("        阻断档（`--strict-report`）且**零命中** ⇒ rc=0"
                   "（挂闸档：此后新增一只未挂/未登记即 rc=1）")
    else:
        out.append("        只报告档 ⇒ rc=0（命中不阻断；挂闸时用 --strict-report 或按 .md §五 走棘轮）")
    out.append("升阻断：**已执行**（2026-09-18 ③批次 · 提交闸 `gates` scope · `tri` + `--strict-report`）"
               "—— 存量 A 命中 8→0 后升档，此后新增一只未挂/未登记即红；全过程见 "
               "scripts/gates/check-wired-scripts.md §五")
    return "\n".join(out)

# ─────────────────────────────────────────────────────────────────────────────
# --list-rules
# ─────────────────────────────────────────────────────────────────────────────
def list_rules():
    L = []
    L.append("门③ check-wired-scripts — 判据 / 闸清单 / 口径（--list-rules）")
    L.append("")
    L.append("断言 A（门脚本必须上岗）：凡门脚本 —— ① 名以 `check-` 开头（scripts/ 顶层 · .py/.sh/无后缀 shebang）"
             "或 ② 说明书自称门（同干 .md 头部 %d 行内出现「门」）—— 必须出现在至少一个闸里。" % DOC_HEAD_LINES)
    L.append("  「在闸里」判据 = **命令位调用**（真接线面）：名字紧跟解释器/启动器"
             "（`python3` / `bash` / `${PY}` / `./` …），中间只允许选项旗标；"
             "可带引号与路径前缀（`python3 \"$Z/scripts/gates/<名>\" \"$D\"`）。")
    L.append("    不算接线：① 纯注释行（`#` 开头）② 纯清单项（整行就是 `\"scripts/<…>/<名>\",` 这类数组/数据项"
             "—— **含被 `EXCLUDES` / `DROP_EXACT` 这类排除表点名**，任意层深）③ 只被文案（`echo \"…\"`）/ "
             "说明文字 / grep 模式串提到。")
    L.append("断言 B（零引用件必须登记）：scripts/ 顶层零引用（引用面里**命令位调用** 0 次 · 与断言 A **同口径** ·"
             " 缺口 `GAP-20260927-148`）⇒ 必须登记白名单"
             "（每项 name + reason + date；缺一即 rc=2；点名的脚本已不存在即 rc=2 悬空）。")
    L.append("")
    L.append("闸清单（判据只吃这份表；实际文件名与调用方式 = 2026-09-18 现读）：")
    for g, kind, label, how in GATES:
        L.append("  [%s] %-36s %s" % ("提交" if kind == "commit" else "发布", g, label))
        L.append("       调用方式：%s" % how)
    L.append("相关但不是闸（不进判据）：")
    for p, why in NOT_GATES:
        L.append("  · %-36s %s" % (p, why))
    L.append("")
    L.append("扫描面与排除域：")
    L.append("  · 候选 = scripts/ 顶层 `scripts/<件>` + 域一层 `scripts/<域>/<件>`（域 = 除 %s 外的子目录）"
             "；排除域如 scripts/exportnames/ 只打印不进候选面" % " · ".join(EXCL_SUBDIRS))
    L.append("  · 引用面 = git 索引里的已跟踪文件（`git ls-files`）；未跟踪件不在判据内")
    L.append("  · 排除 ① 本门自指 %s（否则我自己的名单会把零引用写成有引用）" % " · ".join(SELF_FILES))
    L.append("  · 排除 ② 记录面 glob：%s —— 「记录不能充作引用」" % " · ".join(RECORD_SURFACES))
    L.append("  · 排除 ③ 候选脚本自身")
    L.append("")
    L.append("白名单（%d 条 · 只许减不许增；有引用后提示可下架）：" % len(WHITELIST))
    for e in WHITELIST:
        L.append("  · %-24s date=%s  reason=%s" % (e.get("name"), e.get("date"),
                                                   trunc(e.get("reason", ""), 80)))
    L.append("")
    L.append("退码：0 = 只报告档正常出口（命中不阻断）· 1 = 仅 --strict-report 且有命中 ·"
             " 2 = 缺件/配置错/空转/自检不过（不给结论）")
    L.append("升级路径（.md §五 · **已执行 2026-09-18 ③批次**）：清存量债 → 记基线快照（逐文件计数）→ "
             "--strict-report 挂进闸（新增一条即红）→ 只许减不许增，每还掉 N 条下调一格 → "
             "降到 0 转完全阻断、基线表退役。")
    L.append("  现状：8 只未挂门脚本逐只上岗 ⇒ A 命中 8→0 · B 未登记 0 ⇒ 已挂 `tri` + `--strict-report`（阻断档）。")
    L.append("  ★ 与第 2 步的差异（如实登记）：**基线快照表未建** —— 存量是**当批清到 0** 而不是逐格下调，")
    L.append("    正落在 §五 第 5 步「A 命中 0 且 B 未登记 0 ⇒ 转完全阻断、撤掉基线表」那一格，故不需要棘轮表。")
    L.append("  ★ 阻断面由**闸的步骤**给（模式 `tri` + 开关 `--strict-report`）；本脚本的默认档位**未改**")
    L.append("    （默认仍只报告 ⇒ 手工跑一下不会突然拦人），谁摘掉那个开关就会退化成恒绿步（自检 ⑩ 已钉住）。")
    return "\n".join(L)

# ─────────────────────────────────────────────────────────────────────────────
# 自检（成对负控）
# ─────────────────────────────────────────────────────────────────────────────
def _fixture(base, name, files):
    d = os.path.join(base, name)
    for relp, content in files.items():
        p = os.path.join(d, relp)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as f:
            f.write(content)
    return d


def _write(path, content):
    """夹具写盘助手（修复：自检 ⑰ 用它写未跟踪件，此前未定义 ⇒ 自检 NameError 崩）。"""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)

GATE_A = "#!/usr/bin/env bash\nset -u\n# 闸（夹具）\npython3 scripts/check-alpha.py --scope x\n"
GATE_B = "#!/usr/bin/env bash\npython3 scripts/check-beta.py\n"

def run_case(label, fixture, wl, extra, expect_rc, must, must_not, results):
    cmd = [sys.executable, os.path.abspath(__file__), "--root", fixture, "--no-self-test"]
    if wl is not None:
        # 白名单 JSON 写在**夹具之外**（父目录）：写在夹具里会被算成一条引用，
        # 把「零引用」的期望值搅乱（踩过一次）
        p = os.path.join(os.path.dirname(fixture), "_wl_%s.json" % os.path.basename(fixture))
        with open(p, "w", encoding="utf-8") as f:
            json.dump(wl, f, ensure_ascii=False)
        cmd += ["--whitelist", p]
    cmd += extra
    r = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    out = r.stdout.decode("utf-8", "replace")
    ok = (r.returncode == expect_rc)
    bad = []
    if not ok:
        bad.append("rc=%d（期望 %d）" % (r.returncode, expect_rc))
    for m in must:
        if m not in out:
            bad.append("缺 %r" % m)
    for m in must_not:
        if m in out:
            bad.append("不该出现 %r" % m)
    results.append((label, not bad, "; ".join(bad)))
    return out

def selftest():
    base = tempfile.mkdtemp(prefix="wired-scripts-selftest-")
    results = []

    def check(label, cond):
        """单元负控登记（修复：⑱ 用到 check，此前未定义 ⇒ 自检 NameError 崩）。"""
        results.append((label, bool(cond), "" if cond else "断言不成立"))
    wl_ok = [{"name": "helper.py", "reason": "夹具：一次性", "date": "2026-09-18"}]

    ok_files = {
        "scripts/gates/precommit-gates.sh": GATE_A,
        "scripts/build/publish-preflight.sh": GATE_B,
        "scripts/check-alpha.py": "# ck\n",
        "scripts/check-beta.py": "# ck\n",
        "scripts/helper.py": "# zero-ref but whitelisted\n",
        # 让两只闸自身有「代码侧引用」（否则它们自己是零引用件，会污染 B 的期望值）
        "scripts/run-gates.sh": "#!/usr/bin/env bash\n"
                                "bash scripts/gates/precommit-gates.sh\n"
                                "bash scripts/build/publish-preflight.sh /tmp/out\n",
        "Makefile": "check:\n\tbash scripts/run-gates.sh\n",
        # 闸清单里的每一条都必须「在位」——否则 GAP-154 起 rc=2（缺一即不给结论）。
        # 旧夹具只放了 2 条闸 ⇒ 修好 NameError 后仍会 rc=2，故补齐其余发布闸（内容无关，只求在位）。
        "publish/mirror-public.sh": "#!/usr/bin/env bash\n",
        "publish/mirror-public-lib.py": "# lib\n",
        "publish/ci/ci.yml": "name: ci\n",
        "publish/ci/release-agent.yml": "name: rel\n",
        "publish/mirror-acceptance.sh": "#!/usr/bin/env bash\n",
        # 记录面（本门结论的落账面）：点名 helper.py **不算引用** ⇒ helper.py 仍应是零引用件
        "docs/项目文档/v9.9/债务台账-20990101.md": "helper.py 是零引用\n",
    }
    f_ok = _fixture(base, "ok", ok_files)

    # ① 正控：全挂 + 白名单齐 ⇒ 0 命中 ⇒ rc=0
    run_case("① 正控：全挂且白名单齐 ⇒ rc=0 且 0 命中", f_ok, wl_ok, [],
             0, ["A 命中 0", "未登记 0"], [], results)

    # ② 正控（判据区分度）：记录面点名不算引用 ⇒ helper.py 仍是零引用（靠白名单不命中证明）
    out = run_case("② 记录面不计引用（判据区分度）⇒ 零引用 1 只且已登记", f_ok, wl_ok, [],
                   0, ["零引用 1 只", "已登记（1 只）"], ["零引用 0 只"], results)

    # ③ 负控 A：拿掉一条接线 ⇒ 必须报出来（只报告档仍 rc=0）
    f_noa = _fixture(base, "neg-a", dict(ok_files, **{
        "scripts/gates/precommit-gates.sh": "#!/usr/bin/env bash\nset -u\necho 没有按名调用\n"}))
    run_case("③ 负控 A：拿掉 check-alpha 的接线 ⇒ 报出它，只报告档 rc=0",
             f_noa, wl_ok, [], 0, ["check-alpha.py", "A 命中 1"], [], results)

    # ④ 负控 A':同一夹具 + --strict-report ⇒ rc=1
    run_case("④ 负控 A'：同一夹具 + --strict-report ⇒ rc=1",
             f_noa, wl_ok, ["--strict-report"], 1, ["rc=1"], [], results)

    # ⑤ 负控 A''：注释/清单项不算上岗（有牙齿的那一条）
    f_cmt = _fixture(base, "neg-a-cmt", dict(ok_files, **{
        "scripts/check-gamma.py": "# ck\n",
        "scripts/gates/precommit-gates.sh": GATE_A + "# python3 scripts/check-gamma.py\n"
            "EXCLUDES=(\n  \"scripts/check-gamma.py\"\n)\n"}))
    run_case("⑤ 负控 A''：只在注释行/纯清单项里出现 ⇒ 仍算未挂（判据有牙齿）",
             f_cmt, wl_ok, [], 0, ["check-gamma.py", "A 命中 1"], [], results)

    # ⑥ 负控 B：零引用件不在白名单（且无引用）⇒ 必须报出来
    run_case("⑥ 负控 B：零引用且未登记 ⇒ 报出它（只报告档 rc=0）",
             f_ok, [], [], 0, ["未登记 1", "helper.py"], [], results)

    # ⑦ 负控 B'：同上 + --strict-report ⇒ rc=1
    run_case("⑦ 负控 B'：同夹具 + --strict-report ⇒ rc=1",
             f_ok, [], ["--strict-report"], 1, ["rc=1"], [], results)

    # ⑧ 配置错：白名单缺 reason / 缺 date / date 形态不对 / 悬空 ⇒ 一律 rc=2
    run_case("⑧a 白名单缺 reason ⇒ rc=2", f_ok,
             [{"name": "helper.py", "date": "2026-09-18"}], [], 2, ["缺 reason"], [], results)
    run_case("⑧b 白名单缺 date ⇒ rc=2", f_ok,
             [{"name": "helper.py", "reason": "夹具"}], [], 2, ["缺 date"], [], results)
    run_case("⑧c 白名单 date 形态不对 ⇒ rc=2", f_ok,
             [{"name": "helper.py", "reason": "夹具", "date": "09/18/2026"}], [], 2,
             ["date 形态不对"], [], results)
    run_case("⑧d 白名单悬空（点名的脚本不存在）⇒ rc=2", f_ok,
             wl_ok + [{"name": "ghost.py", "reason": "夹具", "date": "2026-09-18"}], [], 2,
             ["悬空"], [], results)

    # ⑨ 空转：没有候选 ⇒ rc=2（不许静默给绿）
    #   ★ 2026-09-20（脚本分域第二段）：候选面下沉到 `scripts/<域>/` 后，闸文件自己
    #   （scripts/gates/precommit-gates.sh）**也成了候选** ⇒ 旧的「只放一只闸、零候选」夹具
    #   不再空转。改用**闸清单里不在 scripts/ 下**的那条（publish/mirror-public.sh）——
    #   闸在位（不是「缺件」），而 scripts/ 下真的一只脚本都没有 ⇒ 仍是干净的空转夹具。
    f_empty = _fixture(base, "empty", {"publish/mirror-public.sh": "#!/usr/bin/env bash\n",
                                       "scripts/tool-x.md": "夹具占位：scripts/ 目录在、里面一只**脚本**都没有"
                                                            "（.md 不进候选面）\n"})
    run_case("⑨a 空转：候选 0 只 ⇒ rc=2", f_empty, [], [], 2, ["空转", "rc=2"], [], results)

    # ⑩ 缺件：一个闸都不在位 ⇒ rc=2（判据无依据）
    f_nogate = _fixture(base, "no-gate", {
        "scripts/check-solo.py": "# ck\n",
        "README.md": "x\n",
    })
    run_case("⑩ 缺件：闸清单一个都不在位 ⇒ rc=2（不给结论）", f_nogate, [], [], 2,
             ["不给结论", "rc=2"], [], results)

    # ⑩b 负控（与 ⑩ 成对 · `GAP-20260927-154`）：闸清单**只缺一条**（非全缺）⇒ 也必须 rc=2。
    #   旧口径 `n_gate_present == 0` ⇒ 缺 1~5 条恒绿（改名/搬走一条闸即静默）⇒ 这条正是它的牙。
    f_partial = _fixture(base, "one-gate-missing", {
        "scripts/gates/precommit-gates.sh": GATE_A,
        "scripts/build/publish-preflight.sh": GATE_B,
        "publish/mirror-public.sh": "#!/usr/bin/env bash\n",
        "publish/mirror-public-lib.py": "# lib\n",
        "publish/ci/ci.yml": "name: ci\n",
        "scripts/check-solo.py": "# ck\n",
    })
    run_case("⑩b 闸清单只缺 1 条（非全缺）⇒ 必 rc=2 并点名缺件", f_partial, [], [], 2,
             ["不给结论", "缺件", "release-agent.yml"], [], results)

    # ⑪ 说明书自称门：同干 .md 头部含「门」⇒ 该脚本成为候选
    f_decl = _fixture(base, "decl", dict(ok_files, **{
        "scripts/tool-x.sh": "#!/usr/bin/env bash\necho x\n",
        "scripts/tool-x.md": "# tool-x — 本门说明书\n\n写盘门。\n",
    }))
    run_case("⑪ 说明书自称门（同干 .md 头部含「门」）⇒ 成为候选并报未挂",
             f_decl, wl_ok, [], 0, ["tool-x.sh", "说明书自称门"], [], results)

    # ⑫ 说明书不提门 ⇒ 不成为候选（区分度）；给它一条代码侧引用免得它落进 B 清单
    f_nodecl = _fixture(base, "nodecl", dict(ok_files, **{
        "scripts/tool-y.sh": "#!/usr/bin/env bash\necho y\n",
        "scripts/tool-y.md": "# tool-y —— 一个普通工具\n\n没什么特别的。\n",
        "scripts/run-gates.sh": "#!/usr/bin/env bash\n"
                                "bash scripts/tool-y.sh\n"
                                "bash scripts/gates/precommit-gates.sh\n"
                                "bash scripts/build/publish-preflight.sh /tmp/out\n",
    }))
    run_case("⑫ 说明书不提「门」⇒ 不成为候选（区分度）",
             f_nodecl, wl_ok, [], 0, ["A 命中 0"], ["tool-y.sh"], results)

    # ⑬ --list-rules 可用且含闸清单/升级路径
    r = subprocess.run([sys.executable, os.path.abspath(__file__), "--list-rules"],
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    o = r.stdout.decode("utf-8", "replace")
    bad = []
    if r.returncode != 0:
        bad.append("rc=%d" % r.returncode)
    for m in ["断言 A", "断言 B", "precommit-gates.sh", "publish-preflight.sh", "升级路径", "白名单"]:
        if m not in o:
            bad.append("缺 %r" % m)
    results.append(("⑬ --list-rules 可打印判据/闸清单/升级路径", not bad, "; ".join(bad)))

    # ── ⑭~⑯ 2026-09-26 判据收紧（「被排除表点名 ≠ 接线」）的成对负控 ────────────────
    # ⑭ 负控 A3（**病根回归**）：只被一张**排除清单**点名（`scripts/gates/…` 多段路径的纯清单项）
    #     —— 旧判据的「纯清单项」正则只认 `scripts/<名>` 两段 ⇒ 分域后当场失效、把清单项读成接线。
    f_li = _fixture(base, "neg-a-list", dict(ok_files, **{
        "scripts/gates/check-delta.py": "# ck\n",
        "publish/mirror-public-lib.py":
            'DROP_EXACT = (\n    "scripts/gates/check-delta.py",   # 公开面排除表\n)\n',
    }))
    run_case("⑭ 负控 A3: 只被排除表清单项点名（多段路径）⇒ 仍算未挂（判据有牙齿）",
             f_li, wl_ok, [], 0, ["check-delta.py", "A 命中 1"], [], results)

    # ⑮ 正控（真接线面）：命令位调用（带引号 + 变量前缀）⇒ 已挂；同文件里 echo 文案提到的不算
    f_call = _fixture(base, "pos-call", dict(ok_files, **{
        "scripts/gates/check-epsilon.py": "# ck\n",
        "scripts/gates/precommit-gates.sh": GATE_A +
            'python3 "$Z/scripts/gates/check-epsilon.py" "$D" > "$D/x.log" 2>&1\n'
            'echo "   ⑧c ── 已接线（按名调用 check-epsilon.py）"\n',
    }))
    run_case("⑮ 正控：命令位调用（引号 + 变量前缀）⇒ 已挂（A 命中 0）",
             f_call, wl_ok, [], 0, ["A 命中 0", "已挂门脚本 3 只"], ["0 处"], results)

    # ⑯ 负控 A4：只在**文案**里被提到（0 次命令位调用）⇒ 仍算未挂
    f_echo = _fixture(base, "neg-a-echo", dict(ok_files, **{
        "scripts/gates/check-zeta.py": "# ck\n",
        "scripts/gates/precommit-gates.sh": GATE_A +
            'echo "   ⑧d 成对闸 ── 按名调用 check-zeta.py · 明细 /tmp/x.log"\n',
    }))
    run_case("⑯ 负控 A4: 只在文案里被提到（0 次命令位调用）⇒ 仍算未挂",
             f_echo, wl_ok, [], 0, ["check-zeta.py", "A 命中 1"], [], results)

    # ── ⑰/⑱ 候选面换 fs、引用面不换（`GAP-20260927-159/169`）成对负控 ────────────────
    #   ⑰ 正控：git 夹具 + 一个**未跟踪**的新 check- 脚本 ⇒ 必进候选面并被断言 A 报出。
    #      旧口径（候选面 = 索引）⇒ 看不到它 ⇒ A 命中 0（恒静默）。这条是它的牙。
    #      （若环境无 git，list_files 退 fs 遍历 ⇒ ⑰ 也会过：不会假红，只会不覆盖回归。）
    f_unt = _fixture(base, "untracked-cand", dict(ok_files))
    subprocess.run(["git", "init", "-q"], cwd=f_unt,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(["git", "add", "-A"], cwd=f_unt,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    _write(os.path.join(f_unt, "scripts", "check-untracked.py"), "# ck（未跟踪新件）\n")
    run_case("⑰ 未跟踪的新门脚本必进候选面（候选面换 fs · 索引只分档）",
             f_unt, wl_ok, [], 0, ["check-untracked.py", "A 命中 1"], ["A 命中 0"], results)

    #   ⑱ 单元负控（与 ⑰ 成对 · 方向性）：候选面函数在**非 git 档**原样返回（候选面 = 引用面），
    #      证明「换面」只写在 git 分支里、没有顺手把引用面也换掉（在途件洗白 = 松判）。
    check("⑱ candidate_files 非 git 档原样返回（换面只在候选面这一半）",
          candidate_files(base, ["a.py"], "文件系统遍历（非 git 目录）") == ["a.py"])

    # ── ⑲ 记录类表不得充作引用（`GAP-20260927-139` · 与 RECORD_SURFACES 新增条目成对）─────
    #   负控：一只假脚本（check-omega.py）**只被** `scripts/公开标记.tsv`（记录类表面）点名
    #   ⇒ 必须仍判「未登记」（若该表被算作引用，check-omega.py 就有引用 ⇒ 不报未登记 ⇒ 本格红）。
    f_mark = _fixture(base, "neg-record-marks", dict(ok_files, **{
        "scripts/check-omega.py": "# ck\n",
        "scripts/公开标记.tsv": "scripts/check-omega.py\tpublic=no\n",
    }))
    run_case("⑲ 只被记录类表（公开标记.tsv）点名 ⇒ 仍判「未登记」（记录不能充作引用）",
             f_mark, wl_ok, [], 0, ["check-omega.py", "未登记 1"], ["未登记 0"], results)
    #   正控（成对 · 反向）：同夹具 + --strict-report ⇒ 该「未登记」必须升为 rc=1（牙真的在）
    run_case("⑲' 同上 + --strict-report ⇒ rc=1（记录类引用不成牙）",
             f_mark, wl_ok, ["--strict-report"], 1, ["rc=1"], [], results)

    # ── ⑳/㉑ `zerg gate check <判据名>` 接线形（`GAP-20261001-74` · 与 zerg_gate_re 成对）──
    #   ★ 光加 zerg 入 RUNNER_RE 不解：`gate check` 两词夹在启动器与名之间 ⇒ 必须专用规则。
    #   ⑳ 正控：去扩展名形 `zerg gate check check-eta`（命令位）⇒ 已挂（A 命中 0）。
    f_zg = _fixture(base, "pos-zerg", dict(ok_files, **{
        "scripts/gates/check-eta.py": "# ck\n",
        "scripts/gates/precommit-gates.sh": GATE_A +
            'add_step g "门X" tri "$R" "zerg gate check check-eta"\n',
        # 让 check-eta.py 有「代码侧引用」（免得它落 断言 B 零引用件、污染本格 must_not）
        "scripts/run-gates.sh": "#!/usr/bin/env bash\n"
                                "bash scripts/gates/precommit-gates.sh\n"
                                "bash scripts/build/publish-preflight.sh /tmp/out\n"
                                "python3 scripts/gates/check-eta.py\n",
    }))
    run_case("⑳ 正控：`zerg gate check check-eta` 命令位 ⇒ 已挂（A 命中 0）",
             f_zg, wl_ok, [], 0, ["A 命中 0", "已挂门脚本 3 只"], ["0 处"], results)
    #   ㉑ 负控（与 ⑳ 成对 · 方向性）：`zerg check check-eta`（缺 `gate` 子命令）⇒ 不命中
    f_ng = _fixture(base, "neg-zerg-subcmd", dict(ok_files, **{
        "scripts/gates/check-eta.py": "# ck\n",
        "scripts/gates/precommit-gates.sh": GATE_A +
            'add_step g "门X" tri "$R" "zerg check check-eta"\n',
        "scripts/run-gates.sh": "#!/usr/bin/env bash\n"
                                "bash scripts/gates/precommit-gates.sh\n"
                                "bash scripts/build/publish-preflight.sh /tmp/out\n"
                                "python3 scripts/gates/check-eta.py\n",
    }))
    run_case("㉑ 负控：`zerg check check-eta`（缺 gate 子命令）⇒ 仍算未挂（防「只加 zerg 就放行」）",
             f_ng, wl_ok, [], 0, ["check-eta.py", "A 命中 1"], [], results)

    # ── 断言 C：件头自述三态（首版 UNSTATED-only · 显式 token `# zerg-wire:`）成对负控 ──
    #   ★ token 形写死：行首 `#` ＋ `zerg-wire:` ＋ <WIRED|UNWIRED|UNSTATED>；件头前 40 行、全件唯一。
    #   C1（= M2 方向）：件头 `WIRED` 但闸里查无此件 ⇒ **失真**（默认档只报，--strict-report 下 rc=1）。
    f_c1 = _fixture(base, "c-wired-notwired", dict(ok_files, **{
        "scripts/gates/check-iota.py": "# zerg-wire: WIRED\n# ck\n",
        "scripts/run-gates.sh": "#!/usr/bin/env bash\n"
                                "bash scripts/gates/precommit-gates.sh\n"
                                "bash scripts/build/publish-preflight.sh /tmp/out\n"
                                "python3 scripts/gates/check-iota.py\n",
    }))
    run_case("C1 负控（M2 方向）：件头 WIRED 但闸清单查无此件 ⇒ 报失真（默认档 rc=0 不转红）",
             f_c1, wl_ok, [], 0, ["失真 1", "check-iota.py"], [], results)
    run_case("C1' 同上 + --strict-report ⇒ rc=1（失真成牙）",
             f_c1, wl_ok, ["--strict-report"], 1, ["rc=1"], [], results)

    #   C2（= M1 方向 · 与 C1 反向）：件头 `UNWIRED` 但闸里**已**按名调用 ⇒ **失真**（A 命中 0，
    #     这正是断言 C 独有、断言 A 结构上抓不到的一档）。
    f_c2 = _fixture(base, "c-unwired-butwired", dict(ok_files, **{
        "scripts/gates/check-kappa.py": "# zerg-wire: UNWIRED\n# ck\n",
        "scripts/gates/precommit-gates.sh": GATE_A + "python3 scripts/gates/check-kappa.py\n",
    }))
    run_case("C2 负控（M1 方向）：件头 UNWIRED 但闸里已按名调用 ⇒ 报失真（A 命中 0 · 默认档 rc=0）",
             f_c2, wl_ok, [], 0, ["失真 1", "check-kappa.py", "A 命中 0"], [], results)
    run_case("C2' 同上 + --strict-report ⇒ rc=1",
             f_c2, wl_ok, ["--strict-report"], 1, ["rc=1"], [], results)

    #   C3（= M3）：件头**无** token ⇒ 只计入 UNSTATED、**不判红**（默认档与 --strict-report 均 rc=0）。
    #     件**已**接线（闸里按名调用、A 命中 0）—— 这样 --strict-report 下 rc 只由 C 决定，才测得准。
    f_c3 = _fixture(base, "c-unstated", dict(ok_files, **{
        "scripts/gates/check-lambda.py": "# ck\n",
        "scripts/gates/precommit-gates.sh": GATE_A + "python3 scripts/gates/check-lambda.py\n",
    }))
    run_case("C3 正控（M3）：件头无 token ⇒ 只计入 UNSTATED，不判红（默认档 rc=0）",
             f_c3, wl_ok, [], 0, ["UNSTATED 3", "check-lambda.py", "无失真"], [], results)
    run_case("C3' 同上 + --strict-report ⇒ 仍 rc=0（UNSTATED 不锁门）",
             f_c3, wl_ok, ["--strict-report"], 0, ["rc=0"], [], results)

    #   C4（= M4 歧义）：件头两行 token（WIRED + UNWIRED）⇒ 判 UNSTATED 并打印（✗ 不猜）。
    f_c4 = _fixture(base, "c-ambiguous", dict(ok_files, **{
        "scripts/gates/check-mu.py": "# zerg-wire: WIRED\n# zerg-wire: UNWIRED\n# ck\n",
        "scripts/run-gates.sh": "#!/usr/bin/env bash\n"
                                "bash scripts/gates/precommit-gates.sh\n"
                                "bash scripts/build/publish-preflight.sh /tmp/out\n"
                                "python3 scripts/gates/check-mu.py\n",
    }))
    run_case("C4 负控（M4 歧义）：件头两行 token ⇒ 判 UNSTATED 并打印（✗ 不猜 · 失真 0）",
             f_c4, wl_ok, [], 0, ["token 多重", "check-mu.py", "无失真"], [], results)

    #   C5（= M5 词面碰撞回归）：件头含自然语言 `不挂死` / `只报告档起步` 但**无** token ⇒ UNSTATED
    #     （专治设计稿 §1.6 噪声：按词面抽会把 `不挂死` 读成「未接线」）。
    f_c5 = _fixture(base, "c-collision", dict(ok_files, **{
        "scripts/gates/check-nu.py": "# ck\n"
                                     "# 本件不挂死：无器件即立即失败、不卡住\n"
                                     "# 起步只报告档起步（历史档位，与「未接线」无关）\n",
        "scripts/run-gates.sh": "#!/usr/bin/env bash\n"
                                "bash scripts/gates/precommit-gates.sh\n"
                                "bash scripts/build/publish-preflight.sh /tmp/out\n"
                                "python3 scripts/gates/check-nu.py\n",
    }))
    run_case("C5 负控（M5 词面碰撞回归）：件头含 `不挂死`/`只报告档起步` 但无 token ⇒ UNSTATED（✗ 不被读成 UNWIRED）",
             f_c5, wl_ok, [], 0, ["UNSTATED 3", "check-nu.py"], ["UNWIRED 1"], results)

    npass = sum(1 for _lbl, _ok, _w in results if _ok)
    print("门③ --self-test（成对负控 · 真命令行 + 真退出码）：%d/%d 通过" % (npass, len(results)))
    for label, ok, why in results:
        print("  %s %s%s" % ("✓" if ok else "✗", label, "" if ok else "   ← %s" % why))
    print("  夹具目录：%s" % base)
    if npass != len(results):
        print("  ✗ 自检不过 ⇒ 拒绝扫真目标（rc=2）")
        return 2
    return 0

# ─────────────────────────────────────────────────────────────────────────────
# main
# ─────────────────────────────────────────────────────────────────────────────
def main(argv=None):
    ap = argparse.ArgumentParser(
        description="门③ check-wired-scripts — 脚本接线自检（只报告档）",
        formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--strict-report", action="store_true",
                    help="有命中即 rc=1（默认：只报告 ⇒ rc=0）")
    ap.add_argument("--wired-only", dest="wired_only", action="store_true",
                    help="只判断言 A/C，跳过零引用普查（断言 B）—— B 未判，报告里明写（✗ 不把空表印成「零引用 0 · 未登记 0」）")
    ap.add_argument("--list-rules", action="store_true", help="打判据/闸清单/口径/升级路径，然后退出")
    ap.add_argument("--self-test", dest="self_test", action="store_true", help="只跑自检")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（自检内部的子进程用，防递归）")
    ap.add_argument("--root", default=None, help="仓根（默认 = 本脚本上一级；自检夹具用）")
    ap.add_argument("--whitelist", default=None, help="白名单 JSON 覆盖（自检/临时用；默认用内置表）")
    opts = ap.parse_args(argv)

    if opts.list_rules:
        print(list_rules())
        return 0
    if opts.self_test:
        return selftest()

    root = os.path.abspath(opts.root or os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    if not os.path.isdir(os.path.join(root, "scripts")):
        print("FATAL: 仓根缺 scripts/ 目录（root=%s）⇒ rc=2（不给结论）" % root)
        return 2

    if not opts.no_self_test:
        rc_st = selftest()
        print("")
        if rc_st != 0:
            return 2

    files, mode = list_files(root)
    if not files:
        print("FATAL: 引用面 0 个文件（空转）⇒ rc=2")
        return 2
    cand_files = candidate_files(root, files, mode)   # 候选面可换 fs；引用面仍用 files（索引）
    cands, _notes = build_candidates(root, cand_files)
    if not cands:
        print("FATAL: scripts/ 顶层脚本候选 0 只（空转 ⇒ 不给结论）⇒ rc=2")
        return 2
    a_cands = [c for c in cands if c["kind"]]
    b_cands = cands
    if not a_cands:
        print("FATAL: 门脚本候选 0 只（断言 A 无对象 ⇒ 空转，不给结论）⇒ rc=2")
        return 2

    wl = WHITELIST
    if opts.whitelist:
        try:
            with open(opts.whitelist, encoding="utf-8") as f:
                wl = json.load(f)
            if not isinstance(wl, list):
                print("FATAL: --whitelist 必须是 JSON 数组 ⇒ rc=2")
                return 2
        except Exception as e:
            print("FATAL: 读 --whitelist 失败：%s ⇒ rc=2" % e)
            return 2

    hits_a, wired_a, n_gate_present, missing_gates = judge_a(root, a_cands)
    if missing_gates:  # ★ GAP-20260927-154：缺一条即不给结论（旧口径「全缺才 rc=2」⇒ 改名/搬走 1~5 条恒绿）
        print("FATAL: 闸清单有 %d 个**缺件**（判据无依据 ⇒ 不给结论）⇒ rc=2" % len(missing_gates))
        for g in missing_gates:
            print("  缺：%s" % g)
        return 2

    errs, wl_names = validate_whitelist(root, {c["name"] for c in b_cands}, wl)
    if opts.wired_only:
        # ★ --wired-only：只判断言 A/C，**跳过**零引用普查（断言 B 的贵腿 judge_b）—— B 面在本档**未判**：
        #   报告里明写「未判」，✗ 不把空表印成「零引用 0 · 未登记 0」（那是假绿）。
        zero, unreg, reg, stale, doc_only, rows = [], [], [], [], [], []
    else:
        zero, unreg, reg, stale, doc_only, rows = judge_b(root, files, b_cands, wl_names)

    wired_names = {c["name"] for (c, _g, _ev) in wired_a}
    hits_c, c_rows, c_counts = judge_c(root, a_cands, wired_names)

    print(report(root, opts, files, mode, a_cands, b_cands, hits_a, wired_a, n_gate_present,
                 missing_gates, zero, unreg, reg, stale, doc_only, rows, wl, errs,
                 hits_c, c_rows, c_counts))
    if errs:
        return 2
    if opts.strict_report and (hits_a or unreg or hits_c):
        return 1
    return 0

if __name__ == "__main__":
    sys.exit(main())
