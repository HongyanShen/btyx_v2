package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/modules/files"
	"baituber.cn/yanxuan/v2/internal/modules/wallet"
)

func TestTimeContractsAreStrings(t *testing.T) {
	for _, test := range []struct {
		value reflect.Type
		want  string
	}{
		{
			value: reflect.TypeOf(time.Time{}),
			want:  "string",
		},
		{
			value: reflect.TypeOf((*time.Time)(nil)),
			want:  "string | null",
		},
	} {
		t.Run(test.value.String(), func(t *testing.T) {
			if got := typeScriptType(test.value, nil); got != test.want {
				t.Fatalf("time contract = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTimeDTOsMarshalRFC3339WithZone(t *testing.T) {
	instant, err := time.Parse(time.RFC3339Nano, "2026-11-17T10:30:45.123+08:00")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		value any
		field string
	}{
		{
			name: "withdrawal",
			value: wallet.WithdrawalDTO{
				CreatedAt: instant,
			},
			field: "created_at",
		},
		{
			name: "upload",
			value: files.UploadTicket{
				ExpiresAt: &instant,
			},
			field: "expires_at",
		},
		{
			name: "download",
			value: files.DownloadDTO{
				ExpiresAt: &instant,
			},
			field: "expires_at",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}

			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}

			var wireTime string
			if err := json.Unmarshal(fields[test.field], &wireTime); err != nil {
				t.Fatalf("time must be a JSON string: %v", err)
			}

			decoded, err := time.Parse(time.RFC3339Nano, wireTime)
			if err != nil || !decoded.Equal(instant) {
				t.Fatalf("time lost its zone or instant: %q, %v", wireTime, err)
			}
		})
	}

	encoded, err := json.Marshal(files.UploadTicket{Completed: true})
	if err != nil {
		t.Fatal(err)
	}

	var ticket struct {
		ExpiresAt json.RawMessage `json:"expires_at"`
	}
	if err := json.Unmarshal(encoded, &ticket); err != nil {
		t.Fatal(err)
	}

	if string(ticket.ExpiresAt) != "null" {
		t.Fatalf("completed uploads must have null expiry: %s", ticket.ExpiresAt)
	}
}
