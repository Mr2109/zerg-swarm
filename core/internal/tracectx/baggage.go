// baggage.go —— W3C Baggage 的生成/解析纯函数（T1.6 的第二个载体）。
//
// 为什么要有它（B16：会话 / 租户 / **回放标记**）：traceparent 只带 trace/span —— "这是哪条链"有答案，
// "这是谁的链、是不是回放"没有答案。跨进程时子端拿不到主控的会话上下文，只能靠 baggage 捎过去。
//
// 口径：
//  1. **有序成员集**（不是 map）：往返一致要求逐字节可复原；map 迭代序随机 ⇒ 每次 Format 可能给出
//     不同字节序，"parse(format(x)) == x" 就只能在"忽略顺序"的意义上成立——那不算逐字节一致。
//  2. **解析拒绝而非宽容**：空成员、缺 '='、非法键字符、值里带分隔符/空格/控制字符、超长 —— 一律报错。
//     成员两侧的 OWS 允许（W3C 语法如此），但 **Format 只产出规范形**（无空格）⇒ 往返仍逐字节一致。
//  3. **空串 = 没有 baggage**（合法，不是错误）：往返一致在"空"这一端也成立（Format(空) == ""）。
package tracectx

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// 我们自己的 baggage 键（低基数、闭集；扩展时在这里加，别在各调用点拼字符串）
const (
	KeySession = "zerg.session" // 会话 ID（"谁的链"）
	KeyReplay  = "zerg.replay"  // 回放标记（值 "1" = 本请求是回放；不写 = 非回放，不写 "0"）
)

// MaxBaggageBytes —— 单条 baggage 头的字节上限（W3C 建议 8192）。
// 超限**拒绝**而不是截断：截断会造出"看起来合法的半个成员"，比拒绝更难查。
const MaxBaggageBytes = 8192

// baggage 非法输入哨兵
var (
	ErrBaggageTooLong     = errors.New("tracectx: baggage 超长")
	ErrBaggageEmptyMember = errors.New("tracectx: baggage 含空成员")
	ErrBaggageShape       = errors.New("tracectx: baggage 成员缺 '='")
	ErrBaggageKey         = errors.New("tracectx: baggage 键非法")
	ErrBaggageValue       = errors.New("tracectx: baggage 值非法")
)

// ReplayEnv —— 回放标记的**唯一来源**（环境变量；值 "1" 才算回放）。
//
// 为什么不自动推断（时间戳/会话形态/模型名这些"看起来像回放"的信号一个都不用）：B16 要的是
// "回放标记能被传播"，不是"猜这次是不是回放"——猜错会把正常流量标成回放，下游据此决策就会出事。
// 不设 ⇒ 非回放，baggage 里**不写** "zerg.replay=0"（不写 "0" 与写 "0" 是两件事，后者会被读成"确定非回放"）。
const ReplayEnv = "ZERG_REPLAY"

// ReplayMarked —— 本次进程是否被显式标为回放（供调用方决定是否带 zerg.replay=1）。
func ReplayMarked() bool { return os.Getenv(ReplayEnv) == "1" }

// Pair —— baggage 的一个成员（顺序即线上顺序）。
type Pair struct{ Key, Value string }

// Baggage —— 有序成员集（值语义：Set 返回新值，不改接收者；便于当"事实"在包间传递）。
type Baggage struct{ Pairs []Pair }

// Len —— 成员数。
func (b Baggage) Len() int { return len(b.Pairs) }

// Get —— 按键取值（键大小写**敏感**地找；我们自己的键都是小写常量，够用）。
func (b Baggage) Get(key string) string {
	for _, p := range b.Pairs {
		if p.Key == key {
			return p.Value
		}
	}
	return ""
}

// Has —— 键是否存在（与 Get 的区别："k=" 也算存在——存在性与值非空是两件事）。
func (b Baggage) Has(key string) bool {
	for _, p := range b.Pairs {
		if p.Key == key {
			return true
		}
	}
	return false
}

// Set —— 覆盖或追加一个成员，返回**新** Baggage（原位保留 ⇒ 继承来的成员顺序不变）。
func (b Baggage) Set(key, value string) Baggage {
	out := Baggage{Pairs: make([]Pair, len(b.Pairs))}
	copy(out.Pairs, b.Pairs)
	for i := range out.Pairs {
		if out.Pairs[i].Key == key {
			out.Pairs[i].Value = value
			return out
		}
	}
	return Baggage{Pairs: append(out.Pairs, Pair{Key: key, Value: value})}
}

