package ast

// Source: src/ast/ast_smt2_pp.cpp:738-801; src/ast/format.h:67-79.

// smt2FormatTail is the concrete format-pointer mk_seq loop: a line break
// precedes each non-nil document in the supplied suffix.
func (m *ASTManager) smt2FormatTail(args []ASTHandle) (ASTHandle, error) {
	formatOwner, err := m.GetFormatManager()
	if err != nil {
		return ASTHandle{}, err
	}
	parts := make([]ASTHandle, 0, 2*len(args))
	for _, arg := range args {
		kind, e := formatOwner.GetDeclKind(arg)
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
		parts = append(parts, breakDoc, arg)
	}
	return m.FormatCompose(parts)
}

// SMT2PPAppFormat builds process_app's source document after the expression
// traversal has produced ordered child documents. Constant and label/pattern
// dispatch remain the surrounding traversal's responsibility.
func (m *ASTManager) SMT2PPAppFormat(decl ASTHandle, args []ASTHandle) (ASTHandle, error) {
	if len(args) == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	fname, length, err := m.SMT2Fdecl(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	left, err := m.FormatString("(")
	if err != nil {
		return ASTHandle{}, err
	}
	right, err := m.FormatString(")")
	if err != nil {
		return ASTHandle{}, err
	}
	header, err := m.FormatCompose([]ASTHandle{left, fname})
	if err != nil {
		return ASTHandle{}, err
	}
	header, err = m.FormatIndent(1, header)
	if err != nil {
		return ASTHandle{}, err
	}
	if length > 16 {
		tail, e := m.smt2FormatTail(args)
		if e != nil {
			return ASTHandle{}, e
		}
		body, e := m.FormatCompose([]ASTHandle{tail, right})
		if e != nil {
			return ASTHandle{}, e
		}
		body, e = m.FormatIndent(2, body)
		if e != nil {
			return ASTHandle{}, e
		}
		whole, e := m.FormatCompose([]ASTHandle{header, body})
		if e != nil {
			return ASTHandle{}, e
		}
		return m.FormatGroup(whole)
	}
	space, err := m.FormatString(" ")
	if err != nil {
		return ASTHandle{}, err
	}
	tail, err := m.smt2FormatTail(args[1:])
	if err != nil {
		return ASTHandle{}, err
	}
	body, err := m.FormatCompose([]ASTHandle{space, args[0], tail, right})
	if err != nil {
		return ASTHandle{}, err
	}
	body, err = m.FormatIndent(length+2, body)
	if err != nil {
		return ASTHandle{}, err
	}
	whole, err := m.FormatCompose([]ASTHandle{header, body})
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatGroup(whole)
}
