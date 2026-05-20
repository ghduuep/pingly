package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID           int                   `json:"id" db:"id"`
	Username     string                `json:"username" db:"username"`
	Email        string                `json:"email" db:"email"`
	PasswordHash string                `json:"password_hash" db:"password_hash"`
	Channels     []NotificationChannel `json:"channels" db:"channels"`
	CreatedAt    time.Time             `json:"created_at" db:"created_at"`
}

type NotificationChannel struct {
	ID      int              `json:"id" db:"id"`
	UserID  int              `json:"user_id" db:"user_id"`
	Type    NotificationType `json:"type" db:"type"`
	Target  string           `json:"target" db:"target"`
	Enabled bool             `json:"enabled" db:"enabled"`
}

type NotificationType string

const (
	TypeEmail    NotificationType = "email"
	TypeSMS      NotificationType = "sms"
	TypeTelegram NotificationType = "telegram"
)

type MonitorType string

const (
	TypeHTTP MonitorType = "http"
	TypePort MonitorType = "port"
	TypeDNS  MonitorType = "dns"
)

type MonitorStatus string

const (
	StatusUp       MonitorStatus = "up"
	StatusDown     MonitorStatus = "down"
	StatusUnknown  MonitorStatus = "unknown"
	StatusDegraded MonitorStatus = "degraded"
)

type Monitor struct {
	ID               int             `json:"id" db:"id"`
	UserID           int             `json:"user_id" db:"user_id"`
	Target           string          `json:"target" db:"target"`
	Type             MonitorType     `json:"type" db:"type"`
	Config           json.RawMessage `json:"config" db:"config" swaggertype:"string"`
	Interval         time.Duration   `json:"interval" db:"interval" swaggertype:"integer"`
	Timeout          time.Duration   `json:"timeout" db:"timeout" swaggertype:"integer"`
	LatencyThreshold int64           `json:"latency_threshold_ms" db:"latency_threshold_ms"`
	LastCheckStatus  MonitorStatus   `json:"last_check_status" db:"last_check_status"`
	LastCheckAt      *time.Time      `json:"last_check_at" db:"last_check_at"`
	StatusChangedAt  *time.Time      `json:"status_changed_at" db:"status_changed_at"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
}

type CheckResult struct {
	ID          int           `json:"id" db:"id"`
	MonitorID   int           `json:"monitor_id" db:"monitor_id"`
	Status      MonitorStatus `json:"status" db:"status"`
	Latency     int64         `json:"latency_ms,omitempty" db:"latency_ms"`
	StatusCode  int           `json:"status_code,omitempty" db:"status_code"`
	ResultValue string        `json:"result_value,omitempty" db:"result_value"`
	Message     string        `json:"message,omitempty" db:"message"`
	CheckedAt   time.Time     `json:"checked_at" db:"checked_at"`
}

type DNSConfig struct {
	RecordType    string `json:"record_type"`
	ExpectedValue string `json:"expected_value"`
	NameServer    string `json:"name_server,omitempty"`
}

type HTTPConfig struct {
	CheckSSL      bool  `json:"check_ssl"`
	SSLAlertDays  []int `json:"ssl_alert_days,omitempty"`
	ExpectedCodes []int `json:"expected_codes,omitempty"`
}

type Incident struct {
	ID         int            `json:"id" db:"id"`
	MonitorID  int            `json:"monitor_id" db:"monitor_id"`
	StartedAt  time.Time      `json:"started_at" db:"started_at"`
	ResolvedAt *time.Time     `json:"resolved_at" db:"resolved_at"`
	Duration   *time.Duration `json:"duration" db:"duration"`
	ErrorCause string         `json:"error_cause" db:"error_cause"`
}
