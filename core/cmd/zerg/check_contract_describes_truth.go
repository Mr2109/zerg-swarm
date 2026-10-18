// check_contract_describes_truth.go —— 门·**契约描述 ↔ 真源键集对拍**（缺口 `GAP-20260927-228`）。
//
// 病（缺口逐字）：契约登记表 `core/internal/contract/registry.json` 每条 entry 都有一格
// `truth_shape`（**人写的描述**，说真源长什么样）。描述与真源**漂**了没有任何门管得住：
// 真源加一个键、描述里没点，消费方（Go 代码）就继续按旧名单跑，**静默少读一格**。
// 已发生的两例：
//
//	① `S-m`（删面）的 `truth_shape` 自称「源码里不另写第二份口径与名单」，而同格自己又列了
//	   一整组「五类盲区」格名 ⇒ 那几格在 `family_impact_del.go` 里是**逐字手抄的第二个副本**
//	   （`impactDelBlindKeys`）。描述与代码两份名单，谁也不知道谁旧。
//	② `S-j`（分层预算）的 `cap`：真源里有几格、描述点了几格，**只能人眼现读**。
//
// 治法：把「描述点名的键集 ⊇ 真源实读键集」写成机检判据 —— **漏列即红**，并且
// **✗ 不依赖描述是否自称数字**（描述写没写「共 N 项」「五类」都不看；只比**集合**）。
//
// ── 判据（逐字 · 三段）─────────────────────────────────────────────────────
//
//	① 面：只判 `truth` 路径是 `.json`、且件在盘、且顶层是 JSON 对象的 entry。
//	   其余（`.go` / 目录 / 非对象）**不属本门面** ⇒ 跳过并计入「不在本门面」。
//	② 顶层：真源顶层键（去元数据 `schema` / `id` / `note` / `not_in_public_face` 与一切 `_`
//	   起头的键）**每一个**都必须在 `truth_shape` 里被点名。
//	③ 子对象（深度 ≤2）：真源顶层键 K 的值是对象时，若 `truth_shape` **点名了 K 的至少一个子键**
//	   （⇒ 描述对该节**已开始枚举**），则 K 的**全部**子键都必须在描述里被点名；漏一即红。
//	   一个都没点名 ⇒ 判「未枚举」（也红：点名集 = 空集 ⊂ 真源集，同属描述不全）。
//
//	「点名」= 该键名（或 `父.子` 形态）在 `truth_shape` 里**独立成词**出现（前/后字符不是
//	字母 / 数字 / CJK / `.`）。⇒ 散文里把键名与别的字黏在一起（例：「写失败三档」）**不算点名**，
//	单独归「黏连」类（与「真漏列」分开报，见下）。★ 这是本门唯一的口径软化点，照实写在输出里。
//
// ── 出口（退码）────────────────────────────────────────────────────────────
//
//	0 = 全齐（本门面每条 entry 的描述都覆盖真源键集）
//	1 = 有漏列 / 有未枚举 / 有黏连（**逐条点名**哪个 entry 的哪个节点漏了哪些键）
//	8 = 判不了（registry 读不到 / 解不开 / `entries` 为空 · 某条在面内的真源读不到或解不开）
//	     —— 8 = 「有 BLOCKED（读不到）不许当健康」（`exitcodes.go` 主表 `blocked`）
//
// ── 纪律 ───────────────────────────────────────────────────────────────────
//
//	· **零写面**：main 路径一个字节都不写；`--self-test` 的夹具全在**内存**里（不落盘、不 mkdtemp）。
//	· **输出定序**：entry 按 registry 文件序；同一 entry 内节点按真源键序（json 对象 → 排序后键序）；
//	  漏的键名字典序 ⇒ 逐字节可复跑。
//	· **不判「自我声称的数字」**：描述里的「五类」「十键」「三件」一个都不读（数字与集合脱钩 ⇒ 不判）。
//	· **不改真目标**：本门只读 registry 与真源；✗ 不写、✗ 不修、✗ 不自动补描述。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// cdtrGateName —— 判据名（`zerg gate check <名>` 的入口键 · 与 inproc 分发表同字）。
	cdtrGateName = "check-contract-describes-truth"
	// cdtrRegistryRel —— 契约登记表（本门的**唯一输入**）。
	cdtrRegistryRel = "core/internal/contract/registry.json"
	// cdtrSelfRel —— 本件（自指说明用）。
	cdtrSelfRel = "core/cmd/zerg/check_contract_describes_truth.go"
)

