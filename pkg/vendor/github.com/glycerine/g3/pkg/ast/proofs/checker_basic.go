package proofs

import "github.com/glycerine/g3/pkg/ast"

func (c *Checker) checkBasic(p ast.ASTHandle, ps []ast.ASTHandle) (bool, []ast.ASTHandle, error) {
	fid, err := c.manager.GetFamilyID(p)
	if err != nil {
		return invalid(err)
	}
	if fid != c.manager.BasicFamilyID() {
		return false, nil, nil
	}
	k, err := c.manager.GetDeclKind(p)
	if err != nil {
		return invalid(err)
	}
	fact, hasFact, err := c.fact(p)
	if err != nil {
		return invalid(err)
	}
	pf := func(i int) (ast.ASTHandle, bool, error) {
		if i >= len(ps) {
			return ast.ASTHandle{}, false, nil
		}
		return c.fact(ps[i])
	}
	switch k {
	case ast.ASTPrUndef, ast.ASTPrAsserted, ast.ASTPrGoal, ast.ASTPrTrue:
		return true, nil, nil
	case ast.ASTPrModusPonens, ast.ASTPrModusPonensOEq:
		if !hasFact || len(ps) != 2 {
			return false, nil, nil
		}
		f0, y0, e := pf(0)
		if e != nil {
			return invalid(e)
		}
		f1, y1, e := pf(1)
		if e != nil {
			return invalid(e)
		}
		if !y0 || !y1 {
			return false, nil, nil
		}
		op := ast.ASTOpOEq
		if k == ast.ASTPrModusPonens {
			if a, yes, e := c.op(f1, ast.ASTOpImplies, 2); e != nil {
				return invalid(e)
			} else if yes {
				return a[0] == f0 && a[1] == fact, nil, nil
			}
			op = ast.ASTOpIff
		}
		a, yes, e := c.op(f1, op, 2)
		if e != nil {
			return invalid(e)
		}
		return yes && a[0] == f0 && a[1] == fact, nil, nil
	case ast.ASTPrReflexivity:
		if !hasFact || len(ps) != 0 {
			return false, nil, nil
		}
		a, b, yes, e := c.equiv(fact)
		if e != nil {
			return invalid(e)
		}
		return yes && a == b, nil, nil
	case ast.ASTPrSymmetry:
		if !hasFact || len(ps) != 1 {
			return false, nil, nil
		}
		f, yes, e := pf(0)
		if e != nil || !yes {
			return invalid(e)
		}
		d1, l1, r1, y1, e := c.binary(fact)
		if e != nil {
			return invalid(e)
		}
		d2, l2, r2, y2, e := c.binary(f)
		if e != nil {
			return invalid(e)
		}
		same, e := sameEquivalence(c, d1, d2)
		if e != nil {
			return invalid(e)
		}
		return y1 && y2 && same && l1 == r2 && r1 == l2, nil, nil
	case ast.ASTPrTransitivity:
		if !hasFact || len(ps) != 2 {
			return false, nil, nil
		}
		f1, y1, e := pf(0)
		if e != nil || !y1 {
			return invalid(e)
		}
		f2, y2, e := pf(1)
		if e != nil || !y2 {
			return invalid(e)
		}
		d, a, z, y, e := c.binary(fact)
		if e != nil || !y {
			return invalid(e)
		}
		d1, a1, b1, y, e := c.binary(f1)
		if e != nil || !y {
			return invalid(e)
		}
		d2, a2, z2, y, e := c.binary(f2)
		if e != nil || !y {
			return invalid(e)
		}
		return d == d1 && d1 == d2 && a == a1 && b1 == a2 && z2 == z, nil, nil
	case ast.ASTPrTransitivityStar:
		if !hasFact {
			return false, nil, nil
		}
		d, a, b, yes, e := c.binary(fact)
		if e != nil || !yes {
			return invalid(e)
		}
		odd := make(map[ast.ASTHandle]bool)
		for i := range ps {
			f, y, e := pf(i)
			if e != nil || !y {
				return invalid(e)
			}
			di, x, z, y, e := c.binary(f)
			if e != nil || !y || di != d {
				return invalid(e)
			}
			odd[x] = !odd[x]
			odd[z] = !odd[z]
		}
		count := 0
		for _, v := range odd {
			if v {
				count++
			}
		}
		return count == 2 && odd[a] && odd[b], nil, nil
	case ast.ASTPrMonotonicity:
		return c.checkMonotonicity(fact, hasFact, ps)
	case ast.ASTPrQuantIntro:
		return c.checkQuantIntro(fact, hasFact, ps)
	case ast.ASTPrDistributivity, ast.ASTPrRewrite:
		_, _, yes, e := c.equiv(fact)
		if e != nil {
			return invalid(e)
		}
		if !hasFact || !yes || len(ps) != 0 {
			return false, nil, nil
		}
		return true, []ast.ASTHandle{fact}, nil
	case ast.ASTPrRewriteStar:
		_, _, yes, e := c.equiv(fact)
		if e != nil {
			return invalid(e)
		}
		if !hasFact || !yes {
			return false, nil, nil
		}
		s, e := c.sideOr(fact, ps)
		if e != nil {
			return invalid(e)
		}
		return true, []ast.ASTHandle{s}, nil
	case ast.ASTPrAndElim:
		return c.checkAndElim(fact, hasFact, ps)
	case ast.ASTPrNotOrElim:
		return c.checkNotOrElim(fact, hasFact, ps)
	case ast.ASTPrPullQuant:
		if !hasFact || len(ps) != 0 {
			return false, nil, nil
		}
		a, yes, e := c.op(fact, ast.ASTOpIff, 2)
		if e != nil || !yes {
			return invalid(e)
		}
		kind, e := c.manager.GetKind(a[1])
		if e != nil {
			return invalid(e)
		}
		return kind == ast.ASTQuantifier, nil, nil
	case ast.ASTPrPushQuant:
		return c.checkPushQuant(fact, hasFact, ps)
	case ast.ASTPrElimUnusedVars:
		if !hasFact || len(ps) != 0 {
			return false, nil, nil
		}
		_, yes, e := c.op(fact, ast.ASTOpIff, 2)
		if e != nil {
			return invalid(e)
		}
		return yes, nil, nil
	case ast.ASTPrDER:
		if !hasFact || len(ps) != 0 {
			return false, nil, nil
		}
		a, yes, e := c.op(fact, ast.ASTOpIff, 2)
		if e != nil || !yes {
			return invalid(e)
		}
		kind, e := c.manager.GetKind(a[0])
		if e != nil || kind != ast.ASTQuantifier {
			return invalid(e)
		}
		all, e := c.manager.IsForall(a[0])
		if e != nil {
			return invalid(e)
		}
		return all, nil, nil
	case ast.ASTPrHypothesis:
		return hasFact && len(ps) == 0, nil, nil
	case ast.ASTPrLemma:
		return c.checkLemma(p, fact, hasFact, ps)
	case ast.ASTPrUnitResolution:
		return c.checkUnitResolution(fact, hasFact, ps)
	case ast.ASTPrIffTrue, ast.ASTPrIffFalse:
		return c.checkIffConstant(k, fact, hasFact, ps)
	case ast.ASTPrCommutativity:
		return c.checkCommutativity(fact, hasFact, ps)
	case ast.ASTPrDefAxiom, ast.ASTPrDefIntro:
		return hasFact && len(ps) == 0, nil, nil
	case ast.ASTPrApplyDef:
		_, yes, e := c.op(fact, ast.ASTOpOEq, 2)
		if e != nil {
			return invalid(e)
		}
		return hasFact && yes, nil, nil
	case ast.ASTPrIffOEq:
		if !hasFact || len(ps) != 1 {
			return false, nil, nil
		}
		a, y, e := c.op(fact, ast.ASTOpOEq, 2)
		if e != nil || !y {
			return invalid(e)
		}
		f, y, e := pf(0)
		if e != nil || !y {
			return invalid(e)
		}
		b, y, e := c.op(f, ast.ASTOpIff, 2)
		if e != nil {
			return invalid(e)
		}
		return y && a[0] == b[0] && a[1] == b[1], nil, nil
	case ast.ASTPrNNFPos, ast.ASTPrNNFNeg, ast.ASTPrQuantInst:
		return true, nil, nil // source checker intentionally leaves these rules open
	case ast.ASTPrSkolemize:
		return c.checkSkolemize(fact, hasFact)
	case ast.ASTPrTheoryLemma:
		return c.checkTheoryLemma(p)
	case ast.ASTPrHyperResolve:
		return c.checkHyperResolve(p)
	default:
		return false, nil, nil
	}
}
