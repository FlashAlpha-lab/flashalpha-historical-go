package flashalphahistorical

// Typed response model for `GET /v1/exposure/zero-dte/{symbol}?at=...` (Growth+).
//
// Same shape as the live API, replayed at the requested historical minute.
// The historical endpoint requires the `at` parameter; date-only `at` values
// snap to 16:00 ET on that day (`as_of` echoes the snap target).
//
// On weekends/holidays or symbols with no 0DTE on the requested day, NoZeroDte
// is true and most fields are zero/nil — only Symbol, AsOf, Message, and
// NextZeroDteExpiry are populated.
//
// Raw holds the underlying decoded JSON for any field not modeled here.

// ZeroDteRegime is the gamma-regime block of a ZeroDteResponse.
type ZeroDteRegime struct {
	// Label is the dealer-gamma regime classification for the 0DTE chain.
	// Confirmed values: "positive_gamma" (spot above gamma flip — dealers
	// dampen moves, mean reversion likely) or "negative_gamma" (spot below
	// flip — dealers amplify moves, trend-following likely).
	Label string `json:"label"`
	// Description is a plain-English narrative for the current regime — safe
	// to surface verbatim in customer-facing UIs.
	Description string `json:"description"`
	// GammaFlip is the strike where 0DTE net dealer gamma exposure crosses
	// zero. The single most-watched intraday level on this endpoint.
	GammaFlip *float64 `json:"gamma_flip"`
	// SpotVsFlip is "above" or "below" — convenience label matching the sign
	// of (underlying_price - gamma_flip).
	SpotVsFlip string `json:"spot_vs_flip"`
	// SpotToFlipPct is signed % distance from spot to GammaFlip
	// (positive = spot above flip).
	SpotToFlipPct *float64 `json:"spot_to_flip_pct"`
	// DistanceToFlipDollars is the unsigned dollar distance from spot to
	// GammaFlip.
	DistanceToFlipDollars *float64 `json:"distance_to_flip_dollars"`
	// DistanceToFlipSigmas is the 1σ-normalized distance to the gamma flip,
	// using the remaining-time σ during the session (ATM IV × √t_remain)
	// and falling back to a full-day σ when the market is closed.
	DistanceToFlipSigmas *float64 `json:"distance_to_flip_sigmas"`
}

// ZeroDteExposures aggregates net 0DTE GEX/DEX/VEX/CHEX and full-chain context.
type ZeroDteExposures struct {
	// NetGex is net 0DTE gamma exposure in dollars per 1% spot move.
	NetGex *float64 `json:"net_gex"`
	// NetDex is net 0DTE delta exposure in dollars.
	NetDex *float64 `json:"net_dex"`
	// NetVex is net 0DTE vanna exposure in dollars per 1-vol-point.
	NetVex *float64 `json:"net_vex"`
	// NetChex is net 0DTE charm exposure in dollars per day.
	NetChex *float64 `json:"net_chex"`
	// PctOfTotalGex is 0DTE share of full-chain net GEX as a percentage.
	PctOfTotalGex *float64 `json:"pct_of_total_gex"`
	// TotalChainNetGex is full-chain (all expirations) net dealer GEX.
	TotalChainNetGex *float64 `json:"total_chain_net_gex"`
}

// ZeroDteExpectedMove holds implied 1σ move + remaining-session 1σ + ATM straddle.
type ZeroDteExpectedMove struct {
	// Implied1SdDollars is the full-day 1σ implied move in dollars from ATM IV.
	Implied1SdDollars *float64 `json:"implied_1sd_dollars"`
	// Implied1SdPct is Implied1SdDollars as a % of spot.
	Implied1SdPct *float64 `json:"implied_1sd_pct"`
	// Remaining1SdDollars is the 1σ implied move for the time REMAINING in
	// the session.
	Remaining1SdDollars *float64 `json:"remaining_1sd_dollars"`
	// Remaining1SdPct is Remaining1SdDollars as a % of spot.
	Remaining1SdPct *float64 `json:"remaining_1sd_pct"`
	// UpperBound is spot + Remaining1SdDollars.
	UpperBound *float64 `json:"upper_bound"`
	// LowerBound is spot - Remaining1SdDollars.
	LowerBound *float64 `json:"lower_bound"`
	// StraddlePrice is the ATM 0DTE straddle mid in dollars.
	StraddlePrice *float64 `json:"straddle_price"`
	// AtmIv is the ATM implied volatility for the 0DTE chain (annualised %).
	AtmIv *float64 `json:"atm_iv"`
}

