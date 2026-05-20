package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ghduuep/pingly/internal/dto"
	"github.com/ghduuep/pingly/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetMonitorsByUserID(ctx context.Context, db *pgxpool.Pool, userID, limit, offset int, targetFilter, typeFilter string) ([]*models.Monitor, int64, error) {
	query := `
	SELECT id, user_id, target, type, config, interval, timeout, latency_threshold_ms, last_check_status, last_check_at, status_changed_at, created_at, COUNT(*) OVER() as total 
	FROM monitors
	WHERE user_id = $1 
	`

	args := []any{userID}
	argIdx := 2

	if targetFilter != "" {
		query += fmt.Sprintf(" AND target ILIKE $%d", argIdx)
		args = append(args, "%"+targetFilter+"%")
		argIdx++
	}

	if typeFilter != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, typeFilter)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var monitors []*models.Monitor
	var total int64

	for rows.Next() {
		var m models.Monitor

		err := rows.Scan(
			&m.ID, &m.UserID, &m.Target, &m.Type, &m.Config, &m.Interval, &m.Timeout, &m.LatencyThreshold, &m.LastCheckStatus, &m.LastCheckAt, &m.StatusChangedAt, &m.CreatedAt, &total,
		)
		if err != nil {
			return nil, 0, err
		}
		monitors = append(monitors, &m)
	}

	return monitors, total, nil
}

func GetAllMonitors(ctx context.Context, db *pgxpool.Pool) ([]*models.Monitor, error) {
	query := `SELECT id, user_id, target, type, config, interval, timeout, latency_threshold_ms, last_check_status, last_check_at, status_changed_at, created_at FROM monitors`
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	monitors, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[models.Monitor])
	if err != nil {
		return nil, err
	}
	return monitors, nil
}

func CreateMonitor(ctx context.Context, db *pgxpool.Pool, monitor *models.Monitor) error {
	query := `INSERT INTO monitors (user_id, target, type, config, interval, timeout, latency_threshold_ms) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	err := db.QueryRow(ctx, query, monitor.UserID, monitor.Target, monitor.Type, monitor.Config, monitor.Interval, monitor.Timeout, monitor.LatencyThreshold).Scan(&monitor.ID)
	if err != nil {
		return err
	}
	return nil
}

func UpdateMonitorStatus(ctx context.Context, db *pgxpool.Pool, monitorID int, status string) error {
	query := `UPDATE monitors SET last_check_status = $1, last_check_at = NOW(), status_changed_at = NOW() WHERE id = $2`
	_, err := db.Exec(ctx, query, status, monitorID)
	if err != nil {
		return err
	}
	return nil
}

func UpdateLastCheck(ctx context.Context, db *pgxpool.Pool, monitorID int) error {
	query := `UPDATE monitors SET last_check_at = NOW() WHERE id = $1`

	_, err := db.Exec(ctx, query, monitorID)
	return err
}

func UpdateMonitorConfig(ctx context.Context, db *pgxpool.Pool, monitorID int, newConfig []byte) error {
	query := `UPDATE monitors SET config = $1 WHERE id = $2`
	_, err := db.Exec(ctx, query, newConfig, monitorID)
	return err
}

func DeleteMonitor(ctx context.Context, db *pgxpool.Pool, monitorID int, userID int) error {
	query := `DELETE FROM monitors WHERE id = $1 AND user_id = $2`
	_, err := db.Exec(ctx, query, monitorID, userID)
	if err != nil {
		return err
	}
	return nil
}

func SetInitialDNSConfig(ctx context.Context, db *pgxpool.Pool, monitorID int, detectedValue string) error {
	query := `UPDATE monitors SET config = config || jsonb_build_object('expected_value', $1::text) WHERE id = $2
	AND (config->>'expected_value' IS NULL or config->>'expected_value' = '')`

	_, err := db.Exec(ctx, query, detectedValue, monitorID)
	return err
}

func GetMonitorByIDAndUser(ctx context.Context, db *pgxpool.Pool, monitorID int, userID int) (models.Monitor, error) {
	query := `SELECT * FROM monitors WHERE id = $1 AND user_id = $2`

	var monitor models.Monitor
	err := db.QueryRow(ctx, query, monitorID, userID).Scan(&monitor.ID, &monitor.UserID, &monitor.Target, &monitor.Type, &monitor.Config, &monitor.Interval, &monitor.Timeout, &monitor.LatencyThreshold, &monitor.LastCheckStatus, &monitor.LastCheckAt, &monitor.StatusChangedAt, &monitor.CreatedAt)
	if err != nil {
		return models.Monitor{}, err
	}

	return monitor, nil
}

func UpdateMonitor(ctx context.Context, db *pgxpool.Pool, monitorID int, userID int, req dto.UpdateMonitorRequest, interval, timeout *time.Duration) error {
	var setParts []string
	var args []any
	argID := 1

	if req.Target != nil {
		setParts = append(setParts, fmt.Sprintf("target = $%d", argID))
		args = append(args, *req.Target)
		argID++
	}

	if req.Interval != nil {
		setParts = append(setParts, fmt.Sprintf("interval = $%d", argID))
		args = append(args, interval)
		argID++
	}

	if req.Timeout != nil {
		setParts = append(setParts, fmt.Sprintf("timeout = $%d", argID))
		args = append(args, timeout)
		argID++
	}

	if req.Config != nil {
		setParts = append(setParts, fmt.Sprintf("config = $%d", argID))
		args = append(args, req.Config)
		argID++
	}

	if req.LatencyThreshold != nil {
		setParts = append(setParts, fmt.Sprintf("latency_threshold_ms = $%d", argID))
		args = append(args, req.LatencyThreshold)
		argID++
	}

	if len(setParts) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE monitors SET %s WHERE id = $%d AND user_id = $%d", strings.Join(setParts, ", "), argID, argID+1)

	args = append(args, monitorID, userID)

	tag, err := db.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("monitor not found or no permission")
	}

	return nil
}

func GetMonitorSummary(ctx context.Context, db *pgxpool.Pool, userID int) (dto.MonitorSummaryResponse, error) {
	query := `
	SELECT last_check_status, COUNT(*)
	FROM monitors
	WHERE user_id = $1
	GROUP BY last_check_status
	`

	rows, err := db.Query(ctx, query, userID)
	if err != nil {
		return dto.MonitorSummaryResponse{}, err
	}
	defer rows.Close()

	var summary dto.MonitorSummaryResponse
	var total int

	for rows.Next() {
		var status string
		var count int

		if err := rows.Scan(&status, &count); err != nil {
			return dto.MonitorSummaryResponse{}, err
		}

		total += count

		switch models.MonitorStatus(status) {
		case models.StatusUp:
			summary.Up = count
		case models.StatusDown:
			summary.Down = count
		case models.StatusDegraded:
			summary.Degraded = count
		}
	}
	summary.Total = total
	return summary, nil
}
