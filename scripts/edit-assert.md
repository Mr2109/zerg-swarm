# edit-assert — 凡写盘，必过一门（腿二五步 E1–E5 · 判据 J1–J6）

> 规格：`docs/01-设计/设计-改码与变异安全门-v1.1-20260918.md`（§2.2 五步 · §3.2 R-1～R-9 ·
> §4.2 四形态 · §4.3 现况表 · §7.1 J1–J6 · §7.4 落点 · §8 B1–B8）+ 附录A（M1–M18 · C1–C14）。
> 实现在 `scripts/edit-assert`（python3 ≥3.6 · 零三方依赖 · 不联网 · 门内 git **只读**）。
> **腿一（变异器）本轮未接**：本门只做「改动怎么落盘」，不负责「变异点从哪来」（两腿解耦，§2.3）。

## 一条命令的五步

```
  E1 dry-run ──► E2 命中数断言 ──► E3 写盘（原子替换）──► E4 diff 复核 ──► E5 幂等二次跑
  (只读预览)      (0 命中/不匹配⇒红)   (前像 sha256+n)        (改动⊆报告集合)     (二次跑零改动)
        │                 │                    │                    │                  │
        └── 定位=ast-grep scan -r <rule.yml> --json ⇒ 取数组长度（rc 一律不采信，R-8）────┘
```

`E2` 是事故里唯一救命的那一下（写盘前断言）——本门把它做成**不可跳过**的步骤：**0 命中 ⇒ 失败**（不是「什么也没发生」）。

## 用法

```bash
# 只读预览（E1+E2；不写盘、不落台账）
python3 scripts/edit-assert --rule <rule.yml> --target <f> --expect 1 --reason '<为什么是 1>'

# 真写盘（五步全跑；写盘必须给来源文本与目标文本）
python3 scripts/edit-assert --rule <rule.yml> --target <f> --expect 1 --reason '<why>' --write \
        --replace-from '<命中文本，逐字>' --replace-to '<新文本>'

# 四种期望值形态（§4.2；禁止 expect >= 1 当默认）
--expect N          精确        --unique            唯一（= 1）
--set N1,N2         集合（**必须** --reason）        --group <一组断言.json>   一组 (target, expect, reason)

# 自证 / 登记表 / 负控
python3 scripts/edit-assert --self-test              # 自检 16 条（真命令行 + 真退出码）
python3 scripts/edit-assert --list-assertions         # 断言登记表 + 原因码全集 + 台账落点 + 规则模板
python3 scripts/edit-assert --neg-control=<断言名>    # 负控：成对 + 点名（正常 rc=0 / 取反 rc≠0 / 坏名 rc=2）

# 其它开关：--actor · --role author|verifier（verifier 必须 --sandbox-root）· --ledger-path（测试用）
#           --clock <秒>（墙钟上界，默认 60）· --diff-cmd（覆盖 E4 的外部命令）· --ast-grep <路径>
```

**真跑前先自检**（M15-②；同 `precommit-gates.sh:36` 的口径）：自检不过 ⇒ **拒绝跑真目标**。
`--no-self-test` 只给「自检自己的子进程」用（否则无限递归），别在正常调用里用它。

## 退出码三档 + 原因码（每段自带原因码，J4）

| rc | 含义 | 原因码 |
|---|---|---|
| **0** | 通过 | `OK_GREEN`（E1–E5 全过）· `OK_IDEMPOTENT`（幂等复跑零改动）· `OK_DRYRUN` |
| **1** | 失败 | `FAIL_HIT_ZERO`（0 命中＝门的第一颗牙）· `FAIL_HIT_MISMATCH` · `FAIL_REPLACE_MISMATCH`（写盘前拦下、零改动）· `FAIL_NO_CHANGE` · `FAIL_DIFF_SUPERSET` · `FAIL_NOT_IDEMPOTENT` · `FAIL_GROUP_PARTIAL` · `FAIL_E1_LOCATE` · `FAIL_NEG_INVERTED` ·（负控成立走本档：`OK_NEGCONTROL`） |
| **2** | **不给结论**（不是红） | `BLOCKED_MISSING_TOOL` · `BLOCKED_MISSING_TARGET` · `BLOCKED_UNREGISTERED_TOOL` · `BLOCKED_UNPARSABLE` · `BLOCKED_NO_PARSER` · `BLOCKED_RULE_SCOPE` · `BLOCKED_SCAN_UNPARSEABLE` · `BLOCKED_TIMEOUT` · `BLOCKED_UNFINISHED` · `BLOCKED_LEDGER_UNWRITABLE` · `BLOCKED_LEDGER_TMP` · `BLOCKED_LEDGER_CORRUPT` · `BLOCKED_WRITE_ABORTED` · `BLOCKED_USAGE` · `BLOCKED_SELFTEST` · `BLOCKED_NEGCONTROL_NAME` · `BLOCKED_NEGCONTROL_NOTOOTH` |

不阻断但**不静默**：`WARN_RULE_FIELD` · `WARN_GIT_SKIPPED` · `WARN_LEDGER_TMP`。
全集见 `--list-assertions`（每条带档位与含义）。

