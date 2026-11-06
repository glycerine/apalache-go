package duality

// Source: duality_rpfp.cpp, VariableProjector::FourierMotzkinCheap.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// FourierMotzkinCheap combines one indexed upper and lower bound per eligible
// variable. Input expressions are borrowed and returned expressions are owned.
func (g *RPFP) FourierMotzkinCheap(keep, literals []*Expr, simplifier *interp.Manager) (out []*Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if simplifier == nil {
		return nil, ErrWrapperOwner
	}
	m := g.context.manager
	owner, err := simplifier.ASTManager()
	if err != nil {
		return nil, err
	}
	if owner != m {
		return nil, ErrContextMismatch
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	p := &projectionArithmetic{graph: g, simplify: simplifier, family: family}
	var output []*Expr
	defer func() {
		result = errors.Join(result, p.close())
		if result != nil {
			for _, e := range output {
				result = errors.Join(result, e.Close())
			}
			out = nil
		}
	}()
	raw := func(e *Expr) (ast.ASTHandle, error) {
		if err := e.open(); err != nil {
			return ast.ASTHandle{}, err
		}
		if e.value.object.context != g.context {
			return ast.ASTHandle{}, ErrContextMismatch
		}
		return e.Raw()
	}
	kept := make(map[ast.ASTHandle]bool)
	for _, e := range keep {
		h, err := raw(e)
		if err != nil {
			return nil, err
		}
		kept[h] = true
	}
	input := make([]ast.ASTHandle, len(literals))
	for i, e := range literals {
		input[i], err = raw(e)
		if err != nil {
			return nil, err
		}
	}
	values, err := p.redundantBounds(input)
	if err != nil {
		return nil, err
	}
	index := newProjectionIndex(p)
	for i, h := range values {
		if err := index.predicate(true, h, i); err != nil {
			return nil, err
		}
	}
	canon := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		h, err := p.normalized(h, false)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		term, bound, ok, err := p.canonical(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !ok {
			return ast.ASTHandle{}, ErrWrapperOwner
		}
		return p.app(7, term, bound)
	}
	truth, err := p.pin(m.MkTrue())
	if err != nil {
		return nil, err
	}
	zero, err := p.numeral("0")
	if err != nil {
		return nil, err
	}
	for _, variable := range index.positive {
		neg, found := index.indices[0][variable]
		if !found {
			continue
		}
		pos := index.indices[1][variable]
		if pos < 0 || neg < 0 || kept[variable] {
			continue
		}
		a, err := canon(values[pos])
		if err != nil {
			return nil, err
		}
		b, err := canon(values[neg])
		if err != nil {
			return nil, err
		}
		nc, err := p.app(8, index.coefficients[0][variable])
		if err != nil {
			return nil, err
		}
		left, err := p.app(9, nc, a)
		if err != nil {
			return nil, err
		}
		right, err := p.app(9, index.coefficients[1][variable], b)
		if err != nil {
			return nil, err
		}
		sum, err := p.app(6, left, right)
		if err != nil {
			return nil, err
		}
		ineq, err := p.app(2, zero, sum)
		if err != nil {
			return nil, err
		}
		ineq, err = p.normalized(ineq, false)
		if err != nil {
			return nil, err
		}
		values[pos] = ineq
		values[neg] = truth
		index.fixing = true
		err = index.predicate(true, ineq, pos)
		index.fixing = false
		if err != nil {
			return nil, err
		}
	}
	for _, h := range values {
		e, err := NewExpr(g.context, h)
		if err != nil {
			return nil, err
		}
		output = append(output, e)
	}
	return output, nil
}
