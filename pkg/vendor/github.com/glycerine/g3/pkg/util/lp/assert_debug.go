//go:build g3debug

package lp

func sourceAssertionFailure() error { return ErrAssertion }

const sourceDebugEnabled = true
