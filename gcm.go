package manbearpig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

const (
	gcmServiceURL string = "https://android.googleapis.com/gcm/send"
)

// http://developer.android.com/guide/google/gcm/gcm.html#send-msg
type GCMMessage struct {
	RegistrationIDs []string               `json:"registration_ids"`
	CollapseKey     string                 `json:"collapse_key,omitempty"`
	Data            map[string]interface{} `json:"data,omitempty"`
	DelayWhileIdle  bool                   `json:"delay_while_idle,omitempty"`
	TimeToLive      uint32                 `json:"time_to_live,omitempty"`
	DryRun          bool                   `json:"dry_run,omitempty"`
}

// http://developer.android.com/guide/google/gcm/gcm.html#send-msg
type GCMResponse struct {
	MulticastID  int64 `json:"multicast_id"`
	Success      int   `json:"success"`
	Failure      int   `json:"failure"`
	CanonicalIDs int   `json:"canonical_ids"`
	Results      []struct {
		MessageID      string `json:"message_id"`
		RegistrationID string `json:"registration_id"`
		Error          string `json:"error"`
	} `json:"results"`
}

type GCM struct {
	Client *http.Client
}

// NotificationToGCM takes the notification meta and data and converts
// it to JSON format for GCM.
func (g GCM) ConvertNotification(notification *Notification) ([]byte, error) {
	gcm := &GCMMessage{
		RegistrationIDs: notification.DeviceTokens,
		CollapseKey:     notification.AppName,
		Data:            notification.Payload,
		DelayWhileIdle:  true,
		TimeToLive:      2419200, // 4 weeks
		//DryRun: true,
	}

	expiry := notification.Expiry
	if expiry != 0 {
		gcm.TimeToLive = expiry
	}

	return json.Marshal(gcm)
}

func (g GCM) Push(notification *Notification, authKey string) *PushStatus {
	ps := NewPushStatus(notification)
	if len(notification.DeviceTokens) == 0 {
		log.Printf("No Registration IDs given %+v", notification)
		ps.Errors[""] = fmt.Errorf("NoDeviceTokens")
		return ps
	}

	if len(notification.DeviceTokens) > 1000 {
		log.Printf("Too many RegistrationIDs (1000 max): %d", len(notification.DeviceTokens))
		ps.Errors[""] = fmt.Errorf("TooManyDeviceTokens")
		return ps
	}

	if len(notification.Payload) == 0 {
		log.Printf("No Payload Defined %+v", notification)
		ps.Errors[notification.DeviceTokens[0]] = fmt.Errorf("NoPayload")
		return ps
	}

	b, err := g.ConvertNotification(notification)
	if err != nil {
		log.Printf("Invalid JSON %+v", notification)
		ps.Errors[""] = fmt.Errorf("InvalidJSON")
		return ps
	}
	request, err := http.NewRequest("POST", gcmServiceURL, bytes.NewBuffer(b))
	if err != nil {
		ps.Retry = true
		ps.Errors[""] = err
		return ps
	}
	request.Header.Add("Authorization", fmt.Sprintf("key=%s", authKey))
	request.Header.Add("Content-Type", "application/json")

	resp, err := g.Client.Do(request)
	if err != nil {
		log.Printf("%s", err)
		ps.Retry = true
		ps.Errors[""] = err
		return ps
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 503:
		// Service Unavailable
		log.Printf("GCM Service Unavailable %+v", notification)
		// Sleep and retry after a second.
		after := resp.Header.Get("Retry-After")
		sleepFor, _ := strconv.Atoi(after)
		ps.Delay = sleepFor
		ps.Retry = true
		ps.Errors[""] = fmt.Errorf("ServiceUnavailable")
		return ps
	case 500:
		// Internal Server Error
		// Sleep and retry after a second.
		after := resp.Header.Get("Retry-After")
		sleepFor, _ := strconv.Atoi(after)
		ps.Delay = sleepFor
		ps.Retry = true
		ps.Errors[""] = fmt.Errorf("ServiceUnavailable")
		return ps
	case 401:
		// Unauthorized
		// https://developer.android.com/google/gcm/gcm.html#auth_error
		// Possible reasons:
		// - Authorization header missing or with invalid syntax.
		// - Invalid project number sent as key.
		// - Key valid but with GCM service disabled.
		// - Request originated from a server not whitelisted in the Server Key IPs.
		log.Printf("GCM Unauthorized: Key=%s Notification=%+v", authKey, notification)
		ps.Errors[""] = fmt.Errorf("Unauthorized")
		return ps
	case 400:
		// Malformed JSON
		log.Printf("GCM Malformed json %+v", notification)
		ps.Errors[""] = fmt.Errorf("InvalidJSON")
		return ps
	}

	// Catch anything else.
	if resp.StatusCode != 200 {
		log.Printf("GCM Push: %s %+v", resp.Status, notification)
		ps.Retry = true
		ps.Errors[""] = fmt.Errorf("UnknownError")
		return ps
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ps.Retry = true
		ps.Errors[""] = fmt.Errorf("InvalidResponse")
		return ps
	}

	var ret GCMResponse
	err = json.Unmarshal(body, &ret)
	if err != nil {
		log.Printf("GCM Push: %s %+v", err, notification)
		ps.Retry = true
		ps.Errors[""] = err
		return ps
	}
	ps.Successes = ret.Success

	// If the value of failure and canonical_ids is 0,
	// it's not necessary to parse the remainder of the response.
	if ret.Failure == 0 && ret.CanonicalIDs == 0 {
		return ps
	}

	if ret.Failure > 0 {
		after := resp.Header.Get("Retry-After")
		sleepFor, _ := strconv.Atoi(after)
		ps.Delay = sleepFor
	}

	// Iterate through the results field.
	for i, result := range ret.Results {
		// If message ID is set this means the message was processed.
		if result.MessageID != "" {
			// If the registration id is set, this signals to update
			// the one stored in the db.
			if result.RegistrationID != "" {
				ps.Updates[notification.DeviceTokens[i]] = result.RegistrationID
			}
		} else {
			/*
				switch result.Error {
				case "Unavailable":
				case "NotRegistered":
				}
			*/
			ps.Errors[notification.DeviceTokens[i]] = fmt.Errorf(result.Error)
		}
	}
	return ps
}
