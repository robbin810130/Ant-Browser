package workspace

import "testing"

func TestBuildRunEvidenceIndexSelectsLatestByShopAndTask(t *testing.T) {
	index := BuildRunEvidenceIndex([]RunRecord{
		{RunID: "old-open", ShopID: "shop-001", TaskType: "open", Status: "failed", StartedAt: "2026-05-23T00:00:00Z", FailureCode: "ANT_OPEN_FAILED"},
		{RunID: "new-open", ShopID: "shop-001", TaskType: "open", Status: "succeeded", StartedAt: "2026-05-23T00:01:00Z"},
		{RunID: "validate-1", ShopID: "shop-001", TaskType: "validate", Status: "failed", StartedAt: "2026-05-23T00:02:00Z", FailureCode: "VALIDATION_FAILED"},
	})

	shop := index.ByShop["shop-001"]
	if shop.LatestOpen == nil || shop.LatestOpen.RunID != "new-open" {
		t.Fatalf("unexpected latest open: %#v", shop.LatestOpen)
	}
	if shop.LatestValidation == nil || shop.LatestValidation.RunID != "validate-1" {
		t.Fatalf("unexpected latest validation: %#v", shop.LatestValidation)
	}
	if shop.LatestFailure == nil || shop.LatestFailure.RunID != "validate-1" {
		t.Fatalf("unexpected latest failure: %#v", shop.LatestFailure)
	}
}

func TestBuildRunEvidenceIndexTracksActiveRuns(t *testing.T) {
	index := BuildRunEvidenceIndex([]RunRecord{
		{RunID: "run-active", ShopID: "shop-001", TaskType: "open", Status: "launching", StartedAt: "2026-05-23T00:01:00Z"},
		{RunID: "run-done", ShopID: "shop-001", TaskType: "open", Status: "succeeded", StartedAt: "2026-05-23T00:00:00Z"},
	})

	shop := index.ByShop["shop-001"]
	if shop.ActiveRun == nil || shop.ActiveRun.RunID != "run-active" {
		t.Fatalf("unexpected active run: %#v", shop.ActiveRun)
	}
}
