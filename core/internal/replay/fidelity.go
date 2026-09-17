// fidelity.go —— **保真度三指标 + 二分定位**（T4.4；设计稿 docs/01-设计/设计-内建调试版-v1.2-20260917.md §〇 E7 ★★★）。
//
// ── 为什么"回放通过"必须被拆成三条各自可判的指标（E7）──
//
// 原稿只有"不承诺逐位一致"这一句散文。散文不是判据：既不能被机器判定，也不能被证伪，
// 最后必然退化成"我说回放可信"（设计稿 I8：回放可信度**不许自称**）。E7 把它拆成：
//
//	① F = 命中录播事件数 / 回放期尝试事件数。**missed > 0 即 FAIL**（不是警告）
//	② 输出等价性：录制运行的产物 sha256 == 回放运行的产物 sha256（对固定夹具目录）
//	③ 确定性自检：replay(replay(x)) == replay(x)（产物哈希相等）
//	④ 二分定位：两条 trace / 两组产物 ⇒ 最早分歧 seq 与**最小可复现前缀**
//
// ── 三条指标**互不蕴含**（这是本文件存在的理由，不是修辞）──
//
//	· F 只回答"回放期有没有偷偷真发"；F=1.0 与"产物正确"无关（录播本身可能录错、录漏）。
//	· ② 只回答"同一个夹具下录制与回放的产物是否一致"；它有盲区：两侧错得一样时相等。
//	· ③ 只回答"回放是不是纯函数（同输入同输出）"；它抓的是非幂等（追加写、计数器、时间戳）。
//
//	⇒ 缺任何一条，都有一类假绿进得来。所以合并判词不做"平均分/总评分"：任一 FAIL ⇒ 整体 FAIL，
//	  并**逐条保留原话**（评分会让"一条彻底崩掉"被另外两条掩掉）。
//
// ── 写死的三条纪律 ──
//
//  1. **missed 软化 = 假绿**。Verdict() 里 missed>0 与"回放期零尝试"都是 FAIL。软化的代价见
//     变异自证①（把该分支删掉，用例②当场红）。
//  2. **0 == 0 不是证据**。两侧产物集都为空、或三趟产物集全为空 ⇒ FAIL（与 chat/obs 的
//     "字段全空 = 处处相等 = 最危险的假绿"同源）。
//  3. **产物摘要只认相对路径**：绝对路径不参与比较（否则临时目录名会把结论污染成"永远不等"）。
package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// 哨兵错误（调用方一律用 errors.Is 分流，不靠字符串匹配）。
var (
	// ErrFidelityFailed —— 保真度不达标（三条指标的合并专项错误码）。
	ErrFidelityFailed = errors.New("replay: 保真度不达标")
	// ErrArtifacts —— 产物集不可用（目录缺失 / 符号链接 / 非常规文件 / 超上限）。
	ErrArtifacts = errors.New("replay: 产物集不可用")
	// ErrDeterminism —— 确定性自检无法完成（夹具或某趟运行失败）。
	ErrDeterminism = errors.New("replay: 确定性自检失败")
	// ErrBisectNonMonotone —— 二分前提（单调性）不成立：**不猜**数字，直接报错。
	ErrBisectNonMonotone = errors.New("replay: 二分前提不成立")
)

// 报告里最多展开几条（避免一份报告把日志刷爆；超出部分只给条数）。
const (
	maxReportedMisses = 8
	maxReportedDiffs  = 12
)

// ── ① F：命中率与 missed 铁律 ────────────────────────────────────────────────

// MissNote —— 一次未命中的事实（本趟回放内第几次尝试、工具、指纹、错误原文）。
type MissNote struct {
	Index       int    `json:"index"` // 本趟回放内第几次尝试（1 起）
	Tool        string `json:"tool"`
	Fingerprint string `json:"fingerprint"`
	Err         string `json:"err"`
}

// FidelityCounter —— 回放期的事件计数（一趟回放一个；可并发调用）。
//
// 为什么"尝试就计数"要由本包保证而不是交给调用方：只统计成功的调用，分母会缩到等于分子，
// F 恒为 1.0 —— 这正是 I1 说的"写错的探针会给假信号"。
type FidelityCounter struct {
	mu        sync.Mutex
	attempted int
	hit       int
	misses    []MissNote
}

// NewFidelityCounter —— 空计数器。
func NewFidelityCounter() *FidelityCounter { return &FidelityCounter{} }

// Record —— 记一次回放尝试：err == nil ⇒ 命中；err != nil ⇒ 未命中（**照样计入分母**）。
func (c *FidelityCounter) Record(tool, fingerprint string, err error) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attempted++
	if err == nil {
		c.hit++
		return
	}
	c.misses = append(c.misses, MissNote{
		Index:       c.attempted,
		Tool:        tool,
		Fingerprint: fingerprint,
		Err:         err.Error(),
	})
}

