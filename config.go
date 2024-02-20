package main

import "github.com/spf13/viper"

type Config struct {
	DiscordToken string
}

func readConfig(configfile string) Config {
	viper.SetConfigName(configfile)
	viper.AddConfigPath(".")
	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	bot.checkNilErr(err)

	return Config{
		DiscordToken: viper.GetString("discord.token"),
	}
}
