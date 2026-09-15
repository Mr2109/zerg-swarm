// contract.go —— 引擎能力契约（设计稿 §10.2 的十个维度）。
//
// 为什么是"契约"而不是"一套固定策略"（§10.3 第 1 条）：卵里的引擎与模型形态各异
// （llama.cpp 单进程 / vLLM 多进程 / JIT 引擎 / safetensors 分片 / 多组件…），
// 所以 **接入新引擎 = 产一枚新卵（自带它的契约）**，不改沙箱代码。
//
// 每个维度都是**指针**：nil 表示"未声明" ⇒ Validate 必须拒绝（缺项必拒，不许默默取默认值）。
package enclosure

import (
	"fmt"
	"sort"
	"strings"
)

// ContractSchemaVersion 契约结构版本。改字段语义时必须 +1（旧档案可读是硬要求）。
const ContractSchemaVersion = 1

// Contract 引擎能力契约。十维度见设计稿 §10.2 表。
type Contract struct {
	SchemaVersion int           `json:"schema_version" yaml:"schema_version"`
	FileSystem    *FileSystem   `json:"filesystem,omitempty" yaml:"filesystem,omitempty"`
	IPC           *IPC          `json:"ipc,omitempty" yaml:"ipc,omitempty"`
	Exec          *Exec         `json:"exec,omitempty" yaml:"exec,omitempty"`
	Devices       *Devices      `json:"devices,omitempty" yaml:"devices,omitempty"`
	Network       *Network      `json:"network,omitempty" yaml:"network,omitempty"`
	Resources     *Resources    `json:"resources,omitempty" yaml:"resources,omitempty"`
	Readiness     *Readiness    `json:"readiness,omitempty" yaml:"readiness,omitempty"`
	Lifecycle     *Lifecycle    `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	Dependencies  *Dependencies `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Seccomp       *Seccomp      `json:"seccomp,omitempty" yaml:"seccomp,omitempty"`
}

// FileSystem 文件系统维度：多段只读绑定 + 可写区划分。
type FileSystem struct {
	RoBinds    []Bind   `json:"ro_binds" yaml:"ro_binds"`                         // 多段只读绑定（多文件权重/多组件必须支持）
	ScratchDir string   `json:"scratch_dir" yaml:"scratch_dir"`                   // 空间内可写临时区（**每次孵化前清理**，§13.1）
	CacheDir   string   `json:"cache_dir,omitempty" yaml:"cache_dir,omitempty"`   // 可写缓存区（**跨启动保留**，JIT/编译产物）
	DenyWrite  []string `json:"deny_write,omitempty" yaml:"deny_write,omitempty"` // 禁写清单（权重必须只读）
}

// Bind 一段只读绑定：宿主路径 → 空间内落点。
type Bind struct {
	Host  string `json:"host" yaml:"host"`
	Space string `json:"space" yaml:"space"`
}

// IPC IPC/进程维度：多进程引擎（vLLM/Ray 类）的必要条件。
type IPC struct {
	AllowInSpace bool `json:"allow_in_space" yaml:"allow_in_space"` // 空间内 shm/unix socket/memfd
	MultiProcess bool `json:"multi_process" yaml:"multi_process"`   // 是否允许多进程
}

// Exec 可执行性维度：JIT 引擎（Triton/torch.compile）的必要条件。
type Exec struct {
	ProtExec  bool     `json:"prot_exec" yaml:"prot_exec"` // 允许可执行内存（W^X 路径，§10.8）
	ForkExec  bool     `json:"fork_exec" yaml:"fork_exec"` // 允许 fork/exec 子进程
	AllowList []string `json:"allow_list,omitempty" yaml:"allow_list,omitempty"`
}

// Devices 设备维度：GPU 直跑；IoctlLevel 留给 §10.5 第 4 项实测后再收严。
type Devices struct {
	Nodes      []string `json:"nodes" yaml:"nodes"` // 如 /dev/kfd、/dev/dri/renderD128、/dev/nvidia0
	IoctlLevel string   `json:"ioctl_level,omitempty" yaml:"ioctl_level,omitempty"`
}

// Network 网络维度：**默认 deny**，白名单是"例外"而非开关（§10.6 结论 2）。
type Network struct {
	AllowLoopback bool     `json:"allow_loopback" yaml:"allow_loopback"` // 多进程引擎内部通信常用
	EgressAllow   []string `json:"egress_allow,omitempty" yaml:"egress_allow,omitempty"`
}

