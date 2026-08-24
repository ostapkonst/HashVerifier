package base

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
)

// ResolveGenerateAlgorithm determines the algorithm for the generate command: flag → output extension → generate.algorithm config setting.
func ResolveGenerateAlgorithm(cmd *cobra.Command, outputFile string, cfg *settings.Settings) (algorithm.Algorithm, error) {
	if cmd.Flags().Changed("algorithm") {
		raw, err := cmd.Flags().GetString("algorithm")
		if err != nil {
			return algorithm.Unknown, err
		}

		if raw != "" {
			return algorithm.AlgorithmFromExtension(raw)
		}
	}

	if algo, err := algorithm.AlgorithmFromExtension(outputFile); err == nil {
		return algo, nil
	}

	return algorithm.AlgorithmFromExtension(cfg.Generate.Algorithm)
}

// ParseAlgorithms validates and deduplicates a --algorithms list, returning the parsed algorithms plus their string forms for logging.
func ParseAlgorithms(rawList []string) ([]algorithm.Algorithm, []string, error) {
	seen := make(map[algorithm.Algorithm]struct{}, len(rawList))
	algos := make([]algorithm.Algorithm, 0, len(rawList))
	algoStrings := make([]string, 0, len(rawList))

	for _, raw := range rawList {
		algo, err := algorithm.AlgorithmFromExtension(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("unsupported algorithm %q: %w", raw, err)
		}

		if _, ok := seen[algo]; ok {
			continue
		}

		seen[algo] = struct{}{}
		algos = append(algos, algo)
		algoStrings = append(algoStrings, algo.String())
	}

	return algos, algoStrings, nil
}
