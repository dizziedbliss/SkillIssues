package ingest

import (
	"errors"
	"testing"
)

func TestIsResourceLimitError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("github graphql: RESOURCE_LIMITS_EXCEEDED"), true},
		{errors.New("context deadline exceeded: timeout"), true},
		{errors.New("github: HTTP 401 Unauthorized"), false},
		{errors.New("github: repository not found"), false},
	}
	for _, c := range cases {
		if got := isResourceLimitError(c.err); got != c.want {
			t.Errorf("isResourceLimitError(%q) = %v, want %v", c.err, got, c.want)
		}
	}
}
