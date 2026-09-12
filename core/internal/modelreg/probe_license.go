package modelreg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ── probe.license.v1：从本地建材取许可证证据（待修补 #12）────────────────────────
//
// 背景（评审 example-35b-v2 观点 7）：标准 §二 的底线「读权重、不读徽章」此前没有执行者——
// §五/§九 自认「人工补许可证来源」，于是许可证既没有被探测、也没有证据锚，许可红线
// （commercial != yes 不得作默认项）全靠人自觉。本探测器补上这个执行者。
//
// 只认**结构化来源**（与待修补 #23 同一套纪律：拿不到结构化证据就 unknown，不许猜）：
//
//	① 权重 GGUF 自带的许可键：general.license / general.license.name / general.license.link；
//	② 与权重**同目录**、**明确命名**的许可证文件：LICENSE / LICENCE / LICENSE.txt /
//	   LICENCE.txt / LICENSE.md / LICENCE.md / COPYING / COPYING.txt（文件名大小写不敏感）。
//
// ⛔ 明确不做：**不**在 README / 模型卡 / 任何自由文本里做文案匹配猜许可证
// （"这段看起来像 MIT"不是证据；与 #23「不靠响应体文案匹配」同一纪律）。没有任何结构化
// 来源 → spdx=unknown、commercial=unknown，并把尝试过的来源写进 evidence
// ——缺 = 未知，绝不 = 没有，更**绝不默认 yes**。
const EvidenceLicense = "probe.license.v1"

// FailNoLicenseSource 是 probe.license.v1 的失败分类：本地建材里没有任何结构化许可来源。
// 它**不是**「这个模型没有许可证」（那要读权重才知道），而是「这次没取到证据」——
// 与 #27 的 budget_exhausted 同列为"不可判定"，绝不降级成任何结论性取值。
const FailNoLicenseSource = "no_license_source"

// 许可证证据的来源分类（写进 evidence，也用于分辨两种结构化来源）。
const (
	LicenseSourceGGUFKey     = "gguf_key"
	LicenseSourceSameDirFile = "same_dir_file"
	LicenseSourceNone        = "none"
)

// GGUF 许可键名（写死成常量，避免各处随手拼串导致证据不可复现）。
const (
	LicenseGGUFKeySPDX = "general.license"
	LicenseGGUFKeyName = "general.license.name"
	LicenseGGUFKeyLink = "general.license.link"
)

// licenseFileNames 是「明确命名的许可证文件」的**精确名**白名单（按此顺序探测，文件名大小写不敏感）。
// 精确名之后还有一档同样"明确命名"的名字（首 token 为 license/licence/copying，
// 如 LICENSE-MIT）——见 findLicenseFile。
//
// 为什么用白名单而不是"目录里任何含 licen 的文件"：README.md 里写一句"本模型采用 MIT"
// 不是许可证文件；把自由文本当证据正是 #12 要堵的洞。NOTICE 这类**声明**文件不进清单
// ——它通常只是转载声明，不是许可授予，认它等于拿徽章当许可证。
var licenseFileNames = []string{
	"LICENSE", "LICENCE", "LICENSE.txt", "LICENCE.txt", "LICENSE.md", "LICENCE.md",
	"COPYING", "COPYING.txt",
}

// licenseSignature 是一条按**许可证正文特征串**识别的规则：needles 全部出现（大小写不敏感、
// 空白已折叠）才认定。用的是各许可证的法定固定措辞，不是"看起来像"。
type licenseSignature struct {
	SPDX    string
	Needles []string
}

// licenseSignatures 是识别规则表。顺序有意义：更具体的规则在前
// （BSD-3-Clause 必须先于 BSD-2-Clause——前者的正文包含后者的正文）。
var licenseSignatures = []licenseSignature{
	{"Apache-2.0", []string{"apache license", "version 2.0"}},
	{"MIT", []string{"permission is hereby granted, free of charge"}},
	{"BSD-3-Clause", []string{"redistribution and use in source and binary forms", "neither the name"}},
	{"BSD-2-Clause", []string{"redistribution and use in source and binary forms"}},
	{"AGPL-3.0", []string{"gnu affero general public license", "version 3"}},
	{"LGPL-3.0", []string{"gnu lesser general public license", "version 3"}},
	{"GPL-3.0", []string{"gnu general public license", "version 3"}},
	{"MPL-2.0", []string{"mozilla public license version 2.0"}},
}

