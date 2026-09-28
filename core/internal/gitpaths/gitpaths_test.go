package gitpaths

// gitpaths_test.go —— 件名统一出口的判据格（A1 的验收格 · 含牙齿格）。
//
// ★★ 本轮**未跑**：派单硬约束禁 `go build/vet/test/run`（本轮只交付源码 + 仓外机制证）
// ⇒ 本件属**源码审读**物。落地的第一件事就是 `go test ./internal/gitpaths/` 把它跑起来。
//
// 格子（判据 → 变异自证）：
//
//	① TestList_ChineseName            纯中文名 ⇒ 真名（UTF-8 字节逐字节相等）
//	② TestList_QuotedName             含 `"` 的名（引号在**中段**）⇒ 真名
//	③ TestList_BackslashName          含 `\` 的名 ⇒ 真名（`\` 后**不是**三位八进制 ⇒ 不属 C-5 面）
//	④ TestList_EscapedName_HardFail   ★ 牙齿格：故意喂**转义形态** ⇒ List 必须报错，
//	                                  且**不许静默清洗**（entries 必须为空）
//	⑤ TestFaceArgs_AllCarryZ           C-2 的牙：**七类**面的 argv 必须都含 `-z`（拿掉任一处 ⇒ 本格红）
//	⑥ TestAssertGlobalOptions_RejectsTrailingC   C-3：`-c` 落在子命令之后 ⇒ 必报错
//	⑦ TestList_RejectsQuoteOctal      C-8：显式请求 Octal 档 ⇒ 必报错（不静默降级）
//	⑧ TestList_StatusFace_StripPrefix status 面 `XY <path>` 按协议剥前缀；rename 源件原样收
//	⑨ TestList_GrepFace_NeedsPattern  grep 面缺模式 ⇒ 必报错
//	⑩ TestSplitNUL_RejectsInnerEmpty  中间空段 ⇒ 必报错（不静默丢件）
//	⑪ TestList_OthersFace_UntrackedChineseName  ★ 未跟踪面（FaceOthers）：中文**未跟踪**件 ⇒ 真名，
//	                                   且输出**只**含未跟踪那一件（已跟踪件不出现在本面）
//	⑫ TestFaceOthersArgs_CarryZAndOthers ★ 结构断言：argv 逐字 = `ls-files --others
//	                                   --exclude-standard -z [-- <前缀>]`，`-z` 在子命令**之后**、无 `-c`
//	⑬ TestWithPrefix_ZeroOneManyStates   ★ A1 补面甲：多 pathspec 三态 —— 零段 ⇒ argv 无 `--`；
//	                                   一段 ⇒ 与旧单参形态**逐字相同**；N 段 ⇒ `--` 后按序、
//	                                   且 `--` 只出现一次；夹空串 ⇒ 硬失败（不静默跳过）
//	⑭ TestFaceDiffCachedArgs_Verbatim  ★ A1 补面三（索引 vs HEAD）：新档 argv 逐字 =
//	                                   `-C <root> diff --cached --name-only -z`
//	                                   （`--cached` 紧贴子命令、`-z` 在子命令**之后**）
//	⑮ TestFaceDiffArgs_Unchanged       ★ 旧面回归：`FaceDiff` argv 逐字不变 =
//	                                   `-C <root> diff --name-only -z`（无 `--cached`）
//	⑯ TestFaceDiffCached_WithPrefix    新档与 WithPrefix **共存**三态（零段无 `--` /
//	                                   一段 / N 段，`--` 恰 1 次）+ 夹空串硬失败
//	⑰ TestList_DiffCachedFace_NoRevCrosstalk ★ 牙齿格：新档**不吃** WithRev
//	                                   （`git diff --cached <rev>` = 索引 vs <rev>，属另一比较对）
//
// ★ 全格**不跑 git**：假运行器注入 gitRunnerImpl（不碰真仓、不依赖环境、离线可跑）。
//   真跑 git 的行为验收另在仓外探针仓做（见回执「A 段 / B 段」）。
// ⑱ TestWithErrorUnmatch_ThreeStates       ★ A1 补面四：新选项的三态 —— 不传 ⇒ argv 与现读**逐字相同**；
//                                      传 + 有前缀 ⇒ `--error-unmatch` 紧贴 `--` 之前（与 `--` 共存）；
//                                      传 + 无前缀 ⇒ 也合法（探针仓实测 rc=0、照常列全件）
// ⑲ TestWithErrorUnmatch_FaceAllowList     ★ 只有 ls-files 系列两面（FaceTracked / FaceOthers）吃
//                                      `--error-unmatch`；其余五面 + 未知面值 ⇒ **硬失败**（禁静默丢）
// ⑳ TestList_PathspecMissing_Judgeable     ★ 牙齿格：「pathspec 不存在」可判（ErrPathspecNotFound /
//                                      IsPathspecMissing），判据 = errors.As(*exec.ExitError) + ExitCode()==1，
//                                      **不解析错误文本**；未开严档 / 非 1 退出码两条负控
// ㉑ TestFaceStatusArgs_ZeroStateUnchanged  ★ A1 补面五零回归格：FaceStatus **零选项** argv 与现读
//                                      逐字相同（`-C /probe status --porcelain -z`；无 `--`、无旋钮）
// ㉒ TestFaceStatus_WithPrefix              ★ A1 补面五·甲：status 面与 WithPrefix 共存三态
//                                      （零段无 `--` / 一段 / N 段，`--` 恰 1 次）+ 运行期判「出口仍吐件名」
// ㉓ TestWithUntrackedFiles_ThreeStates     ★ A1 补面五·乙：`--untracked-files=<mode>` 三态
//                                      （不传 ⇒ 逐字同现读 / all / no / normal）+ 位次（紧贴 `--` 之前）
//                                      + 与 pathspec 共存 + 未知档值构造即硬失败
// ㉔ TestWithUntrackedFiles_FaceAllowList   ★ 面名单闸：只有 status 面吃该旋钮（其余六面 + 未知面值硬失败）
//                                      ＋ **C-9 七类齐守卫**：`Face(7)`/`Face(8)` 必须硬失败
//                                      （XY 记录面属另一类面，未立设计稿前不许出现）
// 助手 TestZHelperProcess_ExitCode         ⑳ 的子进程助手（造**真的** *exec.ExitError：os.ProcessState
//                                      不可内建，零值取 ExitCode() 会 panic）——**不跑 git**

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// fakeGit —— 注入假运行器（唯一注入点 = 包级 var gitRunnerImpl），返回复原函数。
// ★ 注入而非「跑真 git」：判据要判的是**切分与自检**，不该把 git 行为当作被测面。
func fakeGit(t *testing.T, out []byte) func() {
	t.Helper()
	old := gitRunnerImpl
	gitRunnerImpl = func(argv []string) ([]byte, error) { return out, nil }
	return func() { gitRunnerImpl = old }
}

