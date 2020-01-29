package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/gorilla/websocket"
	"github.com/jdxcode/netrc"
	"golang.org/x/oauth2"
)

var (
	version       = "0.2.0"
	proxyEndpoint = "https://github-events-proxy.herokuapp.com"

	reRepoHTTP  = regexp.MustCompile(`^https?://.*github.com.*/(.+)/(.+?)(?:.git)?$`)
	reRepoSSH   = regexp.MustCompile(`github.com[:/](.+)/(.+).git$`)
	reEventHook = regexp.MustCompile(proxyEndpoint + `/(.*)`)
)

// Message contains github event data
type Message struct {
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

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

func forwardMessage(url string, message Message) {
	body := bytes.NewReader(message.Payload)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		log.Println("Request setup error:", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Delivery", "1")
	req.Header.Set("X-Github-Event", message.Event)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Request error:", err)
		return
	}
	ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	log.Println("Forwarded response:", resp.StatusCode)
}

func startServer() {
	addr := getListenAddr("PORT", "5000")
	server := newServer()

	log.Println("Starting server on", addr)
	if err := server.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func main() {
	var runServer bool
	var repoName string
	var filterType string
	var pretty bool
	var saveFiles bool
	var endpoint string
	var forwardURL string

	flag.BoolVar(&runServer, "server", false, "Start server")
	flag.StringVar(&repoName, "repo", "", "Repository name (namespace/repo)")
	flag.StringVar(&filterType, "only", "", "Filter events by type")
	flag.BoolVar(&pretty, "pretty", false, "Pretty print JSON")
	flag.BoolVar(&saveFiles, "save", false, "Save each event into separate file")
	flag.StringVar(&endpoint, "endpoint", "", "Set custom server endpoint")
	flag.StringVar(&forwardURL, "forward", "", "URL to forward events to")
	flag.Parse()

	if runServer {
		startServer()
		return
	}

	if endpoint != "" {
		proxyEndpoint = endpoint
		reEventHook = regexp.MustCompile(proxyEndpoint + `/(.*)`)
	}

	log.Println("Configuring Github API client")
	client, err := githubClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	var owner, repo string
	if repoName == "" {
		log.Println("Inspecting git remote")
		owner, repo, err = getRepo("origin")
		if err != nil {
			log.Fatal(err)
		}
	} else {
		chunks := strings.SplitN(repoName, "/", 2)
		if len(chunks) < 2 {
			log.Fatal("Invalid repo name")
		}
		owner = chunks[0]
		repo = chunks[1]
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

	log.Println("Listening to events")
	go func() {
		var message Message

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

			if filterType != "" && message.Event != filterType {
				log.Println("Skipped:", message.Event)
				continue
			}

			if pretty {
				newdata, err := json.MarshalIndent(message, "", "  ")
				if err == nil {
					data = newdata
				}
			}

			fmt.Printf("%s", data)

			if saveFiles {
				path := fmt.Sprintf("%v.%s.json", time.Now().UnixNano(), message.Event)
				if err := ioutil.WriteFile(path, data, 0666); err != nil {
					log.Println("File save error:", err)
				}
			}

			if forwardURL != "" {
				go forwardMessage(forwardURL, message)
			}
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c
}
