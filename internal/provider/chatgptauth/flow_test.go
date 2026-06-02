package chatgptauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestLoginAuthURLIncludesOAuthAndPKCEParameters(t *testing.T) {
	browser := &recordingBrowser{}
	callback := &fakeCallbackWaiter{redirectURL: "http://127.0.0.1:8787/callback", callback: Callback{Code: "auth-code"}}
	exchanger := &fakeTokenExchanger{credential: oauthCredential("access-token", "refresh-token")}
	store := &recordingAuthStore{}
	flow := testFlow(browser, callback, exchanger)
	flow.ExtraParams = url.Values{
		"codex_cli_simplified_flow":  {"true"},
		"id_token_add_organizations": {"true"},
		"originator":                 {"opencode"},
	}

	if err := flow.Login(context.Background(), nil, store); err != nil {
		t.Fatal(err)
	}

	parsed, err := url.Parse(browser.openedURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "auth.example.test" || parsed.Path != "/oauth/authorize" {
		t.Fatalf("auth URL = %q, want configured endpoint", browser.openedURL)
	}
	assertQuery(t, query, "client_id", "client-id")
	assertQuery(t, query, "redirect_uri", "http://127.0.0.1:8787/callback")
	assertQuery(t, query, "scope", "openid profile offline_access")
	assertQuery(t, query, "code_challenge_method", "S256")
	assertQuery(t, query, "response_type", "code")
	assertQuery(t, query, "codex_cli_simplified_flow", "true")
	assertQuery(t, query, "id_token_add_organizations", "true")
	assertQuery(t, query, "originator", "opencode")
	if query.Get("state") == "" {
		t.Fatal("state is empty")
	}
	if query.Get("code_challenge") == "" {
		t.Fatal("code_challenge is empty")
	}
	if callback.wantState != query.Get("state") {
		t.Fatalf("wait state = %q, want auth URL state %q", callback.wantState, query.Get("state"))
	}
}

func TestLoginSuccessExchangesAndSavesChatGPTOAuthCredential(t *testing.T) {
	browser := &recordingBrowser{}
	callback := &fakeCallbackWaiter{redirectURL: "http://127.0.0.1:8787/callback", callback: Callback{Code: "auth-code"}}
	exchanger := &fakeTokenExchanger{credential: oauthCredential("access-token", "refresh-token")}
	store := &recordingAuthStore{}
	var stdout bytes.Buffer

	if err := testFlow(browser, callback, exchanger).Login(context.Background(), &stdout, store); err != nil {
		t.Fatal(err)
	}

	if browser.opens != 1 || browser.openedURL == "" {
		t.Fatalf("browser opens=%d url=%q, want one open", browser.opens, browser.openedURL)
	}
	if callback.waits != 1 {
		t.Fatalf("callback waits = %d, want 1", callback.waits)
	}
	if exchanger.exchanges != 1 {
		t.Fatalf("exchanges = %d, want 1", exchanger.exchanges)
	}
	if exchanger.request.Code != "auth-code" {
		t.Fatalf("exchange code = %q, want auth-code", exchanger.request.Code)
	}
	if exchanger.request.CodeVerifier == "" {
		t.Fatal("exchange code verifier is empty")
	}
	if exchanger.request.RedirectURL != "http://127.0.0.1:8787/callback" {
		t.Fatalf("exchange redirect URL = %q", exchanger.request.RedirectURL)
	}
	if store.saves != 1 {
		t.Fatalf("saves = %d, want 1", store.saves)
	}
	if store.credential.Provider != app.ProviderChatGPT || store.credential.Kind != app.CredentialKindOAuth {
		t.Fatalf("saved credential = %+v, want ChatGPT OAuth", store.credential)
	}
	if store.credential.OAuth.AccessToken.Value != "access-token" || store.credential.OAuth.RefreshToken.Value != "refresh-token" {
		t.Fatalf("saved OAuth credential = %#v", store.credential.OAuth)
	}
	assertNoSecrets(t, stdout.String(), "access-token", "refresh-token", "auth-code", exchanger.request.CodeVerifier)
}

func TestLoginMismatchedStateSavesNothingAndDoesNotExchange(t *testing.T) {
	exchanger := &fakeTokenExchanger{credential: oauthCredential("access-token", "refresh-token")}
	store := &recordingAuthStore{}
	callback := &fakeCallbackWaiter{redirectURL: "http://127.0.0.1:8787/callback", callback: Callback{Code: "auth-code", State: "wrong-state"}}

	err := testFlow(&recordingBrowser{}, callback, exchanger).Login(context.Background(), nil, store)
	if err == nil || !strings.Contains(err.Error(), "state mismatch") {
		t.Fatalf("error = %v, want state mismatch", err)
	}
	if exchanger.exchanges != 0 || store.saves != 0 {
		t.Fatalf("side effects: exchanges=%d saves=%d", exchanger.exchanges, store.saves)
	}
}

func TestLoginCallbackOAuthErrorSavesNothing(t *testing.T) {
	exchanger := &fakeTokenExchanger{credential: oauthCredential("access-token", "refresh-token")}
	store := &recordingAuthStore{}
	callback := &fakeCallbackWaiter{redirectURL: "http://127.0.0.1:8787/callback", callback: Callback{Error: "access_denied"}}

	err := testFlow(&recordingBrowser{}, callback, exchanger).Login(context.Background(), nil, store)
	if err == nil || !strings.Contains(err.Error(), "callback returned an error") {
		t.Fatalf("error = %v, want callback error", err)
	}
	if exchanger.exchanges != 0 || store.saves != 0 {
		t.Fatalf("side effects: exchanges=%d saves=%d", exchanger.exchanges, store.saves)
	}
}

func TestLoginBrowserOpenErrorSavesNothing(t *testing.T) {
	browser := &recordingBrowser{err: errors.New("browser failed with access-token refresh-token")}
	exchanger := &fakeTokenExchanger{credential: oauthCredential("access-token", "refresh-token")}
	store := &recordingAuthStore{}

	err := testFlow(browser, &fakeCallbackWaiter{redirectURL: "http://127.0.0.1:8787/callback"}, exchanger).Login(context.Background(), nil, store)
	if err == nil || !strings.Contains(err.Error(), "open browser") {
		t.Fatalf("error = %v, want browser error", err)
	}
	if store.saves != 0 {
		t.Fatalf("saves = %d, want 0", store.saves)
	}
	assertNoSecrets(t, err.Error(), "access-token", "refresh-token")
}

func TestLoginExchangeErrorSavesNothingAndRedactsSecrets(t *testing.T) {
	store := &recordingAuthStore{}
	exchanger := &fakeTokenExchanger{err: errors.New("exchange failed with access-token refresh-token verifier")}
	callback := &fakeCallbackWaiter{redirectURL: "http://127.0.0.1:8787/callback", callback: Callback{Code: "auth-code"}}
	var stdout bytes.Buffer

	err := testFlow(&recordingBrowser{}, callback, exchanger).Login(context.Background(), &stdout, store)
	if err == nil || !strings.Contains(err.Error(), "exchange ChatGPT OAuth token") {
		t.Fatalf("error = %v, want exchange error", err)
	}
	if store.saves != 0 {
		t.Fatalf("saves = %d, want 0", store.saves)
	}
	assertNoSecrets(t, err.Error(), "access-token", "refresh-token", "auth-code", exchanger.request.CodeVerifier)
	assertNoSecrets(t, stdout.String(), "access-token", "refresh-token", "auth-code", exchanger.request.CodeVerifier)
}

func TestLoginContextCancellationBeforeSave(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &recordingAuthStore{}
	exchanger := &fakeTokenExchanger{
		credential: oauthCredential("access-token", "refresh-token"),
		after:      cancel,
	}
	callback := &fakeCallbackWaiter{redirectURL: "http://127.0.0.1:8787/callback", callback: Callback{Code: "auth-code"}}

	err := testFlow(&recordingBrowser{}, callback, exchanger).Login(ctx, nil, store)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if store.saves != 0 {
		t.Fatalf("saves = %d, want 0", store.saves)
	}
}

func TestLoginTimeoutBeforeSave(t *testing.T) {
	store := &recordingAuthStore{}
	exchanger := &fakeTokenExchanger{
		credential: oauthCredential("access-token", "refresh-token"),
		after:      func() { time.Sleep(20 * time.Millisecond) },
	}
	callback := &fakeCallbackWaiter{redirectURL: "http://127.0.0.1:8787/callback", callback: Callback{Code: "auth-code"}}
	flow := testFlow(&recordingBrowser{}, callback, exchanger)
	flow.Timeout = time.Millisecond

	err := flow.Login(context.Background(), nil, store)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
	if store.saves != 0 {
		t.Fatalf("saves = %d, want 0", store.saves)
	}
}

func TestPKCEVerifierAndChallengeAreURLSafeWithoutPadding(t *testing.T) {
	verifier, challenge, err := newPKCE(bytes.NewReader(bytes.Repeat([]byte{0xff}, pkceRandomBytes)))
	if err != nil {
		t.Fatal(err)
	}
	urlSafe := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	for name, value := range map[string]string{"verifier": verifier, "challenge": challenge} {
		if !urlSafe.MatchString(value) {
			t.Fatalf("%s = %q, want URL-safe base64url", name, value)
		}
		if strings.Contains(value, "=") {
			t.Fatalf("%s = %q, want no padding", name, value)
		}
	}
}

func TestPKCEChallengeIsS256HashOfVerifier(t *testing.T) {
	verifier, challenge, err := newPKCE(bytes.NewReader(bytes.Repeat([]byte{0x7a}, pkceRandomBytes)))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(hash[:])
	if challenge != want {
		t.Fatalf("challenge = %q, want S256 hash %q", challenge, want)
	}
}

func testFlow(browser Browser, callback CallbackWaiter, exchanger TokenExchanger) Flow {
	return Flow{
		ClientID:     "client-id",
		AuthEndpoint: "https://auth.example.test/oauth/authorize",
		Scopes:       []string{"openid", "profile", "offline_access"},
		Browser:      browser,
		Callback:     callback,
		Exchanger:    exchanger,
		Rand:         bytes.NewReader(bytes.Repeat([]byte{0x42}, 128)),
	}
}

func assertQuery(t *testing.T, query url.Values, key, want string) {
	t.Helper()
	if got := query.Get(key); got != want {
		t.Fatalf("query[%s] = %q, want %q", key, got, want)
	}
}

func assertNoSecrets(t *testing.T, output string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		if strings.Contains(output, secret) {
			t.Fatalf("output exposed secret %q: %q", secret, output)
		}
	}
}

