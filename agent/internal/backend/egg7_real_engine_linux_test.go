//go:build linux

// egg7_real_engine_linux_test.go —— 卵沙箱第 7 步：**空根里跑真引擎**（真 `llama-server` ＋ rocBLAS
// 的 dlopen 数据目录 · 不碰 84 GB 权重）。
//
// 前一步（第 5/6 步）证到的是「蛋内 libc 的 (dev,inode) 等于展开目录里那件」——那是**探针级**。
// 本件把被测对象换成**真引擎**：`--engine-dir` 收进真 `llama-server` ＋ 按 `DT_NEEDED` 收齐的闭包
// ＋ rocBLAS 的 `.hsaco` 数据目录（不在 `NEEDED` 里、必须显式交进来），权重用 1 MiB 假 GGUF。
//
// 走的是**生产链路**（✗ 不另写一份配方副本）：
//
//	`eggpkg.ExpandEgg` 真展开 ⇒ `hatchSpecFor`（卵声明 → 孵化声明）⇒ `hatch.BuildBwrapArgv` ⇒ 真 `exec`
//
// 判据（缺一不可）：
//
//	① argv 里**没有** `--ro-bind /usr /usr`（蛋模式一条宿主系统路径都不挂）；
//	② 真 exec 的 rc 与 stdout/stderr 逐字取回（✗ 只报「跑起来了」）；
//	③ 空间内 `/proc/<pid>/maps` 里五件的 **(dev,inode)** 逐值等于展开目录里同名件 ——
//	   `libc.so.6` · `libstdc++` · `libggml-base` · **`librocblas`** · **`librocsolver`**
//	   （后两件只有真把 HIP 后端 dlopen 起来才会进 maps）；并列出**宿主同名件**的 (dev,inode) 作对照；
//	④ 负控：把蛋内 `librocblas.so.5` 临时改名 ⇒ 必须 rc≠0 且逐字报缺（证 rocBLAS 真从蛋里来）。
//
// 跑法（edge-node）：
//
//	ZERG_EGG7_EGG=/tmp/egg7verify/real7/out/real7.egg \
//	ZERG_EGG7_ROOT=/tmp/egg7verify/real7/expanded \
//	ZERG_EGG7_WORK=/tmp/egg7verify/real7/work \
//	go test -v -run TestEgg7RealEngineEmptyRoot ./internal/backend/
package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/eggpkg"
	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// egg7Sampler 空间内取样脚本（只用 dash 内建：空根里没有 cat/ls/grep —— 这正是蛋模式的定义）。
//
// 为什么要「后台起引擎 ＋ 环读 maps」：`--unshare-pid` 之后宿主的 `/proc` 看不见空间内的进程，
// maps 只能在**空间内**读；而 `--version` 这类跑得很快，所以必须边跑边读。
//
// `$@` 是要转给引擎的参数（脚本自己占 `$0`）。`( set -- … )` 那几行把「空间里到底能看见什么」
// 一并证出来：`/usr` 缺席、`.hsaco` 目录真能读到、`ROCBLAS_TENSILE_LIBPATH` 真进了进程环境。
const egg7Sampler = `echo "space:SH_OK"
if [ -d /usr ]; then echo "space:usr=PRESENT"; else echo "space:usr=ABSENT"; fi
echo "space:ldpath=$LD_LIBRARY_PATH"
echo "space:tensile=$ROCBLAS_TENSILE_LIBPATH"
( set -- /opt/rocm/lib/rocblas/*/library/*.hsaco; echo "space:hsaco=$#" )
( set -- /engine/lib/librocblas.so*; echo "space:rocblas_link=$#" )
if [ -r /dev/kfd ]; then echo "space:kfd=READABLE"; else echo "space:kfd=NOT_READABLE"; fi
if [ -r /sys/class/kfd/kfd/topology/nodes/1/properties ]; then echo "space:kfdtopo=READABLE"; else echo "space:kfdtopo=NOT_READABLE"; fi
( set -- /dev/dri/*; echo "space:dri=$#" )
/engine/bin/llama-server "$@" > /tmp/eng.out 2>&1 &
p=$!
while [ -d /proc/$p ]; do
  if [ -r /proc/$p/maps ]; then
    exec 9</proc/$p/maps
    while IFS= read -r l <&9; do
      case "$l" in
        *libc.so.6*|*libstdc++*|*libggml-base*|*librocblas*|*librocsolver*) echo "map:$l" ;;
      esac
    done
    exec 9<&-
  fi
done
wait $p
echo "engrc:$?"
exec 8</tmp/eng.out
while IFS= read -r l <&8; do echo "out:$l"; done
exec 8<&-
`

