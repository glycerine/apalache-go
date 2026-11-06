package base

// Source: src/muz/base/dl_context.cpp:1072-1105 get_rules_as_formulas for
// the finalized rule set. Pending raw formulas are owned by the context.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// AppendDisplayFormulas appends owning formula references to the caller's
// vectors. Output-predicate clauses become queries; the other clauses keep
// their names. On an error, the caller still owns every appended prefix.
func (s *RuleSet) AppendDisplayFormulas(rules, queries *[]ast.ASTRef, names *[]string) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	if names == nil {
		return ast.ErrASTDomain
	}
	var labels []symbol.Symbol
	err := s.AppendDisplayFormulasWithSymbols(rules, queries, &labels)
	for _, label := range labels {
		text := ""
		if label != (symbol.Symbol{}) {
			var nameErr error
			text, nameErr = s.manager.manager.Symbols.String(label)
			if nameErr != nil {
				return errors.Join(err, nameErr)
			}
		}
		*names = append(*names, text)
	}
	return err
}

// AppendDisplayFormulasWithSymbols preserves the source label identities.
// The caller owns every appended formula/query prefix, including on failure.
func (s *RuleSet) AppendDisplayFormulasWithSymbols(rules, queries *[]ast.ASTRef, names *[]symbol.Symbol) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	if rules == nil || queries == nil || names == nil || rules == queries {
		return ast.ErrASTDomain
	}
	m := s.manager.manager
	for _, rule := range s.rules {
		if err := func() (resultErr error) {
			owner := &contextSMT2Owner{manager: m}
			defer func() { resultErr = owner.finish(resultErr) }()
			formula, err := rule.ToFormula()
			if formula != (ast.ASTRef{}) {
				owner.refs = append(owner.refs, formula)
			}
			if err != nil {
				return err
			}
			declaration, err := rule.HeadDeclaration()
			if err != nil {
				return err
			}
			output, err := s.IsOutputPredicate(declaration)
			if err != nil {
				return err
			}
			if output {
				if err = s.makeDisplayQuery(formula); err != nil {
					return err
				}
				*queries = append(*queries, formula)
			} else {
				name, err := rule.NameSymbol()
				if err != nil {
					return err
				}
				*rules = append(*rules, formula)
				*names = append(*names, name)
			}
			owner.refs = nil // The caller owns the successfully appended formula.
			return nil
		}(); err != nil {
			return err
		}
	}
	return nil
}

func (s *RuleSet) makeDisplayQuery(formula ast.ASTRef) error {
	m := s.manager.manager
	root, err := m.RefGet(formula)
	if err != nil {
		return err
	}
	kind, err := m.GetKind(root)
	if err != nil {
		return err
	}
	body := root
	if kind == ast.ASTQuantifier {
		body, err = m.GetQuantifierBody(root)
		if err != nil {
			return err
		}
	}
	op, err := m.GetDeclKind(body)
	if err != nil {
		return err
	}
	if op != ast.ASTOpImplies {
		return nil
	}
	body, err = m.GetArg(body, 0)
	if err != nil {
		return err
	}
	if kind != ast.ASTQuantifier {
		return m.SetRef(formula, body)
	}
	count, err := m.GetNumDeclarations(root)
	if err != nil {
		return err
	}
	sorts, names := make([]ast.ASTHandle, count), make([]symbol.Symbol, count)
	for i := uint32(0); i < count; i++ {
		sorts[i], err = m.GetDeclarationSort(root, i)
		if err != nil {
			return err
		}
		names[i], err = m.GetDeclarationName(root, i)
		if err != nil {
			return err
		}
	}
	// Source builds a fresh existential binder with default weight and IDs;
	// a quantified fact takes the earlier no-implication branch unchanged.
	query, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: sorts, Names: names, Body: body, Weight: 0})
	return errors.Join(err, m.SetRef(formula, query))
}
