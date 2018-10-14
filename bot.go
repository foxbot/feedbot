package feedbot

import (
	"log"
	"os"

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
