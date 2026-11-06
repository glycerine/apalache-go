package fp

// Source: rel_context saturation through dl_compiler fact and single-tail
// rules using dl_external_relation operations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) validateExternalRulePredicates() (resultErr error) {
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	for index := 0; index < count; index++ {
		rule, cause := e.set.Rule(index)
		if cause != nil {
			return cause
		}
		declarations := make([]ast.ASTHandle, 0, 2)
		head, cause := rule.HeadDeclaration()
		if cause == nil {
			declarations = append(declarations, head)
		}
		uninterpreted := 0
		if cause == nil {
			uninterpreted, cause = rule.UninterpretedTailSize()
		}
		for tail := 0; cause == nil && tail < uninterpreted; tail++ {
			declaration, tailErr := rule.TailDeclaration(tail)
			cause = errors.Join(cause, tailErr)
			declarations = append(declarations, declaration)
		}
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return cause
		}
		for _, declaration := range declarations {
			state := e.predicates[declaration]
			if state == nil || state.external == nil {
				return ErrPropositionalHorn
			}
		}
	}
	return nil
}

func (e *PropositionalEngine) queryExternalDatalogPredicates(declarations []ast.ASTHandle, ruleCount int) (status lbool.LBool, resultErr error) {
	if e.external == nil {
		return lbool.Undef, ErrPropositionalHorn
	}
	if err := e.discardExternalSimilarity(); err != nil {
		return lbool.Undef, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, e.discardExternalSimilarity())
		if resultErr != nil {
			status = lbool.Undef
		}
	}()
	if ruleCount != 0 {
		if err := e.validateExternalRulePredicates(); err != nil {
			return lbool.Undef, err
		}
		options := rel.ExternalRuleExecutionOptions{}
		similarity, threshold := true, uint32(11)
		if e.params != nil && e.globals != nil {
			view, err := e.fixedpointParamView()
			if err != nil {
				return lbool.Undef, err
			}
			options.CompileWithWidening, err = view.Bool("datalog.compile_with_widening", false)
			if err == nil {
				options.AllOrNothingDeltas, err = view.Bool("datalog.all_or_nothing_deltas", false)
			}
			if err == nil {
				similarity, err = view.Bool("datalog.similarity_compressor", true)
			}
			if err == nil && similarity {
				threshold, err = view.UInt("datalog.similarity_compressor_threshold", 11)
			}
			if err = errors.Join(err, view.Close()); err != nil {
				return lbool.Undef, err
			}
		}
		rules := e.set
		if similarity {
			var err error
			rules, err = e.prepareExternalSimilarity(declarations, threshold)
			if err != nil {
				return lbool.Undef, err
			}
		}
		if err := rel.ExecuteExternalFactRulesWithOptions(e.external, rules, options); err != nil {
			return lbool.Undef, err
		}
	}
	e.compiled = false
	e.datalogStats.Reset()
	for _, declaration := range declarations {
		state := e.predicates[declaration]
		if state.external != nil {
			empty, err := state.external.Empty()
			if err != nil {
				return lbool.Undef, err
			}
			if !empty {
				return lbool.True, nil
			}
			continue
		}
		if state.table != nil {
			rows, err := state.table.Len()
			if err != nil {
				return lbool.Undef, err
			}
			if rows != 0 {
				return lbool.True, nil
			}
		}
	}
	return lbool.FromBool(len(declarations) == 0), nil
}
