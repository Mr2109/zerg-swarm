// family_ci.go —— 公开面 CI「归绿」**只读**一格（组4 §二.4 `W-51` · `研-禁:134` `R-21` 归绿核心 ·
// 任务单序133 · 与组3 `Q-025` 同条）。
//
// 为什么它排这一波（源件原话，不重述）：归绿判据逐字是「**指定 commit 的必需检查集合全部
// `success`、且没有一个 `skipped`**，读数带 `head_sha`」—— 而现读 `grep -rn "gh run" scripts/ core/`
// ⇒ **0 命中** ⇒ 这条判据今天**只能人贴屏**（`W-51` 现读栏逐字「公开面 CI 状态仍只能人贴屏 ✗」）。
// 人贴屏那张图里**没有 `head_sha`**、也**不说「必需集合」是哪几条** ⇒ 它判不了这一句。
//
// 口径（照源件逐字，不自造）：
//
//	形态 `zerg ci green --run <记录件> [--decl <声明件>] [--json <字段>]`
//	     （**只读** · 不写盘 · 不改状态 · 不新开端点 · **不发任何网络请求** ——
//	      只读本机盘上两份件：① check-run **记录件** ② 必需集合**声明件**）
//	判据 归绿 = 声明集合**逐名**在记录里 `status=completed` 且 `conclusion=success`、且
//	     集合内 `skipped` 计数 = 0；**读数带 `head_sha`**（每条读数都要带 —— 与序132 那条
//	     「凡读数都要带口径 + 身份」同一条纪律）
//	退码 `0` 判绿 · `1` 判红（**负控**：集合内有一条 `skipped` ⇒ 判红）· `8` **不给结论**
//	     · `2` 用法错（照既有 `K2` 一套，本命令不另立）
//
// ★ **两处「不许并」写死**（这是本命令最要紧的边界，源件那句「人贴屏不算判据」的落点）：
//
//	① **「声明集合」不许与「GitHub 侧 settings 的必需集合」并成一个词** —— 后者要读 GitHub 侧
//	   branch protection 才有，本机**未核**（`Zerg-内部文档/项目文档/v2.5.11/调研-缺口-门禁证据-20260923.md:209`
//	   逐字记「**未核**（要 GitHub 侧 settings 才看得到）」，同件的门 `check-public-ci-green.py` 也逐字写着
//	   「**不声称**「必需检查集合 = N 个 job」」）⇒ 本命令把这件事**印在口径行里**：声明件自己带
//	   `source` 一格（闭集 `github_settings` / `unverified`），`unverified` ⇒ 读数照出，
//	   但口径行明写「与 GitHub 侧 settings 的对齐**未核**」。消费侧**读得到**这一格 ⇒ 不会误读。
//	② **「记录里没有它」不许当 `success`**（最重的假绿）—— 归绿要的是「**全部** `success`」，
//	   一条**讲不出结论**的必需检查**不许**被当成「它绿了」⇒ 归 `8`（不给结论），**不是**绿。
//	   ★ 判序（写死 · 逐条可核）：① 集合内**有一条判得出非 success** ⇒ **红**（真读数不许被 8 吞）；
//	   ② 集合内**有一条判不出**（记录里没有它）⇒ **8**；③ 其余 ⇒ **绿**。
//
// ★ 记录件为什么是「check-runs 面」这一种形状：它是**人贴屏那张图**的机器替身 ——
// GitHub 上「某一趟 commit 的检查」在 API 面就是 `check_runs[]`，每格三列
// （`name` / `status` / `conclusion`）+ 那趟的 `head_sha`。本命令**只认这一种形状**：
// 认不出的形状 ⇒ `8`（不给结论 · 一行都不出），**不猜、不降级**（同 `check-approve-ci.py` C3
// 「口径件不在 ⇒ 也不出结论」那条）。
//
// ★ 声明件形状（`publish/ci/required-checks.json` —— 落点栏的**消费者**侧）：
//
//	{"id":"zerg.ci.required-checks.v1","source":"unverified","source_note":"…","checks":["…",…]}
//	· `source` 闭集 `github_settings` / `unverified` —— 闭集外 ⇒ `8`（形态不认）
//	· `checks` 空 ⇒ `8` —— **空集**上「全部 success」是**空洞的真**，那是最难发现的一类假绿
//	  （同 `check-ci-package-parity.py` C3 的守卫口径：「配置没了所以两边都空、所以全绿」）
//	· `checks` 有重名 ⇒ `8`（同名两条 ⇒ 讲不出判的是哪一条）
//	★ 声明件与 `publish/ci/ci.yml` 的 job 名**是不是同一集合**，由判据件
//	  `cli_ci_green_test.go` 的 `TestCiGreen_DeclarationMatchesCIFile` 每次 `go test` 现读对拍
//	  （两套真源并存就会漂 ⇒ 立判据——与 `check-ci-package-parity.py` 同一条理由）。
//
// ★ 一处照实记的偏离：本命令**不**去 GitHub 取数（无网络面、无凭据面）⇒「记录件从哪来」这件事
// 本命令**只管读**（`--run` 点名）；「谁去把记录件抓下来」**未落** ✗（照实登记在回执 §八）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ciGreenCaliber —— 这条读数的**口径**（写死一处 · 人面与机器面都读它）。
//
// 为什么抽成一个常量：与序132 同一条纪律 —— 读数必须带口径；口径散在文案里 ⇒ 下一位只会看见
// 「判：绿」两个字，仍然讲不出「按什么算的绿」。
const ciGreenCaliber = "记录件 = check-runs 面（每格 name/status/conclusion + 顶层 head_sha）· 判绿 = 声明集合逐名 status=completed 且 conclusion=success、且集合内 skipped 计数 = 0"

