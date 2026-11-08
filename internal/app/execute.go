package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/browser/safari"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// loginKeychain is the run's one login keychain, shared by Chromium's v10 key and Safari's passwords; nil off macOS.
type loginKeychain interface {
	masterkey.LoginKeychain
	safari.LoginKeychain
}

type dirKeysFunc func(ctx context.Context, dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) (masterkey.Set, error)

// Execute retrieves each dir's master keys from this machine. It handles one user data dir at a time, so at most one
// dir's copies are on disk. Its error means the run could not finish; every other failure is in the Summary.
func Execute(ctx context.Context, plan Plan, output Output, keychainConfig KeychainConfig) (Summary, error) {
	keychain := newLoginKeychain(keychainConfig)
	return execute(ctx, plan, output, extractorsWith(keychain), keysFromMachine(keychain))
}

// ExecuteWithKeys decrypts with a key file's keys, so it never reads this machine's secret stores.
func ExecuteWithKeys(ctx context.Context, plan Plan, output Output, keys map[browser.UserDataDirID]masterkey.Set) (Summary, error) {
	return execute(ctx, plan, output, extractorsWith(nil), keysFromFile(keys))
}

func execute(ctx context.Context, plan Plan, output Output, extractors extractorsFunc, keys dirKeysFunc) (Summary, error) {
	writer, err := export.NewWriter(output.Dir, output.Format, output.Zip)
	if err != nil {
		return Summary{}, err
	}
	summary, err := runner{writer: writer, extractors: extractors, keys: keys, logProgress: true}.run(ctx, plan)
	closeErr := writer.Close()
	summary.Exported = writer.Output()
	return summary, errors.Join(err, closeErr)
}

// CountRecords runs the plan with no key and the keychain locked, counting records instead of exporting them.
func CountRecords(ctx context.Context, plan Plan, keychainConfig KeychainConfig) (Summary, error) {
	keychain := newLoginKeychain(keychainConfig)
	return runner{writer: discard{}, extractors: extractorsWith(keychain), keys: keysFromFile(nil)}.run(ctx, plan)
}

type recordWriter interface {
	Write(dataCategory category.Category, records []record.Record) error
}

type discard struct{}

func (discard) Write(category.Category, []record.Record) error { return nil }

// runner is what every user data dir of one run shares; logProgress logs each dir as it starts.
type runner struct {
	writer      recordWriter
	extractors  extractorsFunc
	keys        dirKeysFunc
	logProgress bool
}

