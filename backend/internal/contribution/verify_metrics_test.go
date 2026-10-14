package contribution

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dizziedbliss/skillissues/internal/db"
)

func TestVerifyResultLabel(t *testing.T) {
	cases := []struct {
		res  *VerifyResult
		err  error
		want string
	}{
		{nil, fmt.Errorf("wrapped: %w", db.ErrNotFound), "not_found"},
		{nil, errors.New("github down"), "error"},
		{&VerifyResult{Verified: true, AlreadyAwarded: true}, nil, "already_awarded"},
		{&VerifyResult{Verified: true}, nil, "verified"},
		{&VerifyResult{}, nil, "not_verified"},
	}
	for _, c := range cases {
		if got := verifyResultLabel(c.res, c.err); got != c.want {
			t.Errorf("verifyResultLabel(%+v, %v) = %q, want %q", c.res, c.err, got, c.want)
		}
	}
}
