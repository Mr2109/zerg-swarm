// hatch_test.go —— 孵化器验收（纯函数部分，跨平台可跑；真执行在 X3 上另验）。
//
// 覆盖：声明 fail-closed（缺字段/版本不认/无档案一律拒孵）· bwrap 命令行配方（含 X3 实测
// 三坑）· systemd-run 只负责归属 · 单元名确定性与合法性 · 收卵幂等语义 · 封闭性核验判据。
package hatch

import (
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// goodProfile 一份能过 Validate 的实测档案（calib_runs≥3、字段全正）。
func goodProfile() monitor.EggProfile {
	return monitor.EggProfile{
		WeightSizeGb:         90,
		PeakGttGb:            32.7,
		PeakMemGb:            40,
		LoadSeconds:          120,
		ThroughputTokS:       4.8,
		SuggestedIdleUnloadS: 600,
		SchemaVersion:        1,
		MeasuredAt:           time.Now(),
		Machine:              "x3",
		CalibRuns:            3,
	}
}

func goodSpec() Spec {
	return Spec{
		EggID:             "example-moe-36b",
		SchemaVersion:     registry.EggSchemaVersionCurrent,
		EnginePathInSpace: "/engine/build-k2/bin/llama-server",
		EngineArgs:        []string{"-m", "/models/k2horizon-q4_k_m.gguf", "--port", "58100"},
		EngineRoots:       []string{"/home/g01/llama-k2"},
		WeightPath:        "/data/models/k2",
		Env:               map[string]string{"LD_LIBRARY_PATH": "/engine/build-k2/bin"},
		WorkDir:           "/tmp",
		Profile:           goodProfile(),
	}
}

// 声明校验 fail-closed：任何一项缺/不对都必须拒孵，不许静默补默认值。
func TestSpec_ValidateFailClosed(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Spec)
		want string
	}{
		{"缺 egg_id", func(s *Spec) { s.EggID = "" }, "egg_id"},
		{"版本不认得", func(s *Spec) { s.SchemaVersion = 99 }, "认不得"},
		{"缺引擎路径", func(s *Spec) { s.EnginePathInSpace = "" }, "引擎在空间内的路径"},
		{"引擎路径非绝对", func(s *Spec) { s.EnginePathInSpace = "llama-server" }, "绝对路径"},
		{"缺权重", func(s *Spec) { s.WeightPath = "" }, "权重路径"},
		{"无实测档案", func(s *Spec) { s.Profile = monitor.EggProfile{} }, "实测档案"},
		{"档案标定轮数不足", func(s *Spec) { p := s.Profile; p.CalibRuns = 1; s.Profile = p }, "实测档案"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := goodSpec()
			c.mut(&s)
			err := s.Validate()
			if err == nil {
				t.Fatalf("应拒孵，却通过了校验")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("错误信息应提到 %q，实得 %q", c.want, err.Error())
			}
			if _, err2 := BuildBwrapArgv(s); err2 == nil {
				t.Fatal("命令行构造也必须跟着拒（不许绕过校验出 argv）")
			}
		})
	}
}

