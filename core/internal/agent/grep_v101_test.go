package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGrepHidden — G3: hidden=true 能搜到隐藏文件，默认搜不到
func TestGrepHidden(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET_KEY=abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte("SECRET_KEY=abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ec := &ExecContext{WorkDir: dir}

	// hidden=false（默认）—— .env 搜不到，config.go 能搜到
	out, err := ec.executeGrep(context.Background(), ".", "SECRET_KEY", map[string]any{"hidden": false}, nil)
	if err != nil {
		t.Fatalf("grep hidden=false: %v", err)
	}
	if !strings.Contains(out, "config.go") {
		t.Errorf("普通文件应搜到——got:\n%s", out)
	}
	if strings.Contains(out, ".env") {
		t.Errorf("hidden=false 时 .env 不应搜到——got:\n%s", out)
	}

	// hidden=true—— .env 能搜到
	outH, err := ec.executeGrep(context.Background(), ".", "SECRET_KEY", map[string]any{"hidden": true}, nil)
	if err != nil {
		t.Fatalf("grep hidden=true: %v", err)
	}
	if !strings.Contains(outH, ".env") {
		t.Errorf("hidden=true 时 .env 应搜到——got:\n%s", outH)
	}
}
