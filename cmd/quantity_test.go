package main

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestParseCPUQuantity(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"2", 2},
		{"250m", 0.25},
		{"0.5", 0.5},
		{"1000m", 1},
		{"0", 0},
		{"  500m ", 0.5},
	}
	for _, c := range cases {
		got := parseCPUQuantity(c.in)
		if !almostEqual(got, c.want) {
			t.Errorf("parseCPUQuantity(%q) = %v, want %v", c.in, got, c.want)
		}
	}
	// Error cases.
	if got := parseCPUQuantity(""); got != -1 {
		t.Errorf("parseCPUQuantity(\"\") = %v, want -1", got)
	}
	if got := parseCPUQuantity("abc"); got != -1 {
		t.Errorf("parseCPUQuantity(\"abc\") = %v, want -1", got)
	}
}

func TestParseMemQuantity(t *testing.T) {
	const (
		ki = 1024
		mi = 1024 * 1024
		gi = 1024 * 1024 * 1024
		ti = 1024 * 1024 * 1024 * 1024
	)
	cases := []struct {
		in   string
		want float64
	}{
		{"8Gi", 8 * gi},
		{"1600Mi", 1600 * mi},
		{"1048576Ki", 1048576 * ki},
		{"1Ti", ti},
		{"1000", 1000},         // plain bytes
		{"1073741824", 1 * gi}, // decimal bytes
		{"1G", 1e9},
		{"512M", 512 * 1e6},
		{"100K", 100 * 1e3},
	}
	for _, c := range cases {
		got := parseMemQuantity(c.in)
		if !almostEqual(got, c.want) {
			t.Errorf("parseMemQuantity(%q) = %v, want %v", c.in, got, c.want)
		}
	}
	if got := parseMemQuantity(""); got != -1 {
		t.Errorf("parseMemQuantity(\"\") = %v, want -1", got)
	}
	if got := parseMemQuantity("abc"); got != -1 {
		t.Errorf("parseMemQuantity(\"abc\") = %v, want -1", got)
	}
}

func TestParseNanoCPU(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"72213343n", 72.213343}, // nano -> milli
		{"1000m", 1000},
		{"500u", 0.5}, // micro -> milli
		{"2", 2000},   // cores -> milli
		{"0", 0},
	}
	for _, c := range cases {
		got := parseNanoCPU(c.in)
		if !almostEqual(got, c.want) {
			t.Errorf("parseNanoCPU(%q) = %v, want %v", c.in, got, c.want)
		}
	}
	if got := parseNanoCPU(""); got != -1 {
		t.Errorf("parseNanoCPU(\"\") = %v, want -1", got)
	}
}
