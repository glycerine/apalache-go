package ast

// Source: src/ast/format.h:132-162. Concrete format-pointer specialization.

import "math"

// FormatSeq3 builds the source split header sequence. The first split
// documents stay aligned after the header; the remaining documents use the
// outer indentation. Nil documents after the first are omitted by mk_seq.
func (m *ASTManager) FormatSeq3(header string, children []ASTHandle, split, indent uint32) (document ASTHandle, resultErr error) {
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
	if split == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	if uint64(len(header))+2 > math.MaxUint32 {
		return ASTHandle{}, ErrASTDomain
	}
	end := uint64(split)
	if end > uint64(len(children)) {
		end = uint64(len(children))
	}
	seq := func(children []ASTHandle) (ASTHandle, error) {
		formatOwner, e := m.GetFormatManager()
		if e != nil {
			return ASTHandle{}, e
		}
		parts := make([]ASTHandle, 0, 2*len(children))
		for _, document := range children {
			kind, e := formatOwner.GetDeclKind(document)
			if e != nil {
				return ASTHandle{}, e
			}
			if kind == ASTFormatNil {
				continue
			}
			line, e := documents.own(m.FormatLineBreak())
			if e != nil {
				return ASTHandle{}, e
			}
			parts = append(parts, line, document)
		}
		return documents.own(m.FormatCompose(parts))
	}
	space, err := documents.own(m.FormatString(" "))
	if err != nil {
		return ASTHandle{}, err
	}
	firstTail, err := seq(children[1:int(end)])
	if err != nil {
		return ASTHandle{}, err
	}
	first, err := documents.own(m.FormatCompose([]ASTHandle{space, children[0], firstTail}))
	if err != nil {
		return ASTHandle{}, err
	}
	first, err = documents.own(m.FormatIndent(uint32(len(header)+2), first))
	if err != nil {
		return ASTHandle{}, err
	}
	first, err = documents.own(m.FormatGroup(first))
	if err != nil {
		return ASTHandle{}, err
	}
	tail, err := seq(children[int(end):])
	if err != nil {
		return ASTHandle{}, err
	}
	tail, err = documents.own(m.FormatIndent(indent, tail))
	if err != nil {
		return ASTHandle{}, err
	}
	whole, err := documents.own(m.FormatCompose([]ASTHandle{left, head, first, tail, right}))
	if err != nil {
		return ASTHandle{}, err
	}
	return documents.own(m.FormatGroup(whole))
}
