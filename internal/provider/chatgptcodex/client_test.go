package chatgptcodex

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestCreateResponseSendsCodexRequest(t *testing.T) {
	var requestPath string
	var authHeader string
	var contentType string
	var originator string
	var accountID string
	var body codexRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authHeader = r.Header.Get("Authorization")
		contentType = r.Header.Get("Content-Type")
		originator = r.Header.Get("originator")
		accountID = r.Header.Get("ChatGPT-Account-Id")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"answer"}`))
	}))
	defer server.Close()

	client, err := NewClient(
		fakeTokenProvider{token: app.Secret{Value: "access-token"}},
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithAccountIDProvider(fakeAccountProvider{id: "account-id"}),
	)
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.CreateResponse(context.Background(), ResponseRequest{
		Model: "codex-mini",
		Messages: []Message{
			{Role: "user", Text: "hello"},
			{Role: "system", Text: "be brief"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if response.Text != "answer" {
		t.Fatalf("response text = %q, want answer", response.Text)
	}
	if requestPath != "/backend-api/codex/responses" {
		t.Fatalf("path = %q, want /backend-api/codex/responses", requestPath)
	}
	if authHeader != "Bearer access-token" {
		t.Fatalf("authorization = %q", authHeader)
	}
	if contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
	if originator != "lingotui" {
		t.Fatalf("originator = %q", originator)
	}
	if accountID != "account-id" {
		t.Fatalf("account id header = %q", accountID)
	}
	if body.Model != "codex-mini" {
		t.Fatalf("model = %q", body.Model)
	}
	if body.Stream {
		t.Fatal("stream = true, want false")
	}
	if len(body.Input) != 2 {
		t.Fatalf("input length = %d, want 2", len(body.Input))
	}
	if body.Input[0].Role != "user" || body.Input[0].Content[0].Type != "input_text" || body.Input[0].Content[0].Text != "hello" {
		t.Fatalf("first input = %+v", body.Input[0])
	}
	if body.Input[1].Role != "system" || body.Input[1].Content[0].Text != "be brief" {
		t.Fatalf("second input = %+v", body.Input[1])
	}
}

func TestCreateResponseOmitsEmptyAccountIDHeader(t *testing.T) {
	var accountHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountHeader = r.Header.Get("ChatGPT-Account-Id")
		_, _ = w.Write([]byte(`{"output_text":"answer"}`))
	}))
	defer server.Close()

	client, err := NewClient(
		fakeTokenProvider{token: app.Secret{Value: "access-token"}},
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithAccountIDProvider(fakeAccountProvider{id: "  "}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateResponse(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if accountHeader != "" {
		t.Fatalf("account header = %q, want empty", accountHeader)
	}
}

func TestCreateResponseParsesNestedText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"output":[{"content":[{"text":"nested answer"}]}]}`))
	}))
	defer server.Close()

	client, err := NewClient(fakeTokenProvider{token: app.Secret{Value: "access-token"}}, WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.CreateResponse(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "nested answer" {
		t.Fatalf("response text = %q, want nested answer", response.Text)
	}
}

func TestCreateResponseHTTPStatusErrorsDoNotLeakSensitiveData(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "unauthorized", status: http.StatusUnauthorized},
		{name: "server error", status: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "access-token account-id secret prompt response body", tt.status)
			}))
			defer server.Close()

			client, err := NewClient(
				fakeTokenProvider{token: app.Secret{Value: "access-token"}},
				WithBaseURL(server.URL),
				WithHTTPClient(server.Client()),
				WithAccountIDProvider(fakeAccountProvider{id: "account-id"}),
			)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CreateResponse(context.Background(), ResponseRequest{Model: "codex-mini", Messages: []Message{{Role: "user", Text: "secret prompt"}}})
			if err == nil {
				t.Fatal("expected error")
			}
			assertNoLeak(t, err.Error(), "access-token", "account-id", "secret prompt", "response body")
			if !strings.Contains(err.Error(), "status "+strconv.Itoa(tt.status)) {
				t.Fatalf("error = %q, want status code", err.Error())
			}
		})
	}
}

func TestCreateResponseProviderErrorsAreSanitizedAndContextErrorsPreserved(t *testing.T) {
	tests := []struct {
		name       string
		tokenErr   error
		accountErr error
		wantErr    error
	}{
		{name: "token provider", tokenErr: errors.New("bad access-token secret prompt")},
		{name: "account provider", accountErr: errors.New("bad account-id secret prompt")},
		{name: "token context canceled", tokenErr: context.Canceled, wantErr: context.Canceled},
		{name: "account deadline", accountErr: context.DeadlineExceeded, wantErr: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(
				fakeTokenProvider{token: app.Secret{Value: "access-token"}, err: tt.tokenErr},
				WithAccountIDProvider(fakeAccountProvider{id: "account-id", err: tt.accountErr}),
			)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CreateResponse(context.Background(), ResponseRequest{Model: "codex-mini", Messages: []Message{{Role: "user", Text: "secret prompt"}}})
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			assertNoLeak(t, err.Error(), "access-token", "account-id", "secret prompt")
		})
	}
}

