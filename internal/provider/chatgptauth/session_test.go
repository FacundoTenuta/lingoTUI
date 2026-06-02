package chatgptauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestSessionManagerReturnsCurrentAccessTokenWhenNotNearExpiry(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{credential: sessionCredential("current-access-token", "refresh-token", now.Add(time.Hour), "account-id")}
	refresher := &sessionRefresher{credential: oauthCredential("new-access-token", "new-refresh-token")}
	manager := newSessionManager(store, refresher, now)

	token, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if token.Value != "current-access-token" {
		t.Fatalf("AccessToken() = %q, want current token", token.Value)
	}
	if refresher.refreshes != 0 {
		t.Fatalf("Refresh() calls = %d, want 0", refresher.refreshes)
	}
	if store.saves != 0 {
		t.Fatalf("SaveCredential() calls = %d, want 0", store.saves)
	}
}

func TestSessionManagerRefreshesWhenExpired(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "account-id")}
	refresher := &sessionRefresher{credential: sessionOAuth("new-access-token", "new-refresh-token", now.Add(time.Hour), "account-id")}
	manager := newSessionManager(store, refresher, now)

	token, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if token.Value != "new-access-token" {
		t.Fatalf("AccessToken() = %q, want refreshed token", token.Value)
	}
	if refresher.request.RefreshToken.Value != "old-refresh-token" {
		t.Fatalf("Refresh() refresh token = %q, want old refresh token", refresher.request.RefreshToken.Value)
	}
}

func TestSessionManagerRefreshesWithinDefaultSkew(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(5*time.Minute), "account-id")}
	refresher := &sessionRefresher{credential: sessionOAuth("new-access-token", "new-refresh-token", now.Add(time.Hour), "account-id")}
	manager := newSessionManager(store, refresher, now)

	token, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if token.Value != "new-access-token" {
		t.Fatalf("AccessToken() = %q, want refreshed token", token.Value)
	}
	if refresher.refreshes != 1 {
		t.Fatalf("Refresh() calls = %d, want 1", refresher.refreshes)
	}
}

func TestSessionManagerSavesUpdatedOAuthCredentialAfterRefresh(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	store := &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "old-account-id")}
	refresher := &sessionRefresher{credential: sessionOAuth("new-access-token", "new-refresh-token", expiresAt, "new-account-id")}
	manager := newSessionManager(store, refresher, now)

	_, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if store.saves != 1 {
		t.Fatalf("SaveCredential() calls = %d, want 1", store.saves)
	}
	assertSavedOAuth(t, store.saved, sessionOAuth("new-access-token", "new-refresh-token", expiresAt, "new-account-id"))
}

func TestSessionManagerPreservesOldRefreshTokenWhenOmitted(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "account-id")}
	refresher := &sessionRefresher{credential: sessionOAuth("new-access-token", "", now.Add(time.Hour), "account-id")}
	manager := newSessionManager(store, refresher, now)

	_, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if store.saved.OAuth.RefreshToken.Value != "old-refresh-token" {
		t.Fatalf("saved refresh token = %q, want old refresh token", store.saved.OAuth.RefreshToken.Value)
	}
}

func TestSessionManagerPreservesOldAccountIDWhenOmitted(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "old-account-id")}
	refresher := &sessionRefresher{credential: sessionOAuth("new-access-token", "new-refresh-token", now.Add(time.Hour), "")}
	manager := newSessionManager(store, refresher, now)

	_, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if store.saved.OAuth.AccountID != "old-account-id" {
		t.Fatalf("saved account id = %q, want old account id", store.saved.OAuth.AccountID)
	}
}

func TestSessionManagerUpdatesAccountIDWhenReturned(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "old-account-id")}
	refresher := &sessionRefresher{credential: sessionOAuth("new-access-token", "new-refresh-token", now.Add(time.Hour), "new-account-id")}
	manager := newSessionManager(store, refresher, now)

	_, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if store.saved.OAuth.AccountID != "new-account-id" {
		t.Fatalf("saved account id = %q, want new account id", store.saved.OAuth.AccountID)
	}
}

func TestSessionManagerMissingCredentialLoadFail(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{loadErr: errors.New("load failed with access-token and refresh-token")}
	manager := newSessionManager(store, &sessionRefresher{}, now)

	token, err := manager.AccessToken(context.Background())
	if !errors.Is(err, ErrMissingOAuthCredential) {
		t.Fatalf("AccessToken() error = %v, want ErrMissingOAuthCredential", err)
	}
	if !token.Empty() {
		t.Fatalf("AccessToken() returned token %q on error", token.Value)
	}
	assertNoSessionSecrets(t, err.Error(), "access-token", "refresh-token")
}

