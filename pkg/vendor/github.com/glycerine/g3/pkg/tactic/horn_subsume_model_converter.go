package tactic

// Source: src/tactic/horn_subsume_model_converter.cpp;
// src/tactic/horn_subsume_model_converter.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// HornSubsumeModelConverter restores interpretations for Horn clauses removed
// by subsumption. The declaration and body vectors own matching entries.
type HornSubsumeModelConverter struct {
	tacticConverterBase
	manager                     *ast.ASTManager
	services                    model.ModelEvaluatorConfig
	funcs, bodies               []ast.ASTHandle
	rewrite                     *rewriter.TheoryRewriter
	delayedHeads, delayedBodies []ast.ASTHandle
}

func NewHornSubsumeModelConverter(m *ast.ASTManager, services model.ModelEvaluatorConfig) (_ *HornSubsumeModelConverter, err error) {
	if m == nil || services.Context == nil || services.Globals == nil || services.Memory == nil || services.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err = m.ProofsEnabled(); err != nil {
		return nil, err
	}
	if services.Context.SymbolsField != m.Symbols || services.Globals.Context != services.Context {
		return nil, ast.ErrASTHandle
	}
	c := &HornSubsumeModelConverter{manager: m, services: services}
	if err = c.init(c, m.Budget(), uint64(reflect.TypeOf(*c).Size())); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Close())
		}
	}()
	c.rewrite, err = rewriter.NewTheoryRewriter(m, services.Context, services.Globals, services.Params)
	return c, err
}

func (c *HornSubsumeModelConverter) ownPair(first ast.ASTHandle, firsts *[]ast.ASTHandle, second ast.ASTHandle, seconds *[]ast.ASTHandle) (err error) {
	if err = c.manager.IncRef(first); err != nil {
		return err
	}
	acquired := true
	defer func() {
		if acquired {
			_, cleanup := c.manager.DecRefConsumed(first)
			err = errors.Join(err, cleanup)
		}
	}()
	if err = c.manager.IncRef(second); err != nil {
		return err
	}
	*firsts = append(*firsts, first)
	*seconds = append(*seconds, second)
	acquired = false
	return nil
}

// Insert delays conversion from a Horn head application until model application.
func (c *HornSubsumeModelConverter) Insert(head, body ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	if kind, e := c.manager.GetKind(head); e != nil || kind != ast.ASTApp {
		if e != nil {
			return e
		}
		return ast.ErrASTDomain
	}
	return c.ownPair(head, &c.delayedHeads, body, &c.delayedBodies)
}

// InsertConjunction is the source array overload and uses bool_rewriter::mk_and.
func (c *HornSubsumeModelConverter) InsertConjunction(head ast.ASTHandle, body []ast.ASTHandle) (err error) {
	if e := c.open(); e != nil {
		return e
	}
	ref, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, c.manager.DelRef(&ref)) }()
	if err = rewriter.SimplifyBooleanGateDefault(c.manager, ast.ASTOpAnd, body, ref); err != nil {
		return err
	}
	value, err := c.manager.RefGet(ref)
	if err != nil {
		return err
	}
	return c.Insert(head, value)
}

// InsertDefinition immediately records a predicate declaration and its body.
func (c *HornSubsumeModelConverter) InsertDefinition(predicate, body ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	if kind, e := c.manager.GetKind(predicate); e != nil || kind != ast.ASTFuncDecl {
		if e != nil {
			return e
		}
		return ast.ErrASTDomain
	}
	return c.ownPair(predicate, &c.funcs, body, &c.bodies)
}

func (c *HornSubsumeModelConverter) uninterpretedApp(h ast.ASTHandle) (bool, error) {
	kind, err := c.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := c.manager.GetFamilyID(h)
	return family == ast.NullFamilyID, err
}

