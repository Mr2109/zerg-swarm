// hatch_from_egg_tiers_test.go —— `GAP-20261015-25` 的判据件（**D5 / D6a / D10 / D11 / D12 五档**）。
//
// 判据口径（逐档两条腿）：
//
//	① **档位面**（纯判据 · 无盘）：给闸喂「该拒」的输入 ⇒ 必须点名该档、报话**逐字等于**设计面常量、
//	   退码等于 13 档表该行；喂「该放」的输入 ⇒ 必须放行（✗ 不误伤）；
//	② **接线面**：同一批档在孵化入口 `hatchFromEgg` 上实跑一遍 ⇒ 拒的**阶段**对、`hfeExitCode` 取到该档的码。
//
// ★ 设计面报话常量**独立写在本件**（✗ 从生产表反读）：生产表被改成「另一种说法」时本件当场红 ——
// 判据不许拿被测对象自己判自己。D11 的 `%s`/`%s` ＝ 还差多少 / 可回收卵清单（与 D7 同法按「模板 ＋ 填值」对）。
//
// ★ D8 不在本件：签名块未落（`GAP-20261015-14`）＋ 真验需 `cosign`（真机面）—— 见 `hatch_from_egg_tiers.go` 件尾。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/store/eggstore"
)

// ---- 设计面（逐字 · 独立于生产表）-------------------------------------------------------------

const (
	hgtDesignD5  = "拒孵：契约段缺必需字段（缺项必拒，不默默取默认值）"
	hgtDesignD6a = "拒孵：段的 mediaType 本机不认识，且为孵化必需 —— 拒（不猜）"
	hgtDesignD10 = "拒孵：卵是未完工的半成品（缺完工标记）"
	hgtDesignD11 = "拒孵：可用磁盘不足（需更多 %s）；可回收卵清单：%s"
	hgtDesignD12 = "孵化继续（不报错）：机器茧壁与卵声明不匹配 ⇒ 改用卵内那份；已在观测面标注"
)

// hgtGoodContract —— 一份**十维齐全**的契约（照 `agent/internal/enclosure/contract.go` 的结构形状）。
const hgtGoodContract = `{
  "schema_version": 1,
  "filesystem": {"ro_binds": [{"host": "/models", "space": "/models"}], "scratch_dir": "/scratch"},
  "ipc": {"allow_in_space": true, "multi_process": false},
  "exec": {"prot_exec": false, "fork_exec": true},
  "devices": {"nodes": ["/dev/dri/renderD128"]},
  "network": {"allow_loopback": true},
  "resources": {"mem_limit_gb": 8},
  "readiness": {"probe": "http"},
  "lifecycle": {"idle_ttl_sec": 60},
  "dependencies": {"engines": ["llama.cpp"]},
  "seccomp": {"profile": "default"}
}`

// hgtTierWant —— 一档的（报话模板 · 退码）期望，取自设计面而非生产表。
func hgtTierWant(tier string) (string, int) {
	switch tier {
	case "D5":
		return hgtDesignD5, exitUsage
	case "D6a":
		return hgtDesignD6a, exitUsage
	case "D10":
		return hgtDesignD10, exitBlocked
	case "D11":
		return hgtDesignD11, exitResource
	case "D12":
		return hgtDesignD12, exitOK
	}
	return "", -1
}

// hgtArg — 夹具段表项（可带 mediaType；`off` 落在第一个对齐边界）。
func hgtArg(path, sha string, size int64, mediaType string) string {
	mt := ""
	if mediaType != "" {
		mt = fmt.Sprintf(`,"mediaType":%q`, mediaType)
	}
	return fmt.Sprintf(`{"path":%q,"off":%d,"size":%d,"sha256":%q,"mode":420%s}`,
		path, zegMinAlign, size, sha, mt)
}

// hgtBody 组 v1 索引体（可带可加面键 `wall_min_version`）。
func hgtBody(entries, wallMin string) []byte {
	extra := ""
	if wallMin != "" {
		extra = fmt.Sprintf(`,"wall_min_version":%q`, wallMin)
	}
	if entries == "" {
		entries = "[]"
	}
	return []byte(fmt.Sprintf(`{"magic":"ZEGG","version":%d,"align":%d,"entries":%s%s}`,
		zegFormatVerV1, zegMinAlign, entries, extra))
}

// ─────────────────────────── D5 ───────────────────────────

