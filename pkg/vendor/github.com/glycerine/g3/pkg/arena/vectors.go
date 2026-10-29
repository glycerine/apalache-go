package arena

// Scalar payload storage adaptation for util/vector.h and util/buffer.h.

import (
	"errors"
	"math/bits"
)

var ErrBounds = errors.New("arena: vector index or length out of bounds")

type VectorRef struct {
	Slot       Slot
	Generation Generation
}
type VectorConfig struct {
	PageSize    uint32
	Descriptors Config
}
type vectorDescriptor struct{ page, offset, length, capacity uint32 }
type VectorStats struct {
	// OwnerBytes charges only this owner header; nested stats account their owners.
	OwnerBytes                                       uint64
	Descriptors                                      Stats
	Live                                             uint32
	PayloadBytes, BlockMetadataBytes, DirectoryBytes uint64
}

func vectorClass(n uint32) (uint32, error) {
	if n == 0 {
		return 0, nil
	}
	if n > 1<<31 {
		return 0, ErrBounds
	}
	return uint32(1) << bits.Len32(n-1), nil
}
