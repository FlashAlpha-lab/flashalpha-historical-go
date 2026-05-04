package flashalphahistorical

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ── unit tests — mocked HTTP only ───────────────────────────────────────────

func TestFormatAt(t *testing.T) {
	ts := time.Date(2026, 3, 5, 15, 30, 0, 0, time.UTC)
	if got := FormatAt(ts); got != "2026-03-05T15:30:00" {
		t.Fatalf("FormatAt: got %q", got)
	}
}

func TestForwardsApiKeyAndAt(t *testing.T) {
	var capturedKey, capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedKey = r.Header.Get("X-Api-Key")
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"regime":"positive_gamma"}`))
	}))
	defer srv.Close()

	c := NewClientWithURL("KEY", srv.URL)
	resp, err := c.ExposureSummary(context.Background(), "SPY", "2026-03-05T15:30:00")
	if err != nil {
		t.Fatalf("ExposureSummary: %v", err)
	}
	if resp["regime"] != "positive_gamma" {
		t.Fatalf("regime: %v", resp["regime"])
	}
	if capturedKey != "KEY" {
		t.Fatalf("X-Api-Key: %q", capturedKey)
	}
	if !strings.Contains(capturedURL, "/v1/exposure/summary/SPY") {
		t.Fatalf("path: %q", capturedURL)
	}
	if !strings.Contains(capturedURL, "at=2026-03-05T15%3A30%3A00") {
		t.Fatalf("at query missing: %q", capturedURL)
	}
}

func mockServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

func TestInvalidAtMapsToTypedError(t *testing.T) {
	srv := mockServer(http.StatusBadRequest, `{"error":"invalid_at","message":"bad"}`)
	defer srv.Close()
	c := NewClientWithURL("KEY", srv.URL)

	_, err := c.ExposureSummary(context.Background(), "SPY", "garbage")
	var iae *InvalidAtError
	if !errors.As(err, &iae) {
		t.Fatalf("expected InvalidAtError, got %T: %v", err, err)
	}
}

func TestNoDataMapsToTypedError(t *testing.T) {
	srv := mockServer(http.StatusNotFound, `{"error":"no_data","message":"outside coverage"}`)
	defer srv.Close()
	c := NewClientWithURL("KEY", srv.URL)

	_, err := c.ExposureSummary(context.Background(), "SPY", "2017-01-01")
	var ndErr *NoDataError
	if !errors.As(err, &ndErr) {
		t.Fatalf("expected NoDataError, got %T: %v", err, err)
	}
}

func TestNoCoverageMapsToTypedError(t *testing.T) {
	srv := mockServer(http.StatusNotFound, `{"error":"no_coverage"}`)
	defer srv.Close()
	c := NewClientWithURL("KEY", srv.URL)

	_, err := c.Tickers(context.Background(), "ZZZZZ")
	var ncErr *NoCoverageError
	if !errors.As(err, &ncErr) {
		t.Fatalf("expected NoCoverageError, got %T: %v", err, err)
	}
}

func TestInsufficientDataMapsToTypedError(t *testing.T) {
	srv := mockServer(http.StatusNotFound, `{"error":"insufficient_data"}`)
	defer srv.Close()
	c := NewClientWithURL("KEY", srv.URL)

	_, err := c.Surface(context.Background(), "SPY", "2018-04-16")
	var idErr *InsufficientDataError
	if !errors.As(err, &idErr) {
		t.Fatalf("expected InsufficientDataError, got %T: %v", err, err)
	}
}

func TestSymbolNotFoundMapsToTypedError(t *testing.T) {
	srv := mockServer(http.StatusNotFound, `{"error":"symbol_not_found"}`)
	defer srv.Close()
	c := NewClientWithURL("KEY", srv.URL)

	_, err := c.StockQuote(context.Background(), "XYZ", "2024-01-02")
	var snfErr *SymbolNotFoundError
	if !errors.As(err, &snfErr) {
		t.Fatalf("expected SymbolNotFoundError, got %T: %v", err, err)
	}
}

func TestTierRestrictedCarriesPlanFields(t *testing.T) {
	srv := mockServer(http.StatusForbidden,
		`{"error":"tier_restricted","current_plan":"Growth","required_plan":"Alpha","message":"needs Alpha"}`)
	defer srv.Close()
	c := NewClientWithURL("KEY", srv.URL)

	_, err := c.ExposureSummary(context.Background(), "SPY", "2026-03-05")
	var trErr *TierRestrictedError
	if !errors.As(err, &trErr) {
		t.Fatalf("expected TierRestrictedError, got %T: %v", err, err)
	}
	if trErr.CurrentPlan != "Growth" || trErr.RequiredPlan != "Alpha" {
		t.Fatalf("plan fields: cur=%q req=%q", trErr.CurrentPlan, trErr.RequiredPlan)
	}
}

func TestAuthenticationMapsToTypedError(t *testing.T) {
	srv := mockServer(http.StatusUnauthorized, ``)
	defer srv.Close()
	c := NewClientWithURL("BAD", srv.URL)

	_, err := c.Tickers(context.Background(), "")
	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthenticationError, got %T: %v", err, err)
	}
}

func TestOptionQuotePassesAllFilters(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"strike":680,"type":"C"}`))
	}))
	defer srv.Close()

	c := NewClientWithURL("KEY", srv.URL)
	_, err := c.OptionQuote(context.Background(), "SPY", "2026-03-05T15:30:00",
		WithExpiry("2026-03-06"), WithStrike(680), WithType("C"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"strike=680", "type=C", "expiry=2026-03-06"} {
		if !strings.Contains(capturedURL, want) {
			t.Errorf("missing %q in %q", want, capturedURL)
		}
	}
}

