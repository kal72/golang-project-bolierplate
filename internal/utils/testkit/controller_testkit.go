package testkit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type ControllerTestKit struct {
	t     *testing.T
	req   *http.Request
	app   http.Handler
	setup func()
}

func Controller(app http.Handler) *ControllerTestKit {
	return &ControllerTestKit{app: app}
}

func (c *ControllerTestKit) SetT(t *testing.T) *ControllerTestKit {
	c.t = t
	return c
}

func (c *ControllerTestKit) WithSetup(fn func()) *ControllerTestKit {
	c.setup = fn
	return c
}

func (c *ControllerTestKit) Request(method, path, body string) *ControllerTestKit {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.req = req
	return c
}

func (c *ControllerTestKit) Should(desc string, assertFn func(t *testing.T, res *httptest.ResponseRecorder)) *ControllerTestKit {
	c.t.Run(desc, func(t *testing.T) {
		if c.setup != nil {
			c.setup()
		}
		rec := httptest.NewRecorder()
		c.app.ServeHTTP(rec, c.req)
		assertFn(t, rec)
	})
	return c
}
