// check_bypass_idioms_test.go —— Go 真搬的**成对负控**定向单测（M2 · 绕闸形态元门）。
//
// ✗ 禁恒绿：本件同时钉住「必然判绿 rc=0」「必然出红明细 rc=1（逐条点名 件:行 + 形态 id + 原文）」
// 「不给结论 rc=2（多档）」「登记转绿」「遍历序按 readdir 原序（✗ 非排序）」「自排除」
// 「`--json`/`--list-forms`/`-h`/未知旗标」「`--self-test` 全过」。
//
// 用例名：TestCheckBypassIdiomsGatePort（定向：go test -run TestCheckBypassIdiomsGatePort ./cmd/zerg）。
//
// ★ 本件正文**零私有面字面量**（家目录 / 用户名 / 内网地址 / 私有项目名 / 仓名一律不写）；
// 夹具一律走**合成串**（`git@example.invalid:o/r.git` · `read_token` · `/srv/demo`）—— 与 py 自检同一套合成值。
// ★ 本件非 `.sh`/`.py` ⇒ **不进本门扫描面**（本门面 = 默认根 `publish/`+`scripts/` 下的 `.sh`/`.py`）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cbiTW —— 造件（自动建父目录）。
func cbiTW(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cbiTRun —— 跑入口，收 (rc, stdout, stderr)。
func cbiTRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkBypassIdiomsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// cbiTReaddir —— 保原序读目录（供遍历序断言取「真相」）。
func cbiTReaddir(t *testing.T, dir string) []string {
	t.Helper()
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ents, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

const (
	cbiTBadL1 = `BASE=$(git ls-remote "git@example.invalid:o/r.git" refs/heads/main 2>/dev/null | cut -f1 || true)`
	cbiTBadL2 = `T=$(read_token 2>/dev/null) || { echo "（无令牌 ⇒ 跳过校验 ✓）"; exit 0; }`
)

func TestCheckBypassIdiomsGatePort(t *testing.T) {
	// ── 正控：等价但已收 rc 的写法（无绕闸形态）⇒ rc=0 ───────────────────────────
	t.Run("正控_无绕闸形态_rc0", func(t *testing.T) {
		root := t.TempDir()
		cbiTW(t, filepath.Join(root, "probe.sh"),
			"# 注释里出现 exit 0 与 2>/dev/null 都不入判\n"+
				`BASE=$(git ls-remote "$R" refs/heads/main); LSRC=$?`+"\n"+
				`if [ "$LSRC" -ne 0 ]; then echo "读不到 ⇒ rc=2" >&2; exit 2; fi`+"\n")
		rc, out, errs := cbiTRun(t, "--root", root)
		if rc != 0 || errs != "" {
			t.Fatalf("正控失败：应 rc=0/stderr 空，得 rc=%d err=%q out=%s", rc, errs, out)
		}
		if !strings.Contains(out, "全绿（rc=0）") || !strings.Contains(out, "整行注释 1 行（不入判）") {
			t.Fatalf("正控失败：正文缺「全绿」/注释计数 out=%s", out)
		}
	})

	// ── 负控①（有牙）：未登记绕闸形态 ⇒ rc=1 且**逐处点名 件:行 + 形态 id + 原文** ─────────
	t.Run("负控_未登记绕闸_rc1_逐处点名", func(t *testing.T) {
		root := t.TempDir()
		cbiTW(t, filepath.Join(root, "probe.sh"), cbiTBadL1+"\n"+cbiTBadL2+"\n")
		rc, out, _ := cbiTRun(t, "--root", root)
		if rc != 1 {
			t.Fatalf("负控失败：应 rc=1，得 rc=%d out=%s", rc, out)
		}
		if !strings.Contains(out, "probe.sh:1  [F01-DEVNULL,F02-OR-TRUE,F07-LSREMOTE-SWALLOW]") {
			t.Fatalf("负控失败：第 1 处未逐字点名件+行号+形态 id out=%s", out)
		}
		if !strings.Contains(out, "probe.sh:2  [F01-DEVNULL,F04-SKIP-EXIT0,F05-MISSING-EXIT0]") {
			t.Fatalf("负控失败：第 2 处未逐字点名件+行号+形态 id out=%s", out)
		}
		if !strings.Contains(out, "未登记的绕闸形态 **2 处**") {
			t.Fatalf("负控失败：未报处数 out=%s", out)
		}
	})

	// ── 退 2 多档：空转 / 登记册缺件 / 未知旗标 ────────────────────────────────
	t.Run("退2_空转_rc2", func(t *testing.T) {
		root := t.TempDir()
		rc, out, errs := cbiTRun(t, "--root", root)
		if rc != 2 || out != "" {
			t.Fatalf("应 rc=2/stdout 空，得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(errs, "空转：扫描面 **0 个** `.sh`/`.py`") || !strings.Contains(errs, "**空转 ≠ 绿**") {
			t.Fatalf("退2_空转 正文不符 err=%q", errs)
		}
	})
	t.Run("退2_登记册缺件_rc2", func(t *testing.T) {
		root := t.TempDir()
		cbiTW(t, filepath.Join(root, "probe.sh"), cbiTBadL1+"\n")
		rc, _, errs := cbiTRun(t, "--root", root, "--registry", filepath.Join(root, "nope.json"))
		if rc != 2 || !strings.Contains(errs, "登记册读不到/不可解析") ||
			!strings.Contains(errs, "[Errno 2] No such file or directory:") {
			t.Fatalf("退2_登记册缺件 不符 rc=%d err=%q", rc, errs)
		}
	})
	t.Run("退2_未知旗标_rc2", func(t *testing.T) {
		rc, out, errs := cbiTRun(t, "--nope")
		if rc != 2 || out != "" {
			t.Fatalf("应 rc=2/stdout 空，得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(errs, "usage: check-bypass-idioms.py [-h] [--root ROOT] [--registry REGISTRY]") ||
			!strings.Contains(errs, "check-bypass-idioms.py: error: unrecognized arguments: --nope") {
			t.Fatalf("退2_未知旗标 正文不符 err=%q", errs)
		}
	})

	// ── 登记转绿（棘轮可减）：同一批绕闸处按**内容锚**登记 ⇒ rc=0 ────────────────────
	t.Run("登记转绿_rc0", func(t *testing.T) {
		root := t.TempDir()
		cbiTW(t, filepath.Join(root, "probe.sh"), cbiTBadL1+"\n"+cbiTBadL2+"\n")
		reg := "[\n" +
			` {"form": "F07-LSREMOTE-SWALLOW", "path": "probe.sh", "anchor": "2>/dev/null | cut -f1 || true",` +
			` "reason": "自检", "source": "自检", "gate": "本件"},` + "\n" +
			` {"form": "F05-MISSING-EXIT0", "path": "probe.sh", "anchor": "（无令牌 ⇒ 跳过校验 ✓）",` +
			` "reason": "自检", "source": "自检", "gate": "本件"}` + "\n]\n"
		regf := filepath.Join(root, "reg.json")
		cbiTW(t, regf, reg)
		rc, out, errs := cbiTRun(t, "--root", root, "--registry", regf)
		if rc != 0 || errs != "" {
			t.Fatalf("登记转绿失败：应 rc=0，得 rc=%d err=%q out=%s", rc, errs, out)
		}
		if !strings.Contains(out, "全绿（rc=0）") {
			t.Fatalf("登记转绿失败：正文缺「全绿」out=%s", out)
		}
	})

	// ── 遍历序：子目录**非字典序创建** ⇒ 明细行序须跟 readdir 原序（✗ 非排序 walk） ─────────
	t.Run("遍历序_readdir原序", func(t *testing.T) {
		root := t.TempDir()
		cbiTW(t, filepath.Join(root, "p1.sh"), "x || true\n")                      // F02
		cbiTW(t, filepath.Join(root, "zdir", "z.sh"), "y || :\n")                  // F03
		cbiTW(t, filepath.Join(root, "adir", "a.sh"), "cmd --force\n")             // F09
		cbiTW(t, filepath.Join(root, "adir", "deep", "d.sh"), "cmd --no-verify\n") // F09
		rc, out, _ := cbiTRun(t, "--root", root)
		if rc != 1 {
			t.Fatalf("遍历序：应 rc=1 得 rc=%d out=%s", rc, out)
		}
		// 根层文件先出（层内按字典序），子目录按 readdir 原序下钻 —— 判定用真 readdir 序。
		rootDirs := cbiTReaddir(t, root)
		var dirSeq []string
		for _, n := range rootDirs {
			if st, err := os.Stat(filepath.Join(root, n)); err == nil && st.IsDir() {
				dirSeq = append(dirSeq, n)
			}
		}
		if len(dirSeq) != 2 {
			t.Fatalf("遍历序夹具异常：dirs=%v", dirSeq)
		}
		posFirst := strings.Index(out, dirSeq[0]+"/")
		posSecond := strings.Index(out, dirSeq[1]+"/")
		if posFirst < 0 || posSecond < 0 || posFirst > posSecond {
			t.Fatalf("遍历序不符 readdir 原序：readdir=%v，输出位置 %d/%d out=%s",
				dirSeq, posFirst, posSecond, out)
		}
		// 子目录内先文件后下钻：adir/a.sh 必在 adir/deep/d.sh 之前。
		if strings.Index(out, "adir/a.sh") > strings.Index(out, "adir/deep/d.sh") {
			t.Fatalf("遍历序：层内文件须先于子目录下钻 out=%s", out)
		}
		if !strings.Contains(out, "adir/deep/d.sh:1  [F09-FORCE-FLAG]") {
			t.Fatalf("遍历序：深一层未点名 out=%s", out)
		}
	})

	// ── 自排除：面内**同名 py** 不入判（py `SELF_NAME` 口径） ────────────────────────
	t.Run("自排除_同名py不入判", func(t *testing.T) {
		root := t.TempDir()
		cbiTW(t, filepath.Join(root, "ok.sh"), "echo hi\n")
		cbiTW(t, filepath.Join(root, "check-bypass-idioms.py"), "x || true\n") // 同名 ⇒ 排除
		rc, out, _ := cbiTRun(t, "--root", root, "--json")
		if rc != 0 {
			t.Fatalf("自排除：应 rc=0 得 rc=%d out=%s", rc, out)
		}
		if !strings.Contains(out, `"files_scanned": 1`) || !strings.Contains(out, `"occurrences": 0`) {
			t.Fatalf("自排除失败：同名 py 竟入判 out=%s", out)
		}
	})

	// ── 机器面 `--json`：键固定序 + 明细结构 ──────────────────────────────────
	t.Run("json_键与明细", func(t *testing.T) {
		root := t.TempDir()
		cbiTW(t, filepath.Join(root, "probe.sh"), cbiTBadL1+"\n")
		rc, out, _ := cbiTRun(t, "--root", root, "--json")
		if rc != 1 {
			t.Fatalf("json：应 rc=1 得 rc=%d out=%s", rc, out)
		}
		if !strings.HasPrefix(out, `{"schema": "zerg/v1", "kind": "BypassIdiomScan"`) {
			t.Fatalf("json：键序不符 out=%s", out)
		}
		if !strings.Contains(out, `"unregistered_items": [{"file": "probe.sh", "line": 1, "forms": ["F01-DEVNULL", "F02-OR-TRUE", "F07-LSREMOTE-SWALLOW"]}]`) {
			t.Fatalf("json：明细结构不符 out=%s", out)
		}
		if !strings.Contains(out, `"registry_ceiling": 555`) || !strings.Contains(out, `"forms": 12`) {
			t.Fatalf("json：常量位不符 out=%s", out)
		}
	})

	// ── `--list-forms` / `-h` / `--self-test` ────────────────────────────────
	t.Run("list_forms_rc0", func(t *testing.T) {
		rc, out, _ := cbiTRun(t, "--list-forms")
		if rc != 0 || !strings.Contains(out, "⑯ 元门 · 绕闸形态登记册") ||
			!strings.Contains(out, "F12-JUDGE-FOLD") || !strings.Contains(out, "[F07-LSREMOTE-SWALLOW] publish/mirror-cn.sh") {
			t.Fatalf("list-forms 不符 rc=%d out=%s", rc, out)
		}
	})
	t.Run("help_rc0", func(t *testing.T) {
		rc, out, _ := cbiTRun(t, "-h")
		if rc != 0 || !strings.Contains(out, "usage: check-bypass-idioms.py [-h] [--root ROOT] [--registry REGISTRY]") ||
			!strings.Contains(out, "optional arguments:") || !strings.Contains(out, "--list-forms") {
			t.Fatalf("help 不符 rc=%d out=%s", rc, out)
		}
	})
	t.Run("selftest_全过_rc0", func(t *testing.T) {
		rc, out, _ := cbiTRun(t, "--self-test")
		if rc != 0 || !strings.Contains(out, "自检：全过 ✓") ||
			!strings.Contains(out, "档④ 登记转绿：同一批绕闸处按内容锚登记 ⇒ rc=0（棘轮可减）") {
			t.Fatalf("self-test 不符 rc=%d out=%s", rc, out)
		}
	})
	// `--selftest` 别名（py 前置串判）与「argparse 之前」语义。
	t.Run("selftest_别名_rc0", func(t *testing.T) {
		rc, out, _ := cbiTRun(t, "--selftest")
		if rc != 0 || !strings.Contains(out, "自检：全过 ✓") {
			t.Fatalf("selftest 别名不符 rc=%d out=%s", rc, out)
		}
	})
}

// ── B 类（读失败静默）回归牙 ────────────────────────────────────────────────────
// 扫描面内一件 .sh 锁成 chmod 000（**仓外夹具 · ✗ 不删仓内件**）⇒ 读失败登记面（cbiReadFails）必响：
// rc=2 + stderr BLOCKED 头 + 逐件点名那份锁死件。把登记/判红删回静默（原 py `except OSError: continue`），
// 本用例必红（改前该件静默不入判 ⇒ rc=0）。同族先例：check_doc_cmds_test.go 的 TestCheckDocCmdsReadFailTeeth。
func TestCheckBypassIdiomsReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 绕过权限位 ⇒ chmod 000 不挡读，本用例无鉴别力")
	}
	root := t.TempDir()
	cbiTW(t, filepath.Join(root, "good.sh"), "echo hi\n")
	locked := filepath.Join(root, "locked.sh")
	cbiTW(t, locked, "x || true\n")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o644)

	rc, out, errb := cbiTRun(t, "--root", root)
	if rc != 2 {
		t.Fatalf("读失败回归牙：锁死件 ⇒ 应 rc=2（登记 + 判红），得 rc=%d err=%q out=%q", rc, errb, out)
	}
	if !strings.Contains(errb, "BLOCKED") || !strings.Contains(errb, "locked.sh") {
		t.Fatalf("读失败回归牙：stderr 须带 BLOCKED 头 + 点名 locked.sh，得 err=%q", errb)
	}

	// 成对反控（✗ 禁恒红）：同构夹具但件可读 ⇒ rc=0 且 stderr 无 BLOCKED。
	ok := t.TempDir()
	cbiTW(t, filepath.Join(ok, "good.sh"), "echo hi\n")
	rcOK, _, errOK := cbiTRun(t, "--root", ok)
	if rcOK != 0 || strings.Contains(errOK, "BLOCKED") {
		t.Fatalf("成对反控失败：好件应 rc=0 且无 BLOCKED，得 rc=%d err=%q", rcOK, errOK)
	}
}
