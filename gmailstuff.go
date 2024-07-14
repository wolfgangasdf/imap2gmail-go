package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// https://developers.google.com/gmail/api/quickstart/go

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func getService() (*gmail.Service, error) {
	ctx := context.Background()
	b, err := os.ReadFile(Conf.ConfGmail.Credentials)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}
	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gmail.GmailLabelsScope, gmail.GmailModifyScope, gmail.GmailInsertScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Gmail client: %v", err)
	}
	return srv, nil
}

// GmailTest list labels
func GmailTest() error {
	srv, err := getService()
	if err != nil {
		return err
	}
	r, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return fmt.Errorf("GmailTest: Unable to retrieve labels: %v", err)
	}
	log.Printf("GmailTest: found %v labels.", len(r.Labels))
	return nil
}

// GmailImport xxx
func GmailImport(traw string) error {
	log.Println("GmailImport starting...")
	srv, err := getService()
	if err != nil {
		return err
	}

	// import message https://developers.google.com/gmail/api/v1/reference/users/messages/import
	pu := func(current, total int64) { // bug: total is zero...
		log.Printf("GmailImport progress: %v", current)
	}
	resm, rese := srv.Users.Messages.Import("me", &gmail.Message{}).
		Media(strings.NewReader(traw), googleapi.ContentType("message/rfc822")).
		ProcessForCalendar(Conf.ConfGmail.ProcessForCalendar).
		ProgressUpdater(pu).Do()
	if rese != nil {
		log.Printf("GmailImport error: %v", rese)
		if gapiErr, ok := rese.(*googleapi.Error); ok {
			log.Printf("   code:%v err:%v msg:%v", gapiErr.Code, gapiErr.Error(), gapiErr.Message)
		}
		return rese
	}
	log.Printf("GmailImport result: em %v, err %v", resm.ServerResponse.HTTPStatusCode, rese)

	_, rese = srv.Users.Messages.Modify("me", resm.Id, &gmail.ModifyMessageRequest{AddLabelIds: []string{"INBOX", "UNREAD"}}).Do()
	if rese != nil {
		return rese
	}
	log.Printf("GmailImport done!")
	return nil
}
