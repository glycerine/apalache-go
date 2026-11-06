package hilbert

import "github.com/glycerine/g3/pkg/util/statistics"

type basisStatistics struct {
	numSubsumptions  uint32
	numResolves      uint32
	numSaturations   uint32
	indexComparisons uint32
	indexHit         uint32
	indexMiss        uint32
	indexFind        uint32
	indexInsert      uint32
}

func (b *Basis) CollectStatistics(out *statistics.Statistics) {
	if out == nil {
		return
	}
	out.UpdateUint("hb.num_subsumptions", b.stats.numSubsumptions)
	out.UpdateUint("hb.num_resolves", b.stats.numResolves)
	out.UpdateUint("hb.num_saturations", b.stats.numSaturations)
	out.UpdateUint("hb.basis_size", uint32(len(b.basis)))
	out.UpdateUint("hb.index.num_comparisons", b.stats.indexComparisons)
	out.UpdateUint("hb.index.num_hit", b.stats.indexHit)
	out.UpdateUint("hb.index.num_miss", b.stats.indexMiss)
	out.UpdateUint("hb.index.num_find", b.stats.indexFind)
	out.UpdateUint("hb.index.num_insert", b.stats.indexInsert)
	out.UpdateUint("hb.index.size", uint32(len(b.basis)))
}

func (b *Basis) ResetStatistics() { b.resetStatistics() }

func (b *Basis) resetStatistics() { b.stats = basisStatistics{} }
