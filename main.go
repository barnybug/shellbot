package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/google/shlex"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

var htmlReplacer = strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")

func htmlEscape(s string) string {
	return htmlReplacer.Replace(s)
}

func runCommand(api *tgbotapi.BotAPI, chatID int64, text string) {
	args, err := shlex.Split(text)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("️️️⚠️ Couldn't parse: %s", err))
		api.Send(msg)
		return
	}

	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ %s", err))
		api.Send(msg)
	}
	if output != nil {
		log.Println("<-", string(output))
		text := "<code>" + htmlEscape(string(output)) + "</code>"
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		api.Send(msg)
	}
}

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	chatID, _ := strconv.Atoi(os.Getenv("TELEGRAM_CHAT_ID"))
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalln("Error connecting to telegram api", err)
	}
	config := tgbotapi.NewUpdate(0)
	config.Timeout = 60
	updates, err := api.GetUpdatesChan(config)
	if err != nil {
		log.Fatalln("Error getting channel", err)
	}
	log.Println("Running...")
	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.Chat.ID != int64(chatID) {
			log.Println("Channel ID:", update.Message.Chat.ID)
			continue
		}

		log.Println("->", update.Message.Text)
		if update.Message.IsCommand() {
			runCommand(api, update.Message.Chat.ID, update.Message.Text[1:])
		} else {
			runCommand(api, update.Message.Chat.ID, update.Message.Text)
		}
	}

}
