# github-events

Utility to listen to Github events for a given repository

## Overview

`github-events` utility will create a temporary webhook for your Github repository
and print out all event payloads as the come in the real time. Once the process is
stopped the temporary webhook is destroyed. 

## Installation

If you have Go installed locally, run:

```
go get github.com/sosedoff/github-events
```

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
Usage of github-events:
  -only string
    	Filter events by type
  -pretty
    	Pretty print JSON
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
```