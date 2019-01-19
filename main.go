package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/schollz/progressbar"
	"github.com/urfave/cli"
	resty "gopkg.in/resty.v1"
)

// SSHKey is an SSH keu reprensentation used for the API
type SSHKey struct {
	ID       float64
	Key      string
	Comment  string
	UserID   float64
	LastEdit float64
}

// User is a user representation used for the API
type User struct {
	ID       float64
	Username string
	Email    string
	Keys     []SSHKey
}

// cfe panic in case of an error
func cfe(err error) bool {
	if err != nil {
		log.Panicln(err)
		return false
	}
	return true
}

// fetchUser return User named `user` on the `server`
func fetchUser(user string, server string) (User, error) {
	resp, err := resty.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		Get(server + "/api/user/match/" + user)
	cfe(err)
	var f User
	err = json.Unmarshal(resp.Body(), &f)
	return f, err
}

// CreateDirIfNotExist creates `dir` if it not exists
func CreateDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		cfe(err)
	}
}

func initFileDB(storagepath string, keyfilepath string) (*sql.DB, error) {
	var dbpath string
	if os.Getenv("AKMEY_STORAGE") != "" {
		fmt.Println("Warning : Databases path is manually configured, this is not recommended.")
		storagepath = os.Getenv("AKMEY_STORAGE")
	}
	CreateDirIfNotExist(storagepath)

	fullfilepath, err := filepath.Abs(keyfilepath)
	hash := md5.Sum([]byte(fullfilepath))
	dbpath = storagepath + "/keys_" + hex.EncodeToString(hash[:]) + ".db"
	db, err := sql.Open("sqlite3", "file:"+dbpath+"?cache=shared&mode=rwc")
	cfe(err)
	sqlStmt := `
	create table if not exists users (id integer not null, name text, email text);
	create table if not exists keys (id integer not null, comment text, value text, userid integer not null);
	`
	_, err = db.Exec(sqlStmt)
	return db, err
}

