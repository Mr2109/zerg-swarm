// cli_net_probe_test.go —— `zerg net probe` 的**真二进制**成对判据（组4 §二.4 `W-41` · 任务单序123）。
//
// 判据栏逐字：`zerg net probe --json` ⇒ **rc=0 出三格**；**取不到 ⇒ rc=8**（不许 rc=0 + 空格）。
//
// 本件把「取到 / 取不到」两头都用**自己造的真源**钉住（不依赖本机今天配没配代理 ——
// 靠本机现况的判据会在别人的机器上假绿）：
//
//	① 环境变量那一处：起一个真 LISTEN ⇒ 三格读得到（退 0）；关掉它 ⇒ `reachable=no`
//	   （那是**一条答案**，不是取不到 ⇒ 三格**照出**、rc=8 「不许当绿」）
//	② 系统代理那一处（`scutil --proxy`）：PATH 里塞一个**假 `scutil`** ⇒ 能造出「直连档」；
//	   `scutil` 不在 PATH ⇒ 造出「系统面读不到」
//	③ 两处都读不到 ⇒ rc=8 且**三格一格都不出**（stdout 0 字节 —— 逐字对着判据栏那句
//	   「不许 rc=0 + 空格」的另一半：这一档连 rc=0 都不许）
package main_test

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// netProbeEnv —— 本件用的环境**从零构造**（不 `os.Environ()`）。
//
// 为什么：本机 shell 里可能就设着 `https_proxy`（那正是本命令要读的东西）⇒ 继承环境
// 等于让判据住在被测对象上。从零起 ⇒ 「env 有没有那枚变量」完全由用例说了算。
func netProbeEnv(extra ...string) []string {
	base := []string{
		"ZERG_REPO=invalid-ignored",
		"HOME=" + os.Getenv("HOME"),
		"PATH=/nonexistent", // 缺省：系统代理那一处**读不到**（除非用例自己给 PATH）
	}
	return append(base, extra...)
}

// netProbeRun 起真二进制跑一条命令（env 由调用方给定 · stdin 空 ⇒ 无 TTY、零提示词）。
func netProbeRun(t *testing.T, bin string, env []string, argv ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, argv...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader("")
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	rc := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("起不了 bin/zerg：%v", err)
		}
	}
	return rc, out.String(), errb.String()
}

// netProbeRow —— 三格 + 包封的 meta。
type netProbeRow struct {
	Schema string `json:"schema"`
	Kind   string `json:"kind"`
	Items  []struct {
		Proxy     string `json:"proxy"`
		Reachable string `json:"reachable"`
		Mode      string `json:"mode"`
	} `json:"items"`
	Meta struct {
		Count int `json:"count"`
	} `json:"meta"`
	Warnings []string `json:"warnings"`
}

func netProbeJSON(t *testing.T, out string) netProbeRow {
	t.Helper()
	var row netProbeRow
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	return row
}

