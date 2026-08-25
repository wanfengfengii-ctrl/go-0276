package httpapi

import (
	"strings"
	"sync"
	"testing"
)

func TestCompleteProducesSingleCredential(t *testing.T) {
	svc, _ := newTestService(t)
	prepareClosedCycle(t, svc, "c-term")
	res, err := svc.Complete("c-term", CompleteRequest{OperationID: "done-1", LogicalTime: 3000})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if res.Kind != "completed" || res.CredentialID == "" {
		t.Fatalf("terminal result = %+v", res)
	}
	view, _ := svc.GetCycle("c-term")
	if view.State != "completed" {
		t.Fatalf("state = %q, want completed", view.State)
	}
	// A second terminal request is stably rejected.
	_, err = svc.Cancel("c-term", CancelRequest{OperationID: "cancel-1", LogicalTime: 3001})
	if err == nil || !strings.Contains(err.Error(), CodeTerminalAlreadyDecided) {
		t.Fatalf("expected terminal already decided, got %v", err)
	}
}

func TestConcurrentTerminalCompetitionSingleWinner(t *testing.T) {
	svc, _ := newTestService(t)
	prepareClosedCycle(t, svc, "c-race")

	const n = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			if idx%2 == 0 {
				_, err := svc.Complete("c-race", CompleteRequest{OperationID: "race-done-" + itoa(int64(idx)), LogicalTime: 4000 + int64(idx)})
				results[idx] = errToString(err)
			} else {
				_, err := svc.Cancel("c-race", CancelRequest{OperationID: "race-cancel-" + itoa(int64(idx)), LogicalTime: 4000 + int64(idx)})
				results[idx] = errToString(err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	wins := 0
	for _, r := range results {
		if r == "" {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected exactly one winner, got %d (results=%v)", wins, results)
	}
	view, _ := svc.GetCycle("c-race")
	if view.TerminalVersion != 1 || view.TerminalKind == "" {
		t.Fatalf("terminal must be set after race: %+v", view)
	}
}

func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
