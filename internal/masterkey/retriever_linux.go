//go:build linux

package masterkey

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// linuxParams follow https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc
var linuxParams = pbkdf2Params{
	salt:       []byte("saltysalt"),
	iterations: 1,
	keySize:    16,
	hashFunc:   sha1.New,
}

// portalParams derive the v12 key from the XDG portal app secret, matching Chromium's SecretPortalKeyProvider.
var portalParams = hkdfParams{
	salt:     []byte("fdo_portal_secret_salt"),
	info:     []byte("HKDF-SHA-256 AES-256-GCM"),
	keySize:  32,
	hashFunc: sha256.New,
}

type hkdfParams struct {
	salt     []byte
	info     []byte
	keySize  int
	hashFunc func() hash.Hash
}

func (p hkdfParams) deriveKey(secret []byte) []byte {
	return crypto.HKDFKey(secret, p.salt, p.info, p.keySize, p.hashFunc)
}

// platformRetrievers run in order: v10 headless, v11 keyring, v12 XDG portal; a profile can hold values of each.
var platformRetrievers = []keyRetriever{posixRetriever{}, dbusRetriever{}, portalRetriever{}}

type dbusRetriever struct{}

func (dbusRetriever) Version() Version { return V11 }

const (
	secretServiceName      = "org.freedesktop.secrets"
	secretServicePath      = "/org/freedesktop/secrets"
	secretServiceInterface = "org.freedesktop.Secret.Service"
	secretItemInterface    = "org.freedesktop.Secret.Item"
	secretSessionInterface = "org.freedesktop.Secret.Session"
	propertiesGet          = "org.freedesktop.DBus.Properties.Get"
	secretCallTimeout      = 10 * time.Second
)

var errKeyringLocked = errors.New("keyring or wallet is locked")

func (dbusRetriever) Retrieve(ctx context.Context, locator Locator) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, secretCallTimeout)
	defer cancel()

	service, err := newSecretService(ctx)
	if err != nil {
		return nil, err
	}
	defer service.Close()

	secret, err := findStorageSecret(ctx, service, locator.SecretLabel)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", locator.SecretLabel, err)
	}
	return linuxParams.deriveKey(secret), nil
}

// secretStore is what the lookups need from the Secret Service, so tests can stand in for D-Bus. secrets reads a
// batch in one call, the only way to read gnome-keyring's portal items, whose D-Bus objects it never exports.
type secretStore interface {
	searchItems(ctx context.Context, attributes map[string]string) (unlocked, locked []dbus.ObjectPath, err error)
	unlock(ctx context.Context, items []dbus.ObjectPath) ([]dbus.ObjectPath, error)
	itemLabel(ctx context.Context, item dbus.ObjectPath) (string, error)
	itemSecret(ctx context.Context, item dbus.ObjectPath) ([]byte, error)
	secrets(ctx context.Context, items []dbus.ObjectPath) (map[dbus.ObjectPath][]byte, error)
}

// searchQueries find the storage item by libsecret's schema (GNOME), KWallet's schema, and KWallet's entry-name
// attribute, the last independent of the schema label, which KWallet versions set inconsistently.
func searchQueries(storage string) []map[string]string {
	return []map[string]string{
		{"xdg:schema": "chrome_libsecret_os_crypt_password_v2"},
		{"xdg:schema": "org.qt.keychain"},
		{"user": storage},
	}
}

// findStorageSecret tries each query; a match behind a wallet that stays locked reads as locked, not as not found.
func findStorageSecret(ctx context.Context, store secretStore, storage string) ([]byte, error) {
	locked := false
	for _, attributes := range searchQueries(storage) {
		secret, err := searchStorageSecret(ctx, store, attributes, storage)
		switch {
		case err == nil:
			return secret, nil
		case errors.Is(err, errKeyringLocked):
			locked = true
		case !errors.Is(err, errStorageNotFound):
			log.Debugf("secret service %v: %v", attributes, err)
		}
	}
	switch {
	case ctx.Err() != nil:
		return nil, fmt.Errorf("secret service lookup: %w", ctx.Err())
	case locked:
		return nil, errKeyringLocked
	}
	return nil, errStorageNotFound
}

// searchStorageSecret relies on SearchItems, which resolves only live items, and reads each match on its own, so a
// stale path that fails is skipped (a whole-collection read aborts on it instead).
func searchStorageSecret(ctx context.Context, store secretStore, attributes map[string]string, storage string) ([]byte, error) {
	items, allOpened, err := searchAndUnlock(ctx, store, attributes)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		label, err := store.itemLabel(ctx, item)
		if err != nil {
			log.Debugf("secret item %s: %v", item, err)
			continue
		}
		if !labelMatches(label, storage) {
			continue
		}
		secret, err := store.itemSecret(ctx, item)
		if err != nil {
			log.Debugf("secret item %s: %v", item, err)
			continue
		}
		if len(secret) > 0 {
			return secret, nil
		}
	}
	if !allOpened {
		return nil, errKeyringLocked
	}
	return nil, errStorageNotFound
}

