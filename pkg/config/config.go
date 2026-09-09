// Package config handles the loading and parsing of application
// settings from configuration files and environment variables.
package config

import (
	"errors"
	"os"
	"path/filepath"

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
	ThumbCacheMaxMB  int    `mapstructure:"thumb_cache_max_mb" json:"thumb_cache_max_mb"`
	Debug            bool   `mapstructure:"debug" json:"debug"`
}

func GetAppDir() string {
	if custom := os.Getenv("CONFIG_DIR"); custom != "" {
		return custom
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "data"
	}
	appDir := filepath.Join(configDir, "acuity")
	_ = os.MkdirAll(appDir, 0755)
	return appDir
}
func Load() (*Config, error) {
	appDir := GetAppDir()
	defaultDBPath := filepath.Join(appDir, "acuity.db")

	viper.SetDefault("port", "3000")
	viper.SetDefault("weaviate_host", "localhost")
	viper.SetDefault("weaviate_port", ":50050")
	viper.SetDefault("db_path", defaultDBPath)
	viper.SetDefault("images_per_page", 100)
	viper.SetDefault("default_sort_by", "name")
	viper.SetDefault("default_sort_order", "desc")
	viper.SetDefault("grid_size", "medium")
	viper.SetDefault("infinite_scroll", false)
	viper.SetDefault("thumb_cache_max_mb", 2048)
	viper.SetDefault("debug", false)



	viper.SetConfigName("config")
	viper.AddConfigPath(appDir)
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	viper.SetConfigType("json")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok {
			// Datei existiert nicht, wir legen eine neue an
			configFile := appDir + "/config.json"
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
	viper.Set("thumb_cache_max_mb", cfg.ThumbCacheMaxMB)

	return viper.WriteConfig()
}
