// wall_bridge_test.go —— 迁移桥的 Go 半边（任务 2'.3）。
//
// **它做什么**：读一份**卵配方 JSON**（键面 = 茧壁 `wall/src/spec.rs` 认的那套，与 `hatch.Spec`
// 的**策略面**一一对应）⇒ 装配成 `hatch.Spec` ⇒ 调 `BuildBwrapArgv` ⇒ 把结果落到 `ZERG_BRIDGE_OUT`。
// Rust 那半边（`zerg-wall plan`）读**同一批配方文件**出 argv，两侧逐条比 —— 不一致即红。
//
// **为什么做成「测试」而不是多一个 cmd**：Go 侧产出 bwrap argv 的**唯一真源**就是 `BuildBwrapArgv`；
// 再包一层独立命令行等于多开一个**可能漂移的出口**（本项目反复踩的正是「同一落点两套真相」）。
// `_test.go` 不进任何制品，天然只在对拍时被叫起来。
//
// **为什么用环境变量当入口**：`go test` 没有别的干净通路把「读哪里、写哪里」传进来；而**没设变量
// 就跳过**是安全的 —— 本用例**不做断言**，真正的门禁是 `scripts/evals/compare-wall-argv.py`，它读不到产物
// 就 rc=2 硬失败（那边不许静默通过）。
//
// 与茧壁那半边的**唯一差异**（已写进两侧注释）：`BuildBwrapArgv` 只给**参数序列**，可执行名
// `bwrap` 由 `BuildSystemdRunArgv` 补；茧壁的 `argv` 是**完整命令行**（`argv[0] = bwrap`）。
// 所以桥的判据是两半：`wall.argv[0] == "bwrap"` **且** `wall.argv[1:]` 与这里逐条相同。
package hatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

// 对拍脚本设定的两个环境变量（入口说明见文件头）。
const (
	envBridgeSpecDir = "ZERG_BRIDGE_SPEC_DIR"
	envBridgeOut     = "ZERG_BRIDGE_OUT"
)

// wallSpecJSON 茧壁配方（`wall/src/spec.rs`）的键面。
//
// 为什么这里要再写一遍键名，而不是给 `hatch.Spec` 加 json tag：这一份是**对拍用适配器** ——
// 生产侧的「Go ⇒ 配方 JSON」序列化归 `backend/hatch_spec.go`（批 3/5 接上时，这一份应当连同
// 它的调用点一起删掉；留两处就是两份真相）。字段顺序与 `spec.rs` 的 `Spec` 结构逐条对齐，
// 方便读的人一眼对起来。
type wallSpecJSON struct {
	EggID             string            `json:"egg_id"`
	SchemaVersion     int               `json:"schema_version"`
	EnginePathInSpace string            `json:"engine_path_in_space"`
	EngineArgs        []string          `json:"engine_args"`
	EngineRoots       []string          `json:"engine_roots"`
	WeightPath        string            `json:"weight_path"`
	WeightFiles       []string          `json:"weight_files"`
	Env               map[string]string `json:"env"`
	Devices           []string          `json:"devices"`
	ExtraROBinds      []string          `json:"extra_ro_binds"`
	ExtraRWBinds      []string          `json:"extra_rw_binds"`
	WorkDir           string            `json:"work_dir"`
	MemlockBytes      int64             `json:"memlock_bytes"`
}

// specFromWallSpec 把配方键面装配成孵化声明（策略面逐条对应）。
//
// 茧壁**不消费**的字段（`Profile` 这类决策面）由 Go 侧这一半补足 —— 见 bridgeProfile。
func specFromWallSpec(w wallSpecJSON) Spec {
	return Spec{
		EggID:             w.EggID,
		SchemaVersion:     w.SchemaVersion,
		EnginePathInSpace: w.EnginePathInSpace,
		EngineArgs:        w.EngineArgs,
		EngineRoots:       w.EngineRoots,
		WeightPath:        w.WeightPath,
		WeightFiles:       w.WeightFiles,
		Env:               w.Env,
		Devices:           w.Devices,
		ExtraROBinds:      w.ExtraROBinds,
		ExtraRWBinds:      w.ExtraRWBinds,
		WorkDir:           w.WorkDir,
		MemlockBytes:      w.MemlockBytes,
		Profile:           bridgeProfile(),
	}
}

