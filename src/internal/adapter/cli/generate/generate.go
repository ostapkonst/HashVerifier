// Package generate implements the `hashverifier generate` subcommand.
package generate

import (
	"context"
	"errors"
	"fmt"
	"github.com/inhies/go-bytesize"
	"github.com/lithammer/dedent"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	"github.com/ostapkonst/HashVerifier/internal/domain/exclude"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
	servicegenerate "github.com/ostapkonst/HashVerifier/internal/service/generate"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"path/filepath"
	"strings"
)

func runGenerate(cmd *cobra.Command, args []string) error {
	excludePaths, err := cmd.Flags().GetStringArray("exclude")
	if err != nil {
		err = fmt.Errorf("internal error reading --exclude flag: %w", err)
		return &base.ExitError{Code: 1, Err: err}
	}

	return base.RunWithShutdown(cmd, func(ctx context.Context) error {
		return execGenerate(ctx, cmd, args, excludePaths)
	})
}

func execGenerate(ctx context.Context, cmd *cobra.Command, args []string, excludePaths []string) error {
	inputDir := filepath.Clean(args[0])
	outputFile := filepath.Clean(args[1])

	force, _ := cmd.Flags().GetBool("force")
	if err := fs.ShouldOverwrite(outputFile, force); err != nil {
		if errors.Is(err, fs.ErrRefuseOverwrite) {
			return &base.ExitError{
				Code: 1,
				Err:  fmt.Errorf("refusing to overwrite existing file: %s (use --force)", outputFile),
			}
		}

		return &base.ExitError{Code: 1, Err: fmt.Errorf("invalid output file: %w", err)}
	}

	cfgSettings := base.LoadAndLog(cmd)

	algorithm, err := base.ResolveGenerateAlgorithm(cmd, outputFile, cfgSettings)
	if err != nil {
		return &base.ExitError{Code: 1, Err: fmt.Errorf("failed to resolve algorithm: %w", err)}
	}

	flatPaths := base.FlagBoolOrDefault(cmd, "flat-paths", cfgSettings.Generate.FlatPaths)

	dirPrefix := ""
	if !flatPaths {
		dirPrefix, err = walk.GetPrefixForFilesInChecksum(inputDir, outputFile)
		if err != nil {
			return &base.ExitError{Code: 1, Err: fmt.Errorf("failed to get prefix: %w", err)}
		}
	}

	cfg := servicegenerate.GenerateConfig{
		InputDir:            inputDir,
		OutputFile:          outputFile,
		Algorithm:           algorithm,
		DirPrefix:           dirPrefix,
		FollowSymbolicLinks: base.FlagBoolOrDefault(cmd, "follow-symbolic-links", cfgSettings.Generate.FollowSymbolicLinks),
		SortPaths:           base.FlagBoolOrDefault(cmd, "sort-paths", cfgSettings.Generate.SortPaths),
		FlatPaths:           flatPaths,
		ExcludeMatcher:      exclude.NewMatcher(excludePaths),
		OnFileHashed: func(res result.GenerateResult) {
			commonFields := func(event *zerolog.Event, err error) *zerolog.Event {
				logger := event.
					Str("file", res.RelPath).
					Str("status", res.Status.String()).
					Str("hash", res.Hash).
					Str("size", bytesize.New(float64(res.ReadBytes)).String())

				if err != nil {
					logger = logger.Err(err)
				}

				return logger
			}

			switch res.Status {
			case result.GenSuccess:
				commonFields(log.Info(), nil).Msg("Hashed")
			case result.GenSkipped:
				commonFields(log.Warn(), res.Err).Msg("Skipped file")
			default:
				commonFields(log.Error(), res.Err).Msg("Failed to hash file")
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
		Bool("flat_paths", cfg.FlatPaths).
		Strs("exclude", excludePaths).
		Msg("Starting generation")

	result, err := servicegenerate.GenerateChecksums(ctx, cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Warn().Msg("Checksum generation canceled")
			return &base.ExitError{Code: 130, Err: context.Canceled}
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

// NewCmd returns the cobra command for `hashverifier generate <input_dir> <checksum_file>`.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate <input_dir> <checksum_file>",
		Short: "Generate checksum file recursively from directory",
		Long: strings.Trim(dedent.Dedent(`
			Generate checksum file recursively from directory.
			Algorithm is determined in this order: --algorithm flag, output file extension, generate.algorithm config setting.
			Settings generate.follow_symbolic_links, generate.sort_paths and generate.flat_paths are loaded from configuration file.
			CLI flags override their respective config settings when passed explicitly.

			Use --exclude to skip specific files or directories relative to <input_dir>.
			Directories should end with a path separator (e.g. --exclude 'build/').
			Repeat the flag to exclude multiple paths.

			Use --flat-paths to strip the root directory name from paths in the checksum file.
			The checksum file should be saved inside the source directory when this flag is used.

			Supported algorithms: .sfv (CRC32), .md4, .md5, .sha1, .sha256, .sha384, .sha512, .sha3-256, .sha3-384, .sha3-512, .blake3, .xxh3, .xxh128.`,
		), "\n"),
		Args: cobra.ExactArgs(2),
		RunE: runGenerate,
	}

	cmd.Flags().StringArray("exclude", nil, "exclude relative path from generation (repeatable; append '/' for directories)")
	cmd.Flags().String("algorithm", "", "hash algorithm with leading dot (e.g., .sha256, .md5, .sfv); overrides output extension detection and generate.algorithm config setting")
	cmd.Flags().Bool("force", false, "overwrite existing output file without prompting")

	base.AddOptBoolFlag(cmd, "follow-symbolic-links", false, "follow symbolic links when scanning directories (default from generate.follow_symbolic_links)")
	base.AddOptBoolFlag(cmd, "sort-paths", false, "sort paths before hashing (default from generate.sort_paths)")
	base.AddOptBoolFlag(cmd, "flat-paths", false, "strip root directory from paths; save checksum file inside source directory (default from generate.flat_paths)")

	return cmd
}
