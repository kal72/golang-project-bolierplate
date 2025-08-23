package testkit

import "testing"

func MustSuccess[T any](t *testing.T, val T, err error) T {
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return val
}
