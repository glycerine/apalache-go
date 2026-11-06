package arith

// Source: src/tactic/arith/probe_arith.cpp/.h Boolean fragment probes.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type ArithmeticFragment uint8

const (
	FragmentQFLIA ArithmeticFragment = iota
	FragmentQFAUFLIA
	FragmentQFLRA
	FragmentQFLIRA
	FragmentILP
	FragmentQFNIA
	FragmentQFNRA
	FragmentNIA
	FragmentNRA
	FragmentNIRA
	FragmentLIA
	FragmentLRA
	FragmentLIRA
	FragmentQFUFNRA
)

type ArithmeticFragmentProbe struct {
	self    *ArithmeticFragmentProbe
	budget  *arena.Budget
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	kind    ArithmeticFragment
	trace   *diagnostic.TraceTags
	refs    uint32
	header  uint64
	closed  bool
}

var _ tactic.ProbeEvaluator = (*ArithmeticFragmentProbe)(nil)

func NewArithmeticFragmentProbe(b *arena.Budget, c *uparams.ParamsContext, g *uparams.GlobalParamsService, kind ArithmeticFragment) (*ArithmeticFragmentProbe, error) {
	if b == nil || c == nil || g == nil || g.Context != c || kind > FragmentQFUFNRA {
		return nil, uparams.ErrParamsOwner
	}
	n := uint64(reflect.TypeOf(ArithmeticFragmentProbe{}).Size())
	if err := b.Reserve(n); err != nil {
		return nil, err
	}
	p := &ArithmeticFragmentProbe{budget: b, context: c, globals: g, kind: kind, header: n}
	p.self = p
	return p, nil
}
func (p *ArithmeticFragmentProbe) Check() error {
	if p == nil || p.self != p || p.closed {
		return tactic.ErrRegistryOwner
	}
	return nil
}
func (p *ArithmeticFragmentProbe) IncRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	p.refs++
	return nil
}
func (p *ArithmeticFragmentProbe) DecRef() error {
	if err := p.Check(); err != nil {
		return err
	}
	if p.refs == 0 {
		return tactic.ErrRegistryOwner
	}
	p.refs--
	if p.refs == 0 {
		return p.Close()
	}
	return nil
}
func (p *ArithmeticFragmentProbe) Close() error {
	if p == nil || p.self != p {
		return tactic.ErrRegistryOwner
	}
	if p.closed {
		return nil
	}
	p.budget.Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}
func (p *ArithmeticFragmentProbe) BindTrace(tags *diagnostic.TraceTags) error {
	if err := p.Check(); err != nil {
		return err
	}
	p.trace = tags
	return nil
}

type arithmeticFragmentScan struct {
	probe                                               *ArithmeticFragmentProbe
	m                                                   *ast.ASTManager
	arith, array                                        ast.FamilyID
	integer, real, quant, linear, arrays, strict, ufnra bool
	nonlinear                                           bool
}

