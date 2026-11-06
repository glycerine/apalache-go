//go:build !linux

package lp

// Source: src/test/lp.cpp:3234-3236, absent _LINUX_ call.

func (d *LPTestDriver) runBatch() error { return nil }
