package z3rand

// Source: src/util/util.h:324-354.

const Max uint32 = 0x7fff

// Generator owns random_gen state. Its zero value has seed zero.
// Concurrent callers must synchronize access to a shared generator.
type Generator struct{ state uint32 }

func New(seed uint32) *Generator             { return &Generator{state: seed} }
func (g *Generator) SetSeed(seed uint32)     { g.state = seed }
func (g *Generator) Next() uint32            { g.state = g.state*214013 + 2531011; return (g.state >> 16) & Max }
func (g *Generator) Bounded(n uint32) uint32 { return g.Next() % n }

func ShuffleUint32(values []uint32, g *Generator) {
	for n := len(values) - 1; n > 0; n-- {
		k := int(g.Next()) % (n + 1)
		values[n], values[k] = values[k], values[n]
	}
}

func ShuffleInt32(values []int32, g *Generator) {
	for n := len(values) - 1; n > 0; n-- {
		k := int(g.Next()) % (n + 1)
		values[n], values[k] = values[k], values[n]
	}
}
