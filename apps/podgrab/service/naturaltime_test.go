// Package service implements business logic for podcast management and downloads.
package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNatualTime tests natural language time formatting.
func TestNatualTime(t *testing.T) {
	// Use a fixed base time for consistent testing
	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		base     time.Time
		value    time.Time
		expected string
	}{
		// Past times
		{
			name:     "few_seconds_ago",
			base:     baseTime,
			value:    baseTime.Add(-30 * time.Second),
			expected: "a few seconds ago",
		},
		{
			name:     "few_minutes_ago",
			base:     baseTime,
			value:    baseTime.Add(-3 * time.Minute),
			expected: "a few minutes ago",
		},
		{
			name:     "minutes_ago",
			base:     baseTime,
			value:    baseTime.Add(-15 * time.Minute),
			expected: "15 minutes ago",
		},
		{
			name:     "hours_ago_today",
			base:     baseTime,
			value:    baseTime.Add(-5 * time.Hour),
			expected: "5 hours ago",
		},
		{
			name:     "yesterday",
			base:     baseTime,
			value:    baseTime.Add(-24 * time.Hour),
			expected: "yesterday",
		},
		{
			name:     "day_before_yesterday",
			base:     baseTime,
			value:    baseTime.Add(-48 * time.Hour),
			expected: "day before yesterday",
		},
		{
			name:     "days_ago",
			base:     baseTime,
			value:    baseTime.Add(-10 * 24 * time.Hour),
			expected: "10 days ago",
		},
		{
			name:     "last_month",
			base:     baseTime,
			value:    baseTime.Add(-35 * 24 * time.Hour),
			expected: "last month",
		},
		{
			name:     "months_ago",
			base:     baseTime,
			value:    baseTime.Add(-90 * 24 * time.Hour),
			expected: "3 months ago",
		},
		{
			name:     "last_year",
			base:     baseTime,
			value:    baseTime.Add(-370 * 24 * time.Hour),
			expected: "last year",
		},
		{
			name:     "years_ago",
			base:     baseTime,
			value:    baseTime.Add(-750 * 24 * time.Hour),
			expected: "2 years ago",
		},

		// Future times
		{
			name:     "in_few_seconds",
			base:     baseTime,
			value:    baseTime.Add(30 * time.Second),
			expected: "in a few seconds",
		},
		{
			name:     "in_few_minutes",
			base:     baseTime,
			value:    baseTime.Add(3 * time.Minute),
			expected: "in a few minutes",
		},
		{
			name:     "in_minutes",
			base:     baseTime,
			value:    baseTime.Add(15 * time.Minute),
			expected: "in 15 minutes",
		},
		{
			name:     "in_hours",
			base:     baseTime,
			value:    baseTime.Add(5 * time.Hour),
			expected: "in 5 hours",
		},
		{
			name:     "tomorrow",
			base:     baseTime,
			value:    baseTime.Add(24 * time.Hour),
			expected: "tomorrow",
		},
		{
			name:     "day_after_tomorrow",
			base:     baseTime,
			value:    baseTime.Add(48 * time.Hour),
			expected: "day after tomorrow",
		},
		{
			name:     "in_days",
			base:     baseTime,
			value:    baseTime.Add(10 * 24 * time.Hour),
			expected: "in 10 days",
		},
		{
			name:     "next_month",
			base:     baseTime,
			value:    baseTime.Add(35 * 24 * time.Hour),
			expected: "next month",
		},
		{
			name:     "in_months",
			base:     baseTime,
			value:    baseTime.Add(90 * 24 * time.Hour),
			expected: "in 3 months",
		},
		{
			name:     "next_year",
			base:     baseTime,
			value:    baseTime.Add(370 * 24 * time.Hour),
			expected: "next year",
		},
		{
			name:     "in_years",
			base:     baseTime,
			value:    baseTime.Add(750 * 24 * time.Hour),
			expected: "in 2 years",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NatualTime(tt.base, tt.value)
			assert.Equal(t, tt.expected, result, "Should format time correctly")
		})
	}
}

// TestPastNaturalTime tests past time formatting.
func TestPastNaturalTime(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		base           time.Time
		value          time.Time
		expectedResult string
	}{
		{
			name:           "within_60_seconds",
			base:           baseTime,
			value:          baseTime.Add(-45 * time.Second),
			expectedResult: "a few seconds ago",
		},
		{
			name:           "within_5_minutes",
			base:           baseTime,
			value:          baseTime.Add(-4 * time.Minute),
			expectedResult: "a few minutes ago",
		},
		{
			name:           "within_60_minutes",
			base:           baseTime,
			value:          baseTime.Add(-30 * time.Minute),
			expectedResult: "30 minutes ago",
		},
		{
			name:           "earlier_today",
			base:           baseTime,
			value:          time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC),
			expectedResult: "4 hours ago",
		},
		{
			name:           "yesterday_exact",
			base:           baseTime,
			value:          time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC),
			expectedResult: "yesterday",
		},
		{
			name:           "day_before_yesterday_exact",
			base:           baseTime,
			value:          time.Date(2024, 1, 13, 12, 0, 0, 0, time.UTC),
			expectedResult: "day before yesterday",
		},
		{
			name:           "week_ago",
			base:           baseTime,
			value:          baseTime.Add(-7 * 24 * time.Hour),
			expectedResult: "7 days ago",
		},
		{
			name:           "month_ago",
			base:           baseTime,
			value:          baseTime.Add(-32 * 24 * time.Hour),
			expectedResult: "last month",
		},
		{
			name:           "several_months_ago",
			base:           baseTime,
			value:          baseTime.Add(-120 * 24 * time.Hour),
			expectedResult: "4 months ago",
		},
		{
			name:           "year_ago",
			base:           baseTime,
			value:          baseTime.Add(-365 * 24 * time.Hour),
			expectedResult: "last year",
		},
		{
			name:           "multiple_years_ago",
			base:           baseTime,
			value:          baseTime.Add(-900 * 24 * time.Hour),
			expectedResult: "2 years ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pastNaturalTime(tt.base, tt.value)
			assert.Equal(t, tt.expectedResult, result, "Should format past time correctly")
		})
	}
}

