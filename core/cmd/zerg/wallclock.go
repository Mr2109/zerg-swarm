// wallclock.go —— 命令面**墙钟上界**（序138 · 组4 §二.4 `W-57`（`研-禁:216` §六 栗④）·
// 上级裁定 **B**：超时一律退 `11`）。
//
// 为什么要有它（判据栏逐字「命令面有墙钟上界旗标；超时 ⇒ 先复原（负控：超时后盘上有残留 ⇒ 判红）」）：
// 病灶是「一条命令挂住就放弃」这一格**今天没有** —— `--timeout <时长>` 这枚旗标
// （`main.go` 的 `valueFlagName` 早已收值）**收得下、不生效**（旧状：一枚消费者都没有）⇒
// 「给错了」被读成「给对了」（脚本 / agent 以为设了上界，实际没有）。
//
// 三半（判据栏逐条 · 一条都不少）：
//
//	① **旗标接消费者**：`--timeout <时长>` 全局通吃（谁用谁读）· 非法时长 ⇒ **先于任何动作**退 `2`；
//	② **先复原再报**：到点 ⇒ ① 杀掉**本趟亲手起的子进程** ② 把本趟登记过的**写面前像**逐件还原
//	   ⇒ **再**报（「报」在「复原」之后，不许先报后复原）；
//	③ **负控**：到点后盘上有残留 ⇒ 判红（成对判据件 `cli_wallclock_test.go`）。
//
// 退码：**`11`**（`exitTimeout` · `exitcodes.go` 的 `{11, "timeout", …}`）。
// 为什么不是 `2`：那枚 `2` 的来历是**门（脚本）侧**设计（`C7`：门侧码空间只有 0/1/2 + 四档），
// 而**命令面**退码表逐字写「超时走 `11`」（`exitcodes.go` 主表 + `main.go` 常量 +
// `errors.go` 的 `timeout` kind + `longtask.go` 自描述面同句）⇒ 命令面照**自家表**取码（裁定 B）。
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// wallclockFlag —— 那枚旗标名（解析表的唯一真源在 `main.go` 的 `valueFlagName`）。
const wallclockFlag = "--timeout"

// parseWallclock 把「时长」串读成 time.Duration。
//
// 认 `time.ParseDuration` 的全部写法（`30s` / `2m` / `1h` / `1.5s` / `500ms`），
// 另认**裸数字**（按**秒**读 —— `90` == `90s`）。空串 / 读不出 / ≤ 0 ⇒ 用法错（退 `2`）。
func parseWallclock(s string) (time.Duration, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, fmt.Errorf("`%s` 要一个时长（例 `30s` / `2m` / `90`；裸数字按**秒**读）", wallclockFlag)
	}
	d, err := time.ParseDuration(t)
	if err != nil {
		if n, convErr := strconv.Atoi(t); convErr == nil {
			d, err = time.Duration(n)*time.Second, nil
		}
	}
	if err != nil {
		return 0, fmt.Errorf("`%s` 的时长读不出来：%q（例 `30s` / `2m` / `90`；裸数字按**秒**读）", wallclockFlag, s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("`%s` 的时长必须为正：%q", wallclockFlag, s)
	}
	return d, nil
}

// preImage —— 一件**写面前像**（「先复原再报」的 ② 那一半的单元格）。
//
//	Existed = true   ⇒ 写前在盘：原样写回（内容 + 权限位）
//	Existed = false  ⇒ 写前不在盘：到点把残留**删掉**（这一步就是负控的靶心）
type preImage struct {
	Path    string
	Existed bool
	Data    []byte
	Mode    os.FileMode
}

var (
	wallclockMu      sync.Mutex
	wallclockJournal []preImage
)

// wallclockRecordPreImage 在**动手写之前**记下一件的现盘样子（不在盘也记 —— 复原时删残留）。
// 真消费者见 `family_dev_edit.go`（`dev edit` 受控写入）与 `family_doc.go`（`doc meta fill`）。
func wallclockRecordPreImage(path string) error {
	wallclockMu.Lock()
	defer wallclockMu.Unlock()
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			wallclockJournal = append(wallclockJournal, preImage{Path: path})
			return nil
		}
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s 不是普通件（不登记前像）", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	wallclockJournal = append(wallclockJournal, preImage{
		Path: path, Existed: true, Data: b, Mode: fi.Mode().Perm(),
	})
	return nil
}

