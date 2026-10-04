package monitor

import (
	"testing"
)

// 缺口 GAP-20260925-34 回归牙：macOS 的 GPU 利用率必须取**真实占用**
// （PerformanceStatistics["Device Utilization %"]），不得再取机器固有的
// gpu-perf-tgt-utilization 目标档位（那份值每台机恒定，本机 =0x46=70）。
func TestParseDeviceUtilization(t *testing.T) {
	cases := []struct {
		name string
		line string
		want float64
		ok   bool
	}{
		{
			name: "真实 ioreg 行（空闲=0）",
			line: `      "PerformanceStatistics" = {"In use system memory (driver)"=0,"Alloc system memory"=3534061568,"Tiler Utilization %"=0,"recoveryCount"=0,"lastRecoveryTime"=0,"Renderer Utilization %"=0,"TiledSceneBytes"=0,"Device Utilization %"=0,"SplitSceneCount"=0,"Allocated PB Size"=121241600,"In use system memory"=1454260224}`,
			want: 0,
			ok:   true,
		},
		{
			name: "有负载=73（Device 与 Tiler 不同值，防取错键）",
			line: `      "PerformanceStatistics" = {"Renderer Utilization %"=12,"Device Utilization %"=73,"Tiler Utilization %"=41}`,
			want: 73,
			ok:   true,
		},
		{
			name: "末位紧跟右括号=100",
			line: `      "PerformanceStatistics" = {"Device Utilization %"=100}`,
			want: 100,
			ok:   true,
		},
		{
			name: "无该键 ⇒ 不命中",
			line: `      "PerformanceStatistics" = {"Tiler Utilization %"=0,"Renderer Utilization %"=0}`,
			ok:   false,
		},
		{
			name: "旧目标档位行 ⇒ 不再被当真实值（本机 <46000000>=70）",
			line: `    "gpu-perf-tgt-utilization" = <46000000>`,
			ok:   false,
		},
	}
	for _, c := range cases {
		got, ok := parseDeviceUtilization(c.line)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: parseDeviceUtilization=%v,%v want %v,%v", c.name, got, ok, c.want, c.ok)
		}
	}
}
