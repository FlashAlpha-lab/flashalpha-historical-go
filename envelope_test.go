package flashalphahistorical

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

const envelopeBody = `{
  "symbol": "SPY",
  "net_gex": 1234.5,
  "endpoint_version": "2026.08.25",
  "data_as_of": {
    "node": "f3",
    "equity_feed": null,
    "equity_options_feed": null,
    "index_feed": null,
    "index_options_feed": null,
    "futures_feed": null,
    "futures_options_feed": null,
    "flow_feed": null,
    "oi_feed": null,
    "macro_feed": null
  },
  "archive_as_of": {
    "node": "f3",
    "equity_feed": "2024-03-15T14:29:59.500Z",
    "equity_options_feed": "2024-03-15T14:29:58.100Z",
    "index_feed": null,
    "index_options_feed": null,
    "futures_feed": null,
    "futures_options_feed": null,
    "flow_feed": null,
    "oi_feed": "2024-03-14T20:00:00.000Z",
    "macro_feed": null
  }
}`

// The envelope is embedded rather than repeated on each response type. That only works
// if encoding/json flattens the anonymous field, so this exercises the unmarshal rather
// than the declaration.
func TestEnvelopeFlattensThroughEmbedding(t *testing.T) {
	var gex GexResponse
	if err := json.Unmarshal([]byte(envelopeBody), &gex); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if gex.EndpointVersion != "2026.08.25" {
		t.Errorf("endpoint_version = %q, want 2026.08.25", gex.EndpointVersion)
	}
	if gex.ArchiveAsOf == nil {
		t.Fatal("archive_as_of is nil; embedding did not flatten")
	}
	if gex.ArchiveAsOf.Node != "f3" {
		t.Errorf("node = %q, want f3", gex.ArchiveAsOf.Node)
	}
	if got := deref(gex.ArchiveAsOf.EquityOptionsFeed); got != "2024-03-15T14:29:58.100Z" {
		t.Errorf("archive equity_options_feed = %q", got)
	}
}

// A replay node reads the archive and consumes no live feed, so every live feed is nil.
// The object is still returned, and that all-nil shape is what stops a historical
// response being mistaken for a live one - so it must survive as a struct with nil
// fields rather than collapsing to a nil pointer.
func TestLiveFeedsAreAllNilButTheObjectSurvives(t *testing.T) {
	var gex GexResponse
	if err := json.Unmarshal([]byte(envelopeBody), &gex); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if gex.DataAsOf == nil {
		t.Fatal("data_as_of collapsed to nil; the all-nil object is what marks a replayed response")
	}
	for name, got := range map[string]*string{
		"equity_feed":         gex.DataAsOf.EquityFeed,
		"equity_options_feed": gex.DataAsOf.EquityOptionsFeed,
		"oi_feed":             gex.DataAsOf.OiFeed,
		"macro_feed":          gex.DataAsOf.MacroFeed,
	} {
		if got != nil {
			t.Errorf("live %s = %q, want nil on a replay node", name, *got)
		}
	}
}

// The archive vintage is what makes a gap detectable, so it must pass through untouched
// rather than being normalised toward the requested instant. Normalising it would erase
// exactly the signal a point-in-time study reads.
func TestArchiveVintagePassesThroughUnmodified(t *testing.T) {
	var gex GexResponse
	if err := json.Unmarshal([]byte(envelopeBody), &gex); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := deref(gex.ArchiveAsOf.EquityFeed); got != "2024-03-15T14:29:59.500Z" {
		t.Errorf("archive equity_feed = %q", got)
	}
	if got := deref(gex.ArchiveAsOf.OiFeed); got != "2024-03-14T20:00:00.000Z" {
		t.Errorf("archive oi_feed = %q, want the prior session close passed through", got)
	}
}

// A response that did not read a class of data reports nil for it.
func TestUnreadDataClassesStayNil(t *testing.T) {
	var gex GexResponse
	if err := json.Unmarshal([]byte(envelopeBody), &gex); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if gex.ArchiveAsOf.FuturesFeed != nil || gex.ArchiveAsOf.FlowFeed != nil {
		t.Error("unread data classes should be nil")
	}
}

// Responses predating the envelope must still unmarshal.
func TestPreEnvelopeResponsesStillUnmarshal(t *testing.T) {
	var gex GexResponse
	if err := json.Unmarshal([]byte(`{"symbol":"SPY","net_gex":1.0}`), &gex); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if gex.DataAsOf != nil || gex.ArchiveAsOf != nil || gex.EndpointVersion != "" {
		t.Error("envelope should be zero-valued when absent")
	}
}

// Marshalling must not nest the envelope under a "ResponseEnvelope" key. Embedding only
// preserves the wire shape while the field stays anonymous; naming it would silently
// change every serialized payload.
func TestEnvelopeDoesNotNestOnMarshal(t *testing.T) {
	var gex GexResponse
	if err := json.Unmarshal([]byte(envelopeBody), &gex); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	out, err := json.Marshal(gex)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "ResponseEnvelope") {
		t.Errorf("envelope nested instead of flattened: %s", out)
	}
	if !strings.Contains(string(out), `"archive_as_of"`) {
		t.Errorf("archive_as_of missing from output: %s", out)
	}
}

// Guard the sweep itself: every exported *Response struct must embed the envelope. Go
// cannot enumerate a package's types at runtime, so this parses the source - which is
// the stronger check anyway, since it reads the declarations directly. Trusting that one
// regex touched every file is exactly the assumption worth testing.
func TestEveryResponseTypeEmbedsTheEnvelope(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	checked := 0
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				spec, ok := n.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(spec.Name.Name, "Response") {
					return true
				}
				st, ok := spec.Type.(*ast.StructType)
				if !ok {
					return true
				}
				checked++
				for _, f := range st.Fields.List {
					if len(f.Names) != 0 {
						continue // named field, not an embed
					}
					if id, ok := f.Type.(*ast.Ident); ok && id.Name == "ResponseEnvelope" {
						return true
					}
				}
				t.Errorf("%s does not embed ResponseEnvelope", spec.Name.Name)
				return true
			})
		}
	}

	if checked == 0 {
		t.Fatal("parsed no *Response structs; the guard is not actually checking anything")
	}
	t.Logf("checked %d response structs", checked)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
