package main

// check_contract_version_diff.go —— 契约「版本间对拍」（旧版 ↔ 新版 registry.json）·
// py 真身 scripts/gates/check-contract-version-diff.py（434 行 · sha16 527dc9d6ab8d8f80）之 Go 形。
// 立项单 `R5` / 账 `G-27`。
//
// 口径四条（同 py 顶注 `:7-15`）：
//   ① 基线 = `git log -1 --format=%H -- core/internal/contract/registry.json`（上一次改动该件的提交）**及其父**
//      —— 不落新快照件 ✗（基线可以是 Git 引用）；
//   ② 比什么 = `entries[].id` 集合 + 每条 6 字段（name / truth / truth_shape / version_field / gate /
//      change_class）+ `change_flow` / `authority` **整体**；`note` / `schema` / `change_note` 不在判据面；
//   ③ 怎么出 = 一行一条差异（`<id> <字段> 旧→新`）· 零差异明写「无变更」；
//   ④ 存哪 = 只存对拍结果 —— 产物带两枚指纹（码侧 `head_sha` + 基线 rev）。
// 退路位：基线 rev 取不到（浅克隆 / 只提交过一次）⇒ 明写「无上一版可比」，不许拿工作树当基线糊过去 ✗。
//
// 三档退码（`G-27` 判据「rc=0/1 三档」）：
//   rc=0 无变更 · rc=1 有差异（**报告档 · 不是 must-be-zero 门**）· rc=2 不给结论
//   （根 / 表 / git / 基线 rev / 父提交 / 旧版件 任一读不到 ⇒ rc=2 · 本仓铁律：读不到不给结论，不许当绿 ✗）。
//
// 纪律：本件只读（不写 registry.json、不落快照件、不写状态目录；`--out` 只在你点名时才写）·
// 自带 `--self-test`（成对：正控 2 组 + 负控 11 组）· 缺读数一律 rc=2。
//
// 接线：本件已挂 `gateCheckGoEntry["check-contract-version-diff"]`（`zerg gate check` 走 inproc）；
// ✗ 不进 main.go 命令树（M2 不接线主体）。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	cvdRegistryRel = pathContractRegistry
	cvdRootAnchor  = pathVersionGo
)

var (
	cvdEntryFields  = []string{"name", "truth", "truth_shape", "version_field", "gate", "change_class"}
	cvdWholeTopKeys = []string{"authority", "change_flow"}
	cvdJSONFields   = []string{"head_sha", "base_rev", "old_entries", "new_entries", "diff_lines",
		"verdict", "worktree_dirty", "elapsed_ms"}
)

// cvdInfo —— py 侧 info 字典的 Go 形（八格一一对应）。
type cvdInfo struct {
	HeadSHA       string
	BaseRev       string
	OldEntries    int
	NewEntries    int
	DiffLines     int
	Verdict       string
	WorktreeDirty bool
	ElapsedMS     int
}

// cvdObj —— 保序的 json 对象（py 的 dict 插入序；`--out` 的键序按口径逐字）。
type cvdObj []cvdKV

type cvdKV struct {
	K string
	V any
}

// ── 小工具 ──────────────────────────────────────────────────────────────────

func cvdHasStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func cvdIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func cvdFirstLine(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(s, "\n")[0])
}

func cvdTrunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// cvdPyOSErr —— 标准库错误文案按 py 形给（`[Errno N] ...: '<路径>'`）：
// 负控里「件读不到」那一格 py 逐字打这串，Go 原生文案对不上 ⇒ 这里手工对齐。
func cvdPyOSErr(err error) string {
	var ee *exec.Error
	if errors.As(err, &ee) {
		if errors.Is(ee.Err, exec.ErrNotFound) {
			return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", ee.Name)
		}
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		if errors.Is(pe.Err, os.ErrNotExist) {
			return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", pe.Path)
		}
		if errors.Is(pe.Err, os.ErrPermission) {
			return fmt.Sprintf("[Errno 13] Permission denied: '%s'", pe.Path)
		}
	}
	return err.Error()
}

func cvdPyErrText(err error) string {
	if err == nil {
		return ""
	}
	return cvdPyOSErr(err)
}

func cvdErrOr(err error, def string) string {
	if err == nil {
		return def
	}
	return cvdPyErrText(err)
}

// cvdPyStr —— py `str(x)` 的常用格（id 面用）。
func cvdPyStr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		return cvdPyNum(t)
	case int:
		return strconv.Itoa(t)
	default:
		return fmt.Sprint(v)
	}
}