// bridgeProfile 合成一份**合法但最小**的实测档案。
//
// 为什么必须有：`Spec.Validate` 要档案（§8.4「无档案不许孵」），而**茧壁根本不消费档案**
// （那是**决策面**，配方里没有这一格 —— 也不该有：档案是每台机器的实测事实，随卵跑的不是它）。
// ⇒ 对拍两侧对「同一条输入」的定义写在代码里：**策略面来自配方文件，决策面（档案）由 Go 侧补足**；
// 比的也只是策略面产出的 argv。用 **v1** 形态（不带 enclosure 块）：本用例与方法本身无关，
// 少写一格就少一处会过期的东西。
func bridgeProfile() monitor.EggProfile {
	return monitor.EggProfile{
		WeightSizeGb:         22.4,
		PeakGttGb:            24.0,
		PeakMemGb:            8.0,
		LoadSeconds:          41.0,
		ThroughputTokS:       31.0,
		SuggestedIdleUnloadS: 900,
		SchemaVersion:        monitor.EggProfileSchemaVersionV1,
		MeasuredAt:           time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
		Machine:              "x3",
		CalibRuns:            3,
	}
}

// bridgeResult 一份配方的对拍产物：要么 argv，要么**拒绝理由**（两侧都拒也是一致）。
type bridgeResult struct {
	Argv  []string `json:"argv"`
	Error string   `json:"error"`
}

// TestWallBridgeDumpBwrapArgv 只做「读配方 ⇒ 出 argv ⇒ 落盘」，**不做断言**（真正的判据在
// `scripts/evals/compare-wall-argv.py`：它同时拿到两侧产物才比得出来）。
//
// 但**空集合必须红**：配方目录里没有 `*.json`、或解析不出来、或写不出产物 —— 一律 `Fatal`，
// 不许悄悄「零条对拍、零条失败」地过去（无区分度的绿比红更危险）。
func TestWallBridgeDumpBwrapArgv(t *testing.T) {
	dir := os.Getenv(envBridgeSpecDir)
	out := os.Getenv(envBridgeOut)
	if dir == "" || out == "" {
		t.Skipf("未设置 %s / %s ⇒ 跳过（对拍脚本 scripts/evals/compare-wall-argv.py 会设置）",
			envBridgeSpecDir, envBridgeOut)
	}
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("枚举配方目录 %s：%v", dir, err)
	}
	sort.Strings(entries)
	if len(entries) == 0 {
		t.Fatalf("配方目录 %s 里一个 *.json 都没有 —— 对拍必须有对象（空集合不算通过）", dir)
	}

	res := make(map[string]bridgeResult, len(entries))
	for _, p := range entries {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("读配方 %s：%v", p, err)
		}
		var w wallSpecJSON
		if err := json.Unmarshal(raw, &w); err != nil {
			t.Fatalf("解析配方 %s（键面必须与 wall/src/spec.rs 同源）：%v", p, err)
		}
		argv, err := BuildBwrapArgv(specFromWallSpec(w))
		if err != nil {
			// 拒绝也是**对拍结果**（两侧都拒 = 一致）；理由原文如实落盘，便于人工核对
			// 「拒的是不是同一件事」。
			res[filepath.Base(p)] = bridgeResult{Error: err.Error()}
			continue
		}
		res[filepath.Base(p)] = bridgeResult{Argv: argv}
	}

	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		t.Fatalf("序列化对拍产物：%v", err)
	}
	if err := os.WriteFile(out, append(b, '\n'), 0o644); err != nil {
		t.Fatalf("写对拍产物 %s：%v", out, err)
	}
	t.Logf("已落 %d 份配方的 Go 侧 argv 到 %s", len(res), out)
}
