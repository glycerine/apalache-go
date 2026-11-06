package proofs

import "github.com/glycerine/g3/pkg/ast"

func (c *Checker) checkMonotonicity(fact ast.ASTHandle, has bool, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	if !has {
		return false, nil, nil
	}
	d, left, right, yes, err := c.binary(fact)
	if err != nil || !yes {
		return invalid(err)
	}
	f1, a1, y1, err := c.app(left)
	if err != nil || !y1 {
		return invalid(err)
	}
	f2, a2, y2, err := c.app(right)
	if err != nil || !y2 {
		return invalid(err)
	}
	if f1 != f2 || len(a1) != len(a2) {
		return false, nil, nil
	}
	for i := range a1 {
		if a1[i] == a2[i] {
			continue
		}
		found := false
		for _, p := range ps {
			f, y, e := c.fact(p)
			if e != nil {
				return invalid(e)
			}
			if !y {
				continue
			}
			d2, x, z, y, e := c.binary(f)
			if e != nil {
				return invalid(e)
			}
			same, e := sameEquivalence(c, d, d2)
			if e != nil {
				return invalid(e)
			}
			if y && same && x == a1[i] && z == a2[i] {
				found = true
				break
			}
		}
		if !found {
			return false, nil, nil
		}
	}
	return true, nil, nil
}

func (c *Checker) checkQuantIntro(fact ast.ASTHandle, has bool, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	if !has || len(ps) != 1 {
		return false, nil, nil
	}
	a, b, yes, err := c.equiv(fact)
	if err != nil || !yes {
		return invalid(err)
	}
	f, y, err := c.fact(ps[0])
	if err != nil || !y {
		return invalid(err)
	}
	x, z, y, err := c.equiv(f)
	if err != nil || !y {
		return invalid(err)
	}
	ka, err := c.manager.GetKind(a)
	if err != nil {
		return invalid(err)
	}
	kb, err := c.manager.GetKind(b)
	if err != nil {
		return invalid(err)
	}
	if ka != ast.ASTQuantifier || kb != ast.ASTQuantifier {
		return false, nil, nil
	}
	ba, err := c.manager.GetQuantifierBody(a)
	if err != nil {
		return invalid(err)
	}
	bb, err := c.manager.GetQuantifierBody(b)
	if err != nil {
		return invalid(err)
	}
	na, err := c.manager.GetNumDeclarations(a)
	if err != nil {
		return invalid(err)
	}
	nb, err := c.manager.GetNumDeclarations(b)
	if err != nil {
		return invalid(err)
	}
	fa, err := c.manager.IsForall(a)
	if err != nil {
		return invalid(err)
	}
	fb, err := c.manager.IsForall(b)
	if err != nil {
		return invalid(err)
	}
	if ba != x || bb != z || na != nb || fa != fb {
		return false, nil, nil
	}
	for i := uint32(0); i < na; i++ {
		sa, e := c.manager.GetDeclarationSort(a, i)
		if e != nil {
			return invalid(e)
		}
		sb, e := c.manager.GetDeclarationSort(b, i)
		if e != nil {
			return invalid(e)
		}
		if sa != sb {
			return false, nil, nil
		}
	}
	return true, nil, nil
}

func (c *Checker) checkAndElim(fact ast.ASTHandle, has bool, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	if !has || len(ps) != 1 {
		return false, nil, nil
	}
	f, yes, err := c.fact(ps[0])
	if err != nil || !yes {
		return invalid(err)
	}
	a, yes, err := c.op(f, ast.ASTOpAnd, -1)
	if err != nil || !yes {
		return invalid(err)
	}
	for _, x := range a {
		if x == fact {
			return true, nil, nil
		}
	}
	return false, nil, nil
}

func (c *Checker) checkNotOrElim(fact ast.ASTHandle, has bool, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	if !has || len(ps) != 1 {
		return false, nil, nil
	}
	f, yes, err := c.fact(ps[0])
	if err != nil || !yes {
		return invalid(err)
	}
	n, yes, err := c.op(f, ast.ASTOpNot, 1)
	if err != nil || !yes {
		return invalid(err)
	}
	terms, yes, err := c.op(n[0], ast.ASTOpOr, -1)
	if err != nil || !yes {
		return invalid(err)
	}
	for _, x := range terms {
		y, e := c.negated(x, fact)
		if e != nil {
			return invalid(e)
		}
		if y {
			return true, nil, nil
		}
	}
	return false, nil, nil
}

