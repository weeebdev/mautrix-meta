package endpoints

const (
	threadsBaseHost = "threads.com"
	threadsWWWHost  = "www." + threadsBaseHost
	threadsBaseUrl  = "https://" + threadsWWWHost
	threadsDGWBase  = "https://gateway." + threadsBaseHost
)

// ThreadsEndpoints are the threads.com equivalents of the InstagramEndpoints keys that the Slide
// client uses. Login endpoints are absent: Threads logins are cookie-based.
var ThreadsEndpoints = map[string]string{
	"host":     threadsWWWHost,
	"base_url": threadsBaseUrl,
	"messages": threadsBaseUrl + "/messages",
	"thread":   threadsBaseUrl + "/messages/t/",
	"graphql":  threadsBaseUrl + "/api/graphql",

	"dgw_lightspeed":       threadsDGWBase + "/ws/lightspeed",
	"dgw_mqttbypass":       threadsDGWBase + "/ws/mqttbypass",
	"dgw_streamcontroller": threadsDGWBase + "/ws/streamcontroller",

	"media_upload": threadsBaseUrl + "/ajax/mercury/upload.php?",

	"navigation":            threadsBaseUrl + "/ajax/navigation/",
	"route_definition":      threadsBaseUrl + "/ajax/route-definition/",
	"bulk_route_definition": threadsBaseUrl + "/ajax/bulk-route-definitions/",
}