func oauthCredential(accessToken, refreshToken string) app.OAuthCredential {
	return app.OAuthCredential{
		AccessToken:  app.Secret{Value: accessToken},
		RefreshToken: app.Secret{Value: refreshToken},
		ExpiresAt:    time.Now().Add(time.Hour),
		AccountID:    "account-id",
	}
}

type recordingBrowser struct {
	opens     int
	openedURL string
	err       error
}

func (b *recordingBrowser) Open(ctx context.Context, authURL string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.opens++
	b.openedURL = authURL
	return b.err
}

type fakeCallbackWaiter struct {
	redirectURL string
	callback    Callback
	waits       int
	wantState   string
	err         error
}

func (w *fakeCallbackWaiter) RedirectURL() string { return w.redirectURL }

func (w *fakeCallbackWaiter) Wait(ctx context.Context, state string) (Callback, error) {
	if err := ctx.Err(); err != nil {
		return Callback{}, err
	}
	w.waits++
	w.wantState = state
	if w.err != nil {
		return Callback{}, w.err
	}
	callback := w.callback
	if callback.State == "" {
		callback.State = state
	}
	return callback, nil
}

type fakeTokenExchanger struct {
	credential app.OAuthCredential
	request    TokenRequest
	exchanges  int
	err        error
	after      func()
}

func (e *fakeTokenExchanger) Exchange(ctx context.Context, request TokenRequest) (app.OAuthCredential, error) {
	if err := ctx.Err(); err != nil {
		return app.OAuthCredential{}, err
	}
	e.exchanges++
	e.request = request
	if e.after != nil {
		e.after()
	}
	if e.err != nil {
		return app.OAuthCredential{}, e.err
	}
	return e.credential, nil
}

type recordingAuthStore struct {
	credential app.Credential
	saves      int
	err        error
}

func (s *recordingAuthStore) SaveCredential(ctx context.Context, credential app.Credential) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.err != nil {
		return s.err
	}
	s.saves++
	s.credential = credential
	return nil
}

func (s *recordingAuthStore) LoadCredential(context.Context, app.ProviderID, app.CredentialKind) (app.Credential, error) {
	return s.credential, s.err
}

func (s *recordingAuthStore) DeleteCredential(context.Context, app.ProviderID, app.CredentialKind) error {
	return s.err
}
