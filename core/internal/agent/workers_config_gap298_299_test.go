package agent

import "testing"

// GAP-20260927-299：yaml 空 hostname 条目不得命中任意主机（strings.Contains(x,"") 恒真）。
// 改后：空串两侧都不参与匹配 ⇒ 回退默认。改前：空条目误命中 ⇒ 漏出该条目的 max_workers。
func TestGAP299_EmptyDeviceHostNoMatch(t *testing.T) {
	seven := 7
	devices := []DeviceConfig{{Hostname: "", MaxWorkers: &seven}}
	got, explicit := resolveByDevice(devices, "somehost", map[string]int{}, 42)
	if got != 42 || explicit {
		t.Fatalf("空 hostname 条目误命中：got=(%d,%v)，want (42,false)", got, explicit)
	}
}

// GAP-20260927-298：hostname 取失败的「错误分支」（hostname=""）不得漏出别机的值（含 0=暂停），
// 必须如实回退默认。改前：Contains(d.Hostname,"") 恒真 ⇒ 返回第一台有值的机器。
func TestGAP298_EmptyQueryHostFallsBack(t *testing.T) {
	one := 1
	devices := []DeviceConfig{{Hostname: "node-b", MaxWorkers: &one}}
	got, explicit := resolveByDevice(devices, "", map[string]int{}, 42)
	if got != 42 || explicit {
		t.Fatalf("hostname 空（错误分支）漏出别机值：got=(%d,%v)，want (42,false)", got, explicit)
	}
}

// GAP-20260927-298/299：内置表里的空键同样不得当万能匹配（Contains(x,"") 恒真）。
func TestGAP298299_EmptyKeyInFallbackNoMatch(t *testing.T) {
	if v := resolveByHostname(map[string]int{"": 9}, 42); v != 42 {
		t.Fatalf("内置表空键当万能匹配：got=%d，want 42", v)
	}
}
