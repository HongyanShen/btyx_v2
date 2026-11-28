package creative

import "testing"

func TestRewardsUseHighestReachedTier(t *testing.T) {
	tiers := []Tier{{
		Likes:     3000,
		RewardFen: "10000",
	}, {
		Likes:     5000,
		RewardFen: "26600",
	}}
	if got := reward(tiers, 6800).String(); got != "26600" {
		t.Fatal(got)
	}

	if got := reward(tiers, 1200).String(); got != "0" {
		t.Fatal(got)
	}
}
