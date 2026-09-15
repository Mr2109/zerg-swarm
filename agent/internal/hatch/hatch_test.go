// hatch_test.go —— 孵化器验收（纯函数部分，跨平台可跑；真执行在 X3 上另验）。
//
// 覆盖：声明 fail-closed（缺字段/版本不认/无档案一律拒孵）· bwrap 命令行配方（含 X3 实测
// 三坑）· systemd-run 只负责归属 · 单元名确定性与合法性 · 收卵幂等语义 · 封闭性核验判据。
package hatch

import (
	"os"
	"os/exec"
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

// goodWeightFile 本卵的权重文件在宿主侧的位置（goodSpec 声明它、也绑它）。
const goodWeightFile = "/data/models/k2/k2horizon-q4_k_m.gguf"

func goodSpec() Spec {
	return Spec{
		EggID:             "example-moe-36b",
		SchemaVersion:     registry.EggSchemaVersionCurrent,
		EnginePathInSpace: "/engine/build-k2/bin/llama-server",
		EngineArgs:        []string{"-m", "/models/k2horizon-q4_k_m.gguf", "--port", "58100"},
		EngineRoots:       []string{"/home/g01/llama-k2"},
		// WeightPath 只用于出证/日志（2026-09-15 收口后它退出挂载面）
		WeightPath: "/data/models/k2",
		// 逐文件权重：声明 + 由映射侧产出的逐文件只读绑定（两者是一份契约的两个半边）
		WeightFiles:  []string{goodWeightFile},
		ExtraROBinds: []string{goodWeightFile + ":/models/k2horizon-q4_k_m.gguf"},
		Env:          map[string]string{"LD_LIBRARY_PATH": "/engine/build-k2/bin"},
		WorkDir:      "/tmp",
		Profile:      goodProfile(),
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
		{"权重两个字段都空", func(s *Spec) { s.WeightPath = ""; s.WeightFiles = nil; s.ExtraROBinds = nil }, "权重"},
		{"工作目录非空间内绝对路径", func(s *Spec) { s.WorkDir = "work" }, "空间内"},
		{"工作目录是 ~ 形态", func(s *Spec) { s.WorkDir = "~/.zerg/work/example-moe-36b" }, "空间内"},
		{"只读绑定缺分隔符", func(s *Spec) { s.ExtraROBinds = []string{"/data/x"} }, "额外只读绑定"},
		{"可写绑定缺分隔符", func(s *Spec) { s.ExtraRWBinds = []string{"/home/g01/.zerg/work/K2"} }, "额外可写绑定"},
		{"可写绑定空宿主", func(s *Spec) { s.ExtraRWBinds = []string{":/work"} }, "额外可写绑定"},
		{"可写绑定空落点", func(s *Spec) { s.ExtraRWBinds = []string{"/home/g01/.zerg/work/K2:"} }, "额外可写绑定"},
		{"可写绑定只有空白", func(s *Spec) { s.ExtraRWBinds = []string{"  :  "} }, "额外可写绑定"},
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

// 权重逐文件（2026-09-15 收口，缺陷 12）：**声明（WeightFiles）↔ 绑定（ExtraROBinds）当契约核**，
// 且**任何绑定都不许落在 /models 本身**（整目录挂载）。任一条不满足即拒孵（fail-closed）。
func TestSpec_ValidateWeightFilesFailClosed(t *testing.T) {
	const (
		declared = "/data/models/k2/k2horizon-q4_k_m.gguf"
		landing  = "/models/k2horizon-q4_k_m.gguf"
	)
	bad := []struct {
		name string
		mut  func(*Spec)
		want string
	}{
		{"清单里有空串", func(s *Spec) { s.WeightFiles = []string{""} }, "weight_files"},
		{"清单里只有空白", func(s *Spec) { s.WeightFiles = []string{"   "} }, "weight_files"},
		{"声明不是宿主绝对路径", func(s *Spec) {
			s.WeightFiles = []string{"data/models/k2/w.gguf"}
			s.ExtraROBinds = []string{"data/models/k2/w.gguf:/models/w.gguf"}
		}, "绝对路径"},
		{"基名撞车（两个目录下同名文件）", func(s *Spec) {
			s.WeightFiles = []string{"/data/a/w.gguf", "/data/b/w.gguf"} // 同一个落点 /models/w.gguf
			s.ExtraROBinds = []string{"/data/a/w.gguf:/models/w.gguf"}
		}, "落点都是"},
		{"声明了却没挂（引擎在空间里找不到它）", func(s *Spec) { s.ExtraROBinds = nil }, "没有对应的只读绑定"},
		{"落点相同但挂的是别的文件", func(s *Spec) {
			s.ExtraROBinds = []string{"/data/models/k2/other.gguf:" + landing}
		}, "挂的是"},
		{"同一落点两条绑定（后挂的静默盖掉先挂的）", func(s *Spec) {
			s.ExtraROBinds = []string{
				declared + ":" + landing,
				"/data/other/k2horizon-q4_k_m.gguf:" + landing,
			}
		}, "多条"},
		{"整目录挂到 /models —— 只读也不许", func(s *Spec) {
			s.ExtraROBinds = append(s.ExtraROBinds, "/data/models/k2:/models")
		}, "整目录"},
		{"整目录**可写**挂到 /models", func(s *Spec) {
			s.ExtraRWBinds = append(s.ExtraRWBinds, "/data/models/k2:/models")
		}, "整目录"},
		{"可写绑定落进 /models/<…>（权重会被引擎改写）", func(s *Spec) {
			s.ExtraRWBinds = append(s.ExtraRWBinds, goodWeightFile+":/models/k2horizon-q4_k_m.gguf")
		}, "可写绑定"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			s := goodSpec()
			c.mut(&s)
			err := s.Validate()
			if err == nil {
				t.Fatalf("应拒孵，却通过了校验（WeightFiles=%v ExtraROBinds=%v）", s.WeightFiles, s.ExtraROBinds)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("错误信息应提到 %q，实得 %q", c.want, err.Error())
			}
			if _, err2 := BuildBwrapArgv(s); err2 == nil {
				t.Error("命令行构造也必须跟着拒（不许绕过校验出 argv）")
			}
		})
	}

	t.Run("整目录绑定的拒绝理由要点名 缺陷 12 与只读落点后果", func(t *testing.T) {
		s := goodSpec()
		s.ExtraROBinds = []string{"/data/models/k2:/models"}
		err := s.Validate()
		if err == nil {
			t.Fatal("整目录挂载必须拒孵")
		}
		msg := err.Error()
		for _, want := range []string{"整目录", "缺陷 12", "Read-only file system", spaceModelsDir} {
			if !strings.Contains(msg, want) {
				t.Errorf("理由应写清 %q（下一个人得知道为什么不行、真机上会怎么死），实得 %q", want, msg)
			}
		}
	})

	t.Run("合法形态照过：声明与绑定逐条对上", func(t *testing.T) {
		s := goodSpec() // WeightFiles=[权重]，ExtraROBinds=[权重→/models/<基名>]
		if err := s.Validate(); err != nil {
			t.Fatalf("声明与绑定对上的卵不该被拒，实得 %v", err)
		}
		// 库目录（/libs）与模板（/templates）走的是同一张 ExtraROBinds，不该被权重这条绊住
		s.ExtraROBinds = append(s.ExtraROBinds, "/home/g01/build-hip-flash:/libs/build-hip-flash", "/data/tpl/chat.jinja:/templates/chat.jinja")
		if err := s.Validate(); err != nil {
			t.Fatalf("同表的库/模板绑定不该把权重那一条判错，实得 %v", err)
		}
	})

	t.Run("只报出证信息的旧形态（只有 WeightPath、没有绑定）仍放行", func(t *testing.T) {
		s := goodSpec()
		s.WeightFiles = nil
		s.ExtraROBinds = nil
		if err := s.Validate(); err != nil {
			t.Fatalf("没有声明清单时没有可核的对象，不该因 WeightFiles 空而拒（拒的是**两个字段都空**），实得 %v", err)
		}
	})

	t.Run("路径写法不同但同一个文件 ⇒ 不算挂错（Clean 口径）", func(t *testing.T) {
		s := goodSpec()
		raw := "/data/models/k2/../k2/k2horizon-q4_k_m.gguf"
		s.WeightFiles = []string{raw}
		s.ExtraROBinds = []string{raw + ":" + landing}
		if err := s.Validate(); err != nil {
			t.Fatalf("同一文件的两种写法不该判成「挂错文件」（误拒一枚好卵比漏判更糟），实得 %v", err)
		}
		if got := spaceWeightLanding(raw); got != landing {
			t.Errorf("落点规则的唯一实现应把 %q 归到 %q，实得 %q", raw, landing, got)
		}
	})
}

// bwrap 配方：X3 实测三坑必须体现（usrmerge 符号链接 / 设备显式绑定 / 权重的挂法）。
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
	// ③ 权重：**逐文件只读**（整目录挂载已退役，缺陷 12）+ 落点由 `--dir /models` 先造出来，
	//    且**不得有任何绑定落在 /models 本身**（那会让逐文件绑定落不进去）。
	if !strings.Contains(joined, "--dir "+spaceModelsDir) {
		t.Errorf("缺 `--dir %s`（逐文件绑定的落点得先存在）", spaceModelsDir)
	}
	for _, want := range []string{
		"--ro-bind " + goodWeightFile + " /models/k2horizon-q4_k_m.gguf",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("缺该卵权重文件的逐文件只读绑定 %q", want)
		}
	}
	if strings.Contains(joined, "--ro-bind "+s.WeightPath+" /models") {
		t.Error("不许再挂权重**所在目录**（缺陷 12：同目录别的模型一起进空间；且 /models 成只读挂载后逐文件绑定落不进去）")
	}
	// 任何绑定（只读/可写）的落点都不得正好是 /models
	for i := 0; i+2 < len(argv); i++ {
		if (argv[i] == "--ro-bind" || argv[i] == "--bind") && argv[i+2] == spaceModelsDir {
			t.Errorf("出现了落在 %s 本身的 %s 绑定（%v）——整目录挂载", spaceModelsDir, argv[i], argv[i:i+3])
		}
	}
	// 宿主权重树里别的模型路径不得出现；本卵权重文件恰好出现一次（就在那条绑定里）
	hostWeights := 0
	for _, a := range argv {
		if strings.Contains(a, "/data/models/") {
			hostWeights++
			if a != goodWeightFile { // 唯一允许出现的宿主权重路径就是本卵点名的那个文件
				t.Errorf("argv 里出现了别的宿主权重路径 %q（只许挂本卵点名的文件）", a)
			}
		}
	}
	if hostWeights != 1 {
		t.Errorf("宿主权重文件路径应恰好出现 1 次，实得 %d 次", hostWeights)
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
	// 按卵环境变量：**经包装 exec** 下发（本机 bwrap --setenv 必失败，见缺陷 4）
	if strings.Contains(joined, "--setenv") {
		t.Errorf("argv 里不得再出现 --setenv（bubblewrap 0.11.1 上必失败）：%q", joined)
	}
	script, scriptArg0, engine, args := engineEntry(t, argv)
	if script == "" {
		t.Fatal("声明了 env 的卵必须走包装 exec（/bin/sh -c <脚本> …）")
	}
	if !strings.Contains(script, "LD_LIBRARY_PATH='/engine/build-k2/bin'") {
		t.Errorf("包装脚本里应把按卵环境变量 export 出去，实得 %q", script)
	}
	if !strings.HasSuffix(script, `exec "$@"`) {
		t.Errorf("包装脚本必须以 exec \"$@\" 收尾（引擎 + 参数原样传下去），实得 %q", script)
	}
	if scriptArg0 != "zerg-egg" {
		t.Errorf("包装脚本的 $0 应是固定普通字（不许用 -- 当 $0：语义随实现而异），实得 %q", scriptArg0)
	}
	if engine != "/engine/build-k2/bin/llama-server" {
		t.Errorf("包装形态下引擎路径应作为 $1，实得 %q", engine)
	}
	if strings.Join(args, " ") != strings.Join(s.EngineArgs, " ") {
		t.Errorf("引擎参数应原样跟在引擎路径之后（顺序与个数都不许变）：want %v got %v", s.EngineArgs, args)
	}
}

// 逐文件权重的 argv 配方（2026-09-15 收口，缺陷 12）：`--dir /models` 打底**在前**、逐文件只读绑定按
// 声明顺序落下、**没有任何**绑定的落点是 /models 本身、同一 Spec 逐字可复现。
//
// 形态取自 x3 报告 §5（第一枚卵真机实测）：真机上挂的是 `--ro-bind /data/models/qwen /models`
// （整个目录），空间内 `ls /models` 能列出 Qwen2.5-VL / Flash-Next 等**同目录的别的模型** ——
// 本用例钉住的就是那一刀改成逐文件（真机形态的 mountinfo 侧在 TestCheckEnclosure_RealMachineShape）。
func TestBuildBwrapArgv_PerFileWeightsLanding(t *testing.T) {
	const (
		wGguf    = "/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf"
		wMMProj  = "/data/models/qwen/mmproj-Qwen3.8-27B-f16.gguf"
		wTpl     = "/data/tpl/chat.jinja"
		landGguf = "/models/Qwen3.8-27B-Q4_K_M-vcruz305.gguf"
		landMM   = "/models/mmproj-Qwen3.8-27B-f16.gguf"
		landTpl  = "/templates/chat.jinja"
	)
	s := goodSpec()
	s.WeightPath = "/data/models/qwen" // 出证/日志用：整个目录（**不许**再据此挂载）
	s.WeightFiles = []string{wGguf, wMMProj}
	s.ExtraROBinds = []string{
		wGguf + ":" + landGguf,
		wMMProj + ":" + landMM,
		wTpl + ":" + landTpl, // 权重目录之外的模板走输入通道（/templates），不是权重
	}
	s.EngineArgs = []string{"-m", landGguf, "--mmproj", landMM, "--chat-template-file", landTpl, "--port", "9010"}

	argv, err := BuildBwrapArgv(s)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	// bindAt 「落点为 want」的绑定下标（-1 = 没有）
	bindAt := func(want string) int {
		for i := 0; i+2 < len(argv); i++ {
			if (argv[i] == "--ro-bind" || argv[i] == "--bind") && argv[i+2] == want {
				return i
			}
		}
		return -1
	}

	// ① 打底：`--dir /models` 恰好一次
	dirIdx := -1
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == "--dir" && argv[i+1] == spaceModelsDir {
			if dirIdx >= 0 {
				t.Errorf("`--dir %s` 重复出现（落点只造一次）：%v", spaceModelsDir, argv)
			}
			dirIdx = i
		}
	}
	if dirIdx < 0 {
		t.Fatalf("缺 `--dir %s`（逐文件绑定的落点得先存在），实得 %v", spaceModelsDir, argv)
	}

	// ② 逐文件只读绑定：宿主文件 → /models/<基名>（落点由声明清单唯一决定）
	iGguf, iMM, iTpl := bindAt(landGguf), bindAt(landMM), bindAt(landTpl)
	if iGguf < 0 || iMM < 0 || iTpl < 0 {
		t.Fatalf("每个权重/模板文件都必须各有一条只读绑定，实得 argv=%v", argv)
	}
	for _, c := range []struct {
		idx  int
		host string
		land string
	}{{iGguf, wGguf, landGguf}, {iMM, wMMProj, landMM}, {iTpl, wTpl, landTpl}} {
		if argv[c.idx] != "--ro-bind" {
			t.Errorf("落点 %s 必须是**只读**绑定，实得 %q（权重可被引擎改写 = 不符）", c.land, argv[c.idx])
		}
		if argv[c.idx+1] != c.host {
			t.Errorf("落点 %s 的宿主侧应是 %q，实得 %q", c.land, c.host, argv[c.idx+1])
		}
	}

	// ③ 顺序：打底在最前 → 权重按**声明顺序** → 模板（输入通道）在其后；顺序即 argv，逐字可复现
	if !(dirIdx < iGguf && iGguf < iMM && iMM < iTpl) {
		t.Fatalf("顺序应为 `--dir` → 权重1 → 权重2 → 模板（dir=%d gguf=%d mmproj=%d tpl=%d）：%v",
			dirIdx, iGguf, iMM, iTpl, argv)
	}

	// ④ 硬边界：没有任何绑定的落点是 /models 本身（整目录挂载）；宿主目录路径只出现在**日志字段**里，
	//    绝不出现在 argv 的绑定中
	for i := 0; i+2 < len(argv); i++ {
		if (argv[i] == "--ro-bind" || argv[i] == "--bind") && argv[i+2] == spaceModelsDir {
			t.Errorf("出现了落在 %s 本身的绑定（整目录挂载）：%v", spaceModelsDir, argv[i:i+3])
		}
		if (argv[i] == "--ro-bind" || argv[i] == "--bind") && argv[i+1] == s.WeightPath {
			t.Errorf("权重**所在目录** %s 不许出现在绑定里（只挂点名的文件）：%v", s.WeightPath, argv[i:i+3])
		}
	}
	if n := strings.Count(joined, wGguf); n != 1 {
		t.Errorf("权重文件路径应只出现 1 次（就在那条只读绑定里），实得 %d 次：%q", n, joined)
	}
	// 同目录的**别的模型**在两个面上都不得出现（绑定面 + 参数面）
	for _, other := range []string{"Qwen2.5-VL-32B", "Flash-Next", "Qwen3.6-35B"} {
		if strings.Contains(joined, other) {
			t.Errorf("同目录的别的模型 %q 不得出现在 argv 里（真机缺陷 12 正是它漏进来了）", other)
		}
	}

	// ⑤ 可复现：同一 Spec 两次构造逐字一致
	a1, _ := BuildBwrapArgv(s)
	a2, _ := BuildBwrapArgv(s)
	if strings.Join(a1, " ") != strings.Join(a2, " ") {
		t.Error("同一 Spec 两次构造 argv 应完全一致（可复现）")
	}
}

