//go:build !windows

package services

func protectIdentitySecret(contents []byte) ([]byte, error) {
	return append([]byte(nil), contents...), nil
}
func unprotectIdentitySecret(contents []byte) ([]byte, error) {
	return append([]byte(nil), contents...), nil
}
