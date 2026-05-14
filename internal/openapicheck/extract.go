// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package openapicheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const clientPkgPath = "github.com/bogware/terraform-provider-langsmith/internal/client"

var clientHTTPMethods = map[string]string{
	"Get":             "GET",
	"Post":            "POST",
	"PostWithQuery":   "POST",
	"Put":             "PUT",
	"Patch":           "PATCH",
	"Delete":          "DELETE",
	"DeleteWithQuery": "DELETE",
	"DeleteWithBody":  "DELETE",
}

// Usage is one client HTTP call the checker extracted from provider code.
type Usage struct {
	HTTPMethod string
	Pattern    []Segment
	Position   token.Position // file:line:col of the CallExpr
}

// UnresolvedPathArg records a client call whose path expression could not be
// analyzed (CI should fail so the call site is made explicit for the contract checker).
type UnresolvedPathArg struct {
	Position token.Position
	Reason   string
}

// Extract loads package patterns (e.g. ./internal/provider) and returns all
// LangSmith client HTTP path usages.
func Extract(dir string, patterns []string) ([]Usage, []UnresolvedPathArg, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule | packages.NeedDeps,
		Dir: dir,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, nil, fmt.Errorf("load packages: %w", err)
	}
	var errs []string
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			for _, e := range p.Errors {
				errs = append(errs, e.Error())
			}
		}
	}
	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("package load errors:\n%s", strings.Join(errs, "\n"))
	}

	var out []Usage
	var unresolved []UnresolvedPathArg
	for _, p := range pkgs {
		if len(p.Syntax) == 0 {
			continue
		}
		for _, f := range p.Syntax {
			info := p.TypesInfo
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					httpMethod, ok := clientHTTPMethods[sel.Sel.Name]
					if !ok {
						return true
					}
					selTyp := info.TypeOf(sel.X)
					if !isClientPointer(selTyp) {
						return true
					}
					if len(call.Args) < 2 {
						return true
					}
					pathArg := call.Args[1]
					patterns, ok := pathPatternsForExpr(fd, pathArg, info)
					if !ok || len(patterns) == 0 {
						unresolved = append(unresolved, UnresolvedPathArg{
							Position: p.Fset.Position(pathArg.Pos()),
							Reason:   "could not resolve path expression to a static template (string literal, + chain, fmt.Sprintf, or local variable built from those)",
						})
						return true
					}
					pos := p.Fset.Position(call.Pos())
					for _, pat := range patterns {
						out = append(out, Usage{
							HTTPMethod: httpMethod,
							Pattern:    pat,
							Position:   pos,
						})
					}
					return true
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Position.Filename != out[j].Position.Filename {
			return out[i].Position.Filename < out[j].Position.Filename
		}
		if out[i].Position.Line != out[j].Position.Line {
			return out[i].Position.Line < out[j].Position.Line
		}
		return out[i].Position.Column < out[j].Position.Column
	})
	return out, unresolved, nil
}

func isClientPointer(t types.Type) bool {
	ptr, ok := t.Underlying().(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Name() == "Client" && obj.Pkg() != nil && obj.Pkg().Path() == clientPkgPath
}

func pathPatternsForExpr(fn *ast.FuncDecl, e ast.Expr, info *types.Info) ([][]Segment, bool) {
	if call, ok := e.(*ast.CallExpr); ok {
		if segs, ok := pathFromSprintf(call); ok {
			return [][]Segment{segs}, true
		}
	}
	if id, ok := e.(*ast.Ident); ok {
		obj, ok := info.Uses[id]
		if !ok {
			obj = info.ObjectOf(id)
		}
		if v, ok := obj.(*types.Var); ok {
			return patternsForVar(fn, v, info)
		}
	}
	if segs, ok := pathFromAddExpr(e, info); ok {
		return [][]Segment{segs}, true
	}
	return nil, false
}

func patternsForVar(fn *ast.FuncDecl, target *types.Var, info *types.Info) ([][]Segment, bool) {
	var rhss []ast.Expr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			lid, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			if info.ObjectOf(lid) != target {
				continue
			}
			var rhs ast.Expr
			if len(as.Rhs) == 1 {
				rhs = as.Rhs[0]
			} else if i < len(as.Rhs) {
				rhs = as.Rhs[i]
			}
			if rhs != nil {
				rhss = append(rhss, rhs)
			}
		}
		return true
	})
	if len(rhss) == 0 {
		return nil, false
	}
	var out [][]Segment
	for _, rhs := range rhss {
		pats, ok := pathPatternsForExpr(fn, rhs, info)
		if !ok {
			return nil, false
		}
		out = append(out, pats...)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// ValidateUsages checks every usage against the OpenAPI index. On failure it
// returns errors with file positions and hints.
func ValidateUsages(idx SpecIndex, usages []Usage) []error {
	var errs []error
nextUsage:
	for _, u := range usages {
		pathsForMethod := idx[u.HTTPMethod]
		if len(pathsForMethod) == 0 {
			errs = append(errs, fmt.Errorf("%s: %s %s — OpenAPI spec defines no %s operations",
				u.Position.String(), u.HTTPMethod, formatPattern(u.Pattern), u.HTTPMethod))
			continue
		}
		for specPath := range pathsForMethod {
			specSegs := ParseOpenAPISegments(specPath)
			if PatternMatchesOpenAPI(u.Pattern, specSegs) {
				continue nextUsage
			}
		}
		errs = append(errs, fmt.Errorf("%s: %s %s — no matching OpenAPI path for this HTTP method (update the provider or refresh the pinned spec)",
			u.Position.String(), u.HTTPMethod, formatPattern(u.Pattern)))
	}
	return errs
}

func formatPattern(p []Segment) string {
	if len(p) == 0 {
		return "/"
	}
	var b strings.Builder
	b.WriteByte('/')
	for i, s := range p {
		if i > 0 {
			b.WriteByte('/')
		}
		if s.Wild {
			b.WriteString("{…}")
		} else {
			b.WriteString(s.Lit)
		}
	}
	return b.String()
}
