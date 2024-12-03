package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const (
	srcPath = "test-video.avi"
	dstPath = "copied-video.avi"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1_000_000,
	WriteBufferSize: 1_000_000,
}

func client() error {
	u := url.URL{Scheme: "ws", Host: "localhost:3000", Path: "/ws"}
	for range 10 {
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			time.Sleep(time.Second * 1)
			continue
		}
		defer c.Close()

		mt, p, err := c.ReadMessage()
		if err != nil {
			return err
		}

		if mt != websocket.TextMessage {
			return fmt.Errorf("invalid message type from server: %d", mt)
		}

		path := string(p)
		f, err := os.Open(path)
		if err != nil {
			return err
		}

		w, err := c.NextWriter(websocket.BinaryMessage)
		if err != nil {
			return err
		}

		if _, err := io.Copy(w, f); err != nil {
			return err
		}

		if err := c.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
			return err
		}

		return nil
	}
	return errors.New("failed to do things after 10 attempts")
}

func server(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not upgrade: %s", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("failed to close websocket: %s", err)
		}
	}()

	err = conn.WriteMessage(websocket.TextMessage, []byte(srcPath))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to write request: %s", err), http.StatusInternalServerError)
	}

	if err := reader(conn); err != nil {
		log.Printf("failed to read from client: %s", err)
		return
	}
}

func reader(conn *websocket.Conn) error {
	mt, r, err := conn.NextReader()
	if err != nil {
		return err
	}

	if mt != websocket.BinaryMessage {
		return fmt.Errorf("invalid message type received from client: %d", mt)
	}

	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return nil
}

func main() {
	go client()

	http.HandleFunc("/ws", server)
	log.Fatal(http.ListenAndServe(":3000", nil))
}
