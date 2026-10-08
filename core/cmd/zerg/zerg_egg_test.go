package main

// zerg_egg_test.go —— `zerg_egg.go`（`scripts/node-a/zerg-egg.py` 的 Go 真搬）的**成对负控测试件**。
//
// 四组牙：
//  ① 格式契约（golden）：固定树 + 固定 `created_at` ⇒ pack 产物的 **sha256 与索引 JSON 逐字节**必须命中常量
//     （任何字段序 / 编码 / 对齐漂移 ⇒ 红）。
//  ② 校验面：干净袋不报；改一字节 / 截断 ⇒ 必报。
//  ③ ★ A 类止血牙：`zegSelfTest` 必须在**自建的临时目录**里跑并**回收干净**（`defer os.RemoveAll`）——
//     把那一行删掉 ⇒ 本用例必红（见回执「哑化负控」）。py 原件在这一档**每次都漏一个目录**（已现读坐实）。
//  ④ 命令行面：未知旗标 / 非法子命令 / 无子命令的 rc 与 py 逐字对齐（退 2 / 2 / 64）。
//
// ★ 本件**不写** `TestMain`（包内他件已占该符号 ⇒ go vet 会报 multiple definitions）。

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zegGoldenEggSHA —— 固定 fixture（`a.txt`="hello" · `sub/b.bin`=0x00..0xFF · 均 0644 ·
// `--now 2000-01-01T00:00:00Z` · align 16384）下 Go 侧 pack 产物的 sha256。与 py 原件**逐字节同**（对拍已证）。
const zegGoldenEggSHA = "57775417aaf053e72e530fa175a0e3c201ef3f8d3dfb3eee67f09487d765ed31"

// zegGoldenIndexJSON —— 同一 fixture 的索引 JSON **逐字节**（字段序即契约）。
const zegGoldenIndexJSON = `{"magic":"ZEGG","version":1,"align":16384,"tool":{"name":"zerg-egg","version":"1.0.0"},"created_at":"2000-01-01T00:00:00Z","entries":[{"path":"a.txt","off":16384,"size":5,"sha256":"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824","mode":420},{"path":"sub/b.bin","off":32768,"size":256,"sha256":"40aff2e9d2d8922e47afd4648e6967497158785fbd1da870e7110266bf944880","mode":420}]}`