// egg7Targets 五件硬判据（maps 里的名字前缀 → 展开目录里的真件名 ＋ 它在蛋内哪一层）。
//
// ★ `libggml-base` 在 `engine/bin/`（它是引擎构建树的产物、与 `llama-server` 同目录），
// 其余四件在 `engine/lib/`（loader 目录＝闭包落点）。宿主对照也据此分两处：
// 引擎件对 `--engine-dir` 的源目录，系统件对 `/usr/lib/x86_64-linux-gnu`。
var egg7Targets = []struct{ key, base, where string }{
	{"libc.so.6", "libc.so.6", "lib"},
	{"libstdc++", "libstdc++.so.6.0.35", "lib"},
	{"libggml-base", "libggml-base.so.0.22.0", "bin"},
	{"librocblas", "librocblas.so.5.1", "lib"},
	{"librocsolver", "librocsolver.so.0.7", "lib"},
}

// TestEgg7RealEngineEmptyRoot —— 空根里跑真引擎：展开 ＋ 逐件 sha ＋ argv ＋ 真 exec ＋ maps ＋ 负控。
func TestEgg7RealEngineEmptyRoot(t *testing.T) {
	egg := strings.TrimSpace(os.Getenv("ZERG_EGG7_EGG"))
	root := strings.TrimSpace(os.Getenv("ZERG_EGG7_ROOT"))
	if egg == "" || root == "" {
		t.Skip("未给 ZERG_EGG7_EGG / ZERG_EGG7_ROOT ⇒ 跳过（本用例要在带 bwrap 的 Linux 上真空根跑真引擎）")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("本机没有 bwrap ⇒ 跳过：空根只能在真 bwrap 上造")
	}
	work := strings.TrimSpace(os.Getenv("ZERG_EGG7_WORK"))
	if work == "" {
		work = filepath.Join(os.TempDir(), "egg7work")
	}
	t.Setenv(EnvWorkDirRoot, work) // ✗ 不往家目录写（edge-node 上只许动 /tmp 自己的目录）

	// ── ① 展开（ExpandEgg：逐段落盘 ＋ 段内 sha256 复核，不符不 rename）─────────────
	res, err := eggpkg.ExpandEgg(egg, root)
	if err != nil {
		t.Fatalf("ExpandEgg(%s → %s)：%v", egg, root, err)
	}
	t.Logf("EGG7|expand|gen=%d|uuid=%s|segments=%d|files=%d|symlinks=%d|bytes=%d|skipped=%v|table_sha=%s",
		res.Gen, res.UUID, res.Segments, res.Files, res.Symlinks, res.Bytes, res.Skipped, res.TableSHA256)

	// ── ② 逐件 sha 复核（本用例自己重算，✗ 不信托展开器的自报）───────────────────
	ix, err := eggpkg.ReadEggIndex(egg)
	if err != nil {
		t.Fatalf("ReadEggIndex(%s)：%v", egg, err)
	}
	var nFiles, nLinks, bad int
	for _, s := range ix.Entries {
		p := filepath.Join(root, filepath.FromSlash(s.Path))
		if s.IsSymlink() {
			tgt, lerr := os.Readlink(p)
			if lerr != nil || tgt != s.LinkTarget {
				bad++
				t.Errorf("链段 %q：展开后 readlink=%q（%v），段表=%q", s.Path, tgt, lerr, s.LinkTarget)
			}
			nLinks++
			continue
		}
		got, herr := egg7Sha256(p)
		st, serr := os.Stat(p)
		switch {
		case herr != nil:
			bad++
			t.Errorf("段 %q：算不出 sha256（%v）", s.Path, herr)
		case serr != nil:
			bad++
			t.Errorf("段 %q：stat 失败（%v）", s.Path, serr)
		case st.Size() != s.Size:
			bad++
			t.Errorf("段 %q：字节 %d ≠ 段表 %d", s.Path, st.Size(), s.Size)
		case got != s.SHA256:
			bad++
			t.Errorf("段 %q：sha256 %s ≠ 段表 %s", s.Path, egg7Short(got), egg7Short(s.SHA256))
		default:
			nFiles++
		}
	}
	if bad != 0 {
		t.Fatalf("逐件 sha 复核：%d 段与段表不符（⇒ 这枚蛋的展开不可信，后面不必跑）", bad)
	}
	t.Logf("EGG7|sha|segments=%d|files_ok=%d|symlinks_ok=%d|bad=0", len(ix.Entries), nFiles, nLinks)

	// ── ③ 生产映射：卵声明 → 孵化声明 → bwrap argv（蛋模式）────────────────────────
	spec, err := egg7Spec(t, egg, root)
	if err != nil {
		t.Fatalf("hatchSpecFor（生产映射）：%v", err)
	}
	argv, err := hatch.BuildBwrapArgv(spec)
	if err != nil {
		t.Fatalf("BuildBwrapArgv：%v", err)
	}
	hasUsr := egg7HasUsrBind(argv)
	if hasUsr {
		t.Fatalf("蛋模式 argv 里仍有 `--ro-bind /usr /usr`（假自包含）：%v", argv)
	}
	t.Logf("EGG7|argv|elements=%d|has_usr_bind=%v|engine_in_space=%s|egg_root=%s|bin=%s|loader=%s",
		len(argv), hasUsr, spec.EnginePathInSpace, spec.EggRoot, spec.EggBinDir, spec.EggLoaderDir)
	for i, a := range argv {
		t.Logf("EGG7|argv[%02d]=%s", i, a)
	}

	// ── ④ 真 exec：先 fast 档 ────────────────────────────────────────────────────
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"version", []string{"--version"}},
		{"help", []string{"--help"}},
	} {
		s := spec
		s.EngineArgs = tc.args
		a, aerr := hatch.BuildBwrapArgv(s)
		if aerr != nil {
			t.Fatalf("%s：BuildBwrapArgv：%v", tc.name, aerr)
		}
		out, errOut, rc := egg7Run(a)
		t.Logf("EGG7|run|%s|rc=%d|stdout=%q|stderr=%q", tc.name, rc, egg7Clip(out, 500), egg7Clip(errOut, 800))
		if rc != 0 {
			t.Errorf("%s：真引擎在空根里 rc=%d（要 0）stdout=%q stderr=%q", tc.name, rc, egg7Clip(out, 500), egg7Clip(errOut, 800))
		}
	}

	// ── ⑤ 跑到「真把 HIP 后端 dlopen 起来」：maps 里取五件的 (dev,inode) ──────────
	s := spec
	s.EnginePathInSpace = "/bin/sh" // 入口换成蛋里那枚 sh：只有它能在空间内读 /proc/<pid>/maps
	s.EngineArgs = append([]string{"-c", egg7Sampler, "zerg-egg7"}, "--list-devices")
	a, err := hatch.BuildBwrapArgv(s)
	if err != nil {
		t.Fatalf("BuildBwrapArgv（maps 档）：%v", err)
	}
	out, errOut, rc := egg7Run(a)
	t.Logf("EGG7|run|list-devices|rc=%d|stderr=%q", rc, egg7Clip(errOut, 1200))
	for _, ln := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(ln, "space:"):
			t.Logf("EGG7|%s", ln)
		case strings.HasPrefix(ln, "engrc:"):
			t.Logf("EGG7|engine|rc=%s", strings.TrimPrefix(ln, "engrc:"))
		case strings.HasPrefix(ln, "out:"):
			t.Logf("EGG7|engine_out|%s", egg7Clip(strings.TrimPrefix(ln, "out:"), 300))
		}
	}
	if !strings.Contains(out, "space:SH_OK") {
		t.Fatalf("空间内的 /bin/sh 没起来（输出=%q）——蛋模式不挂宿主 /usr，/bin 只能由蛋给", egg7Clip(out, 800))
	}
	if strings.Contains(out, "space:usr=PRESENT") {
		t.Fatalf("空间里竟然有 /usr（宿主系统路径还在）：输出=%q", egg7Clip(out, 800))
	}
	mapRows := egg7ParseMaps(out)

	miss := 0
	t.Logf("EGG7|inode_table|file|space_dev|space_ino|space_path|egg_dev|egg_ino|host_dev|host_ino|verdict")
	for _, tg := range egg7Targets {
		row, ok := mapRows[tg.key]
		if !ok {
			miss++
			t.Errorf("%s 没出现在空间内 maps 里（＝它没被加载 ⇒ 这条判据无从判）", tg.key)
			continue
		}
		eggDir, hostDir := filepath.Join(root, "engine", "lib"), "/usr/lib/x86_64-linux-gnu"
		if tg.where == "bin" {
			eggDir = filepath.Join(root, "engine", "bin")
			hostDir = "/home/<user>/llama.cpp-src/build-hip-flash/bin"
		}
		eggDev, eggIno, eerr := egg7DevIno(filepath.Join(eggDir, tg.base))
		hostDev, hostIno, herr := egg7DevIno(filepath.Join(hostDir, tg.base))
		verdict := "SAME_AS_EGG"
		if eerr != nil || herr != nil {
			verdict = "STAT_ERR"
			miss++
			t.Errorf("%s：stat 出错 egg=%v host=%v", tg.key, eerr, herr)
		} else if row.dev != eggDev || row.ino != eggIno {
			verdict = "NOT_EGG"
			miss++
		} else if row.dev == hostDev && row.ino == hostIno {
			verdict = "SAME_AS_HOST_TOO"
		}
		t.Logf("EGG7|inode_table|%s|%s|%s|%s|%s|%s|%s|%s|%s", tg.key, row.dev, row.ino, row.path, eggDev, eggIno, hostDev, hostIno, verdict)
		if verdict != "SAME_AS_EGG" {
			t.Errorf("%s：空间内 (dev,inode)=(%s,%s) 与展开目录里那件 (%s,%s) 不等 ⇒ **不是同一件**（判定 %s）",
				tg.key, row.dev, row.ino, eggDev, eggIno, verdict)
		}
		if verdict == "SAME_AS_EGG" && hostDev != "" && row.dev == hostDev && row.ino == hostIno {
			t.Errorf("%s：空间内那件与**宿主同名件**同 (dev,inode) ⇒ 说明挂的是宿主那份", tg.key)
		}
	}
	if miss != 0 {
		t.Fatalf("五件硬判据有 %d 件没判成（见上）", miss)
	}

	// ── ⑥ 负控：蛋内 librocblas 一改名，就必须 rc≠0 且逐字报缺 ─────────────────
	negTarget := filepath.Join(root, "engine", "lib", "librocblas.so.5")
	hidden := negTarget + ".negctl"
	if _, serr := os.Lstat(negTarget); serr != nil {
		t.Fatalf("负控前置：蛋内 %s 不在（%v）", negTarget, serr)
	}
	if rerr := os.Rename(negTarget, hidden); rerr != nil {
		t.Fatalf("负控：改名蛋内 librocblas.so.5 失败：%v", rerr)
	}
	sNeg := spec
	sNeg.EngineArgs = []string{"--list-devices"}
	aNeg, nerr := hatch.BuildBwrapArgv(sNeg)
	if nerr != nil {
		_ = os.Rename(hidden, negTarget)
		t.Fatalf("负控：BuildBwrapArgv：%v", nerr)
	}
	nOut, nErrOut, nRC := egg7Run(aNeg)
	if rerr := os.Rename(hidden, negTarget); rerr != nil {
		t.Fatalf("负控复原失败（夹具已被改动，要人工把 %s 改回）：%v", hidden, rerr)
	}
	t.Logf("EGG7|negctl|rc=%d|stderr=%q", nRC, egg7Clip(nErrOut, 800))
	if nRC == 0 {
		t.Fatalf("负控失效：蛋内 librocblas.so.5 已改名，空根里却照样 rc=0（说明它在用宿主/别处那份）stdout=%q", egg7Clip(nOut, 500))
	}
	if !strings.Contains(nErrOut, "librocblas.so.5") {
		t.Errorf("负控 stderr 应逐字点名缺 librocblas.so.5，实得 %q", egg7Clip(nErrOut, 800))
	}
	// 复原后必须还能跑（否则负控的红来自「盒子被我弄坏了」而不是「缺那一件」）
	if _, _, okRC := egg7Run(mustArgv(t, sNeg)); okRC != 0 {
		t.Fatalf("负控复原后同一份 argv 仍 rc=%d ⇒ 复核不通过（残留夹具改动）", okRC)
	}
	t.Logf("EGG7|negctl_restore|rc=0|prev_rc=%d", nRC)
}

