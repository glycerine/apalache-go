package cmd_context

// Source: src/cmd_context/check_logic.cpp/.h. This validates the source logic
// profiles, including their intentional permissive cases; it is not an SMT-LIB
// standards checker.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// LogicChecker borrows its manager. The zero value and Reset accept all inputs,
// as does a source logic name without an explicit check_logic policy.
type LogicChecker struct {
	manager                                                                         *ast.ASTManager
	logic, lastError                                                                string
	uf, dt, arrays, bvArrays, reals, ints, diff, nonlinear, bvs, quantifiers, known bool
	arith, bv, array, seq, datatype, pb                                             ast.FamilyID
}

func (c *LogicChecker) Reset()            { *c = LogicChecker{} }
func (c *LogicChecker) LastError() string { return c.lastError }

// SetLogic borrows the initialized manager used by cmd_context, after its
// declaration plugins have been registered. It clears the previous diagnostic.
func (c *LogicChecker) SetLogic(m *ast.ASTManager, logic string) error {
	c.Reset()
	if m == nil {
		return ast.ErrASTHandle
	}
	c.manager, c.logic, c.known = m, logic, true
	// The context registers declaration plugins before constructing the source
	// utility borrowers. No declaration/plugin owner is retained here.
	for _, entry := range []struct {
		name   string
		target *ast.FamilyID
	}{
		{"arith", &c.arith}, {"bv", &c.bv}, {"array", &c.array},
		{"seq", &c.seq}, {"datatype", &c.datatype}, {"pb", &c.pb},
	} {
		name, err := m.Symbols.Intern(entry.name)
		if err != nil {
			c.Reset()
			return err
		}
		family, err := m.FamilyIDForSymbol(name)
		if err != nil {
			c.Reset()
			return err
		}
		*entry.target = family
	}
	switch logic {
	case "AUFLIA":
		c.uf, c.arrays, c.ints, c.quantifiers = true, true, true, true
	case "AUFLIRA":
		c.uf, c.arrays, c.reals, c.ints, c.quantifiers = true, true, true, true, true
	case "AUFNIRA":
		c.uf, c.arrays, c.reals, c.ints, c.nonlinear, c.quantifiers = true, true, true, true, true, true
	case "LRA":
		c.reals, c.quantifiers = true, true
	case "QF_ABV":
		c.bvArrays, c.bvs = true, true
	case "QF_AUFBV":
		c.uf, c.bvArrays, c.bvs = true, true, true
	case "QF_UFBV":
		c.uf, c.bvs = true, true
	case "QF_DT":
		c.uf, c.dt = true, true
	case "QF_AUFLIA":
		c.uf, c.arrays, c.ints = true, true, true
	case "QF_AX":
		c.arrays = true
	case "QF_BV":
		c.bvs = true
	case "QF_IDL":
		c.ints, c.diff = true, true
	case "QF_RDL":
		c.reals, c.diff = true, true
	case "QF_LIA":
		c.ints = true
	case "QF_LRA":
		c.reals = true
	case "QF_NIA":
		c.ints, c.nonlinear = true, true
	case "QF_NRA":
		c.reals, c.nonlinear = true, true
	case "QF_UF":
		c.uf = true
	case "QF_UFIDL":
		c.uf, c.ints, c.diff = true, true, true
	case "QF_UFLIA":
		c.uf, c.ints = true, true
	case "QF_UFLRA":
		c.uf, c.reals = true, true
	case "QF_UFNRA":
		c.uf, c.reals, c.nonlinear = true, true, true
	case "UFLRA":
		c.uf, c.reals, c.quantifiers = true, true, true
	case "UFNIA":
		c.uf, c.ints, c.quantifiers, c.nonlinear = true, true, true, true
	case "UFBV":
		c.uf, c.bvs, c.quantifiers = true, true, true
	case "QF_S":
		c.uf, c.bvs, c.ints, c.arrays, c.reals = true, true, true, true, true
	case "QF_FD":
		c.bvs, c.uf, c.ints, c.dt, c.nonlinear = true, true, true, true, true
	default:
		c.known = false
	}
	return nil
}

func (c *LogicChecker) fail(message string) error {
	c.lastError = message
	return errors.New(message)
}

