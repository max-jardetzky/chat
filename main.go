package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client records a client connecting to the server.
type Client struct {
	conn     *websocket.Conn
	name     string
	ip       string
	msgCount int
}

var srv *http.Server
var clientList []*Client
var f *os.File
var mutedIPs map[string]bool
var err error
var inShutdown bool

func main() {
	rand.Seed(time.Now().UnixNano())
	srv = &http.Server{Addr: ":80"}
	clientList = make([]*Client, 0)
	mutedIPs = map[string]bool{}
	initTimeStr := strings.Replace(time.Now().String()[:19], ":", "-", -1)
	fmt.Println("Server startup process initiated. Type `help` for help.")

	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		os.Mkdir("logs", 0755)
	}
	f, err = os.Create(filepath.Join("logs", initTimeStr+".txt"))
	check(err)

	host, _ := os.Hostname()
	addrs, _ := net.LookupIP(host)
	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			log("Listening on: " + ipv4.String())
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			shutdown(sig.String())
		}
	}()

	// Admin commands from console
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			switch scanner.Text() {
			case "help":
				fmt.Println("Commands: say, users, mute, unmute, delprevlogs, shutdown")
			case "say":
				fmt.Print("Enter message: ")
				scanner.Scan()
				log("SERVER: " + scanner.Text())
				sendAll("SERVER: " + scanner.Text())
			case "users":
				fmt.Println(getUsers(true))
			case "mute":
				mute(true)
			case "unmute":
				mute(false)
			case "delprevlogs":
				dirRead, err := os.Open("logs")
				check(err)
				files, err := dirRead.Readdir(0)
				check(err)
				for i := 0; i < len(files)-1; i++ {
					err = os.Remove(filepath.Join("logs", files[i].Name()))
					check(err)
				}
				log("Deleted previous logs.")
			case "shutdown":
				for {
					fmt.Print("Are you sure? (y/n) ")
					scanner.Scan()
					if scanner.Text() == "y" {
						shutdown("admin command")
						break
					}
					if scanner.Text() == "n" {
						break
					}
				}
			default:
				fmt.Println("invalid command", scanner.Text())
			}
		}

		if scanner.Err() != nil {
			check(err)
		}
	}()

	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			client := &Client{conn, "", "", 0}
			clientList = append(clientList, client)

			defer func() {
				leavingMsg := client.name + " left."
				delete(client)
				if !inShutdown {
					log(leavingMsg)
					sendAll(leavingMsg)
				}
			}()

			for {
				// Read message from browser
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}

				// First login, set *Client values, and connect message.
				if client.msgCount == 0 {
					client.name = string(msg)

					ip, _, err := net.SplitHostPort(r.RemoteAddr)
					check(err)
					client.ip = ip

					sendClient(client, "(SERVER) Welcome! Type `/help` for a list of commands.")

					connectMsg := " connected."
					if val, ok := mutedIPs[client.ip]; val && ok {
						connectMsg += " (MUTED)"
					}
					log(client.name + " (" + client.ip + ")" + connectMsg)
					sendAll(client.name + connectMsg)
				} else { // Any subsequent incoming messages and mute handling
					if val, ok := mutedIPs[client.ip]; val && ok {
						sendIP(client.ip, "(SERVER) Muted.")
					} else {
						totalMsg := " says: " + string(msg)
						log(client.name + " (" + client.ip + ")" + totalMsg)
						sendAll(client.name + totalMsg)
					}
				}
				client.msgCount++
			}
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, getUsers(false))
	})
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	srv.ListenAndServe()
}

func sendIP(ip, msg string) {
	msg = "[" + time.Now().String()[11:19] + "] " + msg
	for _, v := range clientList {
		if v.ip == ip {
			if err := v.conn.WriteMessage(1, []byte(msg)); err != nil {
				check(err)
				return
			}
		}
	}
}

func sendClient(client *Client, msg string) {
	msg = "[" + time.Now().String()[11:19] + "] " + msg
	if err := client.conn.WriteMessage(1, []byte(msg)); err != nil {
		check(err)
		return
	}
}

func sendAll(msg string) {
	msg = "[" + time.Now().String()[11:19] + "] " + msg
	for _, v := range clientList {
		if err := v.conn.WriteMessage(1, []byte(msg)); err != nil {
			check(err)
			return
		}
	}
}

func validIP(ip string) bool {
	isValid := false
	for _, v := range clientList {
		if v.ip == ip {
			isValid = true
		}
	}
	return isValid
}

func log(msg string) {
	logString := "[" + time.Now().String()[11:19] + "] " + msg + "\r\n"
	fmt.Print(logString)
	_, err = f.WriteString(logString)
	if err != nil {
		fmt.Println(err)
	}
}

func shutdown(reason string) {
	inShutdown = true
	shutdownString := "(SERVER) Shutdown by " + reason + "."
	log(shutdownString)
	for _, v := range clientList {
		sendIP(v.ip, shutdownString)
		v.conn.Close()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.Shutdown(ctx)
}

func delete(client *Client) {
	for i, v := range clientList {
		if reflect.DeepEqual(client, v) {
			clientList = append(clientList[:i], clientList[i+1:]...)
			break
		}
	}
}

func getUsers(admin bool) string {
	var userList string
	if len(clientList) == 0 {
		userList = "No connected users."
	} else {
		userList = "Connected users (" + strconv.Itoa(len(clientList)) + "): "
		for _, v := range clientList {
			userList += v.name
			if admin {
				userList += " (" + v.ip
				if val, ok := mutedIPs[v.ip]; val && ok {
					userList += ", MUTED"
				}
				userList += ")"
			}
			userList += ", "
		}
		userList = userList[:len(userList)-2]
	}
	return userList
}

func mute(mute bool) {
	scanner := bufio.NewScanner(os.Stdin)
	if len(clientList) > 0 {
		var ip string
		for {
			fmt.Print("Enter IP to mute (`users` for list, `cancel` to cancel): ")
			scanner.Scan()
			if scanner.Text() == "users" {
				fmt.Println(getUsers(true))
			}
			if scanner.Text() == "cancel" {
				break
			}
			if validIP(scanner.Text()) {
				ip = scanner.Text()
				break
			}
		}
		if ip != "" {
			mutedIPs[ip] = mute
			if mute {
				sendIP(ip, "(SERVER) Muted.")
				log(ip + " muted.")
			} else {
				sendIP(ip, "(SERVER) Unmuted.")
				log(ip + " unmuted.")
			}
		}
	} else {
		fmt.Println("No users available to mute/unmute.")
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