// ZeroDtePinComponents is the sub-score breakdown for ZeroDtePinRisk.PinScore.
type ZeroDtePinComponents struct {
	// OiScore is the OI-concentration sub-score (0-100). Weight: 30%.
	OiScore *int `json:"oi_score"`
	// ProximityScore is the magnet-proximity sub-score (0-100). Weight: 25%.
	ProximityScore *int `json:"proximity_score"`
	// TimeScore is the time-remaining sub-score (0-100). Weight: 25%.
	TimeScore *int `json:"time_score"`
	// GammaScore is the gamma-magnitude sub-score (0-100). Weight: 20%.
	GammaScore *int `json:"gamma_score"`
}

// ZeroDtePinRisk is the pin-risk block — magnet strike + composite + sub-scores.
type ZeroDtePinRisk struct {
	// MagnetStrike is the strike with the largest absolute 0DTE GEX.
	MagnetStrike *float64 `json:"magnet_strike"`
	// MagnetGex is the dealer gamma exposure at MagnetStrike.
	MagnetGex *float64 `json:"magnet_gex"`
	// DistanceToMagnetPct is the signed % distance from spot to MagnetStrike.
	DistanceToMagnetPct *float64 `json:"distance_to_magnet_pct"`
	// PinScore is a 0-100 composite — likelihood spot pins to MagnetStrike.
	PinScore *int `json:"pin_score"`
	// Components is the sub-score breakdown that feeds PinScore.
	Components *ZeroDtePinComponents `json:"components"`
	// MaxPain is the strike where total option-holder intrinsic value is minimized.
	MaxPain *float64 `json:"max_pain"`
	// OiConcentrationTop3Pct is the share of total 0DTE OI (%) at the top-3 strikes.
	OiConcentrationTop3Pct *float64 `json:"oi_concentration_top3_pct"`
	// Description is a plain-English narrative for the current pin setup.
	Description string `json:"description"`
}

// ZeroDteHedgingBucket is one row of dealer hedging flow at a specific spot delta.
type ZeroDteHedgingBucket struct {
	// DealerSharesToTrade is the estimated underlying shares dealers must
	// trade to remain delta-neutral if spot moves to this bucket.
	DealerSharesToTrade *float64 `json:"dealer_shares_to_trade"`
	// Direction is the lowercase convenience label matching the sign of
	// DealerSharesToTrade ("buy" or "sell").
	Direction string `json:"direction"`
	// NotionalUsd is |DealerSharesToTrade| × current_spot.
	NotionalUsd *float64 `json:"notional_usd"`
}

