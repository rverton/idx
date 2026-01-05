package idx

import (
	"context"
	"database/sql"
	"fmt"
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

// Explore explores all configured targets concurrently with a limit on the number of concurrent explorations.
//
// iterating over each target type one by one looks a bit repetitive, but it makes it clear
// what specific analysers and memory stores are being used for each target type.
func Explore(ctx context.Context, config *Config, queries *db.Queries, runID int64, concurrencyLimit int) error {
	detector := detect.DefaultDetector

	// we are using an errgroup to explore multiple targets concurrently
	// while limiting the number of concurrent explorations
	// to avoid overwhelming the system.
	//
	// there are different layers where we can use concurrency here, but we choose
	// to limit it at the target level for simplicity and to avoid hitting rate limits
	// when more than one target is using the same service.
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrencyLimit)

	for name, target := range config.Targets.BitbucketCloud {
		if target.Disabled {
			continue
		}

		g.Go(func() error {
			slog.Info("start exploring", "target", name)
			start := time.Now()

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
			); err != nil {
				return fmt.Errorf("failed to explore Bitbucket Cloud target %s: %w", name, err)
			}

			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
			return nil
		})
	}

	for name, target := range config.Targets.BitbucketDC {
		if target.Disabled {
			continue
		}

		g.Go(func() error {
			slog.Info("start exploring", "target", name)
			start := time.Now()

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
			); err != nil {
				return fmt.Errorf("failed to explore Bitbucket DC target %s: %w", name, err)
			}

			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
			return nil
		})
	}

	for name, target := range config.Targets.ConfluenceDC {
		if target.Disabled {
			continue
		}

		g.Go(func() error {
			slog.Info("start exploring", "target", name)
			start := time.Now()

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
			); err != nil {
				return fmt.Errorf("failed to explore Confluence DC target %s: %w", name, err)
			}

			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
			return nil
		})
	}

	for name, target := range config.Targets.JiraDC {
		if target.Disabled {
			continue
		}

		g.Go(func() error {
			slog.Info("start exploring", "target", name)
			start := time.Now()

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
			); err != nil {
				return fmt.Errorf("failed to explore Jira DC target %s: %w", name, err)
			}

			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
			return nil
		})
	}

	for name, target := range config.Targets.SMB {
		if target.Disabled {
			continue
		}

		g.Go(func() error {
			slog.Info("start exploring", "target", name)
			start := time.Now()

			memory := newMemoryStore(ctx, queries, "smb", name, runID)
			analyse := newAnalyzeFunc(ctx, queries, &detector, "smb", name, runID)
			analyseFilename := newAnalyzeFilenameFunc(ctx, queries, &detector, "smb", name, runID)

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
			); err != nil {
				return fmt.Errorf("failed to explore SMB target %s: %w", name, err)
			}

			slog.Info("finished exploring", "target", name, "duration", time.Since(start))
			return nil
		})
	}

	return g.Wait()
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
	}
}

func newAnalyzeFunc(ctx context.Context, q *db.Queries, detector *detect.Detector, targetType, targetName string, runID int64) func(content detect.Content) {
	return func(content detect.Content) {
		slog.Debug("analyzing content", "key", content.Key, "location", content.Location)

		for _, finding := range detector.Detect(content) {
			slog.Info("finding detected",
				"rule", finding.Rule.Name,
				"description", finding.Rule.Description,
				"content_key", finding.ContentKey,
				"location", finding.Location,
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
				"rule", finding.Rule.Name,
				"description", finding.Rule.Description,
				"content_key", finding.ContentKey,
				"location", finding.Location,
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
