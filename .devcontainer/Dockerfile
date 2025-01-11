FROM golang:1.23.4-bookworm AS golang
FROM debian:bookworm

ENV DEBIAN_FRONTEND="noninteractive"
ENV GOBIN="/usr/local/gobin"
ENV CGO_ENABLED=1
ENV PATH="$PATH:/usr/local/go/bin:/usr/local/gobin"
RUN mkdir -p /usr/local/gobin

# Copy the golang binaries
COPY --from=golang /usr/local/go /usr/local/go

# Install system dependencies
RUN apt-get update && \
    apt-get install -y gcc wget git sqlite3 && \
    rm -rf /var/lib/apt/lists/*

# Install dependencies with go install
RUN go install github.com/go-task/task/v3/cmd/task@v3.40.1 && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.63.4

# Add the startup script on every bash session
COPY ./startup.sh /usr/local/bin/startup.sh
RUN echo "\n\n" >> /root/.bashrc && \
    cat /usr/local/bin/startup.sh >> /root/.bashrc

# Sleep infinity to keep the container running
CMD ["sleep", "infinity"]
