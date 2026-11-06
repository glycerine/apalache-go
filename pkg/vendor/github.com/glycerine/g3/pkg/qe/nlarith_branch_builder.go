package qe

import "github.com/glycerine/g3/pkg/ast"

func (u *NonlinearArithmeticUtil) CreateBranches(variable *ast.ContainsApp, literals []ast.ASTHandle, branches *NLBranchConditions) (bool, error) {
	if err := u.open(); err != nil {
		return false, err
	}
	if branches == nil {
		return false, ast.ErrASTHandle
	}
	if err := branches.Reset(); err != nil {
		return false, err
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	sort, err := u.manager.GetSort(target)
	if err != nil {
		return false, err
	}
	family, err := u.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	if family != u.family {
		return false, nil
	}
	kind, err := u.manager.GetDeclKind(sort)
	if err != nil || kind != 0 {
		return false, err
	}
	analysis, ok, err := u.Analyze(variable, literals)
	if err != nil || !ok {
		return false, err
	}
	defer analysis.Close()
	degree, err := analysis.MaxDegree()
	if err != nil || degree > 2 {
		return false, err
	}
	linear, err := analysis.IsLinear()
	if err != nil {
		return false, err
	}
	if linear && !u.enableLinear {
		return false, nil
	}
	for _, literal := range analysis.Literals {
		h, er := u.manager.RefGet(literal.Atom)
		if er != nil {
			return false, er
		}
		if er = branches.AddPredicate(h); er != nil {
			return false, er
		}
	}
	quadraticIndex, singleQuadratic, err := analysis.HasSingleQuadratic()
	if err != nil {
		return false, err
	}
	for i, literal := range analysis.Literals {
		var candidates []nlCandidate
		var er error
		if singleQuadratic {
			candidates, er = u.singleQuadraticCandidates(literal, uint32(i) == quadraticIndex)
		} else {
			candidates, er = u.candidates(literal)
		}
		if er != nil {
			return false, er
		}
		for _, candidate := range candidates {
			substitutions := make([]ast.ASTHandle, len(analysis.Literals))
			parts := []ast.ASTHandle{candidate.condition}
			selected, er := u.manager.RefGet(analysis.Literals[i].Atom)
			if er != nil {
				return false, er
			}
			parts = append(parts, selected)
			for j, current := range analysis.Literals {
				var replacement ast.ASTHandle
				if candidate.strict {
					replacement, er = u.PlusEpsilonCompare(current.Polynomial, candidate.root, current.Comparison)
				} else {
					replacement, er = u.RootCompare(current.Polynomial, candidate.root, current.Comparison)
				}
				if er != nil {
					return false, er
				}
				substitutions[j] = replacement
				predicate, er := u.manager.RefGet(current.Atom)
				if er != nil {
					return false, er
				}
				implication, er := u.manager.MkImplies(predicate, replacement)
				if er != nil {
					return false, er
				}
				parts = append(parts, implication)
			}
			branch, er := u.and(parts...)
			if er != nil {
				return false, er
			}
			if er = branches.AddBranch(branch, candidate.condition, substitutions, candidate.definition, candidate.a, candidate.b, candidate.c); er != nil {
				return false, er
			}
		}
	}
	// Source final -infinity branch.
	substitutions := make([]ast.ASTHandle, len(analysis.Literals))
	parts := make([]ast.ASTHandle, 0, len(analysis.Literals))
	for i, current := range analysis.Literals {
		replacement, er := u.MinusInfinityCompare(current.Polynomial, current.Comparison)
		if er != nil {
			return false, er
		}
		substitutions[i] = replacement
		predicate, er := u.manager.RefGet(current.Atom)
		if er != nil {
			return false, er
		}
		implication, er := u.manager.MkImplies(predicate, replacement)
		if er != nil {
			return false, er
		}
		parts = append(parts, implication)
	}
	branch, err := u.and(parts...)
	if err != nil {
		return false, err
	}
	truth, err := u.manager.MkTrue()
	if err != nil {
		return false, err
	}
	definition, err := u.realNumeral("-10000")
	if err != nil {
		return false, err
	}
	zero, err := u.realNumeral("0")
	if err != nil {
		return false, err
	}
	if err = branches.AddBranch(branch, truth, substitutions, definition, zero, zero, zero); err != nil {
		return false, err
	}
	return true, nil
}
