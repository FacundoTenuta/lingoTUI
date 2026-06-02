package chatgptauth

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestLocalCallbackWaiterRedirectURLUsesLoopbackCallback(t *testing.T) {
	waiter := newTestCallbackWaiter(t)
	defer waiter.Close()

	parsed, err := url.Parse(waiter.RedirectURL())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "http" {
		t.Fatalf("scheme = %q, want http", parsed.Scheme)
	}
	if parsed.Path != callbackPath {
		t.Fatalf("path = %q, want %q", parsed.Path, callbackPath)
	}
	host, _, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" {
		t.Fatalf("host = %q, want 127.0.0.1", host)
	}
}

func TestLocalCallbackWaiterValidCallbackReturnsCodeAndState(t *testing.T) {
	waiter := newTestCallbackWaiter(t)
	callbackCh := waitForCallback(t, waiter, "state-value")

	response := requestCallback(t, waiter.RedirectURL()+"?code=auth-code&state=state-value")
	if response.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.status, http.StatusOK)
	}
	assertNoSecrets(t, response.body, "auth-code", "state-value", "access-token", "refresh-token", "verifier")

	callback := receiveCallback(t, callbackCh)
	if callback.Code != "auth-code" || callback.State != "state-value" || callback.Error != "" {
		t.Fatalf("callback = %#v, want code and state", callback)
	}
}

func TestLocalCallbackWaiterWrongStateKeepsWaitingForLaterValidCallback(t *testing.T) {
	waiter := newTestCallbackWaiter(t)
	callbackCh := waitForCallback(t, waiter, "state-value")

	response := requestCallback(t, waiter.RedirectURL()+"?code=wrong-code&state=wrong-state")
	if response.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.status, http.StatusBadRequest)
	}
	assertNoCallbackYet(t, callbackCh)

	response = requestCallback(t, waiter.RedirectURL()+"?code=auth-code&state=state-value")
	if response.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.status, http.StatusOK)
	}
	callback := receiveCallback(t, callbackCh)
	if callback.Code != "auth-code" || callback.State != "state-value" {
		t.Fatalf("callback = %#v, want later valid callback", callback)
	}
}

func TestLocalCallbackWaiterWrongPathKeepsWaitingForLaterValidCallback(t *testing.T) {
	waiter := newTestCallbackWaiter(t)
	callbackCh := waitForCallback(t, waiter, "state-value")

	response := requestCallback(t, strings.Replace(waiter.RedirectURL(), callbackPath, "/wrong", 1)+"?code=wrong-code&state=state-value")
	if response.status != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.status, http.StatusNotFound)
	}
	assertNoCallbackYet(t, callbackCh)

	response = requestCallback(t, waiter.RedirectURL()+"?code=auth-code&state=state-value")
	if response.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.status, http.StatusOK)
	}
	callback := receiveCallback(t, callbackCh)
	if callback.Code != "auth-code" || callback.State != "state-value" {
		t.Fatalf("callback = %#v, want later valid callback", callback)
	}
}

func TestLocalCallbackWaiterOAuthErrorReturnsCallbackErrorAndGenericResponse(t *testing.T) {
	waiter := newTestCallbackWaiter(t)
	callbackCh := waitForCallback(t, waiter, "state-value")

	response := requestCallback(t, waiter.RedirectURL()+"?error=access_denied&error_description=secret-detail&state=state-value")
	if response.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.status, http.StatusOK)
	}
	assertNoSecrets(t, response.body, "access_denied", "secret-detail", "state-value")
	callback := receiveCallback(t, callbackCh)
	if callback.Error != "access_denied" || callback.State != "state-value" || callback.Code != "" {
		t.Fatalf("callback = %#v, want OAuth error", callback)
	}
}

func TestLocalCallbackWaiterMissingCodeReturnsControlledFailureAndNoSecretLeakage(t *testing.T) {
	waiter := newTestCallbackWaiter(t)
	callbackCh := waitForCallback(t, waiter, "state-value")

	response := requestCallback(t, waiter.RedirectURL()+"?state=state-value&access_token=access-token&refresh_token=refresh-token&code_verifier=verifier")
	if response.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.status, http.StatusBadRequest)
	}
	assertNoSecrets(t, response.body, "state-value", "access-token", "refresh-token", "verifier")
	callback := receiveCallback(t, callbackCh)
	if callback.Error != "missing_code" || callback.State != "state-value" || callback.Code != "" {
		t.Fatalf("callback = %#v, want missing code error", callback)
	}
}

func TestLocalCallbackWaiterContextCancellationReturnsContextError(t *testing.T) {
	waiter := newTestCallbackWaiter(t)
	ctx, cancel := context.WithCancel(context.Background())
	callbackCh := make(chan callbackResult, 1)
	go func() {
		callback, err := waiter.Wait(ctx, "state-value")
		callbackCh <- callbackResult{callback: callback, err: err}
	}()

	cancel()
	result := receiveResult(t, callbackCh)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", result.err)
	}
}