// cdtrMetaTopKeys —— 跨件通用的自述 / 出处 / 落点标记：**不是契约的键**，不许被要求点名
// （与 `check-contract-registry` 的 R9 元数据键逐字同一份口径 —— 两门对「哪些键不算契约」只能有一套说法）。
var cdtrMetaTopKeys = map[string]bool{"schema": true, "id": true, "note": true, "not_in_public_face": true}

// cdtrFindingKinds —— 三类红的标签（**闭集** · 输出与计数同读这一处）。
const (
	cdtrKindMiss = "漏列"  // 描述里**一个字都没有**这个键名
	cdtrKindGlue = "黏连"  // 字面出现过，但被别的字黏住 ⇒ 不算独立点名
	cdtrKindZero = "未枚举" // 该节点一个子键都没点名（点名集 = 空集）
)

// cdtrEntry —— registry 的一条（只取本门要的四格）。
type cdtrEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Truth      string `json:"truth"`
	TruthShape string `json:"truth_shape"`
}

// cdtrFinding —— 一条红（逐条点名到 entry / 节点 / 键）。
type cdtrFinding struct {
	Entry   string   `json:"entry"`
	Node    string   `json:"node"`
	Kind    string   `json:"kind"`
	Missing []string `json:"missing"`
	Total   int      `json:"total"`
}

// cdtrReport —— 一次判的机器面。
type cdtrReport struct {
	Gate     string        `json:"gate"`
	Root     string        `json:"root"`
	Entries  int           `json:"entries"`
	InScope  int           `json:"in_scope"`
	Skipped  []string      `json:"skipped"`
	Findings []cdtrFinding `json:"findings"`
	Miss     int           `json:"miss_nodes"`
	Glue     int           `json:"glue_nodes"`
	Zero     int           `json:"zero_nodes"`
	Keys     int           `json:"missing_keys"`
	Err      string        `json:"error,omitempty"`
	Rc       int           `json:"rc"`
}

// cdtrTruthPath —— 从 `truth` 那一格里取**件路径**（registry 里有的格写成
// 「`core/internal/contract/xxx.json`（…）」或带说明尾巴 ⇒ 取首个路径 token）。
func cdtrTruthPath(truth string) string {
	s := strings.TrimSpace(truth)
	if s == "" {
		return ""
	}
	// 切在第一个全角/半角括号、反引号、空白处（这三者是 registry 里出现过的分隔形态）。
	cut := len(s)
	for _, sep := range []string{"（", "(", "`", " ", "\t", "·"} {
		if i := strings.Index(s, sep); i >= 0 && i < cut {
			cut = i
		}
	}
	p := strings.TrimSpace(s[:cut])
	return strings.TrimSuffix(p, "/")
}

// cdtrWordRune —— 「词字符」= 字母 / 数字 / CJK（`unicode.IsLetter` 覆盖 CJK）/ 下划线。
func cdtrWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// cdtrNamed —— 键名 key 是否在 shape 里**独立成词**出现（前/后不是词字符、也不是 `.`）。
//
//	★ `.` 也算「非边界」：`impact-state.json` 里的 `state` 不算点名 `state` 这个键
//	  （否则文件名会把注记键「洗绿」—— 门自己先被假绿骗过就不能用）。
func cdtrNamed(shape, key string) bool {
	if key == "" || shape == "" {
		return false
	}
	off := 0
	for {
		i := strings.Index(shape[off:], key)
		if i < 0 {
			return false
		}
		i += off
		j := i + len(key)
		prevOK, nextOK := true, true
		if i > 0 {
			prev, _ := utf8.DecodeLastRuneInString(shape[:i])
			prevOK = !cdtrWordRune(prev) && prev != '.'
		}
		if j < len(shape) {
			next, _ := utf8.DecodeRuneInString(shape[j:])
			nextOK = !cdtrWordRune(next) && next != '.'
		}
		if prevOK && nextOK {
			return true
		}
		if j >= len(shape) {
			return false
		}
		off = i + 1
	}
}

