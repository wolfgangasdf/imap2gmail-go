# imap2gmail

Goal: Get continuously all emails from an IMAP server into GMail without loosing emails, and using all features (filtering, spam detection) of GMail.

Imap2Gmail uses imap IDLE to poll emails from an IMAP server and imports them into a GMail account via GMail API Users.messages.import,
this ensures that mail scanners and filters are applied in GMail. If successful, the email is moved to some folder on the IMAP server, if not it is quarantined (see settings file below).
imap2gmail is supposed to run on some server 24/7 which can send emails for error notification. Disable SPAM mail filtering on the IMAP server.

I wrote this little program because (i) normal GMail doesn't poll IMAP servers and (ii) "redirecting" emails to GMail is very unreliable, in particular with Exchange servers.

### Get your own gmail API for this app
Unfortunately, you need to get your own GMail API keys for imap2gmail; there is a quota for API usage and I don't want to deal with this.

1. Go to click https://console.developers.google.com/start/api?id=gmail
2. Select `Create a project`, `Go to credentials` #REMOVE rest, then on the `Add credentials to your project` page, click the `Cancel` button.
3. `OAuth consent screen`: choose "external" user type, app name: `imap2gmail`. Save.
4. Select the `Credentials` tab, click `Create credentials` and select `OAuth client ID`.
5. Select the application type `Other`, enter the name `imap2gmail`, and click the Create button.
6. Under "OAuth 2.0 Client IDs", download the client_secret*.json file and rename to `credentials.json`.

### first-time run: settings and authentication
* Run imap2gmail: `./imap2gmail` , this creates a config file `imap2gmail.ini`, edit this.
  * make sure that sending notification emails (ConfEmail) is 100% reliable (such as local postfix), or disable it.
* Run imap2gmail again: `./imap2gmail`. Open the link in web browser, click "advanced" and "Go to imap2gmail (unsafe)", click through, and copy the code into the console and press enter.
* This will happen only once.

#### example configuration file imap2gmail.ini
```
; settings for the imap server that is polled
[ConfImap]
Server           = mail.mailserver.com
; port of imap server
Port             = 993
User             = username
Password         = password
; imap folder to watch like INBOX
Folder           = INBOX
; imap folder where to move imported messages to
FolderMoved      = nowingmail
; imap folder where to move failed emails
FolderQuarantine = imap2gmailquarantine
; larger emails are quarantined
MaxEmailSize     = 25000000
; idle timeout minutes
IdleTimeout      = 5m

; Notification and warning emails settings
[ConfMail]
; From email address
From     = root@imap2gmailserver.org
; email address to send notofications to
To       = name@gmail.com
; smtp mail server, leave empty to disable
Server   = localhost
; smtp port
Port     = 25
; smtp user
User     =
Password =

; Gmail settings
[ConfGmail]
; credentials.json file path
Credentials        = credentials.json
; process emails for calendar etc
ProcessForCalendar = true
```

### build & run
```
go build && ./imap2gmail
```

### cross-compile, e.g. for linux
GOOS=linux GOARCH=amd64 go build -o imap2gmail-linux-amd64

### uses

* [go-imap](https://github.com/emersion/go-imap)
* [go-ini](https://pkg.go.dev/mod/gopkg.in/ini.v1)
* [gomail](https://pkg.go.dev/gopkg.in/gomail.v2)
* [go gmail api](https://developers.google.com/gmail/api/quickstart/go)
