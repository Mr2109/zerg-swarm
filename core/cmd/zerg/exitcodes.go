// exitcodes.go —— **退码表的唯一真源**（契约 §三 · 定稿 §4.1 `K3` / §九 M7 `E6`/`E8` / §十二 `P-013`）。
//
// 为什么把它写成代码而不是文档（§九 M16 `P-084`）：命令面自己就是消费者 —— `zerg` 的每个
// 出口都从这里取码，`zerg help exit-codes` 也是从这里渲染。文档只是它的导出物。
//
// 三条纪律（本文件就是它们的落点）：
//
//	① **一张表**：全仓任何「退码」声明都指向这里（6 处组件的声明行统一写「退码唯一真源 = …」）；
//	② **占号纪律**：只在 `3–63` 自留区取新号；每号写明语义与可重试性；**禁止同数字两义**；
//	③ **门禁直通不翻译**：`zerg gate run` 把脚本的码**原样转出**，不改写、不映射
//	   （`gateExitLine` 逐字取自门禁头注释里以「# 退出码：」起头的那一行，见 helptext.go）。
//	   ★ **唯一一格归一 = 脚本的 `2`**（`GAP-20260928-152` · 2026-10-23）：脚本侧 `2` = **不给结论**、
//	   命令面 `2` = **用法错** ⇒ 同一个数字两种含义（调用方分不清「我参数写错了」与「门跑了但没结论」）。
//	   ⇒ `zerg gate run` / `gate self-test` 把脚本的 `2` 折成命令面的 `8`（`blocked` · 不给结论），
//	   命令面的 `2` **只**给用法错；0/1/其余正码**照旧逐字透传**。归一只有一处实现：`gate.go` 的 `gateScriptRC`。
package main

import (
	"fmt"
	"strings"
)

type exitcodeRow struct {
	Code      int
	Name      string // 机器名（供将来的 error.kind / --json 面用）
	Meaning   string
	Retryable string
}

// exitcodeTable —— **主表**（批 B · T-10 起九档 · 命令面自己的码）。
// 批 B 落判（§十二 `P-013` ②③④ 与 §九 M7 的落地）：`10` 资源不足 · `11` 超时 ·
// `12` 不可达 · `14` 冲突/被占（**不许**再用 `507` 表达）从「已挂号」转**启用** ——
// 它们的 kind 见 `zerg help errors`（kind → 码是**唯一真源**，本表是它的投影）。
// ★ `130` 人打断（Ctrl-C）由「已挂号」转**启用**（2026-09-30 · 标注与事实对齐）：本 CLI
// 主动捕获 SIGINT 并退 `130`（落点 `main.go` 的 `exitInterrupted` + `os.Exit(exitInterrupted)`）
// ⇒ 它**早已是启用行为**，此前误挂在「已挂号、尚未启用」段（`kind` = `interrupted`）。
//
// ★ `3` 部分趟（`partial_ok`）—— D15 / 《设计-门禁部分趟-v0.7-20261004》§五 拍板启用：
// **只跑子集且子集全绿** ⇒ **不覆盖全量**（在 0/1/2 之外 ⇒ 任何「判色」都不会把它误读成全量绿）；
// **不可重试**。登记面（`namespaces[cli].table` ＋ `collisions[3]` 改判）同批落，见 `exitcode-namespaces.json`。
var exitcodeTable = []exitcodeRow{
	{0, "ok", "成功", "—"},
	{1, "failed", "一般失败（跑到了、没成功）", "看 error.kind；默认不可重试"},
	{2, "usage", "用法错（**只**这一义：未知旗标 / 步名不在 / 值域不认 / 组合不合法 —— 命令面在 exec 之前就拒）", "改用法后可重试"},
	{3, "partial_ok", "部分趟·只跑子集且子集全绿·不覆盖全量", "不可重试"},
	{4, "unauthenticated", "未认证（缺令牌 / 令牌不对；`403` 与它并码 · kind 分家）", "换凭据后可重试"},
	{8, "blocked", "不给结论：有 BLOCKED（「读不到」不许当健康）／门禁脚本自身不给结论（脚本的 `2` 经 `gate run` 归此处 · `GAP-20260928-152`）", "补齐前置后可重试"},
	{10, "insufficient_resource", "资源不足（`507` 的对面：卸掉占用者后可重试）", "等待 / 卸占用者后可重试"},
	{11, "timeout", "超时（「等到了坏结果」与「等不起」分开）", "可重试"},
	{12, "unreachable", "不可达（打不到主控）—— **fail-closed**，不偷偷换端点", "链路恢复后可重试"},
	{14, "conflict", "冲突 / 被占（**不许**再用 `507` 表达 · 幂等优先）", "显式 `--wait` 才排队；已在该状态按状态报"},
	{130, "interrupted", "人打断（Ctrl-C · 默认只退订、不取消）", "—"},
}

