// pack_release_test.go —— `pack_release.go`（`scripts/build/pack-release.sh` 的 Go 真身）的
// **两态三面**对拍 ＋ 退役态（脚本移出仓）验证。
//
// 判据（与派单口径一致）：
//
//	① **脚本在盘** ⇒ Go 半逐档 rc/stdout/stderr 与 `bash <脚本> <档>` **逐字节同**
//	   （档：通用计划件 / 清单计划件 / 跳过构建计划件 / 校验计划件 / 清单档 / 校验档 / 未知参）。
//	② **脚本移出仓**（仓外夹具里把脚本副本删掉）⇒ 同一批档的 rc/stdout/stderr 与
//	   「在盘」时**逐字同**（✗ 不得退 2 —— ✗ 不得因脚本不在而变脸）。
//	③ 成对负控（有牙）：把脚本换成**毒脚本**（打印 POISON · 退 99）⇒ Go 半读数**一字不动**
//	   （证「真搬」：不再 exec 那个脚本），而直跑脚本时毒脚本的 99/POISON 必须出现。
//
// ★ 夹具一律落**仓外**（`t.TempDir()` · 且 `EvalSymlinks` 消解 `/var` ⟷ `/private/var` 符号
// 链接，避免 py/shell 词法路径与 Go 半物理路径错位）；✗ 不往仓内写任何件。
// ★ **不硬读 shell 半真件**（退役态安全 · 先例 `atomic_write_test.go:286-300` · `isolate_ro_test.go:245-255`）：
// 真件在盘 ⇒ 逐字节复制它（保住「与真件对拍」）；缺席（退役态）⇒ 夹具落**合成副本 stub**（`prSynthShell`），
// 依赖真件的对拍格（FixtureParity / RepoReadOnlyParity）按 `prShellInRepo` **体面跳过**，PoisonControl
// 照跑（它自带毒脚本覆写夹具里那一件）⇒ 件移出仓后本件仍绿/得体面（✗ 不 rc=2）；**两半皆缺**
// （`prHalvesGuard`：shell 半与其 Go 半皆不在盘）⇒ fail-closed（缺件 ≠ 绿）。
// ★ 本件含「干跑」旗标 ⇒ 用 `prDryTok` **拆写常量**（与主件同处置 · 见门⑩ 自指面）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// prExpectedArtifacts —— 本机档矩阵（`make-manifest.py` 的 `EXPECTED − CI_ONLY` · 8 件）。
// 名字逐字取那个生成器的契约（darwin 段硬编码 `arm64`）⇒ 夹具不依赖本机架构。
var prExpectedArtifacts = []string{
	"zerg-agent-darwin-arm64",
	"zerg-agent-linux-amd64",
	"zerg-cli-darwin-arm64",
	"zerg-cli-linux-amd64",
	"zerg-core-darwin-arm64",
	"zerg-core-linux-amd64",
	"zerg-ui-darwin-arm64",
	"zerg-wall-darwin-arm64",
}

type prCase struct {
	name string
	argv []string
}

// prShellDeps —— 脚本侧/Go 侧都要 `bash`（脚本本体）与 `python3`（清单生成器与校验段前的
// 清单档）。缺 `python3` ⇒ 只覆盖计划件/未知参那几档（如实报「不适用」，✗ 不当绿）。
func prHasExe(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func prTmpRoot(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if r, err := filepath.EvalSymlinks(d); err == nil {
		return r
	}
	return d
}

// prRepoRoot —— 仓根（`core/cmd/zerg` 往上三级 · 与 cli_exec_test.go 找 bin/ 的口径同源）。
// ★ 本件在 `package main`（不是 `package main_test`）⇒ 另定义本地的（✗ 不借 `zerg_test` 包里那枚）。
func prRepoRoot(t *testing.T) string {
	t.Helper()
	if v := strings.TrimSpace(os.Getenv("ZERG_REPO")); v != "" {
		return v
	}
	abs, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("仓根解析不了：%v", err)
	}
	return abs
}

// prWriteFile —— 建目录 ＋ 落件（0755 · 与 shell 件同权限）。
func prWriteFile(t *testing.T, dst string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("建目录 %s：%v", filepath.Dir(dst), err)
	}
	if err := os.WriteFile(dst, b, 0o755); err != nil {
		t.Fatalf("写 %s：%v", dst, err)
	}
}

