package monitor

import (
	"context"
	"github.com/ghduuep/pingly/internal/database"
	"github.com/ghduuep/pingly/internal/models"
	"log"
	"time"
)

func (m *MonitorManager) startMonitor(ctx context.Context, mon models.Monitor) {
	monCtx, cancel := context.WithCancel(ctx)

	m.activeMonitors[mon.ID] = &activeMonitor{
		cancel: cancel,
		config: mon,
	}

	go m.runWorker(monCtx, mon)
	log.Printf("[INFO] Started monitoring for %s (%s)", mon.Target, mon.Type)
}

func (m *MonitorManager) runWorker(ctx context.Context, mon models.Monitor) {
	log.Printf("[INFO] Perfoming initial check for monitor %d", mon.ID)
	initialStatus, useFastInterval := m.processCheck(ctx, &mon)

	initialDelay := mon.Interval
	if useFastInterval || initialStatus == models.StatusDown || initialStatus == models.StatusDegraded {
		initialDelay = DownCheckInterval
	}

	timer := time.NewTimer(initialDelay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			_, useFastInterval := m.processCheck(ctx, &mon)

			nextInterval := mon.Interval
			if useFastInterval {
				nextInterval = DownCheckInterval
			}

			timer.Reset(nextInterval)
		}
	}
}

func (m *MonitorManager) processCheck(ctx context.Context, mon *models.Monitor) (models.MonitorStatus, bool) {
	result := performCheck(ctx, *mon)

	m.handleDNSLearning(ctx, mon, &result)

	m.handleSSLAlerts(ctx, mon, &result)

	shouldProceed := m.isConfirmedFailure(ctx, mon, result.Status)
	if !shouldProceed {
		_ = database.UpdateLastCheck(ctx, m.db, mon.ID)
		return mon.LastCheckStatus, true
	}

	m.analyzePerformance(mon, &result)

	if err := database.CreateCheckResult(ctx, m.db, &result); err != nil {
		log.Printf("[ERROR] Failed to save check result for monitor %d", mon.ID)
	}

	if result.Status != mon.LastCheckStatus && result.Status != models.StatusUnknown {
		m.handleStateChange(ctx, mon, result)
	} else {
		_ = database.UpdateLastCheck(ctx, m.db, mon.ID)
	}

	isDownOrDegraded := result.Status == models.StatusDown || result.Status == models.StatusDegraded

	return result.Status, isDownOrDegraded
}

func performCheck(ctx context.Context, m models.Monitor) models.CheckResult {
	switch m.Type {
	case models.TypeHTTP:
		return checkHTTP(ctx, m)
	case models.TypeDNS:
		return checkDNS(ctx, m)
	case models.TypePort:
		return checkPort(ctx, m)
	default:
		return models.CheckResult{
			MonitorID: m.ID,
			Status:    models.StatusDown,
			Message:   "Unknown monitor type",
			CheckedAt: time.Now(),
		}
	}
}
