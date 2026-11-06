package arith

// Source: src/tactic/arith/elim01_tactic.cpp/.h.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type Elim01Tactic struct {
	self                         *Elim01Tactic
	manager                      *ast.ASTManager
	context                      *uparams.ParamsContext
	globals                      *uparams.GlobalParamsService
	services                     model.ModelEvaluatorConfig
	params                       uparams.ParamsRef
	bounds                       *BoundManager
	rewriter                     *rewriter.TheoryRewriter
	ownerBytes                   uint64
	refs, maxCoefficient         uint32
	paramsReady, closed, running bool
}

var _ tactic.Tactic = (*Elim01Tactic)(nil)

func NewElim01Tactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (_ *Elim01Tactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(Elim01Tactic{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &Elim01Tactic{manager: m, context: c, globals: g, services: services, ownerBytes: bytes, maxCoefficient: 8}
	t.self = t
	defer func() {
		if err != nil {
			err = errors.Join(err, t.Close())
		}
	}()
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.bounds, err = NewBoundManager(m, c, g)
	}
	if err == nil {
		t.rewriter, err = rewriter.NewTheoryRewriter(m, c, g, &t.params)
	}
	if err == nil {
		err = t.updatePolicy(&t.params)
	}
	return t, err
}

func MakeElim01Tactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewElim01Tactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}

