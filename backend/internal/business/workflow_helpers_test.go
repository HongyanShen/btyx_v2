package business

import (
	"context"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/database"
)

func OpenTest(ctx context.Context, path string) (*Service, func(), error) {
	c, e := config.Load(path)
	if e != nil {
		return nil, nil, e
	}

	c.Schema = "btyx_test_" + NewID()
	// PostgreSQL folds unquoted identifiers; schema names use lower case throughout.
	c.Schema = strings.ToLower(c.Schema)
	c.Demo = true
	if e = database.Migrate(ctx, c); e != nil {
		return nil, nil, e
	}

	s, e := New(ctx, c)
	if e != nil {
		database.DropTest(ctx, c)
		return nil, nil, e
	}

	s.Now = func() time.Time {
		return time.Date(2026, 11, 16, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	}
	_, e = s.Client.Agreement.Create().
		SetTitle("测试协议").
		SetBody("测试营销任务协议").
		SetCreatorID("admin").
		Save(ctx)
	return s, func() {
		s.Close()
		database.DropTest(context.Background(), c)
	}, e
}

func (s *Service) PrepareTestUser(ctx context.Context, uid string) error {
	current, err := s.Client.UserEnrollment.Query().
		Where(userenrollment.UserIDEQ(uid), userenrollment.IsCurrentEQ(true)).
		Only(ctx)
	if ent.IsNotFound(err) {
		err = s.Client.UserEnrollment.Create().SetUserID(uid).SetStatus(enum.EnrollmentApproved).Exec(ctx)
	} else if err == nil {
		err = s.Client.UserEnrollment.UpdateOne(current).SetStatus(enum.EnrollmentApproved).Exec(ctx)
	}

	if err != nil {
		return err
	}

	return s.Consent(ctx, uid)
}
