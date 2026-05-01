// Package geoip wraps MaxMind GeoLite2 databases for country + ASN lookups.
// Returns empty strings on lookup failure or when disabled.
package geoip

import (
	"net"
	"sync"

	"github.com/oschwald/maxminddb-golang"

	"github.com/mojtaba/portsleuth/backend/internal/config"
)

// Enricher implements sampler.GeoEnricher.
type Enricher struct {
	mu      sync.RWMutex
	country *maxminddb.Reader
	asn     *maxminddb.Reader
	enabled bool
}

// Open loads the databases. Returns a no-op enricher if disabled or paths empty.
func Open(cfg config.GeoIPConfig) (*Enricher, error) {
	e := &Enricher{enabled: cfg.Enabled}
	if !cfg.Enabled {
		return e, nil
	}
	if cfg.CountryDB != "" {
		r, err := maxminddb.Open(cfg.CountryDB)
		if err != nil {
			return e, err
		}
		e.country = r
	}
	if cfg.ASNDB != "" {
		r, err := maxminddb.Open(cfg.ASNDB)
		if err != nil {
			return e, err
		}
		e.asn = r
	}
	return e, nil
}

// Close releases the databases.
func (e *Enricher) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.country != nil {
		_ = e.country.Close()
	}
	if e.asn != nil {
		_ = e.asn.Close()
	}
	return nil
}

// Enrich returns (country_iso, asn_string) for an IP. Empty strings on miss.
func (e *Enricher) Enrich(ip string) (string, string) {
	if !e.enabled {
		return "", ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || isPrivate(parsed) {
		return "", ""
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	var country, asn string
	if e.country != nil {
		var rec struct {
			Country struct {
				ISOCode string `maxminddb:"iso_code"`
			} `maxminddb:"country"`
		}
		if err := e.country.Lookup(parsed, &rec); err == nil {
			country = rec.Country.ISOCode
		}
	}
	if e.asn != nil {
		var rec struct {
			AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
			AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
		}
		if err := e.asn.Lookup(parsed, &rec); err == nil && rec.AutonomousSystemNumber > 0 {
			asn = "AS" + itoa(int(rec.AutonomousSystemNumber))
		}
	}
	return country, asn
}

// isPrivate returns true for RFC1918, loopback, and unique-local IPv6.
func isPrivate(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, cidr := range privateBlocks {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

var privateBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"100.64.0.0/10", "169.254.0.0/16",
		"fc00::/7", "fe80::/10",
	} {
		_, block, _ := net.ParseCIDR(cidr)
		if block != nil {
			privateBlocks = append(privateBlocks, block)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
