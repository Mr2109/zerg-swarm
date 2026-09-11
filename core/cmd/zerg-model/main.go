// zerg-model —— 模型登记记录的工具（虫族 v2.5.9「多模型接入」批 1）。
//
// 用法：
//
//	zerg-model verify <record.json> [--strict] [--json]
//	zerg-model probe  ...   （批 2 实现，当前为占位）
//
// 退出码：0 成功 / 1 探测失败 / 2 记录不合标准 / 3 参数或环境错误 / 4 引擎不可达
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

func usage() {
	fmt.Println("用法:")
	fmt.Println("  zerg-model verify <record.json> [--strict] [--json]   # 校验一条模型登记记录是否符合标准")
	fmt.Println("  zerg-model probe  <路径|模型ID|端点>                   # 探测并生成记录（批 2，未实现）")
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
		fmt.Fprintln(os.Stderr, "probe 属批 2（五探测器），尚未实现；请先看 docs/01-设计/开工方案-模型探测与校验-20260911.md")
		os.Exit(3)
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
