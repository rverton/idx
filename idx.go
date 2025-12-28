package idx

import (
	"context"
	"fmt"
	bitbucketcloud "idx/targets/bitbucket-cloud"
	"log/slog"
	"time"
)

func Explore(ctx context.Context, config *Config) error {

	for name, target := range config.Targets.BitbucketCloud {
		if target.Disabled {
			continue
		}

		slog.Info("start exploring", "target", name)
		start := time.Now()

		if err := bitbucketcloud.Explore(
			ctx,
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
