package types

import "time"

// LoginEntry represents a single saved login credential.
type LoginEntry struct {
	URL       string    `json:"url" csv:"url"`
	Username  string    `json:"username" csv:"username"`
	Password  string    `json:"password" csv:"password"`
	CreatedAt time.Time `json:"created_at" csv:"created_at"`
}

// CookieEntry represents a single browser cookie.
type CookieEntry struct {
	Host         string    `json:"host" csv:"host"`
	Path         string    `json:"path" csv:"path"`
	Name         string    `json:"name" csv:"name"`
	Value        string    `json:"value" csv:"value"`
	IsSecure     bool      `json:"is_secure" csv:"is_secure"`
	IsHTTPOnly   bool      `json:"is_http_only" csv:"is_http_only"`
	HasExpire    bool      `json:"has_expire" csv:"has_expire"`
	IsPersistent bool      `json:"is_persistent" csv:"is_persistent"`
	ExpireAt     time.Time `json:"expire_at" csv:"expire_at"`
	CreatedAt    time.Time `json:"created_at" csv:"created_at"`
}

// BookmarkEntry represents a single browser bookmark.
type BookmarkEntry struct {
	ID        int64     `json:"id" csv:"id"`
	Name      string    `json:"name" csv:"name"`
	Type      string    `json:"type" csv:"type"`
	URL       string    `json:"url" csv:"url"`
	Folder    string    `json:"folder" csv:"folder"`
	CreatedAt time.Time `json:"created_at" csv:"created_at"`
}

// HistoryEntry represents a single browser history record.
type HistoryEntry struct {
	URL        string    `json:"url" csv:"url"`
	Title      string    `json:"title" csv:"title"`
	VisitCount int       `json:"visit_count" csv:"visit_count"`
	LastVisit  time.Time `json:"last_visit" csv:"last_visit"`
}

// DownloadEntry represents a single browser download record.
type DownloadEntry struct {
	URL        string    `json:"url" csv:"url"`
	TargetPath string    `json:"target_path" csv:"target_path"`
	MimeType   string    `json:"mime_type" csv:"mime_type"`
	TotalBytes int64     `json:"total_bytes" csv:"total_bytes"`
	StartTime  time.Time `json:"start_time" csv:"start_time"`
	EndTime    time.Time `json:"end_time" csv:"end_time"`
}

// CreditCardEntry represents a single saved credit card. CVC and Comment are
// Yandex-specific; Chromium leaves them empty.
type CreditCardEntry struct {
	GUID     string `json:"guid" csv:"guid"`
	Name     string `json:"name" csv:"name"`
	Number   string `json:"number" csv:"number"`
	ExpMonth string `json:"exp_month" csv:"exp_month"`
	ExpYear  string `json:"exp_year" csv:"exp_year"`
	NickName string `json:"nick_name" csv:"nick_name"`
	Address  string `json:"address" csv:"address"`
	CVC      string `json:"cvc" csv:"cvc"`
	Comment  string `json:"comment" csv:"comment"`
}

// StorageEntry represents a single key-value pair from local or session storage.
type StorageEntry struct {
	IsMeta bool   `json:"is_meta" csv:"is_meta"`
	URL    string `json:"url" csv:"url"`
	Key    string `json:"key" csv:"key"`
	Value  string `json:"value" csv:"value"`
}

// ExtensionEntry represents a single browser extension.
type ExtensionEntry struct {
	Name        string `json:"name" csv:"name"`
	ID          string `json:"id" csv:"id"`
	Description string `json:"description" csv:"description"`
	Version     string `json:"version" csv:"version"`
	HomepageURL string `json:"homepage_url" csv:"homepage_url"`
	Enabled     bool   `json:"enabled" csv:"enabled"`
}
