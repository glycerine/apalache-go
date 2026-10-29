package solver

// Source: src/solver/mus.cpp; src/solver/mus.h.

import (
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type MUSModelTruth func(*model.ModelCore, ast.ASTHandle) (bool, error)

type MUSConfig struct {
	EvaluateTruth MUSModelTruth
}

// MUS is the retained deletion-based minimal-unsatisfiable-subset extractor.
// It borrows the solver and owns every stored AST and best-model reference.
type MUS struct {
	self                  *MUS
	solver                Solver
	manager               *ast.ASTManager
	config                MUSConfig
	literals, assumptions []ast.ASTRef
	literalIndex          map[ast.ASTHandle]uint32
	bestModel             *model.ModelCore
	bestModelOwned        bool
	soft                  []ast.ASTRef
	weights               []*big.Rat
	weight                big.Rat
	ownerBytes            uint64
	closed                bool
}

func NewMUS(s Solver, config MUSConfig) (*MUS, error) {
	if s == nil {
		return nil, ast.ErrASTHandle
	}
	m, err := s.Manager()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(MUS{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	u := &MUS{solver: s, manager: m, config: config, literalIndex: make(map[ast.ASTHandle]uint32), ownerBytes: amount}
	u.self = u
	return u, nil
}

func (u *MUS) open() error {
	if u == nil || u.self != u {
		return ast.ErrASTHandle
	}
	if u.closed {
		return arena.ErrClosed
	}
	_, err := u.solver.Manager()
	return err
}

func (u *MUS) pin(dst *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := u.manager.NewRef(h)
	if err != nil {
		return err
	}
	*dst = append(*dst, r)
	return nil
}

func (u *MUS) values(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		v, err := u.manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}

func (u *MUS) AddSoft(literal ast.ASTHandle) (uint32, error) {
	if err := u.open(); err != nil {
		return 0, err
	}
	ok, err := IsLiteral(u.manager, literal)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ast.ErrASTDomain
	}
	index := uint32(len(u.literals))
	if err = u.pin(&u.literals, literal); err != nil {
		return 0, err
	}
	u.literalIndex[literal] = index
	return index, nil
}

func (u *MUS) AddSoftAll(literals []ast.ASTHandle) error {
	for _, literal := range literals {
		if _, err := u.AddSoft(literal); err != nil {
			return err
		}
	}
	return nil
}

func (u *MUS) AddAssumption(literal ast.ASTHandle) error {
	if err := u.open(); err != nil {
		return err
	}
	ok, err := IsLiteral(u.manager, literal)
	if err != nil {
		return err
	}
	if !ok {
		return ast.ErrASTDomain
	}
	return u.pin(&u.assumptions, literal)
}

func containsAST(values []ast.ASTHandle, target ast.ASTHandle) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (u *MUS) clearBestModel() error {
	if u.bestModel == nil || !u.bestModelOwned {
		u.bestModel = nil
		return nil
	}
	if err := u.bestModel.DecRef(); err != nil {
		return err
	}
	u.bestModel, u.bestModelOwned = nil, false
	return nil
}

func (u *MUS) updateModel() error {
	if len(u.soft) == 0 {
		return nil
	}
	if u.config.EvaluateTruth == nil {
		return ast.ErrASTHandle
	}
	md, err := u.solver.AcquireModel()
	if err != nil {
		return err
	}
	if md == nil {
		return ast.ErrASTHandle
	}
	keep := false
	defer func() {
		if !keep {
			_ = md.DecRef()
		}
	}()
	var weight big.Rat
	for i := range u.soft {
		h, e := u.manager.RefGet(u.soft[i])
		if e != nil {
			return e
		}
		truth, e := u.config.EvaluateTruth(md, h)
		if e != nil {
			return e
		}
		if !truth {
			weight.Add(&weight, u.weights[i])
		}
	}
	if u.bestModel == nil || weight.Cmp(&u.weight) < 0 {
		if err = u.clearBestModel(); err != nil {
			return err
		}
		u.bestModel, u.bestModelOwned, keep = md, true, true
		u.weight.Set(&weight)
	}
	return nil
}

func (u *MUS) GetMUS() (status lbool.LBool, result []ast.ASTHandle, err error) {
	if err = u.open(); err != nil {
		return lbool.Undef, nil, err
	}
	if err = u.clearBestModel(); err != nil {
		return lbool.Undef, nil, err
	}
	literals, err := u.values(u.literals)
	if err != nil {
		return lbool.Undef, nil, err
	}
	if len(literals) == 1 {
		return lbool.True, []ast.ASTHandle{literals[0]}, nil
	}
	unknown := append([]ast.ASTHandle(nil), literals...)
	assumptions, err := u.values(u.assumptions)
	if err != nil {
		return lbool.Undef, nil, err
	}
	mus := make([]ast.ASTHandle, 0, len(literals))
	for len(unknown) != 0 {
		last := len(unknown) - 1
		literal := unknown[last]
		unknown = unknown[:last]
		notLiteral, e := u.manager.MkNotSimplified(literal)
		if e != nil {
			return lbool.Undef, nil, e
		}
		query := make([]ast.ASTHandle, 0, len(mus)+len(unknown)+len(assumptions)+1)
		query = append(query, mus...)
		query = append(query, unknown...)
		query = append(query, assumptions...)
		query = append(query, notLiteral)
		check, e := u.solver.CheckSat(query)
		if e != nil {
			return lbool.Undef, nil, e
		}
		switch check {
		case lbool.Undef:
			return lbool.Undef, nil, nil
		case lbool.True:
			mus = append(mus, literal)
			if e = u.updateModel(); e != nil {
				return lbool.Undef, nil, e
			}
		case lbool.False:
			core, e := u.solver.UnsatCore()
			if e != nil {
				return lbool.Undef, nil, e
			}
			if !containsAST(core, notLiteral) {
				unknown = unknown[:0]
				for _, value := range core {
					if !containsAST(mus, value) {
						unknown = append(unknown, value)
					}
				}
			}
		}
	}
	return lbool.True, mus, nil
}

func (u *MUS) Reset() error {
	if err := u.open(); err != nil {
		return err
	}
	if err := closeASTRefs(u.manager, &u.literals); err != nil {
		return err
	}
	if err := closeASTRefs(u.manager, &u.assumptions); err != nil {
		return err
	}
	u.literalIndex = make(map[ast.ASTHandle]uint32)
	return nil
}

func (u *MUS) SetSoft(soft []ast.ASTHandle, weights []*big.Rat) error {
	if err := u.open(); err != nil {
		return err
	}
	if len(soft) != len(weights) {
		return ast.ErrASTDomain
	}
	if err := u.clearBestModel(); err != nil {
		return err
	}
	u.weight.SetInt64(0)
	for i, h := range soft {
		if weights[i] == nil {
			return ast.ErrASTHandle
		}
		if err := u.pin(&u.soft, h); err != nil {
			return err
		}
		copyWeight := new(big.Rat).Set(weights[i])
		u.weights = append(u.weights, copyWeight)
		u.weight.Add(&u.weight, copyWeight)
	}
	return nil
}

func (u *MUS) BestModel() (*model.ModelCore, *big.Rat, error) {
	if err := u.open(); err != nil {
		return nil, nil, err
	}
	if u.bestModel != nil {
		if err := u.bestModel.IncRef(); err != nil {
			return nil, nil, err
		}
	}
	return u.bestModel, new(big.Rat).Set(&u.weight), nil
}

func (u *MUS) Close() error {
	if u == nil || u.self != u {
		return ast.ErrASTHandle
	}
	if u.closed {
		return nil
	}
	if err := u.clearBestModel(); err != nil {
		return err
	}
	if err := closeASTRefs(u.manager, &u.soft); err != nil {
		return err
	}
	if err := closeASTRefs(u.manager, &u.assumptions); err != nil {
		return err
	}
	if err := closeASTRefs(u.manager, &u.literals); err != nil {
		return err
	}
	u.weights = nil
	u.literalIndex = nil
	u.manager.Budget().Release(u.ownerBytes)
	u.ownerBytes = 0
	u.closed = true
	return nil
}
