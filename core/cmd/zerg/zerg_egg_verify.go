// zerg_egg_verify.go —— 卵3① 落地件：**逐段并行 sha256 ＋ 内容寻址校验缓存**。
//
// 出处：设计稿 `01-设计/设计-茧壁-统一封闭契约与等级自证.md` 的 §16.7①（加速落地）· §二十①（零拍板三连之首）
// · §14.4（片7 §①-1 并行分块 / §①-2 缓存键与复核）；任务单 `项目文档/v2.5.15/任务单-卵单文件封装-v2-20261015.md`
// 的 卵3① 行（「零格式改动、零依赖 · 缓存键 ＝ 卵内容 sha · `W ＝ clamp(ceil(D/h), 1, NumCPU)`」）。
//
// ---- 落地面（✗ 不改格式 · ✗ 不改 §11.2 任一不变量）------------------------------------------
//
//  1. **逐段并行 sha256**：§11.2 的索引**逐段**存 `sha256` ⇒ 段间天然独立 ⇒ 段级并行不动任一不变量。
//     实现 = **定长 worker pool**（W 个 goroutine 从段队列取活；W ≤ NumCPU ⇒ 上界确定；
//     ✗ 不给每段起一个 goroutine）。每 worker 顺序 `ReadAt` 进**池化 4 MiB 缓冲**增量喂 hasher
//     （✗ 不把整段读进内存 —— 单段可能 GB 级）。
//
//  2. **并发度**：`W = clamp(ceil(D/h), 1, NumCPU)`（§16.7①）。`D` ＝ 本次要过哈希的**数据字节数**
//     （＝索引里各段 `size` 之和，✗ 不硬编码机器）；`h` ＝ `min(本机现测单流 sha256 速率, 0.53 GB/s)` ——
//     0.53 GB/s 是片7 对真实 22 GB 卵实测的单流有效速率，**当上界**用（本机更快 ⇒ ✗ 不越过片7 口径；
//     本机更慢（探针 < 0.53）⇒ 取探针值自动多开一路；探针异常 ⇒ 取 0.53 兜底）。
//     `--workers N`（隐藏取旗标 · 照 `--now` 的同款）可覆盖，仍 clamp 到 `[1, NumCPU]`。
//
//  3. **内容寻址校验缓存**：键 ＝ **卵内容 sha** —— 即 §13.2 的内容寻址命名面 `<sha>.egg` 里那个 sha
//     （「主控只记 sha、不记路径」）⇒ ✗ 不按路径建键、✗ 不按 mtime 建键。件名不是该形态 ⇒ **不启用缓存**
//     （✗ 不拿路径凑一个键顶替）。命中判定 ＝ 片7 §①-2 的**廉价复核束**：
//     `st_dev ＋ st_ino ＋ st_size ＋ st_mtime_ns ＋ sha256(前 32 B 固定前缀)`；
//     ★ 本件另加固一条 `sha256(索引字节)`：片7 的 32 B 前缀只覆盖 `magic/version/align/index_len`，
//     **挡不住「索引体被换」**（段表被换 ＝ 段边界被换），故把索引字节一并纳入复核束（读索引只 O(KB)）。
//
//  4. ★★ **命中档刻意不解析索引** —— 解析会把「前 32 B 复核」这条牙遮掉（改写 `magic` 会被解析器先拦下，
//     于是那条牙在负控里显不出牙口）。命中档只比**字节摘要**，索引用缓存条目里的 `segments/align` 重建**空壳**。
//
//  5. **TOCTOU 口径（片7 §①-2「更严那条」＋ 本件一条 fail-closed 加固）**：一次 `open` ⇒ 对**同一个 fd**
//     `fstat`、读 32 B 前缀、逐段 `ReadAt`（数据面 ≈ 全部字节都走这一个 fd）。
//     ★ 加固：键命中且 `st_dev/st_ino/st_size/st_mtime_ns` **四项全同**、而**前 32 B 摘要变了** ⇒
//     只可能是「原地改写 ＋ 复原时间戳」的同键换件 ⇒ **判红**（✗ 不当普通未命中放行）。
//     ★ 索引体由旧档的按路径入口 `zegParseIndex` 解析（§11.2 第 6 条「读写同源」 ⇒ ✗ 不另写一份解析器）；
//     解析前后各用同一个 fd 现算一次复核束，**两次不等 ⇒ 本次不落条目**（见 `zegvVerifyEntry` ②）。
//
//  6. **缓存落点（运行态面 · ✗ 不进仓 · ✗ 不碰真库路径）**：`$ZERG_EGG_VERIFY_CACHE`；
//     缺省 `<os.UserCacheDir()>/zerg/egg-verify-cache`。条目 = `<dir>/<键前 2 位>/<键>.json`，
//     **只存绿 verdict**，写入走「临时名 ＋ `rename`」原子替换。
//
//  7. **失败/损坏回退（✗ 不得因缓存坏而误报绿）**：条目缺失 / JSON 坏 / 版本或工具版本不符 / 键不符 /
//     物理面任一不匹配 ⇒ 一律**未命中**并**照旧跑全量校验**，结论只由全量校验给；
//     缓存目录不可写 ⇒ 本次不落条目（✗ 不改判结论）。
//
// ---- 与单线程档的一致性 ---------------------------------------------------------------------
//
// `zerg_egg.go` 的 `zegVerify`（单线程全量）仍是**行为的真源**：本件并行档走**同一套判据、同一套问题串**，
// 按**索引序**汇总 problems ⇒ 与单线程档**逐字节一致**（牙：`TestZergEggVerifyParallelMatchesSingle`，
// 另落两态件供仓外 `cmp` 对拍）。
//
// ---- 未覆盖面（如实列出 · ✗ 都属本单不碰的待拍面）--------------------------------------------
//
//	(a) **「只改数据段、头部不动」＋复原 mtime/size/inode 的换件** —— 复核束照样全中 ⇒ 缓存仍会命中
//	    （＝片7 的 **W3**）。正解两条（叠加 fs-verity / 首末抽块复核）**各卡一个拍板**（特权 ＋ digest 口径）
//	    ⇒ 本单 ✗ 不碰，只把 W3 记为残留面。
//	(b) **fs-verity**（Linux 读时按块 Merkle 校验）与**签名块**：均为待拍面。
//	(c) 本件 **✗ 不接线进 `main.go`**（命令面沿用既有 `zerg cocoon verify` 转发壳）；`--workers` 是隐藏旋钮
//	    （✗ 不进 `-h` 帮助面 ⇒ 不污染与 py 的逐字节对拍 —— 同 `--now` 的既有体例）。
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// zegvCacheVer —— 缓存条目格式版本（结构一变就 +1 ⇒ 旧条目自动未命中）。
	zegvCacheVer = 1
	// zegvCacheEnv —— 缓存目录的环境旋钮（运行态面）。
	zegvCacheEnv = "ZERG_EGG_VERIFY_CACHE"
	// zegvCacheMark —— 命中缓存时**逐字**印的标记（任务单 卵3① 行逐字：「(缓存)」）。
	zegvCacheMark = "(缓存)"
	// zegvBufBytes —— 每 worker 的池化读缓冲（片7 建议 1–4 MiB；W ≤ NumCPU ⇒ 总占用上界确定）。
	zegvBufBytes = 4 << 20
	// zegvProbeBytes —— 并发度探针的样本量（内存内 · 不碰盘）。
	zegvProbeBytes = 8 << 20
	// zegvKeyHexLen —— 卵内容 sha 的 hex 长度（`<sha>.egg` 命名面 · §13.2）。
	zegvKeyHexLen = 64
	// zegvIdxMaxBytes —— 命中档读索引体时的**分配上界**（✗ 别按前缀里给的长度裸分配）。
	zegvIdxMaxBytes = 64 << 20
	// zegvRefGBps —— 单流**有效**吞吐的参考值（片7 实测口径：22 GB / 42.3 s ≈ 0.53 GB/s）。
	// ★ 二用：① 探针拿不到读数时的兜底；② 探针读数的**上界**（`h = min(本机现测, 0.53)`）——
	// 机器更慢 ⇒ 自动多开 worker；机器更快 ⇒ ✗ 不越过片7 口径（D 与探针都是现测的，✗ 无本机硬编码常数）。
	zegvRefGBps = 0.53
)

