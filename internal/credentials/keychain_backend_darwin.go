//go:build darwin && cgo

package credentials

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <stdlib.h>
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

const (
	errSecItemNotFoundStatus = C.OSStatus(-25300)
	errSecAuthFailedStatus   = C.OSStatus(-25293)
)

type darwinKeychainBackend struct{}

func defaultKeychainBackend() KeychainBackend { return darwinKeychainBackend{} }

func (darwinKeychainBackend) Save(ctx context.Context, service, account, secret string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	serviceBytes := []byte(service)
	accountBytes := []byte(account)
	secretBytes := []byte(secret)

	var item C.SecKeychainItemRef
	status := C.SecKeychainFindGenericPassword(
		C.CFTypeRef(unsafe.Pointer(nil)),
		C.UInt32(len(serviceBytes)), charPtr(serviceBytes),
		C.UInt32(len(accountBytes)), charPtr(accountBytes),
		nil, nil,
		&item,
	)
	if status == errSecItemNotFoundStatus {
		status = C.SecKeychainAddGenericPassword(
			C.SecKeychainRef(unsafe.Pointer(nil)),
			C.UInt32(len(serviceBytes)), charPtr(serviceBytes),
			C.UInt32(len(accountBytes)), charPtr(accountBytes),
			C.UInt32(len(secretBytes)), dataPtr(secretBytes),
			nil,
		)
		return mapOSStatus(status)
	}
	if status != 0 {
		return mapOSStatus(status)
	}
	defer C.CFRelease(C.CFTypeRef(unsafe.Pointer(item)))

	status = C.SecKeychainItemModifyAttributesAndData(
		item,
		nil,
		C.UInt32(len(secretBytes)), dataPtr(secretBytes),
	)
	return mapOSStatus(status)
}

func (darwinKeychainBackend) Load(ctx context.Context, service, account string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	serviceBytes := []byte(service)
	accountBytes := []byte(account)

	var passwordLength C.UInt32
	var passwordData unsafe.Pointer
	var item C.SecKeychainItemRef
	status := C.SecKeychainFindGenericPassword(
		C.CFTypeRef(unsafe.Pointer(nil)),
		C.UInt32(len(serviceBytes)), charPtr(serviceBytes),
		C.UInt32(len(accountBytes)), charPtr(accountBytes),
		&passwordLength, &passwordData,
		&item,
	)
	if status != 0 {
		return "", mapOSStatus(status)
	}
	defer C.SecKeychainItemFreeContent(nil, passwordData)
	defer C.CFRelease(C.CFTypeRef(unsafe.Pointer(item)))

	return string(C.GoBytes(passwordData, C.int(passwordLength))), nil
}

func (darwinKeychainBackend) Delete(ctx context.Context, service, account string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	serviceBytes := []byte(service)
	accountBytes := []byte(account)

	var item C.SecKeychainItemRef
	status := C.SecKeychainFindGenericPassword(
		C.CFTypeRef(unsafe.Pointer(nil)),
		C.UInt32(len(serviceBytes)), charPtr(serviceBytes),
		C.UInt32(len(accountBytes)), charPtr(accountBytes),
		nil, nil,
		&item,
	)
	if status != 0 {
		return mapOSStatus(status)
	}
	defer C.CFRelease(C.CFTypeRef(unsafe.Pointer(item)))

	return mapOSStatus(C.SecKeychainItemDelete(item))
}

func charPtr(bytes []byte) *C.char {
	if len(bytes) == 0 {
		return nil
	}
	return (*C.char)(unsafe.Pointer(&bytes[0]))
}

func dataPtr(bytes []byte) unsafe.Pointer {
	if len(bytes) == 0 {
		return nil
	}
	return unsafe.Pointer(&bytes[0])
}

func mapOSStatus(status C.OSStatus) error {
	if status == 0 {
		return nil
	}
	if status == errSecItemNotFoundStatus {
		return ErrSecretNotFound
	}
	if status == errSecAuthFailedStatus {
		return ErrKeychainAccessDenied
	}
	return fmt.Errorf("keychain status %d", int(status))
}
