package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config contains the configuration for the application
type Config struct {
	RecordingTypes   []string      `mapstructure:"recording_types"`
	IgnoreTitles     []string      `mapstructure:"ignore_titles"`
	DeleteAfter      bool          `mapstructure:"delete_after"`
	Duration         time.Duration `mapstructure:"duration"`
	Token            string        `mapstructure:"token"`
	ApiEndpoint      *url.URL      `mapstructure:"api_endpoint"`
	AuthEndpoint     *url.URL      `mapstructure:"auth_endpoint"`
	UserID           string        `mapstructure:"user_id"`
	ClientID         string        `mapstructure:"client_id"`
	ClientSecret     string        `mapstructure:"client_secret"`
	Concurrency      int           `mapstructure:"concurrency"`
	ChunckSizeMB     int           `mapstructure:"chunck_size_mb"`
	StartingFromYear int           `mapstructure:"starting_from_year"`
	Storage          struct {
		Directory string     `mapstructure:"directory"`
		SFTP      SFTPConfig `mapstructure:"sftp"`
	} `mapstructure:"storage"`
}

type SFTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Key      string `mapstructure:"key"`
}

// New returns a new initialized configuration based on flags
// and environment variables
func New() *Config {
	cfg := &Config{}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/zoomdl/")
	viper.AddConfigPath("$HOME/.config/zoomdl/")
	viper.AddConfigPath(".")

	viper.SetDefault("delete_after", false)
	viper.SetDefault("duration", "30m")

	viper.SetEnvPrefix("zoomdl")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("error getting configuration %v\n", err)
		os.Exit(1)
	}

	if err := viper.Unmarshal(cfg); err != nil {
		fmt.Printf("error getting configuration %v\n", err)
		os.Exit(1)
	}

	return cfg
}
