// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package openapicheck

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

type stringFrag struct {
	s   string
	dyn bool
}

func collectAddFrags(e ast.Expr, info *types.Info) ([]stringFrag, bool) {
	switch e := e.(type) {
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return nil, false
		}
		left, ok := collectAddFrags(e.X, info)
		if !ok {
			return nil, false
		}
		right, ok := collectAddFrags(e.Y, info)
		if !ok {
			return nil, false
		}
		return append(left, right...), true
	case *ast.ParenExpr:
		return collectAddFrags(e.X, info)
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return nil, false
		}
		s, err := strconv.Unquote(e.Value)
		if err != nil {
			return nil, false
		}
		return []stringFrag{{s: s}}, true
	default:
		if isEmptyStringKnown(e, info) {
			return []stringFrag{{s: ""}}, true
		}
		return []stringFrag{{dyn: true}}, true
	}
}

func isEmptyStringKnown(e ast.Expr, info *types.Info) bool {
	if e == nil {
		return false
	}
	if be, ok := e.(*ast.BasicLit); ok && be.Kind == token.STRING {
		s, err := strconv.Unquote(be.Value)
		return err == nil && s == ""
	}
	tv, ok := info.Types[e]
	if !ok || tv.Value == nil {
		return false
	}
	if tv.Value.Kind() != constant.String {
		return false
	}
	return constant.StringVal(tv.Value) == ""
}

func mergeAdjacentStringFrags(frags []stringFrag) []stringFrag {
	if len(frags) == 0 {
		return nil
	}
	out := make([]stringFrag, 0, len(frags))
	cur := frags[0]
	for i := 1; i < len(frags); i++ {
		f := frags[i]
		if !cur.dyn && !f.dyn {
			cur.s += f.s
			continue
		}
		out = append(out, cur)
		cur = f
	}
	out = append(out, cur)
	return out
}

func fragsToSegments(frags []stringFrag) []Segment {
	frags = mergeAdjacentStringFrags(frags)
	var segs []Segment
	for _, f := range frags {
		if f.dyn {
			segs = append(segs, Segment{Wild: true})
			continue
		}
		p := strings.Trim(f.s, "/")
		if p == "" {
			continue
		}
		for _, part := range strings.Split(p, "/") {
			if part == "" {
				continue
			}
			segs = append(segs, Segment{Lit: part})
		}
	}
	return segs
}

func pathFromAddExpr(e ast.Expr, info *types.Info) ([]Segment, bool) {
	frags, ok := collectAddFrags(e, info)
	if !ok {
		return nil, false
	}
	segs := fragsToSegments(frags)
	return segs, len(segs) > 0
}

// pathFromSprintf parses fmt.Sprintf("...%s...", ...) where the format string is a literal.
// Supported verbs: %s %v %d %q %x (each consumes one wildcard path segment).
func pathFromSprintf(call *ast.CallExpr) ([]Segment, bool) {
	if len(call.Args) == 0 {
		return nil, false
	}
	fun, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || fun.Sel.Name != "Sprintf" {
		return nil, false
	}
	pkgID, ok := fun.X.(*ast.Ident)
	if !ok || pkgID.Name != "fmt" {
		return nil, false
	}
	formatLit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || formatLit.Kind != token.STRING {
		return nil, false
	}
	format, err := strconv.Unquote(formatLit.Value)
	if err != nil {
		return nil, false
	}
	frags, err := parseSprintfFormat(format)
	if err != nil {
		return nil, false
	}
	segs := fragsToSegments(frags)
	return segs, len(segs) > 0
}

func parseSprintfFormat(format string) ([]stringFrag, error) {
	var frags []stringFrag
	var lit strings.Builder
	i := 0
	flushLit := func() {
		if lit.Len() == 0 {
			return
		}
		frags = append(frags, stringFrag{s: lit.String()})
		lit.Reset()
	}
	for i < len(format) {
		if format[i] != '%' {
			lit.WriteByte(format[i])
			i++
			continue
		}
		if i+1 >= len(format) {
			return nil, fmt.Errorf("truncated %% in format string")
		}
		if format[i+1] == '%' {
			lit.WriteByte('%')
			i += 2
			continue
		}
		flushLit()
		i++ // skip '%'
		for i < len(format) {
			c := format[i]
			if c == '*' {
				return nil, fmt.Errorf("unsupported %%* in openapi path format")
			}
			if (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '+' || c == '#' || c == ' ' {
				i++
				continue
			}
			break
		}
		if i >= len(format) {
			return nil, fmt.Errorf("unterminated conversion in format string")
		}
		verb := format[i]
		i++
		switch verb {
		case 's', 'v', 'd', 'q', 'x', 'X', 'o', 'b', 'c', 'U', 'e', 'E', 'f', 'F', 'g', 'G', 'p', 'T':
			frags = append(frags, stringFrag{dyn: true})
		default:
			return nil, fmt.Errorf("unsupported fmt verb %%%c in path format", verb)
		}
	}
	flushLit()
	return frags, nil
}
