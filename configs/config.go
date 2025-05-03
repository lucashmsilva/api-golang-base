package configs

import (
	"github.com/spf13/viper"
)

type DB struct {
	Host     string `mapstructure:"HOST"`
	Port     string `mapstructure:"PORT"`
	Username string `mapstructure:"USERNAME"`
	Password string `mapstructure:"PASSWORD"`
	Database string `mapstructure:"DATABASE"`
}

type Config struct {
	Port string `mapstructure:"PORT"`
	DB   DB     `mapstructure:"DB"`
}

func LoadConfig() (*Config, error) {
	var config *Config

	viper.SetConfigName(".config")
	viper.SetConfigType("json")
	viper.AddConfigPath("./configs/")

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
