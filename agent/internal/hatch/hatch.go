// hatch.go —— 孵化器（P1 施工）：把一枚卵孵成「封闭空间里的推理服务」。
//
// 设计真源：设计-子端沙箱化-20260914.md
//
//	§6.1 孵化形态（**已由 X3 真机实测修正**）：`bubblewrap` 管**封闭空间**（文件系统视图 +
//	     进程视图 + 只读），`systemd-run --user` 管**归属**（独立 slice/瞬态单元 ⇒ 不被
//	     `x3-agent.service` 的 KillMode 带走 = E1 的正面解法）。单用任一个都不完整。
//	§6.9 **静默失效不得当凭据**：隔离是否生效必须运行时核（读 `/proc/<pid>/mountinfo`），
//	     不得以单元状态为凭 —— 实测遇到过 `Result=success` 却零约束的挂载类选项。
//	§6.7 环境需求（EnvReq）：设备与卡号 / 库路径与版本 / 环境变量（含按卵覆盖
//	     `LD_LIBRARY_PATH`）/ 权重路径 / ulimit 与 mmap 限额 —— 孵化器**只照单执行**。
//	§4.3 第八项：`schema_version` 认不得就**明确报错**，不许静默按新格式跑错。
//	§8.4 标定铁律：**无有效实测档案不许孵**（闸门只读实测档案）。
//
// 配方三坑（X3 实测，务必照抄）：
//
//	① Ubuntu usrmerge ⇒ **必须自补 `/bin` `/lib` `/lib64` `/sbin` 符号链接**；
//	   `bwrap: execvp sh: No such file or directory` 是**缺链接，不是 userns 被拒**。
//	② 默认 `--dev /dev` **看不到 GPU** ⇒ 必须显式 `--dev-bind /dev/kfd` + `/dev/dri`。
//	③ `--unshare-user` 写不写行为一致（非 setuid ⇒ 总会自建 userns）。
//
// 本文件是**平台无关**部分：声明校验 + 命令行构造（纯函数，可在 macOS 上单测）。
// 真正执行（起单元 / 停单元）在 hatch_linux.go，非 Linux 平台在 hatch_other.go 明确拒绝。
package hatch

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// SliceName 归属切片：孵化单元一律落这里（与 `x3-agent.service` 的 `/system.slice` 分离）。
const SliceName = "llm.slice"

// Spec 一次孵化的全部输入（**声明式**：孵化器不推断、不补默认值，缺就报错）。
type Spec struct {
	EggID         string // 卵名（注册表键）——单元名与日志的身份
	SchemaVersion int    // 卵声明格式版本（必须 = registry.EggSchemaVersionCurrent）

	EnginePathInSpace string   // 引擎在**空间内**的可执行路径（如 /engine/bin/llama-server）
	EngineArgs        []string // 引擎参数（由适配器产出，已展开占位符）
	EngineRoots       []string // 引擎自己的库/构建目录（主机侧）——只读挂进 /engine

	WeightPath string // 该卵**自己的**权重（文件或目录）——只挂它 ⇒ 别的模型在空间内不存在

	Env     map[string]string // 按卵环境变量（含按引擎覆盖 LD_LIBRARY_PATH / HIP_VISIBLE_DEVICES）
	Devices []string          // 要暴露的设备节点（缺省 /dev/kfd + /dev/dri/renderD128）

	ExtraROBinds []string // 额外只读绑定，形如 "<host>:<space>"（如输入通道）
	WorkDir      string   // 空间内工作目录（可写，一次性语义）

	MemlockBytes int64 // >0 ⇒ 给 LimitMEMLOCK（大模型 mmap 需要；0 = 不设）

	Profile monitor.EggProfile // 实测档案（**必填**：无档案不许孵，§8.4）
}

// defaultDevices 缺省设备：KFD（计算接口）+ 渲染节点（缓冲分配/映射）。
// 注意：**默认 `--dev /dev` 看不到 GPU**（X3 实测），必须显式绑定这两个。
func defaultDevices() []string { return []string{"/dev/kfd", "/dev/dri/renderD128"} }

// Validate 声明校验（fail-closed：缺一律报错，绝不静默补默认值）。
func (s Spec) Validate() error {
	if s.EggID == "" {
		return fmt.Errorf("孵化声明缺 egg_id")
	}
	if s.SchemaVersion != registry.EggSchemaVersionCurrent {
		return fmt.Errorf("卵声明格式版本 %d 认不得（本端认得 %d）——拒孵，不按新格式猜着跑",
			s.SchemaVersion, registry.EggSchemaVersionCurrent)
	}
	if s.EnginePathInSpace == "" {
		return fmt.Errorf("孵化声明缺引擎在空间内的路径（engine_path_in_space）")
	}
	if !strings.HasPrefix(s.EnginePathInSpace, "/") {
		return fmt.Errorf("引擎路径必须是空间内的绝对路径（如 /engine/bin/llama-server），实得 %q", s.EnginePathInSpace)
	}
	if s.WeightPath == "" {
		return fmt.Errorf("孵化声明缺权重路径（只挂该卵自己的权重）")
	}
	if err := s.Profile.Validate(); err != nil {
		return fmt.Errorf("实测档案不可用，拒孵（§8.4 标定铁律）：%w", err)
	}
	return nil
}

