package qe

// Source: src/qe/qe_dl_plugin.cpp, update_eqs and get_num_branches.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

var errDatalogQELessThan = errors.New("datalog quantifier elimination: less-than is not implemented")

type datalogQEAtom struct{ atom, value ast.ASTHandle }
type datalogQEAtoms struct{ positive, negative []datalogQEAtom }

// datalogQEContains follows application arguments and quantifier bodies,
// matching contains_app; quantifier annotations are not visited.
func datalogQEContains(m *ast.ASTManager, term, variable ast.ASTHandle) (bool, error) {
	todo := []ast.ASTHandle{term}
	seen := map[ast.ASTHandle]bool{}
	for len(todo) > 0 {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if h == variable {
			return true, nil
		}
		if seen[h] {
			continue
		}
		seen[h] = true
		k, e := m.GetKind(h)
		if e != nil {
			return false, e
		}
		switch k {
		case ast.ASTApp:
			n, e := m.GetNumArgs(h)
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < n; i++ {
				a, e := m.GetArg(h, i)
				if e != nil {
					return false, e
				}
				todo = append(todo, a)
			}
		case ast.ASTQuantifier:
			b, e := m.GetQuantifierBody(h)
			if e != nil {
				return false, e
			}
			todo = append(todo, b)
		case ast.ASTVar:
		default:
			return false, ast.ErrASTDomain
		}
	}
	return false, nil
}

// collectDatalogQEAtoms scans positive atoms first, then negative atoms, in
// their supplied context order. A non-equality occurrence rejects the batch.
func collectDatalogQEAtoms(m *ast.ASTManager, variable ast.ASTHandle, positive, negative []ast.ASTHandle) (datalogQEAtoms, bool, error) {
	var result datalogQEAtoms
	family, e := m.RegisterDatalogPlugin()
	if e != nil {
		return result, false, e
	}
	for pass, atoms := range [][]ast.ASTHandle{positive, negative} {
		for _, atom := range atoms {
			contains, e := datalogQEContains(m, atom, variable)
			if e != nil {
				return datalogQEAtoms{}, false, e
			}
			if !contains {
				continue
			}
			kind, e := m.GetKind(atom)
			if e != nil {
				return datalogQEAtoms{}, false, e
			}
			if kind != ast.ASTApp {
				return datalogQEAtoms{}, false, nil
			}
			fid, e := m.GetFamilyID(atom)
			if e != nil {
				return datalogQEAtoms{}, false, e
			}
			op, e := m.GetDeclKind(atom)
			if e != nil {
				return datalogQEAtoms{}, false, e
			}
			if fid == family && op == 14 {
				return datalogQEAtoms{}, false, errDatalogQELessThan
			}
			if fid != m.BasicFamilyID() || op != ast.ASTOpEq {
				return datalogQEAtoms{}, false, nil
			}
			left, e := m.GetArg(atom, 0)
			if e != nil {
				return datalogQEAtoms{}, false, e
			}
			right, e := m.GetArg(atom, 1)
			if e != nil {
				return datalogQEAtoms{}, false, e
			}
			if right == variable {
				left, right = right, left
			}
			nested, e := datalogQEContains(m, right, variable)
			if e != nil {
				return datalogQEAtoms{}, false, e
			}
			if left != variable || nested {
				return datalogQEAtoms{}, false, nil
			}
			a := datalogQEAtom{atom, right}
			if pass == 0 {
				result.positive = append(result.positive, a)
			} else {
				result.negative = append(result.negative, a)
			}
		}
	}
	return result, true, nil
}

// datalogQEBranchCount enumerates the domain when the exclusions can cover
// every value. The equality boundary matters: with n distinct exclusions in
// an n-element domain, the synthetic "other" branch may be empty.
func datalogQEBranchCount(domain uint64, atoms datalogQEAtoms) (count uint64, small bool) {
	total := uint64(len(atoms.positive)) + uint64(len(atoms.negative))
	if domain <= total {
		return domain, true
	}
	return uint64(len(atoms.positive)) + 1, false
}
