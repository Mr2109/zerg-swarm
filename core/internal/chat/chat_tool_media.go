// chat_tool_media.go — v2.5.7 P4-43 影音剪辑 + 音乐下载工具（第二批——fcpx skill 脚本链 + musicdl 包装）
// 原则: 真实可用——脚本/环境不存在返回明确提示（不假装）

package chat

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// 环境路径（fcpx skill——AutoCUT 项目）
var (
	autoCutDir  = os.Getenv("ZERG_AUTOCUT_DIR") // 可选：AutoCUT 目录（2026-09-11 B 批：去硬编码）
	footageDB   = autoCutDir + "/data/footage.db"
	fcpxExpFile = os.Getenv("ZERG_FCPX_EXP_FILE") // 可选：FCPX 互通经验文件（空=不注入）
	musicSave   = os.Getenv("ZERG_MUSIC_DIR")     // 可选：音乐下载目录
)

// ═══════════════ 影音剪辑第二批 ═══════════════

// footageQuery — footage.db 只读查询（表名白名单——防注入）
func footageQuery(table, cond string, limit int) (string, error) {
	if _, err := os.Stat(footageDB); err != nil {
		return "", fmt.Errorf("footage.db 不存在（%s——fcpx skill 数据未就绪）", footageDB)
	}
	allowed := map[string]bool{"video_clips": true, "poly_wavs": true, "match_result": true, "resolve_pool": true, "resolve_timeline": true}
	if !allowed[table] {
		return "", fmt.Errorf("不允许的表: %s（白名单: video_clips/poly_wavs/match_result/resolve_pool/resolve_timeline）", table)
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := fmt.Sprintf("SELECT * FROM %s", table)
	if cond != "" {
		q += " WHERE " + cond
	}
	q += fmt.Sprintf(" LIMIT %d", limit)
	out, err := exec.Command("sh", "-c", fmt.Sprintf("sqlite3 -header -column %q %q 2>&1 | head -60", footageDB, q)).Output()
	if err != nil {
		return "", fmt.Errorf("查询失败: %v", err)
	}
	return truncateArgs(string(out), 3000), nil
}

func footageVideoClips(args map[string]any) (string, error) {
	cond, _ := args["cond"].(string)
	limit := 10
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	return footageQuery("video_clips", cond, limit)
}

func footagePolyWavs(args map[string]any) (string, error) {
	cond, _ := args["cond"].(string)
	limit := 10
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	return footageQuery("poly_wavs", cond, limit)
}

func footageMatch(args map[string]any) (string, error) {
	cond, _ := args["cond"].(string)
	limit := 10
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	return footageQuery("match_result", cond, limit)
}

func footageResolvePool(args map[string]any) (string, error) {
	cond, _ := args["cond"].(string)
	limit := 10
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	return footageQuery("resolve_pool", cond, limit)
}

func footageTimeline(args map[string]any) (string, error) {
	cond, _ := args["cond"].(string)
	limit := 10
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	return footageQuery("resolve_timeline", cond, limit)
}

func footageStats() (string, error) {
	if _, err := os.Stat(footageDB); err != nil {
		return "", fmt.Errorf("footage.db 不存在（%s）", footageDB)
	}
	out, err := exec.Command("sh", "-c", fmt.Sprintf("sqlite3 %q \"SELECT 'video_clips', COUNT(*) FROM video_clips UNION ALL SELECT 'poly_wavs', COUNT(*) FROM poly_wavs UNION ALL SELECT 'match_result', COUNT(*) FROM match_result UNION ALL SELECT 'resolve_pool', COUNT(*) FROM resolve_pool UNION ALL SELECT 'timeline', COUNT(*) FROM resolve_timeline;\" 2>&1", footageDB)).Output()
	if err != nil {
		return "", fmt.Errorf("统计失败: %v", err)
	}
	return string(out), nil
}

// fcpxRunScript — 跑 AutoCUT 脚本（PYTHONPATH= .venv/bin/python 前缀——fcpx skill 规定）
func fcpxRunScript(script string, args ...string) (string, error) {
	scriptPath := filepath.Join(autoCutDir, "scripts", script)
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("脚本不存在: %s（fcpx skill 未部署？）", scriptPath)
	}
	venvPy := filepath.Join(autoCutDir, ".venv", "bin", "python")
	// 直接用 venv python 跑脚本文件
	fullArgs := append([]string{scriptPath}, args...)
	cmd := exec.Command(venvPy, fullArgs...)
	cmd.Dir = autoCutDir
	env := append(os.Environ(), "PYTHONPATH=")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("脚本执行失败: %v", err)
	}
	return truncateArgs(string(out), 4000), nil
}

