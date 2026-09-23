// family_archive.go —— `archive` 族：归档 / 仓外仓命令面（缺口 `G-07` · **全账唯一 P0**）。
//
// 为什么它是 P0（`缺口-命令面-20260922.md` §一 `G-07` 逐字）：归档六步**全靠手写 shell** ——
// 源件实测 **1072 次 `shasum`** + **1072 条临时索引** + 只读往返 + 副本重同步；而业界的接口
// 形状是**现成**的（`调研-缺口-归档台面-20260923.md` 表 A `A1` 逐条取到正文）：
//
//	① 一条命令扫全树出「逐件摘要清单」——`apt-ftparchive` 逐字：「Release file containing
//	   (by default) an MD5, SHA1, SHA256 and SHA512 **digest for each file**」；
//	② **一个进程吃 N 件**——coreutils 逐字：「Synopsis: `sha???sum [option]... [file]...`」；
//	③ 归档包 = **载荷 + 逐件清单 + 标签清单**——RFC 8493 BagIt。
//
// 那 1072 次 `shasum` 不是「工作量」，是**接口缺位**：一次进程能吃完的活，今天按 1072 次进程起。
//
// 三条形状（照上面三条，不自造一处语义）：
//
//	`archive hash <件|目录>…`        一进程算一批件的 sha256 ⇒ 逐行 `sha256␣␣路径`，
//	                                 与 `shasum -a 256` **逐字相同**（行数 = 件数 · **无表头**）
//	`archive manifest <载荷> --out <袋>` 出三件套（`bagit.txt` + `manifest-sha256.txt` +
//	                                 `tagmanifest-sha256.txt`，载荷进 `data/`）；
//	                                 `--dry-run` 先出逐件清单 · 真写要 `--yes` · 失败回滚
//	`archive verify <袋>`            重算载荷 ↔ 清单**逐件**对拍（清单被抹一条 ⇒ 红；
//	                                 载荷改一字节 ⇒ 红；盘上多出未登记件 ⇒ 也红）
//
// 红线（本族逐条）：**不碰归档区物理层** ✗ —— 不 `chmod`、不搬归档区的件、不删件、不重同步副本。
// 本族只读 + **只写 `--out` 指定的那个落点**；落点已存在且非空 ⇒ **拒**（不覆盖别人的件）。
package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// RFC 8493 BagIt 的四个件名（逐字照规范，不自造）。
const (
	archiveBagitTxt        = "bagit.txt"
	archiveManifestName    = "manifest-sha256.txt"
	archiveTagmanifestName = "tagmanifest-sha256.txt"
	archiveDataDir         = "data"
)

// bagitTxtBody —— `bagit.txt` 逐字（RFC 8493：两行，版本行 + 编码行）。
const bagitTxtBody = "BagIt-Version: 0.97\nTag-File-Character-Encoding: UTF-8\n"

// ---- 公共：现算一件的 sha256 ----------------------------------------------------------------

// sha256File 现算一件的 sha256（**本进程内**逐件算 —— 与 `shasum -a 256` 同一算法，
// 差别只在进程数：这里一次进程吃 N 件，正是 `G-07` 要收掉的那 1072 次进程起）。
func sha256File(p string) (string, int64, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), n, nil
}

// archiveExpandTargets 把「件 / 目录」参数摊成逐件路径：目录 ⇒ 顶层件（**非递归** · 字典序），
// 路径文本按**给定形态**拼（不转绝对路径）⇒ 与 `shasum -a 256 <目录>/*` 的路径文本逐字对得上。
func archiveExpandTargets(args []string) ([]string, error) {
	out := []string{}
	for _, a := range args {
		fi, err := os.Stat(a)
		if err != nil {
			return nil, fmt.Errorf("路径 %q 读不到：%v", a, err)
		}
		if !fi.IsDir() {
			out = append(out, a)
			continue
		}
		ents, err := os.ReadDir(a)
		if err != nil {
			return nil, fmt.Errorf("目录 %q 读不到：%v", a, err)
		}
		names := []string{}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, n := range names {
			out = append(out, filepath.Join(a, n))
		}
	}
	return out, nil
}

