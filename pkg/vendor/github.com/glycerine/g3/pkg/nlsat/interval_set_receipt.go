package nlsat

// These are concrete owning continuations for the actual union buffer and
// returned set. They never acknowledge another operation's pending buffers,
// allocation outcome or retired interval objects.
type intervalUnionReceipt struct {
	owner  *IntervalSetManager
	buffer *intervalBuffer
}

type intervalJustificationScope struct {
	owner  *IntervalSetManager
	marked []uint32
}

func (m *IntervalSetManager) cleanupJustifications(scope *intervalJustificationScope) error {
	if err := m.owner(); err != nil {
		return err
	}
	if scope == nil || scope.owner != m || m.justifications != scope {
		return nil
	}
	for _, index := range scope.marked {
		m.visited[index] = false
	}
	scope.marked = nil
	m.justifications = nil
	return nil
}

func (m *IntervalSetManager) cleanupRetiredSet(s *IntervalSet) error {
	if err := m.owner(); err != nil {
		return err
	}
	for i, old := range m.retired {
		if old == s {
			if err := m.deleteSet(s); err != nil {
				return err
			}
			copy(m.retired[i:], m.retired[i+1:])
			m.retired[len(m.retired)-1] = nil
			m.retired = m.retired[:len(m.retired)-1]
			break
		}
	}
	return nil
}
func (m *IntervalSetManager) cleanupUnion(receipt *intervalUnionReceipt) error {
	if err := m.owner(); err != nil {
		return err
	}
	if receipt == nil || receipt.owner != m || receipt.buffer == nil {
		return nil
	}
	b := receipt.buffer
	if err := b.close(); err != nil {
		return err
	}
	for i, pending := range m.pending {
		if pending == b {
			copy(m.pending[i:], m.pending[i+1:])
			m.pending[len(m.pending)-1] = nil
			m.pending = m.pending[:len(m.pending)-1]
			break
		}
	}
	if b.allocationFailed && m.allocator.MatchesAllocationOutcome(&b.allocation) {
		if err := m.allocator.CleanupPending(); err != nil {
			return err
		}
		m.allocationFailed = false
		b.allocationFailed = false
	}
	if b.retired != nil {
		if err := m.cleanupRetiredSet(b.retired); err != nil {
			return err
		}
		b.retired = nil
	}
	// The buffer's actual fresh zero-reference result may never have reached
	// the caller's first INC. Retire that exact result without inventing refs.
	if s := b.published; s != nil && !s.closed && s.refs == 0 {
		if err := m.deleteSet(s); err != nil {
			m.retired = append(m.retired, s)
			b.retired = s
			return err
		}
	}
	*receipt = intervalUnionReceipt{}
	return nil
}