func fcpxAddLut(args map[string]any) (string, error) {
	src, _ := args["src"].(string)
	out, _ := args["out"].(string)
	if src == "" {
		return "", fmt.Errorf("参数 src（输入 XML）不能为空")
	}
	if out == "" {
		out = strings.TrimSuffix(src, ".xml") + "_lut.xml"
	}
	return fcpxRunScript("add_lut.py", src, out)
}

func fcpxFixMulticam(args map[string]any) (string, error) {
	src, _ := args["src"].(string)
	out, _ := args["out"].(string)
	rids, _ := args["rids"].(string)
	if src == "" {
		return "", fmt.Errorf("参数 src（输入 XML）不能为空")
	}
	if out == "" {
		out = strings.TrimSuffix(src, ".xml") + "_fixed.xml"
	}
	// rids 可选（不传则脚本内自动提取）
	if rids == "" {
		return fcpxRunScript("fix_multicam_audio.py", "--no-tcstart", "--src", src, "--out", out)
	}
	return fcpxRunScript("fix_multicam_audio.py", "--rids", rids, "--no-tcstart", "--src", src, "--out", out)
}

func fcpxFixPcut02(args map[string]any) (string, error) {
	src, _ := args["src"].(string)
	out, _ := args["out"].(string)
	scriptArgs := []string{}
	if src != "" {
		scriptArgs = append(scriptArgs, "--src", src)
	}
	if out != "" {
		scriptArgs = append(scriptArgs, "--out", out)
	}
	return fcpxRunScript("fix_pcut02_audio.py", scriptArgs...)
}

func fcpxVerifySync(args map[string]any) (string, error) {
	src, _ := args["src"].(string)
	scriptArgs := []string{}
	if src != "" {
		scriptArgs = append(scriptArgs, "--src", src)
	}
	return fcpxRunScript("verify_sync.py", scriptArgs...)
}

func fcpxXmlValidate(args map[string]any) (string, error) {
	src, _ := args["src"].(string)
	if src == "" {
		return "", fmt.Errorf("参数 src（XML 路径）不能为空")
	}
	out, err := exec.Command("sh", "-c", fmt.Sprintf("xmllint --noout %q 2>&1 && echo 'XML 合法'", src)).Output()
	if err != nil {
		return string(out), fmt.Errorf("XML 校验失败（详情见输出）")
	}
	return string(out), nil
}

func fcpxExperience(args map[string]any) (string, error) {
	kw, _ := args["keyword"].(string)
	if _, err := os.Stat(fcpxExpFile); err != nil {
		return "", fmt.Errorf("经验文件不存在: %s", fcpxExpFile)
	}
	data, err := os.ReadFile(fcpxExpFile)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if kw == "" {
		return strings.Join(lines[:min(60, len(lines))], "\n"), nil
	}
	var hit []string
	for i, l := range lines {
		if strings.Contains(l, kw) {
			start := i - 2
			if start < 0 {
				start = 0
			}
			end := i + 3
			if end > len(lines) {
				end = len(lines)
			}
			hit = append(hit, strings.Join(lines[start:end], "\n"))
			if len(hit) >= 5 {
				break
			}
		}
	}
	if len(hit) == 0 {
		return fmt.Sprintf("经验文件中无 %q 相关内容（全文 %d 行）", kw, len(lines)), nil
	}
	return strings.Join(hit, "\n\n---\n\n"), nil
}

func resolveDrpCheck(args map[string]any) (string, error) {
	drp, _ := args["path"].(string)
	if drp == "" {
		drp = os.Getenv("ZERG_DRP_FILE") // 可选：.drp 工程文件
	}
	info, err := os.Stat(drp)
	if err != nil {
		return "", fmt.Errorf("DRP 不存在: %s", drp)
	}
	return fmt.Sprintf("DRP 工程: %s\n大小: %.1f MB\n修改: %s\n（190MB 非纯 sqlite——需达芬奇打开）",
		drp, float64(info.Size())/1024/1024, info.ModTime().Format("2006-01-02 15:04")), nil
}

