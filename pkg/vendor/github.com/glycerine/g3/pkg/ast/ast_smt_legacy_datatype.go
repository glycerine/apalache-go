package ast

// Source: ast_smt_pp.cpp smt_printer::pp_dt and pp_sort_decl.

import (
	"strconv"
	"strings"
)

func (m *ASTManager) legacyDatatypeDefinitions(sort ASTHandle) ([]DatatypeDefinitionDescription, error) {
	if err := m.lock(); err != nil {
		return nil, err
	}
	r, err := m.requireKind(sort, ASTSort)
	var generic ASTHandle
	if err == nil {
		var definition *astDatatypeDefinition
		definition, err = m.datatypeDefinitionForSort(r)
		if err == nil {
			generic = m.external(definition.sort)
		}
	}
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return m.DatatypeDefinitions(generic)
}

func (p *LegacySMTPrinter) datatypeDeclarationText(sort ASTHandle, marked map[ASTHandle]bool) (string, error) {
	definitions, err := p.manager.legacyDatatypeDefinitions(sort)
	if err != nil {
		return "", err
	}
	for _, definition := range definitions {
		if marked[definition.Sort] {
			return "", nil
		}
		marked[definition.Sort] = true
	}
	var out strings.Builder
	out.WriteString("(declare-datatypes (")
	for i, definition := range definitions {
		if i != 0 {
			out.WriteString("\n    ")
		}
		name, err := p.manager.Symbols.Display(definition.Name)
		if err != nil {
			return "", err
		}
		out.WriteString("(" + name + " " + strconv.Itoa(len(definition.Parameters)) + ")")
	}
	out.WriteString(") (")
	for i, definition := range definitions {
		if i != 0 {
			out.WriteString("\n   ")
		}
		if len(definition.Parameters) != 0 {
			out.WriteString("(par (")
			for j, parameter := range definition.Parameters {
				if j != 0 {
					out.WriteByte(' ')
				}
				text, err := p.sortText(parameter)
				if err != nil {
					return "", err
				}
				out.WriteString(text)
			}
			out.WriteByte(')')
		}
		name, err := p.renamedText(definition.Name)
		if err != nil {
			return "", err
		}
		out.WriteString("(" + name + " ")
		for j, constructor := range definition.Constructors {
			if j != 0 {
				out.WriteByte(' ')
			}
			name, err := p.renamedText(constructor.Name)
			if err != nil {
				return "", err
			}
			out.WriteString("(" + name)
			for _, accessor := range constructor.Accessors {
				field, err := p.renamedText(accessor.Name)
				if err != nil {
					return "", err
				}
				text, err := p.sortText(accessor.Range)
				if err != nil {
					return "", err
				}
				out.WriteString(" (" + field + " " + text + ")")
			}
			out.WriteByte(')')
		}
		if len(definition.Parameters) != 0 {
			out.WriteByte(')')
		}
		out.WriteByte(')')
	}
	out.WriteString("))\n")
	return out.String(), nil
}
