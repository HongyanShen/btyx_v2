package identity_test

import (
	"context"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/testkit"
)

func TestSessionExpiryUsesTheSameInstantAcrossZones(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	ctx := context.Background()
	createdAt, err := time.Parse(time.RFC3339Nano, "2026-11-17T23:59:59.123+08:00")
	if err != nil {
		t.Fatal(err)
	}

	services.Now = func() time.Time {
		return createdAt
	}
	login, err := services.DemoLogin(ctx, "session-timezone")
	if err != nil {
		t.Fatal(err)
	}

	expiresAt := createdAt.Add(24 * time.Hour)
	services.Now = func() time.Time {
		return expiresAt.Add(-time.Millisecond).UTC()
	}
	if _, err := services.Authenticate(ctx, login.Token); err != nil {
		t.Fatalf("session expired before its UTC-equivalent deadline: %v", err)
	}

	services.Now = func() time.Time {
		return expiresAt.UTC()
	}
	if _, err := services.Authenticate(ctx, login.Token); err == nil {
		t.Fatal("session remained valid at its expiry instant")
	}
}
