# gcal-to-ics
[![Actions Status](https://github.com/Eun/gcal-to-ics/workflows/push/badge.svg)](https://github.com/Eun/gcal-to-ics/actions)
[![Coverage Status](https://coveralls.io/repos/github/Eun/gcal-to-ics/badge.svg?branch=master)](https://coveralls.io/github/Eun/gcal-to-ics?branch=master)
[![PkgGoDev](https://img.shields.io/badge/pkg.go.dev-reference-blue)](https://pkg.go.dev/github.com/Eun/gcal-to-ics)
[![go-report](https://goreportcard.com/badge/github.com/Eun/gcal-to-ics)](https://goreportcard.com/report/github.com/Eun/gcal-to-ics)
---
Export a google calendar to a ical ics file.



## Usage


### Prerequisites
1. Create a new google app and add the following scopes:
    ```
    https://www.googleapis.com/auth/calendar.readonly
    https://www.googleapis.com/auth/calendar.events.readonly
    ```
2. Create a oauth 2.0 client id and save the client_id & client_secret


### Simple export
```
gcal-to-ics export --client_id=google_client_id         \
                   --client_secret=google_client_secret \
                   --account=name@gmail.com             \
                   --calendar="my calendar"               \
                   --output=out.ics
```

### http server 
1. Create a `config.yml`
   ```yaml
   my-first-calendar:
     account_email: name@gmail.com
     calendar_name: my calendar
     formats:
       - ics
     overwrite_fields:
       visibility: public
     hide_fields:
       organizer: true
       attendees: true
       conference: true
   my-second-calendar:
    account_email: name@gmail.com
    calendar_name: my calendar
    formats:
      - ics
    overwrite_fields:
      visibility: public
    hide_fields:
      organizer: true
      attendees: true
      conference: true
    ```
2. Create a `tokens` dir (this is where the tokens will be stored)
3. Setup your environment:
   ```
    export CLIENT_ID="google client id"
    export CLIENT_SECRET="google client secret"
    export CONFIG_FILE="config.yml"
    export BIND_ADDR=":8080"
    export PUBLIC_URI="http://localhost:8080"
    export TOKEN_DIR="tokens"
    export CRYPT_SECRET="A secret for encrpyting the tokens"
   ```
4. Run the application with the serve subcommand:
   ```
   gcal-to-ics serve
   ```
5. Navigate to `http://localhost:8080/my-first-calendar.ics`

---
### Codequality
Code is not good but does what it should.

### Build History
[![Build history](https://buildstats.info/github/chart/Eun/gcal-to-ics?branch=master)](https://github.com/Eun/gcal-to-ics/actions)