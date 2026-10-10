package tools

import (
	"go/ast"
	"go/token"
)

// This is a local syntactic check, not interprocedural ownership analysis.
// Binding identity prevents a different or shadowed variable's Close from
// hiding a leak. Ambiguous control flow remains a potential leak.
func closedFileCalls(file *ast.File) map[*ast.CallExpr]bool {
	closed := make(map[*ast.CallExpr]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, stmt := range block.List {
			check := func(names []ast.Expr, values []ast.Expr) {
				if len(names) == 0 || len(values) != 1 {
					return
				}
				call, ok := values[0].(*ast.CallExpr)
				if !ok || !isFileOpenFunction(call) {
					return
				}
				resource, ok := names[0].(*ast.Ident)
				if !ok || resource.Obj == nil || resource.Name == "_" {
					return
				}
				var errObj *ast.Object
				if len(names) > 1 {
					if ident, ok := names[1].(*ast.Ident); ok {
						errObj = ident.Obj
					}
				}
				closed[call] = releasesFile(block.List[i+1:], resource.Obj, errObj)
			}
			switch stmt := stmt.(type) {
			case *ast.AssignStmt:
				check(stmt.Lhs, stmt.Rhs)
			case *ast.DeclStmt:
				if decl, ok := stmt.Decl.(*ast.GenDecl); ok {
					for _, spec := range decl.Specs {
						if value, ok := spec.(*ast.ValueSpec); ok {
							names := make([]ast.Expr, len(value.Names))
							for j, name := range value.Names {
								names[j] = name
							}
							check(names, value.Values)
						}
					}
				}
			}
		}
		return true
	})
	return closed
}

func sameBinding(expr ast.Expr, object *ast.Object) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && object != nil && ident.Obj == object
}

func closesFile(call *ast.CallExpr, resource *ast.Object) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Close" && len(call.Args) == 0 && sameBinding(sel.X, resource)
}

func releasesFile(stmts []ast.Stmt, resource, errObj *ast.Object) bool {
	for i, stmt := range stmts {
		var calls []ast.Expr
		switch stmt := stmt.(type) {
		case *ast.DeferStmt:
			if closesFile(stmt.Call, resource) {
				return true // receiver is evaluated when defer executes
			}
			if literal, ok := stmt.Call.Fun.(*ast.FuncLit); ok && len(stmt.Call.Args) == 0 && !hasFileHazard(&ast.BlockStmt{List: stmts[i+1:]}, resource, nil, false) {
				if releasesFile(literal.Body.List, resource, nil) {
					return true
				}
			}
		case *ast.ExprStmt:
			calls = []ast.Expr{stmt.X}
		case *ast.AssignStmt:
			calls = stmt.Rhs
		case *ast.ReturnStmt:
			calls = stmt.Results
		case *ast.BlockStmt:
			if releasesFile(stmt.List, resource, errObj) {
				return true
			}
		case *ast.IfStmt:
			// On the opening call's error/nil branch there is no live handle.
			if stmt.Init == nil && stmt.Else == nil && failedOpenGuard(stmt.Cond, resource, errObj) && !hasFileHazard(stmt.Body, resource, errObj, false) {
				continue
			}
			if stmt.Init == nil && releasesFile(stmt.Body.List, resource, errObj) {
				if other, ok := stmt.Else.(*ast.BlockStmt); ok && releasesFile(other.List, resource, errObj) {
					return true
				}
			}
		}
		for _, expr := range calls {
			if call, ok := expr.(*ast.CallExpr); ok && closesFile(call, resource) {
				return true
			}
		}
		if hasFileHazard(stmt, resource, errObj, true) {
			return false
		}
	}
	return false
}

func failedOpenGuard(expr ast.Expr, resource, errObj *ast.Object) bool {
	condition, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	left, right := condition.X, condition.Y
	if ident, ok := left.(*ast.Ident); ok && ident.Name == "nil" {
		left, right = right, left
	}
	nilIdent, ok := right.(*ast.Ident)
	return ok && nilIdent.Name == "nil" && ((condition.Op == token.NEQ && sameBinding(left, errObj)) || (condition.Op == token.EQL && sameBinding(left, resource)))
}

// Ignore uninvoked nested function bodies. Exits before a proven Close and
// reassignment of the resource/error make a later Close/guard insufficient.
func hasFileHazard(node ast.Node, resource, errObj *ast.Object, exits bool) bool {
	hazard := false
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt, *ast.BranchStmt:
			hazard = hazard || exits
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				hazard = hazard || sameBinding(lhs, resource) || sameBinding(lhs, errObj)
			}
		}
		return !hazard
	})
	return hazard
}
