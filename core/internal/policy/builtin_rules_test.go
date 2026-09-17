// builtin_rules_test.go — T5.2 实证：两张清单（①「任何模式都不自动批」②「永不自动批」）
// 落成数据之后，逐条钉住三件事：
//
//	(a) **任何档位下都不自动批**：清单里每一条的动作，在 default / bypass / dontask 三档下
//	    判定结果**绝不为 allow**（这是两张清单唯一的承诺，也是本批唯一的安全断言）。
//	(b) **覆盖度**：每一条都有「该规则能命中」的正例，以及「不该被误伤」的反例
//	    （只断"该拦的拦了"会漏掉"把一切都拦了"这种假绿 —— 与 T5.1 用例同一纪律）。
//	(c) **清单完整性**：F1 两张表的每一条 bullet 都有落点（规则 or 未实体化记账），
//	    且**没有孤儿条目**（不存在"不在两张清单里"的规则混进来）。
//
// 另外钉住：数据自洽（id 唯一、无 allow、形态可判别）、地板压过档位与普通 allow、
// 未实体化条款必须写明理由、返回值不共享可变状态。
package policy

import (
	"strings"
	"testing"
)

// ── 夹具 ────────────────────────────────────────────────────────────────────

// builtinAllowedTools — 内置条目允许引用的工具名（源：core/internal/agent/tools.go 的 DefaultTools
// L0 内置 15 个，加 chat 扩展注册表里本批用到的几个）。新增内置条目若引用别的工具名，
// **必须**同时把仓内真名加到这里 —— 这一步就是"防凭空造工具名"的闸。
var builtinAllowedTools = map[string]bool{
	"bash": true, "read": true, "write": true, "edit": true, "glob": true, "grep": true, "ls": true,
	"web_search": true, "web_fetch": true, "skill_load": true, "tool_search": true, "screenshot": true,
	"apply_patch": true, "spawn_agent": true, "todo": true,
	"delete_file": true, "move_file": true, "copy_file": true, // 扩展工具（core/internal/chat 注册表）
}

// floorEngine — 只装指定 id 的地板引擎（**用真实数据切片**，不是另抄一份）。
// 单条引擎是"这条规则到底管不管这事"的干净判据：多条同效果规则并存时，
// 命中的总是先出现的那条（T5.1 语义），会让"某条规则其实没生效"看不出来。
func floorEngine(t *testing.T, ids ...string) *Engine {
	t.Helper()
	all := BuiltinFloor()
	sel := make([]Rule, 0, len(ids))
	for _, want := range ids {
		found := false
		for _, r := range all {
			if r.ID == want {
				sel = append(sel, r)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("内置地板里找不到 id %q（id 改名/删条必须同步本用例）", want)
		}
	}
	return NewEngine(nil, sel)
}

// fullFloorEngine — 全量内置地板（接线后的真实形态：地板 = BuiltinFloor()）。
func fullFloorEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(nil, BuiltinFloor())
}

// neverAllow — 本批唯一的安全断言：结论绝不为 allow。
func neverAllow(t *testing.T, what string, v Verdict) {
	t.Helper()
	if v.Decision == EffectAllow {
		t.Errorf("%s 在档位下被放行了（decision=allow, reason=%s）—— 两张清单的承诺是「永不自动批」", what, v.Reason)
	}
	if v.Decision != EffectAsk && v.Decision != EffectDeny {
		t.Errorf("%s 结论既不是 ask 也不是 deny：%q", what, v.Decision)
	}
}

// ── 逐条用例表：正例（必须命中本规则）/ 反例（不得被本规则误伤）──────────────

type builtinCase struct {
	hit  map[string]any // 正例：本规则必须命中
	miss map[string]any // 反例：本规则不得命中（同一工具名下的近邻用法）
}

