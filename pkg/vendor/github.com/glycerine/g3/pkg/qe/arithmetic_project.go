package qe

// Source: src/qe/qe_arith.cpp imp::operator() and live-row reconstruction.

import (
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticProjection) Project(md *model.ModelCore, vars, forms *[]ast.ASTRef) (err error) {
	c, err := p.begin(md)
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if vars == nil || forms == nil {
		return ast.ErrASTHandle
	}
	hasArithmetic := false
	for _, r := range *vars {
		h, e := p.manager.RefGet(r)
		if e != nil {
			return e
		}
		if _, e = p.manager.GetDecl(h); e != nil {
			return e
		}
		_, a, e := p.arithmetic(h)
		if e != nil {
			return e
		}
		hasArithmetic = hasArithmetic || a
	}
	if !hasArithmetic {
		return nil
	}
	if err = c.startEvaluator(md); err != nil {
		return err
	}
	return c.project(vars, forms)
}

// ProjectVariable deliberately leaves the additional-variable vector alone:
// the source arithmetic plugin projects a private singleton and adds none.
func (p *ArithmeticProjection) ProjectVariable(md *model.ModelCore, variable ast.ASTHandle, additional, forms *[]ast.ASTRef) (eliminated bool, err error) {
	if additional == nil || forms == nil {
		return false, ast.ErrASTHandle
	}
	c, err := p.begin(md)
	if err != nil {
		return false, err
	}
	defer p.finish(c, &err)
	if _, err = p.manager.GetDecl(variable); err != nil {
		return false, err
	}
	// Retain the actual mutable singleton vector through interrupted cleanup.
	local := new([]ast.ASTRef)
	c.vectors = append(c.vectors, local)
	if err = p.appendRef(local, variable); err != nil {
		return false, err
	}
	_, arith, err := p.arithmetic(variable)
	if err != nil {
		return false, err
	}
	if !arith {
		return false, nil
	}
	if err = c.startEvaluator(md); err != nil {
		return false, err
	}
	err = c.project(local, forms)
	return len(*local) == 0, err
}

