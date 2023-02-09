package db

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"open-gpt-telegram/internal/log"
	"open-gpt-telegram/pkg/common"
	"time"
)

const (
	database            = "telegram"
	arbitrageCollection = "arbitrageBot"
)

type MongoDB interface {
	Insert(chatId int64) error
	Delete(chatID int64) error
	GetChatIdList() ([]int64, error)
	CheckChatId(chatId int64) (bool, error)
}

type MongoConnector struct {
	client              *mongo.Client
	arbitrageCollection *mongo.Collection
}

func ConnectToMongoDB(dbUri string) (*MongoConnector, error) {
	connector := new(MongoConnector)
	clientOptions := options.Client().ApplyURI(dbUri)
	var err error

	connector.client, err = mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	connector.arbitrageCollection = connector.client.Database(database).Collection(arbitrageCollection)

	if err = connector.client.Ping(context.TODO(), nil); err != nil {
		return nil, err
	}
	log.Info.Println("Successfully connected to MongoDB!")

	return connector, nil
}

func (connector *MongoConnector) Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error

	if err = connector.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("db disconnecting error. Error: %s", err)
	}

	log.Info.Printf("Successfully disconnected from database: %s!\n", database)
	return nil
}

func (connector *MongoConnector) Insert(chatId int64) error {
	_, err := connector.arbitrageCollection.InsertOne(context.TODO(), bson.D{{"chatId", chatId}})
	if err != nil {
		return fmt.Errorf("mongo db inserting error. Error: %s", err)
	}

	log.Info.Printf("Successfully inserted %d to %s!\n", chatId, arbitrageCollection)
	return nil
}

func (connector *MongoConnector) Delete(chatId int64) error {
	_, err := connector.arbitrageCollection.DeleteOne(context.TODO(), bson.D{{"chatId", chatId}})
	if err != nil {
		return fmt.Errorf("mongo db deleting error. Error: %s", err)
	}

	log.Info.Printf("Successfully removed %d from arbitrageCollection: %s!\n", chatId, arbitrageCollection)

	return nil
}

func (connector *MongoConnector) GetChatIdList() ([]int64, error) {
	cursor, err := connector.arbitrageCollection.Find(context.TODO(), bson.D{})
	if err != nil {
		return nil, fmt.Errorf("mongo db searching error. Error: %s", err)
	}
	searchResult := make([]int64, 0)
	for cursor.Next(context.Background()) {
		var elem common.TelegramChatList
		if err = cursor.Decode(&elem); err != nil {

			return nil, fmt.Errorf("mongo db decoding searching result error. Error: %s", err)
		}
		searchResult = append(searchResult, elem.ChatId)
	}
	defer cursor.Close(context.TODO())

	log.Info.Printf("Successfully retrieved info from collection: %s!\n", arbitrageCollection)

	return searchResult, nil
}

func (connector *MongoConnector) CheckChatId(chatId int64) (bool, error) {
	var elem common.TelegramChatList

	err := connector.arbitrageCollection.FindOne(context.TODO(), bson.D{{"chatId", chatId}}).Decode(&elem)
	if err != nil {
		return false, fmt.Errorf("mongo db single searching error. Error: %s", err)
	}
	if elem.ChatId == 0 {
		return false, nil
	}

	return true, nil
}
