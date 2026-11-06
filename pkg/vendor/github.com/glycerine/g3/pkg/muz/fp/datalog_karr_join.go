package fp

// Source: dl_compiler.cpp two-positive-tail join and its local projection.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type karrJoinConstant struct {
	column  uint32
	integer bool
}
type karrJoinPlan struct {
	leftColumns, rightColumns []uint32
	removed                   []uint32
	expressions               []ast.ASTHandle
	constants                 []karrJoinConstant
	constantValues            []ast.ASTHandle
	identical                 [][]uint32
}
type karrJoinArgument struct {
	expr     ast.ASTHandle
	variable bool
	index    uint32
	integer  bool
}

// compileKarrJoin consumes borrowed applications. expressions describes the
// columns after the source join-project, for subsequent head assembly.
func (a *karrDatalogActivation) compileKarrJoin(head, left, right ast.ASTHandle) (karrJoinPlan, error) {
	var p karrJoinPlan
	read := func(app ast.ASTHandle) ([]karrJoinArgument, error) {
		n, err := a.manager.GetNumArgs(app)
		if err != nil {
			return nil, err
		}
		args := make([]karrJoinArgument, n)
		for i := range args {
			h, e := a.manager.GetArg(app, uint32(i))
			if e != nil {
				return nil, e
			}
			args[i].expr = h
			kind, e := a.manager.GetKind(h)
			if e != nil {
				return nil, e
			}
			if kind == ast.ASTVar {
				args[i].variable = true
				args[i].index, e = a.manager.GetVarIndex(h)
				if e != nil {
					return nil, e
				}
				continue
			}
			text, _, matched, e := a.manager.ArithmeticNumeralText(h)
			if e != nil {
				return nil, e
			}
			if !matched {
				return nil, ErrKarrDatalogFragment
			}
			q, ok := new(big.Rat).SetString(text)
			if !ok {
				return nil, ErrKarrDatalogFragment
			}
			args[i].integer = q.IsInt()
		}
		return args, nil
	}
	l, err := read(left)
	if err != nil {
		return p, err
	}
	r, err := read(right)
	if err != nil {
		return p, err
	}
	// The head may contain constants or repeated variables; each variable
	// occurrence reserves a column where possible.
	h, err := read(head)
	if err != nil {
		return p, err
	}
	counts := make(map[uint32]int)
	for _, arg := range h {
		if arg.variable {
			counts[arg.index]--
		}
	}
	for _, arg := range l {
		if arg.variable {
			counts[arg.index]++
		}
	}
	for _, arg := range r {
		if arg.variable {
			counts[arg.index]++
		}
	}
	for i, x := range l {
		if x.variable {
			for j, y := range r {
				if y.variable && x.index == y.index {
					p.leftColumns = append(p.leftColumns, uint32(i))
					p.rightColumns = append(p.rightColumns, uint32(j))
				}
			}
		}
	}
	all := append(append([]karrJoinArgument(nil), l...), r...)
	kept := make([]karrJoinArgument, 0, len(all))
	boundary := 0
	for i, x := range all {
		if x.variable && counts[x.index] > 0 {
			counts[x.index]--
			p.removed = append(p.removed, uint32(i))
			continue
		}
		kept = append(kept, x)
		p.expressions = append(p.expressions, x.expr)
		if i < len(l) {
			boundary++
		}
	}
	groups := make(map[uint32][]uint32)
	var order []uint32
	for i, x := range kept {
		if !x.variable {
			p.constants = append(p.constants, karrJoinConstant{uint32(i), x.integer})
			p.constantValues = append(p.constantValues, x.expr)
			continue
		}
		if len(groups[x.index]) == 0 {
			order = append(order, x.index)
		}
		groups[x.index] = append(groups[x.index], uint32(i))
	}
	for _, v := range order {
		g := groups[v]
		if len(g) > 1 && !(int(g[0]) < boundary && int(g[len(g)-1]) >= boundary) {
			p.identical = append(p.identical, g)
		}
	}
	return p, nil
}

// apply returns a new relation, leaving the borrowed operands' semantics intact.
func (p karrJoinPlan) apply(left, right *rel.KarrRelation) (*rel.KarrRelation, error) {
	result, err := rel.KarrJoin(left, right, p.leftColumns, p.rightColumns)
	if err != nil {
		return nil, err
	}
	if len(p.removed) > 0 {
		result, err = rel.KarrSourceProject(result, p.removed)
		if err != nil {
			return nil, err
		}
	}
	if result.Empty() {
		return result, nil
	}
	for i, c := range p.constants {
		before := result.Clone()
		if err = result.FilterEqual(c.column, c.integer); err != nil {
			return nil, err
		}
		if checker := result.FormulaChecker(); checker != nil {
			if i >= len(p.constantValues) {
				return nil, ErrKarrDatalogFragment
			}
			if err = checker.VerifyFilterEqual(before, c.column, p.constantValues[i], result); err != nil {
				return nil, err
			}
		}
	}
	for _, group := range p.identical {
		if err = result.FilterIdentical(group); err != nil {
			return nil, err
		}
	}
	return result, nil
}