// zegvProbeSink —— 探针结果的落点（✗ 别让编译器把那趟哈希省掉）。
var zegvProbeSink [sha256.Size]byte

// ---- 并发度 ---------------------------------------------------------------------------------

// zegvClampWorkers —— 把并发度 clamp 到 `[1, NumCPU]`（§16.7① 的 clamp 段）。
func zegvClampWorkers(w int) int {
	if w < 1 {
		return 1
	}
	if n := runtime.NumCPU(); w > n {
		return n
	}
	return w
}

// zegvProbeHashGBps —— 本机单流 sha256 吞吐（GB/s）**现测**（✗ 不硬编码机器常数）。
// 样本驻留内存（8 MiB × 2 趟）⇒ 量的是该机这条哈希路的速率，不掺盘速。
func zegvProbeHashGBps() float64 {
	buf := make([]byte, zegvProbeBytes)
	for i := range buf {
		buf[i] = byte(i)
	}
	const rounds = 2
	start := time.Now()
	for r := 0; r < rounds; r++ {
		zegvProbeSink = sha256.Sum256(buf)
	}
	el := time.Since(start).Seconds()
	if el <= 0 {
		return 0
	}
	return float64(len(buf)*rounds) / el / float64(int64(1)<<30)
}

// zegvWorkersFor —— `W = clamp(ceil(D/h), 1, NumCPU)`；`forced > 0`（`--workers N`）时以旗标为准（仍 clamp）。
//
// 推导（逐段写出，✗ 不留暗常数）：
//   - `D` ＝ 索引里各段 `size` 之和（现读，✗ 不硬编码机器）；
//   - `h` ＝ `min(本机现测单流 sha256 速率, 0.53 GB/s)`：
//     0.53 GB/s 是片7 对**真实 22 GB 卵**实测的单流有效速率（22 GB / 42.3 s）⇒ 当**上界**用：
//     本机比它快 ⇒ 仍按 0.53（✗ 不越过片7 口径）；本机比它慢（探针 < 0.53）⇒ 取探针值 ⇒ 自动多开一路；
//     探针异常（≤ 0）⇒ 直接取 0.53 兜底。
//   - `ceil(D/h)` ＝ 「这一趟大约值几秒单路活」⇒ 秒数即 worker 数；`clamp` 到 `[1, NumCPU]`。
//   - `dataBytes <= 0` ⇒ ceil(0) = 0 ⇒ clamp 到 1（零字节活别开多路）。
func zegvWorkersFor(dataBytes int64, hashGBps float64, forced int) int {
	if forced > 0 {
		return zegvClampWorkers(forced)
	}
	if dataBytes <= 0 {
		return 1
	}
	h := zegvRefGBps
	if hashGBps > 0 && hashGBps < h {
		h = hashGBps
	}
	w := int(math.Ceil(float64(dataBytes) / float64(int64(1)<<30) / h))
	return zegvClampWorkers(w)
}

