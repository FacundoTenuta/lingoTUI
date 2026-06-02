package chatgptauth

import (
	"fmt"
	"net/url"
	"strings"
)

func authURL(endpoint string, params authURLParams) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse auth endpoint: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("auth endpoint must be absolute")
	}
	values := parsed.Query()
	values.Set("client_id", params.clientID)
	values.Set("redirect_uri", params.redirectURL)
	values.Set("scope", strings.Join(params.scopes, " "))
	values.Set("state", params.state)
	values.Set("code_challenge", params.codeChallenge)
	values.Set("code_challenge_method", "S256")
	values.Set("response_type", "code")
	for key, value := range params.extraParams {
		if len(value) == 0 {
			continue
		}
		values[key] = append([]string(nil), value...)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

type authURLParams struct {
	clientID      string
	redirectURL   string
	scopes        []string
	state         string
	codeChallenge string
	extraParams   url.Values
}
