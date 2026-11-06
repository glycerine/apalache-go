package arena

import "errors"

type Slot uint32
type Generation uint32

var (
	ErrConfig          = errors.New("arena: invalid configuration")
	ErrPointerful      = errors.New("arena: record contains pointers or uintptr")
	ErrSlotLimit       = errors.New("arena: slot limit reached")
	ErrInvalidSlot     = errors.New("arena: invalid or dead slot")
	ErrClosed          = errors.New("arena: closed")
	ErrBudget          = errors.New("arena: memory budget exceeded")
	ErrInjectedFailure = errors.New("arena: injected allocation failure")
)
