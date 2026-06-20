package lifecycle

import (
	"errors"
	"testing"
)

func TestErrAlreadyErased_Sentinel(t *testing.T) {
	if !errors.Is(ErrAlreadyErased, ErrAlreadyErased) {
		t.Fatal("sentinel comparison failed")
	}
}

func TestErrTransferOwnershipFirst_Sentinel(t *testing.T) {
	if !errors.Is(ErrTransferOwnershipFirst, ErrTransferOwnershipFirst) {
		t.Fatal("sentinel comparison failed")
	}
}

func TestErrUserNotFound_Sentinel(t *testing.T) {
	if !errors.Is(ErrUserNotFound, ErrUserNotFound) {
		t.Fatal("sentinel comparison failed")
	}
}

func TestEraseResult_Shape(t *testing.T) {
	r := &EraseResult{
		UpdatedRows: map[string]int64{"users": 1, "observations": 0},
	}
	if r.UpdatedRows["users"] != 1 {
		t.Fatal("users count")
	}
	if r.RevokedAPIKeys != 0 {
		t.Fatal("default revoked")
	}
}
