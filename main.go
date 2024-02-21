package main

import (
	bot "github.com/bezerker/sndbot/bot"
	config "github.com/bezerker/sndbot/config"
)

func main() {
	config := config.ReadConfig("config.yaml") // call the readConfig function of config.go
	bot.RunBot(config)                         // Run the bot passing in required arguments
}