// ciGreenDeclDefaultRel —— 声明件的**默认落点**（相对仓根）。落点栏逐字点的是
// `publish/ci/**`（消费者）+ `core/cmd/zerg/`（读面）⇒ 声明面在消费者侧、读面在本族。
const ciGreenDeclDefaultRel = "publish/ci/required-checks.json"

// ciGreenDeclID —— 声明件自己的形状号（认不出 ⇒ 不给结论）。
const ciGreenDeclID = "zerg.ci.required-checks.v1"

// ciGreenRecordID —— 记录件自己的形状号（可省；给了就必须逐字相同）。
const ciGreenRecordID = "zerg.ci.check-runs.v1"

// ciGreenSources —— `source` 的**闭集**（源件那条「未核」的落点：闭集外 ⇒ 8）。
var ciGreenSources = []string{"github_settings", "unverified"}

// ciGreenFields —— `--json` 可取的六格（**顺序即人面那一行的顺序**）。
//
// ★ 与 `main.go` 里那条登记**同一个值**（登记那一处必须写成 `[]string{…}` 字面量 ——
// 原因写在 `main.go` `net probe` 那条的注释里：契约脚本的 `FIELDS_RE` 只认字面量）。
// 两处同值由 `cli_ci_green_test.go` 的 `TestCiGreen_UsageFace` 用现跑对拍钉住。
var ciGreenFields = []string{"head_sha", "caliber", "decl_source", "required", "skipped", "verdict"}

// ciGreenIdentity — 每条读数**必须**带的身份两格（与 `publish tree has` 同一条口径）。
//
// 为什么它俩不接受被投影掉：`--json <字段>` 是**字段投影**（§4.1 `K1`/`K2`），而这两格是这条
// 读数的**身份**（「按什么口径算的」「算的是哪一趟 commit」）—— 投影掉任一件，剩下的那个
// 「判：绿」就又变成了源件要治的那个病（归绿读数不带 `head_sha`）。
var ciGreenIdentity = []string{"head_sha", "caliber"}

