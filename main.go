/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/internal/config"
	"k8s.io/sample-controller/pkg/controllers"
	clientset "k8s.io/sample-controller/pkg/generated/clientset/versioned"
	"k8s.io/sample-controller/pkg/metrics"
)

func main() {

	// set up signals so we handle the first shutdown signal gracefully
	ctx, cancel := signal.NotifyContext(context.Background(), []os.Signal{
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGHUP,
		syscall.SIGQUIT,
	}...)
	defer cancel()

	config, err := config.GetConfig()
	if err != nil {
		panic(fmt.Errorf("error getting config: %w", err))
	}
	logger := getLogger(config)

	var cfg *rest.Config
	var errKubeConfig error
	if config.KubeConfig != "" {
		cfg, errKubeConfig = clientcmd.BuildConfigFromFlags("", config.KubeConfig)
	} else {
		cfg, errKubeConfig = rest.InClusterConfig()
	}
	if errKubeConfig != nil {
		logger.Fatalf("error getting kubernetes config: %s", errKubeConfig)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building kubernetes clientset: %s", err)
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building example clientset: %s", err)
	}

	if config.Metrics.Enabled {
		m := metrics.New(
			metrics.Options{
				Path: config.Metrics.Path,
				Port: config.Metrics.Port,
			},
			logger.WithFields(log.Fields{"type": "metrics"}),
		)
		go m.Start()
		defer m.Stop()
	}

	r := controllers.NewRunner(exampleClient, kubeClient, config, logger)
	r.Start(ctx)
}

// GetLogger return a logrus loggerwith context.
func getLogger(config config.Config) *log.Entry {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})
	logger := log.WithFields(log.Fields{"service": "sample-controller"})
	if config.HA.Enabled {
		logger = logger.WithFields(log.Fields{"nodeId": config.HA.NodeId})
	}
	return logger
}
