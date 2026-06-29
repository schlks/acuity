// Package config handles the loading and parsing of application
// settings from configuration files and environment variables.
package config

import "github.com/spf13/viper"

type Config struct {
	Port 			string 		`mapstructure:"port"`
	WeaviateHost 	string 		`mapstructure:"weaviate_host"`
	Folders 		[]string 	`mapstructure:"default_folders"`
	DBPath			string 		`mapstructure:"db_path"`
}

func Load() (*Config, error) {
	viper.SetDefault("port", "3000")
	viper.SetDefault("weaviate_host", "localhost:50050")
	viper.SetDefault("db_path", "./acuity.db")

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
