// client_tool_names_test.go — GAP-20260926-176（P1 · 换件面）的「漏名即红」机检。
//
// 病征（在册账述逐字）：「控制层 rules.yaml 的客户端工具名面没有『漏名即红』机检：
// failclosed_test.go 只扫 core/internal/agent/*.go 的 gate.Check("X")（CA 面）⇒ Hermes
// 每新增一只工具就静默漏名，本地卵（经网关的请求）整发 403 —— 本次实测漏 video_generate
// + x_search 两条」。
//
// 为什么既有棘轮管不住它：core/internal/control/failclosed_test.go 的两条扫法
// （caToolNames 只看 agent 包、scanGateCheckCallSites 扩到全仓）问的都是同一件事 ——
// 「有 gate.Check 调用点的名字在不在册」。客户端名面是**网关转发请求里模型要调的名字**，
// 那些名字在仓内**一个 gate.Check 调用点都没有** ⇒ 两条扫法都看不见它们（扫不扫得到，
// 取决于仓内有没有那个名字的调用点，不是取决于 Hermes 有没有那只工具）。漏名因此静默。
//
// 本文件补的两条判据：
//
//	① 权威面（能真判「与实际新工具一致」的那一半）：直读 Hermes 的工具注册面
//	   （~/.hermes/hermes-agent/tools/*.py 的两种注册形态 name="…" 与 "name": "…"），
//	   取并集后逐个要求在 core/internal/control/rules.yaml 在册 ⇒ 少一个就红。
//	   注册面目录不在的机器上**降级**（打印声明后跳过），不假装判过。
//	② 仓内面（清单 ⟷ 表 一致性棘轮）：core/internal/agent/testdata/client_tool_names.txt
//	   与 rules.yaml 的客户端命名空间**双向相等**（互为子集）⇒ 任一侧增删不同步即红。
//
// 既有断言一条不改（本文件只新增；control 包的 failclosed_test.go 与 rules.yaml 均不碰）。
package agent

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// moduleRoot 从当前包往上找模块根（认 go.mod 的那一层 = core/）。
func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("找不到模块根（从 %s 往上，认 go.mod）", wd)
	return ""
}

// rulesYamlPath —— 控制层规则表（模块根下的 internal/control/rules.yaml）。
func rulesYamlPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "internal", "control", "rules.yaml")
}

var reRuleName = regexp.MustCompile(`-\s*name:\s*"([A-Za-z0-9_*]+)"`)
var reGateCheck = regexp.MustCompile(`gate\.Check\("([A-Za-z0-9_]+)"`)

