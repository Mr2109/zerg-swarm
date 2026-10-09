// isolate_ro_test.go —— 件 core/cmd/zerg/isolate_ro.go（scripts/dev/isolate-ro.sh 的 Go 真搬）的回归牙：
//
//	① 入口三档（usage / rc / 位参解析）
//	② 逐条成对控（--self-test 全格）＋ 关键判定面（危险链接 / --dst 包含性 / 用法错）
//	③ ★ 金标内化（旧 sh 对拍退场 ⇒ 逐格金标收成 Go 常量 · 两态 · ✗ 不 Skip）。
//
// 夹具一律落**仓外**（t.TempDir() ⇒ $TMPDIR · ✗ 不落仓内：门76 会把盘上任何新件算进分母）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ── 入口三档 ─────────────────────────────────────────────────────────────────

func isoT_runEntry(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var ob, eb bytes.Buffer
	rc := isolateRoGateMain(&ob, &eb, argv)
	return rc, ob.String(), eb.String()
}

func TestIsolateRoHelpAndUsage(t *testing.T) {
	rc, out, _ := isoT_runEntry(t, "--help")
	if rc != 0 {
		t.Fatalf("--help：rc=%d（要 0）", rc)
	}
	if !strings.Contains(out, "三个子命令") || !strings.Contains(out, "isolate-ro.sh") {
		t.Fatalf("--help 未印出原件 usage 正文：\n%s", out)
	}
	// 无子命令 ⇒ 印同一 usage 且 rc=2（照原件 case ""）。
	rc2, out2, _ := isoT_runEntry(t)
	if rc2 != 2 || out2 != out {
		t.Fatalf("无参：rc=%d（要 2）· usage 与 --help 是否逐字同=%v", rc2, out2 == out)
	}
}

func TestIsolateRoUnknownFlagAndSubcommand(t *testing.T) {
	if rc, _, errb := isoT_runEntry(t, "--bogus"); rc != 2 || !strings.Contains(errb, "未知旗标 --bogus") {
		t.Fatalf("未知旗标：rc=%d err=%q", rc, errb)
	}
	if rc, _, errb := isoT_runEntry(t, "bogus"); rc != 2 || !strings.Contains(errb, "多余的位置参数") {
		t.Fatalf("未知子命令：rc=%d err=%q", rc, errb)
	}
	if rc, _, errb := isoT_runEntry(t, "new"); rc != 2 || !strings.Contains(errb, "new 缺 <真仓>") {
		t.Fatalf("new 缺参：rc=%d err=%q", rc, errb)
	}
	if rc, _, _ := isoT_runEntry(t, "verify", "/tmp"); rc != 2 {
		t.Fatalf("verify 缺一实参：rc=%d（要 2）", rc)
	}
}

// ── 自检全格（--self-test）────────────────────────────────────────────────────

func TestIsolateRoSelfTestAllGreen(t *testing.T) {
	rc, out, errb := isoT_runEntry(t, "--self-test")
	if rc != 0 {
		t.Fatalf("--self-test：rc=%d（要 0）\n%s%s", rc, out, errb)
	}
	if !strings.Contains(out, "自检结论：全过") {
		t.Fatalf("自检结论非全过：\n%s", out)
	}
	if strings.Contains(out, "✗") {
		t.Fatalf("自检有红格：\n%s", out)
	}
	// 抽样钉住几条关键断言真的在（✗ 不削断言）。
	for _, need := range []string{
		"正控：verify ⇒ rc=0", "负控 A：真仓被改 ⇒ verify rc=1",
		"负控 B'：--reject-symlinks ⇒ rc=1 且点出链接",
		"--only 单件：副本里只有那件 + 清单", "--list：零写盘",
		"包含性：--dst 落在真仓内 ⇒ rc=2",
	} {
		if !strings.Contains(out, need) {
			t.Fatalf("自检缺少该格：%s\n%s", need, out)
		}
	}
}

// ── 关键判定面（有牙：会真判危险形态与包含性）──────────────────────────────────

