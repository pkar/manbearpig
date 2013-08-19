package manbearpig

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type PushStatus struct {
	// Whether or not to retry this notification.
	Retry bool
	// Duration to wait before resending a notification.
	Delay int
	// The number of successful devices pushed to
	Successes int
	// Registration IDs that failed during Push and their error message.
	// This is error type rather than just string to make
	// internal errors mix easier. A single empty hash with an error
	// indicates all were errors.
	Errors map[string]error
	// Registration IDs that need to be updated.
	// This is a map[idtoupdate]newid
	// Currently only GCM supports pushing multiple IDs.
	Updates      map[string]string
	Notification *Notification
	// Authorization token
	Auth string
}

func NewPushStatus(notification *Notification) *PushStatus {
	return &PushStatus{
		Retry:        false,
		Delay:        0,
		Successes:    0,
		Errors:       map[string]error{},
		Updates:      map[string]string{},
		Notification: notification,
	}
}

// Ok determines if the Push request was a success.
func (p *PushStatus) Ok() bool {
	if len(p.Errors) == 0 {
		return true
	}
	return false
}

// Convert the status to a json encoded string of either ok, or
// combined error/update messages.
// After attempting a push serialize any errors/updates.
// If success = {"ok": 1}
// If errors or updates = {"errors": [], "updates":[]}
// they key(errors/updates) only exists if there are any.
func (p *PushStatus) String() string {
	status := map[string]interface{}{}

	nErrors := len(p.Errors)
	nUpdates := len(p.Updates)
	switch {
	case nErrors == 0 && nUpdates == 0:
		status["ok"] = 1
	case nErrors > 0 && nUpdates == 0:
		errs := map[string]string{}
		for k, e := range p.Errors {
			errs[k] = fmt.Sprintf("%s", e)
		}
		status["errors"] = errs
	case nErrors > 0 && nUpdates > 0:
		errs := map[string]string{}
		for k, e := range p.Errors {
			errs[k] = fmt.Sprintf("%s", e)
		}
		status["errors"] = errs
		status["updates"] = p.Updates
	case nErrors == 0 && nUpdates > 0:
		status["updates"] = p.Updates
	}

	var b []byte
	b, err := json.Marshal(status)
	if err != nil {
		return "Unknown"
	}
	return string(b)
}

// Reenqueue jobs after a set delay.
func (p *PushStatus) ReSend(job *Notification) {
	job.Retries++
	if job.Retries > 10 {
		// Give up spaminator.
		return
	}

	switch p.Delay {
	case 0:
		time.Sleep(time.Duration(job.Retries) * time.Second)
	default:
		time.Sleep(time.Duration(p.Delay) * time.Second)
	}

	if !SMGlobal.Quitting {
		go SMGlobal.Work(job, p.Auth)
	}
}

// Create a new job for a single device.
func (p *PushStatus) NewJob(job *Notification, auth string) {
	newJob := job
	p.ReSend(newJob)
}

// ProcessErrors iterates return responses and resends jobs
// or removes tokens depending on the return.
func (p *PushStatus) ProcessErrors(job *Notification) {
	for devToken, err := range p.Errors {
		log.Printf("%s %v", err, devToken)

		switch err.Error() {
		case "NoRegistrationIDs":
			// No-op
		case "TooManyRegistrationIDs":
			// TODO either handle client side or split up here and resend job.
			// For reference
		case "MissingRegistration":
			// No-op
		case "InvalidRegistration":
			// Missing or bad registration_id. Sender should stop sending messages to this device.
			// Remove from db.
		case "MismatchSenderId":
			// A registration ID is tied to a certain group of senders.
			// When an application registers for GCM usage, it must specify
			// which senders are allowed to send messages. Make sure you're using one of
			// those when trying to send messages to the device. If you switch to a
			// different sender, the existing registration IDs won't work.
			// No-op
		case "NotRegistered":
			// If it is NotRegistered, remove the registration ID from your server database.
		case "MessageTooBig":
			// No-op
		case "NoPayload":
			// No-op
		case "InvalidDataKey":
		// The payload data contains a key (such as from or any value prefixed by google.)
		// that is used internally by GCM in the com.google.android.c2dm.intent.RECEIVE
		// Intent and cannot be used. Note that some words (such as collapse_key) are
		// also used by GCM but are allowed in the payload, in which case the payload
		// value will be overridden by the GCM value.
		// No-op
		case "InvalidJSON":
			// No-op
		case "InvalidTtl":
			// The value for the Time to Live field must be an integer representing a duration
			// in seconds between 0 and 2,419,200 (4 weeks).
		case "InvalidPackageName":
			// A message was addressed to a registration ID whose package name did not match
			// the value passed in the request.
			// No-op
		case "InternalServerError":
			p.NewJob(job, devToken)
		case "ServiceUnavailable":
			p.NewJob(job, devToken)
		case "Unavailable":
			p.NewJob(job, devToken)
		case "Unauthorized":
			// Remove auth tokens.
		case "InvalidResponse":
			p.NewJob(job, devToken)
		case "UnknownError":
			p.NewJob(job, devToken)
		default:
		}
	}
}

// Updates are for changing canonical registration ids.
func (p *PushStatus) ProcessUpdates() {
	for devToken, updateId := range p.Updates {
		log.Printf("Updating tokens %s %s", devToken, updateId)
	}
}
