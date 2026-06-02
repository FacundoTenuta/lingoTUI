package provider

import "github.com/FacundoTenuta/lingoTUI/internal/app"

type AuthMethod string

const (
	AuthMethodAPIKey  AuthMethod = "api_key"
	AuthMethodBrowser AuthMethod = "browser"
	AuthMethodOAuth   AuthMethod = "oauth"
)

type Provider struct {
	ID          app.ProviderID
	Name        string
	AuthMethods []AuthMethod
	Models      []app.ModelRef
}

func (p Provider) SupportsAuth(method AuthMethod) bool {
	for _, candidate := range p.AuthMethods {
		if candidate == method {
			return true
		}
	}
	return false
}

func (p Provider) ModelsForPurpose(purpose app.ModelPurpose) []app.ModelRef {
	models := make([]app.ModelRef, 0, len(p.Models))
	for _, model := range p.Models {
		if model.Purpose == purpose {
			models = append(models, model)
		}
	}
	return models
}
