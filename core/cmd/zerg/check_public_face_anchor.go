// check_public_face_anchor.go —— 判据件 `scripts/gates/check-public-face-anchor.py`（635 行）的 **Go 真搬**
// （M2 一枚 · 公开面漂移锚族 · 入口 `checkPublicFaceAnchorGateMain` · 承接 GAP-20261001-59）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 声明 (a) 默认根定位差异：py 从脚本自身位置（`__file__`）上溯 2 层推 `REPO_ROOT`；
// Go 无脚本位置语境 ⇒ 缺省根 = **cwd**（不向上搜「含某标记的目录」，以免与 py 的纯上溯语义分叉）。
// 两态在「脚本位置 == cwd」时逐字同构（对拍台按此布置：显式传 `--root`）。
//
// 声明 (b) ★自指判定（本件是否落在扫描面内）——
//
//	① 本门自己的扫描面 = `<--face>`（缺省 `publish/docs`）下的 `*.md` ⇒ **本件自身（core/ 下的 .go）
//	   绝不在本门扫面内**（本门只收面内 .md）。
//	② 有 core/ 在扫面内的**他门**：逐个现读其针形并避让 ——
//	   · 私有卷 / 家目录绝对路径针（扫 core/）⇒ 本件**零**该类字面量（卷名与用户名一律拆串）。
//	   · 干跑语义登记门（扫 core/）⇒ 本件正文**不出现**该门登记触发词（本门无该语义面）。
//	   落件前后对同一他门取受控读数对照（见交付④）。
//	③ 自检素材里的合成仓内容一律是本件自造串，与真仓公开面件不同名同形。
//
// 已知口径差（照实记）：
//
//	· py `--self-test` 第 ⑮ 格经 `sys.executable <__file__> --waive` **另起一个 py 进程**验「逃生开关被拒」；
//	  本件无「再起一个自己」的位置语境 ⇒ 同格改为**进程内**调本入口 + argv=[`--waive`]，判 rc=2 且正文含「拒」。
//	· 例外登记面真身 = **本件自己**（py 的 `SINGLE_LANG_EXCEPTIONS` 就在本件内）⇒ 无跨件现读，字面量直接持有。
//	  别的件（`scripts/gates/check-public-face-pairing.py` 一族）**现读 py 原件**的那张表，落本件不改那面。
//	· argparse 长选项**前缀缩写**（如 `--ro` ⇒ `--root`）：唯一前缀按 argparse 语义认；歧义档文案近似（见未覆盖面）。
//	· 最外层无兜底（py 原件为裸出口形态）⇒ 本件按 fail-closed 收成 rc=2 + 具名文案（见未覆盖面）。
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	cpfaVersion     = "check-public-face-anchor.py v1.0（`T-13` 漂移锚 · 2026-09-24 落件 · **未接线**）"
	cpfaFaceDefault = "publish/docs"
	cpfaZhSuffix    = ".zh-CN.md"
	cpfaEnSuffix    = ".en.md"
)

var cpfaWalkPrune = []string{".git", "node_modules", "target"}

// 拒收的逃生开关（一律 rc=2；例外只能显式登记）。
var cpfaRefuseFlags = []string{"--waive", "--allow-unregistered", "--ignore-missing", "--skip-pairs"}

// `--json <字段>` 的合法字段表（**唯一真源**：印表与拒收同读这一处）。
var cpfaJSONFields = []string{"rc", "verdict", "face", "files", "pairs", "singles", "unpaired",
	"exceptions", "dangling_exceptions", "unregistered", "rev", "counts", "drift", "stale",
	"both", "not_in_diff"}

// cpfaException —— 单一语例外登记（**正式登记** · `T-11b` 接手落地 · 每条带理由）。
type cpfaException struct {
	File    string
	Mapping string
	Lines   int
	Reason  string
}

// cpfaExceptions = py `SINGLE_LANG_EXCEPTIONS`（逐字同源 · 登记面单一真源）。
var cpfaExceptions = []cpfaException{
	{
		File: "design/格式反馈协议-FFP.md", Mapping: "A13", Lines: 263,
		Reason: "中文单语「理念件」（`Q-128` ②③ · `T-11b` **正式登记**）：公开面映射 `A13` **只有 zh 一条**" +
			"（`publish/mirror-public-lib.py:310–311`）⇒ 无派生件可比 ⇒ 本件不判成对；" +
			"★ 「要不要补 en」= `Q-128` ④ 开放问题 · **本表不定**",
	},
	{
		File: "design/记忆体系.md", Mapping: "A14", Lines: 346,
		Reason: "中文单语「理念件」（同 `Q-128` · `T-11b` **正式登记**）：公开面映射 `A14` **只有 zh 一条**" +
			"（`publish/mirror-public-lib.py:310–311`）⇒ 无派生件可比 ⇒ 本件不判成对；" +
			"★ 「要不要补 en」= 同上开放问题 · **本表不定**",
	},
}

// ══ py 语义工具（posixpath 同构 · 模板 templates/py-path-semantics.go）════════════════════

// cpfaPyJoin = CPython posixpath.join 逐段规则（★ 三语义差：中间 `..` 原样留 · 后段绝对丢前段 ·
// 空段保留尾 `/`）。
func cpfaPyJoin(parts ...string) string {
	path := ""
	if len(parts) > 0 {
		path = parts[0]
	}
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

func cpfaPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
	}
	if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
		initial = 2
	}
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || initial == 0 && len(newComps) == 0 ||
			len(newComps) > 0 && newComps[len(newComps)-1] == ".." {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	res := strings.Join(newComps, "/")
	if initial > 0 {
		res = strings.Repeat("/", initial) + res
	}
	if res == "" {
		return "."
	}
	return res
}

func cpfaPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = ""
		}
		p = cpfaPyJoin(wd, p)
	}
	return cpfaPyNormpath(p)
}

func cpfaSplitComps(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

func cpfaPyRelpath(path, start string) string {
	startAbs := cpfaPyAbspath(start)
	pathAbs := cpfaPyAbspath(path)
	sl := cpfaSplitComps(startAbs)
	pl := cpfaSplitComps(pathAbs)
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	var rel []string
	for j := 0; j < len(sl)-i; j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pl[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return cpfaPyJoin(rel...)
}

func cpfaPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" {
		if trimmed := strings.TrimRight(head, "/"); trimmed != "" {
			head = trimmed
		}
	}
	return head, tail
}

