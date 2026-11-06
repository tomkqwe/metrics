package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// analyzer forbids builtin panic and restricts log.Fatal, log.Fatalf,
// log.Fatalln and os.Exit to the main function of package main.
var analyzer = &analysis.Analyzer{
	Name: "noexit",
	Doc:  "report panic and process termination outside the main function of package main",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Walk(visitor{pass: pass}, file)
	}
	return nil, nil
}

type visitor struct {
	pass   *analysis.Pass
	inMain bool
}

func (v visitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		v.inMain = v.pass.Pkg.Name() == "main" && n.Name.Name == "main" && n.Recv == nil
	case *ast.FuncLit:
		// A closure is a separate function, even when declared inside main.
		v.inMain = false
	case *ast.CallExpr:
		var obj types.Object
		switch fun := ast.Unparen(n.Fun).(type) {
		case *ast.Ident:
			obj = v.pass.TypesInfo.Uses[fun]
		case *ast.SelectorExpr:
			obj = v.pass.TypesInfo.Uses[fun.Sel]
		}
		if builtin, ok := obj.(*types.Builtin); ok && builtin.Name() == "panic" {
			v.pass.Reportf(n.Pos(), "use of builtin panic is forbidden")
		}
		fn, ok := obj.(*types.Func)
		if !ok || v.inMain || fn.Pkg() == nil {
			break
		}
		// Only package functions are restricted, not methods with the same name.
		if fn.Type().(*types.Signature).Recv() != nil {
			break
		}
		pkg, name := fn.Pkg().Path(), fn.Name()
		if pkg == "os" && name == "Exit" || pkg == "log" && (name == "Fatal" || name == "Fatalf" || name == "Fatalln") {
			v.pass.Reportf(n.Pos(), "%s.%s is forbidden outside main.main", pkg, name)
		}
	}
	return v
}
