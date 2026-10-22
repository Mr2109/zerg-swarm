package compress

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Sink —— 审计落点（对话进 chat_obs、任务进任务日志；本包只给统一的写入面）。
type Sink interface {
	Emit(ev Event) error
}

// JSONLSink —— 一行一个事件（append-only，天然并发安全靠调用方串行或 mu）。
type JSONLSink struct {
	mu sync.Mutex
	w  io.Writer
}

func NewJSONLSink(w io.Writer) *JSONLSink { return &JSONLSink{w: w} }

func (s *JSONLSink) Emit(ev Event) error {
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.w.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

// EventOf —— 从 Result 生成一条 completed 事件的便捷构造（三维字段直接搬）。
func EventOf(res Result, model string) Event {
	return Event{
		Phase:      "completed",
		At:         time.Now().UTC(),
		Trigger:    res.Trigger,
		Mechanism:  res.Mechanism,
		Aggressive: res.Aggressive,
		Model:      model,
		Before:     res.Before,
		After:      res.After,
		Fallback:   res.Fallback,
		Reason:     res.Reason,
	}
}