// exitcodeReserved —— **已挂号、尚未启用**（登记在此，不启用；启用是后续版本的拍板项）。
// ★ 现读为空：此前唯一挂号项 `130`（`interrupted`）已随主表转**启用**（见上）。
var exitcodeReserved = []exitcodeRow{}

// exitcodePolicy —— 占号纪律（§九 M7 `E8`）。一字不改地用在一处，别处只许引用。
const exitcodePolicy = "占号纪律：只在 3–63 自留区取号；每号必须在 `zerg help exit-codes` 写明语义与可重试性；**禁止同数字两义**（同数字两义 ⇒ 门禁红）"

// 门禁侧（脚本）自己的码只有 0/1/2 + 四档；命令面**直通不翻译**：0/1 两表逐字同义，
// **2 不同义**（脚本 = 不给结论 · 命令面 = 用法错）⇒ 只有这一格在 `gate run` / `gate self-test`
// 上归一（脚本 2 ⇒ 命令面 8 · 落点 `gate.go` 的 `gateScriptRC` · `GAP-20260928-152`）。
const exitcodeGatePass = "门禁侧 0/1 与命令面同码同义（`zerg gate run` 原样转出）；**门禁侧的 2 = 不给结论** ⇒ 经 `gate run` 归命令面的 8，命令面的 2 **只**给用法错（唯一一格归一 · `GAP-20260928-152`）"

// exitcodeJudgeLine —— 「判不了」档的口径（**甲案** · 2026-09-28 拍板 · 缺口 `GAP-20260928-46`）。
//
// 为什么要有它：同一句「判不了 / 不给结论」过去在不同件里分别退 `2`（用法错）与 `8`（blocked），
// 两档语义**重叠** ⇒ 「空串参该落哪一档」无明文，各件自作主张（自定口径）。本行把它定成一句：
// **参数值无效 / 空串 / 缺实参 ⇒ 用法码 `2`**（主表 `2` = 用法错）；
//
//	**`8` 专留给「不给结论」**（有 BLOCKED · 读不到不许当健康 · 门禁脚本自身不给结论）。★ 本行只**声明口径**，不改任何既有码值。
const exitcodeJudgeLine = "「判不了」档口径（甲案 · 2026-09-28 拍板 · 2026-10-23 补 `GAP-20260928-152`）：参数值无效 / 空串 / 缺实参 ⇒ 用法码 2；8 专留给「不给结论」（有 BLOCKED / 门禁脚本自身不给结论 —— 脚本的 2 经 `gate run` 归 8）"

// exitCodeOf —— 按**机器名**从退码表取码（自描述面「**取自表**」的落点）。
//
// 为什么要有它（`K2` 甲档归一 · 2026-09-24）：归一的**方向写死** —— 退码表是唯一真源、
// 一个字不动；凡要在文案里写码（帮助面 / 契约面 / 漏斗），一律**回表里取**，不许把
// 数字抄进字符串。抄进去的那一份没人看着 ⇒ 表改了它悄悄跟丢（`Q-146` 的病根形态：
// 表说 `usage`(2)、实现与文案写 `1`，「同一错误不同码」）。
//
// 表里查不到 ⇒ 兜底 `exitFail`（构造上到不了：门⑨ 逐格对拍 `main.go` 的常量 ⟷ 本表）。
func exitCodeOf(name string) int {
	for _, rows := range [][]exitcodeRow{exitcodeTable, exitcodeReserved} {
		for _, r := range rows {
			if r.Name == name {
				return r.Code
			}
		}
	}
	return exitFail
}

