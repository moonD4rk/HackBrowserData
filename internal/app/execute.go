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
		Results:     append([]Result(nil), plan.Skipped...),
		DirFailures: append([]DirFailure(nil), plan.DirFailures...),
	}
	for _, dirPlan := range plan.Dirs {
		if err := ctx.Err(); err != nil {
			return summary, fmt.Errorf("run stopped: %w", err)
		}
		dirSummary, err := r.runDir(ctx, dirPlan)
		summary.Results = append(summary.Results, dirSummary.Results...)
		summary.DirFailures = append(summary.DirFailures, dirSummary.DirFailures...)
		if err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (r runner) runDir(ctx context.Context, dirPlan DirPlan) (Summary, error) {
	dir := dirPlan.Dir
	if r.logProgress {
		log.Infof("Extracting %s...", dir.Browser.DisplayName)
	}

	snapshots, err := snapshot.Temp()
	if err != nil {
		return Summary{Results: failAll(dirPlan, err)}, nil
	}
	defer closeSnapshot(snapshots)
	if err := snapshots.Capture(ctx, dirPlan.Files); err != nil {
		if ctx.Err() != nil {
			return Summary{}, err
		}
		return Summary{Results: failAll(dirPlan, err)}, nil
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

	var summary Summary
	for _, profilePlan := range dirPlan.Profiles {
		profile := profilePlan.Profile
		extractors := r.extractors(dir, profile)
		profileSnapshot := browser.NewProfileSnapshot(dirSnapshot, profile, masterKeys)
		for _, dataCategory := range profilePlan.Categories {
			if err := ctx.Err(); err != nil {
				return summary, fmt.Errorf("run stopped: %w", err)
			}
			extraction := Extraction{UserDataDirID: dir.ID, ProfileName: profile.Name, Category: dataCategory}
			result, err := r.runExtraction(ctx, extraction, extractors[dataCategory], profileSnapshot, dir.Browser.DisplayName)
			if err != nil {
				return summary, err
			}
			summary.Results = append(summary.Results, result)
		}
	}
	if dirKeyRetrieval != nil && dirKeyRetrieval.err != nil {
		keyErr := fmt.Errorf("master key retrieval: %w", dirKeyRetrieval.err)
		summary.DirFailures = []DirFailure{{UserDataDirID: dir.ID, Err: keyErr}}
	}
	return summary, nil
}

// runExtraction exports the records emitted before an extraction failed, as they always were; its error stops the run.
func (r runner) runExtraction(
	ctx context.Context, extraction Extraction, extractor browser.Extractor, profileSnapshot browser.ProfileSnapshot, browserName string,
) (Result, error) {
	recordCollector := &collector{browserName: browserName, profileName: extraction.ProfileName}
	err := extractor.Extract(ctx, profileSnapshot, recordCollector)
	if writeErr := r.writer.Write(extraction.Category, recordCollector.records); writeErr != nil {
		return Result{}, writeErr
	}
	return recordCollector.result(extraction, err), nil
}

func failAll(dirPlan DirPlan, err error) []Result {
	var results []Result
	for _, profilePlan := range dirPlan.Profiles {
		for _, dataCategory := range profilePlan.Categories {
			extraction := Extraction{UserDataDirID: dirPlan.Dir.ID, ProfileName: profilePlan.Profile.Name, Category: dataCategory}
			results = append(results, Result{Extraction: extraction, Outcome: OutcomeError, Err: err})
		}
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
