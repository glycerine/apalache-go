package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:403-408,490-523;
// src/ast/rewriter/bv_rewriter.h:35,41-44.

import "github.com/glycerine/g3/pkg/ast"

// bvPowerProduct is the original identity-only overload. Its result borrows
// the input's lifetime; numerical parameters are neither copied nor read.
func bvPowerProduct(m *ast.ASTManager, t ast.ASTHandle) (ast.ASTHandle, error) {
	if m == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	mul, e := m.IsBitVectorOperation(t, 6)
	if e != nil || !mul {
		return t, e
	}
	count, e := m.GetNumArgs(t)
	if e != nil || count != 2 {
		return t, e
	}
	first, e := m.GetArg(t, 0)
	if e != nil {
		return t, e
	}
	numeral, e := m.IsBitVectorNumeral(first)
	if e != nil || !numeral {
		return t, e
	}
	return m.GetArg(t, 1)
}

func bvMonomialLess(m *ast.ASTManager, fid ast.FamilyID, astOrder bool, left, right ast.ASTHandle) (bool, error) {
	if m == nil {
		return false, ast.ErrASTHandle
	}
	if astOrder {
		return m.ASTLess(left, right)
	}
	l, e := bvMonomialOrdinal(m, fid, left)
	if e != nil {
		return false, e
	}
	r, e := bvMonomialOrdinal(m, fid, right)
	return l < r, e
}

// bvMonomialOrdinal specializes mon_lt::ordinal for use_power()==false.
// The source default rational k has no numerical acquisition in this branch.
func bvMonomialOrdinal(m *ast.ASTManager, fid ast.FamilyID, t ast.ASTHandle) (int32, error) {
	if m == nil {
		return 0, ast.ErrASTHandle
	}
	kind, e := m.GetKind(t)
	if e != nil {
		return 0, e
	}
	if kind != ast.ASTVar {
		if kind != ast.ASTApp {
			return 0, ast.ErrASTDomain
		}
		family, e := m.GetFamilyID(t)
		if e != nil {
			return 0, e
		}
		if family == fid {
			mul, e := m.IsBitVectorOperation(t, 6)
			if e != nil {
				return 0, e
			}
			if mul {
				// Unlike get_power_product, this source branch has no arity guard.
				first, e := m.GetArg(t, 0)
				if e != nil {
					return 0, e
				}
				numeral, e := m.IsBitVectorNumeral(first)
				if e != nil {
					return 0, e
				}
				if numeral {
					t, e = m.GetArg(t, 1)
					if e != nil {
						return 0, e
					}
				}
			} else {
				numeral, e := m.IsBitVectorNumeral(t)
				if e != nil {
					return 0, e
				}
				if numeral {
					return -1, nil
				}
			}
		}
	}
	id, e := m.GetASTID(t)
	// Preserve the source unsigned-ID to 32-bit-int conversion, including wrap.
	return int32(id), e
}
