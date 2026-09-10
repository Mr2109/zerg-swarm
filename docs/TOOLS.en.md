# Tool Catalog (TOOLS)

> **English** | [中文](TOOLS.zh-CN.md)
>
> *Source of truth: the Chinese original ([TOOLS.zh-CN.md](TOOLS.zh-CN.md)). If the two disagree, the Chinese text prevails.*
> *Note: the tool **descriptions** below are the exact text sent to the model and are currently Chinese — descriptions are deliberately NOT translated in this release (see the tool-description decision in the project design doc). Tool names, parameters and categories are stable identifiers.*

> This file is **auto-generated** from the tool registry (`core/internal/chat/chat_tool_registry.go`) —
> To change the tool list, change the registry — do not hand-edit this file.
> Stats: **135** registered tools, of which **14** are L0 always-on (written straight into the model prompt); the rest are discovered on demand.

## Two tiers of tools

| Tier | Count | Description |
|---|---|---|
| **L0 always-on** | 14 | High-frequency primitives; their definitions are injected straight into the model prompt and are always callable |
| **Deferred** | 121 | Discovered via `tool_search` (by keyword / category) and only then called, to keep the prompt from bloating |

Calling convention: the model first checks whether the prompt already carries the tool; if not, it searches with `tool_search` (Chinese keywords supported), gets the definition, then calls it.

## By category

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

## Per-tool documentation (résumé)

Each tool's **version, capabilities, parameters and caveats** are written in `tools/<name>.md` (its résumé),
and should stay consistent with the registry and the tool blurb — a hard convention in this project (see `docs/design/工具升级规范.md`).

## Security

- Deletion-style commands in the shell (`bash`) are bound by the **scope gate**: only the task workspace and the `ZERG_EXTRA_ALLOW_DIR` allowlist are permitted
- File tools (`read`/`write`/`edit`/`glob`) are path-domain-limited too; an out-of-bounds request returns an **actionable alternative path** rather than an empty error
- Malformed arguments trigger the **Format Feedback Protocol**: it returns a "system assertion + a correct example" so the model does not resend the same bad request

## Regenerate

```bash
python3 tools/gen_tools_md.py > docs/TOOLS.zh-CN.md   # keep docs/TOOLS.en.md in sync in the same PR
```

