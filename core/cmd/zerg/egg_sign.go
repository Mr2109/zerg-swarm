// egg_sign.go —— 卵安全面（**卵开工 · 卵5** · 2026-10-15）：cosign 子进程**调用形态** ·
// 内层 §11.5 预留块 ＝ **cosign bundle 槽位** · **吊销（signed deny-list）** · **展开区权限模型判据**。
//
// 承接任务单 `Zerg-内部文档/项目文档/v2.5.15/任务单-卵单文件封装-v2-20261015.md` 的 卵5①–④；
// 设计稿出处：`01-设计/设计-茧壁-统一封闭契约与等级自证.md` 的 §9.11③ · §11.1 · §11.5 · §15.8 L24/L27/L28/L29 ·
// §16.4 · §十九 #3 / #12 / #22 / #23 / #25 / #27 / #31。
//
// ---- 本件范围（逐条照任务单 卵5 与已拍结论）------------------------------------------------------------------
//
//	① **调用形态**：组 cosign argv（`version` / `sign-blob` / `verify-blob`）＋ 约定 stdin/stdout；
//	   拄 **CLI 子进程**（✗ 不嵌 cosign 库 —— 拍定 §十九 #12）。
//	② **工具缺失 / 版本不符** ⇒ **退码 2 · 不给结论**（✗ 不得假绿）。
//	③ **bundle 槽位**：`§11.1` 布局末的**预留块** 降为**存放 cosign bundle 的槽位**（拍定 §十九 #3）；
//	   ★ 与卵6 同口径（拍定 §十九 #31）：签名块 v2 ＝ **主号 2** ⇒ 结构变更 ⇒ **旧读端必拒**
//	   （读到槽位里是 bundle ⇒ 拒，✗ 不许当数据段）。
//	④ **吊销**：自造**签名 deny-list**（拍定 §十九 #23）· 孵化前 **fail-closed**；
//	   无网机**降级为人工（带外 / 介质）**，明写（✗ 不假装能自动）。
//	⑤ **展开区权限模型**（拍定 §十九 #25）：属主 ＝ 孵化器 · 权限 **0700** · 父目录对租户不可达 ⇒ 三面可机检。
//
// ---- 本单硬约束（逐条照办 · 违规即越界）--------------------------------------------------------------------
//
//	· ✗ 不装 / ✗ 不下载 cosign · ✗ 不联网 · ✗ **不真签任何东西** —— 本件**只组装命令** ＋ 在**仓外桩件**上真跑。
//	  真签名 / 真验证**不在本件**（靠拍定后的发布链与真 cosign）。
//	· 本件**不接线** `core/cmd/zerg/main.go`（✗ 不注册命令 · ✗ 不改 add_step）；入口 `eggSignSelfTest` 目前**无 CLI 调用点**
//	  （同 `zergEggGateMain` 的 M2 中间态）。
//	· 桩件一律**仓外**（scratch / 测试临时目录运行时生成）· ✗ 不进仓 · ✗ 不写仓内件。
//	· 退码一律走具名常量（✗ 不写裸数字 `return 2`）；`0/1/2` 复用包内既有 `exitOK / exitFail / exitUsage`。
//
// ---- stdin / stdout 约定（逐字；`cosign` 侧的 CLI 契约）----------------------------------------------------
//
//	· `version`：只读 stdout 里 `GitVersion:` 那一行；取不到 / 主版本不符 ⇒ 判「版本不符 ⇒ 不给结论」。
//	· `sign-blob`：blob 走 **argv 路径**（默认）或 **stdin**（`-` 占位）；`--bundle <件>` 落 bundle 到件；
//	  **签名走 stdout**（cosign 默认）——本件把 stdout 字节原样收下（供写入 bundle 槽位）。
//	· `verify-blob`：blob 走 **argv 路径**；**判决只看退码**（rc=0 ⇒ 验过；rc≠0 ⇒ 未过）；
//	  ✗ 不以 stdout 文案当判据（cosign 文案随版本变）。
//
// ---- 出厂声明 ----------------------------------------------------------------------------------------------
//
//	(a) 本件**不扫仓内树、不写仓内件**（自检只落临时目录）⇒ 无自扫描面。
//	(b) 本件**零绝对路径字面量**；桩件路径一律**运行时现取**。
//
// 出厂声明结束。入口 `eggSignSelfTest` 无 CLI 调用点。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ---- 常量 ---------------------------------------------------------------------------------------------------

