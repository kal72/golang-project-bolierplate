package testkit

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/mock"
)

type UsecaseTestKit[T any] struct {
	ctor  func(...any) T
	mocks []any
	t     *testing.T
}

func Usecase[T any](t *testing.T, ctor func(...any) T) *UsecaseTestKit[T] {
	return &UsecaseTestKit[T]{
		t:    t,
		ctor: ctor,
	}
}

func (d *UsecaseTestKit[T]) WithMocks(mocks ...any) *UsecaseTestKit[T] {
	d.mocks = mocks
	return d
}

func (d *UsecaseTestKit[T]) WithMockCalls(mockObj any, calls ...MockCall) *UsecaseTestKit[T] {
	v := reflect.ValueOf(mockObj)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		panic("mockObj must be a pointer to a struct")
	}

	mockField := v.Elem().FieldByName("Mock")
	if !mockField.IsValid() || !mockField.CanAddr() {
		panic("mock object does not embed mock.Mock")
	}

	mockImpl, ok := mockField.Addr().Interface().(*mock.Mock)
	if !ok {
		panic("embedded field Mock is not *mock.Mock")
	}

	for _, call := range calls {
		mockImpl.On(call.Method, call.Args...).Return(call.Return...)
	}

	d.mocks = append(d.mocks, mockObj)
	return d
}

func (d *UsecaseTestKit[T]) Should(desc string, testFunc func(t *testing.T, uc T)) *UsecaseTestKit[T] {
	if len(d.mocks) == 0 {
		panic("no mocks provided via WithMocks()")
	}
	if d.ctor == nil {
		panic("usecase constructor not provided")
	}

	d.t.Run(desc, func(t *testing.T) {
		usecase := d.ctor(d.mocks...)
		testFunc(t, usecase)
	})
	return d
}
