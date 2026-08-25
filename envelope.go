package flashalphahistorical

// DataAsOf reports when each upstream feed last delivered to the node that served
// the response.
//
// On this replay service every feed is nil: a replay node reads the archive and
// consumes no live feed. The object is still returned so the envelope has one shape
// across the live and historical services, and so a historical response cannot be
// mistaken for a live one.
//
// The vintage that matters here is ArchiveAsOf, carried alongside it as
// archive_as_of.
type DataAsOf struct {
	// Node identifies which node answered.
	Node string `json:"node"`
	// EquityFeed covers equity and ETF spot quotes.
	EquityFeed *string `json:"equity_feed"`
	// EquityOptionsFeed covers equity and ETF option quotes.
	EquityOptionsFeed *string `json:"equity_options_feed"`
	// IndexFeed covers index spot - SPX, NDX, RUT, VIX.
	IndexFeed *string `json:"index_feed"`
	// IndexOptionsFeed covers index option quotes.
	IndexOptionsFeed *string `json:"index_options_feed"`
	// FuturesFeed covers futures prices.
	FuturesFeed *string `json:"futures_feed"`
	// FuturesOptionsFeed covers futures option quotes.
	FuturesOptionsFeed *string `json:"futures_options_feed"`
	// FlowFeed covers the classified options and stock trade tape.
	FlowFeed *string `json:"flow_feed"`
	// OiFeed covers settled open interest.
	OiFeed *string `json:"oi_feed"`
	// MacroFeed covers VIX, VVIX, SKEW, MOVE, SPX and Fear & Greed.
	MacroFeed *string `json:"macro_feed"`
}

// ArchiveAsOf is the vintage of the archive rows actually replayed for the timestamp
// you requested.
//
// Same shape as DataAsOf - the key order is a contract shared with the live service -
// but the values describe stored rows rather than live feeds. A field is nil when the
// response did not read that class of data.
//
// This is what makes an archive gap detectable. Request a moment with no row and the
// query returns the most recent earlier row; nothing else in the response
// distinguishes the two. Point-in-time work should read this and drop or flag
// observations whose inputs precede the requested instant by more than the study
// tolerates.
//
// OiFeed trailing by a session is correct rather than a gap: settled open interest is
// published once per session, so the newest figure that existed at any intraday moment
// is the prior close.
type ArchiveAsOf = DataAsOf

// ResponseEnvelope is embedded in every response type and carries the envelope the API
// returns on all successful responses. Because it is embedded anonymously,
// encoding/json flattens it: the wire shape is unchanged and the fields are promoted,
// so they read as gex.ArchiveAsOf and gex.EndpointVersion.
type ResponseEnvelope struct {
	// EndpointVersion identifies the deployment that produced this response.
	EndpointVersion string `json:"endpoint_version,omitempty"`
	// DataAsOf is the live-feed freshness. All nil on this replay service.
	DataAsOf *DataAsOf `json:"data_as_of,omitempty"`
	// ArchiveAsOf is the vintage of the archive rows actually replayed.
	ArchiveAsOf *ArchiveAsOf `json:"archive_as_of,omitempty"`
}
