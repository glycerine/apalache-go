package opt

// Source: src/opt/maxsmt.cpp m_trail release after mutex preprocessing.

func (d *maxSMTMutexDecomposition) close() error {
	if d == nil {
		return nil
	}
	err := d.owner.close()
	if err == nil {
		d.weights, d.order = nil, nil
	}
	return err
}
