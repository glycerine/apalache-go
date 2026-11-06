package rewriter

// Source: src/ast/rewriter/bit2int.cpp:35-49,237-420;
// src/ast/rewriter/bit2int.h:35-51,88-90.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (b *Bit2Int) cached(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	ref, ok := b.cache[term]
	if !ok {
		return ast.ASTHandle{}, false, nil
	}
	h, err := b.manager.RefGet(ref)
	return h, true, err
}

func (b *Bit2Int) rewrittenArguments(term ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := b.manager.GetNumArgs(term)
	if err != nil {
		return nil, err
	}
	args := make([]ast.ASTHandle, count)
	for i := range args {
		original, err := b.manager.GetArg(term, uint32(i))
		if err != nil {
			return nil, err
		}
		value, found, err := b.cached(original)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, ast.ErrASTPending
		}
		args[i] = value
	}
	return args, nil
}

func (b *Bit2Int) fallbackApplication(term ast.ASTHandle, args []ast.ASTHandle) error {
	decl, err := b.manager.GetDecl(term)
	if err != nil {
		return err
	}
	result, err := b.manager.RewriterMakeApp(decl, ast.SliceArguments(args))
	if err != nil {
		return err
	}
	return b.cacheResult(term, result)
}

func (b *Bit2Int) applicationIdentity(term ast.ASTHandle) (ast.FamilyID, int32, error) {
	family, err := b.manager.GetFamilyID(term)
	if err != nil {
		return ast.NullFamilyID, -1, err
	}
	kind, err := b.manager.GetDeclKind(term)
	return family, kind, err
}

func (b *Bit2Int) visitQuantifier(term ast.ASTHandle) error {
	body, err := b.manager.GetQuantifierBody(term)
	if err != nil {
		return err
	}
	result, found, err := b.cached(body)
	if err != nil {
		return err
	}
	if !found {
		return ast.ErrASTPending
	}
	result, err = b.manager.UpdateQuantifierBody(term, result)
	if err != nil {
		return err
	}
	return b.cacheResult(term, result)
}

func (b *Bit2Int) visitApplication(term ast.ASTHandle) (result error) {
	args, err := b.rewrittenArguments(term)
	if err != nil {
		return err
	}
	family, kind, err := b.applicationIdentity(term)
	if err != nil {
		return err
	}
	hasBV2Int := family == b.arithFamily && (kind == 2 || kind == 3 || kind == 4 || kind == 5) || family == b.manager.BasicFamilyID() && kind == ast.ASTOpEq
	for _, arg := range args {
		if hasBV2Int {
			break
		}
		hasBV2Int, err = b.manager.IsBitVectorOperation(arg, 58)
		if err != nil {
			return err
		}
	}
	if !hasBV2Int {
		return b.fallbackApplication(term, args)
	}
	call := bit2IntCall{owner: b}
	defer func() { result = errors.Join(result, call.close()) }()
	publish := func(value ast.ASTHandle) error { return b.cacheResult(term, value) }
	fallback := func() error { return b.fallbackApplication(term, args) }

	if family == b.arithFamily && kind == 6 && len(args) >= 1 {
		value := args[0]
		for i := 1; i < len(args); i++ {
			value, hasBV2Int, err = b.mkAdd(&call, value, args[i])
			if err != nil {
				return err
			}
			if !hasBV2Int {
				return fallback()
			}
		}
		return publish(value)
	}
	if family == b.arithFamily && kind == 9 && len(args) >= 1 {
		value := args[0]
		for i := 1; i < len(args); i++ {
			value, hasBV2Int, err = b.mkMul(&call, value, args[i])
			if err != nil {
				return err
			}
			if !hasBV2Int {
				return fallback()
			}
		}
		return publish(value)
	}
	comparisonKind := family == b.manager.BasicFamilyID() && kind == ast.ASTOpEq || family == b.arithFamily && kind >= 2 && kind <= 5
	if comparisonKind && len(args) >= 2 {
		leftPos, leftNeg, leftPoly, err := b.isBVPoly(&call, args[0])
		if err != nil {
			return err
		}
		rightPos, rightNeg, rightPoly, err := b.isBVPoly(&call, args[1])
		if err != nil {
			return err
		}
		if leftPoly && rightPoly {
			left, ok, err := b.mkAdd(&call, leftPos, rightNeg)
			if err != nil {
				return err
			}
			right, ok2, err := b.mkAdd(&call, leftNeg, rightPos)
			if err != nil {
				return err
			}
			if ok && ok2 {
				mode, reverse, comparison := bit2IntEQ, false, false
				switch {
				case family == b.manager.BasicFamilyID() && kind == ast.ASTOpEq:
					comparison = true
				case family == b.arithFamily && kind == 2:
					mode, comparison = bit2IntLE, true
				case family == b.arithFamily && kind == 3:
					mode, reverse, comparison = bit2IntLE, true, true
				case family == b.arithFamily && kind == 4:
					mode, comparison = bit2IntLT, true
				case family == b.arithFamily && kind == 5:
					mode, reverse, comparison = bit2IntLT, true, true
				}
				if comparison {
					if reverse {
						left, right = right, left
					}
					value, ok, err := b.mkComp(&call, mode, left, right)
					if err != nil {
						return err
					}
					if ok {
						return publish(value)
					}
				}
			}
		}
	}
	if family == b.arithFamily && kind == 13 && len(args) >= 2 {
		value, ok, err := b.rewriteMod(&call, args[0], args[1])
		if err != nil {
			return err
		}
		if ok {
			return publish(value)
		}
	}
	return fallback()
}

