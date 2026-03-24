package errors

import (
	"net/http"
	"testing"
)

func TestGatewayError_HTTPStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  GatewayError
		want int
	}{
		{
			name: "invalid api key",
			err:  InvalidAPIKey("bad key"),
			want: http.StatusUnauthorized,
		},
		{
			name: "missing api key",
			err:  MissingAPIKey("missing key"),
			want: http.StatusUnauthorized,
		},
		{
			name: "bad gateway",
			err:  BadGateway("upstream 502"),
			want: http.StatusBadGateway,
		},
		{
			name: "model overloaded",
			err:  ModelOverloaded("busy"),
			want: http.StatusServiceUnavailable,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.err.HTTPStatus(); got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}
