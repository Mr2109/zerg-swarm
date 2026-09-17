package agent

// tools.go — 虫族 v2.5 Agent 工具定义（2026-08-13）
// 模仿 Claude Code 工具设计：每个工具 description 含完整行为约束（偏好矩阵/用法/示例）

// DefaultTools — 默认工具（静态——内置）
var DefaultTools = []ToolDef{
	toolBash(),
	toolRead(),
	toolWrite(),
	toolEdit(),
	toolGlob(),
	toolGrep(),
	toolLs(),
	toolWebSearch(),
	toolWebFetch(),
	toolSkillLoad(),
	toolToolSearch(),
	toolScreenshot(), // v2.5.5: 虫族看图工具（截图+OCR——开发调试）
	toolApplyPatch(), // v2.5.5 P0: apply_patch 精确补丁（Codex 借鉴——防改错——Mr2109 2026-08-21）
	toolSpawnAgent(), // v2.5.5 P2: spawn_agent 子 agent 委托（agent 间通信——2026-08-21 接入——孤儿工具修复）
	toolTodo(),       // v2.5.5 P2: todo 任务清单（Claude Code 借鉴——复杂任务拆解防遗漏——2026-08-21）
}

// pluginTools — 插件动态工具（v2.5.5 P2 可逆副作用——2026-08-21 Mr2109）
// key: pluginName —— value: 该插件注册的工具
// 插件卸载 → 撤销（从 DefaultTools 移除）——防僵尸工具
var pluginTools = map[string][]ToolDef{}

// AllTools 全部工具（静态 + 插件动态 + 扩展注册——CA 每次请求组装）
func AllTools() []ToolDef {
	tools := make([]ToolDef, 0, len(DefaultTools)+len(pluginTools)+8)
	tools = append(tools, DefaultTools...)
	for _, defs := range pluginTools {
		tools = append(tools, defs...)
	}
	// P4-49 扩展工具（对话层注册——CA/对话统一工具库）
	extra := ExtraTools()
	known := map[string]bool{}
	for _, t := range tools {
		known[t.Function.Name] = true
	}
	for _, et := range extra {
		if known[et.Name] {
			continue // 重名不覆盖（L0 等 agent 原生优先）
		}
		known[et.Name] = true
		tools = append(tools, ToolDef{
			Function: FunctionDef{
				Name:        et.Name,
				Description: et.Desc,
				Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
			},
		})
	}
	return tools
}

// ToolDefOf — 按名取工具定义（多语言 D2：执行前接口约定校验读 schema 用）
func ToolDefOf(name string) (ToolDef, bool) {
	for _, d := range AllTools() {
		if d.Function.Name == name {
			return d, true
		}
	}
	return ToolDef{}, false
}

// RegisterPluginTool 插件注册工具（可逆副作用——记录归属）
func RegisterPluginTool(pluginName string, def ToolDef) {
	pluginTools[pluginName] = append(pluginTools[pluginName], def)
}

// UnregisterPluginTools 插件卸载撤销工具（可逆副作用——从注册表移除）
func UnregisterPluginTools(pluginName string) int {
	n := len(pluginTools[pluginName])
	delete(pluginTools, pluginName)
	return n
}

// toolBash — bash 命令执行工具
func toolBash() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "bash",
			Description: "执行 shell 命令（测试/构建/脚本/命令）。文件操作走专用工具（read/write/edit/glob/grep——有结构化输入与权限检查）。\n\n" +
				"【删除安全】rm 目标限 { 工作区, /tmp, 白名单 }；家目录/根目录/系统路径一律拦截。拦截=预期，勿用变量/base64/换拼写绕过（同样拦截）。\n\n" +
				"【成败判定】首行「⚠️ exit N — 命令失败」=失败（附引导）；正常看 [exit_code] 0。空命令/坏参数回格式教学（勿原样重发）。超长输出头尾保留+溢出落盘路径（read 可续读）。\n\n" +
				"【示例】\"go test ./...\" 跑测试；{\"command\":\"go build\",\"timeout_s\":300} 长编译；{\"command\":\"ls\",\"cwd\":\"sub/dir\"} 指定目录",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "要执行的 bash 命令（执行程序/脚本/测试/构建——文件操作用专用工具）",
					},
					"cwd": map[string]any{
						"type":        "string",
						"description": "可选：执行工作目录（相对工作区路径——默认工作区根；cd 不跨调用持久——跨目录请用此参数）",
					},
					"timeout_s": map[string]any{
						"type":        "integer",
						"description": "可选：超时秒数（默认 30——长编译/测试/下载请显式给大值如 300）",
					},
				},
				"required": []string{"command"},
			},
		},
	}
}