// Equal —— 逐位相等（顺序敏感：这正是"往返一致"要的那种相等）。
func (b Baggage) Equal(o Baggage) bool {
	if len(b.Pairs) != len(o.Pairs) {
		return false
	}
	for i := range b.Pairs {
		if b.Pairs[i] != o.Pairs[i] {
			return false
		}
	}
	return true
}

// String —— 规范形；**仅供日志**（拿不到错误）。拼请求头请用 FormatBaggage。
func (b Baggage) String() string {
	s, err := FormatBaggage(b)
	if err != nil {
		return "<非法 baggage>"
	}
	return s
}

// Session —— zerg.session 的值（空 = 没带）。
func (b Baggage) Session() string { return b.Get(KeySession) }

// Replay —— 是否被标为回放（只有值 "1" 算标记；不写/写别的都不算——不做"非空即真"的猜）。
func (b Baggage) Replay() bool { return b.Get(KeyReplay) == "1" }

// FormatBaggage —— **生成**纯函数：成员集 ⇒ `k=v,k=v`（无空格）。任一成员非法即报错（不跳过、不修）。
func FormatBaggage(b Baggage) (string, error) {
	if len(b.Pairs) == 0 {
		return "", nil // 空 = 不写这个头（不是错误）
	}
	var sb strings.Builder
	for i, p := range b.Pairs {
		if !validBaggageKey(p.Key) {
			return "", fmt.Errorf("%w：%q", ErrBaggageKey, p.Key)
		}
		if !validBaggageValue(p.Value) {
			return "", fmt.Errorf("%w：键 %q 的值 %q", ErrBaggageValue, p.Key, p.Value)
		}
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(p.Key)
		sb.WriteByte('=')
		sb.WriteString(p.Value)
	}
	s := sb.String()
	if len(s) > MaxBaggageBytes {
		return "", fmt.Errorf("%w：%d 字节 > %d", ErrBaggageTooLong, len(s), MaxBaggageBytes)
	}
	return s, nil
}

// ParseBaggage —— **解析**纯函数：`k=v,k=v` ⇒ 成员集。非法输入一律拒绝。
// 空串/仅空白 ⇒ 空成员集（没有 baggage，合法）。
func ParseBaggage(s string) (Baggage, error) {
	if strings.TrimSpace(s) == "" {
		return Baggage{}, nil
	}
	if len(s) > MaxBaggageBytes {
		return Baggage{}, fmt.Errorf("%w：%d 字节 > %d", ErrBaggageTooLong, len(s), MaxBaggageBytes)
	}
	var out Baggage
	for _, m := range strings.Split(s, ",") {
		m = strings.TrimSpace(m) // W3C 允许成员两侧 OWS；规范形不带（Format 不加空格）
		if m == "" {
			return Baggage{}, fmt.Errorf("%w：%q", ErrBaggageEmptyMember, s)
		}
		k, v, ok := strings.Cut(m, "=")
		if !ok {
			return Baggage{}, fmt.Errorf("%w：%q", ErrBaggageShape, m)
		}
		if !validBaggageKey(k) {
			return Baggage{}, fmt.Errorf("%w：%q", ErrBaggageKey, k)
		}
		if !validBaggageValue(v) {
			return Baggage{}, fmt.Errorf("%w：键 %q 的值 %q", ErrBaggageValue, k, v)
		}
		out.Pairs = append(out.Pairs, Pair{Key: k, Value: v})
	}
	return out, nil
}

// validBaggageKey —— 非空 + 全是 token 字符（RFC 7230 token：不含分隔符/空白/控制字符）。
func validBaggageKey(k string) bool {
	if k == "" {
		return false
	}
	for i := 0; i < len(k); i++ {
		if !isTokenByte(k[i]) {
			return false
		}
	}
	return true
}

// validBaggageValue —— 可空 + 全部为可见 ASCII，且不含 `,` `;` `=` `"`（分隔符会破坏成员边界）。
// 空格一律拒绝：允许空格就得处理"值尾空格 vs 成员间隔空格"，那是宽容解析的入口。
func validBaggageValue(v string) bool {
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c < 0x21 || c > 0x7e {
			return false
		}
		switch c {
		case ',', ';', '=', '"':
			return false
		}
	}
	return true
}

// isTokenByte —— RFC 7230 token 字符集。
func isTokenByte(c byte) bool {
	switch {
	case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return false
}
