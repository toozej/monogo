package matcher

import (
	"testing"

	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

func TestMatchTrailsAllowsRawOnlyChecklist(t *testing.T) {
	raw := []types.Trail{{Name: "Uncompleted Trail", Park: "Forest Park"}}
	combined, err := MatchTrails(nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(combined) != 1 || combined[0].Name != raw[0].Name {
		t.Fatalf("combined trails = %#v", combined)
	}
}
