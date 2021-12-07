package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gotway/gotway/pkg/env"
)

type HA struct {
	Enabled       bool
	NodeId        string
	LeaseLockName string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
}

func (c HA) String() string {
	s, _ := json.Marshal(c)
	return string(s)
}

type Metrics struct {
	Enabled bool
	Path    string
	Port    string
}

func (m Metrics) String() string {
	s, _ := json.Marshal(m)
	return string(s)
}

type Config struct {
	KubeConfig string
	Namespace  string
	NumWorkers int
	HA         HA
	Metrics    Metrics
}

func (c Config) String() string {
	s, _ := json.Marshal(c)
	return string(s)
}

func GetConfig() (Config, error) {
	ha := env.GetBool("HA_ENABLED", false)

	var nodeId string
	if ha {
		nodeId = env.Get("HA_NODE_ID", "")
		if nodeId == "" {
			hostname, err := os.Hostname()
			if err != nil {
				return Config{}, fmt.Errorf("error getting node id %v", err)
			}
			nodeId = hostname
		}
	}

	return Config{
		KubeConfig: env.Get("KUBECONFIG", ""),
		Namespace:  env.Get("NAMESPACE", "default"),
		NumWorkers: env.GetInt("NUM_WORKERS", 4),
		HA: HA{
			Enabled:       ha,
			NodeId:        nodeId,
			LeaseLockName: env.Get("HA_LEASE_LOCK_NAME", "sample-controller"),
			LeaseDuration: env.GetDuration("HA_LEASE_DURATION_SECONDS", 15) * time.Second,
			RenewDeadline: env.GetDuration("HA_RENEW_DEADLINE_SECONDS", 10) * time.Second,
			RetryPeriod:   env.GetDuration("HA_RETRY_PERIOD_SECONDS", 2) * time.Second,
		},
		Metrics: Metrics{
			Enabled: env.GetBool("METRICS_ENABLED", true),
			Path:    env.Get("METRICS_PATH", "/metrics"),
			Port:    env.Get("METRICS_PORT", "8000"),
		},
	}, nil
}
