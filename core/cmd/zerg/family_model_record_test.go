// family_model_record_test.go —— `zerg model record verify|probe|ls`（模型登记记录工具归入命令面 · 批 B6）的判据。
//
// 落点 = `package main`（**内部**）：三个动作都走**外部**入口 `run`（与矩阵同一条 argv 面 ⇒ 判据与
// 真跑同形）；只有「探测目标」那条要一枚**合成端点**（`httptest` 起的 OpenAI 兼容假引擎 —— 与源件
// `core/cmd/zerg-model/main_test.go` 同款），不碰真网络、不碰真引擎。
//
// 全部合成盘面（`t.TempDir()` 当模型目录根）：不碰真 `~/.zerg/models`、不碰 `$ZERG_MODELS_DIR`、
// 不起 `zerg-model` 二进制。
//
// 本件钉住的四格（任务单 §三 B6 的「判据件至少」）：
//
//	① **登记面**：命令树里真有 `model record verify|probe|ls` 三条（旗标逐字 · arity 逐条 · 已开放），
//	   且**撞名的两条既有命令一字不动**（`model verify` = 四方对账 · `model ls` = 机型投影 ——
//	   归入是「只增一格」，✗ 不许挤掉它们）；
//	② **正常路径**：`probe <端点> --store --store-root <tmp>` 真探出记录（退 0）→ `ls --root <tmp>` 列得出来
//	   （退 0 · 逐列在）→ `verify <记录件>` 合格（退 0）→ 同一发 `probe` 再跑一遍**幂等**（退 0 · 正文不重写）·
//	   血缘 `--parent` / `--base` 只照抄不推断 · `--json` 面按字段表出行；
//	③ **用法面**：缺/多位置参数 · 未知旗标 · 三对互斥旗标同给 · `--timeout` 非时长 · `--json` 不给字段 ·
//	   同族旗标名互为前缀/专用字段那两枚被**精确**点名拒（`--base` 给 verify、`--strict` 给 ls）⇒
//	   一律 2 + stdout 零字节；且这批旗标**不泄漏**给别族（真给了仍退 2「未知旗标」）；
//	④ **只读面**：`ls` / `verify` 跑前跑后目录**逐件 sha256 不变**；`probe` 不给 `--out`/`--store` 时
//	   **一个字节都不写盘**（源件缺省只打印）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// modelRecordFakeEngine —— 最小 OpenAI 兼容假端点（与源件 `fakeCLIEngine` 同形；
// 只给 probe 真需要的那两条路，别的路 404 ⇒ 探测器的「不可定位」档也照实走）。
func modelRecordFakeEngine(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"object":"list","data":[{"id":"fake-llama","object":"model"}]}`)
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), `"tools"`) {
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_time","arguments":"{\"tz\":\"UTC\"}"}}]}}]}`)
				return
			}
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"你好"}}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// modelRecordRun —— 走**外部**入口跑一次（与矩阵同一条 argv 面）。
func modelRecordRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// modelRecordDirSig —— 目录的**逐件 sha256 指纹**（只读面的两态对拍；排序后拼成一行）。
func modelRecordDirSig(t *testing.T, dir string) string {
	t.Helper()
	var ents []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		sum := sha256.Sum256(data)
		ents = append(ents, p+"="+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("目录指纹读不了 %s：%v", dir, err)
	}
	sort.Strings(ents)
	return strings.Join(ents, "\n")
}

// modelRecordRecords —— 目录下**递归**找出记录正文（标准 §三 的布局是 `<manifests>/<id>/<版>.json`；
// 兄弟文件 `<版>.trace.json` / `<版>.capabilities.json` 不是记录 ⇒ 按 store.List 的同一口径跳过）。
func modelRecordRecords(t *testing.T, dir string) []string {
	t.Helper()
	var hits []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
			return nil
		}
		if strings.HasSuffix(name, ".trace.json") || strings.HasSuffix(name, ".capabilities.json") {
			return nil
		}
		hits = append(hits, p)
		return nil
	})
	if err != nil {
		t.Fatalf("读不了 %s：%v", dir, err)
	}
	sort.Strings(hits)
	return hits
}

