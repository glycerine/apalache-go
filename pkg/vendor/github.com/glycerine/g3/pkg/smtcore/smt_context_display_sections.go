package smtcore

// Source: src/smt/smt_context_pp.cpp:162-196,286-288,307-314,526-539;
// src/smt/smt_relevancy.cpp:519-527.

import (
	"fmt"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

// DisplayClauseSections preserves the separate source auxiliary/lemma vectors.
// Packed binary clauses have no clause object and are printed from watches.
func (s *SMTInternalizerState) DisplayClauseSections(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	out = smtContextDisplayWriter{out}
	for _, lemma := range []bool{false, true} {
		first := true
		for i := range s.clauses {
			clause := s.clauses[i].clause
			if clause == nil || (clause.Kind() != SMTClauseAux) != lemma {
				continue
			}
			if first {
				header := "auxiliary clauses:\n"
				if lemma {
					header = "lemmas:\n"
				}
				if err := smtWriteTheoryText(out, header); err != nil {
					return err
				}
				first = false
			}
			if err := clause.DisplayCompact(out, s.boolExpr); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SMTInternalizerState) DisplayBinaryClauses(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	out = smtContextDisplayWriter{out}
	first := true
	for index := range s.watches {
		left := SMTLiteralFromIndex(int32(index))
		for _, right := range s.watches[index].Literals() {
			if left.Index() >= right.Index() {
				continue
			}
			if first {
				if err := smtWriteTheoryText(out, "binary clauses:\n"); err != nil {
					return err
				}
				first = false
			}
			if err := smtWriteTheoryText(out, "(clause "); err != nil {
				return err
			}
			if err := left.Negated().DisplayCompact(out, s.manager, s.boolExpr); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, " "); err != nil {
				return err
			}
			if err := right.DisplayCompact(out, s.manager, s.boolExpr); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, ")\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SMTInternalizerState) DisplayRelevantExpressions(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	p := s.relevancy
	if p == nil || !p.enabled || len(p.relevant) == 0 {
		return nil
	}
	if err := smtWriteTheoryText(out, "relevant exprs:\n"); err != nil {
		return err
	}
	for _, expression := range p.relevant {
		id, err := s.manager.GetASTID(expression)
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("#%d ", id)); err != nil {
			return err
		}
	}
	return smtWriteTheoryText(out, "\n")
}

type smtDisplayDeclarationRow struct {
	id    uint32
	nodes []*SMTEnode
}

func (s *SMTInternalizerState) DisplayDeclarationEnodes(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if err := smtWriteTheoryText(out, "decl2enodes:\n"); err != nil {
		return err
	}
	var rows []smtDisplayDeclarationRow
	for declaration, nodes := range s.context.byDeclaration {
		// Source mk_enode registers only non-equality enodes with arguments;
		// the Go checker also indexes constants and equalities for lookup.
		var retained []*SMTEnode
		for _, node := range nodes {
			if node.NumArgs() != 0 && !node.IsEquality() {
				retained = append(retained, node)
			}
		}
		if len(retained) == 0 {
			continue
		}
		id, err := s.manager.GetASTID(declaration)
		if err != nil {
			return err
		}
		// ast.h decl::get_decl_id removes the declaration-ID namespace bit.
		rows = append(rows, smtDisplayDeclarationRow{id: id - (1 << 31), nodes: retained})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	for _, row := range rows {
		if err := smtWriteTheoryText(out, fmt.Sprintf("id %d ->", row.id)); err != nil {
			return err
		}
		for _, node := range row.nodes {
			id, err := node.OwnerID()
			if err != nil {
				return err
			}
			if err = smtWriteTheoryText(out, fmt.Sprintf(" #%d", id)); err != nil {
				return err
			}
		}
		if err := smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}
