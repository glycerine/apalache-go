package fp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// An engine owns refused native numeral-root cleanup until a later operation
// can release it, before the engine's AST services or relation owners close.
type arithmeticTableFactAttempt struct {
	manager *ast.ASTManager
	roots   []ast.ASTRef
	handles []ast.ASTHandle
}

func (a *arithmeticTableFactAttempt) Close() (result error) {
	if _, err := a.manager.ProofMode(); err != nil {
		return err
	}
	for i := len(a.roots) - 1; i >= 0; i-- {
		if a.roots[i] != (ast.ASTRef{}) {
			result = errors.Join(result, a.manager.DelRef(&a.roots[i]))
		}
	}
	if result == nil {
		a.roots = nil
	}
	return result
}

type arithmeticTableFactCleanupFailure struct {
	cause, cleanupErr error
	attempt           *arithmeticTableFactAttempt
}

func (f *arithmeticTableFactCleanupFailure) Error() string { return f.cause.Error() }
func (f *arithmeticTableFactCleanupFailure) Unwrap() error { return f.cause }
func (f *arithmeticTableFactCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *arithmeticTableFactCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *arithmeticTableFactCleanupFailure) OriginalCause() error { return f.cause }
func (f *arithmeticTableFactCleanupFailure) CleanupCause() error  { return f.cleanupErr }
func (a *arithmeticTableFactAttempt) dispose() error {
	if err := a.Close(); err != nil {
		return &arithmeticTableFactCleanupFailure{cause: err, cleanupErr: err, attempt: a}
	}
	return nil
}
func (e *PropositionalEngine) finishArithmeticTableFactAttempt(a *arithmeticTableFactAttempt) error {
	if err := a.dispose(); err != nil {
		e.arithmeticFactCleanup = append(e.arithmeticFactCleanup, a)
		return err
	}
	return nil
}
func (e *PropositionalEngine) clearArithmeticTableFactCleanup() error {
	for len(e.arithmeticFactCleanup) != 0 {
		if err := e.arithmeticFactCleanup[0].dispose(); err != nil {
			return err
		}
		e.arithmeticFactCleanup[0] = nil
		e.arithmeticFactCleanup = e.arithmeticFactCleanup[1:]
	}
	e.arithmeticFactCleanup = nil
	return nil
}

// addFact is the insertion stage after numerals have been constructed. It
// keeps both insertion failure and refused root destruction in the result.
func (a *arithmeticTableFactAttempt) addFact(e *PropositionalEngine, declaration ast.ASTHandle) (resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, e.finishArithmeticTableFactAttempt(a)) }()
	return e.AddFact(declaration, a.handles)
}
