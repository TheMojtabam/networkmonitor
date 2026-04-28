package netstat

import (
	"testing"
)

func TestCollectListeningPorts(t *testing.T) {
	ports, err := CollectListeningPorts()
	if err != nil {
		t.Fatalf("CollectListeningPorts failed: %v", err)
	}

	// System should have at least some listening ports
	if len(ports) == 0 {
		t.Log("Warning: no listening ports found (unusual)")
	}

	for _, p := range ports {
		if p.Protocol == "" {
			t.Error("port has empty protocol")
		}

		if p.LocalAddr == "" {
			t.Error("port has empty local address")
		}

		if p.LocalPort == 0 {
			t.Errorf("port %s has zero port number", p.LocalAddr)
		}

		if p.TS.IsZero() {
			t.Error("port has zero timestamp")
		}

		// Listening ports should be in LISTEN state (TCP) or state 7 (UDP)
		if p.Protocol == "tcp" || p.Protocol == "tcp6" {
			if p.State != StateListen {
				t.Errorf("TCP port %s not in LISTEN state: %d", p.LocalAddr, p.State)
			}
		}
	}
}

func TestCollectConnections(t *testing.T) {
	conns, err := CollectConnections()
	if err != nil {
		t.Fatalf("CollectConnections failed: %v", err)
	}

	// May have zero connections (system dependent)
	t.Logf("Found %d active connections", len(conns))

	for _, c := range conns {
		if c.Protocol == "" {
			t.Error("connection has empty protocol")
		}

		if c.LocalAddr == "" {
			t.Error("connection has empty local address")
		}

		if c.RemoteAddr == "" {
			t.Error("connection has empty remote address")
		}

		if c.TS.IsZero() {
			t.Error("connection has zero timestamp")
		}
	}
}

func TestCountConnectionsPerPort(t *testing.T) {
	ports, err := CollectListeningPorts()
	if err != nil {
		t.Fatalf("CollectListeningPorts failed: %v", err)
	}

	conns, err := CollectConnections()
	if err != nil {
		t.Fatalf("CollectConnections failed: %v", err)
	}

	result := CountConnectionsPerPort(ports, conns)

	if len(result) != len(ports) {
		t.Errorf("expected %d results, got %d", len(ports), len(result))
	}

	// Check that counts are non-negative
	for _, p := range result {
		if p.ConnectionCount < 0 {
			t.Errorf("negative connection count for %s: %d",
				p.LocalAddr, p.ConnectionCount)
		}
	}
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		hex      string
		wantIP   string
		wantPort uint16
	}{
		// 127.0.0.1:80 (0x0100007F:0x0050)
		{"0100007F:0050", "127.0.0.1", 80},
		
		// 0.0.0.0:443 (0x00000000:0x01BB)
		{"00000000:01BB", "0.0.0.0", 443},
		
		// 192.168.1.100:8080 (0x6401A8C0:0x1F90)
		{"6401A8C0:1F90", "192.168.1.100", 8080},
	}

	for _, tt := range tests {
		gotIP, gotPort := parseAddress(tt.hex)
		if gotIP != tt.wantIP {
			t.Errorf("parseAddress(%s) IP = %s, want %s",
				tt.hex, gotIP, tt.wantIP)
		}
		if gotPort != tt.wantPort {
			t.Errorf("parseAddress(%s) port = %d, want %d",
				tt.hex, gotPort, tt.wantPort)
		}
	}
}

func TestDecodeIP(t *testing.T) {
	tests := []struct {
		hex  string
		want string
	}{
		{"0100007F", "127.0.0.1"},       // localhost (little-endian)
		{"00000000", "0.0.0.0"},         // any address
		{"0100A8C0", "192.168.0.1"},     // 192.168.0.1
	}

	for _, tt := range tests {
		got := decodeIP(tt.hex)
		if got != tt.want {
			t.Errorf("decodeIP(%s) = %s, want %s", tt.hex, got, tt.want)
		}
	}
}

func TestSSCollector(t *testing.T) {
	collector := NewSSCollector()
	ports, err := collector.CollectWithSS()
	
	// ss command might not be available or might fail
	if err != nil {
		t.Logf("ss collector failed (expected if ss not installed): %v", err)
		return
	}

	t.Logf("ss found %d listening ports", len(ports))

	for _, p := range ports {
		if p.LocalPort == 0 {
			t.Errorf("ss port %s has zero port number", p.LocalAddr)
		}
	}
}

func BenchmarkCollectListeningPorts(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := CollectListeningPorts()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCollectConnections(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := CollectConnections()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseAddress(b *testing.B) {
	hexAddr := "0100007F:0050" // 127.0.0.1:80
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseAddress(hexAddr)
	}
}
