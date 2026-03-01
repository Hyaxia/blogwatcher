package cli

import (
	"fmt"
	"os"

	"github.com/Hyaxia/blogwatcher/internal/scanner"
	"github.com/Hyaxia/blogwatcher/internal/version"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	var userAgent string

	rootCmd := &cobra.Command{
		Use:           "blogwatcher",
		Short:         "BlogWatcher - Track blog articles and detect new posts.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			scanner.SetHTTPUserAgent(userAgent)
		},
	}
	rootCmd.Version = version.Version
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.PersistentFlags().StringVar(&userAgent, "user-agent", "", "Custom HTTP User-Agent for feed/scrape requests")
	rootCmd.AddCommand(newAddCommand())
	rootCmd.AddCommand(newRemoveCommand())
	rootCmd.AddCommand(newBlogsCommand())
	rootCmd.AddCommand(newScanCommand())
	rootCmd.AddCommand(newArticlesCommand())
	rootCmd.AddCommand(newReadCommand())
	rootCmd.AddCommand(newReadAllCommand())
	rootCmd.AddCommand(newUnreadCommand())
	return rootCmd
}

func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		if !isPrinted(err) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