// cdtrNamedPath —— 顶层键是否被点名：独立成词 **或** 以「`键.子键`」的**点路径**形态出现，
// 且点后面那一段确实该键的子键（⇒ 描述引用的是这个节点，不是在写文件名）。
//
//	★ 为什么要这一条：`dirs.index` 点名了 `dirs`；而 `impact-state.json` 里的 `state` **不算**
//	  点名 `state`（`json` 不是 `state` 的子键）—— 两者形态相同、语义相反，只能靠子键集分开。
func cdtrNamedPath(shape, key string, kids map[string]bool) bool {
	if cdtrNamed(shape, key) {
		return true
	}
	off := 0
	for {
		i := strings.Index(shape[off:], key+".")
		if i < 0 {
			return false
		}
		i += off
		j := i + len(key) + 1
		prevOK := true
		if i > 0 {
			prev, _ := utf8.DecodeLastRuneInString(shape[:i])
			prevOK = !cdtrWordRune(prev) && prev != '.'
		}
		if prevOK && j < len(shape) {
			// 点后面那一段（到下一个非词字符为止 · 只认一级）
			k := j
			for k < len(shape) {
				r, sz := utf8.DecodeRuneInString(shape[k:])
				if !cdtrWordRune(r) {
					break
				}
				k += sz
			}
			if kids[shape[j:k]] {
				return true
			}
		}
		off = i + 1
		if off >= len(shape) {
			return false
		}
	}
}

// cdtrClassify —— 漏点名的键归「漏列」还是「黏连」：字面在描述里出现过（但没独立成词）⇒ 黏连。
func cdtrClassify(shape, key string) string {
	if strings.Contains(shape, key) {
		return cdtrKindGlue
	}
	return cdtrKindMiss
}

