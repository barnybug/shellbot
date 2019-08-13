package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

const ScriptTimeout = time.Minute * 20

var htmlReplacer = strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")

func htmlEscape(s string) string {
	return htmlReplacer.Replace(s)
}

type ReplyWriter struct {
	api    *tgbotapi.BotAPI
	chatID int64
}

func (r ReplyWriter) Write(b []byte) (int, error) {
	log.Println("<-", string(b))
	text := "<code>" + htmlEscape(string(b)) + "</code>"
	msg := tgbotapi.NewMessage(r.chatID, text)
	msg.ParseMode = "HTML"
	r.api.Send(msg)
	return len(b), nil
}

// io.Writer that simply resets a Timer on data (which it discards).
type TimerResetter struct {
	timer *time.Timer
}

func (r TimerResetter) Write(b []byte) (int, error) {
	r.timer.Reset(ScriptTimeout)
	return len(b), nil
}

func runCommand(api *tgbotapi.BotAPI, chatID int64, text string) {
	args := []string{
		"-c",
		text,
	}
	cmd := exec.Command("/bin/bash", args...)
	stdout, err := cmd.StdoutPipe()
	stderr, err := cmd.StderrPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ %s", err))
		api.Send(msg)
		return
	}
	timer := time.NewTimer(ScriptTimeout)
	timerResetter := TimerResetter{timer}
	var wg sync.WaitGroup
	wg.Add(2)
	replyWriter := ReplyWriter{api, chatID}
	// write reply back and reset timer
	wr := io.MultiWriter(replyWriter, timerResetter)
	// copy stdout to the stream
	go func() {
		defer wg.Done()
		io.Copy(wr, stdout)
	}()
	// copy stderr to the stream
	go func() {
		defer wg.Done()
		io.Copy(wr, stderr)
	}()

	done := make(chan error, 1)
	go func() {
		// docs: "it is incorrect to call Wait before all reads from the pipe have completed"
		wg.Wait()
		done <- cmd.Wait()
	}()

	go func() {
		// wait for first of process finishing or timeout
		select {
		case err := <-done:
			// process finished
			if err != nil {
				if _, ok := err.(*exec.ExitError); ok {
					msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ %s", err))
					api.Send(msg)
				} else {
					msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ %s", err))
					api.Send(msg)
				}
			}
		case <-timer.C:
			// timeout - kill process
			cmd.Process.Kill()
			msg := tgbotapi.NewMessage(chatID, "⏰ Timeout!")
			api.Send(msg)
		}

		// cleanup
		timer.Stop()
	}()
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
