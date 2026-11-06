package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// GetProof returns an owned proof reference, null-valued for a safe query.
// Source: muz/duality/duality_dl_interface.cpp, get_proof.
func (e *QueryEngine) GetProof() (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if e.active == nil || e.status == lbool.Undef {
		return ast.ASTRef{}, graph.ErrWrapperOwner
	}
	if e.status != lbool.True {
		m, err := e.context.Manager()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return m.NewRef(ast.ASTHandle{})
	}
	background := append([]*graph.Expr(nil), e.config.Theory...)
	background = append(background, e.active.background...)
	var proof ast.ASTRef
	err := e.context.WithProfiler(e.config.Profiler, func() error {
		var err error
		proof, err = e.active.engine.GetCounterexample().Proof(e.config.Factory, background...)
		return err
	})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if len(background) > 0 {
		if err := e.validateBackgroundProof(proof); err != nil {
			m, _ := e.context.Manager()
			return ast.ASTRef{}, errors.Join(err, m.DelRef(&proof))
		}
	}
	return proof, nil
}

func (b *Backend) GetProof() (ast.ASTRef, error) {
	if err := b.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return b.queries.GetProof()
}

// Validate every logical proof-checker obligation without supplying background
// assertions to the checker solver: those dependencies must appear as explicit
// proof premises, not as hidden context.
func (e *QueryEngine) validateBackgroundProof(proof ast.ASTRef) (result error) {
	m, err := e.context.Manager()
	if err != nil {
		return err
	}
	h, err := m.RefGet(proof)
	if err != nil {
		return err
	}
	checker, err := proofs.NewChecker(m)
	if err != nil {
		return err
	}
	valid, side, err := checker.Check(h)
	defer func() { result = errors.Join(result, checker.ReleaseSideConditions(side)) }()
	if err != nil {
		return err
	}
	if !valid {
		return graph.ErrProofWitness
	}
	native, err := e.config.Factory.New(m, nil, false, false, false, "")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, native.Close()) }()
	for _, condition := range side {
		if err := native.Push(); err != nil {
			return err
		}
		negated, err := m.MkNot(condition)
		if err != nil {
			return err
		}
		if err := native.Assert(negated); err != nil {
			return err
		}
		status, err := native.CheckSat(nil)
		if err != nil {
			return err
		}
		if err := native.Pop(1); err != nil {
			return err
		}
		if status != lbool.False {
			return graph.ErrProofWitness
		}
	}
	return nil
}
