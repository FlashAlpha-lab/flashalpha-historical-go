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

// VolatilityTyped is the strongly-typed variant of Volatility. The original
// Volatility continues to return map[string]interface{} unchanged.
//
// Returns a fully-populated *VolatilityResponse for the given symbol at the
// requested historical minute.
func (c *Client) VolatilityTyped(ctx context.Context, symbol, at string) (*VolatilityResponse, error) {
	raw, err := c.Volatility(ctx, symbol, at)
	if err != nil {
		return nil, err
	}
	out := &VolatilityResponse{}
	if err := decodeTyped("volatility", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// AdvVolatilityTyped is the strongly-typed variant of AdvVolatility. The
// original AdvVolatility continues to return map[string]interface{} unchanged.
//
// Returns a fully-populated *AdvVolatilityResponse for the given symbol at
// the requested historical minute.
func (c *Client) AdvVolatilityTyped(ctx context.Context, symbol, at string) (*AdvVolatilityResponse, error) {
	raw, err := c.AdvVolatility(ctx, symbol, at)
	if err != nil {
		return nil, err
	}
	out := &AdvVolatilityResponse{}
	if err := decodeTyped("adv volatility", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// SurfaceTyped is the strongly-typed variant of Surface. The original
// Surface continues to return map[string]interface{} unchanged.
//
// Returns a fully-populated *SurfaceResponse for the given symbol at the
// requested historical minute.
func (c *Client) SurfaceTyped(ctx context.Context, symbol, at string) (*SurfaceResponse, error) {
	raw, err := c.Surface(ctx, symbol, at)
	if err != nil {
		return nil, err
	}
	out := &SurfaceResponse{}
	if err := decodeTyped("surface", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GexTyped is the strongly-typed variant of Gex. The original Gex continues
// to return map[string]interface{} unchanged.
//
// Returns a fully-populated *GexResponse for the given symbol at the
// requested historical minute.
func (c *Client) GexTyped(ctx context.Context, symbol, at string, opts ...Option) (*GexResponse, error) {
	raw, err := c.Gex(ctx, symbol, at, opts...)
	if err != nil {
		return nil, err
	}
	out := &GexResponse{}
	if err := decodeTyped("gex", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DexTyped is the strongly-typed variant of Dex. The original Dex continues
// to return map[string]interface{} unchanged.
//
// Returns a fully-populated *DexResponse for the given symbol at the
// requested historical minute.
func (c *Client) DexTyped(ctx context.Context, symbol, at string, opts ...Option) (*DexResponse, error) {
	raw, err := c.Dex(ctx, symbol, at, opts...)
	if err != nil {
		return nil, err
	}
	out := &DexResponse{}
	if err := decodeTyped("dex", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// VexTyped is the strongly-typed variant of Vex. The original Vex continues
// to return map[string]interface{} unchanged.
//
// Returns a fully-populated *VexResponse for the given symbol at the
// requested historical minute.
func (c *Client) VexTyped(ctx context.Context, symbol, at string, opts ...Option) (*VexResponse, error) {
	raw, err := c.Vex(ctx, symbol, at, opts...)
	if err != nil {
		return nil, err
	}
	out := &VexResponse{}
	if err := decodeTyped("vex", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ChexTyped is the strongly-typed variant of Chex. The original Chex
// continues to return map[string]interface{} unchanged.
//
// Returns a fully-populated *ChexResponse for the given symbol at the
// requested historical minute.
func (c *Client) ChexTyped(ctx context.Context, symbol, at string, opts ...Option) (*ChexResponse, error) {
	raw, err := c.Chex(ctx, symbol, at, opts...)
	if err != nil {
		return nil, err
	}
	out := &ChexResponse{}
	if err := decodeTyped("chex", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}