// cdtrSortedKeys —— 对象键（去 `_` 起头与元数据键）**排序后**返回（定序 ⇒ 可逐字节复跑）。
func cdtrSortedKeys(obj map[string]interface{}, top bool) []string {
	out := make([]string, 0, len(obj))
	for k := range obj {
		if strings.HasPrefix(k, "_") {
			continue
		}
		if top && cdtrMetaTopKeys[k] {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cdtrAddFinding —— 追加一条红（漏的键字典序 · 空集不追加）。
func cdtrAddFinding(rep *cdtrReport, entry, node, kind string, missing []string, total int) {
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	rep.Findings = append(rep.Findings, cdtrFinding{Entry: entry, Node: node, Kind: kind, Missing: missing, Total: total})
	switch kind {
	case cdtrKindMiss:
		rep.Miss++
	case cdtrKindGlue:
		rep.Glue++
	case cdtrKindZero:
		rep.Zero++
	}
	rep.Keys += len(missing)
}

// cdtrCheckEntry —— 判一条 entry（truth 的字节由调用方给 ⇒ 自检可全在内存里跑）。
//
//	返回 rc≠0 ⇒ 判不了（8）：原因写进 why。
func cdtrCheckEntry(rep *cdtrReport, e cdtrEntry, raw []byte) (rc int, why string) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return 8, fmt.Sprintf("真源 `%s` 解不开（%v）", cdtrTruthPath(e.Truth), err)
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s 真源不是 JSON 对象（顶层是 %s）⇒ 不在本门面", e.ID, cdtrJSONKind(v)))
		return 0, ""
	}
	shape := e.TruthShape
	// ② 顶层键（子键集先建好：点路径形态要拿它判「是不是在点这个节点」）
	top := cdtrSortedKeys(obj, true)
	kidSets := map[string]map[string]bool{}
	for _, k := range top {
		sub, ok := obj[k].(map[string]interface{})
		if !ok {
			continue
		}
		set := map[string]bool{}
		for _, s := range cdtrSortedKeys(sub, false) {
			set[s] = true
		}
		kidSets[k] = set
	}
	var missTop, glueTop []string
	for _, k := range top {
		if cdtrNamedPath(shape, k, kidSets[k]) {
			continue
		}
		if cdtrClassify(shape, k) == cdtrKindGlue {
			glueTop = append(glueTop, k)
		} else {
			missTop = append(missTop, k)
		}
	}
	cdtrAddFinding(rep, e.ID, "顶层", cdtrKindMiss, missTop, len(top))
	cdtrAddFinding(rep, e.ID, "顶层", cdtrKindGlue, glueTop, len(top))
	// ③ 子对象（深度 2）
	for _, k := range top {
		sub, ok := obj[k].(map[string]interface{})
		if !ok {
			continue
		}
		kids := cdtrSortedKeys(sub, false)
		if len(kids) == 0 {
			continue
		}
		var hit []string
		for _, s := range kids {
			if cdtrNamed(shape, s) || cdtrNamed(shape, k+"."+s) {
				hit = append(hit, s)
			}
		}
		if len(hit) == 0 {
			// 整个节点一个都没点 ⇒ 未枚举（点名集 = 空集 ⊂ 真源集）。
			cdtrAddFinding(rep, e.ID, k, cdtrKindZero, append([]string{}, kids...), len(kids))
			continue
		}
		var miss, glue []string
		for _, s := range kids {
			if cdtrNamed(shape, s) || cdtrNamed(shape, k+"."+s) {
				continue
			}
			if cdtrClassify(shape, s) == cdtrKindGlue || cdtrClassify(shape, k+"."+s) == cdtrKindGlue {
				glue = append(glue, s)
			} else {
				miss = append(miss, s)
			}
		}
		cdtrAddFinding(rep, e.ID, k, cdtrKindMiss, miss, len(kids))
		cdtrAddFinding(rep, e.ID, k, cdtrKindGlue, glue, len(kids))
	}
	return 0, ""
}

// cdtrJSONKind —— JSON 值的类型名（只用于「不在本门面」那一行，不进退码）。
func cdtrJSONKind(v interface{}) string {
	switch v.(type) {
	case []interface{}:
		return "数组"
	case string:
		return "字符串"
	case float64:
		return "数字"
	case bool:
		return "布尔"
	case nil:
		return "null"
	}
	return "非对象"
}

// cdtrRun —— 真跑：root 下读 registry（read 注入 ⇒ 自检不落盘）。
func cdtrRun(root string, read func(rel string) ([]byte, error)) (cdtrReport, int) {
	rep := cdtrReport{Gate: cdtrGateName, Root: root, Skipped: []string{}}
	regRaw, err := read(cdtrRegistryRel)
	if err != nil {
		rep.Rc, rep.Err = 8, "登记表读不到（"+err.Error()+"）"
		return rep, 8
	}
	var reg struct {
		Entries []cdtrEntry `json:"entries"`
	}
	if err := json.Unmarshal(regRaw, &reg); err != nil {
		rep.Rc, rep.Err = 8, "登记表解不开（"+err.Error()+"）"
		return rep, 8
	}
	rep.Entries = len(reg.Entries)
	if rep.Entries == 0 {
		rep.Rc, rep.Err = 8, "登记表 `entries` 为空 ⇒ 空转，不给结论"
		return rep, 8
	}
	for _, e := range reg.Entries {
		p := cdtrTruthPath(e.Truth)
		if p == "" {
			rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s `truth` 格取不到路径 ⇒ 不在本门面", e.ID))
			continue
		}
		if !strings.HasSuffix(strings.ToLower(p), ".json") {
			rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s 真源 `%s` 不是 .json ⇒ 不在本门面（键集无从机检）", e.ID, p))
			continue
		}
		raw, err := read(filepath.ToSlash(p))
		if err != nil {
			rep.Rc, rep.Err = 8, "真源 `"+p+"` 读不到（"+err.Error()+"）"
			return rep, 8
		}
		rep.InScope++
		if rc, why := cdtrCheckEntry(&rep, e, raw); rc != 0 {
			rep.Rc, rep.Err = rc, why
			return rep, rc
		}
	}
	if len(rep.Findings) > 0 {
		rep.Rc = 1
		return rep, 1
	}
	rep.Rc = 0
	return rep, 0
}

// cdtrRender —— 人读面（定序 · 逐条点名）。
func cdtrRender(w io.Writer, rep cdtrReport) {
	fmt.Fprintf(w, "门 · 契约描述 ↔ 真源对拍（缺口 GAP-20260927-228）· 仓根 %s\n", rep.Root)
	fmt.Fprintf(w, "  真源：%s（%d 条 · 本门面 %d 条 · 不在本门面 %d 条）\n",
		cdtrRegistryRel, rep.Entries, rep.InScope, len(rep.Skipped))
	if rep.Rc == 8 {
		fmt.Fprintf(w, "  ⇒ **判不了**（rc=8）：%s —— ✗ 读不到不许当健康\n", rep.Err)
		return
	}
	fmt.Fprintf(w, "  判据：`truth_shape` 点名的键集 ⊇ 真源实读键集（**漏一即红** · ✗ 不看描述自称的数字）\n")
	for _, s := range rep.Skipped {
		fmt.Fprintf(w, "  · %s\n", s)
	}
	if len(rep.Findings) == 0 {
		fmt.Fprintf(w, "⇒ 描述 ↔ 真源一致（本门面 %d 条全齐 · 键集无漏列 · rc=0）\n", rep.InScope)
		return
	}
	fmt.Fprintf(w, "  ✗ 红 %d 条节点（漏列 %d · 黏连 %d · 未枚举 %d · 漏点名键共 %d 个）：\n",
		len(rep.Findings), rep.Miss, rep.Glue, rep.Zero, rep.Keys)
	for _, f := range rep.Findings {
		var keys []string
		for _, k := range f.Missing {
			keys = append(keys, "`"+k+"`")
		}
		switch f.Kind {
		case cdtrKindZero:
			fmt.Fprintf(w, "    ✗ %s %s：**未枚举**（真源 %d 键一个都没点）——%s\n",
				f.Entry, f.Node, f.Total, strings.Join(keys, " "))
		case cdtrKindGlue:
			fmt.Fprintf(w, "    ✗ %s %s：黏连 %d/%d 键 —— %s（字面出现过，但被别的字黏住 ⇒ 不算点名）\n",
				f.Entry, f.Node, len(f.Missing), f.Total, strings.Join(keys, " "))
		default:
			fmt.Fprintf(w, "    ✗ %s %s：漏列 %d/%d 键 —— %s（描述里一个字都没有）\n",
				f.Entry, f.Node, len(f.Missing), f.Total, strings.Join(keys, " "))
		}
	}
	fmt.Fprintf(w, "⇒ 有漏列（rc=1）：上列每条都要把漏的键名补进 `truth_shape`，或改真源 —— 两者必须同批\n")
}

// ── 自检夹具（全内存 · 不落盘）──────────────────────────────────────────────

// cdtrFixRead —— 把「相对路径 → 字节」的固定表包成 read（自检用）。
func cdtrFixRead(files map[string]string) func(string) ([]byte, error) {
	return func(rel string) ([]byte, error) {
		if b, ok := files[rel]; ok {
			return []byte(b), nil
		}
		return nil, fmt.Errorf("夹具里没有 %s", rel)
	}
}

// cdtrFixtures —— 正控 + 负控（成对 · 每格都断言 rc 与「点名了哪个键」）。
func cdtrFixtures() []struct {
	name string
	rc   int
	want []string
	run  func() (cdtrReport, int, string)
} {
	reg := func(shape string) string {
		return `{"entries":[{"id":"S-t","name":"夹具","truth":"core/internal/contract/fix.json","truth_shape":` +
			cdtrQuote(shape) + `}]}`
	}
	truth5 := `{"schema":"x","id":"y","_出处":"z","甲":{"一":"1","二":"2","三":"3","四":"4","五":"5"}}`
	run := func(regBody string, files map[string]string) (cdtrReport, int, string) {
		rep, rc := cdtrRun("(夹具)", func(rel string) ([]byte, error) {
			if rel == cdtrRegistryRel {
				if regBody == "" {
					return nil, fmt.Errorf("夹具里登记表缺件")
				}
				return []byte(regBody), nil
			}
			return cdtrFixRead(files)(rel)
		})
		var b strings.Builder
		cdtrRender(&b, rep)
		return rep, rc, b.String()
	}
	fixPath := "core/internal/contract/fix.json"
	return []struct {
		name string
		rc   int
		want []string
		run  func() (cdtrReport, int, string)
	}{
		{
			name: "① 正控：真源 5 子键 ⟷ 描述 5 全点 ⇒ rc=0",
			rc:   0,
			want: []string{"描述 ↔ 真源一致"},
			run: func() (cdtrReport, int, string) {
				return run(reg("`甲`（一 / 二 / 三 / 四 / 五）+ 元数据不点名"), map[string]string{fixPath: truth5})
			},
		},
		{
			name: "② 负控：真源 5 子键 ⟷ 描述只点 4 ⇒ rc=1 且点名漏的那一格",
			rc:   1,
			want: []string{"S-t 甲", "漏列 1/5", "`五`"},
			run: func() (cdtrReport, int, string) {
				return run(reg("`甲`（一 / 二 / 三 / 四）"), map[string]string{fixPath: truth5})
			},
		},
		{
			name: "③ 负控：同一件把漏的那一格补回 ⇒ rc=0（与 ② 成对）",
			rc:   0,
			want: []string{"描述 ↔ 真源一致"},
			run: func() (cdtrReport, int, string) {
				return run(reg("`甲`（一 / 二 / 三 / 四 / 五）"), map[string]string{fixPath: truth5})
			},
		},
		{
			name: "④ 负控：整节一个都不点 ⇒ 未枚举（rc=1 · 不因「没自称数字」而放过）",
			rc:   1,
			want: []string{"未枚举", "漏点名键共 5 个"},
			run: func() (cdtrReport, int, string) {
				return run(reg("`甲`（这一节见真源 · 一格都不点）"), map[string]string{fixPath: truth5})
			},
		},
		{
			name: "⑤ 负控：顶层键漏点 ⇒ rc=1（顶层与子层两段各判各的）",
			rc:   1,
			want: []string{"S-t 顶层", "漏列 1/1", "`甲`"},
			run: func() (cdtrReport, int, string) {
				return run(`{"entries":[{"id":"S-t","name":"夹具","truth":"core/internal/contract/fix.json","truth_shape":"（没说顶层）"}]}`,
					map[string]string{fixPath: `{"甲":{"一":"1"}}`})
			},
		},
		{
			name: "⑥ 负控：键名被别的字黏住 ⇒ 判「黏连」而不是放过",
			rc:   1,
			want: []string{"黏连", "`五`"},
			run: func() (cdtrReport, int, string) {
				return run(reg("`甲`（一 / 二 / 三 / 四 / 五五类）"), map[string]string{fixPath: truth5})
			},
		},
		{
			name: "⑦ 负控：元数据键 / `_` 起头的键不要求点名（不许把自述格算成契约键）",
			rc:   0,
			want: []string{"描述 ↔ 真源一致"},
			run: func() (cdtrReport, int, string) {
				return run(reg("`甲`（一）"), map[string]string{fixPath: `{"schema":"x","id":"y","note":"n","not_in_public_face":["a"],"_出处":"z","甲":{"一":"1"}}`})
			},
		},
		{
			name: "⑧ 负控：注册表读不到 ⇒ rc=8（判不了 · 不许当健康）",
			rc:   8,
			want: []string{"判不了", "rc=8"},
			run: func() (cdtrReport, int, string) {
				return run("", map[string]string{})
			},
		},
		{
			name: "⑨ 负控：真源件在盘但解不开 ⇒ rc=8",
			rc:   8,
			want: []string{"解不开"},
			run: func() (cdtrReport, int, string) {
				return run(reg("`甲`（一）"), map[string]string{fixPath: `{"甲":`})
			},
		},
		{
			name: "⑩ 正控：非 .json 真源（`.go`）⇒ 不在本门面、不判红（照实计数）",
			rc:   0,
			want: []string{"不在本门面 1 条", "不是 .json"},
			run: func() (cdtrReport, int, string) {
				return run(`{"entries":[{"id":"S-t","name":"夹具","truth":"core/cmd/zerg/x.go","truth_shape":"随便"}]}`,
					map[string]string{"core/cmd/zerg/x.go": "package main\n"})
			},
		},
	}
}

// cdtrQuote —— 极小的 JSON 字面量包装（自检夹具拼串用 · 只吃 ASCII+CJK 的普通串）。
func cdtrQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cdtrSelfTest —— 自检（正控 + 负控 · 逐格断言 rc 与「点名了哪个键」）。
func cdtrSelfTest(stdout io.Writer) int {
	fmt.Fprintln(stdout, "门 · 契约描述 ↔ 真源对拍 —— 自检档（全内存夹具 · 不碰真目标）")
	fails := 0
	cases := cdtrFixtures()
	for _, c := range cases {
		_, rc, out := c.run()
		ok := rc == c.rc
		missing := []string{}
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				ok = false
				missing = append(missing, w)
			}
		}
		if ok {
			fmt.Fprintf(stdout, "  ✓ %s（rc=%d）\n", c.name, rc)
			continue
		}
		fails++
		fmt.Fprintf(stdout, "  ✗ %s：rc=%d（期望 %d）", c.name, rc, c.rc)
		if len(missing) > 0 {
			fmt.Fprintf(stdout, " · 输出缺 %s", strings.Join(missing, " / "))
		}
		fmt.Fprintf(stdout, "\n%s\n", out)
	}
	if fails > 0 {
		fmt.Fprintf(stdout, "⇒ 自检失败（rc=1）：%d/%d 格不过\n", fails, len(cases))
		return 1
	}
	fmt.Fprintf(stdout, "✓ 自检全过（%d 组：正控 3 + 负控 7 · 含成对回补格 · 全内存夹具）\n", len(cases))
	return 0
}

