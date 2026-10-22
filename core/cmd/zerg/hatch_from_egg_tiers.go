// hatch_from_egg_tiers.go —— `GAP-20261015-25` 的**孵化前闸**：D5 / D6a / D10 / D11 / D12 五档。
//
// 缺口（账内 `GAP-20261015-25`）：卵族 13 档里 D5 / D6a / D8 / D9 / D10 / D11 / D12 七档未逐档接线。
// 本件只落**能在本机判、不依赖真机/引擎**的五档（D8 见件尾「未落」一节）：
//
//	档位  触发                                                      报话/退码
//	D5    契约段缺必需字段（维度/绑定/限额任一为 nil 或缺项）        取 `zegVersionTiers` D5 行（2）
//	D6a   **必需**段（引擎/权重/契约）的 mediaType 本机不认识        取 D6a 行（2）
//	D10   卵本体半成品（缺完工标记 / 临时名未 rename）               取 D10 行（8）
//	D11   孵化前磁盘不足（水线硬线已到 / 展开所需 > 现读可用）       取 D11 行（10）
//	D12   卵内茧壁 ⟷ 机器茧壁不匹配 ⇒ 改用卵内那份（不报错）         取 D12 行（0）
//
// 三条口径（照设计稿，✗ 不自由发挥）：
//
//	① **D6a 的「必需」不由孵化器猜**：段的必需性从 `.path` 推出的 §9.1 五段类来（`zegSegOfPath`）——
//	   引擎段 / 权重段 / 契约段 = 孵化必需；配方段 = 非必需（照 OCI「MUST NOT error」⇒ 跳过并登记，即 D6b）。
//	   ★ 「未标 mediaType」**不**触发 D6a（那是 D6b/登记面的事）：D6a 的触发词是「本机不认识」，不是「没声明」。
//	② **D5 的必需字段真源在 `agent/internal/enclosure/contract.go` 的 `Contract.Validate()`**（「缺项必拒」）；
//	   core 模块**导不到** agent 模块（`core/go.mod` 无该 require，`hatch_from_egg.go` 件头第 2 条已记）
//	   ⇒ 本件落一份**读端镜像**，并由 `TestHgtContractSourceParity` 现读那份真源逐条对拍（真源改了 ⇒ 当场红）。
//	③ **D12 只判「用哪一份」，不搬运茧壁**：判定用 V1 三段判（机器那份 ≥ 卵声明 `wall_min_version`
//	   且同主号 ⇒ 用机器那份；否则用卵内那份），结论写进交接件（观测面标注），**退码 0（不报错）**。
//	   ★ 机器已装茧壁的**版本探测**属真机面（`hgtMachineWallVersion` 默认只认 env 旋钮 / 未装）——
//	   本件落的是判定与标注，探测口留注入位。
//
// 出厂声明：
//
//	(a) 本件**不扫仓内树、不写仓内件**（全部只读：读卵件字节 / 读名册字段）⇒ 无自扫描面。
//	(b) 全件**零绝对路径字面量**；契约段名是**卵内相对路径**（`zegIndex.Entries[].Path` 的比较面）。
//
// 出厂声明结束。接线点：`hatchFromEgg`（`hatch_from_egg.go` 的 ①校验之后 / ②门 / ③展开之前）。
//
// ---- 未落的一档（如实写，✗ 不假装）-------------------------------------------------------------
//
//	D8（签名不匹配 / v2 卵缺签名）：**需真机 ＋ 待他面** —— ① 索引里**没有签名块**（`GAP-20261015-14`
//	未落，`zegAdditiveKeyExamples` 里只有构建自证四字段）；② 判决要跑真 `cosign verify-blob`
//	（`egg_sign.go` 只组装 argv，✗ 不真签/不真验）。两条都不在本单可达范围 ⇒ 本件不落 D8。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/store/eggstore"
)

// ─────────────────────────── D5 · 契约段缺必需字段 ───────────────────────────

// hgtContractNames —— **契约段**的件名闭集（卵内相对路径 · 小写比较）。
//
// ★ 刻意**严于** `zegSegOfPath` 的「契约段」类：那一类把 `config.json` / `policy.json` 也算进去
// （配方类语义最保守地归到配置档），而 D5 判的是**引擎能力契约**这一份 —— 拿 `config.json` 当契约判
// 会把「模型配方」判成「契约缺十个维度」。⇒ 只认契约件的正式名。
var hgtContractNames = []string{
	"contract/contract.json",
	"contract.json",
	"enclosure.json",
	"contract/enclosure.json",
}

