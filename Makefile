MAKEFLAGS += --silent

all:
	go get -v github.com/mattn/go-sqlite3
	go get -v github.com/mitchellh/go-homedir
	go get -v github.com/schollz/progressbar
	go get -v github.com/urfave/cli
	go get -v gopkg.in/resty.v1
	go build -o akmey

get-deps:
	go get -v github.com/mattn/go-sqlite3
	go get -v github.com/mitchellh/go-homedir
	go get -v github.com/schollz/progressbar
	go get -v github.com/urfave/cli
	go get -v gopkg.in/resty.v1

build:
	go build -o akmey

install:
	go get -v github.com/mattn/go-sqlite3
	go get -v github.com/mitchellh/go-homedir
	go get -v github.com/schollz/progressbar
	go get -v github.com/urfave/cli
	go get -v gopkg.in/resty.v1
	go build -o akmey
	install -Dm755 akmey /usr/bin/akmey
