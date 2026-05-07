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
//
// The variance risk premium dashboard — the spread between IMPLIED vol
// (forward-looking, priced into options) and REALIZED vol (backward-looking,
// observed). Positive VRP = options pricing more vol than the underlying
// actually moved → premium for selling vol. Negative = premium for buying.
//
// Every nested block exists for a reason — core metrics (Vrp), directional
// skew (Directional), gamma conditioning (GexConditioned), vanna conditioning
// (VannaConditioned), regime snapshot (Regime), strategy scores
// (StrategyScores), and macro context (Macro) are deliberately separated.
type VrpResponse struct {
	// Echoed from the request path.
	Symbol string `json:"symbol"`
	// Spot mid at the as-of minute.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// ET wall-clock timestamp this snapshot was computed for.
	AsOf string `json:"as_of"`
	// True if NYSE was open at the as-of minute.
	MarketOpen *bool `json:"market_open"`
	// Core VRP metrics block. NetGex is NOT here — see Regime.
	Vrp *VrpCore `json:"vrp"`
	// vrp_20d / 100, expressed as a decimal for variance-swap calculations.
	VarianceRiskPremium *float64 `json:"variance_risk_premium"`
	// fair_vol - atm_iv. Curvature premium between the IV smile and the
	// variance-swap fair vol.
	ConvexityPremium *float64 `json:"convexity_premium"`
	// Variance-swap fair vol — breakeven implied vol for a synthetic
	// variance swap on this name (annualised %).
	FairVol *float64 `json:"fair_vol"`
	// Directional VRP skew (downside vs upside). See VrpDirectional.
	Directional *VrpDirectional `json:"directional"`
	// Term structure — VRP at multiple DTE buckets. Empty when surface
	// fitting fails.
	TermVrp []VrpTermItem `json:"term_vrp"`
	// VRP harvest score conditioned on the prevailing dealer-gamma regime.
	GexConditioned *VrpGexConditioned `json:"gex_conditioned"`
	// VRP outlook conditioned on net dealer vanna exposure.
	VannaConditioned *VrpVannaConditioned `json:"vanna_conditioned"`
	// Regime snapshot block. NetGex lives HERE, not at top level.
	Regime *VrpRegime `json:"regime"`
	// 0-100 strategy suitability scores. nil on historical when warmup is too short.
	StrategyScores *VrpStrategyScores `json:"strategy_scores"`
	// 0-100 composite — overall harvest signal. nil on historical when
	// warmup is too short.
	NetHarvestScore *int `json:"net_harvest_score"`
	// 0-100 — risk that dealer hedging flow disrupts a short-vol harvest
	// (negative-gamma cascade, vanna-driven sell into vol spike, etc.).
	DealerFlowRisk *int `json:"dealer_flow_risk"`
	// Server-side warnings about data quality / regime instability.
	// Always present (possibly empty). E.g. "insufficient_history_for_zscore".
	Warnings []string `json:"warnings"`
	// Macro context.
	Macro *VrpMacro `json:"macro"`
}

// VrpCore is the core VRP metrics block — implied vs realized vol across
// horizons, plus z-score and percentile against a trailing window.
//
// Nested under response.Vrp — NOT top-level. response["z_score"] is a
// silent-null trap; use response.Vrp.ZScore.
type VrpCore struct {
	// At-the-money implied volatility (annualised %, e.g. 18.5 = 18.5%).
	AtmIv *float64 `json:"atm_iv"`
	// Realized vol over trailing 5 trading days (annualised %).
	Rv5d *float64 `json:"rv_5d"`
	// Realized vol over trailing 10 trading days (annualised %).
	Rv10d *float64 `json:"rv_10d"`
	// Realized vol over trailing 20 trading days (annualised %).
	Rv20d *float64 `json:"rv_20d"`
	// Realized vol over trailing 30 trading days (annualised %).
	Rv30d *float64 `json:"rv_30d"`
	// 5-day VRP: AtmIv - Rv5d. Positive = IV rich vs realised → premium for selling vol.
	Vrp5d *float64 `json:"vrp_5d"`
	// 10-day VRP: AtmIv - Rv10d.
	Vrp10d *float64 `json:"vrp_10d"`
	// 20-day VRP: AtmIv - Rv20d. The headline number; ZScore/Percentile measure this.
	Vrp20d *float64 `json:"vrp_20d"`
	// 30-day VRP: AtmIv - Rv30d.
	Vrp30d *float64 `json:"vrp_30d"`
	// Z-score of current 20-day VRP vs trailing window. +2.0 = unusually
	// rich (often a fade signal). nil when warmup is insufficient (close
	// to 2018-04-16, the dataset start).
	ZScore *float64 `json:"z_score"`
	// Percentile rank (0-100) within the trailing window. 100 = highest
	// VRP in living memory; 0 = lowest. nil when warmup is short.
	Percentile *int `json:"percentile"`
	// Trading days in the trailing percentile/z-score window. When this is
	// small (< ~30), treat ZScore and Percentile as noise.
	HistoryDays *int `json:"history_days"`
}