func isoT_seed(t *testing.T, root string) {
	t.Helper()
	for _, d := range []string{
		root + "/real/core/cmd/zerg/testdata", root + "/real/scripts/dev",
		root + "/real/bin", root + "/real/ui/target", root + "/real/sub",
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	isoT_w(t, root+"/real/core/cmd/zerg/testdata/cli-matrix.json", "matrix\n")
	isoT_w(t, root+"/real/scripts/dev/keep.sh", "keep\n")
	isoT_w(t, root+"/real/bin/big.bin", "heavy\n")
	isoT_w(t, root+"/real/ui/target/artifact.bin", "big\n")
	isoT_w(t, root+"/real/sub/aaa.txt", "aaa\n")
	isoT_w(t, root+"/real/top.txt", "top\n")
	// 一条指向副本之外的符号链接（隔离时须被换成硬拷贝 / 严格档被拒）。
	if err := os.Symlink(root+"/real/core/cmd/zerg/testdata/cli-matrix.json", root+"/real/sub/link_out.txt"); err != nil {
		t.Fatal(err)
	}
}

func isoT_w(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsolateRoDangerousLinkAndDstInside(t *testing.T) {
	root := t.TempDir()
	isoT_seed(t, root)

	// 缺省档：外指链接被换成硬拷贝 ⇒ rc=0 且副本里 0 条链接。
	rc, out, errb := isoT_runEntry(t, "new", root+"/real", "--dst", root+"/tree")
	if rc != 0 {
		t.Fatalf("new 缺省档：rc=%d\n%s%s", rc, out, errb)
	}
	if n := len(isoFindSymlinkRels(root + "/tree")); n != 0 {
		t.Fatalf("副本里仍有 %d 条链接（外指链接应被换成硬拷贝）", n)
	}
	if !isoIsFile(root + "/tree/sub/link_out.txt") {
		t.Fatalf("外指链接未被换成真件")
	}

	// 严格档：外指链接 ⇒ rc=1 且点名。
	rc, _, errb = isoT_runEntry(t, "new", root+"/real", "--dst", root+"/tree2", "--reject-symlinks")
	if rc != 1 || !strings.Contains(errb, "link_out.txt") {
		t.Fatalf("--reject-symlinks：rc=%d err=%q", rc, errb)
	}

	// --dst 落在真仓内 ⇒ rc=2，且真仓里没被建出 inside/。
	rc, _, errb = isoT_runEntry(t, "new", root+"/real", "--dst", root+"/real/inside")
	if rc != 2 || !strings.Contains(errb, "真仓内") {
		t.Fatalf("--dst 真仓内：rc=%d err=%q", rc, errb)
	}
	if isoExists(root + "/real/inside") {
		t.Fatalf("拒写后真仓里仍被建出 inside/（有副作用）")
	}
}

// ── 内化金标：旧 sh 对拍（恒 Skip）的逐格金标收成 Go 常量（现算现冻 · ✗ 不臆造）─────────────
//
// 为什么这么测（M2 口径 · 与 `atomic_write_test.go` 的 `TestAwTwoStatesGoldenInternalized` 同族）：
// 本件原「sh 真身 ⟷ Go 真搬」逐字对拍（旧名 `TestIsolateRoTwoStateParity`）的对拍源
// `scripts/dev/isolate-ro.sh` 已退场（M2 批）⇒ 对拍源消失、断言恒 Skip（假绿、零保护）。照
// `atomic_write_test.go`（体例最新）＋ `check_verify_cmd_real_test.go` 把金标真源**收回 Go 半
// 现役行为本身**：
//
//	★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：逐格「seed + argv ⇒
//	  (rc, stdout, stderr)」由 `isolateRoGateMain` 现跑捕获、归一后冻成下表 wantRC/wantOut/wantErr；
//	  **本机时点/位置类量**在比对前归一化（✗ 不冻结 —— 因其本非常量）：
//	    ① 夹具根 `t.TempDir()`（随机路径 + macOS /var↔/private/var）⇒ `<FX>`；
//	    ② `$TMPDIR`（自检 / `new` 自建的临时树）⇒ `<TMP>`；
//	    ③ 自检临时名 `isolate-ro-selftest.<6 位随机>` ⇒ `isolate-ro-selftest.<RND>`；`isolate-ro.<6 位随机>` ⇒ `isolate-ro.<RND>`；
//	    ④ 字节数（`list` 预演按**符号链接自身尺寸**计入 —— 随夹具根路径长度漂）⇒ `字节：<B>（<B> B）`。
//
// 两态（✗ 不许 Skip）：A 态 = 夹具无 sh（真仓现态）；B 态 = 合成「旧态」夹具（把退役 sh 路径
// `scripts/dev/isolate-ro.sh` 放回）⇒ 两态**对同一套金标逐字同命中**，证 sh 在/不在盘都不改判据结论。
//
// ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 `isolate_ro.go` 的一臂哑化 ⇒ 本用例必 FAIL 且
// 带行号（✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。

var (
	isoT_reSelfTmp = regexp.MustCompile(`isolate-ro-selftest\.[A-Za-z0-9]{6}`)
	isoT_reMktmp   = regexp.MustCompile(`isolate-ro\.[A-Za-z0-9]{6}`)
	// 字节数（`list` 预演把符号链接按自身尺寸计入 ⇒ 随夹具根路径长度漂）。
	isoT_reBytes = regexp.MustCompile(`字节：\d+（[\d.]+ B）`)
)

// isoTNormGolden —— 归一本机时点/位置类量（夹具根 / $TMPDIR / 临时随机名 / 路径长度相关的字节数）
// ⇒ 金标跨机可复现。
func isoTNormGolden(s string, roots ...string) string {
	for _, r := range roots {
		if r != "" {
			s = strings.ReplaceAll(s, r, "<FX>")
		}
	}
	s = isoT_reSelfTmp.ReplaceAllString(s, "isolate-ro-selftest.<RND>")
	s = isoT_reMktmp.ReplaceAllString(s, "isolate-ro.<RND>")
	s = isoT_reBytes.ReplaceAllString(s, "字节：<B>（<B> B）")
	if td := os.TempDir(); td != "" {
		s = strings.ReplaceAll(s, strings.TrimRight(td, "/")+"/", "<TMP>")
	}
	return s
}

func isoT_repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "core", "internal", "version", "version.go")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("从 %s 上溯找不到仓根", wd)
		}
		dir = parent
	}
}

