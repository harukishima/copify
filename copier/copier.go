package copier

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	SrcClient   *minio.Client
	DstClient   *minio.Client
	SrcBucket   string
	DstBucket   string
	SrcPrefix   string
	DstPrefix   string
	Concurrency int
	PartSizeMiB int64
	DryRun      bool
}

type object struct {
	key  string
	size int64
}

func Run(ctx context.Context, cfg Config) error {
	// Step 1: List all source objects
	fmt.Printf("Listing objects in %s/%s ...\n", cfg.SrcBucket, cfg.SrcPrefix)

	var objects []object
	var totalBytes int64

	objectCh := cfg.SrcClient.ListObjects(ctx, cfg.SrcBucket, minio.ListObjectsOptions{
		Prefix:    cfg.SrcPrefix,
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			return fmt.Errorf("listing objects: %w", obj.Err)
		}
		objects = append(objects, object{key: obj.Key, size: obj.Size})
		totalBytes += obj.Size
	}

	fmt.Printf("Found %d objects (%s)\n", len(objects), formatBytes(totalBytes))

	if len(objects) == 0 {
		fmt.Println("Nothing to copy.")
		return nil
	}

	// Step 2: Dry run
	if cfg.DryRun {
		for _, obj := range objects {
			dstKey := rewriteKey(obj.key, cfg.SrcPrefix, cfg.DstPrefix)
			fmt.Printf("  %s -> %s (%s)\n", obj.key, dstKey, formatBytes(obj.size))
		}
		return nil
	}

	// Step 3: Copy with bounded concurrency
	progress := NewProgress(int64(len(objects)), totalBytes)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	progress.Start(ctx)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.Concurrency)

	partSize := cfg.PartSizeMiB * 1024 * 1024

	for _, obj := range objects {
		obj := obj
		g.Go(func() error {
			if err := copyObject(gctx, cfg, obj, partSize); err != nil {
				return err
			}
			progress.Add(obj.size)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		cancel()
		fmt.Println()
		return fmt.Errorf("copy failed: %w", err)
	}

	cancel()
	progress.Finish()
	return nil
}

func copyObject(ctx context.Context, cfg Config, obj object, partSize int64) error {
	dstKey := rewriteKey(obj.key, cfg.SrcPrefix, cfg.DstPrefix)

	src, err := cfg.SrcClient.GetObject(ctx, cfg.SrcBucket, obj.key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("get %s: %w", obj.key, err)
	}
	defer src.Close()

	_, err = cfg.DstClient.PutObject(ctx, cfg.DstBucket, dstKey, src, obj.size, minio.PutObjectOptions{
		PartSize: uint64(partSize),
	})
	if err != nil {
		return fmt.Errorf("put %s -> %s: %w", obj.key, dstKey, err)
	}

	return nil
}

func rewriteKey(key, srcPrefix, dstPrefix string) string {
	relative := strings.TrimPrefix(key, srcPrefix)
	return dstPrefix + relative
}
