package tos

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

func TestObjectRequestsUseConfiguredHTTPSProxy(t *testing.T) {
	const host = "test-bucket.tos-proxy-test.invalid:443"
	requests := make(chan string, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requests <- r.Method + " " + r.Host:
		default:
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")

	gateway, err := New(config.ObjectStorage{
		Bucket:    "test-bucket",
		Region:    "cn-beijing",
		Endpoint:  "https://tos-proxy-test.invalid",
		AccessKey: "unit-test-access",
		SecretKey: "unit-test-secret",
		Prefix:    "btyx-v2/test/",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_, err = gateway.Inspect(ctx, objectstore.Reference{
		Bucket: "test-bucket",
		Key:    "btyx-v2/test/uploads/image.png",
	})
	if err == nil {
		t.Fatal("the diagnostic proxy must reject the connection")
	}

	select {
	case request := <-requests:
		if request != "CONNECT "+host {
			t.Fatalf("unexpected proxy request: %s", request)
		}
	default:
		t.Fatal("the TOS client did not use HTTPS_PROXY")
	}
}

func TestOfficialSDKPolicyRestrictsObjectACLTypeAndSize(t *testing.T) {
	gateway, err := New(config.ObjectStorage{
		Bucket:    "test-bucket",
		Region:    "cn-beijing",
		Endpoint:  "https://tos-cn-beijing.volces.com",
		AccessKey: "unit-test-access",
		SecretKey: "unit-test-secret",
		Prefix:    "btyx-v2/test/",
	})
	if err != nil {
		t.Fatal(err)
	}

	form, err := gateway.SignUpload(context.Background(), objectstore.UploadRequest{
		Object: objectstore.Reference{
			Bucket: "test-bucket",
			Key:    "btyx-v2/test/uploads/unique.png",
		},
		Size:      1024,
		MediaType: enum.FilePNG,
		Expires:   5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	policy, err := base64.StdEncoding.DecodeString(form.Fields.Policy)
	if err != nil {
		t.Fatal(err)
	}

	for _, condition := range []string{
		`"bucket":"test-bucket"`,
		`"key":"btyx-v2/test/uploads/unique.png"`,
		`"acl":"private"`,
		`"Content-Type":"image/png"`,
		`["content-length-range",1024,1024]`,
		`"x-tos-forbid-overwrite":"true"`,
	} {
		if !bytes.Contains(policy, []byte(condition)) {
			t.Fatalf("missing policy restriction: %s", condition)
		}
	}

	if form.Fields.Signature == "" || form.URL != "https://test-bucket.tos-cn-beijing.volces.com/" {
		t.Fatal("official SDK did not produce a scoped upload form")
	}

	if bytes.Contains(policy, []byte("unit-test-secret")) {
		t.Fatal("secret key exposed in upload policy")
	}
}
