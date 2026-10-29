package tactic

// Source: src/tactic/probe.cpp:211-424,462-527.

import "github.com/glycerine/g3/pkg/ast"

func walkGoalExpressions(g *Goal, visit func(ast.ASTHandle, ast.ASTKind) (bool, error)) (bool, error) {
	visited := make(map[ast.ASTHandle]bool)
	stack := make([]ast.ASTHandle, 0, len(g.forms))
	for i := len(g.forms) - 1; i >= 0; i-- {
		stack = append(stack, g.forms[i])
	}
	for len(stack) != 0 {
		i := len(stack) - 1
		h := stack[i]
		stack = stack[:i]
		if visited[h] {
			continue
		}
		visited[h] = true
		kind, err := g.manager.GetKind(h)
		if err != nil {
			return false, err
		}
		stop, err := visit(h, kind)
		if err != nil || stop {
			return stop, err
		}
		switch kind {
		case ast.ASTVar:
		case ast.ASTApp:
			n, err := g.manager.GetNumArgs(h)
			if err != nil {
				return false, err
			}
			for j := n; j != 0; j-- {
				arg, err := g.manager.GetArg(h, j-1)
				if err != nil {
					return false, err
				}
				stack = append(stack, arg)
			}
		case ast.ASTQuantifier:
			body, err := g.manager.GetQuantifierBody(h)
			if err != nil {
				return false, err
			}
			stack = append(stack, body)
			n, err := g.manager.GetNumNoPatterns(h)
			if err != nil {
				return false, err
			}
			for j := n; j != 0; j-- {
				pattern, err := g.manager.GetNoPattern(h, j-1)
				if err != nil {
					return false, err
				}
				stack = append(stack, pattern)
			}
			n, err = g.manager.GetNumPatterns(h)
			if err != nil {
				return false, err
			}
			for j := n; j != 0; j-- {
				pattern, err := g.manager.GetPattern(h, j-1)
				if err != nil {
					return false, err
				}
				stack = append(stack, pattern)
			}
		default:
			return false, ast.ErrASTDomain
		}
	}
	return false, nil
}

func probeIsBool(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	sort, err := m.GetSort(h)
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	return sort == boolean, err
}

func (p *Probe) countConstants(g *Goal) (uint32, error) {
	var count uint32
	arithmetic := ast.NullFamilyID
	bitVector := ast.NullFamilyID
	var err error
	if p.kind == ProbeNumArithmeticConsts {
		arithmetic, err = g.manager.ArithmeticFamilyID()
	} else if p.kind == ProbeNumBitVectorConsts {
		bitVector, err = g.manager.BitVectorUtilityFamilyID()
	}
	if err != nil {
		return 0, err
	}
	_, err = walkGoalExpressions(g, func(h ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind != ast.ASTApp {
			return false, nil
		}
		n, err := g.manager.GetNumArgs(h)
		if err != nil || n != 0 {
			return false, err
		}
		value, err := g.manager.IsValue(h)
		if err != nil || value {
			return false, err
		}
		isBool, err := probeIsBool(g.manager, h)
		if err != nil {
			return false, err
		}
		switch p.kind {
		case ProbeNumBoolConsts:
			if isBool {
				count++
			}
		case ProbeNumConsts:
			if !isBool {
				count++
			}
		case ProbeNumArithmeticConsts, ProbeNumBitVectorConsts:
			if isBool {
				return false, nil
			}
			sort, err := g.manager.GetSort(h)
			if err != nil {
				return false, err
			}
			family, err := g.manager.GetFamilyID(sort)
			if err != nil {
				return false, err
			}
			if family == arithmetic || family == bitVector {
				count++
			}
		}
		return false, nil
	})
	return count, err
}

func (p *Probe) goalHasQuantifier(g *Goal) (bool, error) {
	return walkGoalExpressions(g, func(_ ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		return kind == ast.ASTQuantifier, nil
	})
}

func (p *Probe) goalHasPattern(g *Goal) (bool, error) {
	return walkGoalExpressions(g, func(h ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind != ast.ASTQuantifier {
			return false, nil
		}
		n, err := g.manager.GetNumPatterns(h)
		if err != nil || n != 0 {
			return n != 0, err
		}
		n, err = g.manager.GetNumNoPatterns(h)
		return n != 0, err
	})
}

func (p *Probe) goalInFragment(g *Goal) (bool, error) {
	bitVector := ast.NullFamilyID
	array := ast.NullFamilyID
	var err error
	if p.kind != ProbeIsPropositional {
		bitVector, err = g.manager.BitVectorUtilityFamilyID()
		if err != nil {
			return false, err
		}
	}
	if p.kind == ProbeIsQFArrayUFBitVector {
		array, err = g.manager.ArrayFamilyID()
		if err != nil {
			return false, err
		}
	}
	bad, err := walkGoalExpressions(g, func(h ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind != ast.ASTApp {
			return true, nil
		}
		isBool, err := probeIsBool(g.manager, h)
		if err != nil {
			return false, err
		}
		sort, err := g.manager.GetSort(h)
		if err != nil {
			return false, err
		}
		isBV, err := g.manager.IsBitVectorSort(sort)
		if err != nil {
			return false, err
		}
		family, err := g.manager.GetFamilyID(h)
		if err != nil {
			return false, err
		}
		sortFamily, err := g.manager.GetFamilyID(sort)
		if err != nil {
			return false, err
		}
		n, err := g.manager.GetNumArgs(h)
		if err != nil {
			return false, err
		}
		uninterpreted := family == ast.NullFamilyID
		switch p.kind {
		case ProbeIsPropositional:
			return !isBool || family != g.manager.BasicFamilyID() && !(uninterpreted && n == 0), nil
		case ProbeIsQFBitVector:
			if !isBool && !isBV {
				return true, nil
			}
			if family == g.manager.BasicFamilyID() || uninterpreted && n == 0 {
				return false, nil
			}
			if family != bitVector {
				return true, nil
			}
			op, err := g.manager.GetDeclKind(h)
			return op >= 12 && op <= 16, err
		case ProbeIsQFArrayUFBitVector:
			if !isBool && !isBV && sortFamily != array {
				return true, nil
			}
			return family != g.manager.BasicFamilyID() && family != bitVector && family != array && !uninterpreted, nil
		case ProbeIsQFUFBitVector:
			if !isBool && !isBV {
				return true, nil
			}
			return family != g.manager.BasicFamilyID() && family != bitVector && !uninterpreted, nil
		}
		return true, nil
	})
	return !bad, err
}