func (c *Checker) checkPushQuant(fact ast.ASTHandle, has bool, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	if !has || len(ps) != 0 {
		return false, nil, nil
	}
	a, yes, err := c.op(fact, ast.ASTOpIff, 2)
	if err != nil || !yes {
		return invalid(err)
	}
	k, err := c.manager.GetKind(a[0])
	if err != nil || k != ast.ASTQuantifier {
		return invalid(err)
	}
	body, err := c.manager.GetQuantifierBody(a[0])
	if err != nil {
		return invalid(err)
	}
	left, yes, err := c.op(body, ast.ASTOpAnd, -1)
	if err != nil || !yes {
		return invalid(err)
	}
	right, yes, err := c.op(a[1], ast.ASTOpAnd, -1)
	if err != nil || !yes || len(left) != len(right) {
		return invalid(err)
	}
	n, err := c.manager.GetNumDeclarations(a[0])
	if err != nil {
		return invalid(err)
	}
	for i := range left {
		kind, e := c.manager.GetKind(right[i])
		if e != nil || kind != ast.ASTQuantifier {
			return invalid(e)
		}
		b, e := c.manager.GetQuantifierBody(right[i])
		if e != nil {
			return invalid(e)
		}
		n2, e := c.manager.GetNumDeclarations(right[i])
		if e != nil {
			return invalid(e)
		}
		if b != left[i] || n2 != n {
			return false, nil, nil
		}
	}
	// The source implementation falls through to failure after validating this
	// shape; preserve that observable result.
	return false, nil, nil
}

func (c *Checker) proofHypotheses(root ast.ASTHandle) (map[ast.ASTHandle]struct{}, error) {
	memo := make(map[ast.ASTHandle]map[ast.ASTHandle]struct{})
	visiting := make(map[ast.ASTHandle]bool)
	var walk func(ast.ASTHandle) (map[ast.ASTHandle]struct{}, error)
	walk = func(p ast.ASTHandle) (map[ast.ASTHandle]struct{}, error) {
		if v, ok := memo[p]; ok {
			return v, nil
		}
		if visiting[p] {
			return nil, ast.ErrASTDomain
		}
		visiting[p] = true
		defer delete(visiting, p)
		kind, err := c.manager.GetDeclKind(p)
		if err != nil {
			return nil, err
		}
		out := make(map[ast.ASTHandle]struct{})
		if kind == ast.ASTPrHypothesis {
			f, yes, e := c.fact(p)
			if e != nil {
				return nil, e
			}
			if !yes {
				return nil, ast.ErrASTDomain
			}
			out[f] = struct{}{}
		} else if kind != ast.ASTPrLemma {
			ps, e := c.parents(p)
			if e != nil {
				return nil, e
			}
			for _, q := range ps {
				hs, e := walk(q)
				if e != nil {
					return nil, e
				}
				for h := range hs {
					out[h] = struct{}{}
				}
			}
		}
		memo[p] = out
		return out, nil
	}
	return walk(root)
}

func (c *Checker) ors(e ast.ASTHandle) ([]ast.ASTHandle, error) {
	a, yes, err := c.op(e, ast.ASTOpOr, -1)
	if err != nil {
		return nil, err
	}
	if yes {
		return a, nil
	}
	return []ast.ASTHandle{e}, nil
}

func (c *Checker) checkLemma(_ ast.ASTHandle, fact ast.ASTHandle, has bool, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	if !has || len(ps) != 1 {
		return false, nil, nil
	}
	parentFact, yes, err := c.fact(ps[0])
	if err != nil || !yes || !c.manager.IsFalse(parentFact) {
		return invalid(err)
	}
	hs, err := c.proofHypotheses(ps[0])
	if err != nil {
		return invalid(err)
	}
	if len(hs) == 1 {
		for h := range hs {
			y, e := c.negated(h, fact)
			if e != nil {
				return invalid(e)
			}
			if y {
				return true, nil, nil
			}
		}
	}
	terms, err := c.ors(fact)
	if err != nil {
		return invalid(err)
	}
	for h := range hs {
		found := false
		for _, x := range terms {
			y, e := c.negated(x, h)
			if e != nil {
				return invalid(e)
			}
			if y {
				found = true
				break
			}
		}
		if !found {
			return false, nil, nil
		}
	}
	return true, nil, nil
}

