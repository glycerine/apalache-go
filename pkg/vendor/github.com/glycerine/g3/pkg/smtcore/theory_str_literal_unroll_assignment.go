package smtcore

// Source: src/smt/theory_str.cpp computeGCD, computeLCM,
// gen_assign_unroll_Str2Reg and get_eqc_simpleUnroll.

import (
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func stringUnrollLCM(a, b int32) (int32, error) {
	if a < 0 || b < 0 {
		return 0, ast.ErrASTDomain
	}
	x, y := a, b
	if x == 0 {
		x = y
	} else {
		for y != 0 {
			if x > y {
				x -= y
			} else {
				y -= x
			}
		}
	}
	if x == 0 {
		return 0, nil
	}
	product := int64(a/x) * int64(b)
	if product > 1<<31-1 {
		return 0, ast.ErrASTDomain
	} // Native signed multiplication overflow.
	return int32(product), nil
}

func (t *SMTStringTheory) stringClassSimpleUnrolls(word ast.ASTHandle) (constant ast.ASTHandle, unrolls []ast.ASTHandle, result error) {
	constant, all, err := t.stringClassUnrolls(word)
	if err != nil {
		return constant, nil, err
	}
	for _, unroll := range all {
		core, err := t.state.manager.GetArg(unroll, 0)
		if err != nil {
			return constant, nil, err
		}
		op, own, err := t.operation(core)
		if err != nil {
			return constant, nil, err
		}
		if own && op == 11 {
			unrolls = append(unrolls, unroll)
		}
	}
	return constant, unrolls, nil
}

func (t *SMTStringTheory) generateStringLiteralUnrollAssignment(word ast.ASTHandle, unrolls []ast.ASTHandle) (output ast.ASTRef, result error) {
	ordered := append([]ast.ASTHandle(nil), unrolls...)
	stringSortHandles(ordered)
	unique := ordered[:0]
	for _, term := range ordered {
		if len(unique) == 0 || term != unique[len(unique)-1] {
			unique = append(unique, term)
		}
	}
	ordered = unique
	if len(ordered) == 0 {
		return output, ast.ErrASTDomain
	}
	m := t.state.manager
	var cores []string
	lcm := int32(1)
	for _, unroll := range ordered {
		op, own, err := t.operation(unroll)
		if err != nil {
			return output, err
		}
		if !own || op != 43 {
			return output, ast.ErrASTDomain
		}
		regex, err := m.GetArg(unroll, 0)
		if err != nil {
			return output, err
		}
		op, own, err = t.operation(regex)
		if err != nil {
			return output, err
		}
		if !own || op != 11 {
			return output, ast.ErrASTDomain
		}
		core, err := m.GetArg(regex, 0)
		if err != nil {
			return output, err
		}
		text, literal, err := m.SequenceStringValue(core)
		if err != nil {
			return output, err
		}
		// Native nonliteral/empty cores leave an empty zstring and later
		// divide by zero. Do not fabricate a symbolic or empty-core result.
		if !literal || len(text) == 0 || uint64(len(text)) > 1<<31-1 {
			return output, ast.ErrASTDomain
		}
		lcm, err = stringUnrollLCM(lcm, int32(len(text)))
		if err != nil {
			return output, err
		}
		cores = append(cores, text)
	}
	common := strings.Repeat(cores[0], int(lcm)/len(cores[0]))
	compatible := true
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	var premises []ast.ASTHandle
	for i, unroll := range ordered {
		if strings.Repeat(cores[i], int(lcm)/len(cores[i])) != common {
			compatible = false
		}
		equal, err := pin(t.state.mkEqualityAtom(word, unroll))
		if err != nil {
			return output, err
		}
		premises = append(premises, equal)
	}
	if compatible {
		return t.generateStringConditionalUnrollOptions(word, ordered, common)
	}
	guard, err := pin(m.MkAndArray(premises))
	if err != nil {
		return output, err
	}
	empty, err := pin(t.stringLiteral(""))
	if err != nil {
		return output, err
	}
	conclusion, err := pin(t.state.mkEqualityAtom(word, empty))
	if err != nil {
		return output, err
	}
	negative, err := pin(m.MkNotSimplified(guard)) // Native rewrite_implication uses mk_not(m,...).
	if err != nil {
		return output, err
	}
	formula, err := pin(m.MkOr(negative, conclusion))
	if err != nil {
		return output, err
	}
	return m.NewRef(formula)
}
