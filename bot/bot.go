package bot

import (
	"fmt"
	"os"
	"os/signal"
	"strings"

	config "github.com/bezerker/sndbot/config"
	util "github.com/bezerker/sndbot/util"
	"github.com/bwmarrin/discordgo"
)

func RunBot(config config.Config) {

	BotToken := config.DiscordToken
	// create a session
	discord, err := discordgo.New("Bot " + BotToken)
	util.CheckNilErr(err)

	// add a event handler
	discord.AddHandler(newMessage)

	// open session
	discord.Open()
	defer discord.Close() // close session, after function termination

	// keep bot running until there is NO os interruption (ctrl + C)
	fmt.Println("Bot running....")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {

	/* prevent bot responding to its own message
	this is achived by looking into the message author id
	if message.author.id is same as bot.author.id then just return
	*/
	if message.Author.ID == discord.State.User.ID {
		return
	}

	// respond to user message if it contains `!help` or `!bye`
	switch {
	case strings.Contains(message.Content, "!help"):
		discord.ChannelMessageSend(message.ChannelID, "Hello World😃")
	case strings.Contains(message.Content, "!bye"):
		discord.ChannelMessageSend(message.ChannelID, "Good Bye👋")
		// add more cases if required
	case strings.Contains(message.Content, "!ping"):
		discord.ChannelMessageSend(message.ChannelID, "Pong🏓")
	}
}
