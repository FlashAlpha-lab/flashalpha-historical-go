package flashalphahistorical

// Typed response model for `GET /v1/vrp/{symbol}?at=...` (Alpha+).
//
// Same shape as the live API with two macro diffs:
//   - HySpread: populated on historical (live currently returns nil).
//   - FedFunds: absent on historical (this struct omits it; live includes it).
//
// On historical responses with insufficient warm-up (`at` near 2018-04-16),
// Vrp.ZScore, Vrp.Percentile, Regime.VrpRegime, StrategyScores, and
// NetHarvestScore are all nil. Warnings will explain.
//
// Common silent-null traps (now type-checked at the SDK boundary):
//   - response["z_score"]    ✗  → response.Vrp.ZScore
//   - response["percentile"] ✗  → response.Vrp.Percentile
//   - response["put_vrp"]    ✗  → response.Directional.DownsideVrp
//   - response["net_gex"]    ✗  → response.Regime.NetGex
//
// Returns 403 tier_restricted for anything below Alpha plan.

// VrpResponse is the typed body of GET /v1/vrp/{symbol}?at=...
type VrpResponse struct {
	Symbol              string               `json:"symbol"`
	UnderlyingPrice     *float64             `json:"underlying_price"`
	AsOf                string               `json:"as_of"`
	MarketOpen          *bool                `json:"market_open"`
	Vrp                 *VrpCore             `json:"vrp"`
	VarianceRiskPremium *float64             `json:"variance_risk_premium"`
	ConvexityPremium    *float64             `json:"convexity_premium"`
	FairVol             *float64             `json:"fair_vol"`
	Directional         *VrpDirectional      `json:"directional"`
	TermVrp             []VrpTermItem        `json:"term_vrp"`
	GexConditioned      *VrpGexConditioned   `json:"gex_conditioned"`
	VannaConditioned    *VrpVannaConditioned `json:"vanna_conditioned"`
	// Regime snapshot block. NetGex lives HERE, not at top level.
	Regime *VrpRegime `json:"regime"`
	// nil on historical when warmup is too short.
	StrategyScores *VrpStrategyScores `json:"strategy_scores"`
	// nil on historical when warmup is too short.
	NetHarvestScore *int      `json:"net_harvest_score"`
	DealerFlowRisk  *int      `json:"dealer_flow_risk"`
	Warnings        []string  `json:"warnings"`
	Macro           *VrpMacro `json:"macro"`
}

// VrpCore is the core VRP metrics block — implied vs realized vol across
// horizons, plus z-score and percentile against a trailing window.
//
// Nested under response.Vrp — NOT top-level.
type VrpCore struct {
	AtmIv  *float64 `json:"atm_iv"`
	Rv5d   *float64 `json:"rv_5d"`
	Rv10d  *float64 `json:"rv_10d"`
	Rv20d  *float64 `json:"rv_20d"`
	Rv30d  *float64 `json:"rv_30d"`
	Vrp5d  *float64 `json:"vrp_5d"`
	Vrp10d *float64 `json:"vrp_10d"`
	Vrp20d *float64 `json:"vrp_20d"`
	Vrp30d *float64 `json:"vrp_30d"`
	// Z-score of current 20-day VRP. nil when warmup is insufficient
	// (close to 2018-04-16, the dataset start).
	ZScore *float64 `json:"z_score"`
	// Percentile rank (0-100). nil when warmup is short.
	Percentile  *int `json:"percentile"`
	HistoryDays *int `json:"history_days"`
}

// VrpDirectional is the directional VRP skew. Use DownsideVrp / UpsideVrp,
// NOT put_vrp / call_vrp (those don't exist).
type VrpDirectional struct {
	PutWingIv25d  *float64 `json:"put_wing_iv_25d"`
	CallWingIv25d *float64 `json:"call_wing_iv_25d"`
	DownsideRv20d *float64 `json:"downside_rv_20d"`
	UpsideRv20d   *float64 `json:"upside_rv_20d"`
	DownsideVrp   *float64 `json:"downside_vrp"`
	UpsideVrp     *float64 `json:"upside_vrp"`
}

// VrpTermItem is one row of the VRP term structure.
type VrpTermItem struct {
	Dte *int     `json:"dte"`
	Iv  *float64 `json:"iv"`
	Rv  *float64 `json:"rv"`
	Vrp *float64 `json:"vrp"`
}

// VrpGexConditioned is the VRP harvest score conditioned on the prevailing
// dealer-gamma regime.
type VrpGexConditioned struct {
	Regime *string `json:"regime"`
	// 0-100 composite. >70 = strong harvest signal; <30 = avoid.
	HarvestScore   *float64 `json:"harvest_score"`
	Interpretation *string  `json:"interpretation"`
}

// VrpVannaConditioned is the VRP outlook conditioned on net dealer vanna.
type VrpVannaConditioned struct {
	Outlook        *string `json:"outlook"`
	Interpretation *string `json:"interpretation"`
}

// VrpRegime is the regime snapshot block. NetGex lives HERE, not top-level.
type VrpRegime struct {
	Gamma *string `json:"gamma"`
	// nil on historical with insufficient warmup.
	VrpRegime *string  `json:"vrp_regime"`
	NetGex    *float64 `json:"net_gex"`
	GammaFlip *float64 `json:"gamma_flip"`
}

// VrpStrategyScores holds 0-100 suitability scores for canonical short-vol
// strategies. Each field can be nil on historical when inputs aren't
// computable for the given `at` timestamp.
type VrpStrategyScores struct {
	ShortPutSpread *int `json:"short_put_spread"`
	ShortStrangle  *int `json:"short_strangle"`
	IronCondor     *int `json:"iron_condor"`
	CalendarSpread *int `json:"calendar_spread"`
}

// VrpMacro is the macro-context snapshot.
//
// Historical-specific: FedFunds is absent on historical responses (this
// struct doesn't declare it). HySpread is populated here (live returns nil).
type VrpMacro struct {
	Vix          *float64 `json:"vix"`
	Vix3m        *float64 `json:"vix_3m"`
	VixTermSlope *float64 `json:"vix_term_slope"`
	Dgs10        *float64 `json:"dgs10"`
	HySpread     *float64 `json:"hy_spread"`
}
