//go:build darwin && !keychain_gcore

package masterkey

import (
	"errors"

	"github.com/moond4rk/keychainbreaker"
)

// decryptKeychainRecords steps aside in default builds, which leave out the securityd dump (-tags keychain_gcore).
func decryptKeychainRecords() ([]keychainbreaker.GenericPassword, error) {
	return nil, errors.New("keychain gcore dump not built in (rebuild with -tags keychain_gcore)")
}
