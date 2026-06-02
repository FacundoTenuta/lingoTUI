package chatgptauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestHTTPTokenExchangerExchangeSendsAuthorizationCodeFormGrant(t *testing.T) {
	var gotForm url.Values
	var gotContentType string
	var gotAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotForm, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-token","refresh_token":"refresh-token","expires_in":3600,"account_id":"account-id"}`))
	}))
	defer server.Close()
	exchanger := newTestHTTPTokenExchanger(t, server.URL, nil)

	_, err := exchanger.Exchange(context.Background(), TokenRequest{
		Code:         "auth-code",
		CodeVerifier: "code-verifier",
		RedirectURL:  "http://127.0.0.1:8787/callback",
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded", gotContentType)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q, want application/json", gotAccept)
	}
	assertForm(t, gotForm, "grant_type", "authorization_code")
	assertForm(t, gotForm, "client_id", "client-id")
	assertForm(t, gotForm, "code", "auth-code")
	assertForm(t, gotForm, "code_verifier", "code-verifier")
	assertForm(t, gotForm, "redirect_uri", "http://127.0.0.1:8787/callback")
}

func TestHTTPTokenExchangerExchangeParsesCredential(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-token","refresh_token":"refresh-token","expires_in":90,"account_id":"account-id","ignored":"field"}`))
	}))
	defer server.Close()
	exchanger := newTestHTTPTokenExchanger(t, server.URL, func(config *HTTPTokenExchangerConfig) {
		config.Now = func() time.Time { return now }
	})

	credential, err := exchanger.Exchange(context.Background(), TokenRequest{
		Code:         "auth-code",
		CodeVerifier: "code-verifier",
		RedirectURL:  "http://127.0.0.1:8787/callback",
	})
	if err != nil {
		t.Fatal(err)
	}

	if credential.AccessToken.Value != "access-token" {
		t.Fatalf("access token = %q", credential.AccessToken.Value)
	}
	if credential.RefreshToken.Value != "refresh-token" {
		t.Fatalf("refresh token = %q", credential.RefreshToken.Value)
	}
	if !credential.ExpiresAt.Equal(now.Add(90 * time.Second)) {
		t.Fatalf("expires at = %s, want %s", credential.ExpiresAt, now.Add(90*time.Second))
	}
	if credential.AccountID != "account-id" {
		t.Fatalf("account id = %q, want account-id", credential.AccountID)
	}
}

func TestHTTPTokenExchangerRefreshSendsRefreshTokenFormGrant(t *testing.T) {
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotForm, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600,"account_id":"account-id"}`))
	}))
	defer server.Close()
	exchanger := newTestHTTPTokenExchanger(t, server.URL, nil)

	_, err := exchanger.Refresh(context.Background(), RefreshRequest{RefreshToken: app.Secret{Value: "old-refresh-token"}})
	if err != nil {
		t.Fatal(err)
	}

	assertForm(t, gotForm, "grant_type", "refresh_token")
	assertForm(t, gotForm, "client_id", "client-id")
	assertForm(t, gotForm, "refresh_token", "old-refresh-token")
}

func TestHTTPTokenExchangerRefreshPreservesRefreshTokenWhenOmitted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access-token","expires_in":3600,"account_id":"account-id"}`))
	}))
	defer server.Close()
	exchanger := newTestHTTPTokenExchanger(t, server.URL, nil)

	credential, err := exchanger.Refresh(context.Background(), RefreshRequest{RefreshToken: app.Secret{Value: "old-refresh-token"}})
	if err != nil {
		t.Fatal(err)
	}

	if credential.RefreshToken.Value != "old-refresh-token" {
		t.Fatalf("refresh token = %q, want old-refresh-token", credential.RefreshToken.Value)
	}
}

func TestHTTPTokenExchangerNon2xxDoesNotLeakSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "response access-token refresh-token auth-code code-verifier", http.StatusBadGateway)
	}))
	defer server.Close()
	exchanger := newTestHTTPTokenExchanger(t, server.URL, nil)

	_, err := exchanger.Exchange(context.Background(), TokenRequest{
		Code:         "auth-code",
		CodeVerifier: "code-verifier",
		RedirectURL:  "http://127.0.0.1:8787/callback",
	})
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("error = %v, want status code", err)
	}
	assertNoSecrets(t, err.Error(), "access-token", "refresh-token", "auth-code", "code-verifier")
}

func TestHTTPTokenExchangerInvalidJSONDoesNotLeakBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`access-token refresh-token auth-code code-verifier`))
	}))
	defer server.Close()
	exchanger := newTestHTTPTokenExchanger(t, server.URL, nil)

	_, err := exchanger.Exchange(context.Background(), TokenRequest{
		Code:         "auth-code",
		CodeVerifier: "code-verifier",
		RedirectURL:  "http://127.0.0.1:8787/callback",
	})
	if err == nil {
		t.Fatal("error is nil, want invalid JSON error")
	}
	assertNoSecrets(t, err.Error(), "access-token", "refresh-token", "auth-code", "code-verifier")
}

func TestHTTPTokenExchangerClientErrorDoesNotLeakRequestSecrets(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("proxy failed with auth-code code-verifier refresh-token")
	})
	exchanger := newTestHTTPTokenExchanger(t, "https://example.test/token", func(config *HTTPTokenExchangerConfig) {
		config.HTTPClient = &http.Client{Transport: transport}
	})

	_, err := exchanger.Exchange(context.Background(), TokenRequest{
		Code:         "auth-code",
		CodeVerifier: "code-verifier",
		RedirectURL:  "http://127.0.0.1:8787/callback",
	})
	if err == nil || !strings.Contains(err.Error(), "send ChatGPT OAuth token request") {
		t.Fatalf("error = %v, want sanitized send error", err)
	}
	assertNoSecrets(t, err.Error(), "auth-code", "code-verifier", "refresh-token", "http://127.0.0.1:8787/callback")
}