const (
	// eggSignProg —— cosign 可执行名（PATH 里找；✗ 不写死绝对路径）。
	// ★ 桩件按仓内先例注入：把一枚名为 `cosign` 的仓外壳脚本**前置进 PATH**（同 `check_approve_ci_test.go` 的夹具法）。
	eggSignProg = "cosign"
	// eggSignRCNoConclusion —— 工具缺失 / 版本不符 / 拿不到判决时统一退码：**2**（＝ exitUsage「不给结论」）。
	eggSignRCNoConclusion = exitUsage
	// eggSignRCSpawnFail —— 已过 LookPath 但子进程起不来（shell 惯例 127）。
	eggSignRCSpawnFail = 127
	// eggSignExpectVerMajor —— 本仓验收的 cosign 主版本前缀（不符 ⇒ rc=2 不给结论）。
	eggSignExpectVerMajor = "v2."
	// eggSignVerLinePrefix —— `cosign version` stdout 里版本行的前缀。
	eggSignVerLinePrefix = "GitVersion:"
	// eggSignArgvLinePrefix —— 桩件回显 argv 的行前缀（只用于自检对拍）。
	eggSignArgvLinePrefix = "ARGV:"

	// eggSlotMagicBundle —— bundle 槽位的判据前缀（其后为 cosign bundle 的 JSON 字节）。
	eggSlotMagicBundle = "ZEGGBND1"
	// eggSlotV2Major —— 签名块 v2 的主号（§十九 #31：v2 ＝ 主号 2 · 旧读端必拒）。
	eggSlotV2Major = 2
	// eggSlotCoverageNote —— 覆盖范围契约语义（保持自本仓 · 签名本体拄 cosign）。
	eggSlotCoverageNote = "覆盖范围 = 索引 ＋ 全部段（bundle 入预留块，块本身不在被签范围 —— 自指边界）"

	// eggDenyRCRevoked —— 吊销命中：拒孵（rc=1 · 点名该卵）。
	eggDenyRCRevoked = exitFail
	// eggDenyRCFailClosed —— 清单缺失 / 坏 / 过期 ⇒ fail-closed 拒孵（rc=2 · 不给结论）。
	eggDenyRCFailClosed = exitUsage
	// eggDenyDegradeLine —— 无网 / 过期时**明写**的降级话（✗ 不假装能自动）。
	eggDenyDegradeLine = "降级为人工（带外 / 介质更新）"
	// eggDenyManualEnv —— 带外人工已更新清单的显式授权旋钮（由人设，✗ 不自动假设）。
	eggDenyManualEnv = "ZERG_DENY_MANUAL"

	// eggExpandWantPerm —— 展开区权限（属主 ＝ 孵化器 · 恰 0700）。
	eggExpandWantPerm = 0o700
	// eggExpandParentMaxOther —— 父目录对 group/other 的允许位上限（租户不可下钻）。
	eggExpandParentMaxOther = 0o077
)

// ------------------------------------------------------------------------------------------------------------
// ① cosign 调用形态（argv 拼装 · ✗ 不真签）
// ------------------------------------------------------------------------------------------------------------

// eggSignBlobArgv —— `sign-blob` 的 argv（**blob 走路径** · 离线 keyed 签）。
// 逐字形态：cosign sign-blob --key <子钥> --bundle <bundle 件> --yes <卵>
func eggSignBlobArgv(keyPath, bundlePath, blobPath string) []string {
	return []string{"sign-blob", "--key", keyPath, "--bundle", bundlePath, "--yes", blobPath}
}

// eggSignBlobStdinArgv —— `sign-blob` 的 argv（**blob 走 stdin** · `-` 占位）。
func eggSignBlobStdinArgv(keyPath, bundlePath string) []string {
	return []string{"sign-blob", "--key", keyPath, "--bundle", bundlePath, "--yes", "-"}
}

// eggSignVerifyBlobArgv —— `verify-blob` 的 argv（keyed ＋ bundle ⇒ 离线可验）。
// 逐字形态：cosign verify-blob --key <根 / 子钥公钥> --signature <签名件> --bundle <bundle 件> <卵>
func eggSignVerifyBlobArgv(keyPath, sigPath, bundlePath, blobPath string) []string {
	return []string{"verify-blob", "--key", keyPath, "--signature", sigPath, "--bundle", bundlePath, blobPath}
}

