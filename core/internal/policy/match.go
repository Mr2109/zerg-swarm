// match.go — 匹配式（specifier）的三种形态、参数取值与比较。
//
// 本批**只支持三种形态**（照现成语法：`Tool` 或 `Tool(specifier)`，如 `Bash(npm run *)`、`Read(./.env)`、
// `WebFetch(domain:example.com)`）；**认不出的形态一律 SpecUnknown ⇒ 判定时"判不了 ⇒ ask"**，
// 绝不默认放行（设计稿 v1.2 §〇 F2 的 fail-closed 语义）。
//
//	形态        例子                  语义
//	SpecNone    `allow read`          无匹配式：按工具名匹配（该工具的任何调用）
//	SpecPrefix  `bash(npm run *)`     前缀通配：**唯一允许的通配位置是行尾的单个 `*`**；无通配 ⇒ 逐字相等
//	SpecPath    `write(src/**)`       路径前缀（`/**` 结尾 = 该目录之下）/ `write(./.env)` = 逐字相等（按 POSIX 归一路径）
//	SpecDomain  `webfetch(domain:a.b)` 域名：等于该域名，或其子域（带点号边界 ⇒ `notexample.com` 不命中 `example.com`）
//	SpecUnknown `read(*.env)`         认不出（中缀/后缀通配、`**` 不在行尾、正则、域名残缺…）⇒ 判不了 ⇒ ask
//
// ── 取值口径（"规则读哪个参数"是本包的契约，必须确定且可测）──
//
// 按形态查一组**固定键名**，取**第一个存在**的键：
//
//	SpecPrefix → command / cmd / query
//	SpecPath   → path / file_path / filePath / target / dir / file
//	SpecDomain → url / domain
//
// 取到的值必须是**字符串**（容器允许下探，见 MaxArgDepth）。取不到 / 不是字符串 / 嵌套过深 ⇒
// **判不了** ⇒ 该条规则不参与本次判定，并把本次判定升级为 ask（args_unresolved）。
// 设计取舍：**宁可多问一次，不可默默放行** —— 猜参数（数字转字符串、容器取第一个成员、跳过本键再试别的键）
// 都会让"规则写了但没生效"变成静默放行，而静默放行是这一类系统里最难查的缺陷。
package policy

import (
	"path"
	"sort"
	"strings"
)

// SpecKind — 匹配式形态（解析期一次判定，判定期只按它分派）。
type SpecKind string

const (
	SpecNone    SpecKind = "none"    // 无匹配式（只按工具名）
	SpecPrefix  SpecKind = "prefix"  // 前缀通配 / 逐字相等
	SpecPath    SpecKind = "path"    // 路径前缀 / 路径逐字相等
	SpecDomain  SpecKind = "domain"  // 域名（含子域）
	SpecUnknown SpecKind = "unknown" // 认不出 ⇒ 判不了 ⇒ ask
)

// MaxArgDepth — 参数取值的下探深度上限（**嵌套过深 = 判不了**，不是"取最里面的那个"）。
const MaxArgDepth = 8

// 取值的固定键名（顺序即优先级）。三条一处定义，避免各处自造键名。
var (
	prefixArgKeys = []string{"command", "cmd", "query"}
	pathArgKeys   = []string{"path", "file_path", "filePath", "target", "dir", "file"}
	domainArgKeys = []string{"url", "domain"}
)

// classifySpec — 匹配式形态判定。**语法决定形态**（不看调用侧参数），故解析期即可定型并进用例。
func classifySpec(raw string) SpecKind {
	s := strings.TrimSpace(raw)
	if s == "" {
		return SpecNone
	}
	// 域名式：`domain:<主机名>`（残缺/带通配/带端口 ⇒ 认不出）
	if d, ok := strings.CutPrefix(s, "domain:"); ok {
		if validHost(d) {
			return SpecDomain
		}
		return SpecUnknown
	}
	// 整个路径树
	if s == "**" {
		return SpecPath
	}
	// 路径前缀：`x/**`（`**` 只许在行尾，且只能出现一次）
	if strings.HasSuffix(s, "/**") {
		if strings.Contains(strings.TrimSuffix(s, "/**"), "**") {
			return SpecUnknown
		}
		return SpecPath
	}
	// `**` 出现在非行尾位置（如 `src/**/*.go`、`**/x`）⇒ 认不出
	if strings.Contains(s, "**") {
		return SpecUnknown
	}
	pathish := strings.Contains(s, "/") ||
		strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, "~/")
	if pathish {
		if strings.ContainsAny(s, "*?[") {
			return SpecUnknown
		}
		return SpecPath
	}
	// 前缀式：唯一的通配位置是行尾单个 `*`
	if strings.Contains(s, "*") {
		if strings.HasSuffix(s, "*") && strings.Count(s, "*") == 1 {
			return SpecPrefix
		}
		return SpecUnknown
	}
	if strings.ContainsAny(s, "?[") {
		return SpecUnknown
	}
	// 无通配、非路径、非域名 ⇒ 逐字相等
	return SpecPrefix
}

