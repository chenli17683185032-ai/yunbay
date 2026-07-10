package controller

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVideoTaskTokenSettlementUsesAtomicGroupRatioAccessor(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "task_video.go", nil, 0)
	require.NoError(t, err)

	var updateVideoSingleTask *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "updateVideoSingleTask" {
			updateVideoSingleTask = function
			break
		}
	}
	require.NotNil(t, updateVideoSingleTask)

	atomicAccessorCalls := 0
	legacyAccessorCalls := make([]string, 0, 2)
	ast.Inspect(updateVideoSingleTask.Body, func(node ast.Node) bool {
		resultSelector, ok := node.(*ast.SelectorExpr)
		if ok && resultSelector.Sel.Name == "GroupRatio" {
			call, ok := resultSelector.X.(*ast.CallExpr)
			if ok {
				accessor, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				packageName, packageOK := accessor.X.(*ast.Ident)
				if packageOK && packageName.Name == "ratio_setting" && accessor.Sel.Name == "GetGroupRatioInfo" {
					atomicAccessorCalls++
					require.Len(t, call.Args, 2)
					for _, argument := range call.Args {
						identifier, ok := argument.(*ast.Ident)
						require.True(t, ok)
						require.Equal(t, "group", identifier.Name)
					}
				}
			}
		}

		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		packageName, ok := selector.X.(*ast.Ident)
		if !ok || packageName.Name != "ratio_setting" {
			return true
		}
		switch selector.Sel.Name {
		case "GetGroupRatio", "GetGroupGroupRatio":
			legacyAccessorCalls = append(legacyAccessorCalls, selector.Sel.Name)
		}
		return true
	})

	require.Equal(t, 1, atomicAccessorCalls)
	require.Empty(t, legacyAccessorCalls)
}