// toolRead — 读取文件内容
func toolRead() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "read",
			Description: "读取文件内容（文本/PDF/Office/epub 自动抽取；图像/音频/视频给委托指引）。\n\n" +
				"【用法】文本→带行号原文（num=false 关）；PDF/Office→自动抽取（附类型注记）；.xlsx→首 sheet 抽 TSV、.csv 原样；GBK/UTF-16→自动转 UTF-8；大文件→offset/limit 分页，超长行自动截断。\n\n" +
				"【示例】\"read\" path=main.go；{\"path\":\"报告.pdf\",\"offset\":1,\"limit\":100}；{\"path\":\"旧.txt\",\"format\":\"raw\",\"num\":false}",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "文件路径（相对于 WorkDir）",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "起始行号（默认 1——分页用——大文件看下一页传上次的 offset+limit）",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "返回行数上限（默认 500——分页用——文件超限时模型主动翻页）",
					},
					"num": map[string]any{
						"type":        "boolean",
						"description": "行号(默认 true;false=原文无行号)",
					},
					"format": map[string]any{
						"type":        "string",
						"enum":        []any{"auto", "raw"},
						"description": "auto=类型探测自动读(默认);raw=强制按原文文本读(不抽取/不委托)",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// toolWrite — 写文件（原子写）
func toolWrite() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "write",
			Description: "写文件（原子写：临时+回读校验+rename；覆盖整个文件，追加先 read 再写）。路径相对 WorkDir。\n\n" +
				"【行为】文档/二进制类（.pdf/.docx/.xlsx/.pptx/.epub/.odt/图片/音视频）拒绝文本写入（防毁）；.json/.xml 自动语法自检，坏则回滚报错。\n\n" +
				"【示例】\"write\" path=src/main.go content=\"...\"；{\"path\":\"a.json\",\"content\":\"{}\"}（自动校验）；{\"path\":\"win.csv\",\"content\":\"a,b\\n1,2\",\"bom\":true,\"line_end\":\"crlf\"}",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "目标文件路径（相对于 WorkDir）",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "要写入的文件内容",
					},
					"bom": map[string]any{
						"type":        "boolean",
						"description": "true=前加 UTF-8 BOM(Windows/Excel 中文友好)",
					},
					"line_end": map[string]any{
						"type":        "string",
						"enum":        []any{"lf", "crlf"},
						"description": "行尾统一(默认 lf)",
					},
					"format": map[string]any{
						"type":        "string",
						"enum":        []any{"auto", "raw"},
						"description": "auto=防呆+自检(默认);raw=裸写逃生门(跳防呆/自检——慎用)",
					},
				},
				"required": []string{"path", "content"},
			},
		},
	}
}

// toolEdit — 精准编辑（search → replace）
func toolEdit() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "edit",
			Description: "精准替换文件中的字符串（search → replace）。\n\n" +
				"【注意】编辑前必须先 read 该文件（read-before-edit）；search 必须唯一匹配（否则失败——加长上下文或用 replace_all）；保持原缩进；文档/二进制类（.pdf/.docx/.xlsx 等）拒绝替换。\n\n" +
				"【示例】\"edit\" path=main.go search=\"a / b\" replace=\"a // b\"",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "文件路径（相对于 WorkDir）",
					},
					"search": map[string]any{
						"type":        "string",
						"description": "要查找的旧字符串（必须唯一）",
					},
					"replace": map[string]any{
						"type":        "string",
						"description": "替换成的新字符串",
					},
					"replace_all": map[string]any{
						"type":        "boolean",
						"description": "是否替换所有匹配（默认 false）",
					},
				},
				"required": []string{"path", "search", "replace"},
			},
		},
	}
}

// toolGlob — 文件模式匹配
func toolGlob() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "glob",
			Description: "按模式查找文件（不要用 find/ls）。\n\n" +
				"【示例】\"glob\" pattern=\"**/*.go\"",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "glob 模式（如 **/*.go）",
					},
				},
				"required": []string{"pattern"},
			},
		},
	}
}

