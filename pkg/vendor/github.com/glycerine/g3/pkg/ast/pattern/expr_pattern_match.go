package pattern

// Source: src/ast/pattern/expr_pattern_match.cpp:40-381;
// src/ast/pattern/expr_pattern_match.h:24-145.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type patternMatchInstructionKind uint8

const (
	patternBacktrack patternMatchInstructionKind = iota
	patternBind
	patternBindAC
	patternBindC
	patternChooseAC
	patternChooseC
	patternSetVar
	patternCheckVar
	patternCheckTerm
	patternSetBound
	patternCheckBound
	patternYield
)

type patternMatchInstruction struct {
	kind                 patternMatchInstructionKind
	offset, next         uint32
	app, pattern         ast.ASTHandle
	reg, otherReg, count uint32
	numBound             uint32
}

// exprPatternMatcher owns the precompiled database quantifiers. Instructions,
// registers and substitutions contain concrete handles or indices only.
type exprPatternMatcher struct {
	self              *exprPatternMatcher
	manager           *ast.ASTManager
	precompiled       []ast.ASTHandle
	firstInstructions []uint32
	instructions      []patternMatchInstruction
	registers         []ast.ASTHandle
	boundDomain       []ast.ASTHandle
	boundRange        []ast.ASTHandle
	closed            bool
}

func newExprPatternMatcher(m *ast.ASTManager) (*exprPatternMatcher, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	p := &exprPatternMatcher{manager: m}
	p.self = p
	return p, nil
}

func (p *exprPatternMatcher) open() error {
	if p == nil || p.self != p || p.closed || p.manager == nil {
		return ast.ErrASTHandle
	}
	_, err := p.manager.ProofsEnabled()
	return err
}

func (p *exprPatternMatcher) Close() error {
	if p == nil || p.self != p || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	for len(p.precompiled) != 0 {
		i := len(p.precompiled) - 1
		h := p.precompiled[i]
		p.precompiled = p.precompiled[:i]
		if err := p.manager.DecRef(h); err != nil {
			p.precompiled = append(p.precompiled, h)
			return err
		}
	}
	p.firstInstructions = nil
	p.instructions = nil
	p.registers = nil
	p.boundDomain = nil
	p.boundRange = nil
	p.closed = true
	return nil
}

func (p *exprPatternMatcher) isVariableDeclaration(d ast.ASTHandle) (bool, error) {
	name, err := p.manager.GetASTName(d)
	if err != nil {
		return false, err
	}
	text, present, err := p.manager.Symbols.BareString(name)
	return present && len(text) != 0 && text[0] == '?', err
}

