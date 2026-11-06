package rel

func (plan *SparseKeyLookupPlan) Clone() *SparseKeyLookupPlan {
	if plan == nil {
		return nil
	}
	return &SparseKeyLookupPlan{signature: Signature{domains: plan.signature.Domains()}, functional: plan.functional, columns: append([]int(nil), plan.columns...)}
}

func (plan *SparseKeyLookupPlan) Columns() []int {
	if plan == nil {
		return nil
	}
	return append([]int(nil), plan.columns...)
}

func (snapshot *SparseKeyIndexSnapshot) Clone() *SparseKeyIndexSnapshot {
	if snapshot == nil || snapshot.plan == nil {
		return nil
	}
	result := &SparseKeyIndexSnapshot{plan: snapshot.plan.Clone(), groups: snapshot.Groups(), byKey: make(map[string]int, len(snapshot.byKey))}
	for i, group := range result.groups {
		result.byKey[factKey(group.Key)] = i + 1
	}
	return result
}
