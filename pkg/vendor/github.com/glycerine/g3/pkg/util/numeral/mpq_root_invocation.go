package numeral

// Source: mpq.cpp root's two direct mpz root calls, each SET then root.

type RationalRootInvocation struct {
	manager *RationalManager
	roots   [2]IntegerRootInvocationReceipt
	copies  [2]IntegerCopyAllocationReceipt
}

func (r *RationalRootInvocation) Pending() bool {
	if r == nil || r.manager == nil || r.manager.self != r.manager {
		return false
	}
	z := r.manager.Z
	for i := range r.roots {
		if z.MatchesRootInvocation(&r.roots[i]) || z.MatchesCopyAllocationInvocation(&r.copies[i]) {
			return true
		}
	}
	return false
}
func (r *RationalRootInvocation) SourceCause() error {
	if r == nil || r.manager == nil || r.manager.self != r.manager {
		return nil
	}
	for i := len(r.roots) - 1; i >= 0; i-- {
		if e := r.manager.Z.RootInvocationSourceCause(&r.roots[i]); e != nil {
			return e
		}
	}
	return nil
}
func (r *RationalRootInvocation) Cleanup(abort bool) error {
	if r == nil || r.manager == nil || r.manager.self != r.manager {
		return nil
	}
	z := r.manager.Z
	for i := len(r.roots) - 1; i >= 0; i-- {
		if e := z.CleanupRootInvocation(&r.roots[i], false); e != nil {
			return e
		}
	}
	if abort {
		for i := len(r.roots) - 1; i >= 0; i-- {
			if e := z.CleanupRootInvocation(&r.roots[i], true); e != nil {
				return e
			}
			if e := z.CleanupCopyAllocationInvocation(&r.copies[i]); e != nil {
				return e
			}
		}
	}
	return nil
}
