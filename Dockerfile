FROM golang:1.26 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /tasks-mcp .

FROM gcr.io/distroless/static-debian12

COPY --from=build /tasks-mcp /tasks-mcp

VOLUME /data

ENV TASKS_MCP_DB_PATH=/data/tasks.db

ENTRYPOINT ["/tasks-mcp"]