// searchAndUnlock resolves a query to live item paths, unlocking any that open without a prompt; allOpened is false
// when a locked item stayed locked, which the callers report as errKeyringLocked rather than as not found.
func searchAndUnlock(ctx context.Context, store secretStore, attributes map[string]string) (items []dbus.ObjectPath, allOpened bool, err error) {
	unlocked, locked, err := store.searchItems(ctx, attributes)
	if err != nil {
		return nil, false, err
	}
	items = unlocked
	allOpened = true
	if len(locked) > 0 {
		opened, err := store.unlock(ctx, locked)
		if err != nil {
			log.Debugf("secret service: %v", err)
		}
		items = append(items, opened...)
		allOpened = len(opened) >= len(locked)
	}
	return items, allOpened, nil
}

// findPortalSecret looks up the XDG portal app secret for each candidate app id; a secret behind a wallet that stays
// locked reads as locked, and no match at all steps aside so a profile without v12 gets no error row.
func findPortalSecret(ctx context.Context, store secretStore, appIDs []string) ([]byte, error) {
	locked := false
	for _, appID := range appIDs {
		for _, attributes := range portalQueries(appID) {
			secret, err := searchPortalSecret(ctx, store, attributes)
			switch {
			case err == nil:
				return secret, nil
			case errors.Is(err, errKeyringLocked):
				locked = true
			case !errors.Is(err, errStorageNotFound):
				log.Debugf("portal secret %v: %v", attributes, err)
			}
		}
	}
	switch {
	case ctx.Err() != nil:
		return nil, fmt.Errorf("portal secret lookup: %w", ctx.Err())
	case locked:
		return nil, errKeyringLocked
	}
	return nil, errStorageNotFound
}

// portalQueries find the app secret by gnome-keyring's portal schema and app_id, then by ksecretd's synthesized
// folder and entry, which carry no schema.
func portalQueries(appID string) []map[string]string {
	return []map[string]string{
		{"xdg:schema": "org.freedesktop.portal.Secret", "app_id": appID},
		{"server": "xdg-desktop-portal", "user": appID},
	}
}

// searchPortalSecret reads matches with GetSecrets first, since gnome-keyring's portal items answer nothing else,
// and falls back to a per-item read for stores that leave GetSecrets empty.
func searchPortalSecret(ctx context.Context, store secretStore, attributes map[string]string) ([]byte, error) {
	items, allOpened, err := searchAndUnlock(ctx, store, attributes)
	if err != nil {
		return nil, err
	}
	secrets, err := store.secrets(ctx, items)
	if err != nil {
		log.Debugf("portal secret batch: %v", err)
		secrets = nil
	}
	for _, item := range items {
		secret := secrets[item]
		if len(secret) == 0 {
			secret, err = store.itemSecret(ctx, item)
			if err != nil {
				log.Debugf("portal secret %s: %v", item, err)
				continue
			}
		}
		if len(secret) > 0 {
			return secret, nil
		}
	}
	if !allOpened {
		return nil, errKeyringLocked
	}
	return nil, errStorageNotFound
}

// labelMatches accepts the exact Secret Service label and KWallet's "<folder>/<storage>" form.
func labelMatches(label, storage string) bool {
	return label == storage || strings.HasSuffix(label, "/"+storage)
}

type secretService struct {
	connection *dbus.Conn
	object     dbus.BusObject
	session    dbus.ObjectPath
}

func newSecretService(ctx context.Context) (*secretService, error) {
	connection, err := connectSessionBus(ctx)
	if err != nil {
		return nil, fmt.Errorf("dbus session: %w", err)
	}
	service := &secretService{connection: connection, object: connection.Object(secretServiceName, secretServicePath)}
	call := service.object.CallWithContext(ctx, secretServiceInterface+".OpenSession", 0, "plain", dbus.MakeVariant(""))
	var output dbus.Variant
	if err := call.Store(&output, &service.session); err != nil {
		return nil, errors.Join(fmt.Errorf("open secret session: %w", err), connection.Close())
	}
	return service, nil
}

