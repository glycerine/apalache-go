package arena

// Budget accounts owner-local storage. It is not safe for concurrent use.
// It bounds predictable arena growth, not host runtime allocation failures.
// A nil Budget permits unlimited storage. A zero limit permits zero bytes.
type Budget struct {
	limit, used uint64
	remaining   int64
}

func NewBudget(limit uint64) *Budget { return &Budget{limit: limit, remaining: -1} }
func (b *Budget) Reserve(bytes uint64) error {
	if b == nil {
		return nil
	}
	if bytes > b.limit-b.used {
		return ErrBudget
	}
	if b.remaining == 0 {
		return ErrInjectedFailure
	}
	if b.remaining > 0 {
		b.remaining--
	}
	b.used += bytes
	return nil
}

// Release requires a matching prior successful reservation.
func (b *Budget) Release(bytes uint64) {
	if b == nil {
		return
	}
	if bytes > b.used {
		panic("arena: budget release underflow")
	}
	b.used -= bytes
}
func (b *Budget) Used() uint64 {
	if b == nil {
		return 0
	}
	return b.used
}
func (b *Budget) Limit() uint64 {
	if b == nil {
		return ^uint64(0)
	}
	return b.limit
}

// FailAfter injects failure after n subsequent successful reservations.
// A negative count disables injection; failed reservations consume no count.
func (b *Budget) FailAfter(n int64) {
	if b != nil {
		b.remaining = n
	}
}
