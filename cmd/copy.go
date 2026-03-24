package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/harukishima/copify/copier"
	"github.com/harukishima/copify/s3client"
	"github.com/spf13/cobra"
)

var copyFlags struct {
	configFile string

	srcEndpoint  string
	srcBucket    string
	srcAccessKey string
	srcSecretKey string
	srcRegion    string
	srcPrefix    string

	dstEndpoint  string
	dstBucket    string
	dstAccessKey string
	dstSecretKey string
	dstRegion    string
	dstPrefix    string

	concurrency int
	partSizeMiB int64
	dryRun      bool
}

var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy objects from source to destination S3-compatible storage",
	Example: `  # Using flags:
  copify copy \
    --src-endpoint https://s3.amazonaws.com --src-bucket my-source --src-access-key AKIA... --src-secret-key ... \
    --dst-endpoint https://play.min.io --dst-bucket my-dest --dst-access-key ... --dst-secret-key ...

  # Using config file:
  copify copy --config config.yaml

  # Config file with flag overrides:
  copify copy --config config.yaml --src-prefix logs/2024/ --dry-run`,
	RunE: runCopy,
}

func init() {
	f := copyCmd.Flags()

	f.StringVar(&copyFlags.configFile, "config", "", "Path to YAML config file")

	f.StringVar(&copyFlags.srcEndpoint, "src-endpoint", "", "Source S3 endpoint URL")
	f.StringVar(&copyFlags.srcBucket, "src-bucket", "", "Source bucket name")
	f.StringVar(&copyFlags.srcAccessKey, "src-access-key", "", "Source access key")
	f.StringVar(&copyFlags.srcSecretKey, "src-secret-key", "", "Source secret key")
	f.StringVar(&copyFlags.srcRegion, "src-region", "", "Source region")
	f.StringVar(&copyFlags.srcPrefix, "src-prefix", "", "Source key prefix")

	f.StringVar(&copyFlags.dstEndpoint, "dst-endpoint", "", "Destination S3 endpoint URL")
	f.StringVar(&copyFlags.dstBucket, "dst-bucket", "", "Destination bucket name")
	f.StringVar(&copyFlags.dstAccessKey, "dst-access-key", "", "Destination access key")
	f.StringVar(&copyFlags.dstSecretKey, "dst-secret-key", "", "Destination secret key")
	f.StringVar(&copyFlags.dstRegion, "dst-region", "", "Destination region")
	f.StringVar(&copyFlags.dstPrefix, "dst-prefix", "", "Destination key prefix")

	f.IntVar(&copyFlags.concurrency, "concurrency", 5, "Number of parallel object transfers")
	f.Int64Var(&copyFlags.partSizeMiB, "part-size", 10, "Multipart part size in MiB")
	f.BoolVar(&copyFlags.dryRun, "dry-run", false, "List what would be copied without copying")

	rootCmd.AddCommand(copyCmd)
}

func runCopy(cmd *cobra.Command, args []string) error {
	var fileCfg *FileConfig

	// Load config file if provided
	if copyFlags.configFile != "" {
		var err error
		fileCfg, err = loadConfig(copyFlags.configFile)
		if err != nil {
			return err
		}
	}

	// Resolve final values: flags override config file, config file provides defaults
	srcEndpoint := resolve(cmd, "src-endpoint", copyFlags.srcEndpoint, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Source.Endpoint }))
	srcBucket := resolve(cmd, "src-bucket", copyFlags.srcBucket, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Source.Bucket }))
	srcAccessKey := resolve(cmd, "src-access-key", copyFlags.srcAccessKey, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Source.AccessKey }))
	srcSecretKey := resolve(cmd, "src-secret-key", copyFlags.srcSecretKey, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Source.SecretKey }))
	srcRegion := resolveWithDefault(cmd, "src-region", copyFlags.srcRegion, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Source.Region }), "us-east-1")
	srcPrefix := resolve(cmd, "src-prefix", copyFlags.srcPrefix, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Source.Prefix }))

	dstEndpoint := resolve(cmd, "dst-endpoint", copyFlags.dstEndpoint, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Destination.Endpoint }))
	dstBucket := resolve(cmd, "dst-bucket", copyFlags.dstBucket, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Destination.Bucket }))
	dstAccessKey := resolve(cmd, "dst-access-key", copyFlags.dstAccessKey, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Destination.AccessKey }))
	dstSecretKey := resolve(cmd, "dst-secret-key", copyFlags.dstSecretKey, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Destination.SecretKey }))
	dstRegion := resolveWithDefault(cmd, "dst-region", copyFlags.dstRegion, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Destination.Region }), "us-east-1")
	dstPrefix := resolve(cmd, "dst-prefix", copyFlags.dstPrefix, getOrEmpty(fileCfg, func(c *FileConfig) string { return c.Destination.Prefix }))

	concurrency := copyFlags.concurrency
	partSizeMiB := copyFlags.partSizeMiB
	if fileCfg != nil {
		if !cmd.Flags().Changed("concurrency") && fileCfg.Concurrency > 0 {
			concurrency = fileCfg.Concurrency
		}
		if !cmd.Flags().Changed("part-size") && fileCfg.PartSizeMiB > 0 {
			partSizeMiB = fileCfg.PartSizeMiB
		}
	}

	// Validate required fields
	for _, check := range []struct {
		name, value string
	}{
		{"src-endpoint", srcEndpoint}, {"src-bucket", srcBucket},
		{"src-access-key", srcAccessKey}, {"src-secret-key", srcSecretKey},
		{"dst-endpoint", dstEndpoint}, {"dst-bucket", dstBucket},
		{"dst-access-key", dstAccessKey}, {"dst-secret-key", dstSecretKey},
	} {
		if check.value == "" {
			return fmt.Errorf("required flag or config field missing: %s", check.name)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srcClient, err := s3client.NewClient(s3client.Config{
		Endpoint:  srcEndpoint,
		Bucket:    srcBucket,
		AccessKey: srcAccessKey,
		SecretKey: srcSecretKey,
		Region:    srcRegion,
	})
	if err != nil {
		return err
	}

	dstClient, err := s3client.NewClient(s3client.Config{
		Endpoint:  dstEndpoint,
		Bucket:    dstBucket,
		AccessKey: dstAccessKey,
		SecretKey: dstSecretKey,
		Region:    dstRegion,
	})
	if err != nil {
		return err
	}

	return copier.Run(ctx, copier.Config{
		SrcClient:   srcClient,
		DstClient:   dstClient,
		SrcBucket:   srcBucket,
		DstBucket:   dstBucket,
		SrcPrefix:   srcPrefix,
		DstPrefix:   dstPrefix,
		Concurrency: concurrency,
		PartSizeMiB: partSizeMiB,
		DryRun:      copyFlags.dryRun,
	})
}

// resolve returns the flag value if explicitly set, otherwise the config file value.
func resolve(cmd *cobra.Command, flagName, flagValue, configValue string) string {
	if cmd.Flags().Changed(flagName) {
		return flagValue
	}
	if configValue != "" {
		return configValue
	}
	return flagValue
}

// resolveWithDefault is like resolve but applies a default when both flag and config are empty.
func resolveWithDefault(cmd *cobra.Command, flagName, flagValue, configValue, defaultValue string) string {
	v := resolve(cmd, flagName, flagValue, configValue)
	if v == "" {
		return defaultValue
	}
	return v
}

func getOrEmpty(cfg *FileConfig, fn func(*FileConfig) string) string {
	if cfg == nil {
		return ""
	}
	return fn(cfg)
}
