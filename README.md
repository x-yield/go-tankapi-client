go-tankapi-client provides an interface for Yandex.Tank servers with yandex-tank-api.
Allows to synchronize tests workflow on multiple servers.

To validate your config on a certain tank server (tanks may have different local defaults, plugins installed, versions etc.) use "Validate" function.
This will send configs to corresponding tanks, populate Session structs and return them.
If your config is invalid the list of session.Failures will be non-empty and the status will be "failed".
```
{
    Tank: <tank>,
    Config: <config>,
    Failures: [not empty],
    Stage: "validation",
    Status: "failed",
}
```
Update your config according to failures you got and retry.
As soon as your config is considered valid, you can run the test.
You could run it without any prior validation, tank will validate it anyway or fail during runtime

```go
package main
import "github.com/load-tools/go-tankapi-client/tankapi"

func main() {
	client := tankapi.NewClient()
	s1 := tankapi.NewSession("<tankapi address>", "<config in yaml format>")
	sessions := []tankapi.Session{s1}
	sessions = client.Validate(sessions) // optional
	sessions = client.Run(sessions)
	// you can .Poll sessions here
}
```

So, in a basic case "Run" function should be enough.
It will validate your config and try to run the test.
Also it sets session "Name"s
This name is needed for further session poll.
You still need to poll it in order to receive it's current status though.

If the session has "poll" stage this means that it is running.
"finished" stage means what it means, but check the session status whether it is not "failed"

see tankapi stages here https://github.com/load-tools/yandex-tank-api#test-stages

But if you need several tests to start simultaneously, you may want to prepare tests on all the servers first.
In order to do so, use "Prepare" function.
It will send preparing request to corresponding tanks, and they will download ammo, step them, prepare all the plugins etc.
Also it sets session "Name"s.
This name is needed for further session polling and running

"prepared" status indicates that tank is prepared 

And once all the servers are ready to start sending load, you trigger the "Run" function.

Please note that if your sessions do not have names by this time, it means that preparation has failed at some point and "Run" function will use session config to try to start a new session without preparation. 
```go
package main
import "github.com/load-tools/go-tankapi-client/tankapi"

func main() {
	client := tankapi.NewClient()
	s1 := tankapi.NewSession("<tankapi address>", "<config in yaml format>")
	sessions := []tankapi.Session{s1}
	sessions = client.Validate(sessions) // optional 
	sessions = client.Prepare(sessions) 
	// .Poll sessions here 
	sessions = client.Run(sessions) 
	// .Poll sessions here
}
```
