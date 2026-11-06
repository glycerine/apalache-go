package arith

// Source: src/tactic/arith/probe_arith.cpp:25-85.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/tactic"
)

func (p *ArithmeticStatisticProbe) evalDegree(g *tactic.Goal) (_ float64, err error) {
	m := g.Manager()
	algebraic, e := m.ArithmeticAlgebraicManager()
	if e != nil {
		return 0, e
	}
	q, e := algebraic.RationalManager()
	if e != nil {
		return 0, e
	}
	pm, e := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{Core: polynomial.MultivariatePolynomialConfig{Integer: q.Integers(), Rationals: q, Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }}})
	if e != nil {
		return 0, e
	}
	defer func() { err = errors.Join(err, pm.CloseChecked()) }()
	core, e := pm.ArithmeticCore()
	if e != nil {
		return 0, e
	}
	convert, e := ast.NewExpr2Polynomial(ast.Expr2PolynomialConfig{AST: m, Polynomial: pm})
	if e != nil {
		return 0, e
	}
	defer func() { err = errors.Join(err, convert.Close()) }()
	apps, e := statisticApplications(g)
	if e != nil {
		return 0, e
	}
	arith, e := m.ArithmeticFamilyID()
	if e != nil {
		return 0, e
	}
	var max, count uint32
	var total uint64
	for _, h := range apps {
		f, e := m.GetFamilyID(h)
		if e != nil {
			return 0, e
		}
		op, e := m.GetDeclKind(h)
		if e != nil {
			return 0, e
		}
		comparison := f == arith && (op >= 2 && op <= 5)
		equality := f == m.BasicFamilyID() && op == ast.ASTOpEq
		if !comparison && !equality {
			continue
		}
		if equality {
			s, e := m.GetArg(h, 0)
			if e != nil {
				return 0, e
			}
			sort, e := m.GetSort(s)
			if e != nil {
				return 0, e
			}
			sf, e := m.GetFamilyID(sort)
			if e != nil {
				return 0, e
			}
			if sf != arith {
				continue
			}
		}
		for i := uint32(0); i < 2; i++ {
			a, e := m.GetArg(h, i)
			if e != nil {
				return 0, e
			}
			poly, den, matched, e := convert.ToPolynomial(a)
			if e != nil {
				return 0, e
			}
			if !matched {
				continue
			}
			value, e := poly.Get()
			if e != nil {
				return 0, e
			}
			degree, e := core.TotalDegree(value)
			closeErr := errors.Join(poly.Close(), den.Close())
			if e != nil || closeErr != nil {
				return 0, errors.Join(e, closeErr)
			}
			if degree > max {
				max = degree
			}
			total += uint64(degree)
			count++
		}
	}
	return statisticResult(p.kind == statAverageDegree, max, total, count), nil
}
