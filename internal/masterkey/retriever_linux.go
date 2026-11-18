//go:build linux

package masterkey

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// linuxParams follow https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc
var linuxParams = pbkdf2Params{
	salt:       []byte("saltysalt"),
	iterations: 1,
	keySize:    16,
	hashFunc:   sha1.New,
}

// platformRetrievers run both: a profile holds v10 and v11 values once its host moved between headless and keyring.
var platformRetrievers = []keyRetriever{posixRetriever{}, dbusRetriever{}}

type dbusRetriever struct{}

func (dbusRetriever) Version() Version { return V11 }

const (
	secretServiceName = "org.freedesktop.secrets"
	secretServicePath = "/org/freedesktop/secrets"
	secretServiceIntf = "org.freedesktop.Secret.Service"
	secretItemIntf    = "org.freedesktop.Secret.Item"
	secretSessionIntf = "org.freedesktop.Secret.Session"
	propertiesGet     = "org.freedesktop.DBus.Properties.Get"
	secretCallTimeout = 10 * time.Second
)

var errKeyringLocked = errors.New("keyring or wallet is locked") //nolint:unused // only used on linux

// dbusSecret is the org.freedesktop.Secret.Item.GetSecret reply; a plain session leaves Value unencrypted.
type dbusSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

// searchQueries find the storage item by libsecret's schema (GNOME), KWallet's schema, and KWallet's entry-name
// attribute, the last independent of the schema label, which KWallet versions set inconsistently.
func searchQueries(label string) []map[string]string {
	return []map[string]string{
		{"xdg:schema": "chrome_libsecret_os_crypt_password_v2"},
		{"xdg:schema": "org.qt.keychain"},
		{"user": label},
	}
}

// Retrieve looks the storage key up through the Secret Service's SearchItems, which resolves only live items, then
// reads each one by one so a stale path that errors is skipped (a whole-collection read aborts on it instead). A
// match hidden behind a locked wallet is reported as locked, not as not found.
func (dbusRetriever) Retrieve(ctx context.Context, locator Locator) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, secretCallTimeout)
	defer cancel()

	connection, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("dbus session: %w", err)
	}
	service := connection.Object(secretServiceName, secretServicePath)

	session, err := openSecretSession(ctx, service)
	if err != nil {
		return nil, err
	}
	defer closeSecretSession(connection, session)

	sawLocked := false
	for _, attrs := range searchQueries(locator.SecretLabel) {
		secret, locked := findStorageSecret(ctx, connection, service, session, attrs, locator.SecretLabel)
		if len(secret) > 0 {
			return linuxParams.deriveKey(secret), nil
		}
		sawLocked = sawLocked || locked
	}
	if sawLocked {
		return nil, fmt.Errorf("%q: %w", locator.SecretLabel, errKeyringLocked)
	}
	return nil, fmt.Errorf("%q: %w", locator.SecretLabel, errStorageNotFound)
}

func openSecretSession(ctx context.Context, service dbus.BusObject) (dbus.ObjectPath, error) {
	call := service.CallWithContext(ctx, secretServiceIntf+".OpenSession", 0, "plain", dbus.MakeVariant(""))
	if call.Err != nil {
		return "", fmt.Errorf("open secret session: %w", call.Err)
	}
	var output dbus.Variant
	var session dbus.ObjectPath
	if err := call.Store(&output, &session); err != nil {
		return "", fmt.Errorf("open secret session: %w", err)
	}
	return session, nil
}

func closeSecretSession(connection *dbus.Conn, session dbus.ObjectPath) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if call := connection.Object(secretServiceName, session).CallWithContext(ctx, secretSessionIntf+".Close", 0); call.Err != nil {
		log.Debugf("close secret session: %v", call.Err)
	}
}

// findStorageSecret returns the first live item matching attrs whose label fits want, and whether a locked item hid a
// possible match (so the caller can report "locked" rather than "not found").
func findStorageSecret(
	ctx context.Context, conn *dbus.Conn, service dbus.BusObject, session dbus.ObjectPath, attrs map[string]string, want string,
) ([]byte, bool) {
	call := service.CallWithContext(ctx, secretServiceIntf+".SearchItems", 0, attrs)
	if call.Err != nil {
		log.Debugf("secret service SearchItems %v: %v", attrs, call.Err)
		return nil, false
	}
	var unlocked, locked []dbus.ObjectPath
	if err := call.Store(&unlocked, &locked); err != nil {
		return nil, false
	}
	sawLocked := false
	if len(locked) > 0 {
		opened := unlockItems(ctx, service, locked)
		unlocked = append(unlocked, opened...)
		sawLocked = len(opened) < len(locked)
	}
	for _, path := range unlocked {
		if !labelMatches(itemLabel(ctx, conn, path), want) {
			continue
		}
		if secret := itemSecret(ctx, conn, path, session); len(secret) > 0 {
			return secret, sawLocked
		}
	}
	return nil, sawLocked
}

// unlockItems unlocks only what opens without a prompt; a returned prompt path (not "/") means a GUI dialog is needed,
// which a non-interactive extraction skips rather than block on.
func unlockItems(ctx context.Context, service dbus.BusObject, locked []dbus.ObjectPath) []dbus.ObjectPath {
	call := service.CallWithContext(ctx, secretServiceIntf+".Unlock", 0, locked)
	if call.Err != nil {
		return nil
	}
	var unlocked []dbus.ObjectPath
	var prompt dbus.ObjectPath
	if err := call.Store(&unlocked, &prompt); err != nil {
		return nil
	}
	return unlocked
}

// labelMatches accepts the exact Secret Service label and KWallet's "<folder>/<entry>" form, where the entry is want.
func labelMatches(label, want string) bool {
	return label == want || strings.HasSuffix(label, "/"+want)
}

func itemLabel(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) string {
	call := conn.Object(secretServiceName, path).CallWithContext(ctx, propertiesGet, 0, secretItemIntf, "Label")
	if call.Err != nil {
		return ""
	}
	var value dbus.Variant
	if err := call.Store(&value); err != nil {
		return ""
	}
	label, _ := value.Value().(string)
	return label
}

func itemSecret(ctx context.Context, conn *dbus.Conn, path, session dbus.ObjectPath) []byte {
	call := conn.Object(secretServiceName, path).CallWithContext(ctx, secretItemIntf+".GetSecret", 0, session)
	if call.Err != nil {
		return nil
	}
	var secret dbusSecret
	if err := call.Store(&secret); err != nil {
		return nil
	}
	return secret.Value
}

// posixRetriever covers hosts without a keyring (headless, Docker, CI), mirroring Chromium's PosixKeyProvider.
type posixRetriever struct{}

func (posixRetriever) Version() Version { return V10 }

func (posixRetriever) Retrieve(context.Context, Locator) ([]byte, error) {
	return linuxParams.deriveKey([]byte("peanuts")), nil
}
