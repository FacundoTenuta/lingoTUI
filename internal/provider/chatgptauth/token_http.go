package chatgptauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

type HTTPTokenExchangerConfig struct {
	ClientID      string
	TokenEndpoint string
	HTTPClient    *http.Client
	Now           func() time.Time
}

type HTTPTokenExchanger struct {
	clientID      string
	tokenEndpoint string
	httpClient    *http.Client
	now           func() time.Time
}

type RefreshRequest struct {
	RefreshToken app.Secret
}

func NewHTTPTokenExchanger(config HTTPTokenExchangerConfig) (*HTTPTokenExchanger, error) {
	if config.ClientID == "" {
		return nil, fmt.Errorf("client_id is required")
	}
	if config.TokenEndpoint == "" {
		return nil, fmt.Errorf("token_endpoint is required")
	}
	parsed, err := url.Parse(config.TokenEndpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("token_endpoint must be an absolute URL")
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}

	return &HTTPTokenExchanger{
		clientID:      config.ClientID,
		tokenEndpoint: config.TokenEndpoint,
		httpClient:    httpClient,
		now:           now,
	}, nil
}

func (e *HTTPTokenExchanger) Exchange(ctx context.Context, request TokenRequest) (app.OAuthCredential, error) {
	if request.Code == "" {
		return app.OAuthCredential{}, fmt.Errorf("code is required")
	}
	if request.CodeVerifier == "" {
		return app.OAuthCredential{}, fmt.Errorf("code_verifier is required")
	}
	if request.RedirectURL == "" {
		return app.OAuthCredential{}, fmt.Errorf("redirect_uri is required")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", e.clientID)
	form.Set("code", request.Code)
	form.Set("code_verifier", request.CodeVerifier)
	form.Set("redirect_uri", request.RedirectURL)

	return e.exchange(ctx, form, app.Secret{})
}

func (e *HTTPTokenExchanger) Refresh(ctx context.Context, request RefreshRequest) (app.OAuthCredential, error) {
	if request.RefreshToken.Empty() {
		return app.OAuthCredential{}, fmt.Errorf("refresh_token is required")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", e.clientID)
	form.Set("refresh_token", request.RefreshToken.Value)

	return e.exchange(ctx, form, request.RefreshToken)
}

func (e *HTTPTokenExchanger) exchange(ctx context.Context, form url.Values, fallbackRefreshToken app.Secret) (app.OAuthCredential, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return app.OAuthCredential{}, fmt.Errorf("create token request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := e.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return app.OAuthCredential{}, err
		}
		return app.OAuthCredential{}, fmt.Errorf("send ChatGPT OAuth token request")
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return app.OAuthCredential{}, fmt.Errorf("token endpoint returned status %d", response.StatusCode)
	}

	var token tokenResponse
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return app.OAuthCredential{}, fmt.Errorf("decode token response")
	}

	refreshToken := app.Secret{Value: token.RefreshToken}
	if refreshToken.Empty() {
		refreshToken = fallbackRefreshToken
	}

	return app.OAuthCredential{
		AccessToken:  app.Secret{Value: token.AccessToken},
		RefreshToken: refreshToken,
		ExpiresAt:    e.now().Add(time.Duration(token.ExpiresIn) * time.Second),
		AccountID:    token.AccountID,
	}, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	AccountID    string `json:"account_id"`
}
