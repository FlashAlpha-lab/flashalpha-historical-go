package flashalphahistorical

// Typed response model for `GET /v1/surface/{symbol}?at=...` (public).
//
// Same shape as the live API, replayed at the requested historical minute.
//
// The implied-volatility surface — a dense (moneyness × tenor) grid of IVs
// fitted from the listed option chain. May raise InsufficientDataError on
// historical days where the chain is too sparse to fit a surface.
//
// The Iv matrix is indexed [moneyness_idx][tenor_idx] — outer index walks
// Moneyness, inner index walks Tenors. SlicesUsed reports the number of
// expiry slices that survived QC and contributed to the fit.

// SurfaceResponse is the typed body of GET /v1/surface/{symbol}?at=...
type SurfaceResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// Spot is the underlying spot reference used to build the surface.
	Spot *float64 `json:"spot"`
	// AsOf is the ET wall-clock timestamp this snapshot was computed for.
	AsOf string `json:"as_of"`
	// GridSize is the integer grid resolution (e.g. 50 for a 50×50 grid).
	GridSize *int `json:"grid_size"`
	// Tenors is the tenor axis in days (matches the inner Iv index).
	Tenors []float64 `json:"tenors"`
	// Moneyness is the log-moneyness axis (matches the outer Iv index).
	Moneyness []float64 `json:"moneyness"`
	// Iv is the (moneyness × tenor) implied-vol grid (annualised %).
	Iv [][]float64 `json:"iv"`
	// SlicesUsed is the count of expiry slices that contributed to the fit.
	SlicesUsed *int `json:"slices_used"`
}
