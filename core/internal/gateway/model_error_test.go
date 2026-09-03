package gateway

import (
	"errors"
	"net"
	"testing"
	"time"
)

// TestIsModelError — v2.5.5 T1：连接层失败 ≠ 模型失败（不熔断——偶发网络重试就好）
func TestIsModelError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"网络超时", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("timeout")}, false},
		{"连接拒绝", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, false},
		{"超时错误", &timeoutErr{}, false},
		{"EOF截断", &net.OpError{Op: "read", Net: "tcp", Err: errors.New("EOF")}, false},
		{"URL错误", errors.New("unsupported protocol scheme"), true},
		{"通用错误", errors.New("some other error"), true},
	}
	for _, c := range cases {
		got := isModelError(c.err)
		if got != c.want {
			t.Errorf("%s: isModelError(%v) = %v——want %v", c.name, c.err, got, c.want)
		}
	}
	t.Log("✅ 连接层失败不熔断——模型错误才熔断")
}

// timeoutErr 模拟 net.Error（超时）
type timeoutErr struct{}

func (e *timeoutErr) Error() string   { return "timeout" }
func (e *timeoutErr) Timeout() bool   { return true }
func (e *timeoutErr) Temporary() bool { return true }

var _ = time.Now // 占位（time 用于 future）
