package arith

// Source: src/tactic/arith/probe_arith.cpp:87-125; src/util/mpq.h:792-793.
import (
	"github.com/glycerine/g3/pkg/tactic"
	"math/big"
)

func integerBitSize(v *big.Int) uint32 {
	if v.Sign() == 0 {
		return 1
	}
	return uint32(new(big.Int).Abs(v).BitLen())
}
func (p *ArithmeticStatisticProbe) evalBitWidth(g *tactic.Goal) (float64, error) {
	apps, e := statisticApplications(g)
	if e != nil {
		return 0, e
	}
	m := g.Manager()
	var max, count uint32
	var total uint64
	for _, h := range apps {
		text, _, yes, e := m.ArithmeticNumeralText(h)
		if e != nil {
			return 0, e
		}
		if !yes {
			continue
		}
		v, ok := new(big.Rat).SetString(text)
		if !ok {
			continue
		}
		bits := integerBitSize(v.Num())
		if v.Denom().Cmp(big.NewInt(1)) != 0 {
			bits += integerBitSize(v.Denom())
		}
		if bits > max {
			max = bits
		}
		total += uint64(bits)
		count++
	}
	return statisticResult(p.kind == statAverageBitWidth, max, total, count), nil
}
