package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster.cpp:24-116;
// src/ast/rewriter/bit_blaster/bit_blaster.h:24-50.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// Config is the concrete bit_blaster_cfg gate layer. The arithmetic bit-vector
// template is a separate source unit; this value borrows its AST manager.
type Config struct {
	manager       *ast.ASTManager
	family        ast.FamilyID
	params        SMTBitBlasterParams
	flat          bool
	elimAnd       bool
	rewriterGates bool
}

func NewConfig(m *ast.ASTManager, family ast.FamilyID, params SMTBitBlasterParams) (*Config, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &Config{manager: m, family: family, params: params, flat: true}, nil
}

func newRewriterConfig(m *ast.ASTManager, family ast.FamilyID) (*Config, error) {
	c, err := NewConfig(m, family, SMTBitBlasterParams{})
	if err == nil {
		c.flat, c.elimAnd, c.rewriterGates = false, true, true
	}
	return c, err
}

func (c *Config) sorted(a, b, d ast.ASTHandle) ([3]ast.ASTHandle, error) {
	values := [3]ast.ASTHandle{a, b, d}
	ids := make(map[ast.ASTHandle]uint32, 3)
	for _, value := range values {
		id, err := c.manager.GetASTID(value)
		if err != nil {
			return values, err
		}
		ids[value] = id
	}
	sort.Slice(values[:], func(i, j int) bool { return ids[values[i]] < ids[values[j]] })
	return values, nil
}

func (c *Config) complement(a, b ast.ASTHandle) (bool, error) {
	yes, child, err := c.manager.IsNot(a)
	if err != nil || yes && child == b {
		return yes && child == b, err
	}
	yes, child, err = c.manager.IsNot(b)
	return yes && child == a, err
}

func (c *Config) simplify(op int32, out ast.ASTRef, args ...ast.ASTHandle) error {
	return rewriter.SimplifyBooleanGateConfigured(c.manager, op, args, out, c.flat, c.elimAnd)
}

func (c *Config) MkXor3(a, b, d ast.ASTHandle, out ast.ASTRef) (result error) {
	if c.rewriterGates {
		tmp, err := c.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, c.manager.DelRef(&tmp)) }()
		if err = c.simplify(ast.ASTOpXor, tmp, b, d); err != nil {
			return err
		}
		h, err := c.manager.RefGet(tmp)
		if err != nil {
			return err
		}
		return c.simplify(ast.ASTOpXor, out, a, h)
	}
	v, err := c.sorted(a, b, d)
	if err != nil {
		return err
	}
	a, b, d = v[0], v[1], v[2]
	if !c.params.ExtGates {
		tmp, err := c.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, c.manager.DelRef(&tmp)) }()
		if err = c.simplify(ast.ASTOpXor, tmp, a, b); err != nil {
			return err
		}
		value, err := c.manager.RefGet(tmp)
		if err != nil {
			return err
		}
		return c.simplify(ast.ASTOpXor, out, value, d)
	}
	switch {
	case a == b:
		return c.manager.SetRef(out, d)
	case a == d:
		return c.manager.SetRef(out, b)
	case b == d:
		return c.manager.SetRef(out, a)
	}
	if yes, err := c.complement(a, b); err != nil || yes {
		if err != nil {
			return err
		}
		return c.simplify(ast.ASTOpNot, out, d)
	}
	if yes, err := c.complement(a, d); err != nil || yes {
		if err != nil {
			return err
		}
		return c.simplify(ast.ASTOpNot, out, b)
	}
	if yes, err := c.complement(b, d); err != nil || yes {
		if err != nil {
			return err
		}
		return c.simplify(ast.ASTOpNot, out, a)
	}
	truth, err := c.manager.MkTrue()
	if err != nil {
		return err
	}
	falsehood, err := c.manager.MkFalse()
	if err != nil {
		return err
	}
	if a == truth {
		return c.simplify(ast.ASTOpIff, out, b, d)
	}
	if a == falsehood {
		return c.simplify(ast.ASTOpXor, out, b, d)
	}
	if b == truth {
		return c.simplify(ast.ASTOpIff, out, a, d)
	}
	if b == falsehood {
		return c.simplify(ast.ASTOpXor, out, a, d)
	}
	if d == truth {
		return c.simplify(ast.ASTOpIff, out, a, b)
	}
	if d == falsehood {
		return c.simplify(ast.ASTOpXor, out, a, b)
	}
	h, err := c.manager.MkFamilyApp(c.family, 60, nil, []ast.ASTHandle{a, b, d})
	if err != nil {
		return err
	}
	return c.manager.SetRef(out, h)
}

func (c *Config) MkCarry(a, b, d ast.ASTHandle, out ast.ASTRef) (result error) {
	var err error
	if !c.rewriterGates {
		v, e := c.sorted(a, b, d)
		if e != nil {
			return e
		}
		a, b, d = v[0], v[1], v[2]
	}
	if !c.params.ExtGates {
		pairs := [3][2]ast.ASTHandle{{a, b}, {a, d}, {b, d}}
		terms := make([]ast.ASTHandle, 3)
		refs := make([]ast.ASTRef, 3)
		defer func() {
			for i := range refs {
				result = errors.Join(result, c.manager.DelRef(&refs[i]))
			}
		}()
		for i := range pairs {
			refs[i], err = c.manager.NewRef(ast.ASTHandle{})
			if err != nil {
				return err
			}
			if err = c.simplify(ast.ASTOpAnd, refs[i], pairs[i][0], pairs[i][1]); err != nil {
				return err
			}
			terms[i], err = c.manager.RefGet(refs[i])
			if err != nil {
				return err
			}
		}
		return c.simplify(ast.ASTOpOr, out, terms...)
	}
	truth, err := c.manager.MkTrue()
	if err != nil {
		return err
	}
	falsehood, err := c.manager.MkFalse()
	if err != nil {
		return err
	}
	falsePairs := (a == falsehood && b == falsehood) || (a == falsehood && d == falsehood) || (b == falsehood && d == falsehood)
	if falsePairs {
		return c.manager.SetRef(out, falsehood)
	}
	truePairs := (a == truth && b == truth) || (a == truth && d == truth) || (b == truth && d == truth)
	if truePairs {
		return c.manager.SetRef(out, truth)
	}
	if a == b && a == d {
		return c.manager.SetRef(out, a)
	}
	if a == falsehood {
		return c.simplify(ast.ASTOpAnd, out, b, d)
	}
	if b == falsehood {
		return c.simplify(ast.ASTOpAnd, out, a, d)
	}
	if d == falsehood {
		return c.simplify(ast.ASTOpAnd, out, a, b)
	}
	if a == truth {
		return c.simplify(ast.ASTOpOr, out, b, d)
	}
	if b == truth {
		return c.simplify(ast.ASTOpOr, out, a, d)
	}
	if d == truth {
		return c.simplify(ast.ASTOpOr, out, a, b)
	}
	if yes, e := c.complement(a, b); e != nil || yes {
		if e != nil {
			return e
		}
		return c.manager.SetRef(out, d)
	}
	if yes, e := c.complement(a, d); e != nil || yes {
		if e != nil {
			return e
		}
		return c.manager.SetRef(out, b)
	}
	if yes, e := c.complement(b, d); e != nil || yes {
		if e != nil {
			return e
		}
		return c.manager.SetRef(out, a)
	}
	h, err := c.manager.MkFamilyApp(c.family, 59, nil, []ast.ASTHandle{a, b, d})
	if err != nil {
		return err
	}
	return c.manager.SetRef(out, h)
}
