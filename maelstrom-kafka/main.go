package main

import (
	"encoding/json"
	"log"
	"sort"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type SendBody struct {
	Type string `json:"type"`
	Key  string `json:"key"`
	Msg  int    `json:"msg"`
}

type SendOkBody struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
}

type PollBody struct {
	Type    string         `json:"type"`
	Offsets map[string]int `json:"offsets"`
}

type PollOkBody struct {
	Type string             `json:"type"`
	Msgs map[string][][]int `json:"msgs"`
}

type CommitOffsetsBody struct {
	Type    string         `json:"type"`
	Offsets map[string]int `json:"offsets"`
}

type CommitOffsetsOkBody struct {
	Type string `json:"type"`
}

type ListCommittedOffsetsBody struct {
	Type string   `json:"type"`
	Keys []string `json:"keys"`
}

type ListCommittedOffsetsOkBody struct {
	Type    string         `json:"type"`
	Offsets map[string]int `json:"offsets"`
}

func main() {
	// 定义日志的结构体
	type logs struct {
		mutex   sync.RWMutex
		offsets map[string]int
		msgs    map[string]map[int]int
	}

	// 已提交和未提交
	var committed logs
	var uncommitted logs

	uncommitted.offsets = make(map[string]int)
	uncommitted.msgs = make(map[string]map[int]int)

	committed.offsets = make(map[string]int)
	committed.msgs = make(map[string]map[int]int)

	n := maelstrom.NewNode()

	// 处理send类型的消息。这个函数将消息添加到未提交日志，并回复一个send_ok消息，包含消息的偏移量
	n.Handle("send", func(msg maelstrom.Message) error {
		var body SendBody

		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		uncommitted.mutex.Lock()
		defer uncommitted.mutex.Unlock()

		// 检查未提交日志的msgs是否有send类型消息的key，没有就创建一个
		if _, ok := uncommitted.msgs[body.Key]; !ok {
			uncommitted.msgs[body.Key] = make(map[int]int)
		}

		// 检查未提交日志的offsets是否有send类型消息的key，没有就将offsets置0，有的话就赋给send类型消息的offset
		if _, ok := uncommitted.offsets[body.Key]; !ok {
			uncommitted.offsets[body.Key] = 0
		}

		offset := uncommitted.offsets[body.Key]

		// 将消息存储在未提交日志的msgs中，键为body.Key，值为消息内容和偏移量
		uncommitted.msgs[body.Key][offset] = body.Msg

		err := n.Reply(msg, SendOkBody{
			Type:   "send_ok",
			Offset: offset,
		})

		if err != nil {
			return err
		}

		// 更新偏移量offset
		offset++
		uncommitted.offsets[body.Key] = offset

		return nil
	})

	// 处理poll类型的消息。这个函数返回从指定偏移量开始的消息列表
	n.Handle("poll", func(msg maelstrom.Message) error {
		var body PollBody

		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		msgs := make(map[string][][]int)

		uncommitted.mutex.RLock()
		defer uncommitted.mutex.RUnlock()

		// 遍历body.Offsets字典中的键值对。检查未提交日志的msgs中是否存在key对应的键，不存在就跳过ß
		for key, requestedOffset := range body.Offsets {
			keyMsgs, ok := uncommitted.msgs[key]
			if !ok {
				continue
			}

			// 存在的话就获取offsets
			offsets := []int{}
			for offset := range keyMsgs {
				if offset >= requestedOffset {
					offsets = append(offsets, offset)
				}
			}

			sort.Ints(offsets)

			// 把存在消息的偏移量加进msgs中
			for _, offset := range offsets {
				if msg, ok := keyMsgs[offset]; ok {
					msgs[key] = append(msgs[key], []int{offset, msg})
				}
			}
		}

		return n.Reply(msg, PollOkBody{
			Type: "poll_ok",
			Msgs: msgs,
		})
	})

	// 处理commit_offsets类型的消息。这个函数将指定偏移量之前的消息从未提交日志移动到已提交日志
	n.Handle("commit_offsets", func(msg maelstrom.Message) error {
		var body CommitOffsetsBody

		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		uncommitted.mutex.RLock()
		defer uncommitted.mutex.RUnlock()

		committed.mutex.Lock()
		defer committed.mutex.Unlock()

		for key, requestedOffset := range body.Offsets {
			uncommittedKeyMsgs, ok := uncommitted.msgs[key]
			if !ok {
				continue
			}

			for offset, uncommittedMsg := range uncommittedKeyMsgs {
				// 只提交请求偏移量之前的消息
				if offset > requestedOffset {
					continue
				}

				// 更新committed.msgs和committed.offsets字典中的偏移量。
				if _, ok := committed.msgs[key]; !ok {
					committed.msgs[key] = make(map[int]int)
				}
				committed.msgs[key][offset] = uncommittedMsg
			}
			committed.offsets[key] = requestedOffset
		}

		return n.Reply(msg, CommitOffsetsOkBody{
			Type: "commit_offsets_ok",
		})
	})

	// 处理list_committed_offsets类型的消息。这个函数返回已提交日志的偏移量映射
	n.Handle("list_committed_offsets", func(msg maelstrom.Message) error {
		var body ListCommittedOffsetsBody

		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		offsets := make(map[string]int)

		committed.mutex.RLock()
		defer committed.mutex.RUnlock()

		for _, key := range body.Keys {
			if offset, ok := committed.offsets[key]; ok {
				offsets[key] = offset
			}
		}

		return n.Reply(msg, ListCommittedOffsetsOkBody{
			Type:    "list_committed_offsets_ok",
			Offsets: offsets,
		})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
