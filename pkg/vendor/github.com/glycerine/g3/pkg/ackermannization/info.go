package ackermannization

// Source: src/ackermannization/ackr_info.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

// Info owns both sides of each abstraction and the substitution borrowed by
// its default replacer. The manager outlives this counted owner.
type Info struct {
	self                              *Info
	manager                           *ast.ASTManager
	terms                             map[ast.ASTHandle]ast.ASTHandle
	constants                         map[ast.ASTHandle]ast.ASTHandle
	held                              []ast.ASTHandle
	replacer                          *rewriter.DefaultExprReplacer
	substitution                      ast.ASTExprSubstitution
	substitutionReady, sealed, closed bool
	refs                              uint32
	header                            uint64
}
type InfoConstructionFailure struct {
	owner          *Info
	cause, cleanup error
}

func (e *InfoConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *InfoConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *InfoConstructionFailure) OriginalCause() error { return e.cause }
func (e *InfoConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *InfoConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *InfoConstructionFailure) Cleanup() error { return e.owner.Close() }

func NewInfo(m *ast.ASTManager) (*Info, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(Info{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	i := &Info{manager: m, terms: make(map[ast.ASTHandle]ast.ASTHandle), constants: make(map[ast.ASTHandle]ast.ASTHandle), header: n}
	i.self = i
	var err error
	i.replacer, err = rewriter.NewDefaultExprReplacer(m)
	if err == nil {
		i.substitution, err = m.NewExprSubstitution()
		i.substitutionReady = err == nil
	}
	if err != nil {
		if cleanup := i.Close(); cleanup != nil {
			return nil, &InfoConstructionFailure{owner: i, cause: err, cleanup: cleanup}
		}
		return nil, err
	}
	return i, nil
}
func (i *Info) Check() error {
	if i == nil || i.self != i {
		return ast.ErrASTHandle
	}
	if i.closed {
		return arena.ErrClosed
	}
	return nil
}
func (i *Info) IncRef() error {
	if err := i.Check(); err != nil {
		return err
	}
	i.refs++
	return nil
}
func (i *Info) DecRef() error { _, err := i.DecRefConsumed(); return err }
func (i *Info) DecRefConsumed() (bool, error) {
	if err := i.Check(); err != nil {
		return false, err
	}
	if i.refs == 0 {
		return false, arena.ErrBounds
	}
	i.refs--
	if i.refs == 0 {
		return true, i.Close()
	}
	return true, nil
}
func (i *Info) hold(h ast.ASTHandle) error {
	if err := i.manager.IncRef(h); err != nil {
		return err
	}
	i.held = append(i.held, h)
	return nil
}
func (i *Info) SetAbstraction(term, constant ast.ASTHandle) error {
	if err := i.Check(); err != nil {
		return err
	}
	if ast.ASTSourceDebug && i.sealed {
		return ast.ErrASTDomain
	}
	d, err := i.manager.GetDecl(constant)
	if err != nil {
		return err
	}
	if err = i.manager.ExprSubstitutionInsert(i.substitution, term, constant, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
		return err
	}
	if err = i.hold(term); err != nil {
		return err
	}
	if err = i.hold(constant); err != nil {
		return err
	}
	i.terms[term] = constant
	i.constants[d] = term
	return nil
}
func (i *Info) Seal() error {
	if err := i.Check(); err != nil {
		return err
	}
	i.sealed = true
	return i.replacer.SetSubstitution(&i.substitution)
}
func (i *Info) Abstract(term ast.ASTHandle, out ast.ASTRef) error {
	if err := i.Check(); err != nil {
		return err
	}
	if ast.ASTSourceDebug && !i.sealed {
		return ast.ErrASTDomain
	}
	return i.replacer.Rewrite(term, out)
}
func (i *Info) FindTerm(constantDeclaration ast.ASTHandle) (ast.ASTHandle, error) {
	if err := i.Check(); err != nil {
		return ast.ASTHandle{}, err
	}
	return i.constants[constantDeclaration], nil
}
func (i *Info) Abstraction(term ast.ASTHandle) (ast.ASTHandle, error) {
	if err := i.Check(); err != nil {
		return ast.ASTHandle{}, err
	}
	h := i.terms[term]
	if ast.ASTSourceDebug && h.IsNull() {
		return h, ast.ErrASTDomain
	}
	return h, nil
}
func (i *Info) Translate(tr *ast.ASTTranslation) (result *Info, err error) {
	if err = i.Check(); err != nil {
		return nil, err
	}
	if tr == nil || tr.From() != i.manager {
		return nil, ast.ErrASTHandle
	}
	result, err = NewInfo(tr.To())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if cleanup := result.Close(); cleanup != nil {
				err = &InfoConstructionFailure{owner: result, cause: err, cleanup: cleanup}
			}
			result = nil
		}
	}()
	for term, constant := range i.terms {
		t, e := tr.Translate(term)
		if e != nil {
			return result, e
		}
		c, e := tr.Translate(constant)
		if e != nil {
			return result, e
		}
		if err = result.SetAbstraction(t, c); err != nil {
			return result, err
		}
	}
	if i.sealed {
		err = result.Seal()
	}
	return result, err
}
func (i *Info) Close() error {
	if i == nil || i.self != i {
		return ast.ErrASTHandle
	}
	if i.closed {
		return nil
	}
	if i.replacer != nil {
		if err := i.replacer.Close(); err != nil {
			return err
		}
		i.replacer = nil
	}
	if i.substitutionReady {
		if err := i.manager.DelExprSubstitution(&i.substitution); err != nil {
			return err
		}
		i.substitutionReady = false
	}
	for len(i.held) > 0 {
		n := len(i.held) - 1
		if err := i.manager.DecRef(i.held[n]); err != nil {
			return err
		}
		i.held = i.held[:n]
	}
	i.terms = nil
	i.constants = nil
	i.manager.Budget().Release(i.header)
	i.header = 0
	i.closed = true
	return nil
}
