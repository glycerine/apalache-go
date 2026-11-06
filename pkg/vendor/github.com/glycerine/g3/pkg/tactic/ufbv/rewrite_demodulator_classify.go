package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:52-210.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// demodulatorClassifier is the concrete default UFBV term ordering. Its
// temporary variable sets are native Go maps, keyed by de Bruijn index.
type demodulatorClassifier struct {
	manager *ast.ASTManager
	trace   *diagnostic.TraceTags
}

func (c demodulatorClassifier) traceTest(lhs, rhs ast.ASTHandle, subset, smaller int) error {
	tags := c.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("demodulator") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("demodulator", "is_demodulator", "src/tactic/ufbv/ufbv_rewriter.cpp", 63,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "testing is_demodulator:\n")
			left, e := formulaTraceText(c.manager, lhs)
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, left+"\n")
			right, e := formulaTraceText(c.manager, rhs)
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, right+"\nsubset: "+strconv.Itoa(subset)+", smaller: "+strconv.Itoa(smaller)+"\n")
			return nil
		})
	return operandErr
}

func (c demodulatorClassifier) uninterpreted(expr ast.ASTHandle) (bool, error) {
	kind, e := c.manager.GetKind(expr)
	if e != nil || kind != ast.ASTApp {
		return false, e
	}
	family, e := c.manager.GetFamilyID(expr)
	return family == ast.NullFamilyID, e
}

// variables follows for_each_expr over body and quantifier annotations; it
// collects every reached AST_VAR index without adjusting quantifier depth.
func (c demodulatorClassifier) variables(root ast.ASTHandle) (map[uint32]struct{}, error) {
	set := make(map[uint32]struct{})
	seen := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		h := stack[last]
		stack = stack[:last]
		if _, exists := seen[h]; exists {
			continue
		}
		seen[h] = struct{}{}
		kind, e := c.manager.GetKind(h)
		if e != nil {
			return nil, e
		}
		switch kind {
		case ast.ASTVar:
			index, e := c.manager.GetVarIndex(h)
			if e != nil {
				return nil, e
			}
			set[index] = struct{}{}
		case ast.ASTApp:
			n, e := c.manager.GetNumArgs(h)
			if e != nil {
				return nil, e
			}
			for i := n; i > 0; i-- {
				child, e := c.manager.GetArg(h, i-1)
				if e != nil {
					return nil, e
				}
				stack = append(stack, child)
			}
		case ast.ASTQuantifier:
			body, e := c.manager.GetQuantifierBody(h)
			if e != nil {
				return nil, e
			}
			stack = append(stack, body)
			n, e := c.manager.GetNumPatterns(h)
			if e != nil {
				return nil, e
			}
			for i := n; i > 0; i-- {
				child, e := c.manager.GetPattern(h, i-1)
				if e != nil {
					return nil, e
				}
				stack = append(stack, child)
			}
			n, e = c.manager.GetNumNoPatterns(h)
			if e != nil {
				return nil, e
			}
			for i := n; i > 0; i-- {
				child, e := c.manager.GetNoPattern(h, i-1)
				if e != nil {
					return nil, e
				}
				stack = append(stack, child)
			}
		default:
			return nil, ast.ErrASTDomain
		}
	}
	return set, nil
}

// subset matches the source's signed result, including its value fast path.
func (c demodulatorClassifier) subset(left, right ast.ASTHandle) (int, error) {
	value, e := c.manager.IsValue(left)
	if e != nil {
		return 0, e
	}
	if value {
		return 1, nil
	}
	a, e := c.variables(left)
	if e != nil {
		return 0, e
	}
	b, e := c.variables(right)
	if e != nil {
		return 0, e
	}
	leftInRight, rightInLeft := true, true
	for x := range a {
		if _, present := b[x]; !present {
			leftInRight = false
		}
	}
	for x := range b {
		if _, present := a[x]; !present {
			rightInLeft = false
		}
	}
	if leftInRight && rightInLeft {
		return 2, nil
	}
	if leftInRight {
		return 1, nil
	}
	if rightInLeft {
		return -1, nil
	}
	return 0, nil
}

// smaller uses value status, interpreted/uninterpreted rank, uninterpreted
// arity and declaration ID, then cached expression depth in source order.
func (c demodulatorClassifier) smaller(left, right ast.ASTHandle) (int, error) {
	value, e := c.manager.IsValue(left)
	if e != nil {
		return 0, e
	}
	if value {
		return 1, nil
	}
	value, e = c.manager.IsValue(right)
	if e != nil {
		return 0, e
	}
	if value {
		return -1, nil
	}
	a, e := c.uninterpreted(left)
	if e != nil {
		return 0, e
	}
	b, e := c.uninterpreted(right)
	if e != nil {
		return 0, e
	}
	if !a && b {
		return 1, nil
	}
	if a && !b {
		return -1, nil
	}
	if a && b {
		nA, e := c.manager.GetNumArgs(left)
		if e != nil {
			return 0, e
		}
		nB, e := c.manager.GetNumArgs(right)
		if e != nil {
			return 0, e
		}
		if nA < nB {
			return 1, nil
		}
		if nA > nB {
			return -1, nil
		}
		dA, e := c.manager.GetDecl(left)
		if e != nil {
			return 0, e
		}
		dB, e := c.manager.GetDecl(right)
		if e != nil {
			return 0, e
		}
		iA, e := c.manager.GetASTID(dA)
		if e != nil {
			return 0, e
		}
		iB, e := c.manager.GetASTID(dB)
		if e != nil {
			return 0, e
		}
		if iA < iB {
			return 1, nil
		}
		if iA > iB {
			return -1, nil
		}
	}
	dA, e := c.manager.GetDepth(left)
	if e != nil {
		return 0, e
	}
	dB, e := c.manager.GetDepth(right)
	if e != nil {
		return 0, e
	}
	if dA < dB {
		return 1, nil
	}
	if dA > dB {
		return -1, nil
	}
	return 0, nil
}

