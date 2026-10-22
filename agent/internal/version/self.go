// self.go —— 子端**自己**的代码身份（缺口 `GAP-20261018-20` 的身份同步半）。
//
// 病根（现读 · 逐字）：子端心跳把 `code_sha` 报成 `version.Commit`，而 `version.Commit` 只在
// **构建期**由 `-ldflags -X` 注入（未注入 ⇒ `unknown`）。两处失真：
//
//	① 构建时没带 ldflags（手搓 `go build`）⇒ 上报 `unknown`：主控判不了这台机跑哪版；
//	② 换了件（二进制字节已变）但提交号没变（同一 HEAD ＋ 未提交改动 ⇒ `<sha>+dirty`）
//	   ⇒ 换件前后 `code_sha` **逐字相同**、运维面判不出「新件到底跑没跑」。
//
// 治法：身份**从正在跑的这一件二进制自己读**，不再只信一份可能过期的注入值 ——
//
//	甲、`-tags commit=<sha>,build_time=<ts>`（构建器 `scripts/build/cmd/zerg-build-all` 的
//	    第二份注入，进**嵌入段** ⇒ `go version -m <件>` 可读）；
//	乙、`vcs.revision`（`debug/buildinfo` 的 VCS 戳 · 未 `-buildvcs=false` 构建时才有）；
//	丙、`-ldflags -X version.Commit`（旧口径，作最后回落）；
//	丁、**自身可执行件的 sha256 前 16 位**（内容身份）—— 与上面三格分属两种身份：
//	    前三格答「哪笔提交」，丁答「哪一**件字节**」。同提交的两份不同二进制（脏树两次构建）
//	    只有丁能分开 ⇒ 换件核对要靠丁，不靠甲。
//
// ★ 缺口 `GAP-20261021-08`（源提交半）：`Self` 原先只有 Version/Commit/BuildTime/Digest/Source ——
// **源提交（Origin）不在其中**，而 `version.Origin` 也没有任何构建路径注入 ⇒ 子端三台的「同源」
// 在当前代码下**永远判不出来**（`origin_sha` 恒空串 ⇒ 主控 `same_origin` 一律 `unknown`）。
// 治法：Origin 也收进 `Self`，取源三档 —— `-X version.Origin`（与 Commit 同法）> 嵌入段
// `-tags origin=<sha>`（`go version -m <件>` 可读）> 环境变量 `ZERG_ORIGIN`（手搓构建兜底）。
// 缺席仍是**空串**（✗ 不写 `unknown` 字面量）：`origin_sha` 这一格是**比对键**，主控
// `MachineOriginVerdict` 把空串与 `unknown` 同当「判不了」；人面另有 `OriginText` 印 `unknown`。
//
// ★ 纪律：三处「读不到」一律**如实缺席**（空串 + Source 点名来源），✗ 不编、✗ 不拿另一格顶上
// （两套命名空间混成一句就是假判决 —— 与 `origin_sha` 那条同一口径）。
// ★ 只在本进程**启动后第一次调用**时算一次并缓存：换件（`mv` 覆盖）之后**没有重启**的进程，
// 其「正在跑的字节」没变 ⇒ 缓存值才是真话；每次现读盘会把别人的新件说成自己的。
package version

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"sync"
)

// SelfUnknown —— 身份三格取不到时的唯一词（与 `version.Commit` 的未注入值同形）。
const SelfUnknown = "unknown"

// 取源词表（`Self.Source` 与 `Self.OriginSource` 共用这一套闭集 · ✗ 不另立新词）。
const (
	SourceLdflags      = "ldflags"       // 构建期 `-ldflags -X <pkg>.Var=…`
	SourceEmbeddedTags = "embedded-tags" // 嵌入段 `-tags k=v,…`（`go version -m <件>` 可读）
	SourceBuildinfoVCS = "buildinfo-vcs" // `debug/buildinfo` 的 `vcs.revision` 戳
	SourceEnv          = "env"           // 运行期环境变量（`ZERG_ORIGIN`；手搓构建兜底）
	SourceUnknown      = "unknown"       // 取不到 ⇒ 如实缺席
)

