package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

var ch = make(chan string, 100)
var errChan = make(chan error, 1)
var ctx context.Context
var cancelFunc context.CancelFunc

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleWsConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	defer ws.Close()
	go func() {
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				log.Println("Соединение закрыто:", err)
				return
			}
		}
	}()

	for {
		select {
		case err := <-errChan:
			ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("%v", err)))
			return
		case message := <-ch:
			if err := ws.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
				log.Println(err)
				return
			}
		}
	}
}

func wolHandler(w http.ResponseWriter, r *http.Request) {
	mac := ""
	if r.Method == http.MethodPost {
		r.ParseForm()
		mac = r.FormValue("mac-address")
	}
	if mac == "" {
		http.Error(w, "MAC-адрес не указан", http.StatusBadRequest)
		return
	}
	err := sendMagicPacket(mac)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка отправки пакета: %v", err), http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Компьютер с MAC-адресом %s должен проснуться", mac)
}

func formHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "form.html")
}

func scanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		adapterNumStr := r.FormValue("adapter-num")
		cidr := r.FormValue("cidr")

		if adapterNumStr == "" || cidr == "" {
			http.Error(w, "Параметры adapter-num и cidr обязательны", http.StatusBadRequest)
			return
		}

		adapterNum, err := strconv.Atoi(adapterNumStr)
		if err != nil {
			http.Error(w, fmt.Sprintf("Неверный формат adapter-num: %v", err), http.StatusBadRequest)
			return
		}

		if cancelFunc != nil {
			cancelFunc()
		}
		ctx, cancelFunc = context.WithCancel(context.Background())
		go func() {
			err = scan(ctx, cidr, adapterNum-1, ch)
			if err != nil {
				log.Println(err)
				errChan <- err
			}
		}()
	} else {
		tmpl := template.Must(template.ParseFiles("scannerForm.html"))
		listDevices := getListNetworkAdapters()
		data := struct {
			Items []string
		}{listDevices}
		err := tmpl.Execute(w, data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка при выполнении шаблона: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

func main() {
	http.HandleFunc("/wol", wolHandler)
	http.HandleFunc("/", formHandler)
	http.HandleFunc("/scan", scanHandler)
	http.HandleFunc("/ws", handleWsConnections)
	fmt.Println("Запуск сервера на порту :8989")
	err := http.ListenAndServe("192.168.1.66:8989", nil)
	if err != nil {
		log.Println(err)
	}
}
