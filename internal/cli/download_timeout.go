package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	downloadTimeoutEnv     = "ZLIB_DOWNLOAD_TIMEOUT"
	defaultDownloadTimeout = 30 * time.Minute
)

func addDownloadTimeoutFlag(cmd *cobra.Command) {
	cmd.Flags().Duration("download-timeout", defaultDownloadTimeout,
		"Time limit for the selected book's lookup and download (0 disables). Falls back to $"+downloadTimeoutEnv+".")
}

func downloadTimeoutFlag(cmd *cobra.Command) (time.Duration, error) {
	timeout, err := cmd.Flags().GetDuration("download-timeout")
	if err != nil {
		return 0, err
	}
	if !cmd.Flags().Changed("download-timeout") {
		if raw := strings.TrimSpace(os.Getenv(downloadTimeoutEnv)); raw != "" {
			timeout, err = time.ParseDuration(raw)
			if err != nil {
				return 0, fmt.Errorf("invalid %s: %w", downloadTimeoutEnv, err)
			}
		}
	}
	if timeout < 0 {
		return 0, fmt.Errorf("download timeout must not be negative")
	}
	return timeout, nil
}

func downloadContext(cmd *cobra.Command) (context.Context, context.CancelFunc, error) {
	timeout, err := downloadTimeoutFlag(cmd)
	if err != nil {
		return nil, nil, err
	}
	if timeout == 0 {
		ctx, cancel := context.WithCancel(cmd.Context())
		return ctx, cancel, nil
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	return ctx, cancel, nil
}
