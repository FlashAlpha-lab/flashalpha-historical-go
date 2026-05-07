package flashalphahistorical

// Strongly-typed wrapper methods for the existing map-returning client
// endpoints. Each `*Typed` method delegates to the canonical untyped method
// (preserving identical request semantics, including the required `at`
// historical timestamp) and decodes the response into the appropriate typed
// struct from this package.
//
// The original untyped methods continue to return map[string]interface{}
// unchanged — adding these wrappers is purely additive.

import (
	"context"
	"encoding/json"
	"fmt"
)

// decodeTyped re-encodes a map[string]interface{} response and decodes it
// into the supplied typed-struct pointer. Used by the *Typed wrappers below
// to keep them concise and uniform.
func decodeTyped(label string, raw map[string]interface{}, out interface{}) error {
	buf, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("flashalpha-historical: re-encode %s: %w", label, err)
	}
	if err := json.Unmarshal(buf, out); err != nil {
		return fmt.Errorf("flashalpha-historical: decode %s: %w", label, err)
	}
	return nil
}

// StockSummaryTyped is the strongly-typed variant of StockSummary. The
// original StockSummary continues to return map[string]interface{} unchanged.
//
// Returns a fully-populated *StockSummaryResponse for the given symbol at
// the requested historical minute.
func (c *Client) StockSummaryTyped(ctx context.Context, symbol, at string) (*StockSummaryResponse, error) {
	raw, err := c.StockSummary(ctx, symbol, at)
	if err != nil {
		return nil, err
	}
	out := &StockSummaryResponse{}
	if err := decodeTyped("stock summary", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// NarrativeTyped is the strongly-typed variant of Narrative. The original
// Narrative continues to return map[string]interface{} unchanged.
//
// Returns a fully-populated *NarrativeResponse for the given symbol at the
// requested historical minute.
func (c *Client) NarrativeTyped(ctx context.Context, symbol, at string) (*NarrativeResponse, error) {
	raw, err := c.Narrative(ctx, symbol, at)
	if err != nil {
		return nil, err
	}
	out := &NarrativeResponse{}
	if err := decodeTyped("narrative", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// LevelsTyped is the strongly-typed variant of ExposureLevels. The original
// ExposureLevels continues to return map[string]interface{} unchanged.
//
// Returns a fully-populated *LevelsResponse for the given symbol at the
// requested historical minute.
func (c *Client) LevelsTyped(ctx context.Context, symbol, at string) (*LevelsResponse, error) {
	raw, err := c.ExposureLevels(ctx, symbol, at)
	if err != nil {
		return nil, err
	}
	out := &LevelsResponse{}
	if err := decodeTyped("levels", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// MaxPainTyped is the strongly-typed variant of MaxPain. The original
// MaxPain continues to return map[string]interface{} unchanged.
//
// Returns a fully-populated *MaxPainResponse for the given symbol at the
// requested historical minute. Pass WithExpiration to scope the response to
// a single expiry.
func (c *Client) MaxPainTyped(ctx context.Context, symbol, at string, opts ...Option) (*MaxPainResponse, error) {
	raw, err := c.MaxPain(ctx, symbol, at, opts...)
	if err != nil {
		return nil, err
	}
	out := &MaxPainResponse{}
	if err := decodeTyped("max pain", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// VrpTyped is the strongly-typed variant of Vrp. The original Vrp continues
// to return map[string]interface{} unchanged.
//
// Returns a fully-populated *VrpResponse for the given symbol at the
// requested historical minute.
func (c *Client) VrpTyped(ctx context.Context, symbol, at string) (*VrpResponse, error) {
	raw, err := c.Vrp(ctx, symbol, at)
	if err != nil {
		return nil, err
	}
	out := &VrpResponse{}
	if err := decodeTyped("vrp", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ExposureSummaryTyped is the strongly-typed variant of ExposureSummary.
// The original ExposureSummary continues to return map[string]interface{}
// unchanged.
//
// Returns a fully-populated *ExposureSummaryResponse for the given symbol
// at the requested historical minute.
func (c *Client) ExposureSummaryTyped(ctx context.Context, symbol, at string) (*ExposureSummaryResponse, error) {
	raw, err := c.ExposureSummary(ctx, symbol, at)
	if err != nil {
		return nil, err
	}
	out := &ExposureSummaryResponse{}
	if err := decodeTyped("exposure summary", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}
