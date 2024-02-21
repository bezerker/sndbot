package config

import (
	util "github.com/bezerker/sndbot/util"
	"github.com/spf13/viper"
)

type Config struct {
	DiscordToken         string
	BlizzardClientID     string
	BlizzardClientSecret string
}

func ReadConfig(configfile string) Config {
	viper.SetConfigName(configfile)
	viper.AddConfigPath(".")
	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	util.CheckNilErr(err)
	return Config{
		DiscordToken:         viper.GetString("discord.token"),
		BlizzardClientID:     viper.GetString("blizzard.clientid"),
		BlizzardClientSecret: viper.GetString("blizzard.clientsecret"),
	}
}