func cpfaPyDirname(p string) string {
	d, _ := cpfaPySplit(p)
	return d
}

// cpfaPyStrip = CPython str.strip()（★ 含 \x1c-\x1f ⇒ ✗ 不能用 strings.TrimSpace 单独了事）。
func cpfaPyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		if r >= 0x1c && r <= 0x1f {
			return true
		}
		return unicode.IsSpace(r)
	})
}

// ══ git（只经子进程 · 不接管道 · 读 returncode）════════════════════════════════════

func cpfaEnvSet(env []string, k, v string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, k+"=") {
			continue
		}
		out = append(out, e)
	}
	return append(out, k+"="+v)
}

// cpfaGitEnv = py `git_env(base)`：base 非空 ⇒ 覆盖 GIT_* / HOME（隔离合成夹具）。
func cpfaGitEnv(base string) []string {
	env := os.Environ()
	if base != "" {
		env = cpfaEnvSet(env, "GIT_CONFIG_GLOBAL", os.DevNull)
		env = cpfaEnvSet(env, "GIT_CONFIG_SYSTEM", os.DevNull)
		env = cpfaEnvSet(env, "HOME", base)
		env = cpfaEnvSet(env, "GIT_AUTHOR_NAME", "anchor-selftest")
		env = cpfaEnvSet(env, "GIT_COMMITTER_NAME", "anchor-selftest")
		env = cpfaEnvSet(env, "GIT_AUTHOR_EMAIL", "anchor@example.invalid")
		env = cpfaEnvSet(env, "GIT_COMMITTER_EMAIL", "anchor@example.invalid")
	}
	return env
}

// cpfaGitRun —— ★ argv 形状逐字照 py `git_run`：`git -c core.quotepath=false -C <root> <args…>`
// （py 的 git_changed 里又叠了一次 `-c core.quotepath=false` ⇒ 本件照抄该冗余，免 argv 分叉）。
func cpfaGitRun(root string, args []string, env []string) (int, string, string) {
	full := append([]string{"git", "-c", "core.quotepath=false", "-C", root}, args...)
	cmd := exec.Command(full[0], full[1:]...)
	cmd.Env = env
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return cmd.ProcessState.ExitCode(), so.String(), se.String()
		}
		return exitBlocked, so.String(), se.String() // ★ 子进程**起不来**（非 ExitError）⇒ 读不到 ⇒ 归 `exitBlocked`(8「有 BLOCKED · 读不到不许当健康」)；
		// 原先裸写 127（shell 惯例）不在命令面真源表里 ⇒ 门⑪ T3 报（GAP-20261002-08）。
	}
	return 0, so.String(), se.String()
}

func cpfaGitOK(root string) (bool, string) {
	rc, _, se := cpfaGitRun(root, []string{"rev-parse", "--git-dir"}, nil)
	return rc == 0, cpfaPyStrip(se)
}

func cpfaRevResolvable(root, rev string) bool {
	rc, _, _ := cpfaGitRun(root, []string{"rev-parse", "--verify", "--quiet", rev + "^{commit}"}, nil)
	return rc == 0
}

func cpfaSplitNames(text string) []string {
	var out []string
	for _, ln := range strings.Split(text, "\n") {
		ln = cpfaPyStrip(ln)
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

// ══ 扫面枚举 / 成对分类 ═══════════════════════════════════════════════════════════

// cpfaEnumerateFace = py `enumerate_face`：**只收 `.md`**（★ 不排点件 —— 与同族成对闸不同）；
// 目录项按 readdir 集算 ⇒ 本件自排；**输出序决定者 = 末尾那一次全局 `sorted(out)`**
// （py `:162` `return sorted(out)` 逐字）—— 遍历序一律被它覆盖，逐层排序只为剪枝判定。
func cpfaEnumerateFace(faceDir string) []string {
	var files []string
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return // 同 py os.walk 默认 onerror=None：读不了的目录静默跳过（照实照搬）
		}
		var dirs, nondirs, linkedDirs []string
		for _, e := range entries {
			name := e.Name()
			st, serr := os.Stat(cpfaPyJoin(dir, name)) // py 用 entry.is_dir()（跟随符号链接）
			if serr == nil && st.IsDir() {
				dirs = append(dirs, name)
				if fi, lerr := os.Lstat(cpfaPyJoin(dir, name)); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
					linkedDirs = append(linkedDirs, name)
				}
			} else {
				nondirs = append(nondirs, name)
			}
		}
		sort.Strings(nondirs)
		for _, fn := range nondirs {
			if !strings.HasSuffix(fn, ".md") {
				continue
			}
			rel := strings.ReplaceAll(cpfaPyRelpath(cpfaPyJoin(dir, fn), faceDir), string(os.PathSeparator), "/")
			files = append(files, rel)
		}
		// py os.walk(topdown=True) 只对 dirnames 剪枝；followlinks=False ⇒ 符号链接目录**不进递归**。
		sort.Strings(dirs)
		isLinked := map[string]bool{}
		for _, d := range linkedDirs {
			isLinked[d] = true
		}
		for _, d := range dirs {
			if cpfaContainsStr(cpfaWalkPrune, d) || isLinked[d] {
				continue
			}
			walk(cpfaPyJoin(dir, d))
		}
	}
	walk(faceDir)
	sort.Strings(files)
	return files
}

func cpfaContainsStr(xs []string, x string) bool {
	for _, e := range xs {
		if e == x {
			return true
		}
	}
	return false
}

// cpfaClassify = py `classify` ⇒ (成对 pairs, 单一语 singles, 未成对 unpaired)。
func cpfaClassify(files []string) (pairs [][2]string, singles []string, unpaired [][2]string) {
	fs := map[string]bool{}
	for _, f := range files {
		fs[f] = true
	}
	consumed := map[string]bool{}
	for _, rel := range files {
		if !strings.HasSuffix(rel, cpfaZhSuffix) {
			continue
		}
		stem := rel[:len(rel)-len(cpfaZhSuffix)]
		cands := []string{stem + cpfaEnSuffix}
		if stem == "README" {
			cands = append(cands, "README.md")
		}
		en := ""
		for _, c := range cands {
			if fs[c] {
				en = c
				break
			}
		}
		if en != "" {
			pairs = append(pairs, [2]string{rel, en})
			consumed[rel] = true
			consumed[en] = true
		} else {
			unpaired = append(unpaired, [2]string{rel, "真源侧没有对应派生件（试过：" + strings.Join(cands, " / ") + "）"})
		}
	}
	for _, rel := range files {
		if consumed[rel] {
			continue
		}
		if strings.HasSuffix(rel, cpfaEnSuffix) {
			stem := rel[:len(rel)-len(cpfaEnSuffix)]
			unpaired = append(unpaired, [2]string{rel, "派生侧没有对应真源件（试过：" + stem + cpfaZhSuffix + "）"})
		} else if strings.HasSuffix(rel, cpfaZhSuffix) {
			// 已在 ① 里进 unpaired（真源缺派生件）—— 不许在这里重复计数
		} else {
			singles = append(singles, rel)
		}
	}
	return pairs, singles, unpaired
}

