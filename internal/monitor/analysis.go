package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/ghduuep/pingly/internal/database"
	"github.com/ghduuep/pingly/internal/models"
	"github.com/redis/go-redis/v9"
)

func (m *MonitorManager) isConfirmedFailure(ctx context.Context, mon *models.Monitor, currentStatus models.MonitorStatus) bool {
	if currentStatus == models.StatusUp {
		key := fmt.Sprintf("monitor:%d:fails", mon.ID)
		m.redis.Del(ctx, key)
		return true
	}

	if mon.LastCheckStatus == models.StatusDown || mon.LastCheckStatus == models.StatusDegraded {
		return true
	}

	key := fmt.Sprintf("monitor:%d:fails", mon.ID)
	count, err := m.redis.Incr(ctx, key).Result()
	if err != nil {
		log.Printf("[ERROR] Redis error on flapping check: %v", err)
		return true
	}

	m.redis.Expire(ctx, key, mon.Interval*time.Duration(FlappingTTLMulti))

	if count < FailureThreshold {
		log.Printf("[INFO] Monitor %d flapping detected (%d/%d). Suppressing alert", mon.ID, count, FailureThreshold)
		return false
	}

	return true
}

func (m *MonitorManager) handleDNSLearning(ctx context.Context, mon *models.Monitor, res *models.CheckResult) {
	if mon.Type != models.TypeDNS || res.Status != models.StatusUp || res.ResultValue == "" {
		return
	}

	var config models.DNSConfig
	if err := json.Unmarshal(mon.Config, &config); err == nil {
		log.Printf("[INFO] Learning DNS value for monitor %d: %s", mon.ID, res.ResultValue)

		if err := database.SetInitialDNSConfig(ctx, m.db, mon.ID, res.ResultValue); err != nil {
			log.Printf("[ERROR] Failed to persist learned DNS config: %v", err)
		}

		config.ExpectedValue = res.ResultValue
		newJSON, _ := json.Marshal(config)
		mon.Config = newJSON
	}
}

func (m *MonitorManager) analyzePerformance(mon *models.Monitor, res *models.CheckResult) {
	if res.Status != models.StatusUp {
		return
	}

	if mon.LatencyThreshold > 0 && res.Latency > mon.LatencyThreshold {
		log.Printf("[WARN] Latency threshold exceeded for monitor %d (%dms > %dms)", mon.ID, res.Latency, mon.LatencyThreshold)
		res.Status = models.StatusDegraded
		res.Message = fmt.Sprintf("Low performance: %dms (Limit: %dms)", res.Latency, mon.LatencyThreshold)
		return
	}
}

func (m *MonitorManager) handleSSLAlerts(ctx context.Context, mon *models.Monitor, res *models.CheckResult) {
	if mon.Type != models.TypeHTTP || res.ResultValue == "" {
		return
	}

	var config models.HTTPConfig
	if err := json.Unmarshal(mon.Config, &config); err != nil || !config.CheckSSL {
		return
	}

	daysRemaining, err := strconv.Atoi(res.ResultValue)
	if err != nil {
		return
	}

	thresholds := config.SSLAlertDays
	if len(thresholds) == 0 {
		thresholds = []int{30, 14, 7}
	}

	sort.Slice(thresholds, func(i, j int) bool {
		return thresholds[i] > thresholds[j]
	})

	redisKey := fmt.Sprintf("monitor:%d:ssl_last_threshold", mon.ID)

	if len(thresholds) > 0 && daysRemaining > thresholds[0] {
		m.redis.Del(ctx, redisKey)
		return
	}

	lastThresholdStr, err := m.redis.Get(ctx, redisKey).Result()
	lastThreshold := 0
	if err == nil {
		lastThreshold, _ = strconv.Atoi(lastThresholdStr)
	} else if err != redis.Nil {
		log.Printf("[ERROR] Redis error on SSL check: %v", err)
		return
	}

	shouldAlert := false
	currentMatchedThreshold := 0

	for _, t := range thresholds {
		if daysRemaining <= t {
			if lastThreshold == 0 || lastThreshold > t {
				shouldAlert = true
				currentMatchedThreshold = t
				break
			}
		}
	}

	if shouldAlert {
		log.Printf("[INFO] Sending SSL periodic alert for monitor %d (Days: %d, Threshold: %d)", mon.ID, daysRemaining, currentMatchedThreshold)

		err := m.redis.Set(ctx, redisKey, currentMatchedThreshold, 60*24*time.Hour).Err()
		if err != nil {
			log.Printf("[ERROR] Failed to update redis key: %v", err)
		}

		channels, _ := database.GetEnabledUserChannels(ctx, m.db, mon.UserID)

		go m.dispatcher.SendAlert(channels, *mon, *res, nil)
	}
}
