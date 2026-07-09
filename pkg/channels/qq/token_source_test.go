package qq

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestQQTokenResponseUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantTTL int64
		wantErr bool
	}{
		{
			name:    "string expiry",
			body:    `{"code":0,"message":"ok","access_token":"token","expires_in":"7200"}`,
			wantTTL: 7200,
		},
		{
			name:    "numeric expiry",
			body:    `{"code":0,"message":"ok","access_token":"token","expires_in":7200}`,
			wantTTL: 7200,
		},
		{
			name:    "empty expiry",
			body:    `{"code":0,"message":"ok","access_token":"token","expires_in":""}`,
			wantTTL: 0,
		},
		{
			name:    "missing expiry",
			body:    `{"code":0,"message":"ok","access_token":"token"}`,
			wantTTL: 0,
		},
		{
			name:    "invalid expiry",
			body:    `{"code":0,"message":"ok","access_token":"token","expires_in":"abc"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got qqTokenResponse
			err := json.Unmarshal([]byte(tt.body), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if got.ExpiresIn != tt.wantTTL {
				t.Fatalf("ExpiresIn = %d, want %d", got.ExpiresIn, tt.wantTTL)
			}
		})
	}
}

func TestQQTokenSourceAppliesDefaultExpiryWhenMissing(t *testing.T) {
	t.Parallel()

	source := &qqTokenSource{
		appID:     "app-id",
		appSecret: "app-secret",
		client: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"code":0,"message":"ok","access_token":"token","expires_in":""}`)),
				}, nil
			}),
		},
		now: func() time.Time { return time.Unix(1700000000, 0) },
	}

	token, err := source.fetchToken()
	if err != nil {
		t.Fatalf("fetchToken() error = %v", err)
	}

	if token.ExpiresIn != defaultQQTokenExpiresIn {
		t.Fatalf("ExpiresIn = %d, want %d", token.ExpiresIn, defaultQQTokenExpiresIn)
	}
	wantExpiry := source.now().Add(time.Duration(defaultQQTokenExpiresIn) * time.Second)
	if !token.Expiry.Equal(wantExpiry) {
		t.Fatalf("Expiry = %v, want %v", token.Expiry, wantExpiry)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