func TestSessionManagerExpiredMissingRefreshToken(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{credential: sessionCredential("old-access-token", "", now.Add(-time.Second), "account-id")}
	manager := newSessionManager(store, &sessionRefresher{}, now)

	token, err := manager.AccessToken(context.Background())
	if !errors.Is(err, ErrMissingRefreshToken) {
		t.Fatalf("AccessToken() error = %v, want ErrMissingRefreshToken", err)
	}
	if !token.Empty() {
		t.Fatalf("AccessToken() returned token %q on error", token.Value)
	}
}

func TestSessionManagerRefreshedMissingAccessToken(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "account-id")}
	refresher := &sessionRefresher{credential: sessionOAuth("", "new-refresh-token", now.Add(time.Hour), "account-id")}
	manager := newSessionManager(store, refresher, now)

	token, err := manager.AccessToken(context.Background())
	if !errors.Is(err, ErrMissingAccessToken) {
		t.Fatalf("AccessToken() error = %v, want ErrMissingAccessToken", err)
	}
	if !token.Empty() {
		t.Fatalf("AccessToken() returned token %q on error", token.Value)
	}
	if store.saves != 0 {
		t.Fatalf("SaveCredential() calls = %d, want 0", store.saves)
	}
}

func TestSessionManagerSaveErrorReturnsSanitizedErrorAndNoToken(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{
		credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "account-id"),
		saveErr:    errors.New("save failed with new-access-token old-refresh-token"),
	}
	refresher := &sessionRefresher{credential: sessionOAuth("new-access-token", "new-refresh-token", now.Add(time.Hour), "account-id")}
	manager := newSessionManager(store, refresher, now)

	token, err := manager.AccessToken(context.Background())
	if err == nil {
		t.Fatalf("AccessToken() error = nil, want save error")
	}
	if !token.Empty() {
		t.Fatalf("AccessToken() returned token %q on error", token.Value)
	}
	assertNoSessionSecrets(t, err.Error(), "new-access-token", "new-refresh-token", "old-refresh-token")
}

func TestSessionManagerErrorsDoNotLeakAccessOrRefreshTokens(t *testing.T) {
	tests := []struct {
		name      string
		store     *sessionStore
		refresher *sessionRefresher
	}{
		{
			name:      "refresh error",
			store:     &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", time.Time{}, "account-id")},
			refresher: &sessionRefresher{err: errors.New("refresh failed with old-refresh-token and new-access-token")},
		},
		{
			name:      "wrong credential",
			store:     &sessionStore{credential: app.Credential{Provider: app.ProviderOpenAI, Kind: app.CredentialKindAPIKey, APIKey: app.Secret{Value: "api-key"}}},
			refresher: &sessionRefresher{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newSessionManager(tt.store, tt.refresher, time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC))

			_, err := manager.AccessToken(context.Background())
			if err == nil {
				t.Fatalf("AccessToken() error = nil, want error")
			}
			assertNoSessionSecrets(t, err.Error(), "old-access-token", "old-refresh-token", "new-access-token", "new-refresh-token", "api-key")
		})
	}
}

func TestSessionManagerContextCancellationPropagates(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "account-id")}
	manager := newSessionManager(store, &sessionRefresher{}, now)

	_, err := manager.AccessToken(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("AccessToken() error = %v, want context.Canceled", err)
	}
}

func TestSessionManagerContextWrappedErrorsDoNotLeakSecrets(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		store     *sessionStore
		refresher *sessionRefresher
		want      error
	}{
		{
			name:      "load",
			store:     &sessionStore{loadErr: fmt.Errorf("load failed with old-refresh-token: %w", context.Canceled)},
			refresher: &sessionRefresher{},
			want:      context.Canceled,
		},
		{
			name:      "refresh",
			store:     &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "account-id")},
			refresher: &sessionRefresher{err: fmt.Errorf("refresh failed with old-refresh-token: %w", context.DeadlineExceeded)},
			want:      context.DeadlineExceeded,
		},
		{
			name:      "save",
			store:     &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "account-id"), saveErr: fmt.Errorf("save failed with new-access-token: %w", context.Canceled)},
			refresher: &sessionRefresher{credential: sessionOAuth("new-access-token", "new-refresh-token", now.Add(time.Hour), "account-id")},
			want:      context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newSessionManager(tt.store, tt.refresher, now)

			token, err := manager.AccessToken(context.Background())
			if err != tt.want {
				t.Fatalf("AccessToken() error = %v, want exact sentinel %v", err, tt.want)
			}
			if !token.Empty() {
				t.Fatalf("AccessToken() returned token %q on error", token.Value)
			}
			assertNoSessionSecrets(t, err.Error(), "old-access-token", "old-refresh-token", "new-access-token", "new-refresh-token")
		})
	}
}

