// Package generate implements the `hashverifier generate` subcommand.
package generate

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/inhies/go-bytesize"
	"github.com/lithammer/dedent"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	"github.com/ostapkonst/HashVerifier/internal/domain/exclude"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
	servicegenerate "github.com/ostapkonst/HashVerifier/internal/service/generate"
)

func runGenerate(cmd *cobra.Command, args []string) error {
	return base.RunWithShutdown(cmd, func(ctx context.Context) error {
		return execGenerate(ctx, cmd, args)
	})
}

func execGenerate(ctx context.Context, cmd *cobra.Command, args []string) error {
	excludePaths, err := base.FlagStringArray(cmd, "exclude")
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	inputDir := filepath.Clean(args[0])
	outputFile := filepath.Clean(args[1])

	force, err := base.FlagBool(cmd, "force")
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	if err := fs.ShouldOverwrite(outputFile, force); err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	cfgSettings, err := base.LoadAndLog(cmd)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	algorithm, err := base.ResolveGenerateAlgorithm(cmd, outputFile, cfgSettings)
	if err != nil {
		return &base.ExitError{Code: 1, Err: fmt.Errorf("failed to resolve algorithm: %w", err)}
	}

	flatPaths, err := base.FlagBoolOrDefault(cmd, "flat-paths", cfgSettings.Generate.FlatPaths)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	dirPrefix := ""
	if !flatPaths {
		dirPrefix, err = walk.GetPrefixForFilesInChecksum(inputDir, outputFile)
		if err != nil {
			return &base.ExitError{Code: 1, Err: fmt.Errorf("failed to get prefix: %w", err)}
		}
	}

	followSymbolicLinks, err := base.FlagBoolOrDefault(cmd, "follow-symbolic-links", cfgSettings.Generate.FollowSymbolicLinks)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	sortPaths, err := base.FlagBoolOrDefault(cmd, "sort-paths", cfgSettings.Generate.SortPaths)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	skipHidden, err := base.FlagBoolOrDefault(cmd, "skip-hidden", cfgSettings.Generate.SkipHidden)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	excludeRules := parseExcludeRules(excludePaths)

	cfg := servicegenerate.GenerateConfig{
		InputDir:            inputDir,
		OutputFile:          outputFile,
		Algorithm:           algorithm,
		DirPrefix:           dirPrefix,
		FollowSymbolicLinks: followSymbolicLinks,
		SortPaths:           sortPaths,
		SkipHidden:          skipHidden,
		ExcludeMatcher:      exclude.NewMatcher(excludeRules),
		OnFileHashed: func(res result.GenerateResult) {
			commonFields := func(event *zerolog.Event, err error) *zerolog.Event {
				logger := event.
					Str("file", res.RelPath).
					Str("status", res.Status.String()).
					Str("hash", res.Hash).
					Str("size", bytesize.New(float64(res.ReadBytes)).String())

				if res.Note != "" {
					logger = logger.Str("note", res.Note)
				}

				if err != nil {
					logger = logger.Err(err)
				}

				return logger
			}

			switch res.Status {
			case result.GenSuccess:
				commonFields(log.Info(), nil).Msg("Hashed")
			case result.GenSkipped:
				commonFields(log.Warn(), nil).Msg("Skipped file")
			case result.GenFailed:
				commonFields(log.Error(), res.Err).Msg("Failed to hash file")
			default:
				panic("unknown generation status")
			}
		},
	}

	log.Info().
		Str("input_dir", inputDir).
		Str("output_file", outputFile).
		Str("algorithm", algorithm.String()).
		Str("dir_prefix", cfg.DirPrefix).
		Bool("follow_symbolic_links", cfg.FollowSymbolicLinks).
		Bool("sort_paths", cfg.SortPaths).
		Bool("skip_hidden", cfg.SkipHidden).
		Bool("flat_paths", flatPaths).
		Strs("exclude", exclude.RuleStrings(excludeRules)).
		Msg("Starting generation")

	result, err := servicegenerate.GenerateChecksums(ctx, cfg)
	if err != nil {
		// Cancellation is a user abort only when it is the chain's sole cause; a joined failure must surface
		// as an error (exit 1) instead of being masked as a cancel (exit 130).
		if errs.IsContextDone(err) {
			log.Warn().Msg("Generation canceled")
			return &base.ExitError{Code: 130, Err: context.Canceled, Silent: true}
		}

		return &base.ExitError{Code: 1, Err: fmt.Errorf("failed to generate checksums: %w", err)}
	}

	stats := result.Stats
	log.Info().
		Int("processed", stats.Processed).
		Int("skipped", stats.Skipped).
		Int("pending", stats.Pending()).
		Int("with_errors", stats.WithErrors).
		Int("total_files", stats.TotalFiles).
		Msg("Generation stats")

	log.Info().
		Str("output_file", outputFile).
		Msg("Generation completed")

	if stats.WithErrors > 0 {
		return &base.ExitError{
			Code: 1,
			Err:  fmt.Errorf("%d of %d files failed to hash", stats.WithErrors, stats.TotalFiles),
		}
	}

	return nil
}