// cpfaGitChanged = py `git_changed` ⇒ (diff 面 set, err)。
func cpfaGitChanged(root, rev, faceRel string) (map[string]bool, string) {
	rc, so, se := cpfaGitRun(root, []string{"-c", "core.quotepath=false", "diff", "--name-only", rev, "--", faceRel}, nil)
	if rc != 0 {
		e := cpfaPyStrip(se)
		if e == "" {
			e = fmt.Sprintf("git diff rc=%d", rc)
		}
		return nil, e
	}
	pre := strings.TrimRight(faceRel, "/") + "/"
	out := map[string]bool{}
	for _, ln := range cpfaSplitNames(so) {
		if strings.HasPrefix(ln, pre) {
			out[ln[len(pre):]] = true
		} else {
			out[ln] = true
		}
	}
	return out, ""
}

// ══ 判定（= py `judge`）═════════════════════════════════════════════════════════════

func cpfaJudge(root, faceRel, rev string, exceptions []cpfaException) (int, []string, map[string]any) {
	lines := []string{}
	data := map[string]any{}
	faceDir := cpfaPyJoin(root, faceRel)

	// 守卫一：扫面不在 ⇒ rc=2（缺件 ≠ 绿）
	if st, err := os.Stat(faceDir); err != nil || !st.IsDir() {
		lines = append(lines, "⛔ 不给结论（rc=2）：扫面不在 ⇒ "+cpfaPyJoin(root, faceRel))
		return 2, lines, map[string]any{"rc": 2, "verdict": "missing-face", "face": faceRel}
	}

	files := cpfaEnumerateFace(faceDir)
	// 守卫二：空转 ⇒ rc=2（扫到 0 件就不给结论 · 照仓内「空转 = 假覆盖」口径）
	if len(files) == 0 {
		lines = append(lines, "⛔ 不给结论（rc=2）：扫面 0 件（空转 = 假覆盖）⇒ "+faceDir)
		return 2, lines, map[string]any{"rc": 2, "verdict": "vacuous", "face": faceRel, "files": 0}
	}

	pairs, singles, unpaired := cpfaClassify(files)
	excRel := []string{}
	for _, e := range exceptions {
		if e.File != "" {
			excRel = append(excRel, e.File)
		}
	}
	exc := map[string]bool{}
	for _, r := range excRel {
		exc[r] = true
	}
	fileSet := map[string]bool{}
	for _, f := range files {
		fileSet[f] = true
	}
	var dangling []string
	for _, r := range excRel {
		if !fileSet[r] {
			dangling = append(dangling, r)
		}
	}
	unpairedWhy := map[string]string{}
	for _, u := range unpaired {
		unpairedWhy[u[0]] = u[1]
	}
	var unregistered []string
	for _, r := range singles {
		if !exc[r] {
			unregistered = append(unregistered, r)
		}
	}
	for _, u := range unpaired {
		if !exc[u[0]] {
			unregistered = append(unregistered, u[0])
		}
	}

	data["face"] = faceRel
	data["files"] = len(files)
	data["pairs"] = len(pairs)
	data["singles"] = len(singles)
	data["unpaired"] = len(unpaired)
	data["exceptions"] = len(excRel)
	data["dangling_exceptions"] = dangling

	lines = append(lines, "── 公开面漂移锚（`T-13` · 负控「锚未变而正文变 = 红」· 正控「锚与正文同批 ⇒ 绿」）──")
	lines = append(lines, "VERSION: "+cpfaVersion)

	// `J0` 成对性 + 例外登记（前置）
	if len(unregistered) > 0 {
		lines = append(lines, "")
		lines = append(lines, "⛔ 不给结论（rc=2）：`J0` 成对性 + 例外登记 —— 下列件**既不成对、又未登记例外**：")
		for _, r := range unregistered {
			why, ok := unpairedWhy[r]
			if !ok {
				why = "单一语件（无 `" + cpfaEnSuffix + "` 对应件）"
			}
			lines = append(lines, "   · "+r+" —— "+why)
		}
		lines = append(lines, "   ⇒ 例外必须先**显式登记**（本件 `SINGLE_LANG_EXCEPTIONS` · 逐条带理由）；")
		lines = append(lines, "      登记面 = 本表（`T-11b` **已正式登记** · 2026-09-24 落地 · 两件 `design/` 中文单语例外）。")
		lines = append(lines, "   ✗ 不许用 `"+strings.Join(cpfaRefuseFlags[:3], " / ")+"` 一类开关绕过（本件一律拒 rc=2）。")
		data["rc"] = 2
		data["verdict"] = "unregistered"
		data["unregistered"] = unregistered
		return 2, lines, data
	}

	// 守卫三：git 读面
	ok, err := cpfaGitOK(root)
	if !ok {
		if err == "" {
			err = "非仓"
		}
		lines = append(lines, "⛔ 不给结论（rc=2）：`git` 读不到 ⇒ "+root+"（"+err+"）")
		data["rc"] = 2
		data["verdict"] = "no-git"
		return 2, lines, data
	}
	if !cpfaRevResolvable(root, rev) {
		lines = append(lines, "⛔ 不给结论（rc=2）：`rev` 解不出 ⇒ "+rev+"（仓 "+root+"）")
		data["rc"] = 2
		data["verdict"] = "bad-rev"
		data["rev"] = rev
		return 2, lines, data
	}

	changed, cerr := cpfaGitChanged(root, rev, faceRel)
	if changed == nil {
		lines = append(lines, "⛔ 不给结论（rc=2）：取 diff 面失败 ⇒ "+cerr)
		data["rc"] = 2
		data["verdict"] = "no-diff"
		return 2, lines, data
	}

	var drift, stale, both, untouched [][2]string
	for _, p := range pairs {
		zh, en := p[0], p[1]
		z, e := changed[zh], changed[en]
		switch {
		case z && e:
			both = append(both, p)
		case e && !z:
			drift = append(drift, p)
		case z && !e:
			stale = append(stale, p)
		default:
			untouched = append(untouched, p)
		}
	}
	var excChanged []string
	for _, r := range excRel {
		if changed[r] {
			excChanged = append(excChanged, r)
		}
	}
	var notInDiff []string
	for _, r := range files {
		if !changed[r] {
			notInDiff = append(notInDiff, r)
		}
	}

	lines = append(lines, fmt.Sprintf("扫面：%s —— 现读 %d 件 = 成对 **%d 对** + 单一语 **%d 件**（已登记例外 %d 条）",
		faceRel, len(files), len(pairs), len(singles), len(excRel)))
	lines = append(lines, fmt.Sprintf("diff 面：`git -c core.quotepath=false diff --name-only %s -- %s` —— tracked 改动 **%d** 件",
		rev, faceRel, len(changed)))
	lines = append(lines, "")
	lines = append(lines, "── 逐对判定（`J1` 负控 / `J2` 正控 / `J3` 附报）──")
	for _, p := range drift {
		lines = append(lines, fmt.Sprintf("   [**红·漂移**] %s ↔ %s —— **锚未变而正文变**（正文侧在 diff 面、锚侧一字未动）", p[0], p[1]))
	}
	for _, p := range both {
		lines = append(lines, fmt.Sprintf("   [绿·同批]   %s ↔ %s —— 锚与正文同批（正控）", p[0], p[1]))
	}
	for _, p := range stale {
		lines = append(lines, fmt.Sprintf("   [附报·派生过期] %s ↔ %s —— **锚变而正文未变**（`T-11a` ② 面 · 本件只报不判 · 不动退码）", p[0], p[1]))
	}
	for _, p := range untouched {
		lines = append(lines, fmt.Sprintf("   [未判·无输入] %s ↔ %s —— 两侧都不在 diff 面", p[0], p[1]))
	}
	if len(excChanged) > 0 {
		lines = append(lines, "")
		lines = append(lines, "── 单一语例外件在 diff 面里（**无锚可比 ⇒ 本件不判**）──")
		for _, r := range excChanged {
			lines = append(lines, "   · "+r+"（已登记例外 · 处置面 = 该件自己的门）")
		}
	}
	if len(dangling) > 0 {
		lines = append(lines, "")
		lines = append(lines, "── 悬空例外登记（**只打印 · 不判红** · 同族口径）──")
		for _, r := range dangling {
			lines = append(lines, "   · "+r+"（登记了但扫面里没有这件 ⇒ 悬空）")
		}
	}
	if len(notInDiff) > 0 {
		lines = append(lines, "")
		lines = append(lines, "── `J4` 不在 diff 面（**不判** · 本仓多写手并行 ⇒ 兄弟写手的在途件不算漂移）──")
		for _, r := range notInDiff {
			lines = append(lines, "   · "+r)
		}
	}

	rc := 0
	if len(drift) > 0 {
		rc = 1
	}
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("⇒ 成对 %d · 同批 %d · **漂移 %d** · 派生过期(附报) %d · 未判 %d 对 · 不在 diff 面 %d 件 · rc=%d",
		len(pairs), len(both), len(drift), len(stale), len(untouched), len(notInDiff), rc))
	if rc == 1 {
		lines = append(lines, "   处置只有两条：**补上真源侧的改动**，或**撤掉单边改动** —— ✗ 不许放宽判据来凑绿。")
	} else {
		lines = append(lines, "   `J0` 成对性 + 例外登记过 ✓ · `J1` 无漂移 ✓（`J3` 的派生过期只报 · 不改退码）")
	}

	driftL := []any{}
	for _, p := range drift {
		driftL = append(driftL, map[string]any{"zh": p[0], "en": p[1]})
	}
	staleL := []any{}
	for _, p := range stale {
		staleL = append(staleL, map[string]any{"zh": p[0], "en": p[1]})
	}
	bothL := []any{}
	for _, p := range both {
		bothL = append(bothL, map[string]any{"zh": p[0], "en": p[1]})
	}
	data["rc"] = rc
	if len(drift) > 0 {
		data["verdict"] = "drift"
	} else {
		data["verdict"] = "green"
	}
	data["rev"] = rev
	data["counts"] = map[string]any{
		"pairs": len(pairs), "both": len(both), "drift": len(drift),
		"stale": len(stale), "untouched": len(untouched),
		"changed": len(changed), "not_in_diff": len(notInDiff),
		"singles": len(singles), "exceptions": len(excRel),
		"dangling_exceptions": len(dangling), "unregistered": 0,
	}
	data["drift"] = driftL
	data["stale"] = staleL
	data["both"] = bothL
	if notInDiff == nil {
		notInDiff = []string{}
	}
	data["not_in_diff"] = notInDiff
	data["exceptions"] = excRel
	return rc, lines, data
}