func TestHgtContractD5TwoStates(t *testing.T) {
	wantMsg, wantCode := hgtTierWant("D5")

	// 态①：十维齐全 ⇒ 放行（✗ 判据不许把好契约判红）。
	if err := hgtCheckContract([]byte(hgtGoodContract), "contract/contract.json"); err != nil {
		t.Fatalf("齐全契约被判红（误伤）：%v", err)
	}

	// 态②：缺一维（resources = 限额 nil）⇒ 点名 D5、报话逐字、码 2。
	broken := strings.Replace(hgtGoodContract, `"resources": {"mem_limit_gb": 8},`, "", 1)
	err := hgtCheckContract([]byte(broken), "contract/contract.json")
	if err == nil {
		t.Fatalf("缺 resources 维度竟被判绿")
	}
	ref, ok := err.(*zegRefusal)
	if !ok {
		t.Fatalf("D5 拒不是 13 档表的带码拒：%T %v", err, err)
	}
	if ref.Tier != "D5" || ref.Msg != wantMsg || ref.Code != wantCode {
		t.Fatalf("D5 拒不达标：tier=%q msg=%q code=%d（要 %q / %q / %d）",
			ref.Tier, ref.Msg, ref.Code, "D5", wantMsg, wantCode)
	}
	if !strings.Contains(ref.Detail, "resources") {
		t.Fatalf("D5 细节读数没点出缺的那一维：%q", ref.Detail)
	}

	// 子项也必拒：scratch_dir 空 / readiness.probe 越界。
	for _, c := range []struct{ Name, Body, Want string }{
		{"scratch_dir空", strings.Replace(hgtGoodContract, `"scratch_dir": "/scratch"`, `"scratch_dir": ""`, 1), "filesystem.scratch_dir"},
		{"probe越界", strings.Replace(hgtGoodContract, `"probe": "http"`, `"probe": "grpc"`, 1), "readiness.probe"},
	} {
		e := hgtCheckContract([]byte(c.Body), "contract/contract.json")
		if e == nil {
			t.Fatalf("D5（%s）被判绿（真源 `Contract.Validate` 判它缺项）", c.Name)
		}
		if !strings.Contains(e.(*zegRefusal).Detail, c.Want) {
			t.Fatalf("D5（%s）细节没点名 %s：%q", c.Name, c.Want, e.(*zegRefusal).Detail)
		}
	}

	// 接线面：卵里带契约段（缺项）⇒ `hgtContractGate` 从段表里取到它并按 D5 拒。
	dir := t.TempDir()
	payload := []byte(`{"schema_version":1}`)
	egg := zegGap25Write(t, dir, "d5.egg", zegGap25Egg{
		PrefixVer: zegFormatVerV1, BodyVer: zegFormatVerV1,
		Body:    hgtBody("["+hgtArg("contract/contract.json", zegGap25SHA(payload), int64(len(payload)), "")+"]", ""),
		SegData: payload,
	})
	idx, perr := zegParseIndex(egg)
	if perr != nil {
		t.Fatalf("夹具卵解析失败：%v", perr)
	}
	if gerr := hgtContractGate(egg, idx); gerr == nil {
		t.Fatalf("带缺项契约段的卵竟放过（D5 没接上）")
	} else if r, ok := gerr.(*zegRefusal); !ok || r.Tier != "D5" || r.Msg != wantMsg || r.Code != wantCode {
		t.Fatalf("D5 接线面不达标：%v", gerr)
	}

	// 反向：卵里**不带**契约段 ⇒ 本档不介入（✗ 把「没有」当「缺字段」）。
	plain := zegGap25Write(t, dir, "no-contract.egg", zegGap25Egg{
		PrefixVer: zegFormatVerV1, BodyVer: zegFormatVerV1,
		Body:    hgtBody("["+hgtArg("recipe.json", zegGap25SHA(payload), int64(len(payload)), "")+"]", ""),
		SegData: payload,
	})
	pidx, perr2 := zegParseIndex(plain)
	if perr2 != nil {
		t.Fatalf("夹具卵解析失败：%v", perr2)
	}
	if gerr := hgtContractGate(plain, pidx); gerr != nil {
		t.Fatalf("没有契约段的卵被 D5 判红（越界）：%v", gerr)
	}
}

