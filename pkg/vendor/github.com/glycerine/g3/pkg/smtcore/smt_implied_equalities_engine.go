package smtcore

import "errors"

func (e *smtImpliedEqualityEngine) run() (result error) {
	source, modelBased := e.solver.(smtImpliedEqualityEvaluatorSource)
	var evaluator smtImpliedEqualityEvaluator
	if modelBased {
		var err error
		evaluator, err = source.newImpliedEqualityEvaluator()
		if err != nil {
			return err
		}
		modelBased = evaluator != nil
		if modelBased {
			defer func() { result = errors.Join(result, evaluator.Close()) }()
		}
	}
	for sort, indices := range e.bySort {
		if !modelBased || e.isArraySort(sort) {
			e.stats.FallbackTerms += uint32(len(indices))
			if err := e.exact(indices); err != nil {
				return err
			}
			continue
		}
		if err := e.modelPartition(indices, evaluator); err != nil {
			return err
		}
	}
	return nil
}

func (e *smtImpliedEqualityEngine) results() []uint32 {
	result := make([]uint32, len(e.terms))
	for index := range result {
		result[index] = e.classes.find(uint32(index))
	}
	return result
}
