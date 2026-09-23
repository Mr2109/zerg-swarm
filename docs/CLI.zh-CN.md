# 命令行参考（CLI）

[English](CLI.en.md) | **中文**

> *中文原版（唯一真相源）。英文版（[CLI.en.md](CLI.en.md)）为派生翻译；两版如有不一致，以**本文**为准。*

虫族对外只有一条面：**命令行**。能力全部收在命令树里，形态是

```text
zerg <对象> <动作> [参数] [旗标]
```

对象与动作最多三层。用法上只有一条纪律：**只敲命令、不拼脚本** —— 既有的脚本与 HTTP 面都由命令内部收编，命令面是唯一入口。

---

## 0. 本版须知 · 一处**可被外部脚本观察到**的行为变更

`--json` **给了旗标却不给字段**（例如 `zerg gate run --json`）的退出码，由 `1` 归到 **`2`**（用法错）。

* **为什么改**：退码表（`zerg help exit-codes` · 唯一真源）里「用法错 = `2`」。旧实现这一格退 `1`，与自己的表不同向 —— 属「规格自身有错」，**以表为准**（兼容承诺的例外之一：安全 / 未规定行为 / 规格有错）。
* **变的是什么**：只这一格的**退出码**。stdout 仍是 **0 字节**（危险动作档是机器可读错误包封）、字段清单仍走 stderr —— 提示面一个字未动。
* **谁会被打到**：按 `rc == 1` 判断「参数没给全」的脚本/CI 判据。请改判 `rc == 2`（或用 `zerg help exit-codes` 里的语义名，别把数字抄进判据）。
* **不做静默改**：本条同时记在 `CHANGELOG.md`；矩阵（`core/cmd/zerg/testdata/cli-matrix.json`）**同批重冻**，26 格 `want_rc` `1 → 2`。

---

## 1. 要用的命令在哪

帮助面本身就是命令，先问它：

| 命令 | 给出什么 |
|---|---|
| `zerg help` | 全部命令的**用法串**（一行一条 · 逐字来自命令树） |
| `zerg help <主题>` | 单主题说明页（主题表就在命令树里） |
| `zerg help dangerous` | 危险动作清单 + 三态确认的形状 |
| `zerg help exit-codes` | 退出码表（唯一真源） |
| `zerg help config` | 配置优先级链 |
| `zerg help credentials` | 令牌与凭据面 |
| `zerg version` | 单行身份（组件 · 版本 · 代码 sha · 构建时间） |

---

## 2. 全部命令（生成区）

<!-- ===== 生成区 BEGIN · 命令清单（本区由 scripts/docs/gen-cli-reference.py 注入 · 勿手改）===== -->
> **现读真源数**：命令树 **119** 条（开放命令 **81** 条 + 危险动作 **38** 条）· 契约矩阵 **415** 格（must-fail 415）· 空白 **0** · 豁免 **0**。
> 本区逐条的命令名、用法串（旗标）、说明与 `--json` 字段，由生成器照命令树**现算**后注入 —— 名字与旗标**一个都不是手写的**。

**本块为生成物（勿手改）**：由 `scripts/docs/gen-cli-reference.py` 照命令树**现算**后注入 —— 重跑 `python3 scripts/docs/gen-cli-reference.py --emit --target publish/docs/CLI.zh-CN.md`；判 drift 用同名 `--check --target publish/docs/CLI.zh-CN.md`。本块里的命令名、用法串（旗标）、说明、`--json` 字段、投影端点与茧壁层级**逐字来自命令树，一个都不是手写的**。

### 已开放（83 条）