// 环境变量顺序必须可复现（同一 Spec 两次构造 argv 完全一致），且**键按名排序**写进包装脚本。
func TestBuildBwrapArgv_DeterministicEnvOrder(t *testing.T) {
	s := goodSpec()
	s.Env = map[string]string{"ZED": "2", "ALPHA": "1", "MID": "3"}
	a1, _ := BuildBwrapArgv(s)
	a2, _ := BuildBwrapArgv(s)
	if strings.Join(a1, " ") != strings.Join(a2, " ") {
		t.Fatal("同一 Spec 两次构造 argv 应完全一致（可复现）")
	}
	script, _, _, _ := engineEntry(t, a1)
	ia := strings.Index(script, "ALPHA=")
	im := strings.Index(script, "MID=")
	iz := strings.Index(script, "ZED=")
	if !(ia >= 0 && ia < im && im < iz) {
		t.Fatalf("包装脚本里的环境变量应按 key 排序（可复现），实得 %q", script)
	}
}

// engineEntry 拆出 bwrap argv 里「`--` 之后那一段」的入口形态：
// 返回 (包装脚本, 包装脚本的 $0, 引擎路径, 引擎参数)；**没有包装**时脚本与 $0 为空串。
//
// 为什么要有它：入口有两种合法形态（有 env ⇒ 包装 exec；无 env ⇒ 直 exec），用例不该自己
// 猜下标位置（猜错会把「参数错位」这种问题测成通过）。
func engineEntry(t *testing.T, argv []string) (script, arg0, engine string, args []string) {
	t.Helper()
	sep := -1
	for i, a := range argv {
		if a == "--" {
			sep = i // 取最后一个：包装 exec 形态下 `--` 只有 bwrap 那一个（脚本里没有 -- 当 $0）
		}
	}
	if sep < 0 {
		t.Fatalf("argv 里找不到 bwrap 的分隔符 --：%v", argv)
	}
	rest := argv[sep+1:]
	if len(rest) == 0 {
		t.Fatal("`--` 之后没有入口")
	}
	if rest[0] == "/bin/sh" {
		if len(rest) < 5 {
			t.Fatalf("包装 exec 形态至少要有 /bin/sh -c <脚本> <$0> <引擎>：%v", rest)
		}
		if rest[1] != "-c" {
			t.Fatalf("包装 exec 应走 sh -c，实得 %q", rest[1])
		}
		return rest[2], rest[3], rest[4], rest[5:]
	}
	return "", "", rest[0], rest[1:]
}