func mediaProbe(args map[string]any, workDir string) (string, error) {
	p, _ := args["path"].(string)
	if p == "" {
		return "", fmt.Errorf("参数 path 不能为空")
	}
	full, err := resolvePath(p, workDir)
	if err != nil {
		return "", err
	}
	// ffprobe 详细——含视频 start_time/timecode（多机位同步判据）
	out, err := exec.Command("sh", "-c",
		fmt.Sprintf("ffprobe -v error -show_format -show_streams -of json %q | python3 -c \"import sys,json; d=json.load(sys.stdin); [print(f\\\"{s['codec_type']}: {s.get('codec_name','')} {s.get('width','')}x{s.get('height','')} start={s.get('start_time','?')} tc={s.get('tags',{}).get('timecode','无')} dur={s.get('duration','?')} sr={s.get('sample_rate','')}ch={s.get('channels','')}\\\") for s in d['streams']]; print(f\\\"format: dur={d['format'].get('duration','?')} size={d['format'].get('size','?')}\\\")\"", full)).Output()
	if err != nil {
		return "", fmt.Errorf("ffprobe 失败: %v", err)
	}
	return string(out), nil
}

func audioSyncCheck(args map[string]any, workDir string) (string, error) {
	video, _ := args["video"].(string)
	audio, _ := args["audio"].(string)
	if video == "" || audio == "" {
		return "", fmt.Errorf("参数 video/audio 不能为空")
	}
	pv, err := resolvePath(video, workDir)
	if err != nil {
		return "", err
	}
	pa, err := resolvePath(audio, workDir)
	if err != nil {
		return "", err
	}
	// 提取两边 start_time 对比（音频对齐判据——start 差应≈0）
	out, err := exec.Command("sh", "-c",
		fmt.Sprintf("echo '视频:' && ffprobe -v error -show_entries format=start_time -of csv=p=0 %q && echo '音频:' && ffprobe -v error -show_entries format=start_time -of csv=p=0 %q", pv, pa)).Output()
	if err != nil {
		return "", fmt.Errorf("探测失败: %v", err)
	}
	return string(out) + "\n（多机位同步判据: start 差≈0——差大则需音频适应视频——见 fcpx skill）", nil
}

// ═══════════════ 音乐下载第二批 ═══════════════

// qqMusicSearch — QQ 搜索（skill 流程——c.y.qq.com API）
func qqMusicSearch(kw string, n int) (string, error) {
	if n <= 0 || n > 20 {
		n = 5
	}
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf(`curl -s "https://c.y.qq.com/soso/fcgi-bin/client_search_cp?p=1&n=%d&w=%s&format=json" -H "Referer: https://y.qq.com/" -H "User-Agent: Mozilla/5.0" | python3 -c "import sys,json; d=json.load(sys.stdin); songs=d.get('data',{}).get('song',{}).get('list',[]); [print(f\\\"{s['songname']} | {','.join(x['name'] for x in s['singer'])} | {s['songmid']} | {s.get('albumname','')}\\\") for s in songs]"`, n, kw))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("QQ 搜索失败: %v", err)
	}
	if len(out) == 0 {
		return "QQ 搜索无结果", nil
	}
	return string(out), nil
}

func musicSearchQQ(args map[string]any) (string, error) {
	kw, _ := args["keyword"].(string)
	if kw == "" {
		return "", fmt.Errorf("参数 keyword 不能为空")
	}
	return qqMusicSearch(kw, 5)
}

func musicAlbumSearch(args map[string]any) (string, error) {
	kw, _ := args["keyword"].(string)
	if kw == "" {
		return "", fmt.Errorf("参数 keyword（专辑名+歌手）不能为空")
	}
	return qqMusicSearch(kw, 10)
}

