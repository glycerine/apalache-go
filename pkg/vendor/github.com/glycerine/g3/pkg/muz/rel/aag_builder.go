package rel

// Source: src/muz/rel/aig_exporter.{h,cpp} AAG literal allocation,
// expression identity, gate sharing, latch/output storage and serialization.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

var ErrAAG = errors.New("muz/rel: invalid AAG exporter")

type aagGate struct{ out, left, right uint32 }
type aagLatch struct{ current, next uint32 }
type AAGBuilder struct {
	manager     *ast.ASTManager
	next        uint32
	expressions map[ast.ASTHandle]uint32
	gates       map[[2]uint32]uint32
	gateOrder   []aagGate
	inputs      []uint32
	latches     []aagLatch
	outputs     []uint32
	closed      bool
}

func NewAAGBuilder(manager *ast.ASTManager) (*AAGBuilder, error) {
	if manager == nil {
		return nil, ErrAAG
	}
	if _, e := manager.ProofsEnabled(); e != nil {
		return nil, e
	}
	return &AAGBuilder{manager: manager, next: 2, expressions: make(map[ast.ASTHandle]uint32), gates: make(map[[2]uint32]uint32)}, nil
}
func (b *AAGBuilder) open() error {
	if b == nil || b.closed || b.manager == nil {
		return ErrAAG
	}
	return nil
}
func (b *AAGBuilder) newLiteral() (uint32, error) {
	if e := b.open(); e != nil {
		return 0, e
	}
	if b.next > ^uint32(0)-2 {
		return 0, ErrAAG
	}
	r := b.next
	b.next += 2
	return r, nil
}
func (b *AAGBuilder) Close() error {
	if b == nil {
		return ErrAAG
	}
	if b.closed {
		return nil
	}
	b.expressions = nil
	b.gates = nil
	b.gateOrder = nil
	b.inputs = nil
	b.latches = nil
	b.outputs = nil
	b.manager = nil
	b.closed = true
	return nil
}
