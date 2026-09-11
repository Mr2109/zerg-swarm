// zerg-model —— 模型登记记录的工具（虫族 v2.5.9「多模型接入」）。
//
// 用法：
//
//	zerg-model verify <record.json> [--strict] [--json]
//	zerg-model probe  <路径|端点URL> [--out record.json] [--json] [--endpoint URL] [--engine llama.cpp|vllm|ollama]
//
// 退出码：0 成功 / 1 探测失败 / 2 记录不合标准 / 3 参数或环境错误 / 4 引擎不可达
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

func usage() {
	fmt.Println("用法:")
	fmt.Println("  zerg-model verify <record.json> [--strict] [--json]   # 校验一条模型登记记录是否符合标准")
	fmt.Println("  zerg-model probe  <路径|端点URL> [--out <record.json>|--store] [--json]")
	fmt.Println("                    [--endpoint <URL>] [--engine llama.cpp|vllm|ollama] [--id <id>] [--timeout 60s]")
	fmt.Println("                                                          # 探测模型并生成登记记录（默认只打印，不写盘）")
	fmt.Println("                    --store                               # 写进模型目录 manifests/（标准 §三 布局）")
	fmt.Println("                    --store-root <dir>                    # 模型目录根（默认 ~/.zerg/models，或 $ZERG_MODELS_DIR）")
	fmt.Println("  zerg-model list   [--root <模型目录根>] [--manifests <目录>] [--json]")
	fmt.Println("                                                          # 列出模型目录里的记录（只读；含每条记录的校验结论）")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(3)
	}
	switch os.Args[1] {
	case "verify":
		os.Exit(cmdVerify(os.Args[2:]))
	case "probe":
		os.Exit(cmdProbe(os.Args[2:]))
	case "list":
		os.Exit(cmdList(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n", os.Args[1])
		usage()
		os.Exit(3)
	}
}

func cmdVerify(args []string) int {
	strict, asJSON, path := false, false, ""
	for _, a := range args {
		switch a {
		case "--strict":
			strict = true
		case "--json":
			asJSON = true
		default:
			if path == "" {
				path = a
			}
		}
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "verify 需要一个记录文件路径")
		return 3
	}
	rec, err := modelreg.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return 3
	}
	findings := modelreg.Verify(rec, strict)
	nErr := modelreg.CountErrors(findings)
	if asJSON {
		out, _ := json.MarshalIndent(struct {
			Path     string             `json:"path"`
			Schema   string             `json:"schema"`
			Errors   int                `json:"errors"`
			Findings []modelreg.Finding `json:"findings"`
		}{path, rec.Schema, nErr, findings}, "", "  ")
		fmt.Println(string(out))
	} else {
		if len(findings) == 0 {
			fmt.Printf("✅ 合格：%s（schema=%s）\n", path, rec.Schema)
			fmt.Println("   注意：本命令只校验【记录格式】，不校验权重文件本身是否可信（--integrity 属后续批，见待修补 #13）")
		} else {
			for _, f := range findings {
				mark := "✗"
				if f.Level == "warn" {
					mark = "⚠️"
				}
				fmt.Printf("%s [%s] %s：%s\n", mark, f.Level, f.Field, f.Detail)
			}
			fmt.Printf("—— error %d 条，warn %d 条\n", nErr, len(findings)-nErr)
		}
	}
	if nErr > 0 || (strict && len(findings) > 0) {
		return 2
	}
	return 0
}

