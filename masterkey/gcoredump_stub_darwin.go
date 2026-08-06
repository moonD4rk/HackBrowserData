//go:build darwin && !keychain_gcore

package masterkey

import (
	"errors"

	"github.com/moond4rk/keychainbreaker"
)

// DecryptKeychainRecords returns an error in default builds so GcoredumpRetriever
// falls through silently to the next tier. The CVE-2025-24204 securityd-dump
// implementation (gcoredump_darwin.go) is only compiled with -tags keychain_gcore,
// keeping the default `go build` free of the exploit code and its byte signatures.
func DecryptKeychainRecords() ([]keychainbreaker.GenericPassword, error) {
	return nil, errors.New("keychain gcore dump not built in (rebuild with -tags keychain_gcore)")
}
