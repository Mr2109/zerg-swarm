package config

import (
	"os"
	"path/filepath"
	"testing"
)

// boolPtr 返回 bool 指针
func boolPtr(b bool) *bool {
	return &b
}

// collectFields 收集错误/警告的字段名
func collectFields(errors []ValidationError) []string {
	fields := make([]string, 0, len(errors))
	for _, e := range errors {
		fields = append(fields, e.Field)
	}
	return fields
}

// validCandidate 返回一个基本合法的 ModelCandidate
func validCandidate() ModelCandidate {
	return ModelCandidate{
		Name:        "test-model",
		Family:      "llama",
		Host:        "local",
		Backend:     "llama-server",
		File:        filepath.Join(os.TempDir(), "test_model.gguf"),
		MemGb:       20,
		SSD:         false,
		CtxWindow:   32768,
		Modality:    "text",
		ToolSupport: boolPtr(false),
		Arch:        "llama",
	}
}

// TestValidate_MissingRequiredFields 必填字段缺失
func TestValidate_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		candidate ModelCandidate
		wantFatal bool
		wantField string
	}{
		{
			candidate: func() ModelCandidate {
				c := validCandidate()
				c.Name = ""
				return c
			}(),
			wantFatal: true, wantField: "name",
		},
		{
			candidate: func() ModelCandidate {
				c := validCandidate()
				c.Family = ""
				return c
			}(),
			// 存量兼容：family 空但有 name → 从名称推断（Warn 不 Fatal）
			wantFatal: false,
		},
		{
			candidate: func() ModelCandidate {
				c := validCandidate()
				c.Host = ""
				return c
			}(),
			wantFatal: true, wantField: "host",
		},
		{
			candidate: func() ModelCandidate {
				c := validCandidate()
				c.Backend = ""
				return c
			}(),
			wantFatal: true, wantField: "backend",
		},
		{
			candidate: func() ModelCandidate {
				c := validCandidate()
				c.File = ""
				return c
			}(),
			wantFatal: true, wantField: "file",
		},
		{
			candidate: func() ModelCandidate {
				c := validCandidate()
				c.Modality = ""
				return c
			}(),
			// 存量兼容：modality 空 → Warn（默认 text）不 Fatal
			wantFatal: false,
		},
		{
			candidate: func() ModelCandidate {
				c := validCandidate()
				c.ToolSupport = nil
				return c
			}(),
			// 存量兼容：tool_support 空 → Warn（默认 false）不 Fatal
			wantFatal: false,
		},
		{
			candidate: func() ModelCandidate {
				c := validCandidate()
				c.MemGb = -1
				return c
			}(),
			wantFatal: true, wantField: "mem_gb",
		},
		{
			candidate: func() ModelCandidate {
				c := validCandidate()
				c.CtxWindow = 0
				return c
			}(),
			// 存量兼容：ctx_window 空 → Warn（默认 4096）不 Fatal
			wantFatal: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// name 参数：用例 wantField=="name" 时传空（测 name 必填），否则用固定名
			validateName := "test-model"
			if tc.wantField == "name" {
				validateName = ""
			}
			result := Validate(validateName, tc.candidate)
			if tc.wantFatal {
				if !result.HasFatal() {
					t.Fatalf("期望 %s 触发 Fatal，但通过了", tc.name)
				}
				found := false
				for _, e := range result.Errors {
					if e.Field == tc.wantField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("期望 %q 的 Fatal 错误，得到: %v", tc.wantField, collectFields(result.Errors))
				}
			} else if result.HasFatal() {
				t.Errorf("用例 %s 不应有 Fatal（存量兼容），得到: %v", tc.name, collectFields(result.Errors))
			}
		})
	}
}

// TestValidate_FileNotFound_Local 文件不存在 -> Fatal
func TestValidate_FileNotFound_Local(t *testing.T) {
	c := validCandidate()
	c.Host = "local"
	c.File = "/nonexistent/path/model.gguf"
	result := Validate("test-model", c)
	if !result.HasFatal() {
		t.Fatal("期望 Fatal 错误，但通过了")
	}
	found := false
	for _, e := range result.Errors {
		if e.Field == "file" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("期望 file 的 Fatal 错误，得到: %v", collectFields(result.Errors))
	}
}