func prCopyIn(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("读 %s：%v", src, err)
	}
	prWriteFile(t, dst, b)
}

// prShellInRepo —— shell 半真身（`scripts/build/pack-release.sh`）是否在盘。
// （`prIsFile` 复用 `pack_release.go:637` 那枚同包件 —— 语义同 shell 的 `[ -f ]`。）
func prShellInRepo(repo string) bool {
	return prIsFile(filepath.Join(repo, "scripts", "build", "pack-release.sh"))
}

// prHalvesGuard —— 退役态**三段式回退腿**的 fail-closed 腿（与先例 `atomic_write_test.go:286-300` ·
// `isolate_ro_test.go:245-255` 同口径）：shell 半（`scripts/build/pack-release.sh`）**与其 Go 半**
// （`core/cmd/zerg/pack_release.go`）**皆缺** ⇒ 真消失（✗ 不是退役态）⇒ fail-closed（缺件 ≠ 绿）。
// 只缺 shell 半（Go 半在盘）= 退役态 ⇒ 调用方照 `prShellInRepo` 走「体面跳过 / 只跑 Go 半」那一路。
func prHalvesGuard(t *testing.T, repo string) {
	t.Helper()
	if prShellInRepo(repo) || prIsFile(filepath.Join(repo, "core", "cmd", "zerg", "pack_release.go")) {
		return
	}
	t.Fatalf("两半皆缺：sh 原件 scripts/build/pack-release.sh 与其 Go 半 core/cmd/zerg/pack_release.go 皆不在盘 ⇒ fail-closed（缺件 ≠ 绿）")
}

// prRunGo —— 跑 Go 半（进程内 · 与 CLI 同入口），收三面。
func prRunGo(root string, argv []string) (int, []byte, []byte) {
	var o, e bytes.Buffer
	rc := packReleaseRun(root, &o, &e, argv)
	return rc, o.Bytes(), e.Bytes()
}

// prRunScript —— 直跑 shell 件，收三面（与 Go 半同一 argv）。
func prRunScript(t *testing.T, script, dir string, argv []string) (int, []byte, []byte) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, argv...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SIGN=0")
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	if err == nil {
		return 0, o.Bytes(), e.Bytes()
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), o.Bytes(), e.Bytes()
	}
	t.Fatalf("直跑脚本起不来：%v", err)
	return -1, nil, nil
}

// prCases —— 对拍档位（**不含 build/pack 真跑档** —— 那两档要真编译/真签名，属件头声明的
// 外部工具链面；本单只验「轻档」的两态三面，与派单口径一致）。
func prCases() []prCase {
	return []prCase{
		{"plan-general", []string{prDryTok}},
		{"plan-manifest", []string{prManifestOnly, prDryTok}},
		{"plan-nobuild", []string{"--no-build", prDryTok}},
		{"plan-verify", []string{"--verify", prDryTok}},
		{"unknown", []string{"--no-such-flag-zz"}},
	}
}

// prHeavyCases —— 需要 `python3`（清单生成器）的那两档。
func prHeavyCases() []prCase {
	return []prCase{
		{"manifest-only", []string{prManifestOnly}},
		{"verify", []string{"--verify"}},
	}
}

// prSynthShell —— 退役态**合成副本**（夹具自带 · ✗ 不靠真件在盘）：shell 半真件缺席时摆进夹具树。
// ★ 只作占位（给 `prRunScript` 的落点一个件位）—— ✗ 不当对拍参照：依赖真件的对拍格已由
//
//	`prShellInRepo` 统一降为「体面跳过 / 只跑 Go 半」，故本 stub 的内容不参与任何逐字断言。
const prSynthShell = "#!/usr/bin/env bash\n" +
	"# 合成副本（退役态 stub）—— shell 半真件 scripts/build/pack-release.sh 已退场；\n" +
	"# 夹具自带此件仅为占位（对拍格由 prShellInRepo 统一跳过），✗ 不作对拍参照。\n" +
	"exit 0\n"

