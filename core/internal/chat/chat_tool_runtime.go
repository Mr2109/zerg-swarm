// chat_tool_runtime.go — 渐进式常驻工具运行时（P4-50——设计-渐进式常驻工具系统-20260902）
// 机制: 初始无常驻只有查询 → 成功即常驻 → 连续 3 次 exec 失败隐藏 → 上限 10 LRU
// 成败铁律（Mr2109点破）: 只看执行层——工具报错(exec)=失败/参数错(param)=调用方锅不隐藏/有输出=成功
// 错误桶: 全局聚合（工具资产进化数据——驱动下版优化——上限 100 桶/工具/类——90 天老化）

package chat

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"zerg/core/internal/agent"
	"log"
)

// NormalizeToolArgs — 工具参数统一解包（P4-50 bash arguments 双层嵌套修复）
// 现象: 模型把 Hermes 风格 {name, arguments} 整个塞进 chat 格式 function.arguments——
//       parseToolArgs 解一层后 Args = {"arguments":"{\"command\"...}","name":"bash"}——工具拿不到真参
// 处理: ①Args 含 "arguments" 键（字符串 JSON 或对象）→ 解包它覆盖 ②剔除混入的元键（name/type/function）
// 注: 所有工具执行路径统一调用（RunToolLoop + chat_handlers SSE + agent loop）
func NormalizeToolArgs(tc *agent.ToolCall) {
	if tc == nil {
		return
	}
	// 情况 1: Args 里有 "arguments" 键（模型嵌套——真正参数在它里面）
	if v, ok := tc.Args["arguments"]; ok {
		switch vv := v.(type) {
		case string:
			var m map[string]any
			if json.Unmarshal([]byte(vv), &m) == nil {
				tc.Args = m
			}
		case map[string]any:
			tc.Args = vv
		}
	}
	// 情况 2: 剔除混入的元键（顶层 name/type/function——不该是参数）
	for _, k := range []string{"name", "type", "function"} {
		delete(tc.Args, k)
	}
}


// 渐进常驻配置
const (
	MaxResident   = 10  // 常驻工具上限（Mr2109）
	FailHidden    = 3   // 连续 exec 失败次数 → 隐藏
	MaxErrBuckets = 100 // 错误桶上限/工具/类（Mr2109——后期自动任务消融）
	ErrRetention  = 90 * 24 * time.Hour
)

// ToolRuntime — 对话/任务级渐进式常驻工具状态（不跨会话背债——活跃期内存）
type ToolRuntime struct {
	mu       sync.Mutex
	resident []string       // 常驻工具名（≤MaxResident——尾=最新）
	failSeq  map[string]int // 工具连续 exec 失败数
	hidden   map[string]bool
}

// NewToolRuntime — 初始无常驻（设计 3.1 规则 1）
func NewToolRuntime() *ToolRuntime {
	return &ToolRuntime{failSeq: map[string]int{}, hidden: map[string]bool{}}
}

// Resident — 常驻工具快照（尾=最新——注入提示词用）
func (rt *ToolRuntime) Resident() []string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := make([]string, len(rt.resident))
	copy(out, rt.resident)
	return out
}

// AddResident — 成功调用 → 常驻（已有则移到尾=最新——超上限挤掉头=最老 LRU）
func (rt *ToolRuntime) AddResident(name string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	// 已在 → 移到尾（刷新 LRU）
	for i, n := range rt.resident {
		if n == name {
			rt.resident = append(rt.resident[:i], rt.resident[i+1:]...)
			rt.resident = append(rt.resident, name)
			return
		}
	}
	rt.resident = append(rt.resident, name)
	if len(rt.resident) > MaxResident {
		rt.resident = rt.resident[len(rt.resident)-MaxResident:] // 挤掉最老
	}
}

// RemoveResident — 主动移出常驻（3 次 exec 失败——设计 3.3）
func (rt *ToolRuntime) RemoveResident(name string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	for i, n := range rt.resident {
		if n == name {
			rt.resident = append(rt.resident[:i], rt.resident[i+1:]...)
			break
		}
	}
}

