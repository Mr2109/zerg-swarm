// pack_release_test.go —— `pack_release.go`（`scripts/build/pack-release.sh` 的 Go 真身）的
// **金标内化**回归 ＋ 退役态（脚本移出仓）验证。
//
// ★ 内化缘起（DEV-2143 / DEV-2145 / DEV-2146 同族 · 甲类「两件三处」之一）：本件原「sh 真身 ⟷
// Go 真搬」逐字对拍（旧名 TestPackReleaseFixtureParity / TestPackReleaseRepoReadOnlyParity）的
// 对拍源 `scripts/build/pack-release.sh` 已退场（M2 批）⇒ 对拍源消失、两格恒 Skip（假绿、零保护）。
// 照 `atomic_write_test.go`（体例最新）＋`check_verify_cmd_real_test.go` 把金标真源**收回 Go 半
// 现役行为本身**。
//
// 判据：
//
//	① 金标内化（A 态 · 真仓无 sh）：逐档 (rc, stdout, stderr) 与现算现冻的**逐字**比；哨兵锚
//	   Go 半 `core/cmd/zerg/pack_release.go`（✗ 不锚 sh —— 已退场即失锚），找不到 ⇒ `t.Fatalf`
//	   fail-closed（✗ 不留 Skip）。
//	② 两态逐字同（B 态 · 合成「旧态」夹具把退役 sh 路径放回）：sh 在/不在盘都不改判据结论。
//	③ 真仓只读档金标（plan-* 干跑 / 未知参）：不写 `dist/`、不碰 `bin/`。
//	④ 成对负控「有牙」：毒脚本不影响 Go 半（证真搬 · `TestPackReleasePoisonControl`）；
//	   哑化被测判据一臂（`-overlay`）⇒ 金标用例必 FAIL 且带行号（见交付读数）。
//
// ★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：逐格「夹具/真仓 + argv ⇒
// (rc, stdout, stderr)」由 `prRunGo` 现跑捕获、归一后冻结为常量；**本机时点/位置类量**在比对前
// 归一化（✗ 不冻结 —— 因其本非常量）：
//
//	① 夹具/真仓根（`t.TempDir()` 物理路径 + macOS `/var`⟷`/private/var` 符号链接）⇒ `<FX>` / `<REPO>`；
//	② 版本号（`core/internal/version/version.go` 的 `const Version`，随收版漂）⇒ `<VER>`；
//	③ `$TMPDIR` ⇒ `<TMP>`。
//
// ★ `manifest-only` 档委派外部 `python3 scripts/evals/make-manifest.py`（件头声明的**外部工具链面**）
// ⇒ 本机无 `python3` 时该格如实 log「不适用」（✗ 不当绿 · ✗ 不 Skip 整测试）。
// ★ **未覆盖面**（✗ 不声称逐字对齐）：build/pack 真跑档（真编译 / 真签名）不走本件（外部工具链面）。
//
// ★ 夹具一律落**仓外**（`t.TempDir()` · 且 `EvalSymlinks` 消解 `/var`⟷`/private/var` 符号链接，
// 避免 sh 词法路径与 Go 半物理路径错位）；✗ 不往仓内写任何件。
// ★ 本件含「干跑」旗标 ⇒ 用 `prDryTok` **拆写常量**（与主件同处置 · 见门⑩ 自指面；金标里的该旗标
// 亦逐处拆写拼接）。
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

// prHasExe —— 本机是否有某可执行件（`python3` 缺失时重档如实「不适用」）。
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

// prGoHalfRel —— 哨兵锚：Go 半（✗ 不锚已退场的 sh —— 一退役即失锚）。
const prGoHalfRel = "core/cmd/zerg/pack_release.go"

// prShRel —— 已退场 shell 半的仓根相对路径；「两态」就钉在它「在 / 不在盘」。
const prShRel = "scripts/build/pack-release.sh"

// prShellInRepo —— shell 半真身是否在盘（仅用于 `prHalvesGuard` 的 fail-closed 腿）。
func prShellInRepo(repo string) bool {
	return prIsFile(filepath.Join(repo, "scripts", "build", "pack-release.sh"))
}

