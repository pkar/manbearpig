package manbearpig

import (
	"testing"
)

func TestNewServiceManager(t *testing.T) {
	_, err := NewServiceManager()
	if err != nil {
		t.Fatal("Couldn't create service manager", err)
	}
}