func (c *arithmeticProjectionCall) project(vars, forms *[]ast.ASTRef) error {
	p := c.owner
	m := p.manager
	j := 0
	// ITE linearization appends branch literals. Source visits these new
	// elements too before compacting the residue vector.
	for i := 0; i < len(*forms); i++ {
		h, err := m.RefGet((*forms)[i])
		if err != nil {
			return err
		}
		ok, err := c.linearizeLiteral(h, forms)
		if err != nil {
			return err
		}
		if !ok {
			if i != j {
				if err = m.SetRef((*forms)[j], h); err != nil {
					return err
				}
			}
			j++
		} else {
			if _, err = c.hold(h, nil); err != nil {
				return err
			}
			if err = p.emit("operator()", 311, func(w io.Writer) error {
				if e := writeExpression(m, p.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); err != nil {
				return err
			}
		}
	}
	if err := p.shrink(forms, j); err != nil {
		return err
	}
	variableSet := make(map[ast.ASTHandle]bool)
	var err error
	c.mark, err = ast.NewASTExprMark(m)
	if err != nil {
		return err
	}
	for _, r := range *vars {
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		variableSet[h] = true
		_, arith, err := p.arithmetic(h)
		if err != nil {
			return err
		}
		if _, ok := c.ids.Get2(h); arith && !ok {
			if _, err = c.id(h); err != nil {
				return err
			}
			if err = p.emit("operator()", 332, func(w io.Writer) error {
				if e := writeExpression(m, p.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " ")
				if e := writeExpression(m, p.config, w, c.values[h]); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); err != nil {
				return err
			}
		}
	}
	// Preserve source's root-only residue marking. Recursive protection is
	// supplied by the nonprojected arithmetic terms registered below.
	for _, r := range *forms {
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		if err = c.mark.Mark(h, true); err != nil {
			return err
		}
	}
	index := make([]ast.ASTHandle, c.ids.Len())
	for h, id := range c.ids.All() {
		index[id] = h
		if !variableSet[h] {
			// This existing typed visitor marks every node and includes
			// quantifier annotations, matching source mark_rec's overload.
			if _, err := m.GetNumExprsWithExprMark(h, c.mark); err != nil {
				return err
			}
		}
	}
	j = 0
	var selected []uint32
	for i, r := range *vars {
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		_, arith, err := p.arithmetic(h)
		if err != nil {
			return err
		}
		marked, e := c.mark.IsMarked(h)
		if e != nil {
			return e
		}
		if arith && !marked {
			id, ok := c.ids.Get2(h)
			if !ok {
				return ast.ErrASTDomain
			}
			selected = append(selected, id)
		} else {
			if i != j {
				if err = m.SetRef((*vars)[j], h); err != nil {
					return err
				}
			}
			j++
		}
	}
	if err := p.shrink(vars, j); err != nil {
		return err
	}
	if err := p.emit("operator()", 362, func(w io.Writer) error {
		_, _ = io.WriteString(w, "remaining vars: ")
		if e := writeExpressionVector(m, p.config, w, *vars); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		for _, id := range selected {
			_, _ = fmt.Fprintf(w, "v%d ", id)
			if e := writeExpression(m, p.config, w, index[id]); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
		}
		return c.mbo.Display(w)
	}); err != nil {
		return err
	}
	if err := c.mbo.Project(selected...); err != nil {
		return err
	}
	if err := p.emit("operator()", 368, func(w io.Writer) error { return c.mbo.Display(w) }); err != nil {
		return err
	}
	for _, row := range c.mbo.LiveRows() {
		if len(row.Terms) == 0 {
			continue
		}
		h, err := c.rowExpression(&row, index)
		if err != nil {
			return err
		}
		if err = p.appendRef(forms, h); err != nil {
			return err
		}
		value, err := c.eval(h)
		if err != nil {
			return err
		}
		if !m.IsTrue(value) {
			line, label := 427, "Evaluated "
			if len(row.Terms) == 1 && row.Terms[0].Coefficient.Sign() < 0 && row.Relation != simplex.Divides {
				line, label = 394, "Evaluated unit "
			}
			if err = p.emit("operator()", line, func(w io.Writer) error {
				_, _ = io.WriteString(w, label)
				if e := writeExpression(m, p.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " to ")
				if e := writeExpression(m, p.config, w, value); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *arithmeticProjectionCall) rowExpression(row *simplex.OptRow, index []ast.ASTHandle) (ast.ASTHandle, error) {
	p := c.owner
	m := p.manager
	var terms []ast.ASTHandle
	unitNegative := len(row.Terms) == 1 && row.Terms[0].Coefficient.Sign() < 0 && row.Relation != simplex.Divides
	for _, v := range row.Terms {
		if int(v.Variable) >= len(index) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		h := index[v.Variable]
		q := new(big.Rat).Set(&v.Coefficient)
		if unitNegative {
			q.Neg(q)
		}
		if q.Cmp(big.NewRat(1, 1)) != 0 {
			integer, _, err := p.arithmetic(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			coefficient, err := c.hold(m.ArithmeticNumeralRat(q, integer))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			h, err = c.hold(m.MkArithmeticMul(coefficient, h))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		terms = append(terms, h)
	}
	integer, _, err := p.arithmetic(terms[len(terms)-1])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	q := new(big.Rat).Set(&row.Constant)
	if !unitNegative {
		q.Neg(q)
	}
	rhs, err := c.hold(m.ArithmeticNumeralRat(q, integer))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lhs := terms[0]
	if len(terms) > 1 {
		lhs, err = c.hold(m.MkFamilyApp(p.family, 6, nil, terms))
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if row.Relation == simplex.Equal {
		return c.hold(m.MkEq(lhs, rhs))
	}
	if row.Relation == simplex.Divides {
		if row.Constant.Sign() != 0 {
			lhs, err = c.hold(m.MkArithmeticSub(lhs, rhs))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		modulus, err := c.hold(m.ArithmeticNumeralRat(&row.Modulus, true))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		lhs, err = c.hold(m.MkFamilyApp(p.family, 13, nil, []ast.ASTHandle{lhs, modulus}))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		zero, err := c.hold(m.ArithmeticIntegerNumeralText("0"))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return c.hold(m.MkEq(lhs, zero))
	}
	op := int32(2)
	if row.Relation == simplex.Less {
		op = 4
	}
	if unitNegative {
		op++
	}
	return c.hold(m.MkFamilyApp(p.family, op, nil, []ast.ASTHandle{lhs, rhs}))
}