// eggSignResult —— 一次 cosign 调用的读数。
type eggSignResult struct {
	Argv       []string // 本件组装的 argv（✗ 不含程序名）
	RC         int      // 子进程退码；或本件判定的退码（见 Conclusion）
	Conclusion bool     // false ⇒ **不给结论**（工具缺失 / 版本不符）
	Why        string   // 不给结论时的理由（逐字进报话）
	Stdout     []byte
	Stderr     []byte
}

// eggSignRunCosign —— 组装并**真跑**一次 cosign（子进程）。
// ★ 工具缺失 / 版本不符 ⇒ RC=2 · Conclusion=false（**不给结论** · ✗ 不得假绿）。
func eggSignRunCosign(stdin []byte, args ...string) eggSignResult {
	res := eggSignResult{Argv: args}
	if _, err := exec.LookPath(eggSignProg); err != nil {
		res.RC = eggSignRCNoConclusion
		res.Conclusion = false
		res.Why = "cosign 工具缺失（PATH 里找不到）⇒ 不给结论（rc=" + strconv.Itoa(eggSignRCNoConclusion) + "）"
		res.Stderr = []byte(res.Why + "\n")
		return res
	}
	if v, ok := eggSignProbeVersion(); !ok {
		res.RC = eggSignRCNoConclusion
		res.Conclusion = false
		res.Why = "cosign 版本不符（读到 " + v + "，期望主版本 " + eggSignExpectVerMajor + "）⇒ 不给结论（rc=" + strconv.Itoa(eggSignRCNoConclusion) + "）"
		res.Stderr = []byte(res.Why + "\n")
		return res
	}
	res.Conclusion = true
	cmd := exec.Command("cosign", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	res.Stdout, res.Stderr = out.Bytes(), errb.Bytes()
	if err == nil {
		res.RC = exitOK
		return res
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		res.RC = ee.ExitCode()
		return res
	}
	res.RC = eggSignRCSpawnFail
	res.Why = "cosign 子进程起不来：" + err.Error()
	return res
}

// eggSignProbeVersion —— 跑 `cosign version` 取版本并判主版本。
func eggSignProbeVersion() (string, bool) {
	cmd := exec.Command("cosign", "version")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return eggSignParseVersion(out.String())
}

// eggSignParseVersion —— 从 `cosign version` 的 stdout 里取版本行并判主版本（取不到 ⇒ 不符）。
func eggSignParseVersion(out string) (string, bool) {
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, eggSignVerLinePrefix) {
			v := strings.TrimSpace(strings.TrimPrefix(ln, eggSignVerLinePrefix))
			return v, strings.HasPrefix(v, eggSignExpectVerMajor)
		}
	}
	return "", false
}

// ------------------------------------------------------------------------------------------------------------
// ② 内层 §11.5 预留块 ＝ cosign bundle 槽位
// ------------------------------------------------------------------------------------------------------------

// eggSlotKind —— 预留块的类别。
type eggSlotKind string

const (
	eggSlotEmpty   eggSlotKind = "empty"   // v1 预留（全零 / 空）
	eggSlotBundleK eggSlotKind = "bundle"  // 槽位里放着一枚 cosign bundle（v2 结构）
	eggSlotUnknown eggSlotKind = "unknown" // 非零且非 bundle（本机读不出）
)

// eggSlotClassify —— 判定预留块内容类别。
func eggSlotClassify(reserved []byte) eggSlotKind {
	if len(reserved) == 0 {
		return eggSlotEmpty
	}
	if strings.HasPrefix(string(reserved), eggSlotMagicBundle) {
		return eggSlotBundleK
	}
	for _, b := range reserved {
		if b != 0 {
			return eggSlotUnknown
		}
	}
	return eggSlotEmpty
}

// eggSlotEncodeBundle —— 把一枚 cosign bundle 编进槽位字节（magic ＋ bundle JSON）。
func eggSlotEncodeBundle(bundle []byte) []byte {
	out := make([]byte, 0, len(eggSlotMagicBundle)+len(bundle))
	out = append(out, eggSlotMagicBundle...)
	out = append(out, bundle...)
	return out
}

