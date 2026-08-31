package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Hyaxia/blogwatcher/internal/scanner"
	"github.com/spf13/cobra"
)

const (
	storeDescriptionsEnv   = "BLOGWATCHER_STORE_DESCRIPTIONS"
	storeKeywordsEnv       = "BLOGWATCHER_STORE_KEYWORDS"
	descriptionMaxCharsEnv = "BLOGWATCHER_DESCRIPTION_MAX_CHARS"
	defaultDescriptionMax  = 1000
)

func resolveScanOptions(cmd *cobra.Command, storeDescriptions bool, storeKeywords bool, descriptionMaxChars int) (scanner.Options, error) {
	resolvedDescriptions, err := resolveBoolSetting(
		cmd, "store-descriptions", storeDescriptions, storeDescriptionsEnv, true,
	)
	if err != nil {
		return scanner.Options{}, err
	}
	resolvedKeywords, err := resolveBoolSetting(
		cmd, "store-keywords", storeKeywords, storeKeywordsEnv, false,
	)
	if err != nil {
		return scanner.Options{}, err
	}

	maxFromEnvironment, maxEnvironmentSet := nonEmptyEnvironment(descriptionMaxCharsEnv)
	maxFlagSet := cmd.Flags().Changed("description-max-chars")
	if !resolvedDescriptions && (maxFlagSet || maxEnvironmentSet) {
		return scanner.Options{}, fmt.Errorf("description maximum requires description storage to be enabled")
	}

	resolvedMaximum := defaultDescriptionMax
	if resolvedDescriptions {
		switch {
		case maxFlagSet:
			resolvedMaximum = descriptionMaxChars
		case maxEnvironmentSet:
			resolvedMaximum, err = strconv.Atoi(maxFromEnvironment)
			if err != nil {
				return scanner.Options{}, fmt.Errorf("invalid %s value %q: must be an integer", descriptionMaxCharsEnv, maxFromEnvironment)
			}
		}
		if resolvedMaximum < 0 {
			return scanner.Options{}, fmt.Errorf("description maximum must be zero or greater")
		}
	}

	return scanner.Options{
		StoreDescriptions:   resolvedDescriptions,
		StoreKeywords:       resolvedKeywords,
		DescriptionMaxChars: resolvedMaximum,
	}, nil
}

func resolveBoolSetting(
	cmd *cobra.Command,
	flagName string,
	flagValue bool,
	environmentName string,
	defaultValue bool,
) (bool, error) {
	if cmd.Flags().Changed(flagName) {
		return flagValue, nil
	}
	raw, ok := nonEmptyEnvironment(environmentName)
	if !ok {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s value %q: must be true or false", environmentName, raw)
	}
	return parsed, nil
}

func nonEmptyEnvironment(name string) (string, bool) {
	value, ok := os.LookupEnv(name)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}
