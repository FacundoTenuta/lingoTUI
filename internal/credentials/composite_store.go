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
