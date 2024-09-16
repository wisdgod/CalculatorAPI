package utils

import (
	"math/rand"
	"net/http"
	"strings"
)

func GetRealIP(r *http.Request) string {
	cfConnectingIP := r.Header.Get("CF-Connecting-IP")
	realIP := r.Header.Get("X-Real-IP")
	forwardedFor := r.Header.Get("X-Forwarded-For")
	remoteAddr := strings.Split(r.RemoteAddr, ":")[0]

	return cfConnectingIP + "," + realIP + "," + forwardedFor + "," + remoteAddr
}

func GetRandomNonEmptyIP(ip string) string {
	ips := strings.Split(ip, ",")
	nonEmptyIPs := []string{}
	for _, ip := range ips {
		if strings.TrimSpace(ip) != "" {
			nonEmptyIPs = append(nonEmptyIPs, ip)
		}
	}

	if len(nonEmptyIPs) == 0 {
		return "unknown"
	}

	return nonEmptyIPs[rand.Intn(len(nonEmptyIPs))]
}
