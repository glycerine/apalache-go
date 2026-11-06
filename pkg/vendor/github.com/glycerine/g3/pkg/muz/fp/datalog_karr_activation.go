package fp

// Concrete acyclic rule consumer of src/muz/rel/karr_relation.cpp.
// It intentionally rejects shapes requiring the general relational compiler.
import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrKarrDatalogFragment = errors.New("muz/fp: unsupported Karr Datalog fragment")

type karrDatalogActivation struct {
	manager        *ast.ASTManager
	rules          *base.RuleSet
	seeds          map[ast.ASTHandle]*rel.KarrRelation
	results        map[ast.ASTHandle]*rel.KarrRelation
	formulaChecker *rel.KarrRelationFormulaChecker
	formulaConfig  *smtcore.SMTSolverConfig
	closed         bool
}

func newKarrDatalogActivation(manager *ast.ASTManager, rules *base.RuleSet, declarations []ast.ASTHandle, checks ...*smtcore.SMTSolverConfig) (*karrDatalogActivation, error) {
	if manager == nil || rules == nil {
		return nil, ErrKarrDatalogFragment
	}
	a := &karrDatalogActivation{manager: manager, rules: rules, seeds: make(map[ast.ASTHandle]*rel.KarrRelation)}
	if len(checks) > 0 && checks[0] != nil {
		config := *checks[0]
		checker, err := rel.NewKarrRelationFormulaChecker(manager, config)
		if err != nil {
			return nil, err
		}
		a.formulaChecker = checker
		a.formulaConfig = &config
	}
	if err := a.syncPredicates(declarations); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *karrDatalogActivation) checkpoint() error {
	canceled, err := a.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return z3exception.NewDefaultException(a.manager.ResourceLimit().CancelMessage())
	}
	return nil
}

func (a *karrDatalogActivation) syncPredicates(declarations []ast.ASTHandle) error {
	if a == nil || a.closed {
		return ErrKarrDatalogFragment
	}
	seen := make(map[ast.ASTHandle]bool, len(declarations))
	// The initial typed path admits only integer columns and nullaries.
	family, err := a.manager.RegisterArithmeticPlugin()
	if err != nil {
		return err
	}
	integer, err := a.manager.MkFamilySort(family, 1, nil)
	if err != nil {
		return err
	}
	boolean, err := a.manager.MkBoolSort()
	if err != nil {
		return err
	}
	pending := make(map[ast.ASTHandle]*rel.KarrRelation)
	pendingSorts := make(map[ast.ASTHandle][]ast.ASTHandle)
	for _, d := range declarations {
		if seen[d] {
			return ErrKarrDatalogFragment
		}
		seen[d] = true
		sort, err := a.manager.GetRange(d)
		if err != nil {
			return err
		}
		if sort != boolean {
			return ErrKarrDatalogFragment
		}
		arity, err := a.manager.GetArity(d)
		if err != nil {
			return err
		}
		for i := uint32(0); i < arity; i++ {
			s, err := a.manager.GetDomain(d, i)
			if err != nil {
				return err
			}
			if s != integer {
				return ErrKarrDatalogFragment
			}
			pendingSorts[d] = append(pendingSorts[d], s)
		}
		if a.seeds[d] == nil {
			pending[d] = rel.NewKarrEmpty(arity, func() bool { return a.checkpoint() == nil }, a.formulaChecker)
			if a.formulaChecker != nil {
				if err := rel.VerifyKarrRelationConstant(a.manager, pendingSorts[d], *a.formulaConfig, pending[d], false); err != nil {
					return err
				}
			}
		}
	}
	for d := range a.seeds {
		if !seen[d] {
			return ErrKarrDatalogFragment
		}
	}
	for d, r := range pending {
		a.seeds[d] = r
	}
	return nil
}

func (a *karrDatalogActivation) addFact(d ast.ASTHandle, values []ast.ASTHandle) error {
	if a == nil || a.closed || a.seeds[d] == nil {
		return ErrKarrDatalogFragment
	}
	if !a.seeds[d].Empty() {
		return ErrKarrDatalogFragment
	} // source add_fact asserts empty
	nums := make([]*big.Rat, len(values))
	for i, v := range values {
		text, _, matched, err := a.manager.ArithmeticNumeralText(v)
		if err != nil {
			return err
		}
		if matched {
			n, ok := new(big.Rat).SetString(text)
			if !ok {
				return ast.ErrASTDomain
			}
			nums[i] = n
		}
	}
	candidate := a.seeds[d].Clone()
	if err := candidate.AddFact(nums); err != nil {
		return err
	}
	if a.formulaChecker != nil {
		sorts, err := a.relationSorts(d)
		if err != nil {
			return err
		}
		before, err := a.seeds[d].ToFormula(a.manager, sorts)
		if err != nil {
			return err
		}
		defer a.manager.DelRef(&before)
		beforeHandle, err := a.manager.RefGet(before)
		if err != nil {
			return err
		}
		if err = rel.VerifyKarrRelationFactAdd(a.manager, sorts, *a.formulaConfig, beforeHandle, values, candidate); err != nil {
			return err
		}
	}
	a.seeds[d] = candidate
	a.results = nil
	return nil
}

func (a *karrDatalogActivation) clone(rules *base.RuleSet) (*karrDatalogActivation, error) {
	if a == nil || a.closed || rules == nil {
		return nil, ErrKarrDatalogFragment
	}
	b := &karrDatalogActivation{manager: a.manager, rules: rules, seeds: make(map[ast.ASTHandle]*rel.KarrRelation, len(a.seeds)), formulaChecker: a.formulaChecker}
	if a.formulaConfig != nil {
		config := *a.formulaConfig
		b.formulaConfig = &config
	}
	for d, r := range a.seeds {
		b.seeds[d] = r.Clone()
	}
	return b, nil
}

