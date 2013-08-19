package manbearpig

import (
	"fmt"
	"testing"
)

func TestPushStatusString(t *testing.T) {
	ps := NewPushStatus(nil)
	res := ps.String()
	if res != `{"ok":1}` {
		t.Fatalf("Res should be ok %v", res)
	}
	ps.Errors["someRegId"] = fmt.Errorf("test")
	res = ps.String()
	if res == `{"ok":1}` {
		t.Fatalf("Res should be not be ok %v", res)
	}
}

func TestPushStatusOk(t *testing.T) {
	ps := NewPushStatus(nil)
	if !ps.Ok() {
		t.Fatalf("%+v", ps)
	}
	ps.Errors["someRegId"] = fmt.Errorf("test")
	if ps.Ok() {
		t.Fatalf("Should not be true because of error %+v", ps)
	}
	ps.Errors = nil
	ps.Updates["someRegId"] = "someOtherId"
	if !ps.Ok() {
		t.Fatalf("Should true because of updates %+v", ps)
	}
}

func BenchmarkPushStatusString(b *testing.B) {
	ps := NewPushStatus(nil)
	for i := 0; i < b.N; i++ {
		ps.String()
	}
}
