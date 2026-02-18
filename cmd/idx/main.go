package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"idx"
	"idx/db"
	bitbucketcloud "idx/targets/bitbucket-cloud"
	bitbucketdc "idx/targets/bitbucket-dc"
	confluencedc "idx/targets/confluence-dc"
	jiradc "idx/targets/jira-dc"
	"idx/targets/smb"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"golang.org/x/sync/errgroup"
)

var rootFlags = ff.NewFlagSet("idx")
var verbose = rootFlags.Bool('v', "verbose", "Enable debug logging")
var concurrencyLimit = rootFlags.Int('c', "concurrency", -1, "how many types of targets to explore concurrently (default: one worker per type)")
var repeatDuration = rootFlags.Duration(0, "repeat", 0, "wait duration between runs (e.g. 1h, 6h, 24h); 0 disables repeat")

const (
	configFilename    = "config.json"
	configFilenameEnc = "config.json.enc"
	dbFilename        = "idx.db"
)

func main() {
	rootCmd := &ff.Command{
		Name:  "idx",
		Usage: "idx [FLAGS] <subcommand>",
		Flags: rootFlags,
		Subcommands: []*ff.Command{
			runCmd(),
			configCmd(),
			listRunsCmd(),
			listFindingsCmd(),
		},
	}

	if err := rootCmd.ParseAndRun(
		context.Background(),
		os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		if errors.Is(err, ff.ErrHelp) || errors.Is(err, ff.ErrNoExec) {
			fmt.Fprintf(os.Stderr, "%s\n", ffhelp.Command(rootCmd))
			os.Exit(0)
		}

		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func runCmd() *ff.Command {
	runFlags := ff.NewFlagSet("run").SetParent(rootFlags)
	return &ff.Command{
		Name:      "run",
		Usage:     "idx run [FLAGS]",
		ShortHelp: "Starts an exploration run",
		Flags:     runFlags,
		Exec: func(ctx context.Context, args []string) error {
			if *repeatDuration < 0 {
				return fmt.Errorf("--repeat must be >= 0")
			}

			if *verbose {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
			}
			slog.Info("internal data explorer started")

			config, err := loadConfig(configFilename, configFilenameEnc)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			queries, err := db.Connect(ctx, dbFilename)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}

			for {
				runStart := time.Now()
				if err := runExploreOnce(ctx, config, queries, *concurrencyLimit); err != nil {
					return err
				}
				runDuration := time.Since(runStart)

				if *repeatDuration == 0 {
					return nil
				}

				slog.Info("run completed, waiting before next run", "duration", runDuration, "repeat", repeatDuration.String())
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(*repeatDuration):
				}
			}
		},
	}
}

func configCmd() *ff.Command {
	return &ff.Command{
		Name:      "config",
		Usage:     "idx config <subcommand>",
		ShortHelp: "Manage configuration file", // Updated ShortHelp
		Subcommands: []*ff.Command{
			configInitCmd(),
			configEncryptCmd(),
			configDecryptCmd(),
			configVerifyCmd(),
			configListTargetsCmd(),
		},
	}
}

