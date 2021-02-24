package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aiomonitors/godiscord"
	"github.com/fatih/color"
	"gopkg.in/yaml.v2"
)

var (
	cpm       uint
	result    []string
	cfg       Config
	checks    uint
	valid     uint
	errors    uint
	start     time.Time
	proxyList []string
	// Client for HTTP Requests & a timeout
	Client = http.Client{
		Timeout: 5 * time.Second,
	}
)

// Config | config.yml file struct
type Config struct {
	Main struct {
		Workers int `yaml:"workers"`
		Startid int `yaml:"startid"`
		Stopid  int `yaml:"stopid"`
	} `yaml:"main"`
	Webhook struct {
		Webhook string `yaml:"webhook"`
	} `yaml:"webhook"`
}

// GroupInfo | Struct for the API
type GroupInfo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsLocked    bool   `json:"isLocked"`
	Owner       struct {
		BuildersClubMembershipType string `json:"buildersClubMembershipType"`
		UserID                     int    `json:"userId"`
		Username                   string `json:"username"`
		DisplayName                string `json:"displayName"`
	} `json:"owner"`

	MemberCount        int  `json:"memberCount"`
	IsBuildersClubOnly bool `json:"isBuildersClubOnly"`
	PublicEntryAllowed bool `json:"publicEntryAllowed"`
}

func groupscrape(id int) {
RESTART:
	groupreq, err := http.NewRequest("GET", fmt.Sprintf("https://groups.roblox.com/v1/groups/%d", id), nil)
	if err != nil {
		fmt.Println(err)
		errors++
	}

	proxyURL, err := url.Parse(fmt.Sprintf("http://%s", proxyList[rand.Intn(len(proxyList))]))
	if err != nil {
		fmt.Println(err)
		errors++
	}

	http.DefaultTransport = &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	GroupResponse, err := Client.Do(groupreq)
	if err != nil {
		errors++
		goto RESTART
	}
	defer GroupResponse.Body.Close()

	var groupinfo *GroupInfo
	JSONParseError := json.NewDecoder(GroupResponse.Body).Decode(&groupinfo)
	if JSONParseError != nil {
		errors++
		fmt.Println(JSONParseError)
	}

	if groupinfo.Owner.DisplayName == "" {
		if groupinfo.IsLocked != true {
			if groupinfo.PublicEntryAllowed == true {
				valid++
				c := color.New(color.FgHiGreen).Add(color.Underline).Add(color.Bold)
				c.Printf("Group: %d is claimable!\n", groupinfo.ID)
				resultfile, err := os.OpenFile("results.txt", os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					fmt.Println(err)
				}
				resultfile.WriteString(fmt.Sprintf("Group: %d is claimable! | https://www.roblox.com/groups/%d\n", groupinfo.ID, groupinfo.ID))
				resultfile.Close()

				discordwebhook(groupinfo)
			}
		}
	}

	checks++
	cmd := exec.Command("cmd", "/C", "title", fmt.Sprintf("RoFind By Bixmox#2482 | Checks: %d | Valid: %d | Errors: %d | CPM: %d | Elapsed: %v | Threads: %d | ID Range: %d-%d", checks, valid, errors, cpm, time.Since(start), runtime.NumGoroutine(), cfg.Main.Startid, cfg.Main.Stopid))
	cmderr := cmd.Run()
	if cmderr != nil {
		fmt.Println(cmderr)
	}
}

func discordwebhook(groupinfo *GroupInfo) {
	http.DefaultTransport = &http.Transport{}
	embed := godiscord.NewEmbed("RoFind found a unclaimed group!", "", fmt.Sprintf("https://www.roblox.com/groups/%d", groupinfo.ID))
	embed.SetAuthor("Made by Bixmox#2482", "", "https://cdn.discordapp.com/avatars/214402548428177408/d3a08e1b2f70272f49d9d4ab7c7ea1fb.png?size=256")
	embed.AddField("Name", strings.Title(groupinfo.Name), false)
	embed.AddField("ID", fmt.Sprintf("%d", groupinfo.ID), true)
	embed.AddField("Members", fmt.Sprintf("%d", groupinfo.MemberCount), true)
	embed.AddField("Description", groupinfo.Description, false)
	discorderr := embed.SendToWebhook(cfg.Webhook.Webhook)
DISCORDRESTART:
	if discorderr != nil {
		fmt.Println(discorderr)
		goto DISCORDRESTART
	}
}

func cpmcounter() {
	for {
		oldchecked := checks
		time.Sleep(1 * time.Second)
		newchecked := checks
		cpm = (newchecked - oldchecked) * 60
	}
}

func makeRange(min, max int) []int {
	a := make([]int, max-min+1)
	for io := range a {
		a[io] = min + io
	}
	return a
}

func main() {
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	cmd.Run()

	color.Red("Made By Bixmox#2482")
	cf, err := os.Open("config.yml")
	if err != nil {
		fmt.Println(err)
	}
	defer cf.Close()

	decoder := yaml.NewDecoder(cf)
	err = decoder.Decode(&cfg)
	if err != nil {
		fmt.Println(err)
	}
	IDRange := makeRange(int(cfg.Main.Startid), int(cfg.Main.Stopid))
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(IDRange), func(i, o int) { IDRange[i], IDRange[o] = IDRange[o], IDRange[i] })

	go cpmcounter()
	start = time.Now()

	proxyFile, err := os.Open("proxies.txt")
	if err != nil {
		fmt.Println(err)
	}

	scanner := bufio.NewScanner(proxyFile)

	for scanner.Scan() {
		proxyList = append(proxyList, scanner.Text())
	}

	startTime := time.Now()
	wg := &sync.WaitGroup{}
	workChannel := make(chan int)
	for i := 0; i <= cfg.Main.Workers; i++ {
		// fmt.Println("Spawning worker", i)
		wg.Add(1)
		go worker(wg, workChannel)
	}
	for _, a := range IDRange {
		workChannel <- a
	}
	close(workChannel)
	wg.Wait()
	fmt.Println("Took ", time.Since(startTime))
}

func worker(wg *sync.WaitGroup, jobs <-chan int) {
	for j := range jobs {
		groupscrape(j)
		//fmt.Printf("Workers: %v / Finished scraping ID %v\n", runtime.NumGoroutine()-1, j)
	}
	wg.Done()
}