// ExtraRWBinds：**可写**绑定必须落成 bwrap `--bind`（不是 --ro-bind），与只读绑定共存时顺序稳定，
// 且 `--chdir` 用的是**空间内**路径 —— 宿主路径填 WorkDir 会让真孵化 chdir 失败（本用例钉住它）。
func TestBuildBwrapArgv_ExtraRWBindsAreWritable(t *testing.T) {
	const (
		hostWork = "/home/g01/.zerg/work/example-moe-36b"
		hostKV   = "/home/g01/.zerg/kvdisk/example-moe-36b"
	)
	s := goodSpec()
	s.WorkDir = "/work"
	s.ExtraROBinds = []string{goodWeightFile + ":/models/k2horizon-q4_k_m.gguf", "/tmp/tpl:/templates"}
	s.ExtraRWBinds = []string{hostWork + ":/work", hostKV + ":/kvdisk"}

	argv, err := BuildBwrapArgv(s)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	// ① 可写绑定 → `--bind <host> <space>`（**不是** --ro-bind）
	for _, want := range []string{"--bind " + hostWork + " /work", "--bind " + hostKV + " /kvdisk"} {
		if !strings.Contains(joined, want) {
			t.Errorf("缺可写绑定 %q，实得 %q", want, joined)
		}
	}
	for _, banned := range []string{"--ro-bind " + hostWork + " /work", "--ro-bind " + hostKV + " /kvdisk"} {
		if strings.Contains(joined, banned) {
			t.Errorf("可写绑定不得落成只读绑定：%q", banned)
		}
	}
	// ② 与只读绑定共存：只读仍走 --ro-bind，两边不串味
	if !strings.Contains(joined, "--ro-bind /tmp/tpl /templates") {
		t.Errorf("只读绑定应仍走 --ro-bind，实得 %q", joined)
	}
	if strings.Contains(joined, "--bind /tmp/tpl /templates") {
		t.Error("只读绑定不得被当成可写绑定")
	}
	// ③ 顺序稳定：只读绑定在前 → 可写绑定按清单顺序 → --chdir 在所有绑定之后
	iRO := strings.Index(joined, "--ro-bind /tmp/tpl")
	iWork := strings.Index(joined, "--bind "+hostWork)
	iKV := strings.Index(joined, "--bind "+hostKV)
	iChdir := strings.Index(joined, "--chdir")
	if !(iRO >= 0 && iRO < iWork && iWork < iKV && iKV < iChdir) {
		t.Fatalf("顺序应为 只读 → 工作目录 → KV 盘 → --chdir（ro=%d work=%d kv=%d chdir=%d）：%q",
			iRO, iWork, iKV, iChdir, joined)
	}
	// ④ `--chdir` 必须是**空间内**路径；宿主路径只许出现在绑定里
	ci := indexOf(argv, "--chdir")
	if ci < 0 || ci+1 >= len(argv) {
		t.Fatalf("argv 里找不到 --chdir，实得 %v", argv)
	}
	if got := argv[ci+1]; got != "/work" {
		t.Fatalf("--chdir 必须用空间内路径 /work，实得 %q", got)
	}
	if argv[ci+1] == hostWork {
		t.Fatal("--chdir 不得用宿主路径（空间内不存在该目录 ⇒ 真孵化必 chdir 失败）")
	}
	// ⑤ 同一 Spec 两次构造逐字一致（可复现）
	a1, _ := BuildBwrapArgv(s)
	a2, _ := BuildBwrapArgv(s)
	if strings.Join(a1, " ") != strings.Join(a2, " ") {
		t.Fatal("同一 Spec 两次构造 argv 应完全一致（可复现）")
	}
	// ⑥ 格式非法 ⇒ 校验阶段就拒（fail-closed，与只读同口径）
	bad := goodSpec()
	bad.ExtraRWBinds = []string{"/only-host-side"}
	if err := bad.Validate(); err == nil {
		t.Fatal("格式非法的可写绑定必须在校验阶段被拒（fail-closed）")
	}
}

