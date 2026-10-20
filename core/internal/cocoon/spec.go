// spec.go — `cocoon.toml` 契约解析（T2 · 设计稿 v2 §D ＋ v3 §D/§E）
//
// ★ 本文件只用标准库：包 `cocoon` 要被判据门（T7）与子端一起编，不许拖第三方解析器进来
// （go.mod 里虽然已有 yaml/chi 等，但契约解析是最小可信面，自己维护一份 TOML 子集比
// 引一个通用实现更容易证明「它到底接受什么」）。
//
// 支持的 TOML 子集（够 v2 §D 全字段，且**故意**不再多）：
//
//	① 注释：`#` 到行尾（字符串里的 `#` 不算注释）
//	② 键值：`key = value`，键可为裸键（允许中文，如 茧/服务）、`"引号键"`、`'字面键'`，支持点号嵌套 `a.b = 1`
//	③ 值：基本字符串（\n \t \" \\ \uXXXX 等转义）、字面字符串、整数（含 0x/0o/0b、下划线分隔）、
//	   浮点、布尔、数组（可跨行、可尾逗号）、行内表 `{ a = 1 }`
//	④ 表：`[表]`、`[a.b]` 嵌套表、`[[数组表]]`（多次出现 ⇒ 追加元素，如 [[服务]]/[[端口]]）
//	✗ 不支持：多行字符串 `"""`、日期时间类型、`[[a.b]]` 里再套 `[a.b.c]` 的边角写法——
//	  遇到一律**报错点名行号**，绝不静默吞掉（静默 = 契约被悄悄降级，比报错危险）。
//
// ★ 四条硬约定（v2/v3 定稿，改这里必须同步改 T3/T7）：
//
//	① 密钥键名匹配 (?i)(token|secret|password|key) 时，只收**引用形态**的字符串值：
//	   `${VAR}`（`^\$\{[A-Z_][A-Z0-9_]*\}$`）或 `secret://<name>`（`^secret://[a-z0-9][a-z0-9._-]*$`），
//	   整值匹配、不认嵌在长串里的片段。其余任何非空值（明文、数字、布尔、数组/表、
//	   `${}`/`${lower}` 这类畸形引用）⇒ 整份契约拒绝。空串可过（表示「此键存在、值由宿主给」）。
//	   理由：引用只写「去哪儿取值」，真正取值一律走宿主注入（v2 §A 令牌不进 argv/日志、§B 契约宿主侧保管）；
//	   写成最严的「连引用也不收」会把唯一安全的写法堵死，等于逼用户把密钥抄进契约。
//	② digest 必须 `sha256:<64 hex>`（大小写hex都收，存盘归一为小写）；**缺 digest 但给了 image
//	   ⇒ 不报错，只记 Warning**（v2 §D「判据可复现比它，不比标签」：缺它就没法证明可复现，
//	   但「先能跑起来」不能被它卡死——警告必须被 `cocoon inspect` 打出来）。
//	③ 服务名唯一；`depends_on` 必须指向存在的服务、不许自依赖、不许有环（DFS 三色 + 环路径回显），
//	   并顺带产出**启动顺序**（Spec.Order，依赖在前），T3 直接照它起。
//	④ 端口 `inner` ∈ 1..65535 且不重复；`host` 为 "auto" 或 1..65535 的数字；固定 host 端口之间
//	   也不许撞（撞了就是两个茧抢同一个宿主端口，起第二个必失败，晚报不如早报）。
//
// ★ 错误一律是 *SpecError：带 File/Line/Field/Msg，Error() 形如
//
//		cocoon.toml:12: 服务[0].readiness.port: 必须在 1..65535，得到 0
//
//	  ——「点名行号与字段」是判据要求（否则用户在一份 200 行契约里找不到错），测试逐格断言。
//	  校验策略是**首个错误即返回**：契约是要被人改对的东西，一次报一条比一次报二十条更好读；
//	  需要全量报告时调用方循环改即可（不在此处堆砌错误聚合）。
package cocoon

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SpecFileName — 契约文件名（宿主侧保管的那一份；茧内只有只读副本，v2 §B）。
const SpecFileName = "cocoon.toml"

// DefaultReadinessTimeout — readiness 未写 timeout 时的默认值（v2 §D 样例写的是 30s）。
const DefaultReadinessTimeout = 30 * time.Second

// DefaultBind — [[端口]] 未写 bind 时的默认绑定地址：只绑回环，不默认暴露到网卡。
const DefaultBind = "127.0.0.1"

// specRootfsHostRoot — 「不换根」那个根。
//
// ★ 这里是 spec 层，只认「宿主根目录」这一件事，**不引后端常量**（nsjail 的缺省 chroot
// 是后端的事；契约层引后端常量会让两份语义永远绑死）。
const specRootfsHostRoot = "/"

// 合法枚举值。
const (
	ArchARM64 = "arm64"
	ArchAMD64 = "amd64"

	NetNone      = "none"
	NetAllowlist = "allowlist"

	HostAuto = "auto"
)

var (
	// specSecretKeyRe — 密钥键名判定（v2 §A：密钥/令牌永不进契约）。
	specSecretKeyRe = regexp.MustCompile(`(?i)(token|secret|password|key)`)
	// specSecretEnvRefRe — 密钥值的合法形态之一：`${VAR}` 引用（整值匹配，变量名限 POSIX 大写形态）。
	specSecretEnvRefRe = regexp.MustCompile(`^\$\{[A-Z_][A-Z0-9_]*\}$`)
	// specSecretURIRefRe — 另一种合法形态：`secret://<name>` 引用（名字限小写 DNS-ish）。
	specSecretURIRefRe = regexp.MustCompile(`^secret://[a-z0-9][a-z0-9._-]*$`)
	// specDigestRe — digest 规格：`sha256:` + 恰好 64 位 hex。
	specDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	// specNameRe — 茧名/服务名：它们会变成目录名与 socket 名的一部分，
	// 所以按「宽松 DNS label」收（字母数字开头，只含 . _ -），拒绝 "."/".." 之类靠它兜住。
	specNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	// specEnvKeyRe — 环境变量名必须是 POSIX 形态（能安全写进 envp；带 '=' 或空格的键会被 execve 吃掉）。
	specEnvKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// specEnvDefaults — [环境] 的固定默认值（v2 §D 明列）。用户显式写同名键则用用户的；
// 写成空串视同未写（空串不能当 TZ/LANG 用）。
var specEnvDefaults = []struct{ k, v string }{
	{"TZ", "Asia/Shanghai"},
	{"LANG", "C.UTF-8"},
}

// ── 结果类型 ──────────────────────────────────────────────────────────

// Spec — 一份解析并校验通过的契约。
//
// 字段与 v2 §D 一一对应；Line 是各段/各元素在文件里的起始行，供 inspect 与报错复用。
type Spec struct {
	// File — 契约来源（路径；ParseSpec 时为 SpecFileName），只用于报错文案。
	File string

	Cocoon    CocoonSpec
	Services  []ServiceSpec
	Ports     []PortSpec
	Build     BuildSpec
	Network   NetworkSpec
	Resources ResourceSpec
	Env       map[string]string

	// Order — 服务启动顺序（拓扑序，依赖在前，同层按声明顺序）。由 depends_on 算出，
	// 不是文件里的字段；T3 照它起进程、超时时按逆序停。
	Order []string

	// Warnings — 不阻断解析但必须被 inspect 打出来的问题（当前只有「缺 digest」一类）。
	Warnings []SpecWarning
}

// CocoonSpec — [茧]：身份与可复现锚点。
type CocoonSpec struct {
	Name    string
	Image   string
	Digest  string // 已归一为小写；空表示契约未锁 digest（伴随一条 Warning）
	Arch    string
	Workdir string

	// Rootfs — 可选：茧的**根文件系统目录**（宿主路径）。空 = 不换根（既有档：后端用它的
	// 缺省 chroot，nsjail 后端缺省是宿主根 `/`）。
	//
	// ★ 写绝对路径，或写**相对契约文件所在目录**的路径 —— 解析在 decodeCocoon 里一次做完
	// （存进来的永远是绝对路径）：让后端拿到的不是「一个要看 cwd 才知道指向哪」的相对串。
	Rootfs string
	// OverlayUpper — 可选：overlay 的**可写层**目录（宿主路径；空 = `<茧目录>/upper`）。
	//
	// ★ 只在给了 Rootfs 时才有意义：单独给它在解析期就拒（上层没有下层不是配置，是笔误）。
	OverlayUpper string

	Line int
}

// Readiness — 一个服务的就绪判据（v2 §D 里是行内表）。
//
// v3 §D 明确：readiness 只表示「就绪」，不当启动门、不做排序——排序只走 depends_on。
type Readiness struct {
	Port    int
	Path    string
	Timeout time.Duration
}

// ServiceSpec — [[服务]]。
type ServiceSpec struct {
	Name      string
	Entry     []string
	DependsOn []string
	Readiness *Readiness // nil 表示该服务未声明就绪判据
	Line      int

	// dependsLine — depends_on 在文件里的行（未写该键时取服务行）。
	// 只用于报错定位：环/悬空依赖指到「这条边」，比指到服务头更好找。
	dependsLine int
}

// PortSpec — [[端口]]：inner 是茧内端口，Host 是宿主侧（"auto" 或十进制端口串）。
type PortSpec struct {
	Inner int
	Host  string
	Bind  string
	Line  int
}

// HostPort — 把 Host 解析成端口号；auto 返回 (0,false)。
func (p PortSpec) HostPort() (int, bool) {
	if p.Host == "" || p.Host == HostAuto {
		return 0, false
	}
	n, err := strconv.Atoi(p.Host)
	if err != nil {
		return 0, false
	}
	return n, true
}

// BuildSpec — [构建]：唯一允许开网的相位（v2 §D）。
type BuildSpec struct {
	Install []string
	Cache   string
	Net     string // NetAllowlist | NetNone（缺省 none）
	Allow   []string
	Line    int
}

// NetworkSpec — [网络]：运行期网络。v2 §D「运行期强制 none」⇒ 这里只接受 none。
type NetworkSpec struct {
	Mode  string
	Allow []string
	Line  int
}

// ResourceSpec — [资源]：三项都必填（缺省即无上限，等于没设防，所以不给默认值）。
type ResourceSpec struct {
	CPU    float64
	MemGB  float64
	DiskGB float64
	Line   int
}

// SpecWarning — 不改判但必须显示的问题。
type SpecWarning struct {
	Line  int
	Field string
	Msg   string
}

func (w SpecWarning) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", SpecFileName, w.Line, w.Field, w.Msg)
}