| 命令 | 用法（旗标） | 说明 | `--json` 字段 | 投影的远端端点 | 茧壁层级 |
|---|---|---|---|---|---|
| `zerg agent logs` | `zerg agent logs <机器名> [--json <字段>]` | 该机的日志（**主控面今天没有日志端点 ⇒ 不给结论**，退码 8） | machine,available,detail | （/api/logs/* 今天不在路由表里 ⇒ 本命令声明面 + kind=blocked） | `node` |
| `zerg agent ls` | `zerg agent ls [--json <字段>]` | 子端（机器）清单（投影 /api/fleet/status） | machine,healthy,code_version,code_sha,cpu_pct,gpu_pct,mem_available_gb,mem_total_gb,models,last_seen | GET /api/fleet/status | `node` |
| `zerg agent models` | `zerg agent models <机器名> [--json <字段>]` | 该机上可用的模型（投影 /api/fleet/models 里 host 命中的那些） | id,host,backend,modality,mem_gb,file | GET /api/fleet/models | `node` |
| `zerg agent ping` | `zerg agent ping <机器名> [--direct <host:port>] [--json <字段>]` | 探活一台机（默认经主控；`--direct` 直连且回显 via） | machine,healthy,code_version,code_sha,last_seen,via | GET /api/fleet/status（默认档） | `host` |
| `zerg agent probe` | `zerg agent probe <机器名> [--direct] [--json <字段>]` | 探活诊断（默认经主控；直达属 F-2 例外、要显式 `--direct`） | machine,healthy,code_version,code_sha,last_seen,via,direct_gate | GET /api/fleet/status（默认档） | `node` |
| `zerg agent show` | `zerg agent show <机器名> [--json <字段>]` | 看一台子端（投影 /api/fleet/status 的单机条目 —— 与 `agent ls` 同一份载荷） | machine,healthy,code_version,code_sha,cpu_pct,gpu_pct,mem_available_gb,mem_total_gb,models,last_seen | GET /api/fleet/status | `node` |
| `zerg api help` | `zerg api help` | api 族说明（只读面 · 逃生门 `api call` 暂不开） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg api ls` | `zerg api ls [--json <字段>]` | HTTP 能力面（投影 /api/capabilities） | name,endpoint,desc,example | GET /api/capabilities | `host` |
| `zerg api openapi` | `zerg api openapi [--json <字段>]` | HTTP 路径表（投影 /api/openapi.json 的 paths） | path,method,summary | GET /api/openapi.json | `host` |
| `zerg apply` | `zerg apply <件> [--confirm=<目标>] [--json <字段>]` | **做**：只吃那一份意图件（L1 schema → L2 引用 → L3 干跑 → L4 人在环）· **校验四层已开放**；写面（真做）本版未开放 · 拒执退码 2 | plan_id,action,target_name,layers_passed,failed_layer,detail | 本机（无远端对应） | `host` |
| `zerg approve keygen` | `zerg approve keygen --by <人名>` | 生成**操作员密钥**（人在终端上设口令；私钥口令加密落盘，公钥给消费者验签） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg approve ls` | `zerg approve ls [--json <字段>]` | 列人签批准件（逐件带上**验签判决**：验过 / 无签名 / 签名坏 —— 后两者不算批准） | tool,approver,approved_at,scope,note,sig_alg,key_id,state,path | 本机（无远端对应） | `host` |
| `zerg approve show` | `zerg approve show <工具名> [--json <字段>]` | 看一枚批准件的全貌 + 验签判决（消费者只认「验过」那一档） | tool,approver,approved_at,scope,note,sig_alg,key_id,state,path | 本机（无远端对应） | `host` |
| `zerg archive hash` | `zerg archive hash <件\|目录>… [--json <字段>]` | 一批件算 sha256（**一进程吃 N 件**）—— 逐行 `sha256␣␣路径`，与 `shasum -a 256` 逐字相同（手搓 1072 次 shasum 的替身） | path,sha256,bytes | 本机（无远端对应） | `host` |
| `zerg archive manifest` | `zerg archive manifest <载荷目录> --out <袋目录> [--dry-run \| --yes] [--json <字段>]` | 出归档**三件套**（RFC 8493 BagIt：载荷 `data/` + `manifest-sha256.txt` + `tagmanifest-sha256.txt`）· `--dry-run` 先出逐件清单 · 真写要 `--yes` · 失败回滚 | bag,entry,sha256 | 本机（无远端对应） | `host` |
| `zerg archive verify` | `zerg archive verify <袋目录> [--json <字段>]` | 校验一只袋（**重算载荷** ↔ 清单逐件对拍：清单被抹一条 / 载荷改一字节 / 多出未登记件 ⇒ 判红） | entry,want,got,verdict | 本机（无远端对应） | `host` |
| `zerg ask` | `zerg ask <提示> [--capability 名]… [--prefer 名]… [--model 名] [--node 名]… [--min-ctx n] [--min-mem-gb n] [--no-fallback] [--dry-run] [--timeout 时长] [--json <字段>]` | 问一次推理、**不落任务队列**（`task submit` 的对偶）· 能力筛是硬筛 | model,host,backend,matched,capabilities,registry_id,reply,dry_run | GET /api/fleet/models · GET /api/models/registry | `host` |
| `zerg build all` | `zerg build all [--only cli] [--dry-run \| --confirm=<主机名> --yes]` | 重编制品（**自举档已开放**：`--only cli` 只写 bin/zerg 一件；换件档/发布档仍未开放） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg build ls` | `zerg build ls [--json <字段>]` | 制品现状（现读 bin/ 逐件 sha256 + 身份件）—— 清单真源仍是构建脚本 | name,sha256,bytes,mtime,version,code,built | 本机（无远端对应） | `host` |
| `zerg build release` | `zerg build release [--dry-run \| --confirm=<主机名> --yes]` | 打包发布件（**计划面已开放**：`--dry-run` 出计划件；换件档真跑本版未开放 · 拒执退码 2） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg build show` | `zerg build show <件> \| --all [--json <字段>]` | 一件的**身份**（sha256/mtime/inode/type/arch/签名态 —— 换件后验「在跑的件 == 盘上件」要它） | name,sha256,bytes,mtime,inode,type,arch,signed | 本机（无远端对应） | `host` |
| `zerg calib ls` | `zerg calib ls [--json <字段>]` | 标定线 3 件的声明面（名字 · 件 · 角色 · 归属 —— 逐件现读，不另抄一份） | name,script,role,state | 本机（无远端对应） | `host` |
| `zerg calib show` | `zerg calib show <名> [--json <字段>]` | 单件标定脚本的现状（归属 · 角色 · 执行面 · 退码口径） | name,script,role,state,runner,exit | 本机（无远端对应） | `host` |
| `zerg cocoon ls` | `zerg cocoon ls [--json <字段>]` | 虫茧清单（主控面无投影端点 ⇒ 不给结论） | name,state,port,path | GET /api/cocoons（主控面没有 ⇒ 现跑 404） | `host` |
| `zerg cocoon open` | `zerg cocoon open <茧名> [--confirm=<茧名> --yes \| --dry-run]` | 起虫茧的文档服务（8610 · D3 起服务档 · **计划面已开放**：`--dry-run` 出计划件；真跑本版未开放 · 拒执退码 2） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg code find` | `zerg code find <正则> [--path <子目录>] [--glob <模式>] [--json <字段>]` | 在码里找一处东西在哪（只读取证 · 手搓 grep/git grep 的替身） | path,line,text | 本机（无远端对应） | `host` |
| `zerg code show` | `zerg code show <件:行> [--ctx <N>] [--json <字段>]` | 看源码里**某一行**长什么样（带 `件:行` · 只读取证 · 手搓 `sed -n` / `awk` 的替身） | path,line,text,target | 本机（无远端对应） | `host` |
| `zerg config reload` | `zerg config reload [--dry-run] --yes [--json <字段>]` | 热加载主控配置（照 `nginx -s reload`：**先校验、失败回滚**）—— 名册件本地解析不过 ⇒ **不发请求**（旧配置继续跑） | status,models,added,fleet_nodes,fingerprint | POST /api/config/reload（路由已在跑的主控上 ⇒ 主控零改动） | `host` |
| `zerg context ls` | `zerg context ls [--resume] [--json <字段>]` | 档位名册（离线也出表） | name,core,gateway,default_node,token_source | 本机（无远端对应） | `host` |
| `zerg core daemon` | `zerg core daemon ls [--declared] [--json <字段>]` | `daemon ls`：本机服务脚本逐件可查（scripts/svc/ 5 件）+ `--declared` 并给声明面两列与幽灵段（与 `doctor` 同源同值） | name,script,declared,note | 本机（无远端对应） | `host` |
| `zerg core logs` | `zerg core logs [--json <字段>]` | 主控日志（`/api/logs` **路由没接** ⇒ 不给结论，退码 8） | available,detail | GET /api/logs（处理器在 handlers.go:966 · 路由没接 ⇒ 现跑 404） | `host` |
| `zerg core ps` | `zerg core ps [--root <仓根> \| --path <声明件>] [--json <字段>]` | 现值面逐条读（**三格**：pid / 起时 / 命令行）—— 行面 = 声明件点名的进程特征命中的进程（launchd 声明的 + ghost 幽灵都列）· 只读 | pid,start,command,kind,name,owner | 本机（无远端对应） | `host` |
| `zerg core status` | `zerg core status [--json <字段>]` | 主控现状（投影 /api/core/status） | ok,pid,started_at,version | GET /api/core/status | `host` |
| `zerg dev proposal` | `zerg dev proposal new --title <题> --target <待办编号> --goal <目标> --evidence <出处> --rollback <退点> --criterion <可跑的判据> [--file <要改的件>]… [--subject <提出者>] [--subject-kind human\|ai\|egg\|ci] [--egg-id <卵 id>] [--approver <批准者>] [--approver-kind human]` | 提案件通道：只产可审查物（new \| list \| show \| check）· 目标必须回指既有编号 · **判据必须可机检** · 「提 ≠ 批」两对字段（subject/approver） | id,title,target,goal,evidence,rollback_ref,by,state,criterion,created_at,path,files,criterion_state,subject,subject_kind,egg_id,approver,approver_kind | 本机（无远端对应） | `host` |
| `zerg dev receipt` | `zerg dev receipt new --what <做了什么> --next <下一步> [--evidence <证据>]… [--blockers <阻碍>]… [--commit <sha>]… [--trace <trace_id>]` | 交接回执（一轮一页 · 带 trace_id）：new \| ls \| show；读法 = `zerg context ls --resume` | round_id,trace_id,what,next,blockers,evidence,commits,created_at,path | 本机（无远端对应） | `host` |
| `zerg dev verify` | `zerg dev verify --candidate <候选 id> [--results <结果表>] [--code-sha <sha>] [--node <名>] [--layer <档>] [--gate [--human-approval <名>]] [--dry-run] [--json <字段>]` | 合成一份验收证据单（只收证据、**不给「通过」的结论**）· 证据为空 ⇒ 2 | candidate,criterion,verdict,rc,log_path,code_sha,node,layer,contract | 本机（无远端对应） | `host` |
| `zerg doctor` | `zerg doctor [--json <字段>]` | 环境自检（本机项 + 主控可达）· 逐项判定词 | name,verdict,detail,advice | 本机（无远端对应） | `host` |
| `zerg egg ls` | `zerg egg ls [--json <字段>]` | 卵 × 设备矩阵（只读投影：host / model / state 三格 · 照现成端点包装） | egg_id,host,model,state,backend,mem_gb | GET /api/fleet/models + GET /api/fleet/status + GET /api/models/{name}（现成端点包装 · 不新开一条路） | `node` |
| `zerg egg run` | `zerg egg run <卵 id> [--yes \| --dry-run]` | 把卵跑起来（**写面本版未开放**：连干跑一道押后、真跑退码 8；只读投影见 `egg ls` / `egg show`） | （无机器面） | 本机（无远端对应） | `node` |
| `zerg egg show` | `zerg egg show <卵 id> [--json <字段>]` | 单枚卵的现状（只读投影：host / model / state 三格 · 同一份真源） | egg_id,host,model,state,backend,mem_gb | GET /api/fleet/models + GET /api/fleet/status + GET /api/models/{name}（现成端点包装 · 不新开一条路） | `node` |
| `zerg eval ls` | `zerg eval ls [--json <字段>]` | 评测线 22 件的逐件归属（① 收编 5 · ② 保留内部 4 · ④ 维持待拍 13 —— 现算不手写） | name,script,class,state | 本机（无远端对应） | `host` |
| `zerg eval show` | `zerg eval show <名> [--json <字段>]` | 单件评测脚本的现状（归属 · 一句理由 · 执行面） | name,script,class,state,why,runner | 本机（无远端对应） | `host` |
| `zerg gap ls` | `zerg gap ls [--state <仍缺\|已派\|已立项\|已解\|回归\|不做>…] [--prio P0\|P1\|P2] [--impact <命令面\|门禁面\|文档面\|公开面\|换件面\|归档面>] [--json <字段>]` | 缺口账（只读面：不写真源、不写审计）· 可按状态/优先级/影响面收窄 · 账内闭集外的值 ⇒ 判红并**点名到行** | id,prio,impact,state,want,summary,fp,verify_cmd,found_at,last_verified_at,solved_at | 本机（无远端对应） | `host` |
| `zerg gate bench` | `zerg gate bench [--fast \| --scope <s>…] [--repeat n] [--json <字段>]` | 量门禁耗时（逐趟 real/user/sys + 中位/最差 + 门禁身份 sha256） | run,real_ms,user_ms,sys_ms,rc,log | 本机（无远端对应） | `host` |
| `zerg gate explain` | `zerg gate explain <步名> [--json <字段>]` | 读懂某一步到底在判什么（scope/模式/判据/退码口径/日志路径/出处文件:行 —— **精确匹配**步名） | scope,mode,criterion,verdict,exit,log,source,command,script_say | 本机（无远端对应） | `host` |
| `zerg gate ls` | `zerg gate ls` | 门禁步骤表（逐行等于脚本 --list；薄壳不另写一份） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg gate matrix` | `zerg gate matrix [--out <件>] [--json <字段>]` | 命令面自己的 must-fail 矩阵（逐格可读可导 —— 新增命令照着它补格） | command,case,want_rc,why | 本机（无远端对应） | `host` |
| `zerg gate results` | `zerg gate results [--last] [--dir <目录>] [--json <字段>]` | 读**现成**一趟门禁产物的四数（通过/失败/不给结论/只报告 + 步数与总退码）· 只读 | step,status,rc,secs,log | 本机（无远端对应） | `host` |
| `zerg gate run` | `zerg gate run [--scope <s> \| --fast] [--outdir <目录>] … \| zerg gate run --step <步名> [--self-test] [--json <字段>]` | 跑门禁（旗标逐字透传；退码原样转出，不翻译；`--step` 只跑一道门） | step,scope,mode,verdict,rc,secs,log,outdir | 本机（无远端对应） | `host` |
| `zerg gate self-test` | `zerg gate self-test` | 门禁自检（合成步骤 · 不碰真目标） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg gate show` | `zerg gate show <步名> [--json <字段>]` | 看某一步要跑的命令串（脚本 --emit-cmd）；`--json` 另给四格（scope/mode/判据/日志路径） | scope,mode,criterion,log | 本机（无远端对应） | `host` |
| `zerg gateway breakers` | `zerg gateway breakers [--reset] [--dry-run \| --yes]` | 网关断路开关（D2 写面 · `--reset` 要 `--yes`） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg gateway models` | `zerg gateway models [--json <字段>]` | 网关侧模型面（与 `model ls` **同源** —— 网关没有第二份模型表） | id,host,backend,modality,mem_gb,file | GET /api/fleet/models | `host` |
| `zerg help` | `zerg help [<主题>]` | 帮助（主题见 `zerg help <主题>`；表在 topics.go） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg help export` | `zerg help export [--out <目录> \| --docs-ver <X.Y.Z>] [--dry-run] [--json <字段>]` | 把命令树导出 markdown 进版本档案目录（产物 · 勿手改）；只读档 `--dry-run` 出逐条清单、一个字节不写 | path,root,docs_version,commands,dangerous,schema,layers,generated_at,written,dry_run,command,is_dangerous,danger_level,opened,confirm_target,summary,layer,endpoint,fields | 本机（无远端对应） | `host` |
| `zerg impact` | `zerg impact <文件或契约id> [--wide｜--strict] [--deep] [--for-model｜--for-human] [--json <字段>] [--gate-results <那次门禁的结果表｜它的日志目录>]` | 改一处会牵动谁（只读：人面三行 + 波纹卡片 ≤12 条/≤1.2k token + 六键包封；挂干跑属 `A4`） | what,why,how,red | 本机（无远端对应） | `host` |
| `zerg itask interval` | `zerg itask interval [--json <字段>]` | 周期现值（只读面；写面 `itask interval set` 未开放） | intervals,note,state | GET /api/internal-tasks/intervals | `host` |
| `zerg itask ls` | `zerg itask ls [--json <字段>]` | 内部任务清单（16 类 · 含冷却与最近执行 · 投影 /api/internal-tasks） | id,description,cooldown,default_hours,auto_run,last_run,state | GET /api/internal-tasks | `host` |
| `zerg itask mode` | `zerg itask mode [--json <字段>]` | 自动运行开关现值（只读面；写面 `itask mode set` 未开放） | modes,note,state | GET /api/internal-tasks/modes | `host` |
| `zerg itask state` | `zerg itask state [--json <字段>]` | 引擎状态（投影 /api/internal-tasks/state） | note,state | GET /api/internal-tasks/state | `host` |
| `zerg metrics` | `zerg metrics [--input <读数档>] [--json <字段>]` | 「该改哪」那把尺：把四类读数（重复 / 未用符号 / 覆盖率 / 门禁）**归一成一个可比排序**（只读 · 空输入 ⇒ 不给结论） | rank,grade,category,target,value,budget,unit,score,source | 本机（无远端对应） | `host` |
| `zerg model add` | `zerg model add --model <模型名> --host <主机> --file <GGUF 路径> [--backend …] [--mem-gb …] [--ctx …] [--arch …] [--desc …] [--mmproj …] [--added 日期] [--verified] [--dry-run \| --yes] [--json <字段>]` | 往名册件（`gateway/fleet.yaml`）**先校验后写**加一条模型（`--dry-run` 先行 · 真写要 `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚 · 不覆盖别人的条）——**块按 `--model`（模型名）定位/新建**（列表形与裸映射形都认）· `--host` 只作该条的 `host:` 字段值 | model,host,file,fleet,line,added | 本机（无远端对应） | `host` |
| `zerg model ls` | `zerg model ls [--json <字段>]` | 可用模型（投影 /api/fleet/models） | id,host,backend,modality,mem_gb,file | GET /api/fleet/models | `host` |
| `zerg model opts` | `zerg model opts get <模型 id> [--json <字段>] \| zerg model opts set <模型 id> --set k=v… [--dry-run \| --yes]` | 适配器参数（get 只读 / set 实时生效要 --yes） | model,schema,note | GET \| PUT /api/models/{name}/adapter-opts | `host` |
| `zerg model show` | `zerg model show <模型 id> [--json <字段>]` | 看一个模型（投影 /api/fleet/models 的单条；对象是**模型 id**，不是机器名） | id,host,backend,modality,mem_gb,file | GET /api/fleet/models | `host` |
| `zerg plan` | `zerg plan <族> <动作> <对象…> [--node 名]… [--expect 旧值] [--out <件>] [--json <字段>]` | **算**：产出一份意图件（M6 包封 · F1–F7 + 四附加件）· **零副作用** | plan_id,plan_digest,action,target_kind,target_name,nodes,host,status,path,layers_passed | 本机（无远端对应） | `host` |
| `zerg plugin ls` | `zerg plugin ls [--json <字段>]` | 插件清单（`zerg-<名>` 约定 · 零注册表 · 影子告警 + 信任声明） | name,command,path,shadowed,shadow_of,trust | 本机（无远端对应） | `host` |
| `zerg port ls` | `zerg port ls [<端口>] [--json <字段>]` | 看某个端口被谁占着（含 pid/ppid/inode/在跑件路径 · 手敲 lsof 的替身） | port,pid,ppid,process,sock,path | 本机（无远端对应） | `host` |
| `zerg propose ls` | `zerg propose ls [--state 未决\|已批准\|已否决] [--json <字段>]` | 提案清单 | id,title,target,goal,evidence,rollback_ref,by,state,criterion,created_at,path,files,criterion_state,subject,subject_kind,egg_id,approver,approver_kind | 本机（无远端对应） | `host` |
| `zerg repo status` | `zerg repo status [--root <仓根>] [--json <字段>]` | 看仓脏没脏 / HEAD 在哪 / 有没有别人在写它（手敲 git status 的替身） | head,branch,path,status,untracked | 本机（无远端对应） | `host` |
| `zerg resource ledger` | `zerg resource ledger [--json <字段>]` | 资源账本（投影 /api/resources/ledger） | machine,mem_known,mem_total_gb,mem_available_gb,vram_known,gpu_pct,backend_state,fit | GET /api/resources/ledger | `host` |
| `zerg resource ls` | `zerg resource ls [<类型>] [--json <字段>]` | 资源面（投影 /api/resources/ledger 或 /api/resources/{类型}） | machine,mem_known,mem_total_gb,mem_available_gb,vram_known,gpu_pct,backend_state,fit | GET /api/resources/ledger \| /api/resources/{type} | `host` |
| `zerg resource pin` | `zerg resource pin <资源 id> [--dry-run \| --yes]` | 钉住资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg resource unpin` | `zerg resource unpin <资源 id> [--dry-run \| --yes]` | 解钉资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2） | （无机器面） | 本机（无远端对应） | `host` |
| `zerg script ls` | `zerg script ls [--json <字段>]` | 本机脚本清单（scripts/ 逐件 + 公开标记表） | path,public | 本机（无远端对应） | `host` |
| `zerg task diff` | `zerg task diff <任务 id> [--json <字段>]` | 该任务工作树的**只读** `git diff --stat` | id,workdir,diff_stat | GET /api/tasks/{id}（取 workdir）+ 本机只读 git | `host` |
| `zerg task git` | `zerg task git <任务 id> [--json <字段>]` | 该任务工作树的 git 面（分支 / HEAD / 未提交 / 与 main 的距离） | id,workdir,branch,head,dirty,ahead_of_main | GET /api/tasks/{id}（取 workdir）+ 本机只读 git | `host` |
| `zerg task logs` | `zerg task logs <任务 id> [--json <字段>]` | 该任务的日志（`/api/logs/task/{id}` **路由没接** ⇒ 不给结论，退码 8） | id,available,detail | GET /api/logs/task/{id}（处理器在 handlers.go:966 · 路由没接 ⇒ 现跑 404） | `host` |
| `zerg task ls` | `zerg task ls [--json <字段>]` | 任务队列（投影 /api/tasks） | id,status,model,machine,priority,created_at,description | GET /api/tasks | `host` |
| `zerg task show` | `zerg task show <任务 id> [--follow] [--json <字段>]` | 看单个任务（`--follow` 转发到 `zerg watch`，不另起一条流） | （无机器面） | GET /api/tasks | `host` |
| `zerg task submit` | `zerg task submit --desc <描述> [--model <模型>] [--priority <n>] [--slice-id <片>] [--depends-on <片>]… [--acceptance <判据>]… [--dry-run \| --yes]` | 提交任务（D2 写面 · 全旗标 ⇒ API 请求体逐条对上） | （无机器面） | POST /api/tasks | `host` |
| `zerg version` | `zerg version [--json <字段>]` | 单行身份（组件 版本 代码 sha 构建时间）· --json 报三层版本 | name,version,commit,build_time,contract,object_schema,core_version,core_code_sha,window | 本机（无远端对应） | `host` |
| `zerg watch` | `zerg watch [<id>] [--accept <媒体类型>] [--exit-on <kind>] [--follow]` | 订阅事件流（单端点 + Accept 协商 · 唯一名字） | （无机器面） | GET /api/events（**主控面今天没有** ⇒ 本版不给结论） | `host` |

### 危险动作（38 条 · 已登记）

> 导出物抬头逐字：`命令清单 **83** 条 · 危险动作 **38** 条（其中**已开放** 10 条 · 未开放 28 条）` —— 「哪些已开放」逐条以 `zerg help dangerous` 为准；本表四列（档 / 三态 / `--confirm` 的目标 / 它会动什么）逐字来自 `zerg help export`；未开放的那部分真跑一律**拒执**（退出码 2 = 不给结论），只有 `--dry-run` 的计划面可用。

| 命令 | 档 | 三态 | `--confirm` 的目标 | 它会动什么 | 茧壁层级 |
|---|---|---|---|---|---|
| `zerg agent bootstrap` | D3 | --dry-run · --confirm · --yes | 机器名 | 在目标机上装/起子端（**F-1 子端引导例外**：不经主控的显式命令；会改目标机状态） | `node` |
| `zerg agent load` | D2 | --dry-run · --confirm · --yes | 机器名 | 把模型装进该子端（占内存/显存槽 · 单槽机是串行的） | `node` |
| `zerg agent reap` | D3 | --dry-run · --confirm · --yes | 机器名 | 按「声明树 ↔ 现值树」差集回收闲置资源（**默认干跑**；入库件永不进候选 = 红线） | `node` |
| `zerg agent unload` | D3 | --dry-run · --confirm · --yes | 机器名 | 卸掉该子端上的模型（**在跑的任务会被打断**）· 幂等：已在未装载态按「已在该状态」报 | `node` |
| `zerg approve new` | D3 | --dry-run · --confirm · --yes | 工具名 | 签一枚批准件（逃生门）—— 只作 require_approval 的放行凭据；人不在场时等于没签 | `host` |
| `zerg calib run` | D2 | --dry-run · --confirm · --yes | 件名 | 真跑一件标定脚本（会占机器/模型槽 · 出的是实测档案）—— 命令面只转发退码，脚本本体一个字不改 | `host` |
| `zerg core reload` | D2 | --dry-run · --confirm · --yes | 主机名 | 让主控重读配置（规则表/名册）—— 生效面即时 | `host` |
| `zerg core restart` | D3 | --dry-run · --confirm · --yes | 主机名 | 停 + 起主控（**整个虫群的控制面会断一会儿**） | `host` |
| `zerg core start` | D3 | --dry-run · --confirm · --yes | 主机名 | 起主控进程（会绑端口 8580；已在跑时是**换件**前置） | `host` |
| `zerg core stop` | D3 | --dry-run · --confirm · --yes | 主机名 | 停掉主控进程（**整个虫群的控制面会断**） | `host` |
| `zerg core update` | D3 | --dry-run · --confirm · --yes | 主机名 | 换掉在跑的主控制品（**不可逆**；走 F-3 例外清单 + 验签 + 回执） | `host` |
| `zerg dev build` | D2 | --dry-run · --confirm · --yes | 候选 id | 在候选区编出成套制品（底层就是 scripts/build/build-all.sh —— 不新造第二条构建路） | `host` |
| `zerg dev edit` | D3 | --dry-run · --confirm · --yes | 提案 id | 改仓内件（写工作树）—— 作用域 = 提案声明的件；审计一行一事件；回滚 = 提案退点 + git | `host` |
| `zerg dev release` | D3 | --dry-run · --confirm · --yes | 候选 id | 把候选件推上生产面（**只能由人拍板开**；AI 不许自升） | `host` |
| `zerg dev rollback` | D3 | --dry-run · --confirm · --yes | 候选 id | 把生产面退回某个已知状态（回滚件到期前**永不自动**） | `host` |
| `zerg dev test` | D2 | --dry-run · --confirm · --yes | 候选 id | 在候选区跑测试集（底层 = make test + 门禁既有步，不新立判据） | `host` |
| `zerg doc meta` | D2 | --dry-run · --confirm · --yes | （被扫根） | 回填文件头（日期 + 不开源标注）—— 只加机械可判的抬头行、不碰正文语义；可回滚 = git | `host` |
| `zerg egg pin` | D2 | --dry-run · --confirm · --yes | 卵 id | 把该卵标成在孵（**同一时刻至多一枚**，会挤掉别的） | `node` |
| `zerg egg retire` | D3 | --dry-run · --confirm · --yes | 卵 id | 把该卵从名册与盘上退掉（**不可逆**；档 ③ 件永不自动） | `node` |
| `zerg egg unpin` | D2 | --dry-run · --confirm · --yes | 卵 id | 取消在孵标记（原本占有单槽的卵会被换下） | `node` |
| `zerg eval run` | D2 | --dry-run · --confirm · --yes | 件名 | 真跑一件评测脚本（可能要跑着的生产面 · 占机器与模型槽）—— 命令面只转发退码，脚本本体一个字不改 | `host` |
| `zerg gap add` | D2 | --dry-run · --confirm · --yes | 缺口 fp | 往真源（`<状态目录>/zerg-cli-gaps.jsonl`）追加一行 + 审计一行（可逆：删那一行 / 审计历史行不删）；审计写不进 ⇒ 真源一行不写 | `host` |
| `zerg gap verify` | D2 | --dry-run · --confirm · --yes | 缺口 id | 改真源里的 `state` / `solved_evidence` / `last_verified_at` + 审计一行（可逆：照审计那一格回写）；判红（`回归`）只在真跑那一态可达 | `host` |
| `zerg itask run` | D2 | --dry-run · --confirm · --yes | 任务 id | 立刻跑一次该内部任务（绕过它的冷却 · 会占机器） | `host` |
| `zerg itask start` | D2 | --dry-run · --confirm · --yes | （群级：开关不分机） | 让内部任务引擎开始按周期跑（会自动占机器与模型槽） | `host` |
| `zerg itask stop` | D2 | --dry-run · --confirm · --yes | （群级：开关不分机） | 让内部任务引擎停下（在跑的进化任务会跑到当前一轮为止） | `host` |
| `zerg model start` | D2 | --dry-run · --confirm · --yes | 模型 id | 把模型装载起来（占槽位 · 单槽机要排队） | `host` |
| `zerg model stop` | D3 | --dry-run · --confirm · --yes | 模型 id | 停掉该模型的驻留（**在跑任务受影响**）· 幂等优先：已停不报 500 | `host` |
| `zerg repo commit` | D2 | --dry-run · --confirm · --yes | 提交主题 | 把点名的件提交（可逆：`git reset --soft HEAD~1`）；**先跑快速档**，rc≠0 不提交（要带账放行得 `--waive <步名> --reason <…>`） | `host` |
| `zerg script inventory sync` | D2 | --dry-run · --confirm · --yes | 台账件 | 把仓外台账的行面与两个 yes/no 列按现跑重算（可逆：写前备份 + 写后读回，复查不过逐字节写回）；**一条命令写**、**不许第二条写路径** | `host` |
| `zerg script run` | D3 | --dry-run · --confirm · --yes | 脚本 | 按调用者权限执行脚本（**退码原样透传**；核心名硬占位） | `host` |
| `zerg task move` | D2 | --dry-run · --confirm · --yes | 任务 id | 改这条任务在队列里的次序（可能插到别人前面） | `host` |
| `zerg task pause` | D2 | --dry-run · --confirm · --yes | 任务 id | 把排队中的任务置为暂停态（可 resume 回来） | `host` |
| `zerg task resume` | D2 | --dry-run · --confirm · --yes | 任务 id | 把暂停的任务放回排队（可能立刻占机器） | `host` |
| `zerg task retry` | D2 | --dry-run · --confirm · --yes | 任务 id | 把 failed 任务置回 queued（会再占一次机器与模型槽） | `host` |
| `zerg task rm` | D3 | --dry-run · --confirm · --yes | 任务 id | 删除该任务的记录与它指派的工作树/分支（**不可逆**） | `host` |
| `zerg task terminate` | D3 | --dry-run · --confirm · --yes | 任务 id | 终止该任务的执行（CA 侧停 + 任务状态置 terminated）· 已产出的工作树不自动回收 | `host` |
| `zerg update` | D3 | --dry-run · --confirm · --yes | 主机名 | 按真源走一次自更新（校验 + 换件 + 回执；走 F-3 例外清单） | `host` |
<!-- ===== 生成区 END · 命令清单 ===== -->

**本区之外不写第二份命令清单**：公开面出现的每个命令名与旗标，都回到命令树这一份真源上取。

---

## 3. 写命令前先知道的约定

- **干跑优先**：凡会动状态的命令，先 `--dry-run` —— 它只出**计划件**，系统状态逐字不变（零副作用）。
- **危险动作三态**：`--dry-run`（零副作用）· `--confirm=<目标>`（值必须与目标**逐字相同**，给错值 = 拒绝，不是「当没给」）· `--yes`（第三档必须 `--confirm` 与 `--yes` 同时到）。
- **非交互 fail-closed**：没有 TTY 时**零提示词**，缺确认一律**不执行**（不会「停下来等一等」）。
- **机器面**：`--json <字段>` 必须给逗号分隔的字段名 —— 不给则退出码 1，字段清单走 stderr，stdout 零字节；`--plain` 是非 TTY 的默认档（行式输出）。
- **退出码**：0 成功 · 1 一般失败 · 2 用法错/不给结论 · 4 未认证 · 8 有 BLOCKED；逐条见 `zerg help exit-codes`。
- **凭据**：令牌只从环境变量或凭据文件读，**永不进命令行参数**。

---

## 4. 危险动作

会改机器状态的动作（装/卸模型、起停进程、换件、删任务…）在命令树里单独登记为**危险动作**，逐条带「档 · 三态 · `--confirm` 的目标是什么 · 它会动什么」。其中一部分**尚未开放**：这些真跑一律**拒执**（退出码 2 = 不给结论），只有 `--dry-run` 的计划面可用。

逐条清单与档位见 `zerg help dangerous`，或第 2 节生成区。

---

## 5. 本页从哪来（要改它先读这段）

- 第 2 节是**生成区**：由生成器照命令树现算注入（`scripts/docs/gen-cli-reference.py`）—— 改命令面就**重跑生成器**，不要手改本区。
- 第 2 节之外**不写第二份命令清单**（同上一条纪律）。
- 中英两版同构：同一份数据、同一节序；中文为唯一真相源，英文为派生翻译。
