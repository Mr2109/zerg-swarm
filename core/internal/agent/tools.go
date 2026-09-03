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
	toolTodo(), // v2.5.5 P2: todo 任务清单（Claude Code 借鉴——复杂任务拆解防遗漏——2026-08-21）
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
			Description: "执行 bash 命令，在沙盒 shell 中运行，返回 stdout 和 stderr。\n\n" +
				"【工具偏好矩阵】除非必要避免用 bash 运行以下命令——请用专用工具：" +
				"文件搜索用 glob、内容搜索用 grep、读文件用 read、编辑用 edit、写文件用 write" +
				"（专用工具有结构化输入和权限检查，bash 裸命令不可控）。\n\n" +
				"【用法】bash 只用于执行程序/脚本/测试/构建/编译。命令带 30 秒超时，" +
				"输出截断 2000 字符，必须在 WorkDir 内执行。\n\n" +
				"【示例】\"ls -la\" 列目录、\"python3 test.py\" 跑测试、\"go build ./...\" 编译",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "要执行的 bash 命令（执行程序/脚本/测试/构建——文件操作用专用工具）",
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
			Description: "读取文件内容（带行号）。\n\n" +
				"【用法】\n" +
				"- 读文件用本工具（NOT cat/head/tail）\n" +
				"- 大文件自动分页（offset/limit）\n" +
				"- 路径相对于 WorkDir\n\n" +
				"【示例】\"read\" path=src/main.go — 读取文件",
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
			Description: "写文件（原子写——临时文件+rename）。\n\n" +
				"【用法】\n" +
				"- 写文件用本工具（NOT echo 重定向/cat <<EOF）\n" +
				"- 覆盖整个文件（追加请先 read 再写）\n" +
				"- 路径相对于 WorkDir\n\n" +
				"【示例】\"write\" path=src/main.go content=\"package main\\n\\nfunc main() {}\"",
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
			Description: "在文件中精准替换字符串（search → replace）。\n\n" +
				"【用法】\n" +
				"- 编辑前必须先 read 过该文件（read-before-edit 规则）\n" +
				"- search 必须唯一匹配（不唯一会失败——用更多上下文或 replace_all）\n" +
				"- 保持原缩进\n\n" +
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
			Description: "按模式匹配查找文件（文件搜索用本工具——NOT find/ls）。\n\n" +
				"【示例】\"glob\" pattern=\"**/*.go\" — 查找所有 Go 文件",
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
			Description: "按正则搜索文件内容（内容搜索用本工具——NOT grep/rg）。\n\n" +
				"【用法】\n" +
				"- path 可以是文件或目录（目录递归搜索）\n" +
				"- pattern 是正则表达式\n" +
				"- 匹配多时自动分页（最多显示 200 条——用 offset 看后续）\n\n" +
				"【示例】\"grep\" path=src pattern=\"func \" — 搜索所有函数定义",
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
			Description: "列出目录内容（目录列表用本工具）。\n\n" +
				"【示例】\"ls\" path=. — 列出当前目录",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "目录路径（默认 .）",
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
			Description: `网络搜索（searxng 聚合——调研/查证）。参数: query 必填——可选 lang/time_range/domains/fetch_top——不确定先 help`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":      map[string]any{"type": "string", "description": "搜索关键词（自然语言）"},
					"limit":      map[string]any{"type": "integer", "description": "返回条数（默认 5——长 query 自动 8）"},
					"lang":       map[string]any{"type": "string", "description": "语言偏好 zh/en/all（默认 all）"},
					"time_range": map[string]any{"type": "string", "description": "时间过滤 day/week/month/year"},
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
			Description: `抓取网页正文（去标签——截断 10000）。参数: url 必填（http/https 完整地址）——不确定先 help`,
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
			Description: `加载技能正文 SKILL.md。参数: name 必填（系统提示列出的技能名——任务匹配时用）`,
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
			Description: `搜索发现可用工具（按需）。参数: query 必填（描述需要的能力——工具列表没有时用——发现后直接用）`,
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
			Description: `截图+OCR 识别看界面（开发调试——UI 验证/看报错）。参数: vision 可选(true 视觉模型理解)——不确定先 help`,
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
			Description: "按精确补丁修改文件（标准 diff 格式——Codex 借鉴——防改错）。\n\n" +
				"【用法】\n" +
				"- 编辑前必须先 read 过该文件（read-before-edit 规则）\n" +
				"- 补丁格式（git diff 风格）:\n" +
				"    --- a/文件名\n" +
				"    +++ b/文件名\n" +
				"    @@ -起始行,行数 +起始行,行数 @@\n" +
				"     上下文行（锚定——不修改）\n" +
				"    -要删除的行\n" +
				"    +要添加的行\n" +
				"- 必须提供足够上下文（锚定唯一位置——防改错）\n" +
				"- 位置不对/上下文不匹配 → 应用失败返回错误（不默默改错——重新读文件再写）\n" +
				"- 与 edit 的区别: edit 是模糊替换（search→replace）；apply_patch 是精确补丁（行号+上下文严格匹配——改错位置会失败）\n\n" +
				"【示例】\n" +
				"apply_patch path=main.go patch=\"--- a/main.go\\n+++ b/main.go\\n@@ -10,3 +10,3 @@\\n func old() {\\n-    return oldValue\\n+    return newValue\\n }\"",
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
			Description: "派子 agent 执行独立子任务（agent 间通信——Codex 借鉴）。\n\n" +
				"【用法】\n" +
				"- 复杂任务可拆分子任务——派子 agent 执行（独立上下文——不污染父）\n" +
				"- 子 agent 返回精简摘要（1-2K 字符——父上下文干净）\n" +
				"- machine 指定机器（双机协作）: x3/local/mini1/mini2（不传=默认调度）\n" +
				"- 适合: 独立调研/独立文件处理/独立验证——不适合需要父上下文的子任务\n\n" +
				"【示例】\"spawn_agent\" prompt=\"调研 X 项目的依赖结构\" type=\"explore\" machine=\"x3\"",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt": map[string]any{"type": "string", "description": "子任务完整提示词（自包含——子 agent 独立执行）"},
					"type":   map[string]any{"type": "string", "description": "子 agent 类型: explore/plan/general（默认 general）"},
					"machine": map[string]any{"type": "string", "description": "指定机器（双机协作）: x3/local/mini1/mini2（不传=默认调度）"},
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
			Description: "任务清单管理（Claude Code todo 借鉴——复杂任务拆解防遗漏）。\n\n" +
				"【用法】\n" +
				"- 复杂任务（多步骤/多文件）先拆解为 todo 清单——逐步勾选——防止遗漏\n" +
				"- action 参数: create（创建清单）/ update（更新某项状态）/ list（查看当前清单）\n" +
				"- create: items 传待办数组（[{content, status}]——status: pending/in_progress/completed/cancelled）\n" +
				"- update: item_id 传要更新的项 + status 传新状态\n" +
				"- 清单持久化到工作区 .zerg/todo.json——跨轮次保留\n\n" +
				"【示例】\n" +
				"\"todo\" action=\"create\" items=[{\"content\":\"调研依赖\",\"status\":\"completed\"},{\"content\":\"实现核心逻辑\",\"status\":\"in_progress\"},{\"content\":\"验证结果\",\"status\":\"pending\"}]",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{"type": "string", "description": "操作: create/update/list（默认 list）"},
					"items":  map[string]any{"type": "array", "description": "create 时待办数组（[{content, status}]）"},
					"item_id": map[string]any{"type": "integer", "description": "update 时要更新的项序号"},
					"status": map[string]any{"type": "string", "description": "update 时新状态: pending/in_progress/completed/cancelled"},
				},
				"required": []string{"action"},
			},
		},
	}
}
