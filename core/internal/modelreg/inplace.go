package modelreg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ── 待修补 #3（高）+ #17（中）：写入权限收紧 + 记录身份自洽校验（阶段 1）────────
//
// 背景（先取证再动手，取证见开工记录）：manifests 目录里现在**没有任何签名或摘要
// 绑定机制**；`digest` 只由 files[] 的 (role, sha256) 合成（SynthesizeDigest，
// probe.go），**不覆盖** capabilities / license / notes 等声明。且当记录被直接改盘时，
// 没有任何一步会把它与它声称的身份对上。
//
// 阶段 1 只做两件**廉价而真管用**的事，不碰签名（签名 + 官方来源交叉核对 = 阶段 2）：
//   1. 收紧写入权限：目录 ModeDir、文件 ModeFile（见 store.go WriteFileAtomic）；
//   2. 身份自洽校验（VerifyPlacement / CheckPlacement）：记录的**落盘位置**必须等于由
//      它自己的 digest 推出的规范路径 <manifests>/<id>/sha256-<digest前12>.json。
//
// 为什么身份自洽校验能当篡改检出：version 是内容寻址的（VersionOf 只由 digest 推出），
// 所以「文件放在哪」本身就携带了它声称的身份。手改过的记录很难同时让**正文里的 digest**
// 与**它所处的路径**自圆其说——改了 digest 而没改路径名、或把记录挪到别的 id 目录下冒充，
// 都会被这一步当场抓出。store.List 又以**目录名**当 id（不看正文里的 id），所以
// 「记录落在别的 model_id 目录下」正是待修补 #17「同名 model_id 可被替换」的一条真实入口，
// 这里一并对上。
//
// ⚠️ 诚实边界（写进标准，不许含糊）：本校验抓的是「digest 被改」与「记录被挪位置 /
// 冒充别的 id」两类。对「声明（capabilities/license/notes）被改、而 digest 与路径都不动」
// 这一类，本校验**管不到**——digest 的输入里没有这些字段。那需要记录签名与官方来源
// 交叉核对（阶段 2）。别把这一步当成「记录已可信」。

// 写入权限口径（待修补 #3 阶段 1，唯一来源；标准《写入权限与防伪》同步）。
// 目录 0700 / 文件 0600 —— 仅属主可读写/进入，同机其他用户与组一概进不来。
// os.Chmod 不受 umask 影响（精确置位）；os.MkdirAll 对已存在目录不生效，
// 故本口径只约束**新写入**的目录与文件，不改动已存在的（不做意外的权限挪动）。
const (
	ModeDir  os.FileMode = 0o700
	ModeFile os.FileMode = 0o600
)

// IsRecordFile 判定一个文件名是否算「一条记录的正文」。
// 与 store.List 的口径保持一致：只认 .json，跳过隐藏文件，且**跳过兄弟文件**
// （<version>.trace.json 探测留痕 / <version>.capabilities.json 能力快照）——
// 它们是「现状」，不是「身份」，拿它们做身份自洽校验只会误报。
func IsRecordFile(name string) bool {
	if !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
		return false
	}
	if strings.HasSuffix(name, ".trace.json") || strings.HasSuffix(name, ".capabilities.json") {
		return false
	}
	return true
}

// VerifyPlacement 校验一条记录**所在位置**是否与它自己的身份自洽（阶段 1 廉价篡改检出）。
//
// 两条规则，任一不符即 error（不警告、不放行——位置对不上就是不可信）：
//  1. 文件名（去 .json）必须等于 VersionOf(rec)，即 sha256-<digest 前 12>；
//  2. 上级目录名必须等于 rec.ID（store.List 以目录名当 id，位置是身份的又一截面）。
//
// path 是记录文件的路径（不是目录）。rec 是**已从 path 读出**的记录。
func VerifyPlacement(path string, rec *Record) []Finding {
	var f []Finding
	erf := func(field, detail string) { f = append(f, Finding{"error", field, detail}) }

	base := filepath.Base(path)
	wantBase := VersionOf(rec) + ".json"
	if base != wantBase {
		erf("path", fmt.Sprintf("记录所在路径名与它自己的 digest 不自洽：期望文件名 %s，实际 %s（改过 digest 或挪过位置）",
			wantBase, base))
	}
	if dirID := filepath.Base(filepath.Dir(path)); dirID != rec.ID {
		erf("id", fmt.Sprintf("记录所在目录名与记录里的 id 不自洽：目录名 %s，记录 id %s（记录被放进了别的 model_id 目录）",
			dirID, rec.ID))
	}
	return f
}

// PlacementResult 是对一条记录（或一个读不动的文件）做身份自洽校验的结论。
// Findings 只含**位置自洽**结论；记录格式（Verify）的结论分开计数，便于 CLI 分明。
type PlacementResult struct {
	Path         string    `json:"path"`
	ID           string    `json:"id,omitempty"`
	Findings     []Finding `json:"placement_findings,omitempty"`
	FormatErrors int       `json:"format_errors"`
	FormatWarns  int       `json:"format_warns"`
	ReadErr      string    `json:"read_error,omitempty"`
}

// PlacementErrors 是这一条里的位置自洽 error 数。
func (r PlacementResult) PlacementErrors() int { return CountErrors(r.Findings) }

// Failed 判定这一条是否不通过（读不动 / 位置不自洽 / 记录格式有 error；
// strict=true 时格式 warn 也算不通过，与 `zerg-model verify --strict` 同义）。
//
// 注意：Verify 自身的 strict 参数当前不影响 findings 的 Level（warn 仍是 warn），
// `verify --strict` 的“warn 也算不通过”是在 CLI 层落地的——这里对齐同一口径，
// 否则 `--in-place --strict` 会比 `verify --strict` 宽松，成为一处静默不一致。
func (r PlacementResult) Failed(strict bool) bool {
	return r.ReadErr != "" || r.PlacementErrors() > 0 || r.FormatErrors > 0 || (strict && r.FormatWarns > 0)
}

// CheckPlacement 对一个**记录文件**或**目录**做身份自洽校验（待修补 #3+#17 阶段 1）。
//
//	文件 → 只校这一条。
//	目录 → 递归其后**记录正文**（与 store.List 同口径：跳过兄弟文件/非 json/隐藏文件），
//	       逐个校验；目录不存在 = 空结果（只读，不创建）。
//
// 记录读不动/不是合法 JSON → 该条如实记 ReadErr（不中断整趟，反例优先）。
// strict=true 时格式 warn 也计入 FormatErrors（与 verify --strict 同义）。
func CheckPlacement(root string, strict bool) ([]PlacementResult, error) {
	fi, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !fi.IsDir() {
		return []PlacementResult{checkOnePlacement(root, strict)}, nil
	}
	var out []PlacementResult
	walkErr := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 单个目录读不动：跳过，不中断整趟
		}
		if d.IsDir() || !IsRecordFile(d.Name()) {
			return nil
		}
		out = append(out, checkOnePlacement(p, strict))
		return nil
	})
	return out, walkErr
}

func checkOnePlacement(path string, strict bool) PlacementResult {
	res := PlacementResult{Path: path}
	rec, err := Load(path)
	if err != nil {
		res.ReadErr = err.Error()
		return res
	}
	res.ID = rec.ID
	res.Findings = VerifyPlacement(path, rec)
	findings := Verify(rec, strict)
	res.FormatErrors = CountErrors(findings)
	res.FormatWarns = len(findings) - res.FormatErrors
	return res
}
