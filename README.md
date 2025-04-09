# Epic Games Free Games API

A Go-based API server for fetching free games from the Epic Games Store.

## Overview

This project provides a simple API to get information about free games available on the Epic Games Store. It uses Epic Games' GraphQL API to fetch the data and returns it in JSON format.

## Features

- RESTful API for fetching free games information
- Includes both currently free games and upcoming free games (configurable)
- Returns detailed information including:
  - Game title
  - Description
  - Publisher
  - Image URL
  - Game URL
  - Status (free or coming soon)
  - Start and end dates of the promotion
- Support for different country stores and locales
- CORS enabled for front-end integration

## Requirements

- Go 1.18 or higher

## Installation

1. Clone the repository:

   ```
   git clone https://github.com/yourusername/epic-games-api.git
   cd epic-games-api
   ```

2. Install dependencies:
   ```
   go mod download
   ```

## Running the API Server

Start the API server on the default port (8080):

```
go run main.go
```

To use a different port:

```
go run main.go -port 3000
```

## API Documentation

The API server includes a simple documentation page at the root URL (`/`).

### Endpoints

#### GET /api/free-games

Returns information about free games from the Epic Games Store.

##### Query Parameters

| Parameter  | Description                              | Default |
| ---------- | ---------------------------------------- | ------- |
| `upcoming` | Include upcoming free games (true/false) | `true`  |
| `country`  | Country code for the store               | `US`    |
| `locale`   | Locale for text formatting               | `en-US` |

##### Example Requests

Get all free games (current and upcoming):

```
GET /api/free-games
```

Get only currently free games (exclude upcoming):

```
GET /api/free-games?upcoming=false
```

Get free games for the UK store:

```
GET /api/free-games?country=GB&locale=en-GB
```

##### Example Response

```json
{
  "success": true,
  "count": 1,
  "data": [
    {
      "title": "Cat Quest II",
      "description": "Open-world action-RPG in a fantasy realm of cats and dogs. CAT QUEST II lets you play solo or with a friend, as both a cat and dog! Quest in a world filled with magic, defeat monsters and collect loot in a catventure like never before!",
      "image_url": "https://cdn1.epicgames.com/spt-assets/fe812f94c42e44e986691a84c796952d/cat-quest-ii-13gb6.jpg",
      "url": "https://store.epicgames.com/en-US/p/17c196bb2302467d9c930289a0b70562",
      "status": "free",
      "start_date": "2025-04-04T15:00:00.000Z",
      "end_date": "2025-04-11T15:00:00.000Z",
      "publisher": "Kepler Interactive"
    }
  ]
}
```

## Building and Deploying

To build an executable:

```
go build -o epic-games-api
```

Then run it:

```
./epic-games-api -port 8080
```

### Docker Deployment (Optional)

Create a Dockerfile:

```Dockerfile
FROM golang:1.18-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o epic-games-api

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/epic-games-api .
EXPOSE 8080
ENTRYPOINT ["./epic-games-api"]
```

Build and run the Docker container:

```
docker build -t epic-games-api .
docker run -p 8080:8080 epic-games-api
```

## License

MIT
