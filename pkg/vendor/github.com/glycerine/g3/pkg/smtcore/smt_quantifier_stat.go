package smtcore

// Source: src/smt/smt_quantifier_stat.h and src/smt/smt_quantifier_stat.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/approx"
)

type SMTQuantifierStat struct {
	size, depth, generation, caseSplitFactor                      uint32
	numNestedQuantifiers, numInstances, numInstancesCurrentSearch uint32
	numInstancesCurrentBranch, maxGeneration                      uint32
	maxCost                                                       float32
}

func (s *SMTQuantifierStat) Size() uint32                      { return s.size }
func (s *SMTQuantifierStat) Depth() uint32                     { return s.depth }
func (s *SMTQuantifierStat) Generation() uint32                { return s.generation }
func (s *SMTQuantifierStat) CaseSplitFactor() uint32           { return s.caseSplitFactor }
func (s *SMTQuantifierStat) NumNestedQuantifiers() uint32      { return s.numNestedQuantifiers }
func (s *SMTQuantifierStat) NumInstances() uint32              { return s.numInstances }
func (s *SMTQuantifierStat) NumInstancesCurrentSearch() uint32 { return s.numInstancesCurrentSearch }
func (s *SMTQuantifierStat) NumInstancesCurrentBranch() uint32 { return s.numInstancesCurrentBranch }
func (s *SMTQuantifierStat) MaxGeneration() uint32             { return s.maxGeneration }
func (s *SMTQuantifierStat) MaxCost() float32                  { return s.maxCost }

func (s *SMTQuantifierStat) IncrementInstances() {
	s.numInstances++
	s.numInstancesCurrentSearch++
}
func (s *SMTQuantifierStat) IncrementInstancesCurrentBranch() {
	s.numInstancesCurrentBranch++
}
func (s *SMTQuantifierStat) ResetInstancesCurrentSearch() { s.numInstancesCurrentSearch = 0 }
func (s *SMTQuantifierStat) UpdateMaxGeneration(generation uint32) {
	if s.maxGeneration < generation {
		s.maxGeneration = generation
	}
}
func (s *SMTQuantifierStat) UpdateMaxCost(cost float32) {
	if s.maxCost < cost {
		s.maxCost = cost
	}
}

type SMTQuantifierStatGenerator struct{ manager *ast.ASTManager }

func NewSMTQuantifierStatGenerator(m *ast.ASTManager) (*SMTQuantifierStatGenerator, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	return &SMTQuantifierStatGenerator{manager: m}, nil
}

type smtQuantifierStatEntry struct {
	expression ast.ASTHandle
	depth      uint32
	depthOnly  bool
}

func (g *SMTQuantifierStatGenerator) Generate(quantifier ast.ASTHandle, generation uint32) (*SMTQuantifierStat, error) {
	if g == nil || g.manager == nil {
		return nil, ast.ErrASTDomain
	}
	kind, err := g.manager.GetKind(quantifier)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTQuantifier {
		return nil, ast.ErrASTDomain
	}
	body, err := g.manager.GetQuantifierBody(quantifier)
	if err != nil {
		return nil, err
	}
	result := &SMTQuantifierStat{generation: generation, caseSplitFactor: 1}
	alreadyFound := make(map[ast.ASTHandle]uint32)
	todo := []smtQuantifierStatEntry{{expression: body}}
	caseSplit := approx.NewNat(1)
	for len(todo) != 0 {
		last := len(todo) - 1
		entry := todo[last]
		todo = todo[:last]
		depthOnly := entry.depthOnly
		if oldDepth, found := alreadyFound[entry.expression]; found {
			if oldDepth >= entry.depth {
				continue
			}
			depthOnly = true
		}
		alreadyFound[entry.expression] = entry.depth
		if entry.depth >= result.depth {
			result.depth = entry.depth
		}
		kind, err := g.manager.GetKind(entry.expression)
		if err != nil {
			return nil, err
		}
		if !depthOnly {
			result.size++
			if kind == ast.ASTQuantifier {
				result.numNestedQuantifiers++
			}
			if kind == ast.ASTApp {
				family, err := g.manager.GetFamilyID(entry.expression)
				if err != nil {
					return nil, err
				}
				if family == g.manager.BasicFamilyID() {
					op, err := g.manager.GetDeclKind(entry.expression)
					if err != nil {
						return nil, err
					}
					count, err := g.manager.GetNumArgs(entry.expression)
					if err != nil {
						return nil, err
					}
					switch op {
					case ast.ASTOpOr:
						if entry.depth == 0 {
							caseSplit.MulUint32(count)
						} else {
							caseSplit.MulUint32(count + 1)
						}
					case ast.ASTOpAnd:
						if entry.depth > 0 {
							caseSplit.MulUint32(count + 1)
						}
					case ast.ASTOpIff, ast.ASTOpIte:
						if entry.depth == 0 {
							caseSplit.MulUint32(4)
						} else {
							caseSplit.MulUint32(9)
						}
					}
				}
			}
		}
		if kind == ast.ASTApp {
			count, err := g.manager.GetNumArgs(entry.expression)
			if err != nil {
				return nil, err
			}
			for i := count; i > 0; i-- {
				child, err := g.manager.GetArg(entry.expression, i-1)
				if err != nil {
					return nil, err
				}
				todo = append(todo, smtQuantifierStatEntry{expression: child, depth: entry.depth + 1, depthOnly: depthOnly})
			}
		}
	}
	result.caseSplitFactor = caseSplit.Value()
	return result, nil
}
