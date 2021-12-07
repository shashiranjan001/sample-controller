package metrics

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

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