// ══ `--json` 编码（= python json.dumps(obj, ensure_ascii=False, sort_keys=True)）═════════

func cpfaPyJSONString(s string) string {
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
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func cpfaPyJSON(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case string:
		return cpfaPyJSONString(x)
	case []string:
		parts := make([]string, len(x))
		for i, s := range x {
			parts[i] = cpfaPyJSONString(s)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []any:
		parts := make([]string, len(x))
		for i, s := range x {
			parts[i] = cpfaPyJSON(s)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = cpfaPyJSONString(k) + ": " + cpfaPyJSON(x[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "null"
}

// ══ 自检（合成 git 夹具 · 不碰真目标）══════════════════════════════════════════════

func cpfaFixWrite(root, rel, txt string) {
	p := cpfaPyJoin(root, rel)
	_ = os.MkdirAll(cpfaPyDirname(p), 0o755)
	_ = os.WriteFile(p, []byte(txt), 0o644)
}

func cpfaMkdirAll(p string) { _ = os.MkdirAll(p, 0o755) }

// cpfaTouch = py `touch`：向 `<root>/publish/docs/<rel>` **追加**文本。
func cpfaTouch(root, rel, txt string) {
	p := cpfaPyJoin(root, cpfaFaceDefault, rel)
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(txt)
	_ = f.Close()
}

// cpfaMkFixture = py `mk_fixture`：`<base>/repo/publish/docs/<rel>` = files，`git init -q -b main` + 一笔提交。
func cpfaMkFixture(base string, files map[string]string) (string, bool) {
	root := cpfaPyJoin(base, "repo")
	names := make([]string, 0, len(files))
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, rel := range names {
		cpfaFixWrite(root, cpfaPyJoin(cpfaFaceDefault, rel), files[rel])
	}
	env := cpfaGitEnv(base)
	for _, a := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"}, {"commit", "-q", "-m", "init"}} {
		rc, _, _ := cpfaGitRun(root, a, env)
		if rc != 0 {
			return root, false
		}
	}
	return root, true
}

var cpfaPairA = map[string]string{"A.zh-CN.md": "# A 真源\n", "A.en.md": "# A derived\n"}
var cpfaReadmePair = map[string]string{"README.zh-CN.md": "# 真源\n", "README.md": "# Derived\n"}

func cpfaSelfTest(stdout, stderr io.Writer) int {
	type cell struct {
		ok   bool
		desc string
	}
	var checks []cell
	check := func(cond bool, desc string) {
		checks = append(checks, cell{cond, desc})
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", desc)
		} else {
			fmt.Fprintf(stdout, "   ✗ %s\n", desc)
		}
	}

	t, err := os.MkdirTemp("", "pubface-anchor-selftest-")
	if err != nil {
		fmt.Fprintf(stderr, "自检夹具建不起来（%s）\n", err)
		return 2
	}
	defer func() { _ = os.RemoveAll(t) }()

	// ① 正控：锚与正文**同批** ⇒ rc=0（绿）
	r, ok := cpfaMkFixture(cpfaPyJoin(t, "both"), cpfaPairA)
	if !ok {
		check(false, "自检夹具建不起来（both）")
	}
	cpfaTouch(r, "A.zh-CN.md", "真源加了半行\n")
	cpfaTouch(r, "A.en.md", "derived added half a line\n")
	rc, o, d := cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 0 && cpfaCounts(d, "both") == 1 && cpfaCounts(d, "drift") == 0,
		"① 正控：锚与正文**同批** ⇒ rc=0（同批 1 · 漂移 0）")

	// ② 负控（`T-13` 本条的牙齿）：**锚未变而正文变** ⇒ rc=1（红）
	r, _ = cpfaMkFixture(cpfaPyJoin(t, "drift"), cpfaPairA)
	cpfaTouch(r, "A.en.md", "只改了译文这一行\n")
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 1 && cpfaCounts(d, "drift") == 1 && cpfaAnyLine(o, "锚未变而正文变"),
		"② 负控：「只改译文不动锚」⇒ rc=1（漂移 1 · 逐条点名「锚未变而正文变」）")

	// ③ 附报：**锚变而正文未变** ⇒ rc=0 且 stale=1（**不改退码** —— 照 `T-13` 只判 ①）
	r, _ = cpfaMkFixture(cpfaPyJoin(t, "stale"), cpfaPairA)
	cpfaTouch(r, "A.zh-CN.md", "真源动了\n")
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 0 && cpfaCounts(d, "stale") == 1 && cpfaAnyLine(o, "派生过期"),
		"③ 附报：锚变而正文未变 ⇒ rc=0 且报「派生过期」1 处（`T-11a` ② 面 · **不动退码**）")

	// ④ 无输入：两侧都不动 ⇒ diff 面为空 ⇒ rc=0
	r, _ = cpfaMkFixture(cpfaPyJoin(t, "none"), cpfaPairA)
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 0 && cpfaCounts(d, "changed") == 0 && cpfaCounts(d, "untouched") == 1,
		"④ 无输入：diff 面为空 ⇒ rc=0（未判 1）")

	// ⑤ README 镜像对识别（`README.zh-CN.md` ↔ `README.md` · 公开面唯一不同名的一对）
	r, _ = cpfaMkFixture(cpfaPyJoin(t, "readme"), cpfaReadmePair)
	cpfaTouch(r, "README.en.md", "") // 噪音：不该被当派生件（无真源）
	_ = os.Remove(cpfaPyJoin(r, cpfaFaceDefault, "README.en.md"))
	cpfaTouch(r, "README.md", "derived only\n")
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 1 && cpfaCounts(d, "pairs") == 1 && cpfaCounts(d, "drift") == 1,
		"⑤ 镜像对：`README.md` 认作 `README.zh-CN.md` 的派生侧 ⇒ 单边改 ⇒ rc=1（成对 1）")

	// ⑥ 未成对（真源缺派生件）且未登记 ⇒ rc=2
	r, _ = cpfaMkFixture(cpfaPyJoin(t, "zhonly"), map[string]string{"B.zh-CN.md": "# B 真源\n"})
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 2 && cpfaStrList(d, "unregistered") == "B.zh-CN.md",
		"⑥ 未成对：`B.zh-CN.md` 无派生件、未登记 ⇒ rc=2（`J0` 前置有牙）")

	// ⑦ 未成对（派生缺真源）且未登记 ⇒ rc=2
	r, _ = cpfaMkFixture(cpfaPyJoin(t, "enonly"), map[string]string{"C.en.md": "# C derived\n"})
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 2 && cpfaStrList(d, "unregistered") == "C.en.md",
		"⑦ 未成对：`C.en.md` 无真源件、未登记 ⇒ rc=2")

	// ⑧ 单一语例外**已登记** ⇒ 不假红 · rc=0
	excFiles := map[string]string{"design/格式反馈协议-FFP.md": "# 理念件（单语）\n"}
	for k, v := range cpfaPairA {
		excFiles[k] = v
	}
	r, _ = cpfaMkFixture(cpfaPyJoin(t, "exc-ok"), excFiles)
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 0 && cpfaCounts(d, "singles") == 1 && cpfaCounts(d, "exceptions") == 2,
		"⑧ 登记例外：单语件在**已登记**表里 ⇒ rc=0（单一语 1 · 不假红）")

	// ⑨ 同一件**撤掉登记** ⇒ rc=2（证明登记面不是摆设）
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", nil)
	check(rc == 2 && cpfaAnyStr(cpfaListOf(d, "unregistered"), "design/格式反馈协议-FFP.md"),
		"⑨ 与 ⑧ 成对：同一件**撤掉登记** ⇒ rc=2（登记面有区分度 · 不恒绿）")

	// ⑩ 悬空例外（登记了但件不在）⇒ **只打印 · 不判红** ⇒ rc=0
	r, _ = cpfaMkFixture(cpfaPyJoin(t, "dangling"), cpfaPairA)
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 0 && cpfaCounts(d, "dangling_exceptions") == 2 && cpfaAnyLine(o, "悬空"),
		"⑩ 悬空登记：2 条例外件都不在扫面 ⇒ 只打印「悬空」· rc=0（同族口径 · 不判红）")

	// ⑪ 空转（扫面 0 件）⇒ rc=2
	r = cpfaPyJoin(t, "vacuous")
	cpfaMkdirAll(cpfaPyJoin(r, cpfaFaceDefault))
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 2 && cpfaStr(d, "verdict") == "vacuous", "⑪ 空转：扫面 0 件 ⇒ rc=2（空转 = 假覆盖）")

	// ⑫ 缺件（扫面不在）⇒ rc=2
	r = cpfaPyJoin(t, "noface")
	cpfaMkdirAll(r)
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 2 && cpfaStr(d, "verdict") == "missing-face", "⑫ 缺件：扫面不在 ⇒ rc=2（缺件 ≠ 绿）")

	// ⑬ 非仓 / `rev` 解不出 ⇒ rc=2
	r = cpfaPyJoin(t, "nogit")
	cpfaMkdirAll(cpfaPyJoin(r, cpfaFaceDefault))
	_ = os.WriteFile(cpfaPyJoin(r, cpfaFaceDefault, "A.zh-CN.md"), []byte("# A\n"), 0o644)
	_ = os.WriteFile(cpfaPyJoin(r, cpfaFaceDefault, "A.en.md"), []byte("# A\n"), 0o644)
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 2 && (cpfaStr(d, "verdict") == "no-git" || cpfaStr(d, "verdict") == "bad-rev"),
		"⑬ 非仓 / `rev` 解不出 ⇒ rc=2（读不到就不给结论）")

	// ⑭ `J4` untracked 件**不在 diff 面** ⇒ 不判 ⇒ rc=0
	r, _ = cpfaMkFixture(cpfaPyJoin(t, "untracked"), cpfaPairA)
	_ = os.WriteFile(cpfaPyJoin(r, cpfaFaceDefault, "D.zh-CN.md"), []byte("# D 真源\n"), 0o644)
	_ = os.WriteFile(cpfaPyJoin(r, cpfaFaceDefault, "D.en.md"), []byte("# D derived\n"), 0o644)
	rc, o, d = cpfaJudge(r, cpfaFaceDefault, "HEAD", cpfaExceptions)
	check(rc == 0 && cpfaAnyStr(cpfaListOf(d, "not_in_diff"), "D.en.md") && cpfaAnyStr(cpfaListOf(d, "not_in_diff"), "D.zh-CN.md"),
		"⑭ `J4`：untracked 对**不在 diff 面** ⇒ 不判、rc=0（且在输出里逐条点名）")

	// ⑮ 逃生开关（`--waive`）端到端 ⇒ rc=2 被拒（py 经 sys.executable 另起进程；本件进程内调本入口）
	var so2, se2 bytes.Buffer
	rc15 := checkPublicFaceAnchorGateMain(&so2, &se2, []string{"--waive"})
	check(rc15 == 2 && strings.Contains(so2.String()+se2.String(), "拒"),
		"⑮ 逃生开关：`--waive` 端到端 ⇒ rc=2 被拒（例外只能显式登记）")

	// ⑯ 区分度 + 三态齐：同一夹具「改一 ⇒ 1」「同批 ⇒ 0」「撤登记 ⇒ 2」
	r1, _ := cpfaMkFixture(cpfaPyJoin(t, "disc-a"), cpfaPairA)
	cpfaTouch(r1, "A.en.md", "single side\n")
	r2, _ := cpfaMkFixture(cpfaPyJoin(t, "disc-b"), cpfaPairA)
	cpfaTouch(r2, "A.en.md", "both\n")
	cpfaTouch(r2, "A.zh-CN.md", "both\n")
	r3, _ := cpfaMkFixture(cpfaPyJoin(t, "disc-c"), map[string]string{"E.zh-CN.md": "# E\n"})
	rc1, _, _ := cpfaJudge(r1, cpfaFaceDefault, "HEAD", cpfaExceptions)
	rc2, _, _ := cpfaJudge(r2, cpfaFaceDefault, "HEAD", cpfaExceptions)
	rc3, _, _ := cpfaJudge(r3, cpfaFaceDefault, "HEAD", cpfaExceptions)
	got := map[int]bool{rc1: true, rc2: true, rc3: true}
	check(got[0] && got[1] && got[2],
		"⑯ 区分度 + 三态齐：同夹具能出 {0, 1, 2} 三条出口（判据不恒绿 / 不恒红）")

	var bad []string
	for _, c := range checks {
		if !c.ok {
			bad = append(bad, c.desc)
		}
	}
	concl := "全过"
	if len(bad) > 0 {
		concl = "**不过**：" + strings.Join(bad, " · ")
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 **%d** 格 · 失败 %d 格）\n", concl, len(checks), len(bad))
	if len(bad) == 0 {
		return 0
	}
	return 2
}

