package dashboard

import (
	"context"
	"coop-4/test/backend/internal/cache"
	"fmt"
	"github.com/go-redis/redis/v9"
	"github.com/gofiber/websocket/v2"

	"go.uber.org/zap"
)

func Chat(
	redisClient redis.UniversalClient,
	pub cache.PublishRedisFunc,
) func(w *websocket.Conn) {
	return func(w *websocket.Conn) {
		var register = make(chan *websocket.Conn)
		var unregister = make(chan *websocket.Conn)
		redisMsgCh := make(chan string)
		logger := zap.L()
		ctx := context.Background()
		roomNumber := w.Query("roomNumber")
		redisPriceSub := redisClient.Subscribe(ctx, "ROOM:"+roomNumber)
		defer func(redisPriceSub *redis.PubSub) {
			err := redisPriceSub.Close()
			if err != nil {
				logger.Error("Closing redis subscriber", zap.Error(err))
			}
		}(redisPriceSub)
		go cache.SubscribeChannel(redisPriceSub)(ctx, "ROOM:"+roomNumber, redisMsgCh)
		go SocketListener(logger, register, unregister, w, redisMsgCh, pub, ctx)
		defer func() {
			unregister <- w
			_ = w.Close()
			logger.Info("websocket was disconnected")
		}()
		register <- w
		for {
			select {
			case msg := <-redisMsgCh:

				err := w.WriteMessage(websocket.TextMessage, []byte(msg))
				if err != nil {
					break
				}
			case c := <-unregister:
				err := c.Close()
				if err != nil {
					logger.Error("Closing socket", zap.Error(err))
				}
				logger.Info("Websocket was disconnected.")
				return

			}
		}

	}

}

func SocketListener(
	logger *zap.Logger,
	register chan *websocket.Conn,
	unregister chan *websocket.Conn,
	ws *websocket.Conn,
	msg chan string,
	pub cache.PublishRedisFunc,
	ctx context.Context,
) {
	for {
		select {
		case c := <-register:
			fmt.Println("connect with : ", c.Params("roomNumber"))

			//append to list of co
		case c := <-unregister:
			//remove conection from list of clients
			_ = c.Close()
			logger.Info("Closed connection\n")
		default:
			_, message, err := ws.ReadMessage()
			if err != nil {
				logger.Error("ReadMessage", zap.Error(err))
				unregister <- ws
				return
			}

			_ = pub(ctx, "ROOM:"+ws.Query("roomNumber"), message)

			fmt.Printf("Got message of type: %s\nMessage:%s\n", ws.Query("roomNumber"), string(message))

		}

	}
}