// indexOf 取 argv 里第一个等于 want 的下标（找不到返回 -1）。
func indexOf(argv []string, want string) int {
	for i, a := range argv {
		if a == want {
			return i
		}
	}
	return -1
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
	if !strings.Contains(joined, "--ro-bind "+goodWeightFile+" /models/k2horizon-q4_k_m.gguf") {
		t.Error("bwrap 的逐文件权重绑定应完整落在 systemd-run 之后的 argv 里")
	}
	if strings.Contains(joined, "--ro-bind "+s.WeightPath+" /models") {
		t.Error("不许出现整目录权重挂载（缺陷 12：逐文件才是口径）")
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
	// mountinfo 样例（第 6 段 = 每挂载点选项；第 5 段 = 挂载点）
	const (
		rootLine = `25 30 0:23 / / rw,relatime - tmpfs tmpfs rw`
		tmpLine  = `31 25 0:5 / /tmp rw - tmpfs tmpfs rw`
		// 逐文件权重（2026-09-15 收口）：落点是 /models/<基名>；**没有** /models 本身的挂载项
		// （/models 只是 --dir 造的空目录 ⇒ 它本来就不是挂载点）
		modelsRO = `30 25 8:1 /data/models/k2/k2horizon-q4_k_m.gguf /models/k2horizon-q4_k_m.gguf ro,relatime - ext4 /dev/nvme0n1p7 ro`
		workRW   = `32 25 8:1 /home/g01/.zerg/work/k2 /work rw,nosuid,nodev - ext4 /dev/nvme0n1p7 rw`
		kvRW     = `33 25 8:1 /home/g01/.zerg/kvdisk/k2 /kvdisk rw,nosuid,nodev - ext4 /dev/nvme0n1p7 rw`
	)
	// 声明清单（Spec.WeightFiles 原样传进来，宿主侧路径）——判据「核的是哪些文件」由它决定
	declared := []string{goodWeightFile}
	// 进程视图那一项（NewPIDNamespace）不入 mountinfo，由调用方按空间内实际进程数置位。
	verdict := func(lines ...string) EnclosureReport {
		rep := CheckEnclosure(strings.Join(lines, "\n"), declared)
		rep.NewPIDNamespace = true
		return rep
	}

	// 达标样例：权重逐文件 ro、无 /data 无 /home、/tmp 是 tmpfs、/work 可写；该卵没有 KV 盘 ⇒ 通过
	ok := verdict(rootLine, modelsRO, tmpLine, workRW)
	if !ok.Enclosed() {
		t.Fatalf("达标样例应判通过，实得 %s", ok)
	}
	if len(ok.Failures()) != 0 {
		t.Errorf("通过时不应列出任何不符项，实得 %v", ok.Failures())
	}
	if len(ok.ModelsFiles) != 1 || ok.ModelsFiles[0] != "/models/k2horizon-q4_k_m.gguf" {
		t.Errorf("报告应记下**实证**（见到的逐文件只读落点），实得 %v", ok.ModelsFiles)
	}
	// 有 KV 盘且可写 ⇒ 仍通过（「没这个落点」与「有且可写」都算过）
	if okKV := verdict(rootLine, modelsRO, tmpLine, workRW, kvRW); !okKV.Enclosed() {
		t.Fatalf("KV 盘可写时也应判通过，实得 %s", okKV)
	}

	// 漏：/data 可见（正是"静默失效"的形态）
	leak := verdict(rootLine,
		`30 25 8:1 /data /data ro,relatime - ext4 /dev/nvme0n1p7 ro`, tmpLine, workRW)
	if leak.Enclosed() {
		t.Fatalf("/data 可见时不得判通过，实得 %s", leak)
	}
	if leak.DataHidden {
		t.Fatal("/data 出现时必须标记为未隐藏")
	}
	if !strings.Contains(strings.Join(leak.Failures(), "；"), "/data") {
		t.Errorf("不符项应点名 /data，实得 %v", leak.Failures())
	}

	// 权重不是只读 ⇒ 不得判通过（逐文件那一格是 rw）
	notro := verdict(rootLine,
		`30 25 8:1 /data/models/k2/k2horizon-q4_k_m.gguf /models/k2horizon-q4_k_m.gguf rw,relatime - ext4 /dev/nvme0n1p7 rw`,
		tmpLine, workRW)
	if notro.Enclosed() {
		t.Fatal("权重非只读时不得判通过")
	}
	notroMsg := strings.Join(notro.Failures(), "；")
	for _, want := range []string{"/models/k2horizon-q4_k_m.gguf", "不是只读"} {
		if !strings.Contains(notroMsg, want) {
			t.Errorf("不符项应点名 %q（核的是哪个文件必须一眼看得见），实得 %q", want, notroMsg)
		}
	}

	// /home 漏进来 ⇒ 不通过
	home := verdict(rootLine, modelsRO, tmpLine, workRW,
		`34 25 8:1 /home /home ro,relatime - ext4 /dev/nvme0n1p7 ro`)
	home.NewPIDNamespace = true
	if home.Enclosed() {
		t.Fatal("/home 可见时不得判通过")
	}
}

// 真机形态（x3 报告 §5「mountinfo 四判据（原文）」，第一枚卵 2026-09-15）：逐文件之后空间里的权重
// 应该是**每个声明文件一条只读挂载**，而 **/models 本身没有挂载项**（它只是 `--dir` 造的空目录）。
//
// §5 原文（当时是整目录挂法，本用例照它的字段形态改成逐文件）：
//
//	/models  opts=ro,nosuid,nodev,noatime  fstype=ext4  line=6623 6601 259:7 /models/qwen /models ro,…
//	/work    opts=rw,nosuid,nodev          fstype=tmpfs line=6625 6601 0:46 /egg1/work/qwen38-27b-egg /work rw,…
//	/        opts=rw,nosuid,nodev,relatime  fstype=tmpfs line=6601 6547 0:72 /newroot / rw,…
//	空间内进程（bwrap 的子进程）pid=1379473 mnt-ns=mnt:[4026532767] /data 隐藏 / 可见 2 pids
//
// 钉住两件事：① 逐文件形态判**通过**（旧判据在这里会把好卵判成不符：它要 `/models` 挂载项，而新配方
// 里那条**根本不存在**）；② 判据与结论话术都点名「核的是哪两个文件」。
func TestCheckEnclosure_RealMachineShape(t *testing.T) {
	const (
		// ① 根：全新 tmpfs（不是宿主根）
		rootLine = `6601 6547 0:72 /newroot / rw,nosuid,nodev,relatime - tmpfs tmpfs rw`
		// ② 系统目录与引擎目录只读（§5 的 /usr 与 /engine）
		usrLine = `6602 6601 259:2 /usr /usr ro,nosuid,nodev,relatime - ext4 /dev/nvme0n1p2 ro`
		engLine = `6624 6601 259:2 /home/g01/llama.cpp-src/build-hip-flash/bin /engine ro,nosuid,nodev,relatime - ext4 /dev/nvme0n1p2 ro`
		// ③ 权重：**逐文件**只读（§5 的 opts/nosuid,nodev,noatime + fstype=ext4 + 259:7 原样；
		//    唯一的改动就是落点从整目录 `/models` 变成 `/models/<基名>`）
		ggufLine = `6623 6601 259:7 /data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf /models/Qwen3.8-27B-Q4_K_M-vcruz305.gguf ro,nosuid,nodev,noatime - ext4 /dev/nvme0n1p7 ro`
		mmLine   = `6626 6601 259:7 /data/models/qwen/mmproj-Qwen3.8-27B-f16.gguf /models/mmproj-Qwen3.8-27B-f16.gguf ro,nosuid,nodev,noatime - ext4 /dev/nvme0n1p7 ro`
		// ④ 工作目录可写（§5 原文：那一枚卵把工作目录设在 /tmp 下故 fstype=tmpfs）
		workLine = `6625 6601 0:46 /egg1/work/qwen38-27b-egg /work rw,nosuid,nodev - tmpfs tmpfs rw`
		// ⑤ /tmp：配方恒 `--tmpfs /tmp`（§5 的摘录没列它，按配方形态补一行）
		tmpLine = `6603 6601 0:73 / /tmp rw,nosuid,nodev - tmpfs tmpfs rw`
	)
	mountinfo := strings.Join([]string{rootLine, usrLine, engLine, ggufLine, mmLine, workLine, tmpLine}, "\n")
	declared := []string{
		"/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf",
		"/data/models/qwen/mmproj-Qwen3.8-27B-f16.gguf",
	}

	rep := CheckEnclosure(mountinfo, declared)
	// §5 的进程事实：空间内进程是 bwrap 的子进程 1379473（宿主视图的 MainPID 1379471 不算核验对象）
	rep.NewPIDNamespace, rep.PID = true, 1379473
	rep.PIDSource = "给定 pid=1379471 在宿主挂载命名空间（mnt:[4026531832]）⇒ 改验空间内进程 pid=1379473"

	if !rep.Enclosed() {
		t.Fatalf("逐文件形态的真机 mountinfo 应判通过，实得 %s（不符项=%v）", rep, rep.Failures())
	}
	if len(rep.Failures()) != 0 {
		t.Errorf("通过时不应列出不符项，实得 %v", rep.Failures())
	}
	// ① 判据的实证：两个文件各占一格、都是只读；**没有任何**落点在 /models 本身
	if rep.ModelsDirMounted {
		t.Error("逐文件配方里不存在 /models 本身的挂载项（那是整目录挂载的形态）")
	}
	wantLands := []string{"/models/Qwen3.8-27B-Q4_K_M-vcruz305.gguf", "/models/mmproj-Qwen3.8-27B-f16.gguf"}
	if strings.Join(rep.ModelsFiles, " ") != strings.Join(wantLands, " ") {
		t.Errorf("见到的逐文件只读落点应为 %v，实得 %v", wantLands, rep.ModelsFiles)
	}
	if len(rep.ModelsRWFiles) != 0 {
		t.Errorf("不该有非只读的权重落点，实得 %v", rep.ModelsRWFiles)
	}
	// ② 「核的是哪些文件」= 声明清单换算出的落点（宿主路径 → /models/<基名>）
	if strings.Join(rep.WeightsChecked, " ") != strings.Join(wantLands, " ") {
		t.Errorf("判据核过的落点应为声明清单换算出的 %v，实得 %v", wantLands, rep.WeightsChecked)
	}
	if missing := rep.MissingWeightFiles(); len(missing) != 0 {
		t.Errorf("两个声明文件都见到了，不该报缺失，实得 %v", missing)
	}
	// ③ mountinfo 里**确实没有** /models 挂载项（这条就是「看不到它是正常形态」的机器证据：
	//    换了旧判据，这一份 mountinfo 会被判成「/models 不是只读」而毙掉一枚好卵）
	for _, e := range parseMountinfo(mountinfo) {
		if e.MountPoint == spaceModelsDir {
			t.Fatalf("本用例的 mountinfo 不该有 %s 挂载项（逐文件配方里它不是挂载点）", spaceModelsDir)
		}
	}
	// ④ 其余判据照旧：/data 隐藏、/home 无内容、/tmp 是 tmpfs、/work 可写
	if !rep.DataHidden || !rep.HomeHidden || !rep.TmpIsTmpfs || !rep.WorkReadWrite || !rep.KVDiskReadWrite {
		t.Errorf("其余五项应全过，实得 %s", rep)
	}
	// ⑤ 结论话术要点名核的是哪两个文件（读日志的人不必再去机器上 ls /models）
	note := rep.String()
	if !strings.Contains(note, "封闭性核验通过") {
		t.Errorf("结论话术应保留可读的结论词，实得 %q", note)
	}
	for _, want := range append([]string{"pid=1379473"}, wantLands...) {
		if !strings.Contains(note, want) {
			t.Errorf("结论话术应含 %q，实得 %q", want, note)
		}
	}
}

// 权重那一项的判据（2026-09-15 收口）：**每个声明的权重文件都是只读挂载** + **没有整目录的 /models
// 挂载**。任一条不满足即不符（上层据此收卵拒孵）。用例逐条钉住，并钉住「拿不到声明时只核结构」的降级。
func TestCheckEnclosure_PerFileWeightsCriterion(t *testing.T) {
	const (
		rootLine = `25 30 0:23 / / rw,relatime - tmpfs tmpfs rw`
		tmpLine  = `31 25 0:5 / /tmp rw - tmpfs tmpfs rw`
		workRW   = `32 25 8:1 /home/g01/.zerg/work/k2 /work rw,nosuid,nodev - ext4 /dev/nvme0n1p7 rw`
		// 整目录挂载（缺陷 12 的原形）：挂的是权重**所在目录**，且它还是只读的
		dirRO = `30 25 8:1 /data/models/qwen /models ro,relatime - ext4 /dev/nvme0n1p7 ro`
	)
	const (
		hostA = "/data/models/qwen/a.gguf"
		hostB = "/data/models/qwen/mmproj-b.gguf"
		landA = "/models/a.gguf"
		landB = "/models/mmproj-b.gguf"
	)
	fileRO := func(land string) string {
		return `30 25 8:1 /data/models/qwen` + strings.TrimPrefix(land, "/models") + ` ` + land + ` ro,nosuid,nodev,noatime - ext4 /dev/nvme0n1p7 ro`
	}
	fileRW := func(land string) string {
		return `30 25 8:1 /data/models/qwen` + strings.TrimPrefix(land, "/models") + ` ` + land + ` rw,nosuid,nodev,noatime - ext4 /dev/nvme0n1p7 rw`
	}
	// 进程视图那一项不入 mountinfo：核验报告里它由调用方置位（这里恒 true，好让结论只由权重决定）
	verdict := func(declared []string, lines ...string) EnclosureReport {
		rep := CheckEnclosure(strings.Join(lines, "\n"), declared)
		rep.NewPIDNamespace = true
		return rep
	}
	reason := func(rep EnclosureReport) string { return strings.Join(rep.Failures(), "；") }

	t.Run("整目录挂载即便只读 ⇒ 不符（旧判据会在这里判**通过**）", func(t *testing.T) {
		rep := verdict(nil, rootLine, dirRO, tmpLine, workRW)
		if !rep.ModelsDirMounted {
			t.Fatal("必须记下「出现了落在 /models 本身的挂载」这条实证")
		}
		if rep.ModelsReadOnly || rep.Enclosed() {
			t.Fatalf("整目录挂载不得判通过（只读也不够：同目录别的模型一起进空间），实得 %s", rep)
		}
		msg := reason(rep)
		for _, want := range []string{"整目录", "判据=", spaceModelsDir} {
			if !strings.Contains(msg, want) {
				t.Errorf("不符理由应含 %q（写清判据与形态），实得 %q", want, msg)
			}
		}
	})

	t.Run("逐文件落点是 rw ⇒ 不符，且点名那个文件", func(t *testing.T) {
		rep := verdict([]string{hostA}, rootLine, fileRW(landA), tmpLine, workRW)
		if len(rep.ModelsRWFiles) != 1 || rep.ModelsRWFiles[0] != landA {
			t.Fatalf("应记下非只读的落点 %s，实得 %v", landA, rep.ModelsRWFiles)
		}
		if rep.Enclosed() {
			t.Fatal("权重落点可写时不得判通过")
		}
		if msg := reason(rep); !strings.Contains(msg, landA) || !strings.Contains(msg, "不是只读") {
			t.Errorf("不符理由应点名 %s 与「不是只读」，实得 %q", landA, msg)
		}
	})

	t.Run("声明两个、只见到一个 ⇒ 不符，且点名缺的那个", func(t *testing.T) {
		rep := verdict([]string{hostA, hostB}, rootLine, fileRO(landA), tmpLine, workRW)
		if rep.Enclosed() {
			t.Fatalf("%s 没挂进来时不得判通过（引擎找不到它），实得 %s", landB, rep)
		}
		missing := rep.MissingWeightFiles()
		if len(missing) != 1 || missing[0] != landB {
			t.Fatalf("缺失清单应是 [%s]，实得 %v", landB, missing)
		}
		msg := reason(rep)
		for _, want := range []string{landB, "没有以只读挂载出现", landA} {
			if !strings.Contains(msg, want) {
				t.Errorf("不符理由应含 %q（缺谁、已见到谁都要看得见），实得 %q", want, msg)
			}
		}
	})

	t.Run("落点在 /models 之下但不是声明的那一格 ⇒ 声明的那一格仍算缺", func(t *testing.T) {
		// 挂的是同目录另一个文件（落点也不叫 a.gguf）：声明的那一格不在 ⇒ 不符
		rep := verdict([]string{hostA}, rootLine, fileRO("/models/other.gguf"), tmpLine, workRW)
		if rep.Enclosed() {
			t.Fatal("声明文件的落点没出现时不得判通过（挂别的文件不等于挂它）")
		}
		if missing := rep.MissingWeightFiles(); len(missing) != 1 || missing[0] != landA {
			t.Fatalf("缺失清单应是 [%s]，实得 %v", landA, missing)
		}
	})

	t.Run("一个 /models/* 落点都没有 ⇒ 不符（权重压根没挂进来）", func(t *testing.T) {
		rep := verdict(nil, rootLine, tmpLine, workRW)
		if rep.Enclosed() {
			t.Fatal("没有任何逐文件权重落点时不得判通过")
		}
		msg := reason(rep)
		for _, want := range []string{"见不到任何逐文件权重挂载", "本身不是挂载点"} {
			if !strings.Contains(msg, want) {
				t.Errorf("文案应含 %q（否则下一个人会把「看不到 /models」当成故障去修），实得 %q", want, msg)
			}
		}
	})

	t.Run("拿不到声明（nil）⇒ 只核结构那半条：至少一个只读落点即可", func(t *testing.T) {
		rep := verdict(nil, rootLine, fileRO(landA), fileRO(landB), tmpLine, workRW)
		if !rep.Enclosed() {
			t.Fatalf("结构形态齐备时应判通过（真机核验路径拿不到 Spec.WeightFiles），实得 %s", rep)
		}
		if len(rep.WeightsChecked) != 0 {
			t.Errorf("没给声明时不该凭空造出「核过的落点」，实得 %v", rep.WeightsChecked)
		}
		if len(rep.ModelsFiles) != 2 {
			t.Errorf("实证应记下两个只读落点，实得 %v", rep.ModelsFiles)
		}
	})

	t.Run("拿不到声明时，整目录挂载照样不符", func(t *testing.T) {
		rep := verdict(nil, rootLine, dirRO, tmpLine, workRW)
		if rep.Enclosed() {
			t.Fatal("结构形态下整目录挂载仍必须判不符（硬边界不因缺声明而放松）")
		}
	})

	t.Run("声明清单的顺序即核验顺序（与 mountinfo 里的出现次序无关）", func(t *testing.T) {
		rep := verdict([]string{hostB, hostA}, rootLine, fileRO(landA), fileRO(landB), tmpLine, workRW)
		if !rep.Enclosed() {
			t.Fatalf("两个声明文件都在时应判通过，实得 %s", rep)
		}
		if strings.Join(rep.WeightsChecked, " ") != landB+" "+landA {
			t.Errorf("核验清单应按**声明顺序**，实得 %v", rep.WeightsChecked)
		}
		// 两个落点都点进了结论话术（读日志的人一眼看出挂了什么）
		note := rep.String()
		for _, want := range []string{landA, landB, "判据核的落点="} {
			if !strings.Contains(note, want) {
				t.Errorf("结论话术应含 %q，实得 %q", want, note)
			}
		}
	})
}

// /work 与 /kvdisk 的可写性判据（三态口径里「实读且不符」的形态之一，2026-09-15 拍）：
// **/work 必须见到且可写**（每枚孵出来的卵都有工作目录：映射侧恒填 /work + 可写绑定）；
// **/kvdisk 没见到就是「该卵没有这个落点」**（缺省可写），见到却不可写才算不符。
func TestCheckEnclosure_WritableLandings(t *testing.T) {
	const (
		rootLine = `25 30 0:23 / / rw,relatime - tmpfs tmpfs rw`
		tmpLine  = `31 25 0:5 / /tmp rw - tmpfs tmpfs rw`
		modelsRO = `30 25 8:1 /data/models/k2/k2horizon-q4_k_m.gguf /models/k2horizon-q4_k_m.gguf ro,relatime - ext4 /dev/nvme0n1p7 ro`
	)
	verdict := func(lines ...string) EnclosureReport {
		rep := CheckEnclosure(strings.Join(lines, "\n"), []string{goodWeightFile})
		rep.NewPIDNamespace = true
		return rep
	}
	workLine := func(opts string) string {
		return `32 25 8:1 /home/g01/.zerg/work/k2 /work ` + opts + ` - ext4 /dev/nvme0n1p7 rw`
	}
	kvLine := func(opts string) string {
		return `33 25 8:1 /home/g01/.zerg/kvdisk/k2 /kvdisk ` + opts + ` - ext4 /dev/nvme0n1p7 rw`
	}

	t.Run("/work 挂成只读 ⇒ 不符", func(t *testing.T) {
		rep := verdict(rootLine, modelsRO, tmpLine, workLine("ro,relatime"))
		if rep.WorkReadWrite {
			t.Fatal("/work 是只读时必须标记为不可写")
		}
		if rep.Enclosed() {
			t.Fatalf("/work 不可写时不得判通过，实得 %s", rep)
		}
		if !strings.Contains(strings.Join(rep.Failures(), "；"), "/work") {
			t.Errorf("不符项应点名 /work，实得 %v", rep.Failures())
		}
	})

	t.Run("/work 缺席 ⇒ 不符（那条可写绑定没生效）", func(t *testing.T) {
		rep := verdict(rootLine, modelsRO, tmpLine)
		if rep.WorkReadWrite || rep.Enclosed() {
			t.Fatalf("见不到 /work 时必须判不符（不是「该卵没工作目录」而是绑定没生效），实得 %s", rep)
		}
	})

	t.Run("/work 可写按整词认（rw,nosuid,nodev 算可写）", func(t *testing.T) {
		if rep := verdict(rootLine, modelsRO, tmpLine, workLine("rw,nosuid,nodev")); !rep.Enclosed() {
			t.Fatalf("rw,nosuid,nodev 必须算可写，实得 %s", rep)
		}
	})

	t.Run("ro 与 rw 同时出现 ⇒ 从严按只读", func(t *testing.T) {
		if rep := verdict(rootLine, modelsRO, tmpLine, workLine("ro,rw")); rep.WorkReadWrite {
			t.Fatalf("ro 与 rw 并存这种畸形必须按只读处理，实得 %s", rep)
		}
	})

	t.Run("/kvdisk 挂成只读 ⇒ 不符", func(t *testing.T) {
		rep := verdict(rootLine, modelsRO, tmpLine, workLine("rw,relatime"), kvLine("ro,relatime"))
		if rep.KVDiskReadWrite || rep.Enclosed() {
			t.Fatalf("KV 盘只读时不得判通过（写盘会静默失败），实得 %s", rep)
		}
	})

	t.Run("/kvdisk 缺席 ⇒ 该卵没有这个落点，不算不符", func(t *testing.T) {
		rep := verdict(rootLine, modelsRO, tmpLine, workLine("rw,relatime"))
		if !rep.KVDiskReadWrite {
			t.Fatal("没见到 /kvdisk 时应按「该卵无此落点」处理，不得当成不符")
		}
	})
}

// 「读到了但一行都认不得」= 读不到，**不是**不符：空/垃圾 mountinfo 必须在读的那一层报错，
// 不许拿一份零值报告当「实读结论」（否则上层按「不符」收卵拒孵，而真相是「没读到」）。
func TestMountinfoParseable(t *testing.T) {
	for _, txt := range []string{"", "   \n\n", "这不是 mountinfo\n随便两行\n"} {
		if err := mountinfoParseable(txt); err == nil {
			t.Errorf("文本 %q 一行都解析不出来，必须按「读不到」报错", txt)
		}
	}
	if err := mountinfoParseable("25 30 0:23 / / rw,relatime - tmpfs tmpfs rw\n"); err != nil {
		t.Errorf("正常 mountinfo 应算读到了，实得 %v", err)
	}
}

// Failures 与 Enclosed 必须同源：不符项清单为空 ⇔ 判通过；清单点名每一项不符（拒孵理由靠它写清）。
func TestEnclosureReport_Failures(t *testing.T) {
	var none EnclosureReport // 零值：七项全不符
	fails := none.Failures()
	if len(fails) != 7 {
		t.Fatalf("零值 report 应列出 7 项不符，实得 %d：%v", len(fails), fails)
	}
	for _, want := range []string{"/models", "/data", "/home", "/tmp", "/proc", "/work", "/kvdisk"} {
		if !strings.Contains(strings.Join(fails, "；"), want) {
			t.Errorf("不符清单应含 %q，实得 %v", want, fails)
		}
	}
	// 权重那一条的文案要把**判据本身**写出来（2026-09-15 收口后它不再是「/models 是不是 ro」一句），
	// 并说明「/models 本身不是挂载点」——否则拒孵理由会让读的人以为「看不到 /models」是故障。
	modelsItem := strings.Join(fails, "；")
	for _, want := range []string{"判据=每个声明的权重文件都是只读挂载", "整目录", "本身不是挂载点"} {
		if !strings.Contains(modelsItem, want) {
			t.Errorf("权重那条不符理由应含 %q，实得 %q", want, modelsItem)
		}
	}
	if none.Enclosed() {
		t.Fatal("零值 report 不得判通过")
	}
	full := EnclosureReport{
		ModelsReadOnly: true, DataHidden: true, HomeHidden: true,
		TmpIsTmpfs: true, NewPIDNamespace: true, WorkReadWrite: true, KVDiskReadWrite: true,
	}
	if !full.Enclosed() || len(full.Failures()) != 0 {
		t.Fatalf("七项全过必须判通过且不符清单为空，实得 %s / %v", full, full.Failures())
	}
	if !strings.Contains(full.String(), "封闭性核验通过") {
		t.Errorf("通过的结论话术应可读，实得 %q", full.String())
	}
}

// ── 缺陷 3：配方必须只读挂 /sys（引擎认设备靠它）──────────────────────────────

// 通用需求：设备**可不可用**由 /sys 下的拓扑/属性描述，故 /sys 恒只读进空间。
// X3 真机 2026-09-15：缺它 ⇒ `ggml_cuda_init: failed to initialize ROCm: no ROCm-capable
// device is detected`，引擎仍起得来但**静默降级 CPU**（设备节点已 --dev-bind 也认不到）。
func TestBuildBwrapArgv_SysReadOnlyBindPresent(t *testing.T) {
	s := goodSpec()
	argv, err := BuildBwrapArgv(s)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "--ro-bind /sys /sys") {
		t.Fatalf("配方必须只读挂 /sys（缺陷 3：缺它引擎认不到 GPU 并静默降级 CPU），实得 %q", joined)
	}
	if strings.Contains(joined, "--bind /sys /sys") {
		t.Error("/sys 不得挂成**可写**绑定（宿主 /sys 写面不许进空间）")
	}
	if n := strings.Count(joined, "--ro-bind /sys /sys"); n != 1 {
		t.Errorf("/sys 只读绑定应恰好出现 1 次，实得 %d 次", n)
	}
	// 出现在 bwrap 选项段（`--` 之前），不是被塞进引擎参数
	sep := indexOf(argv, "--")
	idx := -1
	for i, a := range argv {
		if a == "--ro-bind" && i+2 < len(argv) && argv[i+1] == "/sys" && argv[i+2] == "/sys" {
			idx = i
		}
	}
	if idx < 0 || idx > sep {
		t.Fatalf("/sys 只读绑定应在 bwrap 选项段（`--` 之前），实得 idx=%d sep=%d", idx, sep)
	}
	// 空 Devices 与声明 Devices 都不影响 /sys（它是配方的一部分，不是"某引擎特性"）
	s2 := goodSpec()
	s2.Devices = []string{"/dev/dri/renderD129"}
	if a2, err := BuildBwrapArgv(s2); err != nil || !strings.Contains(strings.Join(a2, " "), "--ro-bind /sys /sys") {
		t.Errorf("声明了别的设备节点时 /sys 仍须在（通用需求）")
	}
}

