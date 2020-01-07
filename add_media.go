package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bakape/thumbnailer"
	"github.com/cheggaaa/pb"
	ipfs "github.com/ipfs/go-ipfs-api"
	h "github.com/mirisbowring/PrImBoard-Utils/helper"
	"golang.org/x/crypto/ssh/terminal"
)

type config struct {
	IpfsGateway   string `json:"ipfs_gateway"`
	IpfsNodeAPI   string `json:"ipfs_node_api"`
	PrimboardHost string `json:"primboard_host"`
}

type media struct {
	Sha1      string
	Title     string
	Creator   string
	Tags      []int32
	Timestamp int64
	URL       string
	URLThumb  string
	Type      string
	Format    string
}

type user struct {
	Username string
	Password string
}

var u user
var cfg config

func main() {
	// check if at least one argument has been passed
	if len(os.Args) < 2 {
		log.Fatal("You must specify a path!")
	} else if len(os.Args) == 3 {
		// if 2 arguments have been passed, assume that the second is a custom
		// config file
		readConfig(os.Args[2])
	} else {
		// assumes, that the user is using the default config file
		readConfig("env.json")
	}
	// reading first argument as path
	path := os.Args[1]

	// check if passed path is valid
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatal("The passed Path is not valid!")
	}
	// check if host is available
	checkHost()
	// login
	login()
	//collecting files
	var files []string
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		//ignore dirs
		mode, _ := os.Stat(path)
		if !mode.Mode().IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		log.Fatal("An error occured while fetching file informations!")
	}
	log.Printf("Found %d files", len(files))
	log.Println("Start adding files...")
	doMedia(files)
}

/**
 * readConfig reads the static environment variables from the env file
 */
func readConfig(file string) {
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// decode the file content into go object
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		log.Fatal(err)
	}
}

/**
 * doMedia iterates over the passed array that contains all filenames, which
 * should be added and adds them to the ipfs network and the mongo db over the
 * primboard api
 */
func doMedia(files []string) {
	bar := pb.StartNew(len(files))
	// new ipfs shell
	sh := ipfs.NewShell(cfg.IpfsNodeAPI)
	for _, file := range files {
		// create file pointer
		r, _ := os.Open(file)
		// add the file to ipfs
		// do not use the recursive AddDir because we need to add all the files to the mongo
		cid, err := sh.Add(r)
		if err != nil {
			log.Fatal(err)
		}
		r.Close()
		//recreate file pointer (add is manipulating it)
		r, _ = os.Open(file)
		// create thumbnail and receive pointer
		rt, src := h.Thumbnail(r)
		// add the thumbnail to the ipfs
		thumbCid, err := sh.Add(rt)
		if err != nil {
			log.Fatal(err)
		}
		r.Close()
		// if successfull, create a media object with the returned ipfs url
		var m media
		if (src.Meta != thumbnailer.Meta{} && src.Meta.Title != "") {
			m.Title = src.Meta.Title
		}
		m.Sha1 = cid
		m.URL = cfg.IpfsGateway + cid
		m.URLThumb = cfg.IpfsGateway + thumbCid
		// eval mime to generic type
		if src.HasVideo {
			m.Type = "video"
		} else if src.HasAudio {
			m.Type = "audio"
		} else {
			m.Type = "image"
		}
		m.Format = src.Extension
		// encode the object to json
		b := new(bytes.Buffer)
		json.NewEncoder(b).Encode(m)
		// post the object to the api
		res, _ := http.Post("http://"+cfg.PrimboardHost+"/api/v1/media", "application/json", b)
		if res.StatusCode != 201 {
			log.Fatal(res.StatusCode)
		}
		// step forward if nothing went wrong
		bar.Increment()
	}
	bar.Finish()
}

/**
 * checkHost validates, whether the primboard host is reachable or not
 */
func checkHost() {
	timeOut := time.Duration(2 * time.Second)
	_, err := net.DialTimeout("tcp", cfg.PrimboardHost, timeOut)
	if err != nil {
		log.Fatal(err)
	}
}

/**
 * login authenticates the user against the api and retrieves the authentication
 * token
 */
func login() {
	// read username
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Username: ")
	u.Username, _ = reader.ReadString('\n')
	u.Username = strings.TrimSpace(u.Username)
	// read password
	fmt.Print("Password: ")
	bytePassword, err := terminal.ReadPassword(0)
	fmt.Println()
	if err != nil {
		log.Fatal("An error occured while reading the password!")
	}
	u.Password = string(bytePassword)
	// log the user in
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(u)
	res, _ := http.Post("http://"+cfg.PrimboardHost+"/api/v1/login", "application/json", b)
	if res.StatusCode != 200 {
		io.Copy(os.Stdout, res.Body)
		log.Fatal("Cloud not authenitcate to server.")
	}
}

/**
 * isInArray checks if the passed string is in the passed array.
 * Returns:
 *	true -> is in the array
 *	false -> is not in the array
 */
func isInArray(item string, array *[]string) bool {
	for _, i := range *array {
		if i == item {
			return true
		}
	}
	return false
}
