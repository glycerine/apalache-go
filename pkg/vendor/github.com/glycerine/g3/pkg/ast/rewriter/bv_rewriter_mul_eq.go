package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2448-2508.

import "github.com/glycerine/g3/pkg/ast"

func bvBinaryMultiplication(m *ast.ASTManager, arg ast.ASTHandle) (yes bool, coefficient, term ast.ASTHandle, result error) {
	yes, result = m.IsBitVectorOperation(arg, 6)
	if result != nil || !yes {
		return
	}
	count, e := m.GetNumArgs(arg)
	if e != nil {
		return false, coefficient, term, e
	}
	if count != 2 {
		return false, coefficient, term, nil
	}
	coefficient, result = m.GetArg(arg, 0)
	if result != nil {
		return
	}
	term, result = m.GetArg(arg, 1)
	return
}

func (p *BVPolynomialRewriter) RewriteMultiplicationEquality(lhs, rhs ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() {
		p.running = false
		if err != nil {
			p.fail(err)
		}
	}()
	m := p.manager
	n, e := ast.NewBitVectorMulEqualityNumerals(m)
	p.mulEquality = n
	if e != nil {
		return st, e
	}
	finish := func() (RewriteStatus, error) {
		if e := n.SourceEnd(); e != nil {
			return st, e
		}
		if e := n.Close(); e != nil {
			return st, e
		}
		p.mulEquality = nil
		return st, nil
	}
	assign := func(left, right ast.ASTHandle, status RewriteStatus) error {
		h, e := m.MkEq(left, right)
		if e != nil {
			return e
		}
		st = status
		return m.SetRef(out, h)
	}
	productApp := func(a, b ast.ASTHandle) (ast.ASTHandle, error) {
		p.pair[0] = a
		p.pair[1] = b
		return m.MkBitVectorPolynomialMul(ast.ASTHandle{}, ast.SliceArguments(p.pair[:]))
	}
	width := uint32(0)
	binary, c, x, e := bvBinaryMultiplication(m, lhs)
	if e != nil {
		return st, e
	}
	invertible := false
	if binary {
		matched, e := n.Read(0, c, &width)
		if e != nil {
			return st, e
		}
		if matched {
			invertible, e = n.Inverse(0, 1, width)
			if e != nil {
				return st, e
			}
		}
	}
	if invertible {
		if ast.ASTSourceDebug {
			if e = n.Product(0, 1); e != nil {
				return st, e
			}
			if e = n.NormReturn(8, width); e != nil {
				return st, e
			}
			one, e := n.IsOne(8)
			if e != nil {
				return st, e
			}
			if !one {
				return st, ast.ErrASTDomain
			}
			if e = n.End(8, 7); e != nil {
				return st, e
			}
		}
		if e = n.Place(2); e != nil {
			return st, e
		}
		matched, e := n.Read(2, rhs, &width)
		if e != nil {
			return st, e
		}
		if matched {
			if e = n.Product(1, 2); e != nil {
				return st, e
			}
			constant, e := n.Constant(7, width)
			if e != nil {
				return st, e
			}
			if e = assign(x, constant, Rewrite1); e != nil {
				return st, e
			}
			if e = n.End(7); e != nil {
				return st, e
			}
			return finish()
		}
		if e = n.Place(3); e != nil {
			return st, e
		}
		binary, c2, x2, e := bvBinaryMultiplication(m, rhs)
		if e != nil {
			return st, e
		}
		if binary {
			matched, e := n.Read(3, c2, &width)
			if e != nil {
				return st, e
			}
			if matched {
				if e = n.Product(1, 3); e != nil {
					return st, e
				}
				if e = n.NormReturn(4, width); e != nil {
					return st, e
				}
				if e = n.End(7); e != nil {
					return st, e
				}
				one, e := n.IsOne(4)
				if e != nil {
					return st, e
				}
				right := x2
				if !one {
					// Source deliberately recomputes the product instead of reusing new_c2.
					if e = n.Product(1, 3); e != nil {
						return st, e
					}
					constant, e := n.Constant(7, width)
					if e != nil {
						return st, e
					}
					right, e = productApp(constant, x2)
					if e != nil {
						return st, e
					}
				}
				if e = assign(x, right, Rewrite1); e != nil {
					return st, e
				}
				if e = n.End(7); e != nil {
					return st, e
				}
				return finish()
			}
		}
		constantTerms, e := IsBitVectorAddMulConstant(m, rhs)
		if e != nil {
			return st, e
		}
		if constantTerms {
			constant, e := n.Constant(1, width)
			if e != nil {
				return st, e
			}
			right, e := productApp(constant, rhs)
			if e != nil {
				return st, e
			}
			if e = assign(x, right, Rewrite2); e != nil {
				return st, e
			}
			return finish()
		}
		if e = n.End(3, 2); e != nil {
			return st, e
		}
	}
	matched, e := n.Read(0, lhs, &width)
	if e != nil {
		return st, e
	}
	if matched {
		constantTerms, e := IsBitVectorAddMulConstant(m, rhs)
		if e != nil {
			return st, e
		}
		if constantTerms {
			if e = n.Place(5); e != nil {
				return st, e
			}
			if e = n.Place(6); e != nil {
				return st, e
			}
			args, e := m.AppArgumentView(rhs)
			if e != nil {
				return st, e
			}
			found := false
			for i := uint32(0); !found && i < args.Count(); i++ {
				arg, e := args.At(i)
				if e != nil {
					return st, e
				}
				binary, coefficient, _, e := bvBinaryMultiplication(m, arg)
				if e != nil {
					return st, e
				}
				if !binary {
					continue
				}
				matched, e := n.Read(5, coefficient, &width)
				if e != nil {
					return st, e
				}
				if matched {
					found, e = n.Inverse(5, 6, width)
					if e != nil {
						return st, e
					}
				}
			}
			if found {
				if e = n.Product(6, 0); e != nil {
					return st, e
				}
				left, e := n.Constant(7, width)
				if e != nil {
					return st, e
				}
				coefficient, e := n.Constant(6, width)
				if e != nil {
					return st, e
				}
				right, e := productApp(coefficient, rhs)
				if e != nil {
					return st, e
				}
				if e = assign(left, right, Rewrite3); e != nil {
					return st, e
				}
				if e = n.End(7); e != nil {
					return st, e
				}
				return finish()
			}
		}
	}
	return finish()
}
