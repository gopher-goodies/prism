package tools

import (
	"fmt"
	"go/ast"
	"go/token"
)

var (
	profilerImports = []string{"prismProfiler github.com/geckoboard/prism/profiler"}
	sinkImports     = []string{"prismSink github.com/geckoboard/prism/profiler/sink"}
)

// InjectProfilerBootstrap returns a PatchFunc that injects our profiler init code the main function of the target package.
func InjectProfilerBootstrap(profileDir, profileLabel string) PatchFunc {
	return func(cgNode *CallGraphNode, fnDeclNode *ast.BlockStmt) (modifiedAST bool, extraImports []string) {
		imports := append(profilerImports, sinkImports...)
		fnDeclNode.List = append(
			[]ast.Stmt{
				&ast.ExprStmt{
					X: &ast.BasicLit{
						ValuePos: token.NoPos,
						Kind:     token.STRING,
						Value:    fmt.Sprintf("prismProfiler.Init(prismSink.NewFileSink(%q), %q)", profileDir, profileLabel),
					},
				},
				&ast.ExprStmt{
					X: &ast.BasicLit{
						ValuePos: token.NoPos,
						Kind:     token.STRING,
						Value:    `defer prismProfiler.Shutdown()`,
					},
				},
			},
			fnDeclNode.List...,
		)

		return true, imports
	}
}

// InjectProfiler returns a PatchFunc that injects our profiler instrumentation code in all
// functions that are reachable from the profile targets that the user specified.
func InjectProfiler() PatchFunc {
	return func(cgNode *CallGraphNode, fnDeclNode *ast.BlockStmt) (modifiedAST bool, extraImports []string) {
		enterFn, leaveFn := profileFnName(cgNode.Depth)

		// Append our instrumentation calls to the top of the function
		fnDeclNode.List = append(
			[]ast.Stmt{
				&ast.ExprStmt{
					X: &ast.BasicLit{
						ValuePos: token.NoPos,
						Kind:     token.STRING,
						Value:    fmt.Sprintf(`prismProfiler.%s("%s")`, enterFn, cgNode.Name),
					},
				},
				&ast.ExprStmt{
					X: &ast.BasicLit{
						ValuePos: token.NoPos,
						Kind:     token.STRING,
						Value:    fmt.Sprintf(`defer prismProfiler.%s()`, leaveFn),
					},
				},
			},
			fnDeclNode.List...,
		)

		return true, profilerImports
	}
}

// Return the appropriate profiler enter/exit function names depending on whether
// a profile target is a user-specified target (depth=0) or a target discovered
// by analyzing the callgraph from a user-specified target.
func profileFnName(depth int) (enterFn, leaveFn string) {
	if depth == 0 {
		return "BeginProfile", "EndProfile"
	}

	return "Enter", "Leave"
}
