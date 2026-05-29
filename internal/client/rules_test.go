package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRuleEngine(t *testing.T) {
	r := NewRuleEngine([]string{"example.com", "*.test.com"}, true)
	assert.NotNil(t, r)
	assert.True(t, r.defaultDirect)
}

func TestShouldDirectConnect_DomainMatch(t *testing.T) {
	r := NewRuleEngine([]string{"example.com", "*.test.com"}, false)

	tests := []struct {
		name     string
		addr     string
		expected bool
	}{
		{"exact match", "example.com:80", true},
		{"wildcard subdomain", "sub.test.com:443", true},
		{"deep subdomain", "a.b.test.com:80", true},
		{"no match", "other.com:80", false},
		{"default direct true", "any.com:80", true},
		{"IP address", "192.168.1.1:8080", false},
		{"invalid addr uses default", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set defaultDirect based on test name
			if tt.name == "default direct true" {
				r.SetDefaultDirect(true)
			} else {
				r.SetDefaultDirect(false)
			}
			result := r.ShouldDirectConnect(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldDirectConnect_DotPrefix(t *testing.T) {
	r := NewRuleEngine([]string{".example.com"}, false)
	assert.True(t, r.ShouldDirectConnect("sub.example.com:80"))
	assert.True(t, r.ShouldDirectConnect("a.b.example.com:443"))
	assert.False(t, r.ShouldDirectConnect("other.com:80"))
}

func TestAddDomainRule(t *testing.T) {
	r := NewRuleEngine(nil, false)

	// Before adding rule
	assert.False(t, r.ShouldDirectConnect("newdomain.com:80"))

	// Add rule
	r.AddDomainRule("newdomain.com")
	assert.True(t, r.ShouldDirectConnect("newdomain.com:80"))
}

func TestSetDefaultDirect(t *testing.T) {
	r := NewRuleEngine(nil, false)
	assert.False(t, r.ShouldDirectConnect("any.com:80"))

	r.SetDefaultDirect(true)
	assert.True(t, r.ShouldDirectConnect("any.com:80"))

	r.SetDefaultDirect(false)
	assert.False(t, r.ShouldDirectConnect("any.com:80"))
}

func TestMatchDomain(t *testing.T) {
	tests := []struct {
		domain   string
		rule     string
		expected bool
	}{
		{"sub.example.com", "*.example.com", true},
		{"example.com", "*.example.com", false},
		{"sub.example.com", ".example.com", true},
		{"example.com", ".example.com", false},
		{"example.com", "example.com", true},
		{"other.com", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain+"_"+tt.rule, func(t *testing.T) {
			assert.Equal(t, tt.expected, matchDomain(tt.domain, tt.rule))
		})
	}
}
