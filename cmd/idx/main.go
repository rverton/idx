package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"idx"
	"idx/db"
	bitbucketcloud "idx/targets/bitbucket-cloud"
	bitbucketdc "idx/targets/bitbucket-dc"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

var rootFlags = ff.NewFlagSet("idx")
var verbose = rootFlags.Bool('v', "verbose", "Enable debug logging")

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
			configCmd(),
			listRunsCmd(),
			listFindingsCmd(),
		},
		Exec: func(ctx context.Context, args []string) error {
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

			runId, err := queries.InsertRun(ctx, time.Now().Unix())
			if err != nil {
				return fmt.Errorf("failed to update run: %w", err)
			}

			if err := idx.Explore(ctx, config, queries, runId); err != nil {
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
		},
	}

	if err := rootCmd.ParseAndRun(
		context.Background(),
		os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n\n", err)
		fmt.Fprintf(os.Stderr, "%s\n", ffhelp.Command(rootCmd))
		os.Exit(0)
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

			return nil
		},
	}
}

func configVerifyCmd() *ff.Command {
	flags := ff.NewFlagSet("verify")
	return &ff.Command{
		Name:      "verify",
		Usage:     "idx config verify [target]",
		ShortHelp: "Verifies the config file structure and tests connections",
		Flags:     flags,
		Exec: func(ctx context.Context, args []string) error {
			config, err := loadConfig(configFilename, configFilenameEnc)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			verifyTargets(ctx, config)

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

func verifyTargets(ctx context.Context, config *idx.Config) {
	for name, target := range config.Targets.BitbucketCloud {
		client, err := bitbucketcloud.NewAPIClient(target.Username, target.ApiToken)
		if err != nil {
			slog.Error("failed to create Bitbucket Cloud client", "target", name, "error", err)
			continue
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
	}

	// Verify Bitbucket Data Center targets
	for name, target := range config.Targets.BitbucketDC {
		client, err := bitbucketdc.NewAPIClient(target.BaseURL, target.Username, target.ApiToken)
		if err != nil {
			slog.Error("failed to create Bitbucket DC client", "target", name, "error", err)
			continue
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
	}
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
