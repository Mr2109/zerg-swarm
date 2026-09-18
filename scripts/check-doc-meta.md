# check-doc-meta — 文档 frontmatter 元数据门（6 件套必填 + 值域封闭 + 额外字段禁入）

> **规格（唯一真源，脚本不复制其判定）**
> 设计稿 `docs/01-设计/设计-文档体系-v1.0.md`（v1.2 稿）：**§3.4 ①** type 值域封闭六值 + 三条可机器判的判据 ·
> **§3.4 ②** frontmatter **6 件套**必填集与三条硬规矩（未申报字段 = 错 · 用 JSON Schema · type/status 正交）·
> **§3.4 ④** `title` ≤ 60 字（本仓标题 p99 = 59）· **§5.6 ②** 双语字段 · **附录B ⑲** `status` 四值信任层级 ·
> **附录B ⑮/㉔** 行式键值 / `ai_assisted: human|ai|mixed`。
> 调研稿 `docs/调研/调研-文档分块落地-分面与规范.md`：**§B1.3** 推荐字段集（`reviewed_at`：`authoritative` 时必填）·
> **§B2** type 值域与「目录 → type 默认值」映射 · **§A1.3 ③** 回填期口径「**字段不合法即红、缺失只告警**」。
> **值域与字段一律从 `docs/site/frontmatter-schema.json` 读出** —— 改值域只改那一份，脚本不必动。

## 三段（顺序写死）

```
① 读 schema（缺件/非法 JSON/用了未实现的关键字 ⇒ rc=2，不给结论）
② 逐篇：取 frontmatter 块 → 严格子集解析 → 交给最小 JSON Schema 判定 → 运行期判据（updated_at ≤ 今天）
③ 计数 → 退码（不合规 rc=1 / 有判不了的 rc=2 / 全绿 rc=0）；**从不写目标树**
```

## 用法

```bash
cd "<repo>"

python3 scripts/check-doc-meta.py                        # 默认扫描 docs/ 全树（--missing=warn）
python3 scripts/check-doc-meta.py --scope formal --missing=fail   # 正式面第一波（39 篇 + docs/zh）验收口径
python3 scripts/check-doc-meta.py --target docs/zh --json
python3 scripts/check-doc-meta.py --list-rules           # 规则表 M1–M16 + 档位 + 出处
python3 scripts/check-doc-meta.py --self-test            # 成对负控（23 条用例，真命令行 + 真退出码）
python3 scripts/check-doc-meta.py --no-self-test ...     # 内部子进程用（防递归）
```

`--scope`：`docs`（默认，仅 `docs/`）· `repo`（全仓文档面）· `formal`（`docs/项目文档/v2.5.10/` + `docs/常青/` + `docs/skills/` + `docs/zh/` + `docs/en/`）。
★ **2026-09-19 「项目文档」二次分家批2（双认 · 先仓内、缺则仓外取源根）**：`formal` 每棵前缀 = (仓内前缀, 仓外候选)；
首棵 `docs/项目文档/v2.5.10/` 的仓外候选 = `../Zerg-内部文档/项目文档/v2.5.10/`（与 `docs/site/export-and-build.sh`
的 `ZERG_DOCS_ALT` **同根同默认** = 同级 `Zerg-内部文档/`）。语义：**先仓内、缺则仓外**（两棵都在时只取仓内那棵，不双计）；
五棵仓内都没有、仓外候选也没有 ⇒ 仍 **rc=2 不给结论**（逐条打印缺件路径）。`--scope` 的**判据一字未改**。
★ `--scope repo/docs` 的扫描面是**仓内**（仓外那棵不进货架）⇒ 批3 搬出后它们自动少掉那批文件，属预期、不是放宽。
`--missing=warn|fail`：无 frontmatter 块的档位（默认 `warn` = §A1.3③ 的回填期口径；**正式面第一波用 `fail`**）。
`--count-frozen`：把冻结区（`docs/项目文档/`）也计入退码（默认不进：《清单》§2「高（冻结·不改）」）。
★ **二次分家批2（只加注）**：`FROZEN_PREFIX` 的条目值一字未动 —— 对象随**批3** 移出工作树后**仓内零命中**
⇒ 届时按「零命中即删」删除；现在删不掉（对象仍在仓内、`--scope formal` 现跑仍有冻结区 36 篇告警在咬）。
**2026-09-19 开发文档分家**：原第二项 `docs/issues/` 整目录已迁至 `Zerg-内部文档/issues/` ⇒ 从 `FROZEN_PREFIX` 删除
（零命中），冻结区今天只剩 `docs/项目文档/`（17 套快照）。

