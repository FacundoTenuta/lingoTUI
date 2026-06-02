package chatgptauth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

const stateRandomBytes = 32

type Browser interface {
	Open(context.Context, string) error
}

type CallbackWaiter interface {
	RedirectURL() string
	Wait(context.Context, string) (Callback, error)
}

type callbackCloser interface {
	Close() error
}

type TokenExchanger interface {
	Exchange(context.Context, TokenRequest) (app.OAuthCredential, error)
}

type Flow struct {
	ClientID     string
	AuthEndpoint string
	Scopes       []string
	ExtraParams  url.Values
	Browser      Browser
	Callback     CallbackWaiter
	Exchanger    TokenExchanger
	Rand         io.Reader
	Timeout      time.Duration
}

type Callback struct {
	Code  string
	State string
	Error string
}

type TokenRequest struct {
	Code         string
	CodeVerifier string
	RedirectURL  string
}

func (f Flow) Login(ctx context.Context, stdout io.Writer, store app.AuthCredentialStore) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if err := f.validate(); err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("credential store unavailable")
	}
	if closer, ok := f.Callback.(callbackCloser); ok {
		defer closer.Close()
	}
	if f.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.Timeout)
		defer cancel()
	}

	randReader := f.Rand
	if randReader == nil {
		randReader = rand.Reader
	}
	state, err := randomURLSafe(randReader, stateRandomBytes)
	if err != nil {
		return fmt.Errorf("prepare ChatGPT OAuth login: %w", err)
	}
	verifier, challenge, err := newPKCE(randReader)
	if err != nil {
		return fmt.Errorf("prepare ChatGPT OAuth login: %w", err)
	}
	redirectURL := f.Callback.RedirectURL()
	authURL, err := authURL(f.AuthEndpoint, authURLParams{
		clientID:      f.ClientID,
		redirectURL:   redirectURL,
		scopes:        f.Scopes,
		state:         state,
		codeChallenge: challenge,
		extraParams:   f.ExtraParams,
	})
	if err != nil {
		return err
	}

	fprintln(stdout, "Opening browser for ChatGPT OAuth login...")
	if err := f.Browser.Open(ctx, authURL); err != nil {
		if isContextError(err) {
			return err
		}
		return fmt.Errorf("open browser for ChatGPT OAuth login")
	}
	callback, err := f.Callback.Wait(ctx, state)
	if err != nil {
		if isContextError(err) {
			return err
		}
		return fmt.Errorf("wait for ChatGPT OAuth callback")
	}
	if callback.Error != "" {
		return fmt.Errorf("ChatGPT OAuth callback returned an error")
	}
	if callback.State != state {
		return fmt.Errorf("ChatGPT OAuth callback state mismatch")
	}
	if callback.Code == "" {
		return fmt.Errorf("ChatGPT OAuth callback missing code")
	}
	credential, err := f.Exchanger.Exchange(ctx, TokenRequest{
		Code:         callback.Code,
		CodeVerifier: verifier,
		RedirectURL:  redirectURL,
	})
	if err != nil {
		if isContextError(err) {
			return err
		}
		return fmt.Errorf("exchange ChatGPT OAuth token")
	}
	if credential.AccessToken.Empty() && credential.RefreshToken.Empty() {
		return fmt.Errorf("ChatGPT OAuth token exchange returned no tokens")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := store.SaveCredential(ctx, app.Credential{
		Provider: app.ProviderChatGPT,
		Kind:     app.CredentialKindOAuth,
		OAuth:    credential,
	}); err != nil {
		return fmt.Errorf("save ChatGPT OAuth credential")
	}
	return nil
}

func (f Flow) validate() error {
	if f.ClientID == "" {
		return fmt.Errorf("ChatGPT OAuth client ID is required")
	}
	if f.AuthEndpoint == "" {
		return fmt.Errorf("ChatGPT OAuth auth endpoint is required")
	}
	if f.Browser == nil {
		return fmt.Errorf("ChatGPT OAuth browser opener is required")
	}
	if f.Callback == nil {
		return fmt.Errorf("ChatGPT OAuth callback waiter is required")
	}
	if f.Exchanger == nil {
		return fmt.Errorf("ChatGPT OAuth token exchanger is required")
	}
	return nil
}

func fprintln(w io.Writer, text string) {
	_, _ = fmt.Fprintln(w, text)
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