// keysFor — 形态 → 取值键名（其余形态没有可读的参数键）。
func keysFor(kind SpecKind) []string {
	switch kind {
	case SpecPrefix:
		return prefixArgKeys
	case SpecPath:
		return pathArgKeys
	case SpecDomain:
		return domainArgKeys
	}
	return nil
}

// resolveArg — 按形态取参数值。
// 第一个**存在**的键即为本次取值的键；它取不出字符串就**判不了**（不再去试别的键 —— 猜键名会让规则静默失效）。
func resolveArg(args map[string]any, kind SpecKind) (string, bool) {
	for _, k := range keysFor(kind) {
		v, ok := args[k]
		if !ok {
			continue
		}
		return argString(v, 0)
	}
	return "", false
}

// argString — 取字符串值：字符串直接用；容器（映射/切片）允许下探查找**第一个字符串**（键名排序 / 下标序，
// 结果确定），但**深度上限 MaxArgDepth 之外一律判不了**；数字/布尔/nil/其他类型**不猜、不转换**。
func argString(v any, depth int) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case map[string]any:
		if depth >= MaxArgDepth {
			return "", false
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if s, ok := argString(t[k], depth+1); ok {
				return s, true
			}
		}
		return "", false
	case []any:
		if depth >= MaxArgDepth {
			return "", false
		}
		for _, e := range t {
			if s, ok := argString(e, depth+1); ok {
				return s, true
			}
		}
		return "", false
	default:
		// nil / 数字 / 布尔 / 结构体 / 函数 … ⇒ 判不了（不转换、不猜）
		return "", false
	}
}

// matchValue — 形态 + 匹配式 vs 取到的值。调用方保证 value 已是字符串。
func matchValue(kind SpecKind, spec, value string) bool {
	v := strings.TrimSpace(value)
	switch kind {
	case SpecPrefix:
		if strings.HasSuffix(spec, "*") {
			return strings.HasPrefix(v, strings.TrimSuffix(spec, "*"))
		}
		return v == strings.TrimSpace(spec)
	case SpecPath:
		return matchPath(spec, v)
	case SpecDomain:
		return matchDomain(spec, v)
	}
	return false
}

// matchPath — 路径比较。全部先归一（POSIX 分隔符、`path.Clean` 去掉 `./` 与重复斜杠）再比。
func matchPath(spec, v string) bool {
	sp := strings.TrimSpace(spec)
	if sp == "**" {
		return v != ""
	}
	if dir, ok := strings.CutSuffix(sp, "/**"); ok {
		d := cleanPath(dir)
		if d == "" || d == "." {
			return v != ""
		}
		cv := cleanPath(v)
		return cv == d || strings.HasPrefix(cv, d+"/")
	}
	return cleanPath(v) == cleanPath(sp)
}

// cleanPath — 归一路径（空串保持空串，避免 "." 与 "" 混为一谈）。
func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	return path.Clean(p)
}

// matchDomain — 域名比较：等于该域名，或它的子域（带点号边界）。
// 入参是**规则原文**（形如 `domain:example.com`），故先剥掉 `domain:` 前缀。
func matchDomain(spec, v string) bool {
	d := strings.ToLower(strings.TrimSpace(spec))
	d = strings.TrimPrefix(d, "domain:")
	d = strings.TrimSuffix(d, ".")
	host := hostOf(v)
	if d == "" || host == "" {
		return false
	}
	return host == d || strings.HasSuffix(host, "."+d)
}

// hostOf — 从 url/域名字符串里取主机名（去 scheme、去路径查询、去端口、小写；不联网、不引入依赖）。
func hostOf(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, ":"); i >= 0 && allDigits(s[i+1:]) {
		s = s[:i]
	}
	return strings.Trim(s, "[]")
}

// allDigits — 端口判据（只剥纯数字端口；不碰 IPv6 的冒号）。
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// validHost — 域名式的主体是否是一个合法主机名（字母/数字/点/连字符，含至少一个点之外的字符）。
func validHost(h string) bool {
	h = strings.TrimSpace(h)
	if h == "" || len(h) > 253 {
		return false
	}
	for _, r := range h {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-':
		default:
			return false
		}
	}
	if strings.HasPrefix(h, ".") || strings.HasSuffix(h, ".") || strings.Contains(h, "..") {
		return false
	}
	return true
}
