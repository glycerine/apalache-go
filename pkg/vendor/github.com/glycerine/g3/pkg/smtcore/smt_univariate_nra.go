package smtcore

// QF_NRA Boolean-formula frontend over the general NLSAT solver. Unsupported
// syntax and deliberately observable legacy-strategy modes retain ordinary SMT.

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/nlsat"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

type smtNRAAtom struct {
	left, right ast.ASTHandle
	kind        nlsat.AtomKind
	negative    bool
}
type smtNRAInput struct {
	variables []ast.ASTHandle
	integers  []bool
	branches  [][]smtNRAAtom
	nonlinear bool
}

const smtNRAMaxBranches = 1024

func smtNRAAnd(left, right [][]smtNRAAtom) ([][]smtNRAAtom, bool) {
	if len(left) == 0 || len(right) == 0 {
		return nil, true
	}
	if len(left) > smtNRAMaxBranches/len(right) {
		return nil, false
	}
	result := make([][]smtNRAAtom, 0, len(left)*len(right))
	for _, a := range left {
		for _, b := range right {
			branch := make([]smtNRAAtom, 0, len(a)+len(b))
			branch = append(branch, a...)
			branch = append(branch, b...)
			result = append(result, branch)
		}
	}
	return result, true
}

func smtNRAOr(left, right [][]smtNRAAtom) ([][]smtNRAAtom, bool) {
	if len(right) > smtNRAMaxBranches || len(left) > smtNRAMaxBranches-len(right) {
		return nil, false
	}
	result := make([][]smtNRAAtom, 0, len(left)+len(right))
	result = append(result, left...)
	result = append(result, right...)
	return result, true
}

