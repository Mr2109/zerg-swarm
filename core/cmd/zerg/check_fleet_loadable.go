// check_fleet_loadable.go —— 判据件 `scripts/gates/check-fleet-loadable.py` 的 **Go 真搬**
// （M2 · 入口 `checkFleetLoadableGateMain` · 承接缺口 GAP-20260925-12）。
//
// 为什么有这件：
//
//	缺口「名册没有『件能否加载』的机读判据」的**判据面**在 py 半落成后，按 v2.5.14 的 M1/M2
//	定案（M1 = 转发壳 `zerg gate check <判据名>`；M2 = 按价值真搬 Go，落点 `core/cmd/zerg/`）
//	再落一枚 Go 真搬件 ⇒ 两个入口（`zerg gate check` 走 Go inproc · 步骤表那一步走 py）**同批**。
//
//	验收判据 = 对同一 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// ★ 本件**只落码、不接线**（✗ 不改 main.go 命令树 · ✗ 不删 py 原件 · 入口的调用点登记见派单
//
//	回执的「登记清单」；`gate_inproc_table.go` 里那一行是本件与转发壳的唯一接线点）。
//
// ═══════════════════ 出厂声明行 (a) 默认根定位差异 ═══════════════════
//
//	py 从 `__file__` 上溯三级得 REPO_ROOT（= 仓根）；Go 无 `__file__` 语境 ⇒ 本件默认根 =
//	cwd（`os.Getwd()`），同 `check-archive-mount.go` / `check-affected-map.go` 的既有口径。
//	⇒ 「脚本位置 ≠ cwd」时 `--fleet` 的默认推导面不同（✗ 不声称逐字对齐）。对拍一律显式传
//	`--fleet`，或把 cwd 设在仓根。
//
// ═══════════════════ 出厂声明行 (b) 自指检查 ═══════════════════
//
//	真判：本件是否落在它自己扫描的面内？本门扫描面 = **一件具体文件**（`gateway/fleet.yaml`，
//	由 `--fleet` 点名）；**零目录遍历 · 零 glob · 零 git 面** ⇒ 本件**不在自己的扫描面内**。
//	（依据：py 全件零 walk/listdir/glob 调用，读面逐条点名为具体路径。）
//	第四形态（落在他门扫描面内）：本件正文以**描述式**复述被扫形态（字段名、架构名），
//	✗ 不作独立可解析的私有绝对路径字面量；夹具串是**夹具输入**、不是扫描面命中。
//
// 退码（与 py 逐字同）：0 全对 · 1 有可由声明面判定的缺陷 · 2 不给结论（件不在 / 解析不出 /
// 空域 / 自检不过）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// cflBackends 是后端闭集（判据 C；与 py `BACKENDS` 逐字同源）。
var cflBackends = []string{"llama-server", "ds4-server"}

// cflRequiredFields 是候选必备字段（判据 B；与 py `REQUIRED_FIELDS` 逐字同源）。
var cflRequiredFields = []string{"host", "backend", "file"}

// cflMainlineUnsupportedArchs 是主线 llama.cpp 不支持的架构（判据 D 的参照真源；与 py
// `MAINLINE_UNSUPPORTED_ARCHS` 同源，真源同 core/internal/config/validator.go）。
var cflMainlineUnsupportedArchs = []string{"example-moe-36b"}

type cflFinding struct {
	face string
	msg  string
}

