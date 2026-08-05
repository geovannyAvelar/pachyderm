package postgres

import "testing"

func TestGetPortDefaultsWhenUnset(t *testing.T) {
	withTempHome(t)

	port, err := GetPort()
	if err != nil {
		t.Fatalf("GetPort: %v", err)
	}
	if port != DefaultPort {
		t.Fatalf("GetPort = %d, want %d", port, DefaultPort)
	}
}

func TestSetPortRoundTrip(t *testing.T) {
	withTempHome(t)

	if err := SetPort(5433); err != nil {
		t.Fatalf("SetPort: %v", err)
	}

	port, err := GetPort()
	if err != nil {
		t.Fatalf("GetPort: %v", err)
	}
	if port != 5433 {
		t.Fatalf("GetPort = %d, want 5433", port)
	}
}

func TestSetPortRejectsOutOfRange(t *testing.T) {
	withTempHome(t)

	for _, port := range []int{0, 1023, 65536, -1} {
		if err := SetPort(port); err == nil {
			t.Errorf("SetPort(%d): expected error, got nil", port)
		}
	}
}