// isoT_goHalfRel —— 哨兵锚：Go 半（✗ 不锚已退场的 sh —— 一退役即失锚）。
const isoT_goHalfRel = "core/cmd/zerg/isolate_ro.go"

// isoT_assertGoHalf —— 哨兵：锚 **Go 半**在盘（✗ 不再锚已退场 sh）；找不到 ⇒ fail-closed（缺件 ≠ 绿 ·
// ✗ 不留 Skip）。旧版对 sh 缺席就 `t.Skipf`（恒 Skip · 零保护）—— 本改把它换成硬前置。
func isoT_assertGoHalf(t *testing.T, repo string) {
	t.Helper()
	if !isoIsFile(filepath.Join(repo, filepath.FromSlash(isoT_goHalfRel))) {
		t.Fatalf("哨兵失守：未见 Go 半 %s ⇒ fail-closed（缺件 ≠ 绿）", isoT_goHalfRel)
	}
}

// isoT_copyTree 递归复制（供 verify 格造「副本」）。
func isoT_copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if fi, lerr := os.Lstat(s); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
			tgt, rerr := os.Readlink(s)
			if rerr != nil {
				t.Fatal(rerr)
			}
			if serr := os.Symlink(tgt, d); serr != nil {
				t.Fatal(serr)
			}
			continue
		}
		if e.IsDir() {
			isoT_copyTree(t, s, d)
			continue
		}
		b, rerr := os.ReadFile(s)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if werr := os.WriteFile(d, b, 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
}

type isoTGoldenCase struct {
	name    string
	seed    func(t *testing.T, root string)
	argv    func(root string) []string
	wantRC  int
	wantOut string // 归一化后（isoTNormGolden 现算自证 · ✗ 手写）
	wantErr string
}

