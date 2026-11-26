package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	App      App      `mapstructure:"app"`
	MySQL    MySQL    `mapstructure:"mysql"`
	Redis    Redis    `mapstructure:"redis"`
	MinIO    MinIO    `mapstructure:"minio"`
	RocketMQ RocketMQ `mapstructure:"rocketmq"`
	JWT      JWT      `mapstructure:"jwt"`
	FFmpeg   FFmpeg   `mapstructure:"ffmpeg"`
}

type App struct {
	NodeID          int64         `mapstructure:"node_id" validate:"gte=0,lte=1023"`
	TrustedProxies  []string      `mapstructure:"trusted_proxies" validate:"dive,ip|cidr"`
	Mode            string        `mapstructure:"mode" validate:"oneof=debug release test"`
	HTTPAddr        string        `mapstructure:"http_addr" validate:"listenaddr"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" validate:"gt=0"`
}

type MySQL struct {
	Addr           string        `mapstructure:"addr" validate:"endpoint"`
	User           string        `mapstructure:"user" validate:"required"`
	Password       string        `mapstructure:"password"`
	Database       string        `mapstructure:"database" validate:"required"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout" validate:"gt=0"`
}

type Redis struct {
	Addr     string `mapstructure:"addr" validate:"endpoint"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db" validate:"gte=0"`
}

type MinIO struct {
	Endpoint  string `mapstructure:"endpoint" validate:"endpoint"`
	AccessKey string `mapstructure:"access_key" validate:"required"`
	SecretKey string `mapstructure:"secret_key" validate:"required"`
	Bucket    string `mapstructure:"bucket" validate:"required"`
	Secure    bool   `mapstructure:"secure"`
}

type RocketMQ struct {
	NameServers []string `mapstructure:"name_servers" validate:"min=1,dive,endpoint"`
}

type JWT struct {
	Issuer     string        `mapstructure:"issuer" validate:"required"`
	Secret     string        `mapstructure:"secret" validate:"min=32"`
	AccessTTL  time.Duration `mapstructure:"access_ttl" validate:"gte=1000000000"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl" validate:"gtfield=AccessTTL"`
}

type FFmpeg struct {
	Binary      string `mapstructure:"binary" validate:"required"`
	ProbeBinary string `mapstructure:"probe_binary" validate:"required"`
	WorkDir     string `mapstructure:"work_dir" validate:"required"`
}

func Load(path string) (Config, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetConfigType("yaml")
	v.SetConfigFile(path)
	v.SetEnvPrefix("BILIBILI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AllowEmptyEnv(true)
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("读取配置文件 %q: %w", path, err)
	}

	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return Config{}, fmt.Errorf("解析配置文件 %q: %w", path, err)
	}
	if err := validate(cfg); err != nil {
		return Config{}, fmt.Errorf("校验配置文件 %q: %w", path, err)
	}
	return cfg, nil
}

func validate(cfg Config) error {
	check := validator.New()
	if err := check.RegisterValidation("listenaddr", func(field validator.FieldLevel) bool {
		return validAddress(field.Field().String(), true)
	}); err != nil {
		return err
	}
	if err := check.RegisterValidation("endpoint", func(field validator.FieldLevel) bool {
		return validAddress(field.Field().String(), false)
	}); err != nil {
		return err
	}
	return check.Struct(cfg)
}

func validAddress(address string, listen bool) bool {
	host, port, err := net.SplitHostPort(address)
	if err != nil || (!listen && host == "") {
		return false
	}
	number, err := strconv.ParseUint(port, 10, 16)
	return err == nil && (listen || number != 0)
}
