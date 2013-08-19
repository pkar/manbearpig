package manbearpig

import (
	"encoding/json"
	"os/exec"
	"time"
)

// Notification is the meta data and payload
// sent to a Push provider. The data field is the actual
// send information that is built up using some of the
// other fields.
type Notification struct {
	AppName      string                 `json:"app_name"`      // application name
	Provider     string                 `json:"provider"`      // apns/c2dm/gcm
	DeviceTokens []string               `json:"device_tokens"` // array of tokens to send the payload to.
	Payload      map[string]interface{} `json:"payload"`       // data sent to service.
	Expiry       uint32                 `json:"expiry"`        // seconds
	ExtraData    map[string]interface{} `json:"extra_data"`    // optional data for processing
	Guid         string
	CreatedAt    time.Time
	Status       *PushStatus
	Retries      int
}

// Bytes JSON encodes the Payload field of Notification.
func (n *Notification) Bytes() ([]byte, error) {
	return json.Marshal(n.Payload)
}

// Read job information into self.
func (n *Notification) Init() error {
	// Generate a unique id for the push.
	// This may be a bottleneck for high volumes.
	out, err := exec.Command("uuidgen").Output()
	if err != nil {
		return err
	}
	n.Guid = string(out)

	// created_at
	n.CreatedAt = time.Now().UTC()
	n.Status = &PushStatus{}
	return nil
}
