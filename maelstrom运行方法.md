# maelstrom运行方法

[官网](https://fly.io/dist-sys/)

以第一个实验[Echo](https://fly.io/dist-sys/1/)为例。

## 步骤

第一步：在本地创建对应实验的文件夹，并且初始化一个go.mod文件。

```bash
mkdir maelstrom-echo
cd maelstrom-echo
go mod init maelstrom-echo
go mod tidy
```

其中最后一步在运行之后可能会报错，可以忽略，不影响后续步骤进行。

<img width="423" alt="截屏2024-02-27 13 37 41" src="https://github.com/yjcsivsuk/goosip-glomers/assets/96940537/edc40369-fc82-431f-993d-4e94c3563297">

第二步：在刚刚创建的maelstorm-echo文件夹中，新建main.go文件，代码内容就是实验内容。

```go
package main

import (
	"encoding/json"
	"log"
	"os"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()

	// Register a handler for the "echo" message that responds with an "echo_ok".
	n.Handle("echo", func(msg maelstrom.Message) error {
		// Unmarshal the message body as an loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type.
		body["type"] = "echo_ok"

		// Echo the original message back with the updated message type.
		return n.Reply(msg, body)
	})

	// Execute the node's message loop. This will run until STDIN is closed.
	if err := n.Run(); err != nil {
		log.Printf("ERROR: %s", err)
		os.Exit(1)
	}
}

```

第三步：为了编译程序，需要获取maelstrom在github上的包并安装。

```bash
go get github.com/jepsen-io/maelstrom/demo/go
go install .
```

这一步会在本地的~/go/bin（$GOPATH）目录下新建一个maelstrom-echo二进制文件。

第四步：本地电脑安装maelstrom运行所需库。

```bash
brew install openjdk graphviz gnuplot
```

更多所需的库，详情点击https://github.com/jepsen-io/maelstrom/blob/main/doc/01-getting-ready/index.md#prerequisites

并且需要安装官方的[maelstrom库](https://github.com/jepsen-io/maelstrom/releases/tag/v0.2.3)。下载github中最新的release包并解压。解压到和刚才所创建的maelstrom-echo同一个目录下。

第五步：运行maelstrom-echo。先将路径切换到刚刚解压的maelstrom文件夹下，再运行maelstorm-echo二进制文件。

```bash
cd ./maelstrom
./maelstrom test -w echo --bin ~/go/bin/maelstrom-echo --node-count 1 --time-limit 10
```

直到出现下面消息，说明实验成功。

```markdown
Everything looks good! ヽ(‘ー`)ノ
```

以下是运行期间的部分截图：

<img width="1065" alt="截屏2024-03-02 13 27 15" src="https://github.com/yjcsivsuk/goosip-glomers/assets/96940537/f61f690b-a4f2-4198-996e-8e40695a0aa5">

<img width="724" alt="截屏2024-02-27 16 25 36" src="https://github.com/yjcsivsuk/goosip-glomers/assets/96940537/16865757-6ffc-40e0-8308-9433dc4d1410">

第六步（可选）：可以在浏览器中查看运行记录及日志。

```bash
./maelstrom serve
```

出现以下消息

<img width="581" alt="截屏2024-02-27 16 29 24" src="https://github.com/yjcsivsuk/goosip-glomers/assets/96940537/67b73860-2658-4acc-968d-dc158e33d9f6">

之后在浏览器中打开http://localhost:8080/

<img width="599" alt="截屏2024-02-27 16 29 43" src="https://github.com/yjcsivsuk/goosip-glomers/assets/96940537/a16f81f1-4b73-4138-96fc-7a5c30146956">

可以在这里查看更详细的运行情况。