// cvdPyNum —— py 数字的常见写法（整数不带小数点）。
func cvdPyNum(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e16 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// ── 仓根定位 ────────────────────────────────────────────────────────────────

// cvdFindRoot —— py find_root：从 start 起最多上溯 8 层找 ROOT_ANCHOR。
func cvdFindRoot(start string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for i := 0; i < 8; i++ {
		if cvdIsFile(filepath.Join(d, cvdRootAnchor)) {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", false
}

// ── git 子进程 ─────────────────────────────────────────────────────────────

// cvdGit —— py git()：`git -c core.quotepath=false <args>` · cwd=root · 返 (rc, stdout.strip())。
// git 不在盘 / 不是仓 ⇒ rc≠0（调用侧判 rc=2 不给结论）。
func cvdGit(root string, args []string) (int, string) {
	argv := append([]string{"-c", "core.quotepath=false"}, args...)
	cmd := exec.Command("git", argv...)
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), strings.TrimSpace(out.String())
		}
		return exitBlocked, cvdPyOSErr(err) // ★ 子进程**起不来**（非 ExitError）⇒ 读不到 ⇒ 归 `exitBlocked`(8「有 BLOCKED · 读不到不许当健康」)；
		// 原先裸写 127（shell 惯例）不在命令面真源表里 ⇒ 门⑪ T3 报（GAP-20261002-08）。
	}
	return 0, strings.TrimSpace(out.String())
}

// ── 对拍核心（纯函数） ──────────────────────────────────────────────────────

// cvdFlatten —— py flatten：把顶层块摊成「路径 ⇒ 值」（authority / change_flow 是整体比，逐键给差异行）。
func cvdFlatten(doc any, prefix string, out map[string]any) {
	switch t := doc.(type) {
	case map[string]any:
		for k, v := range t {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			cvdFlatten(v, p, out)
		}
	case []any:
		for i, v := range t {
			cvdFlatten(v, fmt.Sprintf("%s[%d]", prefix, i), out)
		}
	default:
		out[prefix] = doc
	}
}

// cvdShow —— py show：值的人面写法（长串截断，免得一行吃掉整个终端）。
func cvdShow(v any) string {
	var s string
	if str, ok := v.(string); ok {
		s = str
	} else {
		s = cvdPyDump(v, "", 0)
	}
	r := []rune(s)
	if len(r) <= 120 {
		return s
	}
	return string(r[:117]) + "…"
}

func cvdAsMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func cvdEntries(doc map[string]any) []any {
	if l, ok := doc["entries"].([]any); ok && l != nil {
		return l
	}
	return []any{}
}

// cvdIdMap —— py 的 {str(id): entry}：**保留首见位置**（py dict 覆盖不改序）。
func cvdIdMap(es []any) ([]string, map[string]map[string]any) {
	order := []string{}
	m := map[string]map[string]any{}
	for _, e := range es {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		raw, ok := em["id"]
		if !ok {
			raw = ""
		}
		id := cvdPyStr(raw)
		if _, seen := m[id]; !seen {
			order = append(order, id)
		}
		m[id] = em
	}
	return order, m
}

