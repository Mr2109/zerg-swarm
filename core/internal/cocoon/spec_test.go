// spec_test.go — 六格判据（v2/v3 §D 定稿）＋ 八格护栏：
//
//	① TestSpecParseFullSample           完整样例逐字段对（服务/端口/构建/网络/资源/环境 + 拓扑序 + 无警告）
//	② TestSpecRejectsSecretValues       密钥值必拒（含嵌套、含非字符串）；明文拒、空串放行
//	②b TestSpecSecretRefForms           `${VAR}` / `secret://name` 引用放行；畸形引用与明文同拒
//	③ TestSpecRejectsBadDigest          digest 非 sha256:<64hex> 必拒；大写 hex 归一
//	④ TestSpecRejectsDependencyCycle    成环必拒，且报出环路径；缺失依赖/自依赖同拒
//	⑤ TestSpecRejectsDuplicateServiceName  服务名重复必拒
//	⑥ TestSpecRejectsBadPorts           端口越界/重复（inner 与固定 host 各一组）/bind 非 IP 必拒
//	⑦ TestSpecWarnsMissingDigest        缺 digest 不报错，但必须记 Warning
//	⑧ TestSpecDefaults                  默认值：TZ/LANG、bind、host=auto、readiness.timeout=30s、net=none
//	⑨ TestSpecRejectsStructural        未知表/未知字段/缺段/无服务 必拒（防拼错静默降级）
//	⑩ TestSpecTOMLSubsetFeatures        注释/行内数组/跨行数组/嵌套表/字面串/转义 都能读
//	⑪ TestSpecRejectsNetworkPolicy      [网络] 运行期 allowlist 必拒；allow 与 net 不一致必拒
//	⑫ TestSpecErrorFormat               错误文案：文件:行:字段:说明（含自定义文件名）
//	⑬ TestSpecLoadFile                  LoadSpec 读盘；文件不存在返回 fs.ErrNotExist（不被包装成 SpecError）
//	⑭ TestSpecConcurrentParse           解析器无共享可变状态（-race 下多协程并发解析）
//	⑮ TestSpecParserNeverPanicsOnGarbage 垃圾输入只报错、不 panic（手写状态机的越界防线）
//
// 每一格负例都断言「行号 + 字段名 + 文案前缀」，因为「报错但看不出在哪」等于没报。
package cocoon

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── 夹具 ──────────────────────────────────────────────────────────────

const specValidDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// specMinimalTmpl — 最小可过契约，digest 由模板参数给（digest 一组负例靠它改）。
const specMinimalTmpl = `[茧]
name   = "shop"
image  = "python:3.12-slim"
digest = "%s"

[[服务]]
name  = "api"
entry = ["uvicorn", "app:api"]

[资源]
cpu     = 1
mem_gb  = 2
disk_gb = 10
`

func specWithDigest(d string) string { return fmt.Sprintf(specMinimalTmpl, d) }

// specFull — v2 §D 的设计稿样例（补上 hashes/net/资源/环境细节），用来验「字段逐项对」。
const specFull = `# 店铺茧：多进程（db + api）+ 端口 + 构建期 + 资源 + 环境
[茧]
name    = "shop"
image   = "python:3.12-slim"
digest  = "sha256:0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef"
arch    = "arm64"
workdir = "/app"

[[服务]]
name       = "api"
entry      = ["uvicorn", "app:api"]
depends_on = ["db"]
readiness  = { port = 8000, path = "/healthz", timeout = "30s" }

[[服务]]
name  = "db"
entry = ["postgres", "-D", "/var/lib/pg"]

[[端口]]
inner = 8000
host  = "auto"
bind  = "127.0.0.1"

[[端口]]
inner = 5432
host  = 15432

[构建]
install = ["pip install --require-hashes -r requirements.txt"]
cache   = "/root/.cache/pip"
net     = "allowlist"
allow   = ["pypi.org", "files.pythonhosted.org"]

[网络]
mode = "none"

[资源]
cpu     = 2
mem_gb  = 4
disk_gb = 20

[环境]
TZ   = "Asia/Shanghai"
LANG = "C.UTF-8"
vars = { APP_ENV = "prod", WORKERS = 4 }
`

// lineOf — 1-based 行号（找不到返回 -1，测试里会因此直接红）。
func lineOf(src, needle string) int { return lineOfAt(src, needle, 1) }

// lineOfAt — 第 n 次出现所在的行号（n 从 1 起）。
func lineOfAt(src, needle string, n int) int {
	seen := 0
	for i, ln := range strings.Split(src, "\n") {
		if strings.Contains(ln, needle) {
			seen++
			if seen == n {
				return i + 1
			}
		}
	}
	return -1
}

func mustParseSpec(t *testing.T, src string) *Spec {
	t.Helper()
	sp, err := ParseSpec([]byte(src))
	if err != nil {
		t.Fatalf("期望解析成功，却失败：%v\n--- 契约 ---\n%s", err, src)
	}
	return sp
}

// wantSpecErr — 负例统一断言：必须是 *SpecError，且 字段名/说明/行号/文案前缀 全对。
func wantSpecErr(t *testing.T, src, wantField, wantMsg string, wantLine int) *SpecError {
	t.Helper()
	_, err := ParseSpec([]byte(src))
	if err == nil {
		t.Fatalf("期望解析失败（field=%s），却成功了\n--- 契约 ---\n%s", wantField, src)
	}
	se, ok := AsSpecError(err)
	if !ok {
		t.Fatalf("错误不是 *SpecError：%T：%v", err, err)
	}
	if se.Field != wantField {
		t.Fatalf("字段名不符：want %q got %q（错误：%v）", wantField, se.Field, err)
	}
	if !strings.Contains(se.Msg, wantMsg) {
		t.Fatalf("错误说明未提及 %q：%q", wantMsg, se.Msg)
	}
	if se.Line <= 0 {
		t.Fatalf("错误缺行号：%v", err)
	}
	if wantLine > 0 && se.Line != wantLine {
		t.Fatalf("行号不符：want %d got %d（错误：%v）", wantLine, se.Line, err)
	}
	wantPrefix := fmt.Sprintf("cocoon.toml:%d: %s: ", se.Line, se.Field)
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Fatalf("错误文案格式不符：want 前缀 %q got %q", wantPrefix, err.Error())
	}
	return se
}