// RecordCall —— 记一次回放结果（接通真链路时用这一条，避免调用方自己判"算不算命中"）。
//
// 严格口径：无错误但来源不是 recorded 的，一律按**未命中**计数 —— 回放模式下只有 recorded 一种
// 合法来源（dispatcher.replay 只产出它），其余来源说明接线错了；而"接线错了却算命中"是最不该放过的一种。
func (c *FidelityCounter) RecordCall(tool, fingerprint string, res Result, err error) {
	if err == nil && res.Source != SourceRecorded {
		err = fmt.Errorf("来源 %q 不是录播（回放模式下仅 %q 合法）", res.Source, SourceRecorded)
	}
	c.Record(tool, fingerprint, err)
}

// Attempted / Hit / Missed —— 计数快照（并发安全）。
func (c *FidelityCounter) Attempted() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.attempted
}

func (c *FidelityCounter) Hit() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hit
}

func (c *FidelityCounter) Missed() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.misses)
}

// FidelityReport —— 指标① 的结论（可序列化：脚本按字段判，不靠输出字样）。
type FidelityReport struct {
	Attempted  int        `json:"attempted"`
	Hit        int        `json:"hit"`
	Missed     int        `json:"missed"`
	F          float64    `json:"f"`
	FUndefined bool       `json:"f_undefined"` // Attempted == 0：F 无定义（**不许**写成 1.0）
	Misses     []MissNote `json:"misses,omitempty"`
}

