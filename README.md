# flashalpha-historical-go

Official Go client for the **FlashAlpha Historical API** — point-in-time
replay of every live analytics endpoint. Ask what GEX, gamma flip, VRP,
narrative, max pain, or the full stock summary looked like at any **minute
back to 2018-04-16**, in the same response shape as the live API.

```bash
go get github.com/FlashAlpha-lab/flashalpha-historical-go
```

Go 1.21+. Same `X-Api-Key` you use for `api.flashalpha.com` — Alpha plan or
higher on every endpoint.

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    fh "github.com/FlashAlpha-lab/flashalpha-historical-go"
)

func main() {
    c := fh.NewClient(os.Getenv("FLASHALPHA_API_KEY"))

    snap, err := c.ExposureSummary(context.Background(), "SPY", "2020-03-16T15:30:00")
    if err != nil { log.Fatal(err) }

    fmt.Printf("regime: %v, gamma_flip: %v\n", snap["regime"], snap["gamma_flip"])
}
```

## Backtesting

```go
ctx := context.Background()
c := fh.NewClient(os.Getenv("FLASHALPHA_API_KEY"))

bt := fh.NewBacktester(c)
bt.Endpoint = fh.EndpointStockSummary
bt.Symbol = "SPY"

start, _ := time.Parse(fh.AtFormatDate, "2024-01-02")
end, _ := time.Parse(fh.AtFormatDate, "2024-03-29")

results, err := bt.Run(ctx, fh.IterDays(start, end), func(at string, snap map[string]interface{}) interface{} {
    vol, _ := snap["volatility"].(map[string]interface{})
    return map[string]interface{}{ "vrp": vol["vrp"] }
})
```

### Minute-level

```go
d, _ := time.Parse(fh.AtFormatDate, "2025-01-15")
steps, _ := fh.Replay(ctx, c, fh.EndpointExposureSummary, "SPY",
    fh.IterMinutes(d, d, 15), nil)
for _, s := range steps {
    fmt.Println(s.At, s.Response["regime"], s.Response["underlying_price"])
}
```

## API surface

| Method | Endpoint |
|---|---|
| `Tickers(ctx, symbol)` | `/v1/tickers` |
| `StockQuote(ctx, t, at)` | `/v1/stockquote/{t}` |
| `OptionQuote(ctx, t, at, ...Option)` | `/v1/optionquote/{t}` |
| `Surface(ctx, s, at)` | `/v1/surface/{s}` |
| `Gex(ctx, s, at, ...Option)` | `/v1/exposure/gex/{s}` |
| `Dex(ctx, s, at, ...Option)` | `/v1/exposure/dex/{s}` |
| `Vex(ctx, s, at, ...Option)` | `/v1/exposure/vex/{s}` |
| `Chex(ctx, s, at, ...Option)` | `/v1/exposure/chex/{s}` |
| `ExposureSummary(ctx, s, at)` | `/v1/exposure/summary/{s}` |
| `ExposureLevels(ctx, s, at)` | `/v1/exposure/levels/{s}` |
| `Narrative(ctx, s, at)` | `/v1/exposure/narrative/{s}` |
| `ZeroDte(ctx, s, at, ...Option)` | `/v1/exposure/zero-dte/{s}` |
| `MaxPain(ctx, s, at, ...Option)` | `/v1/maxpain/{s}` |
| `StockSummary(ctx, s, at)` | `/v1/stock/{s}/summary` |
| `Volatility(ctx, s, at)` | `/v1/volatility/{s}` |
| `AdvVolatility(ctx, s, at)` | `/v1/adv_volatility/{s}` |
| `Vrp(ctx, s, at)` | `/v1/vrp/{s}` |

Filter helpers: `WithExpiration("2024-08-09")`, `WithMinOI(100)`,
`WithExpiry("2024-08-09")`, `WithStrike(520)`, `WithType("C")`,
`WithStrikeRange(0.05)`.

## Errors (use `errors.As`)

| Type | Status |
|---|---|
| `*APIError` | base — wraps everything |
| `*AuthenticationError` | 401 |
| `*TierRestrictedError` | 403 — needs Alpha plan |
| `*InvalidAtError` | 400 invalid_at |
| `*NoDataError` | 404 no_data |
| `*SymbolNotFoundError` | 404 symbol_not_found |
| `*NoCoverageError` | 404 no_coverage |
| `*InsufficientDataError` | 404 insufficient_data |
| `*RateLimitError` | 429 |
| `*ServerError` | 5xx |

## License

MIT
