// Package config handles the loading and parsing of application
// settings from configuration files and environment variables.
package config

import "github.com/spf13/viper"

type Config struct {
	Port         string   `mapstructure:"port"`
	WeaviateHost string   `mapstructure:"weaviate_host"`
	WeaviatePort string   `mapstructure:"weaviate_port"`
	Folders      []string `mapstructure:"default_folders"`
	DBPath       string   `mapstructure:"db_path"`

	ImagesPerPage    int    `mapstructure:"images_per_page"`
	DefaultSortBy    string `mapstructure:"default_sort_by"`
	DefaultSortOrder string `mapstructure:"default_sort_order"`
}

func Load() (*Config, error) {
	viper.SetDefault("port", "3000")
	viper.SetDefault("weaviate_host", "localhost")
	viper.SetDefault("weaviate_port", ":50050")
	viper.SetDefault("db_path", "./acuity.db")
	viper.SetDefault("images_per_page", 500)
	viper.SetDefault("default_sort_by", "name")
	viper.SetDefault("default_sort_order", "desc")

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	viper.SetConfigType("json")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Datei existiert nicht, wir legen eine neue an
			if writeErr := viper.SafeWriteConfigAs("config.json"); writeErr != nil {
				return nil, writeErr
			}
		} else {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func save(cfg *Config) error {
	viper.Set("images_per_page", cfg.ImagesPerPage)
	viper.Set("default_sort_by", cfg.DefaultSortBy)
	viper.Set("default_sort_order", cfg.DefaultSortOrder)

	return viper.WriteConfig()
}
