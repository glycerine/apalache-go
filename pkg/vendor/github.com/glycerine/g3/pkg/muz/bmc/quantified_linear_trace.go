package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::get_model rule tracing.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

// QuantifiedLinearTrace follows model-selected rules from the query to a fact.
// The caller owns each returned rule reference.
func (c *Compiler) QuantifiedLinearTrace(rules *base.RuleSet, query ast.ASTHandle, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result []*base.Rule, resultErr error) {
	steps, err := c.quantifiedLinearSteps(rules, query, width, md, services)
	if err != nil {
		return nil, err
	}
	result = make([]*base.Rule, len(steps))
	for i := range steps {
		result[i] = steps[i].rule
	}
	return result, nil
}

type quantifiedLinearStep struct {
	rule  *base.Rule
	index uint32
	level uint32
}

// model::eval catches rewriter_exception, but not independent cleanup or
// non-rewriter failures joined with it.
func quantifiedLinearEvalException(err error) bool {
	if err == nil {
		return false
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		children := many.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !quantifiedLinearEvalException(child) {
				return false
			}
		}
		return true
	}
	var pending memory.ConstructionFailure
	return !errors.As(err, &pending) && rewriter.IsRewriterException(err)
}

// VERIFY reports a failed numeral check in every source build. After a
// continued debugger action, the C++ code uses an undefined bit-vector
// size; keep the defined Go boundary explicit instead of using that value.
func quantifiedLinearVerifyIndexFailure() error {
	err := diagnostic.BorrowProcessDebugContext().VerifyFailure(
		"src/muz/bmc/dl_bmc_engine.cpp", 262, "m_bv.is_numeral(vl, num, bv_size)")
	if err != nil {
		return err
	}
	return ast.ErrASTDomain
}

func (c *Compiler) quantifiedLinearSteps(rules *base.RuleSet, query ast.ASTHandle, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result []quantifiedLinearStep, resultErr error) {
	return c.quantifiedLinearStepsVisit(rules, query, width, md, services, nil)
}