func (c *LogicChecker) checkSort(sort ast.ASTHandle) error {
	family, err := c.manager.GetFamilyID(sort)
	if err != nil {
		return err
	}
	kind, err := c.manager.GetDeclKind(sort)
	if err != nil {
		return err
	}
	switch {
	case family == ast.NullFamilyID:
		if !c.uf {
			return c.fail("logic does not support uninterpreted sorts")
		}
	case family == c.manager.BasicFamilyID():
		return nil
	case family == c.arith && kind == 1:
		if !c.ints {
			return c.fail("logic does not support integers")
		}
	case family == c.arith && kind == 0:
		if !c.reals {
			return c.fail("logic does not support reals")
		}
	case family == c.bv:
		if !c.bvs {
			return c.fail("logic does not support bitvectors")
		}
	case family == c.array:
		if c.arrays {
			return nil
		}
		if !c.bvArrays {
			return c.fail("logic does not support arrays")
		}
		n, err := c.manager.ArrayArity(sort)
		if err != nil {
			return err
		}
		if n != 1 {
			return c.fail("logic supports only unidimensional arrays")
		}
		rangeSort, err := c.manager.ArrayRange(sort)
		if err != nil {
			return err
		}
		domain, err := c.manager.ArrayDomain(sort, 0)
		if err != nil {
			return err
		}
		rf, err := c.manager.GetFamilyID(rangeSort)
		if err != nil {
			return err
		}
		df, err := c.manager.GetFamilyID(domain)
		if err != nil {
			return err
		}
		if rf != c.bv || df != c.bv {
			return c.fail("logic supports only arrays from bitvectors to bitvectors")
		}
	}
	return nil
}

func (c *LogicChecker) CheckDeclaration(declaration ast.ASTHandle) error {
	if !c.known {
		return nil
	}
	n, err := c.manager.GetArity(declaration)
	if err != nil {
		return err
	}
	if n > 0 && !c.uf {
		return c.fail("logic does not support uninterpreted functions")
	}
	for i := uint32(0); i < n; i++ {
		sort, err := c.manager.GetDomain(declaration, i)
		if err != nil {
			return err
		}
		if err = c.checkSort(sort); err != nil {
			return err
		}
	}
	sort, err := c.manager.GetRange(declaration)
	if err != nil {
		return err
	}
	return c.checkSort(sort)
}

type logicCheckApp struct {
	term   ast.ASTHandle
	family ast.FamilyID
	op     int32
	args   []ast.ASTHandle
	app    bool
}
type logicCheckInvocation struct {
	checker *LogicChecker
	apps    map[ast.ASTHandle]logicCheckApp
}

func (v *logicCheckInvocation) app(h ast.ASTHandle) (logicCheckApp, error) {
	if a, ok := v.apps[h]; ok {
		return a, nil
	}
	a := logicCheckApp{term: h}
	m := v.checker.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return a, err
	}
	if kind != ast.ASTApp {
		return a, nil
	}
	a.app = true
	if a.family, err = m.GetFamilyID(h); err != nil {
		return a, err
	}
	if a.op, err = m.GetDeclKind(h); err != nil {
		return a, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return a, err
	}
	for i := uint32(0); i < n; i++ {
		arg, e := m.GetArg(h, i)
		if e != nil {
			return a, e
		}
		a.args = append(a.args, arg)
	}
	v.apps[h] = a
	return a, nil
}

func (v *logicCheckInvocation) arith(a logicCheckApp, op int32) bool {
	return a.app && a.family == v.checker.arith && a.op == op
}
func (v *logicCheckInvocation) isInt(h ast.ASTHandle) (bool, error) {
	a, err := v.app(h)
	if err != nil {
		return false, err
	}
	if v.arith(a, 8) {
		a, err = v.app(a.args[0])
		if err != nil {
			return false, err
		}
	}
	if v.arith(a, 14) {
		a, err = v.app(a.args[0])
		if err != nil {
			return false, err
		}
	}
	return v.arith(a, 0), nil
}
func (v *logicCheckInvocation) numeral(h ast.ASTHandle) (bool, error) {
	a, err := v.app(h)
	if err != nil {
		return false, err
	}
	if v.arith(a, 8) {
		a, err = v.app(a.args[0])
		if err != nil {
			return false, err
		}
	}
	if yes, e := v.isInt(a.term); e != nil || yes {
		return yes, e
	}
	if !v.arith(a, 10) {
		return false, nil
	}
	x, err := v.isInt(a.args[0])
	if err != nil || !x {
		return false, err
	}
	return v.isInt(a.args[1])
}
func (v *logicCheckInvocation) diffVar(a logicCheckApp) bool {
	return a.app && (a.family == ast.NullFamilyID || a.family == v.checker.manager.BasicFamilyID() && a.op == ast.ASTOpIte)
}
func (v *logicCheckInvocation) diffArg(h ast.ASTHandle) (bool, error) {
	a, err := v.app(h)
	if err != nil {
		return false, err
	}
	if v.diffVar(a) {
		return true, nil
	}
	if yes, e := v.numeral(h); e != nil || yes {
		return yes, e
	}
	if !v.arith(a, 6) && !v.arith(a, 7) {
		return false, nil
	}
	for {
		var nonNumeral ast.ASTHandle
		for _, arg := range a.args {
			yes, e := v.numeral(arg)
			if e != nil {
				return false, e
			}
			if yes {
				continue
			}
			if nonNumeral != (ast.ASTHandle{}) {
				return false, nil
			}
			nonNumeral = arg
		}
		if nonNumeral == (ast.ASTHandle{}) {
			return true, nil
		}
		a, err = v.app(nonNumeral)
		if err != nil {
			return false, err
		}
		if v.diffVar(a) {
			return true, nil
		}
		if !v.arith(a, 6) && !v.arith(a, 7) {
			return false, nil
		}
	}
}
func (v *logicCheckInvocation) isArith(h ast.ASTHandle) (bool, error) {
	sort, err := v.checker.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	family, err := v.checker.manager.GetFamilyID(sort)
	return family == v.checker.arith, err
}
func (v *logicCheckInvocation) nonDiff(h ast.ASTHandle) error {
	// Preserve the live source TRACE before publishing the policy error.
	return v.nonDiffTrace(h, diagnostic.BorrowProcessTraceTags())
}

