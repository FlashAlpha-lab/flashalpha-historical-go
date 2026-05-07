package flashalphahistorical

// Typed response models for the per-strike exposure endpoints, replayed at a
// historical minute:
//
//   - GET /v1/exposure/gex/{symbol}?at=...   — gamma exposure (Gex)
//   - GET /v1/exposure/dex/{symbol}?at=...   — delta exposure (Dex)
//   - GET /v1/exposure/vex/{symbol}?at=...   — vanna exposure (Vex)
//   - GET /v1/exposure/chex/{symbol}?at=...  — charm exposure (Chex)
//
// Each endpoint returns a per-strike breakdown of dealer-net greek exposure
// in dollars (gamma / delta / vanna / charm) plus a top-level net total. Use
// the matching *Typed wrapper on Client to decode into one of the typed
// response structs below; the original map-returning methods continue to
// work unchanged.
//
// Sign conventions:
//   - All net_* fields and per-strike net_* fields are signed: positive =
//     dealers long the greek, negative = short.
//   - call_*/put_* fields are also signed using the same dealer-net convention.
//
// Historical-specific: per-strike volume / OI-change fields on Gex
// reflect the minute-resolution snapshot (volume may be 0 for sparse-volume
// names; OI deltas are computed against the prior session close).

// ── GEX ──────────────────────────────────────────────────────────────────────

// GexResponse is the typed body of GET /v1/exposure/gex/{symbol}?at=...
type GexResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid at AsOf.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// AsOf is the ET wall-clock timestamp this snapshot was computed for.
	AsOf string `json:"as_of"`
	// GammaFlip is the strike where net dealer gamma crosses zero.
	GammaFlip *float64 `json:"gamma_flip"`
	// NetGex is the chain-wide net dealer gamma in dollars per 1% spot move.
	NetGex *float64 `json:"net_gex"`
	// NetGexLabel is a verbal classification of the net regime (e.g.
	// "positive_gamma", "negative_gamma"). Safe to surface verbatim.
	NetGexLabel *string `json:"net_gex_label"`
	// Strikes is the per-strike GEX breakdown.
	Strikes []GexStrike `json:"strikes"`
}

// GexStrike is one row of the per-strike gamma-exposure breakdown.
type GexStrike struct {
	// Strike is the option strike price.
	Strike *float64 `json:"strike"`
	// CallGex is the dealer-net gamma contributed by calls at this strike
	// (signed, dollars per 1% spot move).
	CallGex *float64 `json:"call_gex"`
	// PutGex is the dealer-net gamma contributed by puts at this strike.
	PutGex *float64 `json:"put_gex"`
	// NetGex is CallGex + PutGex.
	NetGex *float64 `json:"net_gex"`
	// CallOi is the call open interest at this strike.
	CallOi *int `json:"call_oi"`
	// PutOi is the put open interest at this strike.
	PutOi *int `json:"put_oi"`
	// CallVolume is the call volume at this strike.
	CallVolume *int `json:"call_volume"`
	// PutVolume is the put volume at this strike.
	PutVolume *int `json:"put_volume"`
	// CallOiChange is the day-over-day change in call OI at this strike.
	CallOiChange *int `json:"call_oi_change"`
	// PutOiChange is the day-over-day change in put OI at this strike.
	PutOiChange *int `json:"put_oi_change"`
}

// ── DEX ──────────────────────────────────────────────────────────────────────

// DexResponse is the typed body of GET /v1/exposure/dex/{symbol}?at=...
type DexResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid at AsOf.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// AsOf is the ET wall-clock timestamp this snapshot was computed for.
	AsOf string `json:"as_of"`
	// NetDex is the chain-wide net dealer delta in dollars (signed).
	NetDex *float64 `json:"net_dex"`
	// Strikes is the per-strike DEX breakdown.
	Strikes []DexStrike `json:"strikes"`
}

// DexStrike is one row of the per-strike delta-exposure breakdown.
type DexStrike struct {
	// Strike is the option strike price.
	Strike *float64 `json:"strike"`
	// CallDex is the dealer-net delta contributed by calls at this strike
	// (signed, dollars).
	CallDex *float64 `json:"call_dex"`
	// PutDex is the dealer-net delta contributed by puts at this strike.
	PutDex *float64 `json:"put_dex"`
	// NetDex is CallDex + PutDex.
	NetDex *float64 `json:"net_dex"`
}

// ── VEX (vanna) ──────────────────────────────────────────────────────────────

// VexResponse is the typed body of GET /v1/exposure/vex/{symbol}?at=...
type VexResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid at AsOf.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// AsOf is the ET wall-clock timestamp this snapshot was computed for.
	AsOf string `json:"as_of"`
	// NetVex is the chain-wide net dealer vanna in dollars (signed).
	NetVex *float64 `json:"net_vex"`
	// VexInterpretation is a plain-English summary of the vanna regime.
	// Safe to surface verbatim.
	VexInterpretation *string `json:"vex_interpretation"`
	// Strikes is the per-strike VEX breakdown.
	Strikes []VexStrike `json:"strikes"`
}

// VexStrike is one row of the per-strike vanna-exposure breakdown.
type VexStrike struct {
	// Strike is the option strike price.
	Strike *float64 `json:"strike"`
	// CallVex is the dealer-net vanna contributed by calls at this strike
	// (signed, dollars).
	CallVex *float64 `json:"call_vex"`
	// PutVex is the dealer-net vanna contributed by puts at this strike.
	PutVex *float64 `json:"put_vex"`
	// NetVex is CallVex + PutVex.
	NetVex *float64 `json:"net_vex"`
}

// ── CHEX (charm) ─────────────────────────────────────────────────────────────

// ChexResponse is the typed body of GET /v1/exposure/chex/{symbol}?at=...
type ChexResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid at AsOf.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// AsOf is the ET wall-clock timestamp this snapshot was computed for.
	AsOf string `json:"as_of"`
	// NetChex is the chain-wide net dealer charm in dollars (signed).
	NetChex *float64 `json:"net_chex"`
	// ChexInterpretation is a plain-English summary of the charm regime.
	// Safe to surface verbatim.
	ChexInterpretation *string `json:"chex_interpretation"`
	// Strikes is the per-strike CHEX breakdown.
	Strikes []ChexStrike `json:"strikes"`
}

// ChexStrike is one row of the per-strike charm-exposure breakdown.
type ChexStrike struct {
	// Strike is the option strike price.
	Strike *float64 `json:"strike"`
	// CallChex is the dealer-net charm contributed by calls at this strike
	// (signed, dollars).
	CallChex *float64 `json:"call_chex"`
	// PutChex is the dealer-net charm contributed by puts at this strike.
	PutChex *float64 `json:"put_chex"`
	// NetChex is CallChex + PutChex.
	NetChex *float64 `json:"net_chex"`
}
