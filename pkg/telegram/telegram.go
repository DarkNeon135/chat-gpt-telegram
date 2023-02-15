package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"net/http"
	"open-gpt-telegram/internal/log"
	"open-gpt-telegram/pkg/db"
	"open-gpt-telegram/pkg/openAI"
	"strings"
	"sync"
	"time"
)

type Telegram struct {
	MongoConnector *db.MongoConnector
	BotApi         *tgbotapi.BotAPI
	OpenAIKey      string
	usersList      map[int64]telegramUser
}

type telegramUser struct {
	lastMessageTime time.Time
	isAllowed       bool
	requestCounter  int
	client          http.Client
	isUserNotified  bool
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

	telegram.usersList = make(map[int64]telegramUser)

	for update := range updates {

		go telegram.handleUserMessage(update)

	}
}

func (telegram *Telegram) handleUserMessage(update tgbotapi.Update) {

	if update.Message == nil {
		return
	}
	chatId := update.Message.Chat.ID

	if update.Message.IsCommand() {
		err := telegram.checkTelegramCommand(update, chatId)
		if err != nil {
			log.Error.Fatal(err)
		}
		return
	}

	_, isExist := telegram.usersList[chatId]

	isOk, err := telegram.processTelegramMessage(isExist, chatId, update)
	if err != nil {
		log.Error.Fatal(err)
	}
	if !isOk {
		return
	}

	gptAnswer, err := openAI.SendRequestToOpenAI(telegram.OpenAIKey, update.Message.Text, telegram.usersList[chatId].client)
	if err != nil && strings.HasSuffix(err.Error(), "context deadline exceeded") {
		_, err = telegram.BotApi.Send(tgbotapi.NewMessage(chatId, "OpenAI under the pressure, please try again in a several minutes."))
		if err != nil {
			log.Error.Fatal(fmt.Errorf("sending message to chat failed. Error: %s", err))
		}
		return

	} else if err != nil {
		log.Warning.Println(err)
	}

	_, err = telegram.BotApi.Send(tgbotapi.NewMessage(chatId, gptAnswer))
	if err != nil {
		log.Error.Fatal(fmt.Errorf("sending message to chat failed. Error: %s", err))
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

func (telegram *Telegram) checkAllowanceToChat(chatId int64, mut *sync.Mutex) bool {

	if !telegram.usersList[chatId].isAllowed {
		if !telegram.usersList[chatId].isUserNotified {
			_, err := telegram.BotApi.Send(tgbotapi.NewMessage(chatId, "You reached request limit, wait a minute to continue."+
				"\nAll your messages will be ignored!"))
			if err != nil {
				log.Error.Fatal(fmt.Errorf("sending message to chat failed. Error: %s", err))
			}
			mut.Lock()
			telegram.usersList[chatId] = telegramUser{
				lastMessageTime: telegram.usersList[chatId].lastMessageTime,
				requestCounter:  telegram.usersList[chatId].requestCounter,
				isAllowed:       true,
				isUserNotified:  true,
				client:          telegram.usersList[chatId].client,
			}
			mut.Unlock()
		}
		return false
	}
	return true
}

func (telegram *Telegram) processTelegramMessage(isExist bool, chatId int64, update tgbotapi.Update) (bool, error) {
	mut := sync.Mutex{}

	if !isExist {
		_, err := telegram.MongoConnector.CheckChatId(chatId) //checks user command /start
		if err != nil {
			return false, nil
		}

		mut.Lock()
		telegram.usersList[chatId] = telegramUser{
			lastMessageTime: time.Now(),
			isAllowed:       true,
			requestCounter:  1,
			client:          http.Client{},
		}
		mut.Unlock()

		if len(update.Message.Text) < 3 {
			_, err = telegram.BotApi.Send(tgbotapi.NewMessage(chatId, "Please, send message more than 3 chars."))
			if err != nil {
				return false, fmt.Errorf("sending message to chat failed. Error: %s", err)
			}
			return false, nil
		}

	} else {
		mut.Lock()
		telegram.usersList[chatId] = checkRequestsFrequency(telegram.usersList[chatId])
		mut.Unlock()

		if !telegram.checkAllowanceToChat(chatId, &mut) {
			return false, nil
		}

		if len(update.Message.Text) < 3 {
			_, err := telegram.BotApi.Send(tgbotapi.NewMessage(chatId, "Send message more than 3 chars"))
			if err != nil {
				return false, fmt.Errorf("sending message to chat failed. Error: %s", err)
			}
			return false, nil
		}

	}
	return true, nil
}

//func (telegram *Telegram) receivePreviousMessage(chatId int64) err {
//	chat, err := telegram.BotApi.GetChat(tgbotapi.ChatInfoConfig{
//		ChatConfig: tgbotapi.ChatConfig{ChatID: chatId},
//	})
//	if err != nil {
//
//	}
//}
