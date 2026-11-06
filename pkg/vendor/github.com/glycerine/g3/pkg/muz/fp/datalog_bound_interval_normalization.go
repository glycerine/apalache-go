package fp

// Source: rel_context::transform_rules runs mk_filter_rules (2000) before
// mk_simple_joins (1000); scoped_query restores the original program and
// predicates after execution. The binary compiler keeps its two-tail limit.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

type boundIntervalNormalization struct {
	source      *boundIntervalDatalogActivation
	rules       *base.RuleManager
	set         *base.RuleSet
	transformer *base.RuleTransformer
	activation  *boundIntervalDatalogActivation
	nested      memory.ConstructionFailure
}

func (s *boundIntervalNormalization) Close() error {
	if s == nil {
		return nil
	}
	if s.nested != nil {
		if err := s.nested.Cleanup(); err != nil {
			return err
		}
		s.nested = nil
	}
	if s.activation != nil {
		if err := s.activation.close(); err != nil {
			return err
		}
		s.activation = nil
	}
	if s.transformer != nil {
		if err := s.transformer.Close(); err != nil {
			return err
		}
		s.transformer = nil
	}
	if s.set != nil {
		if err := s.set.Close(); err != nil {
			return err
		}
		s.set = nil
	}
	if s.rules != nil {
		if err := s.rules.Close(); err != nil {
			return err
		}
		s.rules = nil
	}
	return nil
}

type boundIntervalNormalizationFailure struct {
	original error
	cleanup  error
	reported error
	text     string
	scope    *boundIntervalNormalization
}

func (f *boundIntervalNormalizationFailure) Error() string        { return f.text }
func (f *boundIntervalNormalizationFailure) Unwrap() error        { return f.reported }
func (f *boundIntervalNormalizationFailure) OriginalCause() error { return f.original }
func (f *boundIntervalNormalizationFailure) CleanupCause() error  { return f.cleanup }
func (f *boundIntervalNormalizationFailure) RuntimeOwner() *memory.Runtime {
	return f.scope.source.manager.NumeralService().RuntimeOwner()
}
func (f *boundIntervalNormalizationFailure) Cleanup() error {
	f.cleanup = f.scope.Close()
	return f.cleanup
}

func finishBoundIntervalNormalization(cause error, scope *boundIntervalNormalization) error {
	if cause != nil {
		errors.As(cause, &scope.nested)
	}
	cleanup := scope.Close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &boundIntervalNormalizationFailure{original: original, cleanup: cleanup, reported: reported, text: reported.Error(), scope: scope}
}

var _ memory.ConstructionFailure = (*boundIntervalNormalizationFailure)(nil)

func (a *boundIntervalDatalogActivation) needsJoinNormalization() (bool, error) {
	if err := a.open(); err != nil {
		return false, err
	}
	count, err := a.rules.Len()
	if err != nil {
		return false, err
	}
	for i := 0; i < count; i++ {
		rule, err := a.rules.Rule(i)
		if err != nil {
			return false, err
		}
		positive, err := rule.PositiveTailSize()
		if err = errors.Join(err, rule.Close()); err != nil {
			return false, err
		}
		if positive > 2 {
			return true, nil
		}
	}
	return false, nil
}

