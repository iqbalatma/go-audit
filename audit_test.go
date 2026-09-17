package audit

import "testing"

func TestDiffMapsOnlyChangedFields(t *testing.T) {
	before := map[string]any{"name": "old", "price": float64(100), "unchanged": "x"}
	after := map[string]any{"name": "new", "price": float64(100), "unchanged": "x"}

	db, da := diffMaps(before, after)

	if len(db) != 1 || db["name"] != "old" {
		t.Fatalf("diffBefore = %v, want only {name: old}", db)
	}
	if len(da) != 1 || da["name"] != "new" {
		t.Fatalf("diffAfter = %v, want only {name: new}", da)
	}
}

func TestDiffMapsSkipsNestedFields(t *testing.T) {
	before := map[string]any{"meta": map[string]any{"a": float64(1)}}
	after := map[string]any{"meta": map[string]any{"a": float64(2)}}

	db, da := diffMaps(before, after)

	if len(db) != 0 || len(da) != 0 {
		t.Fatalf("expected nested field to be skipped, got before=%v after=%v", db, da)
	}
}

func TestAddSingleTrailNoChangeAddsNoTrail(t *testing.T) {
	a := &Audit{}
	a.AddSingleTrail("products", 1, map[string]any{"name": "x"}, map[string]any{"name": "x"}, nil, nil)

	if len(a.trails) != 0 {
		t.Fatalf("expected 0 trails when nothing changed, got %d", len(a.trails))
	}
}

func TestAddSingleTrailCreateStoresFullAfter(t *testing.T) {
	a := &Audit{}
	a.AddSingleTrail("products", 1, nil, map[string]any{"name": "x"}, nil, nil)

	if len(a.trails) != 1 {
		t.Fatalf("expected 1 trail, got %d", len(a.trails))
	}
	if a.trails[0].Before != nil {
		t.Fatalf("expected nil before on create, got %v", a.trails[0].Before)
	}
}

func TestAddSingleTrailStoresPerTrailTag(t *testing.T) {
	a := &Audit{}
	a.AddSingleTrail("products", 1, nil, map[string]any{"name": "x"}, map[string]any{"level": "important"}, nil)

	if got := a.trails[0].Tag["level"]; got != "important" {
		t.Fatalf("trail tag = %v, want important", got)
	}
}