// zegTBuildGoldenTree —— 造固定树（内容 + 权限都钉死 ⇒ 产物可逐字节复现）。
func zegTBuildGoldenTree(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 256)
	for i := 0; i < 256; i++ {
		b[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.bin"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	// umask 兜底：显式钉回 0644（索引里的 mode 逐字取决于它）。
	if err := os.Chmod(filepath.Join(root, "a.txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "sub", "b.bin"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// zegTIndexBytes —— 从一枚卵里抠出索引 JSON 字节。
func zegTIndexBytes(t *testing.T, egg string) []byte {
	t.Helper()
	d, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	n := int64(d[16]) | int64(d[17])<<8 | int64(d[18])<<16 | int64(d[19])<<24 |
		int64(d[20])<<32 | int64(d[21])<<40 | int64(d[22])<<48 | int64(d[23])<<56
	if zegHeaderPrefix+int(n) > len(d) {
		t.Fatalf("索引长度越界：n=%d len=%d", n, len(d))
	}
	return d[zegHeaderPrefix : zegHeaderPrefix+int(n)]
}

// ---- ① 格式契约 -----------------------------------------------------------------------------

func TestZergEggGoldenFormat(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	zegTBuildGoldenTree(t, src)
	egg := filepath.Join(dir, "g.egg")
	if _, err := zegPack(src, egg, 16384, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	d, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(d)
	if got := hex.EncodeToString(sum[:]); got != zegGoldenEggSHA {
		t.Fatalf("golden sha256 漂移：got=%s want=%s（格式契约被改）", got, zegGoldenEggSHA)
	}
	if got := string(zegTIndexBytes(t, egg)); got != zegGoldenIndexJSON {
		t.Fatalf("索引 JSON 逐字节不符：\n got=%s\nwant=%s", got, zegGoldenIndexJSON)
	}
	if string(d[0:4]) != zegMagic {
		t.Fatalf("前缀 magic 不符：%q", d[0:4])
	}
}

// ---- ② 校验面 -------------------------------------------------------------------------------

func TestZergEggVerifyDetectsCorruption(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	zegTBuildGoldenTree(t, src)
	egg := filepath.Join(dir, "g.egg")
	if _, err := zegPack(src, egg, 16384, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if probs, _, err := zegVerify(egg, ""); err != nil || len(probs) != 0 {
		t.Fatalf("干净袋不该有问题：probs=%v err=%v", probs, err)
	}
	// 改数据区一字节（落在 a.txt 的 5 字节载荷里，off=16384）
	d, _ := os.ReadFile(egg)
	if len(d) < 16384+20 {
		t.Fatal("卵太小")
	}
	d[16384+2] ^= 0xff
	bad := filepath.Join(dir, "bad.egg")
	if err := os.WriteFile(bad, d, 0o644); err != nil {
		t.Fatal(err)
	}
	probs, _, err := zegVerify(bad, "")
	if err != nil {
		t.Fatal(err)
	}
	hit := false
	for _, p := range probs {
		if strings.Contains(p, "sha256") {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("改字节未被检出：probs=%v", probs)
	}
	// 截断
	trunc := filepath.Join(dir, "trunc.egg")
	if err := os.WriteFile(trunc, d[:16384+1], 0o644); err != nil {
		t.Fatal(err)
	}
	if probs, _, err := zegVerify(trunc, ""); err != nil || len(probs) == 0 {
		t.Fatalf("截断未被检出：probs=%v err=%v", probs, err)
	}
}

// ---- ③ ★ A 类止血牙（哑化 `defer os.RemoveAll(t)` ⇒ 本用例必红）--------------------------------

func TestZergEggSelfTestRecyclesTemp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	before := zegTCountEntries(t, dir)
	var out, errb strings.Builder
	if rc := zegSelfTest(&out, &errb); rc != exitOK {
		t.Fatalf("自检 rc=%d stderr=%s", rc, errb.String())
	}
	if !strings.Contains(out.String(), "self-test 通过（10 项：3 正例 + 4 负例 + 3 条 xfer 用例）") {
		t.Fatalf("自检成功行不符：%q", out.String())
	}
	after := zegTCountEntries(t, dir)
	if after != before {
		t.Fatalf("自检遗留临时目录：before=%d after=%d（A 类泄漏未止血）", before, after)
	}
}

func zegTCountEntries(t *testing.T, dir string) int {
	t.Helper()
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(des)
}

// ---- ④ 命令行面（rc 与 py 逐字对齐）---------------------------------------------------------

func TestZergEggArgvFaces(t *testing.T) {
	// 无子命令 ⇒ 打帮助 + 退 64
	var o, e strings.Builder
	if rc := zergEggGateMain(&o, &e, nil); rc != zegExitToolUsage {
		t.Fatalf("无子命令 rc=%d want=%d", rc, zegExitToolUsage)
	}
	if o.String() != zegHelpText {
		t.Fatalf("无子命令 stdout 非帮助正文")
	}
	// 未知旗标 ⇒ 退 2，stderr 带 argparse 式错误串
	o.Reset()
	e.Reset()
	if rc := zergEggGateMain(&o, &e, []string{"--bogus"}); rc != exitUsage {
		t.Fatalf("未知旗标 rc=%d want=2", rc)
	}
	if !strings.Contains(e.String(), "error: unrecognized arguments: --bogus") {
		t.Fatalf("未知旗标 stderr 不符：%q", e.String())
	}
	// 非法子命令 ⇒ 退 2
	o.Reset()
	e.Reset()
	if rc := zergEggGateMain(&o, &e, []string{"zzz"}); rc != exitUsage {
		t.Fatalf("非法子命令 rc=%d want=2", rc)
	}
	if !strings.Contains(e.String(), "invalid choice: 'zzz'") {
		t.Fatalf("非法子命令 stderr 不符：%q", e.String())
	}
	// pack 缺实参 ⇒ 退 64
	o.Reset()
	e.Reset()
	if rc := zergEggGateMain(&o, &e, []string{"pack"}); rc != zegExitToolUsage {
		t.Fatalf("pack 缺参 rc=%d want=64", rc)
	}
	if !strings.Contains(e.String(), "用法：pack") {
		t.Fatalf("pack 缺参 stderr 不符：%q", e.String())
	}
}