// prAnchorGoHalf —— 哨兵：锚 **Go 半**在盘（✗ 不再锚已退场 sh）；找不到 ⇒ fail-closed（缺件 ≠ 绿 ·
// ✗ 不留 Skip）。旧版对上溯不到 sh 就 `t.Skipf`（恒 Skip · 零保护）—— 本改把它换成硬前置。
func prAnchorGoHalf(t *testing.T, repo string) {
	t.Helper()
	if !prIsFile(filepath.Join(repo, filepath.FromSlash(prGoHalfRel))) {
		t.Fatalf("哨兵失守：上溯未见 Go 半 %s（不是真仓树 / Go 半缺失）⇒ fail-closed", prGoHalfRel)
	}
}

// prHalvesGuard —— 退役态 fail-closed 腿：shell 半与其 Go 半**皆缺** ⇒ 真消失（✗ 不是退役态）⇒
// fail-closed（缺件 ≠ 绿）。只缺 shell 半（Go 半在盘）= 退役态 ⇒ 照常跑金标。
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

// prRunScript —— 直跑 shell 件，收三面（仅供毒脚本负控直跑那一格）。
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

// prCases —— 轻档（**不含 build/pack 真跑档** —— 那两档要真编译/真签名，属件头声明的外部工具链面）。
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

