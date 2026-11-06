package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, get_polarities and
// is_pure_arithmetic. These analyses borrow all expressions from the caller.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type polarity uint8

const (
	positive polarity = iota
	negative
	dual
)

func negatePolarity(p polarity) polarity {
	switch p {
	case positive:
		return negative
	case negative:
		return positive
	default:
		return dual
	}
}

func purificationPolarities(m *ast.ASTManager, roots []ast.ASTHandle) (map[ast.ASTHandle]polarity, error) {
	type occurrence struct {
		term ast.ASTHandle
		pol  polarity
	}
	var todo []occurrence
	for _, root := range roots {
		todo = append(todo, occurrence{root, positive})
	}
	result := make(map[ast.ASTHandle]polarity)
	for len(todo) > 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		p := current.pol
		if old, found := result[current.term]; found {
			if old == p || old == dual {
				continue
			}
			p = dual
		}
		result[current.term] = p
		kind, err := m.GetKind(current.term)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTQuantifier || kind == ast.ASTVar {
			return nil, tactic.NewTacticException("nl-purify tactic does not support quantifiers")
		}
		if kind != ast.ASTApp {
			return nil, ast.ErrASTDomain
		}
		fid, err := m.GetFamilyID(current.term)
		if err != nil {
			return nil, err
		}
		op, err := m.GetDeclKind(current.term)
		if err != nil {
			return nil, err
		}
		if fid == m.BasicFamilyID() && p != dual {
			switch op {
			case ast.ASTOpNot:
				p = negatePolarity(p)
			case ast.ASTOpAnd, ast.ASTOpOr:
			default:
				p = dual
			}
		} else {
			p = dual
		}
		n, err := m.GetNumArgs(current.term)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < n; i++ {
			arg, err := m.GetArg(current.term, i)
			if err != nil {
				return nil, err
			}
			todo = append(todo, occurrence{arg, p})
		}
	}
	return result, nil
}

func pureArithmeticExpressions(m *ast.ASTManager, roots []ast.ASTHandle) (map[ast.ASTHandle]bool, error) {
	result := make(map[ast.ASTHandle]bool)
	for _, root := range roots {
		if _, err := pureArithmeticExpression(m, root, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func pureArithmeticExpression(m *ast.ASTManager, root ast.ASTHandle, result map[ast.ASTHandle]bool) (bool, error) {
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return false, err
	}
	isReal := func(h ast.ASTHandle) (bool, error) {
		sort, err := m.GetSort(h)
		if err != nil {
			return false, err
		}
		fid, err := m.GetFamilyID(sort)
		if err != nil || fid != family {
			return false, err
		}
		kind, err := m.GetDeclKind(sort)
		return kind == 0, err
	}
	type frame struct {
		term ast.ASTHandle
		exit bool
	}
	todo := []frame{{term: root}}
	for len(todo) > 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if _, found := result[current.term]; found {
			continue
		}
		kind, err := m.GetKind(current.term)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			result[current.term] = false
			continue
		}
		n, err := m.GetNumArgs(current.term)
		if err != nil {
			return false, err
		}
		if !current.exit {
			todo = append(todo, frame{term: current.term, exit: true})
			for i := uint32(0); i < n; i++ {
				arg, err := m.GetArg(current.term, i)
				if err != nil {
					return false, err
				}
				todo = append(todo, frame{term: arg})
			}
			continue
		}
		fid, err := m.GetFamilyID(current.term)
		if err != nil {
			return false, err
		}
		op, err := m.GetDeclKind(current.term)
		if err != nil {
			return false, err
		}
		real, err := isReal(current.term)
		if err != nil {
			return false, err
		}
		pure := fid == m.BasicFamilyID() || (fid == family && real) || (fid == ast.NullFamilyID && n == 0 && real)
		if n > 0 {
			arg, err := m.GetArg(current.term, 0)
			if err != nil {
				return false, err
			}
			argReal, err := isReal(arg)
			if err != nil {
				return false, err
			}
			sort, err := m.GetSort(current.term)
			if err != nil {
				return false, err
			}
			pure = pure || (fid == m.BasicFamilyID() && op == ast.ASTOpEq && argReal) || (fid == family && sort == boolean && argReal)
		}
		for i := uint32(0); i < n; i++ {
			arg, err := m.GetArg(current.term, i)
			if err != nil {
				return false, err
			}
			pure = pure && result[arg]
		}
		result[current.term] = pure
	}
	return result[root], nil
}

// removePureArithmetic performs the source's transfer before the cooperating
// solvers run. The destination acquires each formula before the original goal
// slot is replaced by true. Existing proof and dependency handles travel with it.
func removePureArithmetic(g, nonlinear *tactic.Goal) error {
	if g == nil || nonlinear == nil || g == nonlinear || g.Manager() != nonlinear.Manager() {
		return ast.ErrASTDomain
	}
	if err := g.CheckOwner(); err != nil {
		return err
	}
	if err := nonlinear.CheckOwner(); err != nil {
		return err
	}
	n, err := g.Size()
	if err != nil {
		return err
	}
	pureCache := make(map[ast.ASTHandle]bool)
	for i := uint32(0); i < n; i++ {
		root, err := g.Form(i)
		if err != nil {
			return err
		}
		pure, err := pureArithmeticExpression(g.Manager(), root, pureCache)
		if err != nil {
			return err
		}
		if !pure {
			continue
		}
		truth, err := g.Manager().MkTrue()
		if err != nil {
			return err
		}
		proof, err := g.Proof(uint32(i))
		if err != nil {
			return err
		}
		dep, err := g.Dependency(uint32(i))
		if err != nil {
			return err
		}
		if err = nonlinear.AssertWithProof(root, proof, dep); err != nil {
			return err
		}
		if err = g.Update(uint32(i), truth, proof, dep); err != nil {
			return err
		}
	}
	return nil
}