// zegvDataBytes —— 本次要过哈希的数据字节数（＝索引里各段 `size` 之和）。
func zegvDataBytes(entries []zegEntryJSON) int64 {
	var n int64
	for _, e := range entries {
		if e.Size > 0 {
			n += e.Size
		}
	}
	return n
}

// ---- 逐段并行 sha256 --------------------------------------------------------------------------

// zegvHashSeg —— 在**同一个 fd** 上按段增量哈希。
// 语义与 `zegVerify` 的内联档同源：读不动（EOF/短读）⇒ 报「数据截断」（旧档靠 `m == 0` 判，同样把读错当截断）。
func zegvHashSeg(f *os.File, pos, size int64, buf []byte) (string, bool) {
	h := sha256.New()
	left := size
	for left > 0 {
		k := int64(len(buf))
		if left < k {
			k = left
		}
		n, err := f.ReadAt(buf[:k], pos)
		if n > 0 {
			h.Write(buf[:n])
			pos += int64(n)
			left -= int64(n)
		}
		if err != nil && n <= 0 {
			return "", true
		}
		if n == 0 {
			return "", true
		}
	}
	return hex.EncodeToString(h.Sum(nil)), false
}

// zegvVerifySeg —— 单段判据（**逐条**照 `zegVerify` 的内联档；✗ 别在这里加新判据 —— 加判据就要加不变量）。
func zegvVerifySeg(f *os.File, e zegEntryJSON, align, sizeFile int64, buf []byte) []string {
	var problems []string
	if e.Off%align != 0 {
		problems = append(problems, fmt.Sprintf("%s 偏移未对齐（%d %% %d != 0）", e.Path, e.Off, align))
	}
	if e.Off+e.Size > sizeFile {
		problems = append(problems, fmt.Sprintf("%s 越出文件末尾", e.Path))
		return problems
	}
	got, truncated := zegvHashSeg(f, e.Off, e.Size, buf)
	if truncated {
		problems = append(problems, fmt.Sprintf("%s 数据截断", e.Path))
		return problems
	}
	if got != e.SHA256 {
		problems = append(problems, fmt.Sprintf("%s sha256 不符", e.Path))
	}
	return problems
}