func (c *Compiler) quantifiedLinearStepsVisit(rules *base.RuleSet, query ast.ASTHandle, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig, visit func(quantifiedLinearStep) error) (result []quantifiedLinearStep, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	owner := &quantifiedLinearTraceOwner{manager: m}
	defer func() {
		result, resultErr = finishQuantifiedLinearTrace(owner, resultErr)
	}()
	q, err := newQuantifiedLinear(m, width)
	owner.terms = q
	if err != nil {
		return nil, err
	}
	// model::eval constructs a fresh evaluator for every call and explicitly
	// disables completion, independently of model_evaluator parameters.
	evaluate := func(term ast.ASTHandle) (ast.ASTRef, error) {
		evaluator, err := model.NewModelEvaluator(md, services)
		owner.evaluator = evaluator
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err = evaluator.SetModelCompletion(false); err != nil {
			return ast.ASTRef{}, err
		}
		ref, err := evaluator.EvaluateValue(term)
		if err != nil {
			// model::eval catches rewriter_exception and leaves the result null.
			// A failed local cleanup must still escape with its retry owner.
			if quantifiedLinearEvalException(err) {
				if cleanup := evaluator.Close(); cleanup != nil {
					return ast.ASTRef{}, cleanup
				}
				owner.evaluator = nil
				return ast.ASTRef{}, nil
			}
			return ast.ASTRef{}, err
		}
		if err = evaluator.Close(); err != nil {
			return ast.ASTRef{}, disposeASTRefs(m, []ast.ASTRef{ref}, err)
		}
		owner.evaluator = nil
		return ref, nil
	}
	name, err := m.Symbols.Intern("T")
	if err != nil {
		return nil, err
	}
	index, err := q.hold(m.MkConst(name, q.index))
	if err != nil {
		return nil, err
	}
	indexRef, err := evaluate(index)
	if err != nil {
		return nil, err
	}
	if indexRef == (ast.ASTRef{}) {
		return nil, quantifiedLinearVerifyIndexFailure()
	}
	value, err := m.RefGet(indexRef)
	if err != nil {
		return nil, disposeASTRefs(m, []ast.ASTRef{indexRef}, err)
	}
	num, _, matched, err := m.BitVectorNumeralBigInt(value)
	err = disposeASTRefs(m, []ast.ASTRef{indexRef}, err)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, quantifiedLinearVerifyIndexFailure()
	}
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil {
		return nil, err
	}
	unsigned := num.IsUint64() && num.Uint64() <= uint64(^uint32(0))
	if !unsigned {
		if enabled {
			if err = diagnostic.DebugAssertionFailure(debug, "src/muz/bmc/dl_bmc_engine.cpp", 263, "num.is_unsigned()"); err != nil {
				return nil, err
			}
		}
		// Continuing would invoke C++ get_unsigned outside its source domain.
		return nil, ast.ErrASTDomain
	}
	level := uint32(num.Uint64())
	// Source qlinear::get_model checks q(T) only in an enabled SASSERT.
	if enabled {
		levelExpr, e := q.hold(m.BitVectorNumeralRat(new(big.Rat).SetUint64(uint64(level)), width))
		if e != nil {
			return nil, e
		}
		queryAtLevel, e := q.predicate(query, levelExpr)
		if e != nil {
			return nil, e
		}
		checked, e := evaluate(queryAtLevel)
		if e != nil {
			return nil, e
		}
		valid := false
		if checked != (ast.ASTRef{}) {
			var h ast.ASTHandle
			h, e = m.RefGet(checked)
			if e == nil {
				valid = m.IsTrue(h)
			}
			e = disposeASTRefs(m, []ast.ASTRef{checked}, e)
			if e != nil {
				return nil, e
			}
		}
		if !valid {
			if e = diagnostic.DebugAssertionFailure(debug, "src/muz/bmc/dl_bmc_engine.cpp", 265, "m.is_true(eval_q(md, b.m_query_pred, level))"); e != nil {
				return nil, e
			}
		}
	}
	predicate := query
	for {
		canceled, e := m.Canceled()
		if e != nil {
			return result, e
		}
		if canceled {
			return result, fmt.Errorf("bmc: %s", m.ResourceLimit().CancelMessage())
		}
		levelExpr, e := q.hold(m.BitVectorNumeralRat(new(big.Rat).SetUint64(uint64(level)), width))
		if e != nil {
			return result, e
		}
		owner.group, e = rules.PredicateRules(predicate)
		if e != nil {
			return result, e
		}
		var selectedIndex uint32
		for i, rule := range owner.group {
			atom, cause := q.rule(predicate, uint32(i))
			if cause == nil {
				var decl ast.ASTHandle
				decl, cause = m.GetDecl(atom)
				if cause == nil {
					atom, cause = q.hold(m.MkApp(decl, []ast.ASTHandle{levelExpr}))
				}
			}
			if cause == nil {
				var evaluated ast.ASTRef
				evaluated, cause = evaluate(atom)
				if cause == nil && evaluated != (ast.ASTRef{}) {
					var h ast.ASTHandle
					h, cause = m.RefGet(evaluated)
					if cause == nil && m.IsTrue(h) {
						owner.selected, cause = rule.Retain()
						selectedIndex = uint32(i)
					}
					cause = disposeASTRefs(m, []ast.ASTRef{evaluated}, cause)
				}
			}
			if cause != nil {
				e = cause
				break
			}
			if owner.selected != nil {
				break
			}
		}
		if e != nil {
			return result, e
		}
		if e = owner.closeGroup(); e != nil {
			return result, e
		}
		if owner.selected == nil {
			if enabled {
				if e = diagnostic.DebugAssertionFailure(debug, "src/muz/bmc/dl_bmc_engine.cpp", 283, "r"); e != nil {
					return result, e
				}
			}
			return result, fmt.Errorf("bmc: model has no selected quantified rule at level %d", level)
		}
		selected := owner.selected
		owner.steps = append(owner.steps, quantifiedLinearStep{rule: selected, index: selectedIndex, level: level})
		owner.selected = nil
		if visit != nil {
			if e = visit(owner.steps[len(owner.steps)-1]); e != nil {
				return result, e
			}
		}
		tails, e := selected.UninterpretedTailSize()
		if e != nil {
			return result, e
		}
		if tails == 0 {
			return result, nil
		}
		// Corrected C++ accepts a fact at any level, but verifies in every
		// build that a rule with a predecessor can decrease the level.
		if level == 0 {
			if e = debug.VerifyFailure("src/muz/bmc/dl_bmc_engine.cpp", 355, "level > 0"); e != nil {
				return result, e
			}
			return result, ast.ErrASTDomain
		}
		if tails != 1 {
			return result, ast.ErrASTDomain
		}
		predicate, e = selected.TailDeclaration(0)
		if e != nil {
			return result, e
		}
		level--
	}
}