// ── 根定位 ─────────────────────────────────────────────────────────────────

// cdtrIsRoot —— 判一个目录是不是仓根（认得本门的唯一输入件即可）。
func cdtrIsRoot(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(cdtrRegistryRel)))
	return err == nil && !st.IsDir()
}

// cdtrFindRoot —— 缺省根：cwd 上溯 ≤12 层 → 可执行文件上溯两层（`<root>/bin/<名>`）→ 判不了。
// ✗ 不猜、✗ 不硬编码绝对路径（家目录/工作目录形态一律不落字面）。
func cdtrFindRoot() (string, bool) {
	if wd, err := os.Getwd(); err == nil {
		d := wd
		for i := 0; i < 12; i++ {
			if cdtrIsRoot(d) {
				return d, true
			}
			parent := filepath.Dir(d)
			if parent == d {
				break
			}
			d = parent
		}
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(filepath.Dir(exe))
		if cdtrIsRoot(d) {
			return d, true
		}
	}
	return "", false
}

func cdtrReadFromRoot(root string) func(string) ([]byte, error) {
	return func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	}
}

// ── 入口 ───────────────────────────────────────────────────────────────────

// checkContractDescribesTruthGateMain —— 入口（argv 不含程序名 · 与其它判据件同形）。
//
//	退码：0 全齐 · 1 有漏列/黏连/未枚举 · 8 判不了（读不到 registry 或真源）· 2 用法错
func checkContractDescribesTruthGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	asJSON, selfTest, wantHelp := false, false, false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			wantHelp = true
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprintln(stderr, "argument --root: expected one argument")
				return 2
			}
			i++
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--json":
			asJSON = true
		case a == "--self-test":
			selfTest = true
		case a == "--selftest":
			selfTest = true
		default:
			fmt.Fprintln(stderr, "unrecognized arguments: "+a)
			return 2
		}
	}
	if wantHelp {
		fmt.Fprint(stdout, `用法：zerg gate check `+cdtrGateName+` [--root <仓根>] [--json] [--self-test]

判据：对 `+cdtrRegistryRel+` 每条 entry 的 `+"`truth_shape`"+`：它点名的键集必须是真源（`+"`truth`"+`
指向的 .json）实读键集的**超集** —— 顶层键与「描述已开始枚举」的子对象键各判一遍，
**漏一即红**，且 **✗ 不依赖描述自称的数字**（写没写「共 N 项」「五类」都不看）。

退码：0 全齐 · 1 有漏列（逐条点名哪个 entry 的哪个节点漏了哪些键）·
      8 判不了（registry 读不到 / 解不开 / entries 空 · 真源读不到或解不开）
`)
		return 0
	}
	if selfTest {
		return cdtrSelfTest(stdout)
	}
	if root == "" {
		r, ok := cdtrFindRoot()
		if !ok {
			if asJSON {
				fmt.Fprintf(stdout, `{"gate":%q,"rc":8,"error":"根定位不到（不以 core/internal/contract/registry.json 为标记找到仓根）"}`+"\n", cdtrGateName)
			} else {
				fmt.Fprintf(stdout, "门 · 契约描述 ↔ 真源对拍：**判不了**（根定位不到）⇒ rc=8 —— ✗ 读不到就绝不给绿\n")
			}
			return 8
		}
		root = r
	}
	rep, rc := cdtrRun(root, cdtrReadFromRoot(root))
	rep.Root = root
	rep.Rc = rc
	if asJSON {
		b, err := json.Marshal(rep)
		if err != nil {
			fmt.Fprintln(stderr, "报告序列化失败："+err.Error())
			return 8
		}
		fmt.Fprintf(stdout, "%s\n", b)
		return rc
	}
	cdtrRender(stdout, rep)
	return rc
}