func (t *Elim01Tactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.bounds == nil || t.rewriter == nil {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.params)
}
func (t *Elim01Tactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *Elim01Tactic) DecRef() error {
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
func (t *Elim01Tactic) Close() (err error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	if t.rewriter != nil {
		err = errors.Join(err, t.rewriter.Close())
		if err == nil {
			t.rewriter = nil
		}
	}
	if err == nil && t.bounds != nil {
		err = t.bounds.Close()
		if err == nil {
			t.bounds = nil
		}
	}
	if err == nil && t.paramsReady {
		err = t.context.Del(&t.params)
		if err == nil {
			t.paramsReady = false
		}
	}
	if err == nil {
		t.manager.Budget().Release(t.ownerBytes)
		t.ownerBytes = 0
		t.closed = true
	}
	return err
}
func (t *Elim01Tactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.bounds.Reset(); err != nil {
		return err
	}
	return t.rewriter.Reset()
}
func (t *Elim01Tactic) Reset() error { return t.Cleanup() }
func (t *Elim01Tactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewElim01Tactic(m, t.context, t.globals, &t.params, t.services)
}
func (t *Elim01Tactic) updatePolicy(p *uparams.ParamsRef) error {
	key := "max_coefficient"
	value, err := t.context.GetUIntName(p, &key, 8)
	if err == nil {
		t.maxCoefficient = value
	}
	return err
}
func (t *Elim01Tactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	if err := t.rewriter.UpdateParams(p); err != nil {
		return err
	}
	return t.updatePolicy(p)
}
func (t *Elim01Tactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	description := "(default: 1) maximal upper bound for finite range -> Bool conversion"
	return out.InsertName("max_coefficient", uparams.ParamUInt, &description, nil, nil)
}
func (t *Elim01Tactic) CollectStatistics() map[string]float64  { return nil }
func (t *Elim01Tactic) ResetStatistics() error                 { return t.open() }
func (t *Elim01Tactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *Elim01Tactic) SetProgressCallback(func(uint32)) error { return t.open() }

type elim01Conversion struct {
	sub       *rewriter.ExprSafeReplace
	extension *tactic.ExtensionModelConverter
	filter    *tactic.FilterModelConverter
	axioms    []ast.ASTRef
	changed   bool
}

func (t *Elim01Tactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	t.running = true
	defer func() { t.running = false }()
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("elim01", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = t.bounds.Reset(); err != nil {
		return err
	}
	if err = t.bounds.CollectGoal(goal); err != nil {
		return err
	}
	conversion := elim01Conversion{}
	conversion.sub, err = rewriter.NewExprSafeReplace(t.manager)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, conversion.sub.Close()) }()
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	if models {
		conversion.extension, err = tactic.NewExtensionModel(t.manager, t.services)
		if err == nil {
			conversion.filter, err = tactic.NewFilterModel(t.manager)
		}
		if err != nil {
			return err
		}
		if err = conversion.extension.IncRef(); err != nil {
			return err
		}
		defer func() { err = errors.Join(err, conversion.extension.DecRef()) }()
		if err = conversion.filter.IncRef(); err != nil {
			return err
		}
		defer func() { err = errors.Join(err, conversion.filter.DecRef()) }()
	}
	variables, err := t.bounds.Variables()
	if err != nil {
		return err
	}
	for _, variable := range variables {
		if err = t.addVariable(variable, models, &conversion); err != nil {
			return err
		}
	}
	defer func() {
		for i := len(conversion.axioms) - 1; i >= 0; i-- {
			err = errors.Join(err, t.manager.DelRef(&conversion.axioms[i]))
		}
	}()
	if !conversion.changed {
		return result.AddGoal(goal)
	}
	if models {
		converter, e := tactic.ConcatModels(t.manager.Budget(), conversion.filter, conversion.extension)
		if e != nil {
			return e
		}
		if err = result.SetModelConverter(converter); err != nil {
			return errors.Join(err, converter.Close())
		}
	}
	if err = t.rewriteGoal(goal, conversion.sub); err != nil {
		return err
	}
	for i := range conversion.axioms {
		axiom, e := t.manager.RefGet(conversion.axioms[i])
		if e != nil {
			return e
		}
		if err = goal.Assert(axiom); err != nil {
			return err
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}

func (t *Elim01Tactic) addVariable(variable ast.ASTHandle, models bool, conversion *elim01Conversion) error {
	m := t.manager
	sort, err := m.GetSort(variable)
	if err != nil {
		return err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return err
	}
	kind, err := m.GetDeclKind(sort)
	if err != nil || family != t.bounds.family || kind != 1 {
		return err
	}
	lo, lowStrict, hasLow, err := t.bounds.Lower(variable)
	if err != nil {
		return err
	}
	hi, highStrict, hasHigh, err := t.bounds.Upper(variable)
	if err != nil || !hasLow || !hasHigh || lowStrict || highStrict || lo.Sign() < 0 || lo.Cmp(&hi) > 0 || !lo.IsInt() || !hi.IsInt() {
		return err
	}
	max := new(big.Int).Set(hi.Num())
	min := new(big.Int).Set(lo.Num())
	if !max.IsUint64() || max.Uint64() > uint64(t.maxCoefficient) || !min.IsUint64() {
		return nil
	}
	maxValue, minValue := max.Uint64(), min.Uint64()
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return err
	}
	decl, err := m.GetDecl(variable)
	if err != nil {
		return err
	}
	nameSymbol, err := m.GetASTName(decl)
	if err != nil {
		return err
	}
	name, err := m.Symbols.String(nameSymbol)
	if err != nil {
		return err
	}
	terms := make([]ast.ASTHandle, 0, 4)
	for shift := uint32(0); maxValue >= uint64(1)<<shift; shift++ {
		fresh, e := m.MkFreshConst(name, boolSort)
		if e != nil {
			return e
		}
		weight, e := m.ArithmeticIntegerNumeralText(new(big.Int).Lsh(big.NewInt(1), uint(shift)).String())
		if e != nil {
			return e
		}
		term, e := m.MkIte(fresh, weight, zero)
		if e != nil {
			return e
		}
		terms = append(terms, term)
		if models {
			freshDecl, e := m.GetDecl(fresh)
			if e != nil {
				return e
			}
			if e = conversion.filter.Insert(freshDecl); e != nil {
				return e
			}
		}
	}
	definition := zero
	if len(terms) == 1 {
		definition = terms[0]
	} else if len(terms) > 1 {
		definition, err = m.MkFamilyApp(t.bounds.family, 6, nil, terms)
		if err != nil {
			return err
		}
	}
	if err = conversion.sub.Insert(variable, definition); err != nil {
		return err
	}
	if models {
		if err = conversion.extension.Insert(decl, definition); err != nil {
			return err
		}
	}
	appendAxiom := func(op int32, value uint64) error {
		number, e := m.ArithmeticIntegerNumeralText(new(big.Int).SetUint64(value).String())
		if e != nil {
			return e
		}
		axiom, e := m.MkFamilyApp(t.bounds.family, op, nil, []ast.ASTHandle{definition, number})
		if e != nil {
			return e
		}
		ref, e := m.NewRef(axiom)
		if e != nil {
			return e
		}
		conversion.axioms = append(conversion.axioms, ref)
		return nil
	}
	if maxValue&(maxValue+1) != 0 {
		if err = appendAxiom(2, maxValue); err != nil {
			return err
		}
	}
	if minValue > 0 {
		if err = appendAxiom(3, minValue); err != nil {
			return err
		}
	}
	conversion.changed = true
	return nil
}

func (t *Elim01Tactic) rewriteGoal(goal *tactic.Goal, replacement *rewriter.ExprSafeReplace) (err error) {
	m := t.manager
	temporary, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&temporary)) }()
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&out)) }()
	proof, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&proof)) }()
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		form, e := goal.Form(i)
		if e != nil {
			return e
		}
		if e = replacement.Apply(form, temporary); e != nil {
			return e
		}
		replaced, e := m.RefGet(temporary)
		if e != nil {
			return e
		}
		if proofs {
			e = t.rewriter.RewriteWithProof(replaced, out, proof)
		} else {
			e = t.rewriter.Rewrite(replaced, out)
		}
		if e != nil {
			return e
		}
		value, e := m.RefGet(out)
		if e != nil {
			return e
		}
		var pr ast.ASTHandle
		if proofs {
			old, e := goal.Proof(i)
			if e != nil {
				return e
			}
			pr, e = m.MkProofRewrite(form, value)
			if e == nil {
				pr, e = m.MkProofModusPonens(old, pr)
			}
			if e != nil {
				return e
			}
		}
		dependency, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if e = goal.Update(i, value, pr, dependency); e != nil {
			return e
		}
	}
	return nil
}