func TestCreateResponseMissingTokenValidationDoesNotLeakPromptText(t *testing.T) {
	client, err := NewClient(fakeTokenProvider{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateResponse(context.Background(), ResponseRequest{Model: "codex-mini", Messages: []Message{{Role: "user", Text: "secret prompt"}}})
	if err == nil {
		t.Fatal("expected error")
	}
	assertNoLeak(t, err.Error(), "secret prompt")
}

func TestCreateResponseTransportErrorsAreSanitizedAndContextErrorsPreserved(t *testing.T) {
	tests := []struct {
		name    string
		round   roundTripFunc
		wantErr error
	}{
		{name: "transport", round: func(*http.Request) (*http.Response, error) {
			return nil, errors.New("transport access-token account-id secret prompt")
		}},
		{name: "context canceled", round: func(*http.Request) (*http.Response, error) { return nil, context.Canceled }, wantErr: context.Canceled},
		{name: "deadline", round: func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded }, wantErr: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(
				fakeTokenProvider{token: app.Secret{Value: "access-token"}},
				WithHTTPClient(&http.Client{Transport: tt.round}),
				WithAccountIDProvider(fakeAccountProvider{id: "account-id"}),
			)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CreateResponse(context.Background(), ResponseRequest{Model: "codex-mini", Messages: []Message{{Role: "user", Text: "secret prompt"}}})
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			assertNoLeak(t, err.Error(), "access-token", "account-id", "secret prompt")
		})
	}
}

func TestCreateResponseBodyReadContextErrorsArePreserved(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "canceled", err: context.Canceled},
		{name: "deadline", err: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(
				fakeTokenProvider{token: app.Secret{Value: "access-token"}},
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       errorReadCloser{err: tt.err},
					}, nil
				})}),
			)
			if err != nil {
				t.Fatal(err)
			}

			_, err = client.CreateResponse(context.Background(), validRequest())
			if !errors.Is(err, tt.err) {
				t.Fatalf("error = %v, want %v", err, tt.err)
			}
		})
	}
}

func TestCreateResponseValidationErrorsDoNotLeakPromptText(t *testing.T) {
	tests := []struct {
		name string
		req  ResponseRequest
	}{
		{name: "missing model", req: ResponseRequest{Messages: []Message{{Role: "user", Text: "secret prompt"}}}},
		{name: "no messages", req: ResponseRequest{Model: "codex-mini"}},
		{name: "empty text", req: ResponseRequest{Model: "codex-mini", Messages: []Message{{Role: "user", Text: "  "}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(fakeTokenProvider{token: app.Secret{Value: "access-token"}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CreateResponse(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
			assertNoLeak(t, err.Error(), "secret prompt", "access-token")
		})
	}
}

func TestCreateResponseNoTextErrorDoesNotLeakResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"output":[{"content":[{"text":""}]}],"debug":"secret body"}`))
	}))
	defer server.Close()

	client, err := NewClient(fakeTokenProvider{token: app.Secret{Value: "access-token"}}, WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateResponse(context.Background(), validRequest())
	if err == nil {
		t.Fatal("expected error")
	}
	assertNoLeak(t, err.Error(), "secret body")
}

func validRequest() ResponseRequest {
	return ResponseRequest{Model: "codex-mini", Messages: []Message{{Role: "user", Text: "hello"}}}
}

type fakeTokenProvider struct {
	token app.Secret
	err   error
}

func (p fakeTokenProvider) AccessToken(context.Context) (app.Secret, error) {
	if p.err != nil {
		return app.Secret{}, p.err
	}
	return p.token, nil
}

type fakeAccountProvider struct {
	id  string
	err error
}

func (p fakeAccountProvider) AccountID(context.Context) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.id, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type errorReadCloser struct{ err error }

func (r errorReadCloser) Read([]byte) (int, error) { return 0, r.err }
func (r errorReadCloser) Close() error             { return nil }

var _ io.ReadCloser = errorReadCloser{}

func assertNoLeak(t *testing.T, output string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(output, secret) {
			t.Fatalf("output leaked %q: %s", secret, output)
		}
	}
}