// ── 缺陷 4：按卵环境变量经**包装 exec** 下发，不再用 bwrap --setenv ──────────────

// argv 里**不得**再有 --setenv；环境变量与引擎参数必须真的从包装脚本里过（真跑一遍 /bin/sh）。
func TestBuildBwrapArgv_WrapperExecCarriesEnvAndArgs(t *testing.T) {
	s := goodSpec()
	s.Env = map[string]string{"LD_LIBRARY_PATH": "/engine/build-k2/bin", "MARK": "a'b c"}
	argv, err := BuildBwrapArgv(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(argv, " "), "--setenv") {
		t.Fatalf("argv 里不得再出现 --setenv（bubblewrap 0.11.1 上必失败）：%v", argv)
	}
	script, arg0, _, _ := engineEntry(t, argv)
	if script == "" {
		t.Fatal("声明了 env 的卵必须走包装 exec")
	}
	if !strings.Contains(script, `MARK='a'\''b c'`) {
		t.Errorf("值里的单引号必须转义（否则包装脚本语法错或值被截断），实得 %q", script)
	}
	if !strings.Contains(script, `LD_LIBRARY_PATH='/engine/build-k2/bin'`) {
		t.Errorf("按卵 LD_LIBRARY_PATH 必须进包装脚本，实得 %q", script)
	}
	if !strings.HasSuffix(script, `exec "$@"`) {
		t.Errorf("包装脚本须以 exec \"$@\" 收尾，实得 %q", script)
	}

	// 真跑一遍包装脚本（把「引擎」换成一段回显环境变量与 $1 的命令）：
	// 这是「环境变量确实进了包装脚本、参数确实原样传到引擎」的**实证**，不是看字符串。
	inner := `printf '%s|%s|%s' "$LD_LIBRARY_PATH" "$MARK" "$1"`
	cmd := exec.Command(shPath, "-c", script, arg0, shPath, "-c", inner, "inner-arg0", "引擎参数")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("真跑包装脚本失败：%v（脚本=%q）", err, script)
	}
	want := "/engine/build-k2/bin|a'b c|引擎参数"
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("包装 exec 实测不符：\n want %q\n  got %q（脚本=%q）", want, got, script)
	}

	// 值逐字不变性（含「值里带单引号/空格」这些会写坏 shell 引号的形态）：
	// 每个值单独过一遍真 shell —— 包装脚本里值只进过 `export K='…'` 这一处，出不来就说明引号坏了。
	for _, c := range []struct{ name, val string }{
		{"普通值", "/engine/build-k2/bin"},
		{"单引号在中间", "a'b c"},
		{"只有一个单引号", "'"},
		{"连续三个单引号", "'''"},
		{"单引号结尾", "end'"},
		{"换行", "line1\nline2"},
		{"反斜杠", `a\b\c`},
		{"双引号", `he said "hi"`},
		{"美元与反引号", "$HOME`id`"},
		{"首尾空格", "  pad  "},
		{"空值", ""},
	} {
		sp := goodSpec()
		sp.Env = map[string]string{"V": c.val}
		a, err := BuildBwrapArgv(sp)
		if err != nil {
			t.Fatalf("%s：构造 argv 失败：%v", c.name, err)
		}
		sc, a0, _, _ := engineEntry(t, a)
		// `printf '%s' "$V"`：值必须逐字节等于声明里的那份（$ 与反引号若被第二次展开就会露馅）
		cc := exec.Command(shPath, "-c", sc, a0, shPath, "-c", `printf '%s' "$V"`)
		o, err := cc.Output()
		if err != nil {
			t.Fatalf("%s：真跑包装脚本失败：%v（脚本=%q）", c.name, err, sc)
		}
		if got := string(o); got != c.val {
			t.Errorf("%s：值过包装脚本后应逐字不变\n want %q\n  got %q（脚本=%q）", c.name, c.val, got, sc)
		}
	}

	// 可复现：同一 Spec 两次构造逐字一致
	a1, _ := BuildBwrapArgv(s)
	a2, _ := BuildBwrapArgv(s)
	if strings.Join(a1, " ") != strings.Join(a2, " ") {
		t.Error("同一 Spec 两次构造 argv 应完全一致（可复现）")
	}

	// 跨 shell（X3 的 /bin/sh 是 **dash**，macOS 的是 bash）：同一个包装脚本再在别的实现上过一遍。
	// 这一段是为「`--` 当 $0 的语义随实现而异」「单引号转义」这类跨 shell 坑准备的 —— 只有真跑
	// 才算证明（workdir 上哪个 shell 存在就跑哪个，缺了就如实跳过并记一行）。
	for _, sh := range []string{"/bin/dash", "/bin/bash"} {
		if _, err := os.Stat(sh); err != nil {
			t.Logf("跳过 %s（本机没有）", sh)
			continue
		}
		t.Run("跨 shell 同样成立:"+sh, func(t *testing.T) {
			sp := goodSpec()
			sp.Env = map[string]string{"V": "a'b c", "LD_LIBRARY_PATH": "/engine/build-k2/bin"}
			a, err := BuildBwrapArgv(sp)
			if err != nil {
				t.Fatal(err)
			}
			sc, a0, _, _ := engineEntry(t, a)
			cc := exec.Command(sh, "-c", sc, a0, sh, "-c", `printf '%s|%s|%s' "$V" "$LD_LIBRARY_PATH" "$1"`, "i0", "PARAM")
			o, err := cc.Output()
			if err != nil {
				t.Fatalf("%s 上真跑包装脚本失败：%v（脚本=%q）", sh, err, sc)
			}
			if got, want := strings.TrimSpace(string(o)), "a'b c|/engine/build-k2/bin|PARAM"; got != want {
				t.Fatalf("%s：包装 exec 实测不符\n want %q\n  got %q（脚本=%q）", sh, want, got, sc)
			}
		})
	}
}

