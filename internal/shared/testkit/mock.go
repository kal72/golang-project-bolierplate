package testkit

type MockCall struct {
	Method string
	Args   []any
	Return []any
}

func Call(method string, args ...any) *MockCall {
	return &MockCall{
		Method: method,
		Args:   args,
	}
}

func (c *MockCall) Returns(returns ...any) MockCall {
	c.Return = returns
	return *c
}