func (b *Bit2Int) rewriteMod(c *bit2IntCall, dividend, divisor ast.ASTHandle) (ast.ASTHandle, bool, error) {
	pos, neg, poly, err := b.isBVPoly(c, dividend)
	if err != nil || !poly {
		return ast.ASTHandle{}, false, err
	}
	d, divisorOK, err := b.extractBV(c, divisor)
	if err != nil || !divisorOK || d.sign {
		return ast.ASTHandle{}, false, err
	}
	p, posOK, err := b.extractBV(c, pos)
	if err != nil || !posOK {
		return ast.ASTHandle{}, false, err
	}
	n, negOK, err := b.extractBV(c, neg)
	if err != nil || !negOK || p.sign || n.sign {
		return ast.ASTHandle{}, false, err
	}
	zero, err := b.manager.IsBitVectorZero(n.bits)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if zero {
		left, right, err := b.alignSizes(c, p.bits, d.bits)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		rem, err := b.simplifyBVURem(c, left, right)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		value, err := b.mkBV2Int(c, rem)
		return value, err == nil, err
	}
	negative, modulus, err := b.alignSizes(c, n.bits, d.bits)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	negativeRem, err := b.simplifyBVURem(c, negative, modulus)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	modulus, negativeRem, err = b.alignSizes(c, d.bits, negativeRem)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	complement, err := b.simplifyBVSub(c, modulus, negativeRem)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	positive, complement, err := b.alignSizes(c, p.bits, complement)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	width, err := b.manager.BitVectorSize(positive)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	positive, err = b.alignSize(c, positive, width+1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	complement, err = b.alignSize(c, complement, width+1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	sum, err := b.simplifyBVAdd(c, positive, complement)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	sum, modulus, err = b.alignSizes(c, sum, d.bits)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	rem, err := b.simplifyBVURem(c, sum, modulus)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	value, err := b.mkBV2Int(c, rem)
	return value, err == nil, err
}

type bit2IntFrame struct {
	term     ast.ASTHandle
	expanded bool
}

func (b *Bit2Int) rewrite(root ast.ASTHandle, out ast.ASTRef) (result error) {
	if err := b.open(); err != nil {
		return err
	}
	if err := b.flushCache(); err != nil {
		return err
	}
	stack := []bit2IntFrame{{term: root}}
	for len(stack) != 0 {
		last := len(stack) - 1
		frame := stack[last]
		stack = stack[:last]
		if _, found, err := b.cached(frame.term); err != nil {
			return err
		} else if found {
			continue
		}
		kind, err := b.manager.GetKind(frame.term)
		if err != nil {
			return err
		}
		if !frame.expanded {
			switch kind {
			case ast.ASTVar:
				if err = b.cacheResult(frame.term, frame.term); err != nil {
					return err
				}
				continue
			case ast.ASTApp:
				stack = append(stack, bit2IntFrame{term: frame.term, expanded: true})
				count, err := b.manager.GetNumArgs(frame.term)
				if err != nil {
					return err
				}
				for i := count; i > 0; {
					i--
					arg, err := b.manager.GetArg(frame.term, i)
					if err != nil {
						return err
					}
					stack = append(stack, bit2IntFrame{term: arg})
				}
				continue
			case ast.ASTQuantifier:
				stack = append(stack, bit2IntFrame{term: frame.term, expanded: true})
				body, err := b.manager.GetQuantifierBody(frame.term)
				if err != nil {
					return err
				}
				stack = append(stack, bit2IntFrame{term: body})
				continue
			default:
				return ast.ErrASTDomain
			}
		}
		if kind == ast.ASTApp {
			err = b.visitApplication(frame.term)
		} else {
			err = b.visitQuantifier(frame.term)
		}
		if err != nil {
			return err
		}
	}
	value, found, err := b.cached(root)
	if err != nil {
		return err
	}
	if !found {
		return ast.ErrASTPending
	}
	return b.manager.SetRef(out, value)
}

func (b *Bit2Int) Rewrite(root ast.ASTHandle, out ast.ASTRef) error {
	return b.rewrite(root, out)
}

func (b *Bit2Int) RewriteWithProof(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if err := b.rewrite(root, out); err != nil {
		return err
	}
	proofs, err := b.manager.ProofsEnabled()
	if err != nil || !proofs {
		return err
	}
	value, err := b.manager.RefGet(out)
	if err != nil || value == root {
		return err
	}
	p, err := b.manager.MkProofRewrite(root, value)
	if err != nil {
		return err
	}
	return b.manager.SetRef(proof, p)
}