// eggSlotDecodeBundle —— 从槽位字节里取出 bundle（✗ 非 bundle 槽 ⇒ false）。
func eggSlotDecodeBundle(reserved []byte) ([]byte, bool) {
	if eggSlotClassify(reserved) != eggSlotBundleK {
		return nil, false
	}
	return reserved[len(eggSlotMagicBundle):], true
}

// eggSlotOldReaderVerdict —— **旧读端（主号 1）** 读到该预留块时的判决（返回 rc 与报话）。
// ★ 与卵6 同口径（§十九 #31）：签名块 v2 ＝ **主号 2** ⇒ 结构变更 ⇒ **旧读端必拒**；
// 典型症状（片5 F2）= 旧读端把签名块当**数据段** ⇒ 这里「读到槽位里是 bundle ⇒ 拒」。
func eggSlotOldReaderVerdict(indexMajor int, reserved []byte) (int, string) {
	if indexMajor >= eggSlotV2Major {
		return exitUsage, "拒孵：卵格式 version 高于本机认知（前向拒 · D2）—— 旧读端不按新格式猜着跑；请升级孵化器"
	}
	if eggSlotClassify(reserved) == eggSlotBundleK {
		return exitUsage, "拒孵：预留块里是 cosign bundle（签名块 v2 ＝ 主号 2 · 旧读端必拒）—— 不许当数据段"
	}
	return exitOK, "旧读端放行（预留块为空 / 主号 1）"
}

// eggSlotNewReaderVerdict —— **新读端**：bundle 槽位正常承载（✗ 不报错）。
func eggSlotNewReaderVerdict(reserved []byte) (int, string) {
	switch eggSlotClassify(reserved) {
	case eggSlotBundleK:
		return exitOK, "sig_slot=预留 bundle_slot=cosign bundle"
	case eggSlotEmpty:
		return exitOK, "sig_slot=预留 bundle_slot=(空) 未签名"
	default:
		return exitUsage, "拒孵：预留块内容不认识（非空且非 bundle 槽）"
	}
}

// ------------------------------------------------------------------------------------------------------------
// ③ 吊销：自造签名 deny-list（孵化前 fail-closed · 无网降级为人工）
// ------------------------------------------------------------------------------------------------------------

// eggDenyList —— 签名 deny-list 的形状（随发布链分发 · 照 TUF timestamp/snapshot 过期口径防冻结）。
type eggDenyList struct {
	Version  int      `json:"version"`
	IssuedAt string   `json:"issued_at"`
	NotAfter string   `json:"not_after"` // TUF 式过期线：过 ⇒ 防「冻结旧清单」fail-closed
	Revoked  []string `json:"revoked"`   // 被吊销卵的内容 sha256（小写 hex）
	Sig      string   `json:"sig"`       // 签名槽位（cosign bundle / 本地化 trusted_root；本件不真验）
}

// eggDenyDecision —— 一次孵化前吊销检查的判决。
type eggDenyDecision struct {
	RC      int    // 0 放行 / 1 吊销命中 / 2 fail-closed 不给结论
	Verdict string // "allow" / "revoked" / "fail-closed" / "manual-degraded"
	Line    string // 逐字报话（含点名 / 降级话）
}