// egg7Spec 用**生产的卵声明映射**产孵化声明（✗ 不手搓 spec：两份口径必然漂）。
//
// 声明面只给两样：`egg`（蛋件路径 —— 由 hatchSpecFor 自己展开/复用）＋ 权重文件（蛋里那份 1 MiB 假
// GGUF）。`egg_dir` 指到已展开的落点 ⇒ 复跑走幂等档（`.complete`，一个字节都不写）。
func egg7Spec(t *testing.T, egg, root string) (hatch.Spec, error) {
	t.Helper()
	e := &registry.ModelEntry{
		File:          filepath.Join(root, "model", "fake-1mb.gguf"),
		SchemaVersion: registry.EggSchemaVersionCurrent,
		Cmd:           registry.CmdString("/home/<user>/llama.cpp-src/build-hip-flash/bin/llama-server -m {file} --port {port}"),
		EnvReq:        &registry.EnvReq{Devices: []string{"0"}},
		Custom:        map[string]interface{}{"egg": egg, "egg_dir": root},
	}
	e.SetEggNameForTest("egg7-real")
	spec, err := hatchSpecFor(e, "llama-server", 58123, hatchTestProfile())
	if err != nil {
		return spec, err
	}
	if spec.EggRoot != root {
		return spec, fmt.Errorf("蛋挂载源没指到展开目录：EggRoot=%q 期望 %q", spec.EggRoot, root)
	}
	return spec, nil
}

