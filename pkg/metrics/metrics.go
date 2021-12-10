package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

const (
	// namespaceNameDefault is the default namespace for this project.
	namespaceNameDefault = "sample"

	// subsystemNameDefault is the default subsystem for this project.
	subsystemNameDefault = "controller"
)

var (
	cloudRequestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespaceNameDefault,
		Subsystem: subsystemNameDefault,
		Name:      "cloud_api_request_latency_seconds",
		Help:      "Time taken in seconds for the private cloud to respond to a request",
		Buckets:   prometheus.DefBuckets,
	},
		[]string{"method", "path", "statuscode"})

	WorkqueueRetriesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNameDefault,
		Subsystem: subsystemNameDefault,
		Name:      "workqueue_retries_total",
		Help:      "Total number workqueue retries",
	})

	MaxConcurrentReconcilers = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceNameDefault,
		Subsystem: subsystemNameDefault,
		Name:      "max_concurrent_reconcilers",
		Help:      "Maximum number of workers for the controller",
	})
)

// NewExecution creates an Execution instance and starts the timer
func NewExecution(method string, path string) execution {
	return execution{
		begin: time.Now(),
		labels: prometheus.Labels{
			"method": method,
			"path":   path,
		},
	}
}

// execution tracks state for an API execution for emitting metrics
type execution struct {
	begin  time.Time
	labels prometheus.Labels
}

// Finish is used to log duration and success/failure
func (e *execution) Finish(statuscode int) {
	e.labels["statuscode"] = fmt.Sprintf("%d", statuscode)
	duration := time.Since(e.begin)
	cloudRequestLatency.With(e.labels).Observe(duration.Seconds())
}

type Options struct {
	Path string
	Port string
}

type Metrics struct {
	options Options
	server  *http.Server
	logger  *log.Entry
}

func (m *Metrics) Start() {
	http.Handle(m.options.Path, promhttp.Handler())
	m.logger.WithFields(
		log.Fields{
			"path": m.options.Path,
			"port": m.options.Port,
		},
	).Infof("metrics server started")

	if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		m.logger.Errorf("error staring metrics server: %s", err)
		return
	}
}

func (m *Metrics) Stop() {
	if err := m.server.Shutdown(context.Background()); err != nil {
		m.logger.Errorf("error stopping metrics server: %s", err)
		return
	}
	m.logger.Info("stopped metrics server")
}

func New(options Options, logger *log.Entry) *Metrics {
	addr := ":" + options.Port
	server := &http.Server{Addr: addr}
	return &Metrics{options, server, logger}
}
