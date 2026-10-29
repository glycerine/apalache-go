package proofs

// Source: src/ast/proofs/proof_utils.h:72-235.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type eliminateAuxState struct {
	manager *ast.ASTManager
	aux     ast.ASTHandle
	notAux  ast.ASTHandle
	cache   map[ast.ASTHandle]ast.ASTHandle
	pins    []ast.ASTRef
	tmp     ast.ASTRef
}

func (s *eliminateAuxState) close() (err error) {
	for i := len(s.pins) - 1; i >= 0; i-- {
		err = errors.Join(err, s.manager.DelRef(&s.pins[i]))
	}
	err = errors.Join(err, s.manager.DelRef(&s.tmp))
	return err
}

func (s *eliminateAuxState) pin(h ast.ASTHandle) error {
	r, err := s.manager.NewRef(h)
	if err == nil {
		s.pins = append(s.pins, r)
	}
	return err
}

func (s *eliminateAuxState) proofOfFact(args []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if len(args) == 0 {
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	fact := args[len(args)-1]
	for _, arg := range args[:len(args)-1] {
		yes, err := s.manager.IsProof(arg)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if !yes {
			continue
		}
		has, err := s.manager.ProofHasFact(arg)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if has {
			other, err := s.manager.GetProofFact(arg)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if other == fact {
				return arg, true, nil
			}
		}
	}
	return ast.ASTHandle{}, false, nil
}

func (s *eliminateAuxState) rebuild(decl ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
	fid, err := s.manager.GetFamilyID(decl)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := s.manager.GetDeclKind(decl)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if fid == s.manager.BasicFamilyID() && kind == ast.ASTOpOr {
		kept := make([]ast.ASTHandle, 0, len(args))
		for _, arg := range args {
			if !s.manager.IsFalse(arg) {
				kept = append(kept, arg)
			}
		}
		if len(kept) == 0 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if len(kept) == 1 {
			return kept[0], nil
		}
		return s.manager.MkOrArgs(kept)
	}
	if fid == s.manager.BasicFamilyID() && kind == ast.ASTOpIff && len(args) == 2 {
		return s.manager.MkIff(args[0], args[1])
	}
	if err = rewriter.SimplifyBooleanApplicationDefault(s.manager, decl, args, s.tmp); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.manager.RefGet(s.tmp)
}

func (s *eliminateAuxState) run(root ast.ASTHandle) (ast.ASTHandle, error) {
	todo := []ast.ASTHandle{root}
	truth, err := s.manager.MkTrue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	falsehood, err := s.manager.MkFalse()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for len(todo) != 0 {
		p := todo[len(todo)-1]
		// Preserve the source's root-key cache lookup. Child cache entries are
		// consumed while scanning arguments below.
		if _, found := s.cache[root]; found {
			todo = todo[:len(todo)-1]
			continue
		}
		n, err := s.manager.GetNumArgs(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args := make([]ast.ASTHandle, 0, n)
		dirty := false
		before := len(todo)
		for i := uint32(0); i < n; i++ {
			arg, err := s.manager.GetArg(p, i)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			switch arg {
			case s.aux:
				dirty = true
				args = append(args, truth)
			case s.notAux:
				dirty = true
				args = append(args, falsehood)
			default:
				var fact ast.ASTHandle
				asserted, err := s.manager.MatchProofAsserted(arg, &fact)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if asserted && fact == s.aux {
					dirty = true
					continue
				}
				hypothesis, err := s.manager.MatchProofHypothesis(arg, &fact)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if hypothesis && fact == s.aux {
					dirty = true
					continue
				}
				kind, err := s.manager.GetKind(arg)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if kind == ast.ASTApp {
					if replacement, found := s.cache[arg]; found {
						dirty = dirty || replacement != arg
						args = append(args, replacement)
					} else {
						todo = append(todo, arg)
					}
				} else {
					args = append(args, arg)
				}
			}
		}
		if len(todo) != before {
			continue
		}
		todo = todo[:len(todo)-1]
		result := p
		if dirty {
			unit, err := s.manager.IsProofKind(p, ast.ASTPrUnitResolution)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if unit {
				if len(args) == 2 {
					result = args[0]
				} else {
					if len(args) < 2 {
						return ast.ASTHandle{}, ast.ErrASTDomain
					}
					result, err = s.manager.MkProofUnitResolution(args[:len(args)-1])
					if err != nil {
						return ast.ASTHandle{}, err
					}
					fact, err := s.manager.GetProofFact(result)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					if fact != args[len(args)-1] {
						return ast.ASTHandle{}, ast.ErrASTDomain
					}
				}
			} else if match, yes, err := s.proofOfFact(args); err != nil {
				return ast.ASTHandle{}, err
			} else if yes {
				result = match
			} else {
				decl, err := s.manager.GetDecl(p)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				result, err = s.rebuild(decl, args)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		}
		if err := s.pin(result); err != nil {
			return ast.ASTHandle{}, err
		}
		s.cache[p] = result
	}
	result, found := s.cache[root]
	if !found {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return result, nil
}

// EliminateAuxAssertions replaces an auxiliary proposition by true, its
// negation by false, removes asserted/hypothesis leaves for the auxiliary, and
// reconstructs the proof DAG into out.
func EliminateAuxAssertions(m *ast.ASTManager, aux, proof ast.ASTHandle, out ast.ASTRef) (err error) {
	if m == nil {
		return ast.ErrASTHandle
	}
	yes, err := m.IsProof(proof)
	if err != nil || !yes {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	kind, err := m.GetKind(aux)
	if err != nil || kind != ast.ASTApp {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	notAux, err := m.MkNot(aux)
	if err != nil {
		return err
	}
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	state := &eliminateAuxState{manager: m, aux: aux, notAux: notAux, cache: make(map[ast.ASTHandle]ast.ASTHandle), tmp: tmp}
	if err = state.pin(aux); err != nil {
		return errors.Join(err, state.close())
	}
	if err = state.pin(notAux); err != nil {
		return errors.Join(err, state.close())
	}
	defer func() { err = errors.Join(err, state.close()) }()
	result, err := state.run(proof)
	if err != nil {
		return err
	}
	return m.SetRef(out, result)
}
