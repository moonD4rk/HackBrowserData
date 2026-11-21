package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// readOnce lets every protected Firefox profile of a run share one read, since stdin yields the password only once.
func readOnce(read func(context.Context) (string, error)) func(context.Context) (string, error) {
	if read == nil {
		return nil
	}
	var once sync.Once
	var password string
	var err error
	return func(ctx context.Context) (string, error) {
		once.Do(func() { password, err = read(ctx) })
		return password, err
	}
}

// execute gives the runner the run's writer; the caller sets where its extractors, keys, and keychain come from.
// It handles one user data dir at a time, so at most one dir's copies are on disk. Its error means nothing ran.
func execute(
	ctx context.Context, discovery Discovery, categories []category.Category, output OutputOptions, dirRunner runner,
) (Summary, error) {
	writer, err := export.NewWriter(output.Dir, output.Format, output.Zip)
	if err != nil {
		return Summary{}, err
	}
	dirRunner.writer, dirRunner.logProgress = writer, true
	dirRunner.maxStorageValue = output.MaxStorageValue
	summary := dirRunner.run(ctx, discovery, categories)
	summary.Err = errors.Join(summary.Err, writer.Close())
	summary.Exported = writer.Listing()
	return summary, nil
}

type recordWriter interface {
	Write(dataCategory category.Category, records []record.Record) error
}

// runner is what every user data dir of one run shares; logProgress logs each dir as it starts.
type runner struct {
	writer          recordWriter
	extractors      extractorsFunc
	retrieveKeys    dirKeysFunc
	loginKeychain   browser.LoginKeychain
	firefoxPassword func(context.Context) (string, error)
	logProgress     bool
	maxStorageValue int
}

// run ends at the first dir whose Summary has an Err, keeping what ran before it.
func (r runner) run(ctx context.Context, discovery Discovery, categories []category.Category) Summary {
	summary := Summary{DirFailures: append([]DirFailure(nil), discovery.DirFailures...)}
	for _, dir := range discovery.Dirs {
		if err := ctx.Err(); err != nil {
			summary.Err = fmt.Errorf("run stopped: %w", err)
			return summary
		}
		dirSummary := r.runDir(ctx, dir, categories)
		summary.Results = append(summary.Results, dirSummary.Results...)
		summary.DirFailures = append(summary.DirFailures, dirSummary.DirFailures...)
		if dirSummary.Err != nil {
			summary.Err = dirSummary.Err
			return summary
		}
	}
	return summary
}

func (r runner) runDir(ctx context.Context, dir browser.UserDataDir, categories []category.Category) Summary {
	plan := planDir(dir, categories, r.extractors)
	summary := Summary{Results: plan.skipped}
	if len(plan.extractions) == 0 {
		return summary
	}
	if r.logProgress {
		log.Infof("Extracting %s...", dir.Browser.DisplayName)
	}

	root, err := snapshot.NewRoot()
	if err != nil {
		summary.Results = append(summary.Results, failAll(plan.extractions, err)...)
		return summary
	}
	defer closeRoot(root)
	if err := root.Capture(ctx, plan.files); err != nil {
		if ctx.Err() != nil {
			summary.Err = err
			return summary
		}
		summary.Results = append(summary.Results, failAll(plan.extractions, err)...)
		return summary
	}
	dirSnapshot := root.Snapshot(string(dir.ID))

	keys := &dirKeys{dir: dir, dirSnapshot: dirSnapshot}
	if dir.Browser.Base == browser.Chromium {
		keys.retrieve = r.retrieveKeys
	}
	for _, extraction := range plan.extractions {
		if err := ctx.Err(); err != nil {
			summary.Err = fmt.Errorf("run stopped: %w", err)
			return summary
		}
		input := browser.NewInput(dirSnapshot, extraction.profile, keys.get, r.loginKeychain).WithFirefoxPassword(r.firefoxPassword)
		result, err := r.runExtraction(ctx, extraction, input, dir.Browser.DisplayName)
		if err != nil {
			summary.Err = err
			return summary
		}
		summary.Results = append(summary.Results, result)
	}
	if keys.err != nil {
		keyErr := fmt.Errorf("master key retrieval: %w", keys.err)
		summary.DirFailures = []DirFailure{{UserDataDirID: dir.ID, Err: keyErr}}
	}
	return summary
}

// runExtraction exports the records emitted before an extraction failed, as they always were; its error stops the run.
func (r runner) runExtraction(
	ctx context.Context, extraction plannedExtraction, input browser.Input, browserName string,
) (Result, error) {
	recordCollector := &collector{browserName: browserName, profileName: extraction.ProfileName, maxStorageValue: r.maxStorageValue}
	err := extraction.extractor.Extract(ctx, input, recordCollector)
	if writeErr := r.writer.Write(extraction.Category, recordCollector.records); writeErr != nil {
		return Result{}, writeErr
	}
	return recordCollector.result(extraction.Extraction, err), nil
}

func failAll(extractions []plannedExtraction, err error) []Result {
	results := make([]Result, len(extractions))
	for i, extraction := range extractions {
		results[i] = Result{Extraction: extraction.Extraction, Outcome: OutcomeError, Err: err}
	}
	return results
}

func closeRoot(root *snapshot.Root) {
	if err := root.Close(); err != nil {
		log.Warnf("%v", err)
	}
}
