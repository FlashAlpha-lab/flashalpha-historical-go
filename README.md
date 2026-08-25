# flashalpha-historical-go

Official Go client for the **FlashAlpha Historical API** — point-in-time
replay of every live analytics endpoint. Ask what GEX, gamma flip, VRP,
narrative, max pain, or the full stock summary looked like at any **minute
back to 2017-01-03**, in the same response shape as the live API.

> **Point-in-time replay since 2017.** Backtest dealer positioning (GEX, VRP,
> vanna/charm, max pain) at any minute since 2017-01-03, then trade the same
> endpoints live. No look-ahead, no training-serving skew. The Historical API
> is an **Alpha tier** capability.

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

## Data provenance: `data_as_of`

Every successful response carries `data_as_of`, reporting when each upstream feed last
delivered to the node that answered, plus `endpoint_version` identifying the deployment
that produced it.

```go
gex, err := client.Gex(ctx, "SPY", "2024-03-15T14:30:00Z")

*gex.ArchiveAsOf.EquityOptionsFeed // "2024-03-15T14:29:58.100Z"  the rows replayed
*gex.ArchiveAsOf.OiFeed            // "2024-03-14T20:00:00.000Z"  prior session's close
gex.DataAsOf.EquityOptionsFeed     // nil - a replay node consumes no live feed
gex.EndpointVersion                // the deployment that answered
```

Every response type embeds `ResponseEnvelope`, so all three are promoted fields rather
than data `json.Unmarshal` silently drops. Feed fields are `*string`, so a class of data
the response did not read is `nil` rather than `""`.

| Field | Feed | Expected cadence |
|---|---|---|
| `node` | Which node answered | Nodes hydrate independently |
| `equity_feed` | Equity and ETF spot quotes | seconds, during market hours |
| `equity_options_feed` | Equity and ETF option quotes | seconds, during market hours |
| `index_feed` | Index spot (SPX, RUT, VIX and the other index roots) | seconds, during market hours |
| `index_options_feed` | Index option quotes | seconds, during market hours |
| `futures_feed` | Futures prices | seconds, during the futures session |
| `futures_options_feed` | Futures option quotes | seconds, during the futures session |
| `flow_feed` | Classified options and stock trade tape | seconds, during market hours |
| `oi_feed` | Settled open interest | daily, dated to the prior 16:00 ET close |
| `macro_feed` | VIX, VVIX, SKEW, MOVE, SPX, Fear & Greed | minutes; reports its OLDEST component |

Historical responses carry a second object, `archive_as_of`, in the same shape: the
vintage of the archive rows actually replayed for the timestamp you requested. Its
every feed in `data_as_of` is `null`, because a replay node reads the archive and consumes no
live feed.

`archive_as_of` is what makes an archive gap detectable. Request a moment with no row
and the query returns the most recent earlier row; nothing else in the response
distinguishes the two. Point-in-time work should read it and drop or flag observations
whose inputs precede the requested instant by more than the study tolerates.

### How to read it

- **Check the feeds your call depends on.** A GEX call on an equity is answered from
  `equity_feed`, `equity_options_feed` and `oi_feed`. `futures_feed` being `null` in that
  response says nothing about the answer.
- **Compare against the cadence, not the clock.** `oi_feed` at the previous session's
  close is correct: settled open interest is published once per session, so on a Monday
  the newest figure that exists is Friday's. An options feed an hour behind during the
  regular session is not correct.
- **`null` means "not seen on this node", not "broken".** A node that has never been
  asked for a futures symbol has never opened that feed.
- **Spot and options are separate on purpose.** They arrive over different pipes and can
  fail independently.
- **It evidences feed activity, not per-contract freshness.** An illiquid strike may not
  have quoted for hours while its feed is healthy.
- **`data_as_of` is not `as_of`.** `as_of` is response-generation time or the newest
  contract in the payload, depending on the endpoint. `data_as_of` describes the feeds
  behind it.

Full reference: <https://flashalpha.com/docs/lab-api-overview#response-envelope> and the
methodology whitepaper at <https://flashalpha.com/methodology#freshness-reporting>.
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

## Get access

The Historical API requires the **Alpha tier ($1,499/mo)**: the only public source
of aggregate vanna/charm exposure and point-in-time replay since 2017.

Quant teams, prop desks, and vol funds:
**[flashalpha.com/for-quant-teams](https://flashalpha.com/for-quant-teams?utm_source=github&utm_medium=readme&utm_campaign=repo-flashalpha-historical-go)**