func TestSessionManagerNilDependenciesReturnSanitizedErrors(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

	token, err := SessionManager{Now: func() time.Time { return now }}.AccessToken(context.Background())
	if !errors.Is(err, ErrMissingOAuthCredential) || !token.Empty() {
		t.Fatalf("nil store token=%q error=%v, want missing credential and no token", token.Value, err)
	}

	store := &sessionStore{credential: sessionCredential("old-access-token", "old-refresh-token", now.Add(-time.Second), "account-id")}
	token, err = SessionManager{Store: store, Now: func() time.Time { return now }}.AccessToken(context.Background())
	if err == nil || !token.Empty() || strings.Contains(err.Error(), "old-refresh-token") {
		t.Fatalf("nil refresher token=%q error=%v, want sanitized refresh error", token.Value, err)
	}
}

func TestSessionManagerAccountIDReturnsStoredAccountIDWithoutRefresh(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &sessionStore{credential: sessionCredential("access-token", "refresh-token", now.Add(-time.Hour), "account-id")}
	refresher := &sessionRefresher{credential: sessionOAuth("new-access-token", "new-refresh-token", now.Add(time.Hour), "new-account-id")}
	manager := newSessionManager(store, refresher, now)

	accountID, err := manager.AccountID(context.Background())
	if err != nil {
		t.Fatalf("AccountID() error = %v", err)
	}
	if accountID != "account-id" {
		t.Fatalf("AccountID() = %q, want stored account id", accountID)
	}
	if refresher.refreshes != 0 || store.saves != 0 {
		t.Fatalf("AccountID() side effects: refreshes=%d saves=%d, want 0/0", refresher.refreshes, store.saves)
	}
}

func TestSessionManagerAccountIDReturnsEmptyStoredAccountID(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	manager := newSessionManager(&sessionStore{credential: sessionCredential("access-token", "refresh-token", now.Add(time.Hour), "")}, &sessionRefresher{}, now)

	accountID, err := manager.AccountID(context.Background())
	if err != nil {
		t.Fatalf("AccountID() error = %v", err)
	}
	if accountID != "" {
		t.Fatalf("AccountID() = %q, want empty", accountID)
	}
}

func TestSessionManagerAccountIDMissingCredentialReturnsSanitizedError(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	manager := newSessionManager(&sessionStore{loadErr: errors.New("load failed with access-token refresh-token account-id")}, &sessionRefresher{}, now)

	accountID, err := manager.AccountID(context.Background())
	if !errors.Is(err, ErrMissingOAuthCredential) {
		t.Fatalf("AccountID() error = %v, want ErrMissingOAuthCredential", err)
	}
	if accountID != "" {
		t.Fatalf("AccountID() = %q, want empty", accountID)
	}
	assertNoSessionSecrets(t, err.Error(), "access-token", "refresh-token", "account-id")
}

func TestSessionManagerAccountIDRejectsWrongCredentialShape(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		credential app.Credential
	}{
		{
			name: "wrong provider",
			credential: app.Credential{
				Provider: app.ProviderOpenAI,
				Kind:     app.CredentialKindOAuth,
				OAuth:    sessionOAuth("access-token", "refresh-token", now.Add(time.Hour), "account-id"),
			},
		},
		{
			name: "wrong kind",
			credential: app.Credential{
				Provider: app.ProviderChatGPT,
				Kind:     app.CredentialKindAPIKey,
				APIKey:   app.Secret{Value: "api-key"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newSessionManager(&sessionStore{credential: tt.credential}, &sessionRefresher{}, now)

			accountID, err := manager.AccountID(context.Background())
			if !errors.Is(err, ErrMissingOAuthCredential) {
				t.Fatalf("AccountID() error = %v, want ErrMissingOAuthCredential", err)
			}
			if accountID != "" {
				t.Fatalf("AccountID() = %q, want empty", accountID)
			}
			assertNoSessionSecrets(t, err.Error(), "access-token", "refresh-token", "account-id", "api-key")
		})
	}
}

