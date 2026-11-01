//go:build darwin && !keychain_gcore

package masterkey

import (
	"errors"

	"github.com/moond4rk/keychainbreaker"
)

// Default builds leave out the securityd dump (built only with -tags keychain_gcore), so this tier steps aside.
func DecryptKeychainRecords() ([]keychainbreaker.GenericPassword, error) {
	return nil, errors.New("keychain gcore dump not built in (rebuild with -tags keychain_gcore)")
}
