package spacer

// Source: src/muz/spacer/spacer_context.h and spacer_context.cpp
// pred_transformer::pobs.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type ProofObligationStore struct {
	transformer *PredicateTransformer
	reuse       bool
	byPost      map[ast.ASTHandle][]*ProofObligation
	owned       []*ProofObligation
	closed      bool
}

func NewProofObligationStore(transformer *PredicateTransformer, reuse bool) (*ProofObligationStore, error) {
	if transformer == nil || transformer.open() != nil {
		return nil, ErrManagerOwner
	}
	return &ProofObligationStore{transformer: transformer, reuse: reuse, byPost: make(map[ast.ASTHandle][]*ProofObligation)}, nil
}

func (s *ProofObligationStore) open() error {
	if s == nil || s.closed || s.transformer == nil || s.transformer.open() != nil || s.byPost == nil {
		return ErrManagerOwner
	}
	return nil
}

func (s *ProofObligationStore) SetReuse(reuse bool) error {
	if err := s.open(); err != nil {
		return err
	}
	s.reuse = reuse
	return nil
}

func (s *ProofObligationStore) New(parent *ProofObligation, level, depth uint32, post ast.ASTHandle, bindings []ast.ASTHandle) (*ProofObligation, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	candidate, err := NewProofObligation(parent, s.transformer, level, depth, post, bindings)
	if err != nil {
		return nil, err
	}
	if s.reuse {
		normalized, getErr := candidate.Post()
		if getErr != nil {
			return nil, errors.Join(getErr, candidate.Release())
		}
		for _, existing := range s.byPost[normalized] {
			if existing.released || existing.parent != parent {
				continue
			}
			if inheritErr := existing.inherit(candidate); inheritErr != nil {
				return nil, errors.Join(inheritErr, candidate.Release())
			}
			if releaseErr := candidate.Release(); releaseErr != nil {
				return nil, releaseErr
			}
			return existing, nil
		}
		s.byPost[normalized] = append(s.byPost[normalized], candidate)
	}
	s.owned = append(s.owned, candidate)
	return candidate, nil
}

func (s *ProofObligationStore) Close() (result error) {
	if s == nil {
		return ErrManagerOwner
	}
	if s.closed {
		return nil
	}
	for i := len(s.owned) - 1; i >= 0; i-- {
		result = errors.Join(result, s.owned[i].Release())
	}
	if result == nil {
		s.owned = nil
		s.byPost = nil
		s.transformer = nil
		s.closed = true
	}
	return result
}