// zegvVerifyParallel —— 定长 worker pool 逐段并行；problems **按索引序**汇总 ⇒ 与单线程档逐字节一致。
func zegvVerifyParallel(f *os.File, entries []zegEntryJSON, align, sizeFile int64, workers int) []string {
	n := len(entries)
	if n == 0 {
		return nil
	}
	w := zegvClampWorkers(workers)
	if w > n {
		w = n
	}
	perSeg := make([][]string, n)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < w; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 池化：每 worker **一块**定长缓冲（W ≤ NumCPU ⇒ 总占用 ＝ W × 4 MiB，上界确定）。
			buf := make([]byte, zegvBufBytes)
			for k := range jobs {
				perSeg[k] = zegvVerifySeg(f, entries[k], align, sizeFile, buf)
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	var probs []string
	for i := 0; i < n; i++ {
		probs = append(probs, perSeg[i]...)
	}
	return probs
}

// ---- 内容寻址校验缓存 ------------------------------------------------------------------------

// zegvCacheEntry —— 缓存条目（值 ＝ 片7 §①-2 的「廉价复核束」＋ 命中档要印的读数）。
// ★ 只记**件名**不记路径（§13.2：主控只记 sha、不记路径 ⇒ 换机/换目录不影响）。
type zegvCacheEntry struct {
	Ver         int    `json:"ver"`
	Key         string `json:"key"`
	EggBase     string `json:"egg_base"`
	ToolVersion string `json:"tool_version"`
	Segments    int    `json:"segments"`
	Align       int64  `json:"align"`
	DataBytes   int64  `json:"data_bytes"`
	StDev       uint64 `json:"st_dev"`
	StIno       uint64 `json:"st_ino"`
	StSize      int64  `json:"st_size"`
	StMtimeNs   int64  `json:"st_mtime_ns"`
	PrefixSHA   string `json:"prefix_sha256"`
	IndexSHA    string `json:"index_sha256"`
	Workers     int    `json:"workers"`
	CheckedAt   string `json:"checked_at"`
}

// zegvPhys —— 从**同一个 fd** 现算的物理面（TOCTOU：`open`→`fstat`→`read` 同 fd）。
type zegvPhys struct {
	StDev     uint64
	StIno     uint64
	StSize    int64
	StMtimeNs int64
	PrefixSHA string
	IndexSHA  string
	IndexLen  int64
}

// zegvCacheDir —— 缓存目录（运行态面）：env 覆盖，缺省 `<userCache>/zerg/egg-verify-cache`。
func zegvCacheDir() (string, bool) {
	if d := strings.TrimSpace(os.Getenv(zegvCacheEnv)); d != "" {
		return d, true
	}
	d, err := os.UserCacheDir()
	if err != nil || d == "" {
		return "", false
	}
	return filepath.Join(d, "zerg", "egg-verify-cache"), true
}

// zegvKeyFromBase —— 键 ＝ 卵内容 sha：只认 `<64 位小写 hex>.egg` 的件名（§13.2 的内容寻址命名面）。
// ✗ 不按路径、✗ 不按时间建键；件名不是该形态 ⇒ **不启用缓存**（✗ 不拿路径凑一个键顶替）。
func zegvKeyFromBase(base string) (string, bool) {
	if len(base) != zegvKeyHexLen+len(".egg") || !strings.HasSuffix(base, ".egg") {
		return "", false
	}
	k := base[:zegvKeyHexLen]
	for i := 0; i < len(k); i++ {
		c := k[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return "", false
		}
	}
	return k, true
}

// zegvCachePath —— 条目落点（键前 2 位分目录，避免单目录塞爆）。
func zegvCachePath(dir, key string) string {
	return filepath.Join(dir, key[:2], key+".json")
}

// zegvReadCache —— 读条目；读不到 / JSON 坏 / 版本不符 / 工具版本不符 ⇒ `(nil, why)`。
// ★ `why` 只给人看：**判结论一律不依赖缓存**（未命中就照旧跑全量）。
func zegvReadCache(p string) (*zegvCacheEntry, string) {
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "无条目"
		}
		return nil, fmt.Sprintf("条目读不了：%v", err)
	}
	var e zegvCacheEntry
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, fmt.Sprintf("条目损坏：%v", err)
	}
	if e.Ver != zegvCacheVer {
		return nil, fmt.Sprintf("条目版本 %d ≠ %d", e.Ver, zegvCacheVer)
	}
	if e.ToolVersion != zegToolVersion {
		return nil, fmt.Sprintf("产出工具版本 %s ≠ %s", e.ToolVersion, zegToolVersion)
	}
	return &e, ""
}

// zegvWriteCache —— 原子落条目（临时名 ＋ `rename`）；★ 只在**绿 verdict**上调用。
func zegvWriteCache(p string, e *zegvCacheEntry) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp-%d", p, os.Getpid())
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// zegvStatIDs —— 取 `(st_dev, st_ino)`：两平台字段名一致（`Dev`/`Ino`），值类型不同 ⇒ 一律经 `uint64` 转换。
func zegvStatIDs(fi os.FileInfo) (uint64, uint64, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return uint64(st.Dev), uint64(st.Ino), true
}

// zegvReadPhys —— 由**同一个 fd** 现算物理面：32 B 前缀 ＋ 索引体的摘要。
// ★ 本处**只取长度、不判不变量** —— 真正的不变量校验仍在旧档 `zegParseIndex`（§11.2 第 6 条读写同源）；
// 偏移 16..24 ＝ §11.1 的 `index_len(u64 LE)`（同一份格式，由 `TestZergEggVerifyCacheFourStates` 之外的金标钉住）。
func zegvReadPhys(f *os.File, fi os.FileInfo) (zegvPhys, error) {
	var p zegvPhys
	dev, ino, ok := zegvStatIDs(fi)
	if !ok {
		return p, fmt.Errorf("本平台取不到 st_dev/st_ino")
	}
	p.StDev, p.StIno, p.StSize = dev, ino, fi.Size()
	p.StMtimeNs = fi.ModTime().UnixNano()
	prefix := make([]byte, zegHeaderPrefix)
	if _, err := f.ReadAt(prefix, 0); err != nil {
		return p, err
	}
	sum := sha256.Sum256(prefix)
	p.PrefixSHA = hex.EncodeToString(sum[:])
	n := int64(binary.LittleEndian.Uint64(prefix[16:24]))
	if n <= 0 || n > zegvIdxMaxBytes {
		return p, fmt.Errorf("索引长度字段不合理：%d", n)
	}
	body := make([]byte, n)
	got, err := f.ReadAt(body, zegHeaderPrefix)
	if err != nil {
		if got > 0 {
			return p, io.ErrUnexpectedEOF
		}
		return p, err
	}
	sum = sha256.Sum256(body)
	p.IndexSHA = hex.EncodeToString(sum[:])
	p.IndexLen = n
	return p, nil
}

