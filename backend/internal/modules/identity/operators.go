package identity

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/operator"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
	"golang.org/x/crypto/bcrypt"
)

var operatorUsername = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

// A public test hash makes an unknown username take the same bcrypt path.
const unknownOperatorHash = "$2a$10$XajjQh1JFpcqT5yyE/FR.OWxh2vDThv0E/4lZfbY6WZvgmntFA/s."

func (s *Service) LoginAdmin(ctx context.Context, username, password string) (LoginResult, error) {
	username = normalizeOperatorUsername(username)
	if username == "" || password == "" {
		return LoginResult{}, app.Invalid("请输入用户名和密码")
	}

	if !operatorUsername.MatchString(username) || len(password) > 72 {
		return LoginResult{}, invalidCredentials()
	}

	record, err := s.Client.Operator.Query().
		Where(operator.UsernameEQ(username)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return LoginResult{}, err
	}

	passwordHash := unknownOperatorHash
	if record != nil {
		passwordHash = record.PasswordHash
	}

	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil || record == nil || record.Disabled {
		return LoginResult{}, invalidCredentials()
	}

	login, err := app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (LoginResult, error) {
		locked, err := tx.Operator.Query().
			Where(operator.IDEQ(record.ID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return LoginResult{}, err
		}

		if locked.Disabled || locked.PasswordHash != passwordHash {
			return LoginResult{}, invalidCredentials()
		}

		return s.createSession(ctx, tx.Session, locked.ID, enum.RoleAdmin)
	})
	if err == nil {
		telemetry.SetUser(ctx, login.UserID, login.Role)
	}

	return login, err
}

func (s *Service) BootstrapAdmin(ctx context.Context, username, password string) error {
	username = normalizeOperatorUsername(username)
	if !operatorUsername.MatchString(username) {
		return app.Invalid("用户名须为 3 至 64 位字母、数字、点、下划线或连字符，并以字母或数字开头")
	}

	passwordHash, err := hashOperatorPassword(password)
	if err != nil {
		return err
	}

	_, err = app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (bool, error) {
		record, err := tx.Operator.Create().
			SetUsername(username).
			SetPasswordHash(passwordHash).
			SetSuperAdmin(true).
			Save(ctx)
		if ent.IsConstraintError(err) {
			return false, app.Conflict("超级管理员已初始化或用户名已存在")
		}

		if err != nil {
			return false, err
		}

		return true, app.Audit(ctx, tx, record.ID, enum.AuditOperatorCreate, record.ID, "", "权限：超级管理员")
	})

	return err
}

func (s *Service) Operators(ctx context.Context) ([]OperatorDTO, error) {
	records, err := s.Client.Operator.Query().
		Order(ent.Desc(operator.FieldSuperAdmin), ent.Asc(operator.FieldUsername)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]OperatorDTO, 0, len(records))
	for _, record := range records {
		result = append(result, operatorDTO(record))
	}

	return result, nil
}

func (s *Service) CreateOperator(ctx context.Context, actorID string, input CreateOperatorInput) (OperatorDTO, error) {
	username := normalizeOperatorUsername(input.Username)
	if !operatorUsername.MatchString(username) {
		return OperatorDTO{}, app.Invalid("用户名须为 3 至 64 位字母、数字、点、下划线或连字符，并以字母或数字开头")
	}

	passwordHash, err := hashOperatorPassword(input.Password)
	if err != nil {
		return OperatorDTO{}, err
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (OperatorDTO, error) {
		record, err := tx.Operator.Create().
			SetUsername(username).
			SetPasswordHash(passwordHash).
			SetAllPages(input.AllPages).
			SetAllEndpoints(input.AllEndpoints).
			SetPages(input.Pages).
			SetEndpoints(input.Endpoints).
			Save(ctx)
		if ent.IsConstraintError(err) {
			return OperatorDTO{}, app.Conflict("用户名已存在")
		}

		if err != nil {
			return OperatorDTO{}, err
		}

		if err := app.Audit(ctx, tx, actorID, enum.AuditOperatorCreate, record.ID, "", input.OperatorGrants.auditSummary()); err != nil {
			return OperatorDTO{}, err
		}

		return operatorDTO(record), nil
	})
}

func (s *Service) UpdateOperator(ctx context.Context, actorID, id string, input UpdateOperatorInput) (OperatorDTO, error) {
	if input.Disabled == nil {
		return OperatorDTO{}, app.Invalid("请指定账号是否停用")
	}

	var passwordHash string
	if input.Password != "" {
		var err error
		passwordHash, err = hashOperatorPassword(input.Password)
		if err != nil {
			return OperatorDTO{}, err
		}
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (OperatorDTO, error) {
		record, err := tx.Operator.Query().
			Where(operator.IDEQ(id)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return OperatorDTO{}, app.WrapMissing(err)
		}

		if record.SuperAdmin {
			return OperatorDTO{}, app.Forbidden("超级管理员的权限和状态不可通过账号管理修改")
		}

		disabled := *input.Disabled
		update := tx.Operator.UpdateOne(record).
			SetAllPages(input.AllPages).
			SetAllEndpoints(input.AllEndpoints).
			SetPages(input.Pages).
			SetEndpoints(input.Endpoints).
			SetDisabled(disabled)
		if passwordHash != "" {
			update.SetPasswordHash(passwordHash)
		}

		record, err = update.Save(ctx)
		if err != nil {
			return OperatorDTO{}, err
		}

		if disabled || passwordHash != "" {
			if err := revokeOperatorSessions(ctx, tx, id); err != nil {
				return OperatorDTO{}, err
			}
		}

		if err := app.Audit(ctx, tx, actorID, enum.AuditOperatorUpdate, id, "",
			input.OperatorGrants.auditSummary(), app.AuditFlag("停用", disabled), app.AuditFlag("重置密码", passwordHash != "")); err != nil {
			return OperatorDTO{}, err
		}

		return operatorDTO(record), nil
	})
}

func normalizeOperatorUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func hashOperatorPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < 12 || len(password) > 72 {
		return "", app.Invalid("密码至少 12 个字符，且最多 72 字节")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func invalidCredentials() error {
	return &app.Error{
		Status:  http.StatusUnauthorized,
		Code:    enum.ErrorUnauthorized,
		Message: "用户名或密码错误",
	}
}

func operatorDTO(record *ent.Operator) OperatorDTO {
	return OperatorDTO{
		ID:           record.ID,
		Username:     record.Username,
		SuperAdmin:   record.SuperAdmin,
		Disabled:     record.Disabled,
		AllPages:     record.AllPages,
		AllEndpoints: record.AllEndpoints,
		Pages:        append([]string{}, record.Pages...),
		Endpoints:    append([]string{}, record.Endpoints...),
	}
}

func (grants OperatorGrants) auditSummary() string {
	return "页面权限：" + auditGrantList(grants.AllPages, grants.Pages) + "；接口权限：" + auditGrantList(grants.AllEndpoints, grants.Endpoints)
}

func auditGrantList(all bool, values []string) string {
	if all {
		return "全部（含以后新增）"
	}

	if len(values) == 0 {
		return "无"
	}

	return strings.Join(values, "、")
}