func musicParseShare(args map[string]any) (string, error) {
	link, _ := args["link"].(string)
	if link == "" {
		return "", fmt.Errorf("参数 link（微信分享 songDetail 链接）不能为空")
	}
	// 提取 songmid（songDetail/XXX 或 songmid=XXX）
	re := regexp.MustCompile(`songDetail/([A-Za-z0-9]+)`)
	m := re.FindStringSubmatch(link)
	if len(m) < 2 {
		re2 := regexp.MustCompile(`songmid=([A-Za-z0-9]+)`)
		m2 := re2.FindStringSubmatch(link)
		if len(m2) < 2 {
			return "", fmt.Errorf("链接中未找到 songmid（格式: songDetail/000hXXX）")
		}
		m = m2
	}
	songmid := m[1]
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf(`curl -s "https://c.y.qq.com/v8/fcg-bin/fcg_play_single_song.fcg?songmid=%s&format=json" -H "Referer: https://y.qq.com/" -H "User-Agent: Mozilla/5.0" | python3 -c "import sys,json; d=json.load(sys.stdin); s=d.get('data',[{}])[0]; print(f\\\"{s.get('songname','?')} | {','.join(x['name'] for x in s.get('singer',[]))} | {s.get('songmid','?')} | {s.get('album',{}).get('name','')}\\\")"`, songmid))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("链接解析失败: %v", err)
	}
	return fmt.Sprintf("songmid=%s\n%s", songmid, string(out)), nil
}

func neteaseMusicSearch(kw string) (string, error) {
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf(`curl -s "https://music.163.com/api/search/get/web?s=%s&type=1&limit=5&offset=0" -H "User-Agent: Mozilla/5.0" | python3 -c "import sys,json; d=json.load(sys.stdin); songs=d.get('result',{}).get('songs',[]); [print(f\\\"{s['name']} | {','.join(a['name'] for a in s.get('artists',[]))} | id={s['id']} | {s.get('album',{}).get('name','')}\\\") for s in songs]"`, kw))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("网易云搜索失败: %v", err)
	}
	if len(out) == 0 {
		return "网易云搜索无结果", nil
	}
	return string(out), nil
}

func musicSearchNetease(args map[string]any) (string, error) {
	kw, _ := args["keyword"].(string)
	if kw == "" {
		return "", fmt.Errorf("参数 keyword 不能为空")
	}
	return neteaseMusicSearch(kw)
}

func musicSearchKuwo(args map[string]any) (string, error) {
	kw, _ := args["keyword"].(string)
	if kw == "" {
		return "", fmt.Errorf("参数 keyword 不能为空")
	}
	// musicdl KuwoMusicClient（skill 流程——加 signal.alarm 防卡死）
	py := `import signal, sys, json
signal.alarm(30)
from musicdl import MusicClient
api = MusicClient(music_sources=["KuwoMusicClient"], init_music_clients_cfg={})
res = api.search("` + kw + `")
for s in res.get("KuwoMusicClient", [])[:8]:
    print(s.get("name","?"), "|", s.get("artist","?"), "|", s.get("source_url","?"))
`
	out, err := exec.Command("sh", "-c", fmt.Sprintf(`python3 -c %q 2>&1`, py)).Output()
	if err != nil {
		return "", fmt.Errorf("酷我搜索失败（musicdl 环境）: %v", err)
	}
	return string(out), nil
}

func musicDownloadNetease(args map[string]any) (string, error) {
	id := int64(0)
	if v, ok := args["id"].(float64); ok {
		id = int64(v)
	}
	if id <= 0 {
		return "", fmt.Errorf("参数 id（网易云歌曲 id）不能为空——先 music_search_netease 拿 id")
	}
	// enhance/player/url 拿 URL（VIP 返回 null——换源）
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf(`curl -s "https://music.163.com/api/song/enhance/player/url?ids=[%d]&br=320000" -H "User-Agent: Mozilla/5.0" | python3 -c "import sys,json; d=json.load(sys.stdin); u=d.get('data',[{}])[0].get('url'); print(u or 'VIP无URL——换QQ源')"`, id))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取 URL 失败: %v", err)
	}
	u := strings.TrimSpace(string(out))
	if u == "" || strings.Contains(u, "VIP") {
		return string(out), nil
	}
	// 下载（间隔 3s 防 406——skill 规定）
	time.Sleep(3 * time.Second)
	save := musicSave
	if s, ok := args["save_dir"].(string); ok && s != "" {
		save = s
	}
	out2, err := exec.Command("sh", "-c", fmt.Sprintf(`curl -sL -o "%s/netease_%d.mp3" "%s" && echo "已下载 → %s/netease_%d.mp3"`, save, id, u, save, id)).Output()
	if err != nil {
		return "", fmt.Errorf("下载失败: %v", err)
	}
	return string(out2), nil
}