func cpfaCounts(d map[string]any, k string) int {
	if c, ok := d["counts"].(map[string]any); ok {
		if v, ok := c[k].(int); ok {
			return v
		}
	}
	return -1
}

func cpfaStr(d map[string]any, k string) string {
	if v, ok := d[k].(string); ok {
		return v
	}
	return ""
}

func cpfaListOf(d map[string]any, k string) []string {
	if v, ok := d[k].([]string); ok {
		return v
	}
	return nil
}

func cpfaStrList(d map[string]any, k string) string {
	l := cpfaListOf(d, k)
	if len(l) == 0 {
		return ""
	}
	return l[0]
}

func cpfaAnyStr(xs []string, x string) bool { return cpfaContainsStr(xs, x) }

func cpfaAnyLine(lines []string, sub string) bool {
	for _, ln := range lines {
		if strings.Contains(ln, sub) {
			return true
		}
	}
	return false
}

// ══ `--list-rules` ══════════════════════════════════════════════════════════════

func cpfaListRules(stdout io.Writer) int {
	fmt.Fprintln(stdout, cpfaVersion)
	fmt.Fprint(stdout, "真源：任务清单-公开文档面收口-20260923.md:46（`T-13`）· 设计-公开文档面随版收口-v1.1-20260923.md:21/:385/:410\n")
	fmt.Fprint(stdout, "参照件：google/comprehensive-rust `.github/workflows/check-msgid-changes.py`（`E-2` · 已复核）\n")
	fmt.Fprint(stdout, "口径：**锚 = 真源侧**（`*.zh-CN.md` / `README.zh-CN.md`）· **正文 = 派生侧**（`*.en.md` / `README.md`）\n")
	fmt.Fprint(stdout, "`J0` 成对性 + 例外登记（前置 · 既不成对又未登记 ⇒ rc=2 · 逐条带理由 · 登记面 = `SINGLE_LANG_EXCEPTIONS`（`T-11b` 已正式登记））\n")
	fmt.Fprint(stdout, "`J1` 负控：**锚未变而正文变 = 红**（rc=1 · 逐对点名）\n")
	fmt.Fprint(stdout, "`J2` 正控：锚与正文**同批** ⇒ 绿\n")
	fmt.Fprint(stdout, "`J3` 附报（**不改退码**）：锚变而正文未变 ⇒ 「派生过期（stale）」（`T-11a` ② 面 · 只报不判）\n")
	fmt.Fprint(stdout, "`J4` 只在 tracked 的 diff 面里判（untracked 逐条打印「不在 diff 面 ⇒ 不判」）\n")
	fmt.Fprint(stdout, "退码：0 全绿 · 1 有漂移（锚未变而正文变）· 2 不给结论（缺件/空转/未登记/读不到/自检不过/拒收的开关）\n")
	fmt.Fprintf(stdout, "拒收的逃生开关：%s（一律 rc=2）\n", strings.Join(cpfaRefuseFlags, " / "))
	fmt.Fprint(stdout, "例外登记表（**正式登记** · `T-11b` 2026-09-24 接手落地 · 单一真源）：\n")
	for _, e := range cpfaExceptions {
		fmt.Fprintf(stdout, "   · %s —— %s\n", e.File, e.Reason)
	}
	fmt.Fprintf(stdout, "扫面：%s（仓根相对 · `--face` 可换）· 只判 `*.md`\n", cpfaFaceDefault)
	fmt.Fprint(stdout, "**不接闸**：未进 `precommit-gates.sh` · 未进 `scripts/公开标记.tsv` · 未进任何台账\n")
	return 0
}