// SpecError — 契约错误：行号 + 字段名 + 说明（三者缺一，用户就定位不到）。
type SpecError struct {
	File  string
	Line  int
	Field string
	Msg   string
}

func (e *SpecError) Error() string {
	file := e.File
	if file == "" {
		file = SpecFileName
	}
	switch {
	case e.Field == "" && e.Line <= 0:
		return fmt.Sprintf("%s: %s", file, e.Msg)
	case e.Field == "":
		return fmt.Sprintf("%s:%d: %s", file, e.Line, e.Msg)
	case e.Line <= 0:
		return fmt.Sprintf("%s: %s: %s", file, e.Field, e.Msg)
	default:
		return fmt.Sprintf("%s:%d: %s: %s", file, e.Line, e.Field, e.Msg)
	}
}

// AsSpecError — 从错误链里取 *SpecError（调用方据此决定退出码/文案）。
func AsSpecError(err error) (*SpecError, bool) {
	var se *SpecError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}

// Service — 按名取服务（T3/T4 用）。
func (s *Spec) Service(name string) (ServiceSpec, bool) {
	for _, svc := range s.Services {
		if svc.Name == name {
			return svc, true
		}
	}
	return ServiceSpec{}, false
}

// HasDigest — 契约是否锁了 digest（判据「可复现」的前置）。
func (s *Spec) HasDigest() bool { return s.Cocoon.Digest != "" }

// ── 入口 ──────────────────────────────────────────────────────────────

// LoadSpec — 读文件并解析。文件不存在时把 os 错误原样返回（不包装成 SpecError：
// 「文件没有」不是契约内容错，调用方要能 errors.Is(err, fs.ErrNotExist)）。
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sp, err := parseSpecBytes(path, data)
	if err != nil {
		return nil, err
	}
	return sp, nil
}

// ParseSpec — 从内存解析（File 记为 SpecFileName）。
func ParseSpec(data []byte) (*Spec, error) { return parseSpecBytes(SpecFileName, data) }

// ParseSpecFile — 带来源名的解析（测试与 inspect 复用同一入口，保证报错文案里的文件名是真的）。
func ParseSpecFile(name string, data []byte) (*Spec, error) {
	return parseSpecBytes(name, data)
}

func parseSpecBytes(name string, data []byte) (*Spec, error) {
	root, err := parseTOMLSubset(name, data)
	if err != nil {
		return nil, err
	}
	d := &specDecoder{file: name}
	// ① 先扫密钥：任何段里的密钥值都在这里被拦下，早于字段校验——
	//    否则「未知字段」之类的错会抢在前面，把真正的安全问题盖成一条拼写错误。
	if err := d.scanSecrets(root, ""); err != nil {
		return nil, err
	}
	// ② 逐项校验。
	sp, err := d.decode(root)
	if err != nil {
		return nil, err
	}
	return sp, nil
}

// ── 校验/装配 ─────────────────────────────────────────────────────────

type specDecoder struct {
	file string
	spec *Spec
}

func (d *specDecoder) errf(line int, field, format string, a ...any) error {
	return &SpecError{File: d.file, Line: line, Field: field, Msg: fmt.Sprintf(format, a...)}
}

func (d *specDecoder) warnf(line int, field, format string, a ...any) {
	d.spec.Warnings = append(d.spec.Warnings, SpecWarning{Line: line, Field: field, Msg: fmt.Sprintf(format, a...)})
}

