package agent

import (
	"strings"
	"testing"
)

// === 合法流转测试 ===

func TestTransitionIssue_LegalOpenToQueued(t *testing.T) {
	content := "- **状态**: open"
	newContent, err := TransitionIssue(content, "open", "queued")
	if err != nil {
		t.Fatalf("legal transition open→queued should succeed: %v", err)
	}
	if !strings.Contains(newContent, "**状态**: queued") {
		t.Fatalf("expected status queued, got: %s", newContent)
	}
}

func TestTransitionIssue_LegalQueuedToRunning(t *testing.T) {
	content := "- **状态**: queued"
	newContent, err := TransitionIssue(content, "queued", "running")
	if err != nil {
		t.Fatalf("legal transition queued→running should succeed: %v", err)
	}
	if !strings.Contains(newContent, "**状态**: running") {
		t.Fatalf("expected status running, got: %s", newContent)
	}
}

func TestTransitionIssue_LegalRunningToFixing(t *testing.T) {
	content := "- **状态**: running"
	newContent, err := TransitionIssue(content, "running", "fixing")
	if err != nil {
		t.Fatalf("legal transition running→fixing should succeed: %v", err)
	}
	if !strings.Contains(newContent, "**状态**: fixing") {
		t.Fatalf("expected status fixing, got: %s", newContent)
	}
}

func TestTransitionIssue_LegalFixingToVerified(t *testing.T) {
	content := "- **状态**: fixing"
	newContent, err := TransitionIssue(content, "fixing", "verified")
	if err != nil {
		t.Fatalf("legal transition fixing→verified should succeed: %v", err)
	}
	if !strings.Contains(newContent, "**状态**: verified") {
		t.Fatalf("expected status verified, got: %s", newContent)
	}
}

func TestTransitionIssue_LegalVerifiedToDone(t *testing.T) {
	content := "- **状态**: verified"
	newContent, err := TransitionIssue(content, "verified", "done")
	if err != nil {
		t.Fatalf("legal transition verified→done should succeed: %v", err)
	}
	if !strings.Contains(newContent, "**状态**: done") {
		t.Fatalf("expected status done, got: %s", newContent)
	}
}

func TestTransitionIssue_LegalFixingToRetry(t *testing.T) {
	content := "- **状态**: fixing"
	newContent, err := TransitionIssue(content, "fixing", "retry")
	if err != nil {
		t.Fatalf("legal transition fixing→retry should succeed: %v", err)
	}
	if !strings.Contains(newContent, "**状态**: retry") {
		t.Fatalf("expected status retry, got: %s", newContent)
	}
}

// === 非法流转测试 ===

func TestTransitionIssue_IllegalOpenToFixing(t *testing.T) {
	content := "- **状态**: open"
	_, err := TransitionIssue(content, "open", "fixing")
	if err == nil {
		t.Fatal("expected error for illegal transition open→fixing")
	}
}

func TestTransitionIssue_IllegalDoneToQueued(t *testing.T) {
	content := "- **状态**: done"
	_, err := TransitionIssue(content, "done", "queued")
	if err == nil {
		t.Fatal("expected error for illegal transition done→queued (terminal state)")
	}
}

func TestTransitionIssue_IllegalDeadToRunning(t *testing.T) {
	content := "- **状态**: dead"
	_, err := TransitionIssue(content, "dead", "running")
	if err == nil {
		t.Fatal("expected error for illegal transition dead→running (terminal state)")
	}
}

func TestTransitionIssue_IllegalEscalatedToFixing(t *testing.T) {
	content := "- **状态**: escalated"
	_, err := TransitionIssue(content, "escalated", "fixing")
	if err == nil {
		t.Fatal("expected error for illegal transition escalated→fixing (terminal state)")
	}
}

func TestTransitionIssue_IllegalRunningToDone(t *testing.T) {
	content := "- **状态**: running"
	_, err := TransitionIssue(content, "running", "done")
	if err == nil {
		t.Fatal("expected error for illegal transition running→done (skip states)")
	}
}

func TestTransitionIssue_IllegalQueuedToDone(t *testing.T) {
	content := "- **状态**: queued"
	_, err := TransitionIssue(content, "queued", "done")
	if err == nil {
		t.Fatal("expected error for illegal transition queued→done (skip states)")
	}
}

// === 非法状态测试 ===

func TestTransitionIssue_InvalidFromStatus(t *testing.T) {
	content := "- **状态**: open"
	_, err := TransitionIssue(content, "bogus", "queued")
	if err == nil {
		t.Fatal("expected error for invalid source status")
	}
}

func TestTransitionIssue_InvalidToStatus(t *testing.T) {
	content := "- **状态**: open"
	_, err := TransitionIssue(content, "open", "notastate")
	if err == nil {
		t.Fatal("expected error for invalid target status")
	}
}

// === CanEscalate / CanDead 测试 ===

func TestCanEscalate_FromFixing(t *testing.T) {
	content := "- **状态**: fixing"
	newContent, err := CanEscalate(content, "fixing")
	if err != nil {
		t.Fatalf("CanEscalate from fixing should succeed: %v", err)
	}
	if !strings.Contains(newContent, "**状态**: escalated") {
		t.Fatalf("expected escalated, got: %s", newContent)
	}
}

func TestCanEscalate_AlreadyTerminal(t *testing.T) {
	content := "- **状态**: done"
	_, err := CanEscalate(content, "done")
	if err == nil {
		t.Fatal("expected error escalating from terminal state")
	}
}

func TestCanDead_FromRunning(t *testing.T) {
	content := "- **状态**: running"
	newContent, err := CanDead(content, "running")
	if err != nil {
		t.Fatalf("CanDead from running should succeed: %v", err)
	}
	if !strings.Contains(newContent, "**状态**: dead") {
		t.Fatalf("expected dead, got: %s", newContent)
	}
}

// === 全链路流转测试 ===

func TestTransitionIssue_FullChain(t *testing.T) {
	steps := []struct {
		from, to string
	}{
		{"open", "queued"},
		{"queued", "running"},
		{"running", "fixing"},
		{"fixing", "verified"},
		{"verified", "done"},
	}
	content := "- **状态**: open"
	for _, step := range steps {
		var err error
		content, err = TransitionIssue(content, step.from, step.to)
		if err != nil {
			t.Fatalf("step %s→%s failed: %v", step.from, step.to, err)
		}
	}
	if !strings.Contains(content, "**状态**: done") {
		t.Fatalf("expected final status done, got: %s", content)
	}
}

func TestTransitionIssue_RetryBackToQueued(t *testing.T) {
	// fixing → retry → queued → running → fixing → verified → done
	content := "- **状态**: fixing"
	content, err := TransitionIssue(content, "fixing", "retry")
	if err != nil {
		t.Fatalf("fixing→retry failed: %v", err)
	}
	content, err = TransitionIssue(content, "retry", "queued")
	if err != nil {
		t.Fatalf("retry→queued failed: %v", err)
	}
	content, err = TransitionIssue(content, "queued", "running")
	if err != nil {
		t.Fatalf("queued→running failed: %v", err)
	}
	content, err = TransitionIssue(content, "running", "fixing")
	if err != nil {
		t.Fatalf("running→fixing failed: %v", err)
	}
	content, err = TransitionIssue(content, "fixing", "verified")
	if err != nil {
		t.Fatalf("fixing→verified failed: %v", err)
	}
	content, err = TransitionIssue(content, "verified", "done")
	if err != nil {
		t.Fatalf("verified→done failed: %v", err)
	}
	if !strings.Contains(content, "**状态**: done") {
		t.Fatalf("expected done, got: %s", content)
	}
}
