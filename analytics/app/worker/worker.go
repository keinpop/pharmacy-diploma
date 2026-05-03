package worker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"pharmacy/analytics/app/metrics"
	"pharmacy/analytics/domain"
	usecase "pharmacy/analytics/domain/use_case"
)

// Worker polls for pending reports and processes them.
type Worker struct {
	reportRepo   usecase.ReportRepository
	uc           ReportProcessor
	logger       *zap.Logger
	pollInterval time.Duration
}

// ReportProcessor is the interface the worker uses to process a report.
type ReportProcessor interface {
	ProcessReport(ctx context.Context, report *domain.Report) error
}

// NewWorker creates a new Worker.
func NewWorker(
	reportRepo usecase.ReportRepository,
	uc ReportProcessor,
	logger *zap.Logger,
	pollInterval time.Duration,
) *Worker {
	return &Worker{
		reportRepo:   reportRepo,
		uc:           uc,
		logger:       logger,
		pollInterval: pollInterval,
	}
}

// Run starts the worker polling loop. It stops when ctx is canceled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopped")
			return
		case <-ticker.C:
			w.processPending(ctx)
		}
	}
}

// processPending fetches all pending reports and processes each in a separate goroutine.
func (w *Worker) processPending(ctx context.Context) {
	reports, err := w.reportRepo.ListPending(ctx)
	if err != nil {
		w.logger.Warn("list pending reports failed", zap.Error(err))
		return
	}

	for _, report := range reports {
		report := report // capture loop variable
		go func() {
			if err := w.reportRepo.UpdateStatus(ctx, report.ID, domain.StatusProcessing); err != nil {
				w.logger.Warn("update status to processing failed",
					zap.String("report_id", report.ID),
					zap.Error(err),
				)
				return
			}

			if err := w.uc.ProcessReport(ctx, report); err != nil {
				w.logger.Warn("process report failed",
					zap.String("report_id", report.ID),
					zap.String("type", string(report.Type)),
					zap.Error(err),
				)
				if saveErr := w.reportRepo.SaveError(ctx, report.ID, err.Error()); saveErr != nil {
					w.logger.Warn("save error failed",
						zap.String("report_id", report.ID),
						zap.Error(saveErr),
					)
				}
				metrics.ReportsCompleted.WithLabelValues(string(report.Type), "failed").Inc()
				return
			}

			metrics.ReportsCompleted.WithLabelValues(string(report.Type), "success").Inc()
			w.logger.Info("report processed",
				zap.String("report_id", report.ID),
				zap.String("type", string(report.Type)),
			)
		}()
	}
}
