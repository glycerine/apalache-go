package smtcore

// Source: src/smt/theory_lra.cpp:2455-2468 display.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Display is consumed by context::display_theories, including before a model
// is initialized. It preserves the source's conditional cache reads.
func (t *SMTLRATheory) Display(out io.Writer) error {
	if t == nil {
		return ast.ErrASTHandle
	}
	s, err := t.State()
	if err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if t.numeric != nil && t.numeric.solver != nil {
		if err = t.numeric.solver.PrintConstraints(out); err != nil {
			return err
		}
		if err = t.numeric.solver.PrintTerms(out); err != nil {
			return err
		}
	}
	for v, node := range t.variables {
		if err = smtWriteTheoryText(out, fmt.Sprintf("v%d", v)); err != nil {
			return err
		}
		if t.canGetModelValue(SMTTheoryVar(v)) {
			value, err := t.cachedModelValue(SMTTheoryVar(v))
			if err != nil {
				return err
			}
			if err = smtWriteTheoryText(out, ", value: "+value.RatString()); err != nil {
				return err
			}
		}
		shared, err := t.sharedNode(node)
		if err != nil {
			return err
		}
		relevant := s.relevancy == nil || s.relevancy.IsRelevant(node.owner)
		sharedInt, relevantInt := 0, 0
		if shared {
			sharedInt = 1
		}
		if relevant {
			relevantInt = 1
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf(", shared: %d, rel: %d, def: ", sharedInt, relevantInt)); err != nil {
			return err
		}
		if err = t.DisplayFlatApplication(out, node.owner); err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}
