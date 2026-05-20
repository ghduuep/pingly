package monitor

import (
	"context"
	"fmt"
	"github.com/ghduuep/pingly/internal/models"
	"net"
	"strings"
	"time"
)

func checkPort(ctx context.Context, m models.Monitor) models.CheckResult {
	target := m.Target
	if !strings.Contains(target, ":") {
		target = fmt.Sprintf("%s:443", target)
	}

	start := time.Now()

	dialer := net.Dialer{
		Timeout: m.Timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", target)

	latency := time.Since(start).Milliseconds()

	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "timeout") {
			msg = "Timeout: Port unreachable or firewall blocking"
		} else if strings.Contains(msg, "refused") {
			msg = "Connection Refused: Server is up, but service is down"
		} else if strings.Contains(msg, "no such host") {
			msg = "DNS error: Domain not found"
		}

		return models.CheckResult{
			MonitorID: m.ID,
			Status:    models.StatusDown,
			Latency:   0,
			Message:   msg,
			CheckedAt: time.Now(),
		}
	}

	defer conn.Close()

	return models.CheckResult{
		MonitorID:   m.ID,
		Status:      models.StatusUp,
		Latency:     latency,
		ResultValue: fmt.Sprintf("%dms", latency),
		Message:     "Port accessible (TCP handshake)",
		CheckedAt:   time.Now(),
	}
}
