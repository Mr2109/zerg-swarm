// p2_assert_test.go —— P2 辅助：测试内源码结构断言用的文件读取。
package backend

import (
	"os"
	"path/filepath"
)

// readFileForAssertion 读本包内指定源文件内容（结构性断言用——防判定路径被手滑接线）。
func readFileForAssertion(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(".", name))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
