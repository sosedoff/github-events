# github-events

Utility to listen to Github events for a given repository

## Overview

`github-events` utility will create a temporary webhook for your Github repository
and print out all event payloads as they come in the real time. Once the process is
stopped the temporary webhook is destroyed. This is super useful when testing Github
events as there's no need to setup anything (endpoints, webhook receivers, etc).

It works by creating a temporary webhook for the repository that points to an 
URL like `https://github-events-proxy.herokuapp.com/key`, where `key` is a random
token. Github will be sending all events to that URL moving forward. Then utility
connects to the given URL using Websocket prototol and receives all events in JSON
format. Server part is open, check `server.go` file.

## Installation

If you have Go installed locally, run:

```
go get github.com/sosedoff/github-events
```

Or visit [Releases](https://github.com/sosedoff/github-events/releases) page to grab a binary.

## Configuration

There are two ways how you can configure the `github-events`:

1. Environment variable

[Create a personal token](https://github.com/settings/tokens/new) first, then start
the process with:

```
GITHUB_TOKEN=... github-events
```

2. Netrc entry

Add a following record to the `~/.netrc` file:

```
machine api.github.com
  login YOUR_GITHUB_LOGIN
  password YOUR_GITHUB_PERSONAL_TOKEN
```

## Usage

```
Usage of ./github-events:
  -endpoint string
    	Set custom server endpoint
  -forward string
    	URL to forward events to
  -only string
    	Filter events by type
  -pretty
    	Pretty print JSON
  -repo string
    	Repository name (namespace/repo)
  -save
    	Save each event into separate file
```

Some of the use cases:

```bash
# Pipe to jq for pretty printing and colorization
github-events | jq

# Or use internal pretty print option
github-events -pretty

# Save to file
github-events > events.log

# Filter by event type
github-events -only=push

# Save each event to a file.
# They are still printed out to STDOUT.
github-events -save -pretty
```

You can also forward event data to a local HTTP endpoint:

```bash
# lets say you have an app running on localhost:5000
# forward requests by running this command
github-events -forward http://locahost:5000/events
```

While the event proxy server is hosted on Heroku, you can run the server locally:

```bash
github-events server
```

To watch events from a repository that's not cloned in the same directory:

```bash
github-events -repo yourname/reponame
```