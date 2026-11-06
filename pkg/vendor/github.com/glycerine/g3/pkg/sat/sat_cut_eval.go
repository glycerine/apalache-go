package sat

import (
	"strconv"
	"strings"
)

func (c satCut) Eval(env []satCutVal) satCutVal {
	t, n := c.Table(), c.Table() // source intentionally uses table() for both channels
	if c.size == 1 && t == 2 {
		return env[c.elems[0]]
	}
	var v satCutVal
	for i := uint32(0); i < 64; i++ {
		var offset uint32
		for j := uint32(0); j < c.size; j++ {
			offset |= uint32(env[c.elems[j]].t>>i&1) << j
		}
		v.t |= t >> offset & 1 << i
		v.f |= n >> offset & 1 << i
	}
	return v
}

func (c satCut) String() string {
	var b strings.Builder
	b.WriteByte('{')
	for i := uint32(0); i < c.size; i++ {
		if i != 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.FormatUint(uint64(c.elems[i]), 10))
	}
	b.WriteString("} ")
	b.WriteString(satCutTableString(c.size, c.Table()))
	return b.String()
}