// 没有按卵环境变量 ⇒ **不加**包装层（多一层 sh 只会让 execve 失败的归因变含糊），直 exec，
// 且同样不许出现 --setenv。
func TestBuildBwrapArgv_NoEnvNoWrapper(t *testing.T) {
	s := goodSpec()
	s.Env = nil
	argv, err := BuildBwrapArgv(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(argv, " "), "--setenv") {
		t.Errorf("无 env 时也不得出现 --setenv：%v", argv)
	}
	script, _, engine, args := engineEntry(t, argv)
	if script != "" {
		t.Errorf("没有要下发的环境变量时不该加包装层，实得脚本 %q", script)
	}
	if engine != s.EnginePathInSpace || len(args) != len(s.EngineArgs) {
		t.Errorf("直 exec 形态应是 <引擎> <参数…>，实得 engine=%q args=%v", engine, args)
	}
	if got, ok := execWrapperScript(nil); ok || got != "" {
		t.Errorf("execWrapperScript(nil) 应返回 (\"\", false)，实得 (%q, %v)", got, ok)
	}
}

// 环境变量名/值的 fail-closed 校验：名字要能被包装脚本的 `export` 承接（否则整条脚本走偏），
// 值里不许有 execve 不可能接受的 NUL。
func TestValidate_EnvFailClosed(t *testing.T) {
	bad := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"名字含连字符", map[string]string{"FOO-BAR": "1"}, "环境变量名"},
		{"名字以数字开头", map[string]string{"1X": "1"}, "环境变量名"},
		{"名字含空格", map[string]string{"A B": "1"}, "环境变量名"},
		{"名字为空", map[string]string{"": "1"}, "环境变量名"},
		{"值含 NUL", map[string]string{"OK": "a\x00b"}, "NUL"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			s := goodSpec()
			s.Env = c.env
			err := s.Validate()
			if err == nil {
				t.Fatal("非法环境变量声明必须拒孵，却通过了")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("错误信息应提到 %q，实得 %q", c.want, err.Error())
			}
			if _, err2 := BuildBwrapArgv(s); err2 == nil {
				t.Error("命令行构造也必须跟着拒（不许绕过校验出 argv）")
			}
		})
	}
	// 合法形态照过：下划线/数字/空值/含任意怪字符的值
	s := goodSpec()
	s.Env = map[string]string{"_A1": "", "B2_C": "x'y z$`{};"}
	if err := s.Validate(); err != nil {
		t.Errorf("合法的环境变量声明不该被拒，实得 %v", err)
	}
}