func cflInSet(v string, set []string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// cflScalar 把 yaml 节点折成字符串（标量原样；序列/映射给 ""，本门只对 cmd 看非空）。
func cflScalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// cflCandidate 一条候选的扁平面（标量字段 + cmd 非空标记）。
type cflCandidate struct {
	fields map[string]string
	hasCmd bool
}

func (c cflCandidate) get(k string) string { return c.fields[k] }

func (c cflCandidate) cmdPresent() bool { return c.hasCmd }

func (c cflCandidate) arch() string {
	if v := strings.TrimSpace(c.get("architecture")); v != "" {
		return v
	}
	return strings.TrimSpace(c.get("arch"))
}

func (c cflCandidate) implFlag() bool {
	switch strings.ToLower(strings.TrimSpace(c.get("engine_impl_required"))) {
	case "true", "yes", "1":
		return true
	}
	return false
}

// cflNeedsEngineImpl 与 py `needs_engine_impl` 逐字同源（三触发任一；非 llama 家族 backend 不重复要求）。
func cflNeedsEngineImpl(c cflCandidate) bool {
	if c.implFlag() {
		return true
	}
	if cflInSet(c.arch(), cflMainlineUnsupportedArchs) {
		return true
	}
	if strings.TrimSpace(c.get("backend")) != "llama-server" {
		return false
	}
	return c.cmdPresent()
}

// cflParseCandidate 把一条候选映射节点折成扁平面（严格：只取标量 + cmd 非空）。
func cflParseCandidate(mapNode *yaml.Node) cflCandidate {
	c := cflCandidate{fields: map[string]string{}}
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return c
	}
	for i := 0; i+1 < len(mapNode.Content); i += 2 {
		k := mapNode.Content[i].Value
		vn := mapNode.Content[i+1]
		switch vn.Kind {
		case yaml.ScalarNode:
			c.fields[k] = vn.Value
		case yaml.SequenceNode:
			if k == "cmd" {
				c.hasCmd = len(vn.Content) > 0
			}
		}
	}
	return c
}

// cflParseModels 从 `models:` 映射节点取出有序的 {模型名: [候选…]}（保文档序）。
func cflParseModels(modelsNode *yaml.Node) ([]string, map[string][]cflCandidate) {
	var order []string
	out := map[string][]cflCandidate{}
	if modelsNode == nil || modelsNode.Kind != yaml.MappingNode {
		return order, out
	}
	for i := 0; i+1 < len(modelsNode.Content); i += 2 {
		name := modelsNode.Content[i].Value
		vn := modelsNode.Content[i+1]
		order = append(order, name)
		switch vn.Kind {
		case yaml.SequenceNode:
			for _, el := range vn.Content {
				out[name] = append(out[name], cflParseCandidate(el))
			}
		case yaml.MappingNode:
			out[name] = append(out[name], cflParseCandidate(vn))
		}
	}
	return order, out
}

// cflTopMapping 找顶层键 key 的值节点。
func cflTopMapping(doc *yaml.Node, key string) *yaml.Node {
	if doc == nil {
		return nil
	}
	root := doc
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1]
		}
	}
	return nil
}

