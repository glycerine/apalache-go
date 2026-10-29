package arith

// Source specialization: the quantifier-free real DIV/POWER instantiation of
// src/tactic/arith/purify_arith_tactic.cpp used by qfnra_nlsat_tactic.cpp.

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

const (
	qfnraPurifyDiv     int32 = 10
	qfnraPurifyPower   int32 = 18
	qfnraPurifyNegRoot int32 = 33
)

type qfnraPurifyEntry struct {
	term, fresh, proof ast.ASTRef
}

type qfnraPurifyInvocation struct {
	owner       *QFNRAPurifyTactic
	goal        *tactic.Goal
	proofs      bool
	models      bool
	cache       map[ast.ASTHandle]*qfnraPurifyEntry
	entries     []*qfnraPurifyEntry
	constraints []ast.ASTRef
	proofRefs   []ast.ASTRef
	freshDecls  []ast.ASTHandle
}

func newQFNRAPurifyInvocation(owner *QFNRAPurifyTactic, goal *tactic.Goal) (*qfnraPurifyInvocation, error) {
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return nil, err
	}
	return &qfnraPurifyInvocation{owner: owner, goal: goal, proofs: proofs, models: models, cache: make(map[ast.ASTHandle]*qfnraPurifyEntry)}, nil
}

func (i *qfnraPurifyInvocation) close() (err error) {
	m := i.owner.manager
	for j := len(i.proofRefs) - 1; j >= 0; j-- {
		err = errors.Join(err, m.DelRef(&i.proofRefs[j]))
	}
	for j := len(i.constraints) - 1; j >= 0; j-- {
		err = errors.Join(err, m.DelRef(&i.constraints[j]))
	}
	for j := len(i.entries) - 1; j >= 0; j-- {
		e := i.entries[j]
		if e.proof != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&e.proof))
		}
		err = errors.Join(err, m.DelRef(&e.fresh), m.DelRef(&e.term))
	}
	clear(i.cache)
	i.entries = nil
	i.constraints = nil
	i.proofRefs = nil
	i.freshDecls = nil
	return err
}

type qfnraPurifyConfig struct {
	rewriter.DefaultConfiguration
	invocation *qfnraPurifyInvocation
}

