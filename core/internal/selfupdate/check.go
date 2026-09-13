package selfupdate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// checkCacheSeconds —— 6 小时窗口（照抄 Hermes banner.py 的 _UPDATE_CHECK_CACHE_SECONDS）。
// 理由：更新检查会打网络；每次启动/每次开面板都 fetch 既慢又会撞 GitHub 的匿名限流。
const checkCacheSeconds = 6 * 3600

// 检查结论。
const (
	StatusUpToDate     = "up-to-date"   // 目标 == 本地构建 sha
	StatusBehind       = "behind"       // 远端领先（可更新）
	StatusLocalAhead   = "local-ahead"  // 本地领先/脏树（开发态；不催更新——G7）
	StatusDiverged     = "diverged"     // 分叉（本地与远端各有提交）
	StatusInconclusive = "inconclusive" // fetch 失败等——**不缓存**（Hermes #82166）
)

// CheckResult —— 一次检查的完整结论。
type CheckResult struct {
	Status    string `json:"status"`
	LocalSHA  string `json:"local_sha"`
	RemoteSHA string `json:"remote_sha"`
	Behind    int    `json:"behind"` // 落后提交数；判不出为 -1
	Ahead     int    `json:"ahead"`  // 领先提交数（开发态）；判不出为 -1
	Dirty     bool   `json:"dirty"`
	Source    string `json:"source"`  // "cache" | "live"
	Message   string `json:"message"` // 人读一行
}

// Updatable —— 是否"确实有新版可装"（Behind / Diverged）。
func (r CheckResult) Updatable() bool {
	return r.Status == StatusBehind || r.Status == StatusDiverged
}

// CachePath —— 更新检查缓存文件（`~/.zerg/state/update_check.json`）。
func CachePath(stateDir string) string { return filepath.Join(stateDir, "update_check.json") }

type cacheEntry struct {
	TS        int64  `json:"ts"`
	Version   string `json:"version"`
	LocalSHA  string `json:"local_sha"`
	RemoteSHA string `json:"remote_sha"`
	Status    string `json:"status"`
	Behind    int    `json:"behind"`
	Ahead     int    `json:"ahead"`
}

// readCache —— 读缓存；窗口过期 / 本地 sha 或版本变了 ⇒ 视为无缓存。
func readCache(path, localSHA, version string) *CheckResult {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var e cacheEntry
	if err := json.Unmarshal(b, &e); err != nil {
		return nil
	}
	if time.Now().Unix()-e.TS >= checkCacheSeconds {
		return nil
	}
	if e.LocalSHA != localSHA || e.Version != version {
		return nil // 本机代码或版本变了 ⇒ 缓存作废
	}
	if e.Status == StatusInconclusive || e.Status == "" {
		return nil
	}
	return &CheckResult{
		Status: e.Status, LocalSHA: e.LocalSHA, RemoteSHA: e.RemoteSHA,
		Behind: e.Behind, Ahead: e.Ahead, Source: "cache",
	}
}