// ---- `archive hash` ------------------------------------------------------------------------

// cmdArchiveHash —— 一进程吃 N 件，出逐件 `sha256␣␣路径`（**无表头** ⇒ 行数 = 件数，
// 与 `shasum -a 256` 逐字相同）。
//
// 退码：`0` 全算出来 · `2` 用法错（没给目标 / 路径不存在）· `8` 读不到（**不拿「读不到」当「没有」**）。
func cmdArchiveHash(inv *invocation, stdout, stderr io.Writer) int {
	// `--json` 字段面先判（§4.1 K2：给了 `--json` 不给字段 ⇒ 退码由 `requireFields` 取自退码表 · 归一后 = 2 + stdout 0 字节）。
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "`archive hash` 要给至少一件/一个目录")
		fmt.Fprintf(stderr, "%s: `archive hash` 要吃件或目录（例：%s archive hash bin/）\n", progName, progName)
		return exitUsage
	}
	targets, err := archiveExpandTargets(inv.args)
	if err != nil {
		inv.setErr("usage", "bad_target", err.Error())
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "（路径不存在 = 用法错 2 —— 不静默跳过它）\n")
		return exitUsage
	}
	if len(targets) == 0 {
		inv.setErr("blocked", "empty_target", "给的目标里一件都没有")
		fmt.Fprintf(stderr, "%s: 给的目标里一件都没有 —— **不给结论**（退码 8）：空清单不是「算完了」\n", progName)
		return exitBlocked
	}
	rows := make([]map[string]string, 0, len(targets))
	for _, p := range targets {
		sum, n, err := sha256File(p)
		if err != nil {
			inv.setErr("blocked", "unreadable", err.Error())
			fmt.Fprintf(stderr, "%s: %q 读不到：%v ⇒ 不给结论（退码 8）—— **不许把「读不到」当「没有」**\n",
				progName, p, err)
			return exitBlocked
		}
		rows = append(rows, map[string]string{
			"path": p, "sha256": sum, "bytes": strconv.FormatInt(n, 10)})
	}
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	// 裸行面（人面 = 行式面，零装饰）：`sha256` + 两空格 + 路径 —— 与 `shasum -a 256` 逐字同。
	for _, r := range rows {
		fmt.Fprintf(stdout, "%s  %s\n", r["sha256"], r["path"])
	}
	fmt.Fprintf(stderr, "%s: 一进程算完 %d 件（逐行 `sha256␣␣路径` · 与 `shasum -a 256` 逐字同）\n",
		progName, len(rows))
	return exitOK
}

// ---- `archive manifest`（三件套 · 干跑先行 · 失败回滚）--------------------------------------