func (s *arithmeticFragmentScan) sortAllowed(sort ast.ASTHandle) (bool, error) {
	fid, err := s.m.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	if fid == s.m.BasicFamilyID() {
		return true, nil
	}
	if s.arrays && fid == s.array {
		return true, nil
	}
	if fid != s.arith {
		return false, nil
	}
	kind, err := s.m.GetDeclKind(sort)
	return s.integer && kind == 1 || s.real && kind == 0, err
}
func (s *arithmeticFragmentScan) numeral(h ast.ASTHandle) (bool, error) {
	kind, err := s.m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	fid, err := s.m.GetFamilyID(h)
	if err != nil || fid != s.arith {
		return false, err
	}
	op, err := s.m.GetDeclKind(h)
	return op == 0, err
}
func (s *arithmeticFragmentScan) reject(h ast.ASTHandle) (bool, error) {
	if s.strict || s.ufnra {
		return true, nil
	}
	p := s.probe
	tags := p.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("probe", "throw_found", "src/tactic/arith/probe_arith.cpp", 396, func() bool { return true }, func(w io.Writer) (err error) {
		sort, err := s.m.GetSort(h)
		if err != nil {
			return err
		}
		for i, term := range []ast.ASTHandle{h, sort} {
			if i == 1 {
				if _, err = io.WriteString(w, ": "); err != nil {
					return err
				}
			}
			var format ast.ASTHandle
			if i == 0 {
				format, err = s.m.SMT2PPExprTree(term, nil)
			} else {
				format, err = s.m.SMT2PPSort(term)
			}
			if err != nil {
				return err
			}
			fm, e := s.m.GetFormatManager()
			if e != nil {
				return e
			}
			held, e := fm.NewRef(format)
			if e != nil {
				return e
			}
			stream, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.m.Budget()})
			if e != nil {
				return errors.Join(e, fm.DelRef(&held))
			}
			// Source probes have no AST-manager identity. Borrow a compatible
			// printer registry when available; syntax classification itself is
			// independent of the construction context.
			context, globals := p.context, p.globals
			compatible := func(c *uparams.ParamsContext, g *uparams.GlobalParamsService) bool {
				return c != nil && g != nil && c.CheckServices(s.m.Symbols, s.m.NumeralService()) == nil && g.CheckOwner(c, s.m.Budget()) == nil
			}
			if !compatible(context, globals) {
				globals = uparams.BorrowProcessGlobalParamsService()
				if globals != nil {
					context = globals.Context
				}
			}
			if compatible(context, globals) {
				empty := context.EmptyValue()
				e = s.m.PrintFormat(stream, format, context, &empty, globals)
			} else {
				e = s.m.PrintFormat(stream, format, nil, nil, nil)
			}
			text, read := stream.String()
			closeErr := stream.Close()
			if e = errors.Join(e, read, closeErr, fm.DelRef(&held)); e != nil {
				return e
			}
			if _, e = io.WriteString(w, text); e != nil {
				return e
			}
		}
		_, err = io.WriteString(w, "\n")
		return err
	})
	if tags.IsStreamFailure(err) {
		err = nil
	}
	return true, err
}
func (s *arithmeticFragmentScan) visit(h ast.ASTHandle, kind ast.ASTKind) (bool, error) {
	if kind == ast.ASTVar {
		if !s.quant {
			return s.reject(h)
		}
		sort, err := s.m.GetSort(h)
		if err != nil {
			return false, err
		}
		yes, err := s.sortAllowed(sort)
		if err != nil || yes {
			return false, err
		}
		return s.reject(h)
	}
	if kind == ast.ASTQuantifier {
		if !s.quant {
			return s.reject(h)
		}
		return false, nil
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	if !s.ufnra {
		sort, err := s.m.GetSort(h)
		if err != nil {
			return false, err
		}
		yes, err := s.sortAllowed(sort)
		if err != nil {
			return false, err
		}
		if !yes {
			return s.reject(h)
		}
	}
	fid, err := s.m.GetFamilyID(h)
	if err != nil {
		return false, err
	}
	if fid == s.m.BasicFamilyID() {
		return false, nil
	}
	n, err := s.m.GetNumArgs(h)
	if err != nil {
		return false, err
	}
	if fid != s.arith {
		if s.ufnra || fid == ast.NullFamilyID && (n == 0 || s.arrays) {
			return false, nil
		}
		return s.reject(h)
	}
	op, err := s.m.GetDeclKind(h)
	if err != nil {
		return false, err
	}
	arg := func(i uint32) (ast.ASTHandle, error) { return s.m.GetArg(h, i) }
	if s.strict {
		switch op {
		case 0, 2, 3, 4, 5, 6:
			return false, nil
		case 9:
			if n != 2 {
				return s.reject(h)
			}
			a, e := arg(0)
			if e != nil {
				return false, e
			}
			yes, e := s.numeral(a)
			if e != nil || yes {
				return false, e
			}
			return s.reject(h)
		case 14:
			if s.real {
				return false, nil
			}
		}
		return s.reject(h)
	}
	if s.ufnra {
		switch op {
		case 0, 1, 2, 3, 4, 5, 6, 7, 8, 17:
			return false, nil
		case 9:
			if n == 2 {
				a, e := arg(0)
				if e != nil {
					return false, e
				}
				b, e := arg(1)
				if e != nil {
					return false, e
				}
				sort, e := s.m.GetSort(a)
				if e != nil {
					return false, e
				}
				sortKind, e := s.m.GetDeclKind(sort)
				if e != nil {
					return false, e
				}
				left, e := s.numeral(a)
				if e != nil {
					return false, e
				}
				right, e := s.numeral(b)
				if e != nil {
					return false, e
				}
				if sortKind == 0 && !left && !right {
					s.nonlinear = true
				}
			}
			return false, nil
		case 10, 11, 12, 13, 18:
			a, e := arg(1)
			if e != nil {
				return false, e
			}
			yes, e := s.numeral(a)
			if e != nil {
				return false, e
			}
			if !yes {
				return s.reject(h)
			}
			if op == 18 {
				s.nonlinear = true
			}
			return false, nil
		default:
			return s.reject(h)
		}
	}
	switch op {
	case 0, 2, 3, 4, 5, 6, 7, 8, 17:
		return false, nil
	case 9:
		if !s.linear {
			return false, nil
		}
		if n != 2 {
			return s.reject(h)
		}
		a, e := arg(0)
		if e != nil {
			return false, e
		}
		yes, e := s.numeral(a)
		if e != nil || yes {
			return false, e
		}
		return s.reject(h)
	case 10, 11, 12, 13:
		if !s.linear {
			return false, nil
		}
		a, e := arg(1)
		if e != nil {
			return false, e
		}
		yes, e := s.numeral(a)
		if e != nil || yes {
			return false, e
		}
		return s.reject(h)
	case 14, 15:
		return false, nil
	case 16:
		if !s.real {
			return false, nil
		}
	case 18:
		if !s.linear {
			return false, nil
		}
	case 1:
		if !s.linear && s.real {
			return false, nil
		}
	}
	return s.reject(h)
}
func arithmeticLP(g *tactic.Goal, family ast.FamilyID) (bool, error) {
	m := g.Manager()
	n, err := g.Size()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		h, e := g.Form(i)
		if e != nil {
			return false, e
		}
		sign := false
		for {
			neg, arg, e := m.IsNot(h)
			if e != nil {
				return false, e
			}
			if !neg {
				break
			}
			sign = !sign
			h = arg
		}
		kind, e := m.GetKind(h)
		if e != nil {
			return false, e
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		fid, e := m.GetFamilyID(h)
		if e != nil {
			return false, e
		}
		op, e := m.GetDeclKind(h)
		if e != nil {
			return false, e
		}
		if fid == m.BasicFamilyID() && op == ast.ASTOpEq && !sign {
			a, e := m.GetArg(h, 0)
			if e != nil {
				return false, e
			}
			sort, e := m.GetSort(a)
			if e != nil {
				return false, e
			}
			fid, e = m.GetFamilyID(sort)
			if e != nil {
				return false, e
			}
			if fid != family {
				return false, nil
			}
			continue
		}
		if fid != family || op < 2 || op > 5 {
			return false, nil
		}
	}
	return true, nil
}
func (p *ArithmeticFragmentProbe) Eval(g *tactic.Goal) (float64, error) {
	if err := p.Check(); err != nil {
		return 0, err
	}
	if g == nil || g.Manager() == nil {
		return 0, ast.ErrASTHandle
	}
	m := g.Manager()
	s := arithmeticFragmentScan{probe: p, m: m}
	var err error
	s.arith, err = m.ArithmeticFamilyID()
	if err != nil {
		return 0, err
	}
	s.array, err = m.ArrayFamilyID()
	if err != nil {
		return 0, err
	}
	switch p.kind {
	case FragmentQFLIA, FragmentILP:
		s.integer, s.strict = true, true
	case FragmentQFAUFLIA:
		s.integer, s.strict, s.arrays = true, true, true
	case FragmentQFLRA:
		s.real, s.strict = true, true
	case FragmentQFLIRA:
		s.integer, s.real, s.strict = true, true, true
	case FragmentQFNIA:
		s.integer = true
	case FragmentQFNRA:
		s.real = true
	case FragmentNIA:
		s.integer, s.quant = true, true
	case FragmentNRA:
		s.real, s.quant = true, true
	case FragmentNIRA:
		s.integer, s.real, s.quant = true, true, true
	case FragmentLIA:
		s.integer, s.quant, s.linear = true, true, true
	case FragmentLRA:
		s.real, s.quant, s.linear = true, true, true
	case FragmentLIRA:
		s.integer, s.real, s.quant, s.linear = true, true, true, true
	case FragmentQFUFNRA:
		s.ufnra = true
	}
	if s.ufnra {
		proofs, e := g.ProofsEnabled()
		if e != nil {
			return 0, e
		}
		if proofs {
			return 0, nil
		}
		cores, e := g.UnsatCoresEnabled()
		if e != nil {
			return 0, e
		}
		if cores {
			return 0, nil
		}
	}
	rejected, err := visitArithmeticGoal(g, s.visit)
	if err != nil || rejected {
		return 0, err
	}
	if s.ufnra && !s.nonlinear {
		return 0, nil
	}
	if p.kind == FragmentILP {
		ite, e := tactic.HasTermITE(g)
		if e != nil || ite {
			return 0, e
		}
		lp, e := arithmeticLP(g, s.arith)
		if e != nil || !lp {
			return 0, e
		}
	}
	return 1, nil
}
