package api

// Source: src/api/api_ast.cpp:306-327; src/api/api_numeral.cpp:28-38.

import "github.com/glycerine/g3/pkg/ast"

func apiRegisteredFamilyName(m *ast.ASTManager, family ast.FamilyID) (string, error) {
	if family == ast.NullFamilyID {
		return "", nil
	}
	name, err := m.FamilyName(family)
	if err != nil {
		return "", err
	}
	return m.Symbols.String(name)
}

// Z3_get_ast_kind tests the sort's family and the plugin's unique-value
// predicate. Printable numeral recognition has a different source contract:
// rounding modes and pi/e are unique, while algebraics and FP_NUM are not.
func apiASTNumeralKind(m *ast.ASTManager, value ast.ASTHandle) (bool, error) {
	sort, err := m.GetSort(value)
	if err != nil {
		return false, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	name, err := apiRegisteredFamilyName(m, family)
	if err != nil {
		return false, err
	}
	switch name {
	case "arith", "bv", "datalog_relation", "fpa":
		return m.IsUniqueValue(value)
	default:
		return false, nil
	}
}
