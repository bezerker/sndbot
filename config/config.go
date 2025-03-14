package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	DiscordToken     string `mapstructure:"DISCORD_TOKEN"`
	BlizzardClientID string `mapstructure:"BLIZZARD_CLIENT_ID"`
	BlizzardSecret   string `mapstructure:"BLIZZARD_SECRET"`
}

func LoadConfig() (config Config, err error) {
	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