// classify returns owned large and small references when q supplies one of
// the source's admissible rewrite rules. The caller releases both references.
func (c demodulatorClassifier) classify(q ast.ASTHandle) (large, small ast.ASTRef, matched bool, err error) {
	if c.manager == nil {
		return large, small, false, ast.ErrASTHandle
	}
	kind, err := c.manager.GetKind(q)
	if err != nil || kind != ast.ASTQuantifier {
		return large, small, false, err
	}
	forall, err := c.manager.IsForall(q)
	if err != nil || !forall {
		return large, small, false, err
	}
	body, err := c.manager.GetQuantifierBody(q)
	if err != nil {
		return large, small, false, err
	}
	set := func(lhs, rhs ast.ASTHandle) (ast.ASTRef, ast.ASTRef, bool, error) {
		l, e := c.manager.NewRef(lhs)
		if e != nil {
			return ast.ASTRef{}, ast.ASTRef{}, false, e
		}
		s, e := c.manager.NewRef(rhs)
		if e != nil {
			return ast.ASTRef{}, ast.ASTRef{}, false, errors.Join(e, c.manager.DelRef(&l))
		}
		return l, s, true, nil
	}
	termKind, err := c.manager.GetKind(body)
	if err != nil {
		return large, small, false, err
	}
	if termKind == ast.ASTApp {
		family, e := c.manager.GetFamilyID(body)
		if e != nil {
			return large, small, false, e
		}
		if family == c.manager.BasicFamilyID() {
			op, e := c.manager.GetDeclKind(body)
			if e != nil {
				return large, small, false, e
			}
			if op == ast.ASTOpEq || op == ast.ASTOpIff {
				lhs, e := c.manager.GetArg(body, 0)
				if e != nil {
					return large, small, false, e
				}
				rhs, e := c.manager.GetArg(body, 1)
				if e != nil {
					return large, small, false, e
				}
				subset, e := c.subset(lhs, rhs)
				if e != nil {
					return large, small, false, e
				}
				smaller, e := c.smaller(lhs, rhs)
				if e != nil {
					return large, small, false, e
				}
				if e := c.traceTest(lhs, rhs, subset, smaller); e != nil {
					return large, small, false, e
				}
				if (subset == 1 || subset == 2) && smaller == 1 {
					if yes, e := c.uninterpreted(rhs); e != nil {
						return large, small, false, e
					} else if yes {
						return set(rhs, lhs)
					}
					if yes, inner, e := c.manager.IsNot(rhs); e != nil {
						return large, small, false, e
					} else if yes {
						if yes, e := c.uninterpreted(inner); e != nil {
							return large, small, false, e
						} else if yes {
							notLHS, e := c.manager.MkNot(lhs)
							if e != nil {
								return large, small, false, e
							}
							return set(inner, notLHS)
						}
					}
				}
				if (subset == -1 || subset == 2) && smaller == -1 {
					if yes, e := c.uninterpreted(lhs); e != nil {
						return large, small, false, e
					} else if yes {
						return set(lhs, rhs)
					}
					if yes, inner, e := c.manager.IsNot(lhs); e != nil {
						return large, small, false, e
					} else if yes {
						if yes, e := c.uninterpreted(inner); e != nil {
							return large, small, false, e
						} else if yes {
							notRHS, e := c.manager.MkNot(rhs)
							if e != nil {
								return large, small, false, e
							}
							return set(inner, notRHS)
						}
					}
				}
				return large, small, false, nil
			}
		}
	}
	if yes, inner, e := c.manager.IsNot(body); e != nil {
		return large, small, false, e
	} else if yes {
		if yes, e := c.uninterpreted(inner); e != nil {
			return large, small, false, e
		} else if yes {
			falsity, e := c.manager.MkFalse()
			if e != nil {
				return large, small, false, e
			}
			return set(inner, falsity)
		}
	}
	if yes, e := c.uninterpreted(body); e != nil {
		return large, small, false, e
	} else if yes {
		truth, e := c.manager.MkTrue()
		if e != nil {
			return large, small, false, e
		}
		return set(body, truth)
	}
	return large, small, false, nil
}
