package credentials

import (
	"errors"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

const FileName = "auth.json"

var ErrSecretNotFound = errors.New("credential not found")
var ErrStoreUnavailable = errors.New("credential store unavailable")

type Record struct {
	Provider app.ProviderID `json:"provider"`
	Secret   app.Secret     `json:"secret"`
}

func credentialAccount(provider app.ProviderID, kind app.CredentialKind) string {
	return string(provider) + ":" + string(kind)
}

func (r Record) Redacted() Record {
	r.Secret = app.Secret{Value: r.Secret.Redacted()}
	return r
}
