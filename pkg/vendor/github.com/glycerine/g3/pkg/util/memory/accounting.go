package memory

// Counter-only source component from memory_manager.cpp. Its global fields
// belong either to one standalone Runtime or to a shared ProcessAccounting;
// lanes remain confined to their source allocation thread/explicit Go owner.
//
// These methods do not allocate/free storage. Real allocation adapters must
// invoke them at the source point: TLS malloc AFTER physical success, TLS
// realloc BEFORE physical operation, and non-TLS malloc/realloc BEFORE physical
// operation. Inputs include the stored source size_t extent or realloc delta.
// A counter-only getter is not a substitute for runtime-wide allocator hooks.
type utilMemoryAccounting struct {
	size, maxSize, peak, watermark int64
	count, maxCount                int64
	outOfMemory                    bool
}
type utilMemoryLaneAccounting struct{ size, count int64 }
type memoryAccountingResult uint8

const (
	memoryAccountingOK memoryAccountingResult = iota
	memoryAccountingOOM
	memoryAccountingCountLimit
	memoryAccountingDomain
)
const memoryAccountingThreshold int64 = 100000

// Signed source overflow is undefined; preserve preceding field mutations and
// stop at the actual undefined addition, rather than silently wrap in Go.
func memoryAccountingAdd(a, b int64) (int64, bool) {
	const maximum = int64(1<<63 - 1)
	const minimum = -maximum - 1
	if b > 0 && a > maximum-b || b < 0 && a < minimum-b {
		return 0, false
	}
	return a + b, true
}
func (a *utilMemoryAccounting) finishAccounting(allocating bool) memoryAccountingResult {
	if a.size > a.peak {
		a.peak = a.size
	}
	out := a.maxSize != 0 && a.size > a.maxSize
	count := a.maxCount != 0 && a.count > a.maxCount
	if allocating && out {
		a.outOfMemory = true
		return memoryAccountingOOM
	}
	if allocating && count {
		return memoryAccountingCountLimit
	}
	return memoryAccountingOK
}
func (a *utilMemoryAccounting) accountingSynchronize(lane *utilMemoryLaneAccounting, allocating bool) memoryAccountingResult {
	if lane == nil {
		return memoryAccountingDomain
	}
	size, ok := memoryAccountingAdd(a.size, lane.size)
	if !ok {
		return memoryAccountingDomain
	}
	a.size = size
	count, ok := memoryAccountingAdd(a.count, lane.count)
	if !ok {
		return memoryAccountingDomain
	}
	a.count = count
	// Source peak/limit calculation occurs before resetting ONLY the local size.
	// Count intentionally remains cumulative and contributes on every later sync.
	if a.size > a.peak {
		a.peak = a.size
	}
	out := a.maxSize != 0 && a.size > a.maxSize
	countLimit := a.maxCount != 0 && a.count > a.maxCount
	lane.size = 0
	if allocating && out {
		a.outOfMemory = true
		return memoryAccountingOOM
	}
	if allocating && countLimit {
		return memoryAccountingCountLimit
	}
	return memoryAccountingOK
}

// Source size_t promotion is distinct from signed lane/global synchronization.
// LP64 adds/subtracts unsigned64 and converts the modular result back to LL64.
// ILP32 promotes uint32 to LL64, whose signed overflow is an undefined boundary.
func memoryAccountingExtentAdd(a int64, b uint64, bits uint8) (int64, bool) {
	if bits == 64 {
		return int64(uint64(a) + b), true
	}
	if bits == 32 {
		return memoryAccountingAdd(a, int64(uint32(b)))
	}
	return 0, false
}
func memoryAccountingExtentSubtract(a int64, b uint64, bits uint8) (int64, bool) {
	if bits == 64 {
		return int64(uint64(a) - b), true
	}
	if bits == 32 {
		return memoryAccountingAdd(a, -int64(uint32(b)))
	}
	return 0, false
}

// Explicit LP64 convenience for counter fixtures/owners selecting that profile.
// A32-bit source realloc delta must instead be computed as uint32 before the
// accountExtentAllocation call; replacing it with signed new-old changes source.
func (a *utilMemoryAccounting) accountAllocation(lane *utilMemoryLaneAccounting, delta int64, tls bool) memoryAccountingResult {
	return a.accountExtentAllocation(lane, uint64(delta), 64, tls)
}
func (a *utilMemoryAccounting) accountExtentAllocation(lane *utilMemoryLaneAccounting, delta uint64, sizeTBits uint8, threadLocal bool) memoryAccountingResult {
	if threadLocal {
		if lane == nil {
			return memoryAccountingDomain
		}
		size, ok := memoryAccountingExtentAdd(lane.size, delta, sizeTBits)
		if !ok {
			return memoryAccountingDomain
		}
		lane.size = size
		count, ok := memoryAccountingAdd(lane.count, 1)
		if !ok {
			return memoryAccountingDomain
		}
		lane.count = count
		if lane.size > memoryAccountingThreshold {
			return a.accountingSynchronize(lane, true)
		}
		return memoryAccountingOK
	}
	size, ok := memoryAccountingExtentAdd(a.size, delta, sizeTBits)
	if !ok {
		return memoryAccountingDomain
	}
	a.size = size
	count, ok := memoryAccountingAdd(a.count, 1)
	if !ok {
		return memoryAccountingDomain
	}
	a.count = count
	return a.finishAccounting(true)
}
func (a *utilMemoryAccounting) beginDeallocation(lane *utilMemoryLaneAccounting, extent int64, tls bool) memoryAccountingResult {
	if extent < 0 {
		return memoryAccountingDomain
	}
	return a.beginExtentDeallocation(lane, uint64(extent), 64, tls)
}
func (a *utilMemoryAccounting) beginExtentDeallocation(lane *utilMemoryLaneAccounting, extent uint64, sizeTBits uint8, tls bool) memoryAccountingResult {
	if tls {
		if lane == nil {
			return memoryAccountingDomain
		}
		size, ok := memoryAccountingExtentSubtract(lane.size, extent, sizeTBits)
		if !ok {
			return memoryAccountingDomain
		}
		lane.size = size
		return memoryAccountingOK
	}
	size, ok := memoryAccountingExtentSubtract(a.size, extent, sizeTBits)
	if !ok {
		return memoryAccountingDomain
	}
	a.size = size
	return memoryAccountingOK
}

// Called only after the containing allocation adapter physically frees the
// block. Local/global subtraction is already published by beginDeallocation.
func (a *utilMemoryAccounting) finishDeallocation(lane *utilMemoryLaneAccounting, threadLocal bool) memoryAccountingResult {
	if !threadLocal {
		return memoryAccountingOK
	}
	if lane == nil {
		return memoryAccountingDomain
	}
	if lane.size < -memoryAccountingThreshold {
		return a.accountingSynchronize(lane, false)
	}
	return memoryAccountingOK
}
func (a *utilMemoryAccounting) allocationSize() uint64 {
	if a.size < 0 {
		return 0
	}
	return uint64(a.size)
}
func (a *utilMemoryAccounting) maximumUsed() uint64     { return uint64(a.peak) }
func (a *utilMemoryAccounting) allocationCount() uint64 { return uint64(a.count) }
func (a *utilMemoryAccounting) aboveWatermark() bool    { return a.watermark != 0 && a.watermark < a.size }
