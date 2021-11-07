# rabbitmq-golang-client

rabbitmq-golang-client は、RabbitMQ に接続し、メッセージを送受信するためのシンプルな、Golang ランタイム のための ライブラリです。  

## 動作環境

* OS: Linux
* CPU: ARM/AMD/Intel
* Golang Runtime

## 導入方法

go get でインストールしてください。

```sh
go get "github.com/latonaio/rabbitmq-golang-client"
```


## 使用方法

### ライブラリの初期化

import 文を追加します。

```go
import rabbitmq "github.com/latonaio/rabbitmq-golang-client"
```

`rabbitmq.NewRabbitmqClient("<URL>", []string{"<受信するキュー名>"...}, []string{"<送信するキュー名>"...})` でクライアントを作成します。

指定するキューは事前に存在している必要があります。存在しない場合は例外が発生します。

例:

```go
client, err := rabbitmq.NewRabbitmqClient(
	"amqp://username:password@hostname:5672/virtualhost",
	[]string{"queue_from"},
	[]string{"queue_to"}
)
if err != nil {
	// エラー
	log.Println("ERROR")
	return
}

// 受信を終了する
defer client.Close()
```


### キューからメッセージを受信

次のようなループでメッセージを処理します。

メッセージの処理が終わったあと、必ず結果を通知するメソッド (`message.Success()`, `message.Fail()` または `message.Requeue()`) をコールしてください。`Success()` の場合はキューからそのメッセージが正常に削除され、`Fail()` の場合はそのメッセージがデッドレターに送られます (設定されている場合) 。

(何らかの理由で再度メッセージをキューに戻したいときは、`message.Requeue()` をコールしてください。)

`message.QueueName()` で受け取り元キューの名前が、`message.Data()` に受信したデータを受け取れます。

例:

```go
iter, err := client.Iterator()
if err != nil {
	// エラー
	log.Println("ERROR")
	return
}
// 受信を終了する際には Stop を呼ぶ
defer client.Stop()

for message := range iter {
	// 何らかの理由でメッセージを後から再処理したいとき等、
	// 再度キューに戻すときは、message.Requeue() を実行する

	// 何らかの処理
	log.Println("received from: %v", message.QueueName())
	log.Println("data: %v", message.Data())

	// 処理成功
	message.Success()

	// 処理失敗時
	// メッセージがデッドレターという別のキューに入る (定義されている場合)
	// message.Fail()
}
```


### メッセージを送信する

`client.Send("<送信先キュー名>", <データ>)` のように呼び出してください。`<データ>` には `map[string]interface{}` を渡します。

例:

```go
payload := map[string]interface{}{
	"hello": "world"
}
if err := client.Send("queue_to", payload); err != nil {
	log.Printf("error: %v", err)
}
``` 
