package proofs

// Source: src/ast/proofs/proof_utils.cpp:751-814.

import "github.com/glycerine/g3/pkg/ast"

// IsClosed follows proof_is_closed's scoped-lemma walk. Its visited set is
// shared across lemma scopes, including the source's partial-check behavior
// for a proof node reached again beneath another lemma.
func IsClosed(m *ast.ASTManager, root ast.ASTHandle) (bool, error) {
	if m == nil {
		return false, ast.ErrASTHandle
	}
	seen := make(map[ast.ASTHandle]struct{})
	var literals []ast.ASTHandle
	var check func(ast.ASTHandle) (bool, error)
	check = func(p ast.ASTHandle) (bool, error) {
		if _, found := seen[p]; found {
			return true, nil
		}
		lemma, err := m.IsProofKind(p, ast.ASTPrLemma)
		if err != nil {
			return false, err
		}
		hypothesis, err := m.IsProofKind(p, ast.ASTPrHypothesis)
		if err != nil {
			return false, err
		}
		seen[p] = struct{}{}
		switch {
		case lemma:
			fact, e := m.GetProofFact(p)
			if e != nil {
				return false, e
			}
			old := len(literals)
			literals = append(literals, fact)
			factKind, e := m.GetKind(fact)
			if e != nil {
				return false, e
			}
			if factKind == ast.ASTApp {
				decl, declErr := m.GetDecl(fact)
				if declErr != nil {
					return false, declErr
				}
				family, familyErr := m.GetFamilyID(decl)
				if familyErr != nil {
					return false, familyErr
				}
				kind, kindErr := m.GetDeclKind(decl)
				if kindErr != nil {
					return false, kindErr
				}
				if family == m.BasicFamilyID() && kind == ast.ASTOpOr {
					n, countErr := m.GetNumArgs(fact)
					if countErr != nil {
						return false, countErr
					}
					for i := uint32(0); i < n; i++ {
						arg, argErr := m.GetArg(fact, i)
						if argErr != nil {
							return false, argErr
						}
						literals = append(literals, arg)
					}
				}
			}
			n, e := m.GetProofNumParents(p)
			if e != nil {
				return false, e
			}
			if n != 1 {
				return false, ast.ErrASTDomain
			}
			parent, e := m.GetProofParent(p, 0)
			if e != nil {
				return false, e
			}
			closed, e := check(parent)
			literals = literals[:old]
			return closed, e
		case hypothesis:
			fact, e := m.GetProofFact(p)
			if e != nil {
				return false, e
			}
			for _, lit := range literals {
				if (m.IsTrue(lit) && m.IsFalse(fact)) || (m.IsFalse(lit) && m.IsTrue(fact)) {
					return true, nil
				}
				isNot, child, e := m.IsNot(lit)
				if e != nil {
					return false, e
				}
				if isNot && child == fact {
					return true, nil
				}
				isNot, child, e = m.IsNot(fact)
				if e != nil {
					return false, e
				}
				if isNot && child == lit {
					return true, nil
				}
			}
			return false, nil
		default:
			n, e := m.GetProofNumParents(p)
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < n; i++ {
				parent, parentErr := m.GetProofParent(p, i)
				if parentErr != nil {
					return false, parentErr
				}
				closed, checkErr := check(parent)
				if checkErr != nil || !closed {
					return closed, checkErr
				}
			}
			return true, nil
		}
	}
	return check(root)
}
