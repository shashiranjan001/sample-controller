package controllers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// isDeletionCandidate checks if object is candidate to be deleted
func isDeletionCandidate(obj metav1.Object, finalizer string) bool {
	return obj.GetDeletionTimestamp() != nil && containsString(obj.GetFinalizers(), finalizer)
}

// needToAddFinalizer checks if need to add finalizer to object
func needToAddFinalizer(obj metav1.Object, finalizer string) bool {
	return obj.GetDeletionTimestamp() == nil && !containsString(obj.GetFinalizers(), finalizer)
}

// containsString checks if a given slice of strings contains the provided string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// addFinalizer accepts an Object and adds the provided finalizer if not present.
func addFinalizer(o metav1.Object, finalizer string) {
	f := o.GetFinalizers()
	for _, e := range f {
		if e == finalizer {
			return
		}
	}
	o.SetFinalizers(append(f, finalizer))
}

// removeFinalizer accepts an Object and removes the provided finalizer if present.
func removeFinalizer(o metav1.Object, finalizer string) {
	f := o.GetFinalizers()
	for i := 0; i < len(f); i++ {
		if f[i] == finalizer {
			f = append(f[:i], f[i+1:]...)
			i--
		}
	}
	o.SetFinalizers(f)
}

// GetUTCTimeNow return current UTC time as metav1.Time type.
func toMetaV1Time(t time.Time) *metav1.Time {
	return &metav1.Time{
		Time: t,
	}
}
