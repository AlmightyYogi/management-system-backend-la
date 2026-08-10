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
}

type EmailConfig struct {
	SMTPHost   string `mapstructure:"smtp_host"`
	SMTPPort   int    `mapstructure:"smtp_port"`
	SMTPUser   string `mapstructure:"smtp_user"`
	SMTPPass   string `mapstructure:"smtp_pass"`
	FromEmail  string `mapstructure:"from_email"`
	AdminEmail string `mapstructure:"admin_email"`
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
	}
}

func GetConfig() *Config {
	return AppConfigInstance
}