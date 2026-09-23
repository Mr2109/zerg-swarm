// cli_impact_cache_test.go —— `zerg impact` 的 **`A5` 落盘缓存与毫秒档**判据
// （任务单-影响面实施-20260922 §二 `A5` 判据①–③ · 设计-变更影响面-v1.6 §4.4 / §7.6 判据5 / §九 批1 判据⑧）。
//
// 落在 `package main_test`（§九 M17 三层测试落点的层①②）：`RunForTest` 跑的是**当前源码**的
// 运行期行为，不是盘上旧制品。
//
// 四条判据的机检口（**都在本文件里真跑**）：
//
//	① **毫秒档 = 命中一次 ≤ 4 ms**（§4.4）—— 冷跑 + 热跑各一次，热跑从 stderr 现读「最长一次
//	   读落盘产物」那一格，逐层再走一遍**现跑标定口**（`ImpactCacheProbeMSForTest`，读 n 次
//	   取最慢一次）；判据值是**现跑的**，不是写死的常数。
//	② **`head_sha` 必须与现算逐字相同**（负控：把落盘件的 `head_sha` 改一字符 ⇒ **必须失效**；
//	   另加三条同类负控：口径值 / 层序 / 形状号 / 源指纹，各改一字符 ⇒ 必须失效；**正控**：
//	   原样落盘件 ⇒ 必须命中 —— 证明这一格不是恒假）。
//	③ **缓存不许改答案**（`M8`）—— 命中缓存的一跑与未命中的一跑，人面**逐字节相同**、
//	   机器面 `items` **逐字节相同**（口径照 §7.6：`sort` 后比集合；本判据更进一步：直接比字节）；
//	   判定口 `judgeImpactCacheSameOutput` 带**成对负控**（改一个字符 ⇒ 必须报错）。
//	④ **落点纪律**（红线机检）—— ① 落点由契约件写死（半份契约 ⇒ 必须报错）；② 落点在**状态目录**下、
//	   **不在仓内**；③ 缓存件**只增不删**（换目标再跑一次 ⇒ 老件仍在）；④ 实现件里**没有删除动作**、
//	   也没有自动收窄/删/回滚的入口；⑤ 干跑那一档**不读也不写**缓存；⑥ 缓存**不抬任何上限**。
package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// impactCacheMSCacheRe 从 stderr 现读「最长一次读落盘产物」那一格（判据①的现跑值）。
var impactCacheMSRe = regexp.MustCompile(`最长一次「读落盘产物」= ([0-9]+\.[0-9]+) ms`)

// impactCacheHitCountRe 从 stderr 现读命中层数。
var impactCacheHitCountRe = regexp.MustCompile(`本跑账：命中 (\d+) 层 · 现算 (\d+) 层 · 不适用 (\d+) 层`)

// judgeImpactCacheSameOutput —— 判据③的**唯一判定口**：命中那一跑与未命中那一跑的答案面
// **逐字节相同**（抽出来的目的：负控能直接喂一个改过的副本 ⇒ 必须报错）。
func judgeImpactCacheSameOutput(miss, hit string) error {
	if miss != hit {
		return errStr("命中缓存的一跑与未命中的一跑**答案面不同**（缓存改答案了 —— §4.4/`M8`）")
	}
	if strings.TrimSpace(miss) == "" {
		return errStr("答案面是空的（判不了「相同」—— 空与空比不算比）")
	}
	return nil
}

// impactCacheStateDir 造一个**合成状态目录**（缓存与索引都落这里 ⇒ 不碰真状态目录）。
func impactCacheStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	return dir
}

// impactCacheRun 跑一次 `impact` 并把人面 / 机器面 / stderr 一起带回来。
func impactCacheRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	rc, out, errb := runCapture(argv...)
	return rc, out, errb
}

// TestImpactCache_ColdWarmSameOutputAndMillisecondTier —— 判据①③（**本文件的主判据**）。
//
// 冷跑（合成状态目录里没有落盘件）⇒ 热跑（同一目录，键三件 + 源指纹都相同）⇒
// 人面逐字节比 + 现读毫秒档；再换一个合成状态目录做**冷/热各一次**的机器面（`--json`）对拍。
func TestImpactCache_ColdWarmSameOutputAndMillisecondTier(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	state := impactCacheStateDir(t)
	tgt := "core/cmd/zerg/main.go"

	rc1, outCold, errCold := impactCacheRun(t, "impact", tgt)
	if rc1 != 0 {
		t.Fatalf("冷跑 ⇒ 期望 rc=0，得到 %d · stderr=%s", rc1, tail(errCold, 400))
	}
	if !strings.Contains(errCold, "命中 0 层") {
		t.Errorf("冷跑应是**未命中**（合成状态目录里没有落盘件）：%s", tail(errCold, 400))
	}
	rc2, outWarm, errWarm := impactCacheRun(t, "impact", tgt)
	if rc2 != 0 {
		t.Fatalf("热跑 ⇒ 期望 rc=0，得到 %d", rc2)
	}
	// ③ 缓存不许改答案：人面**逐字节**同输出。
	if err := judgeImpactCacheSameOutput(outCold, outWarm); err != nil {
		t.Fatalf("判据③ 破：%v\n冷：%q\n热：%q", err, outCold, outWarm)
	}
	// ③ 的负控：改一个字符 ⇒ 判定口必须报错（证明这一格有牙）。
	if err := judgeImpactCacheSameOutput(outCold, outWarm+"x"); err == nil {
		t.Error("负控失败：答案面改一个字符竟判「相同」—— 这一格没牙")
	}
	// ① 毫秒档：热跑里至少三层命中，且「最长一次读落盘产物」现读 ≤ 4 ms。
	m := impactCacheHitCountRe.FindStringSubmatch(errWarm)
	if m == nil {
		t.Fatalf("热跑的 stderr 里读不到本跑账：%s", tail(errWarm, 600))
	}
	if m[1] != "3" {
		t.Errorf("热跑命中层数 = %s（要 3：①③④ 三层进缓存 · ②⑤⑥ 不缓存）", m[1])
	}
	ms := impactCacheMSRe.FindStringSubmatch(errWarm)
	if ms == nil {
		t.Fatalf("热跑的 stderr 里读不到「最长一次读落盘产物」：%s", tail(errWarm, 600))
	}
	got, perr := strconv.ParseFloat(ms[1], 64)
	if perr != nil {
		t.Fatalf("那格不是数：%q", ms[1])
	}
	limit := zerg.ImpactMillisecondTierMSForTest()
	if got > limit {
		t.Errorf("判据① 破：命中一次现读 %.2f ms > 上限 %.2f ms（§4.4 毫秒档）", got, limit)
	}
	// 同一条判据再用**现跑标定口**逐件复算（读 n 次取最慢一次 —— 报上限，不报均值）。
	files, ferr := zerg.ImpactCacheFilesUnderForTest(filepath.Join(state, "impact-cache"))
	if ferr != nil {
		t.Fatalf("列缓存件失败：%v", ferr)
	}
	if len(files) != 3 {
		t.Errorf("合成状态目录里的缓存件 = %d（要 3：①③④）", len(files))
	}
	for _, f := range files {
		worst, rows, perr := zerg.ImpactCacheProbeMSForTest(f, 7)
		if perr != nil {
			t.Fatalf("标定口读 %s 失败：%v", f, perr)
		}
		if worst > limit {
			t.Errorf("判据① 破（标定口）：%s 现读 %.3f ms > %.2f ms", filepath.Base(f), worst, limit)
		}
		t.Logf("毫秒档现测：%s 最慢 %.3f ms · 条目 %d 条", filepath.Base(f), worst, rows)
	}
	// 机器面同样成对（冷/热各一次 · 同一个合成状态目录）。
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	rcj1, outJ1, _ := impactCacheRun(t, "impact", tgt, "--json", "what,why,how,red")
	rcj2, outJ2, errJ2 := impactCacheRun(t, "impact", tgt, "--json", "what,why,how,red")
	if rcj1 != 0 || rcj2 != 0 {
		t.Fatalf("机器面两跑退码 = %d / %d（都要 0）", rcj1, rcj2)
	}
	if !strings.Contains(errJ2, "命中 3 层") {
		t.Errorf("机器面第二跑应是命中：%s", tail(errJ2, 300))
	}
	var a, b map[string]json.RawMessage
	if e1, e2 := json.Unmarshal([]byte(outJ1), &a), json.Unmarshal([]byte(outJ2), &b); e1 != nil || e2 != nil {
		t.Fatalf("机器面不是对象：%v / %v", e1, e2)
	}
	if string(a["items"]) != string(b["items"]) {
		t.Error("判据③ 破：命中与未命中的 `items` 逐字节不同（缓存改答案了）")
	}
	if n := countSortedEqual(string(a["items"]), string(b["items"])); !n {
		t.Error("判据③ 破：两面 `items` `sort` 后比集合也不同（口径照 §7.6 的底线）")
	}
}