// isoTGoldenScens —— 金标真源（23 格 · seed/argv 与原对拍逐格同 · want* 由 `isolateRoGateMain` 现跑捕获）。
func isoTGoldenScens(t *testing.T) []isoTGoldenCase {
	t.Helper()
	verifyTree := func(t *testing.T, root string) {
		isoT_seed(t, root)
		isoT_copyTree(t, root+"/real", root+"/tree")
	}
	return []isoTGoldenCase{
		{name: "help", argv: func(d string) []string { return []string{"--help"} }, wantRC: 0, wantOut: "zerg-class: write D2\nisolate-ro.sh —— **只读隔离**：给「在别处试」一条不会误删真仓的路（根因 `GAP-20260926-20` P0）。\n\n事故形态（本件要防的那一件）：一枚卵为了「隔离验证」自建临时目录 + **符号链接**农场，\n  其中一条 `ln -s <真仓>/core/cmd/zerg/testdata <农场>/…/testdata`，随后\n  `rm -f <农场>/…/cli-matrix.json` —— 删掉的其实是**真仓的件**（符号链接把删除动作透传回去了）。\n⇒ 规矩只有一条：**「在别处试」= 硬拷贝出一棵独立的工作树**；真仓只读、绝不建链接、绝不删。\n\n三个子命令\n--------\n  new    <真仓>   [--dst <目录>] [--include <glob>]… [--only <件路径>]… [--exclude <路径>]… [--all]\n                  [--reject-symlinks] [--no-manifest] [--quiet] [--list]\n      ★ `--only <件路径>`：**单件直取** —— 只取该一个件（父目录骨架 + 清单，别的一件不取）；可多次（`GAP-20260926-47`）。\n      ★ `--list`：**预演** —— 先列会取哪些件 + 件数 + 字节，**一个字节不写**（`GAP-20260926-47` · 零写盘）。\n      在 `${TMPDIR:-/tmp}` 下硬拷贝出一棵**可随意读写**的工作树；缺省排除重件\n      （`.git` / `bin` / `data` / `dist` / `node_modules` / `vendor` / `ui/target` / `wall/target` / `zerg-wt` …）。\n      ★ 缺省排除是**根锚定**写法（rsync `--exclude=/<名>`）⇒ 名单里的**相对路径**（如 `ui/target`）\n        按仓根逐条锚定；脚本开头会**逐行印出生效的排除清单**（清面必须可见 · `GAP-20260927-96`）。\n      ★ 旗标接受 `--exclude <值>` 与 `--exclude=<值>` **两种写法**（同 `--include`/`--dst`/`--max-examples`）。\n      ★ 源路径本身是**符号链接** ⇒ 打印一行说明后**按它解析到的真身硬拷贝**（`--reject-symlinks` ⇒ 直接拒绝）。\n      ★ 副本里任何**指向副本之外**（含指回真仓）的符号链接，一律**换成硬拷贝**（`--reject-symlinks` ⇒ 拒绝并点名）。\n      ★ 拷贝完在副本根写 `ISOLATED.md`（「这是副本，删无妨；真仓是 …」）+ `ISOLATED.manifest.tsv`（逐件 sha256）。\n      输出：真仓路径 · 副本路径 · 件数 · 总字节。\n\n  verify <副本> <真仓> [--quiet] [--max-examples N]\n      逐件 sha256 **对拍副本与真仓**（副本侧优先取 `new` 留下的 manifest，缺则现算）\n      ⇒ 用来**证明这一轮验证过程一个字节都没改到真仓**。rc：0 全一致 · 1 有差异（点名）· 2 判不了。\n\n  self-test\n      `--self-test` 三档：正控（隔离→写副本→对拍 rc=0）· 负控（改副本 ⇒ rc=1；真仓里造回真仓的链接 ⇒ 被拒/被换）\n      · 用法错（缺参数 / 真仓不存在 ⇒ rc=2）。合成夹具全在临时目录，**一件真东西都不碰**。\n\n用法\n----\n    bash scripts/dev/isolate-ro.sh new \"<真仓>\" --include 'core/cmd/zerg/testdata/*'\n    bash scripts/dev/isolate-ro.sh verify \"$TREE\" \"<真仓>\"\n    bash scripts/dev/isolate-ro.sh --self-test\n\n退码：0 正常 · 1 危险形态（拒绝的符号链接 / 对拍有差异）· 2 **不给结论**（用法错 / 缺件 / 空转 / 自检不过）\n\n硬规矩（本件自己也守）\n", wantErr: ""},
		{name: "noargs", argv: func(d string) []string { return nil }, wantRC: 2, wantOut: "zerg-class: write D2\nisolate-ro.sh —— **只读隔离**：给「在别处试」一条不会误删真仓的路（根因 `GAP-20260926-20` P0）。\n\n事故形态（本件要防的那一件）：一枚卵为了「隔离验证」自建临时目录 + **符号链接**农场，\n  其中一条 `ln -s <真仓>/core/cmd/zerg/testdata <农场>/…/testdata`，随后\n  `rm -f <农场>/…/cli-matrix.json` —— 删掉的其实是**真仓的件**（符号链接把删除动作透传回去了）。\n⇒ 规矩只有一条：**「在别处试」= 硬拷贝出一棵独立的工作树**；真仓只读、绝不建链接、绝不删。\n\n三个子命令\n--------\n  new    <真仓>   [--dst <目录>] [--include <glob>]… [--only <件路径>]… [--exclude <路径>]… [--all]\n                  [--reject-symlinks] [--no-manifest] [--quiet] [--list]\n      ★ `--only <件路径>`：**单件直取** —— 只取该一个件（父目录骨架 + 清单，别的一件不取）；可多次（`GAP-20260926-47`）。\n      ★ `--list`：**预演** —— 先列会取哪些件 + 件数 + 字节，**一个字节不写**（`GAP-20260926-47` · 零写盘）。\n      在 `${TMPDIR:-/tmp}` 下硬拷贝出一棵**可随意读写**的工作树；缺省排除重件\n      （`.git` / `bin` / `data` / `dist` / `node_modules` / `vendor` / `ui/target` / `wall/target` / `zerg-wt` …）。\n      ★ 缺省排除是**根锚定**写法（rsync `--exclude=/<名>`）⇒ 名单里的**相对路径**（如 `ui/target`）\n        按仓根逐条锚定；脚本开头会**逐行印出生效的排除清单**（清面必须可见 · `GAP-20260927-96`）。\n      ★ 旗标接受 `--exclude <值>` 与 `--exclude=<值>` **两种写法**（同 `--include`/`--dst`/`--max-examples`）。\n      ★ 源路径本身是**符号链接** ⇒ 打印一行说明后**按它解析到的真身硬拷贝**（`--reject-symlinks` ⇒ 直接拒绝）。\n      ★ 副本里任何**指向副本之外**（含指回真仓）的符号链接，一律**换成硬拷贝**（`--reject-symlinks` ⇒ 拒绝并点名）。\n      ★ 拷贝完在副本根写 `ISOLATED.md`（「这是副本，删无妨；真仓是 …」）+ `ISOLATED.manifest.tsv`（逐件 sha256）。\n      输出：真仓路径 · 副本路径 · 件数 · 总字节。\n\n  verify <副本> <真仓> [--quiet] [--max-examples N]\n      逐件 sha256 **对拍副本与真仓**（副本侧优先取 `new` 留下的 manifest，缺则现算）\n      ⇒ 用来**证明这一轮验证过程一个字节都没改到真仓**。rc：0 全一致 · 1 有差异（点名）· 2 判不了。\n\n  self-test\n      `--self-test` 三档：正控（隔离→写副本→对拍 rc=0）· 负控（改副本 ⇒ rc=1；真仓里造回真仓的链接 ⇒ 被拒/被换）\n      · 用法错（缺参数 / 真仓不存在 ⇒ rc=2）。合成夹具全在临时目录，**一件真东西都不碰**。\n\n用法\n----\n    bash scripts/dev/isolate-ro.sh new \"<真仓>\" --include 'core/cmd/zerg/testdata/*'\n    bash scripts/dev/isolate-ro.sh verify \"$TREE\" \"<真仓>\"\n    bash scripts/dev/isolate-ro.sh --self-test\n\n退码：0 正常 · 1 危险形态（拒绝的符号链接 / 对拍有差异）· 2 **不给结论**（用法错 / 缺件 / 空转 / 自检不过）\n\n硬规矩（本件自己也守）\n", wantErr: ""},
		{name: "bogus_flag", argv: func(d string) []string { return []string{"--bogus"} }, wantRC: 2, wantOut: "", wantErr: "用法错：未知旗标 --bogus（见 --help）\n"},
		{name: "bogus_pos", argv: func(d string) []string { return []string{"bogus"} }, wantRC: 2, wantOut: "", wantErr: "用法错：多余的位置参数 bogus（见 --help）\n"},
		{name: "new_missing_arg", argv: func(d string) []string { return []string{"new"} }, wantRC: 2, wantOut: "", wantErr: "用法错：new 缺 <真仓>（见 --help）\n"},
		{name: "new_absent_repo", argv: func(d string) []string { return []string{"new", d + "/no-such"} }, wantRC: 2, wantOut: "", wantErr: "判不了：真仓目录不存在 —— <FX>/no-such\n"},
		{name: "verify_missing_arg", argv: func(d string) []string { return []string{"verify", d + "/tree"} }, wantRC: 2, wantOut: "", wantErr: "用法错：verify 要 <副本> <真仓> 两个实参（见 --help）\n"},
		{name: "extra_pos", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "extra"}
		}, wantRC: 2, wantOut: "", wantErr: "用法错：多余的位置参数 extra（见 --help）\n"},
		{name: "new_basic", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree"}
		}, wantRC: 0, wantOut: "== 只读隔离（硬拷贝）==\n· 真仓：<FX>/real\n· 副本：<FX>/tree\n· 生效排除（rsync 过滤器 · 现读实际生效）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n  · 链接→硬拷贝：sub/link_out.txt ⇐ <FX>/real/core/cmd/zerg/testdata/cli-matrix.json\n== 读数 ==\n· 真仓路径：<FX>/real\n· 副本路径：<FX>/tree\n· 件数：5 · 总字节：<B>（<B> B）\n· 链接→硬拷贝：1 条 · ISOLATED.md：<FX>/tree/ISOLATED.md\n· 验回：bash scripts/dev/isolate-ro.sh verify \"<FX>/tree\" \"<FX>/real\"\n· 收尾（本件**不会**替你删）：rm -rf \"<FX>/tree\"\n", wantErr: ""},
		{name: "new_no_manifest", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--no-manifest"}
		}, wantRC: 0, wantOut: "== 只读隔离（硬拷贝）==\n· 真仓：<FX>/real\n· 副本：<FX>/tree\n· 生效排除（rsync 过滤器 · 现读实际生效）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n  · 链接→硬拷贝：sub/link_out.txt ⇐ <FX>/real/core/cmd/zerg/testdata/cli-matrix.json\n== 读数 ==\n· 真仓路径：<FX>/real\n· 副本路径：<FX>/tree\n· 件数：5 · 总字节：<B>（<B> B）\n· 链接→硬拷贝：1 条 · ISOLATED.md：<FX>/tree/ISOLATED.md\n· 验回：bash scripts/dev/isolate-ro.sh verify \"<FX>/tree\" \"<FX>/real\"\n· 收尾（本件**不会**替你删）：rm -rf \"<FX>/tree\"\n", wantErr: ""},
		{name: "new_exclude_eq", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--exclude=bin"}
		}, wantRC: 0, wantOut: "== 只读隔离（硬拷贝）==\n· 真仓：<FX>/real\n· 副本：<FX>/tree\n· 生效排除（rsync 过滤器 · 现读实际生效）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n    --exclude=bin\n  · 链接→硬拷贝：sub/link_out.txt ⇐ <FX>/real/core/cmd/zerg/testdata/cli-matrix.json\n== 读数 ==\n· 真仓路径：<FX>/real\n· 副本路径：<FX>/tree\n· 件数：5 · 总字节：<B>（<B> B）\n· 链接→硬拷贝：1 条 · ISOLATED.md：<FX>/tree/ISOLATED.md\n· 验回：bash scripts/dev/isolate-ro.sh verify \"<FX>/tree\" \"<FX>/real\"\n· 收尾（本件**不会**替你删）：rm -rf \"<FX>/tree\"\n", wantErr: ""},
		{name: "new_all", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--all"}
		}, wantRC: 0, wantOut: "== 只读隔离（硬拷贝）==\n· 真仓：<FX>/real\n· 副本：<FX>/tree\n· 生效排除（rsync 过滤器 · 现读实际生效）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n  · 链接→硬拷贝：sub/link_out.txt ⇐ <FX>/real/core/cmd/zerg/testdata/cli-matrix.json\n== 读数 ==\n· 真仓路径：<FX>/real\n· 副本路径：<FX>/tree\n· 件数：5 · 总字节：<B>（<B> B）\n· 链接→硬拷贝：1 条 · ISOLATED.md：<FX>/tree/ISOLATED.md\n· 验回：bash scripts/dev/isolate-ro.sh verify \"<FX>/tree\" \"<FX>/real\"\n· 收尾（本件**不会**替你删）：rm -rf \"<FX>/tree\"\n", wantErr: ""},
		{name: "new_include", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--include", "core/cmd/zerg/testdata/*"}
		}, wantRC: 0, wantOut: "== 只读隔离（硬拷贝）==\n· 真仓：<FX>/real\n· 副本：<FX>/tree\n· 白名单 --include：core/cmd/zerg/testdata/*\n· 生效排除（rsync 过滤器 · 现读实际生效）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n    --exclude=*\n== 读数 ==\n· 真仓路径：<FX>/real\n· 副本路径：<FX>/tree\n· 件数：1 · 总字节：<B>（<B> B）\n· 链接→硬拷贝：0 条 · ISOLATED.md：<FX>/tree/ISOLATED.md\n· 验回：bash scripts/dev/isolate-ro.sh verify \"<FX>/tree\" \"<FX>/real\"\n· 收尾（本件**不会**替你删）：rm -rf \"<FX>/tree\"\n", wantErr: ""},
		{name: "new_only", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--only", "scripts/dev/keep.sh"}
		}, wantRC: 0, wantOut: "== 只读隔离（硬拷贝）==\n· 真仓：<FX>/real\n· 副本：<FX>/tree\n· 白名单 --include：scripts/dev/keep.sh\n· 生效排除（rsync 过滤器 · 现读实际生效）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n    --exclude=*\n== 读数 ==\n· 真仓路径：<FX>/real\n· 副本路径：<FX>/tree\n· 件数：1 · 总字节：<B>（<B> B）\n· 链接→硬拷贝：0 条 · ISOLATED.md：<FX>/tree/ISOLATED.md\n· 验回：bash scripts/dev/isolate-ro.sh verify \"<FX>/tree\" \"<FX>/real\"\n· 收尾（本件**不会**替你删）：rm -rf \"<FX>/tree\"\n", wantErr: ""},
		{name: "new_only_missing", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--only", "no/such.txt"}
		}, wantRC: 2, wantOut: "", wantErr: "判不了：--only 指的不是真仓里的件 —— no/such.txt\n"},
		{name: "list", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--list"}
		}, wantRC: 0, wantOut: "== 预演（--list · 零写盘）==\n· 真仓：<FX>/real\n· 生效过滤器（现读 · include 在前 exclude 收尾）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n· 会取件数：5 · 字节：<B>（<B> B）\n    top.txt\n    core/cmd/zerg/testdata/cli-matrix.json\n    scripts/dev/keep.sh\n    sub/aaa.txt\n    sub/link_out.txt\n（预演结束 —— 未写任何件；去掉 --list 才真拷贝）\n", wantErr: ""},
		{name: "list_all", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--list", "--all"}
		}, wantRC: 0, wantOut: "== 预演（--list · 零写盘）==\n· 真仓：<FX>/real\n· 生效过滤器（现读 · include 在前 exclude 收尾）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n· 会取件数：5 · 字节：<B>（<B> B）\n    top.txt\n    core/cmd/zerg/testdata/cli-matrix.json\n    scripts/dev/keep.sh\n    sub/aaa.txt\n    sub/link_out.txt\n（预演结束 —— 未写任何件；去掉 --list 才真拷贝）\n", wantErr: ""},
		{name: "dst_inside", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/real/inside"}
		}, wantRC: 2, wantOut: "", wantErr: "拒绝：--dst 落在真仓内（<FX>/real/inside ⊂ <FX>/real）—— 副本必须建在真仓之外\n"},
		{name: "dst_eq_form", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst=" + d + "/tree"}
		}, wantRC: 0, wantOut: "== 只读隔离（硬拷贝）==\n· 真仓：<FX>/real\n· 副本：<FX>/tree\n· 生效排除（rsync 过滤器 · 现读实际生效）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n  · 链接→硬拷贝：sub/link_out.txt ⇐ <FX>/real/core/cmd/zerg/testdata/cli-matrix.json\n== 读数 ==\n· 真仓路径：<FX>/real\n· 副本路径：<FX>/tree\n· 件数：5 · 总字节：<B>（<B> B）\n· 链接→硬拷贝：1 条 · ISOLATED.md：<FX>/tree/ISOLATED.md\n· 验回：bash scripts/dev/isolate-ro.sh verify \"<FX>/tree\" \"<FX>/real\"\n· 收尾（本件**不会**替你删）：rm -rf \"<FX>/tree\"\n", wantErr: ""},
		{name: "reject_symlinks", seed: isoT_seed, argv: func(d string) []string {
			return []string{"new", d + "/real", "--dst", d + "/tree", "--reject-symlinks"}
		}, wantRC: 1, wantOut: "== 只读隔离（硬拷贝）==\n· 真仓：<FX>/real\n· 副本：<FX>/tree\n· 生效排除（rsync 过滤器 · 现读实际生效）：\n    --exclude=/.git\n    --exclude=/.zerg\n    --exclude=/bin\n    --exclude=/data\n    --exclude=/dist\n    --exclude=/node_modules\n    --exclude=/vendor\n    --exclude=/zerg-wt\n    --exclude=/__pycache__\n    --exclude=/.venv\n    --exclude=/venv\n    --exclude=/htmlcov\n    --exclude=/.cargo\n    --exclude=/target\n    --exclude=/ui/target\n    --exclude=/wall/target\n    --exclude=/.build\n    --exclude=/_history\n", wantErr: "  ✗ 拒绝：副本里有指向副本之外的符号链接 —— sub/link_out.txt → <FX>/real/core/cmd/zerg/testdata/cli-matrix.json\n副本里有 1 条**危险符号链接**（指向副本之外/悬空）—— 见上；不许在这种树上做删除动作\n"},
		{name: "verify_ok", seed: verifyTree, argv: func(d string) []string {
			return []string{"verify", d + "/tree", d + "/real"}
		}, wantRC: 0, wantOut: "== 逐件 sha256 对拍（副本 ⟷ 真仓）==\n· 副本：<FX>/tree\n· 真仓：<FX>/real\n· 副本侧口径：现算副本侧 sha256\n· 对拍件数：6 · 一致：6 · 不一致：0 · 真仓侧缺件：0\n⇒ rc=0：副本逐件与真仓一致 ⇒ **这一轮验证一个字节都没改到真仓**\n", wantErr: ""},
		{name: "verify_diff", seed: func(t *testing.T, root string) {
			verifyTree(t, root)
			isoT_w(t, root+"/real/top.txt", "CHANGED-AFTER\n")
		}, argv: func(d string) []string {
			return []string{"verify", d + "/tree", d + "/real"}
		}, wantRC: 1, wantOut: "== 逐件 sha256 对拍（副本 ⟷ 真仓）==\n· 副本：<FX>/tree\n· 真仓：<FX>/real\n· 副本侧口径：现算副本侧 sha256\n· 对拍件数：6 · 一致：5 · 不一致：1 · 真仓侧缺件：0\n  ✗ sha256 不一致: top.txt\n", wantErr: "真仓与副本对不上 ⇒ 要么真仓被改过、要么副本被改过 —— 先查清再谈结论\n"},
		{name: "self_test", argv: func(d string) []string { return []string{"--self-test"} }, wantRC: 0, wantOut: "── isolate-ro.sh 自检（成对控 · 合成夹具：<TMP>isolate-ro-selftest.<RND>/fix）──\n   ✓ 正控：new ⇒ rc=0\n   ✓ 正控：副本里有 ISOLATED.md\n   ✓ 正控：ISOLATED.md 写着真仓路径\n   ✓ 正控：件数/总字节都打印了（3 件 − 被排除的 bin/ 1 件 = 2\n   ✓ 正控：缺省排除生效（bin/ 没进副本）\n   ✓ 正控：硬拷贝是真件（同一字节）\n   ✓ 正控：verify ⇒ rc=0（真仓未被改）\n   ✓ 正控：真仓那件 sha256 未变（删的是副本）\n   ✓ 无清单档：new --no-manifest ⇒ rc=0\n   ✓ 无清单档：副本里没有 ISOLATED.manifest.tsv\n   ✓ 无清单档：verify ⇒ rc=0（现算副本侧 sha · 不假红）\n   ✓ 无清单档：对拍读数「不一致 == 0」\n   ✓ 负控 A：真仓被改 ⇒ verify rc=1\n   ✓ 负控 A：点名那一件\n   ✓ 负控 B：缺省档 rc=0 且把链接换成硬拷贝（副本里 0 条链接）\n   ✓ 负控 B：链接目标内容是真件（可读、对得上）\n   ✓ 负控 B'：--reject-symlinks ⇒ rc=1 且点出链接\n   ✓ 用法错：new 缺参数 ⇒ rc=2\n   ✓ 用法错：真仓不存在 ⇒ rc=2\n   ✓ 用法错：verify 缺一个实参 ⇒ rc=2\n   ✓ 用法错：未知子命令 ⇒ rc=2\n   ✓ 等号写法：--exclude=bin 被接受（旧版报「未知旗标」rc=2）\n   ✓ 生效排除清单可见（脚本开头逐行印 --exclude=…）\n   ✓ 缺省排除含 ui/target（29G 不再被整棵拷进来）\n   ✓ 缺省排除含 wall/target\n   ✓ 包含性：--dst 落在真仓内 ⇒ rc=2（副本不许写进真仓 · GAP-20260927-93）\n   ✓ 包含性：点名「真仓内」\n   ✓ 包含性：拒写后真仓里没被建出 inside/（未产生副作用）\n   ✓ --only 单件：rc=0\n   ✓ --only 单件：副本里只有那件 + 清单\n   ✓ --only 单件：件数 1\n   ✓ --only 指非件 ⇒ rc=2\n   ✓ --list：rc=0\n   ✓ --list：印「预演」且件数=2（现算：cli-matrix.json + keep.sh）\n   ✓ --list：零写盘（真仓件数跑前跑后不变）\n── 自检结论：全过 ──\n", wantErr: ""},
	}
}

// isoT_runGolden —— 逐格现建夹具、现跑 Go 半、归一后与冻值逐字比；返回失配格数。
// plant 非 nil 时在跑前往夹具落「旧态」sh 件（两态用）。
func isoT_runGolden(t *testing.T, tag string, plant func(fx string)) int {
	t.Helper()
	mism := 0
	for _, s := range isoTGoldenScens(t) {
		fx := t.TempDir()
		if plant != nil {
			plant(fx)
		}
		if s.seed != nil {
			s.seed(t, fx)
		}
		var ob, eb bytes.Buffer
		rc := isolateRoGateMain(&ob, &eb, s.argv(fx))
		phys, _ := filepath.EvalSymlinks(fx)
		gotOut := isoTNormGolden(ob.String(), phys, fx)
		gotErr := isoTNormGolden(eb.String(), phys, fx)
		if rc != s.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, s.name, rc, s.wantRC)
		}
		if gotOut != s.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, s.name, gotOut, s.wantOut)
		}
		if gotErr != s.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, s.name, gotErr, s.wantErr)
		}
	}
	return mism
}

// TestIsolateRoGoldenInternalized —— A 态（真仓 · 退役 sh 不在盘）金标自证（✗ 不 Skip · 逐格真断言 ·
// ✗ 不放宽为「只判非空」）。
func TestIsolateRoGoldenInternalized(t *testing.T) {
	repo := isoT_repoRoot(t)
	isoT_assertGoHalf(t, repo)
	if sh := filepath.Join(repo, "scripts", "dev", "isolate-ro.sh"); isoIsFile(sh) {
		t.Fatalf("A 态前提破：退役 sh 竟在盘 %s（本件前提 = 内化后无 sh）", sh)
	}
	if n := isoT_runGolden(t, "A真仓无sh", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestIsolateRoGoldenTwoStates —— 两态逐字同：A 态 = 夹具无 sh（真仓现态）；B 态 = 合成「旧态」
// 夹具（把退役 sh 路径 `scripts/dev/isolate-ro.sh` 放回）⇒ 两态对同一套金标逐字同命中（✗ 不 Skip）。
func TestIsolateRoGoldenTwoStates(t *testing.T) {
	repo := isoT_repoRoot(t)
	isoT_assertGoHalf(t, repo)
	if n := isoT_runGolden(t, "A夹具无sh", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
	plant := func(fx string) {
		p := filepath.Join(fx, "scripts", "dev", "isolate-ro.sh")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n# stub：造「旧态」——退役 sh 在位（本件只证 sh 在/不在盘不改判据结论）\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if n := isoT_runGolden(t, "B旧态有sh", plant); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（sh 在盘不得改判据结论）", n)
	}
}
