package httpapi

import (
	"testing"

	"tomato-bumblebee-pollination-rotation/internal/coverage"
)

func TestDeviceRetryDeterministicBackoff(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-dev")); err != nil {
		t.Fatal(err)
	}
	// Register a device call that will fail twice then succeed.
	key, err := svc.RegisterDeviceCall("c-dev", coverage.DeviceEnvironmentProbe, "z1", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	driver := NewScriptDriver()
	driver.Set(coverage.DeviceEnvironmentProbe, &coverage.FailureScript{
		CycleID:    "c-dev",
		DeviceType: coverage.DeviceEnvironmentProbe,
		Outcomes:   []coverage.DeviceResultStatus{coverage.DeviceDisconnected, coverage.DeviceTimeout, coverage.DeviceSuccess},
	})
	svc.driver = driver

	r1, err := svc.RetryDevice(key)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Status != "disconnected" || r1.Attempts != 1 {
		t.Fatalf("attempt 1 = %+v", r1)
	}
	r2, err := svc.RetryDevice(key)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Status != "timeout" || r2.Attempts != 2 {
		t.Fatalf("attempt 2 = %+v", r2)
	}
	r3, err := svc.RetryDevice(key)
	if err != nil {
		t.Fatal(err)
	}
	if r3.Status != "success" || r3.Attempts != 3 {
		t.Fatalf("attempt 3 = %+v", r3)
	}
}

func TestDeviceRetryPermanentFailureAtMax(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.CreateAndLock(validLockRequest("c-perm")); err != nil {
		t.Fatal(err)
	}
	key, err := svc.RegisterDeviceCall("c-perm", coverage.DeviceHiveScale, "z1", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	driver := NewScriptDriver()
	driver.Set(coverage.DeviceHiveScale, &coverage.FailureScript{
		CycleID:    "c-perm",
		DeviceType: coverage.DeviceHiveScale,
		Outcomes:   []coverage.DeviceResultStatus{coverage.DeviceRejected},
	})
	svc.driver = driver

	for i := 1; i <= 3; i++ {
		if _, err := svc.RetryDevice(key); err != nil {
			t.Fatal(err)
		}
	}
	r, err := svc.RetryDevice(key)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "permanent_fail" {
		t.Fatalf("status = %q, want permanent_fail", r.Status)
	}
}
