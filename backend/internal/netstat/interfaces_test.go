package netstat

import (
	"testing"
	"time"
)

func TestCollectInterfaces(t *testing.T) {
	stats, err := CollectInterfaces()
	if err != nil {
		t.Fatalf("CollectInterfaces failed: %v", err)
	}

	if len(stats) == 0 {
		t.Fatal("expected at least one interface")
	}

	// Should have loopback
	foundLo := false
	for _, s := range stats {
		if s.Name == "lo" {
			foundLo = true
			if s.RxBytes == 0 && s.TxBytes == 0 {
				t.Error("loopback should have some traffic")
			}
		}

		// Validate timestamp
		if s.TS.IsZero() {
			t.Errorf("interface %s has zero timestamp", s.Name)
		}

		// Basic sanity checks
		if s.Name == "" {
			t.Error("interface has empty name")
		}
	}

	if !foundLo {
		t.Log("Warning: loopback interface 'lo' not found")
	}
}

func TestCalculateRates(t *testing.T) {
	// First snapshot
	prev, err := CollectInterfaces()
	if err != nil {
		t.Fatalf("first collect failed: %v", err)
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Second snapshot
	curr, err := CollectInterfaces()
	if err != nil {
		t.Fatalf("second collect failed: %v", err)
	}

	rates := CalculateRates(prev, curr)

	// May have zero rates if no traffic, but should calculate
	if len(rates) == 0 && len(prev) > 0 && len(curr) > 0 {
		t.Error("expected rate calculations for common interfaces")
	}

	for _, r := range rates {
		if r.Interval <= 0 {
			t.Errorf("invalid interval %f for %s", r.Interval, r.Name)
		}

		if r.RxBytesPerSec < 0 || r.TxBytesPerSec < 0 {
			t.Errorf("negative rate for %s: rx=%f tx=%f",
				r.Name, r.RxBytesPerSec, r.TxBytesPerSec)
		}
	}
}

func TestCache(t *testing.T) {
	cache := NewCache(100 * time.Millisecond)

	// First call - no rates yet
	stats1, rates1, err := cache.GetInterfacesWithRates()
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	if len(stats1) == 0 {
		t.Fatal("expected interfaces")
	}

	if len(rates1) != 0 {
		t.Error("first call should not have rates")
	}

	// Immediate second call - too soon for rates
	stats2, rates2, err := cache.GetInterfacesWithRates()
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if len(stats2) == 0 {
		t.Fatal("expected interfaces")
	}

	if len(rates2) != 0 {
		t.Error("second call (too soon) should not have rates")
	}

	// Wait for min interval
	time.Sleep(150 * time.Millisecond)

	// Third call - should have rates now
	stats3, rates3, err := cache.GetInterfacesWithRates()
	if err != nil {
		t.Fatalf("third call failed: %v", err)
	}

	if len(stats3) == 0 {
		t.Fatal("expected interfaces")
	}

	// Should have rates (unless no common interfaces)
	if len(rates3) == 0 && len(stats1) > 0 {
		t.Log("Warning: no rates calculated (might be OK if interfaces changed)")
	}
}

func BenchmarkCollectInterfaces(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := CollectInterfaces()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCalculateRates(b *testing.B) {
	prev, _ := CollectInterfaces()
	time.Sleep(10 * time.Millisecond)
	curr, _ := CollectInterfaces()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateRates(prev, curr)
	}
}
