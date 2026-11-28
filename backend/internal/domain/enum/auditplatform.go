package enum

type AuditPlatform string

const (
	AuditPlatformAdmin   AuditPlatform = "admin"
	AuditPlatformMiniapp AuditPlatform = "miniapp"
)

func (AuditPlatform) Values() []string {
	return []string{string(AuditPlatformAdmin), string(AuditPlatformMiniapp)}
}

func (value AuditPlatform) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *AuditPlatform) UnmarshalJSON(data []byte) error {
	return decode(data, value, (AuditPlatform("")).Values())
}
