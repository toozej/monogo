package search

import (
	"testing"
)

func TestRandomCommonItem(t *testing.T) {
	item := RandomCommonItem(nil)
	if item == "" {
		t.Error("RandomCommonItem(nil) returned empty string")
	}

	found := false
	for _, ci := range DefaultCommonItems {
		if ci == item {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("RandomCommonItem(nil) returned '%s' which is not in DefaultCommonItems list", item)
	}
}

func TestRandomCommonItemWithCustomList(t *testing.T) {
	customItems := []string{"ITEM_A", "ITEM_B", "ITEM_C"}
	item := RandomCommonItem(customItems)

	found := false
	for _, ci := range customItems {
		if ci == item {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("RandomCommonItem() returned '%s' which is not in custom list", item)
	}
}

func TestDefaultCommonItemsNotEmpty(t *testing.T) {
	if len(DefaultCommonItems) == 0 {
		t.Error("DefaultCommonItems list should not be empty")
	}
}

func TestRandomCommonItemDistribution(t *testing.T) {
	results := make(map[string]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		item := RandomCommonItem(nil)
		results[item]++
	}

	if len(results) < 2 {
		t.Errorf("Expected randomness across items, but only %d unique item(s) selected in %d iterations", len(results), iterations)
	}
}
