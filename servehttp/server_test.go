package servehttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer(t *testing.T) {
	t.Run("test statusHandler route", func(t *testing.T) {
		request := NewGetStatusReq()
		response := httptest.NewRecorder()
		StatusHandler(response, request)
		assertResponseBody(t, response.Body.String(), "Server is UP.")
	})
}

func NewGetStatusReq() *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	return req
}

func assertResponseBody(t testing.TB, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("response body is wrong, got %q want %q", got, want)
	}
}
