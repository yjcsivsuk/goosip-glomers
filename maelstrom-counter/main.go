package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	node := maelstrom.NewNode()
	kv := maelstrom.NewSeqKV(node)
	ctx := context.Background()
	ch := make(chan int, 100)  // 创建一个缓冲区大小为100的整数型channel

	// 启动一个goroutine，用于处理计数器的更新。从channel中接收增量delta的值，并尝试在键值存储中更新计数器的值。如果更新失败，它会将增量值重新放回channel中，以便重试
	go func() {
		for {
			select {
			case delta := <-ch:
				v, err := kv.ReadInt(ctx, "counter")
				if err != nil {
					// 如果key不存在，就忽略
					switch t := err.(type) {
						case *maelstrom.RPCError:
							if t.Code != maelstrom.KeyDoesNotExist {
								ch <- delta
								continue
							}
						default:
							ch <- delta
							continue
					}
				}

				if err := kv.CompareAndSwap(ctx, "counter", v, v+delta, true); err != nil {
					ch <- delta
					continue
				}
			}
		}
	}()

	// 处理add类型的消息。这个函数从消息体中解析出要增加的值，转为int类型孩子还发送到channel中。然后发送一个add_ok的回复来确认消息已被接收
	node.Handle("add", func(msg maelstrom.Message) error {
		var reqBody map[string]any
		if err := json.Unmarshal(msg.Body, &reqBody); err != nil {
			return err
		}

		ch <- reqBody["delta"].(int)

		body := make(map[string]any)
		body["type"] = "add_ok"
		return node.Reply(msg, body)
	})

	// 处理read类型的消息。这个函数首先向kv存储中写入一个唯一的值，以确保后续的读取操作能够获取最新的计数器值。然后读取计数器的值并回复给发送者
	node.Handle("read", func(msg maelstrom.Message) error {

		// 生成一个随机数，并写入到切片buf中
		buf := make([]byte, 32)
		_, err := rand.Read(buf)
		if err != nil {
			return err
		}

		if err := kv.Write(ctx, fmt.Sprintf("%s-scratch", node.ID()), buf); err != nil {
			return err
		}

		v, err := kv.ReadInt(ctx, "counter")
		
		if err != nil {
			switch t := err.(type) {
			case *maelstrom.RPCError:
				if t.Code != maelstrom.KeyDoesNotExist {
					return err
				}
			default:
				return err
			}
		}

		body := make(map[string]any)
		body["type"] = "read_ok"
		body["value"] = v
		return node.Reply(msg, body)
	})

	if err := node.Run(); err != nil {
		log.Fatal(err)
	}

}