// ── replay unit tests ────────────────────────────────────────────────────────

func TestIsTradingDay(t *testing.T) {
	cases := []struct {
		date string
		want bool
	}{
		{"2024-01-02", true},
		{"2024-01-06", false}, // Sat
		{"2024-01-07", false}, // Sun
		{"2024-01-01", false}, // New Year
		{"2024-12-25", false}, // Christmas
		{"2024-07-04", false}, // July 4
	}
	for _, tc := range cases {
		d, _ := time.Parse(AtFormatDate, tc.date)
		if got := IsTradingDay(d); got != tc.want {
			t.Errorf("IsTradingDay(%s) = %v, want %v", tc.date, got, tc.want)
		}
	}
}

func TestIterDaysSkipsWeekendsAndHolidays(t *testing.T) {
	start, _ := time.Parse(AtFormatDate, "2024-01-01")
	end, _ := time.Parse(AtFormatDate, "2024-01-08")
	days := IterDays(start, end)
	want := []string{"2024-01-02", "2024-01-03", "2024-01-04", "2024-01-05", "2024-01-08"}
	if len(days) != len(want) {
		t.Fatalf("got %d days, want %d", len(days), len(want))
	}
	for i, d := range days {
		if d.Format(AtFormatDate) != want[i] {
			t.Errorf("day[%d] = %s, want %s", i, d.Format(AtFormatDate), want[i])
		}
		if d.Hour() != 16 || d.Minute() != 0 {
			t.Errorf("day[%d] = %s, want close stamp at 16:00", i, d.Format(time.RFC3339))
		}
	}
}

func TestIterMinutes391Stamps(t *testing.T) {
	d, _ := time.Parse(AtFormatDate, "2024-01-02")
	m := IterMinutes(d, d, 1)
	if len(m) != 391 {
		t.Fatalf("got %d minutes, want 391", len(m))
	}
}

func TestIterMinutes30MinStep(t *testing.T) {
	d, _ := time.Parse(AtFormatDate, "2024-01-02")
	m := IterMinutes(d, d, 30)
	if len(m) != 14 {
		t.Fatalf("got %d minutes, want 14", len(m))
	}
}

func TestIterMinutesPanicsOnZeroStep(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on zero step")
		}
	}()
	d, _ := time.Parse(AtFormatDate, "2024-01-02")
	IterMinutes(d, d, 0)
}