// ── 缺陷 2：核验对象必须是**空间内进程**，不是 bwrap 父进程 ────────────────────

// 真机形态（X3 2026-09-15）：
//
//	pid=1379471 MainPID（bwrap 父进程）在宿主挂载命名空间 → /models 不存在、/data 可见、550 pids
//	pid=1379473 空间内进程（bwrap 子进程）     → /models=ro、/work=rw、/data 隐藏、2 pids
//	pid=1379474 空间内引擎（llama-server）
//
// 挑错（挑到 1379471）⇒ 读到宿主视图 ⇒ 判"不符" ⇒ 每枚卵被自己的核验杀掉（缺陷 2 的后果）。
func TestPickSpacePID_RealMachineShape(t *testing.T) {
	const (
		hostNS  = "mnt:[4026531832]" // MainPID / 宿主的挂载命名空间
		spaceNS = "mnt:[4026532767]" // 空间内进程的挂载命名空间
		unitCg  = "0::/user.slice/user-1000.slice/user@1000.service/llm.slice/zerg-qwen38-27b-egg.service"
		otherCg = "0::/user.slice/user-1000.slice/user@1000.service/x3-agent.service"
	)
	procs := []spaceProc{
		{PID: 1379471, Cgroup: unitCg, MntNS: hostNS, Comm: "bwrap"},         // MainPID：bwrap 父进程（留在原 ns）
		{PID: 1379473, Cgroup: unitCg, MntNS: spaceNS, Comm: "bwrap"},        // 空间内 bwrap
		{PID: 1379474, Cgroup: unitCg, MntNS: spaceNS, Comm: "llama-server"}, // 空间内引擎
		{PID: 999, Cgroup: otherCg, MntNS: spaceNS, Comm: "llama-server"},    // 别的单元（同 ns 也不许挑）
		{PID: 5, Cgroup: otherCg, MntNS: hostNS, Comm: "x3-agentd"},          // 宿主上的别的进程
		{PID: 1379470, Cgroup: unitCg, MntNS: hostNS, Comm: "systemd"},       // 同单元但留在宿主 ns
	}
	pid, why, err := pickSpacePID(1379471, unitCg, hostNS, procs)
	if err != nil {
		t.Fatalf("真机形态应能挑出空间内进程，实得错误 %v", err)
	}
	if pid != 1379474 {
		t.Fatalf("应挑空间内的引擎 pid=1379474（非 bwrap 优先），实得 %d（理由=%q）", pid, why)
	}
	if pid == 1379471 {
		t.Fatal("挑中 bwrap 父进程 = 读宿主视图 = 缺陷 2 原形")
	}
	for _, want := range []string{"1379471", "宿主挂载命名空间", "空间内进程", "1379474", spaceNS} {
		if !strings.Contains(why, want) {
			t.Errorf("判定依据里应能看出 %q，实得 %q", want, why)
		}
	}
	// 候选顺序不同 ⇒ 结论相同（取最小 pid 只为可复现，不是碰运气）
	shuffled := []spaceProc{procs[5], procs[3], procs[2], procs[4], procs[1], procs[0]}
	if pid2, _, err2 := pickSpacePID(1379471, unitCg, hostNS, shuffled); err2 != nil || pid2 != pid {
		t.Errorf("候选顺序不同不应改变结论：先得 %d，换序得 %d（err=%v）", pid, pid2, err2)
	}

	t.Run("只有空间内 bwrap（引擎还没起）⇒ 挑它", func(t *testing.T) {
		only := []spaceProc{procs[0], procs[1], procs[4]}
		pid, why, err := pickSpacePID(1379471, unitCg, hostNS, only)
		if err != nil || pid != 1379473 {
			t.Fatalf("应退而挑空间内的 bwrap 子进程 1379473（mountinfo 按 ns 论，同 ns 内一致），实得 pid=%d err=%v（理由=%q）", pid, err, why)
		}
	})

	t.Run("空间已死（只剩 bwrap 父进程）⇒ 报错，绝不退回宿主视图", func(t *testing.T) {
		dead := []spaceProc{procs[0], procs[5], procs[4]}
		pid, _, err := pickSpacePID(1379471, unitCg, hostNS, dead)
		if err == nil {
			t.Fatalf("找不到空间内进程必须报错（上层记「未核验」），却给出 pid=%d", pid)
		}
		if pid != 0 {
			t.Errorf("报错时不得返回任何 pid（返回值会被误用），实得 %d", pid)
		}
		if !strings.Contains(err.Error(), "空间内进程一个都没找到") {
			t.Errorf("错误应写清「没找到空间内进程」，实得 %q", err.Error())
		}
	})

	t.Run("cgroup 读不出 ⇒ 报错，不猜单元边界", func(t *testing.T) {
		if pid, _, err := pickSpacePID(1379471, "", hostNS, procs); err == nil || pid != 0 {
			t.Fatalf("cgroup 为空时必须报错，实得 pid=%d err=%v", pid, err)
		}
	})

	t.Run("给定 pid 自己不参与挑选", func(t *testing.T) {
		// 即使给定的那个 pid 声称在别的命名空间，也不许把它自己当核验对象
		only := []spaceProc{{PID: 1379471, Cgroup: unitCg, MntNS: spaceNS, Comm: "llama-server"}}
		if pid, _, err := pickSpacePID(1379471, unitCg, hostNS, only); err == nil {
			t.Fatalf("候选里只有给定 pid 自己时必须报错，实得 pid=%d", pid)
		}
	})
}

