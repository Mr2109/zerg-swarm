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
	fmt.Println("  zerg-model probe  <路径|端点URL> [--out <record.json>] [--json]")
	fmt.Println("                    [--endpoint <URL>] [--engine llama.cpp|vllm|ollama] [--id <id>] [--timeout 60s]")
	fmt.Println("                                                          # 探测模型并生成登记记录（默认只打印，不写盘）")
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
	var target, out, endpoint, engine, id string
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
	fmt.Fprintf(os.Stderr, "生成记录：id=%s digest=%s files=%d capabilities=%d\n", rec.ID, rec.Digest, len(rec.Files), len(rec.Capabilities))
	fmt.Fprintf(os.Stderr, "自校验：error %d 条，warn %d 条\n", nErr, nWarn)
	for _, f := range findings {
		mark := "✗"
		if f.Level == "warn" {
			mark = "⚠️"
		}
		fmt.Fprintf(os.Stderr, "%s [%s] %s：%s\n", mark, f.Level, f.Field, f.Detail)
	}

	if asJSON {
		payload, _ := json.MarshalIndent(struct {
			Record     *modelreg.Record     `json:"record"`
			ProbeTrace []modelreg.Trace     `json:"probe_trace"`
			Findings   []modelreg.Finding   `json:"findings"`
			Errors     int                  `json:"errors"`
			Warnings   int                  `json:"warnings"`
		}{rec, rep.Traces, findings, nErr, nWarn}, "", "  ")
		fmt.Println(string(payload))
	} else if out == "" {
		// 默认只打印，不写盘
		fmt.Println(string(recJSON))
	}

	if out != "" {
		if err := os.WriteFile(out, append(recJSON, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "写记录失败: %v\n", err)
			return 3
		}
		fmt.Fprintf(os.Stderr, "已写入：%s\n", out)
	}

	if nErr > 0 {
		return 2
	}
	return 0
}