func musicSaveInfo() (string, error) {
	info, err := os.Stat(musicSave)
	if err != nil {
		return "", fmt.Errorf("下载目录不存在: %s", musicSave)
	}
	return fmt.Sprintf("音乐下载目录: %s\n已存在: %v（%.1f GB）", musicSave, true, float64(info.Size())/1024/1024/1024), nil
}

func musicMergeCheck(args map[string]any) (string, error) {
	// 检查下载目录是否有未归拢子目录（musicdl_outputs 模式）
	out, err := exec.Command("sh", "-c", fmt.Sprintf("find %q -maxdepth 2 -type d -name 'musicdl_outputs' 2>/dev/null; ls %q 2>/dev/null | head -20", musicSave, musicSave)).Output()
	if err != nil {
		return "", fmt.Errorf("检查失败: %v", err)
	}
	return string(out) + "\n（skill 归拢验证: 子目录文件移到顶层 + 重命名 歌手-歌名.ext）", nil
}

// musicDownloadGeneric — 音乐下载通用（QQ/酷我——musicdl）
func musicDownloadGeneric(kw string, source string) (string, error) {
	if kw == "" {
		return "", fmt.Errorf("参数 keyword 不能为空")
	}
	py := fmt.Sprintf(`import signal, sys
signal.alarm(120)
from musicdl import MusicClient
config = {"logfilepath": "/tmp/musicdl_chat.log", "savedir": %q}
api = MusicClient(music_sources=[%q], init_music_clients_cfg={}, config=config)
res = api.search(%q)
hits = res.get(%q, [])
if not hits:
    print("搜索无结果")
else:
    api.download(hits[:1])
    print("已下载:", hits[0].get("name"), "→ 检查下载目录")
`, musicSave, source, kw, source)
	out, err := exec.Command("sh", "-c", fmt.Sprintf(`python3 -c %q 2>&1 | tail -10`, py)).Output()
	if err != nil {
		return "", fmt.Errorf("下载失败（musicdl 环境——需 venv 装 musicdl）: %v", err)
	}
	return string(out), nil
}

func musicDownloadQQ(args map[string]any) (string, error) {
	kw, _ := args["keyword"].(string)
	return musicDownloadGeneric(kw, "QQMusicClient")
}

func musicDownloadKuwo(args map[string]any) (string, error) {
	kw, _ := args["keyword"].(string)
	return musicDownloadGeneric(kw, "KuwoMusicClient")
}

func musicCoverGet(args map[string]any) (string, error) {
	kw, _ := args["keyword"].(string)
	if kw == "" {
		return "", fmt.Errorf("参数 keyword 不能为空")
	}
	// QQ 搜索拿专辑 mid → 封面 URL
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf(`curl -s "https://c.y.qq.com/soso/fcgi-bin/client_search_cp?p=1&n=3&w=%s&format=json" -H "Referer: https://y.qq.com/" | python3 -c "import sys,json; d=json.load(sys.stdin); songs=d.get('data',{}).get('song',{}).get('list',[]); [print(f\\\"{s['songname']} cover: https://y.gtimg.cn/music/photo_new/T002R300x300M000{s.get('albummid','')}.jpg\\\") for s in songs]"`, kw))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("封面获取失败: %v", err)
	}
	return string(out), nil
}

func musicBatchDownload(args map[string]any) (string, error) {
	source, _ := args["source"].(string)
	kws, _ := args["keywords"].(string)
	if kws == "" {
		return "", fmt.Errorf("参数 keywords（逗号分隔歌曲列表）不能为空")
	}
	if source == "" {
		source = "KuwoMusicClient"
	}
	kwList := strings.Split(kws, ",")
	var results []string
	for _, kw := range kwList {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		r, err := musicDownloadGeneric(kw, source)
		if err != nil {
			results = append(results, fmt.Sprintf("%s: %v", kw, err))
		} else {
			results = append(results, fmt.Sprintf("%s: %s", kw, r))
		}
	}
	return strings.Join(results, "\n"), nil
}
