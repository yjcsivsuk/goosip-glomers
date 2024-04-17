package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"golang.org/x/exp/slices"
)

type BroadcastRequestBody struct {
	Type    string `json:"type"`
	Message uint64 `json:"message"`
}

type BroadcastResponseBody struct {
	Type string `json:"type"`
}

type ReadRequestBody struct {
	Type string `json:"type"`
}

type ReadResponseBody struct {
	Type     string   `json:"type"`
	Messages []uint64 `json:"messages"`
}

type TopologyRequestBody struct {
	Type     string              `json:"type"`
	Topology map[string][]string `json:"topology"`
}

type TopologyResponseBody struct {
	Type string `json:"type"`
}

type BroadcastOkResponseBody struct {
	InReplyTo int    `json:"in_reply_to"`
	Type      string `json:"type"`
}

type UnansweredNodeBroadcast struct {
	Message uint64
	Dest    string
	SentAt  time.Time
}

func main() {
	var messages []uint64
	var destinations []string
	var messagesMutex sync.Mutex  // 防止在写入消息时出现竞争

	n := maelstrom.NewNode()

	n.Handle("broadcast_ok", func(msg maelstrom.Message) error {
		return nil
	})

	// 当节点收到broadcast类型的消息时，它会检查是否已经处理过该消息
	n.Handle("broadcast", func(msg maelstrom.Message) error {
		var body BroadcastRequestBody

		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// 如果已经收到了消息，停止广播
		if slices.Contains(messages, body.Message) {
			return n.Reply(msg, BroadcastResponseBody{
				Type: "broadcast_ok",
			})
		}

		// 如果没有，节点将消息添加到其内部列表，并向其所有目的地节点广播该消息
		messagesMutex.Lock()
		messages = append(messages, body.Message)
		messagesMutex.Unlock()

		// 在广播中，启动一个goroutine，使用一个循环来确保消息被发送到所有目的地节点。它使用RPC调用向每个节点发送消息，并等待确认。如果有任何节点没有回复，它会继续尝试，直到所有节点都回复
		go func(destinations []string, body BroadcastRequestBody) {
			var successfulSentDestinations []string
			var successfulSentDestinationsMutex sync.Mutex
			// 直到所有目的地结点都收到消息
			for len(destinations) != len(successfulSentDestinations) {
				// 遍历当前节点的所有目的地节点
				for _, node := range destinations {
					// 对每个目的地节点使用RPC机制发送消息
					n.RPC(node, body, func(msg maelstrom.Message) error {
						var broadcastOkBody BroadcastOkResponseBody

						if err := json.Unmarshal(msg.Body, &broadcastOkBody); err != nil {
							return err
						}

						// 检查它是否是预期的broadcast_ok类型
						if broadcastOkBody.Type != "broadcast_ok" {
							return fmt.Errorf("expeted type broadcast_ok, got %s", broadcastOkBody.Type)
						}

						log.Printf("from %s, to %s, sent %v", n.ID(), node, body)

						// 使用互斥锁更新已成功发送消息的目的地节点列表
						successfulSentDestinationsMutex.Lock()
						defer successfulSentDestinationsMutex.Unlock()

						if !slices.Contains(successfulSentDestinations, node) {
							successfulSentDestinations = append(successfulSentDestinations, node)
						}

						return nil
					})
				}

				time.Sleep(1 * time.Second)
			}
		}(destinations, body)

		return n.Reply(msg, BroadcastResponseBody{
			Type: "broadcast_ok",
		})
	})
	
	// 当节点收到read类型的消息时，它会返回其内部消息列表
	n.Handle("read", func(msg maelstrom.Message) error {
		var body map[string]any

		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		return n.Reply(msg, ReadResponseBody{
			Type:     "read_ok",
			Messages: messages,
		})
	})
	
	// 当节点收到topology类型的消息时，它会更新其目的地节点的列表
	n.Handle("topology", func(msg maelstrom.Message) error {
		var body TopologyRequestBody

		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		destinations = body.Topology[n.ID()]

		return n.Reply(msg, TopologyResponseBody{
			Type: "topology_ok",
		})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
