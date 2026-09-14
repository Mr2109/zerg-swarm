// Package registry — 存储档位与装载下限（P5 批 2）。
//
// 设计真源：docs/01-设计/设计-子端沙箱化-20260914.md
//
//	§9.6 介质档位与「装载下限」：
//	  装载时间下限 ≈ 权重体积 ÷ 实测顺序读带宽（**物理下限，不是估计值**）；
//	  介质档位 = NVMe / SATA SSD / HDD（接口类别，不是型号）；
//	  实测读带宽 = 该机器上**实测**的顺序读，不是标称值。
//	§8.4 标定铁律：凡数字必实测、禁估值、禁照抄声明值。
//
// 口径（写死）：
//   - 两个字段都是**实测档案的载体**，子端只搬运、不生成数字；
//   - 缺任何一个（无档位、无带宽）⇒ 算不出下限就明说 ok=false，**绝不猜**
//     （宁缺毋错：一个编出来的下限会伪装成「物理事实」，比没有更糟）；
//   - 下限**向下取整**到纳秒（time.Duration 的原生精度）——下限只会被用来
//     判断「实测远大于下限 ⇒ 卡在别处」，取整方向永远不许让下限变松。
package registry

import "time"

// StorageTier 存储档位（接口类别，设计 §9.6：不是型号、不是厂商）。
type StorageTier string

const (
	// TierNVMe NVMe SSD（X3 实测 rotational=0，/dev/nvme0n1p7，§2.0 / §9.6）。
	TierNVMe StorageTier = "nvme"
	// TierSataSSD SATA SSD（量级 ⇒ 装载下限上百秒）。
	TierSataSSD StorageTier = "sata_ssd"
	// TierHDD 机械盘（旁注：MoE 按需加载的小随机读会被寻道打死，§9.6——
	// X3 不是这种情况；这条只用于将来别的设备进族时判型）。
	TierHDD StorageTier = "hdd"
)

// Valid 合法的档位枚举之一。
func (t StorageTier) Valid() bool {
	switch t {
	case TierNVMe, TierSataSSD, TierHDD:
		return true
	}
	return false
}

// LoadTimeFloor 装载下限纯函式（设计 §9.6，写死口径）：
//
//	装载时间下限 = 权重体积 ÷ 实测顺序读带宽
//
// 输入一律用实测值：weightBytes 来自权重文件（或实测档案的「权重体积」），
// readBandwidthBps 来自实测顺序读带宽（字节/秒，**不是标称值**）。
//
// 返回 ok=false 的情况（**不确定就明说，不猜**，§8.4 标定铁律）：
//   - weightBytes <= 0（无权重体积——拿不到实测档案也不知道文件多大）；
//   - readBandwidthBps <= 0（该机器没有实测读带宽声明）；
//   - 结果会溢出 time.Duration（>约 292 年——体积/带宽乱填时如实报不可算）。
//
// 下限向下取整到纳秒：下限只会被用来对照「实测远大于下限 ⇒ 瓶颈不在盘」
// （§9.6 的第②条用处），取整方向不许让下限变松。
func LoadTimeFloor(weightBytes, readBandwidthBps int64) (time.Duration, bool) {
	if weightBytes <= 0 || readBandwidthBps <= 0 {
		return 0, false
	}
	// 体积（字节）× 1e9（ns/s）÷ 带宽（字节/秒）= 纳秒。
	// 用大整数除法防溢出：weightBytes 高达百 GB 级时，×1e9 会爆 int64，
	// 先做商与余数再拼回纳秒，精度与顺序都不损失。
	const nsPerSec = int64(time.Second) // 1e9
	q := weightBytes / readBandwidthBps
	r := weightBytes % readBandwidthBps
	if q > (1<<62)/nsPerSec {
		return 0, false // 商已大到 ×1e9 必溢出（>~92 亿秒），如实报不可算
	}
	ns := q*nsPerSec + r*nsPerSec/readBandwidthBps
	if ns < 0 { // 溢出成负数（q 检查兜不住的极端组合）
		return 0, false
	}
	return time.Duration(ns), true
}