func TestHTTPTokenExchangerRefreshClientErrorDoesNotLeakRefreshToken(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("proxy failed with old-refresh-token")
	})
	exchanger := newTestHTTPTokenExchanger(t, "https://example.test/token", func(config *HTTPTokenExchangerConfig) {
		config.HTTPClient = &http.Client{Transport: transport}
	})

	_, err := exchanger.Refresh(context.Background(), RefreshRequest{RefreshToken: app.Secret{Value: "old-refresh-token"}})
	if err == nil || !strings.Contains(err.Error(), "send ChatGPT OAuth token request") {
		t.Fatalf("error = %v, want sanitized send error", err)
	}
	assertNoSecrets(t, err.Error(), "old-refresh-token")
}

func TestHTTPTokenExchangerClientContextErrorPropagates(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})
	exchanger := newTestHTTPTokenExchanger(t, "https://example.test/token", func(config *HTTPTokenExchangerConfig) {
		config.HTTPClient = &http.Client{Transport: transport}
	})

	_, err := exchanger.Exchange(context.Background(), TokenRequest{
		Code:         "auth-code",
		CodeVerifier: "code-verifier",
		RedirectURL:  "http://127.0.0.1:8787/callback",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestHTTPTokenExchangerContextCancellationPropagates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not receive canceled request")
	}))
	defer server.Close()
	exchanger := newTestHTTPTokenExchanger(t, server.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := exchanger.Exchange(ctx, TokenRequest{
		Code:         "auth-code",
		CodeVerifier: "code-verifier",
		RedirectURL:  "http://127.0.0.1:8787/callback",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

func TestNewHTTPTokenExchangerValidatesConfig(t *testing.T) {
	tests := []struct {
		name   string
		config HTTPTokenExchangerConfig
	}{
		{name: "empty client ID", config: HTTPTokenExchangerConfig{TokenEndpoint: "https://example.test/token"}},
		{name: "empty endpoint", config: HTTPTokenExchangerConfig{ClientID: "client-id"}},
		{name: "relative endpoint", config: HTTPTokenExchangerConfig{ClientID: "client-id", TokenEndpoint: "/token"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewHTTPTokenExchanger(tt.config); err == nil {
				t.Fatal("error is nil, want validation error")
			}
		})
	}
}

func TestHTTPTokenExchangerValidatesRequestsWithGenericFieldNames(t *testing.T) {
	exchanger := newTestHTTPTokenExchanger(t, "https://example.test/token", nil)
	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{name: "missing code", run: func() error {
			_, err := exchanger.Exchange(context.Background(), TokenRequest{CodeVerifier: "code-verifier", RedirectURL: "http://127.0.0.1:8787/callback"})
			return err
		}, want: "code"},
		{name: "missing code verifier", run: func() error {
			_, err := exchanger.Exchange(context.Background(), TokenRequest{Code: "auth-code", RedirectURL: "http://127.0.0.1:8787/callback"})
			return err
		}, want: "code_verifier"},
		{name: "missing redirect URI", run: func() error {
			_, err := exchanger.Exchange(context.Background(), TokenRequest{Code: "auth-code", CodeVerifier: "code-verifier"})
			return err
		}, want: "redirect_uri"},
		{name: "missing refresh token", run: func() error {
			_, err := exchanger.Refresh(context.Background(), RefreshRequest{})
			return err
		}, want: "refresh_token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want field %q", err, tt.want)
			}
			assertNoSecrets(t, err.Error(), "auth-code", "code-verifier", "http://127.0.0.1:8787/callback")
		})
	}
}

func TestHTTPTokenExchangerUsesInjectedHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-token","refresh_token":"refresh-token","expires_in":3600,"account_id":"account-id"}`))
	}))
	defer server.Close()
	transport := &recordingTransport{base: http.DefaultTransport}
	exchanger := newTestHTTPTokenExchanger(t, server.URL, func(config *HTTPTokenExchangerConfig) {
		config.HTTPClient = &http.Client{Transport: transport}
	})

	_, err := exchanger.Exchange(context.Background(), TokenRequest{
		Code:         "auth-code",
		CodeVerifier: "code-verifier",
		RedirectURL:  "http://127.0.0.1:8787/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if transport.calls != 1 {
		t.Fatalf("transport calls = %d, want 1", transport.calls)
	}
}

func newTestHTTPTokenExchanger(t *testing.T, endpoint string, modify func(*HTTPTokenExchangerConfig)) *HTTPTokenExchanger {
	t.Helper()
	config := HTTPTokenExchangerConfig{ClientID: "client-id", TokenEndpoint: endpoint}
	if modify != nil {
		modify(&config)
	}
	exchanger, err := NewHTTPTokenExchanger(config)
	if err != nil {
		t.Fatal(err)
	}
	return exchanger
}

func assertForm(t *testing.T, form url.Values, key, want string) {
	t.Helper()
	if got := form.Get(key); got != want {
		t.Fatalf("form[%s] = %q, want %q", key, got, want)
	}
}

type recordingTransport struct {
	base  http.RoundTripper
	calls int
}

func (t *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.calls++
	return t.base.RoundTrip(request)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }
