// family_impact_cache.go —— `A5` **落盘缓存与毫秒档**（变更影响面 · `zerg impact`）。
//
// 依据（逐字）：`设计-变更影响面-v1.6` §4.4（缓存口径：缓存是**落盘产物**（不是内存缓存），
// 键 = **`head_sha` + 层 + 该层口径值** 三者一起；**`head_sha` 一变，缓存即失效** ⇒ 不许拿旧缓存
// 装新答案）；同节分层预算表第一行（**毫秒档 = 命中一次 ≤ 4 ms** · 本机现测上限 `3.52 ms` ·
// 口径 = **读落盘产物 + 切分去重成集合** · 三档现测 meta json 0.02–0.03 ms / 小件 0.03–0.08 ms /
// 全量 39,361,052 B 2.03–3.52 ms）；§7.6 判据 5 + §十 `R42`（**源指纹**：`head_sha` 相同 ≠ 源件相同
// —— 两跑之间源指纹不同 ⇒ 明写「仓变了，结果不可比」）；§九 批 1 判据⑧（`M8`：**命中缓存的一跑与
// 未命中的一跑，结果也必须相同** —— 缓存**不许改答案**）；任务单-影响面实施-20260922 §二 `A5`
// （判据①–③ · 产物 `<状态目录>/impact-cache/` + 失效判据的自检件）。
//
// 三条纪律（本件的判据都落在它们上面）：
//
//	① **键三者一起**：`head_sha` + 层序 + 该层口径值（口径值**逐字**比，不做模糊比）；
//	② **失效条件只许加严**：本批在键之外多一条 `R42` 源指纹（不同即失效）—— 它是**更严**的失效
//	   条件，不是放宽（同一 `head_sha` 下工作树被改动时 `head_sha` 不变 ⇒ 只看 `head_sha`
//	   会把旧答案装成新答案）；
//	③ **只增不改不删、不接任何自动动作**：命中与未命中的**答案面逐字同输出**；本件里没有
//	   `os.Remove` 这类调用（同键重写是「写临时件 + 改名」）；**不接**自动收窄 / 自动删 / 自动回滚
//	   （任务单 §二 `A5` 红线 · 开工令「不许我自行决定」第 6 条）。
//
// 落点（**目录名从契约件读** · 源码里不另写第二份）：`core/internal/contract/impact-state.json`
// 的 `dirs.cache` / `dirs.index`；两级都在 `<状态目录>` 下（`ZERG_STATE_DIR` > `~/.zerg/state`，
// 与 `stateDirOf()` 同口径）⇒ **不在仓内** ⇒ 不进公开面、不进 git 跟踪。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// impactStateContractRel —— 落点契约件（**唯一真源**：目录名 / 缓存键 / 失效条件都读它）。
const impactStateContractRel = "core/internal/contract/impact-state.json"

// impactCacheSchema —— 缓存件的**形状号**（自描述：读到不认的形状号 ⇒ 不许命中）。
const impactCacheSchema = "zerg/impact-cache/1"

// impactMillisecondTierMS —— §4.4「毫秒档 = 命中一次 ≤ 4 ms」这一格（**判据值**，不是估的）。
// 本机现测上限 `3.52 ms`（四组第4组 `4-R32`）；本批（`A5`）**现跑重测**，标定值见回执。
const impactMillisecondTierMS = 4.0

// impactCacheMode —— 缓存**两档**（判据「缓存不许改答案」需要一条**能真关掉缓存**的对照路径）。
type impactCacheMode int

const (
	// impactCacheOff **完全不碰缓存**（不读也不写）—— `dev edit` 干跑那一档用它：
	// `A4` 已把干跑定成零副作用（「不写缓存」），本批把那条**加强**成「连读也不做」
	// ⇒ 干跑与缓存完全无关（`A4` 的判据面一个字不动）。
	impactCacheOff impactCacheMode = iota
	// impactCacheOn 读 + 写（`zerg impact` 的默认档）。
	impactCacheOn
)

