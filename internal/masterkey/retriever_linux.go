//go:build linux

package masterkey

import (
	"context"
	"crypto/sha1"
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
	secretCallTimeout = 10 * time.Second
)

// osCryptSchemas are the Secret Service xdg:schema values Chromium forks use: libsecret's own on GNOME, and the one
// KWallet tags its entries with when it answers the Secret Service on KDE.
var osCryptSchemas = []string{"chrome_libsecret_os_crypt_password_v2", "org.qt.keychain"}

// dbusSecret is the org.freedesktop.Secret.Item.GetSecret reply; a plain session leaves Value unencrypted.
type dbusSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

// Retrieve reads the storage key through the Secret Service's SearchItems, which resolves only live items. The login
// collection can list item paths that no longer exist, and a whole-collection enumeration aborts on the first of them,
// so it misses the live key even while the collection is unlocked. KWallet exposes its entries here labeled
// "<X> Keys/<X> Safe Storage", so the wanted label is matched by suffix as well.
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

	for _, schema := range osCryptSchemas {
		if key := findStorageSecret(ctx, connection, service, session, schema, locator.SecretLabel); key != nil {
			return linuxParams.deriveKey(key), nil
		}
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
	if call := connection.Object(secretServiceName, session).Call(secretSessionIntf+".Close", 0); call.Err != nil {
		log.Debugf("close secret session: %v", call.Err)
	}
}

// findStorageSecret returns the first live item of the schema whose label equals, or ends with "/"+want.
func findStorageSecret(ctx context.Context, conn *dbus.Conn, service dbus.BusObject, session dbus.ObjectPath, schema, want string) []byte {
	call := service.CallWithContext(ctx, secretServiceIntf+".SearchItems", 0, map[string]string{"xdg:schema": schema})
	if call.Err != nil {
		log.Debugf("secret service SearchItems (%s): %v", schema, call.Err)
		return nil
	}
	var unlocked, locked []dbus.ObjectPath
	if err := call.Store(&unlocked, &locked); err != nil {
		return nil
	}
	if len(locked) > 0 {
		unlocked = append(unlocked, unlockItems(ctx, service, locked)...)
	}
	for _, path := range unlocked {
		if !labelMatches(conn, path, want) {
			continue
		}
		if secret := itemSecret(ctx, conn, path, session); len(secret) > 0 {
			return secret
		}
	}
	return nil
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
	if prompt != "/" {
		log.Debugf("secret service: %d item(s) stay locked behind a prompt", len(locked)-len(unlocked))
	}
	return unlocked
}

func labelMatches(conn *dbus.Conn, path dbus.ObjectPath, want string) bool {
	value, err := conn.Object(secretServiceName, path).GetProperty(secretItemIntf + ".Label")
	if err != nil {
		return false
	}
	label, _ := value.Value().(string)
	return label == want || strings.HasSuffix(label, "/"+want)
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
