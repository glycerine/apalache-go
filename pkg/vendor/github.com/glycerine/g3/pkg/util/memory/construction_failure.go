package memory

// ConstructionFailure is the already charged owner retaining real cleanup
// roots. CleanupCause is nil when cleanup is outstanding but has not refused.
type ConstructionFailure interface {
	error
	Unwrap() error
	Cleanup() error
	RuntimeOwner() *Runtime
	OriginalCause() error
	CleanupCause() error
}
