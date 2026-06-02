package provider

import (
	"sort"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

type Registry struct{ providers map[app.ProviderID]Provider }

func DefaultRegistry() Registry { return NewRegistry(OpenAI(), ChatGPT()) }

func NewRegistry(providers ...Provider) Registry {
	registry := Registry{providers: map[app.ProviderID]Provider{}}
	for _, candidate := range providers {
		if candidate.ID == "" {
			continue
		}
		registry.providers[candidate.ID] = candidate
	}
	return registry
}

func OpenAI() Provider {
	return Provider{
		ID:          app.ProviderOpenAI,
		Name:        "OpenAI",
		AuthMethods: []AuthMethod{AuthMethodAPIKey},
		Models: []app.ModelRef{
			{Provider: app.ProviderOpenAI, Name: app.DefaultTranscriptionModel, Purpose: app.ModelPurposeTranscription},
			{Provider: app.ProviderOpenAI, Name: app.DefaultChatModel, Purpose: app.ModelPurposeChat},
		},
	}
}

func ChatGPT() Provider {
	return Provider{
		ID:   app.ProviderChatGPT,
		Name: "ChatGPT Plus/Pro (manual opt-in)",
	}
}

func (r Registry) Get(id app.ProviderID) (Provider, bool) {
	provider, ok := r.providers[id]
	return provider, ok
}

func (r Registry) Providers() []Provider {
	providers := make([]Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].ID < providers[j].ID })
	return providers
}

func (r Registry) Models(id app.ProviderID) []app.ModelRef {
	provider, ok := r.Get(id)
	if !ok {
		return nil
	}
	models := make([]app.ModelRef, len(provider.Models))
	copy(models, provider.Models)
	return models
}
