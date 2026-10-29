package numeral

import (
	"errors"

	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrIntegerLCMPending = errors.New("mpnum: LCM raw integer cleanup pending")

func (m *IntegerManager) finishLCMSource() error {
	if !m.LCMPending {
		return nil
	}
	if _, e := sourceNumeralDel(m, &m.LCMTemporary); e != nil {
		return e
	}
	m.LCMTemporary = Int{}
	m.LCMPending = false
	return nil
}

// CleanupLCMTemporary releases a raw local retained after a stopped body or
// source end-label deletion. It does not replay the source end label.
func (m *IntegerManager) CleanupLCMTemporary() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if !m.LCMPending {
		return nil
	}
	if e := m.Del(&m.LCMTemporary); e != nil {
		return e
	}
	m.LCMTemporary = Int{}
	m.LCMPending = false
	return nil
}