func cmdProbe(args []string) int {
	var target, out, endpoint, engine, id, storeRoot string
	useStore := false
	asJSON := false
	timeout := modelreg.DefaultProbeTimeout

	needVal := func(i *int, flag string) (string, bool) {
		if *i+1 >= len(args) {
			fmt.Fprintf(os.Stderr, "%s 需要一个值\n", flag)
			return "", false
		}
		*i++
		return args[*i], true
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--out":
			v, ok := needVal(&i, "--out")
			if !ok {
				return 3
			}
			out = v
		case strings.HasPrefix(a, "--out="):
			out = strings.TrimPrefix(a, "--out=")
		case a == "--store":
			useStore = true
		case a == "--store-root":
			v, ok := needVal(&i, "--store-root")
			if !ok {
				return 3
			}
			storeRoot = v
		case strings.HasPrefix(a, "--store-root="):
			storeRoot = strings.TrimPrefix(a, "--store-root=")
		case a == "--endpoint":
			v, ok := needVal(&i, "--endpoint")
			if !ok {
				return 3
			}
			endpoint = v
		case strings.HasPrefix(a, "--endpoint="):
			endpoint = strings.TrimPrefix(a, "--endpoint=")
		case a == "--engine":
			v, ok := needVal(&i, "--engine")
			if !ok {
				return 3
			}
			engine = v
		case strings.HasPrefix(a, "--engine="):
			engine = strings.TrimPrefix(a, "--engine=")
		case a == "--id":
			v, ok := needVal(&i, "--id")
			if !ok {
				return 3
			}
			id = v
		case strings.HasPrefix(a, "--id="):
			id = strings.TrimPrefix(a, "--id=")
		case a == "--timeout":
			v, ok := needVal(&i, "--timeout")
			if !ok {
				return 3
			}
			d, err := time.ParseDuration(v)
			if err != nil {
				fmt.Fprintf(os.Stderr, "--timeout 不是合法时长：%v\n", err)
				return 3
			}
			timeout = d
		case strings.HasPrefix(a, "--timeout="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "--timeout="))
			if err != nil {
				fmt.Fprintf(os.Stderr, "--timeout 不是合法时长：%v\n", err)
				return 3
			}
			timeout = d
		case a == "--json":
			asJSON = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "probe 未知参数: %s\n", a)
			return 3
		default:
			if target != "" {
				fmt.Fprintf(os.Stderr, "probe 只接受一个目标，多余的：%s\n", a)
				return 3
			}
			target = a
		}
	}
	if target == "" {
		fmt.Fprintln(os.Stderr, "probe 需要一个本地路径或端点 URL")
		usage()
		return 3
	}

	rec, rep, err := modelreg.Probe(modelreg.ProbeOptions{
		Target:   target,
		Endpoint: endpoint,
		Engine:   engine,
		ID:       id,
		Timeout:  timeout,
	})
	if err != nil {
		var un *modelreg.UnreachableError
		if errors.As(err, &un) {
			fmt.Fprintf(os.Stderr, "✗ 引擎不可达：%v\n", err)
			return 4
		}
		fmt.Fprintf(os.Stderr, "✗ 探测失败：%v\n", err)
		return 1
	}

	recJSON, _ := json.MarshalIndent(rec, "", "  ")
	findings := modelreg.Verify(rec, false)
	nErr := modelreg.CountErrors(findings)
	nWarn := len(findings) - nErr

	// 留痕到 stderr，方便人工复看（对应标准 §九「三样证据」）。
	fmt.Fprintf(os.Stderr, "探测目标：%s\n", rep.Target)
	if rep.Endpoint != "" {
		fmt.Fprintf(os.Stderr, "端点：%s（在线探测%s）\n", rep.Endpoint, map[bool]string{true: "已执行", false: "未执行"}[rep.OnlineProbed])
	}
	fmt.Fprintln(os.Stderr, "探测器留痕：")
	for _, t := range rep.Traces {
		mark := "✓"
		if !t.OK {
			mark = "✗"
		}
		line := fmt.Sprintf("  %s %s ok=%v status=%d ms=%d", mark, t.Probe, t.OK, t.HTTPStatus, t.ElapsedMS)
		if t.FailureClass != "" {
			line += " class=" + t.FailureClass
		}
		if t.Summary != "" {
			line += " resp=" + t.Summary
		}
		fmt.Fprintln(os.Stderr, line)
	}
	fmt.Fprintf(os.Stderr, "生成记录：id=%s digest=%s files=%d capabilities_snapshot=%d\n", rec.ID, rec.Digest, len(rec.Files), len(rep.Capabilities))
	fmt.Fprintf(os.Stderr, "自校验：error %d 条，warn %d 条\n", nErr, nWarn)
	for _, f := range findings {
		mark := "✗"
		if f.Level == "warn" {
			mark = "⚠️"
		}
		fmt.Fprintf(os.Stderr, "%s [%s] %s：%s\n", mark, f.Level, f.Field, f.Detail)
	}

	// 落盘目标（批 3）：--store，或 --out 给的是目录 → 进模型目录；
	// --out 给文件 → 原子写该文件。
	storeDir := ""
	if useStore {
		storeDir = modelreg.NewStore(storeRoot).ManifestsDir()
	} else if out != "" && looksLikeDir(out) {
		storeDir = strings.TrimRight(out, "/")
	}

	if asJSON {
		// 能力快照内容照旧在 --json 里给（给人看）：能力断言 + 证据 + 端点 + 生成时间。
		// 它不再出现在 record 正文里（正文只装身份），改由这里与兄弟文件承载。
		payload, _ := json.MarshalIndent(struct {
			Record             *modelreg.Record                    `json:"record"`
			CapabilitySnapshot modelreg.CapabilitySnapshotArtifact `json:"capability_snapshot"`
			ProbeTrace         []modelreg.Trace                    `json:"probe_trace"`
			Findings           []modelreg.Finding                  `json:"findings"`
			Errors             int                                 `json:"errors"`
			Warnings           int                                 `json:"warnings"`
		}{rec, modelreg.NewCapabilitySnapshot(rec, rep), rep.Traces, findings, nErr, nWarn}, "", "  ")
		fmt.Println(string(payload))
	} else if out == "" && storeDir == "" {
		// 默认只打印，不写盘
		fmt.Println(string(recJSON))
	}

	if out != "" && storeDir == "" {
		changed, err := modelreg.WriteFileAtomic(out, append(recJSON, '\n'))
		if err != nil {
			fmt.Fprintf(os.Stderr, "写记录失败: %v\n", err)
			return 3
		}
		if changed {
			fmt.Fprintf(os.Stderr, "已写入：%s\n", out)
		} else {
			fmt.Fprintf(os.Stderr, "内容一致，未重写：%s\n", out)
		}
		// 待修补 #24：记录正文是确定的（重复跑不重写，changed=false）；易变留痕
		// （生成时间 / 耗时 / 原始响应片段）另写兄弟文件 <文件>.trace.json。
		writeTraceSibling(out, rec, rep)
		// 能力快照（能力断言 + 证据）另写兄弟文件 <文件>.capabilities.json——
		// 正文只装身份，能力不进正文，避免"先登记、后补能力实测"撞防覆盖保护。
		writeCapabilitySibling(out, rec, rep)
	}

	code := 0
	if storeDir != "" {
		res, err := modelreg.NewStoreAtManifests(storeDir).Put(rec, false)
		var adm *modelreg.AdmissionError
		var conf *modelreg.ConflictError
		switch {
		case err == nil && res.Changed:
			fmt.Fprintf(os.Stderr, "已入目录：%s（version=%s）\n", res.Path, res.Version)
		case err == nil:
			fmt.Fprintf(os.Stderr, "目录里已有同内容记录，未重写：%s（version=%s）\n", res.Path, res.Version)
		case errors.As(err, &adm):
			// 门禁如实拒绝（标准 §十二.1「verify 不过直接拒」）：不降级、不写占位值凑绿。
			fmt.Fprintf(os.Stderr, "✗ 拒绝入目录：%s\n", err)
			for _, f := range adm.Findings {
				fmt.Fprintf(os.Stderr, "   [%s] %s：%s\n", f.Level, f.Field, f.Detail)
			}
			fmt.Fprintf(os.Stderr, "   （未写入任何文件；按上面每条 error 修好记录后重跑）\n")
			code = 2
		case errors.As(err, &conf):
			fmt.Fprintf(os.Stderr, "✗ %v\n", err)
			code = 3
		default:
			fmt.Fprintf(os.Stderr, "✗ 写模型目录失败：%v\n", err)
			code = 3
		}
		// 待修补 #24：记录成功入目录后（changed=true 或幂等 changed=false），
		// 把易变留痕写兄弟文件 <version>.trace.json。被门禁拒绝/冲突时不写留痕。
		if err == nil {
			writeTraceSibling(res.Path, rec, rep)
			writeCapabilitySibling(res.Path, rec, rep)
		}
	}

	if nErr > 0 {
		return 2
	}
	return code
}