// zegvCacheHitCheck —— 命中判定：复核束**全中**才算命中。
// ★ fail-closed 一条：`st_dev/st_ino/st_size/st_mtime_ns` 四项全同、而**前 32 B 摘要变了** ⇒ 判红（见件头注 5）。
func zegvCacheHitCheck(f *os.File, fi os.FileInfo, eggBase string, ent *zegvCacheEntry) (bool, string, string) {
	p, err := zegvReadPhys(f, fi)
	if err != nil {
		return false, fmt.Sprintf("物理面读不了：%v", err), ""
	}
	if p.StDev != ent.StDev || p.StIno != ent.StIno {
		return false, "st_dev/st_ino 变（换件或换路径）", ""
	}
	if p.StSize != ent.StSize {
		return false, "st_size 变", ""
	}
	if p.StMtimeNs != ent.StMtimeNs {
		return false, "st_mtime_ns 变", ""
	}
	if p.PrefixSHA != ent.PrefixSHA {
		red := fmt.Sprintf("%s 校验缓存复核失败（同 st_dev/st_ino/st_size/st_mtime_ns 而前 32 B 变 ⇒ 同键换件 / TOCTOU）", eggBase)
		return false, "", red
	}
	if p.IndexSHA != ent.IndexSHA {
		return false, "索引字节变", ""
	}
	return true, "", ""
}

// zegvShellIndex —— 命中档的**空壳索引**（只有 `Segments`/`Align` 有值；✗ 不解析索引 —— 见件头注 4）。
func zegvShellIndex(ent *zegvCacheEntry) *zegIndex {
	return &zegIndex{
		Magic:   zegMagic,
		Version: zegFormatVer,
		Align:   ent.Align,
		Entries: make([]zegEntryJSON, ent.Segments),
	}
}

// zegvNote —— 缓存面的**观测行**（走 stdout；✗ 不改判结论、✗ 不改退码）。
func zegvNote(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "· 校验缓存："+format+"\n", args...)
}

// zegvRebuild —— 按索引只读重建到目录（与旧档 `zegVerify` 的重建段同源；✗ 不改语义）。
func zegvRebuild(path string, idx *zegIndex, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	rf, err := os.Open(path)
	if err != nil {
		return err
	}
	defer rf.Close()
	for _, e := range idx.Entries {
		// ★ 卵1② 重建端（第二处）：落点走 `zegRebuildTarget`（路径规则表 ＋ 根内断言）＋
		// `zegOpenRebuildTarget`（`O_NOFOLLOW|O_EXCL`）—— **第二个可穿点也加固**（`§11.2` 新增第 8 条）。
		tgt, terr := zegRebuildTarget(dst, e.Path)
		if terr != nil {
			return terr
		}
		if err := os.MkdirAll(filepath.Dir(tgt), 0o755); err != nil {
			return err
		}
		if _, err := rf.Seek(e.Off, io.SeekStart); err != nil {
			return err
		}
		g, err := zegOpenRebuildTarget(tgt)
		if err != nil {
			return err
		}
		left := e.Size
		for left > 0 {
			k := int64(1 << 20)
			if left < k {
				k = left
			}
			buf := make([]byte, k)
			m, _ := rf.Read(buf)
			if m == 0 {
				break
			}
			if _, err := g.Write(buf[:m]); err != nil {
				g.Close()
				return err
			}
			left -= int64(m)
		}
		g.Close()
		if err := os.Chmod(tgt, os.FileMode(e.Mode)); err != nil {
			return err
		}
	}
	return nil
}

// ---- verify 子命令的唯一入口 ------------------------------------------------------------------

// zegvVerifyResult —— `verify` 要的三样：problems / 索引（命中档是空壳）/ 是否命中缓存。
type zegvVerifyResult struct {
	Problems []string
	Index    *zegIndex
	CacheHit bool
}

