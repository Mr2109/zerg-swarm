package api

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// chat_callback_nilguard_test.go — 可机检的静态守卫（第 ④ 层，AST 扫描）
//
// 规则：本包（非 `_test.go` 的生产文件）里，**func 类型参数**（= 可选回调，如 onDelta）如果被
// **直接调用**，该调用点必须满足其一：
//   - 被 `if cb != nil { … }` 包住（Then 体内）；或
//   - 调用之前有 `if cb == nil { return / panic / continue / break … }` 的早退守卫；或
//   - 显式登记在 requiredCallbacks 里（= 声明"这是必填参数，不是可选回调"）。
//
// 事故依据（2026-09-16 实测，栈：chat_handlers.go:860 func3.1 ← chat_infer.go:271 ← run.go:233）：
// 包装可选回调时**无条件调用**，等于把被包装者的 nil 保护抹掉 ⇒ 收到 nil 的一方空指针 panic
// ⇒ handler 半路死掉（客户端断流 + 助手回复不落库 + HTTP 仍 200）。这条守卫让同一类回归在
// `go test ./internal/api/` 里就红，不必等到生产再崩。
//
// 变异自证：在任一 api 生产文件里写一句裸调用（如 `onDelta("output", "x")`）⇒ 本用例必红。
func TestOptionalCallbackCallsAreNilGuarded(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("无法读取包目录：%v", err)
	}
	scanned, violations := 0, 0
	used := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, name, nil, 0)
		if perr != nil {
			t.Fatalf("%s 解析失败（守卫无法覆盖）：%v", name, perr)
		}
		scanned++
		for _, c := range collectCallables(f) {
			names := funcTypedParamNames(c.params)
			if len(names) == 0 {
				continue
			}
			ast.Inspect(c.body, func(m ast.Node) bool {
				ce, ok := m.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := ce.Fun.(*ast.Ident)
				if !ok || !names[id.Name] {
					return true
				}
				if nilGuarded(c.body, id.Name, ce.Pos()) {
					return true
				}
				key := fmt.Sprintf("%s:%s:%s", name, c.owner, id.Name)
				if _, ok := requiredCallbacks[key]; ok {
					used[key] = true
					return true
				}
				violations++
				t.Errorf("%s:%d: 可选回调 %q（%s）被直接调用但未判 nil —— 包装可选回调必须保留 nil 语义"+
					"（真 nil 跳过 / 非 nil 原样透传 / 或改走 loopcore.EmitDelta）；若它是必填参数，"+
					"请在 requiredCallbacks 里显式登记并写明理由",
					name, fset.Position(ce.Pos()).Line, id.Name, c.owner)
				return true
			})
		}
	}
	if scanned == 0 {
		t.Fatal("一个生产文件都没扫到 —— 守卫形同虚设（目录/后缀过滤写错了？）")
	}
	stale := []string{}
	for k := range requiredCallbacks {
		if !used[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	for _, k := range stale {
		t.Errorf("requiredCallbacks 条目已失效（不再匹配任何裸调用）：%s —— 请删除或修正", k)
	}
	t.Logf("AST 守卫：扫描生产文件 %d 个，违规 %d 处，必填白名单 %d 条", scanned, violations, len(requiredCallbacks))
}

// requiredCallbacks — 「必填参数」白名单（**不是**可选回调 ⇒ 无需判 nil）。
// 加进这里等于显式声明「此参数必填、调用方必须给非 nil」；未登记又未判 nil 的直接调用一律红。
// 键：文件名:所属函数名:参数名
var requiredCallbacks = map[string]string{
	"zerg_controlled_loop.go:RunStep:callModel": "必填：模型调用函数由调用方注入，缺它本方法无意义" +
		"（调用点 zerg_flow_executor.go:269/334 与全部用例都传函数字面量）——不是可选回调",
}

// callable — 一个函数或函数字面量（owner = 所属函数名，用于白名单键与报错定位）
type callable struct {
	owner  string
	body   *ast.BlockStmt
	params *ast.FieldList
}

// collectCallables — 收集文件里全部函数/函数字面量（字面量归属最近的函数名）。
func collectCallables(f *ast.File) []callable {
	var out []callable
	var decls []*ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
			decls = append(decls, fd)
		}
	}
	ownerOf := func(p token.Pos) string {
		best := ""
		var bestSpan token.Pos
		for _, d := range decls {
			if d.Pos() <= p && p <= d.End() {
				if span := d.End() - d.Pos(); best == "" || span < bestSpan {
					best, bestSpan = d.Name.Name, span
				}
			}
		}
		if best == "" {
			return "<lit>"
		}
		return best + ".lit"
	}
	for _, d := range decls {
		out = append(out, callable{owner: d.Name.Name, body: d.Body, params: d.Type.Params})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok && lit.Body != nil {
			out = append(out, callable{owner: ownerOf(lit.Pos()), body: lit.Body, params: lit.Type.Params})
		}
		return true
	})
	return out
}

// funcTypedParamNames — 参数表里「类型直接写成 func(...)」的参数名集合（可选回调候选）。
// 只认直接 func 类型：命名类型/接口要判 typed-nil，不属于本规则的适用范围。
func funcTypedParamNames(params *ast.FieldList) map[string]bool {
	out := map[string]bool{}
	if params == nil {
		return out
	}
	for _, f := range params.List {
		if _, ok := f.Type.(*ast.FuncType); !ok {
			continue
		}
		for _, nm := range f.Names {
			if nm.Name != "" && nm.Name != "_" {
				out[nm.Name] = true
			}
		}
	}
	return out
}

// nilGuarded — 调用点是否被 nil 守卫支配。
func nilGuarded(body *ast.BlockStmt, name string, call token.Pos) bool {
	guarded := false
	ast.Inspect(body, func(n ast.Node) bool {
		ifs, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		// 形式一：if name != nil { … 调用在这里面 … }
		if isNilCompare(ifs.Cond, name, token.NEQ) && ifs.Body.Pos() < call && call < ifs.Body.End() {
			guarded = true
		}
		// 形式二：if name == nil { …终止… }  调用在其后（早退守卫）
		if isNilCompare(ifs.Cond, name, token.EQL) && ifs.End() < call && blockTerminates(ifs.Body) {
			guarded = true
		}
		return true
	})
	return guarded
}

// isNilCompare — cond 是否为 `name <op> nil` 或 `nil <op> name`。
func isNilCompare(cond ast.Expr, name string, op token.Token) bool {
	be, ok := cond.(*ast.BinaryExpr)
	if !ok || be.Op != op {
		return false
	}
	lhsName := isIdentNamed(be.X, name) && isNilIdent(be.Y)
	rhsName := isIdentNamed(be.Y, name) && isNilIdent(be.X)
	return lhsName || rhsName
}

func isIdentNamed(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

func isNilIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}

// blockTerminates — 块内是否有终止语句（早退守卫判定；近似即可，宁可漏判不可误判为"已守卫"）。
func blockTerminates(b *ast.BlockStmt) bool {
	if b == nil {
		return false
	}
	terminates := false
	ast.Inspect(b, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.ReturnStmt, *ast.BranchStmt:
			terminates = true
		case *ast.ExprStmt:
			if ce, ok := s.X.(*ast.CallExpr); ok {
				if id, ok := ce.Fun.(*ast.Ident); ok && id.Name == "panic" {
					terminates = true
				}
			}
		}
		return true
	})
	return terminates
}
