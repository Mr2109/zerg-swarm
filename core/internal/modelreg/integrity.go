package modelreg

// integrity.go — 待修补 #13 的另一半：校验权重文件本身（不是只校验记录格式）。
//
// 背景：verify 只校验【记录格式】是否符合标准；记录里的 files[] 是否真的对应磁盘上那份文件
// （大小、sha256），此前无人核对。本文件补上这件事：**流式**算 sha256（权重可能几十 GB），
// 逐文件给出 ok / size_mismatch / sha_mismatch / missing / unverifiable。
//
// 硬规则：绝不扫描磁盘、不猜路径。定位不了就如实报 unverifiable（提示 --base-dir），
// 不降级、不静默——与标准「证据优先、不许编造」一致。

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 逐文件完整性状态（枚举）。
const (
	IntegrityOK           = "ok"            // 存在、大小一致（或跳过）、sha256 一致
	IntegritySizeMismatch = "size_mismatch" // 文件字节数与记录的 size 不符
	IntegritySHAMismatch  = "sha_mismatch"  // sha256 与记录不符（内容被改/损坏）
	IntegrityMissing      = "missing"       // 记录指向的位置没有这个文件
	IntegrityUnverifiable = "unverifiable"  // 无法定位/无法读取（如只有文件名却未给 --base-dir）
)

// FileIntegrity 是单个 files[] 条目的完整性结论。
type FileIntegrity struct {
	Index        int    `json:"index"`
	Role         string `json:"role"`
	Name         string `json:"name"`
	Path         string `json:"path,omitempty"` // 实际核对的文件路径（不可定位时为空）
	Status       string `json:"status"`
	SizeRecorded int64  `json:"size_recorded"`
	SizeActual   int64  `json:"size_actual"`
	SizeChecked  bool   `json:"size_checked"` // 是否真的做了大小比对（size=0/缺失时为 false）
	SHAExpected  string `json:"sha256_expected"`
	SHAComputed  string `json:"sha256_computed,omitempty"`
	Note         string `json:"note,omitempty"` // 跳过原因 / 失败细节（如实说明）
}

// IntegrityReport 是一条记录的完整性报告。
type IntegrityReport struct {
	BaseDir      string          `json:"base_dir,omitempty"`
	Files        []FileIntegrity `json:"files"`
	OK           int             `json:"ok"`
	Problems     int             `json:"problems"`     // 硬性不符（size/sha/missing）
	Unverifiable int             `json:"unverifiable"` // 定位/读取不了（strict 时才算失败）
}

// HasFailures 判定报告是否视为不通过：
// 任一硬性不符（size_mismatch / sha_mismatch / missing）必失败；strict 时不可定位也算失败。
func (rep IntegrityReport) HasFailures(strict bool) bool {
	return rep.Problems > 0 || (strict && rep.Unverifiable > 0)
}

// VerifyIntegrity 逐文件核对记录与磁盘上的权重文件是否一致（离线，不联网）。
//
// baseDir 为 --base-dir 指定的基准目录；为空时只能定位绝对路径或自带目录的相对路径的条目。
func VerifyIntegrity(r *Record, baseDir string) IntegrityReport {
	rep := IntegrityReport{BaseDir: baseDir}
	for i, fl := range r.Files {
		fi := FileIntegrity{
			Index:        i,
			Role:         fl.Role,
			Name:         fl.Name,
			SizeRecorded: fl.Size,
			SHAExpected:  normalizeSHA(fl.SHA256),
		}
		checkOneFile(&fi, fl.Name, baseDir)
		switch fi.Status {
		case IntegrityOK:
			rep.OK++
		case IntegrityUnverifiable:
			rep.Unverifiable++
		default:
			rep.Problems++
		}
		rep.Files = append(rep.Files, fi)
	}
	return rep
}

// checkOneFile 核对一个 files[] 条目，把结论写进 fi。
func checkOneFile(fi *FileIntegrity, name, baseDir string) {
	path, located, why := resolveIntegrityPath(name, baseDir)
	fi.Path = path
	if !located {
		fi.Status = IntegrityUnverifiable
		fi.Note = why
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			fi.Status = IntegrityMissing
			fi.Note = "文件不存在"
			return
		}
		fi.Status = IntegrityUnverifiable
		fi.Note = "无法读取文件元信息：" + err.Error()
		return
	}
	if info.IsDir() {
		fi.Status = IntegrityUnverifiable
		fi.Note = "路径是目录而非文件"
		return
	}
	fi.SizeActual = info.Size()

	// 大小比对：size 为 0 / 缺失 → 跳过（标准 §三 允许 size 可选），如实注明。
	if fi.SizeRecorded <= 0 {
		fi.Note = "记录未给 size（0/缺失），跳过大小比对"
	} else if info.Size() != fi.SizeRecorded {
		fi.SizeChecked = true
		fi.Status = IntegritySizeMismatch
		return
	} else {
		fi.SizeChecked = true
	}

	// sha256 比对（流式）。
	if fi.SHAExpected == "" {
		fi.Status = IntegrityUnverifiable
		fi.Note = appendNote(fi.Note, "记录里 sha256 为空，无法比对")
		return
	}
	sum, n, err := hashFileStream(path)
	if err != nil {
		fi.Status = IntegrityUnverifiable
		fi.Note = appendNote(fi.Note, "流式读取失败："+err.Error())
		return
	}
	fi.SHAComputed = sum
	if !fi.SizeChecked {
		fi.SizeActual = n // 顺带记下流式读取得到的实测大小
	}
	if sum != fi.SHAExpected {
		fi.Status = IntegritySHAMismatch
		return
	}
	fi.Status = IntegrityOK
}

// resolveIntegrityPath 决定一个 files[] 条目在磁盘上的位置（绝不扫描磁盘、不做模糊匹配）。
//
//   - name 为空 → 不可定位；
//   - name 是绝对路径 → 直接使用（自带位置）；
//   - name 是相对路径：
//     · 给了 --base-dir → <base-dir>/<name>；
//     · 未给：
//   - name 自带目录（如 sub/model.gguf）→ 按记录字面量用该相对路径（相对当前工作目录，不猜）；
//   - name 只有文件名              → 不可定位（提示需要 --base-dir）。
func resolveIntegrityPath(name, baseDir string) (path string, located bool, why string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false, "记录里 name 为空，无法定位"
	}
	if filepath.IsAbs(name) {
		return filepath.Clean(name), true, ""
	}
	if baseDir != "" {
		return filepath.Join(baseDir, name), true, ""
	}
	if filepath.Dir(name) != "." {
		return filepath.Clean(name), true, ""
	}
	return "", false, "记录里只有文件名（无目录），无法定位——请用 --base-dir 指定基准目录"
}

// hashFileStream 流式计算文件的 sha256（小写十六进制），返回摘要与实际字节数。
//
// 用 io.Copy 以固定大小缓冲逐块读取——几十 GB 的权重文件也不会被整体读进内存。
func hashFileStream(path string) (string, int64, error) {
	fh, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer fh.Close()
	h := sha256.New()
	n, err := io.Copy(h, fh)
	if err != nil {
		return "", n, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// normalizeSHA 归一化 sha256：去空白、转小写、去掉可选的 "sha256:" 前缀，
// 以便与记录里的写法（探测写入的是纯 hex）稳定比对。
func normalizeSHA(s string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "sha256:")
}

// appendNote 追加一条说明（用分号分隔多条）。
func appendNote(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "；" + add
}
