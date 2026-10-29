package ast

// Source: src/ast/format.h:132-162. Concrete format-pointer specialization.

import "math"

// FormatSeq3 builds the source split header sequence. The first split
// documents stay aligned after the header; the remaining documents use the
// outer indentation. Nil documents after the first are omitted by mk_seq.
func (m *ASTManager) FormatSeq3(header string, children []ASTHandle, split, indent uint32) (ASTHandle, error) {
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
	seq := func(documents []ASTHandle) (ASTHandle, error) {
		formatOwner, e := m.GetFormatManager()
		if e != nil {
			return ASTHandle{}, e
		}
		parts := make([]ASTHandle, 0, 2*len(documents))
		for _, document := range documents {
			kind, e := formatOwner.GetDeclKind(document)
			if e != nil {
				return ASTHandle{}, e
			}
			if kind == ASTFormatNil {
				continue
			}
			line, e := m.FormatLineBreak()
			if e != nil {
				return ASTHandle{}, e
			}
			parts = append(parts, line, document)
		}
		return m.FormatCompose(parts)
	}
	space, err := m.FormatString(" ")
	if err != nil {
		return ASTHandle{}, err
	}
	firstTail, err := seq(children[1:int(end)])
	if err != nil {
		return ASTHandle{}, err
	}
	first, err := m.FormatCompose([]ASTHandle{space, children[0], firstTail})
	if err != nil {
		return ASTHandle{}, err
	}
	first, err = m.FormatIndent(uint32(len(header)+2), first)
	if err != nil {
		return ASTHandle{}, err
	}
	first, err = m.FormatGroup(first)
	if err != nil {
		return ASTHandle{}, err
	}
	tail, err := seq(children[int(end):])
	if err != nil {
		return ASTHandle{}, err
	}
	tail, err = m.FormatIndent(indent, tail)
	if err != nil {
		return ASTHandle{}, err
	}
	whole, err := m.FormatCompose([]ASTHandle{left, head, first, tail, right})
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatGroup(whole)
}