// toolGrep — 内容搜索
func toolGrep() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "grep",
			Description: "按正则搜索文件内容（不要用 bash grep/rg）。path 可为文件或目录（目录递归）；匹配多时自动分页（最多 200 条，用 offset 翻页）。\n\n" +
				"【示例】\"grep\" path=src pattern=\"func \"",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "文件或目录路径（目录递归搜索）",
					},
					"pattern": map[string]any{
						"type":        "string",
						"description": "正则表达式",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "跳过前 N 条匹配（分页用——看后续传 offset=200/400）",
					},
					"hidden": map[string]any{
						"type":        "boolean",
						"description": "搜索隐藏文件（.env/.gitignore 等——默认 false 跳过）",
					},
				},
				"required": []string{"path", "pattern"},
			},
		},
	}
}

// toolLs — 目录列表
func toolLs() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "ls",
			Description: "列出目录内容（默认：目录在前、隐藏点项、最多 60 行）。ls=本层概览（可 pattern 收窄）；glob=跨层找文件；grep=内容搜索。\n\n" +
				"【示例】\"ls\"；{\"path\":\".\",\"dir_only\":true}；{\"pattern\":\"*.go\",\"limit\":0}（不截断）；{\"sort_by\":\"time\"}（附时间列）。超限请缩小 pattern，勿重发大列表。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "目录路径(默认 .)",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "最大列出行(默认 60;0=不截断)",
					},
					"dir_only": map[string]any{
						"type":        "boolean",
						"description": "只列目录(默认 false)",
					},
					"hidden": map[string]any{
						"type":        "boolean",
						"description": "显示点文件/目录(默认 false——.zerg 等隐藏)",
					},
					"pattern": map[string]any{
						"type":        "string",
						"description": "名称通配过滤(如 *.go——fnmatch)",
					},
					"sort_by": map[string]any{
						"type":        "string",
						"enum":        []any{"name", "time", "size"},
						"description": "排序(默认 name;目录恒在前;time 附时间列)",
					},
				},
				"required": []string{},
			},
		},
	}
}

// toolWebSearch — 网络搜索工具（M3a——依赖 searxng 8888）
func toolWebSearch() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name:        "web_search",
			Description: `网络搜索（searxng 聚合——调研/查证）。query 必填；可选 lang/time_range/domains/fetch_top/rewrite。不确定先 help:true`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":      map[string]any{"type": "string", "description": "搜索关键词（自然语言）"},
					"limit":      map[string]any{"type": "integer", "description": "返回条数（默认 5——长 query 自动 8）"},
					"lang":       map[string]any{"type": "string", "enum": []any{"zh", "en", "all"}, "description": "语言偏好（默认 all）"},
					"time_range": map[string]any{"type": "string", "enum": []any{"day", "week", "month", "year"}, "description": "时间过滤"},
					"domains":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "限定域名（site: 过滤）"},
					"fetch_top":  map[string]any{"type": "integer", "description": "自动抓取前 N 条正文（1-3）"},
					"rewrite":    map[string]any{"type": "boolean", "description": "LLM 优化搜索词（慢）"},
				},
				"required": []string{"query"},
			},
		},
	}
}

// toolWebFetch — 网页抓取工具（M3a——无依赖）
func toolWebFetch() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name:        "web_fetch",
			Description: `抓取网页正文（去标签，截断 10000）。url 必填（http/https）。不确定先 help:true`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{"type": "string", "description": "完整 URL（http/https）"},
				},
				"required": []string{"url"},
			},
		},
	}
}

// toolSkillLoad — 加载技能（v2.5.1——SKILL.md 触发读正文）
func toolSkillLoad() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name:        "skill_load",
			Description: `加载技能正文 SKILL.md。name 必填（系统提示列出的技能名，任务匹配时用）`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string", "description": "技能名（如 code-optimization）"},
				},
				"required": []string{"name"},
			},
		},
	}
}

// toolToolSearch — 搜索发现工具（v2.5.1——对齐 Codex tool_search——按需发现）
// MCP 等扩展工具不进初始工具列表——需要时用本工具搜索发现
func toolToolSearch() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name:        "tool_search",
			Description: `搜索发现可用工具（按需）。query 必填（描述需要的能力，工具列表没有时用，发现后直接调用）`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "需要的功能描述（自然语言）"},
				},
				"required": []string{"query"},
			},
		},
	}
}

