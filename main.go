package main

import (
	"os"

	bot "github.com/bezerker/sndbot/bot"
)

func main() {
	bot.BotToken = os.Getenv("SNDBOT_DISCORD_TOKEN") // set the bot token from the environment variable
	bot.Run()                                        // call the run function of bot/bot.go
}