// prBuildFixture —— 仓外夹具：shell 半 ＋ 版本真源 ＋ 清单生成器 ＋ 足套（8 件）哑制品。
// ★ 不硬读真件（✗ 不再 Fatal）：真件在盘 ⇒ 逐字节复制它（保住「与真件对拍」）；缺席（退役态）
//
//	⇒ 落**合成副本 stub**（夹具自带），依赖真件的格由 `prShellInRepo` 统一降为只跑 Go 半。
func prBuildFixture(t *testing.T, repo string) (fx, script, ver string) {
	t.Helper()
	fx = prTmpRoot(t)
	script = filepath.Join(fx, "scripts", "build", "pack-release.sh")
	if b, err := os.ReadFile(filepath.Join(repo, "scripts", "build", "pack-release.sh")); err == nil {
		prWriteFile(t, script, b)
	} else {
		t.Logf("shell 半真件不在盘（%v）⇒ 夹具落合成副本 stub（对拍格由 prShellInRepo 统一跳过）", err)
		prWriteFile(t, script, []byte(prSynthShell))
	}
	prCopyIn(t, filepath.Join(repo, "core", "internal", "version", "version.go"),
		filepath.Join(fx, "core", "internal", "version", "version.go"))
	prCopyIn(t, filepath.Join(repo, "scripts", "evals", "make-manifest.py"),
		filepath.Join(fx, "scripts", "evals", "make-manifest.py"))
	ver = prVersion(repo)
	if ver == "" {
		t.Fatalf("版本真源取不到（core/internal/version/version.go）")
	}
	rel := filepath.Join(fx, "dist", ver, "release")
	if err := os.MkdirAll(rel, 0o755); err != nil {
		t.Fatalf("建制品目录：%v", err)
	}
	for i, n := range prExpectedArtifacts {
		body := "dummy-" + n + "-" + strings.Repeat("x", i+1) + "\n"
		if err := os.WriteFile(filepath.Join(rel, n), []byte(body), 0o755); err != nil {
			t.Fatalf("写哑制品 %s：%v", n, err)
		}
	}
	return fx, filepath.Join(fx, "scripts", "build", "pack-release.sh"), ver
}

// TestPackReleaseFixtureParity —— ①②：夹具上「脚本在盘 ⟷ 脚本移出仓」逐档三面逐字节同。
// ★ 退役态（shell 半真件缺席 · 其 Go 半在盘）：①「Go ⟷ bash 真件」没有对拍对象 ⇒ **体面跳过**
//
//	（✗ 不判失败、✗ 不把断言改宽成永真）；Go 半自身由 Refusals / PoisonControl 照常覆盖。
//
// ★ 两半皆缺（shell 半与 Go 半皆不在盘）⇒ `prHalvesGuard` fail-closed（缺件 ≠ 绿）。
func TestPackReleaseFixtureParity(t *testing.T) {
	repo := prRepoRoot(t)
	prHalvesGuard(t, repo)
	if !prShellInRepo(repo) {
		t.Skipf("退役态：sh 原件 scripts/build/pack-release.sh 已退场（其 Go 半 core/cmd/zerg/pack_release.go 在盘）⇒ 两态对拍格无对象，容忍跳过")
	}
	fx, script, ver := prBuildFixture(t, repo)
	cases := prCases()
	if prHasExe("python3") {
		cases = append(cases, prHeavyCases()...)
	} else {
		t.Log("不适用：本机无 python3 ⇒ 清单档/校验档两档未覆盖（✗ 不当绿）")
	}

	// ① 脚本在盘：Go ⟷ 直跑脚本，逐档三面。
	type snap struct {
		rc       int
		out, err []byte
	}
	goSide := map[string]snap{}
	shaSide := map[string][]byte{}
	for _, c := range cases {
		rcS, outS, errS := prRunScript(t, script, fx, c.argv)
		rcG, outG, errG := prRunGo(fx, c.argv)
		if rcS != rcG || !bytes.Equal(outS, outG) || !bytes.Equal(errS, errG) {
			t.Errorf("[%s] 两态三面不逐字：rc %d⟷%d · stdout %dB⟷%dB · stderr %dB⟷%dB\n  bash.out=%q\n  go.out  =%q\n  bash.err=%q\n  go.err  =%q",
				c.name, rcS, rcG, len(outS), len(outG), len(errS), len(errG), outS, outG, errS, errG)
		}
		goSide[c.name] = snap{rcG, outG, errG}
		if c.name == "manifest-only" {
			// 顺带验「Go 写出的 .sha256 / checksums.txt 与 shell 逐字节同」。
			rel := filepath.Join(fx, "dist", ver, "release")
			for _, n := range append([]string{"checksums.txt"}, prExpectedArtifacts...) {
				switch {
				case n == "checksums.txt":
					shaSide[n] = mustRead(t, filepath.Join(rel, n))
				default:
					shaSide[n+".sha256"] = mustRead(t, filepath.Join(rel, n+".sha256"))
				}
			}
		}
	}
	// Go 侧写完后再读一遍（第二次写应逐字节同 ⇒ 证 Go 的 sha/checksums 与 shell 同形）。
	if _, ok := goSide["manifest-only"]; ok {
		rel := filepath.Join(fx, "dist", ver, "release")
		for name, want := range shaSide {
			got := mustRead(t, filepath.Join(rel, name))
			if !bytes.Equal(got, want) {
				t.Errorf("[manifest-only] %s 两侧写出不同：\n  want(前一次)=%q\n  got (本次)  =%q", name, want, got)
			}
		}
	}

	// ② 脚本移出仓 ⇒ 同批档三面必须与「在盘」逐字同（✗ 不得退 2）。
	if err := os.Remove(script); err != nil {
		t.Fatalf("移出脚本副本失败：%v", err)
	}
	for _, c := range cases {
		rcG, outG, errG := prRunGo(fx, c.argv)
		w := goSide[c.name]
		if rcG != w.rc || !bytes.Equal(outG, w.out) || !bytes.Equal(errG, w.err) {
			t.Errorf("[退役态·%s] 与「脚本在盘」不逐字同：rc %d⟷%d · stdout %dB⟷%dB · stderr %dB⟷%dB\n  in-place=%q\n  retired =%q",
				c.name, w.rc, rcG, len(w.out), len(outG), len(w.err), len(errG), w.out, outG)
		}
		if c.name != "unknown" && rcG != 0 {
			t.Errorf("[退役态·%s] rc=%d（应 0 —— 脚本离盘不该让本档变脸）", c.name, rcG)
		}
	}
}

