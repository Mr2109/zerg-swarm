// cli_build_show_test.go —— P0-6 `zerg build show` 的**真二进制**判据（缺口-命令面-20260921 §十一）。
//
// 判据的核心是「**逐字对拍**」：命令给出的 sha256/inode/bytes 必须与独立算出来的真值**逐字相同**
// （sha256 用 Go 的 crypto/sha256 现算，inode 用 syscall.Stat_t 现读）—— 不对拍就等于没给。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestBuildShow_ShaInodeBytesMatchIndependentComputation(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	art := filepath.Join(root, "bin", "fake-core")
	if err := os.MkdirAll(filepath.Dir(art), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("#!/bin/sh\necho fake artifact\n")
	if err := os.WriteFile(art, payload, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	wantSHA := hex.EncodeToString(sum[:])
	fi, err := os.Stat(art)
	if err != nil {
		t.Fatal(err)
	}
	wantInode := ""
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		wantInode = fmt.Sprintf("%d", st.Ino)
	}

	rc, out, errb := execCase(t, bin, root, "build", "show", "fake-core", "--json", "name,sha256,bytes,inode,type,arch,signed")
	if rc != 0 {
		t.Fatalf("件在 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	if len(env.Items) != 1 {
		t.Fatalf("要一件，得到 %d 条", len(env.Items))
	}
	row := env.Items[0]
	if row["sha256"] != wantSHA {
		t.Errorf("sha256 与独立现算不同：命令=%s 现算=%s", row["sha256"], wantSHA)
	}
	if row["bytes"] != fmt.Sprintf("%d", fi.Size()) {
		t.Errorf("bytes 不同：命令=%s 真值=%d", row["bytes"], fi.Size())
	}
	if wantInode != "" && row["inode"] != wantInode {
		t.Errorf("inode 与 stat 现读不同：命令=%s 真值=%s", row["inode"], wantInode)
	}
	if row["name"] != "fake-core" {
		t.Errorf("name 应当是件名（不带绝对路径）：%s", row["name"])
	}
	if row["signed"] == "" {
		t.Errorf("签名态一列不许空着（读不到要说读不到）")
	}
	t.Logf("对拍通过：sha256=%s inode=%s type=%s arch=%s signed=%s",
		row["sha256"][:12]+"…", row["inode"], row["type"], row["arch"], row["signed"])
}

func TestBuildShow_MissingArtifactAndUsage(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	rc, _, errb := execCase(t, bin, root, "build", "show", "nosuch-zz")
	if rc != 1 {
		t.Errorf("件不存在 ⇒ 退 1（不是错），得到 %d · stderr=%s", rc, errb)
	}
	rc, _, _ = execCase(t, bin, root, "build", "show")
	if rc != 2 {
		t.Errorf("缺件名 ⇒ 退 2，得到 %d", rc)
	}
	rc, _, _ = execCase(t, bin, root, "build", "show", "zerg", "--all")
	if rc != 2 {
		t.Errorf("件名与 --all 互斥 ⇒ 退 2，得到 %d", rc)
	}
	rc, _, _ = execCase(t, bin, root, "build", "show", "zerg", "--nosuchflag-zz")
	if rc != 2 {
		t.Errorf("未知旗标 ⇒ 退 2，得到 %d", rc)
	}
}
