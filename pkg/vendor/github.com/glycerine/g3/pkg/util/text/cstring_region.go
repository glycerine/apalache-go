package text

// Borrowed source C strings shared by parameter and diagnostic consumers.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
)

type RegionCString struct {
	Origin  uint64
	Ref     arena.BytesRegionRef
	Offset  uint32
	Present bool
}

var ErrNullCString = errors.New("g3: null C-string failed parameter stream")