// cvdCompareDocs —— py compare_docs：核心对拍（纯函数 · 夹具与真跑同一条路径）⇒ 差异行 list。
//
// 输出序由三处决定（逐条钉死，与 py 逐字同）：
//  1. 删除条目：遍历**旧版** entries 的插入序（首个 id 位置）；
//  2. 新增条目：遍历**新版** entries 的插入序；
//  3. 逐条字段差异：遍历**新版** entries 的插入序 × cvdEntryFields 的固定序
//     （name/truth/truth_shape/version_field/gate/change_class）；再按 cvdWholeTopKeys 的固定序
//     （authority/change_flow）出顶层块，块内路径按**字典序**（py sorted(set(...))）。
func cvdCompareDocs(old, new map[string]any) []string {
	lines := []string{}
	oe := cvdEntries(old)
	ne := cvdEntries(new)
	omo, om := cvdIdMap(oe)
	nmo, nm := cvdIdMap(ne)

	for _, i := range omo {
		if _, ok := nm[i]; !ok {
			lines = append(lines, fmt.Sprintf("%s 整条 旧→新（**删除**条目：该 id 不在新版里）", i))
		}
	}
	for _, i := range nmo {
		if _, ok := om[i]; !ok {
			lines = append(lines, fmt.Sprintf("%s 整条 旧→新（**新增**条目：该 id 不在旧版里）", i))
		}
	}
	for _, i := range nmo {
		if _, ok := om[i]; !ok {
			continue
		}
		for _, f := range cvdEntryFields {
			ov, nv := om[i][f], nm[i][f]
			if !reflect.DeepEqual(ov, nv) {
				lines = append(lines, fmt.Sprintf("%s %s 旧→新 ｜ 旧=%s 新=%s", i, f, cvdShow(ov), cvdShow(nv)))
			}
		}
	}
	for _, k := range cvdWholeTopKeys {
		fo, fn := map[string]any{}, map[string]any{}
		cvdFlatten(map[string]any{k: old[k]}, "", fo)
		cvdFlatten(map[string]any{k: new[k]}, "", fn)
		seen := map[string]bool{}
		for p := range fo {
			seen[p] = true
		}
		for p := range fn {
			seen[p] = true
		}
		paths := make([]string, 0, len(seen))
		for p := range seen {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			ov, ok1 := fo[p]
			if !ok1 {
				ov = "<无>"
			}
			nv, ok2 := fn[p]
			if !ok2 {
				nv = "<无>"
			}
			if !reflect.DeepEqual(ov, nv) {
				lines = append(lines, fmt.Sprintf("顶层 %s 旧→新 ｜ 旧=%s 新=%s", p, cvdShow(ov), cvdShow(nv)))
			}
		}
	}
	return lines
}