// cgroup 原文形态跨版本：v2 单行 `0::/path`；v1 多行 `N:ctrl:/path`。
func TestCgroupPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0::/user.slice/user-1000.slice/llm.slice/zerg-x.service\n", "/user.slice/user-1000.slice/llm.slice/zerg-x.service"},
		{"11:memory:/user.slice/zerg-x.service\n2:cpu:/user.slice/zerg-x.service\n", "/user.slice/zerg-x.service"},
		{"", ""},
		{"垃圾一行\n", ""},
	}
	for _, c := range cases {
		if got := cgroupPath(c.in); got != c.want {
			t.Errorf("cgroupPath(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
	// 同一单元不同原文（v1/v2）必须判成同一路径 ⇒ 判定"同一单元"才成立
	a := cgroupPath("0::/user.slice/u.service/llm.slice/zerg-x.service\n")
	b := cgroupPath("5:devices:/user.slice/u.service/llm.slice/zerg-x.service\n")
	if a != b {
		t.Errorf("同一单元的 v1/v2 原文应判出同一路径：%q vs %q", a, b)
	}
}

// ── 缺陷 13：home_hidden 的判据表述（「无宿主家目录**内容**」，不是「/home 不存在」）──

// 空间内会出现引擎**自建**的空目录 /home/<user>/.cache/...（真机 2026-09-15 实测），
// 那不是泄露；判据只看 mountinfo 里有没有 /home* 挂载，结论话术也必须按"内容"说。
func TestEnclosureReport_HomeCriterion(t *testing.T) {
	const (
		rootLine = `25 30 0:23 / / rw,relatime - tmpfs tmpfs rw`
		tmpLine  = `31 25 0:5 / /tmp rw - tmpfs tmpfs rw`
		modelsRO = `30 25 8:1 /data/models/k2/k2horizon-q4_k_m.gguf /models/k2horizon-q4_k_m.gguf ro,relatime - ext4 /dev/nvme0n1p7 ro`
		workRW   = `32 25 8:1 /home/g01/.zerg/work/k2 /work rw,nosuid,nodev - ext4 /dev/nvme0n1p7 rw`
	)
	// 声明清单（判据「核的是哪些文件」的来源）
	declared := []string{goodWeightFile}
	t.Run("没有 /home* 挂载 ⇒ 通过，且不记实证", func(t *testing.T) {
		rep := CheckEnclosure(strings.Join([]string{rootLine, modelsRO, tmpLine, workRW}, "\n"), declared)
		rep.NewPIDNamespace, rep.PID = true, 1379473
		if !rep.HomeHidden || len(rep.HomeMounts) != 0 {
			t.Fatalf("没有 /home* 挂载时必须判 HomeHidden=true 且无实证，实得 hidden=%v mounts=%v", rep.HomeHidden, rep.HomeMounts)
		}
		if !rep.Enclosed() {
			t.Fatalf("其余项齐时应判通过，实得 %s", rep)
		}
	})
	t.Run("出现 /home 挂载 ⇒ 不符，且点名是哪条挂载", func(t *testing.T) {
		homeMount := `34 25 8:1 /home /home ro,relatime - ext4 /dev/nvme0n1p7 ro`
		rep := CheckEnclosure(strings.Join([]string{rootLine, modelsRO, tmpLine, workRW, homeMount}, "\n"), declared)
		rep.NewPIDNamespace = true
		if rep.HomeHidden || rep.Enclosed() {
			t.Fatalf("mountinfo 里出现 /home 挂载时必须判不符，实得 %s", rep)
		}
		if len(rep.HomeMounts) != 1 || rep.HomeMounts[0] != "/home" {
			t.Fatalf("应记下实证挂载点 /home，实得 %v", rep.HomeMounts)
		}
		reason := strings.Join(rep.Failures(), "；")
		if !strings.Contains(reason, "家目录内容") {
			t.Errorf("话术必须按「家目录**内容**」说（缺陷 13），实得 %q", reason)
		}
		if !strings.Contains(reason, "/home") {
			t.Errorf("不符项应点名 /home，实得 %q", reason)
		}
		if strings.Contains(reason, "/home 在空间内可见") {
			t.Errorf("旧话术（被读成「/home 目录不存在」）不该再出现，实得 %q", reason)
		}
	})
	t.Run("子路径挂载也算（/home/g01）", func(t *testing.T) {
		rep := CheckEnclosure(strings.Join([]string{rootLine, modelsRO, tmpLine, workRW,
			`35 25 8:1 /home/g01 /home/g01 rw,relatime - ext4 /dev/nvme0n1p7 rw`}, "\n"), declared)
		if rep.HomeHidden || len(rep.HomeMounts) != 1 || rep.HomeMounts[0] != "/home/g01" {
			t.Fatalf("/home/g01 挂载必须算泄露并留实证，实得 hidden=%v mounts=%v", rep.HomeHidden, rep.HomeMounts)
		}
	})
}

// 结论话术里必须能看出**核验的是哪个 pid**（缺陷 2：不写 pid 就分不清读的是空间视图还是宿主视图）。
func TestEnclosureReport_StringCarriesPID(t *testing.T) {
	rep := EnclosureReport{
		PID: 1379473, PIDSource: "给定 pid=1379471 在宿主挂载命名空间（mnt:[4026531832]）",
		ModelsReadOnly: true, DataHidden: true, HomeHidden: true,
		TmpIsTmpfs: true, NewPIDNamespace: true, WorkReadWrite: true, KVDiskReadWrite: true,
	}
	s := rep.String()
	if !strings.Contains(s, "pid=1379473") {
		t.Errorf("结论话术必须写明核验对象的 pid，实得 %q", s)
	}
	if !strings.Contains(s, "1379471") {
		t.Errorf("结论话术应带判定依据（谁被换掉了），实得 %q", s)
	}
	if !strings.Contains(s, "封闭性核验通过") {
		t.Errorf("结论话术应保留可读的结论词，实得 %q", s)
	}
	var zero EnclosureReport
	if !strings.Contains(zero.String(), "pid=未知") {
		t.Errorf("没记 pid 的报告要如实写「未知」，不许写成 pid=0 这种像真 pid 的东西，实得 %q", zero.String())
	}
}