// Resources 资源维度：显存/GTT 是"可测的部分"，测不到就不写。
type Resources struct {
	CPUQuota   float64 `json:"cpu_quota,omitempty" yaml:"cpu_quota,omitempty"`
	MemLimitGB float64 `json:"mem_limit_gb,omitempty" yaml:"mem_limit_gb,omitempty"`
	GTTLimitGB float64 `json:"gtt_limit_gb,omitempty" yaml:"gtt_limit_gb,omitempty"`
	MaxProcs   int     `json:"max_procs,omitempty" yaml:"max_procs,omitempty"`
	MaxFDs     int     `json:"max_fds,omitempty" yaml:"max_fds,omitempty"`
}

// Readiness 就绪与活性维度：看门狗要按引擎取证据（§四：判活只看证据）。
type Readiness struct {
	Probe    string `json:"probe" yaml:"probe"`                           // 就绪判据：http|stdio|log|file
	ProbeArg string `json:"probe_arg" yaml:"probe_arg"`                   // 端点/正则/路径
	Progress string `json:"progress,omitempty" yaml:"progress,omitempty"` // 进度证据来源（主证据）
}

// Lifecycle 生命周期维度。
type Lifecycle struct {
	Resident    bool `json:"resident" yaml:"resident"` // 常驻 or 按需
	IdleUnloadS int  `json:"idle_unload_s,omitempty" yaml:"idle_unload_s,omitempty"`
	KeepCache   bool `json:"keep_cache" yaml:"keep_cache"` // 缓存是否跨启动保留
}

// Dependencies 依赖维度：运行时依赖树（libs/数据文件/编译产物）。
type Dependencies struct {
	LibPaths []string `json:"lib_paths,omitempty" yaml:"lib_paths,omitempty"`
	Weights  []string `json:"weights,omitempty" yaml:"weights,omitempty"`
}

// Seccomp seccomp 基线维度：**收紧前必须先重测 JIT**（§10.5 第 5 项）。
type Seccomp struct {
	Profile string `json:"profile" yaml:"profile"` // none|engine-specific 名称
}

// Validate 缺项必拒：任一维度为 nil ⇒ 返回问题列表；未知 schema 版本同样拒绝。
func (c *Contract) Validate() []string {
	var probs []string
	if c == nil {
		return []string{"contract 为 nil"}
	}
	if c.SchemaVersion != ContractSchemaVersion {
		probs = append(probs, fmt.Sprintf("schema_version=%d 不受支持（当前 %d）", c.SchemaVersion, ContractSchemaVersion))
	}
	// 注意：**不能**把指针赋给 any 再判 == nil —— 带类型的 nil 指针装进接口后不等于 nil
	// （这个坑正是被本包的负例用例抓出来的 ⇒ 缺项必拒曾静默失效）。
	dims := []struct {
		name  string
		isNil bool
	}{
		{"filesystem", c.FileSystem == nil}, {"ipc", c.IPC == nil}, {"exec", c.Exec == nil},
		{"devices", c.Devices == nil}, {"network", c.Network == nil}, {"resources", c.Resources == nil},
		{"readiness", c.Readiness == nil}, {"lifecycle", c.Lifecycle == nil},
		{"dependencies", c.Dependencies == nil}, {"seccomp", c.Seccomp == nil},
	}
	for _, d := range dims {
		if d.isNil {
			probs = append(probs, "缺少维度："+d.name)
		}
	}
	if c.FileSystem != nil {
		if c.FileSystem.ScratchDir == "" {
			probs = append(probs, "filesystem.scratch_dir 为空（空间内可写临时区必须显式声明）")
		}
		for i, b := range c.FileSystem.RoBinds {
			if b.Host == "" || b.Space == "" {
				probs = append(probs, fmt.Sprintf("filesystem.ro_binds[%d] 的 host/space 不完整", i))
			}
		}
	}
	if c.Readiness != nil {
		switch c.Readiness.Probe {
		case "http", "stdio", "log", "file":
		default:
			probs = append(probs, "readiness.probe 非法（须为 http|stdio|log|file）："+c.Readiness.Probe)
		}
	}
	if c.Seccomp != nil && c.Seccomp.Profile == "" {
		probs = append(probs, "seccomp.profile 为空（写 none 也要显式声明）")
	}
	sort.Strings(probs)
	return probs
}

// Describe 供日志/观测面：把放行面写成稳定可比较的短清单（排序后）。
func (c *Contract) Describe() string {
	if c == nil {
		return "(nil)"
	}
	var parts []string
	if c.IPC != nil && c.IPC.AllowInSpace {
		parts = append(parts, "ipc:in-space")
	}
	if c.IPC != nil && c.IPC.MultiProcess {
		parts = append(parts, "ipc:multi-proc")
	}
	if c.Exec != nil && c.Exec.ProtExec {
		parts = append(parts, "exec:prot-exec")
	}
	if c.Network != nil && c.Network.AllowLoopback {
		parts = append(parts, "net:loopback")
	}
	if c.FileSystem != nil && c.FileSystem.CacheDir != "" {
		parts = append(parts, "fs:cache")
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
