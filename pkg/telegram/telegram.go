package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"open-gpt-telegram/internal/log"
	"open-gpt-telegram/pkg/db"
	"time"
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

func (telegram *Telegram) CheckUsersStatus() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := telegram.BotApi.GetUpdatesChan(u)

	usersList := make(map[int64]telegramUser)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		chatId := update.Message.Chat.ID

		_, isExist := usersList[chatId]

		if !isExist {
			usersList[chatId] = telegramUser{
				lastMessageTime: time.Now(),
				isAllowed:       true,
				requestCounter:  1,
			}
		} else {
			usersList[chatId] = checkRequestsFrequency(usersList[chatId])

			if !usersList[chatId].isAllowed {
				if !usersList[chatId].isUserNotified {
					_, err := telegram.BotApi.Send(tgbotapi.NewMessage(chatId, "You reached request limit, wait a minute to continue."+
						"\nAll your messages will be ignored!"))
					if err != nil {
						return fmt.Errorf("sending message to chat failed. Error: %s", err)
					}
					usersList[chatId] = telegramUser{
						lastMessageTime: usersList[chatId].lastMessageTime,
						requestCounter:  usersList[chatId].requestCounter,
						isAllowed:       true,
						isUserNotified:  true,
					}
				}
				continue
			}
		}

		if update.Message.IsCommand() {
			err := telegram.checkTelegramCommand(update, chatId)
			if err != nil {
				return err
			}

		} else {
			//TODO: Call OpenAI func

		}

	}
	return nil
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

func (telegram *Telegram) checkTelegramCommand(update tgbotapi.Update, chatId int64) error {
	msg := tgbotapi.NewMessage(chatId, "")

	switch update.Message.Command() {
	case "start":
		_, err := telegram.MongoConnector.CheckChatId(chatId)
		if err != nil {
			if err = telegram.MongoConnector.Insert(chatId); err != nil {
				return fmt.Errorf("inserting in mongoDB failed. Error: %s", err)
			}
		} else {
			msg.Text = "You are already in the system!"
			break
		}
		msg.Text = "Glad to see you here!\nPlease, enter your question."
	case "stop":
		if err := telegram.MongoConnector.Delete(chatId); err != nil {
			return fmt.Errorf("deleting from mongoDB failed. Error: %s", err)
		}
		msg.Text = "You are successfully disconnected!"
	default:
		msg.Text = "I don't know that command!"
	}
	if _, err := telegram.BotApi.Send(msg); err != nil {
		return fmt.Errorf("sending message to chat failed. Error: %s", err)
	}

	return nil
}

func checkRequestsFrequency(userInfo telegramUser) telegramUser {
	now := time.Now()

	userInfo.requestCounter++

	if now.After(userInfo.lastMessageTime.Add(time.Minute)) {
		userInfo.requestCounter = 0
		userInfo.isUserNotified = false
		userInfo.lastMessageTime = now
		userInfo.isAllowed = true

		return userInfo
	}
	if userInfo.requestCounter > 4 && now.Before(userInfo.lastMessageTime.Add(time.Minute)) {
		userInfo.isAllowed = false
		return userInfo
	}

	return userInfo
}

type telegramUser struct {
	lastMessageTime time.Time
	isAllowed       bool
	requestCounter  int
	isUserNotified  bool
}