func (v *logicCheckInvocation) nonDiffTrace(h ast.ASTHandle, tags *diagnostic.TraceTags) error {
	if tags.ShouldTrace("check_logic") {
		err := tags.EmitConditional("check_logic", "fail_non_diff", "src/cmd_context/check_logic.cpp", 309, func() bool { return true }, func(w io.Writer) (result error) {
			m := v.checker.manager
			format, err := m.SMT2PPExprTree(h, nil)
			if err != nil {
				return err
			}
			fm, err := m.GetFormatManager()
			if err != nil {
				return err
			}
			formatRef, err := fm.NewRef(format)
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, fm.DelRef(&formatRef)) }()
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, stream.Close()) }()
			if err = m.PrintFormat(stream, format, nil, nil, nil); err != nil {
				return err
			}
			text, err := stream.String()
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, text+"\n")
			return err
		})
		if err != nil && !tags.IsStreamFailure(err) {
			return err
		}
	}
	return v.checker.fail("logic only supports difference arithmetic")
}
func logicSameArgs(a logicCheckApp) bool {
	if len(a.args) == 0 {
		return false
	}
	for _, h := range a.args[1:] {
		if h != a.args[0] {
			return false
		}
	}
	return true
}
func (v *logicCheckInvocation) diffPredicate(a logicCheckApp) error {
	lhs, rhs := a.args[0], a.args[1]
	if yes, err := v.isArith(lhs); err != nil || !yes {
		return err
	}
	left, err := v.diffArg(lhs)
	if err != nil {
		return err
	}
	if left {
		if right, e := v.diffArg(rhs); e != nil || right {
			return e
		}
	}
	if yes, e := v.numeral(lhs); e != nil {
		return e
	} else if yes {
		lhs, rhs = rhs, lhs
	}
	if yes, e := v.numeral(rhs); e != nil {
		return e
	} else if !yes {
		return v.nonDiff(a.term)
	}
	x, err := v.app(lhs)
	if err != nil {
		return err
	}
	if !v.arith(x, 7) || len(x.args) != 2 {
		return v.nonDiff(a.term)
	}
	t1, err := v.app(x.args[0])
	if err != nil {
		return err
	}
	t2, err := v.app(x.args[1])
	if err != nil {
		return err
	}
	if v.diffVar(t1) && v.diffVar(t2) {
		return nil
	}
	if v.arith(t1, 6) && v.arith(t2, 6) && len(t1.args) == len(t2.args) && logicSameArgs(t1) && logicSameArgs(t2) {
		return nil
	}
	return v.nonDiff(a.term)
}
func (v *logicCheckInvocation) diffArgs(a logicCheckApp) error {
	// The source checks n itself (not each argument), once per argument.
	// In particular Boolean distinct and UF predicates accept arithmetic args.
	if len(a.args) == 0 {
		return nil
	}
	if yes, err := v.isArith(a.term); err != nil || !yes {
		return err
	}
	if yes, err := v.diffArg(a.term); err != nil || yes {
		return err
	}
	return v.nonDiff(a.term)
}
func (v *logicCheckInvocation) checkApp(a logicCheckApp) error {
	c, m := v.checker, v.checker.manager
	sort, err := m.GetSort(a.term)
	if err != nil {
		return err
	}
	if err = c.checkSort(sort); err != nil {
		return err
	}
	switch {
	case a.family == ast.NullFamilyID:
		if !c.uf && len(a.args) > 0 {
			return c.fail("logic does not support uninterpreted functions")
		}
		if c.diff {
			return v.diffArgs(a)
		}
	case a.family == c.arith:
		if !c.nonlinear && a.op == 9 {
			found := false
			for _, h := range a.args {
				yes, e := v.numeral(h)
				if e != nil {
					return e
				}
				if !yes {
					if found {
						return c.fail("logic does not support nonlinear arithmetic")
					}
					found = true
				}
			}
		} else if !c.nonlinear && a.op >= 10 && a.op <= 13 {
			if len(a.args) != 2 {
				return ast.ErrASTDomain
			}
			if yes, e := v.numeral(a.args[1]); e != nil {
				return e
			} else if !yes {
				return c.fail("logic does not support nonlinear arithmetic")
			}
		}
		if c.diff && a.op >= 2 && a.op <= 5 {
			if err = v.diffPredicate(a); err != nil {
				return err
			}
		}
		if (!c.ints || !c.reals) && (a.op == 14 || a.op == 15) {
			return c.fail("logic does not support casting operators")
		}
	case a.family == c.bv:
	case a.family == c.array:
		if c.diff {
			return v.diffArgs(a)
		}
	case a.family == m.BasicFamilyID():
		if c.diff {
			if a.op == ast.ASTOpEq {
				return v.diffPredicate(a)
			}
			if a.op == ast.ASTOpDistinct || a.op == ast.ASTOpIte {
				return v.diffArgs(a)
			}
		}
	case a.family >= 0 && a.family <= 4:
	case a.family == c.seq:
	case a.family == c.datatype && c.dt:
	case a.family == c.pb && c.logic == "QF_FD":
	default:
		name, err := m.FamilyName(a.family)
		if err != nil {
			return err
		}
		text, err := m.Symbols.String(name)
		if err != nil {
			return err
		}
		return c.fail("logic does not support theory " + text)
	}
	return nil
}