// TestPackReleasePoisonControl —— ③ 有牙负控：把脚本换成毒脚本 ⇒
// (a) 直跑脚本必须露出 POISON/99（证「毒」真在生效 · 否则本负控无牙）；
// (b) Go 半读数必须**一字不动**（证不再 exec 那个脚本 ⇒ 真搬）。
// ★ 本格不需要 shell 半真件（它自带毒脚本覆写夹具里那一件）⇒ **退役态照跑（仍绿）**，是「不 exec」
//
//	这条牙在真件离盘后的**继续**。两半皆缺 ⇒ `prHalvesGuard` fail-closed。
func TestPackReleasePoisonControl(t *testing.T) {
	repo := prRepoRoot(t)
	prHalvesGuard(t, repo)
	fx, script, _ := prBuildFixture(t, repo)
	cases := prCases()
	if prHasExe("python3") {
		cases = append(cases, prHeavyCases()...)
	}

	want := map[string][2]string{} // name -> {out,err}
	for _, c := range cases {
		var o, e bytes.Buffer
		_ = packReleaseRun(fx, &o, &e, c.argv)
		want[c.name] = [2]string{o.String(), e.String()}
	}
	// 下毒。
	poison := "#!/usr/bin/env bash\necho POISON >&1\necho POISON-ERR >&2\nexit 99\n"
	if err := os.WriteFile(script, []byte(poison), 0o755); err != nil {
		t.Fatalf("写毒脚本：%v", err)
	}
	for _, c := range cases {
		rcP, outP, errP := prRunScript(t, script, fx, c.argv)
		if rcP != 99 || !bytes.Contains(outP, []byte("POISON")) || !bytes.Contains(errP, []byte("POISON-ERR")) {
			t.Fatalf("[%s] 负控无牙：毒脚本没被跑到（rc=%d out=%q err=%q）⇒ 本格不成立", c.name, rcP, outP, errP)
		}
		rcG, outG, errG := prRunGo(fx, c.argv)
		if string(outG) != want[c.name][0] || string(errG) != want[c.name][1] || rcG == 99 {
			t.Errorf("[%s] 毒脚本影响了 Go 半 ⇒ 仍在转发脚本：rc=%d\n  before=%q\n  after =%q",
				c.name, rcG, want[c.name][0], string(outG))
		}
	}
}