// wallclockJournalReset 清空登记本（每一趟命令开跑前由 dispatch 调一次 —— 一趟一本，不跨趟）。
func wallclockJournalReset() {
	wallclockMu.Lock()
	wallclockJournal = nil
	wallclockMu.Unlock()
}

// wallclockRestoreAll 复原本趟登记过的每一件（**逆序**：后写的先还），返回逐条动作说明
// （供 stderr 复读 —— 「报」在「复原」之后，说明里带每一件）。
// ★ 本来不在盘的 ⇒ **删掉残留**（删不掉也要照实写出来，不许静默成功）。
func wallclockRestoreAll() []string {
	wallclockMu.Lock()
	defer wallclockMu.Unlock()
	notes := make([]string, 0, len(wallclockJournal))
	for i := len(wallclockJournal) - 1; i >= 0; i-- {
		p := wallclockJournal[i]
		if !p.Existed {
			switch err := os.Remove(p.Path); {
			case err == nil:
				notes = append(notes, p.Path+" ← 写前不在盘 ⇒ 删掉残留")
			case os.IsNotExist(err):
				notes = append(notes, p.Path+" ← 写前不在盘 · 盘上也没有（零残留）")
			default:
				notes = append(notes, p.Path+" ← **删残留失败**："+err.Error())
			}
			continue
		}
		if err := os.WriteFile(p.Path, p.Data, p.Mode); err != nil {
			notes = append(notes, p.Path+" ← **还原失败**："+err.Error())
			continue
		}
		notes = append(notes, p.Path+" ← 还原前像（"+strconv.Itoa(len(p.Data))+" 字节）")
	}
	wallclockJournal = nil
	return notes
}

// wallclockRestoreFn —— 复原动作的**调用点**（成对负控留缝：把它换成空档 ⇒ 残留留下 ⇒ 正控当场红）。
// 生产路径恒 = `wallclockRestoreAll`；只有判据件会替它。
var wallclockRestoreFn = wallclockRestoreAll

// wallclockGuard 给「一条命令」加上墙钟上界：到点 ⇒ **先复原再报** ⇒ 退 `11`。
// 命令在自己的 goroutine 里跑；到点这一档**不等它**（等不起就是等不起 —— 与「等到了坏结果」异码）。
func wallclockGuard(d time.Duration, run func() int, stderr io.Writer) int {
	done := make(chan int, 1)
	go func() { done <- run() }()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case rc := <-done:
		return rc
	case <-timer.C:
		// ① 杀掉**本趟亲手起的**子进程（不杀进程组、不碰别人 —— 与 `cancelRunningChild` 同一条纪律）。
		cancelRunningChild()
		// ② 把本趟登记过的写面前像逐件还原。
		notes := wallclockRestoreFn()
		fmt.Fprintf(stderr, "%s: 墙钟到点（%s）—— **先复原再报**：本趟亲手起的子进程已停 · 写面前像复原 %d 件\n",
			progName, d, len(notes))
		for _, n := range notes {
			fmt.Fprintf(stderr, "  · %s\n", n)
		}
		fmt.Fprintf(stderr, "  ⇒ 退码 %d（**超时** · 「等不起」与「等到了坏结果」异码 —— `%s help long-tasks`）\n",
			exitTimeout, progName)
		return exitTimeout
	}
}

// wallclockTimeoutError —— 到点那一格的机器面（`--json` 失败包封用；kind 取自 `errors.go` 闭集）。
func wallclockTimeoutError(d time.Duration) *cliError {
	return &cliError{
		Kind:    kindForExitCode(exitTimeout),
		Detail:  "wallclock_deadline",
		Message: fmt.Sprintf("墙钟到点（%s）：已先复原再报（§九 M8「等不起」与「等到了坏结果」异码）", d),
	}
}
