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

package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	samplev1alpha1 "k8s.io/sample-controller/pkg/apis/samplecontroller/v1alpha1"
	"k8s.io/sample-controller/pkg/cloud"
	clientset "k8s.io/sample-controller/pkg/generated/clientset/versioned"
	samplescheme "k8s.io/sample-controller/pkg/generated/clientset/versioned/scheme"
	informers "k8s.io/sample-controller/pkg/generated/informers/externalversions/samplecontroller/v1alpha1"
	listers "k8s.io/sample-controller/pkg/generated/listers/samplecontroller/v1alpha1"
)

const controllerAgentName = "sample-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a VM is synced
	SuccessSynced = "Synced"
	// MessageResourceSynced is the message used for an Event fired when a VM
	// is synced successfully
	MessageResourceSynced = "VM synced successfully"
)

const (
	VMSyncPeriod = time.Second * 30
)

// Controller is the controller implementation for VM resources
type Controller struct {
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	vmsLister listers.VMLister
	vmsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	vmInformer informers.VMInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		sampleclientset: sampleclientset,
		vmsLister:       vmInformer.Lister(),
		vmsSynced:       vmInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "VMs"),
		recorder:        recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when VM resources change
	vmInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueVM,
		UpdateFunc: func(old, new interface{}) {
			// TODO: check if we can remove the old == new condition.
			newVM := new.(*samplev1alpha1.VM)
			oldVM := old.(*samplev1alpha1.VM)
			if newVM.ResourceVersion == oldVM.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.enqueueVM(new)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting VM controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.vmsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process VM resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, 30*time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// VM resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the VM resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the VM resource with this namespace/name
	vm, err := c.vmsLister.VMs(namespace).Get(name)
	if err != nil {
		// The VM resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("vm '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}
	if vm.Status.NextSyncAt != nil &&
		!vm.Status.NextSyncAt.Before(toMetaV1Time(time.Now().UTC())) {
		return nil
	}
	if err := c.vmHandler(vm); err != nil {
		return err
	}

	c.recorder.Event(vm, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// Returns if requeue is needed, and error.
func (c *Controller) vmHandler(vm *samplev1alpha1.VM) error {
	if IsDeletionCandidate(vm, samplev1alpha1.VMFinalizer) {
		// VM should be deleted. Check if it's deleted and remove finalizer.
		err := cloud.DeleteVM(vm.Spec.VMName)
		if err != nil {
			klog.Errorf("Failed to delete VM. Will retry")
			return err
		}
		return c.removeFinalizer(vm)
	}

	if NeedToAddFinalizer(vm, samplev1alpha1.VMFinalizer) {
		return c.addFinalizer(vm)
	}

	vmID := vm.Status.VMID
	if vmID == "" {
		return c.createVM(vm)
	}
	return c.syncVMStatus(vm)
}

// GetUTCTimeNow return current UTC time as metav1.Time type.
func toMetaV1Time(t time.Time) *metav1.Time {
	return &metav1.Time{
		Time: t,
	}
}

// createVM tries to create the VM by calling the Cloud API endpoints and returns error
// if the creation failed, and we need to retry.
func (c *Controller) createVM(vm *samplev1alpha1.VM) error {
	ok, err := cloud.IsNameValid(vm.Name)
	if err != nil {
		cerr := cloud.ToCloudError(err)
		if cerr == nil {
			// Retry for intermittent errors.
			return err
		}
		// In case the server is down or misbehaving, we do not retry.
		// We also do not retry if the VM name is already in use.
		utilruntime.HandleError(err)
		return nil
	}
	if !ok {
		utilruntime.HandleError(fmt.Errorf("VM name is not valid"))
		return nil
	}
	cvm, err := cloud.CreateVM(vm.Name)
	if err != nil {
		cerr := cloud.ToCloudError(err)
		if cerr == nil {
			// Intermittent errors, retry
			return err
		}
		// In case the server is down or misbehaving, we do not retry.
		// We also do not retry if the VM name is already in use.
		utilruntime.HandleError(err)
		return nil
	}
	vmStatus := samplev1alpha1.VMStatus{
		VMID: cvm.ID,
	}
	// Doing a slighlty expensive call for newly created VMs to minimize conflict.
	if err := c.updateLatestVMStatus(vm, vmStatus); err != nil {
		// This is a tricky part. If the VM is created in the cloud,
		// but we fail to update CR status, then we lose track of the created VM.
		// Retry won't not help here, because cloud API would return http.StatusConflict.
		utilruntime.HandleError(err)
		return nil
	}
	// Enqueue VM for the perioding syncing of the Status.
	c.enqueueVMAfter(vm, VMSyncPeriod)
	return nil
}

func (c *Controller) syncVMStatus(vm *samplev1alpha1.VM) error {
	if vm.Status.VMID == "" {
		utilruntime.HandleError(fmt.Errorf(
			"Cannot sync VM status from cloud, VM ID is empty in CR status"))
	}
	cvmStatus, err := cloud.GetVMStatus(vm.Status.VMID)
	if err != nil {
		cerr := cloud.ToCloudError(err)
		if cerr == nil {
			// Retry for intermittent errors.
			return err
		}
		// In case the server is down or misbehaving, we do not retry.
		// We also do not retry if the VM is not found in the cloud.
		utilruntime.HandleError(err)
		return nil
	}
	nextSyncAt := toMetaV1Time(time.Now().UTC().Add(VMSyncPeriod))
	vmStatus := samplev1alpha1.VMStatus{
		VMID:           vm.Status.VMID,
		CPUUtilization: cvmStatus.CPUUtilization,
		NextSyncAt:     nextSyncAt,
	}
	// We ignore errors while updating VM status because this is periodic resync.
	if err = c.updateVMStatus(vm, vmStatus); err != nil {
		klog.Info("Error syncing VM status: %s", err.Error())
	}
	// Enqueue VM for the perioding syncing of the Status.
	c.enqueueVMAfter(vm, VMSyncPeriod)
	return nil
}

func (c *Controller) updateLatestVMStatus(vm *samplev1alpha1.VM, vmStatus samplev1alpha1.VMStatus) error {
	// Fetching the latest VM from client set, so that we have minimal chances of conflict.
	vmLatest, err := c.sampleclientset.SamplecontrollerV1alpha1().VMs(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	vmLatest.Status = vmStatus
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the VM resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err = c.sampleclientset.SamplecontrollerV1alpha1().VMs(vm.Namespace).UpdateStatus(context.Background(), vmLatest, metav1.UpdateOptions{})
	return err
}

func (c *Controller) updateVMStatus(vm *samplev1alpha1.VM, vmStatus samplev1alpha1.VMStatus) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	vmCopy := vm.DeepCopy()
	vmCopy.Status = vmStatus
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the VM resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.sampleclientset.SamplecontrollerV1alpha1().VMs(vm.Namespace).UpdateStatus(context.Background(), vmCopy, metav1.UpdateOptions{})
	return err
}

// enqueueVM takes a VM resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than VM.
func (c *Controller) enqueueVM(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// enqueueVMAfter is like enqueueVM, but it enqueues the VM object after a given duration of time.
func (c *Controller) enqueueVMAfter(obj interface{}, d time.Duration) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.AddAfter(key, d)
}

func (c *Controller) addFinalizer(vm *samplev1alpha1.VM) error {
	vmCopy := vm.DeepCopy()
	AddFinalizer(vmCopy, samplev1alpha1.VMFinalizer)
	if vmCopy.Labels == nil {
		vmCopy.Labels = make(map[string]string)
	}
	vm, err := c.sampleclientset.SamplecontrollerV1alpha1().VMs(vm.Namespace).Update(context.TODO(), vmCopy, metav1.UpdateOptions{})
	if err != nil {
		klog.V(3).Infof("Error adding finalizer to vm %s: %v", vm.Name, err)
		return err
	}
	klog.V(3).Infof("Added finalizer to vm %s", vm.Name)
	return nil
}

func (c *Controller) removeFinalizer(vm *samplev1alpha1.VM) error {
	clone := vm.DeepCopy()
	vm.GetFinalizers()
	RemoveFinalizer(clone, samplev1alpha1.VMFinalizer)
	vm, err := c.sampleclientset.SamplecontrollerV1alpha1().VMs(vm.Namespace).Update(context.TODO(), clone, metav1.UpdateOptions{})
	if err != nil {
		klog.V(3).Infof("Error removing finalizer from vm %s: %v", vm.Name, err)
		return err
	}
	klog.V(3).Infof("Removed protection finalizer from vm %s", vm.Name)
	return nil
}
