package testkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type RepoTestkit[T any] struct {
	t    *testing.T
	init func() T
	tx   func() error
}

func Repository[T any](init func() T) *RepoTestkit[T] {
	return &RepoTestkit[T]{init: init}
}

func (r *RepoTestkit[T]) SetT(t *testing.T) *RepoTestkit[T] {
	r.t = t
	return r
}

func (r *RepoTestkit[T]) WithTx(setup func() error) *RepoTestkit[T] {
	r.tx = setup
	return r
}

func (r *RepoTestkit[T]) Should(desc string, fn func(t *testing.T, repo T)) *RepoTestkit[T] {
	r.t.Run(desc, func(t *testing.T) {
		if r.tx != nil {
			assert.NoError(t, r.tx())
		}
		repo := r.init()
		fn(t, repo)
	})
	return r
}
