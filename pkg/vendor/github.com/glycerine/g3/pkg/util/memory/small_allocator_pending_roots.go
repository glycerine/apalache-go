package memory

// Source: src/util/small_object_allocator.cpp; src/util/memory_manager.cpp.

// PendingRoots exposes the actual pending allocation retained by this checked
// owner. The two trailing zero slots preserve the receipt comparison shape used
// by current numerical callers; consolidation scratch now uses Go slices.
func (a *SmallAllocator) PendingRoots() ([3]Block, error) {
	if a == nil || a.self != a {
		return [3]Block{}, ErrAllocatorOwner
	}
	return [3]Block{a.pending}, nil
}
