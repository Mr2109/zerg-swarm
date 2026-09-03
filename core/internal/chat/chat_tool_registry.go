// chat_tool_registry.go — v2.5.7 P4-42 对话工具注册中心（deferred 按需加载）
// 设计: L0 核心常驻 12 个（永不 defer——Hermes 原则）——L1 300+ 目录按需 tool_search 发现注入
// 参考: Hermes tools/tool_search.py（三层 Tier）+ CA v2.5.1 toolSearch（loop.go:326 已验证质变）
// 原则: 先实装 60 个真实可用——目录先全——按使用率扩展——绝不写空壳

package chat

import (
	"sort"
	"strings"
)

// ChatToolMeta — 工具元数据（注册中心条目）
type ChatToolMeta struct {
	Name     string   // 工具名（与 ToolDef 对应）
	Category string   // 类别（系统管理/影音剪辑/效率工具…）
	Keywords []string // 中文搜索词（小模型用中文搜——CA 教训）
	L0       bool     // true=常驻主提示（永不 defer）
}

// chatToolRegistry — 工具注册表（300+ 目录——首批实装 60+）
var chatToolRegistry = []ChatToolMeta{
	// ═══ L0 核心常驻（12）═══
	{Name: "bash", Category: "文件与代码", Keywords: []string{"命令", "执行", "shell", "运行"}, L0: true},
	{Name: "read", Category: "文件与代码", Keywords: []string{"读文件", "查看内容", "cat"}, L0: true},
	{Name: "write", Category: "文件与代码", Keywords: []string{"写文件", "创建文件"}, L0: true},
	{Name: "edit", Category: "文件与代码", Keywords: []string{"编辑", "修改", "替换"}, L0: true},
	{Name: "glob", Category: "文件与代码", Keywords: []string{"找文件", "文件匹配", "按名搜"}, L0: true},
	{Name: "grep", Category: "文件与代码", Keywords: []string{"搜内容", "关键词搜索", "代码搜索"}, L0: true},
	{Name: "ls", Category: "文件与代码", Keywords: []string{"列目录", "列表", "目录"}, L0: true},
	{Name: "kb_search", Category: "知识库", Keywords: []string{"经验", "知识", "查库", "教训", "历史"}, L0: true},
	{Name: "web_search", Category: "网络与调研", Keywords: []string{"网络", "搜索", "调研"}, L0: true},
	{Name: "web_fetch", Category: "网络与调研", Keywords: []string{"网页", "抓取", "url"}, L0: true},
	{Name: "tool_search", Category: "系统", Keywords: []string{"发现工具", "找工具", "工具列表"}, L0: true},
	{Name: "skill_load", Category: "知识库", Keywords: []string{"技能", "skill", "加载"}, L0: true},

	// ═══ L1 deferred——首批实装 ═══

	// ── 文件与代码增强（10）──
	{Name: "diff_files", Category: "文件与代码", Keywords: []string{"差异", "对比", "diff"}},
	{Name: "stat_file", Category: "文件与代码", Keywords: []string{"文件信息", "大小", "修改时间"}},
	{Name: "head_file", Category: "文件与代码", Keywords: []string{"开头", "前几行"}},
	{Name: "tail_file", Category: "文件与代码", Keywords: []string{"末尾", "最后几行", "日志尾部"}},
	{Name: "du_usage", Category: "文件与代码", Keywords: []string{"磁盘占用", "目录大小"}},
	{Name: "copy_file", Category: "文件与代码", Keywords: []string{"复制", "拷贝"}},
	{Name: "move_file", Category: "文件与代码", Keywords: []string{"移动", "重命名"}},
	{Name: "delete_file", Category: "文件与代码", Keywords: []string{"删除", "移除"}},
	{Name: "find_name", Category: "文件与代码", Keywords: []string{"按名找", "文件名搜索", "find"}},
	{Name: "file_type", Category: "文件与代码", Keywords: []string{"文件类型", "格式判断"}},

	// ── 系统管理（15）──
	{Name: "sys_info", Category: "系统管理", Keywords: []string{"系统", "机器信息", "硬件"}},
	{Name: "cpu_status", Category: "系统管理", Keywords: []string{"cpu", "负载", "占用"}},
	{Name: "mem_status", Category: "系统管理", Keywords: []string{"内存", "mem", "占用"}},
	{Name: "disk_usage", Category: "系统管理", Keywords: []string{"磁盘", "空间", "df"}},
	{Name: "process_list", Category: "系统管理", Keywords: []string{"进程", "ps", "正在运行"}},
	{Name: "port_check", Category: "系统管理", Keywords: []string{"端口", "占用", "lsof"}},
	{Name: "service_status", Category: "系统管理", Keywords: []string{"服务", "状态", "zerg-core", "zerg-ui"}},
	{Name: "log_tail", Category: "系统管理", Keywords: []string{"日志", "log", "查看"}},
	{Name: "gpu_status", Category: "系统管理", Keywords: []string{"gpu", "显卡", "rocm", "利用率"}},
	{Name: "ping_check", Category: "系统管理", Keywords: []string{"网络", "ping", "连通"}},
	{Name: "network_status", Category: "系统管理", Keywords: []string{"网络接口", "ip", "连接"}},
	{Name: "kill_process", Category: "系统管理", Keywords: []string{"杀进程", "终止", "kill"}},
	{Name: "start_service", Category: "系统管理", Keywords: []string{"启动服务", "服务启动"}},
	{Name: "stop_service", Category: "系统管理", Keywords: []string{"停止服务", "服务停止"}},
	{Name: "uptime_info", Category: "系统管理", Keywords: []string{"运行时间", "uptime", "负载"}},

	// ── 知识库增强（5）──
	{Name: "kb_read", Category: "知识库", Keywords: []string{"知识库全文", "读条目", "kb 详情"}},
	{Name: "kb_categories", Category: "知识库", Keywords: []string{"知识库分类", "目录", "kb 列表"}},
	{Name: "kb_add", Category: "知识库", Keywords: []string{"写知识库", "入库", "记录经验"}},
	{Name: "kb_stats", Category: "知识库", Keywords: []string{"知识库统计", "条数"}},
	{Name: "capture_fix", Category: "知识库", Keywords: []string{"排障", "记录坑", "修复经验"}},

	// ── 网络增强（3）──
	{Name: "url_extract", Category: "网络与调研", Keywords: []string{"网页提取", "正文", "阅读"}},
	{Name: "http_get", Category: "网络与调研", Keywords: []string{"http", "api", "请求"}},
	{Name: "anysearch", Category: "网络与调研", Keywords: []string{"深度搜索", "调研", "anysearch"}},

	// ── 影音与剪辑（10）──
	{Name: "media_info", Category: "影音与剪辑", Keywords: []string{"媒体信息", "ffprobe", "音视频"}},
	{Name: "ffmpeg_transcode", Category: "影音与剪辑", Keywords: []string{"转码", "格式转换", "ffmpeg"}},
	{Name: "audio_extract", Category: "影音与剪辑", Keywords: []string{"提取音频", "音轨", "wav"}},
	{Name: "video_extract", Category: "影音与剪辑", Keywords: []string{"提取视频", "画面", "无音轨"}},
	{Name: "timecode_convert", Category: "影音与剪辑", Keywords: []string{"时间码", "帧数", "换算", "tc"}},
	{Name: "sample_rate_check", Category: "影音与剪辑", Keywords: []string{"采样率", "音频格式", "48k"}},
	{Name: "subtitle_generate", Category: "影音与剪辑", Keywords: []string{"字幕", "asr", "转写"}},
	{Name: "multicam_check", Category: "影音与剪辑", Keywords: []string{"多机位", "同步", "时码", "multicam"}},
	{Name: "fcpx_check", Category: "影音与剪辑", Keywords: []string{"fcpx", "final cut", "资源库"}},
	{Name: "resample_audio", Category: "影音与剪辑", Keywords: []string{"重采样", "音频频率", "转换"}},

	// ── 影音剪辑第二批（P4-43——fcpx skill 脚本链 + footage.db——30 个）──
	{Name: "footage_video_clips", Category: "影音与剪辑", Keywords: []string{"视频素材", "footage", "机位", "素材查询"}},
	{Name: "footage_poly_wavs", Category: "影音与剪辑", Keywords: []string{"poly wav", "录音", "音频素材", "12声道"}},
	{Name: "footage_match", Category: "影音与剪辑", Keywords: []string{"配对", "视频音频匹配", "match_result"}},
	{Name: "footage_resolve_pool", Category: "影音与剪辑", Keywords: []string{"达芬奇素材池", "resolve_pool", "媒体池"}},
	{Name: "footage_timeline", Category: "影音与剪辑", Keywords: []string{"时间线", "timeline", "剪辑序列"}},
	{Name: "footage_stats", Category: "影音与剪辑", Keywords: []string{"素材统计", "数量", "footage"}},
	{Name: "fcpx_add_lut", Category: "影音与剪辑", Keywords: []string{"加lut", "调色", "色彩"}},
	{Name: "fcpx_fix_multicam", Category: "影音与剪辑", Keywords: []string{"多机位修复", "音频修复", "fix_multicam"}},
	{Name: "fcpx_fix_pcut02", Category: "影音与剪辑", Keywords: []string{"pcut02", "原生xml修复", "音频替换"}},
	{Name: "fcpx_verify_sync", Category: "影音与剪辑", Keywords: []string{"同步验证", "verify_sync", "查证"}},
	{Name: "fcpx_xml_validate", Category: "影音与剪辑", Keywords: []string{"xml验证", "xmllint", "合法性"}},
	{Name: "fcpx_experience", Category: "影音与剪辑", Keywords: []string{"fcpx经验", "排坑", "互通经验"}},
	{Name: "resolve_drp_check", Category: "影音与剪辑", Keywords: []string{"drp", "达芬奇工程", "检查"}},
	{Name: "media_probe", Category: "影音与剪辑", Keywords: []string{"素材探测", "时码", "start", "tc"}},
	{Name: "audio_sync_check", Category: "影音与剪辑", Keywords: []string{"音频同步", "对齐", "sync"}},

	// ── 音乐下载第二批（P4-43——musicdl/QQ vkey/网易云 3s——15 个）──
	{Name: "music_search_qq", Category: "音乐下载", Keywords: []string{"qq音乐", "搜歌", "songmid"}},
	{Name: "music_search_netease", Category: "音乐下载", Keywords: []string{"网易云搜索", "搜歌"}},
	{Name: "music_search_kuwo", Category: "音乐下载", Keywords: []string{"酷我搜索", "搜歌"}},
	{Name: "music_download_qq", Category: "音乐下载", Keywords: []string{"qq下载", "vkey", "无损"}},
	{Name: "music_download_netease", Category: "音乐下载", Keywords: []string{"网易云下载", "320k", "防406"}},
	{Name: "music_download_kuwo", Category: "音乐下载", Keywords: []string{"酷我下载", "flac"}},
	{Name: "music_parse_share", Category: "音乐下载", Keywords: []string{"微信分享", "链接解析", "songDetail"}},
	{Name: "music_album_search", Category: "音乐下载", Keywords: []string{"专辑", "歌单", "专辑搜索"}},
	{Name: "music_batch_download", Category: "音乐下载", Keywords: []string{"批量下载", "歌单下载"}},
	{Name: "music_cover_get", Category: "音乐下载", Keywords: []string{"封面", "专辑图", "cover"}},
	{Name: "music_save_info", Category: "音乐下载", Keywords: []string{"下载目录", "保存位置", "music"}},
	{Name: "music_merge_check", Category: "音乐下载", Keywords: []string{"归拢", "重命名", "验证"}},

	// ── 效率工具（10）──
	{Name: "calc", Category: "效率工具", Keywords: []string{"计算", "算术", "数学"}},
	{Name: "json_format", Category: "效率工具", Keywords: []string{"json", "格式化", "美化"}},
	{Name: "yaml_json", Category: "效率工具", Keywords: []string{"yaml", "转换", "配置"}},
	{Name: "timestamp_convert", Category: "效率工具", Keywords: []string{"时间戳", "时间转换", "epoch"}},
	{Name: "hash_calc", Category: "效率工具", Keywords: []string{"哈希", "md5", "sha"}},
	{Name: "base64_codec", Category: "效率工具", Keywords: []string{"base64", "编码", "解码"}},
	{Name: "url_codec", Category: "效率工具", Keywords: []string{"url编码", "转义", "percent"}},
	{Name: "regex_test", Category: "效率工具", Keywords: []string{"正则", "匹配", "regex"}},
	{Name: "unit_convert", Category: "效率工具", Keywords: []string{"单位换算", "kb mb", "大小"}},
	{Name: "text_stats", Category: "效率工具", Keywords: []string{"文本统计", "字数", "行数"}},

	// ── 数据与存储（5）──
	{Name: "sql_query", Category: "数据与存储", Keywords: []string{"sql", "查询", "数据库"}},
	{Name: "db_tables", Category: "数据与存储", Keywords: []string{"表", "数据库结构", "schema"}},
	{Name: "backup_file", Category: "数据与存储", Keywords: []string{"备份", "备份文件"}},
	{Name: "export_json", Category: "数据与存储", Keywords: []string{"导出", "json", "数据"}},
	{Name: "import_json", Category: "数据与存储", Keywords: []string{"导入", "json"}},

	// ── 多模态（5）──
	{Name: "image_desc", Category: "多模态", Keywords: []string{"图片", "描述", "看图", "vision"}},
	{Name: "ocr_text", Category: "多模态", Keywords: []string{"ocr", "文字识别", "截图文字"}},
	{Name: "tts_speak", Category: "多模态", Keywords: []string{"语音", "tts", "朗读"}},
	{Name: "asr_transcribe", Category: "多模态", Keywords: []string{"语音转文字", "asr", "听写"}},
	{Name: "image_resize", Category: "多模态", Keywords: []string{"图片缩放", "resize", "尺寸"}},

	// ── agent 独有工具（原全量 15 里 deferred 的 4 个——注册进 deferred 保持可用）──
	{Name: "screenshot", Category: "GUI与预览", Keywords: []string{"截图", "屏幕", "画面"}},
	{Name: "apply_patch", Category: "文件与代码", Keywords: []string{"补丁", "批量改", "patch"}},
	{Name: "spawn_agent", Category: "任务与调度", Keywords: []string{"子代理", "派任务", "agent"}},
	{Name: "todo", Category: "任务与调度", Keywords: []string{"任务清单", "todo", "计划"}},
	// ── 虫族管理（P4-48 第三批——8）──
	{Name: "task_list", Category: "任务与调度", Keywords: []string{"任务", "队列", "task", "派任务"}},
	{Name: "task_detail", Category: "任务与调度", Keywords: []string{"任务详情", "任务进度", "看任务"}},
	{Name: "task_stats", Category: "任务与调度", Keywords: []string{"任务统计", "统计"}},
	{Name: "fleet_status", Category: "虫族系统管理", Keywords: []string{"集群", "机器", "fleet", "节点", "健康"}},
	{Name: "model_status", Category: "模型与部署", Keywords: []string{"模型", "模型库", "部署", "加载"}},
	{Name: "resource_list", Category: "虫族系统管理", Keywords: []string{"资源", "资源库", "信任度"}},
	{Name: "zerg_health", Category: "虫族系统管理", Keywords: []string{"健康", "状态", "体检", "系统检查"}},
	{Name: "zerg_version", Category: "虫族系统管理", Keywords: []string{"版本", "version", "git"}},

	// ── 效率补充（P4-48 第三批——5）──
	{Name: "tree", Category: "效率工具", Keywords: []string{"目录树", "树形", "结构"}},
	{Name: "zip_create", Category: "效率工具", Keywords: []string{"压缩", "打包", "zip"}},
	{Name: "zip_extract", Category: "效率工具", Keywords: []string{"解压", "unzip", "zip"}},
	{Name: "csv_view", Category: "数据存储", Keywords: []string{"csv", "表格", "电子表格"}},
	{Name: "json_path", Category: "数据存储", Keywords: []string{"json", "提取", "键路径"}},

	// ── 剪辑增强（P4-48 第四批——4）──
	{Name: "footage_search", Category: "影音剪辑", Keywords: []string{"素材", "搜索素材", "找素材", "footage"}},
	{Name: "audio_gain", Category: "影音剪辑", Keywords: []string{"音频", "增益", "音量", "响度"}},
	{Name: "subtitle_srt", Category: "影音剪辑", Keywords: []string{"字幕", "srt", "subtitle"}},
	{Name: "media_duration", Category: "影音剪辑", Keywords: []string{"时长", "媒体信息", "ffprobe"}},

	// ── 文件批处理（P4-48 第四批——3）──
	{Name: "batch_rename", Category: "文件与代码", Keywords: []string{"批量重命名", "改名", "重命名"}},
	{Name: "file_count", Category: "文件与代码", Keywords: []string{"文件统计", "扩展名", "数量"}},
	{Name: "text_freq", Category: "效率工具", Keywords: []string{"词频", "统计", "频率"}},

	// ── 数据（P4-48 第四批——2）──
	{Name: "db_schema", Category: "数据存储", Keywords: []string{"数据库", "表结构", "schema", "sqlite"}},
	{Name: "log_analyze", Category: "效率工具", Keywords: []string{"日志", "分析", "错误统计"}},

	// ── 网络（P4-48 第四批——2）──
	{Name: "download", Category: "网络与调研", Keywords: []string{"下载", "文件下载", "curl"}},
	{Name: "net_info", Category: "网络与调研", Keywords: []string{"网络", "IP", "接口"}},

	// ── 多模态（P4-48 第四批——1）──
	{Name: "image_ocr", Category: "多模态", Keywords: []string{"ocr", "图片文字", "识别"}},

}

