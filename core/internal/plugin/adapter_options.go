// adapter_options.go — 模型适配器可编辑参数 schema（2026-08-27 Mr2109）
// 铁律: 每个模型的适配器都不一样——OptionSchema 由各适配器自己声明——UI 按所选模型 schema 渲染
// 没有的参数不显示、不能编辑——绝不统一套用参数集
package plugin

// OptionDef 单个可编辑参数定义
type OptionDef struct {
	Key     string      `json:"key"`               // 参数名（=适配器 Init 的 cfg key——小写）
	Type    string      `json:"type"`              // number / string / bool / enum / array
	Value   interface{} `json:"value"`             // 当前值
	Desc    string      `json:"desc,omitempty"`    // 说明
	Options []string    `json:"options,omitempty"` // enum 可选值
	Min     float64     `json:"min,omitempty"`     // number 下限
	Max     float64     `json:"max,omitempty"`     // number 上限
}

// OptionedAdapter 可编辑适配器接口（实现 = 支持 UI 编辑 + 实时生效）
// 每个适配器自己声明参数集（各自不同）——UpdateOptions 运行时更新（Init 覆盖式——立即生效）
type OptionedAdapter interface {
	// OptionSchema 声明本适配器的可编辑参数（含当前值）
	OptionSchema() []OptionDef
	// UpdateOptions 运行时更新配置（对运行中实例 Init——实时生效——不重启）
	UpdateOptions(cfg map[string]interface{}) error
}
