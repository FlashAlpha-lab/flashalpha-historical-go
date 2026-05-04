package flashalphahistorical

import "fmt"

// APIError is the base error type returned by the FlashAlpha Historical
// client. It carries the HTTP status code, a human-readable message, the
// upstream error code (e.g. "no_data", "invalid_at"), and the raw response
// body (if any) parsed as a map.
type APIError struct {
	StatusCode int
	Code       string // upstream "error" field, e.g. "no_data", "invalid_at"
	Message    string
	Response   map[string]interface{}
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("flashalpha-historical: HTTP %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("flashalpha-historical: HTTP %d: %s", e.StatusCode, e.Message)
}

// AuthenticationError — HTTP 401.
type AuthenticationError struct{ *APIError }

func (e *AuthenticationError) Error() string { return e.APIError.Error() }

// TierRestrictedError — HTTP 403. Every Historical endpoint requires Alpha+.
type TierRestrictedError struct {
	*APIError
	CurrentPlan  string
	RequiredPlan string
}

func (e *TierRestrictedError) Error() string {
	base := e.APIError.Error()
	if e.CurrentPlan != "" || e.RequiredPlan != "" {
		return fmt.Sprintf("%s (current_plan=%s, required_plan=%s)", base, e.CurrentPlan, e.RequiredPlan)
	}
	return base
}

// InvalidAtError — HTTP 400 with error="invalid_at". The `at` parameter is
// missing or has an invalid format.
type InvalidAtError struct{ *APIError }

func (e *InvalidAtError) Error() string { return e.APIError.Error() }

// NoDataError — HTTP 404 with error="no_data". The (symbol, at) tuple has no
// data — outside the coverage window or inside a known gap.
type NoDataError struct{ *APIError }

func (e *NoDataError) Error() string { return e.APIError.Error() }

// NoCoverageError — HTTP 404 with error="no_coverage". The symbol is not
// in the historical dataset.
type NoCoverageError struct{ *APIError }

func (e *NoCoverageError) Error() string { return e.APIError.Error() }

// SymbolNotFoundError — HTTP 404 with error="symbol_not_found". The symbol
// has no historical data at the requested `at`.
type SymbolNotFoundError struct{ *APIError }

func (e *SymbolNotFoundError) Error() string { return e.APIError.Error() }

// InsufficientDataError — HTTP 404 with error="insufficient_data". The
// surface grid can't be built (too few OTM+liquid contracts).
type InsufficientDataError struct{ *APIError }

func (e *InsufficientDataError) Error() string { return e.APIError.Error() }

// RateLimitError — HTTP 429. Daily quota is shared with the live API.
type RateLimitError struct {
	*APIError
	RetryAfter int
}

func (e *RateLimitError) Error() string {
	base := e.APIError.Error()
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s (retry_after=%ds)", base, e.RetryAfter)
	}
	return base
}

// ServerError — HTTP 5xx.
type ServerError struct{ *APIError }

func (e *ServerError) Error() string { return e.APIError.Error() }
