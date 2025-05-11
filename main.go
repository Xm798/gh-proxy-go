package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type StaticConfig struct {
	Host string `mapstructure:"host" json:"host"`
	Port int    `mapstructure:"port" json:"port"`
}

type DynamicConfig struct {
	WhiteList       []string `mapstructure:"whiteList" json:"whiteList"`
	BlackList       []string `mapstructure:"blackList" json:"blackList"`
	ForceEnUSForRaw bool     `mapstructure:"forceEnUSForRaw" json:"forceEnUSForRaw"`
	SizeLimit       int64    `mapstructure:"sizeLimit" json:"sizeLimit"`
}

var (
	exps = []*regexp.Regexp{
		regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+)/([^/]+)/(?:releases|archive)/.*$`),
		regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+)/([^/]+)/(?:blob|raw)/.*$`),
		regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+)/([^/]+)/(?:info|git-).*$`),
		regexp.MustCompile(`^(?:https?://)?raw\.github(?:usercontent|)\.com/([^/]+)/([^/]+)/.+?/.+$`),
		regexp.MustCompile(`^(?:https?://)?gist\.github\.com/([^/]+)/.+?/.+$`),
	}
	httpClient *http.Client
	staticCfg  *StaticConfig
	dynamicCfg atomic.Value
)

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath("./config")

	// Set default values
	viper.SetDefault("host", "0.0.0.0")
	viper.SetDefault("port", 8080)
	viper.SetDefault("forceEnUSForRaw", false)
	viper.SetDefault("whiteList", []string{})
	viper.SetDefault("blackList", []string{})
	// default size limit: 10240 MB (10GB)
	viper.SetDefault("sizeLimit", 10240)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Printf("Config file not found, using defaults")
		} else {
			log.Printf("Error reading config file: %v", err)
		}
	}

	// Watch config file changes
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("Config file changed: %s", e.Name)
		loadDynamicConfig()
	})

	loadConfig()
}

func loadConfig() {
	staticCfg = &StaticConfig{
		Host: viper.GetString("host"),
		Port: viper.GetInt("port"),
	}

	loadDynamicConfig()
}

func loadDynamicConfig() {
	newCfg := &DynamicConfig{
		WhiteList:       viper.GetStringSlice("whiteList"),
		BlackList:       viper.GetStringSlice("blackList"),
		ForceEnUSForRaw: viper.GetBool("forceEnUSForRaw"),
		SizeLimit:       int64(viper.GetInt("sizeLimit")) * 1024 * 1024,
	}
	dynamicCfg.Store(newCfg)

	log.Printf("Dynamic configuration loaded - WhiteList: %d items, BlackList: %d items, ForceEnUSForRaw: %v, SizeLimit: %d MB",
		len(newCfg.WhiteList), len(newCfg.BlackList), newCfg.ForceEnUSForRaw, newCfg.SizeLimit/1024/1024)
}

func main() {
	log.Println("Starting GitHub Proxy Server...")

	initConfig()

	log.Printf("Listening on %s:%d", staticCfg.Host, staticCfg.Port)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          1000,
			MaxIdleConnsPerHost:   1000,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 300 * time.Second,
		},
	}

	router.StaticFile("/", "./public/index.html")
	router.Static("/favicon", "./public/favicon")

	router.NoRoute(handler)

	err := router.Run(fmt.Sprintf("%s:%d", staticCfg.Host, staticCfg.Port))
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func handler(c *gin.Context) {
	rawPath := strings.TrimPrefix(c.Request.URL.RequestURI(), "/")

	for strings.HasPrefix(rawPath, "/") {
		rawPath = strings.TrimPrefix(rawPath, "/")
	}

	// Add https:// prefix if missing
	if !strings.HasPrefix(rawPath, "http://") && !strings.HasPrefix(rawPath, "https://") {
		rawPath = "https://" + rawPath
	}

	matches := checkURL(rawPath)
	if matches != nil {
		cfg := dynamicCfg.Load().(*DynamicConfig)
		if len(cfg.WhiteList) > 0 && !checkList(matches, cfg.WhiteList) {
			log.Printf("Access denied by white list rules: %s", rawPath)
			c.String(http.StatusForbidden, "Access denied.")
			return
		}
		if len(cfg.BlackList) > 0 && checkList(matches, cfg.BlackList) {
			log.Printf("Access denied by black list rules: %s", rawPath)
			c.String(http.StatusForbidden, "Access denied.")
			return
		}

		if exps[1].MatchString(rawPath) {
			rawPath = strings.Replace(rawPath, "/blob/", "/raw/", 1)
		}

		proxy(c, rawPath, cfg)
	} else {
		c.String(http.StatusForbidden, "Invalid input.")
		return
	}
}

func processReqHeaders(req *http.Request, originalHeaders http.Header, url string, cfg *DynamicConfig) {
	forceEnUS := cfg.ForceEnUSForRaw

	for key, values := range originalHeaders {
		for _, value := range values {
			if forceEnUS && key == "Accept-Language" &&
				strings.Contains(url, "raw.githubusercontent.com") &&
				strings.Contains(value, "zh-CN") {
				req.Header.Add(key, "en-US")
			} else {
				req.Header.Add(key, value)
			}
		}
	}
	req.Header.Del("Host")
}

// processRespHeaders processes and modifies response headers
func processRespHeaders(c *gin.Context, resp *http.Response) {
	// Remove security-related headers
	resp.Header.Del("Content-Security-Policy")
	resp.Header.Del("Referrer-Policy")
	resp.Header.Del("Strict-Transport-Security")

	// Copy all headers to response
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// Handle Location header for redirects
	if location := resp.Header.Get("Location"); location != "" {
		if checkURL(location) != nil {
			c.Header("Location", "/"+location)
		} else {
			proxy(c, location, dynamicCfg.Load().(*DynamicConfig))
			return
		}
	}
}

func proxy(c *gin.Context, u string, cfg *DynamicConfig) {
	req, err := http.NewRequest(c.Request.Method, u, c.Request.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("server error %v", err))
		return
	}

	processReqHeaders(req, c.Request.Header, u, cfg)

	resp, err := httpClient.Do(req)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("server error %v", err))
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if contentLength, ok := resp.Header["Content-Length"]; ok {
		if size, err := strconv.Atoi(contentLength[0]); err == nil && size > int(cfg.SizeLimit) {
			c.String(http.StatusRequestEntityTooLarge, "File too large.")
			return
		}
	}

	processRespHeaders(c, resp)

	c.Status(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		return
	}
}

func checkURL(u string) []string {
	for _, exp := range exps {
		if matches := exp.FindStringSubmatch(u); matches != nil {
			return matches[1:]
		}
	}
	return nil
}

func checkList(matches, list []string) bool {
	for _, item := range list {
		if strings.HasPrefix(matches[0], item) {
			return true
		}
	}
	return false
}
