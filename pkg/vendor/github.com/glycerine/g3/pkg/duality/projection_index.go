package duality

// Source: duality_rpfp.cpp, VariableProjector::IndexLA family.
import "github.com/glycerine/g3/pkg/ast"

type projectionIndex struct {
	arithmetic   *projectionArithmetic
	indices      [2]map[ast.ASTHandle]int
	coefficients [2]map[ast.ASTHandle]ast.ASTHandle
	positive     []ast.ASTHandle
	fixing       bool
}

func newProjectionIndex(p *projectionArithmetic) *projectionIndex {
	x := &projectionIndex{arithmetic: p}
	for i := 0; i < 2; i++ {
		x.indices[i] = make(map[ast.ASTHandle]int)
		x.coefficients[i] = make(map[ast.ASTHandle]ast.ASTHandle)
	}
	return x
}
func (x *projectionIndex) remove(h ast.ASTHandle) error {
	m := x.arithmetic.graph.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return nil
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return err
	}
	family, err := m.GetFamilyID(h)
	if err != nil {
		return err
	}
	if n == 0 && family == ast.NullFamilyID {
		x.indices[0][h] = -1
		x.indices[1][h] = -1
		return nil
	}
	for i := uint32(0); i < n; i++ {
		child, err := m.GetArg(h, i)
		if err != nil {
			return err
		}
		if err := x.remove(child); err != nil {
			return err
		}
	}
	return nil
}
func (x *projectionIndex) linear(positive bool, coefficient, h ast.ASTHandle, id int) error {
	p := x.arithmetic
	m := p.graph.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return nil
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return err
	}
	family, err := m.GetFamilyID(h)
	if err != nil {
		return err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return err
	}
	if family == p.family && op == 0 {
		return nil
	}
	if family == p.family && (op == 6 || op == 7) {
		for i := uint32(0); i < n; i++ {
			child, err := m.GetArg(h, i)
			if err != nil {
				return err
			}
			sign := positive
			if op == 7 && i == 1 {
				sign = !sign
			}
			if err := x.linear(sign, coefficient, child, id); err != nil {
				return err
			}
		}
		return nil
	}
	if family == p.family && op == 9 && n == 2 {
		a, err := m.GetArg(h, 0)
		if err != nil {
			return err
		}
		b, err := m.GetArg(h, 1)
		if err != nil {
			return err
		}
		ak, err := p.kind(a)
		if err != nil {
			return err
		}
		bk, err := p.kind(b)
		if err != nil {
			return err
		}
		if ak != 0 && bk != 0 {
			return nil
		}
		if bk == 0 {
			a, b = b, a
		}
		product, err := p.app(9, coefficient, a)
		if err != nil {
			return err
		}
		product, err = p.normalized(product, false)
		if err != nil {
			return err
		}
		zero, err := p.numeral("0")
		if err != nil {
			return err
		}
		sign, err := p.app(3, product, zero)
		if err != nil {
			return err
		}
		sign, err = p.normalized(sign, false)
		if err != nil {
			return err
		}
		return x.linear(m.IsTrue(sign), product, b, id)
	}
	polarity := 0
	if positive {
		polarity = 1
	}
	if n == 0 && family == ast.NullFamilyID {
		if _, exists := x.indices[polarity][h]; x.fixing || !exists {
			x.indices[polarity][h] = id
			x.coefficients[polarity][h] = coefficient
			if positive && !x.fixing {
				x.positive = append(x.positive, h)
			}
			return nil
		}
	}
	return x.remove(h)
}
func (x *projectionIndex) predicate(positive bool, h ast.ASTHandle, id int) error {
	p := x.arithmetic
	m := p.graph.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return nil
	}
	neg, child, err := m.IsNot(h)
	if err != nil {
		return err
	}
	if neg {
		return x.predicate(!positive, child, id)
	}
	op, err := p.kind(h)
	if err != nil {
		return err
	}
	if op < 2 || op > 5 {
		return x.remove(h)
	}
	a, err := m.GetArg(h, 0)
	if err != nil {
		return err
	}
	b, err := m.GetArg(h, 1)
	if err != nil {
		return err
	}
	sign := positive
	if op == 2 || op == 4 {
		sign = !sign
	}
	for i, term := range []ast.ASTHandle{a, b} {
		pos := sign
		if i == 1 {
			pos = !pos
		}
		value := "-1"
		if pos {
			value = "1"
		}
		coeff, err := p.numeral(value)
		if err != nil {
			return err
		}
		if err := x.linear(pos, coeff, term, id); err != nil {
			return err
		}
	}
	return nil
}
