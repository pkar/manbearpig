package manbearpig

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	c2dmServiceURL string = "http://android.apis.google.com/c2dm/send"
)

type C2DM struct {
	Client *http.Client
}

// https://developers.google.com/android/c2dm/
func (c C2DM) Push(notification *Notification, authKey string) *PushStatus {
	ps := NewPushStatus(notification)
	if len(notification.DeviceTokens) == 0 {
		log.Printf("No Registration IDs %+v", notification)
		ps.Errors[""] = fmt.Errorf("NoDeviceTokens")
		return ps
	}

	for _, devToken := range notification.DeviceTokens {
		regid := devToken
		if len(notification.Payload) == 0 {
			log.Printf("No Payload Defined %+v", notification)
			ps.Errors[regid] = fmt.Errorf("NoPayload")
			return ps
		}

		data := url.Values{}
		data.Set("registration_id", devToken)
		data.Set("collapse_key", notification.AppName)
		//data.Set("delay_while_idle", 60*60)
		for k, v := range notification.Payload {
			switch k {
			case "id":
				continue
			default:
				val, ok := v.(string)
				if ok {
					data.Set("data."+k, val)
				}
			}
		}

		enc := data.Encode()
		if len(enc) >= 1024 {
			log.Printf("Message Too Long (1024 max): %d", len(enc))
			ps.Errors[regid] = fmt.Errorf("MessageTooBig")
			return ps
		}
		request, err := http.NewRequest("POST", c2dmServiceURL, strings.NewReader(enc))
		if err != nil {
			log.Printf("%s", err)
			ps.Errors[regid] = err
			ps.Retry = true
			return ps
		}

		request.Header.Add("Authorization", fmt.Sprintf("GoogleLogin auth=%s", authKey))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := c.Client.Do(request)
		if err != nil {
			log.Printf("%s %+v", err, resp)
			ps.Retry = true
			ps.Errors[regid] = err
			return ps
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case 503:
			// Service Unavailable
			// Sleep and retry after a second.
			log.Printf("C2DM 503 Service Unavailable %+v", notification)
			after := resp.Header.Get("Retry-After")
			sleepFor, _ := strconv.Atoi(after)
			ps.Delay = sleepFor
			ps.Retry = true
			ps.Errors[regid] = fmt.Errorf("ServiceUnavailable")
			return ps
		case 500:
			// Internal Server Error
			// Sleep and retry after a second.
			log.Printf("C2DM 500 Internal Server Error %+v", notification)
			after := resp.Header.Get("Retry-After")
			sleepFor, _ := strconv.Atoi(after)
			ps.Delay = sleepFor
			ps.Retry = true
			ps.Errors[regid] = fmt.Errorf("ServiceUnavailable")
			return ps
		case 401:
			// Unauthorized
			log.Printf("C2DM Unauthorized %+v", notification)
			ps.Errors[regid] = fmt.Errorf("Unauthorized")
			return ps
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("C2DM %v %+v", err, notification)
			ps.Retry = true
			ps.Errors[regid] = err
			return ps
		}
		//regexp.Compile(`id=(.*)`)
		re, err := regexp.Compile(`Error=(.*)`)
		if err != nil {
			log.Printf("C2DM %v %+v", err, notification)
			ps.Retry = true
			ps.Errors[regid] = err
			return ps
		}
		errs := re.FindStringSubmatch(string(body))
		if errs == nil {
			ps.Successes = 1
			return ps
		}

		if len(errs) >= 2 {
			switch errs[1] {
			case "QuotaExceeded":
				// Too many messages, retry after a while.
				log.Printf("C2DM Quota Exceeded %+v", notification)
				ps.Retry = true
				ps.Errors[regid] = fmt.Errorf("QuotaExceeded")
			case "DeviceQuotaExceeded":
				//  Too many messages sent by the sender to a specific device. Retry after a while.
				log.Printf("C2DM Device Quota Exceeded %+v", notification)
				ps.Retry = true
				after := resp.Header.Get("Retry-After")
				sleepFor, _ := strconv.Atoi(after)
				ps.Delay = sleepFor
				ps.Errors[regid] = fmt.Errorf("DeviceQuotaExceeded")
			case "InvalidRegistration":
				log.Printf("C2DM Invalid Registration %+v", notification)
				ps.Errors[regid] = fmt.Errorf("InvalidRegistration")
			case "NotRegistered":
				log.Printf("C2DM Not Registered %+v", notification)
				ps.Errors[regid] = fmt.Errorf("NotRegistered")
			case "MessageTooBig":
				log.Printf("C2DM Message Too Big %+v", notification)
				ps.Errors[regid] = fmt.Errorf("MessageTooBig")
			case "MissingCollapseKey":
				log.Printf("C2DM Missing Collapse Key %+v", notification)
				ps.Errors[regid] = fmt.Errorf("MissingCollapseKey")
			default:
				log.Printf("C2DM Unknown Error %+v", notification)
				ps.Retry = true
				after := resp.Header.Get("Retry-After")
				sleepFor, _ := strconv.Atoi(after)
				ps.Delay = sleepFor
				ps.Errors[regid] = fmt.Errorf("UnknownError")
			}
		}
	}
	return ps
}
