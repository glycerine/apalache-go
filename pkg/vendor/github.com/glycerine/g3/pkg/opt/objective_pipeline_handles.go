package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ObjectivePipeline) refHandles(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		var err error
		result[i], err = p.manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func pipelineWeightPointers(weights []big.Rat) []*big.Rat {
	result := make([]*big.Rat, len(weights))
	for i := range weights {
		result[i] = new(big.Rat).Set(&weights[i])
	}
	return result
}

func (o *pipelineObjective) restore(value *big.Rat) big.Rat {
	var result big.Rat
	if value != nil {
		result.Set(value)
	}
	if o.negate {
		result.Neg(&result)
	}
	result.Add(&result, &o.offset)
	return result
}
