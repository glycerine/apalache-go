package interp

// Source checkpoint: src/interp/iz3base.cpp Boolean simplification.

import "github.com/glycerine/g3/pkg/ast"

func (b *Base) gatherConjuncts(term ast.ASTHandle, result *[]ast.ASTHandle, seen map[ast.ASTHandle]bool) error {
	if seen[term] {
		return nil
	}
	seen[term] = true
	op, err := b.manager.Operator(term)
	if err != nil {
		return err
	}
	if op != OperatorAnd {
		*result = append(*result, term)
		return nil
	}
	count, err := b.manager.NumArgs(term)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		child, e := b.manager.Arg(term, i)
		if e != nil {
			return e
		}
		if e = b.gatherConjuncts(child, result, seen); e != nil {
			return e
		}
	}
	return nil
}

func (b *Base) simplifyConjunction(conjuncts []ast.ASTHandle) (ast.ASTHandle, error) {
	seen := make(map[ast.ASTHandle]bool)
	for i := 0; i < len(conjuncts); {
		isFalse, err := b.manager.IsFalse(conjuncts[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if isFalse {
			return conjuncts[i], nil
		}
		isTrue, err := b.manager.IsTrue(conjuncts[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if isTrue || seen[conjuncts[i]] {
			conjuncts[i] = conjuncts[len(conjuncts)-1]
			conjuncts = conjuncts[:len(conjuncts)-1]
			continue
		}
		negated, err := b.manager.MkNot(conjuncts[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if seen[negated] {
			return b.manager.Make(OperatorFalse)
		}
		seen[conjuncts[i]] = true
		i++
	}
	if len(conjuncts) == 0 {
		return b.manager.Make(OperatorTrue)
	}
	return b.manager.Make(OperatorAnd, conjuncts...)
}

func (b *Base) Simplify(term ast.ASTHandle) (ast.ASTHandle, error) {
	if err := b.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	isNot, body, err := b.manager.ast.IsNot(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if isNot {
		body, err = b.Simplify(body)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return b.manager.MkNot(body)
	}
	if result, found := b.simplified[term]; found {
		return result, nil
	}
	result := term
	op, err := b.manager.Operator(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch op {
	case OperatorAnd:
		var conjuncts []ast.ASTHandle
		if err = b.gatherConjuncts(term, &conjuncts, make(map[ast.ASTHandle]bool)); err != nil {
			return ast.ASTHandle{}, err
		}
		for i := range conjuncts {
			conjuncts[i], err = b.Simplify(conjuncts[i])
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		result, err = b.simplifyConjunction(conjuncts)
	case OperatorEqual:
		left, e := b.manager.Arg(term, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e := b.manager.Arg(term, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		leftID, e := b.manager.ASTID(left)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rightID, e := b.manager.ASTID(right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if leftID > rightID {
			left, right = right, left
		}
		result, err = b.manager.Make(OperatorEqual, left, right)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.hold(result); err != nil {
		return ast.ASTHandle{}, err
	}
	b.simplified[term] = result
	return result, nil
}
