package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ghduuep/pingly/internal/models"
)

func checkHTTP(ctx context.Context, m models.Monitor) models.CheckResult {
	var config models.HTTPConfig
	if len(m.Config) > 0 {
		_ = json.Unmarshal(m.Config, &config)
	}

	client := http.Client{
		Timeout: m.Timeout,
	}

	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Target, nil)
	if err != nil {
		return models.CheckResult{
			MonitorID: m.ID,
			Status:    models.StatusDown,
			Latency:   0,
			Message:   "Failed to create request: " + err.Error(),
			CheckedAt: time.Now(),
		}
	}

	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		message := err.Error()

		if strings.Contains(message, "deadline exceeded") {
			message = "Connection Timeout"
		}

		return models.CheckResult{
			MonitorID: m.ID,
			Status:    models.StatusDown,
			Latency:   latency,
			Message:   message,
			CheckedAt: time.Now(),
		}
	}
	defer resp.Body.Close()

	message := resp.Status
	status := models.StatusDown
	if len(config.ExpectedCodes) > 0 {
		for _, code := range config.ExpectedCodes {
			if resp.StatusCode == code {
				status = models.StatusUp
				break
			}
		}
	} else {
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			status = models.StatusUp
		}
	}

	var resultValue string

	if config.CheckSSL {
		resultValue, status, message = checkSSL(resp, status, message, config)
	}

	return models.CheckResult{
		MonitorID:   m.ID,
		Status:      status,
		Latency:     latency,
		Message:     message,
		StatusCode:  resp.StatusCode,
		ResultValue: resultValue,
		CheckedAt:   time.Now(),
	}
}

func checkSSL(resp *http.Response, currentStatus models.MonitorStatus, currentMessage string, config models.HTTPConfig) (string, models.MonitorStatus, string) {
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return "", currentStatus, currentMessage
	}

	cert := resp.TLS.PeerCertificates[0]
	expiresIn := time.Until(cert.NotAfter)
	days := int(expiresIn.Hours() / 24)

	resultValue := strconv.Itoa(days)
	status := currentStatus
	message := currentMessage

	alertThreshold := 30
	if len(config.SSLAlertDays) > 0 {
		alertThreshold = 0
		for _, d := range config.SSLAlertDays {
			if d > alertThreshold {
				alertThreshold = d
			}
		}
	}

	if expiresIn < 0 {
		status = models.StatusDown
		message = fmt.Sprintf("CRITICAL: SSL certificate expired %s (%d days ago)", cert.NotAfter.Format("02/01/2006"), -days)
	} else if days <= alertThreshold {
		status = models.StatusDegraded
		message = fmt.Sprintf("SSL expires in %d days (%s)", days, cert.NotAfter.Format("02/01/2006"))
	}

	return resultValue, status, message
}
