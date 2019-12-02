// Heroku config: https://github.com/heroku/heroku-buildpack-go
// +heroku goVersion go1.13
// +heroku install

module github.com/sosedoff/github-events

go 1.13

require (
	github.com/gin-gonic/gin v1.5.0
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/gorilla/websocket v1.4.1
	github.com/jdxcode/netrc v0.0.0-20190329161231-b36f1c51d91d
	golang.org/x/oauth2 v0.0.0-20191122200657-5d9234df094c
)