// TestHgtContractSourceParity —— 本件的**读端镜像**≡ 契约真源 `Contract.Validate()` 的十维清单。
//
// ★ 为什么要有这条：core 模块导不到 agent 模块（`core/go.mod` 无该 require）⇒ D5 的判据面只能镜像。
// 镜像不经对拍就是**第二真源**（真源加一维 ⇒ 本件判据静默少判一维）⇒ 现读真源逐条核。
func TestHgtContractSourceParity(t *testing.T) {
	p := filepath.Join("..", "..", "..", "agent", "internal", "enclosure", "contract.go")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("现读不到契约真源（%v）—— 本件在无 agent 模块的树里只核自身清单", err)
	}
	src := string(raw)
	i := strings.Index(src, "dims := []struct {")
	if i < 0 {
		t.Fatalf("契约真源里找不到 `dims := []struct`（`Contract.Validate()` 的形状变了 ⇒ 本件镜像失去锚点）")
	}
	block := src[i:]
	if j := strings.Index(block, "for _, d := range dims"); j > 0 {
		block = block[:j]
	}
	var got []string
	for k := 0; ; {
		p := strings.Index(block[k:], "{\"")
		if p < 0 {
			break
		}
		k += p + 2
		e := strings.IndexByte(block[k:], '"')
		if e < 0 {
			break
		}
		got = append(got, block[k:k+e])
		k += e + 1
	}
	sort.Strings(got)
	mine := append([]string{}, hgtContractDims...)
	sort.Strings(mine)
	if strings.Join(got, ",") != strings.Join(mine, ",") {
		t.Fatalf("D5 镜像清单与契约真源 `Contract.Validate()` 的十维不一致（镜像已漂移）：\n 真源 %v\n 本件 %v", got, mine)
	}
	t.Logf("D5 镜像 ≡ 真源：%d 维 %v", len(got), got)
}

// ─────────────────────────── D6a / D6b ───────────────────────────

func TestHgtMediaTypeD6aD6b(t *testing.T) {
	wantMsg, wantCode := hgtTierWant("D6a")
	unknown := "application/vnd.zerg.weights.v9+unknown"

	cases := []struct {
		Name    string
		Entries []zegEntryJSON
		WantD6a bool
		Notes   int
	}{
		{"必需段·权重段·闭集外 ⇒ D6a",
			[]zegEntryJSON{{Path: "model/w.gguf", MediaType: unknown}}, true, 0},
		{"必需段·引擎段·闭集外 ⇒ D6a",
			[]zegEntryJSON{{Path: "engine/llama-server", MediaType: unknown}}, true, 0},
		{"非必需段·配方段·闭集外 ⇒ D6b（跳过并登记）",
			[]zegEntryJSON{{Path: "recipe.json", MediaType: unknown}}, false, 1},
		{"闭集内（权重段 layer）⇒ 都不触发",
			[]zegEntryJSON{{Path: "model/w.gguf", MediaType: zegMediaLayer}}, false, 0},
		{"未标 mediaType ⇒ 不进 D6a（登记面的事）",
			[]zegEntryJSON{{Path: "model/w.gguf"}}, false, 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			notes, err := hgtMediaTypeGate(&zegIndex{Entries: c.Entries})
			if c.WantD6a {
				if err == nil {
					t.Fatalf("该判 D6a 的段放过了")
				}
				ref, ok := err.(*zegRefusal)
				if !ok || ref.Tier != "D6a" || ref.Msg != wantMsg || ref.Code != wantCode {
					t.Fatalf("D6a 不达标：%v", err)
				}
				if !strings.Contains(ref.Detail, c.Entries[0].Path) {
					t.Fatalf("D6a 细节没点名段：%q", ref.Detail)
				}
				return
			}
			if err != nil {
				t.Fatalf("误伤（该放行却拒）：%v", err)
			}
			if len(notes) != c.Notes {
				t.Fatalf("D6b 登记条数 = %d，要 %d（notes=%v）", len(notes), c.Notes, notes)
			}
			for _, n := range notes {
				if !strings.Contains(n, unknown) {
					t.Fatalf("D6b 登记行没带 mediaType：%q", n)
				}
			}
		})
	}
}

// ─────────────────────────── D10 ───────────────────────────