// ZeroDteHedging holds dealer hedging flow at ±10bp/25bp/50bp/100bp + GEX convexity.
type ZeroDteHedging struct {
	// SpotUp10Bp is the dealer hedging flow if spot rises 10 basis points.
	SpotUp10Bp *ZeroDteHedgingBucket `json:"spot_up_10bp"`
	// SpotDown10Bp is the dealer hedging flow if spot falls 10 basis points.
	SpotDown10Bp *ZeroDteHedgingBucket `json:"spot_down_10bp"`
	// SpotUp25Bp is the dealer hedging flow if spot rises 25 basis points.
	SpotUp25Bp *ZeroDteHedgingBucket `json:"spot_up_25bp"`
	// SpotDown25Bp is the dealer hedging flow if spot falls 25 basis points.
	SpotDown25Bp *ZeroDteHedgingBucket `json:"spot_down_25bp"`
	// SpotUpHalfPct is the dealer hedging flow if spot rises 50 basis points.
	SpotUpHalfPct *ZeroDteHedgingBucket `json:"spot_up_half_pct"`
	// SpotDownHalfPct is the dealer hedging flow if spot falls 50 basis points.
	SpotDownHalfPct *ZeroDteHedgingBucket `json:"spot_down_half_pct"`
	// SpotUp1Pct is the dealer hedging flow if spot rises 1%.
	SpotUp1Pct *ZeroDteHedgingBucket `json:"spot_up_1pct"`
	// SpotDown1Pct is the dealer hedging flow if spot falls 1%.
	SpotDown1Pct *ZeroDteHedgingBucket `json:"spot_down_1pct"`
	// ConvexityAtSpot is the 2nd finite-difference of net GEX taken across
	// the three strikes nearest spot.
	ConvexityAtSpot *float64 `json:"convexity_at_spot"`
}

// ZeroDteDecay is the time-decay block — net theta + per-hour rate + acceleration.
type ZeroDteDecay struct {
	// NetThetaDollars is the net dollar theta of the 0DTE chain.
	NetThetaDollars *float64 `json:"net_theta_dollars"`
	// ThetaPerHourRemaining is NetThetaDollars / hours_to_close.
	ThetaPerHourRemaining *float64 `json:"theta_per_hour_remaining"`
	// CharmRegime is a label classifying the dealer-charm direction.
	CharmRegime string `json:"charm_regime"`
	// CharmDescription is a plain-English narrative for CharmRegime.
	CharmDescription string `json:"charm_description"`
	// GammaAcceleration is the ratio of 0DTE ATM gamma to 7DTE ATM gamma.
	GammaAcceleration *float64 `json:"gamma_acceleration"`
	// Description is a plain-English narrative for the overall decay setup.
	Description string `json:"description"`
}

// ZeroDteVolContext gives the vol-surface context — 0DTE vs 7DTE IV + vanna read.
type ZeroDteVolContext struct {
	// ZeroDteAtmIv is ATM implied vol for the 0DTE chain (annualised %).
	ZeroDteAtmIv *float64 `json:"zero_dte_atm_iv"`
	// SevenDteAtmIv is ATM implied vol for the 7DTE chain (annualised %).
	SevenDteAtmIv *float64 `json:"seven_dte_atm_iv"`
	// IvRatio0Dte7Dte is ZeroDteAtmIv / SevenDteAtmIv.
	IvRatio0Dte7Dte *float64 `json:"iv_ratio_0dte_7dte"`
	// Vix is the CBOE VIX index level — macro vol context.
	Vix *float64 `json:"vix"`
	// VannaExposure is net 0DTE vanna exposure in dollars per 1-vol-point.
	VannaExposure *float64 `json:"vanna_exposure"`
	// VannaInterpretation is a label for the dealer-vanna setup.
	VannaInterpretation string `json:"vanna_interpretation"`
	// Description is a plain-English narrative for the overall vol-context read.
	Description string `json:"description"`
}

