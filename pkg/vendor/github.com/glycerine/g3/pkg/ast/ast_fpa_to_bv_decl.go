package ast

// Source: src/ast/fpa_decl_plugin.cpp:608-649.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpToUBV int32 = astFPAToUBV
const FPAOpToSBV int32 = astFPAToSBV

func (m *ASTManager) floatingPointToBVDecl(kind int32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	name := "fp.to_ubv"
	if kind == astFPAToSBV {
		name = "fp.to_sbv"
	}
	fail := func(message string) (astNodeRef, error) {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException(message)}
	}
	if len(domain) != 2 {
		return fail("invalid number of arguments to " + name)
	}
	if len(ps) != 1 {
		return fail("invalid number of parameters to " + name)
	}
	param, err := m.parameterOfKind(ps[0], ASTParameterInt)
	if err != nil {
		return fail("invalid parameter type; " + name + " expects an int parameter")
	}
	for i, wanted := range [...]int32{1, 0} {
		ref, e := m.internal(domain[i])
		if e != nil {
			return astNodeRef{}, e
		}
		valid := false
		if ref.slot != 0 && m.nodeAt(ref.slot).kind == ASTSort {
			info, e := m.sortInfo(m.sortAt(ref.slot).info)
			if e != nil {
				return astNodeRef{}, e
			}
			valid = info.family == m.floatingPointPlugin.family && info.kind == wanted
		}
		if !valid {
			if i == 0 {
				return fail("sort mismatch, expected first argument of RoundingMode sort")
			}
			return fail("sort mismatch, expected second argument of FloatingPoint sort")
		}
	}
	if param.integer <= 0 {
		return fail("invalid parameter value; " + name + " expects a parameter larger than 0")
	}
	bvSort, err := m.bitVectorPlugin.mkSort(uint32(param.integer))
	if err != nil {
		return astNodeRef{}, err
	}
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(symbol, m.floatingPointPlugin.family, kind, ps, domain, bvSort, ASTFuncFlags{}, false)
}