// ── ① 完整样例逐字段对 ────────────────────────────────────────────────

func TestSpecParseFullSample(t *testing.T) {
	sp := mustParseSpec(t, specFull)

	if sp.File != SpecFileName {
		t.Fatalf("File = %q，want %q", sp.File, SpecFileName)
	}

	// [茧]
	c := sp.Cocoon
	if c.Name != "shop" || c.Image != "python:3.12-slim" || c.Arch != "arm64" || c.Workdir != "/app" {
		t.Fatalf("[茧] 解析不符：%+v", c)
	}
	if c.Digest != specValidDigest {
		t.Fatalf("digest 未归一为小写：%q", c.Digest)
	}
	if !sp.HasDigest() {
		t.Fatal("HasDigest() = false，期望 true")
	}

	// [[服务]] + depends_on + readiness
	if len(sp.Services) != 2 {
		t.Fatalf("服务数 = %d，want 2", len(sp.Services))
	}
	api := sp.Services[0]
	if api.Name != "api" {
		t.Fatalf("服务[0].Name = %q", api.Name)
	}
	if want := []string{"uvicorn", "app:api"}; !slices.Equal(api.Entry, want) {
		t.Fatalf("服务[0].Entry = %v，want %v", api.Entry, want)
	}
	if want := []string{"db"}; !slices.Equal(api.DependsOn, want) {
		t.Fatalf("服务[0].DependsOn = %v，want %v", api.DependsOn, want)
	}
	if api.Readiness == nil {
		t.Fatal("服务[0].Readiness = nil，want 非空")
	}
	if api.Readiness.Port != 8000 || api.Readiness.Path != "/healthz" || api.Readiness.Timeout != 30*time.Second {
		t.Fatalf("服务[0].Readiness = %+v", *api.Readiness)
	}
	db := sp.Services[1]
	if db.Name != "db" || !slices.Equal(db.Entry, []string{"postgres", "-D", "/var/lib/pg"}) {
		t.Fatalf("服务[1] = %+v", db)
	}
	if db.DependsOn != nil {
		t.Fatalf("服务[1].DependsOn = %v，want nil", db.DependsOn)
	}
	if db.Readiness != nil {
		t.Fatalf("服务[1].Readiness = %+v，want nil", *db.Readiness)
	}
	if _, ok := sp.Service("db"); !ok {
		t.Fatal("Service(\"db\") 未找到")
	}

	// 拓扑序：依赖在前（db 先于 api）
	if want := []string{"db", "api"}; !slices.Equal(sp.Order, want) {
		t.Fatalf("Order = %v，want %v", sp.Order, want)
	}

	// [[端口]]
	if len(sp.Ports) != 2 {
		t.Fatalf("端口数 = %d，want 2", len(sp.Ports))
	}
	p0, p1 := sp.Ports[0], sp.Ports[1]
	if p0.Inner != 8000 || p0.Host != "auto" || p0.Bind != "127.0.0.1" {
		t.Fatalf("端口[0] = %+v", p0)
	}
	if _, fixed := p0.HostPort(); fixed {
		t.Fatal("端口[0] host=auto 不该解析出固定端口")
	}
	if p1.Inner != 5432 || p1.Bind != DefaultBind {
		t.Fatalf("端口[1] = %+v（bind 应取默认 %s）", p1, DefaultBind)
	}
	if n, fixed := p1.HostPort(); !fixed || n != 15432 {
		t.Fatalf("端口[1].HostPort() = (%d,%v)，want (15432,true)", n, fixed)
	}

	// [构建]
	b := sp.Build
	if want := []string{"pip install --require-hashes -r requirements.txt"}; !slices.Equal(b.Install, want) {
		t.Fatalf("构建.install = %v", b.Install)
	}
	if b.Cache != "/root/.cache/pip" || b.Net != NetAllowlist {
		t.Fatalf("构建 = %+v", b)
	}
	if want := []string{"pypi.org", "files.pythonhosted.org"}; !slices.Equal(b.Allow, want) {
		t.Fatalf("构建.allow = %v", b.Allow)
	}

	// [网络] / [资源]
	if sp.Network.Mode != NetNone || len(sp.Network.Allow) != 0 {
		t.Fatalf("网络 = %+v", sp.Network)
	}
	if sp.Resources.CPU != 2 || sp.Resources.MemGB != 4 || sp.Resources.DiskGB != 20 {
		t.Fatalf("资源 = %+v", sp.Resources)
	}

	// [环境]：固定默认项 + 字面量 + vars 展开 + 数字归一为字符串
	wantEnv := map[string]string{
		"TZ": "Asia/Shanghai", "LANG": "C.UTF-8",
		"APP_ENV": "prod", "WORKERS": "4",
	}
	for k, v := range wantEnv {
		if got, ok := sp.Env[k]; !ok || got != v {
			t.Fatalf("Env[%q] = %q,%v，want %q", k, got, ok, v)
		}
	}
	if len(sp.Env) != len(wantEnv) {
		t.Fatalf("Env 多出键：%v", sp.Env)
	}

	// 有 digest ⇒ 不该有警告
	if len(sp.Warnings) != 0 {
		t.Fatalf("Warnings = %v，want 空", sp.Warnings)
	}
}

// ── ② 密钥值必拒（引用形态除外） ─────────────────────────────────────