// cflCheck 是同口径核心：→ (rc, findings)。
func cflCheck(fleetPath string) (int, []cflFinding) {
	if st, err := os.Stat(fleetPath); err != nil || st.IsDir() {
		return 2, []cflFinding{{"A", fmt.Sprintf("清单件不在：%s ⇒ 判不了（不给结论）", fleetPath)}}
	}
	data, err := os.ReadFile(fleetPath)
	if err != nil {
		return 2, []cflFinding{{"A", fmt.Sprintf("读不到清单件：%s（%v）", fleetPath, err)}}
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return 2, []cflFinding{{"A", fmt.Sprintf("解析不出 `models:` 段 ⇒ 判不了（不给结论）")}}
	}
	modelsNode := cflTopMapping(&doc, "models")
	if modelsNode == nil {
		return 2, []cflFinding{{"A", "解析不出 `models:` 段 ⇒ 判不了（不给结论）"}}
	}
	order, models := cflParseModels(modelsNode)
	// 剥离空组（py 同规：value 为空不算）
	nonEmpty := 0
	for _, n := range order {
		if len(models[n]) > 0 {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		return 2, []cflFinding{{"A", "`models:` 段为空（零执行 = 假绿）⇒ 判不了（不给结论）"}}
	}

	fleetKeys := map[string]bool{}
	if fk := cflTopMapping(&doc, "fleet"); fk != nil && fk.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(fk.Content); i += 2 {
			fleetKeys[fk.Content[i].Value] = true
		}
	}
	aliases := [][2]string{}
	if an := cflTopMapping(&doc, "aliases"); an != nil && an.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(an.Content); i += 2 {
			aliases = append(aliases, [2]string{an.Content[i].Value, an.Content[i+1].Value})
		}
	}

	var findings []cflFinding
	for _, name := range order {
		for idx, c := range models[name] {
			where := fmt.Sprintf("%s[#%d]", name, idx)
			for _, f := range cflRequiredFields {
				if strings.TrimSpace(c.get(f)) == "" {
					findings = append(findings, cflFinding{"B", fmt.Sprintf("%s：缺必备字段 `%s`", where, f)})
				}
			}
			mg := strings.TrimSpace(c.get("mem_gb"))
			if mg == "" {
				findings = append(findings, cflFinding{"B", fmt.Sprintf("%s：缺 `mem_gb`（无法判内存可加载性）", where)})
			} else if v, err := strconv.ParseFloat(mg, 64); err == nil && v <= 0 {
				findings = append(findings, cflFinding{"B", fmt.Sprintf("%s：`mem_gb`=%s 非正数", where, c.get("mem_gb"))})
			}
			host := strings.TrimSpace(c.get("host"))
			if host != "" && len(fleetKeys) > 0 && !fleetKeys[host] {
				findings = append(findings, cflFinding{"C", fmt.Sprintf("%s：host `%s` 不在 `fleet:` 机器键集 %s 里（引了不存在的机器）", where, host, cflSortedKeys(fleetKeys))})
			}
			if be := strings.TrimSpace(c.get("backend")); be != "" && !cflInSet(be, cflBackends) {
				findings = append(findings, cflFinding{"C", fmt.Sprintf("%s：backend `%s` 不在闭集 %s 里", where, be, cflList(cflBackends))})
			}
			if fp := strings.TrimSpace(c.get("file")); fp != "" && !strings.HasSuffix(fp, ".gguf") {
				findings = append(findings, cflFinding{"C", fmt.Sprintf("%s：`file` 不以 `.gguf` 结尾（不是可加载权重）：%s", where, fp)})
			}
			if mm := strings.TrimSpace(c.get("mmproj")); mm != "" && !strings.HasSuffix(mm, ".gguf") {
				findings = append(findings, cflFinding{"C", fmt.Sprintf("%s：`mmproj` 不以 `.gguf` 结尾：%s", where, mm)})
			}
		}
	}
	for _, name := range order {
		cands := models[name]
		if len(cands) == 0 {
			continue
		}
		any, all := false, true
		for _, c := range cands {
			if cflNeedsEngineImpl(c) {
				any = true
			} else {
				all = false
			}
		}
		if any && !all {
			for _, c := range cands {
				if !cflNeedsEngineImpl(c) {
					host := strings.TrimSpace(c.get("host"))
					if host == "" {
						host = "?"
					}
					findings = append(findings, cflFinding{"D", fmt.Sprintf("%s 在机器 `%s` 上：同组别的候选已声明非主线引擎实现（cmd:/engine_impl_required），本条没有 ⇒ 会**静默退回主线** llama-server（该机不可加载）", name, host)})
				}
			}
		}
	}
	for _, a := range aliases {
		if a[1] != "" {
			if _, ok := models[a[0]]; !ok {
				_ = a
			}
			if _, ok := models[a[1]]; !ok {
				findings = append(findings, cflFinding{"E", fmt.Sprintf("alias `%s` → `%s`：目标模型不在册", a[0], a[1])})
			}
		}
	}
	if len(findings) > 0 {
		return 1, findings
	}
	return 0, nil
}

func cflSortedKeys(m map[string]bool) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	cflSortStrings(ks)
	return cflList(ks)
}

func cflSortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func cflList(s []string) string {
	q := make([]string, len(s))
	for i, v := range s {
		q[i] = "'" + v + "'"
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// cflSelfTest 与 py `run_self_test` 同五格（合成夹具 · 不碰真目标）。
func cflSelfTest() int {
	var fails []string
	expect := func(label, text string, wantRC int, wantFace string) {
		dir, err := os.MkdirTemp("", "fleet-loadable-selftest-")
		if err != nil {
			fails = append(fails, label+"：建临时目录失败")
			return
		}
		defer os.RemoveAll(dir)
		p := filepath.Join(dir, "fleet.yaml")
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			fails = append(fails, label+"：写夹具失败")
			return
		}
		rc, findings := cflCheck(p)
		if rc != wantRC {
			fails = append(fails, fmt.Sprintf("%s：期望 rc=%d，实得 rc=%d（findings=%s）", label, wantRC, rc, findings))
			return
		}
		if wantFace != "" {
			hit := false
			for _, f := range findings {
				if f.face == wantFace {
					hit = true
				}
			}
			if !hit {
				fails = append(fails, fmt.Sprintf("%s：期望命中面 %s，实得 %s", label, wantFace, findings))
			}
		}
	}
	expect("good", cflGoodFixture, 0, "")
	expect("fork-inconsistent", cflBadForkFixture, 1, "D")
	expect("bad-fields", cflBadFieldsFixture, 1, "B")
	expect("empty-domain", cflEmptyFixture, 2, "A")
	if rc, _ := cflCheck("/nonexistent/fleet.yaml"); rc != 2 {
		fails = append(fails, fmt.Sprintf("missing-file：期望 rc=2，实得 rc=%d", rc))
	}
	if len(fails) > 0 {
		fmt.Printf("✗ 自检不过（%d 条）：\n", len(fails))
		for _, f := range fails {
			fmt.Println("  - " + f)
		}
		return 2
	}
	fmt.Println("✓ check-fleet-loadable 自检全过（5 格）")
	return 0
}

const cflGoodFixture = `models:
  Foo-Fork:
    - { host: m1, backend: llama-server, file: /srv/models/foo.gguf, mem_gb: 8, cmd: ["/opt/run-foo.sh", "-m", "{file}"] }
    - { host: m2, backend: llama-server, file: /srv/models/foo.gguf, mem_gb: 8, cmd: ["/opt/run-foo.sh", "-m", "{file}"] }
  Bar:
    - { host: m1, backend: llama-server, file: /srv/models/bar.gguf, mem_gb: 4 }
aliases:
fleet:
  m1: { host: <controller-ip>, port: 8100, os: macos, kind: work }
  m2: { host: <worker-ip>, port: 8100, os: ubuntu, kind: ai }
`

const cflBadForkFixture = `models:
  Foo-Fork:
    - { host: m1, backend: llama-server, file: /srv/models/foo.gguf, mem_gb: 8, cmd: ["/opt/run-foo.sh", "-m", "{file}"] }
    - { host: m2, backend: llama-server, file: /srv/models/foo.gguf, mem_gb: 8 }
fleet:
  m1: { host: <controller-ip>, port: 8100, os: macos, kind: work }
  m2: { host: <worker-ip>, port: 8100, os: ubuntu, kind: ai }
`

const cflBadFieldsFixture = `models:
  Broken:
    - { host: m9, backend: llama-server, file: /srv/models/x.bin, mem_gb: 0 }
fleet:
  m1: { host: <controller-ip>, port: 8100, os: macos, kind: work }
`

const cflEmptyFixture = "models:\nfleet:\n  m1: { host: <controller-ip>, port: 8100 }\n"

// checkFleetLoadableGateMain 是 `zerg gate check check-fleet-loadable` 的进程内入口。
func checkFleetLoadableGateMain(stdout, stderr io.Writer, argv []string) int {
	fleet := ""
	selfTest := false
	for i := 0; i < len(argv); i++ {
		switch {
		case argv[i] == "--self-test":
			selfTest = true
		case argv[i] == "--fleet" && i+1 < len(argv):
			fleet = argv[i+1]
			i++
		case strings.HasPrefix(argv[i], "--fleet="):
			fleet = strings.TrimPrefix(argv[i], "--fleet=")
		}
	}
	if selfTest {
		return cflSelfTest()
	}
	if fleet == "" {
		cwd, _ := os.Getwd()
		fleet = filepath.Join(cwd, "gateway", "fleet.yaml")
	}
	rc, findings := cflCheck(fleet)
	if rc == 2 {
		fmt.Fprintln(stdout, "✗ 不给结论（rc=2）：")
		for _, f := range findings {
			fmt.Fprintf(stdout, "  [%s] %s\n", f.face, f.msg)
		}
		return 2
	}
	if rc == 1 {
		fmt.Fprintf(stdout, "✗ 可加载性缺陷 %d 条（fleet=%s）：\n", len(findings), fleet)
		for _, f := range findings {
			fmt.Fprintf(stdout, "  [%s] %s\n", f.face, f.msg)
		}
		return 1
	}
	fmt.Fprintf(stdout, "✓ 可加载判据全过（fleet=%s）：必备字段齐 · 引用都在册 · 引擎实现一致\n", fleet)
	return 0
}
