package smtcore

// Source: src/smt/smt_context_pp.cpp:198-208,223-262.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Adapt ostream's failed-write detection for the existing expression printers.
// A trace caller can wrap its stream in assertedTraceWriter first to retain
// native sticky-badbit operand evaluation.
type smtContextDisplayWriter struct{ io.Writer }

func (w smtContextDisplayWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

func (s *SMTInternalizerState) DisplayAssignment(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if len(s.assignedLiterals) == 0 {
		return nil
	}
	out = smtContextDisplayWriter{out}
	if err := smtWriteTheoryText(out, "current assignment:\n"); err != nil {
		return err
	}
	for _, literal := range s.assignedLiterals {
		if err := literal.DisplayCompact(out, s.manager, s.boolExpr); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, ": "); err != nil {
			return err
		}
		if err := s.DisplayLiteralVerbose(out, literal); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *SMTInternalizerState) DisplayEquivalenceClasses(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	out = smtContextDisplayWriter{out}
	printer, err := ast.NewSimpleASTPrinterContext(s.manager, out, out)
	if err != nil {
		return err
	}
	first := true
	for _, node := range s.enodeStack {
		root := node.root
		if node.owner == root.owner {
			continue
		}
		if first {
			if err = smtWriteTheoryText(out, "equivalence classes:\n"); err != nil {
				return err
			}
			first = false
		}
		id, err := node.OwnerID()
		if err != nil {
			return err
		}
		rootID, err := root.OwnerID()
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("#%d -> #%d: ", id, rootID)); err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, node.owner, 0); err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, " -> "); err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, root.owner, 0); err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *SMTInternalizerState) DisplayExpressionBoolVarMap(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if len(s.booleanStack) == 0 {
		return nil
	}
	if err := smtWriteTheoryText(out, "expression -> bool_var:\n"); err != nil {
		return err
	}
	for _, expression := range s.booleanStack {
		id, err := s.manager.GetASTID(expression)
		if err != nil {
			return err
		}
		variable, ok := s.boolVar[expression]
		if !ok {
			return ast.ErrASTDomain
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("(#%d -> p!%d) ", id, variable)); err != nil {
			return err
		}
	}
	return smtWriteTheoryText(out, "\n")
}

func (s *SMTInternalizerState) DisplayApplicationEnodeMap(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if len(s.enodeStack) == 0 {
		return nil
	}
	if err := smtWriteTheoryText(out, "expression -> enode:\n"); err != nil {
		return err
	}
	for i, node := range s.enodeStack {
		id, err := node.OwnerID()
		if err != nil {
			return err
		}
		// Source e!i is the internalization-stack position, not the root ID.
		if err = smtWriteTheoryText(out, fmt.Sprintf("(#%d -> e!%d) ", id, i)); err != nil {
			return err
		}
	}
	return smtWriteTheoryText(out, "\n")
}