func TestSpecRejectsSecretValues(t *testing.T) {
	// 明文值 ⇒ 拒（原来那格「非空即拒」在放开引用后收窄为「明文拒」）
	envSecret := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10

[环境]
API_TOKEN = "s3cr3t"
`
	se := wantSpecErr(t, envSecret, "环境.API_TOKEN", "密钥", lineOf(envSecret, "API_TOKEN"))
	if !strings.Contains(se.Msg, "宿主注入") {
		t.Fatalf("错误文案该告诉用户「由宿主注入」：%q", se.Msg)
	}

	// 嵌套在行内表里也逃不掉（readiness 里塞 token 是最容易漏的一种）
	rtSecret := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]
readiness = { port = 8000, token = "abc" }

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`
	wantSpecErr(t, rtSecret, "服务[0].readiness.token", "密钥", lineOf(rtSecret, "readiness = {"))

	// 未知段里夹带（密钥扫描先于字段校验：安全错不能被拼写错盖掉）
	unknownSecret := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10

[构建]
password = "hunter2"
`
	wantSpecErr(t, unknownSecret, "构建.password", "密钥", lineOf(unknownSecret, "password"))

	// 非字符串的密钥值同样拒（数字/布尔也是值）
	numSecret := strings.Replace(envSecret, `API_TOKEN = "s3cr3t"`, `API_TOKEN = 42`, 1)
	wantSpecErr(t, numSecret, "环境.API_TOKEN", "密钥", lineOf(numSecret, "API_TOKEN"))

	// 空串放行：表示「此键存在、值由宿主注入」
	okSrc := strings.Replace(envSecret, `API_TOKEN = "s3cr3t"`, `API_TOKEN = ""`, 1)
	sp := mustParseSpec(t, okSrc)
	if v, ok := sp.Env["API_TOKEN"]; !ok || v != "" {
		t.Fatalf("Env[API_TOKEN] = %q,%v，want \"\",true", v, ok)
	}
}

// ── ②b 密钥引用形态：${VAR} / secret://name 放行，其余非空拒 ──────────

// secretRefSpec — 把 [环境].GITHUB_TOKEN 建成指定原始值（raw 是 TOML 原文，便于同时测字符串/数字/布尔）。
func secretRefSpec(raw string) string {
	return `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10

[环境]
GITHUB_TOKEN = ` + raw + "\n"
}

func TestSpecSecretRefForms(t *testing.T) {
	// ①〜② 两种引用形态放行，且**原样保留**（值留给宿主注入，契约只记「去哪儿取」）
	pass := []struct{ name, raw, want string }{
		{"① ${VAR} 引用", `"${GITHUB_TOKEN}"`, "${GITHUB_TOKEN}"},
		{"② secret:// 引用", `"secret://github"`, "secret://github"},
	}
	for _, tc := range pass {
		t.Run(tc.name, func(t *testing.T) {
			sp := mustParseSpec(t, secretRefSpec(tc.raw))
			if got, ok := sp.Env["GITHUB_TOKEN"]; !ok || got != tc.want {
				t.Fatalf("Env[GITHUB_TOKEN] = %q,%v，want %q", got, ok, tc.want)
			}
		})
	}

	// ③〜④ 明文与畸形引用一律拒：报错文案必须把「只收引用形态」和两种示例都说清
	reject := []struct{ name, raw string }{
		{"③ 明文短语", `"s3cr3t"`},
		{"③ GitHub 令牌明文", `"ghp_0123456789abcdefghijklmnopqrstuvwxyz"`},
		{"④ 空引用", `"${}"`},
		{"④ 小写变量名", `"${lower}"`},
		{"④ 引用只占半截", `"x-${GITHUB_TOKEN}"`},
		{"④ secret:// 无名字", `"secret://"`},
		{"④ secret:// 名字含大写", `"secret://GitHub"`},
		{"④ secret:// 名字含斜杠", `"secret://a/b"`},
		{"数字字面值", `42`},
		{"布尔字面值", `true`},
		{"数组包引用", `["${GITHUB_TOKEN}"]`},
	}
	for _, tc := range reject {
		t.Run(tc.name, func(t *testing.T) {
			src := secretRefSpec(tc.raw)
			se := wantSpecErr(t, src, "环境.GITHUB_TOKEN", "引用形态", lineOf(src, "GITHUB_TOKEN"))
			for _, want := range []string{"${VAR}", "${GITHUB_TOKEN}", "secret://<name>", "secret://github"} {
				if !strings.Contains(se.Msg, want) {
					t.Fatalf("错误文案缺 %q（用户不知道怎么写才对）：%q", want, se.Msg)
				}
			}
		})
	}

	// 键名匹配不受影响：不命中密钥规则的键随便写明文
	if sp := mustParseSpec(t, strings.Replace(secretRefSpec(`"${GITHUB_TOKEN}"`),
		"GITHUB_TOKEN", "REPO_URL", 1)); sp.Env["REPO_URL"] != "${GITHUB_TOKEN}" {
		t.Fatalf("非密钥键被误伤：%v", sp.Env)
	}
}

// ── ③ digest 格式必拒 ─────────────────────────────────────────────────

func TestSpecRejectsBadDigest(t *testing.T) {
	cases := []struct {
		name    string
		digest  string
		wantMsg string
	}{
		{"非 hex", "sha256:xyz", "sha256:<64"},
		{"算法不对", "md5:" + strings.Repeat("a", 32), "sha256:<64"},
		{"只有 63 位", "sha256:" + strings.Repeat("a", 63), "sha256:<64"},
		{"多到 65 位", "sha256:" + strings.Repeat("a", 65), "sha256:<64"},
		{"缺前缀", strings.Repeat("a", 64), "sha256:<64"},
		{"含非 hex 字符", "sha256:" + strings.Repeat("z", 64), "sha256:<64"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := specWithDigest(tc.digest)
			wantSpecErr(t, src, "茧.digest", tc.wantMsg, lineOf(src, "digest"))
		})
	}

	// 大写 hex 是合法 hex：收，但归一为小写（下游比对/落盘只认一种形态）
	upper := strings.ToUpper(specValidDigest) // "SHA256:..." 前缀大小写一起变
	if sp := mustParseSpec(t, specWithDigest(strings.Replace(upper, "SHA256:", "sha256:", 1))); sp.Cocoon.Digest == "" {
		t.Fatal("大写 hex 被拒了")
	}
	if sp := mustParseSpec(t, specWithDigest("sha256:"+strings.ToUpper(strings.TrimPrefix(specValidDigest, "sha256:")))); sp.Cocoon.Digest != specValidDigest {
		t.Fatalf("大写 hex 未归一：%q", sp.Cocoon.Digest)
	}
}

// ── ④ depends_on 必须无环 ─────────────────────────────────────────────

func TestSpecRejectsDependencyCycle(t *testing.T) {
	cycle2 := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]
depends_on = ["db"]

[[服务]]
name  = "db"
entry = ["b"]
depends_on = ["api"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`
	se := wantSpecErr(t, cycle2, "服务[1].depends_on", "循环依赖", lineOf(cycle2, `depends_on = ["api"]`))
	for _, want := range []string{"api", "db", "→"} {
		if !strings.Contains(se.Msg, want) {
			t.Fatalf("环路径未报全（缺 %q）：%q", want, se.Msg)
		}
	}

	// 三元环（a→b→c→a）：环上每个名字都要出现在文案里
	cycle3 := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "a"
entry = ["a"]
depends_on = ["b"]

[[服务]]
name  = "b"
entry = ["b"]
depends_on = ["c"]

[[服务]]
name  = "c"
entry = ["c"]
depends_on = ["a"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`
	se3 := wantSpecErr(t, cycle3, "服务[2].depends_on", "循环依赖", 0)
	for _, want := range []string{"a", "b", "c"} {
		if !strings.Contains(se3.Msg, want) {
			t.Fatalf("三元环未报全（缺 %q）：%q", want, se3.Msg)
		}
	}

	// 自依赖
	self := strings.Replace(cycle2, `depends_on = ["db"]`, `depends_on = ["api"]`, 1)
	wantSpecErr(t, self, "服务[0].depends_on", "依赖自身", lineOf(self, `["api"]`))

	// 依赖不存在的服务
	ghost := strings.Replace(cycle2, `depends_on = ["db"]`, `depends_on = ["nope"]`, 1)
	wantSpecErr(t, ghost, "服务[0].depends_on", "不存在", lineOf(ghost, `["nope"]`))

	// 同一依赖写两遍
	dup := strings.Replace(cycle2, `depends_on = ["db"]`, `depends_on = ["db", "db"]`, 1)
	wantSpecErr(t, dup, "服务[0].depends_on[1]", "重复依赖", lineOf(dup, `["db", "db"]`))
}

// ── ⑤ 服务名唯一 ──────────────────────────────────────────────────────

func TestSpecRejectsDuplicateServiceName(t *testing.T) {
	src := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]

[[服务]]
name  = "api"
entry = ["b"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`
	se := wantSpecErr(t, src, "服务[1].name", "服务名重复", lineOfAt(src, `name  = "api"`, 2))
	if !strings.Contains(se.Msg, "首次出现在第 "+strconv.Itoa(lineOfAt(src, `name  = "api"`, 1))+" 行") {
		t.Fatalf("重复名文案该指出首次出现的位置：%q", se.Msg)
	}
}

// ── ⑥ 端口越界/重复 ───────────────────────────────────────────────────

func TestSpecRejectsBadPorts(t *testing.T) {
	head := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10

`
	cases := []struct {
		name    string
		ports   string
		field   string
		wantMsg string
		lineOf  string
		lineN   int // 0 视作 1：指向第 N 次出现
	}{
		{"inner 为 0", "[[端口]]\ninner = 0\n", "端口[0].inner", "1..65535", "inner = 0", 0},
		{"inner 超上界", "[[端口]]\ninner = 70000\n", "端口[0].inner", "1..65535", "inner = 70000", 0},
		{"inner 为负", "[[端口]]\ninner = -1\n", "端口[0].inner", "1..65535", "inner = -1", 0},
		{"inner 重复", "[[端口]]\ninner = 8000\n\n[[端口]]\ninner = 8000\n", "端口[1].inner", "重复", "inner = 8000", 2},
		{"固定 host 撞车", "[[端口]]\ninner = 8000\nhost = 15432\n\n[[端口]]\ninner = 5432\nhost = 15432\n", "端口[1].host", "宿主端口 15432", "host = 15432", 2},
		{"host 非数字", "[[端口]]\ninner = 8000\nhost = \"八十\"\n", "端口[0].host", "auto", "host = \"八十\"", 0},
		{"host 越界", "[[端口]]\ninner = 8000\nhost = 70000\n", "端口[0].host", "1..65535", "host = 70000", 0},
		{"bind 非 IP", "[[端口]]\ninner = 8000\nbind = \"localhost\"\n", "端口[0].bind", "不是合法 IP", "bind = \"localhost\"", 0},
		{"缺 inner", "[[端口]]\nhost = \"auto\"\n", "端口[0].inner", "必填", "[[端口]]", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := head + tc.ports
			n := tc.lineN
			if n == 0 {
				n = 1
			}
			wantSpecErr(t, src, tc.field, tc.wantMsg, lineOfAt(src, tc.lineOf, n))
		})
	}

	// 同一 inner 重复时，报错要指到第二次出现的那一行
	src := head + "[[端口]]\ninner = 8000\n\n[[端口]]\ninner = 8000\n"
	se := wantSpecErr(t, src, "端口[1].inner", "重复", 0)
	if want := lineOfAt(src, "inner = 8000", 2); se.Line != want {
		t.Fatalf("行号 = %d，want %d（第二次出现处）", se.Line, want)
	}
}

// ── ⑦ 缺 digest 只警告 ────────────────────────────────────────────────

func TestSpecWarnsMissingDigest(t *testing.T) {
	src := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`
	sp := mustParseSpec(t, src) // 不报错
	if sp.HasDigest() {
		t.Fatal("HasDigest() = true，契约里没有 digest")
	}
	if len(sp.Warnings) != 1 {
		t.Fatalf("Warnings = %v，want 恰好 1 条", sp.Warnings)
	}
	w := sp.Warnings[0]
	if w.Field != "茧.digest" {
		t.Fatalf("Warning.Field = %q，want 茧.digest", w.Field)
	}
	if w.Line != lineOf(src, "image") {
		t.Fatalf("Warning.Line = %d，want %d", w.Line, lineOf(src, "image"))
	}
	if !strings.Contains(w.Msg, "可复现") {
		t.Fatalf("警告该说明后果（可复现判据）：%q", w.Msg)
	}
	if !strings.Contains(w.String(), "茧.digest") {
		t.Fatalf("Warning.String() 缺字段名：%q", w.String())
	}
}

// ── ⑧ 默认值 ──────────────────────────────────────────────────────────

func TestSpecDefaults(t *testing.T) {
	src := fmt.Sprintf(`[茧]
name  = "shop"
image = "python:3.12-slim"
digest = %q

[[服务]]
name  = "api"
entry = ["a"]
readiness = { port = 8000 }

[[端口]]
inner = 8000

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`, specValidDigest)
	sp := mustParseSpec(t, src)

	if sp.Env["TZ"] != "Asia/Shanghai" || sp.Env["LANG"] != "C.UTF-8" {
		t.Fatalf("[环境] 缺失时未填固定默认：%v", sp.Env)
	}
	if sp.Build.Net != NetNone {
		t.Fatalf("构建段缺失时 Build.Net = %q，want %q", sp.Build.Net, NetNone)
	}
	if sp.Network.Mode != NetNone {
		t.Fatalf("网络段缺失时 Mode = %q，want %q", sp.Network.Mode, NetNone)
	}
	p := sp.Ports[0]
	if p.Host != HostAuto || p.Bind != DefaultBind {
		t.Fatalf("端口默认 = %+v，want host=%q bind=%q", p, HostAuto, DefaultBind)
	}
	rd := sp.Services[0].Readiness
	if rd == nil || rd.Timeout != DefaultReadinessTimeout || rd.Path != "" {
		t.Fatalf("readiness 默认 = %+v，want timeout=%s path 空", rd, DefaultReadinessTimeout)
	}

	// 显式写了 TZ 就以用户为准（固定默认不是「强制」）
	override := strings.Replace(src, "[资源]", "[环境]\nTZ = \"UTC\"\n\n[资源]", 1)
	if got := mustParseSpec(t, override).Env["TZ"]; got != "UTC" {
		t.Fatalf("显式 TZ 被默认值盖掉：%q", got)
	}
	// 空串视同未写：默认值接管（空 TZ 会让 time 包行为不可预期）
	emptyTZ := strings.Replace(src, "[资源]", "[环境]\nTZ = \"\"\n\n[资源]", 1)
	if got := mustParseSpec(t, emptyTZ).Env["TZ"]; got != "Asia/Shanghai" {
		t.Fatalf("空 TZ 未回落到默认：%q", got)
	}
}

// ── ⑨ 结构校验（防拼错静默降级） ──────────────────────────────────────

func TestSpecRejectsStructural(t *testing.T) {
	head := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`
	// 顶层表拼错：整段被忽略就等于「一个服务都没起」却解析成功
	typoSection := head + "\n[服务s]\nname = \"x\"\n"
	wantSpecErr(t, typoSection, "服务s", "未知的顶层表", lineOf(typoSection, "[服务s]"))

	// [服务] 与 [[服务]] 冲突（表 vs 数组表）也必须报错，不能静默忽略
	tableSvc := head + "\n[服务]\nname = \"x\"\n"
	se := wantSpecErr(t, tableSvc, "服务", "数组", lineOfAt(tableSvc, "[服务]", 2))
	if !strings.Contains(se.Msg, "数组") {
		t.Fatalf("冲突文案该说明它是数组表：%q", se.Msg)
	}

	// 字段拼错
	typoField := strings.Replace(head, "image = \"python:3.12-slim\"", "img = \"python:3.12-slim\"", 1)
	wantSpecErr(t, typoField, "茧.img", "未知字段", lineOf(typoField, "img ="))

	// 缺 [茧]
	noCocoon := `[[服务]]
name  = "api"
entry = ["a"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`
	wantSpecErr(t, noCocoon, "茧", "缺少 [茧]", 0)

	// 缺 [[服务]]
	noSvc := `[茧]
name  = "shop"
image = "python:3.12-slim"

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`
	wantSpecErr(t, noSvc, "服务", "至少需要一个", 0)

	// 缺 [资源]（不给无限默认）
	noRes := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]
`
	wantSpecErr(t, noRes, "资源", "缺少 [资源]", 0)

	// 资源缺项 / 非正
	resMissing := strings.Replace(head, "disk_gb = 10\n", "", 1)
	wantSpecErr(t, resMissing, "资源.disk_gb", "必填", 0)
	resNeg := strings.Replace(head, "cpu = 1", "cpu = 0", 1)
	wantSpecErr(t, resNeg, "资源.cpu", "> 0", lineOf(resNeg, "cpu = 0"))

	// entry 为空数组 / 空串元素
	emptyEntry := strings.Replace(head, `entry = ["a"]`, `entry = []`, 1)
	wantSpecErr(t, emptyEntry, "服务[0].entry", "必填", 0)
	blankArg := strings.Replace(head, `entry = ["a"]`, `entry = ["a", "  "]`, 1)
	wantSpecErr(t, blankArg, "服务[0].entry[1]", "空串", 0)

	// arch 只认 arm64/amd64
	badArch := strings.Replace(head, "image", "arch  = \"x86\"\nimage", 1)
	wantSpecErr(t, badArch, "茧.arch", "arm64", lineOf(badArch, "arch"))

	// workdir 必须绝对路径
	relWorkdir := strings.Replace(head, "image", "workdir = \"app\"\nimage", 1)
	wantSpecErr(t, relWorkdir, "茧.workdir", "绝对路径", lineOf(relWorkdir, "workdir"))

	// image 里夹 @digest（会把可复现锚点藏进标签位）
	atImage := strings.Replace(head, `image = "python:3.12-slim"`, `image = "python@sha256:abc"`, 1)
	wantSpecErr(t, atImage, "茧.image", "digest", lineOf(atImage, "@sha256"))

	// readiness 端口越界 / timeout 非法（行内表；数组表上的 [服务.readiness] 子表不属本子集）
	badReady := strings.Replace(head, `entry = ["a"]`, "entry = [\"a\"]\nreadiness = { port = 0 }", 1)
	wantSpecErr(t, badReady, "服务[0].readiness.port", "1..65535", lineOf(badReady, "readiness"))
	badTimeout := strings.Replace(head, `entry = ["a"]`, "entry = [\"a\"]\nreadiness = { port = 80, timeout = \"30\" }", 1)
	wantSpecErr(t, badTimeout, "服务[0].readiness.timeout", "时长", lineOf(badTimeout, "readiness"))
}

// ── ⑩ TOML 子集能力 ──────────────────────────────────────────────────

func TestSpecTOMLSubsetFeatures(t *testing.T) {
	src := `# 头部注释（应被吃掉）
[茧]  # 行尾注释
name    = 'shop'                 # 字面字符串
image   = "python:3.12-slim"
digest  = "` + specValidDigest + `"
workdir = "/app"

[[服务]]
name  = "api"
entry = [
  "uvicorn",        # 跨行数组里也能写注释
  "app:api",
]

[资源]
cpu     = 1.5
mem_gb  = 2
disk_gb = 10

[环境]
NOTE = "a#b"                     # 字符串里的 '#' 不是注释
WIN  = 'C:\path\no-escape'       # 字面串不处理反斜杠
ESC  = "tab\there \"q\" \\ end"

[环境.vars]
APP_ENV = "prod"
`
	sp := mustParseSpec(t, src)
	if sp.Cocoon.Name != "shop" || sp.Cocoon.Workdir != "/app" {
		t.Fatalf("[茧] = %+v", sp.Cocoon)
	}
	if !slices.Equal(sp.Services[0].Entry, []string{"uvicorn", "app:api"}) {
		t.Fatalf("跨行数组 = %v", sp.Services[0].Entry)
	}
	if sp.Resources.CPU != 1.5 {
		t.Fatalf("cpu = %v，want 1.5", sp.Resources.CPU)
	}
	if sp.Env["NOTE"] != "a#b" {
		t.Fatalf("Env[NOTE] = %q（'#' 被当注释切了？）", sp.Env["NOTE"])
	}
	if want := `C:\path\no-escape`; sp.Env["WIN"] != want {
		t.Fatalf("Env[WIN] = %q，want %q", sp.Env["WIN"], want)
	}
	if want := "tab\there \"q\" \\ end"; sp.Env["ESC"] != want {
		t.Fatalf("Env[ESC] = %q，want %q", sp.Env["ESC"], want)
	}
	if sp.Env["APP_ENV"] != "prod" {
		t.Fatalf("嵌套表 [环境.vars] 未展开：%v", sp.Env)
	}

	// 语法错也要带行号
	_, err := ParseSpec([]byte("[茧\nname = \"x\"\n"))
	if err == nil {
		t.Fatal("未闭合表头被接受了")
	}
	se, ok := AsSpecError(err)
	if !ok || se.Line != 1 {
		t.Fatalf("语法错应带行号 1：%v", err)
	}
	// 未加引号的字符串值
	_, err = ParseSpec([]byte("[茧]\nname = shop\n"))
	if se, ok := AsSpecError(err); !ok || se.Line != 2 || !strings.Contains(se.Msg, "值") {
		t.Fatalf("未加引号的值应报错并指向第 2 行：%v", err)
	}
	// 多行字符串：本子集明说不支持，必须报错而不是当成空串
	_, err = ParseSpec([]byte("[茧]\nname = \"\"\"x\"\"\"\n"))
	if se, ok := AsSpecError(err); !ok || se.Line != 2 {
		t.Fatalf("多行字符串应明确报错：%v", err)
	}
}

// ── ⑪ 网络策略 ────────────────────────────────────────────────────────

func TestSpecRejectsNetworkPolicy(t *testing.T) {
	head := `[茧]
name  = "shop"
image = "python:3.12-slim"

[[服务]]
name  = "api"
entry = ["a"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10

`
	// v2 §D：运行期强制 none ⇒ 契约里写 allowlist 就拒（不许承诺不存在的通路）
	rtAllow := head + "[网络]\nmode = \"allowlist\"\nallow = [\"pypi.org\"]\n"
	wantSpecErr(t, rtAllow, "网络.mode", "运行期网络只能是", lineOf(rtAllow, "mode"))

	// 构建期：net=none 却写 allow ⇒ 拒（写了也不生效，等于自欺）
	buildNoneAllow := head + "[构建]\nnet = \"none\"\nallow = [\"pypi.org\"]\n"
	wantSpecErr(t, buildNoneAllow, "构建.allow", "不允许写 allow", lineOf(buildNoneAllow, "allow = "))
	// 构建期：allowlist 却给空 allow ⇒ 拒（等于断网，意图应写 none）
	buildEmptyAllow := head + "[构建]\nnet = \"allowlist\"\nallow = []\n"
	wantSpecErr(t, buildEmptyAllow, "构建.allow", "空 allow", lineOf(buildEmptyAllow, "allow = "))
	// allow 里带协议/端口/路径 ⇒ 拒
	for _, bad := range []string{`"https://pypi.org"`, `"pypi.org:443"`, `"pypi.org/simple"`} {
		src := head + "[构建]\nnet = \"allowlist\"\nallow = [" + bad + "]\n"
		wantSpecErr(t, src, "构建.allow[0]", "只写主机名", lineOf(src, "allow = "))
	}
	// 构建期 net 枚举
	badNet := head + "[构建]\nnet = \"open\"\n"
	wantSpecErr(t, badNet, "构建.net", "allowlist", lineOf(badNet, "net"))
}

// ── ⑫ 错误文案格式 ────────────────────────────────────────────────────

func TestSpecErrorFormat(t *testing.T) {
	src := "[茧]\nname = \"x\"\nimage = \"y\"\n\n[资源]\n"
	_, err := ParseSpecFile("etc/茧/shop/cocoon.toml", []byte(src))
	if err == nil {
		t.Fatal("缺 [[服务]] 却解析成功")
	}
	if !strings.HasPrefix(err.Error(), "etc/茧/shop/cocoon.toml:") {
		t.Fatalf("文案未用调用方给的来源名：%q", err.Error())
	}
	se, ok := AsSpecError(err)
	if !ok || se.File != "etc/茧/shop/cocoon.toml" {
		t.Fatalf("SpecError.File = %q", se.File)
	}
	// Error() 三段俱全
	if !strings.Contains(err.Error(), ": 服务: ") {
		t.Fatalf("文案缺字段名：%q", err.Error())
	}
	// 零值 SpecError 也不许 panic
	if got := (&SpecError{Msg: "x"}).Error(); !strings.Contains(got, "x") {
		t.Fatalf("零值 SpecError.Error() = %q", got)
	}
}

// ── ⑬ 读盘 ────────────────────────────────────────────────────────────

func TestSpecLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SpecFileName)
	if err := os.WriteFile(path, []byte(specFull), 0o644); err != nil {
		t.Fatal(err)
	}
	sp, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec 失败：%v", err)
	}
	if sp.Cocoon.Name != "shop" || sp.File != path {
		t.Fatalf("LoadSpec 结果 = %+v", sp.Cocoon)
	}
	// 缺文件是「没这个文件」，不是「契约内容错」：调用方要能 errors.Is 判出来
	_, err = LoadSpec(filepath.Join(dir, "nope.toml"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("缺文件错误 = %v，want fs.ErrNotExist", err)
	}
	if _, ok := AsSpecError(err); ok {
		t.Fatal("缺文件不该被包装成 *SpecError")
	}
}

