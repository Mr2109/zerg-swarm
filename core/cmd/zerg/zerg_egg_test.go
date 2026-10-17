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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zegGoldenEggSHA —— 固定 fixture（`a.txt`="hello" · `sub/b.bin`=0x00..0xFF · 均 0644 ·
// `--now 2000-01-01T00:00:00Z` · align 16384）下 Go 侧 pack 产物的 sha256。
//
// ★ **2026-10-15（卵1① 段语义对齐 OCI `mediaType`）起不再与 py 原件逐字节同** —— `entries[]` 逐段**多**
// 一个 `mediaType` 键（`§9.11②` 的**纯声明** · `§11.2` 不变量 7 的**可加面** ⇒ 旧读端**忽略且不报错**；
// py 真身 `scripts/node-a/zerg-egg.py` 已登记退场、盘上无此件）。本值 = **改后现跑**照抄（✗ 不手编）。
const zegGoldenEggSHA = "9c843c0ffc423beb52d5c408b66f154a1238079fc2c133bbab86a373a460d672"

// zegGoldenIndexJSON —— 同一 fixture 的索引 JSON **逐字节**（字段序即契约：
// `path/off/size/sha256/mode/mediaType` —— ★ `mediaType` 由 卵1① 追加在**末位**，✗ 不改前五格序）。
const zegGoldenIndexJSON = `{"magic":"ZEGG","version":1,"align":16384,"tool":{"name":"zerg-egg","version":"1.0.0"},"created_at":"2000-01-01T00:00:00Z","entries":[{"path":"a.txt","off":16384,"size":5,"sha256":"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824","mode":420,"mediaType":"application/vnd.oci.image.config.v1+json"},{"path":"sub/b.bin","off":32768,"size":256,"sha256":"40aff2e9d2d8922e47afd4648e6967497158785fbd1da870e7110266bf944880","mode":420,"mediaType":"application/vnd.oci.image.config.v1+json"}]}`

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

// ---- ⑤ ★ 卵1①② 四态亲测（真命令面 ＋ 真造件）＋ 两枚负控 -----------------------------------------

