FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /autoscout24-mcp ./cmd/autoscout24-mcp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /autoscout24-mcp /autoscout24-mcp
ENTRYPOINT ["/autoscout24-mcp"]
