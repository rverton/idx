package idx

import (
	"context"
	"fmt"
	"idx/detect"
	bitbucketcloud "idx/targets/bitbucket-cloud"
	"log/slog"
	"time"
)

func Explore(ctx context.Context, config *Config) error {
	detector := detect.DefaultDetector

	for name, target := range config.Targets.BitbucketCloud {
		if target.Disabled {
			continue
		}

		slog.Info("start exploring", "target", name)
		start := time.Now()

		if err := bitbucketcloud.Explore(
			ctx,
			newAnalyzeFunc(&detector),
			name,
			target.Username,
			target.ApiToken,
			target.Workspaces,
		); err != nil {
			return fmt.Errorf("failed to explore Bitbucket Cloud target %s: %w", name, err)
		}

		slog.Info("finished exploring", "target", name, "duration", time.Since(start))
	}

	return nil
}

func newAnalyzeFunc(detector *detect.Detector) func(content detect.Content) {
	return func(content detect.Content) {
		slog.Debug("analyzing content", "key", content.Key, "location", content.Location)

		for _, finding := range detector.Detect(content) {
			slog.Info("finding detected",
				"rule", finding.Rule.Name,
				"description", finding.Rule.Description,
				"content_key", finding.ContentKey,
				"location", finding.Location,
			)
		}
	}
}
