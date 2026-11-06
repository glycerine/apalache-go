package fp

// Source: rel_context rule transformation before dl_compiler compilation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalDatalogNormalization struct {
	source      *externalIntervalDatalogActivation
	rules       *base.RuleManager
	set         *base.RuleSet
	transformer *base.RuleTransformer
	activation  *externalIntervalDatalogActivation
}

func (s *externalIntervalDatalogNormalization) Close() error {
	if s == nil {
		return nil
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

type externalIntervalDatalogNormalizationFailure struct {
	original, cleanup, reported error
	text                        string
	scope                       *externalIntervalDatalogNormalization
}

func (f *externalIntervalDatalogNormalizationFailure) Error() string        { return f.text }
func (f *externalIntervalDatalogNormalizationFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalDatalogNormalizationFailure) OriginalCause() error { return f.original }
func (f *externalIntervalDatalogNormalizationFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalDatalogNormalizationFailure) RuntimeOwner() *memory.Runtime {
	return f.scope.source.manager.NumeralService().RuntimeOwner()
}
func (f *externalIntervalDatalogNormalizationFailure) Cleanup() error {
	f.cleanup = f.scope.Close()
	return f.cleanup
}

var _ memory.ConstructionFailure = (*externalIntervalDatalogNormalizationFailure)(nil)

func finishExternalIntervalDatalogNormalization(cause error, scope *externalIntervalDatalogNormalization) error {
	cleanup := scope.Close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalDatalogNormalizationFailure{
		original: original, cleanup: cleanup, reported: reported,
		text: reported.Error(), scope: scope,
	}
}

func externalIntervalNeedsJoinNormalization(set *base.RuleSet) (bool, error) {
	if set == nil {
		return false, ErrExternalIntervalDatalogFragment
	}
	count, err := set.Len()
	if err != nil {
		return false, err
	}
	for index := 0; index < count; index++ {
		rule, err := set.Rule(index)
		if err != nil {
			return false, err
		}
		positive, cause := rule.PositiveTailSize()
		if cause = errors.Join(cause, rule.Close()); cause != nil {
			return false, cause
		}
		if positive > 2 {
			return true, nil
		}
	}
	return false, nil
}

func prepareExternalIntervalDatalogNormalization(source *externalIntervalDatalogActivation, original *base.RuleSet, outputs []ast.ASTHandle) (_ *externalIntervalDatalogNormalization, resultErr error) {
	if err := source.open(); err != nil || original == nil || source.program != nil || source.pending != nil {
		return nil, errors.Join(err, ErrExternalIntervalDatalogFragment)
	}
	scope := &externalIntervalDatalogNormalization{source: source}
	defer func() {
		if resultErr != nil {
			resultErr = finishExternalIntervalDatalogNormalization(resultErr, scope)
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
	count, err := original.Len()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		rule, cause := original.Rule(index)
		if cause != nil {
			return nil, cause
		}
		cause = errors.Join(scope.set.Add(rule), rule.Close())
		if cause != nil {
			return nil, cause
		}
	}
	for _, declaration := range outputs {
		if err = scope.set.SetOutputPredicate(declaration); err != nil {
			return nil, err
		}
	}
	valid, err := scope.set.Finalize()
	if err != nil || !valid {
		return nil, errors.Join(err, ErrExternalIntervalDatalogFragment)
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

	compiler := &externalIntervalDatalogCompiler{
		manager:        source.manager,
		set:            scope.set,
		predicates:     make(map[ast.ASTHandle]*externalIntervalDatalogPredicate, len(source.order)+len(filter.FreshPredicates())+len(joins.FreshPredicates())),
		queryAuxiliary: make(map[ast.ASTHandle]bool),
	}
	known := make(map[ast.ASTHandle][]string, len(compiler.predicates))
	for _, declaration := range source.order {
		entry := source.entries[declaration]
		if entry == nil {
			return nil, ErrExternalIntervalDatalogFragment
		}
		kinds := append([]string(nil), entry.kinds...)
		sorts := append([]ast.ASTHandle(nil), entry.sorts...)
		compiler.predicates[declaration] = &externalIntervalDatalogPredicate{declaration: declaration, sorts: sorts, kinds: kinds}
		known[declaration] = kinds
	}
	helpers := make([]externalIntervalDatalogPredicate, 0, len(filter.FreshPredicates())+len(joins.FreshPredicates()))
	addHelpers := func(declarations []ast.ASTHandle, origin func(ast.ASTHandle) (ast.ASTHandle, bool)) error {
		for _, declaration := range declarations {
			from, found := origin(declaration)
			kinds := known[from]
			if !found || len(kinds) != 2 || !isExternalIntervalRepresentation(kinds) {
				return ErrExternalIntervalDatalogFragment
			}
			arity, err := source.manager.GetArity(declaration)
			if err != nil {
				return err
			}
			sorts := make([]ast.ASTHandle, arity)
			for column := uint32(0); column < arity; column++ {
				sorts[column], err = source.manager.GetDomain(declaration, column)
				if err != nil {
					return err
				}
			}
			metadata := &externalIntervalDatalogPredicate{
				declaration: declaration,
				sorts:       sorts,
				kinds:       append([]string(nil), kinds...),
			}
			compiler.predicates[declaration] = metadata
			compiler.queryAuxiliary[declaration] = false
			known[declaration] = metadata.kinds
			helpers = append(helpers, *metadata)
		}
		return nil
	}
	if err = addHelpers(filter.FreshPredicates(), filter.FreshPredicateOrigin); err != nil {
		return nil, err
	}
	if err = addHelpers(joins.FreshPredicates(), joins.FreshPredicateOrigin); err != nil {
		return nil, err
	}
	program, err := compiler.externalIntervalProgramPlan()
	if err != nil || program == nil {
		return nil, errors.Join(err, ErrExternalIntervalDatalogFragment)
	}
	scope.activation, err = newExternalIntervalDatalogScope(source, helpers, program)
	if err != nil {
		return nil, err
	}
	return scope, nil
}

func (s *externalIntervalDatalogNormalization) query(outputs []ast.ASTHandle) (lbool.LBool, error) {
	if s == nil || s.activation == nil {
		return lbool.Undef, ErrExternalIntervalDatalogFragment
	}
	if err := s.activation.executeExternalIntervalProgram(); err != nil {
		return lbool.Undef, finishExternalIntervalDatalogNormalization(err, s)
	}
	status, err := s.activation.query(outputs)
	if err = finishExternalIntervalDatalogNormalization(err, s); err != nil {
		return lbool.Undef, err
	}
	return status, nil
}
