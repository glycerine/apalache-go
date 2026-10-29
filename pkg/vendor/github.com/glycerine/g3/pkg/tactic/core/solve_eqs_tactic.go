package core

// Source: src/tactic/core/solve_eqs_tactic.cpp/.h, retained simplifying-replacer
// factory. The disabled nonlinear solver and unused external-replacer overload
// are not instantiated by retained source consumers.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SolveEquationsTactic struct {
	self                           *SolveEquationsTactic
	manager                        *ast.ASTManager
	context                        *uparams.ParamsContext
	globals                        *uparams.GlobalParamsService
	services                       model.ModelEvaluatorConfig
	params                         uparams.ParamsRef
	replacer                       *rewriter.SimplifyingExprReplacer
	substitution, normalized       ast.ASTExprSubstitution
	hasSubstitution, hasNormalized bool
	candidateVars, candidateSet    map[ast.ASTHandle]bool
	variables, candidates, ordered []ast.ASTHandle
	occs                           *ast.NumOccurs
	theorySolver, iteSolver        bool
	maxOccurrences                 uint32
	steps, eliminated, refs        uint32
	ownerBytes                     uint64
	trace                          *diagnostic.TraceTags
	proofs, cores, models, closed  bool
}

var _ tactic.Tactic = (*SolveEquationsTactic)(nil)

func NewSolveEquationsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*SolveEquationsTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	params, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(SolveEquationsTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, c.Del(&params))
	}
	t := &SolveEquationsTactic{manager: m, context: c, globals: g, services: services, params: params, ownerBytes: amount, candidateVars: make(map[ast.ASTHandle]bool), candidateSet: make(map[ast.ASTHandle]bool)}
	t.self = t
	if p != nil {
		if err = c.Assign(&t.params, p); err != nil {
			return nil, errors.Join(err, t.Close())
		}
	}
	if err = t.readPolicy(&t.params); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	t.replacer, err = rewriter.NewSimplifyingExprReplacer(m, c, g, &t.params)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	t.occs, err = ast.NewNumOccurs(m, false, true)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func MakeSolveEquationsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewSolveEquationsTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}

func (t *SolveEquationsTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.replacer == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *SolveEquationsTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *SolveEquationsTactic) DecRef() error {
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
func (t *SolveEquationsTactic) checkpoint() error {
	canceled, err := t.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(t.manager.ResourceLimit().CancelMessage())
	}
	return t.services.Cooperation.Cooperate("solve-eqs")
}
func (t *SolveEquationsTactic) readPolicy(p *uparams.ParamsRef) (err error) {
	key := "ite_solver"
	if t.iteSolver, err = t.context.GetBoolName(p, &key, true); err != nil {
		return err
	}
	key = "theory_solver"
	if t.theorySolver, err = t.context.GetBoolName(p, &key, true); err != nil {
		return err
	}
	key = "solve_eqs_max_occs"
	t.maxOccurrences, err = t.context.GetUIntName(p, &key, ^uint32(0))
	return err
}
func (t *SolveEquationsTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	// The native imp updates its solving policy, keeping its existing replacer.
	return t.readPolicy(p)
}
func (t *SolveEquationsTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if out == nil {
		return uparams.ErrParamDescrsPrecondition
	}
	for _, entry := range []struct {
		name        string
		kind        uparams.ParamKind
		description string
	}{
		{"solve_eqs_max_occs", uparams.ParamUInt, "(default: infty) maximum number of occurrences for considering a variable for gaussian eliminations."},
		{"theory_solver", uparams.ParamBool, "(default: true) use theory solvers."},
		{"ite_solver", uparams.ParamBool, "(default: true) use if-then-else solver."},
	} {
		if err := out.InsertName(entry.name, entry.kind, &entry.description, nil, nil); err != nil {
			return err
		}
	}
	return nil
}
func (t *SolveEquationsTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.open() != nil {
		return nil
	}
	if t.eliminated == 0 {
		return map[string]float64{}
	}
	return map[string]float64{"eliminated vars": float64(t.eliminated)}
}
func (t *SolveEquationsTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.eliminated = 0
	return nil
}
func (t *SolveEquationsTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *SolveEquationsTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *SolveEquationsTactic) Reset() error                           { return t.Cleanup() }

