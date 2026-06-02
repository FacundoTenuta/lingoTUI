package chatgptauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

var (
	ErrMissingOAuthCredential = errors.New("missing ChatGPT OAuth credential")
	ErrMissingRefreshToken    = errors.New("missing ChatGPT OAuth refresh token")
	ErrMissingAccessToken     = errors.New("missing ChatGPT OAuth access token")
)

type Refresher interface {
	Refresh(context.Context, RefreshRequest) (app.OAuthCredential, error)
}

type SessionManager struct {
	Store      app.AuthCredentialStore
	Refresher  Refresher
	Now        func() time.Time
	ExpirySkew time.Duration
}

func (m SessionManager) AccessToken(ctx context.Context) (app.Secret, error) {
	if m.Store == nil {
		return app.Secret{}, fmt.Errorf("load ChatGPT OAuth credential: %w", ErrMissingOAuthCredential)
	}
	credential, err := m.Store.LoadCredential(ctx, app.ProviderChatGPT, app.CredentialKindOAuth)
	if err != nil {
		if contextErr := cleanContextError(err); contextErr != nil {
			return app.Secret{}, contextErr
		}
		return app.Secret{}, fmt.Errorf("load ChatGPT OAuth credential: %w", ErrMissingOAuthCredential)
	}
	if credential.Provider != app.ProviderChatGPT || credential.Kind != app.CredentialKindOAuth {
		return app.Secret{}, ErrMissingOAuthCredential
	}

	oauth := credential.OAuth
	if !oauth.AccessToken.Empty() && !shouldRefresh(oauth.ExpiresAt, m.now(), m.expirySkew()) {
		return oauth.AccessToken, nil
	}
	if oauth.RefreshToken.Empty() {
		return app.Secret{}, ErrMissingRefreshToken
	}
	if m.Refresher == nil {
		return app.Secret{}, fmt.Errorf("refresh ChatGPT OAuth credential")
	}

	refreshed, err := m.Refresher.Refresh(ctx, RefreshRequest{RefreshToken: oauth.RefreshToken})
	if err != nil {
		if contextErr := cleanContextError(err); contextErr != nil {
			return app.Secret{}, contextErr
		}
		return app.Secret{}, fmt.Errorf("refresh ChatGPT OAuth credential")
	}
	if refreshed.AccessToken.Empty() {
		return app.Secret{}, ErrMissingAccessToken
	}
	if refreshed.RefreshToken.Empty() {
		refreshed.RefreshToken = oauth.RefreshToken
	}
	if refreshed.AccountID == "" {
		refreshed.AccountID = oauth.AccountID
	}

	updated := app.Credential{
		Provider: app.ProviderChatGPT,
		Kind:     app.CredentialKindOAuth,
		OAuth:    refreshed,
	}
	if err := m.Store.SaveCredential(ctx, updated); err != nil {
		if contextErr := cleanContextError(err); contextErr != nil {
			return app.Secret{}, contextErr
		}
		return app.Secret{}, fmt.Errorf("save ChatGPT OAuth credential")
	}
	return refreshed.AccessToken, nil
}

func (m SessionManager) AccountID(ctx context.Context) (string, error) {
	if m.Store == nil {
		return "", fmt.Errorf("load ChatGPT OAuth credential: %w", ErrMissingOAuthCredential)
	}
	credential, err := m.Store.LoadCredential(ctx, app.ProviderChatGPT, app.CredentialKindOAuth)
	if err != nil {
		if contextErr := cleanContextError(err); contextErr != nil {
			return "", contextErr
		}
		return "", fmt.Errorf("load ChatGPT OAuth credential: %w", ErrMissingOAuthCredential)
	}
	if credential.Provider != app.ProviderChatGPT || credential.Kind != app.CredentialKindOAuth {
		return "", ErrMissingOAuthCredential
	}
	return credential.OAuth.AccountID, nil
}

func cleanContextError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}

func shouldRefresh(expiresAt time.Time, now time.Time, skew time.Duration) bool {
	return expiresAt.IsZero() || !expiresAt.After(now.Add(skew))
}

func (m SessionManager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m SessionManager) expirySkew() time.Duration {
	if m.ExpirySkew != 0 {
		return m.ExpirySkew
	}
	return 5 * time.Minute
}
