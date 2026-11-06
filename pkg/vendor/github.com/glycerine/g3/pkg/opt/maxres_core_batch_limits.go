package opt

// Source: src/opt/maxres.cpp get_cores batching limits.

func (e *maxResEngine) coreBatchLimitReached(count, lastSize int) bool {
	if count <= 0 {
		return false
	}
	if e.config.maxNumCores == 0 || uint64(count) >= uint64(e.config.maxNumCores) {
		return true
	}
	return uint64(lastSize) >= uint64(e.config.maxCoreSize)
}