// modelRecordOne —— 目录里唯一一枚记录正文（找不到 / 多于一枚 ⇒ fatal）。
func modelRecordOne(t *testing.T, dir string) string {
	t.Helper()
	hits := modelRecordRecords(t, dir)
	if len(hits) != 1 {
		t.Fatalf("%s 下应有的记录正文件数 = %d（要恰好 1）：%v", dir, len(hits), hits)
	}
	return hits[0]
}

// ① 登记面：三条 path / 旗标逐字 / arity / 已开放；既有两条撞名命令一字不动。
func TestModelRecordCommandRegistered(t *testing.T) {
	type want struct {
		path   string
		flags  []string
		arity  string
		fields []string
	}
	for _, w := range []want{
		{"model record verify", []string{"--strict", "--in-place", "--integrity", "--base-dir", "--json"}, arityAny,
			[]string{"path", "schema", "verdict", "level", "field", "detail", "integrity_status"}},
		{"model record probe", []string{"--out", "--store", "--store-root", "--endpoint", "--engine", "--id", "--timeout", "--parent", "--base", "--json"}, arityAny,
			[]string{"id", "version", "digest", "files", "capabilities", "errors", "warnings", "target", "endpoint", "out", "changed", "verdict"}},
		{"model record ls", []string{"--root", "--manifests", "--json"}, arityNone,
			[]string{"id", "version", "commercial", "files", "capabilities", "errors", "warns", "path", "default_eligible"}},
	} {
		info, ok := CommandInfoOfForTest(w.path)
		if !ok {
			t.Fatalf("命令树里没有 `zerg %s`（批 B6 的核心那一条）", w.path)
		}
		if !strings.HasPrefix(info.Usage, "zerg "+w.path) {
			t.Errorf("`%s` 的用法串没以 `zerg %s` 开头：%q", w.path, w.path, info.Usage)
		}
		for _, f := range w.flags {
			if !strings.Contains(info.Usage, f) {
				t.Errorf("`%s` 的用法串漏了旗标 %s（源件十六枚必须逐字保留）：%q", w.path, f, info.Usage)
			}
		}
		if info.Passthrough {
			t.Errorf("`%s` 不该是透传型（它自己判旗标语义）", w.path)
		}
		if !info.OpenForRun {
			t.Errorf("`%s` 要标成**已开放**（真跑 —— 不是占位动作）", w.path)
		}
		var arity string
		var fields []string
		for _, c := range commands {
			if joinPath(c.path) == w.path {
				arity, fields = c.arity, c.fields
			}
		}
		if arity != w.arity {
			t.Errorf("`%s` 的 arity = %q（要 %q）", w.path, arity, w.arity)
		}
		got := map[string]bool{}
		for _, f := range fields {
			got[f] = true
		}
		for _, f := range w.fields {
			if !got[f] {
				t.Errorf("`%s` 的字段表缺 %q（--json 面点不出来）：%v", w.path, f, fields)
			}
		}
	}

	// ★ 撞名面：`model verify`（四方对账）与 `model ls`（机型投影）一字不动 —— 归入是「只增一格」。
	mv, ok := CommandInfoOfForTest("model verify")
	if !ok {
		t.Fatalf("`zerg model verify`（四方对账）不见了 —— 本批只许新增 `model record …` 三条")
	}
	if !strings.Contains(mv.Usage, "模型@机") {
		t.Errorf("`model verify` 的语义被改了：它必须是四方对账（对象 = 模型@机），现读 %q", mv.Usage)
	}
	ml, ok := CommandInfoOfForTest("model ls")
	if !ok {
		t.Fatalf("`zerg model ls`（机型投影）不见了 —— 本批只许新增 `model record …` 三条")
	}
	if strings.Contains(ml.Usage, "--store") || strings.Contains(ml.Usage, "--manifests") {
		t.Errorf("`model ls` 被本批改动了（它只投影主控 /api/fleet/models）：%q", ml.Usage)
	}
}

