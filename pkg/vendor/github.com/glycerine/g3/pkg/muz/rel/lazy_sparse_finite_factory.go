package rel

import "errors"

// LazySparseFiniteFactory owns the shared deferred engine used by concrete
// tr_lazy_sparse predicate tables. Relation owners should close before it;
// Close also retires outstanding handles as a retryable fallback.
type LazySparseFiniteFactory struct {
	engine *LazySparseEngine
	closed bool
}

func NewLazySparseFiniteFactory() *LazySparseFiniteFactory {
	return &LazySparseFiniteFactory{engine: NewLazySparseEngine()}
}

func (f *LazySparseFiniteFactory) open() error {
	if f == nil || f.closed || f.engine == nil {
		return ErrLazySparse
	}
	return f.engine.open()
}

func (f *LazySparseFiniteFactory) New(signature Signature) (*LazySparseTable, error) {
	if !f.CanHandleSignature(signature) {
		return nil, ErrLazySparse
	}
	return f.engine.New(signature, 0)
}

// NewFunctional constructs a non-nullary lazy sparse table whose final
// functional columns use the concrete sparse-table update semantics.
func (f *LazySparseFiniteFactory) NewFunctional(signature Signature, functional int) (*LazySparseTable, error) {
	if err := f.open(); err != nil || signature.Len() == 0 || functional <= 0 {
		return nil, ErrLazySparse
	}
	return f.engine.New(signature, functional)
}

// Migrate copies source rows without consuming or aliasing source.
func (f *LazySparseFiniteFactory) Migrate(source FiniteTable) (_ *LazySparseTable, resultErr error) {
	if err := f.open(); err != nil || source == nil {
		return nil, errors.Join(ErrLazySparse, err)
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, err
	}
	replacement, err := f.New(signature)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, replacement.Close())
		}
	}()
	facts, err := source.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = replacement.Add(fact); err != nil {
			return nil, err
		}
	}
	published = true
	return replacement, nil
}

func (f *LazySparseFiniteFactory) Owns(table FiniteTable) bool {
	if f == nil || f.closed || f.engine == nil {
		return false
	}
	lazy, ok := table.(*LazySparseTable)
	return ok && lazy != nil && !lazy.closed && lazy.engine == f.engine
}

func (f *LazySparseFiniteFactory) Close() error {
	if f == nil || f.closed {
		return nil
	}
	if err := f.engine.Close(); err != nil {
		return err
	}
	f.closed, f.engine = true, nil
	return nil
}