// cvdReadJSON —— py read_json：⇒ (doc, err)。读不到 / 解析不了 ⇒ err 非空（调用侧判 rc=2）。
func cvdReadJSON(path string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// cvdResolveBase —— 口径①：基线 = 上一次改动该件的提交及其父 ⇒ (base_rev, parent_rev, err)。
// 永不回落工作树 ✗。
func cvdResolveBase(root, rel string) (string, string, error) {
	rc, out := cvdGit(root, []string{"log", "-1", "--format=%H", "--", rel})
	if rc != 0 {
		return "", "", fmt.Errorf("git log 取不到该件的历史（rc=%d）：%s", rc, out)
	}
	base := cvdFirstLine(out)
	if base == "" {
		return "", "", errors.New("该件在 git 历史里**查无提交**（没提交过 / 浅克隆里没有它）")
	}
	rc, out = cvdGit(root, []string{"rev-parse", base + "^"})
	if rc != 0 {
		return base, "", fmt.Errorf("基线 rev 取不到**父提交**（该件只提交过一次 · 仓太浅）：%s", out)
	}
	return base, cvdFirstLine(out), nil
}

// cvdArgs —— check 的入参（py 的五个形参一一对应；Root 用 RootOK 区分「空串」与「取不到」）。
type cvdArgs struct {
	Root        string
	RootOK      bool
	Registry    string
	BaseRev     string
	HaveBaseRev bool
	OldFile     string
	NewFile     string
}

// cvdCheck —— py check：⇒ (rc, lines, info)。rc=2 = 不给结论。
func cvdCheck(a cvdArgs) (int, []string, cvdInfo) {
	t0 := time.Now()
	info := cvdInfo{}
	lines := []string{}

	done := func(rc int, ls []string) (int, []string, cvdInfo) {
		info.ElapsedMS = int(math.Round(time.Since(t0).Seconds() * 1000))
		return rc, ls, info
	}

	// ── 夹具档：两份 json 直接对拍（不碰 git；负控/自检走这条路） ─────────────
	if a.OldFile != "" && a.NewFile != "" {
		oldDoc, e1 := cvdReadJSON(a.OldFile)
		newDoc, e2 := cvdReadJSON(a.NewFile)
		if e1 != nil || e2 != nil {
			lines = append(lines, fmt.Sprintf(
				"⛔ 不给结论：夹具件读不到 / 解析失败（旧：%s · 新：%s）—— 两侧都得读得到才谈差异",
				cvdErrOr(e1, "ok"), cvdErrOr(e2, "ok")))
			info.Verdict = "blocked"
			return done(2, lines)
		}
		return cvdFinish(&info, lines, cvdAsMap(oldDoc), cvdAsMap(newDoc), a.NewFile, done, "")
	}

	if !a.RootOK {
		lines = append(lines, fmt.Sprintf("⛔ 不给结论：解析不到仓根（找 %s 失败）—— 设 ZERG_REPO=<仓根>", cvdRootAnchor))
		info.Verdict = "blocked"
		return done(2, lines)
	}
	if !cvdIsFile(a.Registry) {
		lines = append(lines, fmt.Sprintf("⛔ 不给结论：新版件读不到（%s）—— 表不在盘 ⇒ 无对象可对拍", a.Registry))
		info.Verdict = "blocked"
		return done(2, lines)
	}

	rc, head := cvdGit(a.Root, []string{"rev-parse", "HEAD"})
	if rc != 0 {
		lines = append(lines, fmt.Sprintf("⛔ 不给结论：取不到 `head_sha`（`git rev-parse HEAD` rc=%d）：%s", rc, head))
		info.Verdict = "blocked"
		return done(2, lines)
	}
	info.HeadSHA = cvdFirstLine(head)

	baseRev := a.BaseRev
	if !a.HaveBaseRev || baseRev == "" {
		got, _, err := cvdResolveBase(a.Root, a.Registry)
		if err != nil || got == "" {
			msg := "基线 rev 取到空值"
			if err != nil {
				msg = err.Error()
			}
			lines = append(lines, fmt.Sprintf("⛔ 不给结论：**无上一版可比** —— %s", msg))
			lines = append(lines, "   （退路位口径：基线 rev 取不到就明写「无上一版可比」，**不许拿工作树当基线糊过去** ✗）")
			info.Verdict = "blocked"
			return done(2, lines)
		}
		baseRev = got
	}

	rc, parent := cvdGit(a.Root, []string{"rev-parse", baseRev + "^"})
	if rc != 0 {
		lines = append(lines, fmt.Sprintf(
			"⛔ 不给结论：**无上一版可比** —— 基线 rev %s 取不到父提交（该件只提交过一次 · 仓太浅）", baseRev))
		info.Verdict = "blocked"
		return done(2, lines)
	}
	parent = cvdFirstLine(parent)
	info.BaseRev = baseRev

	rel, relErr := filepath.Rel(a.Root, a.Registry)
	if relErr != nil {
		rel = a.Registry
	}
	rc, raw := cvdGit(a.Root, []string{"show", parent + ":" + rel})
	if rc != 0 {
		lines = append(lines, fmt.Sprintf("⛔ 不给结论：旧版件在基线父提交上读不到（`git show %s:%s` rc=%d）",
			cvdTrunc(parent, 12), rel, rc))
		info.Verdict = "blocked"
		return done(2, lines)
	}
	var oldDoc any
	if err := json.Unmarshal([]byte(raw), &oldDoc); err != nil {
		lines = append(lines, fmt.Sprintf("⛔ 不给结论：旧版件解析失败（%s:%s）：%s", cvdTrunc(parent, 12), rel, err))
		info.Verdict = "blocked"
		return done(2, lines)
	}

	newDoc, err := cvdReadJSON(a.Registry)
	if err != nil {
		lines = append(lines, fmt.Sprintf("⛔ 不给结论：新版件解析失败（%s）：%s", a.Registry, cvdPyErrText(err)))
		info.Verdict = "blocked"
		return done(2, lines)
	}

	_rc, dirty := cvdGit(a.Root, []string{"status", "--porcelain", "--", rel})
	info.WorktreeDirty = _rc == 0 && dirty != ""
	return cvdFinish(&info, lines, cvdAsMap(oldDoc), cvdAsMap(newDoc), a.Registry, done,
		fmt.Sprintf("%s:%s", cvdTrunc(parent, 12), rel))
}

func cvdFinish(info *cvdInfo, lines []string, old, new map[string]any, newPath string,
	done func(int, []string) (int, []string, cvdInfo), oldLabel string) (int, []string, cvdInfo) {
	ol := oldLabel
	if ol == "" {
		ol = "（夹具旧件）"
	}
	lines = append(lines, fmt.Sprintf("旧版 = %s ｜ 新版 = %s", ol, newPath))
	hs := info.HeadSHA
	if hs == "" {
		hs = "（夹具档无）"
	}
	br := info.BaseRev
	if br == "" {
		br = "（夹具档无）"
	}
	lines = append(lines, fmt.Sprintf("两枚指纹：`head_sha` = %s ｜ 基线 rev = %s", hs, br))
	oe := cvdEntries(old)
	ne := cvdEntries(new)
	info.OldEntries = len(oe)
	info.NewEntries = len(ne)
	diffs := cvdCompareDocs(old, new)
	info.DiffLines = len(diffs)
	lines = append(lines, fmt.Sprintf("条数：旧 %d / 新 %d ｜ 差异行 %d", len(oe), len(ne), len(diffs)))
	if len(diffs) > 0 {
		lines = append(lines, diffs...)
		info.Verdict = "changed"
		return done(1, lines)
	}
	lines = append(lines, "无变更")
	info.Verdict = "unchanged"
	return done(0, lines)
}

// ── 自检（成对：正控 2 组 + 负控 11 组） ────────────────────────────────────

func cvdSelfTest(stdout io.Writer, root string, rootOK bool) int {
	bad := []string{}
	n := 0

	cvdCase := func(name string, wantRC, gotRC int, wantWord string, lines []string) {
		n++
		if gotRC != wantRC {
			bad = append(bad, fmt.Sprintf("✗ %s：want rc=%d got rc=%d", name, wantRC, gotRC))
		} else if wantWord != "" && !cvdAnyContains(lines, wantWord) {
			bad = append(bad, fmt.Sprintf("✗ %s：rc 对了但判词里没有 '%s'", name, wantWord))
		}
	}

	real := cvdRegistryRel
	if rootOK {
		real = filepath.Join(root, cvdRegistryRel)
	}
	tmp, err := os.MkdirTemp("", "zerg-cvd-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "✗ 自检前置：临时目录建不了（%s）\n", cvdPyErrText(err))
		return 1
	}
	defer os.RemoveAll(tmp)

	// ① 正控甲：同件对同件（夹具）⇒ rc=0 且逐字印「无变更」
	rc, lines, _ := cvdCheck(cvdArgs{Registry: real, OldFile: real, NewFile: real})
	cvdCase("① 正控：同件对同件 ⇒ rc=0 且印「无变更」", 0, rc, "无变更", lines)

	// ① 正控乙：真目标（旧版取基线父 · 新版取盘上件）⇒ 只许 0 或 1（正经跑），不许 2
	rcReal, linesReal, infoReal := cvdCheck(cvdArgs{Root: root, RootOK: rootOK, Registry: real})
	n++
	if rcReal != 0 && rcReal != 1 {
		bad = append(bad, fmt.Sprintf("✗ ② 正控：真目标对拍给了 rc=%d（不给结论 ⇒ 基线面或表读不到）", rcReal))
	} else if infoReal.BaseRev == "" || infoReal.HeadSHA == "" {
		bad = append(bad, "✗ ② 正控：两枚指纹没印全（head_sha/base_rev）")
	}
	_ = linesReal

	good, gerr := cvdReadJSON(real)
	if gerr != nil || good == nil {
		bad = append(bad, fmt.Sprintf("✗ 自检前置：真表读不到（%s）", real))
		return 1
	}

	oldText, err := os.ReadFile(real)
	if err != nil {
		bad = append(bad, fmt.Sprintf("✗ 自检前置：真表读不到（%s）", real))
		return 1
	}
	oldp := filepath.Join(tmp, "old.json")
	if err := os.WriteFile(oldp, oldText, 0o644); err != nil {
		bad = append(bad, fmt.Sprintf("✗ 自检前置：旧件写不了（%s）", cvdPyErrText(err)))
		return 1
	}

	pair := func(name string, mutate func(map[string]any)) (int, []string) {
		var doc any
		if err := json.Unmarshal(oldText, &doc); err != nil {
			return 2, []string{}
		}
		mutate(cvdAsMap(doc))
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, []byte(cvdPyDump(doc, " ", 0)), 0o644); err != nil {
			return 2, []string{}
		}
		rc, lines, _ := cvdCheck(cvdArgs{Registry: real, OldFile: oldp, NewFile: p})
		return rc, lines
	}

	// ② 负控：改一条字段（gate 追加一个脚本）⇒ rc=1 且差异行点到该 id + 该字段
	rc, lines = pair("field.json", func(d map[string]any) {
		e := cvdEntries(d)[0].(map[string]any)
		e["gate"] = cvdPyStr(e["gate"]) + " · scripts/gates/check-nonexistent-xyz.py"
	})
	cvdCase("③ 负控：改一条字段 ⇒ rc=1（点到 <id> <字段>）", 1, rc, "gate 旧→新", lines)

	// ③ 负控：新增一条 ⇒ rc=1 且印「新增」
	rc, lines = pair("add.json", func(d map[string]any) {
		es := cvdEntries(d)
		es = append(es, map[string]any{"id": "S-z", "name": "夹具新条",
			"truth": "core/internal/contract/registry.json", "truth_shape": "x", "version_field": "schema",
			"gate": "scripts/gates/check-contract-registry.py", "change_class": "B", "change_note": "x"})
		d["entries"] = es
	})
	cvdCase("④ 负控：新增一条 ⇒ rc=1 且印「新增」", 1, rc, "S-z 整条 旧→新", lines)

	// ④ 负控：删一条 ⇒ rc=1 且印「删除」
	rc, lines = pair("del.json", func(d map[string]any) {
		es := cvdEntries(d)
		d["entries"] = es[:len(es)-1]
	})
	cvdCase("⑤ 负控：删一条 ⇒ rc=1 且印「删除」", 1, rc, "删除", lines)

	// ⑤ 负控：改 authority 一个键（顶层整体比）⇒ rc=1 且点到该键路径
	rc, lines = pair("auth.json", func(d map[string]any) {
		cvdAsMap(d["authority"])["rule"] = "夹具改过的规则"
	})
	cvdCase("⑥ 负控：改 authority 一个键 ⇒ rc=1", 1, rc, "顶层 authority.rule 旧→新", lines)

	// ⑥ 负控：改 change_flow 一处 ⇒ rc=1
	rc, lines = pair("flow.json", func(d map[string]any) {
		cvdAsMap(d["change_flow"])["author_not_approver"] = false
	})
	cvdCase("⑦ 负控：改 change_flow 一处 ⇒ rc=1", 1, rc, "顶层 change_flow.author_not_approver 旧→新", lines)

	// ⑦ 负控：旧件读不到 ⇒ rc=2（不给结论）
	rc, lines, _ = cvdCheck(cvdArgs{Registry: real, OldFile: filepath.Join(tmp, "根本没这份旧件.json"), NewFile: oldp})
	cvdCase("⑧ 负控：旧件读不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)

	// ⑧ 负控：新版件读不到 ⇒ rc=2（不给结论）
	rc, lines, _ = cvdCheck(cvdArgs{Root: root, RootOK: rootOK, Registry: filepath.Join(tmp, "根本没这份表.json")})
	cvdCase("⑨ 负控：新版件读不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)

	// ⑨ 负控：基线 rev 取不到 ⇒ rc=2 且逐字印「无上一版可比」（不许拿工作树当基线）
	rc, lines, _ = cvdCheck(cvdArgs{Root: root, RootOK: rootOK, Registry: real,
		BaseRev: "0000000000000000000000000000000000000000", HaveBaseRev: true})
	cvdCase("⑩ 负控：基线 rev 取不到 ⇒ rc=2 且印「无上一版可比」", 2, rc, "无上一版可比", lines)

	// ⑩ 负控：根解析不到 ⇒ rc=2（不给结论）
	rc, lines, _ = cvdCheck(cvdArgs{Registry: real})
	cvdCase("⑪ 负控：根解析不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)

	// ⑪ 负控：拿一个**没有该件**的临时仓当目标 ⇒ rc=2 且印「无上一版可比」（禁工作树糊过去）
	bare := filepath.Join(tmp, "bare")
	if err := os.MkdirAll(bare, 0o755); err == nil {
		for _, cmd := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
			cvdGit(bare, cmd)
		}
		_ = os.WriteFile(filepath.Join(bare, "x.txt"), []byte("x"), 0o644)
		cvdGit(bare, []string{"add", "x.txt"})
		cvdGit(bare, []string{"commit", "-qm", "init"})
	}
	rc, lines, _ = cvdCheck(cvdArgs{Root: bare, RootOK: true, Registry: filepath.Join(bare, cvdRegistryRel)})
	cvdCase("⑫ 负控：目标仓里查无该件 ⇒ rc=2（新版件读不到）", 2, rc, "不给结论", lines)

	// ⑬ 正控：真跑墙钟（原型 0.062s 档 · 只印数，不设阈值当门）
	fmt.Fprintf(stdout, "   · 真目标墙钟：%d ms（差异行 %d · 旧 %d / 新 %d · verdict=%s）\n",
		infoReal.ElapsedMS, infoReal.DiffLines, infoReal.OldEntries, infoReal.NewEntries, infoReal.Verdict)
	n++

	if len(bad) > 0 {
		fmt.Fprintf(stdout, "✗ 自检失败（%d 组断言未过 / 共 %d 组）：\n", len(bad), n)
		for _, b := range bad {
			fmt.Fprintln(stdout, "   "+b)
		}
		return 1
	}
	fmt.Fprintf(stdout, "✓ 自检全过（%d 组：正控 2 + 负控 11 · 成对）\n", n)
	return 0
}

func cvdAnyContains(lines []string, w string) bool {
	for _, l := range lines {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

// ── 人面 / 机器面输出 ──────────────────────────────────────────────────────

// cvdPyStrLit —— py json 串字面（ensure_ascii=False 档）。
func cvdPyStrLit(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cvdPyDump —— py json.dumps 的形（ind 为空 = 字面量档，用 `, ` / `: ` 分隔；
// ind 非空 = indent 档，Python 的换行 + 缩进形）。键序：cvdObj 保序 · map 走字典序。
func cvdPyDump(v any, ind string, lvl int) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return cvdPyStrLit(t)
	case int:
		return strconv.Itoa(t)
	case float64:
		return cvdPyNum(t)
	case []string:
		parts := make([]string, 0, len(t))
		for _, s := range t {
			parts = append(parts, cvdPyDump(s, ind, lvl+1))
		}
		return cvdWrap(parts, ind, lvl, "[", "]")
	case []any:
		parts := make([]string, 0, len(t))
		for _, s := range t {
			parts = append(parts, cvdPyDump(s, ind, lvl+1))
		}
		return cvdWrap(parts, ind, lvl, "[", "]")
	case cvdObj:
		parts := make([]string, 0, len(t))
		for _, kv := range t {
			parts = append(parts, cvdPyStrLit(kv.K)+": "+cvdPyDump(kv.V, ind, lvl+1))
		}
		return cvdWrap(parts, ind, lvl, "{", "}")
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, cvdPyStrLit(k)+": "+cvdPyDump(t[k], ind, lvl+1))
		}
		return cvdWrap(parts, ind, lvl, "{", "}")
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return "null"
		}
		return string(b)
	}
}

