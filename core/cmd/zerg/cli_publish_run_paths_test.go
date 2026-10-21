// cli_publish_run_paths_test.go —— `zerg publish run` 的**两路档**（缺口 `GAP-20260926-22`）回归牙。
//
// 本笔要钉死三件事：
//
//	① 默认档**必须**走真源 `publish/mirror-public.sh`（逐提交镜像器）—— 计划件里三处同源可查：
//	   `它会执` / `档位` / `走哪一路`；且**不许**出现「应急通道」字样；
//	② `--legacy` 走旧压平器 `scripts/build/publish-public.sh` —— 计划件点名「应急通道」＋ stderr 打弃用警告；
//	③ 两路的**缺省落点不同名**（`/tmp/zerg-mirror-…` vs `/tmp/zerg-public-…`）—— 印错落点 = 让人去错目录找件；
//	④ `--dry-run` **零副作用**：跑完仓根一个字节都没变（逐件 sha 对拍），且真跑档的 D3 三态门禁仍在。
//
// ★ 全程在**仓外合成仓**上跑（`ZERG_REPO`）：`publish/mirror-public-lib.py` 是夹具（吐一行机器面），
//
//	✗ 不碰真仓、✗ 不真发布、✗ 不推远端、✗ 不联网。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// pprFixture —— 造一棵最小仓：只放只读预演要的 `publish/mirror-public-lib.py`（夹具，吐机器面）。
// 返回仓根；调用方用 `runZergRepo` 把 `ZERG_REPO` 指过去。
func pprFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "publish"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "import sys\n" +
		"print('MIRROR_LIST_ONLY {\"will_send\":3,\"skipped\":5,\"skip_by_table\":{\"EXCLUDES\":4,\"DROP_EXACT\":1,\"DROP_PREFIX\":0,\"映射丢弃\":0,\"非 blob\":0},\"empty_pointers\":2,\"empty_by_table\":{},\"whitelisted\":7,\"tracked\":8,\"private_face_hits\":0,\"out_touched\":false,\"network\":false,\"rules_fingerprint\":\"fp-fixture\",\"list_only\":true}')\n" +
		"raise SystemExit(0)\n"
	if err := os.WriteFile(filepath.Join(root, "publish", "mirror-public-lib.py"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("没有 python3（真源只读预演要它）⇒ 跳过")
	}
	return root
}