## 规则表（`--list-rules` 同内容）

| 规则 | 档 | 判据 | 出处 |
|---|---|---|---|
| **M1** | WARN | 文件无 frontmatter 块（未回填） | §A1.3③「缺失只告警」；`--missing=fail` 升为 FAIL |
| **M2** | FAIL | 6 件套缺项（`title`/`type`/`status`/`source_of_truth`/`owner`/`updated_at`） | §3.4② 必填集 |
| **M3** | FAIL | `type` 不在封闭六值内 | §3.4① 判据① |
| **M4** | FAIL | `status` 不在封闭四值内 | 附录B ⑲ |
| **M5** | FAIL | `ai_assisted` 不在 `human\|ai\|mixed` | 附录B ㉔ |
| **M6** | FAIL | 未申报字段（`additionalProperties: false`） | §3.4② 硬规矩① |
| **M7** | FAIL | 字段类型 / 形状不对（非 string、`source_of_truth` 空串或空表、`aliases` 重复等） | §3.4② 表 + schema 的 type/oneOf/minLength/uniqueItems |
| **M8** | FAIL | `title` 空 或 > 60 字（中文按**字符**计） | §3.4④ |
| **M9** | FAIL | `updated_at` 不是 `YYYY-MM-DD` 或不是合法日期 | §3.4② 表（date） |
| **M10** | FAIL | `updated_at` 晚于今天 | §3.4② 表「不得晚于今天」（**JSON Schema 表达不了 ⇒ 运行期判据**） |
| **M11** | FAIL | `status: authoritative` 而缺 `reviewed_at` | §B1.3 / §3.4② 表 |
| **M12** | FAIL | `type: adr` 而缺 `status` / 日期 | §3.4① 判据③ |
| **M13** | FAIL | frontmatter 内重复键（YAML 非法） | 仓内教训：重复键让解析器崩或静默取一条 |
| **M14** | BLOCK | frontmatter 用了本脚本未建模的写法（嵌套映射 / 块标量 / 锚点 / 制表符缩进） | 零三方依赖下的「不给结论」口径 |
| **M15** | BLOCK | schema 用了本脚本未实现的关键字 | 判据同源：schema 加规则而门照绿 = 假覆盖 |
| **M16** | WARN | `type` 与目录默认值不一致 | §3.4① 判据②「告警不失败」（允许显式覆盖） |

**退码三档**（优先级 **2 > 1 > 0**）：`0` 全绿（告警不阻断）· `1` 有不合规项（**只报告不改**）·
`2` 不给结论（用法错 / 输入或 schema 缺件 / 扫描域为空 / 有 M14·M15 项 / 自检未过）。
自检不过 ⇒ **拒绝扫真目标**（仓内口径：门禁自己先要能被证明「会红」）。

## 公开树侧（无 `docs/`）的行为：**BLOCKED、不适用**（2026-09-19 开发文档分家后新增说明）

开发文档分家（2026-09-19）：工作树内 `docs/` 只留**要开源**的（正式面 `zh/`+`en/` + 站点机件 + 活版模块
文档）；设计稿/调研稿/评审稿/问题单整树搬到**工作树之外**的同级目录 `Zerg-内部文档/`（私有、不开源）。
公开树（`publish/whitelist.txt` 只收 `scripts/`）里**没有 `docs/`** ⇒ 本门判不出来（rc=2）。

| 情形 | 改前（实测） | 改后 |
|---|---|---|
| 仿真公开树 `--scope formal --missing=fail`（仓根无 `docs/`） | rc=2（自检的「真 schema 存在」前置不过 ⇒ `自检未过 ⇒ 拒绝扫真目标`，点名 `docs/site/frontmatter-schema.json`） | 同 rc=2，**一字未改**（仍点名那个 schema 路径） |
| 同上，但 `docs/site/frontmatter-schema.json` 在（模拟站点机件先入公开树） | rc=2，正文只说「正式面候选目录一个都不存在」（**看不出缺哪棵**） | 同 rc=2 + **逐条打印缺件路径**（`docs/项目文档/v2.5.10/` · `docs/常青/` · `docs/skills/` · `docs/zh/` · `docs/en/`，各带仓根下绝对路径；★ **批2** 起首棵另带**仓外候选** `../Zerg-内部文档/项目文档/v2.5.10/` 及其解析路径 ⇒ 该格现跑 **6 行**）+ 公开树口径一句 |
| 私有树（`docs/` 在位、正式面 5 棵都建了） | 正常判（`--scope formal` 48 篇） | **一字未改**（前置缺件闸只在「一棵都没有」时触发） |

