// Package sign — macOS 签名身份的**唯一**解析/施加面（`scripts/build/lib-sign.sh` 的 Go 化对等体）。
//
// 设计依据：`~/.zerg/state/设计-批6-自举链-谁编谁-20261007.md` §三（硬约束 · 签名）· §四。
// 拍板：决-07 —— `lib-sign.sh` 并入构建器 Go 包（`build-all` 与 `pack-release` 同 import）；
// 搬后须保 stable/adhoc **两态**，✗ 不得退回 adhoc。
//
// 铁律（`lib-sign.sh:4-7` 逐字）：两个调用者要签名，「两份实现就会漂」——漂的形态正是最怕的
// 那种：一件用稳定身份、另一件回落 ad-hoc ⇒ macOS TCC 的本地网络授权随 cdhash 静默失效
// （现象：主控拨号报 no route to host）。本包是**唯一**实现。
//
// 边界：只解析身份 + 施加签名 —— 不编译、不打包、不碰进程、不读仓内证物；件不在场自带守卫。
package sign

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolved 是身份解析结果（对等 `ZERG_SIGN_ID` / `SIGN_MODE`）。
type Resolved struct {
	Enabled bool   // false = 本档不签名（SIGN=0 / 无 codesign）
	ID      string // 解析到的身份名；回落 ad-hoc 时 = "-"
	Mode    string // "stable:<身份名>" 或 "adhoc"
	Stable  bool
}

// Resolve 只解析身份，不改文件（对等 `resolve_sign_identity`）。
// enabled = 本档要不要签名（build-all 的 `SIGN`）；tmpdir = 探针件落点（省则 /tmp）。
func Resolve(enabled bool, tmpdir string, logf func(string, ...any)) Resolved {
	if !enabled {
		return Resolved{}
	}
	if _, err := exec.LookPath("codesign"); err != nil {
		return Resolved{}
	}
	id := os.Getenv("ZERG_SIGN_ID")
	if id == "" {
		id = "Zerg Local Signing"
	}
	tmp := tmpdir
	if tmp == "" {
		tmp = "/tmp"
	}
	// 探针件必须先造出来（`lib-sign.sh:35-36`：原写法签的是不在盘的路径 ⇒ 该兜底支恒失败）。
	probe := filepath.Join(tmp, fmt.Sprintf(".zerg-sign-probe.%d", os.Getpid()))
	if id != "" {
		if f, err := os.Create(probe); err == nil {
			f.Close()
		}
	}
	sig := ""
	if id != "" {
		// 第一支：身份在钥匙串（`security find-identity | grep -q`）—— 不受信任也不影响签名。
		out, _ := exec.Command("security", "find-identity").CombinedOutput()
		if strings.Contains(string(out), id) {
			sig = "in-keychain-not-trusted"
		} else if err := exec.Command("codesign", "-s", id, "--force", probe).Run(); err == nil {
			// 第二支：探针保底（真的签得出 ⇒ 身份可用）。
			sig = "probe-ok"
		}
	}
	r := Resolved{Enabled: true}
	if sig != "" {
		r.ID, r.Mode, r.Stable = id, "stable:"+id, true
	} else {
		r.ID, r.Mode = "-", "adhoc"
	}
	_ = os.Remove(probe)
	r.report(sig, id, logf)
	return r
}

func (r Resolved) report(sig, id string, logf func(string, ...any)) {
	if logf == nil {
		return
	}
	if !r.Stable {
		logf("   ⚠️  未找到签名身份「-」，回退 ad-hoc（本机 TCC 授权会随重编失效）")
	} else if sig == "in-keychain-not-trusted" {
		logf("   🔏 身份「%s」在钥匙串但**不受信任**（CSSMERR_TP_NOT_TRUSTED）—— 不影响签名，照常签稳定身份", id)
	}
}

// One 单件重签（对等 `sign_one`）；identifier 按既有固定表。件不在场 / 本档不签名 / 无 codesign ⇒ 静默跳过。
//
// ★ 与 `.sh` 逐字同口径：codesign 非 0 **不**阻断（Linux ELF 一类非 Mach-O 件本就签不上，
// 不能把「签不了 ELF」当成构建失败）—— `lib-sign.sh:81-83` 的 fail-open 在本批**原样保留**。
func One(r Resolved, path string, logf func(string, ...any)) {
	if !r.Enabled {
		return
	}
	if _, err := exec.LookPath("codesign"); err != nil {
		return
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return
	}
	name := filepath.Base(path)
	zid := identifier(name)
	if err := exec.Command("codesign", "-s", r.ID, "--identifier", zid, "--force", path).Run(); err == nil {
		if logf != nil {
			logf("   🔏 已重签名 %s（%s，%s）", name, zid, r.Mode)
		}
	}
}

// identifier 是既有固定表（`lib-sign.sh:61-79` 逐字）。
//
// · 本机部署件：三段短名（在跑的那些）；
// · 四段契约名 `zerg-<组件>-<os>-<arch>`：identifier 必须与**同组件本机件**一致
//   （GAP-20261003-10「同一组件两枚二进制不许两套身份」）。
// `zerg-agent-*` **不会**吞掉 `zerg-agentd-*`（`-` ≠ `d`）⇒ 两条可并存，顺序先精确后通配。
func identifier(name string) string {
	switch name {
	case "zerg-core":
		return "com.zerg.core"
	case "zerg-agent":
		return "com.zerg.agent"
	case "zerg-agentd":
		return "com.zerg.agentd"
	case "zerg-ui":
		return "com.zerg.ui"
	case "zerg-wall":
		return "com.zerg.wall"
	case "zerg-cli":
		return "com.zerg.cli"
	}
	switch {
	case strings.HasPrefix(name, "zerg-core-"):
		return "com.zerg.core"
	case strings.HasPrefix(name, "zerg-agent-"):
		return "com.zerg.agent"
	case strings.HasPrefix(name, "zerg-agentd-"):
		return "com.zerg.agentd"
	case strings.HasPrefix(name, "zerg-cli-"):
		return "com.zerg.cli"
	case strings.HasPrefix(name, "zerg-ui-"):
		return "com.zerg.ui"
	case strings.HasPrefix(name, "zerg-wall-"):
		return "com.zerg.wall"
	}
	return "com.zerg." + name
}