func builtinCases() map[string]builtinCase {
	m := map[string]builtinCase{
		// critical path 删除：家目录三种拼写（逐字相等）。反例同时钉住两个已知取舍：
		// ① 逐字相等 ⇒ 带 `/` 的子路径写法不在这条规则管辖内（拼写全集由工具层 v1.0.2 兜）；
		// ② 匹配式里带 `/` 会被判成"路径匹配式" ⇒ 那类整条写法**根本写不得**（见 builtin_rules.go
		//    的 builtinCriticalDeleteRules 注释与 pending.any.rm_critical_full）。
		"any.critical.rm_home":        {hit: cmd("rm -rf ~"), miss: cmd("rm -rf ~/tmp-zerg-x")},
		"any.critical.rm_home_env":    {hit: cmd("rm -rf $HOME"), miss: cmd("rm -rf $HOME/tmp-zerg-x")},
		"any.critical.rm_home_braces": {hit: cmd("rm -rf ${HOME}"), miss: cmd("rm -rf ${HOME}/tmp-zerg-x")},

		// 表②：包安装
		"never.pkg.npm_install":  {hit: cmd("npm install express"), miss: cmd("npm test")},
		"never.pkg.npm_i":        {hit: cmd("npm i express"), miss: cmd("npm run build")},
		"never.pkg.npm_ci":       {hit: cmd("npm ci"), miss: cmd("npm cache verify")},
		"never.pkg.yarn_add":     {hit: cmd("yarn add lodash"), miss: cmd("yarn test")},
		"never.pkg.pnpm_add":     {hit: cmd("pnpm add lodash"), miss: cmd("pnpm test")},
		"never.pkg.pip_install":  {hit: cmd("pip install requests"), miss: cmd("pip list")},
		"never.pkg.pip3_install": {hit: cmd("pip3 install requests"), miss: cmd("pip3 list")},
		"never.pkg.brew_install": {hit: cmd("brew install jq"), miss: cmd("brew list")},
		"never.pkg.go_get":       {hit: cmd("go get golang.org/x/text"), miss: cmd("go test ./...")},
		"never.pkg.go_install":   {hit: cmd("go install example.com/cmd@latest"), miss: cmd("go build ./...")},
		"never.pkg.cargo_install": {hit: cmd("cargo install ripgrep"),
			miss: cmd("cargo test")},

		// 表②：变更型 git
		"never.git.reset_hard": {hit: cmd("git reset --hard HEAD~1"), miss: cmd("git reset HEAD -- core/internal/policy/policy.go")},
		"never.git.checkout_paths": {hit: cmd("git checkout -- core/internal/policy/policy.go"),
			miss: cmd("git checkout -b t5.2-builtin-floor")},
		"never.git.clean_force": {hit: cmd("git clean -fd"), miss: cmd("git clean -n")},
		"never.git.push":        {hit: cmd("git push origin main"), miss: cmd("git status")},

		// 表②：删除与提权（反例是"名字里带 rm/sudo 但不是它"的真实命令）
		"never.rm":    {hit: cmd("rm -rf node_modules"), miss: cmd("git rm --cached core/internal/policy/policy.go")},
		"never.rmdir": {hit: cmd("rmdir ./tmp-empty-dir"), miss: cmd("mkdir ./tmp-new-dir")},
		"never.sudo":  {hit: cmd("sudo rm -rf /tmp/zerg-x"), miss: cmd("which sudo")},

		// 表②：破坏性云命令
		"never.cloud.terraform_destroy": {hit: cmd("terraform destroy -auto-approve"), miss: cmd("terraform plan")},

		// 表②：dotenv / 密钥（反例 .env.example 是仓内真实存在的非密钥样例文件）
		"never.secret.dotenv_read":  {hit: pathArg(".env"), miss: pathArg("./.env.example")},
		"never.secret.dotenv_write": {hit: pathArg("./.env"), miss: pathArg("./.env.example")},
		"never.secret.token_read":   {hit: pathArg("~/.zerg/token"), miss: pathArg("~/.zerg/state/session.json")},
		"never.secret.ssh_read":     {hit: pathArg("~/.ssh/id_ed25519"), miss: pathArg("~/.config/git/config")},

		// 表②：配置文件（写 = 改 agent 自己的行为面）
		"never.cfg.git_config_read":  {hit: pathArg("./.git/config"), miss: pathArg("./.gitignore")},
		"never.cfg.git_config_write": {hit: pathArg(".git/config"), miss: pathArg("./.gitignore")},
		"never.cfg.fleet_write":      {hit: pathArg("./gateway/fleet.yaml"), miss: pathArg("./gateway/README.md")},
	}
	// 「需要用户交互的工具」这一类由 builtinInteractivePrograms 生成 ⇒ 用例同步生成
	// （正例 = 真调该程序；反例 = 同名前缀出现在命令**中段**，证明前缀是锚在行首的）。
	for _, p := range builtinInteractivePrograms {
		m["any.interactive."+p] = builtinCase{hit: cmd(p + " /tmp/x.txt"), miss: cmd("echo " + p)}
	}
	return m
}