// RecordOutcome — 记录工具调用结果（钩子统一入口）
// errType: ""=成功(有输出) / "exec"=工具执行错误 / "param"=参数错误(调用方锅——不隐藏)
// 返回: 提示词附加消息（2 次失败轻提示——3 次隐藏通知）——空=无动作
func (rt *ToolRuntime) RecordOutcome(name, errType, errMsg string) string {
	if errType == "" {
		// 成功（有输出）——failSeq 清零——保持/新常驻
		rt.mu.Lock()
		rt.failSeq[name] = 0
		rt.mu.Unlock()
		rt.AddResident(name)
		return ""
	}
	if errType == "param" {
		// 参数错——调用方锅——不隐藏（工具无错——报错信息已带引导）
		return ""
	}
	// exec 失败——连续计数
	rt.mu.Lock()
	rt.failSeq[name]++
	n := rt.failSeq[name]
	rt.mu.Unlock()
	if n >= FailHidden {
		rt.RemoveResident(name)
		rt.mu.Lock()
		rt.hidden[name] = true
		rt.mu.Unlock()
		return fmt.Sprintf("【系统】工具 %s 已连续 %d 次执行失败——已从你的常驻工具中移除并隐藏（本对话不再推荐——除非无其他可选）。请换其他工具完成目标。", name, n)
	}
	if n == FailHidden-1 {
		return fmt.Sprintf("【系统】工具 %s 已连续 %d 次执行失败——若再失败将移出常驻并隐藏。请考虑换工具。", name, n)
	}
	return ""
}

// IsHidden — 查询/注入时过滤（本对话隐藏的工具）
func (rt *ToolRuntime) IsHidden(name string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.hidden[name]
}

// HiddenList — 隐藏工具列表（tool_search 过滤用——无其他可选时豁免）
func (rt *ToolRuntime) HiddenList() []string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	var out []string
	for n := range rt.hidden {
		out = append(out, n)
	}
	return out
}

// ===== 全局错误桶（工具资产进化数据——跨对话持久——4.x 设计） =====

type errBucket struct {
	Msg        string         `json:"msg"`
	Count      int            `json:"count"`
	First      int64          `json:"first"`
	Last       int64          `json:"last"`
	SampleArgs map[string]any `json:"sample_args,omitempty"`
}

type toolErrorStore struct {
	mu    sync.Mutex
	Tools map[string]map[string]map[string]*errBucket `json:"tools"` // tool → type(param/exec) → hash
}

var (
	errStore     = &toolErrorStore{Tools: map[string]map[string]map[string]*errBucket{}}
	errStoreFile = "/tmp/zerg-tool-errors.json"
	errStoreInit sync.Once
)

// LoadToolErrors — 启动加载（main.go 调用——失败静默——空文件也 OK）
func LoadToolErrors() {
	errStoreInit.Do(func() {
		data, err := os.ReadFile(errStoreFile)
		if err != nil {
			return
		}
		_ = json.Unmarshal(data, errStore)
		if errStore.Tools == nil {
			errStore.Tools = map[string]map[string]map[string]*errBucket{}
		}
	})
}

// errTypeOf — 启发式参数错误分类（记录层——非关键路径——消息含参数格式提示 → param）
func ErrTypeOf(msg string) string {
	m := strings.ToLower(msg)
	if strings.Contains(m, "参数") || strings.Contains(m, "不能为空") || strings.Contains(m, "格式") || strings.Contains(m, "unknown tool") || strings.Contains(m, "未知工具") {
		return "param"
	}
	return "exec"
}