// ── ⑭ 并发解析（-race 下验无共享可变状态） ────────────────────────────
func TestSpecConcurrentParse(t *testing.T) {
	const workers, rounds = 8, 64
	var wg sync.WaitGroup
	errs := make([]error, workers)
	digests := make([]string, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				sp, err := ParseSpec([]byte(specFull))
				if err != nil {
					errs[w] = err
					return
				}
				// 返回值不能被下一个 goroutine 的解析污染
				digests[w] = sp.Cocoon.Digest + "|" + strings.Join(sp.Order, ",")
			}
		}(w)
	}
	wg.Wait()
	for w, err := range errs {
		if err != nil {
			t.Fatalf("worker %d 解析失败：%v", w, err)
		}
		if digests[w] != specValidDigest+"|db,api" {
			t.Fatalf("worker %d 结果被污染：%q", w, digests[w])
		}
	}
}

// ── ⑮ 垃圾输入不许 panic ──────────────────────────────────────────────
//
// 解析器是手写的字节状态机，最容易出的不是「判错」而是「越界 panic」——一个 panic 会让
// `cocoon open` 在拿到的是一份坏文件时直接崩，而不是报一行可读的错。这里用固定种子的
// 伪随机字节流兜住这一类（确定性，不会偶发飘红）。
func TestSpecParserNeverPanicsOnGarbage(t *testing.T) {
	explicit := []string{
		"", " ", "\n\n\n", "# 只有注释\n", "\xef\xbb\xbf", "[]", "[[]]", "[[服务]", "[茧]]",
		"[茧]\n名 = \"x\"", "a = 1", "key", "= 1", "a =", "a = \"x", "a = [1,", "a = {b = }",
		"a = {b = 1", "[t]\na = 1\na = 2", "[t]\n[t]", "[[t]]\n[[t]]\nx = 1",
		"a = 0x", "a = 1e", "a = tru", "a = \"\\q\"", "a = \"\\u12\"", "a = \"\\U0011\"",
		"a = '''x'''", "a = \"\"\"x\"\"\"", "[a.b]\nc = 1", "a.b.c = 1", "a = 0b12", "a = 1_",
		"[茧]\r\nname = \"x\"\r\n", "\x00\x01\x02", "a = -", "a = +", "a = .5e",
	}
	for _, src := range explicit {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on %q: %v", src, r)
				}
			}()
			_, _ = ParseSpec([]byte(src))
		}()
	}

	alphabet := []byte("[]{}.,=#\"' \t\n\r\\abc茧服务0123_-+:eE\u00e9\x00")
	rng := rand.New(rand.NewSource(20261020))
	for i := 0; i < 2000; i++ {
		n := rng.Intn(64)
		buf := make([]byte, n)
		for j := range buf {
			buf[j] = alphabet[rng.Intn(len(alphabet))]
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on 随机输入 %q: %v", buf, r)
				}
			}()
			_, _ = ParseSpec(buf)
		}()
	}
}