// ② 正常路径：probe --store → ls → verify → 幂等重跑 → 血缘照抄 → --json 面。
func TestModelRecordNormalPath(t *testing.T) {
	srv := modelRecordFakeEngine(t)
	root := t.TempDir()
	manifests := filepath.Join(root, "manifests")

	// ① 探测并入库（合成端点 · 不碰真网络）。
	rc, _, errb := modelRecordRun(t, "model", "record", "probe", srv.URL, "--store", "--store-root", root)
	if rc != exitOK {
		t.Fatalf("`model record probe --store` rc=%d（要 0）\nstderr:\n%s", rc, errb)
	}
	recPath := modelRecordOne(t, manifests)

	// ② 列目录：表头 + 那一条 + 汇总行（commercial 红线那一句也在）。
	rc, out, errb := modelRecordRun(t, "model", "record", "ls", "--root", root)
	if rc != exitOK {
		t.Fatalf("`model record ls` rc=%d（要 0）\nstderr:\n%s", rc, errb)
	}
	for _, want := range []string{"模型目录（manifests）：", "ID", "VERSION", "COMMERCIAL", "共 1 条记录", "不得作为默认项"} {
		if !strings.Contains(out, want) {
			t.Errorf("`model record ls` 人面缺 %q：\n%s", want, out)
		}
	}

	// ③ 校验记录：合格（退 0）+ 那句诚实边界。
	rc, out, errb = modelRecordRun(t, "model", "record", "verify", recPath)
	if rc != exitOK {
		t.Fatalf("`model record verify` rc=%d（要 0）\nstdout:\n%s\nstderr:\n%s", rc, out, errb)
	}
	// 合成端点探出的记录**没有来源链接**（离线探测）⇒ 现读是 2 条 warn、0 条 error ⇒ 退 0（非 strict）。
	if !strings.Contains(out, "error 0 条") {
		t.Errorf("`model record verify` 人面缺「error 0 条」汇总行：\n%s", out)
	}

	// ④ 幂等重跑：同内容不重写（退 0 · **记录正文**逐字节相同；留痕 `<版>.trace.json` 允许随探测刷新）。
	before, rerr0 := os.ReadFile(recPath)
	if rerr0 != nil {
		t.Fatalf("读不了记录正文：%v", rerr0)
	}
	rc, _, errb = modelRecordRun(t, "model", "record", "probe", srv.URL, "--store", "--store-root", root)
	if rc != exitOK {
		t.Fatalf("第二次 `model record probe --store` rc=%d（要 0 —— 幂等 changed=false）\nstderr:\n%s", rc, errb)
	}
	after, rerr1 := os.ReadFile(recPath)
	if rerr1 != nil {
		t.Fatalf("第二次跑完记录正文不见了：%v", rerr1)
	}
	if string(before) != string(after) {
		t.Fatalf("第二次 probe 改写了记录正文（正文必须逐字节确定）：\n before=%s\n after =%s", before, after)
	}
	if n := len(modelRecordRecords(t, manifests)); n != 1 {
		t.Fatalf("第二次 probe 多落了一枚记录正文（应有 1 枚）：%d", n)
	}

	// ⑤ `--json` 面：按登记字段表出行（差异 ④），信封形状 + 至少 id/verdict 两格在。
	rc, out, errb = modelRecordRun(t, "model", "record", "probe", srv.URL, "--json", "id,verdict,files")
	if rc != exitOK {
		t.Fatalf("`model record probe --json id,verdict,files` rc=%d（要 0）\nstderr:\n%s", rc, errb)
	}
	if !strings.Contains(out, `"kind"`) || !strings.Contains(out, `"verdict"`) || strings.Contains(out, `"digest"`) {
		t.Errorf("`--json` 面不是「登记字段表的行 + 标准包封」：\n%s", out)
	}

	// ⑥ 血缘只照抄：`--parent` / `--base` 给了就进记录，不给就空（✗ 不推断）。
	outDir := t.TempDir()
	linPath := filepath.Join(outDir, "rec.json")
	rc, _, errb = modelRecordRun(t, "model", "record", "probe", srv.URL, "--out", linPath,
		"--id", "fake-lineage", "--parent", "sha256-0123456789ab", "--base", "qwen-base")
	if rc != exitOK {
		t.Fatalf("`model record probe --out … --parent … --base …` rc=%d（要 0）\nstderr:\n%s", rc, errb)
	}
	body, rerr := os.ReadFile(linPath)
	if rerr != nil {
		t.Fatalf("--out 指定的件不在：%v", rerr)
	}
	for _, want := range []string{`"parent": "sha256-0123456789ab"`, `"base_model": "qwen-base"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("血缘没照抄（要 %s）：\n%s", want, body)
		}
	}
	// `--out` 的两枚兄弟文件（留痕 / 能力快照）也在。
	if _, err := os.Stat(filepath.Join(outDir, "rec.trace.json")); err != nil {
		t.Errorf("`--out` 没写留痕兄弟文件：%v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "rec.capabilities.json")); err != nil {
		t.Errorf("`--out` 没写能力快照兄弟文件：%v", err)
	}
}

// ③ 用法面：一律 2 + stdout 零字节；旗标不泄漏给别族。
func TestModelRecordUsageFace(t *testing.T) {
	dir := t.TempDir()
	rec := filepath.Join(dir, "r.json")
	if err := os.WriteFile(rec, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		argv []string
	}{
		{"verify 缺位置参数", []string{"model", "record", "verify"}},
		{"verify 多位置参数", []string{"model", "record", "verify", rec, rec}},
		{"verify 未知旗标", []string{"model", "record", "verify", rec, "--nosuchflag-zz"}},
		{"verify --base-dir 裸给", []string{"model", "record", "verify", rec, "--base-dir"}},
		{"verify --base-dir 不是目录", []string{"model", "record", "verify", rec, "--base-dir", filepath.Join(dir, "nope")}},
		{"verify --in-place 与 --integrity 同给", []string{"model", "record", "verify", rec, "--in-place", "--integrity"}},
		{"verify --in-place 与 --base-dir 同给", []string{"model", "record", "verify", rec, "--in-place", "--base-dir", dir}},
		{"verify --json 不给字段（K2 甲档）", []string{"model", "record", "verify", rec, "--json"}},
		{"verify 不收 --base（同族前缀名 · 精确点名）", []string{"model", "record", "verify", rec, "--base", "x"}},
		{"probe 缺位置参数", []string{"model", "record", "probe"}},
		{"probe 多位置参数", []string{"model", "record", "probe", "a", "b"}},
		{"probe 未知旗标", []string{"model", "record", "probe", "a", "--nosuchflag-zz"}},
		{"probe --out 与 --store 同给", []string{"model", "record", "probe", "a", "--out", "x.json", "--store", "--store-root", dir}},
		{"probe --timeout 非时长", []string{"model", "record", "probe", "a", "--timeout", "十秒"}},
		{"probe --endpoint 裸给", []string{"model", "record", "probe", "a", "--endpoint"}},
		{"probe --store-root 缺 --store", []string{"model", "record", "probe", "a", "--store-root", dir}},
		{"probe --json 不给字段（K2 甲档）", []string{"model", "record", "probe", "a", "--json"}},
		{"ls 不收位置参数", []string{"model", "record", "ls", "x"}},
		{"ls --root 与 --manifests 同给", []string{"model", "record", "ls", "--root", dir, "--manifests", dir}},
		{"ls 未知旗标", []string{"model", "record", "ls", "--nosuchflag-zz"}},
		{"ls 不收 --strict（专用字段 · 精确点名）", []string{"model", "record", "ls", "--strict"}},
		{"不认的契约主号", []string{"model", "record", "ls", "--schema", "zerg/v9"}},
	}
	for _, c := range cases {
		rc, out, errb := modelRecordRun(t, c.argv...)
		if rc != exitUsage {
			t.Errorf("[%s] rc=%d（要 %d）· argv=%v\nstderr:\n%s", c.name, rc, exitUsage, c.argv, errb)
		}
		if out != "" {
			t.Errorf("[%s] stdout 不是零字节（用法错不许有输出）：%q", c.name, out)
		}
		if errb == "" {
			t.Errorf("[%s] stderr 一个字都没有（用法错要给判词）", c.name)
		}
	}

	// 旗标不泄漏：本批上户口的旗标给别族仍退 2「未知旗标」（硬约束⑤：既有命令的退码一字不动）。
	for _, argv := range [][]string{
		{"model", "ls", "--integrity"},
		{"model", "verify", "x@y", "--store"},
		{"version", "--store-root", "d"},
		{"gate", "ls", "--manifests", "d"},
	} {
		rc, out, errb := modelRecordRun(t, argv...)
		if rc != exitUsage || out != "" {
			t.Errorf("旗标泄漏：argv=%v rc=%d stdout=%q（要 2 / 零字节）\nstderr:%s", argv, rc, out, errb)
		}
	}
}

// ④ 只读面：`ls` / `verify` 跑前跑后逐件 sha256 不变；`probe` 不给落点 ⇒ 零写盘。
func TestModelRecordReadOnly(t *testing.T) {
	srv := modelRecordFakeEngine(t)
	root := t.TempDir()
	if rc, _, errb := modelRecordRun(t, "model", "record", "probe", srv.URL, "--store", "--store-root", root); rc != exitOK {
		t.Fatalf("铺夹具：probe --store rc=%d\nstderr:%s", rc, errb)
	}
	manifests := filepath.Join(root, "manifests")
	rec := modelRecordOne(t, manifests)

	before := modelRecordDirSig(t, root)
	// 两态：非 strict 合格（warn 不算不通过）· strict 把 warn 也算不通过 —— 源件同一条口径
	// （`strict && len(findings) > 0` ⇒ 判红）。两态都**不写盘**。
	for _, c := range []struct {
		argv []string
		want int
	}{
		{[]string{"model", "record", "ls", "--root", root}, exitOK},
		{[]string{"model", "record", "ls", "--manifests", manifests}, exitOK},
		{[]string{"model", "record", "verify", rec}, exitOK},
		{[]string{"model", "record", "verify", rec, "--strict"}, exitFail},
		{[]string{"model", "record", "verify", rec, "--in-place"}, exitOK},
		{[]string{"model", "record", "verify", rec, "--integrity"}, exitOK},
	} {
		if rc, out, errb := modelRecordRun(t, c.argv...); rc != c.want {
			t.Errorf("只读面 argv=%v rc=%d（要 %d）\nstdout:%s\nstderr:%s", c.argv, rc, c.want, out, errb)
		}
	}
	if after := modelRecordDirSig(t, root); after != before {
		t.Fatalf("只读命令改动了盘面：\n before=%s\n after =%s", before, after)
	}

	// `probe` 不给 `--out` / `--store`：源件缺省只打印 ⇒ 一个字节都不写盘（同一条只读铁律）。
	bare := t.TempDir()
	bareBefore := modelRecordDirSig(t, bare)
	if rc, out, errb := modelRecordRun(t, "model", "record", "probe", srv.URL); rc != exitOK {
		t.Fatalf("`probe` 缺省档 rc=%d（要 0）\nstderr:%s", rc, errb)
	} else if !strings.Contains(out, `"schema"`) {
		t.Errorf("`probe` 缺省档要把记录正文打到 stdout：%q", out)
	}
	if after := modelRecordDirSig(t, bare); after != bareBefore {
		t.Fatalf("`probe` 缺省档写了盘：%s ⇒ %s", bareBefore, after)
	}
}
