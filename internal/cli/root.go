package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	defaultConfigFile = "deploy.kdl"
	defaultNATSURL    = "nats://127.0.0.1:4222"
)

var (
	exitFunc                     = os.Exit
	stderrWriter       io.Writer = os.Stderr
	rootCommandFactory           = newRootCmd
)

func Execute() {
	if err := rootCommandFactory().Execute(); err != nil {
		fmt.Fprintln(stderrWriter, err)
		exitFunc(1)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "kdlctl",
		Short:         "Deploy Google Cloud services from KDL configuration",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	viper.SetEnvPrefix("KDLCTL")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	flags := rootCmd.PersistentFlags()
	flags.String("file", defaultConfigFile, "Path to the deploy.kdl file")
	flags.String("project-id", "", "Override the GCP project ID from configuration")
	flags.String("region", "", "Override the GCP region from configuration")
	flags.String("nats-url", defaultNATSURL, "NATS server URL")
	flags.String("github-token", "", "GitHub token for repository operations")

	mustBindFlag("file", flags.Lookup("file"))
	mustBindFlag("project-id", flags.Lookup("project-id"))
	mustBindFlag("region", flags.Lookup("region"))
	mustBindFlag("nats-url", flags.Lookup("nats-url"))
	mustBindFlag("github-token", flags.Lookup("github-token"))

	rootCmd.AddCommand(
		newInitCmd(),
		newValidateCmd(),
		newPlanCmd(),
		newDeployCmd(),
		newStatusCmd(),
		newRollbackCmd(),
		newNATSCmd(),
	)

	return rootCmd
}

func mustBindFlag(key string, flag *pflag.Flag) {
	if err := viper.BindPFlag(key, flag); err != nil {
		panic(err)
	}
}
