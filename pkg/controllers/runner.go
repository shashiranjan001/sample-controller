package controllers

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/sample-controller/internal/config"
	clientset "k8s.io/sample-controller/pkg/generated/clientset/versioned"
	informers "k8s.io/sample-controller/pkg/generated/informers/externalversions"
	"k8s.io/sample-controller/pkg/metrics"
)

type Runner struct {
	kubeClient    *kubernetes.Clientset
	exampleClient *clientset.Clientset
	config        config.Config
	logger        *log.Entry
}

func (r *Runner) Start(ctx context.Context) {
	if r.config.HA.Enabled {
		r.logger.Info("starting HA controller")
		r.runHA(ctx)
	} else {
		r.logger.Info("starting standalone controller")
		r.runSingleNode(ctx)
	}
}

func (r *Runner) runSingleNode(ctx context.Context) {
	logger := r.logger.WithContext(ctx)
	exampleInformerFactory := informers.NewSharedInformerFactory(r.exampleClient, time.Second*30)

	controller := NewController(r.kubeClient, r.exampleClient,
		exampleInformerFactory.Samplecontroller().V1alpha1().VMs(), logger)
	exampleInformerFactory.Start(logger.Context.Done())

	numWorkers := r.config.NumWorkers
	metrics.MaxConcurrentReconcilers.Set(float64(numWorkers))
	if err := controller.Run(logger, numWorkers); err != nil {
		logger.Fatalf("Error running controller: %s", err)
	}
}

func (r *Runner) runHA(ctx context.Context) {
	if !r.config.HA.Enabled {
		log.Fatalf("HA config not set or not enabled")
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      r.config.HA.LeaseLockName,
			Namespace: r.config.Namespace,
		},
		Client: r.kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: r.config.HA.NodeId,
		},
	}
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   r.config.HA.LeaseDuration,
		RenewDeadline:   r.config.HA.RenewDeadline,
		RetryPeriod:     r.config.HA.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Info("started leading")
				r.runSingleNode(ctx)
			},
			OnStoppedLeading: func() {
				log.Info("stopped leading")
			},
			OnNewLeader: func(nodeId string) {
				if nodeId == r.config.HA.NodeId {
					log.Info("obtained leadership")
					return
				}
				log.WithFields(log.Fields{"leaderNodeId": nodeId}).Info("leader elected")
			},
		},
	})
}

func NewRunner(
	exampleClient *clientset.Clientset,
	kubeClient *kubernetes.Clientset,
	config config.Config,
	logger *log.Entry,
) *Runner {
	return &Runner{
		exampleClient: exampleClient,
		kubeClient:    kubeClient,
		config:        config,
		logger:        logger,
	}
}