// prBuildFixture —— 仓外夹具：版本真源 ＋ 清单生成器 ＋ 足套（8 件）哑制品。
// ★ ✗ 不落 sh 件（Go 半从不读它）；`scripts/build/` 目录留位供毒脚本负控覆写。
func prBuildFixture(t *testing.T, repo string) (fx, script, ver string) {
	t.Helper()
	fx = prTmpRoot(t)
	script = filepath.Join(fx, "scripts", "build", "pack-release.sh")
	if err := os.MkdirAll(filepath.Join(fx, "scripts", "build"), 0o755); err != nil {
		t.Fatalf("建 scripts/build：%v", err)
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
	return fx, script, ver
}

// ── 内化金标：旧 sh 对拍的逐格金标收成 Go 常量（现算现冻 · ✗ 不臆造）──────────────────────

// prNormGolden —— 归一本机时点/位置类量（夹具/真仓根 · 版本 · $TMPDIR）⇒ 金标跨机可复现。
// ★ 替换次序：先具体根（物理路径 `phys`，再逻辑 `root`），后版本，末 $TMPDIR（✗ 不能先替 $TMPDIR
// —— 夹具根本身落在 $TMPDIR 之下，先替会把根也吞掉）。
func prNormGolden(s, root, phys, ph, ver string) string {
	if phys != "" && phys != root {
		s = strings.ReplaceAll(s, phys, ph)
	}
	if root != "" {
		s = strings.ReplaceAll(s, root, ph)
	}
	if ver != "" {
		s = strings.ReplaceAll(s, ver, "<VER>")
	}
	if td := os.TempDir(); td != "" {
		s = strings.ReplaceAll(s, td, "<TMP>")
	}
	return s
}

type prGolden struct {
	name    string
	wantRC  int
	wantOut string // 归一化后（prNormGolden 现算自证 · ✗ 手写）
	wantErr string
}

// prFixtureGolden —— 夹具档金标（7 格 · 逐格由 prRunGo 现跑捕获）。
var prFixtureGolden = []prGolden{
	{name: "plan-general", wantRC: 0, wantOut: "计划件（" + prDryTok + " · 零副作用 —— 未执行、未改任何状态）\n  动作     : pack-release.sh（档 = build）\n  会跑哪几步: build 档 = build-all.sh --dist → 交叉编 linux 三件 → rm -rf 重打包 → 编茧壁 → 写清单\n              pack 档 = 跳过构建，其余同上 · verify 档 = 只校验（只读）\n  会写哪些件: <FX>/dist/<VER>/（交叉编三件）· <FX>/dist/<VER>/release/（8 件 + 逐件 .sha256 + checksums.txt + manifest.json）· wall/target/（茧壁）\n  ⇒ 想走**不重打包**的那一档用：--manifest-only " + prDryTok + "\n", wantErr: ""},
	{name: "plan-manifest", wantRC: 0, wantOut: "计划件（--manifest-only " + prDryTok + " · 零副作用 —— 未执行、未改任何状态）\n  动作     : 重生成发布清单（只重算清单与 release 目录内逐件 sha256）\n  危险档   : D2（只写 <FX>/dist/<VER>/release 一个目录；不编译、不重打包、不碰 bin/ 里在跑的件）\n  会跑哪几步:\n    ① 前置检查：<FX>/dist/<VER>/release 存在且内有 zerg-* 制品（不在 ⇒ 退 8 · 不给结论）\n    ② 逐件重算 <FX>/dist/<VER>/release/<件>.sha256 与 <FX>/dist/<VER>/release/checksums.txt\n    ③ ZERG_MANIFEST_LOCAL=1 python3 scripts/evals/make-manifest.py <FX>/dist/<VER>/release <版本> <commit> <build_time>\n       （版本/commit/build_time 取**既有清单**，退而取 build-info.json —— 保「这批制品是谁产的」不变）\n    ④ 复跑校验段：manifest 逐件 sha256 / 体积必须与文件一致\n  会写哪些件（**全在 publish/ 外** · 只此三处）:\n    <FX>/dist/<VER>/release/manifest.json\n    <FX>/dist/<VER>/release/<逐件>.sha256\n    <FX>/dist/<VER>/release/checksums.txt\n  不会动   : bin/（在跑的件）· wall/target/ · 不重编译 · 不拷贝二进制 · 不重签 · 不重启\n  执行要   : --confirm=<主机名> 与 --yes 同时到（命令面：zerg build release --manifest-only --confirm=<主机名> --yes）\n", wantErr: "（--manifest-only " + prDryTok + "：只出计划件 · 零副作用 —— 未执行、未改任何状态）\n"},
	{name: "plan-nobuild", wantRC: 0, wantOut: "计划件（" + prDryTok + " · 零副作用 —— 未执行、未改任何状态）\n  动作     : pack-release.sh（档 = pack）\n  会跑哪几步: build 档 = build-all.sh --dist → 交叉编 linux 三件 → rm -rf 重打包 → 编茧壁 → 写清单\n              pack 档 = 跳过构建，其余同上 · verify 档 = 只校验（只读）\n  会写哪些件: <FX>/dist/<VER>/（交叉编三件）· <FX>/dist/<VER>/release/（8 件 + 逐件 .sha256 + checksums.txt + manifest.json）· wall/target/（茧壁）\n  ⇒ 想走**不重打包**的那一档用：--manifest-only " + prDryTok + "\n", wantErr: ""},
	{name: "plan-verify", wantRC: 0, wantOut: "计划件（" + prDryTok + " · 零副作用 —— 未执行、未改任何状态）\n  动作     : pack-release.sh（档 = verify）\n  会跑哪几步: build 档 = build-all.sh --dist → 交叉编 linux 三件 → rm -rf 重打包 → 编茧壁 → 写清单\n              pack 档 = 跳过构建，其余同上 · verify 档 = 只校验（只读）\n  会写哪些件: <FX>/dist/<VER>/（交叉编三件）· <FX>/dist/<VER>/release/（8 件 + 逐件 .sha256 + checksums.txt + manifest.json）· wall/target/（茧壁）\n  ⇒ 想走**不重打包**的那一档用：--manifest-only " + prDryTok + "\n", wantErr: ""},
	{name: "unknown", wantRC: 2, wantOut: "", wantErr: "未知参数: --no-such-flag-zz\n"},
	{name: "manifest-only", wantRC: 0, wantOut: "=== 只重生成清单（--manifest-only · 不编译、不重打包、不碰 bin/）===\n    <FX>/dist/<VER>/release · 制品 8 件 · 版本 <VER> · commit unknown · build_time unknown\n    写入面只有：<FX>/dist/<VER>/release/manifest.json · <FX>/dist/<VER>/release/<逐件>.sha256 · <FX>/dist/<VER>/release/checksums.txt\n   📝 manifest.json: 8 件（版本 <VER> · 代码 unknown · 档=本机（EXPECTED−CI_ONLY））\n=== 校验 manifest + 逐件 sha256 ===\n   版本 <VER> · 代码 unknown · 逐件 8\n     zerg-agent-darwin-arm64               32 字节  c8b706726098299b…\n     zerg-agent-linux-amd64                32 字节  a1ba97366af4c3e5…\n     zerg-cli-darwin-arm64                 32 字节  fda52c6f41768015…\n     zerg-cli-linux-amd64                  32 字节  3acffbb7dfd0adb5…\n     zerg-core-darwin-arm64                35 字节  35b16601337e7fb7…\n     zerg-core-linux-amd64                 35 字节  e2b36f0d1af7f130…\n     zerg-ui-darwin-arm64                  35 字节  697800a304eeec4e…\n     zerg-wall-darwin-arm64                38 字节  b94f5841542b1f21…\n✅ 校验通过（manifest 与文件一致）\n", wantErr: ""},
	{name: "verify", wantRC: 1, wantOut: "=== 校验 manifest + 逐件 sha256 ===\n❌ 缺 manifest.json\n", wantErr: ""},
}

// prRepoGolden —— 真仓只读档金标（5 格 · 只读 ⇒ 不写 dist/、不碰 bin/）。
var prRepoGolden = []prGolden{
	{name: "plan-general", wantRC: 0, wantOut: "计划件（" + prDryTok + " · 零副作用 —— 未执行、未改任何状态）\n  动作     : pack-release.sh（档 = build）\n  会跑哪几步: build 档 = build-all.sh --dist → 交叉编 linux 三件 → rm -rf 重打包 → 编茧壁 → 写清单\n              pack 档 = 跳过构建，其余同上 · verify 档 = 只校验（只读）\n  会写哪些件: <REPO>/dist/<VER>/（交叉编三件）· <REPO>/dist/<VER>/release/（8 件 + 逐件 .sha256 + checksums.txt + manifest.json）· wall/target/（茧壁）\n  ⇒ 想走**不重打包**的那一档用：--manifest-only " + prDryTok + "\n", wantErr: ""},
	{name: "plan-manifest", wantRC: 0, wantOut: "计划件（--manifest-only " + prDryTok + " · 零副作用 —— 未执行、未改任何状态）\n  动作     : 重生成发布清单（只重算清单与 release 目录内逐件 sha256）\n  危险档   : D2（只写 <REPO>/dist/<VER>/release 一个目录；不编译、不重打包、不碰 bin/ 里在跑的件）\n  会跑哪几步:\n    ① 前置检查：<REPO>/dist/<VER>/release 存在且内有 zerg-* 制品（不在 ⇒ 退 8 · 不给结论）\n    ② 逐件重算 <REPO>/dist/<VER>/release/<件>.sha256 与 <REPO>/dist/<VER>/release/checksums.txt\n    ③ ZERG_MANIFEST_LOCAL=1 python3 scripts/evals/make-manifest.py <REPO>/dist/<VER>/release <版本> <commit> <build_time>\n       （版本/commit/build_time 取**既有清单**，退而取 build-info.json —— 保「这批制品是谁产的」不变）\n    ④ 复跑校验段：manifest 逐件 sha256 / 体积必须与文件一致\n  会写哪些件（**全在 publish/ 外** · 只此三处）:\n    <REPO>/dist/<VER>/release/manifest.json\n    <REPO>/dist/<VER>/release/<逐件>.sha256\n    <REPO>/dist/<VER>/release/checksums.txt\n  不会动   : bin/（在跑的件）· wall/target/ · 不重编译 · 不拷贝二进制 · 不重签 · 不重启\n  执行要   : --confirm=<主机名> 与 --yes 同时到（命令面：zerg build release --manifest-only --confirm=<主机名> --yes）\n", wantErr: "（--manifest-only " + prDryTok + "：只出计划件 · 零副作用 —— 未执行、未改任何状态）\n"},
	{name: "plan-nobuild", wantRC: 0, wantOut: "计划件（" + prDryTok + " · 零副作用 —— 未执行、未改任何状态）\n  动作     : pack-release.sh（档 = pack）\n  会跑哪几步: build 档 = build-all.sh --dist → 交叉编 linux 三件 → rm -rf 重打包 → 编茧壁 → 写清单\n              pack 档 = 跳过构建，其余同上 · verify 档 = 只校验（只读）\n  会写哪些件: <REPO>/dist/<VER>/（交叉编三件）· <REPO>/dist/<VER>/release/（8 件 + 逐件 .sha256 + checksums.txt + manifest.json）· wall/target/（茧壁）\n  ⇒ 想走**不重打包**的那一档用：--manifest-only " + prDryTok + "\n", wantErr: ""},
	{name: "plan-verify", wantRC: 0, wantOut: "计划件（" + prDryTok + " · 零副作用 —— 未执行、未改任何状态）\n  动作     : pack-release.sh（档 = verify）\n  会跑哪几步: build 档 = build-all.sh --dist → 交叉编 linux 三件 → rm -rf 重打包 → 编茧壁 → 写清单\n              pack 档 = 跳过构建，其余同上 · verify 档 = 只校验（只读）\n  会写哪些件: <REPO>/dist/<VER>/（交叉编三件）· <REPO>/dist/<VER>/release/（8 件 + 逐件 .sha256 + checksums.txt + manifest.json）· wall/target/（茧壁）\n  ⇒ 想走**不重打包**的那一档用：--manifest-only " + prDryTok + "\n", wantErr: ""},
	{name: "unknown", wantRC: 2, wantOut: "", wantErr: "未知参数: --no-such-flag-zz\n"},
}

// prArgvByName —— 金标档名 ⇒ argv（与 prCases/prHeavyCases 逐格同）。
func prArgvByName(name string) []string {
	m := map[string][]string{
		"plan-general":  {prDryTok},
		"plan-manifest": {prManifestOnly, prDryTok},
		"plan-nobuild":  {"--no-build", prDryTok},
		"plan-verify":   {"--verify", prDryTok},
		"unknown":       {"--no-such-flag-zz"},
		"manifest-only": {prManifestOnly},
		"verify":        {"--verify"},
	}
	return m[name]
}

// prRunFixtureGolden —— 逐格现建夹具、现跑 Go 半、归一后与冻值逐字比；返回失配格数。
// plant 非 nil 时在跑前往夹具落「旧态」sh 件（两态用）。
func prRunFixtureGolden(t *testing.T, tag string, plant func(fx string)) int {
	t.Helper()
	repo := prRepoRoot(t)
	ver := prVersion(repo)
	mism := 0
	for _, c := range prFixtureGolden {
		if c.name == "manifest-only" && !prHasExe("python3") {
			t.Logf("[%s/%s] 不适用：本机无 python3（清单生成器属外部工具链面）⇒ 该格未覆盖（✗ 不当绿）", tag, c.name)
			continue
		}
		fx, _, _ := prBuildFixture(t, repo)
		if plant != nil {
			plant(fx)
		}
		rc, out, errb := prRunGo(fx, prArgvByName(c.name))
		phys, _ := filepath.EvalSymlinks(fx)
		gotOut := prNormGolden(string(out), fx, phys, "<FX>", ver)
		gotErr := prNormGolden(string(errb), fx, phys, "<FX>", ver)
		if rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, rc, c.wantRC)
		}
		if gotOut != c.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, c.name, gotOut, c.wantOut)
		}
		if gotErr != c.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, c.name, gotErr, c.wantErr)
		}
	}
	return mism
}