// writeTraceSibling 把探测留痕（生成时间 / 每项探测器耗时 / 原始响应片段）写成记录旁的
// 兄弟文件 <version>.trace.json（待修补 #24）。
//
// 这些是易变信息，绝不能进记录正文——正文才是内容寻址的锚。留痕本允许随探测波动，
// 故写失败只告警，不改变记录入库的结论（记录是否 changed / 是否被拒由 Store.Put 定）。
func writeTraceSibling(recordPath string, rec *modelreg.Record, rep *modelreg.ProbeReport) {
	p, err := modelreg.WriteProbeTraceSibling(recordPath, rec, rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 留痕兄弟文件写入失败：%v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "留痕已写：%s\n", p)
}

// writeCapabilitySibling 把能力快照（能力断言 + 证据 + 生成时间 + 端点）写成记录旁的
// 兄弟文件 <version>.capabilities.json。
//
// 与留痕兄弟文件同理：能力快照是"它现在能干什么"，**允许**随探测刷新（端点/引擎一变就该更新），
// 故写失败只告警，不改变记录入库的结论；记录正文一字未动（防覆盖保护不受影响）。
func writeCapabilitySibling(recordPath string, rec *modelreg.Record, rep *modelreg.ProbeReport) {
	p, err := modelreg.WriteCapabilitySnapshot(recordPath, rec, rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ capability snapshot write failed: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "capability snapshot written: %s\n", p)
}

