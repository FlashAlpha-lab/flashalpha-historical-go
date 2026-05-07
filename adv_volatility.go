package flashalphahistorical

// Typed response model for `GET /v1/adv_volatility/{symbol}?at=...` (Alpha+).
//
// Same shape as the live API, replayed at the requested historical minute.
//
// Advanced volatility analytics — the parametric model layer on top of the
// raw IV grid:
//   - SVI (stochastic volatility inspired) raw parameters per expiry,
//   - forward prices implied by put-call parity,
//   - the full total-variance and implied-vol surface grid,
//   - calendar / butterfly arbitrage flags,
//   - variance-swap fair values per expiry, and
//   - second/third-order greek surfaces (vanna, charm, volga, speed).
//
// The grid surfaces (TotalVariance, ImpliedVol, and the four greek surfaces)
// are dense float matrices indexed [moneyness_idx][expiry_idx]. Empty grids
// indicate insufficient data; per-row nullability follows the standard
// pointer convention.
//
// Requires Alpha+ plan; returns 403 tier_restricted for anything below.

// AdvVolatilityResponse is the typed body of GET /v1/adv_volatility/{symbol}?at=...
type AdvVolatilityResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid at AsOf.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// AsOf is the ET wall-clock timestamp this snapshot was computed for.
	AsOf string `json:"as_of"`
	// MarketOpen is true if NYSE was open at AsOf.
	MarketOpen *bool `json:"market_open"`
	// SviParameters is the raw-SVI fit per expiry.
	SviParameters []AdvSviParameters `json:"svi_parameters"`
	// ForwardPrices is the implied-forward / spot / basis ladder per expiry.
	ForwardPrices []AdvForwardPrice `json:"forward_prices"`
	// TotalVarianceSurface is the dense (moneyness × expiry) total-variance
	// and implied-vol surface grid.
	TotalVarianceSurface *AdvTotalVarianceSurface `json:"total_variance_surface"`
	// ArbitrageFlags lists detected calendar/butterfly arbitrage in the surface.
	ArbitrageFlags []AdvArbitrageFlag `json:"arbitrage_flags"`
	// VarianceSwapFairValues is the variance-swap pricing per expiry.
	VarianceSwapFairValues []AdvVarianceSwapFairValue `json:"variance_swap_fair_values"`
	// GreeksSurfaces is the second/third-order greek surfaces (vanna,
	// charm, volga, speed).
	GreeksSurfaces *AdvGreeksSurfaces `json:"greeks_surfaces"`
}

// AdvSviParameters is the raw-SVI fit for one expiry.
//
// The five raw-SVI parameters (a, b, rho, m, sigma) plus the implied
// at-the-money total variance and IV computed from the fit. See Gatheral
// (2004) for the parametrisation: total_variance(k) = a + b·{ρ(k-m) +
// √[(k-m)² + σ²] }.
type AdvSviParameters struct {
	// Expiry is the option expiration date (YYYY-MM-DD).
	Expiry *string `json:"expiry"`
	// DaysToExpiry is the integer days from AsOf to Expiry.
	DaysToExpiry *int `json:"days_to_expiry"`
	// Forward is the implied forward price for this expiry.
	Forward *float64 `json:"forward"`
	// A is the raw-SVI level parameter.
	A *float64 `json:"a"`
	// B is the raw-SVI overall slope parameter.
	B *float64 `json:"b"`
	// Rho is the raw-SVI skew parameter (-1 < rho < 1).
	Rho *float64 `json:"rho"`
	// M is the raw-SVI smile-centre log-moneyness shift.
	M *float64 `json:"m"`
	// Sigma is the raw-SVI smile-curvature parameter (sigma > 0).
	Sigma *float64 `json:"sigma"`
	// AtmTotalVariance is the model ATM total variance (σ²·T).
	AtmTotalVariance *float64 `json:"atm_total_variance"`
	// AtmIv is the model ATM IV (annualised %, e.g. 18.5 = 18.5%).
	AtmIv *float64 `json:"atm_iv"`
}