// zegvVerifyEntry —— `verify` 子命令的唯一入口：**并行逐段校验 ＋ 内容寻址缓存**（`--rebuild-into` 语义照旧）。
// 流程：① 命中判定（只比字节摘要 · 不解析索引）⇒ 命中即回；
//
//	② 未命中 ⇒ 并行全量校验（结论只由它给）⇒ ③ 绿 ＋ 复核束前后一致 ⇒ 才落条目。
func zegvVerifyEntry(stdout io.Writer, path, rebuildInto string, workers int) (zegvVerifyResult, error) {
	var res zegvVerifyResult
	f, err := os.Open(path)
	if err != nil {
		return res, err
	}
	defer f.Close()
	fi, err := f.Stat() // ★ 对**同一个 fd** fstat（✗ 不按路径二次取数）
	if err != nil {
		return res, err
	}
	sizeFile := fi.Size()
	eggBase := filepath.Base(path)
	key, keyKnown := zegvKeyFromBase(eggBase)
	cacheDir, dirOK := zegvCacheDir()
	useCache := keyKnown && dirOK
	var cachePath string
	if useCache {
		cachePath = zegvCachePath(cacheDir, key)
	}

	// ---- ① 命中判定 ----
	missWhy := ""
	if useCache {
		ent, why := zegvReadCache(cachePath)
		switch {
		case ent == nil:
			missWhy = why
		case ent.Key != key:
			missWhy = fmt.Sprintf("条目键 %s ≠ 本卵键 %s", zegvShort(ent.Key), zegvShort(key))
		default:
			hit, why, red := zegvCacheHitCheck(f, fi, eggBase, ent)
			if red != "" {
				// ★ fail-closed：同键换件 ⇒ 判红（✗ 不当未命中放行、✗ 不得静默复用）
				res.Problems = []string{red}
				res.Index = zegvShellIndex(ent)
				return res, nil
			}
			if hit {
				res.Index = zegvShellIndex(ent)
				res.CacheHit = true
				return res, nil
			}
			missWhy = why
		}
	}

	// ---- ② 全量并行校验（结论的唯一来源）----
	hashGBps := 0.0
	if workers <= 0 {
		hashGBps = zegvProbeHashGBps() // 只在自动档探针；`--workers N` 给了就省掉这一趟
	}
	idx, perr := zegParseIndex(path) // 读端仍是旧档唯一实现（§11.2 第 6 条读写同源）
	if perr != nil {
		return res, perr
	}
	dataBytes := zegvDataBytes(idx.Entries)
	w := zegvWorkersFor(dataBytes, hashGBps, workers)
	if useCache {
		if missWhy == "" {
			missWhy = "复核束未全中"
		}
		zegvNote(stdout, "未命中（%s） ⇒ 并行全量重校（W=%d · D=%d B%s）", missWhy, w, dataBytes, zegvProbeTail(hashGBps))
	} else if !keyKnown {
		zegvNote(stdout, "未启用（件名不是 <卵内容 sha>.egg ⇒ 无内容寻址键 · ✗ 不按路径/时间建键） ⇒ 并行全量重校（W=%d · D=%d B%s）", w, dataBytes, zegvProbeTail(hashGBps))
	} else {
		zegvNote(stdout, "未启用（取不到缓存目录 ⇒ %s） ⇒ 并行全量重校（W=%d · D=%d B%s）", zegvCacheEnv+" 未设且无用户缓存目录", w, dataBytes, zegvProbeTail(hashGBps))
	}
	probs := zegvVerifyParallel(f, idx.Entries, idx.Align, sizeFile, w)
	res.Problems = probs
	res.Index = idx
	if rebuildInto != "" {
		if err := zegvRebuild(path, idx, rebuildInto); err != nil {
			return res, err
		}
	}
	if len(probs) != 0 {
		return res, nil // 红 verdict **不落条目**（✗ 不缓存红）
	}

	// ---- ③ 绿 ⇒ 落条目（复核束前后一致才落 —— 校验期间物理面变了就不落）----
	if useCache {
		before, berr := zegvReadPhys(f, fi)
		after, aerr := zegvReadPhys(f, fi)
		switch {
		case berr != nil || aerr != nil:
			zegvNote(stdout, "本次未落条目（复核束现算失败）")
		case before != after:
			zegvNote(stdout, "本次未落条目（校验期间物理面变了：st_mtime_ns/size/摘要不等）")
		default:
			ent := &zegvCacheEntry{
				Ver:         zegvCacheVer,
				Key:         key,
				EggBase:     eggBase,
				ToolVersion: zegToolVersion,
				Segments:    len(idx.Entries),
				Align:       idx.Align,
				DataBytes:   dataBytes,
				StDev:       before.StDev,
				StIno:       before.StIno,
				StSize:      before.StSize,
				StMtimeNs:   before.StMtimeNs,
				PrefixSHA:   before.PrefixSHA,
				IndexSHA:    before.IndexSHA,
				Workers:     w,
				CheckedAt:   time.Now().UTC().Format("2006-01-02T15:04:05") + "Z",
			}
			if werr := zegvWriteCache(cachePath, ent); werr != nil {
				zegvNote(stdout, "本次未落条目（%v）", werr)
			}
		}
	}
	return res, nil
}

// zegvProbeTail —— 观测行尾注：自动档带探针读数，`--workers` 档不带（那一趟没跑探针）。
func zegvProbeTail(hashGBps float64) string {
	if hashGBps <= 0 {
		return " · 并发档＝--workers"
	}
	return fmt.Sprintf(" · 单流 sha256 探针 %.2f GB/s", hashGBps)
}

