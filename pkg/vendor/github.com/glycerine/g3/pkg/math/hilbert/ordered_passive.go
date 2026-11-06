package hilbert

type orderedPassiveEntry struct {
	value     *vector
	opposites []*vector
	next      int
	priority  uint64
	queued    bool
}

type orderedPassive struct {
	owner    *Basis
	positive []*vector
	negative []*vector
	entries  []*orderedPassiveEntry
}

func newOrderedPassive(owner *Basis, support []*vector) *orderedPassive {
	p := &orderedPassive{owner: owner}
	for _, value := range support {
		if value.weight > 0 {
			p.positive = append(p.positive, value)
		} else if value.weight < 0 {
			p.negative = append(p.negative, value)
		}
	}
	return p
}

func (p *orderedPassive) insert(value *vector, offset int) error {
	opposites := p.positive
	if value.weight > 0 {
		opposites = p.negative
	}
	entry := &orderedPassiveEntry{value: value, opposites: opposites, next: offset}
	p.entries = append(p.entries, entry)
	return p.advance(entry)
}

func (p *orderedPassive) advance(entry *orderedPassiveEntry) error {
	entry.queued = false
	for entry.next < len(entry.opposites) {
		support := entry.opposites[entry.next]
		if p.owner.canResolve(entry.value, support) {
			left, err := vectorMagnitude(entry.value)
			if err != nil {
				return err
			}
			right, err := vectorMagnitude(support)
			if err != nil || ^uint64(0)-left < right {
				if err == nil {
					err = ErrOverflow
				}
				return err
			}
			entry.priority = left + right
			entry.queued = true
			return nil
		}
		entry.next++
	}
	return nil
}

func (p *orderedPassive) empty() bool {
	for _, entry := range p.entries {
		if entry.queued {
			return false
		}
	}
	return true
}

// pop preserves passive2::pop's returned support offset. When resolution
// crosses zero, the new result restarts at the first opposite support.
func (p *orderedPassive) pop() (support, passive *vector, offset int, err error) {
	best := -1
	for index, entry := range p.entries {
		if entry.queued && (best < 0 || entry.priority < p.entries[best].priority) {
			best = index
		}
	}
	entry := p.entries[best]
	passive = entry.value
	offset = entry.next
	support = entry.opposites[offset]
	entry.next++
	if err = p.advance(entry); err != nil {
		return nil, nil, 0, err
	}
	combined, addErr := checkedAdd(passive.weight, support.weight)
	if addErr != nil {
		return nil, nil, 0, addErr
	}
	if sign(combined) != sign(passive.weight) {
		offset = 0
	}
	return support, passive, offset, nil
}
