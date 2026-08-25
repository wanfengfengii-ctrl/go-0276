package httpapi

import "testing"

func TestReasonSortDeterministic(t *testing.T) {
	reasons := []Reason{
		{Zone: "z2", Greenhouse: "g1", WindowStart: 5, Code: "X"},
		{Zone: "z1", Greenhouse: "g1", WindowStart: 5, Code: "Y"},
		{Zone: "z1", Greenhouse: "g0", WindowStart: 10, Code: "Z"},
	}
	SortReasons(reasons)
	want := []Reason{
		{Zone: "z1", Greenhouse: "g0", WindowStart: 10, Code: "Z"},
		{Zone: "z1", Greenhouse: "g1", WindowStart: 5, Code: "Y"},
		{Zone: "z2", Greenhouse: "g1", WindowStart: 5, Code: "X"},
	}
	for i := range want {
		if reasons[i] != want[i] {
			t.Fatalf("reasons[%d] = %+v, want %+v", i, reasons[i], want[i])
		}
	}
}

func TestNewErrorSortsReasons(t *testing.T) {
	e := NewError(CodeInvalidRequest, "bad",
		Reason{Zone: "b"}, Reason{Zone: "a"})
	if e.Reasons[0].Zone != "a" || e.Reasons[1].Zone != "b" {
		t.Fatalf("reasons not sorted: %+v", e.Reasons)
	}
	if e.Code != CodeInvalidRequest {
		t.Fatalf("code = %q", e.Code)
	}
}