// licenseEvidenceMax 是许可证据串的上限（键名 + 值原文/文件摘要可能较长，截断只影响可读性）。
const licenseEvidenceMax = 512

// licenseReadMax 是一次最多读进内存的许可证文件字节数（许可证正文远小于此）。
const licenseReadMax = 1 << 20

// LicenseProbe 是 probe.license.v1 的产物（记录正文的 license 块 + evidence 由它生成）。
type LicenseProbe struct {
	SPDX       string   // 许可证身份（SPDX 表达式；取不到 = unknown）
	Name       string   // 非 SPDX 时的许可证名（GGUF general.license.name）
	Link       string   // 许可证链接（GGUF general.license.link）
	Commercial string   // 商用性四态；探测恒为 unknown（见 ProbeLicense 注释）
	Gated      bool     // 是否需申请访问（本地建材无从判定 → false 表示"没有这条信号"）
	Source     string   // gguf_key / same_dir_file / none
	FileSHA256 string   // same_dir_file 时该文件的 sha256（内容锚，不含路径）
	FileName   string   // same_dir_file 时的文件名（只写文件名，不写目录）
	Tried      []string // 尝试过的结构化来源（读不到时逐条列出）
	Evidence   string   // 证据串：探测器名 + 来源（键名 / 文件名 + 内容摘要）
}

