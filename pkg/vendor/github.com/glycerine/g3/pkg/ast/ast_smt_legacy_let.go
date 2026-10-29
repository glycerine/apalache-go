package ast

// Source: src/ast/ast_smt_pp.cpp:536-735.

import "strings"

const legacySMTLineLength = uint32(80)

func (p *LegacySMTPrinter) letIDText(h ASTHandle) (string, error) {
	id, err := p.manager.GetASTID(h)
	if err != nil {
		return "", err
	}
	prefix := "?x"
	isProof, err := p.manager.IsProof(h)
	if err != nil {
		return "", err
	}
	if isProof {
		prefix = "@x"
	} else {
		sort, e := p.manager.GetSort(h)
		if e != nil {
			return "", e
		}
		boolSort, e := p.manager.MkBoolSort()
		if e != nil {
			return "", e
		}
		if sort == boolSort {
			prefix = "$x"
		}
	}
	return prefix + uintText(id), nil
}

func (p *LegacySMTPrinter) isSmallForLets(h ASTHandle, aliases map[ASTHandle]bool, size *uint32) (bool, error) {
	if *size > legacySMTLineLength {
		return false, nil
	}
	if aliases[h] {
		*size += 5
		return *size <= legacySMTLineLength, nil
	}
	kind, err := p.manager.GetKind(h)
	if err != nil {
		return false, err
	}
	switch kind {
	case ASTQuantifier:
		return false, nil
	case ASTVar:
		*size += 5
		return *size <= legacySMTLineLength, nil
	case ASTApp:
		decl, e := p.manager.GetDecl(h)
		if e != nil {
			return false, e
		}
		name, e := p.manager.GetASTName(decl)
		if e != nil {
			return false, e
		}
		if name.IsNumerical() {
			*size += 11
		} else {
			text, _, e := p.manager.Symbols.BareString(name)
			if e != nil {
				return false, e
			}
			*size += 3 + uint32(len(text))
		}
		n, e := p.manager.GetNumArgs(h)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n && *size <= legacySMTLineLength; i++ {
			*size++
			arg, e := p.manager.GetArg(h, i)
			if e != nil {
				return false, e
			}
			small, e := p.isSmallForLets(arg, aliases, size)
			if e != nil || !small {
				return false, e
			}
		}
		return *size <= legacySMTLineLength, nil
	default:
		return false, ErrASTDomain
	}
}

func (p *LegacySMTPrinter) isUnitForLets(h, top ASTHandle, aliases map[ASTHandle]bool) (bool, error) {
	refs, err := p.manager.GetRefCount(h)
	if err != nil {
		return false, err
	}
	if refs <= 2 {
		var size uint32
		small, e := p.isSmallForLets(h, aliases, &size)
		if e != nil {
			return false, e
		}
		if small {
			return true, nil
		}
	}
	if h == top {
		return true, nil
	}
	kind, err := p.manager.GetKind(h)
	if err != nil {
		return false, err
	}
	if kind == ASTVar {
		return true, nil
	}
	if kind == ASTApp {
		n, e := p.manager.GetNumArgs(h)
		return n == 0, e
	}
	return false, nil
}

func (p *LegacySMTPrinter) letOrder(root ASTHandle, aliases map[ASTHandle]bool) ([]ASTHandle, error) {
	kind, err := p.manager.GetKind(root)
	if err != nil {
		return nil, err
	}
	var todo []ASTHandle
	if kind == ASTApp {
		n, e := p.manager.GetNumArgs(root)
		if e != nil {
			return nil, e
		}
		for i := uint32(0); i < n; i++ {
			arg, e := p.manager.GetArg(root, i)
			if e != nil {
				return nil, e
			}
			todo = append(todo, arg)
		}
	}
	var order []ASTHandle
	for len(todo) != 0 {
		last := len(todo) - 1
		h := todo[last]
		if aliases[h] {
			todo = todo[:last]
			continue
		}
		unit, e := p.isUnitForLets(h, root, aliases)
		if e != nil {
			return nil, e
		}
		if unit {
			todo = todo[:last]
			continue
		}
		kind, e := p.manager.GetKind(h)
		if e != nil {
			return nil, e
		}
		pushed := false
		if kind == ASTApp {
			n, e := p.manager.GetNumArgs(h)
			if e != nil {
				return nil, e
			}
			for i := uint32(0); i < n; i++ {
				arg, e := p.manager.GetArg(h, i)
				if e != nil {
					return nil, e
				}
				childUnit, e := p.isUnitForLets(arg, root, aliases)
				if e != nil {
					return nil, e
				}
				if !childUnit && !aliases[arg] {
					todo = append(todo, arg)
					pushed = true
				}
			}
		}
		if pushed {
			continue
		}
		todo = todo[:last]
		aliases[h] = true
		order = append(order, h)
	}
	return order, nil
}

func (p *LegacySMTPrinter) exprTextWithLetsContext(root ASTHandle, ctx *legacySMTExprContext) (string, error) {
	aliases := make(map[ASTHandle]bool)
	order, err := p.letOrder(root, aliases)
	if err != nil {
		return "", err
	}
	active := make(map[ASTHandle]bool, len(aliases))
	ctx.aliases = active
	defer func() {
		ctx.aliases = nil
		ctx.defining = ASTHandle{}
	}()
	var out strings.Builder
	for _, h := range order {
		active[h] = true
		id, e := p.letIDText(h)
		if e != nil {
			return "", e
		}
		ctx.defining = h
		body, e := p.exprTextContext(h, ctx)
		ctx.defining = ASTHandle{}
		if e != nil {
			return "", e
		}
		out.WriteString("(let ((")
		out.WriteString(id)
		out.WriteByte(' ')
		out.WriteString(body)
		out.WriteString("))\n")
		if ctx.indent != 0 {
			out.WriteString(strings.Repeat(" ", int(ctx.indent)))
		}
	}
	body, err := p.exprTextContext(root, ctx)
	if err != nil {
		return "", err
	}
	out.WriteString(body)
	out.WriteString(strings.Repeat(")", len(order)))
	return out.String(), nil
}