// connectSessionBus never launches a bus, unlike dbus.SessionBus, and ties the connection to ctx, so the deadline
// bounds connecting and every later call, Close included.
func connectSessionBus(ctx context.Context) (*dbus.Conn, error) {
	connection, err := dbus.SessionBusPrivateNoAutoStartup(dbus.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if err := connection.Auth(nil); err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	if err := connection.Hello(); err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	return connection, nil
}

func (s *secretService) Close() error {
	call := s.connection.Object(secretServiceName, s.session).Call(secretSessionInterface+".Close", 0)
	return errors.Join(call.Err, s.connection.Close())
}

func (s *secretService) searchItems(ctx context.Context, attributes map[string]string) (unlocked, locked []dbus.ObjectPath, err error) {
	call := s.object.CallWithContext(ctx, secretServiceInterface+".SearchItems", 0, attributes)
	if err = call.Store(&unlocked, &locked); err != nil {
		return nil, nil, fmt.Errorf("search items: %w", err)
	}
	return unlocked, locked, nil
}

// unlock keeps only what opens without a prompt; a returned prompt path (not "/") means a GUI dialog is needed,
// which a non-interactive extraction skips rather than block on.
func (s *secretService) unlock(ctx context.Context, items []dbus.ObjectPath) ([]dbus.ObjectPath, error) {
	call := s.object.CallWithContext(ctx, secretServiceInterface+".Unlock", 0, items)
	var unlocked []dbus.ObjectPath
	var prompt dbus.ObjectPath
	if err := call.Store(&unlocked, &prompt); err != nil {
		return nil, fmt.Errorf("unlock items: %w", err)
	}
	return unlocked, nil
}

func (s *secretService) itemLabel(ctx context.Context, item dbus.ObjectPath) (string, error) {
	call := s.connection.Object(secretServiceName, item).CallWithContext(ctx, propertiesGet, 0, secretItemInterface, "Label")
	var label string
	if err := call.Store(&label); err != nil {
		return "", fmt.Errorf("read label: %w", err)
	}
	return label, nil
}

func (s *secretService) itemSecret(ctx context.Context, item dbus.ObjectPath) ([]byte, error) {
	call := s.connection.Object(secretServiceName, item).CallWithContext(ctx, secretItemInterface+".GetSecret", 0, s.session)
	var reply dbusSecret
	if err := call.Store(&reply); err != nil {
		return nil, fmt.Errorf("get secret: %w", err)
	}
	return reply.Value, nil
}

// secrets reads several items in one call; gnome-keyring answers here for portal items it never exports as objects.
func (s *secretService) secrets(ctx context.Context, items []dbus.ObjectPath) (map[dbus.ObjectPath][]byte, error) {
	if len(items) == 0 {
		return nil, nil
	}
	call := s.object.CallWithContext(ctx, secretServiceInterface+".GetSecrets", 0, items, s.session)
	var replies map[dbus.ObjectPath]dbusSecret
	if err := call.Store(&replies); err != nil {
		return nil, fmt.Errorf("get secrets: %w", err)
	}
	values := make(map[dbus.ObjectPath][]byte, len(replies))
	for path, reply := range replies {
		values[path] = reply.Value
	}
	return values, nil
}

// dbusSecret is the org.freedesktop.Secret.Item.GetSecret reply; a plain session leaves Value unencrypted.
type dbusSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

// portalRetriever reads the XDG desktop portal app secret (Chromium's SecretPortalKeyProvider, the v12 scheme). It
// reads only a secret the portal already stored; it never calls RetrieveSecret, which would create one when missing.
type portalRetriever struct{}

func (portalRetriever) Version() Version { return V12 }

func (portalRetriever) Retrieve(ctx context.Context, locator Locator) ([]byte, error) {
	if len(locator.PortalAppIDs) == 0 {
		return nil, errNotApplicable
	}
	ctx, cancel := context.WithTimeout(ctx, secretCallTimeout)
	defer cancel()

	service, err := newSecretService(ctx)
	if err != nil {
		log.Debugf("portal secret: %v", err)
		return nil, errNotApplicable
	}
	defer service.Close()

	secret, err := findPortalSecret(ctx, service, locator.PortalAppIDs)
	switch {
	case errors.Is(err, errStorageNotFound):
		return nil, errNotApplicable
	case err != nil:
		return nil, err
	}
	return portalParams.deriveKey(secret), nil
}

// posixRetriever covers hosts without a keyring (headless, Docker, CI), mirroring Chromium's PosixKeyProvider.
type posixRetriever struct{}

func (posixRetriever) Version() Version { return V10 }

func (posixRetriever) Retrieve(context.Context, Locator) ([]byte, error) {
	return linuxParams.deriveKey([]byte("peanuts")), nil
}
