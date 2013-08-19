package manbearpig

import (
	"log"
	"net/http"
	"sync"
)

// SMGlobal [...]
var SMGlobal *ServiceManager

// Keep track of the current state of work being done.
type Stats struct {
	Running    int
	APNS       uint64
	APNSErrors uint64
	GCM        uint64
	GCMErrors  uint64
	C2DM       uint64
	C2DMErrors uint64
}

// Service is an abstraction of the final Push endpoint. Currently
// apns/c2dm/gcm are the options
type Service interface {
	// Push does the actual sending depending on the
	// provider. A PushStatus object is returned to indicate if there
	// were errors and if the notification needs to be resent.
	Push(*Notification, string) *PushStatus
}

// ServiceManager routes all requests for Push to the appropriate
// Push object.
type ServiceManager struct {
	Services map[string]Service // Define the available services apns/gcm/c2dm.
	Quit     chan struct{}      // Shutdown signal for go routines
	Quitting bool               // Prevent adding to the jobs channel after closing.
	Stats    *Stats             // Keep track of running jobs
}

// Work takes jobs and creates a new
// Notification object to be sent
// to the appropriate service provider Push.
func (sm *ServiceManager) Work(job *Notification, auth string) {
	// Get the service type from available services and
	// send the notification.
	provider, ok := sm.Services[job.Provider]
	if !ok {
		return
	}
	err := job.Init()
	if err != nil {
		log.Printf("%s %+v", err, job)
		return
	}

	pushStatus := provider.Push(job, auth)
	job.Status = pushStatus
	if pushStatus.Retry {
		log.Printf("Retrying job %+v in %v seconds", job, pushStatus.Delay)
		pushStatus.ReSend(job)
		return
	}

	if len(pushStatus.Errors) > 0 {
		log.Printf("(%d) Push Errors Notification: %+v PushStatus: %+v", sm.Stats.Running, job, pushStatus)
		for _, _ = range pushStatus.Errors {
			switch job.Provider {
			case "apns":
				sm.Stats.APNSErrors++
			case "gcm":
				sm.Stats.GCMErrors++
			case "c2dm":
				sm.Stats.C2DMErrors++
			}
		}
		go pushStatus.ProcessErrors(job)
		return
	}

	if pushStatus.Ok() {
		log.Printf("(%d) Push OK Notification: %+v", sm.Stats.Running, job)
	}

	if len(pushStatus.Updates) > 0 {
		go pushStatus.ProcessUpdates()
	}
}

// Close stops all workers and waits for any processing
// work to finish before returning.
func (sm *ServiceManager) Close() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in Close")
		}
	}()
	sm.Quitting = true
	close(sm.Quit)
}

// NewServiceManager loads a datastore and configuration files.
func NewServiceManager() (*ServiceManager, error) {
	services := make(map[string]Service)
	services["apns"] = APNS{map[string]*APNSConnPool{}, &sync.Mutex{}}
	services["gcm"] = GCM{&http.Client{}}
	services["c2dm"] = C2DM{&http.Client{}}

	quit := make(chan struct{})

	sm := &ServiceManager{
		Services: services,
		Quit:     quit,
		Quitting: false,
		Stats:    &Stats{},
	}
	SMGlobal = sm
	return sm, nil
}
