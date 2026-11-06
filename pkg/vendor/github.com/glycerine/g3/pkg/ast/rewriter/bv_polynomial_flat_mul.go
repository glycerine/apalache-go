package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:140-194.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVPolynomialMultiplicationCore) flatBody(sort ast.ASTHandle, args ast.AppArguments, out ast.ASTRef, o BVPolynomialMultiplicationOptions) (RewriteStatus, error) {
	m := p.manager
	if args.Count() < 2 {
		return RewriteFailed, ast.ErrASTDomain
	}
	flatten := args.Count() != 2
	if !flatten {
		first, e := args.At(0)
		if e != nil {
			return RewriteFailed, e
		}
		numeral, e := m.IsBitVectorNumeral(first)
		if e != nil {
			return RewriteFailed, e
		}
		flatten = !numeral
		if numeral {
			second, e := args.At(1)
			if e != nil {
				return RewriteFailed, e
			}
			mul, e := m.IsBitVectorOperation(second, 6)
			if e != nil {
				return RewriteFailed, e
			}
			if mul {
				head, e := m.GetArg(second, 0)
				if e != nil {
					return RewriteFailed, e
				}
				flatten, e = m.IsBitVectorNumeral(head)
				if e != nil {
					return RewriteFailed, e
				}
			}
		}
	}
	if !flatten {
		return p.body(sort, args, out, o)
	}
	i := uint32(0)
	for ; i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return RewriteFailed, e
		}
		mul, e := m.IsBitVectorOperation(arg, 6)
		if e != nil {
			return RewriteFailed, e
		}
		if mul {
			break
		}
	}
	if i == args.Count() {
		return p.body(sort, args, out, o)
	}
	for j := uint32(0); j < i; j++ {
		arg, e := args.At(j)
		if e != nil {
			return RewriteFailed, e
		}
		p.flatArgs = append(p.flatArgs, arg)
	}
	for j := i; j < args.Count(); j++ {
		arg, e := args.At(j)
		if e != nil {
			return RewriteFailed, e
		}
		mul, e := m.IsBitVectorOperation(arg, 6)
		if e != nil {
			return RewriteFailed, e
		}
		if !mul {
			p.flatArgs = append(p.flatArgs, arg)
			continue
		}
		p.todo = append(p.todo, arg)
		for len(p.todo) != 0 {
			stack := p.todo
			curr := stack[len(stack)-1]
			p.todo[len(p.todo)-1] = ast.ASTHandle{}
			p.todo = p.todo[:len(p.todo)-1]
			mul, e := m.IsBitVectorOperation(curr, 6)
			if e != nil {
				return RewriteFailed, e
			}
			if mul {
				k, e := m.GetNumArgs(curr)
				if e != nil {
					return RewriteFailed, e
				}
				for k > 0 {
					k--
					h, e := m.GetArg(curr, k)
					if e != nil {
						return RewriteFailed, e
					}
					p.todo = append(p.todo, h)
				}
			} else {
				p.flatArgs = append(p.flatArgs, curr)
			}
		}
	}
	flat := ast.SliceArguments(p.flatArgs)
	st, e := p.body(sort, flat, out, o)
	if e != nil {
		return st, e
	}
	if e = p.sourceEnd(); e != nil {
		return st, e
	}
	if st != RewriteFailed {
		return st, nil
	}
	node, e := m.MkBitVectorPolynomialMulWithInvocation(sort, flat, &p.factory)
	if e != nil {
		return RewriteFailed, e
	}
	return RewriteDone, m.SetRef(out, node)
}