func TestLocalCallbackWaiterFirstValidTerminalCallbackWins(t *testing.T) {
	waiter := newTestCallbackWaiter(t)
	callbackCh := waitForCallback(t, waiter, "state-value")

	first := requestCallback(t, waiter.RedirectURL()+"?code=first-code&state=state-value")
	if first.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", first.status, http.StatusOK)
	}
	callback := receiveCallback(t, callbackCh)
	if callback.Code != "first-code" {
		t.Fatalf("callback code = %q, want first-code", callback.Code)
	}
}

func TestNewLocalCallbackWaiterBindsLoopbackByDefault(t *testing.T) {
	waiter, err := NewLocalCallbackWaiter()
	if err != nil {
		t.Fatal(err)
	}
	defer waiter.Close()

	parsed, err := url.Parse(waiter.RedirectURL())
	if err != nil {
		t.Fatal(err)
	}
	host, _, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" {
		t.Fatalf("host = %q, want 127.0.0.1", host)
	}
}

func TestLocalCallbackWaiterWithConfigUsesConfiguredPathAndAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	waiter, err := NewLocalCallbackWaiterWithConfig(LocalCallbackWaiterConfig{
		Address: address,
		Path:    "/auth/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer waiter.Close()
	if waiter.RedirectURL() != "http://"+address+"/auth/callback" {
		t.Fatalf("redirect URL = %q, want configured address and path", waiter.RedirectURL())
	}

	callbackCh := waitForCallback(t, waiter, "state-value")
	wrongPath := strings.Replace(waiter.RedirectURL(), "/auth/callback", callbackPath, 1)
	response := requestCallback(t, wrongPath+"?code=wrong-code&state=state-value")
	if response.status != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.status, http.StatusNotFound)
	}
	assertNoCallbackYet(t, callbackCh)

	response = requestCallback(t, waiter.RedirectURL()+"?code=auth-code&state=state-value")
	if response.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.status, http.StatusOK)
	}
	callback := receiveCallback(t, callbackCh)
	if callback.Code != "auth-code" || callback.State != "state-value" {
		t.Fatalf("callback = %#v, want configured callback path to complete", callback)
	}
}

func TestLoginClosesCallbackWaiterWhenSupported(t *testing.T) {
	callback := &closableFakeCallbackWaiter{fakeCallbackWaiter: fakeCallbackWaiter{redirectURL: "http://127.0.0.1:8787/callback", callback: Callback{Code: "auth-code"}}}
	flow := testFlow(&recordingBrowser{}, callback, &fakeTokenExchanger{credential: oauthCredential("access-token", "refresh-token")})

	if err := flow.Login(context.Background(), nil, &recordingAuthStore{}); err != nil {
		t.Fatal(err)
	}
	if callback.closes != 1 {
		t.Fatalf("closes = %d, want 1", callback.closes)
	}
}

func newTestCallbackWaiter(t *testing.T) *LocalCallbackWaiter {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	waiter, err := NewLocalCallbackWaiterWithListener(listener)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = waiter.Close() })
	return waiter
}

func waitForCallback(t *testing.T, waiter *LocalCallbackWaiter, state string) <-chan callbackResult {
	t.Helper()
	callbackCh := make(chan callbackResult, 1)
	go func() {
		callback, err := waiter.Wait(context.Background(), state)
		callbackCh <- callbackResult{callback: callback, err: err}
	}()
	return callbackCh
}

func requestCallback(t *testing.T, rawURL string) callbackResponse {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(time.Second)
	var response *http.Response
	var err error
	for {
		response, err = client.Get(rawURL)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return callbackResponse{status: response.StatusCode, body: string(body)}
}

func receiveCallback(t *testing.T, callbackCh <-chan callbackResult) Callback {
	t.Helper()
	result := receiveResult(t, callbackCh)
	if result.err != nil {
		t.Fatalf("wait error = %v", result.err)
	}
	return result.callback
}

func receiveResult(t *testing.T, callbackCh <-chan callbackResult) callbackResult {
	t.Helper()
	select {
	case result := <-callbackCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for callback")
		return callbackResult{}
	}
}

func assertNoCallbackYet(t *testing.T, callbackCh <-chan callbackResult) {
	t.Helper()
	select {
	case result := <-callbackCh:
		t.Fatalf("wait completed early: callback=%#v err=%v", result.callback, result.err)
	case <-time.After(25 * time.Millisecond):
	}
}

type callbackResult struct {
	callback Callback
	err      error
}

type callbackResponse struct {
	status int
	body   string
}

type closableFakeCallbackWaiter struct {
	fakeCallbackWaiter
	closes int
}

func (w *closableFakeCallbackWaiter) Close() error {
	w.closes++
	return nil
}