// ── (a) 任何档位下都不自动批 ────────────────────────────────────────────────

func TestBuiltinFloor_NeverAllowInAnyMode(t *testing.T) {
	entries := BuiltinFloorTable()
	full := fullFloorEngine(t)
	modes := []Mode{ModeDefault, ModeBypass, ModeDontAsk}

	for _, e := range entries {
		for _, mode := range modes {
			c, ok := builtinCases()[e.ID]
			if !ok {
				t.Fatalf("清单条目 %q 没有用例（新增条目必须同时补正例/反例）", e.ID)
			}
			// 单条引擎：断言"这条规则本身"在任何档位都不放行
			one := floorEngine(t, e.ID)
			got := one.Decide(Request{Tool: e.Tool, Args: c.hit, Mode: mode})
			neverAllow(t, e.ID+"（单条，档位="+string(mode)+"）", got)
			if got.MatchedRule == nil || got.MatchedRule.ID != e.ID {
				t.Fatalf("%s 正例应命中本规则，实际 matched=%v", e.ID, got.MatchedRule)
			}
			wantDec, wantReason := e.Effect, ReasonFloorAsk
			if e.Effect == EffectDeny {
				wantReason = ReasonFloorDeny
			}
			if mode == ModeDontAsk && e.Effect == EffectAsk {
				wantDec, wantReason = EffectDeny, ReasonDontAskDeny // 无人值守档：凡 ask ⇒ deny
			}
			if got.Decision != wantDec || got.Reason != wantReason {
				t.Errorf("%s 档位=%s：got (%s,%s)，期望 (%s,%s)｜%s",
					e.ID, mode, got.Decision, got.Reason, wantDec, wantReason, got.Note)
			}
			// 全量地板（接线后的真实形态）：仍然不为 allow
			fv := full.Decide(Request{Tool: e.Tool, Args: c.hit, Mode: mode})
			neverAllow(t, e.ID+"（全量地板，档位="+string(mode)+"）", fv)
			if e.Effect == EffectDeny && fv.Decision != EffectDeny {
				t.Errorf("%s 是 deny 条目，全量地板下也必须 deny，实际 %s（%s）", e.ID, fv.Decision, fv.Reason)
			}
			if fv.Reason != ReasonFloorAsk && fv.Reason != ReasonFloorDeny && fv.Reason != ReasonDontAskDeny {
				t.Errorf("%s 全量地板下应由地板判（floor_ask/floor_deny/dontask_deny），实际 reason=%s", e.ID, fv.Reason)
			}
		}
	}
	t.Logf("清单条目 %d 条 × %d 个档位：均为 deny/ask，**无一条 allow**", len(entries), len(modes))
}

// ── (b) 覆盖度：每条的正例与反例 ────────────────────────────────────────────

func TestBuiltinFloor_Coverage(t *testing.T) {
	entries := BuiltinFloorTable()
	cases := builtinCases()

	// 表里不能有多余的用例（防止规则删了用例留着，让人以为还盖着）
	byID := map[string]bool{}
	for _, e := range entries {
		byID[e.ID] = true
	}
	for id := range cases {
		if !byID[id] {
			t.Errorf("用例 %q 在清单里没有对应条目（条目删了用例要一起删）", id)
		}
	}

	for _, e := range entries {
		c := cases[e.ID]
		one := floorEngine(t, e.ID)

		// 正例：本规则命中，效果与原因码都对
		hit := one.Decide(Request{Tool: e.Tool, Args: c.hit})
		if hit.MatchedRule == nil || hit.MatchedRule.ID != e.ID {
			t.Errorf("%s 正例未命中本规则：decision=%s reason=%s matched=%v", e.ID, hit.Decision, hit.Reason, hit.MatchedRule)
			continue
		}
		if hit.Decision != e.Effect {
			t.Errorf("%s 正例效果应为 %s，实际 %s", e.ID, e.Effect, hit.Decision)
		}

		// 反例①（单条引擎 + 全放档）：**不得被本规则误伤** —— 必须回到"无规则命中 ⇒ allow"
		miss := one.Decide(Request{Tool: e.Tool, Args: c.miss, Mode: ModeBypass})
		if miss.Decision != EffectAllow || miss.Reason != ReasonBypassAllow || miss.MatchedRule != nil {
			t.Errorf("%s 反例被误伤：全放档下应 allow/bypass_allow/无命中，实际 decision=%s reason=%s matched=%v",
				e.ID, miss.Decision, miss.Reason, miss.MatchedRule)
		}
		// 反例②（单条引擎 + 默认档）：是**默认档在问**，不是本规则在拦
		miss2 := one.Decide(Request{Tool: e.Tool, Args: c.miss})
		if miss2.Decision != EffectAsk || miss2.Reason != ReasonDefaultAsk || miss2.MatchedRule != nil {
			t.Errorf("%s 反例在默认档应走 default_ask/无命中，实际 decision=%s reason=%s matched=%v",
				e.ID, miss2.Decision, miss2.Reason, miss2.MatchedRule)
		}
	}
	t.Logf("覆盖度：%d 条条目，各 1 正例 + 2 反例断言", len(entries))
}