// TestZergEggMediaPathFourStates —— 卵1①② 的机检件：四态**走真命令面**（`zergEggGateMain`）取
// rc/stdout，✗ 不只调内部函数；负控 = 同一份输入走**不审计**的旧形态必须放行（＝本牙非永真）。
func TestZergEggMediaPathFourStates(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tree")
	if err := os.MkdirAll(filepath.Join(src, "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte("{\"arch\":\"four-states\"}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "model", "w.safetensors"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	egg := filepath.Join(dir, "four.egg")
	if _, err := zegPack(src, egg, 4096, "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	clone := func(name string, edit func(ents []map[string]json.RawMessage) error) string {
		t.Helper()
		pp := filepath.Join(dir, name)
		d, err := os.ReadFile(egg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(pp, d, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := zegVerRebuildEgg(pp, func(idx []byte) ([]byte, error) {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(idx, &m); err != nil {
				return nil, err
			}
			var ents []map[string]json.RawMessage
			if err := json.Unmarshal(m["entries"], &ents); err != nil {
				return nil, err
			}
			if err := edit(ents); err != nil {
				return nil, err
			}
			eb, err := json.Marshal(ents)
			if err != nil {
				return nil, err
			}
			m["entries"] = eb
			return json.Marshal(m)
		}, 0); err != nil {
			t.Fatal(err)
		}
		return pp
	}

	// 态①：正常卵 ⇒ `info` 逐段印 `mediaType=`（取值落 OCI 闭集内）＋ 写端 ⟷ 读端逐条一致
	var o1, e1 strings.Builder
	if rc := zergEggGateMain(&o1, &e1, []string{"info", egg}); rc != exitOK {
		t.Fatalf("态① info rc=%d stderr=%q", rc, e1.String())
	}
	out1 := o1.String()
	if !strings.Contains(out1, "mediaType="+zegMediaConfig) {
		t.Fatalf("态① info 没有 config 类的 mediaType：%q", out1)
	}
	if !strings.Contains(out1, "mediaType="+zegMediaLayer) {
		t.Fatalf("态① info 没有 layer 类的 mediaType：%q", out1)
	}
	idx, err := zegParseIndex(egg)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range idx.Entries {
		if e.MediaType != zegMediaTypeOfPath(e.Path) {
			t.Fatalf("态① 段 %q 的 mediaType=%q ≠ 表推出 %q", e.Path, e.MediaType, zegMediaTypeOfPath(e.Path))
		}
	}
	t.Logf("态① info 读数：%s", strings.ReplaceAll(strings.TrimSpace(out1), "\n", " | "))

	// 态②：`mediaType` 缺失 / 闭集外 ⇒ 照 OCI **放行 ＋ 登记**（✗ 不 error）
	oldEgg := clone("old.egg", func(ents []map[string]json.RawMessage) error {
		for i := range ents {
			delete(ents[i], "mediaType")
		}
		return nil
	})
	var o2, e2 strings.Builder
	if rc := zergEggGateMain(&o2, &e2, []string{"verify", oldEgg}); rc != exitOK {
		t.Fatalf("态② 未标档 verify rc=%d stderr=%q", rc, e2.String())
	}
	if !strings.Contains(o2.String(), "未标 mediaType") {
		t.Fatalf("态② 未标档未逐字印「未标 mediaType」：%q", o2.String())
	}
	t.Logf("态② 未标读数：%q", strings.TrimSpace(o2.String()))
	unkEgg := clone("unk.egg", func(ents []map[string]json.RawMessage) error {
		ents[0]["mediaType"] = json.RawMessage("\"application/vnd.zerg.weights.v9+unknown\"")
		return nil
	})
	var o3, e3 strings.Builder
	if rc := zergEggGateMain(&o3, &e3, []string{"verify", unkEgg}); rc != exitOK {
		t.Fatalf("态② 闭集外 verify rc=%d stderr=%q", rc, e3.String())
	}
	if !strings.Contains(o3.String(), "未知 mediaType") {
		t.Fatalf("态② 闭集外未逐字印「未知 mediaType」：%q", o3.String())
	}
	t.Logf("态② 闭集外读数：%q", strings.TrimSpace(o3.String()))

	// 态③：恶意路径逐类 ⇒ 必拒且**点名规则**
	cases := []struct{ name, bad, rule string }{
		{"..", "../x", "path_dotdot"},
		{"绝对", "/etc/passwd", "path_absolute"},
		{"盘符", "C:/x", "path_windows_drive"},
		{"空段", "a//b", "path_not_normalized"},
		{"NUL", "a\x00b", "path_nul"},
		{"尾随点", "a.", "path_trailing_dot_space"},
		{"保留名", "con.txt", "path_windows_reserved"},
		{"反斜杠", "a\\b", "path_backslash"},
		{"非NFC", "cafe\u0301.json", "path_not_nfc"},
	}
	for i, c := range cases {
		badPath := c.bad
		name, rule := c.name, c.rule
		badEgg := clone("bad-"+itoaForLog(i)+".egg", func(ents []map[string]json.RawMessage) error {
			qb, err := json.Marshal(badPath)
			if err != nil {
				return err
			}
			ents[0]["path"] = json.RawMessage(qb)
			return nil
		})
		_, perr := zegParseIndex(badEgg)
		if perr == nil {
			t.Fatalf("态③ %s 恶意路径竟放行", name)
		}
		ref, ok := perr.(*zegRefusal)
		if !ok || ref.Tier != "D9" || !strings.Contains(ref.Msg, rule) {
			t.Fatalf("态③ %s 未点名 D9/%s：%v", name, rule, perr)
		}
		t.Logf("态③ %s ⇒ rc≠0 · %v", name, perr)
	}

	// 态④：同键换语义（闭集内却与该段 `path` 推出的类不符）⇒ 必拒且点名档位
	semEgg := clone("sem.egg", func(ents []map[string]json.RawMessage) error {
		for i := range ents {
			var p string
			if json.Unmarshal(ents[i]["path"], &p) == nil && p == "config.json" {
				qb, jerr := json.Marshal(zegMediaLayer)
				if jerr != nil {
					return jerr
				}
				ents[i]["mediaType"] = json.RawMessage(qb)
			}
		}
		return nil
	})
	_, serr := zegParseIndex(semEgg)
	if serr == nil {
		t.Fatalf("态④ 同键换语义竟放行")
	}
	if ref, ok := serr.(*zegRefusal); !ok || ref.Tier != zegTierSurface {
		t.Fatalf("态④ 未点名 %s：%v", zegTierSurface, serr)
	}
	t.Logf("态④ ⇒ rc≠0 · %v", serr)

	// 负控 ①：同一份「未标」输入走**不审计**的旧形态 ⇒ 必须放行（＝本牙非永真）
	d, err := os.ReadFile(oldEgg)
	if err != nil {
		t.Fatal(err)
	}
	n := zegIdxLenOf(d)
	if n <= 0 {
		t.Fatal("负控 ① 取不到索引体（夹具坏）")
	}
	body := d[zegHeaderPrefix : zegHeaderPrefix+n]
	if _, aerr := zegCompatGateIndexBody(body); aerr != nil {
		t.Fatalf("负控 ① 旧形态在未标档上竟报错：%v", aerr)
	}
	t.Logf("负控 ① 旧形态（不审计）在同一份未标输入上 ⇒ 放行（牙非永真）")

	// 负控 ②：符号链接「只许相对链」四道具名拒（`GAP-20261018-05`）—— 四条各自点名、互不串名
	symAll := []string{"fs_symlink_absolute", "fs_symlink_escape", "fs_symlink_cycle", "fs_symlink_depth"}
	for _, c := range []struct {
		name string
		rule string
		mk   func(d string) error
	}{
		{"absolute", "fs_symlink_absolute", func(d string) error {
			return os.Symlink("/etc/hosts", filepath.Join(d, "link.bin"))
		}},
		{"escape", "fs_symlink_escape", func(d string) error {
			return os.Symlink("../outside.bin", filepath.Join(d, "link.bin"))
		}},
		{"cycle", "fs_symlink_cycle", func(d string) error {
			if err := os.Symlink("b.bin", filepath.Join(d, "a.bin")); err != nil {
				return err
			}
			return os.Symlink("a.bin", filepath.Join(d, "b.bin"))
		}},
		{"depth", "fs_symlink_depth", func(d string) error {
			for i := 0; i < 10; i++ {
				tgt := fmt.Sprintf("l%02d.bin", i+1)
				if i == 9 {
					tgt = "real.bin"
				}
				if err := os.Symlink(tgt, filepath.Join(d, fmt.Sprintf("l%02d.bin", i))); err != nil {
					return err
				}
			}
			return nil
		}},
	} {
		sd := filepath.Join(dir, "lsrc-"+c.name)
		if err := os.MkdirAll(sd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sd, "real.bin"), []byte("z"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := c.mk(sd); err != nil {
			t.Fatal(err)
		}
		_, lerr := zegPack(sd, filepath.Join(dir, "l-"+c.name+".egg"), 4096, "")
		if lerr == nil {
			t.Fatalf("负控 ②/%s 符号链接源件竟被打包", c.name)
		}
		if !strings.Contains(lerr.Error(), c.rule) {
			t.Fatalf("负控 ②/%s 拒了但没点名 %s：%v", c.name, c.rule, lerr)
		}
		for _, other := range symAll {
			if other != c.rule && strings.Contains(lerr.Error(), other) {
				t.Fatalf("负控 ②/%s 串名（本该 %s，报话里还有 %s）：%v", c.name, c.rule, other, lerr)
			}
		}
		t.Logf("负控 ②/%s ⇒ 拒[%s]", c.name, c.rule)
	}

	// 链尾：`--self-test` 四行（旧档 / 卵3① / 卵6 / 卵1）—— ★ 前三行必须**逐字未漂**
	var so, se strings.Builder
	if rc := zergEggGateMain(&so, &se, []string{"--self-test"}); rc != exitOK {
		t.Fatalf("--self-test rc=%d stderr=%q", rc, se.String())
	}
	for _, frag := range []string{
		"self-test 通过（10 项：3 正例 + 4 负例 + 3 条 xfer 用例）",
		"self-test（卵3① 并行＋缓存）通过（7 项：3 正例 + 4 负例）",
		"self-test（卵6 版本与兼容）通过（4 态 ＋ 1 负控）",
		"self-test（卵1 格式与工具面）通过（4 态 ＋ 4 负控 ＋ 1 正控）",
	} {
		if !strings.Contains(so.String(), frag) {
			t.Fatalf("--self-test 缺行 %q：%q", frag, so.String())
		}
	}
	lines := strings.Split(strings.TrimRight(so.String(), "\n"), "\n")
	lineN := len(lines)
	if lineN != 4 {
		t.Fatalf("--self-test 行数 = %d ≠ 4（四档必须分开印）", lineN)
	}
	t.Logf("--self-test 末行（卵1 那行）= %q", lines[lineN-1])
}
