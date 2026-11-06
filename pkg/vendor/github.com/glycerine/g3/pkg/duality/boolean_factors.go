package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"sort"
)

// ReallySimplifyAndOr implements the source constant reduction followed by
// common-factor extraction. It preserves repeated factors and does not flatten
// arbitrary nested juncts. Inputs are borrowed; the result owns its reference.
// Source: duality_rpfp.cpp:463-535.
func (c *Context) ReallySimplifyAndOr(args []*Expr, isAnd bool) (out *Expr, result error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	m := c.manager
	var pins []*Expr
	defer func() {
		for _, pin := range pins {
			result = errors.Join(result, pin.Close())
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	var simplify func([]ast.ASTHandle, bool) (ast.ASTHandle, error)
	simplify = func(input []ast.ASTHandle, and bool) (ast.ASTHandle, error) {
		reduced := make([]ast.ASTHandle, 0, len(input))
		for _, h := range input {
			if (and && m.IsFalse(h)) || (!and && m.IsTrue(h)) {
				return h, nil
			}
			if (and && m.IsTrue(h)) || (!and && m.IsFalse(h)) {
				continue
			}
			reduced = append(reduced, h)
		}
		var h ast.ASTHandle
		var err error
		switch len(reduced) {
		case 0:
			if and {
				h, err = m.MkTrue()
			} else {
				h, err = m.MkFalse()
			}
		case 1:
			return reduced[0], nil
		default:
			if and {
				h, err = m.MkAndArgs(reduced)
			} else {
				h, err = m.MkOrArgs(reduced)
			}
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pin, err := NewExpr(c, h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, pin)
		return h, nil
	}
	reduced := make([]ast.ASTHandle, 0, len(args))
	for _, term := range args {
		if err := term.open(); err != nil {
			return nil, err
		}
		if term.value.object.context != c {
			return nil, ErrContextMismatch
		}
		h, err := term.Raw()
		if err != nil {
			return nil, err
		}
		if (isAnd && m.IsFalse(h)) || (!isAnd && m.IsTrue(h)) {
			return term.Clone()
		}
		if (isAnd && m.IsTrue(h)) || (!isAnd && m.IsFalse(h)) {
			continue
		}
		reduced = append(reduced, h)
	}
	finish := func() (*Expr, error) {
		h, err := simplify(reduced, isAnd)
		if err != nil {
			return nil, err
		}
		return NewExpr(c, h)
	}
	if len(reduced) < 2 {
		return finish()
	}
	opposite := ast.ASTOpAnd
	if isAnd {
		opposite = ast.ASTOpOr
	}
	var common []ast.ASTHandle
	rows := make([][]ast.ASTHandle, len(reduced))
	// The C++ set comparator orders opaque AST pointers. Arena slot identity
	// provides the corresponding stable order within this manager in Go.
	less := func(a, b ast.ASTHandle) bool {
		if a.Slot() != b.Slot() {
			return a.Slot() < b.Slot()
		}
		return a.Generation() < b.Generation()
	}
	for i, h := range reduced {
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, err
		}
		if kind != ast.ASTApp {
			return finish()
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return nil, err
		}
		if op != opposite {
			return finish()
		}
		count, err := m.GetNumArgs(h)
		if err != nil {
			return nil, err
		}
		row := make([]ast.ASTHandle, count)
		for j := uint32(0); j < count; j++ {
			row[j], err = m.GetArg(h, j)
			if err != nil {
				return nil, err
			}
		}
		rows[i] = row
		ordered := append([]ast.ASTHandle(nil), row...)
		sort.Slice(ordered, func(a, b int) bool { return less(ordered[a], ordered[b]) })
		if i == 0 {
			common = ordered
			continue
		}
		var intersection []ast.ASTHandle
		for a, b := 0, 0; a < len(common) && b < len(ordered); {
			if common[a] == ordered[b] {
				intersection = append(intersection, common[a])
				a++
				b++
			} else if less(common[a], ordered[b]) {
				a++
			} else {
				b++
			}
		}
		common = intersection
	}
	if len(common) == 0 {
		return finish()
	}
	commonSet := make(map[ast.ASTHandle]bool)
	for _, h := range common {
		commonSet[h] = true
	}
	for i, row := range rows {
		var rest []ast.ASTHandle
		for _, h := range row {
			if !commonSet[h] {
				rest = append(rest, h)
			}
		}
		h, err := simplify(rest, !isAnd)
		if err != nil {
			return nil, err
		}
		reduced[i] = h
	}
	rest, err := simplify(reduced, isAnd)
	if err != nil {
		return nil, err
	}
	common = append(common, rest)
	h, err := simplify(common, !isAnd)
	if err != nil {
		return nil, err
	}
	return NewExpr(c, h)
}
