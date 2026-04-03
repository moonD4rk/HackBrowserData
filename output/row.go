package output

import "github.com/moond4rk/hackbrowserdata/types"

// Row types embed Entry structs and add browser/profile context.
// All unexported — only used internally by Output.

type passwordRow struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	types.LoginEntry
}

func (r passwordRow) csvHeader() []string {
	return append([]string{"browser", "profile"}, r.CSVHeader()...)
}

func (r passwordRow) csvRow() []string {
	return append([]string{r.Browser, r.Profile}, r.CSVRow()...)
}

type cookieRow struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	types.CookieEntry
}

func (r cookieRow) csvHeader() []string {
	return append([]string{"browser", "profile"}, r.CSVHeader()...)
}

func (r cookieRow) csvRow() []string {
	return append([]string{r.Browser, r.Profile}, r.CSVRow()...)
}

type historyRow struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	types.HistoryEntry
}

func (r historyRow) csvHeader() []string {
	return append([]string{"browser", "profile"}, r.CSVHeader()...)
}

func (r historyRow) csvRow() []string {
	return append([]string{r.Browser, r.Profile}, r.CSVRow()...)
}

type downloadRow struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	types.DownloadEntry
}

func (r downloadRow) csvHeader() []string {
	return append([]string{"browser", "profile"}, r.CSVHeader()...)
}

func (r downloadRow) csvRow() []string {
	return append([]string{r.Browser, r.Profile}, r.CSVRow()...)
}

type bookmarkRow struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	types.BookmarkEntry
}

func (r bookmarkRow) csvHeader() []string {
	return append([]string{"browser", "profile"}, r.CSVHeader()...)
}

func (r bookmarkRow) csvRow() []string {
	return append([]string{r.Browser, r.Profile}, r.CSVRow()...)
}

type creditCardRow struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	types.CreditCardEntry
}

func (r creditCardRow) csvHeader() []string {
	return append([]string{"browser", "profile"}, r.CSVHeader()...)
}

func (r creditCardRow) csvRow() []string {
	return append([]string{r.Browser, r.Profile}, r.CSVRow()...)
}

type extensionRow struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	types.ExtensionEntry
}

func (r extensionRow) csvHeader() []string {
	return append([]string{"browser", "profile"}, r.CSVHeader()...)
}

func (r extensionRow) csvRow() []string {
	return append([]string{r.Browser, r.Profile}, r.CSVRow()...)
}

type storageRow struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	types.StorageEntry
}

func (r storageRow) csvHeader() []string {
	return append([]string{"browser", "profile"}, r.CSVHeader()...)
}

func (r storageRow) csvRow() []string {
	return append([]string{r.Browser, r.Profile}, r.CSVRow()...)
}

// csvRecord is the internal interface for CSV serialization.
type csvRecord interface {
	csvHeader() []string
	csvRow() []string
}
