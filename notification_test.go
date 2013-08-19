package manbearpig

import (
	"testing"
)

func TestNotificationBytes(t *testing.T) {
	n := &Notification{}
	_, err := n.Bytes()
	if err != nil {
		t.Fatalf("Unmarshal notification payload %+v", err)
	}
}
