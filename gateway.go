package vibeproxy

import (
	"fmt"
	"net/url"
)

const (
	DefaultGatewayHTTPPort  = 3128
	DefaultGatewaySocks5Port = 1080
	DefaultGatewayHost      = "vibe-proxy-gateway.vibe-proxy.svc.cluster.local"
)

// GatewayProxyURL returns an HTTP proxy URL for the gateway that encodes the
// consumer identity. Use this as the proxy URL in http.Transport.Proxy or
// equivalent. The gateway handles lease acquisition and failover internally.
//
// Example:
//
//	proxyURL := vibeproxy.GatewayProxyURL("gw.example.com", 3128, accountID, apiKey)
//	// => "http://acct-123:secret@gw.example.com:3128"
func GatewayProxyURL(host string, port int, consumerID, apiKey string) string {
	return fmt.Sprintf("http://%s:%s@%s:%d",
		url.PathEscape(consumerID),
		url.PathEscape(apiKey),
		host, port)
}

// GatewaySOCKS5URL returns a SOCKS5 proxy URL for the gateway.
func GatewaySOCKS5URL(host string, port int, consumerID, apiKey string) string {
	return fmt.Sprintf("socks5://%s:%s@%s:%d",
		url.PathEscape(consumerID),
		url.PathEscape(apiKey),
		host, port)
}

// GatewayProxyURLWithPool returns an HTTP proxy URL that specifies a
// particular pool, using the "{pool_id}:{consumer_id}" username format.
func GatewayProxyURLWithPool(host string, port int, poolID, consumerID, apiKey string) string {
	username := poolID + ":" + consumerID
	return fmt.Sprintf("http://%s:%s@%s:%d",
		url.PathEscape(username),
		url.PathEscape(apiKey),
		host, port)
}
