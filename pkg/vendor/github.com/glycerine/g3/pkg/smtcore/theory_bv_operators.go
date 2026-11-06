package smtcore

// Source: src/smt/theory_bv.cpp:185-198,687-855,920-980.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// process_args precedes get_arg_bits. Each literal-to-expression conversion
// owns its result independently of the shared bit-blaster temporary trail.
func (c *smtBVExpressionCall) argumentBits(expression ast.ASTHandle, reverse bool) ([][]ast.ASTHandle, error) {
	s := c.theory.state
	count, err := s.manager.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	result := make([][]ast.ASTHandle, count)
	arguments := make([]ast.ASTHandle, count)
	for i := range arguments {
		arguments[i], err = s.manager.GetArg(expression, uint32(i))
		if err != nil {
			return nil, err
		}
		if err = s.Internalize(arguments[i], false); err != nil {
			return nil, err
		}
	}
	for position := range arguments {
		i := position
		if reverse {
			i = len(arguments) - position - 1
		}
		result[i], err = c.bits(arguments[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Copy/extract/concat return borrowed native bit expressions, rather than
// Engine-owned output refs. Retain them until bitsFor publishes its owner.
func (t *SMTBitVectorTheory) retainNativeBits(bits []ast.ASTHandle) ([]ast.ASTHandle, error) {
	for _, bit := range bits {
		ref, err := t.state.manager.NewRef(bit)
		if err != nil {
			return nil, err
		}
		t.temporary = append(t.temporary, ref)
	}
	return bits, nil
}

// assertTermOperatorShape preserves the preconditions of the source operator
// bodies before process_args creates the term enode. MK_BINARY_COND has no
// retained invocation in theory_bv.cpp and therefore has no Go counterpart.
func (t *SMTBitVectorTheory) assertTermOperatorShape(expression ast.ASTHandle, operation int32, count uint32) error {
	s := t.state
	fresh := func(line int32) error {
		return bvAssert(line, "!get_context().e_internalized(n)", func() (bool, error) {
			return s.context.enodes[expression] == nil, nil
		})
	}
	arity := func(line int32, condition string, holds bool) error {
		return bvAssert(line, condition, func() (bool, error) { return holds, nil })
	}
	switch operation {
	case 32, 42, 43:
		if err := fresh(689); err != nil {
			return err
		}
		return arity(690, "n->get_num_args() == 1", count == 1)
	case 17, 18, 19, 20, 21, 44, 45, 46, 47, 50, 51:
		if err := fresh(702); err != nil {
			return err
		}
		return arity(703, "n->get_num_args() == 2", count == 2)
	case 4, 6, 30, 31, 33, 34, 35, 36:
		if err := fresh(718); err != nil {
			return err
		}
		return arity(719, "n->get_num_args() >= 2", count >= 2)
	case 5:
		if err := fresh(766); err != nil {
			return err
		}
		return arity(767, "n->get_num_args() == 2", count == 2)
	case 38, 39, 48, 49:
		if err := fresh(806); err != nil {
			return err
		}
		return arity(807, "n->get_num_args() == 1", count == 1)
	case 40:
		return arity(842, "n->get_num_args() == 1", count == 1)
	default:
		return nil
	}
}

func assertBVOperatorWidth(line int32, condition string, left, right []ast.ASTHandle) error {
	return bvAssert(line, condition, func() (bool, error) { return len(left) == len(right), nil })
}

func assertBVExtractBounds(low, high uint32) error {
	return bvAssert(849, "start <= end", func() (bool, error) { return low <= high, nil })
}

func assertBVAtomArity(operation int32, count int) error {
	switch operation {
	case 52, 53, 54:
		return bvAssert(919, "n->get_num_args() == 2", func() (bool, error) { return count == 2, nil })
	case 22, 23:
		return bvAssert(954, "n->get_num_args() == 2", func() (bool, error) { return count == 2, nil })
	default:
		return nil
	}
}

func retainedBVTermOperation(operation int32) bool {
	switch operation {
	case 0, 4, 5, 6, 17, 18, 19, 20, 21, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 56, 57, 58:
		return true
	default:
		return false
	}
}

func retainedBVAtomOperation(operation int32) bool {
	switch operation {
	case 22, 23, 52, 53, 54, 55, 59, 60:
		return true
	default:
		return false
	}
}

func (t *SMTBitVectorTheory) operationBits(expression ast.ASTHandle, operation int32) (bits []ast.ASTHandle, handled bool, result error) {
	s := t.state
	if operation == 56 {
		count, err := s.manager.GetNumArgs(expression)
		if err != nil {
			return nil, true, err
		}
		bits = make([]ast.ASTHandle, count)
		for i := range bits {
			bits[i], err = s.manager.GetArg(expression, uint32(i))
			if err != nil {
				return nil, true, err
			}
		}
		bits, err = t.retainNativeBits(bits)
		return bits, true, err
	}
	// A refused operator must not try to reinterpret arbitrary arguments as BV.
	switch operation {
	case 4, 5, 6, 17, 18, 19, 20, 21, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51:
	default:
		return nil, false, nil
	}
	call := smtBVExpressionCall{theory: t}
	defer func() { result = errors.Join(result, call.close()) }()
	reverse := operation == 4 || operation == 6 || operation == 30 || operation == 31 || operation == 33 || operation == 34 || operation == 35 || operation == 36 || operation == 37
	arguments, err := call.argumentBits(expression, reverse)
	if err != nil {
		return nil, true, err
	}
	if operation == 44 {
		if len(arguments) == 2 {
			if err = assertBVOperatorWidth(710, "arg1_bits.size() == arg2_bits.size()", arguments[0], arguments[1]); err != nil {
				return nil, true, err
			}
		}
	}
	if operation == 42 || operation == 43 || operation == 44 {
		bit, err := t.nativeAtomDefinition(operation, arguments)
		return []ast.ASTHandle{bit}, true, err
	}
	engine, err := t.nativeBitBlaster()
	if err != nil {
		return nil, true, err
	}
	var refs []ast.ASTRef
	switch operation {
	case 4, 6, 30, 31, 33, 34, 35, 36:
		if len(arguments) < 2 {
			return nil, true, ast.ErrASTDomain
		}
		binary := engine.MkAdder
		switch operation {
		case 6:
			binary = engine.MkMultiplier
		case 30:
			binary = engine.MkAnd
		case 31:
			binary = engine.MkOr
		case 33:
			binary = engine.MkXor
		case 34:
			binary = engine.MkNand
		case 35:
			binary = engine.MkNor
		case 36:
			binary = engine.MkXnor
		}
		// MK_AC_BINARY starts with the last operand and visits previous operands
		// in descending order, keeping their order at every binary call.
		bits = arguments[len(arguments)-1]
		for i := len(arguments) - 2; i >= 0; i-- {
			if err = assertBVOperatorWidth(733, "arg_bits.size() == bits.size()", arguments[i], bits); err != nil {
				return nil, true, err
			}
			refs, err = binary(arguments[i], bits)
			if err != nil {
				return nil, true, err
			}
			bits, err = t.adoptBitBlasterRefs(refs)
			if err != nil {
				return nil, true, err
			}
		}
		return bits, true, nil
	case 32, 38, 39, 40, 48, 49:
		if len(arguments) != 1 || len(arguments[0]) == 0 {
			return nil, true, ast.ErrASTDomain
		}
		switch operation {
		case 32:
			refs, err = engine.MkNot(arguments[0])
		case 38, 39:
			var extra uint32
			extra, err = t.parameter(expression, 0)
			if err == nil {
				if operation == 38 {
					refs, err = engine.MkSignExtend(arguments[0], int(extra))
				} else {
					refs, err = engine.MkZeroExtend(arguments[0], int(extra))
				}
			}
		case 40:
			var high, low uint32
			high, err = t.parameter(expression, 0)
			if err == nil {
				low, err = t.parameter(expression, 1)
			}
			if err != nil {
				return nil, true, err
			}
			if err = assertBVExtractBounds(low, high); err != nil {
				return nil, true, err
			}
			if low > high || uint64(high) >= uint64(len(arguments[0])) {
				return nil, true, ast.ErrASTDomain
			}
			bits, err = t.retainNativeBits(append([]ast.ASTHandle(nil), arguments[0][low:high+1]...))
			return bits, true, err
		case 48, 49:
			var amount uint32
			amount, err = t.parameter(expression, 0)
			if err == nil {
				if operation == 48 {
					refs, err = engine.MkRotateLeft(arguments[0], uint64(amount))
				} else {
					refs, err = engine.MkRotateRight(arguments[0], uint64(amount))
				}
			}
		}
	case 37:
		for i := len(arguments) - 1; i >= 0; i-- {
			bits = append(bits, arguments[i]...)
		}
		bits, err = t.retainNativeBits(bits)
		return bits, true, err
	default:
		if len(arguments) != 2 || len(arguments[0]) == 0 {
			return nil, true, ast.ErrASTDomain
		}
		left, right := arguments[0], arguments[1]
		widthLine := int32(710)
		if operation == 5 {
			widthLine = 774
		}
		if err = assertBVOperatorWidth(widthLine, "arg1_bits.size() == arg2_bits.size()", left, right); err != nil {
			return nil, true, err
		}
		if len(left) != len(right) {
			return nil, true, ast.ErrASTDomain
		}
		switch operation {
		case 5:
			var carry ast.ASTRef
			refs, carry, err = engine.MkSubtracter(left, right)
			if err == nil {
				t.temporary = append(t.temporary, carry)
			}
		case 17:
			refs, err = engine.MkSignedDiv(left, right)
		case 18:
			refs, err = engine.MkUnsignedDiv(left, right)
		case 19:
			refs, err = engine.MkSignedRem(left, right)
		case 20:
			refs, err = engine.MkUnsignedRem(left, right)
		case 21:
			refs, err = engine.MkSignedMod(left, right)
		case 45:
			refs, err = engine.MkShiftLeft(left, right)
		case 46:
			refs, err = engine.MkLogicalShiftRight(left, right)
		case 47:
			refs, err = engine.MkArithmeticShiftRight(left, right)
		case 50:
			refs, err = engine.MkExtendedRotateLeft(left, right)
		case 51:
			refs, err = engine.MkExtendedRotateRight(left, right)
		}
	}
	if err != nil {
		return nil, true, err
	}
	bits, err = t.adoptBitBlasterRefs(refs)
	return bits, true, err
}

// The live scalar bit-blaster consumers produce reductions, comparison atoms,
// and overflow atoms. Their owned output survives recursive gate internalization.
func (t *SMTBitVectorTheory) nativeAtomDefinition(operation int32, arguments [][]ast.ASTHandle) (ast.ASTHandle, error) {
	if err := assertBVAtomArity(operation, len(arguments)); err != nil {
		return ast.ASTHandle{}, err
	}
	engine, err := t.nativeBitBlaster()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(arguments) == 0 || len(arguments[0]) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if (operation == 42 || operation == 43) && len(arguments) != 1 || operation != 42 && operation != 43 && (len(arguments) != 2 || len(arguments[0]) != len(arguments[1])) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	ref, err := t.state.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch operation {
	case 22:
		err = engine.MkUnsignedLE(arguments[0], arguments[1], ref)
	case 23:
		err = engine.MkSignedLE(arguments[0], arguments[1], ref)
	case 42:
		err = engine.MkReductionOr(arguments[0], ref)
	case 43:
		err = engine.MkReductionAnd(arguments[0], ref)
	case 44:
		err = engine.MkComp(arguments[0], arguments[1], ref)
	case 52:
		err = engine.MkUnsignedMulNoOverflow(arguments[0], arguments[1], ref)
	case 53:
		err = engine.MkSignedMulNoOverflow(arguments[0], arguments[1], ref)
	case 54:
		err = engine.MkSignedMulNoUnderflow(arguments[0], arguments[1], ref)
	default:
		err = ast.ErrASTDomain
	}
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, t.state.manager.DelRef(&ref))
	}
	t.temporary = append(t.temporary, ref)
	return t.state.manager.RefGet(ref)
}

func (t *SMTBitVectorTheory) boolXor(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	engine, err := t.nativeBitBlaster()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	refs, err := engine.MkXor([]ast.ASTHandle{left}, []ast.ASTHandle{right})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bits, err := t.adoptBitBlasterRefs(refs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return bits[0], nil
}
