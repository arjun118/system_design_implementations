package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"database/sql"

	"github.com/bwmarrin/snowflake"
	"github.com/deatil/go-encoding/base62"
	"github.com/gin-gonic/gin"

	_ "modernc.org/sqlite"
)

// what do i need for a url shortener
// 100 mil urls generated per day
// a data base
// -> a pk (id) auto increment
// short url
// original url

// hash fn -> long url to short url
// 62 pos characters (0-9, a-z, A-Z)
// 365 bil urls, say n=7 is the length of the
// hash value

// two types of hash functions
// hash + collision resolution

// we use the first 7 characters from either
// crc32 or md5 or sha-1 hash
// and check for collision,
// if found we add a predefined string to the
// original url and rehash and repeat the process
// until no collision is found
// we can leverage bloom filters to check for
// collision

// base 62 converison

// leveraging uuid for unique id generation
// and converting that to base62

var currentNode *snowflake.Node

var db *sql.DB

func getUniqueHash() (string, error) {
	id := currentNode.Generate()
	bBigEndian := make([]byte, 8) // int64 is 8 bytes
	binary.BigEndian.PutUint64(bBigEndian, uint64(id))
	return base62.StdEncoding.EncodeToString(bBigEndian), nil
}

type incomingURL struct {
	URL string `json:"url_string"`
}

func normalizeURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)

	if (parsed.Scheme == "http" && strings.HasSuffix(parsed.Host, ":80")) ||
		(parsed.Scheme == "https" && strings.HasSuffix(parsed.Host, ":443")) {
		host, _, _ := strings.Cut(parsed.Host, ":")
		parsed.Host = host
	}

	parsed.Fragment = ""

	if parsed.Path == "/" {
		parsed.Path = ""
	}

	if parsed.RawQuery != "" {
		q := parsed.Query()

		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		newQuery := url.Values{}
		for _, k := range keys {
			vals := q[k]
			sort.Strings(vals)
			for _, v := range vals {
				newQuery.Add(k, v)
			}
		}
		parsed.RawQuery = newQuery.Encode()
	}

	return parsed.String(), nil
}

func shortenURL(c *gin.Context) {
	var incoming incomingURL
	if err := c.BindJSON(&incoming); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	normURL, err := normalizeURL(incoming.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid URL"})
		return
	}
	var existingCode string
	err = db.QueryRow("SELECT short_code FROM urls WHERE original_url = ?", normURL).Scan(&existingCode)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"short_url": "http://localhost:8181/" + existingCode})
		return
	}

	shortCode, _ := getUniqueHash()
	_, err = db.Exec("INSERT INTO urls (original_url, short_code) VALUES (?, ?)", normURL, shortCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"short_url": "http://localhost:8181/" + shortCode})
}

func redictToOriginal(c *gin.Context) {
	code := c.Param("code")
	var original_url string
	err := db.QueryRow("SELECT original_url FROM urls WHERE short_code = ?", code).Scan(&original_url)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"error": "URL not found"})
		} else {
			c.JSON(500, gin.H{"error": "Database error"})
		}
		return
	}
	c.Redirect(302, original_url)
}

func main() {
	var err error
	currentNode, err = snowflake.NewNode(1)
	if err != nil {
		fmt.Println(err)
		return
	}
	db, err = sql.Open("sqlite", "file:shortener.db")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_url TEXT NOT NULL,
		short_code   TEXT NOT NULL UNIQUE
	);`)
	if err != nil {
		log.Fatal("create table:", err)
	}
	defer db.Close()
	router := gin.Default()
	router.POST("/shorten", shortenURL)
	router.GET("/:code", redictToOriginal)
	router.Run("localhost:8181")

}
