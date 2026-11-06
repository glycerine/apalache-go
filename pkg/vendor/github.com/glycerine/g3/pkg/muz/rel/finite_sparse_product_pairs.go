package rel

// Source: src/muz/base/dl_util.h sort_two_arrays, used by
// finite_product_relation_plugin::filter_identical_pairs_fn.

import "sort"

func sortPairedColumnIndices(pairs [][2]int) {
	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
}