// bwrap 配方：X3 实测三坑必须体现（usrmerge 符号链接 / 设备显式绑定 / 只挂自己的权重）。
func TestBuildBwrapArgv_RecipeFromX3Findings(t *testing.T) {
	s := goodSpec()
	argv, err := BuildBwrapArgv(s)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	// ① usrmerge：缺 /lib64 会 execvp sh: No such file or directory（实测两次复现）
	for _, link := range []string{"--symlink usr/bin /bin", "--symlink usr/lib /lib", "--symlink usr/lib64 /lib64", "--symlink usr/sbin /sbin"} {
		if !strings.Contains(joined, link) {
			t.Errorf("缺 usrmerge 链接 %q（X3 实测坑①）", link)
		}
	}
	// ② GPU 必须显式 --dev-bind（默认 --dev 看不到）
	if !strings.Contains(joined, "--dev-bind /dev/kfd /dev/kfd") {
		t.Error("缺 /dev/kfd 显式绑定（X3 实测坑②：默认 --dev 看不到 GPU）")
	}
	if !strings.Contains(joined, "--dev-bind /dev/dri/renderD128 /dev/dri/renderD128") {
		t.Error("缺渲染节点显式绑定（坑②）")
	}
	// ③ 只挂该卵自己的权重 ⇒ /models 只有一个来源；且**宿主**权重树里别的模型路径不得出现
	if !strings.Contains(joined, "--ro-bind /data/models/k2 /models") {
		t.Error("应只把该卵自己的权重挂成 /models")
	}
	hostWeights := 0
	for _, a := range argv {
		if strings.Contains(a, "/data/models/") {
			hostWeights++
			if a != s.WeightPath { // 唯一允许出现的宿主权重路径就是本卵的权重根
				t.Errorf("argv 里出现了别的宿主权重路径 %q（只许挂本卵自己的）", a)
			}
		}
	}
	if hostWeights != 1 {
		t.Errorf("宿主权重路径应恰好出现 1 次（本卵权重根），实得 %d 次", hostWeights)
	}
	// 空间内路径（引擎参数里的 /models/<file>）是预期的，不属于泄漏
	if !strings.Contains(joined, "-m /models/k2horizon-q4_k_m.gguf") {
		t.Error("引擎参数应使用空间内路径 /models/<file>")
	}
	// 进程视图隔离 + 读到的根是新的
	if !strings.Contains(joined, "--unshare-pid") {
		t.Error("缺 --unshare-pid（进程视图隔离）")
	}
	if !strings.Contains(joined, "--proc /proc") || !strings.Contains(joined, "--tmpfs /tmp") {
		t.Error("缺 /proc 或 /tmp tmpfs（新根的两处关键挂法）")
	}
	// 环境变量按卵给（K2 的 ABI 坑靠这里覆盖/清理 LD_LIBRARY_PATH）
	if !strings.Contains(joined, "--setenv LD_LIBRARY_PATH=/engine/build-k2/bin") {
		t.Error("缺按卵环境变量（LD_LIBRARY_PATH 必须由卵声明给）")
	}
	// 入口：-- 之后是空间内引擎路径 + 参数
	seps := 0
	for i, a := range argv {
		if a == "--" {
			seps = i
		}
	}
	if seps == 0 || argv[seps+1] != "/engine/build-k2/bin/llama-server" {
		from := seps - 1
		if from < 0 {
			from = 0
		}
		t.Fatalf("-- 之后应紧跟空间内引擎路径，实得 %v", argv[from:])
	}
}

// 环境变量顺序必须可复现（同一 Spec 两次构造 argv 完全一致）。
func TestBuildBwrapArgv_DeterministicEnvOrder(t *testing.T) {
	s := goodSpec()
	s.Env = map[string]string{"ZED": "2", "ALPHA": "1", "MID": "3"}
	a1, _ := BuildBwrapArgv(s)
	a2, _ := BuildBwrapArgv(s)
	if strings.Join(a1, " ") != strings.Join(a2, " ") {
		t.Fatal("同一 Spec 两次构造 argv 应完全一致（可复现）")
	}
	ia := strings.Index(strings.Join(a1, " "), "ALPHA=1")
	im := strings.Index(strings.Join(a1, " "), "MID=3")
	iz := strings.Index(strings.Join(a1, " "), "ZED=2")
	if !(ia < im && im < iz) {
		t.Fatal("环境变量应按 key 排序（可复现）")
	}
}

// systemd-run 只管归属：slice / 单元名 / Type=exec / 限额；**不得**在它上面挂挂载类选项
// （§6.1 实测：那些选项在用户实例里静默失效）。
func TestBuildSystemdRunArgv_OwnershipOnly(t *testing.T) {
	s := goodSpec()
	s.MemlockBytes = 1 << 30
	unit := UnitName(s.EggID)
	argv, err := BuildSystemdRunArgv(s, unit)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"systemd-run", "--user", "--unit=" + unit, "--slice=" + SliceName} {
		if !strings.Contains(joined, want) {
			t.Errorf("缺 %q", want)
		}
	}
	if !strings.Contains(joined, "Type=exec") {
		t.Error("缺 Type=exec（execve 失败必须算启动失败）")
	}
	if !strings.Contains(joined, "--property=LimitMEMLOCK=1073741824") {
		t.Error("MemlockBytes>0 时应下发 LimitMEMLOCK")
	}
	// 挂载类选项**不许**出现在 systemd-run 侧（它们会静默失效）
	for _, banned := range []string{"BindReadOnlyPaths", "ReadOnlyPaths", "PrivateTmp", "ProtectHome"} {
		if strings.Contains(joined, banned) {
			t.Errorf("systemd-run 侧不得出现挂载类选项 %s（§6.1 实测静默失效）", banned)
		}
	}
	// bwrap 必须作为它的子命令（"--" 之后紧跟 bwrap）
	i := strings.Index(joined, "-- bwrap ")
	if i < 0 {
		t.Fatalf("systemd-run 应把 bwrap 当子命令执行，实得 %v", argv)
	}
	if !strings.Contains(joined, "--ro-bind /data/models/k2 /models") {
		t.Error("bwrap 的权重绑定应完整落在 systemd-run 之后的 argv 里")
	}
}