// egg7HasUsrBind argv 里有没有 `--ro-bind /usr /usr`。
func egg7HasUsrBind(argv []string) bool {
	for i := 0; i+2 < len(argv); i++ {
		if argv[i] == "--ro-bind" && argv[i+1] == "/usr" && argv[i+2] == "/usr" {
			return true
		}
	}
	return false
}

// egg7Run 真 exec 一条 bwrap argv，取回 (stdout, stderr, rc)。
func egg7Run(argv []string) (string, string, int) {
	cmd := exec.Command("bwrap", argv...)
	var o, e strings.Builder
	cmd.Stdout = &o
	cmd.Stderr = &e
	err := cmd.Run()
	rc := 0
	if err != nil {
		rc = -1
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			e.WriteString("\n" + err.Error())
		}
	}
	return o.String(), e.String(), rc
}

func mustArgv(t *testing.T, s hatch.Spec) []string {
	t.Helper()
	a, err := hatch.BuildBwrapArgv(s)
	if err != nil {
		t.Fatalf("BuildBwrapArgv：%v", err)
	}
	return a
}

// egg7MapRow 一条 maps 读数（`<addr> <perms> <off> <dev> <inode> <path>`）。
type egg7MapRow struct {
	dev, ino, path string
}

// egg7ParseMaps 从取样输出里按五件的名字前缀取**第一条**命中（同一件会重复出现，取第一条即可）。
func egg7ParseMaps(out string) map[string]egg7MapRow {
	rows := map[string]egg7MapRow{}
	for _, ln := range strings.Split(out, "\n") {
		if !strings.HasPrefix(ln, "map:") {
			continue
		}
		f := strings.Fields(strings.TrimPrefix(ln, "map:"))
		if len(f) < 6 {
			continue
		}
		row := egg7MapRow{dev: f[3], ino: f[4], path: f[5]}
		for _, tg := range egg7Targets {
			if _, seen := rows[tg.key]; seen {
				continue
			}
			if strings.Contains(row.path, tg.key) {
				rows[tg.key] = row
			}
		}
	}
	return rows
}

// egg7DevIno 宿主侧文件的 (dev,inode)，格式与 `/proc/<pid>/maps` 逐字同（`%02x:%02x` ＋ 十进制）。
func egg7DevIno(p string) (dev, ino string, err error) {
	fi, serr := os.Stat(p)
	if serr != nil {
		return "", "", serr
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", os.ErrInvalid
	}
	raw := uint64(st.Dev)
	major := uint32((raw >> 8) & 0xfff)
	major |= uint32((raw >> 32) &^ 0xfff)
	minor := uint32(raw & 0xff)
	minor |= uint32((raw >> 12) &^ 0xff)
	return fmt.Sprintf("%02x:%02x", major, minor), fmt.Sprintf("%d", st.Ino), nil
}

func egg7Sha256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func egg7Short(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

func egg7Clip(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