func helpExitCodes() string {
	out := "退码表（**唯一真源** · 契约 §三 · `zerg help exit-codes` 就是它的自描述面）\n\n"
	emit := func(title string, rows []exitcodeRow) {
		out += title + "\n"
		for _, r := range rows {
			code := fmt.Sprintf("%d", r.Code)
			out += "  " + pad(code, 3) + "  " + pad(r.Name, 17) + "  " + pad(r.Meaning, 40) + "  可重试性：" + r.Retryable + "\n"
		}
	}
	emit("主表（五档）：", exitcodeTable)
	out += "\n" + exitcodePolicy + "\n"
	out += "\n" + exitcodeGatePass + "\n"
	out += "  门禁直通档（0/1 逐字转出 · 2 归一）：" + gateExitLine + "\n"
	out += "    ★ 归一那一格（`GAP-20260928-152`）：脚本的 `2`（不给结论）经 `zerg gate run` / `gate self-test` 归命令面 `8`；\n" +
		"      命令面的 `2` 只给**用法错**（命令面在 exec **之前**就拒：未知旗标 / 步名不在 / 值域不认 / 组合不合法）—— 脚本一次都没跑。\n"
	out += "  门禁步骤四档（脚本内部 · 不进退出码）：PASS／FAIL（失败项）／BLOCKED（不给结论）／REPORT（只报告）\n"
	out += "    档位映射：`tri` ⇒ 0=PASS · 1=FAIL · 2=BLOCKED · 其它码=FAIL；`tri-report` 与 `tri` **只差 rc=1 那一格**（rc=1 ⇒ REPORT：红只入清单、不计失败项、不计 BLOCKED、**不影响退出码**）；`rc`／`empty` 两模式语义不变。\n"
	out += "\n一份口径（防误读）：**组件的自有码空间**（`zerg-compat` / `zerg-model` / `zerg-agent` 的进程内语义码）" +
		"与命令面这张表**不是同一张表** —— 命令面只把它们**原样转出**；值面对齐要先给它们的自有语义重新取号，" +
		"属 `P-013` 的拍板项（差集逐条登记在《开工记录-批A-20260920.md》T-04 节）。\n"
	out += "**`130` 消歧（防误读）**：本 CLI 主动捕获 SIGINT 并退 `130`，与惯例 `128+2`（`128` 基 + 信号号 `2`）是同一件事。\n"
	out += "\n" + exitcodeJudgeLine + "\n"
	return out
}

// pad 按**显示宽度**补空格（中文算 2 格）——只为对齐，不改值。
func pad(s string, w int) string {
	n := displayWidth(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

func helpConfig() string {
	out := "配置优先级（契约 §八 · §4.1 `K10` · clig.dev 五级序）：\n\n"
	out += "  " + configPriorityLine + "\n\n"
	out += "逐级说明（左高右低，高者覆盖低者）：\n"
	out += "  1. **旗标**            本次调用显式给的那一个（`--node` / `--outdir` …）\n"
	out += "  2. **`ZERG_*` 环境变量**  如 `ZERG_PORT` / `ZERG_TOKEN` / `ZERG_STATE_DIR` / `ZERG_REPO`\n"
	out += "     · `ZERG_REPO` **换仓根**（无条件采信 · 不校验）：指到非仓根 ⇒ `gate run`/`gate ls` 硬错 rc=2 · `repo status`/`repo commit` 硬错 rc=8 · **`doctor` 会把所指位置报成「仓库根 PASS」**；同族脚本侧旋钮 `ZERG_REPO_ROOT`。写面/门禁动作请 `env -u ZERG_REPO`。\n"
	out += "  3. **项目 `.env`**      仓内 `.env`（不入库；令牌字段一律掩码）\n"
	out += "  4. **`~/.zerg/config.yaml`** 用户级默认（档位、主控地址、默认机器）\n"
	out += "  5. **内置默认**         主控 `127.0.0.1:8580` / 网关 `8082` / 子端 `8100`（与 `core/internal/statepath` 同一份真源）\n\n"
	out += "凭据（契约 §八 · §4.1 `K11` · §九 M2）：\n"
	out += "  · 令牌**永不进 `argv`**（进程列表里看得到命令行 ⇒ 不许从命令行传令牌）\n"
	out += "  · 令牌**永不入库**；非交互场景用 `--token-stdin`\n"
	out += "  · `context show` / `context ls` 一律**掩码**\n"
	out += "  · 缺令牌 ⇒ 退码 `4`（未认证），且**不偷偷降级**（§九 M20 `O3`）\n"
	return out
}
