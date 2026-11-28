package app

import (
	"context"
	"encoding/json"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/hook"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
)

type auditParametersKey struct{}

func WithAuditParameters(ctx context.Context, data string) context.Context {
	return context.WithValue(ctx, auditParametersKey{}, data)
}

// Encrypt at insertion so identity checks and ordinary business audits follow the same rule.
func (s *Runtime) encryptAuditParameters(next ent.Mutator) ent.Mutator {
	return hook.AuditEventFunc(func(ctx context.Context, mutation *ent.AuditEventMutation) (ent.Value, error) {
		if origin, ok := ctx.Value(auditOriginKey{}).(*auditOrigin); ok {
			origin.eventAttempted.Store(true)
			mutation.SetRequestID(logging.RequestID(ctx))
			mutation.SetEndpoint(origin.endpoint)
			mutation.SetPlatform(origin.platform)
		}

		payload, ok := ctx.Value(auditParametersKey{}).(string)
		if !ok {
			summary, exists := mutation.Parameters()
			if !exists || summary == "" {
				return next.Mutate(ctx, mutation)
			}

			// Background operations have no HTTP request. Retain their supplied details.
			data, err := json.Marshal(struct {
				Operation string `json:"operation"`
			}{Operation: summary})
			if err != nil {
				return nil, err
			}

			payload = string(data)
		}

		ciphertext, err := s.Encrypt(payload)
		if err != nil {
			return nil, err
		}

		mutation.SetParameters("enc:v1:" + ciphertext)
		return next.Mutate(ctx, mutation)
	})
}
