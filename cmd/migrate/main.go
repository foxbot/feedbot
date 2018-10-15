package main

import (
	"os"

	"github.com/foxbot/feedbot"
)

func main() {
	if _, err := os.Stat("data.db"); !os.IsNotExist(err) {
		panic("databae already exists, please remove first")
	}
	println("migrating up...")
	c, err := feedbot.NewController()
	if err != nil {
		panic(err)
	}
	err = c.CreateTables()
	if err != nil {
		panic(err)
	}
	println("ok!")
}
