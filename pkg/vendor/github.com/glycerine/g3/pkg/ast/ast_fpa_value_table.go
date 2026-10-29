package ast

// Source: src/ast/fpa_decl_plugin.h:96-116; src/ast/fpa_decl_plugin.cpp:52-69.

// Hash buckets contain borrowed IDs into the owning numeral slice. A hash
// collision still uses MPF equality, including sign and format. Native map/slice
// growth has no simulated allocation accounting; numeral copies remain owned.
func (p *astFloatingPointPluginOwner) findValueID(id uint32) (uint32, int, error) {
	if p == nil || p.self != p || p.owner == nil || p.numeric == nil || !p.tableReady {
		return 0, -1, ErrASTHandle
	}
	value, e := p.values.at(id)
	if e != nil {
		return 0, -1, e
	}
	hash, e := p.numeric.Hash(&value)
	if e != nil {
		return 0, -1, e
	}
	for i, existing := range p.table[hash] {
		prior, e := p.values.at(existing)
		if e != nil {
			return 0, -1, e
		}
		equal, e := p.numeric.EqCore(&prior, &value)
		if e != nil {
			return 0, -1, e
		}
		if equal {
			return hash, i, nil
		}
	}
	return hash, -1, nil
}

func (p *astFloatingPointPluginOwner) internValueID(id uint32) (uint32, error) {
	hash, i, e := p.findValueID(id)
	if e != nil {
		return 0, e
	}
	if i >= 0 {
		return p.table[hash][i], nil
	}
	p.table[hash] = append(p.table[hash], id)
	return id, nil
}

func (p *astFloatingPointPluginOwner) eraseValueID(id uint32) error {
	hash, i, e := p.findValueID(id)
	if e != nil || i < 0 {
		return e
	}
	bucket := p.table[hash]
	copy(bucket[i:], bucket[i+1:])
	bucket = bucket[:len(bucket)-1]
	if len(bucket) == 0 {
		delete(p.table, hash)
	} else {
		p.table[hash] = bucket
	}
	return nil
}
