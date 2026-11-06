package ast

// Source: src/ast/ast_smt2_pp.cpp:738-801; src/ast/format.h:67-79.

// smt2FormatTail is the concrete format-pointer mk_seq loop: a line break
// precedes each non-nil document in the supplied suffix.
func (m *ASTManager) smt2FormatTail(args []ASTHandle) (document ASTHandle, resultErr error) {
	documents, err := newASTFormatDocuments(m)
	if err != nil {
		return ASTHandle{}, err
	}
	defer documents.finish(&document, &resultErr)
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
		breakDoc, e := documents.own(m.FormatLineBreak())
		if e != nil {
			return ASTHandle{}, e
		}
		parts = append(parts, breakDoc, arg)
	}
	return documents.own(m.FormatCompose(parts))
}

// SMT2PPAppFormat builds process_app's source document after the expression
// traversal has produced ordered child documents. Constant and label/pattern
// dispatch remain the surrounding traversal's responsibility.
func (m *ASTManager) SMT2PPAppFormat(decl ASTHandle, args []ASTHandle) (document ASTHandle, resultErr error) {
	documents, err := newASTFormatDocuments(m)
	if err != nil {
		return ASTHandle{}, err
	}
	defer documents.finish(&document, &resultErr)
	if len(args) == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	fname, length, err := m.SMT2Fdecl(decl)
	fname, err = documents.own(fname, err)
	if err != nil {
		return ASTHandle{}, err
	}
	left, err := documents.own(m.FormatString("("))
	if err != nil {
		return ASTHandle{}, err
	}
	right, err := documents.own(m.FormatString(")"))
	if err != nil {
		return ASTHandle{}, err
	}
	header, err := documents.own(m.FormatCompose([]ASTHandle{left, fname}))
	if err != nil {
		return ASTHandle{}, err
	}
	header, err = documents.own(m.FormatIndent(1, header))
	if err != nil {
		return ASTHandle{}, err
	}
	if length > 16 {
		tail, e := documents.own(m.smt2FormatTail(args))
		if e != nil {
			return ASTHandle{}, e
		}
		body, e := documents.own(m.FormatCompose([]ASTHandle{tail, right}))
		if e != nil {
			return ASTHandle{}, e
		}
		body, e = documents.own(m.FormatIndent(2, body))
		if e != nil {
			return ASTHandle{}, e
		}
		whole, e := documents.own(m.FormatCompose([]ASTHandle{header, body}))
		if e != nil {
			return ASTHandle{}, e
		}
		return documents.own(m.FormatGroup(whole))
	}
	space, err := documents.own(m.FormatString(" "))
	if err != nil {
		return ASTHandle{}, err
	}
	tail, err := documents.own(m.smt2FormatTail(args[1:]))
	if err != nil {
		return ASTHandle{}, err
	}
	body, err := documents.own(m.FormatCompose([]ASTHandle{space, args[0], tail, right}))
	if err != nil {
		return ASTHandle{}, err
	}
	body, err = documents.own(m.FormatIndent(length+2, body))
	if err != nil {
		return ASTHandle{}, err
	}
	whole, err := documents.own(m.FormatCompose([]ASTHandle{header, body}))
	if err != nil {
		return ASTHandle{}, err
	}
	return documents.own(m.FormatGroup(whole))
}