func (t *SolveEquationsTactic) clearState() error {
	// Constructor unwind may own a partial replacer without either mapping.
	// Detach installed mappings only; Close handles partial implementation roots.
	if t.replacer != nil && (t.hasSubstitution || t.hasNormalized) {
		if err := t.replacer.SetSubstitution(nil); err != nil {
			return err
		}
	}
	if t.hasSubstitution {
		if err := t.manager.DelExprSubstitution(&t.substitution); err != nil {
			return err
		}
		t.hasSubstitution = false
	}
	if t.hasNormalized {
		if err := t.manager.DelExprSubstitution(&t.normalized); err != nil {
			return err
		}
		t.hasNormalized = false
	}
	clear(t.candidateVars)
	clear(t.candidateSet)
	t.variables = nil
	t.candidates = nil
	t.ordered = nil
	t.steps = 0
	if t.occs != nil {
		t.occs.Reset()
	}
	return nil
}
func (t *SolveEquationsTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.clearState(); err != nil {
		return err
	}
	return t.readPolicy(&t.params)
}
func (t *SolveEquationsTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	translated, err := NewSolveEquationsTactic(m, t.context, t.globals, &t.params, t.services)
	if err != nil {
		return nil, err
	}
	translated.trace = t.trace
	return translated, nil
}
func (t *SolveEquationsTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.clearState(); err != nil {
		return err
	}
	if t.replacer != nil {
		if err := t.replacer.Close(); err != nil {
			return err
		}
		t.replacer = nil
	}
	if err := t.context.Del(&t.params); err != nil {
		return err
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}

func (t *SolveEquationsTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if sorted, e := goal.IsWellSorted(); e != nil {
		return e
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("solve_eqs", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if t.models, err = goal.ModelsEnabled(); err != nil {
		return err
	}
	if t.proofs, err = goal.ProofsEnabled(); err != nil {
		return err
	}
	if t.cores, err = goal.UnsatCoresEnabled(); err != nil {
		return err
	}
	inconsistent, err := goal.Inconsistent()
	if err != nil {
		return err
	}
	if !inconsistent {
		if err = t.clearState(); err != nil {
			return err
		}
		t.substitution, err = t.manager.NewExprSubstitutionPolicy(t.cores, t.proofs)
		if err != nil {
			return err
		}
		t.hasSubstitution = true
		t.normalized, err = t.manager.NewExprSubstitutionPolicy(t.cores, t.proofs)
		if err != nil {
			return err
		}
		t.hasNormalized = true
		var converter *tactic.ExtensionModelConverter
		for {
			if err = t.collectOccurrences(goal); err != nil {
				return err
			}
			if err = t.collect(goal); err != nil {
				return err
			}
			empty, e := t.manager.ExprSubstitutionEmpty(t.substitution)
			if e != nil {
				return e
			}
			if empty {
				break
			}
			if err = t.sortVariables(); err != nil {
				return err
			}
			if len(t.ordered) == 0 {
				break
			}
			if err = t.normalize(); err != nil {
				return err
			}
			if err = t.substitute(goal); err != nil {
				return err
			}
			if inconsistent, err = goal.Inconsistent(); err != nil {
				return err
			}
			if inconsistent {
				if err = result.SetModelConverter(nil); err != nil {
					return err
				}
				break
			}
			if err = t.reportEliminations(); err != nil {
				return err
			}
			t.eliminated += uint32(len(t.ordered))
			if t.models {
				if converter == nil {
					converter, err = tactic.NewExtensionModel(t.manager, t.services)
					if err != nil {
						return err
					}
					if err = result.SetModelConverter(converter); err != nil {
						return errors.Join(err, converter.Close())
					}
				}
				for _, v := range t.ordered {
					def, _, _, e := t.find(t.normalized, v)
					if e != nil {
						return e
					}
					decl, e := t.manager.GetDecl(v)
					if e != nil {
						return e
					}
					if err = converter.Insert(decl, def); err != nil {
						return err
					}
				}
			}
			if err = t.traceRound(goal, converter); err != nil {
				return err
			}
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	if err = t.traceGoal("solve_eqs", "operator()", 780, "", goal); err != nil {
		return err
	}
	if sorted, e := goal.IsWellSorted(); e != nil {
		return e
	} else if !sorted {
		return ast.ErrASTDomain
	}
	return tactic.ReportTacticProgress(":num-elim-vars", t.eliminated)
}

func (t *SolveEquationsTactic) collectOccurrences(goal *tactic.Goal) error {
	if t.maxOccurrences == ^uint32(0) {
		return nil
	}
	t.occs.Reset()
	size, err := goal.Size()
	if err != nil {
		return err
	}
	forms := make([]ast.ASTHandle, size)
	for i := uint32(0); i < size; i++ {
		forms[i], err = goal.Form(i)
		if err != nil {
			return err
		}
	}
	return t.occs.CountAll(forms)
}
func (t *SolveEquationsTactic) checkOccurrences(term ast.ASTHandle) (bool, error) {
	if t.maxOccurrences == ^uint32(0) {
		return true, nil
	}
	num := t.occs.GetNumOccurrences(term)
	if err := t.traceOccurrences(term, num); err != nil {
		return false, err
	}
	return num <= t.maxOccurrences, nil
}
func (t *SolveEquationsTactic) find(s ast.ASTExprSubstitution, key ast.ASTHandle) (def, proof ast.ASTHandle, dep ast.ASTDependency, err error) {
	found, err := t.manager.ExprSubstitutionFindDependency(s, key, &def, &proof, &dep)
	if err == nil && !found {
		err = ast.ErrASTDomain
	}
	return
}
func (t *SolveEquationsTactic) collect(goal *tactic.Goal) error {
	if err := t.manager.ExprSubstitutionReset(t.substitution); err != nil {
		return err
	}
	if err := t.manager.ExprSubstitutionReset(t.normalized); err != nil {
		return err
	}
	if err := t.replacer.SetSubstitution(nil); err != nil {
		return err
	}
	clear(t.candidateVars)
	clear(t.candidateSet)
	t.variables = nil
	t.candidates = nil
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		if err = t.checkpoint(); err != nil {
			return err
		}
		if err = t.collectCandidate(goal, i); err != nil {
			return err
		}
		t.steps++
	}
	return t.traceVariables("collect", 428, "candidate vars:\n", t.variables)
}
func (t *SolveEquationsTactic) collectCandidate(goal *tactic.Goal, index uint32) (err error) {
	frame := solveEquationFrame{manager: t.manager}
	defer func() { err = errors.Join(err, frame.Close()) }()
	f, err := goal.Form(index)
	if err != nil {
		return err
	}
	solution, found, err := t.solve(f, &frame)
	if err != nil || !found {
		return err
	}
	t.variables = append(t.variables, solution.variable)
	t.candidates = append(t.candidates, f)
	t.candidateVars[solution.variable] = true
	t.candidateSet[f] = true
	proof := solution.proof
	if t.proofs {
		old, e := goal.Proof(index)
		if e != nil {
			return e
		}
		if proof.IsNull() {
			proof = old
		} else {
			proof, err = t.manager.MkProofModusPonens(old, proof)
			if err != nil {
				return err
			}
			if err = frame.pin(proof); err != nil {
				return err
			}
		}
	}
	dep, err := goal.Dependency(index)
	if err != nil {
		return err
	}
	return t.manager.ExprSubstitutionInsert(t.substitution, solution.variable, solution.definition, proof, dep)
}

func (t *SolveEquationsTactic) rewrite(term ast.ASTHandle, out, proof ast.ASTRef) (ast.ASTDependency, error) {
	enabled, err := t.manager.ProofsEnabled()
	if err != nil {
		return ast.ASTDependency{}, err
	}
	if enabled {
		return t.replacer.RewriteWithProofAndDependency(term, out, proof)
	}
	if err = t.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return ast.ASTDependency{}, err
	}
	return t.replacer.RewriteWithDependency(term, out)
}

func (t *SolveEquationsTactic) normalize() (err error) {
	if err = t.manager.ExprSubstitutionReset(t.normalized); err != nil {
		return err
	}
	if err = t.replacer.SetSubstitution(&t.normalized); err != nil {
		return err
	}
	out, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	proof, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&proof)) }()
	for _, v := range t.ordered {
		if err = t.checkpoint(); err != nil {
			return err
		}
		def, original, dep, e := t.find(t.substitution, v)
		if e != nil {
			return e
		}
		used, e := t.rewrite(def, out, proof)
		if e != nil {
			return e
		}
		var joined tactic.DependencyRef
		if err = joined.Set(t.manager, used); err != nil {
			return errors.Join(err, t.manager.DependencyDec(used))
		}
		if err = t.manager.DependencyDec(used); err != nil {
			return errors.Join(err, joined.Close())
		}
		err = func() error {
			value, e := t.manager.RefGet(out)
			if e != nil {
				return e
			}
			pr, e := t.manager.RefGet(proof)
			if e != nil {
				return e
			}
			steps, e := t.replacer.NumSteps()
			if e != nil {
				return e
			}
			t.steps += steps + 1
			if t.proofs {
				pr, e = t.manager.MkProofTransitivity(original, pr)
				if e != nil {
					return e
				}
				if e = t.manager.SetRef(proof, pr); e != nil {
					return e
				}
			}
			if t.cores {
				d, e := t.manager.DependencyJoin(dep, joined.Get())
				if e != nil {
					return e
				}
				if e = joined.Set(t.manager, d); e != nil {
					return e
				}
			}
			return t.manager.ExprSubstitutionInsert(t.normalized, v, value, pr, joined.Get())
		}()
		if err = errors.Join(err, joined.Close()); err != nil {
			return err
		}
	}
	if err = t.manager.ExprSubstitutionReset(t.substitution); err != nil {
		return err
	}
	return t.traceNormalized()
}

