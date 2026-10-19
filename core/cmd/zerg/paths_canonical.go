// paths_canonical.go —— 路径类常量**单一真源**（GAP-20261001-69）。
//
// 病：同一路径在 core/cmd/zerg 包里被不同文件各自写字面量，名字各异（例如
// `publish/docs` 一枚值有 6 个名字散在 6 件）⇒ 改一漏一，没有单一真源。
//
// 口径：**同一个值只许在本文出现一次**；其余位置一律写成 `const 旧名 = 规范名`
// （别名是指向真源的引用，不是第二份字面量）。新增路径常量时先把值加到这里。
//
// 本次只收 Top 6（按异名数量排序 · 覆盖 31 处定义）；其余 36 个值按同一套路后续批次收。
package main

const (
	// 公开面目录（公开面四门 + 导出器 + 发布预检共用）
	pathPublishDocs = "publish/docs"
	// 公开标记表（六道门共用）
	pathMarksTable = "scripts/公开标记.tsv"
	// 命令树主件（多处按件名做行号/登记对拍）
	pathCmdMainGo = "core/cmd/zerg/main.go"
	// 命令面包目录
	pathCmdZergDir = "core/cmd/zerg"
	// 公开镜像库（生成器 + 发布/镜像两条面）
	pathMirrorLibPy = "publish/mirror-public-lib.py"
	// 提交门禁脚本（门表/封存/双态/MCP 多处引用）
	pathPrecommitGates = "scripts/gates/precommit-gates.sh"

	// 第二批（2026-10-20）：门脚本目录与几份登记件/清单件
	pathScriptsGates     = "scripts/gates"
	pathVersionGo        = "core/internal/version/version.go"
	pathGateCoverageCfg  = "scripts/gates/gate-coverage.config.json"
	pathPublishExcludes  = "publish/excludes.txt"
	pathJudgeRegistry    = "scripts/gates/judge-registry.json"
	pathFreshnessPy      = "scripts/gates/check-manifest-freshness.py"
	pathPublishWhitelist = "publish/whitelist.txt"
	pathContractRegistry = "core/internal/contract/registry.json"
)