// pprTreeSha —— 仓根的**逐件 sha 清单**（判「零副作用」用；只走文件、不跟符号链接）。
func pprTreeSha(t *testing.T, root string) string {
	t.Helper()
	lines := []string{}
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(b)
		rel, _ := filepath.Rel(root, p)
		lines = append(lines, rel+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// pprRun —— 进程内跑一条命令，仓根指到合成仓（`ZERG_REPO` 一枚旋钮；与 `cli_calib_eval_test.go`
// 的 `runZergRepo` 同形，但本件在 `package main` 内 —— 直接走 `RunForTest`，不另建第二套入口）。
// ★ `HOME` 也换到合成目录（放一份最小名册 `~/.zerg/contexts/default.yaml`）—— 真跑档要先过
// `P-066` 的「对象是某一台机」作用域判，名册必须可复现，✗ 不读本机真名册。
func pprRun(t *testing.T, root string, argv ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	t.Setenv("HOME", pprHome(t))
	var out, errb bytes.Buffer
	rc := RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// pprHome —— 合成 HOME（只放名册：一台 `Mr2109` ＋ `default_node: Mr2109`）。
func pprHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".zerg", "contexts"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "default_node: Mr2109\nnodes:\n  Mr2109:\n    host: 127.0.0.1\n    port: 8580\n"
	if err := os.WriteFile(filepath.Join(home, ".zerg", "contexts", "default.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

// ① 默认档 = 逐提交镜像器（三处同源 + 缺省落点名）。
func TestPublishRunDryRunDefaultPathIsMirror(t *testing.T) {
	root := pprFixture(t)
	before := pprTreeSha(t, root)
	rc, out, errs := pprRun(t, root, "publish", "run", "--dry-run")
	if rc != 0 {
		t.Fatalf("rc = %d，want 0（stderr=%s）", rc, errs)
	}
	for _, must := range []string{
		"它会执   : publish/mirror-public.sh",
		"档位     : 默认（逐提交镜像器 = 正式发布通道）",
		"它会动   : 在**本地** `/tmp/zerg-mirror-<时间戳>`",
		"走哪一路 : " + publishRunPathNotice,
		"会发     : 3 件 / 跳过 5 件（EXCLUDES 4 · DROP_EXACT 1 · DROP_PREFIX 0 · 映射丢弃 0 · 非 blob 0）/ 空指 2 条",
		"规则指纹 : fp-fixture",
	} {
		if !strings.Contains(out, must) {
			t.Fatalf("计划件缺 %q\n--- stdout ---\n%s", must, out)
		}
	}
	for _, banned := range []string{"应急通道", "/tmp/zerg-public-"} {
		if strings.Contains(out, banned) {
			t.Fatalf("默认档的计划件里出现了 %q（默认档不许走旧压平器）:\n%s", banned, out)
		}
	}
	// `走哪一路` 那条**本来就**会点名旧件（它讲的是两档的区别）⇒ 只判「执行面」那两行不指旧件。
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "  它会执") && strings.Contains(ln, "publish-public.sh") {
			t.Fatalf("默认档的「它会执」指向旧压平器：%q", ln)
		}
	}
	if after := pprTreeSha(t, root); after != before {
		t.Fatalf("`--dry-run` 改了仓根（零副作用破了）：\n前:\n%s\n后:\n%s", before, after)
	}
}

// ② `--legacy` = 应急通道（点名旧件 + 弃用警告 + 旧缺省落点）。
func TestPublishRunDryRunLegacyIsEmergencyChannel(t *testing.T) {
	root := pprFixture(t)
	before := pprTreeSha(t, root)
	rc, out, errs := pprRun(t, root, "publish", "run", "--legacy", "--dry-run")
	if rc != 0 {
		t.Fatalf("rc = %d，want 0（`--legacy` 该被认；stderr=%s）", rc, errs)
	}
	for _, must := range []string{
		"它会执   : scripts/build/publish-public.sh",
		"档位     : --legacy（**应急通道** · 已弃用压平器 · v2.6.0 计划删除）",
		"它会动   : 在**本地** `/tmp/zerg-public-<时间戳>`",
		"本档走压平器 ⇒ 下面几行是**镜像器口径**",
	} {
		if !strings.Contains(out, must) {
			t.Fatalf("应急通道计划件缺 %q\n--- stdout ---\n%s", must, out)
		}
	}
	if !strings.Contains(errs, "应急通道") || !strings.Contains(errs, "已弃用") {
		t.Fatalf("stderr 没打「应急通道 / 已弃用」警告：%s", errs)
	}
	if after := pprTreeSha(t, root); after != before {
		t.Fatalf("`--legacy --dry-run` 改了仓根（零副作用破了）")
	}
}

// ③ 两路缺省落点不同名（常量级：印错落点 = 让人去错目录找件）。
func TestPublishRunDefaultOutDiffersByPath(t *testing.T) {
	if publishRunDefaultOut == publishRunMirrorDefaultOut {
		t.Fatalf("两路缺省落点同名（%q）—— 必有一条是错的", publishRunDefaultOut)
	}
	if !strings.Contains(publishRunMirrorDefaultOut, "zerg-mirror-") {
		t.Fatalf("镜像器缺省落点名不带 `zerg-mirror-`：%q", publishRunMirrorDefaultOut)
	}
}

// ④ 真跑档的 D3 三态门禁仍在（缺 --confirm ⇒ 退 2，且不出计划件）。
func TestPublishRunRealRunStillRequiresConfirm(t *testing.T) {
	root := pprFixture(t)
	rc, out, errs := pprRun(t, root, "publish", "run", "--node", "Mr2109")
	if rc != 2 {
		t.Fatalf("rc = %d，want 2（D3 缺确认 ⇒ 不执行）；stderr=%s", rc, errs)
	}
	if out != "" {
		t.Fatalf("缺确认时 stdout 应零字节，实得：%q", out)
	}
	if !strings.Contains(errs, "缺确认") || !strings.Contains(errs, "--confirm=") {
		t.Fatalf("stderr 没点名缺确认：%s", errs)
	}
}

// ⑤ `--legacy` 只归 `publish run`（别族真给 ⇒ 退 2「未知旗标」—— 数据驱动收口的牙）。
func TestPublishRunLegacyFlagNotLeakedToOtherCommands(t *testing.T) {
	root := pprFixture(t)
	rc, out, errs := pprRun(t, root, "publish", "verify", "--legacy")
	if rc != 2 {
		t.Fatalf("rc = %d，want 2（`--legacy` 不归 publish verify）", rc)
	}
	if out != "" {
		t.Fatalf("未知旗标时 stdout 应零字节，实得：%q", out)
	}
	if !strings.Contains(errs, "--legacy") {
		t.Fatalf("stderr 没点名 `--legacy`：%s", errs)
	}
}

// ⑤ `--out` 给旗标不给值 ⇒ 退 2（既有守卫不许被本笔撞坏）。
func TestPublishRunOutFlagWithoutValueRejected(t *testing.T) {
	root := pprFixture(t)
	rc, _, errs := pprRun(t, root, "publish", "run", "--out", "--dry-run")
	if rc != 2 {
		t.Fatalf("rc = %d，want 2（`--out` 没值不许静默当没给）", rc)
	}
	if !strings.Contains(errs, "--out") {
		t.Fatalf("stderr 没点名 `--out`：%s", errs)
	}
}

// ⑥ 逐件对拍面能**点名差异文件**（两路产出的验收口 = `publish/parity-compare.py` 的现跑输出）。
// 造两棵树：只差一个文件的内容 ⇒ 必须 rc≠0 且把那个文件名打出来。
func TestPublishParityCompareNamesDifferingFile(t *testing.T) {
	for _, tool := range []string{"git", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("没有 %s ⇒ 跳过逐件对拍面", tool)
		}
	}
	repo := repoRoot()
	cmp := filepath.Join(repo, "publish", "parity-compare.py")
	if _, err := os.Stat(cmp); err != nil {
		t.Skipf("比较器不在盘（%v）⇒ 跳过", err)
	}
	oldDir, newDir := t.TempDir(), t.TempDir()
	for _, d := range []string{oldDir, newDir} {
		runGitTest(t, d, "init", "-q")
		runGitTest(t, d, "config", "user.email", "t@e")
		runGitTest(t, d, "config", "user.name", "t")
		if err := os.WriteFile(filepath.Join(d, "same.txt"), []byte("same\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "differs.txt"), []byte("old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(newDir, "differs.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{oldDir, newDir} {
		runGitTest(t, d, "add", "-A")
		runGitTest(t, d, "commit", "-qm", "fixture")
	}
	emptyContract := filepath.Join(t.TempDir(), "empty.tsv")
	if err := os.WriteFile(emptyContract, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", cmp, "--old", oldDir, "--new", newDir, "--contract", emptyContract)
	outB, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("两棵树有一个件内容不同，比较器却判绿（rc=0）—— 契约外差异没被判红\n%s", outB)
	}
	out := string(outB)
	if !strings.Contains(out, "differs.txt") {
		t.Fatalf("比较器没点名差异文件 `differs.txt`（「差异要能打出文件名」是本笔判据）:\n%s", out)
	}
	if strings.Contains(out, "same.txt") {
		t.Fatalf("相同的件 `same.txt` 被当成差异（噪声会淹掉真差异）:\n%s", out)
	}
	if !strings.Contains(out, "契约外差异") {
		t.Fatalf("输出没给「契约外差异」判词:\n%s", out)
	}
}

// ⑦ `publish verify` 的 4b 内嵌 python 必须**真能跑**（回归牙：`pathMirrorLibPy` 是 Go 常量名，
// 曾被人肉批量重命名误写进 python 正文 ⇒ 每次真跑都 `NameError`、4b 恒红）。
// 判据两半：① 正文里不再残留 Go 标识符；② 现跑该脚本（仓内真件在盘时）退 0。
func TestPublishVerifyEmbeddedPythonRuns(t *testing.T) {
	repo := repoRoot()
	if repo == "" {
		t.Skip("解析不到仓根 ⇒ 跳过")
	}
	if _, err := os.Stat(filepath.Join(repo, "scripts", "build", "publish-public.sh")); err != nil {
		t.Skipf("旧压平器不在盘（%v）⇒ 跳过", err)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("没有 python3 ⇒ 跳过")
	}
	script := strings.ReplaceAll(publishVerifyBytes, "@@MIRROR_LIB_PY@@", pathMirrorLibPy)
	if strings.Contains(script, "pathMirrorLibPy") {
		t.Fatalf("内嵌 python 正文里残留 Go 标识符 `pathMirrorLibPy`（= 真跑必 NameError）")
	}
	if !strings.Contains(script, pathMirrorLibPy) {
		t.Fatalf("内嵌 python 正文里没有真源路径 %q（占位符没被代进去）", pathMirrorLibPy)
	}
	cmd := exec.Command("python3", "-", repo)
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	txt := string(out)
	// ★ 本牙判的是「脚本**能跑**」（曾经 `NameError` 恒红），不是「内容面当前绿」——
	//   EXCLUDES 登记是否齐由 4b 那一项照实判（现读：缺 `scripts/gates/check-hardcoded-private-paths.py`
	//   ⇒ 4b 真红，属内容面欠账，与内嵌脚本的可运行性分家）。
	if strings.Contains(txt, "Traceback") || strings.Contains(txt, "NameError") {
		t.Fatalf("4b 脚本现跑崩了（%v）：\n%s", err, txt)
	}
	if !strings.Contains(txt, "排除项 ") || !strings.Contains(txt, "发布机制脚本登记齐全:") {
		t.Fatalf("4b 脚本没给两格读数（排除项条数 / 登记齐全判词）：\n%s", txt)
	}
	_ = err // 退码由内容面决定（0 齐 / 1 缺件），本牙不钉
}

// runGitTest —— 夹具用 git（只在这两棵临时仓里跑；✗ 不碰真仓）。
func runGitTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s 失败：%v\n%s", strings.Join(args, " "), err, out)
	}
}
