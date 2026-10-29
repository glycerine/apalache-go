package ast

// Source: src/ast/format.h:67-108. Concrete format-pointer specialization.

import "math"

// FormatSeq1 constructs the source grouped (header arg...) document used by
// SMT2 indexed declarations and sort applications. Nil tail documents are
// skipped by the source mk_seq loop; the first document is always retained.
func (m *ASTManager) FormatSeq1(header string, children []ASTHandle) (ASTHandle, error) {
	left, err := m.FormatString("(")
	if err != nil {
		return ASTHandle{}, err
	}
	head, err := m.FormatString(header)
	if err != nil {
		return ASTHandle{}, err
	}
	right, err := m.FormatString(")")
	if err != nil {
		return ASTHandle{}, err
	}
	if len(children) == 0 {
		return m.FormatCompose([]ASTHandle{left, head, right})
	}
	if uint64(len(header))+2 > math.MaxUint32 {
		return ASTHandle{}, ErrASTDomain
	}
	space, err := m.FormatString(" ")
	if err != nil {
		return ASTHandle{}, err
	}
	formatOwner, err := m.GetFormatManager()
	if err != nil {
		return ASTHandle{}, err
	}
	tail := make([]ASTHandle, 0, 2*(len(children)-1))
	for _, child := range children[1:] {
		kind, e := formatOwner.GetDeclKind(child)
		if e != nil {
			return ASTHandle{}, e
		}
		if kind == ASTFormatNil {
			continue
		}
		breakDoc, e := m.FormatLineBreak()
		if e != nil {
			return ASTHandle{}, e
		}
		tail = append(tail, breakDoc, child)
	}
	seq, err := m.FormatCompose(tail)
	if err != nil {
		return ASTHandle{}, err
	}
	body, err := m.FormatCompose([]ASTHandle{space, children[0], seq, right})
	if err != nil {
		return ASTHandle{}, err
	}
	indented, err := m.FormatIndent(uint32(len(header)+2), body)
	if err != nil {
		return ASTHandle{}, err
	}
	whole, err := m.FormatCompose([]ASTHandle{left, head, indented})
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatGroup(whole)
}
