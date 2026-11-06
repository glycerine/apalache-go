//go:build g3trace

package interval

var intervalBugMulCallID uint64
var intervalBugPowerCallID uint64
var intervalBugInvCallID uint64
var intervalBugDivCallID uint64

func nextIntervalBugMulCallID() uint64 { id := intervalBugMulCallID; intervalBugMulCallID++; return id }
func nextIntervalBugPowerCallID() uint64 {
	id := intervalBugPowerCallID
	intervalBugPowerCallID++
	return id
}
func nextIntervalBugInvCallID() uint64 { id := intervalBugInvCallID; intervalBugInvCallID++; return id }
func nextIntervalBugDivCallID() uint64 { id := intervalBugDivCallID; intervalBugDivCallID++; return id }