// TestFutureNaturalTime tests future time formatting.
func TestFutureNaturalTime(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		base           time.Time
		value          time.Time
		expectedResult string
	}{
		{
			name:           "within_60_seconds",
			base:           baseTime,
			value:          baseTime.Add(45 * time.Second),
			expectedResult: "in a few seconds",
		},
		{
			name:           "within_5_minutes",
			base:           baseTime,
			value:          baseTime.Add(4 * time.Minute),
			expectedResult: "in a few minutes",
		},
		{
			name:           "within_60_minutes",
			base:           baseTime,
			value:          baseTime.Add(30 * time.Minute),
			expectedResult: "in 30 minutes",
		},
		{
			name:           "later_today",
			base:           baseTime,
			value:          baseTime.Add(8 * time.Hour),
			expectedResult: "in 8 hours",
		},
		{
			name:           "tomorrow_exact",
			base:           baseTime,
			value:          baseTime.Add(24 * time.Hour),
			expectedResult: "tomorrow",
		},
		{
			name:           "day_after_tomorrow_exact",
			base:           baseTime,
			value:          baseTime.Add(48 * time.Hour),
			expectedResult: "day after tomorrow",
		},
		{
			name:           "week_from_now",
			base:           baseTime,
			value:          baseTime.Add(7 * 24 * time.Hour),
			expectedResult: "in 7 days",
		},
		{
			name:           "month_from_now",
			base:           baseTime,
			value:          baseTime.Add(32 * 24 * time.Hour),
			expectedResult: "next month",
		},
		{
			name:           "several_months_from_now",
			base:           baseTime,
			value:          baseTime.Add(120 * 24 * time.Hour),
			expectedResult: "in 4 months",
		},
		{
			name:           "year_from_now",
			base:           baseTime,
			value:          baseTime.Add(365 * 24 * time.Hour),
			expectedResult: "next year",
		},
		{
			name:           "multiple_years_from_now",
			base:           baseTime,
			value:          baseTime.Add(900 * 24 * time.Hour),
			expectedResult: "in 2 years",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := futureNaturalTime(tt.base, tt.value)
			assert.Equal(t, tt.expectedResult, result, "Should format future time correctly")
		})
	}
}

// TestNatualTime_BoundaryConditions tests edge cases and boundary conditions.
func TestNatualTime_BoundaryConditions(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		base     time.Time
		value    time.Time
		contains string // Use contains for boundary cases that might vary slightly
	}{
		{
			name:     "exactly_60_seconds_past",
			base:     baseTime,
			value:    baseTime.Add(-60 * time.Second),
			contains: "ago",
		},
		{
			name:     "exactly_5_minutes_past",
			base:     baseTime,
			value:    baseTime.Add(-5 * time.Minute),
			contains: "minutes ago",
		},
		{
			name:     "exactly_60_minutes_past",
			base:     baseTime,
			value:    baseTime.Add(-60 * time.Minute),
			contains: "ago",
		},
		{
			name:     "exactly_24_hours_past",
			base:     baseTime,
			value:    baseTime.Add(-24 * time.Hour),
			contains: "yesterday",
		},
		{
			name:     "exactly_30_days_past",
			base:     baseTime,
			value:    baseTime.Add(-30 * 24 * time.Hour),
			contains: "month",
		},
		{
			name:     "exactly_365_days_past",
			base:     baseTime,
			value:    baseTime.Add(-365 * 24 * time.Hour),
			contains: "year",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NatualTime(tt.base, tt.value)
			assert.Contains(t, result, tt.contains, "Should contain expected time reference")
		})
	}
}

// TestNatualTime_SameTime tests handling of identical base and value times.
func TestNatualTime_SameTime(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	result := NatualTime(baseTime, baseTime)

	// Should handle same time as "a few seconds ago" or similar
	assert.NotEmpty(t, result, "Should return non-empty string for same time")
	assert.Contains(t, result, "second", "Should reference seconds for same time")
}

// TestNatualTime_LeapYear tests handling across leap year boundary.
func TestNatualTime_LeapYear(t *testing.T) {
	// Feb 29, 2024 is a leap year day
	leapDay := time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC)
	nextDay := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)

	result := NatualTime(nextDay, leapDay)

	assert.Contains(t, result, "yesterday", "Should correctly handle leap year boundary")
}

// TestNatualTime_TimezoneHandling tests time calculations across timezones.
func TestNatualTime_TimezoneHandling(t *testing.T) {
	// Create times in different timezones but representing the same instant
	utcTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	estLocation, _ := time.LoadLocation("America/New_York")
	estTime := utcTime.In(estLocation)

	result := NatualTime(utcTime, estTime)

	// Should treat as same time (few seconds difference at most)
	assert.Contains(t, result, "second", "Should handle timezone conversions correctly")
}