// eggDenyCheckBeforeHatch —— 孵化前吊销检查（**fail-closed**）。
//
//	· eggSHA 在 revoked 里 ⇒ 拒（rc=1 · 点名该卵）；
//	· not_after 已过（TUF 防冻结）⇒ 拒 ＋ 逐字出声「降级为人工（带外 / 介质更新）」（✗ 不假装能自动）；
//	  ★ 带外人工已更新清单时由人设 `ZERG_DENY_MANUAL=1` ⇒ 放行但**照旧**出声「降级为人工」；
//	· 清单缺失 / 读不了 / 解析不了 / 过期字段坏 ⇒ fail-closed 拒（✗ 不静默放行）。
func eggDenyCheckBeforeHatch(denyPath, eggSHA string, now time.Time) eggDenyDecision {
	raw, err := os.ReadFile(denyPath)
	if err != nil {
		return eggDenyDecision{RC: eggDenyRCFailClosed, Verdict: "fail-closed",
			Line: "拒孵：吊销清单读不到（fail-closed）—— 不能证明未被吊销 ⇒ 不放行"}
	}
	var dl eggDenyList
	if err := json.Unmarshal(raw, &dl); err != nil {
		return eggDenyDecision{RC: eggDenyRCFailClosed, Verdict: "fail-closed",
			Line: "拒孵：吊销清单不可解析（fail-closed）—— " + err.Error()}
	}
	if strings.TrimSpace(dl.NotAfter) != "" {
		na, perr := time.Parse(time.RFC3339, strings.TrimSpace(dl.NotAfter))
		if perr != nil {
			return eggDenyDecision{RC: eggDenyRCFailClosed, Verdict: "fail-closed",
				Line: "拒孵：吊销清单的 not_after 不可解析（fail-closed）—— " + perr.Error()}
		}
		if now.After(na) {
			if os.Getenv(eggDenyManualEnv) == "1" {
				return eggDenyDecision{RC: exitOK, Verdict: "manual-degraded",
					Line: "吊销清单已过期（超 " + dl.NotAfter + "）—— " + eggDenyDegradeLine + "：人工已带外更新 ⇒ 放行"}
			}
			return eggDenyDecision{RC: eggDenyRCFailClosed, Verdict: "fail-closed",
				Line: "拒孵：吊销清单已过期（超 " + dl.NotAfter + "）—— " + eggDenyDegradeLine + "（无网机只能带外更新）"}
		}
	}
	want := strings.ToLower(strings.TrimSpace(eggSHA))
	for _, r := range dl.Revoked {
		if strings.ToLower(strings.TrimSpace(r)) == want {
			return eggDenyDecision{RC: eggDenyRCRevoked, Verdict: "revoked",
				Line: "拒孵：卵 " + eggSHA + " 在吊销清单里（deny-list hit）—— 孵化前 fail-closed"}
		}
	}
	return eggDenyDecision{RC: exitOK, Verdict: "allow", Line: "放行：吊销清单在册且未命中"}
}

// ------------------------------------------------------------------------------------------------------------
// ④ 展开区权限模型判据（属主 = 孵化器 · 0700 · 父目录对租户不可达）
// ------------------------------------------------------------------------------------------------------------

// eggExpandPermCheck —— 三面可机检：
//
//	① mode 恰 0700 ② 属主 == 孵化器 uid（wantUID）③ 父目录对 group/other 不可达（租户不可下钻）。
//
// 返回 (ok, problems)。ok=false 即判红。
func eggExpandPermCheck(dir string, wantUID uint32) (bool, []string) {
	var problems []string
	fi, err := os.Lstat(dir)
	if err != nil {
		return false, []string{"展开区读不到：" + err.Error()}
	}
	if !fi.IsDir() {
		problems = append(problems, "展开区不是目录")
	}
	if perm := fi.Mode().Perm(); perm != eggExpandWantPerm {
		problems = append(problems, fmt.Sprintf("展开区权限非 0700（实读 %04o）—— 租户可能可达", perm))
	}
	if uid, ok := eggFileOwnerUID(fi); ok {
		if uid != wantUID {
			problems = append(problems, fmt.Sprintf("展开区属主非孵化器 uid（实读 %d · 期望 %d）", uid, wantUID))
		}
	} else {
		problems = append(problems, "展开区属主读不出（本平台不支持）—— 不给结论")
	}
	parent := filepath.Dir(filepath.Clean(dir))
	if pfi, perr := os.Lstat(parent); perr == nil {
		if pperm := pfi.Mode().Perm(); pperm&eggExpandParentMaxOther != 0 {
			problems = append(problems, fmt.Sprintf("父目录对 group/other 可达（实读 %04o）—— 租户可下钻", pperm))
		}
	} else {
		problems = append(problems, "父目录读不到："+perr.Error())
	}
	return len(problems) == 0, problems
}

// eggFileOwnerUID —— 读属主 uid（unix 面走 syscall.Stat_t；本平台读不出 ⇒ false）。
func eggFileOwnerUID(fi os.FileInfo) (uint32, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, false
	}
	return st.Uid, true
}

// ------------------------------------------------------------------------------------------------------------
// 自检（＝ 未接线的 `--self-test` 面 · 只落临时目录 · ✗ 不真签）
// ------------------------------------------------------------------------------------------------------------