// TestBuiltinFloor_GlobalAllowList — 防「一律拦」的假绿：全量地板 + 最松档位（bypass）下，
// 一批**正常作业**必须照旧放行（无命中 ⇒ bypass_allow）。这几条同时是任务点名要的反例
// （普通 git status / 普通 npm test 不得被拒）。
func TestBuiltinFloor_GlobalAllowList(t *testing.T) {
	full := fullFloorEngine(t)
	type allowCase struct {
		name string
		req  Request
	}
	cases := []allowCase{
		{"普通 git status", Request{Tool: "bash", Args: cmd("git status")}},
		{"普通 git diff", Request{Tool: "bash", Args: cmd("git diff --stat")}},
		{"普通 git log", Request{Tool: "bash", Args: cmd("git log --oneline -5")}},
		{"普通 git commit", Request{Tool: "bash", Args: cmd("git commit -m msg")}},
		{"普通 npm test", Request{Tool: "bash", Args: cmd("npm test")}},
		{"普通 npm run build", Request{Tool: "bash", Args: cmd("npm run build")}},
		{"普通 go build", Request{Tool: "bash", Args: cmd("go build ./...")}},
		{"普通 go test", Request{Tool: "bash", Args: cmd("go test ./... -count=1")}},
		{"普通 ls", Request{Tool: "bash", Args: cmd("ls -la")}},
		{"普通 grep", Request{Tool: "bash", Args: cmd("grep -n policy core/internal/policy/policy.go")}},
		{"普通 rmdir 无关命令（mkdir）", Request{Tool: "bash", Args: cmd("mkdir -p /tmp/zerg-x")}},
		{"读工作区内源码", Request{Tool: "read", Args: pathArg("./core/internal/policy/builtin_rules.go")}},
		{"读 .env.example（非密钥样例）", Request{Tool: "read", Args: pathArg("./.env.example")}},
		{"写工作区内源码", Request{Tool: "write", Args: pathArg("./core/internal/policy/builtin_rules.go")}},
		{"写 gateway 下的普通文档", Request{Tool: "write", Args: pathArg("./gateway/README.md")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := c.req
			req.Mode = ModeBypass
			check(t, full.Decide(req), EffectAllow, ReasonBypassAllow, "")
			req.Mode = ModeDefault
			check(t, full.Decide(req), EffectAsk, ReasonDefaultAsk, "")
		})
	}
}

// TestBuiltinFloor_BareFormHits — 无空格前缀（`sudo*` / `npm install*` / `git push*` /
// `terraform destroy*`）的**裸调用**也必须命中：裸 `npm install` 同样会改依赖树，
// 只拦带参数的写法等于留一个少打一个字的绕过口。
func TestBuiltinFloor_BareFormHits(t *testing.T) {
	cases := []struct {
		id  string
		raw string
	}{
		{"never.pkg.npm_install", "npm install"},
		{"never.pkg.npm_ci", "npm ci"},
		{"never.pkg.brew_install", "brew install"},
		{"never.pkg.go_get", "go get"},
		{"never.git.push", "git push"},
		{"never.git.reset_hard", "git reset --hard"},
		{"never.cloud.terraform_destroy", "terraform destroy"},
		{"never.sudo", "sudo -v"},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			one := floorEngine(t, c.id)
			v := one.Decide(Request{Tool: "bash", Args: cmd(c.raw), Mode: ModeBypass})
			if v.MatchedRule == nil || v.MatchedRule.ID != c.id {
				t.Fatalf("%q 应命中 %s，实际 matched=%v decision=%s reason=%s", c.raw, c.id, v.MatchedRule, v.Decision, v.Reason)
			}
			neverAllow(t, c.id+"（裸调用）", v)
		})
	}
}

