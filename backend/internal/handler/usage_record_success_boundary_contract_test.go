package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatCompletionUsageSubmissionStaysAfterForwardErrorReturn(t *testing.T) {
	tests := []struct {
		file     string
		function string
		submit   string
	}{
		{file: "openai_chat_completions.go", function: "ChatCompletions", submit: "submitOpenAIUsageRecordTask"},
		{file: "gateway_handler_chat_completions.go", function: "ChatCompletions", submit: "submitUsageRecordTask"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filepath.Join(".", tt.file), nil, 0)
			require.NoError(t, err)

			fn := findHandlerFunction(file, tt.function)
			require.NotNil(t, fn)

			var submitPos token.Pos
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if ok && selectorName(call.Fun) == tt.submit {
					submitPos = call.Pos()
					return false
				}
				return true
			})
			require.NotEqual(t, token.NoPos, submitPos)

			var errorReturnPositions []token.Pos
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				ifStmt, ok := node.(*ast.IfStmt)
				if !ok || !conditionChecksErr(ifStmt.Cond) || !blockHasReturn(ifStmt.Body) {
					return true
				}
				errorReturnPositions = append(errorReturnPositions, ifStmt.Pos())
				return true
			})

			require.NotEmpty(t, errorReturnPositions)
			for _, pos := range errorReturnPositions {
				require.Less(t, pos, submitPos, "forward error returns must remain before usage submission")
			}
		})
	}
}

func findHandlerFunction(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func selectorName(expr ast.Expr) string {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return selector.Sel.Name
}

func conditionChecksErr(expr ast.Expr) bool {
	matched := false
	ast.Inspect(expr, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && ident.Name == "err" {
			matched = true
			return false
		}
		return true
	})
	return matched
}

func blockHasReturn(block *ast.BlockStmt) bool {
	found := false
	ast.Inspect(block, func(node ast.Node) bool {
		if _, ok := node.(*ast.ReturnStmt); ok {
			found = true
			return false
		}
		return true
	})
	return found
}