// parseExcludeRules converts --exclude strings to canonical Rules; degenerate entries are
// logged and dropped so a slip like `--exclude ./` never silently nukes matching.
func parseExcludeRules(paths []string) []exclude.Rule {
	rules := make([]exclude.Rule, 0, len(paths))

	for _, p := range paths {
		rule, err := exclude.ParseRule(p)
		if err != nil {
			log.Warn().Str("exclude", p).Err(err).Msg("Ignoring invalid --exclude rule")
			continue
		}

		rules = append(rules, rule)
	}

	return rules
}

// NewCmd assembles the generate subcommand and its flags for the root command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate <input_dir> <checksum_file>",
		Short: "Generate checksum file recursively from directory",
		Long: strings.Trim(dedent.Dedent(`
			Generate checksum file recursively from directory.
			Algorithm is determined in this order: --algorithm flag, output file extension, generate.algorithm config setting.
			Settings generate.follow_symbolic_links, generate.sort_paths, generate.skip_hidden and generate.flat_paths are loaded from configuration file.
			CLI flags override their respective config settings when passed explicitly.

			Use --exclude to skip entries relative to <input_dir>.
			A rule matches a top-level entry by name — 'build' excludes 'build/x' but not 'build-tools/x'.
			A rule with a slash matches that exact relative path (e.g. 'build/out.o').
			A trailing '/' is optional — both 'build' and 'build/' exclude the same entry.
			Repeat the flag to exclude multiple paths.

			Use --skip-hidden to skip hidden files and directories (names starting with '.' on Unix,
			the hidden attribute on Windows).

			Use --flat-paths to strip the root directory name from paths in the checksum file.
			The checksum file should be saved inside the source directory when this flag is used.

			By default, generate refuses to overwrite an existing output file (exit code 1).
			Pass --force to overwrite it without prompting.

			Supported algorithms: .sfv (CRC32), .md4, .md5, .sha1, .sha256, .sha384, .sha512, .sha3-256, .sha3-384, .sha3-512, .blake3, .xxh3, .xxh128.

			Files that could not be hashed are failures; skipped files (hidden, user-excluded, or with a
			name the checksum format cannot represent) are not.`,
		), "\n") + "\n\n" + base.ExitCodesHint,
		Args: cobra.ExactArgs(2),
		RunE: runGenerate,
	}

	cmd.Flags().StringArray("exclude", nil, "exclude a tree entry by name or an exact relative path (repeatable; trailing '/' is optional)")
	cmd.Flags().String("algorithm", "", "hash algorithm with leading dot (e.g., .sha256, .md5, .sfv); overrides output extension detection and generate.algorithm config setting")
	cmd.Flags().Bool("force", false, "overwrite existing output file without prompting")

	cmd.Flags().Bool("follow-symbolic-links", false, "follow symbolic links when scanning directories (default from generate.follow_symbolic_links)")
	cmd.Flags().Bool("sort-paths", false, "sort paths before hashing (default from generate.sort_paths)")
	cmd.Flags().Bool("skip-hidden", false, "skip hidden files and directories (dotfiles on Unix, hidden attribute on Windows; default from generate.skip_hidden)")
	cmd.Flags().Bool("flat-paths", false, "strip root directory from paths; save checksum file inside source directory (default from generate.flat_paths)")

	return cmd
}
