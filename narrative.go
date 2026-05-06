package flashalphahistorical

// Typed response model for `GET /v1/exposure/narrative/{symbol}?at=...` (Growth+).
//
// Point-in-time replay of FlashAlpha's "LLM-friendly" verbal-output endpoint.
// Every string in the Narrative block is server-generated from the numeric
// exposures at the historical minute and is SAFE TO SURFACE VERBATIM in
// customer-facing UIs (backtest reports, replays, training data for LLM tools).
//
// AsOf is snapped to the available minute — may differ from the requested
// `at` value when the request lands inside a gap.

// NarrativeResponse is the typed body of GET /v1/exposure/narrative/{symbol}?at=...
type NarrativeResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid in dollars at the as-of minute.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// AsOf is the ET wall-clock timestamp the API actually used (snapped to
	// the available minute).
	AsOf string `json:"as_of"`
	// Narrative is the verbal-output block plus the underlying Data numerics.
	Narrative *NarrativeBlock `json:"narrative"`
}

// NarrativeBlock is the verbal-output block — all strings safe to surface verbatim.
type NarrativeBlock struct {
	// Regime is the dealer-positioning narrative.
	Regime string `json:"regime"`
	// GexChange is the day-over-day GEX change narrative.
	GexChange string `json:"gex_change"`
	// KeyLevels is the call-wall / put-wall / gamma-flip narrative.
	KeyLevels string `json:"key_levels"`
	// Flow is the OI / volume / flow narrative.
	Flow string `json:"flow"`
	// Vanna is the dealer-vanna narrative.
	Vanna string `json:"vanna"`
	// Charm is the dealer-charm narrative.
	Charm string `json:"charm"`
	// ZeroDte is the same-day-expiry narrative.
	ZeroDte string `json:"zero_dte"`
	// Outlook is the forward-looking narrative.
	Outlook string `json:"outlook"`
	// Data is the underlying numerics used to author the narratives.
	Data *NarrativeData `json:"data"`
}

// NarrativeData is the numerics block backing the narrative strings.
type NarrativeData struct {
	// NetGex is net dealer gamma exposure ($/1% spot move) at AsOf.
	NetGex *float64 `json:"net_gex"`
	// NetGexPrior is the prior-session-close net dealer GEX.
	NetGexPrior *float64 `json:"net_gex_prior"`
	// NetGexChangePct is the percent change vs prior session close.
	NetGexChangePct *float64 `json:"net_gex_change_pct"`
	// Vix is the CBOE VIX index level at the as-of minute.
	Vix *float64 `json:"vix"`
	// GammaFlip is the strike where net dealer gamma crosses zero.
	GammaFlip *float64 `json:"gamma_flip"`
	// CallWall is the strike with the largest absolute call GEX.
	CallWall *float64 `json:"call_wall"`
	// PutWall is the strike with the largest absolute put GEX.
	PutWall *float64 `json:"put_wall"`
	// Regime is the dealer-positioning classifier:
	//   "positive_gamma" | "negative_gamma" | "neutral" | "undetermined"
	Regime string `json:"regime"`
	// ZeroDtePct is the 0DTE share of full-chain net GEX (%).
	ZeroDtePct *float64 `json:"zero_dte_pct"`
	// TopOiChanges is the per-strike top of OI changes vs the prior session.
	TopOiChanges []NarrativeOiChange `json:"top_oi_changes"`
}

// NarrativeOiChange is one row in NarrativeData.TopOiChanges.
type NarrativeOiChange struct {
	// Strike is the strike price.
	Strike *float64 `json:"strike"`
	// Type is "call" or "put".
	Type string `json:"type"`
	// OiChange is the change in OI vs the prior session close (signed).
	OiChange *int `json:"oi_change"`
	// Volume is the session volume at this strike+type.
	Volume *int `json:"volume"`
}
