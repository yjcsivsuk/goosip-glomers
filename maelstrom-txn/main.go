package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {

	n := maelstrom.NewNode()
	store := NewSimpleTxnStore()

	n.Handle("txn", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		txn := body["txn"].([]interface{})
		res := store.PerformTxn(txn)  // 使用store实例的PerformTxn方法执行事务
		body["in_reply_to"] = body["msg_id"]
		body["txn"] = res
		body["type"] = "txn_ok"
		return n.Reply(msg, body)
	})

	if err := n.Run(); err != nil {
		log.Printf("ERROR: %s", err)
		os.Exit(1)
	}
}

// 定义了一个简单的事务存储，它包含了一个互斥锁和一个存储事务结果的map
type SimpleTxnStore struct {
	store map[float64]float64
	mu    sync.Mutex
}

// 用于创建一个新的SimpleTxnStore实例
func NewSimpleTxnStore() *SimpleTxnStore {
	return &SimpleTxnStore{store: make(map[float64]float64)}
}

// 这个方法接受一个事务操作列表，并执行每个操作。如果操作是读取，它返回当前存储中的值；如果操作是写入，它更新存储中的值
func (s *SimpleTxnStore) PerformTxn(txn []interface{}) [][]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	var res [][]interface{}

	for _, opr := range txn {
		op := opr.([]any)  // 解析操作
		t := op[0].(string)  // 判断操作的类型是写还是读

		var key float64
		var val float64

		if t == "r" {  // 读的话就获取key对应的value
			key = op[1].(float64)
			val = s.store[key]
		} else {  // 写的话就把新的value存到对应的key中
			key = op[1].(float64)
			val = op[2].(float64)
			s.store[key] = val
		}
		res = append(res, []interface{}{t, key, val})  // 把执行的结果存到res切片中，之后作为事务返回
	}
	return res
}
