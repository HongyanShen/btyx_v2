package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	minihandler "baituber.cn/yanxuan/v2/cmd/miniapi/handler"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	walletent "baituber.cn/yanxuan/v2/ent/wallet"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/wallet"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/shopspring/decimal"
)

func TestWithdrawalAPIReplaysFreezeAndIsolatesMemberRecords(t *testing.T) {
	ctx := context.Background()
	services := testkit.Service(t, "../../../..")
	member, err := services.DemoLogin(ctx, "withdrawal-owner")
	if err != nil {
		t.Fatal(err)
	}

	other, err := services.DemoLogin(ctx, "withdrawal-other")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := services.Client.Wallet.Update().
		Where(walletent.UserIDEQ(member.UserID)).
		SetAvailableFen(decimal.RequireFromString("1500")).
		Save(ctx); err != nil {
		t.Fatal(err)
	}

	router := minihandler.NewRouter(services)
	call := func(method, path, token, key, body string, wantStatus int, data any) {
		t.Helper()

		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}

		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != wantStatus {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, response.Code, wantStatus, response.Body.String())
		}

		if data != nil {
			var envelope struct {
				Code int             `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Code != 0 {
				t.Fatalf("invalid response envelope: %s", response.Body.String())
			}

			if err := json.Unmarshal(envelope.Data, data); err != nil {
				t.Fatal(err)
			}
		}
	}

	const amount = `{"amount_fen":"1000"}`
	var created, replayed wallet.WithdrawalDTO
	call(http.MethodGet, "/withdrawals", "", "", "", http.StatusUnauthorized, nil)
	call(http.MethodPost, "/withdrawals", member.Token, "same-withdrawal", amount, http.StatusOK, &created)
	event, err := services.Client.AuditEvent.Query().
		Where(auditevent.ReferenceIDEQ(created.ID), auditevent.ActionEQ(enum.AuditWithdrawalRequest)).
		Only(ctx)
	if err != nil || event.Platform != enum.AuditPlatformMiniapp {
		t.Fatalf("withdrawal request did not record its miniapp platform: %v", err)
	}

	call(http.MethodPost, "/withdrawals", member.Token, "same-withdrawal", amount, http.StatusOK, &replayed)
	call(http.MethodPost, "/withdrawals", member.Token, "different-withdrawal", amount, http.StatusConflict, nil)
	failed, err := services.Client.AuditEvent.Query().
		Where(auditevent.ActorIDEQ(member.UserID), auditevent.ActionEQ(enum.AuditRequestFailed)).
		Only(ctx)
	if err != nil || failed.Platform != enum.AuditPlatformMiniapp || failed.Endpoint != "POST /api/v1/withdrawals" || failed.Reason == "" {
		t.Fatalf("conflicting withdrawal must leave one separate failed audit: %v", err)
	}

	if created.ID == "" || created.ID != replayed.ID || created.Status != enum.WithdrawalQueued {
		t.Fatalf("withdrawal replay returned a different application: %+v, %+v", created, replayed)
	}

	var balance wallet.WalletDTO
	call(http.MethodGet, "/wallet", member.Token, "", "", http.StatusOK, &balance)
	if balance.AvailableFen != "500" || balance.FrozenFen != "1000" {
		t.Fatalf("incorrect frozen balance: %+v", balance)
	}

	var records []wallet.WithdrawalDTO
	call(http.MethodGet, "/withdrawals", other.Token, "", "", http.StatusOK, &records)
	if records == nil || len(records) != 0 {
		t.Fatalf("other member received records: %+v", records)
	}

	call(http.MethodGet, "/withdrawals", member.Token, "", "", http.StatusOK, &records)
	if len(records) != 1 || records[0].ID != created.ID || records[0].Status != enum.WithdrawalQueued {
		t.Fatalf("expected exactly one visible application: %+v", records)
	}

	for range 2 {
		if err := services.ApplyTransferResult(ctx, created.ID, enum.WithdrawalCancelled, "provider-test-id"); err != nil {
			t.Fatal(err)
		}
	}

	call(http.MethodGet, "/wallet", member.Token, "", "", http.StatusOK, &balance)
	call(http.MethodGet, "/withdrawals", member.Token, "", "", http.StatusOK, &records)
	if balance.AvailableFen != "1500" || balance.FrozenFen != "0" || len(records) != 1 || records[0].Status != enum.WithdrawalCancelled {
		t.Fatalf("cancel did not return the original amount exactly once: %+v, %+v", balance, records)
	}
}