func (c *qfnraPurifyConfig) ReduceApp(decl ast.ASTHandle, arguments ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	i := c.invocation
	m := i.owner.manager
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if family != i.owner.arithmetic {
		return rewriter.RewriteFailed, nil
	}
	op, err := m.GetDeclKind(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if arguments.Count() != 2 || (op != qfnraPurifyDiv && op != qfnraPurifyPower) {
		return rewriter.RewriteFailed, nil
	}
	args := [2]ast.ASTHandle{}
	for j := range args {
		args[j], err = arguments.At(uint32(j))
		if err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	term, err := m.MkApp(decl, args[:])
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if entry := i.cache[term]; entry != nil {
		fresh, e := m.RefGet(entry.fresh)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.SetRef(out, fresh); e != nil {
			return rewriter.RewriteFailed, e
		}
		if i.proofs {
			cachedProof, e := m.RefGet(entry.proof)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			if e = m.SetRef(proof, cachedProof); e != nil {
				return rewriter.RewriteFailed, e
			}
		}
		return rewriter.RewriteDone, nil
	}
	if op == qfnraPurifyPower {
		eligible, e := i.powerEligible(args[1])
		if e != nil || !eligible {
			return rewriter.RewriteFailed, e
		}
	}
	entry, err := i.newEntry(term)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	fresh, err := m.RefGet(entry.fresh)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if err = m.SetRef(out, fresh); err != nil {
		return rewriter.RewriteFailed, err
	}
	if i.proofs {
		definitionProof, e := m.RefGet(entry.proof)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.SetRef(proof, definitionProof); e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	if op == qfnraPurifyDiv {
		err = i.processDivision(args[0], args[1], fresh, entry)
	} else {
		err = i.processPower(args[0], args[1], fresh, entry)
	}
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	return rewriter.RewriteDone, nil
}

func (i *qfnraPurifyInvocation) powerValue(exponent ast.ASTHandle) (value unumeral.OwnedRational, matched bool, err error) {
	text, _, matched, err := i.owner.manager.ArithmeticNumeralText(exponent)
	if err != nil || !matched {
		return value, matched, err
	}
	service := i.owner.manager.NumeralService()
	value, err = service.NewValue()
	if err != nil {
		return value, false, err
	}
	if err = service.SetString(&value, text); err != nil {
		return value, false, errors.Join(err, service.Del(&value))
	}
	return value, true, nil
}

func (i *qfnraPurifyInvocation) powerEligible(exponent ast.ASTHandle) (_ bool, err error) {
	value, matched, err := i.powerValue(exponent)
	if err != nil || !matched {
		return false, err
	}
	service := i.owner.manager.NumeralService()
	defer func() { err = errors.Join(err, service.Del(&value)) }()
	integer, err := service.IsInt(&value)
	if err != nil {
		return false, err
	}
	zero, err := service.IsZero(&value)
	if err != nil {
		return false, err
	}
	return !integer || zero, nil
}

func (i *qfnraPurifyInvocation) newEntry(term ast.ASTHandle) (_ *qfnraPurifyEntry, err error) {
	m := i.owner.manager
	entry := &qfnraPurifyEntry{}
	if entry.term, err = m.NewRef(term); err != nil {
		return nil, err
	}
	sort, err := m.GetSort(term)
	if err != nil {
		return nil, errors.Join(err, m.DelRef(&entry.term))
	}
	fresh, err := m.MkFreshConstNullable(nil, sort)
	if err != nil {
		return nil, errors.Join(err, m.DelRef(&entry.term))
	}
	if entry.fresh, err = m.NewRef(fresh); err != nil {
		return nil, errors.Join(err, m.DelRef(&entry.term))
	}
	if i.proofs {
		equality, e := m.MkEq(fresh, term)
		if e != nil {
			return nil, errors.Join(e, m.DelRef(&entry.fresh), m.DelRef(&entry.term))
		}
		definition, e := m.MkProofDefIntro(equality)
		if e != nil {
			return nil, errors.Join(e, m.DelRef(&entry.fresh), m.DelRef(&entry.term))
		}
		apply, e := m.MkProofApplyDef(term, fresh, definition)
		if e != nil {
			return nil, errors.Join(e, m.DelRef(&entry.fresh), m.DelRef(&entry.term))
		}
		if entry.proof, e = m.NewRef(apply); e != nil {
			return nil, errors.Join(e, m.DelRef(&entry.fresh), m.DelRef(&entry.term))
		}
	}
	i.cache[term] = entry
	i.entries = append(i.entries, entry)
	decl, err := m.GetDecl(fresh)
	if err != nil {
		return nil, err
	}
	i.freshDecls = append(i.freshDecls, decl)
	return entry, nil
}

type qfnraPurifyTemps struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (t *qfnraPurifyTemps) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := t.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	t.refs = append(t.refs, r)
	return h, nil
}

func (t *qfnraPurifyTemps) close() (err error) {
	for j := len(t.refs) - 1; j >= 0; j-- {
		err = errors.Join(err, t.manager.DelRef(&t.refs[j]))
	}
	return err
}

func (i *qfnraPurifyInvocation) addConstraint(constraint ast.ASTHandle, entry *qfnraPurifyEntry) (err error) {
	m := i.owner.manager
	ref, err := m.NewRef(constraint)
	if err != nil {
		return err
	}
	i.constraints = append(i.constraints, ref)
	if i.proofs {
		premise, e := m.RefGet(entry.proof)
		if e != nil {
			return e
		}
		proof, e := m.MkProofTheoryLemma(i.owner.arithmetic, constraint, []ast.ASTHandle{premise}, nil)
		if e != nil {
			return e
		}
		proofRef, e := m.NewRef(proof)
		if e != nil {
			return e
		}
		i.proofRefs = append(i.proofRefs, proofRef)
	}
	return nil
}

func (i *qfnraPurifyInvocation) processDivision(x, y, fresh ast.ASTHandle, entry *qfnraPurifyEntry) (err error) {
	m := i.owner.manager
	temps := qfnraPurifyTemps{manager: m}
	defer func() { err = errors.Join(err, temps.close()) }()
	zero, err := m.ArithmeticRealNumeralText("0")
	if err != nil {
		return err
	}
	yZero, err := m.MkEq(y, zero)
	if err != nil {
		return err
	}
	if _, err = temps.hold(yZero); err != nil {
		return err
	}
	product, err := m.MkArithmeticMul(y, fresh)
	if err != nil {
		return err
	}
	if _, err = temps.hold(product); err != nil {
		return err
	}
	equality, err := m.MkEq(product, x)
	if err != nil {
		return err
	}
	if _, err = temps.hold(equality); err != nil {
		return err
	}
	constraint, err := m.MkOr(yZero, equality)
	if err != nil {
		return err
	}
	if err = i.addConstraint(constraint, entry); err != nil {
		return err
	}
	if !i.owner.complete {
		return nil
	}
	_, _, numeral, err := m.ArithmeticNumeralText(y)
	if err != nil {
		return err
	}
	if numeral {
		text, _, _, e := m.ArithmeticNumeralText(y)
		if e != nil {
			return e
		}
		if text != "0" && text != "0.0" {
			return nil
		}
	}
	notZero, err := m.MkNot(yZero)
	if err != nil {
		return err
	}
	if _, err = temps.hold(notZero); err != nil {
		return err
	}
	divZero, err := m.MkArithmeticDiv(x, zero)
	if err != nil {
		return err
	}
	if _, err = temps.hold(divZero); err != nil {
		return err
	}
	undefined, err := m.MkEq(fresh, divZero)
	if err != nil {
		return err
	}
	if _, err = temps.hold(undefined); err != nil {
		return err
	}
	functional, err := m.MkOr(notZero, undefined)
	if err != nil {
		return err
	}
	return i.addConstraint(functional, entry)
}

func (i *qfnraPurifyInvocation) processPower(x, exponent, fresh ast.ASTHandle, entry *qfnraPurifyEntry) (err error) {
	stage := "read exponent"
	defer func() {
		if err != nil {
			err = fmt.Errorf("qfnra purify power (%s): %w", stage, err)
		}
	}()
	m := i.owner.manager
	service := m.NumeralService()
	value, matched, err := i.powerValue(exponent)
	if err != nil || !matched {
		return errors.Join(ast.ErrASTDomain, err)
	}
	defer func() { err = errors.Join(err, service.Del(&value)) }()
	zeroValue, err := service.IsZero(&value)
	if err != nil {
		return err
	}
	temps := qfnraPurifyTemps{manager: m}
	defer func() { err = errors.Join(err, temps.close()) }()
	zero, err := m.ArithmeticRealNumeralText("0")
	if err != nil {
		return err
	}
	one, err := m.ArithmeticRealNumeralText("1")
	if err != nil {
		return err
	}
	if zeroValue {
		stage = "zero exponent first constraint"
		xZero, e := m.MkEq(x, zero)
		if e != nil {
			return e
		}
		if _, e = temps.hold(xZero); e != nil {
			return e
		}
		freshOne, e := m.MkEq(fresh, one)
		if e != nil {
			return e
		}
		if _, e = temps.hold(freshOne); e != nil {
			return e
		}
		first, e := m.MkOr(xZero, freshOne)
		if e != nil {
			return e
		}
		if e = i.addConstraint(first, entry); e != nil {
			return e
		}
		stage = "zero exponent second constraint"
		notXZero, e := m.MkNot(xZero)
		if e != nil {
			return e
		}
		if _, e = temps.hold(notXZero); e != nil {
			return e
		}
		zeroPowerZero, e := m.MkArithmeticPower(zero, zero)
		if e != nil {
			return e
		}
		if _, e = temps.hold(zeroPowerZero); e != nil {
			return e
		}
		undefined, e := m.MkEq(fresh, zeroPowerZero)
		if e != nil {
			return e
		}
		if _, e = temps.hold(undefined); e != nil {
			return e
		}
		second, e := m.MkOr(notXZero, undefined)
		if e != nil {
			return e
		}
		return i.addConstraint(second, entry)
	}
	stage = "root denominator"
	numeratorOne, err := service.Manager.Z.IsOne(value.Raw.BorrowNumerator())
	if err != nil {
		return err
	}
	if !numeratorOne {
		return ast.ErrASTDomain
	}
	denominatorText, err := service.Manager.Z.String(value.Raw.BorrowDenominator())
	if err != nil {
		return err
	}
	denominator, err := m.ArithmeticRealNumeralText(denominatorText)
	if err != nil {
		return err
	}
	stage = "root power"
	freshPower, err := m.MkArithmeticPower(fresh, denominator)
	if err != nil {
		return err
	}
	if _, err = temps.hold(freshPower); err != nil {
		return err
	}
	rootEquality, err := m.MkEq(x, freshPower)
	if err != nil {
		return err
	}
	if _, err = temps.hold(rootEquality); err != nil {
		return err
	}
	even, err := service.Manager.Z.IsEven(value.Raw.BorrowDenominator())
	if err != nil {
		return err
	}
	if !even {
		stage = "odd root constraint"
		return i.addConstraint(rootEquality, entry)
	}
	stage = "even root positive constraint"
	xNonnegative, err := m.MkArithmeticGE(x, zero)
	if err != nil {
		return err
	}
	if _, err = temps.hold(xNonnegative); err != nil {
		return err
	}
	notXNonnegative, err := m.MkNot(xNonnegative)
	if err != nil {
		return err
	}
	if _, err = temps.hold(notXNonnegative); err != nil {
		return err
	}
	freshNonnegative, err := m.MkArithmeticGE(fresh, zero)
	if err != nil {
		return err
	}
	if _, err = temps.hold(freshNonnegative); err != nil {
		return err
	}
	rootAndSign, err := m.MkAnd(rootEquality, freshNonnegative)
	if err != nil {
		return err
	}
	if _, err = temps.hold(rootAndSign); err != nil {
		return err
	}
	positiveCase, err := m.MkOr(notXNonnegative, rootAndSign)
	if err != nil {
		return err
	}
	if err = i.addConstraint(positiveCase, entry); err != nil {
		return err
	}
	stage = "even root negative constraint"
	negativeRoot, err := m.MkFamilyApp(i.owner.arithmetic, qfnraPurifyNegRoot, nil, []ast.ASTHandle{x, denominator})
	if err != nil {
		return err
	}
	if _, err = temps.hold(negativeRoot); err != nil {
		return err
	}
	negativeEquality, err := m.MkEq(fresh, negativeRoot)
	if err != nil {
		return err
	}
	if _, err = temps.hold(negativeEquality); err != nil {
		return err
	}
	negativeCase, err := m.MkOr(xNonnegative, negativeEquality)
	if err != nil {
		return err
	}
	return i.addConstraint(negativeCase, entry)
}

type QFNRAPurifyTactic struct {
	self       *QFNRAPurifyTactic
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	services   model.ModelEvaluatorConfig
	params     uparams.ParamsRef
	arithmetic ast.FamilyID
	complete   bool
	refs       uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*QFNRAPurifyTactic)(nil)

func NewQFNRAPurifyTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*QFNRAPurifyTactic, error) {
	if m == nil || context == nil || globals == nil || globals.Context != context || services.Context != context || services.Globals != globals {
		return nil, uparams.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	params, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(QFNRAPurifyTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, context.Del(&params))
	}
	t := &QFNRAPurifyTactic{manager: m, context: context, globals: globals, services: services, params: params, ownerBytes: amount}
	t.self = t
	if p != nil {
		err = context.Assign(&t.params, p)
	}
	if err == nil {
		t.arithmetic, err = m.RegisterArithmeticPlugin()
	}
	if err == nil {
		err = t.readPolicy(&t.params)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *QFNRAPurifyTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}

func (t *QFNRAPurifyTactic) readPolicy(p *uparams.ParamsRef) (err error) {
	name := "complete"
	t.complete, err = t.context.GetBoolName(p, &name, true)
	return err
}

func (t *QFNRAPurifyTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *QFNRAPurifyTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}

func (t *QFNRAPurifyTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.readPolicy(p)
}

func (t *QFNRAPurifyTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	description := "(default: true) constrain interpretations of underspecified arithmetic operators as functions."
	return out.InsertName("complete", uparams.ParamBool, &description, nil, nil)
}

func (t *QFNRAPurifyTactic) CollectStatistics() map[string]float64  { return nil }
func (t *QFNRAPurifyTactic) ResetStatistics() error                 { return t.open() }
func (t *QFNRAPurifyTactic) Cleanup() error                         { return t.open() }
func (t *QFNRAPurifyTactic) Reset() error                           { return t.Cleanup() }
func (t *QFNRAPurifyTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *QFNRAPurifyTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *QFNRAPurifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewQFNRAPurifyTactic(m, t.context, t.globals, &t.params, t.services)
}

func (t *QFNRAPurifyTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	err := t.context.Del(&t.params)
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return err
}

func (t *QFNRAPurifyTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("purify-arith", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	invocation, err := newQFNRAPurifyInvocation(t, goal)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, invocation.close()) }()
	config := &qfnraPurifyConfig{invocation: invocation}
	var traversal *rewriter.Traversal
	if invocation.proofs {
		traversal, err = rewriter.NewProofTraversal(t.manager, config)
	} else {
		traversal, err = rewriter.NewTraversal(t.manager, config)
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, traversal.Close()) }()
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for j := uint32(0); j < size; j++ {
		current, e := goal.Form(j)
		if e != nil {
			return e
		}
		newCurrent, e := t.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		newProof, e := t.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			_ = t.manager.DelRef(&newCurrent)
			return e
		}
		if invocation.proofs {
			e = traversal.RewriteWithProof(current, newCurrent, newProof)
		} else {
			e = traversal.Rewrite(current, newCurrent)
		}
		if e == nil {
			rewritten, x := t.manager.RefGet(newCurrent)
			if x != nil {
				e = x
			} else {
				proof := ast.ASTHandle{}
				if invocation.proofs {
					oldProof, x := goal.Proof(j)
					if x != nil {
						e = x
					} else if rewriteProof, x := t.manager.RefGet(newProof); x != nil {
						e = x
					} else {
						proof, e = t.manager.MkProofModusPonens(oldProof, rewriteProof)
					}
				}
				if e == nil {
					if dependency, x := goal.Dependency(j); x != nil {
						e = x
					} else {
						e = goal.Update(j, rewritten, proof, dependency)
					}
				}
			}
		}
		e = errors.Join(e, t.manager.DelRef(&newProof), t.manager.DelRef(&newCurrent))
		if e != nil {
			return e
		}
	}
	for j := range invocation.constraints {
		constraint, e := t.manager.RefGet(invocation.constraints[j])
		if e != nil {
			return e
		}
		proof := ast.ASTHandle{}
		if invocation.proofs {
			proof, e = t.manager.RefGet(invocation.proofRefs[j])
			if e != nil {
				return e
			}
		}
		if err = goal.AssertWithProof(constraint, proof, ast.ASTDependency{}); err != nil {
			return err
		}
	}
	if invocation.models && len(invocation.freshDecls) != 0 {
		filter, e := tactic.NewFilterModel(t.manager)
		if e != nil {
			return e
		}
		if e = filter.IncRef(); e != nil {
			return errors.Join(e, filter.Close())
		}
		defer func() { err = errors.Join(err, filter.DecRef()) }()
		for _, declaration := range invocation.freshDecls {
			if e = filter.Insert(declaration); e != nil {
				return e
			}
		}
		if err = result.SetModelConverter(filter); err != nil {
			return err
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}
