package flashalphahistorical

// sell-window: GET /v1/strategies/sell-window/{symbol}?at=...
//
// Replays the sell-window answer for any session since 2018. Docs:
// https://flashalpha.com/docs/historical-sell-window

import (
	"context"
	"net/url"
)

// SellWindowLeg is one contract of a sell-window trade.
type SellWindowLeg struct {
	Side string `json:"side"` // sell or buy
	// Quantity is contracts per unit of the trade: 2 for the sold body of put_bwb, otherwise 1.
	Quantity int     `json:"quantity"`
	Type     string  `json:"type"` // call or put
	Strike   float64 `json:"strike"`
	// Delta is the contract's own delta when the answer was served (negative for puts).
	Delta float64 `json:"delta"`
	Bid   float64 `json:"bid"`
	Ask   float64 `json:"ask"`
}

// SellWindowCandidate is one trade to consider on a sell window.
type SellWindowCandidate struct {
	// Structure is short_put, short_call, short_strangle, short_straddle,
	// tail_hedged_put, put_credit_spread or put_bwb.
	Structure string `json:"structure"`
	// Exit is expiry (hold to expiration) or tp50 (close at half of the credit, otherwise hold).
	Exit string `json:"exit"`
	// Legs is empty when no option chain could be loaded for the answer.
	Legs []SellWindowLeg `json:"legs,omitempty"`
	// MidCredit is the net credit per share at mid. Multiply by 100 for one unit in dollars.
	MidCredit *float64 `json:"mid_credit,omitempty"`
}

// SellWindowWindow is one of the four windows, named by its listed expiry.
type SellWindowWindow struct {
	// Expiry (yyyy-MM-dd) is nil when no listed expiry falls in the window's range.
	Expiry *string `json:"expiry,omitempty"`
	// DaysToExpiry is calendar days from the session to Expiry.
	DaysToExpiry *int `json:"days_to_expiry,omitempty"`
	// Action is sell, skip or unavailable. Treat unavailable like a skip.
	Action string `json:"action"`
	// Candidates is set only when Action is sell. It can be empty: a trade whose
	// legs cannot all be found in the chain is left out.
	Candidates []SellWindowCandidate `json:"candidates,omitempty"`
}

// SellWindowResponse is the sell-window answer. It has its own shape, not the
// StrategyDecisionResponse envelope. Spot, QuotedAt and LegsIndicative are set when a
// window sells and an option chain could be loaded.
type SellWindowResponse struct {
	Symbol string `json:"symbol"`
	// Session (yyyy-MM-dd) is the trading session the answer applies to.
	Session string `json:"session"`
	// Spot is the underlying mid the strikes were picked against.
	Spot *float64 `json:"spot,omitempty"`
	// QuotedAt is the UTC time of the quotes behind the strikes.
	QuotedAt *string `json:"quoted_at,omitempty"`
	// LegsIndicative is true when the quotes are from before that session's 09:35 ET
	// entry: call again at or after 09:35 ET for the strikes to trade.
	LegsIndicative *bool `json:"legs_indicative,omitempty"`
	// Windows always has four entries: same day, then about one week, two weeks and one month out.
	Windows []SellWindowWindow `json:"windows"`
	// ResponseEnvelope carries data_as_of, archive_as_of and endpoint_version.
	ResponseEnvelope
}

// SellWindow returns the sell-window answer for the session at `at`, with strikes from
// the archive chain. The backtests entered at 09:35 ET, so at = "<session>T09:35:00"
// returns the backtest's entry strikes. The session switches at the close: a date-only
// `at` reads 16:00 ET and returns the answer for the next session.
//
// SPY, QQQ and IWM; any other symbol returns every window as unavailable. Requires Alpha+.
// Use SellWindowTyped for the typed response.
func (c *Client) SellWindow(ctx context.Context, symbol, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/strategies/sell-window/"+seg(symbol), url.Values{"at": []string{at}})
}

// SellWindowTyped is the strongly typed variant of SellWindow.
func (c *Client) SellWindowTyped(ctx context.Context, symbol, at string) (*SellWindowResponse, error) {
	raw, err := c.SellWindow(ctx, symbol, at)
	if err != nil {
		return nil, err
	}
	out := &SellWindowResponse{}
	if err := decodeTyped("sell-window", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}