// TestPackReleaseGoldenInternalized —— A 态（真仓 · 退役 sh 不在盘）金标自证（✗ 不 Skip ·
// 逐格真断言 · ✗ 不放宽为「只判非空」）。
func TestPackReleaseGoldenInternalized(t *testing.T) {
	repo := prRepoRoot(t)
	prAnchorGoHalf(t, repo) // 哨兵：锚 Go 半在盘 ⇒ fail-closed（✗ 不 Skip）
	if sh := filepath.Join(repo, filepath.FromSlash(prShRel)); prIsFile(sh) {
		t.Fatalf("A 态前提破：退役 sh 竟在盘 %s（本件前提 = 内化后无 sh）", sh)
	}
	if len(prFixtureGolden) < 5 {
		t.Fatalf("金标格数 %d < 5", len(prFixtureGolden))
	}
	if n := prRunFixtureGolden(t, "A真仓无sh", nil); n != 0 {
		t.Fatalf("A 态夹具金标失配 %d 格", n)
	}
}

// TestPackReleaseGoldenTwoStates —— 两态逐字同：A 态 = 夹具无 sh（真仓现态）；B 态 = 合成「旧态」
// 夹具（把退役 sh 路径 `scripts/build/pack-release.sh` 放回）⇒ 两态对同一套金标逐字同命中
// （✗ 不 Skip）。证 sh 在/不在盘都不改判据结论（内化彻底 · 将来若有人让 Go 半重新去读 sh，B 态当场分家）。
func TestPackReleaseGoldenTwoStates(t *testing.T) {
	repo := prRepoRoot(t)
	prAnchorGoHalf(t, repo)
	if n := prRunFixtureGolden(t, "A夹具无sh", nil); n != 0 {
		t.Fatalf("A 态夹具金标失配 %d 格", n)
	}
	plant := func(fx string) {
		prWriteFile(t, filepath.Join(fx, "scripts", "build", "pack-release.sh"),
			[]byte("#!/bin/sh\n# stub：造「旧态」——退役 sh 在位（本件只证 sh 在/不在盘不改判据结论）\n"))
	}
	if n := prRunFixtureGolden(t, "B旧态有sh", plant); n != 0 {
		t.Fatalf("B 态夹具金标失配 %d 格（sh 在盘不得改判据结论）", n)
	}
}