// rulesDeclaredNames —— rules.yaml 里所有 `- name: "X"` 的名字（去重排序）。
// 口径：字面在 = 在册（默认档 block 之下，没有被逐条写出来的名字才会落默认档）。
func rulesDeclaredNames(t *testing.T, rulesPath string) []string {
	t.Helper()
	b, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("读控制层规则表 %s：%v", rulesPath, err)
	}
	seen := map[string]bool{}
	for _, m := range reRuleName.FindAllStringSubmatch(string(b), -1) {
		seen[m[1]] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// caCheckNames —— core 模块里 `gate.Check("X")` 的真名集合（CA 面）。
// 与 control 包 failclosed_test.go 的 scanGateCheckCallSites 同一套正则与排除口径
// （按目录名窄排除、只看非测试 .go），保证两侧对「CA 面」的认定不会各说各话。
func caCheckNames(t *testing.T, moduleRoot string) map[string]bool {
	t.Helper()
	skip := map[string]bool{"target": true, "node_modules": true, "dist": true, "bin": true,
		"vendor": true, "data": true, ".git": true, ".venv": true, "venv": true, ".build": true}
	out := map[string]bool{}
	err := filepath.Walk(moduleRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skip[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		for _, m := range reGateCheck.FindAllStringSubmatch(string(b), -1) {
			out[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫 CA 面 gate.Check 调用点：%v", err)
	}
	return out
}

// clientNamespace —— rules.yaml 的「客户端命名空间」逐条：
// 全部 - name: 减去 ① CA 面（gate.Check 真名）减去 ② 后缀通配 mcp_*。
// 为什么这样切：客户端名面的名字在仓内没有调用点，只能靠这个减法把两套命名空间分开
// （rules.yaml 自己也把这两段分块注释开，但注释不是机可读的真源，减法才是）。
func clientNamespace(t *testing.T, rules []string, ca map[string]bool) []string {
	t.Helper()
	var out []string
	for _, n := range rules {
		if n == "mcp_*" || ca[n] {
			continue
		}
		out = append(out, n)
	}
	return out
}

// loadManifest —— 读仓内清单件（一行一名，# 注释与空行忽略）。
func loadManifest(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读客户端工具名清单 %s：%v（它是本检查的仓内真源，不能缺）", path, err)
	}
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		out = append(out, ln)
	}
	sort.Strings(out)
	return out
}

func setOf(ss []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func diffNames(a []string, b map[string]bool) []string {
	var out []string
	for _, s := range a {
		if !b[s] {
			out = append(out, s)
		}
	}
	return out
}

// TestClientToolNames_ManifestMatchesRules —— 判据②：仓内清单件 ⟷ rules.yaml 客户端命名空间
// **双向相等**。任一侧增删不同步即红（删掉清单里一个已在表里的名字 ⇒ 本测试当场红）。
func TestClientToolNames_ManifestMatchesRules(t *testing.T) {
	rulesPath := rulesYamlPath(t)
	root := moduleRoot(t)
	rules := rulesDeclaredNames(t, rulesPath)
	client := clientNamespace(t, rules, caCheckNames(t, root))
	if len(client) == 0 {
		t.Fatalf("空转：从 %s 切不出任何客户端工具名（判据不可判）", rulesPath)
	}
	manifest := loadManifest(t, filepath.Join(root, "internal", "agent", "testdata", "client_tool_names.txt"))
	if len(manifest) == 0 {
		t.Fatalf("空转：清单件 0 条（判据不可判）")
	}
	inRules := setOf(rules)
	inManifest := setOf(manifest)
	// 方向一：表里有、清单里没有 ⇒ 清单漏登（表增了名字没同步清单）
	for _, n := range diffNames(client, inManifest) {
		t.Errorf("工具名 %q 在 rules.yaml 的客户端命名空间里、但不在仓内清单件里 ⇒ "+
			"把 %q 补进 core/internal/agent/testdata/client_tool_names.txt（一行一个名字），"+
			"否则下一个人拿这张清单当全集就会漏判。", n, n)
	}
	// 方向二：清单里有、表里没有 ⇒ 清单登记了却没有在册（表被删条目 / 清单抄多了）
	for _, n := range diffNames(manifest, inRules) {
		t.Errorf("工具名 %q 在仓内清单件里、但 rules.yaml **没有这一条** ⇒ 默认档 block 之下它会被"+
			"静默拦掉（子代理一发 403）。要么去 core/internal/control/rules.yaml 的「客户端命名空间」段"+
			"补一条 - name: %q（读类给 allow · 外发/自动化/动界面类给 block），要么从清单里摘掉 %q。",
			n, n, n)
	}
	t.Logf("客户端工具名面 %d 个：清单 ⟷ rules.yaml 双向一致（CA 面另有 %d 个 gate.Check 真名，不在本判据）",
		len(client), len(caCheckNames(t, root)))
}

// TestClientToolNames_LiveHermesToolsAreRegistered —— 判据①：**权威面**。
// 直读 Hermes 的工具注册面（仓外：~/.hermes/hermes-agent/tools/*.py），取两种注册形态的并集，
// 逐个要求在 rules.yaml 在册 ⇒ 少一个就红（这正是本次漏 video_generate / x_search 的形态）。
//
// 界限声明：注册面目录**不在**的机器上本判据**降级**（打印一行声明后跳过）—— 它判不了
// 「Hermes 有没有新工具」，只能靠「清单 ⟷ 表」那条棘轮兜底。别把降级当绿。
func TestClientToolNames_LiveHermesToolsAreRegistered(t *testing.T) {
	rulesPath := rulesYamlPath(t)
	rules := rulesDeclaredNames(t, rulesPath)
	inRules := setOf(rules)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("取家目录：%v", err)
	}
	dir := filepath.Join(home, ".hermes", "hermes-agent", "tools")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Logf("降级（非绿）：Hermes 工具注册面 %s 读不到（%v）⇒ 本判据跳过；"+
			"本次只剩「仓内清单 ⟷ rules.yaml」一致性棘轮，判不了『与实际新工具一致』。", dir, err)
		return
	}
	forms := []*regexp.Regexp{
		regexp.MustCompile(`name="([a-z_]{3,40})"`),                 // 形态一 name="…"
		regexp.MustCompile(`"name"\s*:\s*"([a-z][a-z0-9_]{2,40})"`), // 形态二 "name": "…"
	}
	declared := map[string]bool{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		b, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			continue
		}
		for _, re := range forms {
			for _, m := range re.FindAllStringSubmatch(string(b), -1) {
				declared[m[1]] = true
			}
		}
	}
	if len(declared) == 0 {
		t.Fatalf("空转：注册面 %s 里一个工具名都扫不到（判据不可判 · 别当绿）", dir)
	}
	var missing []string
	for n := range declared {
		if !inRules[n] {
			missing = append(missing, n)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("Hermes 已宣告但 rules.yaml **未登记**的工具名 %d 个：%v ⇒ "+
			"它们会让经网关的整发请求被默认档 block 打死（子代理一发 403）。"+
			"去 core/internal/control/rules.yaml 的「客户端命名空间」段逐条补 - name: <名>"+
			"（读类给 allow · 外发/自动化/动界面类给 block），并同步 core/internal/agent/"+
			"testdata/client_tool_names.txt。", len(missing), missing)
	}
	t.Logf("Hermes 已宣告工具名 %d 个 · rules.yaml 在册 %d 个 · 缺名 %d 个（注册面 %s）",
		len(declared), len(rules), len(missing), dir)
}
