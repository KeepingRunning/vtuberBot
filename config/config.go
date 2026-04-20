package config

import (
	viper "github.com/spf13/viper"
	"github.com/KeepingRunning/vtuberBot/errors"
)
type Config struct {
	cookie string `mapstructure:"cookie"`
}

// 从config path导入配置文件
func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("toml")
	if err := viper.ReadInConfig(); err != nil {
		return nil, errors.ErrConfigNotFound
	}

	config := &Config{}
	viper.Unmarshal(config)
	
	return config, nil
}