// TestPackReleaseRepoReadOnlyGolden —— 真仓只读档金标：plan-* 干跑 / 未知参（只读 ⇒ 不写 `dist/`、
// 不碰 `bin/`）。哨兵锚 Go 半 · fail-closed（✗ 不留 Skip）。
func TestPackReleaseRepoReadOnlyGolden(t *testing.T) {
	repo := prRepoRoot(t)
	prAnchorGoHalf(t, repo)
	ver := prVersion(repo)
	phys, _ := filepath.EvalSymlinks(repo)
	mism := 0
	for _, c := range prRepoGolden {
		rc, out, errb := prRunGo(repo, prArgvByName(c.name))
		gotOut := prNormGolden(string(out), repo, phys, "<REPO>", ver)
		gotErr := prNormGolden(string(errb), repo, phys, "<REPO>", ver)
		if rc != c.wantRC {
			mism++
			t.Errorf("[真仓/%s] rc=%d（冻值 %d）", c.name, rc, c.wantRC)
		}
		if gotOut != c.wantOut {
			mism++
			t.Errorf("[真仓/%s] stdout 漂移\n实得=%q\n冻值=%q", c.name, gotOut, c.wantOut)
		}
		if gotErr != c.wantErr {
			mism++
			t.Errorf("[真仓/%s] stderr 漂移\n实得=%q\n冻值=%q", c.name, gotErr, c.wantErr)
		}
	}
	if mism != 0 {
		t.Fatalf("真仓只读金标失配 %d 格", mism)
	}
}

// TestPackReleasePoisonControl —— ④ 有牙负控：把脚本换成毒脚本 ⇒
// (a) 直跑脚本必须露出 POISON/99（证「毒」真在生效 · 否则本负控无牙）；
// (b) Go 半读数必须**一字不动**（证不再 exec 那个脚本 ⇒ 真搬）。
// ★ 本格不需要 shell 半真件（它自带毒脚本覆写夹具里那一件）⇒ **退役态照跑（仍绿）**。两半皆缺 ⇒
// `prHalvesGuard` fail-closed。
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
