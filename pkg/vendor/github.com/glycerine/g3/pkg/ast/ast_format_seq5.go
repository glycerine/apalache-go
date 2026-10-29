package ast

// Source: src/ast/format.h:164-195. Concrete format-pointer specialization.

// FormatSeq5 builds the grouped parenthesized sequence used by SMT2 function
// signatures. Its source mk_seq tail skips nil documents but keeps the first.
func (m *ASTManager) FormatSeq5(children []ASTHandle) (ASTHandle, error) {
	left, err := m.FormatString("(")
	if err != nil {
		return ASTHandle{}, err
	}
	right, err := m.FormatString(")")
	if err != nil {
		return ASTHandle{}, err
	}
	if len(children) == 0 {
		return m.FormatCompose([]ASTHandle{left, right})
	}
	formatOwner, err := m.GetFormatManager()
	if err != nil {
		return ASTHandle{}, err
	}
	tail := make([]ASTHandle, 0, len(children)-1)
	for _, child := range children[1:] {
		kind, e := formatOwner.GetDeclKind(child)
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
		tail = append(tail, line, child)
	}
	seq, err := m.FormatCompose(tail)
	if err != nil {
		return ASTHandle{}, err
	}
	first, err := m.FormatCompose([]ASTHandle{left, children[0]})
	if err != nil {
		return ASTHandle{}, err
	}
	first, err = m.FormatIndent(1, first)
	if err != nil {
		return ASTHandle{}, err
	}
	last, err := m.FormatCompose([]ASTHandle{seq, right})
	if err != nil {
		return ASTHandle{}, err
	}
	last, err = m.FormatIndent(1, last)
	if err != nil {
		return ASTHandle{}, err
	}
	whole, err := m.FormatCompose([]ASTHandle{first, last})
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatGroup(whole)
}
