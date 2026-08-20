package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultAddr(t *testing.T) {
	c := Default()
	if c.HTTPAddr != ":18090" {
		t.Fatalf("default addr = %q", c.HTTPAddr)
	}
	if c.ScadaReadingBuffer != 200 {
		t.Fatalf("default buffer = %d", c.ScadaReadingBuffer)
	}
}

func TestFromEnvOverrides(t *testing.T) {
	os.Setenv("GPC_HTTP_ADDR", "9999")
	os.Setenv("GPC_SCADA_BUFFER", "50")
	os.Setenv("GPC_NOTIFY_FAIL_RATE", "1.5") // should clamp to 1
	defer func() {
		os.Unsetenv("GPC_HTTP_ADDR")
		os.Unsetenv("GPC_SCADA_BUFFER")
		os.Unsetenv("GPC_NOTIFY_FAIL_RATE")
	}()
	c := FromEnv()
	if c.HTTPAddr != ":9999" {
		t.Fatalf("addr = %q, want :9999", c.HTTPAddr)
	}
	if c.ScadaReadingBuffer != 50 {
		t.Fatalf("buffer = %d", c.ScadaReadingBuffer)
	}
	if c.NotifyFailureRate != 1 {
		t.Fatalf("fail rate = %v, want 1 (clamped)", c.NotifyFailureRate)
	}
}

func TestFromEnvBadValuesUseDefaults(t *testing.T) {
	os.Setenv("GPC_SCADA_BUFFER", "not-a-number")
	os.Setenv("GPC_READ_TIMEOUT", "garbage")
	defer func() {
		os.Unsetenv("GPC_SCADA_BUFFER")
		os.Unsetenv("GPC_READ_TIMEOUT")
	}()
	c := FromEnv()
	if c.ScadaReadingBuffer != 200 {
		t.Fatalf("bad buffer should fall back to default, got %d", c.ScadaReadingBuffer)
	}
	if c.ReadTimeout != 15*time.Second {
		t.Fatalf("bad timeout should fall back, got %v", c.ReadTimeout)
	}
}