口径：缺件**不许静默降级**（`rc=2` = 不给结论 ≠ 绿 ≠ 红）；**规则表、判据、`--missing` 语义、剔除类别、冻结区口径
一字未改**，本批只改「缺件时多打印缺件路径」这一处措辞与打印面。

## 自检：成对负控 23 条（`--self-test`）

| 组 | 用例 | 期望 |
|---|---|---|
| ① 好件必绿 | 合规件（6 件套齐 + 合法值域） | rc=0 |
| ② **坏件必红** | 12 条合成坏件**逐条点名**：缺 `owner`(M2) · 坏 `type`(M3) · 坏 `status`(M4) · 坏 `ai_assisted`(M5) · 多字段(M6) · 形状错(M7) · 长标题(M8) · 日期形态(M9) · 未来日期(M10) · 权威缺复审(M11) · 重复键(M13) · 目录默认值不符(M16) | 每条 rc=1 且命中**它自己的规则号** |
| ③ **缺件必 rc=2** | 目标路径不存在 · schema 不存在 · 扫描域为空 | rc=2（**不给结论**，不是红） |
| ④ 不可判定 ⇒ rc=2 | 嵌套映射件 · 块标量件 | rc=2 且含 M14 |
| ⑤ 同一件两种口径结论必须不同 | 未回填件：默认 `warn`（rc=0）↔ `--missing=fail`（rc=1） | 两值不同 |
| ⑥ 剔除类别 | `SKILL.md`（技能契约件） | rc=0 且不计不合规 |
| ⑦ 目录默认值不一致 | `docs/skills/` 下写 `type: record` | rc=0 + M16 告警 |
| ⑧ 判据同源 | 合成 schema 带未实现关键字 `dependencies` | rc=2 |
| ⑨ 只报告不改 | 跑完后夹具树 **sha256 逐字节不变** | 一致 |
| ★ 用例条数自证 | 期望 23 条 = 实跑 23 条（防静默截断） | 相等 |

夹具全部落在 `/tmp/check-doc-meta-*/`（自检产物保留，便于复核），**不碰真仓**。

## 不算不合规的类别（剔出并计数，报告里逐类给数）

| 类别 | 判据 | 为什么不是错 |
|---|---|---|
| **技能契约件** | 基名 `SKILL.md`，或 frontmatter 键集 ⊆ {`name`,`description`} | 那是**另一套** frontmatter 契约（Hermes/技能格式），不带文档 6 件套 |
| ~~主树镜像副本~~ **已退役** | `docs/虫族文档/` —— **已于 2026-09-19 退役**（源目录已从 `docs/` 移除；备份见 `~/zerg-backup/…`） | §2.7 的理由（主树镜像，**同一篇两份地址**）随镜像消失 ⇒ 该条从 `EXCL_PREFIX` **删除**（零命中条目留着 = 假覆盖）；镜像重建则需加回，否则镜像内 frontmatter 会按主树判 |
| ~~第三方源码摘录~~ **已删除** | `docs/调研/multi-agent-源码/`（445 件 6.2M）—— **已于 2026-09-19 按用户令从主仓删除**（先 `cp -a` 到 `Zerg-内部文档/调研/multi-agent-源码/`；主仓盘上已不存在） | 外仓 README（`name`/`description`），不属本仓文档面。**仓内零命中 ⇒ 条目删除**（口径：零命中即删；死条目留着 = 假覆盖）；反向探针（/tmp 仿真造树）：条目在 ⇒ 扫描域 1 篇 / 不合规 0，条目删 ⇒ 3 篇 / 不合规 2 ⇒ 条目确有牙。源树若回到 `docs/调研/`，请把该条加回 `EXCL_PREFIX` |
| 冻结区 | `docs/项目文档/` | §8.1/§1.3：**只登记不改** ⇒ 命中数照报，不进退码。**2026-09-19 分家**：原第二项 `docs/issues/` 已迁至 `Zerg-内部文档/issues/` ⇒ 条目已删（零命中）。★ **二次分家批2（只加注、条目值不动）**：本行对象随**批3** 移出工作树后**仓内零命中** ⇒ 届时按「零命中即删」删除（现跑仍有命中：`--scope formal` 冻结区 36 篇告警） |

## 值域差异登记（**待设计稿一句话**）

