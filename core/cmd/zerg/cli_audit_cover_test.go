// cli_audit_cover_test.go —— `zerg audit cover` **纯只读**的自证牙（缺口 `GAP-20260927-77`）。
//
// 判据：本命令自陈「**纯只读** · 不写件 · 不起进程 · **不改仓内状态**」（见 `family_audit.go`
// 件头与那道写旗标闸）⇒ 跑一次**不许写回** `.git/index`。
//
// 病灶（改前）：件里走 `gitRun`（不带 `--no-optional-locks`）⇒ 只要分母里有一件 **stat-dirty**
// （内容与提交同、mtime 回拨 —— 极常见：`touch` / checkout / 别的工具挪过），`git status`
// 就会**刷新并写回** index ⇒ 「纯只读」是假的。
//
// 配方（本仓既有 · 见 `cli_repo_status_shared_test.go:194`）：造一件真修改（进分母 M/A）+
// 一件 **stat-dirty**（mtime 回拨）—— 后者的在场才逼出那次写回；改后走 `gitRunNoLock`
// （`--no-optional-locks`）⇒ index 逐字节不动。
//
// 落点：`package main_test` ＋ `zerg.RunForTest`（跑**当前源码**的行为，不是盘上旧制品）。
// 全程只读：夹具 `t.TempDir()` 自管 · 不碰真仓 / 不碰状态目录（审计落点经 `ZERG_EDIT_AUDIT` 指进临时件）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// acStat3 取 `.git/index` 的 **stat 三件套**（mtime 纳秒 · size · 内容 sha256）——
// 判「这只件动没动过」用它，**不许只比 sha**：实测 git 改写 index 后内容可以逐字节相同
// （只 mtime 变）⇒ 只比 sha 就看不出来（与 `cli_repo_status_shared_test.go` 的 `rsStat3` 同一条纪律）。
func acStat3(t *testing.T, path string) string {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s：%v", path, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s：%v", path, err)
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%d|%d|%s", st.ModTime().UnixNano(), st.Size(), hex.EncodeToString(sum[:]))
}

func TestAuditCoverReadOnlyDoesNotWriteGitIndex(t *testing.T) {
	root := gitRepo(t)
	// 审计落点指进临时件（否则本机 `~/.zerg/state/edit_audit.jsonl` 读不到时 rc=8 · 与本判据无关）。
	auditFile := filepath.Join(t.TempDir(), "edit_audit.jsonl")
	if err := os.WriteFile(auditFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_EDIT_AUDIT", auditFile)
	// ① 真修改件（进分母 M/A）。
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n// changed\n")
	// ② stat-dirty 件：内容与提交同、mtime 回拨 ⇒ 触发 index 刷新（回拨后 plain `git status` 必写回）。
	old := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(root, "README.md"), old, old); err != nil {
		t.Fatal(err)
	}
	idx := filepath.Join(root, ".git", "index")
	before := acStat3(t, idx)

	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"audit", "cover", "--root", root}, &out, &errb)
	if rc != 0 {
		t.Fatalf("audit cover rc=%d（要 0）· stderr=%s", rc, errb.String())
	}
	after := acStat3(t, idx)
	if after != before {
		t.Errorf("audit cover 自陈纯只读，却改了 .git/index（`--no-optional-locks` 没生效）：before=%s after=%s", before, after)
	}
}
