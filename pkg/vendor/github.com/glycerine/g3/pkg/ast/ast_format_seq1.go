package ast

// Source: src/ast/format.h:67-108. Concrete format-pointer specialization.

import "math"

// FormatSeq1 constructs the source grouped (header arg...) document used by
// SMT2 indexed declarations and sort applications. Nil tail documents are
// skipped by the source mk_seq loop; the first document is always retained.
func (m *ASTManager) FormatSeq1(header string, children []ASTHandle) (document ASTHandle, resultErr error) {
	documents, err := newASTFormatDocuments(m)
	if err != nil {
		return ASTHandle{}, err
	}
	defer documents.finish(&document, &resultErr)
	left, err := documents.own(m.FormatString("("))
	if err != nil {
		return ASTHandle{}, err
	}
	head, err := documents.own(m.FormatString(header))
	if err != nil {
		return ASTHandle{}, err
	}
	right, err := documents.own(m.FormatString(")"))
	if err != nil {
		return ASTHandle{}, err
	}
	if len(children) == 0 {
		return documents.own(m.FormatCompose([]ASTHandle{left, head, right}))
	}
	if uint64(len(header))+2 > math.MaxUint32 {
		return ASTHandle{}, ErrASTDomain
	}
	space, err := documents.own(m.FormatString(" "))
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
		breakDoc, e := documents.own(m.FormatLineBreak())
		if e != nil {
			return ASTHandle{}, e
		}
		tail = append(tail, breakDoc, child)
	}
	seq, err := documents.own(m.FormatCompose(tail))
	if err != nil {
		return ASTHandle{}, err
	}
	body, err := documents.own(m.FormatCompose([]ASTHandle{space, children[0], seq, right}))
	if err != nil {
		return ASTHandle{}, err
	}
	indented, err := documents.own(m.FormatIndent(uint32(len(header)+2), body))
	if err != nil {
		return ASTHandle{}, err
	}
	whole, err := documents.own(m.FormatCompose([]ASTHandle{left, head, indented}))
	if err != nil {
		return ASTHandle{}, err
	}
	return documents.own(m.FormatGroup(whole))
}
