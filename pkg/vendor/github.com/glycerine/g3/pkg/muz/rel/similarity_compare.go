package rel

// Source: src/muz/rel/dl_mk_similarity_compressor.cpp, rule comparisons.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func similarityCompareInt(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// similarityRoughCompare groups rules differing only in non-variable arguments
// of the head and positive predicate tails. Rules and ASTs are borrowed.
func similarityRoughCompare(a, b *base.Rule) (int, error) {
	m, e := a.ASTManager()
	if e != nil {
		return 0, e
	}
	other, e := b.ASTManager()
	if e != nil {
		return 0, e
	}
	if m != other {
		return 0, ast.ErrASTDomain
	}
	at, e := a.TailSize()
	if e != nil {
		return 0, e
	}
	bt, e := b.TailSize()
	if e != nil {
		return 0, e
	}
	if c := similarityCompareInt(uint64(at), uint64(bt)); c != 0 {
		return c, nil
	}
	au, e := a.UninterpretedTailSize()
	if e != nil {
		return 0, e
	}
	bu, e := b.UninterpretedTailSize()
	if e != nil {
		return 0, e
	}
	if c := similarityCompareInt(uint64(au), uint64(bu)); c != 0 {
		return c, nil
	}
	ap, e := a.PositiveTailSize()
	if e != nil {
		return 0, e
	}
	bp, e := b.PositiveTailSize()
	if e != nil {
		return 0, e
	}
	if c := similarityCompareInt(uint64(ap), uint64(bp)); c != 0 {
		return c, nil
	}
	for i := -1; i < ap; i++ {
		var x, y ast.ASTHandle
		if i < 0 {
			x, e = a.Head()
			if e != nil {
				return 0, e
			}
			y, e = b.Head()
		} else {
			x, _, e = a.Tail(i)
			if e != nil {
				return 0, e
			}
			y, _, e = b.Tail(i)
		}
		if e != nil {
			return 0, e
		}
		dx, e := m.GetDecl(x)
		if e != nil {
			return 0, e
		}
		dy, e := m.GetDecl(y)
		if e != nil {
			return 0, e
		}
		ix, e := m.GetASTID(dx)
		if e != nil {
			return 0, e
		}
		iy, e := m.GetASTID(dy)
		if e != nil {
			return 0, e
		}
		if c := similarityCompareInt(uint64(ix), uint64(iy)); c != 0 {
			return c, nil
		}
		n, e := m.GetNumArgs(x)
		if e != nil {
			return 0, e
		}
		for j := uint32(0); j < n; j++ {
			xarg, e := m.GetArg(x, j)
			if e != nil {
				return 0, e
			}
			yarg, e := m.GetArg(y, j)
			if e != nil {
				return 0, e
			}
			xk, e := m.GetKind(xarg)
			if e != nil {
				return 0, e
			}
			yk, e := m.GetKind(yarg)
			if e != nil {
				return 0, e
			}
			xv, yv := xk == ast.ASTVar, yk == ast.ASTVar
			if xv != yv {
				if xv {
					return 1, nil
				}
				return -1, nil
			}
			if xv {
				xi, e := m.GetVarIndex(xarg)
				if e != nil {
					return 0, e
				}
				yi, e := m.GetVarIndex(yarg)
				if e != nil {
					return 0, e
				}
				if c := similarityCompareInt(uint64(xi), uint64(yi)); c != 0 {
					return c, nil
				}
			}
		}
	}
	for i := ap; i < at; i++ {
		x, _, e := a.Tail(i)
		if e != nil {
			return 0, e
		}
		y, _, e := b.Tail(i)
		if e != nil {
			return 0, e
		}
		xi, e := m.GetASTID(x)
		if e != nil {
			return 0, e
		}
		yi, e := m.GetASTID(y)
		if e != nil {
			return 0, e
		}
		if c := similarityCompareInt(uint64(xi), uint64(yi)); c != 0 {
			return c, nil
		}
	}
	return 0, nil
}

// similarityTotalCompare compares non-variable arguments, optionally omitting
// one global constant position across the head and positive tails.
func similarityTotalCompare(a, b *base.Rule, skip int) (int, error) {
	rough, e := similarityRoughCompare(a, b)
	if e != nil {
		return 0, e
	}
	if rough != 0 {
		return 0, ast.ErrASTDomain
	}
	m, e := a.ASTManager()
	if e != nil {
		return 0, e
	}
	n, e := a.PositiveTailSize()
	if e != nil {
		return 0, e
	}
	for i := -1; i < n; i++ {
		var x, y ast.ASTHandle
		if i < 0 {
			x, e = a.Head()
			if e != nil {
				return 0, e
			}
			y, e = b.Head()
		} else {
			x, _, e = a.Tail(i)
			if e != nil {
				return 0, e
			}
			y, _, e = b.Tail(i)
		}
		if e != nil {
			return 0, e
		}
		count, e := m.GetNumArgs(x)
		if e != nil {
			return 0, e
		}
		for j := uint32(0); j < count; j++ {
			xa, e := m.GetArg(x, j)
			if e != nil {
				return 0, e
			}
			k, e := m.GetKind(xa)
			if e != nil {
				return 0, e
			}
			if k == ast.ASTVar {
				continue
			}
			ya, e := m.GetArg(y, j)
			if e != nil {
				return 0, e
			}
			omit := skip == 0
			skip--
			if omit {
				continue
			}
			xi, e := m.GetASTID(xa)
			if e != nil {
				return 0, e
			}
			yi, e := m.GetASTID(ya)
			if e != nil {
				return 0, e
			}
			if c := similarityCompareInt(uint64(xi), uint64(yi)); c != 0 {
				return c, nil
			}
		}
	}
	return 0, nil
}