// TestValidate_FileNotChecked_RemoteHost 远程主机不检查文件存在性
func TestValidate_FileNotChecked_RemoteHost(t *testing.T) {
	c := validCandidate()
	c.Host = "x3"
	c.File = "/nonexistent/path/model.gguf"
	result := Validate("test-model", c)
	if result.HasFatal() {
		t.Errorf("远程主机不应检查文件存在性，得到: %v", collectFields(result.Errors))
	}
}

// TestValidate_MemGBOver128NoSSD mem_gb>128 无 SSD -> Warn
func TestValidate_MemGBOver128NoSSD(t *testing.T) {
	c := validCandidate()
	c.MemGb = 256
	c.SSD = false
	result := Validate("test-model", c)
	if result.HasFatal() {
		t.Fatalf("不应有 Fatal 错误，得到: %v", collectFields(result.Errors))
	}
	found := false
	for _, w := range result.Warnings {
		if w.Field == "mem_gb" && w.Level == WarnLevel {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("期望 mem_gb 的 Warn 警告，得到: %v", collectFields(result.Warnings))
	}
}

// TestValidate_MemGBOver128WithSSD mem_gb>128 有 SSD -> 无 Warn
func TestValidate_MemGBOver128WithSSD(t *testing.T) {
	c := validCandidate()
	c.MemGb = 256
	c.SSD = true
	result := Validate("test-model", c)
	if result.HasFatal() {
		t.Fatalf("不应有 Fatal 错误，得到: %v", collectFields(result.Errors))
	}
	for _, w := range result.Warnings {
		if w.Field == "mem_gb" {
			t.Error("有 SSD 时不应警告 mem_gb")
		}
	}
}

// TestValidate_FullValidCandidate 正常完整配置 -> 通过
func TestValidate_FullValidCandidate(t *testing.T) {
	c := validCandidate()
	c.ToolSupport = boolPtr(true)
	c.Description = "测试模型描述"
	c.Arch = "llama"
	c.Family = "llama"
	result := Validate("test-model", c)
	if result.HasFatal() {
		t.Fatalf("完整配置应通过，得到: %v", collectFields(result.Errors))
	}
}

// TestValidate_UnknownHost 未知主机 -> Warn
func TestValidate_UnknownHost(t *testing.T) {
	c := validCandidate()
	c.Host = "unknown-host"
	c.ToolSupport = boolPtr(false)
	c.Description = "测试"
	c.Arch = "llama"
	result := Validate("test-model", c)
	if result.HasFatal() {
		t.Fatalf("不应有 Fatal 错误，得到: %v", collectFields(result.Errors))
	}
	found := false
	for _, w := range result.Warnings {
		if w.Field == "host" && w.Level == WarnLevel {
			found = true
			break
		}
	}
	if !found {
		t.Error("期望 host 的 Warn 警告")
	}
}

// TestValidate_KnownHostsNoWarning 已知主机不触发 V011
func TestValidate_KnownHostsNoWarning(t *testing.T) {
	knownHosts := []string{"local", "x3", "mini1", "mini2", "mini3"}
	for _, host := range knownHosts {
		t.Run(host, func(t *testing.T) {
			c := validCandidate()
			c.Host = host
			c.ToolSupport = boolPtr(false)
			c.Description = "测试"
			c.Arch = "llama"
			result := Validate("test-model", c)
			for _, w := range result.Warnings {
				if w.Field == "host" {
					t.Errorf("host %q 是已知主机，不应警告: %s", host, w.Message)
				}
			}
		})
	}
}

// TestValidate_InfoLevelWarnings Info 级别提示
func TestValidate_InfoLevelWarnings(t *testing.T) {
	c := validCandidate()
	c.ToolSupport = boolPtr(false)
	// Description/Arch 留空——触发 V013（description 空）/V015（arch 空）Info 提示
	result := Validate("test-model", c)
	if result.HasFatal() {
		t.Fatalf("不应有 Fatal 错误，得到: %v", collectFields(result.Errors))
	}
	infoCount := 0
	for _, w := range result.Warnings {
		if w.Level == InfoLevel {
			infoCount++
		}
	}
	if infoCount == 0 {
		t.Error("期望至少一个 Info 级别提示")
	}
}

// TestValidate_MemGBNegative 负数 mem_gb -> Fatal
func TestValidate_MemGBNegative(t *testing.T) {
	c := validCandidate()
	c.MemGb = -1
	c.ToolSupport = boolPtr(false)
	result := Validate("test-model", c)
	if !result.HasFatal() {
		t.Fatal("mem_gb 为负数应触发 Fatal")
	}
	found := false
	for _, e := range result.Errors {
		if e.Field == "mem_gb" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("期望 mem_gb 的 Fatal 错误，得到: %v", collectFields(result.Errors))
	}
}

// TestValidate_CtxWindowZero ctx_window=0 -> Warn（存量兼容默认 4096）
func TestValidate_CtxWindowZero(t *testing.T) {
	c := validCandidate()
	c.CtxWindow = 0
	c.ToolSupport = boolPtr(false)
	result := Validate("test-model", c)
	if result.HasFatal() {
		t.Fatal("ctx_window=0 存量兼容应只有 Warn（默认 4096）")
	}
	// 存量兼容：ctx_window=0 → Warn（默认 4096）——检查 Warnings 里有没有 ctx_window
	foundWarn := false
	for _, w := range result.Warnings {
		if w.Field == "ctx_window" {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Errorf("期望 ctx_window 的 Warn 提示，得到 warnings: %v", collectFields(result.Warnings))
	}
}

// TestValidationLevel_String 校验级别字符串
func TestValidationLevel_String(t *testing.T) {
	if FatalLevel.String() != "Fatal" {
		t.Errorf("FatalLevel.String() = %q, 期望 %q", FatalLevel.String(), "Fatal")
	}
	if WarnLevel.String() != "Warn" {
		t.Errorf("WarnLevel.String() = %q, 期望 %q", WarnLevel.String(), "Warn")
	}
	if InfoLevel.String() != "Info" {
		t.Errorf("InfoLevel.String() = %q, 期望 %q", InfoLevel.String(), "Info")
	}
	if ValidationLevel(99).String() != "Unknown" {
		t.Errorf("未知级别 String() = %q, 期望 %q", ValidationLevel(99).String(), "Unknown")
	}
}

// TestValidationResult_HasFatal HasFatal 方法
func TestValidationResult_HasFatal(t *testing.T) {
	empty := ValidationResult{}
	if empty.HasFatal() {
		t.Error("空结果不应有 Fatal")
	}
	withErr := ValidationResult{
		Errors: []ValidationError{{Field: "name", Level: FatalLevel, Message: "test"}},
	}
	if !withErr.HasFatal() {
		t.Error("有错误的结果应有 Fatal")
	}
}

// TestValidate_AllFatalFields 所有 Fatal 字段同时缺失（核心 4 个：host/backend/file/mem_gb）
func TestValidate_AllFatalFields(t *testing.T) {
	c := ModelCandidate{}
	result := Validate("test-model", c)
	if !result.HasFatal() {
		t.Fatal("全空配置应有多条 Fatal 错误")
	}
	if len(result.Errors) < 4 {
		t.Errorf("全空配置应触发至少 4 个 Fatal（host/backend/file/mem_gb），得到 %d: %v", len(result.Errors), collectFields(result.Errors))
	}
}

// TestMain 创建临时文件供本地文件校验使用
func TestMain(m *testing.M) {
	tmpFile := filepath.Join(os.TempDir(), "test_model.gguf")
	os.WriteFile(tmpFile, []byte("dummy"), 0644)
	defer os.Remove(tmpFile)
	code := m.Run()
	os.Exit(code)
}