func (r runner) run(ctx context.Context, plan Plan) (Summary, error) {
	summary := Summary{
		Results:  append([]Result(nil), plan.Skipped...),
		Failures: append([]DirFailure(nil), plan.Failures...),
	}
	for _, dir := range plan.Dirs {
		if err := ctx.Err(); err != nil {
			return summary, fmt.Errorf("run stopped: %w", err)
		}
		dirSummary, err := r.runDir(ctx, dir, plan)
		summary.Results = append(summary.Results, dirSummary.Results...)
		summary.Failures = append(summary.Failures, dirSummary.Failures...)
		if err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (r runner) runDir(ctx context.Context, dir browser.UserDataDir, plan Plan) (Summary, error) {
	extractions := extractionsOf(plan, dir.ID)
	if len(extractions) == 0 {
		return Summary{}, nil
	}
	if r.logProgress {
		log.Infof("Extracting %s...", dir.Browser.DisplayName)
	}

	snapshots, err := snapshot.Temp()
	if err != nil {
		return Summary{Results: failAll(extractions, err)}, nil
	}
	defer closeSnapshot(snapshots)
	if err := snapshots.Capture(ctx, filesOf(plan, dir.ID)); err != nil {
		if ctx.Err() != nil {
			return Summary{}, err
		}
		return Summary{Results: failAll(extractions, err)}, nil
	}
	dirSnapshot := snapshots.Snapshot(string(dir.ID))

	var (
		dirKeyRetrieval *dirKeys
		masterKeys      func(context.Context) masterkey.Set
	)
	if dir.Browser.Base == browser.Chromium {
		dirKeyRetrieval = &dirKeys{retrieve: r.keys, dir: dir, dirSnapshot: dirSnapshot}
		masterKeys = dirKeyRetrieval.get
	}

	profiles := make(map[string]browser.Profile, len(dir.Profiles))
	byProfile := make(map[string]browser.Extractors, len(dir.Profiles))
	for _, profile := range dir.Profiles {
		profiles[profile.Name] = profile
		byProfile[profile.Name] = r.extractors(dir, profile)
	}

	results := make([]Result, 0, len(extractions))
	for _, extraction := range extractions {
		if err := ctx.Err(); err != nil {
			return Summary{Results: results}, fmt.Errorf("run stopped: %w", err)
		}
		extractor, ok := byProfile[extraction.ProfileName][extraction.Category]
		if !ok || extractor.Extract == nil {
			results = append(results, Result{Extraction: extraction, Outcome: OutcomeUnsupported})
			continue
		}
		profile := profiles[extraction.ProfileName]
		recordCollector := &collector{browserName: dir.Browser.DisplayName, profileName: profile.Name}
		err := extractor.Extract(ctx, browser.NewProfileSnapshot(dirSnapshot, profile, masterKeys), recordCollector)
		// Records emitted before an extraction failed are kept, as they always were.
		if writeErr := r.writer.Write(extraction.Category, recordCollector.records); writeErr != nil {
			return Summary{Results: results}, writeErr
		}
		results = append(results, recordCollector.result(extraction, err))
	}
	dirSummary := Summary{Results: results}
	if dirKeyRetrieval != nil && dirKeyRetrieval.err != nil {
		keyErr := fmt.Errorf("master key retrieval: %w", dirKeyRetrieval.err)
		dirSummary.Failures = []DirFailure{{UserDataDirID: dir.ID, Err: keyErr}}
	}
	return dirSummary, nil
}

func extractionsOf(plan Plan, id browser.UserDataDirID) []Extraction {
	var out []Extraction
	for _, extraction := range plan.Extractions {
		if extraction.UserDataDirID == id {
			out = append(out, extraction)
		}
	}
	return out
}

func filesOf(plan Plan, id browser.UserDataDirID) []snapshot.File {
	var out []snapshot.File
	for _, file := range plan.Files {
		if file.UserDataDirID == string(id) {
			out = append(out, file)
		}
	}
	return out
}

func failAll(extractions []Extraction, err error) []Result {
	results := make([]Result, len(extractions))
	for i, extraction := range extractions {
		results[i] = Result{Extraction: extraction, Outcome: OutcomeError, Err: err}
	}
	return results
}

func closeSnapshot(snapshots *snapshot.Dir) {
	if err := snapshots.Close(); err != nil {
		log.Warnf("%v", err)
	}
}

type collector struct {
	browserName, profileName string
	records                  []record.Record
	failures                 int
	firstFailure             error
}

func (c *collector) Add(row record.Record) {
	row.SetBrowserAndProfile(c.browserName, c.profileName)
	c.records = append(c.records, row)
}

func (c *collector) Fail(err error) {
	c.failures++
	if c.firstFailure == nil {
		c.firstFailure = err
	}
	log.Debugf("%s/%s: %v", c.browserName, c.profileName, err)
}

// result treats a file gone since planning as absent; failures alone are an error, and failures beside records partial.
func (c *collector) result(extraction Extraction, err error) Result {
	result := Result{Extraction: extraction, Records: len(c.records), Failures: c.failures}
	switch {
	case err != nil && errors.Is(err, fs.ErrNotExist) && result.Records == 0:
		result.Outcome = OutcomeAbsent
	case err != nil:
		result.Outcome, result.Err = OutcomeError, err
	case result.Failures > 0 && result.Records == 0:
		result.Outcome, result.Err = OutcomeError, c.firstFailure
	case result.Failures > 0:
		result.Outcome = OutcomePartial
	default:
		result.Outcome = OutcomeOK
	}
	return result
}

// dirKeys retrieves a dir's keys on first use, for all its profiles; Go 1.20 has no sync.OnceValues.
type dirKeys struct {
	once        sync.Once
	retrieve    dirKeysFunc
	dir         browser.UserDataDir
	dirSnapshot snapshot.Snapshot
	keys        masterkey.Set
	err         error
}

func (k *dirKeys) get(ctx context.Context) masterkey.Set {
	k.once.Do(func() { k.keys, k.err = k.retrieve(ctx, k.dir, k.dirSnapshot) })
	return k.keys
}

// keysFromMachine asks this machine's secret stores; on macOS that includes the run's login keychain.
func keysFromMachine(keychain masterkey.LoginKeychain) dirKeysFunc {
	return func(ctx context.Context, dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) (masterkey.Set, error) {
		locator, err := chromium.KeyLocator(dir, dirSnapshot)
		if err != nil {
			return nil, err
		}
		locator.Keychain = keychain
		return masterkey.RetrieveAll(ctx, locator)
	}
}

func keysFromFile(keys map[browser.UserDataDirID]masterkey.Set) dirKeysFunc {
	return func(_ context.Context, dir browser.UserDataDir, _ snapshot.Snapshot) (masterkey.Set, error) {
		return keys[dir.ID], nil
	}
}
