package files

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/assignment"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/ent/creativetask"
	"baituber.cn/yanxuan/v2/ent/creativework"
	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/ent/marketingsettings"
	"baituber.cn/yanxuan/v2/ent/operator"
	"baituber.cn/yanxuan/v2/ent/taskversion"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/ent/userverification"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
)

func ValidateOperatorReference(ctx context.Context, tx *ent.Tx, operatorID, id string) error {
	allowed, err := tx.FileObject.Query().Where(
		fileobject.IDEQ(id),
		fileobject.StatusEQ(enum.FileReady),
		fileobject.Or(
			fileobject.OwnerIDIn(operatorID, "admin"),
			func(selector *sql.Selector) {
				operators := sql.Table(operator.Table)
				operatorIDs := sql.Select(operators.C(operator.FieldID)).From(operators)
				selector.Where(sql.In(selector.C(fileobject.FieldOwnerID), operatorIDs))
			},
		),
	).Exist(ctx)
	if err != nil {
		return err
	}

	if !allowed {
		return app.Invalid("文件尚未确认、已失效或无访问权限")
	}

	return nil
}

func (s *Service) CheckOwnedFile(ctx context.Context, userID, id string) error {
	if id == "" {
		return nil
	}

	record, err := s.Client.FileObject.Get(ctx, id)
	if err != nil || record.OwnerID != userID || record.Status != enum.FileReady {
		return app.NotFound()
	}

	return nil
}

func (s *Service) FileForRead(ctx context.Context, userID string, role enum.Role, id string) (*ent.FileObject, error) {
	record, err := s.Client.FileObject.Get(ctx, id)
	if err != nil || record.Status != enum.FileReady {
		return nil, app.NotFound()
	}

	if role == enum.RoleAdmin {
		if record.IdentityPrivate {
			return nil, app.NotFound()
		}

		identityPhoto, err := s.Client.UserVerification.Query().
			Where(
				userverification.UserIDEQ(record.OwnerID),
				userverification.Or(
					userverification.FrontFileIDEQ(id),
					userverification.BackFileIDEQ(id),
				),
			).
			Exist(ctx)
		if err != nil {
			return nil, err
		}

		if identityPhoto {
			return nil, app.NotFound()
		}

		enrollmentPhoto, err := s.Client.UserEnrollment.Query().
			Where(
				userenrollment.UserIDEQ(record.OwnerID),
				userenrollment.Or(
					userenrollment.FrontFileIDEQ(id),
					userenrollment.BackFileIDEQ(id),
				),
			).
			Exist(ctx)
		if err != nil {
			return nil, err
		}

		if enrollmentPhoto {
			return nil, app.NotFound()
		}

		historicalPhoto, err := s.Client.AuditEvent.Query().
			Where(
				auditevent.ReferenceIDEQ(record.OwnerID),
				auditevent.Or(
					auditevent.SubmittedFrontFileIDEQ(id),
					auditevent.SubmittedBackFileIDEQ(id),
				),
			).
			Exist(ctx)
		if err != nil {
			return nil, err
		}

		if historicalPhoto {
			return nil, app.NotFound()
		}

		return record, nil
	}

	if record.OwnerID == userID {
		return record, nil
	}

	allowed, err := s.Client.Assignment.Query().
		Where(
			assignment.UserIDEQ(userID),
			assignment.StatusNEQ(enum.AssignmentRevoked),
			func(selector *sql.Selector) {
				versions := sql.Table(taskversion.Table)
				matchingVersions := sql.Select(versions.C(taskversion.FieldID)).
					From(versions).
					Where(sqljson.ValueContains(versions.C(taskversion.FieldFiles), id))
				selector.Where(sql.In(selector.C(assignment.FieldVersionID), matchingVersions))
			},
		).
		Exist(ctx)
	if err != nil {
		return nil, err
	}

	if allowed {
		return record, nil
	}

	policy, err := s.Client.MarketingSettings.Query().Where(marketingsettings.ScopeEQ("marketing")).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	available := policy == nil || policy.CreativeOnline
	allowed, err = s.Client.CreativeTask.Query().Where(func(sel *sql.Selector) {
		work := sql.Table(creativework.Table)
		mine := sql.Select(work.C(creativework.FieldTaskID)).From(work).Where(sql.EQ(work.C(creativework.FieldUserID), userID))
		access := sql.In(sel.C(creativetask.FieldID), mine)
		if available {
			access = sql.Or(access, sql.And(sql.EQ(sel.C(creativetask.FieldOnline), true), sql.LTE(sel.C(creativetask.FieldStartsAt), s.Now()), sql.GT(sel.C(creativetask.FieldEndsAt), s.Now())))
		}

		sel.Where(sql.And(access, sqljson.ValueContains(sel.C(creativetask.FieldFiles), id)))
	}).Exist(ctx)
	if err != nil {
		return nil, err
	}

	if allowed {
		return record, nil
	}

	return nil, app.NotFound()
}
