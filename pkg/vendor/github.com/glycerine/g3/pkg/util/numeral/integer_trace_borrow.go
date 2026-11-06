package numeral

import "github.com/glycerine/g3/pkg/util/diagnostic"

// BorrowTraceTags shares the actual numeric source trace context with concrete
// callers. A nil binding resolves to the process tags, as nested mpz calls do.
func (m *IntegerManager) BorrowTraceTags() *diagnostic.TraceTags { return m.sourceTraceTags() }

// BorrowTraceTags returns the modular manager's own source trace binding.
func (m *IntegerModularManager) BorrowTraceTags() *diagnostic.TraceTags { return m.modularTraceTags() }
