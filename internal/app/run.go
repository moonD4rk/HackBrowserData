package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/safari"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

type recordWriter interface {
	Write(dataCategory category.Category, records []record.Record) error
}

// runner is what every user data dir of one run shares; logProgress logs each dir as it starts.
type runner struct {
	keySource              keySource            // Chromium: each dir's master keys
	firefoxPrimaryPassword string               // Firefox
	loginKeychain          safari.LoginKeychain // Safari
	writer                 recordWriter
	logProgress            bool
	maxStorageValue        int
}

// export gives the run its writer; the caller sets each base's secret.
// It handles one user data dir at a time, so at most one dir's copies are on disk. Its error means nothing ran.
func (r runner) export(ctx context.Context, discovery Discovery, categories []category.Category, output OutputOptions) (Summary, error) {
	writer, err := export.NewWriter(output.Dir, output.Format, output.Zip)
	if err != nil {
		return Summary{}, err
	}
	r.writer, r.logProgress = writer, true
	r.maxStorageValue = output.MaxStorageValue
	summary := r.run(ctx, discovery, categories)
	summary.Err = errors.Join(summary.Err, writer.Close())
	summary.Exported = writer.Listing()
	return summary, nil
}

// run ends at the first dir whose Summary has an Err, keeping what ran before it.
func (r runner) run(ctx context.Context, discovery Discovery, categories []category.Category) Summary {
	summary := Summary{DirFailures: append([]DirFailure(nil), discovery.DirFailures...)}
	for _, dir := range discovery.Dirs {
		if err := ctx.Err(); err != nil {
			summary.Err = fmt.Errorf("run stopped: %w", err)
			return summary
		}
		dirSummary := r.runExtractions(ctx, dir, categories)
		summary.Results = append(summary.Results, dirSummary.Results...)
		summary.DirFailures = append(summary.DirFailures, dirSummary.DirFailures...)
		if dirSummary.Err != nil {
			summary.Err = dirSummary.Err
			return summary
		}
	}
	return summary
}

func (r runner) runExtractions(ctx context.Context, dir browser.UserDataDir, categories []category.Category) Summary {
	extractions, files, skipped := planExtractions(dir, categories)
	summary := Summary{Results: skipped}
	if len(extractions) == 0 {
		return summary
	}
	if r.logProgress {
		log.Infof("Extracting %s...", dir.Browser.DisplayName)
	}

	root, err := snapshot.NewRoot()
	if err != nil {
		summary.Results = append(summary.Results, failAll(extractions, err)...)
		return summary
	}
	defer closeRoot(root)
	if err := root.Capture(ctx, files); err != nil {
		if ctx.Err() != nil {
			summary.Err = err
			return summary
		}
		summary.Results = append(summary.Results, failAll(extractions, err)...)
		return summary
	}
	dirSnapshot := root.Snapshot(string(dir.ID))

	var masterKeys masterkey.Set
	if needsMasterKeys(dir, extractions) {
		retrieval := r.keySource.MasterKeys(ctx, dir, dirSnapshot)
		masterKeys = retrieval.Keys
		if retrieval.Err != nil {
			summary.DirFailures = []DirFailure{{DirID: dir.ID, Err: fmt.Errorf("master key retrieval: %w", retrieval.Err)}}
		}
	}
	for _, extraction := range extractions {
		if err := ctx.Err(); err != nil {
			summary.Err = fmt.Errorf("run stopped: %w", err)
			return summary
		}
		captured := browser.NewProfileSnapshot(dir, extraction.Profile, dirSnapshot)
		result, err := r.runExtraction(ctx, extraction, captured, masterKeys)
		if err != nil {
			summary.Err = err
			return summary
		}
		summary.Results = append(summary.Results, result)
	}
	return summary
}

// runExtraction exports the records emitted before an extraction failed, as they always were; its error stops the run.
func (r runner) runExtraction(
	ctx context.Context, extraction Extraction, captured browser.ProfileSnapshot, masterKeys masterkey.Set,
) (Result, error) {
	recordCollector := &collector{
		browserName: captured.UserDataDir.Browser.DisplayName, profileName: extraction.Profile.Name, maxStorageValue: r.maxStorageValue,
	}
	err := r.extract(ctx, captured, masterKeys, extraction.Category, recordCollector)
	if writeErr := r.writer.Write(extraction.Category, recordCollector.records); writeErr != nil {
		return Result{}, writeErr
	}
	return recordCollector.result(extraction, err), nil
}

func failAll(extractions []Extraction, err error) []Result {
	results := make([]Result, len(extractions))
	for i, extraction := range extractions {
		results[i] = Result{Extraction: extraction, Outcome: OutcomeError, Err: err}
	}
	return results
}

func closeRoot(root *snapshot.Root) {
	if err := root.Close(); err != nil {
		log.Warnf("%v", err)
	}
}
