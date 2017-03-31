package main


import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kurrik/oauth1a"
	"github.com/kurrik/twittergo"
	"io/ioutil"
	_ "log"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	fromEmail   = ""
	sender      = "SENDER_GMAIL_USERNAME"
	password    = "SENDER_GMAIL_PASSWORD"
	smtp_server = "smtp.gmail.com"
)

var (
	body string
)

func send() {

	Dest := []string{"TO_EMAIL", "TO_EMAIL_ANOTHER"}

	msg := "From: " + sender + "\n" +
		"To: " + strings.Join(Dest, ",") + "\n" +
		"Subject: " + "LIRR Alert" + "\n" + body
	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	err := smtp.SendMail(smtp_server+":587",
		smtp.PlainAuth("", sender, password, smtp_server),
		sender, Dest, []byte(msg))

	if err != nil {
		fmt.Printf("smtp error: %s", err)
		return
	}

	fmt.Println("Mail sent successfully!")
}

func LoadCredentials() (client *twittergo.Client, err error) {
	credentials, err := ioutil.ReadFile("CREDENTIALS") //credentials file for TWITTER API
	if err != nil {
		return
	}
	lines := strings.Split(string(credentials), "\n")
	config := &oauth1a.ClientConfig{
		ConsumerKey:    lines[0],
		ConsumerSecret: lines[1],
	}
	user := oauth1a.NewAuthorizedConfig(lines[2], lines[3])
	client = twittergo.NewClient(config, user)
	return
}

func isAlert(tweet string) bool {
	alertStrings := []string{"delay", "post-poned", "postponed", "cancel"}
	lineStrings := []string{""} //branches you want alerts on

	for _, alert := range alertStrings {
		for _, line := range lineStrings {
			if strings.Contains(tweet, alert) && strings.Contains(tweet, line) {
				return true
			}
		}
	}
	return false
}

func main() {

	fmt.Printf("starting... \n")
	var (
		err     error
		client  *twittergo.Client
		req     *http.Request
		resp    *twittergo.APIResponse
		max_id  uint64
		query   url.Values
		text    string
		results *twittergo.Timeline
	)

	if client, err = LoadCredentials(); err != nil {
		fmt.Printf("Could not parse CREDENTIALS file: %v\n", err)
		os.Exit(1)
	}

	const (
		count   int = 10
		urltmpl     = "/1.1/statuses/user_timeline.json?%v"
		minwait     = time.Duration(10) * time.Second
	)

	query = url.Values{}
	query.Set("count", fmt.Sprintf("%v", count))
	query.Set("screen_name", "LIRR")
	total := 0

	if max_id != 0 {
		query.Set("max_id", fmt.Sprintf("%v", max_id))
	}
	endpoint := fmt.Sprintf(urltmpl, query.Encode())
	if req, err = http.NewRequest("GET", endpoint, nil); err != nil {
		fmt.Printf("Could not parse request: %v\n", err)
		os.Exit(1)
	}
	if resp, err = client.SendRequest(req); err != nil {
		fmt.Printf("Could not send request: %v\n", err)
		os.Exit(1)
	}

	results = &twittergo.Timeline{}

	if err = resp.Parse(results); err != nil {
		if rle, ok := err.(twittergo.RateLimitError); ok {
			dur := rle.Reset.Sub(time.Now()) + time.Second
			if dur < minwait {
				// Don't wait less than minwait.
				dur = minwait
			}
			msg := "Rate limited. Reset at %v. Waiting for %v\n"
			fmt.Printf(msg, rle.Reset, dur)
			time.Sleep(dur)
		} else {
			fmt.Printf("Problem parsing response: %v\n", err)
		}
	}
	batch := len(*results)
	if batch == 0 {
		fmt.Printf("No more results, end of timeline.\n")
	}

	var buffer bytes.Buffer

	for _, tweet := range *results {
		if _, err = json.Marshal(tweet); err != nil {
			fmt.Printf("Could not encode Tweet: %v\n", err)
			os.Exit(1)
		}
		text = tweet.Text()
		if isAlert(text) {
			buffer.WriteString(text)
			buffer.WriteString("\n")
		}
		max_id = tweet.Id() - 1
		total += 1
	}
	body = buffer.String()
	if len(body) != 0 {
		send()
	}

}
