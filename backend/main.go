package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type manager struct {
	connection *mongo.Client
	ctx        context.Context
	cancel     context.CancelFunc
}

var Mgr Manager

type Manager interface {
	Insert(interface{}) error
	GetAll() ([]Data, error)
	UpdateData(Data) error
	InsertOrUpdate(Data) error
}

func getEnvVars() {
	err := godotenv.Load("Credentials.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func connectDb() {
	godotenv.Load("credentials.env")
	mongouri := os.Getenv("MONGOURI")
	client, err := mongo.NewClient(options.Client().ApplyURI(mongouri))
	if err != nil {
		fmt.Println(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		fmt.Println(err)
		return
	}
	Mgr = &manager{connection: client, ctx: ctx, cancel: cancel}
}

func close(client *mongo.Client, ctx context.Context,
	cancel context.CancelFunc) {
	defer cancel()

	defer func() {

		if err := client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()
}

func init() {
	connectDb()
}

type Data struct {
	Score    int    `json:"score"`
	UserName string `json:"userName"`
}

func main() {

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// bigdata := []Data{}

	app.Get("/healthcheck", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	app.Post("/updatescore", func(c *fiber.Ctx) error {
		var newdata Data

		if err := c.BodyParser(&newdata); err != nil {
			return err
		}

		err := Mgr.InsertOrUpdate(newdata)
		if err != nil {
			return err
		}

		return c.JSON("Data updated successfully!")
	})

	app.Get("/getall", func(c *fiber.Ctx) error {
		// var newdata Data

		// if err := c.BodyParser(&newdata); err != nil {
		// 	return err
		// }

		bigdata, err := Mgr.GetAll()
		if err != nil {
			return err
		}

		return c.JSON(bigdata)
	})
	log.Fatal(app.Listen(":4000"))
}

func (mgr *manager) InsertOrUpdate(data Data) error {
	orgCollection := mgr.connection.Database("cardgame").Collection("users")

	// Check if the data.UserName already exists in the database
	filter := bson.M{"username": data.UserName}
	var existingData Data
	err := orgCollection.FindOne(context.Background(), filter).Decode(&existingData)
	if err == nil {
		// User exists, compare scores
		if existingData.Score != data.Score {
			// Scores are different, update the score in the database
			update := bson.M{"$set": bson.M{"score": data.Score}}
			_, err := orgCollection.UpdateOne(context.Background(), filter, update)
			if err != nil {
				return err
			}
			return nil
		}
		// Scores are the same, do nothing
		return nil
	} else if err == mongo.ErrNoDocuments {
		// User does not exist, insert the data into the database
		_, err := orgCollection.InsertOne(context.Background(), data)
		if err != nil {
			return err
		}
		return nil
	} else {
		// Error occurred while querying the database
		return err
	}
}

func (mgr *manager) Insert(data interface{}) error {
	orgCollection := mgr.connection.Database("cardgame").Collection("users")
	result, err := orgCollection.InsertOne(context.TODO(), data)
	fmt.Println(result.InsertedID)
	return err
}

func (mgr *manager) UpdateData(data Data) error {
	orgCollection := mgr.connection.Database("cardgame").Collection("users")

	filter := bson.M{"userName": data.UserName} // Filter by username
	update := bson.M{"$set": bson.M{"score": data.Score}}

	_, err := orgCollection.UpdateOne(context.TODO(), filter, update)
	return err
}

func (mgr *manager) GetAll() ([]Data, error) {
	orgCollection := mgr.connection.Database("cardgame").Collection("users")

	// Define sorting and limiting options
	opts := options.Find().SetSort(bson.D{{"score", -1}}).SetLimit(5)

	// fmt.Println("ok here")
	// Find all data sorted by score in descending order and limit to 10 entries
	cursor, err := orgCollection.Find(context.Background(), bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var results []Data
	if err := cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}

	return results, nil
}
