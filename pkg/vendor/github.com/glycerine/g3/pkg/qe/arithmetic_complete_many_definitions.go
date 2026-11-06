package qe

import "errors"

func (r *ArithmeticCompleteProjectionResult) ToDefinitionVector(config Config) (result *DefinitionVector, resultErr error) {
	if e := r.open(); e != nil {
		return nil, e
	}
	result, e := NewDefinitionVector(r.manager, config)
	if e != nil {
		return nil, e
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if e = r.validateDefinitionTarget(result); e != nil {
		return nil, e
	}
	if e = r.appendDefinitions(result); e != nil {
		return nil, e
	}
	return result, nil
}
