package smtcore

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *smtImpliedEqualityEngine) modelPartition(indices []uint32, evaluator smtImpliedEqualityEvaluator) (result error) {
	buckets := make(map[ast.ASTHandle][]uint32)
	nonValues := make([]uint32, 0)
	refs := make([]ast.ASTRef, 0, len(indices))
	defer func() {
		for i := range refs {
			result = errors.Join(result, e.manager.DelRef(&refs[i]))
		}
	}()
	for _, index := range indices {
		ref, err := evaluator.EvaluateValue(e.terms[index])
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		value, err := e.manager.RefGet(ref)
		if err != nil {
			return err
		}
		isValue, err := e.manager.IsValue(value)
		if err != nil {
			return err
		}
		e.stats.ValueEvaluations++
		if !isValue {
			nonValues = append(nonValues, index)
			continue
		}
		buckets[value] = append(buckets[value], index)
	}
	e.stats.ValueBuckets += uint32(len(buckets))
	for _, bucket := range buckets {
		if err := e.exact(bucket); err != nil {
			return err
		}
	}
	if len(nonValues) != 0 {
		e.stats.FallbackTerms += uint32(len(nonValues))
		return e.exact(indices)
	}
	return nil
}
