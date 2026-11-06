package opt

// Source: src/util/sorting_network.h psort_nw vc_* cost model. sortmax reaches
// the zero-valued LE comparator mode, which counts one direction of clauses.

type sortMaxNetworkCost struct {
	wires, clauses int64
}

func (c sortMaxNetworkCost) add(other sortMaxNetworkCost) sortMaxNetworkCost {
	return sortMaxNetworkCost{wires: c.wires + other.wires, clauses: c.clauses + other.clauses}
}

func (c sortMaxNetworkCost) score() int64 { return 5*c.wires + c.clauses }

func sortMaxComparatorCost() sortMaxNetworkCost {
	return sortMaxNetworkCost{wires: 2, clauses: 3}
}

func (e *sortMaxEncoder) useDirectSort(n int) bool {
	return n < 10 && e.directSortCost(n).score() < e.recursiveSortCost(n).score()
}

func (e *sortMaxEncoder) directSortCost(n int) sortMaxNetworkCost {
	return sortMaxNetworkCost{wires: int64(n), clauses: int64(1 << (n - 1))}
}

func (e *sortMaxEncoder) recursiveSortCost(n int) sortMaxNetworkCost {
	if n <= 1 {
		return sortMaxNetworkCost{}
	}
	if n == 2 {
		return e.mergeCost(1, 1)
	}
	left := n / 2
	return e.sortCost(left).add(e.sortCost(n - left)).add(e.mergeCost(left, n-left))
}

func (e *sortMaxEncoder) sortCost(n int) sortMaxNetworkCost {
	if n <= 1 {
		return sortMaxNetworkCost{}
	}
	if n == 2 {
		return e.mergeCost(1, 1)
	}
	if e.useDirectSort(n) {
		return e.directSortCost(n)
	}
	return e.recursiveSortCost(n)
}

func (e *sortMaxEncoder) useDirectMerge(a, b, c int) bool {
	return a < 1<<15 && b < 1<<15 && e.directMergeCost(a, b, c).score() < e.recursiveSubmergeCost(a, b, c).score()
}

func (e *sortMaxEncoder) directMergeCost(a, b, c int) sortMaxNetworkCost {
	left, right := minInt(a, c), minInt(b, c)
	productHalf := int64(left) * int64(right) / 2
	return sortMaxNetworkCost{wires: int64(c), clauses: int64(a+b) + productHalf}
}

func (e *sortMaxEncoder) mergeCost(a, b int) sortMaxNetworkCost {
	if a == 0 || b == 0 {
		return sortMaxNetworkCost{}
	}
	if a == 1 && b == 1 {
		return sortMaxComparatorCost()
	}
	if e.useDirectMerge(a, b, a+b) {
		return e.directMergeCost(a, b, a+b)
	}
	return e.recursiveMergeCost(a, b)
}

func (e *sortMaxEncoder) recursiveMergeCost(a, b int) sortMaxNetworkCost {
	ceilA, ceilB := ceilHalf(a), ceilHalf(b)
	floorA, floorB := a/2, b/2
	return e.mergeCost(ceilA, ceilB).
		add(e.mergeCost(floorA, floorB)).
		add(e.interleaveCost(ceilA+ceilB, floorA+floorB))
}

func (e *sortMaxEncoder) interleaveCost(even, odd int) sortMaxNetworkCost {
	comparators := minInt(even-1, odd)
	return sortMaxNetworkCost{wires: 2 * int64(comparators), clauses: 3 * int64(comparators)}
}

func (e *sortMaxEncoder) useDirectSubmerge(a, b, c int) bool {
	return a < 1<<15 && b < 1<<15 && e.directMergeCost(a, b, a+b).score() < e.recursiveSubmergeCost(a, b, c).score()
}

func (e *sortMaxEncoder) recursiveSubmergeCost(a, b, c int) sortMaxNetworkCost {
	ceilA, ceilB := ceilHalf(a), ceilHalf(b)
	floorA, floorB := a/2, b/2
	var c1, c2 int
	if c%2 == 0 {
		c1, c2 = 1+c/2, c/2
	} else {
		c1, c2 = (c+1)/2, (c-1)/2
	}
	return e.submergeCost(ceilA, ceilB, c1).
		add(e.submergeCost(floorA, floorB, c2)).
		add(e.interleaveCost(ceilA+ceilB, floorA+floorB)).
		add(sortMaxNetworkCost{wires: 1, clauses: 2})
}

func (e *sortMaxEncoder) submergeCost(a, b, c int) sortMaxNetworkCost {
	if a == 1 && b == 1 && c == 1 {
		return sortMaxNetworkCost{wires: 1, clauses: 2}
	}
	if a == 0 || b == 0 {
		return sortMaxNetworkCost{}
	}
	if a > c {
		return e.submergeCost(c, b, c)
	}
	if b > c {
		return e.submergeCost(a, c, c)
	}
	if a+b <= c {
		return e.mergeCost(a, b)
	}
	if e.useDirectSubmerge(a, b, c) {
		return e.directMergeCost(a, b, c)
	}
	return e.recursiveSubmergeCost(a, b, c)
}

func ceilHalf(value int) int { return (value + 1) / 2 }

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