### 三态前置顺序（写死，顺序错会让原因码互串 —— D4 实测）

```
① 台账（路径 / 可写性 / 未收尾检测） → ② 缺件（ast-grep · 规则 · 目标） → ③ 可解析（gofmt -e / ast.parse） → ④ 命中数
   BLOCKED_LEDGER_* / BLOCKED_UNFINISHED   BLOCKED_MISSING_TOOL          BLOCKED_UNPARSABLE /              FAIL_HIT_*
                                           BLOCKED_UNREGISTERED_TOOL     BLOCKED_NO_PARSER
```
把 `ast-grep` 从 `PATH` 拿掉、同时目标又是**语法错文件**时，报出来的必须是**缺件**而不是不可解析
（顺序错就会报「不可解析」⇒ 归因错到锚点上，J4 的三态可分辨废掉一半）。

## 规则模板两个必写项（另加一个「钉住被换内容」）

模板：`scripts/edit-assert-rule-template.yml`（照抄改 `pattern`/符号名即可）

```yaml
rule:
  kind: return_statement             # 节点类型（AST 级）
  pattern: return $A == $B           # ★ 钉住「要被换掉的那段内容」（否则换完仍命中 ⇒ E5 报红并复原）
  inside:
    kind: function_declaration
    stopBy: end                      # ★ 必写项①：默认 neighbor ⇒ **静默零命中**（实测）
    has:
      field: name                    # ★ 必写项②：不写 field ⇒ 同名标识符别处出现时**误伤**
      pattern: teamOutcomeOK
```
`inside:`/`has:` 在而没写 `stopBy` ⇒ **rc=2 `BLOCKED_RULE_SCOPE`**；`has:` 在而没写 `field:` ⇒ 告警 `WARN_RULE_FIELD`。

## 台账（§7.4 定案落点；append-only JSONL；begin/end 两行）

```
${ZERG_STATE_DIR:-$HOME/.zerg/state}/edit-assert-ledger.jsonl     # 文件名不含日期（日期在行内 ts）
```
路径解析**逐字抄** `scripts/chat-harness.py:30`（同默认、同 env 名）；`--ledger-path` 可显式覆盖（测试用）；
**不许静默退化到 `/tmp`**（`statepath.go:1-3`：/tmp 会被重启即清 + tmp_cleaner 删除）。**只记写盘动作**：dry-run 不落行。

| 字段 | 含义 |
|---|---|
| `ts` · `run_id` · `phase` | 时间（行内）· 一组 begin/end 的关联 id · `begin`｜`end` |
| `actor` · `role` | 谁改的 · `author`｜`verifier`（verifier 行须带 `sandbox_root`） |
| `target` · `rule` · `rule_sha256` | 目标绝对路径 · 规则文件 · 规则内容 sha256 |
| `expect` · `expect_form` · `reason` | 期望值 · 形态（精确/唯一/集合/一组）· 期望值的理由 |
| `hits` · `hits2` | E1 命中数 · E5 二次跑命中数 |
| `pre_sha` · `post_sha` | 前像 / 后像 sha256（可逆判据 = 逐字节一致） |
| `status` · `reason_code` · `idem_ok` | 结论码 · 原因码 · 幂等是否成立 |
| `xform_key` · `replace_from`/`replace_to` | 变换指纹（查「幂等复跑」用）· 写盘的来源/目标文本（仅 begin 行） |
| `tool` · `tool_version` · `tool_binary_sha256` | 定位器身份（M15 防漂移；启动断言：与 `tools/cli-tools.json` 不符 ⇒ rc=2） |
| `cmd` · `match_level` · `cwd` · `sandbox_root` | 定位命令原文 · 匹配级别（L-AST）· 工作目录 · 副本根 |

**启动做未收尾检测（J6/C2）**：有 `begin` 无 `end` ⇒ `rc=2 BLOCKED_UNFINISHED` + 打印残留路径与**当前 sha256**
（与前像是否一致）。门内失败 ⇒ 门按**内存前像字节**自动复原（`os.replace` 原子回写）；进程被杀 ⇒ 前像字节不可得 ⇒
按台账 `pre_sha256` 人工核验复原（**门不替人动手**）。

## 边界（B1–B8 逐条落到的位置）

- **不改用户工作树**：变异/试验一律在副本（`cp -a`，本机 rsync 是 openrsync，不支持 `-X`/`-A`）里做。
- **audit vs 改动目标**：审计台账是**唯一**允许写 `state` 的目标；改动目标只允许「仓内文件 / 已登记副本根」。
- **git 只读**（B4）：门内只用 `git diff -U0`（`E4`），禁 `add`/`commit`/`reset`/`stash`。
- **没挂进 `precommit-gates.sh`**：Q14（三档 rc 与门禁 `rc`/`empty` 两模式不相容）**未选型** ⇒ 本轮不改门禁脚本，
  也不许把它挂成尾部软检查（B8 的血坑：尾部软门禁的 rc 会吃掉整脚本的 rc）。
- **没有文本锚点降级路径**（R-3/M5）：缺件/超时/不可解析一律 rc=2。