// hgtContractDims —— 契约的**十个维度**（镜像 `agent/internal/enclosure/contract.go` 的 `Contract` 字段）。
// ★ 顺序照真源结构体字段序；`TestHgtContractSourceParity` 现读真源逐条核这十格。
var hgtContractDims = []string{
	"filesystem", "ipc", "exec", "devices", "network",
	"resources", "readiness", "lifecycle", "dependencies", "seccomp",
}

// hgtContractMaxBytes —— 契约段可读上界（4 MiB）：超界 ⇒ 不读（✗ 不按索引里的大数去 alloc）。
const hgtContractMaxBytes = 4 << 20

// hgtContractEntry —— 在段表里找契约段（找到 ⇒ 返回该段；找不到 ⇒ ok=false）。
func hgtContractEntry(idx *zegIndex) (zegEntryJSON, bool) {
	for _, e := range idx.Entries {
		low := strings.ToLower(filepath.ToSlash(e.Path))
		for _, n := range hgtContractNames {
			if low == n {
				return e, true
			}
		}
	}
	return zegEntryJSON{}, false
}

// hgtContractGate —— D5：卵里带契约段 ⇒ 逐项判「必需字段」；不带 ⇒ 本档不介入（历史卵形态）。
//
// ★ 「不带契约段」**不**拒：D5 的触发是「契约段缺必需字段」，不是「没有契约段」（后者属另一档/另一面）。
func hgtContractGate(eggPath string, idx *zegIndex) error {
	e, ok := hgtContractEntry(idx)
	if !ok {
		return nil
	}
	if e.Size < 0 || e.Size > hgtContractMaxBytes {
		return zegTierRefuse("D5", fmt.Sprintf(
			"契约段 %q 的 size=%d 超出可读上界 %d（✗ 不按索引里的大数 alloc）", e.Path, e.Size, hgtContractMaxBytes))
	}
	f, err := os.Open(eggPath)
	if err != nil {
		return fmt.Errorf("契约段读不出来（打开卵件）：%w", err)
	}
	defer f.Close()
	buf := make([]byte, e.Size)
	if _, err := f.ReadAt(buf, e.Off); err != nil {
		return fmt.Errorf("契约段 %q 读不满（off=%d size=%d）：%w", e.Path, e.Off, e.Size, err)
	}
	return hgtCheckContract(buf, e.Path)
}

// hgtCheckContract —— 契约字节 → D5 判决（**缺项必拒，不默默取默认值**）。
//
// 判据逐条对齐真源 `Contract.Validate()`：十个维度任一为 nil/缺失 ⇒ 缺项；`filesystem.scratch_dir` 空、
// `filesystem.ro_binds[]` 的 host/space 不全、`readiness.probe` 不在闭集 ⇒ 同样算缺项（真源也这么判）。
func hgtCheckContract(data []byte, where string) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return zegTierRefuse("D5", fmt.Sprintf(
			"契约段 %s 不是 JSON 对象（缺项必拒，✗ 不默默取默认值）：%v", where, err))
	}
	var missing []string
	for _, d := range hgtContractDims {
		raw, ok := m[d]
		if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || len(bytes.TrimSpace(raw)) == 0 {
			missing = append(missing, d)
		}
	}
	if raw, ok := m["filesystem"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		var fs map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fs); err != nil {
			missing = append(missing, "filesystem（不可解析）")
		} else {
			var scratch string
			if r, ok := fs["scratch_dir"]; !ok || json.Unmarshal(r, &scratch) != nil || strings.TrimSpace(scratch) == "" {
				missing = append(missing, "filesystem.scratch_dir（空间内可写临时区必须显式声明）")
			}
			if r, ok := fs["ro_binds"]; ok {
				var binds []struct {
					Host  string `json:"host"`
					Space string `json:"space"`
				}
				if err := json.Unmarshal(r, &binds); err != nil {
					missing = append(missing, "filesystem.ro_binds（不可解析）")
				} else {
					for i, b := range binds {
						if strings.TrimSpace(b.Host) == "" || strings.TrimSpace(b.Space) == "" {
							missing = append(missing, fmt.Sprintf("filesystem.ro_binds[%d]（host/space 不全）", i))
						}
					}
				}
			}
		}
	}
	if raw, ok := m["readiness"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		var rd struct {
			Probe string `json:"probe"`
		}
		if err := json.Unmarshal(raw, &rd); err != nil {
			missing = append(missing, "readiness（不可解析）")
		} else {
			switch rd.Probe {
			case "http", "stdio", "log", "file":
			default:
				missing = append(missing, fmt.Sprintf("readiness.probe=%q（须为 http|stdio|log|file）", rd.Probe))
			}
		}
	}
	if len(missing) != 0 {
		return zegTierRefuse("D5", fmt.Sprintf(
			"契约段 %s 缺项 %d 处：%s（引擎入口/绑定/限额任一为 nil ⇒ 拒，✗ 不默默取默认值）",
			where, len(missing), strings.Join(missing, " / ")))
	}
	return nil
}

