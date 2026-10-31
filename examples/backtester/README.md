# FlashAlpha options backtester (Go)

A complete, runnable backtester for the FlashAlpha Historical API. It replays the API day by day, decides entries
and exits from the same endpoint outputs you would read live, fills trades on real historical bid/ask quotes, and
writes `trades.csv` and `equity.csv`. It produces the same results as the Python version, to the cent.

Guide: [How to Backtest Options Strategies in Python](https://flashalpha.com/articles/how-to-backtest-options-strategies-python-flashalpha-historical-api).
The reference version is the [Python backtester](https://github.com/FlashAlpha-lab/flashalpha-historical-python/tree/main/examples/backtester).

Requirements: Go 1.21 or later (standard library only) and an Alpha plan or higher.

## Run it

```sh
export FLASHALPHA_API_KEY=your_key
go run . -symbol SPY -start 2024-01-02 -end 2024-06-28
```

On Windows PowerShell, set the key with `$env:FLASHALPHA_API_KEY = "your_key"`.

The run prints a JSON summary and writes `results/trades.csv` and `results/equity.csv`. Every API response is
cached in `.fa_cache/`, so a rerun with changed rules costs no API calls for days already fetched. The cache
files have the same names and contents as the Python version's, so both versions can share one cache folder.

To list every flag with its default, run `go run . -h`. The main ones:

| Flag | Default | Meaning |
|---|---|---|
| `-symbol` | `SPY` | Underlying |
| `-start`, `-end` | `2024-01-02`, `2024-06-28` | Date range, inclusive |
| `-max-positions` | `1` | Open positions allowed at once |
| `-short-delta`, `-width` | `0.2`, `5` | Short put delta, spread width in points |
| `-profit-target`, `-stop-loss`, `-exit-dte` | `0.5`, `2`, `21` | Exit settings |
| `-slippage`, `-commission` | `0.5`, `0.65` | Fill and cost settings |
| `-out`, `-cache` | `results`, `.fa_cache` | Output and cache folders |

## Change the strategy

All the strategy logic is in three functions in `main.go`, under the `STRATEGY: edit these three functions`
banner:

- `entryRules(snap, history, cfg)`: return `true` to open a position at this decision time.
- `selectLegs(chain, snap, cfg)`: return the option legs to trade, or `nil` for no trade.
- `exitRules(pos, snap, history, cfg)`: return a reason string to close the position, or `""` to hold.

The rules read a `Snapshot` built by `buildSnapshot`. To give the rules more signals, add fields there from any
other FlashAlpha endpoint. Snapshot values the API can leave out are pointers, so check for `nil` before you read
one: `snap.VRP20d != nil && *snap.VRP20d > 0`.

Everything below the `Engine` banner runs any strategy without changes.

## Run the tests

```sh
go test ./...
```

The tests in `main_test.go` need no API key or network. A stub that implements the `DataSource` interface replaces
the API with three canned sessions. The tests check the fill math, leg selection, one full trade with exact
accounting (credit 56.70, P&L 32.90), strike formatting, and holiday skipping. To test your own rules offline,
point the stub at the market conditions you want and call `run` with it.
