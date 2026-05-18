package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/djburkhart/kdlctl/internal/config"
	"github.com/djburkhart/kdlctl/pkg/types"
)

func loadProjectConfig() (*types.ProjectConfig, error) {
	path := viper.GetString("file")
	cfg, err := config.LoadFile(path)
	if err != nil {
		return nil, err
	}

	if projectID := viper.GetString("project-id"); projectID != "" {
		cfg.ProjectID = projectID
	}
	if region := viper.GetString("region"); region != "" {
		cfg.Region = region
	}

	return cfg, nil
}

func ensureFileDoesNotExist(path string, force bool) error {
	if force {
		return nil
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; rerun with --force to overwrite", path)
	}

	return nil
}

func writeFile(path string, contents string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func writeJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
