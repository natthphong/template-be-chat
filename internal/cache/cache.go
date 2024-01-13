package cache

import (
	"context"
	"coop-4/test/backend/config"
	"errors"
	"github.com/go-redis/redis/v9"
	"time"
)

// RedisClusterClient struct
var mode string

type redisClient struct {
	ClusterClient *redis.ClusterClient
	Client        *redis.Client
}

func (r *redisClient) Close() error {
	if mode == "cluster" {
		return r.ClusterClient.Close()
	}
	return r.Client.Close()
}

func (r redisClient) CMD() redis.Cmdable {
	if mode == "cluster" {
		return r.ClusterClient
	}
	return r.Client
}

func (r *redisClient) UniversalClient() redis.UniversalClient {
	if mode == "cluster" {
		return r.ClusterClient
	}
	return r.Client
}

func Initialize(ctx context.Context, config config.RedisConfig) (*redisClient, error) {

	var client = &redisClient{}
	mode = config.Mode

	cliCh := make(chan string)
	errCh := make(chan error)

	if config.Mode == "normal" {
		client.Client = redis.NewClient(&redis.Options{
			Addr:         config.Host + ":" + config.Port,
			Password:     config.Password,
			DB:           config.DB,
			PoolTimeout:  time.Second * 10,
			DialTimeout:  time.Second * 10,
			WriteTimeout: time.Second * 5,
			ReadTimeout:  -1,
		})
		go func() {
			cli, err := client.Client.Ping(ctx).Result()
			if err != nil {
				errCh <- err
			}
			if cli == "PONG" {
				cliCh <- cli
			}
		}()
	} else if config.Mode == "cluster" {
		client.ClusterClient = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        config.Cluster.Addr,
			PoolTimeout:  time.Second * 10,
			DialTimeout:  time.Second * 10,
			WriteTimeout: time.Second * 10,
			ReadTimeout:  -1,
		})
		go func() {
			cli, err := client.ClusterClient.Ping(ctx).Result()
			if err != nil {
				errCh <- err
			}
			if cli == "PONG" {
				cliCh <- cli
			}
		}()
	}

	select {
	case <-cliCh:
		return client, nil
	case errMsg := <-errCh:
		return nil, errors.New("Cannot connect to redis : " + errMsg.Error())
	case <-ctx.Done():
		return nil, errors.New("connect to redis timeout")
	}

}

type PingFunc func(ctx context.Context) error

func Ping(cmd redis.Cmdable) PingFunc {
	return func(ctx context.Context) error {
		return cmd.Ping(ctx).Err()
	}
}

type GetRedisFunc func(ctx context.Context, key string) (string, error)

func GetRedis(cmd redis.Cmdable) GetRedisFunc {
	return func(ctx context.Context, key string) (string, error) {
		return cmd.Get(ctx, key).Result()
	}
}

type DelRedisFunc func(ctx context.Context, key string) error

func DelRedis(cmd redis.Cmdable) DelRedisFunc {
	return func(ctx context.Context, key string) error {
		return cmd.Del(ctx, key).Err()
	}
}

type SetRedisFunc func(ctx context.Context, key string, value interface{}, expiration time.Duration) error

func SetRedis(cmd redis.Cmdable) SetRedisFunc {
	return func(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
		return cmd.Set(ctx, key, value, expiration).Err()
	}
}

type InCrRedisFunc func(ctx context.Context, key string) (int64, error)

func InCrRedis(cmd redis.Cmdable) InCrRedisFunc {
	return func(ctx context.Context, key string) (int64, error) {
		return cmd.Incr(ctx, key).Result()
	}
}

type SubscribeChannelFunc func(ctx context.Context, chanel string, redisMsgCh chan string)

func SubscribeChannel(pubsub *redis.PubSub) SubscribeChannelFunc {
	return func(ctx context.Context, chanel string, redisMsgCh chan string) {
		channel := pubsub.Channel()
		go func() {
			for msg := range channel {
				redisMsgCh <- msg.Payload
			}
		}()
	}
}

type PublishRedisFunc func(ctx context.Context, channel string, message interface{}) error

func PublishRedis(client *redisClient) PublishRedisFunc {
	return func(ctx context.Context, channel string, message interface{}) error {
		if mode == "cluster" {
			return client.ClusterClient.Publish(ctx, channel, message).Err()
		}
		return client.Client.Publish(ctx, channel, message).Err()
	}
}
