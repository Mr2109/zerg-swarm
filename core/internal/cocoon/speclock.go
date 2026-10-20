// speclock.go — 契约哈希锁：把 `<茧目录>/cocoon.toml` 的**原始字节**钉成 sha256，用来回答
// 「这份契约还是当初起茧时那一份吗」。
//
// 判据 9（v2 §E）：改一行 `cocoon.toml` ⇒ `inspect` 报漂移、`open` 拒。
//
// ★ 四条口径（每条都是判据，不是文风；改这里先读 v2 §E 与 meta.go 抬头）：
//
//	① **只算哈希、不写盘**：本文件的三个出口（`LockSpec` / `ProbeSpec` / `CheckSpec`）都是
//	  纯读＋纯算 —— 不立锁件、不建目录、不碰 mtime。锁值本身存在 `meta.json` 的 `SpecSHA` 里，
//	  没有第二份真源。理由：锁件是第二份真源，两份真源一定会漂，而对账件就得分清「meta 说 A、
//	  锁件说 B」算谁的 —— 一个字段能说清的事，不引第二处落盘。
//
//	② 哈希算**原始字节**，不是解析后的结构：注释、空白、键序、行尾变了都算漂移。契约是茧
//	  「起法」的唯一凭据；`readiness` 的行号语义、重复键的先后判定、`{{secret:…}}` 的书写位置
//	  都受字节影响 ⇒ 按结构比会放过「同一结构、不同字节」那一档，而那一档恰恰最容易瞒过人。
//
//	③ **错误里只出前 8 位**（`SpecShort`）：完整 64 位进了错误串，等于把整份契约的指纹抄进
//	  stderr / 日志 / 回执；判「一样 / 不一样」前 8 位就够。期望与实得**并列**印，这样
//	  「换了一份」与「拼错了一位」在读数上分得开。
//
//	④ 空契约**不算锁**（`ErrSpecEmpty`）：`sha256("")` 是常量，一个被截成 0 字节的契约会拿到
//	  同一个看起来合法的锁值 ⇒ 空文件当场拒，✗ 不许悄悄锁成 e3b0c442…。
package cocoon

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	// SpecSHAShortLen — 人面/错误串里能出现的哈希位数（前 8 位 hex）。
	SpecSHAShortLen = 8
	// specSHAHexLen — sha256 的完整 hex 长度（64）。
	specSHAHexLen = sha256.Size * 2
)

var (
	// ErrSpecDrift — 现读的契约字节算出的 sha256 与锁定值不符（或锁定值本身不可用 —— 形状非法
	// 也算漂移：锁坏了不比对就等于没锁，fail-closed 一律拒）。
	ErrSpecDrift = errors.New("cocoon: 契约漂移（cocoon.toml 与锁定的 sha256 不符）")
	// ErrSpecUnreadable — 契约读不到（不存在 / 权限 / 是目录），**无从比对**。
	// ★ 与 `ErrSpecDrift` 分开：前者是「内容变了」，后者是「拿不到内容」，报错时的处置不同
	// （人去看文件 vs 人去看路径）。
	ErrSpecUnreadable = errors.New("cocoon: 契约读不到（无法比对锁定值）")
	// ErrSpecNoLock — 调用方没给锁定值（`wantSHA` 空）。不拿「没有锁」冒充「一致」。
	ErrSpecNoLock = errors.New("cocoon: 没有锁定值（无从比对）")
	// ErrSpecEmpty — 要锁的字节是空的。见抬头 ④。
	ErrSpecEmpty = errors.New("cocoon: 契约字节为空（空文件算不出可信的锁）")
)

// LockSpec — 算一份契约字节的 sha256（小写 hex · 64 位）。**只算、不写盘**。
//
// 空字节 ⇒ `ErrSpecEmpty`（抬头 ④）。除此之外不判内容：语义校验是 `ParseSpec` 的事，
// 本函数只回答「这串字节的指纹是什么」—— 两件事分开，才能在契约**解析不过**时照样比对漂移。
func LockSpec(specBytes []byte) (string, error) {
	if len(specBytes) == 0 {
		return "", fmt.Errorf("%w: 0 字节", ErrSpecEmpty)
	}
	sum := sha256.Sum256(specBytes)
	return hex.EncodeToString(sum[:]), nil
}