func (t *SolveEquationsTactic) substitute(goal *tactic.Goal) (err error) {
	if err = t.replacer.SetSubstitution(&t.normalized); err != nil {
		return err
	}
	out, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	proof, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&proof)) }()
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		if err = t.checkpoint(); err != nil {
			return err
		}
		f, e := goal.Form(i)
		if e != nil {
			return e
		}
		if err = t.traceExpression("gaussian_leak", "substitute", 642, "processing:\n", f); err != nil {
			return err
		}
		if t.candidateSet[f] {
			delete(t.candidateSet, f)
			truth, e := t.manager.MkTrue()
			if e != nil {
				return e
			}
			trueProof, e := t.manager.MkProofTrue()
			if e != nil {
				return e
			}
			if err = goal.Update(i, truth, trueProof, ast.ASTDependency{}); err != nil {
				return err
			}
			t.steps++
			continue
		}
		used, e := t.rewrite(f, out, proof)
		if e != nil {
			return e
		}
		var joined tactic.DependencyRef
		if err = joined.Set(t.manager, used); err != nil {
			return errors.Join(err, t.manager.DependencyDec(used))
		}
		if err = t.manager.DependencyDec(used); err != nil {
			return errors.Join(err, joined.Close())
		}
		err = func() error {
			value, e := t.manager.RefGet(out)
			if e != nil {
				return e
			}
			if e = t.traceSubstitution(f, value); e != nil {
				return e
			}
			pr, e := t.manager.RefGet(proof)
			if e != nil {
				return e
			}
			steps, e := t.replacer.NumSteps()
			if e != nil {
				return e
			}
			t.steps += steps + 1
			if t.proofs {
				old, e := goal.Proof(i)
				if e != nil {
					return e
				}
				pr, e = t.manager.MkProofModusPonens(old, pr)
				if e != nil {
					return e
				}
				if e = t.manager.SetRef(proof, pr); e != nil {
					return e
				}
			}
			if t.cores {
				old, e := goal.Dependency(i)
				if e != nil {
					return e
				}
				dep, e := t.manager.DependencyJoin(old, joined.Get())
				if e != nil {
					return e
				}
				if e = joined.Set(t.manager, dep); e != nil {
					return e
				}
			}
			return goal.Update(i, value, pr, joined.Get())
		}()
		if err = errors.Join(err, joined.Close()); err != nil {
			return err
		}
		inconsistent, e := goal.Inconsistent()
		if e != nil || inconsistent {
			return e
		}
	}
	if err = goal.ElimTrue(); err != nil {
		return err
	}
	return t.traceGoal("solve_eqs", "substitute", 666, "after applying substitution\n", goal)
}