// ─────────────────────────── D6a / D6b · 段 mediaType ───────────────────────────

// hgtEssentialSeg —— 该段**是否为孵化必需**（D6a 与 D6b 的唯一分界）。
//
// 必需 = 没有它引擎起不来：引擎段（二进制/依赖库）、权重段（模型本体）、契约段（封闭策略与绑定）。
// 非必需 = 配方段（适配器/参数/标定档案）：缺了能降级，故照 OCI 口径「跳过并登记」。
func hgtEssentialSeg(seg string) bool {
	switch seg {
	case "引擎段", "权重段", "契约段":
		return true
	}
	return false
}

// hgtMediaTypeGate —— D6a（必需段类型本机不认识 ⇒ 拒）＋ D6b（非必需段 ⇒ 跳过并登记）。
//
// ★ 触发面只有**已声明却在闭集外**的 mediaType；「未标」不入这两档（读端已登记在 `idx.UnlabeledMedia`）。
func hgtMediaTypeGate(idx *zegIndex) ([]string, error) {
	closed := zegMediaTypeClosedSet()
	var notes []string
	for _, e := range idx.Entries {
		mt := strings.TrimSpace(e.MediaType)
		if mt == "" {
			continue
		}
		if _, known := closed[mt]; known {
			continue
		}
		seg := zegSegOfPath(e.Path)
		if hgtEssentialSeg(seg) {
			return nil, zegTierRefuse("D6a", fmt.Sprintf(
				"段 %q 属%s（孵化必需）却声明 mediaType=%q —— 本机不认识 ⇒ 拒，✗ 不猜着跑",
				e.Path, seg, mt))
		}
		notes = append(notes, fmt.Sprintf("%s（%s）mediaType=%s ⇒ 跳过并登记", e.Path, seg, mt))
	}
	return notes, nil
}

// ─────────────────────────── D10 · 半成品 ───────────────────────────