// ZeroDteFlow is the flow block — volume/OI aggregates + concentration metrics.
//
// Historical-mode note: volume fields (TotalVolume / CallVolume / PutVolume /
// NetCallMinusPutVolume / PcRatioVolume / VolumeToOiRatio /
// AtmVolumeSharePct / Top3StrikeVolumePct) are 0/nil at historical minutes
// (volume is not replayed); OI fields populate normally.
type ZeroDteFlow struct {
	// TotalVolume is total 0DTE option contracts traded so far in the session.
	TotalVolume *int64 `json:"total_volume"`
	// CallVolume is total 0DTE call contracts traded.
	CallVolume *int64 `json:"call_volume"`
	// PutVolume is total 0DTE put contracts traded.
	PutVolume *int64 `json:"put_volume"`
	// NetCallMinusPutVolume is CallVolume - PutVolume.
	NetCallMinusPutVolume *int64 `json:"net_call_minus_put_volume"`
	// TotalOi is total 0DTE open interest entering the session.
	TotalOi *int64 `json:"total_oi"`
	// CallOi is 0DTE call OI entering the session.
	CallOi *int64 `json:"call_oi"`
	// PutOi is 0DTE put OI entering the session.
	PutOi *int64 `json:"put_oi"`
	// PcRatioVolume is PutVolume / CallVolume.
	PcRatioVolume *float64 `json:"pc_ratio_volume"`
	// PcRatioOi is PutOi / CallOi.
	PcRatioOi *float64 `json:"pc_ratio_oi"`
	// VolumeToOiRatio is TotalVolume / TotalOi.
	VolumeToOiRatio *float64 `json:"volume_to_oi_ratio"`
	// AtmVolumeSharePct is the share of TotalVolume (%) traded at ATM strikes.
	AtmVolumeSharePct *float64 `json:"atm_volume_share_pct"`
	// Top3StrikeVolumePct is the share of TotalVolume (%) at the top-3 strikes.
	Top3StrikeVolumePct *float64 `json:"top3_strike_volume_pct"`
}

// ZeroDteLevels holds key strikes — call/put walls (with strength), gamma extrema,
// highest-OI strike, distance-to-magnet, and the level-cluster composite.
type ZeroDteLevels struct {
	// CallWall is the strike with the largest absolute call GEX.
	CallWall *float64 `json:"call_wall"`
	// CallWallGex is the dealer gamma exposure at CallWall.
	CallWallGex *float64 `json:"call_wall_gex"`
	// CallWallStrength is |CallWallGex| / total absolute call-side GEX.
	CallWallStrength *float64 `json:"call_wall_strength"`
	// DistanceToCallWallPct is signed % distance from spot to CallWall.
	DistanceToCallWallPct *float64 `json:"distance_to_call_wall_pct"`
	// PutWall is the strike with the largest absolute put GEX.
	PutWall *float64 `json:"put_wall"`
	// PutWallGex is the dealer gamma exposure at PutWall.
	PutWallGex *float64 `json:"put_wall_gex"`
	// PutWallStrength is |PutWallGex| / total absolute put-side GEX.
	PutWallStrength *float64 `json:"put_wall_strength"`
	// DistanceToPutWallPct is signed % distance from spot to PutWall.
	DistanceToPutWallPct *float64 `json:"distance_to_put_wall_pct"`
	// DistanceToMagnetDollars is unsigned dollar distance from spot to the magnet.
	DistanceToMagnetDollars *float64 `json:"distance_to_magnet_dollars"`
	// HighestOiStrike is the strike with the largest total 0DTE OI.
	HighestOiStrike *float64 `json:"highest_oi_strike"`
	// HighestOiTotal is the total OI (calls + puts) at HighestOiStrike.
	HighestOiTotal *int64 `json:"highest_oi_total"`
	// MaxPositiveGamma is the strike with the largest positive 0DTE net GEX.
	MaxPositiveGamma *float64 `json:"max_positive_gamma"`
	// MaxNegativeGamma is the strike with the largest negative 0DTE net GEX.
	MaxNegativeGamma *float64 `json:"max_negative_gamma"`
	// LevelClusterScore is a 0-100 composite — how tightly the key levels cluster.
	LevelClusterScore *int `json:"level_cluster_score"`
}

// ZeroDteLiquidity is the bid-ask liquidity context for the 0DTE chain.
type ZeroDteLiquidity struct {
	// AtmSpreadPct is the ATM bid-ask spread as a % of mid.
	AtmSpreadPct *float64 `json:"atm_spread_pct"`
	// WeightedSpreadPct is the volume-weighted average bid-ask spread.
	WeightedSpreadPct *float64 `json:"weighted_spread_pct"`
	// ExecutionScore is a 0-100 liquidity score.
	ExecutionScore *int `json:"execution_score"`
}

