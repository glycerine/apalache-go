package rewriter

// Source: src/ast/rewriter/inj_axiom.cpp; src/ast/rewriter/inj_axiom.h.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func injFormat(m *ast.ASTManager, h ast.ASTHandle, lowLevel bool) (value string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if lowLevel {
		if err = m.DisplayLowLevel(out, h, true, true); err != nil {
			return "", err
		}
	} else {
		format, e := m.SMT2PPExprTree(h, nil)
		if e != nil {
			return "", e
		}
		if err = m.PrintFormat(out, format, nil, nil, nil); err != nil {
			return "", err
		}
	}
	return out.String()
}

func injTraceCandidate(m *ast.ASTManager, q, app1, app2, var1, var2 ast.ASTHandle, numVars uint32) error {
	tags := diagnostic.BorrowProcessTraceTags()
	return tags.EmitConditional("inj_axiom", "simplify_inj_axiom", "src/ast/rewriter/inj_axiom.cpp", 77, func() bool { return true }, func(w io.Writer) error {
		values := make([]string, 5)
		var err error
		values[0], err = injFormat(m, q, true)
		for i, h := range []ast.ASTHandle{app1, app2, var1, var2} {
			if err == nil {
				values[i+1], err = injFormat(m, h, false)
			}
		}
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "Cadidate for simplification:\n"+values[0]+values[1]+"\n"+values[2]+"\n"+values[3]+"\n"+values[4]+"\nnum_vars: "+strconv.FormatUint(uint64(numVars), 10)+"\n")
		return err
	})
}

func injTraceResult(m *ast.ASTManager, h ast.ASTHandle) error {
	tags := diagnostic.BorrowProcessTraceTags()
	return tags.EmitConditional("inj_axiom", "simplify_inj_axiom", "src/ast/rewriter/inj_axiom.cpp", 130, func() bool { return true }, func(w io.Writer) error {
		value, err := injFormat(m, h, false)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "new axiom:\n"+value+"\n")
		return err
	})
}

