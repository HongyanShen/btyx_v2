package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/hook"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/database"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/segmentio/ksuid"
)

type Runtime struct {
	Client      *ent.Client
	DB          *sql.DB
	Config      config.Config
	Now         func() time.Time
	crypt       cipher.AEAD
	NoticeQueue *river.Client[*sql.Tx]
}

func Open(ctx context.Context, c config.Config) (*Runtime, error) {
	db, client, err := database.Open(ctx, c)
	if err != nil {
		return nil, err
	}

	k, _ := hex.DecodeString(c.EncryptionKey)
	block, err := aes.NewCipher(k)
	if err != nil {
		client.Close()
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		client.Close()
		return nil, err
	}

	// Keep insert notifications responsive across independent transactions.
	// The consumer has its own fetch cooldown to coalesce concurrent wakeups.
	queue, err := river.NewClient(riverdatabasesql.New(db), &river.Config{
		Schema:        c.Schema,
		FetchCooldown: river.FetchCooldownMin,
	})
	if err != nil {
		client.Close()
		return nil, err
	}

	runtime := &Runtime{
		NoticeQueue: queue,
		Client:      client,
		DB:          db,
		Config:      c,
		Now:         time.Now,
		crypt:       aead,
	}
	client.AuditEvent.Use(hook.On(runtime.encryptAuditParameters, ent.OpCreate))
	return runtime, nil
}

func NewID() string {
	return ksuid.New().String()
}

func (s *Runtime) Close() error {
	return s.Client.Close()
}

func (s *Runtime) Day() string {
	return BusinessDay(s.Now())
}

var beijingLocation = time.FixedZone("Asia/Shanghai", 8*3600)

func BeijingTime(at time.Time) time.Time {
	return at.In(beijingLocation)
}

func BusinessDay(at time.Time) string {
	return BeijingTime(at).Format("2006-01-02")
}

func (s *Runtime) Encrypt(v string) (string, error) {
	n := make([]byte, s.crypt.NonceSize())
	if _, e := rand.Read(n); e != nil {
		return "", e
	}

	return base64.RawStdEncoding.EncodeToString(s.crypt.Seal(n, n, []byte(v), nil)), nil
}

func (s *Runtime) Decrypt(v string) string {
	b, e := base64.RawStdEncoding.DecodeString(v)
	if e != nil || len(b) < s.crypt.NonceSize() {
		return ""
	}

	n := b[:s.crypt.NonceSize()]
	p, e := s.crypt.Open(nil, n, b[s.crypt.NonceSize():], nil)
	if e != nil {
		return ""
	}

	return string(p)
}

func Hash(v string) string {
	h := sha256.Sum256([]byte(v))
	return hex.EncodeToString(h[:])
}
