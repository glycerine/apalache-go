package ast

// Source: src/ast/fpa_decl_plugin.cpp:940-948.

import "github.com/glycerine/g3/pkg/util/diagnostic"

// These are the asserting fpa_util accessors, distinct from the nonasserting
// FloatingPointSortShape recognizer. Callers already hold the AST lock.
func (m *ASTManager) fpaUtilExponentBits(sort astNodeRef) (uint32, error) {
	return m.fpaUtilWidth(sort, 0, 941)
}
func (m *ASTManager) fpaUtilSignificandBits(sort astNodeRef) (uint32, error) {
	return m.fpaUtilWidth(sort, 1, 946)
}
func (m *ASTManager) fpaUtilWidth(sort astNodeRef, index uint32, line int32) (uint32, error) {
	if err := m.validNode(sort); err != nil {
		return 0, err
	}
	if m.nodeAt(sort.slot).kind != ASTSort {
		return 0, ErrASTDomain
	}
	p := m.floatingPointPlugin
	if p == nil || !p.published || p.sourceEnded {
		return 0, ErrASTHandle
	}
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil {
		return 0, err
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, err := diagnostic.SourceAssertionsEnabled(d)
		if err != nil {
			return 0, err
		}
		if enabled && (info.family != p.family || info.kind != 0) {
			if err = diagnostic.DebugAssertionFailure(d, "src/ast/fpa_decl_plugin.cpp", line, "is_float(s)"); err != nil {
				return 0, err
			}
		}
	}
	// A continued/erased SASSERT still reaches the literal source parameter
	// read. Checked arena bounds stop absent parameters, without inventing widths.
	parameter, err := m.infoParameter(info, index)
	if err != nil {
		return 0, err
	}
	value, err := m.parameterOfKind(parameter, ASTParameterInt)
	if err != nil {
		return 0, err
	}
	return uint32(value.integer), nil
}