func TestHgtHalfD10(t *testing.T) {
	wantMsg, wantCode := hgtTierWant("D10")
	dir := t.TempDir()
	egg := filepath.Join(dir, "egg.egg")
	if err := os.WriteFile(egg, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// ① 完工 ⇒ 放行
	if err := hgtHalfRefuse(egg, "egg", true); err != nil {
		t.Fatalf("完工卵被判红（误伤）：%v", err)
	}
	// ② 完工标记未置位 ⇒ D10 / 码 8
	err := hgtHalfRefuse(egg, "egg", false)
	ref, ok := err.(*zegRefusal)
	if !ok || ref.Tier != "D10" || ref.Msg != wantMsg || ref.Code != wantCode {
		t.Fatalf("D10（完工标记）不达标：%v", err)
	}
	if !strings.Contains(ref.Detail, "完工标记") {
		t.Fatalf("D10 细节没说清哪条腿：%q", ref.Detail)
	}
	// ③ 临时件未 rename ⇒ D10（且细节点名那枚临时件）
	part := egg + ".part"
	if err := os.WriteFile(part, []byte("half"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = hgtHalfRefuse(egg, "egg", true)
	ref, ok = err.(*zegRefusal)
	if !ok || ref.Tier != "D10" || ref.Code != wantCode || ref.Msg != wantMsg {
		t.Fatalf("D10（临时件）不达标：%v", err)
	}
	if !strings.Contains(ref.Detail, part) {
		t.Fatalf("D10 细节没点名临时件 %s：%q", part, ref.Detail)
	}
	// ④ 无临时件时 `hgtPartSibling` 不得误报
	if err := os.Remove(part); err != nil {
		t.Fatal(err)
	}
	if p, ok := hgtPartSibling(egg); ok {
		t.Fatalf("临时件已删仍报命中：%s", p)
	}
}

// ─────────────────────────── D11 ───────────────────────────

func TestHgtDiskD11(t *testing.T) {
	wantMsg, wantCode := hgtTierWant("D11")
	small := func(label string, availBlocks uint64) eggstore.Sample {
		return eggstore.Sample{Label: label, Bsize: 4096, Blocks: 1 << 20, Bavail: availBlocks}
	}

	// ① 够用 ⇒ 放行
	ok1 := eggstore.Precheck{Verdict: eggstore.VerdictAllow, ImageFS: small("imagefs", 1<<20), NodeFS: small("nodefs", 1<<20)}
	if err := hgtDiskGate(ok1, 1<<20); err != nil {
		t.Fatalf("盘面够用却拒（误伤）：%v", err)
	}
	// ② 本卵装不下（容量差）⇒ D11 / 码 10，报话按模板填值
	short := eggstore.Precheck{Verdict: eggstore.VerdictAllow, ImageFS: small("imagefs", 4), NodeFS: small("nodefs", 1<<20)}
	err := hgtDiskGate(short, 64<<20)
	if err == nil {
		t.Fatalf("装不下却放行")
	}
	ref, ok := err.(*hfeRefusal)
	if !ok || ref.Tier != "D11" || ref.Code != wantCode || ref.Stage != hfeStageWaterline {
		t.Fatalf("D11 不达标：%+v", err)
	}
	wantLine := fmt.Sprintf(wantMsg, hgtHumanBytes(64<<20-4*4096), hfeListOrNone(nil))
	if ref.Msg != wantLine {
		t.Fatalf("D11 报话不逐字：\n 得到 %q\n 期望 %q", ref.Msg, wantLine)
	}
	// ③ 水线判词 refuse（容量本身够）⇒ 仍归 D11，且报话模板不破
	refuse := eggstore.Precheck{
		Verdict: eggstore.VerdictRefuse, Reasons: []string{"imagefs 已用 90.00% ≥ 高水线 85%"},
		ImageFS: small("imagefs", 1<<20), NodeFS: small("nodefs", 1<<20),
		Reclaimable: []string{"old-egg"},
	}
	err = hgtDiskGate(refuse, 4096)
	ref, ok = err.(*hfeRefusal)
	if !ok || ref.Tier != "D11" || ref.Code != wantCode {
		t.Fatalf("D11（水线）不达标：%v", err)
	}
	if !strings.HasPrefix(ref.Msg, "拒孵：可用磁盘不足（需更多 ") || !strings.Contains(ref.Msg, "old-egg") {
		t.Fatalf("D11（水线）报话模板/清单不达标：%q", ref.Msg)
	}
	if !strings.Contains(ref.Detail, "高水线") {
		t.Fatalf("D11 细节没带水线判词：%q", ref.Detail)
	}
	// ④ 人面字节：三档
	if got := hgtHumanBytes(1536); got != "1.5 KiB" {
		t.Fatalf("hgtHumanBytes(1536) = %q", got)
	}
	if got := hgtHumanBytes(3 << 20); got != "3.0 MiB" {
		t.Fatalf("hgtHumanBytes(3MiB) = %q", got)
	}
	if got := hgtHumanBytes(7); got != "7 B" {
		t.Fatalf("hgtHumanBytes(7) = %q", got)
	}
}

// ─────────────────────────── D12 ───────────────────────────

func TestHgtWallD12(t *testing.T) {
	wantMsg, wantCode := hgtTierWant("D12")
	if wantCode != exitOK {
		t.Fatalf("D12 设计面退码应为 0（不报错）：%d", wantCode)
	}
	saved := hgtMachineWallVersion
	defer func() { hgtMachineWallVersion = saved }()
	set := func(v string, installed bool) {
		hgtMachineWallVersion = func() (string, bool) { return v, installed }
	}
	bundled := func(min string) *zegIndex {
		return &zegIndex{WallMin: min, Entries: []zegEntryJSON{{Path: "wall/zerg-wall"}}}
	}
	bare := func(min string) *zegIndex { return &zegIndex{WallMin: min} }

	cases := []struct {
		Name    string
		Machine string
		Install bool
		Idx     *zegIndex
		Want    string // machine / egg / ""（两份都用不了 ⇒ 出声拒）
		WantErr bool
		WantIn  string
	}{
		{"机器达标且同主号 ⇒ 用机器那份", "1.4.0", true, bundled("1.2.0"), "machine", false, "1.4.0"},
		{"机器低于最低版 ⇒ 用卵内那份", "1.1.0", true, bundled("1.2.0"), "egg", false, "1.1.0"},
		{"主号不同 ⇒ 用卵内那份", "2.0.0", true, bundled("1.2.0"), "egg", false, "主号不同"},
		{"机器未装 ⇒ 用卵内那份", "", false, bundled("1.0.0"), "egg", false, "机器未装"},
		{"机器未装且卵内无茧壁 ⇒ 两份都用不了", "", false, bare("1.0.0"), "", true, "没有"},
		{"卵未声明最低版但机器装了 ⇒ 用机器那份", "1.9.0", true, bare(""), "machine", false, "未声明"},
		{"历史卵（未声明/未装/卵内也没有）⇒ 本档不介入", "", false, bare(""), "", false, "不介入"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			set(c.Machine, c.Install)
			choice, note, err := hgtWallGate(c.Idx)
			if c.WantErr {
				if err == nil {
					t.Fatalf("两份都用不了却给了 choice=%q note=%q", choice, note)
				}
				if !strings.Contains(err.Error(), c.WantIn) {
					t.Fatalf("出声内容没点明缘由：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("不该拒：%v", err)
			}
			if choice != c.Want {
				t.Fatalf("用哪一份 = %q，要 %q（note=%q）", choice, c.Want, note)
			}
			if c.Want == "egg" {
				if !strings.Contains(note, "已有观側面") && !strings.Contains(note, "观测面") {
					t.Fatalf("改用卵内那份时没写观测面标注：%q", note)
				}
				if !strings.Contains(note, "D12") {
					t.Fatalf("注记没挂 D12 档号：%q", note)
				}
			}
			if c.WantIn != "" && !strings.Contains(note, c.WantIn) {
				t.Fatalf("注记里没点明 %s：%q", c.WantIn, note)
			}
			_ = wantMsg
		})
	}

	// 三段判的版本比较器（`v` 前缀 / 两段写法 / 尾巴噪声）
	for _, c := range []struct {
		A, B string
		Less bool
	}{{"1.2.0", "1.2.1", true}, {"1.10.0", "1.9.9", false}, {"v1.2", "1.2.0", false}, {"1.3.0-dirty", "1.3.0", false}} {
		if got := hgtVerLess(c.A, c.B); got != c.Less {
			t.Fatalf("hgtVerLess(%q,%q) = %v，要 %v", c.A, c.B, got, c.Less)
		}
	}
}

// ─────────────────────────── 接线面（孵化入口实跑）───────────────────────────

// hgtHatchStore 建一个只装一枚卵的卵库（名册侧直接 Upsert ⇒ 可造「半成品 / 非 verified」两态）。
func hgtHatchStore(t *testing.T, dir, name, eggPath string, complete bool) *eggstore.Store {
	t.Helper()
	st, err := eggstore.Open(filepath.Join(dir, "store"))
	if err != nil {
		t.Fatalf("开卵库：%v", err)
	}
	sha, err := zegSha256File(eggPath)
	if err != nil {
		t.Fatalf("算卵 sha：%v", err)
	}
	fi, err := os.Stat(eggPath)
	if err != nil {
		t.Fatalf("取卵大小：%v", err)
	}
	if err := st.Roster.Upsert(eggstore.Egg{Name: name, SHA256: sha, Path: eggPath,
		State: eggstore.StateVerified, Bytes: fi.Size(), Complete: complete, Blocks: 1}); err != nil {
		t.Fatalf("登记 %s：%v", name, err)
	}
	return st
}

// hgtPackFrom 打一枚卵（源树由调用方给）。
func hgtPackFrom(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	src := filepath.Join(dir, "src")
	for rel, body := range files {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	egg := filepath.Join(dir, "probe.egg")
	if _, err := zegPack(src, egg, 0, "2026-10-23T00:00:00Z"); err != nil {
		t.Fatalf("打包夹具卵：%v", err)
	}
	return egg
}

// TestHgtHatchWiringTiers —— D5 / D10 / D11 三档在**孵化入口**上实跑：阶段对、码取 13 档表、报话逐字。
func TestHgtHatchWiringTiers(t *testing.T) {
	t.Run("D5/缺项契约段 ⇒ 在校验阶段拒（码 2）", func(t *testing.T) {
		dir := t.TempDir()
		egg := hgtPackFrom(t, dir, map[string]string{
			"contract/contract.json": `{"schema_version":1}`,
			"model/w.gguf":           "weights",
		})
		st := hgtHatchStore(t, dir, "probe", egg, true)
		_, err := hatchFromEgg(hfeOptions{EggPath: egg, EggName: "probe", RefID: "r1",
			Dst: filepath.Join(dir, "exp", "probe.egg"), ImagePath: dir, NodePath: dir,
			Store: st, Stdout: os.Stderr, Workers: 1})
		if err == nil {
			t.Fatalf("缺项契约段竟孵过去了")
		}
		if got := hfeStageOf(err); got != hfeStageVerify {
			t.Fatalf("D5 应在校验阶段拒（verify），实得 %q（%v）", string(got), err)
		}
		ref, ok := err.(*hfeRefusal)
		if !ok || ref.Tier != "D5" || ref.Code != exitUsage {
			t.Fatalf("D5 接线面不达标：%+v", err)
		}
		if rc, ok := hfeExitCode(err); !ok || rc != exitUsage {
			t.Fatalf("hfeExitCode(D5) = (%d,%v)，要 (%d,true)", rc, ok, exitUsage)
		}
		if first := strings.SplitN(ref.Error(), "\n", 2)[0]; first != hgtDesignD5 {
			t.Fatalf("D5 报话第一行不逐字：%q", first)
		}
	})

	t.Run("D10/完工标记未置位 ⇒ 在门阶段拒（码 8）", func(t *testing.T) {
		dir := t.TempDir()
		egg := hgtPackFrom(t, dir, map[string]string{"model/w.gguf": "weights"})
		st := hgtHatchStore(t, dir, "probe", egg, false)
		_, err := hatchFromEgg(hfeOptions{EggPath: egg, EggName: "probe", RefID: "r2",
			Dst: filepath.Join(dir, "exp", "probe.egg"), ImagePath: dir, NodePath: dir,
			Store: st, Stdout: os.Stderr, Workers: 1})
		if err == nil {
			t.Fatalf("半成品竟孵过去了")
		}
		if got := hfeStageOf(err); got != hfeStageState {
			t.Fatalf("D10 应在门阶段拒（state），实得 %q（%v）", string(got), err)
		}
		if ref, ok := err.(*hfeRefusal); !ok || ref.Tier != "D10" || ref.Code != exitBlocked {
			t.Fatalf("D10 接线面不达标：%+v", err)
		}
		if rc, _ := hfeExitCode(err); rc != exitBlocked {
			t.Fatalf("hfeExitCode(D10) = %d，要 %d", rc, exitBlocked)
		}
	})

	t.Run("D11/盘面到水线 ⇒ 在水线阶段拒（码 10）", func(t *testing.T) {
		dir := t.TempDir()
		egg := hgtPackFrom(t, dir, map[string]string{"model/w.gguf": "weights"})
		st := hgtHatchStore(t, dir, "probe", egg, true)
		st.Waterline = &eggstore.Waterline{HighUsedPercent: 0, LowUsedPercent: 0,
			ImageFSAvailPercent: 0, NodeFSAvailPercent: 0, InodesFreePercent: 0}
		_, err := hatchFromEgg(hfeOptions{EggPath: egg, EggName: "probe", RefID: "r3",
			Dst: filepath.Join(dir, "exp", "probe.egg"), ImagePath: dir, NodePath: dir,
			Store: st, Stdout: os.Stderr, Workers: 1})
		if err == nil {
			t.Fatalf("到水线竟孵过去了")
		}
		if got := hfeStageOf(err); got != hfeStageWaterline {
			t.Fatalf("D11 应在水线阶段拒（waterline），实得 %q（%v）", string(got), err)
		}
		if ref, ok := err.(*hfeRefusal); !ok || ref.Tier != "D11" || ref.Code != exitResource {
			t.Fatalf("D11 接线面不达标：%+v", err)
		}
		if rc, _ := hfeExitCode(err); rc != exitResource {
			t.Fatalf("hfeExitCode(D11) = %d，要 %d", rc, exitResource)
		}
	})

	t.Run("D12/主号不同 ⇒ 继续孵，交接件标 wall=egg＋注记", func(t *testing.T) {
		dir := t.TempDir()
		egg := hgtPackFrom(t, dir, map[string]string{
			"model/w.gguf":   "weights",
			"wall/zerg-wall": "wall-bytes",
		})
		st := hgtHatchStore(t, dir, "probe", egg, true)
		saved := hgtMachineWallVersion
		hgtMachineWallVersion = func() (string, bool) { return "2.0.0", true }
		defer func() { hgtMachineWallVersion = saved }()

		// 卵侧声明最低版：v1 布局的段表直接补一格可加面键（✗ 不走写端 —— 写端不置该键）。
		patched := hgtPatchIndexKey(t, dir, egg, "wall_min_version", `"1.4.0"`)
		h, err := hatchFromEgg(hfeOptions{EggPath: patched, EggName: "probe", RefID: "r4",
			Dst: filepath.Join(dir, "exp", "probe.egg"), ImagePath: dir, NodePath: dir,
			Store: st, Stdout: os.Stderr, Workers: 1})
		if err != nil {
			t.Fatalf("D12 不该拒（机器那份不匹配 ⇒ 改用卵内那份，退码 0）：%v", err)
		}
		if h.Wall != "egg" {
			t.Fatalf("交接件 wall = %q，要 egg（改用卵内那份）", h.Wall)
		}
		if !strings.Contains(h.WallNote, "D12") || !strings.Contains(h.WallNote, "观测面") {
			t.Fatalf("交接件 wall_note 没挂档号/标注：%q", h.WallNote)
		}
	})
}

// hgtPatchIndexKey —— 把 v1 卵的索引体再 Marshal 一次（补/改一个顶层键），落成新件（原件不动）。
func hgtPatchIndexKey(t *testing.T, dir, egg, key, rawJSON string) string {
	t.Helper()
	f, err := os.Open(egg)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	hdr := make([]byte, zegHeaderPrefix)
	if _, err := f.ReadAt(hdr, 0); err != nil {
		t.Fatal(err)
	}
	n := int64(0)
	for i := 0; i < 8; i++ {
		n |= int64(hdr[16+i]) << (8 * uint(i))
	}
	body := make([]byte, n)
	if _, err := f.ReadAt(body, zegHeaderPrefix); err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	m[key] = json.RawMessage(rawJSON)
	nb, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	align := int64(zegMinAlign)
	if int64(zegHeaderPrefix)+int64(len(nb)) > align {
		t.Fatalf("补键后索引体 %d B 放不进头块", len(nb))
	}
	blk := make([]byte, align)
	copy(blk[0:4], zegMagic)
	copy(blk[4:8], hdr[4:8])
	copy(blk[8:16], hdr[8:16])
	for i := 0; i < 8; i++ {
		blk[16+i] = byte(uint64(len(nb)) >> (8 * uint(i)))
	}
	copy(blk[zegHeaderPrefix:], nb)
	// 段数据区照原样搬（v1 夹具的段恒从 align 起）
	rest, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "probe-wallmin.egg")
	if err := os.WriteFile(out, append(blk, rest[align:]...), 0o644); err != nil {
		t.Fatal(err)
	}
	return out
}