// ZeroDteMetadata is the staleness + quality metadata for the snapshot.
type ZeroDteMetadata struct {
	// SnapshotAgeSeconds is the age of the underlying chain snapshot in seconds.
	SnapshotAgeSeconds *float64 `json:"snapshot_age_seconds"`
	// ChainContractCount is the number of contracts in the 0DTE chain.
	ChainContractCount *int `json:"chain_contract_count"`
	// DataQualityScore is a 0-100 composite quality metric.
	DataQualityScore *int `json:"data_quality_score"`
	// GreekSmoothnessScore is a 0-100 measurement of IV smoothness across strikes.
	GreekSmoothnessScore *int `json:"greek_smoothness_score"`
}

// ZeroDteStrike is one row in ZeroDteResponse.Strikes — per-strike exposure,
// flow, greeks, and quote/spread metrics.
//
// Historical-mode note: the volume fields (CallVolume, PutVolume,
// VolumeSharePct, CallSpreadPct, PutSpreadPct) and the per-strike spread
// fields are 0/nil at historical minutes.
type ZeroDteStrike struct {
	// Strike is the strike price (always populated).
	Strike float64 `json:"strike"`
	// DistanceFromSpotPct is signed % distance from spot to Strike.
	DistanceFromSpotPct *float64 `json:"distance_from_spot_pct"`
	// CallSymbol is the OCC option symbol for the call side at this strike.
	CallSymbol string `json:"call_symbol"`
	// PutSymbol is the OCC option symbol for the put side at this strike.
	PutSymbol string `json:"put_symbol"`
	// CallGex is the dealer-side gamma exposure ($/1% spot move) for calls.
	CallGex *float64 `json:"call_gex"`
	// PutGex is the dealer-side gamma exposure ($/1% spot move) for puts.
	PutGex *float64 `json:"put_gex"`
	// NetGex is CallGex + PutGex.
	NetGex *float64 `json:"net_gex"`
	// CallDex is the dealer-side delta exposure ($) for calls at this strike.
	CallDex *float64 `json:"call_dex"`
	// PutDex is the dealer-side delta exposure ($) for puts at this strike.
	PutDex *float64 `json:"put_dex"`
	// NetDex is CallDex + PutDex.
	NetDex *float64 `json:"net_dex"`
	// NetVex is the net dealer vanna exposure at this strike.
	NetVex *float64 `json:"net_vex"`
	// NetChex is the net dealer charm exposure at this strike.
	NetChex *float64 `json:"net_chex"`
	// CallOi is open interest on the call side at this strike.
	CallOi *int64 `json:"call_oi"`
	// PutOi is open interest on the put side at this strike.
	PutOi *int64 `json:"put_oi"`
	// CallVolume is session volume on the call side at this strike.
	CallVolume *int64 `json:"call_volume"`
	// PutVolume is session volume on the put side at this strike.
	PutVolume *int64 `json:"put_volume"`
	// GexSharePct is |NetGex| as a share (%) of total absolute 0DTE GEX.
	GexSharePct *float64 `json:"gex_share_pct"`
	// OiSharePct is (CallOi + PutOi) as a share (%) of total 0DTE OI.
	OiSharePct *float64 `json:"oi_share_pct"`
	// VolumeSharePct is (CallVolume + PutVolume) as a share (%) of total volume.
	VolumeSharePct *float64 `json:"volume_share_pct"`
	// CallIv is the implied vol of the call leg at this strike (annualised %).
	CallIv *float64 `json:"call_iv"`
	// PutIv is the implied vol of the put leg at this strike (annualised %).
	PutIv *float64 `json:"put_iv"`
	// CallDelta is the call leg's delta.
	CallDelta *float64 `json:"call_delta"`
	// PutDelta is the put leg's delta.
	PutDelta *float64 `json:"put_delta"`
	// CallGamma is the call leg's gamma.
	CallGamma *float64 `json:"call_gamma"`
	// PutGamma is the put leg's gamma.
	PutGamma *float64 `json:"put_gamma"`
	// CallTheta is the call leg's theta ($/day).
	CallTheta *float64 `json:"call_theta"`
	// PutTheta is the put leg's theta ($/day).
	PutTheta *float64 `json:"put_theta"`
	// CallMid is the call mid price.
	CallMid *float64 `json:"call_mid"`
	// PutMid is the put mid price.
	PutMid *float64 `json:"put_mid"`
	// CallSpreadPct is the call bid-ask spread as a % of mid.
	CallSpreadPct *float64 `json:"call_spread_pct"`
	// PutSpreadPct is the put bid-ask spread as a % of mid.
	PutSpreadPct *float64 `json:"put_spread_pct"`
}

