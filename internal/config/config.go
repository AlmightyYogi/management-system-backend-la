package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App			AppConfig		`mapstructure:"app"`
	Database	DatabaseConfig	`mapstructure:"db"`
	JWT			JWTConfig		`mapstructure:"jwt"`
	Email		EmailConfig		`mapstructure:"email"`
	VSS      	VSSConfig      	`mapstructure:"vss"`
}

type EmailConfig struct {
	SMTPHost   string `mapstructure:"smtp_host"`
	SMTPPort   int    `mapstructure:"smtp_port"`
	SMTPUser   string `mapstructure:"smtp_user"`
	SMTPPass   string `mapstructure:"smtp_pass"`
	FromEmail  string `mapstructure:"from_email"`
	AdminEmail string `mapstructure:"admin_email"`
}

type VSSConfig struct {
	Enabled           bool
	LoginURL          string
	WSURL             string
	Username          string
	PasswordMD5       string
	DelayThresholdSec int64
	StaleThresholdSec int64
	LogDir            string
	LogReasons		  string
	LogRetentionDays  int

	EmailEnabled      bool
	EmailReasons      string
	EmailCooldownMin  int
	AlertEmails       string
	EmailMode		  string
	EmailDigestMin	  int
	EmailMinDevices	  int
}

type AppConfig struct {
	Env		string
	Port	int
	Name	string
}

type DatabaseConfig struct {
	Host			string
	Port			int
	User			string
	Password		string
	DBName			string
	SSLMode			string
	MaxIdleConns	int
	MaxOpenConns	int
	ConnMaxLifetime	time.Duration
}

type JWTConfig struct {
    Secret             string        `mapstructure:"JWT_SECRET"`
    ExpiresIn          time.Duration `mapstructure:"JWT_EXPIRES_IN"`
    RefreshTokenExpires time.Duration `mapstructure:"REFRESH_TOKEN_EXPIRES_IN"`
}

var AppConfigInstance *Config

func LoadConfig() {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Println("Warning: No, .env file found, using system environment variables")
	}

	delaySec := viper.GetInt64("VSS_DELAY_THRESHOLD_SEC")
	if delaySec == 0 {
		delaySec = 60
	}
	staleSec := viper.GetInt64("VSS_STALE_THRESHOLD_SEC")
	if staleSec == 0 {
		staleSec = 90
	}
	logDir := viper.GetString("VSS_LOG_DIR")
	if logDir == "" {
		logDir = "storage/public/logs/vss"
	}

	enabled := true
	if viper.IsSet("VSS_ENABLED") {
		enabled = viper.GetBool("VSS_ENABLED")
	}

	AppConfigInstance = &Config{
		App: AppConfig{
			Env:	viper.GetString("APP_ENV"),
			Port:	viper.GetInt("APP_PORT"),
			Name:	viper.GetString("APP_NAME"),
		},
		Database: DatabaseConfig{
			Host:            viper.GetString("DB_HOST"),
			Port:            viper.GetInt("DB_PORT"),
			User:            viper.GetString("DB_USER"),
			Password:        viper.GetString("DB_PASSWORD"),
			DBName:          viper.GetString("DB_NAME"),
			SSLMode:         viper.GetString("DB_SSLMODE"),
			MaxIdleConns:    viper.GetInt("DB_MAX_IDLE_CONNS"),
			MaxOpenConns:    viper.GetInt("DB_MAX_OPEN_CONNS"),
			ConnMaxLifetime: viper.GetDuration("DB_CONN_MAX_LIFETIME"),
		},
		JWT: JWTConfig{
			Secret:					viper.GetString("JWT_SECRET"),
			ExpiresIn: 				viper.GetDuration("JWT_EXPIRES_IN"),
			RefreshTokenExpires: 	viper.GetDuration("REFRESH_TOKEN_EXPIRES_IN"),
		},
		Email: EmailConfig{
			SMTPHost:  viper.GetString("SMTP_HOST"),
			SMTPPort:  viper.GetInt("SMTP_PORT"),
			SMTPUser:  viper.GetString("SMTP_USER"),
			SMTPPass:  viper.GetString("SMTP_PASS"),
			FromEmail: viper.GetString("FROM_EMAIL"),
			AdminEmail:viper.GetString("ADMIN_EMAIL"),
		},
		VSS: VSSConfig{
			Enabled:           enabled,
			LoginURL:          	viper.GetString("VSS_LOGIN_URL"),
			WSURL:             	viper.GetString("VSS_WS_URL"),
			Username:          	viper.GetString("VSS_USERNAME"),
			PasswordMD5:       	viper.GetString("VSS_PASSWORD_MD5"),
			DelayThresholdSec: 	delaySec,
			StaleThresholdSec: 	staleSec,
			LogDir:            	logDir,
			LogReasons: 	   	viper.GetString("VSS_LOG_REASONS"),
			LogRetentionDays:  	viper.GetInt("VSS_LOG_RETENTION_DAYS"),
			EmailEnabled:    	viper.GetBool("VSS_EMAIL_ENABLED"),
			EmailReasons:    	viper.GetString("VSS_EMAIL_REASONS"),
			EmailCooldownMin: 	viper.GetInt("VSS_EMAIL_COOLDOWN_MIN"),
			AlertEmails:     	viper.GetString("VSS_ALERT_EMAILS"),
			EmailMode:       	viper.GetString("VSS_EMAIL_MODE"),
			EmailDigestMin:  	viper.GetInt("VSS_EMAIL_DIGEST_MIN"),
			EmailMinDevices: 	viper.GetInt("VSS_EMAIL_MIN_DEVICES"),
		},
	}

	if AppConfigInstance.VSS.LoginURL == "" {
		AppConfigInstance.VSS.LoginURL = "https://nextfleet-vision.ioh.co.id/vss/user/apiLogin.action"
	}
	if AppConfigInstance.VSS.WSURL == "" {
		AppConfigInstance.VSS.WSURL = "wss://nextfleet-vision.ioh.co.id:36301/wss"
	}
	if AppConfigInstance.VSS.LogReasons == "" {
		AppConfigInstance.VSS.LogReasons = "delayed,stale_timestamp,no_heartbeat,disconnect"
	}
	if AppConfigInstance.VSS.LogRetentionDays <= 0 {
		AppConfigInstance.VSS.LogRetentionDays = 30
	}
	if AppConfigInstance.VSS.EmailCooldownMin <= 0 {
		AppConfigInstance.VSS.EmailCooldownMin = 30
	}
	if AppConfigInstance.VSS.EmailDigestMin <= 0 {
		AppConfigInstance.VSS.EmailDigestMin = 10
	}
	if AppConfigInstance.VSS.EmailMinDevices <= 0 {
		AppConfigInstance.VSS.EmailMinDevices = 1
	}
	if AppConfigInstance.VSS.EmailMode == "" {
		AppConfigInstance.VSS.EmailMode = "digest"
	}
}

func GetConfig() *Config {
	return AppConfigInstance
}