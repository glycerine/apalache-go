package diagnostic

// Source: src/util/small_object_allocator.cpp:172-242; src/util/util.cpp:22-40.
// Source allocator diagnostics through an explicit borrowed ostream adapter.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Read the actual borrowed C string at insertion time. Source strlen precedes
// field padding; the complete string consumes width once. No copied spelling,
// intermediate Go string, per-byte public stream calls or nested mutex locks.
func smallObjectCStringInto(out *utext.NumericTextStream, region *arena.BytesRegion, text utext.RegionCString) error {
	if e := out.CheckOpen(); e != nil {
		return e
	}
	if !text.Present {
		return utext.ErrNullCString
	}
	if region == nil || text.Origin != region.OwnerID() {
		return ErrSmallObjectOwner
	}
	return out.AppendCStringRegion(region, text.Ref, text.Offset)
}
func (b *smallObjectIOBridge) diagnostic(a *umemory.SmallAllocator) error {
	out := b.config.Diagnostics
	if e := out.AppendCString("Memory leak detected for small object allocator '"); e != nil {
		return e
	}
	if b.config.BorrowedID != nil || b.config.IDRef.Present || b.config.IDNull {
		id := b.config.IDRef
		if b.config.IDNull {
			id.Present = false
		}
		if e := smallObjectCStringInto(out, b.config.BorrowedID, id); e != nil {
			return e
		}
	} else if e := out.AppendCString(b.config.ID); e != nil {
		return e
	}
	if e := out.AppendCString("'. "); e != nil {
		return e
	}
	count, e := a.AllocationSize()
	if e != nil {
		return e
	}
	if e := out.AppendUint64(count); e != nil {
		return e
	}
	if e := out.AppendCString(" bytes leaked"); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	return out.Flush()
}
func (b *smallObjectIOBridge) verbose(a *umemory.SmallAllocator, end bool) error {
	out := b.config.Verbose
	if b.config.VerboseContext != nil {
		var e error
		out, e = b.config.VerboseContext.VerboseStream()
		if e != nil {
			return e
		}
	}
	// Source selects the stream once for this entire insertion chain.
	prefix := "(allocator-consolidate :wasted-size "
	if end {
		prefix = "(end-allocator-consolidate :wasted-size "
	}
	if e := out.AppendCString(prefix); e != nil {
		return e
	}
	wasted, e := a.WastedSize()
	if e != nil {
		return e
	}
	if e = out.AppendUint64(wasted); e != nil {
		return e
	}
	if e = out.AppendCString(" :memory "); e != nil {
		return e
	}
	if e = out.SetFloatMode(utext.NumericFixedFloat); e != nil {
		return e
	}
	if e = out.SetPrecision(2); e != nil {
		return e
	}
	n, e := b.config.Runtime.AllocationSize()
	if e != nil {
		return e
	}
	if e = out.AppendFloat64(float64(n) / float64(1024*1024)); e != nil {
		return e
	}
	if e = out.AppendCString(")"); e != nil {
		return e
	}
	if e = out.AppendByte('\n'); e != nil {
		return e
	}
	return out.Flush()
}

func (b *smallObjectIOBridge) Emit(a *umemory.SmallAllocator, event umemory.AllocatorIOEvent) error {
	switch event {
	case umemory.AllocatorIOLeak:
		return b.diagnostic(a)
	case umemory.AllocatorIOVerboseStart:
		return b.verbose(a, false)
	case umemory.AllocatorIOVerboseEnd:
		return b.verbose(a, true)
	}
	return umemory.ErrAllocatorIO
}
