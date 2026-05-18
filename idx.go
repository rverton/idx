package idx

import (
	"context"
	"database/sql"
	"idx/db"
	"idx/detect"
	bitbucketcloud "idx/targets/bitbucket-cloud"
	bitbucketdc "idx/targets/bitbucket-dc"
	confluencedc "idx/targets/confluence-dc"
	jiradc "idx/targets/jira-dc"
	"idx/targets/smb"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// throttleDuration returns the configured throttle duration or the given default.
// A negative value disables throttling, zero uses the default.
func throttleDuration(configuredMs, defaultMs int) time.Duration {
	if configuredMs < 0 {
		return 0
	}

	ms := defaultMs
	if configuredMs > 0 {
		ms = configuredMs
	}

	return time.Duration(ms) * time.Millisecond
}

// Explore explores all configured targets concurrently with a limit on the number of concurrent explorations.
//
// iterating over each target type one by one looks a bit repetitive, but it makes it clear
// what specific analysers and memory stores are being used for each target type.
func Explore(ctx context.Context, config *Config, queries *db.Queries, runID int64, concurrencyLimit int) error {
	detector := detect.DefaultDetector

	// we are using an errgroup to explore multiple targets concurrently
	// while limiting the number of concurrent explorations to avoid
	// overwhelming the system.
	//
	// there are different layers where we can use concurrency here, but we choose
	// to limit it at the target level for simplicity and to avoid hitting rate limits
	// when more than one target (for a specific type) is using the same service.
	g := errgroup.Group{}
	g.SetLimit(concurrencyLimit)

	g.Go(func() error {
		for name, target := range config.Targets.BitbucketCloud {
			if target.Disabled {
				continue
			}

			start := time.Now()
			logTargetStarted(runID, "bitbucket-cloud", name)

			memory := newMemoryStore(ctx, queries, "bitbucket-cloud", name, runID)
			analyse := newAnalyzeFunc(ctx, queries, &detector, "bitbucket-cloud", name, runID)

			if err := bitbucketcloud.Explore(
				ctx,
				analyse,
				memory,
				name,
				target.Username,
				target.ApiToken,
				target.Workspaces,
				throttleDuration(target.ThrottleMs, 100),
			); err != nil {
				logTargetFailed(runID, "bitbucket-cloud", name, start, err)
				continue
			}

			logTargetCompleted(runID, "bitbucket-cloud", name, start)
			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
		}

		return nil
	})

	g.Go(func() error {
		for name, target := range config.Targets.BitbucketDC {
			if target.Disabled {
				continue
			}

			start := time.Now()
			logTargetStarted(runID, "bitbucket-dc", name)

			memory := newMemoryStore(ctx, queries, "bitbucket-dc", name, runID)
			analyse := newAnalyzeFunc(ctx, queries, &detector, "bitbucket-dc", name, runID)

			if err := bitbucketdc.Explore(
				ctx,
				analyse,
				memory,
				name,
				target.BaseURL,
				target.Username,
				target.ApiToken,
				throttleDuration(target.ThrottleMs, 100),
			); err != nil {
				logTargetFailed(runID, "bitbucket-dc", name, start, err)
				continue
			}

			logTargetCompleted(runID, "bitbucket-dc", name, start)
			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
		}
		return nil
	})

	g.Go(func() error {
		for name, target := range config.Targets.ConfluenceDC {
			if target.Disabled {
				continue
			}

			start := time.Now()
			logTargetStarted(runID, "confluence-dc", name)

			memory := newMemoryStore(ctx, queries, "confluence-dc", name, runID)
			analyse := newAnalyzeFunc(ctx, queries, &detector, "confluence-dc", name, runID)

			if err := confluencedc.Explore(
				ctx,
				analyse,
				memory,
				name,
				target.BaseURL,
				target.ApiToken,
				target.SpaceNames,
				target.DisableHistorySearch,
				throttleDuration(target.ThrottleMs, 100),
			); err != nil {
				logTargetFailed(runID, "confluence-dc", name, start, err)
				continue
			}

			logTargetCompleted(runID, "confluence-dc", name, start)
			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
		}
		return nil
	})

	g.Go(func() error {
		for name, target := range config.Targets.JiraDC {
			if target.Disabled {
				continue
			}

			start := time.Now()
			logTargetStarted(runID, "jira-dc", name)

			memory := newMemoryStore(ctx, queries, "jira-dc", name, runID)
			analyse := newAnalyzeFunc(ctx, queries, &detector, "jira-dc", name, runID)
			analyseFilename := newAnalyzeFilenameFunc(ctx, queries, &detector, "jira-dc", name, runID)

			if err := jiradc.Explore(
				ctx,
				analyse,
				analyseFilename,
				memory,
				name,
				target.BaseURL,
				target.ApiToken,
				target.ProjectKeys,
				throttleDuration(target.ThrottleMs, 100),
			); err != nil {
				logTargetFailed(runID, "jira-dc", name, start, err)
				continue
			}

			logTargetCompleted(runID, "jira-dc", name, start)
			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
		}
		return nil
	})

	g.Go(func() error {
		for name, target := range config.Targets.SMB {
			if target.Disabled {
				continue
			}

			start := time.Now()
			logTargetStarted(runID, "smb", name)

			memory := newMemoryStore(ctx, queries, "smb", name, runID)
			analyse := newAnalyzeFunc(ctx, queries, &detector, "smb", name, runID)
			analyseFilename := newAnalyzeFilenameFunc(ctx, queries, &detector, "smb", name, runID)

			folderCacheDepth := target.FolderCacheDepth
			if folderCacheDepth == 0 {
				folderCacheDepth = 2
			}

			rescanDuration := 72 * time.Hour
			if target.FolderRescanDuration != "" {
				parsed, err := time.ParseDuration(target.FolderRescanDuration)
				if err != nil {
					slog.Warn("invalid folderRescanDuration, using default",
						"target", name,
						"value", target.FolderRescanDuration,
						"default", "72h")
				} else {
					rescanDuration = parsed
				}
			}

			if err := smb.Explore(
				ctx,
				analyse,
				analyseFilename,
				memory,
				name,
				target.Hostname,
				target.Port,
				target.NTLMUser,
				target.NTLMPassword,
				target.Domain,
				target.MaxRecursiveDepth,
				folderCacheDepth,
				rescanDuration,
				throttleDuration(target.ThrottleMs, 0),
			); err != nil {
				logTargetFailed(runID, "smb", name, start, err)
				continue
			}

			logTargetCompleted(runID, "smb", name, start)
			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
		}
		return nil
	})

	return g.Wait()
}

func logTargetStarted(runID int64, targetType, targetName string) {
	slog.Info("target started",
		"event", "target.started",
		"run_id", runID,
		"target_type", targetType,
		"target_name", targetName,
	)
}

func logTargetCompleted(runID int64, targetType, targetName string, start time.Time) {
	slog.Info("target completed",
		"event", "target.completed",
		"run_id", runID,
		"target_type", targetType,
		"target_name", targetName,
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

func logTargetFailed(runID int64, targetType, targetName string, start time.Time, err error) {
	slog.Error("target failed",
		"event", "target.failed",
		"run_id", runID,
		"target_type", targetType,
		"target_name", targetName,
		"duration_ms", time.Since(start).Milliseconds(),
		"error", err,
	)
}

func newMemoryStore(ctx context.Context, q *db.Queries, targetType, targetName string, runID int64) detect.MemoryStore {
	return detect.MemoryStore{
		Has: func(key string) bool {
			hasKey, err := q.HasMemoryKey(ctx, key)
			if err != nil {
				slog.Error("memory store: failed to check key", "key", key, "error", err)
				return false
			}
			return hasKey == 1
		},
		Set: func(key string) {
			err := q.SetMemoryKey(ctx, db.SetMemoryKeyParams{
				Key:        key,
				TargetType: targetType,
				TargetName: targetName,
				RunID:      sql.NullInt64{Int64: runID, Valid: true},
			})
			if err != nil {
				slog.Error("memory store: failed to set key", "key", key, "error", err)
			}
		},
		GetTimestamp: func(key string) (int64, bool) {
			ts, err := q.GetMemoryAnalyzedAt(ctx, key)
			if err != nil {
				if err == sql.ErrNoRows {
					return 0, false
				}
				slog.Error("memory store: failed to get timestamp", "key", key, "error", err)
				return 0, false
			}
			return ts, true
		},
	}
}

func newAnalyzeFunc(ctx context.Context, q *db.Queries, detector *detect.Detector, targetType, targetName string, runID int64) func(content detect.Content) {
	return func(content detect.Content) {
		slog.Debug("analyzing content", "key", content.Key, "location", content.Location)

		for _, finding := range detector.Detect(content) {
			slog.Info("finding detected",
				"event", "finding.detected",
				"run_id", runID,
				"target_type", targetType,
				"target_name", targetName,
				"rule", finding.Rule.Name,
				"rule_name", finding.Rule.Name,
				"description", finding.Rule.Description,
				"content_key", finding.ContentKey,
				"location", strings.Join(finding.Location, "/"),
			)

			if err := q.InsertFinding(ctx, db.InsertFindingParams{
				RunID:           runID,
				TargetType:      targetType,
				TargetName:      targetName,
				RuleName:        finding.Rule.Name,
				RuleDescription: finding.Rule.Description,
				ContentKey:      finding.ContentKey,
				Location:        strings.Join(finding.Location, "/"),
				Match:           finding.Match,
			}); err != nil {
				slog.Error("failed to insert finding", "error", err)
			}
		}
	}
}

func newAnalyzeFilenameFunc(ctx context.Context, q *db.Queries, detector *detect.Detector, targetType, targetName string, runID int64) func(filename, contentKey string, location []string) {
	return func(filename, contentKey string, location []string) {
		slog.Debug("analyzing filename", "filename", filename, "location", location)

		for _, finding := range detector.DetectFilename(filename, contentKey, location) {
			slog.Info("filename finding detected",
				"event", "finding.detected",
				"run_id", runID,
				"target_type", targetType,
				"target_name", targetName,
				"rule", finding.Rule.Name,
				"rule_name", finding.Rule.Name,
				"description", finding.Rule.Description,
				"content_key", finding.ContentKey,
				"location", strings.Join(finding.Location, "/"),
			)

			if err := q.InsertFinding(ctx, db.InsertFindingParams{
				RunID:           runID,
				TargetType:      targetType,
				TargetName:      targetName,
				RuleName:        finding.Rule.Name,
				RuleDescription: finding.Rule.Description,
				ContentKey:      finding.ContentKey,
				Location:        strings.Join(finding.Location, "/"),
				Match:           finding.Match,
			}); err != nil {
				slog.Error("failed to insert filename finding", "error", err)
			}
		}
	}
}