// SpecShort — 哈希的前 8 位（人面与错误串**只许**用它，见抬头 ③）。
// 入参可以是完整 64 位、也可以是任何长度的串：短于 8 位就原样回（不补零冒充指纹）。
func SpecShort(sha string) string {
	s := strings.ToLower(strings.TrimSpace(sha))
	if len(s) <= SpecSHAShortLen {
		return s
	}
	return s[:SpecSHAShortLen]
}

// SpecStatus — 一次读盘的比对结果（`ProbeSpec` 的产物）。给人面/机器面报告用：**不改盘**。
type SpecStatus struct {
	// Path — 被比对的契约路径。
	Path string
	// Locked — 锁定值（已规整为小写、去空白；可能是空串或非法形状 —— 看 Err）。
	Locked string
	// Current — 现读字节的 sha256（读不到 / 是空文件时为空串）。
	Current string
	// Err — nil = 一致；否则是 `ErrSpecDrift` / `ErrSpecUnreadable` / `ErrSpecNoLock` 之一
	// （已 `%w` 包装，`errors.Is` 可判）。
	Err error
}

// OK — 锁与现读一致。
func (s SpecStatus) OK() bool { return s.Err == nil }

// ProbeSpec — 读 `path` 并算出「锁定值 / 现值 / 判定」三格。**只读**（不建锁件、不写盘）。
//
// 判定次序（每条都只在前一条不成立时才走）：
//
//	读不到        ⇒ ErrSpecUnreadable（连内容都没有，谈不上漂不漂）
//	wantSHA 空     ⇒ ErrSpecNoLock（没锁 ≠ 一致）
//	wantSHA 形状非法 ⇒ ErrSpecDrift（锁坏了按漂移判 —— fail-closed）
//	现读是空文件    ⇒ ErrSpecDrift（0 字节与任何真锁都不符；✗ 不拿 e3b0c442… 当一致）
//	两个哈希不等    ⇒ ErrSpecDrift（带期望/实得各前 8 位）
func ProbeSpec(path, wantSHA string) SpecStatus {
	st := SpecStatus{Path: path, Locked: strings.ToLower(strings.TrimSpace(wantSHA))}

	data, rerr := os.ReadFile(path)
	if rerr != nil {
		st.Err = fmt.Errorf("%w: %s: %v", ErrSpecUnreadable, path, rerr)
		return st
	}
	if got, lerr := LockSpec(data); lerr == nil {
		st.Current = got
	}

	if st.Locked == "" {
		st.Err = fmt.Errorf("%w: %s", ErrSpecNoLock, path)
		return st
	}
	if !isSpecSHA(st.Locked) {
		st.Err = fmt.Errorf("%w: %s 的锁定值形状非法（要 64 位十六进制，得 %d 位）",
			ErrSpecDrift, path, len(st.Locked))
		return st
	}
	if st.Current == "" {
		st.Err = fmt.Errorf("%w: %s 现读 0 字节（锁 %s…）",
			ErrSpecDrift, path, SpecShort(st.Locked))
		return st
	}
	if st.Current != st.Locked {
		st.Err = fmt.Errorf("%w: %s（期望 %s… 实得 %s…）",
			ErrSpecDrift, path, SpecShort(st.Locked), SpecShort(st.Current))
		return st
	}
	return st
}

// CheckSpec — 比一份契约与它的锁定值。一致 ⇒ nil；否则 `ErrSpecDrift` / `ErrSpecUnreadable` /
// `ErrSpecNoLock`（均可 `errors.Is`）。**只读**：与 `ProbeSpec` 同一条路，只是把三格压成一个错。
func CheckSpec(path, wantSHA string) error { return ProbeSpec(path, wantSHA).Err }

// isSpecSHA — 64 位十六进制（小写或大写都收；调用方已规整成小写）。
func isSpecSHA(s string) bool {
	if len(s) != specSHAHexLen {
		return false
	}
	if _, err := hex.DecodeString(s); err != nil {
		return false
	}
	return true
}
