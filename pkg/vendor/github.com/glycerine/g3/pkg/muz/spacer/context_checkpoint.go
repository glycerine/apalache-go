package spacer

// Source: src/muz/spacer/spacer_context.cpp context::checkpoint.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrCanceled = errors.New("spacer canceled")

func spacerCheckpoint(manager *ast.ASTManager) error {
	canceled, err := manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return ErrCanceled
	}
	return nil
}
