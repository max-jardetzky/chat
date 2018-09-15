package main

import (
	"bufio"
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
	"sync"
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

// SafeClientList binds clients with a mutex structure to prevent race conditions.
type SafeClientList struct {
	mutex   sync.Mutex
	clients []*Client
}

var srv *http.Server
var clientList SafeClientList
var f *os.File
var mutedIPs map[string]bool
var err error
var inShutdown bool
var printDisabled bool

func main() {
	rand.Seed(time.Now().UnixNano())
	srv = &http.Server{Addr: ":80"}
	clientList = SafeClientList{
		clients: make([]*Client, 0),
	}
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
	go launchCLI()

	http.HandleFunc("/chat", launchHTTP)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, getUsers(false))
	})
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	srv.ListenAndServe()
}

func launchHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err == nil {
		client := &Client{conn, "", "", 0}
		clientList.mutex.Lock()
		clientList.clients = append(clientList.clients, client)
		clientList.mutex.Unlock()

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
				if err != nil {
					fmt.Println(ip, err)
				}
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
}

func launchCLI() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch scanner.Text() {
		case "help":
			fmt.Println("Commands: say, users, mute, unmute, tmp, delprevlogs, shutdown")
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
		case "tmp":
			printDisabled = !printDisabled
			fmt.Println("printing disabled:", printDisabled)
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
		fmt.Println(err)
	}
}

func sendClient(client *Client, msg string) {
	msg = "[" + time.Now().String()[11:19] + "] " + msg
	if err := client.conn.WriteMessage(1, []byte(msg)); err != nil {
		fmt.Println(err)
		return
	}
}

func sendAll(msg string) {
	defer clientList.mutex.Unlock()
	clientList.mutex.Lock()
	for _, v := range clientList.clients {
		sendClient(v, msg)
	}
}

func sendIP(ip, msg string) {
	defer clientList.mutex.Unlock()
	clientList.mutex.Lock()
	for _, v := range clientList.clients {
		if v.ip == ip {
			sendClient(v, msg)
		}
	}
}

func validIP(ip string) bool {
	clientList.mutex.Lock()
	defer clientList.mutex.Unlock()
	isValid := false
	for _, v := range clientList.clients {
		if v.ip == ip {
			isValid = true
		}
	}
	return isValid
}

func log(msg string) {
	logString := "[" + time.Now().String()[11:19] + "] " + msg + "\r\n"
	if !printDisabled {
		fmt.Print(logString)
	}
	_, err = f.WriteString(logString)
	if err != nil {
		fmt.Println(err)
	}
}

func shutdown(reason string) {
	inShutdown = true
	shutdownString := "(SERVER) Shutdown by " + reason + "."
	log(shutdownString)
	sendAll(shutdownString)
	defer clientList.mutex.Unlock()
	clientList.mutex.Lock()
	for _, v := range clientList.clients {
		if err = v.conn.Close(); err != nil {
			fmt.Println(err)
		}
	}

	if err = srv.Shutdown(nil); err != nil {
		check(err)
	}
}

func delete(client *Client) {
	defer clientList.mutex.Unlock()
	clientList.mutex.Lock()
	for i, v := range clientList.clients {
		if reflect.DeepEqual(client, v) {
			clientList.clients = append(clientList.clients[:i], clientList.clients[i+1:]...)
			break
		}
	}
}

func getUsers(admin bool) string {
	var userList string
	defer clientList.mutex.Unlock()
	clientList.mutex.Lock()
	if len(clientList.clients) == 0 {
		userList = "No connected users."
	} else {
		userList = "Connected users (" + strconv.Itoa(len(clientList.clients)) + "): "
		for _, v := range clientList.clients {
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
	clientList.mutex.Lock()
	numClients := len(clientList.clients)
	clientList.mutex.Unlock()
	if numClients > 0 {
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
