package bv

// Source: src/ast/simplifiers/bv_slice.cpp apply_subst post-order cache.

import "github.com/glycerine/g3/pkg/ast"

type bvSliceVisit struct {
	h        ast.ASTHandle
	expanded bool
}

func (f *bvSliceFrame) rewrite(root ast.ASTHandle) (ast.ASTHandle, error) {
	m := f.owner.manager
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	stack := []bvSliceVisit{{h: root}}
	scratch, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTHandle{}, e
	}
	defer m.DelRef(&scratch)
	for len(stack) > 0 {
		i := len(stack) - 1
		v := stack[i]
		stack = stack[:i]
		if _, ok := memo[v.h]; ok {
			continue
		}
		kind, e := m.GetKind(v.h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if kind != ast.ASTApp {
			memo[v.h] = v.h
			continue
		}
		if !v.expanded {
			stack = append(stack, bvSliceVisit{h: v.h, expanded: true})
			n, e := m.GetNumArgs(v.h)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			for j := n; j > 0; j-- {
				a, e := m.GetArg(v.h, j-1)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				if _, ok := memo[a]; !ok {
					stack = append(stack, bvSliceVisit{h: a})
				}
			}
			continue
		}
		n, e := m.GetNumArgs(v.h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args := make([]ast.ASTHandle, n)
		changed := false
		for j := uint32(0); j < n; j++ {
			a, e := m.GetArg(v.h, j)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			args[j] = memo[a]
			changed = changed || args[j] != a
		}
		result := v.h
		if changed {
			d, e := m.GetDecl(v.h)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if e = f.owner.theory.MakeApp(d, ast.SliceArguments(args), scratch); e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = m.RefGet(scratch)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			// The retained C++ th_rewriter preserves explicit slice concats here.
			// Go's stronger normalization can erase them, or even rewrite their
			// parent to a different shape. Retain the raw application whenever
			// that happens so every registered occurrence remains sliced.
			raw, e := m.MkApp(d, args)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if result != raw {
				result = raw
			}
			if _, e = f.hold(result); e != nil {
				return ast.ASTHandle{}, e
			}
		}
		points := f.points(v.h)
		if len(points) > 0 {
			width, e := m.BitVectorSize(result)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			pieces := make([]ast.ASTHandle, 0, len(points)+1)
			low := uint32(0)
			for _, high := range points {
				piece, e := f.extract(high-1, low, result)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				pieces = append(pieces, piece)
				low = high
			}
			piece, e := f.extract(width-1, low, result)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			pieces = append(pieces, piece)
			for a, b := 0, len(pieces)-1; a < b; a, b = a+1, b-1 {
				pieces[a], pieces[b] = pieces[b], pieces[a]
			}
			result, e = m.MkBitVectorConcat(pieces)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if _, e = f.hold(result); e != nil {
				return ast.ASTHandle{}, e
			}
		}
		memo[v.h] = result
	}
	return memo[root], nil
}