// unitSafe 把卵名收敛成合法的 systemd 单元名片段（只留小写字母/数字/连字符）。
var unitSafe = regexp.MustCompile(`[^a-z0-9]+`)

// UnitName 由卵名派生**确定性**单元名：确定性 ⇒ 收卵可幂等（同名单元停两次不炸）。
func UnitName(eggID string) string {
	s := strings.ToLower(eggID)
	s = unitSafe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "egg"
	}
	if len(s) > 48 { // 单元名过长会让 systemd 报错，截断并靠前缀保持可读
		s = s[:48]
	}
	return "zerg-" + s
}

// BuildBwrapArgv 构造 bwrap 命令行（**纯函数**，可在任意平台单测）。
// 顺序遵循 X3 实测配方：先造根与链接，再挂权重/引擎/设备，最后降权进入。
func BuildBwrapArgv(s Spec) ([]string, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	argv := []string{
		// ① 全新根：只读挂系统目录（usrmerge 下 /bin /lib /lib64 /sbin 都是 /usr 的链接）
		"--ro-bind", "/usr", "/usr",
		"--symlink", "usr/bin", "/bin",
		"--symlink", "usr/sbin", "/sbin",
		"--symlink", "usr/lib", "/lib",
		"--symlink", "usr/lib64", "/lib64",
		// ② 基础伪文件系统
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		// ③ 该卵自己的权重：只挂它 ⇒ 别的模型与 /data 在空间内根本不存在
		"--ro-bind", s.WeightPath, "/models",
	}
	// ④ 引擎自己的库/构建目录（主机侧）→ /engine
	for _, root := range s.EngineRoots {
		argv = append(argv, "--ro-bind", root, "/engine")
	}
	// ⑤ 额外只读绑定（host:space）
	for _, b := range s.ExtraROBinds {
		parts := strings.SplitN(b, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("额外只读绑定必须是 <host>:<space> 形式，实得 %q", b)
		}
		argv = append(argv, "--ro-bind", parts[0], parts[1])
	}
	// ⑥ GPU：**必须显式**（默认 --dev 看不到）
	devices := s.Devices
	if len(devices) == 0 {
		devices = defaultDevices()
	}
	for _, d := range devices {
		argv = append(argv, "--dev-bind", d, d)
	}
	// ⑦ 隔离：进程视图（空间内 pid 1 就是引擎）
	argv = append(argv, "--unshare-pid", "--die-with-parent")
	// ⑧ 环境变量：按卵覆盖（**排序**保证命令行可复现）
	if len(s.Env) > 0 {
		keys := make([]string, 0, len(s.Env))
		for k := range s.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			argv = append(argv, "--setenv", k+"="+s.Env[k])
		}
	}
	// ⑨ 工作目录 + 引擎入口
	if s.WorkDir != "" {
		argv = append(argv, "--chdir", s.WorkDir)
	}
	argv = append(argv, "--", s.EnginePathInSpace)
	argv = append(argv, s.EngineArgs...)
	return argv, nil
}

// BuildSystemdRunArgv 构造 `systemd-run --user` 命令行：**它只管归属**（slice/单元/限额），
// 挂载与设备全部交给 bwrap —— 因为挂载类选项在用户实例里**静默失效**（§6.1 ②③）。
func BuildSystemdRunArgv(s Spec, unit string) ([]string, error) {
	bwrapArgv, err := BuildBwrapArgv(s)
	if err != nil {
		return nil, err
	}
	if unit == "" {
		return nil, fmt.Errorf("缺单元名")
	}
	argv := []string{
		"systemd-run", "--user",
		"--unit=" + unit,
		"--slice=" + SliceName,
		// Type=exec：execve 失败即算启动失败（否则 systemd 认为"起来了"）
		"--property=Type=exec",
		// 收卵语义：停单元时连它整棵子进程一起收（KillMode 默认 control-group 正是我们要的）
		"--property=KillMode=control-group",
	}
	if s.MemlockBytes > 0 {
		argv = append(argv, fmt.Sprintf("--property=LimitMEMLOCK=%d", s.MemlockBytes))
	}
	argv = append(argv, "--", "bwrap")
	argv = append(argv, bwrapArgv...)
	return argv, nil
}

// collectOutcome 把 `systemctl --user stop <unit>` 的结果翻译成结论（**纯函数**，便于单测）。
// X3 实测：二次 stop 返回 rc=5（单元已不存在）——这是**幂等成功**，不是错误。
func collectOutcome(rc int, stderr string) error {
	if rc == 0 {
		return nil
	}
	low := strings.ToLower(stderr)
	for _, benign := range []string{"not found", "not loaded", "no such unit", "could not be found"} {
		if strings.Contains(low, benign) {
			return nil // 幂等：本来就没有 ⇒ 收卵成功
		}
	}
	return fmt.Errorf("收卵失败（rc=%d）：%s", rc, strings.TrimSpace(stderr))
}