func injBasicBinary(m *ast.ASTManager, h ast.ASTHandle, op int32) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != m.BasicFamilyID() {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	declKind, err := m.GetDeclKind(h)
	if err != nil || declKind != op {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil || n != 2 {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	a, err := m.GetArg(h, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	b, err := m.GetArg(h, 1)
	return a, b, err == nil, err
}

func injVariable(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(h)
	return kind == ast.ASTVar, err
}

func injUninterpretedConstant(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	n, err := m.GetNumArgs(h)
	return n == 0, err
}

// SimplifyInjectivityAxiom recognizes the retained injectivity-axiom shape and
// publishes its inverse-function formulation in out.
func SimplifyInjectivityAxiom(m *ast.ASTManager, q ast.ASTHandle, out ast.ASTRef) (changed bool, result error) {
	if m == nil {
		return false, ast.ErrASTHandle
	}
	forall, err := m.IsForall(q)
	if err != nil || !forall {
		return false, err
	}
	body, err := m.GetQuantifierBody(q)
	if err != nil {
		return false, err
	}
	arg1, arg2, ok, err := injBasicBinary(m, body, ast.ASTOpOr)
	if err != nil || !ok {
		return false, err
	}
	if negated, _, err := m.IsNot(arg2); err != nil {
		return false, err
	} else if negated {
		arg1, arg2 = arg2, arg1
	}
	negated, narg, err := m.IsNot(arg1)
	if err != nil || !negated {
		return false, err
	}
	app1, app2, ok, err := injBasicBinary(m, narg, ast.ASTOpEq)
	if err != nil || !ok {
		return false, err
	}
	var1, var2, ok, err := injBasicBinary(m, arg2, ast.ASTOpEq)
	if err != nil || !ok {
		return false, err
	}
	k1, err := m.GetKind(app1)
	if err != nil || k1 != ast.ASTApp {
		return false, err
	}
	k2, err := m.GetKind(app2)
	if err != nil || k2 != ast.ASTApp {
		return false, err
	}
	d1, err := m.GetDecl(app1)
	if err != nil {
		return false, err
	}
	d2, err := m.GetDecl(app2)
	if err != nil || d1 != d2 {
		return false, err
	}
	n, err := m.GetNumArgs(app1)
	if err != nil || n == 0 {
		return false, err
	}
	n2, err := m.GetNumArgs(app2)
	if err != nil || n2 != n {
		return false, err
	}
	family, err := m.GetFamilyID(app1)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	v1, err := injVariable(m, var1)
	if err != nil || !v1 {
		return false, err
	}
	v2, err := injVariable(m, var2)
	if err != nil || !v2 || var1 == var2 {
		return false, err
	}
	found := false
	idx := uint32(0)
	numVars := uint32(1)
	for i := uint32(0); i < n; i++ {
		c1, err := m.GetArg(app1, i)
		if err != nil {
			return false, err
		}
		c2, err := m.GetArg(app2, i)
		if err != nil {
			return false, err
		}
		isVar, err := injVariable(m, c1)
		if err != nil {
			return false, err
		}
		if !isVar {
			constant, err := injUninterpretedConstant(m, c1)
			if err != nil || !constant {
				return false, err
			}
		}
		if c1 == var1 && c2 == var2 || c1 == var2 && c2 == var1 {
			if found {
				return false, nil
			}
			found, idx = true, i
		} else {
			if c1 != c2 || c1 == var1 || c1 == var2 {
				return false, nil
			}
			if isVar {
				numVars++
			}
		}
	}
	free, err := m.HasFreeVars(q)
	if err != nil || !found || free {
		return false, err
	}
	if err = injTraceCandidate(m, q, app1, app2, var1, var2, numVars); err != nil {
		return false, err
	}

	var owned []ast.ASTRef
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&owned[i]))
		}
	}()
	hold := func(h ast.ASTHandle, e error) (ast.ASTHandle, error) {
		if e != nil {
			return ast.ASTHandle{}, e
		}
		ref, e := m.NewRef(h)
		if e == nil {
			owned = append(owned, ref)
		}
		return h, e
	}
	args := make([]ast.ASTHandle, 0, n)
	inverseArgs := make([]ast.ASTHandle, 0, n)
	sorts := make([]ast.ASTHandle, 0, n)
	names := make([]usymbol.Symbol, 0, n)
	var distinguished ast.ASTHandle
	varIndex := uint32(0)
	for i := uint32(0); i < n; i++ {
		c, err := m.GetArg(app1, i)
		if err != nil {
			return false, err
		}
		isVar, err := injVariable(m, c)
		if err != nil {
			return false, err
		}
		if !isVar {
			args = append(args, c)
			continue
		}
		sort, err := m.GetDomain(d1, i)
		if err != nil {
			return false, err
		}
		freshVar, err := hold(m.MkVar(varIndex, sort))
		if err != nil {
			return false, err
		}
		varIndex++
		names = append(names, usymbol.Numerical(i))
		sorts = append(sorts, sort)
		args = append(args, freshVar)
		if i == idx {
			distinguished = freshVar
		} else {
			inverseArgs = append(inverseArgs, freshVar)
		}
	}
	if distinguished.IsNull() {
		return false, ast.ErrASTDomain
	}
	f, err := hold(m.MkApp(d1, args))
	if err != nil {
		return false, err
	}
	inverseArgs = append(inverseArgs, f)
	domain := make([]ast.ASTHandle, len(inverseArgs))
	for i, h := range inverseArgs {
		domain[i], err = m.GetSort(h)
		if err != nil {
			return false, err
		}
	}
	rangeSort, err := m.GetDomain(d1, idx)
	if err != nil {
		return false, err
	}
	prefix, err := m.Symbols.Intern("inj")
	if err != nil {
		return false, err
	}
	inverse, err := hold(m.MkFreshFuncDecl(prefix, usymbol.Symbol{}, domain, rangeSort))
	if err != nil {
		return false, err
	}
	projection, err := hold(m.MkApp(inverse, inverseArgs))
	if err != nil {
		return false, err
	}
	equality, err := hold(m.MkEq(projection, distinguished))
	if err != nil {
		return false, err
	}
	pattern, err := hold(m.MkPattern([]ast.ASTHandle{f}))
	if err != nil {
		return false, err
	}
	for i, j := 0, len(sorts)-1; i < j; i, j = i+1, j-1 {
		sorts[i], sorts[j] = sorts[j], sorts[i]
	}
	optimized, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: equality, Patterns: []ast.ASTHandle{pattern}})
	if err != nil {
		return false, err
	}
	if err = injTraceResult(m, optimized); err != nil {
		return false, err
	}
	if ast.ASTSourceDebug {
		good, err := m.IsWellSorted(optimized, nil)
		if err != nil {
			return false, err
		}
		if !good {
			return false, ast.ErrASTDomain
		}
	}
	return true, m.SetRef(out, optimized)
}