// eggSignStubScript —— 仓外假 cosign 桩（自检**运行时生成** · ✗ 不进仓 · ✗ 不真签）。
const eggSignStubScript = `#!/bin/sh
# 仓外假 cosign 桩（自检运行时生成 · ✗ 不进仓 · ✗ 不真签）
printf 'ARGV:'
for a in "$@"; do printf ' %s' "$a"; done
printf '\n'
case "$1" in
  version) printf 'GitVersion:    %s\n' "${FAKE_COSIGN_VERSION:-v2.4.1}" ;;
  verify-blob) [ "${FAKE_COSIGN_VERIFY_FAIL:-0}" = "1" ] && exit 1 ;;
  sign-blob) cat >/dev/null ;;
esac
exit 0
`

// eggSignStubArgvLine —— 从桩件的 stdout 里取它回显的 argv 行。
func eggSignStubArgvLine(stdout []byte) string {
	for _, ln := range strings.Split(string(stdout), "\n") {
		if strings.HasPrefix(ln, eggSignArgvLinePrefix) {
			return ln
		}
	}
	return ""
}

// eggSignSelfTest —— 四态 ＋ 负控全跑（写入 stdout 的读数就是验收面）。
func eggSignSelfTest(stdout, stderr io.Writer) int {
	ok := 0
	root, err := os.MkdirTemp("", "zerg-egg-sign-selftest")
	if err != nil {
		return eggSignSelfFail(stderr, err)
	}
	defer os.RemoveAll(root) // 自回收（只落临时目录）

	binDir := filepath.Join(root, "bin")
	emptyDir := filepath.Join(root, "emptybin")
	for _, d := range []string{binDir, emptyDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return eggSignSelfFail(stderr, err)
		}
	}
	stub := filepath.Join(binDir, "cosign")
	if err := os.WriteFile(stub, []byte(eggSignStubScript), 0o755); err != nil {
		return eggSignSelfFail(stderr, err)
	}
	oldPath, hadPath := os.LookupEnv("PATH")
	// 桩只在 binDir 里；PATH 另带 /bin:/usr/bin（桩件自身用 shell 内建，不需要外命令）。
	setStubPath := func() {
		os.Setenv("PATH", binDir+string(os.PathListSeparator)+"/bin"+string(os.PathListSeparator)+"/usr/bin")
	}
	setNoStubPath := func() { os.Setenv("PATH", emptyDir) }
	_ = setNoStubPath
	defer func() {
		if hadPath {
			os.Setenv("PATH", oldPath)
		} else {
			os.Unsetenv("PATH")
		}
	}()

	// 夹具：一枚"卵" ＋ 子钥 / 根 / bundle / 签名件。
	egg := filepath.Join(root, "demo.egg")
	if err := os.WriteFile(egg, []byte("ZEGG-demo-egg-bytes"), 0o644); err != nil {
		return eggSignSelfFail(stderr, err)
	}
	key := filepath.Join(root, "sub-key.pem")
	if err := os.WriteFile(key, []byte("PRIVKEY-STUB"), 0o600); err != nil {
		return eggSignSelfFail(stderr, err)
	}
	bundle := filepath.Join(root, "demo.bundle")
	sig := filepath.Join(root, "demo.sig")

	// ---------------- ① 桩存在 ⇒ 调用形态正确（逐字 argv）
	setStubPath()
	res := eggSignRunCosign(nil, eggSignVerifyBlobArgv(key, sig, bundle, egg)...)
	if !res.Conclusion || res.RC != exitOK {
		return eggSignSelfFail(stderr, fmt.Errorf("① verify-blob 调用未成立：rc=%d conclusion=%v why=%s", res.RC, res.Conclusion, res.Why))
	}
	want := eggSignArgvLinePrefix + " " + strings.Join(res.Argv, " ")
	got := eggSignStubArgvLine(res.Stdout)
	if got != want {
		return eggSignSelfFail(stderr, fmt.Errorf("① argv 不符：桩见 %q 期望 %q", got, want))
	}
	fmt.Fprintf(stdout, "① 桩在 PATH ⇒ 调用形态正确（桩逐字见 argv）：%s（rc=%d）\n", got, res.RC)
	ok++

	// ①b sign-blob（blob 走 stdin）argv 逐字
	setStubPath()
	res = eggSignRunCosign([]byte("egg-bytes"), eggSignBlobStdinArgv(key, bundle)...)
	want = eggSignArgvLinePrefix + " " + strings.Join(eggSignBlobStdinArgv(key, bundle), " ")
	if got = eggSignStubArgvLine(res.Stdout); got != want || res.RC != exitOK {
		return eggSignSelfFail(stderr, fmt.Errorf("①b sign-blob(stdin) argv 不符：桩见 %q 期望 %q rc=%d", got, want, res.RC))
	}
	fmt.Fprintf(stdout, "①b sign-blob（blob 走 stdin）⇒ %s（rc=%d）\n", got, res.RC)
	ok++

	// ---------------- ② 桩不存在 ⇒ rc=2 且明说「不给结论」（✗ 不假绿）
	os.Setenv("PATH", emptyDir)
	res = eggSignRunCosign(nil, eggSignVerifyBlobArgv(key, sig, bundle, egg)...)
	if res.RC != eggSignRCNoConclusion || res.Conclusion {
		return eggSignSelfFail(stderr, fmt.Errorf("② 工具缺失应 rc=%d 不给结论，实得 rc=%d conclusion=%v", eggSignRCNoConclusion, res.RC, res.Conclusion))
	}
	if !strings.Contains(string(res.Stderr), "不给结论") {
		return eggSignSelfFail(stderr, fmt.Errorf("② 工具缺失未明说「不给结论」：%s", res.Stderr))
	}
	fmt.Fprintf(stdout, "② 桩不在 PATH ⇒ rc=%d · conclusion=%v（明说「不给结论」）：%s", res.RC, res.Conclusion, res.Stderr)
	ok++

	// ---------------- ③ deny-list 命中 ⇒ 孵化前拒（rc=1 且点名）
	hitList := filepath.Join(root, "deny-hit.json")
	eggSHA := "aa11bb22cc33dd44ee55ff6600112233445566778899aabbccddeeff00112233"
	if err := os.WriteFile(hitList, []byte(`{"version":1,"issued_at":"2026-10-01T00:00:00Z","not_after":"2026-12-31T00:00:00Z","revoked":["`+eggSHA+`"],"sig":"stub"}`), 0o644); err != nil {
		return eggSignSelfFail(stderr, err)
	}
	d := eggDenyCheckBeforeHatch(hitList, eggSHA, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if d.RC != eggDenyRCRevoked || d.Verdict != "revoked" || !strings.Contains(d.Line, eggSHA) {
		return eggSignSelfFail(stderr, fmt.Errorf("③ 吊销命中应 rc=%d 且点名，实得 rc=%d verdict=%s line=%s", eggDenyRCRevoked, d.RC, d.Verdict, d.Line))
	}
	fmt.Fprintf(stdout, "③ deny-list 命中（含该卵 sha）⇒ rc=%d · verdict=%s：%s\n", d.RC, d.Verdict, d.Line)
	ok++

	// ---------------- ④ 展开区 mode 被放宽 ⇒ 判据必红
	expRootParent := filepath.Join(root, "expand-parent")
	if err := os.MkdirAll(expRootParent, 0o700); err != nil {
		return eggSignSelfFail(stderr, err)
	}
	if err := os.Chmod(expRootParent, 0o700); err != nil {
		return eggSignSelfFail(stderr, err)
	}
	expRoot := filepath.Join(expRootParent, "expand")
	if err := os.MkdirAll(expRoot, 0o700); err != nil {
		return eggSignSelfFail(stderr, err)
	}
	uid := uint32(os.Getuid())
	if good, probs := eggExpandPermCheck(expRoot, uid); !good {
		return eggSignSelfFail(stderr, fmt.Errorf("④ 正控：0700 展开区不该红：%v", probs))
	}
	if err := os.Chmod(expRoot, 0o755); err != nil {
		return eggSignSelfFail(stderr, err)
	}
	good, probs := eggExpandPermCheck(expRoot, uid)
	if good || len(probs) == 0 {
		return eggSignSelfFail(stderr, fmt.Errorf("④ 展开区 0755 竟判绿"))
	}
	fmt.Fprintf(stdout, "④ 展开区 mode=0755 ⇒ 判红（%d 项）：%s\n", len(probs), strings.Join(probs, "；"))
	ok++

	// ---------------- 负控 A：桩版本不符 ⇒ rc=2 不给结论
	setStubPath()
	os.Setenv("FAKE_COSIGN_VERSION", "v9.9.9")
	res = eggSignRunCosign(nil, eggSignVerifyBlobArgv(key, sig, bundle, egg)...)
	os.Unsetenv("FAKE_COSIGN_VERSION")
	if res.RC != eggSignRCNoConclusion || res.Conclusion || !strings.Contains(res.Why, "版本不符") {
		return eggSignSelfFail(stderr, fmt.Errorf("负控A 版本不符应 rc=%d 不给结论，实得 rc=%d why=%s", eggSignRCNoConclusion, res.RC, res.Why))
	}
	fmt.Fprintf(stdout, "负控A 桩版本 v9.9.9 ⇒ rc=%d · %s\n", res.RC, res.Why)
	ok++

	// ---------------- 负控 B：deny-list 过期 ⇒ fail-closed ＋ 明写「降级为人工」
	expList := filepath.Join(root, "deny-expired.json")
	if err := os.WriteFile(expList, []byte(`{"version":1,"issued_at":"2026-01-01T00:00:00Z","not_after":"2026-02-01T00:00:00Z","revoked":[],"sig":"stub"}`), 0o644); err != nil {
		return eggSignSelfFail(stderr, err)
	}
	d = eggDenyCheckBeforeHatch(expList, eggSHA, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if d.RC != eggDenyRCFailClosed || d.Verdict != "fail-closed" || !strings.Contains(d.Line, eggDenyDegradeLine) {
		return eggSignSelfFail(stderr, fmt.Errorf("负控B 过期清单应 fail-closed ＋ 「%s」，实得 rc=%d verdict=%s line=%s", eggDenyDegradeLine, d.RC, d.Verdict, d.Line))
	}
	fmt.Fprintf(stdout, "负控B deny-list 过期 ⇒ rc=%d · verdict=%s：%s\n", d.RC, d.Verdict, d.Line)
	ok++

	// ---------------- 负控 C：deny-list 缺失 ⇒ fail-closed（✗ 不静默放行）
	d = eggDenyCheckBeforeHatch(filepath.Join(root, "no-such-list.json"), eggSHA, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if d.RC != eggDenyRCFailClosed {
		return eggSignSelfFail(stderr, fmt.Errorf("负控C 清单缺失应 fail-closed rc=%d，实得 rc=%d", eggDenyRCFailClosed, d.RC))
	}
	fmt.Fprintf(stdout, "负控C deny-list 缺失 ⇒ rc=%d（fail-closed · ✗ 不静默放行）\n", d.RC)
	ok++

	// ---------------- 负控 D：旧读端读到 bundle 槽 ⇒ 必拒；新读端 / 空槽两态
	slot := eggSlotEncodeBundle([]byte(`{"mediaType":"application/vnd.dev.sigstore.bundle+json"}`))
	if rc, _ := eggSlotOldReaderVerdict(eggSlotV2Major-1, slot); rc == exitOK {
		return eggSignSelfFail(stderr, fmt.Errorf("负控D 旧读端读到 bundle 槽竟放行"))
	}
	if rc, _ := eggSlotOldReaderVerdict(eggSlotV2Major, nil); rc == exitOK {
		return eggSignSelfFail(stderr, fmt.Errorf("负控D 旧读端读到主号 2 竟放行"))
	}
	if rc, msg := eggSlotNewReaderVerdict(slot); rc != exitOK {
		return eggSignSelfFail(stderr, fmt.Errorf("负控D 新读端读 bundle 槽应放行：rc=%d %s", rc, msg))
	}
	if rc, _ := eggSlotOldReaderVerdict(1, make([]byte, 8)); rc != exitOK {
		return eggSignSelfFail(stderr, fmt.Errorf("负控D 空槽旧读端应放行"))
	}
	if b, okd := eggSlotDecodeBundle(slot); !okd || !bytes.Contains(b, []byte("sigstore")) {
		return eggSignSelfFail(stderr, fmt.Errorf("负控D bundle 槽解码失败"))
	}
	fmt.Fprintf(stdout, "负控D bundle 槽：旧读端拒（主号≥2 / 槽内有 bundle）· 新读端放行 · 空槽旧读端放行 · 解码可取回 bundle\n")
	ok++

	fmt.Fprintf(stdout, "self-test 通过（%d 项：正向 ① ①b ② ③ ④ ＋ 负控 A–D）\n", ok)
	return exitOK
}

// eggSignSelfFail —— 自检失败出口（照 `zegSelfFail` 同形）。
func eggSignSelfFail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s: 自检失败：%v\n", eggSignProg, err)
	return exitFail
}
