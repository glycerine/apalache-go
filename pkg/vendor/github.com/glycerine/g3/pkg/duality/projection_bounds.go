package duality

// Source: duality_rpfp.cpp, VariableProjector::IsCanonIneq/ElimRedundantBounds.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// projectionArithmetic owns normalized and generated terms until an output
// acquires its independent references. Arithmetic kind numbers follow the
// project's arith_op_kind, rather than a port of any C++ library machinery.
type projectionArithmetic struct {
	graph    *RPFP
	simplify *interp.Manager
	family   ast.FamilyID
	refs     []ast.ASTRef
}

func (p *projectionArithmetic) close() (result error) {
	for i := range p.refs {
		result = errors.Join(result, p.graph.context.manager.DelRef(&p.refs[i]))
	}
	return
}
func (p *projectionArithmetic) pin(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := p.graph.context.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.refs = append(p.refs, ref)
	return h, nil
}
func (p *projectionArithmetic) app(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return p.pin(p.graph.context.manager.MkFamilyApp(p.family, op, nil, args))
}
func (p *projectionArithmetic) numeral(value string) (ast.ASTHandle, error) {
	return p.pin(p.graph.context.manager.ArithmeticIntegerNumeralText(value))
}
func (p *projectionArithmetic) normalized(h ast.ASTHandle, strong bool) (ast.ASTHandle, error) {
	var ref ast.ASTRef
	var err error
	if strong {
		ref, err = p.simplify.ReallySimplify(h)
	} else {
		ref, err = p.simplify.Simplify(h)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.refs = append(p.refs, ref)
	return p.graph.context.manager.RefGet(ref)
}
func (p *projectionArithmetic) kind(h ast.ASTHandle) (int32, error) {
	m := p.graph.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return -1, err
	}
	if kind != ast.ASTApp {
		return -1, nil
	}
	family, err := m.GetFamilyID(h)
	if err != nil {
		return -1, err
	}
	if family != p.family {
		return -1, nil
	}
	return m.GetDeclKind(h)
}

// canonical returns term >= bound, preserving the source's integer-unit
// adjustments for negated inequalities and its GE sign convention.
func (p *projectionArithmetic) canonical(literal ast.ASTHandle) (term, bound ast.ASTHandle, matched bool, result error) {
	m := p.graph.context.manager
	negative, atom, err := m.IsNot(literal)
	if err != nil {
		return term, bound, false, err
	}
	if !negative {
		atom = literal
	}
	op, err := p.kind(atom)
	if err != nil {
		return term, bound, false, err
	}
	if op != 2 && op != 3 {
		return term, bound, false, nil
	}
	lhs, err := m.GetArg(atom, 0)
	if err != nil {
		return term, bound, false, err
	}
	rhs, err := m.GetArg(atom, 1)
	if err != nil {
		return term, bound, false, err
	}
	switch {
	case op == 2 && !negative:
		bound = lhs
		term = rhs
	case op == 3 && !negative:
		bound = rhs
		term = lhs
	case op == 2 && negative:
		one, err := p.numeral("1")
		if err != nil {
			return term, bound, false, err
		}
		bound, err = p.app(6, rhs, one)
		if err != nil {
			return term, bound, false, err
		}
		bound, err = p.normalized(bound, false)
		if err != nil {
			return term, bound, false, err
		}
		term = lhs
	case op == 3 && negative:
		one, err := p.numeral("1")
		if err != nil {
			return term, bound, false, err
		}
		difference, err := p.app(7, rhs, one)
		if err != nil {
			return term, bound, false, err
		}
		bound, err = p.app(8, difference)
		if err != nil {
			return term, bound, false, err
		}
		bound, err = p.normalized(bound, false)
		if err != nil {
			return term, bound, false, err
		}
		term, err = p.app(8, lhs)
		if err != nil {
			return term, bound, false, err
		}
	}
	term, err = p.normalized(term, true)
	if err != nil {
		return term, bound, false, err
	}
	return term, bound, true, nil
}
func (p *projectionArithmetic) redundantBounds(input []ast.ASTHandle) ([]ast.ASTHandle, error) {
	m := p.graph.context.manager
	output := make([]ast.ASTHandle, len(input))
	best := make(map[ast.ASTHandle]int)
	truth, err := p.pin(m.MkTrue())
	if err != nil {
		return nil, err
	}
	for i, literal := range input {
		output[i], err = p.normalized(literal, true)
		if err != nil {
			return nil, err
		}
		term, bound, matched, err := p.canonical(output[i])
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		previous, found := best[term]
		if !found {
			best[term] = i
			continue
		}
		_, oldBound, _, err := p.canonical(output[previous])
		if err != nil {
			return nil, err
		}
		greater, err := p.app(5, bound, oldBound)
		if err != nil {
			return nil, err
		}
		greater, err = p.normalized(greater, false)
		if err != nil {
			return nil, err
		}
		if m.IsTrue(greater) {
			output[previous] = truth
			best[term] = i
		} else {
			output[i] = truth
		}
	}
	return output, nil
}

// ElimRedundantBounds normalizes arithmetic literals and replaces weaker
// canonical bounds with true while preserving source literal positions.
func (g *RPFP) ElimRedundantBounds(literals []*Expr, simplifier *interp.Manager) (out []*Expr, result error) {
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
			for _, term := range output {
				result = errors.Join(result, term.Close())
			}
			out = nil
		}
	}()
	input := make([]ast.ASTHandle, len(literals))
	for i, term := range literals {
		if err := term.open(); err != nil {
			return nil, err
		}
		if term.value.object.context != g.context {
			return nil, ErrContextMismatch
		}
		input[i], err = term.Raw()
		if err != nil {
			return nil, err
		}
	}
	normalized, err := p.redundantBounds(input)
	if err != nil {
		return nil, err
	}
	for _, h := range normalized {
		term, err := NewExpr(g.context, h)
		if err != nil {
			return nil, err
		}
		output = append(output, term)
	}
	return output, nil
}