func TestSessionManagerAccountIDNilStoreReturnsMissingCredential(t *testing.T) {
	accountID, err := SessionManager{}.AccountID(context.Background())
	if !errors.Is(err, ErrMissingOAuthCredential) {
		t.Fatalf("AccountID() error = %v, want ErrMissingOAuthCredential", err)
	}
	if accountID != "" {
		t.Fatalf("AccountID() = %q, want empty", accountID)
	}
}

func TestSessionManagerAccountIDContextSentinelsPreserved(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "canceled", err: context.Canceled},
		{name: "deadline", err: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
			manager := newSessionManager(&sessionStore{loadErr: fmt.Errorf("load failed with account-id: %w", tt.err)}, &sessionRefresher{}, now)

			_, err := manager.AccountID(context.Background())
			if err != tt.err {
				t.Fatalf("AccountID() error = %v, want exact %v", err, tt.err)
			}
		})
	}
}

func newSessionManager(store *sessionStore, refresher *sessionRefresher, now time.Time) SessionManager {
	return SessionManager{
		Store:     store,
		Refresher: refresher,
		Now:       func() time.Time { return now },
	}
}

func sessionCredential(accessToken, refreshToken string, expiresAt time.Time, accountID string) app.Credential {
	return app.Credential{
		Provider: app.ProviderChatGPT,
		Kind:     app.CredentialKindOAuth,
		OAuth:    sessionOAuth(accessToken, refreshToken, expiresAt, accountID),
	}
}

func sessionOAuth(accessToken, refreshToken string, expiresAt time.Time, accountID string) app.OAuthCredential {
	return app.OAuthCredential{
		AccessToken:  app.Secret{Value: accessToken},
		RefreshToken: app.Secret{Value: refreshToken},
		ExpiresAt:    expiresAt,
		AccountID:    accountID,
	}
}

func assertSavedOAuth(t *testing.T, credential app.Credential, want app.OAuthCredential) {
	t.Helper()
	if credential.Provider != app.ProviderChatGPT || credential.Kind != app.CredentialKindOAuth {
		t.Fatalf("saved credential = %#v, want ChatGPT OAuth", credential)
	}
	if credential.OAuth.AccessToken.Value != want.AccessToken.Value {
		t.Fatalf("saved access token = %q, want %q", credential.OAuth.AccessToken.Value, want.AccessToken.Value)
	}
	if credential.OAuth.RefreshToken.Value != want.RefreshToken.Value {
		t.Fatalf("saved refresh token = %q, want %q", credential.OAuth.RefreshToken.Value, want.RefreshToken.Value)
	}
	if !credential.OAuth.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("saved expires at = %v, want %v", credential.OAuth.ExpiresAt, want.ExpiresAt)
	}
	if credential.OAuth.AccountID != want.AccountID {
		t.Fatalf("saved account id = %q, want %q", credential.OAuth.AccountID, want.AccountID)
	}
}

func assertNoSessionSecrets(t *testing.T, output string, secrets ...string) {
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

type sessionStore struct {
	credential app.Credential
	saved      app.Credential
	saves      int
	loadErr    error
	saveErr    error
}

func (s *sessionStore) SaveCredential(ctx context.Context, credential app.Credential) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saves++
	s.saved = credential
	return nil
}

func (s *sessionStore) LoadCredential(ctx context.Context, provider app.ProviderID, kind app.CredentialKind) (app.Credential, error) {
	if err := ctx.Err(); err != nil {
		return app.Credential{}, err
	}
	if s.loadErr != nil {
		return app.Credential{}, s.loadErr
	}
	return s.credential, nil
}

func (s *sessionStore) DeleteCredential(context.Context, app.ProviderID, app.CredentialKind) error {
	return nil
}

type sessionRefresher struct {
	credential app.OAuthCredential
	request    RefreshRequest
	refreshes  int
	err        error
}

func (r *sessionRefresher) Refresh(ctx context.Context, request RefreshRequest) (app.OAuthCredential, error) {
	if err := ctx.Err(); err != nil {
		return app.OAuthCredential{}, err
	}
	r.refreshes++
	r.request = request
	if r.err != nil {
		return app.OAuthCredential{}, r.err
	}
	return r.credential, nil
}
