package flashalphahistorical

// Typed response model for `GET /v1/exposure/levels/{symbol}?at=...`.
//
// Compact view of the most-watched dealer-exposure levels for an underlying
// at the requested historical minute: gamma flip, the strikes with the
// largest +/- net GEX, the call/put walls, the highest-OI strike, and the
// 0DTE magnet.
//
// AsOf is snapped to the available minute.

// LevelsResponse is the typed body of GET /v1/exposure/levels/{symbol}?at=...
type LevelsResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid in dollars at the as-of minute.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// AsOf is the ET wall-clock timestamp the API actually used (snapped to
	// the available minute).
	AsOf string `json:"as_of"`
	// Levels is the key-strike block.
	Levels *LevelsBlock `json:"levels"`
}

// LevelsBlock is the key-strike block.
type LevelsBlock struct {
	// GammaFlip is the strike where net dealer gamma crosses zero.
	GammaFlip *float64 `json:"gamma_flip"`
	// MaxPositiveGamma is the strike with the largest positive net GEX.
	MaxPositiveGamma *float64 `json:"max_positive_gamma"`
	// MaxNegativeGamma is the strike with the largest negative net GEX.
	MaxNegativeGamma *float64 `json:"max_negative_gamma"`
	// CallWall is the strike with the largest absolute call GEX
	// (dealer-side resistance).
	CallWall *float64 `json:"call_wall"`
	// PutWall is the strike with the largest absolute put GEX
	// (dealer-side support).
	PutWall *float64 `json:"put_wall"`
	// HighestOiStrike is the strike with the largest total OI (calls + puts).
	HighestOiStrike *float64 `json:"highest_oi_strike"`
	// ZeroDteMagnet is the single 0DTE strike with the largest absolute
	// dealer GEX. Nil when no 0DTE chain was active at the as-of minute.
	ZeroDteMagnet *float64 `json:"zero_dte_magnet"`
}
