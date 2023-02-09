package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/posipaka-trade/posipaka-trade-cmn/log"
	"openAI/pkg/db"
)

type Telegram struct {
	MongoConnector *db.MongoConnector
	updated        bool
	BotApi         *tgbotapi.BotAPI
}

func ConnectToTelegram(apiToken string) (*tgbotapi.BotAPI, error) {
	botApi, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		return &tgbotapi.BotAPI{}, fmt.Errorf("connecting to telegram network error. Error: %s", err)
	}

	botApi.Debug = false
	log.Info.Println("Successfully connected to telegram network!")
	return botApi, nil
}

func (telegram *Telegram) CheckUsersStatus() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := telegram.BotApi.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if !update.Message.IsCommand() {
			continue
		}

		chatId := update.Message.Chat.ID
		msg := tgbotapi.NewMessage(chatId, "")

		switch update.Message.Command() {
		case "start":
			_, err := telegram.MongoConnector.CheckChatId(chatId)
			if err != nil {
				if err = telegram.MongoConnector.Insert(chatId); err != nil {
					log.Error.Fatal(err)
				}
			} else {
				msg.Text = "You are already in the system!"
				break
			}
			msg.Text = "Glad to see you here!"
		case "stop":
			if err := telegram.MongoConnector.Delete(chatId); err != nil {
				log.Error.Fatal(err)
			}
			msg.Text = "You are successfully disconnected!"
		default:
			msg.Text = "I don't know that command!"
		}
		if _, err := telegram.BotApi.Send(msg); err != nil {
			log.Error.Fatal(err)
		}
	}
}

func (telegram *Telegram) SendMessagesToChannel(message string) error {
	chatIdList, err := telegram.MongoConnector.GetChatIdList()
	if err != nil {
		return err
	}

	for _, chatId := range chatIdList {
		msg := tgbotapi.NewMessage(chatId, message)
		if _, err = telegram.BotApi.Send(msg); err != nil {
			return fmt.Errorf("sending message to telegram channel error. Error: %s", err)
		}
	}
	return nil
}

func checkRequestsFrequency(chatId int64) {

}
