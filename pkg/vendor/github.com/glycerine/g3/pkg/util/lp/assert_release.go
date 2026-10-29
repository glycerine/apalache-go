//go:build !g3debug

package lp

func sourceAssertionFailure() error { return nil }

const sourceDebugEnabled = false
