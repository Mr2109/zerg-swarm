package store

import (
	"encoding/json"
	"testing"
)

// 心跳的身份字段是"版本矩阵"的数据来源（自动升级 L3）——线上形状必须稳定，
// 否则子端升级后主控读不到身份，"混版机群"就不可见。
func TestHeartbeatIdentityRoundTrip(t *testing.T) {
	raw := []byte(`{"machine":"node-a","code_version":"2.5.9","code_sha":"abc1234+a1b2"}`)
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
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "node-a", Healthy: true, CodeVersion: "2.5.9", CodeSHA: "abc1234"})
	snap := s.GetSnapshot("node-a")
	if snap == nil {
		t.Fatal("快照未创建")
	}
	if snap.CodeVersion != "2.5.9" || snap.CodeSHA != "abc1234" {
		t.Fatalf("身份未落库: version=%q sha=%q", snap.CodeVersion, snap.CodeSHA)
	}
	// 升级后再报（同机器）-> 必须覆盖为新身份，否则矩阵永远滞后
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "node-a", Healthy: true, CodeVersion: "2.6.0", CodeSHA: "def5678"})
	if got := s.GetSnapshot("node-a"); got.CodeSHA != "def5678" || got.CodeVersion != "2.6.0" {
		t.Fatalf("身份未随心跳更新: version=%q sha=%q", got.CodeVersion, got.CodeSHA)
	}
}

// 同源判定（GAP-20261013-09 · 候选 c）：心跳带上**源提交**，主控快照必须记住它；
// ★ 空串（该机没带）必须**原样为空** —— ✗ 不许拿 code_sha 顶上冒充源提交（两套 git 命名空间）。
func TestHeartbeatOriginRoundTrip(t *testing.T) {
	raw := []byte(`{"machine":"edge-node","code_sha":"ca565e92","origin_sha":"3b04f6ab"}`)
	var req HeartbeatRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if req.OriginSHA != "3b04f6ab" {
		t.Fatalf("origin_sha 未解析: %q", req.OriginSHA)
	}
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if back["origin_sha"] != "3b04f6ab" {
		t.Fatalf("JSON 键名不符（线上契约）: %v", back)
	}
	if back["code_sha"] != "ca565e92" {
		t.Fatalf("code_sha 必须与 origin_sha 各归各: %v", back)
	}
}

// 快照侧：带上就记住（且不顶替 code_sha）；下一次**不带** ⇒ 必须回到空串（判不了），✗ 不残留旧值。
func TestReceiveHeartbeatKeepsOrigin(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "edge-node", Healthy: true, CodeVersion: "2.5.15",
		CodeSHA: "ca565e92", OriginSHA: "3b04f6ab"})
	snap := s.GetSnapshot("edge-node")
	if snap == nil {
		t.Fatal("快照未创建")
	}
	if snap.OriginSHA != "3b04f6ab" {
		t.Fatalf("源提交未落库: %q", snap.OriginSHA)
	}
	if snap.CodeSHA != "ca565e92" {
		t.Fatalf("origin_sha 不得顶替 code_sha（两套命名空间各归各）: %q", snap.CodeSHA)
	}
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "edge-node", Healthy: true, CodeVersion: "2.6.0"})
	if got := s.GetSnapshot("edge-node").OriginSHA; got != "" {
		t.Fatalf("没带源提交时必须如实留空（缺 ≠ 一致），实得 %q", got)
	}
}