// ciCheckRun —— 记录件里的一格检查（三列，一格不多、一格不少）。
type ciCheckRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// ciRecord —— check-run **记录件**（人贴屏那张图的机器形态）。
type ciRecord struct {
	ID        string       `json:"id"`
	HeadSHA   string       `json:"head_sha"`
	CheckRuns []ciCheckRun `json:"check_runs"`
}

// ciDecl —— 必需集合**声明件**（消费者侧）。
type ciDecl struct {
	ID         string   `json:"id"`
	Source     string   `json:"source"`
	SourceNote string   `json:"source_note"`
	Checks     []string `json:"checks"`
}

// ciGreenHasField —— 用户给的字段序里点名了哪一格（供身份两格的必查）。
func ciGreenHasField(fields []string, want string) bool {
	for _, f := range fields {
		if f == want {
			return true
		}
	}
	return false
}

// ciGreenInSet —— 值在不在闭集里（闭集外 ⇒ 不给结论，不是静默放过）。
func ciGreenInSet(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

// ciGreenReadJSON —— 读一件 JSON 到 `out`；**只读**（不建、不改、不删）。
//
// 三档错因**分开报**（都归 `8`，但不合成一句模糊话）：件不在盘 / 读不动 / 不是合的 JSON。
func ciGreenReadJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("件不在盘或读不动：%v", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("件不是合的 JSON：%v", err)
	}
	return nil
}

// ciGreenDeclPath —— 声明件落点：`--decl` 给了就照给；没给 ⇒ 仓根下的默认落点。
// 仓根解析不到 ⇒ 返回空串（调用方报 `8`，**不猜路径**）。
func ciGreenDeclPath(inv *invocation) string {
	for _, v := range inv.flagVals("--decl") {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	root := repoRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, filepath.FromSlash(ciGreenDeclDefaultRel))
}

