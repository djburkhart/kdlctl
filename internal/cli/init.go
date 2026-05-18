package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/djburkhart/kdlctl/internal/templates"
)

func newInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create starter deployment files",
		RunE: func(cmd *cobra.Command, args []string) error {
			deployFile := "deploy.kdl"
			cloudBuildFile := "cloudbuild.yaml"
			exampleFile := "examples\\deploy.kdl"

			for _, path := range []string{deployFile, cloudBuildFile, exampleFile} {
				if err := ensureFileDoesNotExist(path, force); err != nil {
					return err
				}
			}

			if err := writeFile(deployFile, templates.ExampleDeployKDL); err != nil {
				return err
			}
			if err := writeFile(cloudBuildFile, templates.CloudBuildTemplate); err != nil {
				return err
			}
			if err := writeFile(exampleFile, templates.ExampleDeployKDL); err != nil {
				return err
			}

			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Created deploy.kdl, cloudbuild.yaml, and examples\\deploy.kdl")
			return err
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite starter files if they already exist")
	return cmd
}
