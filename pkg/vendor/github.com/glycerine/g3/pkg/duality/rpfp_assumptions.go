package duality

// Source: duality_rpfp.cpp:1429-1437.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"strconv"
)

// UnderapproxFlag registers the node's source-named Boolean assumption and
// returns an independent owning expression. Repeated calls reuse the flag.
func (g *RPFP) UnderapproxFlag(node *Node) (out *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if node == nil || node.Owner != g || node.closed {
		return nil, ErrWrapperOwner
	}
	flag, err := g.context.BoolConst("@under" + strconv.Itoa(node.Number))
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, flag.Close())
		}
	}()
	h, err := flag.Raw()
	if err != nil {
		return nil, err
	}
	if g.underapproxFlags == nil {
		g.underapproxFlags = make(map[ast.ASTHandle]*Node)
	}
	if _, found := g.underapproxFlags[h]; !found {
		ref, err := g.context.manager.CopyRef(flag.value.ref)
		if err != nil {
			return nil, err
		}
		g.underapproxFlagRefs = append(g.underapproxFlagRefs, ref)
	}
	g.underapproxFlags[h] = node
	return flag, nil
}
func (g *RPFP) UnderapproxFlagRev(flag *Expr) (*Node, error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := flag.open(); err != nil {
		return nil, err
	}
	if flag.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	h, err := flag.Raw()
	if err != nil {
		return nil, err
	}
	return g.underapproxFlags[h], nil
}
