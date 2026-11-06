package numeral

// Source: src/util/scoped_numeral.h:160-183. An expression temporary's
// destructor runs after the arithmetic body, including its failure path.

import "errors"

// ScopedIntExpressionCleanupError retains a stopped expression temporary.
type ScopedIntExpressionCleanupError struct {
	Cause error
	owner *ScopedInt
}

func (e *ScopedIntExpressionCleanupError) Error() string { return e.Cause.Error() }
func (e *ScopedIntExpressionCleanupError) Unwrap() error { return e.Cause }
func (e *ScopedIntExpressionCleanupError) Cleanup() error {
	if e == nil || e.owner == nil {
		return nil
	}
	if err := e.owner.Cleanup(); err != nil {
		return err
	}
	e.owner = nil
	return nil
}

// ScopedRationalExpressionCleanupError retains a stopped rational temporary.
type ScopedRationalExpressionCleanupError struct {
	Cause error
	owner *ScopedRational
}

func (e *ScopedRationalExpressionCleanupError) Error() string { return e.Cause.Error() }
func (e *ScopedRationalExpressionCleanupError) Unwrap() error { return e.Cause }
func (e *ScopedRationalExpressionCleanupError) Cleanup() error {
	if e == nil || e.owner == nil {
		return nil
	}
	if err := e.owner.Cleanup(); err != nil {
		return err
	}
	e.owner = nil
	return nil
}

func finishScopedIntExpressionFailure(local *ScopedInt, body error) error {
	if end := local.Close(); end != nil {
		return &ScopedIntExpressionCleanupError{Cause: errors.Join(body, end), owner: local}
	}
	return body
}

func finishScopedRationalExpressionFailure(local *ScopedRational, body error) error {
	if end := local.Close(); end != nil {
		return &ScopedRationalExpressionCleanupError{Cause: errors.Join(body, end), owner: local}
	}
	return body
}
