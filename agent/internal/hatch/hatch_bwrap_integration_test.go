// hatch_bwrap_integration_test.go —— **真 bwrap 的端到端配方验证**（本机没有 bwrap 就跳过）。
//
// 为什么必须有它（也是「本机（macOS，无 bwrap）能不能合理模拟这套 argv」的答案）：
// `BuildBwrapArgv` 的纯函数用例只能钉住**字符串**（有没有 `--dir /models`、绑定落点对不对、顺序
// 稳不稳），钉不住「bwrap 到底认不认这套 argv」——而缺陷 12 的收口恰恰卡在 bwrap 的**运行时行为**上：
// /models 一旦是被挂进来的**只读目录**，再往里绑 `/models/<基名>` 就是
// `bwrap: Can't create file …: Read-only file system`（整卵 1–3ms 秒死）。这条只有在真 bwrap 上跑
// 一次才算证。
//
// 本用例做三件事（都不需要真权重/真引擎，几个字节的临时文件即可）：
//
//	① 让 bwrap 真的按这份 argv 起一个空间（入口换成 `/bin/sh -c` 的自证脚本）⇒ rc=0 本身就证明
//	   配方自洽（若 /models 是只读挂载，rc≠0 且 stderr 就是上面那句）；
//	② 核空间内 `/models` 的**内容**：只有声明的那两个文件，同目录的「别的模型」不在（缺陷 12 的
//	   真实诉求：不是「能不能读」，是「别的模型根本不在」）；
//	③ 把空间内的 `/proc/self/mountinfo` 原文喂给 `CheckEnclosure`：每个声明文件各有一条只读挂载、
//	   **没有**落点是 /models 本身（判据与配方同源，纯函数与真机不再各说各话）。
//
// macOS 上 `go test ./internal/hatch/` 自动跳过（没有 bwrap）；**真机（X3）上跑它才是这套 argv 的实证**：
//
//	GOOS=linux go test -v -run RealBwrap ./internal/hatch/
package hatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 自证脚本的三个分节标记（用 sh 的 echo 打出来，不依赖 grep/coreutils）。
const (
	markLS  = "===LS==="
	markMNT = "===MNT==="
	markEnd = "===END==="
)

