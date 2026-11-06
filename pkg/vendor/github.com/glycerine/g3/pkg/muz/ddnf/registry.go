package ddnf

// Source: src/muz/ddnf/ddnf.cpp:428-472, class ddnfs.

import (
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// Registry owns one lazily constructed DDNF core for each bit width. The
// source u_map does not use iteration for solver decisions; insertion order is
// retained explicitly for deterministic display and destruction.
type Registry struct {
	config arena.VectorConfig
	cores  map[uint32]*Core
	widths []uint32
	closed bool
}

func NewRegistry(config arena.VectorConfig) *Registry {
	return &Registry{config: config, cores: make(map[uint32]*Core)}
}

func (r *Registry) Get(width uint32) (*Core, error) {
	if r == nil || r.closed {
		return nil, arena.ErrClosed
	}
	if core := r.cores[width]; core != nil {
		return core, nil
	}
	core, err := NewCore(width, r.config)
	if err != nil {
		return nil, err
	}
	// Publish only after the complete core, including its all-X root, exists.
	r.cores[width] = core
	r.widths = append(r.widths, width)
	return core, nil
}

func (r *Registry) AllocateUint64Range(width uint32, value uint64, high, low uint32) (*rel.TBV, error) {
	core, err := r.Get(width)
	if err != nil {
		return nil, err
	}
	return core.Manager().AllocateUint64Range(value, high, low)
}

func (r *Registry) Insert(width uint32, vector *rel.TBV) (*Node, error) {
	core, err := r.Get(width)
	if err != nil {
		return nil, err
	}
	return core.Insert(vector)
}

func (r *Registry) Lookup(width uint32, vector *rel.TBV) ([]*Node, error) {
	if r == nil || r.closed {
		return nil, arena.ErrClosed
	}
	core := r.cores[width]
	if core == nil {
		return nil, ErrCore
	}
	return core.descendants(vector)
}

func (r *Registry) Widths() []uint32 {
	if r == nil || r.closed {
		return nil
	}
	return append([]uint32(nil), r.widths...)
}

func (r *Registry) Display() (string, error) {
	if r == nil || r.closed {
		return "", arena.ErrClosed
	}
	var output strings.Builder
	for _, width := range r.widths {
		text, err := r.cores[width].Display()
		if err != nil {
			return "", err
		}
		output.WriteString(text)
	}
	return output.String(), nil
}

func (r *Registry) Close() {
	if r == nil || r.closed {
		return
	}
	for i := len(r.widths); i > 0; i-- {
		r.cores[r.widths[i-1]].Close()
	}
	r.cores = nil
	r.widths = nil
	r.closed = true
}