// TestNetProbe_EnvProxyThenReachableAndNot —— 判据栏正面 + 一条成对负控（通 ⟷ 不通）。
//
// 「通」⇒ rc=0 · 三格（proxy 读到真地址 · reachable=yes · mode=proxy）；
// 「不通」⇒ rc=8 **且三格照出**（reachable=no）—— 「答案就是不通」与「讲不出答案」**分档**。
func TestNetProbe_EnvProxyThenReachableAndNot(t *testing.T) {
	bin := zergBinary(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("起临时监听：%v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	want := "127.0.0.1:" + strconv.Itoa(port)
	env := netProbeEnv("https_proxy=http://" + want) // 带 scheme ⇒ 顺带验「收敛成 host:port」

	rc, out, errb := netProbeRun(t, bin, env, "net", "probe", "--json", "proxy,reachable,mode")
	if rc != 0 {
		t.Fatalf("代理在听 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	row := netProbeJSON(t, out)
	if row.Schema != "zerg/v1" || row.Kind != "NetProbe" {
		t.Errorf("包封头不对：schema=%q kind=%q", row.Schema, row.Kind)
	}
	if row.Meta.Count != 1 {
		t.Errorf("items 应当恰一格：count=%d · %q", row.Meta.Count, out)
	}
	if len(row.Items) != 1 {
		t.Fatalf("items 不是一格：%q", out)
	}
	got := row.Items[0]
	if got.Proxy != want {
		t.Errorf("proxy 格：want %q got %q（`http://` 前缀没被收敛成 host:port）", want, got.Proxy)
	}
	if got.Reachable != "yes" || got.Mode != "proxy" {
		t.Errorf("reachable/mode 格：want yes/proxy got %s/%s", got.Reachable, got.Mode)
	}
	if strings.Contains(got.Proxy, "//") {
		t.Errorf("proxy 格里还留着 scheme：%q", got.Proxy)
	}
	t.Logf("通：proxy=%s reachable=%s mode=%s", got.Proxy, got.Reachable, got.Mode)

	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	rc, out, errb = netProbeRun(t, bin, env, "net", "probe", "--json", "proxy,reachable,mode")
	if rc != 8 {
		t.Errorf("代理配着而探不到 ⇒ 退 8（不许当绿），得到 %d · stderr=%s", rc, errb)
	}
	down := netProbeJSON(t, out)
	if len(down.Items) != 1 || down.Items[0].Reachable != "no" {
		t.Errorf("「不通」那一档**三格照出**且 reachable=no：%q", out)
	}
	if down.Items[0].Mode != "proxy" || down.Items[0].Proxy != want {
		t.Errorf("「不通」那一档的另两格不该变形：%+v", down.Items[0])
	}
}

// TestNetProbe_SourceUnreadableNoCells —— 判据栏的**反面**那一半：两处真源都读不到 ⇒ rc=8
// 且**三格一格都不出**（裸跑 stdout 0 字节 —— 「不许 rc=0 + 空格」的对面：这一档连 rc=0 都不许）。
func TestNetProbe_SourceUnreadableNoCells(t *testing.T) {
	bin := zergBinary(t)
	env := netProbeEnv() // PATH=/nonexistent（`scutil` 找不到）+ 一个代理变量都没设

	rc, out, errb := netProbeRun(t, bin, env, "net", "probe")
	if rc != 8 {
		t.Fatalf("取不到 ⇒ 退 8，得到 %d · stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("取不到那一档 stdout 必须 0 字节（三格一格都不出），得到 %q", out)
	}
	if !strings.Contains(errb, "取不到") {
		t.Errorf("stderr 没说清「取不到」：%s", errb)
	}

	// 同一档 + `--json <字段>`：按 §九 M7 出**错误包封**（那是错误面，不是结果面）——
	// 「不给结论」与「给结论」因此不会长成同一个形状。
	rc, out, errb = netProbeRun(t, bin, env, "net", "probe", "--json", "proxy,reachable,mode")
	if rc != 8 {
		t.Fatalf("同档带 --json 也退 8，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, `"kind":"blocked"`) || !strings.Contains(out, "net_source_unreadable") {
		t.Errorf("错误包封里没写清 kind/detail：%q", out)
	}
	if strings.Contains(out, `"proxy"`) {
		t.Errorf("错误面里不该出现三格（它不是结果面）：%q", out)
	}
}

// TestNetProbe_SystemFaceDirectAndBroken —— 系统代理那一处的**三态**（认读 / 明示直连 / 读不到）。
//
// 为什么这一格非有不可：「读不到」与「本机没配代理」是两件事 —— 前者 rc=8、后者 rc=0 且
// proxy 格写 `-`。把两者并成同一个形状，就等于把「不知道」写成「没有」（§九 M7 同族）。
func TestNetProbe_SystemFaceDirectAndBroken(t *testing.T) {
	bin := zergBinary(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "scutil")

	// ① 假 `scutil`：本机系统面说「直连」⇒ rc=0 · proxy=`-` · mode=direct · reachable=n/a（三格非空）
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'HTTPSEnable : 0'\necho 'HTTPEnable : 0'\necho 'SOCKSEnable : 0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := netProbeEnv("PATH=" + dir + ":/usr/bin:/bin")
	rc, out, errb := netProbeRun(t, bin, env, "net", "probe", "--json", "proxy,reachable,mode")
	if rc != 0 {
		t.Fatalf("系统面说直连 ⇒ 退 0（三格读到了，只是答案是「没配」），得到 %d · stderr=%s", rc, errb)
	}
	row := netProbeJSON(t, out)
	if len(row.Items) != 1 {
		t.Fatalf("items 不是一格：%q", out)
	}
	if row.Items[0].Proxy != "-" || row.Items[0].Mode != "direct" || row.Items[0].Reachable != "n/a" {
		t.Errorf("直连档三格：want -/n/a/direct got %+v", row.Items[0])
	}
	if row.Items[0].Proxy == "" || row.Items[0].Reachable == "" || row.Items[0].Mode == "" {
		t.Errorf("空格子**不许出现**（判据逐字）：%+v", row.Items[0])
	}

	// ② 假 `scutil` 带代理：系统面那一处也认（`HTTPSEnable : 1`）⇒ mode=proxy
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	sp := ln.Addr().(*net.TCPAddr).Port
	body := "#!/bin/sh\necho 'HTTPSEnable : 1'\necho 'HTTPSProxy : 127.0.0.1'\necho 'HTTPSPort : " +
		strconv.Itoa(sp) + "'\necho 'HTTPEnable : 0'\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, out, errb = netProbeRun(t, bin, env, "net", "probe", "--json", "proxy,reachable,mode")
	if rc != 0 {
		t.Fatalf("系统面配着且在听 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	row = netProbeJSON(t, out)
	if row.Items[0].Proxy != "127.0.0.1:"+strconv.Itoa(sp) || row.Items[0].Mode != "proxy" || row.Items[0].Reachable != "yes" {
		t.Errorf("系统面那条路没认出来：%+v", row.Items[0])
	}

	// ③ 假 `scutil` 跑不起来（退 1）⇒ 这一处**读不到** ⇒ 与「没配」分档：rc=8
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, out, errb = netProbeRun(t, bin, env, "net", "probe")
	if rc != 8 {
		t.Errorf("系统面读不到 ⇒ 退 8（不许降级成「没配」），得到 %d · stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("读不到那一档 stdout 必须 0 字节：%q", out)
	}
}

// TestNetProbe_UsageFace —— 用法面三条（与矩阵的三条候选格同口令 · 成对：坏 ⇒ 2 / 好 ⇒ 0）。
func TestNetProbe_UsageFace(t *testing.T) {
	bin := zergBinary(t)
	env := netProbeEnv("https_proxy=http://127.0.0.1:1")
	for _, bad := range [][]string{
		{"net", "probe", "--nosuchflag-zz"},
		{"net", "probe", "--schema", "zerg/v9"},
		{"net", "probe", "--json"},
		{"net", "probe", "extra-positional"},
	} {
		rc, out, errb := netProbeRun(t, bin, env, bad...)
		if rc != 2 {
			t.Errorf("%v ⇒ 退 2（用法错），得到 %d · stderr=%s", bad, rc, errb)
		}
		if out != "" {
			t.Errorf("%v ⇒ stdout 必须 0 字节（执行前判），得到 %q", bad, out)
		}
	}
	// 点错字段：也是用法错 2，但**形状与上面四条不同** —— 按 §九 M7 补一个错误包封
	// （那是错误面）。两条**分开钉**：混在一条断言里会把「0 字节」与「有错误面」并成一件事。
	rc, out, errb := netProbeRun(t, bin, env, "net", "probe", "--json", "bogusfield")
	if rc != 2 {
		t.Errorf("点名不存在的字段 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, `"kind":"usage"`) || !strings.Contains(out, `"items":[]`) {
		t.Errorf("点错字段那一档该出**错误包封**（items 空 + error.kind=usage）：%q", out)
	}
	// 字段表**两处同值**（registry 里的字面量 ⟷ `netProbeFields` 那个变量）—— 实测对拍：
	// `--json` 裸给时 stderr 印的「可选字段」就是 registry 那一份（`fieldListOf` 读它），
	// 而 `--json proxy,reachable,mode` 能过则证明**本件与 registry 逐字同值**。
	rc, _, errb = netProbeRun(t, bin, env, "net", "probe", "--json")
	if rc != 2 || !strings.Contains(errb, "可选字段: proxy,reachable,mode") {
		t.Errorf("字段表两处同值被破坏（registry 那一份印出来是）：rc=%d · stderr=%s", rc, errb)
	}

	// `--help` 两态的第一态：命令在册 ⇒ 出用法串、不起命令、退 0。
	rc, out, errb = netProbeRun(t, bin, env, "net", "probe", "--help")
	if rc != 0 {
		t.Fatalf("--help（在册命令）⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "zerg net probe") {
		t.Errorf("--help 里没有用法串：%q", out)
	}
}