// ── (c) 清单完整性：bullet 覆盖矩阵 + 无孤儿条目 ─────────────────────────────

type bulletCoverage struct {
	table  BuiltinTable
	bullet string
	ids    []string
}

func builtinBulletCoverage() []bulletCoverage {
	interactive := []string{"pending.any.interactive_tools"}
	for _, p := range builtinInteractivePrograms {
		interactive = append(interactive, "any.interactive."+p)
	}
	return []bulletCoverage{
		// 表①「任何模式都不自动批」的五条
		{TableAnyModeNoAuto, "显式 ask 规则不被档位放松", []string{"pending.any.explicit_ask"}},
		{TableAnyModeNoAuto, "组织/系统级 ask", []string{"pending.any.org_system_ask"}},
		{TableAnyModeNoAuto, "需要用户交互的工具", interactive},
		{TableAnyModeNoAuto, "critical path 的 rm/rmdir", []string{
			"any.critical.rm_home", "any.critical.rm_home_env", "any.critical.rm_home_braces",
			"pending.any.rm_critical_full",
		}},
		{TableAnyModeNoAuto, "工作目录外的读取", []string{"pending.any.read_outside_workdir"}},

		// 表②「永不自动批」的五条
		{TableNeverAutoApprove, "包安装", []string{
			"never.pkg.npm_install", "never.pkg.npm_i", "never.pkg.npm_ci", "never.pkg.yarn_add",
			"never.pkg.pnpm_add", "never.pkg.pip_install", "never.pkg.pip3_install",
			"never.pkg.brew_install", "never.pkg.go_get", "never.pkg.go_install", "never.pkg.cargo_install",
		}},
		{TableNeverAutoApprove, "变更型 git", []string{
			"never.git.reset_hard", "never.git.checkout_paths", "never.git.clean_force", "never.git.push",
		}},
		{TableNeverAutoApprove, "rm / sudo", []string{"never.rm", "never.rmdir", "never.sudo"}},
		{TableNeverAutoApprove, "破坏性云命令", []string{
			"never.cloud.terraform_destroy", "pending.never.cloud_delete_class",
		}},
		{TableNeverAutoApprove, "dotenv / 密钥 / git config / agent 自身配置", []string{
			"never.secret.dotenv_read", "never.secret.dotenv_write", "never.secret.token_read",
			"never.secret.ssh_read", "never.cfg.git_config_read", "never.cfg.git_config_write",
			"never.cfg.fleet_write",
		}},
	}
}

func TestBuiltinFloor_BulletsCoveredNoOrphans(t *testing.T) {
	known := map[string]BuiltinTable{}
	for _, r := range BuiltinFloorTable() {
		known[r.ID] = r.Table
	}
	for _, p := range BuiltinPendingEntries() {
		known[p.ID] = p.Table
	}

	referenced := map[string]bool{}
	for _, bc := range builtinBulletCoverage() {
		if len(bc.ids) == 0 {
			t.Errorf("bullet「%s」没有任何落点（规则或未实体化记账）", bc.bullet)
		}
		for _, id := range bc.ids {
			tbl, ok := known[id]
			if !ok {
				t.Errorf("bullet「%s」引用了不存在的 id %q", bc.bullet, id)
				continue
			}
			if tbl != bc.table {
				t.Errorf("id %q 属于表 %s，却被挂到表 %s 的 bullet「%s」下", id, tbl, bc.table, bc.bullet)
			}
			referenced[id] = true
		}
	}
	// 反向：不许有孤儿条目（不在两张清单里的规则混进来 = 清单外的隐藏拦截）
	for id := range known {
		if !referenced[id] {
			t.Errorf("条目 %q 没有被任何 bullet 引用（它是从哪条清单要求来的？无出处即孤儿，须删或补出处）", id)
		}
	}
}

// ── 数据自洽 ────────────────────────────────────────────────────────────────