// MkHorn converts head :- body into the predicate interpretation update used
// by the source converter.
func (c *HornSubsumeModelConverter) MkHorn(head, body ast.ASTHandle) (predicate ast.ASTHandle, result ast.ASTRef, ok bool, err error) {
	if err = c.open(); err != nil {
		return
	}
	if ok, err = c.uninterpretedApp(head); err != nil || !ok {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	predicate, err = c.manager.GetDecl(head)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	arity, err := c.manager.GetNumArgs(head)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	fv, err := rewriter.NewExprFreeVars(c.manager)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	if err = fv.Collect(head); err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	if err = fv.Accumulate(body); err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	if arity == 0 && fv.Empty() {
		result, err = c.manager.NewRef(body)
		return predicate, result, err == nil, err
	}
	boolSort, err := c.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	if err = fv.SetDefaultSort(boolSort); err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	names := make([]usymbol.Symbol, fv.Size())
	for i := range names {
		names[i] = usymbol.Numerical(uint32(len(names) - 1 - i))
	}
	if err = fv.Reverse(); err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}

	conjuncts := []ast.ASTHandle{body}
	var substitution []ast.ASTHandle
	for i := uint32(0); i < arity; i++ {
		arg, e := c.manager.GetArg(head, i)
		if e != nil {
			err = e
			return
		}
		sort, e := c.manager.GetSort(arg)
		if e != nil {
			err = e
			return
		}
		v, e := c.manager.MkVar(fv.Size()+i, sort)
		if e != nil {
			err = e
			return
		}
		kind, e := c.manager.GetKind(arg)
		if e != nil {
			err = e
			return
		}
		if kind == ast.ASTVar {
			index, e := c.manager.GetVarIndex(arg)
			if e != nil {
				err = e
				return
			}
			if int(index) >= len(substitution) {
				substitution = append(substitution, make([]ast.ASTHandle, int(index)+1-len(substitution))...)
			}
			if !substitution[index].IsNull() {
				eq, e := c.manager.MkEq(v, substitution[index])
				if e != nil {
					err = e
					return
				}
				conjuncts = append(conjuncts, eq)
			} else {
				substitution[index] = v
			}
		} else {
			eq, e := c.manager.MkEq(v, arg)
			if e != nil {
				err = e
				return
			}
			conjuncts = append(conjuncts, eq)
		}
	}
	bodyRef, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, c.manager.DelRef(&bodyRef))
		}
	}()
	if err = rewriter.SimplifyBooleanGateDefault(c.manager, ast.ASTOpAnd, conjuncts, bodyRef); err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	if len(substitution) != 0 {
		vs, e := rewriter.NewVarSubstitution(c.manager, false)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, false, e
		}
		defer func() { err = errors.Join(err, vs.Close()) }()
		term, e := c.manager.RefGet(bodyRef)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, false, e
		}
		if e = vs.Substitute(term, substitution, bodyRef); e != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, false, e
		}
	}
	bodyExpr, err := c.manager.RefGet(bodyRef)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	if fv.Empty() {
		if ast.ASTSourceDebug && len(substitution) != 0 {
			return ast.ASTHandle{}, ast.ASTRef{}, false, diagnostic.ErrDebugAssertion
		}
		published = true
		return predicate, bodyRef, true, nil
	}
	quantified, err := c.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: fv.Sorts(), Names: names, Body: bodyExpr})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	if err = c.rewrite.Rewrite(quantified, bodyRef); err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, false, err
	}
	published = true
	return predicate, bodyRef, true, nil
}

// MkHornClause accepts the source's closed forall* (body => head) form.
func (c *HornSubsumeModelConverter) MkHornClause(clause ast.ASTHandle) (predicate ast.ASTHandle, body ast.ASTRef, ok bool, err error) {
	if err = c.open(); err != nil {
		return
	}
	if ast.ASTSourceDebug {
		fv, e := rewriter.NewExprFreeVars(c.manager)
		if e != nil {
			err = e
			return
		}
		if e = fv.Collect(clause); e != nil {
			err = e
			return
		}
		if !fv.Empty() {
			err = diagnostic.ErrDebugAssertion
			return
		}
	}
	for {
		kind, e := c.manager.GetKind(clause)
		if e != nil {
			err = e
			return
		}
		if kind != ast.ASTQuantifier {
			break
		}
		forall, e := c.manager.IsForall(clause)
		if e != nil {
			err = e
			return
		}
		if !forall {
			break
		}
		clause, e = c.manager.GetQuantifierBody(clause)
		if e != nil {
			err = e
			return
		}
	}
	isImplies, e := c.isBasicOp(clause, ast.ASTOpImplies)
	if e != nil {
		err = e
		return
	}
	if !isImplies {
		return ast.ASTHandle{}, ast.ASTRef{}, false, nil
	}
	lhs, e := c.manager.GetArg(clause, 0)
	if e != nil {
		err = e
		return
	}
	rhs, e := c.manager.GetArg(clause, 1)
	if e != nil {
		err = e
		return
	}
	return c.MkHorn(rhs, lhs)
}