// selfDigestLen —— 内容身份（自身可执行件 sha256）报前多少位十六进制。
// 取 16（64 bit）：够区分两次构建，又不至于让心跳体里多一长串。
const selfDigestLen = 16

// Self —— 正在运行的这一件二进制**自己**报的身份。
type Self struct {
	Version   string // 版本号（`-ldflags -X version.Version`；未注入 ⇒ unknown）
	Commit    string // 代码提交（嵌入 tags > VCS 戳 > ldflags；都取不到 ⇒ unknown）
	BuildTime string // 构建时刻（嵌入 tags > `-ldflags`；取不到 ⇒ unknown）
	Digest    string // 自身可执行件 sha256 的前 16 位（取不到 ⇒ 空串 = 缺席）
	// Source 说明 Commit 那一格是从哪读来的（`embedded-tags` / `buildinfo-vcs` /
	// `ldflags` / `unknown`）—— 运维面据它判「这个 sha 有多可信」，✗ 不猜。
	Source string
	// Origin 源提交（同源判定的**比对键** · GAP-20261013-09 / GAP-20261021-08）：
	// 本机二进制所编那棵树的 HEAD 带 `GitOrigin-RevId` trailer（镜像树）⇒ trailer 值（= 私有图
	// 源提交）；不带（私有权威树）⇒ HEAD 本身；构建器一律截成**同一长度**（12 位）⇒ 两侧逐字可比。
	// 取源：`-X version.Origin` > 嵌入段 `origin=` > `ZERG_ORIGIN` > **空串 = 判不了**。
	// ✗ 绝不回落成 Commit：节点 Commit 是镜像 tip、主控 Commit 是私有提交，回落即假判决。
	Origin string
	// OriginSource 说明 Origin 那一格的来路（`ldflags` / `embedded-tags` / `env` / `unknown`）。
	OriginSource string
}

var (
	selfOnce sync.Once
	selfVal  Self
	// selfEmbeddedOrigin —— 嵌入段里的 `origin=<sha>`（随**这一件字节**走 ⇒ 与其余件事实一起缓存）。
	selfEmbeddedOrigin string
)

// ReadSelf 取本进程的身份（**件事实**首次调用时算一次并缓存；纯只读、零网络）。
//
// ★ Origin 的两层注入（`-X` 变量 / `ZERG_ORIGIN`）属**进程**、不属「件」⇒ 每次读取时现算：
// 读一个字符串变量/环境变量极廉，且测试能在同一进程里改注入值断言；缓存仍钉死
// Commit/BuildTime/Digest/嵌入段 Origin 这些「这一件字节」的事实。
func ReadSelf() Self {
	selfOnce.Do(func() { selfVal, selfEmbeddedOrigin = resolveSelf() })
	s := selfVal
	s.Origin, s.OriginSource = resolveOrigin(selfEmbeddedOrigin)
	return s
}

// resetSelfForTest 只给本包测试用：把缓存清掉再算一次（生产路径不调用）。
func resetSelfForTest() {
	selfOnce = sync.Once{}
	selfVal = Self{}
	selfEmbeddedOrigin = ""
}

// resolveOrigin —— Origin 的取源顺序（**唯一实现处**，`Self.Origin` 与心跳 `origin_sha` 共用）：
//
//	① `-ldflags -X version.Origin`（本仓既有注入法 · 构建器给的值）
//	② 嵌入段 `-tags origin=<sha>`（不执行该件也 `go version -m` 可读；构建器第二份注入）
//	③ `ZERG_ORIGIN` 环境变量（手搓 `go build` 的兜底）
//	④ 都没有 ⇒ **空串 = 判不了**（✗ 不编、✗ 不回落成 Commit）
//
// ★ 解析结果的形状由**构建器**归一（一律 12 位前缀）；本函数只做「取第一个非空」，不改值。
func resolveOrigin(embedded string) (value, source string) {
	if v := strings.TrimSpace(Origin); v != "" {
		return v, SourceLdflags
	}
	if v := strings.TrimSpace(embedded); v != "" {
		return v, SourceEmbeddedTags
	}
	if v := strings.TrimSpace(os.Getenv(OriginEnv)); v != "" {
		return v, SourceEnv
	}
	return "", SourceUnknown
}