func cvdWrap(parts []string, ind string, lvl int, open, close string) string {
	if len(parts) == 0 {
		return open + close
	}
	if ind == "" {
		return open + strings.Join(parts, ", ") + close
	}
	pad := strings.Repeat(ind, lvl+1)
	endpad := strings.Repeat(ind, lvl)
	return open + "\n" + pad + strings.Join(parts, ",\n"+pad) + "\n" + endpad + close
}

func cvdJSONValue(info cvdInfo, f string) (any, bool) {
	switch f {
	case "head_sha":
		return info.HeadSHA, true
	case "base_rev":
		return info.BaseRev, true
	case "old_entries":
		return info.OldEntries, true
	case "new_entries":
		return info.NewEntries, true
	case "diff_lines":
		return info.DiffLines, true
	case "verdict":
		return info.Verdict, true
	case "worktree_dirty":
		return info.WorktreeDirty, true
	case "elapsed_ms":
		return info.ElapsedMS, true
	}
	return nil, false
}

// ── 入口 ────────────────────────────────────────────────────────────────────

// cvdMain 是本件的入口（已挂 `gateCheckGoEntry["check-contract-version-diff"]` ⇒ 走 inproc；
// ✗ 不进 `main.go` 命令树 —— M2 不接线主体）。
func cvdMain(stdout, stderr io.Writer, argv []string) int {
	fs := flag.NewFlagSet("check-contract-version-diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var rootF, regF, baseF, oldF, newF, jsonF, outF string
	var selfF bool
	fs.StringVar(&rootF, "root", "", "仓库根（默认自动定位：找 "+cvdRootAnchor+"）")
	fs.StringVar(&regF, "registry", "", "新版表路径")
	fs.StringVar(&baseF, "base-rev", "", "基线 rev")
	fs.StringVar(&oldF, "old-file", "", "夹具：旧件路径")
	fs.StringVar(&newF, "new-file", "", "夹具：新件路径")
	fs.StringVar(&jsonF, "json", "", "机器面字段（逗号分隔）")
	fs.StringVar(&outF, "out", "", "另存结果件")
	fs.BoolVar(&selfF, "self-test", false, "只跑自检")
	fs.BoolVar(&selfF, "selftest", false, "只跑自检")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	haveJSON := set["json"]
	haveBase := set["base-rev"]
	haveSelf := set["self-test"] || set["selftest"]

	if haveJSON {
		want := []string{}
		for _, f := range strings.Split(jsonF, ",") {
			if f != "" {
				want = append(want, f)
			}
		}
		unknown := []string{}
		for _, f := range want {
			if !cvdHasStr(cvdJSONFields, f) {
				unknown = append(unknown, f)
			}
		}
		if len(want) == 0 || len(unknown) > 0 {
			u := "（空）"
			if len(unknown) > 0 {
				u = strings.Join(unknown, ",")
			}
			fmt.Fprintf(stderr, "✗ `--json` 字段表外：%s（可选字段：%s）\n", u, strings.Join(cvdJSONFields, ","))
			return 2
		}
	}
	if (oldF != "") != (newF != "") {
		fmt.Fprintln(stderr, "✗ `--old-file` 与 `--new-file` 必须成对（夹具档两侧都要给）")
		return 2
	}

	src := rootF
	if src == "" {
		src = os.Getenv("ZERG_REPO")
	}
	if src == "" {
		src = cvdExeDir()
	}
	root, rootOK := cvdFindRoot(src)

	if haveSelf {
		fmt.Fprint(stdout, "门 · 契约「版本间对拍」（旧版 ↔ 新版 registry.json · R5/G-27）· 自检档（真表只读 + 合成夹具 · 不碰真目标）\n")
		return cvdSelfTest(stdout, root, rootOK)
	}

	reg := regF
	if reg == "" {
		if rootOK {
			reg = filepath.Join(root, cvdRegistryRel)
		} else {
			reg = cvdRegistryRel
		}
	}
	rc, lines, info := cvdCheck(cvdArgs{Root: root, RootOK: rootOK, Registry: reg,
		BaseRev: baseF, HaveBaseRev: haveBase, OldFile: oldF, NewFile: newF})

	if haveJSON {
		obj := map[string]any{}
		for _, f := range strings.Split(jsonF, ",") {
			if f == "" {
				continue
			}
			if v, ok := cvdJSONValue(info, f); ok {
				obj[f] = v
			}
		}
		fmt.Fprintln(stdout, cvdPyDump(obj, "", 0))
		return rc
	}

	rootDisp := "None"
	if rootOK {
		rootDisp = root
	}
	fmt.Fprintf(stdout, "门 · 契约「版本间对拍」（旧版 ↔ 新版 registry.json · R5/G-27）· 仓根 %s\n", rootDisp)
	nvp := newF
	if nvp == "" {
		nvp = reg
	}
	fmt.Fprintf(stdout, "  新版件：%s\n", nvp)
	if oldF != "" {
		fmt.Fprintf(stdout, "  旧件（夹具）：%s\n", oldF)
	}
	for _, l := range lines {
		fmt.Fprintln(stdout, "  "+l)
	}
	word := map[string]string{"unchanged": "无变更", "changed": "有差异（报告档 · 不是判红）",
		"blocked": "不给结论"}[info.Verdict]
	fmt.Fprintf(stdout, "⇒ %s（rc=%d · 墙钟 %d ms）\n", word, rc, info.ElapsedMS)

	if outF != "" {
		regSHA := ""
		if cvdIsFile(reg) {
			if b, err := os.ReadFile(reg); err == nil {
				sum := sha256.Sum256(b)
				regSHA = hex.EncodeToString(sum[:])
			}
		}
		obj := cvdObj{
			{K: "head_sha", V: info.HeadSHA},
			{K: "base_rev", V: info.BaseRev},
			{K: "old_entries", V: info.OldEntries},
			{K: "new_entries", V: info.NewEntries},
			{K: "diff_lines", V: info.DiffLines},
			{K: "verdict", V: info.Verdict},
			{K: "registry_sha256", V: regSHA},
			{K: "lines", V: lines},
		}
		if err := os.WriteFile(outF, []byte(cvdPyDump(obj, " ", 0)), 0o644); err != nil {
			fmt.Fprintf(stderr, "  ⛔ 结果件写不了（%s）—— 对拍结论照上（退码不变）\n", cvdPyErrText(err))
		} else {
			fmt.Fprintf(stdout, "  结果件已写：%s\n", outF)
		}
	}
	return rc
}

// cvdExeDir —— 无 --root / ZERG_REPO 时的定位起点（py 用脚本所在目录；Go 用可执行文件所在目录）。
func cvdExeDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
