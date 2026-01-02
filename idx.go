package idx

import (
	"context"
	"database/sql"
	"fmt"
	"idx/db"
	"idx/detect"
	bitbucketcloud "idx/targets/bitbucket-cloud"
	bitbucketdc "idx/targets/bitbucket-dc"
	"log/slog"
	"strings"
	"time"
)

func Explore(ctx context.Context, config *Config, queries *db.Queries, runID int64) error {
	detector := detect.DefaultDetector

	for name, target := range config.Targets.BitbucketCloud {
		if target.Disabled {
			continue
		}

		slog.Info("start exploring", "target", name)
		start := time.Now()

		// memory store is repsonsible for deduplication of content across multiple runs
		memory := newMemoryStore(ctx, queries, "bitbucket-cloud", name, runID)

		// analyze function processes content blobs and runs detection on them
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
	}

	for name, target := range config.Targets.BitbucketDC {
		if target.Disabled {
			continue
		}

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
	}

	return nil
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
