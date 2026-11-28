package identity_test

import (
	"context"
	"testing"

	"baituber.cn/yanxuan/v2/internal/testkit"
	"go.opentelemetry.io/otel/attribute"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTraceIdentifiesOnlyConfirmedAccounts(t *testing.T) {
	s := testkit.Service(t, "../../../..")
	ctx := context.Background()
	if err := s.BootstrapAdmin(ctx, "trace-admin", "trace-password"); err != nil {
		t.Fatal(err)
	}

	recorder := tracetest.NewSpanRecorder()
	tracer := sdk.NewTracerProvider(sdk.WithSpanProcessor(recorder)).Tracer("identity-test")
	check := func(id, role string) {
		t.Helper()
		spans := recorder.Ended()
		attrs := attribute.NewSet(spans[len(spans)-1].Attributes()...)
		user, hasUser := attrs.Value("user.id")
		roles, hasRole := attrs.Value("user.roles")
		if hasUser != (id != "") || hasRole != (role != "") || user.AsString() != id {
			t.Fatalf("unexpected trace identity: %v", attrs.ToSlice())
		}

		if hasRole && (len(roles.AsStringSlice()) != 1 || roles.AsStringSlice()[0] != role) {
			t.Fatalf("unexpected trace roles: %v", roles)
		}
	}
	loginCtx, span := tracer.Start(ctx, "user login")
	member, err := s.DemoLogin(loginCtx, "trace-user")
	span.End()
	if err != nil {
		t.Fatal(err)
	}

	check(member.UserID, "user")
	loginCtx, span = tracer.Start(ctx, "admin login")
	operator, err := s.LoginAdmin(loginCtx, "trace-admin", "trace-password")
	span.End()
	if err != nil {
		t.Fatal(err)
	}

	check(operator.UserID, "admin")
	for _, account := range []struct{ token, id, role string }{
		{member.Token, member.UserID, "user"},
		{operator.Token, operator.UserID, "admin"},
		{"invalid-session", "", ""},
	} {
		authCtx, request := tracer.Start(ctx, "session")
		_, err := s.Authenticate(authCtx, account.token)
		request.End()
		if (err != nil) != (account.id == "") {
			t.Fatalf("unexpected authentication result: %v", err)
		}

		check(account.id, account.role)
	}

	loginCtx, span = tracer.Start(ctx, "failed login")
	_, err = s.LoginAdmin(loginCtx, "trace-admin", "wrong-password")
	span.End()
	if err == nil {
		t.Fatal("invalid credentials accepted")
	}

	check("", "")
}
