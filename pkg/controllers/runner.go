package controllers

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type Config struct {
	KubeConfig string
	Namespace  string
	NumWorkers int
	HA         HA
	Metrics    Metrics
	Env        string
	LogLevel   string
}

type HA struct {
	Enabled       bool
	NodeId        string
	LeaseLockName string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
}

type Metrics struct {
	Enabled bool
	Path    string
	Port    string
}

type Runner struct {
	ctrl      *Controller
	clientset *kubernetes.Clientset
	config    Config
}

func (r *Runner) Start(ctx context.Context) {
	if r.config.HA.Enabled {
		klog.Info("starting HA controller")
		r.runHA(ctx)
	} else {
		klog.Info("starting standalone controller")
		r.runSingleNode(ctx)
	}
}

func (r *Runner) runSingleNode(ctx context.Context) {
	if err := r.ctrl.Run(3, ctx.Done()); err != nil {
		klog.Fatal("error running controller ", err)
	}
}

func (r *Runner) runHA(ctx context.Context) {
	if r.config.HA == (HA{}) || !r.config.HA.Enabled {
		klog.Fatal("HA config not set or not enabled")
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      r.config.HA.LeaseLockName,
			Namespace: r.config.Namespace,
		},
		Client: r.clientset.CoordinationV1(),
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
				klog.Info("start leading")
				r.runSingleNode(ctx)
			},
			OnStoppedLeading: func() {
				klog.Info("stopped leading")
			},
			OnNewLeader: func(identity string) {
				if identity == r.config.HA.NodeId {
					klog.Info("obtained leadership")
					return
				}
				klog.Infof("leader elected: '%s'", identity)
			},
		},
	})
}

func NewRunner(
	ctrl *Controller,
	clientset *kubernetes.Clientset,
	config Config,
) *Runner {
	return &Runner{
		ctrl:      ctrl,
		clientset: clientset,
		config:    config,
	}
}
