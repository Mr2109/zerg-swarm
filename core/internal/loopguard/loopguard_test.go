package loopguard

import (
	"strings"
	"testing"
)

func TestDetectTripleRepeat(t *testing.T) {
	g := New(Config{})
	args := map[string]any{"path": "a.go"}
	for i := 0; i < 3; i++ {
		g.Record("read", args)
		if i < 2 {
			if ok, _ := g.Detect(); ok {
				t.Fatalf("第 %d 次不应检测到", i+1)
			}
		}
	}
	ok, reason := g.Detect()
	if !ok || !strings.Contains(reason, "重复") {
		t.Fatalf("3 次重复应检测到: %v %s", ok, reason)
	}
}

func TestDetectAlternate(t *testing.T) {
	g := New(Config{})
	g.Record("read", map[string]any{"path": "a"})
	g.Record("grep", map[string]any{"pattern": "x"})
	if ok, _ := g.Detect(); ok {
		t.Fatal("两次不同不应检测到")
	}
	g.Record("read", map[string]any{"path": "a"})
	g.Record("grep", map[string]any{"pattern": "x"})
	ok, reason := g.Detect()
	if !ok || !strings.Contains(reason, "交替") {
		t.Fatalf("ABAB 应检测到交替: %v %s", ok, reason)
	}
}

func TestBuildGuideEscalation(t *testing.T) {
	g := New(Config{})
	msg, upgrade := g.BuildGuide("r", nil)
	if upgrade {
		t.Fatal("第 1 次引导不应升级")
	}
	if !strings.Contains(msg, "换一种方法") {
		t.Fatal("第 1 次引导应含换招提示")
	}
	_, upgrade = g.BuildGuide("r", nil)
	if !upgrade {
		t.Fatal("第 2 次应升级收尾")
	}
}