// cmdCiGreen —— `zerg ci green --run <记录件> [--decl <声明件>]`：归绿判据的**只读**一格。
func cmdCiGreen(inv *invocation, stdout, stderr io.Writer) int {
	// ── ① `K2` 甲档：给了 `--json` 而不给字段 ⇒ 退码取自退码表（`usage` = 2）+ stdout 0 字节 ──
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		// ② 身份两格必查（源件判据栏逐字「读数带 `head_sha`」）：`--json` 里缺 ⇒ 拒。
		missing := []string{}
		for _, need := range ciGreenIdentity {
			if !ciGreenHasField(inv.fields, need) {
				missing = append(missing, need)
			}
		}
		if len(missing) > 0 {
			inv.setErr("usage", "identity_cells_required", "归绿读数必须带口径 + head_sha")
			fmt.Fprintf(stderr, "%s: 拒（退码 2 · 不给结论）：`--json` 里缺 %s\n",
				progName, strings.Join(missing, " · "))
			fmt.Fprintf(stderr, "%s: 理由：判据栏逐字「读数带 `head_sha`」（`W-51` · `R-21`）—— "+
				"这两格是**读数的身份**，不是可选投影格 ⇒ 缺任一件就不出这一条读数\n", progName)
			return exitUsage
		}
	}

	// ── ③ 位置参数：**一格都不收**（`--run` / `--decl` 都是值旗标） ──
	if len(inv.args) != 0 {
		inv.setErr("usage", "no_positional", "本命令不收位置参数")
		fmt.Fprintf(stderr, "%s: 用法：%s ci green --run <记录件> [--decl <声明件>] [--json <字段>]（现读给了 %d 个位置参数）\n",
			progName, progName, len(inv.args))
		return exitUsage
	}

	// ── ④ `--run` 必须**点名一件**（不许拿本机 glob 猜：那是「人贴屏」的病根形态） ──
	runs := []string{}
	for _, v := range inv.flagVals("--run") {
		if s := strings.TrimSpace(v); s != "" {
			runs = append(runs, s)
		}
	}
	if len(runs) != 1 {
		inv.setErr("usage", "need_one_run", "要恰好一件 check-run 记录件（--run）")
		fmt.Fprintf(stderr, "%s: 用法：%s ci green --run <记录件> [--decl <声明件>] [--json <字段>]（现读给了 %d 件）\n",
			progName, progName, len(runs))
		fmt.Fprintf(stderr, "%s: 口径：记录件**要点名**（它是**人贴屏那张图**的机器替身）—— 不许拿本机 glob 猜\n", progName)
		return exitUsage
	}
	runPath := runs[0]

	// ── ⑤ 声明件（**必需集合**的来处）：不在盘 / 形状不认 / 空集 / 重名 ⇒ **不给结论**（8） ──
	declPath := ciGreenDeclPath(inv)
	if declPath == "" {
		inv.setErr("blocked", "decl_path_unresolved", "仓根解析不到 ⇒ 默认声明件落点定不下来")
		fmt.Fprintf(stderr, "%s: 仓根解析不到 ⇒ 默认声明件（%s）落点定不下来 ⇒ **不给结论**（退码 8 · 一行都不出）；"+
			"要判请显式给 `--decl <声明件>`\n", progName, ciGreenDeclDefaultRel)
		return exitBlocked
	}
	var decl ciDecl
	if err := ciGreenReadJSON(declPath, &decl); err != nil {
		inv.setErr("blocked", "decl_unreadable", "必需集合声明件读不上来")
		fmt.Fprintf(stderr, "%s: 声明件 %s 取不上来 —— %v ⇒ **不给结论**（退码 8 · 一行都不出）\n",
			progName, declPath, err)
		fmt.Fprintf(stderr, "%s: 口径：归绿的「必需集合」来自**声明件**；口径件不在 ⇒ 也不出结论（不猜、不假绿）\n", progName)
		return exitBlocked
	}
	if decl.ID != ciGreenDeclID {
		inv.setErr("blocked", "decl_shape_unknown", "声明件的形状号认不得")
		fmt.Fprintf(stderr, "%s: 声明件 %s 的形状号 %q 认不得（本版只认 %q）⇒ **不给结论**（退码 8 · 一行都不出）\n",
			progName, declPath, decl.ID, ciGreenDeclID)
		return exitBlocked
	}
	if !ciGreenInSet(ciGreenSources, decl.Source) {
		inv.setErr("blocked", "decl_source_unknown", "声明件的 source 不在闭集里")
		fmt.Fprintf(stderr, "%s: 声明件 %s 的 `source` = %q **不在闭集** {%s} ⇒ **不给结论**（退码 8 · 一行都不出）\n",
			progName, declPath, decl.Source, strings.Join(ciGreenSources, "|"))
		return exitBlocked
	}
	if len(decl.Checks) == 0 {
		inv.setErr("blocked", "decl_checks_empty", "声明集合是空的 —— 空集上「全部 success」是空洞的真")
		fmt.Fprintf(stderr, "%s: 声明件 %s 的 `checks` 是**空的** ⇒ **不给结论**（退码 8 · 一行都不出）\n", progName, declPath)
		fmt.Fprintf(stderr, "%s: 理由：空集上「**全部** success」是**空洞的真**（配置没了 ⇒ 两边都空 ⇒ 全绿）—— "+
			"那是最难发现的一类假绿（同门 `check-ci-package-parity.py` C3 的守卫口径）\n", progName)
		return exitBlocked
	}
	seen := map[string]bool{}
	for _, name := range decl.Checks {
		if strings.TrimSpace(name) == "" {
			inv.setErr("blocked", "decl_name_blank", "声明集合里有名字是空的格")
			fmt.Fprintf(stderr, "%s: 声明件 %s 的 `checks` 里有**空名字** ⇒ **不给结论**（退码 8 · 一行都不出）\n", progName, declPath)
			return exitBlocked
		}
		if seen[name] {
			inv.setErr("blocked", "decl_name_dup", "声明集合里有重名")
			fmt.Fprintf(stderr, "%s: 声明件 %s 的 `checks` 里 %q **出现两次** ⇒ 同名两条讲不出判的是哪一条 ⇒ "+
				"**不给结论**（退码 8 · 一行都不出）\n", progName, declPath, name)
			return exitBlocked
		}
		seen[name] = true
	}

	// ── ⑥ 记录件：不在盘 / 形状不认 / 头部 sha 空 / 一条检查都没有 ⇒ **不给结论**（8） ──
	var rec ciRecord
	if err := ciGreenReadJSON(runPath, &rec); err != nil {
		inv.setErr("blocked", "run_unreadable", "check-run 记录件读不上来")
		fmt.Fprintf(stderr, "%s: 记录件 %s 取不上来 —— %v ⇒ **不给结论**（退码 8 · 一行都不出）\n",
			progName, runPath, err)
		fmt.Fprintf(stderr, "%s: 口径：记录件 = check-runs 面（每格 name/status/conclusion + 顶层 head_sha）—— "+
			"形状认不出就**不猜**（人贴屏那张图也不是判据）\n", progName)
		return exitBlocked
	}
	if rec.ID != "" && rec.ID != ciGreenRecordID {
		inv.setErr("blocked", "run_shape_unknown", "记录件的形状号认不得")
		fmt.Fprintf(stderr, "%s: 记录件 %s 的形状号 %q 认不得（本版只认 %q）⇒ **不给结论**（退码 8 · 一行都不出）\n",
			progName, runPath, rec.ID, ciGreenRecordID)
		return exitBlocked
	}
	if strings.TrimSpace(rec.HeadSHA) == "" {
		inv.setErr("blocked", "run_head_sha_absent", "记录件没有 head_sha")
		fmt.Fprintf(stderr, "%s: 记录件 %s **没有 `head_sha`** ⇒ 这条读数没有身份 ⇒ **不给结论**（退码 8 · 一行都不出）\n",
			progName, runPath)
		fmt.Fprintf(stderr, "%s: 口径：判据栏逐字要求「读数带 `head_sha`」—— 没有身份的读数不是合规读数\n", progName)
		return exitBlocked
	}
	if len(rec.CheckRuns) == 0 {
		inv.setErr("blocked", "run_checks_empty", "记录件里一条检查都没有")
		fmt.Fprintf(stderr, "%s: 记录件 %s 里 `check_runs` 是**空的** ⇒ 讲不出任何一条检查的结论 ⇒ "+
			"**不给结论**（退码 8 · 一行都不出）\n", progName, runPath)
		return exitBlocked
	}
	byName := map[string]ciCheckRun{}
	for _, r := range rec.CheckRuns {
		n := strings.TrimSpace(r.Name)
		if n == "" {
			inv.setErr("blocked", "run_name_blank", "记录件里有名字是空的格")
			fmt.Fprintf(stderr, "%s: 记录件 %s 里有一格的 `name` 是**空的** ⇒ **不给结论**（退码 8 · 一行都不出）\n",
				progName, runPath)
			return exitBlocked
		}
		if _, dup := byName[n]; dup {
			inv.setErr("blocked", "run_name_dup", "记录件里同名检查出现两次")
			fmt.Fprintf(stderr, "%s: 记录件 %s 里 %q **出现两次** ⇒ 同一趟里同名两条讲不出判的是哪一条 ⇒ "+
				"**不给结论**（退码 8 · 一行都不出）\n", progName, runPath, n)
			return exitBlocked
		}
		byName[n] = r
	}

	// ── ⑦ 判（判序写死 · 见文件头）：非 success 一条 ⇒ 红；有一条判不出 ⇒ 8；其余 ⇒ 绿 ──
	nonGreen, absent, skipped := []string{}, []string{}, 0
	for _, name := range decl.Checks {
		r, ok := byName[name]
		if !ok {
			// 「记录里没有它」**不等于** success —— 最重的假绿就长在这里。
			absent = append(absent, name)
			continue
		}
		if r.Status != "completed" || r.Conclusion != "success" {
			nonGreen = append(nonGreen, name)
			if r.Conclusion == "skipped" {
				skipped++
			}
		}
	}

	// 人面：口径 + 身份 + 声明来处（`source`）逐行照出 —— 三条都不许省。
	fmt.Fprintf(stderr, "%s: 口径 = %s\n", progName, ciGreenCaliber)
	fmt.Fprintf(stderr, "%s: 必需集合 = 声明件 %s（%d 条 · `source=%s`）· 记录件 %s（%d 格）\n",
		progName, declPath, len(decl.Checks), decl.Source, runPath, len(rec.CheckRuns))
	if decl.Source == "unverified" {
		fmt.Fprintf(stderr, "%s: ★ 声明件的 `source=unverified` ⇒ 「**必需**集合」这一格**未核**"+
			"（要 GitHub 侧 settings 才看得到 —— `调研-缺口-门禁证据-20260923.md:209` 逐字「未核」）⇒ "+
			"本读数只答「**声明集合**」这一格；**不许**把它读成「GitHub 侧 required 全绿」\n", progName)
	}
	if strings.TrimSpace(decl.SourceNote) != "" {
		fmt.Fprintf(stderr, "%s: 声明件自述 = %s\n", progName, decl.SourceNote)
	}

	verdict := "绿"
	switch {
	case len(nonGreen) > 0:
		verdict = "红"
	case len(absent) > 0:
		verdict = "不给结论"
	}

	fmt.Fprintf(stderr, "%s: 判 = %s · head_sha = %s · 声明 %d 条 · 集合内 skipped = %d · 判不出 %d 条\n",
		progName, verdict, rec.HeadSHA, len(decl.Checks), skipped, len(absent))
	for _, n := range nonGreen {
		fmt.Fprintf(stderr, "%s: ★ 非绿：%s · status=%s · conclusion=%s\n",
			progName, n, byName[n].Status, byName[n].Conclusion)
	}
	for _, n := range absent {
		fmt.Fprintf(stderr, "%s: ★ 判不出：%s **不在记录件里**（「没出现」≠ `success` ⇒ 不作绿算）\n", progName, n)
	}

	row := map[string]string{
		"head_sha":    rec.HeadSHA,
		"caliber":     ciGreenCaliber,
		"decl_source": decl.Source,
		"required":    fmt.Sprintf("%d", len(decl.Checks)),
		"skipped":     fmt.Sprintf("%d", skipped),
		"verdict":     verdict,
	}

	if len(nonGreen) > 0 {
		// 判红 = **一条答案**（不是「不给结论」）⇒ 该行照出 · 退码 1（负控：`skipped` ⇒ 判红）。
		rc := listCmd(inv, stdout, stderr, ciGreenFields, []map[string]string{row})
		if rc != exitOK {
			return rc
		}
		inv.setErr("failed", "required_not_green", "必需集合里有非 success 的检查")
		fmt.Fprintf(stderr, "%s: 判红（退码 1）—— 归绿判据逐字要「**全部** `success`、且没有一个 `skipped`」\n", progName)
		return exitFail
	}
	if len(absent) > 0 {
		// 判不出 = **没有结论** ⇒ 一行都不出（`8`）。
		inv.setErr("blocked", "required_check_absent", "必需集合里有检查不在记录件里")
		fmt.Fprintf(stderr, "%s: 有 %d 条必需检查**判不出** ⇒ **不给结论**（退码 8 · 一行都不出）——\n", progName, len(absent))
		fmt.Fprintf(stderr, "%s: 理由：归绿要「**全部** `success`」，而「记录里没有它」**不是** success（宁可不给结论，不许给半张表）\n", progName)
		return exitBlocked
	}

	// 判绿：声明集合逐名 success + 集合内 skipped = 0。
	fmt.Fprintf(stderr, "%s: 本趟判绿 ⇒ 声明集合 %d 条逐名 `completed`+`success`、集合内 `skipped` = 0（读数带 `head_sha`）\n",
		progName, len(decl.Checks))
	if rc := listCmd(inv, stdout, stderr, ciGreenFields, []map[string]string{row}); rc != exitOK {
		return rc
	}
	return exitOK
}
