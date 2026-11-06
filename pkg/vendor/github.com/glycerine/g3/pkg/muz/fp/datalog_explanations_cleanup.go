package fp

import (
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// Keep concrete query-local owners reachable if native destruction is refused.
// The engine retries them before disposing its borrowed rule manager.
type datalogExplanationOwners struct {
	transformer            *rel.ExplanationTransformer
	set                    *base.RuleSet
	pass                   *base.RuleTransformer
	execution, termination *rel.InstructionBlock
	context                *rel.ExecutionContext
}

func (o *datalogExplanationOwners) Close() error {
	if o.context != nil {
		if err := o.context.Close(); err != nil {
			return err
		}
		o.context = nil
	}
	if o.execution != nil {
		if err := o.execution.Close(); err != nil {
			return err
		}
		o.execution = nil
	}
	if o.termination != nil {
		if err := o.termination.Close(); err != nil {
			return err
		}
		o.termination = nil
	}
	if o.pass != nil {
		if err := o.pass.Close(); err != nil {
			return err
		}
		o.pass = nil
	}
	if o.set != nil {
		if err := o.set.Close(); err != nil {
			return err
		}
		o.set = nil
	}
	if o.transformer != nil {
		if err := o.transformer.Close(); err != nil {
			return err
		}
		o.transformer = nil
	}
	return nil
}

func (e *PropositionalEngine) clearDatalogExplanationCleanup() error {
	for len(e.datalogExplanationCleanup) != 0 {
		owners := e.datalogExplanationCleanup[0]
		if err := owners.Close(); err != nil {
			return err
		}
		e.datalogExplanationCleanup[0] = nil
		e.datalogExplanationCleanup = e.datalogExplanationCleanup[1:]
	}
	e.datalogExplanationCleanup = nil
	return nil
}
