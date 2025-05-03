package configs

import (
	"github.com/spf13/viper"
)

type DB struct {
	Host     string `mapstructure:"DB_HOST"`
	Port     string `mapstructure:"DB_PORT"`
	Username string `mapstructure:"DB_USERNAME"`
	Password string `mapstructure:"DB_PASSWORD"`
	Name     string `mapstructure:"DB_Name"`
}

type Config struct {
	Port string `mapstructure:"PORT"`
	DB   DB     `mapstructure:"DB"`
}

func LoadConfig() (*Config, error) {
	var config *Config

	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath(".config")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	err = viper.Unmarshal(&config)

	if err != nil {
		return nil, err
	}
 
	/*
		ler config do ssm
		configurar logger
	*/

	return config, nil
}