// writeCache —— 只写**确定**的结论；不确定（inconclusive）绝不落盘（Hermes #82166）。
func writeCache(path, version string, r CheckResult) error {
	if r.Status == StatusInconclusive || r.Status == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	e := cacheEntry{
		TS: time.Now().Unix(), Version: version, LocalSHA: r.LocalSHA,
		RemoteSHA: r.RemoteSHA, Status: r.Status, Behind: r.Behind, Ahead: r.Ahead,
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// updatesCheckEnabled —— `updates.check`（fleet.yaml）；缺省 true。
// 环境变量 ZERG_UPDATES_CHECK=0 可强制关闭（测试/离线场景的逃生门）。
func updatesCheckEnabled(fleetYAML string) bool {
	if v := strings.TrimSpace(os.Getenv("ZERG_UPDATES_CHECK")); v != "" {
		return v != "0" && strings.ToLower(v) != "false"
	}
	b, err := os.ReadFile(fleetYAML)
	if err != nil {
		return true // 读不到配置 = 不阻断（默认开）
	}
	var doc struct {
		Updates struct {
			Check *bool `yaml:"check"`
		} `yaml:"updates"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return true
	}
	if doc.Updates.Check != nil {
		return *doc.Updates.Check
	}
	return true
}

// Compare —— 纯比较函数（可单测，不碰网络）：给定 local/remote 两个 sha 与祖先关系判定，
// 得出状态。`ancestor` 为 (isAncestor, ok)；ok=false 表示跨浅边界判不出（不猜）。
func Compare(local, remote string, ancestor func(a, b string) (bool, bool), aheadCount int) CheckResult {
	r := CheckResult{LocalSHA: local, RemoteSHA: remote, Behind: -1, Ahead: -1}
	if local == "" || remote == "" {
		r.Status = StatusInconclusive
		r.Message = "取不到本地或远端的代码身份"
		return r
	}
	if local == remote {
		r.Status = StatusUpToDate
		r.Behind = 0
		r.Ahead = 0
		return r
	}
	remoteInLocal, ok1 := ancestor(remote, local)
	if ok1 && remoteInLocal {
		// 远端是本地祖先 ⇒ 本地领先（开发态）
		r.Status = StatusLocalAhead
		r.Ahead = aheadCount
		r.Behind = 0
		return r
	}
	localInRemote, ok2 := ancestor(local, remote)
	if ok2 && localInRemote {
		r.Status = StatusBehind
		r.Behind = aheadCount
		r.Ahead = 0
		return r
	}
	if ok1 && ok2 {
		// 两者都明确不是对方的祖先 ⇒ 分叉
		r.Status = StatusDiverged
		r.Ahead = aheadCount
		return r
	}
	// 判不出祖先关系：tip 不同 ⇒ 保守按"有新版"处理（宁可提示，不静默漏更新）。
	r.Status = StatusBehind
	r.Message = "历史不足（浅检出跨边界）——按 tip 不同保守判定为有新版"
	return r
}

// Check —— 一次更新检查（带缓存）。repo 为本地检出；remote 为远端名/URL；ref 为分支/ref。
//
// 纪律（逐条对齐 Hermes banner.py）：
//
//	· 6 小时缓存；本地 sha / 版本变化 ⇒ 缓存作废
//	· fetch 失败 ⇒ 结论=inconclusive ⇒ **不写缓存**（下次立即重试）
//	· 本地领先/脏树 ⇒ 报"本地领先（开发态）"而非催更新（G7）
func Check(g Git, remote, ref, stateDir, version string, useCache bool) CheckResult {
	local, err := g.HeadSHA()
	if err != nil {
		return CheckResult{Status: StatusInconclusive, Source: "live", Message: "读不到本地 HEAD：" + err.Error()}
	}
	cp := CachePath(stateDir)
	if useCache {
		if c := readCache(cp, local, version); c != nil {
			c.Dirty = g.IsDirty()
			c.Message = messageFor(*c, version)
			return *c
		}
	}

	remoteSHA, ferr := g.FetchTarget(remote, ref)
	if ferr != nil {
		// 不确定 ⇒ 不缓存（Hermes #82166）；明确告知失败原因
		return CheckResult{
			Status: StatusInconclusive, LocalSHA: local, Source: "live",
			Message: "无法确认远端状态（fetch 失败）：" + ferr.Error(),
		}
	}

	res := Compare(local, remoteSHA, g.IsAncestor, 0)
	res.Source = "live"
	res.Dirty = g.IsDirty()
	// 补齐提交数（用真实 rev-list，越界时保 -1）
	switch res.Status {
	case StatusBehind:
		if n := g.CountAhead(local, remoteSHA); n >= 0 {
			res.Behind = n
		}
	case StatusLocalAhead:
		if n := g.CountAhead(remoteSHA, local); n >= 0 {
			res.Ahead = n
		} else if n := g.CountAhead(remoteSHA, "HEAD"); n >= 0 {
			res.Ahead = n
		}
	case StatusDiverged:
		if n := g.CountAhead(remoteSHA, local); n >= 0 {
			res.Ahead = n
		}
	}
	res.Message = messageFor(res, version)

	if useCache {
		_ = writeCache(cp, version, res)
	}
	return res
}

// messageFor —— 人读一行（开发态不催更新，G7）。
func messageFor(r CheckResult, version string) string {
	switch r.Status {
	case StatusUpToDate:
		return fmt.Sprintf("已是最新（%s @ %s）", version, short(r.LocalSHA))
	case StatusBehind:
		if r.Behind >= 0 {
			return fmt.Sprintf("有新版：落后远端 %d 笔（本地 %s → 远端 %s）", r.Behind, short(r.LocalSHA), short(r.RemoteSHA))
		}
		return fmt.Sprintf("有新版（%s → %s）", short(r.LocalSHA), short(r.RemoteSHA))
	case StatusLocalAhead:
		n := r.Ahead
		extra := ""
		if r.Dirty {
			extra = "，工作树有未提交改动"
		}
		if n >= 0 {
			return fmt.Sprintf("本地领先 %d 笔（开发态）%s——不提示更新", n, extra)
		}
		return fmt.Sprintf("本地领先远端（开发态）%s——不提示更新", extra)
	case StatusDiverged:
		return fmt.Sprintf("本地与远端已分叉（本地 %s / 远端 %s）——需 --to/--force 才继续", short(r.LocalSHA), short(r.RemoteSHA))
	default:
		return r.Message
	}
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
