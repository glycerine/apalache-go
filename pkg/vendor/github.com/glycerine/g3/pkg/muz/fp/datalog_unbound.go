package fp

// Source: rel_context.cpp transform_rules/setup_default_relation and scoped_query.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type datalogUnboundScope struct {
	engine     *PropositionalEngine
	set        *base.RuleSet
	predicates map[ast.ASTHandle]bool
}

// A refused native cleanup keeps its concrete owner reachable through the
// standard memory.ConstructionFailure cleanup contract.
type datalogUnboundCleanupFailure struct {
	cause error
	scope *datalogUnboundScope
}

func (f *datalogUnboundCleanupFailure) Error() string  { return f.cause.Error() }
func (f *datalogUnboundCleanupFailure) Unwrap() error  { return f.cause }
func (f *datalogUnboundCleanupFailure) Cleanup() error { return f.scope.Close() }

func (s *datalogUnboundScope) dispose() error {
	if err := s.Close(); err != nil {
		return &datalogUnboundCleanupFailure{cause: err, scope: s}
	}
	return nil
}

func (s *datalogUnboundScope) Close() error {
	if s == nil {
		return nil
	}
	if err := s.set.Close(); err != nil {
		return err
	}
	if err := s.engine.relations.RestrictPredicates(s.predicates); err != nil {
		return err
	}
	for decl := range s.engine.predicates {
		if !s.predicates[decl] {
			delete(s.engine.predicates, decl)
			delete(s.engine.auxiliary, decl)
		}
	}
	order := s.engine.predicateOrder[:0]
	for _, decl := range s.engine.predicateOrder {
		if s.predicates[decl] {
			order = append(order, decl)
		}
	}
	s.engine.predicateOrder = order
	return nil
}

// prepareDatalogUnbound returns a query-owned transformed set, or nil when no
// compression is needed. Keeping the original set is essential: a later query
// may request a predicate whose unbound columns this query can project away.
func (e *PropositionalEngine) prepareDatalogUnbound(outputs []ast.ASTHandle) (_ *datalogUnboundScope, resultErr error) {
	enabled := true
	compress := true
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return nil, err
		}
		enabled, err = view.Bool("datalog.unbound_compressor", true)
		if err == nil && enabled {
			// The outer registration switch and the transform's own gate
			// are distinct source parameters, with the same local precedence.
			compress, err = view.Bool("xform.compress_unbound", true)
		}
		if err == nil && enabled {
			relation, cause := view.Symbol("datalog.default_relation", symbol.Symbol{})
			err = cause
			if err == nil && relation.Value != 0 {
				name, _, cause := e.manager.Symbols.BareString(relation)
				err = cause
				enabled = name != "doc"
			}
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return nil, err
		}
	}
	if !enabled {
		return nil, nil
	}
	checkRelation, checking, err := e.datalogCheckRelationName()
	if err != nil {
		return nil, err
	}
	if checking && checkRelation == "doc" {
		return nil, nil
	}
	scoped, err := base.NewRuleSet(e.rules)
	if err != nil {
		return nil, err
	}
	scope := &datalogUnboundScope{engine: e, set: scoped, predicates: make(map[ast.ASTHandle]bool, len(e.predicates))}
	for decl := range e.predicates {
		scope.predicates[decl] = true
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, scope.dispose())
		}
	}()
	if err = scoped.AddRules(e.set); err != nil {
		return nil, err
	}
	for _, decl := range outputs {
		if err = scoped.SetOutputPredicate(decl); err != nil {
			return nil, err
		}
	}
	valid, err := scoped.Finalize()
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, fmt.Errorf("muz/fp: negation is not stratified")
	}
	plugin := transforms.NewUnboundCompressorPlugin(e.manager, e.rules, compress, 0)
	transformer := base.NewRuleTransformer()
	if err = transformer.Register(plugin); err != nil {
		return nil, errors.Join(err, plugin.Close(), transformer.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, transformer.Close()) }()
	modified, err := transformer.Apply(scoped)
	if err != nil || !modified {
		return nil, err
	}
	// Scan the resulting program rather than the plugin's one-value-per-source
	// summary: different rules can compress different columns of one predicate.
	count, err := scoped.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, cause := scoped.Rule(i)
		if cause != nil {
			return nil, cause
		}
		head, cause := rule.HeadDeclaration()
		declarations := []ast.ASTHandle{head}
		tails := 0
		if cause == nil {
			tails, cause = rule.UninterpretedTailSize()
		}
		for j := 0; cause == nil && j < tails; j++ {
			var decl ast.ASTHandle
			decl, cause = rule.TailDeclaration(j)
			declarations = append(declarations, decl)
		}
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return nil, cause
		}
		for _, decl := range declarations {
			if e.predicates[decl] != nil {
				continue
			}
			state, cause := e.predicateState(decl)
			if cause != nil {
				return nil, cause
			}
			e.predicates[decl] = state
			e.predicateOrder = append(e.predicateOrder, decl)
			e.auxiliary[decl] = true
		}
	}
	published = true
	return scope, nil
}
