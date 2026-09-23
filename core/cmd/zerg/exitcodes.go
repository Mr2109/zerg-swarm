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
//	   （`gateExitLine` 逐字取自门禁头注释第 35 行，见 helptext.go）。
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
var exitcodeTable = []exitcodeRow{
	{0, "ok", "成功", "—"},
	{1, "failed", "一般失败（跑到了、没成功）", "看 error.kind；默认不可重试"},
	{2, "usage", "用法错 / **不给结论**", "改用法后可重试；「不给结论」不许当失败计"},
	{4, "unauthenticated", "未认证（缺令牌 / 令牌不对；`403` 与它并码 · kind 分家）", "换凭据后可重试"},
	{8, "blocked", "有 BLOCKED（「读不到」不许当健康）", "补齐前置后可重试"},
	{10, "insufficient_resource", "资源不足（`507` 的对面：卸掉占用者后可重试）", "等待 / 卸占用者后可重试"},
	{11, "timeout", "超时（「等到了坏结果」与「等不起」分开）", "可重试"},
	{12, "unreachable", "不可达（打不到主控）—— **fail-closed**，不偷偷换端点", "链路恢复后可重试"},
	{14, "conflict", "冲突 / 被占（**不许**再用 `507` 表达 · 幂等优先）", "显式 `--wait` 才排队；已在该状态按状态报"},
}

// exitcodeReserved —— **已挂号、尚未启用**（登记在此，不启用；启用是后续版本的拍板项）。
var exitcodeReserved = []exitcodeRow{
	{130, "interrupted", "人打断（Ctrl-C · 默认只退订、不取消）", "—"},
}

// exitcodePolicy —— 占号纪律（§九 M7 `E8`）。一字不改地用在一处，别处只许引用。
const exitcodePolicy = "占号纪律：只在 3–63 自留区取号；每号必须在 `zerg help exit-codes` 写明语义与可重试性；**禁止同数字两义**（同数字两义 ⇒ 门禁红）"

// 门禁侧（脚本）自己的码只有 0/1/2 + 四档；命令面**直通不翻译**，两表在 0/1/2 上逐字一致。
const exitcodeGatePass = "门禁侧 0/1/2 与命令面同码同义（`zerg gate run` 原样转出，不做映射）"

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
	out += "\n"
	emit("已挂号、尚未启用（登记在此，不启用）：", exitcodeReserved)
	out += "\n" + exitcodePolicy + "\n"
	out += "\n" + exitcodeGatePass + "\n"
	out += "  门禁直通档（不翻译）：" + gateExitLine + "\n"
	out += "\n一份口径（防误读）：**组件的自有码空间**（`zerg-compat` / `zerg-model` / `zerg-agent` 的进程内语义码）" +
		"与命令面这张表**不是同一张表** —— 命令面只把它们**原样转出**；值面对齐要先给它们的自有语义重新取号，" +
		"属 `P-013` 的拍板项（差集逐条登记在《开工记录-批A-20260920.md》T-04 节）。\n"
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