// ══ 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的**字段名打错**不许静默 / 不许炸堆栈 ✗ ══════

func cpfaRefuseUnknownJSONFields(stderr io.Writer, argv []string) int {
	var want []string
	for i, x := range argv {
		if x == "--json" {
			v := ""
			if i+1 < len(argv) {
				v = argv[i+1]
			}
			want = append(want, strings.Split(v, ",")...)
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(strings.TrimPrefix(x, "--json="), ",")...)
		}
	}
	var clean []string
	for _, f := range want {
		if strings.TrimSpace(f) != "" {
			clean = append(clean, strings.TrimSpace(f))
		}
	}
	var bad []string
	for _, f := range clean {
		if !cpfaContainsStr(cpfaJSONFields, f) {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return 0
	}
	mark := func(f string) string { return "`" + f + "`" }
	badM := make([]string, len(bad))
	for i, b := range bad {
		badM[i] = mark(b)
	}
	allM := make([]string, len(cpfaJSONFields))
	for i, f := range cpfaJSONFields {
		allM[i] = mark(f)
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", strings.Join(badM, " · "))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", strings.Join(allM, " · "))
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开\n")
	return 2
}

// ══ 参数（= argparse 子集）════════════════════════════════════════════════════════

type cpfaArgs struct {
	root, face, rev, json string
	selfTest, noSelfTest  bool
	listRules, refuse     bool
}