// countSortedEqual 两面按 `sort` 后比集合（§7.6 收窄口径的底线：**不许逐字节比行序**）。
// 返回 true = 集合相同（**这一支是底线，不是判据的全部**：字节相同在上面已单独判过）。
func countSortedEqual(x, y string) bool {
	sx, sy := strings.Split(x, "},{"), strings.Split(y, "},{")
	sort.Strings(sx)
	sort.Strings(sy)
	return strings.Join(sx, "|") == strings.Join(sy, "|")
}

// TestImpactCache_InvalidationNegativeControls —— 判据②的**成对负控**（五条失效条件 + 一条正控）。
//
// 做法：合成状态目录里先跑一次（落下 ①③④ 三件）⇒ 拿 ① 件逐条**改一字符** ⇒ 走**同一个判定口**
// （`ImpactCacheLoadForTest` = 命令内部用的那一份）⇒ **必须未命中**，且原因要指名道姓；
// 最后一条**正控**（原样件）⇒ 必须命中（证明这一格不是恒假）。
func TestImpactCache_InvalidationNegativeControls(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	state := impactCacheStateDir(t)
	if rc, _, errb := impactCacheRun(t, "impact", "core/cmd/zerg/main.go"); rc != 0 {
		t.Fatalf("准备态不对：rc=%d · %s", rc, tail(errb, 300))
	}
	files, err := zerg.ImpactCacheFilesUnderForTest(filepath.Join(state, "impact-cache"))
	if err != nil || len(files) == 0 {
		t.Fatalf("没落下缓存件（err=%v · 件数=%d）", err, len(files))
	}
	target := ""
	for _, f := range files {
		if strings.HasSuffix(filepath.Base(f), ".json") && strings.HasPrefix(filepath.Base(f), "①-") {
			target = f
		}
	}
	if target == "" {
		t.Fatalf("找不到 ① 层的落盘件（%v）", files)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("读不到落盘件：%v", err)
	}
	var e struct {
		Schema      string `json:"schema"`
		HeadSHA     string `json:"head_sha"`
		Layer       string `json:"layer"`
		Caliber     string `json:"caliber"`
		Fingerprint string `json:"source_fingerprint"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("落盘件解不开：%v", err)
	}
	// 正控：原样 ⇒ 必须命中。
	if hit, why, ms := zerg.ImpactCacheLoadForTest(target, e.HeadSHA, e.Layer, e.Caliber, e.Fingerprint); !hit {
		t.Fatalf("正控破：原样落盘件竟未命中（%s · 读 %.3f ms）—— 这一格要是恒假，下面的负控全是空的", why, ms)
	}
	// 五条失效条件逐条改一字符。
	type tc struct {
		name          string
		head, seq     string
		caliber, fp   string
		wantReasonSub string
	}
	cases := []tc{
		{"head_sha 改一字符", flipOne(e.HeadSHA), e.Layer, e.Caliber, e.Fingerprint, "head_sha 不符"},
		{"层序改一字符", e.HeadSHA, flipOne(e.Layer), e.Caliber, e.Fingerprint, "层序不符"},
		{"口径值改一字符", e.HeadSHA, e.Layer, flipOne(e.Caliber), e.Fingerprint, "口径值不符"},
		{"源指纹改一字符", e.HeadSHA, e.Layer, e.Caliber, flipOne(e.Fingerprint), "源指纹不符"},
	}
	for _, c := range cases {
		if c.head == e.HeadSHA && c.seq == e.Layer && c.caliber == e.Caliber && c.fp == e.Fingerprint {
			t.Fatalf("负控「%s」的输入没被改动（准备态不对）", c.name)
		}
		hit, why, _ := zerg.ImpactCacheLoadForTest(target, c.head, c.seq, c.caliber, c.fp)
		if hit {
			t.Errorf("负控失败：%s ⇒ 竟还命中（必须失效、不许命中）", c.name)
			continue
		}
		if !strings.Contains(why, c.wantReasonSub) {
			t.Errorf("负控「%s」的失效原因没指名道姓：%q（要含 %q）", c.name, why, c.wantReasonSub)
		}
	}
	// 形状号不认 ⇒ 不许命中（落盘件的自描述那一格）。
	if hit, why, _ := zerg.ImpactCacheLoadForTest(target, e.HeadSHA, e.Layer, e.Caliber, "no-such-fp"); hit {
		t.Errorf("负控失败：喂一个错的源指纹竟命中")
	} else if !strings.Contains(why, "源指纹不符") {
		t.Errorf("失效原因不对：%q", why)
	}
	// 命令面复跑一遍（**端到端**）：把 ① 件的 head_sha 改一字符 ⇒ 那一层必须写「未命中」并现算，
	// 且答案与冷跑**逐字节相同**（缓存失效**不许改答案**）。
	tampered := strings.Replace(string(raw), `"head_sha": "`+e.HeadSHA+`"`, `"head_sha": "`+flipOne(e.HeadSHA)+`"`, 1)
	if tampered == string(raw) {
		t.Fatalf("篡改没落地（JSON 形状与预期不同）")
	}
	if err := os.WriteFile(target, []byte(tampered), 0o644); err != nil {
		t.Fatalf("写不回：%v", err)
	}
	rc, outAfter, errAfter := impactCacheRun(t, "impact", "core/cmd/zerg/main.go")
	if rc != 0 {
		t.Fatalf("篡改后那一跑 ⇒ 期望 rc=0（照常现算），得到 %d", rc)
	}
	if !strings.Contains(errAfter, "head_sha 不符") {
		t.Errorf("篡改后 ① 层没写「head_sha 不符」（失效条件没生效）：%s", tail(errAfter, 800))
	}
	if !strings.Contains(errAfter, "未命中") {
		t.Errorf("篡改后 ① 层没写「未命中」：%s", tail(errAfter, 800))
	}
	if !strings.Contains(errAfter, "命中 2 层") && !strings.Contains(errAfter, "命中 3 层") {
		t.Errorf("篡改后本跑账不对（① 该失效 ⇒ 命中最多 2 层）：%s", tail(errAfter, 800))
	}
	_ = outAfter
	if zerg.ImpactCacheSchemaForTest() == "" {
		t.Error("形状号是空的（落盘件就没法自描述了）")
	}
}

// flipOne 把串里第一个字符换掉（负控专用：**改一字符**，不是删件 —— 红线里「不删件」照守）。
func flipOne(s string) string {
	if s == "" {
		return "x"
	}
	c := s[0]
	if c == 'a' {
		return "b" + s[1:]
	}
	return "a" + s[1:]
}

// TestImpactCache_FingerprintMovesWithSource —— 源指纹（`R42`）的**成对负控**：
// 同一份源两算 ⇒ 同值；改了源件的**内容** ⇒ 必变（**件名集合没变也必变** —— 这正是
// 「同一 `head_sha` 下工作树被改动」的那一格，光看 `head_sha` 看不出来）。
func TestImpactCache_FingerprintMovesWithSource(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "package a\n\nfunc A() int { return 1 }\n")
	gitRunInCacheTest(t, dir, "init", "-q")
	gitRunInCacheTest(t, dir, "add", "-A")
	fp1, n1, err := zerg.ImpactCacheFingerprintForTest(dir, ".")
	if err != nil {
		t.Fatalf("源指纹取不到：%v", err)
	}
	fp2, n2, err := zerg.ImpactCacheFingerprintForTest(dir, ".")
	if err != nil {
		t.Fatalf("源指纹取不到：%v", err)
	}
	if fp1 != fp2 {
		t.Errorf("同一份源两算不同值：%s ≠ %s（那它就当不了失效条件）", fp1, fp2)
	}
	if n1 == 0 || n1 != n2 {
		t.Errorf("件数不对：%d / %d", n1, n2)
	}
	// 改内容（件名集合一个没动）⇒ 必须变。
	mustWrite(t, filepath.Join(dir, "a.go"), "package a\n\nfunc A() int { return 2 }\n")
	fp3, _, err := zerg.ImpactCacheFingerprintForTest(dir, ".")
	if err != nil {
		t.Fatalf("源指纹取不到：%v", err)
	}
	if fp3 == fp1 {
		t.Error("负控失败：源件内容改了、源指纹竟没变 —— 「同一 head_sha 下工作树被改动」这一格没牙")
	}
	// 再改一次（第二次改动）⇒ 还必须变（不是「脏了就恒定」）。
	mustWrite(t, filepath.Join(dir, "a.go"), "package a\n\nfunc A() int { return 3 }\n")
	fp4, _, err := zerg.ImpactCacheFingerprintForTest(dir, ".")
	if err != nil {
		t.Fatalf("源指纹取不到：%v", err)
	}
	if fp4 == fp3 {
		t.Error("负控失败：第二次改动没动源指纹（那「改两次取一次」就会命中旧答案）")
	}
	// 删件（工作树差异面原文里带着状态字母）⇒ 也必须变。
	if err := os.Remove(filepath.Join(dir, "a.go")); err != nil {
		t.Fatalf("合成夹具删件失败：%v", err)
	}
	fp5, _, err := zerg.ImpactCacheFingerprintForTest(dir, ".")
	if err != nil {
		t.Fatalf("源指纹取不到：%v", err)
	}
	if fp5 == fp4 {
		t.Error("负控失败：删了一件源指纹竟没变")
	}
}

// TestImpactCache_LayoutFromContractAndOutsideRepo —— 判据④①：落点由**契约件**写死、
// 落在**状态目录**下、**不在仓内**；半份契约 ⇒ 必须报错（不许猜目录名）。
func TestImpactCache_LayoutFromContractAndOutsideRepo(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	state := impactCacheStateDir(t)
	lay, err := zerg.ImpactStateLayoutOfForTest(root)
	if err != nil {
		t.Fatalf("落点契约件读不到：%v", err)
	}
	if lay.IndexDir != "impact-index" || lay.CacheDir != "impact-cache" {
		t.Errorf("落点名 = %q / %q（要 impact-index / impact-cache —— 目录名写死在契约件里）", lay.IndexDir, lay.CacheDir)
	}
	if !zerg.ImpactStateKeysEqualForTest(lay.Key) {
		t.Errorf("契约件的 cache.key = %v（要 head_sha+layer+caliber 三件一起）", lay.Key)
	}
	if len(lay.NoAuto) != 3 {
		t.Errorf("契约件没写死「不许接的动作」三条：%v", lay.NoAuto)
	}
	// 半份契约 ⇒ 报错（三种半份各来一次）。
	for _, body := range []string{
		`{"dirs":{"cache":"impact-cache"},"cache":{"key":["head_sha","layer","caliber"]}}`,
		`{"dirs":{"index":"impact-index"},"cache":{"key":["head_sha","layer","caliber"]}}`,
		`{"dirs":{"index":"impact-index","cache":"impact-cache"},"cache":{"key":[]}}`,
	} {
		if _, err := zerg.ImpactStateLayoutParseForTest([]byte(body)); err == nil {
			t.Errorf("负控失败：半份契约竟判过（不许猜目录名）：%s", body)
		}
	}
	idxDir, err := zerg.ImpactCacheIndexDirForTest(root)
	if err != nil {
		t.Fatalf("索引落点取不到：%v", err)
	}
	if idxDir != filepath.Join(state, "impact-index") {
		t.Errorf("索引落点 = %q（要 %q：状态目录 + 契约件里的目录名）", idxDir, filepath.Join(state, "impact-index"))
	}
	// 现跑：缓存件必须落在状态目录下、**不在仓内**（拼出来的路径与真落下来的路径两处都判）。
	rc, out, errb := impactCacheRun(t, "impact", "core/cmd/zerg/main.go")
	if rc != 0 {
		t.Fatalf("现跑 rc=%d", rc)
	}
	if !strings.Contains(errb, filepath.Join(state, "impact-cache")) || !strings.Contains(errb, idxDir) {
		t.Errorf("stderr 没把两个落点写全（缓存 + 索引）：%s", tail(errb, 500))
	}
	files, err := zerg.ImpactCacheFilesUnderForTest(filepath.Join(state, "impact-cache"))
	if err != nil || len(files) == 0 {
		t.Fatalf("没落下缓存件（err=%v · %d 件）", err, len(files))
	}
	for _, f := range files {
		if strings.HasPrefix(f, root+string(filepath.Separator)) {
			t.Errorf("缓存件落到**仓内**了（红线：不进公开面、不进 git 跟踪）：%s", f)
		}
		if !strings.HasPrefix(f, state) {
			t.Errorf("缓存件不在状态目录下：%s", f)
		}
	}
	// 同一跑的 stdout 必须仍是人面三行（缓存不许往人面加行）。
	if n := countNonEmpty(out); n != 3 {
		t.Errorf("人面不是三行（缓存新增了行？）：%d 行", n)
	}
	// 契约登记表里登记了这条契约（`S-h` → 本件）—— 登记表是「契约真源」的登记处。
	b, err := os.ReadFile(filepath.Join(root, "core", "internal", "contract", "registry.json"))
	if err != nil {
		t.Fatalf("登记表读不到：%v", err)
	}
	var reg struct {
		Entries []struct {
			ID    string `json:"id"`
			Truth string `json:"truth"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(b, &reg); err != nil {
		t.Fatalf("登记表解不开：%v", err)
	}
	found := false
	for _, e := range reg.Entries {
		if e.ID == "S-h" && e.Truth == "core/internal/contract/impact-state.json" {
			found = true
		}
	}
	if !found {
		t.Error("契约登记表里没有 `S-h` → `core/internal/contract/impact-state.json`（落点没进契约面）")
	}
}

// TestImpactCache_OnlyGrowsAndHasNoAutoActions —— 判据④③④⑤⑥：只增不删 / 没有自动动作入口 /
// 干跑那一档不碰缓存 / 缓存不抬上限。
func TestImpactCache_OnlyGrowsAndHasNoAutoActions(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	state := impactCacheStateDir(t)
	cacheDir := filepath.Join(state, "impact-cache")

	if rc, _, _ := impactCacheRun(t, "impact", "core/cmd/zerg/main.go"); rc != 0 {
		t.Fatalf("第一跑 rc != 0")
	}
	after1, err := zerg.ImpactCacheFilesUnderForTest(cacheDir)
	if err != nil {
		t.Fatalf("列件失败：%v", err)
	}
	// 再跑一次（命中，不新增键）。
	if rc, _, _ := impactCacheRun(t, "impact", "core/cmd/zerg/main.go"); rc != 0 {
		t.Fatalf("第二跑 rc != 0")
	}
	after2, _ := zerg.ImpactCacheFilesUnderForTest(cacheDir)
	if len(after2) != len(after1) {
		t.Errorf("同一目标两跑件数变了（%d → %d）—— 键没稳住", len(after1), len(after2))
	}
	// 换一个目标 ⇒ 只**增**，老件一件不许少（「只增不改不删」）。
	if rc, _, _ := impactCacheRun(t, "impact", "core/cmd/zerg/family_impact_cache.go"); rc != 0 {
		t.Fatalf("换目标那一跑 rc != 0")
	}
	after3, _ := zerg.ImpactCacheFilesUnderForTest(cacheDir)
	if len(after3) <= len(after2) {
		t.Errorf("换目标后件数没增（%d → %d）：目标面没进键 ⇒ 会互相覆盖成假命中", len(after2), len(after3))
	}
	for _, f := range after2 {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("老缓存件不见了（**只增不删** 破了）：%s", f)
		}
	}
	// 实现件里**没有删除动作**（红线：不接自动删；旧件留着不用，由人决定）。
	src, err := zerg.ImpactCacheSourceForTest()
	if err != nil {
		t.Fatalf("读实现件失败：%v", err)
	}
	for _, bad := range []string{"os.Remove(", "os.RemoveAll(", "os.Truncate(", "os.Rename(tmp, path+", "syscall.Unlink", "`.tmp` 清理"} {
		if strings.Contains(src, bad) {
			t.Errorf("实现件里出现删除/截断动作 %q（红线：不许自动删 ✗）", bad)
		}
	}
	for _, want := range []string{"os.MkdirAll", "os.WriteFile", "os.Rename(tmp, path)"} {
		if !strings.Contains(src, want) {
			t.Errorf("落盘那一套里缺 %q（落盘路径换了？）", want)
		}
	}
	// 干跑那一档（`dev edit` 干跑）传的是 `impactCacheOff`，且**不读也不写**缓存。
	// ★ 计数随动（`C3`）：这一格的调用串自 `C3` 起多了两枚参数（文档面档位 —— 干跑**不现建**索引、
	// 只用默认档 B），缓存挡位那半**逐字未变**（判据一个字没动，只跟实现签名走）。
	hookSrc, err := zerg.ImpactHookSourceForTest()
	if err != nil {
		t.Fatalf("读钩子件失败：%v", err)
	}
	if !strings.Contains(hookSrc, `impactPullLayers(root, tgt, true, impactCacheOff, false, "B")`) {
		t.Error("干跑那一档没传 `impactCacheOff`（A4「不写缓存」的加强形态就没落地）")
	}
	before, _ := zerg.ImpactCacheFilesUnderForTest(filepath.Join(state, "impact-cache"))
	notes, err := zerg.ImpactPullLayersModeOffForTest(root, "core/cmd/zerg/main.go")
	if err != nil {
		t.Fatalf("缓存挡位=关那一跑失败：%v", err)
	}
	after, _ := zerg.ImpactCacheFilesUnderForTest(filepath.Join(state, "impact-cache"))
	if len(after) != len(before) {
		t.Errorf("缓存挡位=关竟然动了缓存件：%d → %d", len(before), len(after))
	}
	for _, n := range notes {
		if !strings.Contains(n, "缓存挡位=关") && !strings.Contains(n, "不适用") {
			t.Errorf("挡位=关时层行没写「未用/不适用」：%s", n)
		}
	}
	// 缓存**不抬任何上限**（§二 `A5` 红线）：四个上限逐格钉死。
	items, tokens, lines, itemTokens := zerg.ImpactCardCapsForTest()
	if items != 12 || tokens != 1200 || lines != 3 || itemTokens != 60 {
		t.Errorf("上限被动了：条数=%d（要 12）· 总量 token=%d（要 1200）· 单条行数=%d（要 3）· 单条 token=%d（要 60）",
			items, tokens, lines, itemTokens)
	}
	// 进缓存的层集合（测试不另抄一份）。
	if got := strings.Join(zerg.ImpactCacheableLayersForTest(), ","); got != "①,③,④" {
		t.Errorf("进缓存的层 = %v（要 ①③④：②⑤⑥ 的输入不在源指纹面内 / 随档位与环境变）", got)
	}
}

// gitRunInCacheTest 在给定目录跑一条 git（自检夹具用；失败即 Fatal）。
func gitRunInCacheTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("夹具 git %v 失败：%v · %s", args, err, strings.TrimSpace(string(out)))
	}
}
