package main

import (
	"fmt"
	"net"
	"strings"
)

var (
	help string = `--------------------
!exit        to exit
!list        to list users
!rename NAME to change username
!mute USER   to mute/unmute USER
!muted       to list muted users
!help        for this help message
--------------------

`
	addr  string           = ":8003"
	users map[string]*User = map[string]*User{}
)

const MSG_MAX_SIZE = 1024

func inverse(s string) string {
	return fmt.Sprintf("\x1b[7m%s\x1b[0m", s)
}

func bold(s string) string {
	return fmt.Sprintf("\x1b[1m%s\x1b[0m", s)
}

func eraseLastLine(user *User) {
	user.WriteString("\x1b[1A\x1b[2K")
}

type User struct {
	conn  net.Conn
	name  string
	muted map[*User]bool
}

func (u *User) WriteString(p string) (n int, err error) {
	return u.Write([]byte(p))
}

func (u *User) Read(p []byte) (n int, err error) {
	return u.conn.Read(p)
}

func (u *User) Write(p []byte) (n int, err error) {
	return u.conn.Write(p)
}

func (u *User) Close() {
	u.conn.Close()
}

func (u *User) Post(msg string) {
	for _, user := range users {
		if !user.muted[u] {
			fmt.Fprintf(user, "\x1b[s>> %s: %s\x1b[u\n", bold(u.name), msg)
		}
	}
}

func login(c net.Conn) *User {
	user := new(User)
	user.conn = c
	user.muted = make(map[*User]bool)
	for {
		user.WriteString("Username: ")
		bname := make([]byte, 32)
		n, _ := c.Read(bname)
		name := string(bname)[:n-1]
		name = strings.Replace(name, "\r", "", -1)
		if strings.ContainsAny(name, ": \n") {
			user.WriteString("invalid username.")
			continue
		}
		if users[name] != nil {
			user.WriteString(name + " is taken\n")
			continue
		}
		user.name = name
		users[name] = user
		break
	}
	user.WriteString(help)
	return user
}

func handle(c net.Conn) {
	user := login(c)
	go sysPost(fmt.Sprintf("%s has joined the chat", user.name))
	bmsg := make([]byte, MSG_MAX_SIZE+1)
	for {
		n, err := c.Read(bmsg)
		eraseLastLine(user)
		if err != nil {
			user.WriteString("An error has occured\n")
			user.Close()
		}
		msg := string(bmsg)[:n]
		msg = msg[:n-2]
		switch {
		case strings.Contains(msg, "\n"):
			user.WriteString("Message must not contain newline characters\n")
		case msg == "!help":
			user.WriteString(help)
		case msg == "!list":
			for otherUser, _ := range users {
				fmt.Fprintf(user, "%s %s\n", inverse("|"), otherUser)
			}
		case strings.HasPrefix(msg, "!mute"):
			splits := strings.Split(msg, " ")
			if len(splits) == 2 && users[splits[1]] != nil {
				if user.muted[users[splits[1]]] {
					delete(user.muted, users[splits[1]])
				}
				user.muted[users[splits[1]]] = true
				user.WriteString("User has been muted\n")
			} else {
				user.WriteString("Failed to mute user\n")
			}
		case msg == "!muted":
			for u, _ := range user.muted {
				fmt.Fprintf(user, "%s %s\n", inverse("|"), u.name)
			}
		case strings.HasPrefix(msg, "!rename"):
			splits := strings.Split(msg, " ")
			if len(splits) == 2 && users[splits[1]] == nil {
				name := user.name
				delete(users, name)
				user.name = splits[1]
				users[user.name] = user
				go sysPost(fmt.Sprintf("%s has changed name to %s", name, user.name))
			} else {
				user.WriteString("Failed to change name\n")
			}
		case msg == "!exit":
			user.Close()
			delete(users, user.name)
			sysPost(fmt.Sprintf("%s has left the chat", user.name))
			return
		case true:
			go user.Post(msg)
		}
	}
}

func sysPost(msg string) {
	for _, c := range users {
		go fmt.Fprintf(c, "%s %s\n", inverse("!!"), msg)
	}
}

func main() {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go handle(conn)
	}
}