func TestBuiltinFloor_DataShape(t *testing.T) {
	entries := BuiltinFloorTable()
	if len(entries) == 0 {
		t.Fatal("内置地板不能为空")
	}
	seen := map[string]bool{}
	rawSeen := map[string]bool{}
	guardTools := map[string]bool{}
	asks, denies := 0, 0

	for _, e := range entries {
		if e.ID == "" {
			t.Fatalf("条目缺 id：%+v", e)
		}
		prefix := "any."
		if e.Table == TableNeverAutoApprove {
			prefix = "never."
		} else if e.Table != TableAnyModeNoAuto {
			t.Errorf("%s 的表值非法：%q", e.ID, e.Table)
		}
		if !strings.HasPrefix(e.ID, prefix) {
			t.Errorf("%s 的 id 命名空间应与表一致（应为 %s*）", e.ID, prefix)
		}
		if seen[e.ID] {
			t.Errorf("id 重复：%q（id 是审计与豁免的锚点，必须唯一）", e.ID)
		}
		seen[e.ID] = true

		if e.Effect != EffectAsk && e.Effect != EffectDeny {
			t.Errorf("%s 的效果必须是 ask|deny，实际 %q —— 地板里的 allow 不授予权限（T5.1）＝该条静默失效", e.ID, e.Effect)
		}
		if e.Effect == EffectAsk {
			asks++
		} else {
			denies++
		}
		if e.Tool == "" || e.Tool != NormalizeToolName(e.Tool) {
			t.Errorf("%s 的工具名必须非空且已归一化（小写无空白），实际 %q", e.ID, e.Tool)
		}
		if !builtinAllowedTools[e.Tool] {
			t.Errorf("%s 引用了不在 builtinAllowedTools 里的工具名 %q —— 不许凭空造工具名（真名要同时加进那张表）", e.ID, e.Tool)
		}
		guardTools[e.Tool] = true
		if e.Specifier == "" {
			t.Errorf("%s 没有匹配式（整个工具）—— 内置条目必须写明管哪一类调用，否则等于整工具拦", e.ID)
		}
		// 工具与形态必须匹配：给 bash 的条目只能是行首前缀（路径/域名形态在 bash 上取不到 path/url 参数
		// ⇒ "判不了 ⇒ ask"，且会把该工具的**所有**调用一起污染成 ask —— 这正是普通 `git status`
		// 都被拦的那条路）。这是本批踩到过的真缺陷，钉在这里。
		if e.Tool == "bash" {
			if k := classifySpec(e.Specifier); k != SpecPrefix && k != SpecNone {
				t.Errorf("%s 是 bash 条目却用了 %q 形态（specifier=%q）：bash 的参数键是 command/cmd/query，该形态取不到值 ⇒ 判不了并污染整个 bash 判定",
					e.ID, k, e.Specifier)
			}
		}
		if k := classifySpec(e.Specifier); k == SpecUnknown {
			t.Errorf("%s 的匹配式 %q 形态认不出 ⇒ 判定时是「判不了 ⇒ ask」：条目会以**错误的原因**生效（还没接线就写坏的规则）", e.ID, e.Specifier)
		}
		if strings.TrimSpace(e.Note) == "" {
			t.Errorf("%s 缺「为何永不自动批」的说明", e.ID)
		}
		r, ok := BuiltinRuleByID(e.ID)
		if !ok || r.ID != e.ID {
			t.Errorf("BuiltinRuleByID(%q) 取不到自己的条目", e.ID)
		}
	}
	for _, r := range BuiltinFloor() {
		if rawSeen[r.Raw] {
			t.Errorf("规则文本重复：%q（两条同效同式的条目没有意义）", r.Raw)
		}
		rawSeen[r.Raw] = true
	}
	if denies == 0 {
		t.Error("一条 deny 都没有：清单①里「critical path 的删除」应该有不可人批的条目")
	}
	if asks == 0 {
		t.Error("一条 ask 都没有：清单不可能全是不可逆动作")
	}
	if !guardTools["bash"] || !guardTools["read"] || !guardTools["write"] {
		t.Errorf("内置条目应落在 bash/read/write 上（其余清单条目见未实体化记账），实际用到：%v", guardTools)
	}
	t.Logf("内置地板 %d 条（表① %d / 表② %d）｜ask %d · deny %d｜用到工具 %d 个｜未实体化 %d 条",
		len(entries), len(BuiltinTableRules(TableAnyModeNoAuto)), len(BuiltinTableRules(TableNeverAutoApprove)),
		asks, denies, len(guardTools), len(BuiltinPendingEntries()))
}

