//go:build !g3trace

package interval

func nextIntervalBugMulCallID() uint64   { return 0 }
func nextIntervalBugPowerCallID() uint64 { return 0 }
func nextIntervalBugInvCallID() uint64   { return 0 }
func nextIntervalBugDivCallID() uint64   { return 0 }