// archivePayloadEntries 列载荷目录下的全部件（**递归** · 仓根相对的 slash 形态 · 字典序）。
// 载荷里的空目录也跟着建（BagIt 不丢空目录，否则回放少一层）。
func archivePayloadEntries(payload string) (files []string, dirs []string, err error) {
	walkErr := filepath.WalkDir(payload, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, rerr := filepath.Rel(payload, p)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			dirs = append(dirs, rel)
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if walkErr != nil {
		return nil, nil, walkErr
	}
	sort.Strings(files)
	sort.Strings(dirs)
	return files, dirs, nil
}

// archiveManifestLines 出逐件清单行（`sha256␣␣data/<相对路径>`）—— 清单与载荷同一个进程算完。
func archiveManifestLines(payload string, files []string) ([]string, error) {
	lines := make([]string, 0, len(files))
	for _, rel := range files {
		sum, _, err := sha256File(filepath.Join(payload, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("%s 读不到：%v", rel, err)
		}
		lines = append(lines, sum+"  "+archiveDataDir+"/"+rel)
	}
	return lines, nil
}

// cmdArchiveManifest —— `archive manifest <载荷目录> --out <袋目录> [--dry-run | --yes]`。
//
// 三态（与 `family_h.go` 的写面同一套语义）：`--dry-run` 出计划件（退码 0 · 零副作用）·
// 缺 `--yes` ⇒ `2`（fail-closed）· 齐了才真写。真写**任一步失败 ⇒ 回滚**（删掉本次建的袋目录）。
func cmdArchiveManifest(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "`archive manifest` 要给载荷目录")
		fmt.Fprintf(stderr, "%s: 用法：%s archive manifest <载荷目录> --out <袋目录> [--dry-run | --yes]\n",
			progName, progName)
		return exitUsage
	}
	payload := inv.args[0]
	out := strings.TrimSpace(inv.flagVal("--out"))
	if out == "" {
		inv.setErr("usage", "missing_out", "缺 `--out <袋目录>`")
		fmt.Fprintf(stderr, "%s: 缺 `--out <袋目录>` —— **不给落点就不动盘**（本族不往默认位置偷偷写）\n", progName)
		return exitUsage
	}
	fi, err := os.Stat(payload)
	if err != nil || !fi.IsDir() {
		inv.setErr("usage", "payload_absent", "载荷目录不在或不是目录")
		fmt.Fprintf(stderr, "%s: 载荷 %q 不在（或不是目录）⇒ 用法错 2\n", progName, payload)
		return exitUsage
	}
	absP, errP := filepath.Abs(payload)
	absO, errO := filepath.Abs(out)
	if errP == nil && errO == nil {
		if absO == absP || strings.HasPrefix(absO+string(filepath.Separator), absP+string(filepath.Separator)) {
			inv.setErr("usage", "out_inside_payload", "袋落点落在载荷里面")
			fmt.Fprintf(stderr, "%s: 袋落点 %q 落在载荷 %q 里面 ⇒ 会自己吃自己（用法错 2）\n",
				progName, out, payload)
			return exitUsage
		}
	}
	if ents, err := os.ReadDir(out); err == nil && len(ents) > 0 {
		inv.setErr("usage", "out_not_empty", "袋落点已有件")
		fmt.Fprintf(stderr, "%s: 袋落点 %q 已有 %d 件 ⇒ **拒**（不覆盖别人的件；要重出就换个落点或先把空目录清干净）\n",
			progName, out, len(ents))
		return exitUsage
	}
	files, dirs, err := archivePayloadEntries(payload)
	if err != nil {
		inv.setErr("blocked", "payload_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 载荷读不到：%v ⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}
	if len(files) == 0 {
		inv.setErr("blocked", "payload_empty", "载荷里一件都没有")
		fmt.Fprintf(stderr, "%s: 载荷 %q 里一件都没有 —— **不给结论**（退码 8）：空袋不是「做好了」\n", progName, payload)
		return exitBlocked
	}
	lines, err := archiveManifestLines(payload, files)
	if err != nil {
		inv.setErr("blocked", "payload_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: %v ⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}

	// ---- 干跑：出计划件（逐件清单）· 零副作用 ----
	if inv.dryRun {
		if inv.jsonGiven {
			return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, archiveManifestRows(out, lines))
		}
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : %s archive manifest\n", progName)
		fmt.Fprintf(stdout, "  载荷     : %s（%d 件 · 空目录 %d 个）\n", payload, len(files), len(dirs))
		fmt.Fprintf(stdout, "  袋落点   : %s\n", out)
		fmt.Fprintf(stdout, "  它会动   : 只建这一个袋目录（%s / %s / %s + 载荷进 %s/）\n",
			archiveBagitTxt, archiveManifestName, archiveTagmanifestName, archiveDataDir)
		fmt.Fprintf(stdout, "  逐件清单（%d 行 · `sha256␣␣data/<相对路径>`）：\n", len(lines))
		for _, l := range lines {
			fmt.Fprintf(stdout, "    %s\n", l)
		}
		fmt.Fprintf(stdout, "  归档区   : **不碰**（本命令只写 `--out` 那一处；不 chmod / 不搬件 / 不删件 / 不重同步副本）\n")
		fmt.Fprintf(stdout, "  执行要   : --yes（写面 fail-closed）\n")
		fmt.Fprintf(stdout, "  来源     : §一 G-07 · RFC 8493 BagIt · 调研-缺口-归档台面-20260923 表 A A1\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "写面缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `archive manifest` 是写面 —— **缺 --yes ⇒ 不执行**（fail-closed）\n", progName)
		fmt.Fprintf(stderr, "先看计划件：%s archive manifest %s --out %s --dry-run\n", progName, payload, out)
		return exitUsage
	}

	// ---- 真写（失败回滚：删掉**本次建的**那个袋目录）----
	if err := writeBag(out, payload, files, dirs, lines); err != nil {
		_ = os.RemoveAll(out)
		inv.setErr("failed", "bag_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写袋失败：%v\n", progName, err)
		fmt.Fprintf(stderr, "已**回滚**：本次建的 %q 已删（不留在盘上半个袋）\n", out)
		return exitFail
	}
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, archiveManifestRows(out, lines))
	}
	fmt.Fprintf(stdout, "袋已建：%s\n", out)
	fmt.Fprintf(stdout, "  %s（载荷 %d 件 · 空目录 %d 个）\n", archiveDataDir+"/", len(files), len(dirs))
	fmt.Fprintf(stdout, "  %s（%d 行）\n", archiveManifestName, len(lines))
	fmt.Fprintf(stdout, "  %s（2 行：bagit.txt + manifest-sha256.txt）\n", archiveTagmanifestName)
	fmt.Fprintf(stdout, "  %s\n", archiveBagitTxt)
	fmt.Fprintf(stderr, "%s: 袋已建 · 校验它：%s archive verify %s\n", progName, progName, out)
	return exitOK
}

// archiveManifestRows 把逐件清单摊成行（`--json` 与真跑共用一份形状）。
func archiveManifestRows(bag string, lines []string) []map[string]string {
	rows := make([]map[string]string, 0, len(lines))
	for _, l := range lines {
		digest, entry, ok := archiveSplitManifestLine(l)
		if !ok {
			continue
		}
		rows = append(rows, map[string]string{"bag": bag, "entry": entry, "sha256": digest})
	}
	return rows
}

// writeBag 真写一只袋：`data/`（载荷逐件拷）+ `manifest-sha256.txt` + `bagit.txt` + `tagmanifest-sha256.txt`。
//
// 顺序（先建后签）：① 载荷先落 ⇒ ② 按**落盘后的载荷**重算清单 ⇒ ③ bagit.txt ⇒
// ④ 标签清单按**落盘后的** bagit.txt / 清单件重算 —— 每一步都对**盘上的字节**负责，不对内存里的假设负责。
func writeBag(out, payload string, files, dirs, lines []string) error {
	if err := os.MkdirAll(filepath.Join(out, archiveDataDir), 0o755); err != nil {
		return err
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(out, archiveDataDir, filepath.FromSlash(d)), 0o755); err != nil {
			return err
		}
	}
	for _, rel := range files {
		src := filepath.Join(payload, filepath.FromSlash(rel))
		dst := filepath.Join(out, archiveDataDir, filepath.FromSlash(rel))
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	// 清单按**落盘后的载荷**重算（拷坏一个字节也必须在这里露出来）。
	disk := []string{}
	for _, rel := range files {
		sum, _, err := sha256File(filepath.Join(out, archiveDataDir, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		disk = append(disk, sum+"  "+archiveDataDir+"/"+rel)
	}
	if strings.Join(disk, "\n") != strings.Join(lines, "\n") {
		return fmt.Errorf("落盘后的载荷重算与清单不一致（拷贝出错 ⇒ 停手，不回半个袋）")
	}
	if err := os.WriteFile(filepath.Join(out, archiveManifestName),
		[]byte(strings.Join(disk, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, archiveBagitTxt), []byte(bagitTxtBody), 0o644); err != nil {
		return err
	}
	tag := []string{}
	for _, name := range []string{archiveBagitTxt, archiveManifestName} {
		sum, _, err := sha256File(filepath.Join(out, name))
		if err != nil {
			return err
		}
		tag = append(tag, sum+"  "+name)
	}
	return os.WriteFile(filepath.Join(out, archiveTagmanifestName),
		[]byte(strings.Join(tag, "\n")+"\n"), 0o644)
}

// copyFile 逐字节拷一件（0644 —— 归档件不是可执行件）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ---- `archive verify`（重算载荷 ↔ 清单逐件对拍）--------------------------------------------

// archiveSplitManifestLine 拆一行清单（`sha256␣␣路径`；也容忍制表）。不认的形状 ⇒ ok=false（判红）。
func archiveSplitManifestLine(line string) (digest, entry string, ok bool) {
	s := strings.TrimRight(line, " \t\r")
	if strings.TrimSpace(s) == "" {
		return "", "", false
	}
	i := strings.IndexAny(s, " \t")
	if i <= 0 {
		return "", "", false
	}
	d := strings.TrimSpace(s[:i])
	e := strings.TrimSpace(s[i:])
	if len(d) != 64 || len(e) == 0 {
		return "", "", false
	}
	for _, r := range d {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return "", "", false
		}
	}
	return d, e, true
}

// cmdArchiveVerify —— 校验一只袋。判据三半（**缺一不出结论**）：
//
//	① 清单里每一条：`<袋>/<entry>` 重算 == 清单 digest（缺件 / 不符 ⇒ 红）；
//	② 盘上**多出未登记**的载荷件（清单里没有）⇒ 红 —— 这一半正是「**抹掉清单里任意一条 ⇒ 必红**」；
//	③ `tagmanifest-sha256.txt` 覆盖 `bagit.txt` 与 `manifest-sha256.txt`（标签件也被签）。
//
// 退码：`0` 全对 · `1` 判红（逐条报）· `8` **判不了**（不是袋 / 清单读不到 —— 不把「判不了」当「过了」）。
func cmdArchiveVerify(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "`archive verify` 要给袋目录")
		fmt.Fprintf(stderr, "%s: 用法：%s archive verify <袋目录>\n", progName, progName)
		return exitUsage
	}
	bag := inv.args[0]
	if fi, err := os.Stat(bag); err != nil || !fi.IsDir() {
		inv.setErr("blocked", "bag_absent", "袋目录不在或不是目录")
		fmt.Fprintf(stderr, "%s: %q 不在（或不是目录）⇒ 判不了（退码 8）\n", progName, bag)
		return exitBlocked
	}
	if _, err := os.Stat(filepath.Join(bag, archiveBagitTxt)); err != nil {
		inv.setErr("blocked", "not_a_bag", "袋目录里没有 bagit.txt")
		fmt.Fprintf(stderr, "%s: %q 里没有 %s ⇒ **不是一只袋**（判不了 · 退码 8 —— 不把「判不了」当「过了」）\n",
			progName, bag, archiveBagitTxt)
		return exitBlocked
	}
	manBytes, err := os.ReadFile(filepath.Join(bag, archiveManifestName))
	if err != nil {
		inv.setErr("blocked", "manifest_absent", err.Error())
		fmt.Fprintf(stderr, "%s: 清单 %s 读不到：%v ⇒ 判不了（退码 8）\n", progName, archiveManifestName, err)
		return exitBlocked
	}

	rows := []map[string]string{}
	bad := 0
	declared := map[string]string{}
	for _, line := range strings.Split(string(manBytes), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		digest, entry, ok := archiveSplitManifestLine(line)
		if !ok {
			bad++
			rows = append(rows, map[string]string{"entry": strings.TrimSpace(line),
				"want": "（本行不是 `sha256␣␣路径` 形状）", "got": "-", "verdict": "清单行不认"})
			continue
		}
		declared[entry] = digest
		got, _, err := sha256File(filepath.Join(bag, filepath.FromSlash(entry)))
		switch {
		case err != nil:
			bad++
			rows = append(rows, map[string]string{"entry": entry, "want": digest, "got": "（读不到）",
				"verdict": "缺件"})
		case got != digest:
			bad++
			rows = append(rows, map[string]string{"entry": entry, "want": digest, "got": got,
				"verdict": "不符"})
		default:
			rows = append(rows, map[string]string{"entry": entry, "want": digest, "got": got,
				"verdict": "一致"})
		}
	}
	// ② 反向：盘上多出未登记件 ⇒ 红（清单被抹一条也在这里露出来）。
	onDisk, _, err := archivePayloadEntries(filepath.Join(bag, archiveDataDir))
	if err != nil {
		inv.setErr("blocked", "payload_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 袋里的载荷读不到：%v ⇒ 判不了（退码 8）\n", progName, err)
		return exitBlocked
	}
	for _, rel := range onDisk {
		entry := archiveDataDir + "/" + rel
		if _, ok := declared[entry]; ok {
			continue
		}
		bad++
		rows = append(rows, map[string]string{"entry": entry, "want": "（清单里没有这一条）",
			"got": "（盘上有）", "verdict": "未登记"})
	}
	// ③ 标签清单：签 `bagit.txt` 与清单件本体。
	tagBytes, err := os.ReadFile(filepath.Join(bag, archiveTagmanifestName))
	if err != nil {
		bad++
		rows = append(rows, map[string]string{"entry": archiveTagmanifestName,
			"want": "（这个件该在）", "got": "（读不到）", "verdict": "缺件"})
	} else {
		for _, line := range strings.Split(string(tagBytes), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			digest, entry, ok := archiveSplitManifestLine(line)
			if !ok {
				bad++
				rows = append(rows, map[string]string{"entry": strings.TrimSpace(line),
					"want": "（本行不是 `sha256␣␣路径` 形状）", "got": "-", "verdict": "标签行不认"})
				continue
			}
			got, _, err := sha256File(filepath.Join(bag, filepath.FromSlash(entry)))
			if err != nil || got != digest {
				bad++
				rows = append(rows, map[string]string{"entry": entry, "want": digest,
					"got": "（读不到或不符）", "verdict": "标签不符"})
				continue
			}
			rows = append(rows, map[string]string{"entry": entry, "want": digest, "got": got,
				"verdict": "标签一致"})
		}
	}

	if inv.jsonGiven {
		rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
		if bad > 0 {
			return exitFail
		}
		return rc
	}
	fmt.Fprintf(stdout, "袋：%s\n", bag)
	fmt.Fprintf(stdout, "清单里 %d 条 · 盘上载荷 %d 件\n", len(declared), len(onDisk))
	if bad > 0 {
		fmt.Fprintf(stderr, "%s: **校验红** —— %d 条对不上（逐条如下）\n", progName, bad)
		for _, r := range rows {
			if r["verdict"] == "一致" || r["verdict"] == "标签一致" {
				continue
			}
			fmt.Fprintf(stderr, "  · %s  %s（清单 %s · 重算 %s）\n", r["verdict"], r["entry"], r["want"], r["got"])
		}
		inv.setErr("failed", "bag_verify_mismatch", "袋里的件与清单对不上")
		return exitFail
	}
	fmt.Fprintf(stderr, "%s: 逐件重算 == 清单 ✓（标签件也对得上）—— 袋没坏\n", progName)
	return exitOK
}