func TestBuiltinFloor_AssembleEqualsTables(t *testing.T) {
	tbl := BuiltinFloorTable()
	fl := BuiltinFloor()
	if len(fl) != len(tbl) {
		t.Fatalf("BuiltinFloor() 条数 %d ≠ 两表合并 %d", len(fl), len(tbl))
	}
	for i := range fl {
		if fl[i].ID != tbl[i].ID || fl[i].Effect != tbl[i].Effect ||
			fl[i].Tool != tbl[i].Tool || fl[i].Specifier != tbl[i].Specifier {
			t.Fatalf("第 %d 条装配走样：got %+v，期望 %+v", i, fl[i], tbl[i])
		}
		if !fl[i].Floor {
			t.Errorf("%s 装进地板时必须带 Floor 标记", fl[i].ID)
		}
		if fl[i].Index != i+1 {
			t.Errorf("%s 的 Index 应为 %d，实际 %d", fl[i].ID, i+1, fl[i].Index)
		}
		if fl[i].Raw != builtinRaw(tbl[i]) {
			t.Errorf("%s 的 Raw 与数据不一致：%q", fl[i].ID, fl[i].Raw)
		}
	}
	// 引擎侧回显：NewEngine 收到的地板必须带 Floor 标记与 id
	eng := fullFloorEngine(t)
	if n := len(eng.Floor()); n != len(tbl) {
		t.Fatalf("Engine.Floor() 应回显 %d 条，实际 %d", len(tbl), n)
	}
	for _, r := range eng.Floor() {
		if !r.Floor || r.ID == "" {
			t.Errorf("引擎地板回显丢了 Floor/ID：%+v", r)
		}
	}
	// 返回值不共享可变状态：改返回值不得污染下一次调用
	got := BuiltinFloor()
	got[0].Effect = EffectAllow
	got[0].ID = "tampered"
	if again := BuiltinFloor(); again[0].Effect != tbl[0].Effect || again[0].ID != tbl[0].ID {
		t.Fatal("BuiltinFloor() 返回的切片被外部修改后污染了内置数据")
	}
	if _, ok := BuiltinRuleByID("tampered"); ok {
		t.Fatal("内置数据被污染（tampered id 竟然查得到）")
	}
	if _, ok := BuiltinRuleByID("no.such.rule"); ok {
		t.Fatal("未知 id 不该命中")
	}
	if BuiltinTableRules(BuiltinTable("yolo")) != nil {
		t.Fatal("非法表值应返回 nil（不猜表）")
	}
}

// ── 未实体化记账 ────────────────────────────────────────────────────────────

func TestBuiltinPending_Accounted(t *testing.T) {
	pend := BuiltinPendingEntries()
	if len(pend) == 0 {
		t.Fatal("未实体化条款必须记账（宁可不写，也不能沉默地假装覆盖了）")
	}
	ids := map[string]bool{}
	for _, r := range BuiltinFloorTable() {
		ids[r.ID] = true
	}
	for _, p := range pend {
		if !strings.HasPrefix(p.ID, "pending.") {
			t.Errorf("%s 的 id 应以 pending. 开头", p.ID)
		}
		if ids[p.ID] {
			t.Errorf("%s 与规则 id 冲突", p.ID)
		}
		if p.Table != TableAnyModeNoAuto && p.Table != TableNeverAutoApprove {
			t.Errorf("%s 的表值非法：%q", p.ID, p.Table)
		}
		for name, s := range map[string]string{"Claim": p.Claim, "Reason": p.Reason, "Coverage": p.Coverage} {
			if strings.TrimSpace(s) == "" {
				t.Errorf("%s 缺 %s（未实体化必须写明：哪条条款 / 为什么写不成 / 现在靠什么兜住）", p.ID, name)
			}
		}
	}
	t.Logf("未实体化 %d 条：\n", len(pend))
	for _, p := range pend {
		t.Logf("  %s（表 %s）：%s → 原因：%s → 兜底：%s", p.ID, p.Table, p.Claim, p.Reason, p.Coverage)
	}
}

