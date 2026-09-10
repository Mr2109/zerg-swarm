# 工具目录（TOOLS）

> 本文件由工具注册表**自动生成**（`core/internal/chat/chat_tool_registry.go`）——
> 改工具清单请改注册表，不要手改本文件。
> 统计：**135** 个注册工具，其中 **14** 个为 L0 常驻（直接写进模型提示），其余按需发现。

## 两层工具

| 层 | 数量 | 说明 |
|---|---|---|
| **L0 常驻** | 14 | 高频基础能力，定义直接注入模型提示，随时可调 |
| **延迟加载** | 121 | 通过 `tool_search`（按关键词/类别）发现后再调用，避免提示膨胀 |

调用约定：模型先看提示里有没有；没有就用 `tool_search` 搜（支持中文关键词），拿到定义再调。

## 按类别

### 影音与剪辑（25）

- `audio_extract`
- `audio_sync_check`
- `fcpx_add_lut`
- `fcpx_check`
- `fcpx_experience`
- `fcpx_fix_multicam`
- `fcpx_fix_pcut02`
- `fcpx_verify_sync`
- `fcpx_xml_validate`
- `ffmpeg_transcode`
- `footage_match`
- `footage_poly_wavs`
- `footage_resolve_pool`
- `footage_stats`
- `footage_timeline`
- `footage_video_clips`
- `media_info`
- `media_probe`
- `multicam_check`
- `resample_audio`
- `resolve_drp_check`
- `sample_rate_check`
- `subtitle_generate`
- `timecode_convert`
- `video_extract`

### 文件与代码（20）

- `apply_patch`
- `bash`  ← **L0 常驻**
- `batch_rename`
- `copy_file`
- `delete_file`
- `diff_files`
- `du_usage`
- `edit`  ← **L0 常驻**
- `file_count`
- `file_type`
- `find_name`
- `glob`  ← **L0 常驻**
- `grep`  ← **L0 常驻**
- `head_file`
- `ls`  ← **L0 常驻**
- `move_file`
- `read`  ← **L0 常驻**
- `stat_file`
- `tail_file`
- `write`  ← **L0 常驻**

### 效率工具（15）

- `base64_codec`
- `calc`
- `hash_calc`
- `json_format`
- `log_analyze`
- `regex_test`
- `text_freq`
- `text_stats`
- `timestamp_convert`
- `tree`
- `unit_convert`
- `url_codec`
- `yaml_json`
- `zip_create`
- `zip_extract`

### 系统管理（15）

- `cpu_status`
- `disk_usage`
- `gpu_status`
- `kill_process`
- `log_tail`
- `mem_status`
- `network_status`
- `ping_check`
- `port_check`
- `process_list`
- `service_status`
- `start_service`
- `stop_service`
- `sys_info`
- `uptime_info`

### 音乐下载（12）

- `music_album_search`
- `music_batch_download`
- `music_cover_get`
- `music_download_kuwo`
- `music_download_netease`
- `music_download_qq`
- `music_merge_check`
- `music_parse_share`
- `music_save_info`
- `music_search_kuwo`
- `music_search_netease`
- `music_search_qq`

### 知识库（8）

- `capture_fix`
- `doc_search`  ← **L0 常驻**
- `kb_add`
- `kb_categories`
- `kb_read`
- `kb_search`  ← **L0 常驻**
- `kb_stats`
- `skill_load`  ← **L0 常驻**

### 网络与调研（7）

- `anysearch`
- `download`
- `http_get`
- `net_info`
- `url_extract`
- `web_fetch`  ← **L0 常驻**
- `web_search`  ← **L0 常驻**

### 多模态（6）

- `asr_transcribe`
- `image_desc`
- `image_ocr`
- `image_resize`
- `ocr_text`
- `tts_speak`

### 任务与调度（5）

- `spawn_agent`
- `task_detail`
- `task_list`
- `task_stats`
- `todo`

### 数据与存储（5）

- `backup_file`
- `db_tables`
- `export_json`
- `import_json`
- `sql_query`

### 影音剪辑（4）

- `audio_gain`
- `footage_search`
- `media_duration`
- `subtitle_srt`

### 虫族系统管理（4）

- `fleet_status`
- `resource_list`
- `zerg_health`
- `zerg_version`

### 数据存储（3）

- `csv_view`
- `db_schema`
- `json_path`

### 系统（2）

- `tool_search`  ← **L0 常驻**
- `zerg_overview`  ← **L0 常驻**

### 记忆（2）

- `memory`
- `session_search`

### GUI与预览（1）

- `screenshot`

### 模型与部署（1）

- `model_status`

## 工具自身文档（履历）

每个工具的**版本、能力、参数、注意事项**写在 `tools/<name>.md`（履历），
与注册表、工具简介三者应保持一致——这是本项目的硬约定（见 `docs/design/工具升级规范.md`）。

## 安全相关

- shell（`bash`）的删除类命令受**范围闸门**约束：仅允许任务工作区与 `ZERG_EXTRA_ALLOW_DIR` 白名单
- 文件类工具（`read`/`write`/`edit`/`glob`）同样受路径域限制，越界返回**可执行的替代路径**而非空报错
- 参数格式错误会触发**格式反馈协议**：返回「系统断言 + 正确示例」，避免模型重发同样的坏请求

## 重新生成

```bash
python3 tools/gen_tools_md.py > docs/TOOLS.md
```

