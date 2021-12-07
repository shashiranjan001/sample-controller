package controllers

// import (
// 	"context"

// 	log "github.com/sirupsen/logrus"
// 	"k8s.io/sample-controller/internal/config"

// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/client-go/kubernetes"
// 	"k8s.io/client-go/tools/leaderelection"
// 	"k8s.io/client-go/tools/leaderelection/resourcelock"
// )

// type Runner struct {
// 	ctrl      *Controller
// 	clientset *kubernetes.Clientset
// 	config    config.Config
// 	logger    log.Entry
// }

// func (r *Runner) Start(ctx context.Context) {
// 	if r.config.HA.Enabled {
// 		r.logger.Info("starting HA controller")
// 		r.runHA(ctx)
// 	} else {
// 		r.logger.Info("starting standalone controller")
// 		r.runSingleNode(ctx)
// 	}
// }

// func (r *Runner) runSingleNode(ctx context.Context) {
// 	// if err := r.ctrl.Run(3, ctx.Done()); err != nil {
// 	// 	log.Fatalf("error running controller: %s", err)
// 	// }
// }

// func (r *Runner) runHA() {
// 	if !r.config.HA.Enabled {
// 		log.Fatalf("HA config not set or not enabled")
// 	}

// 	lock := &resourcelock.LeaseLock{
// 		LeaseMeta: metav1.ObjectMeta{
// 			Name:      r.config.HA.LeaseLockName,
// 			Namespace: r.config.Namespace,
// 		},
// 		Client: r.clientset.CoordinationV1(),
// 		LockConfig: resourcelock.ResourceLockConfig{
// 			Identity: r.config.HA.NodeId,
// 		},
// 	}
// 	leaderelection.RunOrDie(r.logger.Context, leaderelection.LeaderElectionConfig{
// 		Lock:            lock,
// 		ReleaseOnCancel: true,
// 		LeaseDuration:   r.config.HA.LeaseDuration,
// 		RenewDeadline:   r.config.HA.RenewDeadline,
// 		RetryPeriod:     r.config.HA.RetryPeriod,
// 		Callbacks: leaderelection.LeaderCallbacks{
// 			OnStartedLeading: func(ctx context.Context) {
// 				log.Info("started leading")
// 				r.runSingleNode(ctx)
// 			},
// 			OnStoppedLeading: func() {
// 				log.Info("stopped leading")
// 			},
// 			OnNewLeader: func(nodeId string) {
// 				if nodeId == r.config.HA.NodeId {
// 					log.Info("obtained leadership")
// 					return
// 				}
// 				log.WithFields(log.Fields{"leaderNodeId": nodeId}).Info("leader elected")
// 			},
// 		},
// 	})
// }

// func NewRunner(
// 	ctrl *Controller,
// 	clientset *kubernetes.Clientset,
// 	config config.Config,
// 	logger log.Entry,
// ) *Runner {
// 	return &Runner{
// 		ctrl:      ctrl,
// 		clientset: clientset,
// 		config:    config,
// 		logger:    logger,
// 	}
// }
