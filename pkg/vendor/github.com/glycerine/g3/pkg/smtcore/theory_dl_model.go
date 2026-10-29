package smtcore

// Source: src/smt/theory_dl.cpp:70-103,165-167;
// src/smt/theory_bv.cpp:508-514.

import (
	"errors"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The expression overload is a read-side native query. It must neither
// internalize an expression nor reconstruct its cached AST bit circuit.
func (t *SMTBitVectorTheory) fixedExpressionValue(expression ast.ASTHandle) (*big.Int, bool, error) {
	s, err := t.State()
	if err != nil {
		return nil, false, err
	}
	node := s.context.enodes[expression]
	if node == nil {
		return nil, false, nil
	}
	return t.fixedLiteralValue(node.TheoryVar(t.FamilyID()))
}

// Borrowed theory/enode lifetime matches dl_value_proc. No numeral,
// representative, or dependency is captured at procedure construction.
type smtDatalogValueProcedure struct {
	theory *SMTDatalogTheory
	node   *SMTEnode
}

func (*smtDatalogValueProcedure) Dependencies() []SMTModelValueDependency { return nil }
func (*smtDatalogValueProcedure) IsFresh() bool                           { return false }

func (p *smtDatalogValueProcedure) MakeValue(generator *SMTModelGenerator, _ []ast.ASTHandle) (result ast.ASTHandle, err error) {
	if p == nil || p.theory == nil || p.node == nil || generator == nil {
		return result, ast.ErrASTDomain
	}
	t := p.theory
	s, err := t.State()
	if err != nil {
		return result, err
	}
	if generator.manager != s.manager || !t.IsAttachedToVariable(p.node) {
		return result, ast.ErrASTDomain
	}
	sort, err := s.manager.GetSort(p.node.owner)
	if err != nil {
		return result, err
	}
	declarations, err := t.declarationsFor(sort)
	if err != nil {
		return result, err
	}
	representative, err := s.manager.MkApp(declarations.rep, []ast.ASTHandle{p.node.owner})
	if err != nil {
		return result, err
	}
	repRef, err := s.manager.NewRef(representative)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, s.manager.DelRef(&repRef)) }()
	family, err := s.manager.BitVectorUtilityFamilyID()
	if err != nil {
		return result, err
	}
	bv, ok := s.theories[family].(*SMTBitVectorTheory)
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return result, e
		}
		if enabled && (!ok || bv == nil) {
			if e = diagnostic.DebugAssertionFailure(d, "src/smt/theory_dl.cpp", 90, "th_bv"); e != nil {
				return result, e
			}
		}
	}
	var number uint64
	if s.context.enodes[representative] != nil && ok && bv != nil {
		value, fixed, e := bv.fixedExpressionValue(representative)
		if e != nil {
			return result, e
		}
		if fixed {
			// Source rational::get_int64 invokes the retained MPQ/MPZ assertion
			// and conversion policy, then converts to dl_decl_util's uint64 input.
			numerals := s.manager.NumeralService()
			numeral, e := numerals.NewValue()
			if e != nil {
				return result, e
			}
			defer func() { err = errors.Join(err, numerals.Del(&numeral)) }()
			if e = numerals.SetString(&numeral, value.String()); e != nil {
				return result, e
			}
			signed, e := numerals.Int64(&numeral)
			if e != nil {
				return result, e
			}
			number = uint64(signed)
		}
	}
	result, err = s.manager.MkDatalogNumeral(number, sort)
	if err != nil {
		return result, err
	}
	if err = t.traceDatalogModelValue(result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (t *SMTDatalogTheory) traceDatalogModelValue(value ast.ASTHandle) error {
	return t.traceDatalogExpression("mk_value", 99, value)
}

func (t *SMTDatalogTheory) traceDatalogExpression(function string, line int, value ast.ASTHandle) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	var operandErr error
	err := tags.EmitConditional("theory_dl", function, "src/smt/theory_dl.cpp", line, func() bool { return true }, func(out io.Writer) error {
		out = assertedTraceWriter{out}
		printer, e := ast.NewSimpleASTPrinterContext(t.state.manager, out, out)
		if e != nil {
			operandErr = e
			return e
		}
		operandErr = printer.DisplayExpr(out, value, 0)
		if operandErr != nil {
			return operandErr
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
