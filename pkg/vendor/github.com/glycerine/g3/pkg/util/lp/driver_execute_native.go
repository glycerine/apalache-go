//go:build !g3debug

package lp

type lpDriverDebug struct{}

// Native preparation cannot select any source debug action.
func (d *LPTestDriver) runDebug() error   { return ErrBounds }
func (d *LPTestDriver) closeDebug() error { return nil }
