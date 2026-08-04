// Package config handles the loading and parsing of application
// settings from configuration files and environment variables.
package config

import (
	"errors"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Port         string   `mapstructure:"port" json:"port"`
	WeaviateHost string   `mapstructure:"weaviate_host" json:"weaviate_host"`
	WeaviatePort string   `mapstructure:"weaviate_port" json:"weaviate_port"`
	Folders      []string `mapstructure:"default_folders" json:"default_folders"`
	DBPath       string   `mapstructure:"db_path" json:"db_path"`

	ImagesPerPage    int    `mapstructure:"images_per_page" json:"images_per_page"`
	DefaultSortBy    string `mapstructure:"default_sort_by" json:"default_sort_by"`
	DefaultSortOrder string `mapstructure:"default_sort_order" json:"default_sort_order"`
	GridSize         string `mapstructure:"grid_size" json:"grid_size"`
	InfiniteScroll   bool   `mapstructure:"infinite_scroll" json:"infinite_scroll"`
	Debug            bool   `mapstructure:"debug" json:"debug"`
}

func Load() (*Config, error) {
	viper.SetDefault("port", "3000")
	viper.SetDefault("weaviate_host", "localhost")
	viper.SetDefault("weaviate_port", ":50050")
	viper.SetDefault("db_path", "data/acuity.db")
	viper.SetDefault("images_per_page", 100)
	viper.SetDefault("default_sort_by", "name")
	viper.SetDefault("default_sort_order", "desc")
	viper.SetDefault("grid_size", "medium")
	viper.SetDefault("infinite_scroll", false)
	viper.SetDefault("debug", false)


	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "data"
	}
	// Ensure the config directory exists
	_ = os.MkdirAll(configDir, 0755)

	viper.SetConfigName("config")
	viper.AddConfigPath(configDir)
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	viper.SetConfigType("json")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok {
			// Datei existiert nicht, wir legen eine neue an
			configFile := configDir + "/config.json"
			if writeErr := viper.SafeWriteConfigAs(configFile); writeErr != nil {
				return nil, writeErr
			}
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func Save(cfg *Config) error {
	viper.Set("images_per_page", cfg.ImagesPerPage)
	viper.Set("default_sort_by", cfg.DefaultSortBy)
	viper.Set("default_sort_order", cfg.DefaultSortOrder)
	viper.Set("infinite_scroll", cfg.InfiniteScroll)
	viper.Set("grid_size", cfg.GridSize)

	return viper.WriteConfig()
}
