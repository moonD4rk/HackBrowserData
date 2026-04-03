package types

import (
	"fmt"
	"time"
)

// CSVRecord is implemented by all Entry types to support CSV output
// without external dependencies (replaces gocsv).
type CSVRecord interface {
	CSVHeader() []string
	CSVRow() []string
}

// LoginEntry represents a single saved login credential.
type LoginEntry struct {
	URL       string    `json:"url"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"created_at"`
}

func (e LoginEntry) CSVHeader() []string {
	return []string{"url", "username", "password", "created_at"}
}

func (e LoginEntry) CSVRow() []string {
	return []string{e.URL, e.Username, e.Password, formatTime(e.CreatedAt)}
}

// CookieEntry represents a single browser cookie.
type CookieEntry struct {
	Host         string    `json:"host"`
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	Value        string    `json:"value"`
	IsSecure     bool      `json:"is_secure"`
	IsHTTPOnly   bool      `json:"is_http_only"`
	HasExpire    bool      `json:"has_expire"`
	IsPersistent bool      `json:"is_persistent"`
	ExpireAt     time.Time `json:"expire_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func (e CookieEntry) CSVHeader() []string {
	return []string{"host", "path", "name", "value", "is_secure", "is_http_only", "has_expire", "is_persistent", "expire_at", "created_at"}
}

func (e CookieEntry) CSVRow() []string {
	return []string{
		e.Host, e.Path, e.Name, e.Value,
		formatBool(e.IsSecure), formatBool(e.IsHTTPOnly),
		formatBool(e.HasExpire), formatBool(e.IsPersistent),
		formatTime(e.ExpireAt), formatTime(e.CreatedAt),
	}
}

// BookmarkEntry represents a single browser bookmark.
type BookmarkEntry struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Folder    string    `json:"folder"`
	CreatedAt time.Time `json:"created_at"`
}

func (e BookmarkEntry) CSVHeader() []string {
	return []string{"name", "url", "folder", "created_at"}
}

func (e BookmarkEntry) CSVRow() []string {
	return []string{e.Name, e.URL, e.Folder, formatTime(e.CreatedAt)}
}

// HistoryEntry represents a single browser history record.
type HistoryEntry struct {
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	VisitCount int       `json:"visit_count"`
	LastVisit  time.Time `json:"last_visit"`
}

func (e HistoryEntry) CSVHeader() []string {
	return []string{"url", "title", "visit_count", "last_visit"}
}

func (e HistoryEntry) CSVRow() []string {
	return []string{e.URL, e.Title, fmt.Sprintf("%d", e.VisitCount), formatTime(e.LastVisit)}
}

// DownloadEntry represents a single browser download record.
type DownloadEntry struct {
	URL        string    `json:"url"`
	TargetPath string    `json:"target_path"`
	MimeType   string    `json:"mime_type"`
	TotalBytes int64     `json:"total_bytes"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
}

func (e DownloadEntry) CSVHeader() []string {
	return []string{"url", "target_path", "mime_type", "total_bytes", "start_time", "end_time"}
}

func (e DownloadEntry) CSVRow() []string {
	return []string{e.URL, e.TargetPath, e.MimeType, fmt.Sprintf("%d", e.TotalBytes), formatTime(e.StartTime), formatTime(e.EndTime)}
}

// CreditCardEntry represents a single saved credit card.
type CreditCardEntry struct {
	Name     string `json:"name"`
	Number   string `json:"number"`
	ExpMonth string `json:"exp_month"`
	ExpYear  string `json:"exp_year"`
	NickName string `json:"nick_name"`
	Address  string `json:"address"`
}

func (e CreditCardEntry) CSVHeader() []string {
	return []string{"name", "number", "exp_month", "exp_year", "nick_name", "address"}
}

func (e CreditCardEntry) CSVRow() []string {
	return []string{e.Name, e.Number, e.ExpMonth, e.ExpYear, e.NickName, e.Address}
}

// StorageEntry represents a single key-value pair from local or session storage.
type StorageEntry struct {
	URL   string `json:"url"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (e StorageEntry) CSVHeader() []string {
	return []string{"url", "key", "value"}
}

func (e StorageEntry) CSVRow() []string {
	return []string{e.URL, e.Key, e.Value}
}

// ExtensionEntry represents a single browser extension.
type ExtensionEntry struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description"`
	Version     string `json:"version"`
	HomepageURL string `json:"homepage_url"`
	Enabled     bool   `json:"enabled"`
}

func (e ExtensionEntry) CSVHeader() []string {
	return []string{"name", "id", "description", "version", "homepage_url", "enabled"}
}

func (e ExtensionEntry) CSVRow() []string {
	return []string{e.Name, e.ID, e.Description, e.Version, e.HomepageURL, formatBool(e.Enabled)}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