// scanSecrets — 遍历整棵树（含未知段：契约里哪怕写了个不认识的段，也不许夹带密钥值）。
func (d *specDecoder) scanSecrets(t *tval, prefix string) error {
	for _, k := range t.ord {
		tv := t.tab[k]
		field := k
		if prefix != "" {
			field = prefix + "." + k
		}
		if specSecretKeyRe.MatchString(k) && !tv.isZeroValue() && !isSecretRef(tv) {
			return d.errf(tv.line, field,
				"键名命中密钥规则（token/secret/password/key）：只收引用形态，"+
					"`${VAR}`（如 ${GITHUB_TOKEN}）或 `secret://<name>`（如 secret://github），"+
					"不收明文/数字/布尔等字面值；留空串表示由宿主注入（v2 §A/§B）")
		}
		switch tv.kind {
		case tvTable:
			if err := d.scanSecrets(tv, field); err != nil {
				return err
			}
		case tvArray:
			for i, el := range tv.arr {
				if el.kind == tvTable {
					if err := d.scanSecrets(el, fmt.Sprintf("%s[%d]", field, i)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (d *specDecoder) decode(root *tval) (*Spec, error) {
	d.spec = &Spec{File: d.file, Env: map[string]string{}}

	// 顶层只认这七段：拼错的段名（[服务] 写成 [服务s]）必须报错，否则会得到一份
	// 「看着解析成功、其实一个服务都没起」的契约。
	allowed := []string{"茧", "服务", "端口", "构建", "网络", "资源", "环境"}
	allowSet := map[string]bool{}
	for _, k := range allowed {
		allowSet[k] = true
	}
	for _, k := range root.ord {
		if !allowSet[k] {
			return nil, d.errf(root.tab[k].line, k, "未知的顶层表（允许：%s）", strings.Join(allowed, " / "))
		}
	}

	cocoonTab, ok := root.tab["茧"]
	if !ok {
		return nil, d.errf(1, "茧", "缺少 [茧] 表")
	}
	if err := d.decodeCocoon(cocoonTab); err != nil {
		return nil, err
	}

	svcTab, ok := root.tab["服务"]
	if !ok {
		// 一个服务都没有的茧没有任何可执行面；与其起个空茧让人猜，不如直接拒绝。
		return nil, d.errf(cocoonTab.line, "服务", "至少需要一个 [[服务]]")
	}
	if err := d.decodeServices(svcTab); err != nil {
		return nil, err
	}

	if portTab, ok := root.tab["端口"]; ok {
		if err := d.decodePorts(portTab); err != nil {
			return nil, err
		}
	}
	if tab, ok := root.tab["构建"]; ok {
		if err := d.decodeBuild(tab); err != nil {
			return nil, err
		}
	} else {
		// 未写 [构建] 时按「不装东西、断网」填充，让下游不必判空串。
		d.spec.Build = BuildSpec{Net: NetNone}
	}
	if tab, ok := root.tab["网络"]; ok {
		if err := d.decodeNetwork(tab); err != nil {
			return nil, err
		}
		// 未写 [网络] 时按 none 填充，让下游不必判空。
	} else {
		d.spec.Network = NetworkSpec{Mode: NetNone}
	}
	if tab, ok := root.tab["资源"]; ok {
		if err := d.decodeResources(tab); err != nil {
			return nil, err
		}
	} else {
		return nil, d.errf(cocoonTab.line, "资源", "缺少 [资源] 表（cpu / mem_gb / disk_gb 必填，不给无限默认）")
	}
	if tab, ok := root.tab["环境"]; ok {
		if err := d.decodeEnv(tab); err != nil {
			return nil, err
		}
	}
	for _, def := range specEnvDefaults {
		if d.spec.Env[def.k] == "" {
			d.spec.Env[def.k] = def.v
		}
	}
	return d.spec, nil
}

// ── [茧] ──────────────────────────────────────────────────────────────

func (d *specDecoder) decodeCocoon(t *tval) error {
	const f = "茧"
	if t.kind != tvTable {
		return d.errf(t.line, f, "必须是表")
	}
	if err := d.checkKeys(t, f, "name", "image", "digest", "arch", "workdir", "rootfs", "overlay_upper"); err != nil {
		return err
	}
	c := CocoonSpec{Line: t.line}

	name, _, err := d.strField(t, f, "name")
	if err != nil {
		return err
	}
	if name == "" {
		return d.errf(t.line, f+".name", "必填")
	}
	if !specNameRe.MatchString(name) {
		return d.errf(t.tab["name"].line, f+".name", "名字非法 %q（字母数字开头，只含字母数字 . _ -，≤64 字符）", name)
	}
	c.Name = name

	image, hasImage, err := d.strField(t, f, "image")
	if err != nil {
		return err
	}
	if hasImage {
		if image == "" {
			return d.errf(t.tab["image"].line, f+".image", "不得为空串（不想指定镜像就别写这个键）")
		}
		if strings.ContainsAny(image, " \t") {
			return d.errf(t.tab["image"].line, f+".image", "不得含空白：%q", image)
		}
		if strings.Contains(image, "@") {
			return d.errf(t.tab["image"].line, f+".image",
				"不要在 image 里写 @digest（%q），digest 必须单独写在 .digest 字段，否则可复现判据读不到", image)
		}
	}
	c.Image = image

	digest, hasDigest, err := d.strField(t, f, "digest")
	if err != nil {
		return err
	}
	if hasDigest && digest != "" {
		low := strings.ToLower(digest)
		if !specDigestRe.MatchString(low) {
			return d.errf(t.tab["digest"].line, f+".digest",
				"必须形如 sha256:<64 位 hex>，得到 %q", digest)
		}
		c.Digest = low
	}

	if !hasImage && c.Digest == "" {
		return d.errf(t.line, f+".image", "image 与 digest 至少要有一个")
	}
	if c.Digest == "" {
		// v2 §D：缺 digest 不阻断，但「可复现」判据（判据 11）就失去了比对的锚点 ⇒ 必须显式警告。
		d.warnf(t.tab["image"].line, f+".digest",
			"未锁 digest：可复现判据（同 digest 两机同结果）无法成立，只认标签等于认 TOFU")
	}

	arch, hasArch, err := d.strField(t, f, "arch")
	if err != nil {
		return err
	}
	if hasArch {
		if arch != ArchARM64 && arch != ArchAMD64 {
			return d.errf(t.tab["arch"].line, f+".arch", "只能是 %q 或 %q，得到 %q", ArchARM64, ArchAMD64, arch)
		}
	}
	c.Arch = arch

	workdir, _, err := d.strField(t, f, "workdir")
	if err != nil {
		return err
	}
	if workdir != "" && !strings.HasPrefix(workdir, "/") {
		return d.errf(t.tab["workdir"].line, f+".workdir", "必须是茧内绝对路径，得到 %q", workdir)
	}
	c.Workdir = workdir

	// rootfs / overlay_upper：两个宿主路径，都是**可选**的（缺省空 = 维持既有行为：
	// 不 chroot、不做 overlay）。★ 这是「既有档」，不是「静默降级」——
	// 给了 rootfs 之后任何一步做不成都必须报错，不许退回这一档（见 nsjailRootfsPlanFor）。
	rootfs, hasRootfs, err := d.strField(t, f, "rootfs")
	if err != nil {
		return err
	}
	if hasRootfs {
		if strings.TrimSpace(rootfs) == "" {
			return d.errf(t.tab["rootfs"].line, f+".rootfs",
				"不得为空串（不想换根就别写这个键：空串会被读成「换到空气里」，与「不换根」是两件事）")
		}
		abs, aerr := resolveContractPath(d.file, rootfs)
		if aerr != nil {
			return d.errf(t.tab["rootfs"].line, f+".rootfs", "解析成绝对路径失败：%v", aerr)
		}
		c.Rootfs = abs
		if abs == specRootfsHostRoot {
			// 显式写 `/` 与不写这个键等价：不阻断（这是用户的明确选择），但必须说出来。
			d.warnf(t.tab["rootfs"].line, f+".rootfs",
				"rootfs = \"/\" 等于不换根（隔离只剩 namespace + cgroup + rlimit，茧内跑的是宿主二进制）")
		}
	}

	upper, hasUpper, err := d.strField(t, f, "overlay_upper")
	if err != nil {
		return err
	}
	if hasUpper {
		if strings.TrimSpace(upper) == "" {
			return d.errf(t.tab["overlay_upper"].line, f+".overlay_upper",
				"不得为空串（缺省可写层是 <茧目录>/upper；不想用 overlay 就别写这个键）")
		}
		if c.Rootfs == "" {
			return d.errf(t.tab["overlay_upper"].line, f+".overlay_upper",
				"要先给 .rootfs：overlay 的上层（可写层）没有下层（镜像根）就无处叠加")
		}
		if c.Rootfs == specRootfsHostRoot {
			return d.errf(t.tab["overlay_upper"].line, f+".overlay_upper",
				"rootfs = \"/\" 时不给 overlay（下层是宿主根，叠出来的可写层等于把宿主根复制一遍）")
		}
		abs, aerr := resolveContractPath(d.file, upper)
		if aerr != nil {
			return d.errf(t.tab["overlay_upper"].line, f+".overlay_upper", "解析成绝对路径失败：%v", aerr)
		}
		c.OverlayUpper = abs
	}

	d.spec.Cocoon = c
	return nil
}

// resolveContractPath — 契约里的宿主路径：绝对路径原样（Clean），相对路径按**契约文件所在目录**
// 解析成绝对路径。
//
// ★ 为什么必须在这里就绝对化：契约在 `A/cocoon.toml` 里写 `rootfs = "images/rootfs"` 时，
// 「相对谁」只能有一个答案（相对契约自己），而下游（后端/编排）拿到的若是相对串，
// 它会相对**自己的 cwd** 解析 —— 那是另一个目录，且错得静默。取 cwd 失败（cwd 被删）时
// 报错而不是回落：这条路径要交给 mount/chroot，猜错等于把宿主的东西当茧的东西。
func resolveContractPath(contractFile, p string) (string, error) {
	p = strings.TrimSpace(p)
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("路径里含 NUL")
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	base := "."
	if contractFile != "" {
		base = filepath.Dir(contractFile)
	}
	joined := filepath.Join(base, p)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("取绝对路径 %q 失败: %w", joined, err)
	}
	return abs, nil
}

// ── [[服务]] ──────────────────────────────────────────────────────────

func (d *specDecoder) decodeServices(t *tval) error {
	if t.kind != tvArray {
		return d.errf(t.line, "服务", "必须写成数组表 [[服务]]")
	}
	seen := map[string]struct {
		line int
		idx  int
	}{}
	svcs := make([]ServiceSpec, 0, len(t.arr))
	for i, el := range t.arr {
		field := fmt.Sprintf("服务[%d]", i)
		if el.kind != tvTable {
			return d.errf(el.line, field, "必须是表")
		}
		if err := d.checkKeys(el, field, "name", "entry", "depends_on", "readiness"); err != nil {
			return err
		}
		var svc ServiceSpec
		svc.Line = el.line

		name, _, err := d.strField(el, field, "name")
		if err != nil {
			return err
		}
		if name == "" {
			return d.errf(el.line, field+".name", "必填")
		}
		if !specNameRe.MatchString(name) {
			return d.errf(el.tab["name"].line, field+".name", "服务名非法 %q", name)
		}
		if prev, dup := seen[name]; dup {
			return d.errf(el.tab["name"].line, field+".name",
				"服务名重复 %q（首次出现在第 %d 行的 服务[%d]）", name, prev.line, prev.idx)
		}
		seen[name] = struct {
			line int
			idx  int
		}{el.tab["name"].line, i}
		svc.Name = name

		entry, _, err := d.strArrField(el, field, "entry")
		if err != nil {
			return err
		}
		if len(entry) == 0 {
			return d.errf(el.line, field+".entry", "必填且不得为空数组（argv[0] 是入口程序）")
		}
		for j, a := range entry {
			if strings.TrimSpace(a) == "" {
				return d.errf(el.tab["entry"].line, fmt.Sprintf("%s.entry[%d]", field, j), "argv 元素不得为空串")
			}
		}
		svc.Entry = entry

		deps, _, err := d.strArrField(el, field, "depends_on")
		if err != nil {
			return err
		}
		dupDep := map[string]bool{}
		for j, dep := range deps {
			if dep == "" {
				return d.errf(el.tab["depends_on"].line, fmt.Sprintf("%s.depends_on[%d]", field, j), "不得为空串")
			}
			if dupDep[dep] {
				return d.errf(el.tab["depends_on"].line, fmt.Sprintf("%s.depends_on[%d]", field, j),
					"重复依赖 %q", dep)
			}
			dupDep[dep] = true
		}
		svc.DependsOn = deps
		if deps != nil {
			svc.dependsLine = el.tab["depends_on"].line
		} else {
			svc.dependsLine = el.line
		}

		if rt, ok := el.tab["readiness"]; ok {
			rd, err := d.decodeReadiness(rt, field+".readiness")
			if err != nil {
				return err
			}
			svc.Readiness = rd
		}
		svcs = append(svcs, svc)
	}
	d.spec.Services = svcs
	return d.validateGraph(svcs)
}

func (d *specDecoder) decodeReadiness(t *tval, field string) (*Readiness, error) {
	if t.kind != tvTable {
		return nil, d.errf(t.line, field, "必须是行内表，如 { port = 8000, path = \"/healthz\", timeout = \"30s\" }")
	}
	if err := d.checkKeys(t, field, "port", "path", "timeout"); err != nil {
		return nil, err
	}
	rd := &Readiness{Timeout: DefaultReadinessTimeout}
	if tv, ok := t.tab["port"]; !ok {
		return nil, d.errf(t.line, field+".port", "必填（就绪探测打在茧内哪个端口）")
	} else if tv.kind != tvInt {
		return nil, d.errf(tv.line, field+".port", "必须是整数")
	} else {
		if err := specPortRange(tv.i, tv.line, d, field+".port"); err != nil {
			return nil, err
		}
		rd.Port = int(tv.i)
	}
	if tv, ok := t.tab["path"]; ok {
		if tv.kind != tvString {
			return nil, d.errf(tv.line, field+".path", "必须是字符串")
		}
		if tv.s != "" && !strings.HasPrefix(tv.s, "/") {
			return nil, d.errf(tv.line, field+".path", "必须以 / 开头，得到 %q", tv.s)
		}
		rd.Path = tv.s
	}
	if tv, ok := t.tab["timeout"]; ok {
		if tv.kind != tvString {
			return nil, d.errf(tv.line, field+".timeout", "必须是时长字符串，如 \"30s\"")
		}
		dur, err := time.ParseDuration(tv.s)
		if err != nil {
			return nil, d.errf(tv.line, field+".timeout", "不是合法时长 %q（例：\"30s\" / \"1m30s\"）", tv.s)
		}
		if dur <= 0 {
			return nil, d.errf(tv.line, field+".timeout", "必须 > 0，得到 %q", tv.s)
		}
		rd.Timeout = dur
	}
	return rd, nil
}

// validateGraph — depends_on 的引用完整性 + 无环 + 拓扑序（v2 §D / v3 §D）。
//
// v3 §D 把 depends_on 拆成「顺序边（After）」与「存活边（BindsTo）」，两者都要求这里先把
// 图判成**有向无环**：有环就永远起不完，早报。
func (d *specDecoder) validateGraph(svcs []ServiceSpec) error {
	byName := map[string]int{}
	lineOf := map[string]int{}
	for i, s := range svcs {
		byName[s.Name] = i
		lineOf[s.Name] = s.Line
	}
	for i, s := range svcs {
		field := fmt.Sprintf("服务[%d]", i)
		for _, dep := range s.DependsOn {
			if _, ok := byName[dep]; !ok {
				return d.errf(s.dependsLine, field+".depends_on", "依赖的服务 %q 不存在（已定义：%s）",
					dep, strings.Join(sortedNames(svcs), ", "))
			}
			if dep == s.Name {
				return d.errf(s.dependsLine, field+".depends_on", "服务 %q 依赖自身（循环依赖：%s → %s）", s.Name, s.Name, s.Name)
			}
		}
	}

	// 三色 DFS：灰链上的回边 = 环。栈里保留现场，报错能打出完整环路径
	// （只说「有环」而不说「哪几个人成环」，用户还得自己数）。
	const (
		white = 0
		grey  = 1
		black = 2
	)
	state := make(map[string]int, len(svcs))
	stack := make([]string, 0, len(svcs))
	order := make([]string, 0, len(svcs))

	var visit func(name string) error
	visit = func(name string) error {
		state[name] = grey
		stack = append(stack, name)
		for _, dep := range svcs[byName[name]].DependsOn {
			switch state[dep] {
			case grey:
				start := 0
				for i, n := range stack {
					if n == dep {
						start = i
						break
					}
				}
				cyc := append(append([]string{}, stack[start:]...), dep)
				i := byName[name]
				return d.errf(svcs[i].dependsLine, fmt.Sprintf("服务[%d].depends_on", i),
					"循环依赖：%s", strings.Join(cyc, " → "))
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = black
		order = append(order, name)
		return nil
	}
	for _, s := range svcs {
		if state[s.Name] == white {
			if err := visit(s.Name); err != nil {
				return err
			}
		}
	}
	d.spec.Order = order
	return nil
}

// ── [[端口]] ──────────────────────────────────────────────────────────

func (d *specDecoder) decodePorts(t *tval) error {
	if t.kind != tvArray {
		return d.errf(t.line, "端口", "必须写成数组表 [[端口]]")
	}
	innerSeen := map[int]int{} // inner → 行号
	hostSeen := map[int]int{}  // 固定 host 端口 → 行号
	ports := make([]PortSpec, 0, len(t.arr))
	for i, el := range t.arr {
		field := fmt.Sprintf("端口[%d]", i)
		if el.kind != tvTable {
			return d.errf(el.line, field, "必须是表")
		}
		if err := d.checkKeys(el, field, "inner", "host", "bind"); err != nil {
			return err
		}
		p := PortSpec{Line: el.line, Host: HostAuto, Bind: DefaultBind}

		tv, ok := el.tab["inner"]
		if !ok {
			return d.errf(el.line, field+".inner", "必填")
		}
		if tv.kind != tvInt {
			return d.errf(tv.line, field+".inner", "必须是整数")
		}
		if err := specPortRange(tv.i, tv.line, d, field+".inner"); err != nil {
			return err
		}
		p.Inner = int(tv.i)
		if prev, dup := innerSeen[p.Inner]; dup {
			return d.errf(tv.line, field+".inner", "茧内端口 %d 重复（首次出现在第 %d 行）", p.Inner, prev)
		}
		innerSeen[p.Inner] = tv.line

		if hv, ok := el.tab["host"]; ok {
			switch hv.kind {
			case tvString:
				if hv.s == HostAuto {
					p.Host = HostAuto
				} else {
					n, err := strconv.Atoi(hv.s)
					if err != nil {
						return d.errf(hv.line, field+".host", "只能是 \"auto\" 或端口号，得到 %q", hv.s)
					}
					if err := specPortRange(int64(n), hv.line, d, field+".host"); err != nil {
						return err
					}
					p.Host = strconv.Itoa(n)
				}
			case tvInt:
				if err := specPortRange(hv.i, hv.line, d, field+".host"); err != nil {
					return err
				}
				p.Host = strconv.FormatInt(hv.i, 10)
			default:
				return d.errf(hv.line, field+".host", "只能是 \"auto\" 或端口号")
			}
			if n, fixed := p.HostPort(); fixed {
				if prev, dup := hostSeen[n]; dup {
					return d.errf(el.tab["host"].line, field+".host",
						"宿主端口 %d 已被第 %d 行的端口占用（两个茧/两条映射抢同一宿主端口，起第二个必失败）", n, prev)
				}
				hostSeen[n] = el.tab["host"].line
			}
		}

		if bv, ok := el.tab["bind"]; ok {
			if bv.kind != tvString {
				return d.errf(bv.line, field+".bind", "必须是 IP 字符串，如 \"127.0.0.1\"")
			}
			if strings.TrimSpace(bv.s) == "" {
				return d.errf(bv.line, field+".bind", "不得为空串（不想指定就别写这个键，默认 "+DefaultBind+"）")
			}
			if net.ParseIP(bv.s) == nil {
				return d.errf(bv.line, field+".bind", "不是合法 IP %q", bv.s)
			}
			if !isLoopbackBind(bv.s) {
				d.warnf(bv.line, field+".bind", "绑定非回环地址 %s：会暴露到网卡（v2 §A 只承诺每茧独立 socket，未承诺监听面收敛）", bv.s)
			}
			p.Bind = bv.s
		}
		ports = append(ports, p)
	}
	d.spec.Ports = ports
	return nil
}

func isLoopbackBind(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func specPortRange(v int64, line int, d *specDecoder, field string) error {
	if v < 1 || v > 65535 {
		return d.errf(line, field, "必须在 1..65535，得到 %d", v)
	}
	return nil
}

// ── [构建] ────────────────────────────────────────────────────────────

func (d *specDecoder) decodeBuild(t *tval) error {
	const f = "构建"
	if t.kind != tvTable {
		return d.errf(t.line, f, "必须是表")
	}
	if err := d.checkKeys(t, f, "install", "cache", "net", "allow"); err != nil {
		return err
	}
	b := BuildSpec{Line: t.line, Net: NetNone}

	install, _, err := d.strArrField(t, f, "install")
	if err != nil {
		return err
	}
	for i, cmd := range install {
		if strings.TrimSpace(cmd) == "" {
			return d.errf(t.tab["install"].line, fmt.Sprintf("%s.install[%d]", f, i), "命令不得为空串")
		}
	}
	b.Install = install

	cache, _, err := d.strField(t, f, "cache")
	if err != nil {
		return err
	}
	if cache != "" && !strings.HasPrefix(cache, "/") {
		return d.errf(t.tab["cache"].line, f+".cache", "必须是茧内绝对路径，得到 %q", cache)
	}
	b.Cache = cache

	netMode, hasNet, err := d.strField(t, f, "net")
	if err != nil {
		return err
	}
	if hasNet {
		switch netMode {
		case NetAllowlist, NetNone:
			b.Net = netMode
		default:
			return d.errf(t.tab["net"].line, f+".net", "只能是 %q 或 %q，得到 %q", NetAllowlist, NetNone, netMode)
		}
	}

	allow, _, err := d.strArrField(t, f, "allow")
	if err != nil {
		return err
	}
	if err := d.checkHostAllow(allow, t, f+".allow", b.Net, "构建"); err != nil {
		return err
	}
	b.Allow = allow

	d.spec.Build = b
	return nil
}

// ── [网络] ────────────────────────────────────────────────────────────

func (d *specDecoder) decodeNetwork(t *tval) error {
	const f = "网络"
	if t.kind != tvTable {
		return d.errf(t.line, f, "必须是表")
	}
	if err := d.checkKeys(t, f, "mode", "allow"); err != nil {
		return err
	}
	n := NetworkSpec{Line: t.line, Mode: NetNone}
	mode, hasMode, err := d.strField(t, f, "mode")
	if err != nil {
		return err
	}
	if hasMode && mode != "" {
		// v2 §D：「net = "allowlist" 仅此相位；运行期强制 none」。运行期写 allowlist
		// 就是在契约里承诺一条不存在的通路 ⇒ 拒，而不是悄悄降级（降级会让用户以为网通着）。
		if mode != NetNone {
			return d.errf(t.tab["mode"].line, f+".mode",
				"运行期网络只能是 %q（v2 §D：allowlist 只允许出现在 [构建] 相位），得到 %q", NetNone, mode)
		}
		n.Mode = NetNone
	}
	allow, _, err := d.strArrField(t, f, "allow")
	if err != nil {
		return err
	}
	if err := d.checkHostAllow(allow, t, f+".allow", n.Mode, "运行期"); err != nil {
		return err
	}
	n.Allow = allow
	d.spec.Network = n
	return nil
}

// checkHostAllow — allow 与 net 的一致性 + 域名形态（v2 §D：allow 只在 allowlist 下有意义）。
func (d *specDecoder) checkHostAllow(allow []string, t *tval, field, netMode, phase string) error {
	line := t.line
	if tv, ok := t.tab["allow"]; ok {
		line = tv.line
	}
	if netMode != NetAllowlist && len(allow) > 0 {
		return d.errf(line, field, "%s net=%q 时不允许写 allow（镜像外无出网，写了也不会生效）", phase, netMode)
	}
	if netMode == NetAllowlist && len(allow) == 0 {
		return d.errf(line, field, "%s net=\"allowlist\" 却给了空 allow：等于断网，把 net 写成 \"none\" 才符合意图", phase)
	}
	for i, h := range allow {
		f := fmt.Sprintf("%s[%d]", field, i)
		if h == "" {
			return d.errf(line, f, "不得为空串")
		}
		if strings.Contains(h, "://") {
			return d.errf(line, f, "只写主机名，不带协议：%q", h)
		}
		if strings.ContainsAny(h, "/ \t?#") {
			return d.errf(line, f, "只写主机名（可带 *. 前缀），不带路径/端口：%q", h)
		}
		if strings.Contains(h, ":") {
			return d.errf(line, f, "只写主机名，不带端口：%q", h)
		}
	}
	return nil
}

// ── [资源] ────────────────────────────────────────────────────────────

func (d *specDecoder) decodeResources(t *tval) error {
	const f = "资源"
	if t.kind != tvTable {
		return d.errf(t.line, f, "必须是表")
	}
	if err := d.checkKeys(t, f, "cpu", "mem_gb", "disk_gb"); err != nil {
		return err
	}
	r := ResourceSpec{Line: t.line}
	read := func(key string) (float64, bool, error) {
		tv, ok := t.tab[key]
		if !ok {
			return 0, false, nil
		}
		var v float64
		switch tv.kind {
		case tvInt:
			v = float64(tv.i)
		case tvFloat:
			v = tv.f
		default:
			return 0, true, d.errf(tv.line, f+"."+key, "必须是数字")
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, true, d.errf(tv.line, f+"."+key, "必须是有限数字")
		}
		if v <= 0 {
			return 0, true, d.errf(tv.line, f+"."+key, "必须 > 0，得到 %v（不设上限就别写这个键，写了就必须是正数）", v)
		}
		return v, true, nil
	}
	cpu, ok, err := read("cpu")
	if err != nil {
		return err
	}
	if !ok {
		return d.errf(t.line, f+".cpu", "必填（核数，可写小数如 1.5）")
	}
	r.CPU = cpu
	mem, ok, err := read("mem_gb")
	if err != nil {
		return err
	}
	if !ok {
		return d.errf(t.line, f+".mem_gb", "必填（GB）")
	}
	r.MemGB = mem
	disk, ok, err := read("disk_gb")
	if err != nil {
		return err
	}
	if !ok {
		return d.errf(t.line, f+".disk_gb", "必填（GB）")
	}
	r.DiskGB = disk
	d.spec.Resources = r
	return nil
}

// ── [环境] ────────────────────────────────────────────────────────────

// decodeEnv — [环境] 的键就是环境变量名（任意键值），另支持把一批变量收进 `vars = { ... }`
// 或 `[环境.vars]`（v2 §D 样例两种写法各出现一次，所以都收；同键冲突以字面量优先并报错，
// 不搞「后者覆盖前者」的静默语义）。
func (d *specDecoder) decodeEnv(t *tval) error {
	const f = "环境"
	if t.kind != tvTable {
		return d.errf(t.line, f, "必须是表")
	}
	for _, k := range t.ord {
		tv := t.tab[k]
		switch {
		case k == "vars" && tv.kind == tvTable:
			if err := d.decodeEnvVars(tv, f+".vars"); err != nil {
				return err
			}
			continue
		case tv.kind == tvTable:
			return d.errf(tv.line, f+"."+k, "环境变量只能是标量（字符串/数字/布尔），不能是表")
		case tv.kind == tvArray:
			return d.errf(tv.line, f+"."+k, "环境变量只能是标量（字符串/数字/布尔），不能是数组")
		}
		if !specEnvKeyRe.MatchString(k) {
			return d.errf(tv.line, f+"."+k, "不是合法的环境变量名（须匹配 ^[A-Za-z_][A-Za-z0-9_]*$）")
		}
		val, err := d.envScalar(tv, f+"."+k)
		if err != nil {
			return err
		}
		if old, dup := d.spec.Env[k]; dup && old != val {
			return d.errf(tv.line, f+"."+k, "环境变量重复且取值不同（先 %q 后 %q）", old, val)
		}
		d.spec.Env[k] = val
	}
	return nil
}

func (d *specDecoder) decodeEnvVars(t *tval, field string) error {
	for _, k := range t.ord {
		tv := t.tab[k]
		if tv.kind == tvTable || tv.kind == tvArray {
			return d.errf(tv.line, field+"."+k, "vars 里只能是标量")
		}
		if !specEnvKeyRe.MatchString(k) {
			return d.errf(tv.line, field+"."+k, "不是合法的环境变量名（须匹配 ^[A-Za-z_][A-Za-z0-9_]*$）")
		}
		val, err := d.envScalar(tv, field+"."+k)
		if err != nil {
			return err
		}
		if old, dup := d.spec.Env[k]; dup && old != val {
			return d.errf(tv.line, field+"."+k, "环境变量重复且取值不同（先 %q 后 %q）", old, val)
		}
		d.spec.Env[k] = val
	}
	return nil
}

func (d *specDecoder) envScalar(tv *tval, field string) (string, error) {
	switch tv.kind {
	case tvString:
		if strings.ContainsRune(tv.s, 0) {
			return "", d.errf(tv.line, field, "含 NUL 字节")
		}
		return tv.s, nil
	case tvInt:
		return strconv.FormatInt(tv.i, 10), nil
	case tvFloat:
		return strconv.FormatFloat(tv.f, 'g', -1, 64), nil
	case tvBool:
		return strconv.FormatBool(tv.b), nil
	default:
		return "", d.errf(tv.line, field, "环境变量只能是标量")
	}
}

// ── 字段读取小工具（都带字段名与行号，保证报错定位） ─────────────────────

func (d *specDecoder) checkKeys(t *tval, field string, allowed ...string) error {
	set := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		set[a] = true
	}
	for _, k := range t.ord {
		if !set[k] {
			return d.errf(t.tab[k].line, field+"."+k, "未知字段（此处允许：%s）", strings.Join(allowed, " / "))
		}
	}
	return nil
}

func (d *specDecoder) strField(t *tval, field, key string) (string, bool, error) {
	tv, ok := t.tab[key]
	if !ok {
		return "", false, nil
	}
	if tv.kind != tvString {
		return "", true, d.errf(tv.line, field+"."+key, "必须是字符串")
	}
	return tv.s, true, nil
}

func (d *specDecoder) strArrField(t *tval, field, key string) ([]string, bool, error) {
	tv, ok := t.tab[key]
	if !ok {
		return nil, false, nil
	}
	if tv.kind != tvArray {
		return nil, true, d.errf(tv.line, field+"."+key, "必须是字符串数组，如 [\"a\", \"b\"]")
	}
	out := make([]string, 0, len(tv.arr))
	for i, el := range tv.arr {
		if el.kind != tvString {
			return nil, true, d.errf(el.line, fmt.Sprintf("%s.%s[%d]", field, key, i), "必须是字符串")
		}
		out = append(out, el.s)
	}
	return out, true, nil
}

func sortedNames(svcs []ServiceSpec) []string {
	out := make([]string, 0, len(svcs))
	for _, s := range svcs {
		out = append(out, s.Name)
	}
	return out
}

// ══════════════════════════════════════════════════════════════════════
// TOML 子集解析器
//
// 只用标准库、按字节手工扫：需求是「明确知道它接受什么」，而通用 TOML 库会把
// 我们并不想支持的写法（多行字符串、日期、嵌套数组表边角）一并收下——
// 契约解析面越窄，越容易证明「茧改不动契约」这类性质（v2 §B）。
// ══════════════════════════════════════════════════════════════════════

type tvKind uint8

const (
	tvString tvKind = iota + 1
	tvInt
	tvFloat
	tvBool
	tvArray
	tvTable
)

func (k tvKind) String() string {
	switch k {
	case tvString:
		return "字符串"
	case tvInt:
		return "整数"
	case tvFloat:
		return "浮点"
	case tvBool:
		return "布尔"
	case tvArray:
		return "数组"
	case tvTable:
		return "表"
	default:
		return "未知"
	}
}

// tval — 解析树节点。数组/表用指针，便于「先建空表再往里塞键」。
type tval struct {
	kind tvKind
	line int
	s    string
	i    int64
	f    float64
	b    bool
	arr  []*tval
	tab  map[string]*tval
	ord  []string // 保持声明顺序：报错与遍历要确定性
	// explicit — 该表是否由 `[x]` 头显式声明过（用于「表重复定义」判定，
	// 隐式表可以由点号键或子表头先造出来，之后仍允许 `[x]` 补显式声明）。
	explicit bool
}

func newTable(line int) *tval {
	return &tval{kind: tvTable, line: line, tab: map[string]*tval{}}
}

// isSecretRef — 密钥值是否为「引用形态」：整值匹配 `${VAR}` 或 `secret://<name>`。
// 只有字符串标量才可能是引用——数组/表/数字/布尔一律当字面值处理（照样拒），
// 免得 `token = ["${A}"]` 或 `token = 42` 从侧门溜进来。
func isSecretRef(t *tval) bool {
	if t.kind != tvString {
		return false
	}
	return specSecretEnvRefRe.MatchString(t.s) || specSecretURIRefRe.MatchString(t.s)
}

// isZeroValue — 「值非空」判定（密钥规则用）：空串/空数组/空表算空。
func (t *tval) isZeroValue() bool {
	switch t.kind {
	case tvString:
		return t.s == ""
	case tvArray:
		return len(t.arr) == 0
	case tvTable:
		return len(t.ord) == 0
	default:
		return false
	}
}

type tomlParser struct {
	file string
	src  []byte
	i    int
	line int
}

func (p *tomlParser) errf(line int, format string, a ...any) error {
	return &SpecError{File: p.file, Line: line, Msg: fmt.Sprintf(format, a...)}
}

func (p *tomlParser) eof() bool { return p.i >= len(p.src) }

func (p *tomlParser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.src[p.i]
}

func (p *tomlParser) next() byte {
	c := p.src[p.i]
	p.i++
	if c == '\n' {
		p.line++
	}
	return c
}

func (p *tomlParser) skipSpaces() {
	for !p.eof() {
		switch p.src[p.i] {
		case ' ', '\t', '\r':
			p.i++
		default:
			return
		}
	}
}

// skipComment — 吃掉 `#` 到行尾（不含换行）。
func (p *tomlParser) skipComment() {
	if p.peek() != '#' {
		return
	}
	for !p.eof() && p.src[p.i] != '\n' {
		p.i++
	}
}

// skipSpaceNL — 空格 + 换行 + 注释（文档级与数组内共用）。
func (p *tomlParser) skipSpaceNL() {
	for !p.eof() {
		switch p.src[p.i] {
		case ' ', '\t', '\r':
			p.i++
		case '\n':
			p.next()
		case '#':
			p.skipComment()
		default:
			return
		}
	}
}

func parseTOMLSubset(file string, src []byte) (*tval, error) {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")) // BOM：编辑器留下的，静默吃掉比报错友好
	p := &tomlParser{file: file, src: src, line: 1}
	root := newTable(1)
	cur := root
	for {
		p.skipSpaceNL()
		if p.eof() {
			break
		}
		if p.peek() == '[' {
			path, isArray, line, err := p.parseHeader()
			if err != nil {
				return nil, err
			}
			t, err := resolveHeader(root, path, isArray, line, p.file)
			if err != nil {
				return nil, err
			}
			cur = t
			continue
		}
		keyLine := p.line
		path, err := p.parseKeyPath()
		if err != nil {
			return nil, err
		}
		p.skipSpaces()
		if p.peek() != '=' {
			return nil, p.errf(p.line, "键 %q 后应为 '='（如需注释，'#' 要另起在行尾）", strings.Join(path, "."))
		}
		p.next() // '='
		p.skipSpaces()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if err := setPath(cur, path, val, keyLine, p.file); err != nil {
			return nil, err
		}
		// 值之后只允许「空白 + 可选注释 + 换行/EOF」：多余字符几乎总是漏了引号或逗号，
		// 静默吞掉会导致后面整段错位，所以这里必须报。
		p.skipSpaces()
		p.skipComment()
		if p.eof() {
			break
		}
		if c := p.peek(); c != '\n' && c != '\r' {
			return nil, p.errf(p.line, "键 %q 的值后有多余字符（多半是漏了引号/逗号）", strings.Join(path, "."))
		}
	}
	return root, nil
}

func (p *tomlParser) parseHeader() ([]string, bool, int, error) {
	line := p.line
	p.next() // '['
	isArray := false
	if p.peek() == '[' {
		isArray = true
		p.next()
	}
	p.skipSpaces()
	path, err := p.parseKeyPath()
	if err != nil {
		return nil, false, 0, err
	}
	if len(path) == 0 {
		return nil, false, 0, p.errf(line, "表名为空")
	}
	p.skipSpaces()
	if p.peek() != ']' {
		return nil, false, 0, p.errf(p.line, "表 %q 未闭合（缺 ']'）", strings.Join(path, "."))
	}
	p.next()
	if isArray {
		if p.peek() != ']' {
			return nil, false, 0, p.errf(p.line, "数组表 %q 未闭合（缺 ']]'）", strings.Join(path, "."))
		}
		p.next()
	}
	return path, isArray, line, nil
}

// resolveHeader — 定位（必要时创建）表头所指的表。
//
// `[[服务]]` 每次出现都**追加**一个新表元素；`[a.b]` 逐段下行，遇到数组表就进最后一个元素
// （这正是 TOML 里 `[[服务]]` 后接 `[服务.x]` 的语义，虽然本契约用不到，但语义要自洽）。
func resolveHeader(root *tval, path []string, isArray bool, line int, file string) (*tval, error) {
	errf := func(format string, a ...any) error {
		return &SpecError{File: file, Line: line, Field: strings.Join(path, "."), Msg: fmt.Sprintf(format, a...)}
	}
	cur := root
	for _, seg := range path[:len(path)-1] {
		tv, ok := cur.tab[seg]
		if !ok {
			nt := newTable(line)
			cur.tab[seg] = nt
			cur.ord = append(cur.ord, seg)
			cur = nt
			continue
		}
		switch tv.kind {
		case tvTable:
			cur = tv
		case tvArray:
			if len(tv.arr) == 0 {
				return nil, errf("路径中的 %q 是空数组表", seg)
			}
			last := tv.arr[len(tv.arr)-1]
			if last.kind != tvTable {
				return nil, errf("路径中的 %q 不是表数组", seg)
			}
			cur = last
		default:
			return nil, errf("表名与已有键冲突（%q 已是%s）", seg, tv.kind)
		}
	}
	last := path[len(path)-1]
	if isArray {
		tv, ok := cur.tab[last]
		if !ok {
			tv = &tval{kind: tvArray, line: line}
			cur.tab[last] = tv
			cur.ord = append(cur.ord, last)
		}
		if tv.kind != tvArray {
			return nil, errf("%q 已作为%s定义过，不能再当数组表", last, tv.kind)
		}
		nt := newTable(line)
		tv.arr = append(tv.arr, nt)
		return nt, nil
	}
	tv, ok := cur.tab[last]
	if !ok {
		nt := newTable(line)
		nt.explicit = true
		cur.tab[last] = nt
		cur.ord = append(cur.ord, last)
		return nt, nil
	}
	if tv.kind != tvTable {
		return nil, errf("%q 已作为%s定义过，不能再当表", last, tv.kind)
	}
	if tv.explicit {
		return nil, errf("表重复定义（首次声明在第 %d 行）", tv.line)
	}
	tv.explicit = true
	return tv, nil
}

func (p *tomlParser) parseKeyPath() ([]string, error) {
	var segs []string
	for {
		seg, err := p.parseKeySegment()
		if err != nil {
			return nil, err
		}
		segs = append(segs, seg)
		p.skipSpaces()
		if p.peek() == '.' {
			p.next()
			p.skipSpaces()
			continue
		}
		return segs, nil
	}
}

func (p *tomlParser) parseKeySegment() (string, error) {
	if p.eof() {
		return "", p.errf(p.line, "缺少键名")
	}
	switch p.peek() {
	case '"':
		return p.parseBasicString()
	case '\'':
		return p.parseLiteralString()
	}
	start := p.i
	for !p.eof() {
		c := p.src[p.i]
		if c < 0x20 || c == ' ' || c == '\t' || c == '\r' || c == '\n' ||
			c == '=' || c == '.' || c == ',' || c == '[' || c == ']' || c == '{' || c == '}' ||
			c == '#' || c == '"' || c == '\'' {
			break
		}
		p.i++
	}
	if p.i == start {
		return "", p.errf(p.line, "缺少键名")
	}
	// 裸键允许非 ASCII（设计稿里就是 茧/服务/端口/构建/网络/资源/环境 这些中文段名），
	// 这是对 TOML 1.0「裸键仅 ASCII」的刻意放宽，换来契约能与设计稿逐字对应。
	return string(p.src[start:p.i]), nil
}

func (p *tomlParser) parseValue() (*tval, error) {
	if p.eof() {
		return nil, p.errf(p.line, "缺少值")
	}
	line := p.line
	switch c := p.peek(); {
	case c == '"':
		if p.hasPrefix(`"""`) || p.hasPrefix(`'''`) {
			return nil, p.errf(line, "不支持多行字符串（本子集只收单行字符串）")
		}
		s, err := p.parseBasicString()
		if err != nil {
			return nil, err
		}
		return &tval{kind: tvString, line: line, s: s}, nil
	case c == '\'':
		s, err := p.parseLiteralString()
		if err != nil {
			return nil, err
		}
		return &tval{kind: tvString, line: line, s: s}, nil
	case c == '[':
		return p.parseArray()
	case c == '{':
		return p.parseInlineTable()
	case c == 't' || c == 'f':
		if p.consumeWord("true") {
			return &tval{kind: tvBool, line: line, b: true}, nil
		}
		if p.consumeWord("false") {
			return &tval{kind: tvBool, line: line, b: false}, nil
		}
		return nil, p.errf(line, "无法识别的值（以 't'/'f' 开头却不是 true/false）")
	default:
		return p.parseNumber()
	}
}

func (p *tomlParser) hasPrefix(s string) bool {
	if p.i+len(s) > len(p.src) {
		return false
	}
	return string(p.src[p.i:p.i+len(s)]) == s
}

func (p *tomlParser) consumeWord(w string) bool {
	if !p.hasPrefix(w) {
		return false
	}
	end := p.i + len(w)
	if end < len(p.src) {
		switch p.src[end] {
		case ' ', '\t', '\r', '\n', ',', ']', '}', '#':
		default:
			return false // trueish / falsey 之类不是布尔
		}
	}
	p.i = end
	return true
}

func (p *tomlParser) parseNumber() (*tval, error) {
	line := p.line
	start := p.i
	for !p.eof() {
		c := p.src[p.i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',' || c == ']' || c == '}' || c == '#' {
			break
		}
		p.i++
	}
	tok := string(p.src[start:p.i])
	if tok == "" {
		return nil, p.errf(line, "缺少值")
	}
	clean := strings.ReplaceAll(tok, "_", "")
	if n, err := strconv.ParseInt(clean, 0, 64); err == nil {
		return &tval{kind: tvInt, line: line, i: n}, nil
	}
	if f, err := strconv.ParseFloat(clean, 64); err == nil {
		return &tval{kind: tvFloat, line: line, f: f}, nil
	}
	return nil, p.errf(line, "无法识别的值 %q（字符串必须加引号）", tok)
}

func (p *tomlParser) parseBasicString() (string, error) {
	line := p.line
	p.next() // '"'
	var b strings.Builder
	for {
		if p.eof() {
			return "", p.errf(line, "字符串未闭合（缺 '\"'）")
		}
		c := p.next()
		switch c {
		case '"':
			return b.String(), nil
		case '\n':
			return "", p.errf(line, "字符串里出现裸换行（未闭合？）")
		case '\\':
			if p.eof() {
				return "", p.errf(line, "转义符后缺少字符")
			}
			e := p.next()
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case '/':
				b.WriteByte('/')
			case 'u', 'U':
				n := 4
				if e == 'U' {
					n = 8
				}
				if p.i+n > len(p.src) {
					return "", p.errf(p.line, "\\%c 转义不足 %d 位", e, n)
				}
				hex := string(p.src[p.i : p.i+n])
				p.i += n
				v, err := strconv.ParseUint(hex, 16, 32)
				if err != nil {
					return "", p.errf(p.line, "\\%c 转义不是合法 hex：%q", e, hex)
				}
				b.WriteRune(rune(v))
			default:
				return "", p.errf(p.line, "不支持的转义 \\%c", e)
			}
		default:
			b.WriteByte(c)
		}
	}
}

func (p *tomlParser) parseLiteralString() (string, error) {
	line := p.line
	p.next() // '\''
	start := p.i
	for !p.eof() {
		if p.src[p.i] == '\'' {
			s := string(p.src[start:p.i])
			p.i++
			return s, nil
		}
		if p.src[p.i] == '\n' {
			return "", p.errf(line, "字面字符串里出现裸换行（未闭合？）")
		}
		p.i++
	}
	return "", p.errf(line, "字面字符串未闭合（缺 '\\''）")
}

func (p *tomlParser) parseArray() (*tval, error) {
	line := p.line
	p.next() // '['
	arr := []*tval{}
	for {
		p.skipSpaceNL()
		if p.eof() {
			return nil, p.errf(line, "数组未闭合（缺 ']'）")
		}
		if p.peek() == ']' {
			p.next()
			break
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
		p.skipSpaceNL()
		if p.eof() {
			return nil, p.errf(line, "数组未闭合（缺 ']'）")
		}
		switch p.peek() {
		case ',':
			p.next()
			continue
		case ']':
			p.next()
			goto done
		default:
			return nil, p.errf(p.line, "数组元素后应为 ',' 或 ']'")
		}
	}
done:
	return &tval{kind: tvArray, line: line, arr: arr}, nil
}

func (p *tomlParser) parseInlineTable() (*tval, error) {
	line := p.line
	p.next() // '{'
	t := newTable(line)
	p.skipSpaces()
	if p.peek() == '}' {
		p.next()
		return t, nil
	}
	for {
		p.skipSpaces()
		keyLine := p.line
		path, err := p.parseKeyPath()
		if err != nil {
			return nil, err
		}
		p.skipSpaces()
		if p.peek() != '=' {
			return nil, p.errf(p.line, "行内表里键 %q 后应为 '='", strings.Join(path, "."))
		}
		p.next()
		p.skipSpaces()
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if err := setPath(t, path, v, keyLine, p.file); err != nil {
			return nil, err
		}
		p.skipSpaces()
		if p.eof() {
			return nil, p.errf(line, "行内表未闭合（缺 '}'）")
		}
		switch p.peek() {
		case ',':
			p.next()
			continue
		case '}':
			p.next()
			return t, nil
		case '\n', '\r':
			return nil, p.errf(line, "行内表不得跨行（缺 '}'）")
		default:
			return nil, p.errf(p.line, "行内表里应为 ',' 或 '}'")
		}
	}
}

// setPath — 把 value 塞进 table 的 path 位置（支持点号键），重复键报错。
func setPath(t *tval, path []string, v *tval, line int, file string) error {
	errf := func(format string, a ...any) error {
		return &SpecError{File: file, Line: line, Field: strings.Join(path, "."), Msg: fmt.Sprintf(format, a...)}
	}
	cur := t
	for _, seg := range path[:len(path)-1] {
		tv, ok := cur.tab[seg]
		if !ok {
			nt := newTable(line)
			cur.tab[seg] = nt
			cur.ord = append(cur.ord, seg)
			cur = nt
			continue
		}
		if tv.kind != tvTable {
			return errf("键 %q 已是%s，不能当表用", seg, tv.kind)
		}
		cur = tv
	}
	last := path[len(path)-1]
	if _, dup := cur.tab[last]; dup {
		return errf("键 %q 重复定义", last)
	}
	cur.tab[last] = v
	cur.ord = append(cur.ord, last)
	return nil
}
