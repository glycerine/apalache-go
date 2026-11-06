package core

// Source: src/tactic/core/cofactor_elim_term_ite.cpp, retained bottom-up path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

type cofactorTermITERewriter struct {
	owner *CofactorTermITETactic
	refs  contextSimplifyRefs
	memo  map[ast.ASTHandle]ast.ASTHandle
}

func newCofactorTermITERewriter(owner *CofactorTermITETactic) *cofactorTermITERewriter {
	return &cofactorTermITERewriter{owner: owner, refs: contextSimplifyRefs{m: owner.manager}, memo: make(map[ast.ASTHandle]ast.ASTHandle)}
}

func (r *cofactorTermITERewriter) Close() error { return r.refs.Close() }

func (r *cofactorTermITERewriter) checkpoint() error {
	if err := r.owner.services.Cooperation.Cooperate("cofactor ite"); err != nil {
		return err
	}
	n, err := r.owner.services.Memory.AllocationSize()
	if err != nil {
		return err
	}
	if n > r.owner.maxMemory {
		return tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	return nil
}

func (r *cofactorTermITERewriter) pin(h ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := r.refs.newRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.owner.manager.RefGet(ref)
}

func (r *cofactorTermITERewriter) isTermITE(h ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != m.BasicFamilyID() {
		return false, err
	}
	op, err := m.GetDeclKind(h)
	if err != nil || op != ast.ASTOpIte {
		return false, err
	}
	sort, err := m.GetSort(h)
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	return sort != boolean, err
}

func (r *cofactorTermITERewriter) isAtom(h ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	sort, err := m.GetSort(h)
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil || sort != boolean {
		return false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != m.BasicFamilyID() {
		return family != m.BasicFamilyID(), err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return false, err
	}
	switch op {
	case ast.ASTOpEq, ast.ASTOpDistinct:
		first, e := m.GetArg(h, 0)
		if e != nil {
			return false, e
		}
		firstSort, e := m.GetSort(first)
		return firstSort != boolean, e
	case ast.ASTOpTrue, ast.ASTOpFalse, ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpNot,
		ast.ASTOpXor, ast.ASTOpImplies, ast.ASTOpIff, ast.ASTOpIte:
		return false, nil
	default:
		return true, nil
	}
}

func (r *cofactorTermITERewriter) isEquality(h ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != m.BasicFamilyID() {
		return false, err
	}
	op, err := m.GetDeclKind(h)
	return op == ast.ASTOpEq, err
}

func (r *cofactorTermITERewriter) bestCondition(root ast.ASTHandle) (ast.ASTHandle, bool, error) {
	m := r.owner.manager
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	counts := make(map[ast.ASTHandle]uint32)
	var order []ast.ASTHandle
	for len(stack) != 0 {
		if err := r.checkpoint(); err != nil {
			return ast.ASTHandle{}, false, err
		}
		i := len(stack) - 1
		current := stack[i]
		stack = stack[:i]
		if seen[current] {
			continue
		}
		seen[current] = true
		yes, err := r.isTermITE(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if yes {
			condition, e := m.GetArg(current, 0)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			if counts[condition] == 0 {
				order = append(order, condition)
			}
			counts[condition]++
		}
		kind, err := m.GetKind(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		n, err := m.GetNumArgs(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		for j := n; j > 0; j-- {
			child, e := m.GetArg(current, j-1)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			stack = append(stack, child)
		}
	}
	var best ast.ASTHandle
	var bestDepth, bestCount uint32
	for _, condition := range order {
		depth, err := m.GetDepth(condition)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		count := counts[condition]
		choose := best.IsNull() || depth < bestDepth || depth == bestDepth && count > bestCount
		if !choose && depth == bestDepth && count == bestCount {
			equality, e1 := r.isEquality(condition)
			bestEquality, e2 := r.isEquality(best)
			if e := errors.Join(e1, e2); e != nil {
				return ast.ASTHandle{}, false, e
			}
			choose = equality && !bestEquality
		}
		if choose {
			best, bestDepth, bestCount = condition, depth, count
		}
	}
	return best, !best.IsNull(), nil
}

func (r *cofactorTermITERewriter) cofactor(root, condition ast.ASTHandle, positive bool) (result ast.ASTHandle, resultErr error) {
	m := r.owner.manager
	replace, err := rewriter.NewExprSafeReplace(m)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	atom := condition
	atomPositive := positive
	negated, argument, err := m.IsNot(condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if negated {
		atom = argument
		atomPositive = !positive
	}
	value, err := m.MkFalse()
	if atomPositive {
		value, err = m.MkTrue()
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = replace.Insert(atom, value); err != nil {
		return ast.ASTHandle{}, err
	}
	if atomPositive && r.owner.cofactorEqualities {
		equality, e := r.isEquality(atom)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if equality {
			left, e1 := m.GetArg(atom, 0)
			right, e2 := m.GetArg(atom, 1)
			if e1 != nil || e2 != nil {
				return ast.ASTHandle{}, errors.Join(e1, e2)
			}
			leftValue, e1 := m.IsUniqueValue(left)
			rightValue, e2 := m.IsUniqueValue(right)
			if e1 != nil || e2 != nil {
				return ast.ASTHandle{}, errors.Join(e1, e2)
			}
			if leftValue {
				err = replace.Insert(right, left)
			} else if rightValue {
				err = replace.Insert(left, right)
			}
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	}
	temporary, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&temporary)) }()
	if err = replace.Apply(root, temporary); err != nil {
		return ast.ASTHandle{}, err
	}
	h, err := m.RefGet(temporary)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&out)) }()
	if err = r.owner.theory.Rewrite(h, out); err != nil {
		return ast.ASTHandle{}, err
	}
	h, err = m.RefGet(out)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.pin(h)
}

func (r *cofactorTermITERewriter) eliminateAtom(atom ast.ASTHandle) (ast.ASTHandle, error) {
	m := r.owner.manager
	current := atom
	for {
		condition, found, err := r.bestCondition(current)
		if err != nil || !found {
			return current, err
		}
		positive, err := r.cofactor(current, condition, true)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		negative, err := r.cofactor(current, condition, false)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		switch {
		case positive == negative:
			current = positive
		case m.IsTrue(positive) && m.IsFalse(negative):
			current = condition
		case m.IsFalse(positive) && m.IsTrue(negative):
			current, err = m.MkNotSimplified(condition)
		default:
			current, err = m.MkIte(condition, positive, negative)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		current, err = r.pin(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
}

func (r *cofactorTermITERewriter) rewrite(root ast.ASTHandle) (ast.ASTHandle, error) {
	if value, ok := r.memo[root]; ok {
		return value, nil
	}
	if err := r.checkpoint(); err != nil {
		return ast.ASTHandle{}, err
	}
	m := r.owner.manager
	kind, err := m.GetKind(root)
	if err != nil || kind != ast.ASTApp {
		return root, err
	}
	n, err := m.GetNumArgs(root)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, n)
	changed := false
	for i := range args {
		child, e := m.GetArg(root, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args[i], e = r.rewrite(child)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || args[i] != child
	}
	current := root
	if changed {
		decl, e := m.GetDecl(root)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		current, e = m.MkApp(decl, args)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		current, e = r.pin(current)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	atom, err := r.isAtom(current)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if atom {
		current, err = r.eliminateAtom(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	r.memo[root] = current
	return current, nil
}
