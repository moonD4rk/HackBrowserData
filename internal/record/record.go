package record

import "time"

// Record is one output row; the collector sets its Browser and Profile, so extractors leave both empty.
type Record interface {
	SetBrowserAndProfile(browser, profile string)
}

// Login is named for the whole row, not Password: it holds a URL and a user name as well as the password.
type Login struct {
	Browser   string    `json:"browser" csv:"browser"`
	Profile   string    `json:"profile" csv:"profile"`
	URL       string    `json:"url" csv:"url"`
	Username  string    `json:"username" csv:"username"`
	Password  string    `json:"password" csv:"password"`
	CreatedAt time.Time `json:"created_at" csv:"created_at"`
	Store     Store     `json:"store" csv:"store"`
}

// Store names the Chromium login database a Login came from; a credential kept in both is exported twice.
type Store string

const (
	StoreLocal   Store = "local"   // Login Data
	StoreAccount Store = "account" // Login Data For Account
)

type Cookie struct {
	Browser      string    `json:"browser" csv:"browser"`
	Profile      string    `json:"profile" csv:"profile"`
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
	SameSite     string    `json:"same_site" csv:"same_site"`
}

type Bookmark struct {
	Browser   string    `json:"browser" csv:"browser"`
	Profile   string    `json:"profile" csv:"profile"`
	ID        int64     `json:"id" csv:"id"`
	Name      string    `json:"name" csv:"name"`
	Type      string    `json:"type" csv:"type"`
	URL       string    `json:"url" csv:"url"`
	Folder    string    `json:"folder" csv:"folder"`
	CreatedAt time.Time `json:"created_at" csv:"created_at"`
}

type History struct {
	Browser    string    `json:"browser" csv:"browser"`
	Profile    string    `json:"profile" csv:"profile"`
	URL        string    `json:"url" csv:"url"`
	Title      string    `json:"title" csv:"title"`
	VisitCount int       `json:"visit_count" csv:"visit_count"`
	LastVisit  time.Time `json:"last_visit" csv:"last_visit"`
}

type Download struct {
	Browser    string    `json:"browser" csv:"browser"`
	Profile    string    `json:"profile" csv:"profile"`
	URL        string    `json:"url" csv:"url"`
	TargetPath string    `json:"target_path" csv:"target_path"`
	MimeType   string    `json:"mime_type" csv:"mime_type"`
	TotalBytes int64     `json:"total_bytes" csv:"total_bytes"`
	StartTime  time.Time `json:"start_time" csv:"start_time"`
	EndTime    time.Time `json:"end_time" csv:"end_time"`
}

// CreditCard has Yandex-only CVC and Comment fields; Chromium leaves them empty.
type CreditCard struct {
	Browser  string `json:"browser" csv:"browser"`
	Profile  string `json:"profile" csv:"profile"`
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

type Extension struct {
	Browser     string `json:"browser" csv:"browser"`
	Profile     string `json:"profile" csv:"profile"`
	Name        string `json:"name" csv:"name"`
	ID          string `json:"id" csv:"id"`
	Description string `json:"description" csv:"description"`
	Version     string `json:"version" csv:"version"`
	HomepageURL string `json:"homepage_url" csv:"homepage_url"`
	Enabled     bool   `json:"enabled" csv:"enabled"`
}

type LocalStorage struct {
	Browser string `json:"browser" csv:"browser"`
	Profile string `json:"profile" csv:"profile"`
	IsMeta  bool   `json:"is_meta" csv:"is_meta"`
	URL     string `json:"url" csv:"url"`
	Key     string `json:"key" csv:"key"`
	Value   string `json:"value" csv:"value"`
}

// SessionStorage repeats LocalStorage's fields so that each record type belongs to exactly one category.
type SessionStorage struct {
	Browser string `json:"browser" csv:"browser"`
	Profile string `json:"profile" csv:"profile"`
	IsMeta  bool   `json:"is_meta" csv:"is_meta"`
	URL     string `json:"url" csv:"url"`
	Key     string `json:"key" csv:"key"`
	Value   string `json:"value" csv:"value"`
}

func (r *Login) SetBrowserAndProfile(browser, profile string) {
	r.Browser, r.Profile = browser, profile
}

func (r *Cookie) SetBrowserAndProfile(browser, profile string) {
	r.Browser, r.Profile = browser, profile
}

func (r *Bookmark) SetBrowserAndProfile(browser, profile string) {
	r.Browser, r.Profile = browser, profile
}

func (r *History) SetBrowserAndProfile(browser, profile string) {
	r.Browser, r.Profile = browser, profile
}

func (r *Download) SetBrowserAndProfile(browser, profile string) {
	r.Browser, r.Profile = browser, profile
}

func (r *CreditCard) SetBrowserAndProfile(browser, profile string) {
	r.Browser, r.Profile = browser, profile
}

func (r *Extension) SetBrowserAndProfile(browser, profile string) {
	r.Browser, r.Profile = browser, profile
}

func (r *LocalStorage) SetBrowserAndProfile(browser, profile string) {
	r.Browser, r.Profile = browser, profile
}

func (r *SessionStorage) SetBrowserAndProfile(browser, profile string) {
	r.Browser, r.Profile = browser, profile
}
