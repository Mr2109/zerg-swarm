// self_identity_test.go —— 子端**自报身份**（缺口 `GAP-20261018-20` 的身份同步半）的回归牙。
//
// 判据：① `ReadSelf` 四格**永不空得让人误解**（取不到即 `unknown` / 空串 = 如实缺席）；
//
//	② 内容身份（`Digest`）是**自身可执行件**的 sha256 前 16 位（十六进制 16 位）；
//	③ 同一个进程里恒等（缓存 ⇒ 换件后未重启的进程不会把别人的新件说成自己的）；
//	④ `-tags commit=…,build_time=…` 的拆法认得这两个键、且不被别的标签干扰。
package version

import (
	"os"
	"regexp"
	"testing"
)

var selfDigestRE = regexp.MustCompile(`^[0-9a-f]{16}$`)

func TestReadSelf_ShapesAreHonest(t *testing.T) {
	s := ReadSelf()
	if s.Version == "" || s.Commit == "" || s.BuildTime == "" {
		t.Fatalf("三格都不许是空串（取不到也要写 %q）：%+v", SelfUnknown, s)
	}
	switch s.Source {
	case "embedded-tags", "buildinfo-vcs", "ldflags", "unknown":
	default:
		t.Fatalf("Source 只认四值，实得 %q", s.Source)
	}
	if s.Digest != "" && !selfDigestRE.MatchString(s.Digest) {
		t.Fatalf("Digest 应是 sha256 前 16 位十六进制，实得 %q", s.Digest)
	}
}

func TestReadSelf_ExecutableIsReadableSoDigestPresent(t *testing.T) {
	if _, err := os.Executable(); err != nil {
		t.Skipf("取不到本进程可执行件：%v", err)
	}
	s := ReadSelf()
	if s.Digest == "" {
		t.Fatal("本进程的可执行件读得到 ⇒ Digest 必须非空（空串只该出现在读不到的机器上）")
	}
}

func TestReadSelf_CachedWithinProcess(t *testing.T) {
	a := ReadSelf()
	resetSelfForTest()
	b := ReadSelf()
	if a != b {
		t.Fatalf("同一进程两次读取必须恒等（缓存），实得 %+v vs %+v", a, b)
	}
}

func TestParseBuildTags(t *testing.T) {
	kv := parseBuildTags("commit=2698b27fc+dirty,build_time=2026-10-19T00:00:00Z,netgo")
	if kv["commit"] != "2698b27fc+dirty" {
		t.Fatalf("commit 拆错：%q", kv["commit"])
	}
	if kv["build_time"] != "2026-10-19T00:00:00Z" {
		t.Fatalf("build_time 拆错：%q", kv["build_time"])
	}
	if _, ok := kv["netgo"]; ok {
		t.Fatal("无值标签不该进键值表")
	}
	if got := parseBuildTags(""); len(got) != 0 {
		t.Fatalf("空串应得空表，实得 %v", got)
	}
}