// VrpDirectional is the directional VRP skew. Use DownsideVrp / UpsideVrp,
// NOT put_vrp / call_vrp (those don't exist on this response).
//
// Splits the variance risk premium by direction: DOWNSIDE (puts) vs UPSIDE
// (calls). Large DownsideVrp with small UpsideVrp is the classic "expensive
// crash insurance" pattern — premium for selling puts in calm tape.
type VrpDirectional struct {
	// IV at the 25-delta put wing (bottom-tail crash insurance pricing).
	PutWingIv25d *float64 `json:"put_wing_iv_25d"`
	// IV at the 25-delta call wing (top-tail upside insurance pricing).
	CallWingIv25d *float64 `json:"call_wing_iv_25d"`
	// Realized vol of the DOWNSIDE-only return distribution (negative spot
	// returns over trailing 20 days, semi-deviation).
	DownsideRv20d *float64 `json:"downside_rv_20d"`
	// Realized vol of the UPSIDE-only return distribution.
	UpsideRv20d *float64 `json:"upside_rv_20d"`
	// PutWingIv25d - DownsideRv20d. Positive = downside crash protection
	// priced richer than the actual downside RV → premium for short-put /
	// short-strangle harvest.
	DownsideVrp *float64 `json:"downside_vrp"`
	// CallWingIv25d - UpsideRv20d. Positive = upside calls priced rich →
	// premium for short-call / covered-call harvest.
	UpsideVrp *float64 `json:"upside_vrp"`
}

// VrpTermItem is one row of the VRP term structure — an (DTE, IV, RV, VRP) tuple.
type VrpTermItem struct {
	// Days to expiry for this row (e.g. 7, 14, 30, 60, 90).
	Dte *int `json:"dte"`
	// Implied vol at this tenor (annualised %).
	Iv *float64 `json:"iv"`
	// Realized vol over a window matched to the tenor (annualised %).
	Rv *float64 `json:"rv"`
	// Tenor-matched VRP: iv - rv for this DTE bucket.
	Vrp *float64 `json:"vrp"`
}

// VrpGexConditioned is the VRP harvest score conditioned on the prevailing
// dealer-gamma regime. The same VRP number means very different things
// depending on whether dealers are long or short gamma:
//   - Long gamma + rich VRP = mean-reverting tape with rich vol → ideal harvest.
//   - Short gamma + rich VRP = trending tape with rich vol → harvest is dangerous.
type VrpGexConditioned struct {
	// Gamma regime at this snapshot. "positive_gamma" | "negative_gamma" | "neutral".
	Regime *string `json:"regime"`
	// 0-100 composite — how favourable the current VRP is to harvest GIVEN
	// the gamma regime. >70 = strong harvest signal; <30 = avoid.
	HarvestScore *float64 `json:"harvest_score"`
	// Plain-English explanation. Safe to surface verbatim.
	Interpretation *string `json:"interpretation"`
}

// VrpVannaConditioned is the VRP outlook conditioned on net dealer vanna.
//
// Dealer vanna determines how the dealer hedge book responds to a vol
// shock. Positive vanna + spike in VIX = dealers buy stock (supportive);
// negative vanna + VIX spike = dealers sell (cascade risk).
type VrpVannaConditioned struct {
	// Forward-looking outlook label (e.g. "vanna_supportive",
	// "vanna_cascade_risk", "vanna_neutral").
	Outlook *string `json:"outlook"`
	// Plain-English narrative for the vanna outlook.
	Interpretation *string `json:"interpretation"`
}

// VrpRegime is the regime snapshot block. NetGex lives HERE, not top-level.
//
// Customers often expect response.NetGex (top-level) — that's a nil pointer.
// Use response.Regime.NetGex.
type VrpRegime struct {
	// "positive_gamma" | "negative_gamma" | "unknown".
	Gamma *string `json:"gamma"`
	// "harvestable" | "selling_too_cheap" | "buying_too_cheap" | "neutral" etc.
	// nil on historical with insufficient warmup.
	VrpRegime *string `json:"vrp_regime"`
	// Net dealer gamma exposure in dollars per 1% spot move. Same definition
	// as exposure_summary.exposures.net_gex.
	NetGex *float64 `json:"net_gex"`
	// Strike where net dealer gamma crosses zero.
	GammaFlip *float64 `json:"gamma_flip"`
}

// VrpStrategyScores holds 0-100 suitability scores for canonical short-vol
// strategies. Higher = better fit for current market conditions. Each field
// can be nil on historical when inputs aren't computable for the given `at`
// timestamp.
type VrpStrategyScores struct {
	// Short put credit spread — sells downside VRP with capped loss.
	ShortPutSpread *int `json:"short_put_spread"`
	// Short strangle — sells both wings; max profit if spot pins.
	ShortStrangle *int `json:"short_strangle"`
	// Iron condor — defined-risk version of short strangle.
	IronCondor *int `json:"iron_condor"`
	// Calendar spread — sells front-month vol, buys back-month. Best when
	// the term structure is steep contango.
	CalendarSpread *int `json:"calendar_spread"`
}

// VrpMacro is the macro-context snapshot used to condition the VRP outlook.
//
// Historical-specific: FedFunds is absent on historical responses (this
// struct doesn't declare it). HySpread is populated here (live returns nil).
type VrpMacro struct {
	// CBOE VIX index level.
	Vix *float64 `json:"vix"`
	// CBOE VIX3M (3-month VIX).
	Vix3m *float64 `json:"vix_3m"`
	// (vix_3m - vix) / vix * 100 — % steepness of near-term term structure.
	// Positive = contango; negative = backwardation.
	VixTermSlope *float64 `json:"vix_term_slope"`
	// 10-year US Treasury yield (%, FRED DGS10).
	Dgs10 *float64 `json:"dgs10"`
	// ICE BofA US High Yield OAS (%, FRED BAMLH0A0HYM2). Populated on
	// historical responses (live returns nil currently).
	HySpread *float64 `json:"hy_spread"`
}
