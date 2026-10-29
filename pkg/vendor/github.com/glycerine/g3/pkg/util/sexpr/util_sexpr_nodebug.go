//go:build !g3debug

package sexpr

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

func (m *SexprManager) sexprBVDiagnostic(c *unumeral.OwnedRational, base int64) error { return nil }
