package fp

import "errors"

// Compile only scopes whose actual manager owners and temporary constructors
// carry the selected concrete checked pair. Formula relation checking keeps
// its existing interpreted evaluator.
func (e *PropositionalEngine) saturateDatalogQuery(check *datalogCheckRelationScope, round *datalogRestartRound) (resultErr error) {
	compile := check == nil
	if check != nil {
		owner, supported, err := check.beginCompiled()
		if err != nil {
			return err
		}
		compile = supported
		if owner != nil {
			defer func() { resultErr = errors.Join(resultErr, owner.Close()) }()
		}
	}
	if !compile {
		e.compiled = false
		return e.saturate()
	}
	var err error
	e.compiled, err = e.tryCompiledSaturation(round)
	if err == nil && !e.compiled {
		err = e.saturate()
	}
	return err
}
