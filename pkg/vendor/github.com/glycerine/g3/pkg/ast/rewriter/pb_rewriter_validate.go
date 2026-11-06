package rewriter

// Source: src/ast/rewriter/pb_rewriter.cpp:77-196,280-288.

import (
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/ast"
)

const (
	pbArithLE  int32 = 2
	pbArithGE  int32 = 3
	pbArithAdd int32 = 6
	pbArithSub int32 = 7
	pbArithMul int32 = 9
)

var pbRewriteLemma atomic.Uint64

func (r *PseudoBooleanRewriter) arithmeticNumeral(q *big.Rat, integer bool) (result ast.ASTHandle, err error) {
	v, err := r.ownedRational(q)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, r.manager.NumeralService().Del(&v)) }()
	return r.manager.ArithmeticNumeral(&v, integer)
}

func (r *PseudoBooleanRewriter) arithmeticApp(kind int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	family, err := r.manager.RegisterArithmeticPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.manager.MkFamilyApp(family, kind, nil, args)
}

// TranslatePBToLIA implements the diagnostic translator used by the source
// pb_validate trace. Variables maps positive Boolean literals to 0/1 integer
// expressions.
func (r *PseudoBooleanRewriter) TranslatePBToLIA(variables map[ast.ASTHandle]ast.ASTHandle, formula ast.ASTHandle) (ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := r.manager.GetFamilyID(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := r.manager.GetDeclKind(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family != r.family || kind < ast.ASTPBOpAtMost || kind > ast.ASTPBOpEQ {
		return formula, nil
	}
	n, err := r.manager.GetNumArgs(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	terms := make([]ast.ASTHandle, n)
	var one ast.ASTHandle
	for i := uint32(0); i < n; i++ {
		literal, e := r.manager.GetArg(formula, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		positive, negated, e := r.negated(literal)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !negated {
			positive = literal
		}
		variable, found := variables[positive]
		if !found {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if negated {
			if one.IsNull() {
				one, e = r.arithmeticNumeral(big.NewRat(1, 1), true)
				if e != nil {
					return ast.ASTHandle{}, e
				}
			}
			variable, e = r.arithmeticApp(pbArithSub, one, variable)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		terms[i] = variable
	}
	cardinality := kind == ast.ASTPBOpAtMost || kind == ast.ASTPBOpAtLeast
	if !cardinality {
		for i := range terms {
			coefficient, e := r.coefficient(formula, uint32(i))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			value, e := r.arithmeticNumeral(coefficient, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			terms[i], e = r.arithmeticApp(pbArithMul, value, terms[i])
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	}
	var sum ast.ASTHandle
	if len(terms) == 0 {
		sum, err = r.arithmeticNumeral(new(big.Rat), true)
	} else {
		sum, err = r.arithmeticApp(pbArithAdd, terms...)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bound, err := r.bound(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	k, err := r.arithmeticNumeral(bound, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch kind {
	case ast.ASTPBOpAtMost, ast.ASTPBOpLE:
		return r.arithmeticApp(pbArithLE, sum, k)
	case ast.ASTPBOpAtLeast, ast.ASTPBOpGE:
		return r.arithmeticApp(pbArithGE, sum, k)
	case ast.ASTPBOpEQ:
		return r.manager.MkEq(sum, k)
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
}

// MakeValidationFormula returns the integer counterexample query emitted by
// the source validator: every Boolean atom is constrained to 0 or 1 and the
// original and rewritten formulas are required to differ.
func (r *PseudoBooleanRewriter) MakeValidationFormula(original, rewritten ast.ASTHandle) (ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := r.manager.RegisterArithmeticPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	integer, err := r.manager.MkFamilySort(family, 1, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := r.arithmeticNumeral(new(big.Rat), true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := r.arithmeticNumeral(big.NewRat(1, 1), true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	variables := make(map[ast.ASTHandle]ast.ASTHandle)
	constraints := make([]ast.ASTHandle, 0)
	n, err := r.manager.GetNumArgs(original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := uint32(0); i < n; i++ {
		literal, e := r.manager.GetArg(original, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		positive, negated, e := r.negated(literal)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !negated {
			positive = literal
		}
		if r.manager.IsTrue(positive) {
			variables[positive] = one
			continue
		}
		if r.manager.IsFalse(positive) {
			variables[positive] = zero
			continue
		}
		name, e := r.manager.Symbols.Intern("x" + strconv.FormatUint(uint64(i), 10))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		x, e := r.manager.MkConst(name, integer)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if _, exists := variables[positive]; !exists {
			variables[positive] = x
		}
		lower, e := r.arithmeticApp(pbArithLE, zero, x)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		upper, e := r.arithmeticApp(pbArithLE, x, one)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		constraints = append(constraints, lower, upper)
	}
	left, err := r.TranslatePBToLIA(variables, original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := r.TranslatePBToLIA(variables, rewritten)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equal, err := r.manager.MkEq(left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	different, err := r.manager.MkNot(equal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	constraints = append(constraints, different)
	return r.manager.MkAndArgs(constraints)
}

func (r *PseudoBooleanRewriter) dumpPBRewriteAt(directory string, formula ast.ASTHandle) (name string, result error) {
	index := pbRewriteLemma.Add(1) - 1
	name = filepath.Join(directory, "pb_rewrite_"+strconv.FormatUint(index, 10)+".smt2")
	out, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return name, err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	printer, err := ast.NewLegacySMTPrinter(r.manager)
	if err != nil {
		return name, err
	}
	return name, printer.DisplayBenchmark(out, formula, ast.LegacySMTBenchmarkConfig{})
}

// DumpPBRewrite emits the next source-named validation query in the current
// directory.
func (r *PseudoBooleanRewriter) DumpPBRewrite(formula ast.ASTHandle) (string, error) {
	if err := r.open(); err != nil {
		return "", err
	}
	return r.dumpPBRewriteAt(".", formula)
}

func pbArgumentSlice(arguments ast.AppArguments) ([]ast.ASTHandle, error) {
	values := make([]ast.ASTHandle, arguments.Count())
	for i := range values {
		value, err := arguments.At(uint32(i))
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

func (r *PseudoBooleanRewriter) validateRewrite(decl ast.ASTHandle, arguments ast.AppArguments, rewritten ast.ASTHandle) error {
	args, err := pbArgumentSlice(arguments)
	if err != nil {
		return err
	}
	original, err := r.manager.MkApp(decl, args)
	if err != nil {
		return err
	}
	formula, err := r.MakeValidationFormula(original, rewritten)
	if err != nil {
		return err
	}
	_, err = r.DumpPBRewrite(formula)
	return err
}

func (r *PseudoBooleanRewriter) traceRewrite(decl ast.ASTHandle, arguments ast.AppArguments, rewritten ast.ASTHandle) error {
	tags := r.traceTags()
	if tags.ShouldTrace("pb") {
		args, err := pbArgumentSlice(arguments)
		if err != nil {
			return err
		}
		original, err := r.manager.MkApp(decl, args)
		if err != nil {
			return err
		}
		printer, err := ast.NewLegacySMTPrinter(r.manager)
		if err != nil {
			return err
		}
		if err = tags.EmitConditional("pb", "mk_app_core", "src/ast/rewriter/pb_rewriter.cpp", 280, func() bool { return true }, func(w io.Writer) error {
			if e := printer.DisplayExpr(w, original, nil); e != nil {
				return e
			}
			if _, e := io.WriteString(w, "\n"); e != nil {
				return e
			}
			if e := printer.DisplayExpr(w, rewritten, nil); e != nil {
				return e
			}
			_, e := io.WriteString(w, "\n")
			return e
		}); err != nil {
			return err
		}
	}
	if tags.ShouldTrace("pb_validate") {
		return r.validateRewrite(decl, arguments, rewritten)
	}
	return nil
}