// impactCacheableLayers —— 进缓存的层（**三层的输入都在源指纹面内**）。
//
// 不进缓存的三层与理由（写进契约件 `cache.not_cacheable`，这里只说结论）：
//
//	② 符号层：产物取决于**档位**（默认档未跑 / 按需档现算），档位不在键的三件里 ⇒
//	   要缓存它就得往键里加第四件（越线）⇒ 本批不缓存；
//	⑤ 语义层：在位判据与索引目录**随环境变**（模型/索引在不在盘上不由 `head_sha` 决定）；
//	⑥ 公开面：输入含**仓外**产出树（不在源指纹面内）。
//
// ⇒ 三条都是「宁少报不猜报」：不缓存只损失速度，缓存错了是**拿旧答案装新答案**。
var impactCacheableLayers = map[string]bool{"①": true, "③": true, "④": true}

// impactStateLayout —— 契约件里的落点（目录名 + 键的三件 + 不许接的动作）。
type impactStateLayout struct {
	Schema      string
	IndexDir    string
	CacheDir    string
	Key         []string
	NoAuto      []string
	FromRelPath string
}

// impactStateLayoutOf 读契约件（**只读** · 这一份是唯一真源）。
// 半份契约（`dirs.index`/`dirs.cache`/`cache.key` 任一为空）⇒ 报错，不当「没有」。
func impactStateLayoutOf(root string) (impactStateLayout, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactStateContractRel)))
	if err != nil {
		return impactStateLayout{}, err
	}
	return impactStateLayoutParse(b)
}

