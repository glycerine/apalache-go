package simplex

// simplexVariableHeap is the concrete heap<uint32, less-than> specialization
// used by src/util/heap.h and src/math/simplex/simplex.h. indices stores heap
// positions plus one, matching the source's zero-as-absent convention.
type simplexVariableHeap struct {
	values  []uint32
	indices []uint32
}

func (h *simplexVariableHeap) setBounds(n uint32) {
	if uint64(n) > uint64(^uint(0)>>1) {
		return
	}
	if uint32(len(h.indices)) < n {
		h.indices = append(h.indices, make([]uint32, int(n)-len(h.indices))...)
	}
}

func (h *simplexVariableHeap) ensure(v uint32) {
	if v >= uint32(len(h.indices)) {
		n := 2*v + 1
		if n <= v { // source unsigned bound growth wraps; Go avoids invalid allocation.
			n = v + 1
		}
		h.setBounds(n)
	}
}

func (h *simplexVariableHeap) contains(v uint32) bool {
	return v < uint32(len(h.indices)) && h.indices[v] != 0
}

func (h *simplexVariableHeap) swap(i, j int) {
	h.values[i], h.values[j] = h.values[j], h.values[i]
	h.indices[h.values[i]] = uint32(i + 1)
	h.indices[h.values[j]] = uint32(j + 1)
}

func (h *simplexVariableHeap) up(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if h.values[p] <= h.values[i] {
			break
		}
		h.swap(i, p)
		i = p
	}
}

func (h *simplexVariableHeap) down(i int) {
	for {
		left := 2*i + 1
		if left >= len(h.values) {
			return
		}
		child := left
		if right := left + 1; right < len(h.values) && h.values[right] < h.values[left] {
			child = right
		}
		if h.values[i] <= h.values[child] {
			return
		}
		h.swap(i, child)
		i = child
	}
}

func (h *simplexVariableHeap) insert(v uint32) {
	h.ensure(v)
	if h.contains(v) || v >= uint32(len(h.indices)) {
		return
	}
	h.values = append(h.values, v)
	h.indices[v] = uint32(len(h.values))
	h.up(len(h.values) - 1)
}

func (h *simplexVariableHeap) erase(v uint32) {
	if !h.contains(v) {
		return
	}
	i := int(h.indices[v] - 1)
	last := len(h.values) - 1
	h.indices[v] = 0
	if i == last {
		h.values = h.values[:last]
		return
	}
	replacement := h.values[last]
	h.values[i] = replacement
	h.values = h.values[:last]
	h.indices[replacement] = uint32(i + 1)
	if i > 0 && replacement < h.values[(i-1)/2] {
		h.up(i)
	} else {
		h.down(i)
	}
}

func (h *simplexVariableHeap) popMin() (uint32, bool) {
	if len(h.values) == 0 {
		return 0, false
	}
	v := h.values[0]
	h.erase(v)
	return v, true
}

func (h *simplexVariableHeap) clear() {
	for _, v := range h.values {
		h.indices[v] = 0
	}
	h.values = h.values[:0]
}
