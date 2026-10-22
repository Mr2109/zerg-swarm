package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ── `zerg context` 族（P1 · 上下压缩的可回查面）────────────────────────────
//
// 口径（设计稿 v5 §6）：
//   - 一切从 `zerg` 打得着（✗ 不另开独立入口）；
//   - 本族只做**只读呈现与取回**，不做压缩决策 —— 压缩由引擎在对话/任务侧进行；
//   - `compact` 暂不提供：CLI 侧没有「压缩目标」（会话/任务上下文），
//     硬造只能是假命令 ⇒ 挂账等有目标那天（设计稿 v5 §6 注）。
const (
	ctxUnloadDirName = "ctx-unload"
	ctxAuditFileName = "compress-audit.jsonl"
	ctxShowMaxList   = 5
)

var contextStatsFields = []string{"dir", "count", "bytes", "events"}

func ctxUnloadDir() string { return statepath.File(ctxUnloadDirName) }
func ctxAuditPath() string { return statepath.File(ctxAuditFileName) }

// cmdContext —— `zerg context <stats|show>` 的分派壳（两条动作共用）。
func cmdContext(inv *invocation, stdout, stderr io.Writer) int {
	action := ""
	if len(inv.path) > 0 {
		action = inv.path[len(inv.path)-1]
	}
	switch action {
	case "stats":
		return contextStats(inv, stdout, stderr)
	case "show":
		return contextShow(inv, stdout, stderr)
	}
	inv.setErr("usage", "context_action_unknown:"+action, "未知动作")
	fmt.Fprintf(stderr, "%s: `context` 只认 stats / show（收到 %q）\n", progName, action)
	return exitUsage
}

// contextStats —— 卸载面与审计面的只读读数。
func contextStats(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "`context stats` 不收位置参数")
		fmt.Fprintf(stderr, "%s: `context stats` 不收位置参数（多给 %d 个）⇒ 用法错 2\n", progName, len(inv.args))
		return exitUsage
	}
	for _, f := range inv.fields {
		if !ctxFieldLegal(f) {
			inv.setErr("usage", "json_field_unknown:"+f, "未知字段")
			fmt.Fprintf(stderr, "%s: `context stats --json` 不认字段 %q；可用：%s\n",
				progName, f, strings.Join(contextStatsFields, " · "))
			return exitUsage
		}
	}

	dir := ctxUnloadDir()
	count, bytes := 0, int64(0)
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
				continue
			}
			info, ierr := e.Info()
			if ierr != nil {
				continue
			}
			count++
			bytes += info.Size()
		}
	}
	events := ctxCountAuditEvents()

	// 字段面：`--json` 不给字段 = 闭集全量
	want := map[string]bool{}
	if inv.jsonGiven && len(inv.fields) == 0 {
		for _, f := range contextStatsFields {
			want[f] = true
		}
	}
	for _, f := range inv.fields {
		want[f] = true
	}

	if inv.jsonGiven {
		obj := map[string]any{}
		if want["dir"] {
			obj["dir"] = dir
		}
		if want["count"] {
			obj["count"] = count
		}
		if want["bytes"] {
			obj["bytes"] = bytes
		}
		if want["events"] {
			obj["events"] = events
		}
		b, err := json.Marshal(obj)
		if err != nil {
			fmt.Fprintf(stderr, "%s: JSON 序列化失败：%v\n", progName, err)
			return exitFail
		}
		fmt.Fprintln(stdout, string(b))
		return exitOK
	}

	fmt.Fprintf(stdout, "卸载件：%d · 合计 %d 字节\n", count, bytes)
	fmt.Fprintf(stdout, "落点：%s\n", dir)
	fmt.Fprintf(stdout, "审计事件：%d（%s · 每次卸载一行 · 只追加）\n", events, ctxAuditPath())
	return exitOK
}

// contextShow —— 按 id 取回卸载原文（`show` 是「可逆」的落地）。
func contextShow(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) != 1 {
		inv.setErr("usage", "arity", "`context show` 要一个 id")
		fmt.Fprintf(stderr, "%s: `context show` 要**恰一个** id（收到 %d 个）⇒ 用法错 2\n", progName, len(inv.args))
		fmt.Fprintf(stderr, "   形态：`zerg context show <sha16>`（id 见 `zerg context stats` 的落点目录）\n")
		return exitUsage
	}
	id := strings.TrimSpace(inv.args[0])
	id = strings.TrimSuffix(id, ".txt")
	// 防越界：只认单段文件名
	if id == "" || strings.ContainsAny(id, "/\\") || id == "." || id == ".." {
		inv.setErr("usage", "ctx_id_illegal", "id 不合法")
		fmt.Fprintf(stderr, "%s: id 只认单段文件名（禁路径/盘符/..）：%q\n", progName, id)
		return exitUsage
	}

	full := filepath.Join(ctxUnloadDir(), id+".txt")
	b, err := os.ReadFile(full)
	if err != nil {
		inv.setErr("usage", "ctx_unload_missing", "卸载件不在盘")
		fmt.Fprintf(stderr, "%s: 卸件 %q 不在盘（%s）\n", progName, id, full)
		if list := ctxUnloadSampleIDs(ctxShowMaxList); len(list) > 0 {
			fmt.Fprintf(stderr, "   现有（前 %d 个）：%s\n", len(list), strings.Join(list, " · "))
		} else {
			fmt.Fprintf(stderr, "   当前没有任何卸载件（`zerg context stats` 可看读数）\n")
		}
		return exitUsage
	}
	if _, werr := stdout.Write(b); werr != nil {
		fmt.Fprintf(stderr, "%s: 写 stdout 失败：%v\n", progName, werr)
		return exitFail
	}
	return exitOK
}

func ctxFieldLegal(f string) bool {
	for _, x := range contextStatsFields {
		if x == f {
			return true
		}
	}
	return false
}

// ctxCountAuditEvents —— 审计行数（解析失败的行不计，但整件可读失败也返回 0）。
func ctxCountAuditEvents() int {
	f, err := os.Open(ctxAuditPath())
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	dec := json.NewDecoder(f)
	for {
		var ev map[string]any
		if err := dec.Decode(&ev); err != nil {
			break
		}
		n++
	}
	return n
}

// ctxUnloadSampleIDs —— 取前 n 个 id（按名字排序，稳定可复现）。
func ctxUnloadSampleIDs(n int) []string {
	ents, err := os.ReadDir(ctxUnloadDir())
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".txt"))
	}
	sort.Strings(ids)
	if len(ids) > n {
		ids = ids[:n]
	}
	return ids
}