// looksLikeDir 判定 --out 给的是目录还是文件（开工方案 §七 批 3 要求
// `probe --out` 能落到 `~/.zerg/models/manifests/`）：
//   - 以 "/" 结尾 → 目录（可以是还没创建的目录）
//   - 已存在的目录 → 目录
//
// 其余一律当文件路径（批 2 的行为不变）。
func looksLikeDir(p string) bool {
	if strings.HasSuffix(p, "/") {
		return true
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func cmdList(args []string) int {
	root, manifests := "", ""
	asJSON := false

	needVal := func(i *int, flag string) (string, bool) {
		if *i+1 >= len(args) {
			fmt.Fprintf(os.Stderr, "%s 需要一个值\n", flag)
			return "", false
		}
		*i++
		return args[*i], true
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			asJSON = true
		case a == "--root":
			v, ok := needVal(&i, "--root")
			if !ok {
				return 3
			}
			root = v
		case strings.HasPrefix(a, "--root="):
			root = strings.TrimPrefix(a, "--root=")
		case a == "--manifests":
			v, ok := needVal(&i, "--manifests")
			if !ok {
				return 3
			}
			manifests = v
		case strings.HasPrefix(a, "--manifests="):
			manifests = strings.TrimPrefix(a, "--manifests=")
		default:
			fmt.Fprintf(os.Stderr, "list 未知参数: %s\n", a)
			return 3
		}
	}

	st := modelreg.NewStore(root)
	if manifests != "" {
		st = modelreg.NewStoreAtManifests(manifests)
	}
	rows, err := st.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取模型目录失败: %v\n", err)
		return 3
	}

	if asJSON {
		out, _ := json.MarshalIndent(struct {
			ManifestsDir string                  `json:"manifests_dir"`
			Count        int                     `json:"count"`
			Records      []modelreg.StoredRecord `json:"records"`
		}{st.ManifestsDir(), len(rows), rows}, "", "  ")
		fmt.Println(string(out))
		return 0
	}

	fmt.Printf("模型目录（manifests）：%s\n", st.ManifestsDir())
	if len(rows) == 0 {
		fmt.Println("（空目录：还没有任何记录。用 `zerg-model probe <路径> --store` 入目录）")
		return 0
	}
	fmt.Printf("%-28s %-16s %-13s %-6s %-24s %-8s %s\n", "ID", "VERSION", "COMMERCIAL", "FILES", "CAPABILITIES", "ERR/WARN", "PATH")
	totalErr, totalWarn, notEligible := 0, 0, 0
	for _, r := range rows {
		caps := strings.Join(r.Capabilities, ",")
		if caps == "" {
			caps = "-"
		}
		fmt.Printf("%-28s %-16s %-13s %-6d %-24s %d/%-6d %s\n", r.ID, r.Version, orDash(r.Commercial), r.Files, caps, r.Errors, r.Warns, r.Path)
		if r.Err != "" {
			fmt.Printf("%-28s %s\n", "", "读取失败："+r.Err)
		}
		totalErr += r.Errors
		totalWarn += r.Warns
		if !r.DefaultEligible {
			notEligible++
		}
	}
	fmt.Printf("—— 共 %d 条记录；error %d / warn %d；commercial != yes 的 %d 条不得作为默认项（标准 §五 红线）\n", len(rows), totalErr, totalWarn, notEligible)
	if totalErr > 0 {
		fmt.Println("   目录里有不合标准的记录（标准 §十二.1 不该留在目录里）——需人工处置")
		return 2
	}
	return 0
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
