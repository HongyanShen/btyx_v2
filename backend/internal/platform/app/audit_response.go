package app

import (
	"context"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/auditevent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
)

type AuditResponse struct {
	Status      int
	ContentType string
	IsFile      bool
	Body        string
	FailedActor string
	ReferenceID string
	Reason      string
}

// The response is known only after the business transaction and HTTP handler
// finish. Link it by request ID without modifying committed audit events.
func (s *Runtime) SaveAuditResponse(ctx context.Context, response AuditResponse) error {
	requestID := logging.RequestID(ctx)
	_, err := Transaction(ctx, s, func(tx *ent.Tx) (bool, error) {
		hasEvent := false
		if AuditEventAttempted(ctx) {
			var err error
			hasEvent, err = tx.AuditEvent.Query().Where(auditevent.RequestIDEQ(requestID)).Exist(ctx)
			if err != nil {
				return false, err
			}
		}

		if !hasEvent {
			if response.FailedActor == "" {
				return false, nil
			}

			if err := Audit(ctx, tx, response.FailedActor, enum.AuditRequestFailed, response.ReferenceID, response.Reason); err != nil {
				return false, err
			}
		}

		create := tx.AuditHTTPResponse.Create().
			SetID(requestID).
			SetStatus(response.Status).
			SetContentType(response.ContentType).
			SetIsFile(response.IsFile)
		if !response.IsFile {
			ciphertext, err := s.Encrypt(response.Body)
			if err != nil {
				return false, err
			}

			create.SetBodyCipher("enc:v1:" + ciphertext)
		}

		return true, create.Exec(ctx)
	})
	return err
}
