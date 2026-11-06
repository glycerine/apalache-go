package api

// Source: generated api_commands.cpp, api_numeral.cpp and z3_replayer.cpp.

import (
	"encoding/binary"
	"math/big"
)

func replayBitVectorBoolNumeral(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	width, err := r.Uint(1)
	if err != nil {
		return err
	}
	words, err := r.UintArray(2)
	if err != nil {
		return err
	}
	if uint64(width) > uint64(len(words)) {
		return ErrZ3ReplayReference
	}
	// The source replayer casts its unsigned-int array to bool*. Preserve its
	// native byte order, including the four bool slots occupied by each word.
	bits := new(big.Int)
	var bytes [4]byte
	for i := uint32(0); i < width; i++ {
		if i%4 == 0 {
			binary.NativeEndian.PutUint32(bytes[:], words[i/4])
		}
		if bytes[i%4] != 0 {
			bits.SetBit(bits, int(i), 1)
		}
	}
	sort, err := owner.context.BitVectorSort(width)
	if err != nil {
		return err
	}
	value, err := owner.context.Numeral(bits.String(), sort)
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}
