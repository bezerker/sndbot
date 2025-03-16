package config

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	DiscordToken       string   `mapstructure:"DISCORD_TOKEN"`
	BlizzardClientID   string   `mapstructure:"BLIZZARD_CLIENT_ID"`
	BlizzardSecret     string   `mapstructure:"BLIZZARD_SECRET"`
	DBPath             string   `mapstructure:"DB_PATH"`
	CommunityRoleID    string   `mapstructure:"COMMUNITY_ROLE_ID"`
	GuildMemberRoleIDs []string `mapstructure:"GUILD_MEMBER_ROLE_IDS"`
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

	// First unmarshal the basic config
	err = viper.Unmarshal(&config)
	if err != nil {
		return
	}

	// Handle the JSON array for guild member role IDs
	roleIDsStr := viper.GetString("GUILD_MEMBER_ROLE_IDS")
	if roleIDsStr != "" {
		var roleIDs []string
		err = json.Unmarshal([]byte(roleIDsStr), &roleIDs)
		if err != nil {
			return config, fmt.Errorf("failed to parse GUILD_MEMBER_ROLE_IDS: %v", err)
		}
		config.GuildMemberRoleIDs = roleIDs
	}

	return
}
