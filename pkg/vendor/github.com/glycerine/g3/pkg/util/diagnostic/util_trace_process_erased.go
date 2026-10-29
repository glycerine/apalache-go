//go:build !g3trace

package diagnostic

func newProcessTraceTags() *TraceTags { return NewTraceTags() }
