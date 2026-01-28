package guardrails

import (
	"regexp"
)

// PIIType represents the type of PII detected.
type PIIType string

const (
	PIITypeEmail      PIIType = "EMAIL"
	PIITypePhone      PIIType = "PHONE"
	PIITypeSSN        PIIType = "SSN"
	PIITypeCreditCard PIIType = "CC"
	PIITypeIPAddress  PIIType = "IP"
)

// PIIPattern defines a regex pattern for detecting PII.
type PIIPattern struct {
	Type    PIIType
	Pattern *regexp.Regexp
}

// defaultPatterns returns the built-in PII detection patterns.
func defaultPatterns() []PIIPattern {
	return []PIIPattern{
		// Email: standard email format
		{
			Type:    PIITypeEmail,
			Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
		},
		// Phone: US formats with optional country code
		// Matches: (123) 456-7890, 123-456-7890, 123.456.7890, +1 123 456 7890, etc.
		{
			Type:    PIITypePhone,
			Pattern: regexp.MustCompile(`(?:\+1[\s.-]?)?\(?\d{3}\)?[\s.\-]?\d{3}[\s.\-]?\d{4}`),
		},
		// SSN: US Social Security Number
		// Matches: 123-45-6789, 123 45 6789, 123456789
		{
			Type:    PIITypeSSN,
			Pattern: regexp.MustCompile(`\b\d{3}[\s\-]?\d{2}[\s\-]?\d{4}\b`),
		},
		// Credit Card: 16-digit card numbers with optional separators
		// Matches: 1234567890123456, 1234-5678-9012-3456, 1234 5678 9012 3456
		{
			Type:    PIITypeCreditCard,
			Pattern: regexp.MustCompile(`\b\d{4}[\s\-]?\d{4}[\s\-]?\d{4}[\s\-]?\d{4}\b`),
		},
		// IP Address: IPv4 format
		// Matches: 192.168.1.1, 10.0.0.1, etc.
		{
			Type:    PIITypeIPAddress,
			Pattern: regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`),
		},
	}
}

// getEnabledPatterns returns patterns based on detector configuration.
func getEnabledPatterns(cfg DetectorConfig) []PIIPattern {
	allPatterns := defaultPatterns()
	var enabled []PIIPattern

	for _, p := range allPatterns {
		switch p.Type {
		case PIITypeEmail:
			if cfg.Email {
				enabled = append(enabled, p)
			}
		case PIITypePhone:
			if cfg.Phone {
				enabled = append(enabled, p)
			}
		case PIITypeSSN:
			if cfg.SSN {
				enabled = append(enabled, p)
			}
		case PIITypeCreditCard:
			if cfg.CreditCard {
				enabled = append(enabled, p)
			}
		case PIITypeIPAddress:
			if cfg.IPAddress {
				enabled = append(enabled, p)
			}
		}
	}

	return enabled
}