func (c *Checker) checkUnitResolution(fact ast.ASTHandle, has bool, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	if !has || len(ps) < 2 {
		return false, nil, nil
	}
	if len(ps) == 2 {
		a, y, e := c.fact(ps[0])
		if e != nil || !y {
			return invalid(e)
		}
		b, y, e := c.fact(ps[1])
		if e != nil || !y {
			return invalid(e)
		}
		n, e := c.negated(a, b)
		if e != nil {
			return invalid(e)
		}
		if n && c.manager.IsFalse(fact) {
			return true, nil, nil
		}
	}
	clause, yes, err := c.fact(ps[0])
	if err != nil || !yes {
		return invalid(err)
	}
	terms, yes, err := c.op(clause, ast.ASTOpOr, -1)
	if err != nil || !yes {
		return invalid(err)
	}
	terms = append([]ast.ASTHandle(nil), terms...)
	for _, p := range ps[1:] {
		u, y, e := c.fact(p)
		if e != nil || !y {
			return invalid(e)
		}
		found := -1
		for j, x := range terms {
			y, e := c.negated(x, u)
			if e != nil {
				return invalid(e)
			}
			if y {
				found = j
				break
			}
		}
		if found < 0 {
			return false, nil, nil
		}
		terms[found] = terms[len(terms)-1]
		terms = terms[:len(terms)-1]
	}
	switch len(terms) {
	case 0:
		return c.manager.IsFalse(fact), nil, nil
	case 1:
		return fact == terms[0], nil, nil
	default:
		got, yes, e := c.op(fact, ast.ASTOpOr, -1)
		if e != nil || !yes {
			return invalid(e)
		}
		for _, x := range terms {
			found := false
			for _, y := range got {
				if x == y {
					found = true
					break
				}
			}
			if !found {
				return false, nil, nil
			}
		}
		return true, nil, nil
	}
}

func (c *Checker) checkIffConstant(kind int32, fact ast.ASTHandle, has bool, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	if !has || len(ps) != 1 {
		return false, nil, nil
	}
	a, yes, err := c.op(fact, ast.ASTOpIff, 2)
	if err != nil || !yes {
		return invalid(err)
	}
	f, yes, err := c.fact(ps[0])
	if err != nil || !yes {
		return invalid(err)
	}
	if kind == ast.ASTPrIffTrue {
		return f == a[0] && c.manager.IsTrue(a[1]), nil, nil
	}
	n, yes, err := c.op(f, ast.ASTOpNot, 1)
	if err != nil {
		return invalid(err)
	}
	return yes && n[0] == a[0] && c.manager.IsFalse(a[1]), nil, nil
}

func (c *Checker) checkCommutativity(fact ast.ASTHandle, has bool, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	if !has || len(ps) != 0 {
		return false, nil, nil
	}
	a, b, yes, err := c.equiv(fact)
	if err != nil || !yes {
		return invalid(err)
	}
	d1, x, y, yes, err := c.binary(a)
	if err != nil || !yes {
		return invalid(err)
	}
	d2, z, w, yes, err := c.binary(b)
	if err != nil || !yes {
		return invalid(err)
	}
	flags, err := c.manager.GetFuncDeclFlags(d1)
	if err != nil {
		return invalid(err)
	}
	return d1 == d2 && x == w && y == z && flags.Commutative, nil, nil
}

func (c *Checker) checkSkolemize(fact ast.ASTHandle, has bool) (bool, []ast.ASTHandle, error) {
	if !has {
		return false, nil, nil
	}
	a, yes, err := c.op(fact, ast.ASTOpOEq, 2)
	if err != nil || !yes {
		return invalid(err)
	}
	e := a[0]
	negated := false
	if n, y, x := c.op(e, ast.ASTOpNot, 1); x != nil {
		return invalid(x)
	} else if y {
		e = n[0]
		negated = true
	}
	kind, err := c.manager.GetKind(e)
	if err != nil || kind != ast.ASTQuantifier {
		return invalid(err)
	}
	forall, err := c.manager.IsForall(e)
	if err != nil {
		return invalid(err)
	}
	return negated == forall, nil, nil
}
