package ast

// Source: src/ast/ast.cpp:2312-2327; src/ast/ast.h:1867-1874.

// MkPattern constructs the concrete pattern-plugin application used by
// quantifier triggers.
func (m *ASTManager) MkPattern(exprs []ASTHandle) (ASTHandle, error) {
	if ASTSourceDebug {
		for _, h := range exprs {
			kind, err := m.GetKind(h)
			if err != nil {
				return ASTHandle{}, err
			}
			if kind != ASTApp {
				return ASTHandle{}, ErrASTDomain
			}
		}
	}
	return m.MkFamilyApp(2, 0, nil, exprs)
}

func (m *ASTManager) IsPattern(h ASTHandle) (bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != 2 {
		return false, err
	}
	op, err := m.GetDeclKind(h)
	return op == 0, err
}