func (a *karrDatalogActivation) close() error {
	if a != nil {
		a.closed = true
		a.seeds = nil
		a.results = nil
	}
	return nil
}
func (a *karrDatalogActivation) reset() error {
	if a == nil || a.closed {
		return ErrKarrDatalogFragment
	}
	for d, r := range a.seeds {
		empty := rel.NewKarrEmpty(r.Dimension(), func() bool { return a.checkpoint() == nil }, a.formulaChecker)
		if a.formulaChecker != nil {
			sorts, err := a.relationSorts(d)
			if err != nil {
				return err
			}
			if err = rel.VerifyKarrRelationConstant(a.manager, sorts, *a.formulaConfig, empty, false); err != nil {
				return err
			}
		}
		a.seeds[d] = empty
	}
	a.results = nil
	return nil
}

func (a *karrDatalogActivation) relationSorts(declaration ast.ASTHandle) ([]ast.ASTHandle, error) {
	arity, err := a.manager.GetArity(declaration)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, arity)
	for i := range sorts {
		sorts[i], err = a.manager.GetDomain(declaration, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return sorts, nil
}

// query evaluates supported rules in dependency order, publishing a snapshot
// only on success.
func (a *karrDatalogActivation) query(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if a == nil || a.closed {
		return lbool.Undef, ErrKarrDatalogFragment
	}
	a.results = nil
	if err := a.checkpoint(); err != nil {
		return lbool.Undef, err
	}
	type compiledRule struct {
		ground *karrGroundRule
		unary  *karrUnaryRule
		binary *karrBinaryRule
		guard  *karrGuardRule
	}
	edges := make(map[ast.ASTHandle][]compiledRule)
	count, err := a.rules.Len()
	if err != nil {
		return lbool.Undef, err
	}
	for i := 0; i < count; i++ {
		rule, err := a.rules.Rule(i)
		if err != nil {
			return lbool.Undef, err
		}
		tailSize, err := rule.TailSize()
		if err == nil && tailSize == 0 {
			var head ast.ASTHandle
			var compiled karrGroundRule
			head, compiled, err = a.compileGroundRule(rule)
			if err == nil {
				edges[head] = append(edges[head], compiledRule{ground: &compiled})
			}
		} else if err == nil && tailSize >= 2 {
			var head ast.ASTHandle
			var uninterpreted int
			uninterpreted, err = rule.UninterpretedTailSize()
			if err == nil && uninterpreted == 1 {
				var compiled karrGuardRule
				head, compiled, err = a.compileGuardRule(rule)
				if err == nil {
					edges[head] = append(edges[head], compiledRule{guard: &compiled})
				}
			} else if err == nil && uninterpreted == 2 {
				var compiled karrBinaryRule
				head, compiled, err = a.compileBinaryRule(rule)
				if err == nil {
					edges[head] = append(edges[head], compiledRule{binary: &compiled})
				}
			} else if err == nil {
				err = ErrKarrDatalogFragment
			}
		} else if err == nil {
			var head ast.ASTHandle
			var compiled karrUnaryRule
			head, compiled, err = a.compileUnaryRule(rule)
			if err == nil {
				edges[head] = append(edges[head], compiledRule{unary: &compiled})
			}
		}
		err = errors.Join(err, rule.Close())
		if err != nil {
			return lbool.Undef, err
		}
	}
	snapshot := make(map[ast.ASTHandle]*rel.KarrRelation, len(a.seeds))
	for d, r := range a.seeds {
		snapshot[d] = r.Clone()
	}
	marks := make(map[ast.ASTHandle]uint8)
	var visit func(ast.ASTHandle) error
	visit = func(d ast.ASTHandle) error {
		if err := a.checkpoint(); err != nil {
			return err
		}
		if marks[d] == 1 {
			return ErrKarrDatalogFragment
		}
		if marks[d] == 2 {
			return nil
		}
		target := snapshot[d]
		if target == nil {
			return ErrKarrDatalogFragment
		}
		marks[d] = 1
		for _, rule := range edges[d] {
			var candidate *rel.KarrRelation
			var err error
			if rule.ground != nil {
				candidate, err = rule.ground.apply(func() bool { return a.checkpoint() == nil })
			} else if rule.guard != nil {
				if err := visit(rule.guard.body); err != nil {
					return err
				}
				candidate, err = a.applyGuardRule(*rule.guard, snapshot[rule.guard.body])
			} else if rule.binary != nil {
				if err := visit(rule.binary.left); err != nil {
					return err
				}
				if err := visit(rule.binary.right); err != nil {
					return err
				}
				candidate, err = a.applyBinaryRule(*rule.binary, snapshot[rule.binary.left], snapshot[rule.binary.right])
			} else {
				if err := visit(rule.unary.body); err != nil {
					return err
				}
				candidate, err = rule.unary.apply(snapshot[rule.unary.body])
			}
			if err != nil {
				return err
			}
			if err := a.checkpoint(); err != nil {
				return err
			}
			if _, err := rel.KarrSourceUnion(target, candidate); err != nil {
				return err
			}
			if err := a.checkpoint(); err != nil {
				return err
			}
		}
		marks[d] = 2
		return nil
	}
	for d := range snapshot {
		if err := visit(d); err != nil {
			return lbool.Undef, err
		}
	}
	status := lbool.False
	if len(declarations) == 0 {
		status = lbool.True
	}
	for _, d := range declarations {
		r := snapshot[d]
		if r == nil {
			return lbool.Undef, ErrKarrDatalogFragment
		}
		if !r.Empty() {
			status = lbool.Undef
		}
	}
	if err := a.checkpoint(); err != nil {
		return lbool.Undef, err
	}
	a.results = snapshot
	return status, nil
}