// RecordToolError — 工具错误入桶（同工具+同类型+同消息 hash → 聚合计数——不逐条存）
// sampleArgs: 模型传参样本（param 错——驱动 schema/描述优化）
func RecordToolError(tool, msg string, args map[string]any) {
	if tool == "" || msg == "" {
		return
	}
	typ := ErrTypeOf(msg)
	sum := sha1.Sum([]byte(msg))
	key := hex.EncodeToString(sum[:6])
	now := time.Now().Unix()

	errStore.mu.Lock()
	defer errStore.mu.Unlock()
	if errStore.Tools[tool] == nil {
		errStore.Tools[tool] = map[string]map[string]*errBucket{}
	}
	if errStore.Tools[tool][typ] == nil {
		errStore.Tools[tool][typ] = map[string]*errBucket{}
	}
	b := errStore.Tools[tool][typ][key]
	if b == nil {
		// 超上限——丢最老（last 最早）
		if len(errStore.Tools[tool][typ]) >= MaxErrBuckets {
			var oldestKey string
			var oldestLast int64 = 1 << 62
			for k, v := range errStore.Tools[tool][typ] {
				if v.Last < oldestLast {
					oldestLast = v.Last
					oldestKey = k
				}
			}
			delete(errStore.Tools[tool][typ], oldestKey)
		}
		b = &errBucket{Msg: msg, First: now, SampleArgs: args}
		errStore.Tools[tool][typ][key] = b
	}
	b.Count++
	b.Last = now
	// 90 天老化（last 到龄——清理）
	for t, types := range errStore.Tools {
		for tp, buckets := range types {
			for k, v := range buckets {
				if time.Since(time.Unix(v.Last, 0)) > ErrRetention {
					delete(buckets, k)
				}
			}
			if len(buckets) == 0 {
				delete(types, tp)
			}
		}
		if len(types) == 0 {
			delete(errStore.Tools, t)
		}
	}
	saveToolErrors()
}

// saveToolErrors — 落盘（小文件——每错即写可接受——或后期攒批）
func saveToolErrors() {
	data, err := json.Marshal(errStore)
	if err != nil {
		log.Printf("[chat] 错误桶序列化失败: %v", err)
		return
	}
	// 2026-09-03 代码优化(skill 日志查漏): 落盘失败原静默——错误桶是工具进化依据——丢失无痕——记日志
	if err := os.WriteFile(errStoreFile, data, 0o644); err != nil {
		log.Printf("[chat] 错误桶落盘失败: %v", err)
	}
}

// ToolErrorsSummary — 查工具错误统计（tool_errors 工具用）
// 空 tool = 全局 Top（按 count 降序）
func ToolErrorsSummary(tool string) string {
	errStore.mu.Lock()
	defer errStore.mu.Unlock()
	var lines []string
	now := time.Now().Unix()
	if tool == "" {
		// 全局 Top（跨工具——按 count）
		var all []struct {
			Tool, Typ, Msg string
			Count          int
			Last           int64
		}
		for t, types := range errStore.Tools {
			for tp, buckets := range types {
				for _, v := range buckets {
					all = append(all, struct {
						Tool, Typ, Msg string
						Count          int
						Last           int64
					}{t, tp, v.Msg, v.Count, v.Last})
				}
			}
		}
		sort.Slice(all, func(i, j int) bool { return all[i].Count > all[j].Count })
		if len(all) == 0 {
			return "（无工具错误记录）"
		}
		for i, v := range all {
			if i >= 20 {
				break
			}
			lines = append(lines, fmt.Sprintf("%d. %s [%s] ×%d——%s", i+1, v.Tool, v.Typ, v.Count, truncateArgs(v.Msg, 90)))
		}
		return strings.Join(lines, "\n")
	}
	// 单工具详情
	types := errStore.Tools[tool]
	if len(types) == 0 {
		return fmt.Sprintf("（工具 %s 无错误记录）", tool)
	}
	for _, tp := range []string{"param", "exec"} {
		for _, v := range types[tp] {
			lines = append(lines, fmt.Sprintf("[%s] ×%d——%s（最近 %s）", tp, v.Count, truncateArgs(v.Msg, 120), timeAgo(now, v.Last)))
		}
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i] > lines[j] })
	if len(lines) == 0 {
		return fmt.Sprintf("（工具 %s 无错误记录）", tool)
	}
	return fmt.Sprintf("工具 %s 错误统计（共 %d 桶——参数错=描述/schema 待优化——exec 错=服务待修）:\n%s", tool, len(lines), strings.Join(lines, "\n"))
}

func timeAgo(now, t int64) string {
	d := now - t
	if d < 60 {
		return fmt.Sprintf("%ds 前", d)
	}
	if d < 3600 {
		return fmt.Sprintf("%dm 前", d/60)
	}
	if d < 86400 {
		return fmt.Sprintf("%dh 前", d/3600)
	}
	return fmt.Sprintf("%dd 前", d/86400)
}
