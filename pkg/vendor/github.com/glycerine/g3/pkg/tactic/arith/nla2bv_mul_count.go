package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:366-397.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// countMultiplications is the concrete count_mul_proc specialization. Arithmetic
// products count every multiplication edge. BV products count only non-numeral
// operands, matching the diagnostic's abstraction of coefficient products.
func (c *nla2BVInvocation) countMultiplications(g *tactic.Goal) (uint32, error) {
	terms, err := c.postorder(g)
	if err != nil {
		return 0, err
	}
	m := c.t.manager
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return 0, err
	}
	bitVector, err := m.BitVectorUtilityFamilyID()
	if err != nil {
		return 0, err
	}
	var result uint32
	for _, term := range terms {
		kind, err := m.GetKind(term)
		if err != nil {
			return 0, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := m.GetFamilyID(term)
		if err != nil {
			return 0, err
		}
		op, err := m.GetDeclKind(term)
		if err != nil {
			return 0, err
		}
		count, err := m.GetNumArgs(term)
		if err != nil {
			return 0, err
		}
		if family == arithmetic && op == 9 {
			result += count - 1
			continue
		}
		if family != bitVector || op != 6 {
			continue
		}
		var symbolic uint32
		for i := uint32(0); i < count; i++ {
			arg, err := m.GetArg(term, i)
			if err != nil {
				return 0, err
			}
			argKind, err := m.GetKind(arg)
			if err != nil {
				return 0, err
			}
			if argKind != ast.ASTApp {
				symbolic++
				continue
			}
			argFamily, err := m.GetFamilyID(arg)
			if err != nil {
				return 0, err
			}
			argOp, err := m.GetDeclKind(arg)
			if err != nil {
				return 0, err
			}
			if argFamily != bitVector || argOp != 0 {
				symbolic++
			}
		}
		if symbolic > 1 {
			result += symbolic - 1
		}
	}
	return result, nil
}
