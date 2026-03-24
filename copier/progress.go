package copier

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

type Progress struct {
	totalObjects int64
	totalBytes   int64
	doneObjects  atomic.Int64
	doneBytes    atomic.Int64
	startTime    time.Time
}

func NewProgress(totalObjects, totalBytes int64) *Progress {
	return &Progress{
		totalObjects: totalObjects,
		totalBytes:   totalBytes,
		startTime:    time.Now(),
	}
}

func (p *Progress) Add(bytes int64) {
	p.doneObjects.Add(1)
	p.doneBytes.Add(bytes)
}

func (p *Progress) Start(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.print()
			}
		}
	}()
}

func (p *Progress) Finish() {
	p.print()
	fmt.Println()
	elapsed := time.Since(p.startTime)
	fmt.Printf("Done: %d objects, %s copied in %s\n",
		p.doneObjects.Load(),
		formatBytes(p.doneBytes.Load()),
		elapsed.Round(time.Millisecond),
	)
}

func (p *Progress) print() {
	done := p.doneBytes.Load()
	elapsed := time.Since(p.startTime).Seconds()

	speed := ""
	eta := ""
	if elapsed > 0 {
		bytesPerSec := float64(done) / elapsed
		speed = formatBytes(int64(bytesPerSec)) + "/s"

		if done > 0 && done < p.totalBytes {
			remaining := float64(p.totalBytes-done) / bytesPerSec
			eta = fmt.Sprintf("ETA: %s", time.Duration(remaining*float64(time.Second)).Round(time.Second))
		}
	}

	fmt.Printf("\r[%d/%d objects] [%s / %s] [%s] [%s]    ",
		p.doneObjects.Load(), p.totalObjects,
		formatBytes(done), formatBytes(p.totalBytes),
		speed, eta,
	)
}

func formatBytes(b int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
		GiB = 1024 * MiB
		TiB = 1024 * GiB
	)
	switch {
	case b >= TiB:
		return fmt.Sprintf("%.1f TiB", float64(b)/float64(TiB))
	case b >= GiB:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(GiB))
	case b >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(MiB))
	case b >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
