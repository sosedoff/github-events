# github-events

Utility to listen to Github events for a given repository

## Overview

`github-events` utility will create a temporary webhook for your Github repository
and print out all event payloads as the come in the real time. Once the process is
stopped the temporary webhook is destroyed. 

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