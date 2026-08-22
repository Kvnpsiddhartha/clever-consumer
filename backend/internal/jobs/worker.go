package jobs

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type Application interface {
	ProcessDueTrackers(ctx context.Context) error
}

type Worker struct {
	app    Application
	logger *zap.SugaredLogger
}

func NewWorker(app Application, logger *zap.SugaredLogger) *Worker {
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
				w.logger.Warnw("process due trackers", "error", err)
			}
		}
	}
}
