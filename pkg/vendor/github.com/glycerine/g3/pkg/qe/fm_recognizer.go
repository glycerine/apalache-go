package qe

// Source: src/qe/qe_lite.cpp fm::fm OCC recognition and forbidden_proc.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// fmRecognizer is the concrete recognition state of the retained FM phase.
// The owner supplies its borrowed selector and source-default options. The
// forbidden set is populated in input order, before constraints are registered.
type fmRecognizer struct {
	manager    *ast.ASTManager
	selection  *VariableTest
	config     Config
	arithmetic ast.FamilyID
	forbidden  map[uint32]bool
	realOnly   bool
	occ        bool
}

func (r *fmRecognizer) operation(h ast.ASTHandle, family ast.FamilyID, operation int32) (bool, error) {
	k, err := r.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := r.manager.GetFamilyID(h)
	if err != nil || f != family {
		return false, err
	}
	op, err := r.manager.GetDeclKind(h)
	return op == operation, err
}

func (r *fmRecognizer) uninterpretedConstant(h ast.ASTHandle) (bool, error) {
	k, err := r.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := r.manager.GetFamilyID(h)
	if err != nil || f != ast.NullFamilyID {
		return false, err
	}
	n, err := r.manager.GetNumArgs(h)
	return n == 0, err
}

func (r *fmRecognizer) literal(h ast.ASTHandle) (bool, error) {
	constant, err := r.uninterpretedConstant(h)
	if err != nil || constant {
		return constant, err
	}
	negative, err := r.operation(h, r.manager.BasicFamilyID(), ast.ASTOpNot)
	if err != nil || !negative {
		return false, err
	}
	h, err = r.manager.GetArg(h, 0)
	if err != nil {
		return false, err
	}
	return r.uninterpretedConstant(h)
}

