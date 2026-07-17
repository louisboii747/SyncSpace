//go:build windows

package services

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protectIdentitySecret(contents []byte) ([]byte, error) {
	if len(contents) == 0 {
		return nil, errors.New("identity secret is empty")
	}
	in := windows.DataBlob{Size: uint32(len(contents)), Data: &contents[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

func unprotectIdentitySecret(contents []byte) ([]byte, error) {
	if len(contents) == 0 {
		return nil, errors.New("protected identity secret is empty")
	}
	in := windows.DataBlob{Size: uint32(len(contents)), Data: &contents[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
