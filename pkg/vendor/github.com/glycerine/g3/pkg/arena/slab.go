package arena

import "reflect"

type Config struct {
	PageSize  uint32
	SlotLimit uint32
	Budget    *Budget
}
type Stats struct {
	// OwnerBytes charges only this owner header; nested stats account their owners.
	OwnerBytes                                 uint64
	Live, Slots, Retired, Pages                uint32
	RecordBytes, MetadataBytes, DirectoryBytes uint64
}
type slotState struct {
	generation    Generation
	next          Slot
	live, retired bool
}

func pointerFree(t reflect.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	case reflect.Array:
		return pointerFree(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if !pointerFree(t.Field(i).Type) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// ValidateRecordType recursively audits hot record types; uintptr is rejected.
func ValidateRecordType(t reflect.Type) error {
	if !pointerFree(t) {
		return ErrPointerful
	}
	return nil
}