// ★ argparse 版式（现读 /usr/bin/python3 3.9.6 复刻）：4 个逃生开关是**同一个 action 的 4 个别名**
// （`add_argument(*REFUSE_FLAGS, dest="refuse", action="store_true")`）⇒ usage 里只出首个别名，
// 帮助里 4 个别名同一行；续行缩进 = len("usage: ")+len(prog)+1 = 35。
var cpfaUsage = "usage: check-public-face-anchor.py [-h] [--root ROOT] [--face FACE]\n" +
	strings.Repeat(" ", 35) + "[--rev REV] [--json JSON] [--list-rules]\n" +
	strings.Repeat(" ", 35) + "[--self-test] [--no-self-test] [--waive]"

var cpfaHelpTextRaw = cpfaUsage + "\n" +
	"\n" +
	"公开面漂移锚（`T-13`）：成对件「锚未变而正文变 = 红」（只报件 · 不改任何件）\n" +
	"\n" +
	// ★ 这一行随 py 版本变：argparse 在 py ≤3.9 印 "optional arguments:"、≥3.10 印 "options:"。
	// 本件实跑环境钉在 **py 3.9**（见 GAP-20261002-04/-07：门跑环境的解释器与交互 shell 不同，
	// 对拍测试同样钉 `/usr/bin/python3`）⇒ 照 3.9 的形逐字对齐。
	//
	// ★★ **这是过渡期的形态，不是终局**（Mr2109 2026-10-02 质询后点明）：`optional arguments:`
	// 是 py 老版本 argparse 的遗留，不是任何设计意图。本件照它写，**唯一理由**是搬运期的验收
	// 判据 = 「Go 体输出 ⟷ `scripts/gates/check-public-face-anchor.py` 输出逐字相同」。
	// **等第 5 批「py 退场」做完**（py 原件离场、对这一格的对拍一并移除）⇒ **这一行就该改回现代的
	// `options:`**，同时删掉/改掉那条对拍格。退场时请一并处理，别把过渡形态留成永久设计。
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT           判哪棵树（默认仓根）\n" +
	"  --face FACE           扫面（仓根相对 · 默认 publish/docs）\n" +
	"  --rev REV             diff 面的基线（默认 HEAD ⇒ 判「HEAD → 工作树」）\n" +
	"  --json JSON           按名输出 JSON 字段（逗号分隔 · 空 = 全量）\n" +
	"  --list-rules          只打印判据与口径\n" +
	"  --self-test           只跑自检（合成 git 夹具 · 不碰真目标）\n" +
	"  --no-self-test        跳过自检直接判真目标（调用方显式选择）\n" +
	"  --waive, --allow-unregistered, --ignore-missing, --skip-pairs\n" +
	strings.Repeat(" ", 24) + "（**故意不支持**）想把例外绕过去 ⇒ 一律拒 rc=2\n"