func prepareBoundIntervalNormalization(source *boundIntervalDatalogActivation, outputs []ast.ASTHandle) (_ *boundIntervalNormalization, resultErr error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	scope := &boundIntervalNormalization{source: source}
	defer func() {
		if resultErr != nil {
			resultErr = finishBoundIntervalNormalization(resultErr, scope)
		}
	}()
	var err error
	scope.rules, err = base.NewRuleManager(source.manager)
	if err != nil {
		return nil, err
	}
	for _, declaration := range source.order {
		if err = scope.rules.RegisterPredicate(declaration); err != nil {
			return nil, err
		}
	}
	scope.set, err = base.NewRuleSet(scope.rules)
	if err != nil {
		return nil, err
	}
	count, err := source.rules.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		if err = source.checkpoint(); err != nil {
			return nil, err
		}
		rule, err := source.rules.Rule(i)
		if err != nil {
			return nil, err
		}
		// Add permits the same AST manager with a separate RuleManager;
		// AddRules intentionally requires the very same RuleManager.
		err = errors.Join(scope.set.Add(rule), rule.Close())
		if err != nil {
			return nil, err
		}
	}
	for _, declaration := range outputs {
		if err = scope.set.SetOutputPredicate(declaration); err != nil {
			return nil, err
		}
	}
	valid, err := scope.set.Finalize()
	if err != nil || !valid {
		return nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	scope.transformer = base.NewRuleTransformer()
	filter := transforms.NewFilterRulesPlugin(source.manager, scope.rules, 2000)
	if err = scope.transformer.Register(filter); err != nil {
		return nil, errors.Join(err, filter.Close())
	}
	joins := transforms.NewSimpleJoinsPlugin(source.manager, scope.rules, 1000)
	if err = scope.transformer.Register(joins); err != nil {
		return nil, errors.Join(err, joins.Close())
	}
	if _, err = scope.transformer.Apply(scope.set); err != nil {
		return nil, err
	}
	if err = source.checkpoint(); err != nil {
		return nil, err
	}
	scope.activation, err = source.clone(scope.set)
	if err != nil {
		return nil, err
	}
	predicates := make([]boundIntervalDatalogPredicate, 0, len(source.order))
	kinds := make(map[ast.ASTHandle][]string)
	for _, declaration := range source.order {
		kinds[declaration] = source.entries[declaration].kinds
		predicates = append(predicates, boundIntervalDatalogPredicate{declaration: declaration, kinds: kinds[declaration]})
	}
	// Source fresh filter heads inherit their tail's request. Split heads
	// inherit the last registered consumer's head request, in creation order.
	for _, declaration := range filter.FreshPredicates() {
		origin, found := filter.FreshPredicateOrigin(declaration)
		if !found || !isBoundIntervalRepresentation(kinds[origin]) {
			return nil, ErrBoundIntervalDatalogFragment
		}
		kinds[declaration] = append([]string(nil), kinds[origin]...)
		predicates = append(predicates, boundIntervalDatalogPredicate{declaration: declaration, kinds: kinds[declaration]})
	}
	for _, declaration := range joins.FreshPredicates() {
		origin, found := joins.FreshPredicateOrigin(declaration)
		if !found || !isBoundIntervalRepresentation(kinds[origin]) {
			return nil, ErrBoundIntervalDatalogFragment
		}
		kinds[declaration] = append([]string(nil), kinds[origin]...)
		predicates = append(predicates, boundIntervalDatalogPredicate{declaration: declaration, kinds: kinds[declaration]})
	}
	if err = scope.activation.syncPredicates(predicates); err != nil {
		return nil, err
	}
	return scope, nil
}

// complete publishes no auxiliary relations, and no result at all when any
// owner refuses cleanup. Product values contain no borrowed AST references.
func (s *boundIntervalNormalization) complete(status lbool.LBool, cause error) (lbool.LBool, error) {
	var results map[ast.ASTHandle]*rel.IntervalBoundProduct
	if cause == nil {
		results = make(map[ast.ASTHandle]*rel.IntervalBoundProduct, len(s.source.order))
		for _, declaration := range s.source.order {
			product := s.activation.evaluated[declaration]
			if product == nil {
				cause = ErrBoundIntervalDatalogFragment
				break
			}
			results[declaration] = product
		}
	}
	if err := finishBoundIntervalNormalization(cause, s); err != nil {
		return lbool.Undef, err
	}
	s.source.evaluated = results
	return status, nil
}

func (a *boundIntervalDatalogActivation) queryNormalized(declarations []ast.ASTHandle, widen, allOrNothing bool) (lbool.LBool, error) {
	a.evaluated = nil
	if err := a.checkpoint(); err != nil {
		return lbool.Undef, err
	}
	scope, err := prepareBoundIntervalNormalization(a, declarations)
	if err != nil {
		return lbool.Undef, err
	}
	status, err := scope.activation.queryWithWidening(declarations, widen, allOrNothing)
	return scope.complete(status, err)
}