// hgtPartSibling —— 「临时名未 rename」的证据：卵件旁边还有同名临时件。
//
// 照 §15.4 L12：「写临时名、逐块校验全过才 rename 成 <sha>.egg」⇒ 临时件还在 = 这枚卵没走完落地。
func hgtPartSibling(eggPath string) (string, bool) {
	base := filepath.Base(eggPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Dir(eggPath)
	cands := []string{
		eggPath + ".part",
		eggPath + ".tmp",
		filepath.Join(dir, stem+".part"),
		filepath.Join(dir, stem+".tmp"),
		filepath.Join(dir, "."+base+".part"),
	}
	for _, c := range cands {
		if fi, err := os.Lstat(c); err == nil && !fi.IsDir() {
			return c, true
		}
	}
	return "", false
}

// hgtHalfRefuse —— D10：临时件未 rename ／ 完工标记未置位，任一命中即拒（码 8 = 有 BLOCKED）。
func hgtHalfRefuse(eggPath, name string, complete bool) error {
	if part, ok := hgtPartSibling(eggPath); ok {
		return zegTierRefuse("D10", fmt.Sprintf(
			"卵 %q 旁边还有未 rename 的临时件 %s（写临时名、逐块校验全过才 rename ⇒ 这枚没走完落地）", name, part))
	}
	if !complete {
		return zegTierRefuse("D10", fmt.Sprintf(
			"卵 %q 的完工标记未置位（★ 文件存在 ≠ 可用：中断留下的半成品一律视为不可用）", name))
	}
	return nil
}

// ─────────────────────────── D11 · 可用磁盘不足 ───────────────────────────

// hgtNeedBytes —— 这枚卵展开要占的字节数（＝段表里 payload 之和）。
// ★ 只算 payload：补零/对齐开销是上界噪声，水线档判的是「装不装得下」。
func hgtNeedBytes(idx *zegIndex) int64 {
	var n int64
	for _, e := range idx.Entries {
		if e.Size > 0 {
			n += e.Size
		}
	}
	return n
}

// hgtHumanBytes —— 字节数人面（GiB / MiB / KiB / B 四档，✗ 不打科学计数）。
func hgtHumanBytes(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	var s string
	switch {
	case n >= 1<<30:
		s = fmt.Sprintf("%.2f GiB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		s = fmt.Sprintf("%.1f MiB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		s = fmt.Sprintf("%.1f KiB", float64(n)/float64(1<<10))
	default:
		s = fmt.Sprintf("%d B", n)
	}
	if neg {
		return "-" + s
	}
	return s
}

// hgtDiskGate —— D11：水线/硬线已到 **或** 本卵展开所需 > 现读可用 ⇒ 拒（码 10 = 资源不足）。
//
// ★ 两条都归 D11 且都**出声**：水线是「盘面整体不够」，shortfall 是「这一枚装不下」；
// 报话逐字取 13 档表 D11 行（`%s` = 还差多少 · `%s` = 可回收卵清单）。
func hgtDiskGate(pre eggstore.Precheck, need int64) error {
	// ★ 体积读数缺失（`Bsize` 为 0：注入口/探针给的 Sample 没有块大小）⇒ **不按容量判**，
	// 只按水线判词（✗ 把「算不出来」当成「可用 0 B」—— 那会把静默哑化的注入判据判成有牙）。
	known := pre.ImageFS.Bsize > 0 && pre.NodeFS.Bsize > 0
	var avail uint64
	var short int64
	if known {
		avail = pre.ImageFS.AvailableBytes()
		if a := pre.NodeFS.AvailableBytes(); a < avail {
			avail = a
		}
		if need > int64(avail) {
			short = need - int64(avail)
		}
	}
	if pre.Verdict != eggstore.VerdictRefuse && short == 0 {
		return nil
	}
	needTxt := hgtHumanBytes(short)
	if short == 0 {
		// 水线/硬线已到（容量本身够这枚卵）—— 报话模板照旧，填值如实说「不是容量差」。
		needTxt = "水线/硬线已到（非本卵容量差）"
	}
	detail := fmt.Sprintf("本卵展开需 %s · 现读可用 %s（imagefs %s / nodefs %s）· 判词 %s",
		hgtHumanBytes(need), hgtHumanBytes(int64(avail)),
		pre.ImageFS.Label, pre.NodeFS.Label, pre.Verdict)
	if !known {
		detail = fmt.Sprintf("体积读数缺失（Bsize=0 ⇒ 容量面判不了，只按水线判词）· 本卵展开需 %s · 判词 %s",
			hgtHumanBytes(need), pre.Verdict)
	}
	if len(pre.Reasons) != 0 {
		detail = strings.Join(pre.Reasons, "；") + " · " + detail
	}
	ref := zegTierRefuse("D11", detail, needTxt, hfeListOrNone(pre.Reclaimable))
	return hfeFromTierRefusal(hfeStageWaterline, ref)
}

// ─────────────────────────── D12 · 茧壁：机器那份 ⟷ 卵内那份 ───────────────────────────

// hgtMachineWallVersion —— 机器**已装**茧壁的版本探测口（★ 真机面）。
//
// 默认实现只认环境旋钮 `ZERG_WALL_VERSION`（离线 / CI / 未装 ⇒ 未装）。真机上要改成读装好的
// 茧壁件自己的版本（`wall` 制品的版本真源）；本件**不猜**盘上有没有茧壁 —— 探测不到就叫「未装」。
var hgtMachineWallVersion = func() (string, bool) {
	v := strings.TrimSpace(os.Getenv("ZERG_WALL_VERSION"))
	if v == "" {
		return "", false
	}
	return v, true
}

// hgtHasBundledWall —— 卵里**带没带**茧壁段（能兜底的前提）：`wall/` 下的段或段基本名是茧壁件。
func hgtHasBundledWall(idx *zegIndex) bool {
	for _, e := range idx.Entries {
		low := strings.ToLower(filepath.ToSlash(e.Path))
		base := filepath.Base(low)
		if strings.HasPrefix(low, "wall/") || strings.Contains(low, "/wall/") ||
			base == "zerg-wall" || base == "wall" || base == "zerg-wall-bin" {
			return true
		}
	}
	return false
}

// hgtParseVer —— `v1.2.3` / `1.2` / `1` 一律吃（缺位补 0）；`-dirty` / `+build` 尾巴忽略。
func hgtParseVer(s string) (maj, min, pat int, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return 0, 0, 0, false
	}
	parts := strings.Split(s, ".")
	nums := make([]int, 0, 3)
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || v < 0 {
			return 0, 0, 0, false
		}
		nums = append(nums, v)
	}
	for len(nums) < 3 {
		nums = append(nums, 0)
	}
	return nums[0], nums[1], nums[2], true
}