func (r *fmRecognizer) variable(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	selected, err := r.selection.IsVariable(h)
	if err != nil || selected {
		return h, selected, err
	}
	// arith_util::is_to_real; one concrete numeral type, no template adapter.
	coercion, err := r.operation(h, r.arithmetic, 14)
	if err != nil || !coercion {
		return ast.ASTHandle{}, false, err
	}
	x, err := r.manager.GetArg(h, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	selected, err = r.selection.IsVariable(x)
	return x, selected, err
}

func (r *fmRecognizer) monomial(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	product, err := r.operation(h, r.arithmetic, 9)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if product {
		n, err := r.manager.GetNumArgs(h)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if n == 2 {
			coefficient, err := r.manager.GetArg(h, 0)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			_, _, numeral, err := r.manager.ArithmeticNumeralText(coefficient)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if numeral {
				x, err := r.manager.GetArg(h, 1)
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				variable, selected, err := r.variable(x)
				if err != nil || selected {
					return variable, selected, err
				}
			}
		}
	}
	return r.variable(h)
}

func (r *fmRecognizer) polynomial(h ast.ASTHandle) (bool, error) {
	addition, err := r.operation(h, r.arithmetic, 6)
	if err != nil {
		return false, err
	}
	monomials := []ast.ASTHandle{h}
	if addition {
		n, err := r.manager.GetNumArgs(h)
		if err != nil {
			return false, err
		}
		monomials = make([]ast.ASTHandle, n)
		for i := range monomials {
			monomials[i], err = r.manager.GetArg(h, uint32(i))
			if err != nil {
				return false, err
			}
		}
	}
	visited := make(map[ast.ASTHandle]bool)
	allForbidden := true
	for _, monomial := range monomials {
		x, linear, err := r.monomial(monomial)
		if err != nil || !linear {
			return false, err
		}
		if visited[x] {
			return false, nil
		}
		visited[x] = true
		index, err := r.manager.GetVarIndex(x)
		if err != nil {
			return false, err
		}
		if r.forbidden[index] {
			continue
		}
		if r.realOnly {
			sort, err := r.manager.GetSort(x)
			if err != nil {
				return false, err
			}
			integer, err := r.manager.MkFamilySort(r.arithmetic, 1, nil)
			if err != nil {
				return false, err
			}
			if sort == integer {
				continue
			}
		}
		allForbidden = false
	}
	return !allForbidden, nil
}

func (r *fmRecognizer) inequality(h ast.ASTHandle) (result bool, err error) {
	negative, err := r.operation(h, r.manager.BasicFamilyID(), ast.ASTOpNot)
	if err != nil {
		return false, err
	}
	if negative {
		h, err = r.manager.GetArg(h, 0)
		if err != nil {
			return false, err
		}
	}
	defer func() {
		if err != nil {
			return
		}
		err = r.emit("is_linear_ineq", 1183, func(out io.Writer) error {
			if err := writeExpression(r.manager, r.config, out, h); err != nil {
				return err
			}
			text := " false\n"
			if result {
				text = " true\n"
			}
			_, _ = io.WriteString(out, text)
			return nil
		})
	}()
	le, err := r.operation(h, r.arithmetic, 2)
	if err != nil {
		return false, err
	}
	ge, err := r.operation(h, r.arithmetic, 3)
	if err != nil || (!le && !ge) {
		return false, err
	}
	rhs, err := r.manager.GetArg(h, 1)
	if err != nil {
		return false, err
	}
	_, _, numeral, err := r.manager.ArithmeticNumeralText(rhs)
	if err != nil || !numeral {
		return false, err
	}
	lhs, err := r.manager.GetArg(h, 0)
	if err != nil {
		return false, err
	}
	return r.polynomial(lhs)
}

func (r *fmRecognizer) clause(h ast.ASTHandle) (bool, error) {
	disjunction, err := r.operation(h, r.manager.BasicFamilyID(), ast.ASTOpOr)
	if err != nil {
		return false, err
	}
	if !r.occ || !disjunction {
		return r.inequality(h)
	}
	n, err := r.manager.GetNumArgs(h)
	if err != nil {
		return false, err
	}
	found := false
	for i := uint32(0); i < n; i++ {
		arg, err := r.manager.GetArg(h, i)
		if err != nil {
			return false, err
		}
		literal, err := r.literal(arg)
		if err != nil {
			return false, err
		}
		if literal {
			continue
		}
		inequality, err := r.inequality(arg)
		if err != nil || !inequality || found {
			return false, err
		}
		found = true
	}
	return found, nil
}

func (r *fmRecognizer) emit(function string, line int, body func(io.Writer) error) error {
	tags := r.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe_lite", function, "src/qe/qe_lite.cpp", line, func() bool { return true }, func(out io.Writer) error { return body(traceBodyWriter{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (r *fmRecognizer) initForbidden(forms []ast.ASTHandle) error {
	r.forbidden = make(map[uint32]bool)
	visited := make(map[ast.ASTHandle]bool)
	for _, form := range forms {
		occ, err := r.clause(form)
		if err != nil {
			return err
		}
		line, prefix := 1523, "OCC: "
		if !occ {
			line, prefix = 1526, "not OCC:\n"
		}
		if err := r.emit("init_forbidden_set", line, func(out io.Writer) error {
			_, _ = io.WriteString(out, prefix)
			if err := writeExpression(r.manager, r.config, out, form); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		}); err != nil {
			return err
		}
		if occ {
			continue
		}
		if err := r.forbidExpression(form, visited); err != nil {
			return err
		}
	}
	return nil
}

func (r *fmRecognizer) forbidExpression(root ast.ASTHandle, visited map[ast.ASTHandle]bool) error {
	// quick_for_each_expr includes quantifier bodies and annotations without
	// shifting variable indices. The caller shares visited across non-OCC rows.
	todo := []ast.ASTHandle{root}
	for len(todo) != 0 {
		last := len(todo) - 1
		h := todo[last]
		todo = todo[:last]
		if visited[h] {
			continue
		}
		visited[h] = true
		kind, err := r.manager.GetKind(h)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTVar:
			_, selected, err := r.variable(h)
			if err != nil {
				return err
			}
			if !selected {
				continue
			}
			sort, err := r.manager.GetSort(h)
			if err != nil {
				return err
			}
			family, err := r.manager.GetFamilyID(sort)
			if err != nil {
				return err
			}
			if family != r.arithmetic {
				continue
			}
			index, err := r.manager.GetVarIndex(h)
			if err != nil {
				return err
			}
			r.forbidden[index] = true
		case ast.ASTApp:
			n, err := r.manager.GetNumArgs(h)
			if err != nil {
				return err
			}
			for i := uint32(0); i < n; i++ {
				arg, err := r.manager.GetArg(h, i)
				if err != nil {
					return err
				}
				todo = append(todo, arg)
			}
		case ast.ASTQuantifier:
			body, err := r.manager.GetQuantifierBody(h)
			if err != nil {
				return err
			}
			todo = append(todo, body)
			n, err := r.manager.GetNumPatterns(h)
			if err != nil {
				return err
			}
			for i := uint32(0); i < n; i++ {
				p, err := r.manager.GetPattern(h, i)
				if err != nil {
					return err
				}
				todo = append(todo, p)
			}
			n, err = r.manager.GetNumNoPatterns(h)
			if err != nil {
				return err
			}
			for i := uint32(0); i < n; i++ {
				p, err := r.manager.GetNoPattern(h, i)
				if err != nil {
					return err
				}
				todo = append(todo, p)
			}
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}