// ① 纯中文名 —— `-z` 出口的原样字节。
func TestList_ChineseName(t *testing.T) {
	defer fakeGit(t, []byte("中文名件.md\x00"))()
	got, err := List("/probe", FaceTracked)
	if err != nil {
		t.Fatalf("真名不该报错，实得 %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("应 1 件，实得 %d 件", len(got))
	}
	want := []byte("中文名件.md")
	if !bytes.Equal(got[0].Bytes(), want) {
		t.Fatalf("字节面不等：want %q got %q", want, got[0].Bytes())
	}
	if got[0].String() != "中文名件.md" { // 末端派生
		t.Fatalf("字符串面不等：%q", got[0].String())
	}
}

// ② 含 `"` 的名 —— 引号在**中段**：首尾判定不该误伤。
func TestList_QuotedName(t *testing.T) {
	defer fakeGit(t, []byte("a\"q.txt\x00"))()
	got, err := List("/probe", FaceTracked)
	if err != nil {
		t.Fatalf("中段引号是合法件名，不该报错，实得 %v", err)
	}
	if len(got) != 1 || got[0].String() != "a\"q.txt" {
		t.Fatalf("应得 1 件 a\"q.txt，实得 %d 件", len(got))
	}
}

// ③ 含 `\` 的名 —— C-5 的命中面只认 `\` + 三位八进制，裸反斜杠名必须放行。
func TestList_BackslashName(t *testing.T) {
	defer fakeGit(t, []byte("b\\s.txt\x00"))()
	got, err := List("/probe", FaceTracked)
	if err != nil {
		t.Fatalf("裸反斜杠名不是转义形态，不该报错，实得 %v", err)
	}
	if len(got) != 1 || got[0].String() != "b\\s.txt" {
		t.Fatalf("应得 1 件 b\\s.txt，实得 %d 件", len(got))
	}
}

// ④ ★ 牙齿格 —— 故意喂一个转义形态（首尾引号 + `\` + 八进制）：
// List **必须硬失败**，且**不得**静默清洗（把件名「修」成看着正常的样子）。
//
// 变异自证：把 checkVerbatim 换成 `return nil`（或改成「去引号 + 反转义」的清洗实现）
// ⇒ 本格必红（前者：err==nil；后者：entries 非空且内容≠喂入字节）。
func TestList_EscapedName_HardFail(t *testing.T) {
	forged := []byte("\"docs/\\350\\257\\264\\346\\230\\216.md\"\x00") // = "docs/\350…\216.md" + NUL
	defer fakeGit(t, forged)()

	got, err := List("/probe", FaceTracked)
	if err == nil {
		t.Fatalf("转义形态必须硬失败，实得 err=nil entries=%q", got)
	}
	var e *EscapedNameError
	if !errors.As(err, &e) {
		t.Fatalf("应是 *EscapedNameError，实得 %T: %v", err, err)
	}
	if len(got) != 0 {
		t.Fatalf("硬失败不许返回半成品件名（= 静默清洗的味道），实得 %q", got)
	}
	if !strings.Contains(e.Reason, `"`) && !strings.Contains(e.Reason, "八进制") {
		t.Fatalf("原因须点名（引号 / 八进制），实得 %q", e.Reason)
	}

	// 负控成对：同一形态去掉「转义」后必须放行 —— 证本格判的是转义、不是「含反斜杠」。
	if err := checkVerbatim([]byte("b\\s.txt")); err != nil {
		t.Fatalf("负控①（裸反斜杠）应放行，实得 %v", err)
	}
	if err := checkVerbatim([]byte("b\\163.txt")); err == nil {
		t.Fatalf("负控②（`\\` + 三位八进制）应命中，实得 err=nil")
	}
}

// ⑤ C-2 的牙 —— 七类面的 argv 必须都带 `-z`。
// 变异自证：把任一面 args 里的 "-z" 拿掉 ⇒ 本格红。
func TestFaceArgs_AllCarryZ(t *testing.T) {
	faces := []struct {
		face Face
		opt  []Option
	}{
		{FaceTracked, []Option{WithPrefix("core/")}},
		{FaceTree, []Option{WithRev("HEAD~1")}},
		{FaceDiff, nil},
		{FaceDiffCached, nil},
		{FaceStatus, nil},
		{FaceGrep, []Option{WithPattern("TODO")}},
		{FaceOthers, []Option{WithPrefix("core/")}},
	}
	for _, c := range faces {
		cfg := config{mode: QuoteVerbatim}
		for _, o := range c.opt {
			if err := o(&cfg); err != nil {
				t.Fatalf("%s: 选项构造失败 %v", c.face, err)
			}
		}
		argv, err := buildArgv("/probe", c.face, &cfg)
		if err != nil {
			t.Fatalf("%s: buildArgv 失败 %v", c.face, err)
		}
		if !contains(argv, "-z") {
			t.Fatalf("%s 面 argv 缺 `-z`（C-2）：%v", c.face, argv)
		}
	}
}

// ⑥ C-3 —— `-c` 落在子命令之后必须报错（放错位置会静默走差路）。
func TestAssertGlobalOptions_RejectsTrailingC(t *testing.T) {
	bad := []string{"-C", "/probe", "ls-files", "-c", "core.quotepath=false"}
	if err := assertGlobalOptionsFirst(bad); err == nil {
		t.Fatalf("C-3 违规必须报错：%v", bad)
	}
	good := []string{"-C", "/probe", "-c", "core.quotepath=false", "ls-files", "-z"}
	if err := assertGlobalOptionsFirst(good); err != nil {
		t.Fatalf("全局区里的 `-c` 应放行，实得 %v（argv=%v）", err, good)
	}
	// `--` 之后的段是 pathspec，不是选项：不该被误判。
	pathspec := []string{"-C", "/probe", "ls-files", "-z", "--", "-c"}
	if err := assertGlobalOptionsFirst(pathspec); err != nil {
		t.Fatalf("pathspec 里的字面 `-c` 应放行，实得 %v", err)
	}
}

// ⑦ C-8 —— 显式请求 Octal 档必须报错（虫族不产出转义档，且不静默降级）。
func TestList_RejectsQuoteOctal(t *testing.T) {
	defer fakeGit(t, []byte("ok.txt\x00"))()
	if _, err := List("/probe", FaceTracked, WithQuoteMode(QuoteOctal)); err == nil {
		t.Fatalf("QuoteOctal 必须被拒")
	}
}

// ⑧ status 面 —— 协议前缀（`XY `）剥掉；rename 源件（无前缀段）原样收。
func TestList_StatusFace_StripPrefix(t *testing.T) {
	// 实测形态（探针仓现读）：` M a"q.txt\0` 与重命名的 `R  new\0old\0`
	defer fakeGit(t, []byte(" M a\"q.txt\x00R  new\u4e2d\u6587.md\x00old\u4e2d\u6587.md\x00"))()
	got, err := List("/probe", FaceStatus)
	if err != nil {
		t.Fatalf("不该报错，实得 %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("应 3 件（改 1 + 改名 2 段），实得 %d 件：%q", len(got), got)
	}
	if got[0].String() != "a\"q.txt" || got[1].String() != "new中文.md" || got[2].String() != "old中文.md" {
		t.Fatalf("前缀剥离结果不符：%q %q %q", got[0].String(), got[1].String(), got[2].String())
	}
}

// ⑨ grep 面缺模式 ⇒ 报错（不许退化成「扫 0 件」的静默绿）。
func TestList_GrepFace_NeedsPattern(t *testing.T) {
	defer fakeGit(t, nil)()
	if _, err := List("/probe", FaceGrep); err == nil {
		t.Fatalf("grep 面缺 WithPattern 必须报错")
	}
}

// ⑩ 中间空段（`\0` 连缀）⇒ 报错，不静默丢件。
func TestSplitNUL_RejectsInnerEmpty(t *testing.T) {
	if _, err := splitNUL([]byte("a\x00\x00b\x00")); err == nil {
		t.Fatalf("中间空段必须报错")
	}
	got, err := splitNUL([]byte("a\x00b\x00")) // 末尾空段是 git 的收尾，合法
	if err != nil || len(got) != 2 {
		t.Fatalf("末尾空段应放行并得 2 件，实得 %d 件 err=%v", len(got), err)
	}
	if _, err := splitNUL(nil); err != nil {
		t.Fatalf("空输出应得 0 件而非报错，实得 %v", err)
	}
}

// fakeGitCapture —— 假运行器 + **记下 argv**：⑪ 既要断件名真值，也要断「发的确实是未跟踪面
// 那一攮 argv」（否则 FakeTracked 的输出也能让件名断言变绿 —— 那就成了假绿）。
func fakeGitCapture(t *testing.T, out []byte) (*[]string, func()) {
	t.Helper()
	old := gitRunnerImpl
	var seen []string
	gitRunnerImpl = func(argv []string) ([]byte, error) {
		seen = append([]string(nil), argv...)
		return out, nil
	}
	return &seen, func() { gitRunnerImpl = old }
}

// ⑪ ★ 未跟踪面（FaceOthers）—— 中文**未跟踪**件 ⇒ 真名，且只吐未跟踪那一件。
//
// 格子类型：**行为断言**（切分 + C-5 自检 + 面 argv 三条一起判）。
// 探针真值来源：仓外探针仓（已跟踪中文件 + 未跟踪中文件），
// `git ls-files --others --exclude-standard -z` 的字节面实测为
// `b"\xe6\x9c\xaa\xe8\xb7\x9f\xe8\xb8\xaa\xe4\xb8\xad\xe6\x96\x87.md\x00"`
// （= `未跟踪中文.md` + NUL），**不含**已跟踪件 ⇒ 本格据此喂假面。
//
// 变异自证：① 把 FaceOthers 的 args 改回 `ls-files -z`（= 索引面）⇒ 本格的
// 「argv 必须带 --others」与「已跟踪面 argv 必须与之不同」两断同时红；
// ② 把 args 里的 `-z` 拿掉 ⇒ ⑤ 与本格一起红（并会在真仓上让 C-5 自检命中）。
func TestList_OthersFace_UntrackedChineseName(t *testing.T) {
	const tracked = "已跟踪中文.md"
	seen, restore := fakeGitCapture(t, []byte("未跟踪中文.md\x00"))
	defer restore()

	got, err := List("/probe", FaceOthers)
	if err != nil {
		t.Fatalf("未跟踪面的真名不该报错，实得 %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("未跟踪面应 1 件，实得 %d 件：%q", len(got), got)
	}
	if !bytes.Equal(got[0].Bytes(), []byte("未跟踪中文.md")) {
		t.Fatalf("字节面不等：got %q", got[0].Bytes())
	}
	if got[0].String() != "未跟踪中文.md" { // 末端派生
		t.Fatalf("字符串面不等：%q", got[0].String())
	}

	// 「只吐未跟踪那一件」：已跟踪件名不得出现在本面输出里（两面不许互替）。
	for _, e := range got {
		if e.String() == tracked {
			t.Fatalf("未跟踪面吐出了已跟踪件 %q ⇒ 两面混面", tracked)
		}
	}

	// 发的确实是未跟踪面那一攮 argv（不是拿索引面冒充）。
	if !contains(*seen, "--others") || !contains(*seen, "--exclude-standard") {
		t.Fatalf("FaceOthers 的 argv 必须带 `--others --exclude-standard`：%v", *seen)
	}
	if !contains(*seen, "-z") {
		t.Fatalf("FaceOthers 的 argv 必须带 `-z`（C-2）：%v", *seen)
	}

	// 与已跟踪面**可区分**：两面的 argv 必须不同（同形不等于同攮）。
	tcfg := config{mode: QuoteVerbatim}
	trackedArgv, err := buildArgv("/probe", FaceTracked, &tcfg)
	if err != nil {
		t.Fatalf("FaceTracked 的 buildArgv 失败 %v", err)
	}
	if contains(trackedArgv, "--others") {
		t.Fatalf("索引面不该带 `--others`：%v", trackedArgv)
	}
	same := len(trackedArgv) == len(*seen)
	if same {
		for i := range trackedArgv {
			if trackedArgv[i] != (*seen)[i] {
				same = false
				break
			}
		}
	}
	if same {
		t.Fatalf("未跟踪面与索引面的 argv 完全相同 ⇒ A2 无法据 argv 区分两面：%v", *seen)
	}
}

// ⑫ ★ 结构断言 —— FaceOthers 的 argv 逐字形态 + `-z` / `-c` 位置。
//
// 格子类型：**结构断言**（直接断 buildArgv 的字符串切片，不喂假输出）。
// 断的逐字形态（无前缀 / 有前缀两版）：
//
//	`-C <root> ls-files --others --exclude-standard -z`
//	`-C <root> ls-files --others --exclude-standard -z -- <前缀>`
//
// 变异自证：`-z` 挪到子命令之前（`-z ls-files …`）⇒ 本格红；拿掉 `-z` ⇒ 本格红；
// 经 WithQuoteMode(QuoteRespectConfig) 让 `-c` 落到子命令之后 ⇒ 本格红。
func TestFaceOthersArgs_CarryZAndOthers(t *testing.T) {
	cfg := config{mode: QuoteVerbatim}
	if err := WithPrefix("sub/")(&cfg); err != nil {
		t.Fatalf("WithPrefix 构造失败 %v", err)
	}
	argv, err := buildArgv("/probe", FaceOthers, &cfg)
	if err != nil {
		t.Fatalf("buildArgv 失败 %v", err)
	}
	want := []string{"-C", "/probe", "ls-files", "--others", "--exclude-standard", "-z", "--", "sub/"}
	if len(argv) != len(want) {
		t.Fatalf("argv 逐字不等（长度）：want %v got %v", want, argv)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv[%d] 逐字不等：want %q got %q（整条 %v）", i, want[i], argv[i], argv)
		}
	}

	// `-z` 位置：必须在**子命令之后**（`-z` 是子命令选项，不是全局选项）。
	sub, z := -1, -1
	for i, a := range argv {
		if a == "ls-files" {
			sub = i
		}
		if a == "-z" {
			z = i
		}
	}
	if sub < 0 {
		t.Fatalf("argv 里没有子命令 ls-files：%v", argv)
	}
	if z < 0 {
		t.Fatalf("FaceOthers 的 argv 必带 `-z`（会转义面）：%v", argv)
	}
	if z < sub {
		t.Fatalf("`-z` 落在子命令之前（%d < %d）⇒ 位置铁律破：%v", z, sub, argv)
	}
	// `--` 之后是 pathspec，不是选项：前缀不得被当成选项。
	if argv[len(argv)-2] != "--" {
		t.Fatalf("前缀前必须有 `--`（免选项歧义）：%v", argv)
	}
	// 默认档（QuoteVerbatim）下不得有 `-c`。
	if contains(argv, "-c") {
		t.Fatalf("默认档不该出现 `-c`：%v", argv)
	}

	// 应急档：`-c` 若出现，**恒在子命令之前**（C-3 的机检在 List 里，此处断位置）。
	ecfg := config{mode: QuoteRespectConfig}
	if err := WithPrefix("sub/")(&ecfg); err != nil {
		t.Fatalf("WithPrefix 构造失败 %v", err)
	}
	eargv, err := buildArgv("/probe", FaceOthers, &ecfg)
	if err != nil {
		t.Fatalf("buildArgv(RespectConfig) 失败 %v", err)
	}
	cidx := -1
	for i, a := range eargv {
		if a == "-c" {
			cidx = i
		}
	}
	if cidx < 0 {
		t.Fatalf("RespectConfig 档必须补 `-c core.quotepath=false`：%v", eargv)
	}
	if cidx > sub {
		t.Fatalf("`-c` 落在子命令之后 ⇒ C-3 破：%v", eargv)
	}
	if err := assertGlobalOptionsFirst(eargv); err != nil {
		t.Fatalf("应急档 argv 应过 C-3 复查，实得 %v（argv=%v）", err, eargv)
	}
}

// ⑬ ★ A1 补面甲（多 pathspec）—— 三态兼容性 + 空段硬失败。
//
// 格子类型：**结构断言**（直接断 buildArgv 的字符串切片，不喂假输出、不跑 git）。
//
// 逐字形态（FaceTracked 面，root=/probe）：
//
//	零段  `-C /probe ls-files -z`                          ← 不许出现 `--`
//	一段  `-C /probe ls-files -z -- core/`                  ← 与旧单参行为逐字相同
//	N段   `-C /probe ls-files -z -- core/ scripts/ 中文 目录/`  ← `--` 一次 + 按调用序
//
// 变异自证：把 pathspecSuffix 的「零段早退」拿掉（零段也拼 `--`）⇒ 三态①红；
//
//	把 `more...` 丢掉（只收首参）⇒ 三态③红；把空段检查删掉 ⇒ 空段格红；
//	把 pathspecSuffix 改成 `append(a, "--")` 后再逐段拼（`--` 出现多次）⇒ 三态③的
//	「`--` 只出现一次」断红。
func TestWithPrefix_ZeroOneManyStates(t *testing.T) {
	// 三态①：零段（不调用 WithPrefix）⇒ argv 里不出现 `--`。
	z := config{mode: QuoteVerbatim}
	zargv, err := buildArgv("/probe", FaceTracked, &z)
	if err != nil {
		t.Fatalf("零段 buildArgv 失败 %v", err)
	}
	if contains(zargv, "--") {
		t.Fatalf("零段不该出现 `--`（与旧行为逐字相同）：%v", zargv)
	}
	wantZero := []string{"-C", "/probe", "ls-files", "-z"}
	if !equalArgv(zargv, wantZero) {
		t.Fatalf("零段 argv 逐字不等（`-z` 属本面固定拼法 · 见本格逐字形态）：want %v got %v", wantZero, zargv)
	}

	// 三态②：一段 ⇒ 与旧单参形态逐字相同（向后兼容，旧调用点无需改）。
	one := config{mode: QuoteVerbatim}
	if err := WithPrefix("core/")(&one); err != nil {
		t.Fatalf("一段 WithPrefix 构造失败 %v", err)
	}
	oargv, err := buildArgv("/probe", FaceTracked, &one)
	if err != nil {
		t.Fatalf("一段 buildArgv 失败 %v", err)
	}
	wantOne := []string{"-C", "/probe", "ls-files", "-z", "--", "core/"}
	if !equalArgv(oargv, wantOne) {
		t.Fatalf("一段 argv 逐字不等：want %v got %v", wantOne, oargv)
	}

	// 三态③：N 段 ⇒ `--` 一次 + 其后按调用序，且 FaceTracked / FaceOthers 两面同形。
	many := []string{"core/", "scripts/", "中文 目录/"}
	mcfg := config{mode: QuoteVerbatim}
	if err := WithPrefix(many[0], many[1:]...)(&mcfg); err != nil {
		t.Fatalf("N 段 WithPrefix 构造失败 %v", err)
	}
	wantMany := []string{"-C", "/probe", "ls-files", "-z", "--", "core/", "scripts/", "中文 目录/"}
	margv, err := buildArgv("/probe", FaceTracked, &mcfg)
	if err != nil {
		t.Fatalf("N 段 buildArgv 失败 %v", err)
	}
	if !equalArgv(margv, wantMany) {
		t.Fatalf("N 段 argv 逐字不等：want %v got %v", wantMany, margv)
	}
	if n := countArg(margv, "--"); n != 1 {
		t.Fatalf("`--` 应恰出现 1 次，实得 %d 次：%v", n, margv)
	}
	// 段的次序必须与调用序一致（不许排序 / 去重：排序属调用点语义）。
	di := indexArg(margv, "--")
	if di < 0 || di+3 >= len(margv) {
		t.Fatalf("`--` 之后的段数不足：%v", margv)
	}
	for k, want := range many {
		if margv[di+1+k] != want {
			t.Fatalf("pathspec[%d] 顺序不符：want %q got %q（整条 %v）", k, want, margv[di+1+k], margv)
		}
	}
	// 两面同吃：FaceOthers 的 N 段形态 = 未跟踪子命令 + 同一段序列。
	ocfg := config{mode: QuoteVerbatim}
	if err := WithPrefix(many[0], many[1:]...)(&ocfg); err != nil {
		t.Fatalf("FaceOthers N 段 WithPrefix 构造失败 %v", err)
	}
	wantOthers := []string{"-C", "/probe", "ls-files", "--others", "--exclude-standard", "-z", "--", "core/", "scripts/", "中文 目录/"}
	oargv2, err := buildArgv("/probe", FaceOthers, &ocfg)
	if err != nil {
		t.Fatalf("FaceOthers N 段 buildArgv 失败 %v", err)
	}
	if !equalArgv(oargv2, wantOthers) {
		t.Fatalf("FaceOthers N 段 argv 逐字不等：want %v got %v", wantOthers, oargv2)
	}
	// C-3 复查必须放行多段形态（`assertGlobalOptionsFirst` 遇到 `--` 即停）。
	if err := assertGlobalOptionsFirst(margv); err != nil {
		t.Fatalf("N 段 argv 应过 C-3 复查，实得 %v", err)
	}

	// 空段 ⇒ 硬失败（不许静默跳过：空 pathspec 会被 git 当整仓）。
	if err := WithPrefix("")(&config{mode: QuoteVerbatim}); err == nil {
		t.Fatalf("首段空串必须报错")
	}
	if err := WithPrefix("core/", "")(&config{mode: QuoteVerbatim}); err == nil {
		t.Fatalf("N 段里夹空串必须报错")
	}
	if err := WithPrefix("core/", "scripts/")(&config{mode: QuoteVerbatim}); err != nil {
		t.Fatalf("两段正常前缀不该报错，实得 %v", err)
	}
}

// ⑭ ★ A1 补面三（索引 vs HEAD）—— 新档 argv **逐字**。
//
// 格子类型：**结构断言**（直接断 buildArgv 的字符串切片，不喂假输出、不跑 git）。
//
// 逐字形态（root=/probe，默认档 QuoteVerbatim）：
//
//	`-C /probe diff --cached --name-only -z`
//
// 两条位置铁律一起判：① `--cached` **紧贴子命令** `diff`（与仓内 4 处真实调用点的 argv
// `<git> -c core.quotepath=false diff --cached --name-only` 同形 —— 该 4 处无 `-z`，
// 本面在其后补 `-z`，`--cached` 的位次逐字照旧）；② `-z` 在子命令**之后**
// （`-z` 是子命令选项：`git -z diff` 被 git 判未知开关）。
//
// 变异自证：把 `--cached` 挪到 `-z` 之后（= 借 rev 槽塞 `--cached` 的旧形态）⇒ 本格红；
// 拿掉 `-z` ⇒ 本格红；把 `-z` 挪到子命令之前 ⇒ 本格红。
func TestFaceDiffCachedArgs_Verbatim(t *testing.T) {
	cfg := config{mode: QuoteVerbatim}
	argv, err := buildArgv("/probe", FaceDiffCached, &cfg)
	if err != nil {
		t.Fatalf("buildArgv 失败 %v", err)
	}
	want := []string{"-C", "/probe", "diff", "--cached", "--name-only", "-z"}
	if !equalArgv(argv, want) {
		t.Fatalf("新档 argv 逐字不等：want %v got %v", want, argv)
	}
	sub, cached, z := indexArg(argv, "diff"), indexArg(argv, "--cached"), indexArg(argv, "-z")
	if sub < 0 {
		t.Fatalf("argv 里没有子命令 diff：%v", argv)
	}
	if cached != sub+1 {
		t.Fatalf("`--cached` 必须**紧贴**子命令 diff：sub=%d cached=%d argv=%v", sub, cached, argv)
	}
	if z < sub {
		t.Fatalf("`-z` 落在子命令之前（%d < %d）⇒ 位置铁律破：%v", z, sub, argv)
	}
	if contains(argv, "-c") {
		t.Fatalf("默认档不该出现 `-c`：%v", argv)
	}
	if err := assertGlobalOptionsFirst(argv); err != nil {
		t.Fatalf("新档 argv 应过 C-3 复查，实得 %v（argv=%v）", err, argv)
	}
}

// ⑮ ★ 旧面回归 —— `FaceDiff`（工作树 vs 索引）argv **逐字不变**（补面三不得动旧面）。
//
// 逐字形态（无 rev / 有 rev 两版）：
//
//	`-C /probe diff --name-only -z`
//	`-C /probe diff --name-only -z HEAD~1`
//
// 变异自证：把新档的 `--cached` 顺手加进 FaceDiff ⇒ 本格红（= 旧调用点被静默改语义）；
// 把 rev 槽接法改掉（如把 rev 插到 `-z` 之前）⇒ 有 rev 那一断红。
func TestFaceDiffArgs_Unchanged(t *testing.T) {
	cfg := config{mode: QuoteVerbatim}
	argv, err := buildArgv("/probe", FaceDiff, &cfg)
	if err != nil {
		t.Fatalf("buildArgv 失败 %v", err)
	}
	want := []string{"-C", "/probe", "diff", "--name-only", "-z"}
	if !equalArgv(argv, want) {
		t.Fatalf("旧 FaceDiff argv 逐字变了：want %v got %v", want, argv)
	}
	if contains(argv, "--cached") {
		t.Fatalf("旧 FaceDiff 不该带 `--cached`：%v", argv)
	}

	rcfg := config{mode: QuoteVerbatim}
	if err := WithRev("HEAD~1")(&rcfg); err != nil {
		t.Fatalf("WithRev 构造失败 %v", err)
	}
	rargv, err := buildArgv("/probe", FaceDiff, &rcfg)
	if err != nil {
		t.Fatalf("buildArgv(rev) 失败 %v", err)
	}
	wantRev := []string{"-C", "/probe", "diff", "--name-only", "-z", "HEAD~1"}
	if !equalArgv(rargv, wantRev) {
		t.Fatalf("旧 FaceDiff(rev) argv 逐字变了：want %v got %v", wantRev, rargv)
	}

	// 两面必须**可区分**（A2 要能据 argv 分辨「索引 vs HEAD」与「工作树 vs 索引」）。
	cargv, err := buildArgv("/probe", FaceDiffCached, &config{mode: QuoteVerbatim})
	if err != nil {
		t.Fatalf("buildArgv(FaceDiffCached) 失败 %v", err)
	}
	if equalArgv(argv, cargv) {
		t.Fatalf("旧面与新档 argv 完全相同 ⇒ 无法据 argv 区分两面：%v", argv)
	}
}

// ⑯ ★ A1 补面三 —— 新档与 WithPrefix **共存**三态（pathspec 后缀与 FaceTracked/FaceOthers 同形）。
//
// 逐字形态（root=/probe）：
//
//	零段  `-C /probe diff --cached --name-only -z`                             ← 不许出现 `--`
//	一段  `-C /probe diff --cached --name-only -z -- core/`
//	N段   `-C /probe diff --cached --name-only -z -- core/ scripts/ 中文 目录/` ← `--` 恰 1 次、按调用序
//
// 变异自证：新档 args 里丢掉 pathspecSuffix ⇒ 一段与 N 段两断红；
// 把 pathspec 拼到 `-z` 之前 ⇒ 零段/一段逐字断红；把空段检查删掉 ⇒ 夹空串一断红。
func TestFaceDiffCached_WithPrefix(t *testing.T) {
	zero := config{mode: QuoteVerbatim}
	zargv, err := buildArgv("/probe", FaceDiffCached, &zero)
	if err != nil {
		t.Fatalf("零段 buildArgv 失败 %v", err)
	}
	wantZero := []string{"-C", "/probe", "diff", "--cached", "--name-only", "-z"}
	if !equalArgv(zargv, wantZero) {
		t.Fatalf("零段 argv 逐字不等：want %v got %v", wantZero, zargv)
	}
	if contains(zargv, "--") {
		t.Fatalf("零段不该出现 `--`（与旧行为逐字相同）：%v", zargv)
	}

	one := config{mode: QuoteVerbatim}
	if err := WithPrefix("core/")(&one); err != nil {
		t.Fatalf("一段 WithPrefix 构造失败 %v", err)
	}
	oargv, err := buildArgv("/probe", FaceDiffCached, &one)
	if err != nil {
		t.Fatalf("一段 buildArgv 失败 %v", err)
	}
	wantOne := []string{"-C", "/probe", "diff", "--cached", "--name-only", "-z", "--", "core/"}
	if !equalArgv(oargv, wantOne) {
		t.Fatalf("一段 argv 逐字不等：want %v got %v", wantOne, oargv)
	}

	many := []string{"core/", "scripts/", "中文 目录/"}
	mcfg := config{mode: QuoteVerbatim}
	if err := WithPrefix(many[0], many[1:]...)(&mcfg); err != nil {
		t.Fatalf("N 段 WithPrefix 构造失败 %v", err)
	}
	margv, err := buildArgv("/probe", FaceDiffCached, &mcfg)
	if err != nil {
		t.Fatalf("N 段 buildArgv 失败 %v", err)
	}
	wantMany := []string{"-C", "/probe", "diff", "--cached", "--name-only", "-z", "--", "core/", "scripts/", "中文 目录/"}
	if !equalArgv(margv, wantMany) {
		t.Fatalf("N 段 argv 逐字不等：want %v got %v", wantMany, margv)
	}
	if n := countArg(margv, "--"); n != 1 {
		t.Fatalf("`--` 应恰出现 1 次，实得 %d 次：%v", n, margv)
	}
	if err := assertGlobalOptionsFirst(margv); err != nil {
		t.Fatalf("N 段 argv 应过 C-3 复查，实得 %v（argv=%v）", err, margv)
	}

	// 空段仍硬失败（与 WithPrefix 一族同口径：不许静默跳过 —— 空 pathspec 会被 git 当整仓）。
	if err := WithPrefix("core/", "")(&config{mode: QuoteVerbatim}); err == nil {
		t.Fatalf("N 段里夹空串必须报错")
	}
}

// ⑰ ★ 牙齿格 —— 新档**不吃** WithRev：`git diff --cached <rev>` 的另一义是「索引 vs <rev>」
// ⇒ 若静默吃下 rev，调用点会拿到「索引 vs HEAD」而以为拿到「索引 vs <rev>」（静默混义）。
//
// 变异自证：把新档 args 改成照 FaceDiff 的槽接 rev（`if cfg.rev != "" { a = append(a, cfg.rev) }`）
// ⇒ 本格红（err==nil 且 argv 尾多一段）；把 `List` 的错误分支去掉 ⇒ 同样红。
func TestList_DiffCachedFace_NoRevCrosstalk(t *testing.T) {
	defer fakeGit(t, []byte("core/a.go\x00"))()
	if _, err := List("/probe", FaceDiffCached, WithRev("HEAD~1")); err == nil {
		t.Fatalf("新档给了 WithRev 必须硬失败（不许静默混义）")
	}
	// 负控成对：不给 rev 时同一面必须放行（证本格判的是 rev、不是本面本身）。
	got, err := List("/probe", FaceDiffCached)
	if err != nil {
		t.Fatalf("不给 rev 应放行，实得 %v", err)
	}
	if len(got) != 1 || got[0].String() != "core/a.go" {
		t.Fatalf("应得 1 件 core/a.go，实得 %d 件：%q", len(got), got)
	}
}

// ⑱ ★ A1 补面四（pathspec 判据）—— WithErrorUnmatch 的三态兼容（结构断言，不喂假输出、不跑 git）。
//
// 逐字形态（root=/probe）：
//
//	① 不传新选项    `-C /probe ls-files -z`   /  `-C /probe ls-files --others --exclude-standard -z`
//	                ← 与现读 argv **逐字相同**（旧调用点零回归）
//	② 传 + 有前缀    `-C /probe ls-files -z --error-unmatch -- core/ scripts/ 中文 目录/`
//	                ← `--error-unmatch` 与 `--` **共存**，位次 = `--` 之前、子命令之后
//	③ 传 + 无前缀    `-C /probe ls-files -z --error-unmatch`
//	                ← 探针仓实测：rc=0、照常列全件（故**不**硬失败：本面语法上不需要前缀）
//
// 位次铁律：落到 `--` **之后**就成 pathspec（探针仓实测 `ls-files -z -- --error-unmatch`
// ⇒ rc=0、0 字节 = 静默失效）⇒ 恒在 pathspecSuffix 之前拼。
//
// 变异自证：errorUnmatchTail 拼到 pathspecSuffix **之后** ⇒ ② 的位次断红；
// 把「零代价早退」删掉（恒拼 `--error-unmatch`）⇒ ① 的逐字断红；
// 把 `--error-unmatch` 挪到子命令之前 ⇒ 位次断红。
func TestWithErrorUnmatch_ThreeStates(t *testing.T) {
	strict := func(extra ...Option) *config {
		c := config{mode: QuoteVerbatim}
		if err := WithErrorUnmatch()(&c); err != nil {
			t.Fatalf("WithErrorUnmatch 构造失败 %v", err)
		}
		for _, o := range extra {
			if err := o(&c); err != nil {
				t.Fatalf("选项构造失败 %v", err)
			}
		}
		return &c
	}

	// ① 不传新选项 ⇒ 与现读 argv 逐字相同（两面都判）。
	wantZero := map[Face][]string{
		FaceTracked: {"-C", "/probe", "ls-files", "-z"},
		FaceOthers:  {"-C", "/probe", "ls-files", "--others", "--exclude-standard", "-z"},
	}
	for _, face := range []Face{FaceTracked, FaceOthers} {
		argv, err := buildArgv("/probe", face, &config{mode: QuoteVerbatim})
		if err != nil {
			t.Fatalf("%s 零态 buildArgv 失败 %v", face, err)
		}
		if !equalArgv(argv, wantZero[face]) {
			t.Fatalf("%s 零态 argv 逐字不等（旧调用点被静默改 argv）：want %v got %v", face, wantZero[face], argv)
		}
		if contains(argv, "--error-unmatch") {
			t.Fatalf("%s 零态不该出现 `--error-unmatch`：%v", face, argv)
		}
	}

	// ② 传 + 有前缀（含 N 段）：与 `--` 共存，位次正确。
	many := []string{"core/", "scripts/", "中文 目录/"}
	cases := []struct {
		face Face
		argv []string
	}{
		{FaceTracked, []string{"-C", "/probe", "ls-files", "-z", "--error-unmatch", "--", "core/", "scripts/", "中文 目录/"}},
		{FaceOthers, []string{"-C", "/probe", "ls-files", "--others", "--exclude-standard", "-z", "--error-unmatch", "--", "core/", "scripts/", "中文 目录/"}},
	}
	for _, c := range cases {
		argv, err := buildArgv("/probe", c.face, strict(WithPrefix(many[0], many[1:]...)))
		if err != nil {
			t.Fatalf("%s 严档 buildArgv 失败 %v", c.face, err)
		}
		if !equalArgv(argv, c.argv) {
			t.Fatalf("%s 严档 argv 逐字不等：want %v got %v", c.face, c.argv, argv)
		}
		sub, eu, dd, z := indexArg(argv, "ls-files"), indexArg(argv, "--error-unmatch"), indexArg(argv, "--"), indexArg(argv, "-z")
		if eu != dd-1 {
			t.Fatalf("%s：`--error-unmatch` 必须**紧贴 `--` 之前**（eu=%d dd=%d）：%v", c.face, eu, dd, argv)
		}
		if eu < sub || eu < z {
			t.Fatalf("%s：`--error-unmatch` 落在子命令之前（eu=%d sub=%d z=%d）：%v", c.face, eu, sub, z, argv)
		}
		if n := countArg(argv, "--error-unmatch"); n != 1 {
			t.Fatalf("%s：`--error-unmatch` 应恰 1 次，实得 %d 次：%v", c.face, n, argv)
		}
		if n := countArg(argv, "--"); n != 1 {
			t.Fatalf("%s：`--` 应恰 1 次，实得 %d 次：%v", c.face, n, argv)
		}
		if err := assertGlobalOptionsFirst(argv); err != nil {
			t.Fatalf("%s 严档 argv 应过 C-3 复查，实得 %v（argv=%v）", c.face, err, argv)
		}
	}

	// ③ 传 + 无前缀：也合法（不硬失败）—— 探针仓实测该形态 rc=0、照常列全件。
	noPrefix := []struct {
		face Face
		argv []string
	}{
		{FaceTracked, []string{"-C", "/probe", "ls-files", "-z", "--error-unmatch"}},
		{FaceOthers, []string{"-C", "/probe", "ls-files", "--others", "--exclude-standard", "-z", "--error-unmatch"}},
	}
	for _, c := range noPrefix {
		argv, err := buildArgv("/probe", c.face, strict())
		if err != nil {
			t.Fatalf("%s 严档·无前缀不该硬失败（探针仓实测合法），实得 %v", c.face, err)
		}
		if !equalArgv(argv, c.argv) {
			t.Fatalf("%s 严档·无前缀 argv 逐字不等：want %v got %v", c.face, c.argv, argv)
		}
		if contains(argv, "--") {
			t.Fatalf("%s 严档·无前缀不该出现 `--`：%v", c.face, argv)
		}
		if indexArg(argv, "--error-unmatch") != len(argv)-1 {
			t.Fatalf("%s 严档·无前缀：`--error-unmatch` 应在 argv 末尾：%v", c.face, argv)
		}
		if err := assertGlobalOptionsFirst(argv); err != nil {
			t.Fatalf("%s 严档·无前缀 argv 应过 C-3 复查，实得 %v", c.face, err)
		}
	}
}

// ⑲ ★ 面名单闸 —— 只有 ls-files 系列两面吃 `--error-unmatch`。
//
// 判据是「git 该子命令有没有这个选项」（ls-files 有：任一 <file> 不在索引里 ⇒ 退 1；
// ls-tree / diff / status / grep 没有）⇒ 其余面（含未知面值）给了一律**硬失败**。
// ★ 禁静默丢：静默丢弃会让调用点以为已开严档，而实际仍是「不存在的 pathspec ⇒ 静默 rc=0」的面。
//
// 变异自证：把 args 顶部的入口闸去掉（其余面照走）⇒ 本格五个面全红；
// 把 acceptsErrorUnmatch 改成 `return true` ⇒ 同样红。
func TestWithErrorUnmatch_FaceAllowList(t *testing.T) {
	defer fakeGit(t, []byte("core/a.go\x00"))()

	// 吃它的两面：放行，且判据面照常出件。
	for _, face := range []Face{FaceTracked, FaceOthers} {
		got, err := List("/probe", face, WithErrorUnmatch(), WithPrefix("core/"))
		if err != nil {
			t.Fatalf("%s 应吃 `--error-unmatch`，实得 %v", face, err)
		}
		if len(got) != 1 || got[0].String() != "core/a.go" {
			t.Fatalf("%s 应得 1 件 core/a.go，实得 %d 件：%q", face, len(got), got)
		}
	}

	// 其余五面：硬失败（List 层 + buildArgv 层各判一次）。
	bad := []struct {
		face Face
		opt  []Option
	}{
		{FaceTree, []Option{WithRev("HEAD")}},
		{FaceDiff, nil},
		{FaceDiffCached, nil},
		{FaceStatus, nil},
		{FaceGrep, []Option{WithPattern("TODO")}},
		{Face(99), nil}, // 未知面值：入口闸必须同样兜住
	}
	for _, c := range bad {
		opt := append([]Option{WithErrorUnmatch()}, c.opt...)
		got, err := List("/probe", c.face, opt...)
		if err == nil {
			t.Fatalf("%s 面不吃 `--error-unmatch` ⇒ 必须硬失败（不许静默丢），实得 entries=%q", c.face, got)
		}
		if len(got) != 0 {
			t.Fatalf("%s 硬失败不许返回半成品件名：%q", c.face, got)
		}
		cfg := config{mode: QuoteVerbatim}
		if err := WithErrorUnmatch()(&cfg); err != nil {
			t.Fatalf("WithErrorUnmatch 构造失败 %v", err)
		}
		if _, err := buildArgv("/probe", c.face, &cfg); err == nil {
			t.Fatalf("%s 面 buildArgv 必须报错（入口闸在拼 argv 之前）", c.face)
		}
	}

	// 负控成对：同一批面**不**给该选项时，其中本来就不缺前置条件的几面必须照常放行
	//（证本格判的是「这个选项能不能吃」，不是「这些面本身坏了」）。
	for _, face := range []Face{FaceDiff, FaceDiffCached, FaceStatus} {
		if _, err := List("/probe", face); err != nil {
			t.Fatalf("%s 不给 WithErrorUnmatch 时应放行，实得 %v", face, err)
		}
	}
}

// ⑳ ★ 牙齿格 —— 「pathspec 不存在」是**可判**的（C-11），且判据**不解析错误文本**。
//
// 判据链：execGit 的 error 已 `%w` 包过 *exec.ExitError ⇒ List 里 errors.As 取
// ExitCode()==1 ⇒ 翻成 ErrPathspecNotFound（原始 error 仍在链上，细节不被吞）。
//
// ★ 这里用的 *exec.ExitError 是**真的**：os.ProcessState 不可内建（零值取 ExitCode() 会 panic），
//
//	只能来自真跑一个退出码确定的子进程 ⇒ 用测试二进制自己做助手（TestZHelperProcess_ExitCode），
//	**不跑 git**。
//
// 变异自证：把 `code == kExitPathspecUnmatched` 改成「只要 ok」（任何非零都算）⇒ 负控②红；
// 把 cfg.strict 那道闸去掉（未开严档也翻）⇒ 负控①红；把哨兵换掉 / 不再 `%w` ⇒ 正例红。
func TestList_PathspecMissing_Judgeable(t *testing.T) {
	// 正例：严档 + git rc=1 ⇒ 可判为「pathspec 不存在」。
	defer fakeGitErr(t, fakeExitErr(t, 1))()
	got, err := List("/probe", FaceTracked, WithErrorUnmatch(), WithPrefix("core/nope.go"))
	if err == nil {
		t.Fatalf("严档下 rc=1 必须硬失败（不许静默 0 件）")
	}
	if !IsPathspecMissing(err) {
		t.Fatalf("应判为「pathspec 不存在」，实得 %v（%T）", err, err)
	}
	if !errors.Is(err, ErrPathspecNotFound) {
		t.Fatalf("errors.Is(err, ErrPathspecNotFound) 应为真：%v", err)
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 1 {
		t.Fatalf("原始 *exec.ExitError 必须仍在错误链上且 ExitCode=1，实得 %T: %v", err, err)
	}
	var escaped *EscapedNameError
	if errors.As(err, &escaped) {
		t.Fatalf("这不是 C-5 自检命中，不该翻成 *EscapedNameError：%v", err)
	}
	if len(got) != 0 {
		t.Fatalf("硬失败不许返回半成品件名，实得 %q", got)
	}

	// 负控①：**未开严档**时同一错误不许被翻成「pathspec 不存在」
	//（未开严档那一档，不存在的 pathspec 是**静默 rc=0 / 0 件**，git 根本不会给 rc=1）。
	restore1 := fakeGitErr(t, fakeExitErr(t, 1))
	_, errOff := List("/probe", FaceTracked, WithPrefix("core/nope.go"))
	restore1()
	if IsPathspecMissing(errOff) {
		t.Fatalf("未开严档不许产「pathspec 不存在」判据：%v", errOff)
	}

	// 负控②：严档下**非 1** 的退出码不许当「pathspec 不存在」（只认 1，宁窄不宽）。
	restore2 := fakeGitErr(t, fakeExitErr(t, 128))
	_, errOther := List("/probe", FaceTracked, WithErrorUnmatch(), WithPrefix("core/nope.go"))
	restore2()
	if IsPathspecMissing(errOther) {
		t.Fatalf("rc=128（非 pathspec 类失败）不许报「pathspec 不存在」：%v", errOther)
	}
	if errOther == nil {
		t.Fatalf("rc=128 仍必须是错误（只是不标 pathspec 不存在）")
	}
}

// fakeGitErr —— 注入返回指定 error 的假运行器（⑳ 专用）。
func fakeGitErr(t *testing.T, err error) func() {
	t.Helper()
	old := gitRunnerImpl
	gitRunnerImpl = func(argv []string) ([]byte, error) { return nil, err }
	return func() { gitRunnerImpl = old }
}

// fakeExitErr —— 造一个**真的** *exec.ExitError（退出码 = code），并照 execGit 的包法
// （`%w` + stderr 文本）回喂 ⇒ 错误链深度与生产一致。
func fakeExitErr(t *testing.T, code int) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestZHelperProcess_ExitCode$")
	cmd.Env = append(os.Environ(), "GITPATHS_HELPER_EXIT="+strconv.Itoa(code))
	_, err := cmd.Output()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("助手子进程未给出 *exec.ExitError（code=%d）：%v", code, err)
	}
	if ee.ExitCode() != code {
		t.Fatalf("助手退出码不符：want %d got %d", code, ee.ExitCode())
	}
	return fmt.Errorf("%w（stderr: %s）", err, "helper")
}

// TestZHelperProcess_ExitCode —— ⑳ 的子进程助手：只在被父进程按名拉起且
// GITPATHS_HELPER_EXIT 有值时退出指定码；正常跑测试时它什么都不做。
// ★ 为什么要它：*exec.ExitError 的 os.ProcessState 不可内建（零值取 ExitCode() 会 panic），
//
//	而 C-11 的判据正是 errors.As(*exec.ExitError) + ExitCode()。
func TestZHelperProcess_ExitCode(t *testing.T) {
	v := os.Getenv("GITPATHS_HELPER_EXIT")
	if v == "" {
		return
	}
	code, convErr := strconv.Atoi(v)
	if convErr != nil {
		os.Exit(97)
	}
	os.Exit(code)
}

func equalArgv(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func countArg(ss []string, want string) int {
	n := 0
	for _, s := range ss {
		if s == want {
			n++
		}
	}
	return n
}

func indexArg(ss []string, want string) int {
	for i, s := range ss {
		if s == want {
			return i
		}
	}
	return -1
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// ㉑ ★ A1 补面五·甲/乙（status 面）—— **零选项** argv 与现读**逐字相同**（旧调用点零回归）。
//
// 格子类型：**结构断言**（直接断 buildArgv 的切片，不喂假输出、不跑 git）。
//
//	`-C /probe status --porcelain -z`
//
// 变异自证：把 untrackedFilesTail / pathspecSuffix 从本面 args 里改成「恒拼」（零态也拼 `--`）
// ⇒ 本格红；把 `-z` 挪到子命令之前 / 拿掉 ⇒ 本格红。
func TestFaceStatusArgs_ZeroStateUnchanged(t *testing.T) {
	argv, err := buildArgv("/probe", FaceStatus, &config{mode: QuoteVerbatim})
	if err != nil {
		t.Fatalf("零态 buildArgv 失败 %v", err)
	}
	want := []string{"-C", "/probe", "status", "--porcelain", "-z"}
	if !equalArgv(argv, want) {
		t.Fatalf("status 零态 argv 逐字变了：want %v got %v", want, argv)
	}
	if contains(argv, "--") {
		t.Fatalf("零态不该出现 `--`（零段 pathspec 不进 argv）：%v", argv)
	}
	for _, a := range argv {
		if strings.HasPrefix(a, kUntrackedFiles) {
			t.Fatalf("零态不该出现 `--untracked-files*`（零值档 = 不传）：%v", argv)
		}
	}
	if z, sub := indexArg(argv, "-z"), indexArg(argv, "status"); z < sub || sub < 0 {
		t.Fatalf("`-z` 必在子命令之后（z=%d sub=%d）：%v", z, sub, argv)
	}
	if err := assertGlobalOptionsFirst(argv); err != nil {
		t.Fatalf("零态 argv 应过 C-3 复查，实得 %v", err)
	}
	// 该面仍**不吃** `--error-unmatch`（C-11 面名单闸在 ⑲ 已判；此处只判 status 面不自称严档）。
	if _, err := List("/probe", FaceStatus, WithErrorUnmatch()); err == nil {
		t.Fatal("status 面不吃 `--error-unmatch` ⇒ 必须硬失败")
	}
}

// ㉒ ★ A1 补面五·甲 —— FaceStatus 与 WithPrefix **共存**三态（与 FaceTracked/FaceOthers/FaceDiffCached 同形）。
//
// 逐字形态（root=/probe）：
//
//	零段  `-C /probe status --porcelain -z`                          ← 不许出现 `--`
//	一段  `-C /probe status --porcelain -z -- core/`
//	N段   `-C /probe status --porcelain -z -- core/ scripts/ 中文 目录/` ← `--` 恰 1 次、按调用序
//
// 变异自证：把 pathspecSuffix 从本面 args 里去掉 ⇒ 一段与 N 段两断红；把它拼到 `-z` 之前 ⇒ 逐字断红。
func TestFaceStatus_WithPrefix(t *testing.T) {
	one := config{mode: QuoteVerbatim}
	if err := WithPrefix("core/")(&one); err != nil {
		t.Fatalf("一段 WithPrefix 构造失败 %v", err)
	}
	oargv, err := buildArgv("/probe", FaceStatus, &one)
	if err != nil {
		t.Fatalf("一段 buildArgv 失败 %v", err)
	}
	wantOne := []string{"-C", "/probe", "status", "--porcelain", "-z", "--", "core/"}
	if !equalArgv(oargv, wantOne) {
		t.Fatalf("一段 argv 逐字不等：want %v got %v", wantOne, oargv)
	}

	many := []string{"core/", "scripts/", "中文 目录/"}
	mcfg := config{mode: QuoteVerbatim}
	if err := WithPrefix(many[0], many[1:]...)(&mcfg); err != nil {
		t.Fatalf("N 段 WithPrefix 构造失败 %v", err)
	}
	margv, err := buildArgv("/probe", FaceStatus, &mcfg)
	if err != nil {
		t.Fatalf("N 段 buildArgv 失败 %v", err)
	}
	wantMany := []string{"-C", "/probe", "status", "--porcelain", "-z", "--", "core/", "scripts/", "中文 目录/"}
	if !equalArgv(margv, wantMany) {
		t.Fatalf("N 段 argv 逐字不等：want %v got %v", wantMany, margv)
	}
	if n := countArg(margv, "--"); n != 1 {
		t.Fatalf("`--` 应恰 1 次，实得 %d 次：%v", n, margv)
	}
	if err := assertGlobalOptionsFirst(margv); err != nil {
		t.Fatalf("N 段 argv 应过 C-3 复查，实得 %v（argv=%v）", err, margv)
	}

	// 运行时面：喂 status 的协议段（` M <名>`）⇒ 出口仍是**件名**（XY 不流出），且只出 pathspec 命中那一件。
	seen, restore := fakeGitCapture(t, []byte(" M \u4e2d\u6587\u540d.fsmd\x00"))
	defer restore()
	got, err := List("/probe", FaceStatus, WithPrefix("中文名.fsmd"))
	if err != nil {
		t.Fatalf("实跑（假运行器）不该报错，实得 %v", err)
	}
	if len(got) != 1 || got[0].String() != "中文名.fsmd" {
		t.Fatalf("应得 1 件 `中文名.fsmd`（XY 已按协议剥掉），实得 %d 件：%q", len(got), got)
	}
	if (*seen)[len(*seen)-1] != "中文名.fsmd" || (*seen)[len(*seen)-2] != "--" {
		t.Fatalf("pathspec 必须落在 `--` 之后且为末段：%v", *seen)
	}
}

// ㉓ ★ A1 补面五·乙 —— WithUntrackedFiles 三态（不传 / all / no）+ 位次 + 与 pathspec 共存。
//
// 逐字形态（root=/probe）：
//
//	不传      `-C /probe status --porcelain -z`
//	all       `-C /probe status --porcelain -z --untracked-files=all`
//	no        `-C /probe status --porcelain -z --untracked-files=no`
//	all+前缀  `-C /probe status --porcelain -z --untracked-files=all -- core/`
//
// 位次铁律：旋钮恒在 `pathspecSuffix` **之前**（探针仓实测：落到 `--` 之后 ⇒ git 当 pathspec
// ⇒ rc=0、**0 字节**（`-- --untracked-files=all`）或**旋钮被静默丢掉**（`-- <前缀> --untracked-files=no` 仍照旧列未跟踪件））。
//
// 变异自证：把 untrackedFilesTail 拼到 pathspecSuffix **之后** ⇒ 位次断红；把「零值早退」删掉
// ⇒ 三态①的逐字断红；把 `=` 拼成空格（`--untracked-files all`）⇒ 逐字断红。
func TestWithUntrackedFiles_ThreeStates(t *testing.T) {
	// ① 不传（零值档）⇒ argv 里不出现该旋钮（与 ㉑ 同断，此处只判本选项的零代价）。
	zcfg := config{mode: QuoteVerbatim}
	if err := WithUntrackedFiles(UntrackedFilesDefault)(&zcfg); err != nil {
		t.Fatalf("零值档不该报错，实得 %v", err)
	}
	zargv, err := buildArgv("/probe", FaceStatus, &zcfg)
	if err != nil {
		t.Fatalf("零值档 buildArgv 失败 %v", err)
	}
	if !equalArgv(zargv, []string{"-C", "/probe", "status", "--porcelain", "-z"}) {
		t.Fatalf("零值档 argv 应逐字等于现读：got %v", zargv)
	}

	// ②③ 显式档：all / no 各自逐字。
	for _, c := range []struct {
		mode UntrackedFilesMode
		arg  string
	}{
		{UntrackedFilesAll, "--untracked-files=all"},
		{UntrackedFilesNo, "--untracked-files=no"},
		{UntrackedFilesNormal, "--untracked-files=normal"},
	} {
		cfg := config{mode: QuoteVerbatim}
		if err := WithUntrackedFiles(c.mode)(&cfg); err != nil {
			t.Fatalf("%s 构造失败 %v", c.mode, err)
		}
		argv, err := buildArgv("/probe", FaceStatus, &cfg)
		if err != nil {
			t.Fatalf("%s buildArgv 失败 %v", c.mode, err)
		}
		want := []string{"-C", "/probe", "status", "--porcelain", "-z", c.arg}
		if !equalArgv(argv, want) {
			t.Fatalf("%s argv 逐字不等：want %v got %v", c.mode, want, argv)
		}
		if n := countArg(argv, c.arg); n != 1 {
			t.Fatalf("%s 应恰出现 1 次，实得 %d 次：%v", c.mode, n, argv)
		}
		// 位次：子命令之后、argv 末尾（无 pathspec 时）。
		if indexArg(argv, c.arg) != len(argv)-1 || indexArg(argv, c.arg) < indexArg(argv, "status") {
			t.Fatalf("%s 位次不对（须在子命令之后且为末段）：%v", c.mode, argv)
		}
		if err := assertGlobalOptionsFirst(argv); err != nil {
			t.Fatalf("%s argv 应过 C-3 复查，实得 %v", c.mode, err)
		}
	}

	// ④ 与 pathspec 共存：旋钮恒在 `--` **之前**（落到 `--` 之后 ⇒ 探针仓实测静默失效）。
	cfg := config{mode: QuoteVerbatim}
	for _, o := range []Option{WithUntrackedFiles(UntrackedFilesAll), WithPrefix("core/", "中文 目录/")} {
		if err := o(&cfg); err != nil {
			t.Fatalf("选项构造失败 %v", err)
		}
	}
	argv, err := buildArgv("/probe", FaceStatus, &cfg)
	if err != nil {
		t.Fatalf("共存 buildArgv 失败 %v", err)
	}
	want := []string{"-C", "/probe", "status", "--porcelain", "-z", "--untracked-files=all", "--", "core/", "中文 目录/"}
	if !equalArgv(argv, want) {
		t.Fatalf("共存 argv 逐字不等：want %v got %v", want, argv)
	}
	if indexArg(argv, "--untracked-files=all") != indexArg(argv, "--")-1 {
		t.Fatalf("旋钮必须**紧贴 `--` 之前**：%v", argv)
	}
	if n := countArg(argv, "--"); n != 1 {
		t.Fatalf("`--` 应恰 1 次，实得 %d 次：%v", n, argv)
	}

	// 未知档值 ⇒ **构造即硬失败**（C-8：只认四值；不许静默降级成默认档）。
	if err := WithUntrackedFiles(UntrackedFilesMode(99))(&config{mode: QuoteVerbatim}); err == nil {
		t.Fatal("未知 UntrackedFilesMode 必须报错（禁静默降级）")
	}
}

// ㉔ ★ 面名单闸（C-12）—— **只有 status 面**吃 `--untracked-files=<mode>`；其余六面 + 未知面值
// 一律**硬失败**（禁静默丢：静默丢会让调用点以为已调档，实际仍是默认档）。
//
// 并附 **C-9 七类齐守卫**：第八类面值（`Face(7)`/`Face(8)`）必须硬失败 —— 本包出口的返回一律是
// 「件名」；XY 记录面属**另一类面**，未立设计稿前不许出现（禁把协议字母混进件名面）。
//
// 变异自证：把 acceptsUntrackedFiles 改成 `return true` ⇒ 本格六个面全红；
// 把 args 顶部的入口闸去掉 ⇒ 同样红。
func TestWithUntrackedFiles_FaceAllowList(t *testing.T) {
	defer fakeGit(t, []byte(" M core/a.go\x00"))()

	// 吃它的面：放行，且判据面照常出件（XY 已剥）。
	got, err := List("/probe", FaceStatus, WithUntrackedFiles(UntrackedFilesNo), WithPrefix("core/"))
	if err != nil {
		t.Fatalf("status 面应吃该旋钮，实得 %v", err)
	}
	if len(got) != 1 || got[0].String() != "core/a.go" {
		t.Fatalf("status 面应得 1 件 core/a.go，实得 %d 件：%q", len(got), got)
	}

	bad := []struct {
		face Face
		opt  []Option
	}{
		{FaceTracked, nil},
		{FaceTree, []Option{WithRev("HEAD")}},
		{FaceDiff, nil},
		{FaceDiffCached, nil},
		{FaceOthers, nil},
		{FaceGrep, []Option{WithPattern("TODO")}},
		{Face(99), nil}, // 未知面值：入口闸必须同样兜住
	}
	for _, c := range bad {
		opt := append([]Option{WithUntrackedFiles(UntrackedFilesAll)}, c.opt...)
		got, err := List("/probe", c.face, opt...)
		if err == nil {
			t.Fatalf("%s 面不吃 `--untracked-files=<mode>` ⇒ 必须硬失败（不许静默丢），实得 entries=%q", c.face, got)
		}
		if len(got) != 0 {
			t.Fatalf("%s 硬失败不许返回半成品件名：%q", c.face, got)
		}
		cfg := config{mode: QuoteVerbatim}
		if err := WithUntrackedFiles(UntrackedFilesAll)(&cfg); err != nil {
			t.Fatalf("构造失败 %v", err)
		}
		if _, err := buildArgv("/probe", c.face, &cfg); err == nil {
			t.Fatalf("%s 面 buildArgv 必须报错（入口闸在拼 argv 之前）", c.face)
		}
	}

	// 负控成对：同一批面**不**给该旋钮时，本来不缺前置条件的必须照常放行。
	for _, face := range []Face{FaceTracked, FaceDiff, FaceDiffCached, FaceStatus} {
		if _, err := List("/probe", face); err != nil {
			t.Fatalf("%s 不给 WithUntrackedFiles 时应放行，实得 %v", face, err)
		}
	}

	// C-9 七类齐守卫：第八类面值必须硬失败（含状态码/记录面若被偷偷加进来的情形）。
	for _, f := range []Face{Face(7), Face(8)} {
		if _, err := List("/probe", f); err == nil {
			t.Fatalf("面值 %v 不该存在（C-9 七类齐）⇒ 必须硬失败", f)
		}
		if _, err := buildArgv("/probe", f, &config{mode: QuoteVerbatim}); err == nil {
			t.Fatalf("面值 %v 的 buildArgv 必须硬失败", f)
		}
	}
}