// AdvForwardPrice is one row of the implied-forward / spot / basis ladder.
type AdvForwardPrice struct {
	// Expiry is the option expiration date (YYYY-MM-DD).
	Expiry *string `json:"expiry"`
	// DaysToExpiry is the integer days from AsOf to Expiry.
	DaysToExpiry *int `json:"days_to_expiry"`
	// Forward is the put-call-parity implied forward price for this expiry.
	Forward *float64 `json:"forward"`
	// Spot is the underlying spot mid at AsOf.
	Spot *float64 `json:"spot"`
	// BasisPct is (Forward - Spot) / Spot * 100 — the % cost-of-carry / dividend basis.
	BasisPct *float64 `json:"basis_pct"`
}

// AdvTotalVarianceSurface is the dense (moneyness × expiry) total-variance
// and implied-vol surface.
//
// The TotalVariance and ImpliedVol matrices are float64[len(Moneyness)][len(Expiries)]
// — outer index is moneyness, inner index is expiry. nil rows / nan cells
// indicate gaps in the input grid.
type AdvTotalVarianceSurface struct {
	// Moneyness is the log-moneyness axis (k = ln(K/F)).
	Moneyness []float64 `json:"moneyness"`
	// Expiries are the expiration dates corresponding to the inner axis.
	Expiries []string `json:"expiries"`
	// Tenors are the day-count tenors for each expiry (matches Expiries).
	Tenors []int `json:"tenors"`
	// TotalVariance is the (moneyness × expiry) total-variance grid.
	TotalVariance [][]float64 `json:"total_variance"`
	// ImpliedVol is the (moneyness × expiry) implied-vol grid (annualised %).
	ImpliedVol [][]float64 `json:"implied_vol"`
}

// AdvArbitrageFlag is one detected arbitrage on the SVI/IV surface.
//
// Type is the arbitrage class (e.g. "calendar", "butterfly"); StrikeOrK is
// either a raw strike or a log-moneyness depending on the surface axis;
// Description is a server-generated explanation safe to surface verbatim.
type AdvArbitrageFlag struct {
	// Expiry is the expiration date involved.
	Expiry *string `json:"expiry"`
	// Type is the arbitrage class (e.g. "calendar", "butterfly").
	Type *string `json:"type"`
	// StrikeOrK is either a strike price or a log-moneyness location.
	StrikeOrK *float64 `json:"strike_or_k"`
	// Description is a plain-English explanation; safe to surface verbatim.
	Description *string `json:"description"`
}

// AdvVarianceSwapFairValue is one row of variance-swap fair values per expiry.
type AdvVarianceSwapFairValue struct {
	// Expiry is the option expiration date (YYYY-MM-DD).
	Expiry *string `json:"expiry"`
	// DaysToExpiry is the integer days from AsOf to Expiry.
	DaysToExpiry *int `json:"days_to_expiry"`
	// FairVariance is the model-fair variance for a synthetic variance swap.
	FairVariance *float64 `json:"fair_variance"`
	// FairVol is sqrt(FairVariance) — the breakeven implied vol for the swap.
	FairVol *float64 `json:"fair_vol"`
	// AtmIv is the at-the-money IV for this expiry (annualised %).
	AtmIv *float64 `json:"atm_iv"`
	// ConvexityAdjustment is FairVol - AtmIv — the curvature premium between
	// the IV smile and the variance-swap fair vol.
	ConvexityAdjustment *float64 `json:"convexity_adjustment"`
}

// AdvGreeksSurfaces is the bundle of second/third-order greek surfaces
// (vanna, charm, volga, speed). Each is a dense (strike × expiry) grid.
type AdvGreeksSurfaces struct {
	// Vanna is the dCalls/dSpot/dVol greek surface.
	Vanna *AdvGreeksSurface `json:"vanna"`
	// Charm is the dDelta/dT greek surface.
	Charm *AdvGreeksSurface `json:"charm"`
	// Volga is the d²Vega/dVol² greek surface.
	Volga *AdvGreeksSurface `json:"volga"`
	// Speed is the d³V/dSpot³ greek surface.
	Speed *AdvGreeksSurface `json:"speed"`
}

// AdvGreeksSurface is one greek surface — a dense (strike × expiry) grid.
//
// Values is float64[len(Strikes)][len(Expiries)] — outer index is strike,
// inner index is expiry.
type AdvGreeksSurface struct {
	// Strikes is the strike axis (raw dollar strikes).
	Strikes []float64 `json:"strikes"`
	// Expiries are the expiration dates corresponding to the inner axis.
	Expiries []string `json:"expiries"`
	// Values is the (strike × expiry) greek-value grid.
	Values [][]float64 `json:"values"`
}