// Report —— 取当前结论（Misses 是副本，调用方改不动计数器）。
func (c *FidelityCounter) Report() FidelityReport {
	if c == nil {
		return FidelityReport{FUndefined: true}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	r := FidelityReport{Attempted: c.attempted, Hit: c.hit, Missed: len(c.misses)}
	r.Misses = append([]MissNote(nil), c.misses...)
	if r.Attempted == 0 {
		r.FUndefined = true
	} else {
		r.F = float64(r.Hit) / float64(r.Attempted)
	}
	return r
}

// Verdict —— 指标① 的判词：nil = PASS。
//
// FAIL 的三种情形都写死在这里（缺一即假绿）：
//   - **回放期零尝试**：F = 0/0 无定义。一次"回放"什么都没尝试，等于没有证据。
//   - **missed > 0**：E7 原文。
//   - **计数自相矛盾**：hit + missed != attempted（计数器被人改过 / 报告被拼过）。
func (r FidelityReport) Verdict() error {
	var why []string
	if r.Attempted == 0 {
		why = append(why, "回放期**零次尝试**：F = 0/0 无定义（不许写成 1.0）—— 这次回放没有任何证据")
	}
	if r.Missed > 0 {
		why = append(why, fmt.Sprintf("missed=%d > 0 ⇒ FAIL（E7 写死：未命中即失败，不软化）· F=%.4f", r.Missed, r.F))
	}
	if r.Hit+r.Missed != r.Attempted {
		why = append(why, fmt.Sprintf("计数自相矛盾：hit(%d) + missed(%d) != attempted(%d)", r.Hit, r.Missed, r.Attempted))
	}
	for i, m := range r.Misses {
		if i >= maxReportedMisses {
			why = append(why, fmt.Sprintf("……另有 %d 处未命中（只展开前 %d 处）", len(r.Misses)-i, maxReportedMisses))
			break
		}
		why = append(why, fmt.Sprintf("未命中 #%d 工具 %s 指纹 %s：%s", m.Index, m.Tool, short(m.Fingerprint), truncate(m.Err, 200)))
	}
	if len(why) == 0 {
		return nil
	}
	return &FidelityError{Kind: "① F 命中率（E7）", Reasons: why}
}

// String —— 一行事实（人读；脚本判据一律用字段，不看这行文字）。
func (r FidelityReport) String() string {
	f := "无定义"
	if !r.FUndefined {
		f = fmt.Sprintf("%.4f", r.F)
	}
	return fmt.Sprintf("F=%s（命中 %d / 尝试 %d，未命中 %d）", f, r.Hit, r.Attempted, r.Missed)
}

// FidelityError —— 保真度判词（专项错误码：errors.Is(err, ErrFidelityFailed)）。
type FidelityError struct {
	Kind    string   // 哪条指标（①②③ 或合并判词）
	Reasons []string // 每条都是可复核的事实，不是形容词
}

func (e *FidelityError) Error() string {
	return fmt.Sprintf("%v：%s —— %s", ErrFidelityFailed, e.Kind, strings.Join(e.Reasons, "；"))
}

// Is —— 支持 errors.Is(err, ErrFidelityFailed)。
func (e *FidelityError) Is(target error) bool { return target == ErrFidelityFailed }

// Verdictor —— 一条自判结论（每条指标各自可判 ⇒ 也能合并看）。
type Verdictor interface{ Verdict() error }

// Verdict —— 汇总多条结论：全过 ⇒ nil；任一不过 ⇒ 一条 *FidelityError（逐条保留原话）。
//
// 刻意不做"总评分"：E7 要求三条各自可判；合成一个数会让"某条彻底崩掉"被另外两条的漂亮分数掩掉。
func Verdict(parts ...Verdictor) error {
	var why []string
	for _, p := range parts {
		if p == nil {
			continue
		}
		err := p.Verdict()
		if err == nil {
			continue
		}
		var fe *FidelityError
		if errors.As(err, &fe) {
			why = append(why, fmt.Sprintf("%s：%s", fe.Kind, strings.Join(fe.Reasons, "；")))
			continue
		}
		why = append(why, err.Error())
	}
	if len(why) == 0 {
		return nil
	}
	return &FidelityError{Kind: "合并判词（三条指标各自可判）", Reasons: why}
}

// ── ②/③ 的地基：产物集摘要 ──────────────────────────────────────────────────

// defaultMaxArtifacts —— 产物文件数上限（超过即报错：截断的产物集会让"相等"变成假绿）。
const defaultMaxArtifacts = 100000

// ArtifactOptions —— 产物扫描口径。**默认最严**：一个文件都不跳（跳文件等于自己给自己减题）。
type ArtifactOptions struct {
	Skip        []string // 相对路径黑名单（含子树）
	SkipSuffix  []string // 后缀黑名单
	IncludeMode bool     // 是否把权限位纳入摘要（默认 false：umask 差异不是"产物差异"）
	MaxFiles    int      // 0 = defaultMaxArtifacts
}

// Artifact —— 一个产物文件。Rel 是**相对路径**（绝对路径不参与比较，否则临时目录名会污染结论）。
type Artifact struct {
	Rel    string `json:"rel"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
	Mode   string `json:"mode,omitempty"`
}

// ArtifactSet —— 一次运行的产物集摘要（稳定序 ⇒ 同样的产物必然同样字节）。
type ArtifactSet struct {
	Root    string     `json:"root"`   // 仅供人读（**不参与**摘要）
	Digest  string     `json:"digest"` // 逐文件 rel‖digest[‖mode] 排序拼接后的 sha256
	Files   []Artifact `json:"files"`
	Skipped []string   `json:"skipped,omitempty"` // 被显式跳过的（如实报告，不静默）
}

// File —— 按相对路径取一个产物。
func (a *ArtifactSet) File(rel string) (Artifact, bool) {
	if a == nil {
		return Artifact{}, false
	}
	for _, f := range a.Files {
		if f.Rel == rel {
			return f, true
		}
	}
	return Artifact{}, false
}

// Len —— 产物文件数。
func (a *ArtifactSet) Len() int {
	if a == nil {
		return 0
	}
	return len(a.Files)
}

// DigestDir —— 产物目录摘要（**严格**）：
//
//	· 目录不存在/不是目录 ⇒ 报错（"两边都缺"不许当成"两边相等"）；
//	· 符号链接 ⇒ 报错（链接能把摘要指向目录外，也让"同一份产物"有两种字节形态）；
//	· 非常规文件（fifo/设备/socket）⇒ 报错（不是可比的产物）；
//	· 文件数超上限 ⇒ 报错（**拒绝截断**）。
func DigestDir(root string, opt ArtifactOptions) (*ArtifactSet, error) {
	st, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("%w: 产物目录 %s：%v", ErrArtifacts, root, err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%w: %s 不是目录", ErrArtifacts, root)
	}
	maxFiles := opt.MaxFiles
	if maxFiles <= 0 {
		maxFiles = defaultMaxArtifacts
	}
	set := &ArtifactSet{Root: root}
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && skipArtifact(rel, opt) {
				set.Skipped = append(set.Skipped, rel+"/（子树）")
				return fs.SkipDir
			}
			return nil
		}
		// 显式跳过**先于**链接/类型检查：Skip 是调用方给的逃生口，且跳过会如实列在 Skipped 里（不静默）。
		if skipArtifact(rel, opt) {
			set.Skipped = append(set.Skipped, rel)
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s 是符号链接（拒绝：摘要会指向目录外）", ErrArtifacts, rel)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%w: %s 不是常规文件（%s）", ErrArtifacts, rel, d.Type())
		}
		if len(set.Files) >= maxFiles {
			return fmt.Errorf("%w: %s 的产物文件数超过上限 %d（拒绝截断）", ErrArtifacts, root, maxFiles)
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		a := Artifact{Rel: rel, Size: int64(len(b)), Digest: Digest(b)}
		if opt.IncludeMode {
			a.Mode = fmt.Sprintf("%04o", info.Mode().Perm())
		}
		set.Files = append(set.Files, a)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("%w: 遍历 %s：%v", ErrArtifacts, root, walkErr)
	}
	// 显式排序（不依赖遍历顺序：遍历顺序是环境的属性，不是产物的属性）。
	sort.Slice(set.Files, func(i, j int) bool { return set.Files[i].Rel < set.Files[j].Rel })
	var sb strings.Builder
	fmt.Fprintf(&sb, "artifacts\x1f%d\x1f", len(set.Files))
	for _, a := range set.Files {
		fmt.Fprintf(&sb, "%s\x1f%s\x1f%s\n", a.Rel, a.Digest, a.Mode)
	}
	set.Digest = Digest([]byte(sb.String()))
	return set, nil
}

func skipArtifact(rel string, opt ArtifactOptions) bool {
	for _, s := range opt.Skip {
		s = strings.TrimSuffix(strings.TrimSpace(s), "/")
		if s == "" {
			continue
		}
		if rel == s || strings.HasPrefix(rel, s+"/") {
			return true
		}
	}
	for _, s := range opt.SkipSuffix {
		if s != "" && strings.HasSuffix(rel, s) {
			return true
		}
	}
	return false
}

// ArtifactDiff —— 一处差异。
type ArtifactDiff struct {
	Rel  string `json:"rel"`
	Kind string `json:"kind"` // 内容不同 / 大小不同 / 权限不同 / 仅基准侧有 / 仅本次侧有
	Want string `json:"want,omitempty"`
	Got  string `json:"got,omitempty"`
}

// EquivalenceReport —— 指标② 的结论。
type EquivalenceReport struct {
	WantDigest string         `json:"want_digest"`
	GotDigest  string         `json:"got_digest"`
	WantCount  int            `json:"want_count"`
	GotCount   int            `json:"got_count"`
	Equal      bool           `json:"equal"`
	Diffs      []ArtifactDiff `json:"diffs,omitempty"`
}

// CompareArtifacts —— 逐文件比对两组产物（差异按相对路径排序 ⇒ 与人读顺序一致）。
func CompareArtifacts(want, got *ArtifactSet) EquivalenceReport {
	rep := EquivalenceReport{}
	if want != nil {
		rep.WantDigest, rep.WantCount = want.Digest, len(want.Files)
	}
	if got != nil {
		rep.GotDigest, rep.GotCount = got.Digest, len(got.Files)
	}
	rep.Equal = want != nil && got != nil && want.Digest != "" && want.Digest == got.Digest

	byRel := map[string][2]*Artifact{} // rel → {基准, 本次}
	for i := range want.Files {
		a := want.Files[i]
		v := byRel[a.Rel]
		v[0] = &a
		byRel[a.Rel] = v
	}
	for i := range got.Files {
		a := got.Files[i]
		v := byRel[a.Rel]
		v[1] = &a
		byRel[a.Rel] = v
	}
	rels := mapKeys(byRel)
	for _, rel := range rels {
		v := byRel[rel]
		switch {
		case v[0] == nil:
			rep.Diffs = append(rep.Diffs, ArtifactDiff{Rel: rel, Kind: "仅本次侧有", Got: short(v[1].Digest)})
		case v[1] == nil:
			rep.Diffs = append(rep.Diffs, ArtifactDiff{Rel: rel, Kind: "仅基准侧有", Want: short(v[0].Digest)})
		case v[0].Digest != v[1].Digest:
			kind := "内容不同"
			if v[0].Size != v[1].Size {
				kind = "大小不同"
			}
			rep.Diffs = append(rep.Diffs, ArtifactDiff{Rel: rel, Kind: kind, Want: short(v[0].Digest), Got: short(v[1].Digest)})
		case v[0].Mode != v[1].Mode:
			rep.Diffs = append(rep.Diffs, ArtifactDiff{Rel: rel, Kind: "权限不同", Want: v[0].Mode, Got: v[1].Mode})
		}
	}
	return rep
}

// Verdict —— 指标② 的判词：nil = PASS。
func (e EquivalenceReport) Verdict() error {
	var why []string
	if e.WantCount+e.GotCount == 0 {
		why = append(why, "两侧产物集都为空：0 == 0 不是等价性证据（最危险的一种假绿）")
	}
	if !e.Equal {
		why = append(why, fmt.Sprintf("产物摘要不等：基准 %s（%d 文件）vs 本次 %s（%d 文件）",
			short(e.WantDigest), e.WantCount, short(e.GotDigest), e.GotCount))
	}
	for i, d := range e.Diffs {
		if i >= maxReportedDiffs {
			why = append(why, fmt.Sprintf("……另有 %d 处差异", len(e.Diffs)-i))
			break
		}
		why = append(why, fmt.Sprintf("%s：%s（基准 %s / 本次 %s）", d.Rel, d.Kind, orDash(d.Want), orDash(d.Got)))
	}
	if len(why) == 0 {
		return nil
	}
	return &FidelityError{Kind: "② 输出等价性（E7）", Reasons: why}
}

// Earliest —— 最早一处差异（按相对路径排序后的第一处；"最早"在产物侧按**排序**定义，写死）。
func (e EquivalenceReport) Earliest() (ArtifactDiff, bool) {
	if len(e.Diffs) == 0 {
		return ArtifactDiff{}, false
	}
	sorted := append([]ArtifactDiff(nil), e.Diffs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Rel < sorted[j].Rel })
	return sorted[0], true
}

// String —— 一行事实。
func (e EquivalenceReport) String() string {
	state := "不等"
	if e.Equal {
		state = "相等"
	}
	return fmt.Sprintf("产物%s（基准 %d 文件 %s · 本次 %d 文件 %s · 差异 %d 处）",
		state, e.WantCount, short(e.WantDigest), e.GotCount, short(e.GotDigest), len(e.Diffs))
}

// ── ③ 确定性自检：replay(replay(x)) == replay(x) ────────────────────────────

// FixtureSetup —— 铺夹具（输入 + trace）：一整趟回放的起点。要求**确定性**（同一夹具每趟同样字节），
// 否则 ③ 测的是"夹具的抖动"而不是"回放的确定性"。
type FixtureSetup func(fixtureDir string) error

// ReplayRun —— 用 fixtureDir 里的输入在 outDir 产出；产物**只**写 outDir（写进夹具目录会被
// FixtureUntouched 抓到）。
type ReplayRun func(fixtureDir, outDir string) error

// DeterminismReport —— 指标③ 的结论。
//
// 两个口径分开记（都要求为真）：
//
//	Stable      —— 两趟**独立起点**的产物相等（同输入同输出的基本盘）；
//	LayerStable —— replay(replay(x)) == replay(x)：第二趟**叠在第一趟的产物上**再跑（抓非幂等：
//	               追加写、计数器、时间戳、自增 id —— 这些的第一次可能看不出来，第二次就露了）。
type DeterminismReport struct {
	FirstDigest  string `json:"first_digest"`
	SecondDigest string `json:"second_digest"`
	ThirdDigest  string `json:"third_digest"`
	FirstFiles   int    `json:"first_files"`
	SecondFiles  int    `json:"second_files"`
	ThirdFiles   int    `json:"third_files"`

	FixtureDigestBefore string `json:"fixture_digest_before"`
	FixtureDigestAfter  string `json:"fixture_digest_after"`
	FixtureUntouched    bool   `json:"fixture_untouched"`

	Stable      bool `json:"stable"`
	LayerStable bool `json:"layer_stable"`

	DiffsAB []ArtifactDiff `json:"diffs_ab,omitempty"`
	DiffsAC []ArtifactDiff `json:"diffs_ac,omitempty"`
	Runs    int            `json:"runs"`
}

// CheckDeterminism —— 跑三趟（A / B / A 叠加）并比较产物摘要。
//
// 顺序写死，因为顺序本身就是判据：第三趟必须**复用第一趟的输出目录**（那就是"回放的回放"），
// 若改成再开一个干净目录，非幂等的那类缺陷就抓不到了。
func CheckDeterminism(setup FixtureSetup, run ReplayRun, root string, opt ArtifactOptions) (*DeterminismReport, error) {
	if setup == nil || run == nil {
		return nil, fmt.Errorf("%w: 需要 setup 与 run 都非 nil（拒绝空跑出结论）", ErrDeterminism)
	}
	fix := filepath.Join(root, "fixture")
	if err := os.MkdirAll(fix, 0o755); err != nil {
		return nil, fmt.Errorf("%w: 建夹具目录 %s：%v", ErrDeterminism, fix, err)
	}
	if err := setup(fix); err != nil {
		return nil, fmt.Errorf("%w: 铺夹具：%v", ErrDeterminism, err)
	}
	before, err := DigestDir(fix, opt)
	if err != nil {
		return nil, err
	}
	outA, outB := filepath.Join(root, "out-a"), filepath.Join(root, "out-b")
	a1, err := runAndDigest(run, fix, outA, opt)
	if err != nil {
		return nil, err
	}
	a2, err := runAndDigest(run, fix, outB, opt)
	if err != nil {
		return nil, err
	}
	a3, err := runAndDigest(run, fix, outA, opt) // 叠在第一趟产物上 ⇒ 这就是 replay(replay(x))
	if err != nil {
		return nil, err
	}
	after, err := DigestDir(fix, opt)
	if err != nil {
		return nil, err
	}
	return NewDeterminismReport(a1, a2, a3, before, after), nil
}

// NewDeterminismReport —— 用三份产物集与夹具前后摘要直接造结论（判定口径与 CheckDeterminism **一字不差**：
// 抽出来的唯一理由是"已经自己跑过三趟"的调用方——门禁夹具就是这样用的：它要的产物目录名是固定的
// （recorded / replay-a / replay-b），脚本要独立地再哈希一遍那几个目录）。
func NewDeterminismReport(first, second, third, fixtureBefore, fixtureAfter *ArtifactSet) *DeterminismReport {
	rep := &DeterminismReport{
		FirstFiles:  first.Len(),
		SecondFiles: second.Len(),
		ThirdFiles:  third.Len(),
		Runs:        3,
	}
	rep.FirstDigest, rep.SecondDigest, rep.ThirdDigest = first.Digest, second.Digest, third.Digest
	if fixtureBefore != nil && fixtureAfter != nil {
		rep.FixtureDigestBefore, rep.FixtureDigestAfter = fixtureBefore.Digest, fixtureAfter.Digest
		rep.FixtureUntouched = fixtureBefore.Digest == fixtureAfter.Digest
	}
	rep.Stable = first.Digest == second.Digest
	rep.LayerStable = first.Digest == third.Digest
	if !rep.Stable {
		rep.DiffsAB = CompareArtifacts(first, second).Diffs
	}
	if !rep.LayerStable {
		rep.DiffsAC = CompareArtifacts(first, third).Diffs
	}
	return rep
}

func runAndDigest(run ReplayRun, fix, out string, opt ArtifactOptions) (*ArtifactSet, error) {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, fmt.Errorf("%w: 建产物目录 %s：%v", ErrDeterminism, out, err)
	}
	if err := run(fix, out); err != nil {
		return nil, fmt.Errorf("%w: 回放运行 %s：%v", ErrDeterminism, out, err)
	}
	return DigestDir(out, opt)
}

// Verdict —— 指标③ 的判词：nil = PASS。
func (d *DeterminismReport) Verdict() error {
	if d == nil {
		return &FidelityError{Kind: "③ 确定性自检（E7）", Reasons: []string{"报告缺失（nil）：没有证据"}}
	}
	var why []string
	if d.FirstFiles+d.SecondFiles+d.ThirdFiles == 0 {
		why = append(why, "三趟产物集全为空：0 == 0 不是确定性证据")
	}
	if !d.Stable {
		why = append(why, fmt.Sprintf("两趟独立起点的产物不同（%s vs %s）⇒ 回放不是纯函数", short(d.FirstDigest), short(d.SecondDigest)))
		why = append(why, diffLines(d.DiffsAB, "A/B")...)
	}
	if !d.LayerStable {
		why = append(why, fmt.Sprintf("replay(replay(x)) != replay(x)（%s vs %s）⇒ 第二趟叠在第一趟产物上时产物变了（典型病原：追加写 / 计数器 / 时间戳 / 自增 id）",
			short(d.FirstDigest), short(d.ThirdDigest)))
		why = append(why, diffLines(d.DiffsAC, "A/A'")...)
	}
	if !d.FixtureUntouched {
		why = append(why, fmt.Sprintf("回放改动了夹具（%s → %s）：回放路径应只读，写入夹具会让下一趟的输入悄悄变样",
			short(d.FixtureDigestBefore), short(d.FixtureDigestAfter)))
	}
	if len(why) == 0 {
		return nil
	}
	return &FidelityError{Kind: "③ 确定性自检（E7）", Reasons: why}
}

func diffLines(diffs []ArtifactDiff, tag string) []string {
	if len(diffs) == 0 {
		return nil
	}
	out := make([]string, 0, len(diffs))
	for i, d := range diffs {
		if i >= maxReportedDiffs {
			out = append(out, fmt.Sprintf("……%s 另有 %d 处差异", tag, len(diffs)-i))
			break
		}
		out = append(out, fmt.Sprintf("%s %s：%s", tag, d.Rel, d.Kind))
	}
	return out
}

// String —— 一行事实。
func (d *DeterminismReport) String() string {
	if d == nil {
		return "确定性报告缺失"
	}
	return fmt.Sprintf("确定性 独立两趟=%v（%s / %s）· 叠加=%v（%s）· 夹具只读=%v · 跑了 %d 趟",
		d.Stable, short(d.FirstDigest), short(d.SecondDigest), d.LayerStable, short(d.ThirdDigest), d.FixtureUntouched, d.Runs)
}

// ── ④ 二分定位最早分歧（E7 末句）────────────────────────────────────────────

// Divergence —— 分歧定位结论（静态比对与行为二分共用同一形态）。
type Divergence struct {
	EarliestSeq int            `json:"earliest_seq"` // 最早分歧 seq（0 = 未发现分歧）
	PrefixLen   int            `json:"prefix_len"`   // 最小可复现前缀长度 = EarliestSeq
	Source      string         `json:"source"`       // "静态逐条比对" / "行为二分"
	Reason      string         `json:"reason"`
	Probes      int            `json:"probes,omitempty"` // 探测次数（行为二分才有）
	Artifacts   []ArtifactDiff `json:"artifacts,omitempty"`

	Prefix []Record `json:"-"` // 最小可复现前缀本体 = a[:PrefixLen]
	Hybrid []Record `json:"-"` // 复现分歧的最小混合 trace = a[:p] + b[p:]
}

// String —— 一行事实。
func (d Divergence) String() string {
	if d.EarliestSeq == 0 {
		return fmt.Sprintf("分歧定位（%s）：无分歧 —— %s", d.Source, d.Reason)
	}
	return fmt.Sprintf("分歧定位（%s）：最早分歧 seq=%d · 最小可复现前缀 %d 条%s —— %s",
		d.Source, d.EarliestSeq, d.PrefixLen, probesNote(d.Probes), d.Reason)
}

func probesNote(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("（%d 次探测）", n)
}

// DiffRecords —— **静态**逐条比对（O(n)，不跑任何回放）：按 seq 对齐，比
// {工具, 参数摘要, 结果摘要, 错误文本}。
//
// 刻意**不比** DurationMS：那是环境读数，不是录播事实 —— 比它会让每次回放都"分歧"（噪声淹没信号）。
// 长度不同 ⇒ 最短侧的下一条即最早分歧点（"少了一条"同样是事实分歧，而且从那条开始）。
func DiffRecords(a, b []Record) Divergence {
	d := Divergence{Source: "静态逐条比对"}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		ra, rb := a[i], b[i]
		// 手写/派生 trace 的 Record 没有载体赋的 seq（Seq=0）⇒ 用序号当 seq，绝不报 0 ——
		// 0 在本包里是"无分歧"的取值，用它表示"第 0 条分歧"会把结论读反。
		seq := ra.Seq
		if seq == 0 {
			seq = i + 1
		}
		if ra.Seq != rb.Seq {
			d.EarliestSeq, d.PrefixLen = seq, i+1
			d.Reason = fmt.Sprintf("seq 不对齐：第 %d 条 a.seq=%d b.seq=%d（两条 trace 的编号本身已分歧）", i+1, ra.Seq, rb.Seq)
			d.Prefix = append([]Record(nil), a[:i+1]...)
			return d
		}
		if why := recordDiff(ra, rb); why != "" {
			d.EarliestSeq, d.PrefixLen = seq, i+1
			d.Reason = fmt.Sprintf("第 %d 条（seq=%d）分歧：%s", i+1, seq, why)
			d.Prefix = append([]Record(nil), a[:i+1]...)
			return d
		}
	}
	if len(a) != len(b) {
		i := n
		d.EarliestSeq, d.PrefixLen = i+1, i+1
		d.Reason = fmt.Sprintf("条数不同（a=%d b=%d）：最短侧的下一条（第 %d 条）即分歧起点", len(a), len(b), i+1)
		if i < len(a) {
			d.Prefix = append([]Record(nil), a[:i+1]...)
		}
		return d
	}
	d.Reason = "逐条比对无分歧（两条 trace 的录播事实相同）"
	return d
}

// recordDiff —— 两条录播事实的差异（空串 = 无差异）。
//
// **摘要缺席时比正文**：手写/派生的 Record（例如门禁夹具里的变异体）可能只有正文没有摘要；
// 此时若只比摘要字段，"两侧都空"就会被读成"相同"——那正是"字段全空 = 处处相等 = 最危险的假绿"。
func recordDiff(a, b Record) string {
	switch {
	case a.Tool != b.Tool:
		return fmt.Sprintf("工具不同（a=%s b=%s）", a.Tool, b.Tool)
	case argsDiffer(a, b):
		return fmt.Sprintf("参数摘要不同（a=%s b=%s）", short(a.ArgsDigest), short(b.ArgsDigest))
	case resultDiffers(a, b):
		return fmt.Sprintf("结果摘要不同（a=%s b=%s）", short(a.ResultDigest), short(b.ResultDigest))
	case a.ErrText != b.ErrText:
		return fmt.Sprintf("错误文本不同（a=%q b=%q）—— 异常路径也入 trace（E10），报错内容同样是事实", truncate(a.ErrText, 80), truncate(b.ErrText, 80))
	}
	return ""
}

func argsDiffer(a, b Record) bool {
	if a.ArgsDigest != "" || b.ArgsDigest != "" {
		return a.ArgsDigest != b.ArgsDigest
	}
	return !bytes.Equal(a.ArgsCanonical, b.ArgsCanonical)
}

func resultDiffers(a, b Record) bool {
	if a.ResultDigest != "" || b.ResultDigest != "" {
		return a.ResultDigest != b.ResultDigest
	}
	return !bytes.Equal(a.ResultBody, b.ResultBody)
}

// Hybrid —— 拼接混合 trace：a 的前 p 条 + b 的其余条（就是"最小可复现前缀"的载体）。
func Hybrid(a, b []Record, p int) []Record {
	if p < 0 {
		p = 0
	}
	if p > len(a) {
		p = len(a)
	}
	out := make([]Record, 0, len(a)+len(b))
	out = append(out, a[:p]...)
	if p < len(b) {
		out = append(out, b[p:]...)
	}
	return out
}

// BisectError —— 二分前提不成立（专项错误码：errors.Is(err, ErrBisectNonMonotone)）。
type BisectError struct {
	Why    string
	Probes int
}

func (e *BisectError) Error() string {
	return fmt.Sprintf("%v：%s（已探测 %d 次；**不猜**一个数字出来 —— 前提不成立时给出的 seq 是毒证据）",
		ErrBisectNonMonotone, e.Why, e.Probes)
}

// Is —— 支持 errors.Is(err, ErrBisectNonMonotone)。
func (e *BisectError) Is(target error) bool { return target == ErrBisectNonMonotone }

// BisectEarliestDivergence —— **行为二分**：找最小前缀 p，使"a 的前 p 条 + b 的其余条"跑出来的产物
// 就已经与 b 的产物不同 ⇒ 最早分歧 seq 与**最小可复现前缀** a[:p]。
//
//	p = 0      ⇒ 完全是 b ⇒ 产物必须与 b 一致。本函数**先验证这一点**：若把 b 原样再跑一遍产物就变了，
//	             说明这份"回放"自己就不确定（同输入两套产物）⇒ 二分毫无意义 ⇒ 报错（顺带充当确定性探针）。
//	p = len(a) ⇒ 完全是 a ⇒ 若仍与 b 无分歧 ⇒ EarliestSeq = 0（两条 trace 在**产物**上不可分：
//	             录播事实有差异，但没有一条改变结果）。
//	单调前提   ⇒ pred(p) 形如 false…false true…true。这是二分成立的前提；本函数校验它用到的边界两点、
//	             并对结论**复验**（pred(p) 必须为真、pred(p-1) 必须为假）；前提破裂 ⇒ 报错，不猜。
//	探测次数    ⇒ ≈ log2(n) + 常数（用例⑤钉住"远小于线性扫描"）。
//
// run 由调用方提供（"用这组录播事实跑一遍，给我产物集"）—— 本包不知道你的回放长什么样，也不假装知道。
func BisectEarliestDivergence(a, b []Record, run func(hybrid []Record) (*ArtifactSet, error)) (Divergence, error) {
	if run == nil {
		return Divergence{}, &BisectError{Why: "run 为 nil（拒绝空跑出结论）"}
	}
	d := Divergence{Source: "行为二分"}
	ref, err := run(b)
	if err != nil {
		return Divergence{}, &BisectError{Why: fmt.Sprintf("参照运行（b 全量）失败：%v", err)}
	}
	probes := 1
	var last *ArtifactSet
	pred := func(p int) (bool, error) {
		probes++
		res, rerr := run(Hybrid(a, b, p))
		if rerr != nil {
			return false, fmt.Errorf("前缀 %d 的混合运行失败：%w", p, rerr)
		}
		last = res
		return res.Digest != ref.Digest, nil
	}
	// ① 复验 p=0（把 b 原样再跑一遍）：产物必须与参照一致，否则这份回放本身不确定。
	d0, err := pred(0)
	if err != nil {
		return Divergence{}, &BisectError{Why: err.Error(), Probes: probes}
	}
	if d0 {
		return Divergence{}, &BisectError{
			Why:    fmt.Sprintf("把 b 原样再跑一遍产物就变了（%s vs %s）⇒ 这份回放不确定（同输入两套产物）⇒ 二分无意义", short(ref.Digest), short(last.Digest)),
			Probes: probes,
		}
	}
	n := len(a)
	if n == 0 {
		d.PrefixLen, d.Probes, d.Reason = 0, probes, "a 为空：没有可比的前缀"
		return d, nil
	}
	// ② 全量代入：若仍无分歧 ⇒ 两条 trace 在产物上不可分。
	dn, err := pred(n)
	if err != nil {
		return Divergence{}, &BisectError{Why: err.Error(), Probes: probes}
	}
	if !dn {
		d.Probes, d.Artifacts = probes, CompareArtifacts(ref, last).Diffs
		d.Reason = fmt.Sprintf("把 a 全量代入也不影响产物 ⇒ 两条 trace 在产物上不可分（%d 条录播事实的差异没有一条改变结果）", len(a))
		return d, nil
	}
	// ③ 二分最小 p。
	lo, hi := 1, n
	for lo < hi {
		mid := lo + (hi-lo)/2
		dm, merr := pred(mid)
		if merr != nil {
			return Divergence{}, &BisectError{Why: merr.Error(), Probes: probes}
		}
		if dm {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	p := lo
	dp, err := pred(p) // 复验结论点
	if err != nil {
		return Divergence{}, &BisectError{Why: err.Error(), Probes: probes}
	}
	if !dp {
		return Divergence{}, &BisectError{
			Why:    fmt.Sprintf("复验失败：前缀 %d 的产物与 b 一致，二分却把它当成了分歧点（探测非单调）", p),
			Probes: probes,
		}
	}
	// 产物侧差异在**复验之后立刻取**：再往后还有 pred(p-1)（那一趟是"不分歧"的），
	// 若在它之后取 last，就会把"不分歧的那一趟"当成分歧点的产物（实测踩过）。
	d.EarliestSeq, d.PrefixLen, d.Probes = p, p, probes
	d.Prefix = append([]Record(nil), a[:p]...)
	d.Hybrid = Hybrid(a, b, p)
	d.Artifacts = CompareArtifacts(ref, last).Diffs
	if p > 1 {
		dprev, perr := pred(p - 1)
		if perr != nil {
			return Divergence{}, &BisectError{Why: perr.Error(), Probes: probes}
		}
		if dprev {
			return Divergence{}, &BisectError{
				Why:    fmt.Sprintf("前缀 %d 就已经分歧（最小性被打破，探测非单调）", p-1),
				Probes: probes,
			}
		}
	}
	d.Reason = fmt.Sprintf("最小可复现前缀 = a[:%d]（第 %d 条 / seq=%d 是第一条让产物改变的事实）；共探测 %d 次（逐条线性扫描需 %d 次）",
		p, p, seqOfRecord(a[p-1], p), probes, n)
	return d, nil
}

// seqOfRecord —— 录播事实的 seq（手写/派生 trace 的 Record 没有载体赋的 seq ⇒ 用序号兜底）。
func seqOfRecord(r Record, ordinal int) int {
	if r.Seq != 0 {
		return r.Seq
	}
	return ordinal
}

// LoadRecords —— 严格读整个 trace（任何损坏即报错；不静默跳过）。二分与脚本的输入都走它。
func LoadRecords(path string) ([]Record, error) {
	r, err := OpenReader(path, 0)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out []Record
	for {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

// ── 小工具（报告里只放可复核的短串，不放整段正文）────────────────────────────

func mapKeys(m map[string][2]*Artifact) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func short(s string) string {
	s = strings.TrimPrefix(s, "sha256:")
	if len(s) > 16 {
		return s[:16]
	}
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// jsonOf —— 稳定序列化（报告写盘用；缩进固定 ⇒ 同样的结论同样字节）。
func jsonOf(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("replay: 序列化报告：%w", err)
	}
	return append(b, '\n'), nil
}