// ZeroDteResponse is the full payload from
// GET /v1/exposure/zero-dte/{symbol}?at=... (historical mode).
//
// The historical endpoint requires `at`; date-only `at` values snap to
// 16:00 ET on that day and AsOf echoes the snap target.
//
// On weekends/holidays or symbols with no 0DTE on that day, NoZeroDte is true
// and most fields are zero/nil — only Symbol, AsOf, Message, and
// NextZeroDteExpiry are populated.
//
// Raw holds the underlying decoded JSON for any field not modeled here.
type ZeroDteResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid at AsOf.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// Expiration is the ISO date ("yyyy-MM-dd") of the 0DTE expiry.
	Expiration *string `json:"expiration"`
	// AsOf is the ET wall-clock timestamp this snapshot was computed for.
	AsOf string `json:"as_of"`
	// MarketOpen is true if NYSE was open at AsOf.
	MarketOpen bool `json:"market_open"`
	// TimeToCloseHours is hours remaining until the regular-session close
	// at AsOf.
	TimeToCloseHours *float64 `json:"time_to_close_hours"`
	// TimeToClosePct is the percent of the regular trading day ELAPSED at AsOf.
	TimeToClosePct *float64 `json:"time_to_close_pct"`
	// Regime is the gamma-regime block. See ZeroDteRegime.
	Regime *ZeroDteRegime `json:"regime"`
	// Exposures is the net 0DTE Greek totals plus the 0DTE share of full-chain GEX.
	Exposures *ZeroDteExposures `json:"exposures"`
	// ExpectedMove is implied 1σ (full-day and remaining-session) plus the
	// ATM straddle price.
	ExpectedMove *ZeroDteExpectedMove `json:"expected_move"`
	// PinRisk is the magnet strike + composite pin score + sub-scores.
	PinRisk *ZeroDtePinRisk `json:"pin_risk"`
	// Hedging is the dealer hedging-flow estimates.
	Hedging *ZeroDteHedging `json:"hedging"`
	// Decay is the time-decay block.
	Decay *ZeroDteDecay `json:"decay"`
	// VolContext is the vol-surface context.
	VolContext *ZeroDteVolContext `json:"vol_context"`
	// Flow is the volume/OI aggregates + concentration metrics for the session.
	Flow *ZeroDteFlow `json:"flow"`
	// Levels holds the key strikes.
	Levels *ZeroDteLevels `json:"levels"`
	// Liquidity is the bid-ask context.
	Liquidity *ZeroDteLiquidity `json:"liquidity"`
	// Metadata is staleness + quality metadata.
	Metadata *ZeroDteMetadata `json:"metadata"`
	// Strikes is the per-strike grid.
	Strikes []ZeroDteStrike `json:"strikes"`

	// Warnings is optional — only present near close (<5 min) when greeks may
	// be unstable.
	Warnings []string `json:"warnings,omitempty"`

	// NoZeroDte is the fallback flag — true on weekends, holidays, or for
	// symbols without a same-day expiry on the requested AsOf. When true,
	// most fields above are nil/zero.
	NoZeroDte bool `json:"no_zero_dte"`
	// Message is a plain-English explanation of the no-0DTE state.
	Message string `json:"message"`
	// NextZeroDteExpiry is the ISO date of the next available 0DTE expiry.
	NextZeroDteExpiry *string `json:"next_zero_dte_expiry"`

	// Raw holds the unparsed JSON for forward compatibility with new fields.
	Raw map[string]interface{} `json:"-"`
}