func (in *smtNRAInput) term(m *ast.ASTManager, h ast.ASTHandle, family ast.FamilyID) (bool, bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, false, err
	}
	count, err := m.GetNumArgs(h)
	if err != nil {
		return false, false, err
	}
	fid, err := m.GetFamilyID(h)
	if err != nil {
		return false, false, err
	}
	if fid == ast.NullFamilyID && count == 0 {
		sort, err := m.GetSort(h)
		if err != nil {
			return false, false, err
		}
		sf, err := m.GetFamilyID(sort)
		if err != nil {
			return false, false, err
		}
		sk, err := m.GetDeclKind(sort)
		if err != nil {
			return false, false, err
		}
		if sf != family || (sk != 0 && sk != 1) {
			return false, false, nil
		}
		for _, variable := range in.variables {
			if variable == h {
				return true, true, nil
			}
		}
		in.variables = append(in.variables, h)
		in.integers = append(in.integers, sk == 1)
		return true, true, nil
	}
	if fid != family {
		return false, false, nil
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return false, false, err
	}
	if op == 0 {
		_, _, matched, err := m.ArithmeticNumeralText(h)
		return matched, false, err
	}
	switch op {
	case 6, 7, 8, 9, 14, 18:
	default:
		return false, false, nil
	}
	varChildren := 0
	limit := count
	if op == 18 {
		if count != 2 {
			return false, false, nil
		}
		exponent, err := m.GetArg(h, 1)
		if err != nil {
			return false, false, err
		}
		text, _, matched, err := m.ArithmeticNumeralText(exponent)
		if err != nil || !matched {
			return false, false, err
		}
		power, err := strconv.ParseUint(text, 10, 32)
		if err != nil {
			return false, false, nil
		}
		if power > 1 {
			in.nonlinear = true
		}
		limit = 1
	}
	for i := uint32(0); i < limit; i++ {
		arg, err := m.GetArg(h, i)
		if err != nil {
			return false, false, err
		}
		ok, variable, err := in.term(m, arg, family)
		if err != nil || !ok {
			return false, false, err
		}
		if variable {
			varChildren++
		}
	}
	if op == 9 && varChildren > 1 {
		in.nonlinear = true
	}
	return true, varChildren > 0, nil
}
func (in *smtNRAInput) formula(k *SMTKernel, h ast.ASTHandle, negative bool, family ast.FamilyID) ([][]smtNRAAtom, bool, error) {
	m := k.manager
	if m.IsTrue(h) {
		if negative {
			return nil, true, nil
		}
		return [][]smtNRAAtom{{}}, true, nil
	}
	if m.IsFalse(h) {
		if negative {
			return [][]smtNRAAtom{{}}, true, nil
		}
		return nil, true, nil
	}
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return nil, false, err
	}
	op, basic, err := k.internalizer.basicOperation(h)
	if err != nil {
		return nil, false, err
	}
	count, err := m.GetNumArgs(h)
	if err != nil {
		return nil, false, err
	}
	if basic && op == ast.ASTOpNot && count == 1 {
		arg, err := m.GetArg(h, 0)
		if err != nil {
			return nil, false, err
		}
		return in.formula(k, arg, !negative, family)
	}
	if basic && (op == ast.ASTOpAnd || op == ast.ASTOpOr) {
		conjunction := op == ast.ASTOpAnd
		if negative {
			conjunction = !conjunction
		}
		result := [][]smtNRAAtom{{}}
		if !conjunction {
			result = nil
		}
		for i := uint32(0); i < count; i++ {
			arg, err := m.GetArg(h, i)
			if err != nil {
				return nil, false, err
			}
			child, ok, err := in.formula(k, arg, negative, family)
			if err != nil || !ok {
				return nil, false, err
			}
			if conjunction {
				result, ok = smtNRAAnd(result, child)
			} else {
				result, ok = smtNRAOr(result, child)
			}
			if !ok {
				return nil, false, nil
			}
		}
		return result, true, nil
	}
	if basic && op == ast.ASTOpImplies && count == 2 {
		a, err := m.GetArg(h, 0)
		if err != nil {
			return nil, false, err
		}
		b, err := m.GetArg(h, 1)
		if err != nil {
			return nil, false, err
		}
		left, ok, err := in.formula(k, a, !negative, family)
		if err != nil || !ok {
			return nil, false, err
		}
		right, ok, err := in.formula(k, b, negative, family)
		if err != nil || !ok {
			return nil, false, err
		}
		if negative {
			left, ok = smtNRAAnd(left, right)
		} else {
			left, ok = smtNRAOr(left, right)
		}
		return left, ok, nil
	}
	booleanComparison := false
	if basic && (op == ast.ASTOpEq || op == ast.ASTOpDistinct) && count == 2 {
		a, err := m.GetArg(h, 0)
		if err != nil {
			return nil, false, err
		}
		booleanComparison, err = k.internalizer.isBoolean(a)
		if err != nil {
			return nil, false, err
		}
	}
	if basic && (op == ast.ASTOpIff || op == ast.ASTOpXor || booleanComparison) && count == 2 {
		a, err := m.GetArg(h, 0)
		if err != nil {
			return nil, false, err
		}
		b, err := m.GetArg(h, 1)
		if err != nil {
			return nil, false, err
		}
		equal := op == ast.ASTOpIff || op == ast.ASTOpEq
		if negative {
			equal = !equal
		}
		ap, ok, err := in.formula(k, a, false, family)
		if err != nil || !ok {
			return nil, false, err
		}
		an, ok, err := in.formula(k, a, true, family)
		if err != nil || !ok {
			return nil, false, err
		}
		bp, ok, err := in.formula(k, b, false, family)
		if err != nil || !ok {
			return nil, false, err
		}
		bn, ok, err := in.formula(k, b, true, family)
		if err != nil || !ok {
			return nil, false, err
		}
		firstRight, secondRight := bp, bn
		if !equal {
			firstRight, secondRight = bn, bp
		}
		first, ok := smtNRAAnd(ap, firstRight)
		if !ok {
			return nil, false, nil
		}
		second, ok := smtNRAAnd(an, secondRight)
		if !ok {
			return nil, false, nil
		}
		result, ok := smtNRAOr(first, second)
		return result, ok, nil
	}
	if basic && op == ast.ASTOpIte && count == 3 {
		condition, err := m.GetArg(h, 0)
		if err != nil {
			return nil, false, err
		}
		thenBranch, err := m.GetArg(h, 1)
		if err != nil {
			return nil, false, err
		}
		elseBranch, err := m.GetArg(h, 2)
		if err != nil {
			return nil, false, err
		}
		cp, ok, err := in.formula(k, condition, false, family)
		if err != nil || !ok {
			return nil, false, err
		}
		cn, ok, err := in.formula(k, condition, true, family)
		if err != nil || !ok {
			return nil, false, err
		}
		tv, ok, err := in.formula(k, thenBranch, negative, family)
		if err != nil || !ok {
			return nil, false, err
		}
		ev, ok, err := in.formula(k, elseBranch, negative, family)
		if err != nil || !ok {
			return nil, false, err
		}
		first, ok := smtNRAAnd(cp, tv)
		if !ok {
			return nil, false, nil
		}
		second, ok := smtNRAAnd(cn, ev)
		if !ok {
			return nil, false, nil
		}
		result, ok := smtNRAOr(first, second)
		return result, ok, nil
	}
	relation := nlsat.AtomEQ
	if basic && op == ast.ASTOpDistinct && count == 2 {
		negative = !negative
	} else if !(basic && op == ast.ASTOpEq) {
		fid, err := m.GetFamilyID(h)
		if err != nil || fid != family {
			return nil, false, err
		}
		op, err = m.GetDeclKind(h)
		if err != nil {
			return nil, false, err
		}
		switch op {
		case smtArithOpLT:
			relation = nlsat.AtomLT
		case smtArithOpGT:
			relation = nlsat.AtomGT
		case smtArithOpLE:
			relation = nlsat.AtomGT
			negative = !negative
		case smtArithOpGE:
			relation = nlsat.AtomLT
			negative = !negative
		default:
			return nil, false, nil
		}
	}
	if count != 2 {
		return nil, false, nil
	}
	a, err := m.GetArg(h, 0)
	if err != nil {
		return nil, false, err
	}
	b, err := m.GetArg(h, 1)
	if err != nil {
		return nil, false, err
	}
	for _, h := range []ast.ASTHandle{a, b} {
		ok, _, err := in.term(m, h, family)
		if err != nil || !ok {
			return nil, false, err
		}
	}
	return [][]smtNRAAtom{{{a, b, relation, negative}}}, true, nil
}