func (c *HornSubsumeModelConverter) isBasicOp(h ast.ASTHandle, op int32) (bool, error) {
	kind, err := c.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := c.manager.GetFamilyID(h)
	if err != nil || family != c.manager.BasicFamilyID() {
		return false, err
	}
	declKind, err := c.manager.GetDeclKind(h)
	return declKind == op, err
}

func (c *HornSubsumeModelConverter) addDefaultFalseInterpretations(root ast.ASTHandle, md *model.ModelCore) error {
	falsehood, err := c.manager.MkFalse()
	if err != nil {
		return err
	}
	seen := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		i := len(stack) - 1
		n := stack[i]
		stack = stack[:i]
		if _, found := seen[n]; found {
			continue
		}
		seen[n] = struct{}{}
		kind, e := c.manager.GetKind(n)
		if e != nil {
			return e
		}
		switch kind {
		case ast.ASTApp:
			decl, e := c.manager.GetDecl(n)
			if e != nil {
				return e
			}
			family, e := c.manager.GetFamilyID(decl)
			if e != nil {
				return e
			}
			sort, e := c.manager.GetSort(n)
			if e != nil {
				return e
			}
			boolSort, e := c.manager.MkBoolSort()
			if e != nil {
				return e
			}
			if sort == boolSort && family == ast.NullFamilyID {
				has, e := md.HasInterpretation(decl)
				if e != nil {
					return e
				}
				if !has {
					arity, e := c.manager.GetArity(decl)
					if e != nil {
						return e
					}
					if arity == 0 {
						if e = md.RegisterConstant(decl, falsehood); e != nil {
							return e
						}
					} else {
						fi, e := model.NewFunctionInterpretation(c.manager, arity)
						if e != nil {
							return e
						}
						if e = fi.SetElse(falsehood); e != nil {
							return errors.Join(e, fi.Close())
						}
						if e = md.RegisterFunction(decl, fi); e != nil {
							return errors.Join(e, fi.Close())
						}
					}
				}
			}
			nargs, e := c.manager.GetNumArgs(n)
			if e != nil {
				return e
			}
			for j := uint32(0); j < nargs; j++ {
				arg, x := c.manager.GetArg(n, j)
				if x != nil {
					return x
				}
				stack = append(stack, arg)
			}
		case ast.ASTQuantifier:
			qbody, e := c.manager.GetQuantifierBody(n)
			if e != nil {
				return e
			}
			stack = append(stack, qbody)
			np, e := c.manager.GetNumPatterns(n)
			if e != nil {
				return e
			}
			for j := uint32(0); j < np; j++ {
				p, x := c.manager.GetPattern(n, j)
				if x != nil {
					return x
				}
				stack = append(stack, p)
			}
			np, e = c.manager.GetNumNoPatterns(n)
			if e != nil {
				return e
			}
			for j := uint32(0); j < np; j++ {
				p, x := c.manager.GetNoPattern(n, j)
				if x != nil {
					return x
				}
				stack = append(stack, p)
			}
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (c *HornSubsumeModelConverter) clearOwned(vector *[]ast.ASTHandle) error {
	for i, h := range *vector {
		if h.IsNull() {
			continue
		}
		consumed, err := c.manager.DecRefConsumed(h)
		if consumed {
			(*vector)[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	*vector = nil
	return nil
}

func (c *HornSubsumeModelConverter) ApplyModel(out *ModelRef, goal uint32) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	if tacticSourceDebug && goal != 0 {
		return errTacticModelConverterGoal
	}
	md, err := out.Get()
	if err != nil {
		return err
	}
	if md == nil || md.Manager() != c.manager || md.DerivedModel() == nil {
		return ast.ErrASTHandle
	}
	if len(c.delayedHeads) != len(c.delayedBodies) || len(c.funcs) != len(c.bodies) {
		return ast.ErrASTDomain
	}
	for i := range c.delayedHeads {
		pred, bodyRef, ok, e := c.MkHorn(c.delayedHeads[i], c.delayedBodies[i])
		if e != nil {
			return e
		}
		if !ok {
			return ast.ErrASTDomain
		}
		body, e := c.manager.RefGet(bodyRef)
		if e != nil {
			return errors.Join(e, c.manager.DelRef(&bodyRef))
		}
		if e = c.InsertDefinition(pred, body); e != nil {
			return errors.Join(e, c.manager.DelRef(&bodyRef))
		}
		if e = c.manager.DelRef(&bodyRef); e != nil {
			return e
		}
	}
	if err = c.clearOwned(&c.delayedBodies); err != nil {
		return err
	}
	if err = c.clearOwned(&c.delayedHeads); err != nil {
		return err
	}
	evaluated, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, c.manager.DelRef(&evaluated)) }()
	rewritten, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, c.manager.DelRef(&rewritten)) }()
	for i := len(c.funcs); i > 0; {
		i--
		decl, body := c.funcs[i], c.bodies[i]
		if err = c.addDefaultFalseInterpretations(body, md); err != nil {
			return err
		}
		if ast.ASTSourceDebug {
			sort, e := c.manager.GetSort(body)
			if e != nil {
				return e
			}
			boolSort, e := c.manager.MkBoolSort()
			if e != nil {
				return e
			}
			if sort != boolSort {
				return diagnostic.ErrDebugAssertion
			}
		}
		if err = md.DerivedModel().Evaluate(body, evaluated, false, c.services); err != nil {
			return err
		}
		body, err = c.manager.RefGet(evaluated)
		if err != nil {
			return err
		}
		arity, e := c.manager.GetArity(decl)
		if e != nil {
			return e
		}
		if arity == 0 {
			existing, e := md.ConstantInterpretation(decl)
			if e != nil {
				return e
			}
			if !existing.IsNull() {
				if body, e = c.manager.MkOr(existing, body); e != nil {
					return e
				}
			}
			if e = c.rewrite.Rewrite(body, rewritten); e != nil {
				return e
			}
			body, e = c.manager.RefGet(rewritten)
			if e != nil {
				return e
			}
			if e = md.RegisterConstant(decl, body); e != nil {
				return e
			}
		} else {
			fi, e := md.FunctionInterpretation(decl)
			if e != nil {
				return e
			}
			if fi != nil {
				existing, e := fi.Else()
				if e != nil {
					return e
				}
				if !existing.IsNull() {
					if body, e = c.manager.MkOr(existing, body); e != nil {
						return e
					}
				}
			} else {
				fi, e = model.NewFunctionInterpretation(c.manager, arity)
				if e != nil {
					return e
				}
				if e = md.RegisterFunction(decl, fi); e != nil {
					return errors.Join(e, fi.Close())
				}
			}
			if e = c.rewrite.Rewrite(body, rewritten); e != nil {
				return e
			}
			body, e = c.manager.RefGet(rewritten)
			if e != nil {
				return e
			}
			if e = fi.SetElse(body); e != nil {
				return e
			}
		}
	}
	return nil
}