- **`status` 四值 vs 五值**：本 schema 取 **⑲ 的四值**（`authoritative|draft|historical|speculation`）—— 与本批（W2）合同一致；
  而 **§3.4② 表那一格写的是五值**（多一个 `reviewed`）。两处不一致，**以 ⑲ 与本合同为准**，差异在此登记。
- **实测证据（两时点对照，同一棵树）**：2026-09-18 **19:43** 那次现跑，`docs/zh/` 九篇全用 `status: reviewed`
  ⇒ **M4 命中 8 条**；**19:47** 复跑，九篇已改成 `status: authoritative` + `reviewed_at: 2026-09-18` ⇒ **M4 命中 0**。
  ⇒ **并行分路已自行收敛到四值**，本 schema 不需要改；若将来要收 `reviewed` 进值域，改 `docs/site/frontmatter-schema.json` 一处即可（脚本无副本）。

## 真仓现跑（2026-09-18 19:46，**只报告不改**）

```bash
python3 scripts/check-doc-meta.py --json          # scope=docs · missing=warn
# → files 1563 · ok 9 · violation 0 · warn 151 · blocked 0 · skipped 1（技能契约件）
#   冻结区：warn 1402（全部是 M1 未回填）· per_rule: M1/live 151 · M1/frozen 1402

python3 scripts/check-doc-meta.py --scope formal --missing=fail
# → 48 篇（v2.5.10 30 + 常青 5 + skills 4 + docs/zh 9）· 合规 9 · 不合规 8 · 剔除 1 · rc=1
#   不合规 8 = docs/skills 4 篇 + docs/常青 4 篇**没有 frontmatter**（M1 由 --missing=fail 提升）
```

- **合规 9 篇** = `docs/zh/` 九篇（6 件套齐、值域合法、`authoritative` 带 `reviewed_at`）—— 它们是**当前唯一**达标的正式面文档。
- **未回填 151 篇**（可改面）+ 1402 篇（冻结区）—— 与调研稿 §A1.4「字段回填是逐篇的」一致：**今天是「不合法即红、缺失只告警」**。
- 报告里每一条命中都给 `路径 → 字段: 说明`（`--max-examples` 控制条数，默认 5）；`--missing=fail` 时 M1 显示为 `FAIL*` 并附一行说明。

## 边界与已知限制（如实列出）

- **YAML 只解析严格子集**：顶层键值 / 引号串 / 行内列表 `[a, b]` / 块列表；**嵌套映射、块标量（`|` `>`）、锚点别名、制表符缩进 ⇒ M14 `rc=2`**（不猜）。
  未加引号的纯量会剥掉「空格 + `#`」形式的行内注释；`2026-09-18` 这类裸日期**保持字符串**（由 schema 的 `format`/`pattern` 判），不转成 YAML 日期对象。
- **`title` 长度按字符计**（`len(str)`），与 §3.4④「中文按字符计」一致；不按字节、不按显示宽度。
- **`updated_at ≤ 今天` 用本机日期**（`datetime.date.today()`）；跨时区/时钟漂移不在本门判据内。
- **M16（目录默认值）只覆盖能判的目录**：`docs/skills/`→`how-to`、`docs/01-设计/`→`explanation`、`docs/项目文档/<版>/` 的 `NN-模块*/架构*/设计*/体系*`→`reference`、`使用-*`→`how-to`、`变更-*/承接项-*/任务表-*/进度记录-*/靶子表*`→`record`、`docs/常青/` 的 `战略-*`→`explanation`/`接入*`→`reference`。
  ★ **批2 双认**：`../Zerg-内部文档/项目文档/<版>/` 走**同一张表**（`default_type_for()` 两处前缀共用一套默认值）⇒ 批3 搬出后 M16 告警不会静默消失（判据一字未改，只是两处都认）。
  §B2 表里给「**或**」的目录（`docs/调研/` `docs/02-调研/` `docs/03-评审/` `docs/thunderbolt/`）**本门不判**（不发明判据）。
- **`m12`（`type: adr` ⇒ 要 status/日期）在四值 schema 下恒被 6 件套覆盖**，保留为显式登记（判据②的第三条不留空）。
- **未挂进 `scripts/precommit-gates.sh`**：三档退码与那套 `rc`/`empty` 两模式不相容（Q14 **未选型**，§16 #14）⇒ 不许挂成尾部软检查。
  语法已被 `pub` scope 的 `scripts/*.py`（ast.parse）步骤覆盖；**是否设阻塞闸待 Q14 拍板**。
- **本门从不写盘**（自检除外，且只写 `/tmp`）；`--json` 输出可供下游（白名单生成 / 健康度 4+1）消费。
