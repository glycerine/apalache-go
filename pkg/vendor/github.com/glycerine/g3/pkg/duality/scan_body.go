package duality

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

// ScannedBody owns the rewritten formula and positional relation declarations.
// Children are borrowed from the source graph in the same order as Parameters.
type ScannedBody struct {
	Term             *Expr
	Parameters       []*FuncDecl
	Children         []*Node
	QuantifiedTheory bool
}

func (s *ScannedBody) Close() error {
	if s == nil {
		return nil
	}
	var result error
	if s.Term != nil {
		if err := s.Term.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.Term = nil
		}
	}
	for i, p := range s.Parameters {
		if p == nil {
			continue
		}
		if err := p.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.Parameters[i] = nil
		}
	}
	return result
}

// ScanBody replaces each distinct predicate application by a positional
// relation parameter. Shared applications share one parameter; different
// applications of the same predicate produce repeated child positions.
// Quantifier patterns are left untouched, as in the source FromClauses path.
// Source: duality_rpfp.cpp, ScanBody and SuffixFuncDecl.
func (g *RPFP) ScanBody(term *Expr, predicates map[ast.ASTHandle]*Node) (out *ScannedBody, result error) {
	return g.scanBody(term, predicates, false)
}

// scanBodyTheoryQuantifiers keeps relation applications below binders as
// quantified theory obligations. A point-instance RPFP child cannot represent
// a universally quantified relation application: RedDualRela would equate
// every bound value to one child tuple.
func (g *RPFP) scanBodyTheoryQuantifiers(term *Expr, predicates map[ast.ASTHandle]*Node) (out *ScannedBody, result error) {
	return g.scanBody(term, predicates, true)
}

func (g *RPFP) scanBody(term *Expr, predicates map[ast.ASTHandle]*Node, quantifiedTheory bool) (out *ScannedBody, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := term.open(); err != nil {
		return nil, err
	}
	if term.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	for _, node := range predicates {
		if node == nil || node.closed || node.Owner != g {
			return nil, ErrWrapperOwner
		}
	}
	out = &ScannedBody{}
	m := g.context.manager
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			result = errors.Join(result, m.DelRef(&pins[i]))
		}
		if result != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	type scanKey struct {
		handle   ast.ASTHandle
		bound    bool
		eligible bool
	}
	memo := make(map[scanKey]ast.ASTHandle)
	var visit func(ast.ASTHandle, bool, bool) (ast.ASTHandle, error)
	visit = func(h ast.ASTHandle, bound, eligible bool) (ast.ASTHandle, error) {
		key := scanKey{handle: h, bound: bound && quantifiedTheory, eligible: !quantifiedTheory || eligible}
		if v, ok := memo[key]; ok {
			return v, nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		changed := h
		switch kind {
		case ast.ASTApp:
			decl, err := m.GetDecl(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			n, err := m.GetNumArgs(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if node, ok := predicates[decl]; ok && !(quantifiedTheory && bound) {
				name, err := m.GetASTName(decl)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				symbol, err := newSymbol(g.context, name)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				text, err := symbol.String()
				if err != nil {
					return ast.ASTHandle{}, err
				}
				suffix, err := m.Symbols.Intern(text + "_" + strconv.Itoa(len(out.Parameters)))
				if err != nil {
					return ast.ASTHandle{}, err
				}
				domain := make([]ast.ASTHandle, n)
				for i := range domain {
					arg, err := m.GetArg(h, uint32(i))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					domain[i], err = m.GetSort(arg)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
				sort, err := m.GetSort(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				decl, err = m.MkFuncDecl(suffix, domain, sort, ast.ASTFuncDeclInfo{})
				if err != nil {
					return ast.ASTHandle{}, err
				}
				parameter, err := NewFuncDecl(g.context, decl)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				out.Parameters = append(out.Parameters, parameter)
				out.Children = append(out.Children, node)
			} else if ok && quantifiedTheory && bound {
				out.QuantifiedTheory = true
			}
			args := make([]ast.ASTHandle, n)
			quantifiedArgument := int32(-1)
			family, familyErr := m.GetFamilyID(h)
			op, opErr := m.GetDeclKind(h)
			if familyErr != nil || opErr != nil {
				return ast.ASTHandle{}, errors.Join(familyErr, opErr)
			}
			if quantifiedTheory && eligible && !bound && n == 2 {
				if family == m.BasicFamilyID() && (op == ast.ASTOpEq || op == ast.ASTOpIff) {
					for i := uint32(0); i < 2; i++ {
						candidate, getErr := m.GetArg(h, i)
						if getErr != nil {
							return ast.ASTHandle{}, getErr
						}
						other, getErr := m.GetArg(h, 1-i)
						if getErr != nil {
							return ast.ASTHandle{}, getErr
						}
						kind, getErr := m.GetKind(candidate)
						if getErr != nil {
							return ast.ASTHandle{}, getErr
						}
						forall := false
						if kind == ast.ASTQuantifier {
							forall, getErr = m.IsForall(candidate)
							if getErr != nil {
								return ast.ASTHandle{}, getErr
							}
						}
						if forall && m.IsTrue(other) {
							quantifiedArgument = int32(i)
							break
						}
					}
				}
			}
			for i := range args {
				arg, err := m.GetArg(h, uint32(i))
				if err != nil {
					return ast.ASTHandle{}, err
				}
				childEligible := true
				if quantifiedTheory {
					// QuantifierAbstraction emits these wrappers directly or as
					// top-level conjuncts. Do not generalize through arbitrary
					// Boolean contexts, even if their current polarity is positive.
					childEligible = eligible && family == m.BasicFamilyID() && op == ast.ASTOpAnd
				}
				args[i], err = visit(arg, bound || int32(i) == quantifiedArgument, childEligible)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			changed, err = m.MkApp(decl, args)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			body, err = visit(body, bound, !quantifiedTheory && eligible)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			changed, err = m.UpdateQuantifierBody(h, body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		ref, err := m.NewRef(changed)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, ref)
		memo[key] = changed
		return changed, nil
	}
	root, err := term.Raw()
	if err != nil {
		return out, err
	}
	h, err := visit(root, false, true)
	if err != nil {
		return out, err
	}
	out.Term, err = NewExpr(g.context, h)
	return out, err
}
