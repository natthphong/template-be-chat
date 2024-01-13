package main

import (
	"context"
	"coop-4/test/backend/chat"
	"coop-4/test/backend/config"
	"coop-4/test/backend/internal/cache"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	_ "github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

func main() {

	config.InitTimeZone()
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Unable to initial config."))
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		logger = zap.L()

		app = initFiber()
	)

	redisClient, err := cache.Initialize(ctx, cfg.RedisConfig)
	if err != nil {
		logger.Fatal("server connect to redis", zap.Error(err))
	}

	defer func() {
		err = redisClient.Close()
		if err != nil {
			logger.Fatal("closing redis connection error", zap.Error(err))
		}
	}()
	redisCMD := redisClient.CMD()

	app.Get("/health", makeHealthHandler(cache.Ping(redisCMD)))

	app.Get("/chat", websocket.New(dashboard.Chat(redisClient.UniversalClient(),
		cache.PublishRedis(redisClient),
	)))

	logger.Info(fmt.Sprintf("Listening on port: %s", cfg.Server.Port))
	func() {
		if err = app.Listen(fmt.Sprintf(":%v", cfg.Server.Port)); err != nil {
			logger.Fatal(err.Error())
		}
	}()

}

func ChanBasic() {

	msg1 := make(chan string)
	msg2 := make(chan string)
	go func() {
		for {
			select {
			case m := <-msg1:
				//TODO
				fmt.Println("from msg1 : ", m)
			case m := <-msg2:
				fmt.Println("from msg2 : ", m)
			}
		}
	}()

	for {
		num := rand.Int()
		if num%2 == 0 {
			msg1 <- "hello1 " + strconv.Itoa(num)
		} else {
			msg2 <- "hello2 " + strconv.Itoa(num)
		}

		time.Sleep(1 * time.Second)
	}

}

func initFiber() *fiber.App {
	app := fiber.New(
		fiber.Config{
			ReadTimeout:           5 * time.Second,
			WriteTimeout:          5 * time.Second,
			IdleTimeout:           30 * time.Second,
			DisableStartupMessage: true,
			CaseSensitive:         true,
			StrictRouting:         true,
		},
	)

	app.Use(SetHeaderID())
	return app
}

func SetHeaderID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		randomTrace := uuid.New().String()
		traceId := c.Get("traceId")

		if traceId == "" {
			traceId = randomTrace
		}

		c.Accepts(fiber.MIMEApplicationJSON)
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		c.Request().Header.Set("traceId", traceId)
		return c.Next()
	}
}

func makeHealthHandler(checkRedis cache.PingFunc) fiber.Handler {
	return func(c *fiber.Ctx) error {

		if err := checkRedis(c.Context()); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"status": "internal server error",
			})
		}
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"status": "healthy",
		})
	}
}
