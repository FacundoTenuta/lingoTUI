//go:build !darwin || !cgo

package credentials

import "context"

type unavailableKeychainBackend struct{}

func defaultKeychainBackend() KeychainBackend { return unavailableKeychainBackend{} }

func (unavailableKeychainBackend) Save(context.Context, string, string, string) error {
	return ErrStoreUnavailable
}

func (unavailableKeychainBackend) Load(context.Context, string, string) (string, error) {
	return "", ErrStoreUnavailable
}

func (unavailableKeychainBackend) Delete(context.Context, string, string) error {
	return ErrStoreUnavailable
}
