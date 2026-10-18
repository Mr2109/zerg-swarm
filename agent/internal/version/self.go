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
}

var (
	selfOnce sync.Once
	selfVal  Self
)

// ReadSelf 取本进程的身份（首次调用时算一次并缓存；纯只读、零网络）。
func ReadSelf() Self {
	selfOnce.Do(func() { selfVal = resolveSelf() })
	return selfVal
}

// resetSelfForTest 只给本包测试用：把缓存清掉再算一次（生产路径不调用）。
func resetSelfForTest() {
	selfOnce = sync.Once{}
	selfVal = Self{}
}

func resolveSelf() Self {
	s := Self{Version: Version, Commit: Commit, BuildTime: BuildTime, Source: "ldflags"}
	if s.Version == "" {
		s.Version = SelfUnknown
	}
	if s.BuildTime == "" {
		s.BuildTime = SelfUnknown
	}
	if s.Commit == "" {
		s.Commit, s.Source = SelfUnknown, "unknown"
	}

	exe, err := os.Executable()
	if err != nil {
		return s
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
					s.Commit, s.Source = v, "embedded-tags"
				}
				if v := strings.TrimSpace(kv["build_time"]); v != "" {
					s.BuildTime = v
				}
			case "vcs.revision":
				vcsRev = strings.TrimSpace(st.Value)
			case "vcs.modified":
				vcsDirty = strings.TrimSpace(st.Value)
			}
		}
		if s.Source != "embedded-tags" && vcsRev != "" {
			s.Commit, s.Source = vcsRev, "buildinfo-vcs"
			if vcsDirty == "true" {
				s.Commit += "+dirty"
			}
		}
	}
	// 丁：内容身份 —— 读的是**本进程的可执行件**。读不到 ⇒ 空串（缺席），✗ 不编。
	if dg, err := fileDigest(exe); err == nil {
		s.Digest = dg
	}
	return s
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
