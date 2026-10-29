package polynomial

// Source: algebraic_numbers.cpp:180-216; mpq.h::del and mpz.h::del.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Literal source destruction must reread capacity on a retained nonnull NUM.
// Explicit owning abort instead acknowledges its recorded release progress.
func deleteAlgebraicCellInteger(z *unumeral.IntegerManager, a *unumeral.Int, owning bool) error {
	if owning {
		return z.Del(a)
	}
	return unumeral.SourceIntegerDel(z, a)
}