func (p *exprPatternMatcher) Compile(q ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	kind, err := p.manager.GetKind(q)
	if err != nil || kind != ast.ASTQuantifier {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	if len(p.instructions) == 0 {
		p.instructions = append(p.instructions, patternMatchInstruction{kind: patternBacktrack})
	}
	body, err := p.manager.GetQuantifierBody(q)
	if err != nil {
		return err
	}
	first := uint32(len(p.instructions))
	type pending struct {
		reg uint32
		pat ast.ASTHandle
	}
	work := []pending{{0, body}}
	maxReg := uint32(1)
	numBound := uint32(0)
	bound := make(map[ast.ASTHandle]uint32)
	substitution := make(map[ast.ASTHandle]uint32)
	compiled := make([]patternMatchInstruction, 0)
	for len(work) != 0 {
		i := len(work) - 1
		item := work[i]
		work = work[:i]
		instruction := patternMatchInstruction{pattern: item.pat, reg: item.reg,
			next: first + uint32(len(compiled)) + 1, offset: maxReg}
		kind, e := p.manager.GetKind(item.pat)
		if e != nil {
			return e
		}
		switch kind {
		case ast.ASTVar:
			if index, found := bound[item.pat]; found {
				instruction.kind, instruction.numBound = patternCheckBound, index
			} else {
				instruction.kind, instruction.numBound = patternSetBound, numBound
				bound[item.pat] = numBound
				numBound++
			}
		case ast.ASTApp:
			d, e := p.manager.GetDecl(item.pat)
			if e != nil {
				return e
			}
			instruction.app = item.pat
			n, e := p.manager.GetNumArgs(item.pat)
			if e != nil {
				return e
			}
			for j := uint32(0); j < n; j++ {
				arg, e := p.manager.GetArg(item.pat, j)
				if e != nil {
					return e
				}
				work = append(work, pending{maxReg, arg})
				maxReg++
			}
			variable, e := p.isVariableDeclaration(d)
			if e != nil {
				return e
			}
			if variable {
				if other, found := substitution[d]; found {
					instruction.kind, instruction.otherReg = patternCheckVar, other
				} else {
					instruction.kind = patternSetVar
					substitution[d] = item.reg
				}
			} else {
				associative, e := p.manager.RewriterAssociative(d)
				if e != nil {
					return e
				}
				commutative, e := p.manager.RewriterCommutative(d)
				if e != nil {
					return e
				}
				switch {
				case associative && commutative:
					instruction.kind = patternBindAC
				case commutative:
					if n != 2 {
						return ast.ErrASTDomain
					}
					instruction.kind = patternBindC
				default:
					instruction.kind = patternBind
				}
			}
		default:
			instruction.kind = patternCheckTerm
		}
		compiled = append(compiled, instruction)
	}
	compiled = append(compiled, patternMatchInstruction{kind: patternYield})
	if err = p.manager.IncRef(q); err != nil {
		return err
	}
	p.precompiled = append(p.precompiled, q)
	p.firstInstructions = append(p.firstInstructions, first)
	p.instructions = append(p.instructions, compiled...)
	if need := int(maxReg + 1); len(p.registers) < need {
		p.registers = append(p.registers, make([]ast.ASTHandle, need-len(p.registers))...)
	}
	if need := int(numBound + 1); len(p.boundDomain) < need {
		p.boundDomain = append(p.boundDomain, make([]ast.ASTHandle, need-len(p.boundDomain))...)
		p.boundRange = append(p.boundRange, make([]ast.ASTHandle, need-len(p.boundRange))...)
	}
	return nil
}

func (p *exprPatternMatcher) matchDeclaration(pattern, actual ast.ASTHandle) (result bool, err error) {
	if pattern == actual {
		return true, nil
	}
	pa, err := p.manager.GetArity(pattern)
	if err != nil {
		return false, err
	}
	aa, err := p.manager.GetArity(actual)
	if err != nil || pa != aa {
		return false, err
	}
	pf, err := p.manager.GetFamilyID(pattern)
	if err != nil || pf == ast.NullFamilyID {
		return false, err
	}
	af, err := p.manager.GetFamilyID(actual)
	if err != nil || af != pf {
		return false, err
	}
	pk, err := p.manager.GetDeclKind(pattern)
	if err != nil {
		return false, err
	}
	ak, err := p.manager.GetDeclKind(actual)
	if err != nil || ak != pk {
		return false, err
	}
	pp, err := p.manager.GetDeclParametersCopy(pattern)
	if err != nil {
		return false, err
	}
	ap, err := p.manager.GetDeclParametersCopy(actual)
	if err != nil {
		for i := range pp {
			_ = p.manager.DelParameter(&pp[i])
		}
		return false, err
	}
	defer func() {
		for i := range ap {
			err = errors.Join(err, p.manager.DelParameter(&ap[i]))
		}
		for i := range pp {
			err = errors.Join(err, p.manager.DelParameter(&pp[i]))
		}
	}()
	if len(pp) != len(ap) {
		return false, nil
	}
	for i := range pp {
		equal, e := p.manager.ParameterEqual(pp[i], ap[i])
		if e != nil || !equal {
			return false, e
		}
	}
	return true, nil
}

func (p *exprPatternMatcher) match(root ast.ASTHandle, initial uint32, substitution map[ast.ASTHandle]uint32) (bool, error) {
	if uint64(initial) >= uint64(len(p.instructions)) {
		return false, ast.ErrASTDomain
	}
	p.registers[0] = root
	pc := p.instructions[initial]
	backtrack := make([]patternMatchInstruction, 0)
	for {
		ok := false
		switch pc.kind {
		case patternYield:
			return true, nil
		case patternCheckTerm:
			ok = pc.pattern == p.registers[pc.reg]
		case patternSetVar, patternCheckVar:
			patternDecl, e := p.manager.GetDecl(pc.pattern)
			if e != nil {
				return false, e
			}
			actual := p.registers[pc.reg]
			kind, e := p.manager.GetKind(actual)
			if e != nil {
				return false, e
			}
			if kind == ast.ASTApp {
				pn, e := p.manager.GetNumArgs(pc.pattern)
				if e != nil {
					return false, e
				}
				an, e := p.manager.GetNumArgs(actual)
				if e != nil {
					return false, e
				}
				ok = pn == an
				if ok && pc.kind == patternCheckVar {
					left, e := p.manager.GetDecl(actual)
					if e != nil {
						return false, e
					}
					other, e := p.manager.GetDecl(p.registers[pc.otherReg])
					if e != nil {
						return false, e
					}
					ok = left == other
				}
				if ok {
					for i := uint32(0); i < an; i++ {
						p.registers[pc.offset+i], e = p.manager.GetArg(actual, i)
						if e != nil {
							return false, e
						}
					}
					if pc.kind == patternSetVar {
						substitution[patternDecl] = pc.reg
					}
				}
			}
		case patternSetBound:
			actual := p.registers[pc.reg]
			kind, e := p.manager.GetKind(actual)
			if e != nil {
				return false, e
			}
			if kind == ast.ASTVar {
				ok = true
				for i := uint32(0); i < pc.numBound; i++ {
					if actual == p.boundRange[i] {
						ok = false
						break
					}
				}
				if ok {
					p.boundDomain[pc.numBound], p.boundRange[pc.numBound] = pc.pattern, actual
				}
			}
		case patternCheckBound:
			ok = p.boundRange[pc.numBound] == p.registers[pc.reg]
		case patternBind, patternBindAC, patternBindC:
			actual := p.registers[pc.reg]
			kind, e := p.manager.GetKind(actual)
			if e != nil {
				return false, e
			}
			if kind == ast.ASTApp {
				pn, e := p.manager.GetNumArgs(pc.pattern)
				if e != nil {
					return false, e
				}
				an, e := p.manager.GetNumArgs(actual)
				if e != nil {
					return false, e
				}
				if pn == an {
					pd, e := p.manager.GetDecl(pc.pattern)
					if e != nil {
						return false, e
					}
					ad, e := p.manager.GetDecl(actual)
					if e != nil {
						return false, e
					}
					matched, e := p.matchDeclaration(pd, ad)
					if e != nil {
						return false, e
					}
					if matched {
						switch pc.kind {
						case patternBind:
							ok = true
							for i := uint32(0); i < an; i++ {
								p.registers[pc.offset+i], e = p.manager.GetArg(actual, i)
								if e != nil {
									return false, e
								}
							}
						case patternBindAC:
							backtrack = append(backtrack, patternMatchInstruction{kind: patternChooseAC, offset: pc.offset, next: pc.next, app: actual, count: 1})
						case patternBindC:
							ok = true
							p.registers[pc.offset], e = p.manager.GetArg(actual, 0)
							if e != nil {
								return false, e
							}
							p.registers[pc.offset+1], e = p.manager.GetArg(actual, 1)
							if e != nil {
								return false, e
							}
							backtrack = append(backtrack, patternMatchInstruction{kind: patternChooseC, offset: pc.offset, next: pc.next, app: actual, count: 2})
						}
					}
				}
			}
		case patternChooseC:
			ok = true
			var e error
			p.registers[pc.offset+1], e = p.manager.GetArg(pc.app, 0)
			if e != nil {
				return false, e
			}
			p.registers[pc.offset], e = p.manager.GetArg(pc.app, 1)
			if e != nil {
				return false, e
			}
		case patternChooseAC:
			ok = true
			n, e := p.manager.GetNumArgs(pc.app)
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < n; i++ {
				p.registers[pc.offset+i], e = p.manager.GetArg(pc.app, i)
				if e != nil {
					return false, e
				}
			}
			k, factorial := pc.count, uint32(1)
			for j := uint32(2); j <= n; j++ {
				factorial *= j - 1
				other := pc.offset + j - ((k / factorial) % j) - 1
				last := pc.offset + j - 1
				p.registers[last], p.registers[other] = p.registers[other], p.registers[last]
			}
			if k < factorial*n {
				pc.count++
				backtrack = append(backtrack, pc)
			}
		case patternBacktrack:
			if len(backtrack) == 0 {
				return false, nil
			}
			pc = backtrack[len(backtrack)-1]
			backtrack = backtrack[:len(backtrack)-1]
			continue
		default:
			return false, ast.ErrASTDomain
		}
		if ok {
			if uint64(pc.next) >= uint64(len(p.instructions)) {
				return false, ast.ErrASTDomain
			}
			pc = p.instructions[pc.next]
		} else {
			pc = p.instructions[0]
		}
	}
}

func (p *exprPatternMatcher) instantiate(h ast.ASTHandle, substitution map[ast.ASTHandle]uint32, bound map[ast.ASTHandle]ast.ASTHandle, memo map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	if value, found := memo[h]; found {
		return value, nil
	}
	kind, err := p.manager.GetKind(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch kind {
	case ast.ASTVar:
		value, found := bound[h]
		if !found {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		memo[h] = value
		return value, nil
	case ast.ASTApp:
		d, err := p.manager.GetDecl(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if reg, found := substitution[d]; found {
			d, err = p.manager.GetDecl(p.registers[reg])
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		n, err := p.manager.GetNumArgs(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args := make([]ast.ASTHandle, n)
		for i := uint32(0); i < n; i++ {
			child, e := p.manager.GetArg(h, i)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			args[i], e = p.instantiate(child, substitution, bound, memo)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		pattern, err := p.manager.IsPattern(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		var result ast.ASTHandle
		if pattern {
			result, err = p.manager.MkPattern(args)
		} else {
			result, err = p.manager.MkApp(d, args)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		memo[h] = result
		return result, nil
	default:
		memo[h] = h
		return h, nil
	}
}

func (p *exprPatternMatcher) MatchQuantifier(q ast.ASTHandle) (patterns []ast.ASTRef, weight uint32, matched bool, err error) {
	if err = p.open(); err != nil {
		return nil, 0, false, err
	}
	defer func() {
		if err == nil {
			return
		}
		for i := range patterns {
			err = errors.Join(err, p.manager.DelRef(&patterns[i]))
		}
		patterns = nil
	}()
	if len(p.registers) == 0 {
		return nil, 0, false, nil
	}
	forall, err := p.manager.IsForall(q)
	if err != nil {
		return nil, 0, false, err
	}
	declarations, err := p.manager.GetNumDeclarations(q)
	if err != nil {
		return nil, 0, false, err
	}
	body, err := p.manager.GetQuantifierBody(q)
	if err != nil {
		return nil, 0, false, err
	}
	for i, compiled := range p.precompiled {
		compiledForall, e := p.manager.IsForall(compiled)
		if e != nil {
			return nil, 0, false, e
		}
		compiledDeclarations, e := p.manager.GetNumDeclarations(compiled)
		if e != nil {
			return nil, 0, false, e
		}
		if compiledForall != forall || compiledDeclarations != declarations {
			continue
		}
		substitution := make(map[ast.ASTHandle]uint32)
		ok, e := p.match(body, p.firstInstructions[i], substitution)
		if e != nil {
			return nil, 0, false, e
		}
		if !ok {
			continue
		}
		bound := make(map[ast.ASTHandle]ast.ASTHandle, declarations)
		for j := uint32(0); j < declarations; j++ {
			bound[p.boundDomain[j]] = p.boundRange[j]
		}
		n, e := p.manager.GetNumPatterns(compiled)
		if e != nil {
			return nil, 0, false, e
		}
		for j := uint32(0); j < n; j++ {
			pattern, e := p.manager.GetPattern(compiled, j)
			if e != nil {
				return nil, 0, false, e
			}
			value, e := p.instantiate(pattern, substitution, bound, make(map[ast.ASTHandle]ast.ASTHandle))
			if e != nil {
				return nil, 0, false, e
			}
			ref, e := p.manager.NewRef(value)
			if e != nil {
				return nil, 0, false, e
			}
			patterns = append(patterns, ref)
		}
		w, e := p.manager.GetQuantifierWeight(compiled)
		if e != nil {
			return nil, 0, false, e
		}
		return patterns, uint32(w), true, nil
	}
	return nil, 0, false, nil
}
