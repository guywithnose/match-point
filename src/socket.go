package main

import (
    "github.com/gorilla/websocket"
    "net/http"
    "time"
    "log"
)

var (
    upgrader = websocket.Upgrader{
        Subprotocols: []string{"match-point"},
        CheckOrigin: func(*http.Request) bool {return true;},
    }

    writeWait = 10 * time.Second
    pongWait = 60 * time.Second
    pingPeriod = (pongWait * 9) / 10
)

func Socket (handleMessage func (*Message, chan<- *Message, <-chan bool)) func(w http.ResponseWriter, r *http.Request) {
    return func(w http.ResponseWriter, r *http.Request) {
        ws, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            log.Println(err)
            return
        }

        defer ws.Close()

        ws.SetReadDeadline(time.Now().Add(pongWait))
        ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

        var sender chan *Message = make(chan *Message);
        var closed chan bool = make(chan bool);

        go handleOutput(ws, sender, closed)
        go ping(ws, closed)
        handleMessages(ws, sender, closed)
    }
}

func ping(ws *websocket.Conn, closed chan bool) {
    ticker := time.NewTicker(pingPeriod)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
                log.Println("ping:", err)
            }
        case <-closed:
            return
        }
    }
}

func handleMessages(ws *websocket.Conn, sender chan<- *Message, closed chan bool) {
    msg := new(Message)
    for {
        err := ws.ReadJSON(msg)
        if err != nil {
            break;
        }

        handleMessage(msg, sender, closed)
    }

    log.Println("Connection Closed")
    close(closed)
}

func handleOutput(ws *websocket.Conn, sender <-chan *Message, closed <-chan bool) {
    for {
        select {
        case msg := <-sender:
            ws.WriteJSON(msg)
        case <-closed:
            break;
        }
    }
}
