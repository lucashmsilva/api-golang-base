package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type ParamPaths map[string]string
type LoadedParams map[string]string

type DbConnConfig struct {
	Database string `json:"database"`
	Host     struct {
		Read  []string `json:"read"`
		Write string   `json:"write"`
	} `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Db map[string]DbConnConfig

type Config struct {
	AppName  string
	Env      string
	Timezone string
	Port     int    `json:"port"`
	LogLevel string `json:"log_level"`
	Db       Db
	// define the rest of the config as needed
}

func LoadConfig() (*Config, error) {
	var loadedParams LoadedParams
	var config Config
	var db Db
	var err error

	env := os.Getenv("GO_ENV")
	appName := os.Getenv("APP_NAME")
	paramPaths := buildParamPaths(env, appName)

	switch env {
	case "testing":
	case "production":
		loadedParams, err = loadSSMParams(paramPaths)
		if err != nil {
			return nil, err
		}

	case "development":
		loadedParams, err = loadLocalParams(paramPaths)
		if err != nil {
			return nil, err
		}

	default:
		return nil, errors.New("env must be one of [development, testing, production]")
	}

	configJson := loadedParams[paramPaths["CONFIG_PATH"]]
	databasesJson := loadedParams[paramPaths["DATABASES_PATH"]]
	timezone := loadedParams[paramPaths["TIMEZONE_PATH"]]

	err = json.Unmarshal([]byte(configJson), &config)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(databasesJson), &db)
	if err != nil {
		return nil, err
	}

	config.AppName = appName
	config.Env = env
	config.Timezone = timezone
	config.Db = db

	return &config, nil
}

func buildParamPaths(env, appName string) ParamPaths {
	return ParamPaths{
		"CONFIG_PATH":    fmt.Sprintf("/%v/%v/config", env, appName),
		"DATABASES_PATH": fmt.Sprintf("/%v/%v/databases", env, appName),
		"TIMEZONE_PATH":  fmt.Sprintf("/%v/common/timezone", env),
	}
}

func loadSSMParams(paramPaths ParamPaths) (loadedParams LoadedParams, err error) {
	loadedParams = make(map[string]string, len(paramPaths))
	paramNames := make([]string, 0, len(paramPaths))

	for _, v := range paramPaths {
		paramNames = append(paramNames, v)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	ssmClient := ssm.NewFromConfig(cfg)
	ssmOutput, err := ssmClient.GetParameters(context.TODO(), &ssm.GetParametersInput{
		Names:          paramNames,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}

	for _, param := range ssmOutput.Parameters {
		loadedParams[*param.Name] = *param.Value
	}

	return loadedParams, nil
}

func loadLocalParams(paramPaths ParamPaths) (loadedParams LoadedParams, err error) {
	configJson, err := os.ReadFile("./internal/config/.config.json")
	if err != nil {
		return nil, err
	}

	databasesJson, err := os.ReadFile("./internal/config/.databases.json")
	if err != nil {
		return nil, err
	}

	return LoadedParams{
		paramPaths["CONFIG_PATH"]:    string(configJson),
		paramPaths["DATABASES_PATH"]: string(databasesJson),
		paramPaths["TIMEZONE_PATH"]:  os.Getenv("TIMEZONE"),
	}, nil
}
