package jobs

import (
	"context"
	"log/slog"
	"time"
)

type Application interface {
	ProcessDueTrackers(ctx context.Context) error
}

type Worker struct {
	app    Application
	logger *slog.Logger
}

func NewWorker(app Application, logger *slog.Logger) *Worker {
	return &Worker{app: app, logger: logger}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.app.ProcessDueTrackers(ctx); err != nil {
				w.logger.Warn("process due trackers", "error", err)
			}
		}
	}
}
