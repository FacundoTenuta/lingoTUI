package credentials

import (
	"context"
	"errors"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

type CompositeStore struct {
	primary  app.CredentialStore
	fallback app.CredentialStore
	path     string
}

func NewCompositeStore(primary, fallback app.CredentialStore, path string) *CompositeStore {
	return &CompositeStore{primary: primary, fallback: fallback, path: path}
}

func (s *CompositeStore) Path() string { return s.path }

func (s *CompositeStore) Save(ctx context.Context, provider app.ProviderID, secret app.Secret) error {
	if s.primary == nil {
		return ErrStoreUnavailable
	}
	return s.primary.Save(ctx, provider, secret)
}

func (s *CompositeStore) SaveCredential(ctx context.Context, credential app.Credential) error {
	primary, ok := s.primary.(app.AuthCredentialStore)
	if s.primary == nil || !ok {
		return ErrStoreUnavailable
	}
	return primary.SaveCredential(ctx, credential)
}

func (s *CompositeStore) LoadCredential(ctx context.Context, provider app.ProviderID, kind app.CredentialKind) (app.Credential, error) {
	if primary, ok := s.primary.(app.AuthCredentialStore); s.primary != nil && ok {
		credential, err := primary.LoadCredential(ctx, provider, kind)
		if err == nil {
			return credential, nil
		}
		if !isFallbackEligible(err) {
			return app.Credential{}, err
		}
	}
	if fallback, ok := s.fallback.(app.AuthCredentialStore); s.fallback != nil && ok {
		return fallback.LoadCredential(ctx, provider, kind)
	}
	return app.Credential{}, ErrSecretNotFound
}

func (s *CompositeStore) DeleteCredential(ctx context.Context, provider app.ProviderID, kind app.CredentialKind) error {
	var firstErr error
	if primary, ok := s.primary.(app.AuthCredentialStore); s.primary != nil && ok {
		if err := primary.DeleteCredential(ctx, provider, kind); err != nil && !isFallbackEligible(err) {
			firstErr = err
		}
	}
	if fallback, ok := s.fallback.(app.AuthCredentialStore); s.fallback != nil && ok {
		if err := fallback.DeleteCredential(ctx, provider, kind); err != nil && !isFallbackEligible(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *CompositeStore) Load(ctx context.Context, provider app.ProviderID) (app.Secret, error) {
	if s.primary != nil {
		secret, err := s.primary.Load(ctx, provider)
		if err == nil {
			return secret, nil
		}
		if !isFallbackEligible(err) {
			return app.Secret{}, err
		}
	}
	if s.fallback == nil {
		return app.Secret{}, ErrSecretNotFound
	}
	return s.fallback.Load(ctx, provider)
}

func (s *CompositeStore) Delete(ctx context.Context, provider app.ProviderID) error {
	var firstErr error
	if s.primary != nil {
		if err := s.primary.Delete(ctx, provider); err != nil && !isFallbackEligible(err) {
			firstErr = err
		}
	}
	if s.fallback != nil {
		if err := s.fallback.Delete(ctx, provider); err != nil && !isFallbackEligible(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func isFallbackEligible(err error) bool {
	return errors.Is(err, ErrSecretNotFound) || errors.Is(err, ErrStoreUnavailable)
}
