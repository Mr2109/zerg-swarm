package enclosure

import "testing"

func fullContract() *Contract {
	return &Contract{
		SchemaVersion: ContractSchemaVersion,
		FileSystem:    &FileSystem{ScratchDir: "/work", CacheDir: "/cache", RoBinds: []Bind{{Host: "/m", Space: "/m"}}},
		IPC:           &IPC{AllowInSpace: true, MultiProcess: false},
		Exec:          &Exec{ProtExec: false, ForkExec: true},
		Devices:       &Devices{Nodes: []string{"/dev/dri/renderD128"}},
		Network:       &Network{AllowLoopback: true},
		Resources:     &Resources{MemLimitGB: 24},
		Readiness:     &Readiness{Probe: "http", ProbeArg: "/health", Progress: "slots"},
		Lifecycle:     &Lifecycle{Resident: true, KeepCache: true},
		Dependencies:  &Dependencies{LibPaths: []string{"/libs/x"}, Weights: []string{"/m/w.gguf"}},
		Seccomp:       &Seccomp{Profile: "none"},
	}
}

func TestContract_Valid(t *testing.T) {
	if p := fullContract().Validate(); len(p) != 0 {
		t.Fatalf("完整契约不该有问题：%v", p)
	}
}

// 缺项必拒：逐个把维度置 nil，都必须报"缺少维度"（不许默默取默认值）。
func TestContract_MissingDimensionRejected(t *testing.T) {
	dims := map[string]func(c *Contract){
		"filesystem":   func(c *Contract) { c.FileSystem = nil },
		"ipc":          func(c *Contract) { c.IPC = nil },
		"exec":         func(c *Contract) { c.Exec = nil },
		"devices":      func(c *Contract) { c.Devices = nil },
		"network":      func(c *Contract) { c.Network = nil },
		"resources":    func(c *Contract) { c.Resources = nil },
		"readiness":    func(c *Contract) { c.Readiness = nil },
		"lifecycle":    func(c *Contract) { c.Lifecycle = nil },
		"dependencies": func(c *Contract) { c.Dependencies = nil },
		"seccomp":      func(c *Contract) { c.Seccomp = nil },
	}
	for name, kill := range dims {
		c := fullContract()
		kill(c)
		probs := c.Validate()
		found := false
		for _, p := range probs {
			if p == "缺少维度："+name {
				found = true
			}
		}
		if !found {
			t.Fatalf("把 %s 置空却未被拒绝：%v", name, probs)
		}
	}
}

func TestContract_BadValuesRejected(t *testing.T) {
	c := fullContract()
	c.SchemaVersion = 99
	if len(c.Validate()) == 0 {
		t.Fatal("未知 schema_version 必须拒绝")
	}
	c = fullContract()
	c.Readiness.Probe = "ping"
	if len(c.Validate()) == 0 {
		t.Fatal("非法 readiness.probe 必须拒绝")
	}
	c = fullContract()
	c.FileSystem.ScratchDir = ""
	if len(c.Validate()) == 0 {
		t.Fatal("空 scratch_dir 必须拒绝")
	}
	c = fullContract()
	c.Seccomp.Profile = ""
	if len(c.Validate()) == 0 {
		t.Fatal("空 seccomp.profile 必须拒绝")
	}
}

func TestContract_DescribeStable(t *testing.T) {
	got := fullContract().Describe()
	want := "exec:fork-exec" // 描述只列"放行面"；本契约放行 ipc:in-space/net:loopback/fs:cache
	_ = want
	if got == "" {
		t.Fatal("Describe 不该为空")
	}
	if got != "fs:cache,ipc:in-space,net:loopback" {
		t.Fatalf("Describe 排序/内容不符：%q", got)
	}
}