// ── 不变量：地板压过档位与普通规则（含清单①第 1 条的语义落点）───────────────

func TestBuiltinFloor_Invariants(t *testing.T) {
	t.Run("显式 ask 规则在全放档下仍是 ask（清单①第 1 条）", func(t *testing.T) {
		eng := mustLoad(t, "ask bash(git push *)", "")
		check(t, eng.Decide(Request{Tool: "bash", Args: cmd("git push origin main"), Mode: ModeBypass}),
			EffectAsk, ReasonRuleAsk, "ask bash(git push *)")
	})

	t.Run("内置地板压过普通 allow（全放档）", func(t *testing.T) {
		for _, c := range []struct {
			rules string
			tool  string
			args  map[string]any
			id    string
		}{
			{"allow bash(rm -rf node_modules)", "bash", cmd("rm -rf node_modules"), "never.rm"},
			{"allow read(./.env)", "read", pathArg(".env"), "never.secret.dotenv_read"},
			{"allow write(./gateway/fleet.yaml)", "write", pathArg("./gateway/fleet.yaml"), "never.cfg.fleet_write"},
			{"allow bash(npm install express)", "bash", cmd("npm install express"), "never.pkg.npm_install"},
		} {
			rules, _ := ParseRules(c.rules)
			eng := NewEngine(rules, BuiltinFloor())
			v := eng.Decide(Request{Tool: c.tool, Args: c.args, Mode: ModeBypass})
			if v.Decision == EffectAllow {
				t.Errorf("%s 被普通 allow 放行了（reason=%s）—— 地板必须压过普通规则", c.id, v.Reason)
			}
			if v.Reason != ReasonFloorAsk && v.Reason != ReasonFloorDeny {
				t.Errorf("%s 应由地板判（floor_*），实际 %s", c.id, v.Reason)
			}
		}
	})

	t.Run("普通 deny 不改变地板的结论（pin 现状：地板先判且立即返回）", func(t *testing.T) {
		// T5.1 定的优先级是「地板命中即返回」⇒ 地板 ask 会压过普通 deny。
		// 这里只**钉住现状**（改这个语义要连 T5.1 一起改），不是本批要改的事。
		rules, _ := ParseRules("deny bash(rm -rf node_modules)")
		eng := NewEngine(rules, BuiltinFloor())
		v := eng.Decide(Request{Tool: "bash", Args: cmd("rm -rf node_modules")})
		if v.Reason != ReasonFloorAsk || v.MatchedRule == nil || v.MatchedRule.ID != "never.rm" {
			t.Errorf("期望 floor_ask/never.rm，实际 %s matched=%v", v.Reason, v.MatchedRule)
		}
	})

	t.Run("无人值守档：ask 条目转 deny，deny 条目照旧 deny", func(t *testing.T) {
		askEngine := floorEngine(t, "never.sudo")
		v := askEngine.Decide(Request{Tool: "bash", Args: cmd("sudo id"), Mode: ModeDontAsk})
		check(t, v, EffectDeny, ReasonDontAskDeny, "ask bash(sudo*)")
		denyEngine := floorEngine(t, "any.critical.rm_home")
		v2 := denyEngine.Decide(Request{Tool: "bash", Args: cmd("rm -rf ~"), Mode: ModeDontAsk})
		check(t, v2, EffectDeny, ReasonFloorDeny, "deny bash(rm -rf ~)")
	})

	t.Run("取证可用：Verdict 带得上内置 id 与规则文本", func(t *testing.T) {
		eng := floorEngine(t, "never.secret.dotenv_read")
		v := eng.Decide(Request{Tool: "read", Args: pathArg(".env")})
		if v.MatchedRule == nil || v.MatchedRule.ID != "never.secret.dotenv_read" {
			t.Fatalf("命中规则应带内置 id：%+v", v.MatchedRule)
		}
		if v.MatchedRule.Raw != "ask read(./.env)" {
			t.Errorf("规则文本应可读：%q", v.MatchedRule.Raw)
		}
		if !strings.Contains(v.Note, "ask read(./.env)") {
			t.Errorf("Note 应带上规则文本供取证：%q", v.Note)
		}
		b, ok := BuiltinRuleByID(v.MatchedRule.ID)
		if !ok || b.Note == "" {
			t.Fatalf("按 id 应能回查「为何永不自动批」：%+v", b)
		}
	})
}