// zegvShort —— sha 前 12 位（给人看的短形；✗ 不参与判据）。
func zegvShort(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// ---- 自检（卵3① 面：并行 ＋ 缓存）------------------------------------------------------------

// zegvSelfTest —— 卵3① 的自检（3 正例 ＋ 4 负例）。
// ★ 与旧档 `zegSelfTest`（py 真搬面 · 10 项）**分开跑、分开报**：旧档那行是钉住的契约读数，✗ 不动它。
func zegvSelfTest(stdout, stderr io.Writer) int {
	ok := 0
	t, err := os.MkdirTemp("", "zerg-egg-verify-selftest")
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", zegProg, err)
		return exitFail
	}
	defer os.RemoveAll(t) // 自回收（TMPDIR 前后 delta=0）
	oldCache := os.Getenv(zegvCacheEnv)
	if err := os.Setenv(zegvCacheEnv, filepath.Join(t, "cache")); err != nil {
		return zegSelfFail(stderr, err)
	}
	defer os.Setenv(zegvCacheEnv, oldCache)

	build := func(name string) (string, error) {
		src := filepath.Join(t, name+".src")
		if err := os.MkdirAll(filepath.Join(src, "model"), 0o755); err != nil {
			return "", err
		}
		buf := make([]byte, 300000)
		for i := range buf {
			buf[i] = byte(i * 7)
		}
		if err := os.WriteFile(filepath.Join(src, "model", "w.safetensors"), buf, 0o644); err != nil {
			return "", err
		}
		// ★ 内容里带上 name ⇒ 每一份夹具的**内容寻址件名**互不相同（否则同名件会互相覆盖 —— 实测踩过）
		cfg := fmt.Sprintf(`{"arch":"selftest","tag":%q}`, name)
		if err := os.WriteFile(filepath.Join(src, "config.json"), []byte(cfg), 0o644); err != nil {
			return "", err
		}
		tmp := filepath.Join(t, name+".egg")
		if _, err := zegPack(src, tmp, 4096, ""); err != nil {
			return "", err
		}
		d, err := os.ReadFile(tmp)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(d)
		// §13.2 内容寻址命名面：件名 ＝ <卵内容 sha>.egg
		final := filepath.Join(t, hex.EncodeToString(sum[:])+".egg")
		if err := os.Rename(tmp, final); err != nil {
			return "", err
		}
		return final, nil
	}
	// 正例①：并行（W=4）与单线程 `zegVerify` 结论**逐字节同**
	egg, err := build("a")
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	var o1 strings.Builder
	res, err := zegvVerifyEntry(&o1, egg, "", 4)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	probs1, _, err := zegVerify(egg, "")
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if strings.Join(res.Problems, "\n") != strings.Join(probs1, "\n") {
		return zegSelfFail(stderr, fmt.Errorf("并行档与单线程档结论不同：%v ⟷ %v", res.Problems, probs1))
	}
	ok++
	// 正例②：冷跑落条目 ⇒ 二次跑**命中缓存**（且结论仍为空）
	var o2 strings.Builder
	res2, err := zegvVerifyEntry(&o2, egg, "", 4)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if !res2.CacheHit {
		return zegSelfFail(stderr, fmt.Errorf("二次跑未命中缓存（首跑该已落条目）"))
	}
	if len(res2.Problems) != 0 {
		return zegSelfFail(stderr, fmt.Errorf("命中缓存却给出 problems：%v", res2.Problems))
	}
	ok++
	// 正例③：自动并发档（`--workers` 缺省 ⇒ 探针 ＋ 公式）结论同、且并发度落在 [1, NumCPU]
	var o3 strings.Builder
	res3, err := zegvVerifyEntry(&o3, egg, "", 0)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if len(res3.Problems) != 0 {
		return zegSelfFail(stderr, fmt.Errorf("自动档给出 problems：%v", res3.Problems))
	}
	if w := zegvWorkersFor(1<<30, 1.0, 0); w != 2 {
		return zegSelfFail(stderr, fmt.Errorf("并发度公式不合：1 GiB ⇒ ceil(1/0.53)=2，实得 %d", w))
	}
	if w := zegvWorkersFor(0, 1.0, 0); w != 1 {
		return zegSelfFail(stderr, fmt.Errorf("零字节活应取 1，实得 %d", w))
	}
	ok++
	// 负例①：改数据段一字节（自然写 ⇒ mtime 走）⇒ 必须**必红**且**缓存被击穿**
	bad, err := build("b")
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	var ow strings.Builder
	if _, err := zegvVerifyEntry(&ow, bad, "", 4); err != nil {
		return zegSelfFail(stderr, err)
	}
	d, err := os.ReadFile(bad)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	bidx, err := zegParseIndex(bad)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if len(bidx.Entries) == 0 {
		return zegSelfFail(stderr, fmt.Errorf("自检夹具卵没有段"))
	}
	// ★ 改**段内**一字节（首段 off+3）：✗ 别改 `d[len(d)-1]` —— 末段之后常有**补零到 align** 的 pad，
	// 改 pad 不破任何段 sha（那一档本来就判绿）⇒ 自检会假红。
	hit := bidx.Entries[0].Off + 3
	if hit+1 > int64(len(d)) {
		return zegSelfFail(stderr, fmt.Errorf("自检夹具卵太短：%d", len(d)))
	}
	d[hit] ^= 0xff
	if err := os.WriteFile(bad, d, 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	var o4 strings.Builder
	res4, err := zegvVerifyEntry(&o4, bad, "", 4)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if res4.CacheHit {
		return zegSelfFail(stderr, fmt.Errorf("改了字节却仍命中缓存"))
	}
	if !zegvHasSHA256Problem(res4.Problems) {
		return zegSelfFail(stderr, fmt.Errorf("改了字节却没报 sha256 不符：%v", res4.Problems))
	}
	ok++
	// 负例②：改**前 32 B**（magic）＋ **复原 mtime** ⇒ 必须必红（★ 这条就是负控锚：哑化「前 32 B 复核」⇒ 本格绿）
	// ★ 用**专属**一份卵：本格会把卵改坏（正例/负例③④ 还要用那份干净的）。
	eggTamper, err := build("c")
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if rc := zegvPrefixTamperCase(stderr, eggTamper); rc != 0 {
		return rc
	}
	ok++
	// 负例③：条目损坏 ⇒ 不得误报绿（结论只由全量校验给）
	key, _ := zegvKeyFromBase(filepath.Base(egg))
	cdir, _ := zegvCacheDir()
	cp := zegvCachePath(cdir, key)
	if err := os.WriteFile(cp, []byte("{ 这不是 JSON"), 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	var o5 strings.Builder
	res5, err := zegvVerifyEntry(&o5, egg, "", 4)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if res5.CacheHit {
		return zegSelfFail(stderr, fmt.Errorf("条目坏了却报命中"))
	}
	if len(res5.Problems) != 0 {
		return zegSelfFail(stderr, fmt.Errorf("条目坏了把干净卵判红：%v", res5.Problems))
	}
	ok++
	// 负例④：条目**键不符**（拿别键的绿顶到本键位）⇒ 不得命中
	other := &zegvCacheEntry{Ver: zegvCacheVer, Key: strings.Repeat("0", 64), ToolVersion: zegToolVersion, Segments: 1, Align: 4096}
	if err := zegvWriteCache(cp, other); err != nil {
		return zegSelfFail(stderr, err)
	}
	var o6 strings.Builder
	res6, err := zegvVerifyEntry(&o6, egg, "", 4)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if res6.CacheHit {
		return zegSelfFail(stderr, fmt.Errorf("键不符却报命中"))
	}
	ok++
	fmt.Fprintf(stdout, "self-test（卵3① 并行＋缓存）通过（%d 项：3 正例 + 4 负例）\n", ok)
	return exitOK
}

// zegvPrefixTamperCase —— 「改前 32 B ＋ 复原 mtime ⇒ 必红」这一格；返回非 0 ⇒ 失败码。
// ★ 本格**会把传入的卵改坏**（magic 被改）⇒ 调用方必须给一份专属的卵。
func zegvPrefixTamperCase(stderr io.Writer, egg string) int {
	// ① 先冷跑一次（**必绿** ⇒ 落条目）—— 本格的前提：命中档真的跑起来（✗ 少了这步就退化成普通未命中/解析报错）
	var o0 strings.Builder
	res0, err := zegvVerifyEntry(&o0, egg, "", 4)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if len(res0.Problems) != 0 {
		return zegSelfFail(stderr, fmt.Errorf("本格前提：冷跑必须绿，实得 %v", res0.Problems))
	}
	d, err := os.ReadFile(egg)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	fi, err := os.Stat(egg)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	d[3] = 'H' // magic "ZEGG" → "ZEGH"（落在前 32 B 内；命中档**不解析** ⇒ 只此一条牙咬住）
	if err := os.WriteFile(egg, d, 0o644); err != nil {
		return zegSelfFail(stderr, err)
	}
	if err := os.Chtimes(egg, fi.ModTime(), fi.ModTime()); err != nil {
		return zegSelfFail(stderr, err)
	}
	var o strings.Builder
	res, err := zegvVerifyEntry(&o, egg, "", 4)
	if err != nil {
		return zegSelfFail(stderr, err)
	}
	if res.CacheHit {
		return zegSelfFail(stderr, fmt.Errorf("前 32 B 被改（mtime 已复原）却仍报命中缓存"))
	}
	if len(res.Problems) == 0 {
		return zegSelfFail(stderr, fmt.Errorf("前 32 B 被改（mtime 已复原）却没报红"))
	}
	return 0
}

// zegvHasSHA256Problem —— problems 里有没有「sha256 不符」（照旧档自检负例①的判法）。
func zegvHasSHA256Problem(probs []string) bool {
	for _, p := range probs {
		if strings.Contains(p, "sha256") {
			return true
		}
	}
	return false
}
