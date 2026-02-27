package monitor

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type ServiceState struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}

type ServicesMonitor struct {
	names   []string
	timeout time.Duration
}

func NewServicesMonitor(names []string, timeout time.Duration) *ServicesMonitor {
	if len(names) == 0 {
		names = []string{"nginx", "php8.1-fpm", "mariadb", "redis-server"}
	}
	if timeout < time.Second {
		timeout = 3 * time.Second
	}
	return &ServicesMonitor{
		names:   names,
		timeout: timeout,
	}
}

func (m *ServicesMonitor) List(ctx context.Context) ([]ServiceState, error) {
	now := time.Now().UTC()
	items := make([]ServiceState, 0, len(m.names))
	for _, name := range m.names {
		status := "unknown"
		if runtime.GOOS == "linux" {
			probeCtx, cancel := context.WithTimeout(ctx, m.timeout)
			out, err := exec.CommandContext(probeCtx, "systemctl", "is-active", name).CombinedOutput()
			cancel()
			if err != nil {
				if len(out) > 0 {
					status = strings.TrimSpace(string(out))
				} else {
					status = "unknown"
				}
			} else {
				status = strings.TrimSpace(string(out))
			}
		}
		items = append(items, ServiceState{
			Name:      name,
			Status:    status,
			CheckedAt: now,
		})
	}
	return items, nil
}

func (m *ServicesMonitor) String() string {
	return fmt.Sprintf("services=%v timeout=%s", m.names, m.timeout)
}