// ProbeLicense 从本地建材取许可证证据（probe.license.v1）。
//
// 参数：
//   - meta：本地权重的 GGUF 元数据（nil 表示没有——例如端点探测或非 GGUF 文件）；
//   - dir： 权重所在目录（空 = 没给本地文件，不扫目录）。
//
// 商业性为什么恒为 unknown（本探测器的核心取舍）：
//
//	四态里 yes/no/revenue_gated 都是**法律判断**——本地结构化信号给出的是"许可证的身份"
//	（spdx/name/link），给不出"能不能商用"。实测本机 gemma-4-26B 的 GGUF 声明
//	general.license="apache-2.0"，但同一条记录的 general.license.link 指向 Google 的
//	Gemma 专用条款（https://ai.google.dev/gemma/docs/gemma_4_license）——仅凭 spdx 串就写
//	yes，与「拿仓库徽章当权重许可证」是同一类错误。故本探测器**不下商业结论**：把身份与
//	来源写进记录，commercial 留 unknown 交人工/声明（no/revenue_gated 仍必须留痕，规则不动）。
//
// 优先级：权重自带的 GGUF 声明 > 同目录许可证文件（本地事实优先，同 #22 的取舍）。
// 两个来源都拿不到 → spdx=unknown，evidence 写明尝试过哪些来源（缺 = 未知）。
//
// 诚实边界（**故意不做的两件事**，都为了"不能判定 ≠ 不支持"与正文确定性）：
//   - 同目录是**共享目录**时（例：本机 ~/models 里混放着多个 .gguf），该 LICENSE
//     未必只属于本模型。这里**不**去数"目录里还有几个权重"来提示归属——那会让正文取决于
//     邻居文件（同一批建材、旁边多放一个模型就会变成不同字节），破坏「同一批建材 → 正文逐字节
//     相同」。故只如实写文件名 + 内容摘要进 evidence，归属交人工（照 evidence 去看那个文件）。
//   - **不**读同目录的 README / 模型卡 / 任何自由文本：那是文案匹配猜许可证，#12 明令禁止。
func ProbeLicense(meta *GGUFMeta, dir string) LicenseProbe {
	p := LicenseProbe{
		SPDX:       "unknown",
		Commercial: "unknown",
		Source:     LicenseSourceNone,
	}
	var anchors []string

	// ① 权重 GGUF 自带的许可键。
	if meta != nil {
		raw := strings.TrimSpace(meta.LicenseSPDX)
		nm := strings.TrimSpace(meta.LicenseName)
		lk := strings.TrimSpace(meta.LicenseLink)
		if raw != "" || nm != "" || lk != "" {
			spdx, name, link := canonicalSPDX(raw, nm, lk)
			p.SPDX = spdx
			p.Name, p.Link, p.Source = name, link, LicenseSourceGGUFKey
			if p.Name == "" {
				p.Name = nm // 认得 SPDX 时 canonicalSPDX 不回带名字；名字本身也是证据，如实保留
			}
			if p.Link == "" {
				p.Link = lk
			}
			if raw != "" {
				anchors = append(anchors, fmt.Sprintf("%s: %s=%q", LicenseSourceGGUFKey, LicenseGGUFKeySPDX, raw))
			}
			if nm != "" {
				anchors = append(anchors, fmt.Sprintf("%s: %s=%q", LicenseSourceGGUFKey, LicenseGGUFKeyName, nm))
			}
			if lk != "" {
				anchors = append(anchors, fmt.Sprintf("%s: %s=%q", LicenseSourceGGUFKey, LicenseGGUFKeyLink, lk))
			}
		}
	}

	// ② 与权重同目录、明确命名的许可证文件。
	fileSPDX := ""
	if dir != "" {
		if name, data, ok := findLicenseFile(dir); ok {
			sum := sha256.Sum256(data)
			p.FileName = name
			p.FileSHA256 = hex.EncodeToString(sum[:])
			fileSPDX = licenseSPDXFromText(string(data))
			if fileSPDX == "" {
				fileSPDX = licenseSPDXFromFileName(name)
			}
			anchor := fmt.Sprintf("%s: %s; sha256=%s", LicenseSourceSameDirFile, name, p.FileSHA256)
			if fileSPDX == "" {
				anchor += "; spdx=unrecognized"
			}
			anchors = append(anchors, anchor)
		}
	}

	switch {
	case p.Source == LicenseSourceNone && fileSPDX != "":
		// 权重没自带声明，但同目录许可证文件是**可识别**的结构化来源。
		p.SPDX, p.Source = fileSPDX, LicenseSourceSameDirFile
	case p.Source == LicenseSourceGGUFKey && fileSPDX != "" && fileSPDX != p.SPDX:
		// 两个结构化来源互相矛盾：如实标出来，绝不静默挑一个（"不能判定"必须可见）。
		anchors = append(anchors, fmt.Sprintf("冲突：同目录许可证文件识别为 %s，与 GGUF 声明 %s 不一致（待人工确认）", fileSPDX, p.SPDX))
	}

	if len(anchors) == 0 {
		// 结构化来源一个都没命中：evidence 逐条列出尝试过的来源（缺 = 未知，绝不 = 没有）。
		p.Tried = []string{
			LicenseSourceGGUFKey + "=" + strings.Join([]string{LicenseGGUFKeySPDX, LicenseGGUFKeyName, LicenseGGUFKeyLink}, "|"),
			LicenseSourceSameDirFile + "=" + strings.Join(licenseFileNames, "|"),
		}
		p.Evidence = truncate(fmt.Sprintf("%s (%s: tried %s)", EvidenceLicense, FailNoLicenseSource, strings.Join(p.Tried, "; ")), licenseEvidenceMax)
		return p
	}
	// 命中文件但识别不出时，SPDX 仍是未知：把失败分类一并写进证据（好让 grep 找得到）。
	if p.Source == LicenseSourceNone {
		p.Tried = []string{FailNoLicenseSource}
		anchors = append(anchors, FailNoLicenseSource)
	}
	p.Evidence = truncate(fmt.Sprintf("%s (%s)", EvidenceLicense, strings.Join(anchors, "; ")), licenseEvidenceMax)
	return p
}

