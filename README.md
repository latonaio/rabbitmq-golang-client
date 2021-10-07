# rabbitmq-golang-client   
## 概要   

RabbitMQ に接続し、メッセージを送受信するためのシンプルな golang 用クライアントライブラリです。   
 

### ライブラリの初期化   
RabbitmqClient を import します。   

``` 
import "git+https://github.com/latonaio/rabbitmq-golang-client/client"

_, cancel := context.WithCancel(context.Background())
quiteCh := make(chan syscall.Signal, 1)
errCh := make(chan error, 1)

newClient, err := NewRabbitmqClient(URL, QUEUE_FROM, QUEUE_TO)
if err != nil {
    errCh <- err
}
defer newClient.Close()


for {
    select {
        case err := <-errCh:
            log.Print(err)
        case <-quiteCh:
            cancel()
    }
}
``` 

### キューからメッセージを受信　　　  　  

```   
go func() {   
    for m := range newClient.Iterator("queue_from") {   
        // map[string]interface{} に変換することで使いやすくする   
        json, err := m.DecodeJSON()   
        if err != nil {   
            // エラー時に呼ぶ   
            m.Fail() // もしくは m.Requeue() 
            errCh <- err   
            continue   
        }   

        // 何らかの処理   
        fmt.Printf("hoge %v\n", json["hoge"])   

        // 成功時   
        m.Success()   
    }   
}   
```   

### メッセージを送信　　　
　　　　　
```   
payload := map[string]interface{}{
    "Hello": "Golang",
}
err := newClient.Send("queue_to", payload)
if err != nil {
    errCh <- err
}
``` 