func main() {
	var server string
	var dest string
	re := regexp.MustCompile("#-- Akmey START --\n((?:.|\n)+)\n#-- Akmey STOP --")
	defaultdest, err := homedir.Expand("~/.ssh/authorized_keys")
	cfe(err)
	storage, err := homedir.Expand("~/.akmey")
	cfe(err)
	defaultserv := "https://akmey.leonekmi.fr"

	app := cli.NewApp()

	app.Name = "akmey"
	app.Usage = "Add/Remove SSH keys to grant access to your friends, coworkers, etc..."
	app.Version = "0.1.1-alpha"
	app.Copyright = "GNU GPL v3 or later"
	app.Author = "Akmey contributors"
	app.Email = "akmey@leonekmi.fr"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "dest, d",
			Value:       defaultdest,
			Usage:       "Where Akmey should act (your authorized_keys file)",
			Destination: &dest,
		},
		cli.StringFlag{
			Name:        "server, s",
			Value:       defaultserv,
			Usage:       "Specify a custom Akmey server here",
			Destination: &server,
		},
	}

	app.Commands = []cli.Command{
		{
			Name:    "install",
			Aliases: []string{"i", "get", "add"},
			Usage:   "Install someone's key(s), sepcifying his e-mail or his username",
			Action: func(c *cli.Context) error {
				db, err := initFileDB(storage, dest)
				defer db.Close()
				tx, err := db.Begin()
				cfe(err)
				checkstmt, err := tx.Prepare("select name from users where email = ? or name = ?")
				cfe(err)
				var check string
				err = checkstmt.QueryRow(c.Args().First(), c.Args().First()).Scan(&check)
				if check != "" {
					fmt.Println("This user is already installed.")
					os.Exit(0)
				}
				err = nil
				stmt, err := tx.Prepare("insert into users(id, name, email) values(?, ?, ?)")
				cfe(err)
				stmt2, err := tx.Prepare("insert into keys(id, value, comment, userid) values(?, ?, ?, ?)")
				cfe(err)
				defer checkstmt.Close()
				defer stmt.Close()
				defer stmt2.Close()
				bar := progressbar.New(3)
				var tobeinserted string
				// Step 1 : fetch the user
				user, err := fetchUser(c.Args().First(), server)
				cfe(err)
				bar.Add(1)
				//fmt.Println(user)
				// Step 2 : Fetch the keys in a beautiful string
				for _, key := range user.Keys {
					stmt2.Exec(key.ID, key.Key, key.Comment, key.UserID)
					tobeinserted += key.Key + " " + key.Comment + "\n"
				}
				if tobeinserted == "" {
					fmt.Println("\nThis user does not exist or doesn't have keys registered.")
					os.Exit(1)
				}
				bar.Add(1)
				stmt.Exec(user.ID, user.Username, user.Email)
				dat, err := ioutil.ReadFile(dest)
				cfe(err)
				match := re.FindStringSubmatch(string(dat))
				if match == nil {
					tobeinserted = "\n#-- Akmey START --\n" + tobeinserted
					tobeinserted += "#-- Akmey STOP --\n"
					f, err := os.OpenFile(dest, os.O_APPEND|os.O_WRONLY, 0600)
					cfe(err)
					defer f.Close()
					_, err = f.WriteString(tobeinserted)
					cfe(err)
				} else {
					tobeinserted = match[1] + tobeinserted
					newContent := strings.Replace(string(dat), match[1], tobeinserted, -1)
					err = ioutil.WriteFile(dest, []byte(newContent), 0)
					cfe(err)
				}
				tx.Commit()
				bar.Add(1)
				return nil
			},
		},
		{
			Name:    "uninstall",
			Aliases: []string{"u", "remove"},
			Usage:   "Uninstall someone's key(s), specifying his e-mail or his username",
			Action: func(c *cli.Context) error {
				db, err := initFileDB(storage, dest)
				defer db.Close()
				tx, err := db.Begin()
				cfe(err)
				checkstmt, err := tx.Prepare("select id from users where email = ? or name = ?")
				cfe(err)
				var check string
				err = checkstmt.QueryRow(c.Args().First(), c.Args().First()).Scan(&check)
				if check == "" {
					fmt.Println("This user is not installed.")
					os.Exit(0)
				}
				err = nil
				stmt, err := tx.Prepare("delete from users where email = ? or name = ?")
				cfe(err)
				stmt2, err := tx.Prepare("delete from keys where value = ?")
				cfe(err)
				stmt3, err := tx.Prepare("select * from keys where userid = ?")
				cfe(err)
				defer checkstmt.Close()
				defer stmt.Close()
				defer stmt2.Close()
				defer stmt3.Close()
				bar := progressbar.New(4)
				// Step 1 : Fetch installed keys
				rows, err := stmt3.Query(check)
				cfe(err)
				defer rows.Close()
				toberemoved := map[int]string{}
				bar.Add(1)
				//fmt.Println(user)
				// Step 2 : Parse the keys in a beautiful map
				for rows.Next() {
					var id int
					var value string
					var comment string
					err = rows.Scan(&id, &value, &comment)
					stmt2.Exec(value)
					toberemoved[id] = "\n" + value + " " + comment
					//tobeinserted += key.Key + " " + key.Comment + "\n"
				}
				err = rows.Err()
				cfe(err)
				bar.Add(1)
				if len(toberemoved) == 0 {
					fmt.Println("\nThis user does not exist or doesn't have keys registered.")
					os.Exit(1)
				}
				stmt.Exec(c.Args().First(), c.Args().First())
				bar.Add(1)
				dat, err := ioutil.ReadFile(dest)
				newContent := ""
				cfe(err)
				match := re.FindStringSubmatch(string(dat))
				if match == nil {
					fmt.Println("Akmey is not present in this file")
					os.Exit(0)
				}
				for _, torm := range toberemoved {
					if newContent == "" {
						newContent = strings.Replace(string(dat), match[1], torm, -1)
					} else {
						newContent = strings.Replace(newContent, match[1], torm, -1)
					}
				}
				err = ioutil.WriteFile(dest, []byte(newContent), 0)
				cfe(err)
				tx.Commit()
				bar.Add(1)
				return nil
			},
		},
		{
			Name:    "reset",
			Aliases: []string{"u-all", "remove-all"},
			Usage:   "Uninstall ALL keys (these from Akmey only)",
			Action: func(c *cli.Context) error {
				db, err := initFileDB(storage, dest)
				defer db.Close()
				tx, err := db.Begin()
				cfe(err)
				stmt, err := tx.Prepare("delete from users")
				cfe(err)
				stmt2, err := tx.Prepare("delete from keys")
				cfe(err)
				stmt3, err := tx.Prepare("select * from keys")
				cfe(err)
				defer stmt.Close()
				defer stmt2.Close()
				defer stmt3.Close()
				bar := progressbar.New(4)
				// Step 1 : Fetch installed keys
				rows, err := stmt3.Query()
				cfe(err)
				defer rows.Close()
				toberemoved := map[int]string{}
				bar.Add(1)
				//fmt.Println(user)
				// Step 2 : Parse the keys in a beautiful map
				for rows.Next() {
					var id int
					var value string
					var comment string
					err = rows.Scan(&id, &value, &comment)
					toberemoved[id] = "\n" + value + " " + comment
					//tobeinserted += key.Key + " " + key.Comment + "\n"
				}
				err = rows.Err()
				stmt2.Exec()
				cfe(err)
				bar.Add(1)
				if len(toberemoved) == 0 {
					fmt.Println("\nThere is no keys installed by Akmey here.")
					os.Exit(1)
				}
				stmt.Exec()
				bar.Add(1)
				dat, err := ioutil.ReadFile(dest)
				newContent := ""
				cfe(err)
				match := re.FindStringSubmatch(string(dat))
				if match == nil {
					fmt.Println("Akmey is not present in this file")
					os.Exit(0)
				}
				for _, torm := range toberemoved {
					if newContent == "" {
						newContent = strings.Replace(string(dat), match[1], torm, -1)
					} else {
						newContent = strings.Replace(newContent, match[1], torm, -1)
					}
				}
				err = ioutil.WriteFile(dest, []byte(newContent), 0)
				cfe(err)
				tx.Commit()
				bar.Add(1)
				return nil
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))

	app.Action = func(c *cli.Context) error {
		fmt.Println("It looks like you entered a unknown command, try `akmey help`.")
		return nil
	}

	cfe(err)
	apperr := app.Run(os.Args)
	cfe(apperr)
}
