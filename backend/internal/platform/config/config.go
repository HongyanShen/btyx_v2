package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"

	"github.com/spf13/viper"
)

type Config struct {
	Host                   string
	Port                   string
	Database               string
	User                   string
	Password               string
	Schema                 string
	BootstrapAdminUsername string
	BootstrapAdminPassword string
	EncryptionKey          string
	MiniAddr               string
	AdminAddr              string
	UploadDir              string
	WechatAppID            string
	WechatAppSecret        string
	IdentityOCRSecretID    string
	IdentityOCRSecretKey   string
	Demo                   bool
	Payment                Payment
	Storage                ObjectStorage
	Logging                Logging
}

var SchemaName = regexp.MustCompile(`^btyx_[a-z0-9_]+$`)

func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("env")
	v.AutomaticEnv()
	v.SetDefault("TOS_OBJECT_PREFIX", "btyx-v2/dev/")
	v.SetDefault("LOG_DIR", "artifacts/logs")
	v.SetDefault("LOG_MAX_SIZE_MB", 20)
	v.SetDefault("LOG_MAX_BACKUPS", 7)
	v.SetDefault("LOG_MAX_AGE_DAYS", 14)
	v.SetDefault("LOG_COMPRESS", true)
	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("读取本地配置失败: %w", err)
	}

	c := Config{
		Host:                   v.GetString("PGHOST"),
		Port:                   v.GetString("PGPORT"),
		Database:               v.GetString("PGDATABASE"),
		User:                   v.GetString("PGUSER"),
		Password:               v.GetString("PGPASSWORD"),
		Schema:                 v.GetString("PGSCHEMA"),
		BootstrapAdminUsername: v.GetString("BOOTSTRAP_ADMIN_USERNAME"),
		BootstrapAdminPassword: v.GetString("BOOTSTRAP_ADMIN_PASSWORD"),
		EncryptionKey:          v.GetString("ENCRYPTION_KEY"),
		Demo:                   v.GetBool("DEMO_ENABLED"),
		MiniAddr:               v.GetString("MINI_ADDR"),
		AdminAddr:              v.GetString("ADMIN_ADDR"),
		UploadDir:              v.GetString("UPLOAD_DIR"),
		WechatAppID:            v.GetString("WECHAT_APP_ID"),
		WechatAppSecret:        v.GetString("WECHAT_APP_SECRET"),
		IdentityOCRSecretID:    v.GetString("TENCENT_OCR_SECRET_ID"),
		IdentityOCRSecretKey:   v.GetString("TENCENT_OCR_SECRET_KEY"),
	}
	if !SchemaName.MatchString(c.Schema) {
		return c, fmt.Errorf("数据库 schema 必须使用 btyx_ 前缀及安全字符")
	}

	if (c.IdentityOCRSecretID == "") != (c.IdentityOCRSecretKey == "") {
		return c, fmt.Errorf("TENCENT_OCR_SECRET_ID 和 TENCENT_OCR_SECRET_KEY 需同时配置")
	}

	k, e := hex.DecodeString(c.EncryptionKey)
	if e != nil || len(k) != 32 {
		return c, fmt.Errorf("ENCRYPTION_KEY 必须是32字节十六进制密钥")
	}

	c.Payment = Payment{
		Enabled:        v.GetBool("WECHAT_PAY_ENABLED"),
		MerchantID:     v.GetString("WECHAT_PAY_MERCHANT_ID"),
		CertificateID:  v.GetString("WECHAT_PAY_CERTIFICATE_ID"),
		PrivateKeyPath: v.GetString("WECHAT_PAY_PRIVATE_KEY_PATH"),
		PublicKeyID:    v.GetString("WECHAT_PAY_PUBLIC_KEY_ID"),
		PublicKeyPath:  v.GetString("WECHAT_PAY_PUBLIC_KEY_PATH"),
		APIv3Key:       v.GetString("WECHAT_PAY_API_V3_KEY"),
		NotifyURL:      v.GetString("WECHAT_PAY_NOTIFY_URL"),
		SceneID:        v.GetString("WECHAT_PAY_SCENE_ID"),
	}
	if reports := v.GetString("WECHAT_PAY_SCENE_REPORTS"); reports != "" {
		if err := json.Unmarshal([]byte(reports), &c.Payment.SceneReports); err != nil {
			return c, fmt.Errorf("WECHAT_PAY_SCENE_REPORTS 必须是 JSON 数组")
		}
	}

	if err := c.Payment.Validate(c.WechatAppID); err != nil {
		return c, err
	}

	c.Storage = ObjectStorage{
		Bucket:    v.GetString("TOS_BUCKET"),
		Region:    v.GetString("TOS_REGION"),
		Endpoint:  v.GetString("TOS_ENDPOINT"),
		AccessKey: v.GetString("TOS_ACCESS_KEY"),
		SecretKey: v.GetString("TOS_SECRET_KEY"),
		Prefix:    v.GetString("TOS_OBJECT_PREFIX"),
	}
	if err := c.Storage.Validate(); err != nil {
		return c, err
	}

	c.Logging = Logging{
		Directory:  v.GetString("LOG_DIR"),
		MaxSizeMB:  v.GetInt("LOG_MAX_SIZE_MB"),
		MaxBackups: v.GetInt("LOG_MAX_BACKUPS"),
		MaxAgeDays: v.GetInt("LOG_MAX_AGE_DAYS"),
		Compress:   v.GetBool("LOG_COMPRESS"),
	}
	if err := c.Logging.Validate(); err != nil {
		return c, err
	}

	if !filepath.IsAbs(c.Logging.Directory) {
		c.Logging.Directory = filepath.Join(filepath.Dir(path), c.Logging.Directory)
	}

	logDirectory, err := filepath.Abs(c.Logging.Directory)
	if err != nil {
		return c, fmt.Errorf("无法解析日志目录: %w", err)
	}

	c.Logging.Directory = logDirectory

	return c, nil
}

func (c Config) DSN(schema bool) string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   c.Host + ":" + c.Port,
		Path:   "/" + c.Database,
	}
	q := url.Values{
		"sslmode":         []string{"disable"},
		"connect_timeout": []string{"5"},
	}
	if schema {
		q.Set("search_path", c.Schema)
	}

	u.RawQuery = q.Encode()
	return u.String()
}
