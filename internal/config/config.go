package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the control service.
// Every field has a safe default so the service can boot without any env vars.
type Config struct {
	// HTTPAddr is the listen address for the HTTP server.
	HTTPAddr string
	// SiteCode identifies the operating company / control center site.
	SiteCode string
	// SiteName is the human-readable name shown on the dashboard.
	SiteName string
	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration
	// WriteTimeout is the maximum duration for writing the response.
	WriteTimeout time.Duration
	// ShutdownTimeout is the grace period for in-flight requests on shutdown.
	ShutdownTimeout time.Duration

	// ScadaReadingBuffer is the number of recent readings kept per point.
	ScadaReadingBuffer int
	// ScadaDefaultHighThreshold is the default pressure high threshold (MPa).
	ScadaDefaultHighThreshold float64
	// ScadaDefaultLowThreshold is the default pressure low threshold (MPa).
	ScadaDefaultLowThreshold float64
	// ScadaRateLimitSec is the maximum allowed rate of change per second for a reading.
	ScadaRateLimit float64

	// MeteringPressureBase is the standard reference pressure (MPa) for volume correction.
	MeteringPressureBase float64
	// MeteringTemperatureBase is the standard reference temperature (°C) for volume correction.
	MeteringTemperatureBase float64

	// PermitWindowMaxHours caps how far in the future a permit window may start.
	PermitWindowMaxHours int

	// NotifyFailureRate simulates a delivery failure rate in (0,1] for PushBatch.
	NotifyFailureRate float64
	// NotifyMaxAttempts bounds the number of delivery retries.
	NotifyMaxAttempts int
	// NotifyBackoff is the base delay between retries.
	NotifyBackoff time.Duration

	// LeakDetectDropRateThreshold is the pressure drop-rate threshold (MPa/min) for leak detection.
	LeakDetectDropRateThreshold float64
	// LeakDetectImbalanceThreshold is the mass-balance deviation threshold (fraction).
	LeakDetectImbalanceThreshold float64
	// LeakDetectWindowMinutes is the rolling window used to aggregate readings.
	LeakDetectWindowMinutes int

	// AuditRetention limits the number of audit entries kept in memory.
	AuditRetention int
}

// Default returns a configuration with sensible defaults for a long-distance
// natural-gas pipeline control center.
func Default() Config {
	return Config{
		HTTPAddr:                     ":18090",
		SiteCode:                     "GPL-CC-01",
		SiteName:                     "Northern Gas Pipeline Control Center",
		ReadTimeout:                  15 * time.Second,
		WriteTimeout:                 20 * time.Second,
		ShutdownTimeout:              10 * time.Second,
		ScadaReadingBuffer:           200,
		ScadaDefaultHighThreshold:    10.0,
		ScadaDefaultLowThreshold:     1.5,
		ScadaRateLimit:               0.5,
		MeteringPressureBase:         0.101325,
		MeteringTemperatureBase:      20.0,
		PermitWindowMaxHours:         720,
		NotifyFailureRate:            0.0,
		NotifyMaxAttempts:            5,
		NotifyBackoff:                30 * time.Second,
		LeakDetectDropRateThreshold:  0.15,
		LeakDetectImbalanceThreshold: 0.05,
		LeakDetectWindowMinutes:      30,
		AuditRetention:               5000,
	}
}

// FromEnv loads configuration from environment variables, falling back to
// defaults for anything missing or malformed. Unknown variables are ignored.
func FromEnv() Config {
	c := Default()

	if v := os.Getenv("GPC_HTTP_ADDR"); v != "" {
		c.HTTPAddr = v
	}
	if v := os.Getenv("GPC_SITE_CODE"); v != "" {
		c.SiteCode = v
	}
	if v := os.Getenv("GPC_SITE_NAME"); v != "" {
		c.SiteName = v
	}
	c.ReadTimeout = envDuration("GPC_READ_TIMEOUT", c.ReadTimeout)
	c.WriteTimeout = envDuration("GPC_WRITE_TIMEOUT", c.WriteTimeout)
	c.ShutdownTimeout = envDuration("GPC_SHUTDOWN_TIMEOUT", c.ShutdownTimeout)

	c.ScadaReadingBuffer = envInt("GPC_SCADA_BUFFER", c.ScadaReadingBuffer)
	c.ScadaDefaultHighThreshold = envFloat("GPC_SCADA_HIGH", c.ScadaDefaultHighThreshold)
	c.ScadaDefaultLowThreshold = envFloat("GPC_SCADA_LOW", c.ScadaDefaultLowThreshold)
	c.ScadaRateLimit = envFloat("GPC_SCADA_RATE", c.ScadaRateLimit)

	c.MeteringPressureBase = envFloat("GPC_METER_PBASE", c.MeteringPressureBase)
	c.MeteringTemperatureBase = envFloat("GPC_METER_TBASE", c.MeteringTemperatureBase)

	c.PermitWindowMaxHours = envInt("GPC_PERMIT_WINDOW_HOURS", c.PermitWindowMaxHours)

	c.NotifyFailureRate = clampFloat(envFloat("GPC_NOTIFY_FAIL_RATE", c.NotifyFailureRate), 0, 1)
	c.NotifyMaxAttempts = envInt("GPC_NOTIFY_MAX_ATTEMPTS", c.NotifyMaxAttempts)
	c.NotifyBackoff = envDuration("GPC_NOTIFY_BACKOFF", c.NotifyBackoff)

	c.LeakDetectDropRateThreshold = envFloat("GPC_LEAK_DROP", c.LeakDetectDropRateThreshold)
	c.LeakDetectImbalanceThreshold = envFloat("GPC_LEAK_IMBALANCE", c.LeakDetectImbalanceThreshold)
	c.LeakDetectWindowMinutes = envInt("GPC_LEAK_WINDOW_MIN", c.LeakDetectWindowMinutes)

	c.AuditRetention = envInt("GPC_AUDIT_RETENTION", c.AuditRetention)

	// Normalize the listen address: a bare port or number becomes ":port".
	if strings.HasPrefix(c.HTTPAddr, ":") || strings.Contains(c.HTTPAddr, ":") {
		// already an address
	} else if _, err := strconv.Atoi(c.HTTPAddr); err == nil {
		c.HTTPAddr = ":" + c.HTTPAddr
	}
	return c
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