func (c *HornSubsumeModelConverter) ApplyUnindexed(out *ModelRef) error         { return c.ApplyModel(out, 0) }
func (c *HornSubsumeModelConverter) ApplyLabels(_ *ModelLabels, _ uint32) error { return c.open() }
func (c *HornSubsumeModelConverter) Cancel() error                              { return c.open() }
func (c *HornSubsumeModelConverter) Display(_ io.Writer) error                  { return c.open() }

func (c *HornSubsumeModelConverter) Translate(translator *ast.ASTTranslation) (_ ModelConverter, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	if translator == nil || translator.From() != c.manager || translator.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err := NewHornSubsumeModelConverter(translator.To(), c.services)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, result.Close())
		}
	}()
	for i := range c.funcs {
		decl, e := translator.Translate(c.funcs[i])
		if e != nil {
			return nil, e
		}
		body, e := translator.Translate(c.bodies[i])
		if e != nil {
			return nil, e
		}
		if e = result.InsertDefinition(decl, body); e != nil {
			return nil, e
		}
	}
	published = true
	return result, nil
}

func (c *HornSubsumeModelConverter) Close() error {
	if c == nil || c.self != &c.tacticConverterBase {
		return ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	if e := c.clearOwned(&c.delayedBodies); e != nil {
		return e
	}
	if e := c.clearOwned(&c.delayedHeads); e != nil {
		return e
	}
	if c.rewrite != nil {
		if e := c.rewrite.Close(); e != nil {
			return e
		}
		c.rewrite = nil
	}
	if e := c.clearOwned(&c.bodies); e != nil {
		return e
	}
	if e := c.clearOwned(&c.funcs); e != nil {
		return e
	}
	c.release()
	return nil
}
