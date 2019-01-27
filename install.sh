
#!/usr/bin/env bash

confirm() {
	read -r -p "${1:-Are you sure? [y/N]} " response
	case "$response" in
		[yY][eE][sS]|[yY])
			true
			;;
		*)
			exit
			;;
	esac
}

echo "
Akmey - Install script for Linux
"


if [ $1 = "compile" ]; then
	confirm "*Compile* Akmey client in /usr/local/bin/ [y/N]"
	 path_to_executable=$(which go)
	if [ -x "$path_to_executable" ] ; then
		echo "Go found: $path_to_executable"
		if [[ $EUID -ne 0 ]]; then
			tput setaf 3
			echo "This script must be run as root" 1>&2
			tput sgr 0
			exit 1
		else
			if [ -f main.go ]
			then
				go get github.com/mattn/go-sqlite3 github.com/mitchellh/go-homedir github.com/schollz/progressbar github.com/urfave/cli
				go build -o bin/akmey
				chmod 755 bin/akmey
				cp bin/akmey /usr/local/bin
				tput setaf 2
				echo "Enjoy keys with akmey command !"
				tput sgr 0
				exit 0
			else
				tput setaf 3
				echo "Missing akmey source file, please clone the entire repository with git" 1>&2
				tput sgr 0
				exit 1
			fi
		fi
	fi
else
	confirm "Install Akmey client in /usr/local/bin/ [y/N]"

	if [[ $EUID -ne 0 ]]; then
		tput setaf 3
		echo "This script must be run as root" 1>&2
		tput sgr 0
		exit 1
	else
		if [ -f bin/akmey ]
		then
			chmod 755 bin/akmey
			cp bin/akmey /usr/local/bin
			tput setaf 2
			echo "Enjoy keys with akmey command !"
			tput sgr 0
			exit 0
		else
			tput setaf 3
			echo "Missing akmey bin, please clone the entire repository with git" 1>&2
			tput sgr 0
			exit 1
		fi
	fi
fi
