package ast

// Source: src/ast/ast_smt2_pp.cpp:1117-1131.

// SMT2PPDeclareFun builds smt2_printer::operator()(func_decl*)'s declaration
// document. This entrypoint uses pp_fdecl_name, not the indexed pp_fdecl form.
func (m *ASTManager) SMT2PPDeclareFun(decl ASTHandle) (result ASTHandle, resultErr error) {
	documents, err := newASTFormatDocuments(m)
	if err != nil {
		return ASTHandle{}, err
	}
	defer documents.finish(&result, &resultErr)
	name, _, err := m.SMT2FdeclNameDecl(decl)
	name, err = documents.own(name, err)
	if err != nil {
		return ASTHandle{}, err
	}
	arity, err := m.GetArity(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	domains := make([]ASTHandle, 0, arity)
	for i := uint32(0); i < arity; i++ {
		sort, e := m.GetDomain(decl, i)
		if e != nil {
			return ASTHandle{}, e
		}
		format, e := documents.own(m.SMT2PPSort(sort))
		if e != nil {
			return ASTHandle{}, e
		}
		domains = append(domains, format)
	}
	printedDomains, err := documents.own(m.FormatSeq5(domains))
	if err != nil {
		return ASTHandle{}, err
	}
	rangeSort, err := m.GetRange(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	printedRange, err := documents.own(m.SMT2PPSort(rangeSort))
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatSeq1("declare-fun", []ASTHandle{name, printedDomains, printedRange})
}