type smtNRAWorkspace struct {
	kernel      *SMTKernel
	q           *unumeral.RationalManager
	am          *polynomial.AlgebraicManager
	pm          *polynomial.MultivariatePolynomialManager
	convert     *ast.Expr2Polynomial
	solver      *nlsat.Solver
	cooperation *cooperate.Cooperation
	primes      *primegen.Generator
	params      uparams.ParamsRef
	paramsReady bool
	witnesses   []polynomial.Algebraic
}

func (w *smtNRAWorkspace) resetBranch() error {
	if w.am != nil {
		for len(w.witnesses) > 0 {
			i := len(w.witnesses) - 1
			if err := w.am.Del(&w.witnesses[i]); err != nil {
				return err
			}
			w.witnesses = w.witnesses[:i]
		}
	}
	return nil
}

func (w *smtNRAWorkspace) Close() error {
	if err := w.resetBranch(); err != nil {
		return err
	}
	if w.solver != nil {
		if err := w.solver.Close(); err != nil {
			return err
		}
		w.solver = nil
	}
	if w.convert != nil {
		if err := w.convert.Close(); err != nil {
			return err
		}
		w.convert = nil
	}
	if w.am != nil {
		if err := w.am.CleanupRetained(); err != nil {
			return err
		}
	}
	if w.pm != nil {
		if err := w.pm.CloseChecked(); err != nil {
			return err
		}
		w.pm = nil
	}
	if w.am != nil {
		if err := w.am.Close(); err != nil {
			return err
		}
		w.am = nil
	}
	if w.q != nil {
		if err := w.q.CloseChecked(); err != nil {
			return err
		}
		w.q = nil
	}
	if w.primes != nil {
		if err := w.primes.Close(); err != nil {
			return err
		}
		w.primes = nil
	}
	if w.cooperation != nil {
		if err := w.cooperation.Close(); err != nil {
			return err
		}
		w.cooperation = nil
	}
	if w.paramsReady {
		if err := w.kernel.paramGlobals.Context.Del(&w.params); err != nil {
			return err
		}
		w.paramsReady = false
	}
	return nil
}
func (k *SMTKernel) tryNLSAT() (result lbool.LBool, handled bool, err error) {
	if (k.setupLogic != "QF_NRA" && k.setupLogic != "QF_NIA") || !k.nraInputsValid || len(k.nraInputs) == 0 {
		return lbool.Undef, false, nil
	}
	proofs, err := k.manager.ProofsEnabled()
	if err != nil || proofs {
		return lbool.Undef, false, err
	}
	family, err := k.manager.RegisterArithmeticPlugin()
	if err != nil {
		return lbool.Undef, false, err
	}
	in := smtNRAInput{}
	in.branches = [][]smtNRAAtom{{}}
	for _, a := range k.nraInputs {
		h, e := k.manager.RefGet(a)
		if e != nil {
			return lbool.Undef, false, e
		}
		branches, ok, e := in.formula(k, h, false, family)
		if e != nil || !ok {
			return lbool.Undef, false, e
		}
		in.branches, ok = smtNRAAnd(in.branches, branches)
		if !ok {
			return lbool.Undef, false, nil
		}
	}
	if !in.nonlinear || len(in.variables) == 0 || k.paramGlobals == nil {
		return lbool.Undef, false, nil
	}
	// Preserve the established non-model arithmetic engine and its strategy
	// observability until the general NLSAT path replaces that backend outright.
	if !k.preprocess || (len(in.variables) > 1 && !k.produceModel) {
		return lbool.Undef, false, nil
	}
	handled = true
	result = lbool.Undef
	w := &smtNRAWorkspace{kernel: k}
	k.univariateNRA = w
	defer func() {
		e := w.Close()
		if e == nil {
			k.univariateNRA = nil
		}
		err = errors.Join(err, e)
		if err != nil {
			result = lbool.Undef
		}
	}()
	w.q, err = k.manager.NumeralService().NewPrivateRationalManager()
	if err != nil {
		return
	}
	w.cooperation, err = cooperate.NewCooperation(k.manager.Budget())
	if err != nil {
		return
	}
	w.primes, err = primegen.NewGenerator(k.manager.Budget())
	if err != nil {
		return
	}
	w.params, err = k.paramGlobals.Context.NewValue()
	if err != nil {
		return
	}
	w.paramsReady = true
	w.am, err = polynomial.NewAlgebraicManager(polynomial.AlgebraicManagerConfig{Rationals: k.manager.NumeralService(), Numerals: w.q, Limit: &k.manager.Limit, Cooperation: w.cooperation, Params: &w.params, Globals: k.paramGlobals, Factorization: polynomial.UPolynomialFactorizationConfig{Primes: w.primes}})
	if err != nil {
		return
	}
	w.pm, err = polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{Core: polynomial.MultivariatePolynomialConfig{Integer: w.q.Integers(), Rationals: w.q, Checkpoint: k.manager.Limit.Inc, Cooperation: func() error { return nil }}, Factorization: polynomial.UPolynomialFactorizationConfig{Primes: w.primes}})
	if err != nil {
		return
	}
	w.solver, err = nlsat.NewSolver(nlsat.SolverConfig{Algebraic: w.am, Polynomial: w.pm, Limit: &k.manager.Limit, SimplifyCore: true, Factor: true})
	if err != nil {
		return
	}
	w.convert, err = ast.NewExpr2Polynomial(ast.Expr2PolynomialConfig{AST: k.manager, Polynomial: w.pm})
	if err != nil {
		return
	}
	// Pin real constants to stable polynomial variables in first-seen order.
	mapping, e := w.convert.Mapping()
	if e != nil {
		err = e
		return
	}
	for i, variable := range in.variables {
		created, e := w.solver.MkVar(in.integers[i])
		if e != nil {
			err = e
			return
		}
		if created != uint32(i) {
			err = ast.ErrASTDomain
			return
		}
		if err = mapping.Insert(variable, created); err != nil {
			return
		}
	}
	selectors := make([]sat.Literal, 0, len(in.branches))
	guarded := len(in.branches) > 1
	for _, branch := range in.branches {
		var selector sat.Literal
		if guarded {
			var b sat.BoolVar
			b, err = w.solver.MkBoolVar()
			if err != nil {
				return
			}
			selector = sat.MakeLiteral(b, false)
			selectors = append(selectors, selector)
		}
		for _, a := range branch {
			difference, e := k.manager.MkFamilyApp(family, smtArithOpSub, nil, []ast.ASTHandle{a.left, a.right})
			if e != nil {
				err = e
				return
			}
			ref, e := k.manager.NewRef(difference)
			if e != nil {
				err = e
				return
			}
			p, d, _, e := w.convert.ToPolynomial(difference)
			e = errors.Join(e, k.manager.DelRef(&ref))
			if e != nil {
				err = e
				return
			}
			value, e := p.Get()
			if e == nil {
				denominator, de := d.Get()
				if de == nil {
					var sign int32
					sign, de = w.q.Integers().Sign(denominator)
					if de == nil && sign <= 0 {
						de = ast.ErrASTDomain
					}
				}
				e = de
			}
			if e == nil {
				var literal sat.Literal
				literal, e = w.solver.MkInequality(a.kind, []polynomial.Polynomial{value}, []bool{false}, a.negative)
				if e == nil {
					clause := []sat.Literal{literal}
					if guarded {
						clause = []sat.Literal{selector.Negated(), literal}
					}
					e = w.solver.AddClause(clause, nlsat.Assumption{})
				}
			}
			e = errors.Join(e, d.Close(), p.Close())
			if e != nil {
				err = e
				return
			}
		}
	}
	if guarded {
		if err = w.solver.AddClause(selectors, nlsat.Assumption{}); err != nil {
			return
		}
	}
	result, err = w.solver.Check()
	if err != nil || result != lbool.True {
		return
	}
	if !k.produceModel {
		return
	}
	for variable := range in.variables {
		w.witnesses = append(w.witnesses, polynomial.Algebraic{})
		if err = w.solver.Value(uint32(variable), &w.witnesses[len(w.witnesses)-1]); err != nil {
			return
		}
	}
	md, e := model.NewModel(k.manager)
	if e != nil {
		err = e
		return
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, md.Close())
		}
	}()
	numeralFor := func(witness *polynomial.Algebraic, integer bool) (ast.ASTHandle, error) {
		rational, e := w.am.IsRational(witness)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if rational {
			var r unumeral.Rational
			if e = w.am.ToRational(witness, &r); e != nil {
				return ast.ASTHandle{}, e
			}
			text, stringErr := w.q.String(&r)
			stringErr = errors.Join(stringErr, w.q.Del(&r))
			if stringErr != nil {
				return ast.ASTHandle{}, stringErr
			}
			number, ok := new(big.Rat).SetString(text)
			if !ok {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
			return k.manager.ArithmeticNumeralRat(number, integer)
		}
		if integer {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		previous, e := uparams.BindProcessGlobalParamsService(k.paramGlobals)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		target, e := k.manager.ArithmeticAlgebraicManager()
		var value polynomial.Algebraic
		var numeral ast.ASTHandle
		if e == nil {
			e = target.CopyFromCore(&value, w.am.AlgebraicRootCore, witness)
		}
		if e == nil {
			numeral, e = k.manager.ArithmeticAlgebraicNumeral(&value)
		}
		if target != nil {
			e = errors.Join(e, target.Del(&value))
		}
		_, restore := uparams.BindProcessGlobalParamsService(previous)
		return numeral, errors.Join(e, restore)
	}
	for i, variable := range in.variables {
		numeral, e := numeralFor(&w.witnesses[i], in.integers[i])
		if e != nil {
			err = e
			return
		}
		decl, e := k.manager.GetDecl(variable)
		if e != nil {
			err = e
			return
		}
		if err = md.Core().RegisterConstant(decl, numeral); err != nil {
			return
		}
	}
	err = md.Core().IncRef()
	if err != nil {
		return
	}
	k.generated = md
	published = true
	return
}

func (k *SMTKernel) restoreNRAInputs(size int) error {
	for len(k.nraInputs) > size {
		last := len(k.nraInputs) - 1
		if err := k.manager.DelRef(&k.nraInputs[last]); err != nil {
			return err
		}
		k.nraInputs = k.nraInputs[:last]
	}
	return nil
}
