package identity

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/fileobject"
	"baituber.cn/yanxuan/v2/ent/predicate"
	"baituber.cn/yanxuan/v2/ent/userverification"
	"baituber.cn/yanxuan/v2/internal/domain/enrollmentstate"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type DocumentInput struct {
	Name        string `json:"name"`
	Identity    string `json:"identity"`
	FrontFileID string `json:"front_file_id"`
	BackFileID  string `json:"back_file_id"`
}

type Document struct {
	NameCipher     string
	IdentityCipher string
	FrontFileID    string
	BackFileID     string
}

type DocumentVerifier interface {
	Check(ctx context.Context, name, number string) error
}

var identityNumberPattern = regexp.MustCompile(`^[0-9]{17}[0-9X]$`)

func MaskIdentityName(name string) string {
	runes := []rune(name)
	switch len(runes) {
	case 0:
		return ""
	case 1:
		return "*"
	default:
		return string(runes[0]) + strings.Repeat("*", len(runes)-1)
	}
}

func validIdentityName(name string) bool {
	return name != "" && utf8.RuneCountInString(name) <= 30 && !strings.ContainsFunc(name, unicode.IsControl)
}

func MaskIdentityNumber(number string) string {
	if !identityNumberPattern.MatchString(number) {
		return ""
	}

	return number[:4] + "**********" + number[14:]
}

func IsVerified(member *ent.User) bool {
	verification := member.Edges.Verification
	if verification != nil && verification.VerifiedAt != nil {
		return true
	}

	// Existing approved enrollments predate the verification timestamp.
	enrollment := enrollmentstate.Current(member)
	return enrollment != nil && enrollment.Status == enum.EnrollmentApproved
}

func PrepareDocument(ctx context.Context, client *ent.Client, runtime *app.Runtime, userID string, input DocumentInput) (Document, error) {
	name := strings.TrimSpace(input.Name)
	if !validIdentityName(name) {
		return Document{}, app.Invalid("请填写与身份证一致的真实姓名")
	}

	number := strings.ToUpper(strings.TrimSpace(input.Identity))
	if err := validateIdentityAge(number, runtime.Day()); err != nil {
		return Document{}, err
	}

	if input.FrontFileID == "" || input.BackFileID == "" || input.FrontFileID == input.BackFileID {
		return Document{}, app.Invalid("请分别上传身份证人像面和国徽面")
	}

	count, err := client.FileObject.Query().
		Where(
			fileobject.IDIn(input.FrontFileID, input.BackFileID),
			ownedIdentityPhoto(userID),
		).
		Count(ctx)
	if err != nil {
		return Document{}, err
	}

	if count != 2 {
		return Document{}, app.Invalid("请上传本人已确认的身份证照片")
	}

	nameCipher, err := runtime.Encrypt(name)
	if err != nil {
		return Document{}, err
	}

	identityCipher, err := runtime.Encrypt(number)
	if err != nil {
		return Document{}, err
	}

	return Document{
		NameCipher:     nameCipher,
		IdentityCipher: identityCipher,
		FrontFileID:    input.FrontFileID,
		BackFileID:     input.BackFileID,
	}, nil
}

func (document Document) Check(ctx context.Context, runtime *app.Runtime, verifier DocumentVerifier) error {
	name := strings.TrimSpace(runtime.Decrypt(document.NameCipher))
	number := strings.ToUpper(strings.TrimSpace(runtime.Decrypt(document.IdentityCipher)))
	if name == "" || number == "" {
		return app.Invalid("实名资料不完整，请重新提交")
	}

	if err := validateIdentityAge(number, runtime.Day()); err != nil {
		return err
	}

	if verifier == nil {
		return &app.Error{
			Status:  http.StatusServiceUnavailable,
			Code:    enum.ErrorUnavailable,
			Message: "实名信息核验尚未配置",
		}
	}

	return verifier.Check(ctx, name, number)
}

func ownedIdentityPhoto(userID string) predicate.FileObject {
	return fileobject.And(fileobject.OwnerIDEQ(userID), ReadyIdentityPhoto())
}

func ReadyIdentityPhoto() predicate.FileObject {
	return fileobject.And(
		fileobject.StatusEQ(enum.FileReady),
		fileobject.StorageEQ(enum.FileStorageTOS),
		fileobject.VerifiedAtNotNil(),
		fileobject.BucketNEQ(""),
		fileobject.ObjectKeyNEQ(""),
		fileobject.MediaTypeIn(enum.FileJPEG, enum.FilePNG),
	)
}

func SaveDocument(ctx context.Context, tx *ent.Tx, userID string, document Document, now time.Time) error {
	return tx.UserVerification.Create().
		SetUserID(userID).
		SetNameCipher(document.NameCipher).
		SetIdentityCipher(document.IdentityCipher).
		SetFrontFileID(document.FrontFileID).
		SetBackFileID(document.BackFileID).
		SetVerifiedAt(now).
		OnConflictColumns(userverification.FieldUserID).
		UpdateNameCipher().
		UpdateIdentityCipher().
		UpdateFrontFileID().
		UpdateBackFileID().
		UpdateVerifiedAt().
		SetUpdatedAt(now).
		Exec(ctx)
}

func validateIdentityAge(number, today string) error {
	formatError := app.Invalid("请输入有效的 18 位身份证号")
	if !identityNumberPattern.MatchString(number) {
		return formatError
	}

	weights := [...]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	sum := 0
	for index, weight := range weights {
		sum += int(number[index]-'0') * weight
	}

	if number[17] != "10X98765432"[sum%11] {
		return formatError
	}

	birth, err := time.Parse("20060102", number[6:14])
	if err != nil || birth.Year() < 1800 || birth.Format(time.DateOnly) > today {
		return formatError
	}

	if birth.AddDate(18, 0, 0).Format(time.DateOnly) > today {
		return app.Invalid("实名认证须年满 18 周岁")
	}

	return nil
}
