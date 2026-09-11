package testutil

import "testing"

// 自证：助手"该跳过时会跳过"——跳过时 t.Skip 终止本测试，后面的 t.Error 不会执行；
// 若没跳过就会执行到 t.Error（= 助手失效，测试变红）。
func TestRequireTool(t *testing.T) {
	t.Run("缺失命令应跳过", func(t *testing.T) {
		RequireTool(t, "definitely-not-a-real-tool-xyz-20260911")
		t.Error("缺少工具时应跳过，却继续执行了")
	})
	t.Run("存在的命令不跳过", func(t *testing.T) {
		RequireTool(t, "sh")
	})
}

func TestRequireDarwinTool(t *testing.T) {
	t.Run("通用命令不跳过", func(t *testing.T) {
		RequireDarwinTool(t, "sh") // darwin 上正常执行；非 darwin 上跳过（同样算通过）
	})
}

func TestRequireEnv(t *testing.T) {
	t.Run("未设变量应跳过", func(t *testing.T) {
		RequireEnv(t, "ZERG_TEST_UNSET_20260911", "示例外部依赖")
		t.Error("未设环境变量时应跳过，却继续执行了")
	})
	t.Run("已设变量不跳过", func(t *testing.T) {
		t.Setenv("ZERG_TEST_SET_20260911", "1")
		RequireEnv(t, "ZERG_TEST_SET_20260911", "示例外部依赖")
	})
}
