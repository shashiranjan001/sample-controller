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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/internal/config"
	"k8s.io/sample-controller/pkg/controllers"
	clientset "k8s.io/sample-controller/pkg/generated/clientset/versioned"
	informers "k8s.io/sample-controller/pkg/generated/informers/externalversions"
)

var (
	masterURL  string
	kubeconfig string
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
	logger := getLogger(config, ctx)

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

	exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)
	controller := controllers.NewController(kubeClient, exampleClient,
		exampleInformerFactory.Samplecontroller().V1alpha1().VMs(), logger)

	exampleInformerFactory.Start(ctx.Done())

	if err = controller.Run(logger, 1); err != nil {
		logger.Fatalf("Error running controller: %s", err)
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

// GetLogger return a logrus loggerwith context.
func getLogger(config config.Config, ctx context.Context) *log.Entry {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})
	logger := log.WithContext(ctx).WithFields(log.Fields{"service": "sample-controller"})
	if config.HA.Enabled {
		logger = logger.WithFields(log.Fields{"nodeId": config.HA.NodeId})
	}
	return logger
}