// TestBuildBwrapArgv_RealBwrapRecipe 真 bwrap 跑一遍逐文件权重配方（无 bwrap ⇒ 跳过，如实记一行）。
func TestBuildBwrapArgv_RealBwrapRecipe(t *testing.T) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("本机没有 bwrap（开发机的常态）⇒ 跳过：这份 argv 的**真机**实证在 X3 上跑本用例")
	}
	// 临时权重目录：两个「权重文件」+ 一个同目录的**别的模型**（后者绝不许进空间）
	dir := t.TempDir()
	mk := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("gguf-placeholder\n"), 0o644); err != nil {
			t.Fatalf("造临时权重 %s 失败：%v", p, err)
		}
		return p
	}
	wGguf := mk("our-model-Q4_K_M.gguf")
	wMMProj := mk("our-model-mmproj-f16.gguf")
	other := mk("Qwen2.5-VL-32B-somebody-elses.gguf") // 同目录的别的模型：不许出现在空间里

	s := goodSpec()
	s.WeightPath = dir // 出证/日志用
	s.WeightFiles = []string{wGguf, wMMProj}
	s.ExtraROBinds = []string{
		wGguf + ":" + spaceWeightLanding(wGguf),
		wMMProj + ":" + spaceWeightLanding(wMMProj),
	}
	// 入口换成自证脚本（不需要真引擎）；设备只挂 /dev/null（**不依赖**本机有没有 GPU 节点），
	// 环境为空（入口就是 /bin/sh -c <脚本>，不加包装层）。
	s.EnginePathInSpace = "/bin/sh"
	s.EngineRoots = nil
	s.Devices = []string{"/dev/null"}
	s.Env = nil
	s.EngineArgs = []string{"-c", "echo " + markLS + "; ls /models; echo " + markMNT + "; cat /proc/self/mountinfo; echo " + markEnd}

	argv, err := BuildBwrapArgv(s)
	if err != nil {
		t.Fatalf("构造 argv 失败：%v", err)
	}
	out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
	if err != nil {
		// bwrap 存在但因本机不具备 userns/权限而失败 ⇒ **读不到 ≠ 配方不符**（§6.9 同一精神），跳过并留痕。
		low := strings.ToLower(string(out))
		for _, benign := range []string{"permission denied", "no permissions", "user namespace", "can't clone", "operation not permitted"} {
			if strings.Contains(low, benign) {
				t.Skipf("bwrap 在本机起不来（环境限制，不是配方问题）⇒ 跳过：%s", strings.TrimSpace(string(out)))
			}
		}
		t.Fatalf("bwrap 拒绝了这份 argv（rc=%v）——配方自洽性不成立（/models 若被挂成只读目录，这里就是 "+
			"`Can't create file …: Read-only file system`）：\n%s", err, string(out))
	}
	ls, mnt := section(string(out), markLS, markMNT), section(string(out), markMNT, markEnd)
	if !strings.Contains(string(out), markLS) || !strings.Contains(string(out), markMNT) || !strings.Contains(string(out), markEnd) {
		t.Fatalf("自证脚本的输出没抓全（标记缺失）：\n%s", string(out))
	}

	// ① /models 的**内容**：只有声明的那两个文件（同目录的别的模型不在）
	names := map[string]bool{}
	for _, f := range strings.Fields(ls) {
		names[f] = true
	}
	for _, want := range []string{filepath.Base(wGguf), filepath.Base(wMMProj)} {
		if !names[want] {
			t.Errorf("空间内 %s 下应见到声明文件 %s，实测 ls=%q", spaceModelsDir, want, ls)
		}
	}
	if names[filepath.Base(other)] {
		t.Errorf("同目录的**别的模型** %s 不该进空间（缺陷 12 的真实诉求），实测 ls=%q", filepath.Base(other), ls)
	}

	// ② 空间内 mountinfo：每个声明文件各一条只读挂载，且没有落点是 /models 本身
	for _, base := range []string{filepath.Base(wGguf), filepath.Base(wMMProj)} {
		land := spaceWeightLanding(filepath.Join(dir, base))
		line := lineWithMountPoint(mnt, land)
		if line == "" {
			t.Errorf("空间内 mountinfo 里应有落点 %s 的挂载，实测：\n%s", land, mnt)
			continue
		}
		if !optsHave(strings.Fields(line)[5], "ro") {
			t.Errorf("落点 %s 应是只读挂载，实测：%s", land, line)
		}
	}
	if line := lineWithMountPoint(mnt, spaceModelsDir); line != "" {
		t.Errorf("%s 本身不该是挂载点（逐文件配方：它只是空目录），实测：%s", spaceModelsDir, line)
	}

	// ③ 判据同源：拿**空间内**的真文本喂 `CheckEnclosure`
	//    - 不带声明（真机核验路径的形态）：结构那半条必须过（至少一个逐文件只读落点 + 无整目录挂载）；
	//    - 带上声明清单：两个落点都见到 ⇒ 权重那一项也必须过。
	//    注意这里只断言**权重那一项**（不判 Enclosed()）：这枚测试卵没绑 /work，故 /work 那一条
	//    （必须见到且可写）本来就该不符 —— 与权重无关，别把两件事混在一起。
	for _, c := range []struct {
		name     string
		declared []string
	}{{"结构形态（不带声明）", nil}, {"带声明清单", s.WeightFiles}} {
		t.Run(c.name, func(t *testing.T) {
			rep := CheckEnclosure(mnt, c.declared)
			if rep.ModelsDirMounted {
				t.Fatalf("不该见到落在 %s 本身的挂载，实测报告=%s", spaceModelsDir, rep)
			}
			if !rep.ModelsReadOnly {
				t.Fatalf("空间内真文本应让权重那一项成立，实得 %s（不符项=%v）", rep, rep.Failures())
			}
			for _, base := range []string{filepath.Base(wGguf), filepath.Base(wMMProj)} {
				land := spaceWeightLanding(filepath.Join(dir, base))
				if !containsAll(rep.ModelsFiles, land) {
					t.Errorf("判据应见到逐文件只读落点 %s，实得 %v", land, rep.ModelsFiles)
				}
			}
			if len(c.declared) > 0 {
				if missing := rep.MissingWeightFiles(); len(missing) != 0 {
					t.Errorf("声明的两个文件都在空间里，不该报缺失，实得 %v", missing)
				}
				if !containsAll(rep.WeightsChecked, spaceWeightLanding(wGguf), spaceWeightLanding(wMMProj)) {
					t.Errorf("判据核过的落点应是声明清单换算出的两个，实得 %v", rep.WeightsChecked)
				}
			}
		})
	}
	// 顺带核一条配方事实：/tmp 在空间里是 tmpfs（判据的另一项，纯函数与真机一致）
	if rep := CheckEnclosure(mnt, nil); !rep.TmpIsTmpfs {
		t.Errorf("空间内 /tmp 应是 tmpfs（配方恒 `--tmpfs /tmp`），实测：\n%s", mnt)
	}
}

// section 取 text 里 from 与 to 两个标记**之间**的那一段（找不全返回空串）。
func section(text, from, to string) string {
	i := strings.Index(text, from)
	if i < 0 {
		return ""
	}
	rest := text[i+len(from):]
	if j := strings.Index(rest, to); j >= 0 {
		return rest[:j]
	}
	return rest
}

// lineWithMountPoint 在 mountinfo 文本里找挂载点**正好**是 want 的那一行（找不到返回空串）。
func lineWithMountPoint(mountinfo, want string) string {
	for _, line := range strings.Split(mountinfo, "\n") {
		f := strings.Fields(line)
		if len(f) >= 6 && f[4] == want {
			return line
		}
	}
	return ""
}

// containsAll 判断 got 里是否含 wants 的每一项（顺序无关）。
func containsAll(got []string, wants ...string) bool {
	set := make(map[string]bool, len(got))
	for _, g := range got {
		set[g] = true
	}
	for _, w := range wants {
		if !set[w] {
			return false
		}
	}
	return true
}