// hgtVerLess —— a < b（三段逐位比；判不了 ⇒ 报「不小于」（调用方按 fail-closed 另判））。
func hgtVerLess(a, b string) bool {
	am, an, ap, aok := hgtParseVer(a)
	bm, bn, bp, bok := hgtParseVer(b)
	if !aok || !bok {
		return false
	}
	if am != bm {
		return am < bm
	}
	if an != bn {
		return an < bn
	}
	return ap < bp
}

// hgtWallGate —— D12 的 V1 三段判（§十九 #32）：机器那份 **≥ 卵声明 `wall_min_version` 且同主号** ⇒
// 用机器那份；否则**改用卵内那份**（不报错 · 退码 0 · 结论写进交接件＝观测面标注）。
//
// 返回 (choice, note, err)：choice ∈ {"machine", "egg", ""}（空 = 两份都用不了 ⇒ 出声拒）。
func hgtWallGate(idx *zegIndex) (string, string, error) {
	bundled := hgtHasBundledWall(idx)
	min := strings.TrimSpace(idx.WallMin)
	mv, installed := hgtMachineWallVersion()

	useEgg := func(why string) (string, string, error) {
		if !bundled {
			return "", "", fmt.Errorf("%s，且卵内**没有**茧壁段 ⇒ 两份都用不了（须升级机器茧壁，✗ 不静默用旧版）", why)
		}
		return "egg", "D12：" + why + " ⇒ 改用卵内那份；已在观测面标注（交接件 wall=egg）", nil
	}

	if min == "" {
		if installed {
			return "machine", fmt.Sprintf("D12：卵未声明 wall_min_version ⇒ 用机器那份 %s", mv), nil
		}
		if !bundled {
			// 历史卵形态：没声明、机器没装、卵里也没有 ⇒ 本档不介入（✗ 不为不存在的两份报错）。
			return "", "D12：机器未装茧壁、卵内无茧壁段、卵亦未声明最低版 ⇒ 本档不介入", nil
		}
		return "egg", "D12：机器未装茧壁且卵未声明最低版 ⇒ 用卵内那份；已在观测面标注（交接件 wall=egg）", nil
	}
	if !installed {
		return useEgg(fmt.Sprintf("机器未装茧壁（卵声明最低 %s）", min))
	}
	majM, _, _, okM := hgtParseVer(mv)
	majE, _, _, okE := hgtParseVer(min)
	if !okM || !okE {
		return useEgg(fmt.Sprintf("版本串判不了（机器 %q / 卵声明最低 %q）", mv, min))
	}
	if majM != majE {
		return useEgg(fmt.Sprintf("主号不同（机器 %s 主号 %d ⟷ 卵声明最低 %s 主号 %d）", mv, majM, min, majE))
	}
	if hgtVerLess(mv, min) {
		return useEgg(fmt.Sprintf("机器茧壁 %s < 卵声明最低 %s", mv, min))
	}
	return "machine", fmt.Sprintf("D12：机器茧壁 %s ≥ 卵声明最低 %s 且同主号（%d）⇒ 用机器那份", mv, min, majM), nil
}
