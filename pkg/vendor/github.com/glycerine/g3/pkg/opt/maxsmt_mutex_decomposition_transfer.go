package opt

// Source: src/opt/maxsmt.cpp m_trail ownership transfer into an engine.

func (d *maxSMTMutexDecomposition) transferOwner(target *maxSMTASTOwner) {
	if d == nil || target == nil {
		return
	}
	target.refs = append(target.refs, d.owner.refs...)
	d.owner.refs = nil
}
