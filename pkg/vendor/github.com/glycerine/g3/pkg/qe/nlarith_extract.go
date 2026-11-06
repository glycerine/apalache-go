package qe

import "github.com/glycerine/g3/pkg/ast"

func (u *NonlinearArithmeticUtil) isVariable(h ast.ASTHandle) (bool, error) {
	family, err := u.manager.GetFamilyID(h)
	if err != nil {
		return false, err
	}
	n, err := u.manager.GetNumArgs(h)
	if err != nil {
		return false, err
	}
	if family != ast.NullFamilyID || n != 0 {
		return false, nil
	}
	sort, err := u.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	family, err = u.manager.GetFamilyID(sort)
	if err != nil || family != u.family {
		return false, err
	}
	kind, err := u.manager.GetDeclKind(sort)
	if err != nil {
		return false, err
	}
	// arith_util::is_real accepts only the arithmetic plugin's real sort.
	// Treating every non-integer nullary symbol as Real incorrectly admits
	// Boolean, bit-vector, and uninterpreted constants as arithmetic variables.
	return kind == 0, nil
}
func (u *NonlinearArithmeticUtil) isNonlinearMul(h ast.ASTHandle) (bool, error) {
	family, err := u.manager.GetFamilyID(h)
	if err != nil || family != u.family {
		return false, err
	}
	kind, err := u.manager.GetDeclKind(h)
	if err != nil || kind != 9 {
		return false, err
	}
	n, err := u.manager.GetNumArgs(h)
	if err != nil {
		return false, err
	}
	nonNumerals := 0
	for i := uint32(0); i < n && nonNumerals < 2; i++ {
		a, cause := u.manager.GetArg(h, i)
		if cause != nil {
			return false, cause
		}
		_, numeral, cause := u.term.numeral(a)
		if cause != nil {
			return false, cause
		}
		if !numeral {
			nonNumerals++
		}
	}
	return nonNumerals >= 2, nil
}
func (u *NonlinearArithmeticUtil) ExtractNonlinear(root ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := u.open(); err != nil {
		return nil, err
	}
	type work struct {
		h         ast.ASTHandle
		nonlinear bool
	}
	todo := []work{{root, false}}
	seen := make(map[work]bool)
	added := make(map[ast.ASTHandle]bool)
	var result []ast.ASTHandle
	for len(todo) > 0 {
		w := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[w] {
			continue
		}
		seen[w] = true
		kind, err := u.manager.GetKind(w.h)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTQuantifier {
			body, cause := u.manager.GetQuantifierBody(w.h)
			if cause != nil {
				return nil, cause
			}
			todo = append(todo, work{body, w.nonlinear})
			continue
		}
		if kind != ast.ASTApp {
			continue
		}
		variable, err := u.isVariable(w.h)
		if err != nil {
			return nil, err
		}
		if variable {
			if (w.nonlinear || u.enableLinear) && !added[w.h] {
				added[w.h] = true
				result = append(result, w.h)
			}
			continue
		}
		family, err := u.manager.GetFamilyID(w.h)
		if err != nil {
			return nil, err
		}
		nl := w.nonlinear
		arithmetical := family == u.family || family == u.manager.BasicFamilyID()
		if arithmetical {
			mul, cause := u.isNonlinearMul(w.h)
			if cause != nil {
				return nil, cause
			}
			nl = nl || mul || u.enableLinear
		} else {
			nl = true
		}
		n, err := u.manager.GetNumArgs(w.h)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < n; i++ {
			a, cause := u.manager.GetArg(w.h, i)
			if cause != nil {
				return nil, cause
			}
			todo = append(todo, work{a, nl})
		}
	}
	return result, nil
}
