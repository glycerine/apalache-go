package ast

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func astRegisterUserName(dictionary *usymbol.DictionaryInt32, names *[]usymbol.Symbol, name usymbol.Symbol) (int32, error) {
	if k, ok, e := dictionary.Find(name); e != nil || ok {
		return k, e
	}
	if uint64(len((*names))) > math.MaxInt32 {
		return 0, arena.ErrBounds
	}
	k := int32(len((*names)))
	// Source pushes the spelling before inserting its dictionary entry. A failed
	// table allocation leaves this completed name-vector prefix intact.
	(*names) = append((*names), name)
	if e := dictionary.Insert(name, k); e != nil {
		return 0, e
	}
	return k, nil
}
