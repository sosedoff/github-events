package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jdxcode/netrc"

	"github.com/google/go-github/github"
	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

var (
	proxyEndpoint = "https://github-events-proxy.herokuapp.com"

	reRepoHTTP  = regexp.MustCompile(`^https?://.*github.com.*/(.+)/(.+?)(?:.git)?$`)
	reRepoSSH   = regexp.MustCompile(`github.com[:/](.+)/(.+).git$`)
	reEventHook = regexp.MustCompile(proxyEndpoint + `/(.*)`)
)

// randomHex returns a random hex string
func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// getRepo returns a owner, repo and an error
func getRepo(remote string) (string, string, error) {
	outBuf := bytes.NewBuffer(nil)
	errBuf := bytes.NewBuffer(nil)

	cmd := exec.Command("git", "remote", "get-url", remote)
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf

	if err := cmd.Run(); err != nil {
		return "", "", err
	}

	output := strings.TrimSpace(outBuf.String())

	matches := reRepoSSH.FindAllStringSubmatch(output, 1)
	if len(matches) > 0 {
		return matches[0][1], matches[0][2], nil
	}

	matches = reRepoHTTP.FindAllStringSubmatch(output, 1)
	if len(matches) > 0 {
		return matches[0][1], matches[0][2], nil
	}

	return "", "", errors.New("Git remote does not belong to Github")
}

func githubClientFromEnv() (*github.Client, error) {
	token := os.Getenv("GITHUB_TOKEN")

	if token == "" {
		path := filepath.Join(os.Getenv("HOME"), ".netrc")

		rc, err := netrc.Parse(path)
		if err != nil {
			return nil, err
		}

		machine := rc.Machine("api.github.com")
		if machine != nil {
			token = machine.Get("password")
		}
	}

	if token == "" {
		return nil, errors.New("Github API token is not set")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)

	return github.NewClient(tc), nil
}

func startWebsocketPing(conn *websocket.Conn, done chan bool) {
	for {
		select {
		case <-done:
			return
		case <-time.Tick(time.Second * 5):
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("Websocket ping error:", err)
			}
		}
	}
}

func main() {
	log.Println("Configuring Github API client")
	client, err := githubClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Inspecting git remote")
	owner, repo, err := getRepo("origin")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Fetching existing webhooks")
	hooks, _, err := client.Repositories.ListHooks(context.Background(), owner, repo, &github.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, hook := range hooks {
		if url, ok := hook.Config["url"].(string); ok {
			if reEventHook.MatchString(url) {
				log.Println("Removing the existing webhook:", *hook.ID)
				_, err := client.Repositories.DeleteHook(context.Background(), owner, repo, *hook.ID)
				if err != nil {
					log.Fatal(err)
				}
				log.Println("Existing webhook has been removed")
			}
		}
	}

	log.Println("Generating a new key")
	key, err := randomHex(20)
	if err != nil {
		log.Fatal(err)
	}

	hookURL := fmt.Sprintf("%s/%s", proxyEndpoint, key)

	hook := github.Hook{}
	hook.Events = []string{"*"}
	hook.Config = map[string]interface{}{
		"url":          hookURL,
		"content_type": "json",
	}

	log.Println("Creating a new webhook")
	newhook, _, err := client.Repositories.CreateHook(context.Background(), owner, repo, &hook)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		log.Println("Removing the webhook")
		_, err := client.Repositories.DeleteHook(context.Background(), owner, repo, *newhook.ID)
		if err != nil {
			log.Println("Failed to remove hook:", err)
		}
	}()

	log.Println("Listening to events")

	wsURL := strings.Replace(hookURL, "https:", "wss:", 1)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	pingClose := make(chan bool)
	defer func() {
		pingClose <- true
	}()
	go startWebsocketPing(conn, pingClose)

	go func() {
		var message = struct {
			Event string `json:"event"`
		}{}

		for {
			mtype, data, err := conn.ReadMessage()
			if err != nil {
				log.Println("Websocket read error:", err)
				break
			}
			if mtype != websocket.TextMessage {
				continue
			}

			if err := json.Unmarshal(data, &message); err != nil {
				log.Println("JSON error:", err)
				continue
			}

			log.Println("Received event:", message.Event)
			fmt.Printf("%s\n", data)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c
}
