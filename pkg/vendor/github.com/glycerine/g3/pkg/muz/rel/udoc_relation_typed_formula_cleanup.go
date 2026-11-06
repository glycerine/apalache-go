package rel

// Source: udoc_relation::to_formula's expr_ref vectors and scoped rational.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type typedUDocFormulaOwner struct {
	manager  *ast.ASTManager
	roots    []ast.ASTRef
	numbers  []*numeral.OwnedRational
	calls    []*numeral.RationalServiceInvocation
	result   ast.ASTRef
	nested   memory.ConstructionFailure
	relation *TypedUDocRelation
}

func (o *typedUDocFormulaOwner) hold(value ast.ASTHandle, cause error) (ast.ASTHandle, error) {
	if cause != nil {
		return ast.ASTHandle{}, fmt.Errorf("UDoc formula temporary: %w", cause)
	}
	ref, err := o.manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("UDoc formula temporary reference: %w", err)
	}
	o.roots = append(o.roots, ref)
	return value, nil
}

func (o *typedUDocFormulaOwner) closeNumbers() error {
	for index := len(o.calls) - 1; index >= 0; index-- {
		if err := o.calls[index].Cleanup(false); err != nil {
			return err
		}
	}
	for len(o.numbers) != 0 {
		last := len(o.numbers) - 1
		value := o.numbers[last]
		if *value == (numeral.OwnedRational{}) {
			o.numbers = o.numbers[:last]
			continue
		}
		err := o.manager.NumeralService().CleanupValue(value)
		if *value == (numeral.OwnedRational{}) {
			o.numbers = o.numbers[:last]
		}
		if err != nil {
			return err
		}
	}
	for len(o.calls) != 0 {
		last := len(o.calls) - 1
		if err := o.calls[last].Cleanup(true); err != nil {
			return err
		}
		if err := o.calls[last].EndContext(); err != nil {
			return err
		}
		o.calls = o.calls[:last]
	}
	return nil
}

func (o *typedUDocFormulaOwner) bitVectorNumeral(number *big.Int, width uint32) (ast.ASTHandle, error) {
	service := o.manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Pointer ownership avoids copying a live native numeral token.
	o.numbers = append(o.numbers, &value)
	// Source builds each fixed range by adding rational::power_of_two for
	// its set bits. Capture each reached numerical scope instead of using
	// a text-parser bridge whose interrupted state outlives this expression.
	for bit := 0; bit < number.BitLen(); bit++ {
		if number.Bit(bit) == 0 {
			continue
		}
		powerCall := &numeral.RationalServiceInvocation{}
		o.calls = append(o.calls, powerCall)
		power, cause := service.PowerOfTwoWithInvocation(uint32(bit), powerCall)
		o.numbers = append(o.numbers, &power)
		if cause != nil {
			return ast.ASTHandle{}, fmt.Errorf("UDoc formula power %d: %w", bit, cause)
		}
		if cause = powerCall.SourceReady(); cause != nil {
			return ast.ASTHandle{}, fmt.Errorf("UDoc formula power %d completion: %w", bit, cause)
		}
		if cause = o.finishNumberCall(); cause != nil {
			return ast.ASTHandle{}, cause
		}
		addCall := &numeral.RationalServiceInvocation{}
		o.calls = append(o.calls, addCall)
		if cause = service.AddAssignWithInvocation(&value, &power, addCall); cause != nil {
			return ast.ASTHandle{}, fmt.Errorf("UDoc formula bit %d accumulation: %w", bit, cause)
		}
		if cause = addCall.SourceReady(); cause != nil {
			return ast.ASTHandle{}, fmt.Errorf("UDoc formula bit %d completion: %w", bit, cause)
		}
		if cause = o.finishNumberCall(); cause != nil {
			return ast.ASTHandle{}, cause
		}
		if cause = service.CleanupValue(&power); cause != nil {
			return ast.ASTHandle{}, cause
		}
		o.numbers = o.numbers[:len(o.numbers)-1]
	}
	result, err := o.hold(o.manager.BitVectorNumeral(&value, width))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Source rational lifetime ends once the numeral expression owns its value.
	// A refused release remains reachable in numbers for cleanup retry.
	if err = o.closeNumbers(); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

// Each source power/add expression finishes before the accumulator is reused.
// Failed or refused frames stay staged for the outer cleanup continuation.
func (o *typedUDocFormulaOwner) finishNumberCall() error {
	last := len(o.calls) - 1
	call := o.calls[last]
	if err := call.Cleanup(false); err != nil {
		return err
	}
	if err := call.Cleanup(true); err != nil {
		return err
	}
	if err := call.EndContext(); err != nil {
		return err
	}
	o.calls = o.calls[:last]
	return nil
}

func (o *typedUDocFormulaOwner) closeTemps() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if err := o.closeNumbers(); err != nil {
		return err
	}
	for len(o.roots) != 0 {
		last := len(o.roots) - 1
		if o.roots[last] == (ast.ASTRef{}) {
			o.roots = o.roots[:last]
			continue
		}
		err := o.manager.DelRef(&o.roots[last])
		if o.roots[last] == (ast.ASTRef{}) {
			o.roots = o.roots[:last]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *typedUDocFormulaOwner) close() error {
	if err := o.closeTemps(); err != nil {
		return err
	}
	if o.result != (ast.ASTRef{}) {
		if err := o.manager.DelRef(&o.result); err != nil {
			return err
		}
	}
	if o.relation != nil {
		if err := o.relation.Close(); err != nil {
			return err
		}
		o.relation = nil
	}
	return nil
}

type typedUDocFormulaCleanupFailure struct {
	owner             *typedUDocFormulaOwner
	original, cleanup error
	reported          error
	text              string
}

func (f *typedUDocFormulaCleanupFailure) Error() string        { return f.text }
func (f *typedUDocFormulaCleanupFailure) Unwrap() error        { return f.reported }
func (f *typedUDocFormulaCleanupFailure) OriginalCause() error { return f.original }
func (f *typedUDocFormulaCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *typedUDocFormulaCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *typedUDocFormulaCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishTypedUDocFormula(owner *typedUDocFormulaOwner, cause error) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.closeTemps()
	if cleanup == nil && cause != nil {
		cleanup = owner.close()
	}
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &typedUDocFormulaCleanupFailure{owner: owner, original: original, cleanup: cleanup, reported: reported, text: reported.Error()}
}

var _ memory.ConstructionFailure = (*typedUDocFormulaCleanupFailure)(nil)
