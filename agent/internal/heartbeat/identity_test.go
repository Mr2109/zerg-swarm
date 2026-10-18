// identity_test.go —— 心跳身份三格的接线（缺口 `GAP-20261018-20`）。
//
// 判据：心跳的 `code_sha` / `code_build_time` / `code_digest` 取自 `version.ReadSelf()`
// （**这一件二进制自己**），不是直接抄构建期注入的 `version.Commit` —— 后者在手搓构建下是
// `unknown`、在脏树两次构建下是同一个 `<sha>+dirty`，两态都判不出「跑的是哪一件」。
package heartbeat

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/version"
)

func TestBuildBody_CodeIdentityComesFromSelf(t *testing.T) {
	r := newTestRunner(baseBackend(), fakeVram{used: 1, total: 8, known: true}, &fakeActive{n: 0})
	body := r.buildBody()
	self := version.ReadSelf()

	if got := body["code_sha"]; got != self.Commit {
		t.Fatalf("心跳 code_sha 应取自 version.ReadSelf().Commit，实得 %v ≠ %v", got, self.Commit)
	}
	if got := body["code_build_time"]; got != self.BuildTime {
		t.Fatalf("心跳 code_build_time 应取自 ReadSelf（缺了就报 %q），实得 %v", version.SelfUnknown, got)
	}
	if got, ok := body["code_digest"]; !ok {
		t.Fatal("心跳必须带 code_digest 这一格（取不到可为空串，但不许整键消失）")
	} else if got != self.Digest {
		t.Fatalf("code_digest 应取自 ReadSelf，实得 %v ≠ %v", got, self.Digest)
	}
	if version.Version == "" {
		t.Fatal("version.Version 不该是空串（包级默认值是 unknown）")
	}
}