// impactStateLayoutParse 解契约件正文（抽出来：自检件能直接喂**半份**契约做负控）。
func impactStateLayoutParse(b []byte) (impactStateLayout, error) {
	var doc struct {
		Schema string `json:"schema"`
		Dirs   struct {
			Index string `json:"index"`
			Cache string `json:"cache"`
		} `json:"dirs"`
		Cache struct {
			Key    []string `json:"key"`
			NoAuto []string `json:"not_auto_actions"`
		} `json:"cache"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return impactStateLayout{}, err
	}
	lay := impactStateLayout{Schema: doc.Schema, IndexDir: doc.Dirs.Index, CacheDir: doc.Dirs.Cache,
		Key: doc.Cache.Key, NoAuto: doc.Cache.NoAuto, FromRelPath: impactStateContractRel}
	if strings.TrimSpace(lay.IndexDir) == "" || strings.TrimSpace(lay.CacheDir) == "" || len(lay.Key) == 0 {
		return impactStateLayout{}, fmt.Errorf("%s 是半份契约（dirs.index / dirs.cache / cache.key 有空格）"+
			"⇒ 取不了落点（不当「没有落点」）", impactStateContractRel)
	}
	return lay, nil
}

// impactStateKeysEqual 键的三件**逐字**对拍（判据①的判定口 · 纯函数 ⇒ 自检件能直接喂坏期望）。
func impactStateKeysEqual(key []string) bool {
	want := []string{"head_sha", "layer", "caliber"}
	if len(key) != len(want) {
		return false
	}
	for i := range want {
		if key[i] != want[i] {
			return false
		}
	}
	return true
}

// ---- 源指纹（`R42` · 本批**加严**口径）-------------------------------------------------------

// impactCacheFingerprint —— 本批的**源指纹**（口径见本文件头 §② 与契约件 `cache.source_fingerprint`）。
//
// 件集合 = 该 module 的全部 `.go` 件（tracked + untracked）∪ 工作树里所有与 `HEAD` 不一致的件
// （任意后缀 · 含未跟踪）；逐件内容 `sha256` 与 `git status --porcelain -z` 原文一起汇总，
// 取 `sha256` 前 16 位（与 §7.6 判据 5 现测 `d0855fdfe71b94d2` **同形**：16 位十六进制）。
//
// 为什么比设计那一版**严**：设计只汇总 module 的 `.go`（`4-H5` 现测 639 件），而本层的输入里
// 还有**非 `.go` 件**（③ 读 `registry.json` · ④ 扫 tracked 的任意后缀 · ① 读 `go.mod`/`go.sum`）
// ⇒ 只汇总 `.go` 会漏。加严**只多失效**、不少失效 ⇒ 判据面一格没放宽。
//
// 返回的第二个值是进指纹的件数（人面报数用；口径不同不相加）。
func impactCacheFingerprint(root, scopeRel string) (string, int, error) {
	scope := strings.TrimSpace(scopeRel)
	if scope == "" {
		scope = "."
	}
	files := map[string]bool{}
	out, _, code, err := impactRunIn(root, "git", "ls-files", "-z", "--", scope)
	if err != nil || code != 0 {
		return "", 0, fmt.Errorf("`git ls-files` 起不来或退码 %d ⇒ 取不了源指纹（不给结论）", code)
	}
	for _, p := range strings.Split(out, "\x00") {
		if strings.HasSuffix(p, ".go") {
			files[p] = true
		}
	}
	// 工作树差异面：`-z` 是 `XY <路径>\0`（改名/复制多一位源路径）——原样进指纹（状态字母也进）。
	st, _, code, err := impactRunIn(root, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || code != 0 {
		return "", 0, fmt.Errorf("`git status` 起不来或退码 %d ⇒ 取不了源指纹（不给结论）", code)
	}
	toks := strings.Split(st, "\x00")
	for i := 0; i < len(toks); i++ {
		e := toks[i]
		if len(e) < 4 {
			continue
		}
		if c := e[0]; c == 'R' || c == 'C' {
			i++ // 改名/复制：下一位是源路径，一起进集合
			continue
		}
		if p := strings.TrimSpace(e[3:]); p != "" {
			files[p] = true
		}
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	h.Write([]byte(st)) // 差异面原文（删件的状态字母就在这里 ⇒ 删件也改指纹）
	n := 0
	for _, p := range paths {
		b, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if rerr != nil {
			continue // 已删件 / 读不到：状态原文那一面已经覆盖它
		}
		fh := sha256.Sum256(b)
		fmt.Fprintf(h, "%s %s\n", p, hex.EncodeToString(fh[:]))
		n++
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:])[:16], n, nil
}

// impactFingerprintScope —— 源指纹的**扫描面**：件目标取它所属 module；契约目标取整仓（`.`）。
func impactFingerprintScope(root string, tgt *impactTarget) string {
	if tgt == nil || tgt.Kind != impactKindFile {
		return "."
	}
	if dir, _ := impactModuleDir(root, tgt.Rel); dir != "" {
		return dir
	}
	return "."
}

// ---- 落盘件的读写 ---------------------------------------------------------------------------

// impactCacheEntry —— 一件落盘缓存（自描述：形状号 + 键三件 + **层名/粒度** + 源指纹 + 该层产物 +
// 落盘时刻与耗时）。存层名与粒度是为了**命中时不重拼**：命中就把这一层的产物原样装回
// （`M8`：命中与未命中必须逐字同输出）。
type impactCacheEntry struct {
	Schema      string              `json:"schema"`
	HeadSHA     string              `json:"head_sha"`
	Layer       string              `json:"layer"`
	LayerName   string              `json:"layer_name"`
	Grane       string              `json:"granularity"`
	Caliber     string              `json:"caliber"`
	Fingerprint string              `json:"source_fingerprint"`
	Status      string              `json:"status"`
	Detail      string              `json:"detail"`
	Rows        []map[string]string `json:"rows"`
	BuiltAt     string              `json:"built_at"`
	BuiltCostMS int64               `json:"built_cost_ms"`
}

// impactCacheFileFor —— 落盘件路径：
// `<状态目录>/<dirs.cache>/<head_sha>/<层序>-<sha256(口径值 + 目标面) 前 16>.json`。
// 目录名来自契约件；状态目录来自 `stateDirOf()`（`ZERG_STATE_DIR` > `~/.zerg/state`）。
func impactCacheFileFor(layout impactStateLayout, head, seq, caliber, scopeKey string) string {
	sum := sha256.Sum256([]byte(caliber + "\x1f" + scopeKey))
	return filepath.Join(stateDirOf(), layout.CacheDir, head, seq+"-"+hex.EncodeToString(sum[:])[:16]+".json")
}

// impactCacheIndexDir —— 第 ⑤ 层索引的落点（同样来自契约件）。
func impactCacheIndexDir(layout impactStateLayout) string {
	return filepath.Join(stateDirOf(), layout.IndexDir)
}

// impactCacheLoad —— 读一件落盘缓存并按**四条失效条件**逐条判（判据②的判定口 · 纯文件读 + 逐条比）。
//
// 返回 `(件, 命中吗, 没命中的原因, 读耗时)`。读耗时口径 = §4.4 的「**读落盘产物 + 切分去重成集合**」
// （`rows` 落盘时就已是四字段条目 ⇒ 解回来即为「切分」后的形态）。
// **任一条不符即未命中** ⇒ 走现算（不是「凑合用」）。
func impactCacheLoad(path string, lay impactLayer, fp string) (impactCacheEntry, bool, string, time.Duration) {
	t0 := time.Now()
	b, err := os.ReadFile(path)
	if err != nil {
		return impactCacheEntry{}, false, "无落盘件", time.Since(t0)
	}
	var e impactCacheEntry
	if uerr := json.Unmarshal(b, &e); uerr != nil {
		return impactCacheEntry{}, false, "落盘件解不开（" + uerr.Error() + "）", time.Since(t0)
	}
	fail := ""
	switch {
	case e.Schema != impactCacheSchema:
		fail = "形状号不认（" + dashIfEmpty(e.Schema) + " ≠ " + impactCacheSchema + "）"
	case e.HeadSHA != lay.HeadSHA:
		fail = "head_sha 不符（落盘 " + shortHex(e.HeadSHA) + " ≠ 现算 " + shortHex(lay.HeadSHA) + "）"
	case e.Layer != lay.Seq:
		fail = "层序不符（落盘 " + dashIfEmpty(e.Layer) + " ≠ 现算 " + lay.Seq + "）"
	case e.Caliber != lay.Caliber:
		fail = "该层口径值不符（口径值变 ⇒ 这一层的键变了）"
	case e.Fingerprint != fp:
		fail = "源指纹不符（`head_sha` 相同 ≠ 源件相同 ⇒ 仓变了，结果不可比 · `R42`）"
	case strings.TrimSpace(e.Status) == "" || e.Status == "未跑":
		fail = "落盘件的状态不可缓存（" + dashIfEmpty(e.Status) + "）"
	}
	if fail != "" {
		return impactCacheEntry{}, false, fail, time.Since(t0)
	}
	return e, true, "", time.Since(t0)
}

// impactCacheStore —— 落一件缓存（**只增不改不删**：写临时件 + 改名；没有删除动作）。
//
// 落点是**状态目录**（不在仓内）：建目录是 `os.MkdirAll`，不是「造仓内的目录」。
// 落盘失败**不阻断**命令（缓存是加速件，不是判据件）—— 但要把失败原因原样带回人面。
func impactCacheStore(path string, lay impactLayer, fp string, cost time.Duration) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	e := impactCacheEntry{
		Schema: impactCacheSchema, HeadSHA: lay.HeadSHA, Layer: lay.Seq,
		LayerName: lay.Name, Grane: lay.Grane, Caliber: lay.Caliber,
		Fingerprint: fp, Status: lay.Status, Detail: lay.Detail, Rows: lay.Rows,
		BuiltAt: impactEffectiveAt(), BuiltCostMS: cost.Milliseconds(),
	}
	b, err := json.MarshalIndent(e, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// impactCacheProbeMS —— 毫秒档的**现跑标定口**：读一件落盘件 + 解回条目（= §4.4 的口径），
// 跑 n 次取**最慢**的一次（报的是上限，不是均值 —— 判据是「≤ 4 ms」）。
func impactCacheProbeMS(path string, n int) (float64, int, error) {
	if n <= 0 {
		n = 1
	}
	worst := 0.0
	rows := 0
	for i := 0; i < n; i++ {
		t0 := time.Now()
		b, err := os.ReadFile(path)
		if err != nil {
			return 0, 0, err
		}
		var e impactCacheEntry
		if uerr := json.Unmarshal(b, &e); uerr != nil {
			return 0, 0, uerr
		}
		ms := float64(time.Since(t0).Microseconds()) / 1000.0
		if ms > worst {
			worst = ms
		}
		rows = len(e.Rows)
	}
	return worst, rows, nil
}

// impactCacheFilesUnder —— 列出某目录下的缓存件（**只读** · 自检件用它判「只增不删」）。
func impactCacheFilesUnder(dir string) ([]string, error) {
	out := []string{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 读不到的枝不阻断（只报数用）
		}
		if !info.IsDir() && strings.HasSuffix(p, ".json") {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func shortHex(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return dashIfEmpty(s)
}
