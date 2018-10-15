package feedbot

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

var l = log.New(os.Stdout, "bot", log.Lshortfile|log.Ltime)

// Bot contains the Bot's state
type Bot struct {
	c  *Controller
	dg *discordgo.Session
	fc *FeedChecker
}

// NewBot creates a new bot instance
func NewBot(token string) (*Bot, error) {
	session, err := discordgo.New(token)
	if err != nil {
		return nil, err
	}

	c, err := NewController()
	if err != nil {
		return nil, err
	}

	fc, err := NewFeedChecker(c)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		c:  c,
		dg: session,
		fc: fc,
	}

	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onMessageCreate)

	return bot, nil
}

// Run the bot
func (bot *Bot) Run() error {
	err := bot.dg.Open()
	if err != nil {
		return err
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, os.Kill)
	<-sc

	return nil
}

func (bot *Bot) onGuildCreate(s *discordgo.Session, e *discordgo.GuildCreate) {
	if e.Guild.Unavailable {
		return
	}
	println("joined guild", e.Name)
	contact := "u:" + e.OwnerID
	err := bot.c.CreateGuildConfig(e.ID, contact)
	if err != nil {
		log.Println(fmt.Sprintf("evt:join err:%v", err))
	}
}
func (bot *Bot) onGuildDelete(s *discordgo.Session, e *discordgo.GuildDelete) {
	if e.Guild.Unavailable {
		return
	}
	println("left guild", e.Name)
}