// chatToolMetaByName — 工具名 → 元数据
var chatToolMetaByName = func() map[string]ChatToolMeta {
	m := make(map[string]ChatToolMeta, len(chatToolRegistry))
	for _, meta := range chatToolRegistry {
		m[meta.Name] = meta
	}
	return m
}()

// L0ToolNames — 常驻工具名集合（主提示只出这些——小模型决策负担 12 选 1）
var L0ToolNames = func() map[string]bool {
	m := make(map[string]bool)
	for _, meta := range chatToolRegistry {
		if meta.L0 {
			m[meta.Name] = true
		}
	}
	return m
}()

// DeferredToolNames — deferred 工具名集合（tool_search 发现）
var DeferredToolNames = func() map[string]bool {
	m := make(map[string]bool)
	for _, meta := range chatToolRegistry {
		if !meta.L0 {
			m[meta.Name] = true
		}
	}
	return m
}()

// ChatToolSearch — 对话版 tool_search（P4-42——分类索引 + 中文关键词——CA v2.5.1 增强版）
// 返回: 匹配的工具名（≤maxReturn——CA 验证值 8——不一次全给）
func ChatToolSearch(query string, have map[string]bool, maxReturn int) []string {
	if maxReturn <= 0 {
		maxReturn = 8
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	// 1. 分类匹配（query 命中类别名——如"系统""剪辑"→ 该类全部）
	var byCategory []string
	var byKeyword []string
	seen := map[string]bool{}
	queryWords := strings.Fields(q)

	for _, meta := range chatToolRegistry {
		if have[meta.Name] || seen[meta.Name] {
			continue
		}
		// 类别匹配
		if strings.Contains(strings.ToLower(meta.Category), q) {
			byCategory = append(byCategory, meta.Name)
			seen[meta.Name] = true
			continue
		}
		// 工具名匹配
		if strings.Contains(strings.ToLower(meta.Name), q) {
			byKeyword = append(byKeyword, meta.Name)
			seen[meta.Name] = true
			continue
		}
		// 关键词匹配（任一查询词命中任一关键词）
		for _, kw := range meta.Keywords {
			for _, w := range queryWords {
				if strings.Contains(kw, w) || strings.Contains(w, kw) {
					byKeyword = append(byKeyword, meta.Name)
					seen[meta.Name] = true
					break
				}
			}
			if seen[meta.Name] {
				break
			}
		}
	}
	// 类别优先（更精准——CA 教训: 按 server 分类）
	out := append(byCategory, byKeyword...)
	if len(out) > maxReturn {
		out = out[:maxReturn]
	}
	sort.Strings(out)
	return out
}

// IsExtraTool — 判断是否为 deferred 新工具（chat 层实现——ExecuteChatTool 可执行）
func IsExtraTool(name string) bool {
	return DeferredToolNames[name] && !agentExtraTools[name]
}

// agentExtraTools — agent 层已有实现（走 ec.ExecuteTool——不归 chat 执行器）
var agentExtraTools = map[string]bool{
	"screenshot": true, "apply_patch": true, "spawn_agent": true, "todo": true,
}

// ChatToolCategoryOf — 工具类别（tool_search 结果展示用）
func ChatToolCategoryOf(name string) string {
	if meta, ok := chatToolMetaByName[name]; ok {
		return meta.Category
	}
	return "其他"
}
