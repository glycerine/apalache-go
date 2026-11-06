package smtcore

// Source: src/smt/smt_context_pp.cpp:404-481 display_lemma_as_smt_problem;
// src/smt/smt_context.h:493-500 literal2expr.

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// The source's local expr_ref/vector borrow the manager. A refused destructor
// leaves these concrete roots reachable through the owning internalizer.
type smtLemmaProblem struct {
	manager  *ast.ASTManager
	current  ast.ASTRef
	refs     []ast.ASTRef
	formulas []ast.ASTHandle
}

func (p *smtLemmaProblem) close() error {
	if p.current != (ast.ASTRef{}) {
		if err := p.manager.DelRef(&p.current); err != nil {
			return err
		}
	}
	for len(p.refs) > 0 {
		i := len(p.refs) - 1
		if err := p.manager.DelRef(&p.refs[i]); err != nil {
			return err
		}
		p.refs = p.refs[:i]
	}
	p.formulas = nil
	return nil
}
func (p *smtLemmaProblem) append(expression ast.ASTHandle) error {
	if err := p.manager.SetRef(p.current, expression); err != nil {
		return err
	}
	ref, err := p.manager.NewRef(expression)
	if err != nil {
		return err
	}
	p.refs = append(p.refs, ref)
	p.formulas = append(p.formulas, expression)
	return nil
}

func (s *SMTInternalizerState) literalExpression(literal SMTLiteral) (ast.ASTHandle, error) {
	switch literal {
	case SMTTrueLiteral:
		return s.manager.MkTrue()
	case SMTFalseLiteral:
		return s.manager.MkFalse()
	}
	expression, err := s.BoolExpression(literal.Var())
	if err != nil || !literal.Sign() {
		return expression, err
	}
	return s.manager.MkNot(expression)
}

// DisplayLemmaAsSMTProblem prints premises followed by the negated conclusion.
// A false conclusion adds nothing; equality premises retain their input order.
// As in C++, the dump contains declarations/assertions and no check-sat command.
func (s *SMTInternalizerState) DisplayLemmaAsSMTProblem(out io.Writer, antecedents []SMTLiteral, equalities []SMTEnodePair, consequent SMTLiteral, logic usymbol.Symbol) (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if s.lemmaProblem != nil {
		return ast.ErrASTPending
	}
	p := &smtLemmaProblem{manager: s.manager}
	s.lemmaProblem = p
	defer func() {
		err := p.close()
		if err == nil {
			s.lemmaProblem = nil
		}
		result = errors.Join(result, err)
	}()
	var err error
	p.current, err = s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	for _, literal := range antecedents {
		expression, err := s.literalExpression(literal)
		if err != nil {
			return err
		}
		if err = p.append(expression); err != nil {
			return err
		}
	}
	for _, pair := range equalities {
		if pair.First == nil || pair.Second == nil || pair.First.manager != s.manager || pair.Second.manager != s.manager || s.context.enodes[pair.First.owner] != pair.First || s.context.enodes[pair.Second.owner] != pair.Second {
			return ast.ErrASTDomain
		}
		expression, err := s.manager.MkEq(pair.First.owner, pair.Second.owner)
		if err != nil {
			return err
		}
		if err = p.append(expression); err != nil {
			return err
		}
	}
	if consequent != SMTFalseLiteral {
		expression, err := s.literalExpression(consequent.Negated())
		if err != nil {
			return err
		}
		if err = p.append(expression); err != nil {
			return err
		}
	}
	if !logic.IsNull() {
		if err = smtWriteTheoryText(out, "(set-logic "); err != nil {
			return err
		}
		if err = s.manager.Symbols.WriteDisplay(out, logic); err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, ")\n"); err != nil {
			return err
		}
	}
	visitor, err := ast.NewASTPPUtil(s.manager)
	if err != nil {
		return err
	}
	if err = visitor.CollectAll(p.formulas); err != nil {
		return err
	}
	if err = visitor.DisplayDeclarations(out); err != nil {
		return err
	}
	return visitor.DisplayAssertions(out, p.formulas, true)
}

// Both native file overloads share this unsigned, single-goroutine counter.
var smtLemmaID uint32

type smtLemmaFileWriter struct {
	out io.Writer
	err error
}

func (w *smtLemmaFileWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return len(p), nil
	}
	n, err := w.out.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.err = err
	}
	return len(p), nil
}

// SMTLemmaDumpError separates formula/owner exceptions from ostream badbit.
// Source diagnostic consumers ignore StreamError and still evaluate validators.
type SMTLemmaDumpError struct{ OperandError, StreamError error }

func (e *SMTLemmaDumpError) Error() string {
	if err := errors.Join(e.OperandError, e.StreamError); err != nil {
		return err.Error()
	}
	return "SMT lemma dump failure"
}
func (e *SMTLemmaDumpError) Unwrap() []error { return []error{e.OperandError, e.StreamError} }

// DumpLemmaAsSMTProblem writes the source lemma_N.smt2 in the current working
// directory. Ostream failure still evaluates operands and advances the counter;
// an AST/owner error unwinds the lexical file without advancing it.
func (s *SMTInternalizerState) DumpLemmaAsSMTProblem(antecedents []SMTLiteral, equalities []SMTEnodePair, consequent SMTLiteral, logic usymbol.Symbol) (name string, result error) {
	if err := s.open(); err != nil {
		return "", err
	}
	name = fmt.Sprintf("lemma_%d.smt2", int32(smtLemmaID))
	file, err := os.Create(name)
	writer := &smtLemmaFileWriter{out: file, err: err}
	closed := false
	var operandErr, closeErr error
	defer func() {
		if file != nil && !closed {
			closeErr = file.Close()
		}
		streamErr := errors.Join(writer.err, closeErr)
		if operandErr != nil || streamErr != nil {
			result = &SMTLemmaDumpError{OperandError: operandErr, StreamError: streamErr}
		}
	}()
	operandErr = s.DisplayLemmaAsSMTProblem(writer, antecedents, equalities, consequent, logic)
	if operandErr != nil {
		return name, nil
	}
	if file != nil {
		closeErr = file.Close()
		closed = true
	}
	smtLemmaID++
	return name, nil
}
