package solver

// Source: src/solver/check_sat_result.h; src/solver/check_sat_result.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// CheckSatResult is the result surface shared by solvers and check-sat
// commands. Returned core and label slices contain borrowed values copied from
// the result; AcquireModel returns one owning model reference.
type CheckSatResult interface {
	IncRef() error
	DecRef() error
	Status() (lbool.LBool, error)
	SetStatus(lbool.LBool) error
	CollectStatistics(*statistics.Statistics) error
	UnsatCore() ([]ast.ASTHandle, error)
	AcquireModel() (*model.ModelCore, error)
	Proof() (ast.ASTHandle, error)
	ReasonUnknown() (string, error)
	SetReasonUnknown(string) error
	Labels() ([]usymbol.Symbol, error)
	Manager() (*ast.ASTManager, error)
}

// ProgressCallback is the source progress_callback virtual surface.
type ProgressCallback interface {
	FastProgressSample()
	SlowProgressSample()
}

type SimpleCheckSatResult struct {
	self                             *SimpleCheckSatResult
	manager                          *ast.ASTManager
	status                           lbool.LBool
	stats                            statistics.Statistics
	model                            *model.ModelCore
	core                             []ast.ASTRef
	proof                            ast.ASTRef
	labels                           []usymbol.Symbol
	unknown                          string
	references                       uint32
	modelOwned, modelCleanup, closed bool
}

func NewSimpleCheckSatResult(m *ast.ASTManager) (*SimpleCheckSatResult, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	proof, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	r := &SimpleCheckSatResult{manager: m, status: lbool.Undef, proof: proof}
	r.self = r
	return r, nil
}

func (r *SimpleCheckSatResult) open() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return arena.ErrClosed
	}
	_, err := r.manager.ProofsEnabled()
	return err
}

func (r *SimpleCheckSatResult) IncRef() error {
	if err := r.open(); err != nil {
		return err
	}
	r.references++
	return nil
}

func (r *SimpleCheckSatResult) DecRef() error {
	if err := r.open(); err != nil {
		return err
	}
	if r.references == 0 {
		return arena.ErrBounds
	}
	r.references--
	if r.references == 0 {
		return r.Close()
	}
	return nil
}

func (r *SimpleCheckSatResult) Status() (lbool.LBool, error) {
	if err := r.open(); err != nil {
		return lbool.Undef, err
	}
	return r.status, nil
}

func (r *SimpleCheckSatResult) SetStatus(status lbool.LBool) error {
	if err := r.open(); err != nil {
		return err
	}
	r.status = status
	return nil
}

func (r *SimpleCheckSatResult) Statistics() (*statistics.Statistics, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return &r.stats, nil
}

func (r *SimpleCheckSatResult) CollectStatistics(dst *statistics.Statistics) error {
	if err := r.open(); err != nil {
		return err
	}
	if dst == nil {
		return arena.ErrInvalidSlot
	}
	dst.Copy(r.stats)
	return nil
}

func closeASTRefs(m *ast.ASTManager, refs *[]ast.ASTRef) error {
	for len(*refs) != 0 {
		if err := m.DelRef(&(*refs)[0]); err != nil {
			return err
		}
		copy(*refs, (*refs)[1:])
		*refs = (*refs)[:len(*refs)-1]
	}
	return nil
}

func (r *SimpleCheckSatResult) SetUnsatCore(values []ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	next := make([]ast.ASTRef, 0, len(values))
	for _, value := range values {
		ref, err := r.manager.NewRef(value)
		if err != nil {
			return errors.Join(err, closeASTRefs(r.manager, &next))
		}
		next = append(next, ref)
	}
	if err := closeASTRefs(r.manager, &r.core); err != nil {
		return errors.Join(err, closeASTRefs(r.manager, &next))
	}
	r.core = next
	return nil
}

func (r *SimpleCheckSatResult) UnsatCore() ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if r.status != lbool.False {
		return []ast.ASTHandle{}, nil
	}
	result := make([]ast.ASTHandle, len(r.core))
	for i := range r.core {
		value, err := r.manager.RefGet(r.core[i])
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}

func (r *SimpleCheckSatResult) releaseModel() error {
	if r.model == nil {
		return nil
	}
	if r.modelCleanup {
		if err := r.model.Close(); err != nil {
			return err
		}
	} else if r.modelOwned {
		consumed, err := r.model.DecRefConsumed()
		if consumed {
			r.modelOwned = false
			r.modelCleanup = err != nil
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	r.model, r.modelCleanup = nil, false
	return nil
}

func (r *SimpleCheckSatResult) SetModel(value *model.ModelCore) error {
	if err := r.open(); err != nil {
		return err
	}
	if value != nil {
		if value.Manager() != r.manager {
			return ast.ErrASTHandle
		}
		if err := value.IncRef(); err != nil {
			return err
		}
	}
	if err := r.releaseModel(); err != nil {
		if value != nil {
			_ = value.DecRef()
		}
		return err
	}
	r.model, r.modelOwned = value, value != nil
	return nil
}

func (r *SimpleCheckSatResult) AcquireModel() (*model.ModelCore, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if r.status == lbool.False || r.model == nil {
		return nil, nil
	}
	if err := r.model.IncRef(); err != nil {
		return nil, err
	}
	return r.model, nil
}

func (r *SimpleCheckSatResult) SetProof(value ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.manager.SetRef(r.proof, value)
}

func (r *SimpleCheckSatResult) Proof() (ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if r.status != lbool.False {
		return ast.ASTHandle{}, nil
	}
	return r.manager.RefGet(r.proof)
}

func (r *SimpleCheckSatResult) ReasonUnknown() (string, error) {
	if err := r.open(); err != nil {
		return "", err
	}
	return r.unknown, nil
}

func (r *SimpleCheckSatResult) SetReasonUnknown(message string) error {
	if err := r.open(); err != nil {
		return err
	}
	r.unknown = message
	return nil
}

func (r *SimpleCheckSatResult) SetReasonUnknownFromCaller(caller control.EventCaller) error {
	var message string
	switch caller {
	case control.EventCallerUnset:
		return r.open()
	case control.EventCallerControlC:
		message = "interrupted from keyboard"
	case control.EventCallerTimeout:
		message = "timeout"
	case control.EventCallerResourceLimit:
		message = "max. resource limit exceeded"
	case control.EventCallerAPIInterrupt:
		message = "interrupted"
	default:
		return arena.ErrBounds
	}
	return r.SetReasonUnknown(message)
}

func (r *SimpleCheckSatResult) SetLabels(labels []usymbol.Symbol) error {
	if err := r.open(); err != nil {
		return err
	}
	r.labels = append(r.labels[:0], labels...)
	return nil
}

func (r *SimpleCheckSatResult) Labels() ([]usymbol.Symbol, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return append([]usymbol.Symbol(nil), r.labels...), nil
}

func (r *SimpleCheckSatResult) Manager() (*ast.ASTManager, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return r.manager, nil
}

func (r *SimpleCheckSatResult) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if err := r.releaseModel(); err != nil {
		return err
	}
	if err := closeASTRefs(r.manager, &r.core); err != nil {
		return err
	}
	if err := r.manager.DelRef(&r.proof); err != nil {
		return err
	}
	r.labels = nil
	r.closed = true
	return nil
}