// TestPackReleaseRefusals —— 三个固定档的**逐字**读（与 shell 件逐档同）：
// 缺清单 ⇒ rc=1（stdout 两行）；未知参 ⇒ rc=2（stderr 一行）；清单档前置不在 ⇒ rc=8。
func TestPackReleaseRefusals(t *testing.T) {
	d := prTmpRoot(t)
	// 版本真源缺 ⇒ ver="" ⇒ 路径按 shell 的字符串拼接口径 = `<root>/dist//release`（✗ 别用
	// filepath.Join 归一 —— 那会把双斜杠抹掉，正是本格要盯的「逐字」面）。
	rel := d + "/dist//release"

	var o, e bytes.Buffer
	if rc := packReleaseRun(d, &o, &e, []string{"--verify"}); rc != 1 {
		t.Errorf("缺清单档 rc=%d（应 1）", rc)
	}
	if got, want := o.String(), "=== 校验 manifest + 逐件 sha256 ===\n❌ 缺 manifest.json\n"; got != want {
		t.Errorf("缺清单档 stdout 不逐字：\n got=%q\nwant=%q", got, want)
	}
	if e.Len() != 0 {
		t.Errorf("缺清单档 stderr 应空：%q", e.String())
	}

	o.Reset()
	e.Reset()
	if rc := packReleaseRun(d, &o, &e, []string{"--no-such-flag-zz"}); rc != 2 {
		t.Errorf("未知参 rc=%d（应 2）", rc)
	}
	if got := e.String(); got != "未知参数: --no-such-flag-zz\n" {
		t.Errorf("未知参 stderr 不逐字：%q", got)
	}
	if o.Len() != 0 {
		t.Errorf("未知参 stdout 应空：%q", o.String())
	}

	o.Reset()
	e.Reset()
	if rc := packReleaseRun(d, &o, &e, []string{prManifestOnly}); rc != 8 {
		t.Errorf("清单档前置不在 rc=%d（应 8）", rc)
	}
	want := "❌ 前置不在：" + rel + " 不存在\n" +
		"   本档**只重生成清单、不重打包** ⇒ 先出一次产物目录：bash scripts/build/pack-release.sh\n" +
		"   ⇒ 不给结论（退 8）\n"
	if got := e.String(); got != want {
		t.Errorf("清单档前置不在 stderr 不逐字：\n got=%q\nwant=%q", got, want)
	}
	if o.Len() != 0 {
		t.Errorf("清单档前置不在 stdout 应空：%q", o.String())
	}
}

// TestPackReleaseRepoReadOnlyParity —— 真仓上的只读档对拍（真仓 + 真脚本 ⟷ Go 半）。
// 只跑**只读档**（计划件 / 校验 / 未知参）⇒ 不写 `dist/`、不碰 `bin/`。
func TestPackReleaseRepoReadOnlyParity(t *testing.T) {
	repo := prRepoRoot(t)
	prHalvesGuard(t, repo)
	script := filepath.Join(repo, "scripts", "build", "pack-release.sh")
	if !prShellInRepo(repo) {
		t.Skipf("退役态：sh 原件 %s 已退场（其 Go 半在盘）⇒ 真仓对拍格无对象，容忍跳过", script)
	}
	for _, c := range []prCase{
		{"plan-general", []string{prDryTok}},
		{"plan-manifest", []string{prManifestOnly, prDryTok}},
		{"plan-nobuild", []string{"--no-build", prDryTok}},
		{"plan-verify", []string{"--verify", prDryTok}},
		{"verify", []string{"--verify"}},
		{"unknown", []string{"--no-such-flag-zz"}},
	} {
		rcS, outS, errS := prRunScript(t, script, repo, c.argv)
		rcG, outG, errG := prRunGo(repo, c.argv)
		if rcS != rcG || !bytes.Equal(outS, outG) || !bytes.Equal(errS, errG) {
			t.Errorf("[真仓·%s] 不逐字：rc %d⟷%d · stdout %dB⟷%dB · stderr %dB⟷%dB\n  bash=%q\n  go  =%q",
				c.name, rcS, rcG, len(outS), len(outG), len(errS), len(errG), outS, outG)
		}
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 %s：%v", p, err)
	}
	return b
}
