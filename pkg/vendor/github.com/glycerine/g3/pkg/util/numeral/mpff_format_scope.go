package numeral

// Source: src/util/mpff.cpp:1229-1342.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// Decimal scratch limbs and character bytes use reusable Go slices. Shared
// MPN numerical workspaces retain their explicit Runtime cleanup.
type fastFormattingScope struct {
	buffers                     [4][]uint32
	active, pending             bool
	originalCause, cleanupCause error
}

func (m *FastFloatManager) beginFastFormatting() error {
	s := &m.formatting
	if s.active || s.pending {
		return arena.ErrInvalidSlot
	}
	*s = fastFormattingScope{buffers: s.buffers, active: true}
	return nil
}
func (m *FastFloatManager) formatView(i int, n uint32) ffLimbView {
	return ffLimbView{external: m.formatting.buffers[i], size: n}
}
func fastNaturalView(v ffLimbView) limbView {
	if v.external != nil {
		return limbView{external: v.external[v.offset:], size: v.size}
	}
	return limbView{external: (*v.v)[v.offset:], size: v.size}
}
func (m *FastFloatManager) beginFastCharacters(v ffLimbView, offset uint32) (uint32, error) {
	n := v.size
	if e := m.natural.beginIntegerCharacters(uint32(11 * n)); e != nil {
		return 0, e
	}
	e := m.natural.sourceToCString(fastNaturalView(v), sourceDecimalDestination{character: m.natural.CharFormatting.Buffer}, uint64(len(m.natural.CharFormatting.Buffer)))
	if e != nil {
		return 0, e
	}
	return m.natural.appendIntegerCString(offset)
}
func (m *FastFloatManager) finishFastFormatting() error {
	s := &m.formatting
	// Actual inner mpn destructors precede the enclosing completed character,
	// which precedes the outer decimal buffers.
	if m.natural.Formatting.CompletedField != 0 {
		if e := m.natural.unwindFormattingLocals(); e != nil {
			return e
		}
		m.natural.Formatting.Active = false
		m.natural.Formatting.PendingField = true
	}
	if e := m.natural.destroyIntegerCharacters(); e != nil {
		return e
	}
	for i := range s.buffers {
		s.buffers[i] = s.buffers[i][:0]
	}
	s.active = false
	s.pending = m.natural.Formatting.PendingField || m.natural.CharFormatting.pending
	if !s.pending {
		*s = fastFormattingScope{buffers: s.buffers}
	}
	return nil
}
func (m *FastFloatManager) endFastFormatting(cause error) error {
	if cause != nil {
		m.formatting.originalCause = cause
	}
	cleanup := m.finishFastFormatting()
	if cleanup != nil {
		m.formatting.active = false
		m.formatting.pending = true
		m.formatting.cleanupCause = cleanup
	}
	if cause != nil {
		return cause
	}
	return cleanup
}

// CleanupFormatting is owning cleanup after the source completed-local cursor.
// It does not fabricate a failed constructor's destructor or repair inputs.
func (m *FastFloatManager) CleanupFormatting() error {
	if m == nil || m.self != m {
		return ErrFastFloatOwner
	}
	if m.closed || m.natural == nil {
		return arena.ErrClosed
	}
	if e := m.finishFastFormatting(); e != nil {
		return e
	}
	if e := m.natural.cleanupOutputFormatting(); e != nil {
		return e
	}
	m.formatting = fastFormattingScope{}
	return nil
}
