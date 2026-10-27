//go:build linux

package browser

// newCredentialInjector is a no-op on Linux: RetrieveAll runs the peanuts (v10) and D-Bus keyring (v11)
// retrievers itself, and a Chromium browser builds the rest of its Locator from its own config.
func newCredentialInjector(_ DiscoverOptions) browserInjector {
	return func(Browser) {}
}