func resolveSelf() (Self, string) {
	s := Self{Version: Version, Commit: Commit, BuildTime: BuildTime, Source: SourceLdflags}
	var embeddedOrigin string
	if s.Version == "" {
		s.Version = SelfUnknown
	}
	if s.BuildTime == "" {
		s.BuildTime = SelfUnknown
	}
	if s.Commit == "" {
		s.Commit, s.Source = SelfUnknown, SourceUnknown
	}

	exe, err := os.Executable()
	if err != nil {
		return s, embeddedOrigin
	}
	// 甲 / 乙：嵌入段（tags）优先，其次 VCS 戳 —— 两者都随**这一个字节件**走，
	// 而 ldflags 那一格可能与盘上的件无关（构建脚本自己算的 sha）。
	if bi, err := buildinfo.ReadFile(exe); err == nil {
		var vcsRev, vcsDirty string
		for _, st := range bi.Settings {
			switch st.Key {
			case "-tags":
				kv := parseBuildTags(st.Value)
				if v := strings.TrimSpace(kv["commit"]); v != "" {
					s.Commit, s.Source = v, SourceEmbeddedTags
				}
				if v := strings.TrimSpace(kv["build_time"]); v != "" {
					s.BuildTime = v
				}
				// 源提交的第二份注入（与 commit/build_time 同一行 tags）：
				// 只**收成候选**，真正的取舍在 resolveOrigin 里（-X 优先）。
				if v := strings.TrimSpace(kv["origin"]); v != "" {
					embeddedOrigin = v
				}
			case "vcs.revision":
				vcsRev = strings.TrimSpace(st.Value)
			case "vcs.modified":
				vcsDirty = strings.TrimSpace(st.Value)
			}
		}
		if s.Source != SourceEmbeddedTags && vcsRev != "" {
			s.Commit, s.Source = vcsRev, SourceBuildinfoVCS
			if vcsDirty == "true" {
				s.Commit += "+dirty"
			}
		}
	}
	// 丁：内容身份 —— 读的是**本进程的可执行件**。读不到 ⇒ 空串（缺席），✗ 不编。
	if dg, err := fileDigest(exe); err == nil {
		s.Digest = dg
	}
	return s, embeddedOrigin
}

// parseBuildTags 拆 `commit=<sha>,build_time=<ts>` 这一串构建标签。
// 形状照 `scripts/build/cmd/zerg-build-all/main.go` 的 `gotags`；别的标签（`netgo` 一类）忽略。
func parseBuildTags(v string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(v, ",") {
		k, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(val)
	}
	return out
}

// fileDigest 取一个件的 sha256 前 `selfDigestLen` 位（流式读，不整件进内存）。
func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if len(sum) < selfDigestLen {
		return "", nil
	}
	return sum[:selfDigestLen], nil
}

// Line 机器可读单行身份（人面/日志用；✗ 不含 Digest 以外的新词汇）。
func (s Self) Line(component string) string {
	return component + " " + s.Version + " " + s.Commit + " " + s.BuildTime
}

// OriginText —— Origin 的**人面**词：缺席 ⇒ `unknown`（机器面仍是空串，见 `Origin`）。
func (s Self) OriginText() string {
	if s.Origin == "" {
		return SelfUnknown
	}
	return s.Origin
}

// Identity —— 机器可读的**完整**单行身份（**新**行；`Line` 的四格格式一个字节不动，
// 现有按空格切 `$1..$4` 的分析脚本照旧）。形状：
//
//	zerg-agentd 2.5.16 abc1234 2026-10-21T18:30:21Z origin=3b04f6abcd (ldflags)
//
// 第 5 格 `origin=<sha|unknown>`、第 6 格 `(<来路>)`：来路取 `ldflags`/`embedded-tags`/`env`/`unknown`。
func (s Self) Identity(component string) string {
	return s.Line(component) + " origin=" + s.OriginText() + " (" + s.OriginSource + ")"
}
