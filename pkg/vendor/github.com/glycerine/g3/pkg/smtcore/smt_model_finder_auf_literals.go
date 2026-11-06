package smtcore

// Source: src/smt/smt_model_finder.cpp direct variable qinfo constraints.

import "github.com/glycerine/g3/pkg/ast"

type smtModelFinderAUFLiteralKind uint8

const (
	aufEqualVariables smtModelFinderAUFLiteralKind = iota
	aufDisequalVariables
	aufEqualGround
	aufDisequalGround
	aufOrderedVariables
	aufSignedOrderedVariables
	aufOrderedGround
)

type smtModelFinderAUFLiteral struct {
	kind       smtModelFinderAUFLiteralKind
	quantifier ast.ASTHandle
	first      uint32
	second     uint32
	term       ast.ASTHandle
}

func (g *smtModelFinderAUFGraph) collectLiterals(quantifier, formula ast.ASTHandle, negative bool) error {
	m := g.finder.manager
	ground, err := m.IsGround(formula)
	if err != nil || ground {
		return err
	}
	flag := uint8(1)
	if negative {
		flag = 2
	}
	key := smtModelFinderAnalysisKey{quantifier, formula}
	if g.analysisSeen[key]&flag != 0 {
		return nil
	}
	g.analysisSeen[key] |= flag
	if err := g.finder.checkpoint(); err != nil {
		return err
	}
	kind, err := m.GetKind(formula)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		variable, err := m.GetVarIndex(formula)
		if err != nil {
			return err
		}
		var term ast.ASTHandle
		if negative {
			term, err = m.MkTrue()
		} else {
			term, err = m.MkFalse()
		}
		if err != nil {
			return err
		}
		g.appendLiteral(smtModelFinderAUFLiteral{kind: aufDisequalGround, quantifier: quantifier, first: variable, term: term})
		return nil
	}
	if kind != ast.ASTApp {
		return nil
	}
	family, err := m.GetFamilyID(formula)
	if err != nil {
		return err
	}
	if family != m.BasicFamilyID() {
		before := len(g.literals)
		if err := g.collectComparison(quantifier, formula, negative); err != nil {
			return err
		}
		if len(g.literals) > before {
			return nil
		}
		return g.analyzeApplication(quantifier, formula)
	}
	op, err := m.GetDeclKind(formula)
	if err != nil {
		return err
	}
	n, err := m.GetNumArgs(formula)
	if err != nil {
		return err
	}
	switch op {
	case ast.ASTOpNot:
		arg, err := m.GetArg(formula, 0)
		if err != nil {
			return err
		}
		return g.collectLiterals(quantifier, arg, !negative)
	case ast.ASTOpOr, ast.ASTOpAnd, ast.ASTOpIff, ast.ASTOpIte:
		for i := uint32(0); i < n; i++ {
			arg, err := m.GetArg(formula, i)
			if err != nil {
				return err
			}
			if err = g.collectLiterals(quantifier, arg, negative); err != nil {
				return err
			}
			if op == ast.ASTOpIff || op == ast.ASTOpIte && i == 0 {
				if err = g.collectLiterals(quantifier, arg, !negative); err != nil {
					return err
				}
			}
		}
		return nil
	case ast.ASTOpEq:
	default:
		return g.analyzeApplication(quantifier, formula)
	}
	left, err := m.GetArg(formula, 0)
	if err != nil {
		return err
	}
	right, err := m.GetArg(formula, 1)
	if err != nil {
		return err
	}
	variable, bound, _, matched, err := g.variableAndGround(left, right)
	if err != nil {
		return err
	}
	row := smtModelFinderAUFLiteral{quantifier: quantifier, first: variable, term: bound}
	if matched {
		row.kind = aufEqualGround
		if negative {
			row.kind = aufDisequalGround
		}
	} else {
		first, second, matched, err := g.variableAndVariable(left, right)
		if err != nil || !matched {
			if err != nil {
				return err
			}
			return g.analyzeApplication(quantifier, formula)
		}
		row.first, row.second = first, second
		row.kind = aufEqualVariables
		if negative {
			row.kind = aufDisequalVariables
		}
	}
	g.appendLiteral(row)
	return nil
}

func (g *smtModelFinderAUFGraph) processLiterals() error {
	for _, info := range g.literals {
		first, err := g.uvar(info.quantifier, info.first)
		if err != nil {
			return err
		}
		switch info.kind {
		case aufEqualVariables, aufDisequalVariables, aufOrderedVariables, aufSignedOrderedVariables:
			second, err := g.uvar(info.quantifier, info.second)
			if err != nil {
				return err
			}
			if info.kind != aufEqualVariables {
				if err = first.merge(second); err != nil {
					return err
				}
				if info.kind == aufOrderedVariables || info.kind == aufSignedOrderedVariables {
					if err = first.setMono(); err != nil {
						return err
					}
				}
				if info.kind == aufSignedOrderedVariables {
					if err = first.setSigned(); err != nil {
						return err
					}
				}
			} else {
				if err = first.insertAvoid(second); err != nil {
					return err
				}
				if err = second.insertAvoid(first); err != nil {
					return err
				}
			}
		case aufEqualGround:
			if err = first.insertException(info.term); err != nil {
				return err
			}
		case aufOrderedGround:
			if err = first.setMono(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) populateLiterals(quantifier ast.ASTHandle) error {
	for _, info := range g.literals {
		if info.quantifier != quantifier {
			continue
		}
		node := g.uvars[smtModelFinderAUFKey{info.quantifier, info.first}]
		switch info.kind {
		case aufDisequalGround, aufOrderedGround:
			if _, err := node.insert(info.term, 0); err != nil {
				return err
			}
		case aufEqualGround:
			uninterpreted, err := g.finder.manager.IsUninterpretedSort(node.sort)
			if err != nil {
				return err
			}
			if !uninterpreted {
				continue
			}
			// For x=t on an uninterpreted sort, the source includes every
			// relevant context term of that sort (paper section 4.1).
			for _, enode := range g.finder.state.context.enodes {
				if !g.finder.state.isRelevant(enode.owner) {
					continue
				}
				sort, err := g.finder.manager.GetSort(enode.owner)
				if err != nil {
					return err
				}
				if sort == node.sort {
					if _, err = node.insert(enode.owner, enode.Generation()); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