// 限额未声明 ⇒ 不下发 LimitMEMLOCK（不许编造）。
func TestBuildSystemdRunArgv_NoMemlockByDefault(t *testing.T) {
	s := goodSpec()
	argv, _ := BuildSystemdRunArgv(s, UnitName(s.EggID))
	if strings.Contains(strings.Join(argv, " "), "LimitMEMLOCK") {
		t.Fatal("未声明限额时不得下发 LimitMEMLOCK")
	}
}

// 单元名：确定性 + 合法（只小写字母数字连字符）+ 空/怪名字也能兜住。
func TestUnitName_SanitizedAndDeterministic(t *testing.T) {
	if UnitName("example-moe-36b") != UnitName("example-moe-36b") {
		t.Fatal("同一卵名必须得到同一单元名（收卵幂等的前提）")
	}
	got := UnitName("Qwen3.8-Flash/Next 实验")
	if strings.ContainsAny(got, "/ .") {
		t.Fatalf("单元名不得含非法字符，实得 %q", got)
	}
	if got != strings.ToLower(got) {
		t.Fatalf("单元名应全小写，实得 %q", got)
	}
	if UnitName("") != "zerg-egg" {
		t.Fatalf("空卵名应兜底为 zerg-egg，实得 %q", UnitName(""))
	}
	if n := len(UnitName(strings.Repeat("很长很长的名字", 20))); n > 56 {
		t.Fatalf("超长卵名应截断到安全长度，实得 %d", n)
	}
}

// 收卵幂等：rc=0 成功；rc=5 + "could not be found" 也算成功（X3 实测二次 stop）；
// 其它错误必须如实上报（不许把失败当成功）。
func TestCollectOutcome_IdempotentButHonest(t *testing.T) {
	if err := collectOutcome(0, ""); err != nil {
		t.Fatalf("rc=0 应成功，实得 %v", err)
	}
	if err := collectOutcome(5, "Unit llm-x.service could not be found."); err != nil {
		t.Fatalf("单元本就不存在应视为收卵成功（幂等），实得 %v", err)
	}
	if err := collectOutcome(1, "Failed to stop unit: permission denied"); err == nil {
		t.Fatal("真失败必须上报（不许静默当成功）")
	}
}

// 封闭性核验判据（§6.9）：以 X3 实测真值形态为样例。
func TestCheckEnclosure_Verdict(t *testing.T) {
	// 达标样例：/models ro、无 /data 无 /home、/tmp 是 tmpfs
	ok := CheckEnclosure(`25 30 0:23 / / rw,relatime - tmpfs tmpfs rw
30 25 8:1 /data/models/k2 /models ro,relatime - ext4 /dev/nvme0n1p7 ro
31 25 0:5 / /tmp rw - tmpfs tmpfs rw`)
	ok.NewPIDNamespace = true
	if !ok.Enclosed() {
		t.Fatalf("达标样例应判通过，实得 %s", ok)
	}
	// 漏：/data 可见（正是"静默失效"的形态）
	leak := CheckEnclosure(`25 30 0:23 / / rw,relatime - tmpfs tmpfs rw
30 25 8:1 /data /data ro,relatime - ext4 /dev/nvme0n1p7 ro
31 25 0:5 / /tmp rw - tmpfs tmpfs rw`)
	leak.NewPIDNamespace = true
	if leak.Enclosed() {
		t.Fatalf("/data 可见时不得判通过，实得 %s", leak)
	}
	if leak.DataHidden {
		t.Fatal("/data 出现时必须标记为未隐藏")
	}
	// 权重不是只读 ⇒ 不得判通过
	notro := CheckEnclosure(`25 30 0:23 / / rw - tmpfs tmpfs rw
30 25 8:1 /data/models/k2 /models rw,relatime - ext4 /dev/nvme0n1p7 rw
31 25 0:5 / /tmp rw - tmpfs tmpfs rw`)
	notro.NewPIDNamespace = true
	if notro.Enclosed() {
		t.Fatal("权重非只读时不得判通过")
	}
}