func runExploreOnce(ctx context.Context, config *idx.Config, queries *db.Queries, concurrencyLimit int) error {
	runId, err := queries.InsertRun(ctx, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	if err := idx.Explore(ctx, config, queries, runId, concurrencyLimit); err != nil {
		if err := queries.UpdateRun(ctx, db.UpdateRunParams{
			ID:     runId,
			Status: "failed",
			ErrorMessage: sql.NullString{
				String: err.Error(),
				Valid:  true,
			},
			FinishedAt: sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		}); err != nil {
			fmt.Printf("failed to update failed run: %v", err)
		}

		return fmt.Errorf("exploration failed: %w", err)
	}

	if err := queries.UpdateRun(ctx, db.UpdateRunParams{
		ID:         runId,
		Status:     "completed",
		FinishedAt: sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	return nil
}

func listRunsCmd() *ff.Command {
	return &ff.Command{
		Name:      "list-runs",
		Usage:     "idx list-runs",
		ShortHelp: "Lists all runs from the database",
		Exec: func(ctx context.Context, args []string) error {
			queries, err := db.Connect(ctx, dbFilename)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}

			runs, err := queries.ListRuns(ctx)
			if err != nil {
				return fmt.Errorf("failed to list runs: %w", err)
			}

			if len(runs) == 0 {
				fmt.Println("No runs found.")
				return nil
			}

			fmt.Println("Runs:")
			for _, run := range runs {
				startedAt := db.FormatTimestamp(run.StartedAt)
				finishedAt := db.FormatNullTimestamp(run.FinishedAt, "(in progress)")

				if run.Status == "failed" && run.ErrorMessage.Valid {
					fmt.Printf("- ID: %d, Status: %s, Started At: %s, Finished At: %s, Error: %s\n",
						run.ID, run.Status, startedAt, finishedAt, run.ErrorMessage.String)
					continue
				}

				fmt.Printf("- ID: %d, Status: %s, Started At: %s, Finished At: %s\n", run.ID, run.Status, startedAt, finishedAt)
			}

			return nil
		},
	}
}

func listFindingsCmd() *ff.Command {
	return &ff.Command{
		Name:      "list-findings",
		Usage:     "idx list-findings",
		ShortHelp: "Lists all findings from the database",
		Exec: func(ctx context.Context, args []string) error {
			queries, err := db.Connect(ctx, dbFilename)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}

			findings, err := queries.ListFindings(ctx)
			if err != nil {
				return fmt.Errorf("failed to list findings: %w", err)
			}

			if len(findings) == 0 {
				fmt.Println("No findings found.")
				return nil
			}

			fmt.Println("Findings:")
			for _, f := range findings {
				detectedAt := db.FormatTimestamp(f.DetectedAt)
				fmt.Printf("- [%s] %s: %s\n", detectedAt, f.RuleName, f.RuleDescription)
				fmt.Printf("  Target: %s/%s\n", f.TargetType, f.TargetName)
				fmt.Printf("  Location: %s\n", f.Location)
				fmt.Printf("  Match: %s\n", f.Match)
				fmt.Println()
			}

			return nil
		},
	}
}

func configListTargetsCmd() *ff.Command {
	flags := ff.NewFlagSet("targets-list")
	return &ff.Command{
		Name:      "targets-list",
		Usage:     "idx config targets-list",
		ShortHelp: "Lists all targets in the config file",
		Flags:     flags,
		Exec: func(ctx context.Context, args []string) error {
			config, err := loadConfig(configFilename, configFilenameEnc)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Println("Targets:")
			for name := range config.Targets.BitbucketCloud {
				fmt.Printf("- Bitbucket Cloud: %s\n", name)
			}
			for name := range config.Targets.BitbucketDC {
				fmt.Printf("- Bitbucket Data Center: %s\n", name)
			}
			for name := range config.Targets.ConfluenceDC {
				fmt.Printf("- Confluence Data Center: %s\n", name)
			}
			for name := range config.Targets.JiraDC {
				fmt.Printf("- Jira Data Center: %s\n", name)
			}
			for name := range config.Targets.SMB {
				fmt.Printf("- SMB: %s\n", name)
			}

			return nil
		},
	}
}

func configVerifyCmd() *ff.Command {
	flags := ff.NewFlagSet("verify").SetParent(rootFlags)
	return &ff.Command{
		Name:      "verify",
		Usage:     "idx config verify [--concurrency N]",
		ShortHelp: "Verifies the config file structure and tests connections",
		Flags:     flags,
		Exec: func(ctx context.Context, args []string) error {
			config, err := loadConfig(configFilename, configFilenameEnc)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			verifyTargets(ctx, config, *concurrencyLimit)

			return nil
		},
	}
}

func configInitCmd() *ff.Command {
	return &ff.Command{
		Name:      "init",
		Usage:     "idx config init",
		ShortHelp: "creates a new (unencrypted) config file",
		Exec: func(ctx context.Context, args []string) error {
			if _, err := os.Stat(configFilename); err == nil {
				return fmt.Errorf("%v already exists", configFilename)
			}

			initFile, err := os.Create(configFilename)
			if err != nil {
				return fmt.Errorf("failed to create %v: %w", configFilename, err)
			}
			defer initFile.Close()

			_, err = initFile.WriteString(tplConfig)
			if err != nil {
				return fmt.Errorf("failed to write to %v: %w", configFilename, err)
			}

			return nil
		},
	}
}

func configEncryptCmd() *ff.Command {
	encFlags := ff.NewFlagSet("encrypt")
	return &ff.Command{
		Name:      "encrypt",
		Usage:     "idx config encrypt",
		ShortHelp: "encrypts the config file and removes the unencrypted one",
		Flags:     encFlags,
		Exec: func(ctx context.Context, args []string) error {
			pw, err := readPasswordSafe(true)
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			if err := encryptConfigFile(configFilename, pw); err != nil {
				return fmt.Errorf("failed to encrypt config file: %w", err)
			}

			// remove unencrypted config file
			if err := os.Remove(configFilename); err != nil {
				return fmt.Errorf("failed to remove unencrypted config file: %w", err)
			}

			return nil
		},
	}
}

func configDecryptCmd() *ff.Command {
	decFlags := ff.NewFlagSet("decrypt")
	return &ff.Command{
		Name:      "decrypt",
		Usage:     "idx config decrypt",
		ShortHelp: "decrypts the config file and removes the encrypted one",
		Flags:     decFlags,
		Exec: func(ctx context.Context, args []string) error {
			pw, err := readPasswordSafe(false)
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			plaintext, err := decryptConfigFile(configFilenameEnc, pw)
			if err != nil {
				return fmt.Errorf("failed to decrypt config file: %w", err)
			}

			// write decrypted config file
			decFile, err := os.Create(configFilename)
			if err != nil {
				return fmt.Errorf("create decrypted config file: %w", err)
			}

			defer decFile.Close()
			if _, err := decFile.Write(plaintext); err != nil {
				return fmt.Errorf("write decrypted config file: %w", err)
			}

			// remove unencrypted config file
			if err := os.Remove(configFilenameEnc); err != nil {
				return fmt.Errorf("failed to remove unencrypted config file: %w", err)
			}

			return nil
		},
	}
}

type verificationResult struct {
	Name     string
	Type     string
	Success  bool
	Error    error
	Duration time.Duration
}

func verifyTargets(ctx context.Context, config *idx.Config, concurrencyLimit int) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrencyLimit)

	for name, target := range config.Targets.BitbucketCloud {
		g.Go(func() error {
			client, err := bitbucketcloud.NewAPIClient(target.Username, target.ApiToken)
			if err != nil {
				slog.Error("failed to create Bitbucket Cloud client", "target", name, "error", err)
				return nil
			}

			if err := client.VerifyConnection(ctx); err != nil {
				slog.Error(
					"Bitbucket Cloud target verification failed",
					"target",
					name,
					"username",
					target.Username,
					"len(apiToken)",
					len(target.ApiToken),
					"error",
					err,
				)
			} else {
				slog.Info(
					"Bitbucket Cloud target verification succeeded",
					"target",
					name,
					"username",
					target.Username,
					"len(apiToken)",
					len(target.ApiToken),
				)
			}
			return nil
		})
	}

	for name, target := range config.Targets.BitbucketDC {
		g.Go(func() error {
			client, err := bitbucketdc.NewAPIClient(target.BaseURL, target.Username, target.ApiToken)
			if err != nil {
				slog.Error("failed to create Bitbucket DC client", "target", name, "error", err)
				return nil
			}

			if err := client.VerifyConnection(ctx); err != nil {
				slog.Error(
					"Bitbucket DC target verification failed",
					"target",
					name,
					"username",
					target.Username,
					"len(apiToken)",
					len(target.ApiToken),
					"error",
					err,
				)
			} else {
				slog.Info(
					"Bitbucket DC target verification succeeded",
					"target",
					name,
					"username",
					target.Username,
					"len(apiToken)",
					len(target.ApiToken),
				)
			}
			return nil
		})
	}

	for name, target := range config.Targets.ConfluenceDC {
		g.Go(func() error {
			client, err := confluencedc.NewAPIClient(target.BaseURL, target.ApiToken)
			if err != nil {
				slog.Error("failed to create Confluence DC client", "target", name, "error", err)
				return nil
			}

			if err := client.VerifyConnection(ctx); err != nil {
				slog.Error(
					"Confluence DC target verification failed",
					"target",
					name,
					"len(apiToken)",
					len(target.ApiToken),
					"error",
					err,
				)
			} else {
				slog.Info(
					"Confluence DC target verification succeeded",
					"target",
					name,
					"len(apiToken)",
					len(target.ApiToken),
				)
			}
			return nil
		})
	}

	for name, target := range config.Targets.JiraDC {
		g.Go(func() error {
			client, err := jiradc.NewAPIClient(target.BaseURL, target.ApiToken)
			if err != nil {
				slog.Error("failed to create Jira DC client", "target", name, "error", err)
				return nil
			}

			if err := client.VerifyConnection(ctx); err != nil {
				slog.Error(
					"Jira DC target verification failed",
					"target",
					name,
					"len(apiToken)",
					len(target.ApiToken),
					"error",
					err,
				)
			} else {
				slog.Info(
					"Jira DC target verification succeeded",
					"target",
					name,
					"len(apiToken)",
					len(target.ApiToken),
				)
			}
			return nil
		})
	}

	for name, target := range config.Targets.SMB {
		g.Go(func() error {
			client, err := smb.NewClient(target.Hostname, target.Port, target.NTLMUser, target.NTLMPassword, target.Domain)
			if err != nil {
				slog.Error("failed to create SMB client", "target", name, "error", err)
				return nil
			}
			defer client.Close()

			if err := client.VerifyConnection(ctx); err != nil {
				slog.Error(
					"SMB target verification failed",
					"target",
					name,
					"hostname",
					target.Hostname,
					"error",
					err,
				)
			} else {
				slog.Info(
					"SMB target verification succeeded",
					"target",
					name,
					"hostname",
					target.Hostname,
				)
			}
			return nil
		})
	}

	g.Wait()
}

func parseConfig(content []byte) (*idx.Config, error) {
	var config idx.Config
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &config, nil
}

func loadConfig(configFilename, configFilenameEnc string) (*idx.Config, error) {
	var plaintext []byte
	var err error

	// first check if there is an unencrypted config file
	// and use it if available
	if _, err = os.Stat(configFilename); err == nil {
		log.Printf("warning: using unencrypted %v", configFilename)
		plaintext, err = os.ReadFile(configFilename)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		// if not, try to read the encrypted config file
	} else {
		pw, err := readPasswordSafe(false)
		if err != nil {
			return nil, fmt.Errorf("failed to read password: %w", err)
		}

		if plaintext, err = decryptConfigFile(configFilenameEnc, pw); err != nil {
			return nil, fmt.Errorf("failed to decrypt config file: %w", err)
		}
	}

	return parseConfig(plaintext)
}
