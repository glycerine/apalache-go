package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *ObjectiveDecomposer) convertBitVector(source DecodedFormulaObjective, index int) (DecomposedObjective, bool, error) {
	isBV, err := d.manager.IsBitVector(source.Term)
	if err != nil || !isBV {
		return DecomposedObjective{}, false, err
	}
	width, err := d.manager.BitVectorSize(source.Term)
	if err != nil {
		return DecomposedObjective{}, false, err
	}
	result := DecomposedObjective{SourceKind: source.Kind, OriginalTerm: source.OriginalTerm, ID: decompositionID(source, index), Converted: true, Negate: source.Kind == FormulaMaximize}
	result.SoftTerms = make([]ast.ASTHandle, 0, width)
	result.Weights = make([]big.Rat, 0, width)
	bitValue := uint32(0)
	if source.Kind == FormulaMaximize {
		bitValue = 1
	}
	value, err := d.manager.BitVectorNumeralUnsigned(bitValue, 1)
	if err != nil {
		return DecomposedObjective{}, false, err
	}
	weight := new(big.Int).SetInt64(1)
	for bit := uint32(0); bit < width; bit++ {
		extract, e := d.manager.MkBitVectorExtract(bit, bit, source.Term)
		if e != nil {
			return DecomposedObjective{}, false, e
		}
		extract, e = d.retain(extract)
		if e != nil {
			return DecomposedObjective{}, false, e
		}
		eq, e := d.manager.MkEq(value, extract)
		if e != nil {
			return DecomposedObjective{}, false, e
		}
		eq, e = d.retain(eq)
		if e != nil {
			return DecomposedObjective{}, false, e
		}
		result.SoftTerms = append(result.SoftTerms, eq)
		var rational big.Rat
		rational.SetInt(weight)
		result.Weights = append(result.Weights, rational)
		if source.Kind == FormulaMaximize {
			result.Offset.Add(&result.Offset, &rational)
		}
		weight.Lsh(weight, 1)
	}
	return result, true, nil
}