// findLicenseFile 在 dir 里按白名单顺序找一个明确命名的许可证文件。
// 只读、只返回文件名（不含目录）与内容——目录属"存放位置"，不进记录正文
// （同一批建材换个目录，正文必须逐字节相同）。
//
// 两档命名口径（都属"明确命名"，不是模糊匹配）：
//   - 白名单精确名（LICENSE / LICENSE.txt / COPYING …）优先；
//   - 否则退到"首 token 是 license/licence/copying"的这种名字（LICENSE-MIT / LICENSE-APACHE-2.0 …），
//     同名多个时取 os.ReadDir 的字典序第一个（确定性）。
//
// 为什么不做"目录里任何含 licen 的文件"：README 里写一句"本模型采用 MIT"不是许可证文件，
// 把它当证据正是 #12 要堵的洞。
func findLicenseFile(dir string) (string, []byte, bool) {
	ents, err := os.ReadDir(dir) // 已按文件名排序 → 多命中时结果确定
	if err != nil {
		return "", nil, false
	}
	byLower := map[string]string{}
	var candidates []string
	for _, en := range ents {
		if en.IsDir() {
			continue
		}
		byLower[strings.ToLower(en.Name())] = en.Name()
		if licenseFileNameCandidate(en.Name()) {
			candidates = append(candidates, en.Name())
		}
	}
	for _, want := range licenseFileNames {
		if actual, ok := byLower[strings.ToLower(want)]; ok {
			if data, ok := readLicenseFile(dir, actual); ok {
				return actual, data, true
			}
		}
	}
	if len(candidates) > 0 {
		if data, ok := readLicenseFile(dir, candidates[0]); ok {
			return candidates[0], data, true
		}
	}
	return "", nil, false
}

// licenseFileNameCandidate 判定文件名是否"明确是许可证文件"：去掉扩展名后，首个
// 字母数字 token 必须是 license / licence / copying。
// 例：LICENSE-MIT ✔ / LICENCE.md ✔ / COPYING ✔ / permit-mit-like.txt ✗ / README.md ✗。
func licenseFileNameCandidate(name string) bool {
	base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	toks := strings.FieldsFunc(base, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	if len(toks) == 0 {
		return false
	}
	switch toks[0] {
	case "license", "licence", "copying":
		return true
	}
	return false
}

// readLicenseFile 读一个候选许可证文件（跳过目录/空文件/超大文件/读不动）。
func readLicenseFile(dir, name string) ([]byte, bool) {
	fi, err := os.Stat(filepath.Join(dir, name))
	if err != nil || fi.IsDir() || fi.Size() == 0 || fi.Size() > licenseReadMax {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, false
	}
	return data, true
}

// licenseSPDXFromText 按**法定固定措辞**识别许可证正文；认不出返回 ""（绝不猜成 other/MIT）。
func licenseSPDXFromText(text string) string {
	low := normalizeLicenseText(text)
	if low == "" {
		return ""
	}
	for _, sig := range licenseSignatures {
		hit := true
		for _, n := range sig.Needles {
			if !strings.Contains(low, n) {
				hit = false
				break
			}
		}
		if hit {
			return sig.SPDX
		}
	}
	return ""
}

// normalizeLicenseText 把正文转小写并折叠空白，让"法定措辞"的匹配不受换行/缩进影响。
func normalizeLicenseText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// licenseSPDXFromFileName 是**窄口径**的文件名规则（正文认不出时才用）。
// 只认整 token，不做子串模糊匹配（避免 "permit"/"unlicensed" 之类误命中）：
//
//	LICENSE-MIT / MIT-LICENSE / LICENSE.MIT → MIT
//	LICENSE-APACHE-2.0 → Apache-2.0
//
// 认不出返回 ""（= 未知，不猜）。
func licenseSPDXFromFileName(name string) string {
	base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	var toks []string
	for _, p := range strings.FieldsFunc(base, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		switch p {
		case "license", "licence", "lic", "copying", "notice", "txt", "md", "the", "file":
			continue
		}
		toks = append(toks, p)
	}
	switch strings.Join(toks, "") {
	case "mit":
		return "MIT"
	case "apache", "apache2", "apache20":
		return "Apache-2.0"
	case "bsd3", "bsd3clause":
		return "BSD-3-Clause"
	case "bsd2", "bsd2clause":
		return "BSD-2-Clause"
	case "gpl3", "gpl30":
		return "GPL-3.0"
	case "agpl3", "agpl30":
		return "AGPL-3.0"
	}
	return ""
}
