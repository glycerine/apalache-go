package rel

func (b *AAGBuilder) And(left, right uint32) (uint32, error) {
	if e := b.open(); e != nil {
		return 0, e
	}
	if left > right {
		left, right = right, left
	}
	key := [2]uint32{left, right}
	if id, ok := b.gates[key]; ok {
		return id, nil
	}
	id, e := b.newLiteral()
	if e != nil {
		return 0, e
	}
	b.gates[key] = id
	b.gateOrder = append(b.gateOrder, aagGate{id, left, right})
	return id, nil
}
func (b *AAGBuilder) Or(left, right uint32) (uint32, error) {
	id, e := b.And(AAGNegate(left), AAGNegate(right))
	if e != nil {
		return 0, e
	}
	return AAGNegate(id), nil
}
func (b *AAGBuilder) AndMany(ids []uint32) (uint32, error) {
	if len(ids) == 0 {
		return AAGTrue(), nil
	}
	r := ids[0]
	var e error
	for _, id := range ids[1:] {
		r, e = b.And(r, id)
		if e != nil {
			return 0, e
		}
	}
	return r, nil
}
func (b *AAGBuilder) OrMany(ids []uint32) (uint32, error) {
	if len(ids) == 0 {
		return AAGFalse(), nil
	}
	r := ids[0]
	var e error
	for _, id := range ids[1:] {
		r, e = b.Or(r, id)
		if e != nil {
			return 0, e
		}
	}
	return r, nil
}
