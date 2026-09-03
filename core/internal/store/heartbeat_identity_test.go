package store

import (
	"encoding/json"
	"testing"
)

// 心跳的身份字段是"版本矩阵"的数据来源（自动升级 L3）——线上形状必须稳定，
// 否则子端升级后主控读不到身份，"混版机群"就不可见。
func TestHeartbeatIdentityRoundTrip(t *testing.T) {
	raw := []byte(`{"machine":"x3","code_version":"2.5.9","code_sha":"abc1234+a1b2"}`)
	var req HeartbeatRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if req.CodeVersion != "2.5.9" || req.CodeSHA != "abc1234+a1b2" {
		t.Fatalf("身份字段未解析: version=%q sha=%q", req.CodeVersion, req.CodeSHA)
	}
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if back["code_sha"] != "abc1234+a1b2" || back["code_version"] != "2.5.9" {
		t.Fatalf("JSON 键名不符（线上契约）: %v", back)
	}
}

// 心跳带上身份 -> 主控快照必须记住（版本矩阵据此判断混版机群）。
func TestReceiveHeartbeatKeepsIdentity(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "x3", Healthy: true, CodeVersion: "2.5.9", CodeSHA: "abc1234"})
	snap := s.GetSnapshot("x3")
	if snap == nil {
		t.Fatal("快照未创建")
	}
	if snap.CodeVersion != "2.5.9" || snap.CodeSHA != "abc1234" {
		t.Fatalf("身份未落库: version=%q sha=%q", snap.CodeVersion, snap.CodeSHA)
	}
	// 升级后再报（同机器）-> 必须覆盖为新身份，否则矩阵永远滞后
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "x3", Healthy: true, CodeVersion: "2.6.0", CodeSHA: "def5678"})
	if got := s.GetSnapshot("x3"); got.CodeSHA != "def5678" || got.CodeVersion != "2.6.0" {
		t.Fatalf("身份未随心跳更新: version=%q sha=%q", got.CodeVersion, got.CodeSHA)
	}
}