var cpfaLongOpts = []string{"--help", "--root", "--face", "--rev", "--json", "--list-rules",
	"--self-test", "--no-self-test", "--waive", "--allow-unregistered", "--ignore-missing", "--skip-pairs"}

func cpfaDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func cpfaArgError(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, cpfaUsage)
	fmt.Fprintf(stderr, "check-public-face-anchor.py: error: %s\n", msg)
	return 2
}

// cpfaResolveOpt —— argparse 长选项匹配：先精确，再唯一前缀；>1 ⇒ 返回歧义候选（按登记序）。
func cpfaResolveOpt(name string) (string, []string) {
	for _, o := range cpfaLongOpts {
		if o == name {
			return o, nil
		}
	}
	var cands []string
	for _, o := range cpfaLongOpts {
		if strings.HasPrefix(o, name) {
			cands = append(cands, o)
		}
	}
	if len(cands) == 1 {
		return cands[0], nil
	}
	return "", cands
}

var cpfaValueOpts = []string{"--root", "--face", "--rev", "--json"}

func cpfaParseArgs(stdout, stderr io.Writer, argv []string) (cpfaArgs, int, bool) {
	a := cpfaArgs{root: cpfaDefaultRoot(), face: cpfaFaceDefault, rev: "HEAD"}
	var extras []string
	flagsTrue := []string{"--list-rules", "--self-test", "--no-self-test"}
	flagsTrue = append(flagsTrue, cpfaRefuseFlags...)
	for i := 0; i < len(argv); i++ {
		x := argv[i]
		if x == "-h" || x == "--help" {
			fmt.Fprint(stdout, cpfaHelpTextRaw)
			return a, 0, true
		}
		if strings.HasPrefix(x, "-") && x != "-" {
			body := x
			val := ""
			hasVal := false
			if j := strings.Index(x, "="); j > 0 {
				body, val, hasVal = x[:j], x[j+1:], true
			}
			canon, amb := cpfaResolveOpt(body)
			if canon == "" {
				if len(amb) > 0 {
					return a, cpfaArgError(stderr, "ambiguous option: "+body+" could match "+strings.Join(amb, ", ")), true
				}
				extras = append(extras, x)
				continue
			}
			switch {
			case cpfaContainsStr(cpfaValueOpts, canon):
				if hasVal {
					cpfaSetVal(&a, canon, val)
					continue
				}
				if i+1 >= len(argv) {
					return a, cpfaArgError(stderr, "argument "+canon+": expected one argument"), true
				}
				i++
				cpfaSetVal(&a, canon, argv[i])
			case canon == "--help":
				if hasVal {
					return a, cpfaArgError(stderr, "argument "+canon+": ignored explicit argument '"+val+"'"), true
				}
				fmt.Fprint(stdout, cpfaHelpTextRaw)
				return a, 0, true
			default: // 布尔旗标
				if hasVal {
					return a, cpfaArgError(stderr, "argument "+canon+": ignored explicit argument '"+val+"'"), true
				}
				switch canon {
				case "--list-rules":
					a.listRules = true
				case "--self-test":
					a.selfTest = true
				case "--no-self-test":
					a.noSelfTest = true
				default:
					a.refuse = true
				}
			}
			continue
		}
		extras = append(extras, x)
	}
	if len(extras) > 0 {
		return a, cpfaArgError(stderr, "unrecognized arguments: "+strings.Join(extras, " ")), true
	}
	return a, 0, false
}

func cpfaSetVal(a *cpfaArgs, canon, val string) {
	switch canon {
	case "--root":
		a.root = val
	case "--face":
		a.face = val
	case "--rev":
		a.rev = val
	case "--json":
		a.json = val
	}
}

// ══ 入口（= py `main()`）═════════════════════════════════════════════════════════

func checkPublicFaceAnchorGateMain(stdout, stderr io.Writer, argv []string) (rc int) {
	// 最外层兜底（py 原件无此档）⇒ fail-closed rc=2 + 具名文案（口径差见件头 / 未覆盖面）。
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stdout, "⛔ 判据内部异常（不给结论）：%v\n", r)
			rc = 2
		}
	}()

	if r := cpfaRefuseUnknownJSONFields(stderr, argv); r != 0 {
		return r
	}
	a, rc2, done := cpfaParseArgs(stdout, stderr, argv)
	if done {
		return rc2
	}
	if a.refuse {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s 一类「把例外绕过去」的开关**本质被拒** —— "+
			"例外只能**显式登记**（本件 `SINGLE_LANG_EXCEPTIONS` · 逐条带理由）。\n",
			strings.Join(cpfaRefuseFlags, " / "))
		return 2
	}
	if a.listRules {
		return cpfaListRules(stdout)
	}
	if a.selfTest {
		fmt.Fprintln(stdout, cpfaVersion)
		return cpfaSelfTest(stdout, stderr)
	}

	fmt.Fprintln(stdout, cpfaVersion)
	if !a.noSelfTest {
		if rcST := cpfaSelfTest(stdout, stderr); rcST != 0 {
			fmt.Fprint(stdout, "⛔ 自检不过 ⇒ 拒绝判真目标（门禁自己先能被证明「会红」）\n")
			return 2
		}
	}
	faceRel := strings.TrimRight(a.face, "/")
	rcJ, lines, data := cpfaJudge(cpfaPyAbspath(a.root), faceRel, a.rev, cpfaExceptions)
	for _, ln := range lines {
		fmt.Fprintln(stdout, ln)
	}
	if a.json != "" {
		var want []string
		for _, w := range strings.Split(a.json, ",") {
			if strings.TrimSpace(w) != "" {
				want = append(want, strings.TrimSpace(w))
			}
		}
		var obj any = data
		if len(want) > 0 {
			sel := map[string]any{}
			for _, k := range want {
				sel[k] = data[k]
			}
			obj = sel
		}
		fmt.Fprintln(stdout, cpfaPyJSON(obj))
	}
	return rcJ
}

var _ = utf8.RuneCountInString
