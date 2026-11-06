// Replay sell-window for one session. The backtests entered at 09:35 ET, so T09:35:00
// returns the backtest's entry strikes.
//
//	FLASHALPHA_API_KEY=... go run ./examples/sell_window_replay SPY 2026-10-16
//
// Docs: https://flashalpha.com/docs/historical-sell-window
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	fh "github.com/FlashAlpha-lab/flashalpha-historical-go"
)

func main() {
	symbol, session := "SPY", "2026-10-16"
	if len(os.Args) > 2 {
		symbol, session = os.Args[1], os.Args[2]
	}

	client := fh.NewClient(os.Getenv("FLASHALPHA_API_KEY"))
	answer, err := client.SellWindowTyped(context.Background(), symbol, session+"T09:35:00")
	if err != nil {
		log.Fatal(err)
	}

	for _, window := range answer.Windows {
		expiry := "no listed expiry"
		if window.Expiry != nil {
			expiry = *window.Expiry
		}
		fmt.Printf("%s: %s\n", expiry, window.Action)
		for _, trade := range window.Candidates {
			fmt.Printf("  %s, exit: %s\n", trade.Structure, trade.Exit)
		}
	}
}