// toolScreenshot — 虫族看图工具（v2.5.5: 截图+OCR识别——开发调试必备）
// 截图（跨平台系统命令）→ RapidOCR 识别（文字+坐标）→ 返回结构化描述
func toolScreenshot() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name:        "screenshot",
			Description: `截图+OCR 识别看界面（开发调试——UI 验证/看报错）。vision 可选(true=视觉模型理解)。不确定先 help:true`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":   map[string]any{"type": "string", "description": "图片路径（不传=截全屏）"},
					"vision": map[string]any{"type": "boolean", "description": "是否用视觉模型理解（语义描述——较慢）"},
				},
			},
		},
	}
}

// toolApplyPatch — 精确补丁（Codex apply_patch 借鉴——2026-08-21 Mr2109 P0）
// 标准 diff 格式（@@ 行号 + 上下文 + -/+ 行）——严格应用——位置不对报错不猜
func toolApplyPatch() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "apply_patch",
			Description: "按精确补丁修改文件（标准 diff——行号+上下文严格匹配，位置错则失败，不默默改错）。编辑前必须先 read 该文件。\n\n" +
				"【补丁格式】git diff 风格：--- a/文件名 / +++ b/文件名 / @@ -起始行,行数 +起始行,行数 @@ / 上下文行（锚定）/ -删除行 / +添加行；必须给足上下文锚定唯一位置。\n\n" +
				"【与 edit 的区别】edit=模糊替换（search→replace）；apply_patch=精确补丁（改错位置会失败）。\n\n" +
				"【示例】apply_patch path=main.go patch=\"--- a/main.go\\n+++ b/main.go\\n@@ -10,3 +10,3 @@\\n func old() {\\n-    return oldValue\\n+    return newValue\\n }\"",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":  map[string]any{"type": "string", "description": "文件路径（相对于 WorkDir）"},
					"patch": map[string]any{"type": "string", "description": "标准 diff 补丁内容（---/+++/@@ 格式）"},
				},
				"required": []string{"path", "patch"},
			},
		},
	}
}
func toolSpawnAgent() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "spawn_agent",
			Description: "派子 agent 执行独立子任务（独立上下文，不污染父；返回 1-2K 精简摘要）。\n\n" +
				"【适合】独立调研/独立文件处理/独立验证；不适合需要父上下文的子任务。machine 指定机器：x3/local/mini1/mini2（不传=默认调度）。\n\n" +
				"【示例】\"spawn_agent\" prompt=\"调研 X 项目的依赖结构\" type=\"explore\" machine=\"x3\"",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt":  map[string]any{"type": "string", "description": "子任务完整提示词（自包含——子 agent 独立执行）"},
					"type":    map[string]any{"type": "string", "enum": []any{"explore", "plan", "general"}, "description": "子 agent 类型（默认 general）"},
					"machine": map[string]any{"type": "string", "x-zerg-format": "ascii", "description": "指定机器（双机协作）: x3/local/mini1/mini2（不传=默认调度）"},
				},
				"required": []string{"prompt"},
			},
		},
	}
}

// toolTodo — 任务清单工具（Claude Code todo 借鉴——2026-08-21 Mr2109）
// 复杂任务拆解为可勾选清单——防止遗漏步骤——模型自维护状态
func toolTodo() ToolDef {
	return ToolDef{
		Type: "function",
		Function: FunctionDef{
			Name: "todo",
			Description: "任务清单管理（复杂任务拆解防遗漏；持久化到工作区 .zerg/todo.json，跨轮次保留）。\n\n" +
				"【用法】复杂任务（多步骤/多文件）先建清单逐步勾选。action=create（items 传 [{content, status}]）/ update（item_id + status）/ list。status: pending/in_progress/completed/cancelled。\n\n" +
				"【示例】\"todo\" action=\"create\" items=[{\"content\":\"调研依赖\",\"status\":\"completed\"},{\"content\":\"实现核心逻辑\",\"status\":\"in_progress\"},{\"content\":\"验证结果\",\"status\":\"pending\"}]",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action":  map[string]any{"type": "string", "enum": []any{"create", "update", "list"}, "description": "操作（默认 list）"},
					"items":   map[string]any{"type": "array", "description": "create 时待办数组（[{content, status}]）"},
					"item_id": map[string]any{"type": "integer", "description": "update 时要更新的项序号"},
					"status":  map[string]any{"type": "string", "enum": []any{"pending", "in_progress", "completed", "cancelled"}, "description": "update 时新状态"},
				},
				"required": []string{"action"},
			},
		},
	}
}
