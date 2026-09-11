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
