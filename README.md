# Manbearpig
A black box for APNS/GCM/C2DM push notifications.

See examples directory for how to run. This is taken from production code that had
lots of legacy dependancies, and may or may not
be fully functional. Use as reference only.
Not that all tests have not been ported over.

If it's usefull for someone, great.

## Server

```go
export GOPATH=$(pwd)
go get 
go run src/examples/example.go
```

## API

### POST /jobs

#### Request
```javascript
{
	jobs :[
		{
			app_name: "fart app",           // application name
			provider: "apns",              // apns/c2dm/gcm
			device_tokens: [],             // always an array of tokens to send payload to
			expiry: 3600,                  // seconds optional
			payload: {"payloadstuff": 1234},
			extra_data: {"whatever": 1},   // optional
		},
		...
	],
	auth: "api token/cert key"
}
```

#### Response
```
200 OK
```

#### Response Error
```
400 Bad Request
```

### Example Job GCM
```javascript
{
	jobs :[ 
		{
			app_name: "fart app",
			provider: "gcm",
			device_tokens: ["APA91bGsnnyg2LzRA7kpV7NmYMcgsaVTJggXz1zp2TWtU6ZRDPA-N4FEBV3lnG1wM-hqGNhZbbQ81lHzwlKvYgr0ukGIqg3YQsY4txJiaNTQy98vYr-cagW1K9EfN9t_esP1BHJo_XO4JrM3nv5H84u7H9tjJK0ChYAlnt6Ihab_L9wuWbKVBns"],
			expiry: 3600,
			payload: {
				"extras":"{\"p\":\"1370528144\"}",
				"senderId":0,
				"message":"Now only: Big sale at the store!",
				"published":1370528402,
				"id":"MmQ1ZjYyYWUtY2ViNC0xMWUyLTg3M2UtZjFhMTZmMDkwOWEz",
				"remote":0
			},
			extra_data: {
				"internal": 1
			}
		}
	],
	auth: "BIaUbyCN8EQbaOCjP6_KbEwJVnkSPoI-e5RpJsI"
}
```

### Example Job C2DM
```javascript
{
	jobs :[ 
		{
			app_name: "fart app",
			provider: "c2dm",
			device_tokens: ["APA91bGsnnyg2LzRA7kpV7NmYMcgsaVTJggXz1zp2TWtU6ZRDPA-N4FEBV3lnG1wM-hqGNhZbbQ81lHzwlKvYgr0ukGIqg3YQsY4txJiaNTQy98vYr-cagW1K9EfN9t_esP1BHJo_XO4JrM3nv5H84u7H9tjJK0ChYAlnt6Ihab_L9wuWbKVBns"],
			expiry: 3600,
			payload: {
				"extras":"{\"p\":\"1370528144\"}",
				"senderId":0,
				"message":"Now only: Big sale at the store!",
				"published":1370528402,
				"id":"MmQ1ZjYyYWUtY2ViNC0xMWUyLTg3M2UtZjFhMTZmMDkwOWEz",
				"remote":0
			},
			extra_data: {
				"internal": 1
			}
		}
	],
	auth: "BIaUbyCN8EQbaOCjP6_KbEwJVnkSPoI-e5RpJsI"
}
```

### Example Job APNS
```javascript
{
	jobs :[ 
		{
			app_name: "fart app",
			provider: "apns",
			device_tokens: ["a7c32058f5d6fa27852728bf8d557e4c45df13048a8475b4f88ae904a579nf97"],
			expiry: 3600,
			payload: {
				aps: {
					"alert": "You\'ve recovered all your points!",
					"sound": "default"
				},
				"x": [
					0,
					"Njg4ZmNlOTYtY2VkNi0xMWUyLTlmYzktZDJkODU5Nzc0ZGUz",
					0,
					{
						"pn_type": "2",
						"pn_type_detail": ""
					},
					1370543104
				]
			},
			extra_data: {
				"internal": 1
			}
		}
	],
	auth: "
		-----BEGIN CERTIFICATE-----
		some cert
		-----END CERTIFICATE-----
		-----BEGIN RSA PRIVATE KEY-----
		some key
		-----END RSA PRIVATE KEY-----
	"
}
```