// ── ⑰ [茧].rootfs / overlay_upper（换真 rootfs 的契约面） ────────────────
//
// 判据四条：
//
//	① 不写这两个键 ⇒ 两个字段都是空（既有档：chroot 由后端给，不是「契约说了换根」）。
//	② 绝对路径原样（只 Clean）；相对路径按**契约文件所在目录**解析成绝对路径。
//	③ 空串 ≠ 不写：空串是「换到空气里」，必须在解析期就拒。
//	④ 只给 overlay_upper、不给 rootfs ⇒ 拒（上层没有下层不是配置，是笔误）。
func TestSpecCocoonRootfsAndOverlay(t *testing.T) {
	tmpl := func(extra string) string {
		return `[茧]
name  = "shop"
image = "debian:bookworm"
` + extra + `
[[服务]]
name  = "api"
entry = ["/bin/sh"]

[资源]
cpu = 1
mem_gb = 2
disk_gb = 10
`
	}

	// ① 既有档：两键都不写。
	plain := mustParseSpec(t, tmpl(""))
	if plain.Cocoon.Rootfs != "" || plain.Cocoon.OverlayUpper != "" {
		t.Fatalf("没写 rootfs/overlay_upper 时两个字段都该是空，得到 %q / %q",
			plain.Cocoon.Rootfs, plain.Cocoon.OverlayUpper)
	}

	// ② 绝对路径：原样（带 ../ 的话要 Clean，不是原串照抄）。
	withAbs := tmpl("rootfs        = \"/srv/images/debian/..//debian\"\noverlay_upper = \"/srv/cocoons/shop/upper\"")
	sp := mustParseSpec(t, withAbs)
	if sp.Cocoon.Rootfs != "/srv/images/debian" {
		t.Fatalf("绝对 rootfs 应当 Clean 成 /srv/images/debian，得到 %q", sp.Cocoon.Rootfs)
	}
	if sp.Cocoon.OverlayUpper != "/srv/cocoons/shop/upper" {
		t.Fatalf("绝对 overlay_upper 不符：%q", sp.Cocoon.OverlayUpper)
	}

	// ②b 只写 rootfs：overlay_upper 留空（缺省由后端算成 <茧目录>/upper，不在契约层猜）。
	onlyRoot := mustParseSpec(t, tmpl("rootfs = \"/srv/images/debian\""))
	if onlyRoot.Cocoon.OverlayUpper != "" {
		t.Fatalf("没写 overlay_upper 时应当留空（缺省由后端定），得到 %q", onlyRoot.Cocoon.OverlayUpper)
	}

	// ②c 相对路径：按**契约文件所在目录**解析（用 LoadSpec 才拿得到真实路径）。
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "images", "debian"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, SpecFileName)
	if err := os.WriteFile(path, []byte(tmpl("rootfs = \"images/debian\"\noverlay_upper = \"upper\"")), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec 失败：%v", err)
	}
	if want := filepath.Join(dir, "images", "debian"); loaded.Cocoon.Rootfs != want {
		t.Fatalf("相对 rootfs 应按契约目录解析成 %q，得到 %q", want, loaded.Cocoon.Rootfs)
	}
	if want := filepath.Join(dir, "upper"); loaded.Cocoon.OverlayUpper != want {
		t.Fatalf("相对 overlay_upper 应按契约目录解析成 %q，得到 %q", want, loaded.Cocoon.OverlayUpper)
	}

	// ③ 空串必须拒（与「不写」分开：空串会被读成「换到空根」）。
	wantSpecErr(t, tmpl("rootfs = \"\""), "茧.rootfs", "不得为空串", lineOf(tmpl("rootfs = \"\""), "rootfs"))
	blankUpperSrc := tmpl("rootfs = \"/srv/i\"\noverlay_upper = \"  \"")
	wantSpecErr(t, blankUpperSrc, "茧.overlay_upper", "不得为空串", lineOf(blankUpperSrc, "overlay_upper"))

	// ④ overlay_upper 单独出现 ⇒ 拒。
	wantSpecErr(t, tmpl("overlay_upper = \"/srv/u\""), "茧.overlay_upper", "要先给 .rootfs", 4)

	// ④b rootfs="/" 时不给 overlay（下层是宿主根，叠出来只是把宿主根复制一遍）。
	hostRootOverlay := tmpl("rootfs = \"/\"\noverlay_upper = \"/srv/u\"")
	wantSpecErr(t, hostRootOverlay, "茧.overlay_upper", "不给 overlay", lineOf(hostRootOverlay, "overlay_upper"))

	// ⑤ rootfs="/" 本身不阻断（用户的明确选择），但要有一条警告说明「等于不换根」。
	hostRoot := mustParseSpec(t, tmpl("rootfs = \"/\""))
	var found bool
	for _, w := range hostRoot.Warnings {
		if w.Field == "茧.rootfs" {
			found = true
			if !strings.Contains(w.Msg, "等于不换根") {
				t.Fatalf("rootfs=/ 的警告该说清「等于不换根」：%q", w.Msg)
			}
		}
	}
	if !found {
		t.Fatalf("rootfs=/ 应当留一条警告（不许静默当成「已换根」），Warnings=%v", hostRoot.Warnings)
	}

	// ⑥ 拼错键名照旧报错，且允许清单里现在含这两个新键。
	se := wantSpecErr(t, tmpl("rootfss = \"/x\""), "茧.rootfss", "未知字段", 4)
	if !strings.Contains(se.Msg, "rootfs") || !strings.Contains(se.Msg, "overlay_upper") {
		t.Fatalf("未知字段的允许清单该列出 rootfs / overlay_upper：%q", se.Msg)
	}
}