// CheckExpression visits children in source postorder: application arguments,
// then quantifier body, patterns and no-patterns. Scratch maps/slices borrow
// handles only for this call; the checker retains no expressions or sorts.
func (c *LogicChecker) CheckExpression(root ast.ASTHandle) error {
	if !c.known {
		return nil
	}
	v := logicCheckInvocation{checker: c, apps: make(map[ast.ASTHandle]logicCheckApp)}
	type frame struct {
		term ast.ASTHandle
		exit bool
	}
	stack := []frame{{term: root}}
	visited := make(map[ast.ASTHandle]bool)
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !f.exit {
			if visited[f.term] {
				continue
			}
			visited[f.term] = true
		}
		kind, err := c.manager.GetKind(f.term)
		if err != nil {
			return err
		}
		if f.exit || kind == ast.ASTVar {
			switch kind {
			case ast.ASTVar:
				if !c.quantifiers {
					return c.fail("logic does not support quantifiers")
				}
				sort, e := c.manager.GetSort(f.term)
				if e != nil {
					return e
				}
				if e = c.checkSort(sort); e != nil {
					return e
				}
			case ast.ASTQuantifier:
				if !c.quantifiers {
					return c.fail("logic does not support quantifiers")
				}
			case ast.ASTApp:
				a, e := v.app(f.term)
				if e != nil {
					return e
				}
				if e = v.checkApp(a); e != nil {
					return e
				}
			default:
				return ast.ErrASTDomain
			}
			continue
		}
		stack = append(stack, frame{term: f.term, exit: true})
		switch kind {
		case ast.ASTApp:
			a, e := v.app(f.term)
			if e != nil {
				return e
			}
			for i := len(a.args) - 1; i >= 0; i-- {
				stack = append(stack, frame{term: a.args[i]})
			}
		case ast.ASTQuantifier:
			n, e := c.manager.GetNumNoPatterns(f.term)
			if e != nil {
				return e
			}
			for i := n; i > 0; i-- {
				h, e := c.manager.GetNoPattern(f.term, i-1)
				if e != nil {
					return e
				}
				stack = append(stack, frame{term: h})
			}
			n, e = c.manager.GetNumPatterns(f.term)
			if e != nil {
				return e
			}
			for i := n; i > 0; i-- {
				h, e := c.manager.GetPattern(f.term, i-1)
				if e != nil {
					return e
				}
				stack = append(stack, frame{term: h})
			}
			body, e := c.manager.GetQuantifierBody(f.term)
			if e != nil {
				return e
			}
			stack = append(stack, frame{term: body})
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}
