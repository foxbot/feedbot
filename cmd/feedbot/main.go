package main

import (
	"flag"
	"fmt"

	"github.com/foxbot/feedbot"
)

var token = flag.String("token", "", "token=unprefixed token")

func main() {
	println("feedbot")
	flag.Parse()

	t := fmt.Sprintf("Bot %s", *token)
	bot, err := feedbot.NewBot(t)
	if err != nil {
		panic(err)
	}

	err = bot.Run()
	if err != nil {
		panic(err)
	}
}
